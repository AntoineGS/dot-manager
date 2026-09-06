package fsys

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
)

// OsFS is the real filesystem implementation that delegates to os and filepath.
type OsFS struct{}

// Stat returns a FileInfo describing the named file.
func (OsFS) Stat(name string) (fs.FileInfo, error) {
	return os.Stat(name)
}

// Lstat returns a FileInfo describing the named file, without following symlinks.
func (OsFS) Lstat(name string) (fs.FileInfo, error) {
	return os.Lstat(name)
}

// ReadFile reads and returns the content of the named file.
func (OsFS) ReadFile(name string) ([]byte, error) {
	return os.ReadFile(name)
}

// WriteFile writes data to the named file, creating it if needed.
func (OsFS) WriteFile(name string, data []byte, perm fs.FileMode) error {
	return os.WriteFile(name, data, perm)
}

// WriteFileExclusive creates and writes a file only when name is unoccupied.
func (OsFS) WriteFileExclusive(name string, data []byte, perm fs.FileMode) error {
	f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}

	_, writeErr := f.Write(data)
	closeErr := f.Close()
	if err := errors.Join(writeErr, closeErr); err != nil {
		// Exclusive creation succeeded, so this is our incomplete file to remove.
		return errors.Join(err, os.Remove(name))
	}

	return nil
}

// Chmod changes the permission bits of name.
func (OsFS) Chmod(name string, mode fs.FileMode) error {
	return os.Chmod(name, mode)
}

// MkdirAll creates path and all necessary parents.
func (OsFS) MkdirAll(path string, perm fs.FileMode) error {
	return os.MkdirAll(path, perm)
}

// Remove removes the named file or empty directory.
func (OsFS) Remove(name string) error {
	return os.Remove(name)
}

// RemoveAll removes path and any children it contains.
func (OsFS) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

// Symlink creates newname as a symbolic link to oldname.
func (OsFS) Symlink(oldname, newname string) error {
	return os.Symlink(oldname, newname)
}

// Readlink returns the destination of the named symbolic link.
func (OsFS) Readlink(name string) (string, error) {
	return os.Readlink(name)
}

// Rename renames (moves) oldpath to newpath.
func (OsFS) Rename(oldpath, newpath string) error {
	return os.Rename(oldpath, newpath)
}

// ReadDir reads the named directory, returning its directory entries sorted by filename.
func (OsFS) ReadDir(name string) ([]os.DirEntry, error) {
	return os.ReadDir(name)
}

// WalkDir walks the file tree rooted at root, calling fn for each file or directory.
func (OsFS) WalkDir(root string, fn fs.WalkDirFunc) error {
	return filepath.WalkDir(root, fn)
}
