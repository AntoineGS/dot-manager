package detection

import (
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/packages"
)

func TestPackageDetectionUsesDomainPlan(t *testing.T) {
	pkg := &config.EntryPackage{Managers: map[string]config.ManagerValue{
		"installer": {Installer: &config.InstallerPackage{Command: map[string]string{"linux": "install-tool"}}},
	}}
	var available []packages.PackageManager
	for _, mgr := range DetectAvailableManagers() {
		if mgr == "git" {
			continue
		}
		pkg.Managers[mgr] = config.ManagerValue{Deps: []string{"dependency"}}
		available = append(available, packages.PackageManager(mgr))
	}
	prefs := &packages.Config{ManagerPriority: available}
	converted := packages.FromPackageSpec("tool", pkg)
	plan := packages.PlanInstallation(*converted, prefs, "linux", available)
	if got := GetPackageInstallMethod(pkg, "linux", prefs); got != "installer" || got != plan.Method {
		t.Fatalf("detection=%s domain=%s", got, plan.Method)
	}
}
