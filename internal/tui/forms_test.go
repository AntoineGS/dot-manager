package tui

import (
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
)

func TestApplicationForm_Validation(t *testing.T) {
	tests := []struct {
		name    string
		app     config.Application
		wantErr bool
	}{
		{
			name: "valid_application",
			app: config.Application{
				Name:        "test",
				Description: "Test app",
			},
			wantErr: false,
		},
		{
			name: "empty_name",
			app: config.Application{
				Name:        "",
				Description: "Test",
			},
			wantErr: true,
		},
		{
			name: "whitespace_name",
			app: config.Application{
				Name:        "   ",
				Description: "Test",
			},
			wantErr: true,
		},
		{
			name: "valid_with_empty_description",
			app: config.Application{
				Name:        "test",
				Description: "",
			},
			wantErr: false,
		},
		{
			name: "valid_with_special_chars",
			app: config.Application{
				Name:        "test-app_v1",
				Description: "Test app with special chars!",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := NewApplicationForm(tt.app, false)
			err := form.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestApplicationForm_EditMode(t *testing.T) {
	app := config.Application{
		Name:        "test",
		Description: "Test app",
	}

	// Test new form (not editing)
	newForm := NewApplicationForm(app, false)
	if newForm.editAppIdx != -1 {
		t.Errorf("NewApplicationForm(false) editAppIdx = %d, want -1", newForm.editAppIdx)
	}

	// Test edit form
	editForm := NewApplicationForm(app, true)
	if editForm.editAppIdx != 0 {
		t.Errorf("NewApplicationForm(true) editAppIdx = %d, want 0", editForm.editAppIdx)
	}
}

func TestSubEntryForm_TypeValidation(t *testing.T) {
	tests := []struct {
		name    string
		entry   config.SubEntry
		wantErr bool
	}{
		{
			name: "valid_config_entry",
			entry: config.SubEntry{
				Name:   "test",
				Backup: "./test",
				Targets: map[string]string{
					"linux": "~/.config/test",
				},
			},
			wantErr: false,
		},
		{
			name: "config_missing_backup",
			entry: config.SubEntry{
				Name: "test",
				Targets: map[string]string{
					"linux": "~/.config/test",
				},
			},
			wantErr: true,
		},
		{
			name: "missing_name",
			entry: config.SubEntry{
				Name:   "",
				Backup: "./test",
				Targets: map[string]string{
					"linux": "~/.config/test",
				},
			},
			wantErr: true,
		},
		{
			name: "missing_targets",
			entry: config.SubEntry{
				Name:    "test",
				Backup:  "./test",
				Targets: map[string]string{},
			},
			wantErr: true,
		},
		{
			name: "whitespace_only_name",
			entry: config.SubEntry{
				Name:   "   ",
				Backup: "./test",
				Targets: map[string]string{
					"linux": "~/.config/test",
				},
			},
			wantErr: true,
		},
		{
			name: "valid_with_both_targets",
			entry: config.SubEntry{
				Name:   "test",
				Backup: "./test",
				Targets: map[string]string{
					"linux":   "~/.config/test",
					"windows": "~/AppData/Local/test",
				},
			},
			wantErr: false,
		},
		{
			name: "config_whitespace_backup",
			entry: config.SubEntry{
				Name:   "test",
				Backup: "   ",
				Targets: map[string]string{
					"linux": "~/.config/test",
				},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := NewSubEntryForm(tt.entry)
			err := form.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSubEntryForm_Construction(t *testing.T) {
	t.Run("config_entry", func(t *testing.T) {
		entry := config.SubEntry{
			Name:   "test",
			Backup: "./test",
			Sudo:   true,
			Files:  []string{".bashrc", ".profile"},
			Targets: map[string]string{
				"linux":   "~/.config/test",
				"windows": "~/AppData/Local/test",
			},
		}

		form := NewSubEntryForm(entry)

		if form.nameInput.Value() != "test" {
			t.Errorf("nameInput = %q, want %q", form.nameInput.Value(), "test")
		}

		if form.backupInput.Value() != "./test" {
			t.Errorf("backupInput = %q, want %q", form.backupInput.Value(), "./test")
		}

		if !form.isSudo {
			t.Error("isSudo = false, want true")
		}

		if form.linuxTargetInput.Value() != "~/.config/test" {
			t.Errorf("linuxTargetInput = %q, want %q", form.linuxTargetInput.Value(), "~/.config/test")
		}

		if form.windowsTargetInput.Value() != "~/AppData/Local/test" {
			t.Errorf("windowsTargetInput = %q, want %q", form.windowsTargetInput.Value(), "~/AppData/Local/test")
		}

		if len(form.files) != 2 {
			t.Errorf("files length = %d, want 2", len(form.files))
		}
	})
}

func TestBuildFiltersFromConditions(t *testing.T) {
	t.Run("empty_conditions", func(t *testing.T) {
		result := buildFiltersFromConditions(nil)
		if result != nil {
			t.Errorf("buildFiltersFromConditions(nil) = %v, want nil", result)
		}
	})

	t.Run("single_include_condition", func(t *testing.T) {
		conditions := []FilterCondition{
			{FilterIndex: 0, IsExclude: false, Key: "os", Value: "linux"},
		}
		result := buildFiltersFromConditions(conditions)

		if len(result) != 1 {
			t.Fatalf("len(result) = %d, want 1", len(result))
		}

		if len(result[0].Include) != 1 {
			t.Errorf("len(Include) = %d, want 1", len(result[0].Include))
		}

		if result[0].Include["os"] != "linux" {
			t.Errorf("Include[os] = %q, want %q", result[0].Include["os"], "linux")
		}
	})

	t.Run("single_exclude_condition", func(t *testing.T) {
		conditions := []FilterCondition{
			{FilterIndex: 0, IsExclude: true, Key: "distro", Value: "ubuntu"},
		}
		result := buildFiltersFromConditions(conditions)

		if len(result) != 1 {
			t.Fatalf("len(result) = %d, want 1", len(result))
		}

		if len(result[0].Exclude) != 1 {
			t.Errorf("len(Exclude) = %d, want 1", len(result[0].Exclude))
		}

		if result[0].Exclude["distro"] != "ubuntu" {
			t.Errorf("Exclude[distro] = %q, want %q", result[0].Exclude["distro"], "ubuntu")
		}
	})

	t.Run("multiple_conditions_same_filter", func(t *testing.T) {
		conditions := []FilterCondition{
			{FilterIndex: 0, IsExclude: false, Key: "os", Value: "linux"},
			{FilterIndex: 0, IsExclude: true, Key: "distro", Value: "ubuntu"},
		}
		result := buildFiltersFromConditions(conditions)

		if len(result) != 1 {
			t.Fatalf("len(result) = %d, want 1", len(result))
		}

		if len(result[0].Include) != 1 {
			t.Errorf("len(Include) = %d, want 1", len(result[0].Include))
		}

		if len(result[0].Exclude) != 1 {
			t.Errorf("len(Exclude) = %d, want 1", len(result[0].Exclude))
		}
	})

	t.Run("multiple_filter_groups", func(t *testing.T) {
		conditions := []FilterCondition{
			{FilterIndex: 0, IsExclude: false, Key: "os", Value: "linux"},
			{FilterIndex: 1, IsExclude: false, Key: "distro", Value: "arch"},
		}
		result := buildFiltersFromConditions(conditions)

		if len(result) != 2 {
			t.Fatalf("len(result) = %d, want 2", len(result))
		}

		if result[0].Include["os"] != "linux" {
			t.Errorf("result[0].Include[os] = %q, want %q", result[0].Include["os"], "linux")
		}

		if result[1].Include["distro"] != "arch" {
			t.Errorf("result[1].Include[distro] = %q, want %q", result[1].Include["distro"], "arch")
		}
	})
}

func TestBuildPackageSpec(t *testing.T) {
	t.Run("empty_managers", func(t *testing.T) {
		result := buildPackageSpec(map[string]string{})
		if result != nil {
			t.Errorf("buildPackageSpec(empty) = %v, want nil", result)
		}
	})

	t.Run("nil_managers", func(t *testing.T) {
		result := buildPackageSpec(nil)
		if result != nil {
			t.Errorf("buildPackageSpec(nil) = %v, want nil", result)
		}
	})

	t.Run("single_manager", func(t *testing.T) {
		managers := map[string]string{
			"pacman": "neovim",
		}
		result := buildPackageSpec(managers)

		if result == nil {
			t.Fatal("buildPackageSpec returned nil, want non-nil")
		}

		if len(result.Managers) != 1 {
			t.Errorf("len(Managers) = %d, want 1", len(result.Managers))
		}

		if result.Managers["pacman"] != "neovim" {
			t.Errorf("Managers[pacman] = %q, want %q", result.Managers["pacman"], "neovim")
		}
	})

	t.Run("multiple_managers", func(t *testing.T) {
		managers := map[string]string{
			"pacman": "neovim",
			"apt":    "neovim",
			"brew":   "neovim",
		}
		result := buildPackageSpec(managers)

		if result == nil {
			t.Fatal("buildPackageSpec returned nil, want non-nil")
		}

		if len(result.Managers) != 3 {
			t.Errorf("len(Managers) = %d, want 3", len(result.Managers))
		}

		for mgr, pkg := range managers {
			if result.Managers[mgr] != pkg {
				t.Errorf("Managers[%s] = %q, want %q", mgr, result.Managers[mgr], pkg)
			}
		}
	})
}
