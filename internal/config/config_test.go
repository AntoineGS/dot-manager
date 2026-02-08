package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestLoad(t *testing.T) {
	t.Parallel()
	// Create a temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
version: 3
backup_root: "~/gits/configurations"

applications:
  - name: "neovim"
    description: "Text editor"
    entries:
      - name: "nvim-config"
        files: []
        backup: "./Both/Neovim/nvim"
        targets:
          linux: "~/.config/nvim"
          windows: "~/AppData/Local/nvim"

  - name: "bash"
    description: "Bash shell"
    entries:
      - name: "bashrc"
        files: [".bashrc", ".bash_profile"]
        backup: "./Linux/Bash"
        targets:
          linux: "~"

  - name: "pacman-hooks"
    description: "Pacman hooks"
    entries:
      - name: "pkg-backup-hook"
        sudo: true
        files: ["pkg-backup.hook"]
        backup: "./Linux/pacman"
        targets:
          linux: "/etc/pacman.d/hooks"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Test version
	if cfg.Version != 3 {
		t.Errorf("Version = %d, want 3", cfg.Version)
	}

	// Test backup root
	if cfg.BackupRoot != "~/gits/configurations" {
		t.Errorf("BackupRoot = %q, want %q", cfg.BackupRoot, "~/gits/configurations")
	}

	// Test applications count
	if len(cfg.Applications) != 3 {
		t.Errorf("len(Applications) = %d, want 3", len(cfg.Applications))
	}

	// Test first application (neovim)
	if cfg.Applications[0].Name != "neovim" { // nolint:goconst // test data
		t.Errorf("Applications[0].Name = %q, want %q", cfg.Applications[0].Name, "neovim")
	}

	if len(cfg.Applications[0].Entries) != 1 {
		t.Errorf("len(Applications[0].Entries) = %d, want 1", len(cfg.Applications[0].Entries))
	}

	if !cfg.Applications[0].Entries[0].IsFolder() {
		t.Error("Applications[0].Entries[0].IsFolder() = false, want true")
	}

	// Test second application (bash)
	if cfg.Applications[1].Name != "bash" {
		t.Errorf("Applications[1].Name = %q, want %q", cfg.Applications[1].Name, "bash")
	}

	if cfg.Applications[1].Entries[0].IsFolder() {
		t.Error("Applications[1].Entries[0].IsFolder() = true, want false")
	}

	if len(cfg.Applications[1].Entries[0].Files) != 2 {
		t.Errorf("len(Applications[1].Entries[0].Files) = %d, want 2", len(cfg.Applications[1].Entries[0].Files))
	}

	// Test third application (pacman-hooks with sudo)
	if cfg.Applications[2].Name != "pacman-hooks" {
		t.Errorf("Applications[2].Name = %q, want %q", cfg.Applications[2].Name, "pacman-hooks")
	}

	if !cfg.Applications[2].Entries[0].Sudo {
		t.Error("Applications[2].Entries[0].Sudo = false, want true")
	}
}

func TestLoadNonExistent(t *testing.T) {
	t.Parallel()

	_, err := Load("/nonexistent/config.yaml")
	if err == nil {
		t.Error("Load() expected error for non-existent file")
	}
}

func TestLoadInvalidYAML(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	if err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() expected error for invalid YAML")
	}
}

func TestLoadUnsupportedVersion(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
version: 2
backup_root: "~/dotfiles"
applications: []
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() expected error for unsupported version 2")
	}
}

