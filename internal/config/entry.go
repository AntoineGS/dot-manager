package config

// Entry is a unified configuration entry that can be:
// - Config type (has backup field): symlink management
// - Git type (has repo field): repository clones
// Both types can optionally have a package field.
type Entry struct {
	// Common fields
	Name        string   `yaml:"name"`
	Description string   `yaml:"description,omitempty"`
	Filters     []Filter `yaml:"filters,omitempty"`
	Root        bool     `yaml:"root,omitempty"`

	// Config fields (identifies config type entries)
	Files   []string          `yaml:"files,omitempty"`
	Backup  string            `yaml:"backup,omitempty"`
	Targets map[string]string `yaml:"targets,omitempty"`

	// Git fields (identifies git type entries)
	Repo   string `yaml:"repo,omitempty"`
	Branch string `yaml:"branch,omitempty"`

	// Package fields (nested - optional for both types)
	Package *EntryPackage `yaml:"package,omitempty"`
}

// EntryPackage contains package installation configuration
type EntryPackage struct {
	Managers map[string]string         `yaml:"managers,omitempty"` // manager -> package name
	Custom   map[string]string         `yaml:"custom,omitempty"`   // os -> command
	URL      map[string]URLInstallSpec `yaml:"url,omitempty"`      // os -> url install
}

// IsConfig returns true if this is a config type entry (has backup field)
func (e *Entry) IsConfig() bool {
	return e.Backup != ""
}

// IsGit returns true if this is a git type entry (has repo field)
func (e *Entry) IsGit() bool {
	return e.Repo != ""
}

// HasConfig returns true if this entry has configuration management (backup/targets)
// Deprecated: Use IsConfig() instead
func (e *Entry) HasConfig() bool {
	return e.IsConfig()
}

// HasPackage returns true if this entry has package installation configuration
func (e *Entry) HasPackage() bool {
	return e.Package != nil
}

// IsFolder returns true if this config entry manages an entire folder (no specific files)
func (e *Entry) IsFolder() bool {
	return e.HasConfig() && len(e.Files) == 0
}

// GetTarget returns the target path for the specified OS
func (e *Entry) GetTarget(osType string) string {
	if target, ok := e.Targets[osType]; ok {
		return target
	}
	return ""
}

// ToPathSpec converts the Entry to a PathSpec for backward compatibility
func (e *Entry) ToPathSpec() PathSpec {
	return PathSpec{
		Name:    e.Name,
		Files:   e.Files,
		Backup:  e.Backup,
		Targets: e.Targets,
	}
}

// ToPackageSpec converts the Entry to a PackageSpec for backward compatibility
func (e *Entry) ToPackageSpec() PackageSpec {
	if e.Package == nil {
		return PackageSpec{
			Name:        e.Name,
			Description: e.Description,
			Filters:     e.Filters,
		}
	}
	return PackageSpec{
		Name:        e.Name,
		Description: e.Description,
		Managers:    e.Package.Managers,
		Custom:      e.Package.Custom,
		URL:         e.Package.URL,
		Filters:     e.Filters,
	}
}

// EntryFromPathSpec creates an Entry from a PathSpec
func EntryFromPathSpec(p PathSpec, root bool) Entry {
	return Entry{
		Name:    p.Name,
		Root:    root,
		Files:   p.Files,
		Backup:  p.Backup,
		Targets: p.Targets,
	}
}

// EntryFromPackageSpec creates an Entry from a PackageSpec
func EntryFromPackageSpec(p PackageSpec) Entry {
	var pkg *EntryPackage
	if len(p.Managers) > 0 || len(p.Custom) > 0 || len(p.URL) > 0 {
		pkg = &EntryPackage{
			Managers: p.Managers,
			Custom:   p.Custom,
			URL:      p.URL,
		}
	}
	return Entry{
		Name:        p.Name,
		Description: p.Description,
		Filters:     p.Filters,
		Package:     pkg,
	}
}
