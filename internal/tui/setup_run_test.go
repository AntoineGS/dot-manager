package tui

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/manager"
	"github.com/AntoineGS/tidydots/internal/platform"
)

// linuxPlatform is the platform used across the setup-entry TUI tests.
func linuxPlatform() *platform.Platform {
	return &platform.Platform{OS: platform.OSLinux, EnvVars: map[string]string{}}
}

// configSubEntry returns a config sub-entry (has a Backup and a target).
func configSubEntry() config.SubEntry {
	return config.SubEntry{
		Name:    "config-file",
		Backup:  "vicinae",
		Targets: map[string]string{"linux": "~/.config/vicinae"},
	}
}

// windowsOnlySetupSubEntry returns a setup entry that does not apply to Linux.
func windowsOnlySetupSubEntry() config.SubEntry {
	return config.SubEntry{
		Name:  "windows-only",
		Check: map[string]string{"windows": "check"},
		Run:   map[string]string{"windows": "run"},
	}
}

// setupOnlyConfig builds a config whose single application holds the given entries.
func setupOnlyConfig(entries ...config.SubEntry) *config.Config {
	return &config.Config{
		Version:    3,
		BackupRoot: "/repo",
		Applications: []config.Application{
			{Name: "vicinae", Entries: entries},
		},
	}
}

// TestInitApplicationItems_IncludesSetupEntries is the regression guard for the
// bug that made the whole setup feature invisible in the TUI: setup entries
// declare no targets (they are actions, not file deployments), and the list
// builder skipped every sub-entry whose target was empty. The rows the TUI is
// supposed to flag as "Needs setup" never existed.
func TestInitApplicationItems_IncludesSetupEntries(t *testing.T) {
	cfg := setupOnlyConfig(configSubEntry(), setupSubEntry())

	m := NewModel(cfg, linuxPlatform(), false)

	if len(m.Applications) != 1 {
		t.Fatalf("Applications = %d, want 1", len(m.Applications))
	}

	subs := m.Applications[0].SubItems
	if len(subs) != 2 {
		t.Fatalf("SubItems = %d, want 2 (the config entry and the setup entry)", len(subs))
	}

	if !subs[1].SubEntry.IsSetup() {
		t.Fatalf("second sub-item is %q, want the setup entry", subs[1].SubEntry.Name)
	}

	if subs[1].Target != "" {
		t.Errorf("setup entry Target = %q, want \"\" (a setup entry deploys no files)", subs[1].Target)
	}

	if subs[1].State != StateLoading {
		t.Errorf("setup entry initial State = %v, want StateLoading (resolved asynchronously by its check)", subs[1].State)
	}
}

// TestInitApplicationItems_SetupOnlyApp_IsNotDropped covers an application that
// holds nothing but setup entries and has no package: it must still be listed.
func TestInitApplicationItems_SetupOnlyApp_IsNotDropped(t *testing.T) {
	m := NewModel(setupOnlyConfig(setupSubEntry()), linuxPlatform(), false)

	if len(m.Applications) != 1 {
		t.Fatalf("Applications = %d, want 1 (an app with only setup entries is still an app)", len(m.Applications))
	}

	if len(m.Applications[0].SubItems) != 1 {
		t.Fatalf("SubItems = %d, want 1", len(m.Applications[0].SubItems))
	}
}

// TestInitApplicationItems_SkipsSetupEntryForOtherOS mirrors runSetupEntry,
// which skips an entry with no run command for the current OS. Listing such an
// entry would report it as "Set up" (its check is absent, so CheckSetup
// reports nothing outstanding), which is a claim about a machine it never
// applied to.
func TestInitApplicationItems_SkipsSetupEntryForOtherOS(t *testing.T) {
	m := NewModel(setupOnlyConfig(configSubEntry(), windowsOnlySetupSubEntry()), linuxPlatform(), false)

	if len(m.Applications) != 1 {
		t.Fatalf("Applications = %d, want 1", len(m.Applications))
	}

	subs := m.Applications[0].SubItems
	if len(subs) != 1 {
		t.Fatalf("SubItems = %d, want 1 (the Windows-only setup entry must not be listed on Linux)", len(subs))
	}

	if subs[0].SubEntry.Name != "config-file" {
		t.Errorf("listed sub-entry = %q, want the config entry", subs[0].SubEntry.Name)
	}
}

