package tui

import (
	"cmp"
	"fmt"
	"slices"

	"charm.land/bubbles/v2/progress"
	tea "charm.land/bubbletea/v2"
	tuiops "github.com/AntoineGS/tidydots/internal/tui/operations"
)

// BatchOperationMsg is an alias for tuiops.BatchOperationMsg so that all
// existing code in this package continues to compile without modification.
type BatchOperationMsg = tuiops.BatchOperationMsg

// BatchCompleteMsg is an alias for tuiops.BatchCompleteMsg so that all
// existing code in this package continues to compile without modification.
type BatchCompleteMsg = tuiops.BatchCompleteMsg

// batchRestoreItem identifies one selected sub-entry in a batch restore.
type batchRestoreItem struct {
	name   string
	appIdx int
	subIdx int
}

// batchRestoreConfigsDoneMsg carries the results of the config half of a batch
// restore, plus the setup entries that still have to run.
//
// The two halves cannot share one command: a setup entry is a subprocess that
// may prompt for a sudo password, so it has to be dispatched through tea.Exec
// (which releases the terminal), one at a time. Config entries are plain file
// operations and stay in this single background command, exactly as before.
type batchRestoreConfigsDoneMsg struct {
	results      []ResultItem
	setups       []setupRunItem
	successCount int
	failCount    int
}

// collectBatchRestoreItems returns every selected sub-entry, de-duplicated
// against apps that are selected as a whole. Iterating m.Applications (rather
// than the selection maps) makes the batch order deterministic: model order,
// not map order.
func (m Model) collectBatchRestoreItems() []batchRestoreItem {
	var items []batchRestoreItem

	for appIdx, app := range m.Applications {
		if app.IsFiltered {
			continue
		}
		name := app.Application.Name

		// Whole app selected: every sub-entry is included.
		if m.selectedApps[name] {
			for subIdx := range app.SubItems {
				items = append(items, batchRestoreItem{
					appIdx: appIdx,
					subIdx: subIdx,
					name:   name + "/" + app.SubItems[subIdx].SubEntry.Name,
				})
			}

			continue
		}

		// Standalone selected sub-entries.
		for subIdx, sub := range app.SubItems {
			if m.selectedSubEntries[subEntryKey{app: name, sub: sub.SubEntry.Name}] {
				items = append(items, batchRestoreItem{
					appIdx: appIdx,
					subIdx: subIdx,
					name:   name + "/" + sub.SubEntry.Name,
				})
			}
		}
	}

	return items
}

// executeBatchRestore executes restore operations for all selected items.
// Config entries are restored in this command; setup entries are handed back on
// the resulting message so they can be run through tea.Exec (see
// batchRestoreConfigsDoneMsg). Before this split, executeBatchRestore called
// performRestoreSubEntry on every selected item with no filter, so each setup
// entry failed as "Not a config entry" and inflated failCount.
func (m Model) executeBatchRestore(forceRender bool) tea.Cmd {
	var (
		configs []batchRestoreItem
		setups  []setupRunItem
	)

	for _, item := range m.collectBatchRestoreItems() {
		sub := m.Applications[item.appIdx].SubItems[item.subIdx]

		if sub.SubEntry.IsSetup() {
			setups = append(setups, setupRunItem{
				appIdx: item.appIdx,
				subIdx: item.subIdx,
				name:   item.name,
				sub:    sub,
			})

			continue
		}

		configs = append(configs, item)
	}

	// Execute the config restores sequentially in the background.
	return func() tea.Msg {
		previous := m.Manager.ForceRender
		m.Manager.ForceRender = forceRender
		defer func() { m.Manager.ForceRender = previous }()

		results := make([]ResultItem, 0, len(configs))
		successCount := 0
		failCount := 0

		for _, item := range configs {
			subItem := m.Applications[item.appIdx].SubItems[item.subIdx]

			success, message := m.performRestoreSubEntry(subItem)

			results = append(results, ResultItem{
				Name:    item.name,
				Success: success,
				Message: message,
			})

			if success {
				successCount++
			} else {
				failCount++
			}
		}

		return batchRestoreConfigsDoneMsg{
			results:      results,
			setups:       setups,
			successCount: successCount,
			failCount:    failCount,
		}
	}
}

// handleBatchRestoreConfigsDone starts the queued setup entries once the config
// half of a batch restore has finished. With no setup entries the batch
// completes exactly as it did before.
func (m Model) handleBatchRestoreConfigsDone(msg batchRestoreConfigsDoneMsg) (tea.Model, tea.Cmd) {
	if len(msg.setups) == 0 {
		return m, func() tea.Msg {
			return BatchCompleteMsg{
				Results:      msg.results,
				SuccessCount: msg.successCount,
				FailCount:    msg.failCount,
			}
		}
	}

	m.results = msg.results
	m.batchSuccessCount = msg.successCount
	m.batchFailCount = msg.failCount

	return m, m.startSetupRun(msg.setups, true)
}

