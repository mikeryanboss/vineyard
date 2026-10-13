package vterm

import (
	"encoding/gob"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/mikeryanboss/vineyard/internal/session"
)

// protocol is the version of the calls between Vineyards and the server.
// Change it with any change to the calls, their arguments, or attaching.
const protocol = 3

// idleGrace is how long a new server runs before it may stop when idle,
// unless a terminal starts sooner.
const idleGrace = 10 * time.Second

// A connection's first byte says what it carries.
const (
	// connRPC carries net/rpc calls.
	connRPC = 'r'
	// connAttach carries one attachment. The byte comes with the user's
	// terminal's input and output file descriptors, and is followed by an
	// AttachRequest, then sizes, from the client, and an AttachResult from
	// the server.
	connAttach = 'a'
)

// SocketPath returns the path of the server socket called name, in a
// directory only the user may use, as tmux keeps its sockets in
// /tmp/tmux-<uid>. It creates the directory.
func SocketPath(name string) (string, error) {
	dir := filepath.Join(os.TempDir(), fmt.Sprintf("vineyard-%d", os.Getuid()))
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", err
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return "", err
	}
	stat := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || int(stat.Uid) != os.Getuid() || info.Mode().Perm()&0o077 != 0 {
		return "", fmt.Errorf("%s must be a directory only you can use", dir)
	}
	return filepath.Join(dir, name), nil
}

// Serve runs the server on socket. It returns once it has neither terminals
// nor connections, or at once when another server already listens on socket.
func Serve(socket string) error {
	d := &daemon{socket: socket, terminals: New(), stopped: make(chan struct{})}
	d.terminals.exited = d.exitIfIdle
	// Servers starting at once probe this one, and the client that started
	// it may connect after them, so it is not idle before a terminal starts
	// or a grace period passes.
	time.AfterFunc(idleGrace, d.arm)
	listener, err := d.listen()
	if err != nil || listener == nil {
		return err
	}
	d.listener = listener
	if d.listening, err = os.Stat(socket); err != nil {
		return err
	}
	calls := rpc.NewServer()
	if err := calls.RegisterName("Terminals", &Service{d}); err != nil {
		return err
	}
	for {
		conn, err := listener.AcceptUnix()
		if err != nil {
			d.mu.Lock()
			closed := d.closed
			d.mu.Unlock()
			if closed {
				<-d.stopped
				return nil
			}
			return err
		}
		d.mu.Lock()
		if d.closed {
			d.mu.Unlock()
			conn.Close()
			continue
		}
		d.conns++
		d.mu.Unlock()
		go func() {
			if err := d.serve(conn, calls); err != nil {
				fmt.Fprintln(os.Stderr, err)
			}
			conn.Close()
			d.mu.Lock()
			d.conns--
			d.mu.Unlock()
			d.exitIfIdle()
		}()
	}
}

type daemon struct {
	socket    string
	terminals *Server
	listener  *net.UnixListener
	// listening is the socket file this server listens on.
	listening fs.FileInfo

	// mu guards conns, armed, and closed.
	mu    sync.Mutex
	conns int
	// armed is set once the server may stop when idle.
	armed bool
	// closed is set when the server starts stopping, and stopped is closed
	// when it has stopped listening.
	closed  bool
	stopped chan struct{}
}

// arm lets the server stop when idle, and stops it if it is.
func (d *daemon) arm() {
	d.mu.Lock()
	d.armed = true
	d.mu.Unlock()
	d.exitIfIdle()
}

// lock takes the lock beside the socket, which serializes servers starting
// and stopping on it, and returns the function that releases it.
func (d *daemon) lock() (func(), error) {
	f, err := os.OpenFile(d.socket+".lock", os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return func() { f.Close() }, nil
}

// listen listens on the socket, replacing a stale one. It returns no listener
// when another server answers on the socket.
func (d *daemon) listen() (*net.UnixListener, error) {
	unlock, err := d.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	if calls, _, err := (&Client{socket: d.socket}).handshake(false); err == nil {
		calls.Close()
		return nil, nil
	}
	if err := os.Remove(d.socket); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return nil, err
	}
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: d.socket, Net: "unix"})
	if err != nil {
		return nil, err
	}
	// Stopping removes the socket under the lock instead, and only its own,
	// not that of a server started while it was stopping.
	listener.SetUnlinkOnClose(false)
	return listener, nil
}

// exitIfIdle stops the server when it has neither terminals nor connections.
// A terminal starts only through a connection, so none can start meanwhile.
// Connections that arrive while it stops are hung up.
func (d *daemon) exitIfIdle() {
	d.mu.Lock()
	if !d.armed || d.closed || d.conns > 0 || d.terminals.count() > 0 {
		d.mu.Unlock()
		return
	}
	d.closed = true
	d.mu.Unlock()
	defer close(d.stopped)
	unlock, err := d.lock()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	defer unlock()
	d.listener.Close()
	if info, err := os.Stat(d.socket); err == nil && os.SameFile(info, d.listening) {
		os.Remove(d.socket)
	}
}

