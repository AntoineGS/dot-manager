package packages

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
	"gopkg.in/yaml.v3"
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
)

// GitConfig represents git-specific package configuration.
// It contains the repository URL, optional branch, and OS-specific clone destinations.
type GitConfig struct {
	URL     string            `yaml:"url"`
	Branch  string            `yaml:"branch,omitempty"`
	Targets map[string]string `yaml:"targets"`
	Sudo    bool              `yaml:"sudo,omitempty"`
}

// Package represents a package to install with multiple installation methods.
// A package can be installed via a package manager (Managers), a custom shell
// command (Custom), or by downloading from a URL (URL). The installation method
// is selected based on availability, with package managers tried first, then
// custom commands, and finally URL-based installation. Filters can be used to
// conditionally include the package based on OS, distro, hostname, or user.
type Package struct {
	Name        string                         `yaml:"name"`
	Description string                         `yaml:"description,omitempty"`
	Managers    map[PackageManager]interface{} `yaml:"managers,omitempty"`
	Custom      map[string]string              `yaml:"custom,omitempty"` // OS -> command
	URL         map[string]URLInstall          `yaml:"url,omitempty"`    // OS -> URL install
	Filters     []config.Filter                `yaml:"filters,omitempty"`
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
		Filters     []config.Filter       `yaml:"filters,omitempty"`
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
	p.Filters = alias.Filters

	// Process managers map
	p.Managers = make(map[PackageManager]interface{})
	for key, valueNode := range alias.Managers {
		pm := PackageManager(key)

		// Special handling for git manager
		if pm == Git {
			var gitCfg GitConfig
			if err := valueNode.Decode(&gitCfg); err != nil {
				return fmt.Errorf("failed to decode git config: %w", err)
			}
			p.Managers[pm] = gitCfg
		} else {
			// Traditional managers are strings
			var pkgName string
			if err := valueNode.Decode(&pkgName); err != nil {
				return fmt.Errorf("failed to decode manager %s: %w", key, err)
			}
			p.Managers[pm] = pkgName
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
	Config    *Config
	OS        string
	Preferred PackageManager
	Available []PackageManager
	DryRun    bool
	Verbose   bool
}

// NewManager creates a new package Manager with the given configuration.
// It detects available package managers on the system and selects a preferred
// manager based on the configuration priority, default manager setting, or
// OS-specific defaults. The osType parameter specifies the target OS (linux/windows),
// and dryRun/verbose control the execution mode.
func NewManager(cfg *Config, osType string, dryRun, verbose bool) *Manager {
	m := &Manager{
		Config:  cfg,
		OS:      osType,
		DryRun:  dryRun,
		Verbose: verbose,
	}
	m.detectAvailableManagers()
	m.selectPreferredManager()

	return m
}

func (m *Manager) detectAvailableManagers() {
	for _, mgr := range platform.DetectAvailableManagers() {
		m.Available = append(m.Available, PackageManager(mgr))
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
	for _, available := range m.Available {
		if available == mgr {
			return true
		}
	}

	return false
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
	if gitCfg, ok := pkg.Managers[Git]; ok {
		result.Method = "git"
		success, msg := m.installGitPackage(pkg, gitCfg)
		result.Success = success
		result.Message = msg
		return result
	}

	// Try package managers
	if len(pkg.Managers) > 0 {
		for _, mgr := range m.Available {
			if pkgName, ok := pkg.Managers[mgr]; ok {
				result.Method = string(mgr)
				// Type assert to string for traditional package managers
				pkgNameStr, ok := pkgName.(string)
				if !ok {
					continue
				}
				success, msg := m.installWithManager(mgr, pkgNameStr)
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

func (m *Manager) installWithManager(mgr PackageManager, pkgName string) (bool, string) {
	var cmd *exec.Cmd

	switch mgr {
	case Pacman:
		cmd = exec.CommandContext(context.Background(), "sudo", "pacman", "-S", "--noconfirm", pkgName)
	case Yay:
		cmd = exec.CommandContext(context.Background(), "yay", "-S", "--noconfirm", pkgName)
	case Paru:
		cmd = exec.CommandContext(context.Background(), "paru", "-S", "--noconfirm", pkgName)
	case Apt:
		cmd = exec.CommandContext(context.Background(), "sudo", "apt-get", "install", "-y", pkgName)
	case Dnf:
		cmd = exec.CommandContext(context.Background(), "sudo", "dnf", "install", "-y", pkgName)
	case Brew:
		cmd = exec.CommandContext(context.Background(), "brew", "install", pkgName)
	case Winget:
		cmd = exec.CommandContext(context.Background(), "winget", "install", "--accept-package-agreements", "--accept-source-agreements", pkgName)
	case Scoop:
		cmd = exec.CommandContext(context.Background(), "scoop", "install", pkgName)
	case Choco:
		cmd = exec.CommandContext(context.Background(), "choco", "install", "-y", pkgName)
	case Git:
		return false, "Git packages should be installed via installGitPackage"
	default:
		return false, fmt.Sprintf("Unknown package manager: %s", mgr)
	}

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
	if runtime.GOOS == platform.OSWindows {
		cmd = exec.CommandContext(context.Background(), "powershell", "-Command", command)
	} else {
		cmd = exec.CommandContext(context.Background(), "sh", "-c", command)
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

	// Create temp file
	tmpFile, err := os.CreateTemp("", "dot-manager-*")
	if err != nil {
		return false, fmt.Sprintf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()

	// Close immediately - we only need the path
	if err := tmpFile.Close(); err != nil {
		return false, fmt.Sprintf("Failed to close temp file: %v", err)
	}

	// Ensure cleanup on all exit paths
	defer func() {
		if err := os.Remove(tmpPath); err != nil && !os.IsNotExist(err) {
			// Log but don't fail on cleanup errors
			fmt.Printf("[WARN] Failed to remove temp file %s: %v\n", tmpPath, err)
		}
	}()

	// Download file
	var downloadCmd *exec.Cmd

	if runtime.GOOS == platform.OSWindows {
		// Escape single quotes in URL and path for PowerShell
		escapedURL := strings.ReplaceAll(urlInstall.URL, "'", "''")
		escapedPath := strings.ReplaceAll(tmpPath, "'", "''")
		downloadCmd = exec.CommandContext(context.Background(), "powershell", "-Command", //nolint:gosec // intentional download command
			fmt.Sprintf("Invoke-WebRequest -Uri '%s' -OutFile '%s'", escapedURL, escapedPath))
	} else {
		downloadCmd = exec.CommandContext(context.Background(), "curl", "-fsSL", "-o", tmpPath, urlInstall.URL) //nolint:gosec // intentional download command
	}

	if err := downloadCmd.Run(); err != nil {
		return false, fmt.Sprintf("Download failed: %v", err)
	}

	// Make executable on Unix
	if runtime.GOOS != platform.OSWindows {
		if err := os.Chmod(tmpPath, 0755); err != nil { //nolint:gosec // installer scripts need to be executable
			return false, fmt.Sprintf("Failed to make executable: %v", err)
		}
	}

	// Run install command
	command := strings.ReplaceAll(urlInstall.Command, "{file}", tmpPath)

	var installCmd *exec.Cmd
	if runtime.GOOS == platform.OSWindows {
		installCmd = exec.CommandContext(context.Background(), "powershell", "-Command", command) //nolint:gosec // intentional install command
	} else {
		installCmd = exec.CommandContext(context.Background(), "sh", "-c", command) //nolint:gosec // intentional install command
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
func (m *Manager) installGitPackage(_ Package, gitCfg interface{}) (bool, string) {
	// Type assert to GitConfig
	cfg, ok := gitCfg.(GitConfig)
	if !ok {
		return false, "Git manager value is not GitConfig"
	}

	targetPath, ok := cfg.Targets[m.OS]
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
		// Already cloned, do git pull
		return m.gitPull(targetPath)
	}

	// Not cloned yet, do git clone
	return m.gitClone(cfg.URL, targetPath, cfg.Branch)
}

func (m *Manager) gitClone(repoURL, targetPath, branch string) (bool, string) {
	var cmd *exec.Cmd

	if branch != "" {
		cmd = exec.CommandContext(context.Background(), "git", "clone", "-b", branch, repoURL, targetPath)
	} else {
		cmd = exec.CommandContext(context.Background(), "git", "clone", repoURL, targetPath)
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

func (m *Manager) gitPull(repoPath string) (bool, string) {
	cmd := exec.CommandContext(context.Background(), "git", "-C", repoPath, "pull")

	if m.DryRun {
		return true, fmt.Sprintf("Would run: git -C %s pull", repoPath)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return false, fmt.Sprintf("Git pull failed: %v", err)
	}

	return true, "Repository updated successfully"
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

// FilterPackages returns packages that match the given filter context.
// It evaluates each package's Filters against the context (OS, distro,
// hostname, user) and returns only those that match.
func FilterPackages(packages []Package, ctx *config.FilterContext) []Package {
	result := make([]Package, 0, len(packages))

	for _, pkg := range packages {
		if config.MatchesFilters(pkg.Filters, ctx) {
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
// It returns the name of the first available package manager, "custom" if a
// custom command is available, "url" for URL-based installation, or "none"
// if no installation method is available.
func (m *Manager) GetInstallMethod(pkg Package) string {
	for _, mgr := range m.Available {
		if _, ok := pkg.Managers[mgr]; ok {
			return string(mgr)
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
func IsInstalled(pkgName string, manager string) bool {
	var cmd *exec.Cmd

	switch PackageManager(manager) {
	case Pacman, Yay, Paru:
		cmd = exec.CommandContext(context.Background(), "pacman", "-Q", pkgName)
	case Apt:
		cmd = exec.CommandContext(context.Background(), "dpkg", "-s", pkgName)
	case Dnf:
		cmd = exec.CommandContext(context.Background(), "rpm", "-q", pkgName)
	case Brew:
		cmd = exec.CommandContext(context.Background(), "brew", "list", pkgName)
	case Winget:
		cmd = exec.CommandContext(context.Background(), "winget", "list", "--id", pkgName)
	case Scoop:
		cmd = exec.CommandContext(context.Background(), "scoop", "info", pkgName)
	case Choco:
		cmd = exec.CommandContext(context.Background(), "choco", "list", "--local-only", pkgName)
	case Git:
		// For Git repos, we can't easily check installation status via this method
		return false
	default:
		// For custom/url methods, we can't easily check installation status
		return false
	}

	// Run silently - just check exit code
	cmd.Stdout = nil
	cmd.Stderr = nil
	err := cmd.Run()

	return err == nil
}

// FromEntry creates a Package from a config.Entry.
// It converts the entry's package configuration into a Package struct,
// mapping managers and URL install configurations. Returns nil if the
// entry does not have a package configuration.
func FromEntry(e config.Entry) *Package {
	if e.Package == nil {
		return nil
	}

	managers := make(map[PackageManager]interface{})
	for k, v := range e.Package.Managers {
		managers[PackageManager(k)] = v
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
		Filters:     e.Filters,
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
