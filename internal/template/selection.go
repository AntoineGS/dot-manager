package template

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
)

// IsCaseInsensitiveFilesystem reports whether the host filesystem treats path
// names case-insensitively. This follows the actual host OS, not a configured
// platform override used to select an application's targets.
func IsCaseInsensitiveFilesystem() bool {
	return runtime.GOOS == "windows"
}

// PathClaimKey returns the comparison key for a filesystem path claim.
// caseInsensitive is deliberately explicit so portable tests can exercise
// Windows claim semantics without probing or emulating a whole filesystem.
// This helper is for filesystem collision/alias checks, not render-state keys.
func PathClaimKey(path string, caseInsensitive bool) string {
	cleanPath := filepath.Clean(path)
	if caseInsensitive {
		return strings.ToLower(cleanPath)
	}
	return cleanPath
}

type selectionPathClaim struct {
	path  string
	owner string
}

// ValidateSelectionCollisions rejects source, generated-artifact, alias, and
// effective-target claims that resolve to the same filesystem path.
func ValidateSelectionCollisions(files []string, caseInsensitive bool) error {
	sourceClaims, targetClaims := selectionPathClaims(files)
	if err := validatePathClaims(sourceClaims, caseInsensitive); err != nil {
		return err
	}
	return validatePathClaims(targetClaims, caseInsensitive)
}

// HasSelectionCollisions reports whether a file selection contains any path
// claim collision. It mirrors ValidateSelectionCollisions for status checks,
// which need a boolean rather than a restore error.
func HasSelectionCollisions(files []string, caseInsensitive bool) bool {
	return ValidateSelectionCollisions(files, caseInsensitive) != nil
}

func selectionPathClaims(files []string) (sourceClaims, targetClaims []selectionPathClaim) {
	for _, file := range files {
		cleanPath := filepath.Clean(file)
		if !IsTemplateFile(file) {
			owner := fmt.Sprintf("literal source %q", file)
			sourceClaims = append(sourceClaims, selectionPathClaim{path: cleanPath, owner: owner})
			targetClaims = append(targetClaims, selectionPathClaim{
				path:  cleanPath,
				owner: fmt.Sprintf("literal target %q", file),
			})
			continue
		}

		renderedPath := filepath.Clean(RenderedPath(cleanPath))
		conflictPath := filepath.Clean(ConflictPath(cleanPath))
		backupPath := renderedPath + ".bak"
		tempPath := filepath.Join(filepath.Dir(renderedPath), "."+filepath.Base(renderedPath)+".tidydots-tmp")
		aliasPath := filepath.Join(filepath.Dir(cleanPath), TargetName(cleanPath))
		owner := fmt.Sprintf("template %q", cleanPath)

		sourceClaims = append(sourceClaims,
			selectionPathClaim{path: cleanPath, owner: owner},
			selectionPathClaim{path: renderedPath, owner: owner},
			selectionPathClaim{path: conflictPath, owner: owner},
			selectionPathClaim{path: backupPath, owner: owner},
			selectionPathClaim{path: tempPath, owner: owner},
			selectionPathClaim{path: aliasPath, owner: owner},
		)
		targetClaims = append(targetClaims, selectionPathClaim{path: aliasPath, owner: owner})
	}

	return sourceClaims, targetClaims
}

func validatePathClaims(claims []selectionPathClaim, caseInsensitive bool) error {
	owners := make(map[string]selectionPathClaim, len(claims))
	for _, claim := range claims {
		key := PathClaimKey(claim.path, caseInsensitive)
		if previous, ok := owners[key]; ok {
			return fmt.Errorf("selected path %q is claimed by both %q and %q",
				claim.path, previous.owner, claim.owner)
		}
		owners[key] = claim
	}
	return nil
}
