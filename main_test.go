package main

import (
	"bytes"
	"errors"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"slices"
	"strings"
	"testing"

	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/session"
	"github.com/mikeryanboss/vineyard/internal/tui"
	"github.com/mikeryanboss/vineyard/internal/vterm"
)

func TestRun_CommandsAndUsageErrors(t *testing.T) {
	cases := []struct {
		args     []string
		code     int
		inStdout string
		inStderr string
	}{
		{args: []string{"version"}, code: 0, inStdout: version},
		{args: []string{"help"}, code: 0, inStdout: "USAGE"},
		{args: []string{"-h"}, code: 0, inStdout: "USAGE"},
		{args: []string{"bogus"}, code: 2, inStderr: "Unknown command: bogus"},
		{args: []string{"version", "extra"}, code: 2, inStderr: "does not accept arguments"},
		{args: []string{"--nope"}, code: 2, inStderr: "flag provided but not defined"},
		{args: []string{"--issue", "x"}, code: 2, inStderr: "invalid value"},
	}
	for _, c := range cases {
		t.Run(strings.Join(c.args, " "), func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(c.args, &stdout, &stderr)
			if code != c.code {
				t.Errorf("exit code = %d, want %d", code, c.code)
			}
			if !strings.Contains(stdout.String(), c.inStdout) || !strings.Contains(stderr.String(), c.inStderr) {
				t.Errorf("stdout %q / stderr %q missing %q / %q", stdout.String(), stderr.String(), c.inStdout, c.inStderr)
			}
		})
	}
}

// Only go install builds take their version from Go: a checkout build may be
// stamped with another worktree's commit.
func TestModuleVersion_OnlyForGoInstall(t *testing.T) {
	cases := []struct {
		name string
		info *debug.BuildInfo
		want string
	}{
		{"go install", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.5"}}, "0.1.5"},
		{"checkout", &debug.BuildInfo{
			Main:     debug.Module{Version: "v0.1.4+dirty"},
			Settings: []debug.BuildSetting{{Key: "vcs", Value: "git"}},
		}, "dev"},
		{"go run", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, "dev"},
		{"no build info", nil, "dev"},
	}
	for _, c := range cases {
		if got := moduleVersion(c.info); got != c.want {
			t.Errorf("%s: moduleVersion = %q, want %q", c.name, got, c.want)
		}
	}
}

// debug reports where a repository's data lives without creating it.
func TestRun_DebugPrintsPaths(t *testing.T) {
	repo := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"-c", "user.name=T", "-c", "user.email=t@example.com", "commit", "-q", "--allow-empty", "-m", "init"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	t.Chdir(repo)
	t.Setenv("TMPDIR", t.TempDir())

	var stdout, stderr bytes.Buffer
	if code := run([]string{"debug"}, &stdout, &stderr); code != 0 {
		t.Fatalf("debug exit code %d: %s", code, stderr.String())
	}
	for _, want := range []string{
		"not running",
		filepath.Join(repo, ".vineyard", "config.toml"),
		filepath.Join(repo, ".vineyard", "worktrees"),
		filepath.Join(repo, ".vineyard", "vineyard.log"),
	} {
		if !strings.Contains(stdout.String(), want) {
			t.Errorf("debug output lacks %s:\n%s", want, stdout.String())
		}
	}
	if _, err := os.Stat(filepath.Join(repo, ".vineyard")); !os.IsNotExist(err) {
		t.Error("debug created .vineyard")
	}
}

// Vineyard's server is always available, tmux only when it is installed.
func TestMultiplexers_WithoutTmux(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("TMPDIR", t.TempDir())
	vineyards, err := newVineyards()
	if err != nil {
		t.Fatal(err)
	}
	terminals := multiplexers(vineyards.current())
	if _, ok := terminals[config.MultiplexerVineyard].(*vterm.Client); !ok || len(terminals) != 1 {
		t.Errorf("multiplexers = %v, want Vineyard's server alone", terminals)
	}
}

// fakeServer answers Version and Exists, which servers of every protocol
// answer alike, and Capture, as one of protocol, running the terminal "a",
// would.
type fakeServer struct{ protocol int }

func (f fakeServer) Version(_ struct{}, reply *vterm.VersionReply) error {
	*reply = vterm.VersionReply{Protocol: f.protocol, PID: 4242}
	return nil
}

func (fakeServer) Exists(name string, reply *bool) error {
	*reply = name == "a"
	return nil
}

func (fakeServer) Capture(name string, reply *string) error {
	*reply = "the screen of " + name
	return nil
}

// serveFake runs a fakeServer of protocol on socket until the test ends.
func serveFake(t *testing.T, socket string, protocol int) {
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })
	calls := rpc.NewServer()
	calls.RegisterName("Terminals", fakeServer{protocol})
	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			conn.Read(make([]byte, 1)) // the connection's kind
			go calls.ServeConn(conn)
		}
	}()
}