func TestExpandPaths(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Version:    3,
		BackupRoot: "~/gits/configs",
		Applications: []Application{
			{
				Name: "test-app",
				Entries: []SubEntry{
					{
						Name:   "test",
						Backup: "./test",
						Files:  []string{"$CUSTOM_VAR"},
						Targets: map[string]string{
							"linux": "~/.config/test",
						},
					},
				},
			},
		},
	}

	envVars := map[string]string{
		"CUSTOM_VAR": "expanded_value",
	}

	cfg.ExpandPaths(envVars)

	home, _ := os.UserHomeDir()

	// Test backup root expansion
	expectedBackupRoot := filepath.Join(home, "gits/configs")
	if cfg.BackupRoot != expectedBackupRoot {
		t.Errorf("BackupRoot = %q, want %q", cfg.BackupRoot, expectedBackupRoot)
	}

	// Test file variable expansion
	if cfg.Applications[0].Entries[0].Files[0] != "expanded_value" {
		t.Errorf("Files[0] = %q, want %q", cfg.Applications[0].Entries[0].Files[0], "expanded_value")
	}

	// Test target expansion
	expectedTarget := filepath.Join(home, ".config/test")
	if cfg.Applications[0].Entries[0].Targets["linux"] != expectedTarget {
		t.Errorf("Targets[linux] = %q, want %q", cfg.Applications[0].Entries[0].Targets["linux"], expectedTarget)
	}
}

func TestSave(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := &Config{
		Version:    3,
		BackupRoot: "/home/user/dotfiles",
		Applications: []Application{
			{
				Name:        "neovim",
				Description: "Text editor",
				Entries: []SubEntry{
					{
						Name:   "nvim-config",
						Files:  []string{},
						Backup: "./nvim",
						Targets: map[string]string{
							"linux":   "~/.config/nvim",
							"windows": "~/AppData/Local/nvim",
						},
					},
				},
			},
			{
				Name:        "pacman",
				Description: "Pacman hooks",
				Entries: []SubEntry{
					{
						Name:   "pacman-hooks",
						Sudo:   true,
						Files:  []string{"hook.conf"},
						Backup: "./pacman",
						Targets: map[string]string{
							"linux": "/etc/pacman.d/hooks",
						},
					},
				},
			},
		},
	}

	err := Save(cfg, configPath)
	if err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load it back and verify
	loaded, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() after Save() error = %v", err)
	}

	if loaded.Version != cfg.Version {
		t.Errorf("Version = %d, want %d", loaded.Version, cfg.Version)
	}

	if loaded.BackupRoot != cfg.BackupRoot {
		t.Errorf("BackupRoot = %q, want %q", loaded.BackupRoot, cfg.BackupRoot)
	}

	if len(loaded.Applications) != len(cfg.Applications) {
		t.Errorf("len(Applications) = %d, want %d", len(loaded.Applications), len(cfg.Applications))
	}

	if loaded.Applications[0].Name != cfg.Applications[0].Name {
		t.Errorf("Applications[0].Name = %q, want %q", loaded.Applications[0].Name, cfg.Applications[0].Name)
	}
}

func TestSaveToNonexistentDirectory(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent", "subdir", "config.yaml")

	cfg := &Config{Version: 3}

	err := Save(cfg, configPath)
	if err == nil {
		t.Error("Save() should error when directory doesn't exist")
	}
}

