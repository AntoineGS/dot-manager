package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/AntoineGS/dot-manager/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

// Layout constants for list table view
const (
	// listTableOverhead is the number of lines used by title, header, separator, and footer
	// Title block (3) + header+separator (2) + footer (4) = 9
	listTableOverhead = 9
	// minVisibleRows is the minimum number of table rows to show
	minVisibleRows = 3
	// minVisibleWithDetail is the minimum rows when detail panel is showing
	minVisibleWithDetail = 1
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

	// Sort applications alphabetically by name
	sort.Slice(m.Applications, func(i, j int) bool {
		return m.Applications[i].Application.Name < m.Applications[j].Application.Name
	})

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
	// Expand ~ in target path for file operations
	targetPath := config.ExpandPath(item.Target, m.Platform.EnvVars)

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

// getApplicationAtCursor returns the application and sub-entry indices for the current cursor position
func (m *Model) getApplicationAtCursor() (int, int) {
	visualRow := 0
	filtered := m.getFilteredApplications()

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

// getVisibleRowCount returns total number of visible rows in the 2-level table (filtered)
func (m *Model) getVisibleRowCount() int {
	count := 0
	filtered := m.getFilteredApplications()

	for _, app := range filtered {
		count++ // Application row
		if app.Expanded {
			count += len(app.SubItems) // Sub-entry rows (no separate separator)
		}
	}

	return count
}

// padRight pads a string with spaces to the right to reach the specified width
func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}

	return s + strings.Repeat(" ", width-len(s))
}

// getAggregateState returns the worst state among all sub-entries in an application
func (m Model) getAggregateState(app ApplicationItem) PathState {
	if len(app.SubItems) == 0 {
		return StateMissing
	}

	hasLinked := false
	hasReady := false
	hasAdopt := false
	hasMissing := false

	for _, sub := range app.SubItems {
		switch sub.State {
		case StateLinked:
			hasLinked = true
		case StateReady:
			hasReady = true
		case StateAdopt:
			hasAdopt = true
		case StateMissing:
			hasMissing = true
		}
	}

	// Return worst state
	if hasMissing {
		return StateMissing
	}

	if hasAdopt {
		return StateAdopt
	}

	if hasReady {
		return StateReady
	}

	if hasLinked {
		return StateLinked
	}

	return StateMissing
}

func (m Model) viewProgress() string {
	var b strings.Builder

	// Title
	title := fmt.Sprintf("⏳  %s in progress...", m.Operation.String())
	b.WriteString(TitleStyle.Render(title))
	b.WriteString("\n\n")

	// Spinner animation would go here
	b.WriteString(SpinnerStyle.Render("Processing..."))
	b.WriteString("\n")

	return BaseStyle.Render(b.String())
}

