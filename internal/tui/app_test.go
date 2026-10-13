package tui

import (
	"errors"
	"fmt"
	"image/color"
	"os/exec"
	"reflect"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/git"
	"github.com/mikeryanboss/vineyard/internal/recap"
	"github.com/mikeryanboss/vineyard/internal/session"
	"github.com/mikeryanboss/vineyard/internal/subagent"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
	"github.com/mikeryanboss/vineyard/internal/tui/testutil"
)

// fakeBackend records what the model asks of the outside world.
type fakeBackend struct {
	mu     sync.Mutex
	screen string
	// captureErr is the error of every capture of an agent's screen.
	captureErr error
	diff       string
	startErr   error
	file       []session.Session   // sessions.json, which other Vineyards share
	saved      [][]session.Session // the file after each write
	follower   bool                // another Vineyard leads
	pasted     []string
	enters     []string
	killed     []string
	lost       map[string][]string // session ID -> commits only on its detached HEAD
	branches   map[string]string   // session ID -> checked-out branch
	resized    [][2]int
	configs    []config.Config
	recaps     map[string]recap.Recap // by session ID
	attaches   int
	tools      []*exec.Cmd // diff tools built
	opened     [][2]int    // position and total of each subagent opened
	openErr    error
	// conversation is what every Claude session's transcripts say, and
	// events each subagent's transcript, by ID.
	conversation subagent.Conversation
	events       map[string][]subagent.Event
	// trees maps a directory to the worktree holding it, and treeDiffs a
	// worktree to its diff.
	trees, treeDiffs map[string]string
	// templates are the template files; Templates adds the inline ones.
	templates []config.Template
	// shellScreen is every shell's screen, shells the sessions whose shell
	// was started, and shellAttaches counts attaches to a shell.
	shellScreen   string
	shells        []string
	shellAttaches int
	// attached is how the last attach, to an agent or a shell, attached.
	attached session.Attachment
	// polls counts status checks.
	polls int
}

func (f *fakeBackend) New(opts session.NewOptions) session.Session {
	return session.Session{
		ID: session.Slug(opts.Title, "s"), Title: opts.Title, Program: opts.Program,
		Branch:        opts.BranchPrefix + session.Slug(opts.Title, "s"),
		PendingPrompt: opts.Prompt, AutoYes: opts.AutoYes, Status: session.StatusLoading,
		Issue: opts.Issue, WorktreePath: opts.WorktreeDir + "/" + session.Slug(opts.Title, "s"),
		MainCheckout: opts.MainCheckout,
	}
}

func (f *fakeBackend) Start(s session.Session, w, h int) (session.Session, error) {
	if f.startErr != nil {
		return s, f.startErr
	}
	s.Status = session.StatusRunning
	return s, nil
}

func (f *fakeBackend) Pause(s session.Session) (session.Session, error) {
	s.Status = session.StatusPaused
	return s, nil
}

func (f *fakeBackend) Resume(s session.Session, w, h int) (session.Session, error) {
	s.Status = session.StatusRunning
	return s, nil
}

func (f *fakeBackend) Kill(s session.Session, discardCommits bool) (session.KillResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if commits := f.lost[s.ID]; len(commits) > 0 && !discardCommits {
		return session.KillResult{}, &session.LostCommitsError{Title: s.Title, Commits: commits}
	}
	f.killed = append(f.killed, s.ID)
	return session.KillResult{KeptBranch: s.Branch}, nil
}

func (f *fakeBackend) Push(s session.Session) (session.Session, error) { return s, nil }
func (f *fakeBackend) EnsureShell(s session.Session, w, h int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if !slices.Contains(f.shells, s.ID) {
		f.shells = append(f.shells, s.ID)
	}
	return nil
}
func (f *fakeBackend) Capture(s session.Session) (string, error)               { return f.screen, f.captureErr }
func (f *fakeBackend) CaptureHistory(s session.Session, n int) (string, error) { return f.screen, nil }
func (f *fakeBackend) AttachCommand(s session.Session, how session.Attachment) tea.ExecCommand {
	f.attaches++
	f.attached = how
	return nil
}

func (f *fakeBackend) DiffTool(s session.Session, command string) (*exec.Cmd, error) {
	cmd, err := diffTool(s.WorktreePath, s.BaseCommit, command)
	if err == nil {
		f.mu.Lock()
		f.tools = append(f.tools, cmd)
		f.mu.Unlock()
	}
	return cmd, err
}

func (f *fakeBackend) ShellAttachCommand(s session.Session, how session.Attachment) tea.ExecCommand {
	f.shellAttaches++
	f.attached = how
	return nil
}
func (f *fakeBackend) CaptureShell(s session.Session) (string, error) { return f.shellScreen, nil }
func (f *fakeBackend) CaptureShellHistory(s session.Session, n int) (string, error) {
	return f.shellScreen, nil
}
func (f *fakeBackend) ResizeShell(s session.Session, w, h int) error { return nil }

func (f *fakeBackend) OpenSubagent(s session.Session, position, total int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.opened = append(f.opened, [2]int{position, total})
	return f.openErr
}
func (f *fakeBackend) Subagents(s session.Session) (subagent.Conversation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.conversation, nil
}
func (f *fakeBackend) SubagentEvents(s session.Session, id string) ([]subagent.Event, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.events[id], nil
}
func (f *fakeBackend) Worktree(dir string) string { return f.trees[dir] }
func (f *fakeBackend) TreeDiff(root string, s session.Session) (string, error) {
	return f.treeDiffs[root], nil
}
func (f *fakeBackend) TreeDiffTool(root string, s session.Session, command string) (*exec.Cmd, error) {
	cmd, err := diffTool(root, "base", command)
	if err == nil {
		f.mu.Lock()
		f.tools = append(f.tools, cmd)
		f.mu.Unlock()
	}
	return cmd, err
}
func (f *fakeBackend) Diff(s session.Session) (string, error)       { return f.diff, nil }
func (f *fakeBackend) DiffStat(s session.Session) (git.Stat, error) { return git.Stat{Added: 3}, nil }

func (f *fakeBackend) Branch(s session.Session) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if branch, ok := f.branches[s.ID]; ok {
		return branch, nil
	}
	return s.Branch, nil
}

func (f *fakeBackend) Recap(s session.Session) (recap.Recap, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.recaps[s.ID], nil
}

func (f *fakeBackend) Resize(s session.Session, w, h int) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resized = append(f.resized, [2]int{w, h})
	return nil
}

func (f *fakeBackend) SendEnter(s session.Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.enters = append(f.enters, s.ID)
	return nil
}

func (f *fakeBackend) Paste(s session.Session, text string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pasted = append(f.pasted, text)
	return nil
}

// write changes the file as the store does, under its lock.
func (f *fakeBackend) write(change func([]session.Session) []session.Session) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.file = change(f.file)
	f.saved = append(f.saved, slices.Clone(f.file))
	return nil
}

func (f *fakeBackend) Add(s session.Session) error {
	return f.write(func(file []session.Session) []session.Session { return append(file, s) })
}

func (f *fakeBackend) Replace(s session.Session) error {
	return f.write(func(file []session.Session) []session.Session {
		if i := slices.IndexFunc(file, func(o session.Session) bool { return o.ID == s.ID }); i >= 0 {
			file[i] = s
		}
		return file
	})
}

func (f *fakeBackend) Remove(id string) error {
	return f.write(func(file []session.Session) []session.Session {
		return slices.DeleteFunc(file, func(o session.Session) bool { return o.ID == id })
	})
}

func (f *fakeBackend) Load(string) ([]session.Session, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.file), nil
}

func (f *fakeBackend) Lead() (bool, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.polls++
	return !f.follower, nil
}

func (f *fakeBackend) SaveConfig(cfg config.Config) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.configs = append(f.configs, cfg)
	return nil
}

func (f *fakeBackend) Templates(inline []config.Template) ([]config.Template, error) {
	return append(slices.Clone(f.templates), inline...), nil
}

const sampleDiff = `diff --git a/auth.go b/auth.go
--- a/auth.go
+++ b/auth.go
@@ -1,3 +1,3 @@ package auth
 package auth
-var timeout = 30
+var timeout = 60
 // end
`

func newTestModel(t *testing.T, sessions []session.Session) (Model, *fakeBackend) {
	t.Helper()
	// Timers never fire in tests; each test sends tick messages itself.
	tick = func(time.Duration, func(time.Time) tea.Msg) tea.Cmd { return nil }
	backend := &fakeBackend{screen: "✻ Thinking…\n\n> fix the redirect", diff: sampleDiff, file: slices.Clone(sessions)}
	m := NewModel(backend, sessions, Options{
		Config:     config.Config{DefaultProgram: "claude", BranchPrefix: "test/", WorktreeDir: ".vineyard/worktrees", Multiplexer: config.MultiplexerVineyard},
		RepoName:   "shop",
		RepoRoot:   "/src/shop",
		Version:    "0.1.0",
		ConfigPath: "~/src/shop/.vineyard/config.toml",
	})
	return update(m, tea.WindowSizeMsg{Width: 110, Height: 26}), backend
}

