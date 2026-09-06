package manager

import (
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/AntoineGS/tidydots/internal/config"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

type copyTemplateClaim struct {
	path        string
	description string
}

// validateCopyTemplateClaimLayout rejects concrete target/artifact claims that
// overlap the repository, selected sources, generated paths, or state files.
// It intentionally does not reject a target root above a repository (for
// example $HOME with $HOME/dotfiles); only concrete files are claims.
func (m *Manager) validateCopyTemplateClaimLayout(
	operation, source string, preflight copyTemplatePreflight,
) error {
	caseInsensitive := tmpl.IsCaseInsensitiveFilesystem()
	sourceRoots := []copyTemplateClaim{{path: source, description: "source tree"}}
	if repository := m.copyTemplateRepositoryRoot(); repository != "" {
		sourceRoots = append(sourceRoots, copyTemplateClaim{
			path:        repository,
			description: "repository tree",
		})
	}

	sourceClaims := copyTemplateClaims(preflight.sourceFiles, "selected source")
	sourceArtifacts := copyTemplateClaims(preflight.sourceArtifacts, "generated source artifact")
	targetClaims := copyTemplateClaims(preflight.targets, "deployed target")
	targetArtifacts := copyTemplateClaims(preflight.targetArtifacts, "generated target artifact")
	stateClaims := copyTemplateClaims(copyTemplateStateArtifactPaths(preflight.stateArtifacts), "state database artifact")

	allTargetClaims := append(append([]copyTemplateClaim{}, targetClaims...), targetArtifacts...)
	if err := validateCopyTemplateClaimRoots(operation, allTargetClaims, sourceRoots, caseInsensitive); err != nil {
		return err
	}
	if err := validateCopyTemplateClaimGroups(operation, targetClaims, caseInsensitive,
		sourceClaims, sourceArtifacts, stateClaims); err != nil {
		return err
	}
	if err := validateCopyTemplateClaimGroups(operation, targetArtifacts, caseInsensitive,
		sourceClaims, sourceArtifacts, stateClaims); err != nil {
		return err
	}
	if err := validateCopyTemplateClaimPairs(operation, targetClaims, caseInsensitive); err != nil {
		return err
	}
	if err := validateCopyTemplateClaimPairs(operation, append(targetClaims, targetArtifacts...), caseInsensitive); err != nil {
		return err
	}

	if err := validateCopyTemplateClaimGroups(operation, sourceClaims, caseInsensitive, stateClaims); err != nil {
		return err
	}
	if err := validateCopyTemplateClaimGroups(operation, sourceArtifacts, caseInsensitive,
		sourceClaims, stateClaims); err != nil {
		return err
	}
	if err := validateCopyTemplateClaimPairs(operation, sourceArtifacts, caseInsensitive); err != nil {
		return err
	}

	return nil
}

func validateCopyTemplateClaimRoots(
	operation string, claims, roots []copyTemplateClaim, caseInsensitive bool,
) error {
	for _, claim := range claims {
		for _, root := range roots {
			if copyTemplateClaimsOverlap(claim.path, root.path, caseInsensitive) {
				return copyTemplateClaimError(operation, claim,
					fmt.Sprintf("overlaps the protected %s", root.description))
			}
		}
	}
	return nil
}

func validateCopyTemplateClaimGroups(
	operation string, claims []copyTemplateClaim, caseInsensitive bool,
	protectedGroups ...[]copyTemplateClaim,
) error {
	for _, claim := range claims {
		for _, protected := range protectedGroups {
			if err := validateCopyTemplateClaimAgainst(operation, claim, protected, caseInsensitive); err != nil {
				return err
			}
		}
	}
	return nil
}

func (m *Manager) copyTemplateRepositoryRoot() string {
	if m.Config == nil || m.Config.BackupRoot == "" {
		return ""
	}
	envVars := map[string]string{}
	if m.Platform != nil {
		envVars = m.Platform.EnvVars
	}
	return config.ExpandPathWithTemplate(m.Config.BackupRoot, envVars, m.templateEngine)
}

func copyTemplateClaims(paths []string, description string) []copyTemplateClaim {
	claims := make([]copyTemplateClaim, 0, len(paths))
	seen := make(map[string]struct{}, len(paths))
	caseInsensitive := tmpl.IsCaseInsensitiveFilesystem()
	for _, path := range paths {
		if path == "" {
			continue
		}
		key := copyTemplateClaimKey(path, caseInsensitive)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		claims = append(claims, copyTemplateClaim{path: filepath.Clean(path), description: description})
	}
	return claims
}

func validateCopyTemplateClaimAgainst(
	operation string,
	claim copyTemplateClaim,
	protected []copyTemplateClaim,
	caseInsensitive bool,
) error {
	for _, other := range protected {
		if copyTemplateClaimsOverlap(claim.path, other.path, caseInsensitive) {
			return copyTemplateClaimError(operation, claim,
				fmt.Sprintf("overlaps the protected %s", other.description))
		}
	}
	return nil
}

func validateCopyTemplateClaimPairs(
	operation string, claims []copyTemplateClaim, caseInsensitive bool,
) error {
	for i, claim := range claims {
		for _, other := range claims[:i] {
			if copyTemplateClaimsOverlap(claim.path, other.path, caseInsensitive) {
				return copyTemplateClaimError(operation, claim,
					fmt.Sprintf("overlaps another %s", other.description))
			}
		}
	}
	return nil
}

func copyTemplateClaimError(operation string, claim copyTemplateClaim, reason string) error {
	return NewPathError(operation, claim.path, errors.New(reason))
}

func copyTemplateClaimKey(path string, caseInsensitive bool) string {
	absolute, err := filepath.Abs(path)
	if err == nil {
		path = absolute
	}
	return tmpl.PathClaimKey(path, caseInsensitive)
}

func copyTemplateClaimsOverlap(first, second string, caseInsensitive bool) bool {
	first = copyTemplateClaimKey(first, caseInsensitive)
	second = copyTemplateClaimKey(second, caseInsensitive)
	if first == second {
		return true
	}

	return copyTemplateClaimContains(first, second) || copyTemplateClaimContains(second, first)
}

func copyTemplateClaimContains(parent, child string) bool {
	relative, err := filepath.Rel(parent, child)
	if err != nil || relative == "." || filepath.IsAbs(relative) {
		return false
	}
	return relative != ".." && !strings.HasPrefix(relative, ".."+string(filepath.Separator))
}

// validateCopyTemplateAliasesNative is the read-only, native-only alias check
// used by status. It uses Lstat and native file identities, with a same-file
// fallback for existing regular files when identity metadata is unavailable;
// it never follows a status target symlink or invokes a privileged command.
func (m *Manager) validateCopyTemplateAliasesNative(
	operation string, preflight copyTemplatePreflight,
) error {
	caseInsensitive := tmpl.IsCaseInsensitiveFilesystem()
	sources, err := m.nativeCopyTemplateClaimSnapshots(preflight.sourceFiles, nil)
	if err != nil {
		return NewPathError(operation, err.path, err.err)
	}
	legacyAliases := make(map[string]bool, len(preflight.selected))
	for _, selection := range preflight.selected {
		legacyAliases[selection.aliasPath] = true
	}
	artifacts, err := m.nativeCopyTemplateClaimSnapshots(preflight.sourceArtifacts, legacyAliases)
	if err != nil {
		return NewPathError(operation, err.path, err.err)
	}
	legacyTargets := make(map[string]bool, len(preflight.legacyTargetPaths))
	for _, path := range preflight.legacyTargetPaths {
		legacyTargets[path] = true
	}
	targetArtifacts, err := m.nativeCopyTemplateClaimSnapshots(preflight.targetArtifacts, legacyTargets)
	if err != nil {
		return NewPathError(operation, err.path, err.err)
	}
	for path, snapshot := range targetArtifacts {
		artifacts[path] = snapshot
	}
	stateSnapshots, err := m.nativeCopyTemplateClaimSnapshots(
		copyTemplateStateArtifactPaths(preflight.stateArtifacts), nil,
	)
	if err != nil {
		return NewPathError(operation, err.path, err.err)
	}
	stateSnapshots = coalesceCopyTemplateStateSnapshots(
		stateSnapshots, preflight.stateArtifacts, caseInsensitive,
	)
	for path, snapshot := range stateSnapshots {
		artifacts[path] = snapshot
	}
	targetSymlinks := make(map[string]bool, len(preflight.targets))
	for _, path := range preflight.targets {
		targetSymlinks[path] = true
	}
	targets, err := m.nativeCopyTemplateClaimSnapshots(preflight.targets, targetSymlinks)
	if err != nil {
		return NewPathError(operation, err.path, err.err)
	}

	if err := m.validateCopyTemplateNativeAliasGroup(operation, targets, sources,
		"target aliases selected source", caseInsensitive); err != nil {
		return err
	}
	if err := m.validateCopyTemplateNativeAliasGroup(operation, targets, artifacts,
		"target aliases protected artifact", caseInsensitive); err != nil {
		return err
	}
	if err := m.validateCopyTemplateNativeAliasPairs(operation, artifacts,
		"protected artifact aliases", caseInsensitive); err != nil {
		return err
	}
	if err := m.validateCopyTemplateNativeAliasGroup(operation, artifacts, sources,
		"protected artifact aliases selected source", caseInsensitive); err != nil {
		return err
	}
	return m.validateCopyTemplateNativeAliasPairs(operation, targets,
		"target aliases another deployed target", caseInsensitive)
}

func (m *Manager) validateCopyTemplateNativeAliasGroup(
	operation string, claims, protected map[string]templateCopySnapshot,
	reason string, caseInsensitive bool,
) error {
	for claimPath, claim := range claims {
		for protectedPath, protectedSnapshot := range protected {
			if m.copyTemplateNativeSnapshotsAlias(claimPath, claim, protectedPath, protectedSnapshot, caseInsensitive) {
				return NewPathError(operation, claimPath, fmt.Errorf("%s %q", reason, protectedPath))
			}
		}
	}
	return nil
}

func (m *Manager) validateCopyTemplateNativeAliasPairs(
	operation string, snapshots map[string]templateCopySnapshot,
	reason string, caseInsensitive bool,
) error {
	paths := make([]string, 0, len(snapshots))
	for path := range snapshots {
		paths = append(paths, path)
	}
	for i, firstPath := range paths {
		for _, secondPath := range paths[:i] {
			if m.copyTemplateNativeSnapshotsAlias(firstPath, snapshots[firstPath], secondPath, snapshots[secondPath], caseInsensitive) {
				return NewPathError(operation, firstPath, fmt.Errorf("%s %q", reason, secondPath))
			}
		}
	}
	return nil
}

func (m *Manager) nativeCopyTemplateClaimSnapshots(
	paths []string, allowedSymlinks map[string]bool,
) (map[string]templateCopySnapshot, *copyTemplateNativeClaimError) {
	snapshots := make(map[string]templateCopySnapshot, len(paths))
	for _, path := range paths {
		info, err := m.fs.Lstat(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				continue
			}
			return nil, &copyTemplateNativeClaimError{path: path, err: err}
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			if allowedSymlinks[path] {
				snapshots[path] = templateCopySnapshot{Exists: true, Symlink: true}
				continue
			}
			return nil, &copyTemplateNativeClaimError{path: path, err: fmt.Errorf("path is a symlink")}
		}
		if !info.Mode().IsRegular() {
			return nil, &copyTemplateNativeClaimError{path: path, err: fmt.Errorf("path is not a regular file")}
		}
		device, inode, known := fileIdentity(info)
		snapshots[path] = templateCopySnapshot{
			Exists:        true,
			Mode:          info.Mode().Perm(),
			Device:        device,
			Inode:         inode,
			IdentityKnown: known,
		}
	}
	return snapshots, nil
}

