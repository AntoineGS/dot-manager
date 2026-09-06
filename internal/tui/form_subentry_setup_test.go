package tui

import (
	"testing"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"github.com/AntoineGS/tidydots/internal/config"
)

// setupSubItemIndex returns the index of the application's setup sub-entry row.
func setupSubItemIndex(t *testing.T, m *Model) int {
	t.Helper()

	for i := range m.Applications[0].SubItems {
		if m.Applications[0].SubItems[i].SubEntry.IsSetup() {
			return i
		}
	}

	t.Fatalf("application %q has no setup sub-entry", m.Applications[0].Application.Name)

	return -1
}

func TestInitSubEntryForm_SetupEntry_OpensEditableForm(t *testing.T) {
	entry := setupSubEntry()
	entry.Sudo = true
	cfg := setupOnlyConfig(entry)
	m := NewModel(cfg, linuxPlatform(), false)

	subIdx := setupSubItemIndex(t, &m)

	m.initSubEntryForm(0, subIdx)

	if m.subEntryForm == nil {
		t.Fatal("the form did not open on a setup entry")
	}
	form := m.subEntryForm
	if !form.IsSetup {
		t.Error("IsSetup = false, want true")
	}
	if got := form.LinuxCheckInput.Value(); got != entry.Check["linux"] {
		t.Errorf("LinuxCheckInput = %q, want %q", got, entry.Check["linux"])
	}
	if got := form.LinuxRunInput.Value(); got != entry.Run["linux"] {
		t.Errorf("LinuxRunInput = %q, want %q", got, entry.Run["linux"])
	}
	if got := form.WindowsCheckInput.Value(); got != entry.Check["windows"] {
		t.Errorf("WindowsCheckInput = %q, want %q", got, entry.Check["windows"])
	}
	if got := form.WindowsRunInput.Value(); got != entry.Run["windows"] {
		t.Errorf("WindowsRunInput = %q, want %q", got, entry.Run["windows"])
	}
	if !form.IsSudo {
		t.Error("IsSudo = false, want true")
	}
}

func TestSetupCheckModeSelectorEditsAndSaves(t *testing.T) {
	entry := setupSubEntry()
	entry.CheckMode = config.CheckModeStatus
	m, path := modelOnDisk(t, setupOnlyConfig(entry))

	subIdx := setupSubItemIndex(t, m)
	m.initSubEntryForm(0, subIdx)
	if m.subEntryForm == nil {
		t.Fatal("the setup form did not open")
	}

	form := m.subEntryForm
	if form.CheckMode != config.CheckModeStatus {
		t.Fatalf("CheckMode = %q, want %q", form.CheckMode, config.CheckModeStatus)
	}
	form.FocusIndex = 7
	m.updateSubEntryFormFocus()
	if got := m.getSubEntryFieldType(); got != subFieldCheckMode {
		t.Fatalf("focus index 7 maps to %v, want check-mode selector", got)
	}
	if form.EditingField {
		t.Fatal("check-mode selector starts in text-edit mode")
	}

	update := func(msg tea.KeyPressMsg) {
		t.Helper()
		updated, _ := m.updateSubEntryForm(msg)
		updatedModel := updated.(Model)
		m = &updatedModel
	}

	update(tea.KeyPressMsg{Code: tea.KeySpace})
	if form.CheckMode != config.CheckModeExitCode {
		t.Fatalf("space toggled CheckMode to %q, want %q", form.CheckMode, config.CheckModeExitCode)
	}
	if form.EditingField {
		t.Fatal("space entered text-edit mode for check mode")
	}

	update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if form.CheckMode != config.CheckModeStatus {
		t.Fatalf("enter toggled CheckMode to %q, want %q", form.CheckMode, config.CheckModeStatus)
	}
	if form.EditingField {
		t.Fatal("enter entered text-edit mode for check mode")
	}

	update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if form.CheckMode != config.CheckModeExitCode {
		t.Fatalf("left selected CheckMode %q, want %q", form.CheckMode, config.CheckModeExitCode)
	}
	update(tea.KeyPressMsg{Code: 'h', Text: "h"})
	if form.CheckMode != config.CheckModeExitCode {
		t.Fatalf("h selected CheckMode %q, want %q", form.CheckMode, config.CheckModeExitCode)
	}
	update(tea.KeyPressMsg{Code: tea.KeyRight})
	if form.CheckMode != config.CheckModeStatus {
		t.Fatalf("right selected CheckMode %q, want %q", form.CheckMode, config.CheckModeStatus)
	}
	update(tea.KeyPressMsg{Code: 'l', Text: "l"})
	if form.CheckMode != config.CheckModeStatus {
		t.Fatalf("l selected CheckMode %q, want %q", form.CheckMode, config.CheckModeStatus)
	}

	update(tea.KeyPressMsg{Code: 'e', Text: "e"})
	if form.CheckMode != config.CheckModeExitCode {
		t.Fatalf("e toggled CheckMode to %q, want %q", form.CheckMode, config.CheckModeExitCode)
	}
	if form.EditingField {
		t.Fatal("e entered text-edit mode for check mode")
	}
	update(tea.KeyPressMsg{Code: tea.KeyRight})

	update(tea.KeyPressMsg{Code: 's', Text: "s"})
	if m.subEntryForm != nil {
		t.Fatal("saving the setup form left it open")
	}

	saved, err := config.Load(path)
	if err != nil {
		t.Fatalf("reloading config: %v", err)
	}
	if got := saved.Applications[0].Entries[0].CheckMode; got != config.CheckModeStatus {
		t.Fatalf("saved CheckMode = %q, want %q", got, config.CheckModeStatus)
	}
}

