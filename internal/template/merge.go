package template

import (
	"fmt"
	"strings"
)

// MergeResult holds the outcome of a 3-way merge.
type MergeResult struct {
	Content     string
	HasConflict bool
}

// ThreeWayMerge performs a line-based 3-way merge.
//
//   - base: previous pure render from DB (no user edits)
//   - theirs: current target file on disk (may have user edits)
//   - ours: newly rendered template output
//
// Fast paths:
//   - base==theirs: no user edits, use ours
//   - base==ours: no template changes, keep theirs
//   - theirs==ours: same result either way, use ours
func ThreeWayMerge(base, theirs, ours string) MergeResult {
	// Fast paths
	if base == theirs {
		return MergeResult{Content: ours}
	}
	if base == ours {
		return MergeResult{Content: theirs}
	}
	if theirs == ours {
		return MergeResult{Content: ours}
	}

	baseLines := splitLines(base)
	theirLines := splitLines(theirs)
	ourLines := splitLines(ours)

	return mergeLines(baseLines, theirLines, ourLines)
}

// splitLines splits text into lines, preserving the trailing newline information.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(s, "\n")
}

// mergeLines performs a positional line-by-line 3-way merge.
//
//nolint:gocyclo // complexity is necessary for comprehensive 3-way merge logic
func mergeLines(base, theirs, ours []string) MergeResult {
	maxLen := len(base)
	if len(theirs) > maxLen {
		maxLen = len(theirs)
	}
	if len(ours) > maxLen {
		maxLen = len(ours)
	}

	var result []string
	hasConflict := false

	i := 0
	for i < maxLen {
		baseLine := getLine(base, i)
		theirLine := getLine(theirs, i)
		ourLine := getLine(ours, i)

		baseExists := i < len(base)
		theirExists := i < len(theirs)
		ourExists := i < len(ours)

		switch {
		case baseExists && theirExists && ourExists:
			//nolint:gocritic // if-else chain is more readable than switch for merge logic
			if baseLine == theirLine && theirLine == ourLine {
				// All agree
				result = append(result, baseLine)
			} else if baseLine == theirLine {
				// Only template changed
				result = append(result, ourLine)
			} else if baseLine == ourLine {
				// Only user changed
				result = append(result, theirLine)
			} else if theirLine == ourLine {
				// Both changed the same way
				result = append(result, ourLine)
			} else {
				// Conflict: both changed differently
				hasConflict = true
				result = append(result,
					"<<<<<<< user-edits",
					theirLine,
					"=======",
					ourLine,
					">>>>>>> template",
				)
			}

		case !baseExists && theirExists && ourExists:
			// Line added in both theirs and ours (beyond base)
			if theirLine == ourLine {
				result = append(result, ourLine)
			} else {
				hasConflict = true
				result = append(result,
					"<<<<<<< user-edits",
					theirLine,
					"=======",
					ourLine,
					">>>>>>> template",
				)
			}

		case baseExists && !theirExists && ourExists:
			// User deleted the line, template still has it
			if baseLine != ourLine {
				// User deleted, template changed -> conflict
				hasConflict = true
				result = append(result,
					"<<<<<<< user-edits",
					fmt.Sprintf("(line deleted by user, was: %s)", baseLine),
					"=======",
					ourLine,
					">>>>>>> template",
				)
				// else: User deleted, template unchanged -> honor user deletion (no append)
			}

		case baseExists && theirExists && !ourExists:
			// Template removed the line, user still has it
			if baseLine != theirLine {
				// Template removed, user changed -> keep user's change
				result = append(result, theirLine)
				// else: Template removed, user unchanged -> honor template deletion (no append)
			}

		case !baseExists && theirExists && !ourExists:
			// User added a line beyond both base and ours
			result = append(result, theirLine)

		case !baseExists && !theirExists && ourExists:
			// Template added a line beyond both base and theirs
			result = append(result, ourLine)

		case baseExists && !theirExists && !ourExists:
			// Both removed the line -> it's gone (no append)
		}

		i++
	}

	return MergeResult{
		Content:     strings.Join(result, "\n"),
		HasConflict: hasConflict,
	}
}

func getLine(lines []string, idx int) string {
	if idx < len(lines) {
		return lines[idx]
	}
	return ""
}
