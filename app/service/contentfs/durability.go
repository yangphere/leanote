package contentfs

import "os"

type durabilityOps struct {
	openFinal     func(*os.Root, string) (*os.File, error)
	syncFinal     func(*os.File) error
	closeFinal    func(*os.File) error
	directorySync func(*os.Root, string) error
}

func newDurabilityOps(
	directorySync func(*os.Root, string) error,
	openFinal func(*os.Root, string) (*os.File, error),
	syncFinal func(*os.File) error,
	closeFinal func(*os.File) error,
) durabilityOps {
	if openFinal == nil {
		openFinal = func(root *os.Root, name string) (*os.File, error) {
			return root.OpenFile(name, os.O_RDWR, 0)
		}
	}
	if syncFinal == nil {
		syncFinal = func(file *os.File) error { return file.Sync() }
	}
	if closeFinal == nil {
		closeFinal = func(file *os.File) error { return file.Close() }
	}
	return durabilityOps{openFinal: openFinal, syncFinal: syncFinal, closeFinal: closeFinal, directorySync: directorySync}
}
