package manager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
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

// TestRestoreFolder_NoMerge_Fails tests that restore fails when NoMerge is enabled
// and target directory has content (without ForceDelete flag).
func TestRestoreFolder_NoMerge_Fails(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backup directory
	backupDir := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(backupDir, DirPerms); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "config.txt"), []byte("backup"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create target directory with content
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, DirPerms); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetDir, "local.txt"), []byte("local config"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.NoMerge = true
	mgr.ForceDelete = false

	entry := config.Entry{Name: "test"}

	// Act: Should fail because NoMerge is enabled and target has content
	err := mgr.RestoreFolder(entry, backupDir, targetDir)

	// Assert: Error should be returned
	if err == nil {
		t.Fatal("RestoreFolder() should have failed with NoMerge=true and non-empty target")
	}

	// Assert: Error message should be helpful
	errMsg := err.Error()
	if !contains(errMsg, "target exists with") {
		t.Errorf("Error message should mention target exists, got: %v", errMsg)
	}
	if !contains(errMsg, "--force") || !contains(errMsg, "merge") {
		t.Errorf("Error message should suggest --force or merge mode, got: %v", errMsg)
	}

	// Assert: Target directory still has content (nothing was deleted)
	if !pathExists(filepath.Join(targetDir, "local.txt")) {
		t.Error("Target file was deleted, should be preserved when operation fails")
	}

	// Assert: Symlink was not created
	if isSymlink(targetDir) {
		t.Error("Target should not be a symlink, operation should have failed before symlinking")
	}
}

