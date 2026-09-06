package manager

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/fsys"
)

func TestRestoreCopyTemplatePreflightRejectsSymlinkInExistingTargetChain(t *testing.T) {
	skipIfNoSymlink(t)
	backup, target, mgr, _ := setupTemplateTest(t)
	source := filepath.Join(backup, "link", "existing", "root.tmpl")
	writeTemplateFile(t, source, "value=1")

	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "existing"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(target, "link")); err != nil {
		t.Fatal(err)
	}

	entry := config.SubEntry{Name: "copy", Method: config.MethodCopy,
		Backup: backup, Files: []string{"link/existing/root.tmpl"}}
	beforeBackup := snapshotTemplateFilesystem(t, backup)
	beforeTarget := snapshotTemplateFilesystem(t, target)

	if err := mgr.RestoreFiles(entry, backup, target); err == nil {
		t.Fatal("restore followed a symlink in the existing target chain")
	}
	if after := snapshotTemplateFilesystem(t, backup); !reflect.DeepEqual(after, beforeBackup) {
		t.Fatalf("backup changed during rejected preflight: before=%v after=%v", beforeBackup, after)
	}
	if after := snapshotTemplateFilesystem(t, target); !reflect.DeepEqual(after, beforeTarget) {
		t.Fatalf("target changed during rejected preflight: before=%v after=%v", beforeTarget, after)
	}
	if _, err := os.Lstat(filepath.Join(outside, "existing", "root")); !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("redirected target was created: %v", err)
	}
}

func TestRestoreCopyTemplatePreflightRejectsMissingLiteralSourceBeforeMutation(t *testing.T) {
	backup, target, mgr, store := setupTemplateTest(t)
	writeTemplateFile(t, filepath.Join(backup, "root.tmpl"), "value=1")
	writeTemplateFile(t, filepath.Join(target, "existing.conf"), "keep")
	entry := config.SubEntry{Name: "copy", Method: config.MethodCopy,
		Backup: backup, Files: []string{"root.tmpl", "missing.conf"}}

	beforeBackup := snapshotTemplateFilesystem(t, backup)
	beforeTarget := snapshotTemplateFilesystem(t, target)
	beforeHistory, err := store.GetRenderHistory(mgr.ctx, "root.tmpl", 10)
	if err != nil {
		t.Fatalf("history before: %v", err)
	}

	if err := mgr.RestoreFiles(entry, backup, target); err == nil {
		t.Fatal("mixed restore accepted a missing literal source")
	}
	if after := snapshotTemplateFilesystem(t, backup); !reflect.DeepEqual(after, beforeBackup) {
		t.Fatalf("backup changed during rejected preflight: before=%v after=%v", beforeBackup, after)
	}
	if after := snapshotTemplateFilesystem(t, target); !reflect.DeepEqual(after, beforeTarget) {
		t.Fatalf("target changed during rejected preflight: before=%v after=%v", beforeTarget, after)
	}
	afterHistory, err := store.GetRenderHistory(mgr.ctx, "root.tmpl", 10)
	if err != nil {
		t.Fatalf("history after: %v", err)
	}
	if !reflect.DeepEqual(afterHistory, beforeHistory) {
		t.Fatalf("history changed during rejected preflight: before=%v after=%v", beforeHistory, afterHistory)
	}
}

