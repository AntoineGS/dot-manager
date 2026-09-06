package manager

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/platform"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
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

			// Setup entries run their check, and their run command if it fails.
			// They are dispatched in YAML order, so a setup entry listed after
			// config entries runs after those entries are deployed.
			if subEntry.IsSetup() {
				if err := m.runSetupEntry(app.Name, subEntry); err != nil {
					m.logger.Error("setup failed",
						slog.String("app", app.Name),
						slog.String("entry", subEntry.Name),
						slog.String("error", err.Error()))

					errs = append(errs, err)
				}

				continue
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

// symlinkPointsTo checks if a symlink at 'path' points to 'expectedTarget'.
func (m *Manager) symlinkPointsTo(path, expectedTarget string) bool {
	if !m.isSymlink(path) {
		return false
	}

	link, err := m.fs.Readlink(path)
	if err != nil {
		return false
	}

	return link == expectedTarget
}

// createSymlink creates a symbolic link from source to target using the
// Manager's filesystem and runner abstractions. When useSudo is true and
// the OS supports it, the underlying ln command is executed with sudo.
func (m *Manager) createSymlink(source, target string, useSudo bool) error {
	// Validate source exists
	if _, err := m.fs.Stat(source); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewPathError("restore", source, fmt.Errorf("symlink source does not exist"))
		}

		return NewPathError("restore", source, fmt.Errorf("cannot access symlink source: %w", err))
	}

	if useSudo && runtime.GOOS != platform.OSWindows {
		if _, err := m.runner.RunWithSudo(m.ctx, "ln", "-s", source, target); err != nil {
			return err
		}
		return nil
	}

	return m.fs.Symlink(source, target)
}

