package tui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// expandTargetPath expands ~ and resolves to absolute path.
// It handles:
// - Empty strings (returned as-is)
// - Tilde expansion (~, ~/path)
// - Relative paths (converted to absolute)
// - Absolute paths (returned as-is)
// - Environment variables (expanded)
func expandTargetPath(targetPath string) (string, error) {
	if targetPath == "" {
		return "", nil
	}

	// Expand ~ to home directory
	if strings.HasPrefix(targetPath, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		targetPath = filepath.Join(home, targetPath[2:])
	} else if targetPath == "~" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		targetPath = home
	}

	// Expand environment variables
	targetPath = os.ExpandEnv(targetPath)

	// Convert to absolute path
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to resolve absolute path: %w", err)
	}

	return absPath, nil
}

// findNearestExistingParent walks up the directory tree to find the first existing parent.
// If the path itself exists, it is returned.
// If no parent exists (reaches root), the root directory is returned.
func findNearestExistingParent(path string) string {
	// Check if path exists
	if _, err := os.Stat(path); err == nil {
		return path
	}

	// Walk up the directory tree
	current := path
	for {
		parent := filepath.Dir(current)

		// Check if we've reached the root
		if parent == current {
			// We're at root, return it
			return parent
		}

		// Check if parent exists
		if _, err := os.Stat(parent); err == nil {
			return parent
		}

		current = parent
	}
}

// resolvePickerStartDirectory determines the starting directory for the file picker.
// It follows this fallback chain:
// 1. If targetPath is empty, return home directory
// 2. Expand targetPath with expandTargetPath()
// 3. Check if expanded path exists
// 4. If not, use findNearestExistingParent()
// The currentOS parameter is reserved for future OS-specific logic.
func resolvePickerStartDirectory(targetPath, _ string) (string, error) {
	// If empty, use home directory
	if targetPath == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get home directory: %w", err)
		}
		return home, nil
	}

	// Expand the target path
	expanded, err := expandTargetPath(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to expand target path: %w", err)
	}

	// Check if expanded path exists
	if info, err := os.Stat(expanded); err == nil {
		// If it's a file, use its directory
		if !info.IsDir() {
			return filepath.Dir(expanded), nil
		}
		return expanded, nil
	}

	// Path doesn't exist, find nearest existing parent
	nearest := findNearestExistingParent(expanded)

	return nearest, nil
}

// convertToRelativePaths converts absolute paths to relative paths relative to targetDir.
// Returns:
// - A slice of relative paths (same length as absPaths)
// - A slice of errors (same length as absPaths, nil for successful conversions)
//
// Special cases:
// - If absPath equals targetDir, returns "."
// - If absPath is outside targetDir, returns empty string with error
func convertToRelativePaths(absPaths []string, targetDir string) ([]string, []error) {
	relativePaths := make([]string, len(absPaths))
	errors := make([]error, len(absPaths))

	// Clean and make targetDir absolute
	targetDir = filepath.Clean(targetDir)
	if !filepath.IsAbs(targetDir) {
		var err error
		targetDir, err = filepath.Abs(targetDir)
		if err != nil {
			// If we can't resolve targetDir, all conversions fail
			for i := range errors {
				errors[i] = fmt.Errorf("failed to resolve target directory: %w", err)
			}
			return relativePaths, errors
		}
	}

	for i, absPath := range absPaths {
		// Clean the path
		absPath = filepath.Clean(absPath)

		// Special case: selecting target directory itself
		if absPath == targetDir {
			relativePaths[i] = "."
			errors[i] = nil
			continue
		}

		// Check if path is under targetDir
		relPath, err := filepath.Rel(targetDir, absPath)
		if err != nil {
			relativePaths[i] = ""
			errors[i] = fmt.Errorf("failed to compute relative path: %w", err)
			continue
		}

		// If relative path starts with "..", it's outside targetDir
		if strings.HasPrefix(relPath, "..") {
			relativePaths[i] = ""
			errors[i] = fmt.Errorf("path is outside target directory: %s", absPath)
			continue
		}

		relativePaths[i] = relPath
		errors[i] = nil
	}

	return relativePaths, errors
}
