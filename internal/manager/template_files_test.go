package manager

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/platform"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

func TestRestoreFiles_SelectedTemplate(t *testing.T) {
	skipIfNoSymlink(t)
	backup, target, mgr, _ := setupTemplateTest(t)
	if err := os.WriteFile(filepath.Join(backup, ".gitconfig.tmpl"), []byte("Host={{ .Hostname }}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(backup, "unlisted.tmpl"), []byte("{{ broken }}"), 0600); err != nil {
		t.Fatal(err)
	}
	entry := config.SubEntry{Name: "git", Backup: backup, Files: []string{".gitconfig.tmpl"}}
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(target, ".gitconfig"))
	if err != nil || string(got) != "Host=testhost" {
		t.Fatalf("deployed content = %q, error = %v", got, err)
	}
	for _, path := range []string{
		filepath.Join(target, ".gitconfig.tmpl"),
		filepath.Join(backup, "unlisted.tmpl.rendered"),
	} {
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			t.Fatalf("unexpected artifact %s: %v", path, err)
		}
	}
}

func TestRestoreFiles_SelectedTemplate_NestedTarget(t *testing.T) {
	skipIfNoSymlink(t)
	backup, target, mgr, _ := setupTemplateTest(t)
	templatePath := filepath.Join(backup, "nested", "gitconfig.tmpl")
	writeTemplateFile(t, templatePath, "Host={{ .Hostname }}")

	entry := config.SubEntry{Name: "git", Backup: backup, Files: []string{"nested/gitconfig.tmpl"}}
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("RestoreFiles: %v", err)
	}

	got := readTemplateTestFile(t, filepath.Join(target, "nested", "gitconfig"))
	if got != expectedHostnameRender {
		t.Fatalf("nested deployed content = %q, want %q", got, expectedHostnameRender)
	}
	if !testIsSymlink(filepath.Join(target, "nested", "gitconfig")) {
		t.Fatal("nested target should be a symlink")
	}
}

func TestRestoreFiles_SelectedTemplate_MixedFilesKeepLiteralLinks(t *testing.T) {
	skipIfNoSymlink(t)
	backup, target, mgr, _ := setupTemplateTest(t)
	writeTemplateFile(t, filepath.Join(backup, "literal.conf"), "literal bytes")
	writeTemplateFile(t, filepath.Join(backup, "rendered.conf.tmpl"), "Host={{ .Hostname }}")

	entry := config.SubEntry{
		Name:   "mixed",
		Backup: backup,
		Files:  []string{"literal.conf", "rendered.conf.tmpl"},
	}
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("RestoreFiles: %v", err)
	}

	if got := readTemplateTestFile(t, filepath.Join(target, "literal.conf")); got != "literal bytes" {
		t.Errorf("literal content = %q, want literal bytes", got)
	}
	if got := readTemplateTestFile(t, filepath.Join(target, "rendered.conf")); got != expectedHostnameRender {
		t.Errorf("rendered content = %q, want %q", got, expectedHostnameRender)
	}
	if !testIsSymlink(filepath.Join(target, "literal.conf")) {
		t.Error("literal file should remain an ordinary symlink")
	}
}

