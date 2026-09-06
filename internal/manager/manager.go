package manager

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"runtime"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/fsys"
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
	fs             fsys.FS
	runner         cmdexec.Runner
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
		fs:             fsys.OsFS{},
		runner:         cmdexec.OsRunner{},
	}
}

// WithFS returns a new Manager with the given filesystem implementation.
// Used primarily for testing with an in-memory filesystem.
func (m *Manager) WithFS(f fsys.FS) *Manager {
	m2 := *m
	m2.fs = f
	return &m2
}

// WithRunner returns a new Manager with the given command runner.
// Used primarily for testing with a stub command runner.
func (m *Manager) WithRunner(r cmdexec.Runner) *Manager {
	m2 := *m
	m2.runner = r
	return &m2
}

// InitStateStore initializes the SQLite state store for template render history.
// The database is placed in the backup root directory.
func (m *Manager) InitStateStore() error {
	backupRoot := config.ExpandPath(m.Config.BackupRoot, m.Platform.EnvVars)
	dbPath := filepath.Join(backupRoot, ".tidydots.db")

	store, err := state.Open(m.ctx, dbPath)
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
	apps := m.Config.GetFilteredApplicationsWithLogger(m.templateEngine, m.logger)
	result := make([]config.Application, 0, len(apps))
	for _, app := range apps {
		app.Entries = config.FilterSubEntriesWithLogger(app.Entries, m.templateEngine, m.logger)
		if len(app.Entries) > 0 || app.HasPackage() {
			result = append(result, app)
		}
	}
	return result
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

// HasOutdatedTemplates returns true if the selected .tmpl files in the backup
// directory need rendering. A template is considered outdated when:
//   - It has never been rendered (no render record in state store)
//   - Its current SHA256 hash differs from the hash stored at last render
//   - Its rendered output is missing or cannot be read
//
// An empty files selection discovers templates recursively. Returns false if
// the state store is nil, the directory doesn't exist, or has no templates.
func (m *Manager) HasOutdatedTemplates(backupDir string, files []string) bool {
	outdated := false
	_ = m.walkTemplateFiles(backupDir, files, func(path, _ string, record *state.RenderRecord) error {
		// No render record = template never rendered = outdated
		if record == nil {
			outdated = true
			return filepath.SkipAll
		}

		if _, readErr := m.fs.ReadFile(tmpl.RenderedPath(path)); readErr != nil {
			outdated = true
			return filepath.SkipAll
		}

		content, readErr := m.fs.ReadFile(path)
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

// HasModifiedRenderedFiles returns true if the selected templates in the backup
// directory contain any .tmpl.rendered files whose content differs from the
// pure render baseline stored in the state store. This indicates the user has
// manually edited a rendered template file.
//
// An empty files selection discovers templates recursively. Returns false if
// the state store is nil, the directory doesn't exist, or has no templates.
func (m *Manager) HasModifiedRenderedFiles(backupDir string, files []string) bool {
	modified := false
	_ = m.walkTemplateFiles(backupDir, files, func(path, _ string, record *state.RenderRecord) error {
		if record == nil {
			return nil
		}

		renderedPath := tmpl.RenderedPath(path)
		renderedContent, readErr := m.fs.ReadFile(renderedPath)
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
	return m.hasTemplateFiles(dir)
}

// isSymlink reports whether the path is a symbolic link, using the Manager's
// filesystem abstraction. On Windows, directory junctions (mklink /J) that do
// not carry the ModeSymlink bit are still detected via Readlink.
func (m *Manager) isSymlink(path string) bool {
	info, err := m.fs.Lstat(path)
	if err != nil {
		return false
	}

	if info.Mode()&os.ModeSymlink != 0 {
		return true
	}

	// On Windows, directory junctions (mklink /J) are not reported as
	// ModeSymlink in recent Go versions, but Readlink still resolves them.
	if runtime.GOOS == platform.OSWindows {
		_, err := m.fs.Readlink(path)
		return err == nil
	}

	return false
}

// pathExists reports whether the path exists, using the Manager's filesystem
// abstraction. It uses Lstat so that broken symlinks are still reported as
// existing.
func (m *Manager) pathExists(path string) bool {
	_, err := m.fs.Lstat(path)
	return err == nil
}

// PathExists is an exported alias of pathExists for callers that need to
// check path existence against the Manager's filesystem.
func (m *Manager) PathExists(path string) bool {
	return m.pathExists(path)
}

// copyFile copies a file from src to dst using the Manager's filesystem
// abstraction. Directories are created as needed and permissions are preserved.
func (m *Manager) copyFile(src, dst string) error {
	data, err := m.fs.ReadFile(src)
	if err != nil {
		return fmt.Errorf("opening source: %w", err)
	}

	srcInfo, err := m.fs.Stat(src)
	if err != nil {
		return fmt.Errorf("stating source: %w", err)
	}

	if err := m.fs.MkdirAll(filepath.Dir(dst), DirPerms); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	if err := m.fs.WriteFile(dst, data, srcInfo.Mode().Perm()); err != nil {
		return fmt.Errorf("writing destination: %w", err)
	}

	return nil
}

// copyDir recursively copies a directory tree from src to dst using the
// Manager's filesystem abstraction.
func (m *Manager) copyDir(src, dst string) error {
	srcInfo, err := m.fs.Stat(src)
	if err != nil {
		return err
	}

	if err := m.fs.MkdirAll(dst, srcInfo.Mode().Perm()); err != nil {
		return err
	}

	entries, err := m.fs.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := m.copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := m.copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// removeAll removes the file or directory at path. Symlinks are left intact
// so that only the underlying target is considered for removal.
func (m *Manager) removeAll(path string) error {
	info, err := m.fs.Lstat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}

		return err
	}

	// Don't remove symlinks - just the target
	if info.Mode()&os.ModeSymlink != 0 {
		return nil
	}

	return m.fs.RemoveAll(path)
}

// hasTemplateFiles reports whether the directory contains any .tmpl files.
// It walks the directory tree and returns early on the first match.
func (m *Manager) hasTemplateFiles(dir string) bool {
	if !m.pathExists(dir) {
		return false
	}

	found := false
	_ = m.fs.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error {
		if err != nil || found {
			return filepath.SkipDir
		}
		if !d.IsDir() && tmpl.IsTemplateFile(d.Name()) {
			found = true
			return filepath.SkipAll
		}
		return nil
	})

	return found
}

// ModifiedTemplate contains the diff data for a single modified template file.
type ModifiedTemplate struct {
	TemplatePath  string // absolute path to .tmpl source file
	CurrentPath   string // absolute path to the current rendered/deployed file
	RelPath       string // relative path within backup dir
	PureRender    []byte // baseline content from state DB
	CurrentOnDisk []byte // current .tmpl.rendered content on disk
}

// GetModifiedTemplateFiles returns selected .tmpl files in the backup directory
// whose rendered output on disk differs from the pure render stored in the state DB.
// An empty files selection discovers templates recursively.
func (m *Manager) GetModifiedTemplateFiles(backupDir string, files []string) ([]ModifiedTemplate, error) {
	if m.stateStore == nil {
		return nil, nil
	}

	var result []ModifiedTemplate

	err := m.walkTemplateFiles(backupDir, files, func(path, relPath string, record *state.RenderRecord) error {
		if record == nil {
			return nil
		}

		renderedPath := tmpl.RenderedPath(path)
		renderedContent, readErr := m.fs.ReadFile(renderedPath)
		if readErr != nil {
			return nil
		}

		if !bytes.Equal(renderedContent, record.PureRender) {
			result = append(result, ModifiedTemplate{
				TemplatePath:  path,
				CurrentPath:   renderedPath,
				RelPath:       relPath,
				PureRender:    record.PureRender,
				CurrentOnDisk: renderedContent,
			})
		}

		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking backup directory: %w", err)
	}

	return result, nil
}
