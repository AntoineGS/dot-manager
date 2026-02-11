package tui

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charmbracelet/bubbles/key"
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
	case OpRestore:
		title = "🔄  Restore Configs - Confirmation"
	case OpDelete, OpList:
		title = "🗑️  Delete Entries - Confirmation"
	}

	b.WriteString(TitleStyle.Render(title))
	b.WriteString("\n\n")

	// Render summary content based on operation
	switch m.summaryOperation {
	case OpInstallPackages:
		b.WriteString(m.renderInstallSummary())
	case OpRestore:
		b.WriteString(m.renderHierarchicalSummary("restore"))
	case OpDelete, OpList:
		b.WriteString(m.renderHierarchicalSummary("delete"))
	}

	// Help
	b.WriteString("\n\n")
	b.WriteString(RenderHelpFromBindings(m.width,
		SummaryKeys.Confirm,
		SummaryKeys.Cancel,
		SharedKeys.Quit,
	))

	return BaseStyle.Render(b.String())
}

// renderInstallSummary renders the install packages summary.
// Shows selected applications (app-level packages only).
func (m Model) renderInstallSummary() string {
	var b strings.Builder

	// Collect and sort selected app indices for deterministic output
	sortedAppIndices := make([]int, 0, len(m.selectedApps))
	for appIdx := range m.selectedApps {
		sortedAppIndices = append(sortedAppIndices, appIdx)
	}
	sort.Ints(sortedAppIndices)

	// Count selected apps with packages
	selectedAppsWithPkg := 0
	for _, appIdx := range sortedAppIndices {
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
	for _, appIdx := range sortedAppIndices {
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

	// Collect and sort selected app indices for deterministic output
	sortedAppIndices := make([]int, 0, len(m.selectedApps))
	for appIdx := range m.selectedApps {
		sortedAppIndices = append(sortedAppIndices, appIdx)
	}
	sort.Ints(sortedAppIndices)

	// Show selected apps (expanded with sub-entries)
	for _, appIdx := range sortedAppIndices {
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

	// Sort the selectedSubEntries keys for deterministic iteration
	sortedSubKeys := make([]string, 0, len(m.selectedSubEntries))
	for key := range m.selectedSubEntries {
		sortedSubKeys = append(sortedSubKeys, key)
	}
	sort.Strings(sortedSubKeys)

	for _, key := range sortedSubKeys {
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

	// Collect and sort standalone app indices for deterministic output
	sortedStandaloneIndices := make([]int, 0, len(standaloneSubs))
	for appIdx := range standaloneSubs {
		sortedStandaloneIndices = append(sortedStandaloneIndices, appIdx)
	}
	sort.Ints(sortedStandaloneIndices)

	// Render standalone sub-entries grouped by app
	for _, appIdx := range sortedStandaloneIndices {
		subIndices := standaloneSubs[appIdx]
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
	if m, cmd, handled := m.handleCommonKeys(msg); handled {
		return m, cmd
	}

	switch {
	case key.Matches(msg, SummaryKeys.Confirm):
		// Confirm - execute the batch operation
		return m.executeConfirmedOperation()

	case key.Matches(msg, SummaryKeys.Cancel):
		// Cancel - return to manage view
		m.Screen = ScreenResults
		m.Operation = OpList
		m.summaryDoublePress = ""
		return m, nil

	case key.Matches(msg, MultiSelectKeys.Restore):
		// Double-press restore trigger
		if m.summaryDoublePress == "r" {
			m.summaryDoublePress = ""
		} else {
			m.summaryDoublePress = "r"
		}
		return m, nil

	case key.Matches(msg, MultiSelectKeys.Install):
		// Double-press install trigger
		if m.summaryDoublePress == "i" {
			m.summaryDoublePress = ""
		} else {
			m.summaryDoublePress = "i"
		}
		return m, nil

	case key.Matches(msg, MultiSelectKeys.Delete):
		// Double-press delete trigger
		if m.summaryDoublePress == "d" {
			m.summaryDoublePress = ""
		} else {
			m.summaryDoublePress = "d"
		}
		return m, nil
	}

	return m, nil
}

// executeConfirmedOperation executes the confirmed batch operation.
// Initializes progress state and switches to progress screen.
func (m Model) executeConfirmedOperation() (tea.Model, tea.Cmd) {
	// Initialize progress bar
	m.batchProgress = initBatchProgress()

	// Reset progress counters
	m.batchCurrentItem = ""
	m.batchCurrentIndex = 0
	m.batchTotalItems = 0
	m.batchSuccessCount = 0
	m.batchFailCount = 0

	// Switch to progress screen
	m.Screen = ScreenProgress
	m.processing = true
	m.summaryDoublePress = ""

	// Execute appropriate batch operation based on summaryOperation
	var cmd tea.Cmd
	switch m.summaryOperation {
	case OpRestore:
		cmd = m.executeBatchRestore()
	case OpInstallPackages:
		cmd = m.executeBatchInstall()
	case OpDelete:
		cmd = m.executeBatchDelete()
	case OpList:
		// OpList should not reach the summary screen; return to manage view
		m.Screen = ScreenResults
		m.Operation = OpList
		m.processing = false
		cmd = nil
	}

	return m, cmd
}
