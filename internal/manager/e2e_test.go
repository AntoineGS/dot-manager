package manager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

// TestE2E_RestoreFromExistingBackup tests restoring configs from an existing backup repository.
// This simulates: user clones their dotfiles repo and runs restore.
func TestE2E_RestoreFromExistingBackup(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Setup: Create a "dotfiles repository" with backed up configs
	repoDir := filepath.Join(tmpDir, "dotfiles-repo")
	homeDir := filepath.Join(tmpDir, "home")

	// Create backup structure (simulating cloned dotfiles repo)
	nvimBackup := filepath.Join(repoDir, "nvim")
	os.MkdirAll(nvimBackup, 0755)
	nvimConfig := "-- Neovim config from backup\nvim.opt.number = true\n"
	os.WriteFile(filepath.Join(nvimBackup, "init.lua"), []byte(nvimConfig), 0644)
	os.MkdirAll(filepath.Join(nvimBackup, "lua"), 0755)
	os.WriteFile(filepath.Join(nvimBackup, "lua", "plugins.lua"), []byte("-- plugins"), 0644)

	bashBackup := filepath.Join(repoDir, "bash")
	os.MkdirAll(bashBackup, 0755)
	bashrcContent := "# Bash config from backup\nexport PATH=$PATH:~/bin\n"
	os.WriteFile(filepath.Join(bashBackup, ".bashrc"), []byte(bashrcContent), 0644)
	os.WriteFile(filepath.Join(bashBackup, ".bash_profile"), []byte("source ~/.bashrc"), 0644)

	// Create config
	cfg := &config.Config{
		Version:    2,
		BackupRoot: repoDir,
		Entries: []config.Entry{
			{
				Name:   "neovim",
				Files:  []string{}, // folder mode
				Backup: "./nvim",
				Targets: map[string]string{
					"linux": filepath.Join(homeDir, ".config", "nvim"),
				},
			},
			{
				Name:   "bash",
				Files:  []string{".bashrc", ".bash_profile"}, // file mode
				Backup: "./bash",
				Targets: map[string]string{
					"linux": homeDir,
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	// Execute restore
	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() failed: %v", err)
	}

	// Verify: nvim target is a symlink pointing to backup
	nvimTarget := filepath.Join(homeDir, ".config", "nvim")
	if !isSymlink(nvimTarget) {
		t.Errorf("nvim target should be a symlink, but is not")
	}

	link, err := os.Readlink(nvimTarget)
	if err != nil {
		t.Fatalf("Failed to read nvim symlink: %v", err)
	}
	if link != nvimBackup {
		t.Errorf("nvim symlink points to %q, want %q", link, nvimBackup)
	}

	// Verify: can read files through the symlink
	initLua := filepath.Join(nvimTarget, "init.lua")
	content, err := os.ReadFile(initLua)
	if err != nil {
		t.Fatalf("Cannot read through nvim symlink: %v", err)
	}
	if string(content) != nvimConfig {
		t.Errorf("Content through symlink = %q, want %q", string(content), nvimConfig)
	}

	// Verify: nested files are accessible
	pluginsLua := filepath.Join(nvimTarget, "lua", "plugins.lua")
	if _, err := os.ReadFile(pluginsLua); err != nil {
		t.Errorf("Cannot read nested file through symlink: %v", err)
	}

	// Verify: .bashrc is a symlink
	bashrcTarget := filepath.Join(homeDir, ".bashrc")
	if !isSymlink(bashrcTarget) {
		t.Errorf(".bashrc should be a symlink, but is not")
	}

	// Verify: can read .bashrc through symlink
	content, err = os.ReadFile(bashrcTarget)
	if err != nil {
		t.Fatalf("Cannot read .bashrc through symlink: %v", err)
	}
	if string(content) != bashrcContent {
		t.Errorf(".bashrc content = %q, want %q", string(content), bashrcContent)
	}

	// Verify: .bash_profile is also a symlink
	bashProfileTarget := filepath.Join(homeDir, ".bash_profile")
	if !isSymlink(bashProfileTarget) {
		t.Errorf(".bash_profile should be a symlink, but is not")
	}
}

// TestE2E_AdoptExistingConfigs tests adopting existing configs that aren't backed up yet.
// This simulates: user has existing configs, clones empty dotfiles repo, runs restore.
func TestE2E_AdoptExistingConfigs(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Setup: User has existing configs
	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "dotfiles-repo")
	os.MkdirAll(repoDir, 0755)

	// User's existing nvim config
	nvimDir := filepath.Join(homeDir, ".config", "nvim")
	os.MkdirAll(nvimDir, 0755)
	originalNvimConfig := "-- My personal nvim config\n"
	os.WriteFile(filepath.Join(nvimDir, "init.lua"), []byte(originalNvimConfig), 0644)

	// User's existing bashrc
	originalBashrc := "# My personal bashrc\n"
	os.MkdirAll(homeDir, 0755)
	os.WriteFile(filepath.Join(homeDir, ".bashrc"), []byte(originalBashrc), 0644)

	// Config with empty backup locations
	cfg := &config.Config{
		Version:    2,
		BackupRoot: repoDir,
		Entries: []config.Entry{
			{
				Name:   "neovim",
				Files:  []string{},
				Backup: "./nvim",
				Targets: map[string]string{
					"linux": nvimDir,
				},
			},
			{
				Name:   "bash",
				Files:  []string{".bashrc"},
				Backup: "./bash",
				Targets: map[string]string{
					"linux": homeDir,
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	// Execute restore (which should adopt)
	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() failed: %v", err)
	}

	// Verify: nvim config was moved to backup
	nvimBackup := filepath.Join(repoDir, "nvim")
	backupInitLua := filepath.Join(nvimBackup, "init.lua")
	if !pathExists(backupInitLua) {
		t.Errorf("nvim config should have been adopted to backup")
	}

	content, _ := os.ReadFile(backupInitLua)
	if string(content) != originalNvimConfig {
		t.Errorf("Adopted nvim content = %q, want %q", string(content), originalNvimConfig)
	}

	// Verify: original nvim dir is now a symlink
	if !isSymlink(nvimDir) {
		t.Errorf("nvim dir should be a symlink after adopt")
	}

	// Verify: can still read the config through symlink
	content, err = os.ReadFile(filepath.Join(nvimDir, "init.lua"))
	if err != nil {
		t.Fatalf("Cannot read nvim config through symlink: %v", err)
	}
	if string(content) != originalNvimConfig {
		t.Errorf("Config through symlink = %q, want %q", string(content), originalNvimConfig)
	}

	// Verify: .bashrc was adopted
	bashBackup := filepath.Join(repoDir, "bash", ".bashrc")
	if !pathExists(bashBackup) {
		t.Errorf(".bashrc should have been adopted to backup")
	}

	// Verify: .bashrc is now a symlink
	bashrcPath := filepath.Join(homeDir, ".bashrc")
	if !isSymlink(bashrcPath) {
		t.Errorf(".bashrc should be a symlink after adopt")
	}
}

// TestE2E_BackupThenRestore tests the complete round-trip: backup existing configs, then restore them.
func TestE2E_BackupThenRestore(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	homeDir := filepath.Join(tmpDir, "home")
	repoDir := filepath.Join(tmpDir, "dotfiles-repo")

	// Setup: Create user configs
	nvimDir := filepath.Join(homeDir, ".config", "nvim")
	os.MkdirAll(nvimDir, 0755)
	nvimConfig := "vim.g.mapleader = ' '\n"
	os.WriteFile(filepath.Join(nvimDir, "init.lua"), []byte(nvimConfig), 0644)

	zshrcContent := "export EDITOR=nvim\n"
	os.MkdirAll(homeDir, 0755)
	os.WriteFile(filepath.Join(homeDir, ".zshrc"), []byte(zshrcContent), 0644)

	// Setup backup directory
	os.MkdirAll(filepath.Join(repoDir, "nvim"), 0755)
	os.MkdirAll(filepath.Join(repoDir, "zsh"), 0755)

	cfg := &config.Config{
		Version:    2,
		BackupRoot: repoDir,
		Entries: []config.Entry{
			{
				Name:   "neovim",
				Files:  []string{},
				Backup: "./nvim",
				Targets: map[string]string{
					"linux": nvimDir,
				},
			},
			{
				Name:   "zsh",
				Files:  []string{".zshrc"},
				Backup: "./zsh",
				Targets: map[string]string{
					"linux": homeDir,
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	// Step 1: Backup
	err := mgr.Backup()
	if err != nil {
		t.Fatalf("Backup() failed: %v", err)
	}

	// Verify backup was created
	backedUpInit := filepath.Join(repoDir, "nvim", "nvim", "init.lua")
	if !pathExists(backedUpInit) {
		t.Errorf("nvim backup should exist at %s", backedUpInit)
	}

	backedUpZshrc := filepath.Join(repoDir, "zsh", ".zshrc")
	if !pathExists(backedUpZshrc) {
		t.Errorf("zsh backup should exist at %s", backedUpZshrc)
	}

	// Step 2: Delete original configs (simulate fresh machine)
	os.RemoveAll(nvimDir)
	os.Remove(filepath.Join(homeDir, ".zshrc"))

	// Create a new manager for restore (simulating different config pointing to backed up folder)
	restoreCfg := &config.Config{
		Version:    2,
		BackupRoot: repoDir,
		Entries: []config.Entry{
			{
				Name:   "neovim",
				Files:  []string{},
				Backup: "./nvim/nvim", // Points to the backed up folder
				Targets: map[string]string{
					"linux": nvimDir,
				},
			},
			{
				Name:   "zsh",
				Files:  []string{".zshrc"},
				Backup: "./zsh",
				Targets: map[string]string{
					"linux": homeDir,
				},
			},
		},
	}

	restoreMgr := New(restoreCfg, plat)

	// Step 3: Restore
	err = restoreMgr.Restore()
	if err != nil {
		t.Fatalf("Restore() failed: %v", err)
	}

	// Verify: configs are accessible through symlinks
	if !isSymlink(nvimDir) {
		t.Errorf("nvim should be a symlink after restore")
	}

	content, err := os.ReadFile(filepath.Join(nvimDir, "init.lua"))
	if err != nil {
		t.Fatalf("Cannot read nvim config after restore: %v", err)
	}
	if string(content) != nvimConfig {
		t.Errorf("Restored nvim config = %q, want %q", string(content), nvimConfig)
	}

	zshrcPath := filepath.Join(homeDir, ".zshrc")
	if !isSymlink(zshrcPath) {
		t.Errorf(".zshrc should be a symlink after restore")
	}

	content, err = os.ReadFile(zshrcPath)
	if err != nil {
		t.Fatalf("Cannot read .zshrc after restore: %v", err)
	}
	if string(content) != zshrcContent {
		t.Errorf("Restored .zshrc = %q, want %q", string(content), zshrcContent)
	}
}

// TestE2E_RestoreIdempotent tests that running restore multiple times is safe.
func TestE2E_RestoreIdempotent(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")

	// Setup backup
	nvimBackup := filepath.Join(repoDir, "nvim")
	os.MkdirAll(nvimBackup, 0755)
	nvimConfig := "-- config\n"
	os.WriteFile(filepath.Join(nvimBackup, "init.lua"), []byte(nvimConfig), 0644)

	cfg := &config.Config{
		Version:    2,
		BackupRoot: repoDir,
		Entries: []config.Entry{
			{
				Name:   "neovim",
				Files:  []string{},
				Backup: "./nvim",
				Targets: map[string]string{
					"linux": filepath.Join(homeDir, ".config", "nvim"),
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	// Run restore multiple times
	for i := 0; i < 3; i++ {
		err := mgr.Restore()
		if err != nil {
			t.Fatalf("Restore() iteration %d failed: %v", i, err)
		}
	}

	// Verify symlink is correct
	nvimTarget := filepath.Join(homeDir, ".config", "nvim")
	if !isSymlink(nvimTarget) {
		t.Errorf("nvim should be a symlink")
	}

	link, _ := os.Readlink(nvimTarget)
	if link != nvimBackup {
		t.Errorf("Symlink target = %q, want %q", link, nvimBackup)
	}

	// Verify content is still accessible
	content, err := os.ReadFile(filepath.Join(nvimTarget, "init.lua"))
	if err != nil {
		t.Fatalf("Cannot read config: %v", err)
	}
	if string(content) != nvimConfig {
		t.Errorf("Content = %q, want %q", string(content), nvimConfig)
	}
}

// TestE2E_SymlinkModification tests that modifying files through symlinks works.
func TestE2E_SymlinkModification(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")

	// Setup
	nvimBackup := filepath.Join(repoDir, "nvim")
	os.MkdirAll(nvimBackup, 0755)
	originalConfig := "-- original\n"
	os.WriteFile(filepath.Join(nvimBackup, "init.lua"), []byte(originalConfig), 0644)

	cfg := &config.Config{
		Version:    2,
		BackupRoot: repoDir,
		Entries: []config.Entry{
			{
				Name:   "neovim",
				Files:  []string{},
				Backup: "./nvim",
				Targets: map[string]string{
					"linux": filepath.Join(homeDir, ".config", "nvim"),
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Restore()

	// Modify file through symlink
	nvimTarget := filepath.Join(homeDir, ".config", "nvim")
	modifiedConfig := "-- modified through symlink\n"
	err := os.WriteFile(filepath.Join(nvimTarget, "init.lua"), []byte(modifiedConfig), 0644)
	if err != nil {
		t.Fatalf("Failed to write through symlink: %v", err)
	}

	// Verify: backup file should be modified
	content, _ := os.ReadFile(filepath.Join(nvimBackup, "init.lua"))
	if string(content) != modifiedConfig {
		t.Errorf("Backup content should be modified, got %q, want %q", string(content), modifiedConfig)
	}

	// Create new file through symlink
	newFile := filepath.Join(nvimTarget, "new.lua")
	newContent := "-- new file\n"
	err = os.WriteFile(newFile, []byte(newContent), 0644)
	if err != nil {
		t.Fatalf("Failed to create new file through symlink: %v", err)
	}

	// Verify: new file exists in backup
	backupNewFile := filepath.Join(nvimBackup, "new.lua")
	if !pathExists(backupNewFile) {
		t.Errorf("New file should exist in backup")
	}

	content, _ = os.ReadFile(backupNewFile)
	if string(content) != newContent {
		t.Errorf("New file content = %q, want %q", string(content), newContent)
	}
}

// TestE2E_MixedFolderAndFiles tests configs with both folder and file modes.
func TestE2E_MixedFolderAndFiles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")

	// Setup backups
	// Folder-based: nvim (entire directory)
	nvimBackup := filepath.Join(repoDir, "nvim")
	os.MkdirAll(filepath.Join(nvimBackup, "lua"), 0755)
	os.WriteFile(filepath.Join(nvimBackup, "init.lua"), []byte("init"), 0644)
	os.WriteFile(filepath.Join(nvimBackup, "lua", "settings.lua"), []byte("settings"), 0644)

	// File-based: shell configs (specific files)
	shellBackup := filepath.Join(repoDir, "shell")
	os.MkdirAll(shellBackup, 0755)
	os.WriteFile(filepath.Join(shellBackup, ".bashrc"), []byte("bashrc"), 0644)
	os.WriteFile(filepath.Join(shellBackup, ".zshrc"), []byte("zshrc"), 0644)
	os.WriteFile(filepath.Join(shellBackup, ".profile"), []byte("profile"), 0644)

	cfg := &config.Config{
		Version:    2,
		BackupRoot: repoDir,
		Entries: []config.Entry{
			{
				Name:   "neovim",
				Files:  []string{}, // folder mode
				Backup: "./nvim",
				Targets: map[string]string{
					"linux": filepath.Join(homeDir, ".config", "nvim"),
				},
			},
			{
				Name:   "shell",
				Files:  []string{".bashrc", ".zshrc", ".profile"}, // file mode
				Backup: "./shell",
				Targets: map[string]string{
					"linux": homeDir,
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() failed: %v", err)
	}

	// Verify folder-based: nvim dir is a symlink
	nvimTarget := filepath.Join(homeDir, ".config", "nvim")
	if !isSymlink(nvimTarget) {
		t.Errorf("nvim should be a symlink (folder mode)")
	}

	// Verify nested file in folder symlink
	settingsPath := filepath.Join(nvimTarget, "lua", "settings.lua")
	content, err := os.ReadFile(settingsPath)
	if err != nil {
		t.Fatalf("Cannot read nested file: %v", err)
	}
	if string(content) != "settings" {
		t.Errorf("Nested file content = %q, want %q", string(content), "settings")
	}

	// Verify file-based: each file is a separate symlink
	shellFiles := []string{".bashrc", ".zshrc", ".profile"}
	for _, f := range shellFiles {
		fPath := filepath.Join(homeDir, f)
		if !isSymlink(fPath) {
			t.Errorf("%s should be a symlink (file mode)", f)
		}

		link, _ := os.Readlink(fPath)
		expectedLink := filepath.Join(shellBackup, f)
		if link != expectedLink {
			t.Errorf("%s symlink = %q, want %q", f, link, expectedLink)
		}
	}

	// Verify: home directory itself is NOT a symlink
	if isSymlink(homeDir) {
		t.Errorf("home directory should NOT be a symlink in file mode")
	}
}

// TestE2E_DryRunNoChanges verifies dry-run makes no filesystem changes.
func TestE2E_DryRunNoChanges(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")

	// Setup backup
	nvimBackup := filepath.Join(repoDir, "nvim")
	os.MkdirAll(nvimBackup, 0755)
	os.WriteFile(filepath.Join(nvimBackup, "init.lua"), []byte("config"), 0644)

	cfg := &config.Config{
		Version:    2,
		BackupRoot: repoDir,
		Entries: []config.Entry{
			{
				Name:   "neovim",
				Files:  []string{},
				Backup: "./nvim",
				Targets: map[string]string{
					"linux": filepath.Join(homeDir, ".config", "nvim"),
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.DryRun = true

	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() failed: %v", err)
	}

	// Verify: target directory should NOT exist
	nvimTarget := filepath.Join(homeDir, ".config", "nvim")
	if pathExists(nvimTarget) {
		t.Errorf("Target should not exist in dry-run mode")
	}

	// Verify: home directory should NOT exist
	if pathExists(homeDir) {
		t.Errorf("Home directory should not be created in dry-run mode")
	}
}

// TestE2E_AdoptDryRunPreservesOriginal tests that adopt in dry-run doesn't move files.
func TestE2E_AdoptDryRunPreservesOriginal(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	os.MkdirAll(repoDir, 0755)

	// User has existing config
	nvimDir := filepath.Join(homeDir, ".config", "nvim")
	os.MkdirAll(nvimDir, 0755)
	originalConfig := "-- original config\n"
	os.WriteFile(filepath.Join(nvimDir, "init.lua"), []byte(originalConfig), 0644)

	cfg := &config.Config{
		Version:    2,
		BackupRoot: repoDir,
		Entries: []config.Entry{
			{
				Name:   "neovim",
				Files:  []string{},
				Backup: "./nvim",
				Targets: map[string]string{
					"linux": nvimDir,
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.DryRun = true

	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() failed: %v", err)
	}

	// Verify: original config still exists (not moved)
	content, err := os.ReadFile(filepath.Join(nvimDir, "init.lua"))
	if err != nil {
		t.Fatalf("Original config should still exist: %v", err)
	}
	if string(content) != originalConfig {
		t.Errorf("Original config content changed")
	}

	// Verify: NOT a symlink
	if isSymlink(nvimDir) {
		t.Errorf("Should NOT be converted to symlink in dry-run")
	}

	// Verify: backup was NOT created
	nvimBackup := filepath.Join(repoDir, "nvim")
	if pathExists(nvimBackup) {
		t.Errorf("Backup should NOT be created in dry-run")
	}
}

// TestE2E_SkipAlreadySymlinked tests that existing correct symlinks are skipped.
func TestE2E_SkipAlreadySymlinked(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")

	// Setup backup
	nvimBackup := filepath.Join(repoDir, "nvim")
	os.MkdirAll(nvimBackup, 0755)
	os.WriteFile(filepath.Join(nvimBackup, "init.lua"), []byte("config"), 0644)

	// Pre-create the symlink
	nvimTarget := filepath.Join(homeDir, ".config", "nvim")
	os.MkdirAll(filepath.Dir(nvimTarget), 0755)
	os.Symlink(nvimBackup, nvimTarget)

	// Get initial symlink info
	initialInfo, _ := os.Lstat(nvimTarget)
	initialModTime := initialInfo.ModTime()

	cfg := &config.Config{
		Version:    2,
		BackupRoot: repoDir,
		Entries: []config.Entry{
			{
				Name:   "neovim",
				Files:  []string{},
				Backup: "./nvim",
				Targets: map[string]string{
					"linux": nvimTarget,
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() failed: %v", err)
	}

	// Verify: symlink still exists and points to same target
	if !isSymlink(nvimTarget) {
		t.Errorf("Symlink should still exist")
	}

	link, _ := os.Readlink(nvimTarget)
	if link != nvimBackup {
		t.Errorf("Symlink target = %q, want %q", link, nvimBackup)
	}

	// Verify: symlink was not recreated (same mod time)
	finalInfo, _ := os.Lstat(nvimTarget)
	if !finalInfo.ModTime().Equal(initialModTime) {
		t.Errorf("Symlink should not have been recreated")
	}
}

// TestE2E_MultipleOSTargets tests that only the current OS target is used.
func TestE2E_MultipleOSTargets(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	repoDir := filepath.Join(tmpDir, "repo")
	linuxHome := filepath.Join(tmpDir, "linux-home")
	windowsHome := filepath.Join(tmpDir, "windows-home")

	// Setup backup
	nvimBackup := filepath.Join(repoDir, "nvim")
	os.MkdirAll(nvimBackup, 0755)
	os.WriteFile(filepath.Join(nvimBackup, "init.lua"), []byte("config"), 0644)

	cfg := &config.Config{
		Version:    2,
		BackupRoot: repoDir,
		Entries: []config.Entry{
			{
				Name:   "neovim",
				Files:  []string{},
				Backup: "./nvim",
				Targets: map[string]string{
					"linux":   filepath.Join(linuxHome, ".config", "nvim"),
					"windows": filepath.Join(windowsHome, "AppData", "Local", "nvim"),
				},
			},
		},
	}

	// Test Linux
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Restore()

	linuxTarget := filepath.Join(linuxHome, ".config", "nvim")
	if !isSymlink(linuxTarget) {
		t.Errorf("Linux target should be a symlink")
	}

	windowsTarget := filepath.Join(windowsHome, "AppData", "Local", "nvim")
	if pathExists(windowsTarget) {
		t.Errorf("Windows target should NOT exist when running on Linux")
	}
}

// TestE2E_RootPaths tests that root paths are only processed when running as root.
func TestE2E_RootPaths(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	repoDir := filepath.Join(tmpDir, "repo")
	homeDir := filepath.Join(tmpDir, "home")
	etcDir := filepath.Join(tmpDir, "etc")

	// Setup backups
	userBackup := filepath.Join(repoDir, "user")
	os.MkdirAll(userBackup, 0755)
	os.WriteFile(filepath.Join(userBackup, "config"), []byte("user config"), 0644)

	systemBackup := filepath.Join(repoDir, "system")
	os.MkdirAll(systemBackup, 0755)
	os.WriteFile(filepath.Join(systemBackup, "system.conf"), []byte("system config"), 0644)

	cfg := &config.Config{
		Version:    2,
		BackupRoot: repoDir,
		Entries: []config.Entry{
			{
				Name:   "user-app",
				Files:  []string{},
				Backup: "./user",
				Targets: map[string]string{
					"linux": filepath.Join(homeDir, ".config", "app"),
				},
			},
			{
				Name:   "system-app",
				Root:   true,
				Files:  []string{"system.conf"},
				Backup: "./system",
				Targets: map[string]string{
					"linux": etcDir,
				},
			},
		},
	}

	// Test as non-root
	plat := &platform.Platform{OS: platform.OSLinux, IsRoot: false}
	mgr := New(cfg, plat)
	mgr.Restore()

	userTarget := filepath.Join(homeDir, ".config", "app")
	if !isSymlink(userTarget) {
		t.Errorf("User target should be symlinked for non-root")
	}

	systemTarget := filepath.Join(etcDir, "system.conf")
	if pathExists(systemTarget) {
		t.Errorf("System target should NOT be created for non-root")
	}

	// Test as root (simulated)
	plat = &platform.Platform{OS: platform.OSLinux, IsRoot: true}
	mgr = New(cfg, plat)
	mgr.Restore()

	if !isSymlink(systemTarget) {
		t.Errorf("System target should be symlinked for root")
	}
}

// TestE2E_RestoreWithNoTarget tests path specs with no target for current OS.
func TestE2E_RestoreWithNoTarget(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	repoDir := filepath.Join(tmpDir, "repo")
	nvimBackup := filepath.Join(repoDir, "nvim")
	os.MkdirAll(nvimBackup, 0755)
	os.WriteFile(filepath.Join(nvimBackup, "init.lua"), []byte("config"), 0644)

	cfg := &config.Config{
		Version:    2,
		BackupRoot: repoDir,
		Entries: []config.Entry{
			{
				Name:   "neovim",
				Files:  []string{},
				Backup: "./nvim",
				Targets: map[string]string{
					"windows": "C:\\Users\\nvim", // Only Windows target
				},
			},
		},
	}

	// Running on Linux with no Linux target
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() should not error when path has no target for OS: %v", err)
	}
}

// TestE2E_BackupWithNoTarget tests backup when path has no target for current OS.
func TestE2E_BackupWithNoTarget(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(repoDir, 0755)

	cfg := &config.Config{
		Version:    2,
		BackupRoot: repoDir,
		Entries: []config.Entry{
			{
				Name:   "windows-only",
				Files:  []string{},
				Backup: "./windows",
				Targets: map[string]string{
					"windows": "C:\\Users\\config",
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	err := mgr.Backup()
	if err != nil {
		t.Fatalf("Backup() should not error when path has no target for OS: %v", err)
	}
}

// TestE2E_BackupNonexistentSource tests backup when source doesn't exist.
func TestE2E_BackupNonexistentSource(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	repoDir := filepath.Join(tmpDir, "repo")
	os.MkdirAll(filepath.Join(repoDir, "nvim"), 0755)

	cfg := &config.Config{
		Version:    2,
		BackupRoot: repoDir,
		Entries: []config.Entry{
			{
				Name:   "neovim",
				Files:  []string{},
				Backup: "./nvim",
				Targets: map[string]string{
					"linux": filepath.Join(tmpDir, "nonexistent", "nvim"),
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	err := mgr.Backup()
	if err != nil {
		t.Fatalf("Backup() should not error when source doesn't exist: %v", err)
	}
}

// TestE2E_RestoreCreatesNestedParentDirs tests that restore creates nested parent directories.
func TestE2E_RestoreCreatesNestedParentDirs(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	repoDir := filepath.Join(tmpDir, "repo")
	nvimBackup := filepath.Join(repoDir, "nvim")
	os.MkdirAll(nvimBackup, 0755)
	os.WriteFile(filepath.Join(nvimBackup, "init.lua"), []byte("config"), 0644)

	// Deep nested target path
	deepTarget := filepath.Join(tmpDir, "home", "user", ".config", "deeply", "nested", "nvim")

	cfg := &config.Config{
		Version:    2,
		BackupRoot: repoDir,
		Entries: []config.Entry{
			{
				Name:   "neovim",
				Files:  []string{},
				Backup: "./nvim",
				Targets: map[string]string{
					"linux": deepTarget,
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() failed: %v", err)
	}

	if !isSymlink(deepTarget) {
		t.Errorf("Deep nested target should be a symlink")
	}
}
