package manager

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"

	"github.com/AntoineGS/tidydots/internal/config"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

// normalizeStateKey converts an OS-native relative template path to a
// forward-slash key so state store lookups are stable across platforms
// for a shared dotfiles repo.
func normalizeStateKey(relPath string) string {
	return filepath.ToSlash(relPath)
}

// writeFileAtomic writes data to path via a sibling temp file and a rename,
// so a crash mid-write cannot truncate the existing content.
func (m *Manager) writeFileAtomic(path string, data []byte, perm fs.FileMode) error {
	tmpPath := atomicTempPath(path)

	if err := m.fs.WriteFile(tmpPath, data, perm); err != nil {
		return fmt.Errorf("writing temp file: %w", err)
	}

	if err := m.fs.Rename(tmpPath, path); err != nil {
		// Best-effort cleanup of orphaned temp file.
		_ = m.fs.Remove(tmpPath)
		return fmt.Errorf("renaming temp file into place: %w", err)
	}

	return nil
}

func atomicTempPath(path string) string {
	return filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tidydots-tmp")
}

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
	if !m.pathExists(source) {
		return nil
	}

	return m.renderTemplatesInBackup(source)
}

// renderTemplatesInBackup walks the backup directory for .tmpl files and
// renders each one, creating a relative symlink in the backup dir.
func (m *Manager) renderTemplatesInBackup(backupDir string) error {
	return m.fs.WalkDir(backupDir, func(path string, d fs.DirEntry, err error) error {
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
	tmplContent, err := m.fs.ReadFile(tmplAbsPath)
	if err != nil {
		return NewPathError("restore", tmplAbsPath, fmt.Errorf("reading template: %w", err))
	}

	// Compute hash of template source
	hash := fmt.Sprintf("%x", sha256.Sum256(tmplContent))

	// The rendered output sits alongside the template as a sibling
	renderedAbsPath := tmpl.RenderedPath(tmplAbsPath)

	// Quick check: if we have a state store, check if template is unchanged
	if m.stateStore != nil && !m.ForceRender {
		record, lookupErr := m.stateStore.GetLatestRender(m.ctx, normalizeStateKey(relPath), m.Platform.OS, m.Platform.Hostname)
		if lookupErr != nil {
			m.logger.Warn("failed to query render history", slog.String("error", lookupErr.Error()))
		} else if record != nil && record.TemplateHash == hash && m.pathExists(renderedAbsPath) {
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
		record, lookupErr := m.stateStore.GetLatestRender(m.ctx, normalizeStateKey(relPath), m.Platform.OS, m.Platform.Hostname)
		if lookupErr != nil {
			m.logger.Warn("failed to query render history", slog.String("error", lookupErr.Error()))
		}

		if record != nil {
			// Re-render scenario: 3-way merge
			base := string(record.PureRender)

			var theirs string
			if m.pathExists(renderedAbsPath) {
				theirsBytes, readErr := m.fs.ReadFile(renderedAbsPath)
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

			decision := tmpl.MergeRender(record.PureRender, []byte(theirs), rendered, true, false)

			if len(decision.Conflict) > 0 {
				conflictPath := tmpl.ConflictPath(tmplAbsPath)
				if writeErr := m.fs.WriteFile(conflictPath, decision.Conflict, FilePerms); writeErr != nil {
					m.logger.Warn("could not write conflict file",
						slog.String("path", conflictPath),
						slog.String("error", writeErr.Error()))
				}
				m.logger.Warn("merge conflict detected",
					slog.String("template", relPath),
					slog.String("conflict_file", conflictPath))
			} else {
				// No conflict this round — remove any stale conflict file from a previous run.
				conflictPath := tmpl.ConflictPath(tmplAbsPath)
				if err := m.fs.Remove(conflictPath); err != nil && !errors.Is(err, fs.ErrNotExist) {
					m.logger.Warn("could not remove stale conflict file",
						slog.String("path", conflictPath),
						slog.String("error", err.Error()))
				}
				finalContent = decision.Content
			}
		} else if m.pathExists(renderedAbsPath) {
			// First render but rendered file exists (orphaned) - back it up
			bakPath := renderedAbsPath + ".bak"
			m.logger.Warn("backing up orphaned rendered file",
				slog.String("from", renderedAbsPath),
				slog.String("to", bakPath))
			if copyErr := m.copyFile(renderedAbsPath, bakPath); copyErr != nil {
				m.logger.Warn("could not backup rendered file",
					slog.String("error", copyErr.Error()))
			}
		}
	}

	// Write the rendered content
	if mkdirErr := m.fs.MkdirAll(filepath.Dir(renderedAbsPath), DirPerms); mkdirErr != nil {
		return NewPathError("restore", renderedAbsPath, fmt.Errorf("creating rendered dir: %w", mkdirErr))
	}

	if writeErr := m.writeFileAtomic(filepath.Clean(renderedAbsPath), finalContent, FilePerms); writeErr != nil {
		return NewPathError("restore", renderedAbsPath, fmt.Errorf("writing rendered file: %w", writeErr))
	}

	// Store pure render in DB (always store the unmerged template output)
	if m.stateStore != nil {
		if saveErr := m.stateStore.SaveRender(m.ctx, normalizeStateKey(relPath), rendered, hash, m.Platform.OS, m.Platform.Hostname); saveErr != nil {
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
// Uses fs.Symlink directly (no sudo needed for same-directory relative links).
func (m *Manager) ensureRelativeSymlink(symlinkPath, target string) error {
	// Check if already a correct relative symlink
	if m.isSymlink(symlinkPath) {
		existing, err := m.fs.Readlink(symlinkPath)
		if err == nil && existing == target {
			return nil
		}
	}

	// Remove existing file or incorrect symlink
	if m.pathExists(symlinkPath) || m.isSymlink(symlinkPath) {
		m.logger.Info("removing existing file/symlink for relative symlink",
			slog.String("path", symlinkPath))
		if !m.DryRun {
			if err := m.fs.Remove(symlinkPath); err != nil {
				return NewPathError("restore", symlinkPath, fmt.Errorf("removing existing: %w", err))
			}
		}
	}

	m.logger.Info("creating relative symlink",
		slog.String("link", symlinkPath),
		slog.String("target", target))

	if !m.DryRun {
		return m.fs.Symlink(target, symlinkPath)
	}

	return nil
}
