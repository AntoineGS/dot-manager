package manager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

func TestRestore_IgnoresGitEntries(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	backupRoot := filepath.Join(tmpDir, "backup")
	if err := os.MkdirAll(backupRoot, 0750); err != nil {
		t.Fatal(err)
	}

	configTarget := filepath.Join(tmpDir, "config-target")
	gitTarget := filepath.Join(tmpDir, "git-target")

	cfg := &config.Config{
		Version:    3,
		BackupRoot: backupRoot,
		Applications: []config.Application{
			{
				Name: "config-app",
				Entries: []config.SubEntry{
					{
						Name:   "config-entry",
						Backup: "./config",
						Targets: map[string]string{
							"linux": configTarget,
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
							"linux": gitTarget,
						},
					},
				},
			},
		},
	}

	// Create backup for config entry
	configBackup := filepath.Join(backupRoot, "config")
	if err := os.MkdirAll(configBackup, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configBackup, "test.conf"), []byte("test"), 0600); err != nil {
		t.Fatal(err)
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	err := mgr.Restore()
	if err != nil {
		t.Fatalf("Restore() error = %v", err)
	}

	// Verify config entry was processed (symlink created)
	if !pathExists(configTarget) {
		t.Error("Expected config target to be created")
	}

	// Verify git entry was NOT processed (directory not created)
	if pathExists(gitTarget) {
		t.Error("Did not expect git target to be created (git entries should be ignored)")
	}
}
