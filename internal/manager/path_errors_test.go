package manager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/platform"
)

func TestOperationsRejectInvalidTemplatePaths(t *testing.T) {
	for _, operation := range []string{"restore", "backup", "list"} {
		for _, field := range []string{"target", "backup", "root"} {
			t.Run(operation+"/"+field, func(t *testing.T) {
				root := t.TempDir()
				target := filepath.Join(root, "target")
				if err := os.Mkdir(target, 0750); err != nil {
					t.Fatal(err)
				}
				file := filepath.Join(target, "config")
				if err := os.WriteFile(file, []byte("preserve"), 0600); err != nil {
					t.Fatal(err)
				}
				sub := config.SubEntry{Name: "config", Backup: "backup", Targets: map[string]string{"linux": target}}
				cfg := &config.Config{Version: 3, BackupRoot: root}
				switch field {
				case "target":
					sub.Targets["linux"] = filepath.Join(root, "{{")
				case "backup":
					sub.Backup = "{{"
				case "root":
					cfg.BackupRoot = filepath.Join(root, "{{")
				}
				cfg.Applications = []config.Application{{Name: "app", Entries: []config.SubEntry{sub}}}
				mgr := New(cfg, &platform.Platform{OS: "linux"})
				var err error
				switch operation {
				case "restore":
					err = mgr.Restore()
				case "backup":
					err = mgr.Backup()
				case "list":
					err = mgr.List()
				}
				if err == nil || !strings.Contains(err.Error(), "rendering path") {
					t.Errorf("expected template diagnostic, got %v", err)
				}
				entries, readErr := os.ReadDir(root)
				if readErr != nil || len(entries) != 1 {
					t.Errorf("unexpected filesystem changes: %v, %v", entries, readErr)
				}
				data, readErr := os.ReadFile(file)
				if readErr != nil || string(data) != "preserve" {
					t.Errorf("target changed: %q, %v", data, readErr)
				}
			})
		}
	}
}
