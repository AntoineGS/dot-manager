package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/packages"
)

func TestPackageAdapterInvalidMainNeverRunsDependencies(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX command fixtures")
	}
	dir := t.TempDir()
	for _, name := range []string{"sudo", "git"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte("#!/bin/sh\nprintf 'called\\n' >> \"$INSTALL_LOG\"\n"), 0700); err != nil {
			t.Fatal(err)
		}
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv("TIDYDOTS_EMPTY_TARGET", "")
	for _, tc := range []struct {
		name, want string
		pkg        packages.Package
	}{
		{name: "missing installer OS", want: "No installer command defined for OS: linux", pkg: packages.Package{Managers: map[packages.PackageManager]packages.ManagerValue{packages.Installer: {Installer: &packages.InstallerConfig{Command: map[string]string{"windows": "install"}}}}}},
		{name: "unsafe URL", want: "URL rejected:", pkg: packages.Package{URL: map[string]packages.URLInstall{"linux": {URL: "file:///etc/passwd"}}}},
		{name: "empty git target", want: "No git target path defined for OS: linux", pkg: packages.Package{Managers: map[packages.PackageManager]packages.ManagerValue{packages.Git: {Git: &packages.GitConfig{URL: "https://example.com/repo.git", Targets: map[string]string{"linux": "$TIDYDOTS_EMPTY_TARGET"}}}}}},
		{name: "invalid git branch", want: "Invalid git branch:", pkg: packages.Package{Managers: map[packages.PackageManager]packages.ManagerValue{packages.Git: {Git: &packages.GitConfig{URL: "https://example.com/repo.git", Targets: map[string]string{"linux": dir}, Branch: "--unsafe"}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.pkg.Managers == nil {
				tc.pkg.Managers = map[packages.PackageManager]packages.ManagerValue{}
			}
			tc.pkg.Managers[packages.Apt] = packages.ManagerValue{Deps: []string{"dependency"}}
			for _, dry := range []bool{false, true} {
				logPath := filepath.Join(t.TempDir(), "commands")
				t.Setenv("INSTALL_LOG", logPath)
				mgr := packages.NewManager(&packages.Config{}, "linux", dry, false)
				mgr.Available = []packages.PackageManager{packages.Apt}
				ex := &packageExec{manager: mgr, pkg: tc.pkg}
				if err := ex.Run(); err == nil || ex.result.Success || !strings.HasPrefix(ex.result.Message, tc.want) {
					t.Errorf("dry=%v result=%+v error=%v", dry, ex.result, err)
				}
				if _, err := os.Stat(logPath); !os.IsNotExist(err) {
					t.Errorf("dry=%v: invalid main ran commands (stat=%v)", dry, err)
				}
			}
		})
	}
}
