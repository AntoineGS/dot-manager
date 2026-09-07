package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/manager"
	"github.com/AntoineGS/tidydots/internal/platform"
	"github.com/AntoineGS/tidydots/internal/testutil"
	"github.com/AntoineGS/tidydots/internal/tui"
)

func preserveCommandGlobals(t *testing.T) {
	t.Helper()

	originalConfigDir := configDir
	originalOSOverride := osOverride
	originalDryRun := dryRun
	originalVerbose := verbose
	originalActions := actions
	originalStatusActions := statusActions
	originalStatusJSON := statusJSON
	originalInteractiveIsTerminal := interactiveIsTerminal
	originalRunInteractiveTUI := runInteractiveTUI

	configDir = ""
	osOverride = ""
	dryRun = false
	verbose = false
	actions = false
	statusActions = false
	statusJSON = false

	t.Cleanup(func() {
		configDir = originalConfigDir
		osOverride = originalOSOverride
		dryRun = originalDryRun
		verbose = originalVerbose
		actions = originalActions
		statusActions = originalStatusActions
		statusJSON = originalStatusJSON
		interactiveIsTerminal = originalInteractiveIsTerminal
		runInteractiveTUI = originalRunInteractiveTUI
	})
}

func writeStatusConfig(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	target := filepath.Join(dir, "target")
	configYAML := fmt.Sprintf(`version: 3
applications:
  - name: tool
    description: Tool description
    entries:
      - name: missing
        backup: ./missing
        targets:
          linux: %q
`, target)
	if err := os.WriteFile(filepath.Join(dir, "tidydots.yaml"), []byte(configYAML), 0o600); err != nil {
		t.Fatalf("writing status config: %v", err)
	}

	return dir
}

func executeStatusCommand(t *testing.T, dir string) (string, error) {
	t.Helper()
	return executeStatusCommandArgs(t, dir, "status", "--actions", "--json")
}

func executeStatusCommandArgs(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()

	root := newRootCommand()
	var output bytes.Buffer
	root.SetOut(&output)
	root.SetErr(io.Discard)
	root.SetArgs(append([]string{"--dir", dir, "--os", "linux", "--verbose"}, args...))
	err := root.Execute()

	return output.String(), err
}

