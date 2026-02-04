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

		rows := flattenApplications(apps, "linux")

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

		rows := flattenApplications(apps, "linux")

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

		rows := flattenApplications(apps, "linux")

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
