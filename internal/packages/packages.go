// Package packages provides multi-package-manager support.
package packages

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
	"gopkg.in/yaml.v3"
)

// File permissions constants
const (
	// ExecPerms are the permissions for executable files (rwxr-xr-x)
	// Owner: read, write, execute; Group: read, execute; Other: read, execute
	ExecPerms os.FileMode = 0755
)

// PackageManager represents a supported package manager identifier.
// It is used to specify which package manager should be used for installing
// a package, such as pacman, apt, brew, winget, etc. The supported values
// are defined as constants (Pacman, Yay, Paru, Apt, Dnf, Brew, Winget, Scoop, Choco).
type PackageManager string

// Supported package manager identifiers.
const (
	// Pacman is the Arch Linux package manager
	Pacman PackageManager = "pacman"
	// Yay is an AUR helper for Arch Linux
	Yay PackageManager = "yay"
	// Paru is an AUR helper for Arch Linux
	Paru PackageManager = "paru"
	// Apt is the Debian/Ubuntu package manager
	Apt PackageManager = "apt"
	// Dnf is the Fedora package manager
	Dnf PackageManager = "dnf"
	// Brew is the macOS package manager
	Brew PackageManager = "brew"
	// Winget is the Windows package manager
	Winget PackageManager = "winget"
	// Scoop is a Windows package manager
	Scoop PackageManager = "scoop"
	// Choco is the Chocolatey Windows package manager
	Choco PackageManager = "choco"
	// Git is the git package manager for repository clones
	Git PackageManager = "git"
	// Installer is the installer package manager for shell command-based installation
	Installer PackageManager = "installer"
)

// GitConfig represents git-specific package configuration.
// It contains the repository URL, optional branch, and OS-specific clone destinations.
type GitConfig struct {
	URL     string            `yaml:"url"`
	Branch  string            `yaml:"branch,omitempty"`
	Targets map[string]string `yaml:"targets"`
	Sudo    bool              `yaml:"sudo,omitempty"`
}

// InstallerConfig represents installer-specific package configuration.
// It contains OS-specific shell commands and an optional binary name for install checks.
type InstallerConfig struct {
	Command map[string]string `yaml:"command"`
	Binary  string            `yaml:"binary,omitempty"`
}

// ManagerValue represents a typed value for a package manager entry.
// It holds either a package name string (for traditional managers like pacman, apt),
// a GitConfig (for git repositories), or an InstallerConfig (for shell command-based installation).
type ManagerValue struct {
	PackageName string
	Git         *GitConfig
	Installer   *InstallerConfig
}

// IsGit returns true if this manager value represents a git package configuration.
func (v ManagerValue) IsGit() bool { return v.Git != nil }

// IsInstaller returns true if this manager value represents an installer package configuration.
func (v ManagerValue) IsInstaller() bool { return v.Installer != nil }

// Package represents a package to install with multiple installation methods.
// A package can be installed via a package manager (Managers), a custom shell
// command (Custom), or by downloading from a URL (URL). The installation method
// is selected based on availability, with package managers tried first, then
// custom commands, and finally URL-based installation. A `when` expression can
// conditionally include the package based on template variables.
type Package struct {
	Name        string                          `yaml:"name"`
	Description string                          `yaml:"description,omitempty"`
	Managers    map[PackageManager]ManagerValue `yaml:"managers,omitempty"`
	Custom      map[string]string               `yaml:"custom,omitempty"` // OS -> command
	URL         map[string]URLInstall           `yaml:"url,omitempty"`    // OS -> URL install
	When        string                          `yaml:"when,omitempty"`
}

