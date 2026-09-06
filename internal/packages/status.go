package packages

import (
	"context"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
	"github.com/AntoineGS/tidydots/internal/config"
)

// installedCache holds the lazily-populated set of installed package IDs for
// managers that support bulk listing (see managerCmd.bulkList).
// The cache is populated once per manager on the first IsInstalled call.
var installedCache sync.Map // map[string]*bulkCacheEntry

type bulkCacheEntry struct {
	once         sync.Once
	installedIDs map[string]bool // lowercase ID → true
}

// IsInstalled checks if a package is installed on the system.
// For managers with bulk list support, it runs a single list command and caches
// the results. For other managers, it runs the per-package check command.
// Returns true if the package is installed, false otherwise.
func IsInstalled(ctx context.Context, pkgName string, manager string) bool {
	return isInstalledWithRunner(ctx, pkgName, manager, cmdexec.OsRunner{})
}

// isInstalledWithRunner checks if a package is installed using the given runner.
func isInstalledWithRunner(ctx context.Context, pkgName string, manager string, r cmdexec.Runner) bool {
	mc, ok := managerCmds[PackageManager(manager)]
	if !ok {
		slog.Debug("no check command for manager, assuming not installed",
			slog.String("package", pkgName),
			slog.String("manager", manager))
		return false
	}

	// Managers with bulk list support: run one command, cache all installed IDs
	if mc.bulkList != nil {
		return isInstalledBulk(ctx, pkgName, manager, mc)
	}

	return isInstalledSingle(ctx, pkgName, manager, mc, r)
}

// isInstalledBulk checks installation via cached bulk list output.
func isInstalledBulk(ctx context.Context, pkgName, manager string, mc managerCmd) bool {
	val, _ := installedCache.LoadOrStore(manager, &bulkCacheEntry{})
	entry, _ := val.(*bulkCacheEntry) //nolint:errcheck // type is guaranteed by LoadOrStore above

	entry.once.Do(func() {
		entry.installedIDs = mc.bulkList(ctx)
	})

	found := entry.installedIDs[strings.ToLower(pkgName)]
	if found {
		slog.Debug("package detected as installed (bulk cache)",
			slog.String("package", pkgName),
			slog.String("manager", manager))
	} else {
		slog.Debug("package not found in bulk cache",
			slog.String("package", pkgName),
			slog.String("manager", manager))
	}

	return found
}

// isInstalledSingle checks installation by running the per-package check command.
func isInstalledSingle(ctx context.Context, pkgName, manager string, mc managerCmd, r cmdexec.Runner) bool {
	args := expandArgs(mc.check, pkgName)

	result, err := r.Run(ctx, args[0], args[1:]...) //nolint:gosec // args from trusted lookup table

	if err != nil {
		errMsg := strings.TrimSpace(string(result.Stderr))
		slog.Debug("package check command failed",
			slog.String("package", pkgName),
			slog.String("manager", manager),
			slog.String("command", strings.Join(args, " ")),
			slog.String("error", err.Error()),
			slog.String("stderr", errMsg))
		return false
	}

	slog.Debug("package detected as installed",
		slog.String("package", pkgName),
		slog.String("manager", manager))

	return true
}

// ResetInstalledCache clears the bulk installed cache, causing the next
// IsInstalled call to re-query. Useful for tests and after install operations.
func ResetInstalledCache() {
	installedCache.Range(func(key, _ any) bool {
		installedCache.Delete(key)
		return true
	})
}

// IsInstallerInstalled checks if an installer package is installed by looking up
// the binary name in PATH. Returns false if no binary name is configured.
func IsInstallerInstalled(binary string) bool {
	if binary == "" {
		return false
	}

	return isInstallerInstalledWithRunner(binary, cmdexec.OsRunner{})
}

// isInstallerInstalledWithRunner checks binary availability using the given runner.
func isInstallerInstalledWithRunner(binary string, r cmdexec.Runner) bool {
	_, err := r.LookPath(binary)
	return err == nil
}

// IsGitInstalled checks whether a git repository has been cloned at the
// OS-specific target path. It returns true if the target directory contains
// a .git subdirectory.
func IsGitInstalled(targets map[string]string, osType string) bool {
	target, ok := targets[osType]
	if !ok || target == "" {
		return false
	}

	// Expand ~ (git clone doesn't do shell tilde expansion)
	target = config.ExpandPath(target, nil)

	_, err := os.Stat(filepath.Join(target, ".git"))
	return err == nil
}

// CanInstall checks if a package can be installed on this system.
// It returns true if any of the package's installation methods (manager,
// custom command, or URL) are available for the current OS and package managers.
func (m *Manager) CanInstall(pkg Package) bool {
	method := m.GetInstallMethod(pkg)
	if method == string(Installer) {
		_, ok := pkg.Managers[Installer].Installer.Command[m.OS]
		return ok
	}
	return method != MethodNone
}

// GetInstallMethod returns the method that would be used to install a package.
// It shares Install's special-method precedence and configured manager priority.
// Configured git/installer methods are reported even if their OS-specific
// settings are missing; Install returns the corresponding explicit error.
func (m *Manager) GetInstallMethod(pkg Package) string {
	return PlanInstallation(pkg, m.Config, m.OS, m.Available).Method
}

// GetInstallablePackages returns packages from the configuration that can be
// installed on this system. It filters the configured packages to only those
// with at least one available installation method.
func (m *Manager) GetInstallablePackages() []Package {
	result := make([]Package, 0, len(m.Config.Packages))

	for _, pkg := range m.Config.Packages {
		if m.CanInstall(pkg) {
			result = append(result, pkg)
		}
	}

	return result
}
