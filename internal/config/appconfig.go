// Package config provides configuration management for dot-manager.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// AppConfig is the minimal configuration stored in ~/.config/dot-manager/
// It only contains the path to the configurations repository
type AppConfig struct {
	// ConfigDir is the path to the configurations repository
	ConfigDir string `yaml:"config_dir"`
}

const (
	appConfigDir   = ".config/dot-manager"
	appConfigFile  = "config.yaml"
	repoConfigFile = "dot-manager.yaml"
)

// LoadAppConfig loads the app configuration from ~/.config/dot-manager/config.yaml
func LoadAppConfig() (*AppConfig, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("getting home directory: %w", err)
	}

	configPath := filepath.Join(home, appConfigDir, appConfigFile)

	data, err := os.ReadFile(configPath) //nolint:gosec // path is from user home dir, intentional
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("app config not found at %s - run 'dot-manager init' or create it manually", configPath)
		}

		return nil, fmt.Errorf("reading app config: %w", err)
	}

	var cfg AppConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing app config: %w", err)
	}

	if cfg.ConfigDir == "" {
		return nil, fmt.Errorf("config_dir not set in %s", configPath)
	}

	// Expand ~ in config_dir
	cfg.ConfigDir = ExpandPath(cfg.ConfigDir, nil)

	// Verify the directory exists
	if _, err := os.Stat(cfg.ConfigDir); os.IsNotExist(err) {
		return nil, fmt.Errorf("configurations directory does not exist: %s", cfg.ConfigDir)
	}

	return &cfg, nil
}

// SaveAppConfig saves the app configuration to ~/.config/dot-manager/config.yaml
func SaveAppConfig(cfg *AppConfig) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("getting home directory: %w", err)
	}

	configDir := filepath.Join(home, appConfigDir)
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	configPath := filepath.Join(configDir, appConfigFile)

	data, err := marshalYAML(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	// Add a header comment
	content := fmt.Sprintf("# dot-manager app configuration\n# This file only stores the path to your configurations repository\n\n%s", string(data))

	// Use 0600 permissions to restrict access to owner only
	if err := os.WriteFile(configPath, []byte(content), 0600); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	return nil
}

// GetRepoConfigPath returns the path to the repository's dot-manager.yaml
func (a *AppConfig) GetRepoConfigPath() string {
	return filepath.Join(a.ConfigDir, repoConfigFile)
}

// AppConfigPath returns the path where the app config is stored.
// Returns an empty string if the home directory cannot be determined.
func AppConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}

	return filepath.Join(home, appConfigDir, appConfigFile)
}
