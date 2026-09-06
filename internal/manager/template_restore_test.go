package manager

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/fsys"
	"github.com/AntoineGS/tidydots/internal/platform"
	"github.com/AntoineGS/tidydots/internal/state"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

const expectedHostnameRender = "Host=testhost"

// setupTemplateTest creates a temporary directory structure for template tests.
// Returns (backupRoot, targetDir, manager, stateStore).
func setupTemplateTest(t *testing.T) (string, string, *Manager, *state.Store) {
	t.Helper()

	backupRoot := t.TempDir()
	targetDir := t.TempDir()

	plat := &platform.Platform{
		OS:       "linux",
		Distro:   "arch",
		Hostname: "testhost",
		User:     "testuser",
		EnvVars:  make(map[string]string),
	}

	cfg := &config.Config{
		BackupRoot: backupRoot,
		Version:    3,
	}

	mgr := New(cfg, plat)

	dbPath := filepath.Join(backupRoot, ".tidydots.db")
	store, err := state.Open(context.Background(), dbPath)
	if err != nil {
		t.Fatalf("failed to open state store: %v", err)
	}
	mgr.stateStore = store
	t.Cleanup(func() { _ = store.Close() }) //nolint:errcheck // cleanup is best-effort

	return backupRoot, targetDir, mgr, store
}

// verifyFolderSymlink checks that target is a symlink pointing to source.
func verifyFolderSymlink(t *testing.T, target, source string) {
	t.Helper()

	if !testIsSymlink(target) {
		t.Fatalf("expected %q to be a symlink", target)
	}

	link, err := os.Readlink(target)
	if err != nil {
		t.Fatalf("failed to read symlink %q: %v", target, err)
	}

	if link != source {
		t.Errorf("folder symlink %q should point to %q, got %q", target, source, link)
	}
}

// verifyRelativeSymlink checks that symlinkPath is a relative symlink pointing to expectedTarget.
func verifyRelativeSymlink(t *testing.T, symlinkPath, expectedTarget string) {
	t.Helper()

	if !testIsSymlink(symlinkPath) {
		t.Fatalf("expected %q to be a symlink", symlinkPath)
	}

	link, err := os.Readlink(symlinkPath)
	if err != nil {
		t.Fatalf("failed to read symlink %q: %v", symlinkPath, err)
	}

	if link != expectedTarget {
		t.Errorf("relative symlink %q should point to %q, got %q", symlinkPath, expectedTarget, link)
	}
}

func TestRestoreFolderWithTemplates_MixedFiles(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, targetDir, mgr, _ := setupTemplateTest(t)

	// Create backup with mixed template and non-template files
	backupDir := filepath.Join(backupRoot, "zsh")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Template file
	tmplContent := `# ZSH config for {{ .Hostname }}
{{ if eq .OS "linux" }}export EDITOR=nvim{{ end }}
`
	if err := os.WriteFile(filepath.Join(backupDir, ".zshrc.tmpl"), []byte(tmplContent), 0600); err != nil {
		t.Fatal(err)
	}

	// Regular file
	regularContent := "# This is a regular file\n"
	if err := os.WriteFile(filepath.Join(backupDir, ".zshenv"), []byte(regularContent), 0600); err != nil {
		t.Fatal(err)
	}

	subEntry := config.SubEntry{
		Name:    "zsh",
		Backup:  "./zsh",
		Targets: map[string]string{"linux": targetDir},
	}

	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatalf("RestoreFolderWithTemplates failed: %v", err)
	}

	// Verify folder symlink: targetDir → backupDir
	verifyFolderSymlink(t, targetDir, backupDir)

	// Verify rendered file exists
	renderedPath := filepath.Join(backupDir, ".zshrc.tmpl.rendered")
	if !testPathExists(renderedPath) {
		t.Fatal("rendered file should exist")
	}
	renderedContent, _ := os.ReadFile(renderedPath) //nolint:gosec
	if !strings.Contains(string(renderedContent), "testhost") {
		t.Errorf("rendered file should contain hostname, got %q", string(renderedContent))
	}
	if !strings.Contains(string(renderedContent), "export EDITOR=nvim") {
		t.Errorf("rendered file should contain linux-conditional content, got %q", string(renderedContent))
	}

	// Verify relative symlink in backup: .zshrc → .zshrc.tmpl.rendered
	verifyRelativeSymlink(t, filepath.Join(backupDir, ".zshrc"), ".zshrc.tmpl.rendered")

	// Verify .zshrc is accessible through the folder symlink chain
	chainedPath := filepath.Join(targetDir, ".zshrc")
	chainedContent, err := os.ReadFile(chainedPath) //nolint:gosec
	if err != nil {
		t.Fatalf("should be able to read .zshrc through symlink chain: %v", err)
	}
	if !strings.Contains(string(chainedContent), "testhost") {
		t.Errorf("chained read should resolve to rendered content, got %q", string(chainedContent))
	}

	// Verify .zshenv is accessible through the folder symlink (no individual symlink needed)
	zshenvPath := filepath.Join(targetDir, ".zshenv")
	zshenvContent, err := os.ReadFile(zshenvPath) //nolint:gosec
	if err != nil {
		t.Fatalf("should be able to read .zshenv through folder symlink: %v", err)
	}
	if string(zshenvContent) != regularContent {
		t.Errorf(".zshenv content should match, got %q", string(zshenvContent))
	}
}