func TestRestoreFiles_SelectedTemplate_InvalidSourcesPreserveTargets(t *testing.T) {
	tests := []struct {
		name          string
		file          string
		source        string
		writeSource   bool
		wantSource    string
		wantErrorText string
	}{
		{
			name:          "missing source",
			file:          "missing.tmpl",
			wantErrorText: "template source does not exist",
		},
		{
			name:          "invalid source",
			file:          "invalid.tmpl",
			source:        "{{ broken }}",
			writeSource:   true,
			wantSource:    "{{ broken }}",
			wantErrorText: "rendering template",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skipIfNoSymlink(t)
			backup, target, mgr, _ := setupTemplateTest(t)
			targetPath := filepath.Join(target, tmpl.TargetName(tt.file))
			writeTemplateFile(t, targetPath, "keep target bytes")
			if tt.writeSource {
				writeTemplateFile(t, filepath.Join(backup, tt.file), tt.source)
			}

			entry := config.SubEntry{Name: tt.name, Backup: backup, Files: []string{tt.file}}
			err := mgr.RestoreFiles(entry, backup, target)
			if err == nil || !strings.Contains(err.Error(), tt.wantErrorText) {
				t.Fatalf("RestoreFiles error = %v, want text %q", err, tt.wantErrorText)
			}

			if got := readTemplateTestFile(t, targetPath); got != "keep target bytes" {
				t.Fatalf("target bytes = %q, want keep target bytes", got)
			}
			if tt.wantSource != "" {
				if got := readTemplateTestFile(t, filepath.Join(backup, tt.file)); got != tt.wantSource {
					t.Fatalf("template source = %q, want %q", got, tt.wantSource)
				}
			}
			if testPathExists(tmpl.RenderedPath(filepath.Join(backup, tt.file))) {
				t.Fatal("invalid source created a rendered artifact")
			}
		})
	}
}

func TestRestoreFiles_SelectedTemplate_NormalConflictPreservesTarget(t *testing.T) {
	skipIfNoSymlink(t)
	backup, target, mgr, _ := setupTemplateTest(t)
	templatePath := filepath.Join(backup, "config.tmpl")
	writeTemplateFile(t, templatePath, "new content")
	writeTemplateFile(t, filepath.Join(target, "config"), "old target content")

	entry := config.SubEntry{Name: "config", Backup: backup, Files: []string{"config.tmpl"}}
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("RestoreFiles: %v", err)
	}

	if got := readTemplateTestFile(t, filepath.Join(target, "config")); got != "new content" {
		t.Fatalf("deployed content = %q, want new content", got)
	}
	conflicts, err := filepath.Glob(filepath.Join(backup, "config_target_*"))
	if err != nil {
		t.Fatal(err)
	}
	if len(conflicts) != 1 {
		t.Fatalf("conflict backups = %v, want one backup", conflicts)
	}
	if got := readTemplateTestFile(t, conflicts[0]); got != "old target content" {
		t.Errorf("conflict backup = %q, want old target content", got)
	}
	if got := readTemplateTestFile(t, templatePath); got != "new content" {
		t.Errorf("template source = %q, want new content", got)
	}
}

func TestRestoreFiles_SelectedTemplate_NoMergePreservesTarget(t *testing.T) {
	skipIfNoSymlink(t)
	backup, target, mgr, _ := setupTemplateTest(t)
	templatePath := filepath.Join(backup, "config.tmpl")
	writeTemplateFile(t, templatePath, "new content")
	targetPath := filepath.Join(target, "config")
	writeTemplateFile(t, targetPath, "old target content")
	mgr.NoMerge = true

	entry := config.SubEntry{Name: "config", Backup: backup, Files: []string{"config.tmpl"}}
	if err := mgr.RestoreFiles(entry, backup, target); err == nil {
		t.Fatal("RestoreFiles succeeded with NoMerge and an existing target")
	}

	if got := readTemplateTestFile(t, targetPath); got != "old target content" {
		t.Fatalf("target bytes = %q, want old target content", got)
	}
	if testPathExists(tmpl.RenderedPath(templatePath)) {
		t.Fatal("NoMerge refusal rendered a template")
	}
	if got := readTemplateTestFile(t, templatePath); got != "new content" {
		t.Errorf("template source = %q, want new content", got)
	}
}

func TestRestoreFiles_SelectedTemplate_ForceReplacesTarget(t *testing.T) {
	skipIfNoSymlink(t)
	backup, target, mgr, _ := setupTemplateTest(t)
	writeTemplateFile(t, filepath.Join(backup, "config.tmpl"), "new content")
	writeTemplateFile(t, filepath.Join(target, "config"), "old target content")
	mgr.NoMerge = true
	mgr.ForceDelete = true

	entry := config.SubEntry{Name: "config", Backup: backup, Files: []string{"config.tmpl"}}
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("RestoreFiles: %v", err)
	}

	if got := readTemplateTestFile(t, filepath.Join(target, "config")); got != "new content" {
		t.Fatalf("deployed content = %q, want new content", got)
	}
	if !testIsSymlink(filepath.Join(target, "config")) {
		t.Fatal("force replacement should create a symlink")
	}
}

