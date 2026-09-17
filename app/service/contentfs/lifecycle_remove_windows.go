//go:build windows

package contentfs

import (
	"errors"
	"os"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var reOpenFile = syscall.NewLazyDLL("kernel32.dll").NewProc("ReOpenFile")

type fileDispositionInfoEx struct {
	Flags uint32
}

func removeOpenedLifecycleFile(file *os.File) error {
	handle, _, callErr := reOpenFile.Call(
		file.Fd(),
		windows.DELETE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		0,
	)
	if windows.Handle(handle) == windows.InvalidHandle {
		return callErr
	}
	reopened := windows.Handle(handle)
	info := fileDispositionInfoEx{Flags: windows.FILE_DISPOSITION_DELETE | windows.FILE_DISPOSITION_POSIX_SEMANTICS | windows.FILE_DISPOSITION_IGNORE_READONLY_ATTRIBUTE}
	err := windows.SetFileInformationByHandle(
		reopened,
		windows.FileDispositionInfoEx,
		(*byte)(unsafe.Pointer(&info)),
		uint32(unsafe.Sizeof(info)),
	)
	return errors.Join(err, windows.CloseHandle(reopened))
}
