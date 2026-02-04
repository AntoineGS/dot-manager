package tui

import (
	tea "github.com/charmbracelet/bubbletea"
)

// viewSummary renders the summary/confirmation screen for batch operations.
// Shows what will be affected by the batch operation (restore, install, delete).
func (m Model) viewSummary() string {
	// TODO: Implement summary view rendering (Task 9)
	return BaseStyle.Render("Summary screen - to be implemented")
}

// updateSummary handles keyboard input for the summary screen.
// Supports y/enter to confirm, r/i/d for double-press, n/esc to cancel.
func (m Model) updateSummary(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// TODO: Implement summary screen navigation (Task 10)
	switch msg.String() {
	case "n", "esc":
		// Cancel - return to manage view
		m.Screen = ScreenResults
		m.Operation = OpList
		m.summaryDoublePress = ""
		return m, nil
	case "q":
		// Quit the application
		return m, tea.Quit
	}

	return m, nil
}
