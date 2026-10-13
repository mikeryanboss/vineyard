package vterm

import (
	"strings"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/mikeryanboss/vineyard/internal/session"
)

// The bar names the session attached to first, so the user knows where they
// are. It starts from the statuses Vineyard sent and then samples the
// screens itself: a changing screen is running, and one that ran during the
// attachment and then sits still has finished. Ended terminals drop out.
// When the titles do not fit, it counts the sessions of each status, after
// the attached session's title.
func TestBarFollowsTheOtherSessions(t *testing.T) {
	s := New()
	start(t, s, "busy", `while :; do date +%s%N; sleep 0.02; done`)
	start(t, s, "idle", `echo idle; sleep 10`)
	waitFor(t, func() (string, error) { return s.Capture("idle") }, "idle")
	b := newBar(true, "Fix login", []session.Peer{
		{Terminal: "busy", Title: "Busy agent"},
		{Terminal: "idle", Title: "Idle agent", Running: true},
		{Terminal: "gone", Title: "Gone agent"},
	}, []string{"ctrl+q"}, []string{"ctrl+\\"}, true)
	text := func(width int) string {
		t.Helper()
		got := b.text(width)
		if w := ansi.StringWidth(got); w != width {
			t.Errorf("the bar is %d cells wide, want %d: %q", w, width, ansi.Strip(got))
		}
		return ansi.Strip(got)
	}

	b.sample(s)
	if got := text(80); !strings.HasPrefix(got, " Fix login │ ● Busy agent  ◑ Idle agent ") || !strings.HasSuffix(got, " ctrl+q back  ctrl+\\ hide ") || strings.Contains(got, "Gone") {
		t.Errorf("before a second sample the bar should show the statuses sent:\n%q", got)
	}
	time.Sleep(200 * time.Millisecond)
	b.sample(s)
	if got := text(80); !strings.HasPrefix(got, " Fix login │ ◑ Busy agent  ✓ Idle agent ") {
		t.Errorf("busy should run and idle have finished:\n%q", got)
	}
	if got := text(30); !strings.HasPrefix(got, " Fix login │ ◑ 1  ✓ 1 ") {
		t.Errorf("a narrow bar should count the sessions:\n%q", got)
	}
}

// The bar is darker than a light terminal's background and lighter than a
// dark one's, and says when it has no session to list.
func TestBarSuitsTheBackground(t *testing.T) {
	for _, c := range []struct {
		dark bool
		want string
	}{{false, "48;5;254"}, {true, "48;5;238"}} {
		b := newBar(true, "Fix login", nil, []string{"ctrl+q"}, nil, c.dark)
		got := b.text(40)
		if !strings.Contains(got, c.want) {
			t.Errorf("dark=%v: the bar lacks background %s: %q", c.dark, c.want, got)
		}
		if !strings.HasPrefix(ansi.Strip(got), " Fix login │ no other sessions ") {
			t.Errorf("dark=%v: an empty bar should say so: %q", c.dark, ansi.Strip(got))
		}
	}
}
