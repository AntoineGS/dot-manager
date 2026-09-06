package forms_test

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/tui/forms"
	"github.com/AntoineGS/tidydots/internal/tui/tuishared"
)

func TestSubEntryFormPreservesSetupCommands(t *testing.T) {
	command := "check\n\tprintf ok\n" + strings.Repeat("x", 513)
	form := forms.NewSubEntryForm(config.SubEntry{
		Name:  "setup",
		Check: map[string]string{"linux": command},
		Run:   map[string]string{"linux": command},
	})
	form.LinuxCheckInput.Focus()
	updated, _ := form.LinuxCheckInput.Update(tea.PasteMsg{Content: "!"})
	form.LinuxCheckInput = updated
	form.LinuxRunInput.Focus()
	updated, _ = form.LinuxRunInput.Update(tea.PasteMsg{Content: "!"})
	form.LinuxRunInput = updated
	command += "!"
	got, err := form.BuildSubEntry()
	if err != nil {
		t.Fatalf("BuildSubEntry() error = %v", err)
	}
	if got.Check["linux"] != command || got.Run["linux"] != command {
		t.Fatalf("setup command text was not preserved: check=%q run=%q", got.Check["linux"], got.Run["linux"])
	}
	path := t.TempDir() + "/tidydots.yaml"
	if err := config.Save(&config.Config{Version: 3, Applications: []config.Application{{Name: "tool", Entries: []config.SubEntry{got}}}}, path); err != nil {
		t.Fatalf("config save failed: %v", err)
	}
	reloaded, err := config.Load(path)
	if err != nil || reloaded.Applications[0].Entries[0].Check["linux"] != command || reloaded.Applications[0].Entries[0].Run["linux"] != command {
		t.Fatalf("interactive setup command changed after config save/reload: err=%v", err)
	}
}

