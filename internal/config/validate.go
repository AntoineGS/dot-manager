package config

import (
	"fmt"
	"strings"
)

// ValidationError represents a validation error for an entry
type ValidationError struct {
	EntryName string
	Field     string
	Message   string
}

func (e ValidationError) Error() string {
	if e.Field != "" {
		return fmt.Sprintf("entry '%s': %s - %s", e.EntryName, e.Field, e.Message)
	}

	return fmt.Sprintf("entry '%s': %s", e.EntryName, e.Message)
}

// ValidatePath checks a path for potential security issues.
// It returns an error if the path contains null bytes or suspicious patterns.
func ValidatePath(path string) error {
	// Check for null bytes
	if strings.ContainsRune(path, '\x00') {
		return fmt.Errorf("path contains null byte")
	}

	// Check for path traversal patterns
	if strings.Contains(path, "..") {
		return fmt.Errorf("path contains traversal pattern '..'")
	}

	return nil
}

// ValidateEntry validates a single entry
func ValidateEntry(e *Entry) error {
	// Name is required
	if strings.TrimSpace(e.Name) == "" {
		return ValidationError{
			EntryName: "(unnamed)",
			Field:     "name",
			Message:   "name is required",
		}
	}

	isConfig := e.IsConfig()
	hasPackage := e.HasPackage()

	// Entry must have at least one of: config or package
	if !isConfig && !hasPackage {
		return ValidationError{
			EntryName: e.Name,
			Message:   "entry must have backup (config type) or package configuration",
		}
	}

	// Validate config fields if present
	if isConfig {
		if err := validateConfigFields(e); err != nil {
			return err
		}
	}

	// Validate package fields if present
	if hasPackage {
		if err := validatePackageFields(e); err != nil {
			return err
		}
	}

	return nil
}

func validateConfigFields(e *Entry) error {
	// Backup is required for config entries
	if strings.TrimSpace(e.Backup) == "" {
		return ValidationError{
			EntryName: e.Name,
			Field:     "backup",
			Message:   "backup path is required for config entries",
		}
	}

	if err := ValidatePath(e.Backup); err != nil {
		return ValidationError{
			EntryName: e.Name,
			Field:     "backup",
			Message:   err.Error(),
		}
	}

	// At least one target is required
	if len(e.Targets) == 0 {
		return ValidationError{
			EntryName: e.Name,
			Field:     "targets",
			Message:   "at least one target is required for config entries",
		}
	}

	// Validate target paths are not empty
	for os, target := range e.Targets {
		if strings.TrimSpace(target) == "" {
			return ValidationError{
				EntryName: e.Name,
				Field:     fmt.Sprintf("targets.%s", os),
				Message:   "target path cannot be empty",
			}
		}

		if err := ValidatePath(target); err != nil {
			return ValidationError{
				EntryName: e.Name,
				Field:     fmt.Sprintf("targets.%s", os),
				Message:   err.Error(),
			}
		}
	}

	return nil
}

func validatePackageFields(e *Entry) error {
	pkg := e.Package

	// At least one of managers/custom/url is required
	hasManagers := len(pkg.Managers) > 0
	hasCustom := len(pkg.Custom) > 0
	hasURL := len(pkg.URL) > 0

	if !hasManagers && !hasCustom && !hasURL {
		return ValidationError{
			EntryName: e.Name,
			Field:     "package",
			Message:   "package must have at least one of: managers, custom, or url",
		}
	}

	return nil
}

// ValidateEntries validates all entries and returns all validation errors
func ValidateEntries(entries []Entry) []error {
	var errors []error
	seen := make(map[string]bool)

	for i := range entries {
		// Check for duplicate names
		if seen[entries[i].Name] {
			errors = append(errors, ValidationError{
				EntryName: entries[i].Name,
				Message:   "duplicate entry name",
			})

			continue
		}
		seen[entries[i].Name] = true

		if err := ValidateEntry(&entries[i]); err != nil {
			errors = append(errors, err)
		}
	}

	return errors
}

// ValidateConfig validates the entire config including all applications
func ValidateConfig(cfg *Config) []error {
	var errors []error

	// Validate version
	if cfg.Version != 3 {
		errors = append(errors, fmt.Errorf("unsupported config version: %d (expected 3)", cfg.Version))
	}

	// TODO: Add validation for applications and sub-entries

	return errors
}
