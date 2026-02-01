package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) viewProgress() string {
	var b strings.Builder

	// Title
	title := fmt.Sprintf("⏳  %s in progress...", m.Operation.String())
	b.WriteString(TitleStyle.Render(title))
	b.WriteString("\n\n")

	// Spinner animation would go here
	b.WriteString(SpinnerStyle.Render("Processing..."))
	b.WriteString("\n")

	return BaseStyle.Render(b.String())
}

func (m Model) updateResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter", "q", "esc":
		if m.Operation == OpList {
			m.Screen = ScreenMenu
			return m, nil
		}
		return m, tea.Quit
	case "r":
		// Return to menu for another operation
		m.Screen = ScreenMenu
		// Reset selections
		for i := range m.Paths {
			m.Paths[i].Selected = true
		}
		m.pathCursor = 0
		m.scrollOffset = 0
		return m, nil
	}
	return m, nil
}

func (m Model) viewResults() string {
	var b strings.Builder

	// Title
	var title string
	if m.Operation == OpList {
		title = "󰋗  Configuration Paths"
	} else {
		title = fmt.Sprintf("✓  %s Complete", m.Operation.String())
	}
	b.WriteString(TitleStyle.Render(title))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n\n")
	}

	// Results summary
	successCount := 0
	failCount := 0
	for _, r := range m.results {
		if r.Success {
			successCount++
		} else {
			failCount++
		}
	}

	if m.Operation != OpList {
		summary := fmt.Sprintf("%d successful", successCount)
		if failCount > 0 {
			summary += fmt.Sprintf(", %d failed", failCount)
		}
		if m.DryRun {
			summary = WarningStyle.Render("[DRY RUN] ") + summary
		}
		b.WriteString(SubtitleStyle.Render(summary))
		b.WriteString("\n\n")
	}

	// Results list
	maxVisible := m.viewHeight
	if maxVisible > len(m.results) {
		maxVisible = len(m.results)
	}

	start := m.scrollOffset
	end := start + maxVisible
	if end > len(m.results) {
		end = len(m.results)
		start = end - maxVisible
		if start < 0 {
			start = 0
		}
	}

	if start > 0 {
		b.WriteString(SubtitleStyle.Render("↑ more above"))
		b.WriteString("\n")
	}

	for i := start; i < end; i++ {
		result := m.results[i]

		var icon string
		var nameStyle func(string) string

		if m.Operation == OpList {
			icon = "󰋗 "
			nameStyle = func(s string) string { return PathNameStyle.Render(s) }
		} else if result.Success {
			icon = SuccessStyle.Render("✓ ")
			nameStyle = func(s string) string { return SuccessStyle.Render(s) }
		} else {
			icon = ErrorStyle.Render("✗ ")
			nameStyle = func(s string) string { return ErrorStyle.Render(s) }
		}

		b.WriteString(icon + nameStyle(result.Name))
		b.WriteString("\n")

		// Show message indented
		if result.Message != "" {
			lines := strings.Split(result.Message, "\n")
			for _, line := range lines {
				b.WriteString("    " + SubtitleStyle.Render(line))
				b.WriteString("\n")
			}
		}
	}

	if end < len(m.results) {
		b.WriteString(SubtitleStyle.Render("↓ more below"))
		b.WriteString("\n")
	}

	// Help
	b.WriteString("\n")
	if m.Operation == OpList {
		b.WriteString(RenderHelp(
			"enter/esc", "back to menu",
			"q", "quit",
		))
	} else {
		b.WriteString(RenderHelp(
			"r", "new operation",
			"q/enter", "quit",
		))
	}

	return BaseStyle.Render(b.String())
}

func (m Model) processNextPath(index int) tea.Cmd {
	// This would be used for animated progress
	// For now, we process all at once in startOperation
	return nil
}
