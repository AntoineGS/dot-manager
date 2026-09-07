package manager

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
	"github.com/AntoineGS/tidydots/internal/fsys"
)

func TestWriteTemplateCopyFile_SudoStagingFailureDoesNotMoveTarget(t *testing.T) {
	skipIfNoSudo(t)
	mgr, _, runner := newSudoManager(t)
	runner.AddResult("mktemp", cmdexec.Result{Stdout: []byte("/etc/.tidydots-copy-test\n")})
	runner.AddResult("dd", cmdexec.Result{ExitCode: 1})

	err := mgr.writeTemplateCopyFile("/etc/root", []byte("private"), 0o600, true, false)
	if err == nil {
		t.Fatal("failed staging must fail deployment")
	}
	for _, call := range runner.Calls {
		if call.Name == "mv" {
			t.Fatal("must not replace target after staging failure")
		}
		for _, arg := range call.Args {
			if strings.Contains(arg, "private") {
				t.Fatal("payload leaked into argv")
			}
		}
	}
	if len(runner.Calls) < 3 || runner.Calls[2].Name != "rm" {
		t.Fatalf("calls = %+v, want cleanup after staging failure", runner.Calls)
	}
}

func TestWriteTemplateCopyFile_NativeRenameFailureLeavesDestinationAndCleansStage(t *testing.T) {
	filesystem := fsys.NewMemFS()
	if err := filesystem.WriteFile("/target", []byte("old"), 0o640); err != nil {
		t.Fatal(err)
	}
	mgr, _ := newMemManager(t)
	mgr = mgr.WithFS(failingCopyRenameFS{FS: filesystem})

	err := mgr.writeTemplateCopyFile("/target", []byte("new"), 0o600, false, false)
	if err == nil {
		t.Fatal("expected rename failure")
	}
	got, readErr := filesystem.ReadFile("/target")
	if readErr != nil || string(got) != "old" {
		t.Fatalf("destination = %q, %v; want unchanged old content", got, readErr)
	}
	entries, err := filesystem.ReadDir("/")
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".tidydots-copy-") {
			t.Errorf("staging file %q was not cleaned up", entry.Name())
		}
	}
}

