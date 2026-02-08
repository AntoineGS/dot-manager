package tui

import (
	"fmt"
	"strings"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
	"github.com/charmbracelet/bubbles/textinput"
)

// displayPackageManagers is platform.KnownPackageManagers excluding "git"
// (git is handled as a special case, not shown in the package manager form)
var displayPackageManagers = func() []string {
	var managers []string
	for _, m := range platform.KnownPackageManagers {
		if m != "git" {
			managers = append(managers, m)
		}
	}
	return managers
}()

// newGitTextInputs creates the four git text inputs with standard placeholders and char limits
func newGitTextInputs() (gitURLInput, gitBranchInput, gitLinuxInput, gitWindowsInput textinput.Model) {
	gitURLInput = textinput.New()
	gitURLInput.Placeholder = PlaceholderGitURL
	gitURLInput.CharLimit = 256
	gitURLInput.Width = 40

	gitBranchInput = textinput.New()
	gitBranchInput.Placeholder = PlaceholderGitBranch
	gitBranchInput.CharLimit = 128
	gitBranchInput.Width = 40

	gitLinuxInput = textinput.New()
	gitLinuxInput.Placeholder = PlaceholderGitLinux
	gitLinuxInput.CharLimit = 256
	gitLinuxInput.Width = 40

	gitWindowsInput = textinput.New()
	gitWindowsInput.Placeholder = PlaceholderGitWindows
	gitWindowsInput.CharLimit = 256
	gitWindowsInput.Width = 40

	return gitURLInput, gitBranchInput, gitLinuxInput, gitWindowsInput
}

// newInstallerTextInputs creates the three installer text inputs with standard placeholders and char limits
func newInstallerTextInputs() (installerLinuxInput, installerWindowsInput, installerBinaryInput textinput.Model) {
	installerLinuxInput = textinput.New()
	installerLinuxInput.Placeholder = PlaceholderInstallerLinux
	installerLinuxInput.CharLimit = 512
	installerLinuxInput.Width = 40

	installerWindowsInput = textinput.New()
	installerWindowsInput.Placeholder = PlaceholderInstallerWindows
	installerWindowsInput.CharLimit = 512
	installerWindowsInput.Width = 40

	installerBinaryInput = textinput.New()
	installerBinaryInput.Placeholder = PlaceholderInstallerBinary
	installerBinaryInput.CharLimit = 128
	installerBinaryInput.Width = 40

	return installerLinuxInput, installerWindowsInput, installerBinaryInput
}

// renderPackagesSection renders the packages list with editing state
// focused indicates if the packages section is currently focused
// packageManagers is the map of manager -> package name
// packagesCursor is the current cursor position within the list
// editingPackage indicates if currently editing a package name
// packageNameInput is the text input for editing package name
func renderPackagesSection(
	focused bool,
	packageManagers map[string]string,
	packagesCursor int,
	editingPackage bool,
	packageNameInput textinput.Model,
) string {
	var b strings.Builder

	for i, manager := range displayPackageManagers {
		prefix := IndentSpaces
		pkgName := packageManagers[manager]

		// Show input if editing this manager's package
		switch {
		case editingPackage && packagesCursor == i:
			b.WriteString(fmt.Sprintf("%s%-8s %s\n", prefix, manager+":", packageNameInput.View()))
		case focused && packagesCursor == i:
			// Focused on this manager
			if pkgName != "" {
				b.WriteString(fmt.Sprintf("%s%s\n", prefix, SelectedMenuItemStyle.Render(fmt.Sprintf("%-8s %s", manager+":", pkgName))))
			} else {
				b.WriteString(fmt.Sprintf("%s%s\n", prefix, SelectedMenuItemStyle.Render(fmt.Sprintf("%-8s (not set)", manager+":"))))
			}
		default:
			// Not focused
			if pkgName != "" {
				b.WriteString(fmt.Sprintf("%s%-8s %s\n", prefix, manager+":", pkgName))
			} else {
				b.WriteString(fmt.Sprintf("%s%-8s %s\n", prefix, manager+":", MutedTextStyle.Render("(not set)")))
			}
		}
	}

	return b.String()
}

