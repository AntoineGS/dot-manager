package testutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCreateMockBinary(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		binName    string
		exitCode   int
		stdout     string
		stderr     string
		wantStdout string
		wantErr    bool
	}{
		{
			name:       "exit 0 with stdout",
			binName:    "mytool",
			exitCode:   0,
			stdout:     "hello world",
			wantStdout: "hello world",
		},
		{
			name:     "exit 1 no output",
			binName:  "failing",
			exitCode: 1,
			wantErr:  true,
		},
		{
			name:       "exit 0 with stderr",
			binName:    "warns",
			exitCode:   0,
			stderr:     "warning message",
			wantStdout: "",
		},
		{
			name:       "exit 0 with both stdout and stderr",
			binName:    "both",
			exitCode:   0,
			stdout:     "output",
			stderr:     "warning",
			wantStdout: "output",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()

			path := CreateMockBinary(t, dir, tt.binName, tt.exitCode, tt.stdout, tt.stderr)

			// Verify file exists
			if _, err := os.Stat(path); os.IsNotExist(err) {
				t.Fatalf("mock binary was not created at %s", path)
			}

			// Execute and check output
			cmd := exec.Command(path)
			output, err := cmd.Output()

			if tt.wantErr && err == nil {
				t.Error("expected non-zero exit code, got nil error")
			}

			if !tt.wantErr && err != nil {
				t.Errorf("expected exit 0, got error: %v", err)
			}

			got := strings.TrimSpace(string(output))
			if got != tt.wantStdout {
				t.Errorf("stdout = %q, want %q", got, tt.wantStdout)
			}
		})
	}
}

func TestCreateMockBinary_IsExecutable(t *testing.T) {
	dir := t.TempDir()

	CreateMockBinary(t, dir, "checkexec", 0, "", "")

	// Should be findable via LookPath when dir is in PATH
	pathEnv := PrependPath(t, dir)

	lookFor := "checkexec"
	if runtime.GOOS == "windows" {
		lookFor = "checkexec.bat"
	}

	// exec.LookPath uses PATH env var, so we need to set it
	t.Setenv("PATH", pathEnv)

	found, err := exec.LookPath(lookFor)
	if err != nil {
		t.Fatalf("LookPath(%q) error: %v", lookFor, err)
	}

	if found == "" {
		t.Error("LookPath returned empty path")
	}
}

func TestPrependPath(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	result := PrependPath(t, dir)

	// Should start with our dir
	if !strings.HasPrefix(result, dir) {
		t.Errorf("PrependPath result %q does not start with %q", result, dir)
	}

	// Should contain the separator
	sep := string(os.PathListSeparator)
	if !strings.Contains(result, sep) {
		t.Errorf("PrependPath result %q does not contain separator %q", result, sep)
	}

	// Should contain original PATH
	originalPath := os.Getenv("PATH")
	if !strings.Contains(result, originalPath) {
		t.Errorf("PrependPath result does not contain original PATH")
	}
}

func TestCreateMockBinary_WindowsExtension(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	path := CreateMockBinary(t, dir, "mytool", 0, "", "")

	if runtime.GOOS == "windows" {
		if filepath.Ext(path) != ".bat" {
			t.Errorf("on Windows, expected .bat extension, got %q", filepath.Ext(path))
		}
	} else {
		if filepath.Ext(path) != "" {
			t.Errorf("on Unix, expected no extension, got %q", filepath.Ext(path))
		}
	}
}
