package manager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/AntoineGS/dot-manager/internal/config"
)

func (m *Manager) Backup() error {
	m.log("Backing up configurations for OS: %s", m.Platform.OS)

	if m.Config.Version == 3 {
		return m.backupV3()
	}

	// v2 format - existing logic
	entries := m.GetEntries()

	for _, entry := range entries {
		target := entry.GetTarget(m.Platform.OS)
		if target == "" {
			m.logVerbose("Skipping %s: no target for OS %s", entry.Name, m.Platform.OS)
			continue
		}

		if err := m.backupEntry(entry, target); err != nil {
			m.log("Error backing up %s: %v", entry.Name, err)
		}
	}

	return nil
}

func (m *Manager) backupV3() error {
	apps := m.GetApplications()

	for _, app := range apps {
		m.log("Backing up application: %s", app.Name)

		for _, subEntry := range app.Entries {
			if !subEntry.IsConfig() {
				m.logVerbose("Skipping %s/%s: git entries don't need backup", app.Name, subEntry.Name)
				continue
			}

			target := subEntry.GetTarget(m.Platform.OS)
			if target == "" {
				m.logVerbose("Skipping %s/%s: no target for OS %s", app.Name, subEntry.Name, m.Platform.OS)
				continue
			}

			if err := m.backupSubEntry(app.Name, subEntry, target); err != nil {
				m.log("Error backing up %s/%s: %v", app.Name, subEntry.Name, err)
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

func (m *Manager) backupFolderSubEntry(appName string, subEntry config.SubEntry, backup, target string) error {
	// Similar to existing backupFolder logic
	if !pathExists(target) {
		m.logVerbose("Target folder does not exist: %s", target)
		return nil
	}

	m.log("Backing up folder %s -> %s", target, backup)
	if !m.DryRun {
		if err := os.MkdirAll(filepath.Dir(backup), 0755); err != nil {
			return NewPathError("backup", backup, fmt.Errorf("creating parent directory: %w", err))
		}

		// Copy source folder into backup directory (e.g., /source/nvim -> /backup/nvim)
		destPath := filepath.Join(backup, filepath.Base(target))
		if subEntry.Sudo {
			cmd := exec.Command("sudo", "cp", "-r", target, destPath)
			return cmd.Run()
		}
		return copyDir(target, destPath)
	}
	return nil
}

func (m *Manager) backupFilesSubEntry(appName string, subEntry config.SubEntry, backup, target string) error {
	// Similar to existing backupFiles logic
	if !pathExists(target) {
		m.logVerbose("Target directory does not exist: %s", target)
		return nil
	}

	if !m.DryRun {
		if err := os.MkdirAll(backup, 0755); err != nil {
			return NewPathError("backup", backup, fmt.Errorf("creating backup directory: %w", err))
		}
	}

	for _, file := range subEntry.Files {
		srcFile := filepath.Join(target, file)
		dstFile := filepath.Join(backup, file)

		if !pathExists(srcFile) {
			m.logVerbose("Source file does not exist: %s", srcFile)
			continue
		}

		m.log("Backing up file %s -> %s", srcFile, dstFile)
		if !m.DryRun {
			if subEntry.Sudo {
				cmd := exec.Command("sudo", "cp", srcFile, dstFile)
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

func (m *Manager) backupEntry(entry config.Entry, source string) error {
	backupPath := m.resolvePath(entry.Backup)

	if entry.IsFolder() {
		return m.backupFolder(entry.Name, source, backupPath)
	}
	return m.backupFiles(entry.Name, entry.Files, source, backupPath)
}

func (m *Manager) backupFolder(name, source, backup string) error {
	if !pathExists(source) {
		m.logVerbose("Source folder does not exist: %s", source)
		return nil
	}

	// Skip symlinks - they point to our backup already
	if isSymlink(source) {
		m.logVerbose("Skipping symlink: %s", source)
		return nil
	}

	m.log("Backing up folder %s to %s", source, backup)
	if !m.DryRun {
		// Copy source folder into backup directory (e.g., /source/config -> /backup/config)
		destPath := filepath.Join(backup, filepath.Base(source))
		if err := copyDir(source, destPath); err != nil {
			return NewPathError("backup", source, fmt.Errorf("copying folder: %w", err))
		}
	}
	return nil
}

func (m *Manager) backupFiles(name string, files []string, source, backup string) error {
	for _, file := range files {
		srcFile := filepath.Join(source, file)
		dstFile := filepath.Join(backup, file)

		if !pathExists(srcFile) {
			m.logVerbose("Source file does not exist: %s", srcFile)
			continue
		}

		// Skip symlinks
		if isSymlink(srcFile) {
			m.logVerbose("Skipping symlink: %s", srcFile)
			continue
		}

		m.log("Backing up file %s to %s", srcFile, dstFile)
		if !m.DryRun {
			if err := copyFile(srcFile, dstFile); err != nil {
				return NewPathError("backup", srcFile, fmt.Errorf("copying file: %w", err))
			}
		}
	}

	return nil
}
