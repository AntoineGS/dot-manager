package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/bubbles/filepicker"
	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
)

// updateFileAddModeChoice handles key events for the Browse/Type mode selection menu
func (m Model) updateFileAddModeChoice(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.subEntryForm == nil {
		return m, nil
	}

	if m, cmd, handled := m.handleCommonKeys(msg); handled {
		return m, cmd
	}

	switch {
	case key.Matches(msg, ModeChooserKeys.Cancel):
		// Cancel mode selection and return to files list
		m.subEntryForm.addFileMode = ModeNone
		m.subEntryForm.modeMenuCursor = 0
		return m, nil

	case key.Matches(msg, ModeChooserKeys.Up):
		// Move up with wrapping (0=Browse target, 1=Browse source, 2=Type)
		m.subEntryForm.modeMenuCursor--
		if m.subEntryForm.modeMenuCursor < 0 {
			m.subEntryForm.modeMenuCursor = 2
		}
		return m, nil

	case key.Matches(msg, ModeChooserKeys.Down):
		// Move down with wrapping
		m.subEntryForm.modeMenuCursor++
		if m.subEntryForm.modeMenuCursor > 2 {
			m.subEntryForm.modeMenuCursor = 0
		}
		return m, nil

	case key.Matches(msg, ModeChooserKeys.Select):
		// Select the current option
		switch m.subEntryForm.modeMenuCursor {
		case 0:
			// Browse target directory - transition to ModePicker
			if err := m.initFilePicker(); err != nil {
				m.subEntryForm.err = fmt.Sprintf("failed to initialize file picker: %v", err)
				m.subEntryForm.addFileMode = ModeNone
				return m, nil
			}
			m.subEntryForm.addFileMode = ModePicker
		case 1:
			// Browse source directory - transition to ModePicker starting at backup path
			if err := m.initFilePickerForBackup(); err != nil {
				m.subEntryForm.err = fmt.Sprintf("failed to initialize file picker: %v", err)
				m.subEntryForm.addFileMode = ModeNone
				return m, nil
			}
			m.subEntryForm.addFileMode = ModePicker
		case 2:
			// Type Path - transition to ModeTextInput
			m.subEntryForm.addFileMode = ModeTextInput
			m.subEntryForm.addingFile = true
			m.subEntryForm.newFileInput.SetValue("")
			m.subEntryForm.newFileInput.Focus()
		}
		return m, nil
	}

	return m, nil
}

// viewFileAddModeMenu renders the Browse/Type choice menu
func (m Model) viewFileAddModeMenu() string {
	if m.subEntryForm == nil {
		return ""
	}

	var b strings.Builder

	// Title
	b.WriteString(TitleStyle.Render("  Choose how to add file:"))
	b.WriteString("\n\n")

	// Menu options
	options := []string{
		"Browse target directory (pick from target path)",
		"Browse source directory (pick from backup path)",
		"Type Path (enter manually)",
	}

	for i, text := range options {
		if m.subEntryForm.modeMenuCursor == i {
			b.WriteString(fmt.Sprintf("  %s\n", SelectedMenuItemStyle.Render("→ "+text)))
		} else {
			b.WriteString(fmt.Sprintf("    %s\n", text))
		}
	}

	b.WriteString("\n")

	// Help
	b.WriteString(RenderHelpFromBindings(m.width,
		ModeChooserKeys.Select,
		ModeChooserKeys.Cancel,
	))

	return BaseStyle.Render(b.String())
}

// initFilePicker initializes the file picker with the appropriate start directory
func (m *Model) initFilePicker() error {
	if m.subEntryForm == nil {
		return fmt.Errorf("subEntryForm is nil")
	}

	// Get the target path for the current OS
	var targetPath string
	switch m.Platform.OS {
	case OSLinux:
		targetPath = m.subEntryForm.linuxTargetInput.Value()
	case OSWindows:
		targetPath = m.subEntryForm.windowsTargetInput.Value()
	default:
		targetPath = m.subEntryForm.linuxTargetInput.Value()
	}

	// Resolve the start directory using phase 2 utility
	startDir, err := resolvePickerStartDirectory(targetPath, m.Platform.OS)
	if err != nil {
		return fmt.Errorf("failed to resolve start directory: %w", err)
	}

	// Initialize the file picker
	picker := filepicker.New()
	picker.CurrentDirectory = startDir
	picker.DirAllowed = true
	picker.FileAllowed = true
	picker.ShowHidden = true

	m.subEntryForm.filePicker = picker

	return nil
}

