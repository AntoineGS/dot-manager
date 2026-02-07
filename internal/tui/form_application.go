package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// applicationFieldType represents the type of field in the ApplicationForm
type applicationFieldType int

const (
	appFieldName applicationFieldType = iota
	appFieldDescription
	appFieldPackages
	appFieldFilters
)

// initApplicationFormNew initializes the form for creating a new application
func (m *Model) initApplicationFormNew() {
	nameInput := textinput.New()
	nameInput.Placeholder = PlaceholderNeovim
	nameInput.Focus()
	nameInput.CharLimit = 64
	nameInput.Width = 40

	descriptionInput := textinput.New()
	descriptionInput.Placeholder = "e.g., Neovim text editor"
	descriptionInput.CharLimit = 256
	descriptionInput.Width = 40

	filterValueInput := textinput.New()
	filterValueInput.Placeholder = "e.g., linux or arch|ubuntu"
	filterValueInput.CharLimit = 128
	filterValueInput.Width = 40

	packageNameInput := textinput.New()
	packageNameInput.Placeholder = PlaceholderNeovim
	packageNameInput.CharLimit = 128
	packageNameInput.Width = 40

	gitURLInput := textinput.New()
	gitURLInput.Placeholder = PlaceholderGitURL
	gitURLInput.CharLimit = 256
	gitURLInput.Width = 40

	gitBranchInput := textinput.New()
	gitBranchInput.Placeholder = PlaceholderGitBranch
	gitBranchInput.CharLimit = 128
	gitBranchInput.Width = 40

	gitLinuxInput := textinput.New()
	gitLinuxInput.Placeholder = PlaceholderGitLinux
	gitLinuxInput.CharLimit = 256
	gitLinuxInput.Width = 40

	gitWindowsInput := textinput.New()
	gitWindowsInput.Placeholder = PlaceholderGitWindows
	gitWindowsInput.CharLimit = 256
	gitWindowsInput.Width = 40

	m.applicationForm = &ApplicationForm{
		nameInput:          nameInput,
		descriptionInput:   descriptionInput,
		packageManagers:    make(map[string]string),
		packagesCursor:     0,
		editingPackage:     false,
		packageNameInput:   packageNameInput,
		lastPackageName:    "",
		filters:            nil,
		filtersCursor:      0,
		addingFilter:       false,
		editingFilter:      false,
		editingFilterIndex: -1,
		filterAddStep:      0,
		filterIsExclude:    false,
		filterValueInput:   filterValueInput,
		filterKeyCursor:    0,
		focusIndex:         0,
		editingField:       false,
		originalValue:      "",
		editAppIdx:         -1,
		err:                "",
		gitURLInput:        gitURLInput,
		gitBranchInput:     gitBranchInput,
		gitLinuxInput:      gitLinuxInput,
		gitWindowsInput:    gitWindowsInput,
		gitFieldCursor:     -1,
		hasGitPackage:      false,
		gitSudo:            false,
	}

	m.activeForm = FormApplication
	m.Screen = ScreenAddForm
}