func TestStatusCommandReportsOnlySelectedTemplateState(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("selected template status command test requires symlinks")
	}
	preserveCommandGlobals(t)

	dir := testutil.CanonicalTempDir(t)
	backup := filepath.Join(dir, "backup")
	target := filepath.Join(testutil.CanonicalTempDir(t), "target")
	if err := os.MkdirAll(backup, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o750); err != nil {
		t.Fatal(err)
	}
	configYAML := fmt.Sprintf(`version: 3
applications:
  - name: tool
    entries:
      - name: config
        backup: ./backup
        files:
          - selected.tmpl
        targets:
          linux: %q
      - name: copy
        backup: ./backup
        files:
          - copy.tmpl
        method: copy
        targets:
          linux: %q
`, target, target)
	if err := os.WriteFile(filepath.Join(dir, "tidydots.yaml"), []byte(configYAML), 0o600); err != nil {
		t.Fatal(err)
	}

	selected := filepath.Join(backup, "selected.tmpl")
	unselected := filepath.Join(backup, "unselected.tmpl")
	copySource := filepath.Join(backup, "copy.tmpl")
	writeCommandStatusFile(t, selected, "selected-v1")
	writeCommandStatusFile(t, unselected, "unselected-v1")
	writeCommandStatusFile(t, copySource, "copy-v1")

	cfg, err := config.Load(filepath.Join(dir, "tidydots.yaml"))
	if err != nil {
		t.Fatalf("load status config: %v", err)
	}
	cfg.BackupRoot = dir
	plat := platform.Detect().WithOS(platform.OSLinux)
	mgr := manager.New(cfg, plat)
	if err := mgr.InitStateStore(); err != nil {
		t.Fatalf("init status state: %v", err)
	}
	seedSelectedEntry := cfg.Applications[0].Entries[0]
	seedSelectedEntry.Files = []string{"selected.tmpl", "unselected.tmpl"}
	if err := mgr.RestoreFiles(seedSelectedEntry, backup, target); err != nil {
		_ = mgr.Close()
		t.Fatalf("seed selected template state: %v", err)
	}
	seedCopyEntry := cfg.Applications[0].Entries[1]
	if err := mgr.RestoreFiles(seedCopyEntry, backup, target); err != nil {
		_ = mgr.Close()
		t.Fatalf("seed copy template state: %v", err)
	}
	if err := mgr.Close(); err != nil {
		t.Fatal(err)
	}

	writeCommandStatusFile(t, unselected, "unselected-v2")
	output, err := executeStatusCommandArgs(t, dir, "status", "--json")
	if err != nil {
		t.Fatalf("status command error: %v", err)
	}

	var report tui.StatusReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("status output is not JSON: %v\n%s", err, output)
	}
	if len(report.Applications) != 1 || len(report.Applications[0].Entries) != 2 {
		t.Fatalf("status entries = %+v, want selected and copy entries", report.Applications)
	}
	selectedEntry := findStatusEntry(t, report, "config")
	copyEntry := findStatusEntry(t, report, "copy")
	if got := selectedEntry.State; got != tui.StateLinked.String() {
		t.Fatalf("selected entry state = %q, want %q", got, tui.StateLinked.String())
	}
	if got := copyEntry.State; got != tui.StateLinked.String() {
		t.Fatalf("copy entry state = %q, want %q", got, tui.StateLinked.String())
	}

	writeCommandStatusFile(t, selected, "selected-v2")
	output, err = executeStatusCommandArgs(t, dir, "status", "--json")
	if err != nil {
		t.Fatalf("status command after selected source drift: %v", err)
	}
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("status output after selected source drift is not JSON: %v\n%s", err, output)
	}
	selectedEntry = findStatusEntry(t, report, "config")
	if got := selectedEntry.State; got != tui.StateOutdated.String() {
		t.Fatalf("selected entry drift state = %q, want %q", got, tui.StateOutdated.String())
	}
	if got := findStatusEntry(t, report, "copy").State; got != tui.StateLinked.String() {
		t.Fatalf("copy entry after selected drift = %q, want %q", got, tui.StateLinked.String())
	}

	writeCommandStatusFile(t, filepath.Join(target, "copy"), "copy-user-edit")
	output, err = executeStatusCommandArgs(t, dir, "status", "--json")
	if err != nil {
		t.Fatalf("status command after copy target edit: %v", err)
	}
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("status output after copy target edit is not JSON: %v\n%s", err, output)
	}
	if got := findStatusEntry(t, report, "copy").State; got != tui.StateModified.String() {
		t.Fatalf("copy target edit state = %q, want %q", got, tui.StateModified.String())
	}
	actionsOutput, err := executeStatusCommandArgs(t, dir, "status", "--actions", "--json")
	if err != nil {
		t.Fatalf("status --actions after copy target edit: %v", err)
	}
	if err := json.Unmarshal([]byte(actionsOutput), &report); err != nil {
		t.Fatalf("actions status output is not JSON: %v\n%s", err, actionsOutput)
	}
	if got := findStatusEntry(t, report, "copy").State; got != tui.StateModified.String() {
		t.Fatalf("copy target edit missing from --actions: %q", got)
	}

	writeCommandStatusFile(t, copySource, "copy-v2")
	output, err = executeStatusCommandArgs(t, dir, "status", "--json")
	if err != nil {
		t.Fatalf("status command after copy source drift: %v", err)
	}
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("status output after copy source drift is not JSON: %v\n%s", err, output)
	}
	if got := findStatusEntry(t, report, "copy").State; got != tui.StateOutdated.String() {
		t.Fatalf("copy source drift state = %q, want %q", got, tui.StateOutdated.String())
	}

	if err := os.Remove(filepath.Join(target, "copy")); err != nil {
		t.Fatal(err)
	}
	output, err = executeStatusCommandArgs(t, dir, "status", "--json")
	if err != nil {
		t.Fatalf("status command after missing copy target: %v", err)
	}
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("status output after missing copy target is not JSON: %v\n%s", err, output)
	}
	if got := findStatusEntry(t, report, "copy").State; got != tui.StateReady.String() {
		t.Fatalf("missing copy target state = %q, want %q", got, tui.StateReady.String())
	}

	writeCommandStatusFile(t, selected, "selected-v1")
	writeCommandStatusFile(t, filepath.Join(backup, "selected.tmpl.rendered"), "selected-user-edit")
	output, err = executeStatusCommandArgs(t, dir, "status", "--json")
	if err != nil {
		t.Fatalf("status command after rendered edit: %v", err)
	}
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("status output after rendered edit is not JSON: %v\n%s", err, output)
	}
	if got := findStatusEntry(t, report, "config").State; got != tui.StateModified.String() {
		t.Fatalf("selected entry rendered-edit state = %q, want %q", got, tui.StateModified.String())
	}

	if err := os.Remove(filepath.Join(backup, "selected.tmpl.rendered")); err != nil {
		t.Fatal(err)
	}
	output, err = executeStatusCommandArgs(t, dir, "status", "--json")
	if err != nil {
		t.Fatalf("status command after missing rendered output: %v", err)
	}
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("status output after missing rendered output is not JSON: %v\n%s", err, output)
	}
	if got := findStatusEntry(t, report, "config").State; got != tui.StateOutdated.String() {
		t.Fatalf("selected entry missing-rendered state = %q, want %q", got, tui.StateOutdated.String())
	}
}

