package manager

import (
	"fmt"
	"io/fs"
	"path/filepath"

	"github.com/AntoineGS/tidydots/internal/state"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

// templateWalkFunc is called for each template file found during walking.
// path is the absolute path to the .tmpl file, relPath is relative to backupDir,
// and record is the latest render record from the state store (may be nil).
type templateWalkFunc func(path, relPath string, record *state.RenderRecord) error

// walkTemplateFiles discovers the selected templates in backupDir. An empty
// selection retains recursive folder discovery; a non-empty selection visits
// only the listed .tmpl source paths.
func (m *Manager) walkTemplateFiles(backupDir string, files []string, fn templateWalkFunc) error {
	if m.stateStore == nil {
		return nil
	}

	if len(files) > 0 {
		return m.walkSelectedTemplateFiles(backupDir, files, fn)
	}

	if !m.hasTemplateFiles(backupDir) {
		return nil
	}

	return m.fs.WalkDir(backupDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
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

		record, lookupErr := m.stateStore.GetLatestRender(m.ctx, normalizeStateKey(relPath), m.Platform.OS, m.Platform.Hostname)
		if lookupErr != nil {
			return nil
		}

		return fn(path, relPath, record)
	})
}

func (m *Manager) walkSelectedTemplateFiles(backupDir string, files []string, fn templateWalkFunc) error {
	for _, file := range files {
		if !tmpl.IsTemplateFile(file) {
			continue
		}

		relPath, err := validateLocalTemplatePath(file)
		if err != nil {
			return NewPathError("status", file, err)
		}

		path := filepath.Join(backupDir, relPath)
		if err := m.validateTemplatePath(backupDir, path, "template selection"); err != nil {
			return err
		}

		info, err := m.fs.Lstat(path)
		if err != nil || info.IsDir() {
			continue
		}
		if !info.Mode().IsRegular() {
			return NewPathError("status", path, fmt.Errorf("template selection is not a regular file"))
		}

		relPath, relErr := filepath.Rel(backupDir, path)
		if relErr != nil {
			continue
		}

		record, lookupErr := m.stateStore.GetLatestRender(m.ctx, normalizeStateKey(relPath), m.Platform.OS, m.Platform.Hostname)
		if lookupErr != nil {
			continue
		}

		if err := fn(path, relPath, record); err != nil {
			return err
		}
	}

	return nil
}
