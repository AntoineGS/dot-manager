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

func TestEmptyExpandedPathsStatusAndRestore(t *testing.T) {
	t.Setenv("TIDYDOTS_EMPTY_PATH_TEST", "")
	if err := os.Unsetenv("TIDYDOTS_EMPTY_PATH_TEST"); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"backup", "target"} {
		for name, expression := range map[string]string{"template": "{{ if false }}nvim{{ end }}", "environment": "$TIDYDOTS_EMPTY_PATH_TEST"} {
			t.Run(field+"/"+name, func(t *testing.T) {
				root := t.TempDir()
				target := filepath.Join(root, "target")
				sub := config.SubEntry{Name: "config", Backup: ".", Targets: map[string]string{"linux": target}}
				if field == "backup" {
					sub.Backup = expression
				} else {
					sub.Targets["linux"] = expression
				}
				cfg := &config.Config{Version: 3, BackupRoot: root, Applications: []config.Application{{Name: "app", Entries: []config.SubEntry{sub}}}}
				plat := &platform.Platform{OS: "linux"}
				m := NewModel(cfg, plat, false)
				m.Manager = manager.New(cfg, plat)
				item := m.Applications[0].SubItems[0]
				state, diagnostic := detectSubEntryStateStatic(item, plat, cfg, m.Manager)
				if state != StateUnavailable || !strings.Contains(diagnostic, "empty") {
					t.Errorf("status = %v, %q", state, diagnostic)
				}
				if state := m.detectSubEntryState(&item); state != StateUnavailable || !strings.Contains(item.CheckError, "empty") {
					t.Errorf("sync status = %v, %q", state, item.CheckError)
				}
				if ok, message := m.performRestoreSubEntry(item); ok || !strings.Contains(message, "empty") {
					t.Errorf("restore = %v, %q", ok, message)
				}
				entries, err := os.ReadDir(root)
				if err != nil || len(entries) != 0 {
					t.Errorf("filesystem changed: %v, %v", entries, err)
				}
			})
		}
	}
}