func TestRestoreFolderWithTemplates_RendersContext(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, targetDir, mgr, _ := setupTemplateTest(t)

	backupDir := filepath.Join(backupRoot, "config")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

	tmplContent := "OS={{ .OS }} Distro={{ .Distro }} Host={{ .Hostname }} User={{ .User }}"
	if err := os.WriteFile(filepath.Join(backupDir, "info.tmpl"), []byte(tmplContent), 0600); err != nil {
		t.Fatal(err)
	}

	subEntry := config.SubEntry{
		Name:    "config",
		Backup:  "./config",
		Targets: map[string]string{"linux": targetDir},
	}

	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}

	// Verify folder symlink
	verifyFolderSymlink(t, targetDir, backupDir)

	// Verify rendered content
	renderedPath := filepath.Join(backupDir, "info.tmpl.rendered")
	content, _ := os.ReadFile(renderedPath) //nolint:gosec
	want := "OS=linux Distro=arch Host=testhost User=testuser"
	if string(content) != want {
		t.Errorf("rendered content = %q, want %q", string(content), want)
	}

	// Verify relative symlink
	verifyRelativeSymlink(t, filepath.Join(backupDir, "info"), "info.tmpl.rendered")
}

func TestRestoreFolderWithTemplates_ReRenderWithUserEdits(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, targetDir, mgr, store := setupTemplateTest(t)

	backupDir := filepath.Join(backupRoot, "zsh")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

	// First render: template v1
	tmplV1 := "export EDITOR=vim\nexport PATH=$PATH:/usr/local/bin\n"
	tmplPath := filepath.Join(backupDir, ".zshrc.tmpl")
	if err := os.WriteFile(tmplPath, []byte(tmplV1), 0600); err != nil {
		t.Fatal(err)
	}

	subEntry := config.SubEntry{
		Name:    "zsh",
		Backup:  "./zsh",
		Targets: map[string]string{"linux": targetDir},
	}

	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}

	// Verify folder symlink
	verifyFolderSymlink(t, targetDir, backupDir)

	// Simulate user editing the rendered file
	renderedPath := filepath.Join(backupDir, ".zshrc.tmpl.rendered")
	userEdited := "export EDITOR=nvim\nexport PATH=$PATH:/usr/local/bin\n"
	if err := os.WriteFile(renderedPath, []byte(userEdited), 0600); err != nil {
		t.Fatal(err)
	}

	// Template v2: change PATH
	tmplV2 := "export EDITOR=vim\nexport PATH=$PATH:/usr/local/bin:/opt/bin\n"
	if err := os.WriteFile(tmplPath, []byte(tmplV2), 0600); err != nil {
		t.Fatal(err)
	}

	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}

	// Read merged result
	merged, _ := os.ReadFile(renderedPath) //nolint:gosec

	// Should have user's EDITOR change preserved
	if !strings.Contains(string(merged), "EDITOR=nvim") {
		t.Errorf("user edit should be preserved, got %q", string(merged))
	}

	// Should have template's PATH change applied
	if !strings.Contains(string(merged), "/opt/bin") {
		t.Errorf("template change should be applied, got %q", string(merged))
	}

	// Verify DB stores pure render (not merged result)
	record, _ := store.GetLatestRender(context.Background(), ".zshrc.tmpl", "linux", "testhost")
	if record == nil {
		t.Fatal("expected render record in DB")
	}
	if strings.Contains(string(record.PureRender), "nvim") {
		t.Error("DB should store pure render without user edits")
	}

	// Verify relative symlink
	verifyRelativeSymlink(t, filepath.Join(backupDir, ".zshrc"), ".zshrc.tmpl.rendered")
}

