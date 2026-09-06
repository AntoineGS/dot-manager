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
	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

func TestTemplateStateKeysSeparateEntries(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, _, mgr, _ := setupTemplateTest(t)

	type entry struct {
		name    string
		content string
	}
	entries := []entry{
		{name: "Both", content: "setting=both\n"},
		{name: "Git", content: "setting=git\n"},
	}

	for _, entry := range entries {
		t.Run(entry.name, func(t *testing.T) {
			backupDir := filepath.Join(backupRoot, entry.name)
			if err := os.MkdirAll(backupDir, 0750); err != nil {
				t.Fatal(err)
			}

			tmplPath := filepath.Join(backupDir, "config.tmpl")
			if err := os.WriteFile(tmplPath, []byte(entry.content), 0600); err != nil {
				t.Fatal(err)
			}

			subEntry := config.SubEntry{
				Name:    entry.name,
				Backup:  "./" + entry.name,
				Targets: map[string]string{"linux": t.TempDir()},
			}
			if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, subEntry.Targets["linux"]); err != nil {
				t.Fatalf("RestoreFolderWithTemplates failed: %v", err)
			}
		})
	}

	for _, entry := range entries {
		backupDir := filepath.Join(backupRoot, entry.name)
		if mgr.HasOutdatedTemplates(backupDir, nil) {
			t.Errorf("%s template should not be outdated after its initial render", entry.name)
		}
		if mgr.HasModifiedRenderedFiles(backupDir, nil) {
			t.Errorf("%s rendered template should not be marked modified after its initial render", entry.name)
		}
	}

	bothTemplate := filepath.Join(backupRoot, "Both", "config.tmpl")
	if err := os.WriteFile(bothTemplate, []byte("setting=both-updated\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if !mgr.HasOutdatedTemplates(filepath.Join(backupRoot, "Both"), nil) {
		t.Error("editing Both/config.tmpl should mark the Both entry outdated")
	}
	if mgr.HasOutdatedTemplates(filepath.Join(backupRoot, "Git"), nil) {
		t.Error("editing Both/config.tmpl should not mark the Git entry outdated")
	}
}

