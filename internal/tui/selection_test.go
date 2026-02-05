package tui

import (
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

func TestNewModel_InitializesSelectionState(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{},
	}
	plat := &platform.Platform{
		OS:      "linux",
		EnvVars: map[string]string{"HOME": "/home/test"},
	}

	m := NewModel(cfg, plat, false)

	if m.selectedApps == nil {
		t.Error("selectedApps map should be initialized")
	}
	if m.selectedSubEntries == nil {
		t.Error("selectedSubEntries map should be initialized")
	}
	if m.multiSelectActive {
		t.Error("multiSelectActive should be false initially")
	}
}

func TestToggleAppSelection(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{
				Name: "app1",
				Entries: []config.SubEntry{
					{Name: "config1", Backup: "./config1", Targets: map[string]string{"linux": "~/.config1"}},
					{Name: "config2", Backup: "./config2", Targets: map[string]string{"linux": "~/.config2"}},
					{Name: "pkg1", Targets: map[string]string{"linux": "~/.pkg1"}}, // Non-config entry (no backup)
				},
			},
			{
				Name: "app2",
				Entries: []config.SubEntry{
					{Name: "config3", Backup: "./config3", Targets: map[string]string{"linux": "~/.config3"}},
				},
			},
		},
	}
	plat := &platform.Platform{
		OS:      "linux",
		EnvVars: map[string]string{"HOME": "/home/test"},
	}

	m := NewModel(cfg, plat, false)

	// Manually populate Applications array for testing
	m.Applications = []ApplicationItem{
		{
			Application: cfg.Applications[0],
			SubItems: []SubEntryItem{
				{SubEntry: cfg.Applications[0].Entries[0]},
				{SubEntry: cfg.Applications[0].Entries[1]},
				{SubEntry: cfg.Applications[0].Entries[2]},
			},
		},
		{
			Application: cfg.Applications[1],
			SubItems: []SubEntryItem{
				{SubEntry: cfg.Applications[1].Entries[0]},
			},
		},
	}

	// Toggle app 0 selection on
	m.toggleAppSelection(0)

	if !m.selectedApps[0] {
		t.Error("App 0 should be selected")
	}
	if !m.selectedSubEntries["0:0"] {
		t.Error("Sub-entry 0:0 should be selected")
	}
	if !m.selectedSubEntries["0:1"] {
		t.Error("Sub-entry 0:1 should be selected")
	}
	if !m.selectedSubEntries["0:2"] {
		t.Error("Sub-entry 0:2 should be selected")
	}
	if !m.multiSelectActive {
		t.Error("multiSelectActive should be true after selection")
	}

	// Toggle app 1 selection on (testing different appIdx)
	m.toggleAppSelection(1)

	if !m.selectedApps[1] {
		t.Error("App 1 should be selected")
	}
	if !m.selectedSubEntries["1:0"] {
		t.Error("Sub-entry 1:0 should be selected")
	}

	// Toggle app 0 selection off
	m.toggleAppSelection(0)

	if m.selectedApps[0] {
		t.Error("App 0 should be deselected")
	}
	if m.selectedSubEntries["0:0"] {
		t.Error("Sub-entry 0:0 should be deselected")
	}
	if m.selectedSubEntries["0:1"] {
		t.Error("Sub-entry 0:1 should be deselected")
	}
	if m.selectedSubEntries["0:2"] {
		t.Error("Sub-entry 0:2 should be deselected")
	}
	if !m.multiSelectActive {
		t.Error("multiSelectActive should still be true (app 1 is still selected)")
	}

	// Toggle app 1 selection off
	m.toggleAppSelection(1)

	if m.multiSelectActive {
		t.Error("multiSelectActive should be false after all deselections")
	}
}

