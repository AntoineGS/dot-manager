package tui

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m Model) updateConfirm(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "y", "Y", KeyEnter:
		m.Screen = ScreenProgress
		m.processing = true
		m.results = nil

		if m.Operation == OpInstallPackages {
			// Initialize package installation state
			m.pendingPackages = nil
			m.currentPackageIndex = 0

			for _, pkg := range m.Packages {
				if pkg.Selected {
					m.pendingPackages = append(m.pendingPackages, pkg)
				}
			}

			if len(m.pendingPackages) == 0 {
				return m, func() tea.Msg {
					return OperationCompleteMsg{Results: nil, Err: nil}
				}
			}

			// Start installing the first package
			return m, m.installNextPackage()
		}

		return m, m.startOperation()
	case "n", "N", KeyEsc:
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
	case OpRestore, OpRestoreDryRun, OpAdd, OpList:
		// Use default icon
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
					b.WriteString(marker + pkg.Entry.Name + methodInfo)
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
		var summary string
		switch m.Operation {
		case OpRestoreDryRun:
			summary = fmt.Sprintf("Preview restore for %d selected paths?\n\nNo changes will be made to your filesystem.", selected)
		case OpRestore:
			summary = fmt.Sprintf("Restore %d selected paths?", selected)
		case OpAdd, OpList, OpInstallPackages:
			summary = fmt.Sprintf("You are about to %s %d path(s):", m.Operation.String(), selected)
		default:
			summary = fmt.Sprintf("You are about to %s %d path(s):", m.Operation.String(), selected)
		}
		b.WriteString(summary)
		b.WriteString("\n\n")

		// List selected paths (up to 10)
		count := 0

		for _, item := range m.Paths {
			if item.Selected {
				count++
				if count <= 10 {
					marker := CheckedStyle.Render("  ✓ ")
					b.WriteString(marker + item.Entry.Name)
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
	b.WriteString(RenderHelpWithWidth(m.width,
		"y/enter", "confirm",
		"n/esc", "cancel",
	))

	return BaseStyle.Render(b.String())
}

func (m Model) startOperation() tea.Cmd {
	// Handle path operations (restore) - these don't need sudo
	return func() tea.Msg {
		var results []ResultItem

		for _, item := range m.Paths {
			if !item.Selected {
				continue
			}

			success, message := m.performRestore(item)

			results = append(results, ResultItem{
				Name:    item.Entry.Name,
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

func (m Model) installNextPackage() tea.Cmd {
	if m.currentPackageIndex >= len(m.pendingPackages) {
		return nil
	}

	pkg := m.pendingPackages[m.currentPackageIndex]

	// Handle dry run
	if m.DryRun {
		return func() tea.Msg {
			return PackageInstallMsg{
				Package: pkg,
				Success: true,
				Message: fmt.Sprintf("Would install via %s", pkg.Method),
			}
		}
	}

	// Build the command
	cmd := m.buildInstallCommand(pkg)
	if cmd == nil {
		return func() tea.Msg {
			return PackageInstallMsg{
				Package: pkg,
				Success: false,
				Message: "No installation method available",
			}
		}
	}

	// Use tea.ExecProcess to properly suspend the TUI and give terminal control to the command
	// This allows sudo to prompt for password correctly
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		if err != nil {
			return PackageInstallMsg{
				Package: pkg,
				Success: false,
				Message: fmt.Sprintf("Installation failed: %v", err),
				Err:     err,
			}
		}

		return PackageInstallMsg{
			Package: pkg,
			Success: true,
			Message: fmt.Sprintf("Installed via %s", pkg.Method),
		}
	})
}

func (m Model) buildInstallCommand(pkg PackageItem) *exec.Cmd {
	if pkg.Entry.Package == nil {
		return nil
	}

	// Handle git package manager specially
	if pkg.Method == "git" {
		if gitPkg, ok := pkg.Entry.Package.GetGitPackage(); ok {
			target := gitPkg.Targets[m.Platform.OS]
			if target == "" {
				return nil
			}

			// Build git clone command with optional branch and sudo support
			args := []string{"clone"}
			if gitPkg.Branch != "" {
				args = append(args, "-b", gitPkg.Branch)
			}
			args = append(args, gitPkg.URL, target)

			if gitPkg.Sudo {
				// Prepend sudo to git command
				args = append([]string{"git"}, args...)
				return exec.CommandContext(context.Background(), "sudo", args...) //nolint:gosec // intentional command
			}

			return exec.CommandContext(context.Background(), "git", args...) //nolint:gosec // intentional command
		}
	}

	// Try package managers first
	if pkgName, ok := pkg.Entry.Package.GetManagerString(pkg.Method); ok {
		switch pkg.Method {
		case "pacman":
			return exec.CommandContext(context.Background(), "sudo", "pacman", "-S", "--noconfirm", pkgName) //nolint:gosec // intentional command
		case "yay":
			return exec.CommandContext(context.Background(), "yay", "-S", "--noconfirm", pkgName) //nolint:gosec // intentional command
		case "paru":
			return exec.CommandContext(context.Background(), "paru", "-S", "--noconfirm", pkgName) //nolint:gosec // intentional command
		case "apt":
			return exec.CommandContext(context.Background(), "sudo", "apt-get", "install", "-y", pkgName) //nolint:gosec // intentional command
		case "dnf":
			return exec.CommandContext(context.Background(), "sudo", "dnf", "install", "-y", pkgName) //nolint:gosec // intentional command
		case "brew":
			return exec.CommandContext(context.Background(), "brew", "install", pkgName) //nolint:gosec // intentional command
		case "winget":
			return exec.CommandContext(context.Background(), "winget", "install", "--accept-package-agreements", "--accept-source-agreements", pkgName) //nolint:gosec // intentional command
		case "scoop":
			return exec.CommandContext(context.Background(), "scoop", "install", pkgName) //nolint:gosec // intentional command
		case "choco":
			return exec.CommandContext(context.Background(), "choco", "install", "-y", pkgName) //nolint:gosec // intentional command
		}
	}

	// Try custom command
	if pkg.Method == "custom" {
		if customCmd, ok := pkg.Entry.Package.Custom[m.Platform.OS]; ok {
			if m.Platform.OS == "windows" {
				return exec.CommandContext(context.Background(), "powershell", "-Command", customCmd) //nolint:gosec // intentional command
			}

			return exec.CommandContext(context.Background(), "sh", "-c", customCmd) //nolint:gosec // intentional command
		}
	}

	// Try URL install - wrap download + install in a single shell command
	if pkg.Method == "url" {
		if urlSpec, ok := pkg.Entry.Package.URL[m.Platform.OS]; ok {
			if m.Platform.OS == "windows" {
				// PowerShell: download to temp, run install command
				script := fmt.Sprintf(`
					$tmpFile = [System.IO.Path]::GetTempFileName()
					Invoke-WebRequest -Uri '%s' -OutFile $tmpFile
					$command = '%s' -replace '\{file\}', $tmpFile
					Invoke-Expression $command
					Remove-Item $tmpFile -ErrorAction SilentlyContinue
				`, urlSpec.URL, urlSpec.Command)

				return exec.CommandContext(context.Background(), "powershell", "-Command", script) //nolint:gosec // intentional command
			}
			// Unix: download to temp, chmod, run install command, cleanup
			script := fmt.Sprintf(`
				tmpfile=$(mktemp)
				trap "rm -f $tmpfile" EXIT
				curl -fsSL -o "$tmpfile" '%s' && \
				chmod +x "$tmpfile" && \
				%s
			`, urlSpec.URL, strings.ReplaceAll(urlSpec.Command, "{file}", "$tmpfile"))

			return exec.CommandContext(context.Background(), "sh", "-c", script) //nolint:gosec // intentional command
		}
	}

	return nil
}

func (m Model) performRestore(item PathItem) (bool, string) {
	backupPath := m.resolvePath(item.Entry.Backup)

	// Use Manager for actual restore operation
	var err error
	if item.Entry.IsFolder() {
		err = m.Manager.RestoreFolder(item.Entry, backupPath, item.Target)
	} else {
		err = m.Manager.RestoreFiles(item.Entry, backupPath, item.Target)
	}

	if err != nil {
		return false, fmt.Sprintf("Failed: %v", err)
	}

	return true, fmt.Sprintf("Restored: %s → %s", item.Target, backupPath)
}
