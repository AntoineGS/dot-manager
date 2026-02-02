package tui

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// initAddForm initializes the add form with empty inputs
func (m *Model) initAddForm() {
	m.initAddFormWithIndex(-1)
}

// initAddFormWithIndex initializes the form, optionally populating with existing data for editing
func (m *Model) initAddFormWithIndex(editIndex int) {
	nameInput := textinput.New()
	nameInput.Placeholder = "e.g., neovim"
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

	isFolder := true
	var files []string

	// Populate with existing data if editing
	if editIndex >= 0 && editIndex < len(m.Paths) {
		spec := m.Paths[editIndex].Spec
		nameInput.SetValue(spec.Name)
		if target, ok := spec.Targets["linux"]; ok {
			linuxTargetInput.SetValue(target)
		}
		if target, ok := spec.Targets["windows"]; ok {
			windowsTargetInput.SetValue(target)
		}
		backupInput.SetValue(spec.Backup)
		isFolder = spec.IsFolder()
		if !isFolder {
			// Copy the files slice
			files = make([]string, len(spec.Files))
			copy(files, spec.Files)
		}
	}

	m.addForm = AddForm{
		nameInput:          nameInput,
		linuxTargetInput:   linuxTargetInput,
		windowsTargetInput: windowsTargetInput,
		backupInput:        backupInput,
		isFolder:           isFolder,
		focusIndex:         0,
		err:                "",
		editIndex:          editIndex,
		editingField:       false,
		originalValue:      "",
		files:              files,
		filesCursor:        0,
		newFileInput:       newFileInput,
		addingFile:         false,
		editingFile:        false,
		editingFileIndex:   -1,
	}
}

// updateAddForm handles key events for the add form
func (m Model) updateAddForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle editing a text field (fields 0-3)
	if m.addForm.editingField {
		return m.updateFieldInput(msg)
	}

	// Handle adding/editing file mode separately
	if m.addForm.addingFile || m.addForm.editingFile {
		return m.updateFileInput(msg)
	}

	// Handle files list navigation when focused on files area
	if m.addForm.focusIndex == 5 && !m.addForm.isFolder {
		return m.updateFilesList(msg)
	}

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		// Return to list if editing, menu if adding
		if m.addForm.editIndex >= 0 {
			m.Screen = ScreenResults
		} else {
			m.Screen = ScreenMenu
		}
		return m, nil

	case "down", "j":
		m.addForm.focusIndex++
		maxIndex := m.addFormMaxIndex()
		if m.addForm.focusIndex > maxIndex {
			m.addForm.focusIndex = 0
		}
		return m, nil

	case "up", "k":
		m.addForm.focusIndex--
		if m.addForm.focusIndex < 0 {
			m.addForm.focusIndex = m.addFormMaxIndex()
		}
		return m, nil

	case "tab":
		m.addForm.focusIndex++
		maxIndex := m.addFormMaxIndex()
		if m.addForm.focusIndex > maxIndex {
			m.addForm.focusIndex = 0
		}
		return m, nil

	case "shift+tab":
		m.addForm.focusIndex--
		if m.addForm.focusIndex < 0 {
			m.addForm.focusIndex = m.addFormMaxIndex()
		}
		return m, nil

	case " ":
		// Toggle isFolder if on the toggle field
		if m.addForm.focusIndex == 4 {
			m.addForm.isFolder = !m.addForm.isFolder
			return m, nil
		}

	case "enter", "e":
		// Enter edit mode for text fields (0-3)
		if m.addForm.focusIndex >= 0 && m.addForm.focusIndex <= 3 {
			m.enterFieldEditMode()
			return m, nil
		}
		// If on toggle field, toggle it
		if m.addForm.focusIndex == 4 {
			m.addForm.isFolder = !m.addForm.isFolder
			return m, nil
		}

	case "s", "ctrl+s":
		// Save the form
		if err := m.saveNewPath(); err != nil {
			m.addForm.err = err.Error()
			return m, nil
		}
		// Success - go back to list if editing, menu if adding
		if m.addForm.editIndex >= 0 {
			m.Screen = ScreenResults
		} else {
			m.Screen = ScreenMenu
		}
		return m, nil
	}

	// Clear error when navigating
	m.addForm.err = ""

	return m, nil
}

