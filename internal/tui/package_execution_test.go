package tui

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/packages"
	"github.com/AntoineGS/tidydots/internal/platform"
)

func TestPackageDryRunRejectsUnsafeURL(t *testing.T) {
	m := Model{DryRun: true, Platform: &platform.Platform{OS: "linux"}}
	m.pendingPackages = []PackageItem{{Name: "unsafe", Method: "url", Package: &config.EntryPackage{
		URL: map[string]config.URLInstallSpec{"linux": {URL: "file:///etc/passwd", Command: "sh {file}"}},
	}}}
	msg, ok := m.installNextPackage()().(PackageInstallMsg)
	if !ok || msg.Success || !strings.Contains(msg.Message, "rejected") {
		t.Fatalf("unsafe dry run result = %+v", msg)
	}
}

func TestPackageExecutionMatchesDomainDependencies(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("uses POSIX test executables")
	}
	for _, tc := range []struct {
		name      string
		fail, dry bool
		want      string
	}{
		{name: "dependency then installer", want: "dependency\nmain\n"},
		{name: "failed dependency stops installer", fail: true, want: "dependency\n"},
		{name: "dry run executes nothing", dry: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			for name, script := range map[string]string{
				"sudo":    "#!/bin/sh\nexec \"$@\"\n",
				"apt-get": "#!/bin/sh\nprintf 'dependency\\n' >> \"$INSTALL_LOG\"\nexit \"$DEPENDENCY_EXIT\"\n",
			} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(script), 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
			t.Setenv("DEPENDENCY_EXIT", "0")
			if tc.fail {
				t.Setenv("DEPENDENCY_EXIT", "1")
			}
			pkg := packages.Package{Name: "tool", Managers: map[packages.PackageManager]packages.ManagerValue{
				packages.Apt:       {Deps: []string{"dependency"}},
				packages.Installer: {Installer: &packages.InstallerConfig{Command: map[string]string{"linux": "printf 'main\\n' >> \"$INSTALL_LOG\""}}},
			}}
			for _, tui := range []bool{false, true} {
				logPath := filepath.Join(dir, "cli.log")
				if tui {
					logPath = filepath.Join(dir, "tui.log")
				}
				t.Setenv("INSTALL_LOG", logPath)
				mgr := packages.NewManager(&packages.Config{}, "linux", tc.dry, false)
				mgr.Available = []packages.PackageManager{packages.Apt}
				var result packages.InstallResult
				if tui {
					ex := &packageExec{manager: mgr, pkg: pkg}
					err := ex.Run()
					if (err != nil) != tc.fail {
						t.Fatalf("TUI error = %v", err)
					}
					result = ex.result
				} else {
					result = mgr.WithRunner(packageTerminalRunner{}).Install(pkg)
				}
				if result.Success == tc.fail {
					t.Fatalf("result = %+v", result)
				}
				got, err := os.ReadFile(logPath)
				if err != nil && (!tc.dry || !os.IsNotExist(err)) {
					t.Fatal(err)
				}
				if string(got) != tc.want {
					t.Fatalf("TUI=%v execution=%q, want %q", tui, got, tc.want)
				}
				if tc.dry && (!strings.Contains(result.Message, "dependency") || !strings.Contains(result.Message, "main")) {
					t.Fatalf("incomplete dry-run preview: %s", result.Message)
				}
			}
		})
	}
}

func TestPackageTerminalRunnerStdio(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("POSIX shell")
	}
	var stdout, stderr bytes.Buffer
	r := packageTerminalRunner{stdin: strings.NewReader("interactive\n"), stdout: &stdout, stderr: &stderr}
	_, err := r.Run(context.Background(), "sh", "-c", "read value; printf '%s' \"$value\"; printf diagnostic >&2")
	if err != nil || stdout.String() != "interactive" || stderr.String() != "diagnostic" {
		t.Fatalf("stdio: stdout=%q stderr=%q err=%v", stdout.String(), stderr.String(), err)
	}
}

func TestPackageExecUsesRepositoryPreferences(t *testing.T) {
	m := Model{DryRun: true, Platform: &platform.Platform{OS: "linux"}, Config: &config.Config{
		DefaultManager: "apt", ManagerPriority: []string{"brew", "apt"},
	}}
	item := PackageItem{Name: "tool", Method: "apt", Package: &config.EntryPackage{
		Managers: map[string]config.ManagerValue{
			"apt":  {PackageName: "apt-tool", Deps: []string{"apt-dep"}},
			"brew": {PackageName: "brew-tool"},
		},
	}}
	ex := m.buildInstallCommand(item)
	ex.manager.Available = []packages.PackageManager{packages.Apt, packages.Brew}
	if err := ex.Run(); err != nil {
		t.Fatal(err)
	}
	if ex.result.Method != "brew" || !strings.Contains(ex.result.Message, "apt-dep") || !strings.Contains(ex.result.Message, "brew-tool") {
		t.Fatalf("repository preferences lost or stale display method trusted: %+v", ex.result)
	}
}