func TestRestoreFiles_SelectedTemplate_DryRunPreservesFilesystem(t *testing.T) {
	skipIfNoSymlink(t)
	backup, target, mgr, store := setupTemplateTest(t)
	writeTemplateFile(t, filepath.Join(backup, "nested", "config.tmpl"), "Host={{ .Hostname }}")
	writeTemplateFile(t, filepath.Join(target, "nested", "config"), "keep target bytes")
	mgr.DryRun = true

	beforeBackup := snapshotTemplateFilesystem(t, backup)
	beforeTarget := snapshotTemplateFilesystem(t, target)
	entry := config.SubEntry{Name: "config", Backup: backup, Files: []string{"nested/config.tmpl"}}
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("RestoreFiles dry-run: %v", err)
	}

	if after := snapshotTemplateFilesystem(t, backup); !reflect.DeepEqual(after, beforeBackup) {
		t.Fatalf("backup changed during dry-run:\nbefore=%v\nafter=%v", beforeBackup, after)
	}
	if after := snapshotTemplateFilesystem(t, target); !reflect.DeepEqual(after, beforeTarget) {
		t.Fatalf("target changed during dry-run:\nbefore=%v\nafter=%v", beforeTarget, after)
	}
	if record, err := store.GetLatestRender(mgr.ctx, "nested/config.tmpl", "linux", "testhost"); err != nil || record != nil {
		t.Fatalf("dry-run render state = (%v, %v), want no record", record, err)
	}
}

func TestRestoreFiles_SelectedTemplate_RejectsAliasesAndAmbiguousSelections(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, backup, target string, mgr *Manager) config.SubEntry
	}{
		{
			name: "target directory aliases backup",
			setup: func(t *testing.T, backup, target string, _ *Manager) config.SubEntry {
				writeTemplateFile(t, filepath.Join(backup, "config.tmpl"), "source")
				if err := os.Remove(target); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(backup, target); err != nil {
					t.Fatal(err)
				}
				return config.SubEntry{Name: "config", Backup: backup, Files: []string{"config.tmpl"}}
			},
		},
		{
			name: "generated artifact aliases target file",
			setup: func(t *testing.T, backup, target string, mgr *Manager) config.SubEntry {
				templatePath := filepath.Join(backup, "config.tmpl")
				writeTemplateFile(t, templatePath, "source")
				if err := mgr.renderTemplateAndLink(templatePath, "config.tmpl"); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(tmpl.RenderedPath(templatePath), filepath.Join(target, "config")); err != nil {
					t.Fatal(err)
				}
				return config.SubEntry{Name: "config", Backup: backup, Files: []string{"config.tmpl"}}
			},
		},
		{
			name: "suffix-free output is selected literally too",
			setup: func(t *testing.T, backup, _ string, _ *Manager) config.SubEntry {
				writeTemplateFile(t, filepath.Join(backup, "config.tmpl"), "template source")
				writeTemplateFile(t, filepath.Join(backup, "config"), "literal source")
				return config.SubEntry{Name: "config", Backup: backup, Files: []string{"config.tmpl", "config"}}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skipIfNoSymlink(t)
			backup, target, mgr, _ := setupTemplateTest(t)
			entry := tt.setup(t, backup, target, mgr)
			templatePath := filepath.Join(backup, "config.tmpl")
			beforeSource := readTemplateTestFile(t, templatePath)
			beforeBackup := snapshotTemplateFilesystem(t, backup)
			beforeTarget := snapshotTemplateFilesystem(t, target)

			if err := mgr.RestoreFiles(entry, backup, target); err == nil {
				t.Fatal("RestoreFiles unexpectedly succeeded for an unsafe selection")
			}

			if got := readTemplateTestFile(t, templatePath); got != beforeSource {
				t.Fatalf("template source changed from %q to %q", beforeSource, got)
			}
			if after := snapshotTemplateFilesystem(t, backup); !reflect.DeepEqual(after, beforeBackup) {
				t.Fatalf("backup changed during unsafe restore:\nbefore=%v\nafter=%v", beforeBackup, after)
			}
			if after := snapshotTemplateFilesystem(t, target); !reflect.DeepEqual(after, beforeTarget) {
				t.Fatalf("target changed during unsafe restore:\nbefore=%v\nafter=%v", beforeTarget, after)
			}
		})
	}
}

