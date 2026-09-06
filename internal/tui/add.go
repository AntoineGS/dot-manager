// Package tui provides the terminal user interface.
package tui

import (
	"fmt"
	"slices"

	tea "charm.land/bubbletea/v2"
	"github.com/AntoineGS/tidydots/internal/config"
)

// updateAddForm handles key events for the add form
// Routes to the appropriate form based on activeForm
func (m Model) updateAddForm(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
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
	if appIdx < 0 || appIdx >= len(m.Config.Applications) || subIdx < -1 || subIdx >= len(m.Config.Applications[appIdx].Entries) {
		return fmt.Errorf("invalid application or sub-entry index")
	}
	name := m.Config.Applications[appIdx].Name
	if subIdx >= 0 {
		name += "/" + m.Config.Applications[appIdx].Entries[subIdx].Name
	}
	if m.previewConfigChange("delete", name) {
		return nil
	}
	// Work on detached slices so a failed save cannot change shared config.
	next := *m.Config
	next.Applications = slices.Clone(m.Config.Applications)
	if subIdx >= 0 {
		// Deleting SubEntry
		app := &next.Applications[appIdx]
		app.Entries = slices.Clone(app.Entries)

		if len(app.Entries) == 1 && app.Package == nil {
			// Last SubEntry - delete whole Application
			next.Applications = append(
				next.Applications[:appIdx],
				next.Applications[appIdx+1:]...,
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
		next.Applications = append(
			next.Applications[:appIdx],
			next.Applications[appIdx+1:]...,
		)
	}

	// Save and rebuild
	if err := config.Save(&next, m.ConfigPath); err != nil {
		return err
	}
	*m.Config = next

	m.reinitPreservingState("")

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

// performRestoreSubEntry performs restore on a SubEntry.
//
// A sub-entry is either a config entry (it deploys files) or a setup entry (it
// runs a command); restore means both, so this handles both. Setup entries used
// to be rejected here with "Not a config entry" — on the very rows the TUI had
// flagged as needing setup.
//
// CAUTION: for a setup entry this shells out and may prompt for a sudo
// password. It must therefore only be reached from a goroutine that does not
// hold the terminal — go through startSetupRun, which wraps the call in
// tea.Exec (see setup_run.go). Never call it for a setup entry from the
// bubbletea Update/View goroutine or from a plain tea.Cmd.
func (m Model) performRestoreSubEntry(item SubEntryItem) (bool, string) {
	if item.SubEntry.IsSetup() {
		return m.runSetupForItem(item)
	}

	subEntry := item.SubEntry
	if !subEntry.IsConfig() {
		return false, "Not a config entry"
	}

	target, backupPath, err := resolveSubEntryPaths(item, m.pathManager())
	if err != nil {
		return false, fmt.Sprintf("Failed: %v", err)
	}
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

	return true, fmt.Sprintf("Restored: %s -> %s", target, backupPath)
}
