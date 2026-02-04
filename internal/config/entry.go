package config

const (
	// SubEntryTypeConfig represents a config type sub-entry
	SubEntryTypeConfig = "config"
)

// Entry is a unified configuration entry that manages symlink configuration.
// Entries can optionally have a package field for installation.
type Entry struct {
	Targets     map[string]string `yaml:"targets,omitempty"`
	Package     *EntryPackage     `yaml:"package,omitempty"`
	Name        string            `yaml:"name"`
	Description string            `yaml:"description,omitempty"`
	Backup      string            `yaml:"backup,omitempty"`
	Filters     []Filter          `yaml:"filters,omitempty"`
	Files       []string          `yaml:"files,omitempty"`
	Sudo        bool              `yaml:"sudo,omitempty"`
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
// An application has a name, optional description, filters, and contains multiple sub-entries.
// It can also have an associated package for installation.
type Application struct {
	Package     *EntryPackage `yaml:"package,omitempty"`
	Name        string        `yaml:"name"`
	Description string        `yaml:"description,omitempty"`
	Filters     []Filter      `yaml:"filters,omitempty"`
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
