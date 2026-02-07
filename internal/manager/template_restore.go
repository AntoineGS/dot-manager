package manager

import (
	"crypto/sha256"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/AntoineGS/dot-manager/internal/config"
	tmpl "github.com/AntoineGS/dot-manager/internal/template"
)

// RestoreFolderWithTemplates handles folders that contain .tmpl files.
// It delegates folder-level operations (adoption, merge, folder symlink) to RestoreFolder,
// then renders templates and creates relative symlinks inside the backup directory.
func (m *Manager) RestoreFolderWithTemplates(subEntry config.SubEntry, source, target string) error {
	// Step 1: Delegate folder-level operations to RestoreFolder
	// (handles adoption, merge, creates folder symlink target → source)
	if err := m.RestoreFolder(subEntry, source, target); err != nil {
		return err
	}

	// Step 2: Render templates and create relative symlinks in backup dir
	if !pathExists(source) {
		return nil
	}

	return m.renderTemplatesInBackup(source)
}

// renderTemplatesInBackup walks the backup directory for .tmpl files and
// renders each one, creating a relative symlink in the backup dir.
func (m *Manager) renderTemplatesInBackup(backupDir string) error {
	return filepath.WalkDir(backupDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			return nil
		}

		// Skip generated files
		if tmpl.IsRenderedFile(d.Name()) || tmpl.IsConflictFile(d.Name()) {
			return nil
		}

		if !tmpl.IsTemplateFile(d.Name()) {
			return nil
		}

		relPath, relErr := filepath.Rel(backupDir, path)
		if relErr != nil {
			return relErr
		}

		return m.renderTemplateAndLink(path, relPath)
	})
}

// renderTemplateAndLink renders a single .tmpl file and creates a relative symlink
// in the backup directory pointing to the rendered output.
//
//nolint:gocyclo // complexity acceptable for template restore logic with merge paths
func (m *Manager) renderTemplateAndLink(tmplAbsPath, relPath string) error {
	// Read template source
	tmplContent, err := os.ReadFile(tmplAbsPath) //nolint:gosec // path from config
	if err != nil {
		return NewPathError("restore", tmplAbsPath, fmt.Errorf("reading template: %w", err))
	}

	// Compute hash of template source
	hash := fmt.Sprintf("%x", sha256.Sum256(tmplContent))

	// The rendered output sits alongside the template as a sibling
	renderedAbsPath := tmpl.RenderedPath(tmplAbsPath)

	// Quick check: if we have a state store, check if template is unchanged
	if m.stateStore != nil && !m.ForceRender {
		record, lookupErr := m.stateStore.GetLatestRender(relPath)
		if lookupErr != nil {
			m.logger.Warn("failed to query render history", slog.String("error", lookupErr.Error()))
		} else if record != nil && record.TemplateHash == hash && pathExists(renderedAbsPath) {
			// Template unchanged and rendered file exists - just ensure relative symlink
			m.logger.Debug("template unchanged, skipping re-render",
				slog.String("template", relPath))
			return m.ensureRelativeSymlinkForTemplate(tmplAbsPath)
		}
	}

	// Render the template
	rendered, renderErr := m.templateEngine.RenderBytes(relPath, tmplContent)
	if renderErr != nil {
		return NewPathError("restore", tmplAbsPath, fmt.Errorf("rendering template: %w", renderErr))
	}

	m.logger.Info("rendering template",
		slog.String("template", relPath),
		slog.String("rendered", renderedAbsPath))

	if m.DryRun {
		return nil
	}

	// Determine what to write
	finalContent := rendered

	if m.stateStore != nil && !m.ForceRender {
		record, lookupErr := m.stateStore.GetLatestRender(relPath)
		if lookupErr != nil {
			m.logger.Warn("failed to query render history", slog.String("error", lookupErr.Error()))
		}

		if record != nil {
			// Re-render scenario: 3-way merge
			base := string(record.PureRender)

			var theirs string
			if pathExists(renderedAbsPath) {
				theirsBytes, readErr := os.ReadFile(renderedAbsPath) //nolint:gosec // generated file
				if readErr != nil {
					m.logger.Warn("could not read current rendered file",
						slog.String("path", renderedAbsPath),
						slog.String("error", readErr.Error()))
					theirs = base // Fall back to base if can't read
				} else {
					theirs = string(theirsBytes)
				}
			} else {
				theirs = base // No rendered file on disk, treat as unchanged
			}

			ours := string(rendered)
			mergeResult := tmpl.ThreeWayMerge(base, theirs, ours)

			if mergeResult.HasConflict {
				conflictPath := tmpl.ConflictPath(tmplAbsPath)
				if writeErr := os.WriteFile(conflictPath, []byte(mergeResult.Content), FilePerms); writeErr != nil {
					m.logger.Warn("could not write conflict file",
						slog.String("path", conflictPath),
						slog.String("error", writeErr.Error()))
				}
				m.logger.Warn("merge conflict detected",
					slog.String("template", relPath),
					slog.String("conflict_file", conflictPath))
			}

			finalContent = []byte(mergeResult.Content)
		} else if pathExists(renderedAbsPath) {
			// First render but rendered file exists (orphaned) - back it up
			bakPath := renderedAbsPath + ".bak"
			m.logger.Warn("backing up orphaned rendered file",
				slog.String("from", renderedAbsPath),
				slog.String("to", bakPath))
			if copyErr := copyFile(renderedAbsPath, bakPath); copyErr != nil {
				m.logger.Warn("could not backup rendered file",
					slog.String("error", copyErr.Error()))
			}
		}
	}

	// Write the rendered content
	if mkdirErr := os.MkdirAll(filepath.Dir(renderedAbsPath), DirPerms); mkdirErr != nil {
		return NewPathError("restore", renderedAbsPath, fmt.Errorf("creating rendered dir: %w", mkdirErr))
	}

	if writeErr := os.WriteFile(renderedAbsPath, finalContent, FilePerms); writeErr != nil {
		return NewPathError("restore", renderedAbsPath, fmt.Errorf("writing rendered file: %w", writeErr))
	}

	// Store pure render in DB (always store the unmerged template output)
	if m.stateStore != nil {
		if saveErr := m.stateStore.SaveRender(relPath, rendered, hash, m.Platform.OS, m.Platform.Hostname); saveErr != nil {
			m.logger.Warn("failed to save render record",
				slog.String("template", relPath),
				slog.String("error", saveErr.Error()))
		}
	}

	// Create relative symlink in backup dir: name → name.tmpl.rendered
	return m.ensureRelativeSymlinkForTemplate(tmplAbsPath)
}

