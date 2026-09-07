package packages

import (
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

func TestGitRejectsBlankExpandedTarget(t *testing.T) {
	// A cwd repository exposes the dangerous git -C "" pull fallback without
	// ever invoking real git: all execution is recorded by the stub runner.
	cwd := t.TempDir()
	if err := os.Mkdir(filepath.Join(cwd, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	t.Chdir(cwd)
	t.Setenv("TIDYDOTS_EMPTY_TARGET", "")
	for _, target := range []string{"", " \t", "$TIDYDOTS_EMPTY_TARGET"} {
		for _, dry := range []bool{false, true} {
			m, stub := newStubManager(t, "linux")
			m.DryRun = dry
			ok, msg := m.installGitPackage(GitConfig{URL: "https://example.com/repo.git", Targets: map[string]string{"linux": target}})
			if ok || msg != "No git target path defined for OS: linux" || len(stub.Calls) != 0 {
				t.Errorf("target=%q dry=%v success=%v msg=%q calls=%+v", target, dry, ok, msg, stub.Calls)
			}
		}
	}
}

func TestGitPreservesSpacesAndValidatesBranchBeforePull(t *testing.T) {
	name := " repo with spaces "
	if runtime.GOOS == "windows" {
		// Windows normal path handling does not support trailing spaces.
		name = " repo with spaces"
	}
	target := filepath.Join(t.TempDir(), name)
	if err := os.MkdirAll(filepath.Join(target, ".git"), 0700); err != nil {
		t.Fatal(err)
	}
	for _, dry := range []bool{false, true} {
		m, stub := newStubManager(t, "linux")
		m.DryRun = dry
		gitCfg := GitConfig{URL: "https://example.com/repo.git", Targets: map[string]string{"linux": target}, Branch: "--unsafe"}
		if ok, msg := m.installGitPackage(gitCfg); ok || !strings.HasPrefix(msg, "Invalid git branch:") || len(stub.Calls) != 0 {
			t.Fatalf("invalid branch accepted for existing repository: ok=%v msg=%q calls=%+v", ok, msg, stub.Calls)
		}
		gitCfg.Branch = "main"
		if ok, msg := m.installGitPackage(gitCfg); !ok {
			t.Fatal(msg)
		}
		if !dry && (len(stub.Calls) != 1 || !reflect.DeepEqual(stub.Calls[0].Args, []string{"-C", target, "pull"})) {
			t.Fatalf("target spaces lost: %+v", stub.Calls)
		}
	}
}

func TestInvalidMainNeverInstallsDependencies(t *testing.T) {
	t.Setenv("TIDYDOTS_EMPTY_TARGET", "")
	for _, tc := range []struct {
		name, want string
		pkg        Package
	}{
		{name: "installer missing OS", want: "No installer command defined for OS: linux", pkg: Package{Managers: map[PackageManager]ManagerValue{Installer: {Installer: &InstallerConfig{Command: map[string]string{"windows": "install-tool"}}}}}},
		{name: "URL rejected", want: "URL rejected:", pkg: Package{URL: map[string]URLInstall{"linux": {URL: "file:///etc/passwd", Command: "sh {file}"}}}},
		{name: "git target missing", want: "No git target path defined for OS: linux", pkg: Package{Managers: map[PackageManager]ManagerValue{Git: {Git: &GitConfig{URL: "https://example.com/repo.git"}}}}},
		{name: "git expanded target empty", want: "No git target path defined for OS: linux", pkg: Package{Managers: map[PackageManager]ManagerValue{Git: {Git: &GitConfig{URL: "https://example.com/repo.git", Targets: map[string]string{"linux": "$TIDYDOTS_EMPTY_TARGET"}}}}}},
		{name: "git branch rejected", want: "Invalid git branch:", pkg: Package{Managers: map[PackageManager]ManagerValue{Git: {Git: &GitConfig{URL: "https://example.com/repo.git", Targets: map[string]string{"linux": t.TempDir()}, Branch: "--unsafe"}}}}},
		{name: "git URL rejected", want: "Git URL rejected:", pkg: Package{Managers: map[PackageManager]ManagerValue{Git: {Git: &GitConfig{URL: "file:///repo", Targets: map[string]string{"linux": t.TempDir()}}}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.pkg.Managers == nil {
				tc.pkg.Managers = map[PackageManager]ManagerValue{}
			}
			tc.pkg.Managers[Apt] = ManagerValue{Deps: []string{"valid-dependency"}}
			for _, dry := range []bool{false, true} {
				m, stub := newStubManager(t, "linux")
				setAvailable(m, Apt)
				m.DryRun = dry
				result := m.Install(tc.pkg)
				if result.Success || !strings.HasPrefix(result.Message, tc.want) || len(stub.Calls) != 0 {
					t.Errorf("dry=%v result=%+v calls=%+v", dry, result, stub.Calls)
				}
			}
		})
	}
}
