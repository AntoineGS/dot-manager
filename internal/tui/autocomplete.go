package tui

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const maxSuggestions = 8

// getPathSuggestions returns directory suggestions based on the current input
func getPathSuggestions(input string, configDir string) []string {
	if input == "" {
		return nil
	}

	// Expand ~ to home directory
	expandedPath := expandPath(input)

	// Handle relative paths (for backup field)
	if strings.HasPrefix(input, "./") || strings.HasPrefix(input, "../") {
		expandedPath = filepath.Join(configDir, input)
	}

	// Get the directory to list and the prefix to match
	dir := expandedPath
	prefix := ""

	// If path doesn't end with separator, treat last component as prefix
	if !strings.HasSuffix(input, string(os.PathSeparator)) && !strings.HasSuffix(input, "/") {
		dir = filepath.Dir(expandedPath)
		prefix = filepath.Base(expandedPath)
	}

	// Read directory
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}

	var suggestions []string
	for _, entry := range entries {
		name := entry.Name()

		// Skip hidden files unless user is explicitly looking for them
		if strings.HasPrefix(name, ".") && !strings.HasPrefix(prefix, ".") {
			continue
		}

		// Match prefix (case-insensitive)
		if prefix != "" && !strings.HasPrefix(strings.ToLower(name), strings.ToLower(prefix)) {
			continue
		}

		// Build the suggestion path
		suggestion := buildSuggestionPath(input, name, entry.IsDir())
		suggestions = append(suggestions, suggestion)
	}

	// Sort first, then truncate to get alphabetically first entries
	sort.Strings(suggestions)
	if len(suggestions) > maxSuggestions {
		suggestions = suggestions[:maxSuggestions]
	}
	return suggestions
}

// buildSuggestionPath constructs the suggestion maintaining the original path style
func buildSuggestionPath(originalInput, name string, isDir bool) string {
	// Find the directory part of the original input
	var dirPart string

	if strings.HasSuffix(originalInput, "/") || strings.HasSuffix(originalInput, string(os.PathSeparator)) {
		dirPart = originalInput
	} else {
		// Get everything up to and including the last separator
		lastSep := strings.LastIndexAny(originalInput, "/\\")
		if lastSep >= 0 {
			dirPart = originalInput[:lastSep+1]
		} else {
			dirPart = ""
		}
	}

	result := dirPart + name
	if isDir {
		result += "/"
	}

	return result
}

// expandPath expands ~ to the home directory
func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return filepath.Join(home, path[2:])
	}
	if path == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return path
		}
		return home
	}
	return path
}
