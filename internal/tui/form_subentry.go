package tui

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// subEntryFieldType represents the type of field in the SubEntryForm
type subEntryFieldType int

const (
	subFieldName subEntryFieldType = iota
	subFieldLinux
	subFieldWindows
	subFieldBackup   // Config-specific
	subFieldIsFolder // Config-specific toggle
	subFieldFiles    // Config-specific list
	subFieldIsSudo   // Sudo toggle
)

// initSubEntryFormNew initializes the form for adding a new sub-entry to an existing application
func (m *Model) initSubEntryFormNew(appIdx int) {
	nameInput := textinput.New()
	nameInput.Placeholder = "e.g., nvim-config"
	nameInput.Focus()
	nameInput.CharLimit = 64
	nameInput.Width = 40

	linuxTargetInput := textinput.New()
	linuxTargetInput.Placeholder = "e.g., ~/.config/nvim"
	linuxTargetInput.CharLimit = 256
	linuxTargetInput.Width = 40

	windowsTargetInput := textinput.New()
	windowsTargetInput.Placeholder = "e.g., ~/AppData/Local/nvim"
	windowsTargetInput.CharLimit = 256
	windowsTargetInput.Width = 40

	backupInput := textinput.New()
	backupInput.Placeholder = "e.g., ./nvim"
	backupInput.CharLimit = 256
	backupInput.Width = 40

	newFileInput := textinput.New()
	newFileInput.Placeholder = "e.g., .bashrc"
	newFileInput.CharLimit = 256
	newFileInput.Width = 40

	m.subEntryForm = &SubEntryForm{
		nameInput:          nameInput,
		linuxTargetInput:   linuxTargetInput,
		windowsTargetInput: windowsTargetInput,
		isSudo:             false,
		backupInput:        backupInput,
		isFolder:           true,
		files:              nil,
		filesCursor:        0,
		newFileInput:       newFileInput,
		addingFile:         false,
		editingFile:        false,
		editingFileIndex:   -1,
		focusIndex:         0,
		editingField:       false,
		originalValue:      "",
		suggestions:        nil,
		suggestionCursor:   -1,
		showSuggestions:    false,
		targetAppIdx:       appIdx,
		editAppIdx:         -1,
		editSubIdx:         -1,
		err:                "",
	}

	m.activeForm = FormSubEntry
	m.Screen = ScreenAddForm
}

// initSubEntryFormEdit initializes the form for editing an existing sub-entry
func (m *Model) initSubEntryFormEdit(appIdx, subIdx int) {
	// appIdx is an index into m.Applications (sorted), not m.Config.Applications (unsorted)
	// We need to find the correct index in m.Config.Applications by application name
	if appIdx < 0 || appIdx >= len(m.Applications) {
		return
	}

	appName := m.Applications[appIdx].Application.Name
	configAppIdx := m.findConfigApplicationIndex(appName)
	if configAppIdx < 0 {
		return
	}

	app := m.Config.Applications[configAppIdx]

	// subIdx is an index into m.Applications[appIdx].SubItems, which may be filtered
	// We need to find the correct index in app.Entries by sub-entry name
	if subIdx < 0 || subIdx >= len(m.Applications[appIdx].SubItems) {
		return
	}

	subEntryName := m.Applications[appIdx].SubItems[subIdx].SubEntry.Name
	configSubIdx := -1
	for i, entry := range app.Entries {
		if entry.Name == subEntryName {
			configSubIdx = i
			break
		}
	}

	if configSubIdx < 0 {
		return
	}

	sub := app.Entries[configSubIdx]

	nameInput := textinput.New()
	nameInput.Placeholder = "e.g., nvim-config"
	nameInput.SetValue(sub.Name)
	nameInput.Focus()
	nameInput.CharLimit = 64
	nameInput.Width = 40

	linuxTargetInput := textinput.New()
	linuxTargetInput.Placeholder = "e.g., ~/.config/nvim"

	if target, ok := sub.Targets["linux"]; ok {
		linuxTargetInput.SetValue(target)
	}
	linuxTargetInput.CharLimit = 256
	linuxTargetInput.Width = 40

	windowsTargetInput := textinput.New()
	windowsTargetInput.Placeholder = "e.g., ~/AppData/Local/nvim"

	if target, ok := sub.Targets["windows"]; ok {
		windowsTargetInput.SetValue(target)
	}
	windowsTargetInput.CharLimit = 256
	windowsTargetInput.Width = 40

	backupInput := textinput.New()
	backupInput.Placeholder = "e.g., ./nvim"
	backupInput.SetValue(sub.Backup)
	backupInput.CharLimit = 256
	backupInput.Width = 40

	newFileInput := textinput.New()
	newFileInput.Placeholder = "e.g., .bashrc"
	newFileInput.CharLimit = 256
	newFileInput.Width = 40

	// Load config-specific fields
	isFolder := sub.IsFolder()
	var files []string

	if !isFolder && len(sub.Files) > 0 {
		files = make([]string, len(sub.Files))
		copy(files, sub.Files)
	}

	m.subEntryForm = &SubEntryForm{
		nameInput:          nameInput,
		linuxTargetInput:   linuxTargetInput,
		windowsTargetInput: windowsTargetInput,
		isSudo:             sub.Sudo,
		backupInput:        backupInput,
		isFolder:           isFolder,
		files:              files,
		filesCursor:        0,
		newFileInput:       newFileInput,
		addingFile:         false,
		editingFile:        false,
		editingFileIndex:   -1,
		focusIndex:         0,
		editingField:       false,
		originalValue:      "",
		suggestions:        nil,
		suggestionCursor:   -1,
		showSuggestions:    false,
		targetAppIdx:       -1,
		editAppIdx:         configAppIdx,
		editSubIdx:         configSubIdx,
		err:                "",
	}

	m.activeForm = FormSubEntry
	m.Screen = ScreenAddForm
}