// enterFieldEditMode enters edit mode for the current text field
func (m *Model) enterFieldEditMode() {
	m.addForm.editingField = true

	// Store original value and focus the input
	switch m.addForm.focusIndex {
	case 0:
		m.addForm.originalValue = m.addForm.nameInput.Value()
		m.addForm.nameInput.Focus()
		m.addForm.nameInput.SetCursor(len(m.addForm.nameInput.Value()))
	case 1:
		m.addForm.originalValue = m.addForm.linuxTargetInput.Value()
		m.addForm.linuxTargetInput.Focus()
		m.addForm.linuxTargetInput.SetCursor(len(m.addForm.linuxTargetInput.Value()))
	case 2:
		m.addForm.originalValue = m.addForm.windowsTargetInput.Value()
		m.addForm.windowsTargetInput.Focus()
		m.addForm.windowsTargetInput.SetCursor(len(m.addForm.windowsTargetInput.Value()))
	case 3:
		m.addForm.originalValue = m.addForm.backupInput.Value()
		m.addForm.backupInput.Focus()
		m.addForm.backupInput.SetCursor(len(m.addForm.backupInput.Value()))
	}
}

// updateFieldInput handles key events when editing a text field
func (m Model) updateFieldInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Check for suggestions
	isPathField := m.addForm.focusIndex >= 1 && m.addForm.focusIndex <= 3
	hasSuggestions := m.addForm.showSuggestions && len(m.addForm.suggestions) > 0
	hasSelectedSuggestion := hasSuggestions && m.addForm.suggestionCursor >= 0

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		// If suggestions are showing, close them first
		if hasSuggestions {
			m.addForm.showSuggestions = false
			return m, nil
		}
		// Cancel editing and restore original value
		m.cancelFieldEdit()
		return m, nil

	case "enter":
		// Accept suggestion only if user has explicitly selected one
		if hasSelectedSuggestion {
			m.acceptSuggestion()
			return m, nil
		}
		// Save and exit edit mode
		m.addForm.editingField = false
		m.addForm.showSuggestions = false
		m.updateAddFormFocus()
		return m, nil

	case "tab":
		// Accept suggestion if selected
		if hasSelectedSuggestion {
			m.acceptSuggestion()
			return m, nil
		}
		// Save and exit edit mode
		m.addForm.editingField = false
		m.addForm.showSuggestions = false
		m.updateAddFormFocus()
		return m, nil

	case "up":
		// Navigate suggestions if showing
		if hasSuggestions {
			if m.addForm.suggestionCursor < 0 {
				m.addForm.suggestionCursor = len(m.addForm.suggestions) - 1
			} else {
				m.addForm.suggestionCursor--
			}
			return m, nil
		}

	case "down":
		// Navigate suggestions if showing
		if hasSuggestions {
			if m.addForm.suggestionCursor < 0 {
				m.addForm.suggestionCursor = 0
			} else {
				m.addForm.suggestionCursor++
				if m.addForm.suggestionCursor >= len(m.addForm.suggestions) {
					m.addForm.suggestionCursor = 0
				}
			}
			return m, nil
		}
	}

	// Handle text input for the focused field
	switch m.addForm.focusIndex {
	case 0:
		m.addForm.nameInput, cmd = m.addForm.nameInput.Update(msg)
	case 1:
		m.addForm.linuxTargetInput, cmd = m.addForm.linuxTargetInput.Update(msg)
	case 2:
		m.addForm.windowsTargetInput, cmd = m.addForm.windowsTargetInput.Update(msg)
	case 3:
		m.addForm.backupInput, cmd = m.addForm.backupInput.Update(msg)
	}

	// Update suggestions for path fields after text changes
	if isPathField {
		m.updateSuggestions()
	}

	// Clear error when typing
	m.addForm.err = ""

	return m, cmd
}

// cancelFieldEdit cancels editing and restores the original value
func (m *Model) cancelFieldEdit() {
	switch m.addForm.focusIndex {
	case 0:
		m.addForm.nameInput.SetValue(m.addForm.originalValue)
	case 1:
		m.addForm.linuxTargetInput.SetValue(m.addForm.originalValue)
	case 2:
		m.addForm.windowsTargetInput.SetValue(m.addForm.originalValue)
	case 3:
		m.addForm.backupInput.SetValue(m.addForm.originalValue)
	}
	m.addForm.editingField = false
	m.addForm.showSuggestions = false
	m.updateAddFormFocus()
}

