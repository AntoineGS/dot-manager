package cmdexec_test

import (
	"context"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
)

func TestOsRunner_RunIn_SetsWorkingDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("pwd is not available on Windows")
	}

	dir := t.TempDir()

	res, err := cmdexec.OsRunner{}.RunIn(context.Background(), cmdexec.RunOptions{Dir: dir}, "pwd")
	if err != nil {
		t.Fatalf("RunIn returned error: %v", err)
	}

	// t.TempDir can sit under a symlinked root (on macOS /var -> /private/var).
	// os/exec sets PWD=Dir verbatim in the child, and `pwd` reports that logical
	// path, so the two sides can spell the same directory differently. Resolve
	// both before comparing.
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatalf("EvalSymlinks(dir): %v", err)
	}

	got, err := filepath.EvalSymlinks(strings.TrimSpace(string(res.Stdout)))
	if err != nil {
		t.Fatalf("EvalSymlinks(stdout): %v", err)
	}

	if got != want {
		t.Errorf("pwd = %q, want %q", got, want)
	}
}

func TestOsRunner_RunIn_NonZeroExitReportsExitCode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("sh is not available on Windows")
	}

	res, err := cmdexec.OsRunner{}.RunIn(context.Background(), cmdexec.RunOptions{}, "sh", "-c", "exit 3")
	if err == nil {
		t.Fatal("RunIn returned nil error for a non-zero exit")
	}

	if res.ExitCode != 3 {
		t.Errorf("ExitCode = %d, want 3", res.ExitCode)
	}
}

func TestOsRunner_RunIn_ForwardsStdin(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("cat is not available on Windows")
	}

	res, err := cmdexec.OsRunner{}.RunIn(
		context.Background(),
		cmdexec.RunOptions{Stdin: []byte("private input")},
		"cat",
	)
	if err != nil {
		t.Fatalf("RunIn returned error: %v", err)
	}
	if string(res.Stdout) != "private input" {
		t.Errorf("stdout = %q, want %q", res.Stdout, "private input")
	}
}
