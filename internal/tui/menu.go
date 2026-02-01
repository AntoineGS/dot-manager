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
	{OpBackup, "Backup", "Copy files from targets to backup directory", "󰆓"},
	{OpInstallPackages, "Install Packages", "Install packages using various package managers", "󰏖"},
	{OpList, "List", "Display all configured paths", "󰋗"},
}

func (m Model) updateMenu(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "up", "k":
		if m.menuCursor > 0 {
			m.menuCursor--
		}
	case "down", "j":
		if m.menuCursor < len(menuItems)-1 {
			m.menuCursor++
		}
	case "enter", " ":
		m.Operation = menuItems[m.menuCursor].op
		if m.Operation == OpList {
			// List doesn't need path selection
			m.Screen = ScreenResults
			m.results = m.generateListResults()
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
	title := TitleStyle.Render("󰣇  dot-manager")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Subtitle with info
	osInfo := fmt.Sprintf("OS: %s", m.Platform.OS)
	if m.Platform.IsRoot {
		osInfo += " (root)"
	}
	if m.Platform.IsArch {
		osInfo += " • Arch Linux"
	}
	if m.DryRun {
		osInfo += " • " + WarningStyle.Render("DRY RUN")
	}
	b.WriteString(SubtitleStyle.Render(osInfo))
	b.WriteString("\n\n")

	// Menu items
	b.WriteString("Select an operation:\n\n")

	for i, item := range menuItems {
		cursor := "  "
		style := MenuItemStyle
		selected := i == m.menuCursor

		if selected {
			cursor = "▸ "
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

func (m Model) generateListResults() []ResultItem {
	var results []ResultItem

	for _, item := range m.Paths {
		var fileInfo string
		if item.Spec.IsFolder() {
			fileInfo = "[folder]"
		} else {
			fileInfo = strings.Join(item.Spec.Files, ", ")
		}

		msg := fmt.Sprintf("%s\n  backup: %s\n  target: %s",
			fileInfo,
			m.resolvePath(item.Spec.Backup),
			item.Target,
		)

		results = append(results, ResultItem{
			Name:    item.Spec.Name,
			Success: true,
			Message: msg,
		})
	}

	return results
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
