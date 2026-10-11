package subagent

import (
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
	"testing"
	"time"
)

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name + ".ansi")
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// The fixtures are screens Claude Code 2.1.296 drew, captured with
// `tmux capture-pane -p -e`.
func TestParse_ClaudeCodeScreens(t *testing.T) {
	row := func(label string) Row { return Row{Type: "general-purpose", Label: label} }
	nested := func(label string) Row { return Row{Type: "general-purpose", Label: label, Depth: 1} }
	sleeping := row("Running sleep 8 (3 of 10)")
	tests := []struct {
		name  string
		want  List
		found bool
	}{
		{"more-below", List{Rows: []Row{
			row("Worker one sleeps"), row("Worker two sleeps"), row("Worker three sleeps"),
			row("Worker four sleeps"), row("Worker five sleeps"),
		}, Below: 1, Shown: 0, Focus: -1}, true},
		{"focused", List{Rows: []Row{
			row("Waiting on sleep call 3"), row("Running sleep 8 (run 3 of 10)"), sleeping,
			row("Running third sleep 8 call"), sleeping,
		}, Below: 1, Shown: 0, Focus: 0}, true},
		{"scrolled", List{Rows: []Row{
			row("Running sleep 8 (run 3 of 10)"), sleeping, row("Running third sleep 8 call"), sleeping, sleeping,
		}, Above: 1, Shown: 0, Focus: 6}, true},
		{"inline", List{Rows: []Row{row("inline one"), row("inline two")}, Shown: 0, Focus: -1}, true},
		{"inline-view", List{Rows: []Row{row("inline one"), row("inline two")}, Shown: 1, Focus: 1}, true},
		{"nested", List{Rows: []Row{
			row("Pausing for 8-second sleep"), nested("Running third of eight sleeps"),
			nested("Running sleep 8 (3 of 8)"), nested("Running sleep 8 (3 of 8)"),
		}, Shown: 1, Focus: 0}, true},
		{"nested-view", List{Rows: []Row{
			row("Pausing for 8-second sleep"), nested("Running third of eight sleeps"),
			nested("Running sleep 8 (3 of 8)"), nested("Running sleep 8 (3 of 8)"),
		}, Shown: 2, Focus: 2}, true},
		{"none", List{}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, found := Parse(fixture(t, tt.name))
			if found != tt.found {
				t.Fatalf("found = %v, want %v", found, tt.found)
			}
			if fmt.Sprint(got) != fmt.Sprint(tt.want) {
				t.Errorf("Parse =\n%+v\nwant\n%+v", got, tt.want)
			}
		})
	}
}

// Transcribed from Claude Code 2.1.296's main view while a subagent ran two
// of its own.
func TestParse_CountsSubagentsASubagentStarted(t *testing.T) {
	screen := "  ● main\n  ◯ general-purpose (+2)  lead                     6s · ↓ 23.5k tokens\n"
	got, _ := Parse(screen)
	want := []Row{{Type: "general-purpose", Label: "lead", Hidden: 2}}
	if !slices.Equal(got.Rows, want) {
		t.Errorf("rows = %+v, want %+v", got.Rows, want)
	}
}

func TestParse_IgnoresMessageBullets(t *testing.T) {
	screen := "● main is the default branch.\n  ◯ general-purpose\n● done"
	if _, found := Parse(screen); found {
		t.Error("message text is not Claude Code's agent list")
	}
}

// fakeClaude models Claude Code's agent list and prompt as observed on 2.1.296.
type fakeClaude struct {
	labels []string
	// draft is the number of draft lines below the prompt's cursor.
	draft int
	// shown and focus are positions; focus is -1 while the prompt has focus.
	shown, focus int
	top          int // first drawn subagent, 0-based
	// ignoreEnter makes opening a view fail; finishOnEnter makes every
	// subagent finish instead.
	ignoreEnter, finishOnEnter bool
	question                   bool // shows a permission prompt
	keys                       []string
	cancelled                  bool // Esc reached the prompt
}

const drawn = 5

func (f *fakeClaude) Capture() (string, error) {
	var b strings.Builder
	b.WriteString("● Waiting for the workers.\n❯ \n")
	if f.question {
		b.WriteString("❯ 1. Yes\n  3. No, and tell Claude what to do differently (esc)\n")
	}
	row := func(position int, text string) {
		cursor, dot := " ", "◯"
		if f.focus == position {
			cursor = "❯"
		}
		if f.shown == position {
			dot = "●"
		}
		fmt.Fprintf(&b, "%s %s %s\n", cursor, dot, text)
	}
	main := "main"
	if f.top > 0 {
		main += fmt.Sprintf("                ↑ %d more", f.top)
	}
	row(0, main)
	end := min(len(f.labels), f.top+drawn)
	for i := f.top; i < end; i++ {
		row(i+1, "general-purpose  "+f.labels[i]+"          5s · ↓ 19.4k tokens")
	}
	if n := len(f.labels) - end; n > 0 {
		fmt.Fprintf(&b, "  ↓ %d more\n", n)
	}
	return b.String(), nil
}

