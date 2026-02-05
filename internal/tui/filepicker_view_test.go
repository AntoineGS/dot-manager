package tui

import (
	"strings"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

// TestRenderFilePicker_Header tests that the header shows directory path
func TestRenderFilePicker_Header(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{Name: "test-app"},
		},
	}
	plat := &platform.Platform{OS: "linux"}
	m := NewModel(cfg, plat, false)
	m.initSubEntryFormNew(0)

	// Set up file picker mode
	m.subEntryForm.addFileMode = ModePicker
	m.subEntryForm.filePicker.CurrentDirectory = testNvimDir

	view := m.viewFilePicker()

	// Verify header contains directory path
	if !strings.Contains(view, testNvimDir) {
		t.Error("header should contain current directory path")
	}

	// Verify "Select Files" title
	if !strings.Contains(view, "Select Files") {
		t.Error("header should contain 'Select Files' title")
	}
}

// TestRenderFilePicker_SelectionCount tests selection count display
func TestRenderFilePicker_SelectionCount(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{Name: "test-app"},
		},
	}
	plat := &platform.Platform{OS: "linux"}
	m := NewModel(cfg, plat, false)
	m.initSubEntryFormNew(0)

	m.subEntryForm.addFileMode = ModePicker
	m.subEntryForm.filePicker.CurrentDirectory = testNvimDir

	tests := []struct {
		name          string
		selectedFiles map[string]bool
		wantText      string
	}{
		{
			name:          "no selections",
			selectedFiles: map[string]bool{},
			wantText:      "0 file(s) selected",
		},
		{
			name: "single selection",
			selectedFiles: map[string]bool{
				testNvimDir + "/init.lua": true,
			},
			wantText: "1 file(s) selected",
		},
		{
			name: "multiple selections",
			selectedFiles: map[string]bool{
				testNvimDir + "/init.lua":    true,
				testNvimDir + "/plugins.lua": true,
				testNvimDir + "/colors.lua":  true,
			},
			wantText: "3 file(s) selected",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m.subEntryForm.selectedFiles = tt.selectedFiles
			view := m.viewFilePicker()

			if !strings.Contains(view, tt.wantText) {
				t.Errorf("view should contain %q, got:\n%s", tt.wantText, view)
			}
		})
	}
}

// TestRenderFilePicker_HelpText tests help text is shown
func TestRenderFilePicker_HelpText(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{Name: "test-app"},
		},
	}
	plat := &platform.Platform{OS: "linux"}
	m := NewModel(cfg, plat, false)
	m.initSubEntryFormNew(0)

	m.subEntryForm.addFileMode = ModePicker
	m.subEntryForm.filePicker.CurrentDirectory = testNvimDir

	view := m.viewFilePicker()

	// Verify help text contains expected keybindings
	helpKeys := []string{"space/tab", "enter", "esc"}
	for _, key := range helpKeys {
		if !strings.Contains(view, key) {
			t.Errorf("help text should contain %q", key)
		}
	}
}

// TestRenderFilePicker_SelectedRowStyle tests that selected files have proper styling
func TestRenderFilePicker_SelectedRowStyle(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{Name: "test-app"},
		},
	}
	plat := &platform.Platform{OS: "linux"}
	m := NewModel(cfg, plat, false)
	m.initSubEntryFormNew(0)

	m.subEntryForm.addFileMode = ModePicker
	m.subEntryForm.filePicker.CurrentDirectory = testNvimDir
	m.subEntryForm.selectedFiles = map[string]bool{
		testNvimDir + "/init.lua": true,
	}

	view := m.viewFilePicker()

	// Note: This test verifies the view contains the selection indicator
	// The actual ANSI styling is applied by lipgloss and can't be easily tested
	// without mocking the entire lipgloss rendering
	if !strings.Contains(view, "1 file(s) selected") {
		t.Error("view should indicate selected file")
	}
}

// TestRenderFilePicker_NoSelections tests rendering with no selections
func TestRenderFilePicker_NoSelections(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{Name: "test-app"},
		},
	}
	plat := &platform.Platform{OS: "linux"}
	m := NewModel(cfg, plat, false)
	m.initSubEntryFormNew(0)

	m.subEntryForm.addFileMode = ModePicker
	m.subEntryForm.filePicker.CurrentDirectory = testNvimDir
	m.subEntryForm.selectedFiles = map[string]bool{}

	view := m.viewFilePicker()

	// Should show 0 selections
	if !strings.Contains(view, "0 file(s) selected") {
		t.Error("view should show 0 file(s) selected")
	}

	// Should contain basic structure
	if !strings.Contains(view, "Select Files") {
		t.Error("view should contain title")
	}
}
