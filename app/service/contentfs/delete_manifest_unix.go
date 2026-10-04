//go:build !windows

package contentfs

import "io/fs"

func manifestModeSafe(info fs.FileInfo) bool {
	return info.Mode().IsRegular() && info.Mode().Perm() == 0o600
}
