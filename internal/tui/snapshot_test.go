package tui

import (
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/sebdah/goldie/v2"
)

// TestScreenResults_Snapshots tests the visual output of ScreenResults
// in various states using golden file snapshots.
func TestScreenResults_Snapshots(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*Model)
	}{
		{"basic_list", setupBasicList},
		{"app_expanded", setupAppExpanded},
		{"multi_select", setupMultiSelect},
		{"search_active", setupSearchActive},
		{"scroll_middle", setupScrollMiddle},
		{"scroll_bottom", setupScrollBottom},
		{"scroll_with_expanded", setupScrollWithExpanded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Force ASCII color profile for consistent rendering
			lipgloss.SetColorProfile(termenv.Ascii)

			// Create model with test setup
			m := createTestModel()
			tt.setupFunc(m)

			// Render the view
			output := m.View()

			// Strip ANSI codes and normalize
			plainText := stripAnsiCodes(output)
			normalized := normalizeOutput(plainText)

			// Compare with golden file
			g := goldie.New(t)
			g.Assert(t, tt.name, []byte(normalized))
		})
	}
}

// createTestModel creates a basic model for testing.
// Individual setup functions will customize it.
func createTestModel() *Model {
	cfg := &config.Config{
		Version:    3,
		BackupRoot: "/home/user/backup",
	}
	plat := &platform.Platform{OS: platform.OSLinux}
	m := NewModel(cfg, plat, false)
	return &m
}

// Placeholder setup functions (to be implemented in Tasks 4-6)

func setupBasicList(_ *Model) {
	// TODO: Implement in Task 4
}

func setupAppExpanded(_ *Model) {
	// TODO: Implement in Task 4
}

func setupMultiSelect(_ *Model) {
	// TODO: Implement in Task 4
}

func setupSearchActive(_ *Model) {
	// TODO: Implement in Task 5
}

func setupScrollMiddle(_ *Model) {
	// TODO: Implement in Task 6
}

func setupScrollBottom(_ *Model) {
	// TODO: Implement in Task 6
}

func setupScrollWithExpanded(_ *Model) {
	// TODO: Implement in Task 6
}
