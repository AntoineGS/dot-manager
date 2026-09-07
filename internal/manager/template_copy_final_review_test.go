package manager

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/platform"
	"github.com/AntoineGS/tidydots/internal/state"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
	"github.com/AntoineGS/tidydots/internal/testutil"
)

func TestRestoreCopyTemplateRejectsConcreteRepositoryAndStateClaims(t *testing.T) {
	tests := []struct {
		name  string
		files []string
	}{
		{name: "template first", files: []string{".tidydots.db.tmpl", "keep.conf"}},
		{name: "literal first", files: []string{"keep.conf", ".tidydots.db.tmpl"}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := testutil.CanonicalTempDir(t)
			source := filepath.Join(repo, "templates")
			if err := os.MkdirAll(source, 0o750); err != nil {
				t.Fatal(err)
			}
			cfg := &config.Config{Version: 3, BackupRoot: repo}
			plat := &platform.Platform{
				OS:       platform.OSLinux,
				Hostname: "review-host",
				EnvVars:  map[string]string{},
			}
			mgr := New(cfg, plat)
			store, err := state.Open(mgr.ctx, filepath.Join(repo, ".tidydots.db"))
			if err != nil {
				t.Fatalf("open state: %v", err)
			}
			mgr.stateStore = store
			t.Cleanup(func() { _ = store.Close() }) //nolint:errcheck // test cleanup

			templateSource := filepath.Join(source, ".tidydots.db.tmpl")
			literalSource := filepath.Join(source, "keep.conf")
			writeTemplateFile(t, templateSource, "replacement")
			writeTemplateFile(t, literalSource, "keep")
			entry := config.SubEntry{
				Name:   "unsafe-repository-target",
				Method: config.MethodCopy,
				Backup: source,
				Files:  tt.files,
			}

			beforeRepo := snapshotTemplateFilesystem(t, repo)
			beforeHistory, err := store.GetRenderHistory(mgr.ctx, copyStateTestKey("./templates/.tidydots.db.tmpl", filepath.Join(repo, ".tidydots.db")), 10)
			if err != nil {
				t.Fatalf("history before: %v", err)
			}
			beforeDB := finalReviewPathBytes(t, filepath.Join(repo, ".tidydots.db"))
			beforeWAL := finalReviewPathBytes(t, filepath.Join(repo, ".tidydots.db-wal"))
			beforeSHM := finalReviewPathBytes(t, filepath.Join(repo, ".tidydots.db-shm"))

			if err := mgr.RestoreFiles(entry, source, repo); err == nil {
				t.Fatal("restore accepted a concrete target inside the repository/state namespace")
			}
			if after := snapshotTemplateFilesystem(t, repo); !reflect.DeepEqual(after, beforeRepo) {
				t.Fatalf("repository changed during rejected preflight: before=%v after=%v", beforeRepo, after)
			}
			for path, before := range map[string]finalReviewBytes{
				filepath.Join(repo, ".tidydots.db"):     beforeDB,
				filepath.Join(repo, ".tidydots.db-wal"): beforeWAL,
				filepath.Join(repo, ".tidydots.db-shm"): beforeSHM,
			} {
				if after := finalReviewPathBytes(t, path); !reflect.DeepEqual(after, before) {
					t.Fatalf("state file %s changed during rejected preflight: before=%v after=%v", path, before, after)
				}
			}
			afterHistory, err := store.GetRenderHistory(mgr.ctx, copyStateTestKey("./templates/.tidydots.db.tmpl", filepath.Join(repo, ".tidydots.db")), 10)
			if err != nil {
				t.Fatalf("history after: %v", err)
			}
			if !reflect.DeepEqual(afterHistory, beforeHistory) {
				t.Fatalf("history changed during rejected preflight: before=%v after=%v", beforeHistory, afterHistory)
			}
		})
	}
}

