package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

var menuItems = []struct {
	op   Operation
	name string
	desc string
	icon string
}{
	{OpRestore, "Restore", "Create symlinks from targets to backup sources", "󰁯"},
	{OpAdd, "Add", "Add a new path configuration", "󰐕"},
	{OpInstallPackages, "Install Packages", "Install packages using various package managers", "󰏖"},
	{OpList, "List", "Display all configured paths", "󰋗"},
}

func (m Model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		} else {
			m.menuCursor = len(menuItems) - 1
		}
	case "down", "j":
		if m.menuCursor < len(menuItems)-1 {
			m.menuCursor++
		} else {
			m.menuCursor = 0
		}
	case "enter", " ":
		m.Operation = menuItems[m.menuCursor].op
		if m.Operation == OpList {
			// List doesn't need path selection, show table view
			m.Screen = ScreenResults
			m.scrollOffset = 0
			m.listCursor = 0
			m.showingDetail = false
			return m, nil
		}
		if m.Operation == OpAdd {
			// Initialize add form and show it
			m.initAddForm()
			m.Screen = ScreenAddForm
			return m, nil
		}
		if m.Operation == OpInstallPackages {
			if len(m.Packages) == 0 {
				m.Screen = ScreenResults
				m.results = []ResultItem{{
					Name:    "No packages",
					Success: false,
					Message: "No installable packages found in configuration",
				}}
				return m, nil
			}
			m.Screen = ScreenPackageSelect
			return m, nil
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
	b.WriteString(RenderOSInfo(m.Platform.OS, m.Platform.IsRoot, m.Platform.IsArch, m.DryRun))
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
		"↑/↓", "navigate",
		"enter", "select",
		"q", "quit",
	))

	return BaseStyle.Render(b.String())
}

func (m Model) resolvePath(path string) string {
	if len(path) > 0 && path[0] == '.' {
		return m.Config.BackupRoot + path[1:]
	}
	return path
}

func mutedText(s string) string {
	return MutedTextStyle.Render(s)
}
