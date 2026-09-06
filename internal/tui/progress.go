package tui

import (
	"context"
	"fmt"
	"os/exec"
	"slices"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/manager"
	"github.com/AntoineGS/tidydots/internal/packages"
)

// pkgCheckResultMsg is sent when a single package install check completes.
type pkgCheckResultMsg struct {
	appIndex  int
	method    string
	installed bool
}

// stateCheckResultMsg is sent when a single sub-entry state check completes.
type stateCheckResultMsg struct {
	appIndex int
	subIndex int
	state    PathState
}

// subEntryAppliesToOS reports whether a sub-entry has anything to do on this OS.
//
// A setup entry qualifies when it declares a run command for the OS — the same
// condition manager.runSetupEntry uses to decide whether to act. Listing an
// entry that does not apply here would report it as "Set up" (its check is
// absent, so IsSetupApplied has nothing outstanding to report), which is a
// claim about a machine the entry never targeted.
//
// A config entry qualifies when it declares a target path for the OS.
func subEntryAppliesToOS(subEntry config.SubEntry, osType string) bool {
	if subEntry.IsSetup() {
		return subEntry.GetRun(osType) != ""
	}

	return subEntry.GetTarget(osType) != ""
}

// initApplicationItems creates ApplicationItem list from v3 config
func (m *Model) initApplicationItems() {
	// Get ALL applications, not just filtered ones
	apps := m.Config.Applications

	m.Applications = make([]ApplicationItem, 0, len(apps))

	for _, app := range apps {
		// Check if this app matches the when expression
		isFiltered := !config.EvaluateWhen(app.When, m.Renderer)

		subItems := make([]SubEntryItem, 0, len(app.Entries))

		entries := app.Entries
		if isFiltered {
			entries = nil
		}
		for _, subEntry := range config.FilterSubEntries(entries, m.Renderer) {
			if !subEntryAppliesToOS(subEntry, m.Platform.OS) {
				continue
			}

			// Setup entries deploy no files, so they have no target. Config
			// entries do; expand ~ and env vars for the file operations.
			expandedTarget := ""
			if target := subEntry.GetTarget(m.Platform.OS); target != "" {
				expandedTarget = config.ExpandPath(target, m.Platform.EnvVars)
			}

			subItem := SubEntryItem{
				SubEntry: subEntry,
				Target:   expandedTarget,
				AppName:  app.Name,
				Index:    len(subItems),
			}

			subItems = append(subItems, subItem)
		}

		// Skip apps with no applicable entries AND no packages
		if len(subItems) == 0 && !app.HasPackage() && !isFiltered {
			continue
		}

		appItem := ApplicationItem{
			Application: app,
			Expanded:    false,
			SubItems:    subItems,
			IsFiltered:  isFiltered,
		}

		// Package method and install check deferred to async

		m.Applications = append(m.Applications, appItem)
	}

	// Sort applications alphabetically by name
	slices.SortFunc(m.Applications, func(a, b ApplicationItem) int {
		return strings.Compare(a.Application.Name, b.Application.Name)
	})

	// Initialize table model with the loaded applications
	m.initTableModel()
}

// reinitPreservingState rebuilds application items from config while preserving
// existing states for all apps except the edited one. The edited app gets its
// sub-entry states synchronously refreshed. Pass empty string to preserve all states.
func (m *Model) reinitPreservingState(editedAppName string) {
	type savedAppState struct {
		subStates    map[string]PathState // subEntry name -> state
		pkgMethod    string
		pkgInstalled *bool
		expanded     bool
	}

	// Save existing states by app name
	saved := make(map[string]savedAppState)
	for _, app := range m.Applications {
		subStates := make(map[string]PathState)
		for _, sub := range app.SubItems {
			subStates[sub.SubEntry.Name] = sub.State
		}
		saved[app.Application.Name] = savedAppState{
			subStates:    subStates,
			pkgMethod:    app.PkgMethod,
			pkgInstalled: app.PkgInstalled,
			expanded:     app.Expanded,
		}
	}

	// Rebuild from config
	m.initApplicationItems()

	// Restore preserved states
	for i, app := range m.Applications {
		prev, ok := saved[app.Application.Name]
		if !ok {
			// New app (e.g., just created) - synchronously detect states
			for j := range m.Applications[i].SubItems {
				m.Applications[i].SubItems[j].State = m.detectSubEntryState(&m.Applications[i].SubItems[j])
			}
			continue
		}

		// Restore expanded and package states for all apps
		m.Applications[i].Expanded = prev.expanded
		m.Applications[i].PkgMethod = prev.pkgMethod
		m.Applications[i].PkgInstalled = prev.pkgInstalled

		if app.Application.Name == editedAppName {
			// For the edited app, synchronously refresh sub-entry states
			for j := range m.Applications[i].SubItems {
				m.Applications[i].SubItems[j].State = m.detectSubEntryState(&m.Applications[i].SubItems[j])
			}
		} else {
			// For other apps, restore previous sub-entry states
			for j, sub := range app.SubItems {
				if state, exists := prev.subStates[sub.SubEntry.Name]; exists {
					m.Applications[i].SubItems[j].State = state
				}
			}
		}
	}

	m.pruneStaleSelections()

	// Rebuild table with restored states
	m.initTableModel()
}

