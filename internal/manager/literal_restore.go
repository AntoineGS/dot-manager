package manager

import (
	"fmt"
	"log/slog"
	"path/filepath"

	"github.com/AntoineGS/tidydots/internal/config"
)

// restoreLiteralFile is the non-template deployment path shared by ordinary
// selected files and rendered template aliases.
//
//nolint:gocyclo // preserves the existing restore conflict/adoption policy
func (m *Manager) restoreLiteralFile(subEntry config.SubEntry, source, target, file string) error {
	srcFile := filepath.Join(source, file)
	dstFile := filepath.Join(target, file)

	// Selected templates can target nested files. RestoreFiles creates the
	// entry root, but each nested parent also needs to exist before symlinking.
	parentDir := filepath.Dir(dstFile)
	if !m.pathExists(parentDir) {
		m.logger.Info("creating directory", slog.String("path", parentDir))
		if !m.DryRun {
			if subEntry.Sudo {
				if _, err := m.runner.RunWithSudo(m.ctx, "mkdir", "-p", parentDir); err != nil {
					return NewPathError("restore", parentDir, fmt.Errorf("creating parent directory: %w", err))
				}
			} else if err := m.fs.MkdirAll(parentDir, DirPerms); err != nil {
				return NewPathError("restore", parentDir, fmt.Errorf("creating parent directory: %w", err))
			}
		}
	}

	// Check if already a symlink pointing to correct source
	if m.symlinkPointsTo(dstFile, srcFile) {
		m.logger.Debug("already a symlink", slog.String("path", dstFile))
		return nil
	}

	// If it's a symlink but points to wrong location, remove it
	if m.isSymlink(dstFile) {
		m.logger.Info("removing incorrect symlink", slog.String("path", dstFile))
		if !m.DryRun {
			if err := m.fs.Remove(dstFile); err != nil {
				return NewPathError("restore", dstFile, fmt.Errorf("removing incorrect symlink: %w", err))
			}
		}
	}

	// Handle merge case: both source and target file exist
	if m.pathExists(srcFile) && m.pathExists(dstFile) && !m.isSymlink(dstFile) {
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
				if err := m.mergeFile(dstFile, source, file, subEntry.Sudo, summary); err != nil {
					return NewPathError("restore", dstFile, fmt.Errorf("merging file: %w", err))
				}

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

	if !m.pathExists(srcFile) && m.pathExists(dstFile) {
		m.logger.Info("adopting file",
			slog.String("from", dstFile),
			slog.String("to", srcFile))

		if !m.DryRun {
			if subEntry.Sudo {
				if _, err := m.runner.RunWithSudo(m.ctx, "mv", dstFile, srcFile); err != nil {
					return NewPathError("adopt", dstFile, fmt.Errorf("moving to backup: %w", err))
				}
			} else {
				if err := m.fs.Rename(dstFile, srcFile); err != nil {
					if err := m.copyFile(dstFile, srcFile); err != nil {
						return NewPathError("adopt", dstFile, fmt.Errorf("copying to backup: %w", err))
					}

					if err := m.fs.Remove(dstFile); err != nil {
						return NewPathError("adopt", dstFile, fmt.Errorf("removing original: %w", err))
					}
				}
			}
		}
	}

	if !m.pathExists(srcFile) {
		if m.DryRun {
			m.logger.Info("source file does not exist (dry-run, skipping)", slog.String("path", srcFile))
			return nil
		}

		return NewPathError("restore", srcFile, fmt.Errorf("source file does not exist"))
	}

	if m.pathExists(dstFile) && !m.isSymlink(dstFile) {
		m.logger.Info("removing file", slog.String("path", dstFile))

		if !m.DryRun {
			if subEntry.Sudo {
				if _, err := m.runner.RunWithSudo(m.ctx, "rm", "-f", dstFile); err != nil {
					return NewPathError("restore", dstFile, fmt.Errorf("removing existing file: %w", err))
				}
			} else if err := m.fs.Remove(dstFile); err != nil {
				return NewPathError("restore", dstFile, fmt.Errorf("removing existing file: %w", err))
			}
		}
	}

	m.logger.Info("creating symlink",
		slog.String("target", dstFile),
		slog.String("source", srcFile))

	if !m.DryRun {
		if err := m.createSymlink(srcFile, dstFile, subEntry.Sudo); err != nil {
			return NewPathError("restore", dstFile, fmt.Errorf("creating symlink: %w", err))
		}
	}

	return nil
}
