package tui

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/manager"
	"github.com/AntoineGS/tidydots/internal/platform"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

func TestDiffActionUsesCurrentSelectionAndExcludesCopyEntries(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("diff action fixture requires symlinks")
	}

	root := t.TempDir()
	backupPath := filepath.Join(root, "backup")
	targetPath := filepath.Join(root, "target")
	if err := os.MkdirAll(backupPath, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetPath, 0o750); err != nil {
		t.Fatal(err)
	}

	selectedFiles := []string{"selected-one.tmpl", "selected-two.tmpl"}
	allFiles := append(append([]string{}, selectedFiles...), "neighbor.tmpl", "copy.tmpl")
	for _, file := range allFiles {
		writeDiffActionFile(t, filepath.Join(backupPath, file), file+" source")
	}

	plat := &platform.Platform{OS: platform.OSLinux, Hostname: "diff-test", EnvVars: map[string]string{}}
	cfg := &config.Config{Version: 3, BackupRoot: root}
	mgr := manager.New(cfg, plat)
	if err := mgr.InitStateStore(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mgr.Close() }) //nolint:errcheck // test cleanup

	seedEntry := config.SubEntry{Backup: backupPath, Files: allFiles}
	if err := mgr.RestoreFiles(seedEntry, backupPath, targetPath); err != nil {
		t.Fatalf("seed template state: %v", err)
	}
	for _, file := range allFiles {
		writeDiffActionFile(t, tmpl.RenderedPath(filepath.Join(backupPath, file)), file+" user edit")
	}

	selectedEntry := config.SubEntry{
		Name:    "selected",
		Backup:  backupPath,
		Files:   selectedFiles,
		Targets: map[string]string{"linux": targetPath},
	}
	copyEntry := config.SubEntry{
		Name:    "copy",
		Backup:  backupPath,
		Files:   []string{"copy.tmpl"},
		Method:  config.MethodCopy,
		Targets: map[string]string{"linux": targetPath},
	}
	cfg.Applications = []config.Application{{Name: "tool", Entries: []config.SubEntry{selectedEntry, copyEntry}}}

	m := NewModel(cfg, plat, false)
	m.Manager = mgr
	m.pendingStateChecks = 0
	m.Applications[0].Expanded = true
	for i := range m.Applications[0].SubItems {
		m.Applications[0].SubItems[i].State = StateModified
	}
	m.rebuildTable()

	selectedRow := findDiffActionRow(t, &m, "selected")
	m.tableCursor = selectedRow
	updated, cmd := m.Update(tea.KeyPressMsg{Code: 'i'})
	selectedModel := updated.(Model)
	if cmd != nil {
		t.Fatal("selected multi-file diff unexpectedly launched an editor")
	}
	if !selectedModel.showingDiffPicker {
		t.Fatal("selected multi-file diff did not open the diff picker")
	}
	if len(selectedModel.diffPickerFiles) != len(selectedFiles) {
		t.Fatalf("selected diff files = %d, want %d", len(selectedModel.diffPickerFiles), len(selectedFiles))
	}
	for i, file := range selectedFiles {
		want := filepath.Join(backupPath, file)
		if selectedModel.diffPickerFiles[i].TemplatePath != want {
			t.Errorf("selected diff[%d] = %q, want %q", i, selectedModel.diffPickerFiles[i].TemplatePath, want)
		}
	}

	copyRow := findDiffActionRow(t, &selectedModel, "copy")
	selectedModel.tableCursor = copyRow
	selectedModel.showingDiffPicker = false
	selectedModel.diffPickerFiles = nil
	updated, cmd = selectedModel.Update(tea.KeyPressMsg{Code: 'i'})
	copyModel := updated.(Model)
	if cmd != nil || copyModel.showingDiffPicker || len(copyModel.diffPickerFiles) != 0 {
		t.Fatal("copy entry incorrectly launched or opened template diff")
	}
}

func findDiffActionRow(t *testing.T, m *Model, subName string) int {
	t.Helper()
	for i, row := range m.tableRows {
		if row.SubName == subName {
			return i
		}
	}
	t.Fatalf("could not find table row for %q", subName)
	return -1
}

func writeDiffActionFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
