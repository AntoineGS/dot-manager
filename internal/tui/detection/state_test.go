package detection

import (
	"os"
	"path/filepath"
	"testing"

	tmpl "github.com/AntoineGS/tidydots/internal/template"
	tuitable "github.com/AntoineGS/tidydots/internal/tui/table"
)

// mkSymlink creates a symlink at dst pointing to src.
func mkSymlink(t *testing.T, src, dst string) {
	t.Helper()
	if err := os.Symlink(src, dst); err != nil {
		t.Fatalf("os.Symlink(%q, %q): %v", src, dst, err)
	}
}

// mkDir creates a directory (including parents).
func mkDir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("os.MkdirAll(%q): %v", path, err)
	}
}

// mkFile creates a file with empty content.
func mkFile(t *testing.T, path string) {
	t.Helper()
	mkDir(t, filepath.Dir(path))
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("os.Create(%q): %v", path, err)
	}
	f.Close()
}

// ── Folder-based tests ──────────────────────────────────────────────────────

func TestDetectConfigState_Folder_Linked(t *testing.T) {
	// targetPath is a symlink → StateLinked
	tmp := t.TempDir()
	backupPath := filepath.Join(tmp, "backup")
	targetPath := filepath.Join(tmp, "target_link")

	mkDir(t, backupPath)
	mkSymlink(t, backupPath, targetPath)

	got := DetectConfigState(backupPath, targetPath, true, nil, false)
	if got != tuitable.StateLinked {
		t.Errorf("folder symlink → want StateLinked, got %v", got)
	}
}

func TestDetectConfigState_Folder_Ready(t *testing.T) {
	// backup exists, target is NOT a symlink → StateReady
	tmp := t.TempDir()
	backupPath := filepath.Join(tmp, "backup")
	targetPath := filepath.Join(tmp, "target")

	mkDir(t, backupPath)
	// target does not exist — backup exists → Ready
	got := DetectConfigState(backupPath, targetPath, true, nil, false)
	if got != tuitable.StateReady {
		t.Errorf("folder backup-only → want StateReady, got %v", got)
	}
}

func TestDetectConfigState_Folder_ReadyWithExistingTarget(t *testing.T) {
	// backup exists AND target exists (real dir, not symlink) → StateReady (backup wins)
	tmp := t.TempDir()
	backupPath := filepath.Join(tmp, "backup")
	targetPath := filepath.Join(tmp, "target")

	mkDir(t, backupPath)
	mkDir(t, targetPath)

	got := DetectConfigState(backupPath, targetPath, true, nil, false)
	if got != tuitable.StateReady {
		t.Errorf("folder backup+target → want StateReady, got %v", got)
	}
}

func TestDetectConfigState_Folder_Adopt(t *testing.T) {
	// no backup, target exists as real directory → StateAdopt
	tmp := t.TempDir()
	backupPath := filepath.Join(tmp, "backup_missing")
	targetPath := filepath.Join(tmp, "target")

	mkDir(t, targetPath)

	got := DetectConfigState(backupPath, targetPath, true, nil, false)
	if got != tuitable.StateAdopt {
		t.Errorf("folder target-only → want StateAdopt, got %v", got)
	}
}

func TestDetectConfigState_Folder_Missing(t *testing.T) {
	// neither backup nor target exists → StateMissing
	tmp := t.TempDir()
	backupPath := filepath.Join(tmp, "backup_missing")
	targetPath := filepath.Join(tmp, "target_missing")

	got := DetectConfigState(backupPath, targetPath, true, nil, false)
	if got != tuitable.StateMissing {
		t.Errorf("folder nothing → want StateMissing, got %v", got)
	}
}

// ── File-based tests ────────────────────────────────────────────────────────

func TestDetectConfigState_Files_Linked(t *testing.T) {
	// All files in backupPath are symlinked from targetPath → StateLinked
	tmp := t.TempDir()
	backupPath := filepath.Join(tmp, "backup")
	targetPath := filepath.Join(tmp, "target")

	mkDir(t, backupPath)
	mkDir(t, targetPath)

	files := []string{".bashrc", ".zshrc"}
	for _, f := range files {
		src := filepath.Join(backupPath, f)
		dst := filepath.Join(targetPath, f)
		mkFile(t, src)
		mkSymlink(t, src, dst)
	}

	got := DetectConfigState(backupPath, targetPath, false, files, false)
	if got != tuitable.StateLinked {
		t.Errorf("files all-symlinked → want StateLinked, got %v", got)
	}
}

