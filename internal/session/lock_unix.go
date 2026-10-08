//go:build unix

package session

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
)

// Lock takes exclusive ownership of a project for this process. Two Vineyard
// processes on one repository would overwrite each other's session state.
// The lock is released by calling the returned function, or when the process exits.
func Lock(projectDir string) (release func(), err error) {
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(filepath.Join(projectDir, "lock"), os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrLocked
		}
		return nil, err
	}
	return func() {
		_ = syscall.Flock(int(f.Fd()), syscall.LOCK_UN)
		_ = f.Close()
	}, nil
}
