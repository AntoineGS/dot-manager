package manager

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AntoineGS/tidydots/internal/config"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

// copyTemplatePreflight keeps legacy target paths separate from recovery
// backups so status can allow only the former to remain symlinks.
type copyTemplatePreflight struct {
	selected          []selectedTemplate
	sourceFiles       []string
	sources           []string
	targets           []string
	generated         []string
	sourceArtifacts   []string
	targetArtifacts   []string
	legacyTargetPaths []string
	stateArtifacts    []copyTemplateStatePath
	sourceSnapshots   map[string]templateCopySnapshot
	artifactSnapshots map[string]templateCopySnapshot
	targetSnapshots   map[string]templateCopySnapshot
	sudo              bool
}

// preflightCopyTemplateFiles validates all selected templates and their
// filesystem claims before RestoreFiles creates a directory or copies an
// ordinary file. A copy entry with no template selection deliberately returns
// immediately so the ordinary copy path retains its existing behavior.
func (m *Manager) preflightCopyTemplateFiles(entry config.SubEntry, source, target string) error {
	preflight, err := newCopyTemplatePreflight(entry, source, target)
	if err != nil {
		return err
	}
	if len(preflight.selected) == 0 {
		return nil
	}
	statePaths, err := m.copyTemplateStatePaths()
	if err != nil {
		return NewPathError("restore", "state", err)
	}
	preflight.stateArtifacts = statePaths

	if err := templateCopySudoValidation(entry.Sudo); err != nil {
		return err
	}
	if err := tmpl.ValidateCopySelectionCollisions(entry.Files, tmpl.IsCaseInsensitiveFilesystem()); err != nil {
		path := filepath.Clean(entry.Files[0])
		return NewPathError("restore", path, err)
	}
	if err := m.validateCopyTemplateClaimLayout("restore", source, preflight); err != nil {
		return err
	}
	if m.pathAliasesSource(source, target) {
		return NewPathError("restore", target,
			fmt.Errorf("source and target resolve to the same location"))
	}

	if err := m.preflightCopySources(source, preflight); err != nil {
		return err
	}
	if err := m.preflightCopyHistory(preflight); err != nil {
		return err
	}
	if err := m.preflightCopyTargets(entry, target, preflight); err != nil {
		return err
	}
	if err := m.preflightCopyArtifacts(preflight); err != nil {
		return err
	}
	return m.preflightCopyAliases(preflight)
}

func newCopyTemplatePreflight(
	entry config.SubEntry,
	source, target string,
) (copyTemplatePreflight, error) {
	preflight := copyTemplatePreflight{
		sourceSnapshots:   make(map[string]templateCopySnapshot),
		artifactSnapshots: make(map[string]templateCopySnapshot),
		targetSnapshots:   make(map[string]templateCopySnapshot),
		sudo:              entry.Sudo,
	}
	for _, file := range entry.Files {
		if !tmpl.IsTemplateFile(file) {
			continue
		}

		relPath, err := validateLocalTemplatePath(file)
		if err != nil {
			return preflight, NewPathError("restore", file, err)
		}

		selection := newSelectedTemplate(source, target, relPath)
		if selection.aliasRelPath == "." || tmpl.TargetName(relPath) == "" {
			return preflight, NewPathError("restore", selection.templatePath,
				fmt.Errorf("template has no suffix-free target name"))
		}
		preflight.selected = append(preflight.selected, selection)
	}

	if len(preflight.selected) == 0 {
		return preflight, nil
	}

	preflight.sources = make([]string, 0, len(entry.Files)+len(preflight.selected))
	for _, file := range entry.Files {
		cleanPath, err := validateLocalTemplatePath(file)
		if err != nil {
			return preflight, NewPathError("restore", file, err)
		}
		sourcePath := filepath.Join(source, cleanPath)
		preflight.sourceFiles = append(preflight.sourceFiles, sourcePath)
		preflight.sources = append(preflight.sources, sourcePath)
	}
	for _, selection := range preflight.selected {
		preflight.sources = append(preflight.sources, selection.conflictPath)
		preflight.sourceArtifacts = append(preflight.sourceArtifacts,
			selection.aliasPath, selection.renderedPath, selection.conflictPath,
			selection.backupPath, selection.tempPath)
		orphanBackupPath := templateCopyOrphanBackupPath(selection.targetPath)
		legacyTargetPath := filepath.Join(target, selection.relPath)
		preflight.targetArtifacts = append(preflight.targetArtifacts,
			orphanBackupPath, legacyTargetPath)
		preflight.legacyTargetPaths = append(preflight.legacyTargetPaths, legacyTargetPath)
		preflight.generated = append(preflight.generated,
			selection.aliasPath, selection.renderedPath,
			selection.conflictPath, selection.backupPath, selection.tempPath,
			orphanBackupPath, legacyTargetPath,
		)
		preflight.targets = append(preflight.targets, selection.targetPath)
	}
	for _, file := range entry.Files {
		if tmpl.IsTemplateFile(file) {
			continue
		}
		cleanPath, err := validateLocalTemplatePath(file)
		if err != nil {
			return preflight, NewPathError("restore", file, err)
		}
		preflight.targets = append(preflight.targets, filepath.Join(target, cleanPath))
	}
	return preflight, nil
}

