package manager

import (
	"errors"
	"io/fs"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
)

func TestReadTemplateCopyTarget_SudoStatFailureRequiresPositiveEntryProbe(t *testing.T) {
	skipIfNoSudo(t)
	tests := []struct {
		name       string
		findResult cmdexec.Result
		wantErr    bool
	}{
		{
			name:       "existing basename is not absence",
			findResult: cmdexec.Result{Stdout: []byte("/parent/target\x00")},
			wantErr:    true,
		},
		{
			name:       "missing basename is positively absent",
			findResult: cmdexec.Result{},
		},
		{
			name:       "entry probe io error stays an error",
			findResult: cmdexec.Result{ExitCode: 1},
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, filesystem, runner := newSudoManager(t)
			mgr = mgr.WithFS(deniedCopyLstatFS{FS: filesystem, denied: "/parent/target"})
			runner.AddResult("stat", cmdexec.Result{ExitCode: 1})
			runner.AddResult("stat", cmdexec.Result{Stdout: []byte("41ed 0 42 100\n")})
			runner.AddResult("find", tt.findResult)

			got, err := mgr.readTemplateCopyTarget("/parent/target", true)
			if len(runner.Calls) != 3 || runner.Calls[0].Name != "stat" || runner.Calls[1].Name != "stat" || runner.Calls[2].Name != "find" {
				t.Fatalf("calls = %+v, want target stat, parent stat, entry probe", runner.Calls)
			}
			if parentArgs := runner.Calls[1].Args; len(parentArgs) != 4 || parentArgs[0] != "--dereference" ||
				parentArgs[1] != "--printf="+templateCopyStatFormat || parentArgs[2] != "--" || parentArgs[3] != "/parent" {
				t.Errorf("parent stat args = %v, want followed strict metadata", parentArgs)
			}
			if gotArgs := runner.Calls[2].Args; len(gotArgs) != 8 || gotArgs[0] != "-H" ||
				gotArgs[1] != "--" || gotArgs[2] != "/parent" || gotArgs[3] != "-mindepth" ||
				gotArgs[4] != "1" || gotArgs[5] != "-maxdepth" || gotArgs[6] != "1" || gotArgs[7] != "-print0" {
				t.Errorf("entry probe args = %v, want bounded no-follow parent enumeration", gotArgs)
			}
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected target inspection error")
				}
				return
			}
			if err != nil {
				t.Fatalf("readTemplateCopyTarget: %v", err)
			}
			if got.Exists || got.Symlink || len(got.Content) != 0 || got.Mode != 0 ||
				got.RootOwned || got.Device != 0 || got.Inode != 0 || got.IdentityKnown {
				t.Errorf("snapshot = %+v, want absent snapshot", got)
			}
		})
	}
}

func TestParseTemplateCopyStat_RequiresRawNumericMetadata(t *testing.T) {
	tests := []struct {
		name    string
		data    string
		wantErr bool
	}{
		{
			name: "regular file",
			data: "8180 0 42 99\n",
		},
		{
			name:    "legacy human file type",
			data:    "600 0 42 99 regular file\n",
			wantErr: true,
		},
		{
			name:    "incomplete",
			data:    "8180 0 42\n",
			wantErr: true,
		},
		{
			name:    "invalid raw mode",
			data:    "not-hex 0 42 99\n",
			wantErr: true,
		},
		{
			name:    "invalid uid",
			data:    "8180 root 42 99\n",
			wantErr: true,
		},
		{
			name:    "special file",
			data:    "11a4 0 42 99\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mode, rootOwned, device, inode, err := parseTemplateCopyStat([]byte(tt.data))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("parseTemplateCopyStat(%q) succeeded", tt.data)
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTemplateCopyStat: %v", err)
			}
			if mode != 0o600 || !rootOwned || device != 42 || inode != 99 {
				t.Errorf("metadata = mode %o root=%v device=%d inode=%d, want regular 0600 root/device=42/inode=99",
					mode, rootOwned, device, inode)
			}
		})
	}
}