func TestBackupFilesSubEntry_SkipsTemplateSources(t *testing.T) {
	tests := []struct {
		name       string
		makeTarget func(t *testing.T, targetDir string) string
	}{
		{
			name: "real target",
			makeTarget: func(t *testing.T, targetDir string) string {
				path := filepath.Join(targetDir, "config.tmpl")
				writeTemplateFile(t, path, "installed template")
				return path
			},
		},
		{
			name: "symlink target",
			makeTarget: func(t *testing.T, targetDir string) string {
				external := filepath.Join(t.TempDir(), "external.tmpl")
				writeTemplateFile(t, external, "installed template")
				path := filepath.Join(targetDir, "config.tmpl")
				if err := os.Symlink(external, path); err != nil {
					t.Fatal(err)
				}
				return path
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skipIfNoSymlink(t)
			tmpDir := t.TempDir()
			targetDir := filepath.Join(tmpDir, "target")
			backupDir := filepath.Join(tmpDir, "backup")
			if err := os.MkdirAll(targetDir, 0750); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(backupDir, 0750); err != nil {
				t.Fatal(err)
			}
			templateSource := filepath.Join(backupDir, "config.tmpl")
			writeTemplateFile(t, templateSource, "template source")
			targetPath := tt.makeTarget(t, targetDir)
			mgr := New(&config.Config{BackupRoot: tmpDir}, setupTemplatePlatform())
			entry := config.SubEntry{Name: "config", Backup: backupDir, Files: []string{"config.tmpl"}}

			if err := mgr.backupFilesSubEntry("config", entry, backupDir, targetDir); err != nil {
				t.Fatalf("backupFilesSubEntry: %v", err)
			}
			if got := readTemplateTestFile(t, templateSource); got != "template source" {
				t.Fatalf("template source changed to %q", got)
			}
			if tt.name == "symlink target" {
				if !testIsSymlink(targetPath) {
					t.Fatal("template symlink target was changed")
				}
			}
		})
	}
}

func TestRestoreFiles_CopyTemplateRemainsLiteral(t *testing.T) {
	backup, target, mgr, _ := setupTemplateTest(t)
	literal := "Host={{ .Hostname }}"
	writeTemplateFile(t, filepath.Join(backup, "config.tmpl"), literal)
	entry := config.SubEntry{
		Name:   "config",
		Backup: backup,
		Files:  []string{"config.tmpl"},
		Method: config.MethodCopy,
	}

	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("RestoreFiles copy mode: %v", err)
	}
	path := filepath.Join(target, "config.tmpl")
	if got := readTemplateTestFile(t, path); got != literal {
		t.Fatalf("copy target = %q, want literal template bytes", got)
	}
	if testIsSymlink(path) {
		t.Fatal("copy mode created a symlink")
	}
}

func TestRestoreFiles_SelectedTemplate_SingleSuffixStrippingIsNotRecursive(t *testing.T) {
	skipIfNoSymlink(t)
	backup, target, mgr, _ := setupTemplateTest(t)
	writeTemplateFile(t, filepath.Join(backup, "config.tmpl.tmpl"), "literal-looking template")
	entry := config.SubEntry{Name: "config", Backup: backup, Files: []string{"config.tmpl.tmpl"}}

	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("RestoreFiles: %v", err)
	}

	if got := readTemplateTestFile(t, filepath.Join(target, "config.tmpl")); got != "literal-looking template" {
		t.Fatalf("suffix-stripped target = %q, want literal-looking template", got)
	}
	if testPathExists(filepath.Join(target, "config.tmpl.tmpl")) {
		t.Fatal("recursive template dispatch created the unsuffixed source name")
	}
}

