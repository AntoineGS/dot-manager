package tui

import (
	"reflect"
	"strings"
	"testing"

	"charm.land/bubbles/v2/table"
	"charm.land/lipgloss/v2"
	"github.com/AntoineGS/tidydots/internal/config"
)

const (
	treeCharBranch = "├─"
	treeCharEnd    = "└─"
)

func TestFlattenApplications(t *testing.T) {
	t.Run("single collapsed application", func(t *testing.T) {
		apps := []ApplicationItem{
			{
				Application: config.Application{Name: "nvim"},
				SubItems: []SubEntryItem{
					{SubEntry: config.SubEntry{Name: "config"}},
				},
				Expanded: false,
			},
		}

		rows := flattenApplications(apps, "linux", false)

		if len(rows) != 1 {
			t.Errorf("Expected 1 row, got %d", len(rows))
		}

		if rows[0].Level != 0 {
			t.Errorf("First row should be level 0")
		}

		if rows[0].SubIndex != -1 {
			t.Errorf("First row should have SubIndex -1, got %d", rows[0].SubIndex)
		}
	})

	t.Run("single expanded application", func(t *testing.T) {
		apps := []ApplicationItem{
			{
				Application: config.Application{Name: "nvim"},
				SubItems: []SubEntryItem{
					{SubEntry: config.SubEntry{Name: "config"}},
				},
				Expanded: true,
			},
		}

		rows := flattenApplications(apps, "linux", false)

		if len(rows) != 2 {
			t.Errorf("Expected 2 rows, got %d", len(rows))
		}

		if rows[0].Level != 0 {
			t.Errorf("First row should be level 0")
		}

		if rows[1].Level != 1 {
			t.Errorf("Second row should be level 1")
		}

		if rows[1].TreeChar != treeCharEnd {
			t.Errorf("Last sub-entry should have %s tree char, got %s", treeCharEnd, rows[1].TreeChar)
		}
	})

	t.Run("multiple sub-entries with correct tree chars", func(t *testing.T) {
		apps := []ApplicationItem{
			{
				Application: config.Application{Name: "nvim"},
				SubItems: []SubEntryItem{
					{SubEntry: config.SubEntry{Name: "config1"}},
					{SubEntry: config.SubEntry{Name: "config2"}},
					{SubEntry: config.SubEntry{Name: "config3"}},
				},
				Expanded: true,
			},
		}

		rows := flattenApplications(apps, "linux", false)

		if len(rows) != 4 {
			t.Errorf("Expected 4 rows, got %d", len(rows))
		}

		if rows[1].TreeChar != treeCharBranch {
			t.Errorf("First sub-entry should have %s tree char, got %s", treeCharBranch, rows[1].TreeChar)
		}

		if rows[2].TreeChar != treeCharBranch {
			t.Errorf("Middle sub-entry should have %s tree char, got %s", treeCharBranch, rows[2].TreeChar)
		}

		if rows[3].TreeChar != treeCharEnd {
			t.Errorf("Last sub-entry should have %s tree char, got %s", treeCharEnd, rows[3].TreeChar)
		}
	})

	t.Run("app with no sub-items has no expansion arrow", func(t *testing.T) {
		apps := []ApplicationItem{
			{
				Application: config.Application{Name: "empty-app"},
				SubItems:    []SubEntryItem{},
				Expanded:    false,
			},
		}

		rows := flattenApplications(apps, "linux", false)

		if len(rows) != 1 {
			t.Errorf("Expected 1 row, got %d", len(rows))
		}

		// App name should have two spaces for alignment (no expansion arrow)
		if rows[0].Data[0] != "  empty-app" {
			t.Errorf("Expected '  empty-app', got %q", rows[0].Data[0])
		}

		// TreeChar should be two spaces (for alignment)
		if rows[0].TreeChar != "  " {
			t.Errorf("Expected two-space TreeChar, got %q", rows[0].TreeChar)
		}
	})
}

