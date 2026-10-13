package vterm

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/creack/pty"
)

// start runs program on a new terminal of s and ends it with the test.
func start(t *testing.T, s *Server, name, program string) {
	t.Helper()
	t.Setenv("SHELL", "/bin/sh")
	if err := s.Start(name, t.TempDir(), program, 40, 10); err != nil {
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

// Attaching draws the agent's screen, passes typed keys to it, and returns on
// ctrl-q without passing that on.
func TestAttachPassesInputUntilCtrlQ(t *testing.T) {
	s := New()
	start(t, s, "a", `printf 'agent screen\n'; stty -echo; cat -v`)
	waitFor(t, func() (string, error) { return s.Capture("a") }, "agent screen")

	user, tty, err := pty.Open()
	if err != nil {
		t.Fatal(err)
	}
	defer user.Close()
	defer tty.Close()
	if err := pty.Setsize(tty, &pty.Winsize{Cols: 50, Rows: 12}); err != nil {
		t.Fatal(err)
	}
	var drawn bytes.Buffer
	copied := make(chan struct{})
	go func() {
		io.Copy(&drawn, user)
		close(copied)
	}()

	a := s.Attach("a")
	a.SetStdin(tty)
	a.SetStdout(tty)
	done := make(chan error)
	go func() { done <- a.Run() }()

	waitFor(t, func() (string, error) { return s.Capture("a") }, "agent screen")
	user.Write([]byte("typed\r"))
	waitFor(t, func() (string, error) { return s.Capture("a") }, "typed")
	user.Write([]byte("more\r\x11ignored\r"))
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ctrl-q did not detach")
	}
	got := waitFor(t, func() (string, error) { return s.Capture("a") }, "more")
	if strings.Contains(got, "ignored") || strings.Contains(got, "^Q") {
		t.Errorf("input after ctrl-q reached the agent: %q", got)
	}
	tty.Close()
	user.Close()
	<-copied
	if !strings.Contains(drawn.String(), "agent screen") {
		t.Errorf("attach did not draw the agent's screen: %q", drawn.String())
	}
}
