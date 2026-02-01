package manager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

func TestAdoptFolder(t *testing.T) {
	tmpDir := t.TempDir()

	// Create a target folder that exists but backup doesn't
	targetDir := filepath.Join(tmpDir, "target", "config")
	os.MkdirAll(targetDir, 0755)
	os.WriteFile(filepath.Join(targetDir, "settings.json"), []byte("my settings"), 0644)

	// Backup location (doesn't exist yet)
	backupDir := filepath.Join(tmpDir, "backup", "config")

	cfg := &config.Config{BackupRoot: filepath.Join(tmpDir, "backup")}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	err := mgr.restoreFolder("test", backupDir, targetDir)
	if err != nil {
		t.Fatalf("restoreFolder() error = %v", err)
	}

	// Check that backup now exists with the content
	backupFile := filepath.Join(backupDir, "settings.json")
	if !pathExists(backupFile) {
		t.Error("Backup file should exist after adopt")
	}

	content, _ := os.ReadFile(backupFile)
	if string(content) != "my settings" {
		t.Errorf("Backup content = %q, want %q", string(content), "my settings")
	}

	// Check that target is now a symlink
	if !isSymlink(targetDir) {
		t.Error("Target should be a symlink after adopt")
	}

	// Check symlink points to backup
	link, _ := os.Readlink(targetDir)
	if link != backupDir {
		t.Errorf("Symlink target = %q, want %q", link, backupDir)
	}
}

func TestAdoptFiles(t *testing.T) {
	tmpDir := t.TempDir()

	// Create target files that exist but backup doesn't
	targetDir := filepath.Join(tmpDir, "target")
	os.MkdirAll(targetDir, 0755)
	os.WriteFile(filepath.Join(targetDir, "config1.txt"), []byte("config1 content"), 0644)
	os.WriteFile(filepath.Join(targetDir, "config2.txt"), []byte("config2 content"), 0644)

	// Backup location (doesn't exist yet)
	backupDir := filepath.Join(tmpDir, "backup")

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	files := []string{"config1.txt", "config2.txt"}
	err := mgr.restoreFiles("test", files, backupDir, targetDir)
	if err != nil {
		t.Fatalf("restoreFiles() error = %v", err)
	}

	// Check that backup files now exist
	for _, file := range files {
		backupFile := filepath.Join(backupDir, file)
		if !pathExists(backupFile) {
			t.Errorf("Backup file %s should exist after adopt", file)
		}

		// Check target is now a symlink
		targetFile := filepath.Join(targetDir, file)
		if !isSymlink(targetFile) {
			t.Errorf("Target file %s should be a symlink after adopt", file)
		}
	}

	// Check content was preserved
	content, _ := os.ReadFile(filepath.Join(backupDir, "config1.txt"))
	if string(content) != "config1 content" {
		t.Errorf("Backup content = %q, want %q", string(content), "config1 content")
	}
}

func TestAdoptSkipsExistingBackup(t *testing.T) {
	tmpDir := t.TempDir()

	// Create both target and backup
	targetDir := filepath.Join(tmpDir, "target", "config")
	os.MkdirAll(targetDir, 0755)
	os.WriteFile(filepath.Join(targetDir, "settings.json"), []byte("target content"), 0644)

	backupDir := filepath.Join(tmpDir, "backup", "config")
	os.MkdirAll(backupDir, 0755)
	os.WriteFile(filepath.Join(backupDir, "settings.json"), []byte("backup content"), 0644)

	cfg := &config.Config{BackupRoot: filepath.Join(tmpDir, "backup")}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	err := mgr.restoreFolder("test", backupDir, targetDir)
	if err != nil {
		t.Fatalf("restoreFolder() error = %v", err)
	}

	// Backup content should be preserved (not overwritten)
	content, _ := os.ReadFile(filepath.Join(backupDir, "settings.json"))
	if string(content) != "backup content" {
		t.Errorf("Backup content = %q, want %q (should not be overwritten)", string(content), "backup content")
	}

	// Target should be a symlink
	if !isSymlink(targetDir) {
		t.Error("Target should be a symlink")
	}
}

func TestAdoptDryRun(t *testing.T) {
	tmpDir := t.TempDir()

	// Create target that exists but backup doesn't
	targetDir := filepath.Join(tmpDir, "target", "config")
	os.MkdirAll(targetDir, 0755)
	os.WriteFile(filepath.Join(targetDir, "settings.json"), []byte("content"), 0644)

	backupDir := filepath.Join(tmpDir, "backup", "config")

	cfg := &config.Config{BackupRoot: filepath.Join(tmpDir, "backup")}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.DryRun = true

	err := mgr.restoreFolder("test", backupDir, targetDir)
	if err != nil {
		t.Fatalf("restoreFolder() error = %v", err)
	}

	// Backup should NOT be created in dry run
	if pathExists(backupDir) {
		t.Error("Backup should not be created in dry run mode")
	}

	// Target should NOT be a symlink in dry run
	if isSymlink(targetDir) {
		t.Error("Target should not be changed in dry run mode")
	}

	// Original target content should still exist
	if !pathExists(filepath.Join(targetDir, "settings.json")) {
		t.Error("Original target content should still exist in dry run")
	}
}

func TestAdoptIntegration(t *testing.T) {
	tmpDir := t.TempDir()

	// Simulate a fresh install where user has existing configs
	homeDir := filepath.Join(tmpDir, "home")

	// User has nvim config
	nvimDir := filepath.Join(homeDir, ".config", "nvim")
	os.MkdirAll(nvimDir, 0755)
	os.WriteFile(filepath.Join(nvimDir, "init.lua"), []byte("-- my nvim config"), 0644)

	// User has .bashrc
	os.WriteFile(filepath.Join(homeDir, ".bashrc"), []byte("# my bashrc"), 0644)

	// Empty backup (fresh repo clone)
	backupRoot := filepath.Join(tmpDir, "backup")
	os.MkdirAll(backupRoot, 0755)

	cfg := &config.Config{
		BackupRoot: backupRoot,
		Paths: []config.PathSpec{
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

	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	// Check nvim was adopted
	nvimBackup := filepath.Join(backupRoot, "nvim")
	if !pathExists(filepath.Join(nvimBackup, "init.lua")) {
		t.Error("nvim config should be adopted to backup")
	}

	if !isSymlink(nvimDir) {
		t.Error("nvim dir should be a symlink after adopt")
	}

	// Check .bashrc was adopted
	bashBackup := filepath.Join(backupRoot, "bash", ".bashrc")
	if !pathExists(bashBackup) {
		t.Error(".bashrc should be adopted to backup")
	}

	content, _ := os.ReadFile(bashBackup)
	if string(content) != "# my bashrc" {
		t.Error(".bashrc content should be preserved")
	}

	bashrcTarget := filepath.Join(homeDir, ".bashrc")
	if !isSymlink(bashrcTarget) {
		t.Error(".bashrc should be a symlink after adopt")
	}
}
