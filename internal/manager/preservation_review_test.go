package manager

import (
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/AntoineGS/tidydots/internal/config"
	"github.com/AntoineGS/tidydots/internal/fsys"
)

func TestPreservationReviewFailedRenderRetainsLiteralAlias(t *testing.T) {
	for _, failure := range []string{"malformed", "occupied-conflict", "source-link"} {
		t.Run(failure, func(t *testing.T) {
			root, target, m, _ := setupTemplateTest(t)
			path := filepath.Join(root, "app.tmpl")
			alias := filepath.Join(root, "app")
			preservationWrite(t, path, "base\n")
			if failure == "occupied-conflict" {
				if err := m.renderTemplateAndLink(path, "app.tmpl"); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(alias); err != nil {
					t.Fatal(err)
				}
				preservationWrite(t, path+".rendered", "edited\n")
				preservationWrite(t, path+".conflict", "previous conflict")
				preservationWrite(t, path, "changed\n")
			}
			preservationWrite(t, alias, "literal\n")
			switch failure {
			case "malformed":
				preservationWrite(t, path, "{{ broken")
			case "source-link":
				skipIfNoSymlink(t)
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink("app", path); err != nil {
					t.Fatal(err)
				}
			}
			if err := m.RestoreFolderWithTemplates(config.SubEntry{Name: "app"}, root, target); err == nil {
				t.Fatal("expected restore failure")
			}
			preservationContent(t, alias, "literal\n")
			info, err := os.Lstat(alias)
			if err != nil || !info.Mode().IsRegular() {
				t.Fatalf("literal alias replaced: %v, %v", info, err)
			}
			if failure == "source-link" {
				preservationContent(t, path, "literal\n")
				if testIsSymlink(target) {
					t.Error("folder mutated before source-link refusal")
				}
			} else {
				preservationContent(t, alias+".tidydots.bak", "literal\n")
				// A failed attempt cannot silently reuse or overwrite its recovery.
				preservationWrite(t, alias, "later literal\n")
				if err := m.renderTemplateAndLink(path, "app.tmpl"); err == nil {
					t.Error("expected occupied backup refusal on retry")
				}
				preservationContent(t, alias, "later literal\n")
				preservationContent(t, alias+".tidydots.bak", "literal\n")
			}
			if failure == "occupied-conflict" {
				preservationContent(t, path+".rendered", "edited\n")
				preservationContent(t, path+".conflict", "previous conflict")
				history, err := m.templateHistory(path, "app.tmpl", "")
				if err != nil {
					t.Fatal(err)
				}
				if history.record == nil || string(history.record.PureRender) != "base\n" {
					t.Fatal("failed render changed history")
				}
			}
		})
	}
}

type aliasPublishFailureFS struct {
	fsys.FS
	alias     string
	failStage bool
}

func (f aliasPublishFailureFS) Symlink(target, link string) error {
	if f.failStage && strings.HasPrefix(filepath.Base(link), ".tidydots-alias-") {
		return fs.ErrPermission
	}
	return f.FS.Symlink(target, link)
}

func (f aliasPublishFailureFS) Rename(from, to string) error {
	if !f.failStage && to == f.alias {
		return fs.ErrPermission
	}
	return f.FS.Rename(from, to)
}

func TestPreservationReviewFailedAliasPublicationRetainsLiteral(t *testing.T) {
	for _, failStage := range []bool{false, true} {
		t.Run(map[bool]string{false: "rename", true: "stage"}[failStage], func(t *testing.T) {
			root, _, m, _ := setupTemplateTest(t)
			path := filepath.Join(root, "app.tmpl")
			alias := filepath.Join(root, "app")
			preservationWrite(t, path, "new render")
			preservationWrite(t, alias, "literal")
			m = m.WithFS(aliasPublishFailureFS{FS: m.fs, alias: alias, failStage: failStage})
			if err := m.renderTemplateAndLink(path, "app.tmpl"); err == nil {
				t.Fatal("expected alias publication failure")
			}
			preservationContent(t, alias, "literal")
			preservationContent(t, alias+".tidydots.bak", "literal")
			if testIsSymlink(alias) {
				t.Fatal("failed publication replaced literal alias")
			}
		})
	}
}

func TestPreservationReviewAliasPreflightIsReadOnly(t *testing.T) {
	root, target, m, _ := setupTemplateTest(t)
	path := filepath.Join(root, "app.tmpl")
	alias := filepath.Join(root, "app")
	preservationWrite(t, path, "new render")
	preservationWrite(t, alias, "literal")
	if err := m.preflightFolderTemplateAliases(root, target); err != nil {
		t.Fatal(err)
	}
	preservationContent(t, alias, "literal")
	if _, err := os.Lstat(alias + ".tidydots.bak"); !os.IsNotExist(err) {
		t.Fatalf("preflight created recovery: %v", err)
	}
	if err := m.RestoreFolderWithTemplates(config.SubEntry{Name: "app"}, root, target); err != nil {
		t.Fatal(err)
	}
	preservationContent(t, alias+".tidydots.bak", "literal")
	verifyRelativeSymlink(t, alias, "app.tmpl.rendered")
	// An already completed replacement is idempotent despite its existing backup.
	if err := m.renderTemplateAndLink(path, "app.tmpl"); err != nil {
		t.Fatal(err)
	}
}

// maskedPreservationFS simulates umask 0077 without changing process-global
// state shared with parallel tests. Chmod failures can be injected separately.
type maskedPreservationFS struct {
	fsys.FS
	failChmod bool
}

func (f maskedPreservationFS) WriteFileExclusive(path string, data []byte, mode fs.FileMode) error {
	return f.FS.WriteFileExclusive(path, data, mode&0o700)
}

func (f maskedPreservationFS) Chmod(path string, mode fs.FileMode) error {
	if f.failChmod {
		return fs.ErrPermission
	}
	return f.FS.Chmod(path, mode)
}

func TestPreservationReviewMergePermissionsIgnoreUmask(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits")
	}
	for _, fail := range []bool{false, true} {
		t.Run(map[bool]string{false: "umask", true: "chmod-failure"}[fail], func(t *testing.T) {
			root, target, m, _ := setupTemplateTest(t)
			src := filepath.Join(target, "script")
			dst := filepath.Join(root, "script")
			preservationWrite(t, src, "#!/bin/sh\n")
			if err := os.Chmod(src, 0o755); err != nil {
				t.Fatal(err)
			}
			m = m.WithFS(maskedPreservationFS{FS: m.fs, failChmod: fail})
			err := m.MergeFolder(root, target, false, NewMergeSummary("app"))
			if fail {
				if err == nil {
					t.Error("expected chmod failure")
				}
				preservationContent(t, src, "#!/bin/sh\n")
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			info, err := os.Stat(dst)
			if err != nil {
				t.Fatal(err)
			}
			if info.Mode().Perm() != 0o755 {
				t.Fatalf("merged mode = %o, want 755", info.Mode().Perm())
			}
			if _, err := os.Lstat(src); !os.IsNotExist(err) {
				t.Fatalf("original not removed after successful preservation: %v", err)
			}
		})
	}
}