func TestRestoreFiles_SelectedTemplate_ReRenderPreservesEdits(t *testing.T) {
	skipIfNoSymlink(t)
	backup, target, mgr, store := setupTemplateTest(t)
	templatePath := filepath.Join(backup, "config.tmpl")
	renderedPath := tmpl.RenderedPath(templatePath)
	writeTemplateFile(t, templatePath, "editor=vim\npath=old\n")
	entry := config.SubEntry{Name: "config", Backup: backup, Files: []string{"config.tmpl"}}

	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("first RestoreFiles: %v", err)
	}
	writeTemplateFile(t, renderedPath, "editor=nvim\npath=old\n")
	writeTemplateFile(t, templatePath, "editor=vim\npath=new\n")
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("second RestoreFiles: %v", err)
	}

	got := readTemplateTestFile(t, renderedPath)
	if !strings.Contains(got, "editor=nvim") || !strings.Contains(got, "path=new") {
		t.Fatalf("merged render = %q, want user and template edits", got)
	}
	record, err := store.GetLatestRender(mgr.ctx, "config.tmpl", "linux", "testhost")
	if err != nil || record == nil {
		t.Fatalf("render state = (%v, %v), want record", record, err)
	}
	if strings.Contains(string(record.PureRender), "editor=nvim") {
		t.Fatal("render state stored user edits instead of pure render")
	}
}

func TestRestoreFiles_SelectedTemplate_ConflictAndForce(t *testing.T) {
	t.Run("conflict", func(t *testing.T) {
		skipIfNoSymlink(t)
		backup, target, mgr, _ := setupTemplateTest(t)
		templatePath := filepath.Join(backup, "config.tmpl")
		renderedPath := tmpl.RenderedPath(templatePath)
		writeTemplateFile(t, templatePath, "line1\nline2\n")
		entry := config.SubEntry{Name: "config", Backup: backup, Files: []string{"config.tmpl"}}
		if err := mgr.RestoreFiles(entry, backup, target); err != nil {
			t.Fatal(err)
		}
		writeTemplateFile(t, renderedPath, "line1\nuser\n")
		writeTemplateFile(t, templatePath, "line1\ntemplate\n")
		if err := mgr.RestoreFiles(entry, backup, target); err != nil {
			t.Fatal(err)
		}

		conflictPath := tmpl.ConflictPath(templatePath)
		if !testPathExists(conflictPath) {
			t.Fatal("expected conflict artifact")
		}
		if got := readTemplateTestFile(t, renderedPath); got != "line1\ntemplate\n" {
			t.Fatalf("rendered content = %q, want fresh template output", got)
		}
		if got := readTemplateTestFile(t, conflictPath); !strings.Contains(got, "user") {
			t.Fatalf("conflict content = %q, want user edit", got)
		}
	})

	t.Run("force render", func(t *testing.T) {
		skipIfNoSymlink(t)
		backup, target, mgr, _ := setupTemplateTest(t)
		mgr.ForceRender = true
		templatePath := filepath.Join(backup, "config.tmpl")
		renderedPath := tmpl.RenderedPath(templatePath)
		writeTemplateFile(t, templatePath, "version1")
		entry := config.SubEntry{Name: "config", Backup: backup, Files: []string{"config.tmpl"}}
		if err := mgr.RestoreFiles(entry, backup, target); err != nil {
			t.Fatal(err)
		}
		writeTemplateFile(t, renderedPath, "user edit")
		writeTemplateFile(t, templatePath, "version2")
		if err := mgr.RestoreFiles(entry, backup, target); err != nil {
			t.Fatal(err)
		}
		if got := readTemplateTestFile(t, renderedPath); got != "version2" {
			t.Fatalf("force-render content = %q, want version2", got)
		}
	})
}

