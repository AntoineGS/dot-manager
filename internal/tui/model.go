package tui

import (
	"path/filepath"

	"charm.land/bubbles/v2/key"
	"charm.land/bubbles/v2/progress"
	"charm.land/bubbles/v2/spinner"
	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/manager"
	"github.com/AntoineGS/tidydots/internal/platform"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
	"github.com/AntoineGS/tidydots/internal/tui/detection"
	"github.com/AntoineGS/tidydots/internal/tui/forms"
	tuiops "github.com/AntoineGS/tidydots/internal/tui/operations"
	tuitable "github.com/AntoineGS/tidydots/internal/tui/table"
)

// Screen represents the current screen being displayed in the TUI.
type Screen int

// TUI screen types.
const (
	// ScreenProgress is the progress display screen
	ScreenProgress Screen = iota
	// ScreenResults is the results display screen
	ScreenResults
	// ScreenAddForm is the add/edit form screen
	ScreenAddForm
	// ScreenEntryTypeChooser is the entry type selection screen
	ScreenEntryTypeChooser
	// ScreenSummary is the summary/confirmation screen for batch operations
	ScreenSummary
)

// Operation is an alias for tuiops.Operation so that all existing code in
// this package continues to compile without modification.
type Operation = tuiops.Operation

// TUI operation type constants re-exported from the operations sub-package.
const (
	// OpRestore is the restore operation
	OpRestore = tuiops.OpRestore
	// OpList is the list entries operation
	OpList = tuiops.OpList
	// OpInstallPackages is the install packages operation
	OpInstallPackages = tuiops.OpInstallPackages
	// OpDelete is the delete entries operation
	OpDelete = tuiops.OpDelete
	// OpForceRestore restores configs while discarding rendered-template edits.
	OpForceRestore = tuiops.OpForceRestore
)

// PathState is an alias for tuitable.PathState so that all existing code in
// this package continues to compile without modification.
type PathState = tuitable.PathState

// Path state constants re-exported from the table sub-package.
const (
	// StateLoading indicates state is still being detected.
	StateLoading = tuitable.StateLoading
	// StateReady indicates backup exists and is ready to restore.
	StateReady = tuitable.StateReady
	// StateAdopt indicates no backup but target exists (will adopt).
	StateAdopt = tuitable.StateAdopt
	// StateMissing indicates neither backup nor target exists.
	StateMissing = tuitable.StateMissing
	// StateLinked indicates already symlinked.
	StateLinked = tuitable.StateLinked
	// StateOutdated indicates linked but template source changed since last render.
	StateOutdated = tuitable.StateOutdated
	// StateModified indicates linked but rendered file has user edits.
	StateModified = tuitable.StateModified
	// StateSetupOk indicates a setup entry whose check command passes.
	StateSetupOk = tuitable.StateSetupOk
	// StateSetupNeeded indicates a setup entry whose check command fails.
	StateSetupNeeded = tuitable.StateSetupNeeded
	// StateUnavailable indicates status could not be inspected safely.
	StateUnavailable = tuitable.StateUnavailable
)

// TableRow is an alias for tuitable.Row so that all existing code in
// this package continues to compile without modification.
type TableRow = tuitable.Row

// FormType distinguishes between different form types
type FormType int

// Form types for the add/edit screen.
const (
	// FormNone indicates no active form
	FormNone FormType = iota
	// FormApplication is the application metadata form
	FormApplication
	// FormSubEntry is the sub-entry form
	FormSubEntry
)

// ApplicationForm is an alias for forms.ApplicationForm.
type ApplicationForm = forms.ApplicationForm

// WhenMode is an alias for forms.WhenMode.
type WhenMode = forms.WhenMode

// SubEntryForm is an alias for forms.SubEntryForm.
type SubEntryForm = forms.SubEntryForm

// AddFileMode is an alias for forms.AddFileMode.
type AddFileMode = forms.AddFileMode

