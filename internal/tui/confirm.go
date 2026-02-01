package tui

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/AntoineGS/dot-manager/internal/config"
	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", "enter":
		m.Screen = ScreenProgress
		m.processing = true
		m.results = nil
		return m, m.startOperation()
	case "n", "N", "esc":
		if m.Operation == OpInstallPackages {
			m.Screen = ScreenPackageSelect
		} else {
			m.Screen = ScreenPathSelect
		}
	}
	return m, nil
}

func (m Model) viewConfirm() string {
	var b strings.Builder

	// Title
	icon := "󰁯"
	switch m.Operation {
	case OpInstallPackages:
		icon = "󰏖"
	}
	title := fmt.Sprintf("%s  Confirm %s", icon, m.Operation.String())
	b.WriteString(TitleStyle.Render(title))
	b.WriteString("\n\n")

	// Warning for dry run
	if m.DryRun {
		b.WriteString(WarningStyle.Render("⚠ DRY RUN MODE - No changes will be made"))
		b.WriteString("\n\n")
	}

	// Handle packages differently
	if m.Operation == OpInstallPackages {
		// Count selected packages
		selected := 0
		for _, pkg := range m.Packages {
			if pkg.Selected {
				selected++
			}
		}

		summary := fmt.Sprintf("You are about to install %d package(s):", selected)
		b.WriteString(summary)
		b.WriteString("\n\n")

		// List selected packages (up to 10)
		count := 0
		for _, pkg := range m.Packages {
			if pkg.Selected {
				count++
				if count <= 10 {
					marker := CheckedStyle.Render("  ✓ ")
					methodInfo := SubtitleStyle.Render(fmt.Sprintf(" (%s)", pkg.Method))
					b.WriteString(marker + pkg.Spec.Name + methodInfo)
					b.WriteString("\n")
				}
			}
		}
		if count > 10 {
			b.WriteString(SubtitleStyle.Render(fmt.Sprintf("  ... and %d more", count-10)))
			b.WriteString("\n")
		}
	} else {
		// Count selected paths
		selected := 0
		for _, p := range m.Paths {
			if p.Selected {
				selected++
			}
		}

		// Summary
		summary := fmt.Sprintf("You are about to create symlinks for %d path(s):", selected)
		b.WriteString(summary)
		b.WriteString("\n\n")

		// List selected paths (up to 10)
		count := 0
		for _, item := range m.Paths {
			if item.Selected {
				count++
				if count <= 10 {
					marker := CheckedStyle.Render("  ✓ ")
					b.WriteString(marker + item.Spec.Name)
					b.WriteString("\n")
				}
			}
		}
		if count > 10 {
			b.WriteString(SubtitleStyle.Render(fmt.Sprintf("  ... and %d more", count-10)))
			b.WriteString("\n")
		}
	}

	// Confirmation prompt
	b.WriteString("\n")
	box := BoxStyle.Render("Proceed with " + m.Operation.String() + "?  " +
		HelpKeyStyle.Render("y") + "/yes  " +
		HelpKeyStyle.Render("n") + "/no")
	b.WriteString(box)

	// Help
	b.WriteString("\n")
	b.WriteString(RenderHelp(
		"y/enter", "confirm",
		"n/esc", "cancel",
	))

	return BaseStyle.Render(b.String())
}

func (m Model) startOperation() tea.Cmd {
	return func() tea.Msg {
		var results []ResultItem

		if m.Operation == OpInstallPackages {
			// Handle package installation
			for _, pkg := range m.Packages {
				if !pkg.Selected {
					continue
				}

				success, message := m.performPackageInstall(pkg)
				results = append(results, ResultItem{
					Name:    pkg.Spec.Name,
					Success: success,
					Message: message,
				})
			}
		} else {
			// Handle path operations (restore)
			for _, item := range m.Paths {
				if !item.Selected {
					continue
				}

				success, message := m.performRestore(item)

				results = append(results, ResultItem{
					Name:    item.Spec.Name,
					Success: success,
					Message: message,
				})
			}
		}

		return OperationCompleteMsg{
			Results: results,
			Err:     nil,
		}
	}
}

func (m Model) performRestore(item PathItem) (bool, string) {
	backupPath := m.resolvePath(item.Spec.Backup)

	if item.Spec.IsFolder() {
		return m.restoreFolder(backupPath, item.Target)
	}
	return m.restoreFiles(item.Spec.Files, backupPath, item.Target)
}

