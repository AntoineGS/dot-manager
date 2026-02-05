package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/packages"
)

func TestRunInit(t *testing.T) {
	// Backup and restore the real app config to avoid polluting user's config
	appConfigPath := config.AppConfigPath()
	var originalConfig []byte
	var hadConfig bool
	if data, err := os.ReadFile(appConfigPath); err == nil { //nolint:gosec // test file path is controlled
		originalConfig = data
		hadConfig = true
	}
	t.Cleanup(func() {
		if hadConfig {
			_ = os.WriteFile(appConfigPath, originalConfig, 0600)
		} else {
			_ = os.Remove(appConfigPath)
		}
	})

	tests := []struct {
		setupPath   func(t *testing.T) string
		name        string
		errContains string
		wantErr     bool
	}{
		{
			name: "valid directory",
			setupPath: func(t *testing.T) string {
				return t.TempDir()
			},
			wantErr: false,
		},
		{
			name: "non-existent directory",
			setupPath: func(t *testing.T) string {
				tmpDir := t.TempDir()
				return filepath.Join(tmpDir, "non-existent")
			},
			wantErr:     true,
			errContains: "directory does not exist",
		},
		{
			name: "file instead of directory",
			setupPath: func(t *testing.T) string {
				tmpDir := t.TempDir()
				filePath := filepath.Join(tmpDir, "file.txt")
				if err := os.WriteFile(filePath, []byte("test"), 0600); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
				return filePath
			},
			wantErr:     true,
			errContains: "not a directory",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setupPath(t)
			args := []string{path}

			err := runInit(nil, args)

			if tt.wantErr {
				if err == nil {
					t.Errorf("runInit() expected error containing %q, got nil", tt.errContains)
					return
				}
				if tt.errContains != "" && !contains(err.Error(), tt.errContains) {
					t.Errorf("runInit() error = %v, want error containing %q", err, tt.errContains)
				}
			} else if err != nil {
				t.Errorf("runInit() unexpected error = %v", err)
			}
		})
	}
}

func TestGetConfigDir(t *testing.T) {
	// Save original value
	originalConfigDir := configDir
	defer func() {
		configDir = originalConfigDir
	}()

	// Backup and restore the real app config since TestRunInit may have modified it
	appConfigPath := config.AppConfigPath()
	var originalConfig []byte
	var hadConfig bool
	if data, err := os.ReadFile(appConfigPath); err == nil { //nolint:gosec // test file path is controlled
		originalConfig = data
		hadConfig = true
	}
	// Remove config file to test "app config not found" case
	if err := os.Remove(appConfigPath); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if hadConfig {
			if err := os.WriteFile(appConfigPath, originalConfig, 0600); err != nil {
				t.Fatal(err)
			}
		}
	})

	tests := []struct {
		name       string
		flagValue  string
		errMessage string
		wantAbs    bool
		wantErr    bool
	}{
		{
			name:      "flag override with relative path",
			flagValue: ".",
			wantAbs:   true,
			wantErr:   false,
		},
		{
			name:      "flag override with absolute path",
			flagValue: "/tmp/test-config",
			wantAbs:   true,
			wantErr:   false,
		},
		{
			name:      "empty flag falls back to app config",
			flagValue: "",
			wantErr:   true, // Will fail because app config was removed
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			configDir = tt.flagValue

			result, err := getConfigDir()

			if tt.wantErr {
				if err == nil {
					t.Errorf("getConfigDir() expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("getConfigDir() unexpected error = %v", err)
				return
			}

			if tt.wantAbs && !filepath.IsAbs(result) {
				t.Errorf("getConfigDir() = %v, want absolute path", result)
			}

			// When flag is set, result should be the absolute version of the flag
			if tt.flagValue != "" {
				expectedAbs, _ := filepath.Abs(tt.flagValue)
				if result != expectedAbs {
					t.Errorf("getConfigDir() = %v, want %v", result, expectedAbs)
				}
			}
		})
	}
}

func TestConvertToPackageManagers(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  []packages.PackageManager
	}{
		{
			name:  "empty slice",
			input: []string{},
			want:  []packages.PackageManager{},
		},
		{
			name:  "single manager",
			input: []string{"pacman"},
			want:  []packages.PackageManager{packages.PackageManager("pacman")},
		},
		{
			name:  "multiple managers",
			input: []string{"yay", "paru", "pacman"},
			want: []packages.PackageManager{
				packages.PackageManager("yay"),
				packages.PackageManager("paru"),
				packages.PackageManager("pacman"),
			},
		},
		{
			name:  "nil input",
			input: nil,
			want:  []packages.PackageManager{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := convertToPackageManagers(tt.input)

			if len(got) != len(tt.want) {
				t.Errorf("convertToPackageManagers() returned %d items, want %d", len(got), len(tt.want))
				return
			}

			for i, pm := range got {
				if pm != tt.want[i] {
					t.Errorf("convertToPackageManagers()[%d] = %v, want %v", i, pm, tt.want[i])
				}
			}
		})
	}
}

// contains checks if substr is in s
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && searchSubstring(s, substr)))
}

func searchSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
