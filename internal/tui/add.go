package tui

import (
	"fmt"
	"strings"

	"github.com/AntoineGS/dot-manager/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

// updateAddForm handles key events for the add form
// Routes to the appropriate form based on activeForm
func (m Model) updateAddForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.activeForm {
	case FormApplication:
		return m.updateApplicationForm(msg)
	case FormSubEntry:
		return m.updateSubEntryForm(msg)
	default:
		// No active form - should not happen, return to menu
		m.Screen = ScreenMenu
		return m, nil
	}
}

// viewAddForm renders the add form
// Routes to the appropriate form view based on activeForm
func (m Model) viewAddForm() string {
	switch m.activeForm {
	case FormApplication:
		return m.viewApplicationForm()
	case FormSubEntry:
		return m.viewSubEntryForm()
	default:
		// No active form - should not happen
		return BaseStyle.Render("Error: No form active")
	}
}

// deleteEntry removes an entry from Paths by finding its Application and SubEntry
// This is a compatibility wrapper for the old deleteEntry function
func (m *Model) deleteEntry(pathsIndex int) error {
	if pathsIndex < 0 || pathsIndex >= len(m.Paths) {
		return fmt.Errorf("invalid index")
	}

	entryName := m.Paths[pathsIndex].Entry.Name

	// Parse the entry name to find app and sub-entry
	// Entry names in v3 are formatted as "app/subentry"
	parts := strings.SplitN(entryName, "/", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid entry name format: %s", entryName)
	}

	appName := parts[0]
	subName := parts[1]

	// Find the application and sub-entry indices
	appIdx := -1
	subIdx := -1

	for i, app := range m.Config.Applications {
		if app.Name == appName {
			appIdx = i
			for j, sub := range app.Entries {
				if sub.Name == subName {
					subIdx = j
					break
				}
			}
			break
		}
	}

	if appIdx < 0 || subIdx < 0 {
		return fmt.Errorf("entry not found in config")
	}

	return m.deleteApplicationOrSubEntry(appIdx, subIdx)
}

// deleteApplication removes an entire Application
func (m *Model) deleteApplication(appIdx int) error {
	return m.deleteApplicationOrSubEntry(appIdx, -1)
}

// deleteSubEntry removes a SubEntry from an Application
func (m *Model) deleteSubEntry(appIdx, subIdx int) error {
	return m.deleteApplicationOrSubEntry(appIdx, subIdx)
}

// deleteApplicationOrSubEntry removes an Application or SubEntry from the config
func (m *Model) deleteApplicationOrSubEntry(appIdx, subIdx int) error {
	if subIdx >= 0 {
		// Deleting SubEntry
		app := &m.Config.Applications[appIdx]

		if len(app.Entries) == 1 {
			// Last SubEntry - delete whole Application
			m.Config.Applications = append(
				m.Config.Applications[:appIdx],
				m.Config.Applications[appIdx+1:]...,
			)
		} else {
			// Delete just this SubEntry
			app.Entries = append(
				app.Entries[:subIdx],
				app.Entries[subIdx+1:]...,
			)
		}
	} else {
		// Deleting entire Application
		m.Config.Applications = append(
			m.Config.Applications[:appIdx],
			m.Config.Applications[appIdx+1:]...,
		)
	}

	// Save and rebuild
	if err := config.Save(m.Config, m.ConfigPath); err != nil {
		return err
	}

	m.initApplicationItems()
	return nil
}

// Stub functions for other phases (to be implemented later)

func (m Model) calcSubEntryDetailHeight(item *SubEntryItem) int {
	// Placeholder - to be implemented in Phase 5
	return 5
}

func (m Model) calcApplicationDetailHeight(item *ApplicationItem) int {
	// Placeholder - to be implemented in Phase 5
	return 5
}

func (m Model) renderApplicationInlineDetail(item *ApplicationItem, width int) string {
	// Placeholder - to be implemented in Phase 5
	return ""
}

func (m Model) renderSubEntryInlineDetail(item *SubEntryItem, width int) string {
	// Placeholder - to be implemented in Phase 5
	return ""
}

// performRestoreSubEntry performs restore on a SubEntry
// This is adapted from performRestore but works with SubEntry instead of PathItem
func (m Model) performRestoreSubEntry(subEntry config.SubEntry, target string) (bool, string) {
	if !subEntry.IsConfig() {
		return false, "Not a config entry"
	}

	backupPath := m.resolvePath(subEntry.Backup)

	if subEntry.IsFolder() {
		return m.restoreFolder(backupPath, target)
	}
	return m.restoreFiles(subEntry.Files, backupPath, target)
}