// getSubEntryFieldType returns the field type at the current focus index
func (m *Model) getSubEntryFieldType() subEntryFieldType {
	if m.subEntryForm == nil {
		return subFieldName
	}

	idx := m.subEntryForm.focusIndex

	// Common fields: name (0), linux (1), windows (2)
	switch idx {
	case 0:
		return subFieldName
	case 1:
		return subFieldLinux
	case 2:
		return subFieldWindows
	}

	// Config-specific fields start at index 3
	if m.subEntryForm.isFolder {
		// Folder mode: backup (3), isFolder (4), isSudo (5)
		switch idx {
		case 3:
			return subFieldBackup
		case 4:
			return subFieldIsFolder
		case 5:
			return subFieldIsSudo
		}
	} else {
		// Files mode: backup (3), isFolder (4), files (5), isSudo (6)
		switch idx {
		case 3:
			return subFieldBackup
		case 4:
			return subFieldIsFolder
		case 5:
			return subFieldFiles
		case 6:
			return subFieldIsSudo
		}
	}

	// Fallback to name field if index is out of range
	return subFieldName
}

// subEntryFormMaxIndex returns the maximum focus index based on state
func (m *Model) subEntryFormMaxIndex() int {
	if m.subEntryForm == nil {
		return 0
	}

	// Common fields: name, linux, windows = 3 fields (0-2)
	// Config-specific fields start at 3
	if m.subEntryForm.isFolder {
		// Config folder: backup, isFolder, isSudo = 3 fields (3-5)
		return 5
	}

	// Config files: backup, isFolder, files, isSudo = 4 fields (3-6)
	return 6
}

// updateSubEntryForm handles key events for the sub-entry form
func (m Model) updateSubEntryForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.subEntryForm == nil {
		return m, nil
	}

	// Handle editing a text field
	if m.subEntryForm.editingField {
		return m.updateSubEntryFieldInput(msg)
	}

	// Handle adding/editing file mode
	if m.subEntryForm.addingFile || m.subEntryForm.editingFile {
		return m.updateSubEntryFileInput(msg)
	}

	// Handle files list navigation
	if m.getSubEntryFieldType() == subFieldFiles {
		return m.updateSubEntryFilesList(msg)
	}

	switch msg.String() {
	case KeyCtrlC:
		return m, tea.Quit

	case "q", KeyEsc:
		// Return to list view
		m.activeForm = FormNone
		m.subEntryForm = nil
		m.Screen = ScreenResults

		return m, nil

	case KeyDown, "j":
		m.subEntryForm.focusIndex++

		maxIndex := m.subEntryFormMaxIndex()
		if m.subEntryForm.focusIndex > maxIndex {
			m.subEntryForm.focusIndex = 0
		}

		m.updateSubEntryFormFocus()

		return m, nil

	case "up", "k":
		m.subEntryForm.focusIndex--
		if m.subEntryForm.focusIndex < 0 {
			m.subEntryForm.focusIndex = m.subEntryFormMaxIndex()
		}

		m.updateSubEntryFormFocus()

		return m, nil

	case KeyTab:
		m.subEntryForm.focusIndex++

		maxIndex := m.subEntryFormMaxIndex()
		if m.subEntryForm.focusIndex > maxIndex {
			m.subEntryForm.focusIndex = 0
		}

		m.updateSubEntryFormFocus()

		return m, nil

	case KeyShiftTab:
		m.subEntryForm.focusIndex--
		if m.subEntryForm.focusIndex < 0 {
			m.subEntryForm.focusIndex = m.subEntryFormMaxIndex()
		}

		m.updateSubEntryFormFocus()

		return m, nil

	case " ":
		// Handle toggles
		ft := m.getSubEntryFieldType()
		switch ft {
		case subFieldIsFolder:
			m.subEntryForm.isFolder = !m.subEntryForm.isFolder
			return m, nil
		case subFieldIsSudo:
			m.subEntryForm.isSudo = !m.subEntryForm.isSudo
			return m, nil
		case subFieldName, subFieldLinux, subFieldWindows, subFieldBackup, subFieldFiles:
			// Text and list fields don't toggle
		}

	case KeyEnter, "e":
		// Enter edit mode for text fields
		ft := m.getSubEntryFieldType()

		if m.isSubEntryTextInputField() {
			m.enterSubEntryFieldEditMode()
			return m, nil
		}
		// Handle toggles on enter
		switch ft {
		case subFieldIsFolder:
			m.subEntryForm.isFolder = !m.subEntryForm.isFolder
			return m, nil
		case subFieldIsSudo:
			m.subEntryForm.isSudo = !m.subEntryForm.isSudo
			return m, nil
		case subFieldName, subFieldLinux, subFieldWindows, subFieldBackup, subFieldFiles:
			// Text and list fields don't toggle
		}

	case "s", "ctrl+s":
		// Save the form
		if err := m.saveSubEntryForm(); err != nil {
			m.subEntryForm.err = err.Error()
			return m, nil
		}
		// Success - go back to list
		m.activeForm = FormNone
		m.subEntryForm = nil
		m.Screen = ScreenResults

		return m, nil
	}

	// Clear error when navigating
	m.subEntryForm.err = ""

	return m, nil
}

