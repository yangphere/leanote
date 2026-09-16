//go:build !windows

package contentpdf

import "io/fs"

func isIndirection(info fs.FileInfo) bool {
	return info.Mode()&fs.ModeSymlink != 0
}

func isExecutable(info fs.FileInfo, _ string) bool {
	return info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}
