package session

import (
	"hash/fnv"
	"path/filepath"
	"strings"
)

// permissionPrompts are texts that agents show while waiting for permission
// to run a tool, keyed by program name.
var permissionPrompts = map[string][]string{
	"claude": {"No, and tell Claude what to do differently"},
	"aider":  {"(Y)es/(N)o/(D)on't ask again"},
	"gemini": {"Yes, allow once"},
}

// trustPrompts are texts agents show when first opened in an unknown
// directory, which every new worktree is.
var trustPrompts = []string{
	"Do you trust the files in this folder?",
}

// ProgramName returns the executable name of a launch command.
func ProgramName(program string) string {
	fields := strings.Fields(program)
	if len(fields) == 0 {
		return ""
	}
	return filepath.Base(fields[0])
}

// AwaitingPermission reports whether screen shows the program asking for
// permission to act. Unknown programs never match.
func AwaitingPermission(program, screen string) bool {
	for _, prompt := range permissionPrompts[ProgramName(program)] {
		if strings.Contains(screen, prompt) {
			return true
		}
	}
	return false
}

// AwaitingTrust reports whether screen shows a workspace trust prompt.
func AwaitingTrust(screen string) bool {
	for _, prompt := range trustPrompts {
		if strings.Contains(screen, prompt) {
			return true
		}
	}
	return false
}

// ScreenHash fingerprints a captured screen. Comparing fingerprints between
// polls is how Vineyard tells a working agent from an idle one: agents animate
// while they work and sit still while they wait.
func ScreenHash(screen string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(screen))
	return h.Sum64()
}
