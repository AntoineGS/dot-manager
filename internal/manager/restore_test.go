package manager

import (
	"errors"
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

func TestRestoreEntry_PathError(t *testing.T) {
	tests := []struct {
		setup       func(t *testing.T, tmpDir string) (*Manager, config.Entry)
		name        string
		wantErr     bool
		wantPathErr bool
	}{
		{
			name: "symlink_creation_failure_returns_path_error",
			setup: func(t *testing.T, tmpDir string) (*Manager, config.Entry) {
				// Create backup but make target dir read-only
				backupRoot := filepath.Join(tmpDir, "backup")
				backupDir := filepath.Join(backupRoot, "test")
				os.MkdirAll(backupDir, 0755)

				targetDir := filepath.Join(tmpDir, "readonly")
				os.MkdirAll(targetDir, 0444) // read-only

				cfg := &config.Config{
					BackupRoot: backupRoot,
					Version:    2,
					Entries: []config.Entry{
						{
							Name:   "test",
							Backup: "./test",
							Targets: map[string]string{
								"linux": filepath.Join(targetDir, "config"),
							},
						},
					},
				}

				plat := &platform.Platform{OS: platform.OSLinux}
				mgr := New(cfg, plat)

				return mgr, cfg.Entries[0]
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
	os.MkdirAll(backupRoot, 0755)
	os.WriteFile(filepath.Join(backupRoot, ".bashrc"), []byte("bashrc content"), 0644)
	os.WriteFile(filepath.Join(backupRoot, ".profile"), []byte("profile content"), 0644)

	// Target directory
	homeDir := filepath.Join(tmpDir, "home")
	os.MkdirAll(homeDir, 0755)

	cfg := &config.Config{
		Version:    3,
		BackupRoot: filepath.Join(tmpDir, "backup"),
		Applications: []config.Application{
			{
				Name: "bash",
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
	content, err := os.ReadFile(bashrcTarget)
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
	os.MkdirAll(backupDir, 0755)
	os.WriteFile(filepath.Join(backupDir, "file.txt"), []byte("new content"), 0644)

	// Create existing file at target
	targetDir := filepath.Join(tmpDir, "target")
	os.MkdirAll(targetDir, 0755)
	existingFile := filepath.Join(targetDir, "file.txt")
	os.WriteFile(existingFile, []byte("old content"), 0644)

	cfg := &config.Config{
		Version:    2,
		BackupRoot: filepath.Join(tmpDir, "backup"),
		Entries: []config.Entry{
			{
				Name:   "test",
				Files:  []string{"file.txt"},
				Backup: "./config",
				Targets: map[string]string{
					"linux": targetDir,
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
	content, err := os.ReadFile(existingFile)
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
	os.MkdirAll(source, 0755)
	os.WriteFile(filepath.Join(source, "file.txt"), []byte("content"), 0644)

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
	os.WriteFile(source, []byte("content"), 0644)

	target := filepath.Join(tmpDir, "target.txt")

	err := createSymlink(source, target, false)
	if err != nil {
		t.Fatalf("createSymlink() error = %v", err)
	}

	if !isSymlink(target) {
		t.Error("target is not a symlink")
	}

	// Verify content through symlink
	content, _ := os.ReadFile(target)
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
	os.MkdirAll(configBackup, 0755)
	os.WriteFile(filepath.Join(configBackup, "config.json"), []byte("config"), 0644)

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
						Name:   "config",
						Type:   "config",
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
	os.MkdirAll(configBackup, 0755)
	os.WriteFile(filepath.Join(configBackup, ".bashrc"), []byte("bashrc"), 0644)
	os.WriteFile(filepath.Join(configBackup, ".profile"), []byte("profile"), 0644)
	os.WriteFile(filepath.Join(configBackup, ".bash_aliases"), []byte("aliases"), 0644)

	// Target directory
	homeDir := filepath.Join(tmpDir, "home")
	os.MkdirAll(homeDir, 0755)

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "shell",
				Entries: []config.SubEntry{
					{
						Name:   "config",
						Type:   "config",
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
	os.MkdirAll(targetDir, 0755)
	os.WriteFile(filepath.Join(targetDir, "existing.txt"), []byte("existing"), 0644)

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
						Name:   "config",
						Type:   "config",
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
	os.MkdirAll(homeDir, 0755)
	os.WriteFile(filepath.Join(homeDir, ".bashrc"), []byte("existing bashrc"), 0644)

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
						Name:   "config",
						Type:   "config",
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

	content, _ := os.ReadFile(adoptedFile)
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
	os.MkdirAll(backupPath, 0755)
	bashrcSrc := filepath.Join(backupPath, ".bashrc")
	os.WriteFile(bashrcSrc, []byte("bashrc content"), 0644)

	// Create target with existing symlink
	homeDir := filepath.Join(tmpDir, "home")
	os.MkdirAll(homeDir, 0755)
	bashrcTarget := filepath.Join(homeDir, ".bashrc")
	os.Symlink(bashrcSrc, bashrcTarget)

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "shell",
				Entries: []config.SubEntry{
					{
						Name:   "config",
						Type:   "config",
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
	os.MkdirAll(backupPath, 0755)
	os.WriteFile(filepath.Join(backupPath, "init.lua"), []byte("config"), 0644)

	// Create target with existing symlink
	targetDir := filepath.Join(tmpDir, "home", ".config", "nvim")
	os.MkdirAll(filepath.Dir(targetDir), 0755)
	os.Symlink(backupPath, targetDir)

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "nvim",
				Entries: []config.SubEntry{
					{
						Name:   "config",
						Type:   "config",
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
	os.MkdirAll(backupPath, 0755)
	os.WriteFile(filepath.Join(backupPath, ".bashrc"), []byte("new content"), 0644)

	// Create existing regular file at target
	homeDir := filepath.Join(tmpDir, "home")
	os.MkdirAll(homeDir, 0755)
	existingFile := filepath.Join(homeDir, ".bashrc")
	os.WriteFile(existingFile, []byte("old content"), 0644)

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "shell",
				Entries: []config.SubEntry{
					{
						Name:   "config",
						Type:   "config",
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
	content, _ := os.ReadFile(existingFile)
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
	os.MkdirAll(backupPath, 0755)
	os.WriteFile(filepath.Join(backupPath, "new.lua"), []byte("new config"), 0644)

	// Create existing folder at target
	targetDir := filepath.Join(tmpDir, "home", ".config", "nvim")
	os.MkdirAll(targetDir, 0755)
	os.WriteFile(filepath.Join(targetDir, "old.lua"), []byte("old config"), 0644)

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "nvim",
				Entries: []config.SubEntry{
					{
						Name:   "config",
						Type:   "config",
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

	// Old file should not exist at target
	oldFile := filepath.Join(targetDir, "old.lua")
	if pathExists(oldFile) {
		t.Error("old.lua should not exist at target (folder was replaced)")
	}

	// New file should be accessible through symlink
	newFile := filepath.Join(targetDir, "new.lua")

	content, _ := os.ReadFile(newFile)
	if string(content) != "new config" {
		t.Errorf("content = %q, want %q", string(content), "new config")
	}
}

func TestRestoreV3_SkipsWrongOS(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	backupRoot := filepath.Join(tmpDir, "backup")
	os.MkdirAll(filepath.Join(backupRoot, "test"), 0755)

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "test-app",
				Entries: []config.SubEntry{
					{
						Name:   "config",
						Type:   "config",
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
	os.MkdirAll(targetDir, 0755)

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	entry := config.Entry{Name: "test", Files: []string{"missing.txt"}}

	err := mgr.restoreFiles(entry, backupDir, targetDir)
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

	err := mgr.restoreFolder(entry, backupDir, targetDir)
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
	os.MkdirAll(backupRoot, 0755)
	invalidBackup := filepath.Join(backupRoot, "invalid")
	os.WriteFile(invalidBackup, []byte("not a directory"), 0644)

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "test",
				Entries: []config.SubEntry{
					{
						Name:   "config",
						Type:   "config",
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
	os.MkdirAll(targetDir, 0755)

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "test",
				Entries: []config.SubEntry{
					{
						Name:   "config",
						Type:   "config",
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
						Name:   "config",
						Type:   "config",
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
