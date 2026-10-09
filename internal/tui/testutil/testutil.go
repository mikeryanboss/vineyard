// Package testutil provides golden-file helpers and fixtures for TUI tests.
// It follows grapes' approach: golden files hold plain text with ANSI codes
// stripped, so they are readable and survive colour changes.
//
//	go test ./internal/tui/... -update   # regenerate golden files
//	go test ./internal/tui/...           # compare against golden files
package testutil

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/mikeryanboss/vineyard/internal/session"
)

var update = flag.Bool("update", false, "update golden files")

// ansiRE matches CSI and OSC escape sequences.
var ansiRE = regexp.MustCompile(`\x1b\[[0-9;:?]*[a-zA-Z]|\x1b\][^\x07\x1b]*(\x07|\x1b\\)`)

// StripANSI removes ANSI escape sequences from s.
func StripANSI(s string) string {
	return ansiRE.ReplaceAllString(s, "")
}

// RequireGolden compares got, with ANSI stripped, to testdata/<test name>.golden.
// With -update it rewrites the golden file instead.
func RequireGolden(t *testing.T, got string) {
	t.Helper()
	name := strings.ReplaceAll(t.Name(), "/", "__")
	path := filepath.Join("testdata", name+".golden")
	clean := StripANSI(got)

	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(clean), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}

	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("golden file %s missing; create it with:\n  go test -run %s -update", path, t.Name())
	}
	if clean == string(want) {
		return
	}
	gotLines, wantLines := strings.Split(clean, "\n"), strings.Split(string(want), "\n")
	var diff strings.Builder
	for i := 0; i < max(len(gotLines), len(wantLines)); i++ {
		var g, w string
		if i < len(gotLines) {
			g = gotLines[i]
		}
		if i < len(wantLines) {
			w = wantLines[i]
		}
		if g != w {
			fmt.Fprintf(&diff, "--- want %d: %s\n+++ got  %d: %s\n", i+1, w, i+1, g)
		}
	}
	t.Errorf("golden mismatch %s; accept with:\n  go test -run %s -update\n\n%s", path, t.Name(), diff.String())
}

// Key returns a key press for a printable key such as "n", or a named key
// such as "enter", "tab", "shift+tab", "esc", "up", or "ctrl+c".
func Key(k string) tea.KeyPressMsg {
	named := map[string]tea.Key{
		"enter":     {Code: tea.KeyEnter},
		"tab":       {Code: tea.KeyTab},
		"shift+tab": {Code: tea.KeyTab, Mod: tea.ModShift},
		"esc":       {Code: tea.KeyEscape},
		"up":        {Code: tea.KeyUp},
		"down":      {Code: tea.KeyDown},
		"left":      {Code: tea.KeyLeft},
		"right":     {Code: tea.KeyRight},
		"space":     {Code: tea.KeySpace, Text: " "},
		"ctrl+c":    {Code: 'c', Mod: tea.ModCtrl},
		"ctrl+d":    {Code: 'd', Mod: tea.ModCtrl},
		"ctrl+u":    {Code: 'u', Mod: tea.ModCtrl},
		"ctrl+s":    {Code: 's', Mod: tea.ModCtrl},
		"backspace": {Code: tea.KeyBackspace},
	}
	if key, ok := named[k]; ok {
		return tea.KeyPressMsg(key)
	}
	r := []rune(k)[0]
	return tea.KeyPressMsg(tea.Key{Code: r, Text: k})
}

// Sessions returns a fixed set of sessions in every status.
func Sessions() []session.Session {
	created := time.Date(2026, 10, 1, 9, 0, 0, 0, time.UTC)
	return []session.Session{
		{ID: "login-1", Title: "Fix login redirect", Program: "claude", Branch: "mboss/fix-login-redirect", Status: session.StatusRunning, CreatedAt: created},
		{ID: "tests-2", Title: "Add unit tests for the parser", Program: "claude", Branch: "mboss/add-unit-tests-for-the-parser", Status: session.StatusReady, AutoYes: true, CreatedAt: created},
		{ID: "docs-3", Title: "Docs pass", Program: "codex", Branch: "mboss/docs-pass", Status: session.StatusPaused, CreatedAt: created},
		{ID: "cache-4", Title: "Cache layer", Program: "aider", Branch: "mboss/cache-layer", Status: session.StatusStopped, CreatedAt: created},
		{ID: "new-5", Title: "Starting up", Program: "claude", Status: session.StatusLoading, CreatedAt: created},
	}
}
