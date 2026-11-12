package main

import (
	"bytes"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/session"
	"github.com/mikeryanboss/vineyard/internal/vterm"
)

type recordedDelivery struct {
	name string
	text string
	keys []string
}

type sendRPCServer struct {
	mu         sync.Mutex
	screens    map[string]func() (string, bool)
	deliveries []recordedDelivery
	delivered  chan struct{}
}

func newSendRPCServer(t *testing.T) *sendRPCServer {
	t.Helper()
	t.Setenv("TMPDIR", t.TempDir())
	t.Setenv("VINEYARD_SOCKET", "vy-send")
	socket, err := vterm.SocketPath(vineyardSocketName(vterm.Protocol()))
	if err != nil {
		t.Fatal(err)
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { listener.Close() })

	srv := &sendRPCServer{
		screens:   map[string]func() (string, bool){},
		delivered: make(chan struct{}, 16),
	}
	calls := rpc.NewServer()
	if err := calls.RegisterName("Terminals", srv); err != nil {
		t.Fatal(err)
	}
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
	return srv
}

func (s *sendRPCServer) setScreen(name string, fn func() (string, bool)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.screens[name] = fn
}

func (s *sendRPCServer) setStaticScreen(name, screen string) {
	s.setScreen(name, func() (string, bool) { return screen, true })
}

func (s *sendRPCServer) Version(_ struct{}, reply *vterm.VersionReply) error {
	*reply = vterm.VersionReply{Protocol: vterm.Protocol(), PID: os.Getpid()}
	return nil
}

func (s *sendRPCServer) Exists(name string, reply *bool) error {
	s.mu.Lock()
	fn := s.screens[name]
	s.mu.Unlock()
	if fn == nil {
		*reply = false
		return nil
	}
	_, *reply = fn()
	return nil
}

func (s *sendRPCServer) Capture(name string, reply *string) error {
	s.mu.Lock()
	fn := s.screens[name]
	s.mu.Unlock()
	if fn == nil {
		return fmt.Errorf("%w: %s", vterm.ErrNoTerminal, name)
	}
	screen, ok := fn()
	if !ok {
		return fmt.Errorf("%w: %s", vterm.ErrNoTerminal, name)
	}
	*reply = screen
	return nil
}

func (s *sendRPCServer) Paste(args vterm.PasteArgs, _ *struct{}) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliveries = append(s.deliveries, recordedDelivery{name: args.Name, text: args.Text})
	return nil
}

func (s *sendRPCServer) SendKeys(args vterm.KeysArgs, _ *struct{}) error {
	s.mu.Lock()
	if n := len(s.deliveries); n > 0 && s.deliveries[n-1].name == args.Name && len(s.deliveries[n-1].keys) == 0 {
		s.deliveries[n-1].keys = slices.Clone(args.Keys)
	} else {
		s.deliveries = append(s.deliveries, recordedDelivery{name: args.Name, keys: slices.Clone(args.Keys)})
	}
	s.mu.Unlock()
	select {
	case s.delivered <- struct{}{}:
	default:
	}
	return nil
}

func (s *sendRPCServer) recorded() []recordedDelivery {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.deliveries)
}

func fastSendTimers(t *testing.T) *sync.WaitGroup {
	t.Helper()
	prevStill, prevPaste, prevPoll, prevSpawn := sendStillFor, sendPasteDelay, drainPollDelay, spawnDrainer
	sendStillFor = 5 * time.Millisecond
	sendPasteDelay = 2 * time.Millisecond
	drainPollDelay = 5 * time.Millisecond
	var drainers sync.WaitGroup
	spawnDrainer = func(_, repoRoot, id string) error {
		drainers.Go(func() { _ = runDrain(repoRoot, id) })
		return nil
	}
	t.Cleanup(func() {
		drainers.Wait()
		sendStillFor, sendPasteDelay, drainPollDelay, spawnDrainer = prevStill, prevPaste, prevPoll, prevSpawn
	})
	return &drainers
}

