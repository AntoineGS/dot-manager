package manager

import (
	"crypto/rand"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
)

// writeTemplateCopyFile installs data using an exclusive sibling stage for a
// replaceable target, or an exclusive destination creation for artifacts that
// must never overwrite an existing path.
func (m *Manager) writeTemplateCopyFile(path string, data []byte, mode fs.FileMode, sudo, exclusive bool) error {
	useSudo, err := templateCopySudoPolicy(runtime.GOOS, sudo)
	if err != nil {
		return err
	}
	if m.DryRun {
		return nil
	}

	if useSudo {
		return m.writeTemplateCopyFileSudo(path, data, mode, exclusive)
	}

	if exclusive {
		if err := m.fs.WriteFileExclusive(path, data, mode.Perm()); err != nil {
			return fmt.Errorf("creating exclusive copy %q: %w", path, err)
		}
		return nil
	}

	stage := filepath.Join(filepath.Dir(path), ".tidydots-copy-"+rand.Text())
	if err := m.fs.WriteFileExclusive(stage, data, 0o600); err != nil {
		return fmt.Errorf("staging copy %q: %w", path, err)
	}

	cleanup := func(original error) error {
		if cleanupErr := m.fs.Remove(stage); cleanupErr != nil && !errors.Is(cleanupErr, fs.ErrNotExist) {
			return errors.Join(original, fmt.Errorf("cleaning copy stage: %w", cleanupErr))
		}
		return original
	}

	if err := m.fs.Chmod(stage, mode.Perm()); err != nil {
		return cleanup(fmt.Errorf("setting copy mode: %w", err))
	}
	if err := m.fs.Rename(stage, path); err != nil {
		return cleanup(fmt.Errorf("installing copy: %w", err))
	}

	return nil
}

func (m *Manager) writeTemplateCopyFileSudo(path string, data []byte, mode fs.FileMode, exclusive bool) error {
	parent := filepath.Dir(path)
	mktempResult, runErr := m.runner.RunWithSudo(
		m.ctx, "mktemp", "--tmpdir="+parent, ".tidydots-copy-XXXXXXXXXX",
	)
	if err := templateCopyCommandError("mktemp", mktempResult, runErr); err != nil {
		return err
	}

	stage, err := validateTemplateCopyStage(string(mktempResult.Stdout), parent)
	if err != nil {
		return err
	}

	cleanup := func(original error) error {
		cleanupResult, cleanupErr := m.runner.RunWithSudo(m.ctx, "rm", "-f", "--", stage)
		if err := templateCopyCommandError("cleaning copy stage", cleanupResult, cleanupErr); err != nil {
			return errors.Join(original, err)
		}
		return original
	}

	ddResult, runErr := m.runner.RunIn(
		m.ctx,
		cmdexec.RunOptions{Sudo: true, Stdin: data},
		"dd", "of="+stage, "status=none",
	)
	if err := templateCopyCommandError("staging copy", ddResult, runErr); err != nil {
		return cleanup(err)
	}

	chmodResult, runErr := m.runner.RunWithSudo(
		m.ctx, "chmod", fmt.Sprintf("%o", mode.Perm()), "--", stage,
	)
	if err := templateCopyCommandError("setting copy mode", chmodResult, runErr); err != nil {
		return cleanup(err)
	}

	if exclusive {
		linkResult, runErr := m.runner.RunWithSudo(m.ctx, "ln", "-T", "--", stage, path)
		if err := templateCopyCommandError("publishing exclusive copy", linkResult, runErr); err != nil {
			return cleanup(err)
		}

		removeResult, runErr := m.runner.RunWithSudo(m.ctx, "rm", "-f", "--", stage)
		if err := templateCopyCommandError("removing copy stage", removeResult, runErr); err != nil {
			return cleanup(err)
		}
		return nil
	}

	moveResult, runErr := m.runner.RunWithSudo(m.ctx, "mv", "-fT", "--", stage, path)
	if err := templateCopyCommandError("installing copy", moveResult, runErr); err != nil {
		return cleanup(err)
	}
	return nil
}

func validateTemplateCopyStage(output, parent string) (string, error) {
	stage := strings.TrimSuffix(output, "\n")
	stage = strings.TrimSuffix(stage, "\r")
	if stage == "" || strings.ContainsAny(stage, "\r\n\t") {
		return "", fmt.Errorf("mktemp returned an invalid stage path")
	}

	stage = filepath.Clean(stage)
	parent = filepath.Clean(parent)
	if !filepath.IsAbs(stage) || filepath.Dir(stage) != parent {
		return "", fmt.Errorf("mktemp stage %q is outside destination parent", stage)
	}

	const prefix = ".tidydots-copy-"
	if base := filepath.Base(stage); !strings.HasPrefix(base, prefix) || len(base) == len(prefix) {
		return "", fmt.Errorf("mktemp stage %q has an unexpected name", stage)
	}

	return stage, nil
}

// removeTemplateCopyArtifact removes only the named artifact, never a
// directory tree. Missing artifacts are already gone and therefore harmless.
func (m *Manager) removeTemplateCopyArtifact(path string, sudo bool) error {
	useSudo, err := templateCopySudoPolicy(runtime.GOOS, sudo)
	if err != nil {
		return err
	}
	if m.DryRun {
		return nil
	}

	if useSudo {
		result, runErr := m.runner.RunWithSudo(m.ctx, "rm", "-f", "--", path)
		return templateCopyCommandError("removing copy artifact", result, runErr)
	}

	if err := m.fs.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("removing copy artifact %q: %w", path, err)
	}
	return nil
}
