package manager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
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
	store, err := state.Open(dbPath)
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

	if !isSymlink(target) {
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

	if !isSymlink(symlinkPath) {
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
	if !pathExists(renderedPath) {
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
	record, _ := store.GetLatestRender(".zshrc.tmpl")
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
	if !strings.Contains(string(content3), "debug=true") {
		t.Errorf("Cycle 3: user's debug addition should survive, got %q", string(content3))
	}

	// Verify relative symlink persists through cycles
	verifyRelativeSymlink(t, filepath.Join(backupDir, "app.conf"), "app.conf.tmpl.rendered")
}

func TestRestoreFolderWithTemplates_Conflict(t *testing.T) {
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
	if !pathExists(conflictPath) {
		t.Fatal("conflict file should exist")
	}
	conflictContent, _ := os.ReadFile(conflictPath) //nolint:gosec
	if !strings.Contains(string(conflictContent), "<<<<<<< user-edits") {
		t.Errorf("conflict file should contain conflict markers, got %q", string(conflictContent))
	}

	// Verify relative symlink
	verifyRelativeSymlink(t, filepath.Join(backupDir, "app.conf"), "app.conf.tmpl.rendered")
}

func TestRestoreFolderWithTemplates_HashUnchanged_SkipsReRender(t *testing.T) {
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
	if isSymlink(targetDir) {
		t.Error("dry run should not create folder symlink")
	}

	// No rendered file should be created
	renderedPath := filepath.Join(backupDir, "file.tmpl.rendered")
	if pathExists(renderedPath) {
		t.Error("dry run should not create rendered file")
	}

	// No DB records
	record, _ := store.GetLatestRender("file.tmpl")
	if record != nil {
		t.Error("dry run should not create DB records")
	}

	// No relative symlink in backup
	if isSymlink(filepath.Join(backupDir, "file")) {
		t.Error("dry run should not create relative symlink")
	}
}

func TestRestoreFolderWithTemplates_ForceRender(t *testing.T) {
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
	if hasTemplateFiles(dir) {
		t.Error("should return false when no .tmpl files")
	}

	// Add template file
	if err := os.WriteFile(filepath.Join(dir, "config.tmpl"), []byte("x"), 0600); err != nil {
		t.Fatal(err)
	}
	if !hasTemplateFiles(dir) {
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
	if !hasTemplateFiles(dir) {
		t.Error("should detect .tmpl files in subdirectories")
	}
}

func TestHasTemplateFiles_NonExistent(t *testing.T) {
	if hasTemplateFiles("/nonexistent/path") {
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
	if !pathExists(renderedPath) {
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

		if mgr.HasOutdatedTemplates(dir) {
			t.Error("should return false when stateStore is nil")
		}
	})

	t.Run("NoTemplateFiles", func(t *testing.T) {
		_, _, mgr, _ := setupTemplateTest(t)

		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "regular.txt"), []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}

		if mgr.HasOutdatedTemplates(dir) {
			t.Error("should return false when no .tmpl files exist")
		}
	})

	t.Run("NoRenderRecord", func(t *testing.T) {
		_, _, mgr, _ := setupTemplateTest(t)

		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "file.tmpl"), []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}

		if !mgr.HasOutdatedTemplates(dir) {
			t.Error("should return true when template has never been rendered")
		}
	})

	t.Run("HashMatches", func(t *testing.T) {
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

		if mgr.HasOutdatedTemplates(backupDir) {
			t.Error("should return false when template hash matches stored hash")
		}
	})

	t.Run("HashDiffers", func(t *testing.T) {
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

		if !mgr.HasOutdatedTemplates(backupDir) {
			t.Error("should return true when template hash differs from stored hash")
		}
	})

	t.Run("MultipleTemplates_OneOutdated", func(t *testing.T) {
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

		if !mgr.HasOutdatedTemplates(backupDir) {
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

		if mgr.HasModifiedRenderedFiles(dir) {
			t.Error("should return false when stateStore is nil")
		}
	})

	t.Run("NoTemplateFiles", func(t *testing.T) {
		_, _, mgr, _ := setupTemplateTest(t)

		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "regular.txt"), []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}

		if mgr.HasModifiedRenderedFiles(dir) {
			t.Error("should return false when no .tmpl files exist")
		}
	})

	t.Run("NoRenderRecord", func(t *testing.T) {
		_, _, mgr, _ := setupTemplateTest(t)

		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "file.tmpl"), []byte("content"), 0600); err != nil {
			t.Fatal(err)
		}

		if mgr.HasModifiedRenderedFiles(dir) {
			t.Error("should return false when no render record exists")
		}
	})

	t.Run("NoRenderedFile", func(t *testing.T) {
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

		if mgr.HasModifiedRenderedFiles(backupDir) {
			t.Error("should return false when rendered file doesn't exist")
		}
	})

	t.Run("ContentMatchesBaseline", func(t *testing.T) {
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

		if mgr.HasModifiedRenderedFiles(backupDir) {
			t.Error("should return false when rendered file matches pure render baseline")
		}
	})

	t.Run("ContentDiffersFromBaseline", func(t *testing.T) {
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

		if !mgr.HasModifiedRenderedFiles(backupDir) {
			t.Error("should return true when rendered file differs from pure render baseline")
		}
	})
}
