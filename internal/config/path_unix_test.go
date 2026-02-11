//go:build !windows

package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandPath_UnixTilde(t *testing.T) {
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
			name: "tilde expands to unix home",
			path: "~/Documents",
			want: filepath.Join(home, "Documents"),
		},
		{
			name: "home starts with slash",
			path: "~",
			want: home,
		},
		{
			name: "forward slashes in result",
			path: "~/.config/nvim",
			want: filepath.Join(home, ".config/nvim"),
		},
		{
			name:    "HOME env var expansion",
			path:    "$HOME/.config",
			envVars: nil,
			want:    filepath.Join(home, ".config"),
		},
		{
			name:    "XDG_CONFIG_HOME expansion",
			path:    "$XDG_CONFIG_HOME/nvim",
			envVars: map[string]string{"XDG_CONFIG_HOME": "/custom/config"},
			want:    "/custom/config/nvim",
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

func TestExpandPath_UnixHomeStartsWithSlash(t *testing.T) {
	t.Parallel()

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("cannot get home dir: %v", err)
	}

	// On Unix, home dir should start with /
	if !strings.HasPrefix(home, "/") {
		t.Errorf("UserHomeDir() = %q, expected to start with /", home)
	}

	got := ExpandPath("~", nil)
	if !strings.HasPrefix(got, "/") {
		t.Errorf("ExpandPath(~) = %q, expected to start with /", got)
	}
}
