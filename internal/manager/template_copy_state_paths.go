package manager

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/AntoineGS/tidydots/internal/config"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

type copyTemplateStatePath struct {
	path       string
	artifactID string
}

// copyTemplateStatePaths returns lexical and canonical spellings of the
// database and SQLite sidecar paths used by the manager. Parent symlinks are
// resolved through the configured filesystem, while missing final entries are
// kept as names below the resolved parent so absent sidecars remain protected.
func (m *Manager) copyTemplateStatePaths() ([]copyTemplateStatePath, error) {
	envVars := map[string]string{}
	if m.Platform != nil {
		envVars = m.Platform.EnvVars
	}
	databases := make([]string, 0, 2)
	if m.stateStore != nil && m.stateStore.Path() != "" {
		databases = append(databases, m.stateStore.Path())
	}
	if m.Config != nil && m.Config.BackupRoot != "" {
		root := config.ExpandPathWithTemplate(m.Config.BackupRoot, envVars, m.templateEngine)
		databases = append(databases, filepath.Join(root, ".tidydots.db"))
	}

	paths := make([]copyTemplateStatePath, 0, len(databases)*8)
	seen := make(map[string]struct{}, len(databases)*8)
	caseInsensitive := tmpl.IsCaseInsensitiveFilesystem()
	for _, database := range databases {
		cleanDatabase := filepath.Clean(database)
		canonicalDatabase, err := m.canonicalCopyTemplatePath(cleanDatabase)
		if err != nil {
			return nil, fmt.Errorf("canonicalizing state database %q: %w", cleanDatabase, err)
		}

		for _, suffix := range []string{"", "-wal", "-shm", "-journal"} {
			artifactID := copyTemplateStateArtifactID(canonicalDatabase, suffix, caseInsensitive)
			appendCopyTemplateStatePath(&paths, seen, cleanDatabase+suffix, artifactID, caseInsensitive)
			appendCopyTemplateStatePath(&paths, seen, canonicalDatabase+suffix, artifactID, caseInsensitive)
		}
	}
	return paths, nil
}

func copyTemplateStateArtifactID(canonicalDatabase, suffix string, caseInsensitive bool) string {
	return copyTemplateClaimKey(canonicalDatabase+suffix, caseInsensitive)
}

func appendCopyTemplateStatePath(
	paths *[]copyTemplateStatePath,
	seen map[string]struct{},
	path, artifactID string,
	caseInsensitive bool,
) {
	cleanPath := filepath.Clean(path)
	key := copyTemplateClaimKey(cleanPath, caseInsensitive)
	if _, ok := seen[key]; ok {
		return
	}
	seen[key] = struct{}{}
	*paths = append(*paths, copyTemplateStatePath{path: cleanPath, artifactID: artifactID})
}

func copyTemplateStateArtifactPaths(paths []copyTemplateStatePath) []string {
	result := make([]string, 0, len(paths))
	for _, path := range paths {
		result = append(result, path.path)
	}
	return result
}

// canonicalCopyTemplatePath resolves only the existing parent components of a
// path. The final name is deliberately not inspected, which keeps an absent
// database sidecar protected under the canonical parent spelling.
func (m *Manager) canonicalCopyTemplatePath(path string) (string, error) {
	absolute, err := filepath.Abs(filepath.Clean(path))
	if err != nil {
		return "", fmt.Errorf("resolving absolute path: %w", err)
	}

	parent, err := m.resolveCopyTemplateParent(filepath.Dir(absolute), 0)
	if err != nil {
		return "", err
	}
	return filepath.Join(parent, filepath.Base(absolute)), nil
}

func (m *Manager) resolveCopyTemplateParent(path string, symlinkHops int) (string, error) {
	if symlinkHops > 40 {
		return "", fmt.Errorf("too many symlink parent redirects")
	}

	components := copyTemplatePathComponents(path)
	if len(components) == 0 {
		return filepath.Clean(path), nil
	}

	current := ""
	for index, component := range components {
		if index == 0 {
			current = component
		} else {
			current = filepath.Join(current, filepath.Base(component))
		}

		info, err := m.fs.Lstat(current)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return appendCopyTemplateMissingParent(current, components[index+1:]), nil
			}
			return "", fmt.Errorf("inspecting state parent %q: %w", current, err)
		}

		if m.isSymlink(current) {
			link, err := m.fs.Readlink(current)
			if err != nil {
				return "", fmt.Errorf("reading state parent symlink %q: %w", current, err)
			}

			resolved := link
			if !filepath.IsAbs(resolved) {
				resolved = filepath.Join(filepath.Dir(current), resolved)
			}
			resolved = filepath.Clean(resolved)
			if remaining := components[index+1:]; len(remaining) > 0 {
				resolved = filepath.Join(resolved, filepath.Join(pathComponentsBase(remaining)...))
			}
			return m.resolveCopyTemplateParent(resolved, symlinkHops+1)
		}

		if !info.IsDir() {
			return "", fmt.Errorf("state parent %q is not a directory", current)
		}
	}

	return filepath.Clean(current), nil
}

func appendCopyTemplateMissingParent(current string, remaining []string) string {
	if len(remaining) == 0 {
		return filepath.Clean(current)
	}
	return filepath.Join(current, filepath.Join(pathComponentsBase(remaining)...))
}

func pathComponentsBase(components []string) []string {
	parts := make([]string, 0, len(components))
	for _, component := range components {
		parts = append(parts, filepath.Base(component))
	}
	return parts
}