// executeBatchInstall executes package installation for all selected apps.
// Returns a command that processes packages sequentially.
func (m Model) executeBatchInstall() tea.Cmd {
	// Collect all selected apps with packages to install
	var packages []PackageItem

	for _, app := range m.Applications {
		if app.IsFiltered {
			continue
		}
		if !m.selectedApps[app.Application.Name] {
			continue
		}

		// Only install if package exists and is not already installed
		if app.PkgInstalled != nil && !*app.PkgInstalled && app.Application.HasPackage() {
			// Convert Application to PackageItem
			pkg := PackageItem{
				Name:     app.Application.Name,
				Package:  app.Application.Package,
				Method:   app.PkgMethod,
				Selected: true,
			}
			packages = append(packages, pkg)
		}
	}

	// If no packages to install, return complete immediately
	if len(packages) == 0 {
		return func() tea.Msg {
			return BatchCompleteMsg{
				Results:      []ResultItem{},
				SuccessCount: 0,
				FailCount:    0,
			}
		}
	}

	// Set pending packages for sequential installation
	// Note: This uses the existing package installation logic
	// The actual installation will be handled by installNextPackage
	return func() tea.Msg {
		// Signal that we need to start package installation
		// This will be handled in the Update method
		return initBatchInstallMsg{packages: packages}
	}
}

// initBatchInstallMsg is an internal message to initialize batch package installation.
type initBatchInstallMsg struct {
	packages []PackageItem
}

// executeBatchDelete executes delete operations for all selected items.
// Returns a command that processes deletions in reverse order to avoid index shifting.
func (m Model) executeBatchDelete() tea.Cmd {
	// Collect all items to delete with their config indices
	type deleteItem struct {
		configAppIdx int // Index in m.Config.Applications (for actual deletion)
		subIdx       int // Index in app.Entries, -1 for app deletion
		name         string
		sortKey      int // For sorting (higher appIdx should be deleted first)
	}

	var items []deleteItem

	for _, app := range m.Applications {
		if app.IsFiltered {
			continue
		}
		name := app.Application.Name

		// Whole app selected: delete the application.
		if m.selectedApps[name] {
			configIdx := m.findConfigApplicationIndex(name)
			if configIdx >= 0 {
				items = append(items, deleteItem{
					configAppIdx: configIdx,
					subIdx:       -1,
					name:         name,
					sortKey:      configIdx * 1000, // App deletions get higher priority per app
				})
			}

			continue
		}

		// Standalone selected sub-entries.
		configAppIdx := m.findConfigApplicationIndex(name)
		if configAppIdx < 0 {
			continue
		}

		for _, sub := range app.SubItems {
			if !m.selectedSubEntries[subEntryKey{app: name, sub: sub.SubEntry.Name}] {
				continue
			}

			// Find the config index for the sub-entry
			configSubIdx := m.findConfigSubEntryIndex(configAppIdx, sub.SubEntry.Name)

			if configSubIdx >= 0 {
				items = append(items, deleteItem{
					configAppIdx: configAppIdx,
					subIdx:       configSubIdx,
					name:         name + "/" + sub.SubEntry.Name,
					sortKey:      configAppIdx*1000 + configSubIdx, // Sub-entries sorted within app
				})
			}
		}
	}

	// Sort items in reverse order (highest index first) to avoid index shifting
	// Use stable sort to maintain order within same app
	slices.SortStableFunc(items, func(a, b deleteItem) int {
		return cmp.Compare(b.sortKey, a.sortKey)
	})

	// Execute deletions
	return func() tea.Msg {
		results := make([]ResultItem, 0, len(items))
		successCount := 0
		failCount := 0

		// Track which apps have been deleted (by config index) to avoid double deletion
		deletedApps := make(map[int]bool)

		for _, item := range items {
			// Skip if app was already deleted
			if deletedApps[item.configAppIdx] && item.subIdx < 0 {
				continue
			}

			var err error
			if item.subIdx >= 0 {
				// Delete sub-entry only if app hasn't been deleted
				if !deletedApps[item.configAppIdx] {
					err = m.deleteSubEntry(item.configAppIdx, item.subIdx)
					// Check if this was the last sub-entry (app gets deleted automatically)
					if err == nil && len(m.Config.Applications) <= item.configAppIdx {
						deletedApps[item.configAppIdx] = true
					}
				}
			} else {
				// Delete entire app
				err = m.deleteApplication(item.configAppIdx)
				if err == nil {
					deletedApps[item.configAppIdx] = true
				}
			}

			success := err == nil
			message := ""
			if success {
				message = "Deleted successfully"
				if m.DryRun || (m.Manager != nil && m.Manager.DryRun) {
					message = "Would delete configuration (dry-run)"
				}
				successCount++
			} else {
				message = fmt.Sprintf("Failed: %v", err)
				failCount++
			}

			results = append(results, ResultItem{
				Name:    item.name,
				Success: success,
				Message: message,
			})
		}

		return BatchCompleteMsg{
			Results:      results,
			SuccessCount: successCount,
			FailCount:    failCount,
		}
	}
}

// initBatchProgress initializes the progress bar model for batch operations.
func initBatchProgress() progress.Model {
	prog := progress.New(
		progress.WithDefaultBlend(),
		progress.WithWidth(60),
	)
	return prog
}
