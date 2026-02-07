// Package tui provides the terminal user interface.
package tui

import (
	"fmt"

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
	case FormNone:
		fallthrough
	default:
		// No active form - should not happen, return to manage view
		m.Screen = ScreenResults
		m.Operation = OpList
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
	case FormNone:
		fallthrough
	default:
		// No active form - should not happen
		return BaseStyle.Render("Error: No form active")
	}
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

func (m Model) renderApplicationInlineDetail(_ *ApplicationItem, _ int) string {
	// Placeholder - to be implemented in Phase 5
	return ""
}

func (m Model) renderSubEntryInlineDetail(_ *SubEntryItem, _ int) string {
	// Placeholder - to be implemented in Phase 5
	return ""
}

// performRestoreSubEntry performs restore on a SubEntry
func (m Model) performRestoreSubEntry(subEntry config.SubEntry, target string) (bool, string) {
	if !subEntry.IsConfig() {
		return false, "Not a config entry"
	}

	backupPath := m.resolvePath(subEntry.Backup)

	var err error
	if subEntry.IsFolder() {
		if m.Manager.HasTemplateFiles(backupPath) {
			err = m.Manager.RestoreFolderWithTemplates(subEntry, backupPath, target)
		} else {
			err = m.Manager.RestoreFolder(subEntry, backupPath, target)
		}
	} else {
		err = m.Manager.RestoreFiles(subEntry, backupPath, target)
	}

	if err != nil {
		return false, fmt.Sprintf("Failed: %v", err)
	}

	return true, fmt.Sprintf("Restored: %s → %s", target, backupPath)
}