// initApplicationFormEdit initializes the form for editing an existing application
func (m *Model) initApplicationFormEdit(appIdx int) {
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

	nameInput := textinput.New()
	nameInput.Placeholder = PlaceholderNeovim
	nameInput.SetValue(app.Name)
	nameInput.Focus()
	nameInput.CharLimit = 64
	nameInput.Width = 40

	descriptionInput := textinput.New()
	descriptionInput.Placeholder = "e.g., Neovim text editor"
	descriptionInput.SetValue(app.Description)
	descriptionInput.CharLimit = 256
	descriptionInput.Width = 40

	filterValueInput := textinput.New()
	filterValueInput.Placeholder = "e.g., linux or arch|ubuntu"
	filterValueInput.CharLimit = 128
	filterValueInput.Width = 40

	packageNameInput := textinput.New()
	packageNameInput.Placeholder = PlaceholderNeovim
	packageNameInput.CharLimit = 128
	packageNameInput.Width = 40

	gitURLInput := textinput.New()
	gitURLInput.Placeholder = PlaceholderGitURL
	gitURLInput.CharLimit = 256
	gitURLInput.Width = 40

	gitBranchInput := textinput.New()
	gitBranchInput.Placeholder = PlaceholderGitBranch
	gitBranchInput.CharLimit = 128
	gitBranchInput.Width = 40

	gitLinuxInput := textinput.New()
	gitLinuxInput.Placeholder = PlaceholderGitLinux
	gitLinuxInput.CharLimit = 256
	gitLinuxInput.Width = 40

	gitWindowsInput := textinput.New()
	gitWindowsInput.Placeholder = PlaceholderGitWindows
	gitWindowsInput.CharLimit = 256
	gitWindowsInput.Width = 40

	// Load filters
	var filters []FilterCondition
	for filterIdx, f := range app.Filters {
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

	// Load package managers (only string-based managers, skip git)
	packageManagers := make(map[string]string)
	if app.Package != nil && len(app.Package.Managers) > 0 {
		for k, v := range app.Package.Managers {
			// Skip git packages as they require special handling
			if k == TypeGit {
				continue
			}
			if !v.IsGit() {
				packageManagers[k] = v.PackageName
			}
		}
	}

	// Load git package if present
	hasGitPackage := false
	gitSudo := false

	if app.Package != nil {
		if gitVal, ok := app.Package.Managers[TypeGit]; ok && gitVal.IsGit() {
			hasGitPackage = true
			gitURLInput.SetValue(gitVal.Git.URL)
			gitBranchInput.SetValue(gitVal.Git.Branch)
			gitSudo = gitVal.Git.Sudo

			if target, ok := gitVal.Git.Targets[OSLinux]; ok {
				gitLinuxInput.SetValue(target)
			}
			if target, ok := gitVal.Git.Targets[OSWindows]; ok {
				gitWindowsInput.SetValue(target)
			}
		}
	}

	m.applicationForm = &ApplicationForm{
		nameInput:          nameInput,
		descriptionInput:   descriptionInput,
		packageManagers:    packageManagers,
		packagesCursor:     0,
		editingPackage:     false,
		packageNameInput:   packageNameInput,
		lastPackageName:    "",
		filters:            filters,
		filtersCursor:      0,
		addingFilter:       false,
		editingFilter:      false,
		editingFilterIndex: -1,
		filterAddStep:      0,
		filterIsExclude:    false,
		filterValueInput:   filterValueInput,
		filterKeyCursor:    0,
		focusIndex:         0,
		editingField:       false,
		originalValue:      "",
		editAppIdx:         configAppIdx,
		err:                "",
		gitURLInput:        gitURLInput,
		gitBranchInput:     gitBranchInput,
		gitLinuxInput:      gitLinuxInput,
		gitWindowsInput:    gitWindowsInput,
		gitFieldCursor:     -1,
		hasGitPackage:      hasGitPackage,
		gitSudo:            gitSudo,
	}

	m.activeForm = FormApplication
	m.Screen = ScreenAddForm
}

// getApplicationFieldType returns the field type at the current focus index
func (m *Model) getApplicationFieldType() applicationFieldType {
	if m.applicationForm == nil {
		return appFieldName
	}

	switch m.applicationForm.focusIndex {
	case 0:
		return appFieldName
	case 1:
		return appFieldDescription
	case 2:
		return appFieldPackages
	case 3:
		return appFieldFilters
	default:
		return appFieldName
	}
}

// updateApplicationForm handles key events for the application form
func (m Model) updateApplicationForm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.applicationForm == nil {
		return m, nil
	}

	// Handle editing a git text field
	if m.applicationForm.editingGitField {
		return m.updateApplicationGitFieldInput(msg)
	}

	// Handle editing a text field
	if m.applicationForm.editingField {
		return m.updateApplicationFieldInput(msg)
	}

	// Handle editing package name
	if m.applicationForm.editingPackage {
		return m.updateApplicationPackageInput(msg)
	}

	// Handle adding/editing filter
	if m.applicationForm.addingFilter || m.applicationForm.editingFilter {
		return m.updateApplicationFilterInput(msg)
	}

	// Handle packages list navigation
	if m.getApplicationFieldType() == appFieldPackages {
		if m.applicationForm.packagesCursor == len(displayPackageManagers) && m.applicationForm.gitFieldCursor >= 0 {
			return m.updateApplicationGitFields(msg)
		}
		return m.updateApplicationPackagesList(msg)
	}

	// Handle filters list navigation
	if m.getApplicationFieldType() == appFieldFilters {
		return m.updateApplicationFiltersList(msg)
	}

	switch msg.String() {
	case KeyCtrlC:
		return m, tea.Quit

	case "q", KeyEsc:
		// Return to list view
		m.activeForm = FormNone
		m.applicationForm = nil
		m.Screen = ScreenResults
		return m, nil

	case KeyDown, "j":
		m.applicationForm.focusIndex++
		if m.applicationForm.focusIndex > 3 {
			m.applicationForm.focusIndex = 0
		}
		m.updateApplicationFormFocus()
		return m, nil

	case "up", "k":
		m.applicationForm.focusIndex--
		if m.applicationForm.focusIndex < 0 {
			m.applicationForm.focusIndex = 3
		}
		m.updateApplicationFormFocus()
		return m, nil

	case KeyTab:
		m.applicationForm.focusIndex++
		if m.applicationForm.focusIndex > 3 {
			m.applicationForm.focusIndex = 0
		}
		m.updateApplicationFormFocus()
		return m, nil

	case KeyShiftTab:
		m.applicationForm.focusIndex--
		if m.applicationForm.focusIndex < 0 {
			m.applicationForm.focusIndex = 3
		}
		m.updateApplicationFormFocus()
		return m, nil

	case KeyEnter, "e":
		// Enter edit mode for text fields
		ft := m.getApplicationFieldType()
		if ft == appFieldName || ft == appFieldDescription {
			m.enterApplicationFieldEditMode()
			return m, nil
		}

	case "s", KeyCtrlS:
		// Save the form
		if err := m.saveApplicationForm(); err != nil {
			m.applicationForm.err = err.Error()
			return m, nil
		}
		// Success - go back to list
		m.activeForm = FormNone
		m.applicationForm = nil
		m.Screen = ScreenResults
		return m, nil
	}

	// Clear error when navigating
	m.applicationForm.err = ""

	return m, nil
}

// updateApplicationFieldInput handles key events when editing a text field
func (m Model) updateApplicationFieldInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.applicationForm == nil {
		return m, nil
	}

	var cmd tea.Cmd
	ft := m.getApplicationFieldType()

	switch msg.String() {
	case KeyCtrlC:
		return m, tea.Quit

	case KeyEsc:
		// Cancel editing and restore original value
		m.cancelApplicationFieldEdit()
		return m, nil

	case KeyEnter, KeyTab:
		// Save and exit edit mode
		m.applicationForm.editingField = false
		m.updateApplicationFormFocus()
		return m, nil
	}

	// Handle text input for the focused field
	switch ft {
	case appFieldName:
		m.applicationForm.nameInput, cmd = m.applicationForm.nameInput.Update(msg)
	case appFieldDescription:
		m.applicationForm.descriptionInput, cmd = m.applicationForm.descriptionInput.Update(msg)
	case appFieldPackages, appFieldFilters:
		// List fields don't need text input updates
	}

	// Clear error when typing
	m.applicationForm.err = ""

	return m, cmd
}

