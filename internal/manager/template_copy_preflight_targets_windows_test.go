//go:build windows

package manager

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/fsys"
)

func TestRestoreCopyTemplatePreflightRejectsNestedJunctionLikeParent(t *testing.T) {
	backup, target, mgr, _ := setupTemplateTest(t)
	relativeTemplate := filepath.Join("nested", "junction", "root.tmpl")
	writeTemplateFile(t, filepath.Join(backup, relativeTemplate), "value=1")

	junction := filepath.Join(target, "nested", "junction")
	if err := os.MkdirAll(junction, 0o750); err != nil {
		t.Fatal(err)
	}

	mgr = mgr.WithFS(junctionLikeReadlinkFS{
		FS:       fsys.OsFS{},
		junction: junction,
	})
	entry := config.SubEntry{
		Name:   "copy",
		Method: config.MethodCopy,
		Backup: backup,
		Files:  []string{relativeTemplate},
	}

	if err := mgr.RestoreFiles(entry, backup, target); err == nil {
		t.Fatal("preflight accepted a nested junction-like target parent")
	}
	if _, err := os.Lstat(filepath.Join(junction, "root")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("restore created a target through the junction-like parent: %v", err)
	}
}

type junctionLikeReadlinkFS struct {
	fsys.FS
	junction string
}

func (f junctionLikeReadlinkFS) Readlink(name string) (string, error) {
	if filepath.Clean(name) == filepath.Clean(f.junction) {
		return filepath.Join(filepath.Dir(name), "outside"), nil
	}
	return f.FS.Readlink(name)
}
