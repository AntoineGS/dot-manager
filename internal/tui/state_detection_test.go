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

func entryWhenConfig(t *testing.T) *config.Config {
	t.Helper()
	return &config.Config{
		Version: 3,
		Applications: []config.Application{
			{
				Name: "entry-filtered",
				Entries: []config.SubEntry{
					{
						Name:    "included",
						When:    `{{ eq .OS "linux" }}`,
						Backup:  "./included",
						Targets: map[string]string{"linux": t.TempDir()},
					},
					{
						Name:    "excluded",
						When:    `{{ eq .OS "windows" }}`,
						Backup:  "./excluded",
						Targets: map[string]string{"linux": t.TempDir()},
					},
				},
			},
			{
				Name: "empty-after-entry-filter",
				Entries: []config.SubEntry{{
					Name:    "excluded-only",
					When:    `{{ eq .OS "windows" }}`,
					Backup:  "./excluded-only",
					Targets: map[string]string{"linux": t.TempDir()},
				}},
			},
			{
				Name:    "package-only",
				Package: &config.EntryPackage{Custom: map[string]string{"linux": "install package-only"}},
			},
		},
	}
}

func TestNewModelEntryWhenExcludesEntriesEverywhere(t *testing.T) {
	cfg := entryWhenConfig(t)
	m := NewModel(cfg, linuxPlatform(), false)

	var visible *ApplicationItem
	for i := range m.Applications {
		if m.Applications[i].Application.Name == "entry-filtered" {
			visible = &m.Applications[i]
		}
		if m.Applications[i].Application.Name == "empty-after-entry-filter" {
			t.Fatal("application with no package and no included entries was retained")
		}
	}
	if visible == nil {
		t.Fatal("entry-filtered application was omitted")
	}
	if got := len(visible.SubItems); got != 1 {
		t.Fatalf("visible sub-items = %d, want 1", got)
	}
	if got := visible.SubItems[0].SubEntry.Name; got != "included" {
		t.Fatalf("visible entry = %q, want included", got)
	}
	if got := len(cfg.Applications[0].Entries); got != 2 {
		t.Fatalf("raw config entries = %d, want 2", got)
	}

	if got := m.pendingStateChecks; got != 2 {
		t.Fatalf("initial state checks = %d, want 2 (one entry and one package)", got)
	}
	if _, got := m.checkSubEntryStatesCmd(); got != 1 {
		t.Fatalf("sub-entry state checks = %d, want 1", got)
	}

	visible.Expanded = true
	m.rebuildTable()
	for _, row := range m.tableRows {
		if strings.Contains(row.Data[0], "excluded") {
			t.Fatalf("table row included excluded entry: %q", row.Data[0])
		}
	}
	m.searchText = "excluded"
	m.rebuildTable()
	for _, row := range m.tableRows {
		if strings.Contains(row.Data[0], "excluded") {
			t.Fatalf("search row included excluded entry: %q", row.Data[0])
		}
	}

	m.searchText = ""
	m.rebuildTable()
	appIdx := -1
	for i := range m.Applications {
		if m.Applications[i].Application.Name == "entry-filtered" {
			appIdx = i
			break
		}
	}
	m.toggleAppSelection(appIdx)
	if !m.selectedSubEntries[subEntryKey{app: "entry-filtered", sub: "included"}] {
		t.Fatal("parent selection did not select included entry")
	}
	if m.selectedSubEntries[subEntryKey{app: "entry-filtered", sub: "excluded"}] {
		t.Fatal("parent selection selected excluded entry")
	}
	items := m.collectBatchRestoreItems()
	if len(items) != 1 || m.Applications[items[0].appIdx].SubItems[items[0].subIdx].SubEntry.Name != "included" {
		t.Fatalf("batch restore items = %+v, want only included entry", items)
	}

	var packageOnly bool
	for _, app := range m.Applications {
		if app.Application.Name == "package-only" && len(app.SubItems) == 0 {
			packageOnly = true
		}
	}
	if !packageOnly {
		t.Fatal("package-only application was not retained")
	}
}

// collectMsgs runs a tea.Cmd and flattens any tea.BatchMsg it produces into
// the individual messages returned by each dispatched sub-command. A nil cmd
// (e.g. zero sub-entries dispatched) yields no messages.
func collectMsgs(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}

	msg := cmd()
	if batch, ok := msg.(tea.BatchMsg); ok {
		var out []tea.Msg
		for _, sub := range batch {
			out = append(out, collectMsgs(sub)...)
		}
		return out
	}

	if msg == nil {
		return nil
	}

	return []tea.Msg{msg}
}

// setupSubEntry returns a canonical setup sub-entry (Run set, no Backup) for
// use across these tests. Setup entries never have a Backup; detection must
// not fall through to the config-state logic that expects one.
func setupSubEntry() config.SubEntry {
	return config.SubEntry{
		Name:  "enable-service",
		Check: map[string]string{"linux": "systemctl --user is-enabled --quiet vicinae.service"},
		Run:   map[string]string{"linux": "systemctl --user enable --now vicinae.service"},
	}
}