func TestRestoreCopyTemplatePreflightRejectsOrdinaryTargetAliasingTemplateSource(t *testing.T) {
	orders := [][]string{
		{"root.tmpl", "ordinary.conf"},
		{"ordinary.conf", "root.tmpl"},
	}

	for _, files := range orders {
		t.Run(filepath.Join(files...), func(t *testing.T) {
			backup, target, mgr, _ := setupTemplateTest(t)
			templateSource := filepath.Join(backup, "root.tmpl")
			ordinarySource := filepath.Join(backup, "ordinary.conf")
			writeTemplateFile(t, templateSource, "template")
			writeTemplateFile(t, ordinarySource, "literal")
			if err := os.Link(templateSource, filepath.Join(target, "ordinary.conf")); err != nil {
				t.Skipf("hardlinks unavailable: %v", err)
			}

			entry := config.SubEntry{Name: "copy", Method: config.MethodCopy,
				Backup: backup, Files: files}
			beforeBackup := snapshotTemplateFilesystem(t, backup)
			beforeTarget := snapshotTemplateFilesystem(t, target)

			if err := mgr.RestoreFiles(entry, backup, target); err == nil {
				t.Fatal("restore accepted an ordinary target hardlink to a template source")
			}
			if after := snapshotTemplateFilesystem(t, backup); !reflect.DeepEqual(after, beforeBackup) {
				t.Fatalf("backup changed during rejected preflight: before=%v after=%v", beforeBackup, after)
			}
			if after := snapshotTemplateFilesystem(t, target); !reflect.DeepEqual(after, beforeTarget) {
				t.Fatalf("target changed during rejected preflight: before=%v after=%v", beforeTarget, after)
			}
			if got := readTemplateTestFile(t, templateSource); got != "template" {
				t.Fatalf("template source was corrupted to %q", got)
			}
		})
	}
}

func TestPreflightCopyTemplateAllowsDeniedAbsentOrphanBackupWithSudo(t *testing.T) {
	skipIfNoSudo(t)
	backup, target, mgr, _ := setupTemplateTest(t)
	writeTemplateFile(t, filepath.Join(backup, "root.tmpl"), "value=1")
	orphan := filepath.Join(target, "root.tidydots.bak")
	runner := cmdexec.NewStubRunner()
	runner.AddResult("stat", cmdexec.Result{ExitCode: 1})
	runner.AddResult("stat", cmdexec.Result{Stdout: []byte("41ed 0 42 100\n")})
	runner.AddResult("find", cmdexec.Result{})
	mgr = mgr.
		WithFS(deniedCopyLstatFS{FS: fsys.OsFS{}, denied: orphan}).
		WithRunner(runner)
	entry := config.SubEntry{Name: "copy", Method: config.MethodCopy, Sudo: true,
		Backup: backup, Files: []string{"root.tmpl"}}

	if err := mgr.preflightCopyTemplateFiles(entry, backup, target); err != nil {
		t.Fatalf("preflight rejected a positively absent sudo orphan backup: %v", err)
	}
	if len(runner.Calls) != 3 || runner.Calls[0].Name != "stat" ||
		runner.Calls[1].Name != "stat" || runner.Calls[2].Name != "find" {
		t.Fatalf("inspection calls = %+v, want artifact stat, parent stat, find", runner.Calls)
	}
}

