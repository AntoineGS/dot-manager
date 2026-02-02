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

	descriptionInput := textinput.New()
	descriptionInput.Placeholder = "e.g., Neovim configuration"
	descriptionInput.CharLimit = 256
	descriptionInput.Width = 40

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

	repoInput := textinput.New()
	repoInput.Placeholder = "e.g., https://github.com/user/repo"
	repoInput.CharLimit = 256
	repoInput.Width = 40

	branchInput := textinput.New()
	branchInput.Placeholder = "e.g., main (optional)"
	branchInput.CharLimit = 64
	branchInput.Width = 40

	newFileInput := textinput.New()
	newFileInput.Placeholder = "e.g., .bashrc"
	newFileInput.CharLimit = 256
	newFileInput.Width = 40

	filterValueInput := textinput.New()
	filterValueInput.Placeholder = "e.g., linux or arch|ubuntu"
	filterValueInput.CharLimit = 128
	filterValueInput.Width = 40

	entryType := EntryTypeConfig
	isFolder := true
	isRoot := false
	var files []string
	var filters []FilterCondition

	// Populate with existing data if editing
	if editIndex >= 0 && editIndex < len(m.Paths) {
		pathItem := m.Paths[editIndex]
		entry := pathItem.Entry
		entryType = pathItem.EntryType

		nameInput.SetValue(entry.Name)
		descriptionInput.SetValue(entry.Description)
		isRoot = entry.Root

		if target, ok := entry.Targets["linux"]; ok {
			linuxTargetInput.SetValue(target)
		}
		if target, ok := entry.Targets["windows"]; ok {
			windowsTargetInput.SetValue(target)
		}

		if entryType == EntryTypeGit {
			repoInput.SetValue(entry.Repo)
			branchInput.SetValue(entry.Branch)
		} else {
			backupInput.SetValue(entry.Backup)
			isFolder = entry.IsFolder()
			if !isFolder {
				files = make([]string, len(entry.Files))
				copy(files, entry.Files)
			}
		}

		// Flatten filters into FilterCondition list
		for filterIdx, f := range entry.Filters {
			for k, v := range f.Include {
				filters = append(filters, FilterCondition{
					FilterIndex: filterIdx,
					IsExclude:   false,
					Key:         k,
					Value:       v,
				})
			}
			for k, v := range f.Exclude {
				filters = append(filters, FilterCondition{
					FilterIndex: filterIdx,
					IsExclude:   true,
					Key:         k,
					Value:       v,
				})
			}
		}
	}

	m.addForm = AddForm{
		entryType:           entryType,
		nameInput:           nameInput,
		descriptionInput:    descriptionInput,
		isRoot:              isRoot,
		linuxTargetInput:    linuxTargetInput,
		windowsTargetInput:  windowsTargetInput,
		backupInput:         backupInput,
		repoInput:           repoInput,
		branchInput:         branchInput,
		isFolder:            isFolder,
		focusIndex:          0,
		err:                 "",
		editIndex:           editIndex,
		editingField:        false,
		originalValue:       "",
		files:               files,
		filesCursor:         0,
		newFileInput:        newFileInput,
		addingFile:          false,
		editingFile:         false,
		editingFileIndex:    -1,
		filters:             filters,
		filtersCursor:       0,
		addingFilter:        false,
		editingFilter:       false,
		editingFilterIndex:  -1,
		filterAddStep:       0,
		filterIsExclude:     false,
		filterValueInput:    filterValueInput,
		filterKeyCursor:     0,
	}
}

