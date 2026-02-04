package manager

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/AntoineGS/dot-manager/internal/config"
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
			expandedTarget := m.expandTarget(target)

			if err := m.backupSubEntry(app.Name, subEntry, expandedTarget); err != nil {
				m.logger.Error("backup failed",
					slog.String("app", app.Name),
					slog.String("entry", subEntry.Name),
					slog.String("error", err.Error()))
			}
		}
	}

	return nil
}

func (m *Manager) backupSubEntry(appName string, subEntry config.SubEntry, target string) error {
	backupPath := m.resolvePath(subEntry.Backup)

	if subEntry.IsFolder() {
		return m.backupFolderSubEntry(appName, subEntry, backupPath, target)
	}

	return m.backupFilesSubEntry(appName, subEntry, backupPath, target)
}

func (m *Manager) backupFolderSubEntry(_ string, subEntry config.SubEntry, backup, target string) error {
	// Similar to existing backupFolder logic
	if !pathExists(target) {
		m.logger.Debug("target folder does not exist", slog.String("path", target))
		return nil
	}

	// Skip symlinks - they point to our backup already
	if isSymlink(target) {
		m.logger.Debug("skipping symlink", slog.String("path", target))
		return nil
	}

	m.logger.Info("backing up folder",
		slog.String("from", target),
		slog.String("to", backup))

	if !m.DryRun {
		if err := os.MkdirAll(filepath.Dir(backup), DirPerms); err != nil {
			return NewPathError("backup", backup, fmt.Errorf("creating parent directory: %w", err))
		}

		// Copy source folder into backup directory (e.g., /source/nvim -> /backup/nvim)
		destPath := filepath.Join(backup, filepath.Base(target))
		if subEntry.Sudo {
			cmd := exec.CommandContext(context.Background(), "sudo", "cp", "-r", target, destPath) //nolint:gosec // intentional sudo command
			return cmd.Run()
		}

		return copyDir(target, destPath)
	}

	return nil
}

func (m *Manager) backupFilesSubEntry(_ string, subEntry config.SubEntry, backup, target string) error {
	// Similar to existing backupFiles logic
	if !pathExists(target) {
		m.logger.Debug("target directory does not exist", slog.String("path", target))
		return nil
	}

	if !m.DryRun {
		if err := os.MkdirAll(backup, DirPerms); err != nil {
			return NewPathError("backup", backup, fmt.Errorf("creating backup directory: %w", err))
		}
	}

	for _, file := range subEntry.Files {
		srcFile := filepath.Join(target, file)
		dstFile := filepath.Join(backup, file)

		if !pathExists(srcFile) {
			m.logger.Debug("source file does not exist", slog.String("path", srcFile))
			continue
		}

		// Skip symlinks
		if isSymlink(srcFile) {
			m.logger.Debug("skipping symlink", slog.String("path", srcFile))
			continue
		}

		m.logger.Info("backing up file",
			slog.String("from", srcFile),
			slog.String("to", dstFile))

		if !m.DryRun {
			if subEntry.Sudo {
				cmd := exec.CommandContext(context.Background(), "sudo", "cp", srcFile, dstFile) //nolint:gosec // intentional sudo command
				if err := cmd.Run(); err != nil {
					return NewPathError("backup", srcFile, fmt.Errorf("copying file: %w", err))
				}
			} else {
				if err := copyFile(srcFile, dstFile); err != nil {
					return NewPathError("backup", srcFile, fmt.Errorf("copying file: %w", err))
				}
			}
		}
	}

	return nil
}

func (m *Manager) backupFolder(_, source, backup string) error {
	if !pathExists(source) {
		m.logger.Debug("source folder does not exist", slog.String("path", source))
		return nil
	}

	// Skip symlinks - they point to our backup already
	if isSymlink(source) {
		m.logger.Debug("skipping symlink", slog.String("path", source))
		return nil
	}

	m.logger.Info("backing up folder",
		slog.String("from", source),
		slog.String("to", backup))

	if !m.DryRun {
		// Copy source folder into backup directory (e.g., /source/config -> /backup/config)
		destPath := filepath.Join(backup, filepath.Base(source))
		if err := copyDir(source, destPath); err != nil {
			return NewPathError("backup", source, fmt.Errorf("copying folder: %w", err))
		}
	}

	return nil
}

func (m *Manager) backupFiles(_ string, files []string, source, backup string) error {
	for _, file := range files {
		srcFile := filepath.Join(source, file)
		dstFile := filepath.Join(backup, file)

		if !pathExists(srcFile) {
			m.logger.Debug("source file does not exist", slog.String("path", srcFile))
			continue
		}

		// Skip symlinks
		if isSymlink(srcFile) {
			m.logger.Debug("skipping symlink", slog.String("path", srcFile))
			continue
		}

		m.logger.Info("backing up file",
			slog.String("from", srcFile),
			slog.String("to", dstFile))

		if !m.DryRun {
			if err := copyFile(srcFile, dstFile); err != nil {
				return NewPathError("backup", srcFile, fmt.Errorf("copying file: %w", err))
			}
		}
	}

	return nil
}
