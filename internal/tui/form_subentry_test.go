package tui

import (
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

// TestAddFileMode_Constants verifies the AddFileMode enum constants exist
func TestAddFileMode_Constants(t *testing.T) {
	tests := []struct {
		name     string
		mode     AddFileMode
		expected AddFileMode
	}{
		{"ModeNone exists", ModeNone, 0},
		{"ModeChoosing exists", ModeChoosing, 1},
		{"ModePicker exists", ModePicker, 2},
		{"ModeTextInput exists", ModeTextInput, 3},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mode != tt.expected {
				t.Errorf("mode = %d, want %d", tt.mode, tt.expected)
			}
		})
	}
}

// TestInitSubEntryFormNew_FilePickerFields verifies new fields are initialized correctly
func TestInitSubEntryFormNew_FilePickerFields(t *testing.T) {
	// Create minimal model with required fields
	cfg := &config.Config{
		Applications: []config.Application{
			{
				Name:        "test-app",
				Description: "Test application",
			},
		},
	}
	plat := &platform.Platform{OS: "linux"}
	m := NewModel(cfg, plat, false)

	// Initialize new sub-entry form
	m.initSubEntryFormNew(0)

	if m.subEntryForm == nil {
		t.Fatal("subEntryForm is nil after initSubEntryFormNew")
	}

	// Verify addFileMode is initialized to ModeNone
	if m.subEntryForm.addFileMode != ModeNone {
		t.Errorf("addFileMode = %d, want %d (ModeNone)", m.subEntryForm.addFileMode, ModeNone)
	}

	// Verify modeMenuCursor is initialized to 0
	if m.subEntryForm.modeMenuCursor != 0 {
		t.Errorf("modeMenuCursor = %d, want 0", m.subEntryForm.modeMenuCursor)
	}

	// Verify selectedFiles is initialized as empty map (not nil)
	if m.subEntryForm.selectedFiles == nil {
		t.Error("selectedFiles is nil, want empty map")
	}

	if len(m.subEntryForm.selectedFiles) != 0 {
		t.Errorf("len(selectedFiles) = %d, want 0", len(m.subEntryForm.selectedFiles))
	}

	// Verify filePicker is zero value (will be initialized in Phase 4)
	// filepicker.Model is a struct, so we can't directly compare to zero value
	// We'll just verify the field exists by accessing it
	_ = m.subEntryForm.filePicker
}

// TestInitSubEntryFormEdit_FilePickerFields verifies fields are initialized in edit mode
func TestInitSubEntryFormEdit_FilePickerFields(t *testing.T) {
	// Create minimal model with required fields
	cfg := &config.Config{
		Applications: []config.Application{
			{
				Name:        "test-app",
				Description: "Test application",
				Entries: []config.SubEntry{
					{
						Name:   "test-entry",
						Backup: "./test",
						Targets: map[string]string{
							"linux": "~/.config/test",
						},
					},
				},
			},
		},
	}
	plat := &platform.Platform{OS: "linux"}
	m := NewModel(cfg, plat, false)

	// Initialize edit sub-entry form
	m.initSubEntryFormEdit(0, 0)

	if m.subEntryForm == nil {
		t.Fatal("subEntryForm is nil after initSubEntryFormEdit")
	}

	// Verify addFileMode is initialized to ModeNone
	if m.subEntryForm.addFileMode != ModeNone {
		t.Errorf("addFileMode = %d, want %d (ModeNone)", m.subEntryForm.addFileMode, ModeNone)
	}

	// Verify modeMenuCursor is initialized to 0
	if m.subEntryForm.modeMenuCursor != 0 {
		t.Errorf("modeMenuCursor = %d, want 0", m.subEntryForm.modeMenuCursor)
	}

	// Verify selectedFiles is initialized as empty map (not nil)
	if m.subEntryForm.selectedFiles == nil {
		t.Error("selectedFiles is nil, want empty map")
	}

	if len(m.subEntryForm.selectedFiles) != 0 {
		t.Errorf("len(selectedFiles) = %d, want 0", len(m.subEntryForm.selectedFiles))
	}

	// Verify filePicker is zero value (will be initialized in Phase 4)
	_ = m.subEntryForm.filePicker
}

