package packages

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

// PackageManager represents a supported package manager
type PackageManager string

const (
	Pacman PackageManager = "pacman"
	Yay    PackageManager = "yay"
	Paru   PackageManager = "paru"
	Apt    PackageManager = "apt"
	Dnf    PackageManager = "dnf"
	Brew   PackageManager = "brew"
	Winget PackageManager = "winget"
	Scoop  PackageManager = "scoop"
	Choco  PackageManager = "choco"
)

// Package represents a package to install
type Package struct {
	Name        string                    `yaml:"name"`
	Description string                    `yaml:"description,omitempty"`
	Managers    map[PackageManager]string `yaml:"managers,omitempty"`
	Custom      map[string]string         `yaml:"custom,omitempty"` // OS -> command
	URL         map[string]URLInstall     `yaml:"url,omitempty"`    // OS -> URL install
	Filters     []config.Filter           `yaml:"filters,omitempty"`
}

// URLInstall represents installation from a URL
type URLInstall struct {
	URL     string `yaml:"url"`
	Command string `yaml:"command"` // Command to run after download, use {file} as placeholder
}

// PackagesConfig holds the packages configuration
type PackagesConfig struct {
	Packages        []Package        `yaml:"packages"`
	DefaultManager  PackageManager   `yaml:"default_manager,omitempty"`
	ManagerPriority []PackageManager `yaml:"manager_priority,omitempty"`
}

// Manager handles package installation
type Manager struct {
	Config    *PackagesConfig
	OS        string
	DryRun    bool
	Verbose   bool
	Available []PackageManager
	Preferred PackageManager
}

// NewManager creates a new package manager
func NewManager(cfg *PackagesConfig, osType string, dryRun, verbose bool) *Manager {
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
	if m.OS == "windows" {
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

// HasManager checks if a package manager is available
func (m *Manager) HasManager(mgr PackageManager) bool {
	for _, available := range m.Available {
		if available == mgr {
			return true
		}
	}
	return false
}

// InstallResult represents the result of an installation
type InstallResult struct {
	Package string
	Success bool
	Message string
	Method  string // e.g., "pacman", "custom", "url"
}

// Install installs a single package
func (m *Manager) Install(pkg Package) InstallResult {
	result := InstallResult{Package: pkg.Name}

	// Try package managers first
	if len(pkg.Managers) > 0 {
		for _, mgr := range m.Available {
			if pkgName, ok := pkg.Managers[mgr]; ok {
				result.Method = string(mgr)
				success, msg := m.installWithManager(mgr, pkgName)
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
		cmd = exec.Command("sudo", "pacman", "-S", "--noconfirm", pkgName)
	case Yay:
		cmd = exec.Command("yay", "-S", "--noconfirm", pkgName)
	case Paru:
		cmd = exec.Command("paru", "-S", "--noconfirm", pkgName)
	case Apt:
		cmd = exec.Command("sudo", "apt-get", "install", "-y", pkgName)
	case Dnf:
		cmd = exec.Command("sudo", "dnf", "install", "-y", pkgName)
	case Brew:
		cmd = exec.Command("brew", "install", pkgName)
	case Winget:
		cmd = exec.Command("winget", "install", "--accept-package-agreements", "--accept-source-agreements", pkgName)
	case Scoop:
		cmd = exec.Command("scoop", "install", pkgName)
	case Choco:
		cmd = exec.Command("choco", "install", "-y", pkgName)
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
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", "-Command", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
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
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Download file
	var downloadCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Escape single quotes in URL and path for PowerShell
		escapedURL := strings.ReplaceAll(urlInstall.URL, "'", "''")
		escapedPath := strings.ReplaceAll(tmpPath, "'", "''")
		downloadCmd = exec.Command("powershell", "-Command",
			fmt.Sprintf("Invoke-WebRequest -Uri '%s' -OutFile '%s'", escapedURL, escapedPath))
	} else {
		downloadCmd = exec.Command("curl", "-fsSL", "-o", tmpPath, urlInstall.URL)
	}

	if err := downloadCmd.Run(); err != nil {
		return false, fmt.Sprintf("Download failed: %v", err)
	}

	// Make executable on Unix
	if runtime.GOOS != "windows" {
		os.Chmod(tmpPath, 0755)
	}

	// Run install command
	command := strings.ReplaceAll(urlInstall.Command, "{file}", tmpPath)

	var installCmd *exec.Cmd
	if runtime.GOOS == "windows" {
		installCmd = exec.Command("powershell", "-Command", command)
	} else {
		installCmd = exec.Command("sh", "-c", command)
	}

	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	installCmd.Stdin = os.Stdin

	if err := installCmd.Run(); err != nil {
		return false, fmt.Sprintf("Install command failed: %v", err)
	}

	return true, "Installed via URL"
}

// InstallAll installs all packages
func (m *Manager) InstallAll(packages []Package) []InstallResult {
	var results []InstallResult
	for _, pkg := range packages {
		results = append(results, m.Install(pkg))
	}
	return results
}

// FilterPackages returns packages that match the given filter context
func FilterPackages(packages []Package, ctx *config.FilterContext) []Package {
	var result []Package
	for _, pkg := range packages {
		if config.MatchesFilters(pkg.Filters, ctx) {
			result = append(result, pkg)
		}
	}
	return result
}

// CanInstall checks if a package can be installed on this system
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

// GetInstallablePackages returns packages that can be installed on this system
func (m *Manager) GetInstallablePackages() []Package {
	var result []Package
	for _, pkg := range m.Config.Packages {
		if m.CanInstall(pkg) {
			result = append(result, pkg)
		}
	}
	return result
}

// GetInstallMethod returns the method that would be used to install a package
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

// FromEntry creates a Package from a config.Entry
func FromEntry(e config.Entry) *Package {
	if e.Package == nil {
		return nil
	}

	managers := make(map[PackageManager]string)
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

// FromEntries creates a slice of Packages from a slice of config.Entry
func FromEntries(entries []config.Entry) []Package {
	var result []Package
	for _, e := range entries {
		if pkg := FromEntry(e); pkg != nil {
			result = append(result, *pkg)
		}
	}
	return result
}
