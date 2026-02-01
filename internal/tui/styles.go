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
				Bold(true).
				PaddingLeft(2)

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
	var result string
	for i := 0; i < len(keys); i += 2 {
		if i > 0 {
			result += "  "
		}
		key := keys[i]
		desc := ""
		if i+1 < len(keys) {
			desc = keys[i+1]
		}
		result += HelpKeyStyle.Render(key) + " " + desc
	}
	return HelpStyle.Render(result)
}
