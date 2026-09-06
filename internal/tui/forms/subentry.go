package forms

import (
	"errors"
	"maps"
	"strings"

	"charm.land/bubbles/v2/filepicker"
	"charm.land/bubbles/v2/textinput"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/tui/tuishared"
)

// SubEntryFieldType represents the type of field in the SubEntryForm
type SubEntryFieldType int

// SubEntryForm field type constants.
const (
	SubFieldName SubEntryFieldType = iota
	SubFieldIsSetup
	SubFieldWhen
	SubFieldLinux
	SubFieldWindows
	SubFieldLinuxCheck
	SubFieldLinuxRun
	SubFieldWindowsCheck
	SubFieldWindowsRun
	SubFieldBackup    // Config-specific
	SubFieldIsFolder  // Config-specific toggle
	SubFieldFiles     // Config-specific list
	SubFieldIsSudo    // Sudo toggle
	SubFieldIsCopy    // Deployment method toggle: copy instead of symlink
	SubFieldCheckMode // Setup-specific check exit-code interpretation
)

// AddFileMode represents the current mode for adding files to the files list
type AddFileMode int

const (
	// ModeNone indicates no file adding operation is active
	ModeNone AddFileMode = iota
	// ModeChoosing indicates user is choosing between browse/type options
	ModeChoosing
	// ModePicker indicates file picker is active for browsing
	ModePicker
	// ModeTextInput indicates manual text input mode is active
	ModeTextInput
)

// SubEntryForm holds state for editing SubEntry data
type SubEntryForm struct {
	Err            string
	SuccessMessage string
	OriginalValue  string
	Suggestions    []string
	Files          []string
	SelectedFiles  map[string]bool
	// Check and Run hold setup commands loaded into the editable command inputs.
	// They are also retained for form consumers that need the original maps.
	Check map[string]string
	Run   map[string]string
	// Method is the entry's deployment method as it was read in. IsCopy is what
	// the toggle edits; Method is kept so that turning the toggle off restores the
	// original spelling ("" or an explicit "symlink") rather than normalizing it.
	Method             string
	NameInput          textinput.Model
	WhenInput          textinput.Model
	LinuxTargetInput   textinput.Model
	WindowsTargetInput textinput.Model
	LinuxCheckInput    CommandInput
	LinuxRunInput      CommandInput
	WindowsCheckInput  CommandInput
	WindowsRunInput    CommandInput
	BackupInput        textinput.Model
	NewFileInput       textinput.Model
	FilePicker         filepicker.Model
	CheckMode          string
	EditingFileIndex   int
	TargetAppIdx       int
	EditSubIdx         int
	EditAppIdx         int
	FocusIndex         int
	FilesCursor        int
	SuggestionCursor   int
	ModeMenuCursor     int
	AddFileMode        AddFileMode
	IsFolder           bool
	IsSetup            bool
	ShowSuggestions    bool
	EditingField       bool
	EditingWhen        bool
	WhenMode           WhenMode
	HostnameCursor     int
	SelectedHostnames  map[string]bool
	AddingFile         bool
	EditingFile        bool
	IsSudo             bool
	IsCopy             bool
}

