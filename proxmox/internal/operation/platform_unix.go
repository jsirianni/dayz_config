//go:build linux

package operation

import (
	"golang.org/x/sys/unix"
	"os"
)

func SecureDir(path string) error {
	if e := os.MkdirAll(path, 0700); e != nil {
		return e
	}
	return os.Chmod(path, 0700)
}
func lockFile(f *os.File) error { return unix.Flock(int(f.Fd()), unix.LOCK_EX|unix.LOCK_NB) }
func unlockFile(f *os.File)     { _ = unix.Flock(int(f.Fd()), unix.LOCK_UN) }
