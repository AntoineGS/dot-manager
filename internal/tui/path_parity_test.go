package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/manager"
	"github.com/AntoineGS/tidydots/internal/platform"
)

func TestTemplatePathParity(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	root := t.TempDir()
	plat := &platform.Platform{OS: "linux", Hostname: "test-host"}
	sub := config.SubEntry{Name: "config", Backup: "./{{ .Hostname }}/nvim", Targets: map[string]string{"linux": "~/.config/{{ .Hostname }}/nvim"}}
	cfg := &config.Config{Version: 3, BackupRoot: root, Applications: []config.Application{{Name: "nvim", Entries: []config.SubEntry{sub}}}}
	backup := filepath.Join(root, "test-host", "nvim")
	target := filepath.Join(home, ".config", "test-host", "nvim")
	if err := os.MkdirAll(backup, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(target), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(backup, target); err != nil {
		t.Fatal(err)
	}
	m := NewModel(cfg, plat, false)
	if got, err := m.pathManager().ResolvePath(sub.Backup); err != nil || got != backup {
		t.Errorf("backup = %q, %v; want %q", got, err, backup)
	}
	item := m.Applications[0].SubItems[0]
	if item.Target != target {
		t.Errorf("target = %q, want %q", item.Target, target)
	}
	for _, mgr := range []*manager.Manager{nil, manager.New(cfg, plat)} {
		state, diagnostic := detectSubEntryStateStatic(item, plat, cfg, mgr)
		if state != StateLinked {
			t.Errorf("state = %v (%s), want linked", state, diagnostic)
		}
	}
	if got := m.detectSubEntryState(&item); got != StateLinked {
		t.Errorf("sync state = %v", got)
	}
	// Exercise actual TUI restore, not just preview and status expansion.
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	m.Manager = manager.New(cfg, plat)
	if ok, message := m.performRestoreSubEntry(item); !ok {
		t.Fatal(message)
	}
	if got, err := os.Readlink(target); err != nil || got != backup {
		t.Fatalf("restored target = %q, %v; want %q", got, err, backup)
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := m.Manager.Restore(); err != nil {
		t.Fatal(err)
	}
	if got, err := os.Readlink(target); err != nil || got != backup {
		t.Fatalf("CLI restored target = %q, %v; want %q", got, err, backup)
	}
}

func TestInvalidTemplatePathsFailBeforeRestore(t *testing.T) {
	for _, invalidField := range []string{"target", "backup"} {
		t.Run(invalidField, func(t *testing.T) {
			root := t.TempDir()
			plat := &platform.Platform{OS: "linux"}
			sub := config.SubEntry{Name: "config", Backup: "backup", Targets: map[string]string{"linux": filepath.Join(root, "target")}}
			if invalidField == "target" {
				sub.Targets["linux"] = filepath.Join(root, "{{")
			} else {
				sub.Backup = "{{"
			}
			cfg := &config.Config{Version: 3, BackupRoot: root, Applications: []config.Application{{Name: "app", Entries: []config.SubEntry{sub}}}}
			m := NewModel(cfg, plat, false)
			m.Manager = manager.New(cfg, plat)
			item := m.Applications[0].SubItems[0]
			state, diagnostic := detectSubEntryStateStatic(item, plat, cfg, m.Manager)
			if state != StateUnavailable || !strings.Contains(diagnostic, "rendering path") {
				t.Fatalf("invalid path status = %v, %q", state, diagnostic)
			}
			if ok, message := m.performRestoreSubEntry(item); ok || !strings.Contains(message, "rendering path") {
				t.Fatalf("invalid path restore = %v, %q", ok, message)
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatalf("invalid path caused filesystem changes: %v, %v", entries, err)
			}
		})
	}
}
