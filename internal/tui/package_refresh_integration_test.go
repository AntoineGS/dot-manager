package tui

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/packages"
	"github.com/AntoineGS/tidydots/internal/platform"
)

func TestPackageStateCheckUsesInstallationPreferences(t *testing.T) {
	platform.ResetAvailableManagersCache()
	t.Cleanup(platform.ResetAvailableManagersCache)
	runner := cmdexec.NewStubRunner()
	for _, name := range platform.KnownPackageManagers {
		runner.AddPath(name, filepath.Join(t.TempDir(), name))
	}
	available := platform.DetectAvailableManagersWithRunner(runner)
	var regular []string
	for _, name := range available {
		if name != "git" && name != "installer" {
			regular = append(regular, name)
		}
	}
	if len(regular) < 2 {
		t.Skip("requires two supported managers")
	}
	preferred := regular[len(regular)-1]
	pkg := &config.EntryPackage{Managers: map[string]config.ManagerValue{}}
	for _, name := range regular {
		pkg.Managers[name] = config.ManagerValue{PackageName: "tidydots-test-nonexistent"}
	}
	m := newStatusRefreshModel(t, pkg)
	m.Config.ManagerPriority = []string{preferred}
	msg := m.packageStateCheckCmd(0)().(pkgCheckResultMsg)
	installation := m.newPackageExec(PackageItem{Name: "tool", Package: pkg})
	if msg.method != preferred || msg.method != installation.manager.GetInstallMethod(installation.pkg) {
		t.Fatalf("status method=%q, installation=%q, want %q", msg.method, installation.manager.GetInstallMethod(installation.pkg), preferred)
	}
}

func TestRefreshInvalidatesInstalledPackageSnapshot(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses a POSIX winget fixture")
	}
	dir := t.TempDir()
	script := "#!/bin/sh\nprintf 'Name  Id         Version\n------------------------\n'\nif [ \"$TEST_PACKAGE_INSTALLED\" = yes ]; then\n  printf 'Tool  Tool.Test  1.0\n'\nfi\n"
	if err := os.WriteFile(filepath.Join(dir, "winget"), []byte(script), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir)
	t.Cleanup(packages.ResetInstalledCache)
	for _, refresh := range []string{"manual", "post-install"} {
		t.Run(refresh, func(t *testing.T) {
			packages.ResetInstalledCache()
			t.Setenv("TEST_PACKAGE_INSTALLED", "no")
			if packages.IsInstalled(context.Background(), "Tool.Test", "winget") {
				t.Fatal("fixture package unexpectedly installed")
			}
			t.Setenv("TEST_PACKAGE_INSTALLED", "yes")
			m := NewModel(&config.Config{Version: 3}, &platform.Platform{OS: "linux"}, false)
			if refresh == "manual" {
				m.refreshAllStates()
			} else {
				m.refreshPackageStates(nil)
			}
			if !packages.IsInstalled(context.Background(), "Tool.Test", "winget") {
				t.Fatal("refresh reused the pre-install snapshot")
			}
		})
	}
}
