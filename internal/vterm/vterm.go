// Package vterm runs agent programs on pseudo-terminals inside Vineyard's own
// process and mirrors their screens with a terminal emulator. Vineyard uses it
// when tmux is not installed. Its terminals end when Vineyard exits, and no
// other process can see them.
package vterm

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	uv "github.com/charmbracelet/ultraviolet"
	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// ErrNoTerminal reports that a terminal does not exist.
var ErrNoTerminal = errors.New("terminal not found")

// Default size of a terminal started without one.
const defaultWidth, defaultHeight = 80, 24

// historyLimit is how many lines of scrollback each terminal keeps, as many
// as Vineyard's tmux server keeps.
const historyLimit = 10000

// Server holds the terminals of this process by name.
type Server struct {
	mu        sync.Mutex
	terminals map[string]*terminal
}

// New returns a server without terminals.
func New() *Server {
	return &Server{terminals: map[string]*terminal{}}
}

// terminal is one program on a pseudo-terminal, mirrored by an emulator.
type terminal struct {
	pty     *os.File
	process *os.Process
	created time.Time
	// exited is closed once the program has exited.
	exited chan struct{}
	// changed receives a value when the screen may have changed.
	changed chan struct{}

	// mu guards emu and modes. The emulator writes its replies to queries,
	// and input sent through it, to a pipe that forward copies to the
	// program; that copy never takes mu, so holding mu while the pipe blocks
	// is safe.
	mu  sync.Mutex
	emu *vt.Emulator
	// modes holds the modes the program has set, as reported by the emulator.
	modes map[ansi.Mode]bool
}

// Start runs program in dir on a new terminal of the given size. An empty
// program starts the user's shell.
func (s *Server) Start(name, dir, program string, width, height int) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.terminals[name]; ok {
		return fmt.Errorf("terminal %s already exists", name)
	}
	if width <= 0 || height <= 0 {
		width, height = defaultWidth, defaultHeight
	}
	shell := os.Getenv("SHELL")
	if shell == "" {
		shell = "/bin/sh"
	}
	cmd := exec.Command(shell)
	if program != "" {
		cmd = exec.Command(shell, "-c", program)
	}
	cmd.Dir = dir
	cmd.Env = append(withoutEnv(os.Environ(), "TMUX", "TERM"), "TERM=xterm-256color")
	f, err := pty.StartWithSize(cmd, &pty.Winsize{Cols: uint16(width), Rows: uint16(height)})
	if err != nil {
		return err
	}
	t := &terminal{
		pty:     f,
		process: cmd.Process,
		created: time.Now(),
		exited:  make(chan struct{}),
		changed: make(chan struct{}, 1),
		emu:     vt.NewEmulator(width, height),
		modes:   map[ansi.Mode]bool{},
	}
	t.emu.SetScrollbackSize(historyLimit)
	t.emu.SetCallbacks(vt.Callbacks{
		EnableMode:  func(m ansi.Mode) { t.modes[m] = true },
		DisableMode: func(m ansi.Mode) { delete(t.modes, m) },
	})
	s.terminals[name] = t
	go t.forward()
	go func() {
		t.mirror()
		cmd.Wait()
		f.Close()
		// Ends forward. Emulator.Close would race with its blocked Read.
		t.emu.InputPipe().(io.Closer).Close()
		s.mu.Lock()
		if s.terminals[name] == t {
			delete(s.terminals, name)
		}
		s.mu.Unlock()
		close(t.exited)
	}()
	return nil
}

// mirror feeds the program's output to the emulator until the program exits.
func (t *terminal) mirror() {
	buf := make([]byte, 32*1024)
	for {
		n, err := t.pty.Read(buf)
		if n > 0 {
			t.mu.Lock()
			t.emu.Write(buf[:n])
			t.mu.Unlock()
			select {
			case t.changed <- struct{}{}:
			default:
			}
		}
		if err != nil {
			return
		}
	}
}

// forward copies what the emulator sends, its replies and the input given
// to it, to the program.
func (t *terminal) forward() {
	io.Copy(t.pty, t.emu)
}

func (s *Server) terminal(name string) (*terminal, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.terminals[name]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrNoTerminal, name)
	}
	return t, nil
}