func (m *Manager) preflightCopySources(source string, preflight copyTemplatePreflight) error {
	selectedByPath := make(map[string]selectedTemplate, len(preflight.selected))
	for _, selection := range preflight.selected {
		selectedByPath[selection.templatePath] = selection
	}

	for _, sourcePath := range preflight.sourceFiles {
		selection, selected := selectedByPath[sourcePath]
		if selected {
			if err := m.preflightTemplateSource(source, selection); err != nil {
				return err
			}
		}

		snapshot, err := m.readTemplateCopyTarget(sourcePath, preflight.sudo)
		if err != nil {
			return NewPathError("restore", sourcePath,
				fmt.Errorf("inspecting copy source: %w", err))
		}
		if !snapshot.Exists {
			if selected || !m.DryRun {
				return NewPathError("restore", sourcePath, fmt.Errorf(
					"source file does not exist; copy mode does not adopt an existing target — run `tidydots backup` first to pull it into your repo"))
			}
			continue
		}
		preflight.sourceSnapshots[sourcePath] = snapshot
	}

	for _, selection := range preflight.selected {
		if err := m.preflightCopyConflict(selection.conflictPath); err != nil {
			return err
		}
	}
	return nil
}

func (m *Manager) preflightCopyHistory(preflight copyTemplatePreflight) error {
	for _, selection := range preflight.selected {
		if _, err := m.latestCopyTemplateRender(selection.relPath); err != nil {
			return NewPathError("restore", selection.templatePath,
				fmt.Errorf("reading render history: %w", err))
		}
	}
	return nil
}

func (m *Manager) preflightCopyTargets(
	entry config.SubEntry,
	target string,
	preflight copyTemplatePreflight,
) error {
	if err := m.preflightCopyTargetParents(target, entry.Sudo); err != nil {
		return err
	}
	if err := m.preflightCopyEntryTarget(target, entry.Sudo); err != nil {
		return err
	}

	for _, targetPath := range preflight.targets {
		if err := m.preflightCopyTargetParents(targetPath, entry.Sudo); err != nil {
			return err
		}
		snapshot, err := m.readTemplateCopyTarget(targetPath, entry.Sudo)
		if err != nil {
			return NewPathError("restore", targetPath,
				fmt.Errorf("inspecting copy target: %w", err))
		}
		preflight.targetSnapshots[targetPath] = snapshot
	}

	for _, selection := range preflight.selected {
		snapshot := preflight.targetSnapshots[selection.targetPath]
		if err := m.preflightCopyTarget(selection, preflight, snapshot); err != nil {
			return err
		}
	}
	return nil
}

func templateCopySudoValidation(sudo bool) error {
	if _, err := templateCopySudoPolicy(runtime.GOOS, sudo); err != nil {
		return err
	}
	return nil
}

func (m *Manager) preflightCopyConflict(path string) error {
	info, err := m.fs.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return NewPathError("restore", path, fmt.Errorf("accessing template conflict: %w", err))
	}
	if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return NewPathError("restore", path,
			fmt.Errorf("template conflict must be a regular file"))
	}
	return nil
}

func (m *Manager) readCopyTargetLink(path string, sudo bool) (string, error) {
	link, err := m.fs.Readlink(path)
	if err == nil {
		return link, nil
	}
	useSudo, policyErr := templateCopySudoPolicy(runtime.GOOS, sudo)
	if policyErr != nil {
		return "", policyErr
	}
	if !useSudo || !isTemplateCopyPermissionError(err) {
		return "", err
	}

	result, runErr := m.runner.RunWithSudo(m.ctx, "readlink", "--", path)
	if commandErr := templateCopyCommandError("readlink target", result, runErr); commandErr != nil {
		return "", commandErr
	}
	link = strings.TrimSuffix(string(result.Stdout), "\n")
	link = strings.TrimSuffix(link, "\r")
	if link == "" || strings.ContainsAny(link, "\r\n") {
		return "", fmt.Errorf("readlink returned an invalid target")
	}
	return link, nil
}

func (m *Manager) isExpectedCopyMigrationLink(selection selectedTemplate, link string) bool {
	resolved := resolveCopyLink(selection.targetPath, link)
	caseInsensitive := tmpl.IsCaseInsensitiveFilesystem()
	if tmpl.PathClaimKey(resolved, caseInsensitive) == tmpl.PathClaimKey(selection.renderedPath, caseInsensitive) {
		return true
	}
	if tmpl.PathClaimKey(resolved, caseInsensitive) != tmpl.PathClaimKey(selection.aliasPath, caseInsensitive) {
		return false
	}

	aliasLink, err := m.fs.Readlink(selection.aliasPath)
	if err != nil {
		return false
	}
	aliasResolved := resolveCopyLink(selection.aliasPath, aliasLink)
	return tmpl.PathClaimKey(aliasResolved, caseInsensitive) ==
		tmpl.PathClaimKey(selection.renderedPath, caseInsensitive)
}

func resolveCopyLink(path, link string) string {
	if filepath.IsAbs(link) {
		return filepath.Clean(link)
	}
	return filepath.Clean(filepath.Join(filepath.Dir(path), link))
}