// Model holds the state for the TUI application including configuration,
// platform information, current screen, operation mode, and UI state.
type Model struct {
	err                      error
	Config                   *config.Config
	Platform                 *platform.Platform
	Renderer                 config.PathRenderer
	Manager                  *manager.Manager
	subEntryForm             *SubEntryForm
	applicationForm          *ApplicationForm
	entryTypeChooser         *entryTypeChooserState
	searchText               string
	ConfigPath               string
	pendingPackages          []PackageItem
	results                  []ResultItem
	Applications             []ApplicationItem
	HostnameChoices          []string
	searchInput              textinput.Model
	tableRows                []TableRow
	tableCursor              int
	sortColumn               string // "name", "status", or "path"
	sortAscending            bool
	viewHeight               int
	height                   int
	width                    int
	currentPackageIndex      int
	Operation                Operation
	scrollOffset             int
	Screen                   Screen
	activeForm               FormType
	DryRun                   bool
	processing               bool
	searching                bool
	confirmingDeleteSubEntry bool
	confirmingDeleteApp      bool
	confirmingFilterToggle   bool // true when showing filter toggle confirmation
	filterToggleHiddenCount  int  // count of selections that would be hidden
	confirmingActionFilter   bool // true when showing action filter confirmation
	actionFilterHiddenCount  int  // count of selections that would be hidden
	showingDetail            bool
	showingResults           bool
	resultsScrollOffset      int

	// Diff picker state
	showingDiffPicker bool
	diffPickerCursor  int
	diffPickerFiles   []manager.ModifiedTemplate

	// Filter state
	filterEnabled       bool // true to hide filtered apps, false to show all
	actionFilterEnabled bool // true to show only applications with actionable work
	pendingG            bool // true after the first g of a gg motion

	// Selection state for multi-select mode
	selectedApps       map[string]bool      // application name -> selected
	selectedSubEntries map[subEntryKey]bool // (app name, entry name) -> selected
	multiSelectActive  bool                 // true when selections exist

	// Summary screen state
	summaryOperation          Operation // Which batch operation: restore, install, delete
	completedOperation        Operation // Operation represented by the current results
	summaryDoublePress        string    // Track double-press state: "r", "i", or "d"
	summaryTransientSelection bool      // Whether summary selections were created for the current row

	// Setup entry execution state. Setup entries are subprocesses that may
	// prompt for a sudo password, so they run one at a time through tea.Exec
	// (see setup_run.go) rather than inside a plain background command.
	pendingSetups     []setupRunItem // Queued setup entries, in run order
	currentSetupIndex int            // Index of the entry currently running
	setupBatch        bool           // The queue belongs to a multi-select batch operation

	// Batch operation progress state
	spinner           spinner.Model  // Loading spinner for async state detection
	batchProgress     progress.Model // Progress bar for batch operations
	batchCurrentItem  string         // Name of current item being processed
	batchTotalItems   int            // Total items in batch
	batchCurrentIndex int            // Current item index (0-based)
	batchSuccessCount int            // Count of successful operations
	batchFailCount    int            // Count of failed operations

	// Pending async state check counter — avoids rebuilding the table on
	// every single pkgCheckResultMsg / stateCheckResultMsg.  The table is
	// rebuilt only once when the counter reaches 0.
	pendingStateChecks int
}

// PackageItem represents a package to be installed, including its name,
// package configuration, installation method, and selection state.
type PackageItem struct {
	Name     string
	Package  *config.EntryPackage
	Method   string // How it would be installed (pacman, apt, custom, url, none)
	Selected bool
}

// ApplicationItem represents a top-level application with sub-entries
type ApplicationItem struct {
	Application  config.Application
	PkgInstalled *bool
	PkgMethod    string
	SubItems     []SubEntryItem
	Expanded     bool
	IsFiltered   bool // True if this app doesn't match the current filter context
}

// SubEntryItem represents a sub-entry within an application (config or git)
type SubEntryItem struct {
	AppName  string
	Target   string
	SubEntry config.SubEntry
	State    PathState
	// Index is this entry's position in its application's SubItems. It is carried
	// on the item because the search filter (getSearchedApplications) hands the
	// table a compacted copy of SubItems holding only the matching entries: a
	// row's position in that copy is not its position in the real slice, and every
	// cursor action indexes the real slice.
	Index int
}

// ResultItem is an alias for tuiops.ResultItem so that all existing code in
// this package continues to compile without modification.
type ResultItem = tuiops.ResultItem

// NewModel creates and initializes a new TUI model with the given configuration,
// platform information, and dry-run mode. It sets up the initial state including
// loading entries, detecting path states, and initializing the UI.
func NewModel(cfg *config.Config, plat *platform.Platform, dryRun bool) Model {
	return NewModelWithActionFilter(cfg, plat, dryRun, false)
}

