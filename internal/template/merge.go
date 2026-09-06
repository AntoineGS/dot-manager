package template

import (
	"cmp"
	"slices"
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
// Independent base-relative edits and identical overlapping edits merge cleanly.
// Conflicts contain both versions of the entire connected overlapping region.
// Insertions at an edit's boundaries are independent; insertions strictly inside
// an edit overlap it. Two insertions at the same position overlap each other.
// Exception: an insertion after an unterminated replacement overlaps that edit,
// since concatenating them would silently join two intended lines.
func ThreeWayMerge(base, theirs, ours string) MergeResult {
	if base == theirs {
		return MergeResult{Content: ours}
	}
	if base == ours {
		return MergeResult{Content: theirs}
	}
	if theirs == ours {
		return MergeResult{Content: ours}
	}

	return mergeLines(splitLines(base), lineEdits(base, theirs, false), lineEdits(base, ours, true))
}

// splitLines retains line terminators as part of each token. In particular, an
// empty file has no tokens, and a final newline does not introduce a phantom line.
func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	lines := strings.SplitAfter(s, "\n")
	if lines[len(lines)-1] == "" {
		lines = lines[:len(lines)-1]
	}
	return lines
}

func mergeLines(base []string, theirs, ours []lineEdit) MergeResult {
	edits := slices.Concat(theirs, ours)
	slices.SortStableFunc(edits, func(a, b lineEdit) int {
		if order := cmp.Compare(a.start, b.start); order != 0 {
			return order
		}
		// Process boundary insertions before the replacement starting there.
		return cmp.Compare(a.end, b.end)
	})

	var out strings.Builder
	hasConflict, cursor := false, 0
	for i := 0; i < len(edits); {
		start, end := edits[i].start, edits[i].end
		j := i + 1
		for j < len(edits) && overlapsRegion(start, end, edits[i:j], edits[j]) {
			end = max(end, edits[j].end)
			j++
		}
		out.WriteString(strings.Join(base[cursor:start], ""))
		group := edits[i:j]
		userText, userChanged := applyRegion(base, start, end, group, false)
		templateText, templateChanged := applyRegion(base, start, end, group, true)
		switch {
		case !userChanged:
			out.WriteString(templateText)
		case !templateChanged || userText == templateText:
			out.WriteString(userText)
		default:
			hasConflict = true
			writeConflict(&out, userText, templateText)
		}
		cursor, i = end, j
	}
	out.WriteString(strings.Join(base[cursor:], ""))
	return MergeResult{Content: out.String(), HasConflict: hasConflict}
}

func overlapsRegion(start, end int, region []lineEdit, next lineEdit) bool {
	if next.start < end || (next.start == start && next.end == start && end == start) {
		return true
	}
	if next.start != end || next.end != end {
		return false
	}
	// A final-line replacement can remove its terminator while the other side
	// appends at EOF. They share the removed line boundary despite disjoint base
	// intervals. Include both full payloads rather than inventing a newline.
	for _, edit := range region {
		if edit.end == end && edit.text != "" && !strings.HasSuffix(edit.text, "\n") {
			return true
		}
	}
	return false
}

// applyRegion reconstructs one side, including unchanged base lines between its
// edits. This keeps transitive overlaps together instead of truncating a conflict.
func applyRegion(base []string, start, end int, edits []lineEdit, templateSide bool) (string, bool) {
	var out strings.Builder
	cursor, changed := start, false
	for _, edit := range edits {
		if edit.templateSide != templateSide {
			continue
		}
		out.WriteString(strings.Join(base[cursor:edit.start], ""))
		out.WriteString(edit.text)
		cursor, changed = edit.end, true
	}
	out.WriteString(strings.Join(base[cursor:end], ""))
	return out.String(), changed
}

// Marker lines use LF. Payload bytes (including CRLF) remain untouched, except
// that unterminated payloads need an LF to put the following marker on its own
// line. A deleted side is empty, not synthetic explanatory file content.
func writeConflict(out *strings.Builder, theirs, ours string) {
	if out.Len() > 0 && !strings.HasSuffix(out.String(), "\n") {
		out.WriteByte('\n')
	}
	out.WriteString("<<<<<<< user-edits\n")
	out.WriteString(theirs)
	if theirs != "" && !strings.HasSuffix(theirs, "\n") {
		out.WriteByte('\n')
	}
	out.WriteString("=======\n")
	out.WriteString(ours)
	if ours != "" && !strings.HasSuffix(ours, "\n") {
		out.WriteByte('\n')
	}
	out.WriteString(">>>>>>> template\n")
}
