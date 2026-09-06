package manager

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/fsys"
)

type preservationFailureFS struct {
	fsys.FS
	readFail  string
	writeFail string
}

func (f preservationFailureFS) ReadFile(path string) ([]byte, error) {
	if path == f.readFail {
		return nil, fs.ErrPermission
	}
	return f.FS.ReadFile(path)
}

func (f preservationFailureFS) WriteFileExclusive(path string, data []byte, mode fs.FileMode) error {
	if path == f.writeFail {
		return fs.ErrPermission
	}
	return f.FS.WriteFileExclusive(path, data, mode)
}

func preservationWrite(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func preservationContent(t *testing.T, path, want string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil || string(got) != want {
		t.Fatalf("%s = %q, %v; want %q", path, got, err, want)
	}
}

func TestPreservationMergeFailureRetainsOriginal(t *testing.T) {
	root, target, m, _ := setupTemplateTest(t)
	source := filepath.Join(root, "backup")
	if err := os.Mkdir(source, 0o700); err != nil {
		t.Fatal(err)
	}
	file := filepath.Join(target, "app.conf")
	preservationWrite(t, file, "edits")
	// A directory at the destination makes both rename and copying fail.
	if err := os.Mkdir(filepath.Join(source, generateConflictNameWithDate("app.conf")), 0o700); err != nil {
		t.Fatal(err)
	}
	preservationWrite(t, filepath.Join(source, "app.conf"), "repo")
	m = m.WithFS(preservationFailureFS{FS: m.fs, readFail: file})
	if err := m.RestoreFolder(config.SubEntry{Name: "app"}, source, target); err == nil {
		t.Error("expected preservation error")
	}
	preservationContent(t, file, "edits")
}

func TestPreservationMergeCollision(t *testing.T) {
	for _, symlink := range []bool{false, true} {
		t.Run(map[bool]string{false: "file", true: "symlink"}[symlink], func(t *testing.T) {
			root, target, m, _ := setupTemplateTest(t)
			file := filepath.Join(target, "app.conf")
			preservationWrite(t, file, "edits")
			preservationWrite(t, filepath.Join(root, "app.conf"), "repo")
			conflict := filepath.Join(root, generateConflictNameWithDate("app.conf"))
			if symlink {
				victim := filepath.Join(root, "victim")
				preservationWrite(t, victim, "old recovery")
				if err := os.Symlink(victim, conflict); err != nil {
					t.Fatal(err)
				}
			} else {
				preservationWrite(t, conflict, "old recovery")
			}
			summary := NewMergeSummary("app")
			if err := m.MergeFolder(root, target, false, summary); err != nil {
				t.Fatal(err)
			}
			preservationContent(t, conflict, "old recovery")
			preservationContent(t, conflict+".1", "edits")
		})
	}
}

func TestPreservationFolderTemplateAliasPreflight(t *testing.T) {
	root, target, m, _ := setupTemplateTest(t)
	preservationWrite(t, filepath.Join(root, "app.conf.tmpl"), "rendered")
	preservationWrite(t, filepath.Join(target, "app.conf"), "edits")
	if err := m.RestoreFolderWithTemplates(config.SubEntry{Name: "app"}, root, target); err == nil {
		t.Error("expected alias conflict refusal")
	}
	preservationContent(t, filepath.Join(target, "app.conf"), "edits")
	if testIsSymlink(target) {
		t.Error("target mutated before preflight")
	}
}

func TestPreservationLiteralAlias(t *testing.T) {
	root, _, m, _ := setupTemplateTest(t)
	path := filepath.Join(root, "app.conf")
	preservationWrite(t, path, "literal")
	if err := m.ensureRelativeSymlink(path, "app.conf.tmpl.rendered"); err != nil {
		t.Fatal(err)
	}
	preservationContent(t, path+".tidydots.bak", "literal")
}

func TestPreservationTemplateFailures(t *testing.T) {
	for _, failure := range []string{"read", "conflict", "orphan", "orphan-occupied"} {
		t.Run(failure, func(t *testing.T) {
			root, _, m, _ := setupTemplateTest(t)
			path := filepath.Join(root, "app.tmpl")
			out := path + ".rendered"
			preservationWrite(t, path, "base\n")
			if failure == "read" || failure == "conflict" {
				if err := m.renderTemplateAndLink(path, "app.tmpl"); err != nil {
					t.Fatal(err)
				}
			}
			preservationWrite(t, out, "user edits\n")
			preservationWrite(t, path, "new template\n")
			fault := preservationFailureFS{FS: m.fs}
			switch failure {
			case "read":
				fault.readFail = out
			case "conflict":
				fault.writeFail = path + ".conflict"
			case "orphan":
				fault.writeFail = out + ".bak"
			case "orphan-occupied":
				preservationWrite(t, out+".bak", "previous recovery")
			}
			m = m.WithFS(fault)
			if err := m.renderTemplateAndLink(path, "app.tmpl"); err == nil {
				t.Error("expected preservation error")
			}
			preservationContent(t, out, "user edits\n")
			history, err := m.templateHistory(path, "app.tmpl", "")
			if err != nil {
				t.Fatal(err)
			}
			if failure == "read" || failure == "conflict" {
				if history.record == nil || string(history.record.PureRender) != "base\n" {
					t.Fatal("render history changed on failure")
				}
			} else if history.record != nil {
				t.Fatal("history created on failed orphan preservation")
			}
			if failure == "orphan-occupied" {
				preservationContent(t, out+".bak", "previous recovery")
			}
		})
	}
}

func TestPreservationSourceAliasFailureBeforeFolderMutation(t *testing.T) {
	root, target, m, _ := setupTemplateTest(t)
	preservationWrite(t, filepath.Join(root, "app.tmpl"), "template")
	preservationWrite(t, filepath.Join(root, "app"), "literal")
	preservationWrite(t, filepath.Join(root, "app.tidydots.bak"), "older literal")
	preservationWrite(t, filepath.Join(target, "unrelated"), "target original")
	if err := m.RestoreFolderWithTemplates(config.SubEntry{Name: "app"}, root, target); err == nil {
		t.Error("expected occupied alias recovery error")
	}
	if testIsSymlink(target) {
		t.Error("target replaced before source alias preservation")
	}
	preservationContent(t, filepath.Join(target, "unrelated"), "target original")
	preservationContent(t, filepath.Join(root, "app"), "literal")
	preservationContent(t, filepath.Join(root, "app.tidydots.bak"), "older literal")
}

func TestPreservationRecoveryPermissionsAndForce(t *testing.T) {
	for _, force := range []bool{false, true} {
		t.Run(map[bool]string{false: "normal", true: "force"}[force], func(t *testing.T) {
			root, _, m, _ := setupTemplateTest(t)
			m.ForceRender = force
			path := filepath.Join(root, "app.tmpl")
			preservationWrite(t, path, "new")
			preservationWrite(t, path+".rendered", "orphan")
			preservationWrite(t, filepath.Join(root, "app"), "literal")
			if err := m.renderTemplateAndLink(path, "app.tmpl"); err != nil {
				t.Fatal(err)
			}
			preservationContent(t, path+".rendered", "new")
			preservationContent(t, filepath.Join(root, "app.tidydots.bak"), "literal")
			if force {
				if _, err := os.Lstat(path + ".rendered.bak"); !os.IsNotExist(err) {
					t.Fatalf("force created orphan backup: %v", err)
				}
			} else {
				preservationContent(t, path+".rendered.bak", "orphan")
				info, err := os.Stat(path + ".rendered.bak")
				if err != nil {
					t.Fatal(err)
				}
				if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
					t.Fatalf("recovery permissions = %o", info.Mode().Perm())
				}
			}
		})
	}
}