func initSendTestRepo(t *testing.T) (repo, wtClaude, wtAgy string, store session.Store) {
	t.Helper()
	repo = t.TempDir()
	runGit := func(dir string, args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-c", "user.name=T", "-c", "user.email=t@example.com"}, args...)...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	runGit(repo, "init", "-q", "-b", "main")
	runGit(repo, "commit", "-q", "--allow-empty", "-m", "init")
	base := runGit(repo, "rev-parse", "HEAD")

	wtClaude = filepath.Join(repo, ".vineyard", "worktrees", "fix-111111")
	wtAgy = filepath.Join(repo, ".vineyard", "worktrees", "coms-222222")
	runGit(repo, "worktree", "add", "-q", "-b", "7/fix", wtClaude)
	runGit(repo, "worktree", "add", "-q", "-b", "8/coms", wtAgy)

	store = session.NewStore(config.Dir(repo))
	for _, s := range []session.Session{
		{
			ID: "fix-111111", Title: "fix", Program: "claude",
			Branch: "7/fix", BaseCommit: base,
			TmuxName: "term-claude", Multiplexer: config.MultiplexerVineyard, Protocol: vterm.Protocol(),
			Status: session.StatusRunning,
		},
		{
			ID: "coms-222222", Title: "coms", Program: "agy",
			Branch: "8/coms", BaseCommit: base,
			TmuxName: "term-agy", Multiplexer: config.MultiplexerVineyard, Protocol: vterm.Protocol(),
			Status: session.StatusRunning,
		},
	} {
		if err := store.Add(s); err != nil {
			t.Fatal(err)
		}
	}
	return repo, wtClaude, wtAgy, store
}

func TestRun_SendDeliversAcrossHarnessesWhenIdle(t *testing.T) {
	srv := newSendRPCServer(t)
	fastSendTimers(t)
	_, wtClaude, wtAgy, _ := initSendTestRepo(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())

	srv.setStaticScreen("term-claude", "❯ ")
	srv.setStaticScreen("term-agy", "> Type a message")

	// Send from a subdirectory of the agy worktree to the claude session by branch,
	// with a message starting with '-' to verify flag parsing leaves it intact.
	subdir := filepath.Join(wtAgy, "internal", "sub")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Chdir(subdir)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"send", "7/fix", "- API change in internal/session is ready."}, &stdout, &stderr); code != 0 {
		t.Fatalf("send exit %d: %s", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "Delivered to 7/fix" {
		t.Errorf("stdout = %q, want %q", stdout.String(), "Delivered to 7/fix\n")
	}

	// Reply from the claude worktree to the agy session by session ID.
	claudeDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	if err := os.Mkdir(filepath.Join(claudeDir, "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}
	entry := fmt.Sprintf(`{"pid": %d, "cwd": %q, "name": "7/fix"}`, os.Getpid(), wtClaude)
	if err := os.WriteFile(filepath.Join(claudeDir, "sessions", "1.json"), []byte(entry), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(wtClaude)
	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"send", "coms-222222", "Tests pass on my side."}, &stdout, &stderr); code != 0 {
		t.Fatalf("reply exit %d: %s", code, stderr.String())
	}

	got := srv.recorded()
	want := []recordedDelivery{
		{
			name: "term-claude",
			text: "<peer_message from=\"8/coms\" reply=\"vineyard send '8/coms' '...'\">\n- API change in internal/session is ready.\n</peer_message>",
			keys: []string{"Enter"},
		},
		{
			name: "term-agy",
			text: "<peer_message from=\"7/fix\" reply=\"vineyard send '7/fix' '...'\">\nTests pass on my side.\n</peer_message>",
			keys: []string{"Enter"},
		},
	}
	if !slices.EqualFunc(got, want, func(a, b recordedDelivery) bool {
		return a.name == b.name && a.text == b.text && slices.Equal(a.keys, b.keys)
	}) {
		t.Errorf("deliveries =\n%+v\nwant\n%+v", got, want)
	}
}

func TestRun_SendRefusesInvalidCallsAndClaudeToClaude(t *testing.T) {
	srv := newSendRPCServer(t)
	fastSendTimers(t)
	repo, wtClaude, wtAgy, store := initSendTestRepo(t)
	srv.setStaticScreen("term-claude", "❯ ")
	srv.setStaticScreen("term-agy", "> ")

	// Add a paused session and two main-checkout sessions sharing "main".
	for _, s := range []session.Session{
		{ID: "paused-333333", Title: "paused", Branch: "9/paused", TmuxName: "term-paused", Multiplexer: config.MultiplexerVineyard, Protocol: vterm.Protocol(), Status: session.StatusPaused},
		{ID: "dead-444444", Title: "dead", Branch: "10/dead", TmuxName: "term-dead", Multiplexer: config.MultiplexerVineyard, Protocol: vterm.Protocol(), Status: session.StatusRunning},
		{ID: "m1-555555", Title: "m1", MainCheckout: true, TmuxName: "term-m1", Multiplexer: config.MultiplexerVineyard, Protocol: vterm.Protocol(), Status: session.StatusRunning},
		{ID: "m2-666666", Title: "m2", MainCheckout: true, TmuxName: "term-m2", Multiplexer: config.MultiplexerVineyard, Protocol: vterm.Protocol(), Status: session.StatusRunning},
	} {
		if err := store.Add(s); err != nil {
			t.Fatal(err)
		}
	}
	// Give both main-checkout sessions the "main" branch in saved state to test ambiguity.
	_, err := store.Update(func(saved []session.Session) ([]session.Session, error) {
		for i := range saved {
			if saved[i].MainCheckout {
				saved[i].Branch = "main"
			}
		}
		return saved, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	claudeDir := t.TempDir()
	t.Setenv("CLAUDE_CONFIG_DIR", claudeDir)
	if err := os.Mkdir(filepath.Join(claudeDir, "sessions"), 0o755); err != nil {
		t.Fatal(err)
	}

	t.Chdir(wtAgy)
	cases := []struct {
		name    string
		dir     string
		to      string
		msg     string
		wantErr string
	}{
		{"empty", wtAgy, "7/fix", "   \n\t ", "message is empty"},
		{"oversized", wtAgy, "7/fix", strings.Repeat("x", maxMessageBytes+1), "exceeds"},
		{"missing", wtAgy, "99/missing", "hi", `no session "99/missing"`},
		{"ambiguous branch", repo, "main", "hi", `several sessions are on branch "main"`},
		{"self by branch", wtAgy, "8/coms", "hi", `cannot send a message to "8/coms" from itself`},
		{"self by id", wtAgy, "coms-222222", "hi", `cannot send a message to "8/coms" from itself`},
		{"paused", wtAgy, "9/paused", "hi", `session "9/paused" is not running`},
		{"stopped terminal", wtAgy, "10/dead", "hi", `session "10/dead" is not running`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Chdir(c.dir)
			var stdout, stderr bytes.Buffer
			code := run([]string{"send", c.to, c.msg}, &stdout, &stderr)
			if code != 1 || !strings.Contains(stderr.String(), c.wantErr) {
				t.Errorf("code=%d stderr=%q, want code=1 containing %q", code, stderr.String(), c.wantErr)
			}
		})
	}

	// Register live Claude Code agents in both worktrees: vineyard send must
	// refuse and direct the caller to Claude Code's native SendMessage.
	for file, cwd := range map[string]string{"1.json": wtClaude, "2.json": wtAgy} {
		entry := fmt.Sprintf(`{"pid": %d, "cwd": %q, "name": %q}`, os.Getpid(), cwd, filepath.Base(cwd))
		if err := os.WriteFile(filepath.Join(claudeDir, "sessions", file), []byte(entry), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(wtAgy)
	var stdout, stderr bytes.Buffer
	code := run([]string{"send", "7/fix", "use native"}, &stdout, &stderr)
	if code != 1 || !strings.Contains(stderr.String(), `SendMessage`) || !strings.Contains(stderr.String(), `fix-111111`) {
		t.Fatalf("claude-to-claude: code=%d stderr=%q, want SendMessage naming fix-111111", code, stderr.String())
	}
	if len(srv.recorded()) != 0 {
		t.Errorf("unexpected deliveries after refused sends: %+v", srv.recorded())
	}
}

func TestRun_SendQueuesWhileBusyAndDeliversInOrderOnceReady(t *testing.T) {
	srv := newSendRPCServer(t)
	drainers := fastSendTimers(t)
	_, _, wtAgy, store := initSendTestRepo(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())

	// Start target blocked by PendingPrompt, then walk through every screen
	// state where pasting must wait before finally settling on the main prompt.
	if err := store.Replace(session.Session{
		ID: "fix-111111", Title: "fix", Program: "claude",
		Branch: "7/fix", PendingPrompt: "initial task",
		TmuxName: "term-claude", Multiplexer: config.MultiplexerVineyard, Protocol: vterm.Protocol(),
		Status: session.StatusRunning,
	}); err != nil {
		t.Fatal(err)
	}

	var mu sync.Mutex
	phase := "blank"
	tick := 0
	srv.setScreen("term-claude", func() (string, bool) {
		mu.Lock()
		defer mu.Unlock()
		switch phase {
		case "blank":
			return "   \n  ", true
		case "spinning":
			tick++
			return fmt.Sprintf("Working %d…", tick), true
		case "trust":
			return "Do you trust the files in this folder?", true
		case "permission":
			return "Allow tool?\nNo, and tell Claude what to do differently", true
		case "subagent-focus":
			return "─ 7/fix ─\n❯ \n❯ ● main\n  ◯ general-purpose  Exploring  2s", true
		case "subagent-view":
			return "─ Exploring ─\n❯ \n  ◯ main\n  ● general-purpose  Exploring  2s", true
		default:
			return "─ 7/fix ─\n❯ \n  ● main\n  ◯ general-purpose  Exploring  2s", true
		}
	})

	t.Chdir(wtAgy)
	for _, msg := range []string{"first note", "second note"} {
		var stdout, stderr bytes.Buffer
		if code := run([]string{"send", "7/fix", msg}, &stdout, &stderr); code != 0 {
			t.Fatalf("send(%q) exit %d: %s", msg, code, stderr.String())
		}
		if strings.TrimSpace(stdout.String()) != "Queued for 7/fix" {
			t.Errorf("stdout = %q, want Queued for 7/fix", stdout.String())
		}
	}

	// Clear PendingPrompt and step through each guarded screen state; none may trigger delivery.
	if err := store.Replace(session.Session{
		ID: "fix-111111", Title: "fix", Program: "claude",
		Branch: "7/fix", PendingPrompt: "",
		TmuxName: "term-claude", Multiplexer: config.MultiplexerVineyard, Protocol: vterm.Protocol(),
		Status: session.StatusRunning,
	}); err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"blank", "spinning", "trust", "permission", "subagent-focus", "subagent-view"} {
		mu.Lock()
		phase = p
		mu.Unlock()
		time.Sleep(15 * time.Millisecond)
		if got := srv.recorded(); len(got) != 0 {
			t.Fatalf("delivered during guarded phase %q: %+v", p, got)
		}
	}

	// Settle on the main conversation with unfocused subagent list: both queued
	// messages must arrive together in FIFO order.
	mu.Lock()
	phase = "ready"
	mu.Unlock()

	select {
	case <-srv.delivered:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for queued messages to deliver")
	}
	drainers.Wait()

	got := srv.recorded()
	if len(got) != 1 {
		t.Fatalf("deliveries = %+v, want 1 batched delivery", got)
	}
	wantText := "<peer_message from=\"8/coms\" reply=\"vineyard send '8/coms' '...'\">\nfirst note\n</peer_message>\n\n" +
		"<peer_message from=\"8/coms\" reply=\"vineyard send '8/coms' '...'\">\nsecond note\n</peer_message>"
	if got[0].text != wantText || !slices.Equal(got[0].keys, []string{"Enter"}) {
		t.Errorf("delivery = %+v, want text=%q keys=[Enter]", got[0], wantText)
	}
}

func TestRun_PeerMessagesDisabledMarksHelpAndStopsSendAndDrain(t *testing.T) {
	srv := newSendRPCServer(t)
	drainers := fastSendTimers(t)
	repo, _, wtAgy, store := initSendTestRepo(t)
	t.Setenv("CLAUDE_CONFIG_DIR", t.TempDir())

	if err := store.Replace(session.Session{
		ID: "fix-111111", Title: "fix", Program: "claude",
		Branch: "7/fix", PendingPrompt: "busy",
		TmuxName: "term-claude", Multiplexer: config.MultiplexerVineyard, Protocol: vterm.Protocol(),
		Status: session.StatusRunning,
	}); err != nil {
		t.Fatal(err)
	}
	srv.setStaticScreen("term-claude", "❯ ")

	// Default config: help shows send without a disabled note, and busy send queues.
	t.Chdir(wtAgy)
	var stdout, stderr bytes.Buffer
	if code := run([]string{"help"}, &stdout, &stderr); code != 0 {
		t.Fatalf("help exit %d: %s", code, stderr.String())
	}
	if strings.Contains(stdout.String(), "disabled in config.toml") {
		t.Fatalf("default help unexpectedly marked send disabled:\n%s", stdout.String())
	}
	stdout.Reset()
	if code := run([]string{"send", "7/fix", "queued before disable"}, &stdout, &stderr); code != 0 {
		t.Fatalf("send before disable exit %d: %s", code, stderr.String())
	}

	// Disable peer_messages in the main checkout's .vineyard/config.toml:
	// help in a linked worktree marks send disabled, send is refused, and the
	// background drainer discards the already-queued message without delivering it.
	cfg, err := config.Load(config.Dir(repo))
	if err != nil {
		t.Fatal(err)
	}
	cfg.PeerMessages = false
	if err := config.Save(config.Dir(repo), cfg); err != nil {
		t.Fatal(err)
	}
	drainers.Wait()

	for _, args := range [][]string{{"help"}, {"-h"}} {
		stdout.Reset()
		stderr.Reset()
		if code := run(args, &stdout, &stderr); code != 0 {
			t.Fatalf("%v exit %d: %s", args, code, stderr.String())
		}
		if !strings.Contains(stdout.String(), "Send a peer message to another running session (disabled in config.toml)") {
			t.Errorf("%v output missing disabled marker:\n%s", args, stdout.String())
		}
	}

	stdout.Reset()
	stderr.Reset()
	if code := run([]string{"send", "7/fix", "blocked"}, &stdout, &stderr); code != 1 || !strings.Contains(stderr.String(), "peer messages are disabled in .vineyard/config.toml") {
		t.Errorf("disabled send: code=%d stderr=%q, want disabled error", code, stderr.String())
	}
	if got := srv.recorded(); len(got) != 0 {
		t.Errorf("unexpected deliveries when peer_messages = false: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(config.Dir(repo), "inbox", "fix-111111.json")); !os.IsNotExist(err) {
		t.Errorf("queued inbox file remained after disabling peer_messages: err=%v", err)
	}
}