func TestDetectConfigState_Files_Ready(t *testing.T) {
	// backup files exist, target files do not exist → StateReady
	tmp := t.TempDir()
	backupPath := filepath.Join(tmp, "backup")
	targetPath := filepath.Join(tmp, "target")

	mkDir(t, backupPath)
	mkDir(t, targetPath)

	files := []string{".bashrc"}
	mkFile(t, filepath.Join(backupPath, files[0]))
	// targetPath/.bashrc intentionally absent

	got := DetectConfigState(backupPath, targetPath, false, files, false)
	if got != tuitable.StateReady {
		t.Errorf("files backup-only → want StateReady, got %v", got)
	}
}

func TestDetectConfigState_Files_NoBackupTargetOnly(t *testing.T) {
	// File-based: backup files do NOT exist, only target files exist.
	// The loop skips files with no backup, so anyBackup=false and anyTarget=false.
	// Result is StateMissing (StateAdopt is unreachable for file-based without a backup).
	tmp := t.TempDir()
	backupPath := filepath.Join(tmp, "backup")
	targetPath := filepath.Join(tmp, "target")

	mkDir(t, backupPath)
	mkDir(t, targetPath)

	files := []string{".bashrc"}
	// backup file is absent; only target file exists
	mkFile(t, filepath.Join(targetPath, files[0]))

	got := DetectConfigState(backupPath, targetPath, false, files, false)
	if got != tuitable.StateMissing {
		t.Errorf("files target-only (no backup) → want StateMissing, got %v", got)
	}
}

func TestDetectConfigState_Files_Missing(t *testing.T) {
	// neither backup nor target files exist → StateMissing
	tmp := t.TempDir()
	backupPath := filepath.Join(tmp, "backup")
	targetPath := filepath.Join(tmp, "target")

	mkDir(t, backupPath)
	mkDir(t, targetPath)

	files := []string{".bashrc"}
	// no files created at all

	got := DetectConfigState(backupPath, targetPath, false, files, false)
	if got != tuitable.StateMissing {
		t.Errorf("files nothing → want StateMissing, got %v", got)
	}
}

func TestDetectConfigState_Files_EmptyFileList(t *testing.T) {
	// empty files list with isFolder=false → StateMissing (no files checked)
	tmp := t.TempDir()
	backupPath := filepath.Join(tmp, "backup")
	targetPath := filepath.Join(tmp, "target")

	mkDir(t, backupPath)

	got := DetectConfigState(backupPath, targetPath, false, []string{}, false)
	if got != tuitable.StateMissing {
		t.Errorf("files empty list → want StateMissing, got %v", got)
	}
}

func TestDetectConfigState_Files_PartialSymlinks(t *testing.T) {
	// Some files symlinked, some only in backup → StateReady (not all linked)
	tmp := t.TempDir()
	backupPath := filepath.Join(tmp, "backup")
	targetPath := filepath.Join(tmp, "target")

	mkDir(t, backupPath)
	mkDir(t, targetPath)

	files := []string{".bashrc", ".zshrc"}

	// .bashrc: backup only
	mkFile(t, filepath.Join(backupPath, files[0]))

	// .zshrc: backup + symlinked
	src := filepath.Join(backupPath, files[1])
	dst := filepath.Join(targetPath, files[1])
	mkFile(t, src)
	mkSymlink(t, src, dst)

	got := DetectConfigState(backupPath, targetPath, false, files, false)
	if got != tuitable.StateReady {
		t.Errorf("files partial-symlinks → want StateReady, got %v", got)
	}
}

func TestDetectConfigState_Files_NonSymlinkTarget(t *testing.T) {
	// backup file exists, target file exists but is a real file (not a symlink) → StateReady
	// (because backup exists and takes priority over anyTarget check)
	tmp := t.TempDir()
	backupPath := filepath.Join(tmp, "backup")
	targetPath := filepath.Join(tmp, "target")

	mkDir(t, backupPath)
	mkDir(t, targetPath)

	files := []string{".vimrc"}
	mkFile(t, filepath.Join(backupPath, files[0]))
	mkFile(t, filepath.Join(targetPath, files[0])) // real file, not symlink

	got := DetectConfigState(backupPath, targetPath, false, files, false)
	if got != tuitable.StateReady {
		t.Errorf("files real-file-target with backup → want StateReady, got %v", got)
	}
}

