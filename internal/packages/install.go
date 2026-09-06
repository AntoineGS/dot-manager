package packages

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AntoineGS/tidydots/internal/platform"
)

// Install installs a single package using the best available method.
// It resolves the main method and dependency order once, then runs dependencies
// before the main installation. Dry-run uses the same validated execution path
// but returns command previews instead of executing them.
func (m *Manager) Install(pkg Package) InstallResult {
	result := InstallResult{Package: pkg.Name}
	if !m.DryRun {
		defer ResetInstalledCache()
	}

	// Validate all package names before executing any commands to prevent flag injection
	if method, msg, ok := validatePackageNames(pkg); !ok {
		result.Method = method
		result.Success = false
		result.Message = msg

		return result
	}

	plan := PlanInstallation(pkg, m.Config, m.OS, m.Available)
	if plan.Method == MethodNone {
		return m.installPlannedMain(pkg, plan, nil)
	}
	if msg := validateSelectedMain(pkg, plan.Method, m.OS); msg != "" {
		result.Method = plan.Method
		result.Message = msg
		return result
	}
	var previews []string
	for _, dep := range plan.Dependencies {
		ok, msg := m.installWithManager(dep.Manager, dep.Name)
		if !ok {
			result.Method = string(dep.Manager)
			result.Message = fmt.Sprintf("Dependency %s failed: %s", dep.Name, msg)
			return result
		}
		previews = append(previews, msg)
	}
	return m.installPlannedMain(pkg, plan, previews)
}

func (m *Manager) installPlannedMain(pkg Package, plan InstallationPlan, previews []string) (result InstallResult) {
	result.Package = pkg.Name
	defer func() {
		if m.DryRun && len(previews) > 0 {
			result.Message = strings.Join(append(previews, result.Message), "\n")
		}
	}()
	result.Method = plan.Method
	switch plan.Method {
	case string(Git):
		result.Success, result.Message = m.installGitPackage(*pkg.Managers[Git].Git)
	case string(Installer):
		result.Success, result.Message = m.installInstallerPackage(*pkg.Managers[Installer].Installer)
	case MethodCustom:
		result.Success, result.Message = m.runCustomCommand(pkg.Custom[m.OS])
	case MethodURL:
		result.Success, result.Message = m.installFromURL(pkg.URL[m.OS])
	case MethodNone:
		result.Method = ""
		result.Message = "No installation method available for this OS/system"
	default:
		mgr := PackageManager(plan.Method)
		result.Success, result.Message = m.installWithManager(mgr, pkg.Managers[mgr].PackageName)
	}

	return result
}

// validatePackageNames checks that all package names and dependency names in the
// package are safe for use as CLI arguments. It returns the manager method, an
// error message, and false if any name is invalid.
func validatePackageNames(pkg Package) (string, string, bool) {
	for mgr, val := range pkg.Managers {
		if mgr == Git || mgr == Installer {
			continue
		}

		if val.PackageName != "" {
			if err := ValidatePackageName(val.PackageName); err != nil {
				return string(mgr), fmt.Sprintf("Invalid package name: %v", err), false
			}
		}

		for _, dep := range val.Deps {
			if err := ValidatePackageName(dep); err != nil {
				return string(mgr), fmt.Sprintf("Invalid dependency name: %v", err), false
			}
		}
	}

	return "", "", true
}

// InstallAll installs all packages in the provided slice sequentially.
// It returns a slice of InstallResult, one for each package, indicating
// the success or failure of each installation.
func (m *Manager) InstallAll(packages []Package) []InstallResult {
	results := make([]InstallResult, 0, len(packages))
	for _, pkg := range packages {
		results = append(results, m.Install(pkg))
	}

	return results
}

func (m *Manager) installWithManager(mgr PackageManager, pkgName string) (bool, string) {
	mc, ok := managerCmds[mgr]
	if !ok {
		if mgr == Git {
			return false, "Git packages should be installed via installGitPackage"
		}

		if mgr == Installer {
			return false, "Installer packages should be installed via installInstallerPackage"
		}

		return false, fmt.Sprintf("Unknown package manager: %s", mgr)
	}

	args := expandArgs(mc.install, pkgName)

	if m.DryRun {
		return true, fmt.Sprintf("Would run: %s", strings.Join(args, " "))
	}

	_, err := m.runner.Run(m.ctx, args[0], args[1:]...) //nolint:gosec // args from trusted lookup table
	if err != nil {
		return false, fmt.Sprintf("Installation failed: %v", err)
	}

	return true, fmt.Sprintf("Installed via %s", mgr)
}

// installGitPackage clones or updates a git repository.
func (m *Manager) installGitPackage(gitCfg GitConfig) (bool, string) {
	targetPath, msg := validatedGitTarget(gitCfg, m.OS)
	if msg != "" {
		return false, msg
	}

	// Check if already cloned
	gitDir := filepath.Join(targetPath, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return m.gitPull(targetPath, gitCfg.Sudo)
	}

	return m.gitClone(gitCfg.URL, targetPath, gitCfg.Branch, gitCfg.Sudo)
}

func (m *Manager) gitClone(repoURL, targetPath, branch string, sudo bool) (bool, string) {
	if err := ValidateGitBranch(branch); err != nil {
		return false, fmt.Sprintf("Invalid git branch: %v", err)
	}
	args := []string{argClone}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	args = append(args, repoURL, targetPath)

	if m.DryRun {
		if sudo {
			return true, fmt.Sprintf("Would run: sudo git %s", strings.Join(args, " "))
		}
		return true, fmt.Sprintf("Would run: git %s", strings.Join(args, " "))
	}

	var err error
	if sudo {
		_, err = m.runner.RunWithSudo(m.ctx, cmdGit, args...)
	} else {
		_, err = m.runner.Run(m.ctx, cmdGit, args...)
	}

	if err != nil {
		return false, fmt.Sprintf("Git clone failed: %v", err)
	}

	return true, "Repository cloned successfully"
}