// newStubManager builds a Manager on Linux backed by the given stub runner.
func newStubManager(stub *cmdexec.StubRunner) *manager.Manager {
	cfg := &config.Config{Version: 3, BackupRoot: "/repo"}
	plat := &platform.Platform{OS: platform.OSLinux, EnvVars: map[string]string{}}

	return manager.New(cfg, plat).WithRunner(stub)
}

func TestDetectSetupPathState_NilManager_ReturnsSetupOk(t *testing.T) {
	got, diagnostic := detectSetupPathState(setupSubEntry(), nil)
	if got != StateSetupOk || diagnostic != "" {
		t.Errorf("detectSetupPathState with nil manager = (%v, %q), want (StateSetupOk, \"\") (a nil manager cannot run the check, so it must not falsely flag the entry)", got, diagnostic)
	}
}

func TestDetectSetupPathState_StatusModeMapsExitCodesAndDiagnostics(t *testing.T) {
	for _, tt := range []struct {
		code       int
		state      PathState
		diagnostic string
	}{
		{code: 0, state: StateSetupOk, diagnostic: ""},
		{code: 1, state: StateSetupNeeded, diagnostic: ""},
		{code: 2, state: StateOutdated, diagnostic: ""},
		{code: 3, state: StateCheckFailed, diagnostic: "remote unavailable"},
	} {
		stub := cmdexec.NewStubRunner()
		stub.AddResult("sh", cmdexec.Result{ExitCode: tt.code, Stderr: []byte("remote unavailable")})
		cfg := &config.Config{Version: 3, BackupRoot: t.TempDir()}
		plat := &platform.Platform{OS: platform.OSLinux}
		mgr := manager.New(cfg, plat).WithRunner(stub)
		entry := config.SubEntry{
			Name:      "binary",
			CheckMode: config.CheckModeStatus,
			Check:     map[string]string{"linux": "check"},
			Run:       map[string]string{"linux": "run"},
		}

		state, diagnostic := detectSetupPathState(entry, mgr)
		if state != tt.state || diagnostic != tt.diagnostic {
			t.Fatalf("code %d: got (%v, %q), want (%v, %q)", tt.code, state, diagnostic, tt.state, tt.diagnostic)
		}
	}
}

func TestStateCheckResult_RecheckClearsPreviousDiagnostic(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	stub.AddResult("sh", cmdexec.Result{ExitCode: 3, Stderr: []byte("remote unavailable")})
	stub.AddResult("sh", cmdexec.Result{ExitCode: 0})

	entry := setupSubEntry()
	entry.CheckMode = config.CheckModeStatus
	cfg := setupOnlyConfig(entry)
	plat := linuxPlatform()
	mgr := manager.New(cfg, plat).WithRunner(stub)
	m := NewModel(cfg, plat, false)
	m.Manager = mgr

	firstMsg := collectMsgs(m.subEntryStateCheckCmd(0, 0))
	if len(firstMsg) != 1 {
		t.Fatalf("first check produced %d messages, want 1", len(firstMsg))
	}
	updated, _ := m.Update(firstMsg[0])
	first := updated.(Model)
	if first.pendingStateChecks != 0 {
		t.Fatalf("pendingStateChecks after failed check = %d, want 0", first.pendingStateChecks)
	}
	if got := first.Applications[0].SubItems[0].CheckError; got != "remote unavailable" {
		t.Fatalf("failed check diagnostic = %q, want %q", got, "remote unavailable")
	}

	refreshCmd := first.refreshAllStates()
	if first.pendingStateChecks != 1 {
		t.Fatalf("pendingStateChecks during recheck = %d, want 1", first.pendingStateChecks)
	}
	if got := first.Applications[0].SubItems[0].CheckError; got != "" {
		t.Fatalf("loading recheck retained stale diagnostic %q", got)
	}

	secondMsg := collectMsgs(refreshCmd)
	if len(secondMsg) != 1 {
		t.Fatalf("successful recheck produced %d messages, want 1", len(secondMsg))
	}
	updated, _ = first.Update(secondMsg[0])
	final := updated.(Model)
	if final.pendingStateChecks != 0 {
		t.Fatalf("pendingStateChecks after successful recheck = %d, want 0", final.pendingStateChecks)
	}
	if got := final.Applications[0].SubItems[0].State; got != StateSetupOk {
		t.Fatalf("state after successful recheck = %v, want StateSetupOk", got)
	}
	if got := final.Applications[0].SubItems[0].CheckError; got != "" {
		t.Fatalf("successful recheck diagnostic = %q, want empty", got)
	}
}

