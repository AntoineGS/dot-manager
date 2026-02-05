package tui

import (
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"github.com/sebdah/goldie/v2"
)

const (
	nvimAppName = "nvim"
)

// TestScreenResults_Snapshots tests the visual output of ScreenResults
// in various states using golden file snapshots.
func TestScreenResults_Snapshots(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(*Model)
	}{
		{"basic_list", setupBasicList},
		{"app_expanded", setupAppExpanded},
		{"multi_select", setupMultiSelect},
		{"search_active", setupSearchActive},
		{"scroll_middle", setupScrollMiddle},
		{"scroll_bottom", setupScrollBottom},
		{"scroll_with_expanded", setupScrollWithExpanded},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Force ASCII color profile for consistent rendering
			lipgloss.SetColorProfile(termenv.Ascii)

			// Create model with test setup
			m := createTestModel()
			tt.setupFunc(m)

			// Render the view
			output := m.View()

			// Strip ANSI codes and normalize
			plainText := stripAnsiCodes(output)
			normalized := normalizeOutput(plainText)

			// Compare with golden file
			g := goldie.New(t)
			g.Assert(t, tt.name, []byte(normalized))
		})
	}
}

// createTestModel creates a basic model for testing.
// Individual setup functions will customize it.
func createTestModel() *Model {
	cfg := &config.Config{
		Version:    3,
		BackupRoot: "/home/user/backup",
	}
	plat := &platform.Platform{OS: platform.OSLinux}
	m := NewModel(cfg, plat, false)
	return &m
}

// setupMultiSelect creates a view with multi-selection active.
// Tests: selection indicators, banner text, multi-select help.
func setupMultiSelect(m *Model) {
	m.width = 100
	m.height = 30

	m.Config.Applications = []config.Application{
		{
			Name:        "bash",
			Description: "Bash shell",
			Entries: []config.SubEntry{
				{Name: "bashrc", Backup: "./bash", Targets: map[string]string{"linux": "~/.bashrc"}},
			},
		},
		{
			Name:        "git",
			Description: "Git version control",
			Entries: []config.SubEntry{
				{Name: "gitconfig", Backup: "./git", Targets: map[string]string{"linux": "~/.gitconfig"}},
			},
		},
		{
			Name:        nvimAppName,
			Description: "Neovim text editor",
			Entries: []config.SubEntry{
				{Name: "init", Backup: "./nvim/init", Targets: map[string]string{"linux": "~/.config/nvim/init.lua"}},
				{Name: "plugins", Backup: "./nvim/plugins", Targets: map[string]string{"linux": "~/.config/nvim/lua/plugins"}},
			},
		},
		{
			Name:        "zsh",
			Description: "Z shell",
			Entries: []config.SubEntry{
				{Name: "zshrc", Backup: "./zsh", Targets: map[string]string{"linux": "~/.zshrc"}},
			},
		},
	}

	m.initApplicationItems()

	// Select bash (app index 0) and nvim (app index 2)
	m.selectedApps[0] = true
	m.selectedApps[2] = true

	// Select a sub-entry from nvim
	m.selectedSubEntries["nvim/init"] = true

	m.multiSelectActive = true
	m.Screen = ScreenResults
	m.Operation = OpList
	m.appCursor = 0
}