// refreshApplicationStates updates the state of all sub-entry items
func (m *Model) refreshApplicationStates() {
	for i := range m.Applications {
		for j := range m.Applications[i].SubItems {
			m.Applications[i].SubItems[j].State = m.detectSubEntryState(&m.Applications[i].SubItems[j])
		}
	}
}

// getApplicationAtCursorFromTable resolves the cursor row to real indices into
// m.Applications (and its SubItems). Resolution goes through the row's AppName:
// names are the stable identity — the table is built from a sorted, filtered
// COPY of m.Applications, so no position on a row can be trusted against the
// model except SubIndex, which flattenApplications stamps from the real slice.
func (m *Model) getApplicationAtCursorFromTable() (int, int) {
	if m.tableCursor < 0 || m.tableCursor >= len(m.tableRows) {
		return -1, -1
	}

	tableRow := m.tableRows[m.tableCursor]

	// Look up the real index in m.Applications by name
	realAppIdx := -1
	for i, app := range m.Applications {
		if app.Application.Name == tableRow.AppName {
			realAppIdx = i
			break
		}
	}

	if realAppIdx == -1 {
		return -1, -1
	}

	return realAppIdx, tableRow.SubIndex
}

func (m Model) viewProgress() string {
	var b strings.Builder

	// Title
	title := fmt.Sprintf("⏳  %s in progress...", m.Operation.String())
	b.WriteString(TitleStyle.Render(title))
	b.WriteString("\n\n")

	// Show current item if batch operation
	if m.batchTotalItems > 0 {
		// Progress counter
		progressText := fmt.Sprintf("Processing %d of %d items", m.batchCurrentIndex+1, m.batchTotalItems)
		b.WriteString(SubtitleStyle.Render(progressText))
		b.WriteString("\n\n")

		// Current item being processed
		if m.batchCurrentItem != "" {
			b.WriteString(PathNameStyle.Render("Current: "))
			b.WriteString(m.batchCurrentItem)
			b.WriteString("\n\n")
		}

		// Progress bar
		percent := float64(m.batchCurrentIndex) / float64(m.batchTotalItems)
		b.WriteString(m.batchProgress.ViewAs(percent))
		b.WriteString("\n\n")

		// Stats
		statsText := fmt.Sprintf("✓ %d successful  ✗ %d failed", m.batchSuccessCount, m.batchFailCount)
		b.WriteString(MutedTextStyle.Render(statsText))
		b.WriteString("\n")
	} else {
		// Fallback for non-batch operations
		b.WriteString(SpinnerStyle.Render("Processing..."))
		b.WriteString("\n")
	}

	return BaseStyle.Render(b.String())
}

