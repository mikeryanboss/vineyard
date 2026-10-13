package vterm

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/term"

	"github.com/mikeryanboss/vineyard/internal/session"
)

// serverStartTimeout is how long a client waits for a server it started to
// listen.
const serverStartTimeout = 5 * time.Second

// Client runs terminals on the server at a socket, starting the server when
// it first starts a terminal and none runs.
type Client struct {
	socket string
	// server is the command that runs a server on socket.
	server []string

	mu    sync.Mutex
	calls *rpc.Client
}

// NewClient returns a client of the server at socket, which server, a
// command, runs.
func NewClient(socket string, server ...string) *Client {
	return &Client{socket: socket, server: server}
}

// Socket returns the path of the server's socket.
func (c *Client) Socket() string { return c.socket }

// Close closes the connection to the server.
func (c *Client) Close() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.calls == nil {
		return nil
	}
	err := c.calls.Close()
	c.calls = nil
	return err
}

// noServer reports whether err says no server listens on the socket.
func noServer(err error) bool {
	return errors.Is(err, syscall.ENOENT) || errors.Is(err, syscall.ECONNREFUSED)
}

func (c *Client) dial() (*net.UnixConn, error) {
	return net.DialUnix("unix", nil, &net.UnixAddr{Name: c.socket, Net: "unix"})
}

// connect returns the connection for calls, dialing the server if need be.
// With start, it starts a server when none runs, and waits out a server that
// is stopping.
func (c *Client) connect(start bool) (*rpc.Client, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.calls != nil {
		return c.calls, nil
	}
	for deadline := time.Now().Add(serverStartTimeout); ; time.Sleep(10 * time.Millisecond) {
		calls, v, err := c.handshake(start)
		if err == nil && v.Protocol != protocol {
			calls.Close()
			return nil, fmt.Errorf("the Vineyard server at %s (process %d) is from another version of Vineyard (protocol %d, want %d); end its agents or kill it", c.socket, v.PID, v.Protocol, protocol)
		}
		if err == nil {
			c.calls = calls
			return calls, nil
		}
		if !start || !hungUp(err) || time.Now().After(deadline) {
			return nil, err
		}
	}
}

// handshake dials the server and asks its version. With start, it starts a
// server when none runs.
func (c *Client) handshake(start bool) (*rpc.Client, VersionReply, error) {
	var v VersionReply
	conn, err := c.dial()
	if err != nil && start && noServer(err) {
		conn, err = c.startServer()
	}
	if err != nil {
		return nil, v, err
	}
	conn.SetDeadline(time.Now().Add(serverStartTimeout))
	if _, err := conn.Write([]byte{connRPC}); err != nil {
		conn.Close()
		return nil, v, err
	}
	calls := rpc.NewClient(conn)
	if err := calls.Call("Terminals.Version", struct{}{}, &v); err != nil {
		calls.Close()
		return nil, v, fmt.Errorf("Vineyard server at %s: %w", c.socket, err)
	}
	conn.SetDeadline(time.Time{})
	return calls, v, nil
}

// hungUp reports whether err says the server hung up, as one that is stopping
// does.
func hungUp(err error) bool {
	return errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) || errors.Is(err, rpc.ErrShutdown) ||
		errors.Is(err, syscall.ECONNRESET) || errors.Is(err, syscall.EPIPE)
}

// startServer starts a server in a session of its own, so that it outlives
// this process and the user's terminal, and dials it. Its output goes to a log
// beside the socket.
func (c *Client) startServer() (*net.UnixConn, error) {
	log, err := os.OpenFile(c.socket+".log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, err
	}
	defer log.Close()
	cmd := exec.Command(c.server[0], c.server[1:]...)
	cmd.Dir = "/"
	cmd.Stdout, cmd.Stderr = log, log
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("starting the Vineyard server: %w", err)
	}
	go cmd.Wait()
	for deadline := time.Now().Add(serverStartTimeout); ; time.Sleep(10 * time.Millisecond) {
		conn, err := c.dial()
		if err == nil || !noServer(err) {
			return conn, err
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("the Vineyard server did not start; see %s", log.Name())
		}
	}
}

// call calls the server's method. Without start, a missing server is an
// error that noServer recognises.
func (c *Client) call(start bool, method string, args, reply any) error {
	calls, err := c.connect(start)
	if err != nil {
		return err
	}
	err = calls.Call("Terminals."+method, args, reply)
	if hungUp(err) {
		// The server is gone; the next call dials anew.
		c.mu.Lock()
		if c.calls == calls {
			c.calls = nil
		}
		c.mu.Unlock()
		calls.Close()
	}
	return err
}

// callTerminal calls a method on the named terminal, which cannot exist when
// no server runs.
func (c *Client) callTerminal(name, method string, args, reply any) error {
	err := c.call(false, method, args, reply)
	if noServer(err) {
		return fmt.Errorf("%w: %s (no Vineyard server)", ErrNoTerminal, name)
	}
	return err
}

