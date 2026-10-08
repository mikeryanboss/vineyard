//go:build !unix

package session

// Lock is a no-op where flock is unavailable. Vineyard depends on tmux, so
// such platforms are unsupported anyway; this keeps the module compiling.
func Lock(projectDir string) (release func(), err error) {
	return func() {}, nil
}
