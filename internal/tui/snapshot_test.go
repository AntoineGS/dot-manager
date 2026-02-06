package tui

import (
	"fmt"
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
		{"expand_when_at_capacity", setupExpandWhenAtCapacity},
		{"multi_select", setupMultiSelect},
		{"search_active", setupSearchActive},
		{"scroll_middle", setupScrollMiddle},
		{"scroll_bottom", setupScrollBottom},
		{"scroll_bottom_then_up", setupScrollBottomThenUp},
		{"scroll_bottom_then_top", setupScrollBottomThenTop},
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
	m.tableCursor = 0
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
	m.tableCursor = 0
}

// setupScrollMiddle creates a long list with cursor in the middle.
// Tests: viewport windowing, scroll indicators, cursor positioning.
func setupScrollMiddle(m *Model) {
	m.width = 100
	m.height = 30

	// Create 40 apps to force scrolling
	apps := make([]config.Application, 40)
	for i := 0; i < 40; i++ {
		apps[i] = config.Application{
			Name:        fmt.Sprintf("app-%02d", i+1),
			Description: fmt.Sprintf("Application %d", i+1),
			Entries: []config.SubEntry{
				{
					Name:    "config",
					Backup:  fmt.Sprintf("./app-%02d", i+1),
					Targets: map[string]string{"linux": fmt.Sprintf("~/.config/app-%02d", i+1)},
				},
			},
		}
	}
	m.Config.Applications = apps

	m.initApplicationItems()
	m.Screen = ScreenResults
	m.Operation = OpList

	// Position cursor at row 20 (middle) - renderTable() will center it in viewport
	m.tableCursor = 20
}

// setupScrollBottom creates a long list with cursor at the bottom.
// Tests: bottom boundary, "end of list" rendering.
func setupScrollBottom(m *Model) {
	m.width = 100
	m.height = 30

	// Create 40 apps to force scrolling
	apps := make([]config.Application, 40)
	for i := 0; i < 40; i++ {
		apps[i] = config.Application{
			Name:        fmt.Sprintf("app-%02d", i+1),
			Description: fmt.Sprintf("Application %d", i+1),
			Entries: []config.SubEntry{
				{
					Name:    "config",
					Backup:  fmt.Sprintf("./app-%02d", i+1),
					Targets: map[string]string{"linux": fmt.Sprintf("~/.config/app-%02d", i+1)},
				},
			},
		}
	}
	m.Config.Applications = apps

	m.initApplicationItems()
	m.Screen = ScreenResults
	m.Operation = OpList

	// Position cursor at last app - renderTable() will auto-scroll to show it
	m.tableCursor = 39
}

func setupScrollBottomThenUp(m *Model) {
	m.width = 100
	m.height = 30

	// Create 40 apps to force scrolling
	apps := make([]config.Application, 40)
	for i := 0; i < 40; i++ {
		apps[i] = config.Application{
			Name:        fmt.Sprintf("app-%02d", i+1),
			Description: fmt.Sprintf("Application %d", i+1),
			Entries: []config.SubEntry{
				{
					Name:    "config",
					Backup:  fmt.Sprintf("./app-%02d", i+1),
					Targets: map[string]string{"linux": fmt.Sprintf("~/.config/app-%02d", i+1)},
				},
			},
		}
	}
	m.Config.Applications = apps

	m.initApplicationItems()
	m.Screen = ScreenResults
	m.Operation = OpList

	// Simulate scrolling to bottom then up by setting cursor and updating scroll offset
	m.tableCursor = 39
	m.updateScrollOffset() // This sets scrollOffset to show bottom

	// Now move cursor up - scrollOffset should stay at bottom since cursor is still visible
	m.tableCursor = 30
	m.updateScrollOffset()
}

func setupScrollBottomThenTop(m *Model) {
	m.width = 100
	m.height = 30

	// Create 40 apps to force scrolling
	apps := make([]config.Application, 40)
	for i := 0; i < 40; i++ {
		apps[i] = config.Application{
			Name:        fmt.Sprintf("app-%02d", i+1),
			Description: fmt.Sprintf("Application %d", i+1),
			Entries: []config.SubEntry{
				{
					Name:    "config",
					Backup:  fmt.Sprintf("./app-%02d", i+1),
					Targets: map[string]string{"linux": fmt.Sprintf("~/.config/app-%02d", i+1)},
				},
			},
		}
	}
	m.Config.Applications = apps

	m.initApplicationItems()
	m.Screen = ScreenResults
	m.Operation = OpList

	// Simulate scrolling to bottom then to top
	m.tableCursor = 39
	m.updateScrollOffset() // This sets scrollOffset to show bottom

	// Now move cursor to top - scrollOffset will adjust to show cursor at top
	m.tableCursor = 0
	m.updateScrollOffset()
}

