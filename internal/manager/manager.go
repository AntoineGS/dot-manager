package manager

import (
	"context"
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
	ctx       context.Context // Internal context
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
		ctx: context.Background(), // Default context
	}
}

// WithContext returns a new Manager with the given context
func (m *Manager) WithContext(ctx context.Context) *Manager {
	m2 := *m
	m2.ctx = ctx
	return &m2
}

// checkContext checks if context is cancelled and returns error
func (m *Manager) checkContext() error {
	select {
	case <-m.ctx.Done():
		return m.ctx.Err()
	default:
		return nil
	}
}

func (m *Manager) GetPaths() []config.PathSpec {
	return m.Config.GetPaths()
}

func (m *Manager) GetEntries() []config.Entry {
	return m.Config.GetFilteredConfigEntries(m.FilterCtx)
}

func (m *Manager) GetGitEntries() []config.Entry {
	return m.Config.GetFilteredGitEntries(m.FilterCtx)
}

func (m *Manager) GetPackageEntries() []config.Entry {
	return m.Config.GetFilteredPackageEntries(m.FilterCtx)
}

func (m *Manager) GetApplications() []config.Application {
	return m.Config.GetFilteredApplications(m.FilterCtx)
}

func (m *Manager) log(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}

func (m *Manager) logVerbose(format string, args ...interface{}) {
	if m.Verbose {
		fmt.Printf(format+"\n", args...)
	}
}

func (m *Manager) logWarn(format string, args ...interface{}) {
	fmt.Printf("[WARN] "+format+"\n", args...)
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

func copyFile(src, dst string) (err error) {
	srcFile, openErr := os.Open(src)
	if openErr != nil {
		return fmt.Errorf("opening source: %w", openErr)
	}
	defer srcFile.Close() // Explicit close

	srcInfo, statErr := srcFile.Stat()
	if statErr != nil {
		return fmt.Errorf("stating source: %w", statErr)
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(dst), 0755); mkdirErr != nil {
		return fmt.Errorf("creating destination directory: %w", mkdirErr)
	}

	dstFile, createErr := os.Create(dst)
	if createErr != nil {
		return fmt.Errorf("creating destination: %w", createErr)
	}
	defer func() {
		if cerr := dstFile.Close(); cerr != nil && err == nil {
			err = fmt.Errorf("closing destination: %w", cerr)
		}
	}()

	if _, err = io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copying data: %w", err)
	}

	// Explicitly sync before setting permissions
	if err = dstFile.Sync(); err != nil {
		return fmt.Errorf("syncing destination: %w", err)
	}

	if err = os.Chmod(dst, srcInfo.Mode()); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}

	return nil
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
