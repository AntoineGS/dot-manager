package tui

import (
	"fmt"
	"os/exec"
	"sort"
	"strings"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/manager"
	"github.com/AntoineGS/tidydots/internal/packages"
	"github.com/AntoineGS/tidydots/internal/platform"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
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
		// Check if this app matches the when expression
		isFiltered := !config.EvaluateWhen(app.When, m.Renderer)

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

		// Skip apps with no applicable entries AND no packages
		if len(subItems) == 0 && !app.HasPackage() {
			continue
		}

		appItem := ApplicationItem{
			Application: app,
			Selected:    true,
			Expanded:    false,
			SubItems:    subItems,
			IsFiltered:  isFiltered,
		}

		// Package method and install check deferred to async

		m.Applications = append(m.Applications, appItem)
	}

	// Sort applications alphabetically by name
	sort.Slice(m.Applications, func(i, j int) bool {
		return m.Applications[i].Application.Name < m.Applications[j].Application.Name
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

	m.tableRows = flattenApplications(filtered, m.Platform.OS, m.filterEnabled)

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

// updateScrollOffset recalculates scrollOffset based on current cursor position.
// This should be called after cursor movements in Update() to maintain scroll state.
func (m *Model) updateScrollOffset() {
	// Estimate available rows: height minus chrome (header, borders, help, etc.)
	// Using a conservative estimate that matches typical rendering
	maxVisibleRows := m.height - 12 // Account for header, borders, help, filter line, etc.
	if maxVisibleRows < 3 {
		maxVisibleRows = 3
	}

	totalRows := len(m.tableRows)
	if totalRows == 0 {
		m.scrollOffset = 0
		return
	}

	scrollOffset := m.scrollOffset

	// Ensure scroll offset is valid
	if scrollOffset < 0 {
		scrollOffset = 0
	}
	if scrollOffset > totalRows-maxVisibleRows && totalRows > maxVisibleRows {
		scrollOffset = totalRows - maxVisibleRows
	}
	if totalRows <= maxVisibleRows {
		scrollOffset = 0
		m.scrollOffset = scrollOffset
		return
	}

	// Calculate cursor position relative to viewport
	cursorPosInViewport := m.tableCursor - scrollOffset

	// Smooth scrolling: keep cursor within buffer zone from edges
	if cursorPosInViewport < ScrollOffsetMargin {
		// Cursor too close to top - scroll up to maintain buffer
		scrollOffset = m.tableCursor - ScrollOffsetMargin
		if scrollOffset < 0 {
			scrollOffset = 0
		}
	} else if cursorPosInViewport >= maxVisibleRows-ScrollOffsetMargin {
		// Cursor too close to bottom - scroll down to maintain buffer
		scrollOffset = m.tableCursor - maxVisibleRows + ScrollOffsetMargin + 1
		if scrollOffset+maxVisibleRows > totalRows {
			scrollOffset = totalRows - maxVisibleRows
			if scrollOffset < 0 {
				scrollOffset = 0
			}
		}
	}

	m.scrollOffset = scrollOffset
}

// renderTable renders the table using lipgloss with custom styling.
// availableHeight is the number of lines available for the entire table (including borders/headers).
func (m *Model) renderTable(availableHeight int) string {
	if len(m.tableRows) == 0 {
		return SubtitleStyle.Render("No entries found")
	}

	// Determine if we have enough width to show backup column
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

	// Calculate viewport from provided height
	// Table structure uses 4 lines (top border, header, separator, bottom border)
	maxVisibleRows := availableHeight - 4
	if maxVisibleRows < 3 {
		maxVisibleRows = 3 // Absolute minimum
	}

	totalRows := len(m.tableRows)

	// Implement smooth incremental scrolling with buffer zone (like vim's scrolloff)
	scrollOffset := m.scrollOffset

	// Ensure scroll offset is valid
	if scrollOffset < 0 {
		scrollOffset = 0
	}
	if scrollOffset > totalRows-maxVisibleRows && totalRows > maxVisibleRows {
		scrollOffset = totalRows - maxVisibleRows
	}
	if totalRows <= maxVisibleRows {
		scrollOffset = 0 // Show all rows if they fit
	}

	// Calculate cursor position relative to viewport
	cursorPosInViewport := m.tableCursor - scrollOffset

	// Smooth scrolling: keep cursor within buffer zone from edges
	if cursorPosInViewport < ScrollOffsetMargin {
		// Cursor too close to top - scroll up to maintain buffer
		scrollOffset = m.tableCursor - ScrollOffsetMargin
		if scrollOffset < 0 {
			scrollOffset = 0
		}
	} else if cursorPosInViewport >= maxVisibleRows-ScrollOffsetMargin {
		// Cursor too close to bottom - scroll down to maintain buffer
		scrollOffset = m.tableCursor - maxVisibleRows + ScrollOffsetMargin + 1
		if scrollOffset+maxVisibleRows > totalRows {
			scrollOffset = totalRows - maxVisibleRows
			if scrollOffset < 0 {
				scrollOffset = 0
			}
		}
	}

	// Save scroll offset for next render
	m.scrollOffset = scrollOffset

	// Calculate visible range
	visibleStart := scrollOffset
	visibleEnd := scrollOffset + maxVisibleRows
	if visibleEnd > totalRows {
		visibleEnd = totalRows
	}

	// Determine if we need scroll indicators
	hasMoreAbove := scrollOffset > 0
	hasMoreBelow := visibleEnd < totalRows

	// Build visible rows with scroll indicators embedded
	rows := m.buildVisibleRowsWithIndicators(
		visibleStart,
		visibleEnd,
		hasMoreAbove,
		hasMoreBelow,
		showBackupColumn,
	)

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(primaryColor)).
		Headers(headers...).
		Rows(rows...).
		BorderHeader(true).
		Width(m.width - 4).
		StyleFunc(func(row, col int) lipgloss.Style {
			// Header row styling
			if row == table.HeaderRow {
				return lipgloss.NewStyle().
					Bold(true).
					Padding(0, 1)
			}

			// Determine if this is an indicator row
			isIndicatorRow := (hasMoreAbove && row == 0) || (hasMoreBelow && row == len(rows)-1)

			// Indicator rows get muted styling
			if isIndicatorRow {
				return lipgloss.NewStyle().
					Foreground(lipgloss.Color("240")). // Dim gray
					Italic(true).
					Padding(0, 1)
			}

			// Calculate actual table row index accounting for indicators
			// When hasMoreAbove: row 0 = indicator, row 1 = tableRows[scrollOffset+1], etc.
			// So we need to map: row N (N >= 1) -> tableRows[scrollOffset + N]
			actualRow := row + scrollOffset

			// Bounds check
			if actualRow < 0 || actualRow >= len(m.tableRows) {
				return lipgloss.NewStyle().Padding(0, 1)
			}

			// Cursor row styling (takes priority)
			if actualRow == m.tableCursor {
				return lipgloss.NewStyle().
					Foreground(textColor).
					Background(primaryColor).
					Bold(true).
					Padding(0, 1)
			}

			// Multi-select styling
			tr := m.tableRows[actualRow]
			appIdx := tr.AppIndex
			subIdx := tr.SubIndex

			isSelected := false
			if subIdx < 0 {
				isSelected = m.isAppSelected(appIdx)
			} else {
				isSelected = m.isSubEntrySelected(appIdx, subIdx)
			}

			if isSelected {
				return SelectedRowStyle
			}

			return cellAttentionStyle(tr, col)
		})

	return t.Render()
}

// cellAttentionStyle returns the appropriate style for a table cell based on
// its attention state (status/info columns with outdated or error colors).
func cellAttentionStyle(tr TableRow, col int) lipgloss.Style {
	baseStyle := lipgloss.NewStyle().Padding(0, 1)

	if col == 1 && tr.StatusAttention {
		if tr.State == StateOutdated || tr.Data[1] == StatusOutdated {
			return baseStyle.Foreground(accentColor)
		}
		if tr.State == StateModified || tr.Data[1] == StatusModified {
			return baseStyle.Foreground(lipgloss.Color("#3B82F6"))
		}
		return baseStyle.Foreground(errorColor)
	}
	if col == 2 && tr.InfoAttention {
		switch {
		case stateSeverity(tr.InfoState) >= 3:
			return baseStyle.Foreground(errorColor)
		case tr.InfoState == StateOutdated:
			return baseStyle.Foreground(accentColor)
		case tr.InfoState == StateModified:
			return baseStyle.Foreground(lipgloss.Color("#3B82F6"))
		default:
			return baseStyle.Foreground(errorColor)
		}
	}

	// Mute "Unknown", "Loading..." status and "0 entries" info text
	if col == 1 && (tr.Data[1] == StatusUnknown || tr.Data[1] == StatusLoading) {
		return baseStyle.Foreground(mutedColor)
	}
	if col == 2 && tr.Data[2] == "0 entries" {
		return baseStyle.Foreground(mutedColor)
	}

	return baseStyle
}

// buildVisibleRowsWithIndicators builds the visible table rows with scroll
// indicators embedded as the first/last rows when scrolling
func (m *Model) buildVisibleRowsWithIndicators(
	visibleStart, visibleEnd int,
	hasMoreAbove, hasMoreBelow bool,
	showBackupColumn bool,
) [][]string {
	// Build rows array - we'll show all rows in range plus swap in indicators where needed
	rows := make([][]string, 0, visibleEnd-visibleStart)

	// Determine actual data indices to show
	// If we have indicators, we skip showing the first/last data rows since indicators replace them
	dataStartIdx := visibleStart
	dataEndIdx := visibleEnd

	switch {
	case hasMoreAbove && hasMoreBelow:
		// Both indicators: skip first and last data row
		dataStartIdx++
		dataEndIdx--
	case hasMoreAbove:
		// Top indicator only: skip first data row
		dataStartIdx++
	case hasMoreBelow:
		// Bottom indicator only: skip last data row
		dataEndIdx--
	}

	// Add top scroll indicator if needed (replaces first row)
	if hasMoreAbove {
		// Since indicator replaces a data row, we hide everything from 0 to dataStartIdx-1
		hiddenAbove := dataStartIdx
		indicator := fmt.Sprintf("↑ %d more above", hiddenAbove)

		// Style indicator without margin (SubtitleStyle has MarginBottom(1) which creates empty row)
		styledIndicator := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true).
			Render(indicator)

		if showBackupColumn {
			rows = append(rows, []string{
				styledIndicator,
				"", "", "", "",
			})
		} else {
			rows = append(rows, []string{
				styledIndicator,
				"", "", "",
			})
		}
	}

	// Add data rows
	for i := dataStartIdx; i < dataEndIdx; i++ {
		// Safety check
		if i < 0 || i >= len(m.tableRows) {
			continue
		}

		tr := m.tableRows[i]

		if showBackupColumn {
			rows = append(rows, []string{
				tr.Data[0],    // Name
				tr.Data[1],    // Status
				tr.Data[2],    // Info
				tr.BackupPath, // Backup
				tr.Data[3],    // Path
			})
		} else {
			rows = append(rows, []string{
				tr.Data[0], // Name
				tr.Data[1], // Status
				tr.Data[2], // Info
				tr.Data[3], // Path
			})
		}
	}

	// Add bottom scroll indicator if needed (replaces last row)
	if hasMoreBelow {
		// Since indicator replaces a data row, we hide everything from dataEndIdx onwards
		hiddenBelow := len(m.tableRows) - dataEndIdx
		indicator := fmt.Sprintf("↓ %d more below", hiddenBelow)

		// Style indicator without margin (SubtitleStyle has MarginBottom(1) which creates empty row)
		styledIndicator := lipgloss.NewStyle().
			Foreground(lipgloss.Color("240")).
			Italic(true).
			Render(indicator)

		if showBackupColumn {
			rows = append(rows, []string{
				styledIndicator,
				"", "", "", "",
			})
		} else {
			rows = append(rows, []string{
				styledIndicator,
				"", "", "",
			})
		}
	}

	return rows
}

