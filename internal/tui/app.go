package tui

import (
	"fmt"
	"os"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/manager"
	"github.com/AntoineGS/dot-manager/internal/platform"
	tea "github.com/charmbracelet/bubbletea"
)

// Run starts the interactive TUI with a new manager
func Run(cfg *config.Config, plat *platform.Platform, dryRun bool) error {
	mgr := manager.New(cfg, plat)
	mgr.DryRun = dryRun

	return RunWithManager(cfg, plat, mgr)
}

// RunWithManager runs the TUI with an existing manager
func RunWithManager(cfg *config.Config, plat *platform.Platform, mgr *manager.Manager) error {
	model := NewModelWithManager(cfg, plat, mgr)

	p := tea.NewProgram(model, tea.WithAltScreen())
	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// If we completed an operation (not list), show a summary
	m := finalModel.(Model)
	if m.Screen == ScreenResults && m.Operation != OpList && len(m.results) > 0 {
		printFinalSummary(m)
	}

	return nil
}

// NewModelWithManager creates a model with a manager for real operations
func NewModelWithManager(cfg *config.Config, plat *platform.Platform, mgr *manager.Manager) Model {
	m := NewModel(cfg, plat, mgr.DryRun)
	m.Manager = mgr
	return m
}

func printFinalSummary(m Model) {
	successCount := 0
	failCount := 0

	for _, r := range m.results {
		if r.Success {
			successCount++
		} else {
			failCount++
		}
	}

	fmt.Printf("\n%s complete: %d successful", m.Operation.String(), successCount)
	if failCount > 0 {
		fmt.Printf(", %d failed", failCount)
	}
	fmt.Println()
}

// IsTerminal checks if stdout is a terminal
func IsTerminal() bool {
	fileInfo, _ := os.Stdout.Stat()
	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
