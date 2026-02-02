package tui

import (
	"fmt"
	"os"
	"strings"

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
	// Handle delete confirmation
	if m.Operation == OpList && m.confirmingDelete {
		switch msg.String() {
		case "y", "Y", "enter":
			// Confirm delete
			m.confirmingDelete = false
			if err := m.deleteEntry(m.listCursor); err == nil {
				// Adjust cursor if needed
				if m.listCursor >= len(m.Paths) && m.listCursor > listStartIndex {
					m.listCursor--
				}
				// Adjust scroll offset if needed
				if m.scrollOffset > listStartIndex && m.scrollOffset >= len(m.Paths) {
					m.scrollOffset = len(m.Paths) - 1
					if m.scrollOffset < listStartIndex {
						m.scrollOffset = listStartIndex
					}
				}
			}
			return m, nil
		case "n", "N", "esc":
			// Cancel delete
			m.confirmingDelete = false
			return m, nil
		}
		return m, nil
	}

	// Handle detail popup separately
	if m.Operation == OpList && m.showingDetail {
		switch msg.String() {
		case "esc", "enter", "h", "left":
			m.showingDetail = false
			return m, nil
		case "q":
			m.showingDetail = false
			m.Screen = ScreenMenu
			return m, nil
		}
		return m, nil
	}

	switch msg.String() {
	case "q":
		if m.Operation == OpList {
			m.Screen = ScreenMenu
			return m, nil
		}
		return m, tea.Quit
	case "esc", "h", "left":
		if m.Operation == OpList {
			m.Screen = ScreenMenu
			return m, nil
		}
		return m, tea.Quit
	case "enter", "l", "right":
		if m.Operation == OpList {
			// Open detail popup for selected item
			if len(m.Paths) > 0 {
				m.showingDetail = true
			}
			return m, nil
		}
		return m, tea.Quit
	case "e":
		// Edit selected path (only in List view)
		if m.Operation == OpList && len(m.Paths) > 0 {
			m.initAddFormWithIndex(m.listCursor)
			m.Screen = ScreenAddForm
			return m, nil
		}
	case "a":
		// Add new path (only in List view)
		if m.Operation == OpList {
			m.initAddForm()
			m.Screen = ScreenAddForm
			return m, nil
		}
	case "d", "delete", "backspace":
		// Ask for delete confirmation (only in List view)
		if m.Operation == OpList && len(m.Paths) > listStartIndex {
			m.confirmingDelete = true
			return m, nil
		}
	case "i":
		// Install package for selected entry (only in List view)
		if m.Operation == OpList && len(m.Paths) > 0 {
			item := m.Paths[m.listCursor]
			if item.PkgInstalled != nil && !*item.PkgInstalled {
				// Setup for package installation
				m.Operation = OpInstallPackages
				m.pendingPackages = []PackageItem{{
					Entry:    item.Entry,
					Method:   item.PkgMethod,
					Selected: true,
				}}
				m.currentPackageIndex = 0
				m.results = nil
				m.Screen = ScreenProgress
				return m, m.installNextPackage()
			}
		}
		return m, nil
	case "m":
		// Install all missing packages (only in List view)
		if m.Operation == OpList {
			var missing []PackageItem
			for _, item := range m.Paths {
				if item.PkgInstalled != nil && !*item.PkgInstalled {
					missing = append(missing, PackageItem{
						Entry:    item.Entry,
						Method:   item.PkgMethod,
						Selected: true,
					})
				}
			}
			if len(missing) > 0 {
				m.Operation = OpInstallPackages
				m.pendingPackages = missing
				m.currentPackageIndex = 0
				m.results = nil
				m.Screen = ScreenProgress
				return m, m.installNextPackage()
			}
		}
		return m, nil
	case "r":
		// Restore selected entry (only in List view for config/git entries)
		if m.Operation == OpList && len(m.Paths) > 0 {
			item := m.Paths[m.listCursor]
			// Only restore config or git entries (not package-only)
			if item.EntryType != EntryTypePackage {
				success, message := m.performRestore(item)
				// Update the state after restore
				if success {
					m.Paths[m.listCursor].State = m.detectPathState(&m.Paths[m.listCursor])
				}
				// Show result briefly in results
				m.results = []ResultItem{{
					Name:    item.Entry.Name,
					Success: success,
					Message: message,
				}}
			}
		}
		return m, nil
	case "up", "k":
		if m.Operation == OpList {
			if m.listCursor > 0 {
				m.listCursor--
				// Scroll up if cursor goes above visible area
				if m.listCursor < m.scrollOffset {
					m.scrollOffset = m.listCursor
				}
			}
		}
		return m, nil
	case "down", "j":
		if m.Operation == OpList {
			if m.listCursor < len(m.Paths)-1 {
				m.listCursor++
				// Scroll down if cursor goes below visible area
				// Use same calculation as viewListTable for visible rows
				visibleRows := m.viewHeight - listTableOverhead
				if visibleRows < minVisibleRows {
					visibleRows = minVisibleRows
				}
				if m.listCursor >= m.scrollOffset+visibleRows {
					m.scrollOffset = m.listCursor - visibleRows + 1
				}
			}
		}
		return m, nil
	}
	return m, nil
}