//nolint:gocyclo // UI handler with many states
func (m Model) updateResults(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Handle search mode input
	if m.Operation == OpList && m.searching {
		switch {
		case key.Matches(msg, SearchKeys.Cancel):
			// Clear search and exit search mode
			m.searching = false
			m.searchText = ""
			m.searchInput.SetValue("")
			m.searchInput.Blur()
			// Rebuild table without search
			m.rebuildTable()

			return m, nil
		case key.Matches(msg, SearchKeys.Confirm):
			// Confirm search and return to navigation mode
			m.searching = false
			m.searchInput.Blur()

			return m, nil
		default:
			// Pass key to text input
			var cmd tea.Cmd
			m.searchInput, cmd = m.searchInput.Update(msg)
			m.searchText = m.searchInput.Value()
			// Rebuild table with new search
			m.rebuildTable()

			return m, cmd
		}
	}

	// Handle results popup
	if m.Operation == OpList && m.showingResults {
		return m.updateResultsPopup(msg)
	}

	// Handle filter toggle confirmation
	if m.Operation == OpList && m.confirmingFilterToggle {
		switch {
		case key.Matches(msg, ConfirmKeys.Yes):
			// Confirm - toggle filter and clear hidden selections
			m.confirmingFilterToggle = false
			m.filterToggleHiddenCount = 0
			m.filterEnabled = true
			m.clearHiddenSelections()
			m.rebuildTable()
			return m, nil
		case key.Matches(msg, ConfirmKeys.No):
			// Cancel - keep filter off
			m.confirmingFilterToggle = false
			m.filterToggleHiddenCount = 0
			return m, nil
		}
		return m, nil
	}

	// Handle action filter toggle confirmation. Unlike the machine filter,
	// action filtering preserves selections after confirmation.
	if m.Operation == OpList && m.confirmingActionFilter {
		switch {
		case key.Matches(msg, ConfirmKeys.Yes):
			m.confirmingActionFilter = false
			m.actionFilterHiddenCount = 0
			m.actionFilterEnabled = true
			m.rebuildTable()
			return m, nil
		case key.Matches(msg, ConfirmKeys.No):
			m.confirmingActionFilter = false
			m.actionFilterHiddenCount = 0
			return m, nil
		}
		return m, nil
	}

	// Handle delete confirmation
	if m.Operation == OpList && (m.confirmingDeleteApp || m.confirmingDeleteSubEntry) {
		switch {
		case key.Matches(msg, ConfirmKeys.Yes):
			// Confirm delete. appIdx/subIdx index the sorted m.Applications and
			// its OS-filtered SubItems; deletion slices the unsorted, unfiltered
			// m.Config, so both must be resolved to config indices by name.
			appIdx, subIdx := m.getApplicationAtCursorFromTable()
			if m.confirmingDeleteApp && appIdx >= 0 {
				m.confirmingDeleteApp = false
				configAppIdx := m.findConfigApplicationIndex(m.Applications[appIdx].Application.Name)
				if configAppIdx >= 0 {
					if err := m.deleteApplication(configAppIdx); err != nil {
						m.err = err
					} else {
						// Rebuild table after deletion
						m.rebuildTable()
					}
				}
			} else if m.confirmingDeleteSubEntry && appIdx >= 0 && subIdx >= 0 {
				m.confirmingDeleteSubEntry = false
				app := m.Applications[appIdx]
				configAppIdx := m.findConfigApplicationIndex(app.Application.Name)
				configSubIdx := -1
				if configAppIdx >= 0 {
					configSubIdx = m.findConfigSubEntryIndex(configAppIdx, app.SubItems[subIdx].SubEntry.Name)
				}
				if configSubIdx >= 0 {
					if err := m.deleteSubEntry(configAppIdx, configSubIdx); err != nil {
						m.err = err
					} else {
						// Rebuild table after deletion
						m.rebuildTable()
					}
				}
			}

			return m, nil
		case key.Matches(msg, ConfirmKeys.No):
			// Cancel delete
			m.confirmingDeleteApp = false
			m.confirmingDeleteSubEntry = false

			return m, nil
		}

		return m, nil
	}

	// Handle diff picker separately
	if m.Operation == OpList && m.showingDiffPicker {
		return m.updateDiffPicker(msg)
	}

	// Handle detail popup separately
	if m.Operation == OpList && m.showingDetail {
		if m, cmd, handled := m.handleCommonKeys(msg); handled {
			return m, cmd
		}

		switch {
		case key.Matches(msg, DetailKeys.Close):
			// Close detail popup (ESC cancels/closes the popup)
			m.showingDetail = false
			return m, nil
		}

		return m, nil
	}

	// Handle ESC to clear active search or selections (when not in search mode but search text or selections are present)
	if m.Operation == OpList && key.Matches(msg, FormNavKeys.Cancel) && !m.searching {
		// Priority 1: Clear search first if active
		if m.searchText != "" {
			m.searchText = ""
			m.searchInput.SetValue("")
			m.rebuildTable()
			return m, nil
		}
		// Priority 2: Clear selections if any exist
		if m.multiSelectActive {
			m.clearSelections()
			return m, nil
		}
	}

	// Helper to check if we're in a clean list state (no modals/search)
	listClean := m.Operation == OpList && !m.searching && !m.confirmingDeleteApp && !m.confirmingDeleteSubEntry && !m.showingDetail
	if m.stateChecksPending() && (key.Matches(msg, ListKeys.Edit) ||
		key.Matches(msg, ListKeys.AddApp) || key.Matches(msg, ListKeys.AddEntry) ||
		key.Matches(msg, ListKeys.Delete) || key.Matches(msg, ListKeys.Install) ||
		key.Matches(msg, ListKeys.Restore) || key.Matches(msg, ListKeys.ForceRestore) ||
		key.Matches(msg, ListKeys.Filter)) {
		return m, nil
	}
	if !listClean || !key.Matches(msg, ListKeys.GoTop) {
		m.pendingG = false
	}

	switch {
	case key.Matches(msg, ListKeys.Refresh):
		if m.canRefreshAllStates() {
			return m, m.refreshAllStates()
		}
		return m, nil
	case key.Matches(msg, ListKeys.GoTop):
		if listClean {
			if m.pendingG {
				m.pendingG = false
				m.tableCursor = 0
				m.scrollOffset = 0
			} else {
				m.pendingG = true
			}
		}
		return m, nil
	case key.Matches(msg, ListKeys.GoBottom):
		if listClean {
			m.pendingG = false
			if len(m.tableRows) > 0 {
				m.tableCursor = len(m.tableRows) - 1
				m.updateScrollOffset()
			} else {
				m.tableCursor = 0
				m.scrollOffset = 0
			}
		}
		return m, nil
	case key.Matches(msg, ListKeys.HalfPageUp):
		if listClean {
			movement := m.computeMaxVisibleRows() / 2
			if movement < 1 {
				movement = 1
			}
			m.tableCursor -= movement
			if m.tableCursor < 0 {
				m.tableCursor = 0
			}
			m.updateScrollOffset()
		}
		return m, nil
	case key.Matches(msg, ListKeys.HalfPageDown):
		if listClean {
			if len(m.tableRows) == 0 {
				m.tableCursor = 0
				m.scrollOffset = 0
				return m, nil
			}
			movement := m.computeMaxVisibleRows() / 2
			if movement < 1 {
				movement = 1
			}
			m.tableCursor += movement
			if m.tableCursor >= len(m.tableRows) {
				m.tableCursor = len(m.tableRows) - 1
			}
			m.updateScrollOffset()
		}
		return m, nil
	case key.Matches(msg, ListKeys.Search):
		// Enter search mode
		if listClean {
			m.searching = true
			m.searchInput.Focus()

			return m, nil
		}
	case key.Matches(msg, ListKeys.SortByName):
		// Sort by name
		if listClean {
			if m.sortColumn == SortColumnName {
				m.sortAscending = !m.sortAscending
			} else {
				m.sortColumn = SortColumnName
				m.sortAscending = true
			}
			m.rebuildTable()
			return m, nil
		}
	case key.Matches(msg, ListKeys.SortByStatus):
		// Sort by status
		if listClean {
			if m.sortColumn == SortColumnStatus {
				m.sortAscending = !m.sortAscending
			} else {
				m.sortColumn = SortColumnStatus
				m.sortAscending = true
			}
			m.rebuildTable()
			return m, nil
		}
	case key.Matches(msg, ListKeys.SortByPath):
		// Sort by path
		if listClean {
			if m.sortColumn == SortColumnPath {
				m.sortAscending = !m.sortAscending
			} else {
				m.sortColumn = SortColumnPath
				m.sortAscending = true
			}
			m.rebuildTable()
			return m, nil
		}
	case key.Matches(msg, ListKeys.Filter):
		// Toggle filter
		if listClean {
			// If toggling filter ON (false -> true), check if selections would be hidden
			if !m.filterEnabled && m.multiSelectActive {
				hiddenCount := m.countHiddenSelections()
				if hiddenCount > 0 {
					// Show confirmation dialog - set a new state flag
					m.confirmingFilterToggle = true
					m.filterToggleHiddenCount = hiddenCount
					return m, nil
				}
			}

			// Toggle filter (no confirmation needed or toggling OFF)
			wasEnabled := m.filterEnabled
			m.filterEnabled = !m.filterEnabled
			m.rebuildTable()

			// When turning filter OFF, scan any filtered apps that haven't been checked yet
			if wasEnabled {
				return m, m.dispatchFilteredStates()
			}

			return m, nil
		}
	case key.Matches(msg, ListKeys.ActionFilter):
		if listClean {
			if !m.actionFilterEnabled && m.multiSelectActive {
				hiddenCount := m.countHiddenActionSelections()
				if hiddenCount > 0 {
					m.confirmingActionFilter = true
					m.actionFilterHiddenCount = hiddenCount
					return m, nil
				}
			}
			m.actionFilterEnabled = !m.actionFilterEnabled
			m.rebuildTable()
		}
		return m, nil
	case key.Matches(msg, SharedKeys.Quit):
		// Quit the application
		return m, tea.Quit
	case key.Matches(msg, ListKeys.Up):
		if m.Operation == OpList {
			// Move cursor up
			if m.tableCursor > 0 {
				m.tableCursor--
				m.updateScrollOffset()
			}
			return m, nil
		}

		return m, nil
	case key.Matches(msg, ListKeys.Down):
		if m.Operation == OpList {
			// Move cursor down
			if m.tableCursor < len(m.tableRows)-1 {
				m.tableCursor++
				m.updateScrollOffset()
			}
			return m, nil
		}

		return m, nil
	case key.Matches(msg, ListKeys.Collapse):
		if m.Operation == OpList {
			// Collapse node if expanded
			appIdx, _ := m.getApplicationAtCursorFromTable()
			if appIdx >= 0 && m.Applications[appIdx].Expanded {
				m.Applications[appIdx].Expanded = false
				// Rebuild table to reflect collapsed state
				m.rebuildTable()
			}
			// If not expanded, 'h' does nothing (use 'q' to go back to menu)

			return m, nil
		}

		return m, tea.Quit
	case key.Matches(msg, ListKeys.Expand):
		if m.Operation == OpList {
			// If showing detail, close it; otherwise expand (not toggle)
			if m.showingDetail {
				m.showingDetail = false
			} else {
				appIdx, subIdx := m.getApplicationAtCursorFromTable()
				if appIdx >= 0 && subIdx < 0 {
					// Only expand application rows, not sub-entries
					m.Applications[appIdx].Expanded = true
					// Rebuild table to show expanded children
					m.rebuildTable()
				}
			}

			return m, nil
		}

		return m, tea.Quit
	case key.Matches(msg, ListKeys.Edit):
		// Edit selected Application or SubEntry (only in List view)
		if m.Operation == OpList {
			appIdx, subIdx := m.getApplicationAtCursorFromTable()
			if appIdx >= 0 {
				if subIdx >= 0 {
					if !m.Applications[appIdx].IsFiltered {
						// Edit SubEntry
						m.initSubEntryForm(appIdx, subIdx)
					}
				} else {
					// Edit Application
					m.initApplicationForm(appIdx)
				}

				if subIdx < 0 || !m.Applications[appIdx].IsFiltered {
					return m, nil
				}
			}
		}
	case key.Matches(msg, ListKeys.AddApp):
		// Add new Application (only in List view)
		if m.Operation == OpList {
			m.initApplicationForm(-1)
			return m, nil
		}
	case key.Matches(msg, ListKeys.AddEntry):
		// Add new SubEntry to current Application (only in List view)
		if m.Operation == OpList {
			appIdx, _ := m.getApplicationAtCursorFromTable()
			if appIdx >= 0 {
				m.startEntryTypeChooser(appIdx)
				return m, nil
			}
		}
	case key.Matches(msg, ListKeys.Delete):
		// Ask for delete confirmation (only in List view)
		if m.Operation == OpList {
			m.pruneStaleSelections()
			// Check if multi-select mode is active
			if m.multiSelectActive {
				// Show summary screen for batch delete
				m.summaryOperation = OpDelete
				m.Screen = ScreenSummary
				return m, nil
			}

			// Single-item delete (original behavior)
			appIdx, subIdx := m.getApplicationAtCursorFromTable()
			if appIdx >= 0 && (subIdx < 0 || !m.Applications[appIdx].IsFiltered) {
				if subIdx >= 0 {
					m.confirmingDeleteSubEntry = true
				} else {
					m.confirmingDeleteApp = true
				}

				return m, nil
			}
		}
	case key.Matches(msg, ListKeys.Install):
		// Install or Diff depending on context (only in List view)
		if m.Operation == OpList {
			m.pruneStaleSelections()
			// Check if multi-select mode is active
			if m.multiSelectActive {
				// Show summary screen for batch install
				m.summaryOperation = OpInstallPackages
				m.Screen = ScreenSummary
				return m, nil
			}

			appIdx, subIdx := m.getApplicationAtCursorFromTable()

			// On a modified config entry: launch diff viewer. Symlink templates
			// compare their rendered cache; copy templates compare their live
			// deployed targets.
			if appIdx >= 0 && subIdx >= 0 && m.Manager != nil {
				subItem := m.Applications[appIdx].SubItems[subIdx]
				if subItem.State == StateModified && subItem.SubEntry.IsConfig() {
					backupPath := m.resolvePath(subItem.SubEntry.Backup)
					targetPath := config.ExpandPath(subItem.Target, m.Platform.EnvVars)
					var (
						modifiedFiles []manager.ModifiedTemplate
						err           error
					)
					if subItem.SubEntry.IsCopy() {
						var inspection manager.CopyTemplateInspection
						inspection, err = m.Manager.InspectCopyTemplates(subItem.SubEntry, backupPath, targetPath)
						modifiedFiles = inspection.Modified
					} else {
						modifiedFiles, err = m.Manager.GetModifiedTemplateFiles(backupPath, subItem.SubEntry.Files)
					}
					if err != nil {
						return m, func() tea.Msg { return editorLaunchCompleteMsg{err: err} }
					}
					if len(modifiedFiles) == 0 {
						return m, nil
					}

					if len(modifiedFiles) == 1 {
						// Single file: launch editor directly
						return m, launchDiffEditor(modifiedFiles[0])
					}

					// Multiple files: show picker
					m.showingDiffPicker = true
					m.diffPickerCursor = 0
					m.diffPickerFiles = modifiedFiles
					return m, nil
				}
			}

			// On an app row: install package (original behavior)
			if appIdx >= 0 && subIdx < 0 && !m.Applications[appIdx].IsFiltered {
				app := m.Applications[appIdx]
				if app.PkgInstalled != nil && !*app.PkgInstalled {
					m.Operation = OpInstallPackages
					m.currentPackageIndex = 0
					m.results = nil
					m.pendingPackages = []PackageItem{{
						Name:     app.Application.Name,
						Package:  app.Application.Package,
						Method:   app.PkgMethod,
						Selected: true,
					}}
					m.Screen = ScreenProgress
					return m, m.installNextPackage()
				}
			}
		}

		return m, nil
	case key.Matches(msg, ListKeys.Restore):
		// Restore selected SubEntry (only in List view for SubEntry rows)
		if m.Operation == OpList {
			m.pruneStaleSelections()
			// Check if multi-select mode is active
			if m.multiSelectActive {
				// Show summary screen for batch restore
				m.summaryOperation = OpRestore
				m.Screen = ScreenSummary
				return m, nil
			}

			// Single-item restore (original behavior)
			appIdx, subIdx := m.getApplicationAtCursorFromTable()
			if appIdx >= 0 && subIdx >= 0 && !m.Applications[appIdx].IsFiltered {
				// Restore single sub-entry
				subItem := &m.Applications[appIdx].SubItems[subIdx]

				// A setup entry runs a command that may prompt for a sudo
				// password, so it is dispatched through tea.Exec (which hands
				// the terminal over) instead of running inline here.
				if subItem.SubEntry.IsSetup() {
					m.results = nil
					m.completedOperation = OpList
					return m, m.startSetupRun([]setupRunItem{{
						appIdx: appIdx,
						subIdx: subIdx,
						name:   subItem.SubEntry.Name,
						sub:    *subItem,
					}}, false)
				}

				success, message := m.performRestoreSubEntry(*subItem)
				if success {
					m.Applications[appIdx].SubItems[subIdx].State = m.detectSubEntryState(subItem)
					m.rebuildTable()
				}
				m.results = []ResultItem{{
					Name:    subItem.SubEntry.Name,
					Success: success,
					Message: message,
				}}
				m.completedOperation = OpList
				m.showingResults = true
				m.resultsScrollOffset = 0
			} else if appIdx >= 0 && subIdx < 0 && !m.Applications[appIdx].IsFiltered {
				// Restore all sub-entries for this application: config entries
				// inline, setup entries queued for the tea.Exec runner.
				m.results = nil
				m.completedOperation = OpList
				for i := range m.Applications[appIdx].SubItems {
					subItem := &m.Applications[appIdx].SubItems[i]
					if subItem.SubEntry.IsSetup() || !subItem.SubEntry.IsConfig() {
						continue
					}
					success, message := m.performRestoreSubEntry(*subItem)
					if success {
						m.Applications[appIdx].SubItems[i].State = m.detectSubEntryState(subItem)
					}
					m.results = append(m.results, ResultItem{
						Name:    subItem.SubEntry.Name,
						Success: success,
						Message: message,
					})
				}
				m.rebuildTable()

				if setups := m.collectAppSetupItems(appIdx); len(setups) > 0 {
					return m, m.startSetupRun(setups, false)
				}

				m.showingResults = true
				m.resultsScrollOffset = 0
			}
		}

		return m, nil
	case key.Matches(msg, ListKeys.ForceRestore):
		if m.Operation == OpList {
			m.pruneStaleSelections()
			if m.multiSelectActive {
				m.summaryOperation = OpForceRestore
				m.Screen = ScreenSummary
				return m, nil
			}
			m.prepareCurrentRowSummary(OpForceRestore)
		}
		return m, nil
	case key.Matches(msg, ListKeys.Toggle):
		// Toggle selection and advance cursor (only in List view)
		if listClean {
			appIdx, subIdx := m.getApplicationAtCursorFromTable()
			if appIdx >= 0 {
				if subIdx >= 0 {
					// Toggle sub-entry selection
					m.toggleSubEntrySelection(appIdx, subIdx)
				} else {
					// Toggle application selection
					m.toggleAppSelection(appIdx)
				}
				// Move to next row
				m.moveToNextExpandedNode()
			}
			return m, nil
		}
	case key.Matches(msg, ListKeys.ShowResults):
		// Show results popup (only in List view with results available)
		if listClean && len(m.results) > 0 {
			m.showingResults = true
			m.resultsScrollOffset = 0
			return m, nil
		}
	}

	return m, nil
}

