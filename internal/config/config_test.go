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
version: 2
backup_root: "~/gits/configurations"

entries:
  - name: "neovim"
    files: []
    backup: "./Both/Neovim/nvim"
    targets:
      linux: "~/.config/nvim"
      windows: "~/AppData/Local/nvim"

  - name: "bash"
    files: [".bashrc", ".bash_profile"]
    backup: "./Linux/Bash"
    targets:
      linux: "~"

  - name: "pacman-hooks"
    sudo: true
    files: ["pkg-backup.hook"]
    backup: "./Linux/pacman"
    targets:
      linux: "/etc/pacman.d/hooks"
`

	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Test version
	if cfg.Version != 2 {
		t.Errorf("Version = %d, want 2", cfg.Version)
	}

	// Test backup root
	if cfg.BackupRoot != "~/gits/configurations" {
		t.Errorf("BackupRoot = %q, want %q", cfg.BackupRoot, "~/gits/configurations")
	}

	// Test entries count
	if len(cfg.Entries) != 3 {
		t.Errorf("len(Entries) = %d, want 3", len(cfg.Entries))
	}

	// Test first entry (neovim)
	if cfg.Entries[0].Name != "neovim" {
		t.Errorf("Entries[0].Name = %q, want %q", cfg.Entries[0].Name, "neovim")
	}

	if !cfg.Entries[0].IsFolder() {
		t.Error("Entries[0].IsFolder() = false, want true")
	}

	// Test second entry (bash)
	if cfg.Entries[1].Name != "bash" {
		t.Errorf("Entries[1].Name = %q, want %q", cfg.Entries[1].Name, "bash")
	}

	if cfg.Entries[1].IsFolder() {
		t.Error("Entries[1].IsFolder() = true, want false")
	}

	if len(cfg.Entries[1].Files) != 2 {
		t.Errorf("len(Entries[1].Files) = %d, want 2", len(cfg.Entries[1].Files))
	}

	// Test third entry (root entry)
	if cfg.Entries[2].Name != "pacman-hooks" {
		t.Errorf("Entries[2].Name = %q, want %q", cfg.Entries[2].Name, "pacman-hooks")
	}

	if !cfg.Entries[2].Sudo {
		t.Error("Entries[2].Sudo = false, want true")
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

	if err := os.WriteFile(configPath, []byte("invalid: yaml: content:"), 0644); err != nil {
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
version: 1
backup_root: "~/dotfiles"
entries: []
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	_, err := Load(configPath)
	if err == nil {
		t.Error("Load() expected error for unsupported version 1")
	}
}

func TestPathSpecGetTarget(t *testing.T) {
	t.Parallel()
	spec := PathSpec{
		Name: "test",
		Targets: map[string]string{
			"linux":   "/home/user/.config",
			"windows": "C:\\Users\\user",
		},
	}

	tests := []struct {
		osType string
		want   string
	}{
		{"linux", "/home/user/.config"},
		{"windows", "C:\\Users\\user"},
		{"darwin", ""},
	}

	for _, tt := range tests {
		t.Run(tt.osType, func(t *testing.T) {
			t.Parallel()
			got := spec.GetTarget(tt.osType)
			if got != tt.want {
				t.Errorf("GetTarget(%q) = %q, want %q", tt.osType, got, tt.want)
			}
		})
	}
}

func TestPathSpecIsFolder(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name  string
		files []string
		want  bool
	}{
		{"empty files", []string{}, true},
		{"nil files", nil, true},
		{"with files", []string{"file1", "file2"}, false},
		{"single file", []string{"file"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			spec := PathSpec{Files: tt.files}
			if got := spec.IsFolder(); got != tt.want {
				t.Errorf("IsFolder() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExpandPaths(t *testing.T) {
	t.Parallel()
	cfg := &Config{
		Version:    2,
		BackupRoot: "~/gits/configs",
		Entries: []Entry{
			{
				Name:   "test",
				Backup: "./test",
				Files:  []string{"$CUSTOM_VAR"},
				Targets: map[string]string{
					"linux": "~/.config/test",
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
	if cfg.Entries[0].Files[0] != "expanded_value" {
		t.Errorf("Files[0] = %q, want %q", cfg.Entries[0].Files[0], "expanded_value")
	}

	// Test target expansion
	expectedTarget := filepath.Join(home, ".config/test")
	if cfg.Entries[0].Targets["linux"] != expectedTarget {
		t.Errorf("Targets[linux] = %q, want %q", cfg.Entries[0].Targets["linux"], expectedTarget)
	}
}

func TestSave(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	cfg := &Config{
		Version:    2,
		BackupRoot: "/home/user/dotfiles",
		Entries: []Entry{
			{
				Name:   "neovim",
				Files:  []string{},
				Backup: "./nvim",
				Targets: map[string]string{
					"linux":   "~/.config/nvim",
					"windows": "~/AppData/Local/nvim",
				},
			},
			{
				Name:   "pacman",
				Sudo:   true,
				Files:  []string{"hook.conf"},
				Backup: "./pacman",
				Targets: map[string]string{
					"linux": "/etc/pacman.d/hooks",
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

	if len(loaded.Entries) != len(cfg.Entries) {
		t.Errorf("len(Entries) = %d, want %d", len(loaded.Entries), len(cfg.Entries))
	}

	if loaded.Entries[0].Name != cfg.Entries[0].Name {
		t.Errorf("Entries[0].Name = %q, want %q", loaded.Entries[0].Name, cfg.Entries[0].Name)
	}
}

func TestSaveToNonexistentDirectory(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent", "subdir", "config.yaml")

	cfg := &Config{Version: 2}

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
entries: []
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	// Should default to version 2
	if cfg.Version != 2 {
		t.Errorf("Version = %d, want 2 (default)", cfg.Version)
	}
}

func TestPathSpecGetTargetEmptyTargets(t *testing.T) {
	t.Parallel()
	spec := PathSpec{
		Name:    "test",
		Targets: map[string]string{},
	}

	got := spec.GetTarget("linux")
	if got != "" {
		t.Errorf("GetTarget() = %q, want empty string", got)
	}
}

func TestPathSpecGetTargetNilTargets(t *testing.T) {
	t.Parallel()
	spec := PathSpec{
		Name:    "test",
		Targets: nil,
	}

	got := spec.GetTarget("linux")
	if got != "" {
		t.Errorf("GetTarget() = %q, want empty string", got)
	}
}

func TestLoadWithPackages(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
version: 2
backup_root: "~/dotfiles"
default_manager: "pacman"
manager_priority:
  - paru
  - yay
  - pacman

entries:
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
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
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

	if len(cfg.Entries) != 1 {
		t.Errorf("len(Entries) = %d, want 1", len(cfg.Entries))
	}

	if cfg.Entries[0].Name != "neovim" {
		t.Errorf("Entries[0].Name = %q, want %q", cfg.Entries[0].Name, "neovim")
	}

	if len(cfg.Entries[0].Filters) != 1 {
		t.Errorf("len(Filters) = %d, want 1", len(cfg.Entries[0].Filters))
	}

	if cfg.Entries[0].Filters[0].Include["os"] != "linux" {
		t.Errorf("Filters[0].Include[os] = %q, want %q", cfg.Entries[0].Filters[0].Include["os"], "linux")
	}

	if cfg.Entries[0].Package == nil {
		t.Fatal("Entries[0].Package is nil, want non-nil")
	}

	if cfg.Entries[0].Package.Managers["pacman"] != "neovim" {
		t.Errorf("Package.Managers[pacman] = %q, want %q", cfg.Entries[0].Package.Managers["pacman"], "neovim")
	}
}

func TestExpandPathOnlyTilde(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Version:    2,
		BackupRoot: "~",
		Entries:    []Entry{},
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
		Version:    2,
		BackupRoot: "",
		Entries:    []Entry{},
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
version: 2
backup_root: "~/dotfiles"

entries:
  - name: custom-tool
    package:
      url:
        linux:
          url: "https://example.com/tool.tar.gz"
          command: "tar xzf {file} -C /usr/local/bin"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(cfg.Entries))
	}

	if cfg.Entries[0].Package == nil {
		t.Fatal("Package is nil")
	}

	urlSpec := cfg.Entries[0].Package.URL["linux"]
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
version: 2
backup_root: "~/dotfiles"

entries:
  - name: custom-tool
    package:
      custom:
        linux: "curl -fsSL https://example.com/install.sh | bash"
        windows: "iwr https://example.com/install.ps1 | iex"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(cfg.Entries))
	}

	if cfg.Entries[0].Package == nil {
		t.Fatal("Package is nil")
	}

	custom := cfg.Entries[0].Package.Custom
	if custom["linux"] != "curl -fsSL https://example.com/install.sh | bash" {
		t.Errorf("Custom[linux] = %q", custom["linux"])
	}
}

func TestLoadWithGitEntry(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
version: 2
backup_root: "~/dotfiles"
entries:
  - name: "oh-my-zsh"
    repo: "https://github.com/ohmyzsh/ohmyzsh.git"
    branch: "master"
    sudo: true
    targets:
      linux: "/usr/share/oh-my-zsh"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	cfg, err := Load(configPath)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if len(cfg.Entries) != 1 {
		t.Fatalf("len(Entries) = %d, want 1", len(cfg.Entries))
	}

	entry := cfg.Entries[0]
	if entry.Name != "oh-my-zsh" {
		t.Errorf("Name = %q, want %q", entry.Name, "oh-my-zsh")
	}
	if entry.Repo != "https://github.com/ohmyzsh/ohmyzsh.git" {
		t.Errorf("Repo = %q", entry.Repo)
	}
	if entry.Branch != "master" {
		t.Errorf("Branch = %q, want %q", entry.Branch, "master")
	}
	if !entry.Sudo {
		t.Error("Sudo = false, want true")
	}
	if !entry.IsGit() {
		t.Error("IsGit() = false, want true")
	}
	if entry.IsConfig() {
		t.Error("IsConfig() = true, want false")
	}
}

func TestGetGitEntries(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Version: 2,
		Entries: []Entry{
			{Name: "neovim", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim"}},
			{Name: "oh-my-zsh", Repo: "https://github.com/ohmyzsh/ohmyzsh.git", Sudo: true, Targets: map[string]string{"linux": "/usr/share/oh-my-zsh"}},
			{Name: "fzf", Repo: "https://github.com/junegunn/fzf.git", Targets: map[string]string{"linux": "~/.fzf"}},
		},
	}

	// Test getting all git entries (both root and non-root)
	entries := cfg.GetGitEntries()
	if len(entries) != 2 {
		t.Errorf("GetGitEntries() returned %d entries, want 2", len(entries))
	}

	// Check both entries are present
	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}
	if !names["fzf"] {
		t.Error("GetGitEntries() should include 'fzf'")
	}
	if !names["oh-my-zsh"] {
		t.Error("GetGitEntries() should include 'oh-my-zsh'")
	}
}

func TestGetConfigEntries(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Version: 2,
		Entries: []Entry{
			{Name: "neovim", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim"}},
			{Name: "pacman", Sudo: true, Backup: "./pacman", Targets: map[string]string{"linux": "/etc/pacman.d"}},
			{Name: "ripgrep", Package: &EntryPackage{Managers: map[string]string{"pacman": "ripgrep"}}},
		},
	}

	// Test getting all config entries (both root and non-root)
	entries := cfg.GetConfigEntries()
	if len(entries) != 2 {
		t.Errorf("GetConfigEntries() returned %d entries, want 2", len(entries))
	}

	// Check both entries are present
	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}
	if !names["neovim"] {
		t.Error("GetConfigEntries() should include 'neovim'")
	}
	if !names["pacman"] {
		t.Error("GetConfigEntries() should include 'pacman'")
	}
}

func TestGetPackageEntries(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Version: 2,
		Entries: []Entry{
			{Name: "neovim", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim"}},
			{Name: "ripgrep", Package: &EntryPackage{Managers: map[string]string{"pacman": "ripgrep"}}},
			{Name: "both", Backup: "./both", Targets: map[string]string{"linux": "~/.both"}, Package: &EntryPackage{Managers: map[string]string{"pacman": "both"}}},
		},
	}

	entries := cfg.GetPackageEntries()
	if len(entries) != 2 {
		t.Errorf("GetPackageEntries() returned %d entries, want 2", len(entries))
	}

	// Should include both ripgrep and "both" entries
	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}
	if !names["ripgrep"] {
		t.Error("GetPackageEntries() should include 'ripgrep'")
	}
	if !names["both"] {
		t.Error("GetPackageEntries() should include 'both'")
	}
}

func TestValidateGitEntry(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		entry   Entry
		wantErr bool
	}{
		{
			name: "valid git entry",
			entry: Entry{
				Name: "plugin",
				Repo: "https://github.com/test/plugin.git",
				Targets: map[string]string{
					"linux": "~/.plugins/test",
				},
			},
			wantErr: false,
		},
		{
			name: "valid git entry with branch",
			entry: Entry{
				Name:   "plugin",
				Repo:   "https://github.com/test/plugin.git",
				Branch: "develop",
				Targets: map[string]string{
					"linux": "~/.plugins/test",
				},
			},
			wantErr: false,
		},
		{
			name: "git entry missing targets",
			entry: Entry{
				Name: "plugin",
				Repo: "https://github.com/test/plugin.git",
			},
			wantErr: true,
		},
		{
			name: "git entry with empty target",
			entry: Entry{
				Name: "plugin",
				Repo: "https://github.com/test/plugin.git",
				Targets: map[string]string{
					"linux": "",
				},
			},
			wantErr: true,
		},
		{
			name: "entry with both backup and repo",
			entry: Entry{
				Name:   "invalid",
				Backup: "./backup",
				Repo:   "https://github.com/test/repo.git",
				Targets: map[string]string{
					"linux": "~/.test",
				},
			},
			wantErr: true,
		},
		{
			name: "entry with neither backup, repo, nor package",
			entry: Entry{
				Name: "empty",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateEntry(&tt.entry)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEntry() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestEntryIsConfigAndIsGit(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		entry    Entry
		isConfig bool
		isGit    bool
	}{
		{
			name:     "config entry",
			entry:    Entry{Name: "nvim", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim"}},
			isConfig: true,
			isGit:    false,
		},
		{
			name:     "git entry",
			entry:    Entry{Name: "plugin", Repo: "https://github.com/test/repo.git", Targets: map[string]string{"linux": "~/.plugins"}},
			isConfig: false,
			isGit:    true,
		},
		{
			name:     "package-only entry",
			entry:    Entry{Name: "ripgrep", Package: &EntryPackage{Managers: map[string]string{"pacman": "ripgrep"}}},
			isConfig: false,
			isGit:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := tt.entry.IsConfig(); got != tt.isConfig {
				t.Errorf("IsConfig() = %v, want %v", got, tt.isConfig)
			}
			if got := tt.entry.IsGit(); got != tt.isGit {
				t.Errorf("IsGit() = %v, want %v", got, tt.isGit)
			}
		})
	}
}

func TestGetFilteredConfigEntries(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Version: 2,
		Entries: []Entry{
			{Name: "linux-config", Backup: "./linux", Targets: map[string]string{"linux": "~/.config/linux"}, Filters: []Filter{{Include: map[string]string{"os": "linux"}}}},
			{Name: "windows-config", Backup: "./windows", Targets: map[string]string{"windows": "~/AppData"}, Filters: []Filter{{Include: map[string]string{"os": "windows"}}}},
			{Name: "all-config", Backup: "./all", Targets: map[string]string{"linux": "~/.config/all"}},
			{Name: "root-config", Sudo: true, Backup: "./root", Targets: map[string]string{"linux": "/etc/root"}},
		},
	}

	linuxCtx := &FilterContext{OS: "linux", Hostname: "desktop", User: "john"}

	// Test entries on Linux (includes both root and non-root)
	entries := cfg.GetFilteredConfigEntries(linuxCtx)
	if len(entries) != 3 {
		t.Errorf("GetFilteredConfigEntries(linux) returned %d entries, want 3", len(entries))
	}

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}
	if !names["linux-config"] {
		t.Error("Expected linux-config to be included")
	}
	if !names["all-config"] {
		t.Error("Expected all-config to be included")
	}
	if !names["root-config"] {
		t.Error("Expected root-config to be included")
	}
	if names["windows-config"] {
		t.Error("Expected windows-config to be excluded")
	}

	// Test with Windows context (windows-config, all-config, root-config pass filters)
	windowsCtx := &FilterContext{OS: "windows", Hostname: "desktop", User: "john"}
	windowsEntries := cfg.GetFilteredConfigEntries(windowsCtx)
	if len(windowsEntries) != 3 {
		t.Errorf("GetFilteredConfigEntries(windows) returned %d entries, want 3", len(windowsEntries))
	}
}

func TestGetFilteredGitEntries(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Version: 2,
		Entries: []Entry{
			{Name: "linux-repo", Repo: "https://github.com/test/linux.git", Targets: map[string]string{"linux": "~/.linux"}, Filters: []Filter{{Include: map[string]string{"os": "linux"}}}},
			{Name: "all-repo", Repo: "https://github.com/test/all.git", Targets: map[string]string{"linux": "~/.all"}},
		},
	}

	linuxCtx := &FilterContext{OS: "linux", Hostname: "desktop", User: "john"}
	entries := cfg.GetFilteredGitEntries(linuxCtx)
	if len(entries) != 2 {
		t.Errorf("GetFilteredGitEntries(linux) returned %d entries, want 2", len(entries))
	}

	windowsCtx := &FilterContext{OS: "windows", Hostname: "desktop", User: "john"}
	windowsEntries := cfg.GetFilteredGitEntries(windowsCtx)
	if len(windowsEntries) != 1 {
		t.Errorf("GetFilteredGitEntries(windows) returned %d entries, want 1", len(windowsEntries))
	}
	if windowsEntries[0].Name != "all-repo" {
		t.Errorf("Expected all-repo, got %s", windowsEntries[0].Name)
	}
}

func TestGetFilteredPackageEntries(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Version: 2,
		Entries: []Entry{
			{Name: "linux-pkg", Package: &EntryPackage{Managers: map[string]string{"pacman": "linux-pkg"}}, Filters: []Filter{{Include: map[string]string{"os": "linux"}}}},
			{Name: "work-pkg", Package: &EntryPackage{Managers: map[string]string{"pacman": "work-pkg"}}, Filters: []Filter{{Include: map[string]string{"hostname": "work-.*"}}}},
			{Name: "non-root-pkg", Package: &EntryPackage{Managers: map[string]string{"pacman": "non-root"}}, Filters: []Filter{{Exclude: map[string]string{"user": "root"}}}},
			{Name: "all-pkg", Package: &EntryPackage{Managers: map[string]string{"pacman": "all-pkg"}}},
		},
	}

	// Test with linux context on work-laptop as non-root
	linuxCtx := &FilterContext{OS: "linux", Hostname: "work-laptop", User: "john"}
	entries := cfg.GetFilteredPackageEntries(linuxCtx)
	if len(entries) != 4 {
		t.Errorf("GetFilteredPackageEntries(linux, work-laptop, john) returned %d entries, want 4", len(entries))
	}

	// Test with linux context on home-desktop as root
	rootCtx := &FilterContext{OS: "linux", Hostname: "home-desktop", User: "root"}
	rootEntries := cfg.GetFilteredPackageEntries(rootCtx)
	if len(rootEntries) != 2 {
		t.Errorf("GetFilteredPackageEntries(linux, home-desktop, root) returned %d entries, want 2", len(rootEntries))
	}

	names := make(map[string]bool)
	for _, e := range rootEntries {
		names[e.Name] = true
	}
	if !names["linux-pkg"] {
		t.Error("Expected linux-pkg to be included")
	}
	if !names["all-pkg"] {
		t.Error("Expected all-pkg to be included")
	}
	if names["non-root-pkg"] {
		t.Error("Expected non-root-pkg to be excluded for root user")
	}
	if names["work-pkg"] {
		t.Error("Expected work-pkg to be excluded for home-desktop")
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

  - name: "oh-my-zsh"
    description: "Zsh framework"
    entries:
      - type: "git"
        name: "oh-my-zsh-repo"
        repo: "https://github.com/ohmyzsh/ohmyzsh.git"
        branch: "master"
        sudo: true
        targets:
          linux: "/usr/share/oh-my-zsh"
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
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
	if subEntry1.Type != "config" {
		t.Errorf("SubEntry[0].Type = %q, want %q", subEntry1.Type, "config")
	}
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
	if subEntry2.Type != "config" {
		t.Errorf("SubEntry[1].Type = %q, want %q", subEntry2.Type, "config")
	}
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

	// Test second application (oh-my-zsh)
	app2 := cfg.Applications[1]
	if app2.Name != "oh-my-zsh" {
		t.Errorf("Applications[1].Name = %q, want %q", app2.Name, "oh-my-zsh")
	}
	if len(app2.Entries) != 1 {
		t.Fatalf("len(Applications[1].Entries) = %d, want 1", len(app2.Entries))
	}

	// Test git sub-entry
	gitEntry := app2.Entries[0]
	if gitEntry.Type != "git" {
		t.Errorf("GitEntry.Type = %q, want %q", gitEntry.Type, "git")
	}
	if !gitEntry.IsGit() {
		t.Error("GitEntry.IsGit() = false, want true")
	}
	if gitEntry.Repo != "https://github.com/ohmyzsh/ohmyzsh.git" {
		t.Errorf("GitEntry.Repo = %q", gitEntry.Repo)
	}
	if gitEntry.Branch != "master" {
		t.Errorf("GitEntry.Branch = %q, want %q", gitEntry.Branch, "master")
	}
	if !gitEntry.Sudo {
		t.Error("GitEntry.Sudo = false, want true")
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
					{Type: "config", Name: "nvim-config", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim"}},
				},
				Package: &EntryPackage{Managers: map[string]string{"pacman": "neovim"}},
			},
			{
				Name:        "vscode",
				Description: "Code editor",
				Filters: []Filter{
					{Include: map[string]string{"os": "windows"}},
				},
				Entries: []SubEntry{
					{Type: "config", Name: "vscode-config", Backup: "./vscode", Targets: map[string]string{"windows": "~/AppData/Roaming/Code"}},
				},
			},
			{
				Name:        "git",
				Description: "Version control",
				Entries: []SubEntry{
					{Type: "config", Name: "gitconfig", Files: []string{".gitconfig"}, Backup: "./git", Targets: map[string]string{"linux": "~", "windows": "~"}},
				},
			},
			{
				Name:        "work-only",
				Description: "Work tools",
				Filters: []Filter{
					{Include: map[string]string{"hostname": "work-.*"}},
				},
				Entries: []SubEntry{
					{Type: "config", Name: "work-config", Backup: "./work", Targets: map[string]string{"linux": "~/.work"}},
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
						Type:   "config",
						Name:   "nvim-config",
						Backup: "./nvim",
						Files:  []string{"$CUSTOM_VAR"},
						Targets: map[string]string{
							"linux": "~/.config/nvim",
						},
					},
					{
						Type: "git",
						Name: "nvim-plugin",
						Repo: "https://github.com/test/plugin.git",
						Targets: map[string]string{
							"linux": "~/.local/share/nvim/site/pack/plugins",
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

	// Test target expansion for git entry
	expectedGitTarget := filepath.Join(home, ".local/share/nvim/site/pack/plugins")
	if cfg.Applications[0].Entries[1].Targets["linux"] != expectedGitTarget {
		t.Errorf("Git SubEntry Targets[linux] = %q, want %q", cfg.Applications[0].Entries[1].Targets["linux"], expectedGitTarget)
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
					{Type: "config", Name: "nvim-config", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim"}},
					{Type: "git", Name: "nvim-plugin", Repo: "https://github.com/test/plugin.git", Targets: map[string]string{"linux": "~/.local/share/nvim"}},
				},
			},
			{
				Name:        "vscode",
				Description: "Code editor",
				Filters: []Filter{
					{Include: map[string]string{"os": "windows"}},
				},
				Entries: []SubEntry{
					{Type: "config", Name: "vscode-config", Backup: "./vscode", Targets: map[string]string{"windows": "~/AppData/Roaming/Code"}},
				},
			},
			{
				Name:        "git",
				Description: "Version control",
				Entries: []SubEntry{
					{Type: "config", Name: "gitconfig", Files: []string{".gitconfig"}, Backup: "./git", Targets: map[string]string{"linux": "~"}},
				},
			},
		},
	}

	// Test with Linux context - should get all sub-entries from neovim and git apps
	linuxCtx := &FilterContext{OS: "linux", Hostname: "laptop", User: "john"}
	subEntries := cfg.GetAllSubEntries(linuxCtx)
	if len(subEntries) != 3 {
		t.Errorf("GetAllSubEntries(linux) returned %d sub-entries, want 3", len(subEntries))
	}

	// Verify we got the correct entries
	names := make(map[string]bool)
	for _, e := range subEntries {
		names[e.Name] = true
	}
	if !names["nvim-config"] {
		t.Error("Expected nvim-config to be included")
	}
	if !names["nvim-plugin"] {
		t.Error("Expected nvim-plugin to be included")
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
					{Type: "config", Name: "nvim-config", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim"}},
					{Type: "git", Name: "nvim-plugin", Repo: "https://github.com/test/plugin.git", Targets: map[string]string{"linux": "~/.local/share/nvim"}},
					{Type: "config", Name: "nvim-local", Files: []string{"local.lua"}, Backup: "./nvim-local", Targets: map[string]string{"linux": "~/.config/nvim/lua"}},
				},
			},
			{
				Name:        "oh-my-zsh",
				Description: "Zsh framework",
				Filters: []Filter{
					{Include: map[string]string{"os": "linux"}},
				},
				Entries: []SubEntry{
					{Type: "git", Name: "oh-my-zsh-repo", Repo: "https://github.com/ohmyzsh/ohmyzsh.git", Targets: map[string]string{"linux": "/usr/share/oh-my-zsh"}},
				},
			},
			{
				Name:        "vscode",
				Description: "Code editor",
				Filters: []Filter{
					{Include: map[string]string{"os": "windows"}},
				},
				Entries: []SubEntry{
					{Type: "config", Name: "vscode-config", Backup: "./vscode", Targets: map[string]string{"windows": "~/AppData/Roaming/Code"}},
				},
			},
		},
	}

	// Test with Linux context - should only get config type sub-entries from neovim
	linuxCtx := &FilterContext{OS: "linux", Hostname: "laptop", User: "john"}
	configSubEntries := cfg.GetAllConfigSubEntries(linuxCtx)
	if len(configSubEntries) != 2 {
		t.Errorf("GetAllConfigSubEntries(linux) returned %d sub-entries, want 2", len(configSubEntries))
	}

	// Verify we only got config type entries
	for _, e := range configSubEntries {
		if !e.IsConfig() {
			t.Errorf("GetAllConfigSubEntries returned non-config entry: %s (type: %s)", e.Name, e.Type)
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
	if names["nvim-plugin"] {
		t.Error("Expected nvim-plugin (git type) to be excluded")
	}
	if names["oh-my-zsh-repo"] {
		t.Error("Expected oh-my-zsh-repo (git type) to be excluded")
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

func TestGetAllGitSubEntries(t *testing.T) {
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
					{Type: "config", Name: "nvim-config", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim"}},
					{Type: "git", Name: "nvim-plugin", Repo: "https://github.com/test/plugin.git", Targets: map[string]string{"linux": "~/.local/share/nvim"}},
				},
			},
			{
				Name:        "oh-my-zsh",
				Description: "Zsh framework",
				Filters: []Filter{
					{Include: map[string]string{"os": "linux"}},
				},
				Entries: []SubEntry{
					{Type: "git", Name: "oh-my-zsh-repo", Repo: "https://github.com/ohmyzsh/ohmyzsh.git", Branch: "master", Sudo: true, Targets: map[string]string{"linux": "/usr/share/oh-my-zsh"}},
				},
			},
			{
				Name:        "windows-tools",
				Description: "Windows utilities",
				Filters: []Filter{
					{Include: map[string]string{"os": "windows"}},
				},
				Entries: []SubEntry{
					{Type: "git", Name: "windows-repo", Repo: "https://github.com/test/windows.git", Targets: map[string]string{"windows": "~/tools"}},
					{Type: "config", Name: "windows-config", Backup: "./windows", Targets: map[string]string{"windows": "~/config"}},
				},
			},
		},
	}

	// Test with Linux context - should only get git type sub-entries from neovim and oh-my-zsh
	linuxCtx := &FilterContext{OS: "linux", Hostname: "laptop", User: "john"}
	gitSubEntries := cfg.GetAllGitSubEntries(linuxCtx)
	if len(gitSubEntries) != 2 {
		t.Errorf("GetAllGitSubEntries(linux) returned %d sub-entries, want 2", len(gitSubEntries))
	}

	// Verify we only got git type entries
	for _, e := range gitSubEntries {
		if !e.IsGit() {
			t.Errorf("GetAllGitSubEntries returned non-git entry: %s (type: %s)", e.Name, e.Type)
		}
	}

	names := make(map[string]bool)
	for _, e := range gitSubEntries {
		names[e.Name] = true
	}
	if !names["nvim-plugin"] {
		t.Error("Expected nvim-plugin to be included")
	}
	if !names["oh-my-zsh-repo"] {
		t.Error("Expected oh-my-zsh-repo to be included")
	}
	if names["nvim-config"] {
		t.Error("Expected nvim-config (config type) to be excluded")
	}

	// Test sudo flag is preserved
	for _, e := range gitSubEntries {
		if e.Name == "oh-my-zsh-repo" && !e.Sudo {
			t.Error("Expected oh-my-zsh-repo to have Sudo=true")
		}
	}

	// Test with Windows context - should get windows-repo only
	windowsCtx := &FilterContext{OS: "windows", Hostname: "desktop", User: "john"}
	windowsGitSubEntries := cfg.GetAllGitSubEntries(windowsCtx)
	if len(windowsGitSubEntries) != 1 {
		t.Errorf("GetAllGitSubEntries(windows) returned %d sub-entries, want 1", len(windowsGitSubEntries))
	}
	if windowsGitSubEntries[0].Name != "windows-repo" {
		t.Errorf("Expected windows-repo, got %s", windowsGitSubEntries[0].Name)
	}

	// Test with no matching entries (all filtered out)
	darwinCtx := &FilterContext{OS: "darwin", Hostname: "mac", User: "john"}
	darwinGitSubEntries := cfg.GetAllGitSubEntries(darwinCtx)
	if len(darwinGitSubEntries) != 0 {
		t.Errorf("GetAllGitSubEntries(darwin) returned %d sub-entries, want 0", len(darwinGitSubEntries))
	}
}