func (f *fakeClaude) SendKeys(keys ...string) error {
	f.keys = append(f.keys, keys...)
	for _, k := range keys {
		switch {
		case k == "Down" && f.focus < 0 && f.draft > 0:
			f.draft--
		case k == "Down" && f.focus < 0:
			f.focus = f.shown
		case k == "Down":
			f.focus = min(f.focus+1, len(f.labels))
		case k == "Up" && f.focus >= 0:
			f.focus = max(f.focus-1, 0)
		case k == "Enter" && f.finishOnEnter:
			f.labels, f.focus, f.shown = nil, -1, 0
		case k == "Enter" && f.focus >= 0 && !f.ignoreEnter:
			f.shown = f.focus
		case k == "Escape" && f.focus >= 0:
			f.focus = -1
		case k == "Escape":
			f.cancelled = true
		}
		// The list scrolls to keep the selection drawn.
		if f.focus > 0 && f.focus-1 < f.top {
			f.top = f.focus - 1
		}
		if f.focus > 0 && f.focus-1 >= f.top+drawn {
			f.top = f.focus - drawn
		}
	}
	return nil
}

func newFake(n int) *fakeClaude {
	f := &fakeClaude{focus: -1}
	for i := range n {
		f.labels = append(f.labels, fmt.Sprintf("worker %d", i+1))
	}
	return f
}

func fastWaits(t *testing.T) {
	t.Helper()
	k, v, p := keyWait, viewWait, pollEvery
	keyWait, viewWait, pollEvery = 20*time.Millisecond, 20*time.Millisecond, time.Millisecond
	t.Cleanup(func() { keyWait, viewWait, pollEvery = k, v, p })
}

func TestOpen_OpensTheSubagentAndGivesThePromptFocus(t *testing.T) {
	fastWaits(t)
	f := newFake(3)
	if err := Open(f, 2, 3); err != nil {
		t.Fatal(err)
	}
	if f.shown != 2 || f.focus != -1 {
		t.Errorf("shown = %d, focus = %d; want subagent 2 shown and the prompt focused", f.shown, f.focus)
	}
	if want := []string{"Down", "Down", "Down", "Enter", "Escape"}; !slices.Equal(f.keys, want) {
		t.Errorf("keys = %q, want %q", f.keys, want)
	}
}

func TestOpen_WalksDownAMultiLineDraft(t *testing.T) {
	fastWaits(t)
	f := newFake(2)
	f.draft = 3
	if err := Open(f, 1, 2); err != nil {
		t.Fatal(err)
	}
	if f.shown != 1 || f.draft != 0 || f.cancelled {
		t.Errorf("shown = %d, draft lines left = %d, cancelled = %v", f.shown, f.draft, f.cancelled)
	}
}

func TestOpen_ScrollsToASubagentOutOfSight(t *testing.T) {
	fastWaits(t)
	f := newFake(7)
	if err := Open(f, 7, 7); err != nil {
		t.Fatal(err)
	}
	if f.shown != 7 {
		t.Errorf("shown = %d, want 7", f.shown)
	}
}

func TestOpen_FromAnotherSubagentsView(t *testing.T) {
	fastWaits(t)
	f := newFake(4)
	f.shown = 4
	if err := Open(f, 1, 4); err != nil {
		t.Fatal(err)
	}
	if f.shown != 1 || f.focus != -1 {
		t.Errorf("shown = %d, focus = %d", f.shown, f.focus)
	}
}

func TestOpen_RefusesAChangedList(t *testing.T) {
	fastWaits(t)
	f := newFake(2)
	if err := Open(f, 2, 3); !errors.Is(err, ErrChanged) {
		t.Fatalf("err = %v, want ErrChanged", err)
	}
	if len(f.keys) != 0 {
		t.Errorf("pressed %q; a changed list must get no keys", f.keys)
	}
}

func TestOpen_LeavesQuestionsAlone(t *testing.T) {
	fastWaits(t)
	f := newFake(2)
	f.question = true
	if err := Open(f, 1, 2); err == nil {
		t.Fatal("opening a subagent while Claude Code asks a question must fail")
	}
	if len(f.keys) != 0 {
		t.Errorf("pressed %q into a question", f.keys)
	}
}

// Outside the agent list, Esc interrupts Claude Code.
func TestOpen_NeverPressesEscOutsideTheList(t *testing.T) {
	fastWaits(t)
	f := newFake(2)
	f.draft = maxDraftLines + 5
	if err := Open(f, 1, 2); err == nil {
		t.Fatal("want an error when the list does not take focus")
	}
	if slices.Contains(f.keys, "Escape") || f.cancelled {
		t.Errorf("list never focused: pressed Esc outside it: %q", f.keys)
	}

	f = newFake(2)
	f.finishOnEnter = true
	if err := Open(f, 1, 2); !errors.Is(err, ErrChanged) {
		t.Fatalf("err = %v, want ErrChanged when the subagents finish", err)
	}
	if slices.Contains(f.keys, "Escape") || f.cancelled {
		t.Errorf("list gone: pressed Esc outside it: %q", f.keys)
	}
}

func TestOpen_FailureGivesThePromptFocusBack(t *testing.T) {
	fastWaits(t)
	f := newFake(2)
	f.ignoreEnter = true
	if err := Open(f, 2, 2); err == nil {
		t.Fatal("want an error when the view does not open")
	}
	if f.focus != -1 || f.cancelled {
		t.Errorf("focus = %d, cancelled = %v; the list must not keep focus, where x stops a subagent", f.focus, f.cancelled)
	}
}
