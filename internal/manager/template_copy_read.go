package manager

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	"github.com/AntoineGS/tidydots/internal/cmdexec"
)

type templateCopySnapshot struct {
	Exists        bool
	Symlink       bool
	Content       []byte
	Mode          fs.FileMode
	RootOwned     bool
	Device        uint64
	Inode         uint64
	IdentityKnown bool
}

const templateCopyStatFormat = "%f %u %d %i"

type templateCopyFileType uint32

const (
	templateCopyFileTypeMask  templateCopyFileType = 0o170000
	templateCopyRegularFile   templateCopyFileType = 0o100000
	templateCopySymlinkFile   templateCopyFileType = 0o120000
	templateCopyDirectoryFile templateCopyFileType = 0o040000
)

type templateCopyStat struct {
	Mode      fs.FileMode
	RootOwned bool
	Device    uint64
	Inode     uint64
	FileType  templateCopyFileType
}

// readTemplateCopyTarget returns a safe-to-read snapshot of a target. Lstat
// establishes the path's object type before any content read; a symlink is
// followed only when its referent is a regular file.
//
//nolint:gocyclo // inspection has deliberately separate error paths for native and sudo access
func (m *Manager) readTemplateCopyTarget(path string, sudo bool) (templateCopySnapshot, error) {
	var empty templateCopySnapshot
	useSudo, err := templateCopySudoPolicy(runtime.GOOS, sudo)
	if err != nil {
		return empty, err
	}

	linkInfo, err := m.fs.Lstat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return empty, nil
		}
		if useSudo && isTemplateCopyPermissionError(err) {
			return m.readTemplateCopyTargetSudo(path, false)
		}
		return empty, fmt.Errorf("inspecting target %q: %w", path, err)
	}

	symlink := linkInfo.Mode()&fs.ModeSymlink != 0
	if !symlink && runtime.GOOS == "windows" {
		// Directory junctions may not carry ModeSymlink, but Readlink still
		// identifies them on supported Windows filesystems.
		_, readlinkErr := m.fs.Readlink(path)
		symlink = readlinkErr == nil
	}

	info := linkInfo
	if symlink {
		info, err = m.fs.Stat(path)
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return templateCopySnapshot{Symlink: true}, nil
			}
			if useSudo && isTemplateCopyPermissionError(err) {
				return m.readTemplateCopyTargetSudo(path, true)
			}
			return templateCopySnapshot{Symlink: true}, fmt.Errorf("inspecting symlink target %q: %w", path, err)
		}
	}

	if !info.Mode().IsRegular() {
		return templateCopySnapshot{Symlink: symlink}, fmt.Errorf("target %q is not a regular file", path)
	}

	content, err := m.fs.ReadFile(path)
	if err != nil {
		if useSudo && isTemplateCopyPermissionError(err) {
			return m.readTemplateCopyTargetSudo(path, symlink)
		}
		return templateCopySnapshot{Symlink: symlink}, fmt.Errorf("reading target %q: %w", path, err)
	}

	device, inode, identityKnown := fileIdentity(info)
	return templateCopySnapshot{
		Exists:        true,
		Symlink:       symlink,
		Content:       content,
		Mode:          info.Mode().Perm(),
		RootOwned:     fileRootOwned(info),
		Device:        device,
		Inode:         inode,
		IdentityKnown: identityKnown,
	}, nil
}

