package manager

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
)

// copyStateTestKey encodes fixture identities independently of manager lookup.
// Test targets are absolute paths under t.TempDir().
func copyStateTestKey(sourceKey, target string) string {
	return fmt.Sprintf("copy:%q:%q", sourceKey, filepath.ToSlash(target))
}

func TestCopyTemplateHistorySeparatesSharedSourceDeployments(t *testing.T) {
	for _, firstMethod := range []string{config.MethodCopy, config.MethodSymlink} {
		t.Run(firstMethod+"-then-copy", func(t *testing.T) {
			if firstMethod == config.MethodSymlink {
				skipIfNoSymlink(t)
			}
			root, targets, mgr, _ := setupTemplateTest(t)
			source := filepath.Join(root, "shared", "config.tmpl")
			writeTemplateFile(t, source, "host={{ .Hostname }}\nlocal=default\nversion=1\n")
			first := config.SubEntry{Name: "first", Method: firstMethod, Backup: "./shared", Files: []string{"config.tmpl"}}
			second := config.SubEntry{Name: "second", Method: config.MethodCopy, Backup: ".", Files: []string{"shared/config.tmpl"}}
			firstBackup, firstTarget := filepath.Join(root, "shared"), filepath.Join(targets, "a")
			secondTarget := filepath.Join(targets, "b")
			for range 2 {
				if err := mgr.RestoreFiles(first, firstBackup, firstTarget); err != nil {
					t.Fatal(err)
				}
				if err := mgr.RestoreFiles(second, root, secondTarget); err != nil {
					t.Fatal(err)
				}
			}
			firstOutput := filepath.Join(firstTarget, "config")
			secondOutput := filepath.Join(secondTarget, "shared", "config")
			writeTemplateFile(t, firstOutput, "host=testhost\nlocal=a\nversion=1\n")
			writeTemplateFile(t, secondOutput, "host=testhost\nlocal=b\nversion=1\n")
			writeTemplateFile(t, source, "host={{ .Hostname }}\nlocal=default\nversion=2\n")
			if err := mgr.RestoreFiles(first, firstBackup, firstTarget); err != nil {
				t.Fatal(err)
			}
			modified := inspectTemplateStateTest(t, mgr, second, root, secondTarget, true)
			if len(modified) != 1 || string(modified[0].PureRender) != "host=testhost\nlocal=default\nversion=1\n" {
				t.Errorf("second target diff lost its own baseline: %+v", modified)
			}
			if got := readTemplateTestFile(t, secondOutput); got != "host=testhost\nlocal=b\nversion=1\n" {
				t.Fatalf("first restore changed second target: %q", got)
			}
			for range 2 {
				if err := mgr.RestoreFiles(second, root, secondTarget); err != nil {
					t.Fatal(err)
				}
				if err := mgr.RestoreFiles(first, firstBackup, firstTarget); err != nil {
					t.Fatal(err)
				}
				if got := readTemplateTestFile(t, firstOutput); got != "host=testhost\nlocal=a\nversion=2\n" {
					t.Errorf("first output lost edits or stayed stale: %q", got)
				}
				if got := readTemplateTestFile(t, secondOutput); got != "host=testhost\nlocal=b\nversion=2\n" {
					t.Errorf("second output lost edits or stayed stale: %q", got)
				}
				inspectTemplateStateTest(t, mgr, second, root, secondTarget, false)
			}
		})
	}
}

