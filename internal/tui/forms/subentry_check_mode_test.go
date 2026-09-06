package forms_test

import (
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/tui/forms"
)

func TestSetupCheckModeFormRoundTrip(t *testing.T) {
	for _, mode := range []string{"", "exit-code", "status"} {
		entry := config.SubEntry{
			Name:      "binary",
			CheckMode: mode,
			Check:     map[string]string{"linux": "check"},
			Run:       map[string]string{"linux": "run"},
		}
		form := forms.NewSubEntryForm(entry)
		got, err := form.BuildSubEntry()
		if err != nil {
			t.Fatal(err)
		}
		if got.CheckMode != mode {
			t.Fatalf("got %q, want %q", got.CheckMode, mode)
		}
		if got := form.MaxIndex(); got != 8 {
			t.Fatalf("setup MaxIndex() = %d, want 8", got)
		}

		found := false
		for i := 0; i <= form.MaxIndex(); i++ {
			form.FocusIndex = i
			if form.GetFieldType() == forms.SubFieldCheckMode {
				found = true
			}
		}
		if !found {
			t.Fatal("check-mode selector is unreachable")
		}

		form.ToggleCheckMode()
		want := config.CheckModeStatus
		if mode == config.CheckModeStatus {
			want = config.CheckModeExitCode
		}
		if form.CheckMode != want {
			t.Fatalf("toggle = %q, want %q", form.CheckMode, want)
		}
	}
}

func TestToggleCheckModeIgnoresConfigForms(t *testing.T) {
	form := forms.NewSubEntryForm(config.SubEntry{
		Name:      "config",
		CheckMode: config.CheckModeStatus,
		Backup:    "./config",
		Targets:   map[string]string{"linux": "~/.config/config"},
	})

	form.ToggleCheckMode()
	if form.CheckMode != config.CheckModeStatus {
		t.Fatalf("config ToggleCheckMode() changed CheckMode to %q", form.CheckMode)
	}
}

func TestConfigCheckModeIsHiddenAndOmitted(t *testing.T) {
	form := forms.NewSubEntryForm(config.SubEntry{
		Name:    "config",
		Backup:  "./config",
		Targets: map[string]string{"linux": "~/.config/config"},
	})
	form.CheckMode = config.CheckModeStatus

	for i := 0; i <= form.MaxIndex(); i++ {
		form.FocusIndex = i
		if form.GetFieldType() == forms.SubFieldCheckMode {
			t.Fatalf("config focus index %d exposes the setup-only check-mode selector", i)
		}
	}

	got, err := form.BuildSubEntry()
	if err != nil {
		t.Fatalf("BuildSubEntry() error = %v", err)
	}
	if got.CheckMode != "" {
		t.Fatalf("config BuildSubEntry() emitted stale setup CheckMode %q", got.CheckMode)
	}
}

func TestSetupCheckModeRejectsUnknownValues(t *testing.T) {
	form := forms.NewSubEntryForm(config.SubEntry{
		Name:      "binary",
		CheckMode: "unknown",
		Check:     map[string]string{"linux": "check"},
		Run:       map[string]string{"linux": "run"},
	})

	if err := form.Validate(); err == nil {
		t.Fatal("Validate() accepted an unknown setup check mode")
	}
	if _, err := form.BuildSubEntry(); err == nil {
		t.Fatal("BuildSubEntry() accepted an unknown setup check mode")
	}
}