// Exists reports whether the named terminal's program is running.
func (s *Server) Exists(name string) bool {
	_, err := s.terminal(name)
	return err == nil
}

// Created returns when the named terminal started its program.
func (s *Server) Created(name string) (time.Time, error) {
	t, err := s.terminal(name)
	if err != nil {
		return time.Time{}, err
	}
	return t.created, nil
}

// Capture returns the visible screen, with colours as ANSI escape sequences.
func (s *Server) Capture(name string) (string, error) {
	t, err := s.terminal(name)
	if err != nil {
		return "", err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.emu.Render(), nil
}

// CaptureHistory returns up to lines of scrollback above the visible screen,
// followed by the screen itself.
func (s *Server) CaptureHistory(name string, lines int) (string, error) {
	t, err := s.terminal(name)
	if err != nil {
		return "", err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	var b strings.Builder
	if history := t.emu.Scrollback(); history != nil {
		all := history.Lines()
		for _, line := range all[max(0, len(all)-lines):] {
			b.WriteString(line.Render())
			b.WriteByte('\n')
		}
	}
	b.WriteString(t.emu.Render())
	return b.String(), nil
}

// Resize sets the terminal's size.
func (s *Server) Resize(name string, width, height int) error {
	if width <= 0 || height <= 0 {
		return nil
	}
	t, err := s.terminal(name)
	if err != nil {
		return err
	}
	return t.resize(width, height)
}

func (t *terminal) resize(width, height int) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.emu.Width() == width && t.emu.Height() == height {
		return nil
	}
	t.emu.Resize(width, height)
	return pty.Setsize(t.pty, &pty.Winsize{Cols: uint16(width), Rows: uint16(height)})
}

// keys are the keys SendKeys presses, named as tmux names them.
var keys = map[string]uv.Key{
	"Enter":  {Code: uv.KeyEnter},
	"Escape": {Code: uv.KeyEscape},
	"Up":     {Code: uv.KeyUp},
	"Down":   {Code: uv.KeyDown},
}

// SendEnter presses Enter in the terminal.
func (s *Server) SendEnter(name string) error { return s.SendKeys(name, "Enter") }

// SendKeys presses keys in the terminal, named as tmux names them. It knows
// only the keys Vineyard presses.
func (s *Server) SendKeys(name string, names ...string) error {
	t, err := s.terminal(name)
	if err != nil {
		return err
	}
	presses := make([]uv.KeyEvent, len(names))
	for i, n := range names {
		k, ok := keys[n]
		if !ok {
			return fmt.Errorf("unknown key %q", n)
		}
		presses[i] = uv.KeyPressEvent(k)
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.emu.SendKeys(presses...)
	return nil
}

// Paste types text into the terminal, as a bracketed paste if the program
// asked for those.
func (s *Server) Paste(name, text string) error {
	t, err := s.terminal(name)
	if err != nil {
		return err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.emu.Paste(text)
	return nil
}

// Kill hangs up the terminal, ending its program and the program's children.
// A missing terminal is not an error.
func (s *Server) Kill(name string) error {
	s.mu.Lock()
	t, ok := s.terminals[name]
	delete(s.terminals, name)
	s.mu.Unlock()
	if !ok {
		return nil
	}
	return t.hangUp()
}

// hangUp sends SIGHUP to the program's process group, which its pseudo-
// terminal session leads.
func (t *terminal) hangUp() error {
	err := syscall.Kill(-t.process.Pid, syscall.SIGHUP)
	if errors.Is(err, syscall.ESRCH) {
		return nil
	}
	return err
}

// Close hangs up every terminal. Vineyard calls it on exit.
func (s *Server) Close() error {
	s.mu.Lock()
	terminals := s.terminals
	s.terminals = map[string]*terminal{}
	s.mu.Unlock()
	var errs []error
	for _, t := range terminals {
		errs = append(errs, t.hangUp())
	}
	return errors.Join(errs...)
}

func withoutEnv(env []string, keys ...string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if name, _, _ := strings.Cut(kv, "="); !slices.Contains(keys, name) {
			out = append(out, kv)
		}
	}
	return out
}
