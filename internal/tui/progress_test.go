package tui

import (
	"strings"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

// TestSearchIncludesAppsWithoutEntries verifies that searching matches apps by
// name/description even when they have no sub-entries (e.g., package-only apps).
func TestSearchIncludesAppsWithoutEntries(t *testing.T) {
	tests := []struct {
		name           string
		searchText     string
		applications   []ApplicationItem
		expectedApps   []string
		expectedCounts map[string]int // app name -> expected sub-entry count
	}{
		{
			name:       "app with no sub-entries matches by name",
			searchText: "pack",
			applications: []ApplicationItem{
				{
					Application: config.Application{Name: "package-only-app", Description: "A package app"},
					SubItems:    nil,
				},
				{
					Application: config.Application{Name: "nvim", Description: "editor"},
					SubItems: []SubEntryItem{
						{SubEntry: config.SubEntry{Name: "init.lua"}, Target: "~/.config/nvim"},
					},
				},
			},
			expectedApps:   []string{"package-only-app"},
			expectedCounts: map[string]int{"package-only-app": 0},
		},
		{
			name:       "app with no sub-entries matches by description",
			searchText: "editor",
			applications: []ApplicationItem{
				{
					Application: config.Application{Name: "helix", Description: "terminal editor"},
					SubItems:    nil,
				},
			},
			expectedApps:   []string{"helix"},
			expectedCounts: map[string]int{"helix": 0},
		},
		{
			name:       "app name matches includes all sub-entries",
			searchText: "nvim",
			applications: []ApplicationItem{
				{
					Application: config.Application{Name: "nvim", Description: "editor"},
					SubItems: []SubEntryItem{
						{SubEntry: config.SubEntry{Name: "init.lua"}, Target: "~/.config/nvim"},
						{SubEntry: config.SubEntry{Name: "plugins"}, Target: "~/.local/share/nvim"},
					},
				},
			},
			expectedApps:   []string{"nvim"},
			expectedCounts: map[string]int{"nvim": 2},
		},
		{
			name:       "no match returns empty",
			searchText: "nonexistent",
			applications: []ApplicationItem{
				{
					Application: config.Application{Name: "nvim", Description: "editor"},
					SubItems:    nil,
				},
			},
			expectedApps: nil,
		},
		{
			name:       "empty search returns all",
			searchText: "",
			applications: []ApplicationItem{
				{
					Application: config.Application{Name: "app1"},
					SubItems:    nil,
				},
				{
					Application: config.Application{Name: "app2"},
					SubItems: []SubEntryItem{
						{SubEntry: config.SubEntry{Name: "entry1"}},
					},
				},
			},
			expectedApps:   []string{"app1", "app2"},
			expectedCounts: map[string]int{"app1": 0, "app2": 1},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			m := Model{
				searchText:   tc.searchText,
				Applications: tc.applications,
			}

			result := m.getSearchedApplications()

			if tc.expectedApps == nil {
				if len(result) != 0 {
					t.Fatalf("expected no results, got %d", len(result))
				}
				return
			}

			if len(result) != len(tc.expectedApps) {
				names := make([]string, len(result))
				for i, r := range result {
					names[i] = r.Application.Name
				}
				t.Fatalf("expected %d apps %v, got %d apps %v",
					len(tc.expectedApps), tc.expectedApps, len(result), names)
			}

			for i, expectedName := range tc.expectedApps {
				if result[i].Application.Name != expectedName {
					t.Errorf("result[%d]: expected app %q, got %q",
						i, expectedName, result[i].Application.Name)
				}

				if expectedCount, ok := tc.expectedCounts[expectedName]; ok {
					if len(result[i].SubItems) != expectedCount {
						t.Errorf("result[%d] (%s): expected %d sub-entries, got %d",
							i, expectedName, expectedCount, len(result[i].SubItems))
					}
				}
			}
		})
	}
}