// updateApplicationPackagesList handles key events when packages list is focused
func (m Model) updateApplicationPackagesList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.applicationForm == nil {
		return m, nil
	}

	maxCursor := len(displayPackageManagers) // includes git item at the end

	switch msg.String() {
	case KeyCtrlC:
		return m, tea.Quit

	case "q", KeyEsc:
		m.activeForm = FormNone
		m.applicationForm = nil
		m.Screen = ScreenResults
		return m, nil

	case "up", "k":
		if m.applicationForm.packagesCursor > 0 {
			m.applicationForm.packagesCursor--
			// Reset git field cursor when moving away from git item
			m.applicationForm.gitFieldCursor = -1
		} else {
			// Move to previous field
			m.applicationForm.focusIndex--
			m.updateApplicationFormFocus()
		}
		return m, nil

	case KeyDown, "j":
		switch {
		case m.applicationForm.packagesCursor < maxCursor:
			m.applicationForm.packagesCursor++
		case m.applicationForm.packagesCursor == maxCursor && m.applicationForm.hasGitPackage && m.applicationForm.gitFieldCursor == -1:
			// On git label, enter sub-fields
			m.applicationForm.gitFieldCursor = 0
		default:
			// Move to next field
			m.applicationForm.focusIndex++
			if m.applicationForm.focusIndex > 3 {
				m.applicationForm.focusIndex = 0
			}
			m.applicationForm.packagesCursor = 0
			m.applicationForm.gitFieldCursor = -1
			m.updateApplicationFormFocus()
		}
		return m, nil

	case KeyTab:
		m.applicationForm.focusIndex++
		if m.applicationForm.focusIndex > 3 {
			m.applicationForm.focusIndex = 0
		}
		m.applicationForm.packagesCursor = 0
		m.applicationForm.gitFieldCursor = -1
		m.updateApplicationFormFocus()
		return m, nil

	case KeyShiftTab:
		m.applicationForm.focusIndex--
		m.applicationForm.gitFieldCursor = -1
		m.updateApplicationFormFocus()
		return m, nil

	case KeyEnter, "e", " ":
		// Handle git item
		if m.applicationForm.packagesCursor == len(displayPackageManagers) {
			if !m.applicationForm.hasGitPackage {
				// Add git package
				m.applicationForm.hasGitPackage = true
				m.applicationForm.gitFieldCursor = GitFieldURL
			}
			return m, nil
		}
		// Edit the selected package manager's package name
		if m.applicationForm.packagesCursor < 0 || m.applicationForm.packagesCursor >= len(displayPackageManagers) {
			return m, nil
		}
		manager := displayPackageManagers[m.applicationForm.packagesCursor]
		currentValue := m.applicationForm.packageManagers[manager]

		// Auto-populate with last package name if empty
		if currentValue == "" && m.applicationForm.lastPackageName != "" {
			currentValue = m.applicationForm.lastPackageName
		}

		m.applicationForm.editingPackage = true
		m.applicationForm.packageNameInput.SetValue(currentValue)
		m.applicationForm.packageNameInput.Focus()
		m.applicationForm.packageNameInput.SetCursor(len(currentValue))
		return m, nil

	case "d", KeyBackspace, KeyDelete:
		// Handle git item deletion
		if m.applicationForm.packagesCursor == len(displayPackageManagers) && m.applicationForm.gitFieldCursor == -1 {
			m.applicationForm.hasGitPackage = false
			m.applicationForm.gitFieldCursor = -1
			m.applicationForm.gitSudo = false
			m.applicationForm.gitURLInput.SetValue("")
			m.applicationForm.gitBranchInput.SetValue("")
			m.applicationForm.gitLinuxInput.SetValue("")
			m.applicationForm.gitWindowsInput.SetValue("")
			m.applicationForm.err = ""
			return m, nil
		}
		// Clear the package name for the selected manager
		if m.applicationForm.packagesCursor < 0 || m.applicationForm.packagesCursor >= len(displayPackageManagers) {
			return m, nil
		}
		manager := displayPackageManagers[m.applicationForm.packagesCursor]
		delete(m.applicationForm.packageManagers, manager)
		m.applicationForm.err = ""
		return m, nil

	case "s", KeyCtrlS:
		// Save the form
		if err := m.saveApplicationForm(); err != nil {
			m.applicationForm.err = err.Error()
			return m, nil
		}
		m.activeForm = FormNone
		m.applicationForm = nil
		m.Screen = ScreenResults
		return m, nil
	}

	return m, nil
}

// updateApplicationPackageInput handles key events when editing a package name
func (m Model) updateApplicationPackageInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.applicationForm == nil {
		return m, nil
	}

	var cmd tea.Cmd

	switch msg.String() {
	case KeyCtrlC:
		return m, tea.Quit

	case KeyEsc:
		// Cancel editing package name
		m.applicationForm.editingPackage = false
		m.applicationForm.packageNameInput.SetValue("")
		m.applicationForm.err = ""
		return m, nil

	case KeyEnter:
		pkgName := strings.TrimSpace(m.applicationForm.packageNameInput.Value())
		if m.applicationForm.packagesCursor < 0 || m.applicationForm.packagesCursor >= len(displayPackageManagers) {
			m.applicationForm.editingPackage = false
			m.applicationForm.packageNameInput.SetValue("")
			return m, nil
		}
		manager := displayPackageManagers[m.applicationForm.packagesCursor]

		if pkgName != "" {
			m.applicationForm.packageManagers[manager] = pkgName
			m.applicationForm.lastPackageName = pkgName // Remember for auto-populate
		} else {
			// Clear if empty
			delete(m.applicationForm.packageManagers, manager)
		}

		m.applicationForm.editingPackage = false
		m.applicationForm.packageNameInput.SetValue("")
		return m, nil
	}

	// Handle text input
	m.applicationForm.packageNameInput, cmd = m.applicationForm.packageNameInput.Update(msg)
	m.applicationForm.err = ""
	return m, cmd
}

