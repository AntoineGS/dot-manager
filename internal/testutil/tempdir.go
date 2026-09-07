package testutil

import (
	"path/filepath"
	"testing"
)

// CanonicalTempDir creates a cleaned-up temporary directory without symlink
// ancestors. Use it for fixtures whose production contract rejects symlinked
// parents: macOS's default temporary directory is beneath /var -> /private/var.
// Resolve only the fixture root, before adding any intentional test symlinks.
func CanonicalTempDir(t testing.TB) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatalf("canonicalize temporary directory: %v", err)
	}
	return dir
}
