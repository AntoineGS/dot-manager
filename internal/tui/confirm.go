package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
		m.Screen = ScreenPathSelect
	}
	return m, nil
}

func (m Model) viewConfirm() string {
	var b strings.Builder

	// Title
	icon := "󰁯"
	if m.Operation == OpBackup {
		icon = "󰆓"
	}
	title := fmt.Sprintf("%s  Confirm %s", icon, m.Operation.String())
	b.WriteString(TitleStyle.Render(title))
	b.WriteString("\n\n")

	// Count selected
	selected := 0
	for _, p := range m.Paths {
		if p.Selected {
			selected++
		}
	}

	// Warning for dry run
	if m.DryRun {
		b.WriteString(WarningStyle.Render("⚠ DRY RUN MODE - No changes will be made"))
		b.WriteString("\n\n")
	}

	// Summary
	action := "create symlinks for"
	if m.Operation == OpBackup {
		action = "backup"
	}

	summary := fmt.Sprintf("You are about to %s %d path(s):", action, selected)
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

		for _, item := range m.Paths {
			if !item.Selected {
				continue
			}

			var success bool
			var message string

			if m.Operation == OpRestore {
				success, message = m.performRestore(item)
			} else {
				success, message = m.performBackup(item)
			}

			results = append(results, ResultItem{
				Name:    item.Spec.Name,
				Success: success,
				Message: message,
			})
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

	sourceExists := fileExists(source)
	targetExists := fileExists(target)
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

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
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

		srcExists := fileExists(srcFile)
		dstExists := fileExists(dstFile)

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

func (m Model) performBackup(item PathItem) (bool, string) {
	backupPath := m.resolvePath(item.Spec.Backup)

	if item.Spec.IsFolder() {
		return m.backupFolder(item.Target, backupPath)
	}
	return m.backupFiles(item.Spec.Files, item.Target, backupPath)
}

func (m Model) backupFolder(source, backup string) (bool, string) {
	if m.DryRun {
		return true, fmt.Sprintf("Would copy folder to: %s", backup)
	}

	// Check source exists
	if _, err := os.Stat(source); os.IsNotExist(err) {
		return false, fmt.Sprintf("Source does not exist: %s", source)
	}

	// Skip symlinks
	if info, err := os.Lstat(source); err == nil {
		if info.Mode()&os.ModeSymlink != 0 {
			return true, "Skipped (is a symlink)"
		}
	}

	// Would copy here - for safety, just report what would happen
	return true, fmt.Sprintf("Would copy folder to: %s", backup)
}

func (m Model) backupFiles(files []string, source, backup string) (bool, string) {
	if m.DryRun {
		return true, fmt.Sprintf("Would copy %d file(s) to: %s", len(files), backup)
	}

	copied := 0
	skipped := 0

	for _, file := range files {
		srcFile := filepath.Join(source, file)

		// Check source exists
		if _, err := os.Stat(srcFile); os.IsNotExist(err) {
			skipped++
			continue
		}

		// Skip symlinks
		if info, err := os.Lstat(srcFile); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				skipped++
				continue
			}
		}

		copied++
	}

	msg := fmt.Sprintf("Would copy %d file(s)", copied)
	if skipped > 0 {
		msg += fmt.Sprintf(", %d skipped", skipped)
	}
	return true, msg
}
