package manager

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/AntoineGS/dot-manager/internal/config"
	"github.com/AntoineGS/dot-manager/internal/platform"
)

// Manager handles dotfile operations including backup, restore, and listing of configuration entries.
// It maintains references to the configuration, platform information, and operational settings.
type Manager struct {
	ctx       context.Context
	Config    *config.Config
	Platform  *platform.Platform
	FilterCtx *config.FilterContext
	logger    *slog.Logger
	DryRun    bool
	Verbose   bool
}

// New creates a new Manager instance with the given configuration and platform information.
// The Manager is initialized with structured logging using slog.
func New(cfg *config.Config, plat *platform.Platform) *Manager {
	// Create default logger
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)

	return &Manager{
		Config:   cfg,
		Platform: plat,
		FilterCtx: &config.FilterContext{
			OS:       plat.OS,
			Distro:   plat.Distro,
			Hostname: plat.Hostname,
			User:     plat.User,
		},
		ctx:    context.Background(), // Default context
		logger: slog.New(handler),
	}
}

// WithContext returns a new Manager with the given context
func (m *Manager) WithContext(ctx context.Context) *Manager {
	m2 := *m
	m2.ctx = ctx

	return &m2
}

// WithLogger sets a custom logger
func (m *Manager) WithLogger(logger *slog.Logger) *Manager {
	m2 := *m
	m2.logger = logger

	return &m2
}

// SetVerbose adjusts log level based on verbose flag
func (m *Manager) SetVerbose(verbose bool) {
	m.Verbose = verbose

	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{Level: level}
	handler := slog.NewTextHandler(os.Stdout, opts)
	m.logger = slog.New(handler)
}

// checkContext checks if context is canceled and returns error
func (m *Manager) checkContext() error {
	select {
	case <-m.ctx.Done():
		return m.ctx.Err()
	default:
		return nil
	}
}

// GetApplications returns all filtered applications from the configuration.
func (m *Manager) GetApplications() []config.Application {
	return m.Config.GetFilteredApplications(m.FilterCtx)
}

func (m *Manager) logf(format string, args ...interface{}) {
	m.logger.Info(fmt.Sprintf(format, args...))
}

func (m *Manager) logVerbosef(format string, args ...interface{}) {
	m.logger.Debug(fmt.Sprintf(format, args...))
}

func (m *Manager) logWarnf(format string, args ...interface{}) {
	m.logger.Warn(fmt.Sprintf(format, args...))
}

func (m *Manager) logErrorf(format string, args ...interface{}) {
	m.logger.Error(fmt.Sprintf(format, args...))
}

// logEntryRestore logs restore operations with structured attributes
func (m *Manager) logEntryRestore(entry config.Entry, target string, err error) {
	if err != nil {
		m.logger.Error("restore failed",
			slog.String("entry", entry.Name),
			slog.String("target", target),
			slog.String("error", err.Error()),
		)
	} else {
		m.logger.Info("restore complete",
			slog.String("entry", entry.Name),
			slog.String("target", target),
		)
	}
}

// resolvePath expands ~ and environment variables in paths and resolves relative paths
// against BackupRoot. This ensures paths work correctly even when stored with ~ in config.
func (m *Manager) resolvePath(path string) string {
	// Expand ~ and env vars in the path
	expandedPath := config.ExpandPath(path, m.Platform.EnvVars)

	// If it's already absolute after expansion, return it
	if filepath.IsAbs(expandedPath) {
		return expandedPath
	}

	// Otherwise, resolve relative to BackupRoot (also expand BackupRoot)
	expandedBackupRoot := config.ExpandPath(m.Config.BackupRoot, m.Platform.EnvVars)
	return filepath.Join(expandedBackupRoot, expandedPath)
}

// expandTarget expands ~ and environment variables in a target path.
// Target paths are typically absolute paths like ~/.config/nvim that need
// expansion before use in file operations.
func (m *Manager) expandTarget(target string) string {
	return config.ExpandPath(target, m.Platform.EnvVars)
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
	srcFile, openErr := os.Open(src) //nolint:gosec // file path from config
	if openErr != nil {
		return fmt.Errorf("opening source: %w", openErr)
	}

	defer func() {
		if closeErr := srcFile.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("closing source file: %w", closeErr)
		}
	}()

	srcInfo, statErr := srcFile.Stat()
	if statErr != nil {
		return fmt.Errorf("stating source: %w", statErr)
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(dst), 0750); mkdirErr != nil {
		return fmt.Errorf("creating destination directory: %w", mkdirErr)
	}

	dstFile, createErr := os.Create(dst) //nolint:gosec // path from config
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
