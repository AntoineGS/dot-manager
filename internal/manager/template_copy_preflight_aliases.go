package manager

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"

	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

type copyTemplateArtifact struct {
	path        string
	description string
	sudo        bool
}

func (m *Manager) preflightCopyArtifacts(preflight copyTemplatePreflight) error {
	for _, selection := range preflight.selected {
		artifacts := []copyTemplateArtifact{
			{path: selection.conflictPath, description: "template conflict"},
			{
				path:        templateCopyOrphanBackupPath(selection.targetPath),
				description: "orphan target backup",
				sudo:        preflight.sudo,
			},
		}
		for _, artifact := range artifacts {
			snapshot, err := m.readTemplateCopyArtifact(artifact.path, artifact.sudo)
			if err != nil {
				return NewPathError("restore", artifact.path,
					fmt.Errorf("accessing %s: %w", artifact.description, err))
			}
			if snapshot.Exists {
				preflight.artifactSnapshots[artifact.path] = snapshot
			}
		}
	}
	stateSnapshots := make(map[string]templateCopySnapshot, len(preflight.stateArtifacts))
	for _, statePath := range preflight.stateArtifacts {
		snapshot, err := m.readTemplateCopyArtifact(statePath.path, preflight.sudo)
		if err != nil {
			return NewPathError("restore", statePath.path,
				fmt.Errorf("accessing state artifact: %w", err))
		}
		if snapshot.Exists {
			stateSnapshots[statePath.path] = snapshot
		}
	}
	stateSnapshots = coalesceCopyTemplateStateSnapshots(
		stateSnapshots, preflight.stateArtifacts, tmpl.IsCaseInsensitiveFilesystem(),
	)
	for path, snapshot := range stateSnapshots {
		preflight.artifactSnapshots[path] = snapshot
	}
	return nil
}
func (m *Manager) readTemplateCopyArtifact(path string, sudo bool) (templateCopySnapshot, error) {
	info, err := m.fs.Lstat(path)
	if err == nil {
		return copyTemplateArtifactSnapshot(info)
	}
	if errors.Is(err, fs.ErrNotExist) {
		return templateCopySnapshot{}, nil
	}

	useSudo, policyErr := templateCopySudoPolicy(runtime.GOOS, sudo)
	if policyErr != nil {
		return templateCopySnapshot{}, policyErr
	}
	if !useSudo || !isTemplateCopyPermissionError(err) {
		return templateCopySnapshot{}, err
	}

	result, runErr := m.runner.RunWithSudo(
		m.ctx, "stat", "--printf="+templateCopyStatFormat, "--", path,
	)
	if commandErr := templateCopyCommandError("stat artifact", result, runErr); commandErr != nil {
		exists, probeErr := m.probeTemplateCopyArtifact(path)
		if probeErr != nil {
			return templateCopySnapshot{}, errors.Join(commandErr, probeErr)
		}
		if exists {
			return templateCopySnapshot{}, commandErr
		}
		return templateCopySnapshot{}, nil
	}

	metadata, parseErr := parseTemplateCopyStatMetadata(result.Stdout)
	if parseErr != nil {
		return templateCopySnapshot{}, fmt.Errorf("parsing artifact metadata: %w", parseErr)
	}
	return copyTemplateArtifactMetadata(metadata)
}

func copyTemplateArtifactSnapshot(info fs.FileInfo) (templateCopySnapshot, error) {
	if info.Mode()&fs.ModeSymlink != 0 || !info.Mode().IsRegular() {
		return templateCopySnapshot{}, fmt.Errorf("artifact must be a regular file")
	}
	device, inode, identityKnown := fileIdentity(info)
	return templateCopySnapshot{
		Exists:        true,
		Mode:          info.Mode().Perm(),
		RootOwned:     fileRootOwned(info),
		Device:        device,
		Inode:         inode,
		IdentityKnown: identityKnown,
	}, nil
}

