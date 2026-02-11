//go:build windows

package platform

import (
	"testing"
)

func TestDetectOS_Windows(t *testing.T) {
	t.Parallel()

	got := detectOS()
	if got != OSWindows {
		t.Errorf("detectOS() = %q, want %q", got, OSWindows)
	}
}

func TestDetectDisplay_Windows(t *testing.T) {
	t.Parallel()

	// Windows always has a display
	got := detectDisplay(OSWindows)
	if !got {
		t.Error("detectDisplay(windows) = false, want true")
	}
}

func TestDetectWSL_Windows(t *testing.T) {
	t.Parallel()

	// Real Windows is not WSL
	got := detectWSL()
	if got {
		t.Error("detectWSL() = true on real Windows, want false")
	}
}

func TestDetect_Windows(t *testing.T) {
	p := Detect()

	if p.OS != OSWindows {
		t.Errorf("Detect().OS = %q, want %q", p.OS, OSWindows)
	}

	if p.IsWSL {
		t.Error("Detect().IsWSL = true on real Windows")
	}

	if !p.HasDisplay {
		t.Error("Detect().HasDisplay = false on Windows, want true")
	}

	if p.Hostname == "" {
		t.Error("Detect().Hostname is empty")
	}

	if p.User == "" {
		t.Error("Detect().User is empty")
	}

	// Distro should be empty on Windows
	if p.Distro != "" {
		t.Errorf("Detect().Distro = %q on Windows, want empty", p.Distro)
	}
}
