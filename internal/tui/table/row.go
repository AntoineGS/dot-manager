// Package table provides the TableRow type and PathState type used by the
// manage-screen table in the TUI.
package table

import "charm.land/bubbles/v2/table"

// PathState represents the state of a path item for restore operations.
// This is the canonical definition; the root tui package re-exports it as a
// type alias to avoid a circular import while keeping all TUI code compatible.
type PathState int

// Path states for restore operations.
const (
	// StateLoading indicates state is still being detected.
	StateLoading PathState = iota
	// StateReady indicates backup exists and is ready to restore.
	StateReady
	// StateAdopt indicates no backup but target exists (will adopt).
	StateAdopt
	// StateMissing indicates neither backup nor target exists.
	StateMissing
	// StateLinked indicates already symlinked.
	StateLinked
	// StateOutdated indicates linked but a template source changed since last
	// render, or a setup entry needs an update.
	StateOutdated
	// StateModified indicates linked but rendered file has user edits.
	StateModified
	// StateSetupOk indicates a setup entry whose check command passes.
	StateSetupOk
	// StateSetupNeeded indicates a setup entry whose check reports setup is needed.
	StateSetupNeeded
	// StateUnavailable indicates status could not be inspected safely.
	StateUnavailable
	// StateCheckFailed indicates a setup entry whose check could not determine
	// the current state.
	StateCheckFailed
)

// stateLinkedLabel is the display label for StateLinked.
const stateLinkedLabel = "Linked"

// String returns the human-readable name of a PathState.
func (s PathState) String() string {
	switch s {
	case StateLoading:
		return "Loading..."
	case StateReady:
		return "Ready"
	case StateAdopt:
		return "Adopt"
	case StateMissing:
		return "Missing"
	case StateLinked:
		return stateLinkedLabel
	case StateOutdated:
		return "Outdated"
	case StateModified:
		return "Modified"
	case StateSetupOk:
		return "Set up"
	case StateSetupNeeded:
		return "Needs setup"
	case StateUnavailable:
		return "Unavailable"
	case StateCheckFailed:
		return "Check failed"
	}

	return "Unknown"
}

// Severity returns the attention level used by the action filter. A positive
// value means the state requires an action; larger values take priority when
// choosing an application's highest-severity entry state.
func (s PathState) Severity() int {
	switch s {
	case StateMissing, StateReady, StateAdopt, StateSetupNeeded, StateCheckFailed:
		return 3
	case StateOutdated:
		return 2
	case StateModified:
		return 1
	case StateLoading, StateLinked, StateSetupOk:
		return 0
	case StateUnavailable:
		return 3
	}

	return 0
}

// Actionable reports whether this state is included by the TUI action filter
// once status checks have completed.
func (s PathState) Actionable() bool {
	return s.Severity() > 0
}

// Row wraps table.Row with hierarchy metadata used for rendering and
// selection logic in the manage screen.
type Row struct {
	Data            table.Row // Actual display data [name, status, info, path]
	Level           int       // 0 = application, 1 = sub-entry
	TreeChar        string    // "▶ ", "▼ ", "├─", "└─"
	IsExpanded      bool
	AppName         string    // Application name; the stable identity for lookups in m.Applications
	SubName         string    // Sub-entry name ("" on application rows); identity for selection lookups
	SubIndex        int       // Real index into the app's SubItems; -1 for application rows
	State           PathState // For badge rendering
	StatusAttention bool      // Status column needs attention
	InfoAttention   bool      // Info column needs attention
	InfoState       PathState // Highest-severity sub-entry state (app rows only)
	BackupPath      string    // Backup/source path for sub-entries (empty for app rows)
}