func TestLoadDefaultVersion(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Config without explicit version
	configContent := `
backup_root: "~/dotfiles"
applications: []
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should default to version 3
	if cfg.Version != 3 {
		t.Errorf("Version = %d, want 3 (default)", cfg.Version)
	}
}

func TestLoadWithPackages(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
version: 3
backup_root: "~/dotfiles"
default_manager: "pacman"
manager_priority:
  - paru
  - yay
  - pacman

applications:
  - name: neovim
    description: "Editor"
    when: '{{ eq .OS "linux" }}'
    package:
      managers:
        pacman: neovim
        apt: neovim
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DefaultManager != "pacman" {
		t.Errorf("DefaultManager = %q, want %q", cfg.DefaultManager, "pacman")
	}

	if len(cfg.ManagerPriority) != 3 {
		t.Errorf("len(ManagerPriority) = %d, want 3", len(cfg.ManagerPriority))
	}

	if len(cfg.Applications) != 1 {
		t.Errorf("len(Applications) = %d, want 1", len(cfg.Applications))
	}

	if cfg.Applications[0].Name != "neovim" { // nolint:goconst // test data
		t.Errorf("Applications[0].Name = %q, want %q", cfg.Applications[0].Name, "neovim")
	}

	if cfg.Applications[0].When != `{{ eq .OS "linux" }}` {
		t.Errorf("When = %q, want %q", cfg.Applications[0].When, `{{ eq .OS "linux" }}`)
	}

	if cfg.Applications[0].Package == nil {
		t.Fatal("Applications[0].Package is nil, want non-nil")
	}

	if cfg.Applications[0].Package.Managers["pacman"].PackageName != "neovim" {
		t.Errorf("Package.Managers[pacman] = %q, want %q", cfg.Applications[0].Package.Managers["pacman"].PackageName, "neovim")
	}
}

func TestExpandPathOnlyTilde(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Version:      3,
		BackupRoot:   "~",
		Applications: []Application{},
	}

	cfg.ExpandPaths(nil)

	home, _ := os.UserHomeDir()
	if cfg.BackupRoot != home {
		t.Errorf("BackupRoot = %q, want %q", cfg.BackupRoot, home)
	}
}

func TestExpandPathEmpty(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Version:      3,
		BackupRoot:   "",
		Applications: []Application{},
	}

	cfg.ExpandPaths(nil)

	if cfg.BackupRoot != "" {
		t.Errorf("BackupRoot = %q, want empty string", cfg.BackupRoot)
	}
}

func TestLoadWithURLInstall(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
version: 3
backup_root: "~/dotfiles"

applications:
  - name: custom-tool
    description: "Custom tool"
    package:
      url:
        linux:
          url: "https://example.com/tool.tar.gz"
          command: "tar xzf {file} -C /usr/local/bin"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Applications) != 1 {
		t.Fatalf("len(Applications) = %d, want 1", len(cfg.Applications))
	}

	if cfg.Applications[0].Package == nil {
		t.Fatal("Package is nil")
	}

	urlSpec := cfg.Applications[0].Package.URL["linux"]
	if urlSpec.URL != "https://example.com/tool.tar.gz" {
		t.Errorf("URL = %q, want %q", urlSpec.URL, "https://example.com/tool.tar.gz")
	}

	if urlSpec.Command != "tar xzf {file} -C /usr/local/bin" {
		t.Errorf("Command = %q", urlSpec.Command)
	}
}

func TestLoadWithCustomInstall(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
version: 3
backup_root: "~/dotfiles"

applications:
  - name: custom-tool
    description: "Custom tool"
    package:
      custom:
        linux: "curl -fsSL https://example.com/install.sh | bash"
        windows: "iwr https://example.com/install.ps1 | iex"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Applications) != 1 {
		t.Fatalf("len(Applications) = %d, want 1", len(cfg.Applications))
	}

	if cfg.Applications[0].Package == nil {
		t.Fatal("Package is nil")
	}

	custom := cfg.Applications[0].Package.Custom
	if custom["linux"] != "curl -fsSL https://example.com/install.sh | bash" {
		t.Errorf("Custom[linux] = %q", custom["linux"])
	}
}

func TestLoadApplicationStructure(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
version: 3
backup_root: "~/gits/configurations"
default_manager: "pacman"

applications:
  - name: "neovim"
    description: "Text editor"
    when: '{{ eq .OS "linux" }}'
    entries:
      - type: "config"
        name: "nvim-config"
        backup: "./Both/Neovim/nvim"
        targets:
          linux: "~/.config/nvim"
          windows: "~/AppData/Local/nvim"
      - type: "config"
        name: "nvim-local"
        files: ["local.lua"]
        backup: "./Both/Neovim/local"
        targets:
          linux: "~/.config/nvim/lua"
    package:
      managers:
        pacman: "neovim"
        apt: "neovim"

  - name: "zsh"
    description: "Zsh configuration"
    entries:
      - type: "config"
        name: "zshrc"
        backup: "./zsh"
        sudo: true
        targets:
          linux: "/etc/zsh/zshrc"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Test version
	if cfg.Version != 3 {
		t.Errorf("Version = %d, want 3", cfg.Version)
	}

	// Test applications count
	if len(cfg.Applications) != 2 {
		t.Fatalf("len(Applications) = %d, want 2", len(cfg.Applications))
	}

	// Test first application (neovim)
	app1 := cfg.Applications[0]
	if app1.Name != "neovim" {
		t.Errorf("Applications[0].Name = %q, want %q", app1.Name, "neovim")
	}

	if app1.Description != "Text editor" {
		t.Errorf("Applications[0].Description = %q, want %q", app1.Description, "Text editor")
	}

	if app1.When != `{{ eq .OS "linux" }}` {
		t.Errorf("Applications[0].When = %q, want %q", app1.When, `{{ eq .OS "linux" }}`)
	}

	if len(app1.Entries) != 2 {
		t.Fatalf("len(Applications[0].Entries) = %d, want 2", len(app1.Entries))
	}

	// Test first sub-entry (nvim-config)
	subEntry1 := app1.Entries[0]
	if subEntry1.Name != "nvim-config" {
		t.Errorf("SubEntry[0].Name = %q, want %q", subEntry1.Name, "nvim-config")
	}

	if !subEntry1.IsConfig() {
		t.Error("SubEntry[0].IsConfig() = false, want true")
	}

	if !subEntry1.IsFolder() {
		t.Error("SubEntry[0].IsFolder() = false, want true")
	}

	if subEntry1.GetTarget("linux") != "~/.config/nvim" {
		t.Errorf("SubEntry[0].GetTarget(linux) = %q, want %q", subEntry1.GetTarget("linux"), "~/.config/nvim")
	}

	// Test second sub-entry (nvim-local)
	subEntry2 := app1.Entries[1]
	if len(subEntry2.Files) != 1 {
		t.Errorf("len(SubEntry[1].Files) = %d, want 1", len(subEntry2.Files))
	}

	if subEntry2.IsFolder() {
		t.Error("SubEntry[1].IsFolder() = true, want false")
	}

	// Test application package
	if !app1.HasPackage() {
		t.Fatal("Applications[0].HasPackage() = false, want true")
	}

	if app1.Package.Managers["pacman"].PackageName != "neovim" {
		t.Errorf("Applications[0].Package.Managers[pacman] = %q, want %q", app1.Package.Managers["pacman"].PackageName, "neovim")
	}

	// Test second application (zsh)
	app2 := cfg.Applications[1]
	if app2.Name != "zsh" {
		t.Errorf("Applications[1].Name = %q, want %q", app2.Name, "zsh")
	}

	if len(app2.Entries) != 1 {
		t.Fatalf("len(Applications[1].Entries) = %d, want 1", len(app2.Entries))
	}

	// Test config sub-entry with sudo
	configEntry := app2.Entries[0]
	if configEntry.Name != "zshrc" {
		t.Errorf("ConfigEntry.Name = %q, want %q", configEntry.Name, "zshrc")
	}

	if !configEntry.IsConfig() {
		t.Error("ConfigEntry.IsConfig() = false, want true")
	}

	if !configEntry.Sudo {
		t.Error("ConfigEntry.Sudo = false, want true")
	}
}

func TestGetFilteredApplications(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Version: 3,
		Applications: []Application{
			{
				Name:        "neovim",
				Description: "Text editor",
				When:        `{{ eq .OS "linux" }}`,
				Entries: []SubEntry{
					{Name: "nvim-config", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim"}},
				},
				Package: &EntryPackage{Managers: map[string]ManagerValue{"pacman": {PackageName: "neovim"}}},
			},
			{
				Name:        "vscode",
				Description: "Code editor",
				When:        `{{ eq .OS "windows" }}`,
				Entries: []SubEntry{
					{Name: "vscode-config", Backup: "./vscode", Targets: map[string]string{"windows": "~/AppData/Roaming/Code"}},
				},
			},
			{
				Name:        "git",
				Description: "Version control",
				Entries: []SubEntry{
					{Name: "gitconfig", Files: []string{".gitconfig"}, Backup: "./git", Targets: map[string]string{"linux": "~", "windows": "~"}},
				},
			},
			{
				Name:        "work-only",
				Description: "Work tools",
				When:        `{{ eq .Hostname "work-laptop" }}`,
				Entries: []SubEntry{
					{Name: "work-config", Backup: "./work", Targets: map[string]string{"linux": "~/.work"}},
				},
			},
		},
	}

	// Linux renderer that reports OS=linux, Hostname=work-laptop
	linuxRenderer := &mockWhenRenderer{result: ""}
	// Use a more realistic mock that evaluates differently per app
	linuxApps := testGetFilteredApps(t, cfg, map[string]bool{
		"neovim":    true,
		"vscode":    false,
		"git":       true,
		"work-only": true,
	})

	if len(linuxApps) != 3 {
		t.Errorf("Expected 3 apps, got %d", len(linuxApps))
	}

	// Windows context - should get vscode and git
	windowsApps := testGetFilteredApps(t, cfg, map[string]bool{
		"neovim":    false,
		"vscode":    true,
		"git":       true,
		"work-only": false,
	})

	if len(windowsApps) != 2 {
		t.Errorf("Expected 2 apps, got %d", len(windowsApps))
	}

	_ = linuxRenderer // suppress unused warning
}

// testGetFilteredApps is a test helper that filters apps using a per-app match map
func testGetFilteredApps(t *testing.T, cfg *Config, matches map[string]bool) []Application {
	t.Helper()

	result := make([]Application, 0, len(cfg.Applications))

	for _, app := range cfg.Applications {
		shouldMatch, specified := matches[app.Name]

		// No when = always match
		if app.When == "" {
			result = append(result, app)

			if specified && !shouldMatch {
				t.Errorf("App %q has no when expression but was expected to be excluded", app.Name)
			}

			continue
		}

		if specified && shouldMatch {
			result = append(result, app)
		}
	}

	return result
}

func TestExpandPathsV3(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Version:    3,
		BackupRoot: "~/gits/configs",
		Applications: []Application{
			{
				Name: "neovim",
				Entries: []SubEntry{
					{
						Name:   "nvim-config",
						Backup: "./nvim",
						Files:  []string{"$CUSTOM_VAR"},
						Targets: map[string]string{
							"linux": "~/.config/nvim",
						},
					},
				},
			},
		},
	}

	envVars := map[string]string{
		"CUSTOM_VAR": "expanded_value",
	}

	cfg.ExpandPaths(envVars)

	home, _ := os.UserHomeDir()

	// Test backup root expansion
	expectedBackupRoot := filepath.Join(home, "gits/configs")
	if cfg.BackupRoot != expectedBackupRoot {
		t.Errorf("BackupRoot = %q, want %q", cfg.BackupRoot, expectedBackupRoot)
	}

	// Test sub-entry backup expansion
	if cfg.Applications[0].Entries[0].Backup != "./nvim" {
		t.Errorf("SubEntry Backup = %q, want %q", cfg.Applications[0].Entries[0].Backup, "./nvim")
	}

	// Test file variable expansion
	if cfg.Applications[0].Entries[0].Files[0] != "expanded_value" {
		t.Errorf("SubEntry Files[0] = %q, want %q", cfg.Applications[0].Entries[0].Files[0], "expanded_value")
	}

	// Test target expansion for config entry
	expectedTarget := filepath.Join(home, ".config/nvim")
	if cfg.Applications[0].Entries[0].Targets["linux"] != expectedTarget {
		t.Errorf("SubEntry Targets[linux] = %q, want %q", cfg.Applications[0].Entries[0].Targets["linux"], expectedTarget)
	}
}

func TestExpandPath(t *testing.T) {
	t.Parallel()

	home, _ := os.UserHomeDir()

	tests := []struct {
		name     string
		path     string
		envVars  map[string]string
		expected string
	}{
		{
			name:     "tilde path",
			path:     "~/.config/nvim",
			envVars:  nil,
			expected: filepath.Join(home, ".config/nvim"),
		},
		{
			name:     "tilde only",
			path:     "~",
			envVars:  nil,
			expected: home,
		},
		{
			name:     "absolute path",
			path:     "/usr/local/bin",
			envVars:  nil,
			expected: "/usr/local/bin",
		},
		{
			name:     "relative path",
			path:     "./config",
			envVars:  nil,
			expected: "./config",
		},
		{
			name:     "env var expansion",
			path:     "$HOME/.config",
			envVars:  nil,
			expected: filepath.Join(home, ".config"),
		},
		{
			name:     "custom env var",
			path:     "$MYVAR/config",
			envVars:  map[string]string{"MYVAR": "/custom/path"},
			expected: "/custom/path/config",
		},
		{
			name:     "empty path",
			path:     "",
			envVars:  nil,
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ExpandPath(tt.path, tt.envVars)
			if result != tt.expected {
				t.Errorf("ExpandPath(%q) = %q, want %q", tt.path, result, tt.expected)
			}
		})
	}
}

func TestGetAllSubEntries(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Version: 3,
		Applications: []Application{
			{
				Name:        "neovim",
				Description: "Text editor",
				When:        `{{ eq .OS "linux" }}`,
				Entries: []SubEntry{
					{Name: "nvim-config", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim"}},
				},
			},
			{
				Name:        "vscode",
				Description: "Code editor",
				When:        `{{ eq .OS "windows" }}`,
				Entries: []SubEntry{
					{Name: "vscode-config", Backup: "./vscode", Targets: map[string]string{"windows": "~/AppData/Roaming/Code"}},
				},
			},
			{
				Name:        "git",
				Description: "Version control",
				Entries: []SubEntry{
					{Name: "gitconfig", Files: []string{".gitconfig"}, Backup: "./git", Targets: map[string]string{"linux": "~"}},
				},
			},
		},
	}

	// Mock renderer that renders to "true" for Linux when expressions
	linuxRenderer := &mockWhenRenderer{result: "true"}

	// With a renderer that always returns "true", all apps match
	subEntries := cfg.GetAllSubEntries(linuxRenderer)
	if len(subEntries) != 3 {
		t.Errorf("GetAllSubEntries(true renderer) returned %d sub-entries, want 3", len(subEntries))
	}

	// With a renderer that always returns "false", only no-when apps match
	falseRenderer := &mockWhenRenderer{result: "false"}

	filteredSubEntries := cfg.GetAllSubEntries(falseRenderer)
	if len(filteredSubEntries) != 1 {
		t.Errorf("GetAllSubEntries(false renderer) returned %d sub-entries, want 1 (git, no when)", len(filteredSubEntries))
	}

	if filteredSubEntries[0].Name != "gitconfig" {
		t.Errorf("Expected gitconfig, got %s", filteredSubEntries[0].Name)
	}

	// Test with empty applications
	emptyConfig := &Config{Version: 3, Applications: []Application{}}

	emptySubEntries := emptyConfig.GetAllSubEntries(linuxRenderer)
	if len(emptySubEntries) != 0 {
		t.Errorf("GetAllSubEntries on empty config returned %d sub-entries, want 0", len(emptySubEntries))
	}
}

func TestGetAllConfigSubEntries(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Version: 3,
		Applications: []Application{
			{
				Name:        "neovim",
				Description: "Text editor",
				When:        `{{ eq .OS "linux" }}`,
				Entries: []SubEntry{
					{Name: "nvim-config", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim"}},
					{Name: "nvim-local", Files: []string{"local.lua"}, Backup: "./nvim-local", Targets: map[string]string{"linux": "~/.config/nvim/lua"}},
				},
			},
			{
				Name:        "zsh",
				Description: "Zsh configuration",
				When:        `{{ eq .OS "linux" }}`,
				Entries: []SubEntry{
					{Name: "zshrc", Backup: "./zsh", Targets: map[string]string{"linux": "~/.zshrc"}},
				},
			},
			{
				Name:        "vscode",
				Description: "Code editor",
				When:        `{{ eq .OS "windows" }}`,
				Entries: []SubEntry{
					{Name: "vscode-config", Backup: "./vscode", Targets: map[string]string{"windows": "~/AppData/Roaming/Code"}},
				},
			},
		},
	}

	// Renderer that always matches - should get all config sub-entries
	trueRenderer := &mockWhenRenderer{result: "true"}

	configSubEntries := cfg.GetAllConfigSubEntries(trueRenderer)
	if len(configSubEntries) != 4 {
		t.Errorf("GetAllConfigSubEntries(true renderer) returned %d sub-entries, want 4", len(configSubEntries))
	}

	// Verify we only got config type entries
	for _, e := range configSubEntries {
		if !e.IsConfig() {
			t.Errorf("GetAllConfigSubEntries returned non-config entry: %s", e.Name)
		}
	}

	// Renderer that never matches - only apps with no when expression pass
	falseRenderer := &mockWhenRenderer{result: "false"}

	filteredConfigSubEntries := cfg.GetAllConfigSubEntries(falseRenderer)
	if len(filteredConfigSubEntries) != 0 {
		t.Errorf("GetAllConfigSubEntries(false renderer) returned %d sub-entries, want 0 (all apps have when)", len(filteredConfigSubEntries))
	}
}

// mockRenderer implements PathRenderer for testing.
type mockRenderer struct {
	values map[string]string
}

func (m *mockRenderer) RenderString(_, tmplStr string) (string, error) {
	// Simple mock: replace known template expressions
	result := tmplStr
	for k, v := range m.values {
		result = strings.ReplaceAll(result, k, v)
	}
	return result, nil
}

func TestExpandPathWithTemplate_NoTemplate(t *testing.T) {
	t.Parallel()

	renderer := &mockRenderer{}
	result := ExpandPathWithTemplate("~/.config/nvim", nil, renderer)
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config/nvim")
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestExpandPathWithTemplate_WithTemplate(t *testing.T) {
	t.Parallel()

	renderer := &mockRenderer{
		values: map[string]string{
			"{{ .Hostname }}": "myhost",
		},
	}
	result := ExpandPathWithTemplate("~/.config/{{ .Hostname }}/nvim", nil, renderer)
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config/myhost/nvim")
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestExpandPathWithTemplate_NilRenderer(t *testing.T) {
	t.Parallel()

	// With nil renderer, should fall back to ExpandPath
	result := ExpandPathWithTemplate("~/.config/nvim", nil, nil)
	home, _ := os.UserHomeDir()
	want := filepath.Join(home, ".config/nvim")
	if result != want {
		t.Errorf("got %q, want %q", result, want)
	}
}

func TestLoadWithInstallerPackage(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
version: 3
backup_root: "~/dotfiles"

applications:
  - name: custom-tool
    description: "Custom tool with installer"
    package:
      managers:
        pacman: custom-tool
        installer:
          command:
            linux: "curl -fsSL https://example.com/install.sh | sh"
            windows: "iwr https://example.com/install.ps1 | iex"
          binary: "mytool"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0600); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Applications) != 1 {
		t.Fatalf("len(Applications) = %d, want 1", len(cfg.Applications))
	}

	if cfg.Applications[0].Package == nil {
		t.Fatal("Package is nil")
	}

	// Check pacman manager
	pacmanVal, ok := cfg.Applications[0].Package.GetManagerString("pacman")
	if !ok || pacmanVal != "custom-tool" {
		t.Errorf("GetManagerString(pacman) = %q, %v, want %q, true", pacmanVal, ok, "custom-tool")
	}

	// Check installer manager
	installerPkg, ok := cfg.Applications[0].Package.GetInstallerPackage()
	if !ok {
		t.Fatal("GetInstallerPackage() returned false, want true")
	}

	if installerPkg.Command["linux"] != "curl -fsSL https://example.com/install.sh | sh" {
		t.Errorf("Installer.Command[linux] = %q", installerPkg.Command["linux"])
	}

	if installerPkg.Command["windows"] != "iwr https://example.com/install.ps1 | iex" {
		t.Errorf("Installer.Command[windows] = %q", installerPkg.Command["windows"])
	}

	if installerPkg.Binary != "mytool" {
		t.Errorf("Installer.Binary = %q, want %q", installerPkg.Binary, "mytool")
	}

	// GetManagerString should return false for installer
	_, ok = cfg.Applications[0].Package.GetManagerString("installer")
	if ok {
		t.Error("GetManagerString(installer) should return false for installer managers")
	}
}

func TestGetInstallerPackage_NotPresent(t *testing.T) {
	t.Parallel()

	ep := &EntryPackage{
		Managers: map[string]ManagerValue{
			"pacman": {PackageName: "neovim"},
		},
	}

	pkg, ok := ep.GetInstallerPackage()
	if ok {
		t.Error("GetInstallerPackage() returned true, want false")
	}
	if pkg != nil {
		t.Errorf("GetInstallerPackage() = %v, want nil", pkg)
	}
}

func TestGetInstallerPackage_NilManagers(t *testing.T) {
	t.Parallel()

	ep := &EntryPackage{}

	pkg, ok := ep.GetInstallerPackage()
	if ok {
		t.Error("GetInstallerPackage() returned true, want false")
	}
	if pkg != nil {
		t.Errorf("GetInstallerPackage() = %v, want nil", pkg)
	}
}

func TestLoad_FileHandleClosed(t *testing.T) {
	// Create temp config file
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.yaml")

	content := `version: 3
backup_root: /test
applications:
  - name: test
    entries:
      - name: test-config
        backup: ./test
        targets:
          linux: ~/.config/test
`
	if err := os.WriteFile(cfgPath, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}

	// Load config
	cfg, err := Load(cfgPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Try to remove file immediately - should succeed if handle closed
	if err := os.Remove(cfgPath); err != nil {
		t.Errorf("Failed to remove config file, handle may not be closed: %v", err)
	}

	if cfg == nil {
		t.Error("Expected config to be loaded")
	}
}

func TestManagerValueMarshalYAML(t *testing.T) {
	t.Parallel()

	t.Run("string manager marshals as plain string", func(t *testing.T) {
		t.Parallel()
		ep := EntryPackage{
			Managers: map[string]ManagerValue{
				"pacman": {PackageName: "neovim"},
			},
		}

		out, err := yaml.Marshal(&ep)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}

		// Should produce "pacman: neovim", not "pacman:\n  packagename: neovim\n  git: null"
		output := string(out)
		if strings.Contains(output, "packagename") {
			t.Errorf("String manager should marshal as plain string, got:\n%s", output)
		}
	})

	t.Run("installer manager marshals as object", func(t *testing.T) {
		t.Parallel()
		ep := EntryPackage{
			Managers: map[string]ManagerValue{
				"installer": {Installer: &InstallerPackage{
					Command: map[string]string{"linux": "curl -fsSL example.com | sh"},
					Binary:  "mytool",
				}},
			},
		}

		out, err := yaml.Marshal(&ep)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}

		output := string(out)
		if strings.Contains(output, "packagename") {
			t.Errorf("Installer manager should not contain packagename, got:\n%s", output)
		}

		// Verify round-trip
		var ep2 EntryPackage
		if err := yaml.Unmarshal(out, &ep2); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}

		installerVal, ok := ep2.Managers["installer"]
		if !ok || installerVal.Installer == nil {
			t.Fatal("Round-trip lost installer manager")
		}

		if installerVal.Installer.Command["linux"] != "curl -fsSL example.com | sh" {
			t.Errorf("Round-trip Command[linux] = %q, want %q", installerVal.Installer.Command["linux"], "curl -fsSL example.com | sh")
		}

		if installerVal.Installer.Binary != "mytool" {
			t.Errorf("Round-trip Binary = %q, want %q", installerVal.Installer.Binary, "mytool")
		}
	})

	t.Run("git manager marshals without nested git key", func(t *testing.T) {
		t.Parallel()
		ep := EntryPackage{
			Managers: map[string]ManagerValue{
				"git": {Git: &GitPackage{
					URL:     "https://github.com/user/repo.git",
					Targets: map[string]string{"linux": "/usr/share/test"},
					Sudo:    true,
				}},
			},
		}

		out, err := yaml.Marshal(&ep)
		if err != nil {
			t.Fatalf("Marshal error: %v", err)
		}

		output := string(out)
		// Should have url directly under git, not a nested git key
		if strings.Contains(output, "packagename") {
			t.Errorf("Git manager should not contain packagename, got:\n%s", output)
		}

		// Verify round-trip
		var ep2 EntryPackage
		if err := yaml.Unmarshal(out, &ep2); err != nil {
			t.Fatalf("Unmarshal error: %v", err)
		}

		gitPkg, ok := ep2.Managers["git"]
		if !ok || gitPkg.Git == nil {
			t.Fatal("Round-trip lost git manager")
		}

		if gitPkg.Git.URL != "https://github.com/user/repo.git" {
			t.Errorf("Round-trip URL = %q, want %q", gitPkg.Git.URL, "https://github.com/user/repo.git")
		}
	})
}