func TestRestoreFolderWithTemplates_MultiCycle(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, targetDir, mgr, _ := setupTemplateTest(t)

	backupDir := filepath.Join(backupRoot, "config")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

	tmplPath := filepath.Join(backupDir, "app.conf.tmpl")
	renderedPath := filepath.Join(backupDir, "app.conf.tmpl.rendered")
	subEntry := config.SubEntry{
		Name:    "config",
		Backup:  "./config",
		Targets: map[string]string{"linux": targetDir},
	}

	// Cycle 1: initial render
	if err := os.WriteFile(tmplPath, []byte("port=8080\nhost=localhost\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}

	// Verify folder symlink
	verifyFolderSymlink(t, targetDir, backupDir)

	// User edits cycle 1 output: changes port
	if err := os.WriteFile(renderedPath, []byte("port=9090\nhost=localhost\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Cycle 2: template changes host
	if err := os.WriteFile(tmplPath, []byte("port=8080\nhost=0.0.0.0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}

	content2, _ := os.ReadFile(renderedPath) //nolint:gosec
	if !strings.Contains(string(content2), "port=9090") {
		t.Errorf("Cycle 2: user's port change should survive, got %q", string(content2))
	}
	if !strings.Contains(string(content2), "host=0.0.0.0") {
		t.Errorf("Cycle 2: template's host change should be applied, got %q", string(content2))
	}

	// User edits cycle 2 output: adds debug line
	if err := os.WriteFile(renderedPath, []byte("port=9090\nhost=0.0.0.0\ndebug=true\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Cycle 3: template changes port back
	if err := os.WriteFile(tmplPath, []byte("port=3000\nhost=0.0.0.0\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}

	content3, _ := os.ReadFile(renderedPath) //nolint:gosec
	if !strings.Contains(string(content3), "port=3000") {
		t.Errorf("Cycle 3: template's port change should be applied, got %q", string(content3))
	}
	if strings.Contains(string(content3), "debug=true") {
		t.Errorf("Cycle 3: conflicted user edits should not be deployed, got %q", string(content3))
	}

	// Verify relative symlink persists through cycles
	verifyRelativeSymlink(t, filepath.Join(backupDir, "app.conf"), "app.conf.tmpl.rendered")
}

func TestRestoreFolderWithTemplates_Conflict(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, targetDir, mgr, _ := setupTemplateTest(t)

	backupDir := filepath.Join(backupRoot, "config")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

	tmplPath := filepath.Join(backupDir, "app.conf.tmpl")
	renderedPath := filepath.Join(backupDir, "app.conf.tmpl.rendered")
	conflictPath := filepath.Join(backupDir, "app.conf.tmpl.conflict")
	subEntry := config.SubEntry{
		Name:    "config",
		Backup:  "./config",
		Targets: map[string]string{"linux": targetDir},
	}

	// Initial render
	if err := os.WriteFile(tmplPath, []byte("line1\nline2\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}

	// Verify folder symlink
	verifyFolderSymlink(t, targetDir, backupDir)

	// User changes line2 to "user-change"
	if err := os.WriteFile(renderedPath, []byte("line1\nuser-change\n"), 0600); err != nil {
		t.Fatal(err)
	}

	// Template changes line2 to "template-change"
	if err := os.WriteFile(tmplPath, []byte("line1\ntemplate-change\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}

	// Conflict file should exist
	if !testPathExists(conflictPath) {
		t.Fatal("conflict file should exist")
	}

	renderedContent, err := os.ReadFile(renderedPath) //nolint:gosec
	if err != nil {
		t.Fatalf("read rendered file: %v", err)
	}
	if got, want := string(renderedContent), "line1\ntemplate-change\n"; got != want {
		t.Errorf("rendered content = %q, want fresh template output %q", got, want)
	}
	if strings.Contains(string(renderedContent), "<<<<<<<") {
		t.Errorf("rendered file should not contain conflict markers, got %q", string(renderedContent))
	}

	conflictContent, err := os.ReadFile(conflictPath) //nolint:gosec
	if err != nil {
		t.Fatalf("read conflict file: %v", err)
	}
	for _, want := range []string{"<<<<<<< user-edits", "user-change", "=======", "template-change", ">>>>>>> template"} {
		if !strings.Contains(string(conflictContent), want) {
			t.Errorf("conflict file should contain %q, got %q", want, string(conflictContent))
		}
	}

	// Verify relative symlink
	verifyRelativeSymlink(t, filepath.Join(backupDir, "app.conf"), "app.conf.tmpl.rendered")
}

func TestRestoreFolderWithTemplates_HashUnchanged_SkipsReRender(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, targetDir, mgr, _ := setupTemplateTest(t)

	backupDir := filepath.Join(backupRoot, "config")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

	tmplContent := "static content"
	if err := os.WriteFile(filepath.Join(backupDir, "file.tmpl"), []byte(tmplContent), 0600); err != nil {
		t.Fatal(err)
	}

	subEntry := config.SubEntry{
		Name:    "config",
		Backup:  "./config",
		Targets: map[string]string{"linux": targetDir},
	}

	// First render
	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}

	// Verify folder symlink
	verifyFolderSymlink(t, targetDir, backupDir)

	renderedPath := filepath.Join(backupDir, "file.tmpl.rendered")
	info1, _ := os.Stat(renderedPath)

	// Second render with same template - should skip
	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}

	info2, _ := os.Stat(renderedPath)
	// File should not have been rewritten (modtime unchanged)
	if info1.ModTime() != info2.ModTime() {
		t.Error("rendered file should not be rewritten when template is unchanged")
	}

	// Verify relative symlink
	verifyRelativeSymlink(t, filepath.Join(backupDir, "file"), "file.tmpl.rendered")
}

func TestRestoreFolderWithTemplates_DryRun(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, targetDir, mgr, store := setupTemplateTest(t)
	mgr.DryRun = true

	backupDir := filepath.Join(backupRoot, "config")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(backupDir, "file.tmpl"), []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}

	subEntry := config.SubEntry{
		Name:    "config",
		Backup:  "./config",
		Targets: map[string]string{"linux": targetDir},
	}

	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}

	// No folder symlink should be created
	if testIsSymlink(targetDir) {
		t.Error("dry run should not create folder symlink")
	}

	// No rendered file should be created
	renderedPath := filepath.Join(backupDir, "file.tmpl.rendered")
	if testPathExists(renderedPath) {
		t.Error("dry run should not create rendered file")
	}

	// No DB records
	record, _ := store.GetLatestRender(context.Background(), "file.tmpl", "linux", "testhost")
	if record != nil {
		t.Error("dry run should not create DB records")
	}

	// No relative symlink in backup
	if testIsSymlink(filepath.Join(backupDir, "file")) {
		t.Error("dry run should not create relative symlink")
	}
}

func TestRestoreFolderWithTemplates_ForceRender(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, targetDir, mgr, _ := setupTemplateTest(t)
	mgr.ForceRender = true

	backupDir := filepath.Join(backupRoot, "config")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

	tmplPath := filepath.Join(backupDir, "file.tmpl")
	renderedPath := filepath.Join(backupDir, "file.tmpl.rendered")

	// Initial render
	if err := os.WriteFile(tmplPath, []byte("version1"), 0600); err != nil {
		t.Fatal(err)
	}
	subEntry := config.SubEntry{
		Name:    "config",
		Backup:  "./config",
		Targets: map[string]string{"linux": targetDir},
	}
	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}

	// Verify folder symlink
	verifyFolderSymlink(t, targetDir, backupDir)

	// User edits
	if err := os.WriteFile(renderedPath, []byte("user-edited"), 0600); err != nil {
		t.Fatal(err)
	}

	// Template changes
	if err := os.WriteFile(tmplPath, []byte("version2"), 0600); err != nil {
		t.Fatal(err)
	}

	// Force render should overwrite without merge
	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}

	content, _ := os.ReadFile(renderedPath) //nolint:gosec
	if string(content) != "version2" {
		t.Errorf("force render should overwrite, got %q", string(content))
	}

	// Verify relative symlink
	verifyRelativeSymlink(t, filepath.Join(backupDir, "file"), "file.tmpl.rendered")
}

func TestRestoreFolderWithTemplates_Idempotent(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, targetDir, mgr, _ := setupTemplateTest(t)

	backupDir := filepath.Join(backupRoot, "config")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(backupDir, "file.tmpl"), []byte("Host={{ .Hostname }}"), 0600); err != nil {
		t.Fatal(err)
	}

	subEntry := config.SubEntry{
		Name:    "config",
		Backup:  "./config",
		Targets: map[string]string{"linux": targetDir},
	}

	// Run twice
	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}
	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}

	// Should produce same result
	renderedPath := filepath.Join(backupDir, "file.tmpl.rendered")
	content, _ := os.ReadFile(renderedPath) //nolint:gosec
	if string(content) != expectedHostnameRender {
		t.Errorf("expected Host=testhost, got %q", string(content))
	}

	// Verify folder symlink
	verifyFolderSymlink(t, targetDir, backupDir)

	// Verify relative symlink is stable
	verifyRelativeSymlink(t, filepath.Join(backupDir, "file"), "file.tmpl.rendered")
}

func TestRestoreFolderWithTemplates_SkipsRenderedAndConflictFiles(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, targetDir, mgr, _ := setupTemplateTest(t)

	backupDir := filepath.Join(backupRoot, "config")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Create template + stale rendered and conflict files
	if err := os.WriteFile(filepath.Join(backupDir, "file.tmpl"), []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "file.tmpl.rendered"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backupDir, "file.tmpl.conflict"), []byte("old"), 0600); err != nil {
		t.Fatal(err)
	}

	subEntry := config.SubEntry{
		Name:    "config",
		Backup:  "./config",
		Targets: map[string]string{"linux": targetDir},
	}

	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}

	// Verify folder symlink
	verifyFolderSymlink(t, targetDir, backupDir)

	// The rendered file should be overwritten with fresh render, not left as "old"
	renderedPath := filepath.Join(backupDir, "file.tmpl.rendered")
	content, _ := os.ReadFile(renderedPath) //nolint:gosec
	if string(content) != "content" {
		t.Errorf("rendered file should be freshly rendered, got %q", string(content))
	}

	// Verify relative symlink exists for template
	verifyRelativeSymlink(t, filepath.Join(backupDir, "file"), "file.tmpl.rendered")
}

func TestHasTemplateFiles(t *testing.T) {
	dir := t.TempDir()

	// No template files
	if err := os.WriteFile(filepath.Join(dir, "regular.txt"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if testHasTemplateFiles(dir) {
		t.Error("should return false when no .tmpl files")
	}

	// Add template file
	if err := os.WriteFile(filepath.Join(dir, "config.tmpl"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if !testHasTemplateFiles(dir) {
		t.Error("should return true when .tmpl file exists")
	}
}

func TestHasTemplateFiles_Nested(t *testing.T) {
	dir := t.TempDir()
	subDir := filepath.Join(dir, "nested")
	if err := os.MkdirAll(subDir, 0750); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(subDir, "config.tmpl"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if !testHasTemplateFiles(dir) {
		t.Error("should detect .tmpl files in subdirectories")
	}
}

func TestHasTemplateFiles_NonExistent(t *testing.T) {
	if testHasTemplateFiles("/nonexistent/path") {
		t.Error("should return false for non-existent directory")
	}
}

func TestIsTemplateFile_Consistency(t *testing.T) {
	// Verify consistency between template and engine helpers
	tests := []struct {
		filename string
		isTmpl   bool
	}{
		{".zshrc.tmpl", true},
		{".zshrc.tmpl.rendered", false},
		{".zshrc.tmpl.conflict", false},
		{".zshrc", false},
	}

	for _, tt := range tests {
		if got := tmpl.IsTemplateFile(tt.filename); got != tt.isTmpl {
			t.Errorf("IsTemplateFile(%q) = %v, want %v", tt.filename, got, tt.isTmpl)
		}
	}
}

func TestRestoreFolderWithTemplates_SubDirectories(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, targetDir, mgr, _ := setupTemplateTest(t)

	backupDir := filepath.Join(backupRoot, "config")
	subDir := filepath.Join(backupDir, "subdir")
	if err := os.MkdirAll(subDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Template in subdirectory
	if err := os.WriteFile(filepath.Join(subDir, "nested.tmpl"), []byte("Host={{ .Hostname }}"), 0600); err != nil {
		t.Fatal(err)
	}
	// Regular file in root
	if err := os.WriteFile(filepath.Join(backupDir, "regular.txt"), []byte("hello"), 0600); err != nil {
		t.Fatal(err)
	}

	subEntry := config.SubEntry{
		Name:    "config",
		Backup:  "./config",
		Targets: map[string]string{"linux": targetDir},
	}

	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}

	// Verify folder symlink
	verifyFolderSymlink(t, targetDir, backupDir)

	// Verify nested template was rendered
	renderedPath := filepath.Join(subDir, "nested.tmpl.rendered")
	if !testPathExists(renderedPath) {
		t.Fatal("nested rendered file should exist")
	}

	content, _ := os.ReadFile(renderedPath) //nolint:gosec
	if string(content) != expectedHostnameRender {
		t.Errorf("nested template should render correctly, got %q", string(content))
	}

	// Verify relative symlink in subdirectory
	verifyRelativeSymlink(t, filepath.Join(subDir, "nested"), "nested.tmpl.rendered")

	// Verify content accessible through symlink chain
	chainedPath := filepath.Join(targetDir, "subdir", "nested")
	chainedContent, err := os.ReadFile(chainedPath) //nolint:gosec
	if err != nil {
		t.Fatalf("should read nested file through symlink chain: %v", err)
	}
	if string(chainedContent) != expectedHostnameRender {
		t.Errorf("chained read should resolve correctly, got %q", string(chainedContent))
	}
}

func TestRestoreFolderWithTemplates_ExistingFileAtSymlinkPath(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, targetDir, mgr, _ := setupTemplateTest(t)

	backupDir := filepath.Join(backupRoot, "config")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Create template
	if err := os.WriteFile(filepath.Join(backupDir, "app.conf.tmpl"), []byte("port=8080"), 0600); err != nil {
		t.Fatal(err)
	}

	// Create a plain file where the relative symlink should go
	if err := os.WriteFile(filepath.Join(backupDir, "app.conf"), []byte("old content"), 0600); err != nil {
		t.Fatal(err)
	}

	subEntry := config.SubEntry{
		Name:    "config",
		Backup:  "./config",
		Targets: map[string]string{"linux": targetDir},
	}

	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}

	// The plain file should be replaced by a relative symlink
	verifyRelativeSymlink(t, filepath.Join(backupDir, "app.conf"), "app.conf.tmpl.rendered")

	// Content should resolve to rendered template
	content, err := os.ReadFile(filepath.Join(backupDir, "app.conf")) //nolint:gosec
	if err != nil {
		t.Fatalf("should read through relative symlink: %v", err)
	}
	if string(content) != "port=8080" {
		t.Errorf("content should be rendered template, got %q", string(content))
	}
}

func TestRestoreFolderWithTemplates_RelativeSymlinkAlreadyCorrect(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, targetDir, mgr, _ := setupTemplateTest(t)

	backupDir := filepath.Join(backupRoot, "config")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

	// Create template
	if err := os.WriteFile(filepath.Join(backupDir, "file.tmpl"), []byte("content"), 0600); err != nil {
		t.Fatal(err)
	}

	subEntry := config.SubEntry{
		Name:    "config",
		Backup:  "./config",
		Targets: map[string]string{"linux": targetDir},
	}

	// First restore creates everything
	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}

	// Get symlink info before second restore
	symlinkPath := filepath.Join(backupDir, "file")
	info1, err := os.Lstat(symlinkPath)
	if err != nil {
		t.Fatalf("failed to stat symlink: %v", err)
	}

	// Second restore should be idempotent - symlink already correct
	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatal(err)
	}

	info2, err := os.Lstat(symlinkPath)
	if err != nil {
		t.Fatalf("failed to stat symlink after second restore: %v", err)
	}

	// Symlink should not have been recreated (same modtime)
	if info1.ModTime() != info2.ModTime() {
		t.Error("correct relative symlink should not be recreated")
	}

	verifyRelativeSymlink(t, symlinkPath, "file.tmpl.rendered")
}

func TestHasOutdatedTemplates(t *testing.T) {
	t.Run("NoStateStore", func(t *testing.T) {
		plat := &platform.Platform{
			OS:       "linux",
			Distro:   "arch",
			Hostname: "testhost",
			User:     "testuser",
			EnvVars:  make(map[string]string),
		}
		cfg := &config.Config{BackupRoot: t.TempDir(), Version: 3}
		mgr := New(cfg, plat)
		// stateStore is nil by default

		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "file.tmpl"), []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}

		if mgr.HasOutdatedTemplates(dir, nil) {
			t.Error("should return false when stateStore is nil")
		}
	})

	t.Run("NoTemplateFiles", func(t *testing.T) {
		_, _, mgr, _ := setupTemplateTest(t)

		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "regular.txt"), []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}

		if mgr.HasOutdatedTemplates(dir, nil) {
			t.Error("should return false when no .tmpl files exist")
		}
	})

	t.Run("NoRenderRecord", func(t *testing.T) {
		_, _, mgr, _ := setupTemplateTest(t)

		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "file.tmpl"), []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}

		if !mgr.HasOutdatedTemplates(dir, nil) {
			t.Error("should return true when template has never been rendered")
		}
	})

	t.Run("HashMatches", func(t *testing.T) {
		skipIfNoSymlink(t)
		backupRoot, targetDir, mgr, _ := setupTemplateTest(t)

		backupDir := filepath.Join(backupRoot, "config")
		if err := os.MkdirAll(backupDir, 0750); err != nil {
			t.Fatal(err)
		}

		tmplContent := "Host={{ .Hostname }}"
		if err := os.WriteFile(filepath.Join(backupDir, "file.tmpl"), []byte(tmplContent), 0600); err != nil {
			t.Fatal(err)
		}

		subEntry := config.SubEntry{
			Name:    "config",
			Backup:  "./config",
			Targets: map[string]string{"linux": targetDir},
		}

		if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
			t.Fatal(err)
		}

		if mgr.HasOutdatedTemplates(backupDir, nil) {
			t.Error("should return false when template hash matches stored hash")
		}
	})

	t.Run("HashDiffers", func(t *testing.T) {
		skipIfNoSymlink(t)
		backupRoot, targetDir, mgr, _ := setupTemplateTest(t)

		backupDir := filepath.Join(backupRoot, "config")
		if err := os.MkdirAll(backupDir, 0750); err != nil {
			t.Fatal(err)
		}

		tmplPath := filepath.Join(backupDir, "file.tmpl")
		if err := os.WriteFile(tmplPath, []byte("version1"), 0600); err != nil {
			t.Fatal(err)
		}

		subEntry := config.SubEntry{
			Name:    "config",
			Backup:  "./config",
			Targets: map[string]string{"linux": targetDir},
		}

		if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
			t.Fatal(err)
		}

		// Modify template after render
		if err := os.WriteFile(tmplPath, []byte("version2"), 0600); err != nil {
			t.Fatal(err)
		}

		if !mgr.HasOutdatedTemplates(backupDir, nil) {
			t.Error("should return true when template hash differs from stored hash")
		}
	})

	t.Run("MultipleTemplates_OneOutdated", func(t *testing.T) {
		skipIfNoSymlink(t)
		backupRoot, targetDir, mgr, _ := setupTemplateTest(t)

		backupDir := filepath.Join(backupRoot, "config")
		if err := os.MkdirAll(backupDir, 0750); err != nil {
			t.Fatal(err)
		}

		tmplPath1 := filepath.Join(backupDir, "file1.tmpl")
		tmplPath2 := filepath.Join(backupDir, "file2.tmpl")
		if err := os.WriteFile(tmplPath1, []byte("content1"), 0600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(tmplPath2, []byte("content2"), 0600); err != nil {
			t.Fatal(err)
		}

		subEntry := config.SubEntry{
			Name:    "config",
			Backup:  "./config",
			Targets: map[string]string{"linux": targetDir},
		}

		if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
			t.Fatal(err)
		}

		// Modify only one template
		if err := os.WriteFile(tmplPath1, []byte("modified"), 0600); err != nil {
			t.Fatal(err)
		}

		if !mgr.HasOutdatedTemplates(backupDir, nil) {
			t.Error("should return true when at least one template is outdated")
		}
	})
}

