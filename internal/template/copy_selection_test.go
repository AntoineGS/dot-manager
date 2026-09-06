package template

import "testing"

func TestValidateCopySelectionCollisions(t *testing.T) {
	tests := []struct {
		name            string
		files           []string
		caseInsensitive bool
		wantErr         bool
	}{
		{name: "ordinary source reserves template conflict", files: []string{"root.tmpl", "root.tmpl.conflict"}, wantErr: true},
		{name: "ordinary target collides with stripped template target", files: []string{"root.tmpl", "root"}, wantErr: true},
		{name: "ordinary source does not reserve rendered cache", files: []string{"root.tmpl", "root.tmpl.rendered"}},
		{name: "ordinary target collides with orphan backup", files: []string{"root.tmpl", "root.tidydots.bak"}, wantErr: true},
		{name: "ancestor claims cannot both deploy", files: []string{"nested", "nested/config.tmpl"}, wantErr: true},
		{name: "case insensitive target collision", files: []string{"Root.tmpl", "root"}, caseInsensitive: true, wantErr: true},
		{name: "case sensitive distinct target names", files: []string{"Root.tmpl", "root"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateCopySelectionCollisions(tt.files, tt.caseInsensitive)
			if tt.wantErr && err == nil {
				t.Fatal("expected selection collision")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected selection collision: %v", err)
			}
		})
	}
}