func (m Model) canRefreshAllStates() bool {
	return m.Operation == OpList &&
		!m.searching &&
		!m.confirmingDeleteApp &&
		!m.confirmingDeleteSubEntry &&
		!m.confirmingFilterToggle &&
		!m.confirmingActionFilter &&
		!m.showingDetail &&
		!m.showingResults &&
		m.pendingStateChecks == 0 &&
		!m.processing
}

func (m Model) stateChecksPending() bool {
	return m.pendingStateChecks > 0
}

func (m Model) viewResults() string {
	// Use table view for List operation
	if m.Operation == OpList {
		// Always use hierarchical viewListTable (supports both v2 and v3)
		return m.viewListTable()
	}

	var b strings.Builder

	// Title
	title := fmt.Sprintf("✓  %s Complete", m.Operation.String())
	b.WriteString(TitleStyle.Render(title))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(ErrorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n\n")
	}

	// Results summary
	successCount := 0
	failCount := 0

	for _, r := range m.results {
		if r.Success {
			successCount++
		} else {
			failCount++
		}
	}

	summary := fmt.Sprintf("%d successful", successCount)
	if failCount > 0 {
		summary += fmt.Sprintf(", %d failed", failCount)
	}

	if m.DryRun {
		summary = WarningStyle.Render("[DRY RUN] ") + summary
	}

	b.WriteString(SubtitleStyle.Render(summary))
	b.WriteString("\n\n")

	// Results list
	maxVisible := m.viewHeight
	if maxVisible > len(m.results) {
		maxVisible = len(m.results)
	}

	start := m.scrollOffset

	end := start + maxVisible
	if end > len(m.results) {
		end = len(m.results)

		start = end - maxVisible
		if start < 0 {
			start = 0
		}
	}

	topIndicator, bottomIndicator := RenderScrollIndicators(start, end, len(m.results))
	b.WriteString(topIndicator)

	for i := start; i < end; i++ {
		result := m.results[i]

		var icon string
		var nameStyle func(string) string

		if result.Success {
			icon = SuccessStyle.Render("✓ ")
			nameStyle = func(s string) string { return SuccessStyle.Render(s) }
		} else {
			icon = ErrorStyle.Render("✗ ")
			nameStyle = func(s string) string { return ErrorStyle.Render(s) }
		}

		b.WriteString(icon + nameStyle(result.Name))
		b.WriteString("\n")

		// Show message indented
		if result.Message != "" {
			lines := strings.Split(result.Message, "\n")
			for _, line := range lines {
				b.WriteString(IndentSpaces + SubtitleStyle.Render(line))
				b.WriteString("\n")
			}
		}
	}

	b.WriteString(bottomIndicator)

	// Help
	b.WriteString("\n")
	b.WriteString(RenderHelpFromBindings(m.width,
		ListKeys.NewOperation,
		ListKeys.QuitOrEnter,
	))

	return BaseStyle.Render(b.String())
}