func TestHasModifiedRenderedFiles(t *testing.T) {
	t.Run("NoStateStore", func(t *testing.T) {
		plat := &platform.Platform{
			OS:       "linux",
			Distro:   "arch",
			Hostname: "testhost",
			User:     "testuser",
			EnvVars:  make(map[string]string),
		}
		cfg := &config.Config{BackupRoot: t.TempDir(), Version: 3}
		mgr := New(cfg, plat)

		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "file.tmpl"), []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}

		if mgr.HasModifiedRenderedFiles(dir, nil) {
			t.Error("should return false when stateStore is nil")
		}
	})

	t.Run("NoTemplateFiles", func(t *testing.T) {
		_, _, mgr, _ := setupTemplateTest(t)

		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "regular.txt"), []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}

		if mgr.HasModifiedRenderedFiles(dir, nil) {
			t.Error("should return false when no .tmpl files exist")
		}
	})

	t.Run("NoRenderRecord", func(t *testing.T) {
		_, _, mgr, _ := setupTemplateTest(t)

		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "file.tmpl"), []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}

		if mgr.HasModifiedRenderedFiles(dir, nil) {
			t.Error("should return false when no render record exists")
		}
	})

	t.Run("NoRenderedFile", func(t *testing.T) {
		skipIfNoSymlink(t)
		backupRoot, targetDir, mgr, _ := setupTemplateTest(t)

		backupDir := filepath.Join(backupRoot, "config")
		if err := os.MkdirAll(backupDir, 0750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(backupDir, "file.tmpl"), []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}

		subEntry := config.SubEntry{
			Name:    "config",
			Backup:  "./config",
			Targets: map[string]string{"linux": targetDir},
		}

		if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
			t.Fatal(err)
		}

		// Remove the rendered file
		renderedPath := filepath.Join(backupDir, "file.tmpl.rendered")
		if err := os.Remove(renderedPath); err != nil {
			t.Fatal(err)
		}

		if mgr.HasModifiedRenderedFiles(backupDir, nil) {
			t.Error("should return false when rendered file doesn't exist")
		}
	})

	t.Run("ContentMatchesBaseline", func(t *testing.T) {
		skipIfNoSymlink(t)
		backupRoot, targetDir, mgr, _ := setupTemplateTest(t)

		backupDir := filepath.Join(backupRoot, "config")
		if err := os.MkdirAll(backupDir, 0750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(backupDir, "file.tmpl"), []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}

		subEntry := config.SubEntry{
			Name:    "config",
			Backup:  "./config",
			Targets: map[string]string{"linux": targetDir},
		}

		if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
			t.Fatal(err)
		}

		if mgr.HasModifiedRenderedFiles(backupDir, nil) {
			t.Error("should return false when rendered file matches pure render baseline")
		}
	})

	t.Run("ContentDiffersFromBaseline", func(t *testing.T) {
		skipIfNoSymlink(t)
		backupRoot, targetDir, mgr, _ := setupTemplateTest(t)

		backupDir := filepath.Join(backupRoot, "config")
		if err := os.MkdirAll(backupDir, 0750); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(filepath.Join(backupDir, "file.tmpl"), []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}

		subEntry := config.SubEntry{
			Name:    "config",
			Backup:  "./config",
			Targets: map[string]string{"linux": targetDir},
		}

		if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
			t.Fatal(err)
		}

		// User edits the rendered file
		renderedPath := filepath.Join(backupDir, "file.tmpl.rendered")
		if err := os.WriteFile(renderedPath, []byte("user-edited-content"), 0600); err != nil {
			t.Fatal(err)
		}

		if !mgr.HasModifiedRenderedFiles(backupDir, nil) {
			t.Error("should return true when rendered file differs from pure render baseline")
		}
	})
}