func TestReadTemplateCopyTarget_SudoUsesFollowedMetadataForSymlink(t *testing.T) {
	skipIfNoSudo(t)
	mgr, filesystem, runner := newSudoManager(t)
	mgr = mgr.WithFS(deniedCopyLstatFS{FS: filesystem, denied: "/link"})
	runner.AddResult("stat", cmdexec.Result{Stdout: []byte("a1ff 0 1 10\n")})
	runner.AddResult("stat", cmdexec.Result{Stdout: []byte("8180 0 42 99\n")})
	runner.AddResult("cat", cmdexec.Result{Stdout: []byte("root content")})

	got, err := mgr.readTemplateCopyTarget("/link", true)
	if err != nil {
		t.Fatalf("readTemplateCopyTarget: %v", err)
	}
	if !got.Exists || !got.Symlink || string(got.Content) != "root content" || got.Mode != 0o600 {
		t.Errorf("snapshot = %+v, want followed regular-file snapshot", got)
	}
	if !got.RootOwned || !got.IdentityKnown || got.Device != 42 || got.Inode != 99 {
		t.Errorf("metadata = %+v, want referent metadata", got)
	}
	if len(runner.Calls) != 3 || runner.Calls[0].Name != "stat" || runner.Calls[1].Name != "stat" || runner.Calls[2].Name != "cat" {
		t.Fatalf("calls = %+v, want no-follow stat, followed stat, cat", runner.Calls)
	}
	if len(runner.Calls[1].Args) == 0 || runner.Calls[1].Args[0] != "--dereference" {
		t.Errorf("followed stat args = %v, want --dereference", runner.Calls[1].Args)
	}
}

func TestReadTemplateCopyTarget_SudoDanglingSymlinkPreservesLinkState(t *testing.T) {
	skipIfNoSudo(t)
	mgr, filesystem, runner := newSudoManager(t)
	mgr = mgr.WithFS(deniedCopyLstatFS{FS: filesystem, denied: "/dangling"})
	runner.AddResult("stat", cmdexec.Result{Stdout: []byte("a1ff 0 1 10\n")})
	runner.AddResult("stat", cmdexec.Result{ExitCode: 1})
	runner.AddResult("find", cmdexec.Result{Stdout: []byte("l")})

	got, err := mgr.readTemplateCopyTarget("/dangling", true)
	if err != nil {
		t.Fatalf("readTemplateCopyTarget: %v", err)
	}
	if got.Exists || !got.Symlink {
		t.Errorf("snapshot = %+v, want dangling symlink state", got)
	}
	for _, call := range runner.Calls {
		if call.Name == "cat" {
			t.Fatal("dangling referent must not be read")
		}
	}
}

func TestReadTemplateCopyTarget_SudoUnreadableReferentReturnsError(t *testing.T) {
	skipIfNoSudo(t)
	mgr, filesystem, runner := newSudoManager(t)
	mgr = mgr.WithFS(deniedCopyLstatFS{FS: filesystem, denied: "/link"})
	runner.AddResult("stat", cmdexec.Result{Stdout: []byte("a1ff 0 1 10\n")})
	runner.AddResult("stat", cmdexec.Result{Stdout: []byte("8180 0 42 99\n")})
	runner.AddResult("cat", cmdexec.Result{ExitCode: 1, Stderr: []byte("permission denied")})

	_, err := mgr.readTemplateCopyTarget("/link", true)
	if err == nil {
		t.Fatal("unreadable referent must return an error")
	}
	if strings.Contains(err.Error(), "root content") {
		t.Fatal("error leaked file content")
	}
}

func TestReadTemplateCopyTarget_SudoRejectsSpecialReferentBeforeCat(t *testing.T) {
	skipIfNoSudo(t)
	mgr, filesystem, runner := newSudoManager(t)
	mgr = mgr.WithFS(deniedCopyLstatFS{FS: filesystem, denied: "/link"})
	runner.AddResult("stat", cmdexec.Result{Stdout: []byte("a1ff 0 1 10\n")})
	runner.AddResult("stat", cmdexec.Result{Stdout: []byte("11a4 0 42 99\n")})

	_, err := mgr.readTemplateCopyTarget("/link", true)
	if err == nil {
		t.Fatal("special referent must be rejected")
	}
	if len(runner.Calls) != 2 {
		t.Fatalf("calls = %+v, want no cat for special referent", runner.Calls)
	}
}

