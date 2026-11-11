package vterm

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/charmbracelet/x/vt"
	"github.com/creack/pty"
)

// start runs program on a new terminal of s and ends it with the test.
func start(t *testing.T, s *Server, name, program string) {
	t.Helper()
	env := append(os.Environ(), "SHELL=/bin/sh")
	if err := s.Start(name, t.TempDir(), program, env, 40, 10); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Kill(name) })
}

// waitFor polls capture until it contains want.
func waitFor(t *testing.T, capture func() (string, error), want string) string {
	t.Helper()
	var got string
	for deadline := time.Now().Add(2 * time.Second); time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		var err error
		if got, err = capture(); err != nil {
			t.Fatal(err)
		}
		if strings.Contains(got, want) {
			return got
		}
	}
	t.Fatalf("screen never showed %q; it shows:\n%s", want, got)
	return ""
}

func TestCaptureKeepsColours(t *testing.T) {
	s := New()
	start(t, s, "a", `printf '\033[31mred\033[0m plain\n'; sleep 5`)
	got := waitFor(t, func() (string, error) { return s.Capture("a") }, "plain")
	if !strings.Contains(got, "\x1b[31mred") {
		t.Errorf("capture lost the colour of red: %q", got)
	}
}

func TestCaptureHistoryReturnsScrollbackThenScreen(t *testing.T) {
	s := New()
	start(t, s, "a", `seq 1 30; sleep 5`)
	waitFor(t, func() (string, error) { return s.Capture("a") }, "30")
	got, err := s.CaptureHistory("a", 100)
	if err != nil {
		t.Fatal(err)
	}
	lines := strings.Split(got, "\n")
	if lines[0] != "1" || !strings.Contains(got, "30") {
		t.Errorf("history = %q, want 1 to 30", got)
	}
	got, err = s.CaptureHistory("a", 2)
	if err != nil {
		t.Fatal(err)
	}
	if lines := strings.Split(got, "\n"); lines[0] != "20" {
		t.Errorf("two lines of history start at %q, want 20", lines[0])
	}

	// Programs on the alternate screen have no scrollback of their own, even
	// after primary-screen output filled history.
	start(t, s, "alt", `seq 1 30; printf '\033[?1049h\033[Halt-only'; sleep 5`)
	waitFor(t, func() (string, error) { return s.Capture("alt") }, "alt-only")
	got, err = s.CaptureHistory("alt", 100)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "15") || !strings.HasPrefix(got, "alt-only") {
		t.Errorf("alternate-screen history included primary scrollback: %q", got)
	}
}

// The agent receives pastes bracketed when it asked for bracketed pastes, as
// Claude Code does, and keys as their escape sequences.
func TestPasteAndKeysReachTheProgram(t *testing.T) {
	s := New()
	start(t, s, "a", `printf '\033[?2004hready\n'; stty -echo; cat -v`)
	waitFor(t, func() (string, error) { return s.Capture("a") }, "ready")
	if err := s.Paste("a", "hello"); err != nil {
		t.Fatal(err)
	}
	if err := s.SendKeys("a", "Up", "Enter"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() (string, error) { return s.Capture("a") }, "^[[200~hello^[[201~^[[A")
	if err := s.SendKeys("a", "F13"); err == nil {
		t.Error("an unknown key was accepted")
	}
}

func TestTerminalEndsWithItsProgram(t *testing.T) {
	s := New()
	start(t, s, "a", `exit 0`)
	for deadline := time.Now().Add(2 * time.Second); s.Exists("a"); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("terminal still exists after its program exited")
		}
	}
	if _, err := s.Capture("a"); !errors.Is(err, ErrNoTerminal) {
		t.Errorf("capture of an ended terminal: err = %v, want ErrNoTerminal", err)
	}
}

// Killing a terminal ends its program's children too, as tmux's kill-session
// does, so no agent outlives its session.
func TestKillEndsTheProgramAndItsChildren(t *testing.T) {
	s := New()
	start(t, s, "a", `sleep 30 & echo "child $!"; wait`)
	got := waitFor(t, func() (string, error) { return s.Capture("a") }, "child ")
	var child int
	if _, err := fmt.Sscanf(got, "child %d", &child); err != nil {
		t.Fatal(err)
	}
	if err := s.Kill("a"); err != nil {
		t.Fatal(err)
	}
	if s.Exists("a") {
		t.Error("terminal exists after Kill")
	}
	for deadline := time.Now().Add(2 * time.Second); syscall.Kill(child, 0) == nil; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("child %d still runs after Kill", child)
		}
	}
	if err := s.Kill("a"); err != nil {
		t.Errorf("killing a missing terminal: %v", err)
	}
}

// Claude Code puts ✳ (E2 9C B3) before its title. Its 0x9C byte must not end
// the title's OSC, which printed the rest of the title in the prompt box.
func TestTitleStaysOffTheScreen(t *testing.T) {
	s := New()
	start(t, s, "a", `printf '\033]0;\342\234\263 Greeting reply\007ready\n'; sleep 5`)
	got := waitFor(t, func() (string, error) { return s.Capture("a") }, "ready")
	if strings.Contains(got, "Greeting") {
		t.Errorf("the title is on the screen: %q", got)
	}
}