// TestCountInitialStateChecks_CountsSetupEntries keeps the pre-computed
// pendingStateChecks in step with what Init() actually dispatches: the counter
// mirrors checkSubEntryStatesCmd, which now sees setup entries too. A drift
// here leaves the table rebuilding early or never.
func TestCountInitialStateChecks_CountsSetupEntries(t *testing.T) {
	m := NewModel(setupOnlyConfig(configSubEntry(), setupSubEntry()), linuxPlatform(), false)

	_, dispatched := m.checkSubEntryStatesCmd()
	if got := m.countInitialStateChecks(); got != dispatched {
		t.Errorf("countInitialStateChecks() = %d, but checkSubEntryStatesCmd dispatches %d", got, dispatched)
	}

	if dispatched != 2 {
		t.Errorf("checkSubEntryStatesCmd dispatched %d check(s), want 2 (config entry + setup entry)", dispatched)
	}
}

// TestGetTypeInfo_SetupEntry_ReportsSetup covers the info column: a setup entry
// has no Files and no Backup, so the file-count logic described it as "0 files".
func TestGetTypeInfo_SetupEntry_ReportsSetup(t *testing.T) {
	got := getTypeInfo(SubEntryItem{AppName: "vicinae", SubEntry: setupSubEntry()})
	if got != TypeSetup {
		t.Errorf("getTypeInfo(setup entry) = %q, want %q", got, TypeSetup)
	}
}

// newSetupModel builds a Model whose single application holds one config entry
// and one setup entry, backed by the given stub runner.
func newSetupModel(t *testing.T, stub *cmdexec.StubRunner, dryRun bool) *Model {
	t.Helper()

	cfg := setupOnlyConfig(configSubEntry(), setupSubEntry())
	mgr := manager.New(cfg, linuxPlatform()).WithRunner(stub)
	mgr.DryRun = dryRun

	m := NewModel(cfg, linuxPlatform(), dryRun)
	m.Manager = mgr

	return &m
}

// setupItemOf returns the queued setup item for the model's setup sub-entry.
func setupItemOf(m *Model) setupRunItem {
	sub := m.Applications[0].SubItems[1]
	return setupRunItem{appIdx: 0, subIdx: 1, name: sub.SubEntry.Name, sub: sub}
}

// TestPerformRestoreSubEntry_SetupEntry_RunsIt is the core of the fix: a setup
// entry used to be rejected with "Not a config entry" by the very action the
// TUI told the user to take on it. It must now run: check, then the setup
// command because the check failed, then the re-check.
func TestPerformRestoreSubEntry_SetupEntry_RunsIt(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	stub.AddResult("sh", cmdexec.Result{ExitCode: 1}) // check fails
	stub.AddResult("sh", cmdexec.Result{ExitCode: 0}) // run succeeds
	stub.AddResult("sh", cmdexec.Result{ExitCode: 0}) // re-check passes

	m := newSetupModel(t, stub, false)

	success, message := m.performRestoreSubEntry(m.Applications[0].SubItems[1])

	if !success {
		t.Fatalf("performRestoreSubEntry on a setup entry failed: %s", message)
	}

	if strings.Contains(message, "Not a config entry") {
		t.Errorf("setup entry was still rejected as a non-config entry: %q", message)
	}

	if len(stub.Calls) != 3 {
		t.Fatalf("executed %d command(s), want 3 (check, run, re-check): %+v", len(stub.Calls), stub.Calls)
	}

	if !strings.Contains(stub.Calls[1].Args[1], "enable --now") {
		t.Errorf("second command should be the setup command, got %q", stub.Calls[1].Args[1])
	}
}

// TestPerformRestoreSubEntry_SetupEntry_SurfacesFailure proves a failing setup
// command is reported with its cause rather than swallowed.
func TestPerformRestoreSubEntry_SetupEntry_SurfacesFailure(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	stub.AddResult("sh", cmdexec.Result{ExitCode: 1})                                      // check fails
	stub.AddResult("sh", cmdexec.Result{ExitCode: 2, Stderr: []byte("permission denied")}) // run fails

	m := newSetupModel(t, stub, false)

	success, message := m.performRestoreSubEntry(m.Applications[0].SubItems[1])

	if success {
		t.Fatal("performRestoreSubEntry reported success for a failing setup command")
	}

	if !strings.Contains(message, "permission denied") {
		t.Errorf("message must surface the cause, got %q", message)
	}
}