// GetFieldType returns the field type at the current focus index
func (f *SubEntryForm) GetFieldType() SubEntryFieldType {
	if f == nil {
		return SubFieldName
	}

	idx := f.FocusIndex

	// Common fields: name (0), entry type (1), when (2)
	switch idx {
	case 0:
		return SubFieldName
	case 1:
		return SubFieldIsSetup
	case 2:
		return SubFieldWhen
	}
	if f.IsSetup {
		switch idx {
		case 3:
			return SubFieldLinuxCheck
		case 4:
			return SubFieldLinuxRun
		case 5:
			return SubFieldWindowsCheck
		case 6:
			return SubFieldWindowsRun
		case 7:
			return SubFieldCheckMode
		case 8:
			return SubFieldIsSudo
		}
		return SubFieldName
	}
	if idx == 3 {
		return SubFieldLinux
	}
	if idx == 4 {
		return SubFieldWindows
	}

	// Config-specific fields start at index 5
	if f.IsFolder {
		// Folder mode: backup (5), isFolder (6), isSudo (7).
		// No copy toggle: copy mode is files-only (see ToggleFolderMode).
		switch idx {
		case 5:
			return SubFieldBackup
		case 6:
			return SubFieldIsFolder
		case 7:
			return SubFieldIsSudo
		}
	} else {
		// Files mode: backup (5), isFolder (6), files (7), isSudo (8), isCopy (9)
		switch idx {
		case 5:
			return SubFieldBackup
		case 6:
			return SubFieldIsFolder
		case 7:
			return SubFieldFiles
		case 8:
			return SubFieldIsSudo
		case 9:
			return SubFieldIsCopy
		}
	}

	// Fallback to name field if index is out of range
	return SubFieldName
}

// MaxIndex returns the maximum focus index based on state
func (f *SubEntryForm) MaxIndex() int {
	if f == nil {
		return 0
	}

	if f.IsSetup {
		return 8
	}

	// Common fields: name, entry type, when, linux, windows = 5 fields (0-4)
	// Config-specific fields start at 5
	if f.IsFolder {
		// Config folder: backup, isFolder, isSudo = 3 fields (5-7)
		return 7
	}

	// Config files: backup, isFolder, files, isSudo, isCopy = 5 fields (5-9)
	return 9
}

// ToggleFolderMode flips between folder and files mode.
//
// Copy mode is files-only: config validation rejects a copy entry with no files
// list, and config.Save does not validate, so a form left holding IsCopy in
// folder mode would write a tidydots.yaml that no longer loads. Switching into
// folder mode therefore clears the copy flag.
func (f *SubEntryForm) ToggleFolderMode() {
	if f == nil {
		return
	}

	f.IsFolder = !f.IsFolder
	if f.IsFolder {
		f.IsCopy = false
	}
}

// IsTextInputField returns true if the current field is a text input
func (f *SubEntryForm) IsTextInputField() bool {
	if f == nil {
		return false
	}

	ft := f.GetFieldType()
	switch ft {
	case SubFieldName, SubFieldWhen, SubFieldLinux, SubFieldWindows, SubFieldBackup,
		SubFieldLinuxCheck, SubFieldLinuxRun, SubFieldWindowsCheck, SubFieldWindowsRun:
		return true
	case SubFieldIsSetup, SubFieldIsFolder, SubFieldFiles, SubFieldIsSudo, SubFieldIsCopy, SubFieldCheckMode:
		// These fields don't have suggestions
	}

	return false
}

// IsToggleField returns true if the current field is a toggle
func (f *SubEntryForm) IsToggleField() bool {
	if f == nil {
		return false
	}

	ft := f.GetFieldType()

	return ft == SubFieldIsSetup || ft == SubFieldIsFolder || ft == SubFieldIsSudo || ft == SubFieldIsCopy || ft == SubFieldCheckMode
}

// UpdateFocus updates which input field is focused
func (f *SubEntryForm) UpdateFocus() {
	if f == nil {
		return
	}

	f.NameInput.Blur()
	f.WhenInput.Blur()
	f.LinuxTargetInput.Blur()
	f.WindowsTargetInput.Blur()
	f.LinuxCheckInput.Blur()
	f.LinuxRunInput.Blur()
	f.WindowsCheckInput.Blur()
	f.WindowsRunInput.Blur()
	f.BackupInput.Blur()
	f.NewFileInput.Blur()

	ft := f.GetFieldType()
	switch ft {
	case SubFieldName:
		f.NameInput.Focus()
	case SubFieldWhen:
		f.WhenInput.Focus()
	case SubFieldLinux:
		f.LinuxTargetInput.Focus()
	case SubFieldWindows:
		f.WindowsTargetInput.Focus()
	case SubFieldLinuxCheck:
		f.LinuxCheckInput.Focus()
	case SubFieldLinuxRun:
		f.LinuxRunInput.Focus()
	case SubFieldWindowsCheck:
		f.WindowsCheckInput.Focus()
	case SubFieldWindowsRun:
		f.WindowsRunInput.Focus()
	case SubFieldBackup:
		f.BackupInput.Focus()
	case SubFieldIsFolder, SubFieldFiles, SubFieldIsSudo, SubFieldIsCopy, SubFieldCheckMode:
		// Boolean and list fields don't use text input focus
	}
}