func TestPreflightCopyTemplateChecksDeniedIntermediateAncestor(t *testing.T) {
	skipIfNoSymlink(t)
	backup, target, mgr, _ := setupTemplateTest(t)
	writeTemplateFile(t, filepath.Join(backup, "link", "existing", "root.tmpl"), "value=1")

	outside := t.TempDir()
	if err := os.MkdirAll(filepath.Join(outside, "existing"), 0o750); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(target, "link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	runner := cmdexec.NewStubRunner()
	runner.AddResult("stat", cmdexec.Result{Stdout: []byte("a1ff 0 42 100\n")})
	mgr = mgr.
		WithFS(deniedCopyLstatFS{FS: fsys.OsFS{}, denied: link}).
		WithRunner(runner)
	entry := config.SubEntry{Name: "copy", Method: config.MethodCopy, Sudo: true,
		Backup: backup, Files: []string{"link/existing/root.tmpl"}}

	if err := mgr.preflightCopyTemplateFiles(entry, backup, target); err == nil {
		t.Fatal("preflight followed a denied symlink ancestor")
	}
	if len(runner.Calls) != 1 || runner.Calls[0].Name != "stat" {
		t.Fatalf("inspection calls = %+v, want one elevated nofollow stat", runner.Calls)
	}
}

func TestPreflightCopyTemplateVerifiesEveryDeniedExistingAncestor(t *testing.T) {
	skipIfNoSudo(t)
	backup, target, mgr, _ := setupTemplateTest(t)
	writeTemplateFile(t, filepath.Join(backup, "one", "two", "existing", "root.tmpl"), "value=1")
	one := filepath.Join(target, "one")
	two := filepath.Join(one, "two")
	if err := os.MkdirAll(filepath.Join(two, "existing"), 0o750); err != nil {
		t.Fatal(err)
	}
	runner := cmdexec.NewStubRunner()
	runner.AddResult("stat", cmdexec.Result{Stdout: []byte("41ed 0 42 100\n")})
	runner.AddResult("stat", cmdexec.Result{Stdout: []byte("41ed 0 42 101\n")})
	mgr = mgr.
		WithFS(deniedCopyLstatPathsFS{FS: fsys.OsFS{}, denied: map[string]bool{one: true, two: true}}).
		WithRunner(runner)
	entry := config.SubEntry{Name: "copy", Method: config.MethodCopy, Sudo: true,
		Backup: backup, Files: []string{"one/two/existing/root.tmpl"}}

	if err := mgr.preflightCopyTemplateFiles(entry, backup, target); err != nil {
		t.Fatalf("preflight rejected verified ancestors: %v", err)
	}
	if len(runner.Calls) != 2 || runner.Calls[0].Name != "stat" || runner.Calls[1].Name != "stat" {
		t.Fatalf("inspection calls = %+v, want one stat per denied component", runner.Calls)
	}
}

func TestRestoreCopyTemplateDryRunPreservesCompleteStateAcrossVariants(t *testing.T) {
	tests := []struct {
		name    string
		seed    bool
		missing bool
		force   bool
		denied  bool
	}{
		{name: "no-history"},
		{name: "missing-target", missing: true},
		{name: "conflict", seed: true},
		{name: "force", seed: true, force: true},
		{name: "denied-target", denied: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backup, target, mgr, store := setupTemplateTest(t)
			source := filepath.Join(backup, "root.tmpl")
			destination := filepath.Join(target, "root")
			writeTemplateFile(t, source, "a=1")
			entry := config.SubEntry{Name: "copy", Method: config.MethodCopy,
				Backup: backup, Files: []string{"root.tmpl"}}

			if tt.seed {
				if err := mgr.RestoreFiles(entry, backup, target); err != nil {
					t.Fatalf("seed restore: %v", err)
				}
				writeTemplateFile(t, destination, "a=2")
				writeTemplateFile(t, source, "a=3")
			} else if !tt.missing {
				writeTemplateFile(t, destination, "a=0")
			}

			if tt.denied {
				writeTemplateFile(t, destination, "a=0")
			}

			beforeBackup := snapshotTemplateFilesystem(t, backup)
			beforeTarget := snapshotTemplateFilesystem(t, target)
			beforeHistory, err := store.GetRenderHistory(mgr.ctx, "root.tmpl", 10)
			if err != nil {
				t.Fatalf("history before: %v", err)
			}

			runner := cmdexec.NewStubRunner()
			if tt.denied {
				mgr = mgr.WithFS(deniedCopyReadFS{FS: fsys.OsFS{}, denied: destination})
				entry.Sudo = true
				for i := 0; i < 2; i++ {
					runner.AddResult("stat", cmdexec.Result{Stdout: []byte("8180 0 42 99\n")})
					runner.AddResult("cat", cmdexec.Result{Stdout: []byte("a=0")})
				}
			}
			mgr = mgr.WithRunner(runner)
			mgr.DryRun = true
			mgr.ForceRender = tt.force

			if err := mgr.RestoreFiles(entry, backup, target); err != nil {
				t.Fatalf("dry-run restore: %v", err)
			}
			if after := snapshotTemplateFilesystem(t, backup); !reflect.DeepEqual(after, beforeBackup) {
				t.Fatalf("backup changed during dry-run: before=%v after=%v", beforeBackup, after)
			}
			if after := snapshotTemplateFilesystem(t, target); !reflect.DeepEqual(after, beforeTarget) {
				t.Fatalf("target changed during dry-run: before=%v after=%v", beforeTarget, after)
			}
			afterHistory, err := store.GetRenderHistory(mgr.ctx, "root.tmpl", 10)
			if err != nil {
				t.Fatalf("history after: %v", err)
			}
			if !reflect.DeepEqual(afterHistory, beforeHistory) {
				t.Fatalf("history changed during dry-run: before=%v after=%v", beforeHistory, afterHistory)
			}
			for _, call := range runner.Calls {
				if isTemplateCopyMutationCommand(call.Name) {
					t.Fatalf("dry-run invoked mutating command: %+v", call)
				}
			}
		})
	}
}

