package fsys

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemFS is an in-memory filesystem implementation intended for use in tests.
//
// Keys are always slash-separated absolute paths, mirroring Unix semantics on
// every host OS. Callers routinely build paths with path/filepath, which emits
// backslashes on Windows, so every path entering MemFS is normalized to the
// slash form via norm before it is used as a key.
type MemFS struct {
	mu       sync.RWMutex
	files    map[string][]byte
	dirs     map[string]bool
	symlinks map[string]string
	perms    map[string]fs.FileMode
}

// norm converts an OS-native or slash-separated path into MemFS's canonical
// slash-separated key form.
func norm(p string) string {
	return path.Clean(filepath.ToSlash(p))
}

// dirPrefix returns the key prefix that matches the children of directory p.
func dirPrefix(p string) string {
	if p == "/" {
		return "/"
	}
	return p + "/"
}

// NewMemFS creates a new empty MemFS with the root directory pre-created.
func NewMemFS() *MemFS {
	m := &MemFS{
		files:    make(map[string][]byte),
		dirs:     make(map[string]bool),
		symlinks: make(map[string]string),
		perms:    make(map[string]fs.FileMode),
	}
	m.dirs["/"] = true
	return m
}

// pathError wraps an error as an os.PathError.
func pathError(op, path string, err error) error {
	return &os.PathError{Op: op, Path: path, Err: err}
}

// Stat returns a FileInfo for the named path, following symlinks.
func (m *MemFS) Stat(name string) (fs.FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	name = norm(name)

	// Follow symlinks.
	resolved, err := m.resolveSymlink(name)
	if err != nil {
		return nil, pathError("stat", name, os.ErrNotExist)
	}

	return m.statResolved(resolved, name)
}

// resolveSymlink resolves a path through symlinks (must be called with lock
// held). name must already be normalized.
func (m *MemFS) resolveSymlink(name string) (string, error) {
	visited := make(map[string]bool)
	current := name

	for {
		if visited[current] {
			return "", fmt.Errorf("too many levels of symbolic links")
		}
		visited[current] = true

		if target, ok := m.symlinks[current]; ok {
			// Targets are stored verbatim so Readlink round-trips like OsFS;
			// normalize only to follow them.
			target = filepath.ToSlash(target)
			if path.IsAbs(target) {
				current = path.Clean(target)
			} else {
				current = path.Join(path.Dir(current), target)
			}
			continue
		}

		// Check if it's a known file or directory.
		if _, ok := m.files[current]; ok {
			return current, nil
		}
		if m.dirs[current] {
			return current, nil
		}

		return "", os.ErrNotExist
	}
}

// statResolved builds a FileInfo for an already-resolved path (lock must be held).
func (m *MemFS) statResolved(resolved, displayName string) (fs.FileInfo, error) {
	if data, ok := m.files[resolved]; ok {
		perm, ok := m.perms[resolved]
		if !ok {
			perm = 0o644
		}
		return &memFileInfo{
			name:  path.Base(displayName),
			size:  int64(len(data)),
			mode:  perm,
			isDir: false,
		}, nil
	}
	if m.dirs[resolved] {
		perm, ok := m.perms[resolved]
		if !ok {
			perm = 0o755
		}
		return &memFileInfo{
			name:  path.Base(displayName),
			size:  0,
			mode:  perm | fs.ModeDir,
			isDir: true,
		}, nil
	}
	return nil, pathError("stat", displayName, os.ErrNotExist)
}

// Lstat returns a FileInfo for the named path without following symlinks.
func (m *MemFS) Lstat(name string) (fs.FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.lstatLocked(norm(name))
}

// ReadFile reads and returns the content of the named file.
func (m *MemFS) ReadFile(name string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	name = norm(name)

	// Follow symlinks.
	resolved, err := m.resolveSymlink(name)
	if err != nil {
		return nil, pathError("open", name, os.ErrNotExist)
	}

	data, ok := m.files[resolved]
	if !ok {
		return nil, pathError("open", name, os.ErrNotExist)
	}

	result := make([]byte, len(data))
	copy(result, data)
	return result, nil
}

// WriteFile writes data to the named file.
func (m *MemFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name = norm(name)

	// Ensure parent directory exists.
	parent := path.Dir(name)
	if parent != name && !m.dirs[parent] {
		return pathError("open", name, os.ErrNotExist)
	}

	content := make([]byte, len(data))
	copy(content, data)
	m.files[name] = content
	m.perms[name] = perm
	return nil
}