func copyTemplateArtifactMetadata(metadata templateCopyStat) (templateCopySnapshot, error) {
	if metadata.FileType != templateCopyRegularFile {
		return templateCopySnapshot{}, fmt.Errorf("artifact must be a regular file")
	}
	return templateCopySnapshot{
		Exists:        true,
		Mode:          metadata.Mode,
		RootOwned:     metadata.RootOwned,
		Device:        metadata.Device,
		Inode:         metadata.Inode,
		IdentityKnown: true,
	}, nil
}

// probeTemplateCopyArtifact positively proves absence after an elevated
// no-follow stat failed. A failed or redirected parent is never treated as an
// absent artifact.
func (m *Manager) probeTemplateCopyArtifact(path string) (bool, error) {
	parent := filepath.Dir(path)
	parentResult, runErr := m.runner.RunWithSudo(
		m.ctx, "stat", "--printf="+templateCopyStatFormat, "--", parent,
	)
	if err := templateCopyCommandError("stat artifact parent", parentResult, runErr); err != nil {
		return false, err
	}
	parentMetadata, err := parseTemplateCopyStatMetadata(parentResult.Stdout)
	if err != nil {
		return false, fmt.Errorf("parsing artifact parent metadata: %w", err)
	}
	if parentMetadata.FileType == templateCopySymlinkFile {
		return false, fmt.Errorf("artifact parent is a symlink")
	}
	if parentMetadata.FileType != templateCopyDirectoryFile {
		return false, fmt.Errorf("artifact parent is not a directory")
	}

	result, runErr := m.runner.RunWithSudo(
		m.ctx,
		"find", "-H", "--", parent, "-mindepth", "1", "-maxdepth", "1", "-print0",
	)
	if err := templateCopyCommandError("find artifact entry", result, runErr); err != nil {
		return false, err
	}
	want := filepath.Clean(path)
	for _, candidate := range bytes.Split(result.Stdout, []byte{0}) {
		if len(candidate) != 0 && filepath.Clean(string(candidate)) == want {
			return true, nil
		}
	}
	return false, nil
}

func (m *Manager) preflightCopyAliases(preflight copyTemplatePreflight) error {
	if err := m.preflightCopyTargetAliases(preflight); err != nil {
		return err
	}
	return m.preflightCopyArtifactAliases(preflight)
}

func (m *Manager) preflightCopyTargetAliases(preflight copyTemplatePreflight) error {
	for _, targetPath := range preflight.targets {
		target, ok := preflight.targetSnapshots[targetPath]
		if !m.copyTargetNeedsAliasChecks(targetPath, target, ok, preflight) {
			continue
		}
		if err := m.preflightCopyTargetAgainstSources(targetPath, target, preflight); err != nil {
			return err
		}
	}
	return m.preflightCopyTargetPairs(preflight)
}

func (m *Manager) copyTargetNeedsAliasChecks(
	path string,
	snapshot templateCopySnapshot,
	found bool,
	preflight copyTemplatePreflight,
) bool {
	return found && snapshot.Exists && !m.isAllowedCopyMigrationTarget(path, snapshot, preflight)
}

func (m *Manager) preflightCopyTargetAgainstSources(
	targetPath string,
	target templateCopySnapshot,
	preflight copyTemplatePreflight,
) error {
	for _, sourcePath := range preflight.sourceFiles {
		source, ok := preflight.sourceSnapshots[sourcePath]
		if ok && m.copyTemplateSnapshotsAlias(targetPath, target, sourcePath, source) {
			return NewPathError("restore", targetPath,
				fmt.Errorf("target aliases a selected source"))
		}
	}
	for artifactPath, artifact := range preflight.artifactSnapshots {
		if m.copyTemplateSnapshotsAlias(targetPath, target, artifactPath, artifact) {
			return NewPathError("restore", targetPath,
				fmt.Errorf("target aliases a generated artifact"))
		}
	}
	if m.copyPathAliasesAny(targetPath, target, preflight.generated) {
		return NewPathError("restore", targetPath,
			fmt.Errorf("target aliases a generated artifact"))
	}
	return nil
}