// updateAddForm handles key events for the add form
func (m Model) updateAddForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// Handle editing a text field
	if m.addForm.editingField {
		return m.updateFieldInput(msg)
	}

	// Handle adding/editing file mode separately
	if m.addForm.addingFile || m.addForm.editingFile {
		return m.updateFileInput(msg)
	}

	// Handle adding/editing filter mode separately
	if m.addForm.addingFilter || m.addForm.editingFilter {
		return m.updateFilterInput(msg)
	}

	// Handle files list navigation when focused on files area
	if m.getFieldType() == fieldTypeFiles {
		return m.updateFilesList(msg)
	}

	// Handle filters list navigation when focused on filters area
	if m.getFieldType() == fieldTypeFilters {
		return m.updateFiltersList(msg)
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
		// Handle toggles
		ft := m.getFieldType()
		switch ft {
		case fieldTypeToggle:
			// Toggle entry type (config <-> git)
			if m.addForm.entryType == EntryTypeConfig {
				m.addForm.entryType = EntryTypeGit
			} else {
				m.addForm.entryType = EntryTypeConfig
			}
			return m, nil
		case fieldTypeIsFolder:
			m.addForm.isFolder = !m.addForm.isFolder
			return m, nil
		case fieldTypeIsRoot:
			m.addForm.isRoot = !m.addForm.isRoot
			return m, nil
		}

	case "enter", "e":
		// Enter edit mode for text fields
		if m.isTextInputField() {
			m.enterFieldEditMode()
			return m, nil
		}
		// Handle toggles on enter
		ft := m.getFieldType()
		switch ft {
		case fieldTypeToggle:
			if m.addForm.entryType == EntryTypeConfig {
				m.addForm.entryType = EntryTypeGit
			} else {
				m.addForm.entryType = EntryTypeConfig
			}
			return m, nil
		case fieldTypeIsFolder:
			m.addForm.isFolder = !m.addForm.isFolder
			return m, nil
		case fieldTypeIsRoot:
			m.addForm.isRoot = !m.addForm.isRoot
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
	ft := m.getFieldType()

	// Store original value and focus the input based on field type
	switch ft {
	case fieldTypeName:
		m.addForm.originalValue = m.addForm.nameInput.Value()
		m.addForm.nameInput.Focus()
		m.addForm.nameInput.SetCursor(len(m.addForm.nameInput.Value()))
	case fieldTypeDesc:
		m.addForm.originalValue = m.addForm.descriptionInput.Value()
		m.addForm.descriptionInput.Focus()
		m.addForm.descriptionInput.SetCursor(len(m.addForm.descriptionInput.Value()))
	case fieldTypeLinux:
		m.addForm.originalValue = m.addForm.linuxTargetInput.Value()
		m.addForm.linuxTargetInput.Focus()
		m.addForm.linuxTargetInput.SetCursor(len(m.addForm.linuxTargetInput.Value()))
	case fieldTypeWindows:
		m.addForm.originalValue = m.addForm.windowsTargetInput.Value()
		m.addForm.windowsTargetInput.Focus()
		m.addForm.windowsTargetInput.SetCursor(len(m.addForm.windowsTargetInput.Value()))
	case fieldTypeBackup:
		m.addForm.originalValue = m.addForm.backupInput.Value()
		m.addForm.backupInput.Focus()
		m.addForm.backupInput.SetCursor(len(m.addForm.backupInput.Value()))
	case fieldTypeRepo:
		m.addForm.originalValue = m.addForm.repoInput.Value()
		m.addForm.repoInput.Focus()
		m.addForm.repoInput.SetCursor(len(m.addForm.repoInput.Value()))
	case fieldTypeBranch:
		m.addForm.originalValue = m.addForm.branchInput.Value()
		m.addForm.branchInput.Focus()
		m.addForm.branchInput.SetCursor(len(m.addForm.branchInput.Value()))
	}
}

// updateFieldInput handles key events when editing a text field
func (m Model) updateFieldInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	ft := m.getFieldType()

	// Check for suggestions (only for path fields)
	isPathField := ft == fieldTypeLinux || ft == fieldTypeWindows || ft == fieldTypeBackup || ft == fieldTypeRepo
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

	// Handle text input for the focused field based on field type
	switch ft {
	case fieldTypeName:
		m.addForm.nameInput, cmd = m.addForm.nameInput.Update(msg)
	case fieldTypeDesc:
		m.addForm.descriptionInput, cmd = m.addForm.descriptionInput.Update(msg)
	case fieldTypeLinux:
		m.addForm.linuxTargetInput, cmd = m.addForm.linuxTargetInput.Update(msg)
	case fieldTypeWindows:
		m.addForm.windowsTargetInput, cmd = m.addForm.windowsTargetInput.Update(msg)
	case fieldTypeBackup:
		m.addForm.backupInput, cmd = m.addForm.backupInput.Update(msg)
	case fieldTypeRepo:
		m.addForm.repoInput, cmd = m.addForm.repoInput.Update(msg)
	case fieldTypeBranch:
		m.addForm.branchInput, cmd = m.addForm.branchInput.Update(msg)
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
	ft := m.getFieldType()
	switch ft {
	case fieldTypeName:
		m.addForm.nameInput.SetValue(m.addForm.originalValue)
	case fieldTypeDesc:
		m.addForm.descriptionInput.SetValue(m.addForm.originalValue)
	case fieldTypeLinux:
		m.addForm.linuxTargetInput.SetValue(m.addForm.originalValue)
	case fieldTypeWindows:
		m.addForm.windowsTargetInput.SetValue(m.addForm.originalValue)
	case fieldTypeBackup:
		m.addForm.backupInput.SetValue(m.addForm.originalValue)
	case fieldTypeRepo:
		m.addForm.repoInput.SetValue(m.addForm.originalValue)
	case fieldTypeBranch:
		m.addForm.branchInput.SetValue(m.addForm.originalValue)
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

// updateFiltersList handles key events when the filters list is focused
func (m Model) updateFiltersList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	// filtersCursor: 0 to len(filters)-1 for filter items, len(filters) for "Add Filter" button
	maxCursor := len(m.addForm.filters) // "Add Filter" button is at index len(filters)

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
		if m.addForm.filtersCursor > 0 {
			m.addForm.filtersCursor--
		} else {
			// Move to previous field
			m.addForm.focusIndex--
			m.updateAddFormFocus()
		}
		return m, nil

	case "down", "j":
		if m.addForm.filtersCursor < maxCursor {
			m.addForm.filtersCursor++
		} else {
			// Wrap to first field
			m.addForm.focusIndex = 0
			m.addForm.filtersCursor = 0
			m.updateAddFormFocus()
		}
		return m, nil

	case "tab":
		// Move to next field (wrap to beginning)
		m.addForm.focusIndex = 0
		m.addForm.filtersCursor = 0
		m.updateAddFormFocus()
		return m, nil

	case "shift+tab":
		// Move to previous field
		m.addForm.focusIndex--
		m.updateAddFormFocus()
		return m, nil

	case "enter", " ":
		// If on "Add Filter" button, start adding
		if m.addForm.filtersCursor == len(m.addForm.filters) {
			m.addForm.addingFilter = true
			m.addForm.filterAddStep = 0
			m.addForm.filterIsExclude = false
			m.addForm.filterKeyCursor = 0
			m.addForm.filterValueInput.SetValue("")
			return m, nil
		}
		// Edit the selected filter
		if m.addForm.filtersCursor < len(m.addForm.filters) {
			fc := m.addForm.filters[m.addForm.filtersCursor]
			m.addForm.editingFilter = true
			m.addForm.editingFilterIndex = m.addForm.filtersCursor
			m.addForm.filterAddStep = 2 // Start at value editing
			m.addForm.filterIsExclude = fc.IsExclude
			// Find key index
			for i, k := range filterKeys {
				if k == fc.Key {
					m.addForm.filterKeyCursor = i
					break
				}
			}
			m.addForm.filterValueInput.SetValue(fc.Value)
			m.addForm.filterValueInput.Focus()
		}
		return m, nil

	case "d", "backspace", "delete":
		// Delete the selected filter
		if m.addForm.filtersCursor < len(m.addForm.filters) && len(m.addForm.filters) > 0 {
			// Remove filter at cursor
			m.addForm.filters = append(m.addForm.filters[:m.addForm.filtersCursor], m.addForm.filters[m.addForm.filtersCursor+1:]...)
			// Adjust cursor if needed
			if m.addForm.filtersCursor >= len(m.addForm.filters) && m.addForm.filtersCursor > 0 {
				m.addForm.filtersCursor--
			}
		}
		return m, nil

	case "s", "ctrl+s":
		// Save the form
		if err := m.saveNewPath(); err != nil {
			m.addForm.err = err.Error()
			return m, nil
		}
		if m.addForm.editIndex >= 0 {
			m.Screen = ScreenResults
		} else {
			m.Screen = ScreenMenu
		}
		return m, nil
	}

	return m, nil
}

// updateFilterInput handles key events when adding or editing a filter
func (m Model) updateFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		// Cancel adding/editing filter
		m.addForm.addingFilter = false
		m.addForm.editingFilter = false
		m.addForm.editingFilterIndex = -1
		m.addForm.filterValueInput.SetValue("")
		return m, nil

	case "left", "h":
		// Navigate in step 0 (type) or step 1 (key)
		if m.addForm.filterAddStep == 0 {
			m.addForm.filterIsExclude = !m.addForm.filterIsExclude
		} else if m.addForm.filterAddStep == 1 {
			if m.addForm.filterKeyCursor > 0 {
				m.addForm.filterKeyCursor--
			}
		}
		return m, nil

	case "right", "l":
		// Navigate in step 0 (type) or step 1 (key)
		if m.addForm.filterAddStep == 0 {
			m.addForm.filterIsExclude = !m.addForm.filterIsExclude
		} else if m.addForm.filterAddStep == 1 {
			if m.addForm.filterKeyCursor < len(filterKeys)-1 {
				m.addForm.filterKeyCursor++
			}
		}
		return m, nil

	case "tab", "enter":
		// Move to next step or save
		if m.addForm.filterAddStep == 0 {
			// Move from type selection to key selection
			m.addForm.filterAddStep = 1
			return m, nil
		} else if m.addForm.filterAddStep == 1 {
			// Move from key selection to value input
			m.addForm.filterAddStep = 2
			m.addForm.filterValueInput.Focus()
			return m, nil
		} else if m.addForm.filterAddStep == 2 {
			// Save the filter
			value := strings.TrimSpace(m.addForm.filterValueInput.Value())
			if value == "" {
				return m, nil // Don't save empty value
			}

			key := filterKeys[m.addForm.filterKeyCursor]

			if m.addForm.editingFilter {
				// Update existing filter
				if m.addForm.editingFilterIndex >= 0 && m.addForm.editingFilterIndex < len(m.addForm.filters) {
					m.addForm.filters[m.addForm.editingFilterIndex] = FilterCondition{
						FilterIndex: m.addForm.filters[m.addForm.editingFilterIndex].FilterIndex,
						IsExclude:   m.addForm.filterIsExclude,
						Key:         key,
						Value:       value,
					}
				}
				m.addForm.editingFilter = false
				m.addForm.editingFilterIndex = -1
			} else {
				// Add new filter - determine filter index
				filterIndex := 0
				if len(m.addForm.filters) > 0 {
					// Use the same filter index as the last one (group conditions together)
					// Or increment for a new filter group
					filterIndex = m.addForm.filters[len(m.addForm.filters)-1].FilterIndex
				}
				m.addForm.filters = append(m.addForm.filters, FilterCondition{
					FilterIndex: filterIndex,
					IsExclude:   m.addForm.filterIsExclude,
					Key:         key,
					Value:       value,
				})
				m.addForm.filtersCursor = len(m.addForm.filters) // Move cursor to "Add Filter" button
				m.addForm.addingFilter = false
			}
			m.addForm.filterValueInput.SetValue("")
			return m, nil
		}

	case "shift+tab":
		// Move to previous step
		if m.addForm.filterAddStep > 0 {
			m.addForm.filterAddStep--
			if m.addForm.filterAddStep < 2 {
				m.addForm.filterValueInput.Blur()
			}
		}
		return m, nil
	}

	// Handle text input only in step 2 (value)
	if m.addForm.filterAddStep == 2 {
		m.addForm.filterValueInput, cmd = m.addForm.filterValueInput.Update(msg)
	}

	return m, cmd
}