func TestStatusCommandActionsIncludesUnavailableCopySelection(t *testing.T) {
	preserveCommandGlobals(t)
	dir := t.TempDir()
	backup := filepath.Join(dir, "backup")
	target := filepath.Join(t.TempDir(), "target")
	if err := os.MkdirAll(backup, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(target, 0o750); err != nil {
		t.Fatal(err)
	}
	configYAML := fmt.Sprintf(`version: 3
applications:
  - name: tool
    entries:
      - name: unsafe-copy
        backup: ./backup
        files:
          - root.tmpl
          - root
        method: copy
        targets:
          linux: %q
`, target)
	if err := os.WriteFile(filepath.Join(dir, "tidydots.yaml"), []byte(configYAML), 0o600); err != nil {
		t.Fatal(err)
	}
	writeCommandStatusFile(t, filepath.Join(backup, "root.tmpl"), "root = 1")
	writeCommandStatusFile(t, filepath.Join(target, "root"), "root = 1")

	output, err := executeStatusCommandArgs(t, dir, "status", "--actions", "--json")
	if err != nil {
		t.Fatalf("status --actions error: %v", err)
	}
	var report tui.StatusReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("status output is not JSON: %v\n%s", err, output)
	}
	entry := findStatusEntry(t, report, "unsafe-copy")
	if entry.State != tui.StateUnavailable.String() || !entry.Actionable {
		t.Fatalf("unavailable entry = %+v, want actionable Unavailable", entry)
	}
	if report.Counts.ActionableEntries != 1 || !report.Actionable {
		t.Fatalf("unavailable action counts = %+v/actionable=%t, want one actionable entry", report.Counts, report.Actionable)
	}
}

func findStatusEntry(t *testing.T, report tui.StatusReport, name string) tui.StatusEntry {
	t.Helper()
	if len(report.Applications) != 1 {
		t.Fatalf("status applications = %d, want 1", len(report.Applications))
	}
	for _, entry := range report.Applications[0].Entries {
		if entry.Name == name {
			return entry
		}
	}
	t.Fatalf("status entry %q not found in %+v", name, report.Applications[0].Entries)
	return tui.StatusEntry{}
}

func writeCommandStatusFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestStatusJSONIsStableAndActionableDoesNotChangeExitStatus(t *testing.T) {
	preserveCommandGlobals(t)
	dir := writeStatusConfig(t)

	first, err := executeStatusCommand(t, dir)
	if err != nil {
		t.Fatalf("status command error for actionable config = %v", err)
	}
	second, err := executeStatusCommand(t, dir)
	if err != nil {
		t.Fatalf("second status command error = %v", err)
	}
	if first != second {
		t.Fatalf("status JSON changed between identical runs:\nfirst:\n%ssecond:\n%s", first, second)
	}

	var report tui.StatusReport
	if err := json.Unmarshal([]byte(first), &report); err != nil {
		t.Fatalf("status output is not JSON: %v\n%s", err, first)
	}
	if !report.Actionable || !report.ActionsOnly {
		t.Fatalf("report flags = actionable:%t actions_only:%t, want true:true", report.Actionable, report.ActionsOnly)
	}
	if report.Counts.ActionableApplications != 1 || report.Counts.ActionableEntries != 1 {
		t.Fatalf("action counts = %+v, want one application and entry", report.Counts)
	}
	if len(report.Applications) != 1 || len(report.Applications[0].Entries) != 1 {
		t.Fatalf("action details = %+v, want one application and entry", report.Applications)
	}
	if report.Applications[0].Name != "tool" || report.Applications[0].Entries[0].Name != "missing" {
		t.Fatalf("detail identity = %+v, want tool/missing", report.Applications)
	}
	if report.Applications[0].Entries[0].State != tui.StateMissing.String() {
		t.Fatalf("entry state = %q, want %q", report.Applications[0].Entries[0].State, tui.StateMissing.String())
	}
}

