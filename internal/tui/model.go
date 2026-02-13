package tui

import (
	"os"
	"path/filepath"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/manager"
	"github.com/AntoineGS/tidydots/internal/platform"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
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
	// ScreenSummary is the summary/confirmation screen for batch operations
	ScreenSummary
)

// Operation represents the type of operation being performed in the TUI.
type Operation int

// TUI operation types.
const (
	// OpRestore is the restore operation
	OpRestore Operation = iota
	// OpList is the list entries operation
	OpList
	// OpInstallPackages is the install packages operation
	OpInstallPackages
	// OpDelete is the delete entries operation
	OpDelete
)

func (o Operation) String() string {
	switch o {
	case OpRestore:
		return "Restore"
	case OpList:
		return "List"
	case OpInstallPackages:
		return "Install Packages"
	case OpDelete:
		return "Delete"
	}

	return "Unknown"
}

// PathState represents the state of a path item for restore operations
type PathState int

// Path states for restore operations.
const (
	// StateLoading indicates state is still being detected
	StateLoading PathState = iota
	// StateReady indicates backup exists and is ready to restore
	StateReady // Backup exists, ready to restore
	// StateAdopt indicates no backup but target exists (will adopt)
	StateAdopt // No backup but target exists (will adopt)
	// StateMissing indicates neither backup nor target exists
	StateMissing // Neither backup nor target exists
	// StateLinked indicates already symlinked
	StateLinked // Already symlinked
	// StateOutdated indicates linked but template source changed since last render
	StateOutdated
	// StateModified indicates linked but rendered file has user edits
	StateModified
)

func (s PathState) String() string {
	switch s {
	case StateLoading:
		return "Loading..."
	case StateReady:
		return "Ready"
	case StateAdopt:
		return "Adopt"
	case StateMissing:
		return "Missing"
	case StateLinked:
		return "Linked"
	case StateOutdated:
		return "Outdated"
	case StateModified:
		return "Modified"
	}

	return "Unknown"
}

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

// ApplicationForm holds state for editing Application metadata
type ApplicationForm struct {
	packageManagers  map[string]string
	lastPackageName  string
	err              string
	originalValue    string
	descriptionInput textinput.Model
	packageNameInput textinput.Model
	nameInput        textinput.Model
	whenInput        textinput.Model
	editAppIdx       int
	packagesCursor   int
	focusIndex       int
	editingField     bool
	editingPackage   bool
	editingWhen      bool

	// Git package fields
	gitURLInput     textinput.Model
	gitBranchInput  textinput.Model
	gitLinuxInput   textinput.Model
	gitWindowsInput textinput.Model
	gitFieldCursor  int  // -1 = on git label/button, 0-4 = on sub-fields
	editingGitField bool // true when editing a git text field
	hasGitPackage   bool // true when git package is configured/expanded
	gitSudo         bool // sudo toggle for git package

	// Installer package fields
	installerLinuxInput   textinput.Model
	installerWindowsInput textinput.Model
	installerBinaryInput  textinput.Model
	installerFieldCursor  int  // -1 = on installer label/button, 0-2 = on sub-fields
	editingInstallerField bool // true when editing an installer text field
	hasInstallerPackage   bool // true when installer package is configured/expanded
}

// SubEntryForm holds state for editing SubEntry data
type SubEntryForm struct {
	err                string
	successMessage     string
	originalValue      string
	suggestions        []string
	files              []string
	selectedFiles      map[string]bool
	nameInput          textinput.Model
	linuxTargetInput   textinput.Model
	windowsTargetInput textinput.Model
	backupInput        textinput.Model
	newFileInput       textinput.Model
	filePicker         filepicker.Model
	editingFileIndex   int
	targetAppIdx       int
	editSubIdx         int
	editAppIdx         int
	focusIndex         int
	filesCursor        int
	suggestionCursor   int
	modeMenuCursor     int
	addFileMode        AddFileMode
	isFolder           bool
	showSuggestions    bool
	editingField       bool
	addingFile         bool
	editingFile        bool
	isSudo             bool
}

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
	searchText               string
	ConfigPath               string
	pendingPackages          []PackageItem
	results                  []ResultItem
	Applications             []ApplicationItem
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
	showingDetail            bool

	// Diff picker state
	showingDiffPicker bool
	diffPickerCursor  int
	diffPickerFiles   []manager.ModifiedTemplate

	// Filter state
	filterEnabled bool // true to hide filtered apps, false to show all

	// Selection state for multi-select mode
	selectedApps       map[int]bool    // appIndex -> selected
	selectedSubEntries map[string]bool // appIndex:subIndex -> selected
	multiSelectActive  bool            // true when selections exist

	// Summary screen state
	summaryOperation   Operation // Which batch operation: restore, install, delete
	summaryDoublePress string    // Track double-press state: "r", "i", or "d"

	// Batch operation progress state
	spinner           spinner.Model  // Loading spinner for async state detection
	batchProgress     progress.Model // Progress bar for batch operations
	batchCurrentItem  string         // Name of current item being processed
	batchTotalItems   int            // Total items in batch
	batchCurrentIndex int            // Current item index (0-based)
	batchSuccessCount int            // Count of successful operations
	batchFailCount    int            // Count of failed operations
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
	Selected     bool
	Expanded     bool
	IsFiltered   bool // True if this app doesn't match the current filter context
}