func isTemplateCopyMutationCommand(name string) bool {
	switch name {
	case "chmod", "chown", "cp", "dd", "ln", "mkdir", "mktemp", "mv", "rm":
		return true
	default:
		return false
	}
}

type deniedCopyLstatPathsFS struct {
	fsys.FS
	denied map[string]bool
}

func (f deniedCopyLstatPathsFS) Lstat(name string) (fs.FileInfo, error) {
	if f.denied[name] {
		return nil, &fs.PathError{Op: "lstat", Path: name, Err: fs.ErrPermission}
	}
	return f.FS.Lstat(name)
}

func TestRestoreCopyTemplateRetainsRestrictiveTargetMode(t *testing.T) {
	backup, target, mgr, _ := setupTemplateTest(t)
	source := filepath.Join(backup, "root.tmpl")
	destination := filepath.Join(target, "root")
	writeTemplateFile(t, source, "a=1")
	if err := os.Chmod(source, 0o640); err != nil {
		t.Fatal(err)
	}
	entry := config.SubEntry{Name: "copy", Method: config.MethodCopy,
		Backup: backup, Files: []string{"root.tmpl"}}
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("initial restore: %v", err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatalf("stat initial target: %v", err)
	}
	if info.Mode().Perm() != 0o640 {
		t.Fatalf("initial mode = %o, want 0640", info.Mode().Perm())
	}

	if err := os.Chmod(destination, 0o600); err != nil {
		t.Fatal(err)
	}
	writeTemplateFile(t, source, "a=2")
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("updated restore: %v", err)
	}
	info, err = os.Stat(destination)
	if err != nil {
		t.Fatalf("stat updated target: %v", err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("updated mode = %o, want retained 0600", info.Mode().Perm())
	}
}

func TestRestoreCopyTemplateRepairsSudoOwnerOnHashNoOp(t *testing.T) {
	skipIfNoSudo(t)
	backup, target, mgr, _ := setupTemplateTest(t)
	source := filepath.Join(backup, "root.tmpl")
	destination := filepath.Join(target, "root")
	writeTemplateFile(t, source, "a=1")
	entry := config.SubEntry{Name: "copy", Method: config.MethodCopy,
		Backup: backup, Files: []string{"root.tmpl"}}
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("initial restore: %v", err)
	}
	info, err := os.Stat(destination)
	if err != nil {
		t.Fatal(err)
	}
	if fileRootOwned(info) {
		t.Skip("target is already root-owned")
	}

	runner := cmdexec.NewStubRunner()
	mgr = mgr.WithRunner(runner)
	entry.Sudo = true
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("hash no-op restore: %v", err)
	}
	if len(runner.Calls) != 1 || runner.Calls[0].Name != "chown" || !runner.Calls[0].Sudo {
		t.Fatalf("owner repair calls = %+v, want one sudo chown", runner.Calls)
	}
	if got := runner.Calls[0].Args; !reflect.DeepEqual(got, []string{"0:0", "--", destination}) {
		t.Fatalf("owner repair args = %v, want [0:0 -- %s]", got, destination)
	}
}
