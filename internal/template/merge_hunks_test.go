package template

import (
	"fmt"
	"strings"
	"testing"
)

func TestThreeWayMergeHunks(t *testing.T) {
	tests := []struct {
		name, base, theirs, ours, want string
		conflict                       bool
	}{
		{"insert before distant edit", "a\nb\nc\n", "x\na\nb\nc\n", "a\nb\nC\n", "x\na\nb\nC\n", false},
		{"delete before distant edit", "a\nb\nc\n", "b\nc\n", "a\nb\nC\n", "b\nC\n", false},
		{"independent deletions", "a\nb\nc\nd\n", "b\nc\nd\n", "a\nb\nc\n", "b\nc\n", false},
		{"adjacent replacements", "a\nb\nc\n", "A\nb\nc\n", "a\nB\nc\n", "A\nB\nc\n", false},
		{"insertion at replacement start", "a\nb\n", "x\na\nb\n", "A\nb\n", "x\nA\nb\n", false},
		{"insertion at replacement end", "a\nb\n", "a\nx\nb\n", "A\nb\n", "A\nx\nb\n", false},
		{"same insertion with independent edits", "a\nb\nc\n", "x\na\nb\nC\n", "x\na\nB\nc\n", "x\na\nB\nC\n", false},
		{"same deletion with independent edits", "a\nb\nc\nd\n", "b\nC\nd\n", "b\nc\nD\n", "b\nC\nD\n", false},
		{"same replacement with independent edits", "a\nb\nc\nd\n", "A\nb\nC\nd\n", "A\nb\nc\nD\n", "A\nb\nC\nD\n", false},
		{"distinct same-point insertions", "a\nb\n", "a\nx\nb\n", "a\ny\nb\n", "a\n<<<<<<< user-edits\nx\n=======\ny\n>>>>>>> template\nb\n", true},
		{"full overlapping replacement", "a\nb\nc\nd\ne\n", "a\nX\nd\ne\n", "a\nb\nY\ne\n", "a\n<<<<<<< user-edits\nX\nd\n=======\nb\nY\n>>>>>>> template\ne\n", true},
		{"delete versus modify", "a\nb\nc\n", "a\nc\n", "a\nB\nc\n", "a\n<<<<<<< user-edits\n=======\nB\n>>>>>>> template\nc\n", true},
		{"modify versus delete", "a\nb\nc\n", "a\nB\nc\n", "a\nc\n", "a\n<<<<<<< user-edits\nB\n=======\n>>>>>>> template\nc\n", true},
		{"insertion inside deletion", "a\nb\nc\nd\n", "a\nd\n", "a\nb\nx\nc\nd\n", "a\n<<<<<<< user-edits\n=======\nb\nx\nc\n>>>>>>> template\nd\n", true},
		{"empty divergent additions", "", "x", "y", "<<<<<<< user-edits\nx\n=======\ny\n>>>>>>> template\n", true},
		{"unicode CRLF", "α\r\nβ\r\nγ", "零\r\nα\r\nβ\r\nγ", "α\r\nΒ\r\nγ", "零\r\nα\r\nΒ\r\nγ", false},
		{"trailing newline removed independently", "a\nb\nc\n", "A\nb\nc\n", "a\nb\nc", "A\nb\nc", false},
		{"trailing newline added independently", "a\nb\nc", "A\nb\nc", "a\nb\nc\n", "A\nb\nc\n", false},
		{"repeated lines", "r\nr\nanchor\nz\n", "r\nanchor\nz\n", "r\nr\nanchor\nZ\n", "r\nanchor\nZ\n", false},
		{"empty result", "a\nb\n", "b\n", "a\n", "", false},
		{"transitive overlapping edits", "a\nb\nc\nd\ne\nf\ng\n", "a\nX\nd\nZ\ng\n", "a\nb\nY\nf\ng\n", "a\n<<<<<<< user-edits\nX\nd\nZ\n=======\nb\nY\nf\n>>>>>>> template\ng\n", true},
		{"multiple separate conflicts", "a\nb\nc\n", "A\nb\nC\n", "X\nb\nZ\n", "<<<<<<< user-edits\nA\n=======\nX\n>>>>>>> template\nb\n<<<<<<< user-edits\nC\n=======\nZ\n>>>>>>> template\n", true},
		{"CRLF conflict preserves payload", "a\r\nb\r\n", "A\r\nb\r\n", "X\r\nb\r\n", "<<<<<<< user-edits\nA\r\n=======\nX\r\n>>>>>>> template\nb\r\n", true},
		{"insertion at deletion start", "a\nb\nc\n", "a\nx\nb\nc\n", "a\nc\n", "a\nx\nc\n", false},
		{"insertion at deletion end", "a\nb\nc\n", "a\nb\nx\nc\n", "a\nc\n", "a\nx\nc\n", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ThreeWayMerge(tt.base, tt.theirs, tt.ours)
			if got.Content != tt.want || got.HasConflict != tt.conflict {
				t.Fatalf("got %#v; want content %q, conflict %v", got, tt.want, tt.conflict)
			}
			if again := ThreeWayMerge(tt.base, tt.theirs, tt.ours); again != got {
				t.Fatalf("non-deterministic merge: %#v versus %#v", got, again)
			}
			if !tt.conflict {
				if swapped := ThreeWayMerge(tt.base, tt.ours, tt.theirs); swapped != got {
					t.Fatalf("clean merge is asymmetric: %#v versus %#v", got, swapped)
				}
			}
		})
	}
}

