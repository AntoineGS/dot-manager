package tui

import (
	"fmt"
	"sort"
	"strings"

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

// fixTreeCharacters recalculates tree characters (|- vs |_ ) after sorting
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
			m.formatHeaderWithShortcut("status", 't', SortColumnStatus),
			"info",
			"backup",
			m.formatHeaderWithShortcut("path", 'p', SortColumnPath),
		}
	} else {
		headers = []string{
			m.formatHeaderWithShortcut("name", 'n', SortColumnName),
			m.formatHeaderWithShortcut("status", 't', SortColumnStatus),
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
	spinnerFrame := m.spinner.View()
	for i := dataStartIdx; i < dataEndIdx; i++ {
		// Safety check
		if i < 0 || i >= len(m.tableRows) {
			continue
		}

		tr := m.tableRows[i]

		// Replace "Loading..." text with spinner frame
		status := tr.Data[1]
		if status == StatusLoading {
			status = spinnerFrame
		}

		if showBackupColumn {
			rows = append(rows, []string{
				tr.Data[0],    // Name
				status,        // Status
				tr.Data[2],    // Info
				tr.BackupPath, // Backup
				tr.Data[3],    // Path
			})
		} else {
			rows = append(rows, []string{
				tr.Data[0], // Name
				status,     // Status
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
