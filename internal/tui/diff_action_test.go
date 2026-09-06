package tui

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/AntoineGS/tidydots/internal/cmdexec"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/fsys"
	"github.com/AntoineGS/tidydots/internal/manager"
	"github.com/AntoineGS/tidydots/internal/platform"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

func TestDiffActionUsesLiveCopyTargetsAndExcludesUnselectedEntries(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("diff action fixture requires symlinks")
	}

	root := t.TempDir()
	backupPath := filepath.Join(root, "backup")
	targetPath := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(backupPath, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetPath, 0o750); err != nil {
		t.Fatal(err)
	}

	selectedFiles := []string{"selected-one.tmpl", "selected-two.tmpl"}
	copyFiles := []string{"copy-one.tmpl", "copy-two.tmpl"}
	allFiles := append(append(append([]string{}, selectedFiles...), copyFiles...), "neighbor.tmpl", "literal.conf")
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

	seedSelectedEntry := config.SubEntry{Backup: backupPath, Files: selectedFiles}
	if err := mgr.RestoreFiles(seedSelectedEntry, backupPath, targetPath); err != nil {
		t.Fatalf("seed symlink template state: %v", err)
	}
	seedCopyEntry := config.SubEntry{Backup: backupPath, Method: config.MethodCopy, Files: copyFiles}
	if err := mgr.RestoreFiles(seedCopyEntry, backupPath, targetPath); err != nil {
		t.Fatalf("seed copy template state: %v", err)
	}
	seedOrdinaryEntry := config.SubEntry{Backup: backupPath, Method: config.MethodCopy, Files: []string{"literal.conf"}}
	if err := mgr.RestoreFiles(seedOrdinaryEntry, backupPath, targetPath); err != nil {
		t.Fatalf("seed ordinary copy state: %v", err)
	}
	for _, file := range selectedFiles {
		writeDiffActionFile(t, tmpl.RenderedPath(filepath.Join(backupPath, file)), file+" user edit")
	}
	for _, file := range copyFiles {
		writeDiffActionFile(t, filepath.Join(targetPath, tmpl.TargetName(file)), file+" user edit")
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
		Files:   copyFiles,
		Method:  config.MethodCopy,
		Targets: map[string]string{"linux": targetPath},
	}
	ordinaryEntry := config.SubEntry{
		Name:    "ordinary-copy",
		Backup:  backupPath,
		Files:   []string{"literal.conf"},
		Method:  config.MethodCopy,
		Targets: map[string]string{"linux": targetPath},
	}
	cfg.Applications = []config.Application{{Name: "tool", Entries: []config.SubEntry{selectedEntry, copyEntry, ordinaryEntry}}}

	m := NewModel(cfg, plat, false)
	m.Manager = mgr
	m.pendingStateChecks = 0
	m.Applications[0].Expanded = true
	for i := range m.Applications[0].SubItems {
		if m.Applications[0].SubItems[i].SubEntry.Name == "ordinary-copy" {
			m.Applications[0].SubItems[i].State = StateLinked
			continue
		}
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
	if cmd != nil || !copyModel.showingDiffPicker {
		t.Fatal("copy entry did not open template diff picker")
	}
	if len(copyModel.diffPickerFiles) != len(copyFiles) {
		t.Fatalf("copy diff files = %d, want %d", len(copyModel.diffPickerFiles), len(copyFiles))
	}
	for i, file := range copyFiles {
		modified := copyModel.diffPickerFiles[i]
		if modified.TemplatePath != filepath.Join(backupPath, file) {
			t.Errorf("copy diff[%d] source = %q, want %q", i, modified.TemplatePath, filepath.Join(backupPath, file))
		}
		if modified.CurrentPath != filepath.Join(targetPath, tmpl.TargetName(file)) {
			t.Errorf("copy diff[%d] current = %q, want live target", i, modified.CurrentPath)
		}
		if string(modified.CurrentOnDisk) != file+" user edit" {
			t.Errorf("copy diff[%d] content = %q, want live target edit", i, modified.CurrentOnDisk)
		}
	}
	diff := generateUnifiedDiff(copyModel.diffPickerFiles[0])
	if !strings.Contains(diff, "+++ edited file ("+filepath.Join(targetPath, tmpl.TargetName(copyFiles[0]))+")") {
		t.Fatalf("copy diff label does not use live target:\n%s", diff)
	}
	if strings.Contains(diff, ".tmpl.rendered") {
		t.Fatalf("copy diff unexpectedly references rendered cache:\n%s", diff)
	}
	copySubIndex := -1
	for i := range copyModel.Applications[0].SubItems {
		if copyModel.Applications[0].SubItems[i].SubEntry.Name == "copy" {
			copySubIndex = i
			break
		}
	}
	if copySubIndex < 0 || copyModel.detectSubEntryState(&copyModel.Applications[0].SubItems[copySubIndex]) != StateModified {
		t.Fatal("copy target was not confirmed Modified before making it inaccessible")
	}

	deniedTarget := filepath.Join(targetPath, tmpl.TargetName(copyFiles[0]))
	stub := cmdexec.NewStubRunner()
	copyModel.Manager = copyModel.Manager.
		WithFS(deniedDiffReadFS{FS: fsys.OsFS{}, denied: deniedTarget}).
		WithRunner(stub)
	copyModel.showingDiffPicker = false
	copyModel.diffPickerFiles = nil
	copyModel.tableCursor = copyRow
	updated, cmd = copyModel.Update(tea.KeyPressMsg{Code: 'i'})
	if cmd == nil {
		t.Fatal("inaccessible copy target did not return an error message command")
	}
	completion := cmd()
	updated, _ = updated.(Model).Update(completion)
	visibleErrorModel := updated.(Model)
	if !visibleErrorModel.showingResults {
		t.Fatal("diff inspection error did not open the results popup")
	}
	if visibleErrorModel.resultsScrollOffset != 0 {
		t.Fatalf("error popup offset = %d, want 0", visibleErrorModel.resultsScrollOffset)
	}
	if len(visibleErrorModel.results) != 1 || visibleErrorModel.results[0].Success {
		t.Fatalf("visible diff error results = %+v, want one failure", visibleErrorModel.results)
	}
	if !strings.Contains(visibleErrorModel.results[0].Message, "permission denied") {
		t.Fatalf("visible diff error = %q, want permission details", visibleErrorModel.results[0].Message)
	}
	if visibleErrorModel.showingDiffPicker {
		t.Fatal("inaccessible copy target launched an editor or opened a picker")
	}
	if len(stub.Calls) != 0 {
		t.Fatalf("diff inspection spawned privileged commands: %+v", stub.Calls)
	}

	ordinaryRow := findDiffActionRow(t, &copyModel, "ordinary-copy")
	copyModel.tableCursor = ordinaryRow
	copyModel.showingDiffPicker = false
	copyModel.diffPickerFiles = nil
	updated, cmd = copyModel.Update(tea.KeyPressMsg{Code: 'i'})
	ordinaryModel := updated.(Model)
	if cmd != nil || ordinaryModel.showingDiffPicker || len(ordinaryModel.diffPickerFiles) != 0 {
		t.Fatal("ordinary copy entry incorrectly launched or opened template diff")
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

type deniedDiffReadFS struct {
	fsys.FS
	denied string
}

func (f deniedDiffReadFS) ReadFile(name string) ([]byte, error) {
	if name == f.denied {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrPermission}
	}
	return f.FS.ReadFile(name)
}
