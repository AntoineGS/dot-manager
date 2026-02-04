package manager

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/AntoineGS/dot-manager/internal/config"
)

// RestoreWithContext restores configurations with context support
func (m *Manager) RestoreWithContext(ctx context.Context) error {
	m = m.WithContext(ctx)
	return m.Restore()
}

// Restore creates symlinks from target locations to backup sources for all managed configuration files.
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

	for _, app := range apps {
		// Check context before each application
		if err := m.checkContext(); err != nil {
			return err
		}

		m.logf("Restoring application: %s", app.Name)

		for _, subEntry := range app.Entries {
			// Check context before each entry
			if err := m.checkContext(); err != nil {
				return err
			}

			// Only process config entries
			if !subEntry.IsConfig() {
				m.logVerbosef("Skipping %s/%s: not a config entry", app.Name, subEntry.Name)
				continue
			}

			target := subEntry.GetTarget(m.Platform.OS)
			if target == "" {
				m.logVerbosef("Skipping %s/%s: no target for OS %s", app.Name, subEntry.Name, m.Platform.OS)
				continue
			}

			// Expand ~ and env vars in target path for file operations
			expandedTarget := m.expandTarget(target)

			if err := m.restoreSubEntry(app.Name, subEntry, expandedTarget); err != nil {
				m.logf("Error restoring %s/%s: %v", app.Name, subEntry.Name, err)
			}
		}
	}

	return nil
}

func (m *Manager) restoreEntry(entry config.Entry, target string) error {
	backupPath := m.resolvePath(entry.Backup)

	if entry.IsFolder() {
		return m.RestoreFolder(entry, backupPath, target)
	}

	return m.RestoreFiles(entry, backupPath, target)
}

