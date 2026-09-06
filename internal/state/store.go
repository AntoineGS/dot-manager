// Package state manages template render history in a SQLite database.
package state

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite" // Pure-Go SQLite driver
)

// RenderRecord represents a single template render stored in the database.
type RenderRecord struct {
	ID           int64
	TemplatePath string
	PureRender   []byte
	TemplateHash string
	RenderedAt   time.Time
	PlatformOS   string
	PlatformHost string
}

// Store manages the SQLite database for template render history.
type Store struct {
	db   *sql.DB
	path string
}

// Open opens or creates the SQLite database at the given path and runs migrations.
func Open(ctx context.Context, dbPath string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0750); err != nil {
		return nil, fmt.Errorf("creating database directory: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("opening database: %w", err)
	}

	// SQLite supports one writer at a time
	db.SetMaxOpenConns(1)

	// Enable WAL mode for better concurrent access
	if _, err := db.ExecContext(ctx, "PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return nil, fmt.Errorf("setting journal mode: %w", err)
	}

	s := &Store{db: db, path: dbPath}
	if err := s.migrate(ctx); err != nil {
		_ = db.Close() //nolint:errcheck,gosec // best-effort cleanup on error path
		return nil, fmt.Errorf("running migrations: %w", err)
	}

	return s, nil
}

// Path returns the database path used by the store, including any relative
// spelling supplied to Open. Callers use it to protect the active database and
// its sidecars from filesystem operations that must remain repository-safe.
func (s *Store) Path() string {
	return s.path
}

// Close closes the database connection.
func (s *Store) Close() error {
	return s.db.Close()
}

func scanRenderRow(row *sql.Row, context string) (*RenderRecord, error) {
	var r RenderRecord
	var renderedAt string

	err := row.Scan(&r.ID, &r.TemplatePath, &r.PureRender, &r.TemplateHash, &renderedAt, &r.PlatformOS, &r.PlatformHost)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // nil means "not found", distinct from error
	}
	if err != nil {
		return nil, fmt.Errorf("%s: %w", context, err)
	}

	r.RenderedAt, err = parseTime(renderedAt)
	if err != nil {
		return nil, fmt.Errorf("parsing rendered_at: %w", err)
	}

	return &r, nil
}

// GetLatestRender returns the most recent render record for the given template
// path on the specified platform (OS + hostname). Returns nil if none exists.
func (s *Store) GetLatestRender(ctx context.Context, templatePath, platformOS, platformHost string) (*RenderRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, template_path, pure_render, template_hash, rendered_at, platform_os, platform_host
		FROM template_renders
		WHERE template_path = ? AND platform_os = ? AND platform_host = ?
		ORDER BY id DESC
		LIMIT 1
	`, templatePath, platformOS, platformHost)

	return scanRenderRow(row, "querying latest render")
}

// SaveRender stores a new render record for the given template.
func (s *Store) SaveRender(ctx context.Context, templatePath string, pureRender []byte, templateHash, platformOS, hostname string) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO template_renders (template_path, pure_render, template_hash, platform_os, platform_host)
		VALUES (?, ?, ?, ?, ?)
	`, templatePath, pureRender, templateHash, platformOS, hostname)
	if err != nil {
		return fmt.Errorf("saving render: %w", err)
	}

	return nil
}

// GetRenderHistory returns the N most recent render records for the given template.
func (s *Store) GetRenderHistory(ctx context.Context, templatePath string, limit int) ([]RenderRecord, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, template_path, pure_render, template_hash, rendered_at, platform_os, platform_host
		FROM template_renders
		WHERE template_path = ?
		ORDER BY id DESC
		LIMIT ?
	`, templatePath, limit)
	if err != nil {
		return nil, fmt.Errorf("querying render history: %w", err)
	}
	defer func() { _ = rows.Close() }() //nolint:errcheck,gosec // defer close is best-effort

	var records []RenderRecord
	for rows.Next() {
		var r RenderRecord
		var renderedAt string

		if err := rows.Scan(&r.ID, &r.TemplatePath, &r.PureRender, &r.TemplateHash, &renderedAt, &r.PlatformOS, &r.PlatformHost); err != nil {
			return nil, fmt.Errorf("scanning render record: %w", err)
		}

		r.RenderedAt, err = parseTime(renderedAt)
		if err != nil {
			return nil, fmt.Errorf("parsing rendered_at: %w", err)
		}

		records = append(records, r)
	}

	return records, rows.Err()
}

// GetRenderByID returns a specific render record by ID.
func (s *Store) GetRenderByID(ctx context.Context, id int64) (*RenderRecord, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, template_path, pure_render, template_hash, rendered_at, platform_os, platform_host
		FROM template_renders
		WHERE id = ?
	`, id)

	return scanRenderRow(row, "querying render by ID")
}