// TestPerformRestoreSubEntry_SetupEntry_DryRun_NeverRuns keeps the dry-run
// guarantee across the TUI path: the check still runs, the setup command never
// does.
func TestPerformRestoreSubEntry_SetupEntry_DryRun_NeverRuns(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	stub.AddResult("sh", cmdexec.Result{ExitCode: 1}) // check fails

	m := newSetupModel(t, stub, true)

	success, message := m.performRestoreSubEntry(m.Applications[0].SubItems[1])

	if !success {
		t.Fatalf("dry run reported failure: %s", message)
	}

	if len(stub.Calls) != 1 {
		t.Fatalf("dry run executed %d command(s), want 1 (the check only): %+v", len(stub.Calls), stub.Calls)
	}

	if !strings.Contains(message, "DRY RUN") {
		t.Errorf("dry-run message must say so, got %q", message)
	}
}

// TestPerformRestoreSubEntry_NoManager_Fails covers the guard for a model with
// no manager (a brand new app in a test harness): it must report, not panic.
func TestPerformRestoreSubEntry_NoManager_Fails(t *testing.T) {
	m := NewModel(setupOnlyConfig(setupSubEntry()), linuxPlatform(), false)

	success, message := m.performRestoreSubEntry(m.Applications[0].SubItems[0])
	if success {
		t.Fatal("a setup entry cannot succeed without a manager to run it")
	}

	if message == "" {
		t.Error("failure message must explain why")
	}
}

// TestRunNextSetup_HandsTheTerminalToBubbletea is the terminal-ownership
// guarantee. A setup command may prompt for a sudo password while bubbletea
// holds the terminal in raw mode with its own input reader; the password would
// never reach sudo. Package installs already solve this with tea.Exec
// (installNextPackage), which releases the terminal for the duration of the
// command and restores it afterwards. Setup entries must go through the same
// door — the message tea.Exec produces is tea.execMsg.
func TestRunNextSetup_HandsTheTerminalToBubbletea(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	m := newSetupModel(t, stub, false)

	cmd := m.startSetupRun([]setupRunItem{setupItemOf(m)}, false)
	if cmd == nil {
		t.Fatal("startSetupRun returned no command for a queued setup entry")
	}

	if got := fmt.Sprintf("%T", cmd()); got != "tea.execMsg" {
		t.Errorf("setup entries are dispatched as %s, want tea.execMsg — they must go through "+
			"tea.Exec so the terminal is released for a sudo prompt", got)
	}

	// tea.Exec only *wraps* the work; nothing may run until bubbletea hands the
	// terminal over and calls Run().
	if len(stub.Calls) != 0 {
		t.Errorf("dispatching the setup ran %d command(s) before the terminal was released: %+v",
			len(stub.Calls), stub.Calls)
	}
}

// TestSetupExec_Run_ExecutesTheEntry proves the ExecCommand bubbletea invokes
// (once it owns nothing but the terminal handover) actually runs the entry.
func TestSetupExec_Run_ExecutesTheEntry(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	stub.AddResult("sh", cmdexec.Result{ExitCode: 1}) // check fails
	stub.AddResult("sh", cmdexec.Result{ExitCode: 0}) // run succeeds
	stub.AddResult("sh", cmdexec.Result{ExitCode: 0}) // re-check passes

	m := newSetupModel(t, stub, false)

	var out bytes.Buffer
	ex := &setupExec{model: *m, item: setupItemOf(m)}
	ex.SetStdin(strings.NewReader(""))
	ex.SetStdout(&out)
	ex.SetStderr(&out)

	if err := ex.Run(); err != nil {
		t.Fatalf("setupExec.Run() = %v, want nil", err)
	}

	if !ex.success {
		t.Errorf("setupExec recorded success = false, message = %q", ex.message)
	}

	if len(stub.Calls) != 3 {
		t.Fatalf("executed %d command(s), want 3 (check, run, re-check)", len(stub.Calls))
	}

	// The handed-over terminal should say what is running, so a sudo prompt is
	// not preceded by an unexplained blank screen.
	if !strings.Contains(out.String(), "enable-service") {
		t.Errorf("nothing announced the running setup on the terminal, got %q", out.String())
	}
}