func (m Model) updateResults(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle filter mode input
	if m.Operation == OpList && m.filtering {
		switch msg.String() {
		case KeyEsc:
			// Clear filter and exit filter mode
			m.filtering = false
			m.filterText = ""
			m.filterInput.SetValue("")
			m.filterInput.Blur()
			// Reset cursor and scroll to beginning
			m.appCursor = 0
			m.scrollOffset = 0

			return m, nil
		case KeyEnter:
			// Confirm filter and return to navigation mode
			m.filtering = false
			m.filterInput.Blur()

			return m, nil
		default:
			// Pass key to text input
			var cmd tea.Cmd
			m.filterInput, cmd = m.filterInput.Update(msg)
			m.filterText = m.filterInput.Value()
			// Reset cursor when filter changes
			m.appCursor = 0
			m.scrollOffset = 0

			return m, cmd
		}
	}

	// Handle delete confirmation
	if m.Operation == OpList && (m.confirmingDeleteApp || m.confirmingDeleteSubEntry) {
		switch msg.String() {
		case "y", "Y", KeyEnter:
			// Confirm delete
			appIdx, subIdx := m.getApplicationAtCursor()
			if m.confirmingDeleteApp && appIdx >= 0 {
				m.confirmingDeleteApp = false
				if err := m.deleteApplication(appIdx); err == nil {
					// Adjust cursor if needed
					visibleCount := m.getVisibleRowCount()
					if m.appCursor >= visibleCount && m.appCursor > 0 {
						m.appCursor--
					}
				}
			} else if m.confirmingDeleteSubEntry && appIdx >= 0 && subIdx >= 0 {
				m.confirmingDeleteSubEntry = false
				if err := m.deleteSubEntry(appIdx, subIdx); err == nil {
					// Adjust cursor if needed
					visibleCount := m.getVisibleRowCount()
					if m.appCursor >= visibleCount && m.appCursor > 0 {
						m.appCursor--
					}
				}
			}

			return m, nil
		case "n", "N", "esc":
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
		case KeyEsc, KeyEnter, "h", KeyLeft:
			// Close detail popup (ESC cancels/closes the popup)
			m.showingDetail = false
			return m, nil
		case "q":
			// q closes popup and goes back to menu
			m.showingDetail = false
			m.Screen = ScreenMenu

			return m, nil
		}

		return m, nil
	}

	switch msg.String() {
	case "/":
		// Enter filter mode
		if m.Operation == OpList && !m.confirmingDeleteApp && !m.confirmingDeleteSubEntry && !m.showingDetail {
			m.filtering = true
			m.filterInput.Focus()

			return m, nil
		}
	case "q":
		if m.Operation == OpList {
			m.Screen = ScreenMenu
			return m, nil
		}

		return m, tea.Quit
	case "up", "k":
		if m.Operation == OpList {
			if m.appCursor > 0 {
				m.appCursor--

				if m.appCursor < m.scrollOffset {
					m.scrollOffset = m.appCursor
				}
			}
		}

		return m, nil
	case "down", "j":
		if m.Operation == OpList {
			visibleCount := m.getVisibleRowCount()
			if m.appCursor < visibleCount-1 {
				m.appCursor++
				// Calculate actual visible rows (accounting for overhead)
				maxTableRows := m.viewHeight - listTableOverhead
				if m.filtering || m.filterText != "" {
					maxTableRows--
				}

				if maxTableRows < minVisibleRows {
					maxTableRows = minVisibleRows
				}

				if m.appCursor >= m.scrollOffset+maxTableRows {
					m.scrollOffset = m.appCursor - maxTableRows + 1
				}
			}
		}

		return m, nil
	case "h", "left":
		if m.Operation == OpList {
			// Collapse node if expanded
			appIdx, subIdx := m.getApplicationAtCursor()
			if appIdx >= 0 && m.Applications[appIdx].Expanded {
				m.Applications[appIdx].Expanded = false
				// If we were on a sub-entry, move cursor to parent app
				if subIdx >= 0 {
					// Calculate the app row position
					visualRow := 0
					for i := 0; i < appIdx; i++ {
						visualRow++
						if m.Applications[i].Expanded {
							visualRow += len(m.Applications[i].SubItems)
						}
					}
					m.appCursor = visualRow
				}
			} else {
				// Not on expanded app, navigate back to menu
				m.Screen = ScreenMenu
			}

			return m, nil
		}

		return m, tea.Quit
	case KeyEnter, "l", "right":
		if m.Operation == OpList {
			// If showing detail, close it; otherwise toggle expand or show detail
			if m.showingDetail {
				m.showingDetail = false
			} else {
				appIdx, _ := m.getApplicationAtCursor()
				if appIdx >= 0 {
					// Toggle expansion
					m.Applications[appIdx].Expanded = !m.Applications[appIdx].Expanded
				}
			}

			return m, nil
		}

		return m, tea.Quit
	case "e":
		// Edit selected Application or SubEntry (only in List view)
		if m.Operation == OpList {
			appIdx, subIdx := m.getApplicationAtCursor()
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
			appIdx, _ := m.getApplicationAtCursor()
			if appIdx >= 0 {
				m.initSubEntryFormNew(appIdx)
				return m, nil
			}
		}
	case "d", "delete", "backspace":
		// Ask for delete confirmation (only in List view)
		if m.Operation == OpList {
			appIdx, subIdx := m.getApplicationAtCursor()
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
			appIdx, _ := m.getApplicationAtCursor()
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
			appIdx, subIdx := m.getApplicationAtCursor()
			if appIdx >= 0 && subIdx >= 0 {
				subItem := &m.Applications[appIdx].SubItems[subIdx]
				// Perform restore using SubEntry data
				success, message := m.performRestoreSubEntry(subItem.SubEntry, subItem.Target)
				// Update the state after restore
				if success {
					m.Applications[appIdx].SubItems[subIdx].State = m.detectSubEntryState(subItem)
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

func (m Model) viewListTable() string {
	var b strings.Builder

	// Title
	b.WriteString(TitleStyle.Render("󰋗  Manage"))
	b.WriteString("\n")

	// Filter input (show when filtering or when filter is active)
	if m.filtering || m.filterText != "" {
		b.WriteString("  / ")

		if m.filtering {
			b.WriteString(m.filterInput.View())
		} else {
			b.WriteString(FilterInputStyle.Render(m.filterText))
		}

		b.WriteString("\n")
	}

	b.WriteString("\n")

	// Calculate visible rows and detail height
	totalVisibleRows := m.getVisibleRowCount()

	// Calculate detail height if showing
	detailHeight := 0

	appIdx, subIdx := m.getApplicationAtCursor()
	if m.showingDetail && appIdx >= 0 {
		if subIdx >= 0 {
			// Detail for SubEntry
			detailHeight = m.calcSubEntryDetailHeight(&m.Applications[appIdx].SubItems[subIdx])
		} else {
			// Detail for Application
			detailHeight = m.calcApplicationDetailHeight(&m.Applications[appIdx])
		}
	}

	// Calculate how many table rows can fit
	maxTableRows := m.viewHeight - listTableOverhead
	// Account for filter bar
	if m.filtering || m.filterText != "" {
		maxTableRows--
	}

	if maxTableRows < minVisibleRows {
		maxTableRows = minVisibleRows
	}

	// Calculate how many rows we can show
	maxVisible := maxTableRows
	if m.showingDetail {
		maxVisible = maxTableRows - detailHeight
		if maxVisible < minVisibleWithDetail {
			maxVisible = minVisibleWithDetail
		}
	}

	if maxVisible > totalVisibleRows {
		maxVisible = totalVisibleRows
	}

	// Keep the same scroll offset - don't change start when toggling detail
	start := m.scrollOffset
	if start >= totalVisibleRows {
		start = 0
	}

	// Ensure cursor is visible within the reduced window when detail is showing
	if m.showingDetail {
		cursorPosInWindow := m.appCursor - start
		if cursorPosInWindow >= maxVisible {
			// Cursor would be hidden, adjust start to show cursor at bottom of reduced window
			start = m.appCursor - maxVisible + 1
		}

		if cursorPosInWindow < 0 {
			start = m.appCursor
		}
	}

	end := start + maxVisible
	if end > totalVisibleRows {
		end = totalVisibleRows
	}

	// Calculate column widths for alignment
	filtered := m.getFilteredApplications()
	maxAppNameWidth := 0
	maxSubNameWidth := 0
	maxTypeWidth := 0
	maxSourceWidth := 0
	maxStatusWidth := 0

	for _, app := range filtered {
		if len(app.Application.Name) > maxAppNameWidth {
			maxAppNameWidth = len(app.Application.Name)
		}

		if app.Expanded {
			for _, subItem := range app.SubItems {
				if len(subItem.SubEntry.Name) > maxSubNameWidth {
					maxSubNameWidth = len(subItem.SubEntry.Name)
				}

				// Type info width
				typeInfo := ""
				if subItem.SubEntry.IsGit() {
					typeInfo = TypeGit
				} else if subItem.SubEntry.IsFolder() {
					typeInfo = TypeFolder
				} else {
					fileCount := len(subItem.SubEntry.Files)
					if fileCount == 1 {
						typeInfo = "1 file"
					} else {
						typeInfo = fmt.Sprintf("%d files", fileCount)
					}
				}

				if len(typeInfo) > maxTypeWidth {
					maxTypeWidth = len(typeInfo)
				}

				// Source path width
				sourcePath := ""
				if subItem.SubEntry.IsGit() {
					sourcePath = truncateStr(subItem.SubEntry.Repo)
				} else {
					// Show backup path as-is (e.g., "./nvim") without resolving
					sourcePath = truncateStr(subItem.SubEntry.Backup)
				}

				if len(sourcePath) > maxSourceWidth {
					maxSourceWidth = len(sourcePath)
				}

				// Status badge width
				statusText := subItem.State.String()
				if len(statusText) > maxStatusWidth {
					maxStatusWidth = len(statusText)
				}
			}
		}
	}

	// Render hierarchical tree structure
	visualRow := 0
	for _, app := range filtered {
		// Render Application row
		if visualRow >= start && visualRow < end {
			isSelected := visualRow == m.appCursor
			cursor := RenderCursor(isSelected)

			// Aggregate state for app (show worst state among sub-entries)
			aggregateState := m.getAggregateState(app)

			// Entry count
			entryCount := fmt.Sprintf("%d entries", len(app.SubItems))

			// Package indicator
			var pkgIndicator string

			if app.PkgInstalled != nil {
				if *app.PkgInstalled {
					pkgIndicator = "✓"
				} else {
					pkgIndicator = "✗"
				}
			} else {
				pkgIndicator = " "
			}

			// Pad to column widths
			paddedName := padRight(app.Application.Name, maxAppNameWidth)
			paddedCount := padRight(entryCount, 12) // Fixed width for entry count

			// Build the complete line with or without selection styling
			var line string

			if isSelected {
				// Apply selection style to entire row (use plain text for state, no badge styling)
				// Match badge visual width: 1 (margin) + 1 (padding) + text + 1 (padding) = 10 total
				statePlainText := " " + padRight(" "+aggregateState.String()+" ", 9)
				line = fmt.Sprintf("%s%s  %s  %s  %s ",
					cursor,
					paddedName,
					statePlainText,
					paddedCount,
					pkgIndicator)
				line = SelectedListItemStyle.Render(line)
			} else {
				// Apply individual column styles (use styled badge)
				stateBadge := renderStateBadge(aggregateState)
				line = fmt.Sprintf("%s%s  %s  %s  %s",
					cursor,
					paddedName,
					stateBadge,
					MutedTextStyle.Render(paddedCount),
					pkgIndicator)
			}

			b.WriteString(line)
			b.WriteString("\n")

			// Show inline detail panel below selected application row
			if isSelected && m.showingDetail && subIdx < 0 {
				b.WriteString(m.renderApplicationInlineDetail(&app, m.width))
			}
		}

		visualRow++

		// Render sub-entry rows if expanded
		if app.Expanded {
			for subItemIdx, subItem := range app.SubItems {
				if visualRow >= start && visualRow < end {
					isSelected := visualRow == m.appCursor

					// Tree connector: ├─ for non-last items, └─ for last item
					treePrefix := "├─"
					if subItemIdx == len(app.SubItems)-1 {
						treePrefix = "└─"
					}

					// Calculate plain status text with padding (match app-level format at line 866)
					statusPlainText := " " + padRight(" "+subItem.State.String()+" ", 9)

					// Cursor or spacing
					cursor := RenderCursor(isSelected)

					// Type info
					typeInfo := ""
					if subItem.SubEntry.IsGit() {
						typeInfo = TypeGit
					} else if subItem.SubEntry.IsFolder() {
						typeInfo = TypeFolder
					} else {
						fileCount := len(subItem.SubEntry.Files)
						if fileCount == 1 {
							typeInfo = "1 file"
						} else {
							typeInfo = fmt.Sprintf("%d files", fileCount)
						}
					}

					// Source path
					sourcePath := ""
					if subItem.SubEntry.IsGit() {
						sourcePath = truncateStr(subItem.SubEntry.Repo)
					} else {
						// Show backup path as-is (e.g., "./nvim") without resolving
						sourcePath = truncateStr(subItem.SubEntry.Backup)
					}

					// Target path
					targetPath := truncateStr(subItem.Target)

					// Pad to column widths
					paddedName := padRight(subItem.SubEntry.Name, maxSubNameWidth)
					paddedType := padRight(typeInfo, maxTypeWidth)
					paddedSource := padRight(sourcePath, maxSourceWidth)

					// Build the complete line with or without selection styling
					var line string
					if isSelected {
						// Apply selection style to entire row
						line = fmt.Sprintf("%s %s %s  %s  %s  %s  %s ",
							cursor,
							treePrefix,
							paddedName,
							statusPlainText,
							paddedType,
							paddedSource,
							targetPath)
						line = SelectedListItemStyle.Render(line)
					} else {
						// Status badge using existing renderStateBadge function
						statusBadge := renderStateBadge(subItem.State)

						// Apply individual column styles for visual distinction
						line = fmt.Sprintf("%s %s %s  %s  %s  %s  %s",
							cursor,
							treePrefix,
							paddedName,
							statusBadge,
							MutedTextStyle.Render(paddedType),
							PathBackupStyle.Render(paddedSource),
							PathTargetStyle.Render(targetPath))
					}

					b.WriteString(line)
					b.WriteString("\n")

					// Show inline detail panel below selected sub-entry row
					if isSelected && m.showingDetail && subIdx >= 0 {
						b.WriteString(m.renderSubEntryInlineDetail(&subItem, m.width))
					}
				}

				visualRow++
			}
		}
	}

	// Scroll indicators (always show line, even if empty, for consistent height)
	scrollInfo := ""
	if start > 0 || end < totalVisibleRows {
		scrollInfo = fmt.Sprintf("Showing %d-%d of %d", start+1, end, totalVisibleRows)
		if start > 0 {
			scrollInfo = "↑ " + scrollInfo
		}

		if end < totalVisibleRows {
			scrollInfo = scrollInfo + " ↓"
		}
	}

	b.WriteString(SubtitleStyle.Render(scrollInfo))
	b.WriteString("\n")

	// Help or confirmation prompt
	b.WriteString("\n")

	if m.confirmingDeleteApp || m.confirmingDeleteSubEntry {
		// Show delete confirmation prompt
		var name string
		if m.confirmingDeleteApp && appIdx >= 0 {
			name = m.Applications[appIdx].Application.Name
		} else if m.confirmingDeleteSubEntry && appIdx >= 0 && subIdx >= 0 {
			name = m.Applications[appIdx].SubItems[subIdx].SubEntry.Name
		}

		if name != "" {
			b.WriteString(WarningStyle.Render(fmt.Sprintf("Delete '%s'? ", name)))
			b.WriteString(RenderHelpWithWidth(m.width, "y/enter", "yes", "n/esc", "no"))
		}
	} else if m.filtering {
		b.WriteString(RenderHelpWithWidth(m.width,
			"enter", "confirm",
			"esc", "clear",
		))
	} else if m.showingDetail {
		b.WriteString(RenderHelpWithWidth(m.width,
			"h/←/esc", "close",
			"q", "menu",
		))
	} else {
		b.WriteString(RenderHelpWithWidth(m.width,
			"/", "filter",
			"l/→", "details",
			"A", "add app",
			"a", "add entry",
			"e", "edit",
			"d", "delete",
			"r", "restore",
			"i", "install",
			"q", "menu",
		))
	}

	return BaseStyle.Render(b.String())
}

// getFilteredApplications returns filtered applications for hierarchical view
func (m Model) getFilteredApplications() []ApplicationItem {
	if m.filterText == "" {
		return m.Applications
	}

	filterLower := strings.ToLower(m.filterText)
	var filtered []ApplicationItem

	for _, app := range m.Applications {
		appMatches := strings.Contains(strings.ToLower(app.Application.Name), filterLower) ||
			strings.Contains(strings.ToLower(app.Application.Description), filterLower)

		// Filter SubItems
		var matchingSubItems []SubEntryItem

		for _, sub := range app.SubItems {
			subMatches := strings.Contains(strings.ToLower(sub.SubEntry.Name), filterLower) ||
				strings.Contains(strings.ToLower(sub.Target), filterLower)

			// Check source field
			if sub.SubEntry.IsGit() {
				subMatches = subMatches || strings.Contains(strings.ToLower(sub.SubEntry.Repo), filterLower)
			} else {
				subMatches = subMatches || strings.Contains(strings.ToLower(sub.SubEntry.Backup), filterLower)
			}

			if appMatches || subMatches {
				matchingSubItems = append(matchingSubItems, sub)
			}
		}

		if len(matchingSubItems) > 0 {
			appCopy := app
			appCopy.SubItems = matchingSubItems
			filtered = append(filtered, appCopy)
		}
	}

	return filtered
}

func truncateStr(s string) string {
	const maxLen = 30
	if len(s) <= maxLen {
		return s
	}

	if maxLen <= 3 {
		return s[:maxLen]
	}

	return s[:maxLen-3] + "..."
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
