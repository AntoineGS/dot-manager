package tui

import (
	"os"
	"path/filepath"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/manager"
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

// AddForm holds the state for the Add path form
type AddForm struct {
	nameInput          textinput.Model
	linuxTargetInput   textinput.Model
	windowsTargetInput textinput.Model
	backupInput        textinput.Model
	isFolder           bool
	focusIndex         int // 0=name, 1=linuxTarget, 2=windowsTarget, 3=backup, 4=isFolder toggle
	err                string

	// Autocomplete state
	suggestions      []string
	suggestionCursor int
	showSuggestions  bool
}

type Model struct {
	Screen    Screen
	Operation Operation

	// Data
	Config     *config.Config
	ConfigPath string // Path to config file for saving
	Platform   *platform.Platform
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

	// Add form
	addForm AddForm

	// Results
	results    []ResultItem
	processing bool
	err        error

	// Window size
	width  int
	height int
}

type PathItem struct {
	Spec     config.PathSpec
	Target   string
	Selected bool
	State    PathState
}

type PackageItem struct {
	Spec     config.PackageSpec
	Method   string // How it would be installed (pacman, apt, custom, url, none)
	Selected bool
}

type ResultItem struct {
	Name    string
	Success bool
	Message string
}

func NewModel(cfg *config.Config, plat *platform.Platform, dryRun bool) Model {
	paths := cfg.Paths
	if plat.IsRoot {
		paths = cfg.RootPaths
	}

	items := make([]PathItem, 0, len(paths))
	for _, p := range paths {
		target := p.GetTarget(plat.OS)
		if target != "" {
			items = append(items, PathItem{
				Spec:     p,
				Target:   target,
				Selected: true, // Select all by default
			})
		}
	}

	// Initialize packages
	pkgItems := make([]PackageItem, 0, len(cfg.Packages.Items))
	for _, pkg := range cfg.Packages.Items {
		method := getPackageInstallMethod(pkg, plat.OS)
		if method != "none" {
			pkgItems = append(pkgItems, PackageItem{
				Spec:     pkg,
				Method:   method,
				Selected: true, // Select all by default
			})
		}
	}

	m := Model{
		Screen:     ScreenMenu,
		Config:     cfg,
		Platform:   plat,
		Paths:      items,
		Packages:   pkgItems,
		DryRun:     dryRun,
		viewHeight: 15,
		width:      80,
		height:     24,
	}

	// Detect initial path states
	m.refreshPathStates()

	return m
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
		// Let the Results screen handle ESC when in List view
		if m.Screen == ScreenResults && m.Operation == OpList {
			return m.updateResults(msg)
		}
		if m.Screen != ScreenMenu && !m.processing {
			m.Screen = ScreenMenu
		}
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

// detectPathState determines the state of a path item
func (m *Model) detectPathState(item *PathItem) PathState {
	backupPath := m.resolvePath(item.Spec.Backup)
	targetPath := item.Target

	// For folder-based paths
	if item.Spec.IsFolder() {
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

	for _, file := range item.Spec.Files {
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

	if allLinked && len(item.Spec.Files) > 0 {
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
