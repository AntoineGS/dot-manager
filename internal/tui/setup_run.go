package tui

import (
	"errors"
	"fmt"
	"io"

	tea "charm.land/bubbletea/v2"
)

// Messages shown for a setup entry in the results popup.
const (
	setupResultOK       = "Setup complete: check passes"
	setupResultDryRun   = "[DRY RUN] check ran; setup command not executed"
	setupResultNoRunner = "Failed: no manager available to run the setup command"
)

// setupRunItem is a setup sub-entry queued to run, together with where its row
// lives so the row's state can be refreshed once the run finishes.
type setupRunItem struct {
	name   string // label for the results popup, following the caller's convention
	sub    SubEntryItem
	appIdx int
	subIdx int
}

// setupRunMsg reports the outcome of one queued setup entry.
type setupRunMsg struct {
	message string
	item    setupRunItem
	success bool
}

// setupExec adapts a setup sub-entry to bubbletea's ExecCommand interface.
//
// A setup entry's check and run are real subprocesses, and an entry with
// sudo: true prompts for a password on the terminal — which bubbletea owns
// (raw mode, alt screen, its own input reader). Run it from an ordinary tea.Cmd
// and the password prompt is invisible and the keystrokes answering it are
// swallowed by the TUI's input reader.
//
// Package installation already solves this: installNextPackage dispatches
// through tea.Exec, which releases the terminal for the duration of the command
// and recaptures it afterwards. Setup entries go through the same door — tea.Exec
// accepts any ExecCommand, not just an *exec.Cmd, so the whole check → run →
// re-check sequence runs inside a single terminal handover and stays in the
// manager where it belongs.
//
// The manager's runner captures the commands' stdout/stderr and reports them
// through the returned error (which the results popup shows), so the writers
// handed over by bubbletea are used only to announce what is about to run.
// sudo writes its prompt to /dev/tty directly, so it is visible and answerable
// once bubbletea has let go of the terminal.
type setupExec struct {
	stdout  io.Writer
	message string
	model   Model
	item    setupRunItem
	success bool
}

// SetStdin satisfies tea.ExecCommand. The setup commands take no input; sudo
// reads its password straight from /dev/tty.
func (s *setupExec) SetStdin(io.Reader) {}

// SetStdout satisfies tea.ExecCommand, capturing the terminal bubbletea hands
// over so the run can announce itself.
func (s *setupExec) SetStdout(w io.Writer) { s.stdout = w }

// SetStderr satisfies tea.ExecCommand. The command's own stderr is captured by
// the manager's runner and surfaced in the failure message.
func (s *setupExec) SetStderr(io.Writer) {}

// Run executes the setup entry while bubbletea is not holding the terminal.
// The outcome is recorded on the receiver: tea.Exec's callback only gets an
// error, and the results popup wants the human-readable message too.
func (s *setupExec) Run() error {
	if s.stdout != nil {
		_, _ = fmt.Fprintf(s.stdout, "\nRunning setup: %s/%s\n", s.item.sub.AppName, s.item.sub.SubEntry.Name)
	}

	s.success, s.message = s.model.runSetupForItem(s.item.sub)
	if !s.success {
		return errors.New(s.message)
	}

	return nil
}

// runSetupForItem executes one setup sub-entry through the manager, which owns
// the check → run → re-check state machine (including the dry-run and sudo
// rules).
//
// This shells out. Callers must be on a goroutine that does not hold the
// terminal: go through startSetupRun/runNextSetup, which wrap this in tea.Exec.
func (m Model) runSetupForItem(item SubEntryItem) (bool, string) {
	if m.Manager == nil {
		return false, setupResultNoRunner
	}

	if err := m.Manager.RunSetup(item.AppName, item.SubEntry); err != nil {
		return false, fmt.Sprintf("Failed: %v", err)
	}

	// The manager runs the check but never the run command in dry-run mode, so
	// say so rather than claiming the system was changed.
	if m.Manager.DryRun {
		return true, setupResultDryRun
	}

	return true, setupResultOK
}