// updateApplicationGitFields handles navigation within git sub-fields (gitFieldCursor >= 0)
func (m Model) updateApplicationGitFields(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.applicationForm == nil {
		return m, nil
	}

	switch msg.String() {
	case KeyCtrlC:
		return m, tea.Quit

	case "q", KeyEsc:
		m.activeForm = FormNone
		m.applicationForm = nil
		m.Screen = ScreenResults
		return m, nil

	case "up", "k":
		if m.applicationForm.gitFieldCursor > 0 {
			m.applicationForm.gitFieldCursor--
		} else {
			// Back to git label (will route to updateApplicationPackagesList on next keypress)
			m.applicationForm.gitFieldCursor = -1
		}
		return m, nil

	case KeyDown, "j":
		if m.applicationForm.gitFieldCursor < GitFieldCount-1 {
			m.applicationForm.gitFieldCursor++
		} else {
			// Move to Filters section
			m.applicationForm.focusIndex++
			if m.applicationForm.focusIndex > 3 {
				m.applicationForm.focusIndex = 0
			}
			m.applicationForm.packagesCursor = 0
			m.applicationForm.gitFieldCursor = -1
			m.updateApplicationFormFocus()
		}
		return m, nil

	case KeyEnter, "e":
		if m.applicationForm.gitFieldCursor == GitFieldSudo {
			m.applicationForm.gitSudo = !m.applicationForm.gitSudo
			return m, nil
		}
		// Enter edit mode for text fields
		input := m.getGitFieldInput()
		if input != nil {
			m.applicationForm.editingGitField = true
			m.applicationForm.originalValue = input.Value()
			input.Focus()
			input.SetCursor(len(input.Value()))
		}
		return m, nil

	case " ":
		if m.applicationForm.gitFieldCursor == GitFieldSudo {
			m.applicationForm.gitSudo = !m.applicationForm.gitSudo
		}
		return m, nil

	case KeyTab:
		m.applicationForm.focusIndex++
		if m.applicationForm.focusIndex > 3 {
			m.applicationForm.focusIndex = 0
		}
		m.applicationForm.packagesCursor = 0
		m.applicationForm.gitFieldCursor = -1
		m.updateApplicationFormFocus()
		return m, nil

	case KeyShiftTab:
		m.applicationForm.focusIndex--
		m.applicationForm.gitFieldCursor = -1
		m.updateApplicationFormFocus()
		return m, nil

	case "s", KeyCtrlS:
		if err := m.saveApplicationForm(); err != nil {
			m.applicationForm.err = err.Error()
			return m, nil
		}
		m.activeForm = FormNone
		m.applicationForm = nil
		m.Screen = ScreenResults
		return m, nil
	}

	return m, nil
}

// updateApplicationGitFieldInput handles text input when editing a git field
func (m Model) updateApplicationGitFieldInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.applicationForm == nil {
		return m, nil
	}

	var cmd tea.Cmd

	switch msg.String() {
	case KeyCtrlC:
		return m, tea.Quit

	case KeyEsc:
		// Restore original value and exit edit mode
		input := m.getGitFieldInput()
		if input != nil {
			input.SetValue(m.applicationForm.originalValue)
		}
		m.applicationForm.editingGitField = false
		return m, nil

	case KeyEnter, KeyTab:
		// Save current value and exit edit mode
		m.applicationForm.editingGitField = false
		return m, nil
	}

	// Pass to the focused text input
	input := m.getGitFieldInput()
	if input != nil {
		*input, cmd = input.Update(msg)
	}

	m.applicationForm.err = ""
	return m, cmd
}

// getGitFieldInput returns a pointer to the current git text input based on gitFieldCursor
func (m *Model) getGitFieldInput() *textinput.Model {
	if m.applicationForm == nil {
		return nil
	}

	switch m.applicationForm.gitFieldCursor {
	case GitFieldURL:
		return &m.applicationForm.gitURLInput
	case GitFieldBranch:
		return &m.applicationForm.gitBranchInput
	case GitFieldLinux:
		return &m.applicationForm.gitLinuxInput
	case GitFieldWindows:
		return &m.applicationForm.gitWindowsInput
	default:
		return nil
	}
}

