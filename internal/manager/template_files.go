package manager

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/AntoineGS/tidydots/internal/config"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

// selectedTemplate describes the source, generated artifacts, and target for a
// template selected by a file entry. relPath is intentionally the source path:
// it is also the stable key used by the render state store.
type selectedTemplate struct {
	relPath      string
	aliasRelPath string
	templatePath string
	renderedPath string
	conflictPath string
	backupPath   string
	tempPath     string
	aliasPath    string
	targetPath   string
}

func newSelectedTemplate(source, target, file string) selectedTemplate {
	relPath := filepath.Clean(file)
	aliasRelPath := filepath.Join(filepath.Dir(relPath), tmpl.TargetName(relPath))
	templatePath := filepath.Join(source, relPath)
	renderedPath := tmpl.RenderedPath(templatePath)

	return selectedTemplate{
		relPath:      relPath,
		aliasRelPath: aliasRelPath,
		templatePath: templatePath,
		renderedPath: renderedPath,
		conflictPath: tmpl.ConflictPath(templatePath),
		backupPath:   renderedPath + ".bak",
		tempPath:     atomicTempPath(renderedPath),
		aliasPath:    filepath.Join(source, aliasRelPath),
		targetPath:   filepath.Join(target, aliasRelPath),
	}
}

// preflightTemplateFiles validates every selected template before RestoreFiles
// creates directories, renders a source, or adopts/replaces a target. This is
// deliberately entry-wide: a later invalid template must not leave earlier
// literal files partially restored.
func (m *Manager) preflightTemplateFiles(subEntry config.SubEntry, source, target string) error {
	selected := make([]selectedTemplate, 0, len(subEntry.Files))
	for _, file := range subEntry.Files {
		if !tmpl.IsTemplateFile(file) {
			continue
		}

		relPath, err := validateLocalTemplatePath(file)
		if err != nil {
			return NewPathError("restore", file, err)
		}

		selection := newSelectedTemplate(source, target, relPath)
		if selection.aliasRelPath == "." {
			return NewPathError("restore", selection.templatePath,
				fmt.Errorf("template has no suffix-free target name"))
		}
		selected = append(selected, selection)
	}

	if len(selected) == 0 {
		return nil
	}

	if err := validateTemplateSelectionCollisions(subEntry.Files, tmpl.IsCaseInsensitiveFilesystem()); err != nil {
		return err
	}

	sources := make([]string, 0, len(selected))
	for _, selection := range selected {
		sources = append(sources, selection.templatePath)
	}

	for _, selection := range selected {
		if err := m.preflightTemplateSource(source, selection); err != nil {
			return err
		}
	}

	for _, selection := range selected {
		if err := m.preflightTemplateAlias(selection); err != nil {
			return err
		}

		if err := m.preflightTemplateTarget(selection); err != nil {
			return err
		}

		if err := m.preflightTemplateArtifacts(source, selection, sources); err != nil {
			return err
		}

		if err := m.preflightTemplatePaths(source, target, selection, selected); err != nil {
			return err
		}
	}

	return nil
}

func validateTemplateSelectionCollisions(files []string, caseInsensitive bool) error {
	if err := tmpl.ValidateSelectionCollisions(files, caseInsensitive); err != nil {
		path := "selection"
		if len(files) > 0 {
			path = filepath.Clean(files[0])
		}
		return NewPathError("restore", path, err)
	}
	return nil
}

func validateLocalTemplatePath(file string) (string, error) {
	if filepath.IsAbs(file) || filepath.VolumeName(file) != "" {
		return "", fmt.Errorf("template selection must be relative to the entry")
	}

	cleanPath := filepath.Clean(file)
	if cleanPath == "." || !pathWithin(".", cleanPath) {
		return "", fmt.Errorf("template selection escapes the entry directory")
	}

	return cleanPath, nil
}

func (m *Manager) preflightTemplateSource(source string, selection selectedTemplate) error {
	if err := m.validateTemplatePath(source, selection.templatePath, "template source"); err != nil {
		return err
	}

	info, err := m.fs.Lstat(selection.templatePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return NewPathError("restore", selection.templatePath,
				fmt.Errorf("template source does not exist"))
		}

		return NewPathError("restore", selection.templatePath,
			fmt.Errorf("accessing template source: %w", err))
	}

	if !info.Mode().IsRegular() {
		return NewPathError("restore", selection.templatePath,
			fmt.Errorf("template source is not a regular file"))
	}

	content, err := m.fs.ReadFile(selection.templatePath)
	if err != nil {
		return NewPathError("restore", selection.templatePath,
			fmt.Errorf("reading template source: %w", err))
	}

	if _, err := m.templateEngine.RenderBytes(selection.relPath, content); err != nil {
		return NewPathError("restore", selection.templatePath,
			fmt.Errorf("rendering template: %w", err))
	}

	return nil
}

