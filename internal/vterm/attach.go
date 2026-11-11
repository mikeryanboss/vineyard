package vterm

import (
	"bytes"
	"errors"
	"io"
	"os"
	"slices"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/term"
	"github.com/muesli/cancelreader"
)

const (
	// frame is the shortest time between two redraws of an attached terminal.
	frame = time.Second / 60
	// readSize is how many input bytes each Read from the user's terminal asks for.
	readSize = 1024
	// scrollStep is how many lines one mouse wheel notch scrolls, matching the
	// preview pane.
	scrollStep = 3
	// maxMouseSeq bounds the length of an SGR mouse sequence (ESC [ < b ; x ; y M).
	maxMouseSeq = 32
)

// programModes are the non-mouse input modes mirrored from the program onto
// the user's terminal, so that keys, focus events, and pastes arrive as the
// program asked. Mouse modes are always kept on so the wheel scrolls
// scrollback even when the program did not ask for the mouse.
var programModes = []ansi.Mode{
	ansi.ModeCursorKeys,
	ansi.ModeFocusEvent,
	ansi.ModeBracketedPaste,
}

// mouseTrackingModes are the modes by which a program asks for mouse reports.
var mouseTrackingModes = []ansi.Mode{
	ansi.ModeMouseX10,
	ansi.ModeMouseNormal,
	ansi.ModeMouseHighlight,
	ansi.ModeMouseButtonEvent,
	ansi.ModeMouseAnyEvent,
}

// size is the size of the user's terminal, in cells.
type size struct{ Width, Height int }

// attach draws t on the user's terminal, given as in and out, with the
// status bar b under it while b is shown, and passes what the user types to
// its program, until the user types one of b's detach keys, the program
// exits, or stop is closed. b's toggle keys show and hide b, and the mouse
// wheel scrolls t's primary-screen scrollback. env is the user's environment,
// which says what their terminal supports. The terminal takes the user's
// terminal's size less b's rows, first initial and then each one resized
// receives. b lists terminals of s.
func attach(s *Server, t *terminal, b *bar, in, out *os.File, env []string, initial size, resized <-chan size, stop <-chan struct{}) error {
	if !term.IsTerminal(in.Fd()) {
		return errors.New("attaching needs a terminal")
	}
	state, err := term.MakeRaw(in.Fd())
	if err != nil {
		return err
	}
	defer term.Restore(in.Fd(), state)
	user := initial
	fit := func() error { return t.resize(user.Width, b.fit(user.Height)) }
	if err := fit(); err != nil {
		return err
	}
	detach, toggle := keyBytes(b.detach), keyBytes(b.toggle)

	input, err := cancelreader.NewReader(in)
	if err != nil {
		return err
	}
	defer input.Close()
	typed := make(chan []byte)
	go func() {
		defer close(typed)
		for {
			buf := make([]byte, readSize)
			n, err := input.Read(buf)
			if n > 0 {
				typed <- buf[:n]
			}
			if err != nil {
				return
			}
		}
	}()
	defer func() {
		input.Cancel()
		for range typed {
		}
	}()

	changed, unwatch := t.watch()
	defer unwatch()
	b.sample(s)
	scr := newScreen(out, env)
	t.mu.Lock()
	err = scr.open(t, b)
	t.mu.Unlock()
	defer scr.close()
	if err != nil {
		return err
	}

	ticker := time.NewTicker(frame)
	defer ticker.Stop()
	sampler := time.NewTicker(sampleEvery)
	defer sampler.Stop()
	dirty := false
	var pending []byte
	for {
		select {
		case typedBytes, ok := <-typed:
			if !ok {
				return nil
			}
			full := len(typedBytes) == readSize
			buf := append(pending, typedBytes...)
			pending = nil
			for len(buf) > 0 {
				if bytes.IndexByte(detach, buf[0]) >= 0 {
					return nil
				}
				if bytes.IndexByte(toggle, buf[0]) >= 0 {
					b.shown = !b.shown
					if err := fit(); err != nil {
						return err
					}
					dirty = true
					buf = buf[1:]
					continue
				}
				if buf[0] == 0x1b {
					ev, n, incomplete := parseMouse(buf, full)
					if incomplete {
						pending = slices.Clone(buf)
						break
					}
					if n > 0 {
						forward, scrolled := scr.handleMouse(t, ev)
						if scrolled {
							dirty = true
						}
						if forward != "" {
							if _, err := io.WriteString(t.emu.InputPipe(), forward); err != nil {
								return err
							}
						}
						buf = buf[n:]
						continue
					}
				}
				k := 1
				for k < len(buf) && buf[k] != 0x1b && bytes.IndexByte(detach, buf[k]) < 0 && bytes.IndexByte(toggle, buf[k]) < 0 {
					k++
				}
				if scr.resetScroll(t) {
					dirty = true
				}
				if _, err := t.emu.InputPipe().Write(buf[:k]); err != nil {
					return err
				}
				buf = buf[k:]
			}
		case user = <-resized:
			if err := fit(); err != nil {
				return err
			}
			dirty = true
		case <-changed:
			dirty = true
		case <-sampler.C:
			b.sample(s)
			dirty = dirty || b.visible
		case <-ticker.C:
			if !dirty {
				continue
			}
			dirty = false
			t.mu.Lock()
			err := scr.draw(t, b)
			t.mu.Unlock()
			if err != nil {
				return err
			}
		case <-t.exited:
			return nil
		case <-stop:
			return nil
		}
	}
}

