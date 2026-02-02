package manager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

func TestRestoreFolder(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create source directory with content
	srcDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "config.txt"), []byte("config"), 0644)

	// Target directory
	targetDir := filepath.Join(tmpDir, "target", "config")

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	entry := config.Entry{Name: "test"}
	err := mgr.restoreFolder(entry, srcDir, targetDir)
	if err != nil {
		t.Fatalf("restoreFolder() error = %v", err)
	}

	// Check symlink was created
	if !isSymlink(targetDir) {
		t.Error("Target is not a symlink")
	}

	// Check symlink points to source
	link, err := os.Readlink(targetDir)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}

	if link != srcDir {
		t.Errorf("Symlink target = %q, want %q", link, srcDir)
	}
}

func TestRestoreFolderSkipsExistingSymlink(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	srcDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(srcDir, 0755)

	targetDir := filepath.Join(tmpDir, "target")
	os.Symlink(srcDir, targetDir)

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	entry := config.Entry{Name: "test"}
	err := mgr.restoreFolder(entry, srcDir, targetDir)
	if err != nil {
		t.Fatalf("restoreFolder() error = %v", err)
	}

	// Symlink should still exist and point to same target
	link, _ := os.Readlink(targetDir)
	if link != srcDir {
		t.Errorf("Symlink target changed to %q, want %q", link, srcDir)
	}
}

func TestRestoreFiles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create source files
	srcDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0644)
	os.WriteFile(filepath.Join(srcDir, "file2.txt"), []byte("content2"), 0644)

	targetDir := filepath.Join(tmpDir, "target")

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	entry := config.Entry{Name: "test", Files: []string{"file1.txt", "file2.txt"}}
	err := mgr.restoreFiles(entry, srcDir, targetDir)
	if err != nil {
		t.Fatalf("restoreFiles() error = %v", err)
	}

	// Check symlinks were created
	for _, file := range entry.Files {
		targetFile := filepath.Join(targetDir, file)
		if !isSymlink(targetFile) {
			t.Errorf("%s is not a symlink", file)
		}

		link, _ := os.Readlink(targetFile)
		expectedLink := filepath.Join(srcDir, file)
		if link != expectedLink {
			t.Errorf("Symlink for %s = %q, want %q", file, link, expectedLink)
		}
	}
}

func TestRestoreFilesRemovesExisting(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	srcDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "config.txt"), []byte("new content"), 0644)

	targetDir := filepath.Join(tmpDir, "target")
	os.MkdirAll(targetDir, 0755)
	os.WriteFile(filepath.Join(targetDir, "config.txt"), []byte("old content"), 0644)

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	entry := config.Entry{Name: "test", Files: []string{"config.txt"}}
	err := mgr.restoreFiles(entry, srcDir, targetDir)
	if err != nil {
		t.Fatalf("restoreFiles() error = %v", err)
	}

	targetFile := filepath.Join(targetDir, "config.txt")
	if !isSymlink(targetFile) {
		t.Error("Target file is not a symlink after restore")
	}
}

func TestRestoreDryRun(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	srcDir := filepath.Join(tmpDir, "source")
	os.MkdirAll(srcDir, 0755)
	os.WriteFile(filepath.Join(srcDir, "config.txt"), []byte("content"), 0644)

	targetDir := filepath.Join(tmpDir, "target")

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.DryRun = true

	entry := config.Entry{Name: "test"}
	err := mgr.restoreFolder(entry, srcDir, targetDir)
	if err != nil {
		t.Fatalf("restoreFolder() error = %v", err)
	}

	// Target should NOT be created in dry run mode
	if pathExists(targetDir) {
		t.Error("Target was created despite dry run mode")
	}
}

