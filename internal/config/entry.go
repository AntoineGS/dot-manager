package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Entry is a unified configuration entry that manages symlink configuration.
// Entries can optionally have a package field for installation.
type Entry struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description,omitempty"`
	Targets     map[string]string `yaml:"targets,omitempty"`
	Package     *EntryPackage     `yaml:"package,omitempty"`
	Backup      string            `yaml:"backup,omitempty"`
	When        string            `yaml:"when,omitempty"`
	Files       []string          `yaml:"files,omitempty"`
	Sudo        bool              `yaml:"sudo,omitempty"`
}

// ManagerValue represents a typed value for a package manager entry.
// It holds either a package name string (for traditional managers like pacman, apt),
// a GitPackage configuration (for git repositories), or an InstallerPackage
// configuration (for shell command-based installation).
type ManagerValue struct {
	PackageName string
	Git         *GitPackage
	Installer   *InstallerPackage
}

// IsGit returns true if this manager value represents a git package configuration.
func (v ManagerValue) IsGit() bool { return v.Git != nil }

// IsInstaller returns true if this manager value represents an installer package configuration.
func (v ManagerValue) IsInstaller() bool { return v.Installer != nil }

// MarshalYAML writes non-git/non-installer manager values as plain strings
func (v ManagerValue) MarshalYAML() (interface{}, error) {
	if v.IsGit() {
		return v.Git, nil
	}

	if v.IsInstaller() {
		return v.Installer, nil
	}

	return v.PackageName, nil
}

// EntryPackage contains package installation configuration
type EntryPackage struct {
	Managers map[string]ManagerValue   `yaml:"managers,omitempty"` // manager -> package name or GitPackage
	Custom   map[string]string         `yaml:"custom,omitempty"`   // os -> command
	URL      map[string]URLInstallSpec `yaml:"url,omitempty"`      // os -> url install
}

// GitPackage represents a git repository package configuration
type GitPackage struct {
	URL     string            `yaml:"url"`
	Branch  string            `yaml:"branch,omitempty"`
	Targets map[string]string `yaml:"targets"`
	Sudo    bool              `yaml:"sudo,omitempty"`
}

// InstallerPackage represents a shell command-based package installation configuration.
// Command is an OS-specific map of shell commands to run for installation.
// Binary is an optional name used to check if the software is already installed via PATH lookup.
type InstallerPackage struct {
	Command map[string]string `yaml:"command"`
	Binary  string            `yaml:"binary,omitempty"`
}

// UnmarshalYAML implements custom YAML unmarshaling for EntryPackage
// to properly handle git manager objects while keeping other managers as strings
func (ep *EntryPackage) UnmarshalYAML(node *yaml.Node) error {
	// Create a temporary struct with the same fields but using interface{} for Managers
	type rawPackage struct {
		Managers map[string]interface{}    `yaml:"managers,omitempty"`
		Custom   map[string]string         `yaml:"custom,omitempty"`
		URL      map[string]URLInstallSpec `yaml:"url,omitempty"`
	}

	var raw rawPackage
	if err := node.Decode(&raw); err != nil {
		return err
	}

	// Process managers to convert git entries to GitPackage
	if raw.Managers != nil {
		ep.Managers = make(map[string]ManagerValue, len(raw.Managers))
		for key, value := range raw.Managers {
			switch key {
			case "git":
				// Convert the map to GitPackage
				gitMap, ok := value.(map[string]interface{})
				if !ok {
					return fmt.Errorf("git manager must be an object, got %T", value)
				}

				gitBytes, err := yaml.Marshal(gitMap)
				if err != nil {
					return fmt.Errorf("marshaling git config: %w", err)
				}

				var gitPkg GitPackage
				if err := yaml.Unmarshal(gitBytes, &gitPkg); err != nil {
					return fmt.Errorf("unmarshaling git config: %w", err)
				}

				ep.Managers[key] = ManagerValue{Git: &gitPkg}

			case "installer":
				// Convert the map to InstallerPackage
				installerMap, ok := value.(map[string]interface{})
				if !ok {
					return fmt.Errorf("installer manager must be an object, got %T", value)
				}

				installerBytes, err := yaml.Marshal(installerMap)
				if err != nil {
					return fmt.Errorf("marshaling installer config: %w", err)
				}

				var installerPkg InstallerPackage
				if err := yaml.Unmarshal(installerBytes, &installerPkg); err != nil {
					return fmt.Errorf("unmarshaling installer config: %w", err)
				}

				ep.Managers[key] = ManagerValue{Installer: &installerPkg}

			default:
				// Traditional managers are strings
				str, ok := value.(string)
				if !ok {
					return fmt.Errorf("manager %s must be a string, got %T", key, value)
				}
				ep.Managers[key] = ManagerValue{PackageName: str}
			}
		}
	}

	ep.Custom = raw.Custom
	ep.URL = raw.URL

	return nil
}

