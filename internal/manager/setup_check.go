package manager

import (
	"errors"
	"fmt"
	"os/exec"
	"strings"
	"unicode"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/charmbracelet/x/ansi"
)

// SetupState describes the state reported by a setup entry's check command.
type SetupState int

const (
	// SetupApplied means the check reports that setup is already complete.
	SetupApplied SetupState = iota
	// SetupNeeded means setup is missing and should be run.
	SetupNeeded
	// SetupOutdated means setup exists but needs the run command to refresh it.
	SetupOutdated
	// SetupCheckFailed means the check result is indeterminate or could not run.
	SetupCheckFailed
)

// SetupCheckResult contains the typed state and any error that prevents a safe
// setup decision.
type SetupCheckResult struct {
	State SetupState
	Err   error
	// Diagnostic is sanitized stderr retained for status-mode exit 1/2 results.
	Diagnostic string
}

// setupCheckFailure keeps a safe, displayable message separate from the raw
// execution error while preserving the original cause for errors.Is/errors.As.
type setupCheckFailure struct {
	message string
	cause   error
}

func (e *setupCheckFailure) Error() string { return e.message }

func (e *setupCheckFailure) Unwrap() error { return e.cause }

// CheckSetup executes the current platform's setup check without elevation.
// An entry without a check for this OS does not apply and is treated as
// already applied.
func (m *Manager) CheckSetup(entry config.SubEntry) SetupCheckResult {
	command := entry.GetCheck(m.Platform.OS)
	if command == "" {
		return SetupCheckResult{State: SetupApplied}
	}

	name, args := shellCommand(m.Platform.OS, command)
	res, err := m.runner.RunIn(
		m.ctx,
		cmdexec.RunOptions{Dir: m.setupWorkDir()},
		name,
		args...,
	) //nolint:gosec // command from trusted config

	if contextErr := m.ctx.Err(); contextErr != nil {
		err = contextErr
	}

	return classifySetupCheck(entry.CheckMode, res, err)
}

// classifySetupCheck maps a check result to the configured check contract.
// Legacy exit-code checks retain their historical zero/nonzero behavior. In
// status mode, only process exits 0, 1, and 2 are meaningful; launch errors,
// cancellation, and all other exit codes are indeterminate failures.
func classifySetupCheck(mode string, res cmdexec.Result, err error) SetupCheckResult {
	if mode != config.CheckModeStatus {
		if commandSucceeded(res, err) {
			return SetupCheckResult{State: SetupApplied}
		}
		return SetupCheckResult{State: SetupNeeded}
	}

	var exitErr *exec.ExitError
	if err != nil && !errors.As(err, &exitErr) {
		return SetupCheckResult{
			State: SetupCheckFailed,
			Err:   setupCheckError(res, err),
		}
	}

	switch res.ExitCode {
	case 0:
		if err == nil {
			return SetupCheckResult{State: SetupApplied}
		}
	case 1:
		return SetupCheckResult{
			State:      SetupNeeded,
			Diagnostic: sanitizeSetupDiagnostic(string(res.Stderr)),
		}
	case 2:
		return SetupCheckResult{
			State:      SetupOutdated,
			Diagnostic: sanitizeSetupDiagnostic(string(res.Stderr)),
		}
	}

	return SetupCheckResult{
		State: SetupCheckFailed,
		Err:   setupCheckError(res, err),
	}
}

// sanitizeSetupDiagnostic removes terminal/control formatting, normalizes
// whitespace, and bounds a check diagnostic for display.
func sanitizeSetupDiagnostic(text string) string {
	text = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		if unicode.IsControl(r) || unicode.Is(unicode.Cf, r) {
			return -1
		}
		return r
	}, ansi.Strip(text))
	text = strings.Join(strings.Fields(text), " ")

	if text == "" {
		return ""
	}

	runes := []rune(text)
	if len(runes) > 240 {
		text = string(runes[:237]) + "..."
	}

	return text
}

// setupCheckError creates a bounded, single-line diagnostic for a failed
// status check. The original execution error is wrapped without exposing its
// unsanitized text in the display message.
func setupCheckError(res cmdexec.Result, err error) error {
	text := string(res.Stderr)
	if strings.TrimSpace(text) == "" && err != nil {
		text = err.Error()
	}

	text = sanitizeSetupDiagnostic(text)
	if text == "" {
		text = fmt.Sprintf("check command failed (exit %d)", res.ExitCode)
	}

	return &setupCheckFailure{message: text, cause: err}
}
