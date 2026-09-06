package tui

import (
	"fmt"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/tui/forms"
)

// previewConfigChange records an honest preview before any configuration mutation.
func (m *Model) previewConfigChange(action, name string) bool {
	if !m.DryRun && (m.Manager == nil || !m.Manager.DryRun) {
		return false
	}
	m.results = []ResultItem{{Name: name, Success: true, Message: "Would " + action + " configuration (dry-run)"}}
	return true
}

// Re-export form helper functions from forms/ for backward compatibility.
var (
	displayPackageManagers        = forms.DisplayPackageManagers
	newFormInput                  = forms.NewFormInput
	newCommandInput               = forms.NewCommandInput
	newGitTextInputs              = forms.NewGitTextInputs
	newInstallerTextInputs        = forms.NewInstallerTextInputs
	renderPackagesSection         = forms.RenderPackagesSection
	renderDepsSection             = forms.RenderDepsSection
	renderGitPackageSection       = forms.RenderGitPackageSection
	renderInstallerPackageSection = forms.RenderInstallerPackageSection
	renderCustomPackageSection    = forms.RenderCustomPackageSection
	renderURLPackageSection       = forms.RenderURLPackageSection
	renderWhenField               = forms.RenderWhenField
	buildPackageSpec              = forms.BuildPackageSpec
	mergeGitPackage               = forms.MergeGitPackage
	mergeInstallerPackage         = forms.MergeInstallerPackage
)

// saveNewApplication saves a new Application to the config
func (m *Model) saveNewApplication(app config.Application) error {
	// Check for duplicate names
	for _, existing := range m.Config.Applications {
		if existing.Name == app.Name {
			return fmt.Errorf("an application with name '%s' already exists", app.Name)
		}
	}

	if m.previewConfigChange("add", app.Name) {
		return nil
	}
	m.Config.Applications = append(m.Config.Applications, app)

	if err := config.Save(m.Config, m.ConfigPath); err != nil {
		// Rollback
		m.Config.Applications = m.Config.Applications[:len(m.Config.Applications)-1]
		return fmt.Errorf("failed to save config: %w", err)
	}

	m.reinitPreservingState(app.Name)

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
	if m.previewConfigChange("edit", name) {
		return nil
	}
	origName, origDesc, origWhen, origPkg := app.Name, app.Description, app.When, app.Package
	app.Name = name
	app.Description = description
	app.When = when
	app.Package = pkg

	if err := config.Save(m.Config, m.ConfigPath); err != nil {
		app.Name, app.Description, app.When, app.Package = origName, origDesc, origWhen, origPkg
		return fmt.Errorf("failed to save config: %w", err)
	}

	m.reinitPreservingState(name)

	return nil
}