// updateApplicationFiltersList handles key events when filters list is focused
func (m Model) updateApplicationFiltersList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.applicationForm == nil {
		return m, nil
	}

	maxCursor := len(m.applicationForm.filters) // "Add Filter" button is at index len(filters)

	switch msg.String() {
	case KeyCtrlC:
		return m, tea.Quit

	case "q", KeyEsc:
		m.activeForm = FormNone
		m.applicationForm = nil
		m.Screen = ScreenResults
		return m, nil

	case "up", "k":
		if m.applicationForm.filtersCursor > 0 {
			m.applicationForm.filtersCursor--
		} else {
			// Move to previous field
			m.applicationForm.focusIndex--
			m.updateApplicationFormFocus()
		}
		return m, nil

	case KeyDown, "j":
		if m.applicationForm.filtersCursor < maxCursor {
			m.applicationForm.filtersCursor++
		} else {
			// Wrap to first field
			m.applicationForm.focusIndex = 0
			m.applicationForm.filtersCursor = 0
			m.updateApplicationFormFocus()
		}
		return m, nil

	case KeyTab:
		// Move to next field (wrap to beginning)
		m.applicationForm.focusIndex = 0
		m.applicationForm.filtersCursor = 0
		m.updateApplicationFormFocus()
		return m, nil

	case KeyShiftTab:
		// Move to previous field
		m.applicationForm.focusIndex--
		m.updateApplicationFormFocus()
		return m, nil

	case "enter", " ":
		// If on "Add Filter" button, start adding
		if m.applicationForm.filtersCursor == len(m.applicationForm.filters) {
			m.applicationForm.addingFilter = true
			m.applicationForm.filterAddStep = 0
			m.applicationForm.filterIsExclude = false
			m.applicationForm.filterKeyCursor = 0
			m.applicationForm.filterValueInput.SetValue("")
			return m, nil
		}
		// Edit the selected filter
		if m.applicationForm.filtersCursor < len(m.applicationForm.filters) {
			fc := m.applicationForm.filters[m.applicationForm.filtersCursor]
			m.applicationForm.editingFilter = true
			m.applicationForm.editingFilterIndex = m.applicationForm.filtersCursor
			m.applicationForm.filterAddStep = filterStepValue // Start at value step
			m.applicationForm.editingFilterValue = false      // Don't start in edit mode
			m.applicationForm.filterIsExclude = fc.IsExclude
			// Find key index
			for i, k := range filterKeys {
				if k == fc.Key {
					m.applicationForm.filterKeyCursor = i
					break
				}
			}
			m.applicationForm.filterValueInput.SetValue(fc.Value)
		}
		return m, nil

	case "d", KeyBackspace, KeyDelete:
		// Delete the selected filter
		if m.applicationForm.filtersCursor < len(m.applicationForm.filters) && len(m.applicationForm.filters) > 0 {
			// Remove filter at cursor
			m.applicationForm.filters = append(
				m.applicationForm.filters[:m.applicationForm.filtersCursor],
				m.applicationForm.filters[m.applicationForm.filtersCursor+1:]...,
			)
			// Adjust cursor if needed
			if m.applicationForm.filtersCursor >= len(m.applicationForm.filters) && m.applicationForm.filtersCursor > 0 {
				m.applicationForm.filtersCursor--
			}
		}
		m.applicationForm.err = ""
		return m, nil

	case "s", KeyCtrlS:
		// Save the form
		if err := m.saveApplicationForm(); err != nil {
			m.applicationForm.err = err.Error()
			return m, nil
		}
		m.activeForm = FormNone
		m.applicationForm = nil
		m.Screen = ScreenResults
		return m, nil
	}

	return m, nil
}

// updateApplicationFilterInput handles key events when adding or editing a filter
//
//nolint:gocyclo // UI handler with many states
func (m Model) updateApplicationFilterInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.applicationForm == nil {
		return m, nil
	}

	var cmd tea.Cmd

	// Handle value editing mode separately
	if m.applicationForm.filterAddStep == filterStepValue && m.applicationForm.editingFilterValue {
		switch msg.String() {
		case KeyCtrlC:
			return m, tea.Quit
		case KeyEsc:
			// Cancel value editing
			m.applicationForm.editingFilterValue = false
			m.applicationForm.filterValueInput.Blur()
			m.applicationForm.err = ""
			return m, nil
		case "enter":
			// Save the filter
			value := strings.TrimSpace(m.applicationForm.filterValueInput.Value())
			if value == "" {
				return m, nil // Don't save empty value
			}

			key := filterKeys[m.applicationForm.filterKeyCursor]

			if m.applicationForm.editingFilter {
				// Update existing filter
				if m.applicationForm.editingFilterIndex >= 0 && m.applicationForm.editingFilterIndex < len(m.applicationForm.filters) {
					m.applicationForm.filters[m.applicationForm.editingFilterIndex] = FilterCondition{
						FilterIndex: m.applicationForm.filters[m.applicationForm.editingFilterIndex].FilterIndex,
						IsExclude:   m.applicationForm.filterIsExclude,
						Key:         key,
						Value:       value,
					}
				}
				m.applicationForm.editingFilter = false
				m.applicationForm.editingFilterIndex = -1
			} else {
				// Add new filter
				filterIndex := 0
				if len(m.applicationForm.filters) > 0 {
					filterIndex = m.applicationForm.filters[len(m.applicationForm.filters)-1].FilterIndex
				}
				m.applicationForm.filters = append(m.applicationForm.filters, FilterCondition{
					FilterIndex: filterIndex,
					IsExclude:   m.applicationForm.filterIsExclude,
					Key:         key,
					Value:       value,
				})
				m.applicationForm.filtersCursor = len(m.applicationForm.filters) // Move to "Add Filter" button
				m.applicationForm.addingFilter = false
			}
			m.applicationForm.editingFilterValue = false
			m.applicationForm.filterValueInput.SetValue("")
			return m, nil
		}
		// Pass all other keys to the text input
		m.applicationForm.filterValueInput, cmd = m.applicationForm.filterValueInput.Update(msg)
		m.applicationForm.err = ""
		return m, cmd
	}

	// Handle navigation mode
	switch msg.String() {
	case KeyCtrlC:
		return m, tea.Quit

	case "esc":
		// Cancel adding/editing filter
		m.applicationForm.addingFilter = false
		m.applicationForm.editingFilter = false
		m.applicationForm.editingFilterIndex = -1
		m.applicationForm.editingFilterValue = false
		m.applicationForm.filterValueInput.SetValue("")
		m.applicationForm.err = ""
		return m, nil

	case "up", "k":
		// Navigate to previous step
		switch m.applicationForm.filterAddStep {
		case filterStepValue:
			m.applicationForm.filterAddStep = filterStepKey
		case filterStepKey:
			m.applicationForm.filterAddStep = filterStepType
		}
		return m, nil

	case KeyDown, "j":
		// Navigate to next step
		switch m.applicationForm.filterAddStep {
		case filterStepType:
			m.applicationForm.filterAddStep = filterStepKey
		case filterStepKey:
			m.applicationForm.filterAddStep = filterStepValue
		}
		return m, nil

	case KeyLeft, "h":
		// Navigate in type or key step
		switch m.applicationForm.filterAddStep {
		case filterStepType:
			m.applicationForm.filterIsExclude = !m.applicationForm.filterIsExclude
		case filterStepKey:
			if m.applicationForm.filterKeyCursor > 0 {
				m.applicationForm.filterKeyCursor--
			}
		}
		return m, nil

	case KeyRight, "l":
		// Navigate in type or key step
		switch m.applicationForm.filterAddStep {
		case filterStepType:
			m.applicationForm.filterIsExclude = !m.applicationForm.filterIsExclude
		case filterStepKey:
			if m.applicationForm.filterKeyCursor < len(filterKeys)-1 {
				m.applicationForm.filterKeyCursor++
			}
		}
		return m, nil

	case KeyTab:
		// Move to next step
		switch m.applicationForm.filterAddStep {
		case filterStepType:
			m.applicationForm.filterAddStep = filterStepKey
		case filterStepKey:
			m.applicationForm.filterAddStep = filterStepValue
			// Auto-start editing when adding
			if m.applicationForm.addingFilter {
				m.applicationForm.editingFilterValue = true
				m.applicationForm.filterValueInput.Focus()
				m.applicationForm.filterValueInput.SetCursor(len(m.applicationForm.filterValueInput.Value()))
			}
		case filterStepValue:
			m.applicationForm.editingFilterValue = true
			m.applicationForm.filterValueInput.Focus()
			m.applicationForm.filterValueInput.SetCursor(len(m.applicationForm.filterValueInput.Value()))
		}
		return m, nil

	case KeyEnter, "e":
		// Enter edit mode for current step, or advance
		switch m.applicationForm.filterAddStep {
		case filterStepType:
			m.applicationForm.filterAddStep = filterStepKey
		case filterStepKey:
			m.applicationForm.filterAddStep = filterStepValue
			// Auto-start editing when adding
			if m.applicationForm.addingFilter {
				m.applicationForm.editingFilterValue = true
				m.applicationForm.filterValueInput.Focus()
				m.applicationForm.filterValueInput.SetCursor(len(m.applicationForm.filterValueInput.Value()))
			}
		case filterStepValue:
			m.applicationForm.editingFilterValue = true
			m.applicationForm.filterValueInput.Focus()
			m.applicationForm.filterValueInput.SetCursor(len(m.applicationForm.filterValueInput.Value()))
		}
		return m, nil

	case KeyShiftTab:
		// Move to previous step
		if m.applicationForm.filterAddStep > filterStepType {
			m.applicationForm.filterAddStep--
		}
		return m, nil
	}

	return m, nil
}