// SubEntryItem represents a sub-entry within an application (config or git)
type SubEntryItem struct {
	AppName  string
	Target   string
	SubEntry config.SubEntry
	State    PathState
	Selected bool
}

// ResultItem represents the result of an operation, including whether it
// succeeded and any associated message.
type ResultItem struct {
	Name    string
	Message string
	Success bool
}

// NewModel creates and initializes a new TUI model with the given configuration,
// platform information, and dry-run mode. It sets up the initial state including
// loading entries, detecting path states, and initializing the UI.
func NewModel(cfg *config.Config, plat *platform.Platform, dryRun bool) Model {
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
		Screen:             ScreenResults, // Start directly in Manage view
		Operation:          OpList,        // Set operation to List (Manage)
		Config:             cfg,
		Platform:           plat,
		Renderer:           renderer,
		DryRun:             dryRun,
		viewHeight:         15,
		width:              80,
		height:             24,
		searchInput:        searchInput,
		spinner:            s,
		sortColumn:         SortColumnName, // Default sort by name
		sortAscending:      true,           // Ascending by default
		filterEnabled:      true,           // Filter ON by default
		selectedApps:       make(map[int]bool),
		selectedSubEntries: make(map[string]bool),
		multiSelectActive:  false,
	}

	// Initialize applications for hierarchical view
	m.initApplicationItems()

	return m
}

// Init initializes the TUI model and returns any initial commands to run.
// This is part of the Bubble Tea model interface.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.spinner.Tick,
		m.checkPackageStatesCmd(),
		m.checkSubEntryStatesCmd(),
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

	case spinner.TickMsg:
		if m.hasLoadingItems() {
			var cmd tea.Cmd
			m.spinner, cmd = m.spinner.Update(msg)
			return m, cmd
		}
		return m, nil

	case tea.KeyMsg:
		return m.handleKeyPress(msg)

	case tea.MouseMsg:
		return m.handleMouseEvent(msg)

	case editorLaunchCompleteMsg:
		// Editor exited - refresh application states since template may have changed
		m.refreshApplicationStates()
		m.rebuildTable()
		if msg.err != nil {
			m.results = []ResultItem{{
				Name:    "Editor",
				Success: false,
				Message: msg.err.Error(),
			}}
		}
		return m, nil

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

		// Update installed status if installation succeeded
		if msg.Success {
			installed := true

			for i := range m.Applications {
				if m.Applications[i].Application.Name == msg.Package.Name && m.Applications[i].PkgInstalled != nil {
					m.Applications[i].PkgInstalled = &installed

					break
				}
			}
		}

		m.currentPackageIndex++

		// Check if there are more packages to install
		if m.currentPackageIndex < len(m.pendingPackages) {
			return m, m.installNextPackage()
		}

		// All done - return to List view
		m.processing = false
		m.pendingPackages = nil
		m.currentPackageIndex = 0
		m.Operation = OpList
		m.Screen = ScreenResults
		m.rebuildTable()

		return m, nil

	case OperationCompleteMsg:
		m.processing = false
		m.results = msg.Results
		m.err = msg.Err
		m.Screen = ScreenResults

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
		m.Screen = ScreenResults
		m.Operation = OpList

		// Update installed status for successful installs
		installed := true
		for _, result := range msg.Results {
			if result.Success {
				for i := range m.Applications {
					if m.Applications[i].Application.Name == result.Name && m.Applications[i].PkgInstalled != nil {
						m.Applications[i].PkgInstalled = &installed

						break
					}
				}
			}
		}

		m.rebuildTable()

		// Clear selections after operation
		m.clearSelections()

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
func (m Model) handleCommonKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
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
func (m Model) handleTextEditKeys(msg tea.KeyMsg) (tea.Model, tea.Cmd, bool) {
	switch {
	case key.Matches(msg, SharedKeys.ForceQuit):
		return m, tea.Quit, true
	}

	return m, nil, false
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
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
	case ScreenSummary:
		return m.updateSummary(msg)
	}

	return m, nil
}

