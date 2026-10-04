//go:build !windows

package contentfs

import (
	"os"
	"path/filepath"
)

func platformPublicationBarrier(root *os.Root, name string, ops durabilityOps) error {
	return ops.directorySync(root, filepath.Dir(name))
}

func platformParentBarrier(root *os.Root, parent string, ops durabilityOps) error {
	return ops.directorySync(root, parent)
}

func platformRemovalBarrier(root *os.Root, name string, ops durabilityOps) error {
	return ops.directorySync(root, filepath.Dir(name))
}

func platformBusinessRemovalBarrier(root *os.Root, name string, ops durabilityOps) error {
	return ops.directorySync(root, filepath.Dir(name))
}
