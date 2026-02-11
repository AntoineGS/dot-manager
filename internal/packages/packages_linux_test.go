//go:build linux

package packages

import (
	"os/exec"
	"testing"

	"github.com/AntoineGS/tidydots/internal/platform"
	"github.com/AntoineGS/tidydots/internal/testutil"
)

func TestBuildCommand_LinuxManagers(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manager  PackageManager
		pkgName  string
		wantArgs []string
	}{
		{
			name:     "pacman install",
			manager:  Pacman,
			pkgName:  "neovim",
			wantArgs: []string{"sudo", "pacman", "-S", "--noconfirm", "neovim"},
		},
		{
			name:     "apt install",
			manager:  Apt,
			pkgName:  "neovim",
			wantArgs: []string{"sudo", "apt-get", "install", "-y", "neovim"},
		},
		{
			name:     "dnf install",
			manager:  Dnf,
			pkgName:  "neovim",
			wantArgs: []string{"sudo", "dnf", "install", "-y", "neovim"},
		},
		{
			name:     "yay install",
			manager:  Yay,
			pkgName:  "neovim-git",
			wantArgs: []string{"yay", "-S", "--noconfirm", "neovim-git"},
		},
		{
			name:     "paru install",
			manager:  Paru,
			pkgName:  "neovim-git",
			wantArgs: []string{"paru", "-S", "--noconfirm", "neovim-git"},
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

			cmd := BuildCommand(pkg, string(tt.manager), "linux")
			if cmd == nil {
				t.Fatal("BuildCommand() returned nil")
			}

			assertArgs(t, cmd, tt.wantArgs)
		})
	}
}

func TestBuildCommand_LinuxCustomUsesShell(t *testing.T) {
	t.Parallel()

	pkg := Package{
		Name:   "custom-tool",
		Custom: map[string]string{"linux": "make install"},
	}

	cmd := BuildCommand(pkg, MethodCustom, "linux")
	if cmd == nil {
		t.Fatal("BuildCommand() returned nil")
	}

	// Linux custom commands should use sh -c
	assertArgs(t, cmd, []string{"sh", "-c", "make install"})
}

func TestBuildCommand_LinuxURLUsesCurl(t *testing.T) {
	t.Parallel()

	pkg := Package{
		Name: "url-tool",
		URL: map[string]URLInstall{
			"linux": {
				URL:     "https://example.com/install.sh",
				Command: "{file}",
			},
		},
	}

	cmd := BuildCommand(pkg, MethodURL, "linux")
	if cmd == nil {
		t.Fatal("BuildCommand() returned nil")
	}

	// Linux URL install should use sh -c with curl
	args := cmd.Args
	if args[0] != "sh" || args[1] != "-c" {
		t.Errorf("expected sh -c, got %v", args[:2])
	}
}

func TestDetectManagers_LinuxWithMocks(t *testing.T) {
	dir := t.TempDir()

	// Create mock pacman and apt binaries
	testutil.CreateMockBinary(t, dir, "pacman", 0, "", "")
	testutil.CreateMockBinary(t, dir, "apt", 0, "", "")

	// Override PATH
	t.Setenv("PATH", testutil.PrependPath(t, dir))

	// Reset cache so detection runs fresh
	platform.ResetAvailableManagersCache()
	platform.SetDetectionHints("linux", false)

	managers := platform.DetectAvailableManagers()

	found := make(map[string]bool)
	for _, m := range managers {
		found[m] = true
	}

	if !found["pacman"] {
		t.Error("expected pacman to be detected")
	}

	if !found["apt"] {
		t.Error("expected apt to be detected")
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