// GetManagerString returns the manager value as a string, or empty string if not found or not a string
func (ep *EntryPackage) GetManagerString(manager string) (string, bool) {
	if ep.Managers == nil {
		return "", false
	}

	value, ok := ep.Managers[manager]
	if !ok || value.IsGit() || value.IsInstaller() {
		return "", false
	}

	return value.PackageName, true
}

// GetGitPackage returns the git manager configuration, or nil if not found or not a GitPackage
func (ep *EntryPackage) GetGitPackage() (*GitPackage, bool) {
	if ep.Managers == nil {
		return nil, false
	}

	value, ok := ep.Managers["git"]
	if !ok || value.Git == nil {
		return nil, false
	}

	return value.Git, true
}

// GetInstallerPackage returns the installer manager configuration, or nil if not found
func (ep *EntryPackage) GetInstallerPackage() (*InstallerPackage, bool) {
	if ep.Managers == nil {
		return nil, false
	}

	value, ok := ep.Managers["installer"]
	if !ok || value.Installer == nil {
		return nil, false
	}

	return value.Installer, true
}

// IsConfig returns true if this is a config type entry (has backup field)
func (e *Entry) IsConfig() bool {
	return e.Backup != ""
}

// HasPackage returns true if this entry has package installation configuration
func (e *Entry) HasPackage() bool {
	return e.Package != nil
}

// IsFolder returns true if this config entry manages an entire folder (no specific files)
func (e *Entry) IsFolder() bool {
	return e.IsConfig() && len(e.Files) == 0
}

// GetTarget returns the target path for the specified OS
func (e *Entry) GetTarget(osType string) string {
	if target, ok := e.Targets[osType]; ok {
		return target
	}

	return ""
}

// Application represents a logical grouping of configuration entries
// An application has a name, optional description, when condition, and contains multiple sub-entries.
// It can also have an associated package for installation.
type Application struct {
	Package     *EntryPackage `yaml:"package,omitempty"`
	Name        string        `yaml:"name"`
	Description string        `yaml:"description,omitempty"`
	When        string        `yaml:"when,omitempty"`
	Entries     []SubEntry    `yaml:"entries"`
}

// SubEntry represents an individual configuration entry within an application
type SubEntry struct {
	Targets map[string]string `yaml:"targets,omitempty"`
	Name    string            `yaml:"name"`
	Backup  string            `yaml:"backup,omitempty"`
	Files   []string          `yaml:"files,omitempty"`
	Sudo    bool              `yaml:"sudo,omitempty"`
}

// IsConfig returns true if this is a config type sub-entry
func (s *SubEntry) IsConfig() bool {
	return s.Backup != ""
}

// IsFolder returns true if this config sub-entry manages an entire folder (no specific files)
func (s *SubEntry) IsFolder() bool {
	return s.IsConfig() && len(s.Files) == 0
}

// GetTarget returns the target path for the specified OS
func (s *SubEntry) GetTarget(osType string) string {
	if target, ok := s.Targets[osType]; ok {
		return target
	}

	return ""
}

// HasPackage returns true if the application has package installation configuration
func (a *Application) HasPackage() bool {
	return a.Package != nil
}