// UnmarshalYAML implements custom YAML unmarshaling for Package.
// It handles the Managers field specially to support both string values (for traditional
// package managers) and GitConfig structs (for git repositories).
func (p *Package) UnmarshalYAML(node *yaml.Node) error {
	// Define a temporary struct that matches Package but with a different Managers type
	type packageAlias struct {
		Name        string                `yaml:"name"`
		Description string                `yaml:"description,omitempty"`
		Managers    map[string]yaml.Node  `yaml:"managers,omitempty"`
		Custom      map[string]string     `yaml:"custom,omitempty"`
		URL         map[string]URLInstall `yaml:"url,omitempty"`
		When        string                `yaml:"when,omitempty"`
	}

	var alias packageAlias
	if err := node.Decode(&alias); err != nil {
		return err
	}

	// Copy simple fields
	p.Name = alias.Name
	p.Description = alias.Description
	p.Custom = alias.Custom
	p.URL = alias.URL
	p.When = alias.When

	// Process managers map
	p.Managers = make(map[PackageManager]ManagerValue)
	for key, valueNode := range alias.Managers {
		pm := PackageManager(key)

		switch pm { //nolint:exhaustive // default handles all traditional string-based managers
		case Git:
			var gitCfg GitConfig
			if err := valueNode.Decode(&gitCfg); err != nil {
				return fmt.Errorf("failed to decode git config: %w", err)
			}
			p.Managers[pm] = ManagerValue{Git: &gitCfg}

		case Installer:
			var installerCfg InstallerConfig
			if err := valueNode.Decode(&installerCfg); err != nil {
				return fmt.Errorf("failed to decode installer config: %w", err)
			}
			p.Managers[pm] = ManagerValue{Installer: &installerCfg}

		default:
			// Traditional managers are strings
			var pkgName string
			if err := valueNode.Decode(&pkgName); err != nil {
				return fmt.Errorf("failed to decode manager %s: %w", key, err)
			}
			p.Managers[pm] = ManagerValue{PackageName: pkgName}
		}
	}

	return nil
}

// URLInstall represents installation from a URL with download and command execution.
// The URL field specifies where to download the installer file, and the Command
// field specifies the shell command to run after download. Use {file} as a
// placeholder in Command to reference the downloaded file path.
type URLInstall struct {
	URL     string `yaml:"url"`
	Command string `yaml:"command"` // Command to run after download, use {file} as placeholder
}

// Config holds the packages configuration including the list of packages
// to manage, default package manager settings, and priority ordering for manager
// selection. DefaultManager specifies which manager to prefer when multiple are
// available, and ManagerPriority allows fine-grained control over the order in
// which package managers are tried.
type Config struct {
	Packages        []Package        `yaml:"packages"`
	DefaultManager  PackageManager   `yaml:"default_manager,omitempty"`
	ManagerPriority []PackageManager `yaml:"manager_priority,omitempty"`
}

// Manager handles package installation with platform detection and manager selection.
// It detects available package managers on the system, selects a preferred manager
// based on configuration and OS, and provides methods to install packages using
// the appropriate installation method. It supports dry-run mode for previewing
// operations and verbose mode for detailed output.
type Manager struct {
	ctx          context.Context
	Config       *Config
	OS           string
	Preferred    PackageManager
	Available    []PackageManager
	availableSet map[PackageManager]bool
	DryRun       bool
	Verbose      bool
}

// NewManager creates a new package Manager with the given configuration.
// It detects available package managers on the system and selects a preferred
// manager based on the configuration priority, default manager setting, or
// OS-specific defaults. The osType parameter specifies the target OS (linux/windows),
// and dryRun/verbose control the execution mode.
func NewManager(cfg *Config, osType string, dryRun, verbose bool) *Manager {
	m := &Manager{
		ctx:     context.Background(),
		Config:  cfg,
		OS:      osType,
		DryRun:  dryRun,
		Verbose: verbose,
	}
	m.detectAvailableManagers()
	m.selectPreferredManager()

	return m
}

// WithContext returns a new Manager with the given context for cancellation support.
func (m *Manager) WithContext(ctx context.Context) *Manager {
	m2 := *m
	m2.ctx = ctx
	return &m2
}

func (m *Manager) detectAvailableManagers() {
	m.availableSet = make(map[PackageManager]bool)
	for _, mgr := range platform.DetectAvailableManagers() {
		pm := PackageManager(mgr)
		m.Available = append(m.Available, pm)
		m.availableSet[pm] = true
	}
}

