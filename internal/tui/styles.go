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

	// BaseStyle is the base style with padding for content.
	BaseStyle = lipgloss.NewStyle().
			Padding(1, 2)

	// TitleStyle is the main title style with border and bold text.
	TitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(primaryColor).
			MarginBottom(1).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor)

	// SubtitleStyle is the subtitle style with muted color and italic text.
	SubtitleStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true).
			MarginBottom(1)

	// MutedTextStyle is inline muted text (no margins, for use within lines).
	MutedTextStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// MenuItemStyle is the default menu item style.
	MenuItemStyle = lipgloss.NewStyle().
			Padding(0, 2)

	// SelectedMenuItemStyle is the style for selected menu items with highlighted background.
	SelectedMenuItemStyle = lipgloss.NewStyle().
				Foreground(textColor).
				Background(primaryColor).
				Bold(true).
				Padding(0, 2)

	// ListItemStyle is the default list item style.
	ListItemStyle = lipgloss.NewStyle()

	// SelectedListItemStyle is the style for selected list items with highlighted background.
	SelectedListItemStyle = lipgloss.NewStyle().
				Foreground(textColor).
				Background(primaryColor).
				Bold(true)

	// CheckedStyle is the style for checked checkboxes.
	CheckedStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	// UncheckedStyle is the style for unchecked checkboxes.
	UncheckedStyle = lipgloss.NewStyle().
			Foreground(mutedColor)

	// PathNameStyle is the style for path names with bold text.
	PathNameStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Bold(true)

	// PathTargetStyle is the style for path target locations with muted italic text.
	PathTargetStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			Italic(true)

	// PathBackupStyle is the style for path backup locations.
	PathBackupStyle = lipgloss.NewStyle().
			Foreground(accentColor)

	// FolderBadgeStyle is the badge style for folder indicators.
	FolderBadgeStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000")).
				Background(accentColor).
				Padding(0, 1).
				MarginLeft(1)

	// StateBadgeReadyStyle is the badge style for ready state (green background).
	StateBadgeReadyStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000")).
				Background(secondaryColor). // Green
				Padding(0, 1).
				MarginLeft(1)

	// StateBadgeAdoptStyle is the badge style for adopt state (amber background).
	StateBadgeAdoptStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#000")).
				Background(accentColor). // Amber
				Padding(0, 1).
				MarginLeft(1)

	// StateBadgeMissingStyle is the badge style for missing state (red background).
	StateBadgeMissingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#fff")).
				Background(errorColor). // Red
				Padding(0, 1).
				MarginLeft(1)

	// StateBadgeLinkedStyle is the badge style for linked state (muted text).
	StateBadgeLinkedStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Padding(0, 1).
				MarginLeft(1)

	// StateBadgeFilteredStyle is the badge style for filtered state (same as linked - muted).
	StateBadgeFilteredStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Padding(0, 1).
				MarginLeft(1)

	// StateBadgeInstalledStyle is the badge style for installed state (muted, like linked).
	StateBadgeInstalledStyle = lipgloss.NewStyle().
					Foreground(mutedColor).
					Padding(0, 1).
					MarginLeft(1)

	// ProgressStyle is the style for progress indicators.
	ProgressStyle = lipgloss.NewStyle().
			Foreground(secondaryColor)

	// SuccessStyle is the style for success messages with bold green text.
	SuccessStyle = lipgloss.NewStyle().
			Foreground(secondaryColor).
			Bold(true)

	// ErrorStyle is the style for error messages with bold red text.
	ErrorStyle = lipgloss.NewStyle().
			Foreground(errorColor).
			Bold(true)

	// WarningStyle is the style for warning messages with amber text.
	WarningStyle = lipgloss.NewStyle().
			Foreground(accentColor)

	// BoxStyle is the default box style with rounded border.
	BoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(primaryColor).
			Padding(1, 2).
			MarginTop(1)

	// ResultBoxStyle is the box style for result displays with green border.
	ResultBoxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(secondaryColor).
			Padding(1, 2).
			MarginTop(1)

	// HelpStyle is the style for help text.
	HelpStyle = lipgloss.NewStyle().
			Foreground(mutedColor).
			MarginTop(1)

	// HelpKeyStyle is the style for help key bindings with bold amber text.
	HelpKeyStyle = lipgloss.NewStyle().
			Foreground(accentColor).
			Bold(true)

	// StatusBarStyle is the style for the status bar.
	StatusBarStyle = lipgloss.NewStyle().
			Foreground(textColor).
			Background(lipgloss.Color("#1F2937")).
			Padding(0, 1).
			MarginTop(1)

	// SpinnerStyle is the style for loading spinners.
	SpinnerStyle = lipgloss.NewStyle().
			Foreground(primaryColor)

	// FilterInputStyle is the style for filter input fields.
	FilterInputStyle = lipgloss.NewStyle().
				Foreground(textColor).
				Background(lipgloss.Color("#1F2937")).
				Padding(0, 1)

	// FilterHighlightStyle is the style for highlighted filter matches with amber background.
	FilterHighlightStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Bold(true)

	// MultiSelectBannerStyle is the style for the multi-select banner showing selection counts.
	MultiSelectBannerStyle = lipgloss.NewStyle().
				Foreground(mutedColor).
				Bold(true).
				Padding(0, 2).
				MarginTop(1).
				MarginBottom(1)

	// SelectedRowStyle is the style for rows that are selected in multi-select mode.
	// Uses a lighter purple background to differentiate from cursor highlight.
	SelectedRowStyle = lipgloss.NewStyle().
				Foreground(textColor).
				Background(lipgloss.Color("#9F7AEA")). // Lighter purple
				Padding(0, 1)
)