// update applies one message and discards the resulting commands.
func update(m Model, msg tea.Msg) Model {
	next, _ := m.Update(msg)
	return next.(Model)
}

// send applies a message, then runs the commands it returns and feeds their
// messages back in, until things settle. Commands still running after a short
// wait, such as ticks, are dropped, which also ends the polling loops.
func send(m Model, msg tea.Msg) Model {
	queue := []tea.Msg{msg}
	for steps := 0; len(queue) > 0 && steps < 100; steps++ {
		next, cmd := m.Update(queue[0])
		m = next.(Model)
		queue = append(queue[1:], collect(cmd)...)
	}
	return m
}

// lastPoll is when the last status check that tests send captured its screens.
var lastPoll time.Time

// polled returns a status check a statusInterval after the one before, as
// the ticks pace them.
func polled(lead bool, results ...screenStatus) statusMsg {
	lastPoll = lastPoll.Add(statusInterval)
	return statusMsg{lead: lead, at: lastPoll, results: results}
}

// collect runs cmd, flattening batches, and returns the messages produced
// within a short deadline.
func collect(cmd tea.Cmd) []tea.Msg {
	if cmd == nil {
		return nil
	}
	results := make(chan tea.Msg, 1)
	go func() { results <- cmd() }()
	select {
	case msg := <-results:
		switch msg := msg.(type) {
		case nil:
			return nil
		case tea.BatchMsg:
			var out []tea.Msg
			for _, c := range msg {
				out = append(out, collect(c)...)
			}
			return out
		case previewTickMsg, statusTickMsg, diffTickMsg, clearStatusMsg:
			return nil // polling and timers are driven explicitly in tests
		default:
			return []tea.Msg{msg}
		}
	case <-time.After(300 * time.Millisecond):
		return nil
	}
}

func keys(m Model, presses ...string) Model {
	for _, k := range presses {
		m = send(m, testutil.Key(k))
	}
	return m
}

// typeText types into the open dialog. Typing only yields cursor-blink
// commands, so they are dropped instead of waited for.
func typeText(m Model, text string) Model {
	for _, r := range text {
		m = update(m, testutil.Key(string(r)))
	}
	return m
}

func statusOf(m Model, id string) session.Status {
	if i := m.find(id); i >= 0 {
		return m.sessions[i].Status
	}
	return ""
}

func TestAppView_Main(t *testing.T) {
	m, _ := newTestModel(t, testutil.Sessions())
	m = send(m, previewTickMsg{})
	m = send(m, diffTickMsg{})
	testutil.RequireGolden(t, m.View().Content)
}

func TestAppView_DiffTab(t *testing.T) {
	m, _ := newTestModel(t, testutil.Sessions())
	m = send(m, diffTickMsg{})
	m = keys(m, "tab", "l")
	testutil.RequireGolden(t, m.View().Content)
}

func TestAppView_PausedSessionDiffTab(t *testing.T) {
	m, _ := newTestModel(t, testutil.Sessions())
	m = keys(m, "j", "j", "tab") // docs-3 is paused
	m = send(m, diffTickMsg{})
	testutil.RequireGolden(t, m.View().Content)
}

func TestAppView_NewSessionDialog(t *testing.T) {
	m, _ := newTestModel(t, testutil.Sessions())
	m = keys(m, "N")
	testutil.RequireGolden(t, m.View().Content)
}

func TestAppView_Empty(t *testing.T) {
	m, _ := newTestModel(t, nil)
	testutil.RequireGolden(t, m.View().Content)
}

func TestApp_CreateSession(t *testing.T) {
	m, backend := newTestModel(t, nil)
	m = keys(m, "n")
	m = typeText(m, "Fix login")
	m = keys(m, "enter")

	if m.dialog != nil {
		t.Fatal("dialog should close after creating")
	}
	s, ok := m.selected()
	if !ok || s.Title != "Fix login" || s.Status != session.StatusRunning || s.Branch != "test/fix-login" {
		t.Fatalf("selected = %+v, want the new running session", s)
	}
	if len(backend.saved) == 0 || len(backend.saved[len(backend.saved)-1]) != 1 {
		t.Error("the started session should be saved")
	}
	if len(backend.resized) == 0 {
		t.Error("the new session's window should be sized to the preview")
	}
}

// N starts the chosen profile. A main-checkout profile's session is never
// committed, checked out, or pushed: the work in the main checkout is not
// the session's own.
func TestApp_NewFromProfileStartsAMainCheckoutTerminal(t *testing.T) {
	m, _ := newTestModel(t, nil)
	m = keys(m, "N", "right", "enter")
	m = typeText(m, "dev server")
	m = keys(m, "enter")
	s, ok := m.selected()
	if !ok || !s.MainCheckout || s.Program != "$SHELL" || s.PendingPrompt != "" {
		t.Fatalf("selected = %+v, want the terminal profile's session in the main checkout", s)
	}

	for _, k := range []string{"c", "s"} {
		m.status = ""
		m = keys(m, k)
		if m.dialog != nil || !strings.Contains(m.status, "main checkout") {
			t.Errorf("%s: dialog = %T, status = %q; want a refusal naming the main checkout", k, m.dialog, m.status)
		}
	}
	m = keys(m, "D")
	if view := screen(m); !strings.Contains(view, "left as it is") {
		t.Errorf("kill should say the main checkout stays:\n%s", view)
	}
}

// T starts the terminal profile with no dialog and attaches once it runs: a
// failed start attaches to nothing, and a disabled profile starts nothing.
func TestApp_TerminalKeyStartsAndAttachesATerminal(t *testing.T) {
	m, backend := newTestModel(t, nil)
	m = keys(m, "T")
	s, ok := m.selected()
	if m.dialog != nil || !ok || s.Title != "terminal" || !s.MainCheckout || s.Program != "$SHELL" || s.Status != session.StatusRunning {
		t.Fatalf("dialog = %T, selected = %+v; want the terminal profile's session running, no dialog", m.dialog, s)
	}
	if backend.attaches != 1 {
		t.Errorf("attaches = %d, want 1 once the terminal runs", backend.attaches)
	}

	m, backend = newTestModel(t, nil)
	backend.startErr = errors.New("zsh: not found")
	m = keys(m, "T")
	if len(m.sessions) != 0 || backend.attaches != 0 || !m.statusIsErr {
		t.Errorf("sessions = %d, attaches = %d, status = %q; want nothing started or attached, and the error", len(m.sessions), backend.attaches, m.status)
	}

	m, backend = newTestModel(t, nil)
	m.opts.Config.DisabledProfiles = []string{"terminal"}
	m = keys(m, "T")
	if len(m.sessions) != 0 || backend.attaches != 0 || !strings.Contains(m.status, "disabled") {
		t.Errorf("sessions = %d, attaches = %d, status = %q; want nothing started and the reason", len(m.sessions), backend.attaches, m.status)
	}
}

// worktree_dir is read when a session is created, so a value saved on the
// config screen applies to the next session. Empty would mean the repository
// root itself, so it is refused.
func TestApp_NewSessionUsesWorktreeDir(t *testing.T) {
	m, _ := newTestModel(t, nil)
	m.opts.Config.WorktreeDir = "../wt"
	m = keys(m, "n")
	m = typeText(m, "fix login")
	m = keys(m, "enter")
	if len(m.sessions) != 1 || m.sessions[0].WorktreePath != "/src/wt/fix-login" {
		t.Fatalf("sessions = %+v, want one worktree at /src/wt/fix-login", m.sessions)
	}

	m.opts.Config.WorktreeDir = ""
	m = keys(m, "n")
	m = typeText(m, "second")
	m = keys(m, "enter")
	if len(m.sessions) != 1 || !strings.Contains(m.status, "worktree_dir is empty") {
		t.Errorf("sessions = %d, status = %q; want no new session and the reason", len(m.sessions), m.status)
	}
}

func TestApp_StartFailureRemovesSession(t *testing.T) {
	m, backend := newTestModel(t, nil)
	backend.startErr = errors.New("claude: command not found")
	m = keys(m, "n")
	m = typeText(m, "Broken")
	m = keys(m, "enter")

	if len(m.sessions) != 0 {
		t.Errorf("a session that failed to start should be removed, have %d", len(m.sessions))
	}
	if !m.statusIsErr || m.status == "" {
		t.Error("the start error should be shown")
	}
}

