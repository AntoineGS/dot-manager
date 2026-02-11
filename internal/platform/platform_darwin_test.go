//go:build darwin

package platform

import (
	"testing"
)

func TestDetectOS_Darwin(t *testing.T) {
	t.Parallel()

	// tidydots maps macOS to "linux" (non-Windows = linux)
	got := detectOS()
	if got != OSLinux {
		t.Errorf("detectOS() = %q on macOS, want %q (tidydots maps non-Windows to linux)", got, OSLinux)
	}
}

func TestDetectDistro_Darwin(t *testing.T) {
	t.Parallel()

	// macOS has no /etc/os-release, so distro should be empty
	got := detectDistro()
	if got != "" {
		t.Errorf("detectDistro() = %q on macOS, want empty (no /etc/os-release)", got)
	}
}

func TestDetectWSL_Darwin(t *testing.T) {
	t.Parallel()

	// macOS is never WSL
	got := detectWSL()
	if got {
		t.Error("detectWSL() = true on macOS, want false")
	}
}

func TestDetect_Darwin(t *testing.T) {
	p := Detect()

	if p.OS != OSLinux {
		t.Errorf("Detect().OS = %q on macOS, want %q", p.OS, OSLinux)
	}

	if p.IsWSL {
		t.Error("Detect().IsWSL = true on macOS")
	}

	if p.Hostname == "" {
		t.Error("Detect().Hostname is empty")
	}

	if p.User == "" {
		t.Error("Detect().User is empty")
	}

	// Distro should be empty on macOS
	if p.Distro != "" {
		t.Errorf("Detect().Distro = %q on macOS, want empty", p.Distro)
	}
}
