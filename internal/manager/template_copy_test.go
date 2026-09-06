package manager

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/fsys"
	"github.com/AntoineGS/tidydots/internal/state"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

func TestRestoreCopyTemplatePreservesTargetEdits(t *testing.T) {
	backup, target, mgr, _ := setupTemplateTest(t)
	src := filepath.Join(backup, "root.tmpl")
	dst := filepath.Join(target, "root")
	writeTemplateFile(t, src, "host={{ .Hostname }}\na=1\nb=1")
	entry := config.SubEntry{Name: "snapper", Method: config.MethodCopy,
		Backup: backup, Files: []string{"root.tmpl"}}
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatal(err)
	}
	info, err := os.Lstat(dst)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("not a real file: %v", err)
	}
	writeTemplateFile(t, dst, "host=testhost\na=2\nb=1")
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatal(err)
	}
	if got := readTemplateTestFile(t, dst); got != "host=testhost\na=2\nb=1" {
		t.Fatalf("unchanged template lost edits: %q", got)
	}
	writeTemplateFile(t, src, "host={{ .Hostname }}\na=1\nb=2")
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatal(err)
	}
	if got := readTemplateTestFile(t, dst); got != "host=testhost\na=2\nb=2" {
		t.Fatalf("merge result: %q", got)
	}
	if _, err := os.Lstat(filepath.Join(backup, "root")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("copy mode unexpectedly created a repository alias: %v", err)
	}
}

