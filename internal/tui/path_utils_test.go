package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandTargetPath(t *testing.T) {
	t.Parallel()

	// Get current working directory for relative path tests
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("Failed to get current directory: %v", err)
	}

	// Get home directory for ~ expansion tests
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	tests := []struct {
		name       string
		input      string
		wantPrefix string // Use prefix for relative paths since exact result depends on cwd
		wantExact  string // Use exact match for absolute paths
		wantErr    bool
	}{
		{
			name:      "empty path",
			input:     "",
			wantExact: "",
			wantErr:   false,
		},
		{
			name:      "tilde only",
			input:     "~",
			wantExact: home,
			wantErr:   false,
		},
		{
			name:      "tilde with path",
			input:     "~/.config/nvim",
			wantExact: filepath.Join(home, ".config", "nvim"),
			wantErr:   false,
		},
		{
			name:       "relative path",
			input:      "relative/path",
			wantPrefix: cwd,
			wantErr:    false,
		},
		{
			name:       "relative path with dot",
			input:      "./relative/path",
			wantPrefix: cwd,
			wantErr:    false,
		},
		{
			name:      "absolute path",
			input:     "/absolute/path",
			wantExact: "/absolute/path",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := expandTargetPath(tt.input)

			if (err != nil) != tt.wantErr {
				t.Errorf("expandTargetPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if tt.wantExact != "" {
				if got != tt.wantExact {
					t.Errorf("expandTargetPath() = %v, want %v", got, tt.wantExact)
				}
			} else if tt.wantPrefix != "" {
				if !strings.HasPrefix(got, tt.wantPrefix) {
					t.Errorf("expandTargetPath() = %v, want prefix %v", got, tt.wantPrefix)
				}
				// Verify it's absolute
				if !filepath.IsAbs(got) {
					t.Errorf("expandTargetPath() = %v, want absolute path", got)
				}
			}
		})
	}
}

