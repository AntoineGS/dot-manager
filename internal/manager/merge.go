package manager

import (
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// MergeSummary tracks merge operations for a single application.
// This type is not thread-safe and should not be used concurrently.
type MergeSummary struct {
	AppName       string
	MergedFiles   []string
	ConflictFiles []ConflictInfo
	FailedFiles   []FailedInfo
}

// ConflictInfo tracks files that were renamed due to conflicts
type ConflictInfo struct {
	OriginalName string
	RenamedTo    string
}

// FailedInfo tracks files that failed to merge
type FailedInfo struct {
	FileName string
	Error    string
}

// NewMergeSummary creates a new merge summary for an application
func NewMergeSummary(appName string) *MergeSummary {
	return &MergeSummary{
		AppName:       appName,
		MergedFiles:   []string{},
		ConflictFiles: []ConflictInfo{},
		FailedFiles:   []FailedInfo{},
	}
}

// AddMerged records a successfully merged file
func (s *MergeSummary) AddMerged(fileName string) {
	s.MergedFiles = append(s.MergedFiles, fileName)
}

// AddConflict records a conflict that was resolved by renaming
func (s *MergeSummary) AddConflict(originalName, renamedTo string) {
	s.ConflictFiles = append(s.ConflictFiles, ConflictInfo{
		OriginalName: originalName,
		RenamedTo:    renamedTo,
	})
}

// AddFailed records a file that failed to merge
func (s *MergeSummary) AddFailed(fileName, errMsg string) {
	s.FailedFiles = append(s.FailedFiles, FailedInfo{
		FileName: fileName,
		Error:    errMsg,
	})
}

// HasOperations returns true if any merge operations occurred
func (s *MergeSummary) HasOperations() bool {
	return len(s.MergedFiles) > 0 || len(s.ConflictFiles) > 0 || len(s.FailedFiles) > 0
}

// generateConflictName creates a renamed filename for conflicts
// Example: config.json with date 20260204 -> config_target_20260204.json
func generateConflictName(filename, date string) string {
	ext := filepath.Ext(filename)
	nameWithoutExt := strings.TrimSuffix(filename, ext)

	if ext == "" {
		return nameWithoutExt + "_target_" + date
	}

	return nameWithoutExt + "_target_" + date + ext
}

// generateConflictNameWithDate generates a conflict name using today's date
func generateConflictNameWithDate(filename string) string {
	date := time.Now().Format("20060102")
	return generateConflictName(filename, date)
}

// mergeFile merges a single file from target into backup.
// If the file exists in backup, it's a conflict and the target file is renamed.
// If the file doesn't exist in backup, it's merged directly.
//
// Parameters:
//   - targetFile: Path to the file in the target location
//   - backupDir: Directory where backup files are stored
//   - relativePath: Relative path of the file (used for the backup location)
//   - useSudo: Whether to use sudo for file operations (not yet implemented)
//   - summary: MergeSummary to record the operation
//
// Returns error if the operation fails.
func mergeFile(targetFile, backupDir, relativePath string, useSudo bool, summary *MergeSummary) error {
	if useSudo {
		return fmt.Errorf("sudo not yet supported")
	}

	backupFile := filepath.Join(backupDir, relativePath)

	// Check if file exists in backup (conflict)
	if pathExists(backupFile) {
		// CONFLICT: Rename target file and move to backup
		filename := filepath.Base(relativePath)
		conflictName := generateConflictNameWithDate(filename)
		conflictPath := filepath.Join(filepath.Dir(backupFile), conflictName)

		slog.Warn("Conflict detected during merge",
			"file", relativePath,
			"renamed_to", conflictName)

		// Try to rename first (faster if same device)
		if err := os.Rename(targetFile, conflictPath); err != nil {
			// If rename fails (cross-device), copy then remove
			if copyErr := copyFile(targetFile, conflictPath); copyErr != nil {
				return fmt.Errorf("copying conflict file: %w", copyErr)
			}
			if removeErr := os.Remove(targetFile); removeErr != nil {
				return fmt.Errorf("removing original target file: %w", removeErr)
			}
		}

		summary.AddConflict(relativePath, conflictName)
		return nil
	}

	// NO CONFLICT: Move file to backup
	// Create parent directory if needed
	backupParent := filepath.Dir(backupFile)
	if err := os.MkdirAll(backupParent, DirPerms); err != nil {
		return fmt.Errorf("creating backup directory: %w", err)
	}

	// Try to rename first (faster if same device)
	if err := os.Rename(targetFile, backupFile); err != nil {
		// If rename fails (cross-device), copy then remove
		if copyErr := copyFile(targetFile, backupFile); copyErr != nil {
			return fmt.Errorf("copying file to backup: %w", copyErr)
		}
		if removeErr := os.Remove(targetFile); removeErr != nil {
			return fmt.Errorf("removing original target file: %w", removeErr)
		}
	}

	slog.Info("Merged file into backup",
		"file", relativePath)

	summary.AddMerged(relativePath)
	return nil
}

// MergeFolder recursively merges all files from targetDir into backupDir.
// It walks the target directory tree and calls mergeFile for each file found.
// Directories are skipped (only files are processed).
// Individual file errors are logged but don't stop the overall operation.
//
// Parameters:
//   - backupDir: Directory where backup files are stored
//   - targetDir: Directory to merge files from
//   - useSudo: Whether to use sudo for file operations (not yet implemented)
//   - summary: MergeSummary to record all operations
//
// Returns error only if the directory walk itself fails.
func MergeFolder(backupDir, targetDir string, useSudo bool, summary *MergeSummary) error {
	return filepath.WalkDir(targetDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories, only process files
		if d.IsDir() {
			return nil
		}

		// Calculate relative path from targetDir
		relativePath, err := filepath.Rel(targetDir, path)
		if err != nil {
			slog.Error("Failed to calculate relative path",
				"path", path,
				"target_dir", targetDir,
				"error", err)
			summary.AddFailed(path, err.Error())
			return nil // Continue walking
		}

		// Merge the file
		if err := mergeFile(path, backupDir, relativePath, useSudo, summary); err != nil {
			slog.Error("Failed to merge file",
				"file", relativePath,
				"error", err)
			summary.AddFailed(relativePath, err.Error())
			return nil // Continue walking
		}

		return nil
	})
}
