package manager

import (
	"path/filepath"
	"reflect"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
)

func TestRestoreCopyTemplatePreflightRejectsUnsafeSelectionsBeforeMutation(t *testing.T) {
	selections := [][]string{
		{"ordinary.conf", "missing.tmpl"},
		{"root", "root.tmpl"},
		{"root.tmpl", "root.tidydots.bak"},
		{"root.tmpl", "root.tmpl.conflict"},
		{"../escape.tmpl"},
		{".tmpl"},
	}

	for _, files := range selections {
		t.Run(filepath.Join(files...), func(t *testing.T) {
			backup, target, mgr, store := setupTemplateTest(t)
			writeTemplateFile(t, filepath.Join(backup, "ordinary.conf"), "ordinary source")
			writeTemplateFile(t, filepath.Join(backup, "root"), "literal source")
			writeTemplateFile(t, filepath.Join(backup, "root.tmpl"), "template source")
			writeTemplateFile(t, filepath.Join(backup, "root.tmpl.conflict"), "conflict source")
			writeTemplateFile(t, filepath.Join(backup, "root.tidydots.bak"), "backup source")
			writeTemplateFile(t, filepath.Join(target, "ordinary.conf"), "keep ordinary target")

			entry := config.SubEntry{Name: "copy", Method: config.MethodCopy, Backup: backup, Files: files}
			beforeBackup := snapshotTemplateFilesystem(t, backup)
			beforeTarget := snapshotTemplateFilesystem(t, target)
			beforeHistory, err := store.GetRenderHistory(mgr.ctx, "root.tmpl", 10)
			if err != nil {
				t.Fatalf("history before: %v", err)
			}
			if err := mgr.RestoreFiles(entry, backup, target); err == nil {
				t.Fatal("unsafe copy selection was accepted")
			}
			if after := snapshotTemplateFilesystem(t, backup); !reflect.DeepEqual(after, beforeBackup) {
				t.Fatalf("backup changed during rejected preflight: before=%v after=%v", beforeBackup, after)
			}
			if after := snapshotTemplateFilesystem(t, target); !reflect.DeepEqual(after, beforeTarget) {
				t.Fatalf("target changed during rejected preflight: before=%v after=%v", beforeTarget, after)
			}
			afterHistory, err := store.GetRenderHistory(mgr.ctx, "root.tmpl", 10)
			if err != nil {
				t.Fatalf("history after: %v", err)
			}
			if !reflect.DeepEqual(afterHistory, beforeHistory) {
				t.Fatalf("history changed during rejected preflight: before=%v after=%v", beforeHistory, afterHistory)
			}
		})
	}
}
