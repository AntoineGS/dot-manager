package manager

import (
	"io/fs"
	"path/filepath"

	"github.com/AntoineGS/tidydots/internal/state"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

// templateWalkFunc is called for each template file found during walking.
// path is the absolute path to the .tmpl file, relPath is relative to backupDir,
// and record is the latest render record from the state store (may be nil).
type templateWalkFunc func(path, relPath string, record *state.RenderRecord) error

// walkTemplateFiles walks backupDir for .tmpl files, filtering out non-template,
// rendered, and conflict files, and calls fn for each template file found.
// It handles the common nil stateStore check and hasTemplateFiles guard.
// Returns nil if stateStore is nil or the directory has no template files.
func (m *Manager) walkTemplateFiles(backupDir string, fn templateWalkFunc) error {
	if m.stateStore == nil {
		return nil
	}

	if !hasTemplateFiles(backupDir) {
		return nil
	}

	return filepath.WalkDir(backupDir, func(path string, d fs.DirEntry, err error) error {
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

		record, lookupErr := m.stateStore.GetLatestRender(relPath)
		if lookupErr != nil {
			return nil
		}

		return fn(path, relPath, record)
	})
}