// RestoreFolder creates a symlink from target to source for a folder entry
//
//nolint:gocyclo,dupl // refactoring would risk breaking existing logic
func (m *Manager) RestoreFolder(entry config.Entry, source, target string) error {
	// Check if already a symlink pointing to the correct source
	if symlinkPointsTo(target, source) {
		m.logVerbosef("Already a symlink pointing to correct source: %s", target)
		return nil
	}

	// If it's a symlink but points to wrong location, remove it
	if isSymlink(target) {
		m.logf("Symlink points to wrong location, removing: %s", target)
		if !m.DryRun {
			if err := os.Remove(target); err != nil {
				return NewPathError("restore", target, fmt.Errorf("removing incorrect symlink: %w", err))
			}
		}
	}

	// Check if we need to adopt: target exists but backup doesn't
	if !pathExists(source) && pathExists(target) {
		m.logf("Adopting folder %s -> %s", target, source)

		if !m.DryRun {
			// Create backup parent directory
			backupParent := filepath.Dir(source)
			if !pathExists(backupParent) {
				if err := os.MkdirAll(backupParent, 0750); err != nil {
					return NewPathError("adopt", source, fmt.Errorf("creating backup parent: %w", err))
				}
			}
			// Move target to backup location
			if entry.Sudo {
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

	// Now check if backup exists
	if !pathExists(source) {
		m.logVerbosef("Source folder does not exist: %s", source)
		return nil
	}

	// Create parent directory if it doesn't exist
	parentDir := filepath.Dir(target)
	if !pathExists(parentDir) {
		m.logf("Creating directory %s", parentDir)

		if !m.DryRun {
			if entry.Sudo {
				cmd := exec.CommandContext(m.ctx, "sudo", "mkdir", "-p", parentDir) //nolint:gosec // intentional sudo command
				if err := cmd.Run(); err != nil {
					return NewPathError("restore", parentDir, fmt.Errorf("creating parent: %w", err))
				}
			} else {
				if err := os.MkdirAll(parentDir, 0750); err != nil {
					return NewPathError("restore", parentDir, fmt.Errorf("creating parent: %w", err))
				}
			}
		}
	}

	// Remove existing folder (if still there after adopt check)
	if pathExists(target) && !isSymlink(target) {
		m.logf("Removing folder %s", target)

		if !m.DryRun {
			if entry.Sudo {
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

	m.logf("Creating symlink %s -> %s", target, source)

	if !m.DryRun {
		if err := createSymlink(source, target, entry.Sudo); err != nil {
			return NewPathError("restore", target, fmt.Errorf("creating symlink: %w", err))
		}
	}

	return nil
}

// RestoreFiles creates symlinks from target to source for individual files in an entry
//
//nolint:dupl,gocyclo // similar logic for SubEntry version, complexity acceptable
func (m *Manager) RestoreFiles(entry config.Entry, source, target string) error {
	// Create backup directory if it doesn't exist (needed for adopting)
	if !pathExists(source) {
		if !m.DryRun {
			if err := os.MkdirAll(source, 0750); err != nil {
				return NewPathError("restore", source, fmt.Errorf("creating backup directory: %w", err))
			}
		}
	}

	// Create target directory if it doesn't exist
	if !pathExists(target) {
		m.logf("Creating directory %s", target)

		if !m.DryRun {
			if entry.Sudo {
				cmd := exec.CommandContext(m.ctx, "sudo", "mkdir", "-p", target) //nolint:gosec // intentional sudo command
				if err := cmd.Run(); err != nil {
					return NewPathError("restore", target, fmt.Errorf("creating target directory: %w", err))
				}
			} else {
				if err := os.MkdirAll(target, 0750); err != nil {
					return NewPathError("restore", target, fmt.Errorf("creating target directory: %w", err))
				}
			}
		}
	}

	for _, file := range entry.Files {
		srcFile := filepath.Join(source, file)
		dstFile := filepath.Join(target, file)

		// Check if already a symlink pointing to correct source
		if symlinkPointsTo(dstFile, srcFile) {
			m.logVerbosef("Already a symlink pointing to correct source: %s", dstFile)
			continue
		}

		// If it's a symlink but points to wrong location, remove it
		if isSymlink(dstFile) {
			m.logf("Symlink points to wrong location, removing: %s", dstFile)
			if !m.DryRun {
				if err := os.Remove(dstFile); err != nil {
					return NewPathError("restore", dstFile, fmt.Errorf("removing incorrect symlink: %w", err))
				}
			}
		}

		// Check if we need to adopt: target exists but backup doesn't
		if !pathExists(srcFile) && pathExists(dstFile) {
			m.logf("Adopting file %s -> %s", dstFile, srcFile)

			if !m.DryRun {
				// Move target file to backup location
				if entry.Sudo {
					cmd := exec.CommandContext(m.ctx, "sudo", "mv", dstFile, srcFile) //nolint:gosec // intentional sudo command
					if err := cmd.Run(); err != nil {
						return NewPathError("adopt", dstFile, fmt.Errorf("moving to backup: %w", err))
					}
				} else {
					if err := os.Rename(dstFile, srcFile); err != nil {
						// If rename fails (cross-device), try copy and delete
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
			m.logVerbosef("Source file does not exist: %s", srcFile)
			continue
		}

		// Remove existing file (if still there after adopt check)
		if pathExists(dstFile) && !isSymlink(dstFile) {
			m.logf("Removing file %s", dstFile)

			if !m.DryRun {
				if entry.Sudo {
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

		m.logf("Creating symlink %s -> %s", dstFile, srcFile)

		if !m.DryRun {
			if err := createSymlink(srcFile, dstFile, entry.Sudo); err != nil {
				return NewPathError("restore", dstFile, fmt.Errorf("creating symlink: %w", err))
			}
		}
	}

	return nil
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

func createSymlink(source, target string, useSudo bool) error {
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
			cmd := exec.CommandContext(context.Background(), "cmd", "/c", "mklink", "/J", target, source)
			return cmd.Run()
		}
		// Use mklink for files
		cmd := exec.CommandContext(context.Background(), "cmd", "/c", "mklink", target, source)

		return cmd.Run()
	}

	if useSudo {
		cmd := exec.CommandContext(context.Background(), "sudo", "ln", "-s", source, target) //nolint:gosec // intentional sudo command
		return cmd.Run()
	}

	return os.Symlink(source, target)
}

// restoreGitEntry clones or updates a git repository
func (m *Manager) restoreSubEntry(appName string, subEntry config.SubEntry, target string) error {
	backupPath := m.resolvePath(subEntry.Backup)

	if subEntry.IsFolder() {
		return m.restoreFolderSubEntry(appName, subEntry, backupPath, target)
	}

	return m.restoreFilesSubEntry(appName, subEntry, backupPath, target)
}

//nolint:gocyclo,dupl // refactoring would risk breaking existing logic
func (m *Manager) restoreFolderSubEntry(_ string, subEntry config.SubEntry, source, target string) error {
	// Similar to restoreFolder but use subEntry fields
	// Check if already a symlink pointing to the correct source
	if symlinkPointsTo(target, source) {
		m.logVerbosef("Already a symlink pointing to correct source: %s", target)
		return nil
	}

	// If it's a symlink but points to wrong location, remove it
	if isSymlink(target) {
		m.logf("Symlink points to wrong location, removing: %s", target)
		if !m.DryRun {
			if err := os.Remove(target); err != nil {
				return NewPathError("restore", target, fmt.Errorf("removing incorrect symlink: %w", err))
			}
		}
	}

	if !pathExists(source) && pathExists(target) {
		m.logf("Adopting folder %s -> %s", target, source)

		if !m.DryRun {
			backupParent := filepath.Dir(source)
			if !pathExists(backupParent) {
				if err := os.MkdirAll(backupParent, 0750); err != nil {
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
		m.logVerbosef("Source folder does not exist: %s", source)
		return nil
	}

	parentDir := filepath.Dir(target)
	if !pathExists(parentDir) {
		m.logf("Creating directory %s", parentDir)

		if !m.DryRun {
			if subEntry.Sudo {
				cmd := exec.CommandContext(m.ctx, "sudo", "mkdir", "-p", parentDir) //nolint:gosec // intentional sudo command
				if err := cmd.Run(); err != nil {
					return NewPathError("restore", parentDir, fmt.Errorf("creating parent: %w", err))
				}
			} else {
				if err := os.MkdirAll(parentDir, 0750); err != nil {
					return NewPathError("restore", parentDir, fmt.Errorf("creating parent: %w", err))
				}
			}
		}
	}

	if pathExists(target) && !isSymlink(target) {
		m.logf("Removing folder %s", target)

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

	m.logf("Creating symlink %s -> %s", target, source)

	if !m.DryRun {
		return createSymlink(source, target, subEntry.Sudo)
	}

	return nil
}

//nolint:dupl,gocyclo // similar logic to restoreFiles, complexity acceptable
func (m *Manager) restoreFilesSubEntry(_ string, subEntry config.SubEntry, source, target string) error {
	// Similar to restoreFiles but use subEntry fields
	if !pathExists(source) {
		if !m.DryRun {
			if err := os.MkdirAll(source, 0750); err != nil {
				return NewPathError("restore", source, fmt.Errorf("creating backup directory: %w", err))
			}
		}
	}

	if !pathExists(target) {
		m.logf("Creating directory %s", target)

		if !m.DryRun {
			if subEntry.Sudo {
				cmd := exec.CommandContext(m.ctx, "sudo", "mkdir", "-p", target) //nolint:gosec // intentional sudo command
				if err := cmd.Run(); err != nil {
					return NewPathError("restore", target, fmt.Errorf("creating target directory: %w", err))
				}
			} else {
				if err := os.MkdirAll(target, 0750); err != nil {
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
			m.logVerbosef("Already a symlink pointing to correct source: %s", dstFile)
			continue
		}

		// If it's a symlink but points to wrong location, remove it
		if isSymlink(dstFile) {
			m.logf("Symlink points to wrong location, removing: %s", dstFile)
			if !m.DryRun {
				if err := os.Remove(dstFile); err != nil {
					return NewPathError("restore", dstFile, fmt.Errorf("removing incorrect symlink: %w", err))
				}
			}
		}

		if !pathExists(srcFile) && pathExists(dstFile) {
			m.logf("Adopting file %s -> %s", dstFile, srcFile)

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
			m.logVerbosef("Source file does not exist: %s", srcFile)
			continue
		}

		if pathExists(dstFile) && !isSymlink(dstFile) {
			m.logf("Removing file %s", dstFile)

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

		m.logf("Creating symlink %s -> %s", dstFile, srcFile)

		if !m.DryRun {
			if err := createSymlink(srcFile, dstFile, subEntry.Sudo); err != nil {
				return NewPathError("restore", dstFile, fmt.Errorf("creating symlink: %w", err))
			}
		}
	}

	return nil
}