func TestToggleSubEntrySelection(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{
				Name: "app1",
				Entries: []config.SubEntry{
					{Name: "config1", Backup: "./config1", Targets: map[string]string{"linux": "~/.config1"}},
					{Name: "config2", Backup: "./config2", Targets: map[string]string{"linux": "~/.config2"}},
				},
			},
		},
	}
	plat := &platform.Platform{
		OS:      "linux",
		EnvVars: map[string]string{"HOME": "/home/test"},
	}

	m := NewModel(cfg, plat, false)

	// Manually populate Applications array for testing
	m.Applications = []ApplicationItem{
		{
			Application: cfg.Applications[0],
			SubItems: []SubEntryItem{
				{SubEntry: cfg.Applications[0].Entries[0]},
				{SubEntry: cfg.Applications[0].Entries[1]},
			},
		},
	}

	// Toggle sub-entry selection on
	m.toggleSubEntrySelection(0, 0)

	if !m.selectedSubEntries["0:0"] {
		t.Error("Sub-entry 0:0 should be selected")
	}
	if !m.multiSelectActive {
		t.Error("multiSelectActive should be true after selection")
	}

	// Toggle another sub-entry
	m.toggleSubEntrySelection(0, 1)

	if !m.selectedSubEntries["0:1"] {
		t.Error("Sub-entry 0:1 should be selected")
	}

	// Toggle sub-entry selection off
	m.toggleSubEntrySelection(0, 0)

	if m.selectedSubEntries["0:0"] {
		t.Error("Sub-entry 0:0 should be deselected")
	}
	if !m.multiSelectActive {
		t.Error("multiSelectActive should still be true (0:1 is still selected)")
	}

	// Toggle last sub-entry off
	m.toggleSubEntrySelection(0, 1)

	if m.selectedSubEntries["0:1"] {
		t.Error("Sub-entry 0:1 should be deselected")
	}
	if m.multiSelectActive {
		t.Error("multiSelectActive should be false after all deselections")
	}
}

func TestClearSelections(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{
				Name: "app1",
				Entries: []config.SubEntry{
					{Name: "config1", Backup: "./config1", Targets: map[string]string{"linux": "~/.config1"}},
				},
			},
			{
				Name: "app2",
				Entries: []config.SubEntry{
					{Name: "config2", Backup: "./config2", Targets: map[string]string{"linux": "~/.config2"}},
				},
			},
		},
	}
	plat := &platform.Platform{
		OS:      "linux",
		EnvVars: map[string]string{"HOME": "/home/test"},
	}

	m := NewModel(cfg, plat, false)

	// Manually populate Applications array for testing
	m.Applications = []ApplicationItem{
		{
			Application: cfg.Applications[0],
			SubItems: []SubEntryItem{
				{SubEntry: cfg.Applications[0].Entries[0]},
			},
		},
		{
			Application: cfg.Applications[1],
			SubItems: []SubEntryItem{
				{SubEntry: cfg.Applications[1].Entries[0]},
			},
		},
	}

	// Select some items
	m.toggleAppSelection(0)
	m.toggleSubEntrySelection(1, 0)

	if !m.multiSelectActive {
		t.Error("multiSelectActive should be true before clearing")
	}
	if len(m.selectedApps) == 0 || len(m.selectedSubEntries) == 0 {
		t.Error("Should have selections before clearing")
	}

	// Clear all selections
	m.clearSelections()

	if len(m.selectedApps) != 0 {
		t.Error("selectedApps should be empty after clearing")
	}
	if len(m.selectedSubEntries) != 0 {
		t.Error("selectedSubEntries should be empty after clearing")
	}
	if m.multiSelectActive {
		t.Error("multiSelectActive should be false after clearing")
	}
}

func TestIsAppSelected(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{
				Name: "app1",
				Entries: []config.SubEntry{
					{Name: "config1", Backup: "./config1", Targets: map[string]string{"linux": "~/.config1"}},
				},
			},
		},
	}
	plat := &platform.Platform{
		OS:      "linux",
		EnvVars: map[string]string{"HOME": "/home/test"},
	}

	m := NewModel(cfg, plat, false)

	// Manually populate Applications array for testing
	m.Applications = []ApplicationItem{
		{
			Application: cfg.Applications[0],
			SubItems: []SubEntryItem{
				{SubEntry: cfg.Applications[0].Entries[0]},
			},
		},
	}

	// Initially not selected
	if m.isAppSelected(0) {
		t.Error("App 0 should not be selected initially")
	}

	// Select the app
	m.toggleAppSelection(0)

	// Now should be selected
	if !m.isAppSelected(0) {
		t.Error("App 0 should be selected after toggleAppSelection")
	}

	// Deselect the app
	m.toggleAppSelection(0)

	// Should be deselected again
	if m.isAppSelected(0) {
		t.Error("App 0 should be deselected after second toggleAppSelection")
	}
}