// updateSubEntryFieldInput handles key events when editing a text field
func (m Model) updateSubEntryFieldInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.subEntryForm == nil {
		return m, nil
	}

	var cmd tea.Cmd
	ft := m.getSubEntryFieldType()

	// Check for suggestions (only for path fields)
	isPathField := ft == subFieldLinux || ft == subFieldWindows || ft == subFieldBackup
	hasSuggestions := m.subEntryForm.showSuggestions && len(m.subEntryForm.suggestions) > 0
	hasSelectedSuggestion := hasSuggestions && m.subEntryForm.suggestionCursor >= 0

	switch msg.String() {
	case KeyCtrlC:
		return m, tea.Quit

	case KeyEsc:
		// If suggestions are showing, close them first
		if hasSuggestions {
			m.subEntryForm.showSuggestions = false
			return m, nil
		}
		// Cancel editing and restore original value
		m.cancelSubEntryFieldEdit()

		return m, nil

	case KeyEnter:
		// Accept suggestion only if user has explicitly selected one
		if hasSelectedSuggestion {
			m.acceptSuggestionSubEntry()
			return m, nil
		}
		// Save and exit edit mode
		m.subEntryForm.editingField = false
		m.subEntryForm.showSuggestions = false
		m.updateSubEntryFormFocus()

		return m, nil

	case KeyTab:
		// Accept suggestion if selected
		if hasSelectedSuggestion {
			m.acceptSuggestionSubEntry()
			return m, nil
		}
		// Save and exit edit mode
		m.subEntryForm.editingField = false
		m.subEntryForm.showSuggestions = false
		m.updateSubEntryFormFocus()

		return m, nil

	case "up":
		// Navigate suggestions if showing
		if hasSuggestions {
			if m.subEntryForm.suggestionCursor < 0 {
				m.subEntryForm.suggestionCursor = len(m.subEntryForm.suggestions) - 1
			} else {
				m.subEntryForm.suggestionCursor--
			}

			return m, nil
		}

	case KeyDown:
		// Navigate suggestions if showing
		if hasSuggestions {
			if m.subEntryForm.suggestionCursor < 0 {
				m.subEntryForm.suggestionCursor = 0
			} else {
				m.subEntryForm.suggestionCursor++
				if m.subEntryForm.suggestionCursor >= len(m.subEntryForm.suggestions) {
					m.subEntryForm.suggestionCursor = 0
				}
			}

			return m, nil
		}
	}

	// Handle text input for the focused field
	switch ft {
	case subFieldName:
		m.subEntryForm.nameInput, cmd = m.subEntryForm.nameInput.Update(msg)
	case subFieldLinux:
		m.subEntryForm.linuxTargetInput, cmd = m.subEntryForm.linuxTargetInput.Update(msg)
	case subFieldWindows:
		m.subEntryForm.windowsTargetInput, cmd = m.subEntryForm.windowsTargetInput.Update(msg)
	case subFieldBackup:
		m.subEntryForm.backupInput, cmd = m.subEntryForm.backupInput.Update(msg)
	case subFieldIsFolder, subFieldFiles, subFieldIsSudo:
		// Boolean and list fields don't use text input
	}

	// Update suggestions for path fields after text changes
	if isPathField {
		m.updateSuggestionsSubEntry()
	}

	// Clear error when typing
	m.subEntryForm.err = ""

	return m, cmd
}

