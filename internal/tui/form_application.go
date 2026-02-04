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
	nameInput.Placeholder = "PlaceholderNeovim"
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
			if k == "git" {
				continue
			}
			// Only include string values
			if str, ok := v.(string); ok {
				packageManagers[k] = str
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

	maxCursor := len(knownPackageManagers) - 1

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
		} else {
			// Move to previous field
			m.applicationForm.focusIndex--
			m.updateApplicationFormFocus()
		}
		return m, nil

	case KeyDown, "j":
		if m.applicationForm.packagesCursor < maxCursor {
			m.applicationForm.packagesCursor++
		} else {
			// Move to next field
			m.applicationForm.focusIndex++
			if m.applicationForm.focusIndex > 3 {
				m.applicationForm.focusIndex = 0
			}
			m.applicationForm.packagesCursor = 0
			m.updateApplicationFormFocus()
		}
		return m, nil

	case KeyTab:
		m.applicationForm.focusIndex++
		if m.applicationForm.focusIndex > 3 {
			m.applicationForm.focusIndex = 0
		}
		m.applicationForm.packagesCursor = 0
		m.updateApplicationFormFocus()
		return m, nil

	case KeyShiftTab:
		m.applicationForm.focusIndex--
		m.updateApplicationFormFocus()
		return m, nil

	case KeyEnter, "e", " ":
		// Edit the selected package manager's package name
		if m.applicationForm.packagesCursor < 0 || m.applicationForm.packagesCursor >= len(knownPackageManagers) {
			return m, nil
		}
		manager := knownPackageManagers[m.applicationForm.packagesCursor]
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
		// Clear the package name for the selected manager
		if m.applicationForm.packagesCursor < 0 || m.applicationForm.packagesCursor >= len(knownPackageManagers) {
			return m, nil
		}
		manager := knownPackageManagers[m.applicationForm.packagesCursor]
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
		if m.applicationForm.packagesCursor < 0 || m.applicationForm.packagesCursor >= len(knownPackageManagers) {
			m.applicationForm.editingPackage = false
			m.applicationForm.packageNameInput.SetValue("")
			return m, nil
		}
		manager := knownPackageManagers[m.applicationForm.packagesCursor]

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

	case "left", "h":
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

	case "right", "l":
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

	if m.applicationForm.editingPackage {
		return RenderHelp(
			"enter", "save",
			"esc", "cancel",
		)
	}

	if m.applicationForm.addingFilter || m.applicationForm.editingFilter {
		switch m.applicationForm.filterAddStep {
		case filterStepType:
			return RenderHelp(
				"←/h →/l", "select type",
				"↓/j", "next step",
				"enter/tab", "next",
				"esc", "cancel",
			)
		case filterStepKey:
			return RenderHelp(
				"←/h →/l", "select key",
				"enter/tab", "next",
				"esc", "cancel",
			)
		case filterStepValue:
			if m.applicationForm.editingFilterValue {
				return RenderHelp(
					"enter", "save filter",
					"esc", "cancel edit",
				)
			}
			return RenderHelp(
				"enter/e", "edit value",
				"esc", "cancel",
			)
		}
	}

	if m.applicationForm.editingField {
		return RenderHelp(
			"enter/tab", "save",
			"esc", "cancel edit",
		)
	}

	if ft == appFieldPackages {
		// Bounds check for packagesCursor
		if m.applicationForm.packagesCursor >= 0 && m.applicationForm.packagesCursor < len(knownPackageManagers) {
			manager := knownPackageManagers[m.applicationForm.packagesCursor]
			if m.applicationForm.packageManagers[manager] != "" {
				return RenderHelp(
					"enter/e", "edit",
					"d/del", "clear",
					"s", "save",
					"q", "back",
				)
			}
		}
		return RenderHelp(
			"enter/e", "set package",
			"s", "save",
			"q", "back",
		)
	}

	if ft == appFieldFilters {
		if m.applicationForm.filtersCursor < len(m.applicationForm.filters) {
			return RenderHelp(
				"enter", "edit",
				"d/del", "remove",
				"s", "save",
				"q", "back",
			)
		}
		return RenderHelp(
			"enter", "add filter",
			"s", "save",
			"q", "back",
		)
	}

	// Text field focused (not editing)
	return RenderHelp(
		"enter/e", "edit",
		"s", "save",
		"q", "back",
	)
}

// saveApplicationForm validates and saves the application form
func (m *Model) saveApplicationForm() error {
	if m.applicationForm == nil {
		return fmt.Errorf("no form data")
	}

	name := strings.TrimSpace(m.applicationForm.nameInput.Value())
	description := strings.TrimSpace(m.applicationForm.descriptionInput.Value())

	// Validation
	if name == "" {
		return fmt.Errorf("name is required")
	}

	// Build filters and package
	filters := buildFiltersFromConditions(m.applicationForm.filters)
	pkg := buildPackageSpec(m.applicationForm.packageManagers)

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

	return &ApplicationForm{
		nameInput:        nameInput,
		descriptionInput: descriptionInput,
		editAppIdx:       editAppIdx,
	}
}

// Validate checks if the ApplicationForm has valid data
func (f *ApplicationForm) Validate() error {
	if strings.TrimSpace(f.nameInput.Value()) == "" {
		return errors.New("application name is required")
	}
	return nil
}
