package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/AntoineGS/dot-manager/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

// initApplicationItems creates ApplicationItem list from v3 config
func (m *Model) initApplicationItems() {
	apps := m.Config.GetFilteredApplications(m.FilterCtx)

	m.Applications = make([]ApplicationItem, 0, len(apps))

	for _, app := range apps {
		subItems := make([]SubEntryItem, 0, len(app.Entries))

		for _, subEntry := range app.Entries {
			target := subEntry.GetTarget(m.Platform.OS)
			if target == "" {
				continue
			}

			subItem := SubEntryItem{
				SubEntry: subEntry,
				Target:   target,
				Selected: true,
				AppName:  app.Name,
			}

			subItems = append(subItems, subItem)
		}

		// Skip apps with no applicable entries
		if len(subItems) == 0 {
			continue
		}

		appItem := ApplicationItem{
			Application: app,
			Selected:    true,
			Expanded:    false,
			SubItems:    subItems,
		}

		// Add package info
		if app.HasPackage() {
			spec := config.PackageSpec{
				Name:     app.Name,
				Managers: app.Package.Managers,
				Custom:   app.Package.Custom,
				URL:      app.Package.URL,
			}
			method := getPackageInstallMethod(spec, m.Platform.OS)
			appItem.PkgMethod = method
			if method != "none" {
				installed := isPackageInstalled(spec, method)
				appItem.PkgInstalled = &installed
			}
		}

		m.Applications = append(m.Applications, appItem)
	}

	// Detect states for all sub-items
	m.refreshApplicationStates()
}

// refreshApplicationStates updates the state of all sub-entry items
func (m *Model) refreshApplicationStates() {
	for i := range m.Applications {
		for j := range m.Applications[i].SubItems {
			m.Applications[i].SubItems[j].State = m.detectSubEntryState(&m.Applications[i].SubItems[j])
		}
	}
}

// detectSubEntryState determines the state of a sub-entry item
func (m *Model) detectSubEntryState(item *SubEntryItem) PathState {
	// Similar to detectPathState but for SubEntry
	targetPath := item.Target

	if item.SubEntry.IsGit() {
		// Git entry logic (same as before)
		if pathExists(targetPath) {
			gitDir := filepath.Join(targetPath, ".git")
			if pathExists(gitDir) {
				return StateLinked
			}
			return StateAdopt
		}
		return StateReady
	}

	// Config entry logic
	backupPath := m.resolvePath(item.SubEntry.Backup)

	if item.SubEntry.IsFolder() {
		if info, err := os.Lstat(targetPath); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return StateLinked
			}
		}

		backupExists := pathExists(backupPath)
		targetExists := pathExists(targetPath)

		if backupExists {
			return StateReady
		}
		if targetExists {
			return StateAdopt
		}
		return StateMissing
	}

	// File-based config
	allLinked := true
	anyBackup := false
	anyTarget := false

	for _, file := range item.SubEntry.Files {
		srcFile := filepath.Join(backupPath, file)
		dstFile := filepath.Join(targetPath, file)

		if info, err := os.Lstat(dstFile); err == nil {
			if info.Mode()&os.ModeSymlink == 0 {
				allLinked = false
			}
		} else {
			allLinked = false
		}

		if pathExists(srcFile) {
			anyBackup = true
		}
		if pathExists(dstFile) {
			anyTarget = true
		}
	}

	if allLinked && len(item.SubEntry.Files) > 0 {
		return StateLinked
	}
	if anyBackup {
		return StateReady
	}
	if anyTarget {
		return StateAdopt
	}
	return StateMissing
}

func (m Model) updateApplicationSelect(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "q", "h", "left":
		// Go back to menu
		m.Screen = ScreenMenu
		return m, nil
	case "up", "k":
		if m.appCursor > 0 {
			m.appCursor--
			if m.appCursor < m.scrollOffset {
				m.scrollOffset = m.appCursor
			}
		}
	case "down", "j":
		visibleCount := m.getVisibleRowCount()
		if m.appCursor < visibleCount-1 {
			m.appCursor++
			if m.appCursor >= m.scrollOffset+m.viewHeight {
				m.scrollOffset = m.appCursor - m.viewHeight + 1
			}
		}
	case "enter", "tab", "right", "l":
		// Toggle expansion
		appIdx, _ := m.getApplicationAtCursor()
		if appIdx >= 0 {
			m.Applications[appIdx].Expanded = !m.Applications[appIdx].Expanded
		}
	case " ", "x":
		// Toggle selection
		appIdx, subIdx := m.getApplicationAtCursor()
		if appIdx >= 0 {
			if subIdx < 0 {
				// Toggle entire application
				m.Applications[appIdx].Selected = !m.Applications[appIdx].Selected
				// Propagate to sub-items
				for i := range m.Applications[appIdx].SubItems {
					m.Applications[appIdx].SubItems[i].Selected = m.Applications[appIdx].Selected
				}
			} else {
				// Toggle individual sub-item
				m.Applications[appIdx].SubItems[subIdx].Selected = !m.Applications[appIdx].SubItems[subIdx].Selected
			}
		}
	case "a":
		// Select all
		for i := range m.Applications {
			m.Applications[i].Selected = true
			for j := range m.Applications[i].SubItems {
				m.Applications[i].SubItems[j].Selected = true
			}
		}
	case "n":
		// Select none
		for i := range m.Applications {
			m.Applications[i].Selected = false
			for j := range m.Applications[i].SubItems {
				m.Applications[i].SubItems[j].Selected = false
			}
		}
	}
	return m, nil
}

