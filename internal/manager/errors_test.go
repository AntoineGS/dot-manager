package manager

import (
	"errors"
	"fmt"
	"testing"
)

func TestPathError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		wantPath string
		wantOp   string
	}{
		{
			name:     "path_error_wraps_underlying",
			err:      NewPathError("restore", "/home/user/.config", fmt.Errorf("permission denied")),
			wantPath: "/home/user/.config",
			wantOp:   "restore",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var pathErr *PathError
			if !errors.As(tt.err, &pathErr) {
				t.Fatalf("error is not PathError: %v", tt.err)
			}
			if pathErr.Path != tt.wantPath {
				t.Errorf("got path %s, want %s", pathErr.Path, tt.wantPath)
			}
			if pathErr.Op != tt.wantOp {
				t.Errorf("got op %s, want %s", pathErr.Op, tt.wantOp)
			}
		})
	}
}

func TestSentinelErrors(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want error
	}{
		{
			name: "is_not_found",
			err:  fmt.Errorf("backup: %w", ErrBackupNotFound),
			want: ErrBackupNotFound,
		},
		{
			name: "is_already_exists",
			err:  fmt.Errorf("target: %w", ErrTargetExists),
			want: ErrTargetExists,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !errors.Is(tt.err, tt.want) {
				t.Errorf("errors.Is() = false, want true")
			}
		})
	}
}

func TestPathError_Unwrap(t *testing.T) {
	underlying := fmt.Errorf("permission denied")
	pathErr := NewPathError("restore", "/home/user", underlying)

	unwrapped := errors.Unwrap(pathErr)
	if unwrapped == nil {
		t.Fatal("Unwrap returned nil")
	}
	if unwrapped.Error() != underlying.Error() {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlying)
	}
}

func TestPathError_Error(t *testing.T) {
	pathErr := NewPathError("restore", "/home/user/.config", fmt.Errorf("file not found"))

	errMsg := pathErr.Error()
	if errMsg == "" {
		t.Error("Error() returned empty string")
	}

	// Should contain operation, path, and underlying error
	if !contains(errMsg, "restore") {
		t.Errorf("Error message missing operation: %s", errMsg)
	}
	if !contains(errMsg, "/home/user/.config") {
		t.Errorf("Error message missing path: %s", errMsg)
	}
	if !contains(errMsg, "file not found") {
		t.Errorf("Error message missing underlying error: %s", errMsg)
	}
}

func TestGitError(t *testing.T) {
	underlying := fmt.Errorf("command failed")
	gitErr := NewGitError("clone", "https://github.com/test/repo.git", "main", underlying)

	var ge *GitError
	if !errors.As(gitErr, &ge) {
		t.Fatal("error is not GitError")
	}

	if ge.Repo != "https://github.com/test/repo.git" {
		t.Errorf("Repo = %s, want %s", ge.Repo, "https://github.com/test/repo.git")
	}

	if ge.Branch != "main" {
		t.Errorf("Branch = %s, want %s", ge.Branch, "main")
	}

	if ge.Op != "clone" {
		t.Errorf("Op = %s, want %s", ge.Op, "clone")
	}

	errMsg := ge.Error()
	if !contains(errMsg, "git") {
		t.Errorf("Error message should mention git: %s", errMsg)
	}
	if !contains(errMsg, "https://github.com/test/repo.git") {
		t.Errorf("Error message should contain repo URL: %s", errMsg)
	}
	if !contains(errMsg, "main") {
		t.Errorf("Error message should contain branch: %s", errMsg)
	}
}

func TestGitError_Unwrap(t *testing.T) {
	underlying := fmt.Errorf("clone failed")
	gitErr := NewGitError("clone", "https://github.com/test/repo.git", "main", underlying)

	unwrapped := errors.Unwrap(gitErr)
	if unwrapped == nil {
		t.Fatal("Unwrap returned nil")
	}
	if unwrapped.Error() != underlying.Error() {
		t.Errorf("Unwrap() = %v, want %v", unwrapped, underlying)
	}
}

func contains(s, substr string) bool {
	return len(s) > 0 && len(substr) > 0 && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || findInString(s, substr)))
}

func findInString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