// TestSetupExec_Run_ReportsFailureAsError proves a failed setup surfaces to
// bubbletea as an error, which is what tea.Exec's callback keys off.
func TestSetupExec_Run_ReportsFailureAsError(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	stub.AddResult("sh", cmdexec.Result{ExitCode: 1})                              // check fails
	stub.AddResult("sh", cmdexec.Result{ExitCode: 2, Stderr: []byte("boom boom")}) // run fails

	m := newSetupModel(t, stub, false)

	ex := &setupExec{model: *m, item: setupItemOf(m)}

	err := ex.Run()
	if err == nil {
		t.Fatal("setupExec.Run() = nil, want an error for a failing setup command")
	}

	if ex.success {
		t.Error("setupExec recorded success for a failing setup command")
	}

	if !strings.Contains(ex.message, "boom boom") {
		t.Errorf("recorded message must surface the cause, got %q", ex.message)
	}
}

// TestHandleSetupRunResult_ReRunsTheCheckInsteadOfAssumingSuccess is the
// state-refresh guarantee. A successful run does not license the TUI to claim
// StateSetupOk: the check is the source of truth. The row is parked at
// StateLoading and re-resolved through the existing async pipeline.
func TestHandleSetupRunResult_ReRunsTheCheckInsteadOfAssumingSuccess(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	stub.AddResult("sh", cmdexec.Result{ExitCode: 0}) // the re-resolution check passes

	m := newSetupModel(t, stub, false)
	m.Applications[0].SubItems[1].State = StateSetupNeeded
	m.pendingSetups = []setupRunItem{setupItemOf(m)}
	m.currentSetupIndex = 0

	item := setupItemOf(m)
	pendingBefore := m.pendingStateChecks

	updated, cmd := m.handleSetupRunResult(setupRunMsg{item: item, success: true, message: "Setup complete"})

	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("unexpected model type %T", updated)
	}

	if got.Applications[0].SubItems[1].State != StateLoading {
		t.Errorf("state after a successful run = %v, want StateLoading (the check re-resolves it; success is not assumed)",
			got.Applications[0].SubItems[1].State)
	}

	// Exactly one re-check is dispatched, and it will deliver exactly one
	// stateCheckResultMsg — so the pendingStateChecks counter stays balanced.
	if got.pendingStateChecks != pendingBefore+1 {
		t.Errorf("pendingStateChecks = %d, want %d (one dispatched re-check)", got.pendingStateChecks, pendingBefore+1)
	}

	if len(got.results) != 1 || !got.results[0].Success {
		t.Fatalf("results = %+v, want one successful entry", got.results)
	}

	if !got.showingResults {
		t.Error("the results popup must open so the user sees the outcome")
	}

	msgs := collectMsgs(cmd)
	if len(msgs) != 1 {
		t.Fatalf("dispatched %d message(s), want 1 (the re-check)", len(msgs))
	}

	res, ok := msgs[0].(stateCheckResultMsg)
	if !ok {
		t.Fatalf("unexpected message type %T, want stateCheckResultMsg", msgs[0])
	}

	if res.state != StateSetupOk {
		t.Errorf("re-check resolved to %v, want StateSetupOk", res.state)
	}
}

// TestHandleSetupRunResult_RunsQueuedEntriesOneAtATime proves the queue is
// sequential: two setup entries never contend for the terminal at once.
func TestHandleSetupRunResult_RunsQueuedEntriesOneAtATime(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	m := newSetupModel(t, stub, false)

	first := setupItemOf(m)
	second := setupItemOf(m)
	m.pendingSetups = []setupRunItem{first, second}
	m.currentSetupIndex = 0

	updated, cmd := m.handleSetupRunResult(setupRunMsg{item: first, success: true, message: "Setup complete"})

	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("unexpected model type %T", updated)
	}

	if got.currentSetupIndex != 1 {
		t.Errorf("currentSetupIndex = %d, want 1", got.currentSetupIndex)
	}

	if got.showingResults {
		t.Error("results popup opened while a setup entry is still queued")
	}

	if cmd == nil {
		t.Fatal("no command dispatched for the second queued setup entry")
	}

	if msgType := fmt.Sprintf("%T", cmd()); msgType != "tea.execMsg" {
		t.Errorf("second setup dispatched as %s, want tea.execMsg", msgType)
	}
}