// startSetupRun queues setup entries and starts the first one. Entries run one
// at a time: each holds the terminal while it runs, so they must never overlap.
// batch marks the run as part of a multi-select batch operation, whose
// selections are cleared when the queue drains.
//
// A run already in flight wins: the tea.Exec dispatched for it has not reached
// the event loop yet, so a second keypress (double-tapping `r`) would queue the
// same entry again and report it twice.
func (m *Model) startSetupRun(items []setupRunItem, batch bool) tea.Cmd {
	if len(m.pendingSetups) > 0 {
		return nil
	}

	m.pendingSetups = items
	m.currentSetupIndex = 0
	m.setupBatch = batch

	return m.runNextSetup()
}

// runNextSetup dispatches the setup entry at the head of the queue through
// tea.Exec, handing the terminal to it. Returns nil when the queue is drained.
func (m Model) runNextSetup() tea.Cmd {
	if m.currentSetupIndex >= len(m.pendingSetups) {
		return nil
	}

	item := m.pendingSetups[m.currentSetupIndex]
	ex := &setupExec{model: m, item: item}

	return tea.Exec(ex, func(err error) tea.Msg {
		return setupRunResult(ex, err)
	})
}

// setupRunResult turns what tea.Exec reports into the message the results popup
// shows.
//
// err covers two very different failures: the setup itself failing (Run already
// recorded the details on ex) and bubbletea failing to hand the terminal over
// (in which case Run never ran, so ex holds nothing at all). Prefer the recorded
// message, but never report a failure with nothing to say — an empty row in the
// results popup discards the only account of what went wrong.
func setupRunResult(ex *setupExec, err error) setupRunMsg {
	success, message := ex.success, ex.message

	if err != nil {
		success = false

		// Run recorded nothing, so the handover itself failed: err is the only
		// account of what happened. Anything Run did record is more specific than
		// "the terminal could not be released", so keep it.
		if message == "" {
			message = fmt.Sprintf("Failed: %v", err)
		}
	}

	return setupRunMsg{item: ex.item, success: success, message: message}
}

// handleSetupRunResult records the outcome of one setup entry, refreshes its
// row, and either starts the next queued entry or finishes the run.
func (m Model) handleSetupRunResult(msg setupRunMsg) (tea.Model, tea.Cmd) {
	m.results = append(m.results, ResultItem{
		Name:    msg.item.name,
		Success: msg.success,
		Message: msg.message,
	})

	if m.setupBatch {
		if msg.success {
			m.batchSuccessCount++
		} else {
			m.batchFailCount++
		}
	}

	// A run that exited 0 is not proof the system changed — the check is the
	// source of truth, and the manager already re-ran it. Park the row at
	// StateLoading and let the async resolver run the check once more rather
	// than painting a state this goroutine cannot verify. (The synchronous
	// detectSubEntryState must not shell out, which is why it leaves setup
	// entries at StateLoading in the first place.)
	if msg.item.appIdx >= 0 && msg.item.appIdx < len(m.Applications) &&
		msg.item.subIdx >= 0 && msg.item.subIdx < len(m.Applications[msg.item.appIdx].SubItems) {
		m.Applications[msg.item.appIdx].SubItems[msg.item.subIdx].State = StateLoading
		m.Applications[msg.item.appIdx].SubItems[msg.item.subIdx].CheckError = ""
	}

	m.currentSetupIndex++
	if m.currentSetupIndex < len(m.pendingSetups) {
		return m, m.runNextSetup()
	}

	batch := m.setupBatch

	m.pendingSetups = nil
	m.currentSetupIndex = 0
	m.setupBatch = false

	m.processing = false
	if batch {
		m.completedOperation = m.Operation
	} else {
		m.completedOperation = OpList
	}
	m.Screen = ScreenResults
	m.Operation = OpList

	if batch {
		m.clearSelections()
		m.clearTransientSummarySelection()
	}

	m.rebuildTable()

	m.showingResults = true
	m.resultsScrollOffset = 0

	return m, m.dispatchLoadingSubEntryStates()
}

// collectAppSetupItems returns the setup entries of one application, labeled
// the way single-application restore labels its results (by entry name).
func (m Model) collectAppSetupItems(appIdx int) []setupRunItem {
	var items []setupRunItem

	for subIdx, sub := range m.Applications[appIdx].SubItems {
		if !sub.SubEntry.IsSetup() {
			continue
		}

		items = append(items, setupRunItem{
			appIdx: appIdx,
			subIdx: subIdx,
			name:   sub.SubEntry.Name,
			sub:    sub,
		})
	}

	return items
}
