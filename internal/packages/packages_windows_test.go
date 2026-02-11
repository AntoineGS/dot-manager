//go:build windows

package packages

import (
	"os/exec"
	"testing"

	"github.com/AntoineGS/tidydots/internal/platform"
	"github.com/AntoineGS/tidydots/internal/testutil"
)

func TestBuildCommand_WindowsManagers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manager  PackageManager
		pkgName  string
		wantArgs []string
	}{
		{
			name:     "winget install",
			manager:  Winget,
			pkgName:  "Microsoft.VisualStudioCode",
			wantArgs: []string{"winget", "install", "--accept-package-agreements", "--accept-source-agreements", "Microsoft.VisualStudioCode"},
		},
		{
			name:     "scoop install",
			manager:  Scoop,
			pkgName:  "neovim",
			wantArgs: []string{"scoop", "install", "neovim"},
		},
		{
			name:     "choco install",
			manager:  Choco,
			pkgName:  "neovim",
			wantArgs: []string{"choco", "install", "-y", "neovim"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			pkg := Package{
				Name: "test-pkg",
				Managers: map[PackageManager]ManagerValue{
					tt.manager: {PackageName: tt.pkgName},
				},
			}

			cmd := BuildCommand(pkg, string(tt.manager), "windows")
			if cmd == nil {
				t.Fatal("BuildCommand() returned nil")
			}

			assertArgs(t, cmd, tt.wantArgs)
		})
	}
}

func TestBuildCommand_WindowsCustomUsesPowerShell(t *testing.T) {
	t.Parallel()

	pkg := Package{
		Name:   "custom-tool",
		Custom: map[string]string{"windows": "msbuild /t:install"},
	}

	cmd := BuildCommand(pkg, MethodCustom, "windows")
	if cmd == nil {
		t.Fatal("BuildCommand() returned nil")
	}

	// Windows custom commands should use powershell -Command
	assertArgs(t, cmd, []string{"powershell", "-Command", "msbuild /t:install"})
}

func TestBuildCommand_WindowsURLUsesInvokeWebRequest(t *testing.T) {
	t.Parallel()

	pkg := Package{
		Name: "url-tool",
		URL: map[string]URLInstall{
			"windows": {
				URL:     "https://example.com/setup.exe",
				Command: "{file}",
			},
		},
	}

	cmd := BuildCommand(pkg, MethodURL, "windows")
	if cmd == nil {
		t.Fatal("BuildCommand() returned nil")
	}

	// Windows URL install should use powershell with Invoke-WebRequest
	args := cmd.Args
	if args[0] != "powershell" || args[1] != "-Command" {
		t.Errorf("expected powershell -Command, got %v", args[:2])
	}
}

func TestDetectManagers_WindowsWithMocks(t *testing.T) {
	dir := t.TempDir()

	// Create mock winget and scoop .bat files
	testutil.CreateMockBinary(t, dir, "winget", 0, "", "")
	testutil.CreateMockBinary(t, dir, "scoop", 0, "", "")

	// Override PATH
	t.Setenv("PATH", testutil.PrependPath(t, dir))

	// Reset cache so detection runs fresh
	platform.ResetAvailableManagersCache()
	platform.SetDetectionHints("windows", false)

	managers := platform.DetectAvailableManagers()

	found := make(map[string]bool)
	for _, m := range managers {
		found[m] = true
	}

	if !found["winget"] {
		t.Error("expected winget to be detected")
	}

	if !found["scoop"] {
		t.Error("expected scoop to be detected")
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