// TestExpandNodePreservesPosition verifies that expanding a node doesn't move it
// to the bottom due to re-sorting.
func TestExpandNodePreservesPosition(t *testing.T) {
	const nvimAppName = "nvim"
	// Create test config with three applications
	cfg := &config.Config{
		Applications: []config.Application{
			{
				Name: "zsh",
				Entries: []config.SubEntry{
					{
						Name:   "zshrc",
						Backup: "./zsh/zshrc",
						Targets: map[string]string{
							"linux": "~/.zshrc",
						},
					},
				},
			},
			{
				Name: nvimAppName,
				Entries: []config.SubEntry{
					{
						Name:   "init.lua",
						Backup: "./nvim/init.lua",
						Targets: map[string]string{
							"linux": "~/.config/nvim/init.lua",
						},
					},
				},
			},
			{
				Name: "bash",
				Entries: []config.SubEntry{
					{
						Name:   "bashrc",
						Backup: "./bash/bashrc",
						Targets: map[string]string{
							"linux": "~/.bashrc",
						},
					},
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	model := NewModel(cfg, plat, false)
	model.initApplicationItems()

	// Default sort should be by name ascending: bash(0), nvimAppName(1), zsh(2)
	model.sortColumn = SortColumnName
	model.sortAscending = true

	// Build initial table
	model.initTableModel()

	// Verify initial order: bash, nvimAppName, zsh
	if len(model.tableRows) != 3 {
		t.Fatalf("Expected 3 rows initially, got %d", len(model.tableRows))
	}

	// Record the names in order
	initialOrder := make([]string, len(model.tableRows))
	for i, row := range model.tableRows {
		// Extract name by removing the expand char
		name := strings.TrimPrefix(row.Data[0], "▶ ")
		name = strings.TrimPrefix(name, "▼ ")
		initialOrder[i] = name
	}

	// Verify sorted by name
	expectedOrder := []string{"bash", nvimAppName, "zsh"}
	for i, expected := range expectedOrder {
		if initialOrder[i] != expected {
			t.Errorf("Initial order at position %d: expected %s, got %s", i, expected, initialOrder[i])
		}
	}

	// Expand the middle node (nvim) at visual position 1
	// Find the actual app index for nvim
	var nvimAppIdx int
	for i, app := range model.Applications {
		if app.Application.Name == nvimAppName {
			nvimAppIdx = i
			break
		}
	}

	// Expand nvim
	model.Applications[nvimAppIdx].Expanded = true
	model.rebuildTable()

	// After expansion, nvim should still be at position 1 (index 1)
	// Expected table rows:
	// 0: bash
	// 1: nvim (expanded)
	// 2:   └─ init.lua (sub-entry)
	// 3: zsh

	if len(model.tableRows) != 4 {
		t.Fatalf("Expected 4 rows after expansion, got %d", len(model.tableRows))
	}

	// Check that nvim is still at position 1
	nvimRowName := strings.TrimPrefix(model.tableRows[1].Data[0], "▼ ")
	if nvimRowName != nvimAppName {
		t.Errorf("After expansion, row 1 should be %s, got %s", nvimAppName, nvimRowName)
		t.Logf("Current order after expansion:")
		for i, row := range model.tableRows {
			name := strings.TrimPrefix(row.Data[0], "▶ ")
			name = strings.TrimPrefix(name, "▼ ")
			name = strings.TrimSpace(name)
			name = strings.TrimPrefix(name, "├─")
			name = strings.TrimPrefix(name, "└─")
			name = strings.TrimSpace(name)
			t.Logf("  [%d] %s (Level=%d, AppIndex=%d, SubIndex=%d)", i, name, row.Level, row.AppIndex, row.SubIndex)
		}
	}

	// Verify the sub-entry is immediately after nvimAppName
	if model.tableRows[2].Level != 1 {
		t.Errorf("Row 2 should be a sub-entry (Level=1), got Level=%d", model.tableRows[2].Level)
	}

	// Verify zsh is still at position 3
	zshRowName := strings.TrimPrefix(model.tableRows[3].Data[0], "▶ ")
	if zshRowName != "zsh" {
		t.Errorf("After expansion, row 3 should be zsh, got %s", zshRowName)
	}
}
