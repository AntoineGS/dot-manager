package manager

import "testing"

func TestValidateTemplateSelectionCollisions_CaseSemantics(t *testing.T) {
	tests := []struct {
		name  string
		files []string
	}{
		{name: "source names", files: []string{"Config.tmpl", "config.tmpl"}},
		{name: "effective target names", files: []string{"Config", "config.tmpl"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := validateTemplateSelectionCollisions(tt.files, true); err == nil {
				t.Fatal("case-insensitive path claims accepted a collision")
			}
			if err := validateTemplateSelectionCollisions(tt.files, false); err != nil {
				t.Fatalf("case-sensitive path claims rejected distinct paths: %v", err)
			}
		})
	}
}
