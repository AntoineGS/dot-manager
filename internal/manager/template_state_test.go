package manager

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/state"
)

func TestTemplateHistoryIsolatedAcrossBackupDirectories(t *testing.T) {
	for _, method := range []string{config.MethodSymlink, config.MethodCopy} {
		t.Run(method, func(t *testing.T) {
			if method == config.MethodSymlink {
				skipIfNoSymlink(t)
			}
			root, targets, mgr, store := setupTemplateTest(t)
			entries := []config.SubEntry{
				{Name: "git", Method: method, Backup: "git", Files: []string{"config.tmpl"}},
				{Name: "ghostty", Method: method, Backup: "ghostty", Files: []string{"config.tmpl"}},
			}
			for _, entry := range entries {
				writeTemplateFile(t, filepath.Join(root, entry.Backup, "config.tmpl"), entry.Name+"={{ .Hostname }}\na=1\nb=1\n")
			}
			for round := range 3 {
				for _, entry := range entries {
					backup, target := filepath.Join(root, entry.Backup), filepath.Join(targets, entry.Name)
					if err := mgr.RestoreFiles(entry, backup, target); err != nil {
						t.Fatalf("round %d restore %s: %v", round, entry.Name, err)
					}
				}
				for _, entry := range entries {
					backup, target := filepath.Join(root, entry.Backup), filepath.Join(targets, entry.Name)
					modified := inspectTemplateStateTest(t, mgr, entry, backup, target, false)
					if len(modified) != 0 {
						t.Errorf("round %d %s falsely modified: %+v", round, entry.Name, modified)
					}
				}
			}

			for i, key := range []string{"./git/config.tmpl", "./ghostty/config.tmpl"} {
				if method == config.MethodCopy {
					key = copyStateTestKey(key, filepath.Join(targets, entries[i].Name, "config"))
				}
				history, err := store.GetRenderHistory(mgr.ctx, key, -1)
				if err != nil || len(history) != 1 {
					t.Fatalf("repeated restore history %s = %v, %v; want one render", key, history, err)
				}
			}
			legacy, err := store.GetRenderHistory(mgr.ctx, "config.tmpl", -1)
			if err != nil || len(legacy) != 0 {
				t.Fatalf("new restores wrote legacy history: %v, %v", legacy, err)
			}

			gitOutput := filepath.Join(targets, "git", "config")
			writeTemplateFile(t, gitOutput, "git=testhost\na=2\nb=1\n")
			modified := inspectTemplateStateTest(t, mgr, entries[0], filepath.Join(root, "git"), filepath.Join(targets, "git"), false)
			if len(modified) != 1 || string(modified[0].PureRender) != "git=testhost\na=1\nb=1\n" {
				t.Fatalf("git diff has wrong baseline: %+v", modified)
			}
			writeTemplateFile(t, filepath.Join(root, "git", "config.tmpl"), "git={{ .Hostname }}\na=1\nb=2\n")
			inspectTemplateStateTest(t, mgr, entries[0], filepath.Join(root, "git"), filepath.Join(targets, "git"), true)
			for range 2 {
				for _, entry := range entries {
					if err := mgr.RestoreFiles(entry, filepath.Join(root, entry.Backup), filepath.Join(targets, entry.Name)); err != nil {
						t.Fatal(err)
					}
				}
				if got := readTemplateTestFile(t, gitOutput); got != "git=testhost\na=2\nb=2\n" {
					t.Fatalf("git local edit not merged independently: %q", got)
				}
				if got := readTemplateTestFile(t, filepath.Join(targets, "ghostty", "config")); got != "ghostty=testhost\na=1\nb=1\n" {
					t.Fatalf("ghostty contaminated: %q", got)
				}
			}
		})
	}
}

