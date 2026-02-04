package tui

import (
	"fmt"

	"github.com/charmbracelet/bubbles/table"
)

// TableRow wraps table.Row with hierarchy metadata
type TableRow struct {
	Data       table.Row // Actual display data [name, status, info, path]
	Level      int       // 0 = application, 1 = sub-entry
	TreeChar   string    // "▶ ", "▼ ", "├─", "└─"
	IsExpanded bool
	AppIndex   int
	SubIndex   int       // -1 for application rows
	State      PathState // For badge rendering
}

// flattenApplications converts hierarchical apps to flat table rows
func flattenApplications(apps []ApplicationItem, osType string) []TableRow {
	var rows []TableRow

	for appIdx, app := range apps {
		// Level 0: Application row
		expandChar := "▶ "
		if app.Expanded {
			expandChar = "▼ "
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
			Level:      0,
			TreeChar:   expandChar,
			IsExpanded: app.Expanded,
			AppIndex:   appIdx,
			SubIndex:   -1,
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
					Level:    1,
					TreeChar: treeChar,
					AppIndex: appIdx,
					SubIndex: subIdx,
					State:    subItem.State,
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