// TestExecuteBatchRestore_DoesNotFailSetupEntries is the batch half of the bug:
// executeBatchRestore called performRestoreSubEntry on every selected item with
// no filter, so every setup entry became a spurious "Not a config entry"
// failure and inflated failCount. Setup entries must instead be handed to the
// tea.Exec queue.
func TestExecuteBatchRestore_DoesNotFailSetupEntries(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	m := newSetupModel(t, stub, false)
	m.selectedSubEntries = map[subEntryKey]bool{{app: "vicinae", sub: "enable-service"}: true}
	m.multiSelectActive = true

	cmd := m.executeBatchRestore(false)
	if cmd == nil {
		t.Fatal("executeBatchRestore returned no command")
	}

	msg, ok := cmd().(batchRestoreConfigsDoneMsg)
	if !ok {
		t.Fatalf("unexpected message type %T, want batchRestoreConfigsDoneMsg", cmd())
	}

	if msg.failCount != 0 {
		t.Errorf("failCount = %d, want 0: %+v", msg.failCount, msg.results)
	}

	for _, r := range msg.results {
		if strings.Contains(r.Message, "Not a config entry") {
			t.Errorf("setup entry reported as %q; it should have been queued to run", r.Message)
		}
	}

	if len(msg.setups) != 1 {
		t.Fatalf("queued %d setup entry(ies), want 1", len(msg.setups))
	}

	if msg.setups[0].sub.SubEntry.Name != "enable-service" {
		t.Errorf("queued the wrong entry: %q", msg.setups[0].sub.SubEntry.Name)
	}

	// Nothing may have executed yet: the queue runs through tea.Exec.
	if len(stub.Calls) != 0 {
		t.Errorf("batch restore executed %d setup command(s) inline: %+v", len(stub.Calls), stub.Calls)
	}
}

// TestHandleBatchRestoreConfigsDone_StartsQueuedSetups wires the two halves of
// a batch restore together.
func TestHandleBatchRestoreConfigsDone_StartsQueuedSetups(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	m := newSetupModel(t, stub, false)

	msg := batchRestoreConfigsDoneMsg{
		results:      []ResultItem{{Name: "vicinae/config-file", Success: true}},
		setups:       []setupRunItem{setupItemOf(m)},
		successCount: 1,
	}

	updated, cmd := m.handleBatchRestoreConfigsDone(msg)

	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("unexpected model type %T", updated)
	}

	if len(got.pendingSetups) != 1 {
		t.Fatalf("pendingSetups = %d, want 1", len(got.pendingSetups))
	}

	if !got.setupBatch {
		t.Error("setupBatch = false; the run belongs to a batch and must clear selections when it finishes")
	}

	if got.batchSuccessCount != 1 {
		t.Errorf("batchSuccessCount = %d, want 1 (carried over from the config half)", got.batchSuccessCount)
	}

	if cmd == nil {
		t.Fatal("no command dispatched to run the queued setup entry")
	}

	if msgType := fmt.Sprintf("%T", cmd()); msgType != "tea.execMsg" {
		t.Errorf("queued setup dispatched as %s, want tea.execMsg", msgType)
	}
}

// TestHandleBatchRestoreConfigsDone_NoSetups_CompletesAsBefore keeps a batch
// with no setup entries on exactly the path it took before this change.
func TestHandleBatchRestoreConfigsDone_NoSetups_CompletesAsBefore(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	m := newSetupModel(t, stub, false)

	msg := batchRestoreConfigsDoneMsg{
		results:      []ResultItem{{Name: "vicinae/config-file", Success: true}},
		successCount: 1,
	}

	_, cmd := m.handleBatchRestoreConfigsDone(msg)
	if cmd == nil {
		t.Fatal("no completion command dispatched")
	}

	done, ok := cmd().(BatchCompleteMsg)
	if !ok {
		t.Fatalf("unexpected message type %T, want BatchCompleteMsg", cmd())
	}

	if done.SuccessCount != 1 || done.FailCount != 0 {
		t.Errorf("BatchCompleteMsg = %+v, want 1 success / 0 failures", done)
	}
}

