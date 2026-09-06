package config

import (
	"fmt"
	"strings"
)

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

// isTemplatePath returns true if the path contains Go template delimiters,
// meaning it will be rendered later and should not be validated now.
func isTemplatePath(path string) bool {
	return strings.Contains(path, "{{")
}

// validateSetupEntry validates a setup sub-entry: one declaring a `run` command.
// A sub-entry is either a config entry (it has a backup and targets) or a setup
// entry (it has run/check commands) — never both.
//
// The run/check OS-key pairing is what makes `check` genuinely required. Without
// it, a run command with no check would execute on every single restore.
func validateSetupEntry(appName string, entry SubEntry) []error {
	var errs []error

	entryPath := fmt.Sprintf("%s/%s", appName, entry.Name)

	if !entry.IsSetup() {
		if entry.CheckMode != "" {
			errs = append(errs, NewFieldError(entryPath, "check_mode", entry.CheckMode,
				fmt.Errorf("check_mode is only valid for setup entries")))
		}

		// Not a setup entry. A stray `check` with no `run` is dead config.
		if len(entry.Check) > 0 {
			errs = append(errs, NewFieldError(entryPath, "check", "",
				fmt.Errorf("check requires a matching run command")))
		}

		return errs
	}

	switch entry.CheckMode {
	case "", CheckModeExitCode, CheckModeStatus:
	default:
		errs = append(errs, NewFieldError(entryPath, "check_mode", entry.CheckMode,
			fmt.Errorf("must be %q or %q", CheckModeExitCode, CheckModeStatus)))
	}

	// A setup entry deploys nothing, so backup and targets are meaningless on it.
	if entry.Backup != "" {
		errs = append(errs, NewFieldError(entryPath, "backup", entry.Backup,
			fmt.Errorf("a setup entry (one with run) cannot also declare a backup")))
	}

	if len(entry.Targets) > 0 {
		errs = append(errs, NewFieldError(entryPath, "targets", "",
			fmt.Errorf("a setup entry (one with run) cannot declare targets")))
	}

	// Every OS with a run command needs a check command, and vice versa.
	for os, cmd := range entry.Run {
		if strings.TrimSpace(cmd) == "" {
			errs = append(errs, NewFieldError(entryPath, fmt.Sprintf("run[%s]", os), cmd,
				fmt.Errorf("command cannot be empty")))
		}

		if strings.TrimSpace(entry.Check[os]) == "" {
			errs = append(errs, NewFieldError(entryPath, fmt.Sprintf("check[%s]", os), "",
				fmt.Errorf("run[%s] requires a non-empty check command for the same OS", os)))
		}
	}

	for os := range entry.Check {
		if _, ok := entry.Run[os]; !ok {
			errs = append(errs, NewFieldError(entryPath, fmt.Sprintf("check[%s]", os), "",
				fmt.Errorf("check for %q has no matching run command", os)))
		}
	}

	return errs
}

// validateEntryPaths validates all path fields on a sub-entry, skipping
// paths that contain template expressions (they will be rendered later).
func validateEntryPaths(appName string, entry SubEntry) []error {
	var errs []error

	// Config-type entries must declare at least one target OS.
	if entry.IsConfig() && len(entry.Targets) == 0 {
		errs = append(errs, NewFieldError(
			fmt.Sprintf("%s/%s", appName, entry.Name),
			"targets", "", fmt.Errorf("at least one target is required"),
		))
	}

	// Validate backup path
	if entry.Backup != "" && !isTemplatePath(entry.Backup) {
		if err := ValidatePath(entry.Backup); err != nil {
			errs = append(errs, NewFieldError(
				fmt.Sprintf("%s/%s", appName, entry.Name),
				"backup", entry.Backup, err,
			))
		}
	}

	// Validate target paths
	for os, target := range entry.Targets {
		if target == "" || isTemplatePath(target) {
			continue
		}

		if err := ValidatePath(target); err != nil {
			errs = append(errs, NewFieldError(
				fmt.Sprintf("%s/%s", appName, entry.Name),
				fmt.Sprintf("targets[%s]", os), target, err,
			))
		}
	}

	// Validate deployment method.
	switch entry.Method {
	case "", MethodSymlink, MethodCopy:
	default:
		errs = append(errs, NewFieldError(
			fmt.Sprintf("%s/%s", appName, entry.Name),
			"method", entry.Method,
			fmt.Errorf("must be %q or %q", MethodSymlink, MethodCopy),
		))
	}

	// Copy mode requires an explicit, non-empty files list (v1: files-only).
	if entry.Method == MethodCopy && len(entry.Files) == 0 {
		errs = append(errs, NewFieldError(
			fmt.Sprintf("%s/%s", appName, entry.Name),
			"files", "",
			fmt.Errorf("copy mode requires an explicit files list"),
		))
	}

	// Copy mode is only meaningful for config entries, which declare a backup.
	if entry.Method == MethodCopy && entry.Backup == "" {
		errs = append(errs, NewFieldError(
			fmt.Sprintf("%s/%s", appName, entry.Name),
			"backup", "",
			fmt.Errorf("copy mode requires a backup path"),
		))
	}

	errs = append(errs, validateSetupEntry(appName, entry)...)

	return errs
}

// validateGitPackagePaths validates target paths on a git package configuration.
func validateGitPackagePaths(appName string, gitPkg *GitPackage) []error {
	var errs []error

	for os, target := range gitPkg.Targets {
		if target == "" || isTemplatePath(target) {
			continue
		}

		if err := ValidatePath(target); err != nil {
			errs = append(errs, NewFieldError(
				appName,
				fmt.Sprintf("package.managers.git.targets[%s]", os), target, err,
			))
		}
	}

	return errs
}

// ValidateConfig validates the entire config including all applications
func ValidateConfig(cfg *Config) []error {
	var errs []error

	// Validate version
	if cfg.Version != 3 {
		errs = append(errs, fmt.Errorf("%w: %d (expected 3)", ErrUnsupportedVersion, cfg.Version))
	}

	// Validate applications
	appNames := make(map[string]bool)

	for _, app := range cfg.Applications {
		if app.Name == "" {
			errs = append(errs, fmt.Errorf("%w: application has empty name", ErrInvalidConfig))

			continue
		}

		if appNames[app.Name] {
			errs = append(errs, fmt.Errorf("%w: duplicate application name %q", ErrInvalidConfig, app.Name))
		}

		appNames[app.Name] = true

		// Validate sub-entries
		subNames := make(map[string]bool)
		for _, entry := range app.Entries {
			if entry.Name == "" {
				errs = append(errs, fmt.Errorf("%w: application %q has entry with empty name", ErrInvalidConfig, app.Name))

				continue
			}

			if subNames[entry.Name] {
				errs = append(errs, fmt.Errorf("%w: application %q has duplicate entry name %q", ErrInvalidConfig, app.Name, entry.Name))
			}

			subNames[entry.Name] = true

			// Validate paths on each entry
			errs = append(errs, validateEntryPaths(app.Name, entry)...)
		}

		// Validate git package target paths
		if app.Package != nil {
			if gitPkg, ok := app.Package.GetGitPackage(); ok {
				errs = append(errs, validateGitPackagePaths(app.Name, gitPkg)...)
			}
		}
	}

	return errs
}