// viewApplicationForm renders the application form
func (m Model) viewApplicationForm() string {
	if m.applicationForm == nil {
		return ""
	}

	var b strings.Builder
	ft := m.getApplicationFieldType()

	// Title
	if m.applicationForm.editAppIdx >= 0 {
		b.WriteString(TitleStyle.Render("  Edit Application"))
		b.WriteString("\n\n")
		b.WriteString(SubtitleStyle.Render("Edit the application metadata"))
	} else {
		b.WriteString(TitleStyle.Render("  Add Application"))
		b.WriteString("\n\n")
		b.WriteString(SubtitleStyle.Render("Add a new application to your configuration"))
	}
	b.WriteString("\n\n")

	// Name field
	nameLabel := "Name:"
	if ft == appFieldName {
		nameLabel = HelpKeyStyle.Render("Name:")
	}
	b.WriteString(fmt.Sprintf("  %s\n", nameLabel))
	b.WriteString(fmt.Sprintf("  %s\n\n", m.renderApplicationFieldValue(appFieldName, "(empty)")))

	// Description field
	descLabel := "Description:"
	if ft == appFieldDescription {
		descLabel = HelpKeyStyle.Render("Description:")
	}
	b.WriteString(fmt.Sprintf("  %s\n", descLabel))
	b.WriteString(fmt.Sprintf("  %s\n\n", m.renderApplicationFieldValue(appFieldDescription, "(optional)")))

	// Packages section
	packagesLabel := "Packages:"
	if ft == appFieldPackages {
		packagesLabel = HelpKeyStyle.Render("Packages:")
	}
	b.WriteString(fmt.Sprintf("  %s\n", packagesLabel))
	b.WriteString(renderPackagesSection(
		ft == appFieldPackages,
		m.applicationForm.packageManagers,
		m.applicationForm.packagesCursor,
		m.applicationForm.editingPackage,
		m.applicationForm.packageNameInput,
	))
	onGitItem := ft == appFieldPackages && m.applicationForm.packagesCursor == len(displayPackageManagers)
	b.WriteString(renderGitPackageSection(
		ft == appFieldPackages,
		onGitItem,
		m.applicationForm.hasGitPackage,
		m.applicationForm.gitFieldCursor,
		m.applicationForm.editingGitField,
		m.applicationForm.gitURLInput,
		m.applicationForm.gitBranchInput,
		m.applicationForm.gitLinuxInput,
		m.applicationForm.gitWindowsInput,
		m.applicationForm.gitSudo,
	))
	b.WriteString("\n")

	// Filters section
	filtersLabel := "Filters:"
	if ft == appFieldFilters {
		filtersLabel = HelpKeyStyle.Render("Filters:")
	}
	b.WriteString(fmt.Sprintf("  %s\n", filtersLabel))
	b.WriteString(renderFiltersSection(
		ft == appFieldFilters,
		m.applicationForm.filters,
		m.applicationForm.filtersCursor,
		m.applicationForm.addingFilter,
		m.applicationForm.editingFilter,
		m.applicationForm.filterAddStep,
		m.applicationForm.filterIsExclude,
		m.applicationForm.filterKeyCursor,
		m.applicationForm.editingFilterValue,
		m.applicationForm.filterValueInput,
	))
	b.WriteString("\n")

	// Error message
	if m.applicationForm.err != "" {
		b.WriteString(ErrorStyle.Render("  Error: " + m.applicationForm.err))
		b.WriteString("\n\n")
	}

	// Help
	b.WriteString(m.renderApplicationFormHelp())

	return BaseStyle.Render(b.String())
}

