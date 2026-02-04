package manager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

// setupNvimTestEnvironment creates a test environment with backup and target directories
func setupNvimTestEnvironment(t *testing.T, tmpRoot string) (nvimBackup, targetNvim string,
	trackedFiles, localFiles map[string]string,
	conflictFiles map[string]struct{ backupContent, targetContent string }) {
	t.Helper()

	// Setup: Create dotfiles repository structure
	dotfilesRepo := filepath.Join(tmpRoot, "dotfiles")
	nvimBackup = filepath.Join(dotfilesRepo, "nvim")
	if err := os.MkdirAll(nvimBackup, DirPerms); err != nil {
		t.Fatal(err)
	}

	// Create tracked nvim config files (in dotfiles repo)
	trackedFiles = map[string]string{
		"init.lua":                  "-- Tracked init.lua\nrequire('core')",
		"lua/core/options.lua":      "-- Core options from repo",
		"lua/plugins/telescope.lua": "-- Telescope config from repo",
		"lua/plugins/lsp.lua":       "-- LSP config from repo",
	}

	for relPath, content := range trackedFiles {
		fullPath := filepath.Join(nvimBackup, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), DirPerms); err != nil {
			t.Fatalf("Failed to create directory for %q: %v", relPath, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0600); err != nil {
			t.Fatalf("Failed to create tracked file %q: %v", relPath, err)
		}
	}

	// Setup: Create target nvim config directory (simulating existing local config)
	targetNvim = filepath.Join(tmpRoot, "config", "nvim")
	if err := os.MkdirAll(targetNvim, DirPerms); err != nil {
		t.Fatal(err)
	}

	// Create local machine-specific files (will be merged)
	localFiles = map[string]string{
		"lua/local_plugin.lua":    "-- Machine-specific plugin",
		"lua/work_settings.lua":   "-- Work environment settings",
		"after/plugin/custom.lua": "-- Custom after plugin",
		"spell/en.utf-8.add":      "-- Custom spell words",
	}

	for relPath, content := range localFiles {
		fullPath := filepath.Join(targetNvim, relPath)
		if err := os.MkdirAll(filepath.Dir(fullPath), DirPerms); err != nil {
			t.Fatalf("Failed to create directory for %q: %v", relPath, err)
		}
		if err := os.WriteFile(fullPath, []byte(content), 0600); err != nil {
			t.Fatalf("Failed to create local file %q: %v", relPath, err)
		}
	}

	// Create conflicting files (exist in both backup and target with different content)
	conflictFiles = map[string]struct{ backupContent, targetContent string }{
		"lua/plugins/telescope.lua": {
			backupContent: "-- Telescope config from repo",
			targetContent: "-- Local telescope tweaks",
		},
	}

	for relPath, contents := range conflictFiles {
		// Target file (will be renamed during merge)
		targetPath := filepath.Join(targetNvim, relPath)
		if err := os.MkdirAll(filepath.Dir(targetPath), DirPerms); err != nil {
			t.Fatalf("Failed to create directory for conflict %q: %v", relPath, err)
		}
		if err := os.WriteFile(targetPath, []byte(contents.targetContent), 0600); err != nil {
			t.Fatalf("Failed to create conflict target file %q: %v", relPath, err)
		}
		// Note: backup file already created in trackedFiles above
	}

	return nvimBackup, targetNvim, trackedFiles, localFiles, conflictFiles
}

// verifyMergeResults checks that merge operation completed correctly
func verifyMergeResults(t *testing.T, nvimBackup, targetNvim string,
	trackedFiles, localFiles map[string]string,
	conflictFiles map[string]struct{ backupContent, targetContent string }) {
	t.Helper()

	// Assert: Target is now a symlink pointing to backup
	if !isSymlink(targetNvim) {
		t.Error("Target nvim directory should be a symlink after restore")
	}

	linkTarget, err := os.Readlink(targetNvim)
	if err != nil {
		t.Fatalf("Failed to read symlink: %v", err)
	}
	if linkTarget != nvimBackup {
		t.Errorf("Symlink target = %q, want %q", linkTarget, nvimBackup)
	}

	// Verify tracked files
	verifyTrackedFiles(t, nvimBackup, trackedFiles)

	// Verify merged local files
	verifyMergedFiles(t, nvimBackup, localFiles)

	// Verify conflict resolution
	verifyConflictResolution(t, nvimBackup, conflictFiles)

	// Verify directory structure
	verifyDirectoryStructure(t, nvimBackup)

	// Verify total file count
	verifyFileCount(t, nvimBackup, len(trackedFiles), len(localFiles), len(conflictFiles))
}

