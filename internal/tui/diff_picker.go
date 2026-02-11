package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// updateDiffPicker handles key events when the diff picker is showing.
func (m Model) updateDiffPicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case KeyEsc:
		m.showingDiffPicker = false
		m.diffPickerCursor = 0
		m.diffPickerFiles = nil
		return m, nil

	case "up", "k":
		if m.diffPickerCursor > 0 {
			m.diffPickerCursor--
		}
		return m, nil

	case "down", "j":
		if m.diffPickerCursor < len(m.diffPickerFiles)-1 {
			m.diffPickerCursor++
		}
		return m, nil

	case KeyEnter:
		if m.diffPickerCursor >= 0 && m.diffPickerCursor < len(m.diffPickerFiles) {
			selected := m.diffPickerFiles[m.diffPickerCursor]
			m.showingDiffPicker = false
			m.diffPickerCursor = 0
			m.diffPickerFiles = nil
			return m, launchDiffEditor(selected)
		}
		return m, nil
	}

	return m, nil
}

// viewDiffPicker renders the diff picker overlay showing modified template files.
func (m Model) viewDiffPicker() string {
	var b strings.Builder

	b.WriteString(SubtitleStyle.Render("Select a modified template to diff:"))
	b.WriteString("\n\n")

	for i, mt := range m.diffPickerFiles {
		cursor := "  "
		if i == m.diffPickerCursor {
			cursor = "> "
		}

		style := ListItemStyle
		if i == m.diffPickerCursor {
			style = SelectedListItemStyle
		}

		b.WriteString(fmt.Sprintf("%s%s\n", cursor, style.Render(mt.RelPath)))
	}

	b.WriteString("\n")
	b.WriteString(RenderHelpWithWidth(m.width,
		"enter", "select",
		"esc", "cancel",
	))

	return b.String()
}
