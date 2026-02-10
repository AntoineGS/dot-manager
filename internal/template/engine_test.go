package template

import (
	"testing"
)

func TestRenderString(t *testing.T) {
	ctx := &Context{
		OS:         "linux",
		Distro:     "arch",
		Hostname:   "myhost",
		User:       "testuser",
		HasDisplay: true,
		Env: map[string]string{
			"HOME":   "/home/testuser",
			"EDITOR": "nvim",
		},
	}
	engine := NewEngine(ctx)

	tests := []struct {
		name     string
		template string
		want     string
		wantErr  bool
	}{
		{
			name:     "simple variable",
			template: "OS is {{ .OS }}",
			want:     "OS is linux",
		},
		{
			name:     "multiple variables",
			template: "{{ .OS }}/{{ .Distro }}/{{ .Hostname }}",
			want:     "linux/arch/myhost",
		},
		{
			name:     "conditional block",
			template: `{{ if eq .OS "linux" }}is linux{{ else }}not linux{{ end }}`,
			want:     "is linux",
		},
		{
			name:     "conditional false",
			template: `{{ if eq .OS "windows" }}is windows{{ else }}not windows{{ end }}`,
			want:     "not windows",
		},
		{
			name:     "no delimiters passthrough",
			template: "just a plain string",
			want:     "just a plain string",
		},
		{
			name:     "empty string passthrough",
			template: "",
			want:     "",
		},
		{
			name:     "env var access",
			template: "home={{ index .Env \"HOME\" }}",
			want:     "home=/home/testuser",
		},
		{
			name:     "sprout upper function",
			template: `{{ "hello" | toUpper }}`,
			want:     "HELLO",
		},
		{
			name:     "sprout lower function",
			template: `{{ "HELLO" | toLower }}`,
			want:     "hello",
		},
		{
			name:     "sprout trim function",
			template: `{{ "  hello  " | trim }}`,
			want:     "hello",
		},
		{
			name:     "user field",
			template: "user={{ .User }}",
			want:     "user=testuser",
		},
		{
			name:     "HasDisplay true",
			template: `{{ if .HasDisplay }}gui{{ else }}headless{{ end }}`,
			want:     "gui",
		},
		{
			name:     "invalid template",
			template: "{{ .Invalid",
			wantErr:  true,
		},
		{
			name:     "unknown field",
			template: "{{ .NonExistent }}",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := engine.RenderString(tt.name, tt.template)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRenderString_HasDisplayFalse(t *testing.T) {
	ctx := &Context{
		OS:         "linux",
		HasDisplay: false,
		Env:        map[string]string{},
	}
	engine := NewEngine(ctx)

	got, err := engine.RenderString("hasdisplay-false", `{{ if .HasDisplay }}gui{{ else }}headless{{ end }}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got != "headless" {
		t.Errorf("got %q, want %q", got, "headless")
	}
}

func TestRenderBytes(t *testing.T) {
	ctx := &Context{
		OS:       "linux",
		Distro:   "arch",
		Hostname: "myhost",
		User:     "testuser",
		Env:      map[string]string{},
	}
	engine := NewEngine(ctx)

	content := []byte(`# Config for {{ .Hostname }}
{{ if eq .OS "linux" }}export EDITOR=nvim{{ end }}
`)

	got, err := engine.RenderBytes("test", content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := "# Config for myhost\nexport EDITOR=nvim\n"
	if string(got) != want {
		t.Errorf("got %q, want %q", string(got), want)
	}
}

func TestRenderBytes_Error(t *testing.T) {
	ctx := &Context{OS: "linux", Env: map[string]string{}}
	engine := NewEngine(ctx)

	_, err := engine.RenderBytes("bad", []byte("{{ .Invalid"))
	if err == nil {
		t.Error("expected error for invalid template")
	}
}

func TestIsTemplateFile(t *testing.T) {
	tests := []struct {
		name     string
		filename string
		want     bool
	}{
		{"template file", ".zshrc.tmpl", true},
		{"nested template", "config/init.lua.tmpl", true},
		{"regular file", ".zshrc", false},
		{"rendered file", ".zshrc.tmpl.rendered", false},
		{"conflict file", ".zshrc.tmpl.conflict", false},
		{"tmpl in middle", "tmpl/config", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsTemplateFile(tt.filename); got != tt.want {
				t.Errorf("IsTemplateFile(%q) = %v, want %v", tt.filename, got, tt.want)
			}
		})
	}
}

func TestRenderedPath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{".zshrc.tmpl", ".zshrc.tmpl.rendered"},
		{"config/init.lua.tmpl", "config/init.lua.tmpl.rendered"},
	}
	for _, tt := range tests {
		if got := RenderedPath(tt.input); got != tt.want {
			t.Errorf("RenderedPath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestTargetName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{".zshrc.tmpl", ".zshrc"},
		{"config/init.lua.tmpl", "init.lua"},
		{"plain.txt", "plain.txt"},
	}
	for _, tt := range tests {
		if got := TargetName(tt.input); got != tt.want {
			t.Errorf("TargetName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestConflictPath(t *testing.T) {
	if got := ConflictPath(".zshrc.tmpl"); got != ".zshrc.tmpl.conflict" {
		t.Errorf("ConflictPath got %q", got)
	}
}

func TestIsRenderedFile(t *testing.T) {
	if !IsRenderedFile("foo.tmpl.rendered") {
		t.Error("expected true for .tmpl.rendered")
	}
	if IsRenderedFile("foo.tmpl") {
		t.Error("expected false for .tmpl")
	}
}

func TestIsConflictFile(t *testing.T) {
	if !IsConflictFile("foo.tmpl.conflict") {
		t.Error("expected true for .tmpl.conflict")
	}
	if IsConflictFile("foo.tmpl") {
		t.Error("expected false for .tmpl")
	}
}