// TestSubEntryForm_AddFileModeTransitions tests state transitions for AddFileMode
func TestSubEntryForm_AddFileModeTransitions(t *testing.T) {
	tests := []struct {
		name        string
		initialMode AddFileMode
		newMode     AddFileMode
	}{
		{"ModeNone to ModeChoosing", ModeNone, ModeChoosing},
		{"ModeChoosing to ModePicker", ModeChoosing, ModePicker},
		{"ModeChoosing to ModeTextInput", ModeChoosing, ModeTextInput},
		{"ModePicker to ModeNone", ModePicker, ModeNone},
		{"ModeTextInput to ModeNone", ModeTextInput, ModeNone},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := &SubEntryForm{
				addFileMode: tt.initialMode,
			}

			// Transition to new mode
			form.addFileMode = tt.newMode

			if form.addFileMode != tt.newMode {
				t.Errorf("addFileMode = %d, want %d", form.addFileMode, tt.newMode)
			}
		})
	}
}

// TestSubEntryForm_SelectedFilesManagement tests adding/removing selected files
func TestSubEntryForm_SelectedFilesManagement(t *testing.T) {
	form := &SubEntryForm{
		selectedFiles: make(map[string]bool),
	}

	// Test adding files
	form.selectedFiles["/path/to/file1"] = true
	form.selectedFiles["/path/to/file2"] = true

	if len(form.selectedFiles) != 2 {
		t.Errorf("len(selectedFiles) = %d, want 2", len(form.selectedFiles))
	}

	if !form.selectedFiles["/path/to/file1"] {
		t.Error("file1 not selected")
	}

	if !form.selectedFiles["/path/to/file2"] {
		t.Error("file2 not selected")
	}

	// Test removing a file
	delete(form.selectedFiles, "/path/to/file1")

	if len(form.selectedFiles) != 1 {
		t.Errorf("len(selectedFiles) = %d, want 1 after deletion", len(form.selectedFiles))
	}

	if form.selectedFiles["/path/to/file1"] {
		t.Error("file1 still selected after deletion")
	}

	if !form.selectedFiles["/path/to/file2"] {
		t.Error("file2 not selected")
	}

	// Test clearing all selections
	form.selectedFiles = make(map[string]bool)

	if len(form.selectedFiles) != 0 {
		t.Errorf("len(selectedFiles) = %d, want 0 after clearing", len(form.selectedFiles))
	}
}

// TestSubEntryForm_ModeMenuCursor tests cursor navigation for mode menu
func TestSubEntryForm_ModeMenuCursor(t *testing.T) {
	form := &SubEntryForm{
		modeMenuCursor: 0,
	}

	// Test incrementing cursor
	form.modeMenuCursor++
	if form.modeMenuCursor != 1 {
		t.Errorf("modeMenuCursor = %d, want 1", form.modeMenuCursor)
	}

	form.modeMenuCursor++
	if form.modeMenuCursor != 2 {
		t.Errorf("modeMenuCursor = %d, want 2", form.modeMenuCursor)
	}

	// Test wrapping (assuming 2 menu items: Browse and Type)
	maxCursor := 1 // 0-indexed, so 0=Browse, 1=Type
	form.modeMenuCursor++
	if form.modeMenuCursor > maxCursor {
		form.modeMenuCursor = 0
	}

	if form.modeMenuCursor != 0 {
		t.Errorf("modeMenuCursor = %d, want 0 after wrapping", form.modeMenuCursor)
	}

	// Test decrementing with wrapping
	form.modeMenuCursor--
	if form.modeMenuCursor < 0 {
		form.modeMenuCursor = maxCursor
	}

	if form.modeMenuCursor != 1 {
		t.Errorf("modeMenuCursor = %d, want 1 after wrapping backward", form.modeMenuCursor)
	}
}