func (m *Manager) readTemplateCopyTargetSudo(path string, symlink bool) (templateCopySnapshot, error) {
	statResult, runErr := m.runner.RunWithSudo(
		m.ctx, "stat", "--printf="+templateCopyStatFormat, "--", path,
	)
	statErr := templateCopyCommandError("stat", statResult, runErr)
	if statErr != nil {
		exists, probeErr := m.probeTemplateCopyEntry(path)
		if probeErr != nil {
			return templateCopySnapshot{Symlink: symlink}, fmt.Errorf(
				"inspecting target %q: %w", path, errors.Join(statErr, probeErr))
		}
		if exists {
			return templateCopySnapshot{Symlink: symlink}, fmt.Errorf(
				"inspecting target %q: %w", path, statErr)
		}
		return templateCopySnapshot{Symlink: symlink}, nil
	}

	linkMetadata, err := parseTemplateCopyStatMetadata(statResult.Stdout)
	if err != nil {
		return templateCopySnapshot{Symlink: symlink}, fmt.Errorf("parsing target metadata %q: %w", path, err)
	}
	if linkMetadata.FileType != templateCopyRegularFile && linkMetadata.FileType != templateCopySymlinkFile {
		return templateCopySnapshot{Symlink: symlink}, fmt.Errorf("target %q is not a regular file or symlink", path)
	}

	symlink = linkMetadata.FileType == templateCopySymlinkFile
	if symlink {
		followResult, runErr := m.runner.RunWithSudo(
			m.ctx, "stat", "--dereference", "--printf="+templateCopyStatFormat, "--", path,
		)
		followErr := templateCopyCommandError("stat referent", followResult, runErr)
		if followErr != nil {
			dangling, probeErr := m.probeTemplateCopyDangling(path)
			if probeErr != nil {
				return templateCopySnapshot{Symlink: true}, fmt.Errorf(
					"inspecting symlink referent %q: %w", path, errors.Join(followErr, probeErr))
			}
			if dangling {
				return templateCopySnapshot{Symlink: true}, nil
			}
			return templateCopySnapshot{Symlink: true}, fmt.Errorf(
				"inspecting symlink referent %q: %w", path, followErr)
		}

		linkMetadata, err = parseTemplateCopyStatMetadata(followResult.Stdout)
		if err != nil {
			return templateCopySnapshot{Symlink: true}, fmt.Errorf(
				"parsing symlink referent metadata %q: %w", path, err)
		}
		if linkMetadata.FileType != templateCopyRegularFile {
			return templateCopySnapshot{Symlink: true}, fmt.Errorf(
				"symlink referent %q is not a regular file", path)
		}
	}

	catResult, runErr := m.runner.RunWithSudo(m.ctx, "cat", "--", path)
	if err := templateCopyCommandError("cat", catResult, runErr); err != nil {
		return templateCopySnapshot{Symlink: symlink}, fmt.Errorf("reading target %q: %w", path, err)
	}

	return templateCopySnapshot{
		Exists:        true,
		Symlink:       symlink,
		Content:       catResult.Stdout,
		Mode:          linkMetadata.Mode,
		RootOwned:     linkMetadata.RootOwned,
		Device:        linkMetadata.Device,
		Inode:         linkMetadata.Inode,
		IdentityKnown: true,
	}, nil
}

// probeTemplateCopyEntry positively checks whether a directory entry exists,
// following only a symlink used as the command-line parent. Child entries are
// listed without dereferencing them. An empty successful find means absent; a
// failed find is an inspection error rather than absence.
func (m *Manager) probeTemplateCopyEntry(path string) (bool, error) {
	parent := filepath.Dir(path)
	parentResult, runErr := m.runner.RunWithSudo(
		m.ctx, "stat", "--dereference", "--printf="+templateCopyStatFormat, "--", parent,
	)
	if err := templateCopyCommandError("stat parent", parentResult, runErr); err != nil {
		return false, err
	}
	parentMetadata, err := parseTemplateCopyStatMetadata(parentResult.Stdout)
	if err != nil {
		return false, fmt.Errorf("parsing parent metadata %q: %w", parent, err)
	}
	if parentMetadata.FileType != templateCopyDirectoryFile {
		return false, fmt.Errorf("parent %q is not a directory", parent)
	}

	result, runErr := m.runner.RunWithSudo(
		m.ctx,
		"find", "-H", "--", parent, "-mindepth", "1", "-maxdepth", "1", "-print0",
	)
	if err := templateCopyCommandError("find target entry", result, runErr); err != nil {
		return false, err
	}

	want := filepath.Clean(path)
	for _, candidate := range bytes.Split(result.Stdout, []byte{0}) {
		if len(candidate) != 0 && filepath.Clean(string(candidate)) == want {
			return true, nil
		}
	}
	return false, nil
}

