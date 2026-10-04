//go:build windows

package contentpdf

import (
	"io/fs"
	"path/filepath"
	"strings"
	"syscall"
)

const fileAttributeReparsePoint = 0x400

func isIndirection(info fs.FileInfo) bool {
	if info.Mode()&fs.ModeSymlink != 0 {
		return true
	}
	data, ok := info.Sys().(*syscall.Win32FileAttributeData)
	return ok && data.FileAttributes&fileAttributeReparsePoint != 0
}

func isExecutable(info fs.FileInfo, name string) bool {
	extension := strings.ToLower(filepath.Ext(name))
	return info.Mode().IsRegular() && (extension == ".exe" || extension == ".com")
}
