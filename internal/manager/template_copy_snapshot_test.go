package manager

import (
	"errors"
	"io/fs"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
	"github.com/AntoineGS/tidydots/internal/fsys"
)

func TestReadTemplateCopyTarget_NativeSnapshots(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(t *testing.T, filesystem *fsys.MemFS) string
		want    templateCopySnapshot
		wantErr bool
	}{
		{
			name: "regular file",
			setup: func(t *testing.T, filesystem *fsys.MemFS) string {
				t.Helper()
				if err := filesystem.WriteFile("/regular", []byte("content"), 0o640); err != nil {
					t.Fatal(err)
				}
				return "/regular"
			},
			want: templateCopySnapshot{
				Exists: true, Content: []byte("content"), Mode: 0o640,
			},
		},
		{
			name: "missing path",
			setup: func(t *testing.T, _ *fsys.MemFS) string {
				t.Helper()
				return "/missing"
			},
			want: templateCopySnapshot{},
		},
		{
			name: "symlink to regular file",
			setup: func(t *testing.T, filesystem *fsys.MemFS) string {
				t.Helper()
				if err := filesystem.WriteFile("/referent", []byte("through link"), 0o600); err != nil {
					t.Fatal(err)
				}
				if err := filesystem.Symlink("/referent", "/link"); err != nil {
					t.Fatal(err)
				}
				return "/link"
			},
			want: templateCopySnapshot{
				Exists: true, Symlink: true, Content: []byte("through link"), Mode: 0o600,
			},
		},
		{
			name: "dangling symlink",
			setup: func(t *testing.T, filesystem *fsys.MemFS) string {
				t.Helper()
				if err := filesystem.Symlink("/gone", "/dangling"); err != nil {
					t.Fatal(err)
				}
				return "/dangling"
			},
			want: templateCopySnapshot{Symlink: true},
		},
		{
			name: "directory is rejected",
			setup: func(t *testing.T, filesystem *fsys.MemFS) string {
				t.Helper()
				if err := filesystem.MkdirAll("/directory", 0o750); err != nil {
					t.Fatal(err)
				}
				return "/directory"
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, filesystem := newMemManager(t)
			path := tt.setup(t, filesystem)

			got, err := mgr.readTemplateCopyTarget(path, false)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected an error")
				}
				return
			}
			if err != nil {
				t.Fatalf("readTemplateCopyTarget: %v", err)
			}
			if got.Exists != tt.want.Exists || got.Symlink != tt.want.Symlink ||
				got.Mode != tt.want.Mode || string(got.Content) != string(tt.want.Content) {
				t.Errorf("snapshot = %+v, want %+v", got, tt.want)
			}
			if got.RootOwned || got.IdentityKnown {
				t.Errorf("memory metadata should be unknown/non-root: %+v", got)
			}
		})
	}
}

func TestReadTemplateCopyTarget_DeniedReadPreservesPermissionErrorWithoutSudo(t *testing.T) {
	mgr, filesystem, runner := newSudoManager(t)
	if err := filesystem.WriteFile("/target", []byte("secret"), 0o600); err != nil {
		t.Fatal(err)
	}
	mgr = mgr.WithFS(deniedCopyReadFS{FS: filesystem, denied: "/target"})

	_, err := mgr.readTemplateCopyTarget("/target", false)
	if !errors.Is(err, fs.ErrPermission) {
		t.Fatalf("error = %v, want permission error", err)
	}
	if len(runner.Calls) != 0 {
		t.Fatalf("sudo=false spawned commands: %+v", runner.Calls)
	}
}

func TestReadTemplateCopyTarget_SudoFallbackReadsMetadataAndContent(t *testing.T) {
	skipIfNoSudo(t)
	mgr, filesystem, runner := newSudoManager(t)
	if err := filesystem.WriteFile("/target", []byte("native content"), 0o600); err != nil {
		t.Fatal(err)
	}
	mgr = mgr.WithFS(deniedCopyReadFS{FS: filesystem, denied: "/target"})
	runner.AddResult("stat", cmdexec.Result{Stdout: []byte("8180 0 42 99\n")})
	runner.AddResult("cat", cmdexec.Result{Stdout: []byte("root content")})

	got, err := mgr.readTemplateCopyTarget("/target", true)
	if err != nil {
		t.Fatalf("readTemplateCopyTarget: %v", err)
	}
	if !got.Exists || got.Symlink || string(got.Content) != "root content" || got.Mode != 0o600 {
		t.Errorf("snapshot = %+v, want existing regular root-owned snapshot", got)
	}
	if !got.RootOwned || !got.IdentityKnown || got.Device != 42 || got.Inode != 99 {
		t.Errorf("metadata = %+v, want root/device=42/inode=99", got)
	}
	if len(runner.Calls) != 2 || runner.Calls[0].Name != "stat" || runner.Calls[1].Name != "cat" {
		t.Fatalf("calls = %+v, want sudo stat then cat", runner.Calls)
	}
	for _, call := range runner.Calls {
		if !call.Sudo {
			t.Errorf("call %q was not sudo: %+v", call.Name, call)
		}
		if call.Name == "cat" && (len(call.Args) != 2 || call.Args[0] != "--" || call.Args[1] != "/target") {
			t.Errorf("cat args = %v, want [-- /target]", call.Args)
		}
	}
}

func TestReadTemplateCopyTarget_SudoCatNonzeroWithoutErrorFails(t *testing.T) {
	skipIfNoSudo(t)
	mgr, filesystem, runner := newSudoManager(t)
	if err := filesystem.WriteFile("/target", []byte("native content"), 0o600); err != nil {
		t.Fatal(err)
	}
	mgr = mgr.WithFS(deniedCopyReadFS{FS: filesystem, denied: "/target"})
	runner.AddResult("stat", cmdexec.Result{Stdout: []byte("8180 0 42 99\n")})
	runner.AddResult("cat", cmdexec.Result{ExitCode: 1})

	_, err := mgr.readTemplateCopyTarget("/target", true)
	if err == nil {
		t.Fatal("nonzero cat result with nil runner error must fail")
	}
	if strings.Contains(err.Error(), "native content") {
		t.Error("error leaked file content")
	}
}

func TestReadTemplateCopyTarget_SudoRejectsSpecialFileFromMetadata(t *testing.T) {
	skipIfNoSudo(t)
	mgr, filesystem, runner := newSudoManager(t)
	mgr = mgr.WithFS(deniedCopyLstatFS{FS: filesystem, denied: "/target"})
	runner.AddResult("stat", cmdexec.Result{Stdout: []byte("11a4 0 42 99\n")})

	_, err := mgr.readTemplateCopyTarget("/target", true)
	if err == nil {
		t.Fatal("special file metadata must be rejected")
	}
	for _, call := range runner.Calls {
		if call.Name == "cat" {
			t.Fatal("must not read a special file")
		}
	}
}

type deniedCopyReadFS struct {
	fsys.FS
	denied string
}

func (f deniedCopyReadFS) ReadFile(name string) ([]byte, error) {
	if name == f.denied {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrPermission}
	}
	return f.FS.ReadFile(name)
}

type deniedCopyLstatFS struct {
	fsys.FS
	denied string
}

func (f deniedCopyLstatFS) Lstat(name string) (fs.FileInfo, error) {
	if filepath.Clean(name) == filepath.Clean(f.denied) {
		return nil, &fs.PathError{Op: "lstat", Path: name, Err: fs.ErrPermission}
	}
	return f.FS.Lstat(name)
}
