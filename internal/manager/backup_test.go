package manager

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

func TestBackupFolder(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create source directory with content
	srcDir := filepath.Join(tmpDir, "source", "config")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "settings.json"), []byte("settings"), 0644)

	// Backup directory
	backupDir := filepath.Join(tmpDir, "backup")
	os.MkdirAll(backupDir, 0755)

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	err := mgr.backupFolder("test", srcDir, backupDir)
	if err != nil {
		t.Fatalf("backupFolder() error = %v", err)
	}

	// Check files were copied
	copiedFile := filepath.Join(backupDir, "config", "settings.json")
	if !pathExists(copiedFile) {
		t.Error("File was not backed up")
	}

	content, _ := os.ReadFile(copiedFile)
	if string(content) != "settings" {
		t.Errorf("Backup content = %q, want %q", string(content), "settings")
	}
}

func TestBackupFolderSkipsSymlink(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create real source
	realSrc := filepath.Join(tmpDir, "real")
	os.MkdirAll(realSrc, 0755)
	os.WriteFile(filepath.Join(realSrc, "file.txt"), []byte("content"), 0644)

	// Create symlink as "source"
	symlinkSrc := filepath.Join(tmpDir, "symlink")
	os.Symlink(realSrc, symlinkSrc)

	backupDir := filepath.Join(tmpDir, "backup")
	os.MkdirAll(backupDir, 0755)

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	err := mgr.backupFolder("test", symlinkSrc, backupDir)
	if err != nil {
		t.Fatalf("backupFolder() error = %v", err)
	}

	// Backup directory should be empty (symlink was skipped)
	entries, _ := os.ReadDir(backupDir)
	if len(entries) != 0 {
		t.Error("Backup should be empty when source is a symlink")
	}
}

func TestBackupFiles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create source files
	srcDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "config1.txt"), []byte("config1"), 0644)
	os.WriteFile(filepath.Join(srcDir, "config2.txt"), []byte("config2"), 0644)

	backupDir := filepath.Join(tmpDir, "backup")
	os.MkdirAll(backupDir, 0755)

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	files := []string{"config1.txt", "config2.txt"}
	err := mgr.backupFiles("test", files, srcDir, backupDir)
	if err != nil {
		t.Fatalf("backupFiles() error = %v", err)
	}

	// Check files were copied
	for _, file := range files {
		backupFile := filepath.Join(backupDir, file)
		if !pathExists(backupFile) {
			t.Errorf("File %s was not backed up", file)
		}
	}
}

func TestBackupFilesSkipsSymlinks(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create real file
	srcDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(srcDir, 0755)
	realFile := filepath.Join(srcDir, "real.txt")
	os.WriteFile(realFile, []byte("real content"), 0644)

	// Create symlink
	symlinkFile := filepath.Join(srcDir, "symlink.txt")
	os.Symlink(realFile, symlinkFile)

	backupDir := filepath.Join(tmpDir, "backup")
	os.MkdirAll(backupDir, 0755)

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	err := mgr.backupFiles("test", []string{"symlink.txt"}, srcDir, backupDir)
	if err != nil {
		t.Fatalf("backupFiles() error = %v", err)
	}

	// Symlink should not have been backed up
	backupFile := filepath.Join(backupDir, "symlink.txt")
	if pathExists(backupFile) {
		t.Error("Symlink should not be backed up")
	}
}

func TestBackupDryRun(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	srcDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "config.txt"), []byte("content"), 0644)

	backupDir := filepath.Join(tmpDir, "backup")
	os.MkdirAll(backupDir, 0755)

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.DryRun = true

	err := mgr.backupFiles("test", []string{"config.txt"}, srcDir, backupDir)
	if err != nil {
		t.Fatalf("backupFiles() error = %v", err)
	}

	// File should NOT be copied in dry run mode
	backupFile := filepath.Join(backupDir, "config.txt")
	if pathExists(backupFile) {
		t.Error("File was copied despite dry run mode")
	}
}

