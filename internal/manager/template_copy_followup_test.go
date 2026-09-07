package manager

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/fsys"
	"github.com/AntoineGS/tidydots/internal/state"
	"github.com/AntoineGS/tidydots/internal/testutil"
)

func TestRestoreCopyTemplateRejectsAbsentStateSidecarThroughSymlinkedParent(t *testing.T) {
	skipIfNoSymlink(t)

	home := testutil.CanonicalTempDir(t)
	repo := filepath.Join(home, "repo")
	stateReal := filepath.Join(home, "state-real")
	stateAlias := filepath.Join(home, "state-alias")
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(stateReal, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(stateReal, stateAlias); err != nil {
		t.Fatal(err)
	}

	dbPath := filepath.Join(stateAlias, ".tidydots.db")
	store, err := state.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() }) //nolint:errcheck // test cleanup

	mgr := New(&config.Config{Version: 3, BackupRoot: filepath.Join(home, "config-root")}, setupTemplatePlatform())
	mgr.stateStore = store
	source := filepath.Join(repo, ".tidydots.db-journal.tmpl")
	target := filepath.Join(stateReal, ".tidydots.db-journal")
	if err := os.Remove(target); err != nil && !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("remove pre-existing journal sidecar: %v", err)
	}
	if _, err := os.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("journal sidecar is not absent before the regression: %v", err)
	}
	writeTemplateFile(t, source, "value=1")
	entry := config.SubEntry{
		Name:   "state-sidecar",
		Method: config.MethodCopy,
		Backup: repo,
		Files:  []string{".tidydots.db-journal.tmpl"},
	}

	beforeSource := readTemplateTestFile(t, source)
	beforeHistory, err := store.GetRenderHistory(mgr.ctx, copyStateTestKey("./../repo/.tidydots.db-journal.tmpl", target), 10)
	if err != nil {
		t.Fatalf("history before: %v", err)
	}
	statePaths := []string{
		filepath.Join(stateReal, ".tidydots.db"),
		filepath.Join(stateReal, ".tidydots.db-wal"),
		filepath.Join(stateReal, ".tidydots.db-shm"),
		filepath.Join(stateReal, ".tidydots.db-journal"),
	}
	beforeState := make(map[string]finalReviewBytes, len(statePaths))
	for _, path := range statePaths {
		beforeState[path] = finalReviewPathBytes(t, path)
	}
	if err := mgr.RestoreFiles(entry, repo, stateReal); err == nil {
		t.Fatal("restore accepted an absent state sidecar through a symlinked parent")
	}
	if got := readTemplateTestFile(t, source); got != beforeSource {
		t.Fatalf("source changed during rejected preflight: got %q, want %q", got, beforeSource)
	}
	if _, err := os.Lstat(target); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("state sidecar was created or changed: %v", err)
	}
	for path, before := range beforeState {
		if after := finalReviewPathBytes(t, path); !reflect.DeepEqual(after, before) {
			t.Fatalf("state artifact %s changed during rejected preflight: before=%v after=%v", path, before, after)
		}
	}
	afterHistory, err := store.GetRenderHistory(mgr.ctx, copyStateTestKey("./../repo/.tidydots.db-journal.tmpl", target), 10)
	if err != nil {
		t.Fatalf("history after: %v", err)
	}
	if !reflect.DeepEqual(afterHistory, beforeHistory) {
		t.Fatalf("history changed during rejected preflight: before=%v after=%v", beforeHistory, afterHistory)
	}
}

func TestCopyTemplateStateAliasesRepresentOneProtectedArtifact(t *testing.T) {
	skipIfNoSymlink(t)

	home := testutil.CanonicalTempDir(t)
	repo := filepath.Join(home, "repo")
	stateReal := filepath.Join(home, "state-real")
	stateAlias := filepath.Join(home, "state-alias")
	target := filepath.Join(home, "target")
	for _, dir := range []string{repo, stateReal, target} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink(stateReal, stateAlias); err != nil {
		t.Fatal(err)
	}

	store, err := state.Open(context.Background(), filepath.Join(stateAlias, ".tidydots.db"))
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() }) //nolint:errcheck // test cleanup
	mgr := New(&config.Config{Version: 3, BackupRoot: filepath.Join(home, "config-root")}, setupTemplatePlatform())
	mgr.stateStore = store
	writeTemplateFile(t, filepath.Join(repo, "root.tmpl"), "value=1")
	entry := config.SubEntry{Name: "aliased-state", Method: config.MethodCopy,
		Backup: repo, Files: []string{"root.tmpl"}}

	if err := mgr.RestoreFiles(entry, repo, target); err != nil {
		t.Fatalf("restore with lexical and canonical state aliases: %v", err)
	}
	inspection, err := mgr.InspectCopyTemplates(entry, repo, target)
	if err != nil {
		t.Fatalf("status with lexical and canonical state aliases: %v", err)
	}
	if inspection.NeedsRestore || inspection.Outdated || len(inspection.Modified) != 0 {
		t.Fatalf("inspection = %+v, want healthy copy template", inspection)
	}
}