// WriteFileExclusive creates and writes a file only when name is unoccupied.
// The occupancy check and insertion happen under one lock so concurrent
// writers cannot replace one another.
func (m *MemFS) WriteFileExclusive(name string, data []byte, perm fs.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name = norm(name)

	if _, ok := m.files[name]; ok {
		return pathError("open", name, os.ErrExist)
	}
	if m.dirs[name] {
		return pathError("open", name, os.ErrExist)
	}
	if _, ok := m.symlinks[name]; ok {
		return pathError("open", name, os.ErrExist)
	}

	parent := path.Dir(name)
	if parent != name && !m.dirs[parent] {
		return pathError("open", name, os.ErrNotExist)
	}

	content := make([]byte, len(data))
	copy(content, data)
	m.files[name] = content
	m.perms[name] = perm
	return nil
}

// Chmod changes the permission bits of name. Like os.Chmod, a symlink is
// followed and the referent is changed.
func (m *MemFS) Chmod(name string, mode fs.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name = norm(name)
	resolved, err := m.resolveSymlink(name)
	if err != nil {
		return pathError("chmod", name, os.ErrNotExist)
	}
	if _, ok := m.files[resolved]; ok {
		m.perms[resolved] = mode.Perm()
		return nil
	}
	if m.dirs[resolved] {
		m.perms[resolved] = mode.Perm()
		return nil
	}

	return pathError("chmod", name, os.ErrNotExist)
}

// MkdirAll creates dir and all necessary parent directories.
func (m *MemFS) MkdirAll(dir string, perm fs.FileMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	parts := strings.Split(norm(dir), "/")
	current := ""
	for _, part := range parts {
		if part == "" {
			current = "/"
		} else {
			if current == "/" {
				current = "/" + part
			} else {
				current = current + "/" + part
			}
		}
		if !m.dirs[current] {
			m.dirs[current] = true
			m.perms[current] = perm | fs.ModeDir
		}
	}
	return nil
}

// Remove removes the named file or empty directory.
func (m *MemFS) Remove(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name = norm(name)

	if _, ok := m.files[name]; ok {
		delete(m.files, name)
		delete(m.perms, name)
		return nil
	}
	if _, ok := m.symlinks[name]; ok {
		delete(m.symlinks, name)
		return nil
	}
	if m.dirs[name] {
		// Check the directory is empty.
		prefix := dirPrefix(name)
		for k := range m.files {
			if strings.HasPrefix(k, prefix) {
				return pathError("remove", name, fmt.Errorf("directory not empty"))
			}
		}
		for k := range m.dirs {
			if strings.HasPrefix(k, prefix) {
				return pathError("remove", name, fmt.Errorf("directory not empty"))
			}
		}
		delete(m.dirs, name)
		delete(m.perms, name)
		return nil
	}

	return pathError("remove", name, os.ErrNotExist)
}

// RemoveAll removes name and any children it contains.
func (m *MemFS) RemoveAll(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	name = norm(name)
	prefix := dirPrefix(name)

	for k := range m.files {
		if k == name || strings.HasPrefix(k, prefix) {
			delete(m.files, k)
			delete(m.perms, k)
		}
	}
	for k := range m.symlinks {
		if k == name || strings.HasPrefix(k, prefix) {
			delete(m.symlinks, k)
		}
	}
	for k := range m.dirs {
		if k == name || strings.HasPrefix(k, prefix) {
			delete(m.dirs, k)
			delete(m.perms, k)
		}
	}
	return nil
}

// Symlink creates newname as a symbolic link pointing to oldname. The target is
// stored verbatim, mirroring how the real filesystem reports it back.
func (m *MemFS) Symlink(oldname, newname string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	newname = norm(newname)

	if _, ok := m.symlinks[newname]; ok {
		return pathError("symlink", newname, os.ErrExist)
	}
	if _, ok := m.files[newname]; ok {
		return pathError("symlink", newname, os.ErrExist)
	}

	// Ensure parent directory exists.
	parent := path.Dir(newname)
	if parent != newname && !m.dirs[parent] {
		return pathError("symlink", newname, os.ErrNotExist)
	}

	m.symlinks[newname] = oldname
	return nil
}

// Readlink returns the destination of the named symbolic link.
func (m *MemFS) Readlink(name string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	target, ok := m.symlinks[norm(name)]
	if !ok {
		return "", pathError("readlink", name, os.ErrNotExist)
	}
	return target, nil
}

