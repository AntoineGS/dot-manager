package manager

import (
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
)

func TestList_FiltersByOS(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)

	m.Config.Entries = []config.Entry{
		{
			Name:   "linux-only",
			Backup: "./linux",
			Targets: map[string]string{
				"linux": "~/.config/linux",
			},
			Filters: []config.Filter{
				{Include: map[string]string{"os": "linux"}},
			},
		},
		{
			Name:   "windows-only",
			Backup: "./windows",
			Targets: map[string]string{
				"windows": "~/AppData/windows",
			},
			Filters: []config.Filter{
				{Include: map[string]string{"os": "windows"}},
			},
		},
		{
			Name:   "no-filter",
			Backup: "./both",
			Targets: map[string]string{
				"linux": "~/.config/both",
			},
		},
	}

	m.Platform.OS = "linux"
	entries := m.GetEntries()

	// Should get linux-only and no-filter
	if len(entries) != 2 {
		t.Errorf("got %d entries, want 2", len(entries))
	}

	names := make(map[string]bool)
	for _, e := range entries {
		names[e.Name] = true
	}

	if !names["linux-only"] {
		t.Error("missing linux-only entry")
	}
	if !names["no-filter"] {
		t.Error("missing no-filter entry")
	}
	if names["windows-only"] {
		t.Error("should not include windows-only")
	}
}

func TestList_EmptyConfig(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)
	m.Config.Entries = []config.Entry{}

	entries := m.GetEntries()

	if len(entries) != 0 {
		t.Errorf("got %d entries, want 0", len(entries))
	}
}

func TestList_V2Format(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)

	m.Config.Version = 2
	m.Config.Entries = []config.Entry{
		{
			Name:   "test-config",
			Backup: "./test",
			Files:  []string{},
			Targets: map[string]string{
				"linux": "~/.config/test",
			},
		},
		{
			Name:   "git-entry",
			Repo:   "https://github.com/test/repo.git",
			Branch: "main",
			Targets: map[string]string{
				"linux": "~/.local/share/test",
			},
		},
	}

	// List should not error
	err := m.List()
	if err != nil {
		t.Errorf("List() error = %v", err)
	}
}

func TestList_V3Format(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)

	m.Config.Version = 3
	m.Config.Applications = []config.Application{
		{
			Name:        "test-app",
			Description: "Test application",
			Entries: []config.SubEntry{
				{
					Name:   "config",
					Type:   "config",
					Backup: "./test",
					Targets: map[string]string{
						"linux": "~/.config/test",
					},
				},
				{
					Name:   "repo",
					Type:   "git",
					Repo:   "https://github.com/test/repo.git",
					Branch: "main",
					Targets: map[string]string{
						"linux": "~/.local/share/test",
					},
				},
			},
			Package: &config.EntryPackage{
				Managers: map[string]string{
					"pacman": "test-package",
				},
			},
		},
	}

	// List should not error
	err := m.List()
	if err != nil {
		t.Errorf("List() error = %v", err)
	}
}