func TestRestoreCopyTemplateRejectsAbsentGeneratedClaimInsideRepository(t *testing.T) {
	tests := []struct {
		name  string
		files []string
	}{
		{
			name:  "template first",
			files: []string{"templates/foo.tmpl", "repo/templates/foo.tmpl.conflict"},
		},
		{
			name:  "literal first",
			files: []string{"repo/templates/foo.tmpl.conflict", "templates/foo.tmpl"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			home := testutil.CanonicalTempDir(t)
			repo := filepath.Join(home, "repo")
			if err := os.MkdirAll(filepath.Join(repo, "templates"), 0o750); err != nil {
				t.Fatal(err)
			}
			writeTemplateFile(t, filepath.Join(repo, "templates", "foo.tmpl"), "value=1")
			writeTemplateFile(t, filepath.Join(repo, "repo", "templates", "foo.tmpl.conflict"), "literal")
			mgr := New(&config.Config{Version: 3, BackupRoot: repo}, &platform.Platform{
				OS: platform.OSLinux, EnvVars: map[string]string{}, Hostname: "review-host",
			})
			entry := config.SubEntry{Name: "unsafe-generated-target", Method: config.MethodCopy,
				Backup: repo, Files: tt.files}

			beforeHome := snapshotTemplateFilesystem(t, home)
			if err := mgr.RestoreFiles(entry, repo, home); err == nil {
				t.Fatal("restore accepted a target claim for an absent generated artifact")
			}
			if after := snapshotTemplateFilesystem(t, home); !reflect.DeepEqual(after, beforeHome) {
				t.Fatalf("home/repository changed during rejected preflight: before=%v after=%v", beforeHome, after)
			}
		})
	}
}

func TestRestoreCopyTemplateAllowsTargetOutsideNestedRepository(t *testing.T) {
	home := testutil.CanonicalTempDir(t)
	repo := filepath.Join(home, "dotfiles")
	if err := os.MkdirAll(repo, 0o750); err != nil {
		t.Fatal(err)
	}
	writeTemplateFile(t, filepath.Join(repo, "config.tmpl"), "value=1")
	mgr := New(&config.Config{Version: 3, BackupRoot: repo}, &platform.Platform{
		OS: platform.OSLinux, EnvVars: map[string]string{}, Hostname: "review-host",
	})
	entry := config.SubEntry{Name: "home-target", Method: config.MethodCopy,
		Backup: repo, Files: []string{"config.tmpl"}}

	if err := mgr.RestoreFiles(entry, repo, home); err != nil {
		t.Fatalf("restore with repository nested below target root: %v", err)
	}
	if got := readTemplateTestFile(t, filepath.Join(home, "config")); got != "value=1" {
		t.Fatalf("target = %q, want value=1", got)
	}
	if testPathExists(filepath.Join(repo, "config")) {
		t.Fatal("copy restore created a repository alias")
	}
}

func TestRestoreCopyTemplateReplacesDanglingTargetWithoutFollowingReferent(t *testing.T) {
	tests := []struct {
		name    string
		history bool
		force   bool
	}{
		{name: "no history"},
		{name: "no history force", force: true},
		{name: "history", history: true},
		{name: "history force", history: true, force: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backup, target, mgr, _ := setupTemplateTest(t)
			source := filepath.Join(backup, "root.tmpl")
			destination := filepath.Join(target, "root")
			entry := config.SubEntry{Name: "dangling", Method: config.MethodCopy,
				Backup: backup, Files: []string{"root.tmpl"}}
			writeTemplateFile(t, source, "value=1")
			if tt.history {
				if err := mgr.RestoreFiles(entry, backup, target); err != nil {
					t.Fatalf("seed restore: %v", err)
				}
				if err := os.Remove(destination); err != nil {
					t.Fatalf("remove seed target: %v", err)
				}
			}
			missingReferent := filepath.Join(target, "not-created")
			if err := os.Symlink(missingReferent, destination); err != nil {
				t.Fatal(err)
			}
			mgr.ForceRender = tt.force

			if err := mgr.RestoreFiles(entry, backup, target); err != nil {
				t.Fatalf("restore dangling target: %v", err)
			}
			info, err := os.Lstat(destination)
			if err != nil || !info.Mode().IsRegular() {
				t.Fatalf("destination = %v, %v; want regular file", info, err)
			}
			if got := readTemplateTestFile(t, destination); got != "value=1" {
				t.Fatalf("destination = %q, want value=1", got)
			}
			if _, err := os.Lstat(missingReferent); !os.IsNotExist(err) {
				t.Fatalf("dangling referent was created: %v", err)
			}
		})
	}
}

