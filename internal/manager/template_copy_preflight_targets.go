package manager

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"
)

// preflightCopyTargetParents validates every component from the filesystem
// root through the destination parent. Stopping at the first existing
// directory is unsafe: a symlink ancestor can redirect that directory's
// descendants elsewhere.
func (m *Manager) preflightCopyTargetParents(path string, sudo bool) error {
	useSudo, err := templateCopySudoPolicy(runtime.GOOS, sudo)
	if err != nil {
		return err
	}

	parent := filepath.Dir(filepath.Clean(path))
	absoluteParent, err := filepath.Abs(parent)
	if err != nil {
		return NewPathError("restore", parent, fmt.Errorf("resolving target parent: %w", err))
	}
	for _, component := range copyTemplatePathComponents(absoluteParent) {
		if err := m.preflightCopyParentComponent(component, useSudo); err != nil {
			return err
		}
	}
	return nil
}

func copyTemplatePathComponents(path string) []string {
	clean := filepath.Clean(path)
	volume := filepath.VolumeName(clean)
	rest := strings.TrimPrefix(clean, volume)
	separator := string(filepath.Separator)
	components := make([]string, 0)
	current := volume
	if filepath.IsAbs(clean) {
		current += separator
		components = append(components, filepath.Clean(current))
		rest = strings.TrimPrefix(rest, separator)
	}
	for _, part := range strings.Split(rest, separator) {
		if part == "" || part == "." {
			continue
		}
		current = filepath.Join(current, part)
		components = append(components, current)
	}
	return components
}

func (m *Manager) preflightCopyParentComponent(path string, useSudo bool) error {
	info, err := m.fs.Lstat(path)
	if err == nil {
		if m.isSymlink(path) {
			return NewPathError("restore", path, fmt.Errorf("target parent is a symlink"))
		}
		if !info.IsDir() {
			return NewPathError("restore", path, fmt.Errorf("target parent is not a directory"))
		}
		return nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if !useSudo || !isTemplateCopyPermissionError(err) {
		return NewPathError("restore", path,
			fmt.Errorf("accessing target parent: %w", err))
	}

	result, runErr := m.runner.RunWithSudo(
		m.ctx, "stat", "--printf="+templateCopyStatFormat, "--", path,
	)
	if commandErr := templateCopyCommandError("stat target parent", result, runErr); commandErr != nil {
		return NewPathError("restore", path,
			fmt.Errorf("verifying target parent: %w", commandErr))
	}
	metadata, parseErr := parseTemplateCopyStatMetadata(result.Stdout)
	if parseErr != nil {
		return NewPathError("restore", path,
			fmt.Errorf("parsing target parent metadata: %w", parseErr))
	}
	if metadata.FileType == templateCopySymlinkFile {
		return NewPathError("restore", path, fmt.Errorf("target parent is a symlink"))
	}
	if metadata.FileType != templateCopyDirectoryFile {
		return NewPathError("restore", path, fmt.Errorf("target parent is not a directory"))
	}
	return nil
}

func (m *Manager) preflightCopyEntryTarget(path string, sudo bool) error {
	info, err := m.fs.Lstat(path)
	if err == nil {
		if m.isSymlink(path) {
			return NewPathError("restore", path, fmt.Errorf("copy target directory is a symlink"))
		}
		if !info.IsDir() {
			return NewPathError("restore", path, fmt.Errorf("copy target is not a directory"))
		}
		return nil
	}
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}

	useSudo, policyErr := templateCopySudoPolicy(runtime.GOOS, sudo)
	if policyErr != nil {
		return policyErr
	}
	if !useSudo || !isTemplateCopyPermissionError(err) {
		return NewPathError("restore", path, fmt.Errorf("accessing copy target: %w", err))
	}
	result, runErr := m.runner.RunWithSudo(
		m.ctx, "stat", "--printf="+templateCopyStatFormat, "--", path,
	)
	if commandErr := templateCopyCommandError("stat copy target", result, runErr); commandErr != nil {
		return NewPathError("restore", path,
			fmt.Errorf("verifying copy target: %w", commandErr))
	}
	metadata, parseErr := parseTemplateCopyStatMetadata(result.Stdout)
	if parseErr != nil {
		return NewPathError("restore", path,
			fmt.Errorf("parsing copy target metadata: %w", parseErr))
	}
	if metadata.FileType == templateCopySymlinkFile {
		return NewPathError("restore", path, fmt.Errorf("copy target directory is a symlink"))
	}
	if metadata.FileType != templateCopyDirectoryFile {
		return NewPathError("restore", path, fmt.Errorf("copy target is not a directory"))
	}
	return nil
}

func (m *Manager) preflightCopyTarget(
	selection selectedTemplate,
	preflight copyTemplatePreflight,
	snapshot templateCopySnapshot,
) error {
	if snapshot.Symlink {
		if !snapshot.Exists {
			return nil
		}
		link, linkErr := m.readCopyTargetLink(selection.targetPath, preflight.sudo)
		if linkErr != nil {
			return NewPathError("restore", selection.targetPath,
				fmt.Errorf("reading target symlink: %w", linkErr))
		}
		if !m.isExpectedCopyMigrationLink(selection, link) {
			return NewPathError("restore", selection.targetPath,
				fmt.Errorf("target symlink is not the expected template migration chain"))
		}
		return nil
	}

	if snapshot.Exists && m.copyPathAliasesAny(selection.targetPath, snapshot, preflight.sources, preflight.generated) {
		return NewPathError("restore", selection.targetPath,
			fmt.Errorf("target aliases a source or generated artifact"))
	}
	return nil
}