// NewModelWithActionFilter creates a TUI model and optionally enables the
// action filter before the initial status checks are dispatched.
func NewModelWithActionFilter(cfg *config.Config, plat *platform.Platform, dryRun, actionFilterEnabled bool) Model {
	// Create template engine for when expression evaluation
	tmplCtx := tmpl.NewContextFromPlatform(plat)
	renderer := tmpl.NewEngine(tmplCtx)

	// Initialize search input
	searchInput := textinput.New()
	searchInput.Placeholder = "type to search..."
	searchInput.CharLimit = 100

	// Initialize loading spinner
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = SpinnerStyle

	m := Model{
		Screen:              ScreenResults, // Start directly in Manage view
		Operation:           OpList,        // Set operation to List (Manage)
		Config:              cfg,
		Platform:            plat,
		Renderer:            renderer,
		DryRun:              dryRun,
		viewHeight:          15,
		width:               80,
		height:              24,
		searchInput:         searchInput,
		spinner:             s,
		sortColumn:          SortColumnName, // Default sort by name
		sortAscending:       true,           // Ascending by default
		filterEnabled:       true,           // Filter ON by default
		actionFilterEnabled: actionFilterEnabled,
		selectedApps:        make(map[string]bool),
		selectedSubEntries:  make(map[subEntryKey]bool),
		multiSelectActive:   false,
	}

	// Initialize applications for hierarchical view
	m.initApplicationItems()

	// Pre-calculate the number of async state checks that Init() will dispatch.
	// Init() uses a value receiver and cannot persist state mutations, so we
	// compute the count here while we still have a mutable model.
	m.pendingStateChecks = m.countInitialStateChecks()
	if actionFilterEnabled {
		m.rebuildTable()
	}

	return m
}

// Init initializes the TUI model and returns any initial commands to run.
// This is part of the Bubble Tea model interface.
func (m Model) Init() tea.Cmd {
	pkgCmd, _ := m.checkPackageStatesCmd()
	subCmd, _ := m.checkSubEntryStatesCmd()

	// pendingStateChecks is pre-calculated in NewModel() because Init()
	// uses a value receiver and cannot persist state mutations.

	return tea.Batch(
		m.spinner.Tick,
		pkgCmd,
		subCmd,
	)
}

// Update processes messages and updates the model state accordingly.
// This is part of the Bubble Tea model interface.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case pkgCheckResultMsg:
		return m.handlePkgCheckResult(msg)

	case stateCheckResultMsg:
		return m.handleStateCheckResult(msg)

	case batchRestoreConfigsDoneMsg:
		return m.handleBatchRestoreConfigsDone(msg)

	case setupRunMsg:
		return m.handleSetupRunResult(msg)

	case spinner.TickMsg:
		if m.hasLoadingItems() {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKeyPress(msg)

	case tea.MouseClickMsg:
		return m.handleMouseClickEvent(msg)
	case tea.MouseWheelMsg:
		return m.handleMouseWheelEvent(msg)

	case editorLaunchCompleteMsg:
		// Editor exited - refresh application states since template may have changed.
		// refreshApplicationStates never shells out; setup entries land at
		// StateLoading and are resolved by the dispatched cmd below.
		m.refreshApplicationStates()
		m.rebuildTable()
		if msg.err != nil {
			m.results = []ResultItem{{
				Name:    "Editor",
				Success: false,
				Message: msg.err.Error(),
			}}
			m.showingResults = true
			m.resultsScrollOffset = 0
		}
		return m, m.dispatchLoadingSubEntryStates()

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.viewHeight = msg.Height - 10

		if m.viewHeight < 5 {
			m.viewHeight = 5
		}

		return m, nil

	case PackageInstallMsg:
		// Record result
		m.results = append(m.results, ResultItem{
			Name:    msg.Package.Name,
			Success: msg.Success,
			Message: msg.Message,
		})

		m.currentPackageIndex++

		// Check if there are more packages to install
		if m.currentPackageIndex < len(m.pendingPackages) {
			return m, m.installNextPackage()
		}

		completedPackages := m.pendingPackages

		// All done - return to List view
		m.processing = false
		m.pendingPackages = nil
		m.currentPackageIndex = 0
		m.Operation = OpList
		m.completedOperation = OpList
		m.Screen = ScreenResults
		m.rebuildTable()
		m.showingResults = true
		m.resultsScrollOffset = 0

		return m, m.refreshPackageStates(completedPackages)

	case OperationCompleteMsg:
		m.processing = false
		m.results = msg.Results
		m.err = msg.Err
		m.Screen = ScreenResults
		m.showingResults = true
		m.resultsScrollOffset = 0

		return m, nil

	case BatchOperationMsg:
		// Update batch progress state
		m.batchCurrentItem = msg.ItemName
		m.batchCurrentIndex = msg.ItemIndex
		m.batchTotalItems = msg.TotalItems

		if msg.Success {
			m.batchSuccessCount++
		} else {
			m.batchFailCount++
		}

		return m, nil

	case BatchCompleteMsg:
		// Batch operation complete - show results
		m.processing = false
		m.results = msg.Results
		m.completedOperation = m.Operation
		m.Screen = ScreenResults
		m.Operation = OpList

		m.rebuildTable()

		// Clear selections after operation
		m.clearSelections()
		m.clearTransientSummarySelection()
		m.showingResults = true
		m.resultsScrollOffset = 0

		return m, nil

	case initBatchInstallMsg:
		// Initialize batch package installation
		m.pendingPackages = msg.packages
		m.currentPackageIndex = 0

		// Start installing first package
		if len(m.pendingPackages) > 0 {
			return m, m.installNextPackage()
		}

		// No packages to install
		return m, func() tea.Msg {
			return BatchCompleteMsg{
				Results:      []ResultItem{},
				SuccessCount: 0,
				FailCount:    0,
			}
		}
	}

	return m, nil
}

