//go:build !windows

package contentfs

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// removeOpenedLifecycleFile removes exactly the file behind the verified
// handle. POSIX has no unlink-by-descriptor, so the rooted name is first
// renamed atomically to a private sibling in the same directory. Only after
// the renamed entry is confirmed to be the same inode as the verified handle is
// it removed; a name replaced between verification and rename is never deleted.
func removeOpenedLifecycleFile(root *os.Root, name string, file *os.File) error {
	opened, err := file.Stat()
	if err != nil {
		return fmt.Errorf("stat verified handle: %w", err)
	}
	suffix := make([]byte, 16)
	if _, err := rand.Read(suffix); err != nil {
		return fmt.Errorf("generate removal name: %w", err)
	}
	private := filepath.Join(filepath.Dir(name), ".leanote-remove-"+hex.EncodeToString(suffix))
	if err := root.Rename(name, private); err != nil {
		return fmt.Errorf("detach verified name: %w", err)
	}
	detached, err := root.Lstat(private)
	if err != nil {
		return fmt.Errorf("stat detached name: %w", err)
	}
	if !os.SameFile(opened, detached) {
		// Another writer replaced the name after verification. Put its file
		// back only when the original name is still free, and report conflict.
		restoreErr := os.ErrExist
		if _, statErr := root.Lstat(name); errors.Is(statErr, os.ErrNotExist) {
			restoreErr = root.Rename(private, name)
		}
		return errors.Join(errors.New("verified file was replaced before removal"), restoreErr)
	}
	return root.Remove(private)
}
