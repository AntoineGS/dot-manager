package tui

import (
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/packages"
	"github.com/AntoineGS/tidydots/internal/tui/detection"
)

// isPackageInstalledFromPackage checks if a package is installed using the packages package.
func isPackageInstalledFromPackage(pkg *config.EntryPackage, method, entryName, osType string) bool {
	return detection.IsPackageInstalled(pkg, method, entryName, osType)
}

// packageConfig shares repository selection preferences between status and install.
func (m Model) packageConfig() *packages.Config {
	cfg := &packages.Config{}
	if m.Config != nil {
		cfg.DefaultManager = packages.PackageManager(m.Config.DefaultManager)
		for _, mgr := range m.Config.ManagerPriority {
			cfg.ManagerPriority = append(cfg.ManagerPriority, packages.PackageManager(mgr))
		}
	}
	return cfg
}