// renderHelpForCurrentState returns the help text for the current screen state.
// This allows us to measure the help text height before rendering.
func (m Model) renderHelpForCurrentState() string {
	appIdx, subIdx := m.getApplicationAtCursorFromTable()

	switch {
	case m.confirmingDeleteApp || m.confirmingDeleteSubEntry:
		// Delete confirmation prompt
		var name string
		switch {
		case m.confirmingDeleteApp && appIdx >= 0:
			name = m.Applications[appIdx].Application.Name
		case m.confirmingDeleteSubEntry && appIdx >= 0 && subIdx >= 0:
			name = m.Applications[appIdx].SubItems[subIdx].SubEntry.Name
		}

		if name != "" {
			return WarningStyle.Render(fmt.Sprintf("Delete '%s'? ", name)) +
				RenderHelpFromBindings(m.width, ConfirmKeys.Yes, ConfirmKeys.No)
		}
		return RenderHelpFromBindings(m.width, ConfirmKeys.Yes, ConfirmKeys.No)

	case m.confirmingFilterToggle:
		// Filter toggle confirmation dialog
		itemText := "item(s)"
		if m.filterToggleHiddenCount == 1 {
			itemText = "item"
		}
		prompt := fmt.Sprintf("Enabling filter will hide %d selected %s. Continue? (y/n)",
			m.filterToggleHiddenCount, itemText)
		return WarningStyle.Render(prompt)

	case m.confirmingActionFilter:
		itemText := "item(s)"
		if m.actionFilterHiddenCount == 1 {
			itemText = "item"
		}
		prompt := fmt.Sprintf("Enabling action filter will hide %d selected %s. Continue? (y/n)",
			m.actionFilterHiddenCount, itemText)
		return WarningStyle.Render(prompt)

	case m.searching:
		return RenderHelpFromBindings(m.width,
			SearchKeys.Confirm,
			SearchKeys.Cancel,
		)

	case m.showingDiffPicker:
		return RenderHelpFromBindings(m.width,
			DiffPickerKeys.Select,
			DiffPickerKeys.Cancel,
		)

	case m.showingDetail:
		return RenderHelpFromBindings(m.width,
			DetailKeys.Close,
			SharedKeys.Quit,
		)

	case m.showingResults:
		return RenderHelpFromBindings(m.width,
			ResultsPopupKeys.Up,
			ResultsPopupKeys.Down,
			ResultsPopupKeys.Close,
		)

	default:
		// Build help text based on cursor position and multi-select mode
		if m.multiSelectActive {
			return RenderHelpFromBindings(m.width,
				MultiSelectKeys.Toggle,
				MultiSelectKeys.Clear,
				MultiSelectKeys.Restore,
				MultiSelectKeys.ForceRestore,
				MultiSelectKeys.Install,
				MultiSelectKeys.Delete,
				ListKeys.Refresh,
				ListKeys.ActionFilter,
				SharedKeys.Quit,
			)
		}

		// Normal mode help text
		bindings := []key.Binding{
			ListKeys.Search,
			ListKeys.ActionFilter,
			ListKeys.HalfPageUp,
			ListKeys.HalfPageDown,
			ListKeys.GoTop,
			ListKeys.GoBottom,
			ListKeys.AddApp,
			ListKeys.AddEntry,
			ListKeys.Edit,
			ListKeys.Delete,
			ListKeys.Restore,
			ListKeys.ForceRestore,
			ListKeys.Refresh,
		}

		// Show context-sensitive "i" help
		if subIdx < 0 {
			// App row: install
			bindings = append(bindings, ListKeys.Install)
		} else if appIdx >= 0 && subIdx >= 0 && appIdx < len(m.Applications) &&
			subIdx < len(m.Applications[appIdx].SubItems) &&
			m.Applications[appIdx].SubItems[subIdx].State == StateModified {
			// Modified sub-entry: diff
			diffBinding := key.NewBinding(key.WithKeys("i"), key.WithHelp("i", "diff"))
			bindings = append(bindings, diffBinding)
		}

		// Show "m" for messages if results exist
		if len(m.results) > 0 {
			bindings = append(bindings, ListKeys.ShowResults)
		}

		bindings = append(bindings, SharedKeys.Quit)
		return RenderHelpFromBindings(m.width, bindings...)
	}
}