func inspectTemplateStateTest(t *testing.T, mgr *Manager, entry config.SubEntry, backup, target string, outdated bool) []ModifiedTemplate {
	t.Helper()
	if entry.IsCopy() {
		inspection, err := mgr.InspectCopyTemplates(entry, backup, target)
		if err != nil {
			t.Fatal(err)
		}
		if inspection.Outdated != outdated || inspection.NeedsRestore {
			t.Errorf("%s status = %+v, want outdated=%v and deployed", entry.Name, inspection, outdated)
		}
		return inspection.Modified
	}
	for _, files := range [][]string{entry.Files, nil} {
		if got := mgr.HasOutdatedTemplates(backup, files); got != outdated {
			t.Errorf("%s outdated (%v) = %v, want %v", entry.Name, files, got, outdated)
		}
	}
	modified, err := mgr.GetModifiedTemplateFiles(backup, entry.Files)
	if err != nil {
		t.Fatal(err)
	}
	if got := mgr.HasModifiedRenderedFiles(backup, entry.Files); got != (len(modified) > 0) {
		t.Errorf("modified status and diff disagree: %v / %+v", got, modified)
	}
	return modified
}

func TestTemplateHistoryLegacyUpgrade(t *testing.T) {
	for _, method := range []string{config.MethodSymlink, config.MethodCopy} {
		for _, scenario := range []string{"matching older history", "root template", "no matching hash", "ambiguous baseline", "other platform only"} {
			t.Run(method+"/"+scenario, func(t *testing.T) {
				if method == config.MethodSymlink {
					skipIfNoSymlink(t)
				}
				root, target, mgr, store := setupTemplateTest(t)
				backup := filepath.Join(root, "git")
				scopedKey := "./git/config.tmpl"
				if scenario == "root template" {
					backup = root
					scopedKey = "./config.tmpl"
				}
				if method == config.MethodCopy {
					scopedKey = copyStateTestKey(scopedKey, filepath.Join(target, "config"))
				}
				source := "git={{ .Hostname }}\na=1\nb=1\n"
				pure := "git=testhost\na=1\nb=1\n"
				edited := "git=testhost\na=2\nb=1\n"
				writeTemplateFile(t, filepath.Join(backup, "config.tmpl"), source)
				writeTemplateFile(t, filepath.Join(backup, "ordinary.conf"), "literal")
				entry := config.SubEntry{Name: "git", Method: method, Backup: backup, Files: []string{"ordinary.conf", "config.tmpl"}}
				output := filepath.Join(target, "config")
				if method == config.MethodSymlink {
					output = filepath.Join(backup, "config.tmpl.rendered")
				}
				writeTemplateFile(t, output, edited)
				if scenario != "no matching hash" {
					host := "testhost"
					if scenario == "other platform only" {
						host = "otherhost"
					}
					saveLegacyTemplateTest(t, mgr, store, source, pure, host)
				}
				if scenario == "ambiguous baseline" {
					saveLegacyTemplateTest(t, mgr, store, source, "different context\na=1\nb=1\n", "testhost")
				}
				saveLegacyTemplateTest(t, mgr, store, "ghostty source", "ghostty render", "testhost")
				legacyBefore, err := store.GetRenderHistory(mgr.ctx, "config.tmpl", -1)
				if err != nil {
					t.Fatal(err)
				}
				trusted := scenario == "matching older history" || scenario == "root template"
				modified := inspectTemplateStateTest(t, mgr, entry, backup, target, !trusted)
				if trusted && (len(modified) != 1 || string(modified[0].PureRender) != pure) {
					t.Errorf("legacy diff did not recover matching baseline: %+v", modified)
				}
				if !trusted && len(modified) != 0 {
					t.Errorf("untrusted legacy history used for diff: %+v", modified)
				}
				before, err := store.GetRenderHistory(mgr.ctx, scopedKey, -1)
				if err != nil || len(before) != 0 {
					t.Fatalf("inspection wrote scoped history: %v, %v", before, err)
				}
				mgr.DryRun = true
				_ = mgr.RestoreFiles(entry, backup, target)
				mgr.DryRun = false
				afterDryRun, err := store.GetRenderHistory(mgr.ctx, scopedKey, -1)
				if err != nil || len(afterDryRun) != 0 {
					t.Fatalf("dry run migrated history: %v, %v", afterDryRun, err)
				}
				if got := readTemplateTestFile(t, output); got != edited {
					t.Fatalf("dry run changed output: %q", got)
				}
				err = mgr.RestoreFiles(entry, backup, target)
				if trusted && err != nil {
					t.Fatalf("trusted upgrade: %v", err)
				}
				if !trusted && err == nil {
					t.Error("untrusted legacy upgrade should refuse overwrite")
				}
				if got := readTemplateTestFile(t, output); got != edited {
					t.Fatalf("upgrade lost local edits: %q", got)
				}
				legacyAfter, err := store.GetRenderHistory(mgr.ctx, "config.tmpl", -1)
				if err != nil || len(legacyAfter) != len(legacyBefore) || legacyAfter[0].ID != legacyBefore[0].ID {
					t.Fatalf("upgrade changed legacy history: %v, %v", legacyAfter, err)
				}
				if !trusted {
					if _, err := os.Lstat(filepath.Join(target, "ordinary.conf")); !errors.Is(err, os.ErrNotExist) {
						t.Errorf("untrusted history did not stop entry before literal deployment: %v", err)
					}
					return
				}
				// Fast-path migration must persist the baseline before the source changes.
				writeTemplateFile(t, filepath.Join(backup, "config.tmpl"), "git={{ .Hostname }}\na=1\nb=2\n")
				if err := mgr.RestoreFiles(entry, backup, target); err != nil {
					t.Fatal(err)
				}
				if got := readTemplateTestFile(t, output); got != "git=testhost\na=2\nb=2\n" {
					t.Fatalf("migrated merge lost edits: %q", got)
				}
			})
		}
	}
}

