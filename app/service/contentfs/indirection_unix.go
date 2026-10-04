//go:build !windows

package contentfs

import "os"

func isIndirection(info os.FileInfo) bool {
	return info.Mode()&os.ModeSymlink != 0
}
