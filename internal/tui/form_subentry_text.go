package tui

import (
	"path/filepath"
	"strings"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
)

// updateSubEntryFieldInput handles key events when editing a text field
func (m Model) updateSubEntryFieldInput(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if m.subEntryForm == nil {
		return m, nil
	}

	var cmd tea.Cmd
	ft := m.getSubEntryFieldType()

	// Check for suggestions (only for path fields)
	isPathField := ft == subFieldLinux || ft == subFieldWindows || ft == subFieldBackup
	hasSuggestions := m.subEntryForm.ShowSuggestions && len(m.subEntryForm.Suggestions) > 0
	hasSelectedSuggestion := hasSuggestions && m.subEntryForm.SuggestionCursor >= 0

	if m, cmd, handled := m.handleTextEditKeys(msg); handled {
		return m, cmd
	}

	switch {
	case key.Matches(msg, TextEditKeys.Cancel):
		// If suggestions are showing, close them first
		if hasSuggestions {
			m.subEntryForm.ShowSuggestions = false
			return m, nil
		}
		// Cancel editing and restore original value
		m.cancelSubEntryFieldEdit()

		return m, nil

	case key.Matches(msg, SearchKeys.Confirm) || key.Matches(msg, TextEditKeys.SaveForm):
		// Accept suggestion only if user has explicitly selected one
		if hasSelectedSuggestion {
			m.acceptSuggestionSubEntry()
			return m, nil
		}
		// Save and exit edit mode
		m.subEntryForm.EditingField = false
		m.subEntryForm.ShowSuggestions = false
		m.updateSubEntryFormFocus()

		return m, nil

	case key.Matches(msg, FormNavKeys.TabNext):
		// Accept suggestion if selected
		if hasSelectedSuggestion {
			m.acceptSuggestionSubEntry()
			return m, nil
		}
		// Save and exit edit mode
		m.subEntryForm.EditingField = false
		m.subEntryForm.ShowSuggestions = false
		m.updateSubEntryFormFocus()

		return m, nil

	case key.Matches(msg, SuggestionKeys.Up):
		// Navigate suggestions if showing
		if hasSuggestions {
			if m.subEntryForm.SuggestionCursor < 0 {
				m.subEntryForm.SuggestionCursor = len(m.subEntryForm.Suggestions) - 1
			} else {
				m.subEntryForm.SuggestionCursor--
			}

			return m, nil
		}

	case key.Matches(msg, SuggestionKeys.Down):
		// Navigate suggestions if showing
		if hasSuggestions {
			if m.subEntryForm.SuggestionCursor < 0 {
				m.subEntryForm.SuggestionCursor = 0
			} else {
				m.subEntryForm.SuggestionCursor++
				if m.subEntryForm.SuggestionCursor >= len(m.subEntryForm.Suggestions) {
					m.subEntryForm.SuggestionCursor = 0
				}
			}

			return m, nil
		}
	}

	// Handle text input for the focused field
	switch ft {
	case subFieldName:
		m.subEntryForm.NameInput, cmd = m.subEntryForm.NameInput.Update(msg)
	case subFieldLinux:
		m.subEntryForm.LinuxTargetInput, cmd = m.subEntryForm.LinuxTargetInput.Update(msg)
	case subFieldWindows:
		m.subEntryForm.WindowsTargetInput, cmd = m.subEntryForm.WindowsTargetInput.Update(msg)
	case subFieldBackup:
		m.subEntryForm.BackupInput, cmd = m.subEntryForm.BackupInput.Update(msg)
	case subFieldLinuxCheck:
		m.subEntryForm.LinuxCheckInput, cmd = m.subEntryForm.LinuxCheckInput.Update(msg)
	case subFieldLinuxRun:
		m.subEntryForm.LinuxRunInput, cmd = m.subEntryForm.LinuxRunInput.Update(msg)
	case subFieldWindowsCheck:
		m.subEntryForm.WindowsCheckInput, cmd = m.subEntryForm.WindowsCheckInput.Update(msg)
	case subFieldWindowsRun:
		m.subEntryForm.WindowsRunInput, cmd = m.subEntryForm.WindowsRunInput.Update(msg)
	case subFieldIsSetup, subFieldWhen, subFieldIsFolder, subFieldFiles, subFieldIsSudo, subFieldIsCopy, subFieldCheckMode:
		// Boolean and list fields don't use text input
	}

	// Update suggestions for path fields after text changes
	if isPathField {
		m.updateSuggestionsSubEntry()
	}

	// Clear error when typing
	m.subEntryForm.Err = ""

	return m, cmd
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
		input = m.subEntryForm.LinuxTargetInput.Value()
	case subFieldWindows:
		input = m.subEntryForm.WindowsTargetInput.Value()
	case subFieldBackup:
		input = m.subEntryForm.BackupInput.Value()
	case subFieldName, subFieldWhen, subFieldIsSetup, subFieldIsFolder, subFieldFiles, subFieldIsSudo, subFieldIsCopy, subFieldCheckMode,
		subFieldLinuxCheck, subFieldLinuxRun, subFieldWindowsCheck, subFieldWindowsRun:
		m.subEntryForm.ShowSuggestions = false
		m.subEntryForm.Suggestions = nil
		return
	default:
		m.subEntryForm.ShowSuggestions = false
		m.subEntryForm.Suggestions = nil

		return
	}

	suggestions := getPathSuggestions(input, configDir)
	m.subEntryForm.Suggestions = suggestions
	m.subEntryForm.SuggestionCursor = -1 // No selection until user uses arrows
	m.subEntryForm.ShowSuggestions = len(suggestions) > 0
}

// acceptSuggestionSubEntry fills in the selected suggestion
func (m *Model) acceptSuggestionSubEntry() {
	if m.subEntryForm == nil || len(m.subEntryForm.Suggestions) == 0 {
		return
	}

	suggestion := m.subEntryForm.Suggestions[m.subEntryForm.SuggestionCursor]
	ft := m.getSubEntryFieldType()

	switch ft {
	case subFieldLinux:
		m.subEntryForm.LinuxTargetInput.SetValue(suggestion)
		m.subEntryForm.LinuxTargetInput.SetCursor(len(suggestion))
	case subFieldWindows:
		m.subEntryForm.WindowsTargetInput.SetValue(suggestion)
		m.subEntryForm.WindowsTargetInput.SetCursor(len(suggestion))
	case subFieldBackup:
		m.subEntryForm.BackupInput.SetValue(suggestion)
		m.subEntryForm.BackupInput.SetCursor(len(suggestion))
	case subFieldIsFolder, subFieldFiles, subFieldIsSudo, subFieldIsCopy, subFieldCheckMode, subFieldName:
		// Other fields don't use suggestions
	}

	// Keep suggestions open for continued navigation if it's a directory
	if strings.HasSuffix(suggestion, "/") {
		m.updateSuggestionsSubEntry()
	} else {
		m.subEntryForm.ShowSuggestions = false
		m.subEntryForm.Suggestions = nil
	}
}
