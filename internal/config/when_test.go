package config

import (
	"fmt"
	"testing"
)

// mockWhenRenderer is a simple PathRenderer for testing EvaluateWhen.
// It renders templates by looking up values in its data map.
type mockWhenRenderer struct {
	result string
	err    error
}

func (m *mockWhenRenderer) RenderString(_, _ string) (string, error) {
	if m.err != nil {
		return "", m.err
	}

	return m.result, nil
}

func TestEvaluateWhen(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		when     string
		renderer PathRenderer
		want     bool
	}{
		{
			name:     "empty string returns true",
			when:     "",
			renderer: &mockWhenRenderer{result: ""},
			want:     true,
		},
		{
			name:     "whitespace only returns true",
			when:     "   ",
			renderer: &mockWhenRenderer{result: ""},
			want:     true,
		},
		{
			name:     "nil renderer returns false",
			when:     "{{ eq .OS \"linux\" }}",
			renderer: nil,
			want:     false,
		},
		{
			name:     "renders to true",
			when:     "{{ eq .OS \"linux\" }}",
			renderer: &mockWhenRenderer{result: "true"},
			want:     true,
		},
		{
			name:     "renders to true with whitespace",
			when:     "{{ eq .OS \"linux\" }}",
			renderer: &mockWhenRenderer{result: " true "},
			want:     true,
		},
		{
			name:     "renders to false",
			when:     "{{ eq .OS \"linux\" }}",
			renderer: &mockWhenRenderer{result: "false"},
			want:     false,
		},
		{
			name:     "renders to arbitrary string",
			when:     "{{ .OS }}",
			renderer: &mockWhenRenderer{result: "linux"},
			want:     false,
		},
		{
			name:     "render error returns false",
			when:     "{{ invalid }}",
			renderer: &mockWhenRenderer{err: fmt.Errorf("template error")},
			want:     false,
		},
		{
			name:     "renders to empty string",
			when:     "{{ .Missing }}",
			renderer: &mockWhenRenderer{result: ""},
			want:     false,
		},
		{
			name:     "renders to TRUE (case sensitive)",
			when:     "{{ .OS }}",
			renderer: &mockWhenRenderer{result: "TRUE"},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := EvaluateWhen(tt.when, tt.renderer)
			if got != tt.want {
				t.Errorf("EvaluateWhen(%q) = %v, want %v", tt.when, got, tt.want)
			}
		})
	}
}