// initFilePickerForBackup initializes the file picker starting at the backup/source directory
func (m *Model) initFilePickerForBackup() error {
	if m.subEntryForm == nil {
		return fmt.Errorf("subEntryForm is nil")
	}

	// Get the backup path and resolve it relative to the config directory
	backupPath := m.subEntryForm.backupInput.Value()

	var startDir string
	if backupPath == "" {
		// No backup path set, fall back to config directory
		if m.ConfigPath != "" {
			startDir = filepath.Dir(m.ConfigPath)
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return fmt.Errorf("failed to get home directory: %w", err)
			}
			startDir = home
		}
	} else {
		// Resolve backup path relative to the config directory
		configDir := filepath.Dir(m.ConfigPath)
		resolvedPath := backupPath
		if !filepath.IsAbs(backupPath) {
			resolvedPath = filepath.Join(configDir, backupPath)
		}

		var err error
		startDir, err = resolvePickerStartDirectory(resolvedPath, m.Platform.OS)
		if err != nil {
			return fmt.Errorf("failed to resolve start directory: %w", err)
		}
	}

	// Initialize the file picker
	picker := filepicker.New()
	picker.CurrentDirectory = startDir
	picker.DirAllowed = true
	picker.FileAllowed = true
	picker.ShowHidden = true

	m.subEntryForm.filePicker = picker

	return nil
}

// updateSubEntryFilePicker handles key events when the file picker is active
func (m Model) updateSubEntryFilePicker(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	if m.subEntryForm == nil {
		return m, nil
	}

	var cmd tea.Cmd

	if m, cmd, handled := m.handleCommonKeys(msg); handled {
		return m, cmd
	}

	switch {
	case key.Matches(msg, FilePickerKeys.Cancel):
		// Cancel file picker, clear selections, and return to files list
		m.subEntryForm.addFileMode = ModeNone
		m.subEntryForm.selectedFiles = make(map[string]bool)
		return m, nil

	case key.Matches(msg, FilePickerKeys.Toggle):
		// Toggle selection of current file/directory
		// The selectedFiles map tracks absolute paths of files marked for addition
		// When user presses space/tab, we add or remove the current file from this map
		currentPath := filepath.Join(
			m.subEntryForm.filePicker.CurrentDirectory,
			m.subEntryForm.filePicker.Path,
		)

		if currentPath == "" || m.subEntryForm.filePicker.Path == "" {
			// No valid selection (e.g., on ".." or empty path)
			// Pass the key through to the file picker for normal navigation
			m.subEntryForm.filePicker, cmd = m.subEntryForm.filePicker.Update(msg)
			return m, cmd
		}

		// Toggle selection: if already selected, remove it; otherwise add it
		// This allows users to build up a multi-selection before confirming
		if m.subEntryForm.selectedFiles[currentPath] {
			delete(m.subEntryForm.selectedFiles, currentPath)
		} else {
			m.subEntryForm.selectedFiles[currentPath] = true
		}

		return m, nil

	case key.Matches(msg, FilePickerKeys.Confirm):
		// Confirm file selections and add them to the files list
		// This is the final step of the file picker workflow: take all selected
		// absolute paths, convert them to relative paths (relative to target directory),
		// and add them to the config entry's files list

		// Get the target directory for the current OS
		// This is where the config will be symlinked to, and serves as the base
		// for converting absolute file paths to relative paths
		var targetPath string
		switch m.Platform.OS {
		case OSLinux:
			targetPath = m.subEntryForm.linuxTargetInput.Value()
		case OSWindows:
			targetPath = m.subEntryForm.windowsTargetInput.Value()
		default:
			targetPath = m.subEntryForm.linuxTargetInput.Value()
		}

		// Expand target path to absolute (resolve ~ and env vars)
		// This is required for accurate relative path calculation
		expandedTarget, err := expandTargetPath(targetPath)
		if err != nil {
			m.subEntryForm.err = fmt.Sprintf("failed to expand target path: %v", err)
			m.subEntryForm.addFileMode = ModeNone
			m.subEntryForm.selectedFiles = make(map[string]bool)
			return m, nil
		}

		// Process all selected files (if any)
		if len(m.subEntryForm.selectedFiles) > 0 {
			// Collect all selected paths from the map
			// selectedFiles uses absolute paths as keys for accurate tracking
			selectedPaths := make([]string, 0, len(m.subEntryForm.selectedFiles))
			for path := range m.subEntryForm.selectedFiles {
				selectedPaths = append(selectedPaths, path)
			}

			// Convert all absolute paths to relative paths
			// Files must be relative to the target directory to work in the config
			relativePaths, errs := convertToRelativePaths(selectedPaths, expandedTarget)

			// Add all successfully converted paths to files list
			// Skip any that failed conversion (e.g., outside target directory)
			addedCount := 0
			for i, relativePath := range relativePaths {
				if errs[i] == nil && relativePath != "" {
					m.subEntryForm.files = append(m.subEntryForm.files, relativePath)
					addedCount++
				}
			}

			// Clear selections for next use
			m.subEntryForm.selectedFiles = make(map[string]bool)

			// Move cursor to "Add File" button for convenience
			m.subEntryForm.filesCursor = len(m.subEntryForm.files)

			// Set success message to show user feedback
			if addedCount > 0 {
				m.subEntryForm.successMessage = fmt.Sprintf("Added %d file(s)", addedCount)
			}

			// Exit picker mode and return to files list
			m.subEntryForm.addFileMode = ModeNone
			return m, nil
		}

		// No selections - just cancel and return to files list
		m.subEntryForm.addFileMode = ModeNone
		return m, nil
	}

	// Update the file picker with the key message
	m.subEntryForm.filePicker, cmd = m.subEntryForm.filePicker.Update(msg)

	return m, cmd
}

