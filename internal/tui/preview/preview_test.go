package preview_test

import (
	"fmt"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
	"github.com/mikeryanboss/vineyard/internal/tui/preview"
	"github.com/mikeryanboss/vineyard/internal/tui/testutil"
)

func newPreview() preview.Model {
	return preview.New(common.NewTheme(true)).SetSize(30, 6)
}

func msgOf(cmd tea.Cmd) tea.Msg {
	if cmd == nil {
		return nil
	}
	return cmd()
}

func TestPreviewView_LiveScreenFitsPane(t *testing.T) {
	screen := "\x1b[32m✻ Working…\x1b[0m\n> a line much longer than the thirty cell pane\n\n\n\n\nbeyond the pane height\n"
	testutil.RequireGolden(t, newPreview().SetScreen(screen).View())
}

func TestPreviewView_Placeholder(t *testing.T) {
	testutil.RequireGolden(t, newPreview().SetPlaceholder("Paused").View())
}

// A placeholder longer than the pane is wide wraps inside it: the terminal
// would otherwise wrap it, pushing the rest of the screen down.
func TestPreviewView_LongPlaceholderFitsPane(t *testing.T) {
	view := newPreview().SetPlaceholder("this agent runs on an older Vineyard multiplexer, which this Vineyard cannot reach").View()
	lines := strings.Split(view, "\n")
	if len(lines) != 6 {
		t.Errorf("%d lines, want 6:\n%s", len(lines), view)
	}
	for _, line := range lines {
		if w := ansi.StringWidth(line); w != 30 {
			t.Errorf("line %q is %d cells wide, want 30", ansi.Strip(line), w)
		}
	}
}

func TestPreview_ColourDoesNotBleed(t *testing.T) {
	view := newPreview().SetScreen("\x1b[41mred background never closed").View()
	for i, line := range strings.Split(view, "\n") {
		if strings.Contains(line, "\x1b[") && !strings.HasSuffix(line, "\x1b[0m") {
			t.Errorf("line %d does not end with a reset: %q", i, line)
		}
	}
}

func TestPreview_ScrollUpRequestsScrollback(t *testing.T) {
	m := newPreview().SetScreen("live")
	_, cmd := m.Update(testutil.Key("k"))
	if _, ok := msgOf(cmd).(common.ScrollbackRequestMsg); !ok {
		t.Fatal("scrolling up on the live screen should request scrollback")
	}
}

func TestPreview_ScrollbackReturnsToLiveAtBottom(t *testing.T) {
	var history []string
	for i := range 20 {
		history = append(history, fmt.Sprintf("line %d", i))
	}
	m := newPreview().SetScreen("live").SetScrollback(strings.Join(history, "\n"))
	if !m.Scrolling() {
		t.Fatal("SetScrollback should enter scroll mode")
	}
	testutil.RequireGolden(t, m.View())

	m, _ = m.Update(testutil.Key("j"))
	if m.Scrolling() {
		t.Error("scrolling back to the bottom should return to the live screen")
	}
}

// A program on the alternate screen, such as Claude Code drawing fullscreen,
// has no scrollback: tmux returns only the screen.
func TestPreview_ScrollbackOfOnlyTheScreenStaysLive(t *testing.T) {
	screen := "one\ntwo\nthree\n"
	if newPreview().SetScreen(screen).SetScrollback(screen).Scrolling() {
		t.Error("history no longer than the screen must leave the preview live")
	}
}

func TestPreview_BackLeavesPane(t *testing.T) {
	m := newPreview().SetScrollback("a\nb\nc\nd\ne\nf\ng\nh")
	m, cmd := m.Update(testutil.Key("esc"))
	if _, ok := msgOf(cmd).(common.LeavePaneMsg); !ok {
		t.Error("esc should leave the pane")
	}
	if m.Scrolling() {
		t.Error("leaving the pane should exit scroll mode")
	}
}