// renderApplicationFieldValue renders a field value with appropriate styling
func (m Model) renderApplicationFieldValue(fieldType applicationFieldType, placeholder string) string {
	if m.applicationForm == nil {
		return placeholder
	}

	currentFt := m.getApplicationFieldType()
	isEditing := m.applicationForm.editingField && currentFt == fieldType
	isFocused := currentFt == fieldType

	var input textinput.Model
	switch fieldType {
	case appFieldName:
		input = m.applicationForm.nameInput
	case appFieldDescription:
		input = m.applicationForm.descriptionInput
	case appFieldPackages, appFieldFilters:
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

// renderApplicationFormHelp renders context-sensitive help for the application form
func (m Model) renderApplicationFormHelp() string {
	if m.applicationForm == nil {
		return ""
	}

	ft := m.getApplicationFieldType()

	if m.applicationForm.editingGitField {
		return RenderHelpWithWidth(m.width,
			"enter", "save",
			"esc", "cancel",
		)
	}

	if m.applicationForm.editingPackage {
		return RenderHelpWithWidth(m.width,
			"enter", "save",
			"esc", "cancel",
		)
	}

	if m.applicationForm.addingFilter || m.applicationForm.editingFilter {
		switch m.applicationForm.filterAddStep {
		case filterStepType:
			return RenderHelpWithWidth(m.width,
				"←/h →/l", "select type",
				"↓/j", "next step",
				"enter/tab", "next",
				"esc", "cancel",
			)
		case filterStepKey:
			return RenderHelpWithWidth(m.width,
				"←/h →/l", "select key",
				"enter/tab", "next",
				"esc", "cancel",
			)
		case filterStepValue:
			if m.applicationForm.editingFilterValue {
				return RenderHelpWithWidth(m.width,
					"enter", "save filter",
					"esc", "cancel edit",
				)
			}
			return RenderHelpWithWidth(m.width,
				"enter/e", "edit value",
				"esc", "cancel",
			)
		}
	}

	if m.applicationForm.editingField {
		return RenderHelpWithWidth(m.width,
			"enter/tab", "save",
			"esc", "cancel edit",
		)
	}

	if ft == appFieldPackages {
		// Git package states
		if m.applicationForm.packagesCursor == len(displayPackageManagers) {
			if !m.applicationForm.hasGitPackage {
				return RenderHelpWithWidth(m.width, "enter", "add", "s", "save", "q", "back")
			}
			if m.applicationForm.gitFieldCursor == -1 {
				return RenderHelpWithWidth(m.width, "d/del", "delete", "s", "save", "q", "back")
			}
			if m.applicationForm.gitFieldCursor == GitFieldSudo {
				return RenderHelpWithWidth(m.width, "space", "toggle", "s", "save", "q", "back")
			}
			return RenderHelpWithWidth(m.width, "enter/e", "edit", "s", "save", "q", "back")
		}
		// Bounds check for packagesCursor
		if m.applicationForm.packagesCursor >= 0 && m.applicationForm.packagesCursor < len(displayPackageManagers) {
			manager := displayPackageManagers[m.applicationForm.packagesCursor]
			if m.applicationForm.packageManagers[manager] != "" {
				return RenderHelpWithWidth(m.width,
					"enter/e", "edit",
					"d/del", "clear",
					"s", "save",
					"q", "back",
				)
			}
		}
		return RenderHelpWithWidth(m.width,
			"enter/e", "set package",
			"s", "save",
			"q", "back",
		)
	}

	if ft == appFieldFilters {
		if m.applicationForm.filtersCursor < len(m.applicationForm.filters) {
			return RenderHelpWithWidth(m.width,
				"enter", "edit",
				"d/del", "remove",
				"s", "save",
				"q", "back",
			)
		}
		return RenderHelpWithWidth(m.width,
			"enter", "add filter",
			"s", "save",
			"q", "back",
		)
	}

	// Text field focused (not editing)
	return RenderHelpWithWidth(m.width,
		"enter/e", "edit",
		"s", "save",
		"q", "back",
	)
}

// saveApplicationForm validates and saves the application form
func (m *Model) saveApplicationForm() error {
	if m.applicationForm == nil {
		return errors.New("no form data")
	}

	name := strings.TrimSpace(m.applicationForm.nameInput.Value())
	description := strings.TrimSpace(m.applicationForm.descriptionInput.Value())

	// Validation
	if name == "" {
		return errors.New("name is required")
	}

	// Build filters and package
	filters := buildFiltersFromConditions(m.applicationForm.filters)
	pkg := buildPackageSpec(m.applicationForm.packageManagers)

	// Merge git package data
	pkg = mergeGitPackage(
		pkg,
		m.applicationForm.hasGitPackage,
		m.applicationForm.gitURLInput,
		m.applicationForm.gitBranchInput,
		m.applicationForm.gitLinuxInput,
		m.applicationForm.gitWindowsInput,
		m.applicationForm.gitSudo,
	)

	// Validate git package if present
	if m.applicationForm.hasGitPackage {
		gitURL := strings.TrimSpace(m.applicationForm.gitURLInput.Value())
		if gitURL == "" {
			return errors.New("git package URL is required")
		}
		gitLinux := strings.TrimSpace(m.applicationForm.gitLinuxInput.Value())
		gitWindows := strings.TrimSpace(m.applicationForm.gitWindowsInput.Value())
		if gitLinux == "" && gitWindows == "" {
			return errors.New("git package requires at least one target (Linux or Windows)")
		}
	}

	// Save based on edit mode
	if m.applicationForm.editAppIdx >= 0 {
		return m.saveEditedApplication(m.applicationForm.editAppIdx, name, description, filters, pkg)
	}
	return m.saveNewApplication(config.Application{
		Name:        name,
		Description: description,
		Filters:     filters,
		Package:     pkg,
		Entries:     []config.SubEntry{}, // Empty entries initially
	})
}

// updateApplicationFormFocus updates which input field is focused
func (m *Model) updateApplicationFormFocus() {
	if m.applicationForm == nil {
		return
	}

	m.applicationForm.nameInput.Blur()
	m.applicationForm.descriptionInput.Blur()

	ft := m.getApplicationFieldType()
	switch ft {
	case appFieldName:
		m.applicationForm.nameInput.Focus()
	case appFieldDescription:
		m.applicationForm.descriptionInput.Focus()
	case appFieldPackages, appFieldFilters:
		// List fields don't use textinput focus
	}
}

// enterApplicationFieldEditMode enters edit mode for the current text field
func (m *Model) enterApplicationFieldEditMode() {
	if m.applicationForm == nil {
		return
	}

	m.applicationForm.editingField = true
	ft := m.getApplicationFieldType()

	switch ft {
	case appFieldName:
		m.applicationForm.originalValue = m.applicationForm.nameInput.Value()
		m.applicationForm.nameInput.Focus()
		m.applicationForm.nameInput.SetCursor(len(m.applicationForm.nameInput.Value()))
	case appFieldDescription:
		m.applicationForm.originalValue = m.applicationForm.descriptionInput.Value()
		m.applicationForm.descriptionInput.Focus()
		m.applicationForm.descriptionInput.SetCursor(len(m.applicationForm.descriptionInput.Value()))
	case appFieldPackages, appFieldFilters:
		// List fields don't use text input editing
	}
}

// cancelApplicationFieldEdit cancels editing and restores the original value
func (m *Model) cancelApplicationFieldEdit() {
	if m.applicationForm == nil {
		return
	}

	ft := m.getApplicationFieldType()
	switch ft {
	case appFieldName:
		m.applicationForm.nameInput.SetValue(m.applicationForm.originalValue)
	case appFieldDescription:
		m.applicationForm.descriptionInput.SetValue(m.applicationForm.originalValue)
	case appFieldPackages, appFieldFilters:
		// List fields don't use text input restoration
	}

	m.applicationForm.editingField = false
	m.applicationForm.err = ""
	m.updateApplicationFormFocus()
}

// NewApplicationForm creates a new ApplicationForm for testing purposes
func NewApplicationForm(app config.Application, isEdit bool) *ApplicationForm {
	nameInput := textinput.New()
	nameInput.SetValue(app.Name)

	descriptionInput := textinput.New()
	descriptionInput.SetValue(app.Description)

	editAppIdx := -1
	if isEdit {
		editAppIdx = 0
	}

	// Initialize git text inputs
	gitURLInput := textinput.New()
	gitURLInput.Placeholder = PlaceholderGitURL
	gitURLInput.CharLimit = 256
	gitURLInput.Width = 40

	gitBranchInput := textinput.New()
	gitBranchInput.Placeholder = PlaceholderGitBranch
	gitBranchInput.CharLimit = 128
	gitBranchInput.Width = 40

	gitLinuxInput := textinput.New()
	gitLinuxInput.Placeholder = PlaceholderGitLinux
	gitLinuxInput.CharLimit = 256
	gitLinuxInput.Width = 40

	gitWindowsInput := textinput.New()
	gitWindowsInput.Placeholder = PlaceholderGitWindows
	gitWindowsInput.CharLimit = 256
	gitWindowsInput.Width = 40

	// Load git package if present
	hasGitPackage := false
	gitSudo := false

	if app.Package != nil {
		if gitVal, ok := app.Package.Managers[TypeGit]; ok && gitVal.IsGit() {
			hasGitPackage = true
			gitURLInput.SetValue(gitVal.Git.URL)
			gitBranchInput.SetValue(gitVal.Git.Branch)
			gitSudo = gitVal.Git.Sudo

			if target, ok := gitVal.Git.Targets[OSLinux]; ok {
				gitLinuxInput.SetValue(target)
			}
			if target, ok := gitVal.Git.Targets[OSWindows]; ok {
				gitWindowsInput.SetValue(target)
			}
		}
	}

	return &ApplicationForm{
		nameInput:        nameInput,
		descriptionInput: descriptionInput,
		editAppIdx:       editAppIdx,
		gitURLInput:      gitURLInput,
		gitBranchInput:   gitBranchInput,
		gitLinuxInput:    gitLinuxInput,
		gitWindowsInput:  gitWindowsInput,
		gitFieldCursor:   -1,
		hasGitPackage:    hasGitPackage,
		gitSudo:          gitSudo,
	}
}

// Validate checks if the ApplicationForm has valid data
func (f *ApplicationForm) Validate() error {
	if strings.TrimSpace(f.nameInput.Value()) == "" {
		return errors.New("application name is required")
	}
	return nil
}
