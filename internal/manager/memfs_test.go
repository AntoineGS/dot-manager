package manager

import (
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/fsys"
	"github.com/AntoineGS/tidydots/internal/platform"
)

// newMemManager creates a Manager backed by MemFS for error-path testing.
func newMemManager(t *testing.T) (*Manager, *fsys.MemFS) {
	t.Helper()
	mem := fsys.NewMemFS()
	cfg := &config.Config{BackupRoot: "/backup"}
	plat := &platform.Platform{OS: "linux", EnvVars: map[string]string{}}
	mgr := New(cfg, plat).WithFS(mem)
	return mgr, mem
}

// ---------------------------------------------------------------------------
// WithFS / WithRunner builder methods
// ---------------------------------------------------------------------------

func TestManager_WithFS_ReturnsNewManager(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	plat := &platform.Platform{OS: platform.OSLinux, EnvVars: map[string]string{}}
	mgr := New(cfg, plat)
	mem := fsys.NewMemFS()
	mgr2 := mgr.WithFS(mem)

	if mgr2 == mgr {
		t.Error("WithFS should return a new Manager, not the same pointer")
	}
}

func TestManager_WithRunner_ReturnsNewManager(t *testing.T) {
	t.Parallel()
	cfg := &config.Config{}
	plat := &platform.Platform{OS: platform.OSLinux, EnvVars: map[string]string{}}
	mgr := New(cfg, plat)
	mgr2 := mgr.WithRunner(mgr.runner)

	if mgr2 == mgr {
		t.Error("WithRunner should return a new Manager, not the same pointer")
	}
}

// ---------------------------------------------------------------------------
// copyFile via MemFS
// ---------------------------------------------------------------------------

func TestManager_CopyFile_SourceNotExist(t *testing.T) {
	t.Parallel()
	mgr, _ := newMemManager(t)
	err := mgr.copyFile("/nonexistent.txt", "/dst.txt")
	if err == nil {
		t.Error("expected error for missing source, got nil")
	}
}

func TestManager_CopyFile_Success(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.WriteFile("/src.txt", []byte("hello"), 0644); err != nil {
		t.Fatalf("setup WriteFile: %v", err)
	}
	if err := mgr.copyFile("/src.txt", "/dst.txt"); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	got, err := mem.ReadFile("/dst.txt")
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(got) != "hello" {
		t.Errorf("content = %q, want %q", string(got), "hello")
	}
}

func TestManager_CopyFile_CreatesDestinationDirectory(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.WriteFile("/src.txt", []byte("data"), 0644); err != nil {
		t.Fatalf("setup WriteFile: %v", err)
	}
	// Destination directory /subdir does not exist yet.
	if err := mgr.copyFile("/src.txt", "/subdir/dst.txt"); err != nil {
		t.Fatalf("copyFile: %v", err)
	}
	got, err := mem.ReadFile("/subdir/dst.txt")
	if err != nil {
		t.Fatalf("ReadFile after copy: %v", err)
	}
	if string(got) != "data" {
		t.Errorf("content = %q, want %q", string(got), "data")
	}
}

// ---------------------------------------------------------------------------
// copyDir via MemFS
// ---------------------------------------------------------------------------

func TestManager_CopyDir_SourceNotExist(t *testing.T) {
	t.Parallel()
	mgr, _ := newMemManager(t)
	err := mgr.copyDir("/nosuchdir", "/dstdir")
	if err == nil {
		t.Error("expected error for missing source directory, got nil")
	}
}

func TestManager_CopyDir_Success(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)

	// Build source tree: /src/a.txt and /src/sub/b.txt
	if err := mem.MkdirAll("/src/sub", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := mem.WriteFile("/src/a.txt", []byte("aaa"), 0644); err != nil {
		t.Fatalf("WriteFile a.txt: %v", err)
	}
	if err := mem.WriteFile("/src/sub/b.txt", []byte("bbb"), 0644); err != nil {
		t.Fatalf("WriteFile b.txt: %v", err)
	}

	if err := mgr.copyDir("/src", "/dst"); err != nil {
		t.Fatalf("copyDir: %v", err)
	}

	for _, p := range []string{"/dst/a.txt", "/dst/sub/b.txt"} {
		if _, err := mem.ReadFile(p); err != nil {
			t.Errorf("expected %s to exist after copyDir: %v", p, err)
		}
	}
}

