package manager

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
)

func TestCopyTemplateFixtureWithSymlinkedTempRoot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("TMPDIR is a Unix temp-directory setting")
	}
	root := t.TempDir()
	alias := filepath.Join(root, "temp-alias")
	if err := os.Symlink(root, alias); err != nil {
		t.Fatal(err)
	}
	t.Setenv("TMPDIR", alias)

	// Model macOS /var -> /private/var without changing production policy.
	t.Run("restore", func(t *testing.T) {
		backup, target, mgr, _ := setupTemplateTest(t)
		writeTemplateFile(t, filepath.Join(backup, "root.tmpl"), "value=1")
		entry := config.SubEntry{Name: "copy", Method: config.MethodCopy,
			Backup: backup, Files: []string{"root.tmpl"}}
		if err := mgr.RestoreFiles(entry, backup, target); err != nil {
			t.Fatalf("restore with canonical fixture paths: %v", err)
		}
		// Fixture canonicalization must not hide intentional symlinks or relax
		// the production prohibition on symlinked target ancestors.
		link := filepath.Join(target, "link")
		if err := os.Symlink(target, link); err != nil {
			t.Fatal(err)
		}
		err := mgr.RestoreFiles(entry, backup, filepath.Join(link, "nested"))
		if err == nil || !strings.Contains(err.Error(), "target parent is a symlink") {
			t.Fatalf("symlinked target ancestor must still be rejected: %v", err)
		}
	})
}