func TestApp_StatusFollowsScreenChanges(t *testing.T) {
	m, _ := newTestModel(t, testutil.Sessions())
	id := "login-1"
	m = send(m, polled(true, screenStatus{id: id, hash: 1}))
	if statusOf(m, id) != session.StatusRunning {
		t.Fatal("the first fingerprint has nothing to compare with and must not change state")
	}
	m = send(m, polled(true, screenStatus{id: id, hash: 1}))
	if got := statusOf(m, id); got != session.StatusReady {
		t.Errorf("unchanged screen: status = %s, want ready", got)
	}
	m = send(m, polled(true, screenStatus{id: id, hash: 2}))
	if got := statusOf(m, id); got != session.StatusRunning {
		t.Errorf("changed screen: status = %s, want running", got)
	}
	m = send(m, polled(true, screenStatus{id: id, gone: true}))
	if got := statusOf(m, id); got != session.StatusStopped {
		t.Errorf("vanished tmux session: status = %s, want stopped", got)
	}
}

// Checks closer together than a statusInterval, as after a detach, call a
// session ready only once its screen has stayed unchanged for stillFor.
func TestApp_ReadyNeedsTheScreenStillForStillFor(t *testing.T) {
	m, _ := newTestModel(t, testutil.Sessions())
	id := "login-1"
	start := time.Now()
	check := func(after time.Duration, hash uint64) session.Status {
		m = send(m, statusMsg{lead: true, at: start.Add(after), results: []screenStatus{{id: id, hash: hash}}})
		return statusOf(m, id)
	}
	check(0, 1)
	if got := check(stillFor-time.Millisecond, 1); got != session.StatusRunning {
		t.Errorf("unchanged for less than stillFor: status = %s, want running", got)
	}
	if got := check(stillFor, 1); got != session.StatusReady {
		t.Errorf("unchanged for stillFor: status = %s, want ready", got)
	}
	if got := check(stillFor+quickInterval, 2); got != session.StatusRunning {
		t.Errorf("changed screen: status = %s, want running", got)
	}
	if got := check(stillFor+2*quickInterval, 2); got != session.StatusRunning {
		t.Errorf("unchanged for a quickInterval since the change: status = %s, want running", got)
	}
}

// No status check runs while the user is attached, so detaching checks at
// once and then quickly for a while, to catch up.
func TestApp_DetachingChecksStatusesQuickly(t *testing.T) {
	m, backend := newTestModel(t, testutil.Sessions())
	m = send(m, attachDoneMsg{id: "login-1"})
	if backend.polls != 1 {
		t.Fatalf("status checks right after detaching = %d, want 1", backend.polls)
	}
	// The quick ticks, sent here, stop after quickChecks checks.
	for range quickChecks + 2 {
		m = send(m, quickStatusMsg{})
	}
	if backend.polls != quickChecks {
		t.Errorf("status checks after detaching = %d, want %d", backend.polls, quickChecks)
	}
}

func readySession(prompt string, autoYes bool) session.Session {
	return session.Session{ID: "s1", Title: "S1", Program: "claude", Status: session.StatusReady, PendingPrompt: prompt, AutoYes: autoYes}
}

func TestApp_PendingPromptIsSentOnceReady(t *testing.T) {
	m, backend := newTestModel(t, []session.Session{readySession("write tests", false)})
	m = send(m, polled(true, screenStatus{id: "s1", hash: 7}))
	if len(backend.pasted) != 0 {
		t.Fatal("the prompt must wait until the screen has been still for a poll")
	}
	m = send(m, polled(true, screenStatus{id: "s1", hash: 7}))
	if !slices.Equal(backend.pasted, []string{"write tests"}) || !slices.Equal(backend.enters, []string{"s1"}) {
		t.Fatalf("pasted %q and pressed enter for %q", backend.pasted, backend.enters)
	}
	if m.sessions[0].PendingPrompt != "" {
		t.Error("a delivered prompt must be cleared so it is never sent twice")
	}
	last := backend.saved[len(backend.saved)-1]
	if last[0].PendingPrompt != "" {
		t.Error("the cleared prompt must be saved")
	}
}

// Resume continues the conversation a Claude session was last seen in, so the
// leader saves it whenever it changes, as after /clear.
func TestApp_LeaderSavesTheConversationToResume(t *testing.T) {
	m, backend := newTestModel(t, []session.Session{readySession("", false)})
	before := len(backend.saved)
	poll := func(lead bool, id string) {
		m = send(m, polled(lead, screenStatus{id: "s1", hash: 7, conversation: subagent.Conversation{ID: id}}))
	}
	poll(false, "first")
	if len(backend.saved) != before {
		t.Fatal("a follower must leave saving the conversation to the leader")
	}
	for _, id := range []string{"first", "first", "cleared"} {
		poll(true, id)
	}
	var got []string
	for _, file := range backend.saved[before:] {
		got = append(got, file[0].Conversation)
	}
	if want := []string{"first", "cleared"}; !slices.Equal(got, want) {
		t.Errorf("saved conversations %q, want each new one once: %q", got, want)
	}
}

func TestApp_TrustPromptHoldsPendingPrompt(t *testing.T) {
	m, backend := newTestModel(t, []session.Session{readySession("go", false)})
	m = send(m, polled(true, screenStatus{id: "s1", hash: 7, trust: true}))
	m = send(m, polled(true, screenStatus{id: "s1", hash: 7, trust: true}))
	if len(backend.pasted) != 0 || len(backend.enters) != 0 {
		t.Error("nothing may be typed into a trust prompt without auto-yes")
	}
	if m.sessions[0].PendingPrompt != "go" {
		t.Error("the prompt must be kept for after the trust prompt is answered")
	}
}

func TestApp_AutoYesAnswersPermissionPrompts(t *testing.T) {
	m, backend := newTestModel(t, []session.Session{readySession("", true)})
	send(m, polled(true, screenStatus{id: "s1", hash: 7, permission: true}))
	if !slices.Equal(backend.enters, []string{"s1"}) {
		t.Errorf("enters = %q, want one for s1", backend.enters)
	}

	m, backend = newTestModel(t, []session.Session{readySession("", false)})
	send(m, polled(true, screenStatus{id: "s1", hash: 7, permission: true}))
	if len(backend.enters) != 0 {
		t.Error("without auto-yes, permission prompts are left for the user")
	}
}

// Every Vineyard on a repository polls every screen; only the one that leads
// types into agents, so no prompt is pasted twice and no Enter lands on the
// agent's next question.
func TestApp_OnlyTheLeaderTypesIntoAgents(t *testing.T) {
	m, backend := newTestModel(t, []session.Session{readySession("write tests", true)})
	for range 2 {
		m = send(m, polled(false, screenStatus{id: "s1", hash: 7, permission: true}))
	}
	if len(backend.pasted) != 0 || len(backend.enters) != 0 {
		t.Fatalf("a follower pasted %q and pressed enter for %q", backend.pasted, backend.enters)
	}
	if m.sessions[0].PendingPrompt != "write tests" {
		t.Fatal("a follower must leave the pending prompt to the leader")
	}
	m = send(m, polled(true, screenStatus{id: "s1", hash: 7}))
	if !slices.Equal(backend.pasted, []string{"write tests"}) {
		t.Errorf("the leader pasted %q, want the pending prompt", backend.pasted)
	}
}

// A reload that read the file before this process's own write must not undo
// it: restoring a delivered prompt would send it again.
func TestApp_StaleReloadIsDropped(t *testing.T) {
	m, backend := newTestModel(t, []session.Session{readySession("write tests", false)})
	stale := reloadedMsg{sessions: slices.Clone(backend.file), writes: m.writes}
	m = send(m, polled(true, screenStatus{id: "s1", hash: 7}))
	m = send(m, polled(true, screenStatus{id: "s1", hash: 7}))
	m = send(m, stale)
	if m.sessions[0].PendingPrompt != "" {
		t.Fatal("a reload that predates the write restored the delivered prompt")
	}
	m = send(m, polled(true, screenStatus{id: "s1", hash: 7}))
	if len(backend.pasted) != 1 {
		t.Errorf("pasted %q, want the prompt once", backend.pasted)
	}
}

