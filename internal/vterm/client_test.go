package vterm

import (
	"bytes"
	"fmt"
	"io"
	"net"
	"net/rpc"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/charmbracelet/x/ansi"
	"github.com/creack/pty"

	"github.com/mikeryanboss/vineyard/internal/session"
)

// TestMain runs a server instead of the tests when a client of serverSocket
// starts this binary as its server.
func TestMain(m *testing.M) {
	if socket := os.Getenv("VINEYARD_TEST_SERVE"); socket != "" {
		if err := Serve(socket); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// serverSocket returns a socket for the test, and the command clients start
// a server on it with: this test binary, in a process of its own. Its
// directory is removed once the servers are gone, which may write there
// after the test ends: one that lost a race to start exits on its own time.
func serverSocket(t *testing.T) (string, []string) {
	dir, err := os.MkdirTemp("", "vterm")
	if err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(dir, "s")
	t.Setenv("VINEYARD_TEST_SERVE", socket)
	t.Setenv("SHELL", "/bin/sh")
	t.Cleanup(func() {
		if log, _ := os.ReadFile(socket + ".log"); t.Failed() && len(log) > 0 {
			t.Logf("server log:\n%s", log)
		}
		waitForExit(t, socket)
		for deadline := time.Now().Add(time.Second); ; time.Sleep(10 * time.Millisecond) {
			err := os.RemoveAll(dir)
			if err == nil {
				return
			}
			if time.Now().After(deadline) {
				t.Error(err)
				return
			}
		}
	})
	return socket, []string{os.Args[0]}
}

// startVia runs program on a new terminal through c and ends it with the test.
func startVia(t *testing.T, c *Client, name, program string) {
	t.Helper()
	if err := c.Start(name, t.TempDir(), program, 50, 12); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { c.Kill(name) })
}

// waitForExit waits for the server at socket to stop, which removes its socket.
func waitForExit(t *testing.T, socket string) {
	t.Helper()
	for deadline := time.Now().Add(3 * time.Second); ; time.Sleep(10 * time.Millisecond) {
		if _, err := os.Stat(socket); os.IsNotExist(err) {
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("the server still runs")
		}
	}
}

// An agent keeps running after the Vineyard that started it quits, and the
// next Vineyard finds it. The server stops once its last terminal ends and no
// Vineyard is connected.
func TestAgentsOutliveTheVineyardThatStartedThem(t *testing.T) {
	socket, server := serverSocket(t)
	first := NewClient(socket, server...)
	if first.Exists("a") {
		t.Fatal("a terminal exists before any server runs")
	}
	if err := first.Start("a", t.TempDir(), `echo started; sleep 30`, 50, 12); err != nil {
		t.Fatal(err)
	}
	first.Close()

	second := NewClient(socket, server...)
	if !second.Exists("a") {
		t.Fatal("the terminal ended with the client that started it")
	}
	waitFor(t, func() (string, error) { return second.Capture("a") }, "started")
	if err := second.Kill("a"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(socket); err != nil {
		t.Fatalf("the server stopped while a client was connected: %v", err)
	}
	second.Close()
	waitForExit(t, socket)
	if _, err := second.Capture("a"); err == nil {
		t.Error("capture without a server succeeded")
	}
	if err := second.Kill("a"); err != nil {
		t.Errorf("killing a terminal without a server: %v", err)
	}
}

// Vineyards that start agents at once start one server between them, and
// replace the socket a crashed server left behind.
func TestRacingVineyardsShareOneServer(t *testing.T) {
	socket, server := serverSocket(t)
	stale, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	stale.SetUnlinkOnClose(false)
	stale.Close()

	clients := make([]*Client, 4)
	for i := range clients {
		clients[i] = NewClient(socket, server...)
	}
	t.Cleanup(func() {
		for _, c := range clients {
			c.Close()
		}
	})
	var wg sync.WaitGroup
	for i, c := range clients {
		wg.Go(func() { startVia(t, c, fmt.Sprint(i), `sleep 30`) })
	}
	wg.Wait()
	pids := map[int]bool{}
	for i, c := range clients {
		var v VersionReply
		if err := c.call(false, "Version", struct{}{}, &v); err != nil {
			t.Fatal(err)
		}
		pids[v.PID] = true
		if !clients[0].Exists(fmt.Sprint(i)) {
			t.Errorf("terminal %d is not on the shared server", i)
		}
	}
	if len(pids) != 1 {
		t.Errorf("clients reached %d servers, want 1", len(pids))
	}
}

// fakeServer answers Version, and nothing else, as a server of protocol
// version 0 would.
type fakeServer struct{}

func (fakeServer) Version(_ struct{}, reply *VersionReply) error {
	*reply = VersionReply{Protocol: 0, PID: 4242}
	return nil
}

func TestServerOfAnotherVersionIsAnError(t *testing.T) {
	socket := filepath.Join(t.TempDir(), "s")
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	calls := rpc.NewServer()
	calls.RegisterName("Terminals", fakeServer{})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Read(make([]byte, 1))
			go calls.ServeConn(conn)
		}
	}()
	err = NewClient(socket).Start("a", t.TempDir(), "", 50, 12)
	if err == nil || !strings.Contains(err.Error(), "process 4242") {
		t.Errorf("err = %v, want one naming the server's process", err)
	}
}

