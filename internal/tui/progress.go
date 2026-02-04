package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AntoineGS/dot-manager/internal/config"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
)

// sortTableRows sorts the table rows based on the current sort column and direction.
// It preserves the application order from the existing tableRows and only sorts sub-entries.
func (m *Model) sortTableRows() {
	if len(m.tableRows) == 0 {
		return
	}

	// Group rows by application
	type appGroup struct {
		appRow     TableRow
		subEntries []TableRow
	}

	groups := make(map[int]*appGroup)
	var appIndices []int

	for _, row := range m.tableRows {
		if _, exists := groups[row.AppIndex]; !exists {
			groups[row.AppIndex] = &appGroup{}
			// appIndices are added in the order they appear in tableRows,
			// which preserves the current visual order
			appIndices = append(appIndices, row.AppIndex)
		}

		if row.SubIndex == -1 {
			groups[row.AppIndex].appRow = row
		} else {
			groups[row.AppIndex].subEntries = append(groups[row.AppIndex].subEntries, row)
		}
	}

	// NOTE: We do NOT sort appIndices here. The appIndices array already
	// represents the current visual order from tableRows. Re-sorting would
	// cause applications to jump positions when expanding/collapsing.
	//
	// Application sorting happens in initTableModel by sorting the filtered
	// applications BEFORE calling flattenApplications.

	// Sort sub-entries within each app
	for _, group := range groups {
		if len(group.subEntries) > 0 {
			sort.SliceStable(group.subEntries, func(i, j int) bool {
				var less bool
				switch m.sortColumn {
				case SortColumnName:
					less = strings.ToLower(group.subEntries[i].Data[0]) < strings.ToLower(group.subEntries[j].Data[0])
				case SortColumnStatus:
					less = strings.ToLower(group.subEntries[i].Data[1]) < strings.ToLower(group.subEntries[j].Data[1])
				case SortColumnPath:
					less = strings.ToLower(group.subEntries[i].Data[3]) < strings.ToLower(group.subEntries[j].Data[3])
				default:
					less = strings.ToLower(group.subEntries[i].Data[0]) < strings.ToLower(group.subEntries[j].Data[0])
				}

				if m.sortAscending {
					return less
				}
				return !less
			})
		}
	}

	// Rebuild tableRows in sorted order
	m.tableRows = make([]TableRow, 0, len(m.tableRows))
	for _, appIdx := range appIndices {
		group := groups[appIdx]
		m.tableRows = append(m.tableRows, group.appRow)
		m.tableRows = append(m.tableRows, group.subEntries...)
	}

	// Fix tree characters after sorting
	m.fixTreeCharacters()
}

// fixTreeCharacters recalculates tree characters (├─ vs └─) after sorting
func (m *Model) fixTreeCharacters() {
	for i := range m.tableRows {
		if m.tableRows[i].SubIndex == -1 {
			// App row - skip
			continue
		}

		// Find if this is the last sub-entry for its app
		isLast := true
		for j := i + 1; j < len(m.tableRows); j++ {
			if m.tableRows[j].AppIndex != m.tableRows[i].AppIndex {
				// Different app, we're done
				break
			}
			if m.tableRows[j].SubIndex != -1 {
				// Found another sub-entry for the same app
				isLast = false
				break
			}
		}

		// Update tree character
		if isLast {
			m.tableRows[i].TreeChar = "└─"
			m.tableRows[i].Data[0] = "  └─ " + strings.TrimPrefix(
				strings.TrimPrefix(m.tableRows[i].Data[0], "  ├─ "),
				"  └─ ",
			)
		} else {
			m.tableRows[i].TreeChar = "├─"
			m.tableRows[i].Data[0] = "  ├─ " + strings.TrimPrefix(
				strings.TrimPrefix(m.tableRows[i].Data[0], "  ├─ "),
				"  └─ ",
			)
		}
	}
}