func (m *Manager) selectPreferredManager() {
	// Use configured priority if available
	if len(m.Config.ManagerPriority) > 0 {
		for _, mgr := range m.Config.ManagerPriority {
			if m.HasManager(mgr) {
				m.Preferred = mgr
				return
			}
		}
	}

	// Use default if set and available
	if m.Config.DefaultManager != "" && m.HasManager(m.Config.DefaultManager) {
		m.Preferred = m.Config.DefaultManager
		return
	}

	// Auto-select based on OS
	if m.OS == platform.OSWindows {
		for _, mgr := range []PackageManager{Winget, Scoop, Choco} {
			if m.HasManager(mgr) {
				m.Preferred = mgr
				return
			}
		}
	} else {
		// Linux/macOS priority
		for _, mgr := range []PackageManager{Yay, Paru, Pacman, Apt, Dnf, Brew} {
			if m.HasManager(mgr) {
				m.Preferred = mgr
				return
			}
		}
	}
}

// HasManager checks if a package manager is available on the system.
// It returns true if the specified manager was detected during initialization.
func (m *Manager) HasManager(mgr PackageManager) bool {
	if m.availableSet == nil {
		m.availableSet = make(map[PackageManager]bool, len(m.Available))
		for _, pm := range m.Available {
			m.availableSet[pm] = true
		}
	}
	return m.availableSet[mgr]
}

// InstallResult represents the result of a package installation attempt.
// It contains the package name, whether the installation succeeded, a message
// describing the outcome, and the method used (e.g., "pacman", "custom", "url").
// This is returned by Install and InstallAll methods to report installation status.
type InstallResult struct {
	Package string
	Message string
	Method  string
	Success bool
}

// Install installs a single package using the best available method.
// It tries git packages first, then package managers (in order of availability),
// then custom commands, and finally URL-based installation. Returns an InstallResult
// indicating success or failure with a descriptive message.
func (m *Manager) Install(pkg Package) InstallResult {
	result := InstallResult{Package: pkg.Name}

	// Check if this is a git package
	if gitValue, ok := pkg.Managers[Git]; ok && gitValue.IsGit() {
		result.Method = string(Git)
		success, msg := m.installGitPackage(*gitValue.Git)
		result.Success = success
		result.Message = msg
		return result
	}

	// Check if this is an installer package
	if installerValue, ok := pkg.Managers[Installer]; ok && installerValue.IsInstaller() {
		result.Method = string(Installer)
		success, msg := m.installInstallerPackage(*installerValue.Installer)
		result.Success = success
		result.Message = msg
		return result
	}

	// Try package managers
	if len(pkg.Managers) > 0 {
		for _, mgr := range m.Available {
			// Skip git and installer managers (already handled above)
			if mgr == Git || mgr == Installer {
				continue
			}

			if val, ok := pkg.Managers[mgr]; ok {
				result.Method = string(mgr)
				success, msg := m.installWithManager(mgr, val.PackageName)
				result.Success = success
				result.Message = msg

				return result
			}
		}
	}

	// Try custom command
	if cmd, ok := pkg.Custom[m.OS]; ok {
		result.Method = "custom"
		success, msg := m.runCustomCommand(cmd)
		result.Success = success
		result.Message = msg

		return result
	}

	// Try URL install
	if urlInstall, ok := pkg.URL[m.OS]; ok {
		result.Method = "url"
		success, msg := m.installFromURL(urlInstall)
		result.Success = success
		result.Message = msg

		return result
	}

	result.Success = false
	result.Message = "No installation method available for this OS/system"

	return result
}

// managerCmd defines the install and check commands for a package manager.
// The placeholder "{pkg}" in args is replaced with the actual package name.
type managerCmd struct {
	install []string // command args for install, e.g. {"sudo", "pacman", "-S", "--noconfirm", "{pkg}"}
	check   []string // command args for checking install status, e.g. {"pacman", "-Q", "{pkg}"}
}