// renderGitPackageSection renders the git package section within Packages
// focused indicates if the packages section is currently focused
// onGitItem indicates if the packagesCursor is on the git item (last position)
func renderGitPackageSection(
	focused bool,
	onGitItem bool,
	hasGitPackage bool,
	gitFieldCursor int,
	editingGitField bool,
	gitURLInput textinput.Model,
	gitBranchInput textinput.Model,
	gitLinuxInput textinput.Model,
	gitWindowsInput textinput.Model,
	gitSudo bool,
) string {
	var b strings.Builder
	prefix := IndentSpaces

	if !hasGitPackage {
		// Collapsed: show add button
		addText := "[+ Add Git Package]"
		if focused && onGitItem {
			b.WriteString(fmt.Sprintf("%s%s\n", prefix, SelectedMenuItemStyle.Render(addText)))
		} else {
			b.WriteString(fmt.Sprintf("%s%s\n", prefix, MutedTextStyle.Render(addText)))
		}
		return b.String()
	}

	// Expanded: show git label and sub-fields
	gitLabel := "git:"
	if focused && onGitItem && gitFieldCursor == -1 {
		b.WriteString(fmt.Sprintf("%s%s\n", prefix, SelectedMenuItemStyle.Render(gitLabel)))
	} else {
		b.WriteString(fmt.Sprintf("%s%s\n", prefix, gitLabel))
	}

	// Sub-fields with deeper indent
	onSubFields := focused && onGitItem && gitFieldCursor >= 0

	b.WriteString(renderGitField("URL:     ", gitURLInput, onSubFields, gitFieldCursor == GitFieldURL, editingGitField && gitFieldCursor == GitFieldURL))
	b.WriteString(renderGitField("Branch:  ", gitBranchInput, onSubFields, gitFieldCursor == GitFieldBranch, editingGitField && gitFieldCursor == GitFieldBranch))
	b.WriteString(renderGitField("Linux:   ", gitLinuxInput, onSubFields, gitFieldCursor == GitFieldLinux, editingGitField && gitFieldCursor == GitFieldLinux))
	b.WriteString(renderGitField("Windows: ", gitWindowsInput, onSubFields, gitFieldCursor == GitFieldWindows, editingGitField && gitFieldCursor == GitFieldWindows))

	// Sudo toggle
	subPrefix := IndentSpaces + "  "
	sudoText := CheckboxUnchecked + " No"
	if gitSudo {
		sudoText = CheckboxChecked + " Yes"
	}
	if onSubFields && gitFieldCursor == GitFieldSudo {
		b.WriteString(fmt.Sprintf("%sSudo:    %s\n", subPrefix, SelectedMenuItemStyle.Render(sudoText)))
	} else {
		b.WriteString(fmt.Sprintf("%sSudo:    %s\n", subPrefix, sudoText))
	}

	return b.String()
}

// renderGitField renders a single git text field with appropriate styling
func renderGitField(label string, input textinput.Model, onSubFields, isCurrent, isEditing bool) string {
	prefix := IndentSpaces + "  "

	if isEditing {
		return fmt.Sprintf("%s%s%s\n", prefix, label, input.View())
	}

	value := input.Value()
	if value == "" {
		value = MutedTextStyle.Render("(not set)")
	}

	if onSubFields && isCurrent {
		return fmt.Sprintf("%s%s\n", prefix, SelectedMenuItemStyle.Render(fmt.Sprintf("%s%s", label, value)))
	}

	return fmt.Sprintf("%s%s%s\n", prefix, label, value)
}

// renderInstallerPackageSection renders the installer package section within Packages
// focused indicates if the packages section is currently focused
// onInstallerItem indicates if the packagesCursor is on the installer item
func renderInstallerPackageSection(
	focused bool,
	onInstallerItem bool,
	hasInstallerPackage bool,
	installerFieldCursor int,
	editingInstallerField bool,
	installerLinuxInput textinput.Model,
	installerWindowsInput textinput.Model,
	installerBinaryInput textinput.Model,
) string {
	var b strings.Builder
	prefix := IndentSpaces

	if !hasInstallerPackage {
		// Collapsed: show add button
		addText := "[+ Add Installer Package]"
		if focused && onInstallerItem {
			b.WriteString(fmt.Sprintf("%s%s\n", prefix, SelectedMenuItemStyle.Render(addText)))
		} else {
			b.WriteString(fmt.Sprintf("%s%s\n", prefix, MutedTextStyle.Render(addText)))
		}
		return b.String()
	}

	// Expanded: show installer label and sub-fields
	installerLabel := "installer:"
	if focused && onInstallerItem && installerFieldCursor == -1 {
		b.WriteString(fmt.Sprintf("%s%s\n", prefix, SelectedMenuItemStyle.Render(installerLabel)))
	} else {
		b.WriteString(fmt.Sprintf("%s%s\n", prefix, installerLabel))
	}

	// Sub-fields with deeper indent
	onSubFields := focused && onInstallerItem && installerFieldCursor >= 0

	b.WriteString(renderGitField("Linux:   ", installerLinuxInput, onSubFields, installerFieldCursor == InstallerFieldLinux, editingInstallerField && installerFieldCursor == InstallerFieldLinux))
	b.WriteString(renderGitField("Windows: ", installerWindowsInput, onSubFields, installerFieldCursor == InstallerFieldWindows, editingInstallerField && installerFieldCursor == InstallerFieldWindows))
	b.WriteString(renderGitField("Binary:  ", installerBinaryInput, onSubFields, installerFieldCursor == InstallerFieldBinary, editingInstallerField && installerFieldCursor == InstallerFieldBinary))

	return b.String()
}