// updateFilesList handles key events when the files list is focused
func (m Model) updateFilesList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// filesCursor: 0 to len(files)-1 for file items, len(files) for "Add File" button
	maxCursor := len(m.addForm.files) // "Add File" button is at index len(files)

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		// Return to list if editing, menu if adding
		if m.addForm.editIndex >= 0 {
			m.Screen = ScreenResults
		} else {
			m.Screen = ScreenMenu
		}
		return m, nil

	case "up", "k":
		if m.addForm.filesCursor > 0 {
			m.addForm.filesCursor--
		} else {
			// Move to previous field
			m.addForm.focusIndex--
			m.updateAddFormFocus()
		}
		return m, nil

	case "down", "j":
		if m.addForm.filesCursor < maxCursor {
			m.addForm.filesCursor++
		} else {
			// Wrap to first field
			m.addForm.focusIndex = 0
			m.addForm.filesCursor = 0
			m.updateAddFormFocus()
		}
		return m, nil

	case "tab":
		// Move to next field (wrap to beginning)
		m.addForm.focusIndex = 0
		m.addForm.filesCursor = 0
		m.updateAddFormFocus()
		return m, nil

	case "shift+tab":
		// Move to previous field
		m.addForm.focusIndex--
		m.updateAddFormFocus()
		return m, nil

	case "enter", " ":
		// If on "Add File" button, start adding
		if m.addForm.filesCursor == len(m.addForm.files) {
			m.addForm.addingFile = true
			m.addForm.newFileInput.SetValue("")
			m.addForm.newFileInput.Focus()
			return m, nil
		}
		// Edit the selected file
		if m.addForm.filesCursor < len(m.addForm.files) {
			m.addForm.editingFile = true
			m.addForm.editingFileIndex = m.addForm.filesCursor
			m.addForm.newFileInput.SetValue(m.addForm.files[m.addForm.filesCursor])
			m.addForm.newFileInput.Focus()
			m.addForm.newFileInput.SetCursor(len(m.addForm.files[m.addForm.filesCursor]))
		}
		return m, nil

	case "d", "backspace", "delete":
		// Delete the selected file
		if m.addForm.filesCursor < len(m.addForm.files) && len(m.addForm.files) > 0 {
			// Remove file at cursor
			m.addForm.files = append(m.addForm.files[:m.addForm.filesCursor], m.addForm.files[m.addForm.filesCursor+1:]...)
			// Adjust cursor if needed
			if m.addForm.filesCursor >= len(m.addForm.files) && m.addForm.filesCursor > 0 {
				m.addForm.filesCursor--
			}
		}
		return m, nil
	}

	return m, nil
}

// updateFileInput handles key events when adding or editing a file
func (m Model) updateFileInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		// Cancel adding/editing file
		m.addForm.addingFile = false
		m.addForm.editingFile = false
		m.addForm.editingFileIndex = -1
		m.addForm.newFileInput.SetValue("")
		return m, nil

	case "enter":
		fileName := strings.TrimSpace(m.addForm.newFileInput.Value())
		if m.addForm.editingFile {
			// Update existing file if not empty
			if fileName != "" && m.addForm.editingFileIndex >= 0 && m.addForm.editingFileIndex < len(m.addForm.files) {
				m.addForm.files[m.addForm.editingFileIndex] = fileName
			}
			m.addForm.editingFile = false
			m.addForm.editingFileIndex = -1
		} else {
			// Add new file if not empty
			if fileName != "" {
				m.addForm.files = append(m.addForm.files, fileName)
				m.addForm.filesCursor = len(m.addForm.files) // Move cursor to "Add File" button
			}
			m.addForm.addingFile = false
		}
		m.addForm.newFileInput.SetValue("")
		return m, nil
	}

	// Handle text input
	m.addForm.newFileInput, cmd = m.addForm.newFileInput.Update(msg)
	return m, cmd
}

// updateAddFormFocus updates which input field is focused
func (m *Model) updateAddFormFocus() {
	m.addForm.nameInput.Blur()
	m.addForm.linuxTargetInput.Blur()
	m.addForm.windowsTargetInput.Blur()
	m.addForm.backupInput.Blur()
	m.addForm.newFileInput.Blur()

	switch m.addForm.focusIndex {
	case 0:
		m.addForm.nameInput.Focus()
	case 1:
		m.addForm.linuxTargetInput.Focus()
	case 2:
		m.addForm.windowsTargetInput.Focus()
	case 3:
		m.addForm.backupInput.Focus()
		// case 4 is the isFolder toggle - no text input
		// case 5 is the files list area - handled separately
	}
}

