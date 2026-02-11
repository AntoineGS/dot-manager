package tui

import (
	"errors"
	"fmt"
	"strings"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// applicationFieldType represents the type of field in the ApplicationForm
type applicationFieldType int

const (
	appFieldName applicationFieldType = iota
	appFieldDescription
	appFieldPackages
	appFieldWhen
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

	packageNameInput := textinput.New()
	packageNameInput.Placeholder = PlaceholderNeovim
	packageNameInput.CharLimit = 128
	packageNameInput.Width = 40

	whenInput := textinput.New()
	whenInput.Placeholder = PlaceholderWhen
	whenInput.CharLimit = 512
	whenInput.Width = 60

	gitURLInput, gitBranchInput, gitLinuxInput, gitWindowsInput := newGitTextInputs()
	installerLinuxInput, installerWindowsInput, installerBinaryInput := newInstallerTextInputs()

	m.applicationForm = &ApplicationForm{
		nameInput:             nameInput,
		descriptionInput:      descriptionInput,
		packageManagers:       make(map[string]string),
		packagesCursor:        0,
		editingPackage:        false,
		packageNameInput:      packageNameInput,
		lastPackageName:       "",
		whenInput:             whenInput,
		focusIndex:            0,
		editingField:          false,
		originalValue:         "",
		editAppIdx:            -1,
		err:                   "",
		gitURLInput:           gitURLInput,
		gitBranchInput:        gitBranchInput,
		gitLinuxInput:         gitLinuxInput,
		gitWindowsInput:       gitWindowsInput,
		gitFieldCursor:        -1,
		hasGitPackage:         false,
		gitSudo:               false,
		installerLinuxInput:   installerLinuxInput,
		installerWindowsInput: installerWindowsInput,
		installerBinaryInput:  installerBinaryInput,
		installerFieldCursor:  -1,
		hasInstallerPackage:   false,
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

	packageNameInput := textinput.New()
	packageNameInput.Placeholder = PlaceholderNeovim
	packageNameInput.CharLimit = 128
	packageNameInput.Width = 40

	whenInput := textinput.New()
	whenInput.Placeholder = PlaceholderWhen
	whenInput.CharLimit = 512
	whenInput.Width = 60
	whenInput.SetValue(app.When)

	gitURLInput, gitBranchInput, gitLinuxInput, gitWindowsInput := newGitTextInputs()
	installerLinuxInput, installerWindowsInput, installerBinaryInput := newInstallerTextInputs()

	// Load package managers (only string-based managers, skip git and installer)
	packageManagers := make(map[string]string)
	if app.Package != nil && len(app.Package.Managers) > 0 {
		for k, v := range app.Package.Managers {
			if k == TypeGit || k == TypeInstaller {
				continue
			}
			if !v.IsGit() && !v.IsInstaller() {
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

	// Load installer package if present
	hasInstallerPackage := false

	if app.Package != nil {
		if installerVal, ok := app.Package.Managers[TypeInstaller]; ok && installerVal.IsInstaller() {
			hasInstallerPackage = true
			if cmd, ok := installerVal.Installer.Command[OSLinux]; ok {
				installerLinuxInput.SetValue(cmd)
			}
			if cmd, ok := installerVal.Installer.Command[OSWindows]; ok {
				installerWindowsInput.SetValue(cmd)
			}
			installerBinaryInput.SetValue(installerVal.Installer.Binary)
		}
	}

	m.applicationForm = &ApplicationForm{
		nameInput:             nameInput,
		descriptionInput:      descriptionInput,
		packageManagers:       packageManagers,
		packagesCursor:        0,
		editingPackage:        false,
		packageNameInput:      packageNameInput,
		lastPackageName:       "",
		whenInput:             whenInput,
		focusIndex:            0,
		editingField:          false,
		originalValue:         "",
		editAppIdx:            configAppIdx,
		err:                   "",
		gitURLInput:           gitURLInput,
		gitBranchInput:        gitBranchInput,
		gitLinuxInput:         gitLinuxInput,
		gitWindowsInput:       gitWindowsInput,
		gitFieldCursor:        -1,
		hasGitPackage:         hasGitPackage,
		gitSudo:               gitSudo,
		installerLinuxInput:   installerLinuxInput,
		installerWindowsInput: installerWindowsInput,
		installerBinaryInput:  installerBinaryInput,
		installerFieldCursor:  -1,
		hasInstallerPackage:   hasInstallerPackage,
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
		return appFieldWhen
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

	// Handle editing an installer text field
	if m.applicationForm.editingInstallerField {
		return m.updateApplicationInstallerFieldInput(msg)
	}

	// Handle editing a text field
	if m.applicationForm.editingField {
		return m.updateApplicationFieldInput(msg)
	}

	// Handle editing package name
	if m.applicationForm.editingPackage {
		return m.updateApplicationPackageInput(msg)
	}

	// Handle editing when expression
	if m.applicationForm.editingWhen {
		return m.updateApplicationWhenInput(msg)
	}

	// Handle packages list navigation
	if m.getApplicationFieldType() == appFieldPackages {
		if m.applicationForm.packagesCursor == len(displayPackageManagers) && m.applicationForm.gitFieldCursor >= 0 {
			return m.updateApplicationGitFields(msg)
		}
		if m.applicationForm.packagesCursor == len(displayPackageManagers)+1 && m.applicationForm.installerFieldCursor >= 0 {
			return m.updateApplicationInstallerFields(msg)
		}
		return m.updateApplicationPackagesList(msg)
	}

	if m, cmd, handled := m.handleCommonKeys(msg); handled {
		return m, cmd
	}

	switch {
	case key.Matches(msg, FormNavKeys.Cancel):
		// Return to list view
		m.activeForm = FormNone
		m.applicationForm = nil
		m.Screen = ScreenResults
		return m, nil

	case key.Matches(msg, FormNavKeys.Down):
		m.applicationForm.focusIndex++
		if m.applicationForm.focusIndex > 3 {
			m.applicationForm.focusIndex = 0
		}
		m.updateApplicationFormFocus()
		return m, nil

	case key.Matches(msg, FormNavKeys.Up):
		m.applicationForm.focusIndex--
		if m.applicationForm.focusIndex < 0 {
			m.applicationForm.focusIndex = 3
		}
		if m.getApplicationFieldType() == appFieldPackages {
			m.applicationForm.packagesCursor = len(displayPackageManagers) + 1
			m.applicationForm.gitFieldCursor = -1
			m.applicationForm.installerFieldCursor = -1
		}
		m.updateApplicationFormFocus()
		return m, nil

	case key.Matches(msg, FormNavKeys.TabNext):
		m.applicationForm.focusIndex++
		if m.applicationForm.focusIndex > 3 {
			m.applicationForm.focusIndex = 0
		}
		m.updateApplicationFormFocus()
		return m, nil

	case key.Matches(msg, FormNavKeys.TabPrev):
		m.applicationForm.focusIndex--
		if m.applicationForm.focusIndex < 0 {
			m.applicationForm.focusIndex = 3
		}
		if m.getApplicationFieldType() == appFieldPackages {
			m.applicationForm.packagesCursor = len(displayPackageManagers) + 1
			m.applicationForm.gitFieldCursor = -1
			m.applicationForm.installerFieldCursor = -1
		}
		m.updateApplicationFormFocus()
		return m, nil

	case key.Matches(msg, FormNavKeys.Edit):
		// Enter edit mode for text fields
		ft := m.getApplicationFieldType()
		if ft == appFieldName || ft == appFieldDescription {
			m.enterApplicationFieldEditMode()
			return m, nil
		}
		if ft == appFieldWhen {
			m.applicationForm.editingWhen = true
			m.applicationForm.originalValue = m.applicationForm.whenInput.Value()
			m.applicationForm.whenInput.Focus()
			m.applicationForm.whenInput.SetCursor(len(m.applicationForm.whenInput.Value()))
			return m, nil
		}

	case key.Matches(msg, FormNavKeys.Save):
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

	if m, cmd, handled := m.handleTextEditKeys(msg); handled {
		return m, cmd
	}

	switch {
	case key.Matches(msg, TextEditKeys.Cancel):
		// Cancel editing and restore original value
		m.cancelApplicationFieldEdit()
		return m, nil

	case key.Matches(msg, TextEditKeys.Confirm):
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
	case appFieldPackages, appFieldWhen:
		// List/when fields don't need text input updates here
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

	gitItemIdx := len(displayPackageManagers)
	installerItemIdx := len(displayPackageManagers) + 1
	maxCursor := installerItemIdx // includes git and installer items at the end

	if m, cmd, handled := m.handleCommonKeys(msg); handled {
		return m, cmd
	}

	switch {
	case key.Matches(msg, FormNavKeys.Cancel):
		m.activeForm = FormNone
		m.applicationForm = nil
		m.Screen = ScreenResults
		return m, nil

	case key.Matches(msg, FormNavKeys.Up):
		if m.applicationForm.packagesCursor > 0 {
			m.applicationForm.packagesCursor--
			// Reset field cursors when moving between items
			m.applicationForm.gitFieldCursor = -1
			m.applicationForm.installerFieldCursor = -1
		} else {
			// Move to previous field
			m.applicationForm.focusIndex--
			m.updateApplicationFormFocus()
		}
		return m, nil

	case key.Matches(msg, FormNavKeys.Down):
		switch {
		case m.applicationForm.packagesCursor < maxCursor:
			// Moving to next item - handle git sub-field entry
			if m.applicationForm.packagesCursor == gitItemIdx && m.applicationForm.hasGitPackage && m.applicationForm.gitFieldCursor == -1 {
				m.applicationForm.gitFieldCursor = 0
				return m, nil
			}
			m.applicationForm.packagesCursor++
			m.applicationForm.gitFieldCursor = -1
			m.applicationForm.installerFieldCursor = -1
		case m.applicationForm.packagesCursor == installerItemIdx && m.applicationForm.hasInstallerPackage && m.applicationForm.installerFieldCursor == -1:
			// Enter installer sub-fields (handled separately since installer is the last item, so packagesCursor == maxCursor)
			m.applicationForm.installerFieldCursor = 0
		default:
			// Move to next field
			m.applicationForm.focusIndex++
			if m.applicationForm.focusIndex > 3 {
				m.applicationForm.focusIndex = 0
			}
			m.applicationForm.packagesCursor = 0
			m.applicationForm.gitFieldCursor = -1
			m.applicationForm.installerFieldCursor = -1
			m.updateApplicationFormFocus()
		}
		return m, nil

	case key.Matches(msg, FormNavKeys.TabNext):
		m.applicationForm.focusIndex++
		if m.applicationForm.focusIndex > 3 {
			m.applicationForm.focusIndex = 0
		}
		m.applicationForm.packagesCursor = 0
		m.applicationForm.gitFieldCursor = -1
		m.applicationForm.installerFieldCursor = -1
		m.updateApplicationFormFocus()
		return m, nil

	case key.Matches(msg, FormNavKeys.TabPrev):
		m.applicationForm.focusIndex--
		m.applicationForm.gitFieldCursor = -1
		m.applicationForm.installerFieldCursor = -1
		m.updateApplicationFormFocus()
		return m, nil

	case key.Matches(msg, FilesListKeys.Edit):
		return m.handlePackagesListActivate(gitItemIdx, installerItemIdx)

	case key.Matches(msg, FormNavKeys.Delete):
		return m.handlePackagesListDelete(gitItemIdx, installerItemIdx)

	case key.Matches(msg, FormNavKeys.Save):
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

// handlePackagesListActivate handles enter/space on the packages list
func (m Model) handlePackagesListActivate(gitItemIdx, installerItemIdx int) (tea.Model, tea.Cmd) {
	// Handle git item
	if m.applicationForm.packagesCursor == gitItemIdx {
		if !m.applicationForm.hasGitPackage {
			m.applicationForm.hasGitPackage = true
		}
		m.applicationForm.gitFieldCursor = GitFieldURL
		return m, nil
	}
	// Handle installer item
	if m.applicationForm.packagesCursor == installerItemIdx {
		if !m.applicationForm.hasInstallerPackage {
			m.applicationForm.hasInstallerPackage = true
		}
		m.applicationForm.installerFieldCursor = InstallerFieldLinux
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
}

// handlePackagesListDelete handles delete/backspace on the packages list
func (m Model) handlePackagesListDelete(gitItemIdx, installerItemIdx int) (tea.Model, tea.Cmd) {
	// Handle git item deletion
	if m.applicationForm.packagesCursor == gitItemIdx && m.applicationForm.gitFieldCursor == -1 {
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
	// Handle installer item deletion
	if m.applicationForm.packagesCursor == installerItemIdx && m.applicationForm.installerFieldCursor == -1 {
		m.applicationForm.hasInstallerPackage = false
		m.applicationForm.installerFieldCursor = -1
		m.applicationForm.installerLinuxInput.SetValue("")
		m.applicationForm.installerWindowsInput.SetValue("")
		m.applicationForm.installerBinaryInput.SetValue("")
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
}

// updateApplicationPackageInput handles key events when editing a package name
func (m Model) updateApplicationPackageInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.applicationForm == nil {
		return m, nil
	}

	var cmd tea.Cmd

	if m, cmd, handled := m.handleTextEditKeys(msg); handled {
		return m, cmd
	}

	switch {
	case key.Matches(msg, TextEditKeys.Cancel):
		// Cancel editing package name
		m.applicationForm.editingPackage = false
		m.applicationForm.packageNameInput.SetValue("")
		m.applicationForm.err = ""
		return m, nil

	case key.Matches(msg, SearchKeys.Confirm):
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

	if m, cmd, handled := m.handleCommonKeys(msg); handled {
		return m, cmd
	}

	switch {
	case key.Matches(msg, FormNavKeys.Cancel):
		m.activeForm = FormNone
		m.applicationForm = nil
		m.Screen = ScreenResults
		return m, nil

	case key.Matches(msg, FormNavKeys.Up):
		if m.applicationForm.gitFieldCursor > 0 {
			m.applicationForm.gitFieldCursor--
		} else {
			// Back to git label (will route to updateApplicationPackagesList on next keypress)
			m.applicationForm.gitFieldCursor = -1
		}
		return m, nil

	case key.Matches(msg, FormNavKeys.Down):
		if m.applicationForm.gitFieldCursor < GitFieldCount-1 {
			m.applicationForm.gitFieldCursor++
		} else {
			// Move to installer item (next in packages list)
			m.applicationForm.packagesCursor = len(displayPackageManagers) + 1
			m.applicationForm.gitFieldCursor = -1
			m.applicationForm.installerFieldCursor = -1
		}
		return m, nil

	case key.Matches(msg, FormNavKeys.Edit):
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

	case key.Matches(msg, FormNavKeys.Toggle):
		if m.applicationForm.gitFieldCursor == GitFieldSudo {
			m.applicationForm.gitSudo = !m.applicationForm.gitSudo
		}
		return m, nil

	case key.Matches(msg, FormNavKeys.TabNext):
		m.applicationForm.focusIndex++
		if m.applicationForm.focusIndex > 3 {
			m.applicationForm.focusIndex = 0
		}
		m.applicationForm.packagesCursor = 0
		m.applicationForm.gitFieldCursor = -1
		m.updateApplicationFormFocus()
		return m, nil

	case key.Matches(msg, FormNavKeys.TabPrev):
		m.applicationForm.focusIndex--
		m.applicationForm.gitFieldCursor = -1
		m.updateApplicationFormFocus()
		return m, nil

	case key.Matches(msg, FormNavKeys.Save):
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

	if m, cmd, handled := m.handleTextEditKeys(msg); handled {
		return m, cmd
	}

	switch {
	case key.Matches(msg, TextEditKeys.Cancel):
		// Restore original value and exit edit mode
		input := m.getGitFieldInput()
		if input != nil {
			input.SetValue(m.applicationForm.originalValue)
		}
		m.applicationForm.editingGitField = false
		return m, nil

	case key.Matches(msg, TextEditKeys.Confirm):
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

// updateApplicationInstallerFields handles navigation within installer sub-fields (installerFieldCursor >= 0)
func (m Model) updateApplicationInstallerFields(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.applicationForm == nil {
		return m, nil
	}

	if m, cmd, handled := m.handleCommonKeys(msg); handled {
		return m, cmd
	}

	switch {
	case key.Matches(msg, FormNavKeys.Cancel):
		m.activeForm = FormNone
		m.applicationForm = nil
		m.Screen = ScreenResults
		return m, nil

	case key.Matches(msg, FormNavKeys.Up):
		if m.applicationForm.installerFieldCursor > 0 {
			m.applicationForm.installerFieldCursor--
		} else {
			// Back to installer label (will route to updateApplicationPackagesList on next keypress)
			m.applicationForm.installerFieldCursor = -1
		}
		return m, nil

	case key.Matches(msg, FormNavKeys.Down):
		if m.applicationForm.installerFieldCursor < InstallerFieldCount-1 {
			m.applicationForm.installerFieldCursor++
		} else {
			// Move to When section
			m.applicationForm.focusIndex++
			if m.applicationForm.focusIndex > 3 {
				m.applicationForm.focusIndex = 0
			}
			m.applicationForm.packagesCursor = 0
			m.applicationForm.installerFieldCursor = -1
			m.updateApplicationFormFocus()
		}
		return m, nil

	case key.Matches(msg, FormNavKeys.Edit):
		// Enter edit mode for text fields
		input := m.getInstallerFieldInput()
		if input != nil {
			m.applicationForm.editingInstallerField = true
			m.applicationForm.originalValue = input.Value()
			input.Focus()
			input.SetCursor(len(input.Value()))
		}
		return m, nil

	case key.Matches(msg, FormNavKeys.TabNext):
		m.applicationForm.focusIndex++
		if m.applicationForm.focusIndex > 3 {
			m.applicationForm.focusIndex = 0
		}
		m.applicationForm.packagesCursor = 0
		m.applicationForm.installerFieldCursor = -1
		m.updateApplicationFormFocus()
		return m, nil

	case key.Matches(msg, FormNavKeys.TabPrev):
		m.applicationForm.focusIndex--
		m.applicationForm.installerFieldCursor = -1
		m.updateApplicationFormFocus()
		return m, nil

	case key.Matches(msg, FormNavKeys.Save):
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

// updateApplicationInstallerFieldInput handles text input when editing an installer field
func (m Model) updateApplicationInstallerFieldInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.applicationForm == nil {
		return m, nil
	}

	var cmd tea.Cmd

	if m, cmd, handled := m.handleTextEditKeys(msg); handled {
		return m, cmd
	}

	switch {
	case key.Matches(msg, TextEditKeys.Cancel):
		// Restore original value and exit edit mode
		input := m.getInstallerFieldInput()
		if input != nil {
			input.SetValue(m.applicationForm.originalValue)
		}
		m.applicationForm.editingInstallerField = false
		return m, nil

	case key.Matches(msg, TextEditKeys.Confirm):
		// Save current value and exit edit mode
		m.applicationForm.editingInstallerField = false
		return m, nil
	}

	// Pass to the focused text input
	input := m.getInstallerFieldInput()
	if input != nil {
		*input, cmd = input.Update(msg)
	}

	m.applicationForm.err = ""
	return m, cmd
}

// getInstallerFieldInput returns a pointer to the current installer text input based on installerFieldCursor
func (m *Model) getInstallerFieldInput() *textinput.Model {
	if m.applicationForm == nil {
		return nil
	}

	switch m.applicationForm.installerFieldCursor {
	case InstallerFieldLinux:
		return &m.applicationForm.installerLinuxInput
	case InstallerFieldWindows:
		return &m.applicationForm.installerWindowsInput
	case InstallerFieldBinary:
		return &m.applicationForm.installerBinaryInput
	default:
		return nil
	}
}

// updateApplicationWhenInput handles key events when editing the when expression
func (m Model) updateApplicationWhenInput(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.applicationForm == nil {
		return m, nil
	}

	var cmd tea.Cmd

	if m, cmd, handled := m.handleTextEditKeys(msg); handled {
		return m, cmd
	}

	switch {
	case key.Matches(msg, TextEditKeys.Cancel):
		// Cancel editing and restore original value
		m.applicationForm.whenInput.SetValue(m.applicationForm.originalValue)
		m.applicationForm.editingWhen = false
		m.applicationForm.whenInput.Blur()
		m.applicationForm.err = ""
		return m, nil

	case key.Matches(msg, TextEditKeys.Confirm):
		// Save and exit edit mode
		m.applicationForm.editingWhen = false
		m.applicationForm.whenInput.Blur()
		return m, nil
	}

	// Handle text input
	m.applicationForm.whenInput, cmd = m.applicationForm.whenInput.Update(msg)
	m.applicationForm.err = ""

	return m, cmd
}

// viewApplicationForm renders the application form
func (m Model) viewApplicationForm() string {
	if m.applicationForm == nil {
		return ""
	}

	var b strings.Builder
	ft := m.getApplicationFieldType()

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
	onInstallerItem := ft == appFieldPackages && m.applicationForm.packagesCursor == len(displayPackageManagers)+1
	b.WriteString(renderInstallerPackageSection(
		ft == appFieldPackages,
		onInstallerItem,
		m.applicationForm.hasInstallerPackage,
		m.applicationForm.installerFieldCursor,
		m.applicationForm.editingInstallerField,
		m.applicationForm.installerLinuxInput,
		m.applicationForm.installerWindowsInput,
		m.applicationForm.installerBinaryInput,
	))
	b.WriteString("\n")

	// When section
	whenLabel := "When:"
	if ft == appFieldWhen {
		whenLabel = HelpKeyStyle.Render("When:")
	}
	b.WriteString(fmt.Sprintf("  %s\n", whenLabel))
	b.WriteString(renderWhenField(
		ft == appFieldWhen,
		m.applicationForm.editingWhen,
		m.applicationForm.whenInput,
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
	case appFieldPackages, appFieldWhen:
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

	if m.applicationForm.editingGitField || m.applicationForm.editingInstallerField {
		return RenderHelpFromBindings(m.width,
			TextEditKeys.Confirm,
			TextEditKeys.SaveForm,
			TextEditKeys.Cancel,
		)
	}

	if m.applicationForm.editingPackage {
		return RenderHelpFromBindings(m.width,
			TextEditKeys.Confirm,
			TextEditKeys.SaveForm,
			TextEditKeys.Cancel,
		)
	}

	if m.applicationForm.editingWhen {
		return RenderHelpFromBindings(m.width,
			TextEditKeys.Confirm,
			TextEditKeys.SaveForm,
			TextEditKeys.Cancel,
		)
	}

	if m.applicationForm.editingField {
		return RenderHelpFromBindings(m.width,
			TextEditKeys.Confirm,
			TextEditKeys.SaveForm,
			TextEditKeys.Cancel,
		)
	}

	if ft == appFieldPackages {
		// Git package states
		if m.applicationForm.packagesCursor == len(displayPackageManagers) {
			if !m.applicationForm.hasGitPackage {
				addBinding := key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "add"))
				return RenderHelpFromBindings(m.width, addBinding, FormNavKeys.Save)
			}
			if m.applicationForm.gitFieldCursor == -1 {
				return RenderHelpFromBindings(m.width, FormNavKeys.Delete, FormNavKeys.Save)
			}
			if m.applicationForm.gitFieldCursor == GitFieldSudo {
				return RenderHelpFromBindings(m.width, FormNavKeys.Toggle, FormNavKeys.Save)
			}
			return RenderHelpFromBindings(m.width, FormNavKeys.Edit, FormNavKeys.Save)
		}
		// Installer package states
		if m.applicationForm.packagesCursor == len(displayPackageManagers)+1 {
			if !m.applicationForm.hasInstallerPackage {
				addBinding := key.NewBinding(key.WithKeys("enter"), key.WithHelp("enter", "add"))
				return RenderHelpFromBindings(m.width, addBinding, FormNavKeys.Save)
			}
			if m.applicationForm.installerFieldCursor == -1 {
				return RenderHelpFromBindings(m.width, FormNavKeys.Delete, FormNavKeys.Save)
			}
			return RenderHelpFromBindings(m.width, FormNavKeys.Edit, FormNavKeys.Save)
		}
		// Bounds check for packagesCursor
		if m.applicationForm.packagesCursor >= 0 && m.applicationForm.packagesCursor < len(displayPackageManagers) {
			manager := displayPackageManagers[m.applicationForm.packagesCursor]
			if m.applicationForm.packageManagers[manager] != "" {
				return RenderHelpFromBindings(m.width,
					FormNavKeys.Edit,
					FormNavKeys.Delete,
					FormNavKeys.Save,
				)
			}
		}
		return RenderHelpFromBindings(m.width,
			FormNavKeys.Edit,
			FormNavKeys.Save,
		)
	}

	if ft == appFieldWhen {
		return RenderHelpFromBindings(m.width,
			FormNavKeys.Edit,
			FormNavKeys.Save,
		)
	}

	// Text field focused (not editing)
	return RenderHelpFromBindings(m.width,
		FormNavKeys.Edit,
		FormNavKeys.Save,
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

	// Build when expression and package
	when := strings.TrimSpace(m.applicationForm.whenInput.Value())
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

	// Merge installer package data
	pkg = mergeInstallerPackage(
		pkg,
		m.applicationForm.hasInstallerPackage,
		m.applicationForm.installerLinuxInput,
		m.applicationForm.installerWindowsInput,
		m.applicationForm.installerBinaryInput,
	)

	// Validate installer package if present
	if m.applicationForm.hasInstallerPackage {
		installerLinux := strings.TrimSpace(m.applicationForm.installerLinuxInput.Value())
		installerWindows := strings.TrimSpace(m.applicationForm.installerWindowsInput.Value())
		if installerLinux == "" && installerWindows == "" {
			return errors.New("installer package requires at least one command (Linux or Windows)")
		}
	}

	// Save based on edit mode
	if m.applicationForm.editAppIdx >= 0 {
		return m.saveEditedApplication(m.applicationForm.editAppIdx, name, description, when, pkg)
	}
	return m.saveNewApplication(config.Application{
		Name:        name,
		Description: description,
		When:        when,
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
	case appFieldPackages:
		// List fields don't use textinput focus
	case appFieldWhen:
		// When field focus is handled separately
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
	case appFieldPackages:
		// List fields don't use text input editing
	case appFieldWhen:
		// When field has its own edit mode
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
	case appFieldPackages:
		// List fields don't use text input restoration
	case appFieldWhen:
		// When field has its own cancel handling
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

	whenInput := textinput.New()
	whenInput.Placeholder = PlaceholderWhen
	whenInput.CharLimit = 512
	whenInput.Width = 60
	whenInput.SetValue(app.When)

	editAppIdx := -1
	if isEdit {
		editAppIdx = 0
	}

	gitURLInput, gitBranchInput, gitLinuxInput, gitWindowsInput := newGitTextInputs()
	installerLinuxInput, installerWindowsInput, installerBinaryInput := newInstallerTextInputs()

	// Load package managers (only string-based managers, skip git and installer)
	packageManagers := make(map[string]string)
	if app.Package != nil && len(app.Package.Managers) > 0 {
		for k, v := range app.Package.Managers {
			if k == TypeGit || k == TypeInstaller {
				continue
			}
			if !v.IsGit() && !v.IsInstaller() {
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

	// Load installer package if present
	hasInstallerPackage := false

	if app.Package != nil {
		if installerVal, ok := app.Package.Managers[TypeInstaller]; ok && installerVal.IsInstaller() {
			hasInstallerPackage = true
			if cmd, ok := installerVal.Installer.Command[OSLinux]; ok {
				installerLinuxInput.SetValue(cmd)
			}
			if cmd, ok := installerVal.Installer.Command[OSWindows]; ok {
				installerWindowsInput.SetValue(cmd)
			}
			installerBinaryInput.SetValue(installerVal.Installer.Binary)
		}
	}

	return &ApplicationForm{
		nameInput:             nameInput,
		descriptionInput:      descriptionInput,
		whenInput:             whenInput,
		packageManagers:       packageManagers,
		editAppIdx:            editAppIdx,
		gitURLInput:           gitURLInput,
		gitBranchInput:        gitBranchInput,
		gitLinuxInput:         gitLinuxInput,
		gitWindowsInput:       gitWindowsInput,
		gitFieldCursor:        -1,
		hasGitPackage:         hasGitPackage,
		gitSudo:               gitSudo,
		installerLinuxInput:   installerLinuxInput,
		installerWindowsInput: installerWindowsInput,
		installerBinaryInput:  installerBinaryInput,
		installerFieldCursor:  -1,
		hasInstallerPackage:   hasInstallerPackage,
	}
}

// Validate checks if the ApplicationForm has valid data
func (f *ApplicationForm) Validate() error {
	if strings.TrimSpace(f.nameInput.Value()) == "" {
		return errors.New("application name is required")
	}
	return nil
}
