//go:build windows

package contentfs

import "io/fs"

// Windows reports synthetic POSIX permission bits even after Chmod. The
// manifest is still created with 0600; ACL validation is delivery evidence.
func manifestModeSafe(info fs.FileInfo) bool {
	return info.Mode().IsRegular()
}