type copyTemplateNativeClaimError struct {
	path string
	err  error
}

func (m *Manager) copyTemplateNativeSnapshotsAlias(
	firstPath string,
	first templateCopySnapshot,
	secondPath string,
	second templateCopySnapshot,
	caseInsensitive bool,
) bool {
	if copyTemplateClaimKey(firstPath, caseInsensitive) == copyTemplateClaimKey(secondPath, caseInsensitive) {
		return true
	}
	if !first.Exists || !second.Exists || first.Symlink || second.Symlink {
		return false
	}
	if first.IdentityKnown && second.IdentityKnown {
		return first.Device == second.Device && first.Inode == second.Inode
	}

	// Windows FileInfo values do not expose the Unix device/inode tuple. For
	// existing regular claims, use the native filesystem's same-file check as
	// the identity fallback. Symlink claims are excluded above so this does not
	// follow an allowed legacy or target migration link merely for comparison.
	return m.sameLocation(firstPath, secondPath)
}

func coalesceCopyTemplateStateSnapshots(
	snapshots map[string]templateCopySnapshot,
	statePaths []copyTemplateStatePath,
	caseInsensitive bool,
) map[string]templateCopySnapshot {
	artifactIDs := make(map[string]string, len(statePaths))
	for _, statePath := range statePaths {
		artifactIDs[copyTemplateClaimKey(statePath.path, caseInsensitive)] = statePath.artifactID
	}

	coalesced := make(map[string]templateCopySnapshot, len(snapshots))
	coalescedIDs := make(map[string]string, len(snapshots))
	for path, snapshot := range snapshots {
		duplicate := false
		artifactID := artifactIDs[copyTemplateClaimKey(path, caseInsensitive)]
		for existingPath := range coalesced {
			if artifactID != "" && artifactID == coalescedIDs[existingPath] {
				duplicate = true
				break
			}
		}
		if !duplicate {
			coalesced[path] = snapshot
			coalescedIDs[path] = artifactID
		}
	}
	return coalesced
}