func TestWriteFileAtomic_WritesViaTempAndRename(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "rendered.txt")

	// Pre-populate the target with content that must survive a failed write.
	if err := os.WriteFile(target, []byte("old"), 0o644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	mgr := &Manager{fs: fsys.OsFS{}}
	if err := mgr.writeFileAtomic(target, []byte("new"), 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if string(got) != "new" {
		t.Errorf("target content = %q, want %q", got, "new")
	}

	// No leftover temp file in the directory
	entries, err := os.ReadDir(tmp)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tidydots-tmp") {
			t.Errorf("leftover temp file: %s", e.Name())
		}
	}
}

// spyFS records call order for writeFileAtomic tests and can inject errors.
type spyFS struct {
	fsys.OsFS
	calls     []string
	renameErr error
	removeErr error
}

func (s *spyFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	s.calls = append(s.calls, "WriteFile:"+name)
	return s.OsFS.WriteFile(name, data, perm)
}

func (s *spyFS) Rename(oldpath, newpath string) error {
	s.calls = append(s.calls, "Rename:"+oldpath+"->"+newpath)
	if s.renameErr != nil {
		return s.renameErr
	}
	return s.OsFS.Rename(oldpath, newpath)
}

func (s *spyFS) Remove(name string) error {
	s.calls = append(s.calls, "Remove:"+name)
	if s.removeErr != nil {
		return s.removeErr
	}
	return s.OsFS.Remove(name)
}

