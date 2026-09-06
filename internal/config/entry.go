package config

import (
	"fmt"

	"gopkg.in/yaml.v3"
)

// Deployment methods for a config SubEntry.
const (
	// MethodSymlink symlinks each file from the repo to its target (default).
	MethodSymlink = "symlink"
	// MethodCopy writes a real copy of each file at its target.
	MethodCopy = "copy"
)

// Setup check modes for a setup SubEntry.
const (
	// CheckModeExitCode interprets a zero check exit code as already set up.
	CheckModeExitCode = "exit-code"
	// CheckModeStatus interprets check exit codes as setup status values.
	CheckModeStatus = "status"
)

// managerGit is the managers-map key whose value is a GitPackage object rather
// than a plain package name.
const managerGit = "git"

// ManagerValue represents a typed value for a package manager entry.
// It holds either a package name string (for traditional managers like pacman, apt),
// a GitPackage configuration (for git repositories), or an InstallerPackage
// configuration (for shell command-based installation).
type ManagerValue struct {
	PackageName string
	Git         *GitPackage
	Installer   *InstallerPackage
	Deps        []string
}

// IsGit returns true if this manager value represents a git package configuration.
func (v ManagerValue) IsGit() bool { return v.Git != nil }

// IsInstaller returns true if this manager value represents an installer package configuration.
func (v ManagerValue) IsInstaller() bool { return v.Installer != nil }