func TestCopyTemplateHistoryRejectsUnattributedSourceOnlyHistory(t *testing.T) {
	root, target, mgr, store := setupTemplateTest(t)
	source := "version=2\nlocal=default\n"
	writeTemplateFile(t, filepath.Join(root, "shared", "config.tmpl"), source)
	output := filepath.Join(target, "shared", "config")
	edited := "version=1\nlocal=b\n"
	writeTemplateFile(t, output, edited)
	// The earlier candidate may have recorded A's newer render without recording
	// which target it deployed. A matching hash alone cannot establish B's base.
	if err := store.SaveRender(mgr.ctx, "./shared/config.tmpl", []byte(source),
		fmt.Sprintf("%x", sha256.Sum256([]byte(source))), "linux", "testhost"); err != nil {
		t.Fatal(err)
	}
	entry := config.SubEntry{Name: "second", Method: config.MethodCopy, Files: []string{"shared/config.tmpl"}}
	modified := inspectTemplateStateTest(t, mgr, entry, root, target, true)
	if len(modified) != 0 {
		t.Errorf("source-only record used as copy diff base: %+v", modified)
	}
	if err := mgr.RestoreFiles(entry, root, target); err == nil {
		t.Error("unattributed source-only history accepted for copy target")
	}
	if got := readTemplateTestFile(t, output); got != edited {
		t.Fatalf("refused upgrade changed target: %q", got)
	}
}

func TestCopyTemplateHistoryInitializesMatchingOutputWithoutAttribution(t *testing.T) {
	root, target, mgr, store := setupTemplateTest(t)
	source := "version={{ .Hostname }}\n"
	pure := "version=testhost\n"
	writeTemplateFile(t, filepath.Join(root, "config.tmpl"), source)
	output := filepath.Join(target, "config")
	writeTemplateFile(t, output, pure)
	if err := store.SaveRender(mgr.ctx, "./config.tmpl", []byte("unattributed old render"),
		"old-hash", "linux", "testhost"); err != nil {
		t.Fatal(err)
	}
	entry := config.SubEntry{Name: "copy", Method: config.MethodCopy, Files: []string{"config.tmpl"}}
	if err := mgr.RestoreFiles(entry, root, target); err != nil {
		t.Fatal(err)
	}
	if got := readTemplateTestFile(t, output); got != pure {
		t.Fatalf("matching output changed: %q", got)
	}
	if modified := inspectTemplateStateTest(t, mgr, entry, root, target, false); len(modified) != 0 {
		t.Fatalf("matching target has wrong baseline: %+v", modified)
	}
}

func TestSymlinkTemplateHistoryFollowsSharedRenderedOutput(t *testing.T) {
	skipIfNoSymlink(t)
	root, targets, mgr, _ := setupTemplateTest(t)
	source := filepath.Join(root, "shared", "config.tmpl")
	writeTemplateFile(t, source, "local=default\nversion=1\n")
	first := config.SubEntry{Name: "first", Files: []string{"config.tmpl"}}
	second := config.SubEntry{Name: "second", Files: []string{"shared/config.tmpl"}}
	firstBackup, firstTarget := filepath.Join(root, "shared"), filepath.Join(targets, "a")
	secondTarget := filepath.Join(targets, "b")
	if err := mgr.RestoreFiles(first, firstBackup, firstTarget); err != nil {
		t.Fatal(err)
	}
	if err := mgr.RestoreFiles(second, root, secondTarget); err != nil {
		t.Fatal(err)
	}
	writeTemplateFile(t, filepath.Join(firstTarget, "config"), "local=edited\nversion=1\n")
	writeTemplateFile(t, source, "local=default\nversion=2\n")
	if err := mgr.RestoreFiles(first, firstBackup, firstTarget); err != nil {
		t.Fatal(err)
	}
	modified := inspectTemplateStateTest(t, mgr, second, root, secondTarget, false)
	if len(modified) != 1 || string(modified[0].PureRender) != "local=default\nversion=2\n" {
		t.Fatalf("shared symlink baseline not updated: %+v", modified)
	}
	if got := readTemplateTestFile(t, filepath.Join(secondTarget, "shared", "config")); got != "local=edited\nversion=2\n" {
		t.Fatalf("shared rendered output = %q", got)
	}
}

