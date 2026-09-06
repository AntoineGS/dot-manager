package tui

import (
	"path/filepath"

	tea "charm.land/bubbletea/v2"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/manager"
	"github.com/AntoineGS/tidydots/internal/platform"
)

// detectSetupPathState reports the state of a setup sub-entry by running its
// check command. A nil manager means the check cannot be run, so the entry is
// reported as satisfied rather than falsely flagged.
func detectSetupPathState(sub config.SubEntry, mgr *manager.Manager) PathState {
	if mgr == nil || mgr.IsSetupApplied(sub) {
		return StateSetupOk
	}
	return StateSetupNeeded
}

// detectSubEntryState determines the state of a sub-entry item.
//
// This method runs synchronously on the bubbletea UI goroutine (called from
// refreshApplicationStates and reinitPreservingState), so it must never shell
// out. A setup entry's state can only be determined by running its check
// command (see detectSetupPathState), which is real subprocess execution —
// doing that here would visibly stall the UI. Instead, setup entries are left
// at StateLoading and resolved asynchronously; see
// checkLoadingSubEntryStatesCmd, which is dispatched by the callers above and
// by detectSubEntryStateStatic (the goroutine-safe variant used by the async
// detection pipeline).
func (m *Model) detectSubEntryState(item *SubEntryItem) PathState {
	if item.SubEntry.IsSetup() {
		return StateLoading
	}

	targetPath := config.ExpandPath(item.Target, m.Platform.EnvVars)
	backupPath := m.resolvePath(item.SubEntry.Backup)

	st := detectConfigState(backupPath, targetPath, item.SubEntry.IsFolder(), item.SubEntry.Files, item.SubEntry.IsCopy())

	if st == StateLinked && item.SubEntry.IsConfig() && !item.SubEntry.IsCopy() && m.Manager != nil {
		if m.Manager.HasOutdatedTemplates(backupPath, item.SubEntry.Files) {
			return StateOutdated
		}
		if m.Manager.HasModifiedRenderedFiles(backupPath, item.SubEntry.Files) {
			return StateModified
		}
	}

	return st
}

// countInitialStateChecks counts how many async state checks Init() will dispatch.
// This mirrors the filtering logic in checkPackageStatesCmd and checkSubEntryStatesCmd
// so that pendingStateChecks can be set before Init() runs.
func (m Model) countInitialStateChecks() int {
	count := 0
	for _, app := range m.Applications {
		if app.IsFiltered {
			continue
		}
		if app.Application.HasPackage() {
			count++
		}
		count += len(app.SubItems)
	}
	return count
}

// checkPackageStatesCmd returns a tea.Cmd that detects package methods and checks install statuses.
// Each package gets its own concurrent command so results stream in individually.
// It also returns the number of checks dispatched so the caller can track pending work.
func (m Model) checkPackageStatesCmd() (tea.Cmd, int) {
	var cmds []tea.Cmd

	for i, app := range m.Applications {
		if app.IsFiltered || !app.Application.HasPackage() {
			continue
		}
		cmds = append(cmds, m.packageStateCheckCmd(i))
	}

	return tea.Batch(cmds...), len(cmds)
}

// checkSubEntryStatesCmd returns a tea.Cmd that checks all sub-entry states.
// Each sub-entry gets its own concurrent command so results stream in individually.
// Filtered apps are skipped; use checkFilteredStatesCmd when the filter is toggled off.
// It also returns the number of checks dispatched so the caller can track pending work.
func (m Model) checkSubEntryStatesCmd() (tea.Cmd, int) {
	var cmds []tea.Cmd

	for i, app := range m.Applications {
		if app.IsFiltered {
			continue
		}
		for j := range app.SubItems {
			cmds = append(cmds, m.subEntryStateCheckCmd(i, j))
		}
	}

	return tea.Batch(cmds...), len(cmds)
}

// checkUncheckedPackageStatesCmd triggers async checks for apps whose package state hasn't been resolved yet.
// Called after saving a new or edited application to resolve "Loading..." status.
// It also returns the number of checks dispatched so the caller can track pending work.
func (m Model) checkUncheckedPackageStatesCmd() (tea.Cmd, int) {
	var cmds []tea.Cmd

	for i, app := range m.Applications {
		if app.IsFiltered || !app.Application.HasPackage() || app.PkgInstalled != nil {
			continue
		}
		cmds = append(cmds, m.packageStateCheckCmd(i))
	}

	return tea.Batch(cmds...), len(cmds)
}

// checkFilteredStatesCmd triggers async checks for filtered apps that haven't been scanned yet.
// Called when the user toggles the filter off, revealing previously-hidden apps.
// It also returns the number of checks dispatched so the caller can track pending work.
func (m Model) checkFilteredStatesCmd() (tea.Cmd, int) {
	// A false application condition is visibility-only when the filter is off.
	// Its raw config remains editable, but it must never become operational.
	return nil, 0
}

