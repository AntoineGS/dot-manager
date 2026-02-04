package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the main configuration structure
type Config struct {
	BackupRoot      string        `yaml:"backup_root"`
	DefaultManager  string        `yaml:"default_manager,omitempty"`
	Applications    []Application `yaml:"applications,omitempty"`
	ManagerPriority []string      `yaml:"manager_priority,omitempty"`
	Version         int           `yaml:"version"`
}

// URLInstallSpec defines URL-based installation
type URLInstallSpec struct {
	URL     string `yaml:"url"`
	Command string `yaml:"command"` // Use {file} as placeholder for downloaded file
}

// Load reads and parses the configuration file from the given path.
// It supports both v2 and v3 configuration formats, returning an error
// if the version is unsupported or if the file cannot be read or parsed.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path) //nolint:gosec // path is from user config, intentional
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if cfg.Version == 0 {
		cfg.Version = 3
	}

	if cfg.Version != 3 {
		return nil, fmt.Errorf("unsupported config version %d (expected 3)", cfg.Version)
	}

	return &cfg, nil
}

// ExpandPaths expands environment variables and tilde (~) in all path fields
// of the configuration, including backup paths, target paths, and file paths.
func (c *Config) ExpandPaths(envVars map[string]string) {
	c.BackupRoot = expandPath(c.BackupRoot, envVars)

	// Expand Applications
	for i := range c.Applications {
		for j := range c.Applications[i].Entries {
			c.Applications[i].Entries[j].Backup = expandPath(c.Applications[i].Entries[j].Backup, envVars)
			for k, v := range c.Applications[i].Entries[j].Targets {
				c.Applications[i].Entries[j].Targets[k] = expandPath(v, envVars)
			}

			for k := range c.Applications[i].Entries[j].Files {
				c.Applications[i].Entries[j].Files[k] = expandPath(c.Applications[i].Entries[j].Files[k], envVars)
			}
		}
	}
}

// GetFilteredApplications returns applications filtered by filter context
func (c *Config) GetFilteredApplications(ctx *FilterContext) []Application {
	result := make([]Application, 0, len(c.Applications))

	for _, app := range c.Applications {
		if MatchesFilters(app.Filters, ctx) {
			result = append(result, app)
		}
	}

	return result
}

// GetAllSubEntries returns all sub-entries from all applications filtered by context
func (c *Config) GetAllSubEntries(ctx *FilterContext) []SubEntry {
	apps := c.GetFilteredApplications(ctx)
	// Estimate capacity based on average entries per app
	estimatedCap := len(apps) * 5

	result := make([]SubEntry, 0, estimatedCap)
	for _, app := range apps {
		result = append(result, app.Entries...)
	}

	return result
}

// GetAllConfigSubEntries returns only config type sub-entries from filtered applications
func (c *Config) GetAllConfigSubEntries(ctx *FilterContext) []SubEntry {
	apps := c.GetFilteredApplications(ctx)
	// Estimate capacity based on average entries per app
	estimatedCap := len(apps) * 3
	result := make([]SubEntry, 0, estimatedCap)

	for _, app := range apps {
		for _, entry := range app.Entries {
			if entry.IsConfig() {
				result = append(result, entry)
			}
		}
	}

	return result
}

// GetFilteredPackages returns all packages from filtered applications as pseudo-Entry objects
func (c *Config) GetFilteredPackages(ctx *FilterContext) []Entry {
	apps := c.GetFilteredApplications(ctx)
	result := make([]Entry, 0, len(apps))

	for _, app := range apps {
		if app.HasPackage() {
			// Create a pseudo-entry for the package
			entry := Entry{
				Name:        app.Name,
				Description: app.Description,
				Package:     app.Package,
				Filters:     app.Filters,
			}
			result = append(result, entry)
		}
	}

	return result
}

// ExpandPath expands ~ and environment variables in a single path.
// This should be used when a path is needed for file operations.
// The path is kept unexpanded in the config to maintain portability.
func ExpandPath(path string, envVars map[string]string) string {
	if path == "" {
		return path
	}

	// Expand ~ to home directory
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, path[2:])
		}
	} else if path == "~" {
		home, err := os.UserHomeDir()
		if err == nil {
			path = home
		}
	}

	// Expand environment variables from the provided map
	for key, value := range envVars {
		path = strings.ReplaceAll(path, "$"+key, value)
	}

	// Also expand standard environment variables
	path = os.ExpandEnv(path)

	return path
}

// expandPath is a private wrapper that calls ExpandPath for internal use
func expandPath(path string, envVars map[string]string) string {
	return ExpandPath(path, envVars)
}

// Save writes the config to the specified file path
func Save(cfg *Config, path string) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Use 0600 permissions to restrict access to owner only,
	// as config may contain sensitive path information
	if err := os.WriteFile(path, data, 0600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}
