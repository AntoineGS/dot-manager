package manager

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/fsys"
	"github.com/AntoineGS/tidydots/internal/state"
)

func TestCopyOnlyOrdinaryEntrySkipsIrrelevantStateInspection(t *testing.T) {
	sourceRoot := t.TempDir()
	targetRoot := t.TempDir()
	stateParent := filepath.Join(t.TempDir(), "irrelevant-state")
	source := filepath.Join(sourceRoot, "ordinary.conf")
	writeTemplateFile(t, source, "ordinary source")

	filesystem := &stateReadProbeFS{FS: fsys.OsFS{}, root: stateParent}
	runner := cmdexec.NewStubRunner()
	mgr := newUtilityManager()
	mgr.Config.BackupRoot = stateParent
	mgr = mgr.WithFS(filesystem).WithRunner(runner)
	entry := config.SubEntry{
		Name:   "ordinary-copy",
		Method: config.MethodCopy,
		Backup: sourceRoot,
		Files:  []string{"ordinary.conf"},
	}

	if !filepath.IsAbs(source) {
		t.Fatalf("source fixture is not absolute: %q", source)
	}
	if err := mgr.RestoreFiles(entry, sourceRoot, targetRoot); err != nil {
		t.Fatalf("ordinary restore touched irrelevant state: %v", err)
	}
	if got := readTemplateTestFile(t, filepath.Join(targetRoot, "ordinary.conf")); got != "ordinary source" {
		t.Fatalf("ordinary target = %q, want ordinary source", got)
	}
	if filesystem.reads != 0 {
		t.Fatalf("ordinary restore read irrelevant state %d time(s)", filesystem.reads)
	}
	if len(runner.Calls) != 0 {
		t.Fatalf("ordinary restore invoked subprocesses: %+v", runner.Calls)
	}

	inspection, err := mgr.InspectCopyTemplates(entry, sourceRoot, targetRoot)
	if err != nil {
		t.Fatalf("ordinary inspection touched irrelevant state: %v", err)
	}
	if inspection.NeedsRestore || inspection.Outdated || len(inspection.Modified) != 0 {
		t.Fatalf("ordinary inspection = %+v, want empty inspection", inspection)
	}
	if filesystem.reads != 0 {
		t.Fatalf("ordinary inspection read irrelevant state %d time(s)", filesystem.reads)
	}
	if len(runner.Calls) != 0 {
		t.Fatalf("ordinary inspection invoked subprocesses: %+v", runner.Calls)
	}
}

func TestCopyTemplateRejectsHardlinkedDistinctStateSidecars(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		name := "native-identity"
		if fallback {
			name = "identity-unavailable-fallback"
		}
		t.Run(name, func(t *testing.T) {
			backup := t.TempDir()
			stateDir := t.TempDir()
			stateDatabase := filepath.Join(stateDir, ".tidydots.db")
			wal := stateDatabase + "-wal"
			journal := stateDatabase + "-journal"
			writeTemplateFile(t, filepath.Join(backup, "root.tmpl"), "value=1")
			writeTemplateFile(t, wal, "sidecar")
			if err := os.Link(wal, journal); err != nil {
				t.Skipf("hardlinks unavailable: %v", err)
			}

			mgr := newUtilityManager()
			mgr.Config.BackupRoot = stateDir
			if fallback {
				mgr = mgr.WithFS(unknownIdentityLstatFS{FS: fsys.OsFS{}})
			}
			entry := config.SubEntry{
				Name:   "distinct-sidecars",
				Method: config.MethodCopy,
				Backup: backup,
				Files:  []string{"root.tmpl"},
			}

			t.Run("restore", func(t *testing.T) {
				runner := cmdexec.NewStubRunner()
				if err := mgr.WithRunner(runner).RestoreFiles(entry, backup, t.TempDir()); err == nil {
					t.Fatal("restore accepted hard-linked distinct state sidecars")
				}
				if len(runner.Calls) != 0 {
					t.Fatalf("restore invoked subprocesses: %+v", runner.Calls)
				}
			})

			t.Run("status", func(t *testing.T) {
				runner := cmdexec.NewStubRunner()
				if _, err := mgr.WithRunner(runner).InspectCopyTemplates(entry, backup, t.TempDir()); err == nil {
					t.Fatal("status accepted hard-linked distinct state sidecars")
				}
				if len(runner.Calls) != 0 {
					t.Fatalf("status invoked subprocesses: %+v", runner.Calls)
				}
			})
		})
	}
}