func TestWriteFileAtomic_CallOrderUsesTempThenRename(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "out.txt")

	spy := &spyFS{}
	mgr := &Manager{fs: spy}

	if err := mgr.writeFileAtomic(target, []byte("data"), 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	if len(spy.calls) != 2 {
		t.Fatalf("expected 2 calls, got %d: %v", len(spy.calls), spy.calls)
	}
	expectedTmp := filepath.Join(tmp, ".out.txt.tidydots-tmp")
	if spy.calls[0] != "WriteFile:"+expectedTmp {
		t.Errorf("call[0] = %q, want WriteFile on temp path %q", spy.calls[0], expectedTmp)
	}
	if spy.calls[1] != "Rename:"+expectedTmp+"->"+target {
		t.Errorf("call[1] = %q, want Rename from temp to target", spy.calls[1])
	}
}

func TestWriteFileAtomic_RenameFailureRemovesTempAndWrapsError(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "out.txt")

	spy := &spyFS{renameErr: errors.New("simulated rename failure")}
	mgr := &Manager{fs: spy}

	err := mgr.writeFileAtomic(target, []byte("data"), 0o644)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "renaming temp file") {
		t.Errorf("error message = %q, want it to mention renaming", err.Error())
	}
	if !strings.Contains(err.Error(), "simulated rename failure") {
		t.Errorf("error message = %q, want it to wrap original error", err.Error())
	}

	// Verify the cleanup Remove was called on the temp path.
	expectedTmp := filepath.Join(tmp, ".out.txt.tidydots-tmp")
	var sawRemove bool
	for _, c := range spy.calls {
		if c == "Remove:"+expectedTmp {
			sawRemove = true
			break
		}
	}
	if !sawRemove {
		t.Errorf("expected Remove call on temp path %q, calls = %v", expectedTmp, spy.calls)
	}
}

