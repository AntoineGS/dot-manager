package tui

import (
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

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

	if m, cmd, handled := m.handleTextEditKeys(msg); handled {
		return m, cmd
	}

	switch {
	case key.Matches(msg, TextEditKeys.Cancel):
		// If suggestions are showing, close them first
		if hasSuggestions {
			m.subEntryForm.showSuggestions = false
			return m, nil
		}
		// Cancel editing and restore original value
		m.cancelSubEntryFieldEdit()

		return m, nil

	case key.Matches(msg, SearchKeys.Confirm):
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

	case key.Matches(msg, FormNavKeys.TabNext):
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

	case key.Matches(msg, SuggestionKeys.Up):
		// Navigate suggestions if showing
		if hasSuggestions {
			if m.subEntryForm.suggestionCursor < 0 {
				m.subEntryForm.suggestionCursor = len(m.subEntryForm.suggestions) - 1
			} else {
				m.subEntryForm.suggestionCursor--
			}

			return m, nil
		}

	case key.Matches(msg, SuggestionKeys.Down):
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