// keyBytes returns the bytes the user's terminal sends for keys, each ctrl
// and a letter or ctrl+\.
func keyBytes(keys []string) []byte {
	var out []byte
	for _, k := range keys {
		out = append(out, k[len("ctrl+")]&0x1f)
	}
	return out
}

// hasMouse reports whether t's program has enabled any mouse tracking mode.
// Hold t.mu.
func (t *terminal) hasMouse() bool {
	for _, m := range mouseTrackingModes {
		if t.modes[m] {
			return true
		}
	}
	return false
}

// mouseEvent is one SGR or legacy X10 mouse report from the user's terminal,
// with 1-based cell coordinates x and y.
type mouseEvent struct {
	button  int
	x, y    int
	release bool
}

func (e mouseEvent) wheelVertical() bool { return e.button&0xc0 == 0x40 && e.button&3 < 2 }
func (e mouseEvent) wheelUp() bool       { return e.button&3 == 0 }

// parseMouse parses a mouse sequence at the start of buf. incomplete reports
// that buf ends mid-sequence and needs the next Read; bare ESC and ESC [ count
// as incomplete only when full says the Read buffer was filled.
func parseMouse(buf []byte, full bool) (ev mouseEvent, consumed int, incomplete bool) {
	if len(buf) == 0 || buf[0] != 0x1b {
		return mouseEvent{}, 0, false
	}
	if len(buf) == 1 {
		return mouseEvent{}, 0, full
	}
	if buf[1] != '[' {
		return mouseEvent{}, 0, false
	}
	if len(buf) == 2 {
		return mouseEvent{}, 0, full
	}
	switch buf[2] {
	case '<':
		j := 3
		for j < len(buf) && j < maxMouseSeq && ((buf[j] >= '0' && buf[j] <= '9') || buf[j] == ';') {
			j++
		}
		if j == len(buf) && j < maxMouseSeq {
			return mouseEvent{}, 0, true
		}
		if j >= len(buf) || (buf[j] != 'M' && buf[j] != 'm') {
			return mouseEvent{}, 0, false
		}
		b, x, y, ok := parseSGRParams(buf[3:j])
		if !ok || x < 1 || y < 1 {
			return mouseEvent{}, 0, false
		}
		return mouseEvent{button: b, x: x, y: y, release: buf[j] == 'm'}, j + 1, false
	case 'M':
		if len(buf) < 6 {
			return mouseEvent{}, 0, true
		}
		if buf[3] < 32 || buf[4] < 32 || buf[5] < 32 {
			return mouseEvent{}, 0, false
		}
		b := int(buf[3]) - 32
		release := b&0xc0 == 0 && b&3 == 3
		return mouseEvent{button: b, x: int(buf[4]) - 32, y: int(buf[5]) - 32, release: release}, 6, false
	}
	return mouseEvent{}, 0, false
}

