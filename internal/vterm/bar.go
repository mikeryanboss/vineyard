package vterm

import (
	"hash/fnv"
	"strconv"
	"strings"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/mikeryanboss/vineyard/internal/session"
)

// sampleEvery is how often an attached terminal's status bar fingerprints the
// other sessions' screens, as often as Vineyard polls them.
const sampleEvery = time.Second

// bar is the status bar on the last row of an attached terminal. It lists
// the other sessions, each running, ready, or finished, and the keys that
// detach and hide it.
type bar struct {
	// shown is set while the user wants the bar, and visible while it is
	// drawn: when shown, unless it would leave the program no row.
	shown, visible bool
	peers          []peer
	// detach and toggle are the names of the detach and status bar keys.
	detach, toggle []string
}

// peer is a session the bar lists.
type peer struct {
	session.Peer
	// screen fingerprints its screen when last sampled; sampled is set once
	// it has been.
	screen  uint64
	sampled bool
	// ran is set once it has been seen running during the attachment, and
	// gone once its terminal has ended.
	ran, gone bool
}

func newBar(shown bool, peers []session.Peer, detach, toggle []string) *bar {
	b := &bar{shown: shown, detach: detach, toggle: toggle}
	for _, p := range peers {
		b.peers = append(b.peers, peer{Peer: p, ran: p.Running})
	}
	return b
}

// fit sets whether the bar is visible on a user's terminal height rows high,
// and returns how many rows that leaves the program.
func (b *bar) fit(height int) int {
	b.visible = b.shown && height > 1
	if b.visible {
		return height - 1
	}
	return height
}

// sample fingerprints each peer's screen on s. A screen changed since the
// last sample is running and one unchanged is ready, as Vineyard decides;
// until a peer has been sampled twice, it keeps the status Vineyard sent.
func (b *bar) sample(s *Server) {
	for i := range b.peers {
		p := &b.peers[i]
		t, err := s.terminal(p.Terminal)
		if err != nil {
			p.gone = true
			continue
		}
		h := fnv.New64a()
		t.mu.Lock()
		h.Write([]byte(t.emu.Render()))
		t.mu.Unlock()
		screen := h.Sum64()
		if p.sampled {
			p.Running = screen != p.screen
		}
		p.ran = p.ran || p.Running
		p.screen, p.sampled = screen, true
	}
}

// A peer's status in the bar.
type peerStatus int

const (
	peerRunning peerStatus = iota
	// peerFinished is ready after running during the attachment.
	peerFinished
	peerReady
)

func (p peer) status() peerStatus {
	switch {
	case p.Running:
		return peerRunning
	case p.ran:
		return peerFinished
	}
	return peerReady
}

// Styles of the bar: a dark bar, as tmux's status line is green, on dark and
// light terminals alike. The server does not know which the user's is, and
// themes change the 16 basic colours, so the bar takes its colours from the
// 256-colour palette, which themes leave alone.
var (
	barStyle  = ansi.NewStyle().BackgroundColor(ansi.IndexedColor(238)).ForegroundColor(ansi.IndexedColor(252))
	hintStyle = barStyle.ForegroundColor(ansi.IndexedColor(248))
	// iconStyles style each status's icon, and finished titles too.
	iconStyles = map[peerStatus]ansi.Style{
		peerRunning:  barStyle.ForegroundColor(ansi.IndexedColor(220)),
		peerFinished: barStyle.ForegroundColor(ansi.IndexedColor(114)).Bold(),
		peerReady:    barStyle,
	}
	icons = map[peerStatus]string{peerRunning: "◑", peerFinished: "✓", peerReady: "●"}
)

// text returns the bar as styled text width cells wide: each peer's status
// icon and title, or when those do not fit, how many peers have each status,
// then the keys on the right.
func (b *bar) text(width int) string {
	var items []string
	counts := map[peerStatus]int{}
	for _, p := range b.peers {
		if p.gone {
			continue
		}
		st := p.status()
		title := barStyle.Styled(p.Title)
		if st == peerFinished {
			title = iconStyles[st].Styled(p.Title)
		}
		items = append(items, iconStyles[st].Styled(icons[st])+barStyle.Styled(" ")+title)
		counts[st]++
	}
	sep := barStyle.Styled("  ")
	left := barStyle.Styled(" ") + strings.Join(items, sep)
	var hints []string
	if len(b.detach) > 0 {
		hints = append(hints, strings.Join(b.detach, "/")+" back")
	}
	if len(b.toggle) > 0 {
		hints = append(hints, strings.Join(b.toggle, "/")+" hide")
	}
	right := hintStyle.Styled(strings.Join(hints, "  ") + " ")
	if ansi.StringWidth(left)+ansi.StringWidth(right)+1 > width {
		var summary []string
		for _, st := range []peerStatus{peerRunning, peerFinished, peerReady} {
			if counts[st] > 0 {
				summary = append(summary, iconStyles[st].Styled(icons[st])+barStyle.Styled(" "+strconv.Itoa(counts[st])))
			}
		}
		left = barStyle.Styled(" ") + strings.Join(summary, sep)
	}
	gap := width - ansi.StringWidth(left) - ansi.StringWidth(right)
	if gap < 1 {
		right, gap = "", width-ansi.StringWidth(left)
	}
	line := left + barStyle.Styled(strings.Repeat(" ", max(0, gap))) + right
	return ansi.Truncate(line, width, "")
}

// draw draws the bar on row y of buf.
func (b *bar) draw(buf *uv.RenderBuffer, y int) {
	width := buf.Width()
	uv.NewStyledString(b.text(width)).Draw(canvas{buf}, uv.Rect(0, y, width, 1))
}

// canvas is a frame's buffer as a screen that styled text draws on.
type canvas struct{ *uv.RenderBuffer }

func (canvas) WidthMethod() uv.WidthMethod { return ansi.GraphemeWidth }