func verifyTrackedFiles(t *testing.T, nvimBackup string, trackedFiles map[string]string) {
	t.Helper()
	for relPath, expectedContent := range trackedFiles {
		fullPath := filepath.Join(nvimBackup, relPath)
		if !pathExists(fullPath) {
			t.Errorf("Tracked file %q should still exist in backup", relPath)
			continue
		}

		content, err := os.ReadFile(fullPath) //nolint:gosec // test file
		if err != nil {
			t.Errorf("Failed to read tracked file %q: %v", relPath, err)
			continue
		}

		if string(content) != expectedContent {
			t.Errorf("Tracked file %q content changed. Got %q, want %q",
				relPath, string(content), expectedContent)
		}
	}
}

func verifyMergedFiles(t *testing.T, nvimBackup string, localFiles map[string]string) {
	t.Helper()
	for relPath, expectedContent := range localFiles {
		fullPath := filepath.Join(nvimBackup, relPath)
		if !pathExists(fullPath) {
			t.Errorf("Local file %q should be merged into backup", relPath)
			continue
		}

		content, err := os.ReadFile(fullPath) //nolint:gosec // test file
		if err != nil {
			t.Errorf("Failed to read merged file %q: %v", relPath, err)
			continue
		}

		if string(content) != expectedContent {
			t.Errorf("Merged file %q content = %q, want %q",
				relPath, string(content), expectedContent)
		}
	}
}

func verifyConflictResolution(t *testing.T, nvimBackup string,
	conflictFiles map[string]struct{ backupContent, targetContent string }) {
	t.Helper()
	for relPath, contents := range conflictFiles {
		// Original backup file should preserve backup content
		backupPath := filepath.Join(nvimBackup, relPath)
		backupContent, err := os.ReadFile(backupPath) //nolint:gosec // test file
		if err != nil {
			t.Errorf("Failed to read backup file %q: %v", relPath, err)
			continue
		}

		if string(backupContent) != contents.backupContent {
			t.Errorf("Backup file %q should preserve backup content. Got %q, want %q",
				relPath, string(backupContent), contents.backupContent)
		}

		// Conflict file should exist with target content
		filename := filepath.Base(relPath)
		dir := filepath.Dir(relPath)
		ext := filepath.Ext(filename)
		nameWithoutExt := filename[:len(filename)-len(ext)]

		pattern := filepath.Join(nvimBackup, dir, nameWithoutExt+"_target_*"+ext)
		matches, err := filepath.Glob(pattern)
		if err != nil {
			t.Errorf("Failed to glob conflict files for %q: %v", relPath, err)
			continue
		}

		if len(matches) != 1 {
			t.Errorf("Expected 1 conflict file for %q, got %d", relPath, len(matches))
			continue
		}

		conflictContent, err := os.ReadFile(matches[0]) //nolint:gosec // test file
		if err != nil {
			t.Errorf("Failed to read conflict file %q: %v", matches[0], err)
			continue
		}

		if string(conflictContent) != contents.targetContent {
			t.Errorf("Conflict file should have target content. Got %q, want %q",
				string(conflictContent), contents.targetContent)
		}
	}
}

func verifyDirectoryStructure(t *testing.T, nvimBackup string) {
	t.Helper()
	expectedDirs := []string{
		"lua",
		"lua/core",
		"lua/plugins",
		"after/plugin",
		"spell",
	}

	for _, dir := range expectedDirs {
		fullPath := filepath.Join(nvimBackup, dir)
		if !pathExists(fullPath) {
			t.Errorf("Directory %q should exist in backup", dir)
		}
	}
}

func verifyFileCount(t *testing.T, nvimBackup string, trackedCount, localCount, conflictCount int) {
	t.Helper()
	var fileCount int
	err := filepath.WalkDir(nvimBackup, func(_ string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			fileCount++
		}
		return nil
	})
	if err != nil {
		t.Fatalf("Failed to count files: %v", err)
	}

	expectedFileCount := trackedCount + localCount + conflictCount
	if fileCount != expectedFileCount {
		t.Errorf("Expected %d files in backup, got %d", expectedFileCount, fileCount)
	}
}