// RenderHelp renders help text with key bindings, wrapping to 80 characters.
// It takes alternating key and description strings and formats them with styling.
func RenderHelp(keys ...string) string {
	return RenderHelpWithWidth(80, keys...)
}

// RenderHelpWithWidth renders help text with key bindings, wrapping to the specified width.
// It takes a width and alternating key and description strings, formatting them with styling.
// For single-character keys (except "q"), it highlights the matching letter within the description.
// For "q" and multi-character keys, it renders them separately as "key desc".
func RenderHelpWithWidth(width int, keys ...string) string {
	if width < 20 {
		width = 20
	}

	var lines []string
	var lineTexts []string    // Styled text for current line
	var currentVisibleLen int // Track visible length separately
	separator := "  "
	separatorLen := len(separator)

	for i := 0; i < len(keys); i += 2 {
		key := keys[i]

		desc := ""
		if i+1 < len(keys) {
			desc = keys[i+1]
		}

		var itemText string
		var itemLen int

		// Special handling: if key is a single character and appears in desc,
		// highlight it within the description
		if len(key) == 1 {
			// Find the first occurrence of the key character in the description (case-insensitive)
			keyRune := []rune(key)[0]
			descRunes := []rune(desc)
			found := false

			for j, r := range descRunes {
				if r == keyRune || r == keyRune-32 || r == keyRune+32 {
					// Found the key character - highlight it using the key's case
					before := string(descRunes[:j])
					highlighted := HelpKeyStyle.Render(key) // Use key's case, not matched character
					after := string(descRunes[j+1:])
					itemText = before + highlighted + after
					itemLen = len(desc) // Visual length is just the description length
					found = true
					break
				}
			}

			if !found {
				// Fallback: render as separate key and description
				itemText = HelpKeyStyle.Render(key) + " " + desc
				itemLen = len(key) + 1 + len(desc)
			}
		} else {
			// For "q" and multi-character keys, render as separate key and description
			itemText = HelpKeyStyle.Render(key) + " " + desc
			itemLen = len(key) + 1 + len(desc)
		}

		// Check if adding this item would exceed width
		willExceed := len(lineTexts) > 0 && currentVisibleLen+separatorLen+itemLen > width-4

		if willExceed {
			// Wrap to new line
			lines = append(lines, joinLineTexts(lineTexts, separator))
			lineTexts = []string{itemText}
			currentVisibleLen = itemLen
		} else {
			lineTexts = append(lineTexts, itemText)
			if len(lineTexts) > 1 {
				currentVisibleLen += separatorLen
			}
			currentVisibleLen += itemLen
		}
	}

	// Add remaining items
	if len(lineTexts) > 0 {
		lines = append(lines, joinLineTexts(lineTexts, separator))
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

// joinLineTexts joins styled text items with a separator.
func joinLineTexts(items []string, separator string) string {
	result := ""
	for i, item := range items {
		if i > 0 {
			result += separator
		}
		result += item
	}
	return result
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
		return CheckedStyle.Render(CheckboxChecked)
	}

	return UncheckedStyle.Render(CheckboxUnchecked)
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
func RenderOSInfo(osName string, isArch, dryRun bool) string {
	osInfo := "OS: " + osName
	if isArch {
		osInfo += " • Arch Linux"
	}

	if dryRun {
		osInfo += " • " + WarningStyle.Render("DRY RUN")
	}

	return SubtitleStyle.Render(osInfo)
}
