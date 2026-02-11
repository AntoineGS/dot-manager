package manager

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/platform"
)

func TestNew(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	plat := &platform.Platform{OS: platform.OSLinux}

	mgr := New(cfg, plat)

	if mgr == nil {
		t.Fatal("New() returned nil")
	}

	if mgr.Config != cfg {
		t.Error("Config not set correctly")
	}

	if mgr.Platform != plat {
		t.Error("Platform not set correctly")
	}

	if mgr.DryRun != false {
		t.Error("DryRun should default to false")
	}

	if mgr.Verbose != false {
		t.Error("Verbose should default to false")
	}
}

func TestGetPaths(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Version: 3,
		Applications: []config.Application{
			{
				Name: "user-app",
				Entries: []config.SubEntry{
					{Name: "user-path", Backup: "./user", Targets: map[string]string{"linux": "~/.config"}},
				},
			},
			{
				Name: "root-app",
				Entries: []config.SubEntry{
					{Name: "root-path", Sudo: true, Backup: "./root", Targets: map[string]string{"linux": "/etc"}},
				},
			},
		},
	}

	// All paths are returned regardless of Root flag
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	// Use nil renderer (no when expressions = all match)
	paths := mgr.Config.GetAllConfigSubEntries(nil)

	if len(paths) != 2 {
		t.Fatalf("GetPaths() returned %d paths, want 2", len(paths))
	}

	// Verify both paths are present
	names := make(map[string]bool)
	for _, p := range paths {
		names[p.Name] = true
	}

	if !names["user-path"] {
		t.Error("GetPaths() should include 'user-path'")
	}

	if !names["root-path"] {
		t.Error("GetPaths() should include 'root-path'")
	}
}