// renderWhenField renders the when expression text field
func renderWhenField(
	focused bool,
	editing bool,
	whenInput textinput.Model,
) string {
	prefix := IndentSpaces

	if editing {
		return fmt.Sprintf("%s%s\n", prefix, whenInput.View())
	}

	value := whenInput.Value()
	if value == "" {
		if focused {
			return fmt.Sprintf("%s%s\n", prefix, SelectedMenuItemStyle.Render("(no condition)"))
		}
		return fmt.Sprintf("%s%s\n", prefix, MutedTextStyle.Render("(no condition)"))
	}

	if focused {
		return fmt.Sprintf("%s%s\n", prefix, SelectedMenuItemStyle.Render(value))
	}

	return fmt.Sprintf("%s%s\n", prefix, value)
}

// buildPackageSpec creates a config.EntryPackage from a managers map
func buildPackageSpec(managers map[string]string) *config.EntryPackage {
	if len(managers) == 0 {
		return nil
	}

	managersTyped := make(map[string]config.ManagerValue, len(managers))
	for k, v := range managers {
		managersTyped[k] = config.ManagerValue{PackageName: v}
	}

	return &config.EntryPackage{
		Managers: managersTyped,
	}
}

// mergeGitPackage merges git package data into an existing EntryPackage
func mergeGitPackage(
	pkg *config.EntryPackage,
	hasGit bool,
	urlInput textinput.Model,
	branchInput textinput.Model,
	linuxInput textinput.Model,
	windowsInput textinput.Model,
	sudo bool,
) *config.EntryPackage {
	if !hasGit {
		return pkg
	}

	url := strings.TrimSpace(urlInput.Value())
	if url == "" {
		return pkg
	}

	if pkg == nil {
		pkg = &config.EntryPackage{
			Managers: make(map[string]config.ManagerValue),
		}
	}

	gitPkg := &config.GitPackage{
		URL:     url,
		Branch:  strings.TrimSpace(branchInput.Value()),
		Targets: make(map[string]string),
		Sudo:    sudo,
	}

	if linux := strings.TrimSpace(linuxInput.Value()); linux != "" {
		gitPkg.Targets[OSLinux] = linux
	}

	if windows := strings.TrimSpace(windowsInput.Value()); windows != "" {
		gitPkg.Targets[OSWindows] = windows
	}

	pkg.Managers[TypeGit] = config.ManagerValue{Git: gitPkg}

	return pkg
}

// mergeInstallerPackage merges installer package data into an existing EntryPackage
func mergeInstallerPackage(
	pkg *config.EntryPackage,
	hasInstaller bool,
	linuxInput textinput.Model,
	windowsInput textinput.Model,
	binaryInput textinput.Model,
) *config.EntryPackage {
	if !hasInstaller {
		return pkg
	}

	linux := strings.TrimSpace(linuxInput.Value())
	windows := strings.TrimSpace(windowsInput.Value())

	if linux == "" && windows == "" {
		return pkg
	}

	if pkg == nil {
		pkg = &config.EntryPackage{
			Managers: make(map[string]config.ManagerValue),
		}
	}

	installerPkg := &config.InstallerPackage{
		Command: make(map[string]string),
		Binary:  strings.TrimSpace(binaryInput.Value()),
	}

	if linux != "" {
		installerPkg.Command[OSLinux] = linux
	}

	if windows != "" {
		installerPkg.Command[OSWindows] = windows
	}

	pkg.Managers[TypeInstaller] = config.ManagerValue{Installer: installerPkg}

	return pkg
}

// saveNewApplication saves a new Application to the config
func (m *Model) saveNewApplication(app config.Application) error {
	// Check for duplicate names
	for _, existing := range m.Config.Applications {
		if existing.Name == app.Name {
			return fmt.Errorf("an application with name '%s' already exists", app.Name)
		}
	}

	m.Config.Applications = append(m.Config.Applications, app)

	if err := config.Save(m.Config, m.ConfigPath); err != nil {
		// Rollback
		m.Config.Applications = m.Config.Applications[:len(m.Config.Applications)-1]
		return fmt.Errorf("failed to save config: %w", err)
	}

	m.initApplicationItems()

	return nil
}

// saveEditedApplication updates Application metadata only (no SubEntry changes)
func (m *Model) saveEditedApplication(appIdx int, name, description, when string, pkg *config.EntryPackage) error {
	app := &m.Config.Applications[appIdx]

	// Check for duplicate names (skip the one being edited)
	for i, existing := range m.Config.Applications {
		if i != appIdx && existing.Name == name {
			return fmt.Errorf("an application with name '%s' already exists", name)
		}
	}

	// Update Application metadata
	app.Name = name
	app.Description = description
	app.When = when
	app.Package = pkg

	if err := config.Save(m.Config, m.ConfigPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	m.initApplicationItems()

	return nil
}