func TestWriteTemplateCopyFile_NativeExclusiveDoesNotOverwriteBackup(t *testing.T) {
	mgr, filesystem := newMemManager(t)
	if err := filesystem.WriteFile("/backup", []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := mgr.writeTemplateCopyFile("/backup", []byte("replace"), 0o600, false, true)
	if !errors.Is(err, fs.ErrExist) {
		t.Fatalf("error = %v, want ErrExist", err)
	}
	got, readErr := filesystem.ReadFile("/backup")
	if readErr != nil || string(got) != "keep" {
		t.Fatalf("backup = %q, %v; want unchanged keep", got, readErr)
	}
}

func TestWriteTemplateCopyFile_NativeReplacementDoesNotModifySymlinkReferent(t *testing.T) {
	mgr, filesystem := newMemManager(t)
	if err := filesystem.WriteFile("/referent", []byte("referent"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := filesystem.Symlink("/referent", "/target"); err != nil {
		t.Fatal(err)
	}

	if err := mgr.writeTemplateCopyFile("/target", []byte("replacement"), 0o600, false, false); err != nil {
		t.Fatalf("writeTemplateCopyFile: %v", err)
	}
	if info, err := filesystem.Lstat("/target"); err != nil || info.Mode()&fs.ModeSymlink != 0 {
		t.Fatalf("target = %v, %v; want regular file", info, err)
	}
	got, err := filesystem.ReadFile("/referent")
	if err != nil || string(got) != "referent" {
		t.Fatalf("referent = %q, %v; want unchanged referent", got, err)
	}
	got, err = filesystem.ReadFile("/target")
	if err != nil || string(got) != "replacement" {
		t.Fatalf("target = %q, %v; want replacement", got, err)
	}
}

func TestWriteTemplateCopyFile_NativePreservesRestrictiveMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Windows chmod does not support POSIX permission bits")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "target")
	mgr := newUtilityManager()

	if err := mgr.writeTemplateCopyFile(path, []byte("secret"), 0o600, false, false); err != nil {
		t.Fatalf("writeTemplateCopyFile: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Errorf("mode = %o, want 0600", got)
	}
}

func TestWriteTemplateCopyFile_DryRunDoesNotWriteOrRunCommands(t *testing.T) {
	mgr, filesystem, runner := newSudoManager(t)
	mgr.DryRun = true
	if err := filesystem.WriteFile("/target", []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := mgr.writeTemplateCopyFile("/target", []byte("new"), 0o600, true, false)
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		if err == nil || !strings.Contains(err.Error(), "sudo operations are unsupported") {
			t.Fatalf("dry-run must reject unsupported sudo runtime: %v", err)
		}
	} else if err != nil {
		t.Fatalf("writeTemplateCopyFile dry-run: %v", err)
	}
	if len(runner.Calls) != 0 {
		t.Fatalf("dry-run spawned mutation commands: %+v", runner.Calls)
	}
	got, err := filesystem.ReadFile("/target")
	if err != nil || string(got) != "old" {
		t.Fatalf("target = %q, %v; want unchanged old content", got, err)
	}
}

func TestWriteTemplateCopyFile_SudoUsesSafeCommandShapes(t *testing.T) {
	skipIfNoSudo(t)
	mgr, _, runner := newSudoManager(t)
	runner.AddResult("mktemp", cmdexec.Result{Stdout: []byte("/etc/.tidydots-copy-stage\n")})
	runner.AddResult("dd", cmdexec.Result{})
	runner.AddResult("chmod", cmdexec.Result{})
	runner.AddResult("mv", cmdexec.Result{})

	if err := mgr.writeTemplateCopyFile("/etc/root", []byte("private"), 0o640, true, false); err != nil {
		t.Fatalf("writeTemplateCopyFile: %v", err)
	}
	if len(runner.Calls) != 4 {
		t.Fatalf("calls = %+v, want mktemp, dd, chmod, mv", runner.Calls)
	}
	if got := runner.Calls[0]; got.Name != "mktemp" || !got.Sudo || len(got.Args) != 2 ||
		got.Args[0] != "--tmpdir=/etc" || got.Args[1] != ".tidydots-copy-XXXXXXXXXX" {
		t.Errorf("mktemp call = %+v", got)
	}
	if got := runner.Calls[1]; got.Name != "dd" || !got.Sudo || string(got.Stdin) != "private" ||
		len(got.Args) != 2 || got.Args[0] != "of=/etc/.tidydots-copy-stage" || got.Args[1] != "status=none" {
		t.Errorf("dd call = %+v", got)
	}
	if got := runner.Calls[2]; got.Name != "chmod" || !got.Sudo || len(got.Args) != 3 ||
		got.Args[0] != "640" || got.Args[1] != "--" || got.Args[2] != "/etc/.tidydots-copy-stage" {
		t.Errorf("chmod call = %+v", got)
	}
	if got := runner.Calls[3]; got.Name != "mv" || !got.Sudo || len(got.Args) != 4 ||
		got.Args[0] != "-fT" || got.Args[1] != "--" || got.Args[2] != "/etc/.tidydots-copy-stage" || got.Args[3] != "/etc/root" {
		t.Errorf("mv call = %+v", got)
	}
}

func TestWriteTemplateCopyFile_SudoExclusivePublishesWithoutOverwrite(t *testing.T) {
	skipIfNoSudo(t)
	mgr, _, runner := newSudoManager(t)
	runner.AddResult("mktemp", cmdexec.Result{Stdout: []byte("/etc/.tidydots-copy-stage\n")})
	runner.AddResult("dd", cmdexec.Result{})
	runner.AddResult("chmod", cmdexec.Result{})
	runner.AddResult("ln", cmdexec.Result{})
	runner.AddResult("rm", cmdexec.Result{})

	if err := mgr.writeTemplateCopyFile("/etc/backup", []byte("data"), 0o600, true, true); err != nil {
		t.Fatalf("writeTemplateCopyFile: %v", err)
	}
	if len(runner.Calls) != 5 || runner.Calls[3].Name != "ln" || runner.Calls[4].Name != "rm" {
		t.Fatalf("calls = %+v, want ln then rm after staging", runner.Calls)
	}
	if got := runner.Calls[3]; len(got.Args) != 4 || got.Args[0] != "-T" || got.Args[1] != "--" ||
		got.Args[2] != "/etc/.tidydots-copy-stage" || got.Args[3] != "/etc/backup" {
		t.Errorf("ln call = %+v", got)
	}
	if got := runner.Calls[4]; len(got.Args) != 3 || got.Args[0] != "-f" || got.Args[1] != "--" || got.Args[2] != "/etc/.tidydots-copy-stage" {
		t.Errorf("rm call = %+v", got)
	}
}

func TestWriteTemplateCopyFile_SudoRenameFailureCleansStage(t *testing.T) {
	skipIfNoSudo(t)
	mgr, _, runner := newSudoManager(t)
	runner.AddResult("mktemp", cmdexec.Result{Stdout: []byte("/etc/.tidydots-copy-stage\n")})
	runner.AddResult("dd", cmdexec.Result{})
	runner.AddResult("chmod", cmdexec.Result{})
	runner.AddResult("mv", cmdexec.Result{ExitCode: 1})

	if err := mgr.writeTemplateCopyFile("/etc/target", []byte("data"), 0o600, true, false); err == nil {
		t.Fatal("nonzero mv result must fail deployment")
	}
	if len(runner.Calls) != 5 || runner.Calls[4].Name != "rm" {
		t.Fatalf("calls = %+v, want cleanup rm after failed mv", runner.Calls)
	}
}

func TestWriteTemplateCopyFile_SudoRejectsInvalidStageWithoutUsingIt(t *testing.T) {
	skipIfNoSudo(t)
	tests := []struct {
		name  string
		stage string
	}{
		{name: "empty", stage: ""},
		{name: "relative", stage: ".tidydots-copy-stage"},
		{name: "outside parent", stage: "/tmp/.tidydots-copy-stage"},
		{name: "wrong prefix", stage: "/etc/not-a-copy-stage"},
		{name: "multiple paths", stage: "/etc/.tidydots-copy-stage\n/etc/other"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, _, runner := newSudoManager(t)
			runner.AddResult("mktemp", cmdexec.Result{Stdout: []byte(tt.stage + "\n")})

			if err := mgr.writeTemplateCopyFile("/etc/target", []byte("data"), 0o600, true, false); err == nil {
				t.Fatal("invalid stage path must fail")
			}
			if len(runner.Calls) != 1 || runner.Calls[0].Name != "mktemp" {
				t.Fatalf("calls = %+v, want only mktemp", runner.Calls)
			}
		})
	}
}

func TestWriteTemplateCopyFile_SudoExclusiveConflictCleansStage(t *testing.T) {
	skipIfNoSudo(t)
	mgr, _, runner := newSudoManager(t)
	runner.AddResult("mktemp", cmdexec.Result{Stdout: []byte("/etc/.tidydots-copy-stage\n")})
	runner.AddResult("dd", cmdexec.Result{})
	runner.AddResult("chmod", cmdexec.Result{})
	runner.AddResult("ln", cmdexec.Result{ExitCode: 1})

	if err := mgr.writeTemplateCopyFile("/etc/backup", []byte("data"), 0o600, true, true); err == nil {
		t.Fatal("occupied exclusive destination must fail")
	}
	if len(runner.Calls) != 5 || runner.Calls[3].Name != "ln" || runner.Calls[4].Name != "rm" {
		t.Fatalf("calls = %+v, want failed ln followed by cleanup rm", runner.Calls)
	}
	for _, call := range runner.Calls {
		if call.Name == "mv" {
			t.Fatal("exclusive publish must not use overwrite-capable mv")
		}
	}
}

func TestRemoveTemplateCopyArtifact_UsesNonRecursiveOperations(t *testing.T) {
	mgr, filesystem := newMemManager(t)
	if err := filesystem.MkdirAll("/directory", 0o755); err != nil {
		t.Fatal(err)
	}
	if err := filesystem.WriteFile("/directory/file", []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := mgr.removeTemplateCopyArtifact("/directory", false); err == nil {
		t.Fatal("removing non-empty directory should fail")
	}
	if _, err := filesystem.Lstat("/directory/file"); err != nil {
		t.Fatalf("directory contents were removed: %v", err)
	}
}

func TestRemoveTemplateCopyArtifact_DryRunDoesNothing(t *testing.T) {
	mgr, filesystem, runner := newSudoManager(t)
	mgr.DryRun = true
	if err := filesystem.WriteFile("/artifact", []byte("keep"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := mgr.removeTemplateCopyArtifact("/artifact", true)
	if runtime.GOOS != "linux" && runtime.GOOS != "windows" {
		if err == nil || !strings.Contains(err.Error(), "sudo operations are unsupported") {
			t.Fatalf("dry-run must reject unsupported sudo runtime: %v", err)
		}
	} else if err != nil {
		t.Fatalf("removeTemplateCopyArtifact dry-run: %v", err)
	}
	if len(runner.Calls) != 0 {
		t.Fatalf("dry-run spawned mutation commands: %+v", runner.Calls)
	}
	if _, err := filesystem.Lstat("/artifact"); err != nil {
		t.Fatalf("artifact removed during dry-run: %v", err)
	}
}

func TestRemoveTemplateCopyArtifact_SudoChecksExitCode(t *testing.T) {
	skipIfNoSudo(t)
	mgr, _, runner := newSudoManager(t)
	runner.AddResult("rm", cmdexec.Result{ExitCode: 1})

	if err := mgr.removeTemplateCopyArtifact("/etc/artifact", true); err == nil {
		t.Fatal("nonzero rm result with nil runner error must fail")
	}
	if len(runner.Calls) != 1 || runner.Calls[0].Name != "rm" || !runner.Calls[0].Sudo {
		t.Fatalf("calls = %+v, want one sudo rm", runner.Calls)
	}
}

type failingCopyRenameFS struct {
	fsys.FS
}

func (failingCopyRenameFS) Rename(_, _ string) error {
	return errors.New("simulated rename failure")
}
