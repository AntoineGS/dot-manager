package manager

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

func TestRestoreFolder(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create source directory with content
	srcDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(srcDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "config.txt"), []byte("config"), 0600); err != nil {
		t.Fatal(err)
	}

	// Target directory
	targetDir := filepath.Join(tmpDir, "target", "config")

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	entry := config.Entry{Name: "test"}

	err := mgr.RestoreFolder(entry, srcDir, targetDir)
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
	if err := os.MkdirAll(srcDir, 0750); err != nil {
		t.Fatal(err)
	}

	targetDir := filepath.Join(tmpDir, "target")
	if err := os.Symlink(srcDir, targetDir); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	entry := config.Entry{Name: "test"}

	err := mgr.RestoreFolder(entry, srcDir, targetDir)
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
	if err := os.MkdirAll(srcDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("content1"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file2.txt"), []byte("content2"), 0600); err != nil {
		t.Fatal(err)
	}

	targetDir := filepath.Join(tmpDir, "target")

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	entry := config.Entry{Name: "test", Files: []string{"file1.txt", "file2.txt"}}

	err := mgr.RestoreFiles(entry, srcDir, targetDir)
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
	if err := os.MkdirAll(srcDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "config.txt"), []byte("new content"), 0600); err != nil {
		t.Fatal(err)
	}

	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "config.txt"), []byte("old content"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	entry := config.Entry{Name: "test", Files: []string{"config.txt"}}

	err := mgr.RestoreFiles(entry, srcDir, targetDir)
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
	if err := os.MkdirAll(srcDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "config.txt"), []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}

	targetDir := filepath.Join(tmpDir, "target")

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.DryRun = true

	entry := config.Entry{Name: "test"}

	err := mgr.RestoreFolder(entry, srcDir, targetDir)
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
	if err := os.MkdirAll(nvimBackup, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvimBackup, "init.lua"), []byte("vim config"), 0600); err != nil {
		t.Fatal(err)
	}

	bashBackup := filepath.Join(backupRoot, "bash")
	if err := os.MkdirAll(bashBackup, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(bashBackup, ".bashrc"), []byte("bash config"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create config
	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "nvim",
				Entries: []config.SubEntry{
					{
						Name:   "config",
						Files:  []string{},
						Backup: "./nvim",
						Targets: map[string]string{
							"linux": filepath.Join(tmpDir, "home", ".config", "nvim"),
						},
					},
				},
			},
			{
				Name: "bash",
				Entries: []config.SubEntry{
					{
						Name:   "config",
						Files:  []string{".bashrc"},
						Backup: "./bash",
						Targets: map[string]string{
							"linux": filepath.Join(tmpDir, "home"),
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

func TestRestoreV3Application(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backup structure
	backupRoot := filepath.Join(tmpDir, "backup")
	nvimBackup := filepath.Join(backupRoot, "nvim")
	if err := os.MkdirAll(nvimBackup, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvimBackup, "init.lua"), []byte("vim config"), 0600); err != nil {
		t.Fatal(err)
	}

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
	if err := os.MkdirAll(configBackup, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configBackup, "init.lua"), []byte("config"), 0600); err != nil {
		t.Fatal(err)
	}

	dataBackup := filepath.Join(backupRoot, "nvim-data")
	if err := os.MkdirAll(dataBackup, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dataBackup, "lazy.lua"), []byte("data"), 0600); err != nil {
		t.Fatal(err)
	}

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

func TestRestoreEntry_PathError(t *testing.T) {
	tests := []struct {
		setup       func(t *testing.T, tmpDir string) (*Manager, config.Entry)
		name        string
		wantErr     bool
		wantPathErr bool
	}{
		{
			name: "symlink_creation_failure_returns_path_error",
			setup: func(_ *testing.T, tmpDir string) (*Manager, config.Entry) {
				// Create backup but make target dir read-only
				backupRoot := filepath.Join(tmpDir, "backup")
				backupDir := filepath.Join(backupRoot, "test")
				if err := os.MkdirAll(backupDir, 0750); err != nil {
					t.Fatal(err)
				}

				targetDir := filepath.Join(tmpDir, "readonly")
				_ = os.MkdirAll(targetDir, 0444) //nolint:gosec // intentionally read-only for test

				cfg := &config.Config{
					BackupRoot: backupRoot,
					Version:    3,
					Applications: []config.Application{
						{
							Name: "test",
							Entries: []config.SubEntry{
								{
									Name:   "config",
									Backup: "./test",
									Targets: map[string]string{
										"linux": filepath.Join(targetDir, "config"),
									},
								},
							},
						},
					},
				}

				plat := &platform.Platform{OS: platform.OSLinux}
				mgr := New(cfg, plat)

				ctx := &config.FilterContext{}
				entries := cfg.GetAllConfigSubEntries(ctx)
				// Convert SubEntry to Entry for compatibility with test
				entry := config.Entry{
					Name:    entries[0].Name,
					Backup:  entries[0].Backup,
					Targets: entries[0].Targets,
					Files:   entries[0].Files,
					Sudo:    entries[0].Sudo,
				}
				return mgr, entry
			},
			wantErr:     true,
			wantPathErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			mgr, entry := tt.setup(t, tmpDir)

			target := entry.GetTarget(mgr.Platform.OS)
			err := mgr.restoreEntry(entry, target)

			if (err != nil) != tt.wantErr {
				t.Errorf("restoreEntry() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantPathErr {
				var pathErr *PathError
				if !errors.As(err, &pathErr) {
					t.Errorf("error is not PathError: %v", err)
				}
			}
		})
	}
}

func TestRestoreV3_FilesSubEntry(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backup files
	backupRoot := filepath.Join(tmpDir, "backup", "bash")
	if err := os.MkdirAll(backupRoot, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupRoot, ".bashrc"), []byte("bashrc content"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupRoot, ".profile"), []byte("profile content"), 0600); err != nil {
		t.Fatal(err)
	}

	// Target directory
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version:    3,
		BackupRoot: filepath.Join(tmpDir, "backup"),
		Applications: []config.Application{
			{
				Name: "bash",
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

	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	// Check symlinks were created
	bashrcTarget := filepath.Join(homeDir, ".bashrc")
	profileTarget := filepath.Join(homeDir, ".profile")

	if !isSymlink(bashrcTarget) {
		t.Error(".bashrc is not a symlink")
	}

	if !isSymlink(profileTarget) {
		t.Error(".profile is not a symlink")
	}

	// Read through symlinks to verify content
	content, err := os.ReadFile(bashrcTarget) //nolint:gosec // test file
	if err != nil {
		t.Fatalf("Failed to read .bashrc: %v", err)
	}

	if string(content) != "bashrc content" {
		t.Errorf("Content = %q, want %q", string(content), "bashrc content")
	}
}

func TestRestore_ReplacesExistingFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backup
	backupDir := filepath.Join(tmpDir, "backup", "config")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "file.txt"), []byte("new content"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create existing file at target
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, 0750); err != nil {
		t.Fatal(err)
	}
	existingFile := filepath.Join(targetDir, "file.txt")
	if err := os.WriteFile(existingFile, []byte("old content"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version:    3,
		BackupRoot: filepath.Join(tmpDir, "backup"),
		Applications: []config.Application{
			{
				Name: "test",
				Entries: []config.SubEntry{
					{
						Name:   "config",
						Files:  []string{"file.txt"},
						Backup: "./config",
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

	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	// Should now be a symlink
	if !isSymlink(existingFile) {
		t.Error("file.txt should be a symlink")
	}

	// Read content through symlink
	content, err := os.ReadFile(existingFile) //nolint:gosec // test file
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}

	if string(content) != "new content" {
		t.Errorf("Content = %q, want %q", string(content), "new content")
	}
}

func TestCreateSymlink_Success(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	source := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(source, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(source, "file.txt"), []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(tmpDir, "target")

	err := createSymlink(source, target, false)
	if err != nil {
		t.Fatalf("createSymlink() error = %v", err)
	}

	if !isSymlink(target) {
		t.Error("target is not a symlink")
	}

	link, _ := os.Readlink(target)
	if link != source {
		t.Errorf("symlink target = %q, want %q", link, source)
	}
}

func TestCreateSymlink_SourceNotExist(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	source := filepath.Join(tmpDir, "nonexistent")
	target := filepath.Join(tmpDir, "target")

	err := createSymlink(source, target, false)
	if err == nil {
		t.Fatal("createSymlink() should fail when source doesn't exist")
	}

	var pathErr *PathError
	if !errors.As(err, &pathErr) {
		t.Errorf("error is not PathError: %v", err)
	}
}

func TestCreateSymlink_FileSource(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	source := filepath.Join(tmpDir, "source.txt")
	if err := os.WriteFile(source, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}

	target := filepath.Join(tmpDir, "target.txt")

	err := createSymlink(source, target, false)
	if err != nil {
		t.Fatalf("createSymlink() error = %v", err)
	}

	if !isSymlink(target) {
		t.Error("target is not a symlink")
	}

	// Verify content through symlink
	content, _ := os.ReadFile(target) //nolint:gosec // test file
	if string(content) != "content" {
		t.Errorf("content = %q, want %q", string(content), "content")
	}
}

func TestRestoreV3_FolderSubEntry(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backup structure
	backupRoot := filepath.Join(tmpDir, "backup")
	configBackup := filepath.Join(backupRoot, "app-config")
	if err := os.MkdirAll(configBackup, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configBackup, "config.json"), []byte("config"), 0600); err != nil {
		t.Fatal(err)
	}

	// Target directory
	targetDir := filepath.Join(tmpDir, "home", ".config", "app")

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "test-app",
				Entries: []config.SubEntry{
					{
						Name: "config",

						Backup: "./app-config",
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

	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	// Check symlink was created
	if !isSymlink(targetDir) {
		t.Error("target is not a symlink")
	}

	link, _ := os.Readlink(targetDir)
	if link != configBackup {
		t.Errorf("symlink target = %q, want %q", link, configBackup)
	}
}

func TestRestoreV3_FilesSubEntry_MultipleFiles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backup structure with multiple files
	backupRoot := filepath.Join(tmpDir, "backup")
	configBackup := filepath.Join(backupRoot, "shell")
	if err := os.MkdirAll(configBackup, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configBackup, ".bashrc"), []byte("bashrc"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configBackup, ".profile"), []byte("profile"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configBackup, ".bash_aliases"), []byte("aliases"), 0600); err != nil {
		t.Fatal(err)
	}

	// Target directory
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "shell",
				Entries: []config.SubEntry{
					{
						Name: "config",

						Files:  []string{".bashrc", ".profile", ".bash_aliases"},
						Backup: "./shell",
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

	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	// Check all symlinks were created
	files := []string{".bashrc", ".profile", ".bash_aliases"}
	for _, file := range files {
		targetFile := filepath.Join(homeDir, file)
		if !isSymlink(targetFile) {
			t.Errorf("%s is not a symlink", file)
		}

		link, _ := os.Readlink(targetFile)
		expectedLink := filepath.Join(configBackup, file)

		if link != expectedLink {
			t.Errorf("symlink for %s = %q, want %q", file, link, expectedLink)
		}
	}
}

func TestRestoreV3_FolderSubEntry_Adoption(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create existing target (to be adopted)
	targetDir := filepath.Join(tmpDir, "home", ".config", "app")
	if err := os.MkdirAll(targetDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "existing.txt"), []byte("existing"), 0600); err != nil {
		t.Fatal(err)
	}

	backupRoot := filepath.Join(tmpDir, "backup")
	backupPath := filepath.Join(backupRoot, "app-config")

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "test-app",
				Entries: []config.SubEntry{
					{
						Name: "config",

						Backup: "./app-config",
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

	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	// Check target was adopted into backup
	adoptedFile := filepath.Join(backupPath, "existing.txt")
	if !pathExists(adoptedFile) {
		t.Error("existing.txt was not adopted into backup")
	}

	// Check symlink was created
	if !isSymlink(targetDir) {
		t.Error("target is not a symlink")
	}
}

func TestRestoreV3_FilesSubEntry_Adoption(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create existing target files (to be adopted)
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(homeDir, ".bashrc"), []byte("existing bashrc"), 0600); err != nil {
		t.Fatal(err)
	}

	backupRoot := filepath.Join(tmpDir, "backup")
	backupPath := filepath.Join(backupRoot, "shell")

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "shell",
				Entries: []config.SubEntry{
					{
						Name: "config",

						Files:  []string{".bashrc"},
						Backup: "./shell",
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

	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	// Check file was adopted
	adoptedFile := filepath.Join(backupPath, ".bashrc")
	if !pathExists(adoptedFile) {
		t.Error(".bashrc was not adopted into backup")
	}

	content, _ := os.ReadFile(adoptedFile) //nolint:gosec // test file
	if string(content) != "existing bashrc" {
		t.Errorf("adopted content = %q, want %q", string(content), "existing bashrc")
	}

	// Check symlink was created
	targetFile := filepath.Join(homeDir, ".bashrc")
	if !isSymlink(targetFile) {
		t.Error(".bashrc is not a symlink")
	}
}

func TestRestoreV3_FilesSubEntry_SkipsExistingSymlinks(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backup
	backupRoot := filepath.Join(tmpDir, "backup")
	backupPath := filepath.Join(backupRoot, "shell")
	if err := os.MkdirAll(backupPath, 0750); err != nil {
		t.Fatal(err)
	}
	bashrcSrc := filepath.Join(backupPath, ".bashrc")
	if err := os.WriteFile(bashrcSrc, []byte("bashrc content"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create target with existing symlink
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0750); err != nil {
		t.Fatal(err)
	}
	bashrcTarget := filepath.Join(homeDir, ".bashrc")
	if err := os.Symlink(bashrcSrc, bashrcTarget); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "shell",
				Entries: []config.SubEntry{
					{
						Name: "config",

						Files:  []string{".bashrc"},
						Backup: "./shell",
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

	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	// Symlink should still point to same source
	link, _ := os.Readlink(bashrcTarget)
	if link != bashrcSrc {
		t.Errorf("symlink changed, got %q, want %q", link, bashrcSrc)
	}
}

func TestRestoreV3_FolderSubEntry_SkipsExistingSymlink(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backup
	backupRoot := filepath.Join(tmpDir, "backup")
	backupPath := filepath.Join(backupRoot, "nvim")
	if err := os.MkdirAll(backupPath, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupPath, "init.lua"), []byte("config"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create target with existing symlink
	targetDir := filepath.Join(tmpDir, "home", ".config", "nvim")
	if err := os.MkdirAll(filepath.Dir(targetDir), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(backupPath, targetDir); err != nil {
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

	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	// Symlink should still exist and point to same target
	link, _ := os.Readlink(targetDir)
	if link != backupPath {
		t.Errorf("symlink changed, got %q, want %q", link, backupPath)
	}
}

func TestRestoreV3_FilesSubEntry_ReplacesExisting(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backup
	backupRoot := filepath.Join(tmpDir, "backup")
	backupPath := filepath.Join(backupRoot, "shell")
	if err := os.MkdirAll(backupPath, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupPath, ".bashrc"), []byte("new content"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create existing regular file at target
	homeDir := filepath.Join(tmpDir, "home")
	if err := os.MkdirAll(homeDir, 0750); err != nil {
		t.Fatal(err)
	}
	existingFile := filepath.Join(homeDir, ".bashrc")
	if err := os.WriteFile(existingFile, []byte("old content"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "shell",
				Entries: []config.SubEntry{
					{
						Name: "config",

						Files:  []string{".bashrc"},
						Backup: "./shell",
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

	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	// Should now be a symlink
	if !isSymlink(existingFile) {
		t.Error(".bashrc should be a symlink")
	}

	// Read content through symlink
	content, _ := os.ReadFile(existingFile) //nolint:gosec // test file
	if string(content) != "new content" {
		t.Errorf("content = %q, want %q", string(content), "new content")
	}
}

func TestRestoreV3_FolderSubEntry_ReplacesExisting(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backup
	backupRoot := filepath.Join(tmpDir, "backup")
	backupPath := filepath.Join(backupRoot, "nvim")
	if err := os.MkdirAll(backupPath, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupPath, "new.lua"), []byte("new config"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create existing folder at target
	targetDir := filepath.Join(tmpDir, "home", ".config", "nvim")
	if err := os.MkdirAll(targetDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "old.lua"), []byte("old config"), 0600); err != nil {
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
							"linux": targetDir,
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

	// Should now be a symlink
	if !isSymlink(targetDir) {
		t.Error("target should be a symlink")
	}

	// Old file should exist at backup (merged from target)
	oldFileBackup := filepath.Join(backupPath, "old.lua")
	if !pathExists(oldFileBackup) {
		t.Error("old.lua should exist in backup (merged from target)")
	}
	content, _ := os.ReadFile(oldFileBackup) //nolint:gosec // test file
	if string(content) != "old config" {
		t.Errorf("old.lua content = %q, want %q", string(content), "old config")
	}

	// Old file should be accessible through symlink (points to backup)
	oldFile := filepath.Join(targetDir, "old.lua")
	if !pathExists(oldFile) {
		t.Error("old.lua should be accessible through symlink")
	}

	// New file should be accessible through symlink
	newFile := filepath.Join(targetDir, "new.lua")

	content, _ = os.ReadFile(newFile) //nolint:gosec // test file
	if string(content) != "new config" {
		t.Errorf("content = %q, want %q", string(content), "new config")
	}
}

func TestRestoreV3_SkipsWrongOS(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	backupRoot := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(filepath.Join(backupRoot, "test"), 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "test-app",
				Entries: []config.SubEntry{
					{
						Name: "config",

						Backup: "./test",
						Targets: map[string]string{
							"windows": filepath.Join(tmpDir, "target"),
						},
					},
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	// Target should not be created (wrong OS)
	targetDir := filepath.Join(tmpDir, "target")
	if pathExists(targetDir) {
		t.Error("target should not exist for wrong OS")
	}
}

func TestRestoreFiles_SourceMissing(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// No source files exist
	backupDir := filepath.Join(tmpDir, "backup")
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, 0750); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	entry := config.Entry{Name: "test", Files: []string{"missing.txt"}}

	err := mgr.RestoreFiles(entry, backupDir, targetDir)
	if err != nil {
		t.Fatalf("restoreFiles() error = %v", err)
	}

	// No symlink should be created (source doesn't exist)
	targetFile := filepath.Join(targetDir, "missing.txt")
	if pathExists(targetFile) {
		t.Error("symlink should not be created for missing source")
	}
}

func TestRestoreFolder_SourceMissing(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// No source folder exists
	backupDir := filepath.Join(tmpDir, "backup", "missing")
	targetDir := filepath.Join(tmpDir, "target")

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	entry := config.Entry{Name: "test"}

	err := mgr.RestoreFolder(entry, backupDir, targetDir)
	if err != nil {
		t.Fatalf("restoreFolder() error = %v", err)
	}

	// Target should not be created (source doesn't exist)
	if pathExists(targetDir) {
		t.Error("target should not be created when source is missing")
	}
}

func TestRestoreV3_ErrorHandling(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backup that's actually a file (invalid)
	backupRoot := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(backupRoot, 0750); err != nil {
		t.Fatal(err)
	}
	invalidBackup := filepath.Join(backupRoot, "invalid")
	if err := os.WriteFile(invalidBackup, []byte("not a directory"), 0600); err != nil {
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

						Backup: "./invalid",
						Targets: map[string]string{
							"linux": filepath.Join(tmpDir, "target"),
						},
					},
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	err := mgr.Restore()
	// Should log error but not fail completely
	if err != nil {
		t.Logf("Restore() returned: %v", err)
	}
}

func TestRestoreFilesSubEntry_SourceMissing(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	backupRoot := filepath.Join(tmpDir, "backup")
	backupPath := filepath.Join(backupRoot, "missing")
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, 0750); err != nil {
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

						Files:  []string{"file.txt"},
						Backup: "./missing",
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

	err := mgr.restoreFilesSubEntry("test", subEntry, backupPath, targetDir)
	if err != nil {
		t.Fatalf("restoreFilesSubEntry() error = %v", err)
	}

	// No symlink should be created
	targetFile := filepath.Join(targetDir, "file.txt")
	if pathExists(targetFile) {
		t.Error("symlink should not be created for missing source")
	}
}

//nolint:dupl // similar test structure is intentional
func TestRestoreFolderSubEntry_SourceMissing(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	backupRoot := filepath.Join(tmpDir, "backup")
	backupPath := filepath.Join(backupRoot, "missing")
	targetDir := filepath.Join(tmpDir, "target")

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "test",
				Entries: []config.SubEntry{
					{
						Name: "config",

						Backup: "./missing",
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

	err := mgr.restoreFolderSubEntry("test", subEntry, backupPath, targetDir)
	if err != nil {
		t.Fatalf("restoreFolderSubEntry() error = %v", err)
	}

	// Target should not be created
	if pathExists(targetDir) {
		t.Error("target should not be created when source is missing")
	}
}

func TestRestoreFolder_RecreatesChangedSymlink(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create correct source directory
	correctSrc := filepath.Join(tmpDir, "correct-source")
	if err := os.MkdirAll(correctSrc, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(correctSrc, "config.txt"), []byte("correct"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create wrong source directory (symlink currently points here)
	wrongSrc := filepath.Join(tmpDir, "wrong-source")
	if err := os.MkdirAll(wrongSrc, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(wrongSrc, "config.txt"), []byte("wrong"), 0600); err != nil {
		t.Fatal(err)
	}

	// Target is a symlink pointing to the wrong location
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.Symlink(wrongSrc, targetDir); err != nil {
		t.Fatal(err)
	}

	// Verify it points to wrong source
	link, err := os.Readlink(targetDir)
	if err != nil {
		t.Fatal(err)
	}
	if link != wrongSrc {
		t.Fatalf("Setup failed: symlink should point to %s, got %s", wrongSrc, link)
	}

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	entry := config.Entry{Name: "test"}

	err = mgr.RestoreFolder(entry, correctSrc, targetDir)
	if err != nil {
		t.Fatalf("restoreFolder() error = %v", err)
	}

	// Check symlink still exists
	if !isSymlink(targetDir) {
		t.Error("Target should still be a symlink")
	}

	// Check symlink now points to correct source
	link, err = os.Readlink(targetDir)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	if link != correctSrc {
		t.Errorf("Symlink should point to %s, got %s", correctSrc, link)
	}
}

func TestRestoreFiles_RecreatesChangedSymlink(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create correct source
	correctSrc := filepath.Join(tmpDir, "correct-source")
	if err := os.MkdirAll(correctSrc, 0750); err != nil {
		t.Fatal(err)
	}
	correctFile := filepath.Join(correctSrc, "config.txt")
	if err := os.WriteFile(correctFile, []byte("correct"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create wrong source (symlink currently points here)
	wrongSrc := filepath.Join(tmpDir, "wrong-source")
	if err := os.MkdirAll(wrongSrc, 0750); err != nil {
		t.Fatal(err)
	}
	wrongFile := filepath.Join(wrongSrc, "config.txt")
	if err := os.WriteFile(wrongFile, []byte("wrong"), 0600); err != nil {
		t.Fatal(err)
	}

	// Target directory with symlink pointing to wrong file
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, 0750); err != nil {
		t.Fatal(err)
	}
	targetFile := filepath.Join(targetDir, "config.txt")
	if err := os.Symlink(wrongFile, targetFile); err != nil {
		t.Fatal(err)
	}

	// Verify it points to wrong file
	link, err := os.Readlink(targetFile)
	if err != nil {
		t.Fatal(err)
	}
	if link != wrongFile {
		t.Fatalf("Setup failed: symlink should point to %s, got %s", wrongFile, link)
	}

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	entry := config.Entry{
		Name:  "test",
		Files: []string{"config.txt"},
	}

	err = mgr.RestoreFiles(entry, correctSrc, targetDir)
	if err != nil {
		t.Fatalf("restoreFiles() error = %v", err)
	}

	// Check symlink still exists
	if !isSymlink(targetFile) {
		t.Error("Target file should still be a symlink")
	}

	// Check symlink now points to correct source
	link, err = os.Readlink(targetFile)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	if link != correctFile {
		t.Errorf("Symlink should point to %s, got %s", correctFile, link)
	}
}

func TestRestoreFolder_MergesExistingContent(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backup with existing content
	backupDir := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "config.json"), []byte("backup config"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create target with additional content (to be merged)
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "local.json"), []byte("local config"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "cache.json"), []byte("cache data"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	entry := config.Entry{Name: "test"}

	err := mgr.RestoreFolder(entry, backupDir, targetDir)
	if err != nil {
		t.Fatalf("RestoreFolder() error = %v", err)
	}

	// Target should be a symlink now
	if !isSymlink(targetDir) {
		t.Error("target should be a symlink")
	}

	// Backup should contain merged files
	mergedFile1 := filepath.Join(backupDir, "local.json")
	if !pathExists(mergedFile1) {
		t.Error("local.json should be merged into backup")
	}

	mergedFile2 := filepath.Join(backupDir, "cache.json")
	if !pathExists(mergedFile2) {
		t.Error("cache.json should be merged into backup")
	}

	// Original backup file should still exist
	originalFile := filepath.Join(backupDir, "config.json")
	if !pathExists(originalFile) {
		t.Error("original backup file should still exist")
	}
}

func TestRestoreFolder_ConflictsRenamed(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backup with existing content
	backupDir := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "config.json"), []byte("backup config"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create target with conflicting file (same name as in backup)
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "config.json"), []byte("target config"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	entry := config.Entry{Name: "test"}

	err := mgr.RestoreFolder(entry, backupDir, targetDir)
	if err != nil {
		t.Fatalf("RestoreFolder() error = %v", err)
	}

	// Target should be a symlink now
	if !isSymlink(targetDir) {
		t.Error("target should be a symlink")
	}

	// Original backup file should still exist with original content
	originalFile := filepath.Join(backupDir, "config.json")
	if !pathExists(originalFile) {
		t.Error("original backup file should still exist")
	}

	content, err := os.ReadFile(originalFile) //nolint:gosec // test file
	if err != nil {
		t.Fatalf("Failed to read backup file: %v", err)
	}
	if string(content) != "backup config" {
		t.Errorf("backup file should have original content, got %q", string(content))
	}

	// Conflict file should be created with renamed name
	// Look for a file matching the pattern config_target_*.json
	entries, err := os.ReadDir(backupDir)
	if err != nil {
		t.Fatalf("Failed to read backup dir: %v", err)
	}

	conflictFound := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "config_target_") && strings.HasSuffix(entry.Name(), ".json") {
			conflictFound = true
			conflictFile := filepath.Join(backupDir, entry.Name())
			conflictContent, _ := os.ReadFile(conflictFile) //nolint:gosec // test file
			if string(conflictContent) != "target config" {
				t.Errorf("conflict file should have target content, got %q", string(conflictContent))
			}
			break
		}
	}

	if !conflictFound {
		t.Error("conflict file (config_target_*.json) should be created")
	}
}

func TestRestoreFiles_OnlyMergesListedFiles(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backup with listed file
	backupDir := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "config.txt"), []byte("backup config"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create target directory with listed file and unlisted file
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, 0750); err != nil {
		t.Fatal(err)
	}
	// Listed file - should be merged
	if err := os.WriteFile(filepath.Join(targetDir, "config.txt"), []byte("target config"), 0600); err != nil {
		t.Fatal(err)
	}
	// Unlisted file - should NOT be touched
	if err := os.WriteFile(filepath.Join(targetDir, "other.txt"), []byte("other content"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	entry := config.Entry{
		Name:  "test",
		Files: []string{"config.txt"}, // Only config.txt is listed
	}

	err := mgr.RestoreFiles(entry, backupDir, targetDir)
	if err != nil {
		t.Fatalf("RestoreFiles() error = %v", err)
	}

	// Listed file should be a symlink
	listedFile := filepath.Join(targetDir, "config.txt")
	if !isSymlink(listedFile) {
		t.Error("config.txt should be a symlink")
	}

	// Unlisted file should still exist as regular file (not touched)
	unlistedFile := filepath.Join(targetDir, "other.txt")
	if isSymlink(unlistedFile) {
		t.Error("other.txt should not be a symlink")
	}
	if !pathExists(unlistedFile) {
		t.Error("other.txt should still exist")
	}
	content, _ := os.ReadFile(unlistedFile) //nolint:gosec // test file
	if string(content) != "other content" {
		t.Errorf("other.txt content = %q, want %q", string(content), "other content")
	}

	// Listed file content in backup should have merged target's version
	// The target version should be renamed as conflict file
	entries, _ := os.ReadDir(backupDir)
	conflictFound := false
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), "config_target_") && strings.HasSuffix(entry.Name(), ".txt") {
			conflictFound = true
			conflictFile := filepath.Join(backupDir, entry.Name())
			conflictContent, _ := os.ReadFile(conflictFile) //nolint:gosec // test file
			if string(conflictContent) != "target config" {
				t.Errorf("conflict file should have target content, got %q", string(conflictContent))
			}
			break
		}
	}

	if !conflictFound {
		t.Error("conflict file (config_target_*.txt) should be created for the merged file")
	}
}
