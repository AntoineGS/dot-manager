package tui

import (
	"fmt"
	"strings"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
	"github.com/charmbracelet/bubbles/textinput"
)

// Filter add wizard step constants
const (
	filterStepType  = 0 // Select include/exclude
	filterStepKey   = 1 // Select attribute key (os, distro, hostname, user)
	filterStepValue = 2 // Enter value
)

// Filter attribute keys
var filterKeys = []string{"os", "distro", "hostname", "user"}

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

// renderFiltersSection renders the filters list with add/edit UI
// focused indicates if the filters section is currently focused
// filters is the list of filter conditions
// filtersCursor is the current cursor position
// addingFilter indicates if currently adding a filter
// editingFilter indicates if currently editing a filter
func renderFiltersSection(
	focused bool,
	filters []FilterCondition,
	filtersCursor int,
	addingFilter bool,
	editingFilter bool,
	filterAddStep int,
	filterIsExclude bool,
	filterKeyCursor int,
	editingFilterValue bool,
	filterValueInput textinput.Model,
) string {
	var b strings.Builder

	// Render filters based on state
	if addingFilter || editingFilter {
		// Show filter add/edit UI
		b.WriteString(renderFilterAddUI(
			addingFilter,
			filterAddStep,
			filterIsExclude,
			filterKeyCursor,
			editingFilterValue,
			filterValueInput,
		))
	} else {
		// Show filter list
		if len(filters) == 0 {
			b.WriteString(MutedTextStyle.Render("    (no filters)"))
			b.WriteString("\n")
		} else {
			for i, fc := range filters {
				prefix := IndentSpaces

				typeStr := "include"
				if fc.IsExclude {
					typeStr = "exclude"
				}
				condStr := fmt.Sprintf("%s: %s=%s", typeStr, fc.Key, fc.Value)

				if focused && filtersCursor == i {
					b.WriteString(fmt.Sprintf("%s%s\n", prefix, SelectedMenuItemStyle.Render("• "+condStr)))
				} else {
					b.WriteString(fmt.Sprintf("%s• %s\n", prefix, condStr))
				}
			}
		}

		// Add Filter button
		addFilterText := "[+ Add Filter]"
		if focused && filtersCursor == len(filters) {
			b.WriteString(fmt.Sprintf("    %s\n", SelectedMenuItemStyle.Render(addFilterText)))
		} else {
			b.WriteString(fmt.Sprintf("    %s\n", MutedTextStyle.Render(addFilterText)))
		}
	}

	return b.String()
}

// renderFilterAddUI renders the filter wizard UI for adding/editing a filter
func renderFilterAddUI(
	addingFilter bool,
	filterAddStep int,
	filterIsExclude bool,
	filterKeyCursor int,
	editingFilterValue bool,
	filterValueInput textinput.Model,
) string {
	var b strings.Builder

	actionText := "Add filter"
	if !addingFilter {
		actionText = "Edit filter"
	}

	b.WriteString(fmt.Sprintf("    %s:\n", MutedTextStyle.Render(actionText)))

	// Type selection (include/exclude)
	typeLabel := "    Type: "
	includeCheck := CheckboxUnchecked
	excludeCheck := CheckboxChecked

	if !filterIsExclude {
		includeCheck = CheckboxChecked
		excludeCheck = CheckboxUnchecked
	}

	typeStr := fmt.Sprintf("%s include  %s exclude", includeCheck, excludeCheck)
	if filterAddStep == filterStepType {
		b.WriteString(typeLabel + SelectedMenuItemStyle.Render(typeStr) + "\n")
	} else {
		b.WriteString(typeLabel + typeStr + "\n")
	}

	// Key selection
	keyLabel := "    Key:  "
	var keyOptions []string

	for i, k := range filterKeys {
		if i == filterKeyCursor {
			keyOptions = append(keyOptions, "["+k+"]")
		} else {
			keyOptions = append(keyOptions, " "+k+" ")
		}
	}

	keyStr := strings.Join(keyOptions, " ")
	if filterAddStep == filterStepKey {
		b.WriteString(keyLabel + SelectedMenuItemStyle.Render(keyStr) + "\n")
	} else {
		b.WriteString(keyLabel + keyStr + "\n")
	}

	// Value input
	valueLabel := "    Value: "
	switch {
	case filterAddStep == filterStepValue && editingFilterValue:
		// Actively editing - show the text input
		b.WriteString(valueLabel + filterValueInput.View() + "\n")
	case filterAddStep == filterStepValue:
		// Focused but not editing - show highlighted value
		value := filterValueInput.Value()
		if value == "" {
			value = "(enter value)"
		}

		b.WriteString(valueLabel + SelectedMenuItemStyle.Render(value) + "\n")
	default:
		// Not focused
		value := filterValueInput.Value()
		if value == "" {
			value = MutedTextStyle.Render("(enter value)")
		}

		b.WriteString(valueLabel + value + "\n")
	}

	return b.String()
}

// renderFormField renders a text input field with focus highlighting
// fieldName is the label for the field
// focused indicates if this field is currently focused
// editing indicates if the field is in edit mode
// input is the textinput.Model
// placeholder is shown when the field is empty

// buildFiltersFromConditions converts the flat FilterCondition list back to config.Filter format
func buildFiltersFromConditions(conditions []FilterCondition) []config.Filter {
	if len(conditions) == 0 {
		return nil
	}

	// Group conditions by filter index
	filterMap := make(map[int]*config.Filter)
	maxIndex := 0

	for _, fc := range conditions {
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
func (m *Model) saveEditedApplication(appIdx int, name, description string, filters []config.Filter, pkg *config.EntryPackage) error {
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
	app.Filters = filters
	app.Package = pkg

	if err := config.Save(m.Config, m.ConfigPath); err != nil {
		return fmt.Errorf("failed to save config: %w", err)
	}

	m.initApplicationItems()

	return nil
}
