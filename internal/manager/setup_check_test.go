package manager

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"testing"
	"unicode"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/platform"
)

func TestClassifySetupCheck(t *testing.T) {
	for _, tt := range []struct {
		name, mode string
		code       int
		err        error
		want       SetupState
	}{
		{"default success", "", 0, nil, SetupApplied},
		{"default nonzero", "", 3, nil, SetupNeeded},
		{"explicit legacy", "exit-code", 2, nil, SetupNeeded},
		{"legacy launch failure", "", 0, exec.ErrNotFound, SetupNeeded},
		{"status success", "status", 0, nil, SetupApplied},
		{"status missing", "status", 1, nil, SetupNeeded},
		{"status outdated", "status", 2, nil, SetupOutdated},
		{"status failed", "status", 3, nil, SetupCheckFailed},
		{"unexpected exit", "status", 255, nil, SetupCheckFailed},
		{"killed", "status", -1, nil, SetupCheckFailed},
		{"launch failed", "status", 0, exec.ErrNotFound, SetupCheckFailed},
		{"canceled", "status", 2, context.Canceled, SetupCheckFailed},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := classifySetupCheck(tt.mode, cmdexec.Result{ExitCode: tt.code}, tt.err)
			if got.State != tt.want {
				t.Fatalf("state = %v, want %v", got.State, tt.want)
			}
			if (got.Err != nil) != (tt.want == SetupCheckFailed) {
				t.Fatalf("error = %v", got.Err)
			}
		})
	}
}

func TestClassifySetupCheckAcceptsExitErrors(t *testing.T) {
	if _, err := exec.LookPath("sh"); err != nil {
		t.Skip("sh is unavailable")
	}

	for _, tt := range []struct {
		name, command, wantDiagnostic string
		want                          SetupState
	}{
		{
			name:           "outdated",
			command:        "printf 'refresh available' >&2; exit 2",
			want:           SetupOutdated,
			wantDiagnostic: "refresh available",
		},
		{
			name:           "missing",
			command:        "printf 'not installed' >&2; exit 1",
			want:           SetupNeeded,
			wantDiagnostic: "not installed",
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			res, err := (cmdexec.OsRunner{}).RunIn(
				context.Background(), cmdexec.RunOptions{}, "sh", "-c", tt.command,
			)
			if err == nil {
				t.Fatal("RunIn returned nil error for a non-zero exit")
			}

			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) {
				t.Fatalf("RunIn error = %T %v, want *exec.ExitError", err, err)
			}

			got := classifySetupCheck(config.CheckModeStatus, res, err)
			if got.State != tt.want {
				t.Fatalf("state = %v, want %v", got.State, tt.want)
			}
			if got.Err != nil {
				t.Fatalf("error = %v, want nil", got.Err)
			}
			if got.Diagnostic != tt.wantDiagnostic {
				t.Fatalf("diagnostic = %q, want %q", got.Diagnostic, tt.wantDiagnostic)
			}
		})
	}
}

func TestClassifySetupCheckDiagnosticIsSanitizedAndBounded(t *testing.T) {
	stderr := "\x1b]0;status\a\x1b[31mremote\r\nunavailable\x1b[0m\u0085\u202e" + strings.Repeat(" detail", 80)
	got := classifySetupCheck(
		config.CheckModeStatus,
		cmdexec.Result{ExitCode: 1, Stderr: []byte(stderr)},
		nil,
	)

	if got.State != SetupNeeded {
		t.Fatalf("state = %v, want %v", got.State, SetupNeeded)
	}
	if got.Err != nil {
		t.Fatalf("error = %v, want nil", got.Err)
	}
	if !strings.Contains(got.Diagnostic, "remote unavailable") {
		t.Fatalf("diagnostic = %q, want stderr text", got.Diagnostic)
	}
	if strings.ContainsAny(got.Diagnostic, "\r\n") {
		t.Fatalf("diagnostic contains a line break: %q", got.Diagnostic)
	}
	for _, unwanted := range []string{"\x1b", "\a", "\u0085", "\u202e"} {
		if strings.Contains(got.Diagnostic, unwanted) {
			t.Fatalf("diagnostic contains unsanitized %q: %q", unwanted, got.Diagnostic)
		}
	}
	for _, r := range got.Diagnostic {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			t.Fatalf("diagnostic contains control or format rune %U: %q", r, got.Diagnostic)
		}
	}
	if len([]rune(got.Diagnostic)) > 240 {
		t.Fatalf("diagnostic has %d runes, want at most 240", len([]rune(got.Diagnostic)))
	}
	if !strings.HasSuffix(got.Diagnostic, "...") {
		t.Fatalf("diagnostic = %q, want truncation suffix", got.Diagnostic)
	}
}