// getApplicationAtCursor returns the app index and sub-entry index at current cursor
// Returns (-1, -1) if cursor is out of bounds
// Returns (appIdx, -1) if cursor is on application row
// Returns (appIdx, subIdx) if cursor is on sub-entry row
func (m *Model) getApplicationAtCursor() (int, int) {
	visualRow := 0

	for appIdx, app := range m.Applications {
		if visualRow == m.appCursor {
			return appIdx, -1
		}
		visualRow++

		if app.Expanded {
			for subIdx := range app.SubItems {
				if visualRow == m.appCursor {
					return appIdx, subIdx
				}
				visualRow++
			}
		}
	}

	return -1, -1
}

// getVisibleRowCount returns total number of visible rows in the 2-level table
func (m *Model) getVisibleRowCount() int {
	count := 0
	for _, app := range m.Applications {
		count++ // Application row
		if app.Expanded {
			count += len(app.SubItems) // Sub-entry rows
		}
	}
	return count
}

func (m Model) viewApplicationSelect() string {
	var b strings.Builder

	title := fmt.Sprintf("󰋗  Select items to %s", strings.ToLower(m.Operation.String()))
	b.WriteString(TitleStyle.Render(title))
	b.WriteString("\n\n")

	// Count selected
	selectedApps := 0
	selectedSubs := 0
	for _, app := range m.Applications {
		if app.Selected {
			selectedApps++
		}
		for _, sub := range app.SubItems {
			if sub.Selected {
				selectedSubs++
			}
		}
	}
	statusText := fmt.Sprintf("%d apps, %d entries selected", selectedApps, selectedSubs)
	b.WriteString(SubtitleStyle.Render(statusText))
	b.WriteString("\n\n")

	// Render 2-level table
	startIdx := m.scrollOffset
	endIdx := m.scrollOffset + m.viewHeight
	visualRow := 0

	for _, app := range m.Applications {
		// Render application row
		if visualRow >= startIdx && visualRow < endIdx {
			isSelected := visualRow == m.appCursor
			cursor := RenderCursor(isSelected)
			checkbox := RenderCheckbox(app.Selected)

			nameStyle := ListItemStyle
			if isSelected {
				nameStyle = SelectedListItemStyle
			}

			expandIcon := "▶"
			if app.Expanded {
				expandIcon = "▼"
			}

			entryCount := fmt.Sprintf("%d entries", len(app.SubItems))

			line := fmt.Sprintf("%s%s %s %s  %s",
				cursor, checkbox, expandIcon,
				nameStyle.Render(app.Application.Name),
				MutedTextStyle.Render(entryCount))

			b.WriteString(line)
			b.WriteString("\n")
		}
		visualRow++

		// Render sub-entry rows if expanded
		if app.Expanded {
			for _, subItem := range app.SubItems {
				if visualRow >= startIdx && visualRow < endIdx {
					isSelected := visualRow == m.appCursor
					cursor := RenderCursor(isSelected)
					checkbox := RenderCheckbox(subItem.Selected)

					nameStyle := ListItemStyle
					if isSelected {
						nameStyle = SelectedListItemStyle
					}

					typeIcon := "📄"
					if subItem.SubEntry.IsGit() {
						typeIcon = "📦"
					} else if subItem.SubEntry.IsFolder() {
						typeIcon = "📁"
					}

					stateBadge := ""
					if m.Operation == OpRestore {
						stateBadge = renderStateBadge(subItem.State)
					}

					fileInfo := ""
					if subItem.SubEntry.IsConfig() && !subItem.SubEntry.IsFolder() {
						fileInfo = fmt.Sprintf("%d file", len(subItem.SubEntry.Files))
						if len(subItem.SubEntry.Files) != 1 {
							fileInfo += "s"
						}
					}

					line := fmt.Sprintf("%s%s   ├─ %s %s%s  %s",
						cursor, checkbox, typeIcon,
						nameStyle.Render(subItem.SubEntry.Name),
						stateBadge,
						MutedTextStyle.Render(fileInfo))

					b.WriteString(line)
					b.WriteString("\n")

					// Show details on selected row
					if isSelected {
						if subItem.SubEntry.IsConfig() {
							detailLine := fmt.Sprintf("      %s → %s",
								PathBackupStyle.Render(truncatePath(m.resolvePath(subItem.SubEntry.Backup), 30)),
								PathTargetStyle.Render(truncatePath(subItem.Target, 30)))
							b.WriteString(detailLine)
							b.WriteString("\n")
						} else if subItem.SubEntry.IsGit() {
							detailLine := fmt.Sprintf("      %s → %s",
								PathBackupStyle.Render(truncatePath(subItem.SubEntry.Repo, 30)),
								PathTargetStyle.Render(truncatePath(subItem.Target, 30)))
							b.WriteString(detailLine)
							b.WriteString("\n")
						}
					}
				}
				visualRow++
			}
		}
	}

	// Help
	b.WriteString("\n")
	b.WriteString(RenderHelp(
		"space", "toggle",
		"enter/→", "expand",
		"a/n", "all/none",
		"q", "back",
	))

	return BaseStyle.Render(b.String())
}
