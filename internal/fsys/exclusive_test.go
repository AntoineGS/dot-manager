package fsys_test

import (
	"errors"
	"io/fs"
	"path/filepath"
	"testing"

	"github.com/AntoineGS/tidydots/internal/fsys"
)

func TestWriteFileExclusiveDoesNotReplaceOccupants(t *testing.T) {
	tests := []struct {
		name   string
		newFS  func(t *testing.T) (fsys.FS, string)
		occupy func(t *testing.T, filesystem fsys.FS, path string)
		assert func(t *testing.T, filesystem fsys.FS, path string)
	}{
		{
			name: "os regular file",
			newFS: func(t *testing.T) (fsys.FS, string) {
				t.Helper()
				dir := t.TempDir()
				return fsys.OsFS{}, filepath.Join(dir, "occupied")
			},
			occupy: func(t *testing.T, filesystem fsys.FS, path string) {
				t.Helper()
				if err := filesystem.WriteFile(path, []byte("keep"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			assert: func(t *testing.T, filesystem fsys.FS, path string) {
				t.Helper()
				got, err := filesystem.ReadFile(path)
				if err != nil || string(got) != "keep" {
					t.Fatalf("exclusive write altered occupant: %q %v", got, err)
				}
			},
		},
		{
			name: "os symlink",
			newFS: func(t *testing.T) (fsys.FS, string) {
				t.Helper()
				dir := t.TempDir()
				return fsys.OsFS{}, filepath.Join(dir, "occupied")
			},
			occupy: func(t *testing.T, filesystem fsys.FS, path string) {
				t.Helper()
				referent := filepath.Join(filepath.Dir(path), "referent")
				if err := filesystem.WriteFile(referent, []byte("keep"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := filesystem.Symlink(referent, path); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			},
			assert: func(t *testing.T, filesystem fsys.FS, path string) {
				t.Helper()
				link, err := filesystem.Readlink(path)
				if err != nil {
					t.Fatalf("symlink occupant disappeared: %v", err)
				}
				got, err := filesystem.ReadFile(path)
				if err != nil || string(got) != "keep" {
					t.Fatalf("symlink referent changed: link=%q content=%q err=%v", link, got, err)
				}
			},
		},
		{
			name: "mem regular file",
			newFS: func(t *testing.T) (fsys.FS, string) {
				t.Helper()
				filesystem := fsys.NewMemFS()
				if err := filesystem.MkdirAll("/base", 0o755); err != nil {
					t.Fatal(err)
				}
				return filesystem, "/base/occupied"
			},
			occupy: func(t *testing.T, filesystem fsys.FS, path string) {
				t.Helper()
				if err := filesystem.WriteFile(path, []byte("keep"), 0o600); err != nil {
					t.Fatal(err)
				}
			},
			assert: func(t *testing.T, filesystem fsys.FS, path string) {
				t.Helper()
				got, err := filesystem.ReadFile(path)
				if err != nil || string(got) != "keep" {
					t.Fatalf("exclusive write altered occupant: %q %v", got, err)
				}
			},
		},
		{
			name: "mem directory",
			newFS: func(t *testing.T) (fsys.FS, string) {
				t.Helper()
				filesystem := fsys.NewMemFS()
				if err := filesystem.MkdirAll("/base", 0o755); err != nil {
					t.Fatal(err)
				}
				return filesystem, "/base/occupied"
			},
			occupy: func(t *testing.T, filesystem fsys.FS, path string) {
				t.Helper()
				if err := filesystem.MkdirAll(path, 0o700); err != nil {
					t.Fatal(err)
				}
			},
			assert: func(t *testing.T, filesystem fsys.FS, path string) {
				t.Helper()
				info, err := filesystem.Lstat(path)
				if err != nil {
					t.Fatalf("directory occupant disappeared: %v", err)
				}
				if !info.IsDir() {
					t.Fatal("directory occupant was replaced")
				}
			},
		},
		{
			name: "mem symlink",
			newFS: func(t *testing.T) (fsys.FS, string) {
				t.Helper()
				filesystem := fsys.NewMemFS()
				if err := filesystem.MkdirAll("/base", 0o755); err != nil {
					t.Fatal(err)
				}
				return filesystem, "/base/occupied"
			},
			occupy: func(t *testing.T, filesystem fsys.FS, path string) {
				t.Helper()
				referent := filepath.Join(filepath.Dir(path), "referent")
				if err := filesystem.WriteFile(referent, []byte("keep"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := filesystem.Symlink(referent, path); err != nil {
					t.Fatal(err)
				}
			},
			assert: func(t *testing.T, filesystem fsys.FS, path string) {
				t.Helper()
				link, err := filesystem.Readlink(path)
				if err != nil {
					t.Fatalf("symlink occupant disappeared: %v", err)
				}
				got, err := filesystem.ReadFile(path)
				if err != nil || string(got) != "keep" {
					t.Fatalf("symlink referent changed: link=%q content=%q err=%v", link, got, err)
				}
			},
		},
		{
			name: "mem dangling symlink",
			newFS: func(t *testing.T) (fsys.FS, string) {
				t.Helper()
				filesystem := fsys.NewMemFS()
				if err := filesystem.MkdirAll("/base", 0o755); err != nil {
					t.Fatal(err)
				}
				return filesystem, "/base/occupied"
			},
			occupy: func(t *testing.T, filesystem fsys.FS, path string) {
				t.Helper()
				if err := filesystem.Symlink("/base/missing", path); err != nil {
					t.Fatal(err)
				}
			},
			assert: func(t *testing.T, filesystem fsys.FS, path string) {
				t.Helper()
				if _, err := filesystem.Readlink(path); err != nil {
					t.Fatalf("dangling symlink occupant disappeared: %v", err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filesystem, path := tt.newFS(t)
			tt.occupy(t, filesystem, path)

			err := filesystem.WriteFileExclusive(path, []byte("replace"), 0o600)
			if !errors.Is(err, fs.ErrExist) {
				t.Fatalf("expected ErrExist, got %v", err)
			}
			tt.assert(t, filesystem, path)
		})
	}
}

func TestMemFS_ChmodPreservesExplicitZeroMode(t *testing.T) {
	filesystem := fsys.NewMemFS()
	if err := filesystem.MkdirAll("/base", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := filesystem.WriteFile("/base/file", []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := filesystem.Chmod("/base/file", 0); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	info, err := filesystem.Lstat("/base/file")
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0 {
		t.Errorf("mode permissions = %o, want 0", got)
	}
}

func TestOsFS_ChmodPreservesExplicitZeroMode(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file")
	filesystem := fsys.OsFS{}
	if err := filesystem.WriteFile(path, []byte("data"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := filesystem.Chmod(path, 0); err != nil {
		t.Fatalf("Chmod: %v", err)
	}
	info, err := filesystem.Lstat(path)
	if err != nil {
		t.Fatalf("Lstat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0 {
		t.Errorf("mode permissions = %o, want 0", got)
	}
}
