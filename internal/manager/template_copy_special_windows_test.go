//go:build windows

package manager

import "testing"

func createTemplateCopySpecialFile(t *testing.T, _ string) {
	t.Skip("special files are not portable to Windows")
}