func TestRestoreFiles_SelectedTemplate_RegeneratesMissingRenderedOutput(t *testing.T) {
	skipIfNoSymlink(t)
	backup, target, mgr, store := setupTemplateTest(t)
	templatePath := filepath.Join(backup, "config.tmpl")
	renderedPath := tmpl.RenderedPath(templatePath)
	writeTemplateFile(t, templatePath, "Host={{ .Hostname }}")
	entry := config.SubEntry{Name: "config", Backup: backup, Files: []string{"config.tmpl"}}
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(renderedPath); err != nil {
		t.Fatal(err)
	}
	if err := mgr.RestoreFiles(entry, backup, target); err != nil {
		t.Fatalf("restore after removing rendered output: %v", err)
	}
	if got := readTemplateTestFile(t, renderedPath); got != expectedHostnameRender {
		t.Fatalf("regenerated content = %q, want %q", got, expectedHostnameRender)
	}
	record, err := store.GetLatestRender(mgr.ctx, "config.tmpl", "linux", "testhost")
	if err != nil || record == nil {
		t.Fatalf("render state = (%v, %v), want retained history", record, err)
	}
}

func TestRestoreFiles_SelectedTemplate_RejectsGeneratedTempAliases(t *testing.T) {
	for _, linkKind := range []string{"symlink", "hardlink"} {
		t.Run(linkKind, func(t *testing.T) {
			skipIfNoSymlink(t)
			backup, target, mgr, _ := setupTemplateTest(t)
			templatePath := filepath.Join(backup, "config.tmpl")
			original := "Host={{ .Hostname }}"
			writeTemplateFile(t, templatePath, original)
			linkTestPath(t, templatePath, templateTempPath(templatePath), linkKind)

			entry := config.SubEntry{Name: "config", Backup: backup, Files: []string{"config.tmpl"}}
			if err := mgr.RestoreFiles(entry, backup, target); err == nil {
				t.Fatal("RestoreFiles accepted a temporary path aliasing the template source")
			}
			if got := readTemplateTestFile(t, templatePath); got != original {
				t.Fatalf("template source changed from %q to %q", original, got)
			}
			if testPathExists(filepath.Join(target, "config")) {
				t.Fatal("target was changed after rejecting the temporary alias")
			}
		})
	}
}

func TestRestoreFiles_SelectedTemplate_RejectsGeneratedConflictAliases(t *testing.T) {
	for _, linkKind := range []string{"symlink", "hardlink"} {
		t.Run(linkKind, func(t *testing.T) {
			skipIfNoSymlink(t)
			backup, target, mgr, _ := setupTemplateTest(t)
			templatePath := filepath.Join(backup, "config.tmpl")
			renderedPath := tmpl.RenderedPath(templatePath)
			conflictPath := tmpl.ConflictPath(templatePath)
			writeTemplateFile(t, templatePath, "line1\nline2\n")
			entry := config.SubEntry{Name: "config", Backup: backup, Files: []string{"config.tmpl"}}
			if err := mgr.RestoreFiles(entry, backup, target); err != nil {
				t.Fatalf("initial RestoreFiles: %v", err)
			}

			writeTemplateFile(t, renderedPath, "line1\nuser\n")
			writeTemplateFile(t, templatePath, "line1\ntemplate\n")
			linkTestPath(t, templatePath, conflictPath, linkKind)

			if err := mgr.RestoreFiles(entry, backup, target); err == nil {
				t.Fatal("RestoreFiles accepted a conflict path aliasing the template source")
			}
			if got := readTemplateTestFile(t, templatePath); got != "line1\ntemplate\n" {
				t.Fatalf("template source was overwritten through conflict alias: %q", got)
			}
		})
	}
}