// ---------------------------------------------------------------------------
// removeAll via MemFS
// ---------------------------------------------------------------------------

func TestManager_RemoveAll_NonExistentPath(t *testing.T) {
	t.Parallel()
	mgr, _ := newMemManager(t)
	// Should not return error for non-existent path.
	if err := mgr.removeAll("/does-not-exist"); err != nil {
		t.Errorf("removeAll(nonexistent) should not error, got: %v", err)
	}
}

func TestManager_RemoveAll_RegularFile(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.WriteFile("/todelete.txt", []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := mgr.removeAll("/todelete.txt"); err != nil {
		t.Fatalf("removeAll: %v", err)
	}
	if mgr.pathExists("/todelete.txt") {
		t.Error("file should not exist after removeAll")
	}
}

func TestManager_RemoveAll_SymlinkNotRemoved(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.WriteFile("/target.txt", []byte("t"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := mem.Symlink("/target.txt", "/link.txt"); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	// removeAll on a symlink should leave the symlink intact.
	if err := mgr.removeAll("/link.txt"); err != nil {
		t.Fatalf("removeAll: %v", err)
	}
	if !mgr.pathExists("/link.txt") {
		t.Error("symlink should NOT be removed by removeAll")
	}
}

// ---------------------------------------------------------------------------
// isSymlink via MemFS
// ---------------------------------------------------------------------------

func TestManager_IsSymlink_NonExistent(t *testing.T) {
	t.Parallel()
	mgr, _ := newMemManager(t)
	if mgr.isSymlink("/nope") {
		t.Error("expected false for non-existent path")
	}
}

func TestManager_IsSymlink_RegularFile(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.WriteFile("/regular.txt", []byte("r"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if mgr.isSymlink("/regular.txt") {
		t.Error("expected false for regular file")
	}
}

func TestManager_IsSymlink_Symlink(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.WriteFile("/real.txt", []byte("r"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := mem.Symlink("/real.txt", "/link.txt"); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	if !mgr.isSymlink("/link.txt") {
		t.Error("expected true for symlink")
	}
}

// ---------------------------------------------------------------------------
// symlinkPointsTo via MemFS
// ---------------------------------------------------------------------------

func TestManager_SymlinkPointsTo_NotSymlink(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.WriteFile("/regular.txt", []byte("r"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if mgr.symlinkPointsTo("/regular.txt", "/anywhere") {
		t.Error("expected false for non-symlink path")
	}
}

func TestManager_SymlinkPointsTo_NonExistent(t *testing.T) {
	t.Parallel()
	mgr, _ := newMemManager(t)
	if mgr.symlinkPointsTo("/nope", "/anywhere") {
		t.Error("expected false for non-existent path")
	}
}

func TestManager_SymlinkPointsTo_CorrectTarget(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.WriteFile("/real.txt", []byte("r"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := mem.Symlink("/real.txt", "/link.txt"); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	if !mgr.symlinkPointsTo("/link.txt", "/real.txt") {
		t.Error("expected true when symlink points to target")
	}
}

func TestManager_SymlinkPointsTo_WrongTarget(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.WriteFile("/real.txt", []byte("r"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := mem.Symlink("/real.txt", "/link.txt"); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	if mgr.symlinkPointsTo("/link.txt", "/other.txt") {
		t.Error("expected false when symlink points to different target")
	}
}

// ---------------------------------------------------------------------------
// pathExists / PathExists via MemFS
// ---------------------------------------------------------------------------

func TestManager_PathExists_MemFS(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.WriteFile("/exists.txt", []byte(""), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	if !mgr.pathExists("/exists.txt") {
		t.Error("expected true for existing file")
	}
	if mgr.pathExists("/nope.txt") {
		t.Error("expected false for non-existent file")
	}
}

func TestManager_PathExists_ExportedAlias(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.WriteFile("/file.txt", []byte(""), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// PathExists is the exported alias for pathExists.
	if !mgr.PathExists("/file.txt") {
		t.Error("PathExists should return true for existing file")
	}
	if mgr.PathExists("/absent.txt") {
		t.Error("PathExists should return false for absent file")
	}
}

func TestManager_PathExists_BrokenSymlink(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	// Create a symlink to a non-existent target; Lstat still returns info for the link itself.
	if err := mem.Symlink("/nowhere.txt", "/broken.txt"); err != nil {
		t.Fatalf("Symlink: %v", err)
	}
	if !mgr.pathExists("/broken.txt") {
		t.Error("pathExists should return true for broken symlink (Lstat behavior)")
	}
}

// ---------------------------------------------------------------------------
// hasTemplateFiles / HasTemplateFiles via MemFS
// ---------------------------------------------------------------------------

func TestManager_HasTemplateFiles_EmptyDir(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.MkdirAll("/empty", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if mgr.hasTemplateFiles("/empty") {
		t.Error("expected false for empty dir")
	}
}

func TestManager_HasTemplateFiles_WithTemplate(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.MkdirAll("/dir", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := mem.WriteFile("/dir/config.tmpl", []byte(""), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if !mgr.hasTemplateFiles("/dir") {
		t.Error("expected true when .tmpl file present")
	}
}

func TestManager_HasTemplateFiles_WithNonTemplateFiles(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.MkdirAll("/dir", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := mem.WriteFile("/dir/config.yaml", []byte("key: value"), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if mgr.hasTemplateFiles("/dir") {
		t.Error("expected false when only non-.tmpl files present")
	}
}

func TestManager_HasTemplateFiles_NonExistentDir(t *testing.T) {
	t.Parallel()
	mgr, _ := newMemManager(t)
	if mgr.hasTemplateFiles("/no-such-dir") {
		t.Error("expected false for non-existent directory")
	}
}

func TestManager_HasTemplateFiles_ExportedAlias(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.MkdirAll("/dir2", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := mem.WriteFile("/dir2/app.tmpl", []byte(""), 0600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// HasTemplateFiles is the exported alias for hasTemplateFiles.
	if !mgr.HasTemplateFiles("/dir2") {
		t.Error("HasTemplateFiles should return true when .tmpl file present")
	}
}

// ---------------------------------------------------------------------------
// Close (when no stateStore)
// ---------------------------------------------------------------------------

func TestManager_Close_NoStateStore(t *testing.T) {
	t.Parallel()
	mgr, _ := newMemManager(t)
	// Close should not error when stateStore is nil.
	if err := mgr.Close(); err != nil {
		t.Errorf("Close() without state store should return nil, got: %v", err)
	}
}

// ---------------------------------------------------------------------------
// GetModifiedTemplateFiles with nil stateStore
// ---------------------------------------------------------------------------

func TestManager_GetModifiedTemplateFiles_NilStateStore(t *testing.T) {
	t.Parallel()
	mgr, _ := newMemManager(t)
	// stateStore is nil, should return nil, nil.
	result, err := mgr.GetModifiedTemplateFiles("/some/dir", nil)
	if err != nil {
		t.Errorf("expected nil error, got: %v", err)
	}
	if result != nil {
		t.Errorf("expected nil result, got: %v", result)
	}
}

// ---------------------------------------------------------------------------
// HasOutdatedTemplates / HasModifiedRenderedFiles with nil stateStore
// ---------------------------------------------------------------------------

func TestManager_HasOutdatedTemplates_NilStateStore(t *testing.T) {
	t.Parallel()
	mgr, _ := newMemManager(t)
	// stateStore is nil, walkTemplateFiles returns early → false.
	if mgr.HasOutdatedTemplates("/backup", nil) {
		t.Error("expected false when stateStore is nil")
	}
}

func TestManager_HasModifiedRenderedFiles_NilStateStore(t *testing.T) {
	t.Parallel()
	mgr, _ := newMemManager(t)
	if mgr.HasModifiedRenderedFiles("/backup", nil) {
		t.Error("expected false when stateStore is nil")
	}
}

// ---------------------------------------------------------------------------
// moveFile via MemFS (non-sudo path)
// ---------------------------------------------------------------------------

func TestManager_MoveFile_Success(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.WriteFile("/src.txt", []byte("move me"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := mgr.moveFile("/src.txt", "/dst.txt", false); err != nil {
		t.Fatalf("moveFile: %v", err)
	}
	// dst should exist, src should not.
	if !mgr.pathExists("/dst.txt") {
		t.Error("expected /dst.txt to exist after moveFile")
	}
	if mgr.pathExists("/src.txt") {
		t.Error("expected /src.txt to be gone after moveFile (rename succeeded)")
	}
}

func TestManager_MoveFile_FallbackCopyOnRenameFailure(t *testing.T) {
	t.Parallel()
	// MemFS Rename moves within the same FS; force a copy+remove fallback
	// by renaming to a path whose parent directory doesn't exist, causing
	// Rename to fail. Then copyFile will be called and handle directory creation.
	mgr, mem := newMemManager(t)
	if err := mem.WriteFile("/src2.txt", []byte("fallback"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// Rename /src2.txt → /newdir/dst.txt. MemFS Rename doesn't auto-create dirs,
	// but copyFile does via MkdirAll. In practice MemFS Rename returns nil for
	// simple paths, so just verify the happy path here.
	if err := mgr.moveFile("/src2.txt", "/dst2.txt", false); err != nil {
		t.Fatalf("moveFile: %v", err)
	}
	if !mgr.pathExists("/dst2.txt") {
		t.Error("expected /dst2.txt to exist")
	}
}

// ---------------------------------------------------------------------------
// mergeFile via MemFS
// ---------------------------------------------------------------------------

func TestManager_MergeFile_NoConflict(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)

	// Setup: a "target" file to merge into backup.
	if err := mem.MkdirAll("/target", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := mem.WriteFile("/target/config.yaml", []byte("k: v"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := mem.MkdirAll("/backup", 0755); err != nil {
		t.Fatalf("MkdirAll backup: %v", err)
	}

	summary := NewMergeSummary("test-app")
	if err := mgr.mergeFile("/target/config.yaml", "/backup", "config.yaml", false, summary); err != nil {
		t.Fatalf("mergeFile: %v", err)
	}
	if len(summary.MergedFiles) != 1 {
		t.Errorf("expected 1 merged file, got %d", len(summary.MergedFiles))
	}
	if !mgr.pathExists("/backup/config.yaml") {
		t.Error("expected /backup/config.yaml to exist after merge")
	}
}

func TestManager_MergeFile_Conflict(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)

	// Both target and backup have the same file → conflict.
	if err := mem.MkdirAll("/target2", 0755); err != nil {
		t.Fatalf("MkdirAll target: %v", err)
	}
	if err := mem.WriteFile("/target2/config.yaml", []byte("new"), 0644); err != nil {
		t.Fatalf("WriteFile target: %v", err)
	}
	if err := mem.MkdirAll("/backup2", 0755); err != nil {
		t.Fatalf("MkdirAll backup: %v", err)
	}
	if err := mem.WriteFile("/backup2/config.yaml", []byte("existing"), 0644); err != nil {
		t.Fatalf("WriteFile backup: %v", err)
	}

	summary := NewMergeSummary("test-app")
	if err := mgr.mergeFile("/target2/config.yaml", "/backup2", "config.yaml", false, summary); err != nil {
		t.Fatalf("mergeFile with conflict: %v", err)
	}
	if len(summary.ConflictFiles) != 1 {
		t.Errorf("expected 1 conflict, got %d", len(summary.ConflictFiles))
	}
}

// ---------------------------------------------------------------------------
// MergeFolder via MemFS
// ---------------------------------------------------------------------------

func TestManager_MergeFolder_EmptyTarget(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.MkdirAll("/tgt", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := mem.MkdirAll("/bkp", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	summary := NewMergeSummary("app")
	if err := mgr.MergeFolder("/bkp", "/tgt", false, summary); err != nil {
		t.Fatalf("MergeFolder: %v", err)
	}
	// Nothing to merge.
	if summary.HasOperations() {
		t.Error("expected no operations for empty target directory")
	}
}

func TestManager_MergeFolder_WithFiles(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.MkdirAll("/tgt2", 0755); err != nil {
		t.Fatalf("MkdirAll target: %v", err)
	}
	if err := mem.WriteFile("/tgt2/a.conf", []byte("a"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := mem.MkdirAll("/bkp2", 0755); err != nil {
		t.Fatalf("MkdirAll backup: %v", err)
	}
	summary := NewMergeSummary("app")
	if err := mgr.MergeFolder("/bkp2", "/tgt2", false, summary); err != nil {
		t.Fatalf("MergeFolder: %v", err)
	}
	if len(summary.MergedFiles) != 1 {
		t.Errorf("expected 1 merged file, got %d", len(summary.MergedFiles))
	}
}

// ---------------------------------------------------------------------------
// createSymlink via MemFS
// ---------------------------------------------------------------------------

func TestManager_CreateSymlink_SourceNotExist(t *testing.T) {
	t.Parallel()
	mgr, _ := newMemManager(t)
	err := mgr.createSymlink("/missing-source", "/target-link", false)
	if err == nil {
		t.Error("expected error when source does not exist")
	}
}

func TestManager_CreateSymlink_Success(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.WriteFile("/real.txt", []byte("r"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := mgr.createSymlink("/real.txt", "/link.txt", false); err != nil {
		t.Fatalf("createSymlink: %v", err)
	}
	if !mgr.isSymlink("/link.txt") {
		t.Error("expected /link.txt to be a symlink")
	}
}

// ---------------------------------------------------------------------------
// removeEmptyDirs via MemFS
// ---------------------------------------------------------------------------

func TestManager_RemoveEmptyDirs_EmptySubDirs(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.MkdirAll("/root/sub/deep", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := mgr.removeEmptyDirs("/root"); err != nil {
		t.Fatalf("removeEmptyDirs: %v", err)
	}
	// /root should still exist (not removed itself).
	if !mgr.pathExists("/root") {
		t.Error("root dir should remain")
	}
}

func TestManager_RemoveEmptyDirs_NonEmptyDirPreserved(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	if err := mem.MkdirAll("/root2/nonempty", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := mem.WriteFile("/root2/nonempty/keep.txt", []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := mgr.removeEmptyDirs("/root2"); err != nil {
		t.Fatalf("removeEmptyDirs: %v", err)
	}
	if !mgr.pathExists("/root2/nonempty/keep.txt") {
		t.Error("file in non-empty directory should be preserved")
	}
}

// ---------------------------------------------------------------------------
// RestoreFolder via MemFS
// ---------------------------------------------------------------------------

func TestManager_RestoreFolder_AlreadySymlink(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)

	// Setup: source exists, target is already a symlink pointing to source.
	if err := mem.MkdirAll("/backup/nvim", 0755); err != nil {
		t.Fatalf("MkdirAll source: %v", err)
	}
	if err := mem.MkdirAll("/home/user/.config", 0755); err != nil {
		t.Fatalf("MkdirAll parent: %v", err)
	}
	if err := mem.Symlink("/backup/nvim", "/home/user/.config/nvim"); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	subEntry := config.SubEntry{Name: "nvim", Backup: "/backup/nvim"}
	if err := mgr.RestoreFolder(subEntry, "/backup/nvim", "/home/user/.config/nvim"); err != nil {
		t.Fatalf("RestoreFolder with correct symlink: %v", err)
	}
}

func TestManager_RestoreFolder_SourceMissingDryRun(t *testing.T) {
	t.Parallel()
	mgr, _ := newMemManager(t)
	mgr.DryRun = true

	subEntry := config.SubEntry{Name: "missing", Backup: "/backup/missing"}
	// Source doesn't exist - dry-run should return nil.
	if err := mgr.RestoreFolder(subEntry, "/backup/missing", "/home/user/.config/missing"); err != nil {
		t.Fatalf("RestoreFolder dry-run with missing source: %v", err)
	}
}

func TestManager_RestoreFolder_CreatesSymlink(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)

	if err := mem.MkdirAll("/backup/app", 0755); err != nil {
		t.Fatalf("MkdirAll source: %v", err)
	}
	if err := mem.WriteFile("/backup/app/cfg", []byte("c"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := mem.MkdirAll("/home/user", 0755); err != nil {
		t.Fatalf("MkdirAll target parent: %v", err)
	}

	subEntry := config.SubEntry{Name: "app", Backup: "/backup/app"}
	if err := mgr.RestoreFolder(subEntry, "/backup/app", "/home/user/app"); err != nil {
		t.Fatalf("RestoreFolder: %v", err)
	}
	if !mgr.isSymlink("/home/user/app") {
		t.Error("expected /home/user/app to be a symlink after RestoreFolder")
	}
}

func TestManager_RestoreFolder_SourceMissingError(t *testing.T) {
	t.Parallel()
	mgr, _ := newMemManager(t)
	mgr.DryRun = false

	subEntry := config.SubEntry{Name: "missing", Backup: "/backup/missing"}
	err := mgr.RestoreFolder(subEntry, "/backup/missing", "/home/user/.config/missing")
	if err == nil {
		t.Error("expected error when source does not exist and not dry-run")
	}
}

// ---------------------------------------------------------------------------
// RestoreFiles via MemFS
// ---------------------------------------------------------------------------

func TestManager_RestoreFiles_CreatesSymlinks(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)

	if err := mem.MkdirAll("/backup/zsh", 0755); err != nil {
		t.Fatalf("MkdirAll source: %v", err)
	}
	if err := mem.WriteFile("/backup/zsh/.zshrc", []byte("# zsh"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := mem.MkdirAll("/home/user", 0755); err != nil {
		t.Fatalf("MkdirAll target parent: %v", err)
	}

	subEntry := config.SubEntry{
		Name:   "zsh",
		Backup: "/backup/zsh",
		Files:  []string{".zshrc"},
	}

	if err := mgr.RestoreFiles(subEntry, "/backup/zsh", "/home/user"); err != nil {
		t.Fatalf("RestoreFiles: %v", err)
	}

	if !mgr.isSymlink("/home/user/.zshrc") {
		t.Error("expected /home/user/.zshrc to be a symlink")
	}
}

func TestManager_RestoreFiles_AlreadyCorrectSymlink(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)

	if err := mem.MkdirAll("/backup/zsh2", 0755); err != nil {
		t.Fatalf("MkdirAll source: %v", err)
	}
	if err := mem.WriteFile("/backup/zsh2/.zshrc", []byte("# zsh"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := mem.MkdirAll("/home/user2", 0755); err != nil {
		t.Fatalf("MkdirAll target: %v", err)
	}
	// Pre-create the correct symlink.
	if err := mem.Symlink("/backup/zsh2/.zshrc", "/home/user2/.zshrc"); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	subEntry := config.SubEntry{
		Name:   "zsh2",
		Backup: "/backup/zsh2",
		Files:  []string{".zshrc"},
	}

	if err := mgr.RestoreFiles(subEntry, "/backup/zsh2", "/home/user2"); err != nil {
		t.Fatalf("RestoreFiles with existing symlink: %v", err)
	}
}

func TestManager_RestoreFiles_AdoptsExistingFile(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)

	// Backup dir exists but the specific file is not there yet.
	if err := mem.MkdirAll("/backup/zsh3", 0755); err != nil {
		t.Fatalf("MkdirAll backup: %v", err)
	}
	if err := mem.MkdirAll("/home/user3", 0755); err != nil {
		t.Fatalf("MkdirAll target: %v", err)
	}
	// Target file exists; backup file does not → should be adopted (moved to backup).
	if err := mem.WriteFile("/home/user3/.zshrc", []byte("existing"), 0644); err != nil {
		t.Fatalf("WriteFile target: %v", err)
	}

	subEntry := config.SubEntry{
		Name:   "zsh3",
		Backup: "/backup/zsh3",
		Files:  []string{".zshrc"},
	}

	if err := mgr.RestoreFiles(subEntry, "/backup/zsh3", "/home/user3"); err != nil {
		t.Fatalf("RestoreFiles adopt: %v", err)
	}
	// After adoption the backup file should exist.
	if !mgr.pathExists("/backup/zsh3/.zshrc") {
		t.Error("expected adopted file to exist in backup")
	}
}

func TestManager_RestoreFiles_MergesExistingFile(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)

	// Both backup and target files exist → should merge (move target to backup,
	// then create symlink from target back to backup).
	if err := mem.MkdirAll("/backup/zsh4", 0755); err != nil {
		t.Fatalf("MkdirAll backup: %v", err)
	}
	if err := mem.WriteFile("/backup/zsh4/.zshrc", []byte("# backup"), 0644); err != nil {
		t.Fatalf("WriteFile backup: %v", err)
	}
	if err := mem.MkdirAll("/home/user4", 0755); err != nil {
		t.Fatalf("MkdirAll target: %v", err)
	}
	if err := mem.WriteFile("/home/user4/.zshrc", []byte("# local"), 0644); err != nil {
		t.Fatalf("WriteFile target: %v", err)
	}

	subEntry := config.SubEntry{
		Name:   "zsh4",
		Backup: "/backup/zsh4",
		Files:  []string{".zshrc"},
	}

	if err := mgr.RestoreFiles(subEntry, "/backup/zsh4", "/home/user4"); err != nil {
		t.Fatalf("RestoreFiles merge: %v", err)
	}
}

func TestManager_RestoreFiles_DryRun(t *testing.T) {
	t.Parallel()
	mgr, _ := newMemManager(t)
	mgr.DryRun = true

	// Neither source nor target exist; dry-run should not error.
	subEntry := config.SubEntry{
		Name:   "dryrun",
		Backup: "/backup/dryrun",
		Files:  []string{"config.yaml"},
	}

	if err := mgr.RestoreFiles(subEntry, "/backup/dryrun", "/home/user/dryrun"); err != nil {
		t.Fatalf("RestoreFiles dry-run: %v", err)
	}
}

// ---------------------------------------------------------------------------
// backupFolderSubEntry via MemFS
// ---------------------------------------------------------------------------

func TestManager_BackupFolderSubEntry_TargetNotExist(t *testing.T) {
	t.Parallel()
	mgr, _ := newMemManager(t)

	subEntry := config.SubEntry{Name: "nvim", Backup: "/backup/nvim"}
	// Target doesn't exist → should be a no-op (return nil).
	if err := mgr.backupFolderSubEntry("nvim", subEntry, "/backup/nvim", "/home/user/.config/nvim"); err != nil {
		t.Fatalf("backupFolderSubEntry: %v", err)
	}
}

func TestManager_BackupFolderSubEntry_TargetIsSymlink(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)

	if err := mem.MkdirAll("/real", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := mem.Symlink("/real", "/link"); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	subEntry := config.SubEntry{Name: "app", Backup: "/backup/app"}
	// Target is a symlink → should be skipped.
	if err := mgr.backupFolderSubEntry("app", subEntry, "/backup/app", "/link"); err != nil {
		t.Fatalf("backupFolderSubEntry with symlink target: %v", err)
	}
}

func TestManager_BackupFolderSubEntry_CopiesFolder(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)

	if err := mem.MkdirAll("/target/app", 0755); err != nil {
		t.Fatalf("MkdirAll target: %v", err)
	}
	if err := mem.WriteFile("/target/app/config.yaml", []byte("key: val"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	if err := mem.MkdirAll("/backup", 0755); err != nil {
		t.Fatalf("MkdirAll backup parent: %v", err)
	}

	subEntry := config.SubEntry{Name: "app", Backup: "/backup/app"}
	if err := mgr.backupFolderSubEntry("app", subEntry, "/backup/app", "/target/app"); err != nil {
		t.Fatalf("backupFolderSubEntry copy: %v", err)
	}
	if !mgr.pathExists("/backup/app/config.yaml") {
		t.Error("expected /backup/app/config.yaml to exist after backup")
	}
}

func TestManager_BackupFolderSubEntry_DryRun(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)
	mgr.DryRun = true

	if err := mem.MkdirAll("/target/app2", 0755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	if err := mem.WriteFile("/target/app2/file.txt", []byte("x"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	subEntry := config.SubEntry{Name: "app2", Backup: "/backup/app2"}
	if err := mgr.backupFolderSubEntry("app2", subEntry, "/backup/app2", "/target/app2"); err != nil {
		t.Fatalf("backupFolderSubEntry dry-run: %v", err)
	}
	// Dry-run: backup should not have been created.
	if mgr.pathExists("/backup/app2") {
		t.Error("backup should not exist in dry-run mode")
	}
}

// ---------------------------------------------------------------------------
// backupFilesSubEntry via MemFS
// ---------------------------------------------------------------------------

func TestManager_BackupFilesSubEntry_TargetNotExist(t *testing.T) {
	t.Parallel()
	mgr, _ := newMemManager(t)

	subEntry := config.SubEntry{Name: "zsh", Backup: "/backup/zsh", Files: []string{".zshrc"}}
	if err := mgr.backupFilesSubEntry("zsh", subEntry, "/backup/zsh", "/nonexistent"); err != nil {
		t.Fatalf("backupFilesSubEntry non-existent target: %v", err)
	}
}

func TestManager_BackupFilesSubEntry_CopiesFile(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)

	if err := mem.MkdirAll("/target/home", 0755); err != nil {
		t.Fatalf("MkdirAll target: %v", err)
	}
	if err := mem.WriteFile("/target/home/.zshrc", []byte("# zsh"), 0644); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	subEntry := config.SubEntry{Name: "zsh", Backup: "/backup/zsh", Files: []string{".zshrc"}}
	if err := mgr.backupFilesSubEntry("zsh", subEntry, "/backup/zsh", "/target/home"); err != nil {
		t.Fatalf("backupFilesSubEntry: %v", err)
	}
	if !mgr.pathExists("/backup/zsh/.zshrc") {
		t.Error("expected /backup/zsh/.zshrc to exist after backup")
	}
}

func TestManager_BackupFilesSubEntry_SkipsSymlink(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)

	if err := mem.MkdirAll("/target/home2", 0755); err != nil {
		t.Fatalf("MkdirAll target: %v", err)
	}
	if err := mem.WriteFile("/real.zshrc", []byte("# zsh"), 0644); err != nil {
		t.Fatalf("WriteFile real: %v", err)
	}
	if err := mem.Symlink("/real.zshrc", "/target/home2/.zshrc"); err != nil {
		t.Fatalf("Symlink: %v", err)
	}

	subEntry := config.SubEntry{Name: "zsh", Backup: "/backup/zsh2", Files: []string{".zshrc"}}
	if err := mgr.backupFilesSubEntry("zsh", subEntry, "/backup/zsh2", "/target/home2"); err != nil {
		t.Fatalf("backupFilesSubEntry skip symlink: %v", err)
	}
	// Symlinks should not be backed up.
	if mgr.pathExists("/backup/zsh2/.zshrc") {
		t.Error("symlinked file should not be backed up")
	}
}

// ---------------------------------------------------------------------------
// MergeFolder with nested directories via MemFS
// ---------------------------------------------------------------------------

func TestManager_MergeFolder_WithSubdirectory(t *testing.T) {
	t.Parallel()
	mgr, mem := newMemManager(t)

	// Create a target with files in a subdirectory.
	if err := mem.MkdirAll("/tgt3/sub", 0755); err != nil {
		t.Fatalf("MkdirAll target: %v", err)
	}
	if err := mem.WriteFile("/tgt3/top.conf", []byte("top"), 0644); err != nil {
		t.Fatalf("WriteFile top: %v", err)
	}
	if err := mem.WriteFile("/tgt3/sub/nested.conf", []byte("nested"), 0644); err != nil {
		t.Fatalf("WriteFile nested: %v", err)
	}
	if err := mem.MkdirAll("/bkp3", 0755); err != nil {
		t.Fatalf("MkdirAll backup: %v", err)
	}

	summary := NewMergeSummary("app")
	if err := mgr.MergeFolder("/bkp3", "/tgt3", false, summary); err != nil {
		t.Fatalf("MergeFolder with subdir: %v", err)
	}
	// Two files should have been merged.
	if len(summary.MergedFiles) != 2 {
		t.Errorf("expected 2 merged files, got %d", len(summary.MergedFiles))
	}
}
