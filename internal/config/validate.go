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
	isGit := e.IsGit()
	hasPackage := e.HasPackage()

	// Entry must be exactly one of: config type, git type, or package-only
	if isConfig && isGit {
		return ValidationError{
			EntryName: e.Name,
			Message:   "entry cannot have both backup (config type) and repo (git type)",
		}
	}

	// Entry must have at least one of: config, git, or package
	if !isConfig && !isGit && !hasPackage {
		return ValidationError{
			EntryName: e.Name,
			Message:   "entry must have backup (config type), repo (git type), or package configuration",
		}
	}

	// Validate config fields if present
	if isConfig {
		if err := validateConfigFields(e); err != nil {
			return err
		}
	}

	// Validate git fields if present
	if isGit {
		if err := validateGitFields(e); err != nil {
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
	}

	return nil
}

func validateGitFields(e *Entry) error {
	// Repo is already validated by IsGit() being true

	// At least one target is required for git entries
	if len(e.Targets) == 0 {
		return ValidationError{
			EntryName: e.Name,
			Field:     "targets",
			Message:   "at least one target is required for git entries",
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

// ValidateConfig validates the entire config including all entries
func ValidateConfig(cfg *Config) []error {
	var errors []error

	// Validate version
	if cfg.Version != 2 {
		errors = append(errors, fmt.Errorf("unsupported config version: %d (expected 2)", cfg.Version))
	}

	// Validate entries
	entryErrors := ValidateEntries(cfg.Entries)
	errors = append(errors, entryErrors...)

	return errors
}
