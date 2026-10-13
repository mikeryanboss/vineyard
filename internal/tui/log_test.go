package tui

import (
	"bytes"
	"errors"
	"log"
	"strings"
	"testing"
)

// Errors vanish from the status line, so the log keeps them: once while a
// failing poll repeats one. It also keeps a panic's stack, which Bubble Tea
// prints only to the terminal it restores.
func TestApp_LogsErrorsAndPanics(t *testing.T) {
	var buf bytes.Buffer
	out := log.Writer()
	log.SetOutput(&buf)
	t.Cleanup(func() { log.SetOutput(out) })

	m, _ := newTestModel(t, nil)
	for range 3 {
		m = update(m, reloadedMsg{err: errors.New("disk full")})
	}
	update(m, savedMsg{err: errors.New("saving sessions: disk full")})
	if n := strings.Count(buf.String(), "reloading sessions: disk full"); n != 1 {
		t.Errorf("the repeated error is logged %d times, want once:\n%s", n, buf.String())
	}
	if !strings.Contains(buf.String(), "saving sessions: disk full") {
		t.Errorf("the log lacks the second error:\n%s", buf.String())
	}

	func() {
		defer func() { recover() }()
		defer logPanic()
		var sizes map[string]int
		sizes["a"] = 1
	}()
	for _, want := range []string{"panic: assignment to entry in nil map", "TestApp_LogsErrorsAndPanics"} {
		if !strings.Contains(buf.String(), want) {
			t.Errorf("the log lacks %q:\n%s", want, buf.String())
		}
	}
}