// EnterFieldEditMode enters edit mode for the current text field
func (f *SubEntryForm) EnterFieldEditMode() {
	if f == nil {
		return
	}

	f.EditingField = true
	ft := f.GetFieldType()

	switch ft {
	case SubFieldName:
		f.OriginalValue = f.NameInput.Value()
		f.NameInput.Focus()
		f.NameInput.SetCursor(len(f.NameInput.Value()))
	case SubFieldWhen:
		f.OriginalValue = f.WhenInput.Value()
		f.WhenInput.Focus()
		f.WhenInput.SetCursor(len(f.WhenInput.Value()))
	case SubFieldLinux:
		f.OriginalValue = f.LinuxTargetInput.Value()
		f.LinuxTargetInput.Focus()
		f.LinuxTargetInput.SetCursor(len(f.LinuxTargetInput.Value()))
	case SubFieldWindows:
		f.OriginalValue = f.WindowsTargetInput.Value()
		f.WindowsTargetInput.Focus()
		f.WindowsTargetInput.SetCursor(len(f.WindowsTargetInput.Value()))
	case SubFieldLinuxCheck:
		f.OriginalValue = f.LinuxCheckInput.Value()
		f.LinuxCheckInput.Focus()
		f.LinuxCheckInput.MoveCursorToEnd()
	case SubFieldLinuxRun:
		f.OriginalValue = f.LinuxRunInput.Value()
		f.LinuxRunInput.Focus()
		f.LinuxRunInput.MoveCursorToEnd()
	case SubFieldWindowsCheck:
		f.OriginalValue = f.WindowsCheckInput.Value()
		f.WindowsCheckInput.Focus()
		f.WindowsCheckInput.MoveCursorToEnd()
	case SubFieldWindowsRun:
		f.OriginalValue = f.WindowsRunInput.Value()
		f.WindowsRunInput.Focus()
		f.WindowsRunInput.MoveCursorToEnd()
	case SubFieldBackup:
		f.OriginalValue = f.BackupInput.Value()
		f.BackupInput.Focus()
		f.BackupInput.SetCursor(len(f.BackupInput.Value()))
	case SubFieldIsFolder, SubFieldFiles, SubFieldIsSudo, SubFieldIsCopy, SubFieldCheckMode:
		// Boolean and list fields don't use text input editing
	}
}

// CancelFieldEdit cancels editing and restores the original value
func (f *SubEntryForm) CancelFieldEdit() {
	if f == nil {
		return
	}

	ft := f.GetFieldType()
	switch ft {
	case SubFieldName:
		f.NameInput.SetValue(f.OriginalValue)
	case SubFieldWhen:
		f.WhenInput.SetValue(f.OriginalValue)
	case SubFieldLinux:
		f.LinuxTargetInput.SetValue(f.OriginalValue)
	case SubFieldWindows:
		f.WindowsTargetInput.SetValue(f.OriginalValue)
	case SubFieldLinuxCheck:
		f.LinuxCheckInput.SetValue(f.OriginalValue)
	case SubFieldLinuxRun:
		f.LinuxRunInput.SetValue(f.OriginalValue)
	case SubFieldWindowsCheck:
		f.WindowsCheckInput.SetValue(f.OriginalValue)
	case SubFieldWindowsRun:
		f.WindowsRunInput.SetValue(f.OriginalValue)
	case SubFieldBackup:
		f.BackupInput.SetValue(f.OriginalValue)
	case SubFieldIsFolder, SubFieldFiles, SubFieldIsSudo, SubFieldIsCopy, SubFieldCheckMode:
		// Boolean and list fields don't use text input restoration
	}

	f.EditingField = false
	f.ShowSuggestions = false
	f.Err = ""
	f.UpdateFocus()
}