func TestRestoreCopyTemplatePropagatesStateParentCanonicalizationFailure(t *testing.T) {
	home := testutil.CanonicalTempDir(t)
	repo := filepath.Join(home, "repo")
	stateDir := filepath.Join(home, "state")
	target := filepath.Join(home, "target")
	for _, dir := range []string{repo, stateDir, target} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}

	dbPath := filepath.Join(stateDir, ".tidydots.db")
	store, err := state.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("open state store: %v", err)
	}
	t.Cleanup(func() { _ = store.Close() }) //nolint:errcheck // test cleanup

	mgr := New(&config.Config{Version: 3, BackupRoot: repo}, setupTemplatePlatform())
	mgr.stateStore = store
	source := filepath.Join(repo, "root.tmpl")
	writeTemplateFile(t, source, "value=1")
	writeTemplateFile(t, filepath.Join(target, "keep"), "unchanged")
	entry := config.SubEntry{Name: "canonicalization-error", Method: config.MethodCopy,
		Backup: repo, Files: []string{"root.tmpl"}}

	beforeRepo := snapshotTemplateFilesystem(t, repo)
	beforeTarget := snapshotTemplateFilesystem(t, target)
	beforeHistory, err := store.GetRenderHistory(mgr.ctx, copyStateTestKey("./root.tmpl", filepath.Join(target, "root")), 10)
	if err != nil {
		t.Fatalf("history before: %v", err)
	}

	filesystem := failingStateCanonicalizationFS{FS: fsys.OsFS{}, denied: stateDir}
	if err := mgr.WithFS(filesystem).RestoreFiles(entry, repo, target); err == nil {
		t.Fatal("restore ignored state-parent canonicalization failure")
	} else if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("restore error = %v, want permission error", err)
	}
	if after := snapshotTemplateFilesystem(t, repo); !reflect.DeepEqual(after, beforeRepo) {
		t.Fatalf("repository changed during rejected preflight: before=%v after=%v", beforeRepo, after)
	}
	if after := snapshotTemplateFilesystem(t, target); !reflect.DeepEqual(after, beforeTarget) {
		t.Fatalf("target changed during rejected preflight: before=%v after=%v", beforeTarget, after)
	}
	afterHistory, err := store.GetRenderHistory(mgr.ctx, copyStateTestKey("./root.tmpl", filepath.Join(target, "root")), 10)
	if err != nil {
		t.Fatalf("history after: %v", err)
	}
	if !reflect.DeepEqual(afterHistory, beforeHistory) {
		t.Fatalf("history changed during rejected preflight: before=%v after=%v", beforeHistory, afterHistory)
	}
}

func TestCopyTemplateStatusAndRestoreRejectHardlinkWhenNativeIdentityUnavailable(t *testing.T) {
	backup, target, mgr, _ := setupTemplateTest(t)
	source := filepath.Join(backup, "root.tmpl")
	destination := filepath.Join(target, "root")
	entry := config.SubEntry{Name: "hardlink", Method: config.MethodCopy,
		Backup: backup, Files: []string{"root.tmpl"}}
	writeTemplateFile(t, source, "value=1")
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("seed restore: %v", err)
	}
	if err := os.Remove(destination); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(source, destination); err != nil {
		t.Skipf("hardlinks unavailable: %v", err)
	}

	runner := cmdexec.NewStubRunner()
	unknownIdentity := mgr.
		WithFS(unknownIdentityLstatFS{FS: fsys.OsFS{}}).
		WithRunner(runner)
	if _, err := unknownIdentity.InspectCopyTemplates(entry, backup, target); err == nil {
		t.Fatal("status accepted a target hardlink to its template source without identity metadata")
	}
	if len(runner.Calls) != 0 {
		t.Fatalf("status invoked privileged commands: %+v", runner.Calls)
	}
	if err := unknownIdentity.RestoreFiles(entry, backup, target); err == nil {
		t.Fatal("restore accepted a target hardlink to its template source without identity metadata")
	}
	if len(runner.Calls) != 0 {
		t.Fatalf("restore invoked commands before rejecting native hardlink: %+v", runner.Calls)
	}
}