func TestReinitPreservingState_PreservesUneditedSetupDiagnostic(t *testing.T) {
	failedEntry := setupSubEntry()
	failedEntry.Name = "failed-check"
	failedEntry.CheckMode = config.CheckModeStatus

	editedEntry := setupSubEntry()
	editedEntry.Name = "existing-check"
	editedEntry.CheckMode = config.CheckModeStatus

	const failedAppName = "failed-app"
	const editedAppName = "edited-app"
	cfg := &config.Config{
		Version:    3,
		BackupRoot: t.TempDir(),
		Applications: []config.Application{
			{Name: failedAppName, Entries: []config.SubEntry{failedEntry}},
			{Name: editedAppName, Entries: []config.SubEntry{editedEntry}},
		},
	}
	stub := cmdexec.NewStubRunner()
	stub.AddResult("sh", cmdexec.Result{ExitCode: 3, Stderr: []byte("remote unavailable")})
	stub.AddResult("sh", cmdexec.Result{ExitCode: 0})
	stub.AddResult("sh", cmdexec.Result{ExitCode: 0})
	stub.AddResult("sh", cmdexec.Result{ExitCode: 0})
	plat := linuxPlatform()
	mgr := manager.New(cfg, plat).WithRunner(stub)
	m := NewModel(cfg, plat, false)
	m.Manager = mgr

	applyCheck := func(appName string) {
		t.Helper()
		appIndex := -1
		for i, app := range m.Applications {
			if app.Application.Name == appName {
				appIndex = i
				break
			}
		}
		if appIndex == -1 {
			t.Fatalf("application %q not found", appName)
		}

		messages := collectMsgs(m.subEntryStateCheckCmd(appIndex, 0))
		if len(messages) != 1 {
			t.Fatalf("%s check produced %d messages, want 1", appName, len(messages))
		}
		updated, _ := m.Update(messages[0])
		next, ok := updated.(Model)
		if !ok {
			t.Fatalf("%s check returned model type %T, want Model", appName, updated)
		}
		m = next
	}

	applyCheck(failedAppName)
	applyCheck(editedAppName)

	failedAppIndex := -1
	editedAppIndex := -1
	for i, app := range m.Applications {
		switch app.Application.Name {
		case failedAppName:
			failedAppIndex = i
		case editedAppName:
			editedAppIndex = i
		}
	}
	if failedAppIndex == -1 || editedAppIndex == -1 {
		t.Fatalf("rebuilt model omitted test applications: failed=%d edited=%d", failedAppIndex, editedAppIndex)
	}
	if got := m.Applications[failedAppIndex].SubItems[0].CheckError; got != "remote unavailable" {
		t.Fatalf("initial failed diagnostic = %q, want %q", got, "remote unavailable")
	}
	if m.pendingStateChecks != 0 {
		t.Fatalf("pending checks before rebuild = %d, want 0", m.pendingStateChecks)
	}

	for i, app := range m.Config.Applications {
		if app.Name != editedAppName {
			continue
		}
		m.Config.Applications[i].Description = "unrelated edit"
		addedEntry := setupSubEntry()
		addedEntry.Name = "added-check"
		addedEntry.CheckMode = config.CheckModeStatus
		m.Config.Applications[i].Entries = append(m.Config.Applications[i].Entries, addedEntry)
	}
	m.reinitPreservingState(editedAppName)

	for _, app := range m.Applications {
		switch app.Application.Name {
		case failedAppName:
			if got := app.SubItems[0].State; got != StateCheckFailed {
				t.Fatalf("failed entry state after unrelated rebuild = %v, want StateCheckFailed", got)
			}
			if got := app.SubItems[0].CheckError; got != "remote unavailable" {
				t.Fatalf("failed entry diagnostic after unrelated rebuild = %q, want %q", got, "remote unavailable")
			}
		case editedAppName:
			if len(app.SubItems) != 2 {
				t.Fatalf("edited app entries after add = %d, want 2", len(app.SubItems))
			}
			for _, sub := range app.SubItems {
				if sub.State != StateLoading || sub.CheckError != "" {
					t.Fatalf("edited entry after rebuild = (%v, %q), want Loading with an empty diagnostic", sub.State, sub.CheckError)
				}
			}
		}
	}

	cmd := m.dispatchLoadingSubEntryStates()
	if m.pendingStateChecks != 2 {
		t.Fatalf("pending checks after edited-app rebuild = %d, want 2", m.pendingStateChecks)
	}
	for _, message := range collectMsgs(cmd) {
		updated, _ := m.Update(message)
		next, ok := updated.(Model)
		if !ok {
			t.Fatalf("recheck returned model type %T, want Model", updated)
		}
		m = next
	}
	if m.pendingStateChecks != 0 {
		t.Fatalf("pending checks after edited-app recheck = %d, want 0", m.pendingStateChecks)
	}

	for _, app := range m.Applications {
		switch app.Application.Name {
		case failedAppName:
			if got := app.SubItems[0].CheckError; got != "remote unavailable" {
				t.Fatalf("failed entry diagnostic after edited-app recheck = %q, want %q", got, "remote unavailable")
			}
		case editedAppName:
			for _, sub := range app.SubItems {
				if sub.State != StateSetupOk || sub.CheckError != "" {
					t.Fatalf("edited entry after successful recheck = (%v, %q), want SetupOk with an empty diagnostic", sub.State, sub.CheckError)
				}
			}
		}
	}
}