var managerCmds = map[PackageManager]managerCmd{
	Pacman: {install: []string{"sudo", "pacman", "-S", "--noconfirm", "{pkg}"}, check: []string{"pacman", "-Q", "{pkg}"}},
	Yay:    {install: []string{"yay", "-S", "--noconfirm", "{pkg}"}, check: []string{"pacman", "-Q", "{pkg}"}},
	Paru:   {install: []string{"paru", "-S", "--noconfirm", "{pkg}"}, check: []string{"pacman", "-Q", "{pkg}"}},
	Apt:    {install: []string{"sudo", "apt-get", "install", "-y", "{pkg}"}, check: []string{"dpkg", "-s", "{pkg}"}},
	Dnf:    {install: []string{"sudo", "dnf", "install", "-y", "{pkg}"}, check: []string{"rpm", "-q", "{pkg}"}},
	Brew:   {install: []string{"brew", "install", "{pkg}"}, check: []string{"brew", "list", "{pkg}"}},
	Winget: {install: []string{"winget", "install", "--accept-package-agreements", "--accept-source-agreements", "{pkg}"}, check: []string{"winget", "list", "--id", "{pkg}"}},
	Scoop:  {install: []string{"scoop", "install", "{pkg}"}, check: []string{"scoop", "info", "{pkg}"}},
	Choco:  {install: []string{"choco", "install", "-y", "{pkg}"}, check: []string{"choco", "list", "--local-only", "{pkg}"}},
}

// expandArgs replaces "{pkg}" placeholders in args with the actual package name.
func expandArgs(args []string, pkgName string) []string {
	result := make([]string, len(args))
	for i, arg := range args {
		if arg == "{pkg}" {
			result[i] = pkgName
		} else {
			result[i] = arg
		}
	}

	return result
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
	cmd := exec.CommandContext(m.ctx, args[0], args[1:]...) //nolint:gosec // args from trusted lookup table

	if m.DryRun {
		return true, fmt.Sprintf("Would run: %s", strings.Join(cmd.Args, " "))
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return false, fmt.Sprintf("Installation failed: %v", err)
	}

	return true, fmt.Sprintf("Installed via %s", mgr)
}

// runCustomCommand executes a custom shell command from the configuration.
// SECURITY NOTE: This intentionally executes arbitrary shell commands from the
// user's configuration file. Users should only use configurations they trust,
// as malicious configs could execute harmful commands.
func (m *Manager) runCustomCommand(command string) (bool, string) {
	if m.DryRun {
		return true, fmt.Sprintf("Would run: %s", command)
	}

	var cmd *exec.Cmd
	if m.OS == platform.OSWindows {
		cmd = exec.CommandContext(m.ctx, "powershell", "-Command", command)
	} else {
		cmd = exec.CommandContext(m.ctx, "sh", "-c", command)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return false, fmt.Sprintf("Custom command failed: %v", err)
	}

	return true, "Installed via custom command"
}