func TestBackupIntegration(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create "installed" configs
	homeDir := filepath.Join(tmpDir, "home")

	nvimDir := filepath.Join(homeDir, ".config", "nvim")
	os.MkdirAll(nvimDir, 0755)
	os.WriteFile(filepath.Join(nvimDir, "init.lua"), []byte("vim config"), 0644)

	os.WriteFile(filepath.Join(homeDir, ".bashrc"), []byte("bash config"), 0644)

	// Create backup directories
	backupRoot := filepath.Join(tmpDir, "backup")
	os.MkdirAll(filepath.Join(backupRoot, "nvim"), 0755)
	os.MkdirAll(filepath.Join(backupRoot, "bash"), 0755)

	cfg := &config.Config{
		Version:    2,
		BackupRoot: backupRoot,
		Entries: []config.Entry{
			{
				Name:   "nvim",
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

	err := mgr.Backup()
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	// Check nvim was backed up (as folder)
	backedUpInit := filepath.Join(backupRoot, "nvim", "nvim", "init.lua")
	if !pathExists(backedUpInit) {
		t.Error("nvim/init.lua was not backed up")
	}

	// Check bashrc was backed up
	backedUpBashrc := filepath.Join(backupRoot, "bash", ".bashrc")
	if !pathExists(backedUpBashrc) {
		t.Error(".bashrc was not backed up")
	}

	content, _ := os.ReadFile(backedUpBashrc)
	if string(content) != "bash config" {
		t.Errorf("Backup content = %q, want %q", string(content), "bash config")
	}
}

func TestBackupV3Application(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create installed config
	homeDir := filepath.Join(tmpDir, "home")
	nvimDir := filepath.Join(homeDir, ".config", "nvim")
	os.MkdirAll(nvimDir, 0755)
	os.WriteFile(filepath.Join(nvimDir, "init.lua"), []byte("vim config"), 0644)

	// Create backup directory
	backupRoot := filepath.Join(tmpDir, "backup")
	os.MkdirAll(filepath.Join(backupRoot, "nvim"), 0755)

	// Create v3 config
	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name:        "neovim",
				Description: "Neovim editor",
				Entries: []config.SubEntry{
					{
						Name:   "config",
						Backup: "./nvim",
						Targets: map[string]string{
							"linux": nvimDir,
						},
					},
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	err := mgr.Backup()
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	// Check file was backed up (folder backup adds base name as subfolder)
	backedUpInit := filepath.Join(backupRoot, "nvim", "nvim", "init.lua")
	if !pathExists(backedUpInit) {
		t.Errorf("nvim/init.lua was not backed up at %s", backedUpInit)
		// List what actually exists
		entries, _ := os.ReadDir(filepath.Join(backupRoot, "nvim"))
		t.Logf("Contents of nvim backup dir: %v", entries)
		return
	}

	content, err := os.ReadFile(backedUpInit)
	if err != nil {
		t.Fatalf("Failed to read backed up file: %v", err)
	}
	if string(content) != "vim config" {
		t.Errorf("Backup content = %q, want %q", string(content), "vim config")
	}
}

func TestBackup_SymlinkAlreadyExists(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)

	// Create target as symlink
	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "config")
	backupDir := filepath.Join(m.Config.BackupRoot, "test")
	os.MkdirAll(backupDir, 0755)

	// Create symlink first
	os.Symlink(backupDir, target)

	entry := config.Entry{
		Name:   "test",
		Backup: "test",
		Targets: map[string]string{
			"linux": target,
		},
	}

	err := m.backupEntry(entry, target)

	// Should skip without error
	if err != nil {
		t.Errorf("backupEntry() unexpected error: %v", err)
	}
}

func TestBackup_FilePermissionsPreserved(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)

	// Create file with specific permissions
	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "config")
	os.WriteFile(target, []byte("test"), 0600)

	backupPath := filepath.Join(m.Config.BackupRoot, "test")

	entry := config.Entry{
		Name:   "test",
		Files:  []string{"config"},
		Backup: "test",
		Targets: map[string]string{
			"linux": targetDir,
		},
	}

	err := m.backupEntry(entry, targetDir)
	if err != nil {
		t.Fatalf("backupEntry() error = %v", err)
	}

	// Check backup has same permissions
	backupFile := filepath.Join(backupPath, "config")
	info, err := os.Stat(backupFile)
	if err != nil {
		t.Fatalf("stat backup: %v", err)
	}

	// Compare permission bits (ignore file type bits)
	expectedPerm := os.FileMode(0600)
	actualPerm := info.Mode().Perm()
	if actualPerm != expectedPerm {
		t.Errorf("permissions = %o, want %o", actualPerm, expectedPerm)
	}
}