// TestDetectSubEntryState_SetupEntry_NeverShellsOut is the regression guard
// for the Task 5 bug: detectSubEntryState is a Model method that runs
// synchronously on the bubbletea UI goroutine (called from
// refreshApplicationStates and reinitPreservingState). Resolving a setup
// entry's state requires running its check command as a real subprocess
// (detectSetupPathState -> manager.CheckSetup -> runner.RunIn), and doing
// that here would visibly stall the UI. The sync path must therefore defer to
// StateLoading without touching the runner at all — the strongest assertion
// of that is that the stub recorded zero calls.
func TestDetectSubEntryState_SetupEntry_NeverShellsOut(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	// Queue a result so that if the sync path incorrectly ran the check, the
	// state returned would differ from StateLoading too — belt and braces on
	// top of the zero-Calls assertion below.
	stub.AddResult("sh", cmdexec.Result{ExitCode: 0})

	m := &Model{
		Config:   &config.Config{Version: 3, BackupRoot: "/repo"},
		Platform: &platform.Platform{OS: platform.OSLinux, EnvVars: map[string]string{}},
		Manager:  newStubManager(stub),
	}
	item := &SubEntryItem{AppName: "vicinae", SubEntry: setupSubEntry()}

	got := m.detectSubEntryState(item)

	if got != StateLoading {
		t.Errorf("detectSubEntryState for a setup entry = %v, want StateLoading (must defer to async resolution, not resolve inline)", got)
	}
	if len(stub.Calls) != 0 {
		t.Errorf("detectSubEntryState executed %d command(s) on the UI goroutine, want 0: %+v", len(stub.Calls), stub.Calls)
	}
}

// TestDetectSubEntryState_SetupEntry_NilManager_ReturnsStateLoading confirms
// the sync path defers to StateLoading for setup entries even when there is
// no Manager at all (e.g. immediately after adding a brand new app), rather
// than reporting a falsely-resolved state.
func TestDetectSubEntryState_SetupEntry_NilManager_ReturnsStateLoading(t *testing.T) {
	m := &Model{
		Config:   &config.Config{Version: 3, BackupRoot: "/repo"},
		Platform: &platform.Platform{OS: platform.OSLinux, EnvVars: map[string]string{}},
	}
	item := &SubEntryItem{AppName: "vicinae", SubEntry: setupSubEntry()}

	got := m.detectSubEntryState(item)
	if got != StateLoading {
		t.Errorf("detectSubEntryState for a setup entry with nil Manager = %v, want StateLoading", got)
	}
}

// TestDetectSubEntryStateStatic_SetupEntry_RoutesThroughSetupBranch mirrors the
// Model-method test above for the goroutine-safe static variant used by the
// async detection pipeline.
func TestDetectSubEntryStateStatic_SetupEntry_RoutesThroughSetupBranch(t *testing.T) {
	plat := &platform.Platform{OS: platform.OSLinux, EnvVars: map[string]string{}}
	cfg := &config.Config{Version: 3, BackupRoot: "/repo"}
	item := SubEntryItem{AppName: "vicinae", SubEntry: setupSubEntry()}

	got, diagnostic := detectSubEntryStateStatic(item, plat, cfg, nil)
	if got != StateSetupOk || diagnostic != "" {
		t.Errorf("detectSubEntryStateStatic for a setup entry with nil manager = (%v, %q), want (StateSetupOk, \"\")", got, diagnostic)
	}
}

func TestDetectSubEntryStateStatic_SetupEntryCheckPasses_ReturnsSetupOk(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	stub.AddResult("sh", cmdexec.Result{ExitCode: 0})

	plat := &platform.Platform{OS: platform.OSLinux, EnvVars: map[string]string{}}
	cfg := &config.Config{Version: 3, BackupRoot: "/repo"}
	item := SubEntryItem{AppName: "vicinae", SubEntry: setupSubEntry()}

	got, diagnostic := detectSubEntryStateStatic(item, plat, cfg, newStubManager(stub))
	if got != StateSetupOk || diagnostic != "" {
		t.Errorf("detectSubEntryStateStatic for a passing setup entry = (%v, %q), want (StateSetupOk, \"\")", got, diagnostic)
	}
}

func TestDetectSubEntryState_SelectedTemplateStatesSyncAndAsync(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, templatePath string)
		want   PathState
	}{
		{name: "linked", want: StateLinked},
		{
			name: "outdated",
			mutate: func(t *testing.T, templatePath string) {
				writeTUITemplateFile(t, templatePath, "source-v2")
			},
			want: StateOutdated,
		},
		{
			name: "modified",
			mutate: func(t *testing.T, templatePath string) {
				writeTUITemplateFile(t, tmpl.RenderedPath(templatePath), "user-edit")
			},
			want: StateModified,
		},
		{
			name: "missing-rendered",
			mutate: func(t *testing.T, templatePath string) {
				if err := os.Remove(tmpl.RenderedPath(templatePath)); err != nil {
					t.Fatal(err)
				}
			},
			want: StateOutdated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newTUISelectedTemplateFixture(t)
			if tt.mutate != nil {
				tt.mutate(t, fixture.templatePath)
			}

			gotSync := fixture.model.detectSubEntryState(&fixture.item)
			if gotSync != tt.want {
				t.Fatalf("sync state = %v, want %v", gotSync, tt.want)
			}

			gotAsync, diagnostic := detectSubEntryStateStatic(fixture.item, fixture.platform, fixture.config, fixture.manager)
			if gotAsync != tt.want || diagnostic != "" {
				t.Errorf("async state = (%v, %q), want (%v, \"\")", gotAsync, diagnostic, tt.want)
			}
		})
	}
}

