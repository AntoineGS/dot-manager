package manager

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/AntoineGS/dot-manager/internal/config"
)

func (m *Manager) Restore() error {
	m.log("Restoring configurations for OS: %s (root: %v)", m.Platform.OS, m.Platform.IsRoot)

	// Restore config entries (symlinks)
	paths := m.GetPaths()

	for _, path := range paths {
		target := path.GetTarget(m.Platform.OS)
		if target == "" {
			m.logVerbose("Skipping %s: no target for OS %s", path.Name, m.Platform.OS)
			continue
		}

		if err := m.restorePath(path, target); err != nil {
			m.log("Error restoring %s: %v", path.Name, err)
		}
	}

	// Restore git entries (clones)
	gitEntries := m.Config.GetGitEntries(m.Platform.IsRoot)
	for _, entry := range gitEntries {
		target := entry.GetTarget(m.Platform.OS)
		if target == "" {
			m.logVerbose("Skipping git entry %s: no target for OS %s", entry.Name, m.Platform.OS)
			continue
		}

		if err := m.restoreGitEntry(entry, target); err != nil {
			m.log("Error restoring git entry %s: %v", entry.Name, err)
		}
	}

	return nil
}

func (m *Manager) restorePath(spec config.PathSpec, target string) error {
	backupPath := m.resolvePath(spec.Backup)

	if spec.IsFolder() {
		return m.restoreFolder(spec.Name, backupPath, target)
	}
	return m.restoreFiles(spec.Name, spec.Files, backupPath, target)
}

func (m *Manager) restoreFolder(name, source, target string) error {
	// Skip if already a symlink
	if isSymlink(target) {
		m.logVerbose("Already a symlink: %s", target)
		return nil
	}

	// Check if we need to adopt: target exists but backup doesn't
	if !pathExists(source) && pathExists(target) {
		m.log("Adopting folder %s -> %s", target, source)
		if !m.DryRun {
			// Create backup parent directory
			backupParent := filepath.Dir(source)
			if !pathExists(backupParent) {
				if err := os.MkdirAll(backupParent, 0755); err != nil {
					return fmt.Errorf("creating backup parent directory: %w", err)
				}
			}
			// Move target to backup location
			if err := os.Rename(target, source); err != nil {
				return fmt.Errorf("adopting folder (moving to backup): %w", err)
			}
		}
	}

	// Now check if backup exists
	if !pathExists(source) {
		m.logVerbose("Source folder does not exist: %s", source)
		return nil
	}

	// Create parent directory if it doesn't exist
	parentDir := filepath.Dir(target)
	if !pathExists(parentDir) {
		m.log("Creating directory %s", parentDir)
		if !m.DryRun {
			if err := os.MkdirAll(parentDir, 0755); err != nil {
				return fmt.Errorf("creating parent directory: %w", err)
			}
		}
	}

	// Remove existing folder (if still there after adopt check)
	if pathExists(target) && !isSymlink(target) {
		m.log("Removing folder %s", target)
		if !m.DryRun {
			if err := removeAll(target); err != nil {
				return fmt.Errorf("removing existing folder: %w", err)
			}
		}
	}

	m.log("Creating symlink %s -> %s", target, source)
	if !m.DryRun {
		return createSymlink(source, target)
	}
	return nil
}

