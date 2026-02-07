package manager

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AntoineGS/dot-manager/internal/config"
	tmpl "github.com/AntoineGS/dot-manager/internal/template"
)

// RestoreWithContext restores configurations with context support
func (m *Manager) RestoreWithContext(ctx context.Context) error {
	m = m.WithContext(ctx)
	return m.Restore()
}

// Restore creates symlinks from target locations to backup sources for all managed configuration files.
//
//nolint:dupl // similar structure to Backup, but semantically different operations
func (m *Manager) Restore() error {
	// Check context before starting
	if err := m.checkContext(); err != nil {
		return err
	}

	m.logger.Info("starting restore",
		slog.String("os", m.Platform.OS),
		slog.Int("version", m.Config.Version),
	)

	apps := m.GetApplications()

	var errs []error

	for _, app := range apps {
		// Check context before each application
		if err := m.checkContext(); err != nil {
			return err
		}

		m.logger.Info("restoring application", slog.String("app", app.Name))

		for _, subEntry := range app.Entries {
			// Check context before each entry
			if err := m.checkContext(); err != nil {
				return err
			}

			// Only process config entries
			if !subEntry.IsConfig() {
				m.logger.Debug("skipping entry",
					slog.String("app", app.Name),
					slog.String("entry", subEntry.Name),
					slog.String("reason", "not a config entry"))
				continue
			}

			target := subEntry.GetTarget(m.Platform.OS)
			if target == "" {
				m.logger.Debug("skipping entry",
					slog.String("app", app.Name),
					slog.String("entry", subEntry.Name),
					slog.String("os", m.Platform.OS),
					slog.String("reason", "no target for OS"))
				continue
			}

			// Expand ~ and env vars in target path for file operations
			expandedTarget := m.expandTarget(target)

			if err := m.restoreSubEntry(app.Name, subEntry, expandedTarget); err != nil {
				m.logger.Error("restore failed",
					slog.String("app", app.Name),
					slog.String("entry", subEntry.Name),
					slog.String("error", err.Error()))
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}

// symlinkPointsTo checks if a symlink at 'path' points to 'expectedTarget'
func symlinkPointsTo(path, expectedTarget string) bool {
	if !isSymlink(path) {
		return false
	}

	link, err := os.Readlink(path)
	if err != nil {
		return false
	}

	return link == expectedTarget
}

func createSymlink(ctx context.Context, source, target string, useSudo bool) error {
	// Validate source exists
	if _, err := os.Stat(source); err != nil {
		if os.IsNotExist(err) {
			return NewPathError("restore", source, fmt.Errorf("symlink source does not exist"))
		}

		return NewPathError("restore", source, fmt.Errorf("cannot access symlink source: %w", err))
	}

	if runtime.GOOS == "windows" {
		// Check if source is a directory
		info, err := os.Stat(source)
		if err != nil {
			return err
		}

		if info.IsDir() {
			// Use mklink /J for directory junctions on Windows
			cmd := exec.CommandContext(ctx, "cmd", "/c", "mklink", "/J", target, source)
			return cmd.Run()
		}
		// Use mklink for files
		cmd := exec.CommandContext(ctx, "cmd", "/c", "mklink", target, source)

		return cmd.Run()
	}

	if useSudo {
		cmd := exec.CommandContext(ctx, "sudo", "ln", "-s", source, target) //nolint:gosec // intentional sudo command
		return cmd.Run()
	}

	return os.Symlink(source, target)
}

func (m *Manager) restoreSubEntry(_ string, subEntry config.SubEntry, target string) error {
	backupPath := m.resolvePath(subEntry.Backup)

	if subEntry.IsFolder() {
		// Check if folder contains template files
		if hasTemplateFiles(backupPath) {
			return m.RestoreFolderWithTemplates(subEntry, backupPath, target)
		}
		return m.RestoreFolder(subEntry, backupPath, target)
	}

	return m.RestoreFiles(subEntry, backupPath, target)
}

// RestoreFolder creates a symlink from target to source for a folder entry.
//
//nolint:gocyclo // complexity acceptable for restore logic
func (m *Manager) RestoreFolder(subEntry config.SubEntry, source, target string) error {
	// Check if already a symlink pointing to the correct source
	if symlinkPointsTo(target, source) {
		m.logger.Debug("already a symlink", slog.String("path", target))
		return nil
	}

	// If it's a symlink but points to wrong location, remove it
	if isSymlink(target) {
		m.logger.Info("removing incorrect symlink", slog.String("path", target))
		if !m.DryRun {
			if err := os.Remove(target); err != nil {
				return NewPathError("restore", target, fmt.Errorf("removing incorrect symlink: %w", err))
			}
		}
	}

	// Handle merge case: both source and target exist
	if pathExists(source) && pathExists(target) && !isSymlink(target) {
		if m.NoMerge {
			if !m.ForceDelete {
				// List files and return error
				var fileList []string
				err := filepath.WalkDir(target, func(path string, d fs.DirEntry, walkErr error) error {
					if walkErr != nil {
						return walkErr
					}
					if !d.IsDir() {
						relPath, _ := filepath.Rel(target, path)
						fileList = append(fileList, relPath)
					}
					return nil
				})
				if err != nil {
					return NewPathError("restore", target, fmt.Errorf("listing files: %w", err))
				}
				return NewPathError("restore", target, fmt.Errorf(
					"target exists with %d file(s): %s. Use merge mode or --force to proceed",
					len(fileList),
					strings.Join(fileList, ", ")))
			}
			// ForceDelete is true, skip to removal logic below
		} else {
			// Merge target into backup
			m.logger.Info("merging existing content into backup",
				slog.String("target", target),
				slog.String("backup", source))

			if !m.DryRun {
				summary := NewMergeSummary(subEntry.Name)
				if err := MergeFolder(source, target, subEntry.Sudo, summary); err != nil {
					return NewPathError("restore", target, fmt.Errorf("merging folder: %w", err))
				}

				// Log merge summary
				if summary.HasOperations() {
					m.logger.Info("merge complete",
						slog.Int("merged", len(summary.MergedFiles)),
						slog.Int("conflicts", len(summary.ConflictFiles)),
						slog.Int("failed", len(summary.FailedFiles)))

					for _, conflict := range summary.ConflictFiles {
						m.logger.Warn("conflict resolved by renaming",
							slog.String("file", conflict.OriginalName),
							slog.String("renamed_to", conflict.RenamedTo))
					}

					for _, failed := range summary.FailedFiles {
						m.logger.Error("merge failed for file",
							slog.String("file", failed.FileName),
							slog.String("error", failed.Error))
					}
				}

				// Remove empty directories after merge
				if err := removeEmptyDirs(target); err != nil {
					m.logger.Warn("failed to clean up empty directories",
						slog.String("target", target),
						slog.String("error", err.Error()))
				}
			}
		}
	}

	if !pathExists(source) && pathExists(target) {
		m.logger.Info("adopting folder",
			slog.String("from", target),
			slog.String("to", source))

		if !m.DryRun {
			backupParent := filepath.Dir(source)
			if !pathExists(backupParent) {
				if err := os.MkdirAll(backupParent, DirPerms); err != nil {
					return NewPathError("adopt", source, fmt.Errorf("creating backup parent: %w", err))
				}
			}

			if subEntry.Sudo {
				cmd := exec.CommandContext(m.ctx, "sudo", "mv", target, source) //nolint:gosec // intentional sudo command
				if err := cmd.Run(); err != nil {
					return NewPathError("adopt", target, fmt.Errorf("moving to backup: %w", err))
				}
			} else {
				if err := os.Rename(target, source); err != nil {
					return NewPathError("adopt", target, fmt.Errorf("moving to backup: %w", err))
				}
			}
		}
	}

	if !pathExists(source) {
		m.logger.Debug("source folder does not exist", slog.String("path", source))
		return nil
	}

	parentDir := filepath.Dir(target)
	if !pathExists(parentDir) {
		m.logger.Info("creating directory", slog.String("path", parentDir))

		if !m.DryRun {
			if subEntry.Sudo {
				cmd := exec.CommandContext(m.ctx, "sudo", "mkdir", "-p", parentDir) //nolint:gosec // intentional sudo command
				if err := cmd.Run(); err != nil {
					return NewPathError("restore", parentDir, fmt.Errorf("creating parent: %w", err))
				}
			} else {
				if err := os.MkdirAll(parentDir, DirPerms); err != nil {
					return NewPathError("restore", parentDir, fmt.Errorf("creating parent: %w", err))
				}
			}
		}
	}

	if pathExists(target) && !isSymlink(target) {
		m.logger.Info("removing folder", slog.String("path", target))

		if !m.DryRun {
			if subEntry.Sudo {
				cmd := exec.CommandContext(m.ctx, "sudo", "rm", "-rf", target) //nolint:gosec // intentional sudo command
				if err := cmd.Run(); err != nil {
					return NewPathError("restore", target, fmt.Errorf("removing existing: %w", err))
				}
			} else {
				if err := removeAll(target); err != nil {
					return NewPathError("restore", target, fmt.Errorf("removing existing: %w", err))
				}
			}
		}
	}

	m.logger.Info("creating symlink",
		slog.String("target", target),
		slog.String("source", source))

	if !m.DryRun {
		return createSymlink(m.ctx, source, target, subEntry.Sudo)
	}

	return nil
}

// RestoreFiles creates symlinks from target to source for individual files in an entry.
//
//nolint:gocyclo // complexity acceptable for restore logic
func (m *Manager) RestoreFiles(subEntry config.SubEntry, source, target string) error {
	if !pathExists(source) {
		if !m.DryRun {
			if err := os.MkdirAll(source, DirPerms); err != nil {
				return NewPathError("restore", source, fmt.Errorf("creating backup directory: %w", err))
			}
		}
	}

	if !pathExists(target) {
		m.logger.Info("creating directory", slog.String("path", target))

		if !m.DryRun {
			if subEntry.Sudo {
				cmd := exec.CommandContext(m.ctx, "sudo", "mkdir", "-p", target) //nolint:gosec // intentional sudo command
				if err := cmd.Run(); err != nil {
					return NewPathError("restore", target, fmt.Errorf("creating target directory: %w", err))
				}
			} else {
				if err := os.MkdirAll(target, DirPerms); err != nil {
					return NewPathError("restore", target, fmt.Errorf("creating target directory: %w", err))
				}
			}
		}
	}

	for _, file := range subEntry.Files {
		srcFile := filepath.Join(source, file)
		dstFile := filepath.Join(target, file)

		// Check if already a symlink pointing to correct source
		if symlinkPointsTo(dstFile, srcFile) {
			m.logger.Debug("already a symlink", slog.String("path", dstFile))
			continue
		}

		// If it's a symlink but points to wrong location, remove it
		if isSymlink(dstFile) {
			m.logger.Info("removing incorrect symlink", slog.String("path", dstFile))
			if !m.DryRun {
				if err := os.Remove(dstFile); err != nil {
					return NewPathError("restore", dstFile, fmt.Errorf("removing incorrect symlink: %w", err))
				}
			}
		}

		// Handle merge case: both source and target file exist
		if pathExists(srcFile) && pathExists(dstFile) && !isSymlink(dstFile) {
			if m.NoMerge {
				if !m.ForceDelete {
					return NewPathError("restore", dstFile, fmt.Errorf(
						"target file exists. Use merge mode or --force to proceed"))
				}
				// ForceDelete is true, skip to removal logic below
			} else {
				// Merge target file into backup
				m.logger.Info("merging existing file into backup",
					slog.String("target", dstFile),
					slog.String("backup", srcFile))

				if !m.DryRun {
					summary := NewMergeSummary(subEntry.Name)
					if err := mergeFile(dstFile, source, file, subEntry.Sudo, summary); err != nil {
						return NewPathError("restore", dstFile, fmt.Errorf("merging file: %w", err))
					}

					// Log merge summary
					if summary.HasOperations() {
						for _, conflict := range summary.ConflictFiles {
							m.logger.Warn("conflict resolved by renaming",
								slog.String("file", conflict.OriginalName),
								slog.String("renamed_to", conflict.RenamedTo))
						}

						for _, failed := range summary.FailedFiles {
							m.logger.Error("merge failed for file",
								slog.String("file", failed.FileName),
								slog.String("error", failed.Error))
						}
					}
				}
			}
		}

		if !pathExists(srcFile) && pathExists(dstFile) {
			m.logger.Info("adopting file",
				slog.String("from", dstFile),
				slog.String("to", srcFile))

			if !m.DryRun {
				if subEntry.Sudo {
					cmd := exec.CommandContext(m.ctx, "sudo", "mv", dstFile, srcFile) //nolint:gosec // intentional sudo command
					if err := cmd.Run(); err != nil {
						return NewPathError("adopt", dstFile, fmt.Errorf("moving to backup: %w", err))
					}
				} else {
					if err := os.Rename(dstFile, srcFile); err != nil {
						if err := copyFile(dstFile, srcFile); err != nil {
							return NewPathError("adopt", dstFile, fmt.Errorf("copying to backup: %w", err))
						}

						if err := os.Remove(dstFile); err != nil {
							return NewPathError("adopt", dstFile, fmt.Errorf("removing original: %w", err))
						}
					}
				}
			}
		}

		if !pathExists(srcFile) {
			m.logger.Debug("source file does not exist", slog.String("path", srcFile))
			continue
		}

		if pathExists(dstFile) && !isSymlink(dstFile) {
			m.logger.Info("removing file", slog.String("path", dstFile))

			if !m.DryRun {
				if subEntry.Sudo {
					cmd := exec.CommandContext(m.ctx, "sudo", "rm", "-f", dstFile) //nolint:gosec // intentional sudo command
					if err := cmd.Run(); err != nil {
						return NewPathError("restore", dstFile, fmt.Errorf("removing existing file: %w", err))
					}
				} else {
					if err := os.Remove(dstFile); err != nil {
						return NewPathError("restore", dstFile, fmt.Errorf("removing existing file: %w", err))
					}
				}
			}
		}

		m.logger.Info("creating symlink",
			slog.String("target", dstFile),
			slog.String("source", srcFile))

		if !m.DryRun {
			if err := createSymlink(m.ctx, srcFile, dstFile, subEntry.Sudo); err != nil {
				return NewPathError("restore", dstFile, fmt.Errorf("creating symlink: %w", err))
			}
		}
	}

	return nil
}

// hasTemplateFiles returns true if the directory contains any .tmpl files.
// It walks the directory tree and returns early on the first match.
func hasTemplateFiles(dir string) bool {
	if !pathExists(dir) {
		return false
	}

	found := false
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || found {
			return filepath.SkipDir
		}
		if !d.IsDir() && tmpl.IsTemplateFile(d.Name()) {
			found = true
			return filepath.SkipAll
		}
		return nil
	})

	return found
}