func TestDetectConfigState_SelectedTemplate_UsesSuffixFreeTarget(t *testing.T) {
	tmp := t.TempDir()
	backupPath := filepath.Join(tmp, "backup")
	targetPath := filepath.Join(tmp, "target")
	mkDir(t, backupPath)
	mkDir(t, targetPath)

	templatePath := filepath.Join(backupPath, "config.tmpl")
	renderedPath := tmpl.RenderedPath(templatePath)
	aliasPath := filepath.Join(backupPath, "config")
	mkFile(t, templatePath)
	mkFile(t, renderedPath)
	mkSymlink(t, renderedPath, aliasPath)
	mkSymlink(t, aliasPath, filepath.Join(targetPath, "config"))

	got := DetectConfigState(backupPath, targetPath, false, []string{"config.tmpl"}, false)
	if got != tuitable.StateLinked {
		t.Errorf("selected template chain = %v, want StateLinked", got)
	}
}

func TestDetectConfigState_SelectedTemplate_WrongTargetChainIsNotLinked(t *testing.T) {
	tmp := t.TempDir()
	backupPath := filepath.Join(tmp, "backup")
	targetPath := filepath.Join(tmp, "target")
	mkDir(t, backupPath)
	mkDir(t, targetPath)

	mkFile(t, filepath.Join(backupPath, "config.tmpl"))
	mkFile(t, filepath.Join(backupPath, "other"))
	mkSymlink(t, filepath.Join(backupPath, "other"), filepath.Join(targetPath, "config"))

	got := DetectConfigState(backupPath, targetPath, false, []string{"config.tmpl"}, false)
	if got == tuitable.StateLinked {
		t.Fatal("wrong selected template target chain was reported healthy")
	}
}

func TestDetectConfigState_SelectedTemplateCollisionIsNotLinked(t *testing.T) {
	tmp := t.TempDir()
	backupPath := filepath.Join(tmp, "backup")
	targetPath := filepath.Join(tmp, "target")
	mkDir(t, backupPath)
	mkDir(t, targetPath)

	templatePath := filepath.Join(backupPath, "config.tmpl")
	renderedPath := tmpl.RenderedPath(templatePath)
	aliasPath := filepath.Join(backupPath, "config")
	mkFile(t, templatePath)
	mkFile(t, renderedPath)
	// This is a link layout that makes both selections look linked
	// independently, even though restore rejects the shared effective name.
	mkSymlink(t, renderedPath, aliasPath)
	mkSymlink(t, aliasPath, filepath.Join(targetPath, "config"))

	got := DetectConfigState(backupPath, targetPath, false, []string{"config", "config.tmpl"}, false)
	if got == tuitable.StateLinked {
		t.Fatal("ambiguous literal/template selections were reported healthy")
	}
}

func TestDetectConfigState_SelectedTemplateGeneratedCollisionIsNotLinked(t *testing.T) {
	tmp := t.TempDir()
	backupPath := filepath.Join(tmp, "backup")
	targetPath := filepath.Join(tmp, "target")
	mkDir(t, backupPath)
	mkDir(t, targetPath)

	firstTemplate := filepath.Join(backupPath, "config.tmpl")
	secondTemplate := filepath.Join(backupPath, "config.tmpl.rendered.tmpl")
	firstRendered := tmpl.RenderedPath(firstTemplate)
	secondRendered := tmpl.RenderedPath(secondTemplate)
	firstAlias := filepath.Join(backupPath, "config")
	mkFile(t, firstTemplate)
	mkFile(t, secondTemplate)
	mkFile(t, secondRendered)
	// The first template's rendered path is also the second template's
	// suffix-free alias. Both individual chains look correct, but restore's
	// generated-path preflight rejects the cross-selection collision.
	mkSymlink(t, secondRendered, firstRendered)
	mkSymlink(t, firstRendered, firstAlias)
	mkSymlink(t, firstAlias, filepath.Join(targetPath, "config"))
	mkSymlink(t, firstRendered, filepath.Join(targetPath, "config.tmpl.rendered"))

	got := DetectConfigState(backupPath, targetPath, false, []string{
		"config.tmpl",
		"config.tmpl.rendered.tmpl",
	}, false)
	if got == tuitable.StateLinked {
		t.Fatal("template-generated cross-selection collision was reported healthy")
	}
}