// updateAddFormFocus updates which input field is focused
func (m *Model) updateAddFormFocus() {
	m.addForm.nameInput.Blur()
	m.addForm.descriptionInput.Blur()
	m.addForm.linuxTargetInput.Blur()
	m.addForm.windowsTargetInput.Blur()
	m.addForm.backupInput.Blur()
	m.addForm.repoInput.Blur()
	m.addForm.branchInput.Blur()
	m.addForm.newFileInput.Blur()

	ft := m.getFieldType()
	switch ft {
	case fieldTypeName:
		m.addForm.nameInput.Focus()
	case fieldTypeDesc:
		m.addForm.descriptionInput.Focus()
	case fieldTypeLinux:
		m.addForm.linuxTargetInput.Focus()
	case fieldTypeWindows:
		m.addForm.windowsTargetInput.Focus()
	case fieldTypeBackup:
		m.addForm.backupInput.Focus()
	case fieldTypeRepo:
		m.addForm.repoInput.Focus()
	case fieldTypeBranch:
		m.addForm.branchInput.Focus()
	// Toggles and files list don't have text input focus
	}
}

// addFormMaxIndex returns the maximum focus index based on entry type and state
// For new entries: type toggle is at index 0, shifting all fields by 1
// For editing: no type toggle, fields start at 0
func (m *Model) addFormMaxIndex() int {
	offset := 0
	if m.addForm.editIndex < 0 {
		offset = 1 // Add 1 for type toggle on new entries
	}

	if m.addForm.entryType == EntryTypeGit {
		// Git type: name, description, linuxTarget, windowsTarget, repo, branch, isRoot, filters
		return offset + 7
	}
	// Config type: name, description, linuxTarget, windowsTarget, backup, isFolder, isRoot, filters
	if m.addForm.isFolder {
		return offset + 7
	}
	// Config type with files: add files list, then filters
	return offset + 8
}