// initApplicationItems creates ApplicationItem list from v3 config
func (m *Model) initApplicationItems() {
	// Get ALL applications, not just filtered ones
	apps := m.Config.Applications

	m.Applications = make([]ApplicationItem, 0, len(apps))

	for _, app := range apps {
		// Check if this app matches the filter
		isFiltered := !config.MatchesFilters(app.Filters, m.FilterCtx)

		subItems := make([]SubEntryItem, 0, len(app.Entries))

		for _, subEntry := range app.Entries {
			target := subEntry.GetTarget(m.Platform.OS)
			if target == "" {
				continue
			}

			// Expand ~ and env vars in target path for file operations
			expandedTarget := config.ExpandPath(target, m.Platform.EnvVars)

			subItem := SubEntryItem{
				SubEntry: subEntry,
				Target:   expandedTarget,
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
			IsFiltered:  isFiltered,
		}

		// Add package info
		if app.HasPackage() {
			method := getPackageInstallMethodFromPackage(app.Package, m.Platform.OS)
			appItem.PkgMethod = method

			if method != "none" {
				installed := isPackageInstalledFromPackage(app.Package, method, app.Name)
				appItem.PkgInstalled = &installed
			}
		}

		m.Applications = append(m.Applications, appItem)
	}

	// Sort applications alphabetically by name
	sort.Slice(m.Applications, func(i, j int) bool {
		return m.Applications[i].Application.Name < m.Applications[j].Application.Name
	})

	// Detect states for all sub-items
	m.refreshApplicationStates()

	// Initialize table model with the loaded applications
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

// initTableModel initializes the table data and cursor
func (m *Model) initTableModel() {
	// Flatten hierarchical data with current search
	filtered := m.getSearchedApplications()

	// Sort applications before flattening (only if sort column applies to apps)
	if m.sortColumn == SortColumnName || m.sortColumn == SortColumnStatus {
		sort.SliceStable(filtered, func(i, j int) bool {
			var less bool
			if m.sortColumn == SortColumnName {
				less = strings.ToLower(filtered[i].Application.Name) < strings.ToLower(filtered[j].Application.Name)
			} else { // SortColumnStatus
				statusI := getApplicationStatus(filtered[i])
				statusJ := getApplicationStatus(filtered[j])
				less = strings.ToLower(statusI) < strings.ToLower(statusJ)
			}

			if m.sortAscending {
				return less
			}
			return !less
		})
	}

	m.tableRows = flattenApplications(filtered, m.Platform.OS)

	// Apply sorting (only sorts sub-entries now, preserves app order)
	m.sortTableRows()

	// Ensure cursor is within bounds
	if m.tableCursor >= len(m.tableRows) {
		if len(m.tableRows) > 0 {
			m.tableCursor = len(m.tableRows) - 1
		} else {
			m.tableCursor = 0
		}
	}
}

// rebuildTable rebuilds the table with current data (after expand/collapse or search changes)
func (m *Model) rebuildTable() {
	// Save current cursor position
	currentCursor := m.tableCursor

	// Rebuild table with new data
	m.initTableModel()

	// Restore cursor if still valid
	if currentCursor < len(m.tableRows) {
		m.tableCursor = currentCursor
	} else if len(m.tableRows) > 0 {
		m.tableCursor = len(m.tableRows) - 1
	}
}

// formatHeaderWithShortcut creates a header string with highlighted shortcut letter and sort indicator
func (m *Model) formatHeaderWithShortcut(text string, shortcut rune, columnName string) string {
	runes := []rune(text)
	var result string

	for i, r := range runes {
		if r == shortcut {
			before := string(runes[:i])
			// Use accent color for the highlighted letter
			highlighted := lipgloss.NewStyle().
				Foreground(accentColor).
				Bold(true).
				Render(string(shortcut))
			after := string(runes[i+1:])
			result = before + highlighted + after
			break
		}
	}

	if result == "" {
		result = text
	}

	// Add sort indicator if this column is currently sorted
	if m.sortColumn == columnName {
		indicator := " ↑"
		if !m.sortAscending {
			indicator = " ↓"
		}
		result += lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true).
			Render(indicator)
	}

	return result
}

// renderTable renders the table using lipgloss with custom styling
func (m *Model) renderTable() string {
	if len(m.tableRows) == 0 {
		return SubtitleStyle.Render("No entries found")
	}

	// Determine if we have enough width to show backup column
	// Width threshold: 140 chars allows for all columns including backup
	showBackupColumn := m.width >= 140

	// Build headers with highlighted shortcuts and sort indicators
	var headers []string
	if showBackupColumn {
		headers = []string{
			m.formatHeaderWithShortcut("name", 'n', SortColumnName),
			m.formatHeaderWithShortcut("status", 's', SortColumnStatus),
			"info",
			"backup",
			m.formatHeaderWithShortcut("path", 'p', SortColumnPath),
		}
	} else {
		headers = []string{
			m.formatHeaderWithShortcut("name", 'n', SortColumnName),
			m.formatHeaderWithShortcut("status", 's', SortColumnStatus),
			"info",
			m.formatHeaderWithShortcut("path", 'p', SortColumnPath),
		}
	}

	// Convert tableRows to string data for lipgloss table
	rows := make([][]string, len(m.tableRows))
	for i, tr := range m.tableRows {
		if showBackupColumn {
			rows[i] = []string{
				tr.Data[0],    // Name (with tree chars)
				tr.Data[1],    // Status
				tr.Data[2],    // Info
				tr.BackupPath, // Backup path
				tr.Data[3],    // Path (target)
			}
		} else {
			rows[i] = []string{
				tr.Data[0], // Name (with tree chars)
				tr.Data[1], // Status
				tr.Data[2], // Info
				tr.Data[3], // Path
			}
		}
	}

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(primaryColor)).
		Headers(headers...).
		Rows(rows...).
		BorderHeader(true).
		Width(m.width - 4).
		StyleFunc(func(row, col int) lipgloss.Style {
			switch row {
			case table.HeaderRow:
				// Header styling
				return lipgloss.NewStyle().
					Bold(true).
					Padding(0, 1)
			case m.tableCursor:
				// Cursor row styling (always takes priority)
				return lipgloss.NewStyle().
					Foreground(textColor).
					Background(primaryColor).
					Bold(true).
					Padding(0, 1)
			default:
				// Check if this row is selected (multi-select)
				if row >= 0 && row < len(m.tableRows) {
					tr := m.tableRows[row]
					appIdx := tr.AppIndex
					subIdx := tr.SubIndex

					isSelected := false
					if subIdx < 0 {
						// Application row
						isSelected = m.isAppSelected(appIdx)
					} else {
						// Sub-entry row
						isSelected = m.isSubEntrySelected(appIdx, subIdx)
					}

					if isSelected {
						// Selected row styling (lighter purple)
						return SelectedRowStyle
					}

					// Regular cell styling - check if needs attention
					baseStyle := lipgloss.NewStyle().Padding(0, 1)

					// Column 1 is status, column 2 is info (regardless of backup column)
					if col == 1 && tr.StatusAttention {
						return baseStyle.Foreground(errorColor)
					}
					if col == 2 && tr.InfoAttention {
						return baseStyle.Foreground(errorColor)
					}

					return baseStyle
				}

				return lipgloss.NewStyle().Padding(0, 1)
			}
		})

	return t.Render()
}