func TestHandleSetupRunResult_BatchCompletionClearsTransientSelection(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	m := newSetupModel(t, stub, false)
	m.pendingSetups = []setupRunItem{setupItemOf(m)}
	m.currentSetupIndex = 0
	m.setupBatch = true
	m.summaryTransientSelection = true
	m.multiSelectActive = true

	updated, _ := m.handleSetupRunResult(setupRunMsg{
		item: setupItemOf(m), success: true, message: "Setup complete",
	})
	got := updated.(Model)
	if got.summaryTransientSelection {
		t.Fatal("setup-ending completion left transient summary selection marked")
	}
}

// TestSummary_DescribesSetupEntriesAccurately covers the pre-flight summary,
// which listed setup entries as file restores to an empty target path.
func TestSummary_DescribesSetupEntriesAccurately(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	m := newSetupModel(t, stub, false)
	m.selectedApps = map[string]bool{"vicinae": true}
	m.multiSelectActive = true
	m.summaryOperation = OpRestore

	out := stripAnsiCodes(m.renderHierarchicalSummary("restore"))

	if !strings.Contains(out, "enable-service") {
		t.Fatalf("summary omitted the setup entry:\n%s", out)
	}

	// The config entry still shows its target; the setup entry must not be
	// described as a restore to an empty path.
	if strings.Contains(out, "enable-service → ") {
		t.Errorf("setup entry rendered as a file restore to an empty target:\n%s", out)
	}

	if !strings.Contains(out, TypeSetup) {
		t.Errorf("summary does not say the entry is a setup command:\n%s", out)
	}
}

// TestSetupRun_EndToEnd_RealSubprocess drives the whole TUI path against real
// subprocesses — no stub runner — proving the wiring actually works: the row is
// flagged "Needs setup" because the check fails, pressing restore runs the real
// setup command, and the re-resolved state (from re-running the real check)
// flips to "Set up".
func TestSetupRun_EndToEnd_RealSubprocess(t *testing.T) {
	if runtime.GOOS == OSWindows {
		t.Skip("the entry's check/run commands are POSIX shell")
	}

	// The "system state" the setup entry brings about: a marker file.
	marker := filepath.Join(t.TempDir(), "enabled")

	entry := config.SubEntry{
		Name:  "enable-service",
		Check: map[string]string{platform.OSLinux: "test -f " + marker},
		Run:   map[string]string{platform.OSLinux: "touch " + marker},
	}

	cfg := &config.Config{
		Version:      3,
		BackupRoot:   t.TempDir(), // setup commands run from the repo root
		Applications: []config.Application{{Name: "vicinae", Entries: []config.SubEntry{entry}}},
	}

	plat := linuxPlatform()
	mgr := manager.New(cfg, plat)

	m := NewModel(cfg, plat, false)
	m.Manager = mgr

	sub := m.Applications[0].SubItems[0]

	// Before: the check fails, so the row is flagged as needing setup.
	if got, diagnostic := detectSubEntryStateStatic(sub, plat, cfg, mgr); got != StateSetupNeeded || diagnostic != "" {
		t.Fatalf("state before running = (%v, %q), want (StateSetupNeeded, \"\")", got, diagnostic)
	}

	// Restore the row: the setup entry runs for real, through the same
	// setupExec that bubbletea invokes once it has released the terminal.
	item := setupRunItem{appIdx: 0, subIdx: 0, name: sub.SubEntry.Name, sub: sub}
	ex := &setupExec{model: m, item: item}

	if err := ex.Run(); err != nil {
		t.Fatalf("setupExec.Run() = %v, want nil", err)
	}

	if _, err := os.Stat(marker); err != nil {
		t.Fatalf("the setup command did not run: %v", err)
	}

	// After: feed the outcome back through the model and let the async resolver
	// re-run the real check.
	updated, cmd := m.handleSetupRunResult(setupRunMsg{item: item, success: ex.success, message: ex.message})

	got, ok := updated.(Model)
	if !ok {
		t.Fatalf("unexpected model type %T", updated)
	}

	if got.Applications[0].SubItems[0].State != StateLoading {
		t.Fatalf("row state = %v, want StateLoading pending the re-check", got.Applications[0].SubItems[0].State)
	}

	msgs := collectMsgs(cmd)
	if len(msgs) != 1 {
		t.Fatalf("dispatched %d re-check(s), want 1", len(msgs))
	}

	res, ok := msgs[0].(stateCheckResultMsg)
	if !ok {
		t.Fatalf("unexpected message type %T", msgs[0])
	}

	if res.state != StateSetupOk {
		t.Errorf("state after the real setup ran = %v, want StateSetupOk", res.state)
	}

	// Running it again is a no-op: the check now passes, so the run command
	// must not fire a second time.
	before, err := os.Stat(marker)
	if err != nil {
		t.Fatalf("stat marker: %v", err)
	}

	if err := os.WriteFile(marker, []byte("touched"), 0o600); err != nil {
		t.Fatalf("write marker: %v", err)
	}

	again := &setupExec{model: m, item: item}
	if err := again.Run(); err != nil {
		t.Fatalf("second setupExec.Run() = %v, want nil", err)
	}

	after, err := os.ReadFile(marker)
	if err != nil {
		t.Fatalf("read marker: %v", err)
	}

	if string(after) != "touched" {
		t.Errorf("the setup command ran again even though its check passed (marker content %q, size before %d)",
			string(after), before.Size())
	}
}