// PruneHistory keeps only the N most recent renders per template, deleting older ones.
func (s *Store) PruneHistory(ctx context.Context, templatePath string, keepN int) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM template_renders
		WHERE template_path = ?
		AND id NOT IN (
			SELECT id FROM template_renders
			WHERE template_path = ?
			ORDER BY id DESC
			LIMIT ?
		)
	`, templatePath, templatePath, keepN)
	if err != nil {
		return fmt.Errorf("pruning history: %w", err)
	}

	return nil
}

// RemoveTemplate deletes all render records for the given template.
func (s *Store) RemoveTemplate(ctx context.Context, templatePath string) error {
	_, err := s.db.ExecContext(ctx, `
		DELETE FROM template_renders WHERE template_path = ?
	`, templatePath)
	if err != nil {
		return fmt.Errorf("removing template: %w", err)
	}

	return nil
}

// migrate runs schema migrations.
func (s *Store) migrate(ctx context.Context) error {
	currentVersion := s.getSchemaVersion(ctx)

	migrations := []func(context.Context, *sql.Tx) error{
		migrateV1,
	}

	for i := currentVersion; i < len(migrations); i++ {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("beginning migration %d: %w", i+1, err)
		}

		if err := migrations[i](ctx, tx); err != nil {
			_ = tx.Rollback() //nolint:errcheck,gosec // rollback best-effort on migration failure
			return fmt.Errorf("migration %d failed: %w", i+1, err)
		}

		// Update schema version
		if _, err := tx.ExecContext(ctx, `DELETE FROM schema_version`); err != nil {
			_ = tx.Rollback() //nolint:errcheck,gosec // rollback best-effort
			return fmt.Errorf("updating schema version: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO schema_version (version) VALUES (?)`, i+1); err != nil {
			_ = tx.Rollback() //nolint:errcheck,gosec // rollback best-effort
			return fmt.Errorf("inserting schema version: %w", err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("committing migration %d: %w", i+1, err)
		}
	}

	return nil
}

// getSchemaVersion returns the current schema version, or 0 if the schema_version table doesn't exist.
func (s *Store) getSchemaVersion(ctx context.Context) int {
	// Check if schema_version table exists
	var tableName string
	err := s.db.QueryRowContext(ctx, `SELECT name FROM sqlite_master WHERE type='table' AND name='schema_version'`).Scan(&tableName)
	if err != nil {
		return 0
	}

	var version int
	if err := s.db.QueryRowContext(ctx, `SELECT version FROM schema_version LIMIT 1`).Scan(&version); err != nil {
		return 0
	}

	return version
}

// parseTime parses a timestamp string from SQLite, trying multiple formats.
func parseTime(s string) (time.Time, error) {
	formats := []string{
		time.RFC3339,
		"2006-01-02T15:04:05Z",
		"2006-01-02 15:04:05",
	}
	for _, f := range formats {
		if t, err := time.Parse(f, s); err == nil {
			return t, nil
		}
	}
	return time.Time{}, fmt.Errorf("cannot parse time %q", s)
}

// migrateV1 creates the initial schema.
func migrateV1(ctx context.Context, tx *sql.Tx) error {
	stmts := []string{
		`CREATE TABLE IF NOT EXISTS schema_version (
			version INTEGER NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS template_renders (
			id              INTEGER PRIMARY KEY AUTOINCREMENT,
			template_path   TEXT NOT NULL,
			pure_render     BLOB NOT NULL,
			template_hash   TEXT NOT NULL,
			rendered_at     DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			platform_os     TEXT NOT NULL,
			platform_host   TEXT NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_template_renders_path
			ON template_renders(template_path, id DESC)`,
	}

	for _, stmt := range stmts {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("executing %q: %w", stmt[:40], err)
		}
	}

	return nil
}
