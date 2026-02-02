package manager

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

type Manager struct {
	Config    *config.Config
	Platform  *platform.Platform
	FilterCtx *config.FilterContext
	DryRun    bool
	Verbose   bool
}

func New(cfg *config.Config, plat *platform.Platform) *Manager {
	return &Manager{
		Config:   cfg,
		Platform: plat,
		FilterCtx: &config.FilterContext{
			OS:       plat.OS,
			Distro:   plat.Distro,
			Hostname: plat.Hostname,
			User:     plat.User,
		},
	}
}

func (m *Manager) GetPaths() []config.PathSpec {
	return m.Config.GetPaths(m.Platform.IsRoot)
}

func (m *Manager) GetEntries() []config.Entry {
	return m.Config.GetFilteredConfigEntries(m.Platform.IsRoot, m.FilterCtx)
}

func (m *Manager) GetGitEntries() []config.Entry {
	return m.Config.GetFilteredGitEntries(m.Platform.IsRoot, m.FilterCtx)
}

func (m *Manager) GetPackageEntries() []config.Entry {
	return m.Config.GetFilteredPackageEntries(m.FilterCtx)
}

func (m *Manager) log(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func (m *Manager) logVerbose(format string, args ...interface{}) {
	if m.Verbose {
		fmt.Printf(format+"\n", args...)
	}
}

func (m *Manager) resolvePath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(m.Config.BackupRoot, path)
}

func isSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

func pathExists(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

func copyFile(src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return err
	}

	return os.Chmod(dst, srcInfo.Mode())
}

func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dst, srcInfo.Mode()); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

func removeAll(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	// Don't remove symlinks - just the target
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}

	return os.RemoveAll(path)
}