// Rename renames (moves) oldpath to newpath.
func (m *MemFS) Rename(oldpath, newpath string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	oldpath, newpath = norm(oldpath), norm(newpath)
	if oldpath == newpath {
		if _, ok := m.files[oldpath]; ok {
			return nil
		}
		if _, ok := m.symlinks[oldpath]; ok {
			return nil
		}
		if m.dirs[oldpath] {
			return nil
		}
		return pathError("rename", oldpath, os.ErrNotExist)
	}

	if data, ok := m.files[oldpath]; ok {
		if err := m.prepareRenameDestination(newpath, false); err != nil {
			return err
		}
		perm := m.perms[oldpath]
		m.files[newpath] = data
		m.perms[newpath] = perm
		delete(m.files, oldpath)
		delete(m.perms, oldpath)
		return nil
	}
	if target, ok := m.symlinks[oldpath]; ok {
		if err := m.prepareRenameDestination(newpath, false); err != nil {
			return err
		}
		m.symlinks[newpath] = target
		delete(m.symlinks, oldpath)
		return nil
	}
	if m.dirs[oldpath] {
		if err := m.prepareRenameDestination(newpath, true); err != nil {
			return err
		}
		// Move directory and all its contents.
		prefix := dirPrefix(oldpath)
		newPrefix := dirPrefix(newpath)

		m.dirs[newpath] = true
		m.perms[newpath] = m.perms[oldpath]
		delete(m.dirs, oldpath)
		delete(m.perms, oldpath)

		for k, v := range m.files {
			if strings.HasPrefix(k, prefix) {
				newKey := newPrefix + k[len(prefix):]
				m.files[newKey] = v
				m.perms[newKey] = m.perms[k]
				delete(m.files, k)
				delete(m.perms, k)
			}
		}
		for k := range m.dirs {
			if strings.HasPrefix(k, prefix) {
				newKey := newPrefix + k[len(prefix):]
				m.dirs[newKey] = true
				m.perms[newKey] = m.perms[k]
				delete(m.dirs, k)
				delete(m.perms, k)
			}
		}
		return nil
	}

	return pathError("rename", oldpath, os.ErrNotExist)
}

// prepareRenameDestination applies the replacement rules needed by Rename.
// Regular files and symlinks replace one another, while directories may only
// replace an empty directory and may not replace a non-directory occupant.
// The lock must be held by the caller.
func (m *MemFS) prepareRenameDestination(path string, sourceDir bool) error {
	if _, ok := m.files[path]; ok {
		if sourceDir {
			return pathError("rename", path, os.ErrExist)
		}
		delete(m.files, path)
		delete(m.perms, path)
	}
	if _, ok := m.symlinks[path]; ok {
		if sourceDir {
			return pathError("rename", path, os.ErrExist)
		}
		delete(m.symlinks, path)
	}
	if m.dirs[path] {
		if !sourceDir {
			return pathError("rename", path, os.ErrExist)
		}
		prefix := dirPrefix(path)
		for child := range m.files {
			if strings.HasPrefix(child, prefix) {
				return pathError("rename", path, fmt.Errorf("directory not empty"))
			}
		}
		for child := range m.symlinks {
			if strings.HasPrefix(child, prefix) {
				return pathError("rename", path, fmt.Errorf("directory not empty"))
			}
		}
		for child := range m.dirs {
			if strings.HasPrefix(child, prefix) {
				return pathError("rename", path, fmt.Errorf("directory not empty"))
			}
		}
		delete(m.dirs, path)
		delete(m.perms, path)
	}
	return nil
}

// ReadDir reads the named directory, returning sorted directory entries.
func (m *MemFS) ReadDir(name string) ([]os.DirEntry, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	name = norm(name)

	if !m.dirs[name] {
		return nil, pathError("open", name, os.ErrNotExist)
	}

	prefix := dirPrefix(name)
	seen := make(map[string]bool)
	var entries []os.DirEntry

	entries = m.readDirFiles(prefix, seen, entries)
	entries = m.readDirSymlinks(prefix, seen, entries)
	entries = m.readDirDirs(prefix, seen, entries)

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() < entries[j].Name()
	})

	return entries, nil
}

// readDirFiles appends direct file children of prefix to entries (lock must be held).
func (m *MemFS) readDirFiles(prefix string, seen map[string]bool, entries []os.DirEntry) []os.DirEntry {
	for k := range m.files {
		rest, ok := directChild(k, prefix)
		if !ok || seen[rest] {
			continue
		}
		seen[rest] = true
		perm, ok := m.perms[k]
		if !ok {
			perm = 0o644
		}
		entries = append(entries, &memDirEntry{name: rest, isDir: false, mode: perm})
	}
	return entries
}

// readDirSymlinks appends direct symlink children of prefix to entries (lock must be held).
func (m *MemFS) readDirSymlinks(prefix string, seen map[string]bool, entries []os.DirEntry) []os.DirEntry {
	for k := range m.symlinks {
		rest, ok := directChild(k, prefix)
		if !ok || seen[rest] {
			continue
		}
		seen[rest] = true
		entries = append(entries, &memDirEntry{name: rest, isDir: false, mode: fs.ModeSymlink | 0o777})
	}
	return entries
}

