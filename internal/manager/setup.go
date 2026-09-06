package manager

import (
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/platform"
)

// setupRunError reports a setup entry whose run command failed.
//
// The exit code alone does not identify the failure. When the command never
// launched at all — the shell is missing, the working directory does not
// exist, the context was already canceled — OsRunner returns a non-nil error
// together with ExitCode 0 and no captured output. Formatting only the exit
// code and stderr then produced "command failed (exit 0): ", which reads as
// success and discarded the error, the one thing that explained what went
// wrong. reason therefore prefers stderr and falls back to that error, and
// Unwrap keeps it reachable through errors.Is/errors.As (context.Canceled in
// particular).
type setupRunError struct {
	err      error
	app      string
	entry    string
	reason   string
	exitCode int
}

func (e *setupRunError) Error() string {
	msg := fmt.Sprintf("setup %s/%s: command failed (exit %d)", e.app, e.entry, e.exitCode)

	// No stderr and no error to report: stop after the exit code rather than
	// trailing a bare ": ".
	if e.reason != "" {
		msg += ": " + e.reason
	}

	return msg
}

// Unwrap exposes the underlying execution error, so a caller can still detect
// e.g. context.Canceled behind the formatted message.
func (e *setupRunError) Unwrap() error { return e.err }

// newSetupRunError builds a setupRunError from the result of a run command.
func newSetupRunError(appName, entryName string, res cmdexec.Result, err error) *setupRunError {
	reason := strings.TrimSpace(string(res.Stderr))
	if reason == "" && err != nil {
		reason = err.Error()
	}

	return &setupRunError{
		app:      appName,
		entry:    entryName,
		reason:   reason,
		exitCode: res.ExitCode,
		err:      err,
	}
}

// commandSucceeded reports whether a command completed with a zero exit status.
//
// Both the error and the exit code must be consulted: OsRunner reports a
// non-zero exit as an *exec.ExitError AND sets ExitCode, while StubRunner
// returns a nil error and signals failure through ExitCode alone. Checking only
// the error would make every one of these paths untestable with the stub.
func commandSucceeded(res cmdexec.Result, err error) bool {
	return err == nil && res.ExitCode == 0
}

// shellCommand returns the shell and arguments used to execute a raw command
// string on the given platform. This mirrors runCustomCommand in
// internal/packages/install.go.
//
// SECURITY NOTE: this intentionally executes arbitrary shell commands from the
// user's configuration file. Users should only use configurations they trust.
func shellCommand(osType, command string) (string, []string) {
	if osType == platform.OSWindows {
		return "powershell", []string{"-Command", command}
	}

	return "sh", []string{"-c", command}
}

// setupWorkDir returns the fully expanded configurations repo root. Setup
// commands run from here so that repo-relative script paths resolve.
func (m *Manager) setupWorkDir() string {
	return config.ExpandPathWithTemplate(m.Config.BackupRoot, m.Platform.EnvVars, m.templateEngine)
}

// RunSetup executes a single setup sub-entry: it runs the entry's check and,
// if the check fails, the setup command, then re-runs the check to confirm the
// effect landed. It is the exported entry point for callers outside this
// package (the TUI's restore action); Restore drives runSetupEntry directly.
//
// The semantics are exactly runSetupEntry's — dry run still executes the check
// and never the run command, and a passing check is a no-op.
//
// The check and the run are real subprocesses, and an entry with sudo: true may
// prompt for a password on the terminal. A caller that owns the terminal (the
// bubbletea TUI) must therefore release it for the duration of this call; see
// internal/tui/setup_run.go.
func (m *Manager) RunSetup(appName string, e config.SubEntry) error {
	return m.runSetupEntry(appName, e)
}

// runSetupEntry executes a single setup sub-entry:
//
//  1. no run command for this OS  -> skip
//  2. check passes                -> skip (already set up)
//  3. dry run                     -> report, never execute the run command
//  4. execute the run command     -> non-zero exit is an error
//  5. re-run the check            -> still failing is an error (silent failure)
func (m *Manager) runSetupEntry(appName string, e config.SubEntry) error {
	command := e.GetRun(m.Platform.OS)
	if command == "" {
		m.logger.Debug("skipping setup entry",
			slog.String("app", appName),
			slog.String("entry", e.Name),
			slog.String("os", m.Platform.OS),
			slog.String("reason", "no run command for OS"))

		return nil
	}

	// Validation guarantees this, but a hand-built config could bypass it. An
	// empty check would run as `sh -c ""`, exit 0, and silently suppress the
	// setup forever — so fail loudly instead.
	check := e.GetCheck(m.Platform.OS)
	if check == "" {
		return fmt.Errorf("setup %s/%s: run command for %s has no matching check command",
			appName, e.Name, m.Platform.OS)
	}

	checkResult := m.CheckSetup(e)
	if checkResult.Err != nil {
		return fmt.Errorf("setup %s/%s: %w", appName, e.Name, checkResult.Err)
	}

	if checkResult.State == SetupApplied {
		m.logger.Info("setup already applied",
			slog.String("app", appName),
			slog.String("entry", e.Name))

		return nil
	}

	if m.DryRun {
		m.logger.Info("would run setup",
			slog.String("app", appName),
			slog.String("entry", e.Name),
			slog.String("command", command))

		return nil
	}

	name, args := shellCommand(m.Platform.OS, command)

	res, err := m.runner.RunIn(m.ctx, //nolint:gosec // command from trusted config
		cmdexec.RunOptions{Dir: m.setupWorkDir(), Sudo: e.Sudo}, name, args...)

	if !commandSucceeded(res, err) {
		return newSetupRunError(appName, e.Name, res, err)
	}

	// Confirm the command actually achieved what it claimed. A script can exit 0
	// without doing its job; without this, that failure would be invisible.
	postCheck := m.CheckSetup(e)
	if postCheck.Err != nil {
		return fmt.Errorf("setup %s/%s: %w", appName, e.Name, postCheck.Err)
	}
	if postCheck.State != SetupApplied {
		message := fmt.Sprintf("setup %s/%s: command succeeded but check still fails",
			appName, e.Name)
		if postCheck.Diagnostic != "" {
			message += ": " + postCheck.Diagnostic
		}
		return errors.New(message)
	}

	m.logger.Info("setup applied",
		slog.String("app", appName),
		slog.String("entry", e.Name))

	return nil
}
