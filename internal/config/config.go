package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the main configuration structure (v2 format)
type Config struct {
	Version    int     `yaml:"version"`
	BackupRoot string  `yaml:"backup_root"`
	Entries    []Entry `yaml:"entries"`

	// Package manager configuration
	DefaultManager  string   `yaml:"default_manager,omitempty"`
	ManagerPriority []string `yaml:"manager_priority,omitempty"`
}

// PackageSpec defines a package to install (for backward compatibility)
type PackageSpec struct {
	Name        string                    `yaml:"name"`
	Description string                    `yaml:"description,omitempty"`
	Managers    map[string]string         `yaml:"managers,omitempty"` // manager -> package name
	Custom      map[string]string         `yaml:"custom,omitempty"`   // os -> command
	URL         map[string]URLInstallSpec `yaml:"url,omitempty"`      // os -> url install
	Filters     []Filter                  `yaml:"filters,omitempty"`
}

// URLInstallSpec defines URL-based installation
type URLInstallSpec struct {
	URL     string `yaml:"url"`
	Command string `yaml:"command"` // Use {file} as placeholder for downloaded file
}

// PathSpec defines a path configuration (for backward compatibility)
type PathSpec struct {
	Name    string            `yaml:"name"`
	Files   []string          `yaml:"files"`
	Backup  string            `yaml:"backup"`
	Targets map[string]string `yaml:"targets"`
}


func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config file: %w", err)
	}

	if cfg.Version == 0 {
		cfg.Version = 2
	}

	if cfg.Version != 2 {
		return nil, fmt.Errorf("unsupported config version %d (expected 2)", cfg.Version)
	}

	return &cfg, nil
}

func (c *Config) ExpandPaths(envVars map[string]string) {
	c.BackupRoot = expandPath(c.BackupRoot, envVars)

	for i := range c.Entries {
		c.Entries[i].Backup = expandPath(c.Entries[i].Backup, envVars)
		for k, v := range c.Entries[i].Targets {
			c.Entries[i].Targets[k] = expandPath(v, envVars)
		}
		for j := range c.Entries[i].Files {
			c.Entries[i].Files[j] = expandPath(c.Entries[i].Files[j], envVars)
		}
	}
}

// GetConfigEntries returns entries that are config type (have backup) filtered by root flag
func (c *Config) GetConfigEntries(isRoot bool) []Entry {
	var result []Entry
	for _, e := range c.Entries {
		if e.IsConfig() && e.Root == isRoot {
			result = append(result, e)
		}
	}
	return result
}

// GetFilteredConfigEntries returns config entries filtered by root flag and filter context
func (c *Config) GetFilteredConfigEntries(isRoot bool, ctx *FilterContext) []Entry {
	var result []Entry
	for _, e := range c.Entries {
		if e.IsConfig() && e.Root == isRoot && MatchesFilters(e.Filters, ctx) {
			result = append(result, e)
		}
	}
	return result
}

// GetGitEntries returns entries that are git type (have repo) filtered by root flag
func (c *Config) GetGitEntries(isRoot bool) []Entry {
	var result []Entry
	for _, e := range c.Entries {
		if e.IsGit() && e.Root == isRoot {
			result = append(result, e)
		}
	}
	return result
}

// GetFilteredGitEntries returns git entries filtered by root flag and filter context
func (c *Config) GetFilteredGitEntries(isRoot bool, ctx *FilterContext) []Entry {
	var result []Entry
	for _, e := range c.Entries {
		if e.IsGit() && e.Root == isRoot && MatchesFilters(e.Filters, ctx) {
			result = append(result, e)
		}
	}
	return result
}

// GetPackageEntries returns entries that have package configuration
func (c *Config) GetPackageEntries() []Entry {
	var result []Entry
	for _, e := range c.Entries {
		if e.HasPackage() {
			result = append(result, e)
		}
	}
	return result
}

// GetFilteredPackageEntries returns package entries filtered by filter context
func (c *Config) GetFilteredPackageEntries(ctx *FilterContext) []Entry {
	var result []Entry
	for _, e := range c.Entries {
		if e.HasPackage() && MatchesFilters(e.Filters, ctx) {
			result = append(result, e)
		}
	}
	return result
}

// GetPaths returns PathSpecs for entries with config (for backward compatibility)
func (c *Config) GetPaths(isRoot bool) []PathSpec {
	entries := c.GetConfigEntries(isRoot)
	result := make([]PathSpec, 0, len(entries))
	for _, e := range entries {
		result = append(result, e.ToPathSpec())
	}
	return result
}

// GetPackageSpecs returns PackageSpecs for entries with packages (for backward compatibility)
func (c *Config) GetPackageSpecs() []PackageSpec {
	entries := c.GetPackageEntries()
	result := make([]PackageSpec, 0, len(entries))
	for _, e := range entries {
		result = append(result, e.ToPackageSpec())
	}
	return result
}

func expandPath(path string, envVars map[string]string) string {
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

func (p *PathSpec) IsFolder() bool {
	return len(p.Files) == 0
}

func (p *PathSpec) GetTarget(osType string) string {
	if target, ok := p.Targets[osType]; ok {
		return target
	}
	return ""
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
