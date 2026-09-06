package manager

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/platform"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

// CopyTemplateInspection describes the live state of the selected templates in
// a copy entry. Unlike symlink-template inspection, CurrentPath points at the
// deployed target because copy mode has no rendered repository cache.
type CopyTemplateInspection struct {
	NeedsRestore bool
	Outdated     bool
	Modified     []ModifiedTemplate
}

// InspectCopyTemplates inspects selected copy-mode templates without changing
// the filesystem, render history, or invoking a privileged command. Target
// reads are deliberately native-only even when entry.Sudo is true; status must
// never prompt for or acquire privileges.
func (m *Manager) InspectCopyTemplates(
	entry config.SubEntry,
	backupDir, targetDir string,
) (CopyTemplateInspection, error) {
	preflight, err := newCopyTemplatePreflight(entry, backupDir, targetDir)
	if err != nil {
		return CopyTemplateInspection{}, err
	}
	if len(preflight.selected) == 0 {
		return CopyTemplateInspection{}, nil
	}
	statePaths, err := m.copyTemplateStatePaths()
	if err != nil {
		return CopyTemplateInspection{}, NewPathError("status", "state", err)
	}
	preflight.stateArtifacts = statePaths
	if err := m.validateCopyTemplateClaimLayout("status", backupDir, preflight); err != nil {
		return CopyTemplateInspection{}, err
	}
	if err := m.validateCopyTemplateAliasesNative("status", preflight); err != nil {
		return CopyTemplateInspection{}, err
	}

	selections, err := m.copyTemplateSelections(entry, backupDir, targetDir)
	if err != nil {
		return CopyTemplateInspection{}, err
	}
	if len(selections) == 0 {
		return CopyTemplateInspection{}, nil
	}

	inspection := CopyTemplateInspection{}
	for _, selection := range selections {
		if err := m.inspectCopyTemplate(entry, selection, &inspection); err != nil {
			return CopyTemplateInspection{}, err
		}
	}

	return inspection, nil
}

func (m *Manager) copyTemplateSelections(
	entry config.SubEntry,
	backupDir, targetDir string,
) ([]selectedTemplate, error) {
	var selections []selectedTemplate
	for _, file := range entry.Files {
		if !tmpl.IsTemplateFile(file) {
			continue
		}

		relPath, err := validateLocalTemplatePath(file)
		if err != nil {
			return nil, NewPathError("status", file, err)
		}

		selection := newSelectedTemplate(backupDir, targetDir, relPath)
		if selection.aliasRelPath == "." || tmpl.TargetName(relPath) == "" {
			return nil, NewPathError("status", selection.templatePath,
				fmt.Errorf("template has no suffix-free target name"))
		}
		if err := m.validateTemplateInspectionPath(backupDir, selection.templatePath, "template source"); err != nil {
			return nil, err
		}
		if err := m.validateTemplateInspectionPath(targetDir, selection.targetPath, "template target"); err != nil {
			return nil, err
		}

		selections = append(selections, selection)
	}

	if len(selections) == 0 {
		return nil, nil
	}
	if err := tmpl.ValidateCopySelectionCollisions(entry.Files, tmpl.IsCaseInsensitiveFilesystem()); err != nil {
		path := filepath.Clean(entry.Files[0])
		return nil, NewPathError("status", path, err)
	}

	return selections, nil
}

