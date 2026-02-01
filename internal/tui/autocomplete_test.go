package tui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetPathSuggestions(t *testing.T) {
	// Create temp directory structure
	tmpDir := t.TempDir()

	// Create some test directories and files
	dirs := []string{"config", "cache", "local", ".hidden"}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(tmpDir, d), 0755); err != nil {
			t.Fatal(err)
		}
	}

	// Create a file
	if err := os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name      string
		input     string
		configDir string
		wantMin   int // minimum expected suggestions
	}{
		{
			name:      "empty input returns nil",
			input:     "",
			configDir: tmpDir,
			wantMin:   0,
		},
		{
			name:      "list directory contents",
			input:     tmpDir + "/",
			configDir: tmpDir,
			wantMin:   3, // at least config, cache, local (hidden excluded by default)
		},
		{
			name:      "filter by prefix",
			input:     tmpDir + "/c",
			configDir: tmpDir,
			wantMin:   2, // config, cache
		},
		{
			name:      "hidden files with dot prefix",
			input:     tmpDir + "/.",
			configDir: tmpDir,
			wantMin:   1, // .hidden
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			suggestions := getPathSuggestions(tt.input, tt.configDir)
			if len(suggestions) < tt.wantMin {
				t.Errorf("getPathSuggestions() got %d suggestions, want at least %d", len(suggestions), tt.wantMin)
			}
		})
	}
}

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Skip("cannot get home directory")
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "tilde expansion",
			input: "~/test",
			want:  filepath.Join(home, "test"),
		},
		{
			name:  "just tilde",
			input: "~",
			want:  home,
		},
		{
			name:  "no tilde",
			input: "/usr/local",
			want:  "/usr/local",
		},
		{
			name:  "relative path",
			input: "./config",
			want:  "./config",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := expandPath(tt.input)
			if got != tt.want {
				t.Errorf("expandPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildSuggestionPath(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		filename string
		isDir    bool
		want     string
	}{
		{
			name:     "directory with trailing slash",
			input:    "~/.config/",
			filename: "nvim",
			isDir:    true,
			want:     "~/.config/nvim/",
		},
		{
			name:     "partial path",
			input:    "~/.config/nv",
			filename: "nvim",
			isDir:    true,
			want:     "~/.config/nvim/",
		},
		{
			name:     "file suggestion",
			input:    "~/",
			filename: "file.txt",
			isDir:    false,
			want:     "~/file.txt",
		},
		{
			name:     "relative path",
			input:    "./con",
			filename: "config",
			isDir:    true,
			want:     "./config/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildSuggestionPath(tt.input, tt.filename, tt.isDir)
			if got != tt.want {
				t.Errorf("buildSuggestionPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