// TestMergeIntegration_FullRestoreWorkflow tests the complete merge-on-restore
// workflow with a realistic nvim configuration scenario.
//
// Scenario:
// 1. User has nvim config in dotfiles repo (backup)
// 2. User has local machine-specific nvim plugins (target)
// 3. Restore operation should merge local plugins into backup
// 4. Conflicts should be renamed with _target_YYYYMMDD suffix
// 5. Final state: symlink points to backup with merged content
func TestMergeIntegration_FullRestoreWorkflow(t *testing.T) {
	t.Parallel()

	// Setup test environment
	tmpRoot := t.TempDir()
	nvimBackup, targetNvim, trackedFiles, localFiles, conflictFiles := setupNvimTestEnvironment(t, tmpRoot)

	// Setup: Create manager with config
	dotfilesRepo := filepath.Dir(nvimBackup)
	cfg := &config.Config{BackupRoot: dotfilesRepo}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	entry := config.Entry{Name: "nvim-config"}

	// Act: Perform restore (should trigger merge)
	err := mgr.RestoreFolder(entry, nvimBackup, targetNvim)

	// Assert: No error
	if err != nil {
		t.Fatalf("RestoreFolder() failed: %v", err)
	}

	// Verify all merge results
	verifyMergeResults(t, nvimBackup, targetNvim, trackedFiles, localFiles, conflictFiles)
}

// TestMergeIntegration_WithNoMergeFlag tests that the --no-merge flag
// prevents merge and fails appropriately.
func TestMergeIntegration_WithNoMergeFlag(t *testing.T) {
	t.Parallel()

	tmpRoot := t.TempDir()

	// Setup: Create backup
	dotfilesRepo := filepath.Join(tmpRoot, "dotfiles")
	nvimBackup := filepath.Join(dotfilesRepo, "nvim")
	if err := os.MkdirAll(nvimBackup, DirPerms); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvimBackup, "init.lua"), []byte("tracked"), 0600); err != nil {
		t.Fatal(err)
	}

	// Setup: Create target with content
	targetNvim := filepath.Join(tmpRoot, "config", "nvim")
	if err := os.MkdirAll(targetNvim, DirPerms); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(targetNvim, "local.lua"), []byte("local"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{BackupRoot: dotfilesRepo}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.NoMerge = true

	entry := config.Entry{Name: "nvim-config"}

	// Act: Restore should fail
	err := mgr.RestoreFolder(entry, nvimBackup, targetNvim)

	// Assert: Error returned
	if err == nil {
		t.Fatal("RestoreFolder() should fail with NoMerge=true and non-empty target")
	}

	// Assert: Target still has original content
	if !pathExists(filepath.Join(targetNvim, "local.lua")) {
		t.Error("Target content should be preserved when restore fails")
	}

	// Assert: Target is not a symlink
	if isSymlink(targetNvim) {
		t.Error("Target should not become a symlink when restore fails")
	}
}

// TestMergeIntegration_DryRunMode tests that dry-run mode previews merge
// without actually changing files.
func TestMergeIntegration_DryRunMode(t *testing.T) {
	t.Parallel()

	tmpRoot := t.TempDir()

	// Setup: Create backup
	dotfilesRepo := filepath.Join(tmpRoot, "dotfiles")
	nvimBackup := filepath.Join(dotfilesRepo, "nvim")
	if err := os.MkdirAll(nvimBackup, DirPerms); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nvimBackup, "init.lua"), []byte("tracked"), 0600); err != nil {
		t.Fatal(err)
	}

	// Setup: Create target with content
	targetNvim := filepath.Join(tmpRoot, "config", "nvim")
	if err := os.MkdirAll(targetNvim, DirPerms); err != nil {
		t.Fatal(err)
	}
	targetFile := filepath.Join(targetNvim, "local.lua")
	if err := os.WriteFile(targetFile, []byte("local"), 0600); err != nil {
		t.Fatal(err)
	}

	cfg := &config.Config{BackupRoot: dotfilesRepo}
	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.DryRun = true

	entry := config.Entry{Name: "nvim-config"}

	// Act: Restore in dry-run mode
	err := mgr.RestoreFolder(entry, nvimBackup, targetNvim)

	// Assert: No error
	if err != nil {
		t.Fatalf("RestoreFolder() failed in dry-run: %v", err)
	}

	// Assert: Target file still exists (not actually merged)
	if !pathExists(targetFile) {
		t.Error("Target file should still exist in dry-run mode")
	}

	// Assert: Target is not a symlink (dry-run doesn't change anything)
	if isSymlink(targetNvim) {
		t.Error("Target should not be a symlink in dry-run mode")
	}

	// Assert: Backup doesn't have merged file (dry-run doesn't merge)
	if pathExists(filepath.Join(nvimBackup, "local.lua")) {
		t.Error("Backup should not have merged file in dry-run mode")
	}
}
