package manager

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/AntoineGS/tidydots/internal/state"
	"github.com/AntoineGS/tidydots/internal/template"
)

func TestSelectedTemplateStatusAndDiffIgnoreUnselectedFiles(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, _, mgr, _ := setupTemplateTest(t)
	backupDir := filepath.Join(backupRoot, "config")
	if err := os.MkdirAll(backupDir, 0o750); err != nil {
		t.Fatal(err)
	}

	firstPath := filepath.Join(backupDir, "first.tmpl")
	secondPath := filepath.Join(backupDir, "second.tmpl")
	writeTemplateFile(t, firstPath, "first-v1")
	writeTemplateFile(t, secondPath, "second-v1")
	if err := mgr.renderTemplateAndLink(firstPath, "first.tmpl"); err != nil {
		t.Fatalf("render first template: %v", err)
	}
	if err := mgr.renderTemplateAndLink(secondPath, "second.tmpl"); err != nil {
		t.Fatalf("render second template: %v", err)
	}

	writeTemplateFile(t, secondPath, "second-v2")
	writeTemplateFile(t, template.RenderedPath(secondPath), "second-user-edit")

	if mgr.HasOutdatedTemplates(backupDir, []string{"first.tmpl"}) {
		t.Fatal("selected first template reported outdated because unselected template changed")
	}
	if mgr.HasModifiedRenderedFiles(backupDir, []string{"first.tmpl"}) {
		t.Fatal("selected first template reported modified because unselected output changed")
	}
	modified, err := mgr.GetModifiedTemplateFiles(backupDir, []string{"first.tmpl"})
	if err != nil {
		t.Fatalf("selected diff discovery: %v", err)
	}
	if len(modified) != 0 {
		t.Fatalf("selected diff = %+v, want empty", modified)
	}

	if !mgr.HasOutdatedTemplates(backupDir, []string{"second.tmpl"}) {
		t.Fatal("selected second template did not report its source change")
	}
	if !mgr.HasModifiedRenderedFiles(backupDir, []string{"second.tmpl"}) {
		t.Fatal("selected second template did not report its rendered edit")
	}
	modified, err = mgr.GetModifiedTemplateFiles(backupDir, []string{"second.tmpl"})
	if err != nil {
		t.Fatalf("selected second diff discovery: %v", err)
	}
	if len(modified) != 1 {
		t.Fatalf("selected second diff count = %d, want 1", len(modified))
	}
	if modified[0].TemplatePath != secondPath {
		t.Errorf("selected diff template path = %q, want %q", modified[0].TemplatePath, secondPath)
	}
	if modified[0].RenderedPath != template.RenderedPath(secondPath) {
		t.Errorf("selected diff rendered path = %q, want %q", modified[0].RenderedPath, template.RenderedPath(secondPath))
	}
	if modified[0].RelPath != "second.tmpl" {
		t.Errorf("selected diff relative path = %q, want %q", modified[0].RelPath, "second.tmpl")
	}

	if !mgr.HasOutdatedTemplates(backupDir, nil) {
		t.Fatal("nil selection did not retain recursive folder discovery")
	}
	modified, err = mgr.GetModifiedTemplateFiles(backupDir, nil)
	if err != nil {
		t.Fatalf("recursive diff discovery: %v", err)
	}
	if len(modified) != 1 || modified[0].TemplatePath != secondPath {
		t.Fatalf("recursive diff = %+v, want only second template", modified)
	}
}

func TestSelectedTemplateStatusTreatsMissingRenderedOutputAsOutdated(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, _, mgr, _ := setupTemplateTest(t)
	backupDir := filepath.Join(backupRoot, "config")
	templatePath := filepath.Join(backupDir, "config.tmpl")
	writeTemplateFile(t, templatePath, "content")
	if err := mgr.renderTemplateAndLink(templatePath, "config.tmpl"); err != nil {
		t.Fatalf("render template: %v", err)
	}
	if err := os.Remove(template.RenderedPath(templatePath)); err != nil {
		t.Fatal(err)
	}

	if !mgr.HasOutdatedTemplates(backupDir, []string{"config.tmpl"}) {
		t.Fatal("missing rendered output was reported healthy")
	}
}

func TestSelectedTemplateDiscoveryVisitsOnlyListedTemplates(t *testing.T) {
	backupRoot, _, mgr, _ := setupTemplateTest(t)
	backupDir := filepath.Join(backupRoot, "config")
	selectedPath := filepath.Join(backupDir, "nested", "selected.tmpl")
	unselectedPath := filepath.Join(backupDir, "nested", "unselected.tmpl")
	writeTemplateFile(t, selectedPath, "selected")
	writeTemplateFile(t, unselectedPath, "unselected")

	var visited []string
	err := mgr.walkTemplateFiles(backupDir, []string{"nested/selected.tmpl", "ordinary.conf"},
		func(path, relPath string, _ *state.RenderRecord) error {
			visited = append(visited, path+"|"+relPath)
			return nil
		})
	if err != nil {
		t.Fatalf("walk selected templates: %v", err)
	}
	if len(visited) != 1 || visited[0] != selectedPath+"|nested/selected.tmpl" {
		t.Fatalf("visited templates = %v, want only selected source", visited)
	}

	visited = nil
	err = mgr.walkTemplateFiles(backupDir, []string{"ordinary.conf"},
		func(path, relPath string, _ *state.RenderRecord) error {
			visited = append(visited, path+"|"+relPath)
			return nil
		})
	if err != nil {
		t.Fatalf("walk non-template selection: %v", err)
	}
	if len(visited) != 0 {
		t.Fatalf("non-template selection visited %v, want nothing", visited)
	}
}

