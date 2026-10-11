// Package subagent reads Claude Code's list of running subagents off its
// screen, and opens a subagent's view by pressing Claude Code's own keys.
//
// Claude Code runs subagents inside its own process, so they have no terminal
// of their own. It lists the running ones below its prompt, here with the
// list focused and main selected:
//
//	❯ ● main
//	  ◯ general-purpose  Worker one sleeps               5s · ↓ 19.4k tokens
//	  ◯ general-purpose  Worker two sleeps               5s · ↓ 19.4k tokens
//	  ↓ 1 more
//
// ● marks the conversation on screen. ❯ marks the selected row while the
// list has focus, and is two spaces otherwise. Subagents a subagent started
// follow it as a tree (├ ◯ general-purpose ...) while its view is shown, and
// are otherwise only counted after its type (general-purpose (+2)). None of
// this is documented; it was read off Claude Code 2.1.296.
package subagent

import (
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
	"github.com/mikeryanboss/vineyard/internal/session"
)

// Row is one subagent in Claude Code's list.
type Row struct {
	// Type is the agent type, such as general-purpose.
	Type string
	// Label is Claude Code's live summary of what the subagent is doing.
	Label string
	// Depth is 0 for a subagent of the main conversation, 1 for one that
	// subagent started, and so on.
	Depth int
	// Hidden counts the subagents it started that Claude Code does not draw.
	Hidden int
}

// List is Claude Code's agent list as one screen draws it. Positions count
// rows of the whole list: 0 is the main conversation, 1 the first subagent,
// whether drawn or scrolled out of sight.
type List struct {
	// Rows are the subagents drawn, top to bottom.
	Rows []Row
	// Above and Below count the subagents scrolled out of sight.
	Above, Below int
	// Shown is the position of the conversation on screen, or -1 when it is
	// a subagent scrolled out of sight.
	Shown int
	// Focus is the position of the selected row while the list has focus,
	// or -1.
	Focus int
	// View is the title on the rule above the prompt: the description of
	// the subagent shown, or the conversation's name. It may be cut short
	// with "…".
	View string
}

// Len returns the number of subagents.
func (l List) Len() int { return l.Above + len(l.Rows) + l.Below }

// Position returns the position of drawn row i.
func (l List) Position(i int) int { return l.Above + i + 1 }

var (
	mainRow  = regexp.MustCompile(`^(❯| ) ([●◯]) main(?:\s+↑ (\d+) more)?\s*$`)
	agentRow = regexp.MustCompile(`^(❯| ) ((?:[│├└] |  )*)([●◯]) (\S+)(?: \(\+(\d+)\))?  +(.*)$`)
	moreRow  = regexp.MustCompile(`^  ↓ (\d+) more\s*$`)
	titled   = regexp.MustCompile(`^─+ (.+) ─\s*$`)
	// columns separates a row's label from its statistics.
	columns = regexp.MustCompile(`\s{2,}`)
)

// Parse finds Claude Code's agent list on a captured screen. It reports false
// when the screen has none, as when no subagent runs.
func Parse(screen string) (List, bool) {
	lines := strings.Split(ansi.Strip(screen), "\n")
	start := -1
	for i, line := range lines {
		if mainRow.MatchString(line) {
			start = i // the list is the last thing on screen
		}
	}
	if start < 0 {
		return List{}, false
	}
	l := List{Shown: -1, Focus: -1}
	for _, line := range lines[:start] {
		if m := titled.FindStringSubmatch(line); m != nil {
			l.View = m[1]
		}
	}
	m := mainRow.FindStringSubmatch(lines[start])
	l.mark(0, m[1], m[2])
	if m[3] != "" {
		l.Above, _ = strconv.Atoi(m[3])
	}
	for _, line := range lines[start+1:] {
		if m := moreRow.FindStringSubmatch(line); m != nil {
			l.Below, _ = strconv.Atoi(m[1])
			break
		}
		m := agentRow.FindStringSubmatch(line)
		if m == nil {
			break
		}
		l.mark(l.Position(len(l.Rows)), m[1], m[3])
		row := Row{
			Type:  m[4],
			Label: columns.Split(strings.TrimSpace(m[6]), 2)[0],
			Depth: utf8.RuneCountInString(m[2]) / 2,
		}
		if m[5] != "" {
			row.Hidden, _ = strconv.Atoi(m[5])
		}
		l.Rows = append(l.Rows, row)
	}
	return l, len(l.Rows) > 0
}

// mark records the markers of the row at position.
func (l *List) mark(position int, cursor, dot string) {
	if cursor == "❯" {
		l.Focus = position
	}
	if dot == "●" {
		l.Shown = position
	}
}