func parseSGRParams(p []byte) (b, x, y int, ok bool) {
	var vals [3]int
	part, digits := 0, 0
	for _, c := range p {
		if c == ';' {
			if digits == 0 || part == 2 {
				return 0, 0, 0, false
			}
			part++
			digits = 0
			continue
		}
		digits++
		if digits > 6 {
			return 0, 0, 0, false
		}
		vals[part] = vals[part]*10 + int(c-'0')
	}
	if part != 2 || digits == 0 {
		return 0, 0, 0, false
	}
	return vals[0], vals[1], vals[2], true
}

// screen draws a terminal's emulator on the user's terminal, sending only
// what changed since the last frame.
type screen struct {
	out      io.Writer
	renderer *uv.TerminalRenderer
	buf      *uv.RenderBuffer
	hidden   bool
	// modes are the input modes set on the user's terminal.
	modes []ansi.Mode
	// scroll is how many lines above the live screen are shown, and history
	// was the emulator's scrollback length when scroll was last updated.
	scroll, history int
}

func newScreen(out io.Writer, env []string) *screen {
	return &screen{out: out, renderer: uv.NewTerminalRenderer(out, env)}
}

// open takes over the user's terminal with the alternate screen and draws
// the first frame. Hold t.mu.
func (s *screen) open(t *terminal, b *bar) error {
	s.renderer.EnterAltScreen()
	return s.draw(t, b)
}

// syncScroll updates history from t's scrollback and keeps scroll anchored on
// the same lines as new lines enter scrollback. Hold t.mu.
func (s *screen) syncScroll(t *terminal) {
	if t.emu.IsAltScreen() {
		s.scroll, s.history = 0, 0
		return
	}
	n := t.emu.ScrollbackLen()
	if s.scroll > 0 {
		if n > s.history {
			s.scroll += n - s.history
		}
		s.scroll = min(s.scroll, n)
	}
	s.history = n
}

// resetScroll returns the view to the live screen and reports whether it was
// scrolled up.
func (s *screen) resetScroll(t *terminal) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	was := s.scroll > 0
	s.scroll = 0
	if !t.emu.IsAltScreen() {
		s.history = t.emu.ScrollbackLen()
	} else {
		s.history = 0
	}
	return was
}

// handleMouse scrolls primary-screen scrollback on wheel up/down and otherwise
// returns the sequence to forward to t's program, if any, and whether to
// redraw.
func (s *screen) handleMouse(t *terminal, ev mouseEvent) (string, bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	s.syncScroll(t)
	if ev.wheelVertical() {
		if !t.emu.IsAltScreen() {
			if ev.release {
				return "", false
			}
			before := s.scroll
			if ev.wheelUp() {
				s.scroll = min(s.history, s.scroll+scrollStep)
			} else {
				s.scroll = max(0, s.scroll-scrollStep)
			}
			return "", s.scroll != before
		}
	} else if s.scroll > 0 {
		return "", false
	}
	if !t.hasMouse() || ev.y > t.emu.Height() {
		return "", false
	}
	if t.modes[ansi.ModeMouseExtSgr] {
		return ansi.MouseSgr(byte(ev.button), ev.x-1, ev.y-1, ev.release), false
	}
	if (t.modes[ansi.ModeMouseX10] && ev.release) || ev.x > 223 || ev.y > 223 {
		return "", false
	}
	b := byte(ev.button)
	if ev.release {
		b = (b &^ 3) | 3
	}
	return ansi.MouseX10(b, ev.x-1, ev.y-1), false
}