func (m Model) packageStateCheckCmd(appIndex int) tea.Cmd {
	app := m.Applications[appIndex]
	pkg := app.Application.Package
	name := app.Application.Name
	osType := m.Platform.OS
	return func() tea.Msg {
		method := getPackageInstallMethodFromPackage(pkg, osType)
		installed := false
		if method != TypeNone {
			installed = isPackageInstalledFromPackage(pkg, method, name, osType)
		}
		return pkgCheckResultMsg{appIndex: appIndex, method: method, installed: installed}
	}
}

func (m Model) subEntryStateCheckCmd(appIndex, subIndex int) tea.Cmd {
	subItem := m.Applications[appIndex].SubItems[subIndex]
	plat, cfg, mgr := m.Platform, m.Config, m.Manager
	return func() tea.Msg {
		state := detectSubEntryStateStatic(subItem, plat, cfg, mgr)
		return stateCheckResultMsg{appIndex: appIndex, subIndex: subIndex, state: state}
	}
}

func (m *Model) refreshAllStates() tea.Cmd {
	var cmds []tea.Cmd
	for i := range m.Applications {
		if m.Applications[i].IsFiltered {
			continue
		}
		if m.Applications[i].Application.HasPackage() {
			m.Applications[i].PkgInstalled = nil
			cmds = append(cmds, m.packageStateCheckCmd(i))
		}
		for j := range m.Applications[i].SubItems {
			m.Applications[i].SubItems[j].State = StateLoading
			cmds = append(cmds, m.subEntryStateCheckCmd(i, j))
		}
	}
	m.pendingStateChecks += len(cmds)
	m.rebuildTable()
	return tea.Batch(cmds...)
}

func (m *Model) refreshPackageStates(packages []PackageItem) tea.Cmd {
	names := make(map[string]struct{}, len(packages))
	for _, pkg := range packages {
		names[pkg.Name] = struct{}{}
	}

	cmds := make([]tea.Cmd, 0, len(names))
	for i := range m.Applications {
		if m.Applications[i].IsFiltered {
			continue
		}
		if _, ok := names[m.Applications[i].Application.Name]; !ok || !m.Applications[i].Application.HasPackage() {
			continue
		}
		m.Applications[i].PkgInstalled = nil
		cmds = append(cmds, m.packageStateCheckCmd(i))
	}
	m.pendingStateChecks += len(cmds)
	m.rebuildTable()
	return tea.Batch(cmds...)
}

// checkLoadingSubEntryStatesCmd returns a tea.Cmd that resolves sub-entry
// states still stuck at StateLoading after a synchronous rebuild — chiefly
// setup entries, whose state can only be determined by running their check
// command (detectSubEntryState refuses to do that on the UI goroutine and
// leaves them at StateLoading instead). Each check gets its own concurrent
// command so results stream in individually, matching checkSubEntryStatesCmd
// and checkFilteredStatesCmd.
//
// Filtered applications are not operational, even when they are visible with
// the machine filter disabled.
//
// Only setup entries are considered. StateLoading is also the zero value of
// PathState, so a config entry whose initial async check from Init() has not
// landed yet still reads as StateLoading; re-dispatching those would duplicate
// subprocess work that is already in flight (and unbalance pendingStateChecks
// against the messages Init() will still deliver).
// It also returns the number of checks dispatched so the caller can track pending work.
func (m Model) checkLoadingSubEntryStatesCmd() (tea.Cmd, int) {
	var cmds []tea.Cmd

	for i, app := range m.Applications {
		if app.IsFiltered {
			continue
		}
		for j, sub := range app.SubItems {
			if !sub.SubEntry.IsSetup() || sub.State != StateLoading {
				continue
			}
			cmds = append(cmds, m.subEntryStateCheckCmd(i, j))
		}
	}

	return tea.Batch(cmds...), len(cmds)
}

// detectSubEntryStateStatic determines the state of a sub-entry item without using Model receiver.
// This is safe to call from goroutines since it takes explicit dependencies.
func detectSubEntryStateStatic(item SubEntryItem, plat *platform.Platform, cfg *config.Config, mgr *manager.Manager) PathState {
	if item.SubEntry.IsSetup() {
		return detectSetupPathState(item.SubEntry, mgr)
	}

	targetPath := config.ExpandPath(item.Target, plat.EnvVars)
	backupPath := resolvePathStatic(item.SubEntry.Backup, cfg, plat.EnvVars)

	st := detectConfigState(backupPath, targetPath, item.SubEntry.IsFolder(), item.SubEntry.Files, item.SubEntry.IsCopy())

	if st == StateLinked && item.SubEntry.IsConfig() && !item.SubEntry.IsCopy() && mgr != nil {
		if mgr.HasOutdatedTemplates(backupPath, item.SubEntry.Files) {
			return StateOutdated
		}
		if mgr.HasModifiedRenderedFiles(backupPath, item.SubEntry.Files) {
			return StateModified
		}
	}

	return st
}

// resolvePathStatic resolves relative paths against BackupRoot and expands ~ without using Model receiver.
func resolvePathStatic(path string, cfg *config.Config, envVars map[string]string) string {
	expandedPath := config.ExpandPath(path, envVars)

	if filepath.IsAbs(expandedPath) {
		return expandedPath
	}

	expandedBackupRoot := config.ExpandPath(cfg.BackupRoot, envVars)
	return filepath.Join(expandedBackupRoot, expandedPath)
}