// setupSearchActive creates a view with active search filtering.
// Tests: search filtering, matching behavior.
func setupSearchActive(m *Model) {
	m.width = 100
	m.height = 30

	m.Config.Applications = []config.Application{
		{
			Name:        "alacritty",
			Description: "GPU-accelerated terminal",
			Entries: []config.SubEntry{
				{Name: "alacritty.yml", Backup: "./alacritty", Targets: map[string]string{"linux": "~/.config/alacritty/alacritty.yml"}},
			},
		},
		{
			Name:        "bash",
			Description: "Bash shell",
			Entries: []config.SubEntry{
				{Name: "bashrc", Backup: "./bash", Targets: map[string]string{"linux": "~/.bashrc"}},
			},
		},
		{
			Name:        "git",
			Description: "Git version control",
			Entries: []config.SubEntry{
				{Name: "gitconfig", Backup: "./git", Targets: map[string]string{"linux": "~/.gitconfig"}},
			},
		},
		{
			Name:        nvimAppName,
			Description: "Neovim text editor",
			Entries: []config.SubEntry{
				{Name: "init", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim"}},
			},
		},
		{
			Name:        "tmux",
			Description: "Terminal multiplexer",
			Entries: []config.SubEntry{
				{Name: "tmux.conf", Backup: "./tmux", Targets: map[string]string{"linux": "~/.tmux.conf"}},
			},
		},
		{
			Name:        "vim",
			Description: "Vi improved",
			Entries: []config.SubEntry{
				{Name: "vimrc", Backup: "./vim", Targets: map[string]string{"linux": "~/.vimrc"}},
			},
		},
		{
			Name:        "vscode",
			Description: "Visual Studio Code",
			Entries: []config.SubEntry{
				{Name: "settings", Backup: "./vscode", Targets: map[string]string{"linux": "~/.config/Code/User/settings.json"}},
			},
		},
		{
			Name:        "zsh",
			Description: "Z shell",
			Entries: []config.SubEntry{
				{Name: "zshrc", Backup: "./zsh", Targets: map[string]string{"linux": "~/.zshrc"}},
			},
		},
	}

	m.initApplicationItems()

	// Search for "vim" - should match nvim, vim, vscode
	m.searchText = "vim"

	m.Screen = ScreenResults
	m.Operation = OpList
	m.appCursor = 0
}

func setupScrollMiddle(_ *Model) {
	// TODO: Implement in Task 6
}

func setupScrollBottom(_ *Model) {
	// TODO: Implement in Task 6
}

func setupScrollWithExpanded(_ *Model) {
	// TODO: Implement in Task 6
}

// setupBasicList creates a basic list view with 5 collapsed applications.
// Tests: basic rendering, spacing, help text.
func setupBasicList(m *Model) {
	// Set fixed dimensions for consistent rendering
	m.width = 100
	m.height = 30

	// Create 5 simple applications
	m.Config.Applications = []config.Application{
		{
			Name:        "bash",
			Description: "Bash shell",
			Entries: []config.SubEntry{
				{Name: "bashrc", Backup: "./bash", Targets: map[string]string{"linux": "~/.bashrc"}},
			},
		},
		{
			Name:        "git",
			Description: "Git version control",
			Entries: []config.SubEntry{
				{Name: "gitconfig", Backup: "./git", Targets: map[string]string{"linux": "~/.gitconfig"}},
			},
		},
		{
			Name:        nvimAppName,
			Description: "Neovim text editor",
			Entries: []config.SubEntry{
				{Name: "init", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim"}},
			},
		},
		{
			Name:        "tmux",
			Description: "Terminal multiplexer",
			Entries: []config.SubEntry{
				{Name: "tmux.conf", Backup: "./tmux", Targets: map[string]string{"linux": "~/.tmux.conf"}},
			},
		},
		{
			Name:        "zsh",
			Description: "Z shell",
			Entries: []config.SubEntry{
				{Name: "zshrc", Backup: "./zsh", Targets: map[string]string{"linux": "~/.zshrc"}},
			},
		},
	}

	// Initialize the model
	m.initApplicationItems()
	m.Screen = ScreenResults
	m.Operation = OpList
	m.appCursor = 0
}

// setupAppExpanded creates a view with expanded application showing sub-entries.
// Tests: indentation, sub-entry rendering, expansion indicator.
func setupAppExpanded(m *Model) {
	m.width = 100
	m.height = 30

	// Create 3 apps, middle one has multiple sub-entries
	m.Config.Applications = []config.Application{
		{
			Name:        "bash",
			Description: "Bash shell",
			Entries: []config.SubEntry{
				{Name: "bashrc", Backup: "./bash", Targets: map[string]string{"linux": "~/.bashrc"}},
			},
		},
		{
			Name:        nvimAppName,
			Description: "Neovim text editor",
			Entries: []config.SubEntry{
				{Name: "init.lua", Backup: "./nvim/init", Targets: map[string]string{"linux": "~/.config/nvim/init.lua"}},
				{Name: "plugins", Backup: "./nvim/plugins", Targets: map[string]string{"linux": "~/.config/nvim/lua/plugins"}},
				{Name: "mappings", Backup: "./nvim/mappings", Targets: map[string]string{"linux": "~/.config/nvim/lua/mappings.lua"}},
				{Name: "settings", Backup: "./nvim/settings", Targets: map[string]string{"linux": "~/.config/nvim/lua/settings.lua"}},
			},
		},
		{
			Name:        "zsh",
			Description: "Z shell",
			Entries: []config.SubEntry{
				{Name: "zshrc", Backup: "./zsh", Targets: map[string]string{"linux": "~/.zshrc"}},
			},
		},
	}

	m.initApplicationItems()

	// Expand the middle app (nvim, index 1 after sorting)
	for i, app := range m.Applications {
		if app.Application.Name == nvimAppName {
			m.Applications[i].Expanded = true
			break
		}
	}

	m.Screen = ScreenResults
	m.Operation = OpList
	m.appCursor = 1 // Cursor on nvim
}
