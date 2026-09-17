//go:build !windows && !linux

package contentfs

import (
	"errors"
	"os"
)

func tryLockFile(_ *os.File) (func() error, bool, error) {
	return nil, false, errors.New("content manifest leases require Windows or Linux kernel file locking")
}
