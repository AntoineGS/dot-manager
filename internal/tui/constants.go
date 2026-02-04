package tui

// Key binding constants for TUI navigation and interaction
const (
	KeyEnter     = "enter"
	KeyEsc       = "esc"
	KeyCtrlC     = "ctrl+c"
	KeyCtrlS     = "ctrl+s"
	KeyDown      = "down"
	KeyTab       = "tab"
	KeyShiftTab  = "shift+tab"
	KeyBackspace = "backspace"
	KeyDelete    = "delete"
	KeyLeft      = "left"
)

// UI element constants
const (
	PlaceholderNeovim = "e.g., neovim"
	IndentSpaces      = "    "
	CheckboxUnchecked = "[ ]"
	CheckboxChecked   = "[✓]"
)

// Entry type constants
const (
	TypeGit    = "git"
	TypeFolder = "folder"
	TypeNone   = "none"
)

// Application status constants for level 1 rows
const (
	StatusInstalled = "Installed"
	StatusMissing   = "Missing"
	StatusFiltered  = "Filtered"
)

// Sort column constants
const (
	SortColumnName   = "name"
	SortColumnStatus = "status"
	SortColumnPath   = "path"
)