func TestBackup_DryRunNoChanges(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)
	m.DryRun = true

	targetDir := t.TempDir()
	target := filepath.Join(targetDir, "config")
	os.WriteFile(target, []byte("test"), 0644)

	entry := config.Entry{
		Name:   "dryrun-test",
		Files:  []string{"config"},
		Backup: "dryrun-backup",
		Targets: map[string]string{
			"linux": targetDir,
		},
	}

	err := m.backupEntry(entry, targetDir)
	if err != nil {
		t.Fatalf("backupEntry() error = %v", err)
	}

	// Verify no backup created
	backupPath := filepath.Join(m.Config.BackupRoot, "dryrun-backup")
	if _, err := os.Stat(backupPath); !os.IsNotExist(err) {
		t.Error("dry-run created backup directory")
	}

	// Verify target unchanged
	if _, err := os.Stat(target); err != nil {
		t.Error("dry-run modified target")
	}
}

func TestBackupWithContext(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	homeDir := filepath.Join(tmpDir, "home")
	nvimDir := filepath.Join(homeDir, ".config", "nvim")
	os.MkdirAll(nvimDir, 0755)
	os.WriteFile(filepath.Join(nvimDir, "init.lua"), []byte("vim config"), 0644)

	backupRoot := filepath.Join(tmpDir, "backup")
	os.MkdirAll(filepath.Join(backupRoot, "nvim"), 0755)

	cfg := &config.Config{
		Version:    2,
		BackupRoot: backupRoot,
		Entries: []config.Entry{
			{
				Name:   "nvim",
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

	ctx := context.Background()
	err := mgr.BackupWithContext(ctx)
	if err != nil {
		t.Fatalf("BackupWithContext() error = %v", err)
	}

	backedUpInit := filepath.Join(backupRoot, "nvim", "nvim", "init.lua")
	if !pathExists(backedUpInit) {
		t.Error("nvim/init.lua was not backed up")
	}
}

func TestBackupV3_WithFiles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	homeDir := filepath.Join(tmpDir, "home")
	os.MkdirAll(homeDir, 0755)
	os.WriteFile(filepath.Join(homeDir, ".bashrc"), []byte("bash config"), 0644)
	os.WriteFile(filepath.Join(homeDir, ".profile"), []byte("profile config"), 0644)

	backupRoot := filepath.Join(tmpDir, "backup")
	os.MkdirAll(filepath.Join(backupRoot, "bash"), 0755)

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name:        "bash",
				Description: "Bash shell",
				Entries: []config.SubEntry{
					{
						Name:   "config",
						Type:   "config",
						Files:  []string{".bashrc", ".profile"},
						Backup: "./bash",
						Targets: map[string]string{
							"linux": homeDir,
						},
					},
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	err := mgr.Backup()
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	backedUpBashrc := filepath.Join(backupRoot, "bash", ".bashrc")
	backedUpProfile := filepath.Join(backupRoot, "bash", ".profile")

	if !pathExists(backedUpBashrc) {
		t.Error(".bashrc was not backed up")
	}
	if !pathExists(backedUpProfile) {
		t.Error(".profile was not backed up")
	}

	content, _ := os.ReadFile(backedUpBashrc)
	if string(content) != "bash config" {
		t.Errorf("Backup content = %q, want %q", string(content), "bash config")
	}
}