// Attaching through the server draws the agent's screen on the user's
// terminal with the status bar on its last row, passes typed keys to it,
// follows the user's terminal's size less the bar, hides the bar on its key,
// and returns on the detach key, here ctrl-g, without passing either key on.
func TestAttachThroughTheServer(t *testing.T) {
	socket, server := serverSocket(t)
	c := NewClient(socket, server...)
	t.Cleanup(func() { c.Close() })
	startVia(t, c, "a", "")
	startVia(t, c, "b", "echo peer; sleep 10")
	capture := func() (string, error) { return c.Capture("a") }
	waitFor(t, capture, "$")

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

	a := c.Attach("a", session.Attachment{
		Detach: []string{"ctrl+g"}, ToggleBar: []string{"ctrl+\\"}, ShowBar: true,
		Peers: []session.Peer{{Terminal: "b", Title: "Peer session"}},
	})
	a.SetStdin(tty)
	a.SetStdout(tty)
	done := make(chan error)
	go func() { done <- a.Run() }()
	// size waits for the agent's terminal to be width by height.
	size := func(height, width int) {
		t.Helper()
		want := fmt.Sprintf("%d %d", height, width)
		for deadline := time.Now().Add(2 * time.Second); ; {
			user.Write([]byte("stty size\r"))
			got, err := c.Capture("a")
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(got, want) {
				return
			}
			if time.Now().After(deadline) {
				t.Fatalf("the terminal did not become %s:\n%s", want, got)
			}
			time.Sleep(50 * time.Millisecond)
		}
	}

	user.Write([]byte("\x11echo typed\r")) // ctrl-q, the default detach key, does not detach
	waitFor(t, capture, "typed")
	size(11, 50)
	if err := pty.Setsize(tty, &pty.Winsize{Cols: 60, Rows: 15}); err != nil {
		t.Fatal(err)
	}
	syscall.Kill(os.Getpid(), syscall.SIGWINCH)
	size(14, 60)
	user.Write([]byte("\x1c"))
	size(15, 60)

	user.Write([]byte("echo more\r\x07echo ignored\r"))
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("ctrl-g did not detach")
	}
	got := waitFor(t, capture, "more")
	if strings.Contains(got, "ignored") || strings.Contains(got, "^G") || strings.Contains(got, "^\\") {
		t.Errorf("a detach or status bar key reached the agent: %q", got)
	}
	tty.Close()
	user.Close()
	<-copied
	screen := ansi.Strip(drawn.String())
	for _, want := range []string{"typed", "● Peer session", "ctrl+g back  ctrl+\\ hide"} {
		if !strings.Contains(screen, want) {
			t.Errorf("attach did not draw %q: %q", want, screen)
		}
	}
}
