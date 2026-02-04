package config

import (
	"os"
	"path/filepath"
	"testing"
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
    filters:
      - include:
          os: "linux"
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

	if len(cfg.Applications[0].Filters) != 1 {
		t.Errorf("len(Filters) = %d, want 1", len(cfg.Applications[0].Filters))
	}

	if cfg.Applications[0].Filters[0].Include["os"] != "linux" {
		t.Errorf("Filters[0].Include[os] = %q, want %q", cfg.Applications[0].Filters[0].Include["os"], "linux")
	}

	if cfg.Applications[0].Package == nil {
		t.Fatal("Applications[0].Package is nil, want non-nil")
	}

	if cfg.Applications[0].Package.Managers["pacman"] != "neovim" {
		t.Errorf("Package.Managers[pacman] = %q, want %q", cfg.Applications[0].Package.Managers["pacman"], "neovim")
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
    filters:
      - include:
          os: "linux"
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

	if len(app1.Filters) != 1 {
		t.Errorf("len(Applications[0].Filters) = %d, want 1", len(app1.Filters))
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

	if app1.Package.Managers["pacman"] != "neovim" {
		t.Errorf("Applications[0].Package.Managers[pacman] = %q, want %q", app1.Package.Managers["pacman"], "neovim")
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
				Filters: []Filter{
					{Include: map[string]string{"os": "linux"}},
				},
				Entries: []SubEntry{
					{Name: "nvim-config", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim"}},
				},
				Package: &EntryPackage{Managers: map[string]interface{}{"pacman": "neovim"}},
			},
			{
				Name:        "vscode",
				Description: "Code editor",
				Filters: []Filter{
					{Include: map[string]string{"os": "windows"}},
				},
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
				Filters: []Filter{
					{Include: map[string]string{"hostname": "work-.*"}},
				},
				Entries: []SubEntry{
					{Name: "work-config", Backup: "./work", Targets: map[string]string{"linux": "~/.work"}},
				},
			},
		},
	}

	// Test Linux context - should get neovim, git, and work-only (no hostname filter)
	linuxCtx := &FilterContext{OS: "linux", Hostname: "work-laptop", User: "john"}

	apps := cfg.GetFilteredApplications(linuxCtx)
	if len(apps) != 3 {
		t.Errorf("GetFilteredApplications(linux, work-laptop) returned %d apps, want 3", len(apps))
	}

	names := make(map[string]bool)
	for _, app := range apps {
		names[app.Name] = true
	}

	if !names["neovim"] {
		t.Error("Expected neovim to be included on Linux")
	}

	if !names["git"] {
		t.Error("Expected git to be included (no filter)")
	}

	if !names["work-only"] {
		t.Error("Expected work-only to be included on work-laptop")
	}

	if names["vscode"] {
		t.Error("Expected vscode to be excluded on Linux")
	}

	// Test Windows context - should get vscode and git
	windowsCtx := &FilterContext{OS: "windows", Hostname: "home-desktop", User: "john"}

	windowsApps := cfg.GetFilteredApplications(windowsCtx)
	if len(windowsApps) != 2 {
		t.Errorf("GetFilteredApplications(windows, home-desktop) returned %d apps, want 2", len(windowsApps))
	}

	windowsNames := make(map[string]bool)
	for _, app := range windowsApps {
		windowsNames[app.Name] = true
	}

	if !windowsNames["vscode"] {
		t.Error("Expected vscode to be included on Windows")
	}

	if !windowsNames["git"] {
		t.Error("Expected git to be included (no filter)")
	}

	if windowsNames["neovim"] {
		t.Error("Expected neovim to be excluded on Windows")
	}

	if windowsNames["work-only"] {
		t.Error("Expected work-only to be excluded on home-desktop")
	}
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
				Filters: []Filter{
					{Include: map[string]string{"os": "linux"}},
				},
				Entries: []SubEntry{
					{Name: "nvim-config", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim"}},
				},
			},
			{
				Name:        "vscode",
				Description: "Code editor",
				Filters: []Filter{
					{Include: map[string]string{"os": "windows"}},
				},
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

	// Test with Linux context - should get all sub-entries from neovim and git apps
	linuxCtx := &FilterContext{OS: "linux", Hostname: "laptop", User: "john"}

	subEntries := cfg.GetAllSubEntries(linuxCtx)
	if len(subEntries) != 2 {
		t.Errorf("GetAllSubEntries(linux) returned %d sub-entries, want 2", len(subEntries))
	}

	// Verify we got the correct entries
	names := make(map[string]bool)
	for _, e := range subEntries {
		names[e.Name] = true
	}

	if !names["nvim-config"] {
		t.Error("Expected nvim-config to be included")
	}

	if !names["gitconfig"] {
		t.Error("Expected gitconfig to be included")
	}

	if names["vscode-config"] {
		t.Error("Expected vscode-config to be excluded on Linux")
	}

	// Test with Windows context - should get sub-entries from vscode and git apps
	windowsCtx := &FilterContext{OS: "windows", Hostname: "desktop", User: "john"}

	windowsSubEntries := cfg.GetAllSubEntries(windowsCtx)
	if len(windowsSubEntries) != 2 {
		t.Errorf("GetAllSubEntries(windows) returned %d sub-entries, want 2", len(windowsSubEntries))
	}

	windowsNames := make(map[string]bool)
	for _, e := range windowsSubEntries {
		windowsNames[e.Name] = true
	}

	if !windowsNames["vscode-config"] {
		t.Error("Expected vscode-config to be included on Windows")
	}

	if !windowsNames["gitconfig"] {
		t.Error("Expected gitconfig to be included (no filter)")
	}

	// Test with empty applications
	emptyConfig := &Config{Version: 3, Applications: []Application{}}
	emptyCtx := &FilterContext{OS: "linux", Hostname: "laptop", User: "john"}

	emptySubEntries := emptyConfig.GetAllSubEntries(emptyCtx)
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
				Filters: []Filter{
					{Include: map[string]string{"os": "linux"}},
				},
				Entries: []SubEntry{
					{Name: "nvim-config", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim"}},
					{Name: "nvim-local", Files: []string{"local.lua"}, Backup: "./nvim-local", Targets: map[string]string{"linux": "~/.config/nvim/lua"}},
				},
			},
			{
				Name:        "zsh",
				Description: "Zsh configuration",
				Filters: []Filter{
					{Include: map[string]string{"os": "linux"}},
				},
				Entries: []SubEntry{
					{Name: "zshrc", Backup: "./zsh", Targets: map[string]string{"linux": "~/.zshrc"}},
				},
			},
			{
				Name:        "vscode",
				Description: "Code editor",
				Filters: []Filter{
					{Include: map[string]string{"os": "windows"}},
				},
				Entries: []SubEntry{
					{Name: "vscode-config", Backup: "./vscode", Targets: map[string]string{"windows": "~/AppData/Roaming/Code"}},
				},
			},
		},
	}

	// Test with Linux context - should get config sub-entries from neovim and zsh
	linuxCtx := &FilterContext{OS: "linux", Hostname: "laptop", User: "john"}

	configSubEntries := cfg.GetAllConfigSubEntries(linuxCtx)
	if len(configSubEntries) != 3 {
		t.Errorf("GetAllConfigSubEntries(linux) returned %d sub-entries, want 3", len(configSubEntries))
	}

	// Verify we only got config type entries
	for _, e := range configSubEntries {
		if !e.IsConfig() {
			t.Errorf("GetAllConfigSubEntries returned non-config entry: %s", e.Name)
		}
	}

	names := make(map[string]bool)
	for _, e := range configSubEntries {
		names[e.Name] = true
	}

	if !names["nvim-config"] {
		t.Error("Expected nvim-config to be included")
	}

	if !names["nvim-local"] {
		t.Error("Expected nvim-local to be included")
	}

	if !names["zshrc"] {
		t.Error("Expected zshrc to be included")
	}

	// Test with Windows context - should get vscode-config only
	windowsCtx := &FilterContext{OS: "windows", Hostname: "desktop", User: "john"}

	windowsConfigSubEntries := cfg.GetAllConfigSubEntries(windowsCtx)
	if len(windowsConfigSubEntries) != 1 {
		t.Errorf("GetAllConfigSubEntries(windows) returned %d sub-entries, want 1", len(windowsConfigSubEntries))
	}

	if windowsConfigSubEntries[0].Name != "vscode-config" {
		t.Errorf("Expected vscode-config, got %s", windowsConfigSubEntries[0].Name)
	}

	// Test with no matching entries (all filtered out)
	darwinCtx := &FilterContext{OS: "darwin", Hostname: "mac", User: "john"}

	darwinConfigSubEntries := cfg.GetAllConfigSubEntries(darwinCtx)
	if len(darwinConfigSubEntries) != 0 {
		t.Errorf("GetAllConfigSubEntries(darwin) returned %d sub-entries, want 0", len(darwinConfigSubEntries))
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