// Validate checks if the SubEntryForm has valid data
func (f *SubEntryForm) Validate() error {
	if strings.TrimSpace(f.NameInput.Value()) == "" {
		return errors.New("entry name is required")
	}
	if f.IsSetup {
		_, err := f.buildSetupEntry(strings.TrimSpace(f.NameInput.Value()))
		return err
	}

	if strings.TrimSpace(f.BackupInput.Value()) == "" {
		return errors.New("backup path is required")
	}

	// Check if at least one target is specified
	hasTarget := strings.TrimSpace(f.LinuxTargetInput.Value()) != "" ||
		strings.TrimSpace(f.WindowsTargetInput.Value()) != ""

	if !hasTarget {
		return errors.New("at least one target is required")
	}

	return nil
}

func (f *SubEntryForm) buildSetupEntry(name string) (config.SubEntry, error) {
	switch f.CheckMode {
	case "", config.CheckModeExitCode, config.CheckModeStatus:
	default:
		return config.SubEntry{}, errors.New("check mode must be \"exit-code\" or \"status\"")
	}

	check := make(map[string]string)
	run := make(map[string]string)
	fields := []struct {
		osName string
		check  CommandInput
		run    CommandInput
	}{
		{osName: "linux", check: f.LinuxCheckInput, run: f.LinuxRunInput},
		{osName: "windows", check: f.WindowsCheckInput, run: f.WindowsRunInput},
	}
	for _, field := range fields {
		checkValue := field.check.Value()
		runValue := field.run.Value()
		if strings.TrimSpace(checkValue) == "" && strings.TrimSpace(runValue) == "" {
			continue
		}
		if strings.TrimSpace(checkValue) == "" || strings.TrimSpace(runValue) == "" {
			return config.SubEntry{}, errors.New("setup check and run commands must be provided together for each OS")
		}
		check[field.osName] = checkValue
		run[field.osName] = runValue
	}
	if len(run) == 0 {
		return config.SubEntry{}, errors.New("at least one OS must have both setup check and run commands")
	}
	return config.SubEntry{
		Name:      name,
		When:      strings.TrimSpace(f.WhenInput.Value()),
		Check:     check,
		Run:       run,
		CheckMode: f.CheckMode,
		Sudo:      f.IsSudo,
	}, nil
}

// buildMethod resolves the toggle back to a method string. Turning copy off
// restores the method the entry came in with, so an explicit "symlink" survives
// a round-trip and an absent method stays absent — symlink is the default, and
// writing it out would churn every file the form touches.
func (f *SubEntryForm) buildMethod() string {
	if f.IsCopy {
		return config.MethodCopy
	}

	if f.Method == config.MethodCopy {
		return ""
	}

	return f.Method
}

