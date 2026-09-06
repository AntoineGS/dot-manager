package manager

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/AntoineGS/tidydots/internal/config"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

// BackupWithContext backs up configurations with context support
func (m *Manager) BackupWithContext(ctx context.Context) error {
	m = m.WithContext(ctx)
	return m.Backup()
}

// Backup copies configuration files from their target locations to the backup directory.
//
//nolint:dupl // similar structure to Restore, but semantically different operations
func (m *Manager) Backup() error {
	// Check context before starting
	if err := m.checkContext(); err != nil {
		return err
	}

	m.logger.Info("backing up configurations", slog.String("os", m.Platform.OS)) //nolint:dupl // similar structure to restoreV3, but semantically different
	apps := m.GetApplications()

	var errs []error

	for _, app := range apps {
		// Check context before each application
		if err := m.checkContext(); err != nil {
			return err
		}

		m.logger.Info("backing up application", slog.String("app", app.Name))

		for _, subEntry := range app.Entries {
			// Check context before each entry
			if err := m.checkContext(); err != nil {
				return err
			}

			if !subEntry.IsConfig() {
				m.logger.Debug("skipping entry",
					slog.String("app", app.Name),
					slog.String("entry", subEntry.Name),
					slog.String("reason", "only config entries can be backed up"))
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
			expandedTarget, err := m.ExpandTarget(target)
			if err != nil {
				errs = append(errs, fmt.Errorf("%s/%s: %w", app.Name, subEntry.Name, err))
				continue
			}

			if err := m.backupSubEntry(app.Name, subEntry, expandedTarget); err != nil {
				m.logger.Error("backup failed",
					slog.String("app", app.Name),
					slog.String("entry", subEntry.Name),
					slog.String("error", err.Error()))
				errs = append(errs, err)
			}
		}
	}

	return errors.Join(errs...)
}

func (m *Manager) backupSubEntry(appName string, subEntry config.SubEntry, target string) error {
	backupPath, err := m.ResolvePath(subEntry.Backup)
	if err != nil {
		return fmt.Errorf("entry %s: %w", subEntry.Name, err)
	}

	if subEntry.IsFolder() {
		return m.backupFolderSubEntry(appName, subEntry, backupPath, target)
	}

	return m.backupFilesSubEntry(appName, subEntry, backupPath, target)
}

func (m *Manager) backupFolderSubEntry(_ string, subEntry config.SubEntry, backup, target string) error {
	// Similar to existing backupFolder logic
	if !m.pathExists(target) {
		m.logger.Debug("target folder does not exist", slog.String("path", target))
		return nil
	}

	// Skip symlinks - they point to our backup already
	if m.isSymlink(target) {
		m.logger.Debug("skipping symlink", slog.String("path", target))
		return nil
	}

	m.logger.Info("backing up folder",
		slog.String("from", target),
		slog.String("to", backup))

	if !m.DryRun {
		if err := m.fs.MkdirAll(filepath.Dir(backup), DirPerms); err != nil {
			return NewPathError("backup", backup, fmt.Errorf("creating parent directory: %w", err))
		}

		// Copy source folder contents into backup directory (e.g., /source/nvim/* -> /backup/*)
		if subEntry.Sudo {
			if _, err := m.runner.RunWithSudo(m.ctx, "cp", "-rT", target, backup); err != nil {
				return err
			}
			return nil
		}

		return m.copyDir(target, backup)
	}

	return nil
}

func (m *Manager) backupFilesSubEntry(_ string, subEntry config.SubEntry, backup, target string) error {
	// Similar to existing backupFiles logic
	if !m.pathExists(target) {
		m.logger.Debug("target directory does not exist", slog.String("path", target))
		return nil
	}

	if !m.DryRun {
		if err := m.fs.MkdirAll(backup, DirPerms); err != nil {
			return NewPathError("backup", backup, fmt.Errorf("creating backup directory: %w", err))
		}
	}

	for _, file := range subEntry.Files {
		if tmpl.IsTemplateFile(file) {
			m.logger.Debug("skipping template source during backup", slog.String("file", file))
			continue
		}

		srcFile := filepath.Join(target, file)
		dstFile := filepath.Join(backup, file)

		if !m.pathExists(srcFile) {
			m.logger.Debug("source file does not exist", slog.String("path", srcFile))
			continue
		}

		// Skip symlinks
		if m.isSymlink(srcFile) {
			m.logger.Debug("skipping symlink", slog.String("path", srcFile))
			continue
		}

		// Skip template-generated artifacts
		if tmpl.IsRenderedFile(file) || tmpl.IsConflictFile(file) {
			m.logger.Debug("skipping template artifact", slog.String("path", srcFile))
			continue
		}

		m.logger.Info("backing up file",
			slog.String("from", srcFile),
			slog.String("to", dstFile))

		if !m.DryRun {
			if subEntry.Sudo {
				if _, err := m.runner.RunWithSudo(m.ctx, "cp", srcFile, dstFile); err != nil {
					return NewPathError("backup", srcFile, fmt.Errorf("copying file: %w", err))
				}
			} else {
				if err := m.copyFile(srcFile, dstFile); err != nil {
					return NewPathError("backup", srcFile, fmt.Errorf("copying file: %w", err))
				}
			}
		}
	}

	return nil
}