func TestWriteFileAtomic_OverwritesStaleTempFile(t *testing.T) {
	tmp := t.TempDir()
	target := filepath.Join(tmp, "out.txt")
	staleTmp := filepath.Join(tmp, ".out.txt.tidydots-tmp")

	// Leftover temp file from a simulated prior crash.
	if err := os.WriteFile(staleTmp, []byte("stale"), 0o644); err != nil {
		t.Fatalf("seed stale: %v", err)
	}

	mgr := &Manager{fs: fsys.OsFS{}}
	if err := mgr.writeFileAtomic(target, []byte("fresh"), 0o644); err != nil {
		t.Fatalf("writeFileAtomic: %v", err)
	}

	got, err := os.ReadFile(target)
	if err != nil {
		t.Fatalf("read target: %v", err)
	}
	if string(got) != "fresh" {
		t.Errorf("target content = %q, want %q", got, "fresh")
	}

	// No leftover temp file after success.
	if _, err := os.Stat(staleTmp); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("stale temp file still present: err=%v", err)
	}
}

func TestRenderTemplateAndLink_RemovesStaleConflictFile(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, targetDir, mgr, _ := setupTemplateTest(t)

	backupDir := filepath.Join(backupRoot, "config")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

	tmplPath := filepath.Join(backupDir, "zshrc.tmpl")
	renderedPath := filepath.Join(backupDir, "zshrc.tmpl.rendered")
	conflictPath := filepath.Join(backupDir, "zshrc.tmpl.conflict")

	subEntry := config.SubEntry{
		Name:    "config",
		Backup:  "./config",
		Targets: map[string]string{"linux": targetDir},
	}

	// Initial render: seed the state store with a record for zshrc.tmpl
	// whose PureRender is the template output ("clean\n").
	if err := os.WriteFile(tmplPath, []byte("clean\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatalf("initial RestoreFolderWithTemplates: %v", err)
	}

	// Plant a stale conflict file from a hypothetical previous run.
	staleConflict := []byte("<<<<<<< user\nstale\n=======\nother\n>>>>>>> template\n")
	if err := os.WriteFile(conflictPath, staleConflict, 0600); err != nil {
		t.Fatalf("seed stale conflict file: %v", err)
	}

	// Remove the rendered file on disk so the unchanged-template early-exit
	// (which requires the rendered file to exist) doesn't fire. This forces
	// the re-render path into the merge branch with record != nil.
	if err := os.Remove(renderedPath); err != nil {
		t.Fatalf("remove rendered file: %v", err)
	}

	// Re-render with the template unchanged. The merge path will see
	// base == theirs == ours == "clean\n", so HasConflict is false and
	// the cleanup branch must remove the stale conflict file.
	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatalf("re-render RestoreFolderWithTemplates: %v", err)
	}

	// Assert: stale conflict file has been removed.
	if _, err := os.Stat(conflictPath); !errors.Is(err, fs.ErrNotExist) {
		t.Errorf("stale conflict file still present: err=%v", err)
	}
}

func TestStateStoreKey_IsForwardSlashNormalized(t *testing.T) {
	// Simulate a Windows-style relative path and assert the state store key
	// is normalized to forward slashes regardless of OS separator.
	rel := filepath.FromSlash("nvim/init.tmpl") // uses OS separator
	got := normalizeStateKey(rel)
	want := "nvim/init.tmpl"
	if got != want {
		t.Errorf("normalizeStateKey(%q) = %q, want %q", rel, got, want)
	}
}