// ensureRelativeSymlinkForTemplate creates a relative symlink in the backup directory
// for a template file: e.g., "config" → "config.tmpl.rendered".
func (m *Manager) ensureRelativeSymlinkForTemplate(tmplAbsPath string) error {
	targetFileName := tmpl.TargetName(filepath.Base(tmplAbsPath))
	symlinkPath := filepath.Join(filepath.Dir(tmplAbsPath), targetFileName)
	renderedFileName := filepath.Base(tmpl.RenderedPath(tmplAbsPath))

	return m.ensureRelativeSymlink(symlinkPath, renderedFileName)
}

// ensureRelativeSymlink is an idempotent helper that creates a relative symlink.
// It checks if the symlink already points to the correct target, removes any
// existing file/symlink if not, and creates a new relative symlink.
// Uses os.Symlink directly (no sudo needed for same-directory relative links).
func (m *Manager) ensureRelativeSymlink(symlinkPath, target string) error {
	// Check if already a correct relative symlink
	if isSymlink(symlinkPath) {
		existing, err := os.Readlink(symlinkPath)
		if err == nil && existing == target {
			return nil
		}
	}

	// Remove existing file or incorrect symlink
	if pathExists(symlinkPath) || isSymlink(symlinkPath) {
		m.logger.Info("removing existing file/symlink for relative symlink",
			slog.String("path", symlinkPath))
		if !m.DryRun {
			if err := os.Remove(symlinkPath); err != nil {
				return NewPathError("restore", symlinkPath, fmt.Errorf("removing existing: %w", err))
			}
		}
	}

	m.logger.Info("creating relative symlink",
		slog.String("link", symlinkPath),
		slog.String("target", target))

	if !m.DryRun {
		return os.Symlink(target, symlinkPath)
	}

	return nil
}
