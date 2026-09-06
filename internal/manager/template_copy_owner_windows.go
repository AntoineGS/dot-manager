//go:build windows

package manager

import "io/fs"

// Windows metadata does not expose the Unix uid/device/inode tuple used by the
// copy safety checks. Callers must use their existing native same-location
// checks instead.
func fileRootOwned(_ fs.FileInfo) bool {
	return false
}

func fileIdentity(_ fs.FileInfo) (device, inode uint64, known bool) {
	return 0, 0, false
}
