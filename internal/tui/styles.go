package tui

import (
	"github.com/charmbracelet/lipgloss"
)

var (
	// Colors
	primaryColor   = lipgloss.Color("#7C3AED") // Purple
	secondaryColor = lipgloss.Color("#10B981") // Green
	accentColor    = lipgloss.Color("#F59E0B") // Amber
	errorColor     = lipgloss.Color("#EF4444") // Red
	mutedColor     = lipgloss.Color("#6B7280") // Gray
	textColor      = lipgloss.Color("#F3F4F6") // Light gray

	// Base styles
	BaseStyle = lipgloss.NewStyle().
			Padding(1, 2)

	// Title styles
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor)

	SubtitleStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true).
			MarginBottom(1)

	// Inline muted text (no margins, for use within lines)
	MutedTextStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Menu styles
	MenuItemStyle = lipgloss.NewStyle().
			Padding(0, 2)

	SelectedMenuItemStyle = lipgloss.NewStyle().
				Foreground(textColor).
				Background(primaryColor).
				Bold(true).
				Padding(0, 2)

	// List styles
	ListItemStyle = lipgloss.NewStyle().
			PaddingLeft(2)

	SelectedListItemStyle = lipgloss.NewStyle().
				Foreground(primaryColor).
				Bold(true)

	CheckedStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	UncheckedStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// Path info styles
	PathNameStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Bold(true)

	PathTargetStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true)

	PathBackupStyle = lipgloss.NewStyle().
			Foreground(accentColor)

	FolderBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000")).
				Background(accentColor).
				Padding(0, 1).
				MarginLeft(1)

	// Path state badge styles
	StateBadgeReadyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000")).
				Background(secondaryColor). // Green
				Padding(0, 1).
				MarginLeft(1)

	StateBadgeAdoptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000")).
				Background(accentColor). // Amber
				Padding(0, 1).
				MarginLeft(1)

	StateBadgeMissingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#fff")).
				Background(errorColor). // Red
				Padding(0, 1).
				MarginLeft(1)

	StateBadgeLinkedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#fff")).
				Background(primaryColor). // Purple
				Padding(0, 1).
				MarginLeft(1)

	// Progress styles
	ProgressStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	SuccessStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	ErrorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	WarningStyle = lipgloss.NewStyle().
			Foreground(accentColor)

	// Box styles
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2).
			MarginTop(1)

	ResultBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(secondaryColor).
			Padding(1, 2).
			MarginTop(1)

	// Help styles
	HelpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1)

	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(primaryColor).
			Bold(true)

	// Status bar
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(lipgloss.Color("#1F2937")).
			Padding(0, 1).
			MarginTop(1)

	// Spinner
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(primaryColor)
)

func RenderHelp(keys ...string) string {
	return RenderHelpWithWidth(80, keys...)
}

func RenderHelpWithWidth(width int, keys ...string) string {
	if width < 20 {
		width = 20
	}

	var lines []string
	var currentLine string
	separator := "  "

	for i := 0; i < len(keys); i += 2 {
		key := keys[i]
		desc := ""
		if i+1 < len(keys) {
			desc = keys[i+1]
		}

		// Calculate the visual length of this item (without ANSI codes)
		itemText := key + " " + desc
		itemLen := len(itemText)

		// Calculate current line length (approximate, ignoring ANSI)
		currentLen := 0
		if currentLine != "" {
			// Count visible characters roughly
			for _, r := range currentLine {
				if r != '\x1b' {
					currentLen++
				}
			}
			// Rough estimate: subtract ANSI overhead per item
			currentLen = len(currentLine) / 2
		}

		// Check if adding this item would exceed width
		if currentLine != "" && currentLen+len(separator)+itemLen > width-4 {
			// Wrap to new line
			lines = append(lines, currentLine)
			currentLine = HelpKeyStyle.Render(key) + " " + desc
		} else {
			if currentLine != "" {
				currentLine += separator
			}
			currentLine += HelpKeyStyle.Render(key) + " " + desc
		}
	}

	if currentLine != "" {
		lines = append(lines, currentLine)
	}

	// Join lines and apply help style
	result := ""
	for i, line := range lines {
		if i > 0 {
			result += "\n"
		}
		result += line
	}

	return HelpStyle.Render(result)
}

// RenderCursor renders a cursor indicator for list items
func RenderCursor(isSelected bool) string {
	if isSelected {
		return "> "
	}
	return "  "
}

// RenderCheckbox renders a checkbox indicator
func RenderCheckbox(isChecked bool) string {
	if isChecked {
		return CheckedStyle.Render("[✓]")
	}
	return UncheckedStyle.Render("[ ]")
}

// RenderScrollIndicators renders scroll indicators if needed
// Returns (topIndicator, bottomIndicator) strings with newlines included
func RenderScrollIndicators(start, end, total int) (string, string) {
	var top, bottom string

	if start > 0 {
		top = SubtitleStyle.Render("  ↑ more above") + "\n"
	}
	if end < total {
		bottom = SubtitleStyle.Render("  ↓ more below") + "\n"
	}

	return top, bottom
}

// CalculateVisibleRange calculates the visible range for a scrollable list
func CalculateVisibleRange(offset, viewHeight, total int) (start, end int) {
	start = offset
	end = offset + viewHeight
	if end > total {
		end = total
	}
	return start, end
}

// RenderOSInfo renders the OS information subtitle
func RenderOSInfo(osName string, isRoot, isArch, dryRun bool) string {
	osInfo := "OS: " + osName
	if isRoot {
		osInfo += " (root)"
	}
	if isArch {
		osInfo += " • Arch Linux"
	}
	if dryRun {
		osInfo += " • " + WarningStyle.Render("DRY RUN")
	}
	return SubtitleStyle.Render(osInfo)
}