// updateSubEntryFilesList handles key events when the files list is focused
func (m Model) updateSubEntryFilesList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.subEntryForm == nil {
		return m, nil
	}

	// filesCursor: 0 to len(files)-1 for file items, len(files) for "Add File" button
	maxCursor := len(m.subEntryForm.files)

	switch msg.String() {
	case KeyCtrlC:
		return m, tea.Quit

	case "q", KeyEsc:
		m.activeForm = FormNone
		m.subEntryForm = nil
		m.Screen = ScreenResults

		return m, nil

	case "up", "k":
		if m.subEntryForm.filesCursor > 0 {
			m.subEntryForm.filesCursor--
		} else {
			// Move to previous field
			m.subEntryForm.focusIndex--
			m.updateSubEntryFormFocus()
		}

		return m, nil

	case KeyDown, "j":
		if m.subEntryForm.filesCursor < maxCursor {
			m.subEntryForm.filesCursor++
		} else {
			// Move to next field
			m.subEntryForm.focusIndex++

			maxIndex := m.subEntryFormMaxIndex()
			if m.subEntryForm.focusIndex > maxIndex {
				m.subEntryForm.focusIndex = 0
			}
			m.subEntryForm.filesCursor = 0
			m.updateSubEntryFormFocus()
		}

		return m, nil

	case KeyTab:
		// Move to next field
		m.subEntryForm.focusIndex++

		maxIndex := m.subEntryFormMaxIndex()
		if m.subEntryForm.focusIndex > maxIndex {
			m.subEntryForm.focusIndex = 0
		}
		m.subEntryForm.filesCursor = 0
		m.updateSubEntryFormFocus()

		return m, nil

	case KeyShiftTab:
		// Move to previous field
		m.subEntryForm.focusIndex--
		m.subEntryForm.filesCursor = 0
		m.updateSubEntryFormFocus()

		return m, nil

	case KeyEnter, " ":
		// If on "Add File" button, start adding
		if m.subEntryForm.filesCursor == len(m.subEntryForm.files) {
			m.subEntryForm.addingFile = true
			m.subEntryForm.newFileInput.SetValue("")
			m.subEntryForm.newFileInput.Focus()

			return m, nil
		}
		// Edit the selected file
		if m.subEntryForm.filesCursor < len(m.subEntryForm.files) {
			m.subEntryForm.editingFile = true
			m.subEntryForm.editingFileIndex = m.subEntryForm.filesCursor
			m.subEntryForm.newFileInput.SetValue(m.subEntryForm.files[m.subEntryForm.filesCursor])
			m.subEntryForm.newFileInput.Focus()
			m.subEntryForm.newFileInput.SetCursor(len(m.subEntryForm.files[m.subEntryForm.filesCursor]))
		}

		return m, nil

	case "d", "backspace", KeyDelete:
		// Delete the selected file
		if m.subEntryForm.filesCursor < len(m.subEntryForm.files) && len(m.subEntryForm.files) > 0 {
			// Remove file at cursor
			m.subEntryForm.files = append(
				m.subEntryForm.files[:m.subEntryForm.filesCursor],
				m.subEntryForm.files[m.subEntryForm.filesCursor+1:]...,
			)
			// Adjust cursor if needed
			if m.subEntryForm.filesCursor >= len(m.subEntryForm.files) && m.subEntryForm.filesCursor > 0 {
				m.subEntryForm.filesCursor--
			}
		}

		return m, nil

	case "s", "ctrl+s":
		// Save the form
		if err := m.saveSubEntryForm(); err != nil {
			m.subEntryForm.err = err.Error()
			return m, nil
		}
		m.activeForm = FormNone
		m.subEntryForm = nil
		m.Screen = ScreenResults

		return m, nil
	}

	return m, nil
}

// updateSubEntryFileInput handles key events when adding or editing a file
func (m Model) updateSubEntryFileInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.subEntryForm == nil {
		return m, nil
	}

	var cmd tea.Cmd

	switch msg.String() {
	case KeyCtrlC:
		return m, tea.Quit

	case KeyEsc:
		// Cancel adding/editing file
		m.subEntryForm.addingFile = false
		m.subEntryForm.editingFile = false
		m.subEntryForm.editingFileIndex = -1
		m.subEntryForm.newFileInput.SetValue("")

		return m, nil

	case KeyEnter:
		fileName := strings.TrimSpace(m.subEntryForm.newFileInput.Value())
		if m.subEntryForm.editingFile {
			// Update existing file if not empty
			if fileName != "" && m.subEntryForm.editingFileIndex >= 0 && m.subEntryForm.editingFileIndex < len(m.subEntryForm.files) {
				m.subEntryForm.files[m.subEntryForm.editingFileIndex] = fileName
			}
			m.subEntryForm.editingFile = false
			m.subEntryForm.editingFileIndex = -1
		} else {
			// Add new file if not empty
			if fileName != "" {
				m.subEntryForm.files = append(m.subEntryForm.files, fileName)
				m.subEntryForm.filesCursor = len(m.subEntryForm.files) // Move cursor to "Add File" button
			}
			m.subEntryForm.addingFile = false
		}

		m.subEntryForm.newFileInput.SetValue("")

		return m, nil
	}

	// Handle text input
	m.subEntryForm.newFileInput, cmd = m.subEntryForm.newFileInput.Update(msg)

	return m, cmd
}