// detectSubEntryState determines the state of a sub-entry item
func (m *Model) detectSubEntryState(item *SubEntryItem) PathState {
	// Similar to detectPathState but for SubEntry
	// Expand ~ in target path for file operations
	targetPath := config.ExpandPath(item.Target, m.Platform.EnvVars)

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
	checkedAnyFile := false

	for _, file := range item.SubEntry.Files {
		srcFile := filepath.Join(backupPath, file)
		dstFile := filepath.Join(targetPath, file)

		// Skip files that don't exist in backup (shouldn't affect state)
		if !pathExists(srcFile) {
			continue
		}

		checkedAnyFile = true
		anyBackup = true

		if info, err := os.Lstat(dstFile); err == nil {
			anyTarget = true
			if info.Mode()&os.ModeSymlink == 0 {
				allLinked = false
			}
		} else {
			allLinked = false
		}
	}

	// If all existing backup files are symlinked at target
	if allLinked && checkedAnyFile {
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

// getApplicationAtCursor returns the application and sub-entry indices for the current cursor position
func (m *Model) getApplicationAtCursor() (int, int) {
	visualRow := 0
	filtered := m.getSearchedApplications()

	for _, fapp := range filtered {
		if visualRow == m.appCursor {
			// Find the real index in m.Applications
			for appIdx, app := range m.Applications {
				if app.Application.Name == fapp.Application.Name {
					return appIdx, -1
				}
			}
		}

		visualRow++

		if fapp.Expanded {
			for fsubIdx, fsub := range fapp.SubItems {
				if visualRow == m.appCursor {
					// Find the real indices in m.Applications
					for appIdx, app := range m.Applications {
						if app.Application.Name == fapp.Application.Name {
							// Find the sub-entry index by name
							for subIdx, sub := range app.SubItems {
								if sub.SubEntry.Name == fsub.SubEntry.Name {
									return appIdx, subIdx
								}
							}
							// If not found (shouldn't happen), return with the filtered sub index
							return appIdx, fsubIdx
						}
					}
				}

				visualRow++
			}
		}
	}

	return -1, -1
}

// getApplicationAtCursorFromTable returns the application and sub-entry indices from table cursor
func (m *Model) getApplicationAtCursorFromTable() (int, int) {
	if m.tableCursor < 0 || m.tableCursor >= len(m.tableRows) {
		return -1, -1
	}

	tr := m.tableRows[m.tableCursor]
	return tr.AppIndex, tr.SubIndex
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
func (m Model) updateResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle search mode input
	if m.Operation == OpList && m.searching {
		switch msg.String() {
		case KeyEsc:
			// Clear search and exit search mode
			m.searching = false
			m.searchText = ""
			m.searchInput.SetValue("")
			m.searchInput.Blur()
			// Rebuild table without search
			m.rebuildTable()

			return m, nil
		case KeyEnter:
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

	// Handle delete confirmation
	if m.Operation == OpList && (m.confirmingDeleteApp || m.confirmingDeleteSubEntry) {
		switch msg.String() {
		case "y", "Y", KeyEnter:
			// Confirm delete
			appIdx, subIdx := m.getApplicationAtCursorFromTable()
			if m.confirmingDeleteApp && appIdx >= 0 {
				m.confirmingDeleteApp = false
				if err := m.deleteApplication(appIdx); err == nil {
					// Rebuild table after deletion
					m.rebuildTable()
				}
			} else if m.confirmingDeleteSubEntry && appIdx >= 0 && subIdx >= 0 {
				m.confirmingDeleteSubEntry = false
				if err := m.deleteSubEntry(appIdx, subIdx); err == nil {
					// Rebuild table after deletion
					m.rebuildTable()
				}
			}

			return m, nil
		case "n", "N", KeyEsc:
			// Cancel delete
			m.confirmingDeleteApp = false
			m.confirmingDeleteSubEntry = false

			return m, nil
		}

		return m, nil
	}

	// Handle detail popup separately
	if m.Operation == OpList && m.showingDetail {
		switch msg.String() {
		case KeyEsc, KeyEnter:
			// Close detail popup (ESC cancels/closes the popup)
			m.showingDetail = false
			return m, nil
		case "q":
			// q quits the application
			return m, tea.Quit
		}

		return m, nil
	}

	// Handle ESC to clear active search or selections (when not in search mode but search text or selections are present)
	if m.Operation == OpList && msg.String() == KeyEsc && !m.searching {
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

	switch msg.String() {
	case "/":
		// Enter search mode
		if m.Operation == OpList && !m.confirmingDeleteApp && !m.confirmingDeleteSubEntry && !m.showingDetail {
			m.searching = true
			m.searchInput.Focus()

			return m, nil
		}
	case "n":
		// Sort by name
		if m.Operation == OpList && !m.searching && !m.confirmingDeleteApp && !m.confirmingDeleteSubEntry && !m.showingDetail {
			if m.sortColumn == SortColumnName {
				m.sortAscending = !m.sortAscending
			} else {
				m.sortColumn = SortColumnName
				m.sortAscending = true
			}
			m.rebuildTable()
			return m, nil
		}
	case "s":
		// Sort by status
		if m.Operation == OpList && !m.searching && !m.confirmingDeleteApp && !m.confirmingDeleteSubEntry && !m.showingDetail {
			if m.sortColumn == SortColumnStatus {
				m.sortAscending = !m.sortAscending
			} else {
				m.sortColumn = SortColumnStatus
				m.sortAscending = true
			}
			m.rebuildTable()
			return m, nil
		}
	case "p":
		// Sort by path
		if m.Operation == OpList && !m.searching && !m.confirmingDeleteApp && !m.confirmingDeleteSubEntry && !m.showingDetail {
			if m.sortColumn == SortColumnPath {
				m.sortAscending = !m.sortAscending
			} else {
				m.sortColumn = SortColumnPath
				m.sortAscending = true
			}
			m.rebuildTable()
			return m, nil
		}
	case "q":
		// Quit the application
		return m, tea.Quit
	case "up", "k":
		if m.Operation == OpList {
			// Clear any previous restore results when navigating
			m.results = nil
			// Move cursor up
			if m.tableCursor > 0 {
				m.tableCursor--
			}
			return m, nil
		}

		return m, nil
	case "down", "j":
		if m.Operation == OpList {
			// Clear any previous restore results when navigating
			m.results = nil
			// Move cursor down
			if m.tableCursor < len(m.tableRows)-1 {
				m.tableCursor++
			}
			return m, nil
		}

		return m, nil
	case "h", "left":
		if m.Operation == OpList {
			// Clear any previous restore results when navigating
			m.results = nil
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
	case KeyEnter, "l", "right":
		if m.Operation == OpList {
			// Clear any previous restore results when navigating
			m.results = nil
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
	case "e":
		// Edit selected Application or SubEntry (only in List view)
		if m.Operation == OpList {
			appIdx, subIdx := m.getApplicationAtCursorFromTable()
			if appIdx >= 0 {
				if subIdx >= 0 {
					// Edit SubEntry
					m.initSubEntryFormEdit(appIdx, subIdx)
				} else {
					// Edit Application
					m.initApplicationFormEdit(appIdx)
				}

				return m, nil
			}
		}
	case "A":
		// Add new Application (only in List view)
		if m.Operation == OpList {
			m.initApplicationFormNew()
			return m, nil
		}
	case "a":
		// Add new SubEntry to current Application (only in List view)
		if m.Operation == OpList {
			appIdx, _ := m.getApplicationAtCursorFromTable()
			if appIdx >= 0 {
				m.initSubEntryFormNew(appIdx)
				return m, nil
			}
		}
	case "d", "delete", "backspace":
		// Ask for delete confirmation (only in List view)
		if m.Operation == OpList {
			// Check if multi-select mode is active
			if m.multiSelectActive {
				// Show summary screen for batch delete
				// No specific delete operation type exists, so we use OpList with a different summary
				m.summaryOperation = OpList // Will be interpreted as delete in summary screen
				m.Screen = ScreenSummary
				return m, nil
			}

			// Single-item delete (original behavior)
			appIdx, subIdx := m.getApplicationAtCursorFromTable()
			if appIdx >= 0 {
				if subIdx >= 0 {
					m.confirmingDeleteSubEntry = true
				} else {
					m.confirmingDeleteApp = true
				}

				return m, nil
			}
		}
	case "i":
		// Install package at Application level (only in List view)
		if m.Operation == OpList {
			// Check if multi-select mode is active
			if m.multiSelectActive {
				// Show summary screen for batch install
				m.summaryOperation = OpInstallPackages
				m.Screen = ScreenSummary
				return m, nil
			}

			// Single-item install (original behavior)
			appIdx, _ := m.getApplicationAtCursorFromTable()
			if appIdx >= 0 {
				app := m.Applications[appIdx]
				if app.PkgInstalled != nil && !*app.PkgInstalled {
					// Setup for package installation
					m.Operation = OpInstallPackages
					// TODO: Convert Application to PackageItem format
					// For now, we need the Application's package spec
					m.currentPackageIndex = 0
					m.results = nil
					m.Screen = ScreenProgress

					return m, m.installNextPackage()
				}
			}
		}

		return m, nil
	case "r":
		// Restore selected SubEntry (only in List view for SubEntry rows)
		if m.Operation == OpList {
			// Check if multi-select mode is active
			if m.multiSelectActive {
				// Show summary screen for batch restore
				m.summaryOperation = OpRestore
				m.Screen = ScreenSummary
				return m, nil
			}

			// Single-item restore (original behavior)
			appIdx, subIdx := m.getApplicationAtCursorFromTable()
			if appIdx >= 0 && subIdx >= 0 {
				subItem := &m.Applications[appIdx].SubItems[subIdx]
				// Ensure Manager is in real mode (not dry-run) for Manage screen restores
				originalDryRun := m.Manager.DryRun
				m.Manager.DryRun = false
				// Perform restore using SubEntry data
				success, message := m.performRestoreSubEntry(subItem.SubEntry, subItem.Target)
				// Restore original dry-run state
				m.Manager.DryRun = originalDryRun
				// Update the state after restore
				if success {
					m.Applications[appIdx].SubItems[subIdx].State = m.detectSubEntryState(subItem)
					// Rebuild table to reflect updated state
					m.rebuildTable()
				}
				// Show result briefly in results
				m.results = []ResultItem{{
					Name:    subItem.SubEntry.Name,
					Success: success,
					Message: message,
				}}
			}
		}

		return m, nil
	case "tab", " ":
		// Toggle selection and advance cursor (only in List view)
		if m.Operation == OpList && !m.searching && !m.confirmingDeleteApp && !m.confirmingDeleteSubEntry && !m.showingDetail {
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
	}

	return m, nil
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
	b.WriteString(RenderHelp(
		"r", "new operation",
		"q/enter", "quit",
	))

	return BaseStyle.Render(b.String())
}

//nolint:gocyclo // UI rendering with many states
func (m Model) viewListTable() string {
	var b strings.Builder

	// Title
	b.WriteString(TitleStyle.Render("󰋗  Manage"))
	b.WriteString("\n")

	// Search input (show when searching or when search is active)
	if m.searching || m.searchText != "" {
		b.WriteString("  / ")

		if m.searching {
			b.WriteString(m.searchInput.View())
		} else {
			b.WriteString(FilterInputStyle.Render(m.searchText))
		}

		b.WriteString("\n")
	}

	// Multi-select banner (show when selections exist)
	if m.multiSelectActive {
		appCount, subCount := m.getSelectionCounts()
		bannerText := fmt.Sprintf("  %d app(s), %d item(s) selected", appCount, subCount)
		b.WriteString(MultiSelectBannerStyle.Render(bannerText))
		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Initialize table if not already initialized
	if len(m.tableRows) == 0 {
		m.initTableModel()
	}

	// Render table
	b.WriteString(m.renderTable())
	b.WriteString("\n")

	// Show inline detail panel below table if needed
	appIdx, subIdx := m.getApplicationAtCursorFromTable()
	if m.showingDetail && appIdx >= 0 {
		if subIdx >= 0 {
			// Detail for SubEntry
			b.WriteString(m.renderSubEntryInlineDetail(&m.Applications[appIdx].SubItems[subIdx], m.width))
		} else {
			// Detail for Application
			filtered := m.getSearchedApplications()
			if appIdx < len(filtered) {
				b.WriteString(m.renderApplicationInlineDetail(&filtered[appIdx], m.width))
			}
		}
		b.WriteString("\n")
	}

	// Show restore result if present
	if len(m.results) > 0 {
		b.WriteString("\n")
		result := m.results[len(m.results)-1] // Show most recent result
		var resultText string
		if result.Success {
			resultText = SuccessStyle.Render(fmt.Sprintf("✓ %s: %s", result.Name, result.Message))
		} else {
			resultText = ErrorStyle.Render(fmt.Sprintf("✗ %s: %s", result.Name, result.Message))
		}
		b.WriteString(resultText)
	}

	// Help or confirmation prompt
	b.WriteString("\n")

	switch {
	case m.confirmingDeleteApp || m.confirmingDeleteSubEntry:
		// Show delete confirmation prompt
		var name string
		switch {
		case m.confirmingDeleteApp && appIdx >= 0:
			name = m.Applications[appIdx].Application.Name
		case m.confirmingDeleteSubEntry && appIdx >= 0 && subIdx >= 0:
			name = m.Applications[appIdx].SubItems[subIdx].SubEntry.Name
		}

		if name != "" {
			b.WriteString(WarningStyle.Render(fmt.Sprintf("Delete '%s'? ", name)))
			b.WriteString(RenderHelpWithWidth(m.width, "y/enter", "yes", "n/esc", "no"))
		}
	case m.searching:
		b.WriteString(RenderHelpWithWidth(m.width,
			"enter", "confirm",
			"esc", "clear",
		))
	case m.showingDetail:
		b.WriteString(RenderHelpWithWidth(m.width,
			"h/←/esc", "close",
			"q", "menu",
		))
	default:
		// Build help text based on cursor position and multi-select mode
		var helpItems []string

		if m.multiSelectActive {
			// Multi-select mode help text
			helpItems = []string{
				"tab", "toggle",
				"esc", "clear",
				"r", "batch restore",
				"i", "batch install",
				"d", "batch delete",
				"q", "menu",
			}
		} else {
			// Normal mode help text
			helpItems = []string{
				"/", "search",
				"A", "add app",
				"a", "add entry",
				"e", "edit",
				"d", "delete",
				"r", "restore",
			}

			// Only show "i install" when on level 1 (application), not on level 2 (sub-entry)
			if subIdx < 0 {
				helpItems = append(helpItems, "i", "install")
			}

			helpItems = append(helpItems, "q", "menu")
		}

		b.WriteString(RenderHelpWithWidth(m.width, helpItems...))
	}

	return BaseStyle.Render(b.String())
}

// getSearchedApplications returns searched applications for hierarchical view
func (m Model) getSearchedApplications() []ApplicationItem {
	if m.searchText == "" {
		return m.Applications
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

		if len(matchingSubItems) > 0 {
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