// installFromURL downloads a file from a URL and runs an install command.
// SECURITY NOTE: This intentionally downloads and executes content from URLs
// specified in the user's configuration file. Users should only use configurations
// they trust, as malicious configs could download and execute harmful code.
func (m *Manager) installFromURL(urlInstall URLInstall) (bool, string) {
	if m.DryRun {
		return true, fmt.Sprintf("Would download %s and run: %s", urlInstall.URL, urlInstall.Command)
	}

	// Create temp directory to avoid TOCTOU race on the file path
	tmpDir, err := os.MkdirTemp("", "dot-manager-*")
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
	var downloadCmd *exec.Cmd

	if m.OS == platform.OSWindows {
		// Escape single quotes in URL and path for PowerShell
		escapedURL := strings.ReplaceAll(urlInstall.URL, "'", "''")
		escapedPath := strings.ReplaceAll(tmpPath, "'", "''")
		downloadCmd = exec.CommandContext(m.ctx, "powershell", "-Command", //nolint:gosec // intentional download command
			fmt.Sprintf("Invoke-WebRequest -Uri '%s' -OutFile '%s'", escapedURL, escapedPath))
	} else {
		downloadCmd = exec.CommandContext(m.ctx, "curl", "-fsSL", "-o", tmpPath, urlInstall.URL) //nolint:gosec // intentional download command
	}

	if err := downloadCmd.Run(); err != nil {
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

	var installCmd *exec.Cmd
	if m.OS == platform.OSWindows {
		installCmd = exec.CommandContext(m.ctx, "powershell", "-Command", command) //nolint:gosec // intentional install command
	} else {
		installCmd = exec.CommandContext(m.ctx, "sh", "-c", command) //nolint:gosec // intentional install command
	}

	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	installCmd.Stdin = os.Stdin

	if err := installCmd.Run(); err != nil {
		return false, fmt.Sprintf("Install command failed: %v", err)
	}

	return true, "Installed via URL"
}

// installGitPackage clones or updates a git repository.
func (m *Manager) installGitPackage(gitCfg GitConfig) (bool, string) {
	// Get target path for current OS
	targetPath, ok := gitCfg.Targets[m.OS]
	if !ok {
		return false, fmt.Sprintf("No git target path defined for OS: %s", m.OS)
	}

	// Expand path (handle ~)
	if strings.HasPrefix(targetPath, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return false, fmt.Sprintf("Failed to get home directory: %v", err)
		}
		targetPath = filepath.Join(home, targetPath[1:])
	}

	// Check if already cloned
	gitDir := filepath.Join(targetPath, ".git")
	if _, err := os.Stat(gitDir); err == nil {
		return m.gitPull(targetPath, gitCfg.Sudo)
	}

	return m.gitClone(gitCfg.URL, targetPath, gitCfg.Branch, gitCfg.Sudo)
}

func (m *Manager) gitClone(repoURL, targetPath, branch string, sudo bool) (bool, string) {
	args := []string{"clone"}
	if branch != "" {
		args = append(args, "-b", branch)
	}
	args = append(args, repoURL, targetPath)

	var cmd *exec.Cmd
	if sudo {
		// Prepend sudo to the command
		args = append([]string{"git"}, args...)
		cmd = exec.CommandContext(m.ctx, "sudo", args...)
	} else {
		cmd = exec.CommandContext(m.ctx, "git", args...)
	}

	if m.DryRun {
		return true, fmt.Sprintf("Would run: %s", strings.Join(cmd.Args, " "))
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return false, fmt.Sprintf("Git clone failed: %v", err)
	}

	return true, "Repository cloned successfully"
}

