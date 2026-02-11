//go:build windows

package manager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSymlink_Windows_FileSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	source := filepath.Join(dir, "source.txt")
	target := filepath.Join(dir, "link.txt")

	if err := os.WriteFile(source, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	err := os.Symlink(source, target)
	if err != nil {
		t.Skipf("symlink creation failed (dev mode may not be enabled): %v", err)
	}

	// Verify it's a symlink
	fi, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("Lstat() error: %v", err)
	}

	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("target is not a symlink")
	}

	got, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("Readlink() error: %v", err)
	}

	if got != source {
		t.Errorf("Readlink() = %q, want %q", got, source)
	}
}

func TestSymlink_Windows_DirectorySymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	sourceDir := filepath.Join(dir, "sourcedir")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(sourceDir, "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	linkDir := filepath.Join(dir, "linkdir")
	err := os.Symlink(sourceDir, linkDir)
	if err != nil {
		t.Skipf("directory symlink creation failed: %v", err)
	}

	// File should be accessible through symlink
	data, err := os.ReadFile(filepath.Join(linkDir, "file.txt"))
	if err != nil {
		t.Fatalf("ReadFile through symlink error: %v", err)
	}

	if string(data) != "content" {
		t.Errorf("content through symlink = %q, want %q", string(data), "content")
	}
}

func TestSymlink_Windows_ReadlinkBackslashes(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	source := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(source, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	link := filepath.Join(dir, "link.txt")
	err := os.Symlink(source, link)
	if err != nil {
		t.Skipf("symlink creation failed: %v", err)
	}

	got, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("Readlink() error: %v", err)
	}

	// On Windows, readlink should return a path with backslashes
	if strings.Contains(got, "/") {
		t.Errorf("Readlink() = %q, expected backslash-only path on Windows", got)
	}
}

func TestSymlink_Windows_OverwriteSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	source1 := filepath.Join(dir, "source1.txt")
	source2 := filepath.Join(dir, "source2.txt")
	link := filepath.Join(dir, "link.txt")

	if err := os.WriteFile(source1, []byte("first"), 0o644); err != nil {
		t.Fatalf("failed to create source1: %v", err)
	}

	if err := os.WriteFile(source2, []byte("second"), 0o644); err != nil {
		t.Fatalf("failed to create source2: %v", err)
	}

	err := os.Symlink(source1, link)
	if err != nil {
		t.Skipf("symlink creation failed: %v", err)
	}

	// Remove and recreate
	if err := os.Remove(link); err != nil {
		t.Fatalf("Remove() error: %v", err)
	}

	if err := os.Symlink(source2, link); err != nil {
		t.Fatalf("second Symlink() error: %v", err)
	}

	got, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("Readlink() error: %v", err)
	}

	if got != source2 {
		t.Errorf("Readlink() = %q, want %q", got, source2)
	}
}

func TestSymlink_Windows_GracefulFailure(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	source := filepath.Join(dir, "nonexistent.txt")
	link := filepath.Join(dir, "link.txt")

	// Symlink to nonexistent source should still create the symlink (dangling)
	// or fail gracefully depending on Windows version
	err := os.Symlink(source, link)
	if err != nil {
		// On Windows without dev mode, this is expected
		t.Logf("Symlink to nonexistent source: %v (may be expected)", err)
		return
	}

	// If it succeeded, readlink should still work
	got, err := os.Readlink(link)
	if err != nil {
		t.Fatalf("Readlink() error: %v", err)
	}

	if got != source {
		t.Errorf("Readlink() = %q, want %q", got, source)
	}
}
