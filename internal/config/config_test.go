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
    root: true
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

	if !cfg.Entries[2].Root {
		t.Error("Entries[2].Root = false, want true")
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
				Root:   true,
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
    root: true
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
	if !entry.Root {
		t.Error("Root = false, want true")
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
			{Name: "oh-my-zsh", Repo: "https://github.com/ohmyzsh/ohmyzsh.git", Root: true, Targets: map[string]string{"linux": "/usr/share/oh-my-zsh"}},
			{Name: "fzf", Repo: "https://github.com/junegunn/fzf.git", Targets: map[string]string{"linux": "~/.fzf"}},
		},
	}

	// Test getting non-root git entries
	entries := cfg.GetGitEntries(false)
	if len(entries) != 1 {
		t.Errorf("GetGitEntries(false) returned %d entries, want 1", len(entries))
	}
	if entries[0].Name != "fzf" {
		t.Errorf("GetGitEntries(false)[0].Name = %q, want %q", entries[0].Name, "fzf")
	}

	// Test getting root git entries
	rootEntries := cfg.GetGitEntries(true)
	if len(rootEntries) != 1 {
		t.Errorf("GetGitEntries(true) returned %d entries, want 1", len(rootEntries))
	}
	if rootEntries[0].Name != "oh-my-zsh" {
		t.Errorf("GetGitEntries(true)[0].Name = %q, want %q", rootEntries[0].Name, "oh-my-zsh")
	}
}

func TestGetConfigEntries(t *testing.T) {
	t.Parallel()

	cfg := &Config{
		Version: 2,
		Entries: []Entry{
			{Name: "neovim", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim"}},
			{Name: "pacman", Root: true, Backup: "./pacman", Targets: map[string]string{"linux": "/etc/pacman.d"}},
			{Name: "ripgrep", Package: &EntryPackage{Managers: map[string]string{"pacman": "ripgrep"}}},
		},
	}

	// Test getting non-root entries
	entries := cfg.GetConfigEntries(false)
	if len(entries) != 1 {
		t.Errorf("GetConfigEntries(false) returned %d entries, want 1", len(entries))
	}
	if entries[0].Name != "neovim" {
		t.Errorf("GetConfigEntries(false)[0].Name = %q, want %q", entries[0].Name, "neovim")
	}

	// Test getting root entries
	rootEntries := cfg.GetConfigEntries(true)
	if len(rootEntries) != 1 {
		t.Errorf("GetConfigEntries(true) returned %d entries, want 1", len(rootEntries))
	}
	if rootEntries[0].Name != "pacman" {
		t.Errorf("GetConfigEntries(true)[0].Name = %q, want %q", rootEntries[0].Name, "pacman")
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
			{Name: "root-config", Root: true, Backup: "./root", Targets: map[string]string{"linux": "/etc/root"}},
		},
	}

	linuxCtx := &FilterContext{OS: "linux", Hostname: "desktop", User: "john"}

	// Test non-root entries on Linux
	entries := cfg.GetFilteredConfigEntries(false, linuxCtx)
	if len(entries) != 2 {
		t.Errorf("GetFilteredConfigEntries(false, linux) returned %d entries, want 2", len(entries))
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
	if names["windows-config"] {
		t.Error("Expected windows-config to be excluded")
	}

	// Test with Windows context
	windowsCtx := &FilterContext{OS: "windows", Hostname: "desktop", User: "john"}
	windowsEntries := cfg.GetFilteredConfigEntries(false, windowsCtx)
	if len(windowsEntries) != 2 {
		t.Errorf("GetFilteredConfigEntries(false, windows) returned %d entries, want 2", len(windowsEntries))
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
	entries := cfg.GetFilteredGitEntries(false, linuxCtx)
	if len(entries) != 2 {
		t.Errorf("GetFilteredGitEntries(false, linux) returned %d entries, want 2", len(entries))
	}

	windowsCtx := &FilterContext{OS: "windows", Hostname: "desktop", User: "john"}
	windowsEntries := cfg.GetFilteredGitEntries(false, windowsCtx)
	if len(windowsEntries) != 1 {
		t.Errorf("GetFilteredGitEntries(false, windows) returned %d entries, want 1", len(windowsEntries))
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
