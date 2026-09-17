//go:build windows

package contentfs

import (
	"errors"
	"os"
	"path/filepath"
	"unsafe"

	"golang.org/x/sys/windows"
)

// Windows commits a published business entry by independently reopening its
// final root-relative path and flushing that file handle. Directory creation
// is provisional until this barrier succeeds; losing an empty parent is safe.
func platformPublicationBarrier(root *os.Root, name string, ops durabilityOps) error {
	file, err := ops.openFinal(root, name)
	if err != nil {
		return err
	}
	return errors.Join(ops.syncFinal(file), ops.closeFinal(file))
}

func platformParentBarrier(*os.Root, string, durabilityOps) error { return nil }

// Windows cannot durably flush removal through os.Root. Staging cleanup and
// terminal GC therefore rely on idempotent reappearance-and-retry semantics;
// this is not a publication barrier for a business mutation.
func platformRemovalBarrier(*os.Root, string, durabilityOps) error { return nil }

func platformBusinessRemovalBarrier(root *os.Root, name string, _ durabilityOps) error {
	parent := filepath.Dir(name)
	base, err := root.Open(filepath.Dir(parent))
	if err != nil {
		return err
	}
	objectName, err := windows.NewNTUnicodeString(filepath.Base(parent))
	if err != nil {
		return errors.Join(err, base.Close())
	}
	attributes := &windows.OBJECT_ATTRIBUTES{
		Length:        uint32(unsafe.Sizeof(windows.OBJECT_ATTRIBUTES{})),
		RootDirectory: windows.Handle(base.Fd()),
		ObjectName:    objectName,
		Attributes:    windows.OBJ_CASE_INSENSITIVE | windows.OBJ_DONT_REPARSE,
	}
	var handle windows.Handle
	var status windows.IO_STATUS_BLOCK
	err = windows.NtCreateFile(
		&handle,
		windows.FILE_GENERIC_READ|windows.FILE_GENERIC_WRITE,
		attributes,
		&status,
		nil,
		0,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE,
		windows.FILE_OPEN,
		windows.FILE_DIRECTORY_FILE|windows.FILE_SYNCHRONOUS_IO_NONALERT|windows.FILE_OPEN_FOR_BACKUP_INTENT|windows.FILE_OPEN_REPARSE_POINT,
		0,
		0,
	)
	if err != nil {
		return errors.Join(err, base.Close())
	}
	return errors.Join(windows.FlushFileBuffers(handle), windows.CloseHandle(handle), base.Close())
}