func TestRestoreCopyTemplateUsesHostnameContext(t *testing.T) {
	tests := []struct {
		name     string
		hostname string
		want     string
	}{
		{name: "omarchbook", hostname: "omarchbook", want: "no/1"},
		{name: "desktop", hostname: "desktop", want: "yes/7"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backup, target, mgr, _ := setupTemplateTest(t)
			mgr.Platform.Hostname = tt.hostname
			mgr.templateEngine = tmpl.NewEngine(tmpl.NewContextFromPlatform(mgr.Platform))
			src := filepath.Join(backup, "root.tmpl")
			writeTemplateFile(t, src, `{{ if eq .Hostname "omarchbook" }}no/1{{ else }}yes/7{{ end }}`)
			entry := config.SubEntry{Name: "snapper", Method: config.MethodCopy,
				Backup: backup, Files: []string{"root.tmpl"}}

			if err := mgr.RestoreFiles(entry, backup, target); err != nil {
				t.Fatalf("restore: %v", err)
			}
			if got := readTemplateTestFile(t, filepath.Join(target, "root")); got != tt.want {
				t.Fatalf("rendered content = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRestoreCopyTemplateConflictDeploysPureRenderAndSavesRecovery(t *testing.T) {
	backup, target, mgr, store := setupTemplateTest(t)
	src := filepath.Join(backup, "root.tmpl")
	dst := filepath.Join(target, "root")
	entry := config.SubEntry{Name: "snapper", Method: config.MethodCopy,
		Backup: backup, Files: []string{"root.tmpl"}}

	writeTemplateFile(t, src, "a=1")
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("initial restore: %v", err)
	}
	writeTemplateFile(t, dst, "a=2")
	writeTemplateFile(t, src, "a=3")

	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("conflicting restore: %v", err)
	}
	if got := readTemplateTestFile(t, dst); got != "a=3" {
		t.Fatalf("target = %q, want pure render", got)
	}
	conflict := readTemplateTestFile(t, tmpl.ConflictPath(src))
	if !strings.Contains(conflict, "a=2") || !strings.Contains(conflict, "a=3") {
		t.Fatalf("conflict = %q, want both variants", conflict)
	}
	record, err := store.GetLatestRender(mgr.ctx, "root.tmpl", "linux", "testhost")
	if err != nil || record == nil {
		t.Fatalf("render record = (%v, %v), want record", record, err)
	}
	if string(record.PureRender) != "a=3" {
		t.Fatalf("pure render = %q, want a=3", record.PureRender)
	}
}

func TestRestoreCopyTemplateConflictWriterFailurePreservesTargetAndHistory(t *testing.T) {
	backup, target, mgr, store := setupTemplateTest(t)
	src := filepath.Join(backup, "root.tmpl")
	dst := filepath.Join(target, "root")
	entry := config.SubEntry{Name: "snapper", Method: config.MethodCopy,
		Backup: backup, Files: []string{"root.tmpl"}}

	writeTemplateFile(t, src, "a=1")
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("initial restore: %v", err)
	}
	writeTemplateFile(t, dst, "a=2")
	writeTemplateFile(t, src, "a=3")
	before, err := store.GetRenderHistory(mgr.ctx, "root.tmpl", 10)
	if err != nil {
		t.Fatalf("history before: %v", err)
	}

	mgr = mgr.WithFS(failingCopyRenameFS{FS: fsys.OsFS{}})
	if err := mgr.RestoreFiles(entry, backup, target); err == nil {
		t.Fatal("conflict writer failure was ignored")
	}
	if got := readTemplateTestFile(t, dst); got != "a=2" {
		t.Fatalf("target = %q, want unchanged a=2", got)
	}
	after, err := store.GetRenderHistory(mgr.ctx, "root.tmpl", 10)
	if err != nil {
		t.Fatalf("history after: %v", err)
	}
	if len(after) != len(before) {
		t.Fatalf("history count = %d, want %d", len(after), len(before))
	}
}

func TestRestoreCopyTemplateOrphanBackupIsExclusive(t *testing.T) {
	backup, target, mgr, _ := setupTemplateTest(t)
	src := filepath.Join(backup, "root.tmpl")
	dst := filepath.Join(target, "root")
	entry := config.SubEntry{Name: "snapper", Method: config.MethodCopy,
		Backup: backup, Files: []string{"root.tmpl"}}

	writeTemplateFile(t, src, "new")
	writeTemplateFile(t, dst, "old")
	backupPath := templateCopyOrphanBackupPath(dst)
	writeTemplateFile(t, backupPath, "occupied")

	if err := mgr.RestoreFiles(entry, backup, target); err == nil {
		t.Fatal("occupied orphan backup was overwritten")
	}
	if got := readTemplateTestFile(t, dst); got != "old" {
		t.Fatalf("target = %q, want old", got)
	}
	if got := readTemplateTestFile(t, backupPath); got != "occupied" {
		t.Fatalf("orphan backup = %q, want occupied", got)
	}
}

func TestRestoreCopyTemplateTargetRenameFailurePreservesTarget(t *testing.T) {
	backup, target, mgr, _ := setupTemplateTest(t)
	src := filepath.Join(backup, "root.tmpl")
	dst := filepath.Join(target, "root")
	entry := config.SubEntry{Name: "snapper", Method: config.MethodCopy,
		Backup: backup, Files: []string{"root.tmpl"}}

	writeTemplateFile(t, src, "new")
	writeTemplateFile(t, dst, "old")
	mgr = mgr.WithFS(failingCopyRenameFS{FS: fsys.OsFS{}})
	if err := mgr.RestoreFiles(entry, backup, target); err == nil {
		t.Fatal("target rename failure was ignored")
	}
	if got := readTemplateTestFile(t, dst); got != "old" {
		t.Fatalf("target = %q, want old", got)
	}
}

func TestRestoreCopyTemplateMissingTargetUsesHistoryBase(t *testing.T) {
	backup, target, mgr, _ := setupTemplateTest(t)
	src := filepath.Join(backup, "root.tmpl")
	dst := filepath.Join(target, "root")
	entry := config.SubEntry{Name: "snapper", Method: config.MethodCopy,
		Backup: backup, Files: []string{"root.tmpl"}}

	writeTemplateFile(t, src, "a=1")
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("initial restore: %v", err)
	}
	if err := os.Remove(dst); err != nil {
		t.Fatalf("remove target: %v", err)
	}
	writeTemplateFile(t, src, "a=2")
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("restore missing target: %v", err)
	}
	if got := readTemplateTestFile(t, dst); got != "a=2" {
		t.Fatalf("target = %q, want a=2", got)
	}
}

func TestRestoreCopyTemplateNestedSuffixAndUnselectedInvalidSource(t *testing.T) {
	backup, target, mgr, _ := setupTemplateTest(t)
	selected := filepath.Join(backup, "nested", "config.tmpl.tmpl")
	unselected := filepath.Join(backup, "nested", "broken.tmpl")
	writeTemplateFile(t, selected, "value=ok")
	writeTemplateFile(t, unselected, "{{ broken }}")
	entry := config.SubEntry{Name: "snapper", Method: config.MethodCopy,
		Backup: backup, Files: []string{"nested/config.tmpl.tmpl"}}

	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("nested restore: %v", err)
	}
	if got := readTemplateTestFile(t, filepath.Join(target, "nested", "config.tmpl")); got != "value=ok" {
		t.Fatalf("nested target = %q, want value=ok", got)
	}
	if _, err := os.Lstat(filepath.Join(target, "nested", "config.tmpl.tmpl")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("source suffix was not stripped once: %v", err)
	}
	if _, err := os.Lstat(tmpl.RenderedPath(selected)); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("copy mode created rendered cache: %v", err)
	}
	if _, err := os.Lstat(filepath.Join(backup, "nested", "config.tmpl")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("copy mode created repository alias: %v", err)
	}
}

