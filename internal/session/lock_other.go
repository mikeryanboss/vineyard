//go:build !unix

package session

import "errors"

var errBusy = errors.New("lock held by another process")

// lock is a no-op where flock is unavailable. Vineyard depends on tmux, so
// such platforms are unsupported anyway; this keeps the module compiling.
func lock(path string, wait bool) (release func(), err error) {
	return func() {}, nil
}