func FuzzThreeWayMergeIdentity(f *testing.F) {
	f.Add("a\nb\n", "x\na\nb\n")
	f.Add("", "\xff\r\n世界")
	f.Fuzz(func(t *testing.T, base, changed string) {
		for _, got := range []MergeResult{
			ThreeWayMerge(base, base, changed),
			ThreeWayMerge(base, changed, base),
			ThreeWayMerge(base, changed, changed),
		} {
			if got.HasConflict || got.Content != changed {
				t.Fatalf("identity failed: %#v, want %q", got, changed)
			}
		}
	})
}

func FuzzThreeWayMergeIndependentEdits(f *testing.F) {
	f.Add("hello", "世界", true)
	f.Add("\xff\r", "", false)
	f.Fuzz(func(t *testing.T, user, render string, insert bool) {
		// Tagged lines keep arbitrary bytes while making edit locations unambiguous.
		user = "user:" + strings.ReplaceAll(user, "\n", "") + "\n"
		render = "render:" + strings.ReplaceAll(render, "\n", "") + "\n"
		base := "first\nanchor\nlast\n"
		theirs, want := "anchor\nlast\n", "anchor\n"+render
		if insert {
			theirs, want = user+base, user+"first\nanchor\n"+render
		}
		got := ThreeWayMerge(base, theirs, "first\nanchor\n"+render)
		if got.HasConflict || got.Content != want {
			t.Fatalf("independent edits: %#v, want %q", got, want)
		}
	})
}

func BenchmarkThreeWayMergeLarge(b *testing.B) {
	var text strings.Builder
	for i := 0; i < 10000; i++ {
		fmt.Fprintf(&text, "setting-%05d=value\n", i)
	}
	base := text.String()
	theirs := "# user header\n" + base
	ours := strings.Replace(base, "setting-09000=value", "setting-09000=new", 1)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		got := ThreeWayMerge(base, theirs, ours)
		if got.HasConflict || got.Content != "# user header\n"+ours {
			b.Fatal("incorrect large merge")
		}
	}
}

func TestThreeWayMergeBeyond16BitLineIDs(t *testing.T) {
	var text strings.Builder
	for i := 0; i < 70000; i++ {
		fmt.Fprintf(&text, "setting-%05d=value\n", i)
	}
	base := text.String()
	ours := strings.Replace(base, "setting-66000=value", "setting-66000=new", 1)
	got := ThreeWayMerge(base, "header\n"+base, ours)
	if got.HasConflict || got.Content != "header\n"+ours {
		t.Fatal("line token encoding lost content beyond 16-bit line IDs")
	}
}

func TestLineEditsLargeCommonSuffix(t *testing.T) {
	base := strings.Repeat("line\n", 500000)
	changed := "header\n" + base
	edits := lineEdits(base, changed, false)
	if len(edits) != 1 || edits[0].start != 0 || edits[0].end != 0 || edits[0].text != "header\n" {
		t.Fatal("a large exact common suffix must not force a coarse replacement")
	}
}

func FuzzLineEditsReconstruct(f *testing.F) {
	f.Add("a\nb\na\n", "a\nx\na\n")
	f.Add("\xff\n\r\n世界", "世界\n\x00")
	f.Fuzz(func(t *testing.T, base, changed string) {
		// Bound fuzz input/corpus size; production diff work is bounded separately.
		if len(base)+len(changed) > 16000 {
			t.Skip()
		}
		lines := splitLines(base)
		edits := lineEdits(base, changed, false)
		cursor := 0
		for _, edit := range edits {
			if edit.start < cursor || edit.end < edit.start || edit.end > len(lines) {
				t.Fatalf("invalid edit range: %#v", edit)
			}
			cursor = edit.end
		}
		got, _ := applyRegion(lines, 0, len(lines), edits, false)
		if got != changed {
			t.Fatalf("edit reconstruction = %q, want %q", got, changed)
		}
	})
}

func FuzzThreeWayMergeSymmetry(f *testing.F) {
	f.Add("a\nb\na\n", "a\nx\na\n", "a\nb\ny\na\n")
	f.Add("a\nb\nc\nd\n", "a\nd\n", "a\nb\nx\nc\nd\n")
	f.Add("\xff\r\n", "", "\x00")
	f.Add("key=value\n", "key=value", "key=value\nnext=other\n")
	f.Add("key=value\n", "key=value\nnext=other\n", "key=edited")
	f.Fuzz(func(t *testing.T, base, theirs, ours string) {
		if len(base)+len(theirs)+len(ours) > 16000 {
			t.Skip()
		}
		got := ThreeWayMerge(base, theirs, ours)
		if again := ThreeWayMerge(base, theirs, ours); again != got {
			t.Fatal("non-deterministic merge")
		}
		swapped := ThreeWayMerge(base, ours, theirs)
		if swapped.HasConflict != got.HasConflict || (!got.HasConflict && swapped.Content != got.Content) {
			t.Fatalf("asymmetric merge: %#v versus %#v", got, swapped)
		}
	})
}