func (m Model) restoreFolder(source, target string) (bool, string) {
	// Check if already a symlink
	if info, err := os.Lstat(target); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return true, "Already a symlink"
		}
	}

	sourceExists := pathExists(source)
	targetExists := pathExists(target)
	adopted := false

	// Check if we need to adopt: target exists but backup doesn't
	if !sourceExists && targetExists {
		if m.DryRun {
			return true, fmt.Sprintf("Would adopt: %s → %s, then create symlink", target, source)
		}

		// Create backup parent directory
		backupParent := filepath.Dir(source)
		if _, err := os.Stat(backupParent); os.IsNotExist(err) {
			if err := os.MkdirAll(backupParent, 0755); err != nil {
				return false, fmt.Sprintf("Failed to create backup directory: %v", err)
			}
		}

		// Move target to backup location
		if err := os.Rename(target, source); err != nil {
			return false, fmt.Sprintf("Failed to adopt (move to backup): %v", err)
		}
		adopted = true
		sourceExists = true
	}

	if m.DryRun {
		return true, fmt.Sprintf("Would create symlink: %s → %s", target, source)
	}

	// Check if source exists now
	if !sourceExists {
		return false, fmt.Sprintf("Source does not exist: %s", source)
	}

	// Create parent directory if needed
	parentDir := filepath.Dir(target)
	if _, err := os.Stat(parentDir); os.IsNotExist(err) {
		if err := os.MkdirAll(parentDir, 0755); err != nil {
			return false, fmt.Sprintf("Failed to create directory: %v", err)
		}
	}

	// Remove existing (if still there)
	if info, err := os.Lstat(target); err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			if err := os.RemoveAll(target); err != nil {
				return false, fmt.Sprintf("Failed to remove existing: %v", err)
			}
		}
	}

	// Create symlink
	if err := os.Symlink(source, target); err != nil {
		return false, fmt.Sprintf("Failed to create symlink: %v", err)
	}

	if adopted {
		return true, fmt.Sprintf("Adopted and linked: %s → %s", target, source)
	}
	return true, fmt.Sprintf("Created symlink: %s → %s", target, source)
}

func (m Model) restoreFiles(files []string, source, target string) (bool, string) {
	// Create backup directory if needed (for adopting)
	if _, err := os.Stat(source); os.IsNotExist(err) {
		if !m.DryRun {
			if err := os.MkdirAll(source, 0755); err != nil {
				return false, fmt.Sprintf("Failed to create backup directory: %v", err)
			}
		}
	}

	// Create target directory if needed
	if _, err := os.Stat(target); os.IsNotExist(err) {
		if !m.DryRun {
			if err := os.MkdirAll(target, 0755); err != nil {
				return false, fmt.Sprintf("Failed to create directory: %v", err)
			}
		}
	}

	created := 0
	skipped := 0
	adopted := 0
	var lastErr string

	for _, file := range files {
		srcFile := filepath.Join(source, file)
		dstFile := filepath.Join(target, file)

		// Check if already a symlink
		if info, err := os.Lstat(dstFile); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				skipped++
				continue
			}
		}

		srcExists := pathExists(srcFile)
		dstExists := pathExists(dstFile)

		// Check if we need to adopt: target exists but backup doesn't
		if !srcExists && dstExists {
			if m.DryRun {
				adopted++
				continue
			}

			// Move target file to backup location
			if err := os.Rename(dstFile, srcFile); err != nil {
				// If rename fails (cross-device), try copy and delete
				if err := copyFileSimple(dstFile, srcFile); err != nil {
					lastErr = fmt.Sprintf("Failed to adopt %s: %v", file, err)
					continue
				}
				if err := os.Remove(dstFile); err != nil {
					lastErr = fmt.Sprintf("Failed to remove original %s: %v", file, err)
					continue
				}
			}
			adopted++
			srcExists = true
		}

		if !srcExists {
			skipped++
			continue
		}

		if m.DryRun {
			created++
			continue
		}

		// Remove existing (if still there)
		if info, err := os.Lstat(dstFile); err == nil {
			if info.Mode()&os.ModeSymlink == 0 {
				if err := os.Remove(dstFile); err != nil {
					lastErr = fmt.Sprintf("Failed to remove %s: %v", file, err)
					continue
				}
			}
		}

		// Create symlink
		if err := os.Symlink(srcFile, dstFile); err != nil {
			lastErr = fmt.Sprintf("Failed to symlink %s: %v", file, err)
			continue
		}
		created++
	}

	if lastErr != "" {
		return false, lastErr
	}

	if m.DryRun {
		msg := fmt.Sprintf("Would create %d symlink(s)", created)
		if adopted > 0 {
			msg += fmt.Sprintf(", adopt %d", adopted)
		}
		if skipped > 0 {
			msg += fmt.Sprintf(", skip %d", skipped)
		}
		return true, msg
	}

	msg := fmt.Sprintf("Created %d symlink(s)", created)
	if adopted > 0 {
		msg += fmt.Sprintf(", adopted %d", adopted)
	}
	if skipped > 0 {
		msg += fmt.Sprintf(", skipped %d", skipped)
	}
	return true, msg
}

