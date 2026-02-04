package manager

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestMergeSummary_Add(t *testing.T) {
	t.Parallel()

	summary := NewMergeSummary("test-app")

	summary.AddMerged("file1.txt")
	summary.AddMerged("file2.txt")
	summary.AddConflict("config.json", "config_target_20260204.json")

	if len(summary.MergedFiles) != 2 {
		t.Errorf("MergedFiles count = %d, want 2", len(summary.MergedFiles))
	}

	if len(summary.ConflictFiles) != 1 {
		t.Errorf("ConflictFiles count = %d, want 1", len(summary.ConflictFiles))
	}

	if summary.ConflictFiles[0].OriginalName != "config.json" {
		t.Errorf("ConflictFiles[0].OriginalName = %q, want %q",
			summary.ConflictFiles[0].OriginalName, "config.json")
	}
}

func TestMergeSummary_AddFailed(t *testing.T) {
	t.Parallel()

	summary := NewMergeSummary("test-app")

	summary.AddFailed("broken.txt", "permission denied")
	summary.AddFailed("invalid.json", "malformed JSON")

	if len(summary.FailedFiles) != 2 {
		t.Errorf("FailedFiles count = %d, want 2", len(summary.FailedFiles))
	}

	if summary.FailedFiles[0].FileName != "broken.txt" {
		t.Errorf("FailedFiles[0].FileName = %q, want %q",
			summary.FailedFiles[0].FileName, "broken.txt")
	}

	if summary.FailedFiles[0].Error != "permission denied" {
		t.Errorf("FailedFiles[0].Error = %q, want %q",
			summary.FailedFiles[0].Error, "permission denied")
	}

	if summary.FailedFiles[1].FileName != "invalid.json" {
		t.Errorf("FailedFiles[1].FileName = %q, want %q",
			summary.FailedFiles[1].FileName, "invalid.json")
	}

	if summary.FailedFiles[1].Error != "malformed JSON" {
		t.Errorf("FailedFiles[1].Error = %q, want %q",
			summary.FailedFiles[1].Error, "malformed JSON")
	}
}

func TestMergeSummary_HasOperations(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		setup    func(*MergeSummary)
		expected bool
	}{
		{
			name:     "empty summary",
			setup:    func(_ *MergeSummary) {},
			expected: false,
		},
		{
			name: "only merged files",
			setup: func(s *MergeSummary) {
				s.AddMerged("file1.txt")
			},
			expected: true,
		},
		{
			name: "only conflicts",
			setup: func(s *MergeSummary) {
				s.AddConflict("config.json", "config_backup.json")
			},
			expected: true,
		},
		{
			name: "only failed files",
			setup: func(s *MergeSummary) {
				s.AddFailed("broken.txt", "error")
			},
			expected: true,
		},
		{
			name: "mixed operations",
			setup: func(s *MergeSummary) {
				s.AddMerged("file1.txt")
				s.AddConflict("config.json", "config_backup.json")
				s.AddFailed("broken.txt", "error")
			},
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			summary := NewMergeSummary("test-app")
			tt.setup(summary)

			got := summary.HasOperations()
			if got != tt.expected {
				t.Errorf("HasOperations() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGenerateConflictName(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
		date     string
		want     string
	}{
		{
			name:     "simple extension",
			filename: "config.json",
			date:     "20260204",
			want:     "config_target_20260204.json",
		},
		{
			name:     "double extension",
			filename: "settings.conf.yaml",
			date:     "20260204",
			want:     "settings.conf_target_20260204.yaml",
		},
		{
			name:     "no extension",
			filename: "README",
			date:     "20260204",
			want:     "README_target_20260204",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateConflictName(tt.filename, tt.date)
			if got != tt.want {
				t.Errorf("generateConflictName(%q, %q) = %q, want %q",
					tt.filename, tt.date, got, tt.want)
			}
		})
	}
}

func TestGenerateConflictNameWithDate(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		filename string
	}{
		{
			name:     "simple extension",
			filename: "config.json",
		},
		{
			name:     "double extension",
			filename: "settings.conf.yaml",
		},
		{
			name:     "no extension",
			filename: "README",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := generateConflictNameWithDate(tt.filename)

			// Verify it contains "_target_" and has the proper structure
			if !contains(got, "_target_") {
				t.Errorf("generateConflictNameWithDate(%q) = %q, should contain '_target_'",
					tt.filename, got)
			}

			// Verify it starts with the base name
			ext := filepath.Ext(tt.filename)
			nameWithoutExt := strings.TrimSuffix(tt.filename, ext)
			if !strings.HasPrefix(got, nameWithoutExt) {
				t.Errorf("generateConflictNameWithDate(%q) = %q, should start with %q",
					tt.filename, got, nameWithoutExt)
			}

			// Verify it ends with the extension (if any)
			if ext != "" && !strings.HasSuffix(got, ext) {
				t.Errorf("generateConflictNameWithDate(%q) = %q, should end with %q",
					tt.filename, got, ext)
			}
		})
	}
}

// Helper function for string containment check
func contains(s, substr string) bool {
	return strings.Contains(s, substr)
}
