package tui

import (
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
	tea "github.com/charmbracelet/bubbletea"
)

func TestUpdateMenu_RestoreDryRun_SetsDryRunFlag(t *testing.T) {
	// Create a minimal model
	cfg := &config.Config{}
	plat := &platform.Platform{OS: "linux"}
	m := NewModel(cfg, plat, false)

	// Position cursor on "Restore (Dry Run)" (index 1)
	m.menuCursor = 1

	// Simulate pressing enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ := m.updateMenu(msg)

	updatedModel := updated.(Model)

	// Verify operation and dry-run flag
	if updatedModel.Operation != OpRestoreDryRun {
		t.Errorf("Operation = %v, want %v", updatedModel.Operation, OpRestoreDryRun)
	}
	if !updatedModel.DryRun {
		t.Error("DryRun = false, want true")
	}
	if updatedModel.Screen != ScreenPathSelect {
		t.Errorf("Screen = %v, want %v", updatedModel.Screen, ScreenPathSelect)
	}
}

func TestUpdateMenu_Restore_DoesNotSetDryRunFlag(t *testing.T) {
	// Create a minimal model
	cfg := &config.Config{}
	plat := &platform.Platform{OS: "linux"}
	m := NewModel(cfg, plat, false)

	// Position cursor on "Restore" (index 0)
	m.menuCursor = 0

	// Simulate pressing enter
	msg := tea.KeyMsg{Type: tea.KeyEnter}
	updated, _ := m.updateMenu(msg)

	updatedModel := updated.(Model)

	// Verify operation and dry-run flag
	if updatedModel.Operation != OpRestore {
		t.Errorf("Operation = %v, want %v", updatedModel.Operation, OpRestore)
	}
	if updatedModel.DryRun {
		t.Error("DryRun = true, want false")
	}
	if updatedModel.Screen != ScreenPathSelect {
		t.Errorf("Screen = %v, want %v", updatedModel.Screen, ScreenPathSelect)
	}
}