func TestFilterActionableApplications(t *testing.T) {
	installed := true
	missing := false
	apps := []ApplicationItem{
		{
			Application:  config.Application{Name: "package-missing", Package: &config.EntryPackage{}},
			PkgInstalled: &missing,
		},
		{
			Application: config.Application{Name: "entry-actionable"},
			SubItems: []SubEntryItem{
				{SubEntry: config.SubEntry{Name: "linked"}, State: StateLinked, Index: 0},
				{SubEntry: config.SubEntry{Name: "missing"}, State: StateMissing, Index: 1},
			},
			Expanded: true,
		},
		{
			Application: config.Application{Name: "mixed"},
			SubItems: []SubEntryItem{
				{SubEntry: config.SubEntry{Name: "ready"}, State: StateReady, Index: 0},
				{SubEntry: config.SubEntry{Name: "loading"}, State: StateLoading, Index: 1},
				{SubEntry: config.SubEntry{Name: "unknown"}, State: StateLinked, Index: 2},
			},
			Expanded: true,
		},
		{
			Application:  config.Application{Name: "installed"},
			PkgInstalled: &installed,
			SubItems: []SubEntryItem{
				{SubEntry: config.SubEntry{Name: "linked"}, State: StateLinked, Index: 0},
			},
		},
		{
			Application: config.Application{Name: "loading"},
			SubItems: []SubEntryItem{
				{SubEntry: config.SubEntry{Name: "loading"}, State: StateLoading, Index: 0},
			},
		},
		{
			Application: config.Application{Name: "unknown"},
			SubItems: []SubEntryItem{
				{SubEntry: config.SubEntry{Name: "unknown"}, State: PathState(999), Index: 0},
			},
		},
	}

	filtered := filterActionableApplications(apps, false)
	if got, want := len(filtered), 3; got != want {
		t.Fatalf("got %d applications, want %d", got, want)
	}

	if filtered[0].Application.Name != "package-missing" || len(filtered[0].SubItems) != 0 {
		t.Errorf("package-missing should remain without children: %+v", filtered[0])
	}
	if filtered[1].Application.Name != "entry-actionable" || len(filtered[1].SubItems) != 1 ||
		filtered[1].SubItems[0].SubEntry.Name != "missing" || filtered[1].SubItems[0].Index != 1 {
		t.Errorf("entry-actionable children were not reduced with original index: %+v", filtered[1])
	}
	if filtered[2].Application.Name != "mixed" || len(filtered[2].SubItems) != 1 ||
		filtered[2].SubItems[0].SubEntry.Name != "ready" || filtered[2].SubItems[0].Index != 0 {
		t.Errorf("mixed children were not reduced: %+v", filtered[2])
	}
	if apps[1].SubItems[0].State != StateLinked || len(apps[1].SubItems) != 2 {
		t.Error("action filtering must not mutate the source applications")
	}
}

func TestFilterActionableApplicationsIncludesUnavailable(t *testing.T) {
	apps := []ApplicationItem{
		{
			Application: config.Application{Name: "protected-config"},
			SubItems: []SubEntryItem{
				{SubEntry: config.SubEntry{Name: "root-file"}, State: StateUnavailable},
			},
		},
	}

	filtered := filterActionableApplications(apps, false)
	if len(filtered) != 1 || len(filtered[0].SubItems) != 1 {
		t.Fatalf("unavailable entry was filtered out: %+v", filtered)
	}
	if got := filtered[0].SubItems[0].State; got != StateUnavailable {
		t.Fatalf("filtered state = %v, want StateUnavailable", got)
	}
}

func TestActionFilterBannerCountMatchesVisibleApplications(t *testing.T) {
	missing := false
	m := Model{
		Applications: []ApplicationItem{
			{Application: config.Application{Name: "visible", Package: &config.EntryPackage{}}, PkgInstalled: &missing},
			{Application: config.Application{Name: "machine-filtered", Package: &config.EntryPackage{}}, PkgInstalled: &missing, IsFiltered: true},
		},
		Platform:            linuxPlatform(),
		Operation:           OpList,
		Screen:              ScreenResults,
		actionFilterEnabled: true,
		filterEnabled:       true,
		height:              24,
		width:               120,
	}
	m.rebuildTable()

	view := m.viewListTable()
	if !strings.Contains(view, "action filter: on (1 apps)") {
		t.Fatalf("action-filter banner did not match visible application rows:\n%s", view)
	}
}