func TestSelectedTemplateDiscoveryRejectsEscapingSelection(t *testing.T) {
	backupRoot, _, mgr, _ := setupTemplateTest(t)
	backupDir := filepath.Join(backupRoot, "config")
	if err := os.MkdirAll(backupDir, 0o750); err != nil {
		t.Fatal(err)
	}
	neighborPath := filepath.Join(backupRoot, "neighbor.tmpl")
	writeTemplateFile(t, neighborPath, "outside entry")

	var visited []string
	err := mgr.walkTemplateFiles(backupDir, []string{"../neighbor.tmpl"},
		func(path, relPath string, _ *state.RenderRecord) error {
			visited = append(visited, path+"|"+relPath)
			return nil
		})
	if err == nil {
		t.Fatal("selected discovery accepted a path outside the entry")
	}
	if len(visited) != 0 {
		t.Fatalf("selected discovery visited outside paths: %v", visited)
	}
}

func TestSelectedTemplateDiscoveryRejectsSymlinkParentSelection(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, _, mgr, _ := setupTemplateTest(t)
	backupDir := filepath.Join(backupRoot, "config")
	if err := os.MkdirAll(backupDir, 0o750); err != nil {
		t.Fatal(err)
	}
	outside := t.TempDir()
	writeTemplateFile(t, filepath.Join(outside, "neighbor.tmpl"), "outside entry")
	if err := os.Symlink(outside, filepath.Join(backupDir, "nested")); err != nil {
		t.Fatal(err)
	}

	var visited []string
	callback := func(path, relPath string, _ *state.RenderRecord) error {
		visited = append(visited, path+"|"+relPath)
		return nil
	}
	err := mgr.walkTemplateFiles(backupDir, []string{"nested/neighbor.tmpl"}, callback)
	if err == nil {
		t.Fatal("selected discovery accepted a symlink-parent escape")
	}
	if len(visited) != 0 {
		t.Fatalf("selected discovery visited symlink-parent paths: %v", visited)
	}
}

func TestSelectedTemplateDiscoveryKeepsNormalizedNestedStateKey(t *testing.T) {
	backupRoot, _, mgr, store := setupTemplateTest(t)
	backupDir := filepath.Join(backupRoot, "config")
	templatePath := filepath.Join(backupDir, "nested", "config.tmpl")
	writeTemplateFile(t, templatePath, "content")
	if err := mgr.renderTemplateAndLink(templatePath, "nested/config.tmpl"); err != nil {
		t.Fatalf("render nested template: %v", err)
	}

	record, err := store.GetLatestRender(mgr.ctx, "nested/config.tmpl", "linux", "testhost")
	if err != nil {
		t.Fatalf("get nested render record: %v", err)
	}
	if record == nil {
		t.Fatal("nested render record is missing")
	}

	var gotRelPath string
	err = mgr.walkTemplateFiles(backupDir, []string{"nested/config.tmpl"},
		func(_, relPath string, gotRecord *state.RenderRecord) error {
			gotRelPath = relPath
			if gotRecord == nil {
				t.Fatal("selected nested template callback received a nil render record")
			}
			return nil
		})
	if err != nil {
		t.Fatalf("walk nested template: %v", err)
	}
	if gotRelPath != "nested/config.tmpl" {
		t.Fatalf("selected relative path = %q, want %q", gotRelPath, "nested/config.tmpl")
	}
}

func TestSelectedTemplateMethodsWithNoTemplateSelectionVisitNothing(t *testing.T) {
	backupRoot, _, mgr, _ := setupTemplateTest(t)
	backupDir := filepath.Join(backupRoot, "config")
	writeTemplateFile(t, filepath.Join(backupDir, "config.tmpl"), "content")

	if mgr.HasOutdatedTemplates(backupDir, []string{"config.conf"}) {
		t.Fatal("non-template selection reported an outdated template")
	}
	if mgr.HasModifiedRenderedFiles(backupDir, []string{"config.conf"}) {
		t.Fatal("non-template selection reported a modified template")
	}
	modified, err := mgr.GetModifiedTemplateFiles(backupDir, []string{"config.conf"})
	if err != nil {
		t.Fatalf("non-template diff discovery: %v", err)
	}
	if len(modified) != 0 {
		t.Fatalf("non-template selection diff = %+v, want empty", modified)
	}
}