func (m *Manager) gitPull(repoPath string, sudo bool) (bool, string) {
	if m.DryRun {
		if sudo {
			return true, fmt.Sprintf("Would run: sudo git -C %s pull", repoPath)
		}
		return true, fmt.Sprintf("Would run: git -C %s pull", repoPath)
	}

	var err error
	if sudo {
		_, err = m.runner.RunWithSudo(m.ctx, cmdGit, "-C", repoPath, "pull")
	} else {
		_, err = m.runner.Run(m.ctx, cmdGit, "-C", repoPath, "pull")
	}

	if err != nil {
		return false, fmt.Sprintf("Git pull failed: %v", err)
	}

	return true, "Repository updated successfully"
}

// installInstallerPackage runs an OS-specific shell command to install a package.
// SECURITY NOTE: This intentionally executes arbitrary shell commands from the
// user's configuration file. Users should only use configurations they trust,
// as malicious configs could execute harmful commands.
func (m *Manager) installInstallerPackage(cfg InstallerConfig) (bool, string) {
	command, ok := cfg.Command[m.OS]
	if !ok {
		return false, fmt.Sprintf("No installer command defined for OS: %s", m.OS)
	}

	if m.DryRun {
		return true, fmt.Sprintf("Would run: %s", command)
	}

	var err error
	if m.OS == platform.OSWindows {
		_, err = m.runner.Run(m.ctx, "powershell", "-Command", command) //nolint:gosec // intentional install command from user config
	} else {
		_, err = m.runner.Run(m.ctx, "sh", "-c", command) //nolint:gosec // intentional install command from user config
	}

	if err != nil {
		return false, fmt.Sprintf("Installer command failed: %v", err)
	}

	return true, "Installed via installer"
}

// runCustomCommand executes a custom shell command from the configuration.
// SECURITY NOTE: This intentionally executes arbitrary shell commands from the
// user's configuration file. Users should only use configurations they trust,
// as malicious configs could execute harmful commands.
func (m *Manager) runCustomCommand(command string) (bool, string) {
	if m.DryRun {
		return true, fmt.Sprintf("Would run: %s", command)
	}

	var err error
	if m.OS == platform.OSWindows {
		_, err = m.runner.Run(m.ctx, "powershell", "-Command", command) //nolint:gosec // intentional command from user config
	} else {
		_, err = m.runner.Run(m.ctx, "sh", "-c", command) //nolint:gosec // intentional command from user config
	}

	if err != nil {
		return false, fmt.Sprintf("Custom command failed: %v", err)
	}

	return true, "Installed via custom command"
}

// installFromURL downloads a file from a URL and runs an install command.
// SECURITY NOTE: This intentionally downloads and executes content from URLs
// specified in the user's configuration file. Users should only use configurations
// they trust, as malicious configs could download and execute harmful code.
func (m *Manager) installFromURL(urlInstall URLInstall) (bool, string) {
	if err := validateURLScheme(urlInstall.URL); err != nil {
		return false, fmt.Sprintf("URL rejected: %v", err)
	}

	if m.DryRun {
		return true, fmt.Sprintf("Would download %s and run: %s", urlInstall.URL, urlInstall.Command)
	}

	// Create temp directory to avoid TOCTOU race on the file path
	tmpDir, err := os.MkdirTemp("", "tidydots-*")
	if err != nil {
		return false, fmt.Sprintf("Failed to create temp directory: %v", err)
	}

	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			// Log but don't fail on cleanup errors
			fmt.Printf("[WARN] Failed to remove temp directory %s: %v\n", tmpDir, err)
		}
	}()

	tmpPath := filepath.Join(tmpDir, "installer")

	// Download file
	if m.OS == platform.OSWindows {
		// Escape single quotes in URL and path for PowerShell
		escapedURL := escapePowerShellSingleQuote(urlInstall.URL)
		escapedPath := escapePowerShellSingleQuote(tmpPath)
		_, err = m.runner.Run(m.ctx, "powershell", "-Command", //nolint:gosec // intentional download command
			fmt.Sprintf("Invoke-WebRequest -Uri '%s' -OutFile '%s'", escapedURL, escapedPath))
	} else {
		_, err = m.runner.Run(m.ctx, "curl", "-fsSL", "-o", tmpPath, urlInstall.URL) //nolint:gosec // intentional download command
	}

	if err != nil {
		return false, fmt.Sprintf("Download failed: %v", err)
	}

	// Make executable on Unix
	if m.OS != platform.OSWindows {
		if err := os.Chmod(tmpPath, ExecPerms); err != nil { //nolint:gosec // installer scripts need to be executable
			return false, fmt.Sprintf("Failed to make executable: %v", err)
		}
	}

	// Run install command
	command := strings.ReplaceAll(urlInstall.Command, "{file}", tmpPath)

	if m.OS == platform.OSWindows {
		_, err = m.runner.Run(m.ctx, "powershell", "-Command", command) //nolint:gosec // intentional install command
	} else {
		_, err = m.runner.Run(m.ctx, "sh", "-c", command) //nolint:gosec // intentional install command
	}

	if err != nil {
		return false, fmt.Sprintf("Install command failed: %v", err)
	}

	return true, "Installed via URL"
}
