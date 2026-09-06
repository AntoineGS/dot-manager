package manager

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/fsys"
)

func TestInspectCopyTemplatesUsesLiveTarget(t *testing.T) {
	backup, target, mgr, _ := setupTemplateTest(t)
	entry := config.SubEntry{Name: "copy", Method: config.MethodCopy,
		Backup: backup, Files: []string{"root.tmpl"}}
	writeTemplateFile(t, filepath.Join(backup, "root.tmpl"), "a=1")
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatal(err)
	}
	writeTemplateFile(t, filepath.Join(target, "root"), "a=2")
	writeTemplateFile(t, filepath.Join(backup, "root.tmpl.rendered"), "cache is not authoritative")
	got, err := mgr.InspectCopyTemplates(entry, backup, target)
	if err != nil {
		t.Fatal(err)
	}
	if got.NeedsRestore || got.Outdated || len(got.Modified) != 1 {
		t.Fatalf("unexpected inspection: %+v", got)
	}
	if got.Modified[0].CurrentPath != filepath.Join(target, "root") ||
		string(got.Modified[0].CurrentOnDisk) != "a=2" {
		t.Fatalf("wrong edited side: %+v", got.Modified[0])
	}
}

func TestInspectCopyTemplatesReportsSourceDriftAsOutdated(t *testing.T) {
	backup, target, mgr, _ := setupTemplateTest(t)
	entry := config.SubEntry{Method: config.MethodCopy, Backup: backup, Files: []string{"root.tmpl"}}
	source := filepath.Join(backup, "root.tmpl")
	writeTemplateFile(t, source, "a=1")
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatal(err)
	}
	writeTemplateFile(t, source, "a=2")

	got, err := mgr.InspectCopyTemplates(entry, backup, target)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Outdated || got.NeedsRestore || len(got.Modified) != 0 {
		t.Fatalf("inspection = %+v, want source drift only", got)
	}
}

func TestInspectCopyTemplatesReportsMissingTargetAsNeedsRestore(t *testing.T) {
	backup, target, mgr, _ := setupTemplateTest(t)
	entry := config.SubEntry{Method: config.MethodCopy, Backup: backup, Files: []string{"root.tmpl"}}
	writeTemplateFile(t, filepath.Join(backup, "root.tmpl"), "a=1")
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatal(err)
	}
	if err := mgr.fs.Remove(filepath.Join(target, "root")); err != nil {
		t.Fatal(err)
	}

	got, err := mgr.InspectCopyTemplates(entry, backup, target)
	if err != nil {
		t.Fatal(err)
	}
	if !got.NeedsRestore || got.Outdated || len(got.Modified) != 0 {
		t.Fatalf("inspection = %+v, want missing target only", got)
	}
}

func TestInspectCopyTemplatesMissingHistoryIsOutdated(t *testing.T) {
	backup, target, mgr, _ := setupTemplateTest(t)
	entry := config.SubEntry{Method: config.MethodCopy, Backup: backup, Files: []string{"root.tmpl"}}
	writeTemplateFile(t, filepath.Join(backup, "root.tmpl"), "a=1")
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatal(err)
	}
	mgr.stateStore = nil

	got, err := mgr.InspectCopyTemplates(entry, backup, target)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Outdated || got.NeedsRestore || len(got.Modified) != 0 {
		t.Fatalf("inspection = %+v, want missing history only", got)
	}
}

func TestInspectCopyTemplatesRejectsInvalidSource(t *testing.T) {
	backup, target, mgr, _ := setupTemplateTest(t)
	entry := config.SubEntry{Method: config.MethodCopy, Backup: backup, Files: []string{"root.tmpl"}}
	writeTemplateFile(t, filepath.Join(backup, "root.tmpl"), "{{ invalid")

	if _, err := mgr.InspectCopyTemplates(entry, backup, target); err == nil {
		t.Fatal("invalid copy template was reported as inspectable")
	}
}

func TestInspectCopyTemplatesSymlinkTargetNeedsRestore(t *testing.T) {
	skipIfNoSymlink(t)
	backup, target, mgr, _ := setupTemplateTest(t)
	entry := config.SubEntry{Method: config.MethodCopy, Backup: backup, Files: []string{"root.tmpl"}}
	writeTemplateFile(t, filepath.Join(backup, "root.tmpl"), "a=1")
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatal(err)
	}
	linkTarget := filepath.Join(t.TempDir(), "outside")
	writeTemplateFile(t, linkTarget, "a=1")
	if err := mgr.fs.Remove(filepath.Join(target, "root")); err != nil {
		t.Fatal(err)
	}
	if err := mgr.fs.Symlink(linkTarget, filepath.Join(target, "root")); err != nil {
		t.Fatal(err)
	}

	got, err := mgr.InspectCopyTemplates(entry, backup, target)
	if err != nil {
		t.Fatal(err)
	}
	if !got.NeedsRestore {
		t.Fatalf("inspection = %+v, want symlink target to need restore", got)
	}
}

