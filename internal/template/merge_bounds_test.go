package template

import (
	"fmt"
	"strings"
	"testing"
)

func TestThreeWayMergeUnterminatedBoundary(t *testing.T) {
	const base = "key=value\n"
	const appended = "key=value\nnext=other\n"
	for _, replacement := range []string{"key=value", "key=edited"} {
		for _, swapped := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/swapped=%v", replacement, swapped), func(t *testing.T) {
				theirs, ours := replacement, appended
				userPayload, templatePayload := replacement+"\n", appended
				if swapped {
					theirs, ours = ours, theirs
					userPayload, templatePayload = templatePayload, userPayload
				}
				want := "<<<<<<< user-edits\n" + userPayload + "=======\n" + templatePayload + ">>>>>>> template\n"
				got := ThreeWayMerge(base, theirs, ours)
				if !got.HasConflict || got.Content != want {
					t.Fatalf("unterminated replacement plus append: %#v, want conflict %q", got, want)
				}
			})
		}
	}
}

// An exact interior anchor must not trigger an unbounded search across a large
// changed region. Keep exact outside context, but coarsen the unresolved middle.
func TestLineEditsHighDistanceGuard(t *testing.T) {
	base, changed := highDistanceText(8000)
	base = "prefix\n" + base + "suffix\n"
	changed = "prefix\n" + changed + "suffix\n"
	edits := lineEdits(base, changed, false)
	if len(edits) != 1 || edits[0].start != 1 || edits[0].end != 8001 ||
		edits[0].text != strings.TrimSuffix(strings.TrimPrefix(changed, "prefix\n"), "suffix\n") {
		t.Fatalf("expected one bounded middle replacement, got %d edits", len(edits))
	}
	got, _ := applyRegion(splitLines(base), 0, len(splitLines(base)), edits, false)
	if got != changed {
		t.Fatal("coarse replacement did not reconstruct exact bytes")
	}
}

func TestThreeWayMergeCoarseConflictPreservesContext(t *testing.T) {
	base, rendered := highDistanceText(8000)
	const prefix = "前\r\n"
	const suffix = "末尾\xff"
	theirs := strings.Replace(base, "key-00001=世界\n", "key-00001=user\n", 1)
	for _, swapped := range []bool{false, true} {
		userPayload, templatePayload := theirs, rendered
		if swapped {
			userPayload, templatePayload = templatePayload, userPayload
		}
		want := prefix + "<<<<<<< user-edits\n" + userPayload + "=======\n" + templatePayload + ">>>>>>> template\n" + suffix
		got := ThreeWayMerge(prefix+base+suffix, prefix+userPayload+suffix, prefix+templatePayload+suffix)
		if !got.HasConflict || got.Content != want {
			t.Fatalf("coarse conflict lost context or payload bytes (swapped=%v)", swapped)
		}
	}
}

func TestLineEditsDetailedDiffSizeBoundary(t *testing.T) {
	for _, tt := range []struct{ lines, wantEdits int }{{2048, 2}, {2049, 1}} {
		base, changed := highDistanceText(tt.lines)
		edits := lineEdits(base, changed, false)
		if len(edits) != tt.wantEdits {
			t.Fatalf("%d combined lines: got %d edits, want %d", tt.lines*2, len(edits), tt.wantEdits)
		}
	}
}

func highDistanceText(lines int) (string, string) {
	var base, changed strings.Builder
	for i := 0; i < lines; i++ {
		fmt.Fprintf(&base, "key-%05d=世界\n", i)
		if i == lines/2 {
			// Keeping one interior line equal exposes the coarse-fallback policy.
			fmt.Fprintf(&changed, "key-%05d=世界\n", i)
		} else {
			fmt.Fprintf(&changed, "key-%05d=世界\r\n", i)
		}
	}
	return base.String(), changed.String()
}

func BenchmarkThreeWayMergeHighDistance(b *testing.B) {
	base, _ := highDistanceText(64000)
	ours := strings.ReplaceAll(base, "\n", "\r\n")
	theirs, want := "# user header\n"+base, "# user header\n"+ours
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got := ThreeWayMerge(base, theirs, ours)
		if got.HasConflict || got.Content != want {
			b.Fatal("high-distance merge lost content")
		}
	}
}
