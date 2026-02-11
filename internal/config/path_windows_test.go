//go:build windows

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandPath_WindowsTilde(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot get home dir: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		envVars map[string]string
		want    string
	}{
		{
			name: "tilde expands to windows home",
			path: "~/Documents",
			want: filepath.Join(home, "Documents"),
		},
		{
			name: "tilde only",
			path: "~",
			want: home,
		},
		{
			name: "tilde with AppData",
			path: "~/AppData/Local/nvim",
			want: filepath.Join(home, "AppData/Local/nvim"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := ExpandPath(tt.path, tt.envVars)
			if got != tt.want {
				t.Errorf("ExpandPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestExpandPath_WindowsHomeHasDriveLetter(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot get home dir: %v", err)
	}

	// On Windows, home dir should contain Users and a drive letter
	if !strings.Contains(home, `\Users\`) {
		t.Errorf("UserHomeDir() = %q, expected to contain \\Users\\", home)
	}

	got := ExpandPath("~", nil)
	if !filepath.IsAbs(got) {
		t.Errorf("ExpandPath(~) = %q, expected absolute path", got)
	}
}

func TestExpandPath_WindowsBackslashPaths(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot get home dir: %v", err)
	}

	// filepath.Join on Windows should produce backslash-separated paths
	got := ExpandPath("~/.config/nvim", nil)
	want := filepath.Join(home, ".config", "nvim")

	if got != want {
		t.Errorf("ExpandPath(~/.config/nvim) = %q, want %q", got, want)
	}

	// Result should contain backslashes (Windows path separator)
	if !strings.Contains(got, `\`) {
		t.Errorf("ExpandPath result %q should contain backslashes on Windows", got)
	}
}
