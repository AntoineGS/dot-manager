package template

import (
	"strings"
	"testing"
)

func TestMergeRender(t *testing.T) {
	tests := []struct {
		name, base, current, rendered, want string
		history, force, conflict            bool
	}{
		{"merge", "a=1\nb=1", "a=2\nb=1", "a=1\nb=2", "a=2\nb=2", true, false, false},
		{"conflict", "a=1", "a=2", "a=3", "a=3", true, false, true},
		{"force", "a=1", "a=2", "a=3", "a=3", true, true, false},
		{"no history", "", "manual", "fresh", "fresh", false, false, false},
		{"unchanged output", "a=1", "a=2", "a=1", "a=2", true, false, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MergeRender([]byte(tt.base), []byte(tt.current), []byte(tt.rendered), tt.history, tt.force)
			if string(got.Content) != tt.want || (len(got.Conflict) > 0) != tt.conflict {
				t.Fatalf("content=%q conflict=%q", got.Content, got.Conflict)
			}
			if tt.conflict && !strings.Contains(string(got.Conflict), "<<<<<<< user-edits") {
				t.Fatal("missing conflict markers in recovery artifact")
			}
		})
	}
}
