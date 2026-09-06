//go:build linux

package manager

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestReadTemplateCopyTarget_NativeOsFSReportsIdentityAndOwnership(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "target")
	if err := os.WriteFile(path, []byte("content"), 0o640); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	got, err := newUtilityManager().readTemplateCopyTarget(path, false)
	if err != nil {
		t.Fatalf("readTemplateCopyTarget: %v", err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		t.Fatalf("native Stat returned %T, want *syscall.Stat_t", info.Sys())
	}
	if !got.Exists || got.RootOwned != (stat.Uid == 0) || !got.IdentityKnown ||
		got.Device != stat.Dev || got.Inode != stat.Ino || got.Mode != 0o640 {
		t.Errorf("snapshot = %+v, want native metadata identity/ownership", got)
	}
}