func TestRestoreFiles_SelectedTemplate_RejectsGeneratedOrphanAliases(t *testing.T) {
	for _, linkKind := range []string{"symlink", "hardlink"} {
		t.Run(linkKind, func(t *testing.T) {
			skipIfNoSymlink(t)
			backup, target, mgr, _ := setupTemplateTest(t)
			templatePath := filepath.Join(backup, "config.tmpl")
			renderedPath := tmpl.RenderedPath(templatePath)
			writeTemplateFile(t, templatePath, "new render")
			writeTemplateFile(t, renderedPath, "orphaned render")
			linkTestPath(t, templatePath, renderedPath+".bak", linkKind)

			entry := config.SubEntry{Name: "config", Backup: backup, Files: []string{"config.tmpl"}}
			if err := mgr.RestoreFiles(entry, backup, target); err == nil {
				t.Fatal("RestoreFiles accepted an orphan backup path aliasing the template source")
			}
			if got := readTemplateTestFile(t, templatePath); got != "new render" {
				t.Fatalf("template source was overwritten through orphan alias: %q", got)
			}
		})
	}
}

func TestRestoreFiles_SelectedTemplate_RejectsRenderedAliasesOnFastPath(t *testing.T) {
	for _, linkKind := range []string{"symlink", "hardlink"} {
		t.Run(linkKind, func(t *testing.T) {
			skipIfNoSymlink(t)
			backup, target, mgr, _ := setupTemplateTest(t)
			templatePath := filepath.Join(backup, "config.tmpl")
			renderedPath := tmpl.RenderedPath(templatePath)
			writeTemplateFile(t, templatePath, "pure template")
			entry := config.SubEntry{Name: "config", Backup: backup, Files: []string{"config.tmpl"}}
			if err := mgr.RestoreFiles(entry, backup, target); err != nil {
				t.Fatalf("initial RestoreFiles: %v", err)
			}

			if err := os.Remove(renderedPath); err != nil {
				t.Fatal(err)
			}
			linkTestPath(t, templatePath, renderedPath, linkKind)

			if err := mgr.RestoreFiles(entry, backup, target); err == nil {
				t.Fatal("RestoreFiles trusted a rendered artifact aliasing the template source")
			}
			if got := readTemplateTestFile(t, templatePath); got != "pure template" {
				t.Fatalf("template source changed after rendered alias rejection: %q", got)
			}
		})
	}
}

func TestRestoreFiles_SelectedTemplate_RejectsGeneratedPathCollisions(t *testing.T) {
	skipIfNoSymlink(t)
	backup, target, mgr, _ := setupTemplateTest(t)
	firstSource := filepath.Join(backup, "config.tmpl")
	secondSource := filepath.Join(backup, "config.tmpl.rendered.tmpl")
	writeTemplateFile(t, firstSource, "first")
	writeTemplateFile(t, secondSource, "second")
	entry := config.SubEntry{
		Name:   "config",
		Backup: backup,
		Files:  []string{"config.tmpl", "config.tmpl.rendered.tmpl"},
	}

	if err := mgr.RestoreFiles(entry, backup, target); err == nil {
		t.Fatal("RestoreFiles accepted selected templates whose generated paths collide")
	}
	if got := readTemplateTestFile(t, firstSource); got != "first" {
		t.Fatalf("first template source changed to %q", got)
	}
	if got := readTemplateTestFile(t, secondSource); got != "second" {
		t.Fatalf("second template source changed to %q", got)
	}
	for _, path := range []string{
		filepath.Join(backup, "config.tmpl.rendered"),
		filepath.Join(backup, "config"),
		filepath.Join(target, "config"),
		filepath.Join(target, "config.tmpl.rendered"),
	} {
		if testPathExists(path) {
			t.Fatalf("generated collision created %s", path)
		}
	}
}

func TestRestoreFiles_SelectedTemplate_RejectsEscapingSelection(t *testing.T) {
	skipIfNoSymlink(t)
	backup, targetRoot, mgr, _ := setupTemplateTest(t)
	source := filepath.Join(backup, "repo")
	target := filepath.Join(targetRoot, "home")
	if err := os.MkdirAll(source, 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0750); err != nil {
		t.Fatal(err)
	}
	victim := filepath.Join(backup, "victim.tmpl")
	writeTemplateFile(t, victim, "victim source")
	entry := config.SubEntry{Name: "escape", Backup: source, Files: []string{"../victim.tmpl"}}

	if err := mgr.RestoreFiles(entry, source, target); err == nil {
		t.Fatal("RestoreFiles accepted a template selection outside the entry")
	}
	if got := readTemplateTestFile(t, victim); got != "victim source" {
		t.Fatalf("escaped selection changed victim source to %q", got)
	}
	for _, path := range []string{
		filepath.Join(backup, "victim.tmpl.rendered"),
		filepath.Join(targetRoot, "victim"),
	} {
		if testPathExists(path) {
			t.Fatalf("escaped selection created %s", path)
		}
	}
}