func TestIsSymlink(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a regular file
	regularFile := filepath.Join(tmpDir, "regular.txt")
	if err := os.WriteFile(regularFile, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create a symlink
	symlinkFile := filepath.Join(tmpDir, "symlink.txt")
	if err := os.Symlink(regularFile, symlinkFile); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"regular file", regularFile, false},
		{"symlink", symlinkFile, true},
		{"non-existent", filepath.Join(tmpDir, "nonexistent"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isSymlink(tt.path)
			if got != tt.want {
				t.Errorf("isSymlink(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestPathExists(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	existingFile := filepath.Join(tmpDir, "exists.txt")
	if err := os.WriteFile(existingFile, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		path string
		want bool
	}{
		{"existing file", existingFile, true},
		{"existing dir", tmpDir, true},
		{"non-existent", filepath.Join(tmpDir, "nonexistent"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pathExists(tt.path)
			if got != tt.want {
				t.Errorf("pathExists(%q) = %v, want %v", tt.path, got, tt.want)
			}
		})
	}
}

func TestCopyFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "source.txt")
	content := []byte("test content")

	if err := os.WriteFile(srcFile, content, 0600); err != nil {
		t.Fatal(err)
	}

	dstFile := filepath.Join(tmpDir, "subdir", "dest.txt")

	if err := copyFile(srcFile, dstFile); err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	// Check content
	got, err := os.ReadFile(dstFile) //nolint:gosec // test file
	if err != nil {
		t.Fatalf("Failed to read destination: %v", err)
	}

	if string(got) != string(content) {
		t.Errorf("Content = %q, want %q", string(got), string(content))
	}
}

func TestCopyDir(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create source directory structure
	srcDir := filepath.Join(tmpDir, "source")
	if err := os.MkdirAll(filepath.Join(srcDir, "subdir"), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("file1"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("file2"), 0600); err != nil {
		t.Fatal(err)
	}

	dstDir := filepath.Join(tmpDir, "dest")

	if err := copyDir(srcDir, dstDir); err != nil {
		t.Fatalf("copyDir() error = %v", err)
	}

	// Check files exist
	if !pathExists(filepath.Join(dstDir, "file1.txt")) {
		t.Error("file1.txt not copied")
	}

	if !pathExists(filepath.Join(dstDir, "subdir", "file2.txt")) {
		t.Error("subdir/file2.txt not copied")
	}

	// Check content
	content, _ := os.ReadFile(filepath.Join(dstDir, "file1.txt")) //nolint:gosec // test file
	if string(content) != "file1" {
		t.Errorf("file1.txt content = %q, want %q", string(content), "file1")
	}
}

func TestRemoveAll(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Test removing regular file
	regularFile := filepath.Join(tmpDir, "regular.txt")
	if err := os.WriteFile(regularFile, []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := removeAll(regularFile); err != nil {
		t.Fatalf("removeAll(regular file) error = %v", err)
	}

	if pathExists(regularFile) {
		t.Error("Regular file still exists after removeAll")
	}

	// Test removing directory
	dir := filepath.Join(tmpDir, "testdir")
	if err := os.MkdirAll(dir, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "file.txt"), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := removeAll(dir); err != nil {
		t.Fatalf("removeAll(dir) error = %v", err)
	}

	if pathExists(dir) {
		t.Error("Directory still exists after removeAll")
	}

	// Test that symlinks are not removed
	target := filepath.Join(tmpDir, "target.txt")
	if err := os.WriteFile(target, []byte("target"), 0600); err != nil {
		t.Fatal(err)
	}
	symlink := filepath.Join(tmpDir, "symlink.txt")
	if err := os.Symlink(target, symlink); err != nil {
		t.Fatal(err)
	}

	if err := removeAll(symlink); err != nil {
		t.Fatalf("removeAll(symlink) error = %v", err)
	}

	if !pathExists(symlink) {
		t.Error("Symlink should not be removed by removeAll")
	}

	// Test non-existent path (should not error)
	if err := removeAll(filepath.Join(tmpDir, "nonexistent")); err != nil {
		t.Fatalf("removeAll(nonexistent) error = %v", err)
	}
}

func TestResolvePath(t *testing.T) {
	t.Parallel()
	backupRoot := filepath.Join("/home", "user", "backups")
	cfg := &config.Config{
		BackupRoot: backupRoot,
	}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	tests := []struct {
		name string
		path string
		want string
	}{
		{"relative path", "./configs", filepath.Join(backupRoot, "configs")},
		{"absolute path", filepath.Join("/etc", "config"), filepath.Join("/etc", "config")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := mgr.resolvePath(tt.path)
			if got != tt.want {
				t.Errorf("resolvePath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestCopyFilePreservesPermissions(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "source.sh")
	content := []byte("#!/bin/bash\necho hello")

	if err := os.WriteFile(srcFile, content, 0600); err != nil {
		t.Fatal(err)
	}

	dstFile := filepath.Join(tmpDir, "dest.sh")

	if err := copyFile(srcFile, dstFile); err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	// Check permissions were preserved
	srcInfo, _ := os.Stat(srcFile)
	dstInfo, _ := os.Stat(dstFile)

	if srcInfo.Mode() != dstInfo.Mode() {
		t.Errorf("Permissions = %v, want %v", dstInfo.Mode(), srcInfo.Mode())
	}
}

func TestCopyFileNonexistentSource(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	srcFile := filepath.Join(tmpDir, "nonexistent.txt")
	dstFile := filepath.Join(tmpDir, "dest.txt")

	err := copyFile(srcFile, dstFile)
	if err == nil {
		t.Error("copyFile() should error for nonexistent source")
	}
}

func TestCopyDirNonexistentSource(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	srcDir := filepath.Join(tmpDir, "nonexistent")
	dstDir := filepath.Join(tmpDir, "dest")

	err := copyDir(srcDir, dstDir)
	if err == nil {
		t.Error("copyDir() should error for nonexistent source")
	}
}

func TestIsSymlinkWithDirectory(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a directory
	dir := filepath.Join(tmpDir, "realdir")
	if err := os.MkdirAll(dir, 0750); err != nil {
		t.Fatal(err)
	}

	// Create a symlink to the directory
	symlinkDir := filepath.Join(tmpDir, "symlinkdir")
	if err := os.Symlink(dir, symlinkDir); err != nil {
		t.Fatal(err)
	}

	if !isSymlink(symlinkDir) {
		t.Error("isSymlink() should return true for directory symlink")
	}

	if isSymlink(dir) {
		t.Error("isSymlink() should return false for real directory")
	}
}

func TestPathExistsWithSymlink(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create a file and symlink
	realFile := filepath.Join(tmpDir, "real.txt")
	if err := os.WriteFile(realFile, []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}

	symlink := filepath.Join(tmpDir, "link.txt")
	if err := os.Symlink(realFile, symlink); err != nil {
		t.Fatal(err)
	}

	if !pathExists(symlink) {
		t.Error("pathExists() should return true for symlink")
	}

	// Test with broken symlink
	brokenLink := filepath.Join(tmpDir, "broken.txt")
	if err := os.Symlink(filepath.Join(tmpDir, "nonexistent"), brokenLink); err != nil {
		t.Fatal(err)
	}

	// Broken symlinks still "exist" in terms of Lstat
	if !pathExists(brokenLink) {
		t.Error("pathExists() should return true for broken symlink (Lstat behavior)")
	}
}

func TestGetApplications(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Version: 3,
		Applications: []config.Application{
			{
				Name: "test-app",
				When: `{{ eq .OS "linux" }}`,
				Entries: []config.SubEntry{
					{Name: "config1", Backup: "./config1", Targets: map[string]string{"linux": "~/.config"}},
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux, Hostname: "test", User: "test"}
	mgr := New(cfg, plat)

	apps := mgr.GetApplications()
	if len(apps) != 1 {
		t.Fatalf("GetApplications() returned %d, want 1", len(apps))
	}

	if apps[0].Name != "test-app" {
		t.Errorf("Application name = %q, want %q", apps[0].Name, "test-app")
	}
}

func TestGetPackageEntries(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{
		Version: 3,
		Applications: []config.Application{
			{
				Name: "with-package",
				Package: &config.EntryPackage{
					Managers: map[string]config.ManagerValue{"pacman": {PackageName: "neovim"}},
				},
				Entries: []config.SubEntry{
					{
						Name:   "config",
						Backup: "./backup",
					},
				},
			},
			{
				Name: "without-package",
				Entries: []config.SubEntry{
					{
						Name:   "config",
						Backup: "./backup2",
					},
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	// Use nil renderer - apps with no when expression always match
	apps := mgr.Config.GetFilteredApplications(nil)

	// Should return all applications, filter for those with packages
	appsWithPkg := 0
	for _, app := range apps {
		if app.Package != nil {
			appsWithPkg++
			if app.Name != "with-package" {
				t.Errorf("Application name = %q, want %q", app.Name, "with-package")
			}
		}
	}

	if appsWithPkg != 1 {
		t.Fatalf("Found %d applications with packages, want 1", appsWithPkg)
	}
}

func TestLogWarn(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	// Just call it to get coverage - uses structured logging now
	mgr.logger.Warn("test warning message", slog.String("test", "value"))
	mgr.logger.Warn("test warning with arg", slog.String("arg", "value"))
}

func TestManager_SatisfiesInterface(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)

	// Verify Manager implements DotfileManager
	var _ DotfileManager = m
	var _ Restorer = m
	var _ Backuper = m
	var _ Lister = m
}