func TestCopyTemplateRejectsHardlinkedDistinctActiveAndDefaultDatabases(t *testing.T) {
	for _, fallback := range []bool{false, true} {
		name := "native-identity"
		if fallback {
			name = "identity-unavailable-fallback"
		}
		t.Run(name, func(t *testing.T) {
			backup := t.TempDir()
			activeDir := t.TempDir()
			defaultDir := t.TempDir()
			activeDatabase := filepath.Join(activeDir, ".tidydots.db")
			defaultDatabase := filepath.Join(defaultDir, ".tidydots.db")
			store, err := state.Open(context.Background(), activeDatabase)
			if err != nil {
				t.Fatalf("open active state store: %v", err)
			}
			t.Cleanup(func() { _ = store.Close() }) //nolint:errcheck // test cleanup
			if err := os.Link(activeDatabase, defaultDatabase); err != nil {
				t.Fatalf("hardlink active/default databases: %v", err)
			}
			writeTemplateFile(t, filepath.Join(backup, "root.tmpl"), "value=1")

			mgr := New(&config.Config{Version: 3, BackupRoot: defaultDir}, setupTemplatePlatform())
			mgr.stateStore = store
			if fallback {
				mgr = mgr.WithFS(unknownIdentityLstatFS{FS: fsys.OsFS{}})
			}
			entry := config.SubEntry{
				Name:   "distinct-databases",
				Method: config.MethodCopy,
				Backup: backup,
				Files:  []string{"root.tmpl"},
			}

			t.Run("restore", func(t *testing.T) {
				runner := cmdexec.NewStubRunner()
				if err := mgr.WithRunner(runner).RestoreFiles(entry, backup, t.TempDir()); err == nil {
					t.Fatal("restore accepted hard-linked active/default databases")
				}
				if len(runner.Calls) != 0 {
					t.Fatalf("restore invoked subprocesses: %+v", runner.Calls)
				}
			})

			t.Run("status", func(t *testing.T) {
				runner := cmdexec.NewStubRunner()
				if _, err := mgr.WithRunner(runner).InspectCopyTemplates(entry, backup, t.TempDir()); err == nil {
					t.Fatal("status accepted hard-linked active/default databases")
				}
				if len(runner.Calls) != 0 {
					t.Fatalf("status invoked subprocesses: %+v", runner.Calls)
				}
			})
		})
	}
}

type stateReadProbeFS struct {
	fsys.FS
	root  string
	reads int
}

func (f *stateReadProbeFS) isStatePath(name string) bool {
	root, err := filepath.Abs(f.root)
	if err != nil {
		return false
	}
	candidate, err := filepath.Abs(name)
	if err != nil {
		return false
	}
	relative, err := filepath.Rel(root, candidate)
	if err != nil {
		return false
	}
	return relative == "." || (relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator)))
}

func (f *stateReadProbeFS) denyRead(op, name string) (fs.FileInfo, error) {
	f.reads++
	return nil, &fs.PathError{Op: op, Path: name, Err: fs.ErrPermission}
}

func (f *stateReadProbeFS) Lstat(name string) (fs.FileInfo, error) {
	if f.isStatePath(name) {
		return f.denyRead("lstat", name)
	}
	return f.FS.Lstat(name)
}

func (f *stateReadProbeFS) Stat(name string) (fs.FileInfo, error) {
	if f.isStatePath(name) {
		return f.denyRead("stat", name)
	}
	return f.FS.Stat(name)
}

func (f *stateReadProbeFS) ReadFile(name string) ([]byte, error) {
	if f.isStatePath(name) {
		f.reads++
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrPermission}
	}
	return f.FS.ReadFile(name)
}

func (f *stateReadProbeFS) Readlink(name string) (string, error) {
	if f.isStatePath(name) {
		f.reads++
		return "", &fs.PathError{Op: "readlink", Path: name, Err: fs.ErrPermission}
	}
	return f.FS.Readlink(name)
}

func (f *stateReadProbeFS) ReadDir(name string) ([]os.DirEntry, error) {
	if f.isStatePath(name) {
		f.reads++
		return nil, &fs.PathError{Op: "readdir", Path: name, Err: fs.ErrPermission}
	}
	return f.FS.ReadDir(name)
}

func (f *stateReadProbeFS) WalkDir(name string, fn fs.WalkDirFunc) error {
	if f.isStatePath(name) {
		f.reads++
		return &fs.PathError{Op: "walk", Path: name, Err: fs.ErrPermission}
	}
	return f.FS.WalkDir(name, fn)
}
