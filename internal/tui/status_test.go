package tui

import (
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/fsys"
	"github.com/AntoineGS/tidydots/internal/manager"
	"github.com/AntoineGS/tidydots/internal/platform"
	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

func TestComputeStatusResolvesChecksAndUsesActionFilter(t *testing.T) {
	repo := t.TempDir()
	linkedBackup := filepath.Join(repo, "linked")
	readyBackup := filepath.Join(repo, "ready")
	if err := os.MkdirAll(linkedBackup, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(readyBackup, 0o755); err != nil {
		t.Fatal(err)
	}

	linkedTarget := filepath.Join(t.TempDir(), "linked")
	if err := os.Symlink(linkedBackup, linkedTarget); err != nil {
		t.Fatal(err)
	}

	setup := config.SubEntry{
		Name:  "setup",
		Check: map[string]string{"linux": "false"},
		Run:   map[string]string{"linux": "true"},
	}
	cfg := &config.Config{
		Version:    3,
		BackupRoot: repo,
		Applications: []config.Application{{
			Name:        "tool",
			Description: "Tool description",
			Package:     &config.EntryPackage{Custom: map[string]string{"linux": "install tool"}},
			Entries: []config.SubEntry{
				{Name: "linked", Backup: "linked", Targets: map[string]string{"linux": linkedTarget}},
				{Name: "ready", Backup: "ready", Targets: map[string]string{"linux": filepath.Join(t.TempDir(), "ready")}},
				setup,
			},
		}},
	}
	plat := linuxPlatform()

	stub := cmdexec.NewStubRunner()
	stub.AddResult("sh", cmdexec.Result{ExitCode: 1})
	stub.AddResult("sh", cmdexec.Result{ExitCode: 1})
	mgr := manager.New(cfg, plat).WithRunner(stub)

	all, err := ComputeStatus(cfg, plat, mgr, false)
	if err != nil {
		t.Fatalf("ComputeStatus() error = %v", err)
	}
	if !all.Actionable || all.ActionsOnly {
		t.Fatalf("report flags = actionable:%t actions_only:%t, want true:false", all.Actionable, all.ActionsOnly)
	}
	if all.Counts.Applications != 1 || all.Counts.Entries != 3 || all.Counts.Packages != 1 {
		t.Fatalf("total counts = %+v, want 1 application, 3 entries, 1 package", all.Counts)
	}
	if all.Counts.ActionableApplications != 1 || all.Counts.ActionableEntries != 2 || all.Counts.ActionablePackages != 1 {
		t.Fatalf("action counts = %+v, want 1 application, 2 entries, 1 package", all.Counts)
	}
	if len(all.Applications) != 1 || len(all.Applications[0].Entries) != 3 {
		t.Fatalf("all details = %+v, want one application with three entries", all.Applications)
	}
	if got := all.Applications[0].Entries[0].State; got != StateLinked.String() {
		t.Errorf("linked entry state = %q, want %q", got, StateLinked.String())
	}
	if got := all.Applications[0].Entries[1].State; got != StateReady.String() {
		t.Errorf("ready entry state = %q, want %q", got, StateReady.String())
	}
	if got := all.Applications[0].Entries[2].State; got != StateSetupNeeded.String() {
		t.Errorf("setup entry state = %q, want %q", got, StateSetupNeeded.String())
	}

	actions, err := ComputeStatus(cfg, plat, mgr, true)
	if err != nil {
		t.Fatalf("ComputeStatus(actionsOnly) error = %v", err)
	}
	if !actions.ActionsOnly || !actions.Actionable {
		t.Fatalf("action report flags = actionable:%t actions_only:%t, want true:true", actions.Actionable, actions.ActionsOnly)
	}
	if len(actions.Applications) != 1 || len(actions.Applications[0].Entries) != 2 {
		t.Fatalf("action details = %+v, want one application with two entries", actions.Applications)
	}
	if actions.Applications[0].Entries[0].Name != "ready" || actions.Applications[0].Entries[1].Name != "setup" {
		t.Fatalf("action entries = %+v, want ready and setup", actions.Applications[0].Entries)
	}
}

func TestComputeStatusPropagatesSetupDiagnosticsAndOmitsSuccessfulErrors(t *testing.T) {
	failed := config.SubEntry{
		Name:      "remote-check",
		CheckMode: config.CheckModeStatus,
		Check:     map[string]string{"linux": "check"},
		Run:       map[string]string{"linux": "run"},
	}
	successful := config.SubEntry{
		Name:      "local-check",
		CheckMode: config.CheckModeStatus,
		Check:     map[string]string{"linux": "check"},
		Run:       map[string]string{"linux": "run"},
	}
	cfg := &config.Config{
		Version:    3,
		BackupRoot: t.TempDir(),
		Applications: []config.Application{{
			Name:    "tool",
			Entries: []config.SubEntry{failed, successful},
		}},
	}
	plat := linuxPlatform()
	stub := cmdexec.NewStubRunner()
	stub.AddResult("sh", cmdexec.Result{ExitCode: 3, Stderr: []byte("remote unavailable")})
	stub.AddResult("sh", cmdexec.Result{ExitCode: 0})
	mgr := manager.New(cfg, plat).WithRunner(stub)

	report, err := ComputeStatus(cfg, plat, mgr, false)
	if err != nil {
		t.Fatalf("ComputeStatus() error = %v", err)
	}
	if len(report.Applications) != 1 || len(report.Applications[0].Entries) != 2 {
		t.Fatalf("report entries = %+v, want two setup entries", report.Applications)
	}

	failedEntry := report.Applications[0].Entries[0]
	if failedEntry.State != StateCheckFailed.String() || failedEntry.Error != "remote unavailable" || !failedEntry.Actionable {
		t.Fatalf("failed entry = %+v, want check failed, diagnostic, actionable", failedEntry)
	}
	successfulEntry := report.Applications[0].Entries[1]
	if successfulEntry.Error != "" {
		t.Fatalf("successful entry error = %q, want empty", successfulEntry.Error)
	}

	data, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal(report): %v", err)
	}
	if !strings.Contains(string(data), `"error":"remote unavailable"`) {
		t.Fatalf("JSON omitted failed diagnostic: %s", data)
	}
	if strings.Contains(string(data), `"error":""`) {
		t.Fatalf("JSON encoded an empty successful diagnostic: %s", data)
	}
}

func TestComputeStatusAmbiguousTemplateSelectionIsActionable(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("status fixture requires symlink creation")
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

	templatePath := filepath.Join(backupPath, "config.tmpl")
	renderedPath := tmpl.RenderedPath(templatePath)
	aliasPath := filepath.Join(backupPath, "config")
	if err := os.WriteFile(templatePath, []byte("config"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(renderedPath, []byte("rendered"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(renderedPath, aliasPath); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(aliasPath, filepath.Join(targetPath, "config")); err != nil {
		t.Fatal(err)
	}

	plat := linuxPlatform()
	cfg := &config.Config{
		Version:    3,
		BackupRoot: root,
		Applications: []config.Application{{
			Name: "tool",
			Entries: []config.SubEntry{{
				Name:    "ambiguous",
				Backup:  backupPath,
				Files:   []string{"config", "config.tmpl"},
				Targets: map[string]string{"linux": targetPath},
			}},
		}},
	}

	report, err := ComputeStatus(cfg, plat, manager.New(cfg, plat), true)
	if err != nil {
		t.Fatalf("ComputeStatus(actionsOnly): %v", err)
	}
	if len(report.Applications) != 1 || len(report.Applications[0].Entries) != 1 {
		t.Fatalf("action report = %+v, want ambiguous entry", report)
	}
	entry := report.Applications[0].Entries[0]
	if entry.State != StateReady.String() || !entry.Actionable {
		t.Fatalf("ambiguous entry status = %+v, want actionable Ready", entry)
	}
}

func TestNewModelWithActionFilterEnablesFilterBeforeChecksSettle(t *testing.T) {
	cfg := &config.Config{
		Version: 3,
		Applications: []config.Application{{
			Name:    "tool",
			Entries: []config.SubEntry{{Name: "config", Backup: "config", Targets: map[string]string{"linux": t.TempDir()}}},
		}},
	}

	model := NewModelWithActionFilter(cfg, linuxPlatform(), false, true)
	if !model.actionFilterEnabled {
		t.Fatal("action filter was not enabled at startup")
	}
	if model.pendingStateChecks != 1 {
		t.Fatalf("pending state checks = %d, want 1", model.pendingStateChecks)
	}
	if len(model.tableRows) != 1 || model.tableRows[0].AppName != "tool" {
		t.Fatalf("startup action-filter rows = %+v, want the loading application row", model.tableRows)
	}
}

func TestComputeStatusSelectedTemplateStates(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, selected, unselected string)
		want   PathState
	}{
		{name: "linked", want: StateLinked},
		{
			name: "outdated",
			mutate: func(t *testing.T, selected, _ string) {
				writeStatusTemplateFile(t, selected, "selected-v2")
			},
			want: StateOutdated,
		},
		{
			name: "modified",
			mutate: func(t *testing.T, selected, _ string) {
				writeStatusTemplateFile(t, tmpl.RenderedPath(selected), "selected-user-edit")
			},
			want: StateModified,
		},
		{
			name: "missing-rendered",
			mutate: func(t *testing.T, selected, _ string) {
				if err := os.Remove(tmpl.RenderedPath(selected)); err != nil {
					t.Fatal(err)
				}
			},
			want: StateOutdated,
		},
		{
			name: "unselected-template",
			mutate: func(t *testing.T, _, unselected string) {
				writeStatusTemplateFile(t, unselected, "unselected-v2")
				writeStatusTemplateFile(t, tmpl.RenderedPath(unselected), "unselected-user-edit")
			},
			want: StateLinked,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newStatusSelectedTemplateFixture(t, false)
			if tt.mutate != nil {
				tt.mutate(t, fixture.selectedPath, fixture.unselectedPath)
			}

			report, err := ComputeStatus(fixture.config, fixture.platform, fixture.manager, false)
			if err != nil {
				t.Fatalf("ComputeStatus: %v", err)
			}
			if len(report.Applications) != 1 || len(report.Applications[0].Entries) != 1 {
				t.Fatalf("status entries = %+v, want one selected entry", report.Applications)
			}
			if got := report.Applications[0].Entries[0].State; got != tt.want.String() {
				t.Errorf("status state = %q, want %q", got, tt.want.String())
			}
		})
	}
}

func TestComputeStatusCopyTemplateUsesRenderedTarget(t *testing.T) {
	fixture := newStatusSelectedTemplateFixture(t, true)

	report, err := ComputeStatus(fixture.config, fixture.platform, fixture.manager, false)
	if err != nil {
		t.Fatalf("ComputeStatus: %v", err)
	}
	if got := report.Applications[0].Entries[0].State; got != StateLinked.String() {
		t.Fatalf("copy template status = %q, want healthy rendered target %q", got, StateLinked.String())
	}
}

func TestComputeStatusCopyTemplateStates(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, fixture statusTemplateFixture)
		want   PathState
	}{
		{
			name: "source drift",
			mutate: func(t *testing.T, fixture statusTemplateFixture) {
				writeStatusTemplateFile(t, fixture.selectedPath, "selected-v2")
			},
			want: StateOutdated,
		},
		{
			name: "live target edit",
			mutate: func(t *testing.T, fixture statusTemplateFixture) {
				target := fixture.config.Applications[0].Entries[0].Targets["linux"]
				writeStatusTemplateFile(t, filepath.Join(target, "selected"), "selected-user-edit")
			},
			want: StateModified,
		},
		{
			name: "missing target",
			mutate: func(t *testing.T, fixture statusTemplateFixture) {
				target := fixture.config.Applications[0].Entries[0].Targets["linux"]
				if err := os.Remove(filepath.Join(target, "selected")); err != nil {
					t.Fatal(err)
				}
			},
			want: StateReady,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newStatusSelectedTemplateFixture(t, true)
			tt.mutate(t, fixture)
			report, err := ComputeStatus(fixture.config, fixture.platform, fixture.manager, false)
			if err != nil {
				t.Fatalf("ComputeStatus: %v", err)
			}
			got := report.Applications[0].Entries[0].State
			if got != tt.want.String() {
				t.Fatalf("copy template state = %q, want %q", got, tt.want.String())
			}
		})
	}
}

