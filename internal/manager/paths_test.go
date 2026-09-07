package manager

import (
	"path/filepath"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/platform"
)

func TestSharedPathResolution(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	m := New(&config.Config{BackupRoot: "~/repo/{{ .Hostname }}"}, &platform.Platform{OS: "linux", Hostname: "host"})
	for _, tc := range []struct {
		path string
		want string
	}{
		{"./{{ .Hostname }}/nvim", filepath.Join(home, "repo", "host", "host", "nvim")},
		{"~/.config/{{ .Hostname }}/nvim", filepath.Join(home, ".config", "host", "nvim")},
	} {
		got, err := m.ResolvePath(tc.path)
		if err != nil || got != tc.want {
			t.Errorf("ResolvePath(%q) = %q, %v; want %q", tc.path, got, err, tc.want)
		}
	}
	for _, path := range []string{"{{", "{{ .Unknown }}", "{{ missingFunction }}"} {
		if got, err := m.ResolvePath(path); err == nil || got != "" {
			t.Errorf("invalid template %q = %q, %v", path, got, err)
		}
		if got, err := m.ExpandTarget(path); err == nil || got != "" {
			t.Errorf("invalid target %q = %q, %v", path, got, err)
		}
	}
	m.Config.BackupRoot = "{{"
	if _, err := m.ResolvePath("relative"); err == nil {
		t.Fatal("invalid root accepted")
	}
	if err := m.InitStateStore(); err == nil {
		t.Fatal("invalid state root accepted")
	}
}