func TestRestoreCopyTemplateHashNoOpDoesNotWrite(t *testing.T) {
	backup, target, mgr, _ := setupTemplateTest(t)
	src := filepath.Join(backup, "root.tmpl")
	dst := filepath.Join(target, "root")
	entry := config.SubEntry{Name: "snapper", Method: config.MethodCopy,
		Backup: backup, Files: []string{"root.tmpl"}}
	writeTemplateFile(t, src, "a=1")
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("initial restore: %v", err)
	}
	writeTemplateFile(t, dst, "user=edit")

	spy := &countingCopyFS{FS: fsys.OsFS{}}
	mgr = mgr.WithFS(spy)
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("hash-matching restore: %v", err)
	}
	if spy.writes != 0 || spy.exclusiveWrites != 0 || spy.renames != 0 {
		t.Fatalf("hash-matching restore wrote filesystem: %+v", spy)
	}
	if got := readTemplateTestFile(t, dst); got != "user=edit" {
		t.Fatalf("target = %q, want retained user edit", got)
	}
}

func TestRestoreCopyTemplateDryRunDoesNotMutateOrSaveHistory(t *testing.T) {
	backup, target, mgr, store := setupTemplateTest(t)
	src := filepath.Join(backup, "root.tmpl")
	dst := filepath.Join(target, "root")
	writeTemplateFile(t, src, "a=1")
	writeTemplateFile(t, dst, "a=0")
	mgr.DryRun = true
	entry := config.SubEntry{Name: "snapper", Method: config.MethodCopy,
		Backup: backup, Files: []string{"root.tmpl"}}

	beforeBackup := snapshotTemplateFilesystem(t, backup)
	beforeTarget := snapshotTemplateFilesystem(t, target)
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("copy dry-run: %v", err)
	}
	if got := snapshotTemplateFilesystem(t, backup); len(got) != len(beforeBackup) {
		t.Fatalf("backup changed during dry-run: before=%v after=%v", beforeBackup, got)
	}
	if got := snapshotTemplateFilesystem(t, target); len(got) != len(beforeTarget) {
		t.Fatalf("target changed during dry-run: before=%v after=%v", beforeTarget, got)
	}
	record, err := store.GetLatestRender(mgr.ctx, "root.tmpl", "linux", "testhost")
	if err != nil || record != nil {
		t.Fatalf("dry-run history = (%v, %v), want no record", record, err)
	}
}

func TestRestoreCopyTemplateHistoryFailureReportsPartialSuccess(t *testing.T) {
	backup, target, mgr, store := setupTemplateTest(t)
	src := filepath.Join(backup, "root.tmpl")
	dst := filepath.Join(target, "root")
	entry := config.SubEntry{Name: "snapper", Method: config.MethodCopy,
		Backup: backup, Files: []string{"root.tmpl"}}
	writeTemplateFile(t, src, "a=1")
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("initial restore: %v", err)
	}
	writeTemplateFile(t, src, "a=2")
	mgr = mgr.WithFS(&closeStoreOnTargetRenameFS{FS: fsys.OsFS{}, target: dst, store: store})

	err := mgr.RestoreFiles(entry, backup, target)
	if err == nil || !strings.Contains(err.Error(), "partial success") {
		t.Fatalf("history failure = %v, want partial-success error", err)
	}
	if got := readTemplateTestFile(t, dst); got != "a=2" {
		t.Fatalf("target = %q, want deployed a=2", got)
	}
}

func TestRestoreCopyTemplateAllowsExpectedSymlinkMigration(t *testing.T) {
	skipIfNoSymlink(t)
	backup, target, mgr, _ := setupTemplateTest(t)
	src := filepath.Join(backup, "root.tmpl")
	rendered := tmpl.RenderedPath(src)
	alias := filepath.Join(backup, "root")
	dst := filepath.Join(target, "root")
	writeTemplateFile(t, src, "value=1")
	if err := mgr.renderTemplateAndLink(src, "root.tmpl"); err != nil {
		t.Fatalf("seed symlink-mode render: %v", err)
	}
	if err := os.Symlink(alias, dst); err != nil {
		t.Fatal(err)
	}
	entry := config.SubEntry{Name: "snapper", Method: config.MethodCopy,
		Backup: backup, Files: []string{"root.tmpl"}}

	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("copy migration: %v", err)
	}
	info, err := os.Lstat(dst)
	if err != nil || !info.Mode().IsRegular() {
		t.Fatalf("migrated target = %v, %v; want regular file", info, err)
	}
	if got := readTemplateTestFile(t, dst); got != "value=1" {
		t.Fatalf("migrated content = %q, want value=1", got)
	}
	if !testIsSymlink(alias) || !testPathExists(rendered) {
		t.Fatal("migration removed the source alias/rendered chain")
	}
}

