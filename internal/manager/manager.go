package manager

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/platform"
	"github.com/AntoineGS/tidydots/internal/state"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

// File permissions constants
const (
	// DirPerms are the default permissions for created directories (rwxr-x---)
	// Owner: read, write, execute; Group: read, execute; Other: none
	DirPerms os.FileMode = 0750

	// FilePerms are the default permissions for created files (rw-------)
	// Owner: read, write; Group: none; Other: none
	FilePerms os.FileMode = 0600
)

// Manager handles dotfile operations including backup, restore, and listing of configuration entries.
// It maintains references to the configuration, platform information, and operational settings.
type Manager struct {
	ctx            context.Context
	Config         *config.Config
	Platform       *platform.Platform
	logger         *slog.Logger
	templateEngine *tmpl.Engine
	stateStore     *state.Store
	DryRun         bool
	Verbose        bool
	NoMerge        bool
	ForceDelete    bool
	ForceRender    bool
}

// New creates a new Manager instance with the given configuration and platform information.
// The Manager is initialized with structured logging using slog.
func New(cfg *config.Config, plat *platform.Platform) *Manager {
	// Create default logger
	opts := &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}
	handler := slog.NewTextHandler(os.Stdout, opts)

	// Create template engine
	tmplCtx := tmpl.NewContextFromPlatform(plat)
	engine := tmpl.NewEngine(tmplCtx)

	return &Manager{
		Config:         cfg,
		Platform:       plat,
		ctx:            context.Background(), // Default context
		logger:         slog.New(handler),
		templateEngine: engine,
	}
}

// InitStateStore initializes the SQLite state store for template render history.
// The database is placed in the backup root directory.
func (m *Manager) InitStateStore() error {
	backupRoot := config.ExpandPath(m.Config.BackupRoot, m.Platform.EnvVars)
	dbPath := filepath.Join(backupRoot, ".tidydots.db")

	store, err := state.Open(dbPath)
	if err != nil {
		return fmt.Errorf("opening state store: %w", err)
	}

	m.stateStore = store
	return nil
}

// Close releases resources held by the Manager, including the state store.
func (m *Manager) Close() error {
	if m.stateStore != nil {
		return m.stateStore.Close()
	}
	return nil
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

// WithVerbose returns a new Manager with adjusted log level based on verbose flag.
// This follows the builder pattern used by WithContext and WithLogger.
func (m *Manager) WithVerbose(verbose bool) *Manager {
	m2 := *m
	m2.Verbose = verbose

	level := slog.LevelInfo
	if verbose {
		level = slog.LevelDebug
	}

	opts := &slog.HandlerOptions{Level: level}
	handler := slog.NewTextHandler(os.Stdout, opts)
	m2.logger = slog.New(handler)

	return &m2
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
	return m.Config.GetFilteredApplications(m.templateEngine)
}

// resolvePath expands templates, ~ and environment variables in paths and resolves
// relative paths against BackupRoot. This ensures paths work correctly even when
// stored with ~ in config.
func (m *Manager) resolvePath(path string) string {
	// Expand templates, ~ and env vars in the path
	expandedPath := config.ExpandPathWithTemplate(path, m.Platform.EnvVars, m.templateEngine)

	// If it's already absolute after expansion, return it
	if filepath.IsAbs(expandedPath) {
		return expandedPath
	}

	// Otherwise, resolve relative to BackupRoot (also expand BackupRoot)
	expandedBackupRoot := config.ExpandPathWithTemplate(m.Config.BackupRoot, m.Platform.EnvVars, m.templateEngine)
	return filepath.Join(expandedBackupRoot, expandedPath)
}

// expandTarget expands templates, ~ and environment variables in a target path.
// Target paths are typically absolute paths like ~/.config/nvim that need
// expansion before use in file operations.
func (m *Manager) expandTarget(target string) string {
	return config.ExpandPathWithTemplate(target, m.Platform.EnvVars, m.templateEngine)
}

// HasOutdatedTemplates returns true if the backup directory contains any .tmpl files
// that need rendering. A template is considered outdated when:
//   - It has never been rendered (no render record in state store)
//   - Its current SHA256 hash differs from the hash stored at last render
//
// Returns false if the state store is nil, the directory doesn't exist, or has no templates.
func (m *Manager) HasOutdatedTemplates(backupDir string) bool {
	if m.stateStore == nil {
		return false
	}

	if !hasTemplateFiles(backupDir) {
		return false
	}

	outdated := false
	_ = filepath.WalkDir(backupDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || outdated {
			return filepath.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		if tmpl.IsRenderedFile(d.Name()) || tmpl.IsConflictFile(d.Name()) {
			return nil
		}

		if !tmpl.IsTemplateFile(d.Name()) {
			return nil
		}

		relPath, relErr := filepath.Rel(backupDir, path)
		if relErr != nil {
			return nil
		}

		record, lookupErr := m.stateStore.GetLatestRender(relPath)
		if lookupErr != nil {
			return nil
		}

		// No render record = template never rendered = outdated
		if record == nil {
			outdated = true
			return filepath.SkipAll
		}

		content, readErr := os.ReadFile(path) //nolint:gosec // path from config
		if readErr != nil {
			return nil
		}

		hash := fmt.Sprintf("%x", sha256.Sum256(content))
		if hash != record.TemplateHash {
			outdated = true
			return filepath.SkipAll
		}

		return nil
	})

	return outdated
}

// HasModifiedRenderedFiles returns true if the backup directory contains any
// .tmpl.rendered files whose content differs from the pure render baseline
// stored in the state store. This indicates the user has manually edited
// a rendered template file.
//
// Returns false if the state store is nil, the directory doesn't exist, or has no templates.
func (m *Manager) HasModifiedRenderedFiles(backupDir string) bool {
	if m.stateStore == nil {
		return false
	}

	if !hasTemplateFiles(backupDir) {
		return false
	}

	modified := false
	_ = filepath.WalkDir(backupDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || modified {
			return filepath.SkipDir
		}

		if d.IsDir() {
			return nil
		}

		if tmpl.IsRenderedFile(d.Name()) || tmpl.IsConflictFile(d.Name()) {
			return nil
		}

		if !tmpl.IsTemplateFile(d.Name()) {
			return nil
		}

		relPath, relErr := filepath.Rel(backupDir, path)
		if relErr != nil {
			return nil
		}

		record, lookupErr := m.stateStore.GetLatestRender(relPath)
		if lookupErr != nil || record == nil {
			return nil
		}

		renderedPath := tmpl.RenderedPath(path)
		renderedContent, readErr := os.ReadFile(renderedPath) //nolint:gosec // path from config
		if readErr != nil {
			return nil
		}

		if !bytes.Equal(renderedContent, record.PureRender) {
			modified = true
			return filepath.SkipAll
		}

		return nil
	})

	return modified
}

// HasTemplateFiles returns true if the directory contains any .tmpl files.
func (m *Manager) HasTemplateFiles(dir string) bool {
	return hasTemplateFiles(dir)
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

	if mkdirErr := os.MkdirAll(filepath.Dir(dst), DirPerms); mkdirErr != nil {
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
