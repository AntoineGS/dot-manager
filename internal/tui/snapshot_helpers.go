// Package tui provides the terminal user interface.
package tui

import (
	"regexp"
	"strings"
)

// ansiRegex matches ANSI escape sequences for color and formatting
var ansiRegex = regexp.MustCompile(`\x1b\[[0-9;]*[a-zA-Z]`) //nolint:unused // Will be used in snapshot tests

// stripAnsiCodes removes all ANSI escape sequences from a string,
// leaving only the plain text content.
func stripAnsiCodes(s string) string { //nolint:unused // Will be used in snapshot tests
	return ansiRegex.ReplaceAllString(s, "")
}

// normalizeOutput normalizes terminal output for consistent comparison:
// - Trims trailing whitespace from each line
// - Removes trailing empty lines
// - Ensures consistent line endings (LF)
func normalizeOutput(s string) string { //nolint:unused // Will be used in snapshot tests
	// Split into lines
	lines := strings.Split(s, "\n")

	// Trim trailing whitespace from each line
	// (terminal width differences shouldn't break tests)
	for i, line := range lines {
		lines[i] = strings.TrimRight(line, " \t")
	}

	// Remove trailing empty lines
	for len(lines) > 0 && lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}

	// Rejoin with consistent line endings
	return strings.Join(lines, "\n")
}