func TestSetupCheckErrorSanitizesDiagnosticsAndWrapsCause(t *testing.T) {
	cause := context.Canceled
	stderr := "\x1b]0;remote check\a\x1b[31mremote\r\nunavailable\x1b[0m\u0085\u202e" + strings.Repeat(" detail", 80)

	err := setupCheckError(cmdexec.Result{ExitCode: 3, Stderr: []byte(stderr)}, cause)
	if err == nil {
		t.Fatal("setupCheckError returned nil")
	}
	if !errors.Is(err, cause) {
		t.Fatalf("error does not wrap cause: %v", err)
	}

	message := err.Error()
	if !strings.Contains(message, "remote unavailable") {
		t.Fatalf("message = %q, want stderr diagnostic", message)
	}
	if strings.ContainsAny(message, "\r\n") {
		t.Fatalf("message contains a line break: %q", message)
	}
	for _, unwanted := range []string{"\x1b", "\a", "\u0085", "\u202e"} {
		if strings.Contains(message, unwanted) {
			t.Fatalf("message contains unsanitized %q: %q", unwanted, message)
		}
	}
	for _, r := range message {
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			t.Fatalf("message contains control or format rune %U: %q", r, message)
		}
	}
	if len([]rune(message)) > 240 {
		t.Fatalf("message has %d runes, want at most 240", len([]rune(message)))
	}
	if !strings.HasSuffix(message, "...") {
		t.Fatalf("long message = %q, want truncation suffix", message)
	}
}

func TestSetupCheckErrorUsesFixedFallback(t *testing.T) {
	err := setupCheckError(cmdexec.Result{ExitCode: 7}, nil)
	if got, want := err.Error(), "check command failed (exit 7)"; got != want {
		t.Fatalf("error = %q, want %q", got, want)
	}
}

func TestSetupCheckErrorSanitizesExecutionErrorWhenStderrIsEmpty(t *testing.T) {
	cause := errors.New("\x1b[31mremote\r\nunavailable\x1b[0m\u202e")
	err := setupCheckError(cmdexec.Result{ExitCode: 3}, cause)

	if !errors.Is(err, cause) {
		t.Fatalf("error does not wrap execution cause: %v", err)
	}
	if got, want := err.Error(), "remote unavailable"; got != want {
		t.Fatalf("error = %q, want sanitized execution diagnostic %q", got, want)
	}
}

func TestCheckSetupCanceledContextIsAnError(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	stub.AddResult("sh", cmdexec.Result{ExitCode: 2})

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	m := newSetupManager(stub, false).WithContext(ctx)

	entry := setupEntry()
	entry.CheckMode = config.CheckModeStatus
	got := m.CheckSetup(entry)
	if got.State != SetupCheckFailed {
		t.Fatalf("state = %v, want %v", got.State, SetupCheckFailed)
	}
	if !errors.Is(got.Err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", got.Err)
	}
}

func TestCheckSetupWindowsUsesPowerShellAndNeverSudoChecks(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	stub.AddResult("powershell", cmdexec.Result{ExitCode: 1})
	stub.AddResult("powershell", cmdexec.Result{ExitCode: 0})
	stub.AddResult("powershell", cmdexec.Result{ExitCode: 0})

	cfg := &config.Config{Version: 3, BackupRoot: "/repo"}
	plat := &platform.Platform{OS: platform.OSWindows, EnvVars: map[string]string{}}
	m := New(cfg, plat).WithRunner(stub)
	e := config.SubEntry{
		Name:      "enable-service",
		Check:     map[string]string{platform.OSWindows: "Get-Service tool"},
		Run:       map[string]string{platform.OSWindows: "Start-Service tool"},
		CheckMode: config.CheckModeStatus,
		Sudo:      true,
	}

	if err := m.RunSetup("tool", e); err != nil {
		t.Fatalf("RunSetup returned error: %v", err)
	}

	if len(stub.Calls) != 3 {
		t.Fatalf("calls = %+v, want check/run/check", stub.Calls)
	}
	for _, index := range []int{0, 2} {
		call := stub.Calls[index]
		if call.Name != "powershell" {
			t.Errorf("call %d name = %q, want powershell", index, call.Name)
		}
		if len(call.Args) != 2 || call.Args[0] != "-Command" {
			t.Errorf("call %d args = %#v, want -Command plus command", index, call.Args)
		}
		if call.Sudo {
			t.Errorf("call %d unexpectedly used sudo", index)
		}
	}
	if !stub.Calls[1].Sudo {
		t.Error("run command did not use sudo")
	}
}
