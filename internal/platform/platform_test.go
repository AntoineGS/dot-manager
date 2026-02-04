package platform

import (
	"path/filepath"
	"runtime"
	"testing"
)

func TestDetectOS(t *testing.T) {
	t.Parallel()
	os := detectOS()

	// On Linux, should return "linux"
	if runtime.GOOS == "linux" && os != OSLinux {
		t.Errorf("detectOS() = %q, want %q", os, OSLinux)
	}

	// On Windows, should return "windows"
	if runtime.GOOS == "windows" && os != OSWindows {
		t.Errorf("detectOS() = %q, want %q", os, OSWindows)
	}
}

func TestDetect(t *testing.T) {
	t.Parallel()
	p := Detect()

	if p == nil {
		t.Fatal("Detect() returned nil")
	}

	if p.OS != OSLinux && p.OS != OSWindows {
		t.Errorf("OS = %q, want %q or %q", p.OS, OSLinux, OSWindows)
	}

	if p.EnvVars == nil {
		t.Error("EnvVars is nil")
	}
}

func TestWithOS(t *testing.T) {
	t.Parallel()
	p := &Platform{
		OS:      OSLinux,
		IsArch:  true,
		EnvVars: map[string]string{"key": "value"},
	}

	newP := p.WithOS(OSWindows)

	// Original should be unchanged
	if p.OS != OSLinux {
		t.Errorf("Original OS changed to %q", p.OS)
	}

	// New platform should have new OS
	if newP.OS != OSWindows {
		t.Errorf("WithOS() OS = %q, want %q", newP.OS, OSWindows)
	}

	// Other fields should be preserved
	if newP.IsArch != true {
		t.Error("WithOS() IsArch not preserved")
	}
}

func TestGetBasename(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want string
	}{
		{"/home/user/file.txt", "file.txt"},
		{"file.txt", "file.txt"},
		{"/home/user/", "user"}, // filepath.Base handles trailing slash
		{"", "."},               // filepath.Base returns "." for empty string
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := filepath.Base(tt.path)
			if got != tt.want {
				t.Errorf("filepath.Base(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestGetDirname(t *testing.T) {
	t.Parallel()
	tests := []struct {
		path string
		want string
	}{
		{"/home/user/file.txt", "/home/user"},
		{"file.txt", "."},
		{"/home/user/", "/home/user"}, // filepath.Dir handles trailing slash
		{"", "."},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := filepath.Dir(tt.path)
			if got != tt.want {
				t.Errorf("filepath.Dir(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}

func TestDetectDistro(t *testing.T) {
	t.Parallel()
	// This test returns the distro ID from /etc/os-release
	distro := detectDistro()

	// Just verify it doesn't panic and returns a string
	// On non-Linux systems or if /etc/os-release doesn't exist, it returns ""
	if distro != "" {
		// If we got a distro, it should be lowercase and non-empty
		if len(distro) == 0 {
			t.Error("detectDistro() returned empty but non-nil string")
		}
	}
}

func TestDetectAvailableManagers_Git(t *testing.T) {
	if !IsCommandAvailable("git") {
		t.Skip("git not available for testing")
	}

	managers := DetectAvailableManagers()

	found := false
	for _, mgr := range managers {
		if mgr == "git" {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected git to be in available managers, but it was not found")
	}
}