func (m *Manager) preflightTemplateArtifacts(source string, selection selectedTemplate, sources []string) error {
	artifacts := []struct {
		name string
		path string
	}{
		{name: "rendered output", path: selection.renderedPath},
		{name: "conflict output", path: selection.conflictPath},
		{name: "orphan backup", path: selection.backupPath},
		{name: "temporary output", path: selection.tempPath},
	}

	for _, artifact := range artifacts {
		if err := m.validateTemplatePath(source, artifact.path, artifact.name); err != nil {
			return err
		}

		info, err := m.fs.Lstat(artifact.path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}

			return NewPathError("restore", artifact.path,
				fmt.Errorf("accessing %s: %w", artifact.name, err))
		}

		if m.isSymlink(artifact.path) {
			return NewPathError("restore", artifact.path,
				fmt.Errorf("%s must not be a symlink", artifact.name))
		}
		if info.IsDir() {
			return NewPathError("restore", artifact.path,
				fmt.Errorf("%s must not be a directory", artifact.name))
		}

		for _, sourcePath := range sources {
			if m.sameLocation(artifact.path, sourcePath) {
				return NewPathError("restore", artifact.path,
					fmt.Errorf("%s aliases template source %q", artifact.name, sourcePath))
			}
		}
	}

	return nil
}

func (m *Manager) preflightTemplateAlias(selection selectedTemplate) error {
	_, err := m.fs.Lstat(selection.aliasPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return NewPathError("restore", selection.aliasPath,
			fmt.Errorf("accessing template output alias: %w", err))
	}

	if m.isSymlink(selection.aliasPath) && m.symlinkResolvesTo(selection.aliasPath, selection.renderedPath) {
		return nil
	}

	return NewPathError("restore", selection.aliasPath,
		fmt.Errorf("template output path already exists"))
}

func (m *Manager) preflightTemplateTarget(selection selectedTemplate) error {
	info, err := m.fs.Lstat(selection.targetPath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return NewPathError("restore", selection.targetPath,
			fmt.Errorf("accessing target: %w", err))
	}

	if info.IsDir() {
		return NewPathError("restore", selection.targetPath,
			fmt.Errorf("target path is a directory"))
	}

	// A symlink is always replaceable by ordinary file deployment. In
	// particular, this preserves idempotent restores under --no-merge.
	if m.isSymlink(selection.targetPath) {
		return nil
	}

	if m.NoMerge && !m.ForceDelete {
		return NewPathError("restore", selection.targetPath, fmt.Errorf(
			"target file exists. Use merge mode or --force to proceed"))
	}

	return nil
}

func (m *Manager) preflightTemplatePaths(source, target string, selection selectedTemplate, selected []selectedTemplate) error {
	if m.pathAliasesSource(source, target) {
		return NewPathError("restore", target,
			fmt.Errorf("source and target resolve to the same location"))
	}

	if err := m.validateTemplatePath(source, selection.aliasPath, "template output alias"); err != nil {
		return err
	}
	if err := m.validateTemplatePath(target, selection.targetPath, "template target"); err != nil {
		return err
	}

	// The entry target itself must not be a generated file or a template
	// source. This catches aliases even when the target directory has not been
	// created yet.
	protected := templateProtectedPaths(selected)
	for _, protectedPath := range protected {
		if m.sameLocation(target, protectedPath) {
			return NewPathError("restore", target,
				fmt.Errorf("source and target resolve to the same location"))
		}
	}

	// A target file may already be the expected link created by an earlier
	// restore. Other links to the generated artifacts are unsafe aliases and
	// must be rejected before rendering or unlinking anything.
	if !m.symlinkResolvesTo(selection.targetPath, selection.aliasPath) {
		if m.pathAliasesWithin(source, selection.targetPath) {
			return NewPathError("restore", selection.targetPath,
				fmt.Errorf("source and target resolve to the same location"))
		}

		for _, protectedPath := range protected {
			if tmpl.PathClaimKey(selection.targetPath, tmpl.IsCaseInsensitiveFilesystem()) ==
				tmpl.PathClaimKey(protectedPath, tmpl.IsCaseInsensitiveFilesystem()) ||
				m.sameLocation(selection.targetPath, protectedPath) ||
				m.symlinkResolvesTo(selection.targetPath, protectedPath) {
				return NewPathError("restore", selection.targetPath,
					fmt.Errorf("source and target resolve to the same location"))
			}
		}
	}

	return nil
}