// addFormFieldType represents the type of field at a focus index
type addFormFieldType int

const (
	fieldTypeToggle    addFormFieldType = iota // Entry type toggle (new only)
	fieldTypeName                              // Name input
	fieldTypeDesc                              // Description input
	fieldTypeLinux                             // Linux target input
	fieldTypeWindows                           // Windows target input
	fieldTypeBackup                            // Backup path input (config)
	fieldTypeRepo                              // Repository URL input (git)
	fieldTypeBranch                            // Branch input (git)
	fieldTypeIsFolder                          // Folder/files toggle (config)
	fieldTypeIsRoot                            // Root toggle
	fieldTypeFiles                             // Files list (config, !isFolder)
	fieldTypeFilters                           // Filters list
)

// Filter attribute keys
var filterKeys = []string{"os", "distro", "hostname", "user"}

// getFieldType returns the field type at the current focus index
func (m *Model) getFieldType() addFormFieldType {
	idx := m.addForm.focusIndex
	isNew := m.addForm.editIndex < 0
	isGit := m.addForm.entryType == EntryTypeGit

	// Handle type toggle for new entries
	if isNew {
		if idx == 0 {
			return fieldTypeToggle
		}
		idx-- // Adjust for remaining fields
	}

	// Common fields: name(0), desc(1), linux(2), windows(3)
	switch idx {
	case 0:
		return fieldTypeName
	case 1:
		return fieldTypeDesc
	case 2:
		return fieldTypeLinux
	case 3:
		return fieldTypeWindows
	}

	// Type-specific fields
	if isGit {
		// Git type: 4=repo, 5=branch, 6=isRoot, 7=filters
		switch idx {
		case 4:
			return fieldTypeRepo
		case 5:
			return fieldTypeBranch
		case 6:
			return fieldTypeIsRoot
		case 7:
			return fieldTypeFilters
		}
	} else {
		// Config type visual order: backup, isFolder, [files if !isFolder], isRoot, filters
		if m.addForm.isFolder {
			// Folder mode: 4=backup, 5=isFolder, 6=isRoot, 7=filters
			switch idx {
			case 4:
				return fieldTypeBackup
			case 5:
				return fieldTypeIsFolder
			case 6:
				return fieldTypeIsRoot
			case 7:
				return fieldTypeFilters
			}
		} else {
			// Files mode: 4=backup, 5=isFolder, 6=files, 7=isRoot, 8=filters
			switch idx {
			case 4:
				return fieldTypeBackup
			case 5:
				return fieldTypeIsFolder
			case 6:
				return fieldTypeFiles
			case 7:
				return fieldTypeIsRoot
			case 8:
				return fieldTypeFilters
			}
		}
	}

	return fieldTypeName // fallback
}

// isTextInputField returns true if the current field is a text input
func (m *Model) isTextInputField() bool {
	ft := m.getFieldType()
	switch ft {
	case fieldTypeName, fieldTypeDesc, fieldTypeLinux, fieldTypeWindows,
		fieldTypeBackup, fieldTypeRepo, fieldTypeBranch:
		return true
	}
	return false
}