// detectSubEntryState determines the state of a sub-entry item
func (m *Model) detectSubEntryState(item *SubEntryItem) PathState {
	targetPath := config.ExpandPath(item.Target, m.Platform.EnvVars)
	backupPath := m.resolvePath(item.SubEntry.Backup)

	st := detectConfigState(backupPath, targetPath, item.SubEntry.IsFolder(), item.SubEntry.Files)

	if st == StateLinked && item.SubEntry.IsConfig() && item.SubEntry.IsFolder() && m.Manager != nil {
		if m.Manager.HasOutdatedTemplates(backupPath) {
			return StateOutdated
		}
		if m.Manager.HasModifiedRenderedFiles(backupPath) {
			return StateModified
		}
	}

	return st
}

// getApplicationAtCursorFromTable returns the application and sub-entry indices from table cursor
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

	// Handle filter toggle confirmation
	if m.Operation == OpList && m.confirmingFilterToggle {
		switch msg.String() {
		case "y", "Y", KeyEnter:
			// Confirm - toggle filter and clear hidden selections
			m.confirmingFilterToggle = false
			m.filterToggleHiddenCount = 0
			m.filterEnabled = true
			m.clearHiddenSelections()
			m.rebuildTable()
			return m, nil
		case "n", "N", KeyEsc:
			// Cancel - keep filter off
			m.confirmingFilterToggle = false
			m.filterToggleHiddenCount = 0
			return m, nil
		}
		return m, nil
	}

	// Handle delete confirmation
	if m.Operation == OpList && (m.confirmingDeleteApp || m.confirmingDeleteSubEntry) {
		switch msg.String() {
		case "y", "Y", KeyEnter:
			// Confirm delete
			appIdx, subIdx := m.getApplicationAtCursorFromTable()
			if m.confirmingDeleteApp && appIdx >= 0 {
				m.confirmingDeleteApp = false
				if err := m.deleteApplication(appIdx); err != nil {
					m.err = err
				} else {
					// Rebuild table after deletion
					m.rebuildTable()
				}
			} else if m.confirmingDeleteSubEntry && appIdx >= 0 && subIdx >= 0 {
				m.confirmingDeleteSubEntry = false
				if err := m.deleteSubEntry(appIdx, subIdx); err != nil {
					m.err = err
				} else {
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

	// Handle diff picker separately
	if m.Operation == OpList && m.showingDiffPicker {
		return m.updateDiffPicker(msg)
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
	case "f":
		// Toggle filter
		if m.Operation == OpList && !m.searching && !m.confirmingDeleteApp && !m.confirmingDeleteSubEntry && !m.showingDetail {
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
				return m, m.checkFilteredStatesCmd()
			}

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
				m.updateScrollOffset()
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
				m.updateScrollOffset()
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
				m.summaryOperation = OpDelete
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
		// Install or Diff depending on context (only in List view)
		if m.Operation == OpList {
			// Check if multi-select mode is active
			if m.multiSelectActive {
				// Show summary screen for batch install
				m.summaryOperation = OpInstallPackages
				m.Screen = ScreenSummary
				return m, nil
			}

			appIdx, subIdx := m.getApplicationAtCursorFromTable()

			// On a modified sub-entry: launch diff viewer
			if appIdx >= 0 && subIdx >= 0 && m.Manager != nil {
				subItem := m.Applications[appIdx].SubItems[subIdx]
				if subItem.State == StateModified {
					backupPath := m.resolvePath(subItem.SubEntry.Backup)
					modifiedFiles, err := m.Manager.GetModifiedTemplateFiles(backupPath)
					if err != nil || len(modifiedFiles) == 0 {
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
			if appIdx >= 0 && subIdx < 0 {
				app := m.Applications[appIdx]
				if app.PkgInstalled != nil && !*app.PkgInstalled {
					m.Operation = OpInstallPackages
					m.currentPackageIndex = 0
					m.results = nil
					m.pendingPackages = []PackageItem{{
						Entry: config.Entry{
							Name:    app.Application.Name,
							Package: app.Application.Package,
						},
						Method:   app.PkgMethod,
						Selected: true,
					}}
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
				// Restore single sub-entry
				subItem := &m.Applications[appIdx].SubItems[subIdx]
				success, message := m.performRestoreSubEntry(subItem.SubEntry, subItem.Target)
				if success {
					m.Applications[appIdx].SubItems[subIdx].State = m.detectSubEntryState(subItem)
					m.rebuildTable()
				}
				m.results = []ResultItem{{
					Name:    subItem.SubEntry.Name,
					Success: success,
					Message: message,
				}}
			} else if appIdx >= 0 && subIdx < 0 {
				// Restore all sub-entries for this application
				m.results = nil
				for i := range m.Applications[appIdx].SubItems {
					subItem := &m.Applications[appIdx].SubItems[i]
					if !subItem.SubEntry.IsConfig() {
						continue
					}
					success, message := m.performRestoreSubEntry(subItem.SubEntry, subItem.Target)
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
	b.WriteString(RenderHelpWithWidth(m.width,
		"r", "new operation",
		"q/enter", "quit",
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
				RenderHelpWithWidth(m.width, "y/enter", "yes", "n/esc", "no")
		}
		return HelpStyle.Render("y/enter: yes | n/esc: no")

	case m.confirmingFilterToggle:
		// Filter toggle confirmation dialog
		itemText := "item(s)"
		if m.filterToggleHiddenCount == 1 {
			itemText = "item"
		}
		prompt := fmt.Sprintf("Enabling filter will hide %d selected %s. Continue? (y/n)",
			m.filterToggleHiddenCount, itemText)
		return WarningStyle.Render(prompt)

	case m.searching:
		return RenderHelpWithWidth(m.width,
			"enter", "confirm",
			"esc", "clear",
		)

	case m.showingDiffPicker:
		return RenderHelpWithWidth(m.width,
			"enter", "select",
			"esc", "cancel",
		)

	case m.showingDetail:
		return RenderHelpWithWidth(m.width,
			"h/←/esc", "close",
			"q", "quit",
		)

	default:
		// Build help text based on cursor position and multi-select mode
		var helpItems []string

		if m.multiSelectActive {
			// Multi-select mode help text
			helpItems = []string{
				"tab", "toggle",
				"esc", "clear",
				"r", "restore",
				"i", "install",
				"d", "delete",
				"q", "quit",
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

			// Show context-sensitive "i" help
			if subIdx < 0 {
				// App row: install
				helpItems = append(helpItems, "i", "install")
			} else if appIdx >= 0 && subIdx >= 0 && appIdx < len(m.Applications) &&
				subIdx < len(m.Applications[appIdx].SubItems) &&
				m.Applications[appIdx].SubItems[subIdx].State == StateModified {
				// Modified sub-entry: diff
				helpItems = append(helpItems, "i", "diff")
			}

			helpItems = append(helpItems, "q", "quit")
		}

		return RenderHelpWithWidth(m.width, helpItems...)
	}
}

//nolint:gocyclo // UI rendering with many states
func (m Model) viewListTable() string {
	var b strings.Builder
	linesUsed := 0

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
	linesUsed++

	// Table should already be initialized via Update()/initTableModel()
	// Do not call initTableModel() from View() — mutating state in View is a Bubble Tea anti-pattern

	// Count lines after table
	linesAfterTable := 1 // Blank line or multi-select banner after table

	// Add line for multi-select banner if active
	// (The banner replaces the blank line, so no additional line needed)

	// Detail panel
	appIdx, subIdx := m.getApplicationAtCursorFromTable()
	var detailContent string
	if m.showingDetail && appIdx >= 0 {
		if subIdx >= 0 {
			detailContent = m.renderSubEntryInlineDetail(&m.Applications[appIdx].SubItems[subIdx], m.width)
		} else {
			filtered := m.getSearchedApplications()
			if appIdx < len(filtered) {
				detailContent = m.renderApplicationInlineDetail(&filtered[appIdx], m.width)
			}
		}
		if detailContent != "" {
			linesAfterTable += strings.Count(detailContent, "\n") + 1
		}
	}

	// Diff picker panel
	var diffPickerContent string
	if m.showingDiffPicker {
		diffPickerContent = m.viewDiffPicker()
		linesAfterTable += strings.Count(diffPickerContent, "\n") + 1
	}

	// Result message
	if len(m.results) > 0 {
		linesAfterTable += 2 // Blank line + result
	}

	// Help footer - measure it
	helpText := m.renderHelpForCurrentState()
	helpLines := strings.Count(helpText, "\n") + 1
	linesAfterTable += 1 + helpLines // Blank line + help

	// Calculate available height for table
	// Subtract 2 for BaseStyle vertical padding (1 top + 1 bottom)
	availableForTable := m.height - linesUsed - linesAfterTable - 2
	if availableForTable < 10 {
		availableForTable = 10 // Minimum table height
	}

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

	// Result message
	if len(m.results) > 0 {
		b.WriteString("\n")
		result := m.results[len(m.results)-1]
		var resultText string
		if result.Success {
			resultText = SuccessStyle.Render(fmt.Sprintf("✓ %s: %s", result.Name, result.Message))
		} else {
			resultText = ErrorStyle.Render(fmt.Sprintf("✗ %s: %s", result.Name, result.Message))
		}
		b.WriteString(resultText)
	}

	// Help footer
	b.WriteString("\n")
	b.WriteString(helpText)

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
	converted := packages.FromEntry(pkg.Entry)
	if converted == nil {
		return nil
	}

	return packages.BuildCommand(*converted, pkg.Method, m.Platform.OS)
}

// checkPackageStatesCmd returns a tea.Cmd that detects package methods and checks install statuses.
// Each package gets its own concurrent command so results stream in individually.
func (m Model) checkPackageStatesCmd() tea.Cmd {
	var cmds []tea.Cmd
	osType := m.Platform.OS

	for i, app := range m.Applications {
		if app.IsFiltered || !app.Application.HasPackage() {
			continue
		}
		appIndex := i
		pkg := app.Application.Package
		name := app.Application.Name
		cmds = append(cmds, func() tea.Msg {
			method := getPackageInstallMethodFromPackage(pkg, osType)
			installed := false
			if method != TypeNone {
				installed = isPackageInstalledFromPackage(pkg, method, name)
			}
			return pkgCheckResultMsg{appIndex: appIndex, method: method, installed: installed}
		})
	}

	return tea.Batch(cmds...)
}

// checkSubEntryStatesCmd returns a tea.Cmd that checks all sub-entry states.
// Each sub-entry gets its own concurrent command so results stream in individually.
// Filtered apps are skipped; use checkFilteredStatesCmd when the filter is toggled off.
func (m Model) checkSubEntryStatesCmd() tea.Cmd {
	var cmds []tea.Cmd
	plat := m.Platform
	cfg := m.Config
	mgr := m.Manager

	for i, app := range m.Applications {
		if app.IsFiltered {
			continue
		}
		for j, sub := range app.SubItems {
			appIndex := i
			subIndex := j
			subItem := sub
			cmds = append(cmds, func() tea.Msg {
				state := detectSubEntryStateStatic(subItem, plat, cfg, mgr)
				return stateCheckResultMsg{appIndex: appIndex, subIndex: subIndex, state: state}
			})
		}
	}

	return tea.Batch(cmds...)
}

// checkFilteredStatesCmd triggers async checks for filtered apps that haven't been scanned yet.
// Called when the user toggles the filter off, revealing previously-hidden apps.
func (m Model) checkFilteredStatesCmd() tea.Cmd {
	var cmds []tea.Cmd
	osType := m.Platform.OS
	plat := m.Platform
	cfg := m.Config
	mgr := m.Manager

	for i, app := range m.Applications {
		if !app.IsFiltered {
			continue
		}

		// Package check (only if not already resolved)
		if app.Application.HasPackage() && app.PkgInstalled == nil {
			appIndex := i
			pkg := app.Application.Package
			name := app.Application.Name
			cmds = append(cmds, func() tea.Msg {
				method := getPackageInstallMethodFromPackage(pkg, osType)
				installed := false
				if method != TypeNone {
					installed = isPackageInstalledFromPackage(pkg, method, name)
				}
				return pkgCheckResultMsg{appIndex: appIndex, method: method, installed: installed}
			})
		}

		// Sub-entry state checks (only if still at StateLoading)
		for j, sub := range app.SubItems {
			if sub.State != StateLoading {
				continue
			}
			appIndex := i
			subIndex := j
			subItem := sub
			cmds = append(cmds, func() tea.Msg {
				state := detectSubEntryStateStatic(subItem, plat, cfg, mgr)
				return stateCheckResultMsg{appIndex: appIndex, subIndex: subIndex, state: state}
			})
		}
	}

	return tea.Batch(cmds...)
}

// detectSubEntryStateStatic determines the state of a sub-entry item without using Model receiver.
// This is safe to call from goroutines since it takes explicit dependencies.
func detectSubEntryStateStatic(item SubEntryItem, plat *platform.Platform, cfg *config.Config, mgr *manager.Manager) PathState {
	targetPath := config.ExpandPath(item.Target, plat.EnvVars)
	backupPath := resolvePathStatic(item.SubEntry.Backup, cfg, plat.EnvVars)

	st := detectConfigState(backupPath, targetPath, item.SubEntry.IsFolder(), item.SubEntry.Files)

	if st == StateLinked && item.SubEntry.IsConfig() && item.SubEntry.IsFolder() && mgr != nil {
		if mgr.HasOutdatedTemplates(backupPath) {
			return StateOutdated
		}
		if mgr.HasModifiedRenderedFiles(backupPath) {
			return StateModified
		}
	}

	return st
}

// resolvePathStatic resolves relative paths and expands ~ without using Model receiver.
func resolvePathStatic(path string, cfg *config.Config, envVars map[string]string) string {
	resolvedPath := path
	if len(path) > 0 && path[0] == '.' {
		resolvedPath = cfg.BackupRoot + path[1:]
	}

	return config.ExpandPath(resolvedPath, envVars)
}
