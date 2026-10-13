package tmux

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/mikeryanboss/vineyard/internal/session"
)

// recorder is a Runner that records commands and replies with canned errors.
type recorder struct {
	calls  [][]string
	stderr string
	err    error
}

func (r *recorder) run(args ...string) ([]byte, []byte, error) {
	r.calls = append(r.calls, args)
	return nil, []byte(r.stderr), r.err
}

func TestStart_SizesSessionAndConfiguresServer(t *testing.T) {
	r := &recorder{}
	c := NewWithRunner("test", r.run)
	if err := c.Start("s1", "/work", "claude --flag", 80, 24); err != nil {
		t.Fatal(err)
	}
	if len(r.calls) != 2 {
		t.Fatalf("got %d tmux calls, want new-session then server config", len(r.calls))
	}
	got := strings.Join(r.calls[0], " ")
	want := "new-session -d -s s1 -c /work -x 80 -y 24 claude --flag"
	if got != want {
		t.Errorf("new-session args = %q, want %q", got, want)
	}
	if config := strings.Join(r.calls[1], " "); !strings.Contains(config, "mouse on") {
		t.Errorf("server config does not turn the mouse on: %q", config)
	}
}

func TestCommands_TargetSessionsExactly(t *testing.T) {
	r := &recorder{}
	c := NewWithRunner("test", r.run)
	_, _ = c.Capture("s1")
	_ = c.SendEnter("s1")
	_ = c.SendKeys("s1", "Down")
	_ = c.Exists("s1")
	_, _ = c.Created("s1")
	for _, call := range r.calls {
		joined := strings.Join(call, " ")
		if !strings.Contains(joined, "-t =s1") {
			t.Errorf("%q must target =s1 so session s1 never matches s10 by prefix", joined)
		}
	}
}

func TestKill_MissingSessionIsNotAnError(t *testing.T) {
	r := &recorder{stderr: "can't find session: =gone", err: errors.New("exit status 1")}
	c := NewWithRunner("test", r.run)
	if err := c.Kill("gone"); err != nil {
		t.Errorf("Kill of a missing session = %v, want nil", err)
	}
}

func TestCapture_MissingSessionReportsErrNoSession(t *testing.T) {
	r := &recorder{stderr: "no server running on /tmp/tmux-1000/test", err: errors.New("exit status 1")}
	c := NewWithRunner("test", r.run)
	if _, err := c.Capture("gone"); !errors.Is(err, ErrNoSession) {
		t.Errorf("err = %v, want ErrNoSession", err)
	}
}

func TestAttachCommand_DropsTMUXAndRestoresWindowSizing(t *testing.T) {
	t.Setenv("TMUX", "/tmp/tmux-1000/default,1,0")
	cmd := New("test").AttachCommand("s1", session.Attachment{Detach: []string{"ctrl+q"}})
	args := strings.Join(cmd.Args, " ")
	if !strings.Contains(args, "-L test") || !strings.Contains(args, "window-size latest ;") || !strings.HasSuffix(args, "attach-session -t =s1") {
		t.Errorf("attach args = %q", args)
	}
	for _, kv := range cmd.Env {
		if strings.HasPrefix(kv, "TMUX=") {
			t.Error("TMUX must be removed so attaching works from inside tmux")
		}
	}
}

// The detach and status bar keys are bound in a key table of the session's
// own, named after them, so Vineyards with other keys sharing the server
// keep theirs.
func TestAttachCommand_BindsTheDetachKeysForTheSession(t *testing.T) {
	cmd := New("test").AttachCommand("s1", session.Attachment{Detach: []string{"ctrl+g", "ctrl+x"}, ToggleBar: []string{"ctrl+\\"}})
	args := strings.Join(cmd.Args, " ")
	for _, want := range []string{
		"bind-key -T vineyard-detach-C-g-C-x-bar-C-backslash C-g detach-client ;",
		"bind-key -T vineyard-detach-C-g-C-x-bar-C-backslash C-x detach-client ;",
		"bind-key -T vineyard-detach-C-g-C-x-bar-C-backslash C-\\ if-shell -F #{==:#{status},off} set-option status on set-option status off ;",
		"set-option -t =s1: key-table vineyard-detach-C-g-C-x-bar-C-backslash ;",
		"set-option -t =s1: status off ;",
		"status-right  ctrl+g or ctrl+x: back to vineyard  ;",
	} {
		if !strings.Contains(args, want) {
			t.Errorf("attach args lack %q: %q", want, args)
		}
	}
	if strings.Contains(args, "C-q") {
		t.Errorf("attach args bind the default detach key: %q", args)
	}
}

// TestLiveSession runs against a real tmux server on a throwaway socket.
func TestLiveSession(t *testing.T) {
	if _, err := exec.LookPath("tmux"); err != nil {
		t.Skip("tmux not installed")
	}
	socket := fmt.Sprintf("vineyard-test-%d", os.Getpid())
	c := New(socket)
	t.Cleanup(func() { _ = exec.Command("tmux", "-L", socket, "kill-server").Run() })

	before := time.Now().Truncate(time.Second)
	if err := c.Start("live", t.TempDir(), "sh", 60, 10); err != nil {
		t.Fatal(err)
	}
	if created, err := c.Created("live"); err != nil || created.Before(before) || created.After(time.Now()) {
		t.Errorf("Created = %v, %v; want the time Start ran", created, err)
	}
	if !c.Exists("live") {
		t.Fatal("session should exist after Start")
	}
	if c.Exists("liv") {
		t.Fatal("a name prefix must not match the session")
	}

	if err := c.Paste("live", "echo vine-yard"); err != nil {
		t.Fatal(err)
	}
	if err := c.SendEnter("live"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, func() bool {
		screen, err := c.Capture("live")
		return err == nil && strings.Count(screen, "vine-yard") >= 2 // the command and its output
	})

	if err := c.Resize("live", 40, 8); err != nil {
		t.Fatal(err)
	}
	screen, _ := c.Capture("live")
	if lines := strings.Count(screen, "\n"); lines != 8 {
		t.Errorf("captured %d lines after resize, want 8", lines)
	}

	// Attaching needs a terminal; the commands before attach-session must
	// run on a real server.
	attach := New(socket).AttachCommand("live", session.Attachment{Detach: []string{"ctrl+g"}, ToggleBar: []string{"ctrl+\\"}, ShowBar: true})
	setup := attach.Args[:len(attach.Args)-4] // drop "; attach-session -t =live"
	if out, err := exec.Command(setup[0], setup[1:]...).CombinedOutput(); err != nil {
		t.Fatalf("attach setup %q: %v: %s", setup, err, out)
	}
	if out, _ := exec.Command("tmux", "-L", socket, "show-options", "-t", "=live:", "key-table").Output(); string(out) != "key-table vineyard-detach-C-g-bar-C-backslash\n" {
		t.Errorf("session key-table = %q after the attach setup", out)
	}
	keys, _ := exec.Command("tmux", "-L", socket, "list-keys", "-T", "vineyard-detach-C-g-bar-C-backslash").Output()
	if !strings.Contains(string(keys), "if-shell -F") || !strings.Contains(string(keys), "detach-client") {
		t.Errorf("the detach and status bar keys are not bound:\n%s", keys)
	}

	if err := c.Kill("live"); err != nil {
		t.Fatal(err)
	}
	if c.Exists("live") {
		t.Error("session should be gone after Kill")
	}
	if _, err := c.Capture("live"); !errors.Is(err, ErrNoSession) {
		t.Errorf("capture after kill: err = %v, want ErrNoSession", err)
	}
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cond() {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatal("condition not met within 5s")
}