//nolint:gocyclo // inspection keeps source, history, target, and ownership outcomes explicit
func (m *Manager) inspectCopyTemplate(
	entry config.SubEntry,
	selection selectedTemplate,
	inspection *CopyTemplateInspection,
) error {
	sourceInfo, err := m.fs.Lstat(selection.templatePath)
	if err != nil {
		return NewPathError("status", selection.templatePath,
			fmt.Errorf("accessing template source: %w", err))
	}
	if !sourceInfo.Mode().IsRegular() {
		return NewPathError("status", selection.templatePath,
			fmt.Errorf("template source is not a regular file"))
	}

	sourceContent, err := m.fs.ReadFile(selection.templatePath)
	if err != nil {
		return NewPathError("status", selection.templatePath,
			fmt.Errorf("reading template source: %w", err))
	}
	if m.templateEngine == nil {
		return NewPathError("status", selection.templatePath,
			fmt.Errorf("template engine is unavailable"))
	}
	if _, err := m.templateEngine.RenderBytes(selection.relPath, sourceContent); err != nil {
		return NewPathError("status", selection.templatePath,
			fmt.Errorf("rendering template: %w", err))
	}

	history, err := m.templateHistory(selection.templatePath, selection.relPath, selection.targetPath)
	if err != nil {
		return NewPathError("status", selection.templatePath,
			fmt.Errorf("reading render history: %w", err))
	}
	record := history.record
	if record == nil || record.TemplateHash != fmt.Sprintf("%x", sha256.Sum256(sourceContent)) {
		inspection.Outdated = true
	}

	targetExists, targetSymlink, err := m.classifyTemplateCopyTarget(selection.targetPath)
	if err != nil {
		return NewPathError("status", selection.targetPath,
			fmt.Errorf("inspecting copy target: %w", err))
	}
	if !targetExists || targetSymlink {
		inspection.NeedsRestore = true
		return nil
	}

	// Status intentionally passes false here. The inspector must never fall
	// back to sudo, even when the restore entry requests elevated writes.
	target, err := m.readTemplateCopyTarget(selection.targetPath, false)
	if err != nil {
		return NewPathError("status", selection.targetPath,
			fmt.Errorf("inspecting copy target: %w", err))
	}
	if !target.Exists || target.Symlink || copyTemplateTargetNeedsRoot(entry, target) {
		inspection.NeedsRestore = true
		return nil
	}

	if record != nil && !bytes.Equal(target.Content, record.PureRender) {
		inspection.Modified = append(inspection.Modified, ModifiedTemplate{
			TemplatePath:  selection.templatePath,
			CurrentPath:   selection.targetPath,
			RelPath:       selection.relPath,
			PureRender:    record.PureRender,
			CurrentOnDisk: target.Content,
		})
	}

	return nil
}

// classifyTemplateCopyTarget establishes the target's own object type without
// following it. Status treats every link as needing restore, so referent shape
// and readability must not turn a safe migration state into an inspection
// error. Native Lstat errors remain actionable inspection errors.
func (m *Manager) classifyTemplateCopyTarget(path string) (exists, symlink bool, err error) {
	info, err := m.fs.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, false, nil
		}
		return false, false, fmt.Errorf("inspecting target %q: %w", path, err)
	}

	symlink = info.Mode()&fs.ModeSymlink != 0
	if !symlink && runtime.GOOS == platform.OSWindows {
		// Preserve Manager.isSymlink's junction detection for Windows paths that
		// do not expose ModeSymlink through Lstat.
		_, readlinkErr := m.fs.Readlink(path)
		symlink = readlinkErr == nil
	}

	return true, symlink, nil
}

// validateTemplateInspectionPath checks existing parent components without
// swallowing native Lstat failures. Missing parents are valid because the
// target may be absent and therefore restore-needed.
//
//nolint:gocyclo // native path validation has distinct missing, denied, and link branches
func (m *Manager) validateTemplateInspectionPath(root, candidate, description string) error {
	caseInsensitive := tmpl.IsCaseInsensitiveFilesystem()
	cleanRoot := tmpl.PathClaimKey(root, caseInsensitive)
	cleanCandidate := tmpl.PathClaimKey(candidate, caseInsensitive)
	relPath, err := filepath.Rel(cleanRoot, cleanCandidate)
	if err != nil || relPath == ".." || len(relPath) > 3 && relPath[:3] == ".."+string(filepath.Separator) {
		return NewPathError("status", candidate,
			fmt.Errorf("%s escapes the entry directory", description))
	}
	if relPath == "." {
		return nil
	}

	rootInfo, err := m.fs.Lstat(cleanRoot)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil
		}
		return NewPathError("status", candidate,
			fmt.Errorf("accessing %s root %q: %w", description, cleanRoot, err))
	}
	if rootInfo.Mode()&fs.ModeSymlink != 0 || m.isWindowsJunction(cleanRoot) {
		return NewPathError("status", candidate,
			fmt.Errorf("%s has a symlink parent", description))
	}

	for current := filepath.Dir(cleanCandidate); current != cleanRoot; current = filepath.Dir(current) {
		info, err := m.fs.Lstat(current)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				// A missing descendant can still sit below an existing symlink
				// ancestor. Keep walking upward so that status cannot miss the
				// unsafe parent chain before reporting a restore-needed target.
				continue
			}
			return NewPathError("status", candidate,
				fmt.Errorf("accessing %s parent %q: %w", description, current, err))
		}

		if info.Mode()&fs.ModeSymlink != 0 || m.isWindowsJunction(current) {
			return NewPathError("status", candidate,
				fmt.Errorf("%s has a symlink parent", description))
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}

	return nil
}

func (m *Manager) isWindowsJunction(path string) bool {
	if runtime.GOOS != platform.OSWindows {
		return false
	}
	_, err := m.fs.Readlink(path)
	return err == nil
}

func copyTemplateTargetNeedsRoot(entry config.SubEntry, target templateCopySnapshot) bool {
	return entry.Sudo && runtime.GOOS == "linux" && !target.RootOwned
}
