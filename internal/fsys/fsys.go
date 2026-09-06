package fsys

import (
	"io/fs"
	"os"
)

// FS abstracts filesystem operations used across the codebase.
type FS interface {
	Stat(name string) (fs.FileInfo, error)
	Lstat(name string) (fs.FileInfo, error)
	ReadFile(name string) ([]byte, error)
	WriteFile(name string, data []byte, perm fs.FileMode) error
	WriteFileExclusive(name string, data []byte, perm fs.FileMode) error
	Chmod(name string, mode fs.FileMode) error
	MkdirAll(path string, perm fs.FileMode) error
	Remove(name string) error
	RemoveAll(path string) error
	Symlink(oldname, newname string) error
	Readlink(name string) (string, error)
	Rename(oldpath, newpath string) error
	ReadDir(name string) ([]os.DirEntry, error)
	WalkDir(root string, fn fs.WalkDirFunc) error
}
