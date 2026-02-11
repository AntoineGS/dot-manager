//go:build darwin

package manager

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSymlink_Darwin_FileSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	source := filepath.Join(dir, "source.txt")
	target := filepath.Join(dir, "link.txt")

	if err := os.WriteFile(source, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	if err := os.Symlink(source, target); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

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

func TestSymlink_Darwin_DirectorySymlink(t *testing.T) {
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
	if err := os.Symlink(sourceDir, linkDir); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(linkDir, "file.txt"))
	if err != nil {
		t.Fatalf("ReadFile through symlink error: %v", err)
	}

	if string(data) != "content" {
		t.Errorf("content through symlink = %q, want %q", string(data), "content")
	}
}

func TestSymlink_Darwin_CaseInsensitiveFS(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// macOS default filesystem (APFS) is case-insensitive
	source := filepath.Join(dir, "Source.txt")
	if err := os.WriteFile(source, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	// Try to access with different casing
	lowerPath := filepath.Join(dir, "source.txt")
	_, err := os.Stat(lowerPath)

	// On case-insensitive FS, this should succeed
	if err != nil {
		t.Logf("filesystem appears to be case-sensitive: %v", err)
		return
	}

	// Create symlink with original case
	link := filepath.Join(dir, "link.txt")
	if err := os.Symlink(source, link); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	data, err := os.ReadFile(link)
	if err != nil {
		t.Fatalf("ReadFile through symlink error: %v", err)
	}

	if string(data) != "content" {
		t.Errorf("content = %q, want %q", string(data), "content")
	}
}

func TestSymlink_Darwin_NoPrivilegeRequired(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	source := filepath.Join(dir, "source")
	if err := os.WriteFile(source, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// On macOS (POSIX), symlinks never require special privileges
	err := os.Symlink(source, filepath.Join(dir, "link"))
	if err != nil {
		t.Errorf("Symlink() should not require privileges on macOS, got error: %v", err)
	}
}

func TestSymlink_Darwin_RelativeSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	source := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(source, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	target := filepath.Join(dir, "link.txt")
	if err := os.Symlink("source.txt", target); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	got, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("Readlink() error: %v", err)
	}

	if got != "source.txt" {
		t.Errorf("Readlink() = %q, want %q", got, "source.txt")
	}
}