func TestSetupCheckModeSelectorRendersBeforeSudo(t *testing.T) {
	entry := setupSubEntry()
	entry.CheckMode = config.CheckModeStatus
	m := NewModel(setupOnlyConfig(entry), linuxPlatform(), false)
	subIdx := setupSubItemIndex(t, &m)
	m.initSubEntryForm(0, subIdx)
	m.subEntryForm.FocusIndex = 7
	m.updateSubEntryFormFocus()

	view := m.viewSubEntryForm()
	checkModePos := indexOfText(t, view, "Check mode:")
	sudoPos := indexOfText(t, view, "Root only:")
	if checkModePos >= sudoPos {
		t.Fatalf("setup view places check mode at %d and sudo at %d", checkModePos, sudoPos)
	}
	if !containsText(view, "status") {
		t.Fatalf("setup view does not render the selected status check mode: %q", view)
	}
	help := m.renderSubEntryFormHelp()
	for _, text := range []string{"exit-code", "status", "toggle"} {
		if !containsText(help, text) {
			t.Fatalf("check-mode help does not contain %q: %q", text, help)
		}
	}
}

func TestSetupCheckModeSelectorBindingAdvertisesSpaceAndEnter(t *testing.T) {
	for _, msg := range []tea.KeyPressMsg{
		{Code: tea.KeySpace},
		{Code: tea.KeyEnter},
	} {
		if !key.Matches(msg, subEntryCheckModeToggleKey) {
			t.Errorf("check-mode toggle binding does not match %q", msg)
		}
	}

	entry := setupSubEntry()
	entry.CheckMode = config.CheckModeStatus
	m := NewModel(setupOnlyConfig(entry), linuxPlatform(), false)
	subIdx := setupSubItemIndex(t, &m)
	m.initSubEntryForm(0, subIdx)
	m.subEntryForm.FocusIndex = 7
	m.updateSubEntryFormFocus()
	help := m.renderSubEntryFormHelp()
	if !containsText(help, "space/enter") {
		t.Fatalf("check-mode help = %q, want space/enter binding", help)
	}
}

func indexOfText(t *testing.T, text, needle string) int {
	t.Helper()
	for i := 0; i+len(needle) <= len(text); i++ {
		if text[i:i+len(needle)] == needle {
			return i
		}
	}
	t.Fatalf("view does not contain %q: %q", needle, text)
	return -1
}

func containsText(text, needle string) bool {
	for i := 0; i+len(needle) <= len(text); i++ {
		if text[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}

// TestInitSubEntryForm_ConfigEntry_StillOpens proves the guard is narrow: config
// entries are still editable.
func TestSaveSubEntryForm_SetupEntry_RoundTripsEdits(t *testing.T) {
	entry := setupSubEntry()
	entry.Sudo = true
	cfg := setupOnlyConfig(entry)
	m, path := modelOnDisk(t, cfg)
	subIdx := setupSubItemIndex(t, m)
	m.initSubEntryForm(0, subIdx)
	form := m.subEntryForm
	form.NameInput.SetValue("edited-setup")
	form.LinuxCheckInput.SetValue("test -x /usr/bin/edited")
	form.LinuxRunInput.SetValue("install-edited")
	form.WindowsCheckInput.SetValue("where edited")
	form.WindowsRunInput.SetValue("install-edited.exe")

	if err := m.saveSubEntryForm(); err != nil {
		t.Fatalf("saveSubEntryForm() error = %v", err)
	}

	saved, err := config.Load(path)
	if err != nil {
		t.Fatalf("reloading config: %v", err)
	}
	got := saved.Applications[0].Entries[0]
	if got.Name != "edited-setup" || got.Backup != "" || len(got.Targets) != 0 || len(got.Files) != 0 || got.Method != "" {
		t.Fatalf("saved setup entry has config fields: %+v", got)
	}
	if got.Check["linux"] != "test -x /usr/bin/edited" || got.Run["linux"] != "install-edited" {
		t.Errorf("saved Linux commands = %v/%v", got.Check["linux"], got.Run["linux"])
	}
	if got.Check["windows"] != "where edited" || got.Run["windows"] != "install-edited.exe" {
		t.Errorf("saved Windows commands = %v/%v", got.Check["windows"], got.Run["windows"])
	}
	if !got.Sudo {
		t.Error("saved Sudo = false, want true")
	}
}

func TestInitSubEntryForm_ConfigEntry_StillOpens(t *testing.T) {
	cfg := setupOnlyConfig(configSubEntry())
	m := NewModel(cfg, linuxPlatform(), false)

	m.initSubEntryForm(0, 0)

	if m.subEntryForm == nil {
		t.Fatal("the form did not open on a config entry")
	}

	if m.subEntryForm.NameInput.Value() != "config-file" {
		t.Errorf("form opened on %q, want %q", m.subEntryForm.NameInput.Value(), "config-file")
	}

	if m.Screen != ScreenAddForm {
		t.Errorf("Screen = %v, want ScreenAddForm", m.Screen)
	}
}
