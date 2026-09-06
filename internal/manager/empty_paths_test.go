package manager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/platform"
)

func TestEmptyExpandedPathsRejected(t *testing.T) {
	t.Setenv("TIDYDOTS_EMPTY_PATH_TEST", "")
	mgr := New(&config.Config{BackupRoot: t.TempDir()}, &platform.Platform{OS: "linux", Hostname: "home"})
	for _, path := range []string{"", "{{ if eq .Hostname \"work\" }}./nvim{{ end }}", "$TIDYDOTS_EMPTY_PATH_TEST"} {
		for _, resolve := range []func(string) (string, error){mgr.ExpandTarget, mgr.ResolvePath} {
			if got, err := resolve(path); err == nil || got != "" {
				t.Errorf("empty path %q = %q, %v", path, got, err)
			}
		}
	}
	if got, err := mgr.ResolvePath("."); err != nil || got != mgr.Config.BackupRoot {
		t.Fatalf("explicit root = %q, %v", got, err)
	}
	// Programmatic configs historically use cwd when BackupRoot is unset.
	// This exception applies to the raw root only, never an entry path or a
	// non-empty root expression whose expansion happens to be empty.
	mgr.Config.BackupRoot = ""
	if got, err := mgr.ResolvePath("nvim"); err != nil || got != "nvim" {
		t.Fatalf("unset root = %q, %v", got, err)
	}
	for _, root := range []string{"{{ if false }}repo{{ end }}", "$TIDYDOTS_EMPTY_PATH_TEST"} {
		mgr.Config.BackupRoot = root
		if _, err := mgr.ResolvePath("nvim"); err == nil {
			t.Errorf("empty expanded root %q accepted", root)
		}
		if err := mgr.InitStateStore(); err == nil {
			t.Fatal("empty expanded state root accepted")
		}
	}
}

func TestOperationsRejectEmptyExpandedPaths(t *testing.T) {
	t.Setenv("TIDYDOTS_EMPTY_PATH_TEST", "")
	if err := os.Unsetenv("TIDYDOTS_EMPTY_PATH_TEST"); err != nil {
		t.Fatal(err)
	}
	for _, operation := range []string{"restore", "backup"} {
		for _, field := range []string{"backup", "target"} {
			for name, expression := range map[string]string{"template": "{{ if false }}nvim{{ end }}", "environment": "$TIDYDOTS_EMPTY_PATH_TEST"} {
				t.Run(operation+"/"+field+"/"+name, func(t *testing.T) {
					root := t.TempDir()
					repo := filepath.Join(root, "repo")
					target := filepath.Join(root, "target")
					for _, dir := range []string{repo, target} {
						if err := os.Mkdir(dir, 0750); err != nil {
							t.Fatal(err)
						}
						if err := os.WriteFile(filepath.Join(dir, "keep"), []byte(dir), 0600); err != nil {
							t.Fatal(err)
						}
					}
					sub := config.SubEntry{Name: "config", Backup: ".", Targets: map[string]string{"linux": target}}
					if field == "backup" {
						sub.Backup = expression
					} else {
						sub.Targets["linux"] = expression
					}
					cfg := &config.Config{Version: 3, BackupRoot: repo, Applications: []config.Application{{Name: "app", Entries: []config.SubEntry{sub}}}}
					mgr := New(cfg, &platform.Platform{OS: "linux"})
					var err error
					if operation == "restore" {
						err = mgr.Restore()
					} else {
						err = mgr.Backup()
					}
					if err == nil || !strings.Contains(err.Error(), "empty") {
						t.Errorf("expected empty-path error, got %v", err)
					}
					for _, dir := range []string{repo, target} {
						info, statErr := os.Lstat(dir)
						if statErr != nil || !info.IsDir() {
							t.Errorf("directory changed: %s, %v", dir, statErr)
						}
						data, readErr := os.ReadFile(filepath.Join(dir, "keep"))
						if readErr != nil || string(data) != dir {
							t.Errorf("content changed: %q, %v", data, readErr)
						}
						entries, readErr := os.ReadDir(dir)
						if readErr != nil || len(entries) != 1 {
							t.Errorf("directory entries changed: %v, %v", entries, readErr)
						}
					}
				})
			}
		}
	}
}
