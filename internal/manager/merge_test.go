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

func TestMergeFolder_Recursive(t *testing.T) {
	t.Parallel()

	// Setup: Create nested target directory structure
	targetDir := t.TempDir()
	backupDir := t.TempDir()

	// Create nested files
	files := []struct {
		path    string
		content string
	}{
		{"file1.txt", "content1"},
		{"subdir/file2.txt", "content2"},
		{"subdir/nested/file3.txt", "content3"},
	}

	for _, f := range files {
		fullPath := filepath.Join(targetDir, f.path)
		if err := os.MkdirAll(filepath.Dir(fullPath), DirPerms); err != nil {
			t.Fatalf("Failed to create directory for %q: %v", f.path, err)
		}
		if err := os.WriteFile(fullPath, []byte(f.content), 0600); err != nil {
			t.Fatalf("Failed to create file %q: %v", f.path, err)
		}
	}

	summary := NewMergeSummary("test-app")

	// Act: Merge the entire folder
	err := MergeFolder(backupDir, targetDir, false, summary)

	// Assert: No error
	if err != nil {
		t.Fatalf("MergeFolder() error = %v, want nil", err)
	}

	// Assert: All files were merged
	if len(summary.MergedFiles) != 3 {
		t.Errorf("MergedFiles count = %d, want 3", len(summary.MergedFiles))
	}

	// Assert: No conflicts
	if len(summary.ConflictFiles) != 0 {
		t.Errorf("ConflictFiles count = %d, want 0", len(summary.ConflictFiles))
	}

	// Assert: All files exist in backup with correct content
	for _, f := range files {
		backupFile := filepath.Join(backupDir, f.path)
		if !pathExists(backupFile) {
			t.Errorf("Backup file not created at %q", backupFile)
			continue
		}

		content, err := os.ReadFile(backupFile) //nolint:gosec // test file
		if err != nil {
			t.Errorf("Failed to read backup file %q: %v", backupFile, err)
			continue
		}
		if string(content) != f.content {
			t.Errorf("Backup file %q content = %q, want %q", f.path, string(content), f.content)
		}

		// Assert: Target files no longer exist
		targetFile := filepath.Join(targetDir, f.path)
		if pathExists(targetFile) {
			t.Errorf("Target file still exists at %q, should have been moved", targetFile)
		}
	}
}

