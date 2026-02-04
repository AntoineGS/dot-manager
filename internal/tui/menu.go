package tui

import (
	"fmt"
	"strings"

	"github.com/AntoineGS/dot-manager/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

var menuItems = []struct {
	name string
	desc string
	icon string
	op   Operation
}{
	{"Restore", "Create symlinks from targets to backup sources", "󰁯", OpRestore},
	{"Restore (Dry Run)", "Preview restore without making changes", "󰋖", OpRestoreDryRun},
	{"Manage", "Browse, edit and install", "󰋗", OpList},
}

func (m Model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		} else {
			m.menuCursor = len(menuItems) - 1
		}
	case KeyDown, "j":
		if m.menuCursor < len(menuItems)-1 {
			m.menuCursor++
		} else {
			m.menuCursor = 0
		}
	case KeyEnter, " ", "l":
		m.Operation = menuItems[m.menuCursor].op
		if m.Operation == OpList {
			// List doesn't need path selection, show table view
			// Initialize applications for hierarchical view
			m.initApplicationItems()
			m.Screen = ScreenResults
			m.scrollOffset = 0
			m.appCursor = 0
			m.showingDetail = false

			return m, nil
		}
		// Set dry-run flag based on operation
		switch m.Operation {
		case OpRestoreDryRun:
			m.DryRun = true
		case OpRestore:
			m.DryRun = false
		case OpAdd, OpList, OpInstallPackages:
			// Other operations don\'t use dry-run flag
		}
		m.Screen = ScreenPathSelect

		return m, nil
	}

	return m, nil
}

func (m Model) viewMenu() string {
	var b strings.Builder

	// Title
	b.WriteString(TitleStyle.Render("󰣇  dot-manager"))
	b.WriteString("\n\n")

	// Subtitle with info
	b.WriteString(RenderOSInfo(m.Platform.OS, m.Platform.IsArch, m.DryRun))
	b.WriteString("\n\n")

	// Menu items
	b.WriteString("Select an operation:\n\n")

	for i, item := range menuItems {
		selected := i == m.menuCursor
		cursor := RenderCursor(selected)

		style := MenuItemStyle
		if selected {
			style = SelectedMenuItemStyle
		}

		var line string
		if selected {
			// Don't apply muted style when selected - it breaks contrast
			line = fmt.Sprintf("%s %s  %s", item.icon, item.name, item.desc)
		} else {
			line = fmt.Sprintf("%s %s  %s", item.icon, item.name, mutedText(item.desc))
		}

		b.WriteString(cursor + style.Render(line) + "\n")
	}

	// Help
	b.WriteString("\n")
	b.WriteString(RenderHelp(
		"q", "quit",
	))

	return BaseStyle.Render(b.String())
}

func (m Model) resolvePath(path string) string {
	// Resolve relative paths against BackupRoot
	resolvedPath := path
	if len(path) > 0 && path[0] == '.' {
		resolvedPath = m.Config.BackupRoot + path[1:]
	}

	// Expand ~ for file operations
	return config.ExpandPath(resolvedPath, m.Platform.EnvVars)
}

func mutedText(s string) string {
	return MutedTextStyle.Render(s)
}