//nolint:gocyclo // UI rendering with many states
func (m Model) viewListTable() string {
	var b strings.Builder

	// Filter banner (always shown) with search input right-aligned on the same line
	highlightedF := lipgloss.NewStyle().
		Foreground(accentColor).
		Bold(true).
		Render("f")

	var filterBanner string
	if m.filterEnabled {
		visibleCount := 0
		totalCount := len(m.Applications)

		for _, app := range m.Applications {
			if !app.IsFiltered {
				visibleCount++
			}
		}

		countInfo := MutedTextStyle.Render(fmt.Sprintf(" (showing %d of %d apps)", visibleCount, totalCount))
		filterBanner = "  " + highlightedF + "ilter: on" + countInfo
	} else {
		filterBanner = "  " + highlightedF + "ilter: off"
	}

	highlightedX := lipgloss.NewStyle().Foreground(accentColor).Bold(true).Render("x")
	if m.actionFilterEnabled {
		visibleCount := 0
		for _, app := range m.getFilteredApplications() {
			if m.filterEnabled && app.IsFiltered {
				continue
			}
			visibleCount++
		}
		filterBanner += "  " + highlightedX + fmt.Sprintf(" action filter: on (%d apps)", visibleCount)
	} else {
		filterBanner += "  " + highlightedX + " action filter: off"
	}

	// Append search input after the filter banner on the same line
	if m.searching || m.searchText != "" {
		var searchPart string
		if m.searching {
			searchPart = "/ " + m.searchInput.View()
		} else {
			searchPart = "/ " + FilterInputStyle.Render(m.searchText)
		}

		filterBanner += "    " + searchPart
	}

	b.WriteString(filterBanner)
	b.WriteString("\n")

	// Table should already be initialized via Update()/initTableModel()
	// Do not call initTableModel() from View() — mutating state in View is a Bubble Tea anti-pattern

	// Calculate available height using shared method (keeps Update and View in sync).
	// No minimum override — trust computeMaxVisibleRows() which clamps to 3 data rows.
	// TODO: computeMaxVisibleRows() also renders help/detail/diff to measure heights,
	// causing double computation. Acceptable while detail stubs return "".
	maxVisibleRows := m.computeMaxVisibleRows()
	availableForTable := maxVisibleRows + 4 // Add back table border lines

	// Derive content needed below the table for rendering
	detailContent := m.detailContent()

	var diffPickerContent string
	if m.showingDiffPicker {
		diffPickerContent = m.viewDiffPicker()
	}

	helpText := m.renderHelpForCurrentState()

	// Render table with exact available height
	b.WriteString(m.renderTable(availableForTable))
	b.WriteString("\n")

	// Multi-select banner (replaces one of two blank lines to keep help position consistent)
	if m.multiSelectActive {
		appCount, subCount := m.getSelectionCounts()
		bannerText := fmt.Sprintf("  %d app(s), %d item(s) selected", appCount, subCount)
		b.WriteString(MultiSelectBannerStyle.Render(bannerText))
	}

	// Detail panel
	if detailContent != "" {
		b.WriteString(detailContent)
		b.WriteString("\n")
	}

	// Diff picker panel
	if diffPickerContent != "" {
		b.WriteString("\n")
		b.WriteString(diffPickerContent)
	}

	// Help footer
	b.WriteString("\n")
	b.WriteString(helpText)

	return BaseStyle.Render(b.String())
}

