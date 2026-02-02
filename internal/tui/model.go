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

type Screen int

const (
	ScreenMenu Screen = iota
	ScreenPathSelect
	ScreenPackageSelect
	ScreenConfirm
	ScreenProgress
	ScreenResults
	ScreenAddForm
)

type Operation int

const (
	OpRestore Operation = iota
	OpAdd
	OpList
	OpInstallPackages
)

func (o Operation) String() string {
	switch o {
	case OpRestore:
		return "Restore"
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

const (
	StateReady   PathState = iota // Backup exists, ready to restore
	StateAdopt                    // No backup but target exists (will adopt)
	StateMissing                  // Neither backup nor target exists
	StateLinked                   // Already symlinked
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
	FilterIndex int    // Which filter group this belongs to (0-based)
	IsExclude   bool   // true for exclude, false for include
	Key         string // os, distro, hostname, user
	Value       string // the pattern/value
}

// AddForm holds the state for the Add path form
type AddForm struct {
	// Entry type (config vs git)
	entryType EntryType

	// Common fields
	nameInput        textinput.Model
	descriptionInput textinput.Model
	isSudo           bool

	// Target fields (both types)
	linuxTargetInput   textinput.Model
	windowsTargetInput textinput.Model

	// Config-specific fields
	backupInput textinput.Model
	isFolder    bool

	// Git-specific fields
	repoInput   textinput.Model
	branchInput textinput.Model

	// Focus index: depends on entry type and mode
	// Config type: 0=name, 1=description, 2=linuxTarget, 3=windowsTarget, 4=backup, 5=isFolder toggle, 6=isSudo toggle, 7=files list (when !isFolder), 8=filters
	// Git type: 0=name, 1=description, 2=linuxTarget, 3=windowsTarget, 4=repo, 5=branch, 6=isSudo toggle, 7=filters
	// New entries add type toggle at position 0, shifting others by 1
	focusIndex int
	err        string
	editIndex  int // -1 for new, >= 0 for editing existing path

	// Field editing state
	editingField  bool   // Whether we're currently editing a text field
	originalValue string // Original value before editing (for cancel)

	// Files list state (when config type and !isFolder)
	files            []string
	filesCursor      int             // Cursor position in files list
	newFileInput     textinput.Model // Input for adding/editing files
	addingFile       bool            // Whether we're currently adding a file
	editingFile      bool            // Whether we're currently editing a file
	editingFileIndex int             // Index of the file being edited

	// Filters list state
	filters             []FilterCondition // Flattened list of filter conditions
	filtersCursor       int               // Cursor position in filters list
	addingFilter        bool              // Whether we're currently adding a filter
	editingFilter       bool              // Whether we're currently editing a filter
	editingFilterIndex  int               // Index of the filter being edited
	filterAddStep       int               // 0=type(include/exclude), 1=key, 2=value
	filterIsExclude     bool              // For adding: is this an exclude condition
	editingFilterValue  bool              // Whether we're actively editing the filter value text
	filterKeyInput      textinput.Model   // Input for filter key selection
	filterValueInput    textinput.Model   // Input for filter value
	filterKeyCursor     int               // Cursor for key selection (0-3: os, distro, hostname, user)

	// Autocomplete state
	suggestions      []string
	suggestionCursor int
	showSuggestions  bool

	// Package managers state
	packageManagers    map[string]string // Manager name -> package name
	packagesCursor     int               // Position in package managers list
	editingPackage     bool              // Whether we're editing a package name
	packageNameInput   textinput.Model   // Input for package name
	lastPackageName    string            // Last entered package name for auto-populate
}

type Model struct {
	Screen    Screen
	Operation Operation

	// Data
	Config     *config.Config
	ConfigPath string // Path to config file for saving
	Platform   *platform.Platform
	FilterCtx  *config.FilterContext // Filter context for entry filtering
	Manager    *manager.Manager
	Paths      []PathItem
	Packages   []PackageItem
	DryRun     bool


	// UI state
	menuCursor    int
	pathCursor    int
	packageCursor int
	scrollOffset  int
	viewHeight    int
	listCursor    int  // Cursor for list table view
	showingDetail bool // Whether detail popup is showing

	// Confirmation state for list view
	confirmingDelete bool // Whether we're confirming a delete

	// Filter state for list view
	filtering   bool              // Whether we're in filter mode
	filterInput textinput.Model   // Text input for filter
	filterText  string            // Current filter text (for highlighting)

	// Add form
	addForm AddForm

	// Results
	results    []ResultItem
	processing bool
	err        error

	// Package installation state (for sequential tea.Exec calls)
	pendingPackages     []PackageItem
	currentPackageIndex int

	// Window size
	width  int
	height int
}

// EntryType distinguishes between config, git, and package-only type entries
type EntryType int

const (
	EntryTypeConfig EntryType = iota
	EntryTypeGit
	EntryTypePackage // Package-only entry (no config or git)
)

type PathItem struct {
	Entry     config.Entry
	Target    string
	Selected  bool
	State     PathState
	EntryType EntryType
	// Package fields
	PkgMethod    string // How package would be installed (pacman, apt, custom, url, none)
	PkgInstalled *bool  // nil = no package, true = installed, false = not installed
}

type PackageItem struct {
	Entry    config.Entry
	Method   string // How it would be installed (pacman, apt, custom, url, none)
	Selected bool
}

type ResultItem struct {
	Name    string
	Success bool
	Message string
}

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

	// For v3 configs, flatten applications into PathItems
	if cfg.Version == 3 {
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
				}

				var entryType EntryType
				if subEntry.Type == "git" {
					entryType = EntryTypeGit
					entry.Repo = subEntry.Repo
					entry.Branch = subEntry.Branch
					entry.Targets = subEntry.Targets
				} else {
					entryType = EntryTypeConfig
					entry.Files = subEntry.Files
					entry.Backup = subEntry.Backup
					entry.Targets = subEntry.Targets
				}

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
					spec := entry.ToPackageSpec()
					method := getPackageInstallMethod(spec, plat.OS)
					item.PkgMethod = method
					if method != "none" {
						installed := isPackageInstalled(spec, method)
						item.PkgInstalled = &installed
					}
				}

				items = append(items, item)
				addedEntries[entry.Name] = true
			}
		}
	} else {
		// For v2 configs, use existing logic
		// Get all config entries filtered by filter context
		configEntries := cfg.GetFilteredConfigEntries(filterCtx)

		for _, e := range configEntries {
			target := e.GetTarget(plat.OS)
			item := PathItem{
				Entry:     e,
				Target:    target,
				Selected:  true, // Select all by default
				EntryType: EntryTypeConfig,
			}
			// Add package info if entry has a package
			if e.HasPackage() {
				spec := e.ToPackageSpec()
				method := getPackageInstallMethod(spec, plat.OS)
				item.PkgMethod = method
				if method != "none" {
					installed := isPackageInstalled(spec, method)
					item.PkgInstalled = &installed
				}
			}
			items = append(items, item)
			addedEntries[e.Name] = true
		}

		// Get all git entries filtered by filter context
		gitEntries := cfg.GetFilteredGitEntries(filterCtx)
		for _, e := range gitEntries {
			target := e.GetTarget(plat.OS)
			item := PathItem{
				Entry:     e,
				Target:    target,
				Selected:  true, // Select all by default
				EntryType: EntryTypeGit,
			}
			// Add package info if entry has a package
			if e.HasPackage() {
				spec := e.ToPackageSpec()
				method := getPackageInstallMethod(spec, plat.OS)
				item.PkgMethod = method
				if method != "none" {
					installed := isPackageInstalled(spec, method)
					item.PkgInstalled = &installed
				}
			}
			items = append(items, item)
			addedEntries[e.Name] = true
		}

		// Add package-only entries (those not already added as config or git entries)
		packageEntries := cfg.GetFilteredPackageEntries(filterCtx)
		for _, e := range packageEntries {
			// Skip if already added as config or git entry
			if addedEntries[e.Name] {
				continue
			}
			spec := e.ToPackageSpec()
			method := getPackageInstallMethod(spec, plat.OS)
			if method != "none" {
				installed := isPackageInstalled(spec, method)
				items = append(items, PathItem{
					Entry:        e,
					Target:       "", // Package-only entries have no target
					Selected:     true,
					EntryType:    EntryTypePackage,
					PkgMethod:    method,
					PkgInstalled: &installed,
				})
			}
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

// isPackageInstalled checks if a package is installed using the packages package
func isPackageInstalled(spec config.PackageSpec, method string) bool {
	// Get the package name for the detected manager
	pkgName := ""
	if name, ok := spec.Managers[method]; ok {
		pkgName = name
	} else {
		// For custom/url methods, use the entry name
		pkgName = spec.Name
	}

	return packages.IsInstalled(pkgName, method)
}

// getPackageInstallMethod determines how a package would be installed
func getPackageInstallMethod(pkg config.PackageSpec, osType string) string {
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
	return "none"
}

func detectAvailableManagers() []string {
	return platform.DetectAvailableManagers()
}

func (m Model) Init() tea.Cmd {
	return nil
}

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
		return m.updateAddForm(msg)
	}

	switch msg.String() {
	case "ctrl+c":
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

	case "esc":
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
	}

	return m, nil
}

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
		return m.viewAddForm()
	}
	return ""
}

// Messages
type OperationCompleteMsg struct {
	Results []ResultItem
	Err     error
}

// PackageInstallMsg is sent after each individual package installation completes
type PackageInstallMsg struct {
	Package PackageItem
	Success bool
	Message string
	Err     error
}

// detectPathState determines the state of a path item
func (m *Model) detectPathState(item *PathItem) PathState {
	targetPath := item.Target

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