// serve answers one connection.
func (d *daemon) serve(conn *net.UnixConn, calls *rpc.Server) error {
	kind := make([]byte, 1)
	oob := make([]byte, syscall.CmsgSpace(2*4))
	n, oobn, _, _, err := conn.ReadMsgUnix(kind, oob)
	if err != nil || n == 0 {
		return err
	}
	switch kind[0] {
	case connRPC:
		calls.ServeConn(conn)
		return nil
	case connAttach:
		in, out, err := terminalFiles(oob[:oobn])
		if err != nil {
			return err
		}
		defer in.Close()
		defer out.Close()
		return d.attach(conn, in, out)
	}
	return fmt.Errorf("unknown connection kind %q", kind[0])
}

// terminalFiles returns the input and output files an attaching client sent.
func terminalFiles(oob []byte) (in, out *os.File, err error) {
	msgs, err := syscall.ParseSocketControlMessage(oob)
	if err != nil {
		return nil, nil, err
	}
	var fds []int
	for _, msg := range msgs {
		rights, err := syscall.ParseUnixRights(&msg)
		if err != nil {
			return nil, nil, err
		}
		fds = append(fds, rights...)
	}
	if len(fds) != 2 {
		for _, fd := range fds {
			syscall.Close(fd)
		}
		return nil, nil, fmt.Errorf("attach sent %d file descriptors, want 2", len(fds))
	}
	return os.NewFile(uintptr(fds[0]), "input"), os.NewFile(uintptr(fds[1]), "output"), nil
}

// AttachRequest asks to attach the user's terminal to the named terminal.
type AttachRequest struct {
	Name string
	// Env is the user's environment, which says what their terminal supports.
	Env           []string
	Width, Height int
	// Detach are the keys that end the attachment, and ToggleBar those that
	// show or hide the status bar, each ctrl and a letter or ctrl+\.
	Detach, ToggleBar []string
	// ShowBar shows the status bar at first.
	ShowBar bool
	// Peers are the sessions the status bar lists.
	Peers []session.Peer
}

// AttachResult ends an attachment: Err is empty when the user detached or the
// program exited.
type AttachResult struct{ Err string }

// attach runs one attachment on conn until it ends or the client hangs up.
func (d *daemon) attach(conn *net.UnixConn, in, out *os.File) error {
	dec := gob.NewDecoder(conn)
	var req AttachRequest
	if err := dec.Decode(&req); err != nil {
		return err
	}
	t, err := d.terminals.terminal(req.Name)
	if err == nil {
		resized := make(chan size, 1)
		hungUp := make(chan struct{})
		go func() {
			defer close(hungUp)
			for {
				var sz size
				if dec.Decode(&sz) != nil {
					return
				}
				select {
				case <-resized:
				default:
				}
				resized <- sz
			}
		}()
		b := newBar(req.ShowBar, req.Peers, req.Detach, req.ToggleBar)
		err = attach(d.terminals, t, b, in, out, req.Env, size{req.Width, req.Height}, resized, hungUp)
	}
	var result AttachResult
	if err != nil {
		result.Err = err.Error()
	}
	return gob.NewEncoder(conn).Encode(result)
}

// Service holds the calls the server answers over net/rpc, each the method
// of Server named alike.
type Service struct{ d *daemon }

// VersionReply identifies a server.
type VersionReply struct{ Protocol, PID int }

type StartArgs struct {
	Name, Dir, Program string
	Env                []string
	Width, Height      int
}

type HistoryArgs struct {
	Name  string
	Lines int
}

type ResizeArgs struct {
	Name          string
	Width, Height int
}

type KeysArgs struct {
	Name string
	Keys []string
}

type PasteArgs struct{ Name, Text string }

func (s *Service) Version(_ struct{}, reply *VersionReply) error {
	*reply = VersionReply{Protocol: protocol, PID: os.Getpid()}
	return nil
}

func (s *Service) Start(args StartArgs, _ *struct{}) error {
	err := s.d.terminals.Start(args.Name, args.Dir, args.Program, args.Env, args.Width, args.Height)
	if err == nil {
		s.d.arm()
	}
	return err
}

func (s *Service) Exists(name string, reply *bool) error {
	*reply = s.d.terminals.Exists(name)
	return nil
}

func (s *Service) Created(name string, reply *time.Time) (err error) {
	*reply, err = s.d.terminals.Created(name)
	return err
}

func (s *Service) Capture(name string, reply *string) (err error) {
	*reply, err = s.d.terminals.Capture(name)
	return err
}

func (s *Service) CaptureHistory(args HistoryArgs, reply *string) (err error) {
	*reply, err = s.d.terminals.CaptureHistory(args.Name, args.Lines)
	return err
}

func (s *Service) Resize(args ResizeArgs, _ *struct{}) error {
	return s.d.terminals.Resize(args.Name, args.Width, args.Height)
}

func (s *Service) SendKeys(args KeysArgs, _ *struct{}) error {
	return s.d.terminals.SendKeys(args.Name, args.Keys...)
}

func (s *Service) Paste(args PasteArgs, _ *struct{}) error {
	return s.d.terminals.Paste(args.Name, args.Text)
}

func (s *Service) Kill(name string, _ *struct{}) error {
	return s.d.terminals.Kill(name)
}
