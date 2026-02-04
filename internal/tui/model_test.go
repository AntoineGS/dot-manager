package tui

import (
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel(t *testing.T) {
	cfg := &config.Config{
		Version:    3,
		BackupRoot: "/home/user/backup",
		Applications: []config.Application{
			{
				Name:        "nvim",
				Description: "Neovim editor",
				Entries: []config.SubEntry{
					{
						Name:   "nvim-config",
						Files:  []string{},
						Backup: "./nvim",
						Targets: map[string]string{
							"linux": "~/.config/nvim",
						},
					},
				},
			},
			{
				Name:        "bash",
				Description: "Bash shell",
				Entries: []config.SubEntry{
					{
						Name:   "bashrc",
						Files:  []string{".bashrc"},
						Backup: "./bash",
						Targets: map[string]string{
							"linux": "~",
						},
					},
				},
			},
			{
				Name:        "windows-only",
				Description: "Windows only app",
				Entries: []config.SubEntry{
					{
						Name:   "windows-config",
						Backup: "./windows",
						Targets: map[string]string{
							"windows": "~/AppData",
						},
					},
				},
				Filters: []config.Filter{{Include: map[string]string{"os": "windows"}}},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}

	model := NewModel(cfg, plat, false)

	// Should have 2 paths (windows-only excluded)
	if len(model.Paths) != 2 {
		t.Errorf("Expected 2 paths, got %d", len(model.Paths))
	}

	// All should be selected by default
	for _, p := range model.Paths {
		if !p.Selected {
			t.Errorf("Path %s should be selected by default", p.Entry.Name)
		}
	}

	// Should start at menu screen
	if model.Screen != ScreenMenu {
		t.Errorf("Expected ScreenMenu, got %v", model.Screen)
	}
}

func TestNewModelRootPaths(t *testing.T) {
	cfg := &config.Config{
		Version:    3,
		BackupRoot: "/home/user/backup",
		Applications: []config.Application{
			{
				Name: "user-app",
				Entries: []config.SubEntry{
					{Name: "user-config", Backup: "./user", Targets: map[string]string{"linux": "~/.config"}},
				},
			},
			{
				Name: "root-app",
				Entries: []config.SubEntry{
					{Name: "root-config", Backup: "./root", Targets: map[string]string{"linux": "/etc"}, Sudo: true},
				},
			},
		},
	}

	// All entries are shown regardless of Root flag
	plat := &platform.Platform{OS: platform.OSLinux}
	model := NewModel(cfg, plat, false)

	if len(model.Paths) != 2 {
		t.Errorf("Expected 2 paths, got %d", len(model.Paths))
	}

	// Verify both paths are present (names are prefixed with app name)
	names := make(map[string]bool)
	for _, p := range model.Paths {
		names[p.Entry.Name] = true
	}

	if !names["user-app/user-config"] {
		t.Error("Should include user-app/user-config")
	}

	if !names["root-app/root-config"] {
		t.Error("Should include root-app/root-config")
	}
}

func TestModelUpdate(t *testing.T) {
	cfg := &config.Config{
		Version:    3,
		BackupRoot: "/home/user/backup",
		Applications: []config.Application{
			{
				Name: "test-app",
				Entries: []config.SubEntry{
					{Name: "test", Backup: "./test", Targets: map[string]string{"linux": "~/.config"}},
				},
			},
		},
	}
	plat := &platform.Platform{OS: platform.OSLinux}

	model := NewModel(cfg, plat, false)

	// Test window size message
	newModel, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 50})
	m := newModel.(Model)

	if m.width != 100 || m.height != 50 {
		t.Error("Window size not updated correctly")
	}
}

func TestOperationString(t *testing.T) {
	tests := []struct {
		want string
		op   Operation
	}{
		{"Restore", OpRestore},
		{"Restore (Dry Run)", OpRestoreDryRun},
		{"Add", OpAdd},
		{"List", OpList},
		{"Install Packages", OpInstallPackages},
	}

	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			if got := tt.op.String(); got != tt.want {
				t.Errorf("Operation.String() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestPathItemIsFolder(t *testing.T) {
	cfg := &config.Config{
		Version:    3,
		BackupRoot: "/backup",
		Applications: []config.Application{
			{
				Name: "folder-app",
				Entries: []config.SubEntry{
					{Name: "folder", Files: []string{}, Backup: "./folder", Targets: map[string]string{"linux": "~/.config/folder"}},
				},
			},
			{
				Name: "files-app",
				Entries: []config.SubEntry{
					{Name: "files", Files: []string{"a.txt", "b.txt"}, Backup: "./files", Targets: map[string]string{"linux": "~/.config"}},
				},
			},
		},
	}
	plat := &platform.Platform{OS: platform.OSLinux}

	model := NewModel(cfg, plat, false)

	// Entries are sorted by name, so find them by name
	var folderPath, filesPath *PathItem

	for i := range model.Paths {
		switch model.Paths[i].Entry.Name {
		case "folder-app/folder":
			folderPath = &model.Paths[i]
		case "files-app/files":
			filesPath = &model.Paths[i]
		}
	}

	if folderPath == nil || !folderPath.Entry.IsFolder() {
		t.Error("folder entry should be a folder")
	}

	if filesPath == nil || filesPath.Entry.IsFolder() {
		t.Error("files entry should not be a folder")
	}
}

func TestModelView(t *testing.T) {
	cfg := &config.Config{
		Version:    3,
		BackupRoot: "/backup",
		Applications: []config.Application{
			{
				Name: "test-app",
				Entries: []config.SubEntry{
					{Name: "test", Backup: "./test", Targets: map[string]string{"linux": "~/.config"}},
				},
			},
		},
	}
	plat := &platform.Platform{OS: platform.OSLinux}

	model := NewModel(cfg, plat, false)

	// Test menu view
	view := model.View()
	if view == "" {
		t.Error("Menu view should not be empty")
	}

	// Test path select view
	model.Screen = ScreenPathSelect

	view = model.View()
	if view == "" {
		t.Error("Path select view should not be empty")
	}

	// Test confirm view
	model.Screen = ScreenConfirm

	view = model.View()
	if view == "" {
		t.Error("Confirm view should not be empty")
	}

	// Test results view
	model.Screen = ScreenResults
	model.results = []ResultItem{{Name: "test", Success: true, Message: "OK"}}

	view = model.View()
	if view == "" {
		t.Error("Results view should not be empty")
	}
}

func TestDryRunMode(t *testing.T) {
	cfg := &config.Config{
		Version:    3,
		BackupRoot: "/backup",
		Applications: []config.Application{
			{
				Name: "test-app",
				Entries: []config.SubEntry{
					{Name: "test", Backup: "./test", Targets: map[string]string{"linux": "~/.config"}},
				},
			},
		},
	}
	plat := &platform.Platform{OS: platform.OSLinux}

	model := NewModel(cfg, plat, true)

	if !model.DryRun {
		t.Error("DryRun should be true")
	}
}

func TestGetApplicationAtCursor(t *testing.T) {
	// Create a v3 config with multiple applications
	cfg := &config.Config{
		Version:    3,
		BackupRoot: "/backup",
		Applications: []config.Application{
			{
				Name:        "zsh",
				Description: "Z Shell",
				Entries: []config.SubEntry{
					{Name: "zshrc", Backup: "./zsh", Targets: map[string]string{"linux": "~/.zshrc"}},
				},
			},
			{
				Name:        "nvim",
				Description: "Neovim",
				Entries: []config.SubEntry{
					{Name: "init.lua", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim/init.lua"}},
					{Name: "plugins", Backup: "./nvim/plugins", Targets: map[string]string{"linux": "~/.config/nvim/lua/plugins"}},
				},
			},
			{
				Name:        "bash",
				Description: "Bash Shell",
				Entries: []config.SubEntry{
					{Name: "bashrc", Backup: "./bash", Targets: map[string]string{"linux": "~/.bashrc"}},
				},
			},
		},
	}
	plat := &platform.Platform{OS: platform.OSLinux}

	model := NewModel(cfg, plat, false)

	// Initialize applications for hierarchical view
	model.initApplicationItems()

	// Applications should be sorted: bash, nvim, zsh
	if len(model.Applications) != 3 {
		t.Fatalf("Expected 3 applications, got %d", len(model.Applications))
	}

	if model.Applications[0].Application.Name != "bash" {
		t.Errorf("Expected first app to be 'bash', got '%s'", model.Applications[0].Application.Name)
	}

	if model.Applications[1].Application.Name != "nvim" { // nolint:goconst // test data
		t.Errorf("Expected second app to be 'nvim', got '%s'", model.Applications[1].Application.Name)
	}

	if model.Applications[2].Application.Name != "zsh" {
		t.Errorf("Expected third app to be 'zsh', got '%s'", model.Applications[2].Application.Name)
	}

	// Test cursor positions after sorting
	tests := []struct {
		expanded    map[int]bool // which apps are expanded
		name        string
		wantAppName string
		wantSubName string
		cursor      int
		wantAppIdx  int
		wantSubIdx  int
	}{
		{
			name:        "cursor on first app (bash)",
			cursor:      0,
			expanded:    nil,
			wantAppIdx:  0,
			wantSubIdx:  -1,
			wantAppName: "bash",
		},
		{
			name:        "cursor on second app (nvim)",
			cursor:      1,
			expanded:    nil,
			wantAppIdx:  1,
			wantSubIdx:  -1,
			wantAppName: "nvim",
		},
		{
			name:        "cursor on nvim's first sub-entry",
			cursor:      2,
			expanded:    map[int]bool{1: true}, // expand nvim
			wantAppIdx:  1,
			wantSubIdx:  0,
			wantAppName: "nvim",
			wantSubName: "init.lua",
		},
		{
			name:        "cursor on nvim's second sub-entry",
			cursor:      3,
			expanded:    map[int]bool{1: true}, // expand nvim
			wantAppIdx:  1,
			wantSubIdx:  1,
			wantAppName: "nvim",
			wantSubName: "plugins",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Reset expansion state
			for i := range model.Applications {
				model.Applications[i].Expanded = false
			}

			// Set expansion state for this test
			if tt.expanded != nil {
				for idx, expanded := range tt.expanded {
					model.Applications[idx].Expanded = expanded
				}
			}

			model.appCursor = tt.cursor
			appIdx, subIdx := model.getApplicationAtCursor()

			if appIdx != tt.wantAppIdx {
				t.Errorf("getApplicationAtCursor() appIdx = %d, want %d", appIdx, tt.wantAppIdx)
			}

			if subIdx != tt.wantSubIdx {
				t.Errorf("getApplicationAtCursor() subIdx = %d, want %d", subIdx, tt.wantSubIdx)
			}

			// Verify we got the right application
			if appIdx >= 0 && appIdx < len(model.Applications) {
				gotAppName := model.Applications[appIdx].Application.Name
				if gotAppName != tt.wantAppName {
					t.Errorf("Got app name %q, want %q", gotAppName, tt.wantAppName)
				}

				// Verify we got the right sub-entry
				if subIdx >= 0 && subIdx < len(model.Applications[appIdx].SubItems) {
					gotSubName := model.Applications[appIdx].SubItems[subIdx].SubEntry.Name
					if gotSubName != tt.wantSubName {
						t.Errorf("Got sub-entry name %q, want %q", gotSubName, tt.wantSubName)
					}
				}
			}
		})
	}
}

func TestGetApplicationAtCursorWithFiltering(t *testing.T) {
	// Create a v3 config with multiple applications
	cfg := &config.Config{
		Version:    3,
		BackupRoot: "/backup",
		Applications: []config.Application{
			{
				Name:        "zsh",
				Description: "Z Shell",
				Entries: []config.SubEntry{
					{Name: "zshrc", Backup: "./zsh", Targets: map[string]string{"linux": "~/.zshrc"}},
				},
			},
			{
				Name:        "nvim",
				Description: "Neovim",
				Entries: []config.SubEntry{
					{Name: "alpha", Backup: "./nvim/alpha", Targets: map[string]string{"linux": "~/.config/nvim/alpha.lua"}},
					{Name: "beta", Backup: "./nvim/beta", Targets: map[string]string{"linux": "~/.config/nvim/beta.lua"}},
					{Name: "gamma", Backup: "./nvim/gamma", Targets: map[string]string{"linux": "~/.config/nvim/gamma.lua"}},
				},
			},
			{
				Name:        "bash",
				Description: "Bash Shell",
				Entries: []config.SubEntry{
					{Name: "bashrc", Backup: "./bash", Targets: map[string]string{"linux": "~/.bashrc"}},
				},
			},
		},
	}
	plat := &platform.Platform{OS: platform.OSLinux}

	model := NewModel(cfg, plat, false)
	model.initApplicationItems()
	model.Applications[1].Expanded = true // Expand nvim

	// Applications are sorted: bash(0), nvim(1), zsh(2)
	// nvim has sub-items: alpha(0), beta(1), gamma(2)

	// Apply filter that matches only "alpha" and "gamma" sub-entries
	model.filterText = "amm" // Matches "alpha" and "gamma" (both contain 'a')

	// Visual layout after filtering:
	// Row 0: bash
	// Row 1: nvim (expanded)
	// Row 2:   alpha (matches filter)
	// Row 3:   gamma (matches filter)
	// Row 4: zsh

	// Now test that cursor on row 3 (gamma) correctly returns appIdx=1, subIdx=2
	// (gamma is at index 2 in the original SubItems, not index 1 in filtered)
	model.appCursor = 1

	appIdx, subIdx := model.getApplicationAtCursor()

	if appIdx != 1 {
		t.Errorf("Expected appIdx=1 (nvim), got %d", appIdx)
	}

	if subIdx != 2 {
		t.Errorf("Expected subIdx=2 (gamma in original array), got %d", subIdx)
	}

	// Verify we got the correct sub-entry
	if appIdx >= 0 && subIdx >= 0 && appIdx < len(model.Applications) && subIdx < len(model.Applications[appIdx].SubItems) {
		gotName := model.Applications[appIdx].SubItems[subIdx].SubEntry.Name
		if gotName != "gamma" {
			t.Errorf("Expected sub-entry 'gamma', got %q", gotName)
		}
	}
}

func TestEditAfterSortingBug(t *testing.T) {
	// This test reproduces the user's reported bug:
	// "when the list sorts by name, pressing 'e' edits the wrong record"

	// Create a v3 config where applications will be re-ordered after sorting
	cfg := &config.Config{
		Version:    3,
		BackupRoot: "/backup",
		Applications: []config.Application{
			{
				Name:        "zsh", // Will be last after sorting
				Description: "Z Shell",
				Entries: []config.SubEntry{
					{Name: "zshrc", Backup: "./zsh", Targets: map[string]string{"linux": "~/.zshrc"}},
				},
			},
			{
				Name:        "bash", // Will be first after sorting
				Description: "Bash Shell",
				Entries: []config.SubEntry{
					{Name: "bashrc", Backup: "./bash", Targets: map[string]string{"linux": "~/.bashrc"}},
				},
			},
			{
				Name:        "nvim", // Will be middle after sorting
				Description: "Neovim",
				Entries: []config.SubEntry{
					{Name: "init", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim/init.lua"}},
				},
			},
		},
	}
	plat := &platform.Platform{OS: platform.OSLinux}

	model := NewModel(cfg, plat, false)
	model.initApplicationItems()

	// Applications should now be sorted: bash(0), nvim(1), zsh(2)
	if len(model.Applications) != 3 {
		t.Fatalf("Expected 3 applications, got %d", len(model.Applications))
	}

	// Verify sorted order
	expectedOrder := []string{"bash", "nvim", "zsh"}
	for i, expected := range expectedOrder {
		if model.Applications[i].Application.Name != expected {
			t.Errorf("Expected app[%d] to be %q, got %q", i, expected, model.Applications[i].Application.Name)
		}
	}

	// Move cursor to row 1 (nvim) and try to "edit" it
	model.appCursor = 1
	appIdx, subIdx := model.getApplicationAtCursor()

	if appIdx != 1 {
		t.Errorf("Cursor at row 1 should return appIdx=1 (nvim), got %d", appIdx)
	}

	if subIdx != -1 {
		t.Errorf("Cursor at row 1 (app level) should return subIdx=-1, got %d", subIdx)
	}

	// Verify we're editing the correct application
	if appIdx >= 0 && appIdx < len(model.Applications) {
		gotName := model.Applications[appIdx].Application.Name
		if gotName != "nvim" {
			t.Errorf("Expected to edit 'nvim', but got %q", gotName)
		}
	}
}

func TestEditWithSortedApplications(t *testing.T) {
	// Reproduce the user's bug: after sorting, pressing 'e' should edit the correct application
	cfg := &config.Config{
		Version:    3,
		BackupRoot: "/backup",
		// Applications are defined in this order, but will be sorted alphabetically
		Applications: []config.Application{
			{
				Name:        "zsh",
				Description: "Z Shell",
				Entries: []config.SubEntry{
					{Name: "zshrc", Backup: "./zsh", Targets: map[string]string{"linux": "~/.zshrc"}},
				},
			},
			{
				Name:        "bash",
				Description: "Bash Shell",
				Entries: []config.SubEntry{
					{Name: "bashrc", Backup: "./bash", Targets: map[string]string{"linux": "~/.bashrc"}},
				},
			},
			{
				Name:        "nvim",
				Description: "Neovim",
				Entries: []config.SubEntry{
					{Name: "init", Backup: "./nvim", Targets: map[string]string{"linux": "~/.config/nvim/init.lua"}},
					{Name: "plugins", Backup: "./nvim/plugins", Targets: map[string]string{"linux": "~/.config/nvim/plugins"}},
				},
			},
		},
	}
	plat := &platform.Platform{OS: platform.OSLinux}

	model := NewModel(cfg, plat, false)
	model.initApplicationItems()
	model.Applications[1].Expanded = true // Expand nvim

	// After sorting: bash(0), nvim(1), zsh(2)
	// But in Config.Applications: zsh(0), bash(1), nvim(2)

	// Test editing application at visual row 1 (nvim)
	model.appCursor = 1
	appIdx, _ := model.getApplicationAtCursor()

	// Initialize the application form for editing
	model.initApplicationFormEdit(appIdx)

	// Verify the form was initialized with the correct application data
	if model.applicationForm == nil {
		t.Fatal("Application form should be initialized")
	}

	if model.applicationForm.nameInput.Value() != "nvim" {
		t.Errorf("Expected form to edit 'nvim', got %q", model.applicationForm.nameInput.Value())
	}

	// Test editing sub-entry at visual row 2 (init)
	model.appCursor = 2
	appIdx, subIdx := model.getApplicationAtCursor()

	model.initSubEntryFormEdit(appIdx, subIdx)

	if model.subEntryForm == nil {
		t.Fatal("Sub-entry form should be initialized")
	}

	if model.subEntryForm.nameInput.Value() != "init" {
		t.Errorf("Expected form to edit 'init', got %q", model.subEntryForm.nameInput.Value())
	}

	// Test editing sub-entry at visual row 3 (plugins)
	model.appCursor = 3
	appIdx, subIdx = model.getApplicationAtCursor()

	model.initSubEntryFormEdit(appIdx, subIdx)

	if model.subEntryForm == nil {
		t.Fatal("Sub-entry form should be initialized")
	}

	if model.subEntryForm.nameInput.Value() != "plugins" {
		t.Errorf("Expected form to edit 'plugins', got %q", model.subEntryForm.nameInput.Value())
	}
}
