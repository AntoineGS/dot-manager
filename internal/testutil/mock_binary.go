// Package testutil provides cross-platform test helpers.
package testutil

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// CreateMockBinary creates a fake executable in dir that writes stdout/stderr
// content and exits with the given code.
// On Unix: creates a shell script. On Windows: creates a .bat file.
// Returns the full path to the created binary.
func CreateMockBinary(t *testing.T, dir, name string, exitCode int, stdout, stderr string) string {
	t.Helper()

	if runtime.GOOS == "windows" {
		return createWindowsBat(t, dir, name, exitCode, stdout, stderr)
	}

	return createUnixScript(t, dir, name, exitCode, stdout, stderr)
}

func createUnixScript(t *testing.T, dir, name string, exitCode int, stdout, stderr string) string {
	t.Helper()

	path := filepath.Join(dir, name)

	var script string
	script = "#!/bin/sh\n"

	if stdout != "" {
		script += fmt.Sprintf("echo '%s'\n", stdout)
	}

	if stderr != "" {
		script += fmt.Sprintf("echo '%s' >&2\n", stderr)
	}

	script += fmt.Sprintf("exit %d\n", exitCode)

	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to create mock binary %s: %v", name, err)
	}

	return path
}

func createWindowsBat(t *testing.T, dir, name string, exitCode int, stdout, stderr string) string {
	t.Helper()

	path := filepath.Join(dir, name+".bat")

	script := "@echo off\r\n"

	if stdout != "" {
		script += fmt.Sprintf("echo %s\r\n", stdout)
	}

	if stderr != "" {
		script += fmt.Sprintf("echo %s 1>&2\r\n", stderr)
	}

	script += fmt.Sprintf("exit /b %d\r\n", exitCode)

	if err := os.WriteFile(path, []byte(script), 0o755); err != nil {
		t.Fatalf("failed to create mock binary %s: %v", name, err)
	}

	return path
}

// PrependPath returns the current PATH with dir prepended, using the
// OS-appropriate path list separator.
func PrependPath(t *testing.T, dir string) string {
	t.Helper()

	return dir + string(os.PathListSeparator) + os.Getenv("PATH")
}