func (m *Manager) restoreFiles(name string, files []string, source, target string) error {
	// Create backup directory if it doesn't exist (needed for adopting)
	if !pathExists(source) {
		if !m.DryRun {
			if err := os.MkdirAll(source, 0755); err != nil {
				return fmt.Errorf("creating backup directory: %w", err)
			}
		}
	}

	// Create target directory if it doesn't exist
	if !pathExists(target) {
		m.log("Creating directory %s", target)
		if !m.DryRun {
			if err := os.MkdirAll(target, 0755); err != nil {
				return fmt.Errorf("creating target directory: %w", err)
			}
		}
	}

	for _, file := range files {
		srcFile := filepath.Join(source, file)
		dstFile := filepath.Join(target, file)

		// Skip if already a symlink
		if isSymlink(dstFile) {
			m.logVerbose("Already a symlink: %s", dstFile)
			continue
		}

		// Check if we need to adopt: target exists but backup doesn't
		if !pathExists(srcFile) && pathExists(dstFile) {
			m.log("Adopting file %s -> %s", dstFile, srcFile)
			if !m.DryRun {
				// Move target file to backup location
				if err := os.Rename(dstFile, srcFile); err != nil {
					// If rename fails (cross-device), try copy and delete
					if err := copyFile(dstFile, srcFile); err != nil {
						return fmt.Errorf("adopting file (copying to backup): %w", err)
					}
					if err := os.Remove(dstFile); err != nil {
						return fmt.Errorf("adopting file (removing original): %w", err)
					}
				}
			}
		}

		if !pathExists(srcFile) {
			m.logVerbose("Source file does not exist: %s", srcFile)
			continue
		}

		// Remove existing file (if still there after adopt check)
		if pathExists(dstFile) && !isSymlink(dstFile) {
			m.log("Removing file %s", dstFile)
			if !m.DryRun {
				if err := os.Remove(dstFile); err != nil {
					return fmt.Errorf("removing existing file: %w", err)
				}
			}
		}

		m.log("Creating symlink %s -> %s", dstFile, srcFile)
		if !m.DryRun {
			if err := createSymlink(srcFile, dstFile); err != nil {
				return fmt.Errorf("creating symlink: %w", err)
			}
		}
	}

	return nil
}

func createSymlink(source, target string) error {
	if runtime.GOOS == "windows" {
		// Check if source is a directory
		info, err := os.Stat(source)
		if err != nil {
			return err
		}

		if info.IsDir() {
			// Use mklink /J for directory junctions on Windows
			cmd := exec.Command("cmd", "/c", "mklink", "/J", target, source)
			return cmd.Run()
		}
		// Use mklink for files
		cmd := exec.Command("cmd", "/c", "mklink", target, source)
		return cmd.Run()
	}

	return os.Symlink(source, target)
}

// restoreGitEntry clones or updates a git repository
func (m *Manager) restoreGitEntry(entry config.Entry, target string) error {
	if pathExists(target) {
		// Check if it's a git repository
		gitDir := filepath.Join(target, ".git")
		if pathExists(gitDir) {
			m.log("Updating git repo %s at %s...", entry.Name, target)
			if !m.DryRun {
				var cmd *exec.Cmd
				if entry.Root {
					cmd = exec.Command("sudo", "git", "-C", target, "pull")
				} else {
					cmd = exec.Command("git", "-C", target, "pull")
				}
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					return fmt.Errorf("failed to update %s: %w", entry.Name, err)
				}
				m.log("[ok] %s updated successfully", entry.Name)
			}
			return nil
		}
		// Target exists but is not a git repo - skip
		m.logVerbose("Target %s exists but is not a git repository, skipping", target)
		return nil
	}

	// Clone the repository
	m.log("Cloning %s to %s...", entry.Name, target)
	if !m.DryRun {
		parentDir := filepath.Dir(target)
		if !pathExists(parentDir) {
			if entry.Root {
				mkdirCmd := exec.Command("sudo", "mkdir", "-p", parentDir)
				if err := mkdirCmd.Run(); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", parentDir, err)
				}
			} else {
				if err := os.MkdirAll(parentDir, 0755); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", parentDir, err)
				}
			}
		}

		args := []string{"clone"}
		if entry.Branch != "" {
			args = append(args, "-b", entry.Branch)
		}
		args = append(args, entry.Repo, target)

		var cmd *exec.Cmd
		if entry.Root {
			cmd = exec.Command("sudo", append([]string{"git"}, args...)...)
		} else {
			cmd = exec.Command("git", args...)
		}
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			return fmt.Errorf("failed to clone %s: %w", entry.Name, err)
		}
		m.log("[ok] %s cloned successfully", entry.Name)
	}

	return nil
}
