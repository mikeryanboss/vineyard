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

// attach draws t on the user's terminal, given as in and out, and passes what
// the user types to its program, until the user types one of the detach
// bytes, the program exits, or stop is closed. env is the user's
// environment, which says what their terminal supports. The terminal takes
// the user's terminal's size, first initial and then each one resized
// receives.
func attach(t *terminal, in, out *os.File, env []string, detach []byte, initial size, resized <-chan size, stop <-chan struct{}) error {
	if !term.IsTerminal(in.Fd()) {
		return errors.New("attaching needs a terminal")
	}
	state, err := term.MakeRaw(in.Fd())
	if err != nil {
		return err
	}
	defer term.Restore(in.Fd(), state)
	if err := t.resize(initial.Width, initial.Height); err != nil {
		return err
	}

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
	s := newScreen(out, env)
	t.mu.Lock()
	err = s.open(t)
	t.mu.Unlock()
	defer s.close()
	if err != nil {
		return err
	}

	ticker := time.NewTicker(frame)
	defer ticker.Stop()
	dirty := false
	for {
		select {
		case b, ok := <-typed:
			if !ok {
				return nil
			}
			i := bytes.IndexAny(b, string(detach))
			if i < 0 {
				i = len(b)
			}
			if _, err := t.emu.InputPipe().Write(b[:i]); err != nil {
				return err
			}
			if i < len(b) {
				return nil
			}
		case sz := <-resized:
			if err := t.resize(sz.Width, sz.Height); err != nil {
				return err
			}
			dirty = true
		case <-changed:
			dirty = true
		case <-ticker.C:
			if !dirty {
				continue
			}
			dirty = false
			t.mu.Lock()
			err := s.draw(t)
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
func (s *screen) open(t *terminal) error {
	s.renderer.EnterAltScreen()
	return s.draw(t)
}

// draw sends the changes since the last frame, and the cursor, to the user's
// terminal. Hold t.mu.
func (s *screen) draw(t *terminal) error {
	if err := s.syncModes(t); err != nil {
		return err
	}
	width, height := t.emu.Width(), t.emu.Height()
	if s.buf == nil || s.buf.Width() != width || s.buf.Height() != height {
		s.buf = uv.NewRenderBuffer(width, height)
		s.renderer.Resize(width, height)
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