func templateProtectedPaths(selected []selectedTemplate) []string {
	protected := make([]string, 0, len(selected)*6)
	for _, selection := range selected {
		protected = append(protected,
			selection.templatePath,
			selection.renderedPath,
			selection.conflictPath,
			selection.backupPath,
			selection.tempPath,
			selection.aliasPath,
		)
	}
	return protected
}

func (m *Manager) validateTemplatePath(root, candidate, description string) error {
	caseInsensitive := tmpl.IsCaseInsensitiveFilesystem()
	cleanRoot := tmpl.PathClaimKey(root, caseInsensitive)
	cleanCandidate := tmpl.PathClaimKey(candidate, caseInsensitive)
	relPath, err := filepath.Rel(cleanRoot, cleanCandidate)
	if err != nil || relPath == ".." || strings.HasPrefix(relPath, ".."+string(filepath.Separator)) {
		return NewPathError("restore", candidate,
			fmt.Errorf("%s escapes the entry directory", description))
	}
	if relPath == "." {
		return nil
	}

	for current := filepath.Dir(cleanCandidate); current != cleanRoot; current = filepath.Dir(current) {
		if m.isSymlink(current) {
			return NewPathError("restore", candidate,
				fmt.Errorf("%s has a symlink parent", description))
		}
	}

	return nil
}

// pathAliasesSource checks a path and all existing ancestors for an alias of
// source. Checking ancestors matters when target is not yet present but its
// parent is a symlink into the backup tree.
func (m *Manager) pathAliasesSource(source, candidate string) bool {
	if m.sameLocation(source, candidate) || pathWithin(source, candidate) || m.pathAliasesWithin(source, candidate) {
		return true
	}

	for current := filepath.Clean(candidate); ; current = filepath.Dir(current) {
		if m.sameLocation(source, current) || pathWithin(source, current) || m.pathAliasesWithin(source, current) {
			return true
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}

	return false
}

func (m *Manager) pathAliasesWithin(root, candidate string) bool {
	aliased := false
	_ = m.fs.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return filepath.SkipAll
		}
		if m.sameLocation(path, candidate) {
			aliased = true
			return filepath.SkipAll
		}
		return nil
	})
	return aliased
}

func (m *Manager) sameLocation(first, second string) bool {
	caseInsensitive := tmpl.IsCaseInsensitiveFilesystem()
	if tmpl.PathClaimKey(first, caseInsensitive) == tmpl.PathClaimKey(second, caseInsensitive) {
		return true
	}

	firstInfo, firstErr := m.fs.Stat(first)
	secondInfo, secondErr := m.fs.Stat(second)
	return firstErr == nil && secondErr == nil && os.SameFile(firstInfo, secondInfo)
}

func pathWithin(base, candidate string) bool {
	caseInsensitive := tmpl.IsCaseInsensitiveFilesystem()
	rel, err := filepath.Rel(
		tmpl.PathClaimKey(base, caseInsensitive),
		tmpl.PathClaimKey(candidate, caseInsensitive),
	)
	if err != nil || rel == "." || filepath.IsAbs(rel) {
		return false
	}

	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func (m *Manager) symlinkResolvesTo(path, expected string) bool {
	link, err := m.fs.Readlink(path)
	if err != nil {
		return false
	}

	resolved := link
	if !filepath.IsAbs(resolved) {
		resolved = filepath.Join(filepath.Dir(path), resolved)
	}

	caseInsensitive := tmpl.IsCaseInsensitiveFilesystem()
	return tmpl.PathClaimKey(resolved, caseInsensitive) == tmpl.PathClaimKey(expected, caseInsensitive)
}

// restoreTemplateFile renders one selected template and then uses the ordinary
// literal-file deployment path for its suffix-free backup alias. The alias is
// deliberately deployed through restoreLiteralFile rather than RestoreFiles:
// TargetName("config.tmpl.tmpl") is "config.tmpl", which must remain literal
// and must not trigger recursive template dispatch.
func (m *Manager) restoreTemplateFile(subEntry config.SubEntry, source, target, file string) error {
	selection := newSelectedTemplate(source, target, file)
	if err := m.renderTemplateAndLink(selection.templatePath, selection.relPath); err != nil {
		return err
	}

	// renderTemplateAndLink intentionally creates no rendered alias or state
	// record during dry-run, so ordinary deployment must not require it yet.
	if m.DryRun {
		return nil
	}

	literalEntry := subEntry
	literalEntry.Files = []string{selection.aliasRelPath}
	return m.restoreLiteralFile(literalEntry, source, target, selection.aliasRelPath)
}