func TestRestoreCopyTemplateRejectsUnsafeTargetLinksAndAliases(t *testing.T) {
	skipIfNoSymlink(t)
	tests := []struct {
		name  string
		setup func(t *testing.T, src, rendered, target string)
	}{
		{
			name: "source template symlink",
			setup: func(t *testing.T, src, _, target string) {
				if err := os.Symlink(src, target); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "hardlink to source",
			setup: func(t *testing.T, src, _, target string) {
				if err := os.Link(src, target); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "hardlink to rendered artifact",
			setup: func(t *testing.T, _, rendered, target string) {
				if err := os.Link(rendered, target); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backup, targetDir, mgr, _ := setupTemplateTest(t)
			src := filepath.Join(backup, "root.tmpl")
			rendered := tmpl.RenderedPath(src)
			dst := filepath.Join(targetDir, "root")
			writeTemplateFile(t, src, "value=1")
			if tt.name == "hardlink to rendered artifact" {
				if err := os.WriteFile(rendered, []byte("rendered"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			tt.setup(t, src, rendered, dst)
			entry := config.SubEntry{Name: "snapper", Method: config.MethodCopy,
				Backup: backup, Files: []string{"root.tmpl"}}

			if err := mgr.RestoreFiles(entry, backup, targetDir); err == nil {
				t.Fatal("unsafe target alias was accepted")
			}
		})
	}
}

func TestRestoreCopyTemplateRejectsSymlinkParentAndSpecialTarget(t *testing.T) {
	skipIfNoSymlink(t)
	t.Run("symlink parent", func(t *testing.T) {
		backup, target, mgr, _ := setupTemplateTest(t)
		src := filepath.Join(backup, "nested", "config.tmpl")
		writeTemplateFile(t, src, "value=1")
		outside := t.TempDir()
		if err := os.Symlink(outside, filepath.Join(target, "nested")); err != nil {
			t.Fatal(err)
		}
		entry := config.SubEntry{Name: "copy", Method: config.MethodCopy,
			Backup: backup, Files: []string{"nested/config.tmpl"}}
		if err := mgr.RestoreFiles(entry, backup, target); err == nil {
			t.Fatal("symlink target parent was accepted")
		}
	})

	t.Run("special target", func(t *testing.T) {
		backup, target, mgr, _ := setupTemplateTest(t)
		src := filepath.Join(backup, "root.tmpl")
		dst := filepath.Join(target, "root")
		writeTemplateFile(t, src, "value=1")
		createTemplateCopySpecialFile(t, dst)
		entry := config.SubEntry{Name: "copy", Method: config.MethodCopy,
			Backup: backup, Files: []string{"root.tmpl"}}
		if err := mgr.RestoreFiles(entry, backup, target); err == nil {
			t.Fatal("special target was accepted")
		}
	})
}

type countingCopyFS struct {
	fsys.FS
	writes          int
	exclusiveWrites int
	renames         int
}

func (f *countingCopyFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	f.writes++
	return f.FS.WriteFile(name, data, perm)
}

func (f *countingCopyFS) WriteFileExclusive(name string, data []byte, perm fs.FileMode) error {
	f.exclusiveWrites++
	return f.FS.WriteFileExclusive(name, data, perm)
}

func (f *countingCopyFS) Rename(oldpath, newpath string) error {
	f.renames++
	return f.FS.Rename(oldpath, newpath)
}

type closeStoreOnTargetRenameFS struct {
	fsys.FS
	target string
	store  *state.Store
	closed bool
}

func (f *closeStoreOnTargetRenameFS) Rename(oldpath, newpath string) error {
	err := f.FS.Rename(oldpath, newpath)
	if err == nil && newpath == f.target && !f.closed {
		f.closed = true
		_ = f.store.Close() //nolint:errcheck // the test deliberately closes the store
	}
	return err
}