func TestPreservationSudoFailureRetainsOriginal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("Linux sudo policy")
	}
	root, target, m, _ := setupTemplateTest(t)
	file := filepath.Join(target, "app")
	preservationWrite(t, file, "protected edits")
	runner := cmdexec.NewStubRunner()
	runner.AddResult("rm", cmdexec.Result{ExitCode: 1})
	m = m.WithRunner(runner)
	summary := NewMergeSummary("app")
	if err := m.MergeFolder(root, target, true, summary); err == nil {
		t.Error("expected failed sudo removal to fail merge")
	}
	preservationContent(t, file, "protected edits")
	preservationContent(t, filepath.Join(root, "app"), "protected edits")
	if len(summary.FailedFiles) != 1 {
		t.Fatalf("failed files = %v", summary.FailedFiles)
	}
}

func TestPreservationOrphanSymlinkRecoveryOccupied(t *testing.T) {
	skipIfNoSymlink(t)
	root, _, m, _ := setupTemplateTest(t)
	path := filepath.Join(root, "app.tmpl")
	preservationWrite(t, path, "new")
	preservationWrite(t, path+".rendered", "orphan")
	victim := filepath.Join(root, "victim")
	preservationWrite(t, victim, "old recovery")
	if err := os.Symlink(victim, path+".rendered.bak"); err != nil {
		t.Fatal(err)
	}
	if err := m.renderTemplateAndLink(path, "app.tmpl"); err == nil {
		t.Error("expected occupied recovery error")
	}
	preservationContent(t, path+".rendered", "orphan")
	preservationContent(t, victim, "old recovery")
}

func TestPreservationMergeKeepsExecutableMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	root, target, m, _ := setupTemplateTest(t)
	file := filepath.Join(target, "script")
	preservationWrite(t, file, "#!/bin/sh\n")
	if err := os.Chmod(file, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := m.MergeFolder(root, target, false, NewMergeSummary("app")); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(filepath.Join(root, "script"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o750 {
		t.Fatalf("merged script mode = %o; want 750", info.Mode().Perm())
	}
}

func TestPreservationAliasPreflightBeforeWrongFolderLinkRemoval(t *testing.T) {
	skipIfNoSymlink(t)
	root, target, m, _ := setupTemplateTest(t)
	preservationWrite(t, filepath.Join(root, "app.tmpl"), "template")
	preservationWrite(t, filepath.Join(root, "app"), "literal")
	preservationWrite(t, filepath.Join(root, "app.tidydots.bak"), "older literal")
	link := filepath.Join(target, "link")
	other := t.TempDir()
	if err := os.Symlink(other, link); err != nil {
		t.Fatal(err)
	}
	if err := m.RestoreFolderWithTemplates(config.SubEntry{Name: "app"}, root, link); err == nil {
		t.Error("expected alias recovery error")
	}
	got, err := os.Readlink(link)
	if err != nil || got != other {
		t.Fatalf("folder link changed before alias preservation: %q, %v", got, err)
	}
}

func TestPreservationRenderDoesNotFollowOccupiedStage(t *testing.T) {
	skipIfNoSymlink(t)
	root, _, m, _ := setupTemplateTest(t)
	path := filepath.Join(root, "app.tmpl")
	preservationWrite(t, path, "template")
	victim := filepath.Join(root, "victim")
	preservationWrite(t, victim, "literal")
	if err := os.Symlink(victim, atomicTempPath(path+".rendered")); err != nil {
		t.Fatal(err)
	}
	if err := m.renderTemplateAndLink(path, "app.tmpl"); err != nil {
		t.Fatal(err)
	}
	preservationContent(t, victim, "literal")
	preservationContent(t, path+".rendered", "template")
}