// Each tick reloads the sessions other Vineyards save: their new sessions
// appear and the ones they killed go. A session this process is still
// starting is not saved yet and stays.
func TestApp_ReloadAdoptsOtherVineyardsSessions(t *testing.T) {
	mine := session.Session{ID: "mine", Title: "Mine", Status: session.StatusRunning}
	killed := session.Session{ID: "killed", Title: "Killed", Status: session.StatusReady}
	m, backend := newTestModel(t, []session.Session{mine, killed})
	starting := session.Session{ID: "starting", Title: "Starting", Status: session.StatusLoading}
	m.sessions = append(m.sessions, starting)

	saved := mine
	saved.Status = session.StatusReady // as another Vineyard last saw it
	saved.AutoYes = true               // switched on in another Vineyard
	other := session.Session{ID: "other", Title: "Other", Status: session.StatusPaused}
	backend.file = []session.Session{saved, other}

	m = send(m, statusTickMsg{})
	var ids []string
	for _, s := range m.sessions {
		ids = append(ids, s.ID)
	}
	if !slices.Equal(ids, []string{"mine", "other", "starting"}) {
		t.Fatalf("sessions = %q, want mine, other, starting", ids)
	}
	if got := m.sessions[0]; got.Status != session.StatusRunning || !got.AutoYes {
		t.Errorf("mine = %+v; want this process's running status and the saved auto-yes", got)
	}
}

func TestApp_KillAfterConfirmation(t *testing.T) {
	m, backend := newTestModel(t, testutil.Sessions())
	m = keys(m, "D")
	if m.dialog == nil {
		t.Fatal("kill should ask for confirmation")
	}
	m = keys(m, "n")
	if len(backend.killed) != 0 || len(m.sessions) != 5 {
		t.Fatal("declining must not kill")
	}
	m = keys(m, "D", "y")
	if !slices.Equal(backend.killed, []string{"login-1"}) || m.find("login-1") >= 0 {
		t.Errorf("killed %q; session still listed: %v", backend.killed, m.find("login-1") >= 0)
	}
	if s, ok := m.selected(); !ok || s.ID != "tests-2" {
		t.Errorf("selection after kill = %q, want the next session", s.ID)
	}
}

// Agents switch branches inside their worktree; the list must follow git.
func TestApp_GitPollFollowsBranchSwitches(t *testing.T) {
	m, backend := newTestModel(t, testutil.Sessions())
	backend.branches = map[string]string{"login-1": "19/real-work", "tests-2": "20/other"}
	m = send(m, m.diffCmd()())
	for id, want := range backend.branches {
		if got := m.sessions[m.find(id)].Branch; got != want {
			t.Errorf("%s branch = %q, want %q", id, got, want)
		}
	}
}

func TestApp_PauseAndResume(t *testing.T) {
	m, _ := newTestModel(t, testutil.Sessions())
	m = keys(m, "c", "y")
	if got := statusOf(m, "login-1"); got != session.StatusPaused {
		t.Fatalf("after checkout: %s, want paused", got)
	}
	m = keys(m, "enter")
	if m.status == "" {
		t.Error("attaching to a paused session should explain why it cannot")
	}
	m = keys(m, "r")
	if got := statusOf(m, "login-1"); got != session.StatusRunning {
		t.Errorf("after resume: %s, want running", got)
	}
}

func TestApp_ToggleAutoYesIsSaved(t *testing.T) {
	m, backend := newTestModel(t, testutil.Sessions())
	m = keys(m, "a")
	if !m.sessions[0].AutoYes {
		t.Fatal("a should turn auto-yes on")
	}
	if last := backend.saved[len(backend.saved)-1]; !last[0].AutoYes {
		t.Error("the change should be saved")
	}
}

func TestApp_PaneFocusRoutesKeys(t *testing.T) {
	m, _ := newTestModel(t, testutil.Sessions())
	m = keys(m, "l")
	if m.focus != focusPane {
		t.Fatal("l should focus the pane")
	}
	m = keys(m, "j") // scrolls the pane, not the list
	if s, _ := m.selected(); s.ID != "login-1" {
		t.Error("keys must go to the focused pane, not move the selection")
	}
	m = keys(m, "esc")
	if m.focus != focusList {
		t.Error("esc should return focus to the list")
	}
}

// diffSession is a running session with a worktree and a base commit, for
// tests of the diff tool.
func diffSession() session.Session {
	return session.Session{ID: "login-1", Title: "Fix login redirect", Program: "claude", Status: session.StatusRunning,
		WorktreePath: "/src/shop/.vineyard/worktrees/login-1", BaseCommit: "abc123"}
}

// Enter opens the shown tab: the agent, the diff tool, grapes, or a zoomed
// pane. o always attaches.
func TestApp_EnterRunsTheDiffTool(t *testing.T) {
	m, backend := newTestModel(t, []session.Session{diffSession()})
	m.opts.Config.DiffCommand = "true {base} --watch"
	m = keys(m, "tab", "enter")

	if len(backend.tools) != 1 {
		t.Fatalf("enter on the diff tab built %d diff tools, want 1", len(backend.tools))
	}
	tool := backend.tools[0]
	if want := []string{"sh", "-c", "true abc123 --watch"}; !slices.Equal(tool.Args, want) {
		t.Errorf("diff tool args = %q, want %q", tool.Args, want)
	}
	if tool.Dir != diffSession().WorktreePath {
		t.Errorf("diff tool runs in %q, want the worktree", tool.Dir)
	}
	if backend.attaches != 0 || m.zoomed {
		t.Errorf("enter on the diff tab should only run the diff tool; attaches=%d zoomed=%v", backend.attaches, m.zoomed)
	}

	m = keys(m, "o")
	if backend.attaches != 1 {
		t.Error("o should attach from the diff tab")
	}
}

func TestApp_EnterZoomsTheDiffWithoutADiffTool(t *testing.T) {
	for _, command := range []string{"", "vineyard-missing-diff-tool {base}"} {
		m, backend := newTestModel(t, []session.Session{diffSession()})
		m.opts.Config.DiffCommand = command
		m = send(m, diffTickMsg{})
		m = send(m, previewTickMsg{}) // sizes the agent's window
		split := backend.resized[len(backend.resized)-1]
		m = keys(m, "tab", "enter")

		if !m.zoomed || m.focus != focusPane {
			t.Fatalf("diff command %q: enter should zoom our diff; zoomed=%v focus=%d", command, m.zoomed, m.focus)
		}
		if view := screen(m); contains(view, "Fix login redirect") || !contains(view, "timeout = 60") {
			t.Errorf("the zoomed diff should replace the list:\n%s", view)
		}

		// Agents keep the split-pane size, so zooming never reflows them.
		m.sizes = map[string][2]int{} // as after an attach
		m = send(m, previewTickMsg{})
		if got := backend.resized[len(backend.resized)-1]; got != split {
			t.Errorf("zooming resized the agent from %v to %v", split, got)
		}

		m = keys(m, "esc")
		if m.zoomed || m.focus != focusList {
			t.Errorf("esc should leave the zoom for the list; zoomed=%v focus=%d", m.zoomed, m.focus)
		}
		if backend.attaches != 0 {
			t.Error("enter on the diff tab must not attach")
		}
	}
}

func TestApp_EnterOnPreviewAttachesAndOnRecapZooms(t *testing.T) {
	m, backend := newTestModel(t, testutil.Sessions())
	m = keys(m, "enter")
	if backend.attaches != 1 {
		t.Error("enter on the preview tab should attach")
	}

	m = keys(m, "shift+tab", "enter")
	if m.tab != tabRecap || !m.zoomed {
		t.Fatalf("enter on the recap tab should zoom it; tab=%s zoomed=%v", tabNames[m.tab], m.zoomed)
	}
	m = keys(m, "tab")
	if m.zoomed {
		t.Error("switching tabs should leave the zoom")
	}
}

// An attached terminal's status bar shows as configured and lists the active
// sessions in the attached one's multiplexer, which its server can sample,
// but for the agent attached to; attached to a shell, it lists its agent too.
// Its colours suit the terminal's background.
func TestApp_AttachListsTheOtherSessionsInTheStatusBar(t *testing.T) {
	sessions := testutil.Sessions()
	for i := range sessions {
		sessions[i].TmuxName = "vineyard-shop-" + sessions[i].ID
	}
	sessions = append(sessions, session.Session{ID: "tmux-6", Title: "In tmux", Program: "claude", Status: session.StatusRunning, Multiplexer: config.MultiplexerTmux})
	m, backend := newTestModel(t, sessions)
	m.opts.Config.StatusBar = true
	if s, _ := m.selected(); s.ID != "login-1" {
		t.Fatalf("selected %s, want login-1", s.ID)
	}
	agent := session.Peer{Terminal: "vineyard-shop-login-1", Title: "Fix login redirect", Running: true}
	tests := session.Peer{Terminal: "vineyard-shop-tests-2", Title: "Add unit tests for the parser"}

	m = keys(m, "o")
	want := session.Attachment{Detach: []string{"ctrl+q"}, ToggleBar: []string{"ctrl+\\"}, ShowBar: true, Peers: []session.Peer{tests}}
	if !reflect.DeepEqual(backend.attached, want) {
		t.Errorf("attaching to the agent: %+v, want %+v", backend.attached, want)
	}

	m.opts.Config.StatusBar = false
	keys(m, "tab", "tab", "tab", "enter")
	want = session.Attachment{Detach: []string{"ctrl+q"}, ToggleBar: []string{"ctrl+\\"}, Peers: []session.Peer{agent, tests}}
	if backend.shellAttaches != 1 || !reflect.DeepEqual(backend.attached, want) {
		t.Errorf("attaching to the shell (%d times): %+v, want %+v", backend.shellAttaches, backend.attached, want)
	}
	// The bar takes its colours from the background the terminal reports.
	m = update(m, tea.BackgroundColorMsg{Color: color.Black})
	keys(m, "o")
	if !backend.attached.Dark {
		t.Error("attaching from a dark terminal should ask for the dark bar")
	}
}