// getSearchedApplications returns searched applications for hierarchical view.
// It always returns a fresh slice: initTableModel sorts the result in place,
// and m.Applications must keep its load-time order — the selection maps and
// every cursor action resolve against it. A shallow clone is enough because
// view code only reads SubItems, never reorders them.
func (m Model) getSearchedApplications() []ApplicationItem {
	if m.searchText == "" {
		return slices.Clone(m.Applications)
	}

	searchLower := strings.ToLower(m.searchText)
	var searched []ApplicationItem

	for _, app := range m.Applications {
		appMatches := strings.Contains(strings.ToLower(app.Application.Name), searchLower) ||
			strings.Contains(strings.ToLower(app.Application.Description), searchLower)

		// Search SubItems
		var matchingSubItems []SubEntryItem

		for _, sub := range app.SubItems {
			subMatches := strings.Contains(strings.ToLower(sub.SubEntry.Name), searchLower) ||
				strings.Contains(strings.ToLower(sub.Target), searchLower)

			// Check backup field
			subMatches = subMatches || strings.Contains(strings.ToLower(sub.SubEntry.Backup), searchLower)

			if appMatches || subMatches {
				matchingSubItems = append(matchingSubItems, sub)
			}
		}

		if appMatches || len(matchingSubItems) > 0 {
			appCopy := app
			appCopy.SubItems = matchingSubItems
			searched = append(searched, appCopy)
		}
	}

	return searched
}

