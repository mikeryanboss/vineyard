package main

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/mikeryanboss/vineyard/internal/session"
	"github.com/mikeryanboss/vineyard/internal/tui"
	"github.com/mikeryanboss/vineyard/internal/vterm"
)

// vineyards are the Vineyard multiplexers, one for each protocol, on sockets
// side by side: this build's, which it starts when it first starts a
// terminal, and older ones that earlier Vineyards started and that run their
// agents until those end. An update that changes the protocol thus leaves the
// agents running, and new sessions start on the new multiplexer.
type vineyards struct {
	// dir holds the sockets, and exe runs a multiplexer.
	dir, exe string

	mu    sync.Mutex
	older map[int]tui.Terminal
}

func newVineyards() (*vineyards, error) {
	socket, err := vterm.SocketPath(vineyardSocketName(vterm.Protocol()))
	if err != nil {
		return nil, err
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	return &vineyards{dir: filepath.Dir(socket), exe: exe, older: map[int]tui.Terminal{}}, nil
}

// vineyardSocketName names the socket of the Vineyard multiplexer of
// protocol. Protocol 0 names the one socket that servers of protocols 1 to 3
// shared before each protocol had its own, where the sessions recorded then
// run.
func vineyardSocketName(protocol int) string {
	if protocol == 0 {
		return socketName()
	}
	return socketName() + "-p" + strconv.Itoa(protocol)
}

// client returns a client of the multiplexer of protocol, which starts it
// when it starts a terminal and none runs.
func (v *vineyards) client(protocol int) *vterm.Client {
	socket := filepath.Join(v.dir, vineyardSocketName(protocol))
	return vterm.NewClient(socket, v.exe, "server", socket)
}

// current returns a client of this build's multiplexer.
func (v *vineyards) current() *vterm.Client { return v.client(vterm.Protocol()) }

// Older returns the multiplexer that runs the sessions recorded with
// protocol, another than this build's: a client of it when it speaks this
// build's protocol, as one on protocol 0's socket may; a client for all but
// attaching when its calls match this build's; and otherwise one that only
// tells whether it still runs a terminal.
func (v *vineyards) Older(protocol int) tui.Terminal {
	v.mu.Lock()
	defer v.mu.Unlock()
	if t, ok := v.older[protocol]; ok {
		return t
	}
	client := v.client(protocol)
	info, err := client.Info()
	var t tui.Terminal
	switch {
	case err == nil && info != nil && info.Protocol == vterm.Protocol():
		t = client
	case err == nil && info != nil && vterm.SameCalls(info.Protocol):
		t = vterm.NewOlderClient(client.Socket())
	default:
		t = olderVineyard{client, olderErr(info, err)}
	}
	v.older[protocol] = t
	return t
}

// olderErr is the error of calls to the terminals of an older multiplexer,
// described by info, or by err when it did not answer.
func olderErr(info *vterm.ServerInfo, err error) error {
	switch {
	case err != nil:
		return fmt.Errorf("%w: %v", session.ErrOlderMultiplexer, err)
	case info == nil:
		return fmt.Errorf("%w, which no longer runs", session.ErrOlderMultiplexer)
	}
	return fmt.Errorf("%w (process %d, protocol %d), which this Vineyard cannot reach. Use the Vineyard that started it, or let the agent finish",
		session.ErrOlderMultiplexer, info.PID, info.Protocol)
}

// protocols returns the protocols of the Vineyard multiplexers whose sockets
// there are, this build's first, whether or not it runs.
func (v *vineyards) protocols() ([]int, error) {
	protocols := []int{vterm.Protocol()}
	if _, err := os.Stat(filepath.Join(v.dir, vineyardSocketName(0))); err == nil {
		protocols = append(protocols, 0)
	}
	matches, err := filepath.Glob(filepath.Join(v.dir, vineyardSocketName(0)+"-p*"))
	if err != nil {
		return nil, err
	}
	for _, path := range matches {
		n, err := strconv.Atoi(strings.TrimPrefix(filepath.Base(path), vineyardSocketName(0)+"-p"))
		if err == nil && n != vterm.Protocol() {
			protocols = append(protocols, n)
		}
	}
	slices.Sort(protocols[1:])
	return protocols, nil
}

// olderVineyard is a Vineyard multiplexer of a protocol whose calls differ
// from this build's, which this build cannot use. It can only tell whether the
// multiplexer still runs a terminal; while it does, every other call fails
// with err, which says so.
type olderVineyard struct {
	server *vterm.Client
	err    error
}

func (o olderVineyard) Exists(name string) bool { return o.server.Holds(name) }

// fail returns o.err while the named terminal runs, and otherwise that it
// does not.
func (o olderVineyard) fail(name string) error {
	if o.Exists(name) {
		return o.err
	}
	return fmt.Errorf("%w: %s", vterm.ErrNoTerminal, name)
}

func (o olderVineyard) Start(string, string, string, int, int) error { return o.err }

// Kill fails while the terminal runs, which this build cannot end.
func (o olderVineyard) Kill(name string) error {
	if o.Exists(name) {
		return o.err
	}
	return nil
}

func (o olderVineyard) Created(name string) (time.Time, error) { return time.Time{}, o.fail(name) }
func (o olderVineyard) Capture(name string) (string, error)    { return "", o.fail(name) }
func (o olderVineyard) CaptureHistory(name string, _ int) (string, error) {
	return "", o.fail(name)
}
func (o olderVineyard) Resize(name string, _, _ int) error      { return o.fail(name) }
func (o olderVineyard) SendEnter(name string) error             { return o.fail(name) }
func (o olderVineyard) SendKeys(name string, _ ...string) error { return o.fail(name) }
func (o olderVineyard) Paste(name, _ string) error              { return o.fail(name) }
func (o olderVineyard) Attach(name string, _ session.Attachment) tea.ExecCommand {
	return failedCommand{o.fail(name)}
}

// failedCommand fails with err without running anything.
type failedCommand struct{ err error }

func (f failedCommand) Run() error        { return f.err }
func (failedCommand) SetStdin(io.Reader)  {}
func (failedCommand) SetStdout(io.Writer) {}
func (failedCommand) SetStderr(io.Writer) {}
