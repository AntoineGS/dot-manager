package cmdexec_test

import (
	"context"
	"errors"
	"os/exec"
	"testing"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
)

func TestStubRunner_Run_RecordsCallAndReturnsResult(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	want := cmdexec.Result{Stdout: []byte("hello"), ExitCode: 0}
	stub.AddResult("echo", want)

	got, err := stub.Run(context.Background(), "echo", "hello")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if string(got.Stdout) != string(want.Stdout) {
		t.Errorf("Stdout: got %q, want %q", got.Stdout, want.Stdout)
	}

	if len(stub.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(stub.Calls))
	}

	call := stub.Calls[0]
	if call.Name != "echo" {
		t.Errorf("Name: got %q, want %q", call.Name, "echo")
	}
	if call.Sudo {
		t.Error("expected Sudo=false for Run")
	}
}

func TestStubRunner_RunWithSudo_RecordsSudo(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	want := cmdexec.Result{ExitCode: 0}
	stub.AddResult("apt", want)

	_, err := stub.RunWithSudo(context.Background(), "apt", "install", "vim")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(stub.Calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(stub.Calls))
	}

	call := stub.Calls[0]
	if !call.Sudo {
		t.Error("expected Sudo=true for RunWithSudo")
	}
	if call.Name != "apt" {
		t.Errorf("Name: got %q, want %q", call.Name, "apt")
	}
}

func TestStubRunner_MultipleResults_ReturnedInFIFOOrder(t *testing.T) {
	stub := cmdexec.NewStubRunner()

	first := cmdexec.Result{Stdout: []byte("first"), ExitCode: 0}
	second := cmdexec.Result{Stdout: []byte("second"), ExitCode: 1}
	stub.AddResult("cmd", first)
	stub.AddResult("cmd", second)

	got1, _ := stub.Run(context.Background(), "cmd")
	got2, _ := stub.Run(context.Background(), "cmd")

	if string(got1.Stdout) != "first" {
		t.Errorf("first call Stdout: got %q, want %q", got1.Stdout, "first")
	}
	if string(got2.Stdout) != "second" {
		t.Errorf("second call Stdout: got %q, want %q", got2.Stdout, "second")
	}
	if got2.ExitCode != 1 {
		t.Errorf("second call ExitCode: got %d, want 1", got2.ExitCode)
	}
}

func TestStubRunner_LookPath_ReturnsRegisteredPath(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	stub.AddPath("git", "/usr/bin/git")

	got, err := stub.LookPath("git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != "/usr/bin/git" {
		t.Errorf("LookPath: got %q, want %q", got, "/usr/bin/git")
	}
}

func TestStubRunner_LookPath_ReturnsErrNotFoundForUnregistered(t *testing.T) {
	stub := cmdexec.NewStubRunner()

	_, err := stub.LookPath("nonexistent-tool")
	if err == nil {
		t.Fatal("expected an error, got nil")
	}
	if !errors.Is(err, exec.ErrNotFound) {
		t.Errorf("expected exec.ErrNotFound, got %v", err)
	}
}

func TestStubRunner_Run_NoQueuedResults_ReturnsZeroResult(t *testing.T) {
	stub := cmdexec.NewStubRunner()

	got, err := stub.Run(context.Background(), "anything")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.ExitCode != 0 {
		t.Errorf("ExitCode: got %d, want 0", got.ExitCode)
	}
	if len(got.Stdout) != 0 {
		t.Errorf("Stdout: got %q, want empty", got.Stdout)
	}
	if len(got.Stderr) != 0 {
		t.Errorf("Stderr: got %q, want empty", got.Stderr)
	}
}

func TestStubRunner_RunIn_RecordsDirAndSudo(t *testing.T) {
	s := cmdexec.NewStubRunner()
	s.AddResult("sh", cmdexec.Result{ExitCode: 7})

	res, err := s.RunIn(context.Background(), cmdexec.RunOptions{Dir: "/repo", Sudo: true}, "sh", "-c", "true")
	if err != nil {
		t.Fatalf("RunIn returned error: %v", err)
	}

	if res.ExitCode != 7 {
		t.Errorf("ExitCode = %d, want 7 (queued result must be returned)", res.ExitCode)
	}

	if len(s.Calls) != 1 {
		t.Fatalf("expected 1 recorded call, got %d", len(s.Calls))
	}

	got := s.Calls[0]
	if got.Dir != "/repo" {
		t.Errorf("Call.Dir = %q, want %q", got.Dir, "/repo")
	}

	if !got.Sudo {
		t.Error("Call.Sudo = false, want true")
	}
}

func TestStubRunner_RunIn_SnapshotsStdin(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	input := []byte("private input")

	if _, err := stub.RunIn(
		context.Background(),
		cmdexec.RunOptions{Stdin: input},
		"private-command",
	); err != nil {
		t.Fatalf("RunIn returned error: %v", err)
	}

	input[0] = 'X'
	if len(stub.Calls) != 1 {
		t.Fatalf("expected 1 recorded call, got %d", len(stub.Calls))
	}
	if got, want := string(stub.Calls[0].Stdin), "private input"; got != want {
		t.Errorf("Call.Stdin = %q, want snapshot %q", got, want)
	}
}
