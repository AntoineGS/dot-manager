package manager

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"log/slog"
	"path/filepath"
	"runtime"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/state"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

const templateCopyOrphanBackupSuffix = ".tidydots.bak"

func templateCopyOrphanBackupPath(target string) string {
	return target + templateCopyOrphanBackupSuffix
}

// restoreCopyTemplateFile renders one selected template directly into its
// suffix-free target. Unlike symlink-mode rendering, copy mode keeps the
// current target as the merge input and does not create a repository alias or
// rendered cache.
//
//nolint:gocyclo // the ordered deployment stages are intentionally explicit
func (m *Manager) restoreCopyTemplateFile(entry config.SubEntry, source, target, file string) error {
	selection := newSelectedTemplate(source, target, file)

	sourceContent, err := m.fs.ReadFile(selection.templatePath)
	if err != nil {
		return NewPathError("restore", selection.templatePath,
			fmt.Errorf("reading template source: %w", err))
	}
	sourceInfo, err := m.fs.Stat(selection.templatePath)
	if err != nil {
		return NewPathError("restore", selection.templatePath,
			fmt.Errorf("stating template source: %w", err))
	}

	snapshot, err := m.readTemplateCopyTarget(selection.targetPath, entry.Sudo)
	if err != nil {
		return NewPathError("restore", selection.targetPath, err)
	}

	hash := fmt.Sprintf("%x", sha256.Sum256(sourceContent))
	record, err := m.latestCopyTemplateRender(selection.relPath)
	if err != nil {
		return NewPathError("restore", selection.templatePath,
			fmt.Errorf("reading render history: %w", err))
	}

	if record != nil && record.TemplateHash == hash && snapshot.Exists && !m.ForceRender {
		m.logCopyTemplateAction("preserving unchanged copy template", selection)
		return m.repairUnchangedCopyTemplate(entry, selection.targetPath, sourceInfo, snapshot)
	}

	m.logCopyTemplateAction("rendering copy template", selection)
	rendered, err := m.templateEngine.RenderBytes(selection.relPath, sourceContent)
	if err != nil {
		return NewPathError("restore", selection.templatePath,
			fmt.Errorf("rendering template: %w", err))
	}

	current := snapshot.Content
	hasHistory := record != nil
	var base []byte
	if hasHistory {
		base = record.PureRender
		if !snapshot.Exists {
			current = base
		}
	}
	decision := tmpl.MergeRender(base, current, rendered, hasHistory, m.ForceRender)
	mode := templateCopyTargetMode(entry, sourceInfo, snapshot)
	needsWrite := !snapshot.Exists || snapshot.Symlink || !bytes.Equal(snapshot.Content, decision.Content)

	if needsWrite && snapshot.Exists && !hasHistory && !m.ForceRender {
		backupPath := templateCopyOrphanBackupPath(selection.targetPath)
		m.logCopyTemplateArtifactAction("backing up existing copy target", selection.targetPath, backupPath)
		if err := m.writeTemplateCopyFile(backupPath, snapshot.Content, FilePerms, entry.Sudo, true); err != nil {
			return NewPathError("restore", backupPath,
				fmt.Errorf("saving orphan target backup: %w", err))
		}
	}

	if len(decision.Conflict) > 0 {
		conflictMessage := "copy template merge conflict; deploying pure render"
		if m.DryRun {
			conflictMessage = "would " + conflictMessage
		}
		m.logger.Warn(conflictMessage,
			slog.String("template", selection.relPath),
			slog.String("target", selection.targetPath),
			slog.String("conflict_file", selection.conflictPath),
			slog.Bool("dry_run", m.DryRun))
		m.logCopyTemplateArtifactAction("saving copy template conflict", selection.targetPath, selection.conflictPath)
		if err := m.writeTemplateCopyFile(selection.conflictPath, decision.Conflict, FilePerms, false, false); err != nil {
			return NewPathError("restore", selection.conflictPath,
				fmt.Errorf("saving template conflict: %w", err))
		}
	} else {
		if m.pathExists(selection.conflictPath) {
			m.logCopyTemplateArtifactAction("removing stale copy template conflict", selection.targetPath, selection.conflictPath)
		}
		if err := m.removeTemplateCopyArtifact(selection.conflictPath, false); err != nil {
			return NewPathError("restore", selection.conflictPath,
				fmt.Errorf("removing stale template conflict: %w", err))
		}
	}

	if needsWrite {
		m.logCopyTemplateArtifactAction("copying rendered template", selection.templatePath, selection.targetPath)
		if err := m.ensureTemplateCopyParent(filepath.Dir(selection.targetPath), entry.Sudo); err != nil {
			return NewPathError("restore", selection.targetPath,
				fmt.Errorf("creating target parent: %w", err))
		}
		if err := m.writeTemplateCopyFile(selection.targetPath, decision.Content, mode, entry.Sudo, false); err != nil {
			return NewPathError("restore", selection.targetPath,
				fmt.Errorf("writing rendered copy: %w", err))
		}
	} else if err := m.repairTemplateCopyOwner(selection.targetPath, entry.Sudo, snapshot); err != nil {
		return NewPathError("restore", selection.targetPath,
			fmt.Errorf("repairing target owner: %w", err))
	}

	if m.DryRun || m.stateStore == nil {
		return nil
	}
	if err := m.stateStore.SaveRender(
		m.ctx, normalizeStateKey(selection.relPath), rendered, hash,
		m.Platform.OS, m.Platform.Hostname,
	); err != nil {
		return NewPathError("restore", selection.targetPath, fmt.Errorf(
			"template deployed but render history save failed (partial success): %w", err))
	}

	return nil
}