// readDirDirs appends direct sub-directory children of prefix to entries (lock must be held).
func (m *MemFS) readDirDirs(prefix string, seen map[string]bool, entries []os.DirEntry) []os.DirEntry {
	for k := range m.dirs {
		rest, ok := directChild(k, prefix)
		if !ok || seen[rest] {
			continue
		}
		seen[rest] = true
		perm, ok := m.perms[k]
		if !ok {
			perm = 0o755
		}
		entries = append(entries, &memDirEntry{name: rest, isDir: true, mode: perm | fs.ModeDir})
	}
	return entries
}

// directChild returns the name of a direct child of the directory identified by
// prefix, and true if k is indeed a direct child. Returns "", false otherwise.
func directChild(k, prefix string) (string, bool) {
	if !strings.HasPrefix(k, prefix) {
		return "", false
	}
	rest := k[len(prefix):]
	if strings.Contains(rest, "/") {
		return "", false
	}
	return rest, true
}

// WalkDir walks the file tree rooted at root, calling fn for each file or directory.
func (m *MemFS) WalkDir(root string, fn fs.WalkDirFunc) error {
	root = norm(root)

	m.mu.RLock()
	// Collect all paths under root (snapshot).
	paths := m.collectPaths(root)
	m.mu.RUnlock()

	sort.Strings(paths)

	for _, p := range paths {
		m.mu.RLock()
		info, err := m.lstatLocked(p)
		m.mu.RUnlock()

		var de os.DirEntry
		if info != nil {
			de = &memDirEntry{
				name:  path.Base(p),
				isDir: info.IsDir(),
				mode:  info.Mode(),
			}
		}

		if walkErr := fn(p, de, err); walkErr != nil {
			if errors.Is(walkErr, fs.SkipDir) {
				// Skip remaining entries under this directory.
				continue
			}
			return walkErr
		}
	}
	return nil
}

// collectPaths collects all paths under root (must be called with lock held).
// root must already be normalized.
func (m *MemFS) collectPaths(root string) []string {
	seen := make(map[string]bool)
	prefix := dirPrefix(root)

	addPath := func(p string) {
		if p == root || strings.HasPrefix(p, prefix) {
			seen[p] = true
		}
	}

	if m.dirs[root] {
		seen[root] = true
	}
	for k := range m.files {
		addPath(k)
	}
	for k := range m.dirs {
		addPath(k)
	}
	for k := range m.symlinks {
		addPath(k)
	}

	result := make([]string, 0, len(seen))
	for p := range seen {
		result = append(result, p)
	}
	return result
}

// lstatLocked returns FileInfo without following symlinks (must be called with
// lock held). name must already be normalized.
func (m *MemFS) lstatLocked(name string) (fs.FileInfo, error) {
	if _, ok := m.symlinks[name]; ok {
		return &memFileInfo{
			name:  path.Base(name),
			size:  0,
			mode:  fs.ModeSymlink | 0o777,
			isDir: false,
		}, nil
	}
	if data, ok := m.files[name]; ok {
		perm, ok := m.perms[name]
		if !ok {
			perm = 0o644
		}
		return &memFileInfo{
			name:  path.Base(name),
			size:  int64(len(data)),
			mode:  perm,
			isDir: false,
		}, nil
	}
	if m.dirs[name] {
		perm, ok := m.perms[name]
		if !ok {
			perm = 0o755
		}
		return &memFileInfo{
			name:  path.Base(name),
			size:  0,
			mode:  perm | fs.ModeDir,
			isDir: true,
		}, nil
	}
	return nil, pathError("lstat", name, os.ErrNotExist)
}

// memFileInfo implements fs.FileInfo.
type memFileInfo struct {
	name  string
	size  int64
	mode  fs.FileMode
	isDir bool
}

func (f *memFileInfo) Name() string       { return f.name }
func (f *memFileInfo) Size() int64        { return f.size }
func (f *memFileInfo) Mode() fs.FileMode  { return f.mode }
func (f *memFileInfo) ModTime() time.Time { return time.Time{} }
func (f *memFileInfo) IsDir() bool        { return f.isDir }
func (f *memFileInfo) Sys() any           { return nil }

// memDirEntry implements os.DirEntry.
type memDirEntry struct {
	name  string
	isDir bool
	mode  fs.FileMode
}

func (d *memDirEntry) Name() string { return d.name }
func (d *memDirEntry) IsDir() bool  { return d.isDir }
func (d *memDirEntry) Type() fs.FileMode {
	return d.mode.Type()
}
func (d *memDirEntry) Info() (fs.FileInfo, error) {
	return &memFileInfo{
		name:  d.name,
		size:  0,
		mode:  d.mode,
		isDir: d.isDir,
	}, nil
}
