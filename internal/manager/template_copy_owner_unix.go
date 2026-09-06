//go:build !windows

package manager

import (
	"io/fs"
	"reflect"
)

// fileRootOwned reports whether native file metadata identifies the file as
// owned by uid 0. A missing or unsupported Sys value is treated as unknown and
// therefore not root-owned.
func fileRootOwned(info fs.FileInfo) bool {
	uid, ok := fileStatUint(info, "Uid", "UID", "Uid_t")
	return ok && uid == 0
}

// fileIdentity returns the device and inode from native file metadata when
// both fields are available.
func fileIdentity(info fs.FileInfo) (device, inode uint64, known bool) {
	device, deviceKnown := fileStatUint(info, "Dev", "Dev_t", "Device")
	inode, inodeKnown := fileStatUint(info, "Ino", "Inode")
	return device, inode, deviceKnown && inodeKnown
}

func fileStatUint(info fs.FileInfo, names ...string) (uint64, bool) {
	if info == nil || info.Sys() == nil {
		return 0, false
	}

	value := reflect.ValueOf(info.Sys())
	for value.Kind() == reflect.Pointer || value.Kind() == reflect.Interface {
		if value.IsNil() {
			return 0, false
		}
		value = value.Elem()
	}
	if value.Kind() != reflect.Struct {
		return 0, false
	}

	for _, name := range names {
		field := value.FieldByName(name)
		if !field.IsValid() {
			continue
		}

		//nolint:exhaustive // only numeric metadata fields are relevant here
		switch field.Kind() {
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr:
			return field.Uint(), true
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			integer := field.Int()
			if integer < 0 {
				return 0, false
			}
			return uint64(integer), true
		default:
			continue
		}
	}

	return 0, false
}