func TestDetectSubEntryState_SelectedTemplateIgnoresUnselectedNeighborSyncAndAsync(t *testing.T) {
	fixture := newTUISelectedTemplateFixture(t)
	neighborPath := filepath.Join(filepath.Dir(fixture.templatePath), "neighbor.tmpl")
	writeTUITemplateFile(t, neighborPath, "neighbor-v1")

	allTemplatesEntry := fixture.item.SubEntry
	allTemplatesEntry.Files = []string{"config.tmpl", "neighbor.tmpl"}
	if err := fixture.manager.RestoreFiles(allTemplatesEntry, filepath.Dir(fixture.templatePath), fixture.item.Target); err != nil {
		t.Fatalf("seed neighboring template state: %v", err)
	}
	writeTUITemplateFile(t, neighborPath, "neighbor-v2")
	writeTUITemplateFile(t, tmpl.RenderedPath(neighborPath), "neighbor-user-edit")

	if got := fixture.model.detectSubEntryState(&fixture.item); got != StateLinked {
		t.Fatalf("sync selected state = %v, want StateLinked", got)
	}
	if got, diagnostic := detectSubEntryStateStatic(fixture.item, fixture.platform, fixture.config, fixture.manager); got != StateLinked || diagnostic != "" {
		t.Fatalf("async selected state = (%v, %q), want (StateLinked, \"\")", got, diagnostic)
	}
}

func TestDetectSubEntryState_CopyTemplateUsesLiveTargetStatus(t *testing.T) {
	fixture := newTUICopyTemplateFixture(t)

	if got := fixture.model.detectSubEntryState(&fixture.item); got != StateLinked {
		t.Fatalf("sync copy state = %v, want StateLinked", got)
	}
	if got, diagnostic := detectSubEntryStateStatic(fixture.item, fixture.platform, fixture.config, fixture.manager); got != StateLinked || diagnostic != "" {
		t.Fatalf("async copy state = (%v, %q), want (StateLinked, \"\")", got, diagnostic)
	}
}

func TestDetectSubEntryState_CopyTemplateSourceDriftIsOutdated(t *testing.T) {
	fixture := newTUICopyTemplateFixture(t)
	writeTUITemplateFile(t, fixture.templatePath, "changed {{ .Hostname }}")

	if got := fixture.model.detectSubEntryState(&fixture.item); got != StateOutdated {
		t.Fatalf("sync copy state = %v, want StateOutdated", got)
	}
	if got, diagnostic := detectSubEntryStateStatic(fixture.item, fixture.platform, fixture.config, fixture.manager); got != StateOutdated || diagnostic != "" {
		t.Fatalf("async copy state = %v, want StateOutdated", got)
	}
}

func TestDetectSubEntryState_CopyTemplateTargetEditIsModified(t *testing.T) {
	fixture := newTUICopyTemplateFixture(t)
	writeTUITemplateFile(t, filepath.Join(fixture.item.Target, "config"), "user edit")

	if got := fixture.model.detectSubEntryState(&fixture.item); got != StateModified {
		t.Fatalf("sync copy state = %v, want StateModified", got)
	}
	if got, diagnostic := detectSubEntryStateStatic(fixture.item, fixture.platform, fixture.config, fixture.manager); got != StateModified || diagnostic != "" {
		t.Fatalf("async copy state = %v, want StateModified", got)
	}
}

func TestDetectSubEntryState_CopyTemplateWithoutManagerIsUnavailable(t *testing.T) {
	fixture := newTUICopyTemplateFixture(t)
	fixture.model.Manager = nil

	if got := fixture.model.detectSubEntryState(&fixture.item); got != StateUnavailable {
		t.Fatalf("sync copy state = %v, want StateUnavailable", got)
	}
	if got, diagnostic := detectSubEntryStateStatic(fixture.item, fixture.platform, fixture.config, nil); got != StateUnavailable || diagnostic != "" {
		t.Fatalf("async copy state = %v, want StateUnavailable", got)
	}
}

func TestDetectSubEntryState_CopyTemplateDeniedReadIsUnavailableWithoutSudo(t *testing.T) {
	fixture := newTUICopyTemplateFixture(t)
	fixture.item.SubEntry.Sudo = true
	stub := cmdexec.NewStubRunner()
	denied := fixture.manager.WithFS(deniedTUIReadFS{
		FS:     fsys.OsFS{},
		denied: filepath.Join(fixture.item.Target, "config"),
	}).WithRunner(stub)
	fixture.model.Manager = denied

	if got := fixture.model.detectSubEntryState(&fixture.item); got != StateUnavailable {
		t.Fatalf("sync copy state = %v, want StateUnavailable", got)
	}
	if got, diagnostic := detectSubEntryStateStatic(fixture.item, fixture.platform, fixture.config, denied); got != StateUnavailable || diagnostic != "" {
		t.Fatalf("async copy state = %v, want StateUnavailable", got)
	}
	if len(stub.Calls) != 0 {
		t.Fatalf("status inspection spawned privileged commands: %+v", stub.Calls)
	}
}

