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
	if err := os.MkdirAll(srcDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "settings.json"), []byte("settings"), 0600); err != nil {
		t.Fatal(err)
	}

	// Backup directory
	backupDir := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

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

	content, _ := os.ReadFile(copiedFile) //nolint:gosec // test file
	if string(content) != "settings" {
		t.Errorf("Backup content = %q, want %q", string(content), "settings")
	}
}

func TestBackupFolderSkipsSymlink(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create real source
	realSrc := filepath.Join(tmpDir, "real")
	if err := os.MkdirAll(realSrc, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(realSrc, "file.txt"), []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create symlink as "source"
	symlinkSrc := filepath.Join(tmpDir, "symlink")
	if err := os.Symlink(realSrc, symlinkSrc); err != nil {
		t.Fatal(err)
	}

	backupDir := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

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
	if err := os.MkdirAll(srcDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "config1.txt"), []byte("config1"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "config2.txt"), []byte("config2"), 0600); err != nil {
		t.Fatal(err)
	}

	backupDir := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

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
	if err := os.MkdirAll(srcDir, 0750); err != nil {
		t.Fatal(err)
	}
	realFile := filepath.Join(srcDir, "real.txt")
	if err := os.WriteFile(realFile, []byte("real content"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create symlink
	symlinkFile := filepath.Join(srcDir, "symlink.txt")
	if err := os.Symlink(realFile, symlinkFile); err != nil {
		t.Fatal(err)
	}

	backupDir := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

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
	if err := os.MkdirAll(srcDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "config.txt"), []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}

	backupDir := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

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
	if err := os.MkdirAll(nvimDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvimDir, "init.lua"), []byte("vim config"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(homeDir, ".bashrc"), []byte("bash config"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create backup directories
	backupRoot := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(filepath.Join(backupRoot, "nvim"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(backupRoot, "bash"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "nvim-app",
				Entries: []config.SubEntry{
					{
						Name:   "nvim",
						Files:  []string{},
						Backup: "./nvim",
						Targets: map[string]string{
							"linux": nvimDir,
						},
					},
				},
			},
			{
				Name: "bash-app",
				Entries: []config.SubEntry{
					{
						Name:   "bash",
						Files:  []string{".bashrc"},
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

	content, _ := os.ReadFile(backedUpBashrc) //nolint:gosec // test file
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
	if err := os.MkdirAll(nvimDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvimDir, "init.lua"), []byte("vim config"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create backup directory
	backupRoot := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(filepath.Join(backupRoot, "nvim"), 0750); err != nil {
		t.Fatal(err)
	}

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

	content, err := os.ReadFile(backedUpInit) //nolint:gosec // test file
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
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Create symlink first
	if err := os.Symlink(backupDir, target); err != nil {
		t.Fatal(err)
	}

	subEntry := config.SubEntry{
		Name:   "test",
		Backup: "test",
		Targets: map[string]string{
			"linux": target,
		},
	}

	backupPath := filepath.Join(m.Config.BackupRoot, "test")
	err := m.backupFolderSubEntry("test", subEntry, backupPath, target)

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
	if err := os.WriteFile(target, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}

	backupPath := filepath.Join(m.Config.BackupRoot, "test")

	subEntry := config.SubEntry{
		Name:   "test",
		Files:  []string{"config"},
		Backup: "test",
		Targets: map[string]string{
			"linux": targetDir,
		},
	}

	err := m.backupFilesSubEntry("test", subEntry, backupPath, targetDir)
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
	if err := os.WriteFile(target, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}

	subEntry := config.SubEntry{
		Name:   "dryrun-test",
		Files:  []string{"config"},
		Backup: "dryrun-backup",
		Targets: map[string]string{
			"linux": targetDir,
		},
	}

	backupPath := filepath.Join(m.Config.BackupRoot, "dryrun-backup")
	err := m.backupFilesSubEntry("dryrun-test", subEntry, backupPath, targetDir)
	if err != nil {
		t.Fatalf("backupEntry() error = %v", err)
	}

	// Verify no backup created
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
	if err := os.MkdirAll(nvimDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvimDir, "init.lua"), []byte("vim config"), 0600); err != nil {
		t.Fatal(err)
	}

	backupRoot := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(filepath.Join(backupRoot, "nvim"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "nvim-app",
				Entries: []config.SubEntry{
					{
						Name:   "nvim",
						Files:  []string{},
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
	if err := os.MkdirAll(homeDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".bashrc"), []byte("bash config"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".profile"), []byte("profile config"), 0600); err != nil {
		t.Fatal(err)
	}

	backupRoot := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(filepath.Join(backupRoot, "bash"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name:        "bash",
				Description: "Bash shell",
				Entries: []config.SubEntry{
					{
						Name: "config",

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

	content, _ := os.ReadFile(backedUpBashrc) //nolint:gosec // test file
	if string(content) != "bash config" {
		t.Errorf("Backup content = %q, want %q", string(content), "bash config")
	}
}

func TestBackupV3_FolderSubEntry(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create config folder
	configDir := filepath.Join(tmpDir, "home", ".config", "app")
	if err := os.MkdirAll(configDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte("config"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configDir, "settings.json"), []byte("settings"), 0600); err != nil {
		t.Fatal(err)
	}

	backupRoot := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(filepath.Join(backupRoot, "app"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "app",
				Entries: []config.SubEntry{
					{
						Name: "config",

						Backup: "./app",
						Targets: map[string]string{
							"linux": configDir,
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

	// Check files were backed up (folder backup adds base name as subfolder)
	backedUpConfig := filepath.Join(backupRoot, "app", "app", "config.json")
	backedUpSettings := filepath.Join(backupRoot, "app", "app", "settings.json")

	if !pathExists(backedUpConfig) {
		t.Error("config.json was not backed up")
	}

	if !pathExists(backedUpSettings) {
		t.Error("settings.json was not backed up")
	}
}

func TestBackupV3_MultipleApplications(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create multiple config directories
	homeDir := filepath.Join(tmpDir, "home")
	nvimDir := filepath.Join(homeDir, ".config", "nvim")
	if err := os.MkdirAll(nvimDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvimDir, "init.lua"), []byte("vim"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(homeDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".bashrc"), []byte("bash"), 0600); err != nil {
		t.Fatal(err)
	}

	backupRoot := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(filepath.Join(backupRoot, "nvim"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(backupRoot, "bash"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "nvim",
				Entries: []config.SubEntry{
					{
						Name: "config",

						Backup: "./nvim",
						Targets: map[string]string{
							"linux": nvimDir,
						},
					},
				},
			},
			{
				Name: "bash",
				Entries: []config.SubEntry{
					{
						Name: "config",

						Files:  []string{".bashrc"},
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

	// Check nvim was backed up
	backedUpNvim := filepath.Join(backupRoot, "nvim", "nvim", "init.lua")
	if !pathExists(backedUpNvim) {
		t.Error("nvim/init.lua was not backed up")
	}

	// Check bash was backed up
	backedUpBash := filepath.Join(backupRoot, "bash", ".bashrc")
	if !pathExists(backedUpBash) {
		t.Error(".bashrc was not backed up")
	}
}

//nolint:dupl // similar test structure is intentional
func TestBackupV3_SkipsWrongOS(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".bashrc"), []byte("bash"), 0600); err != nil {
		t.Fatal(err)
	}

	backupRoot := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(filepath.Join(backupRoot, "bash"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "bash",
				Entries: []config.SubEntry{
					{
						Name: "config",

						Files:  []string{".bashrc"},
						Backup: "./bash",
						Targets: map[string]string{
							"windows": homeDir, // Wrong OS
						},
					},
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	err := mgr.Backup()
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	// Nothing should be backed up (wrong OS)
	backedUpFile := filepath.Join(backupRoot, "bash", ".bashrc")
	if pathExists(backedUpFile) {
		t.Error(".bashrc should not be backed up for wrong OS")
	}
}

//nolint:dupl // similar test structure is intentional
func TestBackupV3_DryRun(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".bashrc"), []byte("bash"), 0600); err != nil {
		t.Fatal(err)
	}

	backupRoot := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(filepath.Join(backupRoot, "bash"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "bash",
				Entries: []config.SubEntry{
					{
						Name: "config",

						Files:  []string{".bashrc"},
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
	mgr.DryRun = true

	err := mgr.Backup()
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	// Nothing should be backed up in dry-run mode
	backedUpFile := filepath.Join(backupRoot, "bash", ".bashrc")
	if pathExists(backedUpFile) {
		t.Error(".bashrc should not be backed up in dry-run mode")
	}
}

func TestBackupFilesSubEntry_SourceMissing(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Target doesn't exist
	homeDir := filepath.Join(tmpDir, "home")
	backupRoot := filepath.Join(tmpDir, "backup")
	backupPath := filepath.Join(backupRoot, "test")

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "test",
				Entries: []config.SubEntry{
					{
						Name: "config",

						Files:  []string{"missing.txt"},
						Backup: "./test",
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
	mgr.Verbose = true

	subEntry := cfg.Applications[0].Entries[0]

	err := mgr.backupFilesSubEntry("test", subEntry, backupPath, homeDir)
	if err != nil {
		t.Fatalf("backupFilesSubEntry() error = %v", err)
	}

	// Nothing should be backed up
	backedUpFile := filepath.Join(backupPath, "missing.txt")
	if pathExists(backedUpFile) {
		t.Error("file should not be backed up when target doesn't exist")
	}
}

//nolint:dupl // similar test structure is intentional
func TestBackupFolderSubEntry_SourceMissing(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Target doesn't exist
	targetDir := filepath.Join(tmpDir, "target")
	backupRoot := filepath.Join(tmpDir, "backup")
	backupPath := filepath.Join(backupRoot, "test")

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "test",
				Entries: []config.SubEntry{
					{
						Name: "config",

						Backup: "./test",
						Targets: map[string]string{
							"linux": targetDir,
						},
					},
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	subEntry := cfg.Applications[0].Entries[0]

	err := mgr.backupFolderSubEntry("test", subEntry, backupPath, targetDir)
	if err != nil {
		t.Fatalf("backupFolderSubEntry() error = %v", err)
	}

	// Backup should not be created
	if pathExists(backupPath) {
		t.Error("backup should not be created when target doesn't exist")
	}
}

func TestBackupFilesSubEntry_MissingFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Target directory exists but specific file doesn't
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, "exists.txt"), []byte("exists"), 0600); err != nil {
		t.Fatal(err)
	}

	backupRoot := filepath.Join(tmpDir, "backup")
	backupPath := filepath.Join(backupRoot, "test")

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "test",
				Entries: []config.SubEntry{
					{
						Name: "config",

						Files:  []string{"exists.txt", "missing.txt"},
						Backup: "./test",
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
	mgr.Verbose = true

	subEntry := cfg.Applications[0].Entries[0]

	err := mgr.backupFilesSubEntry("test", subEntry, backupPath, homeDir)
	if err != nil {
		t.Fatalf("backupFilesSubEntry() error = %v", err)
	}

	// Only existing file should be backed up
	backedUpExists := filepath.Join(backupPath, "exists.txt")
	backedUpMissing := filepath.Join(backupPath, "missing.txt")

	if !pathExists(backedUpExists) {
		t.Error("exists.txt should be backed up")
	}

	if pathExists(backedUpMissing) {
		t.Error("missing.txt should not be backed up")
	}
}

func TestBackupV3_ErrorInSubEntry(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a target that's a file (invalid for folder backup)
	targetFile := filepath.Join(tmpDir, "target")
	if err := os.WriteFile(targetFile, []byte("not a folder"), 0600); err != nil {
		t.Fatal(err)
	}

	backupRoot := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(filepath.Join(backupRoot, "test"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "test",
				Entries: []config.SubEntry{
					{
						Name: "config",

						Backup: "./test",
						Targets: map[string]string{
							"linux": targetFile,
						},
					},
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	err := mgr.Backup()
	// Should log error but not fail completely
	if err != nil {
		t.Logf("Backup() returned: %v", err)
	}
}

func TestBackup_SkipsGitEntries(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	backupRoot := filepath.Join(tmpDir, "backup")

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "test-app",
				Entries: []config.SubEntry{
					{
						Name: "git-entry",

						Targets: map[string]string{
							"linux": filepath.Join(tmpDir, "source"),
						},
					},
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	err := mgr.Backup()
	if err != nil {
		t.Fatalf("Backup() error = %v", err)
	}

	// Backup directory should not be created (git entry was skipped)
	if pathExists(backupRoot) {
		entries, _ := os.ReadDir(backupRoot)
		if len(entries) != 0 {
			t.Error("Expected no backup files for git entry")
		}
	}
}
