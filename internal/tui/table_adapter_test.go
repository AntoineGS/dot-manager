package tui

import (
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/charmbracelet/lipgloss"
)

const (
	treeCharBranch = "├─"
	treeCharEnd    = "└─"
)

func TestFlattenApplications(t *testing.T) {
	t.Run("single collapsed application", func(t *testing.T) {
		apps := []ApplicationItem{
			{
				Application: config.Application{Name: "nvim"},
				SubItems: []SubEntryItem{
					{SubEntry: config.SubEntry{Name: "config"}},
				},
				Expanded: false,
			},
		}

		rows := flattenApplications(apps, "linux", false)

		if len(rows) != 1 {
			t.Errorf("Expected 1 row, got %d", len(rows))
		}

		if rows[0].Level != 0 {
			t.Errorf("First row should be level 0")
		}

		if rows[0].SubIndex != -1 {
			t.Errorf("First row should have SubIndex -1, got %d", rows[0].SubIndex)
		}
	})

	t.Run("single expanded application", func(t *testing.T) {
		apps := []ApplicationItem{
			{
				Application: config.Application{Name: "nvim"},
				SubItems: []SubEntryItem{
					{SubEntry: config.SubEntry{Name: "config"}},
				},
				Expanded: true,
			},
		}

		rows := flattenApplications(apps, "linux", false)

		if len(rows) != 2 {
			t.Errorf("Expected 2 rows, got %d", len(rows))
		}

		if rows[0].Level != 0 {
			t.Errorf("First row should be level 0")
		}

		if rows[1].Level != 1 {
			t.Errorf("Second row should be level 1")
		}

		if rows[1].TreeChar != treeCharEnd {
			t.Errorf("Last sub-entry should have %s tree char, got %s", treeCharEnd, rows[1].TreeChar)
		}
	})

	t.Run("multiple sub-entries with correct tree chars", func(t *testing.T) {
		apps := []ApplicationItem{
			{
				Application: config.Application{Name: "nvim"},
				SubItems: []SubEntryItem{
					{SubEntry: config.SubEntry{Name: "config1"}},
					{SubEntry: config.SubEntry{Name: "config2"}},
					{SubEntry: config.SubEntry{Name: "config3"}},
				},
				Expanded: true,
			},
		}

		rows := flattenApplications(apps, "linux", false)

		if len(rows) != 4 {
			t.Errorf("Expected 4 rows, got %d", len(rows))
		}

		if rows[1].TreeChar != treeCharBranch {
			t.Errorf("First sub-entry should have %s tree char, got %s", treeCharBranch, rows[1].TreeChar)
		}

		if rows[2].TreeChar != treeCharBranch {
			t.Errorf("Middle sub-entry should have %s tree char, got %s", treeCharBranch, rows[2].TreeChar)
		}

		if rows[3].TreeChar != treeCharEnd {
			t.Errorf("Last sub-entry should have %s tree char, got %s", treeCharEnd, rows[3].TreeChar)
		}
	})
}

func TestGetApplicationStatus(t *testing.T) {
	t.Run("filtered application", func(t *testing.T) {
		app := ApplicationItem{
			IsFiltered: true,
		}

		status := getApplicationStatus(app)
		if status != StatusFiltered {
			t.Errorf("Expected StatusFiltered, got %s", status)
		}
	})

	t.Run("all linked", func(t *testing.T) {
		app := ApplicationItem{
			SubItems: []SubEntryItem{
				{State: StateLinked},
				{State: StateLinked},
			},
		}

		status := getApplicationStatus(app)
		if status != StatusInstalled {
			t.Errorf("Expected StatusInstalled, got %s", status)
		}
	})

	t.Run("some missing", func(t *testing.T) {
		app := ApplicationItem{
			SubItems: []SubEntryItem{
				{State: StateLinked},
				{State: StateMissing},
			},
		}

		status := getApplicationStatus(app)
		if status != StatusMissing {
			t.Errorf("Expected StatusMissing, got %s", status)
		}
	})
}

func TestGetTypeInfo(t *testing.T) {
	t.Run("folder", func(t *testing.T) {
		item := SubEntryItem{
			SubEntry: config.SubEntry{
				Files:  []string{}, // Empty files = folder
				Backup: "./test",   // Backup path indicates config type
			},
		}

		typeInfo := getTypeInfo(item)
		if typeInfo != TypeFolder {
			t.Errorf("Expected TypeFolder, got %s", typeInfo)
		}
	})

	t.Run("single file", func(t *testing.T) {
		item := SubEntryItem{
			SubEntry: config.SubEntry{
				Files: []string{"file1.txt"},
			},
		}

		typeInfo := getTypeInfo(item)
		if typeInfo != "1 file" {
			t.Errorf("Expected '1 file', got %s", typeInfo)
		}
	})

	t.Run("multiple files", func(t *testing.T) {
		item := SubEntryItem{
			SubEntry: config.SubEntry{
				Files: []string{"file1.txt", "file2.txt", "file3.txt"},
			},
		}

		typeInfo := getTypeInfo(item)
		if typeInfo != "3 files" {
			t.Errorf("Expected '3 files', got %s", typeInfo)
		}
	})
}

func TestVisualWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"ASCII", "hello", 5},
		{"Tree chars", "├─", 2},
		{"Emoji", "🎉", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width := lipgloss.Width(tt.input)
			if width != tt.expected {
				t.Errorf("Width of %q: expected %d, got %d", tt.input, tt.expected, width)
			}
		})
	}
}

func TestNeedsAttention(t *testing.T) {
	t.Run("status needs attention when not Installed", func(t *testing.T) {
		if !needsAttention(StatusMissing) {
			t.Errorf("StatusMissing should need attention")
		}
		if !needsAttention(StatusFiltered) {
			t.Errorf("StatusFiltered should need attention")
		}
	})

	t.Run("status does not need attention when Installed", func(t *testing.T) {
		if needsAttention(StatusInstalled) {
			t.Errorf("StatusInstalled should not need attention")
		}
	})

	t.Run("sub-entry state needs attention when not Linked", func(t *testing.T) {
		if !needsAttention(StateMissing.String()) {
			t.Errorf("StateMissing should need attention")
		}
		if !needsAttention(StateReady.String()) {
			t.Errorf("StateReady should need attention")
		}
		if !needsAttention(StateAdopt.String()) {
			t.Errorf("StateAdopt should need attention")
		}
	})

	t.Run("sub-entry state does not need attention when Linked", func(t *testing.T) {
		if needsAttention(StateLinked.String()) {
			t.Errorf("StateLinked should not need attention")
		}
	})
}

func TestAppInfoNeedsAttention(t *testing.T) {
	t.Run("app info needs attention when any sub-entry is not Linked", func(t *testing.T) {
		app := ApplicationItem{
			SubItems: []SubEntryItem{
				{State: StateLinked},
				{State: StateMissing},
			},
		}
		if !appInfoNeedsAttention(app) {
			t.Errorf("App with non-Linked sub-entry should need attention")
		}
	})

	t.Run("app info does not need attention when all sub-entries are Linked", func(t *testing.T) {
		app := ApplicationItem{
			SubItems: []SubEntryItem{
				{State: StateLinked},
				{State: StateLinked},
			},
		}
		if appInfoNeedsAttention(app) {
			t.Errorf("App with all Linked sub-entries should not need attention")
		}
	})

	t.Run("app info does not need attention when filtered", func(t *testing.T) {
		app := ApplicationItem{
			IsFiltered: true,
			SubItems: []SubEntryItem{
				{State: StateMissing},
			},
		}
		if appInfoNeedsAttention(app) {
			t.Errorf("Filtered app should not need attention")
		}
	})
}

func TestFlattenApplications_WithFilterEnabled(t *testing.T) {
	apps := []ApplicationItem{
		{
			Application: config.Application{Name: "visible-app"},
			SubItems: []SubEntryItem{
				{SubEntry: config.SubEntry{Name: "config1"}},
			},
			Expanded:   true,
			IsFiltered: false, // Not filtered - should be visible
		},
		{
			Application: config.Application{Name: "filtered-app"},
			SubItems: []SubEntryItem{
				{SubEntry: config.SubEntry{Name: "config2"}},
			},
			Expanded:   true,
			IsFiltered: true, // Filtered - should be hidden
		},
	}

	t.Run("filter enabled hides filtered apps", func(t *testing.T) {
		rows := flattenApplications(apps, "linux", true)

		// Should only show visible-app (1 app + 1 sub-entry = 2 rows)
		if len(rows) != 2 {
			t.Errorf("Expected 2 rows (1 app + 1 sub-entry), got %d", len(rows))
		}

		if rows[0].Data[0] != "▼ visible-app" {
			t.Errorf("Expected first row to be visible-app, got %s", rows[0].Data[0])
		}
	})

	t.Run("filter disabled shows all apps", func(t *testing.T) {
		rows := flattenApplications(apps, "linux", false)

		// Should show both apps (2 apps + 2 sub-entries = 4 rows)
		if len(rows) != 4 {
			t.Errorf("Expected 4 rows (2 apps + 2 sub-entries), got %d", len(rows))
		}
	})
}
