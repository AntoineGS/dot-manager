package tui

import (
	"fmt"
	"strings"

	"charm.land/bubbles/v2/key"
	"github.com/AntoineGS/tidydots/internal/config"
)

const setupEmptyValue = "(empty)"

var (
	subEntryCheckModeToggleKey = key.NewBinding(
		key.WithKeys("space", "enter"),
		key.WithHelp("space/enter", "toggle"),
	)
	subEntryCheckModeExitCodeKey = key.NewBinding(
		key.WithKeys("left", "h"),
		key.WithHelp("←/h", "exit-code"),
	)
	subEntryCheckModeStatusKey = key.NewBinding(
		key.WithKeys("right", "l"),
		key.WithHelp("→/l", "status"),
	)
)

func (m Model) renderSubEntryToggleValue(field subEntryFieldType, value string) string {
	if m.getSubEntryFieldType() == field {
		return SelectedMenuItemStyle.Render(value)
	}
	return value
}

func (m Model) renderSubEntrySetupFields() string {
	var b strings.Builder
	fields := []struct {
		field subEntryFieldType
		label string
		empty string
	}{
		{subFieldLinuxCheck, "Check (linux):", setupEmptyValue},
		{subFieldLinuxRun, "Run (linux):", setupEmptyValue},
		{subFieldWindowsCheck, "Check (windows):", setupEmptyValue},
		{subFieldWindowsRun, "Run (windows):", setupEmptyValue},
	}
	for _, field := range fields {
		label := field.label
		if m.getSubEntryFieldType() == field.field {
			label = HelpKeyStyle.Render(label)
		}
		fmt.Fprintf(&b, "  %s\n  %s\n\n", label, m.renderSubEntryFieldValue(field.field, field.empty))
	}

	value := config.CheckModeExitCode
	if m.subEntryForm.CheckMode == config.CheckModeStatus {
		value = config.CheckModeStatus
	}
	fmt.Fprintf(&b, "  Check mode:\n  %s\n\n",
		m.renderSubEntryToggleValue(subFieldCheckMode, value))

	rootLabel := "Root only:"
	if m.getSubEntryFieldType() == subFieldIsSudo {
		rootLabel = HelpKeyStyle.Render(rootLabel)
	}
	rootCheck := CheckboxUnchecked
	if m.subEntryForm.IsSudo {
		rootCheck = CheckboxChecked
	}
	fmt.Fprintf(&b, "  %s  %s Yes\n\n", rootLabel, rootCheck)

	return b.String()
}