// The terminal tab, between issue and recap, starts the selected session's
// shell and mirrors it; enter attaches to it. A paused session has no
// worktree to run one in.
func TestTerminalTab_MirrorsTheSessionsShell(t *testing.T) {
	m, backend := newTestModel(t, testutil.Sessions())
	backend.shellScreen = "$ go test ./...\nok"
	m = keys(m, "tab", "tab", "tab")
	if m.tab != tabTerminal {
		t.Fatalf("three tabs from the preview showed %s; want terminal", tabNames[m.tab])
	}
	view := testutil.StripANSI(m.renderPane())
	issue, terminal, recap := strings.Index(view, "Issue"), strings.Index(view, "Terminal"), strings.Index(view, "Recap")
	if !(issue < terminal && terminal < recap) || !contains(view, "$ go test ./...") {
		t.Errorf("the terminal tab should sit between issue and recap and show the shell:\n%s", view)
	}
	if !slices.Equal(backend.shells, []string{"login-1"}) {
		t.Errorf("started shells %v; want login-1's", backend.shells)
	}

	m = keys(m, "enter")
	if backend.shellAttaches != 1 || backend.attaches != 0 {
		t.Errorf("enter on the terminal tab attached %d times to the shell, %d to the agent; want the shell once", backend.shellAttaches, backend.attaches)
	}

	m = keys(m, "j", "j") // docs-3, paused
	m = send(m, previewTickMsg{})
	if view := testutil.StripANSI(m.terminal.View()); !contains(view, "Paused") || slices.Contains(backend.shells, "docs-3") {
		t.Errorf("a paused session's terminal tab should say so and start no shell; shells %v:\n%s", backend.shells, view)
	}
}

func TestApp_StaleScreenIsIgnored(t *testing.T) {
	m, _ := newTestModel(t, testutil.Sessions())
	m = keys(m, "j") // select tests-2
	m = send(m, screenMsg{id: "login-1", screen: "stale screen"})
	if view := testutil.StripANSI(m.preview.View()); contains(view, "stale screen") {
		t.Error("a capture of a previously selected session must not be shown")
	}
}

func TestApp_SwitchTabFromPane(t *testing.T) {
	m, _ := newTestModel(t, testutil.Sessions())
	m = keys(m, "l", "tab")
	if m.tab != tabDiff || m.focus != focusPane {
		t.Errorf("tab in the pane should switch to diff and keep focus; tab=%d focus=%d", m.tab, m.focus)
	}
	m = send(m, common.LeavePaneMsg{})
	if m.focus != focusList {
		t.Error("LeavePaneMsg should focus the list")
	}
}

// Shift+tab walks the tabs backward from the list and from every pane, which
// each forward the key on their own.
func TestApp_ShiftTabSwitchesToThePreviousTab(t *testing.T) {
	m, _ := newTestModel(t, testutil.Sessions())
	m = keys(m, "shift+tab")
	if m.tab != tabRecap || m.focus != focusList {
		t.Fatalf("shift+tab in the list should switch from preview to recap; tab=%d focus=%d", m.tab, m.focus)
	}
	m = keys(m, "l")
	for _, want := range []tab{tabTerminal, tabIssue, tabDiff, tabPreview, tabRecap} {
		m = keys(m, "shift+tab")
		if m.tab != want || m.focus != focusPane {
			t.Fatalf("shift+tab in the pane should switch to %s and keep focus; tab=%s focus=%d", tabNames[want], tabNames[m.tab], m.focus)
		}
	}
}

// The recap tab shows the selected session's conversation, the user's
// messages and Claude Code's recaps, oldest first and opened at its end, and
// says why other agents have none.
func TestRecapTab_ShowsTheSelectedSessionsConversation(t *testing.T) {
	m, backend := newTestModel(t, testutil.Sessions())
	at := time.Date(2026, 10, 9, 9, 0, 0, 0, time.UTC)
	entries := []recap.Entry{{Time: at, Text: "fix the login redirect"}}
	for i := range 30 {
		entries = append(entries, recap.Entry{Time: at.Add(time.Duration(i) * time.Minute), Text: "and then?"})
	}
	entries = append(entries,
		recap.Entry{Time: at.Add(time.Hour), Text: "now run the tests"},
		recap.Entry{Time: at.Add(2 * time.Hour), Text: "The redirect is fixed. Next, run the tests.", Recap: true},
	)
	backend.recaps = map[string]recap.Recap{
		"login-1": {Found: true, Title: "Login redirect fix", Entries: entries},
		"tests-2": {Found: true, Title: "Parser tests"},
	}

	m = keys(m, "shift+tab")
	if m.tab != tabRecap {
		t.Fatalf("shift+tab from preview should reach the recap tab, got %v", m.tab)
	}
	got := screen(m)
	message, summary := strings.Index(got, "now run the tests"), strings.Index(got, "The redirect is fixed")
	if message < 0 || summary < 0 || message > summary || !strings.Contains(got, "You") || !strings.Contains(got, "Recap") {
		t.Fatalf("the recap tab should show the last message, then the recap after it, each labelled:\n%s", got)
	}
	if strings.Contains(got, "fix the login redirect") {
		t.Errorf("the recap tab should open at the end of a long conversation:\n%s", got)
	}

	m = keys(m, "j")
	if got := screen(m); !strings.Contains(got, "Nothing said") {
		t.Errorf("a session without messages should say so:\n%s", got)
	}
	m = keys(m, "j")
	if got := screen(m); !strings.Contains(got, "this session runs codex") {
		t.Errorf("a session of another agent should say recaps come from Claude Code:\n%s", got)
	}

	m = keys(m, "tab")
	if m.tab != tabPreview {
		t.Errorf("tab from the recap tab should return to the preview, got %v", m.tab)
	}
}

func TestRecapTab_DropsAnotherSessionsRecap(t *testing.T) {
	m, _ := newTestModel(t, testutil.Sessions())
	m = keys(m, "shift+tab") // the recap; login-1 is shown
	m = send(m, recapMsg{id: "tests-2", recap: recap.Recap{Found: true, Title: "Parser tests"}})
	if got := screen(m); strings.Contains(got, "Parser tests") {
		t.Errorf("a recap of a session that is not shown must not be displayed:\n%s", got)
	}
}

