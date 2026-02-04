package manager

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

func TestList_FiltersByOS(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)

	m.Config.Applications = []config.Application{
		{
			Name: "linux-only",
			Filters: []config.Filter{
				{Include: map[string]string{"os": "linux"}},
			},
			Entries: []config.SubEntry{
				{
					Name:   "linux-only",
					Backup: "./linux",
					Targets: map[string]string{
						"linux": "~/.config/linux",
					},
				},
			},
		},
		{
			Name: "windows-only",
			Filters: []config.Filter{
				{Include: map[string]string{"os": "windows"}},
			},
			Entries: []config.SubEntry{
				{
					Name:   "windows-only",
					Backup: "./windows",
					Targets: map[string]string{
						"windows": "~/AppData/windows",
					},
				},
			},
		},
		{
			Name: "no-filter",
			Entries: []config.SubEntry{
				{
					Name:   "no-filter",
					Backup: "./both",
					Targets: map[string]string{
						"linux": "~/.config/both",
					},
				},
			},
		},
	}

	m.Platform.OS = "linux"
	apps := m.GetApplications()

	// Should get linux-only and no-filter
	if len(apps) != 2 {
		t.Errorf("got %d applications, want 2", len(apps))
	}

	names := make(map[string]bool)
	for _, app := range apps {
		names[app.Name] = true
	}

	if !names["linux-only"] {
		t.Error("missing linux-only application")
	}

	if !names["no-filter"] {
		t.Error("missing no-filter application")
	}

	if names["windows-only"] {
		t.Error("should not include windows-only")
	}
}

func TestList_EmptyConfig(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)
	m.Config.Applications = []config.Application{}

	apps := m.GetApplications()

	if len(apps) != 0 {
		t.Errorf("got %d applications, want 0", len(apps))
	}
}

func TestList_V2Format(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)

	m.Config.Version = 3
	m.Config.Applications = []config.Application{
		{
			Name: "test-config",
			Entries: []config.SubEntry{
				{
					Name:   "test-config",
					Backup: "./test",
					Files:  []string{},
					Targets: map[string]string{
						"linux": "~/.config/test",
					},
				},
			},
		},
		{
			Name: "git-app",
			Entries: []config.SubEntry{
				{
					Name: "git-entry",

					Targets: map[string]string{
						"linux": "~/.local/share/test",
					},
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
					Backup: "./test",
					Targets: map[string]string{
						"linux": "~/.config/test",
					},
				},
			},
			Package: &config.EntryPackage{
				Managers: map[string]interface{}{
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

func TestList_SkipsGitEntries(t *testing.T) {
	tmpDir := t.TempDir()

	cfg := &config.Config{
		Version:    3,
		BackupRoot: tmpDir,
		Applications: []config.Application{
			{
				Name: "test-app",
				Entries: []config.SubEntry{
					{

						Name:   "config-entry",
						Backup: "./config",
						Targets: map[string]string{
							"linux": "/home/user/.config/app",
						},
					},
					{

						Name: "git-entry",

						Targets: map[string]string{
							"linux": "/home/user/.local/share/app",
						},
					},
				},
			},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	_ = mgr.List()

	_ = w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	_, _ = buf.ReadFrom(r)
	output := buf.String()

	// Should contain config-entry
	if !strings.Contains(output, "config-entry") {
		t.Error("Expected config-entry in output")
	}

	// Should not contain git-entry
	if strings.Contains(output, "git-entry") {
		t.Error("Did not expect git-entry in output")
	}
}