func (m Model) viewResults() string {
	// Use table view for List operation
	if m.Operation == OpList {
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
				b.WriteString("    " + SubtitleStyle.Render(line))
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
	b.WriteString("\n\n")

	// Calculate column widths based on terminal width
	// Reserve space for: padding (4) + cursor (2) + separators (10) + minimum content
	availWidth := m.width - 14
	if availWidth < 60 {
		availWidth = 60
	}

	// Column widths: Name (20%), Type (8), Pkg (1), Source (35%), Target (35%)
	nameWidth := availWidth * 20 / 100
	if nameWidth < 12 {
		nameWidth = 12
	}
	typeWidth := 8
	pkgWidth := 1 // Single character: ✓, ✗, or space
	pathWidth := (availWidth - nameWidth - typeWidth - pkgWidth) / 2

	// Total table width: cursor(2) + name + sep(2) + type + sep(2) + pkg + sep(2) + source + sep(2) + target
	tableWidth := 2 + nameWidth + 2 + typeWidth + 2 + pkgWidth + 2 + pathWidth + 2 + pathWidth

	// Table header (with space for cursor)
	headerStyle := PathNameStyle.Bold(true)
	header := fmt.Sprintf("  %-*s  %-*s  %s  %-*s  %-*s",
		nameWidth, "Name",
		typeWidth, "Type",
		"P", // Single char header for Package status
		pathWidth, "Source",
		pathWidth, "Target")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	// Header separator
	separator := "  " + strings.Repeat("─", nameWidth) + "──" +
		strings.Repeat("─", typeWidth) + "──" +
		strings.Repeat("─", pkgWidth) + "──" +
		strings.Repeat("─", pathWidth) + "──" +
		strings.Repeat("─", pathWidth)
	b.WriteString(MutedTextStyle.Render(separator))
	b.WriteString("\n")

	// Calculate detail height if showing
	detailHeight := 0
	if m.showingDetail && m.listCursor < len(m.Paths) {
		detailHeight = m.calcDetailHeight(m.Paths[m.listCursor])
	}

	// Calculate how many table rows can fit
	maxTableRows := m.viewHeight - listTableOverhead
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
	if maxVisible > len(m.Paths) {
		maxVisible = len(m.Paths)
	}

	// Keep the same scroll offset - don't change start when toggling detail
	start := m.scrollOffset

	// Ensure cursor is visible within the reduced window when detail is showing
	if m.showingDetail {
		cursorPosInWindow := m.listCursor - start
		if cursorPosInWindow >= maxVisible {
			// Cursor would be hidden, adjust start to show cursor at bottom of reduced window
			start = m.listCursor - maxVisible + 1
		}
		if cursorPosInWindow < 0 {
			start = m.listCursor
		}
	}

	end := start + maxVisible
	if end > len(m.Paths) {
		end = len(m.Paths)
	}

	for i := start; i < end; i++ {
		item := m.Paths[i]
		isSelected := i == m.listCursor
		cursor := RenderCursor(isSelected)

		// Determine type based on entry type
		var typeStr string
		var sourceStr string
		switch item.EntryType {
		case EntryTypeGit:
			typeStr = "git"
			sourceStr = truncateStr(item.Entry.Repo, pathWidth)
		case EntryTypePackage:
			typeStr = "package"
			sourceStr = truncateStr(item.PkgMethod, pathWidth)
		default: // EntryTypeConfig
			if item.Entry.IsFolder() {
				typeStr = "folder"
			} else {
				typeStr = fmt.Sprintf("%d files", len(item.Entry.Files))
			}
			sourceStr = truncateStr(item.Entry.Backup, pathWidth)
		}

		// Determine installed status indicator
		var pkgIndicator string
		if item.PkgInstalled != nil {
			if *item.PkgInstalled {
				pkgIndicator = "✓"
			} else {
				pkgIndicator = "✗"
			}
		} else {
			pkgIndicator = " "
		}

		// Truncate paths if needed (show config-style values with ~)
		name := item.Entry.Name
		// Add root indicator (use text instead of emoji to preserve ANSI styling)
		// Show [R] for root entries or packages that require sudo
		needsSudo := item.Entry.Root || requiresSudo(item.PkgMethod)
		if needsSudo {
			name = "[R] " + name
		}
		name = truncateStr(name, nameWidth)
		target := truncateStr(unexpandHome(item.Entry.Targets[m.Platform.OS]), pathWidth)

		// Build row with fixed-width columns
		row := fmt.Sprintf("%-*s  %-*s  ",
			nameWidth, name,
			typeWidth, typeStr)

		// Add package indicator (no styling here - will be styled with whole row)
		row += pkgIndicator
		row += "  " // separator after pkg column

		row += fmt.Sprintf("%-*s  %-*s",
			pathWidth, sourceStr,
			pathWidth, target)

		// Apply styling based on selection (cursor always outside styled content)
		// Style the row first, then add colored package indicator separately if needed
		if isSelected {
			b.WriteString(cursor + SelectedListItemStyle.Render(row))
		} else {
			b.WriteString(cursor + MutedTextStyle.Render(row))
		}
		b.WriteString("\n")

		// Show inline detail panel below selected row
		if isSelected && m.showingDetail {
			b.WriteString(m.renderInlineDetail(item, tableWidth))
		}
	}

	// Scroll indicators (always show line, even if empty, for consistent height)
	scrollInfo := ""
	if start > 0 || end < len(m.Paths) {
		scrollInfo = fmt.Sprintf("Showing %d-%d of %d", start+1, end, len(m.Paths))
		if start > 0 {
			scrollInfo = "↑ " + scrollInfo
		}
		if end < len(m.Paths) {
			scrollInfo = scrollInfo + " ↓"
		}
	}
	b.WriteString(SubtitleStyle.Render(scrollInfo))
	b.WriteString("\n")

	// Help or confirmation prompt
	b.WriteString("\n")
	if m.confirmingDelete {
		// Show delete confirmation prompt
		if m.listCursor < len(m.Paths) {
			name := m.Paths[m.listCursor].Entry.Name
			b.WriteString(WarningStyle.Render(fmt.Sprintf("Delete '%s'? ", name)))
			b.WriteString(RenderHelpWithWidth(m.width, "y/enter", "yes", "n/esc", "no"))
		}
	} else if m.showingDetail {
		b.WriteString(RenderHelpWithWidth(m.width,
			"h/←/esc", "close",
			"q", "back",
		))
	} else {
		b.WriteString(RenderHelpWithWidth(m.width,
			"l/→", "details",
			"a", "add",
			"e", "edit",
			"d", "delete",
			"r", "restore",
			"i", "install",
			"m", "install missing",
			"q", "menu",
		))
	}

	return BaseStyle.Render(b.String())
}

func (m Model) calcDetailHeight(item PathItem) int {
	// Calculate how many lines the detail panel takes
	lines := 0

	// Type line
	lines++

	// Description line (if present)
	if item.Entry.Description != "" {
		lines++
	}

	// Root line (if true)
	if item.Entry.Root {
		lines++
	}

	// Package line (if present)
	if item.PkgInstalled != nil {
		lines++
	}

	switch item.EntryType {
	case EntryTypeGit:
		// Repo line
		lines++
		// Branch line (if specified)
		if item.Entry.Branch != "" {
			lines++
		}
	case EntryTypePackage:
		// Package-only entries don't have additional lines here
	default: // EntryTypeConfig
		// Files line (only for non-folders)
		if !item.Entry.IsFolder() {
			lines++
		}
		// Backup line
		lines++
	}

	// Targets header and lines (only for non-package entries)
	if item.EntryType != EntryTypePackage && len(item.Entry.Targets) > 0 {
		lines++ // Targets header
		lines += len(item.Entry.Targets)
	}

	// Filters (if present)
	if len(item.Entry.Filters) > 0 {
		lines++ // Filters header
		for _, f := range item.Entry.Filters {
			if len(f.Include) > 0 || len(f.Exclude) > 0 {
				lines++ // Each filter gets one line
			}
		}
	}

	// Bottom border
	lines++

	return lines
}

func (m Model) renderInlineDetail(item PathItem, tableWidth int) string {
	var detail strings.Builder

	// Type and source info
	switch item.EntryType {
	case EntryTypeGit:
		detail.WriteString("    │ ")
		detail.WriteString(MutedTextStyle.Render("Type: "))
		detail.WriteString(WarningStyle.Render("git"))
		detail.WriteString("\n")
	case EntryTypePackage:
		detail.WriteString("    │ ")
		detail.WriteString(MutedTextStyle.Render("Type: "))
		detail.WriteString(WarningStyle.Render("package"))
		detail.WriteString("\n")
	default: // EntryTypeConfig
		if item.Entry.IsFolder() {
			detail.WriteString("    │ ")
			detail.WriteString(MutedTextStyle.Render("Type: "))
			detail.WriteString(WarningStyle.Render("folder"))
			detail.WriteString("\n")
		} else {
			detail.WriteString("    │ ")
			detail.WriteString(MutedTextStyle.Render("Type: "))
			detail.WriteString(fmt.Sprintf("%d files", len(item.Entry.Files)))
			detail.WriteString("\n")
		}
	}

	// Description (if present)
	if item.Entry.Description != "" {
		detail.WriteString("    │ ")
		detail.WriteString(MutedTextStyle.Render("Description: "))
		detail.WriteString(item.Entry.Description)
		detail.WriteString("\n")
	}

	// Root flag (if true)
	if item.Entry.Root {
		detail.WriteString("    │ ")
		detail.WriteString(MutedTextStyle.Render("Root: "))
		detail.WriteString(WarningStyle.Render("yes"))
		detail.WriteString("\n")
	}

	// Package info (if present)
	if item.PkgInstalled != nil {
		detail.WriteString("    │ ")
		detail.WriteString(MutedTextStyle.Render("Package: "))
		detail.WriteString(item.PkgMethod)
		if *item.PkgInstalled {
			detail.WriteString(" " + SuccessStyle.Render("(installed)"))
		} else {
			detail.WriteString(" " + ErrorStyle.Render("(not installed)"))
		}
		detail.WriteString("\n")
	}

	// Type-specific fields
	switch item.EntryType {
	case EntryTypeGit:
		// Repo URL
		detail.WriteString("    │ ")
		detail.WriteString(MutedTextStyle.Render("Repo: "))
		detail.WriteString(PathBackupStyle.Render(item.Entry.Repo))
		detail.WriteString("\n")

		// Branch (if specified)
		if item.Entry.Branch != "" {
			detail.WriteString("    │ ")
			detail.WriteString(MutedTextStyle.Render("Branch: "))
			detail.WriteString(item.Entry.Branch)
			detail.WriteString("\n")
		}
	case EntryTypePackage:
		// Package-only entries show manager info in package section above
	default: // EntryTypeConfig
		// Files list (only for non-folders)
		if !item.Entry.IsFolder() {
			detail.WriteString("    │ ")
			detail.WriteString(MutedTextStyle.Render("Files: "))
			detail.WriteString(strings.Join(item.Entry.Files, ", "))
			detail.WriteString("\n")
		}

		// Backup path
		detail.WriteString("    │ ")
		detail.WriteString(MutedTextStyle.Render("Backup: "))
		detail.WriteString(PathBackupStyle.Render(item.Entry.Backup))
		detail.WriteString("\n")
	}

	// Targets by OS (only for non-package entries)
	if item.EntryType != EntryTypePackage && len(item.Entry.Targets) > 0 {
		detail.WriteString("    │ ")
		detail.WriteString(MutedTextStyle.Render("Targets:"))
		detail.WriteString("\n")
		for os, target := range item.Entry.Targets {
			detail.WriteString("    │   ")
			osLabel := fmt.Sprintf("%-8s ", os+":")
			detail.WriteString(MutedTextStyle.Render(osLabel))
			detail.WriteString(PathTargetStyle.Render(unexpandHome(target)))
			detail.WriteString("\n")
		}
	}

	// Filters (if present)
	if len(item.Entry.Filters) > 0 {
		detail.WriteString("    │ ")
		detail.WriteString(MutedTextStyle.Render("Filters:"))
		detail.WriteString("\n")
		for _, f := range item.Entry.Filters {
			detail.WriteString("    │   ")
			filterParts := []string{}
			for k, v := range f.Include {
				filterParts = append(filterParts, fmt.Sprintf("%s=%s", k, v))
			}
			for k, v := range f.Exclude {
				filterParts = append(filterParts, fmt.Sprintf("!%s=%s", k, v))
			}
			if len(filterParts) > 0 {
				detail.WriteString(strings.Join(filterParts, ", "))
				detail.WriteString("\n")
			}
		}
	}

	// Bottom line extending to table width
	detail.WriteString("    └")
	bottomWidth := tableWidth - 5
	if bottomWidth < 10 {
		bottomWidth = 10
	}
	detail.WriteString(strings.Repeat("─", bottomWidth))
	detail.WriteString("\n")

	return detail.String()
}

func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	if maxLen <= 3 {
		return s[:maxLen]
	}
	return s[:maxLen-3] + "..."
}

// unexpandHome converts expanded home directory paths back to ~ for display
func unexpandHome(path string) string {
	home, err := os.UserHomeDir()
	if err != nil || home == "" {
		return path
	}
	if strings.HasPrefix(path, home) {
		return "~" + path[len(home):]
	}
	return path
}

// requiresSudo returns true if the package manager method requires sudo
func requiresSudo(method string) bool {
	switch method {
	case "pacman", "apt", "dnf":
		return true
	}
	return false
}
