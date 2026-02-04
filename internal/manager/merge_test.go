package manager

import (
	"os"
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

func TestMergeFile_NoConflict(t *testing.T) {
	t.Parallel()

	// Setup: Create target file and backup directory
	targetDir := t.TempDir()
	backupDir := t.TempDir()

	targetFile := filepath.Join(targetDir, "unique.txt")
	if err := os.WriteFile(targetFile, []byte("target content"), 0600); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	summary := NewMergeSummary("test-app")

	// Act: Merge the file
	err := mergeFile(targetFile, backupDir, "unique.txt", false, summary)

	// Assert: No error
	if err != nil {
		t.Fatalf("mergeFile() error = %v, want nil", err)
	}

	// Assert: Target file was moved to backup
	backupFile := filepath.Join(backupDir, "unique.txt")
	if !pathExists(backupFile) {
		t.Errorf("Backup file not created at %q", backupFile)
	}

	// Assert: Target file no longer exists
	if pathExists(targetFile) {
		t.Errorf("Target file still exists at %q, should have been moved", targetFile)
	}

	// Assert: Content is correct
	content, err := os.ReadFile(backupFile) //nolint:gosec // test file
	if err != nil {
		t.Fatalf("Failed to read backup file: %v", err)
	}
	if string(content) != "target content" {
		t.Errorf("Backup file content = %q, want %q", string(content), "target content")
	}

	// Assert: Summary shows merge (not conflict)
	if len(summary.MergedFiles) != 1 {
		t.Errorf("MergedFiles count = %d, want 1", len(summary.MergedFiles))
	}
	if len(summary.ConflictFiles) != 0 {
		t.Errorf("ConflictFiles count = %d, want 0", len(summary.ConflictFiles))
	}
}

func TestMergeFile_WithConflict(t *testing.T) {
	t.Parallel()

	// Setup: Create both target and backup files
	targetDir := t.TempDir()
	backupDir := t.TempDir()

	targetFile := filepath.Join(targetDir, "config.json")
	backupFile := filepath.Join(backupDir, "config.json")

	if err := os.WriteFile(targetFile, []byte("target version"), 0600); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}
	if err := os.WriteFile(backupFile, []byte("backup version"), 0600); err != nil {
		t.Fatalf("Failed to create backup file: %v", err)
	}

	summary := NewMergeSummary("test-app")

	// Act: Merge the file
	err := mergeFile(targetFile, backupDir, "config.json", false, summary)

	// Assert: No error
	if err != nil {
		t.Fatalf("mergeFile() error = %v, want nil", err)
	}

	// Assert: Backup file still exists with original content
	content, err := os.ReadFile(backupFile) //nolint:gosec // test file
	if err != nil {
		t.Fatalf("Failed to read backup file: %v", err)
	}
	if string(content) != "backup version" {
		t.Errorf("Backup file content = %q, want %q", string(content), "backup version")
	}

	// Assert: Conflict file was created with renamed name
	conflictFiles, err := filepath.Glob(filepath.Join(backupDir, "config_target_*.json"))
	if err != nil {
		t.Fatalf("Failed to glob conflict files: %v", err)
	}
	if len(conflictFiles) != 1 {
		t.Fatalf("Conflict files count = %d, want 1", len(conflictFiles))
	}

	// Assert: Conflict file has target content
	conflictContent, err := os.ReadFile(conflictFiles[0]) //nolint:gosec // test file
	if err != nil {
		t.Fatalf("Failed to read conflict file: %v", err)
	}
	if string(conflictContent) != "target version" {
		t.Errorf("Conflict file content = %q, want %q", string(conflictContent), "target version")
	}

	// Assert: Target file no longer exists
	if pathExists(targetFile) {
		t.Errorf("Target file still exists at %q, should have been moved", targetFile)
	}

	// Assert: Summary shows conflict (not merge)
	if len(summary.MergedFiles) != 0 {
		t.Errorf("MergedFiles count = %d, want 0", len(summary.MergedFiles))
	}
	if len(summary.ConflictFiles) != 1 {
		t.Errorf("ConflictFiles count = %d, want 1", len(summary.ConflictFiles))
	}
	if summary.ConflictFiles[0].OriginalName != "config.json" {
		t.Errorf("ConflictFiles[0].OriginalName = %q, want %q",
			summary.ConflictFiles[0].OriginalName, "config.json")
	}
}