// viewSubEntryForm renders the sub-entry form
//
//nolint:gocyclo // UI rendering with many states
func (m Model) viewSubEntryForm() string {
	if m.subEntryForm == nil {
		return ""
	}

	var b strings.Builder
	ft := m.getSubEntryFieldType()

	// Title
	if m.subEntryForm.editAppIdx >= 0 {
		b.WriteString(TitleStyle.Render("  Edit Config Entry"))
		b.WriteString("\n\n")
		b.WriteString(SubtitleStyle.Render("Edit the entry configuration"))
	} else {
		b.WriteString(TitleStyle.Render("  Add Entry"))
		b.WriteString("\n\n")
		b.WriteString(SubtitleStyle.Render("Add a new entry to the application"))
	}

	b.WriteString("\n\n")

	// Name field
	nameLabel := "Name:"
	if ft == subFieldName {
		nameLabel = HelpKeyStyle.Render("Name:")
	}

	b.WriteString(fmt.Sprintf("  %s\n", nameLabel))
	b.WriteString(fmt.Sprintf("  %s\n\n", m.renderSubEntryFieldValue(subFieldName, "(empty)")))

	// Linux target field
	linuxTargetLabel := "Target (linux):"
	if ft == subFieldLinux {
		linuxTargetLabel = HelpKeyStyle.Render(linuxTargetLabel)
	}

	b.WriteString(fmt.Sprintf("  %s\n", linuxTargetLabel))
	b.WriteString(fmt.Sprintf("  %s\n", m.renderSubEntryFieldValue(subFieldLinux, "(empty)")))

	if m.subEntryForm.editingField && ft == subFieldLinux && m.subEntryForm.showSuggestions {
		b.WriteString(m.renderSubEntrySuggestions())
	}

	b.WriteString("\n")

	// Windows target field
	windowsTargetLabel := "Target (windows):"
	if ft == subFieldWindows {
		windowsTargetLabel = HelpKeyStyle.Render(windowsTargetLabel)
	}

	b.WriteString(fmt.Sprintf("  %s\n", windowsTargetLabel))
	b.WriteString(fmt.Sprintf("  %s\n", m.renderSubEntryFieldValue(subFieldWindows, "(empty)")))

	if m.subEntryForm.editingField && ft == subFieldWindows && m.subEntryForm.showSuggestions {
		b.WriteString(m.renderSubEntrySuggestions())
	}

	b.WriteString("\n")

	// Backup field
	backupLabel := "Backup path:"
	if ft == subFieldBackup {
		backupLabel = HelpKeyStyle.Render("Backup path:")
	}

	b.WriteString(fmt.Sprintf("  %s\n", backupLabel))
	b.WriteString(fmt.Sprintf("  %s\n", m.renderSubEntryFieldValue(subFieldBackup, "(empty)")))

	if m.subEntryForm.editingField && ft == subFieldBackup && m.subEntryForm.showSuggestions {
		b.WriteString(m.renderSubEntrySuggestions())
	}

	b.WriteString("\n")

	// Is folder toggle
	toggleLabel := "Backup type:"
	if ft == subFieldIsFolder {
		toggleLabel = HelpKeyStyle.Render("Backup type:")
	}
	folderCheck := CheckboxUnchecked
	filesCheck := CheckboxChecked

	if m.subEntryForm.isFolder {
		folderCheck = CheckboxChecked
		filesCheck = CheckboxUnchecked
	}

	b.WriteString(fmt.Sprintf("  %s  %s Folder  %s Files\n\n", toggleLabel, folderCheck, filesCheck))

	// Files list (only shown when Files mode is selected)
	if !m.subEntryForm.isFolder {
		filesLabel := "Files:"
		if ft == subFieldFiles {
			filesLabel = HelpKeyStyle.Render("Files:")
		}

		b.WriteString(fmt.Sprintf("  %s\n", filesLabel))

		// Render file list
		if len(m.subEntryForm.files) == 0 && !m.subEntryForm.addingFile {
			b.WriteString(MutedTextStyle.Render("    (no files added)"))
			b.WriteString("\n")
		} else {
			for i, file := range m.subEntryForm.files {
				prefix := IndentSpaces
				// Show input if editing this file
				switch {
				case m.subEntryForm.editingFile && m.subEntryForm.editingFileIndex == i:
					b.WriteString(fmt.Sprintf("%s%s\n", prefix, m.subEntryForm.newFileInput.View()))
				case ft == subFieldFiles && !m.subEntryForm.addingFile && !m.subEntryForm.editingFile && m.subEntryForm.filesCursor == i:
					b.WriteString(fmt.Sprintf("%s%s\n", prefix, SelectedMenuItemStyle.Render("• "+file)))
				default:
					b.WriteString(fmt.Sprintf("%s• %s\n", prefix, file))
				}
			}
		}

		// Add File button or input
		if m.subEntryForm.addingFile {
			b.WriteString(fmt.Sprintf("    %s\n", m.subEntryForm.newFileInput.View()))
		} else if !m.subEntryForm.editingFile {
			addFileText := "[+ Add File]"
			if ft == subFieldFiles && m.subEntryForm.filesCursor == len(m.subEntryForm.files) {
				b.WriteString(fmt.Sprintf("    %s\n", SelectedMenuItemStyle.Render(addFileText)))
			} else {
				b.WriteString(fmt.Sprintf("    %s\n", MutedTextStyle.Render(addFileText)))
			}
		}

		b.WriteString("\n")
	}

	// Root toggle
	rootLabel := "Root only:"
	if ft == subFieldIsSudo {
		rootLabel = HelpKeyStyle.Render("Root only:")
	}

	rootCheck := CheckboxUnchecked
	if m.subEntryForm.isSudo {
		rootCheck = CheckboxChecked
	}

	b.WriteString(fmt.Sprintf("  %s  %s Yes\n\n", rootLabel, rootCheck))

	// Error message
	if m.subEntryForm.err != "" {
		b.WriteString(ErrorStyle.Render("  Error: " + m.subEntryForm.err))
		b.WriteString("\n\n")
	}

	// Help
	b.WriteString(m.renderSubEntryFormHelp())

	return BaseStyle.Render(b.String())
}