func TestComputeStatusCopyTemplateMissingHistoryIsOutdated(t *testing.T) {
	fixture := newStatusSelectedTemplateFixture(t, true)
	withoutHistory := manager.New(fixture.config, fixture.platform)

	report, err := ComputeStatus(fixture.config, fixture.platform, withoutHistory, false)
	if err != nil {
		t.Fatal(err)
	}
	if got := report.Applications[0].Entries[0].State; got != StateOutdated.String() {
		t.Fatalf("copy template state without history = %q, want %q", got, StateOutdated.String())
	}
}

func TestComputeStatusCopyTemplateUnsafeSelectionIsUnavailable(t *testing.T) {
	fixture := newStatusSelectedTemplateFixture(t, true)
	fixture.config.Applications[0].Entries[0].Files = []string{"selected.tmpl", "selected"}

	report, err := ComputeStatus(fixture.config, fixture.platform, fixture.manager, true)
	if err != nil {
		t.Fatal(err)
	}
	entry := report.Applications[0].Entries[0]
	if entry.State != StateUnavailable.String() || !entry.Actionable {
		t.Fatalf("unsafe copy selection status = %+v, want actionable Unavailable", entry)
	}
}

func TestComputeStatusCopyTemplateDeniedReadIsUnavailableWithoutSudo(t *testing.T) {
	fixture := newStatusSelectedTemplateFixture(t, true)
	target := fixture.config.Applications[0].Entries[0].Targets["linux"]
	fixture.config.Applications[0].Entries[0].Sudo = true
	stub := cmdexec.NewStubRunner()
	fixture.manager = fixture.manager.WithFS(deniedStatusReadFS{
		FS:     fsys.OsFS{},
		denied: filepath.Join(target, "selected"),
	}).WithRunner(stub)

	report, err := ComputeStatus(fixture.config, fixture.platform, fixture.manager, true)
	if err != nil {
		t.Fatal(err)
	}
	entry := report.Applications[0].Entries[0]
	if entry.State != StateUnavailable.String() || !entry.Actionable {
		t.Fatalf("denied copy target status = %+v, want actionable Unavailable", entry)
	}
	if len(stub.Calls) != 0 {
		t.Fatalf("status inspection spawned privileged commands: %+v", stub.Calls)
	}
}