func (m *Manager) preflightCopyTargetPairs(preflight copyTemplatePreflight) error {
	for i, firstPath := range preflight.targets {
		first, ok := preflight.targetSnapshots[firstPath]
		if !m.copyTargetNeedsAliasChecks(firstPath, first, ok, preflight) {
			continue
		}
		for _, secondPath := range preflight.targets[i+1:] {
			second, ok := preflight.targetSnapshots[secondPath]
			if !m.copyTargetNeedsAliasChecks(secondPath, second, ok, preflight) {
				continue
			}
			if m.copyTemplateSnapshotsAlias(firstPath, first, secondPath, second) {
				return NewPathError("restore", firstPath,
					fmt.Errorf("target aliases another selected target"))
			}
		}
	}
	return nil
}

func (m *Manager) preflightCopyArtifactAliases(preflight copyTemplatePreflight) error {
	for artifactPath, artifact := range preflight.artifactSnapshots {
		for _, sourcePath := range preflight.sourceFiles {
			source, ok := preflight.sourceSnapshots[sourcePath]
			if ok && m.copyTemplateSnapshotsAlias(artifactPath, artifact, sourcePath, source) {
				return NewPathError("restore", artifactPath,
					fmt.Errorf("generated artifact aliases a selected source"))
			}
		}
		for _, targetPath := range preflight.targets {
			target, ok := preflight.targetSnapshots[targetPath]
			if ok && m.copyTemplateSnapshotsAlias(artifactPath, artifact, targetPath, target) {
				return NewPathError("restore", artifactPath,
					fmt.Errorf("generated artifact aliases a target"))
			}
		}
		for otherPath, other := range preflight.artifactSnapshots {
			if artifactPath != otherPath && m.copyTemplateSnapshotsAlias(artifactPath, artifact, otherPath, other) {
				return NewPathError("restore", artifactPath,
					fmt.Errorf("generated artifact aliases another generated artifact"))
			}
		}
		if m.copyPathAliasesAny(artifactPath, artifact, preflight.generated) {
			return NewPathError("restore", artifactPath,
				fmt.Errorf("generated artifact aliases another generated path"))
		}
	}
	return nil
}

func (m *Manager) isAllowedCopyMigrationTarget(
	path string,
	snapshot templateCopySnapshot,
	preflight copyTemplatePreflight,
) bool {
	if !snapshot.Symlink {
		return false
	}
	caseInsensitive := tmpl.IsCaseInsensitiveFilesystem()
	for _, selection := range preflight.selected {
		if tmpl.PathClaimKey(path, caseInsensitive) ==
			tmpl.PathClaimKey(selection.targetPath, caseInsensitive) {
			return true
		}
	}
	return false
}

func (m *Manager) copyTemplateSnapshotsAlias(
	firstPath string,
	first templateCopySnapshot,
	secondPath string,
	second templateCopySnapshot,
) bool {
	if filepath.Clean(firstPath) == filepath.Clean(secondPath) {
		return true
	}
	if !first.Exists || !second.Exists {
		return false
	}
	if first.IdentityKnown && second.IdentityKnown &&
		first.Device == second.Device && first.Inode == second.Inode {
		return true
	}
	return m.sameLocation(firstPath, secondPath)
}

func (m *Manager) copyPathAliasesAny(
	path string,
	snapshot templateCopySnapshot,
	paths ...[]string,
) bool {
	for _, candidates := range paths {
		for _, candidate := range candidates {
			if filepath.Clean(path) == filepath.Clean(candidate) {
				continue
			}
			if m.sameLocation(path, candidate) {
				return true
			}
			if snapshot.IdentityKnown {
				info, err := m.fs.Stat(candidate)
				if err != nil {
					continue
				}
				device, inode, known := fileIdentity(info)
				if known && device == snapshot.Device && inode == snapshot.Inode {
					return true
				}
			}
		}
	}
	return false
}
