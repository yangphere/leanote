//go:build !windows

package contentfs

import (
	"errors"
	"os"
)

func removeOpenedLifecycleFile(*os.File) error {
	return errors.New("exact lifecycle removal by verified handle is unsupported on this platform")
}
