//go:build unix

package session

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// errBusy reports that another process holds a lock taken without waiting.
var errBusy = errors.New("lock held by another process")

// lock takes an exclusive flock on path, creating the file. With wait, it
// blocks until the lock is free; without, it fails with errBusy. The lock is
// released by calling the returned function, or when the process exits.
func lock(path string, wait bool) (release func(), err error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	how := syscall.LOCK_EX
	if !wait {
		how |= syscall.LOCK_NB
	}
	if err := syscall.Flock(int(f.Fd()), how); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, errBusy
		}
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}