func TestCopyTemplateStateMatrixMatchesSyncStaticAndStatus(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("copy state matrix requires symlinks")
	}
	tests := []struct {
		name       string
		mutate     func(t *testing.T, fixture *tuiTemplateFixture) *manager.Manager
		want       PathState
		wantAction bool
	}{
		{
			name: "healthy",
			mutate: func(_ *testing.T, fixture *tuiTemplateFixture) *manager.Manager {
				return fixture.manager
			},
			want: StateLinked,
		},
		{
			name: "missing target",
			mutate: func(t *testing.T, fixture *tuiTemplateFixture) *manager.Manager {
				if err := os.Remove(filepath.Join(fixture.item.Target, "config")); err != nil {
					t.Fatal(err)
				}
				return fixture.manager
			},
			want:       StateReady,
			wantAction: true,
		},
		{
			name: "wrong link",
			mutate: func(t *testing.T, fixture *tuiTemplateFixture) *manager.Manager {
				destination := filepath.Join(fixture.item.Target, "config")
				if err := os.Remove(destination); err != nil {
					t.Fatal(err)
				}
				outside := filepath.Join(t.TempDir(), "wrong-target")
				writeTUITemplateFile(t, outside, "wrong")
				if err := os.Symlink(outside, destination); err != nil {
					t.Fatal(err)
				}
				return fixture.manager
			},
			want:       StateReady,
			wantAction: true,
		},
		{
			name: "missing history",
			mutate: func(_ *testing.T, fixture *tuiTemplateFixture) *manager.Manager {
				withoutHistory := manager.New(fixture.config, fixture.platform)
				fixture.model.Manager = withoutHistory
				return withoutHistory
			},
			want:       StateOutdated,
			wantAction: true,
		},
		{
			name: "unsafe selection",
			mutate: func(_ *testing.T, fixture *tuiTemplateFixture) *manager.Manager {
				fixture.item.SubEntry.Files = []string{"config.tmpl", "config"}
				return fixture.manager
			},
			want:       StateUnavailable,
			wantAction: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fixture := newTUICopyTemplateFixture(t)
			inspectionManager := tt.mutate(t, &fixture)
			fixture.config.Applications = []config.Application{{
				Name:    "tool",
				Entries: []config.SubEntry{fixture.item.SubEntry},
			}}

			if got := fixture.model.detectSubEntryState(&fixture.item); got != tt.want {
				t.Fatalf("sync state = %v, want %v", got, tt.want)
			}
			if got, diagnostic := detectSubEntryStateStatic(fixture.item, fixture.platform, fixture.config, inspectionManager); got != tt.want || diagnostic != "" {
				t.Fatalf("static state = %v, want %v", got, tt.want)
			}

			report, err := ComputeStatus(fixture.config, fixture.platform, inspectionManager, tt.wantAction)
			if err != nil {
				t.Fatalf("ComputeStatus: %v", err)
			}
			if len(report.Applications) != 1 || len(report.Applications[0].Entries) != 1 {
				t.Fatalf("status report = %+v, want one actionable entry", report)
			}
			entry := report.Applications[0].Entries[0]
			if entry.State != tt.want.String() || entry.Actionable != tt.wantAction {
				t.Fatalf("status entry = %+v, want state %q/actionable %v", entry, tt.want.String(), tt.wantAction)
			}
			if tt.want.Actionable() && !report.Actionable {
				t.Fatalf("status report = %+v, want actionable report", report)
			}
		})
	}
}

type deniedTUIReadFS struct {
	fsys.FS
	denied string
}

func (f deniedTUIReadFS) ReadFile(name string) ([]byte, error) {
	if name == f.denied {
		return nil, &fs.PathError{Op: "read", Path: name, Err: fs.ErrPermission}
	}
	return f.FS.ReadFile(name)
}

type tuiTemplateFixture struct {
	model        *Model
	item         SubEntryItem
	manager      *manager.Manager
	config       *config.Config
	platform     *platform.Platform
	templatePath string
}

func newTUISelectedTemplateFixture(t *testing.T) tuiTemplateFixture {
	t.Helper()
	root := t.TempDir()
	backupPath := filepath.Join(root, "backup")
	targetPath := filepath.Join(root, "target")
	if err := os.MkdirAll(backupPath, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(targetPath, 0o750); err != nil {
		t.Fatal(err)
	}

	plat := &platform.Platform{OS: platform.OSLinux, EnvVars: map[string]string{}}
	cfg := &config.Config{Version: 3, BackupRoot: root}
	mgr := manager.New(cfg, plat)
	if err := mgr.InitStateStore(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mgr.Close() }) //nolint:errcheck // test cleanup

	templatePath := filepath.Join(backupPath, "config.tmpl")
	writeTUITemplateFile(t, templatePath, "source-v1")
	entry := config.SubEntry{
		Name:    "config",
		Backup:  backupPath,
		Files:   []string{"config.tmpl"},
		Targets: map[string]string{"linux": targetPath},
	}
	if err := mgr.RestoreFiles(entry, backupPath, targetPath); err != nil {
		t.Fatal(err)
	}

	item := SubEntryItem{AppName: "tool", Target: targetPath, SubEntry: entry}
	model := &Model{Config: cfg, Platform: plat, Manager: mgr}
	return tuiTemplateFixture{
		model:        model,
		item:         item,
		manager:      mgr,
		config:       cfg,
		platform:     plat,
		templatePath: templatePath,
	}
}

