package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updatePathSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.pathCursor > 0 {
			m.pathCursor--
			// Scroll up if needed
			if m.pathCursor < m.scrollOffset {
				m.scrollOffset = m.pathCursor
			}
		}
	case "down", "j":
		if m.pathCursor < len(m.Paths)-1 {
			m.pathCursor++
			// Scroll down if needed
			if m.pathCursor >= m.scrollOffset+m.viewHeight {
				m.scrollOffset = m.pathCursor - m.viewHeight + 1
			}
		}
	case " ", "x":
		// Toggle selection
		m.Paths[m.pathCursor].Selected = !m.Paths[m.pathCursor].Selected
	case "a":
		// Select all
		for i := range m.Paths {
			m.Paths[i].Selected = true
		}
	case "n":
		// Select none
		for i := range m.Paths {
			m.Paths[i].Selected = false
		}
	case "i":
		// Invert selection
		for i := range m.Paths {
			m.Paths[i].Selected = !m.Paths[i].Selected
		}
	case "enter":
		// Count selected
		selected := 0
		for _, p := range m.Paths {
			if p.Selected {
				selected++
			}
		}
		if selected > 0 {
			m.Screen = ScreenConfirm
		}
	case "g":
		// Go to top
		m.pathCursor = 0
		m.scrollOffset = 0
	case "G":
		// Go to bottom
		m.pathCursor = len(m.Paths) - 1
		if m.pathCursor >= m.viewHeight {
			m.scrollOffset = m.pathCursor - m.viewHeight + 1
		}
	}
	return m, nil
}

func (m Model) viewPathSelect() string {
	var b strings.Builder

	// Title
	title := fmt.Sprintf("󰋗  Select paths to %s", strings.ToLower(m.Operation.String()))
	b.WriteString(TitleStyle.Render(title))
	b.WriteString("\n\n")

	// Count selected
	selected := 0
	for _, p := range m.Paths {
		if p.Selected {
			selected++
		}
	}
	statusText := fmt.Sprintf("%d/%d selected", selected, len(m.Paths))
	b.WriteString(SubtitleStyle.Render(statusText))
	b.WriteString("\n\n")

	// Path list
	startIdx, endIdx := CalculateVisibleRange(m.scrollOffset, m.viewHeight, len(m.Paths))
	topIndicator, bottomIndicator := RenderScrollIndicators(startIdx, endIdx, len(m.Paths))

	b.WriteString(topIndicator)

	for i := startIdx; i < endIdx; i++ {
		item := m.Paths[i]
		isSelected := i == m.pathCursor
		cursor := RenderCursor(isSelected)
		checkbox := RenderCheckbox(item.Selected)

		// Name
		nameStyle := ListItemStyle
		if isSelected {
			nameStyle = SelectedListItemStyle
		}
		name := nameStyle.Render(item.Spec.Name)

		// State badge (only for Restore operation)
		stateBadge := ""
		if m.Operation == OpRestore {
			stateBadge = renderStateBadge(item.State)
		}

		// Badge for folders
		folderBadge := ""
		if item.Spec.IsFolder() {
			folderBadge = FolderBadgeStyle.Render("folder")
		}

		line := fmt.Sprintf("%s%s %s%s%s", cursor, checkbox, name, stateBadge, folderBadge)
		b.WriteString(line)
		b.WriteString("\n")

		// Show target on selected line
		if isSelected {
			targetLine := fmt.Sprintf("      %s → %s",
				PathBackupStyle.Render(truncatePath(m.resolvePath(item.Spec.Backup), 30)),
				PathTargetStyle.Render(truncatePath(item.Target, 30)),
			)
			b.WriteString(targetLine)
			b.WriteString("\n")
		}
	}

	b.WriteString(bottomIndicator)

	// Help
	b.WriteString("\n")
	b.WriteString(RenderHelp(
		"↑/k ↓/j", "navigate",
		"space", "toggle",
		"a/n/i", "all/none/invert",
		"enter", "confirm",
		"esc", "back",
	))

	return BaseStyle.Render(b.String())
}

func truncatePath(path string, maxLen int) string {
	if len(path) <= maxLen {
		return path
	}
	return "..." + path[len(path)-maxLen+3:]
}

func renderStateBadge(state PathState) string {
	switch state {
	case StateReady:
		return StateBadgeReadyStyle.Render("Ready")
	case StateAdopt:
		return StateBadgeAdoptStyle.Render("Adopt")
	case StateMissing:
		return StateBadgeMissingStyle.Render("Missing")
	case StateLinked:
		return StateBadgeLinkedStyle.Render("Linked")
	}
	return ""
}