func TestDetectConfigState_SelectedTemplateRejectsEscapingSelection(t *testing.T) {
	tmp := t.TempDir()
	backupPath := filepath.Join(tmp, "backup")
	targetRoot := filepath.Join(tmp, "target-root")
	targetPath := filepath.Join(targetRoot, "entry")
	mkDir(t, backupPath)
	mkDir(t, targetPath)

	templatePath := filepath.Join(tmp, "neighbor.tmpl")
	renderedPath := tmpl.RenderedPath(templatePath)
	aliasPath := filepath.Join(tmp, "neighbor")
	mkFile(t, templatePath)
	mkFile(t, renderedPath)
	mkSymlink(t, renderedPath, aliasPath)
	mkSymlink(t, aliasPath, filepath.Join(targetRoot, "neighbor"))

	got := DetectConfigState(backupPath, targetPath, false, []string{"../neighbor.tmpl"}, false)
	if got == tuitable.StateLinked {
		t.Fatal("selected template outside the backup entry was reported healthy")
	}
}

func TestDetectConfigState_SelectedTemplateRejectsSymlinkParentEscapes(t *testing.T) {
	tests := []struct {
		name         string
		sourceParent bool
		targetParent bool
	}{
		{name: "source parent", sourceParent: true},
		{name: "target parent", targetParent: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			backupPath := filepath.Join(tmp, "backup")
			targetPath := filepath.Join(tmp, "target")
			sourceOutside := filepath.Join(tmp, "source-outside")
			targetOutside := filepath.Join(tmp, "target-outside")
			mkDir(t, backupPath)
			mkDir(t, targetPath)
			mkDir(t, sourceOutside)
			mkDir(t, targetOutside)

			if tt.sourceParent {
				mkSymlink(t, sourceOutside, filepath.Join(backupPath, "nested"))
			} else {
				mkDir(t, filepath.Join(backupPath, "nested"))
			}
			if tt.targetParent {
				mkSymlink(t, targetOutside, filepath.Join(targetPath, "nested"))
			} else {
				mkDir(t, filepath.Join(targetPath, "nested"))
			}

			sourceDir := filepath.Join(backupPath, "nested")
			if tt.sourceParent {
				sourceDir = sourceOutside
			}
			targetDir := filepath.Join(targetPath, "nested")
			if tt.targetParent {
				targetDir = targetOutside
			}
			templatePath := filepath.Join(sourceDir, "config.tmpl")
			mkFile(t, templatePath)
			mkFile(t, tmpl.RenderedPath(templatePath))

			aliasPath := filepath.Join(backupPath, "nested", "config")
			mkSymlink(t, filepath.Join(backupPath, "nested", "config.tmpl.rendered"), filepath.Join(sourceDir, "config"))
			mkSymlink(t, aliasPath, filepath.Join(targetDir, "config"))

			got := DetectConfigState(backupPath, targetPath, false, []string{"nested/config.tmpl"}, false)
			if got == tuitable.StateLinked {
				t.Fatalf("selected template through %s was reported healthy", tt.name)
			}
		})
	}
}

// ── Copy-mode tests ─────────────────────────────────────────────────────────

func TestDetectConfigState_MixedTemplatesNeverReportsInvalidSelectionAsLinked(t *testing.T) {
	tests := []struct {
		name       string
		file       string
		setupOther func(t *testing.T, backupPath, targetPath string)
	}{
		{
			name: "escaping selection",
			file: "../neighbor.tmpl",
			setupOther: func(t *testing.T, backupPath, targetPath string) {
				tmp := filepath.Dir(backupPath)
				templatePath := filepath.Join(tmp, "neighbor.tmpl")
				renderedPath := tmpl.RenderedPath(templatePath)
				aliasPath := filepath.Join(tmp, "neighbor")
				targetRoot := filepath.Dir(targetPath)
				mkFile(t, templatePath)
				mkFile(t, renderedPath)
				mkSymlink(t, renderedPath, aliasPath)
				mkSymlink(t, aliasPath, filepath.Join(targetRoot, "neighbor"))
			},
		},
		{
			name: "missing source",
			file: "missing.tmpl",
		},
		{
			name: "nonregular source",
			file: "nonregular.tmpl",
			setupOther: func(t *testing.T, backupPath, targetPath string) {
				sourcePath := filepath.Join(backupPath, "nonregular.tmpl")
				renderedPath := tmpl.RenderedPath(sourcePath)
				aliasPath := filepath.Join(backupPath, "nonregular")
				mkDir(t, sourcePath)
				mkFile(t, renderedPath)
				mkSymlink(t, renderedPath, aliasPath)
				mkSymlink(t, aliasPath, filepath.Join(targetPath, "nonregular"))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmp := t.TempDir()
			backupPath := filepath.Join(tmp, "backup")
			targetRoot := filepath.Join(tmp, "target-root")
			targetPath := filepath.Join(targetRoot, "entry")
			mkDir(t, backupPath)
			mkDir(t, targetPath)

			validTemplatePath := filepath.Join(backupPath, "valid.tmpl")
			validRenderedPath := tmpl.RenderedPath(validTemplatePath)
			validAliasPath := filepath.Join(backupPath, "valid")
			mkFile(t, validTemplatePath)
			mkFile(t, validRenderedPath)
			mkSymlink(t, validRenderedPath, validAliasPath)
			mkSymlink(t, validAliasPath, filepath.Join(targetPath, "valid"))
			if tt.setupOther != nil {
				tt.setupOther(t, backupPath, targetPath)
			}

			files := []string{"valid.tmpl", tt.file}
			got := DetectConfigState(backupPath, targetPath, false, files, false)
			if got == tuitable.StateLinked {
				t.Fatalf("valid template plus %s was reported healthy", tt.name)
			}
		})
	}
}