// findConfigApplicationIndex finds the index of an application in m.Config.Applications by name
// This is needed because m.Applications is sorted but m.Config.Applications is not
func (m *Model) findConfigApplicationIndex(appName string) int {
	for i, app := range m.Config.Applications {
		if app.Name == appName {
			return i
		}
	}

	return -1
}

// findConfigSubEntryIndex finds the index of a sub-entry in
// m.Config.Applications[configAppIdx].Entries by name. A SubItems index cannot
// be used instead: SubItems skips entries that do not apply to the running OS.
func (m *Model) findConfigSubEntryIndex(configAppIdx int, subName string) int {
	for i, entry := range m.Config.Applications[configAppIdx].Entries {
		if entry.Name == subName {
			return i
		}
	}

	return -1
}

func (m Model) installNextPackage() tea.Cmd {
	if m.currentPackageIndex >= len(m.pendingPackages) {
		return nil
	}

	pkg := m.pendingPackages[m.currentPackageIndex]

	// Handle dry run
	if m.DryRun {
		return func() tea.Msg {
			return PackageInstallMsg{
				Package: pkg,
				Success: true,
				Message: fmt.Sprintf("Would install via %s", pkg.Method),
			}
		}
	}

	// Build the command
	cmd := m.buildInstallCommand(pkg)
	if cmd == nil {
		return func() tea.Msg {
			return PackageInstallMsg{
				Package: pkg,
				Success: false,
				Message: "No installation method available",
			}
		}
	}

	// Use tea.Exec to properly suspend the TUI and give terminal control to the command.
	// This allows sudo to prompt for password correctly.
	// pauseOnFailExec wraps the command to pause on failure so the user can read error output.
	return tea.Exec(&pauseOnFailExec{cmd: cmd}, func(err error) tea.Msg {
		if err != nil {
			return PackageInstallMsg{
				Package: pkg,
				Success: false,
				Message: fmt.Sprintf("Installation failed: %v", err),
				Err:     err,
			}
		}

		return PackageInstallMsg{
			Package: pkg,
			Success: true,
			Message: fmt.Sprintf("Installed via %s", pkg.Method),
		}
	})
}

func (m Model) buildInstallCommand(pkg PackageItem) *exec.Cmd {
	converted := packages.FromPackageSpec(pkg.Name, pkg.Package)
	if converted == nil {
		return nil
	}

	return packages.BuildCommand(context.Background(), *converted, pkg.Method, m.Platform.OS)
}
