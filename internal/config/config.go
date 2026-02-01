package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Version         int              `yaml:"version"`
	BackupRoot      string           `yaml:"backup_root"`
	Paths           []PathSpec       `yaml:"paths"`
	RootPaths       []PathSpec       `yaml:"root_paths"`
	Hooks           Hooks            `yaml:"hooks"`
	Packages        PackagesConfig   `yaml:"packages"`
}

// PackagesConfig holds package installation configuration
type PackagesConfig struct {
	DefaultManager  string           `yaml:"default_manager,omitempty"`
	ManagerPriority []string         `yaml:"manager_priority,omitempty"`
	Items           []PackageSpec    `yaml:"items"`
}

// PackageSpec defines a package to install
type PackageSpec struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description,omitempty"`
	Managers    map[string]string `yaml:"managers,omitempty"`  // manager -> package name
	Custom      map[string]string `yaml:"custom,omitempty"`    // os -> command
	URL         map[string]URLInstallSpec `yaml:"url,omitempty"` // os -> url install
	Tags        []string          `yaml:"tags,omitempty"`
}

// URLInstallSpec defines URL-based installation
type URLInstallSpec struct {
	URL     string `yaml:"url"`
	Command string `yaml:"command"` // Use {file} as placeholder for downloaded file
}

type PathSpec struct {
	Name    string            `yaml:"name"`
	Files   []string          `yaml:"files"`
	Backup  string            `yaml:"backup"`
	Targets map[string]string `yaml:"targets"`
}

type Hooks struct {
	PostRestore map[string][]Hook `yaml:"post_restore"`
}

type Hook struct {
	Type       string      `yaml:"type"`
	SkipOnArch bool        `yaml:"skip_on_arch"`
	Plugins    []Plugin    `yaml:"plugins,omitempty"`
	Source     string      `yaml:"source,omitempty"`
	FzfSymlinks []FzfSymlink `yaml:"fzf_symlinks,omitempty"`
}

type Plugin struct {
	Name string `yaml:"name"`
	Repo string `yaml:"repo"`
	Path string `yaml:"path"`
}

type FzfSymlink struct {
	Target string `yaml:"target"`
	Link   string `yaml:"link"`
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
		cfg.Version = 1
	}

	return &cfg, nil
}

func (c *Config) ExpandPaths(envVars map[string]string) {
	c.BackupRoot = expandPath(c.BackupRoot, envVars)

	for i := range c.Paths {
		c.Paths[i].Backup = expandPath(c.Paths[i].Backup, envVars)
		for k, v := range c.Paths[i].Targets {
			c.Paths[i].Targets[k] = expandPath(v, envVars)
		}
		for j := range c.Paths[i].Files {
			c.Paths[i].Files[j] = expandPath(c.Paths[i].Files[j], envVars)
		}
	}

	for i := range c.RootPaths {
		c.RootPaths[i].Backup = expandPath(c.RootPaths[i].Backup, envVars)
		for k, v := range c.RootPaths[i].Targets {
			c.RootPaths[i].Targets[k] = expandPath(v, envVars)
		}
		for j := range c.RootPaths[i].Files {
			c.RootPaths[i].Files[j] = expandPath(c.RootPaths[i].Files[j], envVars)
		}
	}

	for os, hooks := range c.Hooks.PostRestore {
		for i := range hooks {
			hooks[i].Source = expandPath(hooks[i].Source, envVars)
			for j := range hooks[i].Plugins {
				hooks[i].Plugins[j].Path = expandPath(hooks[i].Plugins[j].Path, envVars)
			}
		}
		c.Hooks.PostRestore[os] = hooks
	}
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