func TestTemplateStateKeysUseEntryBaselineForMerge(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, _, mgr, _ := setupTemplateTest(t)

	bothDir := filepath.Join(backupRoot, "Both")
	gitDir := filepath.Join(backupRoot, "Git")
	for _, dir := range []string{bothDir, gitDir} {
		if err := os.MkdirAll(dir, 0750); err != nil {
			t.Fatal(err)
		}
	}

	bothTemplate := filepath.Join(bothDir, "config.tmpl")
	gitTemplate := filepath.Join(gitDir, "config.tmpl")
	if err := os.WriteFile(bothTemplate, []byte("setting=both-v1\nshared=old\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(gitTemplate, []byte("setting=git-v1\nshared=old\n"), 0600); err != nil {
		t.Fatal(err)
	}

	bothTarget := t.TempDir()
	gitTarget := t.TempDir()
	bothEntry := config.SubEntry{Name: "Both", Backup: "./Both", Targets: map[string]string{"linux": bothTarget}}
	gitEntry := config.SubEntry{Name: "Git", Backup: "./Git", Targets: map[string]string{"linux": gitTarget}}
	if err := mgr.RestoreFolderWithTemplates(bothEntry, bothDir, bothTarget); err != nil {
		t.Fatalf("initial Both restore: %v", err)
	}
	if err := mgr.RestoreFolderWithTemplates(gitEntry, gitDir, gitTarget); err != nil {
		t.Fatalf("initial Git restore: %v", err)
	}

	renderedPath := filepath.Join(bothDir, "config.tmpl.rendered")
	if err := os.WriteFile(renderedPath, []byte("setting=both-user\nshared=old\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(bothTemplate, []byte("setting=both-v1\nshared=new\n"), 0600); err != nil {
		t.Fatal(err)
	}

	if err := mgr.RestoreFolderWithTemplates(bothEntry, bothDir, bothTarget); err != nil {
		t.Fatalf("re-render Both: %v", err)
	}

	merged, err := os.ReadFile(renderedPath)
	if err != nil {
		t.Fatal(err)
	}
	want := "setting=both-user\nshared=new\n"
	if string(merged) != want {
		t.Errorf("merged Both content = %q, want %q", string(merged), want)
	}
	if testPathExists(tmpl.ConflictPath(bothTemplate)) {
		t.Error("using the Both baseline should not create a merge conflict")
	}
}

func TestTemplateStateKeysBackupOrphanWithoutHistory(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, targetDir, mgr, store := setupTemplateTest(t)

	backupDir := filepath.Join(backupRoot, "Both")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

	templateContent := []byte("fresh=rendered\n")
	tmplPath := filepath.Join(backupDir, "config.tmpl")
	renderedPath := filepath.Join(backupDir, "config.tmpl.rendered")
	oldContent := []byte("old=orphaned\n")
	if err := os.WriteFile(tmplPath, templateContent, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(renderedPath, oldContent, 0600); err != nil {
		t.Fatal(err)
	}

	subEntry := config.SubEntry{
		Name:    "Both",
		Backup:  "./Both",
		Targets: map[string]string{"linux": targetDir},
	}
	if err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir); err != nil {
		t.Fatalf("RestoreFolderWithTemplates failed: %v", err)
	}

	backupContent, err := os.ReadFile(renderedPath + ".bak")
	if err != nil {
		t.Fatalf("read orphan backup: %v", err)
	}
	if string(backupContent) != string(oldContent) {
		t.Errorf("orphan backup = %q, want %q", string(backupContent), string(oldContent))
	}

	renderedContent, err := os.ReadFile(renderedPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(renderedContent) != string(templateContent) {
		t.Errorf("rendered content = %q, want fresh render %q", string(renderedContent), string(templateContent))
	}

	newKey := "./Both/config.tmpl"
	record, err := store.GetLatestRender(context.Background(), newKey, "linux", "testhost")
	if err != nil {
		t.Fatalf("query new state key: %v", err)
	}
	if record == nil {
		t.Fatalf("expected render record for new state key %q", newKey)
	}
	if string(record.PureRender) != string(templateContent) {
		t.Errorf("pure render = %q, want %q", record.PureRender, templateContent)
	}
}

func TestTemplateRestore_RefusesExistingOrphanBackup(t *testing.T) {
	skipIfNoSymlink(t)

	tests := []struct {
		name        string
		seedBackup  func(t *testing.T, path string)
		checkBackup func(t *testing.T, path string)
	}{
		{
			name: "regular backup",
			seedBackup: func(t *testing.T, path string) {
				t.Helper()
				if err := os.WriteFile(path, []byte("original backup\n"), 0600); err != nil {
					t.Fatal(err)
				}
			},
			checkBackup: func(t *testing.T, path string) {
				t.Helper()
				content, err := os.ReadFile(path)
				if err != nil {
					t.Fatal(err)
				}
				if string(content) != "original backup\n" {
					t.Errorf("backup content = %q, want original backup", string(content))
				}
			},
		},
		{
			name: "dangling symlink",
			seedBackup: func(t *testing.T, path string) {
				t.Helper()
				if err := os.Symlink("missing-rendered-file", path); err != nil {
					t.Fatal(err)
				}
			},
			checkBackup: func(t *testing.T, path string) {
				t.Helper()
				link, err := os.Readlink(path)
				if err != nil {
					t.Fatal(err)
				}
				if link != "missing-rendered-file" {
					t.Errorf("backup symlink target = %q, want %q", link, "missing-rendered-file")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backupRoot, targetDir, mgr, store := setupTemplateTest(t)
			backupDir := filepath.Join(backupRoot, "Both")
			if err := os.MkdirAll(backupDir, 0750); err != nil {
				t.Fatal(err)
			}

			templateContent := []byte("fresh=rendered\n")
			tmplPath := filepath.Join(backupDir, "config.tmpl")
			renderedPath := filepath.Join(backupDir, "config.tmpl.rendered")
			bakPath := renderedPath + ".bak"
			if err := os.WriteFile(tmplPath, templateContent, 0600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(renderedPath, []byte("orphaned content\n"), 0600); err != nil {
				t.Fatal(err)
			}
			tt.seedBackup(t, bakPath)

			subEntry := config.SubEntry{
				Name:    "Both",
				Backup:  "./Both",
				Targets: map[string]string{"linux": targetDir},
			}
			err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir)
			if err == nil {
				t.Fatal("expected restore to refuse an existing orphan backup")
			}
			if !strings.Contains(err.Error(), bakPath) || !errors.Is(err, fs.ErrExist) {
				t.Errorf("error = %q, want actionable existing orphan backup error", err)
			}

			rendered, readErr := os.ReadFile(renderedPath)
			if readErr != nil {
				t.Fatal(readErr)
			}
			if string(rendered) != "orphaned content\n" {
				t.Errorf("rendered content = %q, want original orphan content", string(rendered))
			}
			tt.checkBackup(t, bakPath)

			key := "./Both/config.tmpl"
			record, queryErr := store.GetLatestRender(context.Background(), key, "linux", "testhost")
			if queryErr != nil {
				t.Fatalf("query state record: %v", queryErr)
			}
			if record != nil {
				t.Error("failed orphan backup must not create a state record")
			}
		})
	}
}

type orphanBackupFailFS struct {
	fsys.OsFS
	failPath string
	err      error
}

func (f *orphanBackupFailFS) WriteFileExclusive(name string, data []byte, perm fs.FileMode) error {
	if filepath.Clean(name) == filepath.Clean(f.failPath) {
		return f.err
	}
	return f.OsFS.WriteFileExclusive(name, data, perm)
}

func TestTemplateRestore_OrphanBackupCopyFailureFailsClosed(t *testing.T) {
	skipIfNoSymlink(t)
	backupRoot, targetDir, mgr, store := setupTemplateTest(t)
	backupDir := filepath.Join(backupRoot, "Both")
	if err := os.MkdirAll(backupDir, 0750); err != nil {
		t.Fatal(err)
	}

	templateContent := []byte("fresh=rendered\n")
	tmplPath := filepath.Join(backupDir, "config.tmpl")
	renderedPath := filepath.Join(backupDir, "config.tmpl.rendered")
	bakPath := renderedPath + ".bak"
	if err := os.WriteFile(tmplPath, templateContent, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(renderedPath, []byte("orphaned content\n"), 0600); err != nil {
		t.Fatal(err)
	}

	injectedErr := errors.New("injected orphan backup failure")
	mgr = mgr.WithFS(&orphanBackupFailFS{
		failPath: bakPath,
		err:      injectedErr,
	})
	subEntry := config.SubEntry{
		Name:    "Both",
		Backup:  "./Both",
		Targets: map[string]string{"linux": targetDir},
	}
	err := mgr.RestoreFolderWithTemplates(subEntry, backupDir, targetDir)
	if err == nil {
		t.Fatal("expected restore to fail when orphan backup cannot be copied")
	}
	if !strings.Contains(err.Error(), injectedErr.Error()) {
		t.Errorf("error = %q, want injected backup failure", err)
	}

	rendered, readErr := os.ReadFile(renderedPath)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(rendered) != "orphaned content\n" {
		t.Errorf("rendered content = %q, want original orphan content", string(rendered))
	}
	if _, statErr := os.Lstat(bakPath); !errors.Is(statErr, fs.ErrNotExist) {
		t.Errorf("orphan backup stat error = %v, want not exist", statErr)
	}

	key := "./Both/config.tmpl"
	record, queryErr := store.GetLatestRender(context.Background(), key, "linux", "testhost")
	if queryErr != nil {
		t.Fatalf("query state record: %v", queryErr)
	}
	if record != nil {
		t.Error("failed orphan backup must not create a state record")
	}
}