func contains(s, sub string) bool {
	return len(sub) > 0 && len(s) >= len(sub) && (s == sub || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}

// press applies keys without running their commands, for keys whose only
// command is a text input's cursor blink.
func press(m Model, presses ...string) Model {
	for _, k := range presses {
		m = update(m, testutil.Key(k))
	}
	return m
}

func TestAppView_Config(t *testing.T) {
	m, _ := newTestModel(t, testutil.Sessions())
	m = keys(m, "C", "l")
	testutil.RequireGolden(t, m.View().Content)
}

func TestApp_SavedConfigAppliesToTheNextSession(t *testing.T) {
	m, backend := newTestModel(t, nil)
	m = press(m, "C", "l", "j", "enter") // edit the branch prefix
	for range len("test/") {
		m = update(m, testutil.Key("backspace"))
	}
	m = typeText(m, "agents/")
	m = press(m, "enter", "j", "enter") // and turn on auto-yes
	m = keys(m, "ctrl+s")

	if m.configOpen {
		t.Fatal("saving should close the config screen")
	}
	if len(backend.configs) != 1 || backend.configs[0].BranchPrefix != "agents/" || !backend.configs[0].AutoYes {
		t.Fatalf("saved configs = %+v", backend.configs)
	}

	m = keys(m, "n")
	m = typeText(m, "Fix login")
	m = keys(m, "enter")
	s, ok := m.selected()
	if !ok || s.Branch != "agents/fix-login" || !s.AutoYes {
		t.Errorf("new session = %+v, want the saved prefix and auto-yes", s)
	}
}

func TestApp_ConfigEscDiscardsEdits(t *testing.T) {
	m, backend := newTestModel(t, nil)
	m = keys(m, "C", "l", "j", "j", "enter", "esc", "esc") // auto-yes on, then leave
	if m.configOpen || len(backend.configs) != 0 || m.opts.Config.AutoYes {
		t.Errorf("esc should leave without saving: open %v, saved %+v, auto-yes %v", m.configOpen, backend.configs, m.opts.Config.AutoYes)
	}
	m = keys(m, "C", "l", "j", "j")
	if strings.Contains(testutil.StripANSI(m.View().Content), "Auto-yes for new sessions  on") {
		t.Error("reopening the screen should show the saved config, not the discarded edit")
	}
}

func TestApp_ConfigNeverOverwritesAFileThatFailedToLoad(t *testing.T) {
	m, backend := newTestModel(t, nil)
	m.opts.ConfigErr = errors.New("config.toml: expected value")
	m = keys(m, "C", "ctrl+s")
	if len(backend.configs) != 0 {
		t.Errorf("config saved despite the load error: %+v", backend.configs)
	}
}

func TestTheme_LightUntilTerminalReportsDark(t *testing.T) {
	m, _ := newTestModel(t, nil)
	if m.theme.IsDark {
		t.Fatal("a new model should start with the light theme")
	}
	m = update(m, tea.BackgroundColorMsg{Color: color.Black})
	if !m.theme.IsDark {
		t.Error("a terminal reporting a dark background should switch to the dark theme")
	}
}

// running is a conversation whose subagents all run, with descriptions, in
// the order they started.
func running(descriptions ...string) subagent.Conversation {
	var c subagent.Conversation
	for i, d := range descriptions {
		c.Agents = append(c.Agents, subagent.Agent{
			ID: fmt.Sprintf("a%d", i+1), Type: "general-purpose", Description: d, Started: time.Unix(int64(i), 0),
		})
	}
	return c
}

// listing is Claude Code's list of n subagents, all drawn, with the
// conversation shown.
func listing(n int) subagent.List {
	return subagent.List{Rows: slices.Repeat([]subagent.Row{{Type: "general-purpose"}}, n), Shown: 0, Focus: -1}
}

// agentScreen is a Claude Code screen listing two subagents, with one more
// out of sight.
const agentScreen = `❯ 
  ● main
  ◯ general-purpose  Running the parser tests        5s · ↓ 19.4k tokens
  ◯ Explore  Finding the redirect                    5s · ↓ 19.4k tokens
  ↓ 1 more
`

func TestApp_StatusPollListsClaudeSubagents(t *testing.T) {
	m, backend := newTestModel(t, []session.Session{
		{ID: "c1", Title: "Claude session", Program: "claude --model opus", Status: session.StatusRunning},
		{ID: "a1", Title: "Aider session", Program: "aider", Status: session.StatusRunning},
	})
	backend.screen = agentScreen
	backend.conversation = running("parser tests", "find the redirect", "update the docs")
	m = send(m, statusTickMsg{})
	view := screen(m)
	claude, count, aider := indexOf(view, "Claude session"), indexOf(view, "▸ 3 subagents running"), indexOf(view, "Aider session")
	if !(claude < count && count < aider) || strings.Count(view, "▸") != 1 || contains(view, "parser tests") {
		t.Errorf("want three subagents counted, not listed, under the Claude session only:\n%s", view)
	}
	view = screen(keys(m, "right"))
	claude, rows, aider := indexOf(view, "Claude session"), indexOf(view, "├ ◑ parser tests"), indexOf(view, "Aider session")
	if !(claude < rows && rows < aider) || strings.Count(view, "├")+strings.Count(view, "└") != 3 || !contains(view, "└ ◑ update the docs") {
		t.Errorf("→ should list the three subagents the transcripts record under the Claude session:\n%s", view)
	}
}

// Claude Code draws its agent list last, so a poll can catch it redrawing,
// as right after a detach, with fewer subagents or none. The rows come from
// the transcripts and keep their count. The list on screen, which opening a
// subagent needs, is believed to shrink only when the next poll agrees.
func TestApp_SubagentsSurviveAPollCatchingARedraw(t *testing.T) {
	m, backend := newTestModel(t, testutil.Sessions())
	conv := running("one", "two", "three")
	poll := func(m Model, listed subagent.List) Model {
		return send(m, polled(false, screenStatus{id: "login-1", hash: 1, subagents: listed, conversation: conv}))
	}
	m = poll(m, listing(3))
	if m = poll(m, subagent.List{}); !contains(screen(m), "▸ 3 subagents running") {
		t.Errorf("a poll without the list should keep the count:\n%s", screen(m))
	}
	m = keys(m, "l", "j", "o")
	if !slices.Equal(backend.opened, [][2]int{{1, 3}}) {
		t.Errorf("opened %v; one poll without the list should still open the subagent at 1 of 3", backend.opened)
	}
	m = keys(poll(m, subagent.List{}), "o")
	if len(backend.opened) != 1 || !contains(m.status, "does not match") {
		t.Errorf("opened %v, status %q; two polls without the list mean Claude Code no longer lists them", backend.opened, m.status)
	}

	// Pausing or resuming restarts Claude Code, which forgets its subagents.
	m = poll(m, listing(3))
	paused := testutil.Sessions()[0]
	paused.Status = session.StatusPaused
	m = send(m, lifecycleMsg{session: paused, verb: "pause"})
	paused.Status = session.StatusRunning
	if m = send(m, lifecycleMsg{session: paused, verb: "resume"}); contains(screen(m), "subagent") {
		t.Errorf("a resumed session should show no subagents before a poll finds them:\n%s", screen(m))
	}
}

// → and ← fold a session's subagent rows, and → gives the pane focus where
// there is nothing to show.
func TestApp_ArrowsFoldSubagentsBeforeThePane(t *testing.T) {
	m, _ := newTestModel(t, testutil.Sessions())
	m = send(m, polled(false, screenStatus{id: "login-1", hash: 1, subagents: listing(1), conversation: running("one")}))
	if !contains(screen(m), "l show subagents") {
		t.Errorf("the status bar should offer to show the subagents:\n%s", screen(m))
	}
	m = keys(m, "l")
	if m.focus != focusList || !contains(screen(m), "└ ◑ one") {
		t.Fatalf("l should show the subagent rows and keep the list focused; focus=%d\n%s", m.focus, screen(m))
	}
	m = keys(m, "j", "left")
	if _, ok := m.list.SelectedSubagent(); ok || contains(screen(m), "└ ◑ one") || selectedID(m) != "login-1" {
		t.Errorf("← on a subagent row should hide the rows and select the session:\n%s", screen(m))
	}
	m = keys(m, "right", "right")
	if m.focus != focusPane {
		t.Errorf("→ on a session showing its subagents should give the pane focus; focus=%d", m.focus)
	}
	m = keys(m, "esc", "left", "j", "right")
	if selectedID(m) == "login-1" || m.focus != focusPane {
		t.Errorf("→ on a session without subagents should give the pane focus; focus=%d", m.focus)
	}
}

func TestApp_EnterOnASubagentRowOpensItThenAttaches(t *testing.T) {
	m, backend := newTestModel(t, testutil.Sessions())
	listed := subagent.List{
		Rows:  []subagent.Row{{Type: "general-purpose", Label: "two"}, {Type: "general-purpose", Label: "three"}},
		Above: 1, Below: 1, Shown: 0, Focus: -1,
	}
	conv := running("one", "two", "three", "four")
	m = send(m, polled(false, screenStatus{id: "login-1", hash: 1, subagents: listed, conversation: conv}))
	m = keys(m, "l", "j", "j", "j") // the third subagent, the second Claude Code draws
	if len(backend.opened) != 0 || len(backend.enters) != 0 || backend.attaches != 0 {
		t.Fatal("selecting a subagent row must not type into the agent")
	}
	m = keys(m, "enter")
	if !slices.Equal(backend.opened, [][2]int{{3, 4}}) || backend.attaches != 1 {
		t.Fatalf("opened %v and attached %d times; want position 3 of 4, then attach", backend.opened, backend.attaches)
	}
	keys(m, "tab", "o")
	if len(backend.opened) != 2 || backend.attaches != 2 {
		t.Errorf("o on the diff tab opened %v and attached %d times", backend.opened, backend.attaches)
	}
}

// A finished subagent stays, marked done, until the user's next prompt.
func TestApp_SubagentsShowTheirStatusUntilTheNextPrompt(t *testing.T) {
	m, _ := newTestModel(t, testutil.Sessions())
	conv := running("review the parser", "write the tests")
	conv.Agents[0].Status, conv.Agents[0].Ended = subagent.Done, time.Unix(100, 0)
	conv.Prompted = time.Unix(50, 0)
	m = send(m, polled(false, screenStatus{id: "login-1", hash: 1, subagents: listing(1), conversation: conv}))
	m = keys(m, "l")
	view := screen(m)
	if !contains(view, "▾ 2 subagents: 1 running") || !contains(view, "├ ● review the parser") || !contains(view, "└ ◑ write the tests") {
		t.Errorf("want the finished subagent kept and marked done, the other running:\n%s", view)
	}
	conv.Prompted = time.Unix(150, 0)
	m = send(m, polled(false, screenStatus{id: "login-1", hash: 2, subagents: listing(1), conversation: conv}))
	if view := screen(m); contains(view, "review the parser") || !contains(view, "▾ 1 subagent running") {
		t.Errorf("a prompt after it finished should drop the finished subagent:\n%s", view)
	}
}

// Claude Code no longer lists a finished subagent, except the one it shows,
// which still takes a place in its list.
func TestApp_OpenFindsTheSubagentInClaudeCodesList(t *testing.T) {
	m, backend := newTestModel(t, testutil.Sessions())
	conv := running("shown, finished", "running", "finished")
	conv.Agents[0].Status, conv.Agents[0].Ended = subagent.Done, time.Unix(100, 0)
	conv.Agents[2].Status, conv.Agents[2].Ended = subagent.Done, time.Unix(100, 0)
	listed := listing(2)
	listed.Shown = 1
	m = send(m, polled(false, screenStatus{id: "login-1", hash: 1, subagents: listed, conversation: conv}))
	m = keys(m, "l", "j", "j", "o")
	if !slices.Equal(backend.opened, [][2]int{{2, 2}}) {
		t.Fatalf("opened %v, want the running subagent at position 2 of 2, after the finished one in view", backend.opened)
	}
	m = keys(m, "j", "o")
	if len(backend.opened) != 1 || !contains(m.status, "has finished") {
		t.Errorf("opened %v, status %q; a finished subagent out of Claude Code's list must not be opened", backend.opened, m.status)
	}
}

// A subagent row shows the subagent in every tab: its transcript in place of
// the screen, its own worktree's diff and issue, and what it reported.
func TestApp_ASubagentRowShowsTheSubagent(t *testing.T) {
	sessions := testutil.Sessions()
	sessions[0].WorktreePath = "/src/shop/.vineyard/worktrees/login"
	m, backend := newTestModel(t, sessions)
	conv := running("write the tests", "update the docs")
	conv.Agents[0].Dir = "/src/shop/.claude/worktrees/tests/internal"
	conv.Agents[0].Status, conv.Agents[0].Ended = subagent.Done, time.Unix(100, 0)
	conv.Agents[0].Prompt, conv.Agents[0].Report = "Write the parser tests.", "All 12 tests pass."
	conv.Agents[1].Edited = []string{"/src/shop/.vineyard/worktrees/login/.grapes/3/comments.md"}
	backend.conversation = conv
	backend.events = map[string][]subagent.Event{"a1": {
		{Kind: subagent.Prompt, Text: "Write the parser tests."},
		{Kind: subagent.Tool, Name: "Bash", Text: "go test ./..."},
		{Kind: subagent.Result, Text: "ok  shop/parser"},
		{Kind: subagent.Text, Text: "All 12 tests pass."},
	}}
	backend.trees = map[string]string{"/src/shop/.claude/worktrees/tests/internal": "/src/shop/.claude/worktrees/tests"}
	backend.treeDiffs = map[string]string{"/src/shop/.claude/worktrees/tests": strings.ReplaceAll(sampleDiff, "auth.go", "parser_test.go") +
		"diff --git a/.grapes/7/meta.toml b/.grapes/7/meta.toml\n--- a/.grapes/7/meta.toml\n+++ b/.grapes/7/meta.toml\n@@ -1 +1 @@\n-status = 'todo'\n+status = 'done'\n"}
	m = send(m, polled(false, screenStatus{id: "login-1", hash: 1, subagents: listing(1), conversation: conv}))
	m = keys(m, "l", "j")

	view := screen(m)
	for _, want := range []string{"❯ Write the parser tests.", "● Bash(go test ./...)", "⎿ ok  shop/parser", "● All 12 tests pass."} {
		if !contains(view, want) {
			t.Errorf("the preview should show the subagent's transcript, with %q:\n%s", want, view)
		}
	}
	if contains(view, "fix the redirect") {
		t.Errorf("the preview still shows the session's screen:\n%s", view)
	}
	m = send(m, diffTickMsg{})
	if view := screen(keys(m, "tab")); !contains(view, "parser_test.go") || contains(view, "auth.go") {
		t.Errorf("the diff should be the subagent's own worktree's:\n%s", view)
	}
	s, _ := m.selected()
	if ids := m.shownIssues(s); !slices.Equal(ids, []int{7}) {
		t.Errorf("issues = %v, want 7, from its worktree's diff", ids)
	}
	if view := screen(keys(m, "tab", "tab", "tab", "tab")); !contains(view, "write the tests") || !contains(view, "done") || !contains(view, "All 12 tests pass.") || !contains(view, "Write the parser tests.") {
		t.Errorf("the recap should show the subagent's description, status, report, and prompt:\n%s", view)
	}

	m = keys(m, "j") // the subagent in the session's worktree, on the preview tab
	m = send(m, diffTickMsg{})
	if s, _ := m.selected(); !slices.Equal(m.shownIssues(s), []int{3}) || m.agentTree != "" {
		t.Errorf("issues = %v, tree %q; want 3, whose file it edited, in the session's worktree", m.shownIssues(s), m.agentTree)
	}
	m = keys(m, "k", "k")
	if view := screen(m); !contains(view, "fix the redirect") {
		t.Errorf("the session row should show the session's screen again:\n%s", view)
	}
}

func TestApp_FailedSubagentOpenDoesNotAttach(t *testing.T) {
	m, backend := newTestModel(t, testutil.Sessions())
	backend.openErr = subagent.ErrChanged
	m = send(m, polled(true, screenStatus{id: "login-1", hash: 1, subagents: listing(1), conversation: running("one")}))
	m = keys(m, "l", "j", "enter")
	if backend.attaches != 0 {
		t.Error("a subagent that did not open must not be attached to")
	}
	if !m.statusIsErr || !contains(m.status, subagent.ErrChanged.Error()) {
		t.Errorf("status = %q, want the error", m.status)
	}
}

// clickOn clicks the first cell of the first text on the screen, from line
// from down.
func clickOn(t *testing.T, m Model, text string, from int) Model {
	t.Helper()
	return send(m, find(t, m, text, from))
}

// find returns a click on the first cell of the first text on the screen,
// from line from down.
func find(t *testing.T, m Model, text string, from int) tea.MouseClickMsg {
	t.Helper()
	lines := strings.Split(screen(m), "\n")
	for y := from; y < len(lines); y++ {
		if i := strings.Index(lines[y], text); i >= 0 {
			return tea.MouseClickMsg{X: ansi.StringWidth(lines[y][:i]), Y: y}
		}
	}
	t.Fatalf("%q is not on the screen:\n%s", text, screen(m))
	return tea.MouseClickMsg{}
}

func TestApp_ClickingATabShowsIt(t *testing.T) {
	m, _ := newTestModel(t, testutil.Sessions())
	tabBar := headerHeight + 1
	m = clickOn(t, m, "Diff", tabBar)
	if m.tab != tabDiff || m.focus != focusList {
		t.Fatalf("a click on Diff left tab %d, focus %d; want the diff, focus unchanged", m.tab, m.focus)
	}
	m = clickOn(t, m, "Recap", tabBar)
	m = keys(m, "enter") // zoom the recap
	if !m.zoomed {
		t.Fatal("enter on the recap tab should zoom")
	}
	m = clickOn(t, m, "Preview", tabBar)
	if m.tab != tabPreview || m.zoomed {
		t.Errorf("a click on Preview in a zoomed pane left tab %d, zoomed %v", m.tab, m.zoomed)
	}
}

func TestApp_ClicksOnTheHeaderAndStatusBarDoNothing(t *testing.T) {
	m, _ := newTestModel(t, testutil.Sessions())
	m = keys(m, "j")
	selected := selectedID(m)
	for _, y := range []int{0, 1, m.height - 1} {
		for _, x := range []int{2, m.width - 2} {
			m = send(m, tea.MouseClickMsg{X: x, Y: y})
			if selectedID(m) != selected || m.focus != focusList {
				t.Errorf("a click at %d,%d selected %s, focus %d", x, y, selectedID(m), m.focus)
			}
		}
	}
}

// An agent drawing on the alternate screen, as Claude Code does with
// "tui": "fullscreen", has no scrollback; the fake returns only the screen.
func TestApp_PreviewWithoutScrollbackStaysLive(t *testing.T) {
	m, _ := newTestModel(t, testutil.Sessions())
	m = send(m, previewTickMsg{}) // capture the screen
	m = keys(m, "l", "k")
	if m.preview.Scrolling() || !contains(m.status, "No scrollback") {
		t.Errorf("scrolling %v, status %q; want the live preview and a message", m.preview.Scrolling(), m.status)
	}
}

func TestApp_KillLosingCommitsAsksAgain(t *testing.T) {
	m, backend := newTestModel(t, testutil.Sessions())
	var commits []string
	for i := range 7 {
		commits = append(commits, fmt.Sprintf("c0ffee%d Try parser %d", i, i))
	}
	backend.lost = map[string][]string{"login-1": commits}

	m = keys(m, "D", "y")
	if len(backend.killed) != 0 || m.dialog == nil {
		t.Fatal("a kill that loses commits should ask again instead of killing")
	}
	view := testutil.StripANSI(m.View().Content)
	for _, want := range []string{"c0ffee0 Try parser 0", "c0ffee4 Try parser 4", "… and 2 more", "git switch -c"} {
		if !strings.Contains(view, want) {
			t.Errorf("the dialog should show %q:\n%s", want, view)
		}
	}
	if strings.Contains(view, "c0ffee5") {
		t.Errorf("the dialog should list at most five commits:\n%s", view)
	}

	m = keys(m, "n")
	if len(backend.killed) != 0 || m.find("login-1") < 0 {
		t.Fatal("declining must keep the session and its commits")
	}
	m = keys(m, "D", "y", "y")
	if !slices.Equal(backend.killed, []string{"login-1"}) || m.find("login-1") >= 0 {
		t.Errorf("killed %q; session still listed: %v", backend.killed, m.find("login-1") >= 0)
	}
}

func TestApp_DialogsAndConfigTakeClicks(t *testing.T) {
	m, backend := newTestModel(t, testutil.Sessions())
	m = keys(m, "D")
	m = clickOn(t, m, "n cancel", 0)
	if m.dialog != nil || len(backend.killed) != 0 {
		t.Fatalf("a click on cancel left dialog %v, killed %v", m.dialog != nil, backend.killed)
	}
	m = keys(m, "D")
	m = clickOn(t, m, "y confirm", 0)
	if m.dialog != nil || len(backend.killed) != 1 {
		t.Errorf("a click on confirm left dialog %v, killed %v", m.dialog != nil, backend.killed)
	}

	m = keys(m, "C")
	m = clickOn(t, m, "Profiles", headerHeight)
	if !contains(screen(m), "+ Add profile") {
		t.Errorf("a click on Profiles should show its fields:\n%s", screen(m))
	}
}

// clock makes now return a time the test moves on, and returns it.
func clock(t *testing.T) *time.Time {
	t.Helper()
	c, before := time.Unix(0, 0), now
	now = func() time.Time { return c }
	t.Cleanup(func() { now = before })
	return &c
}

// doubleClickOn clicks the cell text starts on twice, 100 ms apart.
func doubleClickOn(t *testing.T, m Model, c *time.Time, text string, from int) Model {
	t.Helper()
	at := find(t, m, text, from)
	m = send(m, at)
	*c = c.Add(100 * time.Millisecond)
	return send(m, at)
}

func TestApp_DoubleClickOnASessionIsEnter(t *testing.T) {
	c := clock(t)
	m, backend := newTestModel(t, testutil.Sessions())
	m = clickOn(t, m, "Fix login redirect", 0)
	*c = c.Add(600 * time.Millisecond)
	m = clickOn(t, m, "Fix login redirect", 0)
	if backend.attaches != 0 {
		t.Fatal("two clicks 600 ms apart are two single clicks")
	}
	*c = c.Add(600 * time.Millisecond)
	m = doubleClickOn(t, m, c, "Fix login redirect", 0)
	if backend.attaches != 1 {
		t.Errorf("a double click on a session in the preview tab attached %d times, want 1", backend.attaches)
	}
	*c = c.Add(time.Second)
	m = doubleClickOn(t, m, c, "Recap", headerHeight+1)
	if !m.zoomed || m.tab != tabRecap {
		t.Errorf("a double click on the Recap tab should show and zoom it; tab %d, zoomed %v", m.tab, m.zoomed)
	}
}

func TestApp_DoubleClickOnASubagentOpensIt(t *testing.T) {
	c := clock(t)
	m, backend := newTestModel(t, testutil.Sessions())
	m = send(m, polled(false, screenStatus{id: "login-1", hash: 1, subagents: listing(1), conversation: running("one")}))
	m = doubleClickOn(t, m, c, "1 subagent", 0)
	if !contains(screen(m), "▾ 1 subagent") || backend.attaches != 0 {
		t.Fatalf("a double click on the count line should expand it once and attach nothing; attached %d\n%s", backend.attaches, screen(m))
	}
	m = clickOn(t, m, "Diff", headerHeight+1)
	*c = c.Add(time.Second)
	doubleClickOn(t, m, c, "└ ◑ one", 0)
	if !slices.Equal(backend.opened, [][2]int{{1, 1}}) || backend.attaches != 1 {
		t.Errorf("a double click on a subagent on the diff tab opened %v and attached %d times", backend.opened, backend.attaches)
	}
}

func TestApp_DoubleClickInThePane(t *testing.T) {
	c := clock(t)
	m, backend := newTestModel(t, testutil.Sessions())
	m = send(m, previewTickMsg{}) // capture the screen
	m = doubleClickOn(t, m, c, "fix the redirect", headerHeight+2)
	if backend.attaches != 1 {
		t.Errorf("a double click on the preview attached %d times, want 1", backend.attaches)
	}
	*c = c.Add(time.Second)
	m = keys(m, "tab") // the diff
	m = send(m, diffTickMsg{})
	m = doubleClickOn(t, m, c, "auth.go", headerHeight+2) // in the summary
	if backend.attaches != 1 || m.zoomed || m.diff.CurrentFile() != 0 || !contains(screen(m), "▾ M auth.go") {
		t.Fatalf("a double click on the diff summary should only jump to the file: attached %d, zoomed %v, file %d\n%s", backend.attaches, m.zoomed, m.diff.CurrentFile(), screen(m))
	}
	*c = c.Add(time.Second)
	m = doubleClickOn(t, m, c, "timeout = 60", headerHeight+2)
	if !m.zoomed || m.tab != tabDiff {
		t.Errorf("a double click on the diff's code should open the full diff, here zoomed without a diff tool; zoomed %v, tab %d", m.zoomed, m.tab)
	}
}

// The click that answers a dialog closes it; the clicks before and after it
// on the same cell land on the screen beneath and must not make a double
// click there.
func TestApp_ClosingADialogMakesNoDoubleClick(t *testing.T) {
	c := clock(t)
	m, backend := newTestModel(t, testutil.Sessions())
	cancel := find(t, keys(m, "D"), "n cancel", 0)
	m = send(m, cancel) // on the preview, beneath where the dialog opens
	m = keys(m, "esc", "D")
	for range 2 {
		*c = c.Add(100 * time.Millisecond)
		m = send(m, cancel)
	}
	if m.dialog != nil || backend.attaches != 0 || len(backend.killed) != 0 {
		t.Errorf("dialog open %v, attached %d, killed %v", m.dialog != nil, backend.attaches, backend.killed)
	}
}

// An agent on an older Vineyard multiplexer still runs, though this Vineyard
// cannot see its screen: its status stays, and the preview says why it is
// blank instead.
func TestApp_AgentOnAnOlderMultiplexerKeepsRunning(t *testing.T) {
	m, backend := newTestModel(t, testutil.Sessions())
	backend.captureErr = fmt.Errorf("%w (process 4242, protocol 1)", session.ErrOlderMultiplexer)
	before := statusOf(m, m.sessions[0].ID)
	m = send(m, statusTickMsg{})
	m = send(m, previewTickMsg{})
	if got := statusOf(m, m.sessions[0].ID); got != before {
		t.Errorf("status = %s, want %s, as before", got, before)
	}
	if view := ansi.Strip(m.View().Content); !strings.Contains(view, "older Vineyard multiplexer") {
		t.Errorf("the preview does not say why it is blank:\n%s", view)
	}
}
