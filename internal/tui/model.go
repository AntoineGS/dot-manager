package tui

import (
	"os"
	"path/filepath"
	"sort"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/manager"
	"github.com/AntoineGS/dot-manager/internal/packages"
	"github.com/AntoineGS/dot-manager/internal/platform"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// Screen represents the current screen being displayed in the TUI.
type Screen int

// TUI screen types.
const (
	// ScreenMenu is the main menu screen
	ScreenMenu Screen = iota
	// ScreenPathSelect is the path selection screen
	ScreenPathSelect
	// ScreenPackageSelect is the package selection screen
	ScreenPackageSelect
	// ScreenConfirm is the confirmation screen
	ScreenConfirm
	// ScreenProgress is the progress display screen
	ScreenProgress
	// ScreenResults is the results display screen
	ScreenResults
	// ScreenAddForm is the add/edit form screen
	ScreenAddForm
)

// Operation represents the type of operation being performed in the TUI.
type Operation int

// TUI operation types.
const (
	// OpRestore is the restore operation
	OpRestore Operation = iota
	// OpRestoreDryRun is the restore dry-run operation
	OpRestoreDryRun
	// OpAdd is the add entry operation
	OpAdd
	// OpList is the list entries operation
	OpList
	// OpInstallPackages is the install packages operation
	OpInstallPackages
)

func (o Operation) String() string {
	switch o {
	case OpRestore:
		return "Restore"
	case OpRestoreDryRun:
		return "Restore (Dry Run)"
	case OpAdd:
		return "Add"
	case OpList:
		return "List"
	case OpInstallPackages:
		return "Install Packages"
	}

	return "Unknown"
}

// PathState represents the state of a path item for restore operations
type PathState int

// Path states for restore operations.
const (
	// StateReady indicates backup exists and is ready to restore
	StateReady PathState = iota // Backup exists, ready to restore
	// StateAdopt indicates no backup but target exists (will adopt)
	StateAdopt // No backup but target exists (will adopt)
	// StateMissing indicates neither backup nor target exists
	StateMissing // Neither backup nor target exists
	// StateLinked indicates already symlinked
	StateLinked // Already symlinked
)

func (s PathState) String() string {
	switch s {
	case StateReady:
		return "Ready"
	case StateAdopt:
		return "Adopt"
	case StateMissing:
		return "Missing"
	case StateLinked:
		return "Linked"
	}

	return "Unknown"
}

// FilterCondition represents a single filter condition for the UI
type FilterCondition struct {
	Key         string
	Value       string
	FilterIndex int
	IsExclude   bool
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
	packageManagers    map[string]string
	lastPackageName    string
	err                string
	originalValue      string
	filters            []FilterCondition
	filterValueInput   textinput.Model
	descriptionInput   textinput.Model
	packageNameInput   textinput.Model
	nameInput          textinput.Model
	filterKeyCursor    int
	filtersCursor      int
	editAppIdx         int
	packagesCursor     int
	editingFilterIndex int
	filterAddStep      int
	focusIndex         int
	editingFilterValue bool
	filterIsExclude    bool
	editingField       bool
	editingFilter      bool
	addingFilter       bool
	editingPackage     bool
}

// SubEntryForm holds state for editing SubEntry data
type SubEntryForm struct {
	err                string
	originalValue      string
	suggestions        []string
	files              []string
	nameInput          textinput.Model
	linuxTargetInput   textinput.Model
	windowsTargetInput textinput.Model
	backupInput        textinput.Model
	newFileInput       textinput.Model
	editingFileIndex   int
	targetAppIdx       int
	editSubIdx         int
	editAppIdx         int
	focusIndex         int
	filesCursor        int
	suggestionCursor   int
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
	FilterCtx                *config.FilterContext
	Manager                  *manager.Manager
	subEntryForm             *SubEntryForm
	applicationForm          *ApplicationForm
	filterText               string
	ConfigPath               string
	Packages                 []PackageItem
	pendingPackages          []PackageItem
	results                  []ResultItem
	Paths                    []PathItem
	Applications             []ApplicationItem
	filterInput              textinput.Model
	viewHeight               int
	pathCursor               int
	height                   int
	width                    int
	currentPackageIndex      int
	menuCursor               int
	Operation                Operation
	scrollOffset             int
	appCursor                int
	Screen                   Screen
	packageCursor            int
	activeForm               FormType
	DryRun                   bool
	processing               bool
	filtering                bool
	confirmingDeleteSubEntry bool
	confirmingDeleteApp      bool
	showingDetail            bool
}

// EntryType distinguishes between config, git, and package-only type entries
type EntryType int

// Entry types for configuration entries.
const (
	// EntryTypeConfig indicates a config type entry (symlink management)
	EntryTypeConfig EntryType = iota
	// EntryTypeGit indicates a git type entry (repository clone)
	EntryTypeGit
	// EntryTypePackage indicates a package-only entry (no config or git)
	EntryTypePackage // Package-only entry (no config or git)
)

// PathItem represents a configuration entry in the path selection list,
// including its state, target path, and package information.
//
//nolint:govet // field order optimized for readability over memory layout
type PathItem struct {
	Entry        config.Entry
	PkgInstalled *bool
	Target       string
	PkgMethod    string
	State        PathState
	EntryType    EntryType
	Selected     bool
}

// PackageItem represents a package to be installed, including its entry
// configuration, installation method, and selection state.
type PackageItem struct {
	Method   string // How it would be installed (pacman, apt, custom, url, none)
	Entry    config.Entry
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
	// Create filter context from platform
	filterCtx := &config.FilterContext{
		OS:       plat.OS,
		Distro:   plat.Distro,
		Hostname: plat.Hostname,
		User:     plat.User,
	}

	// Track entries we've already added (by name) to avoid duplicates
	addedEntries := make(map[string]bool)

	items := make([]PathItem, 0)

	// Flatten applications into PathItems
	apps := cfg.GetFilteredApplications(filterCtx)
	for _, app := range apps {
		// Convert each SubEntry to a PathItem
		for _, subEntry := range app.Entries {
			// Create Entry from SubEntry
			entry := config.Entry{
				Name:        app.Name + "/" + subEntry.Name, // Prefix with app name
				Description: app.Description,                // Use app description
				Sudo:        subEntry.Sudo,
				Filters:     app.Filters, // Use app filters
				Files:       subEntry.Files,
				Backup:      subEntry.Backup,
				Targets:     subEntry.Targets,
			}

			entryType := EntryTypeConfig

			// Add package from app-level if present
			if app.Package != nil {
				entry.Package = app.Package
			}

			target := entry.GetTarget(plat.OS)
			item := PathItem{
				Entry:     entry,
				Target:    target,
				Selected:  true,
				EntryType: entryType,
			}

			// Add package info if entry has a package
			if entry.HasPackage() {
				method := getPackageInstallMethodFromPackage(entry.Package, plat.OS)
				item.PkgMethod = method

				if method != TypeNone {
					installed := isPackageInstalledFromPackage(entry.Package, method, entry.Name)
					item.PkgInstalled = &installed
				}
			}

			items = append(items, item)
			addedEntries[entry.Name] = true
		}
	}

	// Sort all items by name
	sort.Slice(items, func(i, j int) bool {
		return items[i].Entry.Name < items[j].Entry.Name
	})

	// Keep Packages slice for backward compatibility with install operations
	// Build from PathItems that have packages
	pkgItems := make([]PackageItem, 0)

	for _, item := range items {
		if item.PkgInstalled != nil {
			pkgItems = append(pkgItems, PackageItem{
				Entry:    item.Entry,
				Method:   item.PkgMethod,
				Selected: true, // Select all by default
			})
		}
	}

	// Initialize filter input
	filterInput := textinput.New()
	filterInput.Placeholder = "type to filter..."
	filterInput.CharLimit = 100

	m := Model{
		Screen:      ScreenMenu,
		Config:      cfg,
		Platform:    plat,
		FilterCtx:   filterCtx,
		Paths:       items,
		Packages:    pkgItems,
		DryRun:      dryRun,
		viewHeight:  15,
		width:       80,
		height:      24,
		filterInput: filterInput,
	}

	// Detect initial path states
	m.refreshPathStates()

	return m
}

// isPackageInstalledFromPackage checks if a package is installed using the packages package
func isPackageInstalledFromPackage(pkg *config.EntryPackage, method, entryName string) bool {
	if pkg == nil {
		return false
	}

	// Get the package name for the detected manager
	pkgName := ""
	if name, ok := pkg.Managers[method]; ok {
		pkgName = name
	} else {
		// For custom/url methods, use the entry name
		pkgName = entryName
	}

	return packages.IsInstalled(pkgName, method)
}

// getPackageInstallMethodFromPackage determines how a package would be installed
func getPackageInstallMethodFromPackage(pkg *config.EntryPackage, osType string) string {
	if pkg == nil {
		return TypeNone
	}

	// Check package managers
	availableManagers := detectAvailableManagers()
	for _, mgr := range availableManagers {
		if _, ok := pkg.Managers[mgr]; ok {
			return mgr
		}
	}
	// Check custom
	if _, ok := pkg.Custom[osType]; ok {
		return "custom"
	}
	// Check URL
	if _, ok := pkg.URL[osType]; ok {
		return "url"
	}

	return TypeNone
}

func detectAvailableManagers() []string {
	return platform.DetectAvailableManagers()
}

// Init initializes the TUI model and returns any initial commands to run.
// This is part of the Bubble Tea model interface.
func (m Model) Init() tea.Cmd {
	return nil
}

// Update processes messages and updates the model state accordingly.
// This is part of the Bubble Tea model interface.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		return m.handleKeyPress(msg)

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
			Name:    msg.Package.Entry.Name,
			Success: msg.Success,
			Message: msg.Message,
		})

		// Update installed status in Paths if installation succeeded
		if msg.Success {
			for i := range m.Paths {
				if m.Paths[i].Entry.Name == msg.Package.Entry.Name && m.Paths[i].PkgInstalled != nil {
					installed := true
					m.Paths[i].PkgInstalled = &installed

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

		return m, nil

	case OperationCompleteMsg:
		m.processing = false
		m.results = msg.Results
		m.err = msg.Err
		m.Screen = ScreenResults

		return m, nil
	}

	return m, nil
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

	switch msg.String() {
	case KeyCtrlC:
		return m, tea.Quit

	case "q":
		// Let the List view handle q (goes back to menu, not quit)
		if m.Screen == ScreenResults && m.Operation == OpList {
			return m.updateResults(msg)
		}

		if m.Screen == ScreenResults || m.Screen == ScreenMenu {
			return m, tea.Quit
		}
		// Go back to menu
		m.Screen = ScreenMenu

		return m, nil

	case KeyEsc:
		// ESC is only for canceling operations, not navigation
		// Let screens that need it handle it (filter mode, delete confirmation, detail popup)
		if m.Screen == ScreenResults && m.Operation == OpList {
			return m.updateResults(msg)
		}
		// For other screens, ESC does nothing (use q to go back)
		return m, nil
	}

	switch m.Screen {
	case ScreenMenu:
		return m.updateMenu(msg)
	case ScreenPathSelect:
		return m.updatePathSelect(msg)
	case ScreenPackageSelect:
		return m.updatePackageSelect(msg)
	case ScreenConfirm:
		return m.updateConfirm(msg)
	case ScreenResults:
		return m.updateResults(msg)
	case ScreenProgress:
		// Progress screen doesn't handle key events
		return m, nil
	case ScreenAddForm:
		// AddForm is handled earlier, but adding case for exhaustiveness
		return m, nil
	}

	return m, nil
}

// View renders the current screen and returns the string to display.
// This is part of the Bubble Tea model interface.
func (m Model) View() string {
	switch m.Screen {
	case ScreenMenu:
		return m.viewMenu()
	case ScreenPathSelect:
		return m.viewPathSelect()
	case ScreenPackageSelect:
		return m.viewPackageSelect()
	case ScreenConfirm:
		return m.viewConfirm()
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

// detectPathState determines the state of a path item
func (m *Model) detectPathState(item *PathItem) PathState {
	// Expand ~ in target path for file operations
	targetPath := config.ExpandPath(item.Target, m.Platform.EnvVars)

	// For git entries
	if item.EntryType == EntryTypeGit {
		if pathExists(targetPath) {
			gitDir := filepath.Join(targetPath, ".git")
			if pathExists(gitDir) {
				return StateLinked // Already cloned
			}

			return StateAdopt // Target exists but not a git repo
		}

		return StateReady // Ready to clone
	}

	// For config entries (symlinks)
	backupPath := m.resolvePath(item.Entry.Backup)

	// For folder-based paths
	if item.Entry.IsFolder() {
		// Check if target is already a symlink
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

	// For file-based paths, check if all files are ready
	allLinked := true
	anyBackup := false
	anyTarget := false

	for _, file := range item.Entry.Files {
		srcFile := filepath.Join(backupPath, file)
		dstFile := filepath.Join(targetPath, file)

		// Check if already a symlink
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

	if allLinked && len(item.Entry.Files) > 0 {
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

// refreshPathStates updates the state of all path items
func (m *Model) refreshPathStates() {
	for i := range m.Paths {
		m.Paths[i].State = m.detectPathState(&m.Paths[i])
	}
}

func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