// BuildSubEntry validates and returns the SubEntry from the form, or an error.
func (f *SubEntryForm) BuildSubEntry() (config.SubEntry, error) {
	if f == nil {
		return config.SubEntry{}, errors.New("no form data")
	}

	name := strings.TrimSpace(f.NameInput.Value())
	// Validation
	if name == "" {
		return config.SubEntry{}, errors.New("name is required")
	}
	if f.IsSetup {
		return f.buildSetupEntry(name)
	}

	targets := BuildTargetsFromInputs(f.LinuxTargetInput, f.WindowsTargetInput)

	if len(targets) == 0 {
		return config.SubEntry{}, errors.New("at least one target is required")
	}

	backup := strings.TrimSpace(f.BackupInput.Value())
	if backup == "" {
		return config.SubEntry{}, errors.New("backup path is required")
	}

	// Build a config entry from the config-specific form fields. Setup commands are
	// built separately above and never appear in this branch.
	subEntry := config.SubEntry{
		Name:    name,
		When:    strings.TrimSpace(f.WhenInput.Value()),
		Targets: targets,
		Sudo:    f.IsSudo,
		Method:  f.buildMethod(),
		Backup:  backup,
	}

	// Add files if in files mode
	if !f.IsFolder {
		if len(f.Files) == 0 {
			return config.SubEntry{}, errors.New("at least one file is required when using Files mode")
		}
		subEntry.Files = make([]string, len(f.Files))
		copy(subEntry.Files, f.Files)
	}

	// Copy mode is files-only: ValidateConfig rejects a copy entry with no files
	// list, and config.Save does not validate. Emitting one would write a
	// tidydots.yaml that no longer loads, so refuse instead. This asserts the rule
	// itself rather than the two ways the UI can reach it (folder mode, or a files
	// list emptied one entry at a time), so it still holds if either changes.
	if subEntry.Method == config.MethodCopy && len(subEntry.Files) == 0 {
		return config.SubEntry{}, errors.New("copy mode requires a files list")
	}

	return subEntry, nil
}

// NewSubEntryForm creates a new SubEntryForm for testing purposes
func NewSubEntryForm(entry config.SubEntry) *SubEntryForm {
	nameInput := NewFormInput("e.g., nvim-config", tuishared.CharLimitName, tuishared.InputWidthNarrow)
	nameInput.SetValue(entry.Name)
	whenInput := NewFormInput(tuishared.PlaceholderWhen, 0, tuishared.InputWidthNarrow)
	whenInput.SetValue(entry.When)

	linuxTargetInput := NewFormInput("e.g., ~/.config/nvim", tuishared.CharLimitPath, tuishared.InputWidthNarrow)
	if target, ok := entry.Targets["linux"]; ok {
		linuxTargetInput.SetValue(target)
	}

	windowsTargetInput := NewFormInput("e.g., ~/AppData/Local/nvim", tuishared.CharLimitPath, tuishared.InputWidthNarrow)
	if target, ok := entry.Targets["windows"]; ok {
		windowsTargetInput.SetValue(target)
	}

	backupInput := NewFormInput("e.g., ./nvim", tuishared.CharLimitPath, tuishared.InputWidthNarrow)
	backupInput.SetValue(entry.Backup)
	linuxCheckInput := NewCommandInput("e.g., command -v foo", tuishared.InputWidthNarrow)
	linuxRunInput := NewCommandInput("e.g., install foo", tuishared.InputWidthNarrow)
	windowsCheckInput := NewCommandInput("e.g., where foo", tuishared.InputWidthNarrow)
	windowsRunInput := NewCommandInput("e.g., install foo", tuishared.InputWidthNarrow)
	linuxCheckInput.SetValue(entry.Check["linux"])
	linuxRunInput.SetValue(entry.Run["linux"])
	windowsCheckInput.SetValue(entry.Check["windows"])
	windowsRunInput.SetValue(entry.Run["windows"])

	return &SubEntryForm{
		NameInput:          nameInput,
		WhenInput:          whenInput,
		LinuxTargetInput:   linuxTargetInput,
		WindowsTargetInput: windowsTargetInput,
		LinuxCheckInput:    linuxCheckInput,
		LinuxRunInput:      linuxRunInput,
		WindowsCheckInput:  windowsCheckInput,
		WindowsRunInput:    windowsRunInput,
		BackupInput:        backupInput,
		CheckMode:          entry.CheckMode,
		IsSudo:             entry.Sudo,
		IsCopy:             entry.IsCopy(),
		Method:             entry.Method,
		IsFolder:           entry.IsFolder(),
		IsSetup:            entry.IsSetup(),
		Files:              entry.Files,
		Check:              maps.Clone(entry.Check),
		Run:                maps.Clone(entry.Run),
	}
}
