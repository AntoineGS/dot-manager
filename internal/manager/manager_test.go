package manager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
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
		Version: 2,
		Entries: []config.Entry{
			{Name: "user-path", Backup: "./user", Targets: map[string]string{"linux": "~/.config"}},
			{Name: "root-path", Sudo: true, Backup: "./root", Targets: map[string]string{"linux": "/etc"}},
		},
	}

	// All paths are returned regardless of Root flag
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	paths := mgr.GetPaths()

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
	if err := os.WriteFile(regularFile, []byte("test"), 0644); err != nil {
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
	if err := os.WriteFile(existingFile, []byte("test"), 0644); err != nil {
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
	if err := os.WriteFile(srcFile, content, 0644); err != nil {
		t.Fatal(err)
	}

	dstFile := filepath.Join(tmpDir, "subdir", "dest.txt")

	if err := copyFile(srcFile, dstFile); err != nil {
		t.Fatalf("copyFile() error = %v", err)
	}

	// Check content
	got, err := os.ReadFile(dstFile)
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
	os.MkdirAll(filepath.Join(srcDir, "subdir"), 0755)
	os.WriteFile(filepath.Join(srcDir, "file1.txt"), []byte("file1"), 0644)
	os.WriteFile(filepath.Join(srcDir, "subdir", "file2.txt"), []byte("file2"), 0644)

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
	content, _ := os.ReadFile(filepath.Join(dstDir, "file1.txt"))
	if string(content) != "file1" {
		t.Errorf("file1.txt content = %q, want %q", string(content), "file1")
	}
}

func TestRemoveAll(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Test removing regular file
	regularFile := filepath.Join(tmpDir, "regular.txt")
	os.WriteFile(regularFile, []byte("test"), 0644)

	if err := removeAll(regularFile); err != nil {
		t.Fatalf("removeAll(regular file) error = %v", err)
	}

	if pathExists(regularFile) {
		t.Error("Regular file still exists after removeAll")
	}

	// Test removing directory
	dir := filepath.Join(tmpDir, "testdir")
	os.MkdirAll(dir, 0755)
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("test"), 0644)

	if err := removeAll(dir); err != nil {
		t.Fatalf("removeAll(dir) error = %v", err)
	}

	if pathExists(dir) {
		t.Error("Directory still exists after removeAll")
	}

	// Test that symlinks are not removed
	target := filepath.Join(tmpDir, "target.txt")
	os.WriteFile(target, []byte("target"), 0644)
	symlink := filepath.Join(tmpDir, "symlink.txt")
	os.Symlink(target, symlink)

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
	cfg := &config.Config{
		BackupRoot: "/home/user/backups",
	}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	tests := []struct {
		name string
		path string
		want string
	}{
		{"relative path", "./configs", "/home/user/backups/configs"},
		{"absolute path", "/etc/config", "/etc/config"},
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
	if err := os.WriteFile(srcFile, content, 0755); err != nil {
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
	os.MkdirAll(dir, 0755)

	// Create a symlink to the directory
	symlinkDir := filepath.Join(tmpDir, "symlinkdir")
	os.Symlink(dir, symlinkDir)

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
	os.WriteFile(realFile, []byte("content"), 0644)

	symlink := filepath.Join(tmpDir, "link.txt")
	os.Symlink(realFile, symlink)

	if !pathExists(symlink) {
		t.Error("pathExists() should return true for symlink")
	}

	// Test with broken symlink
	brokenLink := filepath.Join(tmpDir, "broken.txt")
	os.Symlink(filepath.Join(tmpDir, "nonexistent"), brokenLink)

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
				Name:    "test-app",
				Filters: []config.Filter{{Include: map[string]string{"os": "linux"}}},
				Entries: []config.SubEntry{
					{Type: "config", Name: "config1", Backup: "./config1", Targets: map[string]string{"linux": "~/.config"}},
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
		Version: 2,
		Entries: []config.Entry{
			{
				Name:   "with-package",
				Backup: "./backup",
				Package: &config.EntryPackage{
					Managers: map[string]string{"pacman": "neovim"},
				},
			},
			{
				Name:   "without-package",
				Backup: "./backup2",
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	pkgEntries := mgr.GetPackageEntries()

	// Should only return entries with packages
	if len(pkgEntries) != 1 {
		t.Fatalf("GetPackageEntries() returned %d, want 1", len(pkgEntries))
	}

	if pkgEntries[0].Name != "with-package" {
		t.Errorf("Entry name = %q, want %q", pkgEntries[0].Name, "with-package")
	}
}

func TestLogWarn(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	// Just call it to get coverage - hard to test output
	mgr.logWarn("test warning message")
	mgr.logWarn("test warning with %s", "arg")
}
