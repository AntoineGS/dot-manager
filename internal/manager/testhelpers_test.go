package manager

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/platform"
)

func skipIfNoSymlink(t *testing.T) {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("symlinks require elevated privileges on Windows")
	}
}

// skipIfNoSudo skips tests that assert on the Linux sudo code path. The
// template-copy primitives use native access on Windows and reject elevated
// copy-template operations on unsupported Unix platforms.
func skipIfNoSudo(t *testing.T) {
	t.Helper()

	if runtime.GOOS != platform.OSLinux {
		t.Skip("sudo code paths are Linux-only")
	}
}

// newUtilityManager returns a Manager suitable for calling utility methods
// such as copyFile, copyDir, removeAll, mergeFile, createSymlink, etc. in
// tests. It uses the real OS filesystem and command runner, so callers can
// assert against the real filesystem.
func newUtilityManager() *Manager {
	cfg := &config.Config{}
	plat := &platform.Platform{OS: platform.OSLinux, EnvVars: map[string]string{}}
	return New(cfg, plat)
}

// testPathExists reports whether the path exists on the real filesystem.
// It is used by tests to assert filesystem state after Manager operations.
func testPathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// testIsSymlink reports whether the path is a symbolic link on the real
// filesystem. It mirrors the behavior of Manager.isSymlink.
func testIsSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return true
	}

	// On Windows, directory junctions (mklink /J) are not reported as
	// ModeSymlink in recent Go versions, but Readlink still resolves them.
	if runtime.GOOS == platform.OSWindows {
		_, err := os.Readlink(path)
		return err == nil
	}

	return false
}

// testHasTemplateFiles reports whether dir contains any .tmpl files on the
// real filesystem. It is used only by unit tests of the template detection
// helper and mirrors the behavior of Manager.hasTemplateFiles.
func testHasTemplateFiles(dir string) bool {
	if !testPathExists(dir) {
		return false
	}

	found := false
	_ = filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || found {
			return filepath.SkipDir
		}
		if !d.IsDir() && filepath.Ext(d.Name()) == ".tmpl" {
			found = true
			return filepath.SkipAll
		}
		return nil
	})

	return found
}
