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
	case KeyEnter:
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

	// Determine if we have enough width to show backup path inline
	// Width threshold: 120 chars allows for cursor(2) + checkbox(2) + name(30) + state(8) + folder(8) + backup(25) + target(30) + margins
	showBackupColumn := m.width >= 120

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
		name := nameStyle.Render(item.Entry.Name)

		// State badge (only for Restore operation)
		stateBadge := ""
		if m.Operation == OpRestore {
			stateBadge = renderStateBadge(item.State)
		}

		// Badge for folders
		folderBadge := ""
		if item.Entry.IsFolder() {
			folderBadge = FolderBadgeStyle.Render("folder")
		}

		// Build line with optional backup column
		var line string
		if showBackupColumn {
			// Show backup path inline
			backupPath := PathBackupStyle.Render(truncatePath(item.Entry.Backup, 25))
			targetPath := PathTargetStyle.Render(truncatePath(item.Target, 30))
			line = fmt.Sprintf("%s%s %s%s%s  %s → %s",
				cursor, checkbox, name, stateBadge, folderBadge,
				backupPath, targetPath,
			)
		} else {
			// Original single-line format
			line = fmt.Sprintf("%s%s %s%s%s", cursor, checkbox, name, stateBadge, folderBadge)
		}
		b.WriteString(line)
		b.WriteString("\n")

		// Show target on selected line (only if not already shown inline)
		if isSelected && !showBackupColumn {
			targetLine := fmt.Sprintf("      %s → %s",
				// Show backup path as-is (e.g., "./nvim") without resolving
				PathBackupStyle.Render(truncatePath(item.Entry.Backup, 30)),
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
		"space", "toggle",
		"a/n/i", "all/none/invert",
		"enter", "confirm",
		"q", "back",
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
	// Render styled text, then pad to 7 chars (length of "Missing") for consistent alignment
	// Padding is added AFTER styling so spaces aren't highlighted
	switch state {
	case StateReady:
		return StateBadgeReadyStyle.Render("Ready") + "  "
	case StateAdopt:
		return StateBadgeAdoptStyle.Render("Adopt") + "  "
	case StateMissing:
		return StateBadgeMissingStyle.Render("Missing")
	case StateLinked:
		return StateBadgeLinkedStyle.Render("Linked") + " "
	}

	return "       " // 7 spaces for empty state
}
