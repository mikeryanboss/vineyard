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

// frame is the shortest time between two redraws of an attached terminal.
const frame = time.Second / 60

// inputModes are the modes that change what the user's terminal sends. An
// attached terminal sets those its program set on the user's terminal, so
// that keys, mouse, and pastes arrive as the program asked.
var inputModes = []ansi.Mode{
	ansi.ModeCursorKeys,
	ansi.ModeMouseNormal,
	ansi.ModeMouseButtonEvent,
	ansi.ModeMouseAnyEvent,
	ansi.ModeFocusEvent,
	ansi.ModeMouseExtSgr,
	ansi.ModeBracketedPaste,
}

// size is the size of the user's terminal, in cells.
type size struct{ Width, Height int }

// attach draws t on the user's terminal, given as in and out, with the
// status bar b under it while b is shown, and passes what the user types to
// its program, until the user types one of b's detach keys, the program
// exits, or stop is closed. b's toggle keys show and hide b. env is the
// user's environment, which says what their terminal supports. The terminal
// takes the user's terminal's size less b's rows, first initial and then each
// one resized receives. b lists terminals of s.
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
			buf := make([]byte, 1024)
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
	for {
		select {
		case typedBytes, ok := <-typed:
			if !ok {
				return nil
			}
			for len(typedBytes) > 0 {
				i := bytes.IndexAny(typedBytes, string(detach)+string(toggle))
				if i < 0 {
					i = len(typedBytes)
				}
				if _, err := t.emu.InputPipe().Write(typedBytes[:i]); err != nil {
					return err
				}
				if i == len(typedBytes) {
					break
				}
				if bytes.IndexByte(detach, typedBytes[i]) >= 0 {
					return nil
				}
				b.shown = !b.shown
				if err := fit(); err != nil {
					return err
				}
				dirty = true
				typedBytes = typedBytes[i+1:]
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

// screen draws a terminal's emulator on the user's terminal, sending only
// what changed since the last frame.
type screen struct {
	out      io.Writer
	renderer *uv.TerminalRenderer
	buf      *uv.RenderBuffer
	hidden   bool
	// modes are the input modes set on the user's terminal.
	modes []ansi.Mode
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

// draw sends the changes since the last frame, and the cursor, to the user's
// terminal: t's screen, and b under it while b is visible. Hold t.mu.
func (s *screen) draw(t *terminal, b *bar) error {
	if err := s.syncModes(t); err != nil {
		return err
	}
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
		for x := 0; x < width; {
			cell := t.emu.CellAt(x, y)
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
	cursor := t.emu.CursorPosition()
	s.renderer.MoveTo(cursor.X, cursor.Y)
	if err := s.renderer.Flush(); err != nil {
		return err
	}
	if hidden := t.emu.CursorHidden(); hidden != s.hidden {
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

// syncModes sets on the user's terminal the input modes the program set or
// reset since the last frame.
func (s *screen) syncModes(t *terminal) error {
	var set, reset, now []ansi.Mode
	for _, m := range inputModes {
		was := slices.Contains(s.modes, m)
		switch {
		case t.modes[m] && !was:
			set = append(set, m)
		case !t.modes[m] && was:
			reset = append(reset, m)
		}
		if t.modes[m] {
			now = append(now, m)
		}
	}
	s.modes = now
	var seq string
	if len(set) > 0 {
		seq += ansi.SetMode(set...)
	}
	if len(reset) > 0 {
		seq += ansi.ResetMode(reset...)
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
