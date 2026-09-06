// Package detection provides pure detection functions for the TUI package.
// These functions have no dependency on the Model type and can be safely
// called from goroutines.
package detection

import (
	"bytes"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	tmpl "github.com/AntoineGS/tidydots/internal/template"
	tuitable "github.com/AntoineGS/tidydots/internal/tui/table"
)

// pathExists reports whether the path exists on the filesystem, using os.Lstat
// so that broken symlinks are still reported as existing.
func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// DetectConfigState determines the state of a config entry given its paths and file list.
// This is a pure function that takes paths and returns a PathState. It uses only
// filesystem path operations and does NOT reference Model.
func DetectConfigState(backupPath, targetPath string, isFolder bool, files []string, isCopy bool) tuitable.PathState {
	return detectConfigStateForCase(backupPath, targetPath, isFolder, files, isCopy,
		tmpl.IsCaseInsensitiveFilesystem())
}

func detectConfigStateForCase(backupPath, targetPath string, isFolder bool, files []string, isCopy, caseInsensitive bool) tuitable.PathState {
	if isFolder {
		if info, err := os.Lstat(targetPath); err == nil {
			if info.Mode()&os.ModeSymlink != 0 {
				return tuitable.StateLinked
			}
		}

		backupExists := pathExists(backupPath)
		targetExists := pathExists(targetPath)

		if backupExists {
			return tuitable.StateReady
		}

		if targetExists {
			return tuitable.StateAdopt
		}

		return tuitable.StateMissing
	}

	// File-based config
	allLinked := true
	anyBackup := false
	anyTarget := false
	checkedAnyFile := false
	hasTemplateSelection := false
	for _, file := range files {
		if !isCopy && tmpl.IsTemplateFile(file) {
			hasTemplateSelection = true
			break
		}
	}
	if hasTemplateSelection && tmpl.HasSelectionCollisions(files, caseInsensitive) {
		// Restore preflight rejects the whole selection. Do not present an
		// independently healthy link layout as actionable-free status.
		allLinked = false
	}

	for _, file := range files {
		isSelectedTemplate := !isCopy && tmpl.IsTemplateFile(file)
		if !isLocalTemplateSelection(backupPath, targetPath, file, isCopy) {
			if isSelectedTemplate {
				allLinked = false
			}
			continue
		}

		srcFile := filepath.Join(backupPath, file)
		targetName := file
		if isSelectedTemplate {
			file = filepath.Clean(file)
			targetName = filepath.Join(filepath.Dir(file), tmpl.TargetName(file))
		}
		dstFile := filepath.Join(targetPath, targetName)

		if !pathExists(srcFile) {
			if isSelectedTemplate {
				allLinked = false
			}
			continue
		}
		if isSelectedTemplate {
			info, err := os.Lstat(srcFile)
			if err != nil || !info.Mode().IsRegular() {
				allLinked = false
				continue
			}
		}

		checkedAnyFile = true
		anyBackup = true

		if info, err := os.Lstat(dstFile); err == nil {
			anyTarget = true
			if isCopy {
				// os.ReadFile follows symlinks, so a stale symlink pointing back
				// into the backup would compare equal to its own source and be
				// misreported as in sync. A copy target must be a real file.
				if info.Mode()&os.ModeSymlink != 0 || !filesContentEqual(srcFile, dstFile) {
					allLinked = false
				}
			} else if isSelectedTemplate {
				if !templateSymlinkChainMatches(backupPath, file, dstFile) {
					allLinked = false
				}
			} else if info.Mode()&os.ModeSymlink == 0 {
				allLinked = false
			}
		} else {
			allLinked = false
		}
	}

	if allLinked && checkedAnyFile {
		return tuitable.StateLinked
	}

	if anyBackup {
		return tuitable.StateReady
	}

	if anyTarget {
		return tuitable.StateAdopt
	}

	return tuitable.StateMissing
}

// isLocalTemplateSelection applies the same entry-boundary checks as selected
// template restore. Copy-mode and literal selections intentionally bypass this
// helper to preserve their existing target mapping behavior.
func isLocalTemplateSelection(backupPath, targetPath, file string, isCopy bool) bool {
	if isCopy || !tmpl.IsTemplateFile(file) {
		return true
	}

	if filepath.IsAbs(file) || filepath.VolumeName(file) != "" {
		return false
	}

	cleanFile := filepath.Clean(file)
	if cleanFile == "." || !isWithinPath(".", cleanFile) {
		return false
	}

	targetName := filepath.Join(filepath.Dir(cleanFile), tmpl.TargetName(cleanFile))
	if targetName == "." || !isWithinPath(".", targetName) {
		return false
	}

	sourcePath := filepath.Join(backupPath, cleanFile)
	targetRoot := targetPath
	targetPath = filepath.Join(targetRoot, targetName)
	return !pathIsSymlink(sourcePath) &&
		!hasSymlinkParent(backupPath, sourcePath) && !hasSymlinkParent(targetRoot, targetPath)
}

func isWithinPath(base, candidate string) bool {
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

func hasSymlinkParent(root, candidate string) bool {
	caseInsensitive := tmpl.IsCaseInsensitiveFilesystem()
	cleanRoot := tmpl.PathClaimKey(root, caseInsensitive)
	cleanCandidate := tmpl.PathClaimKey(candidate, caseInsensitive)
	for current := filepath.Dir(cleanCandidate); current != cleanRoot; current = filepath.Dir(current) {
		if pathIsSymlink(current) {
			return true
		}

		parent := filepath.Dir(current)
		if parent == current {
			break
		}
	}

	return false
}

func pathIsSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return true
	}

	if runtime.GOOS == "windows" {
		_, err := os.Readlink(path)
		return err == nil
	}

	return false
}

// templateSymlinkChainMatches verifies the two links produced for an
// explicitly selected template. A missing rendered file is left to template
// status detection so it can be reported as StateOutdated; a missing or
// unrelated link in either hop is not a linked config.
func templateSymlinkChainMatches(backupPath, file, targetPath string) bool {
	targetName := filepath.Join(filepath.Dir(file), tmpl.TargetName(file))
	aliasPath := filepath.Join(backupPath, targetName)
	templatePath := filepath.Join(backupPath, file)
	renderedPath := tmpl.RenderedPath(templatePath)

	if !symlinkResolvesTo(targetPath, aliasPath) {
		return false
	}

	info, err := os.Lstat(aliasPath)
	if err != nil || info.Mode()&os.ModeSymlink == 0 {
		return false
	}

	return symlinkResolvesTo(aliasPath, renderedPath)
}

func symlinkResolvesTo(path, expected string) bool {
	link, err := os.Readlink(path)
	if err != nil {
		return false
	}
	if !filepath.IsAbs(link) {
		link = filepath.Join(filepath.Dir(path), link)
	}
	return tmpl.PathClaimKey(link, tmpl.IsCaseInsensitiveFilesystem()) ==
		tmpl.PathClaimKey(expected, tmpl.IsCaseInsensitiveFilesystem())
}

// filesContentEqual reports whether two files have identical contents. Any read
// error (missing or unreadable file) counts as not equal.
func filesContentEqual(a, b string) bool {
	da, err := os.ReadFile(a)
	if err != nil {
		return false
	}
	db, err := os.ReadFile(b)
	if err != nil {
		return false
	}
	return bytes.Equal(da, db)
}
