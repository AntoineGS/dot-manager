package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// viewSummary renders the summary/confirmation screen for batch operations.
// Shows what will be affected by the batch operation (restore, install, delete).
func (m Model) viewSummary() string {
	var b strings.Builder

	// Title based on operation
	var title string
	switch m.summaryOperation {
	case OpInstallPackages:
		title = "📦  Install Packages - Confirmation"
	case OpRestore, OpRestoreDryRun:
		title = "🔄  Restore Configs - Confirmation"
	case OpAdd, OpList:
		// These operations don't use summary screen
		title = "⚠️  Unexpected Operation"
	default:
		title = "🗑️  Delete Entries - Confirmation"
	}

	b.WriteString(TitleStyle.Render(title))
	b.WriteString("\n\n")

	// Render summary content based on operation
	switch m.summaryOperation {
	case OpInstallPackages:
		b.WriteString(m.renderInstallSummary())
	case OpRestore, OpRestoreDryRun:
		b.WriteString(m.renderHierarchicalSummary("restore"))
	case OpAdd, OpList:
		// These operations don't use summary screen
		b.WriteString(ErrorStyle.Render("Error: Invalid operation for summary screen"))
		b.WriteString("\n")
	default:
		b.WriteString(m.renderHierarchicalSummary("delete"))
	}

	// Help
	b.WriteString("\n\n")
	b.WriteString(RenderHelp(
		"y/enter", "confirm",
		"n/esc", "cancel",
		"q", "quit",
	))

	return BaseStyle.Render(b.String())
}

// renderInstallSummary renders the install packages summary.
// Shows selected applications (app-level packages only).
func (m Model) renderInstallSummary() string {
	var b strings.Builder

	// Count selected apps with packages
	selectedAppsWithPkg := 0
	for appIdx := range m.selectedApps {
		if appIdx >= 0 && appIdx < len(m.Applications) {
			app := m.Applications[appIdx]
			if app.PkgInstalled != nil && !*app.PkgInstalled {
				selectedAppsWithPkg++
			}
		}
	}

	b.WriteString(SubtitleStyle.Render(fmt.Sprintf("Will install packages for %d application(s):", selectedAppsWithPkg)))
	b.WriteString("\n\n")

	// List selected apps with packages
	for appIdx := range m.selectedApps {
		if appIdx >= 0 && appIdx < len(m.Applications) {
			app := m.Applications[appIdx]
			if app.PkgInstalled != nil && !*app.PkgInstalled {
				b.WriteString(CheckedStyle.Render("  • "))
				b.WriteString(PathNameStyle.Render(app.Application.Name))
				if app.PkgMethod != "" && app.PkgMethod != TypeNone {
					b.WriteString(MutedTextStyle.Render(fmt.Sprintf(" (%s)", app.PkgMethod)))
				}
				b.WriteString("\n")
			}
		}
	}

	if selectedAppsWithPkg == 0 {
		b.WriteString(MutedTextStyle.Render("  No packages to install (all already installed)"))
		b.WriteString("\n")
	}

	return b.String()
}

// renderHierarchicalSummary renders the hierarchical summary for restore/delete operations.
// Shows selected apps + sub-entries with their details.
func (m Model) renderHierarchicalSummary(operation string) string {
	var b strings.Builder

	// Count selections
	appCount, subEntryCount := m.getSelectionCounts()

	actionVerb := "restored"
	if operation == "delete" {
		actionVerb = "deleted"
	}

	b.WriteString(SubtitleStyle.Render(fmt.Sprintf("%d application(s), %d item(s) will be %s:", appCount, subEntryCount, actionVerb)))
	b.WriteString("\n\n")

	// Show selected apps (expanded with sub-entries)
	for appIdx := range m.selectedApps {
		if appIdx >= 0 && appIdx < len(m.Applications) {
			app := m.Applications[appIdx]
			// App header
			b.WriteString(CheckedStyle.Render("▼ "))
			b.WriteString(PathNameStyle.Render(app.Application.Name))
			b.WriteString(MutedTextStyle.Render(fmt.Sprintf(" (%d entries)", len(app.SubItems))))
			b.WriteString("\n")

			// Sub-entries
			for _, sub := range app.SubItems {
				b.WriteString("  ")
				b.WriteString(CheckedStyle.Render("  • "))
				b.WriteString(sub.SubEntry.Name)
				b.WriteString(MutedTextStyle.Render(fmt.Sprintf(" → %s", sub.Target)))
				b.WriteString("\n")
			}
		}
	}

	// Show standalone selected sub-entries (parent not selected)
	standaloneSubs := make(map[int][]int) // appIdx -> []subIdx
	for key := range m.selectedSubEntries {
		var appIdx, subIdx int
		if _, err := fmt.Sscanf(key, "%d:%d", &appIdx, &subIdx); err != nil {
			continue
		}

		// Skip if parent app is selected (already shown above)
		if m.selectedApps[appIdx] {
			continue
		}

		standaloneSubs[appIdx] = append(standaloneSubs[appIdx], subIdx)
	}

	// Render standalone sub-entries grouped by app
	for appIdx, subIndices := range standaloneSubs {
		if appIdx >= 0 && appIdx < len(m.Applications) {
			app := m.Applications[appIdx]
			// Show app header (not fully selected, just a container)
			b.WriteString(MutedTextStyle.Render("▶ "))
			b.WriteString(app.Application.Name)
			b.WriteString(MutedTextStyle.Render(fmt.Sprintf(" (%d/%d entries)", len(subIndices), len(app.SubItems))))
			b.WriteString("\n")

			// Show selected sub-entries
			for _, subIdx := range subIndices {
				if subIdx >= 0 && subIdx < len(app.SubItems) {
					sub := app.SubItems[subIdx]
					b.WriteString("  ")
					b.WriteString(CheckedStyle.Render("  • "))
					b.WriteString(sub.SubEntry.Name)
					b.WriteString(MutedTextStyle.Render(fmt.Sprintf(" → %s", sub.Target)))
					b.WriteString("\n")
				}
			}
		}
	}

	if appCount == 0 && subEntryCount == 0 {
		b.WriteString(MutedTextStyle.Render("  No items selected"))
		b.WriteString("\n")
	}

	return b.String()
}

// updateSummary handles keyboard input for the summary screen.
// Supports y/enter to confirm, r/i/d for double-press, n/esc to cancel.
func (m Model) updateSummary(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// TODO: Implement summary screen navigation (Task 10)
	switch msg.String() {
	case "n", KeyEsc:
		// Cancel - return to manage view
		m.Screen = ScreenResults
		m.Operation = OpList
		m.summaryDoublePress = ""
		return m, nil
	case "q":
		// Quit the application
		return m, tea.Quit
	}

	return m, nil
}
