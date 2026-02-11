//go:build linux

package manager

import (
	"os"
	"path/filepath"
	"testing"
)

func TestSymlink_Linux_FileSymlink(t *testing.T) {
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

	// Verify it's a symlink
	fi, err := os.Lstat(target)
	if err != nil {
		t.Fatalf("Lstat() error: %v", err)
	}

	if fi.Mode()&os.ModeSymlink == 0 {
		t.Error("target is not a symlink")
	}

	// Verify readlink resolves correctly
	got, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("Readlink() error: %v", err)
	}

	if got != source {
		t.Errorf("Readlink() = %q, want %q", got, source)
	}
}

func TestSymlink_Linux_DirectorySymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	sourceDir := filepath.Join(dir, "sourcedir")
	if err := os.MkdirAll(sourceDir, 0o755); err != nil {
		t.Fatalf("failed to create source dir: %v", err)
	}

	// Create a file inside source dir
	if err := os.WriteFile(filepath.Join(sourceDir, "file.txt"), []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create file: %v", err)
	}

	linkDir := filepath.Join(dir, "linkdir")
	if err := os.Symlink(sourceDir, linkDir); err != nil {
		t.Fatalf("Symlink() error: %v", err)
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

func TestSymlink_Linux_RelativeSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	source := filepath.Join(dir, "source.txt")
	if err := os.WriteFile(source, []byte("content"), 0o644); err != nil {
		t.Fatalf("failed to create source: %v", err)
	}

	// Create symlink with relative target
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

	// Content should still be readable
	data, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("ReadFile through relative symlink error: %v", err)
	}

	if string(data) != "content" {
		t.Errorf("content = %q, want %q", string(data), "content")
	}
}

func TestSymlink_Linux_OverwriteSymlink(t *testing.T) {
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

	// Create symlink to source1
	if err := os.Symlink(source1, link); err != nil {
		t.Fatalf("first Symlink() error: %v", err)
	}

	// Remove and recreate to source2 (Linux requires remove first)
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

func TestSymlink_Linux_NestedDirectories(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	// Create nested source structure
	nested := filepath.Join(dir, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("MkdirAll() error: %v", err)
	}

	deepFile := filepath.Join(nested, "deep.txt")
	if err := os.WriteFile(deepFile, []byte("deep"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// Symlink to the nested directory
	link := filepath.Join(dir, "link")
	if err := os.Symlink(nested, link); err != nil {
		t.Fatalf("Symlink() error: %v", err)
	}

	data, err := os.ReadFile(filepath.Join(link, "deep.txt"))
	if err != nil {
		t.Fatalf("ReadFile through nested symlink error: %v", err)
	}

	if string(data) != "deep" {
		t.Errorf("content = %q, want %q", string(data), "deep")
	}
}

func TestSymlink_Linux_NoPrivilegeRequired(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	source := filepath.Join(dir, "source")
	if err := os.WriteFile(source, []byte("x"), 0o644); err != nil {
		t.Fatalf("WriteFile() error: %v", err)
	}

	// On Linux, symlinks never require special privileges
	err := os.Symlink(source, filepath.Join(dir, "link"))
	if err != nil {
		t.Errorf("Symlink() should not require privileges on Linux, got error: %v", err)
	}
}