func TestSubEntryFormWhenRoundTrip(t *testing.T) {
	const when = `{{ eq .Hostname "omarchbook" }}`
	tests := []struct {
		name  string
		entry config.SubEntry
	}{
		{
			name:  "config",
			entry: config.SubEntry{Name: "config", Backup: "./config", Targets: map[string]string{"linux": "~/.config"}},
		},
		{
			name:  "setup",
			entry: config.SubEntry{Name: "setup", Check: map[string]string{"linux": "check"}, Run: map[string]string{"linux": "run"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entry := tt.entry
			entry.When = when
			form := forms.NewSubEntryForm(entry)
			if got := form.WhenInput.Value(); got != when {
				t.Fatalf("WhenInput = %q, want %q", got, when)
			}
			got, err := form.BuildSubEntry()
			if err != nil {
				t.Fatalf("BuildSubEntry() error = %v", err)
			}
			if got.When != when {
				t.Errorf("When = %q, want %q", got.When, when)
			}
		})
	}
}

func TestSubEntryFormLongWhenRoundTripForConfigAndSetup(t *testing.T) {
	when := `{{/*` + strings.Repeat("x", 600) + `*/}}true`
	tests := []struct {
		name  string
		entry config.SubEntry
	}{
		{
			name:  "config",
			entry: config.SubEntry{Name: "config", Backup: "./config", Targets: map[string]string{"linux": "~/.config"}},
		},
		{
			name:  "setup",
			entry: config.SubEntry{Name: "setup", Check: map[string]string{"linux": "check"}, Run: map[string]string{"linux": "run"}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.entry.When = when
			form := forms.NewSubEntryForm(tt.entry)
			if got := form.WhenInput.Value(); got != when {
				t.Fatalf("loaded When length=%d, want %d", len(got), len(when))
			}
			got, err := form.BuildSubEntry()
			if err != nil {
				t.Fatalf("BuildSubEntry() error = %v", err)
			}
			if got.When != when {
				t.Fatalf("built When length=%d, want %d", len(got.When), len(when))
			}
			path := t.TempDir() + "/tidydots.yaml"
			cfg := &config.Config{Version: 3, Applications: []config.Application{{Name: "tool", Entries: []config.SubEntry{got}}}}
			if err := config.Save(cfg, path); err != nil {
				t.Fatalf("config save failed: %v", err)
			}
			reloaded, err := config.Load(path)
			if err != nil {
				t.Fatalf("config reload failed: %v", err)
			}
			if got := reloaded.Applications[0].Entries[0].When; got != when {
				t.Errorf("reloaded When length=%d, want %d", len(got), len(when))
			}
		})
	}
}

func TestSubEntryFormWhenTrimsAndOmitsEmpty(t *testing.T) {
	form := forms.NewSubEntryForm(config.SubEntry{Name: "config", Backup: "./config", Targets: map[string]string{"linux": "~/.config"}})
	form.WhenInput.SetValue("  \t")
	got, err := form.BuildSubEntry()
	if err != nil {
		t.Fatalf("BuildSubEntry() error = %v", err)
	}
	if got.When != "" {
		t.Errorf("When = %q, want empty", got.When)
	}
}

func TestNewSubEntryForm_Empty(t *testing.T) {
	form := forms.NewSubEntryForm(config.SubEntry{})

	if form == nil {
		t.Fatal("NewSubEntryForm returned nil")
	}

	t.Run("inputs_are_empty", func(t *testing.T) {
		if form.NameInput.Value() != "" {
			t.Errorf("NameInput = %q, want empty", form.NameInput.Value())
		}
		if form.BackupInput.Value() != "" {
			t.Errorf("BackupInput = %q, want empty", form.BackupInput.Value())
		}
		if form.LinuxTargetInput.Value() != "" {
			t.Errorf("LinuxTargetInput = %q, want empty", form.LinuxTargetInput.Value())
		}
		if form.WindowsTargetInput.Value() != "" {
			t.Errorf("WindowsTargetInput = %q, want empty", form.WindowsTargetInput.Value())
		}
	})

	t.Run("sudo_is_false_by_default", func(t *testing.T) {
		if form.IsSudo {
			t.Error("IsSudo = true, want false")
		}
	})

	t.Run("is_folder_false_when_no_backup", func(t *testing.T) {
		// IsFolder requires both a backup path and no files;
		// without a backup, IsConfig() is false, so IsFolder() is false.
		if form.IsFolder {
			t.Error("IsFolder = true, want false for entry with no backup")
		}
	})
}

func TestNewSubEntryForm_PopulatedEntry(t *testing.T) {
	entry := config.SubEntry{
		Name:   "nvim-config",
		Backup: "./nvim",
		Sudo:   true,
		Files:  []string{"init.lua", "lua/config.lua"},
		Targets: map[string]string{
			"linux":   "~/.config/nvim",
			"windows": "~/AppData/Local/nvim",
		},
	}
	form := forms.NewSubEntryForm(entry)

	if form == nil {
		t.Fatal("NewSubEntryForm returned nil")
	}

	t.Run("name_loaded", func(t *testing.T) {
		if form.NameInput.Value() != "nvim-config" {
			t.Errorf("NameInput = %q, want %q", form.NameInput.Value(), "nvim-config")
		}
	})

	t.Run("backup_loaded", func(t *testing.T) {
		if form.BackupInput.Value() != "./nvim" {
			t.Errorf("BackupInput = %q, want %q", form.BackupInput.Value(), "./nvim")
		}
	})

	t.Run("targets_loaded", func(t *testing.T) {
		if form.LinuxTargetInput.Value() != "~/.config/nvim" {
			t.Errorf("LinuxTargetInput = %q, want %q", form.LinuxTargetInput.Value(), "~/.config/nvim")
		}
		if form.WindowsTargetInput.Value() != "~/AppData/Local/nvim" {
			t.Errorf("WindowsTargetInput = %q, want %q", form.WindowsTargetInput.Value(), "~/AppData/Local/nvim")
		}
	})

	t.Run("sudo_loaded", func(t *testing.T) {
		if !form.IsSudo {
			t.Error("IsSudo = false, want true")
		}
	})

	t.Run("files_loaded", func(t *testing.T) {
		if len(form.Files) != 2 {
			t.Errorf("len(Files) = %d, want 2", len(form.Files))
		}
		if form.Files[0] != "init.lua" {
			t.Errorf("Files[0] = %q, want %q", form.Files[0], "init.lua")
		}
		if form.Files[1] != "lua/config.lua" {
			t.Errorf("Files[1] = %q, want %q", form.Files[1], "lua/config.lua")
		}
	})

	t.Run("is_folder_false_when_files_present", func(t *testing.T) {
		if form.IsFolder {
			t.Error("IsFolder = true, want false when files are specified")
		}
	})
}

func TestNewSubEntryForm_FolderEntry(t *testing.T) {
	entry := config.SubEntry{
		Name:   "nvim-config",
		Backup: "./nvim",
		Targets: map[string]string{
			"linux": "~/.config/nvim",
		},
		// No Files → IsFolder
	}
	form := forms.NewSubEntryForm(entry)

	if !form.IsFolder {
		t.Error("IsFolder = false, want true when no files specified")
	}
}

func TestNewSubEntryForm_PartialTargets(t *testing.T) {
	t.Run("linux_only", func(t *testing.T) {
		entry := config.SubEntry{
			Name:    "test",
			Backup:  "./test",
			Targets: map[string]string{"linux": "~/.config/test"},
		}
		form := forms.NewSubEntryForm(entry)

		if form.LinuxTargetInput.Value() != "~/.config/test" {
			t.Errorf("LinuxTargetInput = %q, want %q", form.LinuxTargetInput.Value(), "~/.config/test")
		}
		if form.WindowsTargetInput.Value() != "" {
			t.Errorf("WindowsTargetInput = %q, want empty", form.WindowsTargetInput.Value())
		}
	})

	t.Run("windows_only", func(t *testing.T) {
		entry := config.SubEntry{
			Name:    "test",
			Backup:  "./test",
			Targets: map[string]string{"windows": "~/AppData/Local/test"},
		}
		form := forms.NewSubEntryForm(entry)

		if form.LinuxTargetInput.Value() != "" {
			t.Errorf("LinuxTargetInput = %q, want empty", form.LinuxTargetInput.Value())
		}
		if form.WindowsTargetInput.Value() != "~/AppData/Local/test" {
			t.Errorf("WindowsTargetInput = %q, want %q", form.WindowsTargetInput.Value(), "~/AppData/Local/test")
		}
	})
}

func TestSubEntryForm_Validate(t *testing.T) {
	tests := []struct {
		name    string
		entry   config.SubEntry
		wantErr bool
	}{
		{
			name: "valid_config_entry",
			entry: config.SubEntry{
				Name:    "nvim-config",
				Backup:  "./nvim",
				Targets: map[string]string{"linux": "~/.config/nvim"},
			},
			wantErr: false,
		},
		{
			name: "empty_name_returns_error",
			entry: config.SubEntry{
				Name:    "",
				Backup:  "./nvim",
				Targets: map[string]string{"linux": "~/.config/nvim"},
			},
			wantErr: true,
		},
		{
			name: "whitespace_name_returns_error",
			entry: config.SubEntry{
				Name:    "   ",
				Backup:  "./nvim",
				Targets: map[string]string{"linux": "~/.config/nvim"},
			},
			wantErr: true,
		},
		{
			name: "empty_backup_returns_error",
			entry: config.SubEntry{
				Name:    "nvim-config",
				Backup:  "",
				Targets: map[string]string{"linux": "~/.config/nvim"},
			},
			wantErr: true,
		},
		{
			name: "whitespace_backup_returns_error",
			entry: config.SubEntry{
				Name:    "nvim-config",
				Backup:  "   ",
				Targets: map[string]string{"linux": "~/.config/nvim"},
			},
			wantErr: true,
		},
		{
			name: "no_targets_returns_error",
			entry: config.SubEntry{
				Name:    "nvim-config",
				Backup:  "./nvim",
				Targets: map[string]string{},
			},
			wantErr: true,
		},
		{
			name: "valid_with_both_targets",
			entry: config.SubEntry{
				Name:   "nvim-config",
				Backup: "./nvim",
				Targets: map[string]string{
					"linux":   "~/.config/nvim",
					"windows": "~/AppData/Local/nvim",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := forms.NewSubEntryForm(tt.entry)
			err := form.Validate()

			if (err != nil) != tt.wantErr {
				t.Errorf("Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestSubEntryForm_BuildSubEntry(t *testing.T) {
	t.Run("nil_form_returns_error", func(t *testing.T) {
		var f *forms.SubEntryForm
		_, err := f.BuildSubEntry()
		if err == nil {
			t.Error("BuildSubEntry() on nil form should return error")
		}
	})

	t.Run("empty_name_returns_error", func(t *testing.T) {
		entry := config.SubEntry{
			Name:    "",
			Backup:  "./nvim",
			Targets: map[string]string{"linux": "~/.config/nvim"},
		}
		form := forms.NewSubEntryForm(entry)
		_, err := form.BuildSubEntry()
		if err == nil {
			t.Error("BuildSubEntry() with empty name should return error")
		}
	})

	t.Run("no_targets_returns_error", func(t *testing.T) {
		form := forms.NewSubEntryForm(config.SubEntry{
			Name:   "test",
			Backup: "./test",
		})
		_, err := form.BuildSubEntry()
		if err == nil {
			t.Error("BuildSubEntry() with no targets should return error")
		}
	})

	t.Run("no_backup_returns_error", func(t *testing.T) {
		form := forms.NewSubEntryForm(config.SubEntry{
			Name:    "test",
			Targets: map[string]string{"linux": "~/.config/test"},
		})
		_, err := form.BuildSubEntry()
		if err == nil {
			t.Error("BuildSubEntry() with no backup should return error")
		}
	})

	t.Run("builds_folder_entry", func(t *testing.T) {
		entry := config.SubEntry{
			Name:   "nvim-config",
			Backup: "./nvim",
			Targets: map[string]string{
				"linux": "~/.config/nvim",
			},
		}
		form := forms.NewSubEntryForm(entry)
		// IsFolder=true because no Files

		result, err := form.BuildSubEntry()
		if err != nil {
			t.Fatalf("BuildSubEntry() unexpected error: %v", err)
		}
		if result.Name != "nvim-config" {
			t.Errorf("Name = %q, want %q", result.Name, "nvim-config")
		}
		if result.Backup != "./nvim" {
			t.Errorf("Backup = %q, want %q", result.Backup, "./nvim")
		}
		if result.Targets["linux"] != "~/.config/nvim" {
			t.Errorf("Targets[linux] = %q, want %q", result.Targets["linux"], "~/.config/nvim")
		}
		if len(result.Files) != 0 {
			t.Errorf("len(Files) = %d, want 0 (folder mode)", len(result.Files))
		}
	})

	t.Run("builds_files_entry", func(t *testing.T) {
		entry := config.SubEntry{
			Name:   "nvim-config",
			Backup: "./nvim",
			Files:  []string{"init.lua", "lua/config.lua"},
			Targets: map[string]string{
				"linux": "~/.config/nvim",
			},
		}
		form := forms.NewSubEntryForm(entry)

		result, err := form.BuildSubEntry()
		if err != nil {
			t.Fatalf("BuildSubEntry() unexpected error: %v", err)
		}
		if len(result.Files) != 2 {
			t.Errorf("len(Files) = %d, want 2", len(result.Files))
		}
		if result.Files[0] != "init.lua" {
			t.Errorf("Files[0] = %q, want %q", result.Files[0], "init.lua")
		}
	})

	t.Run("files_mode_with_no_files_returns_error", func(t *testing.T) {
		form := forms.NewSubEntryForm(config.SubEntry{
			Name:    "test",
			Backup:  "./test",
			Targets: map[string]string{"linux": "~/.config/test"},
		})
		// IsFolder=true by default for empty Files; force files mode
		form.IsFolder = false
		form.Files = nil

		_, err := form.BuildSubEntry()
		if err == nil {
			t.Error("BuildSubEntry() in files mode with no files should return error")
		}
	})

	t.Run("sudo_preserved", func(t *testing.T) {
		entry := config.SubEntry{
			Name:    "hosts",
			Backup:  "./system/hosts",
			Sudo:    true,
			Targets: map[string]string{"linux": "/etc/hosts"},
		}
		form := forms.NewSubEntryForm(entry)

		result, err := form.BuildSubEntry()
		if err != nil {
			t.Fatalf("BuildSubEntry() unexpected error: %v", err)
		}
		if !result.Sudo {
			t.Error("Sudo = false, want true")
		}
	})
}

func TestNewSubEntryForm_SetupEntry(t *testing.T) {
	entry := config.SubEntry{
		Name: "setup",
		Check: map[string]string{
			"linux":   "command -v foo",
			"windows": "where foo",
		},
		Run: map[string]string{
			"linux":   "install foo",
			"windows": "install foo.exe",
		},
		Sudo: true,
	}
	form := forms.NewSubEntryForm(entry)

	if !form.IsSetup {
		t.Error("IsSetup = false, want true")
	}
	for name, got := range map[string]string{
		"linux check":   form.LinuxCheckInput.Value(),
		"linux run":     form.LinuxRunInput.Value(),
		"windows check": form.WindowsCheckInput.Value(),
		"windows run":   form.WindowsRunInput.Value(),
	} {
		want := map[string]string{
			"linux check":   entry.Check["linux"],
			"linux run":     entry.Run["linux"],
			"windows check": entry.Check["windows"],
			"windows run":   entry.Run["windows"],
		}[name]
		if got != want {
			t.Errorf("%s = %q, want %q", name, got, want)
		}
	}
	if !form.IsSudo {
		t.Error("IsSudo = false, want true")
	}
}

func TestSubEntryForm_SetupCommandExceedsPathLimit(t *testing.T) {
	command := strings.Repeat("x", tuishared.CharLimitPath+1)
	form := forms.NewSubEntryForm(config.SubEntry{
		Name:  "setup",
		Check: map[string]string{"linux": command},
		Run:   map[string]string{"linux": "run"},
	})

	got, err := form.BuildSubEntry()
	if err != nil {
		t.Fatalf("BuildSubEntry() error = %v", err)
	}
	if got.Check["linux"] != command {
		t.Errorf("Linux check command length = %d, want %d", len(got.Check["linux"]), len(command))
	}
}

func TestSubEntryForm_BuildSetupEntryValidation(t *testing.T) {
	tests := []struct {
		name      string
		check     map[string]string
		run       map[string]string
		wantErr   bool
		wantLinux bool
	}{
		{name: "no OS pair", wantErr: true},
		{name: "check only", check: map[string]string{"linux": "check"}, wantErr: true},
		{name: "run only", run: map[string]string{"linux": "run"}, wantErr: true},
		{name: "valid Windows pair", check: map[string]string{"windows": "check"}, run: map[string]string{"windows": "run"}, wantLinux: false},
		{name: "valid Linux pair", check: map[string]string{"linux": "check"}, run: map[string]string{"linux": "run"}, wantLinux: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := forms.NewSubEntryForm(config.SubEntry{Name: "setup", Check: tt.check, Run: tt.run})
			got, err := form.BuildSubEntry()
			if (err != nil) != tt.wantErr {
				t.Fatalf("BuildSubEntry() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.Backup != "" || len(got.Targets) != 0 || len(got.Files) != 0 || got.Method != "" {
				t.Errorf("setup entry emitted config fields: %+v", got)
			}
		})
	}
}

func TestSubEntryForm_GetFieldType(t *testing.T) {
	tests := []struct {
		name       string
		focusIndex int
		isFolder   bool
		wantType   forms.SubEntryFieldType
	}{
		{
			name:       "index_0_is_name",
			focusIndex: 0,
			wantType:   forms.SubFieldName,
		},
		{
			name:       "index_2_is_when",
			focusIndex: 2,
			wantType:   forms.SubFieldWhen,
		},
		{
			name:       "index_3_is_linux",
			focusIndex: 3,
			wantType:   forms.SubFieldLinux,
		},
		{
			name:       "index_4_is_windows",
			focusIndex: 4,
			wantType:   forms.SubFieldWindows,
		},
		{
			name:       "index_5_is_backup",
			focusIndex: 5,
			wantType:   forms.SubFieldBackup,
		},
		{
			name:       "index_6_is_isFolder",
			focusIndex: 6,
			wantType:   forms.SubFieldIsFolder,
		},
		{
			name:       "index_7_in_folder_mode_is_sudo",
			focusIndex: 7,
			isFolder:   true,
			wantType:   forms.SubFieldIsSudo,
		},
		{
			name:       "index_7_in_files_mode_is_files",
			focusIndex: 7,
			isFolder:   false,
			wantType:   forms.SubFieldFiles,
		},
		{
			name:       "index_8_in_files_mode_is_sudo",
			focusIndex: 8,
			isFolder:   false,
			wantType:   forms.SubFieldIsSudo,
		},
		{
			name:       "out_of_range_defaults_to_name",
			focusIndex: 99,
			wantType:   forms.SubFieldName,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := forms.NewSubEntryForm(config.SubEntry{})
			form.FocusIndex = tt.focusIndex
			form.IsFolder = tt.isFolder

			got := form.GetFieldType()
			if got != tt.wantType {
				t.Errorf("GetFieldType() = %v, want %v", got, tt.wantType)
			}
		})
	}

	t.Run("nil_form_returns_name", func(t *testing.T) {
		var f *forms.SubEntryForm
		got := f.GetFieldType()
		if got != forms.SubFieldName {
			t.Errorf("GetFieldType() on nil = %v, want SubFieldName", got)
		}
	})

	t.Run("setup_indexes_keep_when_before_commands", func(t *testing.T) {
		form := forms.NewSubEntryForm(config.SubEntry{
			Name:  "setup",
			Check: map[string]string{"linux": "check"},
			Run:   map[string]string{"linux": "run"},
		})
		want := []forms.SubEntryFieldType{
			forms.SubFieldName, forms.SubFieldIsSetup, forms.SubFieldWhen,
			forms.SubFieldLinuxCheck, forms.SubFieldLinuxRun,
			forms.SubFieldWindowsCheck, forms.SubFieldWindowsRun,
			forms.SubFieldCheckMode, forms.SubFieldIsSudo,
		}
		if form.MaxIndex() != len(want)-1 {
			t.Fatalf("MaxIndex() = %d, want %d", form.MaxIndex(), len(want)-1)
		}
		for i, expected := range want {
			form.FocusIndex = i
			if got := form.GetFieldType(); got != expected {
				t.Errorf("GetFieldType() at %d = %v, want %v", i, got, expected)
			}
		}
	})
}

func TestSubEntryForm_MaxIndex(t *testing.T) {
	tests := []struct {
		name      string
		isFolder  bool
		wantIndex int
	}{
		{
			name:      "folder_mode_max_is_6",
			isFolder:  true,
			wantIndex: 7,
		},
		{
			name:      "files_mode_max_is_8",
			isFolder:  false,
			wantIndex: 9,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := forms.NewSubEntryForm(config.SubEntry{})
			form.IsFolder = tt.isFolder

			got := form.MaxIndex()
			if got != tt.wantIndex {
				t.Errorf("MaxIndex() = %d, want %d", got, tt.wantIndex)
			}
		})
	}

	t.Run("nil_form_returns_0", func(t *testing.T) {
		var f *forms.SubEntryForm
		got := f.MaxIndex()
		if got != 0 {
			t.Errorf("MaxIndex() on nil = %d, want 0", got)
		}
	})
}

func TestSubEntryForm_IsTextInputField(t *testing.T) {
	tests := []struct {
		name       string
		focusIndex int
		isFolder   bool
		isSetup    bool
		want       bool
	}{
		{name: "name_is_text_input", focusIndex: 0, want: true},
		{name: "when_is_text_input", focusIndex: 2, want: true},
		{name: "linux_is_text_input", focusIndex: 3, want: true},
		{name: "windows_is_text_input", focusIndex: 4, want: true},
		{name: "backup_is_text_input", focusIndex: 5, want: true},
		{name: "isFolder_is_not_text_input", focusIndex: 6, want: false},
		{name: "files_is_not_text_input", focusIndex: 7, isFolder: false, want: false},
		{name: "sudo_in_folder_mode_is_not_text_input", focusIndex: 7, isFolder: true, want: false},
		{name: "check_mode_is_not_text_input", focusIndex: 7, isSetup: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := forms.NewSubEntryForm(config.SubEntry{})
			form.FocusIndex = tt.focusIndex
			form.IsFolder = tt.isFolder
			form.IsSetup = tt.isSetup

			got := form.IsTextInputField()
			if got != tt.want {
				t.Errorf("IsTextInputField() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSubEntryForm_IsToggleField(t *testing.T) {
	tests := []struct {
		name       string
		focusIndex int
		isFolder   bool
		want       bool
	}{
		{name: "name_is_not_toggle", focusIndex: 0, want: false},
		{name: "when_is_not_toggle", focusIndex: 2, want: false},
		{name: "linux_is_not_toggle", focusIndex: 3, want: false},
		{name: "isFolder_is_toggle", focusIndex: 6, want: true},
		{name: "sudo_in_folder_mode_is_toggle", focusIndex: 7, isFolder: true, want: true},
		{name: "files_is_not_toggle", focusIndex: 7, isFolder: false, want: false},
		{name: "sudo_in_files_mode_is_toggle", focusIndex: 8, isFolder: false, want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			form := forms.NewSubEntryForm(config.SubEntry{})
			form.FocusIndex = tt.focusIndex
			form.IsFolder = tt.isFolder

			got := form.IsToggleField()
			if got != tt.want {
				t.Errorf("IsToggleField() = %v, want %v", got, tt.want)
			}
		})
	}

	setup := forms.NewSubEntryForm(config.SubEntry{
		Name:  "setup",
		Check: map[string]string{"linux": "check"},
		Run:   map[string]string{"linux": "run"},
	})
	setup.FocusIndex = 7
	if !setup.IsToggleField() {
		t.Error("check mode is not classified as a toggle field")
	}
}

func TestSubEntryForm_RoundTripsCheckAndRun(t *testing.T) {
	entry := config.SubEntry{
		Name:  "enable-service",
		Check: map[string]string{"linux": "systemctl --user is-enabled --quiet vicinae.service"},
		Run:   map[string]string{"linux": "systemctl --user enable --now vicinae.service"},
	}

	form := forms.NewSubEntryForm(entry)

	got, err := form.BuildSubEntry()
	if err != nil {
		t.Fatalf("BuildSubEntry() = %v, want no error", err)
	}

	if got.Check["linux"] != entry.Check["linux"] {
		t.Errorf("check survived as %v, want %v — the form dropped it, which deletes the setup step on save",
			got.Check, entry.Check)
	}

	if got.Run["linux"] != entry.Run["linux"] {
		t.Errorf("run survived as %v, want %v — the form dropped it, which deletes the setup step on save",
			got.Run, entry.Run)
	}
}