// Each protocol's Vineyard multiplexer has a socket of its own. An older one
// whose calls match this build's serves all but attaching. One whose calls
// differ only tells whether it runs a terminal; while it does, the rest fails
// with an error saying why. One that speaks this build's protocol, as one on
// the socket protocols 1 to 3 shared may, serves fully.
func TestVineyards(t *testing.T) {
	t.Setenv("VINEYARD_SOCKET", "vy")
	dir := t.TempDir()
	v := &vineyards{dir: dir, exe: "/nonexistent", older: map[int]tui.Terminal{}}
	if got := v.client(0).Socket(); got != filepath.Join(dir, "vy") {
		t.Errorf("protocol 0's socket = %s, want %s/vy", got, dir)
	}
	serveFake(t, filepath.Join(dir, "vy-p1"), 1)
	if protocols, err := v.protocols(); err != nil || !slices.Equal(protocols, []int{vterm.Protocol(), 1}) {
		t.Errorf("protocols = %v, %v; want this build's, then 1", protocols, err)
	}

	same := v.Older(1)
	if screen, err := same.Capture("a"); err != nil || screen != "the screen of a" {
		t.Errorf("capturing on an older multiplexer whose calls match: %q, %v", screen, err)
	}
	if err := same.Attach("a", session.Attachment{}).Run(); !errors.Is(err, session.ErrOlderMultiplexer) || !strings.Contains(err.Error(), "process 4242") {
		t.Errorf("attaching there: err = %v, want ErrOlderMultiplexer naming the process", err)
	}

	// Protocol 0 predates the calls this build makes.
	serveFake(t, filepath.Join(dir, "vy-p5"), 0)
	differ := v.Older(5)
	if !differ.Exists("a") || differ.Exists("b") {
		t.Error("an older multiplexer whose calls differ does not say which terminals it runs")
	}
	if _, err := differ.Capture("a"); !errors.Is(err, session.ErrOlderMultiplexer) || !strings.Contains(err.Error(), "process 4242") {
		t.Errorf("capturing a running terminal there: err = %v, want ErrOlderMultiplexer naming the process", err)
	}
	if _, err := differ.Capture("b"); err == nil || errors.Is(err, session.ErrOlderMultiplexer) {
		t.Errorf("capturing an ended terminal there: err = %v, want one saying it is gone", err)
	}
	if differ.Kill("a") == nil || differ.Kill("b") != nil || differ.Start("c", dir, "", 80, 24) == nil {
		t.Error("it must refuse to start or kill terminals, and find ended ones gone")
	}
	if v.Older(0).Exists("a") {
		t.Error("a terminal runs on protocol 0's socket, where nothing listens")
	}

	serveFake(t, filepath.Join(dir, "vy-p7"), vterm.Protocol())
	// Attaching gets as far as needing a terminal.
	if err := v.Older(7).Attach("a", session.Attachment{}).Run(); err == nil || errors.Is(err, session.ErrOlderMultiplexer) {
		t.Errorf("attaching on a multiplexer that speaks this build's protocol: err = %v, want only that it needs a terminal", err)
	}
}
