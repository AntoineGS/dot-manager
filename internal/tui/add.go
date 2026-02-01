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

	m.addForm = AddForm{
		nameInput:          nameInput,
		linuxTargetInput:   linuxTargetInput,
		windowsTargetInput: windowsTargetInput,
		backupInput:        backupInput,
		isFolder:           true,
		focusIndex:         0,
		err:                "",
	}
}

// updateAddForm handles key events for the add form
func (m Model) updateAddForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd

	// Check if we're in a path field with suggestions showing
	isPathField := m.addForm.focusIndex >= 1 && m.addForm.focusIndex <= 3
	hasSuggestions := m.addForm.showSuggestions && len(m.addForm.suggestions) > 0

	switch msg.String() {
	case "ctrl+c":
		return m, tea.Quit

	case "esc":
		// If suggestions are showing, close them first
		if hasSuggestions {
			m.addForm.showSuggestions = false
			return m, nil
		}
		m.Screen = ScreenMenu
		return m, nil

	case "down":
		// Navigate suggestions if showing
		if hasSuggestions {
			m.addForm.suggestionCursor++
			if m.addForm.suggestionCursor >= len(m.addForm.suggestions) {
				m.addForm.suggestionCursor = 0
			}
			return m, nil
		}
		// Otherwise move to next field
		m.addForm.showSuggestions = false
		m.addForm.focusIndex++
		if m.addForm.focusIndex > 4 {
			m.addForm.focusIndex = 0
		}
		m.updateAddFormFocus()
		m.updateSuggestions()
		return m, nil

	case "up":
		// Navigate suggestions if showing
		if hasSuggestions {
			m.addForm.suggestionCursor--
			if m.addForm.suggestionCursor < 0 {
				m.addForm.suggestionCursor = len(m.addForm.suggestions) - 1
			}
			return m, nil
		}
		// Otherwise move to previous field
		m.addForm.showSuggestions = false
		m.addForm.focusIndex--
		if m.addForm.focusIndex < 0 {
			m.addForm.focusIndex = 4
		}
		m.updateAddFormFocus()
		m.updateSuggestions()
		return m, nil

	case "tab":
		// Accept suggestion if showing
		if hasSuggestions {
			m.acceptSuggestion()
			return m, nil
		}
		// Otherwise move to next field
		m.addForm.showSuggestions = false
		m.addForm.focusIndex++
		if m.addForm.focusIndex > 4 {
			m.addForm.focusIndex = 0
		}
		m.updateAddFormFocus()
		m.updateSuggestions()
		return m, nil

	case "shift+tab":
		// Move to previous field
		m.addForm.showSuggestions = false
		m.addForm.focusIndex--
		if m.addForm.focusIndex < 0 {
			m.addForm.focusIndex = 4
		}
		m.updateAddFormFocus()
		m.updateSuggestions()
		return m, nil

	case " ":
		// Toggle isFolder if on the toggle field
		if m.addForm.focusIndex == 4 {
			m.addForm.isFolder = !m.addForm.isFolder
			return m, nil
		}

	case "enter":
		// Accept suggestion if showing
		if hasSuggestions {
			m.acceptSuggestion()
			return m, nil
		}
		// If on toggle field, toggle it
		if m.addForm.focusIndex == 4 {
			m.addForm.isFolder = !m.addForm.isFolder
			return m, nil
		}
		// Otherwise try to save
		if err := m.saveNewPath(); err != nil {
			m.addForm.err = err.Error()
			return m, nil
		}
		// Success - go back to menu
		m.Screen = ScreenMenu
		return m, nil
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

// updateAddFormFocus updates which input field is focused
func (m *Model) updateAddFormFocus() {
	m.addForm.nameInput.Blur()
	m.addForm.linuxTargetInput.Blur()
	m.addForm.windowsTargetInput.Blur()
	m.addForm.backupInput.Blur()

	switch m.addForm.focusIndex {
	case 0:
		m.addForm.nameInput.Focus()
	case 1:
		m.addForm.linuxTargetInput.Focus()
	case 2:
		m.addForm.windowsTargetInput.Focus()
	case 3:
		m.addForm.backupInput.Focus()
	}
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
	m.addForm.suggestionCursor = 0
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

// viewAddForm renders the add form
func (m Model) viewAddForm() string {
	var b strings.Builder

	// Title
	title := TitleStyle.Render("󰐕  Add Path Configuration")
	b.WriteString(title)
	b.WriteString("\n\n")

	// Instructions
	b.WriteString(SubtitleStyle.Render("Add a new path to your dotfiles configuration"))
	b.WriteString("\n\n")

	// Name field
	nameLabel := "Name:"
	if m.addForm.focusIndex == 0 {
		nameLabel = HelpKeyStyle.Render("Name:")
	}
	b.WriteString(fmt.Sprintf("  %s\n", nameLabel))
	b.WriteString(fmt.Sprintf("  %s\n\n", m.addForm.nameInput.View()))

	// Linux target field
	linuxTargetLabel := "Target (linux):"
	if m.addForm.focusIndex == 1 {
		linuxTargetLabel = HelpKeyStyle.Render(linuxTargetLabel)
	}
	b.WriteString(fmt.Sprintf("  %s\n", linuxTargetLabel))
	b.WriteString(fmt.Sprintf("  %s\n", m.addForm.linuxTargetInput.View()))
	if m.addForm.focusIndex == 1 && m.addForm.showSuggestions {
		b.WriteString(m.renderSuggestions())
	}
	b.WriteString("\n")

	// Windows target field
	windowsTargetLabel := "Target (windows):"
	if m.addForm.focusIndex == 2 {
		windowsTargetLabel = HelpKeyStyle.Render(windowsTargetLabel)
	}
	b.WriteString(fmt.Sprintf("  %s\n", windowsTargetLabel))
	b.WriteString(fmt.Sprintf("  %s\n", m.addForm.windowsTargetInput.View()))
	if m.addForm.focusIndex == 2 && m.addForm.showSuggestions {
		b.WriteString(m.renderSuggestions())
	}
	b.WriteString("\n")

	// Backup field
	backupLabel := "Backup path:"
	if m.addForm.focusIndex == 3 {
		backupLabel = HelpKeyStyle.Render("Backup path:")
	}
	b.WriteString(fmt.Sprintf("  %s\n", backupLabel))
	b.WriteString(fmt.Sprintf("  %s\n", m.addForm.backupInput.View()))
	if m.addForm.focusIndex == 3 && m.addForm.showSuggestions {
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

	// Error message
	if m.addForm.err != "" {
		b.WriteString(ErrorStyle.Render("  Error: " + m.addForm.err))
		b.WriteString("\n\n")
	}

	// Help - show different help when suggestions are visible
	if m.addForm.showSuggestions && len(m.addForm.suggestions) > 0 {
		b.WriteString(RenderHelp(
			"↑/↓", "select",
			"tab/enter", "accept",
			"esc", "close",
		))
	} else {
		b.WriteString(RenderHelp(
			"tab/↓", "next field",
			"shift+tab/↑", "prev field",
			"enter", "save",
			"esc", "cancel",
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

	// Check for duplicate names
	for _, p := range m.Config.Paths {
		if p.Name == name {
			return fmt.Errorf("a path with name '%s' already exists", name)
		}
	}

	// Create new PathSpec with targets
	targets := make(map[string]string)
	if linuxTarget != "" {
		targets["linux"] = linuxTarget
	}
	if windowsTarget != "" {
		targets["windows"] = windowsTarget
	}

	newPath := config.PathSpec{
		Name:    name,
		Backup:  backup,
		Targets: targets,
	}

	// For files mode, we'd need file names - for now only support folder mode
	if !m.addForm.isFolder {
		return fmt.Errorf("file mode not yet supported in TUI - use folder mode or edit config directly")
	}

	// Add to config
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