func TestRestoreIntegration(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backup structure
	backupRoot := filepath.Join(tmpDir, "backup")
	nvimBackup := filepath.Join(backupRoot, "nvim")
	os.MkdirAll(nvimBackup, 0755)
	os.WriteFile(filepath.Join(nvimBackup, "init.lua"), []byte("vim config"), 0644)

	bashBackup := filepath.Join(backupRoot, "bash")
	os.MkdirAll(bashBackup, 0755)
	os.WriteFile(filepath.Join(bashBackup, ".bashrc"), []byte("bash config"), 0644)

	// Create config
	cfg := &config.Config{
		Version:    2,
		BackupRoot: backupRoot,
		Entries: []config.Entry{
			{
				Name:   "nvim",
				Files:  []string{},
				Backup: "./nvim",
				Targets: map[string]string{
					"linux": filepath.Join(tmpDir, "home", ".config", "nvim"),
				},
			},
			{
				Name:   "bash",
				Files:  []string{".bashrc"},
				Backup: "./bash",
				Targets: map[string]string{
					"linux": filepath.Join(tmpDir, "home"),
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

	// Check nvim folder symlink
	nvimTarget := filepath.Join(tmpDir, "home", ".config", "nvim")
	if !isSymlink(nvimTarget) {
		t.Error("nvim target is not a symlink")
	}

	// Check bashrc file symlink
	bashrcTarget := filepath.Join(tmpDir, "home", ".bashrc")
	if !isSymlink(bashrcTarget) {
		t.Error(".bashrc target is not a symlink")
	}
}

func TestRestoreGitEntryDryRun(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	targetDir := filepath.Join(tmpDir, "target", "plugin")

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.DryRun = true

	entry := config.Entry{
		Name:   "test-plugin",
		Repo:   "https://github.com/test/plugin.git",
		Branch: "main",
		Targets: map[string]string{
			"linux": targetDir,
		},
	}

	err := mgr.restoreGitEntry(entry, targetDir)
	if err != nil {
		t.Fatalf("restoreGitEntry() error = %v", err)
	}

	// Target should NOT be created in dry run mode
	if pathExists(targetDir) {
		t.Error("Target was created despite dry run mode")
	}
}

func TestRestoreGitEntrySkipsExistingNonGit(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a target that exists but is not a git repo
	targetDir := filepath.Join(tmpDir, "target")
	os.MkdirAll(targetDir, 0755)
	os.WriteFile(filepath.Join(targetDir, "file.txt"), []byte("content"), 0644)

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	entry := config.Entry{
		Name: "test-plugin",
		Repo: "https://github.com/test/plugin.git",
		Targets: map[string]string{
			"linux": targetDir,
		},
	}

	err := mgr.restoreGitEntry(entry, targetDir)
	if err != nil {
		t.Fatalf("restoreGitEntry() error = %v", err)
	}

	// Target should still exist but .git should not
	gitDir := filepath.Join(targetDir, ".git")
	if pathExists(gitDir) {
		t.Error(".git directory should not exist (we don't clone over non-git dirs)")
	}
}

func TestRestoreV3Application(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backup structure
	backupRoot := filepath.Join(tmpDir, "backup")
	nvimBackup := filepath.Join(backupRoot, "nvim")
	os.MkdirAll(nvimBackup, 0755)
	os.WriteFile(filepath.Join(nvimBackup, "init.lua"), []byte("vim config"), 0644)

	// Create v3 config with Application
	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name:        "neovim",
				Description: "Neovim editor",
				Entries: []config.SubEntry{
					{
						Name:   "nvim",
						Backup: "./nvim",
						Targets: map[string]string{
							"linux": filepath.Join(tmpDir, "home", ".config", "nvim"),
						},
					},
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

	// Check nvim folder symlink was created
	nvimTarget := filepath.Join(tmpDir, "home", ".config", "nvim")
	if !isSymlink(nvimTarget) {
		t.Error("nvim target is not a symlink")
	}

	link, _ := os.Readlink(nvimTarget)
	if link != nvimBackup {
		t.Errorf("Symlink target = %q, want %q", link, nvimBackup)
	}
}

func TestRestoreV3MultipleSubEntries(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backup structure for multiple sub-entries
	backupRoot := filepath.Join(tmpDir, "backup")

	configBackup := filepath.Join(backupRoot, "nvim-config")
	os.MkdirAll(configBackup, 0755)
	os.WriteFile(filepath.Join(configBackup, "init.lua"), []byte("config"), 0644)

	dataBackup := filepath.Join(backupRoot, "nvim-data")
	os.MkdirAll(dataBackup, 0755)
	os.WriteFile(filepath.Join(dataBackup, "lazy.lua"), []byte("data"), 0644)

	// Create v3 config with multiple sub-entries
	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name:        "neovim",
				Description: "Neovim with separate config and data",
				Entries: []config.SubEntry{
					{
						Name:   "config",
						Backup: "./nvim-config",
						Targets: map[string]string{
							"linux": filepath.Join(tmpDir, "home", ".config", "nvim"),
						},
					},
					{
						Name:   "data",
						Backup: "./nvim-data",
						Targets: map[string]string{
							"linux": filepath.Join(tmpDir, "home", ".local", "share", "nvim"),
						},
					},
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

	// Check both symlinks were created
	configTarget := filepath.Join(tmpDir, "home", ".config", "nvim")
	if !isSymlink(configTarget) {
		t.Error("config target is not a symlink")
	}

	dataTarget := filepath.Join(tmpDir, "home", ".local", "share", "nvim")
	if !isSymlink(dataTarget) {
		t.Error("data target is not a symlink")
	}
}