// setupScrollWithExpanded creates a scrollable list with an expanded app.
// Tests: expanded app scrolling, sub-entry visibility in viewport.
func setupScrollWithExpanded(m *Model) {
	m.width = 100
	m.height = 30

	// Create 25 apps to force scrolling even with expansion
	apps := make([]config.Application, 25)
	for i := 0; i < 25; i++ {
		if i == 7 {
			// App at index 7 has multiple sub-entries
			apps[i] = config.Application{
				Name:        "app-08",
				Description: "Application 8 with many entries",
				Entries: []config.SubEntry{
					{Name: "entry-1", Backup: "./app-08/entry-1", Targets: map[string]string{"linux": "~/.config/app-08/entry-1"}},
					{Name: "entry-2", Backup: "./app-08/entry-2", Targets: map[string]string{"linux": "~/.config/app-08/entry-2"}},
					{Name: "entry-3", Backup: "./app-08/entry-3", Targets: map[string]string{"linux": "~/.config/app-08/entry-3"}},
					{Name: "entry-4", Backup: "./app-08/entry-4", Targets: map[string]string{"linux": "~/.config/app-08/entry-4"}},
					{Name: "entry-5", Backup: "./app-08/entry-5", Targets: map[string]string{"linux": "~/.config/app-08/entry-5"}},
				},
			}
		} else {
			apps[i] = config.Application{
				Name:        fmt.Sprintf("app-%02d", i+1),
				Description: fmt.Sprintf("Application %d", i+1),
				Entries: []config.SubEntry{
					{
						Name:    "config",
						Backup:  fmt.Sprintf("./app-%02d", i+1),
						Targets: map[string]string{"linux": fmt.Sprintf("~/.config/app-%02d", i+1)},
					},
				},
			}
		}
	}
	m.Config.Applications = apps

	m.initApplicationItems()

	// Expand app-08 (index 7 after alphabetical sort)
	m.Applications[7].Expanded = true

	// Rebuild table to show expanded sub-entries
	m.initTableModel()

	m.Screen = ScreenResults
	m.Operation = OpList

	// Position cursor on 3rd sub-entry of app-08
	// With 25 apps + 5 sub-entries visible, cursor at row 10 will test scrolling
	// Visual layout: app-08 is at row 7, sub-entries start at row 8
	// So 3rd sub-entry is at row 10 (7 + 1 for app + 2 for first 2 sub-entries)
	m.tableCursor = 10
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
	m.tableCursor = 0
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

	m.Applications[1].Expanded = true
	m.initTableModel()

	m.Screen = ScreenResults
	m.Operation = OpList
	m.tableCursor = 1
}

func setupExpandWhenAtCapacity(m *Model) {
	m.width = 150
	m.height = 48

	apps := make([]config.Application, 36)
	for i := 0; i < 36; i++ {
		apps[i] = config.Application{
			Name:        fmt.Sprintf("app-%02d", i+1),
			Description: fmt.Sprintf("Application %d", i+1),
			Entries: []config.SubEntry{
				{Name: "init.lua", Backup: "./nvim/init", Targets: map[string]string{"linux": "~/.config/nvim/init.lua"}},
				{Name: "plugins", Backup: "./nvim/plugins", Targets: map[string]string{"linux": "~/.config/nvim/lua/plugins"}},
				{Name: "mappings", Backup: "./nvim/mappings", Targets: map[string]string{"linux": "~/.config/nvim/lua/mappings.lua"}},
				{Name: "settings", Backup: "./nvim/settings", Targets: map[string]string{"linux": "~/.config/nvim/lua/settings.lua"}},
			},
		}
	}
	m.Config.Applications = apps
	m.initApplicationItems()

	m.Applications[1].Expanded = true
	m.initTableModel()

	m.Screen = ScreenResults
	m.Operation = OpList
	m.tableCursor = 1
}