// MarshalYAML writes non-git/non-installer manager values as plain strings
// when no deps exist, or as an object with name/deps when deps are present.
func (v ManagerValue) MarshalYAML() (any, error) {
	if v.IsGit() {
		return v.Git, nil
	}

	if v.IsInstaller() {
		return v.Installer, nil
	}

	// Collapse to plain string when no deps
	if len(v.Deps) == 0 {
		return v.PackageName, nil
	}

	// Object form with name and deps
	result := map[string]any{}
	if v.PackageName != "" {
		result["name"] = v.PackageName
	}
	result["deps"] = v.Deps

	return result, nil
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

// unmarshalGitManager converts a raw any value into a ManagerValue with a GitPackage.
func unmarshalGitManager(value any) (ManagerValue, error) {
	gitMap, ok := value.(map[string]any)
	if !ok {
		return ManagerValue{}, fmt.Errorf("git manager must be an object, got %T", value)
	}

	gitBytes, err := yaml.Marshal(gitMap)
	if err != nil {
		return ManagerValue{}, fmt.Errorf("marshaling git config: %w", err)
	}

	var gitPkg GitPackage
	if err := yaml.Unmarshal(gitBytes, &gitPkg); err != nil {
		return ManagerValue{}, fmt.Errorf("unmarshaling git config: %w", err)
	}

	return ManagerValue{Git: &gitPkg}, nil
}

// unmarshalInstallerManager converts a raw any value into a ManagerValue with an InstallerPackage.
func unmarshalInstallerManager(value any) (ManagerValue, error) {
	installerMap, ok := value.(map[string]any)
	if !ok {
		return ManagerValue{}, fmt.Errorf("installer manager must be an object, got %T", value)
	}

	installerBytes, err := yaml.Marshal(installerMap)
	if err != nil {
		return ManagerValue{}, fmt.Errorf("marshaling installer config: %w", err)
	}

	var installerPkg InstallerPackage
	if err := yaml.Unmarshal(installerBytes, &installerPkg); err != nil {
		return ManagerValue{}, fmt.Errorf("unmarshaling installer config: %w", err)
	}

	return ManagerValue{Installer: &installerPkg}, nil
}

// unmarshalNativeManager converts a raw any value into a ManagerValue for a standard
// package manager. It supports both plain string format and object format with name/deps.
func unmarshalNativeManager(key string, value any) (ManagerValue, error) {
	// Try string first (backward compat)
	str, ok := value.(string)
	if ok {
		return ManagerValue{PackageName: str}, nil
	}

	// Try object with name/deps
	objMap, ok := value.(map[string]any)
	if !ok {
		return ManagerValue{}, fmt.Errorf("manager %s must be a string or object, got %T", key, value)
	}

	var mv ManagerValue
	if name, ok := objMap["name"]; ok {
		if nameStr, ok := name.(string); ok {
			mv.PackageName = nameStr
		}
	}

	if deps, ok := objMap["deps"]; ok {
		depsSlice, ok := deps.([]any)
		if !ok {
			return ManagerValue{}, fmt.Errorf("manager %s deps must be a list, got %T", key, deps)
		}

		for _, d := range depsSlice {
			if dStr, ok := d.(string); ok {
				mv.Deps = append(mv.Deps, dStr)
			}
		}
	}

	return mv, nil
}

// UnmarshalYAML implements custom YAML unmarshaling for EntryPackage
// to properly handle git manager objects while keeping other managers as strings
func (ep *EntryPackage) UnmarshalYAML(node *yaml.Node) error {
	// Create a temporary struct with the same fields but using any for Managers
	type rawPackage struct {
		Managers map[string]any            `yaml:"managers,omitempty"`
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
			var (
				mv  ManagerValue
				err error
			)

			switch key {
			case managerGit:
				mv, err = unmarshalGitManager(value)
			case "installer":
				mv, err = unmarshalInstallerManager(value)
			default:
				mv, err = unmarshalNativeManager(key, value)
			}

			if err != nil {
				return err
			}

			ep.Managers[key] = mv
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

	value, ok := ep.Managers[managerGit]
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

// SubEntry represents an individual configuration entry within an application.
// A sub-entry is either a config entry (it has a Backup) or a setup entry (it
// has a Run command). Never both — see validateSetupEntry.
type SubEntry struct {
	Targets map[string]string `yaml:"targets,omitempty"`
	Check   map[string]string `yaml:"check,omitempty"` // os -> command; exit 0 means already set up
	Run     map[string]string `yaml:"run,omitempty"`   // os -> command; runs only when check fails
	Name    string            `yaml:"name"`
	When    string            `yaml:"when,omitempty"`
	Method  string            `yaml:"method,omitempty"` // "" | "symlink" (default) | "copy"
	// CheckMode selects how a setup entry interprets check command exit codes.
	CheckMode string   `yaml:"check_mode,omitempty"`
	Backup    string   `yaml:"backup,omitempty"`
	Files     []string `yaml:"files,omitempty"`
	Sudo      bool     `yaml:"sudo,omitempty"`
}

// IsConfig returns true if this is a config type sub-entry
func (s *SubEntry) IsConfig() bool {
	return s.Backup != ""
}

// IsSetup returns true if this is a setup sub-entry: one that runs a command to
// bring the system into a desired state, rather than deploying config files.
func (s *SubEntry) IsSetup() bool {
	return len(s.Run) > 0
}

// GetCheck returns the check command for the specified OS, or "" if the entry
// declares none. An empty result means the setup step does not apply to this OS.
func (s *SubEntry) GetCheck(osType string) string {
	return s.Check[osType]
}

// GetRun returns the setup command for the specified OS, or "" if the entry
// declares none. An empty result means the setup step does not apply to this OS.
func (s *SubEntry) GetRun(osType string) string {
	return s.Run[osType]
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

// EffectiveMethod returns the deployment method, defaulting to symlink.
func (s *SubEntry) EffectiveMethod() string {
	if s.Method == "" {
		return MethodSymlink
	}
	return s.Method
}

// IsCopy reports whether the entry deploys by copying rather than symlinking.
func (s *SubEntry) IsCopy() bool { return s.EffectiveMethod() == MethodCopy }

// HasPackage returns true if the application has package installation configuration
func (a *Application) HasPackage() bool {
	return a.Package != nil
}