// outerTerm runs attach on a pseudo-terminal pair mirrored into an emulator of
// the user's outer terminal, so tests can inspect the exact screen and modes
// the user sees.
type outerTerm struct {
	user  *os.File
	done  chan error
	mu    sync.Mutex
	emu   *vt.Emulator
	modes map[ansi.Mode]bool
}

func attachTestTerm(t *testing.T, s *Server, name string, showBar bool) *outerTerm {
	t.Helper()
	term, err := s.terminal(name)
	if err != nil {
		t.Fatal(err)
	}
	user, tty, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	const cols, rows = 40, 10
	if err := pty.Setsize(tty, &pty.Winsize{Cols: cols, Rows: rows}); err != nil {
		t.Fatal(err)
	}
	o := &outerTerm{
		user:  user,
		done:  make(chan error, 1),
		emu:   vt.NewEmulator(cols, rows),
		modes: map[ansi.Mode]bool{},
	}
	o.emu.SetCallbacks(vt.Callbacks{
		EnableMode:  func(m ansi.Mode) { o.modes[m] = true },
		DisableMode: func(m ansi.Mode) { delete(o.modes, m) },
	})
	copied := make(chan struct{})
	go func() {
		defer close(copied)
		buf := make([]byte, 4096)
		for {
			n, err := user.Read(buf)
			if n > 0 {
				o.mu.Lock()
				o.emu.Write(buf[:n])
				o.mu.Unlock()
			}
			if err != nil {
				return
			}
		}
	}()
	stop := make(chan struct{})
	exited := make(chan struct{})
	b := newBar(showBar, "Test", nil, []string{"ctrl+q"}, nil, true)
	go func() {
		defer close(exited)
		o.done <- attach(s, term, b, tty, tty, []string{"TERM=xterm-256color"}, size{cols, rows}, nil, stop)
	}()
	t.Cleanup(func() {
		close(stop)
		<-exited
		tty.Close()
		user.Close()
		<-copied
	})
	return o
}

func (o *outerTerm) screen() string {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.emu.String()
}

func (o *outerTerm) hasMode(m ansi.Mode) bool {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.modes[m]
}