func TestFilterActionableApplications_ComposesWithSearch(t *testing.T) {
	missing := false
	apps := []ApplicationItem{
		{
			Application:  config.Application{Name: "package-match", Package: &config.EntryPackage{}},
			PkgInstalled: &missing,
		},
		{
			Application: config.Application{Name: "other"},
			SubItems: []SubEntryItem{
				{SubEntry: config.SubEntry{Name: "match"}, State: StateMissing, Index: 0},
				{SubEntry: config.SubEntry{Name: "unmatched"}, State: StateLinked, Index: 1},
			},
		},
	}

	m := Model{Applications: apps, searchText: "match", actionFilterEnabled: true}
	searched := m.getSearchedApplications()
	filtered := filterActionableApplications(searched, false)

	if got, want := len(filtered), 2; got != want {
		t.Fatalf("got %d applications, want %d", got, want)
	}
	if len(filtered[1].SubItems) != 1 || filtered[1].SubItems[0].Index != 0 {
		t.Errorf("combined filter lost actionable search result index: %+v", filtered[1].SubItems)
	}
}

func TestGetApplicationStatus(t *testing.T) {
	t.Run("filtered application", func(t *testing.T) {
		app := ApplicationItem{
			IsFiltered: true,
		}

		status := getApplicationStatus(app)
		if status != StatusFiltered {
			t.Errorf("Expected StatusFiltered, got %s", status)
		}
	})

	t.Run("no package configured", func(t *testing.T) {
		app := ApplicationItem{
			SubItems: []SubEntryItem{
				{State: StateLinked},
			},
		}

		status := getApplicationStatus(app)
		if status != StatusUnknown {
			t.Errorf("Expected StatusUnknown, got %s", status)
		}
	})

	t.Run("package configured but no manager available", func(t *testing.T) {
		app := ApplicationItem{
			Application: config.Application{
				Package: &config.EntryPackage{
					Managers: map[string]config.ManagerValue{
						"pacman": {PackageName: "yazi"},
					},
				},
			},
			PkgMethod: TypeNone,
		}

		status := getApplicationStatus(app)
		if status != StatusUnknown {
			t.Errorf("Expected StatusUnknown, got %s", status)
		}
	})
}

func TestGetTypeInfo(t *testing.T) {
	t.Run("folder", func(t *testing.T) {
		item := SubEntryItem{
			SubEntry: config.SubEntry{
				Files:  []string{}, // Empty files = folder
				Backup: "./test",   // Backup path indicates config type
			},
		}

		typeInfo := getTypeInfo(item)
		if typeInfo != TypeFolder {
			t.Errorf("Expected TypeFolder, got %s", typeInfo)
		}
	})

	t.Run("single file", func(t *testing.T) {
		item := SubEntryItem{
			SubEntry: config.SubEntry{
				Files: []string{"file1.txt"},
			},
		}

		typeInfo := getTypeInfo(item)
		if typeInfo != "1 file" {
			t.Errorf("Expected '1 file', got %s", typeInfo)
		}
	})

	t.Run("multiple files", func(t *testing.T) {
		item := SubEntryItem{
			SubEntry: config.SubEntry{
				Files: []string{"file1.txt", "file2.txt", "file3.txt"},
			},
		}

		typeInfo := getTypeInfo(item)
		if typeInfo != "3 files" {
			t.Errorf("Expected '3 files', got %s", typeInfo)
		}
	})
}

func TestVisualWidth(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected int
	}{
		{"ASCII", "hello", 5},
		{"Tree chars", "├─", 2},
		{"Emoji", "🎉", 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			width := lipgloss.Width(tt.input)
			if width != tt.expected {
				t.Errorf("Width of %q: expected %d, got %d", tt.input, tt.expected, width)
			}
		})
	}
}

