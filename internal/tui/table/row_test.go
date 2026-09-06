package table

import (
	"testing"

	"charm.land/bubbles/v2/table"
)

func TestPathStateString(t *testing.T) {
	tests := []struct {
		name  string
		state PathState
		want  string
	}{
		{"StateLoading", StateLoading, "Loading..."},
		{"StateReady", StateReady, "Ready"},
		{"StateAdopt", StateAdopt, "Adopt"},
		{"StateMissing", StateMissing, "Missing"},
		{"StateLinked", StateLinked, "Linked"},
		{"StateOutdated", StateOutdated, "Outdated"},
		{"StateModified", StateModified, "Modified"},
		{"unknown value", PathState(99), "Unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.state.String()
			if got != tt.want {
				t.Errorf("PathState(%d).String() = %q, want %q", tt.state, got, tt.want)
			}
		})
	}
}

func TestPathStateIota(t *testing.T) {
	// Verify iota ordering is stable — any reordering would be a breaking change.
	if StateLoading != 0 {
		t.Errorf("StateLoading should be 0, got %d", StateLoading)
	}
	if StateReady != 1 {
		t.Errorf("StateReady should be 1, got %d", StateReady)
	}
	if StateAdopt != 2 {
		t.Errorf("StateAdopt should be 2, got %d", StateAdopt)
	}
	if StateMissing != 3 {
		t.Errorf("StateMissing should be 3, got %d", StateMissing)
	}
	if StateLinked != 4 {
		t.Errorf("StateLinked should be 4, got %d", StateLinked)
	}
	if StateOutdated != 5 {
		t.Errorf("StateOutdated should be 5, got %d", StateOutdated)
	}
	if StateModified != 6 {
		t.Errorf("StateModified should be 6, got %d", StateModified)
	}
}

func TestRowFields(t *testing.T) {
	r := Row{
		Data:            table.Row{"nvim-config", "Linked", "", "/home/user/.config/nvim"},
		Level:           1,
		TreeChar:        "└─",
		IsExpanded:      false,
		AppName:         "nvim",
		SubName:         "nvim-config",
		SubIndex:        0,
		State:           StateLinked,
		StatusAttention: false,
		InfoAttention:   true,
		InfoState:       StateOutdated,
		BackupPath:      "/home/user/backup/nvim",
	}

	if r.Level != 1 {
		t.Errorf("Row.Level = %d, want 1", r.Level)
	}
	if r.TreeChar != "└─" {
		t.Errorf("Row.TreeChar = %q, want %q", r.TreeChar, "└─")
	}
	if r.IsExpanded {
		t.Error("Row.IsExpanded should be false")
	}
	if r.SubName != "nvim-config" {
		t.Errorf("Row.SubName = %q, want %q", r.SubName, "nvim-config")
	}
	if r.AppName != "nvim" {
		t.Errorf("Row.AppName = %q, want %q", r.AppName, "nvim")
	}
	if r.SubIndex != 0 {
		t.Errorf("Row.SubIndex = %d, want 0", r.SubIndex)
	}
	if r.State != StateLinked {
		t.Errorf("Row.State = %v, want StateLinked", r.State)
	}
	if r.StatusAttention {
		t.Error("Row.StatusAttention should be false")
	}
	if !r.InfoAttention {
		t.Error("Row.InfoAttention should be true")
	}
	if r.InfoState != StateOutdated {
		t.Errorf("Row.InfoState = %v, want StateOutdated", r.InfoState)
	}
	if r.BackupPath != "/home/user/backup/nvim" {
		t.Errorf("Row.BackupPath = %q, want %q", r.BackupPath, "/home/user/backup/nvim")
	}
	if len(r.Data) != 4 {
		t.Errorf("Row.Data length = %d, want 4", len(r.Data))
	}
	if r.Data[0] != "nvim-config" {
		t.Errorf("Row.Data[0] = %q, want %q", r.Data[0], "nvim-config")
	}
}

func TestPathState_String_SetupStates(t *testing.T) {
	if got, want := StateSetupOk.String(), "Set up"; got != want {
		t.Errorf("StateSetupOk.String() = %q, want %q", got, want)
	}

	if got, want := StateSetupNeeded.String(), "Needs setup"; got != want {
		t.Errorf("StateSetupNeeded.String() = %q, want %q", got, want)
	}
}

func TestPathState_Actionable(t *testing.T) {
	tests := []struct {
		state      PathState
		actionable bool
		severity   int
	}{
		{StateLoading, false, 0},
		{StateReady, true, 3},
		{StateAdopt, true, 3},
		{StateMissing, true, 3},
		{StateLinked, false, 0},
		{StateOutdated, true, 2},
		{StateModified, true, 1},
		{StateSetupOk, false, 0},
		{StateSetupNeeded, true, 3},
		{PathState(99), false, 0},
	}

	for _, tt := range tests {
		if got := tt.state.Actionable(); got != tt.actionable {
			t.Errorf("PathState(%d).Actionable() = %t, want %t", tt.state, got, tt.actionable)
		}
		if got := tt.state.Severity(); got != tt.severity {
			t.Errorf("PathState(%d).Severity() = %d, want %d", tt.state, got, tt.severity)
		}
	}
}

func TestUnavailableStateRequiresAttention(t *testing.T) {
	if StateUnavailable.String() != "Unavailable" || !StateUnavailable.Actionable() ||
		StateUnavailable.Severity() != 3 {
		t.Fatal("unavailable inspection must not be healthy or remain loading")
	}
}

// The existing states are declared with iota; appending must not renumber them,
// or persisted/compared state values would silently shift meaning.
func TestPathState_ExistingValuesUnchanged(t *testing.T) {
	if StateLoading != 0 {
		t.Errorf("StateLoading = %d, want 0", StateLoading)
	}

	if StateModified != 6 {
		t.Errorf("StateModified = %d, want 6; new states must be appended, not inserted", StateModified)
	}
}

func TestRowAppLevel(t *testing.T) {
	// Application-level row (Level=0, SubIndex=-1)
	r := Row{
		Data:      table.Row{"nvim", "Linked", "1/1", ""},
		Level:     0,
		TreeChar:  "▶ ",
		AppName:   "nvim",
		SubIndex:  -1,
		State:     StateLinked,
		InfoState: StateLinked,
	}

	if r.Level != 0 {
		t.Errorf("app row Level = %d, want 0", r.Level)
	}
	if r.SubIndex != -1 {
		t.Errorf("app row SubIndex = %d, want -1", r.SubIndex)
	}
	if r.BackupPath != "" {
		t.Errorf("app row BackupPath should be empty, got %q", r.BackupPath)
	}
}
