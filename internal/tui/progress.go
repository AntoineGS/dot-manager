package tui

import (
	"fmt"
	"os"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
	case "r":
		// Return to menu for another operation
		m.Screen = ScreenMenu
		// Reset selections
		for i := range m.Paths {
			m.Paths[i].Selected = true
		}
		m.pathCursor = 0
		m.scrollOffset = 0
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
				visibleRows := m.viewHeight - 9
				if visibleRows < 3 {
					visibleRows = 3
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
	b.WriteString(TitleStyle.Render("󰋗  List"))
	b.WriteString("\n\n")

	// Calculate column widths based on terminal width
	// Reserve space for: padding (4) + cursor (2) + separators (6) + minimum content
	availWidth := m.width - 12
	if availWidth < 60 {
		availWidth = 60
	}

	// Column widths: Name (20%), Type (10%), Backup (35%), Target (35%)
	nameWidth := availWidth * 20 / 100
	if nameWidth < 12 {
		nameWidth = 12
	}
	typeWidth := 8
	pathWidth := (availWidth - nameWidth - typeWidth) / 2

	// Total table width: cursor(2) + name + sep(2) + type + sep(2) + backup + sep(2) + target
	tableWidth := 2 + nameWidth + 2 + typeWidth + 2 + pathWidth + 2 + pathWidth

	// Table header (with space for cursor)
	headerStyle := PathNameStyle.Bold(true)
	header := fmt.Sprintf("  %-*s  %-*s  %-*s  %-*s",
		nameWidth, "Name",
		typeWidth, "Type",
		pathWidth, "Source",
		pathWidth, "Target")
	b.WriteString(headerStyle.Render(header))
	b.WriteString("\n")

	// Header separator
	separator := "  " + strings.Repeat("─", nameWidth) + "──" +
		strings.Repeat("─", typeWidth) + "──" +
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
	// Subtract lines for: title block (3), header+separator (2), footer (4)
	maxTableRows := m.viewHeight - 9
	if maxTableRows < 3 {
		maxTableRows = 3
	}

	// Calculate how many rows we can show
	maxVisible := maxTableRows
	if m.showingDetail {
		maxVisible = maxTableRows - detailHeight
		if maxVisible < 1 {
			maxVisible = 1
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
		if item.EntryType == EntryTypeGit {
			typeStr = "git"
			sourceStr = truncateStr(item.Entry.Repo, pathWidth)
		} else {
			if item.Entry.IsFolder() {
				typeStr = "folder"
			} else {
				typeStr = fmt.Sprintf("%d files", len(item.Entry.Files))
			}
			sourceStr = truncateStr(item.Entry.Backup, pathWidth)
		}

		// Truncate paths if needed (show config-style values with ~)
		name := item.Entry.Name
		// Add root indicator
		if item.Entry.Root {
			name = "🔒 " + name
		}
		name = truncateStr(name, nameWidth)
		target := truncateStr(unexpandHome(item.Entry.Targets[m.Platform.OS]), pathWidth)

		// Build row
		row := fmt.Sprintf("%-*s  ", nameWidth, name)
		row += fmt.Sprintf("%-*s  ", typeWidth, typeStr)
		row += fmt.Sprintf("%-*s  ", pathWidth, sourceStr)
		row += fmt.Sprintf("%-*s", pathWidth, target)

		// Apply styling based on selection
		if isSelected {
			b.WriteString(SelectedListItemStyle.Render(cursor + row))
		} else {
			b.WriteString(cursor + PathTargetStyle.Render(row))
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

	// Help
	b.WriteString("\n")
	if m.showingDetail {
		b.WriteString(RenderHelp(
			"h/←/esc", "close",
			"q", "back",
		))
	} else {
		b.WriteString(RenderHelp(
			"↑/k ↓/j", "navigate",
			"l/→", "details",
			"a", "add",
			"e", "edit",
			"h/←", "back",
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

	if item.EntryType == EntryTypeGit {
		// Repo line
		lines++
		// Branch line (if specified)
		if item.Entry.Branch != "" {
			lines++
		}
	} else {
		// Files line (only for non-folders)
		if !item.Entry.IsFolder() {
			lines++
		}
		// Backup line
		lines++
	}

	// Targets header
	lines++

	// One line per target OS
	lines += len(item.Entry.Targets)

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
	if item.EntryType == EntryTypeGit {
		detail.WriteString("    │ ")
		detail.WriteString(MutedTextStyle.Render("Type: "))
		detail.WriteString(WarningStyle.Render("git"))
		detail.WriteString("\n")
	} else if item.Entry.IsFolder() {
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

	// Type-specific fields
	if item.EntryType == EntryTypeGit {
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
	} else {
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

	// Targets by OS
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