func TestNeedsAttention(t *testing.T) {
	t.Run("status needs attention when not Installed", func(t *testing.T) {
		if !needsAttention(StatusMissing) {
			t.Errorf("StatusMissing should need attention")
		}
		if !needsAttention(StatusFiltered) {
			t.Errorf("StatusFiltered should need attention")
		}
	})

	t.Run("status does not need attention when Installed", func(t *testing.T) {
		if needsAttention(StatusInstalled) {
			t.Errorf("StatusInstalled should not need attention")
		}
	})

	t.Run("status does not need attention when Unknown", func(t *testing.T) {
		if needsAttention(StatusUnknown) {
			t.Errorf("StatusUnknown should not need attention")
		}
	})

	t.Run("sub-entry state needs attention when not Linked", func(t *testing.T) {
		if !needsAttention(StateMissing.String()) {
			t.Errorf("StateMissing should need attention")
		}
		if !needsAttention(StateReady.String()) {
			t.Errorf("StateReady should need attention")
		}
		if !needsAttention(StateAdopt.String()) {
			t.Errorf("StateAdopt should need attention")
		}
		if !needsAttention(StateCheckFailed.String()) {
			t.Errorf("StateCheckFailed should need attention")
		}
	})

	t.Run("sub-entry state does not need attention when Linked", func(t *testing.T) {
		if needsAttention(StateLinked.String()) {
			t.Errorf("StateLinked should not need attention")
		}
	})
}

func TestAppInfoMaxState(t *testing.T) {
	t.Run("returns highest severity state", func(t *testing.T) {
		app := ApplicationItem{
			SubItems: []SubEntryItem{
				{State: StateLinked},
				{State: StateMissing},
			},
		}
		if got := appInfoMaxState(app); got != StateMissing {
			t.Errorf("expected StateMissing, got %v", got)
		}
	})

	t.Run("returns StateLinked when all sub-entries are Linked", func(t *testing.T) {
		app := ApplicationItem{
			SubItems: []SubEntryItem{
				{State: StateLinked},
				{State: StateLinked},
			},
		}
		if got := appInfoMaxState(app); got != StateLinked {
			t.Errorf("expected StateLinked, got %v", got)
		}
	})

	t.Run("returns StateLinked when filtered", func(t *testing.T) {
		app := ApplicationItem{
			IsFiltered: true,
			SubItems: []SubEntryItem{
				{State: StateMissing},
			},
		}
		if got := appInfoMaxState(app); got != StateLinked {
			t.Errorf("expected StateLinked for filtered app, got %v", got)
		}
	})

	t.Run("red state wins over amber and blue", func(t *testing.T) {
		app := ApplicationItem{
			SubItems: []SubEntryItem{
				{State: StateModified},
				{State: StateOutdated},
				{State: StateMissing},
			},
		}
		if got := appInfoMaxState(app); got != StateMissing {
			t.Errorf("expected StateMissing (red), got %v", got)
		}
	})

	t.Run("amber state wins over blue", func(t *testing.T) {
		app := ApplicationItem{
			SubItems: []SubEntryItem{
				{State: StateLinked},
				{State: StateModified},
				{State: StateOutdated},
			},
		}
		if got := appInfoMaxState(app); got != StateOutdated {
			t.Errorf("expected StateOutdated (amber), got %v", got)
		}
	})

	t.Run("blue state wins over linked", func(t *testing.T) {
		app := ApplicationItem{
			SubItems: []SubEntryItem{
				{State: StateLinked},
				{State: StateModified},
			},
		}
		if got := appInfoMaxState(app); got != StateModified {
			t.Errorf("expected StateModified (blue), got %v", got)
		}
	})
}