func TestDetectConfigState_Copy_InSync(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	backup := filepath.Join(dir, "backup")
	target := filepath.Join(dir, "target")
	_ = os.MkdirAll(backup, 0755)
	_ = os.MkdirAll(target, 0755)
	_ = os.WriteFile(filepath.Join(backup, "f"), []byte("x"), 0644)
	_ = os.WriteFile(filepath.Join(target, "f"), []byte("x"), 0644)

	got := DetectConfigState(backup, target, false, []string{"f"}, true)
	if got != tuitable.StateLinked {
		t.Errorf("state = %v, want StateLinked (in sync)", got)
	}
}

func TestDetectConfigState_Copy_Drift(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	backup := filepath.Join(dir, "backup")
	target := filepath.Join(dir, "target")
	_ = os.MkdirAll(backup, 0755)
	_ = os.MkdirAll(target, 0755)
	_ = os.WriteFile(filepath.Join(backup, "f"), []byte("new"), 0644)
	_ = os.WriteFile(filepath.Join(target, "f"), []byte("old"), 0644)

	got := DetectConfigState(backup, target, false, []string{"f"}, true)
	if got != tuitable.StateReady {
		t.Errorf("state = %v, want StateReady (drift)", got)
	}
}

func TestDetectConfigState_Copy_TargetMissing(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	backup := filepath.Join(dir, "backup")
	target := filepath.Join(dir, "target")
	_ = os.MkdirAll(backup, 0755)
	_ = os.WriteFile(filepath.Join(backup, "f"), []byte("x"), 0644)

	got := DetectConfigState(backup, target, false, []string{"f"}, true)
	if got != tuitable.StateReady {
		t.Errorf("state = %v, want StateReady (backup present, target absent)", got)
	}
}

func TestDetectConfigState_Copy_TargetStillSymlink(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	backup := filepath.Join(dir, "backup")
	target := filepath.Join(dir, "target")
	_ = os.MkdirAll(backup, 0755)
	_ = os.MkdirAll(target, 0755)
	src := filepath.Join(backup, "f")
	_ = os.WriteFile(src, []byte("x"), 0644)
	// Stale symlink left by a previous symlink-mode deployment.
	if err := os.Symlink(src, filepath.Join(target, "f")); err != nil {
		t.Fatalf("symlink: %v", err)
	}

	got := DetectConfigState(backup, target, false, []string{"f"}, true)
	if got != tuitable.StateReady {
		t.Errorf("state = %v, want StateReady (a symlinked target must not report in sync)", got)
	}
}

func TestDetectConfigState_Copy_TemplateNameRemainsLiteral(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	backup := filepath.Join(dir, "backup")
	target := filepath.Join(dir, "target")
	_ = os.MkdirAll(backup, 0o755)
	_ = os.MkdirAll(target, 0o755)
	_ = os.WriteFile(filepath.Join(backup, "config.tmpl"), []byte("literal"), 0o644)
	_ = os.WriteFile(filepath.Join(target, "config.tmpl"), []byte("literal"), 0o644)

	got := DetectConfigState(backup, target, false, []string{"config.tmpl"}, true)
	if got != tuitable.StateLinked {
		t.Errorf("copy template-name state = %v, want StateLinked", got)
	}
}
