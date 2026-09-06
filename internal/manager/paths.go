package manager

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/AntoineGS/tidydots/internal/config"
)

// ExpandTarget renders path templates and expands home and environment variables.
// Unlike the legacy config expander it never falls back to a literal template
// or accepts an empty result. Use an explicit "." for the current directory.
func (m *Manager) ExpandTarget(path string) (string, error) {
	original := path
	if strings.Contains(path, "{{") {
		if m.templateEngine == nil {
			return "", fmt.Errorf("cannot render path %q without a template engine", path)
		}
		rendered, err := m.templateEngine.RenderString("path", path)
		if err != nil {
			return "", fmt.Errorf("rendering path %q: %w", path, err)
		}
		path = rendered
	}
	expanded := config.ExpandPath(path, m.Platform.EnvVars)
	if expanded == "" {
		return "", fmt.Errorf("path %q expands to an empty path", original)
	}
	return expanded, nil
}

// ResolvePath expands a backup path and resolves it against the repository root.
func (m *Manager) ResolvePath(path string) (string, error) {
	expanded, err := m.ExpandTarget(path)
	if err != nil {
		return "", err
	}
	if filepath.IsAbs(expanded) {
		return expanded, nil
	}
	root, err := m.repositoryRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, expanded), nil
}

// repositoryRoot preserves cwd for an unset programmatic configuration root.
// A non-empty root expression must still expand to a non-empty path.
func (m *Manager) repositoryRoot() (string, error) {
	if m.Config.BackupRoot == "" {
		return ".", nil
	}
	return m.ExpandTarget(m.Config.BackupRoot)
}
