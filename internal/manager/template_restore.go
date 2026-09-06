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

// writeFileAtomic installs through an exclusive, randomly named sibling stage,
// so failed writes cannot truncate the existing content or an occupied stage.
func (m *Manager) writeFileAtomic(path string, data []byte, perm fs.FileMode) error {
	return m.writeTemplateCopyFile(path, data, perm, false, false)
}

func atomicTempPath(path string) string {
	return filepath.Join(filepath.Dir(path), "."+filepath.Base(path)+".tidydots-tmp")
}

// RestoreFolderWithTemplates handles folders that contain .tmpl files.
// It delegates folder-level operations (adoption, merge, folder symlink) to RestoreFolder,
// then renders templates and creates relative symlinks inside the backup directory.
func (m *Manager) RestoreFolderWithTemplates(subEntry config.SubEntry, source, target string) error {
	if err := m.preflightFolderTemplateAliases(source, target); err != nil {
		return NewPathError("restore", target, err)
	}
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
	if err := m.preflightLiteralTemplateAlias(tmplAbsPath); err != nil {
		return NewPathError("restore", tmplAbsPath, err)
	}
	alias := filepath.Join(filepath.Dir(tmplAbsPath), tmpl.TargetName(filepath.Base(tmplAbsPath)))
	preserved, err := m.preserveLiteralAlias(alias)
	if err != nil {
		return NewPathError("restore", alias, fmt.Errorf("preserving template alias: %w", err))
	}
	// Read template source
	tmplContent, err := m.fs.ReadFile(tmplAbsPath)
	if err != nil {
		return NewPathError("restore", tmplAbsPath, fmt.Errorf("reading template: %w", err))
	}

	// Compute hash of template source
	hash := fmt.Sprintf("%x", sha256.Sum256(tmplContent))

	// The rendered output sits alongside the template as a sibling
	renderedAbsPath := tmpl.RenderedPath(tmplAbsPath)
	history, err := m.templateHistory(tmplAbsPath, relPath, "")
	if err != nil {
		return NewPathError("restore", tmplAbsPath, fmt.Errorf("reading render history: %w", err))
	}
	if err := history.checkLegacyOverwrite(m.pathExists(renderedAbsPath), m.ForceRender); err != nil {
		return NewPathError("restore", tmplAbsPath, err)
	}
	record := history.record

	// Quick check: if we have a state store, check if template is unchanged
	if m.stateStore != nil && !m.ForceRender {
		if record != nil && record.TemplateHash == hash && m.pathExists(renderedAbsPath) {
			if err := m.migrateTemplateHistory(history); err != nil {
				return NewPathError("restore", tmplAbsPath, fmt.Errorf("migrating render history: %w", err))
			}
			// Template unchanged and rendered file exists - just ensure relative symlink
			m.logger.Debug("template unchanged, skipping re-render",
				slog.String("template", relPath))
			return m.ensureRelativeSymlinkForTemplate(tmplAbsPath, preserved)
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

	if !m.ForceRender {
		if record != nil {
			// Re-render scenario: 3-way merge
			base := string(record.PureRender)

			var theirs string
			current, readErr := m.readTemplateCopyTarget(renderedAbsPath, false)
			if readErr != nil {
				return NewPathError("restore", renderedAbsPath, readErr)
			}
			if current.Symlink && !current.Exists {
				return fmt.Errorf("rendered output %q is a dangling symlink", renderedAbsPath)
			}
			if current.Exists {
				theirs = string(current.Content)
			} else {
				theirs = base // No rendered file on disk, treat as unchanged
			}

			decision := tmpl.MergeRender(record.PureRender, []byte(theirs), rendered, true, false)

			if len(decision.Conflict) > 0 {
				conflictPath := tmpl.ConflictPath(tmplAbsPath)
				if writeErr := m.writeTemplateCopyFile(conflictPath, decision.Conflict, 0o600, false, true); writeErr != nil {
					return NewPathError("restore", conflictPath, fmt.Errorf("preserving merge conflict: %w", writeErr))
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
		} else if err := m.preserveOrphanRender(renderedAbsPath); err != nil {
			return NewPathError("restore", renderedAbsPath, fmt.Errorf("preserving orphan render: %w", err))
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
		if saveErr := m.stateStore.SaveRender(m.ctx, history.key, rendered, hash, m.Platform.OS, m.Platform.Hostname); saveErr != nil {
			m.logger.Warn("failed to save render record",
				slog.String("template", relPath),
				slog.String("error", saveErr.Error()))
		}
	}

	// Create relative symlink in backup dir: name → name.tmpl.rendered
	return m.ensureRelativeSymlinkForTemplate(tmplAbsPath, preserved)
}

// ensureRelativeSymlinkForTemplate creates a relative symlink in the backup directory
// for a template file: e.g., "config" → "config.tmpl.rendered".
func (m *Manager) ensureRelativeSymlinkForTemplate(tmplAbsPath string, preserved *literalAliasPreservation) error {
	targetFileName := tmpl.TargetName(filepath.Base(tmplAbsPath))
	symlinkPath := filepath.Join(filepath.Dir(tmplAbsPath), targetFileName)
	renderedFileName := filepath.Base(tmpl.RenderedPath(tmplAbsPath))

	return m.ensureRelativeSymlinkPrepared(symlinkPath, renderedFileName, preserved)
}

// ensureRelativeSymlink is an idempotent helper that creates a relative symlink.
// It checks if the symlink already points to the correct target and preserves
// any existing literal before atomically replacing it with a relative symlink.
// Uses fs.Symlink directly (no sudo needed for same-directory relative links).
func (m *Manager) ensureRelativeSymlink(symlinkPath, target string) error {
	preserved, err := m.preserveLiteralAlias(symlinkPath)
	if err != nil {
		return NewPathError("restore", symlinkPath, fmt.Errorf("preserving template alias: %w", err))
	}
	return m.ensureRelativeSymlinkPrepared(symlinkPath, target, preserved)
}

func (m *Manager) ensureRelativeSymlinkPrepared(symlinkPath, target string, preserved *literalAliasPreservation) error {
	// Check if already a correct relative symlink
	if m.isSymlink(symlinkPath) {
		existing, err := m.fs.Readlink(symlinkPath)
		if err == nil && existing == target {
			return nil
		}
	}

	m.logger.Info("creating relative symlink",
		slog.String("link", symlinkPath),
		slog.String("target", target))

	if !m.DryRun {
		return m.replacePreparedAlias(symlinkPath, target, preserved)
	}

	return nil
}