// probeTemplateCopyDangling distinguishes a dangling symlink from a failed
// referent inspection. GNU find's %y output is a stable one-byte type code;
// with -L, a dangling symlink remains 'l', while a regular referent is 'f'.
func (m *Manager) probeTemplateCopyDangling(path string) (bool, error) {
	result, runErr := m.runner.RunWithSudo(
		m.ctx, "find", "-L", "--", path, "-maxdepth", "0", "-printf", "%y",
	)
	if err := templateCopyCommandError("find symlink referent", result, runErr); err != nil {
		return false, err
	}

	typeCode := strings.TrimSpace(string(result.Stdout))
	if typeCode == "l" {
		return true, nil
	}
	if len(typeCode) != 1 {
		return false, fmt.Errorf("find returned invalid referent type %q", typeCode)
	}
	return false, nil
}

func parseTemplateCopyStat(data []byte) (mode fs.FileMode, rootOwned bool, device, inode uint64, err error) {
	metadata, err := parseTemplateCopyStatMetadata(data)
	if err != nil {
		return 0, false, 0, 0, err
	}
	if metadata.FileType != templateCopyRegularFile && metadata.FileType != templateCopySymlinkFile {
		return 0, false, 0, 0, fmt.Errorf("target has unsupported file type %#o", metadata.FileType)
	}
	return metadata.Mode, metadata.RootOwned, metadata.Device, metadata.Inode, nil
}

func parseTemplateCopyStatMetadata(data []byte) (templateCopyStat, error) {
	fields := strings.Fields(string(data))
	if len(fields) != 4 {
		return templateCopyStat{}, fmt.Errorf("expected raw mode, uid, device, and inode")
	}

	modeValue, parseErr := strconv.ParseUint(fields[0], 16, 32)
	if parseErr != nil {
		return templateCopyStat{}, fmt.Errorf("invalid raw mode %q: %w", fields[0], parseErr)
	}
	uid, parseErr := strconv.ParseUint(fields[1], 10, 64)
	if parseErr != nil {
		return templateCopyStat{}, fmt.Errorf("invalid uid %q: %w", fields[1], parseErr)
	}
	device, parseErr := strconv.ParseUint(fields[2], 10, 64)
	if parseErr != nil {
		return templateCopyStat{}, fmt.Errorf("invalid device %q: %w", fields[2], parseErr)
	}
	inode, parseErr := strconv.ParseUint(fields[3], 10, 64)
	if parseErr != nil {
		return templateCopyStat{}, fmt.Errorf("invalid inode %q: %w", fields[3], parseErr)
	}

	fileType := templateCopyFileType(modeValue) & templateCopyFileTypeMask

	return templateCopyStat{
		Mode:      fs.FileMode(modeValue & 0o777),
		RootOwned: uid == 0,
		Device:    device,
		Inode:     inode,
		FileType:  fileType,
	}, nil
}

func isTemplateCopyPermissionError(err error) bool {
	return errors.Is(err, fs.ErrPermission)
}

func templateCopySudoPolicy(goos string, sudo bool) (bool, error) {
	if !sudo || goos == "windows" {
		return false, nil
	}
	if goos == "linux" {
		return true, nil
	}
	return false, fmt.Errorf("copy-template sudo operations are unsupported on %s", goos)
}

func templateCopyCommandError(name string, result cmdexec.Result, runErr error) error {
	if runErr != nil {
		return fmt.Errorf("%s: %w", name, runErr)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("%s exited with status %d", name, result.ExitCode)
	}
	return nil
}