func copyFileSimple(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	info, err := os.Stat(src)
	if err != nil {
		return err
	}

	return os.WriteFile(dst, data, info.Mode())
}

func (m Model) performPackageInstall(pkg PackageItem) (bool, string) {
	if m.DryRun {
		return true, fmt.Sprintf("Would install via %s", pkg.Method)
	}

	// Try package managers first
	if pkgName, ok := pkg.Spec.Managers[pkg.Method]; ok {
		return m.installWithManager(pkg.Method, pkgName)
	}

	// Try custom command
	if cmd, ok := pkg.Spec.Custom[m.Platform.OS]; ok {
		return m.runCustomCommand(cmd)
	}

	// Try URL install
	if urlSpec, ok := pkg.Spec.URL[m.Platform.OS]; ok {
		return m.installFromURL(urlSpec)
	}

	return false, "No installation method available"
}

func (m Model) installWithManager(manager, pkgName string) (bool, string) {
	var cmd *exec.Cmd

	switch manager {
	case "pacman":
		cmd = exec.Command("sudo", "pacman", "-S", "--noconfirm", pkgName)
	case "yay":
		cmd = exec.Command("yay", "-S", "--noconfirm", pkgName)
	case "paru":
		cmd = exec.Command("paru", "-S", "--noconfirm", pkgName)
	case "apt":
		cmd = exec.Command("sudo", "apt-get", "install", "-y", pkgName)
	case "dnf":
		cmd = exec.Command("sudo", "dnf", "install", "-y", pkgName)
	case "brew":
		cmd = exec.Command("brew", "install", pkgName)
	case "winget":
		cmd = exec.Command("winget", "install", "--accept-package-agreements", "--accept-source-agreements", pkgName)
	case "scoop":
		cmd = exec.Command("scoop", "install", pkgName)
	case "choco":
		cmd = exec.Command("choco", "install", "-y", pkgName)
	default:
		return false, fmt.Sprintf("Unknown package manager: %s", manager)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return false, fmt.Sprintf("Installation failed: %v", err)
	}

	return true, fmt.Sprintf("Installed via %s", manager)
}

func (m Model) runCustomCommand(command string) (bool, string) {
	var cmd *exec.Cmd
	if m.Platform.OS == "windows" {
		cmd = exec.Command("powershell", "-Command", command)
	} else {
		cmd = exec.Command("sh", "-c", command)
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin

	if err := cmd.Run(); err != nil {
		return false, fmt.Sprintf("Custom command failed: %v", err)
	}

	return true, "Installed via custom command"
}

func (m Model) installFromURL(urlSpec config.URLInstallSpec) (bool, string) {
	// Create temp file
	tmpFile, err := os.CreateTemp("", "dot-manager-*")
	if err != nil {
		return false, fmt.Sprintf("Failed to create temp file: %v", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// Download file
	var downloadCmd *exec.Cmd
	if m.Platform.OS == "windows" {
		downloadCmd = exec.Command("powershell", "-Command",
			fmt.Sprintf("Invoke-WebRequest -Uri '%s' -OutFile '%s'", urlSpec.URL, tmpPath))
	} else {
		downloadCmd = exec.Command("curl", "-fsSL", "-o", tmpPath, urlSpec.URL)
	}

	if err := downloadCmd.Run(); err != nil {
		return false, fmt.Sprintf("Download failed: %v", err)
	}

	// Make executable on Unix
	if m.Platform.OS != "windows" {
		os.Chmod(tmpPath, 0755)
	}

	// Run install command
	command := strings.ReplaceAll(urlSpec.Command, "{file}", tmpPath)

	var installCmd *exec.Cmd
	if m.Platform.OS == "windows" {
		installCmd = exec.Command("powershell", "-Command", command)
	} else {
		installCmd = exec.Command("sh", "-c", command)
	}

	installCmd.Stdout = os.Stdout
	installCmd.Stderr = os.Stderr
	installCmd.Stdin = os.Stdin

	if err := installCmd.Run(); err != nil {
		return false, fmt.Sprintf("Install command failed: %v", err)
	}

	return true, "Installed via URL"
}
