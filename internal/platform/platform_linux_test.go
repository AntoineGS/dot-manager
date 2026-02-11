//go:build linux

package platform

import (
	"os"
	"strings"
	"testing"
)

func TestDetectOS_Linux(t *testing.T) {
	t.Parallel()

	got := detectOS()
	if got != OSLinux {
		t.Errorf("detectOS() = %q, want %q", got, OSLinux)
	}
}

func TestDetectDistro_Linux(t *testing.T) {
	t.Parallel()

	got := detectDistro()

	// On a real Linux system, /etc/os-release should exist and return a non-empty distro.
	// GitHub Actions ubuntu-latest has this.
	if _, err := os.Stat("/etc/os-release"); err == nil {
		if got == "" {
			t.Error("detectDistro() returned empty string, but /etc/os-release exists")
		}
	}
}

func TestDetectWSL_Linux(t *testing.T) {
	t.Parallel()

	got := detectWSL()

	// On real Linux (not WSL), /proc/version should not contain "microsoft"
	data, err := os.ReadFile("/proc/version")
	if err != nil {
		// No /proc/version means definitely not WSL
		if got {
			t.Error("detectWSL() = true, but /proc/version does not exist")
		}

		return
	}

	// If /proc/version doesn't contain "microsoft", WSL should be false
	content := string(data)
	if got && !containsWSLMarker(content) {
		t.Error("detectWSL() = true, but /proc/version has no WSL markers")
	}
}

func containsWSLMarker(s string) bool {
	lower := strings.ToLower(s)
	return strings.Contains(lower, "microsoft") || strings.Contains(lower, "wsl")
}

func TestDetectDisplay_Linux(t *testing.T) {
	tests := []struct {
		name           string
		display        string
		waylandDisplay string
		want           bool
	}{
		{
			name:    "X11 display set",
			display: ":0",
			want:    true,
		},
		{
			name:           "Wayland display set",
			waylandDisplay: "wayland-0",
			want:           true,
		},
		{
			name: "no display",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("DISPLAY", tt.display)
			t.Setenv("WAYLAND_DISPLAY", tt.waylandDisplay)

			got := detectDisplay(OSLinux)
			if got != tt.want {
				t.Errorf("detectDisplay(%q) = %v, want %v", OSLinux, got, tt.want)
			}
		})
	}
}

func TestDetect_Linux(t *testing.T) {
	p := Detect()

	if p.OS != OSLinux {
		t.Errorf("Detect().OS = %q, want %q", p.OS, OSLinux)
	}

	if p.Hostname == "" {
		t.Error("Detect().Hostname is empty")
	}

	if p.User == "" {
		t.Error("Detect().User is empty")
	}
}