func TestMergeFolder_WithConflicts(t *testing.T) {
	t.Parallel()

	// Setup: Create target and backup directories with some overlapping files
	targetDir := t.TempDir()
	backupDir := t.TempDir()

	// Files that exist only in target (should be merged)
	uniqueFiles := []struct {
		path    string
		content string
	}{
		{"unique1.txt", "unique content 1"},
		{"subdir/unique2.txt", "unique content 2"},
	}

	// Files that exist in both (should create conflicts)
	conflictFiles := []struct {
		path          string
		targetContent string
		backupContent string
	}{
		{"conflict.json", "target version", "backup version"},
		{"subdir/settings.conf", "target settings", "backup settings"},
	}

	// Create unique files in target
	for _, f := range uniqueFiles {
		fullPath := filepath.Join(targetDir, f.path)
		if err := os.MkdirAll(filepath.Dir(fullPath), DirPerms); err != nil {
			t.Fatalf("Failed to create directory for %q: %v", f.path, err)
		}
		if err := os.WriteFile(fullPath, []byte(f.content), 0600); err != nil {
			t.Fatalf("Failed to create file %q: %v", f.path, err)
		}
	}

	// Create conflict files in both target and backup
	for _, f := range conflictFiles {
		// Target
		targetPath := filepath.Join(targetDir, f.path)
		if err := os.MkdirAll(filepath.Dir(targetPath), DirPerms); err != nil {
			t.Fatalf("Failed to create directory for %q: %v", f.path, err)
		}
		if err := os.WriteFile(targetPath, []byte(f.targetContent), 0600); err != nil {
			t.Fatalf("Failed to create target file %q: %v", f.path, err)
		}

		// Backup
		backupPath := filepath.Join(backupDir, f.path)
		if err := os.MkdirAll(filepath.Dir(backupPath), DirPerms); err != nil {
			t.Fatalf("Failed to create backup directory for %q: %v", f.path, err)
		}
		if err := os.WriteFile(backupPath, []byte(f.backupContent), 0600); err != nil {
			t.Fatalf("Failed to create backup file %q: %v", f.path, err)
		}
	}

	summary := NewMergeSummary("test-app")

	// Act: Merge the entire folder
	err := MergeFolder(backupDir, targetDir, false, summary)

	// Assert: No error
	if err != nil {
		t.Fatalf("MergeFolder() error = %v, want nil", err)
	}

	// Assert: Unique files were merged
	if len(summary.MergedFiles) != 2 {
		t.Errorf("MergedFiles count = %d, want 2", len(summary.MergedFiles))
	}

	// Assert: Conflicts were detected
	if len(summary.ConflictFiles) != 2 {
		t.Errorf("ConflictFiles count = %d, want 2", len(summary.ConflictFiles))
	}

	// Assert: Unique files exist in backup with correct content
	for _, f := range uniqueFiles {
		backupFile := filepath.Join(backupDir, f.path)
		if !pathExists(backupFile) {
			t.Errorf("Backup file not created at %q", backupFile)
			continue
		}

		content, err := os.ReadFile(backupFile) //nolint:gosec // test file
		if err != nil {
			t.Errorf("Failed to read backup file %q: %v", backupFile, err)
			continue
		}
		if string(content) != f.content {
			t.Errorf("Backup file %q content = %q, want %q", f.path, string(content), f.content)
		}
	}

	// Assert: Conflict files have backup version preserved and target renamed
	for _, f := range conflictFiles {
		backupFile := filepath.Join(backupDir, f.path)

		// Original backup should still have backup content
		content, err := os.ReadFile(backupFile) //nolint:gosec // test file
		if err != nil {
			t.Errorf("Failed to read backup file %q: %v", backupFile, err)
			continue
		}
		if string(content) != f.backupContent {
			t.Errorf("Backup file %q content = %q, want %q", f.path, string(content), f.backupContent)
		}

		// Conflict file should exist with target content
		filename := filepath.Base(f.path)
		dir := filepath.Dir(f.path)
		pattern := filepath.Join(backupDir, dir, strings.TrimSuffix(filename, filepath.Ext(filename))+"_target_*"+filepath.Ext(filename))
		conflictFiles, err := filepath.Glob(pattern)
		if err != nil {
			t.Errorf("Failed to glob conflict files for %q: %v", f.path, err)
			continue
		}
		if len(conflictFiles) != 1 {
			t.Errorf("Conflict files for %q count = %d, want 1", f.path, len(conflictFiles))
			continue
		}

		conflictContent, err := os.ReadFile(conflictFiles[0]) //nolint:gosec // test file
		if err != nil {
			t.Errorf("Failed to read conflict file %q: %v", conflictFiles[0], err)
			continue
		}
		if string(conflictContent) != f.targetContent {
			t.Errorf("Conflict file %q content = %q, want %q", f.path, string(conflictContent), f.targetContent)
		}
	}
}

func TestRemoveEmptyDirs(t *testing.T) {
	t.Parallel()

	// Setup: Create directory structure
	rootDir := t.TempDir()

	// Create nested empty directories: a/b/c/d
	deepDir := filepath.Join(rootDir, "a", "b", "c", "d")
	if err := os.MkdirAll(deepDir, DirPerms); err != nil {
		t.Fatalf("Failed to create deep directory: %v", err)
	}

	// Create directory with a file: e/f/file.txt
	dirWithFile := filepath.Join(rootDir, "e", "f")
	if err := os.MkdirAll(dirWithFile, DirPerms); err != nil {
		t.Fatalf("Failed to create directory with file: %v", err)
	}
	fileInDir := filepath.Join(dirWithFile, "file.txt")
	if err := os.WriteFile(fileInDir, []byte("content"), 0600); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Act: Remove empty directories
	err := removeEmptyDirs(rootDir)

	// Assert: No error
	if err != nil {
		t.Fatalf("removeEmptyDirs() error = %v, want nil", err)
	}

	// Assert: Empty directories are removed (a/b/c/d should all be gone)
	if pathExists(filepath.Join(rootDir, "a")) {
		t.Errorf("Empty directory 'a' still exists, should be removed")
	}

	// Assert: Directory with file is preserved
	if !pathExists(dirWithFile) {
		t.Errorf("Directory with file 'e/f' was removed, should be preserved")
	}
	if !pathExists(fileInDir) {
		t.Errorf("File 'e/f/file.txt' was removed, should be preserved")
	}

	// Assert: Root directory still exists (should never be removed)
	if !pathExists(rootDir) {
		t.Errorf("Root directory was removed, should be preserved")
	}
}
