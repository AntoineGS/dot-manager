package tui

import (
	"os"
	"reflect"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
)

func TestConfigMutationDryRun(t *testing.T) {
	for _, operation := range []string{"delete-app", "delete-entry", "batch-delete", "add-app", "edit-app", "add-entry", "edit-entry"} {
		t.Run(operation, func(t *testing.T) {
			m, _ := modelOnDisk(t, deleteProbeConfig())
			m.DryRun = true
			before, err := os.ReadFile(m.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			var mutationErr error
			switch operation {
			case "delete-app":
				mutationErr = m.deleteApplication(0)
			case "delete-entry":
				mutationErr = m.deleteSubEntry(0, 0)
			case "batch-delete":
				m.selectedApps["zebra"] = true
				msg := m.executeBatchDelete()().(BatchCompleteMsg)
				if len(msg.Results) != 1 || !strings.Contains(msg.Results[0].Message, "Would delete") {
					t.Errorf("results = %+v", msg.Results)
				}
			case "add-app":
				mutationErr = m.saveNewApplication(config.Application{Name: "new"})
			case "edit-app":
				mutationErr = m.saveEditedApplication(0, "changed", "changed", "", nil)
			case "add-entry":
				mutationErr = m.addSubEntryToApp(0, config.SubEntry{Name: "new"})
			case "edit-entry":
				mutationErr = m.updateSubEntry(0, 0, config.SubEntry{Name: "changed"})
			}
			if mutationErr != nil {
				t.Fatal(mutationErr)
			}
			if strings.HasPrefix(operation, "delete-") && (len(m.results) != 1 || !strings.Contains(m.results[0].Message, "Would delete")) {
				t.Errorf("single deletion did not report preview: %+v", m.results)
			}
			if !reflect.DeepEqual(m.Config, deleteProbeConfig()) {
				t.Errorf("dry-run mutated in-memory config: %+v", m.Config)
			}
			after, err := os.ReadFile(m.ConfigPath)
			if err != nil {
				t.Fatal(err)
			}
			if string(after) != string(before) {
				t.Error("dry-run changed config file")
			}
		})
	}
}

func TestDeleteLastEntryRetainsPackage(t *testing.T) {
	cfg := deleteProbeConfig()
	cfg.Applications[0].Package = &config.EntryPackage{}
	m, _ := modelOnDisk(t, cfg)
	if err := m.deleteSubEntry(0, 0); err != nil {
		t.Fatal(err)
	}
	if len(cfg.Applications) != 2 || cfg.Applications[0].Name != "zebra" || cfg.Applications[0].Package == nil || len(cfg.Applications[0].Entries) != 0 {
		t.Fatalf("package application removed: %+v", cfg.Applications)
	}
}

func TestDeleteSaveFailureLeavesConfigUntouched(t *testing.T) {
	m, _ := modelOnDisk(t, deleteProbeConfig())
	m.ConfigPath = t.TempDir()
	if err := m.deleteApplication(0); err == nil {
		t.Fatal("expected save failure")
	}
	if !reflect.DeepEqual(m.Config, deleteProbeConfig()) {
		t.Fatal("failed save mutated config")
	}
}
