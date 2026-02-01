package tui

import (
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel(t *testing.T) {
	cfg := &config.Config{
		BackupRoot: "/home/user/backup",
		Paths: []config.PathSpec{
			{
				Name:   "nvim",
				Files:  []string{},
				Backup: "./nvim",
				Targets: map[string]string{
					"linux": "~/.config/nvim",
				},
			},
			{
				Name:   "bash",
				Files:  []string{".bashrc"},
				Backup: "./bash",
				Targets: map[string]string{
					"linux": "~",
				},
			},
			{
				Name:   "windows-only",
				Backup: "./windows",
				Targets: map[string]string{
					"windows": "~/AppData",
				},
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
			t.Errorf("Path %s should be selected by default", p.Spec.Name)
		}
	}

	// Should start at menu screen
	if model.Screen != ScreenMenu {
		t.Errorf("Expected ScreenMenu, got %v", model.Screen)
	}
}

func TestNewModelRootPaths(t *testing.T) {
	cfg := &config.Config{
		BackupRoot: "/home/user/backup",
		Paths: []config.PathSpec{
			{Name: "user-config", Backup: "./user", Targets: map[string]string{"linux": "~/.config"}},
		},
		RootPaths: []config.PathSpec{
			{Name: "root-config", Backup: "./root", Targets: map[string]string{"linux": "/etc"}},
		},
	}

	// Non-root user
	plat := &platform.Platform{OS: platform.OSLinux, IsRoot: false}
	model := NewModel(cfg, plat, false)

	if len(model.Paths) != 1 || model.Paths[0].Spec.Name != "user-config" {
		t.Error("Non-root user should see user paths")
	}

	// Root user
	plat = &platform.Platform{OS: platform.OSLinux, IsRoot: true}
	model = NewModel(cfg, plat, false)

	if len(model.Paths) != 1 || model.Paths[0].Spec.Name != "root-config" {
		t.Error("Root user should see root paths")
	}
}

func TestModelUpdate(t *testing.T) {
	cfg := &config.Config{
		BackupRoot: "/home/user/backup",
		Paths: []config.PathSpec{
			{Name: "test", Backup: "./test", Targets: map[string]string{"linux": "~/.config"}},
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
		op   Operation
		want string
	}{
		{OpRestore, "Restore"},
		{OpBackup, "Backup"},
		{OpList, "List"},
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
		BackupRoot: "/backup",
		Paths: []config.PathSpec{
			{Name: "folder", Files: []string{}, Backup: "./folder", Targets: map[string]string{"linux": "~/.config/folder"}},
			{Name: "files", Files: []string{"a.txt", "b.txt"}, Backup: "./files", Targets: map[string]string{"linux": "~/.config"}},
		},
	}
	plat := &platform.Platform{OS: platform.OSLinux}

	model := NewModel(cfg, plat, false)

	if !model.Paths[0].Spec.IsFolder() {
		t.Error("First path should be a folder")
	}

	if model.Paths[1].Spec.IsFolder() {
		t.Error("Second path should not be a folder")
	}
}

func TestModelView(t *testing.T) {
	cfg := &config.Config{
		BackupRoot: "/backup",
		Paths: []config.PathSpec{
			{Name: "test", Backup: "./test", Targets: map[string]string{"linux": "~/.config"}},
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
		BackupRoot: "/backup",
		Paths: []config.PathSpec{
			{Name: "test", Backup: "./test", Targets: map[string]string{"linux": "~/.config"}},
		},
	}
	plat := &platform.Platform{OS: platform.OSLinux}

	model := NewModel(cfg, plat, true)

	if !model.DryRun {
		t.Error("DryRun should be true")
	}
}