func TestRestoreFiles_SelectedTemplate_RejectsSymlinkParentEscapes(t *testing.T) {
	tests := []struct {
		name  string
		setup func(t *testing.T, backup, target string)
	}{
		{
			name: "source parent",
			setup: func(t *testing.T, backup, _ string) {
				outside := t.TempDir()
				writeTemplateFile(t, filepath.Join(outside, "config.tmpl"), "outside source")
				if err := os.Symlink(outside, filepath.Join(backup, "nested")); err != nil {
					t.Fatal(err)
				}
			},
		},
		{
			name: "target parent",
			setup: func(t *testing.T, backup, target string) {
				writeTemplateFile(t, filepath.Join(backup, "nested", "config.tmpl"), "safe source")
				outside := t.TempDir()
				if err := os.Symlink(outside, filepath.Join(target, "nested")); err != nil {
					t.Fatal(err)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			skipIfNoSymlink(t)
			backup, target, mgr, _ := setupTemplateTest(t)
			tt.setup(t, backup, target)
			entry := config.SubEntry{Name: tt.name, Backup: backup, Files: []string{"nested/config.tmpl"}}

			if err := mgr.RestoreFiles(entry, backup, target); err == nil {
				t.Fatal("RestoreFiles accepted a symlink-parent escape")
			}
			if testPathExists(filepath.Join(backup, "nested", "config.tmpl.rendered")) {
				t.Fatal("symlink-parent escape created a rendered artifact")
			}
			if testPathExists(filepath.Join(target, "nested", "config")) {
				t.Fatal("symlink-parent escape changed the target")
			}
		})
	}
}

func linkTestPath(t *testing.T, source, destination, kind string) {
	t.Helper()
	switch kind {
	case "symlink":
		if err := os.Symlink(source, destination); err != nil {
			t.Fatal(err)
		}
	case "hardlink":
		if err := os.Link(source, destination); err != nil {
			t.Fatal(err)
		}
	default:
		t.Fatalf("unknown test link kind %q", kind)
	}
}

func templateTempPath(templatePath string) string {
	renderedPath := tmpl.RenderedPath(templatePath)
	return filepath.Join(filepath.Dir(renderedPath), "."+filepath.Base(renderedPath)+".tidydots-tmp")
}

func writeTemplateFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0600); err != nil {
		t.Fatal(err)
	}
}

func readTemplateTestFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path) //nolint:gosec // test paths are temporary
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}

func snapshotTemplateFilesystem(t *testing.T, root string) map[string]string {
	t.Helper()
	snapshot := make(map[string]string)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if strings.HasPrefix(filepath.Base(path), ".tidydots.db") {
			return nil
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		switch {
		case info.Mode()&os.ModeSymlink != 0:
			link, err := os.Readlink(path)
			if err != nil {
				return err
			}
			snapshot[rel] = "symlink:" + link
		case entry.IsDir():
			snapshot[rel] = "directory"
		default:
			content, err := os.ReadFile(path) //nolint:gosec // test paths are temporary
			if err != nil {
				return err
			}
			snapshot[rel] = fmt.Sprintf("file:%o:%x", info.Mode().Perm(), content)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("snapshot %s: %v", root, err)
	}
	return snapshot
}

func setupTemplatePlatform() *platform.Platform {
	return &platform.Platform{
		OS:       "linux",
		Distro:   "arch",
		Hostname: "testhost",
		User:     "testuser",
		EnvVars:  make(map[string]string),
	}
}