func TestInspectCopyTemplatesSymlinkReferentsNeedRestoreWithoutFollowing(t *testing.T) {
	skipIfNoSymlink(t)
	tests := []struct {
		name      string
		configure func(t *testing.T, referent string)
		statErr   error
		statInfo  fs.FileInfo
		readErr   error
	}{
		{
			name: "directory referent",
			configure: func(t *testing.T, referent string) {
				if err := os.MkdirAll(referent, 0o750); err != nil {
					t.Fatal(err)
				}
			},
			statInfo: staticCopyFileInfo{mode: fs.ModeDir},
		},
		{
			name:    "missing referent",
			statErr: fs.ErrNotExist,
		},
		{
			name:    "unreadable referent",
			statErr: fs.ErrPermission,
			readErr: fs.ErrPermission,
		},
		{
			name:     "special referent",
			statInfo: staticCopyFileInfo{mode: fs.ModeDevice},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backup, target, mgr, _ := setupTemplateTest(t)
			entry := config.SubEntry{Method: config.MethodCopy, Backup: backup, Files: []string{"root.tmpl"}}
			writeTemplateFile(t, filepath.Join(backup, "root.tmpl"), "a=1")
			if err := mgr.RestoreFiles(entry, backup, target); err != nil {
				t.Fatal(err)
			}

			destination := filepath.Join(target, "root")
			if err := os.Remove(destination); err != nil {
				t.Fatal(err)
			}
			referent := filepath.Join(t.TempDir(), "referent")
			if tt.configure != nil {
				tt.configure(t, referent)
			}
			if err := os.Symlink(referent, destination); err != nil {
				t.Fatal(err)
			}

			probeFS := &symlinkReferentProbeFS{
				FS:       fsys.OsFS{},
				link:     destination,
				statErr:  tt.statErr,
				statInfo: tt.statInfo,
				readErr:  tt.readErr,
			}
			got, err := mgr.WithFS(probeFS).InspectCopyTemplates(entry, backup, target)
			if err != nil {
				t.Fatalf("InspectCopyTemplates: %v", err)
			}
			if !got.NeedsRestore || got.Outdated || len(got.Modified) != 0 {
				t.Fatalf("inspection = %+v, want restore-needed link", got)
			}
			if probeFS.statCalls != 0 || probeFS.readCalls != 0 {
				t.Fatalf("symlink referent was followed: stat=%d read=%d", probeFS.statCalls, probeFS.readCalls)
			}
		})
	}
}