// Screen is the terminal a Claude Code session runs in.
type Screen interface {
	// Capture returns the visible screen.
	Capture() (string, error)
	// SendKeys presses keys named as tmux names them, such as Down.
	SendKeys(keys ...string) error
}

// ErrChanged reports that Claude Code's list no longer holds the subagents
// the caller saw.
var ErrChanged = errors.New("the subagents changed; try again")

var (
	// keyWait is how long a key press may take to show on screen. A press
	// that moves the cursor down a multi-line draft never shows, so giving
	// the list focus waits this long for each line below the cursor.
	keyWait = 500 * time.Millisecond
	// viewWait is how long opening a subagent's view may take.
	viewWait = 2 * time.Second
	// pollEvery paces captures while waiting for the screen.
	pollEvery = 20 * time.Millisecond
)

// maxDraftLines bounds the presses that give the list focus, each of which
// may only move the cursor down a draft.
const maxDraftLines = 20

// Open opens the view of the subagent at position in Claude Code's list,
// which must still hold total subagents, and gives focus back to Claude
// Code's prompt, so that typing messages the subagent. It checks the screen
// after every step and stops at the first surprise. Esc is pressed only while
// the screen shows the list focused: anywhere else it interrupts Claude Code.
func Open(s Screen, position, total int) error {
	screen, err := s.Capture()
	if err != nil {
		return err
	}
	if session.AwaitingPermission("claude", screen) || session.AwaitingTrust(screen) {
		return errors.New("Claude Code is asking a question; attach and answer it first")
	}
	l, ok := Parse(screen)
	if !ok || l.Len() != total {
		return ErrChanged
	}
	err = choose(s, l, position, total)
	if released := release(s); err == nil {
		err = released
	}
	return err
}

// choose gives the list focus, selects position, and opens it.
func choose(s Screen, l List, position, total int) error {
	for presses := 0; l.Focus < 0; presses++ {
		if presses == maxDraftLines {
			return errors.New("Claude Code's agent list did not take focus")
		}
		if err := s.SendKeys("Down"); err != nil {
			return err
		}
		var err error
		if l, err = waitFor(s, keyWait, func(l List) bool { return l.Focus >= 0 }); err != nil && !errors.Is(err, errTimeout) {
			return err
		}
	}
	if l.Len() != total {
		return ErrChanged
	}
	if moves := position - l.Focus; moves != 0 {
		key := "Down"
		if moves < 0 {
			key, moves = "Up", -moves
		}
		if err := s.SendKeys(slices.Repeat([]string{key}, moves)...); err != nil {
			return err
		}
		if _, err := waitFor(s, keyWait, func(l List) bool { return l.Focus == position }); err != nil {
			return fmt.Errorf("selecting the subagent: %w", err)
		}
	}
	if err := s.SendKeys("Enter"); err != nil {
		return err
	}
	// Claude Code drops the subagent it showed when that one has finished,
	// which moves those after it up a place.
	moved := position
	if l.Shown > 0 && l.Shown < position {
		moved--
	}
	opened := func(after List) bool {
		return after.Shown == position || after.Len() == total-1 && l.Shown > 0 && after.Shown == moved
	}
	if _, err := waitFor(s, viewWait, opened); err != nil {
		return fmt.Errorf("opening the subagent: %w", err)
	}
	return nil
}

// release gives focus back to the prompt if the list has it. A list that is
// gone has given it back already.
func release(s Screen) error {
	screen, err := s.Capture()
	if err != nil {
		return err
	}
	if l, ok := Parse(screen); !ok || l.Focus < 0 {
		return nil
	}
	if err := s.SendKeys("Escape"); err != nil {
		return err
	}
	_, err = waitFor(s, keyWait, func(l List) bool { return l.Focus < 0 })
	if err != nil && !errors.Is(err, ErrChanged) {
		return fmt.Errorf("leaving the agent list: %w", err)
	}
	return nil
}

var errTimeout = errors.New("Claude Code's screen did not change as expected")

// waitFor captures the screen until its list satisfies ok, and returns that
// list. A screen without a list never does: the subagents are gone.
func waitFor(s Screen, timeout time.Duration, ok func(List) bool) (List, error) {
	deadline := time.Now().Add(timeout)
	for {
		screen, err := s.Capture()
		if err != nil {
			return List{}, err
		}
		l, found := Parse(screen)
		if !found {
			return List{}, ErrChanged
		}
		if ok(l) {
			return l, nil
		}
		if time.Now().After(deadline) {
			return l, errTimeout
		}
		time.Sleep(pollEvery)
	}
}