// addFormMaxIndex returns the maximum focus index based on isFolder state
func (m *Model) addFormMaxIndex() int {
	if m.addForm.isFolder {
		return 4 // 0-4: name, linux, windows, backup, toggle
	}
	return 5 // 0-5: includes files input
}

// updateSuggestions refreshes the autocomplete suggestions for the current path field
func (m *Model) updateSuggestions() {
	var input string
	var configDir string

	// Get config directory for relative path resolution
	if m.ConfigPath != "" {
		configDir = filepath.Dir(m.ConfigPath)
	}

	switch m.addForm.focusIndex {
	case 1:
		input = m.addForm.linuxTargetInput.Value()
	case 2:
		input = m.addForm.windowsTargetInput.Value()
	case 3:
		input = m.addForm.backupInput.Value()
	default:
		m.addForm.showSuggestions = false
		m.addForm.suggestions = nil
		return
	}

	suggestions := getPathSuggestions(input, configDir)
	m.addForm.suggestions = suggestions
	m.addForm.suggestionCursor = -1 // No selection until user uses arrows
	m.addForm.showSuggestions = len(suggestions) > 0
}

// acceptSuggestion fills in the selected suggestion
func (m *Model) acceptSuggestion() {
	if len(m.addForm.suggestions) == 0 {
		return
	}

	suggestion := m.addForm.suggestions[m.addForm.suggestionCursor]

	switch m.addForm.focusIndex {
	case 1:
		m.addForm.linuxTargetInput.SetValue(suggestion)
		m.addForm.linuxTargetInput.SetCursor(len(suggestion))
	case 2:
		m.addForm.windowsTargetInput.SetValue(suggestion)
		m.addForm.windowsTargetInput.SetCursor(len(suggestion))
	case 3:
		m.addForm.backupInput.SetValue(suggestion)
		m.addForm.backupInput.SetCursor(len(suggestion))
	}

	// Keep suggestions open for continued navigation if it's a directory
	if strings.HasSuffix(suggestion, "/") {
		m.updateSuggestions()
	} else {
		m.addForm.showSuggestions = false
		m.addForm.suggestions = nil
	}
}

