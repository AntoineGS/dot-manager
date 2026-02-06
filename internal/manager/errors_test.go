package manager

import (
	"errors"
	"fmt"
	"strings"
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
		err  error
		want error
		name string
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
	if !strings.Contains(errMsg, "restore") {
		t.Errorf("Error message missing operation: %s", errMsg)
	}

	if !strings.Contains(errMsg, "/home/user/.config") {
		t.Errorf("Error message missing path: %s", errMsg)
	}

	if !strings.Contains(errMsg, "file not found") {
		t.Errorf("Error message missing underlying error: %s", errMsg)
	}
}