func (m *Manager) restoreSubEntry(_ string, subEntry config.SubEntry, target string) error {
	backupPath := m.resolvePath(subEntry.Backup)

	if subEntry.IsFolder() {
		// Check if folder contains template files
		if m.hasTemplateFiles(backupPath) {
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
	if m.symlinkPointsTo(target, source) {
		m.logger.Debug("already a symlink", slog.String("path", target))
		return nil
	}

	// A symlinked parent can make distinct path strings refer to the same
	// directory. Refuse before merge or removal can destroy the source.
	sourceInfo, sourceErr := m.fs.Stat(source)
	targetInfo, targetErr := m.fs.Stat(target)
	if sourceErr == nil && targetErr == nil && os.SameFile(sourceInfo, targetInfo) {
		return NewPathError("restore", target, errors.New("source and target resolve to the same location"))
	}

	// If it's a symlink but points to wrong location, remove it
	if m.isSymlink(target) {
		m.logger.Info("removing incorrect symlink", slog.String("path", target))
		if !m.DryRun {
			if err := m.fs.Remove(target); err != nil {
				return NewPathError("restore", target, fmt.Errorf("removing incorrect symlink: %w", err))
			}
		}
	}

	// Handle merge case: both source and target exist
	if m.pathExists(source) && m.pathExists(target) && !m.isSymlink(target) {
		if m.NoMerge {
			if !m.ForceDelete {
				// List files and return error
				var fileList []string
				err := m.fs.WalkDir(target, func(path string, d fs.DirEntry, walkErr error) error {
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
				if err := m.MergeFolder(source, target, subEntry.Sudo, summary); err != nil {
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
				if err := m.removeEmptyDirs(target); err != nil {
					m.logger.Warn("failed to clean up empty directories",
						slog.String("target", target),
						slog.String("error", err.Error()))
				}
			}
		}
	}

	if !m.pathExists(source) && m.pathExists(target) {
		m.logger.Info("adopting folder",
			slog.String("from", target),
			slog.String("to", source))

		if !m.DryRun {
			backupParent := filepath.Dir(source)
			if !m.pathExists(backupParent) {
				if err := m.fs.MkdirAll(backupParent, DirPerms); err != nil {
					return NewPathError("adopt", source, fmt.Errorf("creating backup parent: %w", err))
				}
			}

			if subEntry.Sudo {
				if _, err := m.runner.RunWithSudo(m.ctx, "mv", target, source); err != nil {
					return NewPathError("adopt", target, fmt.Errorf("moving to backup: %w", err))
				}
			} else {
				if err := m.fs.Rename(target, source); err != nil {
					return NewPathError("adopt", target, fmt.Errorf("moving to backup: %w", err))
				}
			}
		}
	}

	if !m.pathExists(source) {
		if m.DryRun {
			m.logger.Info("source folder does not exist (dry-run, skipping)", slog.String("path", source))
			return nil
		}

		return NewPathError("restore", source, fmt.Errorf("source folder does not exist"))
	}

	parentDir := filepath.Dir(target)
	if !m.pathExists(parentDir) {
		m.logger.Info("creating directory", slog.String("path", parentDir))

		if !m.DryRun {
			if subEntry.Sudo {
				if _, err := m.runner.RunWithSudo(m.ctx, "mkdir", "-p", parentDir); err != nil {
					return NewPathError("restore", parentDir, fmt.Errorf("creating parent: %w", err))
				}
			} else {
				if err := m.fs.MkdirAll(parentDir, DirPerms); err != nil {
					return NewPathError("restore", parentDir, fmt.Errorf("creating parent: %w", err))
				}
			}
		}
	}

	if m.pathExists(target) && !m.isSymlink(target) {
		m.logger.Info("removing folder", slog.String("path", target))

		if !m.DryRun {
			if subEntry.Sudo {
				if _, err := m.runner.RunWithSudo(m.ctx, "rm", "-rf", target); err != nil {
					return NewPathError("restore", target, fmt.Errorf("removing existing: %w", err))
				}
			} else {
				if err := m.removeAll(target); err != nil {
					return NewPathError("restore", target, fmt.Errorf("removing existing: %w", err))
				}
			}
		}
	}

	m.logger.Info("creating symlink",
		slog.String("target", target),
		slog.String("source", source))

	if !m.DryRun {
		return m.createSymlink(source, target, subEntry.Sudo)
	}

	return nil
}

// RestoreFiles creates symlinks from target to source for individual files in an entry.
//
//nolint:gocyclo // complexity acceptable for restore logic
func (m *Manager) RestoreFiles(subEntry config.SubEntry, source, target string) error {
	if !subEntry.IsCopy() {
		if err := m.preflightTemplateFiles(subEntry, source, target); err != nil {
			return err
		}
	}

	if !m.pathExists(source) {
		if !m.DryRun {
			if err := m.fs.MkdirAll(source, DirPerms); err != nil {
				return NewPathError("restore", source, fmt.Errorf("creating backup directory: %w", err))
			}
		}
	}

	if !m.pathExists(target) {
		m.logger.Info("creating directory", slog.String("path", target))

		if !m.DryRun {
			if subEntry.Sudo {
				if _, err := m.runner.RunWithSudo(m.ctx, "mkdir", "-p", target); err != nil {
					return NewPathError("restore", target, fmt.Errorf("creating target directory: %w", err))
				}
			} else {
				if err := m.fs.MkdirAll(target, DirPerms); err != nil {
					return NewPathError("restore", target, fmt.Errorf("creating target directory: %w", err))
				}
			}
		}
	}

	for _, file := range subEntry.Files {
		if subEntry.IsCopy() {
			srcFile := filepath.Join(source, file)
			dstFile := filepath.Join(target, file)
			if err := m.restoreFileCopy(subEntry, srcFile, dstFile); err != nil {
				return err
			}
			continue
		}

		if tmpl.IsTemplateFile(file) {
			if err := m.restoreTemplateFile(subEntry, source, target, file); err != nil {
				return err
			}
			continue
		}

		if err := m.restoreLiteralFile(subEntry, source, target, file); err != nil {
			return err
		}
	}

	return nil
}
