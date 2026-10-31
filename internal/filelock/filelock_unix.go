//go:build !windows

package filelock

import (
	"errors"
	"os"
	"syscall"
)

func lockFile(f *os.File, block bool) error {
	how := syscall.LOCK_EX
	if !block {
		how |= syscall.LOCK_NB
	}
	for {
		err := syscall.Flock(int(f.Fd()), how)
		if !errors.Is(err, syscall.EINTR) {
			return err
		}
	}
}

func unlockFile(f *os.File) error {
	return syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
}

func isWouldBlock(err error) bool {
	return errors.Is(err, syscall.EWOULDBLOCK)
}