func TestIsSubEntrySelected(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{
				Name: "app1",
				Entries: []config.SubEntry{
					{Name: "config1", Backup: "./config1", Targets: map[string]string{"linux": "~/.config1"}},
					{Name: "config2", Backup: "./config2", Targets: map[string]string{"linux": "~/.config2"}},
				},
			},
			{
				Name: "app2",
				Entries: []config.SubEntry{
					{Name: "config3", Backup: "./config3", Targets: map[string]string{"linux": "~/.config3"}},
				},
			},
		},
	}
	plat := &platform.Platform{
		OS:      "linux",
		EnvVars: map[string]string{"HOME": "/home/test"},
	}

	m := NewModel(cfg, plat, false)

	// Manually populate Applications array for testing
	m.Applications = []ApplicationItem{
		{
			Application: cfg.Applications[0],
			SubItems: []SubEntryItem{
				{SubEntry: cfg.Applications[0].Entries[0]},
				{SubEntry: cfg.Applications[0].Entries[1]},
			},
		},
		{
			Application: cfg.Applications[1],
			SubItems: []SubEntryItem{
				{SubEntry: cfg.Applications[1].Entries[0]},
			},
		},
	}

	// Initially not selected
	if m.isSubEntrySelected(0, 0) {
		t.Error("Sub-entry 0:0 should not be selected initially")
	}

	// Select the sub-entry directly
	m.toggleSubEntrySelection(0, 0)

	if !m.isSubEntrySelected(0, 0) {
		t.Error("Sub-entry 0:0 should be selected after toggleSubEntrySelection")
	}

	// Deselect it
	m.toggleSubEntrySelection(0, 0)

	if m.isSubEntrySelected(0, 0) {
		t.Error("Sub-entry 0:0 should be deselected after second toggle")
	}

	// Test implicit selection via parent app
	m.toggleAppSelection(0)

	if !m.isSubEntrySelected(0, 0) {
		t.Error("Sub-entry 0:0 should be implicitly selected when app is selected")
	}
	if !m.isSubEntrySelected(0, 1) {
		t.Error("Sub-entry 0:1 should be implicitly selected when app is selected")
	}

	// Test different app index (app 1)
	m.toggleSubEntrySelection(1, 0)

	if !m.isSubEntrySelected(1, 0) {
		t.Error("Sub-entry 1:0 should be selected after toggleSubEntrySelection")
	}
}

func TestGetSelectionCounts(t *testing.T) {
	cfg := &config.Config{
		Applications: []config.Application{
			{
				Name: "nvim",
				Entries: []config.SubEntry{
					{Name: "config", Targets: map[string]string{"linux": "~/.config/nvim"}},
					{Name: "plugins", Targets: map[string]string{"linux": "~/.local/share/nvim"}},
				},
			},
			{
				Name: "zsh",
				Entries: []config.SubEntry{
					{Name: "zshrc", Targets: map[string]string{"linux": "~/.zshrc"}},
				},
			},
		},
	}
	plat := &platform.Platform{
		OS:      "linux",
		EnvVars: map[string]string{"HOME": "/home/test"},
	}

	m := NewModel(cfg, plat, false)
	m.initApplicationItems()

	// Select one app (nvim) - should count as 1 app, 0 independent sub-entries
	m.toggleAppSelection(0)

	appCount, subCount := m.getSelectionCounts()
	if appCount != 1 {
		t.Errorf("Expected 1 app selected, got %d", appCount)
	}
	if subCount != 0 {
		t.Errorf("Expected 0 independent sub-entries, got %d", subCount)
	}

	// Select one sub-entry from zsh (not selecting the app)
	m.toggleSubEntrySelection(1, 0)

	appCount, subCount = m.getSelectionCounts()
	if appCount != 1 {
		t.Errorf("Expected 1 app selected, got %d", appCount)
	}
	if subCount != 1 {
		t.Errorf("Expected 1 independent sub-entry, got %d", subCount)
	}

	// Deselect one sub-entry from nvim (partial selection)
	m.toggleSubEntrySelection(0, 0)

	appCount, subCount = m.getSelectionCounts()
	if appCount != 1 {
		t.Errorf("Expected 1 app selected, got %d", appCount)
	}
	if subCount != 1 {
		t.Errorf("Expected 1 independent sub-entry (from zsh), got %d", subCount)
	}
}

