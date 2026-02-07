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
// Non-template files get normal symlinks; template files get rendered and symlinked.
func (m *Manager) RestoreFolderWithTemplates(subEntry config.SubEntry, source, target string) error {
	if !pathExists(source) {
		m.logger.Debug("source folder does not exist", slog.String("path", source))
		return nil
	}

	// Create target directory if needed
	if !pathExists(target) {
		m.logger.Info("creating directory", slog.String("path", target))
		if !m.DryRun {
			if err := os.MkdirAll(target, DirPerms); err != nil {
				return NewPathError("restore", target, fmt.Errorf("creating target directory: %w", err))
			}
		}
	}

	return filepath.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, relErr := filepath.Rel(source, path)
		if relErr != nil {
			return relErr
		}

		// Skip root directory
		if relPath == "." {
			return nil
		}

		// Skip generated files
		if tmpl.IsRenderedFile(d.Name()) || tmpl.IsConflictFile(d.Name()) {
			return nil
		}

		targetPath := filepath.Join(target, relPath)

		if d.IsDir() {
			if !pathExists(targetPath) && !m.DryRun {
				if err := os.MkdirAll(targetPath, DirPerms); err != nil {
					return NewPathError("restore", targetPath, fmt.Errorf("creating directory: %w", err))
				}
			}
			return nil
		}

		if tmpl.IsTemplateFile(d.Name()) {
			return m.restoreTemplateFile(subEntry, path, relPath, target)
		}

		// Non-template file: create symlink
		return m.restoreSingleFile(subEntry, path, targetPath)
	})
}

// restoreTemplateFile renders a .tmpl file and manages the symlink to the .rendered output.
//
//nolint:gocyclo // complexity acceptable for template restore logic with merge paths
func (m *Manager) restoreTemplateFile(subEntry config.SubEntry, tmplAbsPath, relPath, target string) error {
	// Read template source
	tmplContent, err := os.ReadFile(tmplAbsPath) //nolint:gosec // path from config
	if err != nil {
		return NewPathError("restore", tmplAbsPath, fmt.Errorf("reading template: %w", err))
	}

	// Compute hash of template source
	hash := fmt.Sprintf("%x", sha256.Sum256(tmplContent))

	// The rendered output sits alongside the template as a sibling
	renderedAbsPath := tmpl.RenderedPath(tmplAbsPath)

	// The target symlink name strips .tmpl from the filename
	targetFileName := tmpl.TargetName(filepath.Base(tmplAbsPath))
	targetDir := filepath.Join(target, filepath.Dir(relPath))
	targetFilePath := filepath.Join(targetDir, targetFileName)

	// Quick check: if we have a state store, check if template is unchanged
	if m.stateStore != nil && !m.ForceRender {
		record, lookupErr := m.stateStore.GetLatestRender(relPath)
		if lookupErr != nil {
			m.logger.Warn("failed to query render history", slog.String("error", lookupErr.Error()))
		} else if record != nil && record.TemplateHash == hash && pathExists(renderedAbsPath) {
			// Template unchanged and rendered file exists - just ensure symlink
			m.logger.Debug("template unchanged, skipping re-render",
				slog.String("template", relPath))
			return m.ensureSymlink(subEntry, renderedAbsPath, targetFilePath)
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

	// Create/update symlink: target -> rendered file
	return m.ensureSymlink(subEntry, renderedAbsPath, targetFilePath)
}

// restoreSingleFile creates a symlink from source to target for a single non-template file.
func (m *Manager) restoreSingleFile(subEntry config.SubEntry, srcFile, dstFile string) error {
	if symlinkPointsTo(dstFile, srcFile) {
		m.logger.Debug("already a symlink", slog.String("path", dstFile))
		return nil
	}

	if isSymlink(dstFile) {
		m.logger.Info("removing incorrect symlink", slog.String("path", dstFile))
		if !m.DryRun {
			if err := os.Remove(dstFile); err != nil {
				return NewPathError("restore", dstFile, fmt.Errorf("removing incorrect symlink: %w", err))
			}
		}
	}

	if pathExists(dstFile) && !isSymlink(dstFile) {
		m.logger.Info("removing existing file for symlink", slog.String("path", dstFile))
		if !m.DryRun {
			if err := os.Remove(dstFile); err != nil {
				return NewPathError("restore", dstFile, fmt.Errorf("removing existing file: %w", err))
			}
		}
	}

	m.logger.Info("creating symlink",
		slog.String("target", dstFile),
		slog.String("source", srcFile))

	if !m.DryRun {
		return createSymlink(m.ctx, srcFile, dstFile, subEntry.Sudo)
	}
	return nil
}

// ensureSymlink creates or updates a symlink from source to target.
func (m *Manager) ensureSymlink(subEntry config.SubEntry, source, target string) error {
	if symlinkPointsTo(target, source) {
		return nil
	}

	// Ensure parent directory exists
	parentDir := filepath.Dir(target)
	if !pathExists(parentDir) && !m.DryRun {
		if err := os.MkdirAll(parentDir, DirPerms); err != nil {
			return NewPathError("restore", parentDir, fmt.Errorf("creating parent: %w", err))
		}
	}

	// Remove existing symlink/file
	if pathExists(target) || isSymlink(target) {
		if !m.DryRun {
			if err := os.Remove(target); err != nil {
				return NewPathError("restore", target, fmt.Errorf("removing existing: %w", err))
			}
		}
	}

	m.logger.Info("creating symlink",
		slog.String("target", target),
		slog.String("source", source))

	if !m.DryRun {
		return createSymlink(m.ctx, source, target, subEntry.Sudo)
	}
	return nil
}