// renderSubEntryFieldValue renders a field value with appropriate styling
//
//nolint:unparam // placeholder parameter kept for consistency and future extensibility
func (m Model) renderSubEntryFieldValue(fieldType subEntryFieldType, placeholder string) string {
	if m.subEntryForm == nil {
		return placeholder
	}

	currentFt := m.getSubEntryFieldType()
	isEditing := m.subEntryForm.editingField && currentFt == fieldType
	isFocused := currentFt == fieldType

	var input textinput.Model

	switch fieldType {
	case subFieldName:
		input = m.subEntryForm.nameInput
	case subFieldLinux:
		input = m.subEntryForm.linuxTargetInput
	case subFieldWindows:
		input = m.subEntryForm.windowsTargetInput
	case subFieldBackup:
		input = m.subEntryForm.backupInput
	case subFieldIsFolder, subFieldFiles, subFieldIsSudo:
		return placeholder
	default:
		return placeholder
	}

	if isEditing {
		return input.View()
	}

	value := input.Value()
	if value == "" {
		value = MutedTextStyle.Render(placeholder)
	}

	if isFocused {
		return SelectedMenuItemStyle.Render(value)
	}

	return value
}

// renderSubEntryFormHelp renders context-sensitive help for the sub-entry form
func (m Model) renderSubEntryFormHelp() string {
	if m.subEntryForm == nil {
		return ""
	}

	ft := m.getSubEntryFieldType()

	if m.subEntryForm.addingFile {
		return RenderHelp(
			"enter", "add file",
			"esc", "cancel",
		)
	}

	if m.subEntryForm.editingFile {
		return RenderHelp(
			"enter", "save",
			"esc", "cancel",
		)
	}

	if m.subEntryForm.editingField {
		// Editing a text field
		if m.subEntryForm.showSuggestions && len(m.subEntryForm.suggestions) > 0 && m.subEntryForm.suggestionCursor >= 0 {
			return RenderHelp(
				"↑/↓", "select",
				"tab/enter", "accept",
				"esc", "cancel edit",
			)
		}

		if m.subEntryForm.showSuggestions && len(m.subEntryForm.suggestions) > 0 {
			return RenderHelp(
				"↑/↓", "select suggestion",
				"enter/tab", "save",
				"esc", "cancel edit",
			)
		}

		return RenderHelp(
			"enter/tab", "save",
			"esc", "cancel edit",
		)
	}

	if ft == subFieldFiles {
		// Files list focused
		if m.subEntryForm.filesCursor < len(m.subEntryForm.files) {
			return RenderHelp(
				"enter/e", "edit",
				"d/del", "remove",
				"s", "save",
				"q", "back",
			)
		}

		return RenderHelp(
			"enter/e", "add file",
			"s", "save",
			"q", "back",
		)
	}

	if m.isSubEntryTextInputField() {
		// Text field focused (not editing)
		return RenderHelp(
			"enter/e", "edit",
			"s", "save",
			"q", "back",
		)
	}

	if m.isSubEntryToggleField() {
		// Toggle field focused
		return RenderHelp(
			"enter/space", "toggle",
			"s", "save",
			"q", "back",
		)
	}

	return RenderHelp(
		"enter/e", "edit",
		"s", "save",
		"q", "back",
	)
}

// renderSubEntrySuggestions renders the autocomplete dropdown
func (m Model) renderSubEntrySuggestions() string {
	if m.subEntryForm == nil || len(m.subEntryForm.suggestions) == 0 {
		return ""
	}

	var b strings.Builder

	for i, suggestion := range m.subEntryForm.suggestions {
		if i == m.subEntryForm.suggestionCursor {
			b.WriteString(fmt.Sprintf("  %s\n", SelectedMenuItemStyle.Render(suggestion)))
		} else {
			b.WriteString(fmt.Sprintf("  %s\n", MutedTextStyle.Render(suggestion)))
		}
	}

	return b.String()
}

// saveSubEntryForm validates and saves the sub-entry form
func (m *Model) saveSubEntryForm() error {
	if m.subEntryForm == nil {
		return fmt.Errorf("no form data")
	}

	name := strings.TrimSpace(m.subEntryForm.nameInput.Value())
	targets := buildTargetsFromSubEntryForm(m.subEntryForm)

	// Validation
	if name == "" {
		return fmt.Errorf("name is required")
	}

	if len(targets) == 0 {
		return fmt.Errorf("at least one target is required")
	}

	backup := strings.TrimSpace(m.subEntryForm.backupInput.Value())
	if backup == "" {
		return fmt.Errorf("backup path is required")
	}

	// Build SubEntry from form
	subEntry := config.SubEntry{
		Name:    name,
		Targets: targets,
		Sudo:    m.subEntryForm.isSudo,
		Backup:  backup,
	}

	// Add files if in files mode
	if !m.subEntryForm.isFolder {
		if len(m.subEntryForm.files) == 0 {
			return fmt.Errorf("at least one file is required when using Files mode")
		}
		subEntry.Files = make([]string, len(m.subEntryForm.files))
		copy(subEntry.Files, m.subEntryForm.files)
	}

	// Route to correct save operation
	if m.subEntryForm.editAppIdx >= 0 && m.subEntryForm.editSubIdx >= 0 {
		// Editing existing SubEntry
		return m.updateSubEntry(m.subEntryForm.editAppIdx, m.subEntryForm.editSubIdx, subEntry)
	} else if m.subEntryForm.targetAppIdx >= 0 {
		// Adding SubEntry to existing Application
		return m.addSubEntryToApp(m.subEntryForm.targetAppIdx, subEntry)
	}

	return fmt.Errorf("invalid form state")
}

// Helper functions