// isToggleField returns true if the current field is a toggle
func (m *Model) isToggleField() bool {
	ft := m.getFieldType()
	return ft == fieldTypeToggle || ft == fieldTypeIsFolder || ft == fieldTypeIsRoot
}

// updateSuggestions refreshes the autocomplete suggestions for the current path field
func (m *Model) updateSuggestions() {
	var input string
	var configDir string
	ft := m.getFieldType()

	// Get config directory for relative path resolution
	if m.ConfigPath != "" {
		configDir = filepath.Dir(m.ConfigPath)
	}

	switch ft {
	case fieldTypeLinux:
		input = m.addForm.linuxTargetInput.Value()
	case fieldTypeWindows:
		input = m.addForm.windowsTargetInput.Value()
	case fieldTypeBackup:
		input = m.addForm.backupInput.Value()
	case fieldTypeRepo:
		// No suggestions for repo URLs
		m.addForm.showSuggestions = false
		m.addForm.suggestions = nil
		return
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
	ft := m.getFieldType()

	switch ft {
	case fieldTypeLinux:
		m.addForm.linuxTargetInput.SetValue(suggestion)
		m.addForm.linuxTargetInput.SetCursor(len(suggestion))
	case fieldTypeWindows:
		m.addForm.windowsTargetInput.SetValue(suggestion)
		m.addForm.windowsTargetInput.SetCursor(len(suggestion))
	case fieldTypeBackup:
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

// viewAddForm renders the add form
func (m Model) viewAddForm() string {
	var b strings.Builder
	ft := m.getFieldType()
	isNew := m.addForm.editIndex < 0
	isGit := m.addForm.entryType == EntryTypeGit

	// Title
	if m.addForm.editIndex >= 0 {
		if isGit {
			b.WriteString(TitleStyle.Render("󰏫  Edit Git Entry"))
		} else {
			b.WriteString(TitleStyle.Render("󰏫  Edit Config Entry"))
		}
		b.WriteString("\n\n")
		b.WriteString(SubtitleStyle.Render("Edit the entry configuration"))
	} else {
		b.WriteString(TitleStyle.Render("󰐕  Add Entry"))
		b.WriteString("\n\n")
		b.WriteString(SubtitleStyle.Render("Add a new entry to your dotfiles configuration"))
	}
	b.WriteString("\n\n")

	// Entry type toggle (only for new entries)
	if isNew {
		typeLabel := "Entry Type:"
		if ft == fieldTypeToggle {
			typeLabel = HelpKeyStyle.Render("Entry Type:")
		}
		configCheck := "[ ]"
		gitCheck := "[✓]"
		if m.addForm.entryType == EntryTypeConfig {
			configCheck = "[✓]"
			gitCheck = "[ ]"
		}
		b.WriteString(fmt.Sprintf("  %s  %s Config  %s Git\n\n", typeLabel, configCheck, gitCheck))
	}

	// Name field
	nameLabel := "Name:"
	if ft == fieldTypeName {
		nameLabel = HelpKeyStyle.Render("Name:")
	}
	b.WriteString(fmt.Sprintf("  %s\n", nameLabel))
	b.WriteString(fmt.Sprintf("  %s\n\n", m.renderFieldValueByType(fieldTypeName, m.addForm.nameInput, "(empty)")))

	// Description field
	descLabel := "Description:"
	if ft == fieldTypeDesc {
		descLabel = HelpKeyStyle.Render("Description:")
	}
	b.WriteString(fmt.Sprintf("  %s\n", descLabel))
	b.WriteString(fmt.Sprintf("  %s\n\n", m.renderFieldValueByType(fieldTypeDesc, m.addForm.descriptionInput, "(optional)")))

	// Linux target field
	linuxTargetLabel := "Target (linux):"
	if ft == fieldTypeLinux {
		linuxTargetLabel = HelpKeyStyle.Render(linuxTargetLabel)
	}
	b.WriteString(fmt.Sprintf("  %s\n", linuxTargetLabel))
	b.WriteString(fmt.Sprintf("  %s\n", m.renderFieldValueByType(fieldTypeLinux, m.addForm.linuxTargetInput, "(empty)")))
	if m.addForm.editingField && ft == fieldTypeLinux && m.addForm.showSuggestions {
		b.WriteString(m.renderSuggestions())
	}
	b.WriteString("\n")

	// Windows target field
	windowsTargetLabel := "Target (windows):"
	if ft == fieldTypeWindows {
		windowsTargetLabel = HelpKeyStyle.Render(windowsTargetLabel)
	}
	b.WriteString(fmt.Sprintf("  %s\n", windowsTargetLabel))
	b.WriteString(fmt.Sprintf("  %s\n", m.renderFieldValueByType(fieldTypeWindows, m.addForm.windowsTargetInput, "(empty)")))
	if m.addForm.editingField && ft == fieldTypeWindows && m.addForm.showSuggestions {
		b.WriteString(m.renderSuggestions())
	}
	b.WriteString("\n")

	// Type-specific fields
	if isGit {
		// Repository field
		repoLabel := "Repository URL:"
		if ft == fieldTypeRepo {
			repoLabel = HelpKeyStyle.Render("Repository URL:")
		}
		b.WriteString(fmt.Sprintf("  %s\n", repoLabel))
		b.WriteString(fmt.Sprintf("  %s\n\n", m.renderFieldValueByType(fieldTypeRepo, m.addForm.repoInput, "(empty)")))

		// Branch field
		branchLabel := "Branch:"
		if ft == fieldTypeBranch {
			branchLabel = HelpKeyStyle.Render("Branch:")
		}
		b.WriteString(fmt.Sprintf("  %s\n", branchLabel))
		b.WriteString(fmt.Sprintf("  %s\n\n", m.renderFieldValueByType(fieldTypeBranch, m.addForm.branchInput, "(optional, defaults to default branch)")))
	} else {
		// Backup field
		backupLabel := "Backup path:"
		if ft == fieldTypeBackup {
			backupLabel = HelpKeyStyle.Render("Backup path:")
		}
		b.WriteString(fmt.Sprintf("  %s\n", backupLabel))
		b.WriteString(fmt.Sprintf("  %s\n", m.renderFieldValueByType(fieldTypeBackup, m.addForm.backupInput, "(empty)")))
		if m.addForm.editingField && ft == fieldTypeBackup && m.addForm.showSuggestions {
			b.WriteString(m.renderSuggestions())
		}
		b.WriteString("\n")

		// Is folder toggle
		toggleLabel := "Backup type:"
		if ft == fieldTypeIsFolder {
			toggleLabel = HelpKeyStyle.Render("Backup type:")
		}
		folderCheck := "[ ]"
		filesCheck := "[✓]"
		if m.addForm.isFolder {
			folderCheck = "[✓]"
			filesCheck = "[ ]"
		}
		b.WriteString(fmt.Sprintf("  %s  %s Folder  %s Files\n\n", toggleLabel, folderCheck, filesCheck))

		// Files list (only shown when Files mode is selected)
		if !m.addForm.isFolder {
			filesLabel := "Files:"
			if ft == fieldTypeFiles {
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
					} else if ft == fieldTypeFiles && !m.addForm.addingFile && !m.addForm.editingFile && m.addForm.filesCursor == i {
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
				if ft == fieldTypeFiles && m.addForm.filesCursor == len(m.addForm.files) {
					b.WriteString(fmt.Sprintf("    %s\n", SelectedMenuItemStyle.Render(addFileText)))
				} else {
					b.WriteString(fmt.Sprintf("    %s\n", MutedTextStyle.Render(addFileText)))
				}
			}
			b.WriteString("\n")
		}
	}

	// Root toggle
	rootLabel := "Root only:"
	if ft == fieldTypeIsRoot {
		rootLabel = HelpKeyStyle.Render("Root only:")
	}
	rootCheck := "[ ]"
	if m.addForm.isRoot {
		rootCheck = "[✓]"
	}
	b.WriteString(fmt.Sprintf("  %s  %s Yes\n\n", rootLabel, rootCheck))

	// Filters section
	filtersLabel := "Filters:"
	if ft == fieldTypeFilters {
		filtersLabel = HelpKeyStyle.Render("Filters:")
	}
	b.WriteString(fmt.Sprintf("  %s\n", filtersLabel))

	// Render filters based on state
	if m.addForm.addingFilter || m.addForm.editingFilter {
		// Show filter add/edit UI
		b.WriteString(m.renderFilterAddUI())
	} else {
		// Show filter list
		if len(m.addForm.filters) == 0 {
			b.WriteString(MutedTextStyle.Render("    (no filters)"))
			b.WriteString("\n")
		} else {
			for i, fc := range m.addForm.filters {
				prefix := "    "
				typeStr := "include"
				if fc.IsExclude {
					typeStr = "exclude"
				}
				condStr := fmt.Sprintf("%s: %s=%s", typeStr, fc.Key, fc.Value)

				if ft == fieldTypeFilters && m.addForm.filtersCursor == i {
					b.WriteString(fmt.Sprintf("%s%s\n", prefix, SelectedMenuItemStyle.Render("• "+condStr)))
				} else {
					b.WriteString(fmt.Sprintf("%s• %s\n", prefix, condStr))
				}
			}
		}

		// Add Filter button
		addFilterText := "[+ Add Filter]"
		if ft == fieldTypeFilters && m.addForm.filtersCursor == len(m.addForm.filters) {
			b.WriteString(fmt.Sprintf("    %s\n", SelectedMenuItemStyle.Render(addFilterText)))
		} else {
			b.WriteString(fmt.Sprintf("    %s\n", MutedTextStyle.Render(addFilterText)))
		}
	}
	b.WriteString("\n")

	// Error message
	if m.addForm.err != "" {
		b.WriteString(ErrorStyle.Render("  Error: " + m.addForm.err))
		b.WriteString("\n\n")
	}

	// Help - show context-sensitive help
	b.WriteString(m.renderAddFormHelp())

	return BaseStyle.Render(b.String())
}

// renderFilterAddUI renders the filter add/edit UI
func (m Model) renderFilterAddUI() string {
	var b strings.Builder

	actionText := "Add filter"
	if m.addForm.editingFilter {
		actionText = "Edit filter"
	}
	b.WriteString(fmt.Sprintf("    %s:\n", MutedTextStyle.Render(actionText)))

	// Step 0: Type selection (include/exclude)
	typeLabel := "    Type: "
	includeCheck := "[ ]"
	excludeCheck := "[✓]"
	if !m.addForm.filterIsExclude {
		includeCheck = "[✓]"
		excludeCheck = "[ ]"
	}
	typeStr := fmt.Sprintf("%s include  %s exclude", includeCheck, excludeCheck)
	if m.addForm.filterAddStep == 0 {
		b.WriteString(typeLabel + SelectedMenuItemStyle.Render(typeStr) + "\n")
	} else {
		b.WriteString(typeLabel + typeStr + "\n")
	}

	// Step 1: Key selection
	keyLabel := "    Key:  "
	var keyOptions []string
	for i, k := range filterKeys {
		if i == m.addForm.filterKeyCursor {
			keyOptions = append(keyOptions, "["+k+"]")
		} else {
			keyOptions = append(keyOptions, " "+k+" ")
		}
	}
	keyStr := strings.Join(keyOptions, " ")
	if m.addForm.filterAddStep == 1 {
		b.WriteString(keyLabel + SelectedMenuItemStyle.Render(keyStr) + "\n")
	} else {
		b.WriteString(keyLabel + keyStr + "\n")
	}

	// Step 2: Value input
	valueLabel := "    Value: "
	if m.addForm.filterAddStep == 2 {
		b.WriteString(valueLabel + m.addForm.filterValueInput.View() + "\n")
	} else {
		value := m.addForm.filterValueInput.Value()
		if value == "" {
			value = MutedTextStyle.Render("(enter value)")
		}
		b.WriteString(valueLabel + value + "\n")
	}

	return b.String()
}

// renderFieldValueByType renders a field value based on field type
func (m Model) renderFieldValueByType(fieldType addFormFieldType, input textinput.Model, placeholder string) string {
	currentFt := m.getFieldType()
	isEditing := m.addForm.editingField && currentFt == fieldType
	isFocused := currentFt == fieldType

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

// renderAddFormHelp renders context-sensitive help for the add form
func (m Model) renderAddFormHelp() string {
	ft := m.getFieldType()

	if m.addForm.addingFile {
		return RenderHelp(
			"enter", "add file",
			"esc", "cancel",
		)
	}
	if m.addForm.editingFile {
		return RenderHelp(
			"enter", "save",
			"esc", "cancel",
		)
	}
	if m.addForm.addingFilter || m.addForm.editingFilter {
		// Adding/editing a filter
		switch m.addForm.filterAddStep {
		case 0:
			return RenderHelp(
				"←/→", "select type",
				"tab/enter", "next",
				"esc", "cancel",
			)
		case 1:
			return RenderHelp(
				"←/→", "select key",
				"tab/enter", "next",
				"shift+tab", "back",
				"esc", "cancel",
			)
		case 2:
			return RenderHelp(
				"enter", "save filter",
				"shift+tab", "back",
				"esc", "cancel",
			)
		}
	}
	if m.addForm.editingField {
		// Editing a text field
		if m.addForm.showSuggestions && len(m.addForm.suggestions) > 0 && m.addForm.suggestionCursor >= 0 {
			return RenderHelp(
				"↑/↓", "select",
				"tab/enter", "accept",
				"esc", "cancel edit",
			)
		}
		if m.addForm.showSuggestions && len(m.addForm.suggestions) > 0 {
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
	if ft == fieldTypeFiles {
		// Files list focused
		if m.addForm.filesCursor < len(m.addForm.files) {
			return RenderHelp(
				"↑/k ↓/j", "navigate",
				"enter/e", "edit",
				"d/del", "remove",
				"esc", "back",
			)
		}
		return RenderHelp(
			"↑/k ↓/j", "navigate",
			"enter/e", "add file",
			"s", "save",
			"esc", "back",
		)
	}
	if ft == fieldTypeFilters {
		// Filters list focused
		if m.addForm.filtersCursor < len(m.addForm.filters) {
			return RenderHelp(
				"↑/k ↓/j", "navigate",
				"enter", "edit",
				"d/del", "remove",
				"s", "save",
				"esc", "back",
			)
		}
		return RenderHelp(
			"↑/k ↓/j", "navigate",
			"enter", "add filter",
			"s", "save",
			"esc", "back",
		)
	}
	if m.isTextInputField() {
		// Text field focused (not editing)
		return RenderHelp(
			"↑/k ↓/j", "navigate",
			"enter/e", "edit",
			"s", "save",
			"esc", "back",
		)
	}
	if m.isToggleField() {
		// Toggle field focused
		return RenderHelp(
			"↑/k ↓/j", "navigate",
			"enter/space", "toggle",
			"s", "save",
			"esc", "back",
		)
	}
	return RenderHelp(
		"↑/k ↓/j", "navigate",
		"enter/e", "edit",
		"s", "save",
		"esc", "back",
	)
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

// saveNewPath validates the form and saves the new entry to the config
func (m *Model) saveNewPath() error {
	name := strings.TrimSpace(m.addForm.nameInput.Value())
	description := strings.TrimSpace(m.addForm.descriptionInput.Value())
	linuxTarget := strings.TrimSpace(m.addForm.linuxTargetInput.Value())
	windowsTarget := strings.TrimSpace(m.addForm.windowsTargetInput.Value())
	isGit := m.addForm.entryType == EntryTypeGit

	// Validate required fields
	if name == "" {
		return fmt.Errorf("name is required")
	}
	if linuxTarget == "" && windowsTarget == "" {
		return fmt.Errorf("at least one target path is required")
	}

	// Type-specific validation
	var backup, repo, branch string
	var files []string

	if isGit {
		repo = strings.TrimSpace(m.addForm.repoInput.Value())
		branch = strings.TrimSpace(m.addForm.branchInput.Value())
		if repo == "" {
			return fmt.Errorf("repository URL is required for git entries")
		}
	} else {
		backup = strings.TrimSpace(m.addForm.backupInput.Value())
		if backup == "" {
			return fmt.Errorf("backup path is required for config entries")
		}
		if !m.addForm.isFolder {
			if len(m.addForm.files) == 0 {
				return fmt.Errorf("at least one file is required when using Files mode")
			}
			files = make([]string, len(m.addForm.files))
			copy(files, m.addForm.files)
		}
	}

	// Check for duplicate names (skip the item being edited)
	for i, e := range m.Config.Entries {
		if e.Name == name && i != m.findConfigEntryIndex(m.addForm.editIndex) {
			return fmt.Errorf("an entry with name '%s' already exists", name)
		}
	}

	// Create targets map
	targets := make(map[string]string)
	if linuxTarget != "" {
		targets["linux"] = linuxTarget
	}
	if windowsTarget != "" {
		targets["windows"] = windowsTarget
	}

	// Convert filter conditions back to config.Filter format
	filters := m.buildFiltersFromConditions()

	newEntry := config.Entry{
		Name:        name,
		Description: description,
		Root:        m.addForm.isRoot,
		Targets:     targets,
		Filters:     filters,
	}

	if isGit {
		newEntry.Repo = repo
		newEntry.Branch = branch
	} else {
		newEntry.Backup = backup
		newEntry.Files = files
	}

	// Editing existing entry
	if m.addForm.editIndex >= 0 {
		configIdx := m.findConfigEntryIndex(m.addForm.editIndex)
		if configIdx >= 0 {
			// Preserve package info if it exists
			if m.Config.Entries[configIdx].Package != nil {
				newEntry.Package = m.Config.Entries[configIdx].Package
			}

			m.Config.Entries[configIdx] = newEntry
		}

		// Save config to file
		if err := config.Save(m.Config, m.ConfigPath); err != nil {
			return fmt.Errorf("failed to save config: %w", err)
		}

		// Update the Paths slice in the model
		currentTarget := newEntry.GetTarget(m.Platform.OS)
		if currentTarget != "" {
			m.Paths[m.addForm.editIndex] = PathItem{
				Entry:     newEntry,
				Target:    currentTarget,
				Selected:  true,
				EntryType: m.addForm.entryType,
			}
		}

		// Refresh path states
		m.refreshPathStates()
		return nil
	}

	// Adding new entry
	m.Config.Entries = append(m.Config.Entries, newEntry)

	// Save config to file
	if err := config.Save(m.Config, m.ConfigPath); err != nil {
		// Remove the entry we just added since save failed
		m.Config.Entries = m.Config.Entries[:len(m.Config.Entries)-1]
		return fmt.Errorf("failed to save config: %w", err)
	}

	// Update the Paths slice in the model (only if current platform has a target)
	currentTarget := newEntry.GetTarget(m.Platform.OS)
	if currentTarget != "" {
		m.Paths = append(m.Paths, PathItem{
			Entry:     newEntry,
			Target:    currentTarget,
			Selected:  true,
			EntryType: m.addForm.entryType,
		})

		// Refresh path states
		m.refreshPathStates()
	}

	return nil
}

// buildFiltersFromConditions converts the flat FilterCondition list back to config.Filter format
func (m *Model) buildFiltersFromConditions() []config.Filter {
	if len(m.addForm.filters) == 0 {
		return nil
	}

	// Group conditions by filter index
	filterMap := make(map[int]*config.Filter)
	maxIndex := 0

	for _, fc := range m.addForm.filters {
		if fc.FilterIndex > maxIndex {
			maxIndex = fc.FilterIndex
		}
		if _, ok := filterMap[fc.FilterIndex]; !ok {
			filterMap[fc.FilterIndex] = &config.Filter{
				Include: make(map[string]string),
				Exclude: make(map[string]string),
			}
		}
		if fc.IsExclude {
			filterMap[fc.FilterIndex].Exclude[fc.Key] = fc.Value
		} else {
			filterMap[fc.FilterIndex].Include[fc.Key] = fc.Value
		}
	}

	// Build result slice in order
	var result []config.Filter
	for i := 0; i <= maxIndex; i++ {
		if f, ok := filterMap[i]; ok {
			// Only include non-empty filters
			if len(f.Include) > 0 || len(f.Exclude) > 0 {
				result = append(result, *f)
			}
		}
	}

	return result
}

// findConfigEntryIndex finds the config entry index corresponding to a Paths slice index
func (m *Model) findConfigEntryIndex(pathsIndex int) int {
	if pathsIndex < 0 || pathsIndex >= len(m.Paths) {
		return -1
	}
	entryName := m.Paths[pathsIndex].Entry.Name
	for i, e := range m.Config.Entries {
		if e.Name == entryName {
			return i
		}
	}
	return -1
}