// handleCommonKeys checks for keys that are common across all navigation screens.
// Returns (model, cmd, handled). If handled is true, the caller should return immediately.
// This should NOT be used in text-editing handlers where "q" is valid typed input;
// use handleTextEditKeys instead.
func (m Model) handleCommonKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, SharedKeys.ForceQuit):
		return m, tea.Quit, true
	case key.Matches(msg, SharedKeys.Quit):
		return m, tea.Quit, true
	}

	return m, nil, false
}

// handleTextEditKeys checks for keys common to all text-editing handlers.
// Handles ctrl+c (force quit).
func (m Model) handleTextEditKeys(msg tea.KeyPressMsg) (tea.Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, SharedKeys.ForceQuit):
		return m, tea.Quit, true
	}

	return m, nil, false
}

func (m Model) handleKeyPress(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	// Handle AddForm separately (needs text input handling)
	if m.Screen == ScreenAddForm {
		// Route to appropriate form handler based on activeForm
		switch m.activeForm {
		case FormApplication:
			return m.updateApplicationForm(msg)
		case FormSubEntry:
			return m.updateSubEntryForm(msg)
		case FormNone:
			fallthrough
		default:
			return m.updateAddForm(msg)
		}
	}
	if m.Screen == ScreenEntryTypeChooser {
		return m.updateEntryTypeChooser(msg)
	}

	switch {
	case key.Matches(msg, SharedKeys.ForceQuit):
		return m, tea.Quit

	case key.Matches(msg, SharedKeys.Quit):
		// Let the List view handle q for sub-screens
		if m.Screen == ScreenResults && m.Operation == OpList {
			return m.updateResults(msg)
		}

		// Quit the application
		return m, tea.Quit

	case key.Matches(msg, FormNavKeys.Cancel):
		// ESC is only for canceling operations, not navigation
		// Let screens that need it handle it (filter mode, delete confirmation, detail popup)
		if m.Screen == ScreenResults && m.Operation == OpList {
			return m.updateResults(msg)
		}
		if m.Screen == ScreenSummary {
			return m.updateSummary(msg)
		}
		// For other screens, ESC does nothing (use q to go back)
		return m, nil
	}

	switch m.Screen {
	case ScreenResults:
		return m.updateResults(msg)
	case ScreenProgress:
		// Progress screen doesn't handle key events
		return m, nil
	case ScreenAddForm:
		// AddForm is handled earlier, but adding case for exhaustiveness
		return m, nil
	case ScreenEntryTypeChooser:
		return m.updateEntryTypeChooser(msg)
	case ScreenSummary:
		return m.updateSummary(msg)
	}

	return m, nil
}

// hasLoadingItems returns true if any application or sub-entry is still loading.
func (m Model) hasLoadingItems() bool {
	for i := range m.Applications {
		app := &m.Applications[i]
		if app.IsFiltered {
			continue
		}
		if app.Application.HasPackage() && app.PkgInstalled == nil && app.PkgMethod != TypeNone {
			return true
		}
		for j := range app.SubItems {
			if app.SubItems[j].State == StateLoading {
				return true
			}
		}
	}
	return false
}

