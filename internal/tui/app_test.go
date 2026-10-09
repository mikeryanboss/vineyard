package tui

import (
	"errors"
	"image/color"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/git"
	"github.com/mikeryanboss/vineyard/internal/recap"
	"github.com/mikeryanboss/vineyard/internal/session"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
	"github.com/mikeryanboss/vineyard/internal/tui/testutil"
)

// fakeBackend records what the model asks of the outside world.
type fakeBackend struct {
	mu       sync.Mutex
	screen   string
	diff     string
	startErr error
	file     []session.Session   // sessions.json, which other Vineyards share
	saved    [][]session.Session // the file after each write
	follower bool                // another Vineyard leads
	pasted   []string
	enters   []string
	killed   []string
	branches map[string]string // session ID -> checked-out branch
	resized  [][2]int
	configs  []config.Config
	recaps   map[string]recap.Recap // by session ID
	attaches int
	tools    []*exec.Cmd // diff tools built
}

func (f *fakeBackend) New(opts session.NewOptions) session.Session {
	return session.Session{
		ID: session.Slug(opts.Title, "s"), Title: opts.Title, Program: opts.Program,
		Branch:        opts.BranchPrefix + session.Slug(opts.Title, "s"),
		PendingPrompt: opts.Prompt, AutoYes: opts.AutoYes, Status: session.StatusLoading,
		Issue: opts.Issue, WorktreePath: opts.WorktreeDir + "/" + session.Slug(opts.Title, "s"),
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

func (f *fakeBackend) Kill(s session.Session) (session.KillResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.killed = append(f.killed, s.ID)
	return session.KillResult{KeptBranch: s.Branch}, nil
}

func (f *fakeBackend) Push(s session.Session) (session.Session, error)         { return s, nil }
func (f *fakeBackend) EnsureShell(s session.Session, w, h int) error           { return nil }
func (f *fakeBackend) Capture(s session.Session) (string, error)               { return f.screen, nil }
func (f *fakeBackend) CaptureHistory(s session.Session, n int) (string, error) { return f.screen, nil }
func (f *fakeBackend) AttachCommand(s session.Session) *exec.Cmd {
	f.attaches++
	return exec.Command("true")
}

func (f *fakeBackend) DiffTool(s session.Session, command string) (*exec.Cmd, error) {
	cmd, err := diffTool(s, command)
	if err == nil {
		f.mu.Lock()
		f.tools = append(f.tools, cmd)
		f.mu.Unlock()
	}
	return cmd, err
}

func (f *fakeBackend) ShellAttachCommand(s session.Session) *exec.Cmd { return exec.Command("true") }
func (f *fakeBackend) Diff(s session.Session) (string, error)         { return f.diff, nil }
func (f *fakeBackend) DiffStat(s session.Session) (git.Stat, error)   { return git.Stat{Added: 3}, nil }

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

func (f *fakeBackend) Lead() (bool, error) { return !f.follower, nil }

func (f *fakeBackend) SaveConfig(cfg config.Config) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.configs = append(f.configs, cfg)
	return nil
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
		Config:     config.Config{DefaultProgram: "claude", BranchPrefix: "test/", WorktreeDir: ".vineyard/worktrees"},
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
	m = send(m, statusMsg{lead: true, results: []screenStatus{{id: id, hash: 1}}})
	if statusOf(m, id) != session.StatusRunning {
		t.Fatal("the first fingerprint has nothing to compare with and must not change state")
	}
	m = send(m, statusMsg{lead: true, results: []screenStatus{{id: id, hash: 1}}})
	if got := statusOf(m, id); got != session.StatusReady {
		t.Errorf("unchanged screen: status = %s, want ready", got)
	}
	m = send(m, statusMsg{lead: true, results: []screenStatus{{id: id, hash: 2}}})
	if got := statusOf(m, id); got != session.StatusRunning {
		t.Errorf("changed screen: status = %s, want running", got)
	}
	m = send(m, statusMsg{lead: true, results: []screenStatus{{id: id, gone: true}}})
	if got := statusOf(m, id); got != session.StatusStopped {
		t.Errorf("vanished tmux session: status = %s, want stopped", got)
	}
}

func readySession(prompt string, autoYes bool) session.Session {
	return session.Session{ID: "s1", Title: "S1", Program: "claude", Status: session.StatusReady, PendingPrompt: prompt, AutoYes: autoYes}
}

func TestApp_PendingPromptIsSentOnceReady(t *testing.T) {
	m, backend := newTestModel(t, []session.Session{readySession("write tests", false)})
	m = send(m, statusMsg{lead: true, results: []screenStatus{{id: "s1", hash: 7}}})
	if len(backend.pasted) != 0 {
		t.Fatal("the prompt must wait until the screen has been still for a poll")
	}
	m = send(m, statusMsg{lead: true, results: []screenStatus{{id: "s1", hash: 7}}})
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

func TestApp_TrustPromptHoldsPendingPrompt(t *testing.T) {
	m, backend := newTestModel(t, []session.Session{readySession("go", false)})
	m = send(m, statusMsg{lead: true, results: []screenStatus{{id: "s1", hash: 7, trust: true}}})
	m = send(m, statusMsg{lead: true, results: []screenStatus{{id: "s1", hash: 7, trust: true}}})
	if len(backend.pasted) != 0 || len(backend.enters) != 0 {
		t.Error("nothing may be typed into a trust prompt without auto-yes")
	}
	if m.sessions[0].PendingPrompt != "go" {
		t.Error("the prompt must be kept for after the trust prompt is answered")
	}
}

func TestApp_AutoYesAnswersPermissionPrompts(t *testing.T) {
	m, backend := newTestModel(t, []session.Session{readySession("", true)})
	send(m, statusMsg{lead: true, results: []screenStatus{{id: "s1", hash: 7, permission: true}}})
	if !slices.Equal(backend.enters, []string{"s1"}) {
		t.Errorf("enters = %q, want one for s1", backend.enters)
	}

	m, backend = newTestModel(t, []session.Session{readySession("", false)})
	send(m, statusMsg{lead: true, results: []screenStatus{{id: "s1", hash: 7, permission: true}}})
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
		m = send(m, statusMsg{lead: false, results: []screenStatus{{id: "s1", hash: 7, permission: true}}})
	}
	if len(backend.pasted) != 0 || len(backend.enters) != 0 {
		t.Fatalf("a follower pasted %q and pressed enter for %q", backend.pasted, backend.enters)
	}
	if m.sessions[0].PendingPrompt != "write tests" {
		t.Fatal("a follower must leave the pending prompt to the leader")
	}
	m = send(m, statusMsg{lead: true, results: []screenStatus{{id: "s1", hash: 7}}})
	if !slices.Equal(backend.pasted, []string{"write tests"}) {
		t.Errorf("the leader pasted %q, want the pending prompt", backend.pasted)
	}
}

// A reload that read the file before this process's own write must not undo
// it: restoring a delivered prompt would send it again.
func TestApp_StaleReloadIsDropped(t *testing.T) {
	m, backend := newTestModel(t, []session.Session{readySession("write tests", false)})
	stale := reloadedMsg{sessions: slices.Clone(backend.file), writes: m.writes}
	m = send(m, statusMsg{lead: true, results: []screenStatus{{id: "s1", hash: 7}}})
	m = send(m, statusMsg{lead: true, results: []screenStatus{{id: "s1", hash: 7}}})
	m = send(m, stale)
	if m.sessions[0].PendingPrompt != "" {
		t.Fatal("a reload that predates the write restored the delivered prompt")
	}
	m = send(m, statusMsg{lead: true, results: []screenStatus{{id: "s1", hash: 7}}})
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
	for _, want := range []tab{tabIssue, tabDiff, tabPreview, tabRecap} {
		m = keys(m, "shift+tab")
		if m.tab != want || m.focus != focusPane {
			t.Fatalf("shift+tab in the pane should switch to %s and keep focus; tab=%s focus=%d", tabNames[want], tabNames[m.tab], m.focus)
		}
	}
}

// The recap tab shows Claude Code's recaps of the selected session, newest
// first, falls back to the last prompt, and says why other agents have none.
func TestRecapTab_ShowsTheSelectedSessionsRecaps(t *testing.T) {
	m, backend := newTestModel(t, testutil.Sessions())
	at := time.Date(2026, 10, 9, 9, 0, 0, 0, time.UTC)
	backend.recaps = map[string]recap.Recap{
		"login-1": {Found: true, Title: "Login redirect fix", Entries: []recap.Entry{
			{Time: at.Add(time.Hour), Text: "The redirect is fixed. Next, run the tests."},
			{Time: at, Text: "I found the redirect bug."},
		}},
		"tests-2": {Found: true, Title: "Parser tests", LastPrompt: "add tests for the parser"},
	}

	m = keys(m, "tab", "tab", "tab")
	if m.tab != tabRecap {
		t.Fatalf("three tabs from preview should reach the recap tab, got %v", m.tab)
	}
	got := screen(m)
	newer, older := strings.Index(got, "The redirect is fixed"), strings.Index(got, "I found the redirect bug")
	if !strings.Contains(got, "Login redirect fix") || newer < 0 || older < 0 || newer > older {
		t.Fatalf("the recap tab should show the title and both recaps, newest first:\n%s", got)
	}

	m = keys(m, "j")
	if got := screen(m); !strings.Contains(got, "No recap yet") || !strings.Contains(got, "add tests for the parser") {
		t.Errorf("a session without recaps should show its last prompt:\n%s", got)
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
	m = keys(m, "tab", "tab", "tab") // login-1 is shown
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