func TestCountHiddenSelections(t *testing.T) {
	m := Model{
		Applications: []ApplicationItem{
			{
				Application: config.Application{Name: "app1"},
				IsFiltered:  false,
				SubItems: []SubEntryItem{
					{SubEntry: config.SubEntry{Name: "sub1"}},
				},
			},
			{
				Application: config.Application{Name: "app2"},
				IsFiltered:  true, // This app is filtered
				SubItems: []SubEntryItem{
					{SubEntry: config.SubEntry{Name: "sub2"}},
				},
			},
			{
				Application: config.Application{Name: "app3"},
				IsFiltered:  true, // This app is filtered
				SubItems: []SubEntryItem{
					{SubEntry: config.SubEntry{Name: "sub3"}},
				},
			},
		},
		selectedApps:       make(map[int]bool),
		selectedSubEntries: make(map[string]bool),
	}

	t.Run("counts selected filtered apps", func(t *testing.T) {
		m.selectedApps[1] = true // Select filtered app2
		m.selectedApps[2] = true // Select filtered app3

		count := m.countHiddenSelections()
		if count != 2 {
			t.Errorf("Expected 2 hidden selections, got %d", count)
		}
	})

	t.Run("counts selected sub-entries under filtered apps", func(t *testing.T) {
		m.selectedApps = make(map[int]bool)
		m.selectedSubEntries["1:0"] = true // Sub-entry under filtered app2

		count := m.countHiddenSelections()
		if count != 1 {
			t.Errorf("Expected 1 hidden selection, got %d", count)
		}
	})

	t.Run("ignores selections under visible apps", func(t *testing.T) {
		m.selectedApps = make(map[int]bool)
		m.selectedSubEntries = make(map[string]bool)
		m.selectedApps[0] = true // Select visible app1

		count := m.countHiddenSelections()
		if count != 0 {
			t.Errorf("Expected 0 hidden selections, got %d", count)
		}
	})
}

func TestClearHiddenSelections(t *testing.T) {
	m := Model{
		Applications: []ApplicationItem{
			{
				Application: config.Application{Name: "app1"},
				IsFiltered:  false,
			},
			{
				Application: config.Application{Name: "app2"},
				IsFiltered:  true,
			},
		},
		selectedApps:       map[int]bool{0: true, 1: true},
		selectedSubEntries: map[string]bool{"0:0": true, "1:0": true},
		multiSelectActive:  true,
	}

	m.clearHiddenSelections()

	// Should keep app1 selection
	if !m.selectedApps[0] {
		t.Error("Expected app1 (visible) to remain selected")
	}

	// Should remove app2 selection
	if m.selectedApps[1] {
		t.Error("Expected app2 (filtered) to be deselected")
	}

	// Should keep sub-entry under visible app
	if !m.selectedSubEntries["0:0"] {
		t.Error("Expected sub-entry under visible app to remain selected")
	}

	// Should remove sub-entry under filtered app
	if m.selectedSubEntries["1:0"] {
		t.Error("Expected sub-entry under filtered app to be deselected")
	}

	// multiSelectActive should be updated
	if !m.multiSelectActive {
		t.Error("Expected multiSelectActive to remain true (visible app still selected)")
	}
}