// draw sends the changes since the last frame, and the cursor, to the user's
// terminal: t's screen or scrollback, and b under it while b is visible.
// Hold t.mu.
func (s *screen) draw(t *terminal, b *bar) error {
	if err := s.syncModes(t); err != nil {
		return err
	}
	s.syncScroll(t)
	width, height := t.emu.Width(), t.emu.Height()
	rows := height
	if b.visible {
		rows++
	}
	if s.buf == nil || s.buf.Width() != width || s.buf.Height() != rows {
		s.buf = uv.NewRenderBuffer(width, rows)
		s.renderer.Resize(width, rows)
		s.renderer.Erase()
	}
	for y := range height {
		line := s.history - s.scroll + y
		for x := 0; x < width; {
			var cell *uv.Cell
			if line < s.history {
				cell = t.emu.ScrollbackCellAt(x, line)
			} else {
				cell = t.emu.CellAt(x, line-s.history)
			}
			s.buf.SetCell(x, y, cell)
			x++
			if cell != nil && cell.Width > 1 {
				x += cell.Width - 1
			}
		}
	}
	if b.visible {
		b.draw(s.buf, height)
	}
	s.renderer.Render(s.buf)
	if s.scroll == 0 {
		cursor := t.emu.CursorPosition()
		s.renderer.MoveTo(cursor.X, cursor.Y)
	}
	if err := s.renderer.Flush(); err != nil {
		return err
	}
	if hidden := t.emu.CursorHidden() || s.scroll > 0; hidden != s.hidden {
		s.hidden = hidden
		seq := ansi.ShowCursor
		if hidden {
			seq = ansi.HideCursor
		}
		if _, err := io.WriteString(s.out, seq); err != nil {
			return err
		}
	}
	return nil
}

// desiredModes returns the input modes to keep set on the user's terminal:
// mouse tracking (upgraded when the program asks for motion) and SGR mouse
// coordinates always, plus any programModes the program enabled. Hold t.mu.
func desiredModes(t *terminal) []ansi.Mode {
	var modes []ansi.Mode
	for _, m := range programModes {
		if t.modes[m] {
			modes = append(modes, m)
		}
	}
	mouse := ansi.Mode(ansi.ModeMouseNormal)
	switch {
	case t.modes[ansi.ModeMouseAnyEvent]:
		mouse = ansi.ModeMouseAnyEvent
	case t.modes[ansi.ModeMouseButtonEvent]:
		mouse = ansi.ModeMouseButtonEvent
	}
	return append(modes, mouse, ansi.ModeMouseExtSgr)
}

// syncModes sets and resets input modes on the user's terminal to match
// desiredModes. Resets precede sets so switching between mutually exclusive
// mouse tracking modes (such as ?1002l then ?1000h) leaves tracking enabled.
// Hold t.mu.
func (s *screen) syncModes(t *terminal) error {
	want := desiredModes(t)
	var set, reset []ansi.Mode
	for _, m := range s.modes {
		if !slices.Contains(want, m) {
			reset = append(reset, m)
		}
	}
	for _, m := range want {
		if !slices.Contains(s.modes, m) {
			set = append(set, m)
		}
	}
	s.modes = want
	var seq string
	if len(reset) > 0 {
		seq += ansi.ResetMode(reset...)
	}
	if len(set) > 0 {
		seq += ansi.SetMode(set...)
	}
	if seq == "" {
		return nil
	}
	_, err := io.WriteString(s.out, seq)
	return err
}

// close resets the input modes it set and leaves the alternate screen.
func (s *screen) close() error {
	seq := ansi.ResetStyle + ansi.ShowCursor
	if len(s.modes) > 0 {
		seq += ansi.ResetMode(s.modes...)
	}
	if _, err := io.WriteString(s.out, seq); err != nil {
		return err
	}
	s.renderer.ExitAltScreen()
	return s.renderer.Flush()
}
