package manager

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

func setupTestManager(t *testing.T) *Manager {
	t.Helper()
	tmpDir := t.TempDir()

	// Create a simple config with some entries
	cfg := &config.Config{
		Version:    2,
		BackupRoot: tmpDir,
		Entries: []config.Entry{
			{Name: "test-entry", Backup: "./test", Targets: map[string]string{"linux": filepath.Join(tmpDir, "target")}},
		},
	}

	plat := &platform.Platform{OS: platform.OSLinux}
	mgr := New(cfg, plat)
	mgr.Verbose = true

	// Create source directory
	srcDir := filepath.Join(tmpDir, "test")
	if err := os.MkdirAll(srcDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(srcDir, "config.txt"), []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	return mgr
}

func TestRestore_ContextCancellation(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)

	// Create context that cancels immediately
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel before operation starts

	err := m.RestoreWithContext(ctx)

	if err != context.Canceled {
		t.Errorf("RestoreWithContext() error = %v, want context.Canceled", err)
	}
}

func TestRestore_ContextTimeout(t *testing.T) {
	t.Parallel()
	m := setupTestManager(t)

	// Create context with very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancel()

	time.Sleep(10 * time.Millisecond) // Ensure timeout

	err := m.RestoreWithContext(ctx)

	if err != context.DeadlineExceeded {
		t.Errorf("RestoreWithContext() error = %v, want context.DeadlineExceeded", err)
	}
}
