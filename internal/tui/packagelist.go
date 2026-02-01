package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updatePackageSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.packageCursor > 0 {
			m.packageCursor--
			// Scroll up if needed
			if m.packageCursor < m.scrollOffset {
				m.scrollOffset = m.packageCursor
			}
		}
	case "down", "j":
		if m.packageCursor < len(m.Packages)-1 {
			m.packageCursor++
			// Scroll down if needed
			if m.packageCursor >= m.scrollOffset+m.viewHeight {
				m.scrollOffset = m.packageCursor - m.viewHeight + 1
			}
		}
	case " ":
		// Toggle selection
		m.Packages[m.packageCursor].Selected = !m.Packages[m.packageCursor].Selected
	case "a":
		// Select all
		for i := range m.Packages {
			m.Packages[i].Selected = true
		}
	case "n":
		// Deselect all
		for i := range m.Packages {
			m.Packages[i].Selected = false
		}
	case "enter":
		// Count selected
		selected := 0
		for _, pkg := range m.Packages {
			if pkg.Selected {
				selected++
			}
		}
		if selected > 0 {
			m.Screen = ScreenConfirm
		}
		return m, nil
	}
	return m, nil
}

func (m Model) viewPackageSelect() string {
	var b strings.Builder

	// Title (centered)
	b.WriteString(RenderCenteredTitle("󰏖  Select Packages to Install", m.width))
	b.WriteString("\n\n")

	// Count selected
	selected := 0
	for _, pkg := range m.Packages {
		if pkg.Selected {
			selected++
		}
	}

	// Status line
	status := fmt.Sprintf("%d of %d packages selected", selected, len(m.Packages))
	b.WriteString(SubtitleStyle.Render(status))
	b.WriteString("\n\n")

	// Package list with scrolling
	visibleStart, visibleEnd := CalculateVisibleRange(m.scrollOffset, m.viewHeight, len(m.Packages))

	for i := visibleStart; i < visibleEnd; i++ {
		pkg := m.Packages[i]
		isSelected := i == m.packageCursor
		cursor := RenderCursor(isSelected)
		checkbox := RenderCheckbox(pkg.Selected)

		// Format: [✓] package-name (method)
		methodInfo := fmt.Sprintf("(%s)", pkg.Method)
		line := fmt.Sprintf("%s %s %s %s",
			cursor,
			checkbox,
			pkg.Spec.Name,
			SubtitleStyle.Render(methodInfo),
		)

		if isSelected {
			b.WriteString(SelectedListItemStyle.Render(line))
		} else {
			b.WriteString(line)
		}
		b.WriteString("\n")
	}

	// Scroll indicator
	if len(m.Packages) > m.viewHeight {
		b.WriteString("\n")
		scrollInfo := fmt.Sprintf("Showing %d-%d of %d", visibleStart+1, visibleEnd, len(m.Packages))
		b.WriteString(SubtitleStyle.Render(scrollInfo))
	}

	// Show description for current item
	if m.packageCursor < len(m.Packages) {
		pkg := m.Packages[m.packageCursor]
		if pkg.Spec.Description != "" {
			b.WriteString("\n\n")
			b.WriteString(BoxStyle.Render(pkg.Spec.Description))
		}
	}

	// Help
	b.WriteString("\n")
	b.WriteString(RenderHelp(
		"↑/↓", "navigate",
		"space", "toggle",
		"a", "all",
		"n", "none",
		"enter", "confirm",
		"esc", "back",
	))

	return BaseStyle.Render(b.String())
}
