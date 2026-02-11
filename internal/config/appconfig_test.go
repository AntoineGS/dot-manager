package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Note: Tests that modify HOME environment variable cannot run in parallel
// because os.Setenv affects the entire process.

func TestSaveAppConfig(t *testing.T) {
	// Create a temporary home directory
	tmpDir := t.TempDir()

	// Override HOME for this test
	origHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HOME", origHome) }()

	cfg := &AppConfig{
		ConfigDir: "/path/to/configs",
	}

	err := SaveAppConfig(cfg)
	if err != nil {
		t.Fatalf("SaveAppConfig() error = %v", err)
	}

	// Verify file was created
	configPath := filepath.Join(tmpDir, appConfigDir, appConfigFile)
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Verify content
	data, err := os.ReadFile(configPath) //nolint:gosec // test file path is controlled
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)

	if !strings.Contains(content, "config_dir: /path/to/configs") {
		t.Errorf("Config file should contain config_dir, got: %s", content)
	}

	if !strings.Contains(content, "# tidydots app configuration") {
		t.Error("Config file should have header comment")
	}
}

func TestLoadAppConfig(t *testing.T) {
	tmpDir := t.TempDir()

	// Override HOME
	origHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HOME", origHome) }()

	// Create config directory
	configDir := filepath.Join(tmpDir, appConfigDir)
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Create a configs directory that will be referenced
	configsRepo := filepath.Join(tmpDir, "my-dotfiles")
	if err := os.MkdirAll(configsRepo, 0750); err != nil {
		t.Fatal(err)
	}

	// Write app config
	configContent := "config_dir: " + configsRepo + "\n"
	configPath := filepath.Join(configDir, appConfigFile)
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatal(err)
	}

	// Load it
	cfg, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}

	if cfg.ConfigDir != configsRepo {
		t.Errorf("ConfigDir = %q, want %q", cfg.ConfigDir, configsRepo)
	}
}

func TestLoadAppConfigWithTilde(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HOME", origHome) }()

	// Create config directory
	configDir := filepath.Join(tmpDir, appConfigDir)
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Create a configs directory
	configsRepo := filepath.Join(tmpDir, "dotfiles")
	if err := os.MkdirAll(configsRepo, 0750); err != nil {
		t.Fatal(err)
	}

	// Write app config with tilde
	configContent := "config_dir: ~/dotfiles\n"
	configPath := filepath.Join(configDir, appConfigFile)
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatal(err)
	}

	cfg, err := LoadAppConfig()
	if err != nil {
		t.Fatalf("LoadAppConfig() error = %v", err)
	}

	expected := filepath.Join(tmpDir, "dotfiles")
	if cfg.ConfigDir != expected {
		t.Errorf("ConfigDir = %q, want %q", cfg.ConfigDir, expected)
	}
}

func TestLoadAppConfigNotFound(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HOME", origHome) }()

	_, err := LoadAppConfig()
	if err == nil {
		t.Error("LoadAppConfig() should error when config doesn't exist")
	}

	if !strings.Contains(err.Error(), "app config not found") {
		t.Errorf("Error should mention 'app config not found', got: %v", err)
	}
}

func TestLoadAppConfigInvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HOME", origHome) }()

	configDir := filepath.Join(tmpDir, appConfigDir)
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Write invalid YAML
	configPath := filepath.Join(configDir, appConfigFile)
	if err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadAppConfig()
	if err == nil {
		t.Error("LoadAppConfig() should error for invalid YAML")
	}
}

//nolint:dupl // similar test structure is intentional
func TestLoadAppConfigEmptyConfigDir(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HOME", origHome) }()

	configDir := filepath.Join(tmpDir, appConfigDir)
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Write config with empty config_dir
	configPath := filepath.Join(configDir, appConfigFile)
	if err := os.WriteFile(configPath, []byte("config_dir: \"\"\n"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadAppConfig()
	if err == nil {
		t.Error("LoadAppConfig() should error when config_dir is empty")
	}

	if !strings.Contains(err.Error(), "config_dir not set") {
		t.Errorf("Error should mention 'config_dir not set', got: %v", err)
	}
}

//nolint:dupl // similar test structure is intentional
func TestLoadAppConfigNonexistentConfigDir(t *testing.T) {
	tmpDir := t.TempDir()

	origHome := os.Getenv("HOME")
	if err := os.Setenv("HOME", tmpDir); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Setenv("HOME", origHome) }()

	configDir := filepath.Join(tmpDir, appConfigDir)
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Write config pointing to nonexistent directory
	configPath := filepath.Join(configDir, appConfigFile)
	if err := os.WriteFile(configPath, []byte("config_dir: /nonexistent/path\n"), 0600); err != nil {
		t.Fatal(err)
	}

	_, err := LoadAppConfig()
	if err == nil {
		t.Error("LoadAppConfig() should error when config_dir doesn't exist")
	}

	if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("Error should mention 'does not exist', got: %v", err)
	}
}

func TestGetRepoConfigPath(t *testing.T) {
	t.Parallel()

	configDir := filepath.Join("/home", "user", "dotfiles")
	cfg := &AppConfig{
		ConfigDir: configDir,
	}

	got := cfg.GetRepoConfigPath()
	want := filepath.Join(configDir, "tidydots.yaml")

	if got != want {
		t.Errorf("GetRepoConfigPath() = %q, want %q", got, want)
	}
}

func TestAppConfigPath(t *testing.T) {
	// Cannot run in parallel - uses os.UserHomeDir()
	path := AppConfigPath()

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, ".config/tidydots/config.yaml")

	if path != expected {
		t.Errorf("AppConfigPath() = %q, want %q", path, expected)
	}
}