// updateSubEntryFormFocus updates which input field is focused
func (m *Model) updateSubEntryFormFocus() {
	if m.subEntryForm == nil {
		return
	}

	m.subEntryForm.nameInput.Blur()
	m.subEntryForm.linuxTargetInput.Blur()
	m.subEntryForm.windowsTargetInput.Blur()
	m.subEntryForm.backupInput.Blur()
	m.subEntryForm.newFileInput.Blur()

	ft := m.getSubEntryFieldType()
	switch ft {
	case subFieldName:
		m.subEntryForm.nameInput.Focus()
	case subFieldLinux:
		m.subEntryForm.linuxTargetInput.Focus()
	case subFieldWindows:
		m.subEntryForm.windowsTargetInput.Focus()
	case subFieldBackup:
		m.subEntryForm.backupInput.Focus()
	case subFieldIsFolder, subFieldFiles, subFieldIsSudo:
		// Boolean and list fields don't use text input focus
	}
}

// enterSubEntryFieldEditMode enters edit mode for the current text field
func (m *Model) enterSubEntryFieldEditMode() {
	if m.subEntryForm == nil {
		return
	}

	m.subEntryForm.editingField = true
	ft := m.getSubEntryFieldType()

	switch ft {
	case subFieldName:
		m.subEntryForm.originalValue = m.subEntryForm.nameInput.Value()
		m.subEntryForm.nameInput.Focus()
		m.subEntryForm.nameInput.SetCursor(len(m.subEntryForm.nameInput.Value()))
	case subFieldLinux:
		m.subEntryForm.originalValue = m.subEntryForm.linuxTargetInput.Value()
		m.subEntryForm.linuxTargetInput.Focus()
		m.subEntryForm.linuxTargetInput.SetCursor(len(m.subEntryForm.linuxTargetInput.Value()))
	case subFieldWindows:
		m.subEntryForm.originalValue = m.subEntryForm.windowsTargetInput.Value()
		m.subEntryForm.windowsTargetInput.Focus()
		m.subEntryForm.windowsTargetInput.SetCursor(len(m.subEntryForm.windowsTargetInput.Value()))
	case subFieldBackup:
		m.subEntryForm.originalValue = m.subEntryForm.backupInput.Value()
		m.subEntryForm.backupInput.Focus()
		m.subEntryForm.backupInput.SetCursor(len(m.subEntryForm.backupInput.Value()))
	case subFieldIsFolder, subFieldFiles, subFieldIsSudo:
		// Boolean and list fields don't use text input editing
	}
}

// cancelSubEntryFieldEdit cancels editing and restores the original value
func (m *Model) cancelSubEntryFieldEdit() {
	if m.subEntryForm == nil {
		return
	}

	ft := m.getSubEntryFieldType()
	switch ft {
	case subFieldName:
		m.subEntryForm.nameInput.SetValue(m.subEntryForm.originalValue)
	case subFieldLinux:
		m.subEntryForm.linuxTargetInput.SetValue(m.subEntryForm.originalValue)
	case subFieldWindows:
		m.subEntryForm.windowsTargetInput.SetValue(m.subEntryForm.originalValue)
	case subFieldBackup:
		m.subEntryForm.backupInput.SetValue(m.subEntryForm.originalValue)
	case subFieldIsFolder, subFieldFiles, subFieldIsSudo:
		// Boolean and list fields don't use text input restoration
	}

	m.subEntryForm.editingField = false
	m.subEntryForm.showSuggestions = false
	m.subEntryForm.err = ""
	m.updateSubEntryFormFocus()
}

// updateSuggestionsSubEntry refreshes the autocomplete suggestions for the current path field
func (m *Model) updateSuggestionsSubEntry() {
	if m.subEntryForm == nil {
		return
	}

	var input string
	var configDir string
	ft := m.getSubEntryFieldType()

	// Get config directory for relative path resolution
	if m.ConfigPath != "" {
		configDir = filepath.Dir(m.ConfigPath)
	}

	switch ft {
	case subFieldLinux:
		input = m.subEntryForm.linuxTargetInput.Value()
	case subFieldWindows:
		input = m.subEntryForm.windowsTargetInput.Value()
	case subFieldBackup:
		input = m.subEntryForm.backupInput.Value()
	case subFieldName, subFieldIsFolder, subFieldFiles, subFieldIsSudo:
		m.subEntryForm.showSuggestions = false
		m.subEntryForm.suggestions = nil
		return
	default:
		m.subEntryForm.showSuggestions = false
		m.subEntryForm.suggestions = nil

		return
	}

	suggestions := getPathSuggestions(input, configDir)
	m.subEntryForm.suggestions = suggestions
	m.subEntryForm.suggestionCursor = -1 // No selection until user uses arrows
	m.subEntryForm.showSuggestions = len(suggestions) > 0
}