func TestFlattenApplications_WithFilterEnabled(t *testing.T) {
	apps := []ApplicationItem{
		{
			Application: config.Application{Name: "visible-app"},
			SubItems: []SubEntryItem{
				{SubEntry: config.SubEntry{Name: "config1"}},
			},
			Expanded:   true,
			IsFiltered: false, // Not filtered - should be visible
		},
		{
			Application: config.Application{Name: "filtered-app"},
			SubItems: []SubEntryItem{
				{SubEntry: config.SubEntry{Name: "config2"}},
			},
			Expanded:   true,
			IsFiltered: true, // Filtered - should be hidden
		},
	}

	t.Run("filter enabled hides filtered apps", func(t *testing.T) {
		rows := flattenApplications(apps, "linux", true)

		// Should only show visible-app (1 app + 1 sub-entry = 2 rows)
		if len(rows) != 2 {
			t.Errorf("Expected 2 rows (1 app + 1 sub-entry), got %d", len(rows))
		}

		if rows[0].Data[0] != "▼ visible-app" {
			t.Errorf("Expected first row to be visible-app, got %s", rows[0].Data[0])
		}
	})

	t.Run("filter disabled shows all apps", func(t *testing.T) {
		rows := flattenApplications(apps, "linux", false)

		// Should show both apps (2 apps + 2 sub-entries = 4 rows)
		if len(rows) != 4 {
			t.Errorf("Expected 4 rows (2 apps + 2 sub-entries), got %d", len(rows))
		}
	})
}

func TestFlattenApplications_AppNameMapping(t *testing.T) {
	apps := []ApplicationItem{
		{
			Application: config.Application{Name: "app-alpha"},
			SubItems: []SubEntryItem{
				{SubEntry: config.SubEntry{Name: "config1"}},
			},
			Expanded: true,
		},
		{
			Application: config.Application{Name: "app-beta"},
			SubItems: []SubEntryItem{
				{SubEntry: config.SubEntry{Name: "config2"}},
			},
			Expanded: true,
		},
	}

	rows := flattenApplications(apps, "linux", false)

	// Verify we have 4 rows (2 apps + 2 sub-entries)
	if len(rows) != 4 {
		t.Errorf("Expected 4 rows, got %d", len(rows))
	}

	// Verify all rows have correct AppName
	if rows[0].AppName != "app-alpha" {
		t.Errorf("Expected AppName 'app-alpha', got %s", rows[0].AppName)
	}
	if rows[1].AppName != "app-alpha" {
		t.Errorf("Expected AppName 'app-alpha' for sub-entry, got %s", rows[1].AppName)
	}
	if rows[2].AppName != "app-beta" {
		t.Errorf("Expected AppName 'app-beta', got %s", rows[2].AppName)
	}
	if rows[3].AppName != "app-beta" {
		t.Errorf("Expected AppName 'app-beta' for sub-entry, got %s", rows[3].AppName)
	}
}

func TestGetApplicationStatus_PackageState(t *testing.T) {
	notInstalled := false
	installed := true

	t.Run("package not installed overrides linked configs", func(t *testing.T) {
		app := ApplicationItem{
			PkgInstalled: &notInstalled,
			SubItems: []SubEntryItem{
				{State: StateLinked},
			},
		}

		status := getApplicationStatus(app)
		if status != StatusMissing {
			t.Errorf("Expected StatusMissing, got %s", status)
		}
	})

	t.Run("package not installed with no configs", func(t *testing.T) {
		app := ApplicationItem{
			PkgInstalled: &notInstalled,
		}

		status := getApplicationStatus(app)
		if status != StatusMissing {
			t.Errorf("Expected StatusMissing, got %s", status)
		}
	})

	t.Run("package installed overrides missing configs", func(t *testing.T) {
		app := ApplicationItem{
			PkgInstalled: &installed,
			SubItems: []SubEntryItem{
				{State: StateMissing},
			},
		}

		status := getApplicationStatus(app)
		if status != StatusInstalled {
			t.Errorf("Expected StatusInstalled, got %s", status)
		}
	})

	t.Run("package installed with linked configs", func(t *testing.T) {
		app := ApplicationItem{
			PkgInstalled: &installed,
			SubItems: []SubEntryItem{
				{State: StateLinked},
			},
		}

		status := getApplicationStatus(app)
		if status != StatusInstalled {
			t.Errorf("Expected StatusInstalled, got %s", status)
		}
	})
}

func TestAppInfoMaxState_Outdated(t *testing.T) {
	app := ApplicationItem{
		SubItems: []SubEntryItem{
			{State: StateLinked},
			{State: StateOutdated},
		},
	}
	if got := appInfoMaxState(app); got != StateOutdated {
		t.Errorf("expected StateOutdated, got %v", got)
	}
}