func newTUICopyTemplateFixture(t *testing.T) tuiTemplateFixture {
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

	plat := &platform.Platform{OS: platform.OSLinux, EnvVars: map[string]string{}}
	cfg := &config.Config{Version: 3, BackupRoot: root}
	mgr := manager.New(cfg, plat)
	if err := mgr.InitStateStore(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = mgr.Close() }) //nolint:errcheck // test cleanup

	templatePath := filepath.Join(backupPath, "config.tmpl")
	writeTUITemplateFile(t, templatePath, "literal {{ .Hostname }}")
	entry := config.SubEntry{
		Name:    "config",
		Backup:  backupPath,
		Files:   []string{"config.tmpl"},
		Method:  config.MethodCopy,
		Targets: map[string]string{"linux": targetPath},
	}
	if err := mgr.RestoreFiles(entry, backupPath, targetPath); err != nil {
		t.Fatal(err)
	}

	item := SubEntryItem{AppName: "tool", Target: targetPath, SubEntry: entry}
	model := &Model{Config: cfg, Platform: plat, Manager: mgr}
	return tuiTemplateFixture{
		model:        model,
		item:         item,
		manager:      mgr,
		config:       cfg,
		platform:     plat,
		templatePath: templatePath,
	}
}

func writeTUITemplateFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// otherSetupSubEntry returns a second, distinct setup sub-entry so tests can
// tell two dispatched checks apart by their resolved state.
func otherSetupSubEntry() config.SubEntry {
	return config.SubEntry{
		Name:  "enable-other",
		Check: map[string]string{"linux": "true"},
		Run:   map[string]string{"linux": "true"},
	}
}

// TestCheckLoadingSubEntryStatesCmd_ResolvesOnlyLoadingEntries is the
// regression guard for the async resolver: it must dispatch exactly one
// check per sub-entry still at StateLoading in an operational app, and must leave
// already-resolved sub-entries untouched.
func TestCheckLoadingSubEntryStatesCmd_ResolvesOnlyLoadingEntries(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	stub.AddResult("sh", cmdexec.Result{ExitCode: 1}) // other/enable-other check fails

	m := Model{
		Config:   &config.Config{Version: 3, BackupRoot: "/repo"},
		Platform: &platform.Platform{OS: platform.OSLinux, EnvVars: map[string]string{}},
		Manager:  newStubManager(stub),
		Applications: []ApplicationItem{
			{
				Application: config.Application{Name: "vicinae"},
				IsFiltered:  true, // deliberately hidden: must never be resolved
				SubItems: []SubEntryItem{
					{AppName: "vicinae", SubEntry: setupSubEntry(), State: StateLoading},
					{AppName: "vicinae", SubEntry: config.SubEntry{Name: "config-file", Backup: "cfg"}, State: StateLinked},
				},
			},
			{
				Application: config.Application{Name: "other"},
				SubItems: []SubEntryItem{
					{AppName: "other", SubEntry: otherSetupSubEntry(), State: StateLoading},
				},
			},
		},
	}

	cmd, count := m.checkLoadingSubEntryStatesCmd()
	if count != 1 {
		t.Fatalf("checkLoadingSubEntryStatesCmd() count = %d, want 1 (filtered apps are not operational)", count)
	}

	msgs := collectMsgs(cmd)
	if len(msgs) != 1 {
		t.Fatalf("dispatched %d message(s), want 1", len(msgs))
	}

	got := map[[2]int]PathState{}
	for _, msg := range msgs {
		res, ok := msg.(stateCheckResultMsg)
		if !ok {
			t.Fatalf("unexpected message type %T", msg)
		}
		got[[2]int{res.appIndex, res.subIndex}] = res.state
	}

	if state, resolved := got[[2]int{1, 0}]; !resolved || state != StateSetupNeeded {
		t.Errorf("other/enable-other resolved to (%v, resolved=%v), want (StateSetupNeeded, true)", state, resolved)
	}
	if _, resolved := got[[2]int{0, 1}]; resolved {
		t.Errorf("config-file sub-entry (not StateLoading) was unexpectedly resolved by the loading-only resolver")
	}
}

