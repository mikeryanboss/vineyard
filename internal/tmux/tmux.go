// Package tmux runs agent programs in detached tmux sessions and mirrors their
// screens.
//
// Vineyard uses its own tmux server (a named socket) rather than the user's
// default one. That keeps its key bindings, such as ctrl-q to detach, from
// leaking into the user's own tmux sessions. List them with `tmux -L vineyard ls`.
package tmux

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

// DefaultSocket is the tmux socket name Vineyard's sessions live on.
const DefaultSocket = "vineyard"

// DetachKey is bound on Vineyard's tmux server to return to the TUI.
const DetachKey = "C-q"

// ErrNoSession reports that a tmux session does not exist.
var ErrNoSession = errors.New("tmux session not found")

// Runner executes tmux with args and returns stdout and stderr. It is a seam
// so tests can exercise command construction without a tmux server.
type Runner func(args ...string) (stdout, stderr []byte, err error)

// Client talks to one tmux server.
type Client struct {
	socket string
	run    Runner
}

// New returns a client for the tmux server on the named socket.
func New(socket string) *Client {
	c := &Client{socket: socket}
	c.run = c.exec
	return c
}

// NewWithRunner returns a client that sends commands to run instead of tmux.
func NewWithRunner(socket string, run Runner) *Client {
	return &Client{socket: socket, run: run}
}

// Socket returns the socket name the client uses.
func (c *Client) Socket() string { return c.socket }

func (c *Client) exec(args ...string) ([]byte, []byte, error) {
	cmd := exec.Command("tmux", append([]string{"-L", c.socket}, args...)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.Bytes(), stderr.Bytes(), err
}

// command runs tmux and folds stderr into the returned error.
func (c *Client) command(args ...string) (string, error) {
	stdout, stderr, err := c.run(args...)
	if err == nil {
		return string(stdout), nil
	}
	msg := strings.TrimSpace(string(stderr))
	if isMissingSession(msg) {
		return "", fmt.Errorf("%w: %s", ErrNoSession, msg)
	}
	if msg == "" {
		return "", fmt.Errorf("tmux %s: %w", args[0], err)
	}
	return "", fmt.Errorf("tmux %s: %s", args[0], msg)
}

func isMissingSession(stderr string) bool {
	stderr = strings.ToLower(stderr)
	for _, phrase := range []string{"can't find session", "no server running", "error connecting to", "no such file or directory"} {
		if strings.Contains(stderr, phrase) {
			return true
		}
	}
	return false
}

// exact makes tmux match a session name exactly instead of by prefix.
func exact(name string) string { return "=" + name }

// Start creates a detached session running program in dir, sized to the
// preview so the agent lays itself out for the space it is shown in.
func (c *Client) Start(name, dir, program string, width, height int) error {
	args := []string{"new-session", "-d", "-s", name, "-c", dir}
	if width > 0 && height > 0 {
		args = append(args, "-x", strconv.Itoa(width), "-y", strconv.Itoa(height))
	}
	if program != "" {
		args = append(args, program)
	}
	if _, err := c.command(args...); err != nil {
		return err
	}
	return c.configureServer()
}

// configureServer applies Vineyard's server-wide settings. It runs after every
// session start because the server exits, and forgets them, when its last
// session ends. The settings are idempotent.
func (c *Client) configureServer() error {
	_, err := c.command(
		"set-option", "-g", "history-limit", "10000", ";",
		"set-option", "-g", "mouse", "on", ";",
		"set-option", "-g", "status-right", " ctrl-q: back to vineyard ", ";",
		"bind-key", "-n", DetachKey, "detach-client",
	)
	return err
}

// Exists reports whether the named session is running.
func (c *Client) Exists(name string) bool {
	_, err := c.command("has-session", "-t", exact(name))
	return err == nil
}

// Capture returns the visible screen of the session's active pane, with
// colours preserved as ANSI escape sequences.
func (c *Client) Capture(name string) (string, error) {
	return c.command("capture-pane", "-p", "-e", "-t", exact(name)+":")
}

// CaptureHistory returns up to lines of scrollback above the visible screen,
// followed by the screen itself.
func (c *Client) CaptureHistory(name string, lines int) (string, error) {
	return c.command("capture-pane", "-p", "-e", "-S", "-"+strconv.Itoa(lines), "-t", exact(name)+":")
}

// Resize sets the session's window size. Detached windows keep this size
// until a client attaches.
func (c *Client) Resize(name string, width, height int) error {
	if width <= 0 || height <= 0 {
		return nil
	}
	_, err := c.command("resize-window", "-t", exact(name)+":", "-x", strconv.Itoa(width), "-y", strconv.Itoa(height))
	return err
}

// SendEnter presses Enter in the session.
func (c *Client) SendEnter(name string) error {
	_, err := c.command("send-keys", "-t", exact(name)+":", "Enter")
	return err
}

// Paste types text into the session as a bracketed paste, so multi-line text
// arrives as one input instead of being submitted line by line.
func (c *Client) Paste(name, text string) error {
	buffer := "vineyard-" + name
	_, err := c.command(
		"set-buffer", "-b", buffer, "--", text, ";",
		"paste-buffer", "-d", "-p", "-b", buffer, "-t", exact(name)+":",
	)
	return err
}

// Kill ends the session and every process in it. A missing session is not an error.
func (c *Client) Kill(name string) error {
	_, err := c.command("kill-session", "-t", exact(name))
	if errors.Is(err, ErrNoSession) {
		return nil
	}
	return err
}

// AttachCommand builds the command that hands the terminal to the session.
//
// The window is switched back to following the client's size first: Resize
// pins detached windows to the preview size, which would otherwise leave the
// attached view the size of the preview pane. TMUX is removed from the
// environment so attaching works when Vineyard itself runs inside tmux.
func (c *Client) AttachCommand(name string) *exec.Cmd {
	cmd := exec.Command("tmux", "-L", c.socket,
		"set-option", "-w", "-t", exact(name)+":", "window-size", "latest", ";",
		"attach-session", "-t", exact(name),
	)
	cmd.Env = withoutEnv(os.Environ(), "TMUX")
	return cmd
}

func withoutEnv(env []string, key string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		if !strings.HasPrefix(kv, key+"=") {
			out = append(out, kv)
		}
	}
	return out
}