func TestAppInfoMaxState_Unavailable(t *testing.T) {
	app := ApplicationItem{
		SubItems: []SubEntryItem{
			{State: StateModified},
			{State: StateUnavailable},
		},
	}
	if got := appInfoMaxState(app); got != StateUnavailable {
		t.Fatalf("app info state = %v, want StateUnavailable", got)
	}
}

func TestStateOutdated_String(t *testing.T) {
	if StateOutdated.String() != "Outdated" {
		t.Errorf("StateOutdated.String() = %q, want %q", StateOutdated.String(), "Outdated")
	}
}

func TestStateModified_String(t *testing.T) {
	if StateModified.String() != "Modified" {
		t.Errorf("StateModified.String() = %q, want %q", StateModified.String(), "Modified")
	}
}

func TestStateModified_NeedsAttention(t *testing.T) {
	if !needsAttention(StateModified.String()) {
		t.Error("StateModified should need attention")
	}
}

func TestAppInfoMaxState_Modified(t *testing.T) {
	app := ApplicationItem{
		SubItems: []SubEntryItem{
			{State: StateLinked},
			{State: StateModified},
		},
	}
	if got := appInfoMaxState(app); got != StateModified {
		t.Errorf("expected StateModified, got %v", got)
	}
}

func TestStateSetupNeeded_NeedsAttention(t *testing.T) {
	if !needsAttention(StateSetupNeeded.String()) {
		t.Error("StateSetupNeeded should need attention")
	}
}

func TestStateSetupOk_DoesNotNeedAttention(t *testing.T) {
	if needsAttention(StateSetupOk.String()) {
		t.Error("StateSetupOk should not need attention")
	}
}

func TestStateSeverity_SetupStates(t *testing.T) {
	if got, want := stateSeverity(StateSetupNeeded), 3; got != want {
		t.Errorf("stateSeverity(StateSetupNeeded) = %d, want %d (red, action required)", got, want)
	}

	if got, want := stateSeverity(StateSetupOk), 0; got != want {
		t.Errorf("stateSeverity(StateSetupOk) = %d, want %d (no attention)", got, want)
	}
}

func TestUnavailableStateUsesErrorStatusStyle(t *testing.T) {
	row := TableRow{
		Data:            table.Row{"entry", StateUnavailable.String()},
		State:           StateUnavailable,
		StatusAttention: true,
	}

	if got := cellAttentionStyle(row, 1).GetForeground(); !reflect.DeepEqual(got, errorColor) {
		t.Fatalf("Unavailable foreground = %v, want error color %v", got, errorColor)
	}
}

func TestAppInfoMaxState_SetupNeeded(t *testing.T) {
	app := ApplicationItem{
		SubItems: []SubEntryItem{
			{State: StateLinked},
			{State: StateSetupNeeded},
		},
	}
	if got := appInfoMaxState(app); got != StateSetupNeeded {
		t.Errorf("expected StateSetupNeeded, got %v", got)
	}
}

func TestAppInfoMaxState_CheckFailedBeatsOutdated(t *testing.T) {
	app := ApplicationItem{
		SubItems: []SubEntryItem{
			{State: StateOutdated},
			{State: StateCheckFailed},
		},
	}
	if got := appInfoMaxState(app); got != StateCheckFailed {
		t.Errorf("expected StateCheckFailed, got %v", got)
	}
}

func TestFlattenApplications_SetupDiagnosticReplacesGenericInfo(t *testing.T) {
	apps := []ApplicationItem{
		{
			Application: config.Application{Name: "tool"},
			SubItems: []SubEntryItem{{
				SubEntry:   config.SubEntry{Name: "remote-check", Check: map[string]string{"linux": "check"}, Run: map[string]string{"linux": "run"}},
				State:      StateCheckFailed,
				CheckError: "remote unavailable",
			}},
			Expanded: true,
		},
	}

	rows := flattenApplications(apps, "linux", false)
	if len(rows) != 2 {
		t.Fatalf("flattenApplications returned %d rows, want 2", len(rows))
	}
	if got := rows[1].Data[2]; got != "remote unavailable" {
		t.Errorf("setup diagnostic info = %q, want %q", got, "remote unavailable")
	}
}
