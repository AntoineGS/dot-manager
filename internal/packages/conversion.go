package packages

import (
	"github.com/AntoineGS/tidydots/internal/config"
)

// FilterPackages returns packages that match the given when expressions.
// It evaluates each package's When expression and returns only those that match.
func FilterPackages(packages []Package, renderer config.PathRenderer) []Package {
	result := make([]Package, 0, len(packages))

	for _, pkg := range packages {
		if config.EvaluateWhen(pkg.When, renderer) {
			result = append(result, pkg)
		}
	}

	return result
}

// convertPackage converts a config.EntryPackage into Package fields (managers, custom, url).
// This is the shared conversion logic used by FromApplication and FromPackageSpec.
// Since GitConfig, InstallerConfig, ManagerValue, and URLInstall are now type aliases
// for the config package types, no field-by-field copying is needed.
func convertPackage(pkg *config.EntryPackage) (map[PackageManager]ManagerValue, map[string]string, map[string]URLInstall) {
	managers := make(map[PackageManager]ManagerValue, len(pkg.Managers))
	for k, v := range pkg.Managers {
		managers[PackageManager(k)] = v
	}

	urlInstalls := make(map[string]URLInstall, len(pkg.URL))
	for k, v := range pkg.URL {
		urlInstalls[k] = v
	}

	return managers, pkg.Custom, urlInstalls
}

// FromApplication creates a Package from a config.Application.
// It converts the application's package configuration into a Package struct,
// mapping managers and URL install configurations. Returns nil if the
// application does not have a package configuration.
func FromApplication(app config.Application) *Package {
	if app.Package == nil {
		return nil
	}

	managers, custom, urlInstalls := convertPackage(app.Package)

	return &Package{
		Name:        app.Name,
		Description: app.Description,
		Managers:    managers,
		Custom:      custom,
		URL:         urlInstalls,
		When:        app.When,
	}
}

// FromApplications creates a slice of Packages from a slice of config.Application.
// It filters the applications to only those with package configurations and
// converts each to a Package struct.
func FromApplications(apps []config.Application) []Package {
	result := make([]Package, 0, len(apps))

	for _, app := range apps {
		if pkg := FromApplication(app); pkg != nil {
			result = append(result, *pkg)
		}
	}

	return result
}

// FromPackageSpec creates a Package from a name and EntryPackage.
// This is used by the TUI's buildInstallCommand which only has a name and
// package spec (no description/when, which installation does not need).
// Returns nil if pkg is nil.
func FromPackageSpec(name string, pkg *config.EntryPackage) *Package {
	if pkg == nil {
		return nil
	}

	managers, custom, urlInstalls := convertPackage(pkg)

	return &Package{
		Name:     name,
		Managers: managers,
		Custom:   custom,
		URL:      urlInstalls,
	}
}
