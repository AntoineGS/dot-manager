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
	KeyRight     = "right"
)

// UI element constants
const (
	PlaceholderWhen             = `e.g., {{ eq .OS "linux" }}`
	PlaceholderNeovim           = "e.g., neovim"
	PlaceholderGitURL           = "e.g., https://github.com/user/repo.git"
	PlaceholderGitBranch        = "e.g., main"
	PlaceholderGitLinux         = "e.g., ~/.local/share/app"
	PlaceholderGitWindows       = "e.g., ~/AppData/Local/app"
	PlaceholderInstallerLinux   = "e.g., curl ... | sh"
	PlaceholderInstallerWindows = "e.g., winget install ..."
	PlaceholderInstallerBinary  = "e.g., cargo"
	IndentSpaces                = "    "
	CheckboxUnchecked           = "[ ]"
	CheckboxChecked             = "[✓]"
)

// Entry type constants
const (
	TypeGit       = "git"
	TypeInstaller = "installer"
	TypeFolder    = "folder"
	TypeNone      = "none"
)

// Application status constants for level 1 rows
const (
	StatusInstalled = "Installed"
	StatusMissing   = "Missing"
	StatusFiltered  = "Filtered"
	StatusOutdated  = "Outdated"
	StatusUnknown   = "Unknown"
)

// Sort column constants
const (
	SortColumnName   = "name"
	SortColumnStatus = "status"
	SortColumnPath   = "path"
)

// Scrolling behavior constants
const (
	// ScrollOffsetMargin is the minimum number of rows to keep between cursor and viewport edges
	// Similar to vim's 'scrolloff' setting - provides smooth scrolling with buffer zone
	ScrollOffsetMargin = 3
)

// OS constants
const (
	OSLinux   = "linux"
	OSWindows = "windows"
)

// Git package field indices within the git sub-section
const (
	GitFieldURL     = 0
	GitFieldBranch  = 1
	GitFieldLinux   = 2
	GitFieldWindows = 3
	GitFieldSudo    = 4
	GitFieldCount   = 5
)

// Installer package field indices within the installer sub-section
const (
	InstallerFieldLinux   = 0
	InstallerFieldWindows = 1
	InstallerFieldBinary  = 2
	InstallerFieldCount   = 3
)