// viewFilePicker renders the file picker interface
// renderStyledFilePicker renders the file picker with selection styling.
//
// This function parses the raw file picker view and applies visual styling to indicate:
// - Cursor position (darker purple): The currently focused file/directory
// - Selected files (lighter purple): Files selected for addition via space/tab
// - Unselected files (no styling): Regular files
//
// The parsing logic extracts file names from each line of the picker output and builds
// full paths by joining with currentDir. These paths are checked against selectedFiles
// map to determine if styling should be applied.
func (m Model) renderStyledFilePicker() string {
	if m.subEntryForm == nil {
		return ""
	}

	// Get the raw picker view from the underlying file picker component
	rawView := m.subEntryForm.filePicker.View()
	lines := strings.Split(rawView, "\n")

	var styledLines []string
	currentDir := m.subEntryForm.filePicker.CurrentDirectory

	for _, line := range lines {
		// Skip empty lines (preserve spacing)
		if strings.TrimSpace(line) == "" {
			styledLines = append(styledLines, line)
			continue
		}

		// Extract the file name from the line
		// File picker lines typically look like: "  filename" or "> filename"
		// The ">" prefix indicates the cursor position
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			styledLines = append(styledLines, line)
			continue
		}

		// Check if this line is the cursor position (starts with ">")
		isCursor := strings.HasPrefix(trimmed, ">")
		if isCursor {
			// Remove cursor prefix to get actual filename
			trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, ">"))
		}

		// Build full path for this file by joining current directory with filename
		// This is used to look up selection state in selectedFiles map
		var fullPath string
		if trimmed == ".." {
			// Parent directory marker - don't apply selection styling
			styledLines = append(styledLines, line)
			continue
		}
		fullPath = filepath.Join(currentDir, trimmed)

		// Check if this file is in the selectedFiles map (user pressed space/tab on it)
		isSelected := m.subEntryForm.selectedFiles[fullPath]

		// Apply styling based on cursor and selection state
		// Priority: cursor styling (darker purple) > selected styling (lighter purple) > no styling
		switch {
		case isCursor:
			// Cursor position uses SelectedMenuItemStyle (darker purple #7C3AED)
			// This is the "active" file that would be selected if user presses space
			styledLines = append(styledLines, SelectedMenuItemStyle.Render(line))
		case isSelected:
			// Selected files use SelectedRowStyle (lighter purple #9F7AEA)
			// These are files that have been marked for addition
			styledLines = append(styledLines, SelectedRowStyle.Render(line))
		default:
			// Unselected files remain unstyled
			styledLines = append(styledLines, line)
		}
	}

	return strings.Join(styledLines, "\n")
}

func (m Model) viewFilePicker() string {
	if m.subEntryForm == nil {
		return ""
	}

	var b strings.Builder

	// Header with current directory
	currentDir := m.subEntryForm.filePicker.CurrentDirectory
	if currentDir == "" {
		currentDir = "/"
	}
	b.WriteString(TitleStyle.Render("  Select Files"))
	b.WriteString("\n")
	b.WriteString(SubtitleStyle.Render(currentDir))
	b.WriteString("\n\n")

	// Show the file picker with styled selected rows
	pickerView := m.renderStyledFilePicker()
	b.WriteString(pickerView)
	b.WriteString("\n\n")

	// Selection count
	selectionCount := len(m.subEntryForm.selectedFiles)
	countText := fmt.Sprintf("%d file(s) selected", selectionCount)
	b.WriteString(MutedTextStyle.Render(countText))
	b.WriteString("\n\n")

	// Help
	b.WriteString(RenderHelpFromBindings(m.width,
		FilePickerKeys.Toggle,
		FilePickerKeys.Confirm,
		FilePickerKeys.Cancel,
	))

	return BaseStyle.Render(b.String())
}
