package tui

import (
	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/manager"
	"github.com/AntoineGS/dot-manager/internal/platform"
	tea "github.com/charmbracelet/bubbletea"
)

type Screen int

const (
	ScreenMenu Screen = iota
	ScreenPathSelect
	ScreenConfirm
	ScreenProgress
	ScreenResults
)

type Operation int

const (
	OpRestore Operation = iota
	OpBackup
	OpList
)

func (o Operation) String() string {
	switch o {
	case OpRestore:
		return "Restore"
	case OpBackup:
		return "Backup"
	case OpList:
		return "List"
	}
	return "Unknown"
}

type Model struct {
	Screen    Screen
	Operation Operation

	// Data
	Config   *config.Config
	Platform *platform.Platform
	Manager  *manager.Manager
	Paths    []PathItem
	DryRun   bool

	// UI state
	menuCursor   int
	pathCursor   int
	scrollOffset int
	viewHeight   int

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

	return Model{
		Screen:     ScreenMenu,
		Config:     cfg,
		Platform:   plat,
		Paths:      items,
		DryRun:     dryRun,
		viewHeight: 15,
		width:      80,
		height:     24,
	}
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

	case ProcessPathMsg:
		return m, m.processNextPath(msg.Index)
	}

	return m, nil
}

func (m Model) handleKeyPress(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "ctrl+c", "q":
		if m.Screen == ScreenResults || m.Screen == ScreenMenu {
			return m, tea.Quit
		}
		// Go back to menu
		m.Screen = ScreenMenu
		return m, nil

	case "esc":
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
	case ScreenConfirm:
		return m.viewConfirm()
	case ScreenProgress:
		return m.viewProgress()
	case ScreenResults:
		return m.viewResults()
	}
	return ""
}

// Messages
type OperationCompleteMsg struct {
	Results []ResultItem
	Err     error
}

type ProcessPathMsg struct {
	Index int
}