// TestRestoreFolder_NoMerge_ForceDelete_Succeeds tests that restore succeeds
// when both NoMerge and ForceDelete are enabled, reverting to old destructive behavior.
func TestRestoreFolder_NoMerge_ForceDelete_Succeeds(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backup directory
	backupDir := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(backupDir, DirPerms); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "config.txt"), []byte("backup"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create target directory with content
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, DirPerms); err != nil {
		t.Fatal(err)
	}
	targetFile := filepath.Join(targetDir, "local.txt")
	if err := os.WriteFile(targetFile, []byte("local config"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.NoMerge = true
	mgr.ForceDelete = true

	entry := config.Entry{Name: "test"}

	// Act: Should succeed because ForceDelete is enabled
	err := mgr.RestoreFolder(entry, backupDir, targetDir)

	// Assert: No error
	if err != nil {
		t.Fatalf("RestoreFolder() failed: %v", err)
	}

	// Assert: Target is now a symlink
	if !isSymlink(targetDir) {
		t.Error("Target should be a symlink")
	}

	// Assert: Symlink points to backup
	link, err := os.Readlink(targetDir)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	if link != backupDir {
		t.Errorf("Symlink target = %q, want %q", link, backupDir)
	}

	// Assert: Target file was deleted (not merged)
	// Since targetDir is now a symlink, we can't check for the old file
	// But we can verify the backup directory doesn't have the target file
	if pathExists(filepath.Join(backupDir, "local.txt")) {
		t.Error("Target file should NOT have been merged into backup with ForceDelete")
	}
}

// TestMergeFolder_SymlinkInTarget tests that symlinks in the target folder
// are moved to backup (current behavior: symlinks are preserved as-is).
// NOTE: Future enhancement could resolve symlinks and copy their content.
func TestMergeFolder_SymlinkInTarget(t *testing.T) {
	t.Parallel()

	// Setup: Create target and backup directories
	tmpRoot := t.TempDir()
	targetDir := filepath.Join(tmpRoot, "target")
	backupDir := filepath.Join(tmpRoot, "backup")

	if err := os.MkdirAll(targetDir, DirPerms); err != nil {
		t.Fatalf("Failed to create target dir: %v", err)
	}
	if err := os.MkdirAll(backupDir, DirPerms); err != nil {
		t.Fatalf("Failed to create backup dir: %v", err)
	}

	// Create a real file that we'll symlink to
	realFileDir := filepath.Join(tmpRoot, "elsewhere")
	if err := os.MkdirAll(realFileDir, DirPerms); err != nil {
		t.Fatalf("Failed to create real file dir: %v", err)
	}
	realFile := filepath.Join(realFileDir, "realfile.txt")
	if err := os.WriteFile(realFile, []byte("real content"), 0600); err != nil {
		t.Fatalf("Failed to create real file: %v", err)
	}

	// Create a symlink in target pointing to the real file
	symlinkInTarget := filepath.Join(targetDir, "linked.txt")
	if err := os.Symlink(realFile, symlinkInTarget); err != nil {
		t.Fatalf("Failed to create symlink: %v", err)
	}

	// Create a regular file in target as well
	regularFile := filepath.Join(targetDir, "regular.txt")
	if err := os.WriteFile(regularFile, []byte("regular content"), 0600); err != nil {
		t.Fatalf("Failed to create regular file: %v", err)
	}

	summary := NewMergeSummary("test-app")

	// Act: Merge the folder
	err := MergeFolder(backupDir, targetDir, false, summary)

	// Assert: No error
	if err != nil {
		t.Fatalf("MergeFolder() error = %v, want nil", err)
	}

	// Assert: 2 files were merged
	if len(summary.MergedFiles) != 2 {
		t.Errorf("MergedFiles count = %d, want 2", len(summary.MergedFiles))
	}

	// Assert: Regular file exists in backup
	regularBackup := filepath.Join(backupDir, "regular.txt")
	if !pathExists(regularBackup) {
		t.Error("Regular file not merged to backup")
	}

	// Assert: Symlink was moved to backup
	linkedBackup := filepath.Join(backupDir, "linked.txt")
	if !pathExists(linkedBackup) {
		t.Error("Symlink not merged to backup")
	}

	// Current behavior: Symlink is preserved as-is
	// (Future enhancement: could resolve and copy content instead)
	if !isSymlink(linkedBackup) {
		t.Error("Backup file should be a symlink (current behavior)")
	}

	// Assert: Symlink still points to original location
	linkTarget, err := os.Readlink(linkedBackup)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	if linkTarget != realFile {
		t.Errorf("Symlink target = %q, want %q", linkTarget, realFile)
	}
}

// TestRestoreFolder_NoMerge_FailsEvenIfEmpty tests that NoMerge fails
// even if target directory exists but is empty (strict mode).
func TestRestoreFolder_NoMerge_FailsEvenIfEmpty(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()

	// Create backup directory
	backupDir := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(backupDir, DirPerms); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "config.txt"), []byte("backup"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create empty target directory
	targetDir := filepath.Join(tmpDir, "target")
	if err := os.MkdirAll(targetDir, DirPerms); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{BackupRoot: tmpDir}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.NoMerge = true

	entry := config.Entry{Name: "test"}

	// Act: Should fail because NoMerge is strict (target exists)
	err := mgr.RestoreFolder(entry, backupDir, targetDir)

	// Assert: Error should be returned
	if err == nil {
		t.Fatal("RestoreFolder() should fail with NoMerge even if target is empty")
	}

	// Assert: Error message shows 0 files
	errMsg := err.Error()
	if !contains(errMsg, "0 file(s)") {
		t.Errorf("Error message should show 0 files for empty directory, got: %v", errMsg)
	}
}

// TestMergeFolder_DuplicateConflicts tests handling of multiple conflicts
// with same filename on the same day.
// NOTE: Current behavior overwrites the first conflict file (no unique counter yet).
// Future enhancement: Add counter suffix like _target_20260204_1.json
func TestMergeFolder_DuplicateConflicts(t *testing.T) {
	t.Parallel()

	targetDir := t.TempDir()
	backupDir := t.TempDir()

	// Create backup file
	backupFile := filepath.Join(backupDir, "config.json")
	if err := os.WriteFile(backupFile, []byte("backup version 1"), 0600); err != nil {
		t.Fatalf("Failed to create backup file: %v", err)
	}

	// First merge: Create target file and merge
	targetFile := filepath.Join(targetDir, "config.json")
	if err := os.WriteFile(targetFile, []byte("target version 1"), 0600); err != nil {
		t.Fatalf("Failed to create target file: %v", err)
	}

	summary1 := NewMergeSummary("test-app")
	err := MergeFolder(backupDir, targetDir, false, summary1)
	if err != nil {
		t.Fatalf("First MergeFolder() error = %v", err)
	}

	// Assert: First conflict file was created
	pattern1 := filepath.Join(backupDir, "config_target_*.json")
	conflicts1, _ := filepath.Glob(pattern1)
	if len(conflicts1) != 1 {
		t.Fatalf("First merge should create 1 conflict file, got %d", len(conflicts1))
	}

	// Read first conflict content
	firstConflictContent, _ := os.ReadFile(conflicts1[0]) //nolint:gosec // test file

	// Second merge: Recreate target directory with same file
	if err := os.MkdirAll(targetDir, DirPerms); err != nil {
		t.Fatalf("Failed to recreate target dir: %v", err)
	}
	if err := os.WriteFile(targetFile, []byte("target version 2"), 0600); err != nil {
		t.Fatalf("Failed to recreate target file: %v", err)
	}

	summary2 := NewMergeSummary("test-app")
	err = MergeFolder(backupDir, targetDir, false, summary2)
	if err != nil {
		t.Fatalf("Second MergeFolder() error = %v", err)
	}

	// Assert: Still only 1 conflict file (current behavior: overwrites)
	conflicts2, _ := filepath.Glob(pattern1)
	if len(conflicts2) != 1 {
		t.Fatalf("Second merge should still have 1 conflict file (overwrites), got %d", len(conflicts2))
	}

	// Assert: Conflict file has second merge content (overwrote first)
	secondConflictContent, _ := os.ReadFile(conflicts2[0]) //nolint:gosec // test file
	if string(secondConflictContent) == string(firstConflictContent) {
		t.Error("Second merge should have overwritten first conflict file")
	}
	if string(secondConflictContent) != "target version 2" {
		t.Errorf("Conflict file content = %q, want %q", string(secondConflictContent), "target version 2")
	}
}

// TestMergeFolder_EmptyTargetDir tests that an empty target directory
// doesn't cause merge errors and is cleanly removed.
func TestMergeFolder_EmptyTargetDir(t *testing.T) {
	t.Parallel()

	targetDir := t.TempDir()
	backupDir := t.TempDir()

	// Target is empty (nothing to merge)
	summary := NewMergeSummary("test-app")

	// Act: Merge empty folder
	err := MergeFolder(backupDir, targetDir, false, summary)

	// Assert: No error
	if err != nil {
		t.Fatalf("MergeFolder() error = %v, want nil", err)
	}

	// Assert: No operations recorded
	if summary.HasOperations() {
		t.Error("Summary should have no operations for empty target")
	}
}