// acceptSuggestionSubEntry fills in the selected suggestion
func (m *Model) acceptSuggestionSubEntry() {
	if m.subEntryForm == nil || len(m.subEntryForm.suggestions) == 0 {
		return
	}

	suggestion := m.subEntryForm.suggestions[m.subEntryForm.suggestionCursor]
	ft := m.getSubEntryFieldType()

	switch ft {
	case subFieldLinux:
		m.subEntryForm.linuxTargetInput.SetValue(suggestion)
		m.subEntryForm.linuxTargetInput.SetCursor(len(suggestion))
	case subFieldWindows:
		m.subEntryForm.windowsTargetInput.SetValue(suggestion)
		m.subEntryForm.windowsTargetInput.SetCursor(len(suggestion))
	case subFieldBackup:
		m.subEntryForm.backupInput.SetValue(suggestion)
		m.subEntryForm.backupInput.SetCursor(len(suggestion))
	case subFieldIsFolder, subFieldFiles, subFieldIsSudo, subFieldName:
		// Other fields don't use suggestions
	}

	// Keep suggestions open for continued navigation if it's a directory
	if strings.HasSuffix(suggestion, "/") {
		m.updateSuggestionsSubEntry()
	} else {
		m.subEntryForm.showSuggestions = false
		m.subEntryForm.suggestions = nil
	}
}

// isSubEntryTextInputField returns true if the current field is a text input
func (m *Model) isSubEntryTextInputField() bool {
	if m.subEntryForm == nil {
		return false
	}

	ft := m.getSubEntryFieldType()
	switch ft {
	case subFieldName, subFieldLinux, subFieldWindows, subFieldBackup:
		return true
	case subFieldIsFolder, subFieldFiles, subFieldIsSudo:
		// These fields don't have suggestions
	}

	return false
}

// isSubEntryToggleField returns true if the current field is a toggle
func (m *Model) isSubEntryToggleField() bool {
	if m.subEntryForm == nil {
		return false
	}

	ft := m.getSubEntryFieldType()

	return ft == subFieldIsFolder || ft == subFieldIsSudo
}

// buildTargetsFromSubEntryForm creates Targets map from form inputs
func buildTargetsFromSubEntryForm(form *SubEntryForm) map[string]string {
	targets := make(map[string]string)
	if linux := strings.TrimSpace(form.linuxTargetInput.Value()); linux != "" {
		targets["linux"] = linux
	}

	if windows := strings.TrimSpace(form.windowsTargetInput.Value()); windows != "" {
		targets["windows"] = windows
	}

	return targets
}

// addSubEntryToApp adds a SubEntry to an existing Application
func (m *Model) addSubEntryToApp(appIdx int, subEntry config.SubEntry) error {
	if appIdx < 0 || appIdx >= len(m.Config.Applications) {
		return fmt.Errorf("invalid application index")
	}

	app := &m.Config.Applications[appIdx]

	// Check for duplicate SubEntry names within this Application
	for _, existing := range app.Entries {
		if existing.Name == subEntry.Name {
			return fmt.Errorf("a sub-entry with name '%s' already exists in this application", subEntry.Name)
		}
	}

	app.Entries = append(app.Entries, subEntry)

	if err := config.Save(m.Config, m.ConfigPath); err != nil {
		// Rollback
		app.Entries = app.Entries[:len(app.Entries)-1]
		return fmt.Errorf("failed to save config: %w", err)
	}

	m.initApplicationItems()

	return nil
}

// updateSubEntry updates an existing SubEntry
func (m *Model) updateSubEntry(appIdx, subIdx int, subEntry config.SubEntry) error {
	if appIdx < 0 || appIdx >= len(m.Config.Applications) {
		return fmt.Errorf("invalid application index")
	}

	app := &m.Config.Applications[appIdx]

	if subIdx < 0 || subIdx >= len(app.Entries) {
		return fmt.Errorf("invalid sub-entry index")
	}

	// Check for duplicate names (skip the one being edited)
	for i, existing := range app.Entries {
		if i != subIdx && existing.Name == subEntry.Name {
			return fmt.Errorf("a sub-entry with name '%s' already exists in this application", subEntry.Name)
		}
	}

	// Update SubEntry
	app.Entries[subIdx] = subEntry

	if err := config.Save(m.Config, m.ConfigPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	m.initApplicationItems()

	return nil
}

// NewSubEntryForm creates a new SubEntryForm for testing purposes
func NewSubEntryForm(entry config.SubEntry) *SubEntryForm {
	nameInput := textinput.New()
	nameInput.SetValue(entry.Name)

	linuxTargetInput := textinput.New()
	if target, ok := entry.Targets["linux"]; ok {
		linuxTargetInput.SetValue(target)
	}

	windowsTargetInput := textinput.New()
	if target, ok := entry.Targets["windows"]; ok {
		windowsTargetInput.SetValue(target)
	}

	backupInput := textinput.New()
	backupInput.SetValue(entry.Backup)

	return &SubEntryForm{
		nameInput:          nameInput,
		linuxTargetInput:   linuxTargetInput,
		windowsTargetInput: windowsTargetInput,
		backupInput:        backupInput,
		isSudo:             entry.Sudo,
		isFolder:           entry.IsFolder(),
		files:              entry.Files,
	}
}

// Validate checks if the SubEntryForm has valid data
func (f *SubEntryForm) Validate() error {
	if strings.TrimSpace(f.nameInput.Value()) == "" {
		return errors.New("entry name is required")
	}

	if strings.TrimSpace(f.backupInput.Value()) == "" {
		return errors.New("backup path is required")
	}

	// Check if at least one target is specified
	hasTarget := strings.TrimSpace(f.linuxTargetInput.Value()) != "" ||
		strings.TrimSpace(f.windowsTargetInput.Value()) != ""

	if !hasTarget {
		return errors.New("at least one target is required")
	}

	return nil
}