// View renders the current screen and returns the string to display.
// This is part of the Bubble Tea model interface.
func (m Model) View() tea.View {
	var content string

	switch m.Screen {
	case ScreenProgress:
		content = m.viewProgress()
	case ScreenResults:
		content = m.viewResults()
		if m.showingResults && m.Operation == OpList && len(m.results) > 0 {
			content = m.renderResultsPopup()
		}
	case ScreenAddForm:
		// Route to appropriate form view based on activeForm
		switch m.activeForm {
		case FormApplication:
			content = m.viewApplicationForm()
		case FormSubEntry:
			content = m.viewSubEntryForm()
		case FormNone:
			content = m.viewAddForm()
		default:
			content = m.viewAddForm()
		}
	case ScreenEntryTypeChooser:
		content = m.viewEntryTypeChooser()
	case ScreenSummary:
		content = m.viewSummary()
	}

	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

// OperationCompleteMsg is an alias for tuiops.OperationCompleteMsg so that
// all existing code in this package continues to compile without modification.
type OperationCompleteMsg = tuiops.OperationCompleteMsg

// PackageInstallMsg is sent after each individual package installation completes
type PackageInstallMsg struct {
	Err     error
	Message string
	Package PackageItem
	Success bool
}

// detectConfigState determines the state of a config entry given its paths and file list.
// This is the shared logic used by both detectPathState and detectSubEntryState.
// It delegates to detection.DetectConfigState which is the canonical implementation.
func detectConfigState(backupPath, targetPath string, isFolder bool, files []string, isCopy bool) PathState {
	return detection.DetectConfigState(backupPath, targetPath, isFolder, files, isCopy)
}

// handlePkgCheckResult processes the result of a single async package install check.
func (m Model) handlePkgCheckResult(msg pkgCheckResultMsg) (tea.Model, tea.Cmd) {
	if msg.appIndex < len(m.Applications) {
		m.Applications[msg.appIndex].PkgMethod = msg.method
		if msg.method != TypeNone {
			installed := msg.installed
			m.Applications[msg.appIndex].PkgInstalled = &installed
		}
	}
	m.decrementPendingAndRebuild()
	return m, nil
}

// handleStateCheckResult processes the result of a single async sub-entry state check.
func (m Model) handleStateCheckResult(msg stateCheckResultMsg) (tea.Model, tea.Cmd) {
	if msg.appIndex < len(m.Applications) && msg.subIndex < len(m.Applications[msg.appIndex].SubItems) {
		m.Applications[msg.appIndex].SubItems[msg.subIndex].State = msg.state
	}
	m.decrementPendingAndRebuild()
	return m, nil
}

// decrementPendingAndRebuild decrements the pending state check counter and
// rebuilds the table only when all pending checks have completed.  If the
// counter is already 0 (e.g. a manual refresh or ad-hoc check), the table is
// rebuilt immediately.
func (m *Model) decrementPendingAndRebuild() {
	if m.pendingStateChecks > 0 {
		m.pendingStateChecks--
	}
	if m.pendingStateChecks == 0 {
		m.initTableModel()
	}
}

// dispatchUncheckedPackageStates dispatches async package checks and tracks
// the pending count on the model.  Designed for use in value-receiver Update
// handlers where the modified Model is returned.
func (m *Model) dispatchUncheckedPackageStates() tea.Cmd {
	cmd, count := m.checkUncheckedPackageStatesCmd()
	m.pendingStateChecks += count
	return cmd
}

// dispatchFilteredStates dispatches async state checks for filtered (hidden)
// apps and tracks the pending count on the model.
func (m *Model) dispatchFilteredStates() tea.Cmd {
	cmd, count := m.checkFilteredStatesCmd()
	m.pendingStateChecks += count
	return cmd
}

// dispatchLoadingSubEntryStates dispatches async state checks for sub-entries
// left at StateLoading by a synchronous rebuild (e.g. setup entries after
// refreshApplicationStates or reinitPreservingState) and tracks the pending
// count on the model.
func (m *Model) dispatchLoadingSubEntryStates() tea.Cmd {
	cmd, count := m.checkLoadingSubEntryStatesCmd()
	m.pendingStateChecks += count
	return cmd
}

// handleMouseClickEvent processes mouse click events for the TUI.
func (m Model) handleMouseClickEvent(msg tea.MouseClickMsg) (tea.Model, tea.Cmd) {
	// Only handle mouse events on the list table screen
	if m.Screen != ScreenResults || m.Operation != OpList {
		return m, nil
	}

	// Don't handle mouse during modal states
	if m.searching || m.confirmingDeleteApp || m.confirmingDeleteSubEntry ||
		m.confirmingFilterToggle || m.confirmingActionFilter || m.showingDetail || m.showingResults {
		return m, nil
	}

	mouse := msg.Mouse()

	switch msg.Button {
	case tea.MouseLeft:
		return m.handleMouseClick(mouse.Y, false)
	case tea.MouseRight:
		return m.handleMouseClick(mouse.Y, true)
	}

	return m, nil

	// handleMouseWheelEvent processes mouse wheel events for the TUI.
}

func (m Model) handleMouseWheelEvent(msg tea.MouseWheelMsg) (tea.Model, tea.Cmd) {
	// Only handle mouse events on the list table screen
	if m.Screen != ScreenResults || m.Operation != OpList {
		return m, nil
	}

	// Don't handle mouse during modal states
	if m.searching || m.confirmingDeleteApp || m.confirmingDeleteSubEntry ||
		m.confirmingFilterToggle || m.confirmingActionFilter || m.showingDetail || m.showingResults {
		return m, nil
	}

	switch msg.Button {
	case tea.MouseWheelUp:
		if m.tableCursor > 0 {
			m.tableCursor -= 3
			if m.tableCursor < 0 {
				m.tableCursor = 0
			}
			m.updateScrollOffset()
		}
		return m, nil

	case tea.MouseWheelDown:
		if m.tableCursor < len(m.tableRows)-1 {
			m.tableCursor += 3
			if m.tableCursor >= len(m.tableRows) {
				m.tableCursor = len(m.tableRows) - 1
			}
			m.updateScrollOffset()
		}
		return m, nil
	}

	return m, nil
}

// handleMouseClick handles a mouse click on the table. Left click moves the
// cursor, right click toggles selection (like tab/space).
func (m Model) handleMouseClick(mouseY int, toggleSelect bool) (tea.Model, tea.Cmd) {
	// Layout offset from top of screen to first data row in the lipgloss table:
	// BaseStyle top padding (1) + filter banner (1) + table top border (1) +
	// header row (1) + header separator (1) = 5
	const tableDataStartY = 5

	lipglossRow := mouseY - tableDataStartY
	if lipglossRow < 0 {
		return m, nil
	}

	tableRowIdx := lipglossRow + m.scrollOffset
	if tableRowIdx < 0 || tableRowIdx >= len(m.tableRows) {
		return m, nil
	}

	// Detect scroll indicator rows to avoid selecting hidden rows
	maxVisibleRows := m.computeMaxVisibleRows()

	totalRows := len(m.tableRows)
	visibleEnd := m.scrollOffset + maxVisibleRows
	if visibleEnd > totalRows {
		visibleEnd = totalRows
	}

	hasMoreAbove := m.scrollOffset > 0
	hasMoreBelow := visibleEnd < totalRows

	if hasMoreAbove && lipglossRow == 0 {
		return m, nil
	}

	renderedRows := visibleEnd - m.scrollOffset
	if hasMoreBelow && lipglossRow >= renderedRows-1 {
		return m, nil
	}

	// Move cursor to clicked row
	m.tableCursor = tableRowIdx

	// Right click toggles selection (like tab/space)
	if toggleSelect {
		appIdx, subIdx := m.getApplicationAtCursorFromTable()
		if appIdx >= 0 {
			if subIdx >= 0 {
				m.toggleSubEntrySelection(appIdx, subIdx)
			} else {
				m.toggleAppSelection(appIdx)
			}
		}
	}

	return m, nil
}

// resolvePath resolves relative paths against BackupRoot and expands ~ in paths
func (m Model) resolvePath(path string) string {
	expandedPath := config.ExpandPath(path, m.Platform.EnvVars)

	if filepath.IsAbs(expandedPath) {
		return expandedPath
	}

	expandedBackupRoot := config.ExpandPath(m.Config.BackupRoot, m.Platform.EnvVars)
	return filepath.Join(expandedBackupRoot, expandedPath)
}