func TestTemplateCopySudoPolicy(t *testing.T) {
	tests := []struct {
		name     string
		goos     string
		sudo     bool
		wantSudo bool
		wantErr  bool
	}{
		{name: "linux elevated", goos: "linux", sudo: true, wantSudo: true},
		{name: "linux native", goos: "linux", wantSudo: false},
		{name: "windows native even when requested", goos: "windows", sudo: true},
		{name: "windows native", goos: "windows"},
		{name: "darwin unsupported elevated", goos: "darwin", sudo: true, wantErr: true},
		{name: "darwin native", goos: "darwin"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotSudo, err := templateCopySudoPolicy(tt.goos, tt.sudo)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected unsupported-platform error")
				}
				return
			}
			if err != nil {
				t.Fatalf("templateCopySudoPolicy: %v", err)
			}
			if gotSudo != tt.wantSudo {
				t.Errorf("useSudo = %v, want %v", gotSudo, tt.wantSudo)
			}
		})
	}
}

func TestReadTemplateCopyTarget_SudoStatFailureWithGoErrorStaysError(t *testing.T) {
	skipIfNoSudo(t)
	mgr, filesystem, runner := newSudoManager(t)
	mgr = mgr.WithFS(deniedCopyLstatFS{FS: filesystem, denied: "/parent/target"})
	runner.AddResult("stat", cmdexec.Result{ExitCode: 1})
	runner.AddResult("stat", cmdexec.Result{Stdout: []byte("41ed 0 42 100\n")})
	runner.AddResult("find", cmdexec.Result{ExitCode: 1})

	_, err := mgr.readTemplateCopyTarget("/parent/target", true)
	if err == nil {
		t.Fatal("entry-probe failure must not be treated as absence")
	}
}

func TestReadTemplateCopyTarget_SudoStatFailureRejectsNonDirectoryParent(t *testing.T) {
	skipIfNoSudo(t)
	mgr, filesystem, runner := newSudoManager(t)
	mgr = mgr.WithFS(deniedCopyLstatFS{FS: filesystem, denied: "/parent/target"})
	runner.AddResult("stat", cmdexec.Result{ExitCode: 1})
	runner.AddResult("stat", cmdexec.Result{Stdout: []byte("8180 0 42 100\n")})
	runner.AddResult("find", cmdexec.Result{})

	_, err := mgr.readTemplateCopyTarget("/parent/target", true)
	if err == nil {
		t.Fatal("regular parent must not verify target absence")
	}
	if !strings.Contains(err.Error(), "stat exited with status 1") {
		t.Fatalf("error = %v, want original target inspection error preserved", err)
	}
	if len(runner.Calls) != 2 || runner.Calls[1].Name != "stat" {
		t.Fatalf("calls = %+v, want target and parent stat only", runner.Calls)
	}
}

func TestReadTemplateCopyTarget_SudoStatFailurePreservesParentMetadataError(t *testing.T) {
	skipIfNoSudo(t)
	tests := []struct {
		name         string
		parentResult cmdexec.Result
	}{
		{name: "parent stat failed", parentResult: cmdexec.Result{ExitCode: 1}},
		{name: "parent metadata malformed", parentResult: cmdexec.Result{Stdout: []byte("not metadata\n")}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr, filesystem, runner := newSudoManager(t)
			mgr = mgr.WithFS(deniedCopyLstatFS{FS: filesystem, denied: "/parent/target"})
			runner.AddResult("stat", cmdexec.Result{ExitCode: 1})
			runner.AddResult("stat", tt.parentResult)
			runner.AddResult("find", cmdexec.Result{})

			_, err := mgr.readTemplateCopyTarget("/parent/target", true)
			if err == nil {
				t.Fatal("failed parent verification must not be treated as absence")
			}
			if !strings.Contains(err.Error(), "stat exited with status 1") {
				t.Fatalf("error = %v, want original target inspection error preserved", err)
			}
			if len(runner.Calls) != 2 || runner.Calls[1].Name != "stat" {
				t.Fatalf("calls = %+v, want target and parent stat only", runner.Calls)
			}
		})
	}
}

func TestTemplateCopyStatParserDoesNotAcceptHumanFileType(t *testing.T) {
	_, _, _, _, err := parseTemplateCopyStat([]byte("8180 0 42 99 regular file\n"))
	if err == nil {
		t.Fatal("human file-type suffix must be rejected")
	}
	if errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("parser returned unrelated absence error: %v", err)
	}
}