// On the primary screen, turning the mouse wheel scrolls the emulator's
// scrollback even when the program did not ask for mouse events (as agy and
// shells do), clicks do not write escape garbage to the program, new output
// keeps the viewed scrollback line anchored, and typing returns to the live
// screen.
func TestAttachWheelScrollsPrimaryScrollback(t *testing.T) {
	s := New()
	fifo := t.TempDir() + "/more"
	if err := syscall.Mkfifo(fifo, 0o600); err != nil {
		t.Fatal(err)
	}
	prog := fmt.Sprintf(`i=1; while [ $i -le 25 ]; do printf 'row-%%02d\n' $i; i=$((i+1)); done; (cat %q) & stty -echo; cat -v`, fifo)
	start(t, s, "a", prog)
	waitFor(t, func() (string, error) { return s.Capture("a") }, "row-25")

	o := attachTestTerm(t, s, "a", true)
	waitFor(t, func() (string, error) { return o.screen(), nil }, "row-25")
	if !o.hasMode(ansi.ModeMouseNormal) || !o.hasMode(ansi.ModeMouseExtSgr) {
		t.Fatalf("outer terminal lacks mouse reporting modes")
	}

	// Clicking when the program enabled no mouse mode must not reach cat -v.
	o.user.Write([]byte("\x1b[<0;5;5M\x1b[<0;5;5m"))

	// Two wheel-up notches (6 lines) bring earlier scrollback rows onto the
	// 9-row program viewport and scroll row-25 off the bottom.
	o.user.Write([]byte("\x1b[<64;5;5M\x1b[<64;5;5M"))
	scrolled := waitFor(t, func() (string, error) { return o.screen(), nil }, "row-12")
	if strings.Contains(scrolled, "row-25") {
		t.Fatalf("live bottom still visible while scrolled up:\n%s", scrolled)
	}
	topLine := strings.Split(scrolled, "\n")[0]

	// Streaming more lines below keeps the viewed scrollback anchored on the
	// same top line instead of jumping.
	more, err := os.OpenFile(fifo, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprint(more, "row-26\nrow-27\n")
	more.Close()
	waitFor(t, func() (string, error) { return s.Capture("a") }, "row-27")
	time.Sleep(3 * frame)
	if gotTop := strings.Split(o.screen(), "\n")[0]; gotTop != topLine {
		t.Errorf("top line shifted after new output: got %q, want %q", gotTop, topLine)
	}

	// Wheel-down returns toward the bottom, and typing jumps all the way back
	// to the live screen and delivers the typed text without the earlier click.
	o.user.Write([]byte("\x1b[<65;5;5M"))
	o.user.Write([]byte("hello\n"))
	live := waitFor(t, func() (string, error) { return o.screen(), nil }, "hello")
	if !strings.Contains(live, "row-27") {
		t.Errorf("typing did not return to live bottom:\n%s", live)
	}
	if strings.Contains(live, "<0;5;5") {
		t.Errorf("click reached a program that did not enable mouse mode:\n%s", live)
	}
}

// Programs that enter the alternate screen with mouse tracking (such as Claude
// Code fullscreen or agy's modals) receive wheel and click events, excluding
// clicks on Vineyard's status bar, and leaving the alternate screen with
// ?1002l/?1006l leaves primary-screen wheel scrollback working.
func TestAttachMouseModesAcrossAltScreen(t *testing.T) {
	s := New()
	ctl := t.TempDir() + "/ctl"
	if err := syscall.Mkfifo(ctl, 0o600); err != nil {
		t.Fatal(err)
	}
	prog := fmt.Sprintf(`i=1; while [ $i -le 25 ]; do printf 'hist-%%02d\n' $i; i=$((i+1)); done
printf '\033[?1049h\033[?1002h\033[?1006h\033[Halt-ready\n'
(read _ < %q; printf '\033[?1049l\033[?1002l\033[?1003l\033[?1006l\r\nmain-ready\n') &
stty -echo; cat -v`, ctl)
	start(t, s, "a", prog)
	waitFor(t, func() (string, error) { return s.Capture("a") }, "alt-ready")

	o := attachTestTerm(t, s, "a", true)
	waitFor(t, func() (string, error) { return o.screen(), nil }, "alt-ready")
	if !o.hasMode(ansi.ModeMouseButtonEvent) {
		t.Errorf("outer terminal did not upgrade to ModeMouseButtonEvent on alt screen")
	}

	// Row 10 is the status bar (program has 9 rows); row 3 reaches the program.
	o.user.Write([]byte("\x1b[<0;4;10M\x1b[<64;4;3M\x1b[<0;4;3M\r"))
	got := waitFor(t, func() (string, error) { return o.screen(), nil }, "^[[<0;4;3M")
	if !strings.Contains(got, "^[[<64;4;3M") || strings.Contains(got, "4;10M") {
		t.Errorf("unexpected mouse forwarding on alt screen:\n%s", got)
	}

	// Leaving the modal (?1049l, ?1002l, ?1006l) must keep ?1000h + ?1006h set
	// on the outer terminal and scroll primary history on wheel-up.
	f, err := os.OpenFile(ctl, os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	fmt.Fprintln(f)
	f.Close()
	waitFor(t, func() (string, error) { return o.screen(), nil }, "main-ready")
	if !o.hasMode(ansi.ModeMouseNormal) || !o.hasMode(ansi.ModeMouseExtSgr) {
		t.Errorf("outer terminal lost mouse modes after leaving alt screen")
	}
	o.user.Write([]byte("\x1b[<64;5;5M\x1b[<64;5;5M"))
	waitFor(t, func() (string, error) { return o.screen(), nil }, "hist-13")

	// Detach resets mouse modes on the outer terminal.
	o.user.Write([]byte{0x11}) // ctrl+q
	if err := <-o.done; err != nil {
		t.Fatalf("attach returned error: %v", err)
	}
	if o.hasMode(ansi.ModeMouseNormal) || o.hasMode(ansi.ModeMouseExtSgr) {
		t.Errorf("mouse modes stayed enabled on the outer terminal after detach")
	}
}

func TestParseMouse(t *testing.T) {
	// Bare ESC and arrow keys are never held back on normal reads.
	if _, n, inc := parseMouse([]byte("\x1b"), false); n != 0 || inc {
		t.Errorf("bare ESC: n=%d inc=%v, want 0, false", n, inc)
	}
	if _, n, inc := parseMouse([]byte("\x1b[A"), false); n != 0 || inc {
		t.Errorf("arrow key: n=%d inc=%v, want 0, false", n, inc)
	}
	// Incomplete SGR and legacy X10 sequences wait for the rest of the report.
	if _, _, inc := parseMouse([]byte("\x1b[<64;10"), false); !inc {
		t.Error("incomplete SGR sequence was not marked incomplete")
	}
	if _, _, inc := parseMouse([]byte("\x1b[M\x60\x25"), false); !inc {
		t.Error("incomplete X10 sequence was not marked incomplete")
	}
	// Legacy X10 wheel-up (button 64 -> byte 96) and release (button 3 -> byte 35).
	ev, n, inc := parseMouse([]byte("\x1b[M\x60\x25\x23rest"), false)
	if inc || n != 6 || !ev.wheelVertical() || !ev.wheelUp() || ev.x != 5 || ev.y != 3 {
		t.Errorf("X10 wheel-up = %+v n=%d inc=%v", ev, n, inc)
	}
	ev, n, inc = parseMouse([]byte("\x1b[M\x23\x25\x23"), false)
	if inc || n != 6 || !ev.release {
		t.Errorf("X10 release = %+v n=%d inc=%v", ev, n, inc)
	}
}