// renderFieldValue renders a field as either editable input or static text
func (m Model) renderFieldValue(fieldIndex int, input textinput.Model, placeholder string) string {
	isEditing := m.addForm.editingField && m.addForm.focusIndex == fieldIndex
	isFocused := m.addForm.focusIndex == fieldIndex

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

// viewAddForm renders the add form
func (m Model) viewAddForm() string {
	var b strings.Builder

	// Title
	if m.addForm.editIndex >= 0 {
		b.WriteString(TitleStyle.Render("󰏫  Edit Path Configuration"))
		b.WriteString("\n\n")
		b.WriteString(SubtitleStyle.Render("Edit the path configuration"))
	} else {
		b.WriteString(TitleStyle.Render("󰐕  Add Path Configuration"))
		b.WriteString("\n\n")
		b.WriteString(SubtitleStyle.Render("Add a new path to your dotfiles configuration"))
	}
	b.WriteString("\n\n")

	// Name field
	nameLabel := "Name:"
	if m.addForm.focusIndex == 0 {
		nameLabel = HelpKeyStyle.Render("Name:")
	}
	b.WriteString(fmt.Sprintf("  %s\n", nameLabel))
	b.WriteString(fmt.Sprintf("  %s\n\n", m.renderFieldValue(0, m.addForm.nameInput, "(empty)")))

	// Linux target field
	linuxTargetLabel := "Target (linux):"
	if m.addForm.focusIndex == 1 {
		linuxTargetLabel = HelpKeyStyle.Render(linuxTargetLabel)
	}
	b.WriteString(fmt.Sprintf("  %s\n", linuxTargetLabel))
	b.WriteString(fmt.Sprintf("  %s\n", m.renderFieldValue(1, m.addForm.linuxTargetInput, "(empty)")))
	if m.addForm.editingField && m.addForm.focusIndex == 1 && m.addForm.showSuggestions {
		b.WriteString(m.renderSuggestions())
	}
	b.WriteString("\n")

	// Windows target field
	windowsTargetLabel := "Target (windows):"
	if m.addForm.focusIndex == 2 {
		windowsTargetLabel = HelpKeyStyle.Render(windowsTargetLabel)
	}
	b.WriteString(fmt.Sprintf("  %s\n", windowsTargetLabel))
	b.WriteString(fmt.Sprintf("  %s\n", m.renderFieldValue(2, m.addForm.windowsTargetInput, "(empty)")))
	if m.addForm.editingField && m.addForm.focusIndex == 2 && m.addForm.showSuggestions {
		b.WriteString(m.renderSuggestions())
	}
	b.WriteString("\n")

	// Backup field
	backupLabel := "Backup path:"
	if m.addForm.focusIndex == 3 {
		backupLabel = HelpKeyStyle.Render("Backup path:")
	}
	b.WriteString(fmt.Sprintf("  %s\n", backupLabel))
	b.WriteString(fmt.Sprintf("  %s\n", m.renderFieldValue(3, m.addForm.backupInput, "(empty)")))
	if m.addForm.editingField && m.addForm.focusIndex == 3 && m.addForm.showSuggestions {
		b.WriteString(m.renderSuggestions())
	}
	b.WriteString("\n")

	// Is folder toggle
	toggleLabel := "Type:"
	if m.addForm.focusIndex == 4 {
		toggleLabel = HelpKeyStyle.Render("Type:")
	}
	folderCheck := "[ ]"
	filesCheck := "[✓]"
	if m.addForm.isFolder {
		folderCheck = "[✓]"
		filesCheck = "[ ]"
	}
	b.WriteString(fmt.Sprintf("  %s  %s Folder  %s Files\n", toggleLabel, folderCheck, filesCheck))
	b.WriteString("\n")

	// Files list (only shown when Files mode is selected)
	if !m.addForm.isFolder {
		filesLabel := "Files:"
		if m.addForm.focusIndex == 5 {
			filesLabel = HelpKeyStyle.Render("Files:")
		}
		b.WriteString(fmt.Sprintf("  %s\n", filesLabel))

		// Render file list
		if len(m.addForm.files) == 0 && !m.addForm.addingFile {
			b.WriteString(MutedTextStyle.Render("    (no files added)"))
			b.WriteString("\n")
		} else {
			for i, file := range m.addForm.files {
				prefix := "    "
				// Show input if editing this file
				if m.addForm.editingFile && m.addForm.editingFileIndex == i {
					b.WriteString(fmt.Sprintf("%s%s\n", prefix, m.addForm.newFileInput.View()))
				} else if m.addForm.focusIndex == 5 && !m.addForm.addingFile && !m.addForm.editingFile && m.addForm.filesCursor == i {
					b.WriteString(fmt.Sprintf("%s%s\n", prefix, SelectedMenuItemStyle.Render("• "+file)))
				} else {
					b.WriteString(fmt.Sprintf("%s• %s\n", prefix, file))
				}
			}
		}

		// Add File button or input
		if m.addForm.addingFile {
			b.WriteString(fmt.Sprintf("    %s\n", m.addForm.newFileInput.View()))
		} else if !m.addForm.editingFile {
			addFileText := "[+ Add File]"
			if m.addForm.focusIndex == 5 && m.addForm.filesCursor == len(m.addForm.files) {
				b.WriteString(fmt.Sprintf("    %s\n", SelectedMenuItemStyle.Render(addFileText)))
			} else {
				b.WriteString(fmt.Sprintf("    %s\n", MutedTextStyle.Render(addFileText)))
			}
		}
		b.WriteString("\n")
	}

	// Error message
	if m.addForm.err != "" {
		b.WriteString(ErrorStyle.Render("  Error: " + m.addForm.err))
		b.WriteString("\n\n")
	}

	// Help - show context-sensitive help
	if m.addForm.addingFile {
		b.WriteString(RenderHelp(
			"enter", "add file",
			"esc", "cancel",
		))
	} else if m.addForm.editingFile {
		b.WriteString(RenderHelp(
			"enter", "save",
			"esc", "cancel",
		))
	} else if m.addForm.editingField {
		// Editing a text field
		if m.addForm.showSuggestions && len(m.addForm.suggestions) > 0 && m.addForm.suggestionCursor >= 0 {
			b.WriteString(RenderHelp(
				"↑/↓", "select",
				"tab/enter", "accept",
				"esc", "cancel edit",
			))
		} else if m.addForm.showSuggestions && len(m.addForm.suggestions) > 0 {
			b.WriteString(RenderHelp(
				"↑/↓", "select suggestion",
				"enter/tab", "save",
				"esc", "cancel edit",
			))
		} else {
			b.WriteString(RenderHelp(
				"enter/tab", "save",
				"esc", "cancel edit",
			))
		}
	} else if m.addForm.focusIndex == 5 && !m.addForm.isFolder {
		// Files list focused
		if m.addForm.filesCursor < len(m.addForm.files) {
			b.WriteString(RenderHelp(
				"↑/k ↓/j", "navigate",
				"enter/e", "edit",
				"d/del", "remove",
				"esc", "back",
			))
		} else {
			b.WriteString(RenderHelp(
				"↑/k ↓/j", "navigate",
				"enter/e", "add file",
				"s", "save",
				"esc", "back",
			))
		}
	} else if m.addForm.focusIndex >= 0 && m.addForm.focusIndex <= 3 {
		// Text field focused (not editing)
		b.WriteString(RenderHelp(
			"↑/k ↓/j", "navigate",
			"enter/e", "edit",
			"s", "save",
			"esc", "back",
		))
	} else if m.addForm.focusIndex == 4 {
		// Toggle field focused
		b.WriteString(RenderHelp(
			"↑/k ↓/j", "navigate",
			"enter/space", "toggle",
			"s", "save",
			"esc", "back",
		))
	} else {
		b.WriteString(RenderHelp(
			"↑/k ↓/j", "navigate",
			"enter/e", "edit",
			"s", "save",
			"esc", "back",
		))
	}

	return BaseStyle.Render(b.String())
}

// renderSuggestions renders the autocomplete dropdown
func (m Model) renderSuggestions() string {
	if len(m.addForm.suggestions) == 0 {
		return ""
	}

	var b strings.Builder
	for i, suggestion := range m.addForm.suggestions {
		if i == m.addForm.suggestionCursor {
			b.WriteString(fmt.Sprintf("  %s\n", SelectedMenuItemStyle.Render(suggestion)))
		} else {
			b.WriteString(fmt.Sprintf("  %s\n", MutedTextStyle.Render(suggestion)))
		}
	}
	return b.String()
}

// saveNewPath validates the form and saves the new path to the config
func (m *Model) saveNewPath() error {
	name := strings.TrimSpace(m.addForm.nameInput.Value())
	linuxTarget := strings.TrimSpace(m.addForm.linuxTargetInput.Value())
	windowsTarget := strings.TrimSpace(m.addForm.windowsTargetInput.Value())
	backup := strings.TrimSpace(m.addForm.backupInput.Value())

	// Validate required fields
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if linuxTarget == "" && windowsTarget == "" {
		return fmt.Errorf("at least one target path is required")
	}
	if backup == "" {
		return fmt.Errorf("backup path is required")
	}

	// Check for duplicate names (skip the item being edited)
	for i, p := range m.Config.Paths {
		if p.Name == name && i != m.addForm.editIndex {
			return fmt.Errorf("a path with name '%s' already exists", name)
		}
	}

	// Create PathSpec with targets
	targets := make(map[string]string)
	if linuxTarget != "" {
		targets["linux"] = linuxTarget
	}
	if windowsTarget != "" {
		targets["windows"] = windowsTarget
	}

	// Get files from the list if in files mode
	var files []string
	if !m.addForm.isFolder {
		if len(m.addForm.files) == 0 {
			return fmt.Errorf("at least one file is required when using Files mode")
		}
		files = make([]string, len(m.addForm.files))
		copy(files, m.addForm.files)
	}

	newPath := config.PathSpec{
		Name:    name,
		Files:   files,
		Backup:  backup,
		Targets: targets,
	}

	// Editing existing path
	if m.addForm.editIndex >= 0 {
		// Update in config
		m.Config.Paths[m.addForm.editIndex] = newPath

		// Save config to file
		if err := config.Save(m.Config, m.ConfigPath); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		// Update the Paths slice in the model
		currentTarget := newPath.GetTarget(m.Platform.OS)
		if currentTarget != "" {
			m.Paths[m.addForm.editIndex] = PathItem{
				Spec:     newPath,
				Target:   currentTarget,
				Selected: true,
			}
		}

		// Refresh path states
		m.refreshPathStates()
		return nil
	}

	// Adding new path
	m.Config.Paths = append(m.Config.Paths, newPath)

	// Save config to file
	if err := config.Save(m.Config, m.ConfigPath); err != nil {
		// Remove the path we just added since save failed
		m.Config.Paths = m.Config.Paths[:len(m.Config.Paths)-1]
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Update the Paths slice in the model (only if current platform has a target)
	currentTarget := newPath.GetTarget(m.Platform.OS)
	if currentTarget != "" {
		m.Paths = append(m.Paths, PathItem{
			Spec:     newPath,
			Target:   currentTarget,
			Selected: true,
		})

		// Refresh path states
		m.refreshPathStates()
	}

	return nil
}
