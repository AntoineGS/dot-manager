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
