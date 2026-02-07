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
func Run(cfg *config.Config, plat *platform.Platform, dryRun bool, configPath string) error {
	mgr := manager.New(cfg, plat)
	mgr.DryRun = dryRun

	if err := mgr.InitStateStore(); err != nil {
		// Non-fatal: outdated detection won't work, but TUI is still usable
		fmt.Fprintf(os.Stderr, "Warning: could not initialize state store: %v\n", err)
	}
	defer func() { _ = mgr.Close() }()

	return RunWithManager(cfg, plat, mgr, configPath)
}

// RunWithManager runs the TUI with an existing manager
func RunWithManager(cfg *config.Config, plat *platform.Platform, mgr *manager.Manager, configPath string) error {
	model := NewModelWithManager(cfg, plat, mgr, configPath)

	p := tea.NewProgram(model, tea.WithAltScreen())

	finalModel, err := p.Run()
	if err != nil {
		return fmt.Errorf("TUI error: %w", err)
	}

	// If we completed an operation (not list), show a summary
	m, ok := finalModel.(Model)
	if !ok {
		return fmt.Errorf("unexpected model type")
	}

	if m.Screen == ScreenResults && m.Operation != OpList && len(m.results) > 0 {
		printFinalSummary(m)
	}

	return nil
}

// NewModelWithManager creates a model with a manager for real operations
func NewModelWithManager(cfg *config.Config, plat *platform.Platform, mgr *manager.Manager, configPath string) Model {
	m := NewModel(cfg, plat, mgr.DryRun)
	m.Manager = mgr
	m.ConfigPath = configPath

	// Re-detect application states now that the Manager is available.
	// NewModel() runs state detection before Manager is assigned, so features
	// like outdated template detection (which require the state store) are missed.
	m.refreshApplicationStates()
	m.initTableModel()

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
	fileInfo, err := os.Stdout.Stat()
	if err != nil {
		return false
	}

	return (fileInfo.Mode() & os.ModeCharDevice) != 0
}
