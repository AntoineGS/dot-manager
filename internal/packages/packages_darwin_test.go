//go:build darwin

package packages

import (
	"os/exec"
	"testing"

	"github.com/AntoineGS/tidydots/internal/platform"
	"github.com/AntoineGS/tidydots/internal/testutil"
)

func TestBuildCommand_DarwinBrew(t *testing.T) {
	t.Parallel()

	pkg := Package{
		Name: "test-pkg",
		Managers: map[PackageManager]ManagerValue{
			Brew: {PackageName: "neovim"},
		},
	}

	cmd := BuildCommand(pkg, string(Brew), "linux") // tidydots maps macOS to "linux"
	if cmd == nil {
		t.Fatal("BuildCommand() returned nil")
	}

	assertArgs(t, cmd, []string{"brew", "install", "neovim"})
}

func TestBuildCommand_DarwinCustomUsesShell(t *testing.T) {
	t.Parallel()

	// macOS uses sh -c like Linux (both POSIX)
	pkg := Package{
		Name:   "custom-tool",
		Custom: map[string]string{"linux": "brew install --cask firefox"},
	}

	cmd := BuildCommand(pkg, MethodCustom, "linux") // tidydots maps macOS to "linux"
	if cmd == nil {
		t.Fatal("BuildCommand() returned nil")
	}

	assertArgs(t, cmd, []string{"sh", "-c", "brew install --cask firefox"})
}

func TestBuildCommand_DarwinURLUsesCurl(t *testing.T) {
	t.Parallel()

	pkg := Package{
		Name: "url-tool",
		URL: map[string]URLInstall{
			"linux": { // tidydots maps macOS to "linux"
				URL:     "https://example.com/install.sh",
				Command: "{file}",
			},
		},
	}

	cmd := BuildCommand(pkg, MethodURL, "linux")
	if cmd == nil {
		t.Fatal("BuildCommand() returned nil")
	}

	// macOS URL install should use sh -c with curl (same as Linux)
	args := cmd.Args
	if args[0] != "sh" || args[1] != "-c" {
		t.Errorf("expected sh -c, got %v", args[:2])
	}
}

func TestDetectManagers_DarwinWithMocks(t *testing.T) {
	dir := t.TempDir()

	// Create mock brew binary
	testutil.CreateMockBinary(t, dir, "brew", 0, "", "")
	testutil.CreateMockBinary(t, dir, "git", 0, "", "")

	// Override PATH
	t.Setenv("PATH", testutil.PrependPath(t, dir))

	// Reset cache so detection runs fresh
	platform.ResetAvailableManagersCache()
	platform.SetDetectionHints("linux", false) // tidydots maps macOS to "linux"

	managers := platform.DetectAvailableManagers()

	found := make(map[string]bool)
	for _, m := range managers {
		found[m] = true
	}

	if !found["brew"] {
		t.Error("expected brew to be detected")
	}

	if !found["git"] {
		t.Error("expected git to be detected")
	}

	// Reset for other tests
	platform.ResetAvailableManagersCache()
}

func assertArgs(t *testing.T, cmd *exec.Cmd, wantArgs []string) {
	t.Helper()

	got := cmd.Args
	if len(got) != len(wantArgs) {
		t.Errorf("args length = %d, want %d\ngot:  %v\nwant: %v", len(got), len(wantArgs), got, wantArgs)
		return
	}

	for i := range got {
		if got[i] != wantArgs[i] {
			t.Errorf("args[%d] = %q, want %q", i, got[i], wantArgs[i])
		}
	}
}