func (m *Manager) latestCopyTemplateRender(relPath string) (record *renderRecord, err error) {
	if m.stateStore == nil {
		return nil, nil
	}

	return m.stateStore.GetLatestRender(
		m.ctx, normalizeStateKey(relPath), m.Platform.OS, m.Platform.Hostname,
	)
}

func (m *Manager) repairUnchangedCopyTemplate(
	entry config.SubEntry,
	target string,
	sourceInfo fs.FileInfo,
	snapshot templateCopySnapshot,
) error {
	if snapshot.Symlink {
		m.logCopyTemplateArtifactAction("replacing copy template migration link", target, target)
		mode := templateCopyTargetMode(entry, sourceInfo, snapshot)
		if err := m.writeTemplateCopyFile(target, snapshot.Content, mode, entry.Sudo, false); err != nil {
			return NewPathError("restore", target,
				fmt.Errorf("replacing template migration symlink: %w", err))
		}
		return nil
	}

	if err := m.repairTemplateCopyOwner(target, entry.Sudo, snapshot); err != nil {
		return NewPathError("restore", target,
			fmt.Errorf("repairing target owner: %w", err))
	}
	return nil
}

// renderRecord is the small part of state.RenderRecord used by copy restore.
// It keeps the orchestration independent of the storage implementation while
// allowing the concrete store API to remain unchanged.
type renderRecord = state.RenderRecord

func templateCopyTargetMode(
	entry config.SubEntry,
	sourceInfo fs.FileInfo,
	snapshot templateCopySnapshot,
) fs.FileMode {
	if snapshot.Exists && !snapshot.Symlink {
		return snapshot.Mode
	}
	useSudo, _ := templateCopySudoPolicy(runtime.GOOS, entry.Sudo)
	if useSudo {
		return FilePerms
	}
	return sourceInfo.Mode().Perm()
}

func (m *Manager) ensureTemplateCopyParent(parent string, sudo bool) error {
	if m.pathExists(parent) {
		return nil
	}
	if m.DryRun {
		return nil
	}
	useSudo, err := templateCopySudoPolicy(runtime.GOOS, sudo)
	if err != nil {
		return err
	}
	if useSudo {
		result, runErr := m.runner.RunWithSudo(m.ctx, "mkdir", "-p", parent)
		return templateCopyCommandError("creating target parent", result, runErr)
	}
	return m.fs.MkdirAll(parent, DirPerms)
}

func (m *Manager) repairTemplateCopyOwner(
	path string,
	sudo bool,
	snapshot templateCopySnapshot,
) error {
	if !snapshot.Exists || snapshot.RootOwned {
		return nil
	}
	m.logCopyTemplatePathAction("repairing copy template ownership", path)
	useSudo, err := templateCopySudoPolicy(runtime.GOOS, sudo)
	if err != nil {
		return err
	}
	if !useSudo || m.DryRun {
		return nil
	}
	result, runErr := m.runner.RunWithSudo(m.ctx, "chown", "0:0", "--", path)
	return templateCopyCommandError("repairing target owner", result, runErr)
}

func (m *Manager) logCopyTemplateAction(message string, selection selectedTemplate) {
	attrs := []any{slog.String("template", selection.relPath), slog.Bool("dry_run", m.DryRun)}
	if selection.targetPath != "" {
		attrs = append(attrs, slog.String("target", selection.targetPath))
	}
	if m.DryRun {
		message = "would " + message
	}
	m.logger.Info(message, attrs...)
}

func (m *Manager) logCopyTemplateArtifactAction(message, from, to string) {
	attrs := []any{slog.String("path", to), slog.Bool("dry_run", m.DryRun)}
	if from != "" {
		attrs = append(attrs, slog.String("source", from))
	}
	if m.DryRun {
		message = "would " + message
	}
	m.logger.Info(message, attrs...)
}

func (m *Manager) logCopyTemplatePathAction(message, path string) {
	if m.DryRun {
		message = "would " + message
	}
	m.logger.Info(message, slog.String("path", path), slog.Bool("dry_run", m.DryRun))
}