func (m *Manager) gitPull(repoPath string, sudo bool) (bool, string) {
	var cmd *exec.Cmd
	if sudo {
		cmd = exec.CommandContext(m.ctx, "sudo", "git", "-C", repoPath, "pull")
	} else {
		cmd = exec.CommandContext(m.ctx, "git", "-C", repoPath, "pull")
	}

	if m.DryRun {
		return true, fmt.Sprintf("Would run: %s", strings.Join(cmd.Args, " "))
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
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

	var cmd *exec.Cmd
	if m.OS == platform.OSWindows {
		cmd = exec.CommandContext(m.ctx, "powershell", "-Command", command) //nolint:gosec // intentional install command from user config
	} else {
		cmd = exec.CommandContext(m.ctx, "sh", "-c", command) //nolint:gosec // intentional install command from user config
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return false, fmt.Sprintf("Installer command failed: %v", err)
	}

	return true, "Installed via installer"
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

// FilterPackages returns packages that match the given when expressions.
// It evaluates each package's When expression and returns only those that match.
func FilterPackages(packages []Package, renderer config.PathRenderer) []Package {
	result := make([]Package, 0, len(packages))

	for _, pkg := range packages {
		if config.EvaluateWhen(pkg.When, renderer) {
			result = append(result, pkg)
		}
	}

	return result
}

// CanInstall checks if a package can be installed on this system.
// It returns true if any of the package's installation methods (manager,
// custom command, or URL) are available for the current OS and package managers.
func (m *Manager) CanInstall(pkg Package) bool {
	// Check managers
	for _, mgr := range m.Available {
		if _, ok := pkg.Managers[mgr]; ok {
			return true
		}
	}
	// Check installer (always available when configured with a command for current OS)
	if val, ok := pkg.Managers[Installer]; ok && val.IsInstaller() {
		if _, hasCmd := val.Installer.Command[m.OS]; hasCmd {
			return true
		}
	}
	// Check custom
	if _, ok := pkg.Custom[m.OS]; ok {
		return true
	}
	// Check URL
	if _, ok := pkg.URL[m.OS]; ok {
		return true
	}

	return false
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

// GetInstallMethod returns the method that would be used to install a package.
// It returns the name of the first available package manager, "installer" for
// installer packages, "custom" if a custom command is available, "url" for
// URL-based installation, or "none" if no installation method is available.
func (m *Manager) GetInstallMethod(pkg Package) string {
	for _, mgr := range m.Available {
		if _, ok := pkg.Managers[mgr]; ok {
			return string(mgr)
		}
	}

	// Check installer (always available when configured with a command for current OS)
	if val, ok := pkg.Managers[Installer]; ok && val.IsInstaller() {
		if _, hasCmd := val.Installer.Command[m.OS]; hasCmd {
			return string(Installer)
		}
	}

	if _, ok := pkg.Custom[m.OS]; ok {
		return "custom"
	}

	if _, ok := pkg.URL[m.OS]; ok {
		return "url"
	}

	return "none"
}

// IsInstalled checks if a package is installed on the system.
// It uses the appropriate package manager query command based on the installation method.
// Returns true if the package is installed, false otherwise.
func IsInstalled(ctx context.Context, pkgName string, manager string) bool {
	mc, ok := managerCmds[PackageManager(manager)]
	if !ok {
		// For git, custom, url methods, we can't easily check installation status
		return false
	}

	args := expandArgs(mc.check, pkgName)
	cmd := exec.CommandContext(ctx, args[0], args[1:]...) //nolint:gosec // args from trusted lookup table

	// Run silently - just check exit code
	cmd.Stdout = nil
	cmd.Stderr = nil
	err := cmd.Run()

	return err == nil
}

// IsInstallerInstalled checks if an installer package is installed by looking up
// the binary name in PATH. Returns false if no binary name is configured.
func IsInstallerInstalled(binary string) bool {
	if binary == "" {
		return false
	}

	return platform.IsCommandAvailable(binary)
}

// FromEntry creates a Package from a config.Entry.
// It converts the entry's package configuration into a Package struct,
// mapping managers and URL install configurations. Returns nil if the
// entry does not have a package configuration.
func FromEntry(e config.Entry) *Package {
	if e.Package == nil {
		return nil
	}

	managers := make(map[PackageManager]ManagerValue)
	for k, v := range e.Package.Managers {
		// Convert config.GitPackage to packages.GitConfig
		if k == "git" && v.IsGit() {
			managers[Git] = ManagerValue{Git: &GitConfig{
				URL:     v.Git.URL,
				Branch:  v.Git.Branch,
				Targets: v.Git.Targets,
				Sudo:    v.Git.Sudo,
			}}
			continue
		}
		// Convert config.InstallerPackage to packages.InstallerConfig
		if k == "installer" && v.IsInstaller() {
			managers[Installer] = ManagerValue{Installer: &InstallerConfig{
				Command: v.Installer.Command,
				Binary:  v.Installer.Binary,
			}}
			continue
		}
		managers[PackageManager(k)] = ManagerValue{PackageName: v.PackageName}
	}

	urlInstalls := make(map[string]URLInstall)
	for k, v := range e.Package.URL {
		urlInstalls[k] = URLInstall{
			URL:     v.URL,
			Command: v.Command,
		}
	}

	return &Package{
		Name:        e.Name,
		Description: e.Description,
		Managers:    managers,
		Custom:      e.Package.Custom,
		URL:         urlInstalls,
		When:        e.When,
	}
}

// FromEntries creates a slice of Packages from a slice of config.Entry.
// It filters the entries to only those with package configurations and
// converts each to a Package struct.
func FromEntries(entries []config.Entry) []Package {
	result := make([]Package, 0, len(entries))

	for _, e := range entries {
		if pkg := FromEntry(e); pkg != nil {
			result = append(result, *pkg)
		}
	}

	return result
}