// TestStartSetupRun_IgnoresADispatchWhileOneIsQueued guards against double-tapping
// `r`: the second keypress lands before the first run's execMsg is handled, and
// would queue the same entry twice — producing a duplicate row in the results
// popup.
func TestStartSetupRun_IgnoresADispatchWhileOneIsQueued(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	m := newSetupModel(t, stub, false)

	if cmd := m.startSetupRun([]setupRunItem{setupItemOf(m)}, false); cmd == nil {
		t.Fatal("the first startSetupRun returned no command")
	}

	if cmd := m.startSetupRun([]setupRunItem{setupItemOf(m)}, false); cmd != nil {
		t.Error("a second startSetupRun dispatched while one is in flight; the entry would run (and be reported) twice")
	}

	if len(m.pendingSetups) != 1 {
		t.Errorf("pendingSetups = %d, want 1 (the queue in flight must not be replaced)", len(m.pendingSetups))
	}

	if m.currentSetupIndex != 0 {
		t.Errorf("currentSetupIndex = %d, want 0 (the in-flight run must not be rewound)", m.currentSetupIndex)
	}
}

// TestSetupRunResult_SurfacesATerminalHandoverFailure guards the blank-message
// bug. If bubbletea fails to release the terminal, Run() never executes: the
// exec carries no outcome (success = false, message = ""), and the results popup
// would show a failed row saying nothing while the real error was discarded.
func TestSetupRunResult_SurfacesATerminalHandoverFailure(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	m := newSetupModel(t, stub, false)

	// Nothing ran: this is the state of the exec when the handover itself failed.
	ex := &setupExec{model: *m, item: setupItemOf(m)}

	msg := setupRunResult(ex, errors.New("release terminal: inappropriate ioctl for device"))

	if msg.success {
		t.Error("a failed terminal handover was reported as a success")
	}

	if msg.message == "" {
		t.Fatal("the results popup would show a failed row with a blank message; the real error was discarded")
	}

	if !strings.Contains(msg.message, "inappropriate ioctl for device") {
		t.Errorf("message = %q, want it to carry the underlying error", msg.message)
	}
}

// TestSetupRunResult_KeepsTheRecordedFailure proves the handover error never
// overwrites the more specific message the setup itself produced.
func TestSetupRunResult_KeepsTheRecordedFailure(t *testing.T) {
	stub := cmdexec.NewStubRunner()
	m := newSetupModel(t, stub, false)

	ex := &setupExec{model: *m, item: setupItemOf(m), success: false, message: "Failed: check command not found"}

	msg := setupRunResult(ex, errors.New("release terminal: broken pipe"))

	if msg.message != "Failed: check command not found" {
		t.Errorf("message = %q, want the recorded failure to survive (it is the more specific one)", msg.message)
	}
}
