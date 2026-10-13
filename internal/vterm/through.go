package vterm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/mikeryanboss/vineyard/internal/session"
)

// Attaching is the part of the protocol that changes, and a build's client
// speaks only its own. So a Vineyard attaches to an older server's terminal
// through the executable that server runs, which speaks its protocol:
//
//	<executable> attach <socket> <terminal> <attachment as JSON>
//
// Those arguments are the contract between versions; never change them. JSON
// ignores fields it does not know, so session.Attachment may grow. A build
// before this command exits with status 2, as for any unknown command, and
// RunAttach never does.

// AttachThroughServer returns the command that attaches to the named
// terminal through the server's own executable, as the server's process
// still has it: on Linux, /proc/<pid>/exe runs it even after the file was
// replaced, as by an update.
func (c *Client) AttachThroughServer(name string, how session.Attachment) tea.ExecCommand {
	return &throughServer{client: c, name: name, how: how}
}

type throughServer struct {
	client *Client
	name   string
	how    session.Attachment
	in     io.Reader
	out    io.Writer
	errOut bytes.Buffer
}

func (a *throughServer) SetStdin(r io.Reader)  { a.in = r }
func (a *throughServer) SetStdout(w io.Writer) { a.out = w }
func (a *throughServer) SetStderr(io.Writer)   {}

func (a *throughServer) Run() error {
	info, err := a.client.Info()
	if err != nil || info == nil {
		return fmt.Errorf("%w, which no longer answers: %v", session.ErrOlderMultiplexer, err)
	}
	how, err := json.Marshal(a.how)
	if err != nil {
		return err
	}
	exe := fmt.Sprintf("/proc/%d/exe", info.PID)
	cmd := exec.Command(exe, "attach", a.client.socket, a.name, string(how))
	cmd.Stdin, cmd.Stdout, cmd.Stderr = a.in, a.out, &a.errOut
	err = cmd.Run()
	var exit *exec.ExitError
	switch {
	case err == nil:
		return nil
	case errors.As(err, &exit) && exit.ExitCode() == 2:
		return fmt.Errorf("use the Vineyard that started it; %w (process %d, protocol %d), whose executable predates attaching through it",
			session.ErrOlderMultiplexer, info.PID, info.Protocol)
	case errors.As(err, &exit):
		return fmt.Errorf("attaching through %s: %s", exe, strings.TrimSpace(a.errOut.String()))
	}
	// Without /proc, as on macOS, there is no executable to run.
	return fmt.Errorf("use the Vineyard that started it; %w (process %d, protocol %d), whose executable cannot run here: %v",
		session.ErrOlderMultiplexer, info.PID, info.Protocol, err)
}

// RunAttach is the attach command: it attaches the terminal on standard
// input and output to the named terminal of the server at socket, with this
// build's client, as how, a session.Attachment in JSON, says.
func RunAttach(socket, name, how string) error {
	var attachment session.Attachment
	if err := json.Unmarshal([]byte(how), &attachment); err != nil {
		return fmt.Errorf("attachment: %w", err)
	}
	a := NewClient(socket).Attach(name, attachment)
	a.SetStdin(os.Stdin)
	a.SetStdout(os.Stdout)
	return a.Run()
}