type statusTemplateFixture struct {
	config         *config.Config
	platform       *platform.Platform
	manager        *manager.Manager
	selectedPath   string
	unselectedPath string
}

func newStatusSelectedTemplateFixture(t *testing.T, copyMode bool) statusTemplateFixture {
	t.Helper()
	root := t.TempDir()
	backupPath := filepath.Join(root, "backup")
	targetPath := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(backupPath, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetPath, 0o750); err != nil {
		t.Fatal(err)
	}

	plat := linuxPlatform()
	cfg := &config.Config{
		Version:    3,
		BackupRoot: root,
		Applications: []config.Application{{
			Name: "tool",
			Entries: []config.SubEntry{{
				Name:    "config",
				Backup:  backupPath,
				Files:   []string{"selected.tmpl"},
				Targets: map[string]string{"linux": targetPath},
			}},
		}},
	}
	entry := cfg.Applications[0].Entries[0]
	if copyMode {
		entry.Method = config.MethodCopy
	}
	cfg.Applications[0].Entries[0] = entry

	mgr := manager.New(cfg, plat)
	if err := mgr.InitStateStore(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mgr.Close() }) //nolint:errcheck // test cleanup

	selectedPath := filepath.Join(backupPath, "selected.tmpl")
	unselectedPath := filepath.Join(backupPath, "unselected.tmpl")
	writeStatusTemplateFile(t, selectedPath, "selected-v1")
	writeStatusTemplateFile(t, unselectedPath, "unselected-v1")

	if copyMode {
		if err := mgr.RestoreFiles(entry, backupPath, targetPath); err != nil {
			t.Fatal(err)
		}
	} else {
		allTemplatesEntry := entry
		allTemplatesEntry.Files = []string{"selected.tmpl", "unselected.tmpl"}
		if err := mgr.RestoreFiles(allTemplatesEntry, backupPath, targetPath); err != nil {
			t.Fatal(err)
		}
	}

	return statusTemplateFixture{
		config:         cfg,
		platform:       plat,
		manager:        mgr,
		selectedPath:   selectedPath,
		unselectedPath: unselectedPath,
	}
}

func writeStatusTemplateFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

type deniedStatusReadFS struct {
	fsys.FS
	denied string
}

func (f deniedStatusReadFS) ReadFile(name string) ([]byte, error) {
	if name == f.denied {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrPermission}
	}
	return f.FS.ReadFile(name)
}