func TestTemplateHistoryUntrustedLegacyAllowsMissingOrForcedOutput(t *testing.T) {
	for _, method := range []string{config.MethodSymlink, config.MethodCopy} {
		for _, force := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/force=%v", method, force), func(t *testing.T) {
				if method == config.MethodSymlink {
					skipIfNoSymlink(t)
				}
				root, target, mgr, store := setupTemplateTest(t)
				backup := filepath.Join(root, "git")
				writeTemplateFile(t, filepath.Join(backup, "config.tmpl"), "git={{ .Hostname }}")
				saveLegacyTemplateTest(t, mgr, store, "ghostty source", "ghostty render", "testhost")
				output := filepath.Join(target, "config")
				if method == config.MethodSymlink {
					output = filepath.Join(backup, "config.tmpl.rendered")
				}
				if force {
					writeTemplateFile(t, output, "local edit explicitly discarded")
				}
				mgr.ForceRender = force
				entry := config.SubEntry{Name: "git", Method: method, Files: []string{"config.tmpl"}}
				if err := mgr.RestoreFiles(entry, backup, target); err != nil {
					t.Fatal(err)
				}
				if got := readTemplateTestFile(t, output); got != "git=testhost" {
					t.Fatalf("render = %q, want pure git render", got)
				}
				inspectTemplateStateTest(t, mgr, entry, backup, target, false)
			})
		}
	}
}

func TestTemplateHistoryUnavailablePreservesOutput(t *testing.T) {
	skipIfNoSymlink(t)
	root, target, mgr, store := setupTemplateTest(t)
	writeTemplateFile(t, filepath.Join(root, "config.tmpl"), "original")
	entry := config.SubEntry{Name: "git", Files: []string{"config.tmpl"}}
	if err := mgr.RestoreFiles(entry, root, target); err != nil {
		t.Fatal(err)
	}
	writeTemplateFile(t, filepath.Join(root, "config.tmpl.rendered"), "local edit")
	writeTemplateFile(t, filepath.Join(root, "config.tmpl"), "new source")
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if !mgr.HasOutdatedTemplates(root, entry.Files) {
		t.Error("unavailable history reported healthy")
	}
	if _, err := mgr.GetModifiedTemplateFiles(root, entry.Files); err == nil {
		t.Error("diff ignored unavailable history")
	}
	if err := mgr.RestoreFiles(entry, root, target); err == nil {
		t.Error("restore ignored unavailable history")
	}
	if got := readTemplateTestFile(t, filepath.Join(root, "config.tmpl.rendered")); got != "local edit" {
		t.Fatalf("unavailable history lost local edits: %q", got)
	}
}

func saveLegacyTemplateTest(t *testing.T, mgr *Manager, store *state.Store, source, pure, host string) {
	t.Helper()
	if err := store.SaveRender(mgr.ctx, "config.tmpl", []byte(pure), fmt.Sprintf("%x", sha256.Sum256([]byte(source))), "linux", host); err != nil {
		t.Fatal(err)
	}
}