// hasLoadingItems returns true if any application or sub-entry is still loading.
func (m Model) hasLoadingItems() bool {
	for i := range m.Applications {
		app := &m.Applications[i]
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
func (m Model) View() string {
	switch m.Screen {
	case ScreenProgress:
		return m.viewProgress()
	case ScreenResults:
		return m.viewResults()
	case ScreenAddForm:
		// Route to appropriate form view based on activeForm
		switch m.activeForm {
		case FormApplication:
			return m.viewApplicationForm()
		case FormSubEntry:
			return m.viewSubEntryForm()
		case FormNone:
			return m.viewAddForm()
		default:
			return m.viewAddForm()
		}
	case ScreenSummary:
		return m.viewSummary()
	}

	return ""
}

// OperationCompleteMsg is sent when an operation completes, containing any error
// and results from the operation.
type OperationCompleteMsg struct {
	Err     error
	Results []ResultItem
}

// PackageInstallMsg is sent after each individual package installation completes
type PackageInstallMsg struct {
	Err     error
	Message string
	Package PackageItem
	Success bool
}

// detectConfigState determines the state of a config entry given its paths and file list.
// This is the shared logic used by both detectPathState and detectSubEntryState.
func detectConfigState(backupPath, targetPath string, isFolder bool, files []string) PathState {
	if isFolder {
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
	checkedAnyFile := false

	for _, file := range files {
		srcFile := filepath.Join(backupPath, file)
		dstFile := filepath.Join(targetPath, file)

		if !pathExists(srcFile) {
			continue
		}

		checkedAnyFile = true
		anyBackup = true

		if info, err := os.Lstat(dstFile); err == nil {
			anyTarget = true
			if info.Mode()&os.ModeSymlink == 0 {
				allLinked = false
			}
		} else {
			allLinked = false
		}
	}

	if allLinked && checkedAnyFile {
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

// handlePkgCheckResult processes the result of a single async package install check.
func (m Model) handlePkgCheckResult(msg pkgCheckResultMsg) (tea.Model, tea.Cmd) {
	if msg.appIndex < len(m.Applications) {
		m.Applications[msg.appIndex].PkgMethod = msg.method
		if msg.method != TypeNone {
			installed := msg.installed
			m.Applications[msg.appIndex].PkgInstalled = &installed
		}
	}
	m.initTableModel()
	return m, nil
}

// handleStateCheckResult processes the result of a single async sub-entry state check.
func (m Model) handleStateCheckResult(msg stateCheckResultMsg) (tea.Model, tea.Cmd) {
	if msg.appIndex < len(m.Applications) && msg.subIndex < len(m.Applications[msg.appIndex].SubItems) {
		m.Applications[msg.appIndex].SubItems[msg.subIndex].State = msg.state
	}
	m.initTableModel()
	return m, nil
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// handleMouseEvent processes mouse events for the TUI.
func (m Model) handleMouseEvent(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	// Only handle mouse events on the list table screen
	if m.Screen != ScreenResults || m.Operation != OpList {
		return m, nil
	}

	// Don't handle mouse during modal states
	if m.searching || m.confirmingDeleteApp || m.confirmingDeleteSubEntry ||
		m.confirmingFilterToggle || m.showingDetail {
		return m, nil
	}

	switch {
	case msg.Button == tea.MouseButtonLeft && msg.Action == tea.MouseActionPress:
		return m.handleMouseClick(msg.Y, false)

	case msg.Button == tea.MouseButtonRight && msg.Action == tea.MouseActionPress:
		return m.handleMouseClick(msg.Y, true)

	case msg.Button == tea.MouseButtonWheelUp:
		if m.tableCursor > 0 {
			m.tableCursor -= 3
			if m.tableCursor < 0 {
				m.tableCursor = 0
			}
			m.results = nil
			m.updateScrollOffset()
		}
		return m, nil

	case msg.Button == tea.MouseButtonWheelDown:
		if m.tableCursor < len(m.tableRows)-1 {
			m.tableCursor += 3
			if m.tableCursor >= len(m.tableRows) {
				m.tableCursor = len(m.tableRows) - 1
			}
			m.results = nil
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
	maxVisibleRows := m.height - 12
	if maxVisibleRows < 3 {
		maxVisibleRows = 3
	}

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
	m.results = nil

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