func TestFindNearestExistingParent(t *testing.T) {
	t.Parallel()

	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()
	existingDir := filepath.Join(tmpDir, "existing")
	err := os.Mkdir(existingDir, 0o750)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		want    string
		wantErr bool
	}{
		{
			name:    "existing directory",
			path:    existingDir,
			want:    existingDir,
			wantErr: false,
		},
		{
			name:    "non-existing child",
			path:    filepath.Join(existingDir, "nonexistent"),
			want:    existingDir,
			wantErr: false,
		},
		{
			name:    "deeply nested non-existing",
			path:    filepath.Join(existingDir, "a", "b", "c", "d"),
			want:    existingDir,
			wantErr: false,
		},
		{
			name:    "non-existing under tmpDir",
			path:    filepath.Join(tmpDir, "nonexistent", "deep", "path"),
			want:    tmpDir,
			wantErr: false,
		},
		{
			name:    "root directory",
			path:    string(filepath.Separator),
			want:    string(filepath.Separator),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := findNearestExistingParent(tt.path)

			if (err != nil) != tt.wantErr {
				t.Errorf("findNearestExistingParent() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if got != tt.want {
				t.Errorf("findNearestExistingParent() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResolvePickerStartDirectory(t *testing.T) {
	t.Parallel()

	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()
	existingDir := filepath.Join(tmpDir, "existing")
	err := os.Mkdir(existingDir, 0o750)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	tests := []struct {
		name      string
		target    string
		currentOS string
		want      string
		wantErr   bool
	}{
		{
			name:      "existing absolute path",
			target:    existingDir,
			currentOS: "linux",
			want:      existingDir,
			wantErr:   false,
		},
		{
			name:      "non-existing path falls back to parent",
			target:    filepath.Join(existingDir, "nonexistent", "child"),
			currentOS: "linux",
			want:      existingDir,
			wantErr:   false,
		},
		{
			name:      "empty target uses home",
			target:    "",
			currentOS: "linux",
			want:      home,
			wantErr:   false,
		},
		{
			name:      "tilde expands to home",
			target:    "~",
			currentOS: "linux",
			want:      home,
			wantErr:   false,
		},
		{
			name:      "tilde with path",
			target:    "~/.config",
			currentOS: "linux",
			want:      "", // We'll check if it exists or falls back
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := resolvePickerStartDirectory(tt.target, tt.currentOS)

			if (err != nil) != tt.wantErr {
				t.Errorf("resolvePickerStartDirectory() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			// For the tilde with path test, we just verify it returned something valid
			if tt.want == "" {
				// Verify result is a directory that exists
				if info, statErr := os.Stat(got); statErr != nil || !info.IsDir() {
					t.Errorf("resolvePickerStartDirectory() returned non-existent or non-directory: %v", got)
				}
				return
			}

			if got != tt.want {
				t.Errorf("resolvePickerStartDirectory() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConvertToRelativePaths(t *testing.T) {
	t.Parallel()

	// Create a temporary directory structure for testing
	tmpDir := t.TempDir()
	targetDir := filepath.Join(tmpDir, "target")
	err := os.Mkdir(targetDir, 0o750)
	if err != nil {
		t.Fatalf("Failed to create test directory: %v", err)
	}

	// Create a file in target
	testFile := filepath.Join(targetDir, "test.txt")
	err = os.WriteFile(testFile, []byte("test"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	// Create a nested directory
	nestedDir := filepath.Join(targetDir, "nested")
	err = os.Mkdir(nestedDir, 0o750)
	if err != nil {
		t.Fatalf("Failed to create nested directory: %v", err)
	}

	nestedFile := filepath.Join(nestedDir, "nested.txt")
	err = os.WriteFile(nestedFile, []byte("nested"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create nested file: %v", err)
	}

	// Create an outside directory
	outsideDir := filepath.Join(tmpDir, "outside")
	err = os.Mkdir(outsideDir, 0o750)
	if err != nil {
		t.Fatalf("Failed to create outside directory: %v", err)
	}

	outsideFile := filepath.Join(outsideDir, "outside.txt")
	err = os.WriteFile(outsideFile, []byte("outside"), 0o600)
	if err != nil {
		t.Fatalf("Failed to create outside file: %v", err)
	}

	tests := []struct {
		name         string
		absPaths     []string
		targetDir    string
		wantPaths    []string
		wantErrCount int // Number of errors expected
	}{
		{
			name:         "single file in target",
			absPaths:     []string{testFile},
			targetDir:    targetDir,
			wantPaths:    []string{"test.txt"},
			wantErrCount: 0,
		},
		{
			name:         "nested file in target",
			absPaths:     []string{nestedFile},
			targetDir:    targetDir,
			wantPaths:    []string{filepath.Join("nested", "nested.txt")},
			wantErrCount: 0,
		},
		{
			name:         "multiple files in target",
			absPaths:     []string{testFile, nestedFile},
			targetDir:    targetDir,
			wantPaths:    []string{"test.txt", filepath.Join("nested", "nested.txt")},
			wantErrCount: 0,
		},
		{
			name:         "target directory itself",
			absPaths:     []string{targetDir},
			targetDir:    targetDir,
			wantPaths:    []string{"."},
			wantErrCount: 0,
		},
		{
			name:         "file outside target",
			absPaths:     []string{outsideFile},
			targetDir:    targetDir,
			wantPaths:    []string{""},
			wantErrCount: 1,
		},
		{
			name:         "mixed inside and outside",
			absPaths:     []string{testFile, outsideFile, nestedFile},
			targetDir:    targetDir,
			wantPaths:    []string{"test.txt", "", filepath.Join("nested", "nested.txt")},
			wantErrCount: 1,
		},
		{
			name:         "empty input",
			absPaths:     []string{},
			targetDir:    targetDir,
			wantPaths:    []string{},
			wantErrCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			gotPaths, gotErrs := convertToRelativePaths(tt.absPaths, tt.targetDir)

			// Check number of errors
			errCount := 0
			for _, err := range gotErrs {
				if err != nil {
					errCount++
				}
			}

			if errCount != tt.wantErrCount {
				t.Errorf("convertToRelativePaths() error count = %v, want %v", errCount, tt.wantErrCount)
				t.Logf("Errors: %v", gotErrs)
			}

			// Check paths
			if len(gotPaths) != len(tt.wantPaths) {
				t.Errorf("convertToRelativePaths() returned %d paths, want %d", len(gotPaths), len(tt.wantPaths))
				return
			}

			for i := range gotPaths {
				if gotPaths[i] != tt.wantPaths[i] {
					t.Errorf("convertToRelativePaths()[%d] = %v, want %v", i, gotPaths[i], tt.wantPaths[i])
				}
			}
		})
	}
}