func TestStatusCommandReportsFailedSetupCheckInTextAndJSON(t *testing.T) {
	preserveCommandGlobals(t)

	dir := t.TempDir()
	configYAML := `version: 3
applications:
  - name: tool
    entries:
      - name: remote-check
        check_mode: status
        check:
          linux: "printf 'remote unavailable' >&2; exit 3"
        run:
          linux: "printf 'setup should not run' >&2; exit 0"
`
	if err := os.WriteFile(filepath.Join(dir, "tidydots.yaml"), []byte(configYAML), 0o600); err != nil {
		t.Fatalf("writing failed-check config: %v", err)
	}

	textOutput, err := executeStatusCommandArgs(t, dir, "status")
	if err != nil {
		t.Fatalf("text status command error = %v", err)
	}
	if !strings.Contains(textOutput, "remote-check: Check failed: remote unavailable") {
		t.Fatalf("text status omitted failed-check diagnostic:\n%s", textOutput)
	}

	jsonOutput, err := executeStatusCommandArgs(t, dir, "status", "--json")
	if err != nil {
		t.Fatalf("JSON status command error = %v", err)
	}
	var report tui.StatusReport
	if err := json.Unmarshal([]byte(jsonOutput), &report); err != nil {
		t.Fatalf("failed-check status output is not JSON: %v\n%s", err, jsonOutput)
	}
	entry := findStatusEntry(t, report, "remote-check")
	if entry.State != tui.StateCheckFailed.String() || entry.Error != "remote unavailable" || !entry.Actionable {
		t.Fatalf("failed-check status entry = %+v, want state, diagnostic, and actionable", entry)
	}

	actionsOutput, err := executeStatusCommandArgs(t, dir, "status", "--actions", "--json")
	if err != nil {
		t.Fatalf("actionable status command returned an error: %v", err)
	}
	if err := json.Unmarshal([]byte(actionsOutput), &report); err != nil {
		t.Fatalf("actionable status output is not JSON: %v\n%s", err, actionsOutput)
	}
	if !report.Actionable || len(report.Applications) != 1 || len(report.Applications[0].Entries) != 1 {
		t.Fatalf("actionable report = %+v, want one actionable application and entry", report)
	}
}

func TestStatusFailureReturnsError(t *testing.T) {
	preserveCommandGlobals(t)

	root := newRootCommand()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--dir", filepath.Join(t.TempDir(), "missing"), "status", "--json"})
	if err := root.Execute(); err == nil {
		t.Fatal("status command succeeded with a missing configuration directory")
	}
}

func TestRootActionsStartsInteractiveTUIWithActionFilter(t *testing.T) {
	preserveCommandGlobals(t)
	dir := writeStatusConfig(t)

	interactiveIsTerminal = func() bool { return true }
	var gotActionFilter bool
	runInteractiveTUI = func(_ *config.Config, _ *platform.Platform, _ bool, _ string, actionFilterEnabled bool) error {
		gotActionFilter = actionFilterEnabled
		return nil
	}

	root := newRootCommand()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--dir", dir, "--os", "linux", "--actions"})
	if err := root.Execute(); err != nil {
		t.Fatalf("root --actions command error = %v", err)
	}
	if !gotActionFilter {
		t.Fatal("root --actions did not enable the TUI action filter")
	}
}

func TestRootNoArgsPreservesInteractiveStartup(t *testing.T) {
	preserveCommandGlobals(t)
	dir := writeStatusConfig(t)

	interactiveIsTerminal = func() bool { return true }
	var gotActionFilter bool
	runInteractiveTUI = func(_ *config.Config, _ *platform.Platform, _ bool, _ string, actionFilterEnabled bool) error {
		gotActionFilter = actionFilterEnabled
		return nil
	}

	root := newRootCommand()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--dir", dir, "--os", "linux"})
	if err := root.Execute(); err != nil {
		t.Fatalf("no-argument command error = %v", err)
	}
	if gotActionFilter {
		t.Fatal("no-argument startup unexpectedly enabled the action filter")
	}
}

func TestRootActionsRejectsSubcommandCombination(t *testing.T) {
	preserveCommandGlobals(t)

	root := newRootCommand()
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	root.SetArgs([]string{"--actions", "list"})
	err := root.Execute()
	if err == nil || !strings.Contains(err.Error(), "unknown flag: --actions") {
		t.Fatalf("root --actions list error = %v, want invalid combination", err)
	}
}
