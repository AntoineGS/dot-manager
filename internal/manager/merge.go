package manager

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
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

// moveFile preserves a file exclusively before removing the original. Backup
// destinations are user-owned, even when reading/removing the source needs sudo.
func (m *Manager) moveFile(src, dst string, useSudo bool) error {
	return m.moveFileExclusive(src, dst, useSudo, false)
}

func (m *Manager) moveFileExclusive(src, dst string, useSudo, protected bool) error {
	useSudo, err := templateCopySudoPolicy(runtime.GOOS, useSudo)
	if err != nil {
		return err
	}
	if !useSudo && m.isSymlink(src) {
		link, err := m.fs.Readlink(src)
		if err != nil {
			return err
		}
		if err := m.fs.Symlink(link, dst); err != nil {
			return err
		}
	} else {
		snapshot, err := m.readTemplateCopyTarget(src, useSudo)
		if err != nil {
			return err
		}
		if !snapshot.Exists {
			return fmt.Errorf("source %q is missing", src)
		}
		mode := snapshot.Mode
		if protected {
			mode = 0o600
		}
		if err := m.writeTemplateCopyFile(dst, snapshot.Content, mode, false, true); err != nil {
			return err
		}
		// Exclusive creation is subject to umask. Apply the required mode before
		// removing the original, including when the source was read with sudo.
		if err := m.fs.Chmod(dst, mode); err != nil {
			return fmt.Errorf("setting preserved file permissions: %w", err)
		}
	}
	return m.removeTemplateCopyArtifact(src, useSudo)
}

// mergeFile merges a single file from target into backup.
// If the file exists in backup, it's a conflict and the target file is renamed.
// If the file doesn't exist in backup, it's merged directly.
//
// Parameters:
//   - targetFile: Path to the file in the target location
//   - backupDir: Directory where backup files are stored
//   - relativePath: Relative path of the file (used for the backup location)
//   - useSudo: Whether to use sudo for file operations on the target
//   - summary: MergeSummary to record the operation
//
// Returns error if the operation fails.
func (m *Manager) mergeFile(targetFile, backupDir, relativePath string, useSudo bool, summary *MergeSummary) error {
	backupFile := filepath.Join(backupDir, relativePath)

	// Check if file exists in backup (conflict)
	_, statErr := m.fs.Lstat(backupFile)
	if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
		return statErr
	}
	if statErr == nil {
		// CONFLICT: Rename target file and move to backup
		filename := filepath.Base(relativePath)
		conflictName := generateConflictNameWithDate(filename)
		conflictPath := filepath.Join(filepath.Dir(backupFile), conflictName)

		slog.Warn("Conflict detected during merge",
			slog.String("file", relativePath),
			slog.String("renamed_to", conflictName))

		for suffix := 0; ; suffix++ {
			candidate := conflictPath
			if suffix > 0 {
				candidate = fmt.Sprintf("%s.%d", conflictPath, suffix)
			}
			if err := m.moveFileExclusive(targetFile, candidate, useSudo, true); err != nil {
				if errors.Is(err, fs.ErrExist) {
					continue
				}
				return fmt.Errorf("moving conflict file: %w", err)
			}
			conflictName = filepath.Base(candidate)
			break
		}

		summary.AddConflict(relativePath, conflictName)
		return nil
	}

	// NO CONFLICT: Move file to backup
	// Create parent directory if needed (backup is user-owned, no sudo needed)
	backupParent := filepath.Dir(backupFile)
	if err := m.fs.MkdirAll(backupParent, DirPerms); err != nil {
		return fmt.Errorf("creating backup directory: %w", err)
	}

	if err := m.moveFile(targetFile, backupFile, useSudo); err != nil {
		return fmt.Errorf("moving file to backup: %w", err)
	}

	slog.Info("Merged file into backup",
		slog.String("file", relativePath))

	summary.AddMerged(relativePath)
	return nil
}

// MergeFolder recursively merges all files from targetDir into backupDir.
// It walks the target directory tree and calls mergeFile for each file found.
// Directories are skipped (only files are processed).
// Individual file errors are logged and collected while the remaining files
// are attempted. Callers must not remove the target when any error is returned.
//
// Parameters:
//   - backupDir: Directory where backup files are stored
//   - targetDir: Directory to merge files from
//   - useSudo: Whether to use sudo for file operations on the target
//   - summary: MergeSummary to record all operations
//
// Returns an error if any file could not be preserved or the walk fails.
func (m *Manager) MergeFolder(backupDir, targetDir string, useSudo bool, summary *MergeSummary) error {
	var failures []error
	walkErr := m.fs.WalkDir(targetDir, func(path string, d fs.DirEntry, err error) error {
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
				slog.String("path", path),
				slog.String("target_dir", targetDir),
				slog.Any("error", err))
			summary.AddFailed(path, err.Error())
			failures = append(failures, err)
			return nil // Continue walking
		}

		// Merge the file
		if err := m.mergeFile(path, backupDir, relativePath, useSudo, summary); err != nil {
			slog.Error("Failed to merge file",
				slog.String("file", relativePath),
				slog.Any("error", err))
			summary.AddFailed(relativePath, err.Error())
			failures = append(failures, fmt.Errorf("preserving %s: %w", relativePath, err))
			return nil // Continue walking
		}

		return nil
	})
	return errors.Join(append(failures, walkErr)...)
}

// removeEmptyDirs removes empty directories in a bottom-up manner.
// It walks the directory tree, collects all subdirectories (excluding the root),
// and attempts to remove them in reverse order (deepest first).
// Only truly empty directories will be removed; fs.Remove will fail for non-empty dirs.
//
// Parameters:
//   - rootDir: The root directory to clean up (will not be removed itself)
//
// Returns error only if the directory walk itself fails.
// Errors from individual directory removals are logged but don't stop the operation.
func (m *Manager) removeEmptyDirs(rootDir string) error {
	// Collect all subdirectories (not the root itself)
	var dirs []string

	err := m.fs.WalkDir(rootDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip the root directory itself
		if path == rootDir {
			return nil
		}

		// Collect directories
		if d.IsDir() {
			dirs = append(dirs, path)
		}

		return nil
	})

	if err != nil {
		return fmt.Errorf("walking directory tree: %w", err)
	}

	// Process directories in reverse order (deepest first)
	for i := len(dirs) - 1; i >= 0; i-- {
		dir := dirs[i]

		if err := m.fs.Remove(dir); err != nil {
			// Ignore "directory not empty" errors (expected for dirs with content)
			// Log other errors at debug level
			if !errors.Is(err, syscall.ENOTEMPTY) {
				slog.Debug("Failed to remove directory",
					slog.String("dir", dir),
					slog.Any("error", err))
			}
		}
	}

	return nil
}
