package manager

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"path/filepath"

	"github.com/AntoineGS/tidydots/internal/state"
)

type templateRenderHistory struct {
	key        string
	record     *state.RenderRecord
	legacy     bool
	sourceOnly bool
}

// templateStateKey identifies the source relative to the repository, not the
// entry. The ./ marker separates scoped keys from legacy filepath.Rel keys,
// including templates directly in the repository root.
func (m *Manager) templateStateKey(path string) (string, error) {
	root, err := m.ResolvePath(".")
	if err != nil {
		return "", err
	}
	root, err = filepath.Abs(root)
	if err != nil {
		return "", err
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(root, absPath)
	if err != nil {
		return "", fmt.Errorf("resolving template state key: %w", err)
	}
	return "./" + normalizeStateKey(rel), nil
}

// templateHistory is read-only. Symlinks share the source's rendered output;
// copies have independent output at copyTarget (the suffix-free deployment path).
func (m *Manager) templateHistory(path, legacyRelPath, copyTarget string) (templateRenderHistory, error) {
	sourceKey, err := m.templateStateKey(path)
	history := templateRenderHistory{key: sourceKey}
	if err != nil || m.stateStore == nil {
		return history, err
	}
	if copyTarget != "" {
		target, absErr := filepath.Abs(copyTarget)
		if absErr != nil {
			return history, fmt.Errorf("resolving copy template target: %w", absErr)
		}
		// Quoting both components makes the tuple unambiguous even for paths
		// containing delimiters. Do not follow migration symlinks at the target.
		history.key = fmt.Sprintf("copy:%q:%q", sourceKey, normalizeStateKey(target))
	}
	history.record, err = m.stateStore.GetLatestRender(m.ctx, history.key, m.Platform.OS, m.Platform.Hostname)
	if err != nil || history.record != nil {
		return history, err
	}
	if copyTarget != "" {
		// Earlier source-only records cannot tell us which copy target was
		// updated. Their presence prevents treating existing output as an orphan.
		records, lookupErr := m.stateStore.GetRenderHistory(m.ctx, sourceKey, 1)
		if lookupErr != nil {
			return history, lookupErr
		}
		history.legacy = len(records) > 0
		history.sourceOnly = history.legacy
	}
	return m.legacyTemplateHistory(path, legacyRelPath, history)
}

// Legacy entry-relative histories can contain multiple applications; only an
// exact source hash with a consistent pure render on this OS/host is safe to
// reuse. Never infer a base from local edits or another deployment's new key.
func (m *Manager) legacyTemplateHistory(path, legacyRelPath string, history templateRenderHistory) (templateRenderHistory, error) {
	records, err := m.stateStore.GetRenderHistory(m.ctx, normalizeStateKey(legacyRelPath), -1)
	if err != nil || len(records) == 0 {
		return history, err
	}
	history.legacy = true
	history.sourceOnly = false
	source, err := m.fs.ReadFile(path)
	if err != nil {
		return history, fmt.Errorf("reading source for legacy history: %w", err)
	}
	hash := fmt.Sprintf("%x", sha256.Sum256(source))
	for _, record := range records {
		if record.PlatformOS != m.Platform.OS || record.PlatformHost != m.Platform.Hostname || record.TemplateHash != hash {
			continue
		}
		if history.record != nil && !bytes.Equal(history.record.PureRender, record.PureRender) {
			history.record = nil
			return history, nil
		}
		if history.record == nil {
			history.record = &record
		}
	}
	return history, nil
}

func (h templateRenderHistory) checkLegacyOverwrite(exists, force bool) error {
	if h.legacy && h.record == nil && exists && !force {
		return fmt.Errorf("legacy render history cannot be safely attributed to %q; existing output left unchanged; preserve local edits before using --force-render", h.key)
	}
	return nil
}

func (m *Manager) checkCopyLegacyOverwrite(history templateRenderHistory, selection selectedTemplate, snapshot templateCopySnapshot) error {
	guardErr := history.checkLegacyOverwrite(snapshot.Exists, m.ForceRender)
	if guardErr == nil || !history.sourceOnly {
		return guardErr
	}
	// A target identical to a fresh render needs no historical base. This also
	// permits unedited symlink-to-copy migrations without trusting shared history.
	source, err := m.fs.ReadFile(selection.templatePath)
	if err != nil {
		return fmt.Errorf("reading source for copy baseline: %w", err)
	}
	rendered, err := m.templateEngine.RenderBytes(selection.relPath, source)
	if err != nil {
		return fmt.Errorf("rendering copy baseline: %w", err)
	}
	if bytes.Equal(snapshot.Content, rendered) {
		return nil
	}
	return guardErr
}

func (m *Manager) preflightSymlinkTemplateHistory(selected []selectedTemplate) error {
	for _, selection := range selected {
		history, err := m.templateHistory(selection.templatePath, selection.relPath, "")
		if err != nil {
			return NewPathError("restore", selection.templatePath, fmt.Errorf("reading render history: %w", err))
		}
		if err := history.checkLegacyOverwrite(m.pathExists(selection.renderedPath), m.ForceRender); err != nil {
			return NewPathError("restore", selection.templatePath, err)
		}
	}
	return nil
}

// migrateTemplateHistory copies only the verified baseline, leaving the mixed
// legacy history intact. Fast-path restores must do this before source changes.
func (m *Manager) migrateTemplateHistory(history templateRenderHistory) error {
	if m.DryRun || !history.legacy || history.record == nil {
		return nil
	}
	record := history.record
	return m.stateStore.SaveRender(m.ctx, history.key, record.PureRender, record.TemplateHash, m.Platform.OS, m.Platform.Hostname)
}