func TestCopyTemplateRecoveryBackupSymlinkIsUnavailableAndRejected(t *testing.T) {
	skipIfNoSymlink(t)

	t.Run("status", func(t *testing.T) {
		backup, target, mgr, _ := setupTemplateTest(t)
		entry := config.SubEntry{Name: "recovery-status", Method: config.MethodCopy,
			Backup: backup, Files: []string{"root.tmpl"}}
		writeTemplateFile(t, filepath.Join(backup, "root.tmpl"), "value=1")
		if err := mgr.RestoreFiles(entry, backup, target); err != nil {
			t.Fatalf("seed restore: %v", err)
		}

		recovery := templateCopyOrphanBackupPath(filepath.Join(target, "root"))
		outside := filepath.Join(t.TempDir(), "outside")
		writeTemplateFile(t, outside, "not a recovery backup")
		if err := os.Symlink(outside, recovery); err != nil {
			t.Fatal(err)
		}

		runner := cmdexec.NewStubRunner()
		if _, err := mgr.WithRunner(runner).InspectCopyTemplates(entry, backup, target); err == nil {
			t.Fatal("status accepted a symlinked recovery backup")
		}
		if len(runner.Calls) != 0 {
			t.Fatalf("status invoked privileged commands: %+v", runner.Calls)
		}
	})

	t.Run("restore", func(t *testing.T) {
		backup, target, mgr, store := setupTemplateTest(t)
		entry := config.SubEntry{Name: "recovery-restore", Method: config.MethodCopy,
			Backup: backup, Files: []string{"root.tmpl"}}
		writeTemplateFile(t, filepath.Join(backup, "root.tmpl"), "value=1")
		if err := mgr.RestoreFiles(entry, backup, target); err != nil {
			t.Fatalf("seed restore: %v", err)
		}

		recovery := templateCopyOrphanBackupPath(filepath.Join(target, "root"))
		outside := filepath.Join(t.TempDir(), "outside")
		writeTemplateFile(t, outside, "not a recovery backup")
		if err := os.Symlink(outside, recovery); err != nil {
			t.Fatal(err)
		}
		beforeTarget := snapshotTemplateFilesystem(t, target)
		beforeHistory, err := store.GetRenderHistory(mgr.ctx, copyStateTestKey("./root.tmpl", filepath.Join(target, "root")), 10)
		if err != nil {
			t.Fatalf("history before: %v", err)
		}

		if err := mgr.RestoreFiles(entry, backup, target); err == nil {
			t.Fatal("restore accepted a symlinked recovery backup")
		}
		if after := snapshotTemplateFilesystem(t, target); !reflect.DeepEqual(after, beforeTarget) {
			t.Fatalf("target changed during rejected restore: before=%v after=%v", beforeTarget, after)
		}
		afterHistory, err := store.GetRenderHistory(mgr.ctx, copyStateTestKey("./root.tmpl", filepath.Join(target, "root")), 10)
		if err != nil {
			t.Fatalf("history after: %v", err)
		}
		if !reflect.DeepEqual(afterHistory, beforeHistory) {
			t.Fatalf("history changed during rejected restore: before=%v after=%v", beforeHistory, afterHistory)
		}
	})
}

func TestInspectCopyTemplatesAllowsLegacyTemplateTargetSymlink(t *testing.T) {
	skipIfNoSymlink(t)
	backup, target, mgr, _ := setupTemplateTest(t)
	entry := config.SubEntry{Name: "legacy-target", Method: config.MethodCopy,
		Backup: backup, Files: []string{"root.tmpl"}}
	writeTemplateFile(t, filepath.Join(backup, "root.tmpl"), "value=1")
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("seed restore: %v", err)
	}

	legacy := filepath.Join(target, "root.tmpl")
	if err := os.Symlink(filepath.Join(backup, "root.tmpl"), legacy); err != nil {
		t.Fatal(err)
	}

	got, err := mgr.InspectCopyTemplates(entry, backup, target)
	if err != nil {
		t.Fatalf("legacy target symlink was rejected: %v", err)
	}
	if got.NeedsRestore || got.Outdated || len(got.Modified) != 0 {
		t.Fatalf("inspection = %+v, want healthy copy target with permitted legacy symlink", got)
	}
}

type failingStateCanonicalizationFS struct {
	fsys.FS
	denied string
}

func (f failingStateCanonicalizationFS) Lstat(name string) (fs.FileInfo, error) {
	if filepath.Clean(name) == filepath.Clean(f.denied) {
		return nil, &fs.PathError{Op: "lstat", Path: name, Err: fs.ErrPermission}
	}
	return f.FS.Lstat(name)
}

type unknownIdentityLstatFS struct {
	fsys.FS
}

func (f unknownIdentityLstatFS) Lstat(name string) (fs.FileInfo, error) {
	info, err := f.FS.Lstat(name)
	if err != nil {
		return nil, err
	}
	return unknownIdentityFileInfo{FileInfo: info}, nil
}

type unknownIdentityFileInfo struct {
	fs.FileInfo
}

func (unknownIdentityFileInfo) Sys() any { return nil }
