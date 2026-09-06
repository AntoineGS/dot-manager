package manager

import (
	"bytes"
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"

	tmpl "github.com/AntoineGS/tidydots/internal/template"
)

// preflightFolderTemplateAliases refuses target aliases before folder merge can
// adopt them and subsequently replace them with generated template links.
func (m *Manager) preflightFolderTemplateAliases(source, target string) error {
	targetIsLink := m.isSymlink(target)
	err := m.fs.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if targetIsLink || d.IsDir() || !tmpl.IsTemplateFile(d.Name()) {
			return nil
		}
		rel, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		alias := filepath.Join(target, tmpl.TargetName(rel))
		if _, err := m.fs.Lstat(alias); err == nil {
			return fmt.Errorf("template alias %q is occupied; preserve or relocate it before restoring", alias)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	return m.fs.WalkDir(source, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !tmpl.IsTemplateFile(d.Name()) {
			return nil
		}
		return m.preflightLiteralTemplateAlias(path)
	})
}

// preflightLiteralTemplateAlias is read-only. Refuse symlink sources when replacing a
// literal alias could remove their referent, and refuse occupied recovery paths.
func (m *Manager) preflightLiteralTemplateAlias(templatePath string) error {
	alias := filepath.Join(filepath.Dir(templatePath), tmpl.TargetName(filepath.Base(templatePath)))
	info, err := m.literalAliasInfo(alias)
	if err != nil || info == nil {
		return err
	}
	templateInfo, err := m.fs.Lstat(templatePath)
	if err != nil {
		return err
	}
	if templateInfo.Mode()&fs.ModeSymlink != 0 {
		return fmt.Errorf("symlink template %q with literal alias %q requires manual preservation before restore", templatePath, alias)
	}
	if _, err := m.fs.Lstat(alias + ".tidydots.bak"); err == nil {
		return fmt.Errorf("alias recovery %q is occupied: %w", alias+".tidydots.bak", fs.ErrExist)
	} else if !errors.Is(err, fs.ErrNotExist) {
		return err
	}
	return nil
}

func (m *Manager) literalAliasInfo(path string) (fs.FileInfo, error) {
	info, err := m.fs.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if info.Mode()&fs.ModeSymlink != 0 {
		return nil, nil
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("template alias %q is not a regular file", path)
	}
	return info, nil
}

// literalAliasPreservation belongs to one render attempt. Passing it to the
// final link step avoids making a second backup or trusting an old recovery.
type literalAliasPreservation struct {
	path    string
	content []byte
}

// preserveLiteralAlias copies without removing the live literal. Failed renders
// leave both it and its exclusive recovery intact; retries refuse occupied backups.
func (m *Manager) preserveLiteralAlias(path string) (*literalAliasPreservation, error) {
	info, err := m.literalAliasInfo(path)
	if err != nil || info == nil || m.DryRun {
		return nil, err
	}
	content, err := m.fs.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if err := m.writeTemplateCopyFile(path+".tidydots.bak", content, 0o600, false, true); err != nil {
		return nil, err
	}
	return &literalAliasPreservation{path: path, content: content}, nil
}

// replacePreparedAlias creates the new link before atomically replacing an
// occupied path. A failed stage creation or rename keeps the original alias.
func (m *Manager) replacePreparedAlias(path, target string, preserved *literalAliasPreservation) error {
	info, err := m.fs.Lstat(path)
	if errors.Is(err, fs.ErrNotExist) {
		return m.fs.Symlink(target, path)
	}
	if err != nil {
		return err
	}
	if info.Mode().IsRegular() {
		if preserved == nil || preserved.path != path {
			return fmt.Errorf("literal alias %q has not been preserved", path)
		}
		current, err := m.fs.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Equal(current, preserved.content) {
			return fmt.Errorf("literal alias %q changed after preservation", path)
		}
	} else if info.Mode()&fs.ModeSymlink == 0 {
		return fmt.Errorf("template alias %q is not a file or symlink", path)
	}
	stage := filepath.Join(filepath.Dir(path), ".tidydots-alias-"+rand.Text())
	if err := m.fs.Symlink(target, stage); err != nil {
		return err
	}
	if err := m.fs.Rename(stage, path); err != nil {
		cleanupErr := m.fs.Remove(stage)
		return errors.Join(fmt.Errorf("replacing template alias: %w", err), cleanupErr)
	}
	return nil
}

func (m *Manager) preserveOrphanRender(path string) error {
	current, err := m.readTemplateCopyTarget(path, false)
	if err != nil {
		return err
	}
	if !current.Exists && !current.Symlink {
		return nil
	}
	if !current.Exists {
		return fmt.Errorf("rendered output %q is a dangling symlink", path)
	}
	return m.writeTemplateCopyFile(path+".bak", current.Content, 0o600, false, true)
}
