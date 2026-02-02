package manager

import (
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
