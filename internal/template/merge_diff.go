package template

import (
	"strings"

	"github.com/sergi/go-diff/diffmatchpatch"
)

// lineEdit replaces the half-open base interval [start,end) with text.
// An insertion has start == end. Edits from each side are ordered and disjoint.
type lineEdit struct {
	start, end   int
	text         string
	templateSide bool
}

// maxDetailedDiffLines bounds combined unresolved input to Myers' bisect. Its
// worst-case edit-distance work can be quadratic even without an LCS table.
// Exact common prefixes/suffixes are removed before applying this limit, so a
// small local edit or insertion remains precise even in a very large file.
const maxDetailedDiffLines = 4096

// lineEdits preserves complete line bytes and deterministic alignment. Large
// unresolved middles become one lossless replacement, potentially broadening
// conflicts. Unlike a timeout, this policy does not depend on machine load.
func lineEdits(base, changed string, templateSide bool) []lineEdit {
	baseLines, changedLines := splitLines(base), splitLines(changed)
	prefix := 0
	for prefix < len(baseLines) && prefix < len(changedLines) && baseLines[prefix] == changedLines[prefix] {
		prefix++
	}
	baseEnd, changedEnd := len(baseLines), len(changedLines)
	for baseEnd > prefix && changedEnd > prefix && baseLines[baseEnd-1] == changedLines[changedEnd-1] {
		baseEnd--
		changedEnd--
	}
	if baseEnd == prefix && changedEnd == prefix {
		return nil
	}
	replacementText := strings.Join(changedLines[prefix:changedEnd], "")
	if baseEnd == prefix || changedEnd == prefix || baseEnd-prefix+changedEnd-prefix > maxDetailedDiffLines {
		return []lineEdit{{start: prefix, end: baseEnd, text: replacementText, templateSide: templateSide}}
	}
	return detailedLineEdits(strings.Join(baseLines[prefix:baseEnd], ""), replacementText, prefix, templateSide)
}

// Only bounded middles reach go-diff. The size guard also keeps line IDs within
// its Unicode token encoder's finite range. Disable character-level line-mode
// refinement and timeouts: the input bound, not a clock, limits diff work.
func detailedLineEdits(base, changed string, offset int, templateSide bool) []lineEdit {
	dmp := diffmatchpatch.New()
	dmp.DiffTimeout = 0
	baseTokens, changedTokens, lines := dmp.DiffLinesToRunes(base, changed)
	diffs := dmp.DiffCharsToLines(dmp.DiffMainRunes(baseTokens, changedTokens, false), lines)
	var edits []lineEdit
	var replacement strings.Builder
	position, start := offset, -1
	flush := func() {
		if start >= 0 {
			edits = append(edits, lineEdit{start: start, end: position, text: replacement.String(), templateSide: templateSide})
			replacement.Reset()
			start = -1
		}
	}
	for _, diff := range diffs {
		if diff.Type == diffmatchpatch.DiffEqual {
			flush()
			position += len(splitLines(diff.Text))
			continue
		}
		if start < 0 {
			start = position
		}
		if diff.Type == diffmatchpatch.DiffDelete {
			position += len(splitLines(diff.Text))
		} else {
			replacement.WriteString(diff.Text)
		}
	}
	flush()
	return edits
}