// Start runs program in dir on a new terminal of the server, with this
// process's environment.
func (c *Client) Start(name, dir, program string, width, height int) error {
	args := StartArgs{Name: name, Dir: dir, Program: program, Env: os.Environ(), Width: width, Height: height}
	return c.call(true, "Start", args, &struct{}{})
}

// Exists reports whether the named terminal's program is running.
func (c *Client) Exists(name string) bool {
	var ok bool
	return c.call(false, "Exists", name, &ok) == nil && ok
}

// Created returns when the named terminal started its program.
func (c *Client) Created(name string) (time.Time, error) {
	var created time.Time
	err := c.callTerminal(name, "Created", name, &created)
	return created, err
}

// Capture returns the visible screen, with colours as ANSI escape sequences.
func (c *Client) Capture(name string) (string, error) {
	var screen string
	err := c.callTerminal(name, "Capture", name, &screen)
	return screen, err
}

// CaptureHistory returns up to lines of scrollback above the visible screen,
// followed by the screen itself.
func (c *Client) CaptureHistory(name string, lines int) (string, error) {
	var history string
	err := c.callTerminal(name, "CaptureHistory", HistoryArgs{Name: name, Lines: lines}, &history)
	return history, err
}

// Resize sets the terminal's size.
func (c *Client) Resize(name string, width, height int) error {
	return c.callTerminal(name, "Resize", ResizeArgs{Name: name, Width: width, Height: height}, &struct{}{})
}

// SendEnter presses Enter in the terminal.
func (c *Client) SendEnter(name string) error { return c.SendKeys(name, "Enter") }

// SendKeys presses keys in the terminal, named as tmux names them.
func (c *Client) SendKeys(name string, keys ...string) error {
	return c.callTerminal(name, "SendKeys", KeysArgs{Name: name, Keys: keys}, &struct{}{})
}

// Paste types text into the terminal, as a bracketed paste if the program
// asked for those.
func (c *Client) Paste(name, text string) error {
	return c.callTerminal(name, "Paste", PasteArgs{Name: name, Text: text}, &struct{}{})
}

// Kill hangs up the terminal, ending its program and the program's children.
// A missing terminal is not an error.
func (c *Client) Kill(name string) error {
	err := c.call(false, "Kill", name, &struct{}{})
	if noServer(err) {
		return nil
	}
	return err
}

// Attach returns the command that hands the user's terminal to the named
// terminal until the user presses one of the detach keys, or its program
// exits. As tmux's client does, it passes the user's terminal to the server,
// which draws the terminal's screen and its status bar on it and reads the
// user's input from it.
func (c *Client) Attach(name string, how session.Attachment) tea.ExecCommand {
	return &attachment{client: c, name: name, how: how}
}

type attachment struct {
	client *Client
	name   string
	how    session.Attachment
	in     io.Reader
	out    io.Writer
}

func (a *attachment) SetStdin(r io.Reader)  { a.in = r }
func (a *attachment) SetStdout(w io.Writer) { a.out = w }
func (a *attachment) SetStderr(io.Writer)   {}

func (a *attachment) Run() error {
	in, inOK := a.in.(*os.File)
	out, outOK := a.out.(*os.File)
	if !inOK || !outOK || !term.IsTerminal(in.Fd()) || !term.IsTerminal(out.Fd()) {
		return errors.New("attaching needs a terminal")
	}
	conn, err := a.client.dial()
	if noServer(err) {
		return fmt.Errorf("%w: %s (no Vineyard server)", ErrNoTerminal, a.name)
	}
	if err != nil {
		return err
	}
	defer conn.Close()

	resized := make(chan os.Signal, 1)
	signal.Notify(resized, syscall.SIGWINCH)
	defer signal.Stop(resized)
	width, height, err := term.GetSize(out.Fd())
	if err != nil {
		return fmt.Errorf("terminal size: %w", err)
	}
	rights := syscall.UnixRights(int(in.Fd()), int(out.Fd()))
	if _, _, err := conn.WriteMsgUnix([]byte{connAttach}, rights, nil); err != nil {
		return err
	}
	enc := gob.NewEncoder(conn)
	req := AttachRequest{
		Name: a.name, Env: os.Environ(), Width: width, Height: height,
		Detach: a.how.Detach, ToggleBar: a.how.ToggleBar, ShowBar: a.how.ShowBar, Peers: a.how.Peers,
	}
	if err := enc.Encode(req); err != nil {
		return err
	}

	done := make(chan error, 1)
	go func() {
		var result AttachResult
		err := gob.NewDecoder(conn).Decode(&result)
		if err == nil && result.Err != "" {
			err = errors.New(result.Err)
		}
		done <- err
	}()
	for {
		select {
		case <-resized:
			width, height, err := term.GetSize(out.Fd())
			if err != nil {
				return fmt.Errorf("terminal size: %w", err)
			}
			if err := enc.Encode(size{width, height}); err != nil {
				return err
			}
		case err := <-done:
			return err
		}
	}
}