func TestCopyTemplateHistorySameDeploymentAcrossEntryRoots(t *testing.T) {
	root, target, mgr, _ := setupTemplateTest(t)
	source := filepath.Join(root, "shared", "config.tmpl")
	writeTemplateFile(t, source, "local=default\nversion=1\n")
	first := config.SubEntry{Name: "first", Method: config.MethodCopy, Files: []string{"config.tmpl"}}
	if err := mgr.RestoreFiles(first, filepath.Join(root, "shared"), filepath.Join(target, "shared")); err != nil {
		t.Fatal(err)
	}
	output := filepath.Join(target, "shared", "config")
	writeTemplateFile(t, output, "local=edited\nversion=1\n")
	writeTemplateFile(t, source, "local=default\nversion=2\n")
	second := config.SubEntry{Name: "renamed", Method: config.MethodCopy, Files: []string{"shared/config.tmpl"}}
	// Exercise absolute/relative target spellings without changing the actual deployment.
	t.Chdir(filepath.Dir(target))
	relativeTarget := "./" + filepath.Base(target)
	if err := mgr.RestoreFiles(second, root, relativeTarget); err != nil {
		t.Fatal(err)
	}
	if got := readTemplateTestFile(t, output); got != "local=edited\nversion=2\n" {
		t.Fatalf("equivalent deployment lost history: %q", got)
	}
	inspectTemplateStateTest(t, mgr, second, root, target, false)
}

func TestCopyTemplateHistoryMigratesDistinctLegacyEntryKeys(t *testing.T) {
	root, targets, mgr, store := setupTemplateTest(t)
	source := filepath.Join(root, "shared", "config.tmpl")
	content := "local=default\nversion={{ index .Env \"VERSION\" }}\n"
	writeTemplateFile(t, source, content)
	first := config.SubEntry{Name: "first", Method: config.MethodCopy, Files: []string{"config.tmpl"}}
	second := config.SubEntry{Name: "second", Method: config.MethodCopy, Files: []string{"shared/config.tmpl"}}
	firstBackup, firstTarget := filepath.Join(root, "shared"), filepath.Join(targets, "a")
	secondTarget := filepath.Join(targets, "b")
	firstOutput, secondOutput := filepath.Join(firstTarget, "config"), filepath.Join(secondTarget, "shared", "config")
	// Old entry keys can legitimately retain different renders from environment changes.
	for i, key := range []string{"config.tmpl", "shared/config.tmpl"} {
		pure := fmt.Sprintf("local=default\nversion=%d\n", i+1)
		if err := store.SaveRender(mgr.ctx, key, []byte(pure),
			fmt.Sprintf("%x", sha256.Sum256([]byte(content))), "linux", "testhost"); err != nil {
			t.Fatal(err)
		}
	}
	writeTemplateFile(t, firstOutput, "local=a\nversion=1\n")
	writeTemplateFile(t, secondOutput, "local=b\nversion=2\n")
	if err := mgr.RestoreFiles(first, firstBackup, firstTarget); err != nil {
		t.Fatal(err)
	}
	if err := mgr.RestoreFiles(second, root, secondTarget); err != nil {
		t.Fatal(err)
	}
	writeTemplateFile(t, source, "local=default\nversion=3\n")
	if err := mgr.RestoreFiles(first, firstBackup, firstTarget); err != nil {
		t.Fatal(err)
	}
	modified := inspectTemplateStateTest(t, mgr, second, root, secondTarget, true)
	if len(modified) != 1 || string(modified[0].PureRender) != "local=default\nversion=2\n" {
		t.Fatalf("second deployment lost its legacy baseline: %+v", modified)
	}
	if err := mgr.RestoreFiles(second, root, secondTarget); err != nil {
		t.Fatal(err)
	}
	if got := readTemplateTestFile(t, firstOutput); got != "local=a\nversion=3\n" {
		t.Errorf("first migrated merge = %q", got)
	}
	if got := readTemplateTestFile(t, secondOutput); got != "local=b\nversion=3\n" {
		t.Errorf("second migrated merge = %q", got)
	}
}