func TestRestoreCopyTemplateMigrationUsesSourceModeOnNativeChangedAndHashPaths(t *testing.T) {
	skipIfNoSymlink(t)
	tests := []struct {
		name    string
		changed bool
	}{
		{name: "changed source", changed: true},
		{name: "matching hash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backup, target, mgr, store := setupTemplateTest(t)
			source := filepath.Join(backup, "root.tmpl")
			rendered := tmpl.RenderedPath(source)
			alias := filepath.Join(backup, "root")
			destination := filepath.Join(target, "root")
			writeTemplateFile(t, source, "old")
			if err := os.Chmod(source, 0o640); err != nil {
				t.Fatal(err)
			}
			writeTemplateFile(t, rendered, "old")
			if err := os.Chmod(rendered, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Base(rendered), alias); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(alias, destination); err != nil {
				t.Fatal(err)
			}
			if err := store.SaveRender(mgr.ctx, copyStateTestKey("./root.tmpl", destination), []byte("old"),
				fmt.Sprintf("%x", sha256.Sum256([]byte("old"))), "linux", "testhost"); err != nil {
				t.Fatal(err)
			}
			if tt.changed {
				if err := os.WriteFile(source, []byte("new"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			entry := config.SubEntry{Name: "mode", Method: config.MethodCopy,
				Backup: backup, Files: []string{"root.tmpl"}}

			if err := mgr.RestoreFiles(entry, backup, target); err != nil {
				t.Fatalf("migration restore: %v", err)
			}
			info, err := os.Stat(destination)
			if err != nil {
				t.Fatal(err)
			}
			if got := info.Mode().Perm(); got != 0o640 {
				t.Fatalf("migrated mode = %o, want source mode 0640", got)
			}
		})
	}
}

func TestRestoreCopyTemplateMigrationUses0600ForSudoChangedAndHashPaths(t *testing.T) {
	skipIfNoSudo(t)
	tests := []struct {
		name    string
		changed bool
	}{
		{name: "changed source", changed: true},
		{name: "matching hash"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			backup, target, mgr, store := setupTemplateTest(t)
			source := filepath.Join(backup, "root.tmpl")
			rendered := tmpl.RenderedPath(source)
			alias := filepath.Join(backup, "root")
			destination := filepath.Join(target, "root")
			writeTemplateFile(t, source, "old")
			if err := os.Chmod(source, 0o644); err != nil {
				t.Fatal(err)
			}
			writeTemplateFile(t, rendered, "old")
			if err := os.Chmod(rendered, 0o644); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Base(rendered), alias); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(alias, destination); err != nil {
				t.Fatal(err)
			}
			if err := store.SaveRender(mgr.ctx, copyStateTestKey("./root.tmpl", destination), []byte("old"),
				fmt.Sprintf("%x", sha256.Sum256([]byte("old"))), "linux", "testhost"); err != nil {
				t.Fatal(err)
			}
			if tt.changed {
				if err := os.WriteFile(source, []byte("new"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			entry := config.SubEntry{Name: "mode", Method: config.MethodCopy, Sudo: true,
				Backup: backup, Files: []string{"root.tmpl"}}
			runner := cmdexec.NewStubRunner()
			stage := filepath.Join(target, ".tidydots-copy-stage")
			runner.AddResult("mktemp", cmdexec.Result{Stdout: []byte(stage + "\n")})
			runner.AddResult("dd", cmdexec.Result{})
			runner.AddResult("chmod", cmdexec.Result{})
			runner.AddResult("mv", cmdexec.Result{})
			mgr = mgr.WithRunner(runner)

			if err := mgr.RestoreFiles(entry, backup, target); err != nil {
				t.Fatalf("sudo migration restore: %v", err)
			}
			var chmodCall *cmdexec.Call
			for i := range runner.Calls {
				if runner.Calls[i].Name == "chmod" {
					chmodCall = &runner.Calls[i]
					break
				}
			}
			if chmodCall == nil || len(chmodCall.Args) != 3 || chmodCall.Args[0] != "600" || !chmodCall.Sudo {
				t.Fatalf("chmod call = %+v, want sudo chmod 600", chmodCall)
			}
		})
	}
}

func TestInspectCopyTemplatesRejectsTargetHardlinkToSourceWithoutSudo(t *testing.T) {
	backup, target, mgr, _ := setupTemplateTest(t)
	source := filepath.Join(backup, "root.tmpl")
	destination := filepath.Join(target, "root")
	entry := config.SubEntry{Name: "unsafe-status", Method: config.MethodCopy, Sudo: true,
		Backup: backup, Files: []string{"root.tmpl"}}
	writeTemplateFile(t, source, "value=1")
	seedEntry := entry
	seedEntry.Sudo = false
	if err := mgr.RestoreFiles(seedEntry, backup, target); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(destination); err != nil {
		t.Fatal(err)
	}
	if err := os.Link(source, destination); err != nil {
		t.Skipf("hardlinks unavailable: %v", err)
	}
	runner := cmdexec.NewStubRunner()

	if _, err := mgr.WithRunner(runner).InspectCopyTemplates(entry, backup, target); err == nil {
		t.Fatal("status reported a target hardlink to the source as healthy")
	}
	if len(runner.Calls) != 0 {
		t.Fatalf("status inspection invoked privileged commands: %+v", runner.Calls)
	}
}

func TestRestoreCopyTemplateLogsConflictAndDryRunActionsWithoutContent(t *testing.T) {
	t.Run("conflict", func(t *testing.T) {
		backup, target, mgr, _ := setupTemplateTest(t)
		source := filepath.Join(backup, "root.tmpl")
		destination := filepath.Join(target, "root")
		entry := config.SubEntry{Name: "logged", Method: config.MethodCopy,
			Backup: backup, Files: []string{"root.tmpl"}}
		var logs bytes.Buffer
		mgr = mgr.WithLogger(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo})))
		writeTemplateFile(t, source, "value=1")
		if err := mgr.RestoreFiles(entry, backup, target); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(destination, []byte("SECRET-USER-EDIT"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(source, []byte("value=2"), 0o600); err != nil {
			t.Fatal(err)
		}
		logs.Reset()
		if err := mgr.RestoreFiles(entry, backup, target); err != nil {
			t.Fatal(err)
		}
		output := logs.String()
		for _, want := range []string{"root.tmpl", "root", "conflict"} {
			if !strings.Contains(output, want) {
				t.Errorf("logs = %q, want %q", output, want)
			}
		}
		if strings.Contains(output, "SECRET-USER-EDIT") || strings.Contains(output, "value=2") {
			t.Fatalf("logs exposed rendered/user content: %q", output)
		}
	})

	t.Run("dry-run", func(t *testing.T) {
		backup, target, mgr, _ := setupTemplateTest(t)
		source := filepath.Join(backup, "root.tmpl")
		destination := filepath.Join(target, "root")
		entry := config.SubEntry{Name: "dry-run", Method: config.MethodCopy,
			Backup: backup, Files: []string{"root.tmpl"}}
		writeTemplateFile(t, source, "DRY-RUN-TEMPLATE")
		writeTemplateFile(t, destination, "DRY-RUN-TARGET")
		var logs bytes.Buffer
		mgr = mgr.WithLogger(slog.New(slog.NewTextHandler(&logs, &slog.HandlerOptions{Level: slog.LevelInfo})))
		mgr.DryRun = true
		if err := mgr.RestoreFiles(entry, backup, target); err != nil {
			t.Fatal(err)
		}
		output := logs.String()
		for _, want := range []string{"root.tmpl", "root", "dry"} {
			if !strings.Contains(strings.ToLower(output), strings.ToLower(want)) {
				t.Errorf("logs = %q, want %q", output, want)
			}
		}
		if strings.Contains(output, "DRY-RUN-TEMPLATE") || strings.Contains(output, "DRY-RUN-TARGET") {
			t.Fatalf("logs exposed rendered/user content: %q", output)
		}
		if got := readTemplateTestFile(t, destination); got != "DRY-RUN-TARGET" {
			t.Fatalf("dry-run target = %q, want unchanged target", got)
		}
	})
}

type finalReviewBytes struct {
	data   []byte
	exists bool
}

func finalReviewPathBytes(t *testing.T, path string) finalReviewBytes {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // test path is temporary
	if os.IsNotExist(err) {
		return finalReviewBytes{}
	}
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return finalReviewBytes{data: data, exists: true}
}
