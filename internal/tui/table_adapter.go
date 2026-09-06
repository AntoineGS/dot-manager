package tui

import (
	"fmt"
	"slices"

	"charm.land/bubbles/v2/table"
)

// filterActionableApplications returns a cloned application list containing
// only package-missing apps and sub-entries that need an action. The source
// list is never changed because it is also used for stable cursor resolution.
func filterActionableApplications(apps []ApplicationItem, includeLoading bool) []ApplicationItem {
	filtered := make([]ApplicationItem, 0, len(apps))
	for _, app := range apps {
		appCopy := app
		appCopy.SubItems = make([]SubEntryItem, 0, len(app.SubItems))
		for _, sub := range app.SubItems {
			if subEntryActionable(sub.State, includeLoading) {
				appCopy.SubItems = append(appCopy.SubItems, sub)
			}
		}
		if applicationActionable(app, includeLoading) {
			filtered = append(filtered, appCopy)
		}
	}
	return slices.Clip(filtered)
}

// applicationPackageActionable reports whether an application's package needs
// installation. An unresolved or unavailable package method is not actionable.
func applicationPackageActionable(app ApplicationItem) bool {
	return app.Application.HasPackage() && app.PkgInstalled != nil && !*app.PkgInstalled
}

// subEntryActionable applies the canonical path-state action predicate. Loading
// is included only while the TUI is waiting for its initial checks.
func subEntryActionable(state PathState, includeLoading bool) bool {
	return state.Actionable() || includeLoading && state == StateLoading
}

// flattenApplications converts hierarchical apps to flat table rows
func flattenApplications(apps []ApplicationItem, osType string, filterEnabled bool) []TableRow {
	var rows []TableRow

	for _, app := range apps {
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

		infoState := appInfoMaxState(app)

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
			AppName:         app.Application.Name,
			SubIndex:        -1,
			StatusAttention: needsAttention(statusText),
			InfoAttention:   infoState != StateLinked,
			InfoState:       infoState,
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
					AppName:  app.Application.Name,
					SubName:  subItem.SubEntry.Name,
					// subItem.Index, not subIdx: under a search filter this app is a
					// copy whose SubItems hold only the matching entries, so subIdx is
					// a position in that copy. Cursor actions index the real slice.
					SubIndex:        subItem.Index,
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

// getApplicationStatus determines status text for application row based on
// package install state only. Config sub-entry states are reflected in the
// info column via appInfoNeedsAttention.
func getApplicationStatus(app ApplicationItem) string {
	if app.IsFiltered {
		return StatusFiltered
	}

	if app.PkgInstalled == nil {
		if app.Application.HasPackage() {
			// If method is already resolved to "none", no manager is available for this OS
			if app.PkgMethod == TypeNone {
				return StatusUnknown
			}
			return StatusLoading
		}
		return StatusUnknown
	}

	if *app.PkgInstalled {
		return StatusInstalled
	}

	return StatusMissing
}

// getTypeInfo returns type information for a sub-entry
func getTypeInfo(subItem SubEntryItem) string {
	// A setup entry deploys no files; reporting a file count would describe it
	// as an empty config entry ("0 files").
	if subItem.SubEntry.IsSetup() {
		if subItem.CheckError != "" {
			return subItem.CheckError
		}
		return TypeSetup
	}

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
	return status != StatusInstalled &&
		status != StatusUnknown &&
		status != StatusLoading &&
		status != StateLinked.String() &&
		status != StateSetupOk.String()
}

// stateSeverity returns a numeric severity for a PathState.
// Higher values indicate more urgent states that should take priority in the info column.
func stateSeverity(s PathState) int {
	return s.Severity()
}

// appInfoMaxState returns the highest-severity sub-entry state for an application.
// Returns StateLinked when no sub-entry needs attention or the app is filtered.
func appInfoMaxState(app ApplicationItem) PathState {
	if app.IsFiltered {
		return StateLinked
	}

	maxState := StateLinked
	maxSev := 0

	for _, sub := range app.SubItems {
		if sev := stateSeverity(sub.State); sev > maxSev {
			maxSev = sev
			maxState = sub.State
		}
	}

	return maxState
}