// TestCheckLoadingSubEntryStatesCmd_IgnoresUnresolvedConfigEntries guards the
// resolver's scope. StateLoading is also the zero value of PathState, so a
// config entry whose initial async check from Init() has not landed yet still
// reads as StateLoading. If a form is saved while that batch is in flight, a
// resolver keyed purely off StateLoading would re-dispatch those config
// entries and duplicate subprocess work already in flight. Only setup entries
// are genuinely parked at StateLoading by the synchronous rebuild
// (detectSubEntryState refuses to run their check on the UI goroutine), so
// only they belong here.
func TestCheckLoadingSubEntryStatesCmd_IgnoresUnresolvedConfigEntries(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	stub.AddResult("sh", cmdexec.Result{ExitCode: 0}) // vicinae/enable-service check passes

	m := Model{
		Config:   &config.Config{Version: 3, BackupRoot: "/repo"},
		Platform: &platform.Platform{OS: platform.OSLinux, EnvVars: map[string]string{}},
		Manager:  newStubManager(stub),
		Applications: []ApplicationItem{
			{
				Application: config.Application{Name: "vicinae"},
				SubItems: []SubEntryItem{
					{AppName: "vicinae", SubEntry: setupSubEntry(), State: StateLoading},
					// A config entry whose Init() check has not resolved yet.
					{AppName: "vicinae", SubEntry: config.SubEntry{Name: "config-file", Backup: "cfg"}, State: StateLoading},
				},
			},
		},
	}

	cmd, count := m.checkLoadingSubEntryStatesCmd()
	if count != 1 {
		t.Fatalf("checkLoadingSubEntryStatesCmd() count = %d, want 1 (the setup entry only; the unresolved "+
			"config entry already has a check in flight from Init())", count)
	}

	msgs := collectMsgs(cmd)
	if len(msgs) != 1 {
		t.Fatalf("dispatched %d message(s), want 1", len(msgs))
	}

	res, ok := msgs[0].(stateCheckResultMsg)
	if !ok {
		t.Fatalf("unexpected message type %T", msgs[0])
	}

	if res.subIndex != 0 {
		t.Errorf("resolved subIndex = %d, want 0 (the setup entry, not the config entry)", res.subIndex)
	}

	// Exactly one dispatch means exactly one stateCheckResultMsg, keeping the
	// pendingStateChecks counter balanced.
	if count != len(msgs) {
		t.Errorf("dispatched count (%d) != messages produced (%d); pendingStateChecks would drift", count, len(msgs))
	}
}

// TestCheckLoadingSubEntryStatesCmd_NoLoadingEntries_DispatchesNothing
// confirms the resolver is a no-op (and does not touch the runner) once every
// sub-entry has already been resolved.
func TestCheckLoadingSubEntryStatesCmd_NoLoadingEntries_DispatchesNothing(t *testing.T) {
	stub := cmdexec.NewStubRunner()

	m := Model{
		Config:   &config.Config{Version: 3, BackupRoot: "/repo"},
		Platform: &platform.Platform{OS: platform.OSLinux, EnvVars: map[string]string{}},
		Manager:  newStubManager(stub),
		Applications: []ApplicationItem{
			{
				Application: config.Application{Name: "vicinae"},
				SubItems: []SubEntryItem{
					{AppName: "vicinae", SubEntry: setupSubEntry(), State: StateSetupOk},
				},
			},
		},
	}

	cmd, count := m.checkLoadingSubEntryStatesCmd()
	if count != 0 {
		t.Errorf("checkLoadingSubEntryStatesCmd() count = %d, want 0", count)
	}
	if cmd != nil {
		t.Errorf("checkLoadingSubEntryStatesCmd() cmd = %v, want nil", cmd)
	}
	if len(stub.Calls) != 0 {
		t.Errorf("checkLoadingSubEntryStatesCmd executed %d command(s), want 0: %+v", len(stub.Calls), stub.Calls)
	}
}

// TestDispatchLoadingSubEntryStates_TracksPendingCount proves
// dispatchLoadingSubEntryStates dispatches the async resolver AND adds its
// count to pendingStateChecks (rather than overwriting it), matching the
// bookkeeping convention of dispatchFilteredStates / dispatchUncheckedPackageStates.
// A drift here would leave the TUI's spinner/rebuild logic waiting forever
// (or rebuilding early).
func TestDispatchLoadingSubEntryStates_TracksPendingCount(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	stub.AddResult("sh", cmdexec.Result{ExitCode: 0})

	m := &Model{
		Config:   &config.Config{Version: 3, BackupRoot: "/repo"},
		Platform: &platform.Platform{OS: platform.OSLinux, EnvVars: map[string]string{}},
		Manager:  newStubManager(stub),
		Applications: []ApplicationItem{
			{
				Application: config.Application{Name: "vicinae"},
				SubItems: []SubEntryItem{
					{AppName: "vicinae", SubEntry: setupSubEntry(), State: StateLoading},
				},
			},
		},
		pendingStateChecks: 3, // simulate other in-flight checks already tracked
	}

	cmd := m.dispatchLoadingSubEntryStates()
	if cmd == nil {
		t.Fatal("dispatchLoadingSubEntryStates() returned a nil cmd despite a StateLoading sub-entry")
	}
	if m.pendingStateChecks != 4 {
		t.Errorf("pendingStateChecks = %d, want 4 (3 pre-existing + 1 dispatched)", m.pendingStateChecks)
	}

	msgs := collectMsgs(cmd)
	if len(msgs) != 1 {
		t.Fatalf("dispatched %d message(s), want 1", len(msgs))
	}
	res, ok := msgs[0].(stateCheckResultMsg)
	if !ok {
		t.Fatalf("unexpected message type %T", msgs[0])
	}
	if res.state != StateSetupOk {
		t.Errorf("resolved state = %v, want StateSetupOk", res.state)
	}
}
