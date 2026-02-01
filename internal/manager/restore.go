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

	if err := m.runPostRestoreHooks(); err != nil {
		m.log("Error running post-restore hooks: %v", err)
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

func (m *Manager) runPostRestoreHooks() error {
	hooks, ok := m.Config.Hooks.PostRestore[m.Platform.OS]
	if !ok {
		return nil
	}

	for _, hook := range hooks {
		if hook.SkipOnArch && m.Platform.IsArch {
			m.log("Skipping hook %s on Arch Linux (managed by pacman)", hook.Type)
			continue
		}

		switch hook.Type {
		case "zsh-plugins":
			if err := m.runZshPluginsHook(hook); err != nil {
				m.log("Error running zsh-plugins hook: %v", err)
			}
		case "ghostty-terminfo":
			if err := m.runGhosttyTerminfoHook(hook); err != nil {
				m.log("Error running ghostty-terminfo hook: %v", err)
			}
		default:
			m.logVerbose("Unknown hook type: %s", hook.Type)
		}
	}

	return nil
}

func (m *Manager) runZshPluginsHook(hook config.Hook) error {
	m.log("Setting up zsh plugins...")

	for _, plugin := range hook.Plugins {
		if pathExists(plugin.Path) {
			m.log("Updating %s at %s...", plugin.Name, plugin.Path)
			if !m.DryRun {
				cmd := exec.Command("sudo", "git", "-C", plugin.Path, "pull")
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					m.log("Failed to update %s: %v", plugin.Name, err)
				} else {
					m.log("✓ %s updated successfully", plugin.Name)
				}
			}
		} else {
			m.log("Cloning %s to %s...", plugin.Name, plugin.Path)
			if !m.DryRun {
				parentDir := filepath.Dir(plugin.Path)
				if !pathExists(parentDir) {
					exec.Command("sudo", "mkdir", "-p", parentDir).Run()
				}

				cmd := exec.Command("sudo", "git", "clone", plugin.Repo, plugin.Path)
				cmd.Stdout = os.Stdout
				cmd.Stderr = os.Stderr
				if err := cmd.Run(); err != nil {
					m.log("Failed to clone %s: %v", plugin.Name, err)
				} else {
					m.log("✓ %s cloned successfully", plugin.Name)
				}
			}
		}

		// Handle fzf symlinks
		if plugin.Name == "fzf" {
			for _, sl := range hook.FzfSymlinks {
				target := filepath.Join(plugin.Path, sl.Target)
				link := filepath.Join(plugin.Path, sl.Link)

				if pathExists(target) && !pathExists(link) {
					m.log("Creating symlink %s -> %s", link, target)
					if !m.DryRun {
						cmd := exec.Command("sudo", "ln", "-s", target, link)
						if err := cmd.Run(); err != nil {
							m.log("Failed to create symlink for %s: %v", sl.Link, err)
						} else {
							m.log("✓ Created symlink %s", link)
						}
					}
				}
			}
		}
	}

	return nil
}

func (m *Manager) runGhosttyTerminfoHook(hook config.Hook) error {
	source := m.resolvePath(hook.Source)

	if !pathExists(source) {
		m.log("Ghostty terminfo source not found: %s - skipping", source)
		return nil
	}

	m.log("Installing ghostty terminfo...")

	if !m.DryRun {
		home, _ := os.UserHomeDir()
		terminfo := filepath.Join(home, ".terminfo")

		cmd := exec.Command("tic", "-x", "-o", terminfo, source)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		if err := cmd.Run(); err != nil {
			m.log("✗ Failed to install ghostty terminfo: %v", err)
			return err
		}
		m.log("✓ Ghostty terminfo installed successfully")
	}

	return nil
}
