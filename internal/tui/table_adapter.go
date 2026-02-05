package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
)

// TableRow wraps table.Row with hierarchy metadata
type TableRow struct {
	Data            table.Row // Actual display data [name, status, info, path] or [name, status, info, backup, path]
	Level           int       // 0 = application, 1 = sub-entry
	TreeChar        string    // "▶ ", "▼ ", "├─", "└─"
	IsExpanded      bool
	AppIndex        int       // Index in filtered array (for display)
	AppName         string    // Application name for lookup in m.Applications
	SubIndex        int       // -1 for application rows
	State           PathState // For badge rendering
	StatusAttention bool      // Status column needs attention
	InfoAttention   bool      // Info column needs attention
	BackupPath      string    // Backup/source path for sub-entries (empty for app rows)
}

// flattenApplications converts hierarchical apps to flat table rows
func flattenApplications(apps []ApplicationItem, osType string, filterEnabled bool) []TableRow {
	var rows []TableRow

	for appIdx, app := range apps {
		// Skip filtered apps when filter is enabled
		if filterEnabled && app.IsFiltered {
			continue
		}

		// Level 0: Application row
		expandChar := "  " // Default padding for apps with no sub-items
		if len(app.SubItems) > 0 {
			expandChar = "▶ "
			if app.Expanded {
				expandChar = "▼ "
			}
		}

		// Determine status text
		statusText := getApplicationStatus(app)

		// Entry count text
		entryText := "entries"
		if len(app.SubItems) == 1 {
			entryText = "entry"
		}
		entryCount := fmt.Sprintf("%d %s", len(app.SubItems), entryText)

		rows = append(rows, TableRow{
			Data: table.Row{
				expandChar + app.Application.Name,
				statusText,
				entryCount,
				"", // No path for app rows
			},
			Level:           0,
			TreeChar:        expandChar,
			IsExpanded:      app.Expanded,
			AppIndex:        appIdx,
			AppName:         app.Application.Name,
			SubIndex:        -1,
			StatusAttention: needsAttention(statusText),
			InfoAttention:   appInfoNeedsAttention(app),
		})

		// Level 1: Sub-entry rows (if expanded)
		if app.Expanded {
			for subIdx, subItem := range app.SubItems {
				treeChar := "├─"
				if subIdx == len(app.SubItems)-1 {
					treeChar = "└─"
				}

				// Type info
				typeInfo := getTypeInfo(subItem)

				// Get original unexpanded target from config (with ~ and relative paths)
				displayTarget := subItem.SubEntry.GetTarget(osType)

				rows = append(rows, TableRow{
					Data: table.Row{
						"  " + treeChar + " " + subItem.SubEntry.Name,
						subItem.State.String(),
						typeInfo,
						displayTarget, // Show original config path, not expanded
					},
					Level:           1,
					TreeChar:        treeChar,
					AppIndex:        appIdx,
					AppName:         app.Application.Name,
					SubIndex:        subIdx,
					State:           subItem.State,
					StatusAttention: needsAttention(subItem.State.String()),
					InfoAttention:   false, // Sub-entries don't have info attention
					BackupPath:      subItem.SubEntry.Backup,
				})
			}
		}
	}

	return rows
}

// getApplicationStatus determines status text for application row
func getApplicationStatus(app ApplicationItem) string {
	if app.IsFiltered {
		return StatusFiltered
	}

	allLinked := true
	for _, sub := range app.SubItems {
		if sub.State != StateLinked {
			allLinked = false
			break
		}
	}

	if allLinked {
		return StatusInstalled
	}

	return StatusMissing
}

// getTypeInfo returns type information for a sub-entry
func getTypeInfo(subItem SubEntryItem) string {
	if subItem.SubEntry.IsFolder() {
		return TypeFolder
	}

	fileCount := len(subItem.SubEntry.Files)
	if fileCount == 1 {
		return "1 file"
	}

	return fmt.Sprintf("%d files", fileCount)
}

// needsAttention returns true if the status text indicates something needs attention
func needsAttention(status string) bool {
	return status != StatusInstalled && status != StateLinked.String()
}

// appInfoNeedsAttention returns true if the application has any configs that need attention
func appInfoNeedsAttention(app ApplicationItem) bool {
	// Filtered apps don't need attention
	if app.IsFiltered {
		return false
	}

	// Check if any sub-entry is not linked
	for _, sub := range app.SubItems {
		if sub.State != StateLinked {
			return true
		}
	}

	return false
}