func TestInspectCopyTemplatesDeniedNativeParentTraversalDoesNotSpawn(t *testing.T) {
	mgr, filesystem, runner := newSudoManager(t)
	if err := filesystem.MkdirAll("/backup", 0o750); err != nil {
		t.Fatal(err)
	}
	if err := filesystem.MkdirAll("/target", 0o750); err != nil {
		t.Fatal(err)
	}
	if err := filesystem.WriteFile("/backup/root.tmpl", []byte("a=1"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := filesystem.WriteFile("/target/root", []byte("a=1"), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := config.SubEntry{Method: config.MethodCopy, Sudo: true, Backup: "/backup", Files: []string{"root.tmpl"}}
	_, err := mgr.WithFS(deniedCopyParentLstatFS{
		FS:     filesystem,
		denied: "/target",
	}).InspectCopyTemplates(entry, "/backup", "/target")
	if err == nil || !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("inspection error = %v, want denied parent traversal", err)
	}
	if len(runner.Calls) != 0 {
		t.Fatalf("status inspection spawned privileged commands: %+v", runner.Calls)
	}
}

func TestInspectCopyTemplatesMissingDescendantBelowSymlinkAncestorReturnsError(t *testing.T) {
	skipIfNoSymlink(t)
	backup, target, mgr, _ := setupTemplateTest(t)
	source := filepath.Join(backup, "nested", "missing", "file.tmpl")
	writeTemplateFile(t, source, "a=1")

	outside := t.TempDir()
	if err := os.Symlink(outside, filepath.Join(target, "nested")); err != nil {
		t.Fatal(err)
	}
	runner := cmdexec.NewStubRunner()
	entry := config.SubEntry{
		Method: config.MethodCopy,
		Sudo:   true,
		Backup: backup,
		Files:  []string{"nested/missing/file.tmpl"},
	}

	_, err := mgr.WithRunner(runner).InspectCopyTemplates(entry, backup, target)
	if err == nil {
		t.Fatal("inspection followed an existing symlink ancestor above a missing descendant")
	}
	if !strings.Contains(err.Error(), "symlink parent") {
		t.Fatalf("inspection error = %v, want symlink-parent error", err)
	}
	if len(runner.Calls) != 0 {
		t.Fatalf("status inspection spawned privileged commands: %+v", runner.Calls)
	}
}

func TestInspectCopyTemplatesWrongSudoOwnerNeedsRestore(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("sudo ownership is Linux-specific")
	}
	backup, target, mgr, _ := setupTemplateTest(t)
	entry := config.SubEntry{Method: config.MethodCopy, Sudo: true, Backup: backup, Files: []string{"root.tmpl"}}
	writeTemplateFile(t, filepath.Join(backup, "root.tmpl"), "a=1")
	seedEntry := entry
	seedEntry.Sudo = false
	if err := mgr.RestoreFiles(seedEntry, backup, target); err != nil {
		t.Fatal(err)
	}

	destination := filepath.Join(target, "root")
	got, err := mgr.WithFS(nonRootCopyLstatFS{FS: fsys.OsFS{}, target: destination}).InspectCopyTemplates(entry, backup, target)
	if err != nil {
		t.Fatal(err)
	}
	if !got.NeedsRestore || got.Outdated || len(got.Modified) != 0 {
		t.Fatalf("inspection = %+v, want restore-needed wrong owner", got)
	}
}

func TestInspectCopyTemplatesSudoNeverSpawnsForDeniedTarget(t *testing.T) {
	mgr, filesystem, runner := newSudoManager(t)
	if err := filesystem.MkdirAll("/backup", 0o750); err != nil {
		t.Fatal(err)
	}
	if err := filesystem.MkdirAll("/target", 0o750); err != nil {
		t.Fatal(err)
	}
	if err := filesystem.WriteFile("/backup/root.tmpl", []byte("a=1"), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := config.SubEntry{Method: config.MethodCopy, Sudo: true, Backup: "/backup", Files: []string{"root.tmpl"}}
	mgr = mgr.WithFS(deniedCopyLstatFS{FS: filesystem, denied: "/target/root"})

	_, err := mgr.InspectCopyTemplates(entry, "/backup", "/target")
	if err == nil || !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("inspection error = %v, want permission error", err)
	}
	if len(runner.Calls) != 0 {
		t.Fatalf("status inspection spawned privileged commands: %+v", runner.Calls)
	}
}

func TestInspectCopyTemplatesOrdinaryCopyDoesNotTouchFiles(t *testing.T) {
	mgr := newUtilityManager().WithFS(noTouchCopyFS{})
	entry := config.SubEntry{Method: config.MethodCopy, Files: []string{"literal.conf"}}

	got, err := mgr.InspectCopyTemplates(entry, "/missing-backup", "/missing-target")
	if err != nil {
		t.Fatal(err)
	}
	if got.NeedsRestore || got.Outdated || len(got.Modified) != 0 {
		t.Fatalf("inspection = %+v, want empty inspection", got)
	}
}

type noTouchCopyFS struct {
	fsys.FS
}

type symlinkReferentProbeFS struct {
	fsys.FS
	link      string
	statErr   error
	statInfo  fs.FileInfo
	readErr   error
	statCalls int
	readCalls int
}

func (f *symlinkReferentProbeFS) Stat(name string) (fs.FileInfo, error) {
	if name == f.link {
		f.statCalls++
		if f.statErr != nil {
			return nil, &fs.PathError{Op: "stat", Path: name, Err: f.statErr}
		}
		if f.statInfo != nil {
			return f.statInfo, nil
		}
	}
	return f.FS.Stat(name)
}

func (f *symlinkReferentProbeFS) ReadFile(name string) ([]byte, error) {
	if name == f.link {
		f.readCalls++
		if f.readErr != nil {
			return nil, &fs.PathError{Op: "read", Path: name, Err: f.readErr}
		}
	}
	return f.FS.ReadFile(name)
}

type staticCopyFileInfo struct {
	mode fs.FileMode
}

func (staticCopyFileInfo) Name() string        { return "referent" }
func (f staticCopyFileInfo) Size() int64       { return 0 }
func (f staticCopyFileInfo) Mode() fs.FileMode { return f.mode }
func (staticCopyFileInfo) ModTime() time.Time  { return time.Time{} }
func (f staticCopyFileInfo) IsDir() bool       { return f.mode.IsDir() }
func (staticCopyFileInfo) Sys() any            { return nil }

type deniedCopyParentLstatFS struct {
	fsys.FS
	denied string
}

func (f deniedCopyParentLstatFS) Lstat(name string) (fs.FileInfo, error) {
	if name == f.denied {
		return nil, &fs.PathError{Op: "lstat", Path: name, Err: fs.ErrPermission}
	}
	return f.FS.Lstat(name)
}

type nonRootCopyLstatFS struct {
	fsys.FS
	target string
}

func (f nonRootCopyLstatFS) Lstat(name string) (fs.FileInfo, error) {
	info, err := f.FS.Lstat(name)
	if err != nil || name != f.target {
		return info, err
	}
	return nonRootCopyFileInfo{FileInfo: info}, nil
}

type nonRootCopyFileInfo struct {
	fs.FileInfo
}

func (nonRootCopyFileInfo) Sys() any { return nonRootCopyStat{UID: 1000} }

type nonRootCopyStat struct {
	UID uint32
	Dev uint64
	Ino uint64
}
