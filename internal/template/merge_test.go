package template

import (
	"strings"
	"testing"
)

func TestThreeWayMerge(t *testing.T) {
	tests := []struct {
		name           string
		base           string
		theirs         string
		ours           string
		wantContent    string
		wantConflict   bool
		wantContains   []string // partial content checks (for conflict markers)
		wantNotContain []string
	}{
		{
			name:         "no user edits - use ours",
			base:         "line1\nline2\nline3",
			theirs:       "line1\nline2\nline3",
			ours:         "line1\nchanged\nline3",
			wantContent:  "line1\nchanged\nline3",
			wantConflict: false,
		},
		{
			name:         "no template changes - keep theirs",
			base:         "line1\nline2\nline3",
			theirs:       "line1\nuser-edit\nline3",
			ours:         "line1\nline2\nline3",
			wantContent:  "line1\nuser-edit\nline3",
			wantConflict: false,
		},
		{
			name:         "same changes both sides",
			base:         "line1\nline2\nline3",
			theirs:       "line1\nsame-change\nline3",
			ours:         "line1\nsame-change\nline3",
			wantContent:  "line1\nsame-change\nline3",
			wantConflict: false,
		},
		{
			name:         "user edits preserved - different lines",
			base:         "line1\nline2\nline3",
			theirs:       "line1\nuser-edit\nline3",
			ours:         "line1\nline2\ntemplate-change",
			wantContent:  "line1\nuser-edit\ntemplate-change",
			wantConflict: false,
		},
		{
			name:         "template changes applied - different lines",
			base:         "line1\nline2\nline3",
			theirs:       "line1\nline2\nline3",
			ours:         "new-line1\nline2\nline3",
			wantContent:  "new-line1\nline2\nline3",
			wantConflict: false,
		},
		{
			name:         "conflict - same line changed differently",
			base:         "line1\nline2\nline3",
			theirs:       "line1\nuser-change\nline3",
			ours:         "line1\ntemplate-change\nline3",
			wantConflict: true,
			wantContains: []string{"<<<<<<< user-edits", "user-change", "=======", "template-change", ">>>>>>> template"},
		},
		{
			name:         "user adds lines at end",
			base:         "line1\nline2",
			theirs:       "line1\nline2\nuser-added",
			ours:         "line1\nline2",
			wantContent:  "line1\nline2\nuser-added",
			wantConflict: false,
		},
		{
			name:         "template adds lines at end",
			base:         "line1\nline2",
			theirs:       "line1\nline2",
			ours:         "line1\nline2\ntemplate-added",
			wantContent:  "line1\nline2\ntemplate-added",
			wantConflict: false,
		},
		{
			name:         "user deletes line - template unchanged",
			base:         "line1\nline2\nline3",
			theirs:       "line1\nline3",
			ours:         "line1\nline2\nline3",
			wantContent:  "line1\nline3",
			wantConflict: false,
		},
		{
			name:         "template deletes line - user unchanged",
			base:         "line1\nline2\nline3",
			theirs:       "line1\nline2\nline3",
			ours:         "line1\nline3",
			wantContent:  "line1\nline3",
			wantConflict: false,
		},
		{
			name:         "empty base - first render with existing file",
			base:         "",
			theirs:       "existing-content",
			ours:         "rendered-content",
			wantConflict: true,
			wantContains: []string{"<<<<<<< user-edits", "existing-content", "rendered-content", ">>>>>>> template"},
		},
		{
			name:         "all empty",
			base:         "",
			theirs:       "",
			ours:         "",
			wantContent:  "",
			wantConflict: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ThreeWayMerge(tt.base, tt.theirs, tt.ours)

			if result.HasConflict != tt.wantConflict {
				t.Errorf("HasConflict = %v, want %v", result.HasConflict, tt.wantConflict)
			}

			if tt.wantContent != "" && result.Content != tt.wantContent {
				t.Errorf("Content = %q, want %q", result.Content, tt.wantContent)
			}

			for _, s := range tt.wantContains {
				if !strings.Contains(result.Content, s) {
					t.Errorf("Content should contain %q, got %q", s, result.Content)
				}
			}

			for _, s := range tt.wantNotContain {
				if strings.Contains(result.Content, s) {
					t.Errorf("Content should NOT contain %q, got %q", s, result.Content)
				}
			}
		})
	}
}

func TestThreeWayMerge_MultiCycle(t *testing.T) {
	// Cycle 1: initial render
	renderV1 := "export EDITOR=vim\nexport PATH=$PATH:/usr/local/bin\n# end"

	// User edits: changes editor
	userEditV1 := "export EDITOR=nvim\nexport PATH=$PATH:/usr/local/bin\n# end"

	// Cycle 2: template changes PATH line
	renderV2 := "export EDITOR=vim\nexport PATH=$PATH:/usr/local/bin:/opt/bin\n# end"

	result1 := ThreeWayMerge(renderV1, userEditV1, renderV2)
	if result1.HasConflict {
		t.Fatalf("Cycle 2: unexpected conflict: %s", result1.Content)
	}

	// Should have user's EDITOR change AND template's PATH change
	wantCycle2 := "export EDITOR=nvim\nexport PATH=$PATH:/usr/local/bin:/opt/bin\n# end"
	if result1.Content != wantCycle2 {
		t.Errorf("Cycle 2:\ngot:  %q\nwant: %q", result1.Content, wantCycle2)
	}

	// Cycle 3: user makes another edit (to cycle 2 merged result)
	// Pure render stored in DB is renderV2 (not the merge result)
	userEditV2 := "export EDITOR=nvim\nexport PATH=$PATH:/usr/local/bin:/opt/bin\nexport TERM=xterm-256color\n# end"

	// Template changes again
	renderV3 := "export EDITOR=vim\nexport PATH=$HOME/.local/bin:$PATH:/usr/local/bin:/opt/bin\n# end"

	result2 := ThreeWayMerge(renderV2, userEditV2, renderV3)
	if result2.HasConflict {
		t.Fatalf("Cycle 3: unexpected conflict: %s", result2.Content)
	}

	// Should preserve: user's EDITOR=nvim, user's TERM addition, template's new PATH
	if !strings.Contains(result2.Content, "EDITOR=nvim") {
		t.Error("Cycle 3: lost user's EDITOR change")
	}
	if !strings.Contains(result2.Content, "TERM=xterm-256color") {
		t.Error("Cycle 3: lost user's TERM addition")
	}
	if !strings.Contains(result2.Content, "HOME/.local/bin") {
		t.Error("Cycle 3: lost template's PATH change")
	}
}
