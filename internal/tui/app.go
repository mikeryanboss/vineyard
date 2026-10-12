// Package tui is Vineyard's terminal interface. The root Model in this file
// owns the sessions and every side effect; the views in its subpackages only
// render and emit messages.
package tui

import (
	"errors"
	"fmt"
	"image/color"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/Mibokess/grapes/embedded"
	"github.com/charmbracelet/x/ansi"
	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/git"
	"github.com/mikeryanboss/vineyard/internal/prompt"
	"github.com/mikeryanboss/vineyard/internal/recap"
	"github.com/mikeryanboss/vineyard/internal/session"
	"github.com/mikeryanboss/vineyard/internal/subagent"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
	"github.com/mikeryanboss/vineyard/internal/tui/dialog"
	"github.com/mikeryanboss/vineyard/internal/tui/diffview"
	"github.com/mikeryanboss/vineyard/internal/tui/list"
	"github.com/mikeryanboss/vineyard/internal/tui/preview"
	"github.com/mikeryanboss/vineyard/internal/tui/settings"
	"github.com/mikeryanboss/vineyard/internal/tui/textview"
)

const (
	// previewInterval paces captures of the selected session's screen.
	previewInterval = 150 * time.Millisecond
	// statusInterval paces the running/ready check of every session. Agents
	// animate continuously while working, so a screen unchanged for this
	// long means the agent is waiting.
	statusInterval = time.Second
	// diffInterval paces diff refreshes.
	diffInterval = 2 * time.Second
	// statusMessageDuration is how long a status message stays up.
	statusMessageDuration = 4 * time.Second
	// scrollbackLines is how much history scroll mode loads.
	scrollbackLines = 5000
	// doubleClickTime is how soon a second click on the same cell makes a
	// double click.
	doubleClickTime = 500 * time.Millisecond

	headerHeight    = 2
	statusBarHeight = 1
	minWidth        = 60
	minHeight       = 12
)

type focus int

const (
	focusList focus = iota
	focusPane
)

type tab int

const (
	tabPreview tab = iota
	tabDiff
	tabIssue
	tabRecap
)

// Messages produced by the model's own commands.
type (
	previewTickMsg struct{}
	statusTickMsg  struct{}
	diffTickMsg    struct{}
	screenMsg      struct {
		id     string
		screen string
		size   [2]int
		err    error
	}
	screenStatus struct {
		id         string
		hash       uint64
		permission bool
		trust      bool
		gone       bool
		// subagents is Claude Code's list of running subagents on the screen.
		subagents subagent.List
		// conversation is what Claude Code's transcripts say about them.
		conversation    subagent.Conversation
		conversationErr error
	}
	statusMsg struct {
		results []screenStatus
		// lead reports whether this process leads, and so types into agents.
		lead bool
		err  error
	}
	// reloadedMsg carries the sessions as saved when a reload began, which
	// had seen the given number of this process's writes.
	reloadedMsg struct {
		sessions []session.Session
		writes   int
		err      error
	}
	recapMsg struct {
		id    string
		recap recap.Recap
		err   error
	}
	diffMsg struct {
		selectedID string
		// agent is the subagent of selectedID whose diff raw is, or "".
		agent string
		// tree is the worktree that subagent works in, when it is not its
		// session's.
		tree   string
		hasRaw bool // whether raw was fetched; paused sessions have no worktree to diff
		raw    string
		rawErr error
		stats  map[string]git.Stat
		// branches holds each polled session's checked-out branch, which
		// the agent may have switched.
		branches map[string]string
	}
	// transcriptMsg carries the condensed transcript of a session's subagent.
	transcriptMsg struct {
		id, agent string
		events    []subagent.Event
		err       error
	}
	startedMsg struct {
		session session.Session
		err     error
	}
	lifecycleMsg struct {
		session session.Session
		verb    string
		err     error
	}
	killedMsg struct {
		session session.Session
		result  session.KillResult
		err     error
	}
	pushedMsg struct {
		session session.Session
		err     error
	}
	shellReadyMsg struct {
		session session.Session
		err     error
	}
	attachDoneMsg struct {
		id  string
		err error
	}
	// subagentOpenedMsg reports that a session shows a subagent's view, ready
	// to attach to, or why it does not.
	subagentOpenedMsg struct {
		id  string
		err error
	}
	// diffToolMsg carries the diff tool built for a session, or why there is none.
	diffToolMsg struct {
		id  string
		cmd *exec.Cmd
		err error
	}
	diffToolDoneMsg struct {
		err error
	}
	scrollbackMsg struct {
		id      string
		history string
		err     error
	}
	promptSentMsg struct {
		id  string
		err error
	}
	configSavedMsg struct {
		config config.Config
		err    error
	}
	savedMsg       struct{ err error }
	clearStatusMsg struct{ seq int }

	// jumpToSessionMsg closes the issues screen and selects a session.
	jumpToSessionMsg struct{ id string }
	// openIssueMsg shows an issue's detail on the issues screen.
	openIssueMsg struct{ id int }
	// templatesMsg carries the prompt templates read to start a session on
	// an issue.
	templatesMsg struct {
		issue     int
		templates []config.Template
		err       error
	}
)

// tick schedules timer messages. Tests replace it to drive the polling loops
// and status timeouts by hand.
var tick = tea.Tick

// now tells the time of a click. Tests replace it to space clicks apart.
var now = time.Now

// Options configure the model.
type Options struct {
	Config   config.Config
	RepoName string
	// RepoRoot is the repository's main checkout, which a relative
	// worktree_dir is resolved against.
	RepoRoot string
	Version  string
	// AutoYes turns auto-yes on for new sessions regardless of config.
	AutoYes bool
	// Program, when set, is the only agent offered for new sessions.
	Program string
	// ConfigPath is where the config screen saves, as shown to the user.
	ConfigPath string
	// ConfigErr is the error from loading the config file, if it failed.
	// Config then holds defaults, and the config screen refuses to save.
	ConfigErr error
	// Grapes is the repository's issue tracker, shown by the issues screen.
	// When it is nil, GrapesErr says why.
	Grapes    *embedded.Model
	GrapesErr error
	// Issue, when set, is a grapes issue to act on at startup as if the user
	// pressed the sessions key on it. Grapes must know the issue.
	Issue int
}

// Model is the root TUI model.
type Model struct {
	backend Backend
	opts    Options
	theme   common.Theme
	width   int
	height  int

	sessions []session.Session
	stats    map[string]git.Stat
	hashes   map[string]uint64 // last screen fingerprint per session
	busy     map[string]bool   // sessions with a lifecycle operation in flight
	sizes    map[string][2]int // tmux window size last applied per session
	shownID  string            // session whose screen and diff are displayed
	// subagents are the running subagents Claude Code lists on each
	// session's screen, as of the last status poll that is believed.
	// Opening one needs its place there.
	subagents map[string]subagent.List
	// fewer marks sessions whose last poll showed fewer subagents than are
	// shown, which the next poll must confirm (see seeSubagents).
	fewer map[string]bool
	// conversations are what Claude Code's transcripts said about each
	// session's subagents at the last status poll. The rows come from them.
	conversations map[string]subagent.Conversation

	previewInFlight bool
	statusInFlight  bool
	reloadInFlight  bool
	diffInFlight    bool
	// writes counts changes to the saved sessions this process has issued,
	// and pendingWrites those not yet finished. A reload is only started with
	// none pending, and dropped if another was issued while it ran.
	writes        int
	pendingWrites int

	list    list.Model
	preview preview.Model
	diff    diffview.Model
	issue   textview.Model
	recap   textview.Model
	dialog  dialog.Dialog
	focus   focus
	tab     tab
	// zoomed shows the pane in place of the list and pane.
	zoomed bool
	// recapShown is the latest recap fetched for the shown session, kept to
	// re-render the recap tab when the pane changes size or theme.
	recapShown recapMsg
	// shownAgent is the subagent of shownID whose transcript, diff, issues,
	// and recap are displayed, or "" for the session's own.
	shownAgent string
	// agentTree is the worktree the shown subagent works in, when it is not
	// its session's.
	agentTree string
	// transcript shows the shown subagent's transcript in place of the
	// preview. transcriptShown is the latest fetched, kept to re-render it.
	transcript         textview.Model
	transcriptShown    transcriptMsg
	transcriptInFlight bool
	// lastClick is the last click on the list or pane, to tell double clicks.
	lastClick clickAt

	// configOpen shows the config screen in place of the list and panes.
	configOpen bool
	settings   settings.Model

	// grapes is the issue tracker. It runs, and is sent every message vineyard
	// does not handle, even while hidden, so it keeps itself up to date.
	// grapesErr is set instead when the repository has none.
	grapes    embedded.Model
	grapesErr error
	// issuesOpen shows grapes in place of the whole screen.
	issuesOpen bool
	// startIssue is Options.Issue until the first window size arrives, which
	// the sessions dialogs need.
	startIssue int

	status      string
	statusIsErr bool
	statusSeq   int
}

// NewModel returns the root model showing sessions.
func NewModel(backend Backend, sessions []session.Session, opts Options) Model {
	theme := common.NewTheme(false) // light until the terminal reports its background
	m := Model{
		backend:  backend,
		opts:     opts,
		theme:    theme,
		sessions: sessions,
		stats:    map[string]git.Stat{},
		hashes:   map[string]uint64{},
		busy:     map[string]bool{},
		sizes:    map[string][2]int{},
		list:     list.New(theme),
		preview:  preview.New(theme),
		diff:     diffview.New(theme),
		issue:    textview.New(theme),
		recap:    textview.New(theme),

		transcript: textview.New(theme),

		subagents:     map[string]subagent.List{},
		fewer:         map[string]bool{},
		conversations: map[string]subagent.Conversation{},
		startIssue:    opts.Issue,
	}
	if opts.ConfigErr != nil {
		m.status, m.statusIsErr = "Config error (using defaults): "+opts.ConfigErr.Error(), true
	}
	switch {
	case opts.Grapes != nil:
		// Grapes starts dark until the terminal reports its background;
		// vineyard starts light. Tell grapes what vineyard assumes, so the two
		// match until the report arrives, which both then follow.
		m.grapes, _ = opts.Grapes.Update(tea.BackgroundColorMsg{Color: color.White})
	case opts.GrapesErr != nil:
		m.grapesErr = opts.GrapesErr
	default:
		m.grapesErr = errors.New("no issue tracker")
	}
	m.syncList()
	// Mark the initial selection as shown; the first ticks fetch its screen and diff.
	_ = m.refreshShown()
	m.diffInFlight = false // the fetch refreshShown planned was discarded above
	return m
}

// Init starts the polling loops.
func (m Model) Init() tea.Cmd {
	cmds := []tea.Cmd{
		tea.RequestBackgroundColor,
		func() tea.Msg { return previewTickMsg{} },
		func() tea.Msg { return statusTickMsg{} },
		func() tea.Msg { return diffTickMsg{} },
	}
	if m.grapesErr == nil {
		cmds = append(cmds, m.grapes.Init())
	}
	return tea.Batch(cmds...)
}

// --- Helpers ---

func (m Model) find(id string) int {
	return slices.IndexFunc(m.sessions, func(s session.Session) bool { return s.ID == id })
}

// selected returns the selected session.
func (m Model) selected() (session.Session, bool) {
	item, ok := m.list.Selected()
	if !ok {
		return session.Session{}, false
	}
	if i := m.find(item.Session.ID); i >= 0 {
		return m.sessions[i], true
	}
	return session.Session{}, false
}

// replace stores an updated session.
func (m *Model) replace(s session.Session) {
	if i := m.find(s.ID); i >= 0 {
		m.sessions[i] = s
	}
}

func (m *Model) remove(id string) {
	if i := m.find(id); i >= 0 {
		m.sessions = slices.Delete(m.sessions, i, i+1)
	}
	delete(m.stats, id)
	delete(m.hashes, id)
	delete(m.busy, id)
	delete(m.sizes, id)
	delete(m.subagents, id)
	delete(m.fewer, id)
	delete(m.conversations, id)
}

// syncList rebuilds the list entries from the sessions.
func (m *Model) syncList() {
	items := make([]list.Item, len(m.sessions))
	for i, s := range m.sessions {
		if m.busy[s.ID] {
			s.Status = session.StatusLoading
		}
		items[i] = list.Item{Session: s, Stat: m.stats[s.ID], Issues: m.issuesOf(s)}
		if s.Status.Active() {
			items[i].Subagents = m.conversations[s.ID].Kept()
			items[i].Shown = m.shownSubagent(s.ID)
		}
	}
	m.list = m.list.SetItems(items)
}

// layout returns the list width, pane width, and body height. A zoomed pane
// has the whole width.
func (m Model) layout() (listWidth, paneWidth, bodyHeight int) {
	bodyHeight = m.height - headerHeight - statusBarHeight
	if m.zoomed {
		return 0, m.width, bodyHeight
	}
	listWidth = max(26, min(44, m.width*3/10))
	paneWidth = m.width - listWidth
	return listWidth, paneWidth, bodyHeight
}

// paneContentSize is the size of the pane's content area: the pane minus its
// border and tab bar.
func (m Model) paneContentSize() (int, int) {
	_, paneWidth, bodyHeight := m.layout()
	return max(1, paneWidth-2), max(1, bodyHeight-3)
}

// agentSize is the size agents' tmux windows are kept at: the content area of
// the pane beside the list. Zooming leaves it alone, so agents never reflow.
func (m Model) agentSize() (int, int) {
	m.zoomed = false // m is a copy
	return m.paneContentSize()
}

func (m *Model) applySizes() {
	listWidth, _, bodyHeight := m.layout()
	w, h := m.paneContentSize()
	m.list = m.list.SetSize(listWidth, bodyHeight)
	m.preview = m.preview.SetSize(m.agentSize())
	m.diff = m.diff.SetSize(w, h)
	m.issue = m.issue.SetSize(w, h)
	m.recap = m.recap.SetSize(w, h)
	m.transcript = m.transcript.SetSize(w, h)
	m.settings = m.settings.SetSize(m.width, bodyHeight)
}

func (m *Model) applyTheme() {
	m.list = m.list.SetTheme(m.theme)
	m.preview = m.preview.SetTheme(m.theme)
	m.diff = m.diff.SetTheme(m.theme)
	m.issue = m.issue.SetTheme(m.theme)
	m.recap = m.recap.SetTheme(m.theme)
	m.transcript = m.transcript.SetTheme(m.theme)
	m.settings = m.settings.SetTheme(m.theme)
}

func (m *Model) setFocus(f focus) {
	m.focus = f
	m.list = m.list.SetFocused(f == focusList)
	if f == focusList {
		m.preview = m.preview.ExitScroll()
	}
}

// setZoom zooms the pane in or out, re-rendering what depends on its width.
func (m *Model) setZoom(zoomed bool) {
	m.zoomed = zoomed
	m.applySizes()
	m.refreshIssue()
	m.refreshRecap()
}

// setStatus shows a message and schedules its removal.
func (m *Model) setStatus(msg string, isErr bool) tea.Cmd {
	m.statusSeq++
	m.status, m.statusIsErr = msg, isErr
	seq := m.statusSeq
	return tick(statusMessageDuration, func(time.Time) tea.Msg { return clearStatusMsg{seq: seq} })
}

func (m *Model) setError(err error) tea.Cmd { return m.setStatus(err.Error(), true) }

// placeholder is what the preview shows instead of a screen, or "".
func (m Model) placeholder(s session.Session, ok bool) string {
	switch {
	case !ok:
		return "Press n to start a session."
	case m.busy[s.ID]:
		return "Working…"
	case s.Status == session.StatusLoading:
		return "Starting " + session.ProgramName(s.Program) + "…"
	case s.Status == session.StatusPaused:
		return "Paused. The work is committed to " + s.Branch + ".\nPress r to resume."
	case s.Status == session.StatusStopped:
		return "The agent is no longer running.\nPress r to restart it in its worktree."
	}
	return ""
}

// diffable reports whether s has a worktree in a state worth diffing.
func (m Model) diffable(s session.Session) bool {
	return s.Status != session.StatusPaused && s.Status != session.StatusLoading && !m.busy[s.ID]
}

// refreshShown updates the panes when the selected session changes, and
// fetches its screen and diff right away instead of waiting for a tick.
func (m *Model) refreshShown() tea.Cmd {
	s, ok := m.selected()
	a, _ := m.list.SelectedSubagent()
	if text := m.placeholder(s, ok); text != "" {
		m.preview = m.preview.SetPlaceholder(text)
	}
	if s.ID == m.shownID && a.ID == m.shownAgent {
		return nil
	}
	m.shownID, m.shownAgent, m.agentTree = s.ID, a.ID, ""
	m.preview = m.preview.ExitScroll()
	m.transcript = m.transcript.SetPlaceholder("Reading the transcript…")
	m.diff = m.diff.SetDiff("")
	m.issue = m.issue.GotoTop()
	m.refreshIssue()
	m.recap = m.recap.GotoTop()
	m.refreshRecap()
	if !ok {
		return nil
	}
	m.diffInFlight = true
	cmds := []tea.Cmd{m.diffCmd(), m.recapCmd()}
	switch {
	case a.ID != "":
		m.transcriptInFlight = true
		cmds = append(cmds, m.transcriptCmd())
	case s.Status.Active() && !m.busy[s.ID]: // otherwise there is no live screen to capture
		cmds = append(cmds, m.captureCmd(s))
	}
	return tea.Batch(cmds...)
}

// writeCmd changes the saved sessions, which other Vineyards on the
// repository share.
func (m *Model) writeCmd(write func(Backend) error) tea.Cmd {
	m.writes++
	m.pendingWrites++
	backend := m.backend
	return func() tea.Msg {
		if err := write(backend); err != nil {
			return savedMsg{err: fmt.Errorf("saving sessions: %w", err)}
		}
		return savedMsg{}
	}
}

func (m *Model) replaceCmd(sessions ...session.Session) tea.Cmd {
	return m.writeCmd(func(b Backend) error {
		for _, s := range sessions {
			if err := b.Replace(s); err != nil {
				return err
			}
		}
		return nil
	})
}

func (m Model) reloadCmd() tea.Cmd {
	backend := m.backend
	writes := m.writes
	worktreeDir, err := config.ResolveWorktreeDir(m.opts.RepoRoot, m.opts.Config.WorktreeDir)
	return func() tea.Msg {
		if err != nil {
			return reloadedMsg{writes: writes, err: fmt.Errorf("worktree directory: %w", err)}
		}
		sessions, err := backend.Load(worktreeDir)
		return reloadedMsg{sessions: sessions, writes: writes, err: err}
	}
}

// adopt takes the sessions as saved, which other Vineyards on the repository
// change too. A session this process is starting or operating on keeps its
// local state; the operation saves its result. While a session is active both
// here and as saved, the local running or ready state wins: this process's
// own screen polls decide it.
func (m *Model) adopt(saved []session.Session) {
	var next []session.Session
	for _, s := range saved {
		if i := m.find(s.ID); i >= 0 {
			local := m.sessions[i]
			if m.busy[s.ID] {
				next = append(next, local)
				continue
			}
			if local.Status.Active() && s.Status.Active() {
				s.Status = local.Status
			}
			if s.Status != local.Status {
				// Paused, resumed, or stopped elsewhere: start over as after
				// this process's own lifecycle operations.
				delete(m.hashes, s.ID)
				delete(m.sizes, s.ID)
				delete(m.subagents, s.ID)
				delete(m.conversations, s.ID)
				if s.ID == m.shownID {
					m.shownID = ""
				}
			}
		}
		next = append(next, s)
	}
	var gone []string
	for _, s := range m.sessions {
		if slices.ContainsFunc(saved, func(o session.Session) bool { return o.ID == s.ID }) {
			continue
		}
		if m.busy[s.ID] || s.Status == session.StatusLoading {
			next = append(next, s) // not saved until it has started
		} else {
			gone = append(gone, s.ID)
		}
	}
	m.sessions = next
	for _, id := range gone {
		m.remove(id) // forgets its fingerprints, sizes, and stats
	}
}

// --- Commands ---

func (m Model) captureCmd(s session.Session) tea.Cmd {
	backend := m.backend
	w, h := m.agentSize()
	applied := m.sizes[s.ID]
	return func() tea.Msg {
		size := applied
		if size != [2]int{w, h} {
			if err := backend.Resize(s, w, h); err == nil {
				size = [2]int{w, h}
			}
		}
		screen, err := backend.Capture(s)
		return screenMsg{id: s.ID, screen: screen, size: size, err: err}
	}
}

func (m Model) statusCmd() tea.Cmd {
	backend := m.backend
	var active []session.Session
	for _, s := range m.sessions {
		if s.Status.Active() && !m.busy[s.ID] {
			active = append(active, s)
		}
	}
	return func() tea.Msg {
		lead, err := backend.Lead()
		if err != nil {
			err = fmt.Errorf("claiming the lead: %w", err)
		}
		results := make([]screenStatus, len(active))
		done := make(chan struct{}, len(active))
		for i, s := range active {
			go func() {
				defer func() { done <- struct{}{} }()
				screen, err := backend.Capture(s)
				if err != nil {
					results[i] = screenStatus{id: s.ID, gone: true}
					return
				}
				results[i] = screenStatus{
					id:         s.ID,
					hash:       session.ScreenHash(screen),
					permission: session.AwaitingPermission(s.Program, screen),
					trust:      session.AwaitingTrust(screen),
				}
				if isClaude(s) {
					results[i].subagents, _ = subagent.Parse(screen)
					results[i].conversation, results[i].conversationErr = backend.Subagents(s)
				}
			}()
		}
		for range active {
			<-done
		}
		return statusMsg{results: results, lead: lead, err: err}
	}
}

// diffCmd fetches the full diff of the selected session and the line counts
// of all others with a worktree.
func (m Model) diffCmd() tea.Cmd {
	backend := m.backend
	selected, hasSelected := m.selected()
	agent, ofAgent := m.list.SelectedSubagent()
	var others []session.Session
	for _, s := range m.sessions {
		if s.ID != selected.ID && (s.Status.Active() || s.Status == session.StatusStopped) && !m.busy[s.ID] {
			others = append(others, s)
		}
	}
	showDiff := hasSelected && m.diffable(selected)
	return func() tea.Msg {
		msg := diffMsg{selectedID: selected.ID, agent: agent.ID, hasRaw: showDiff, stats: map[string]git.Stat{}, branches: map[string]string{}}
		if showDiff && ofAgent && agent.Dir != "" {
			// A subagent working in another worktree has that one's diff.
			if tree := backend.Worktree(agent.Dir); tree != "" && tree != selected.WorktreePath {
				msg.tree = tree
			}
		}
		if msg.tree != "" {
			msg.raw, msg.rawErr = backend.TreeDiff(msg.tree)
		} else if showDiff {
			msg.raw, msg.rawErr = backend.Diff(selected)
			if branch, err := backend.Branch(selected); err == nil {
				msg.branches[selected.ID] = branch
			}
		}
		for _, s := range others {
			if stat, err := backend.DiffStat(s); err == nil {
				msg.stats[s.ID] = stat
			}
			if branch, err := backend.Branch(s); err == nil {
				msg.branches[s.ID] = branch
			}
		}
		return msg
	}
}

// recapCmd reads the selected session's recaps, while the recap tab is shown.
func (m Model) recapCmd() tea.Cmd {
	s, ok := m.selected()
	if m.tab != tabRecap || !ok || !isClaude(s) || m.shownAgent != "" {
		return nil
	}
	backend := m.backend
	return func() tea.Msg {
		r, err := backend.Recap(s)
		return recapMsg{id: s.ID, recap: r, err: err}
	}
}

// transcriptCmd reads the transcript of the selected subagent.
func (m Model) transcriptCmd() tea.Cmd {
	s, _ := m.selected()
	a, _ := m.list.SelectedSubagent()
	backend := m.backend
	return func() tea.Msg {
		events, err := backend.SubagentEvents(s, a.ID)
		return transcriptMsg{id: s.ID, agent: a.ID, events: events, err: err}
	}
}

func (m Model) startCmd(s session.Session) tea.Cmd {
	backend := m.backend
	w, h := m.agentSize()
	return func() tea.Msg {
		started, err := backend.Start(s, w, h)
		return startedMsg{session: started, err: err}
	}
}

func (m Model) promptCmd(s session.Session, prompt string) tea.Cmd {
	backend := m.backend
	return func() tea.Msg {
		if err := backend.Paste(s, prompt); err != nil {
			return promptSentMsg{id: s.ID, err: err}
		}
		// Give the agent a moment to take in the paste before submitting it.
		time.Sleep(150 * time.Millisecond)
		return promptSentMsg{id: s.ID, err: backend.SendEnter(s)}
	}
}

// shownSubagent returns the ID of the subagent whose view the session with
// id shows in place of its conversation, or "" when it shows the
// conversation or Claude Code's list cannot be read.
func (m Model) shownSubagent(id string) string {
	l := m.subagents[id]
	ids, ok := subagent.Match(m.conversations[id].Agents, l, now())
	if !ok || l.Shown <= 0 {
		return ""
	}
	return ids[l.Shown-1]
}

// openSubagent makes s show the view of its subagent a, which needs a's
// place in Claude Code's list.
func (m Model) openSubagent(s session.Session, a subagent.Agent) (tea.Model, tea.Cmd) {
	l := m.subagents[s.ID]
	ids, ok := subagent.Match(m.conversations[s.ID].Agents, l, now())
	position := slices.Index(ids, a.ID) + 1
	switch {
	case ok && position > 0:
	case a.Status != subagent.Running:
		return m, m.setStatus(a.Description+" has finished, and Claude Code no longer lists it. Ask the conversation to message it to resume it.", false)
	default:
		return m, m.setStatus("Claude Code's list of subagents does not match its transcripts yet. Try again, or open it in Claude Code.", false)
	}
	backend := m.backend
	// Opening types into the agent at the user's request, so every Vineyard
	// may do it, not only the one that leads.
	return m, tea.Batch(m.setStatus("Opening the subagent…", false), func() tea.Msg {
		return subagentOpenedMsg{id: s.ID, err: backend.OpenSubagent(s, position, l.Len())}
	})
}

func (m Model) enterCmd(s session.Session) tea.Cmd {
	backend := m.backend
	return func() tea.Msg {
		_ = backend.SendEnter(s)
		return nil
	}
}

// --- Update ---

// Update handles a message.
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.applySizes()
		m.refreshRecap()
		m.refreshTranscript()
		if id := m.startIssue; id != 0 {
			m.startIssue = 0
			next, cmd := m.updateGrapes(msg)
			shown, sessionsCmd := next.showSessions(id)
			return shown, tea.Batch(cmd, sessionsCmd)
		}
		return m.updateGrapes(msg)

	case tea.BackgroundColorMsg:
		m.theme = common.NewTheme(msg.IsDark())
		m.applyTheme()
		m.refreshRecap()
		m.refreshTranscript()
		return m.updateGrapes(msg)

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.MouseClickMsg:
		return m.handleMouse(msg)
	case tea.MouseWheelMsg:
		return m.handleMouse(msg)

	case previewTickMsg:
		next := tick(previewInterval, func(time.Time) tea.Msg { return previewTickMsg{} })
		if m.shownAgent != "" {
			if m.transcriptInFlight {
				return m, next
			}
			m.transcriptInFlight = true
			return m, tea.Batch(next, m.transcriptCmd())
		}
		s, ok := m.selected()
		if !ok || !s.Status.Active() || m.busy[s.ID] || m.previewInFlight || m.preview.Scrolling() {
			return m, next
		}
		m.previewInFlight = true
		return m, tea.Batch(next, m.captureCmd(s))

	case transcriptMsg:
		m.transcriptInFlight = false
		if msg.id == m.shownID && msg.agent == m.shownAgent {
			first := m.transcriptShown.id != msg.id || m.transcriptShown.agent != msg.agent
			m.transcriptShown = msg
			m.refreshTranscript()
			if first {
				m.transcript = m.transcript.GotoBottom()
			}
		}
		return m, nil

	case screenMsg:
		m.previewInFlight = false
		if msg.err == nil {
			m.sizes[msg.id] = msg.size
			if msg.id == m.shownID && !m.preview.Scrolling() {
				m.preview = m.preview.SetScreen(msg.screen)
			}
		}
		return m, nil

	case statusTickMsg:
		cmds := []tea.Cmd{tick(statusInterval, func(time.Time) tea.Msg { return statusTickMsg{} })}
		if !m.statusInFlight {
			m.statusInFlight = true
			cmds = append(cmds, m.statusCmd())
		}
		if !m.reloadInFlight && m.pendingWrites == 0 {
			m.reloadInFlight = true
			cmds = append(cmds, m.reloadCmd())
		}
		return m, tea.Batch(cmds...)

	case statusMsg:
		m.statusInFlight = false
		return m.applyStatus(msg)

	case diffTickMsg:
		next := tick(diffInterval, func(time.Time) tea.Msg { return diffTickMsg{} })
		if m.diffInFlight {
			return m, next
		}
		m.diffInFlight = true
		return m, tea.Batch(next, m.diffCmd(), m.recapCmd())

	case diffMsg:
		m.diffInFlight = false
		shown, _ := m.selected()
		shownIssues := m.shownIssues(shown)
		for id, stat := range msg.stats {
			m.stats[id] = stat
		}
		for id, branch := range msg.branches {
			if i := m.find(id); i >= 0 {
				m.sessions[i].Branch = branch
			}
		}
		if msg.selectedID == m.shownID && msg.agent == m.shownAgent && msg.hasRaw && msg.rawErr == nil {
			m.diff = m.diff.SetDiff(msg.raw)
			m.agentTree = msg.tree
			if msg.tree == "" {
				var stat git.Stat
				for _, f := range m.diff.Files() {
					stat.Added += f.Added
					stat.Removed += f.Removed
					stat.Paths = append(stat.Paths, f.OldPath, f.NewPath)
				}
				m.stats[msg.selectedID] = stat
			}
		}
		m.syncList()
		if !slices.Equal(m.shownIssues(shown), shownIssues) {
			m.refreshIssue()
		}
		return m, nil

	case recapMsg:
		if msg.id == m.shownID {
			m.recapShown = msg
			m.refreshRecap()
		}
		return m, nil

	case startedMsg:
		delete(m.busy, msg.session.ID)
		if msg.err != nil {
			m.remove(msg.session.ID)
			m.syncList()
			return m, tea.Batch(m.setError(msg.err), m.refreshShown())
		}
		m.replace(msg.session)
		m.syncList()
		m.shownID = "" // force a fresh capture now that the session runs
		started := msg.session
		write := m.writeCmd(func(b Backend) error { return b.Add(started) })
		return m, tea.Batch(write, m.refreshShown())

	case lifecycleMsg:
		delete(m.busy, msg.session.ID)
		var cmds []tea.Cmd
		if msg.err != nil {
			cmds = append(cmds, m.setError(msg.err))
		} else {
			m.replace(msg.session)
			delete(m.hashes, msg.session.ID)
			delete(m.subagents, msg.session.ID)
			delete(m.conversations, msg.session.ID)
			cmds = append(cmds, m.replaceCmd(msg.session))
			switch msg.verb {
			case "pause":
				cmds = append(cmds, m.setStatus("Checked out: "+msg.session.Branch+" is free to use elsewhere.", false))
			case "resume":
				delete(m.sizes, msg.session.ID)
				cmds = append(cmds, m.setStatus("Resumed "+msg.session.Title+".", false))
			}
		}
		m.syncList()
		m.shownID = ""
		return m, tea.Batch(append(cmds, m.refreshShown())...)

	case killedMsg:
		if msg.err != nil {
			delete(m.busy, msg.session.ID)
			m.syncList()
			var lost *session.LostCommitsError
			if errors.As(msg.err, &lost) {
				lines := lost.Commits
				if len(lines) > 5 {
					lines = append(lines[:5:5], fmt.Sprintf("… and %d more", len(lost.Commits)-5))
				}
				m.dialog = dialog.NewConfirm(m.theme,
					fmt.Sprintf("Kill %q anyway? The commits below exist only on its detached HEAD and are lost with its worktree. To keep them, cancel, press t, and run git switch -c <branch>.", msg.session.Title),
					common.ActionKillDiscardingCommits, msg.session.ID, lines...)
				return m, nil
			}
			return m, m.setError(msg.err)
		}
		m.remove(msg.session.ID)
		m.syncList()
		text := "Killed " + msg.session.Title + "."
		if msg.result.KeptBranch != "" {
			text = "Killed " + msg.session.Title + ". Its commits are kept on " + msg.result.KeptBranch + "."
		}
		id := msg.session.ID
		write := m.writeCmd(func(b Backend) error { return b.Remove(id) })
		return m, tea.Batch(write, m.setStatus(text, false), m.refreshShown())

	case pushedMsg:
		delete(m.busy, msg.session.ID)
		m.replace(msg.session) // carries the branch Push read from git
		m.syncList()
		if msg.err != nil {
			return m, tea.Batch(m.setError(msg.err), m.refreshShown())
		}
		m.shownID = ""
		return m, tea.Batch(m.setStatus("Pushed "+msg.session.Branch+" to origin.", false), m.refreshShown())

	case shellReadyMsg:
		if msg.err != nil {
			return m, m.setError(msg.err)
		}
		id := msg.session.ID
		return m, tea.ExecProcess(m.backend.ShellAttachCommand(msg.session), func(err error) tea.Msg {
			return attachDoneMsg{id: id, err: err}
		})

	case attachDoneMsg:
		// The window followed the attached client's size; restore the preview size.
		delete(m.sizes, msg.id)
		var cmds []tea.Cmd
		if msg.err != nil {
			cmds = append(cmds, m.setError(fmt.Errorf("attach: %w", msg.err)))
		}
		if s, ok := m.selected(); ok && s.Status.Active() {
			cmds = append(cmds, m.captureCmd(s))
		}
		return m, tea.Batch(cmds...)

	case subagentOpenedMsg:
		if msg.err != nil {
			return m, m.setError(fmt.Errorf("opening the subagent: %w", msg.err))
		}
		i := m.find(msg.id)
		if i < 0 || !m.sessions[i].Status.Active() {
			return m, nil // killed or stopped while its subagent opened
		}
		m.status = ""
		id := msg.id
		return m, tea.ExecProcess(m.backend.AttachCommand(m.sessions[i]), func(err error) tea.Msg {
			return attachDoneMsg{id: id, err: err}
		})

	case diffToolMsg:
		if s, ok := m.selected(); !ok || s.ID != msg.id || m.tab != tabDiff {
			return m, nil
		}
		if errors.Is(msg.err, exec.ErrNotFound) {
			m.setFocus(focusPane)
			m.setZoom(true)
			return m, nil
		}
		if msg.err != nil {
			return m, m.setError(fmt.Errorf("diff tool: %w", msg.err))
		}
		return m, tea.ExecProcess(msg.cmd, func(err error) tea.Msg { return diffToolDoneMsg{err: err} })

	case diffToolDoneMsg:
		if msg.err != nil {
			return m, m.setError(fmt.Errorf("diff tool: %w", msg.err))
		}
		return m, nil

	case scrollbackMsg:
		if msg.err != nil {
			return m, m.setError(msg.err)
		}
		if msg.id != m.shownID {
			return m, nil
		}
		if m.preview = m.preview.SetScrollback(msg.history); !m.preview.Scrolling() {
			return m, m.setStatus("No scrollback: tmux keeps none for an agent that draws fullscreen. Attach to scroll in the agent.", false)
		}
		return m, nil

	case promptSentMsg:
		if msg.err != nil {
			return m, m.setError(fmt.Errorf("sending prompt: %w", msg.err))
		}
		return m, nil

	case configSavedMsg:
		if msg.err != nil {
			return m, m.setError(fmt.Errorf("saving config: %w", msg.err))
		}
		m.opts.Config = msg.config
		m.configOpen = false
		return m, m.setStatus("Saved "+m.opts.ConfigPath+". New sessions use the new settings.", false)

	case savedMsg:
		m.pendingWrites--
		if msg.err != nil {
			return m, m.setError(msg.err)
		}
		return m, nil

	case reloadedMsg:
		m.reloadInFlight = false
		if msg.err != nil {
			return m, m.setError(fmt.Errorf("reloading sessions: %w", msg.err))
		}
		if msg.writes != m.writes {
			return m, nil // may predate this process's own change; the next tick reloads
		}
		m.adopt(msg.sessions)
		m.syncList()
		return m, m.refreshShown()

	case clearStatusMsg:
		if msg.seq == m.statusSeq {
			m.status = ""
		}
		return m, nil

	case common.NewSessionMsg:
		m.dialog = nil
		return m.createSession(msg)

	case embedded.CloseMsg:
		m.issuesOpen = false
		return m, nil

	case embedded.SessionsMsg:
		return m.showSessions(msg.IssueID)

	case templatesMsg:
		if msg.err != nil {
			return m, m.setError(fmt.Errorf("reading prompt templates: %w", msg.err))
		}
		issue, _ := m.grapes.Issue(msg.issue)
		d, cmd := dialog.NewSessionDialog(m.theme, m.profiles(), true, min(70, m.width-4))
		m.dialog = d.ForIssue(issue.Title, msg.templates, m.promptData(issue), m.height-statusBarHeight)
		return m, cmd

	case jumpToSessionMsg:
		m.dialog = nil
		m.issuesOpen = false
		m.list = m.list.Select(msg.id)
		m.setFocus(focusList)
		return m, m.refreshShown()

	case openIssueMsg:
		m.dialog = nil
		return m.openIssue(msg.id)

	case common.ConfirmedMsg:
		m.dialog = nil
		return m.runConfirmed(msg)

	case common.DialogCancelledMsg:
		m.dialog = nil
		return m, nil

	case common.SaveConfigMsg:
		backend, cfg := m.backend, msg.Config
		return m, func() tea.Msg { return configSavedMsg{config: cfg, err: backend.SaveConfig(cfg)} }

	case common.CloseConfigMsg:
		m.configOpen = false
		return m, nil

	case common.LeavePaneMsg:
		m.setFocus(focusList)
		m.setZoom(false)
		return m, nil

	case common.SwitchTabMsg:
		m.setZoom(false)
		return m, m.switchTab(msg.Back)

	case common.ScrollbackRequestMsg:
		s, ok := m.selected()
		if !ok || !s.Status.Active() {
			return m, nil
		}
		backend := m.backend
		return m, func() tea.Msg {
			history, err := backend.CaptureHistory(s, scrollbackLines)
			return scrollbackMsg{id: s.ID, history: history, err: err}
		}
	}

	// Anything else, such as cursor blinks, belongs to an open dialog or the
	// config screen, and to grapes, whose file watching and reloads report
	// back through vineyard's update loop.
	var cmd tea.Cmd
	switch {
	case m.dialog != nil:
		m.dialog, cmd = m.dialog.Update(msg)
	case m.configOpen:
		m.settings, cmd = m.settings.Update(msg)
	}
	next, grapesCmd := m.updateGrapes(msg)
	return next, tea.Batch(cmd, grapesCmd)
}

// updateGrapes passes msg to grapes, when there is one. A grapes reload may
// change what issues say, so the issue tab is rebuilt.
func (m Model) updateGrapes(msg tea.Msg) (Model, tea.Cmd) {
	if m.grapesErr != nil {
		return m, nil
	}
	var cmd tea.Cmd
	m.grapes, cmd = m.grapes.Update(msg)
	m.refreshIssue()
	return m, cmd
}

// refreshIssue renders the selected session's issues into the issue tab, while
// the tab is shown. Rendering markdown is too slow to repeat for every frame,
// so it happens only when the issues, their session, or the pane change.
func (m *Model) refreshIssue() {
	if m.tab != tabIssue {
		return
	}
	s, ok := m.selected()
	switch {
	case !ok:
		m.issue = m.issue.SetPlaceholder("Press n to start a session.")
		return
	case m.grapesErr != nil:
		m.issue = m.issue.SetPlaceholder("No issues to show: " + m.grapesErr.Error())
		return
	}
	width, _ := m.paneContentSize()
	tree := s.WorktreePath
	if m.agentTree != "" {
		tree = m.agentTree
	}
	var rendered []string
	for _, id := range m.shownIssues(s) {
		if text, ok := m.grapes.RenderIssue(id, tree, width); ok {
			rendered = append(rendered, text)
		}
	}
	switch {
	case len(rendered) == 0 && m.shownAgent != "":
		m.issue = m.issue.SetPlaceholder("This subagent has changed no issue. Press i to browse issues.")
		return
	case len(rendered) == 0:
		m.issue = m.issue.SetPlaceholder("No issue linked. Press i to browse issues.")
		return
	}
	rule := m.theme.StyleSeparator.Render(strings.Repeat("─", width))
	m.issue = m.issue.SetContent(strings.Join(rendered, "\n"+rule+"\n"))
}

// refreshRecap renders the shown session's latest recaps into the recap tab,
// while the tab is shown.
func (m *Model) refreshRecap() {
	if m.tab != tabRecap {
		return
	}
	s, ok := m.selected()
	r := m.recapShown
	if a, ok := m.list.SelectedSubagent(); ok {
		m.recap = m.recap.SetContent(m.renderAgentRecap(a))
		return
	}
	switch {
	case !ok:
		m.recap = m.recap.SetPlaceholder("Press n to start a session.")
		return
	case !isClaude(s):
		m.recap = m.recap.SetPlaceholder("Recaps come from Claude Code; this session runs " + session.ProgramName(s.Program) + ".")
		return
	case r.id != s.ID:
		m.recap = m.recap.SetPlaceholder("Reading recaps…")
		return
	case r.err != nil:
		m.recap = m.recap.SetPlaceholder("Cannot read recaps: " + r.err.Error())
		return
	case !r.recap.Found:
		m.recap = m.recap.SetPlaceholder("Claude Code has no transcript for this session yet.")
		return
	}
	width, _ := m.paneContentSize()
	t := m.theme
	var b strings.Builder
	if r.recap.Title != "" {
		b.WriteString(t.StyleTitle.Render(ansi.Wrap(r.recap.Title, width, "")) + "\n\n")
	}
	for _, e := range r.recap.Entries {
		b.WriteString(t.StyleFaint.Render(e.Time.Local().Format("Jan 2 15:04")) + "\n")
		b.WriteString(ansi.Wrap(e.Text, width, "") + "\n\n")
	}
	if len(r.recap.Entries) == 0 {
		b.WriteString(t.StyleSubtitle.Render(ansi.Wrap("No recap yet. Claude Code writes one when you return after being away.", width, "")) + "\n\n")
		if r.recap.LastPrompt != "" {
			b.WriteString(t.StyleFaint.Render("Last prompt") + "\n")
			b.WriteString(ansi.Wrap(r.recap.LastPrompt, width, "") + "\n")
		}
	}
	m.recap = m.recap.SetContent(strings.TrimRight(b.String(), "\n"))
}

// refreshTranscript renders the shown subagent's transcript, as Claude Code
// shows a conversation: its messages, and each tool call with the first
// lines of its result. It keeps the scroll position, following new lines
// while scrolled to the bottom.
func (m *Model) refreshTranscript() {
	r := m.transcriptShown
	if r.id != m.shownID || r.agent != m.shownAgent || m.shownAgent == "" {
		return
	}
	if r.err != nil {
		m.transcript = m.transcript.SetPlaceholder("Cannot read the transcript: " + r.err.Error())
		return
	}
	width, _ := m.paneContentSize()
	t := m.theme
	// block renders text after a marker, its later lines indented under the
	// first.
	block := func(marker string, style lipgloss.Style, text string) string {
		indent := ansi.StringWidth(marker)
		lines := strings.Split(ansi.Wrap(text, max(1, width-indent), ""), "\n")
		for i, line := range lines {
			lines[i] = style.Render(line)
		}
		return marker + strings.Join(lines, "\n"+strings.Repeat(" ", indent))
	}
	var parts []string
	for i, e := range r.events {
		switch e.Kind {
		case subagent.Prompt:
			parts = append(parts, block(t.StyleTitle.Render("❯ "), lipgloss.NewStyle(), e.Text))
		case subagent.Text:
			parts = append(parts, block(t.StyleTitle.Render("● "), lipgloss.NewStyle(), e.Text))
		case subagent.Tool:
			call := t.StyleTitle.Render(e.Name)
			if e.Text != "" {
				call += t.StyleSubtitle.Render("(" + strings.ReplaceAll(e.Text, "\n", " ") + ")")
			}
			parts = append(parts, lipgloss.NewStyle().Foreground(t.ColorAccent).Render("● ")+ansi.Truncate(call, max(1, width-2), "…"))
		case subagent.Result:
			style := t.StyleFaint
			if e.Error {
				style = t.StyleError
			}
			result := block(style.Render("  ⎿ "), style, e.Text)
			if i > 0 && r.events[i-1].Kind == subagent.Tool {
				parts[len(parts)-1] += "\n" + result // under its call
			} else {
				parts = append(parts, result)
			}
		}
	}
	if len(parts) == 0 {
		m.transcript = m.transcript.SetPlaceholder("The subagent has written nothing yet.")
		return
	}
	following := m.transcript.AtBottom()
	m.transcript = m.transcript.SetContent(strings.Join(parts, "\n\n"))
	if following {
		m.transcript = m.transcript.GotoBottom()
	}
}

// statusNames are what the recap calls a subagent's status.
var statusNames = map[subagent.Status]string{subagent.Running: "running", subagent.Done: "done", subagent.Stopped: "stopped"}

// renderAgentRecap renders what a subagent was asked and what it reported.
func (m Model) renderAgentRecap(a subagent.Agent) string {
	width, _ := m.paneContentSize()
	t := m.theme
	var b strings.Builder
	b.WriteString(t.StyleTitle.Render(ansi.Wrap(a.Description, width, "")) + "\n")
	facts := a.Type + " · " + statusNames[a.Status] + " · started " + a.Started.Local().Format("Jan 2 15:04")
	if a.Status != subagent.Running {
		facts += ", ended " + a.Ended.Local().Format("15:04")
	}
	b.WriteString(t.StyleFaint.Render(ansi.Wrap(facts, width, "")) + "\n\n")
	b.WriteString(t.StyleFaint.Render("Report") + "\n")
	switch {
	case a.Report != "":
		b.WriteString(ansi.Wrap(a.Report, width, "") + "\n\n")
	default:
		b.WriteString(t.StyleSubtitle.Render("Nothing reported yet.") + "\n\n")
	}
	b.WriteString(t.StyleFaint.Render("Prompt") + "\n")
	b.WriteString(ansi.Wrap(a.Prompt, width, ""))
	return b.String()
}

// isClaude reports whether s runs Claude Code, the only agent whose recaps
// vineyard reads.
func isClaude(s session.Session) bool { return session.ProgramName(s.Program) == "claude" }

// issuesOf returns the grapes issues session s works on: the one it was
// started for and those whose files its diff changes, in ascending order.
//
// The diff runs from where the branch's own work begins (git.Base), so the
// issues stay linked after the branch merges. Grapes' TouchedIssues compares
// against the default branch's tip instead and loses them then.
func (m Model) issuesOf(s session.Session) []int {
	var ids []int
	if s.Issue > 0 {
		ids = append(ids, s.Issue)
	}
	return issueIDs(ids, m.stats[s.ID].Paths)
}

// shownIssues returns the issues of what the pane shows for s: those of s,
// or of its subagent shown. A subagent in a worktree of its own has the
// issues of that worktree's diff; one in its session's, those whose files it
// edited.
func (m Model) shownIssues(s session.Session) []int {
	a, ok := m.list.SelectedSubagent()
	if !ok || a.ID != m.shownAgent {
		return m.issuesOf(s)
	}
	var paths []string
	if m.agentTree != "" {
		for _, f := range m.diff.Files() {
			paths = append(paths, f.OldPath, f.NewPath)
		}
		return issueIDs(nil, paths)
	}
	for _, path := range a.Edited {
		if rel, err := filepath.Rel(s.WorktreePath, path); err == nil {
			paths = append(paths, rel)
		}
	}
	return issueIDs(nil, paths)
}

// issueIDs adds to ids the grapes issues whose files paths, relative to a
// worktree, lie in, and sorts them.
func issueIDs(ids []int, paths []string) []int {
	for _, path := range paths {
		rest, ok := strings.CutPrefix(path, ".grapes/")
		if !ok {
			continue
		}
		dir, _, _ := strings.Cut(rest, "/")
		if id, err := strconv.Atoi(dir); err == nil && id > 0 && !slices.Contains(ids, id) {
			ids = append(ids, id)
		}
	}
	slices.Sort(ids)
	return ids
}

// showSessions answers grapes' request for the sessions working on an issue.
// A single session is selected directly; several are offered in a picker;
// none means the user wants to start one.
func (m Model) showSessions(issueID int) (tea.Model, tea.Cmd) {
	var found []session.Session
	for _, s := range m.sessions {
		if slices.Contains(m.issuesOf(s), issueID) {
			found = append(found, s)
		}
	}
	width := min(70, m.width-4)
	switch len(found) {
	case 0:
		// The dialog opens once the templates are read.
		backend, inline := m.backend, m.opts.Config.Templates
		return m, func() tea.Msg {
			templates, err := backend.Templates(inline)
			return templatesMsg{issue: issueID, templates: templates, err: err}
		}
	case 1:
		return m.Update(jumpToSessionMsg{id: found[0].ID})
	}
	choices := make([]dialog.Choice, len(found))
	for i, s := range found {
		choices[i] = dialog.Choice{
			Label: common.StatusIcon(s.Status) + " " + s.Title + "  " + s.Branch,
			Msg:   jumpToSessionMsg{id: s.ID},
		}
	}
	m.dialog = dialog.NewPick(m.theme, fmt.Sprintf("Sessions working on #%d", issueID), choices, width)
	return m, nil
}

// promptData describes issue to the templates of its sessions' prompts.
func (m Model) promptData(issue embedded.Issue) prompt.Data {
	ref := func(id int) prompt.Issue {
		iss, _ := m.grapes.Issue(id)
		return prompt.Issue{ID: id, Title: iss.Title, Status: iss.Status}
	}
	data := prompt.Data{Issue: ref(issue.ID), Labels: issue.Labels}
	if issue.Parent > 0 {
		parent := ref(issue.Parent)
		data.Parent = &parent
	}
	for _, id := range issue.Children {
		sub := prompt.SubIssue{Issue: ref(id)}
		for _, s := range m.sessions {
			if slices.Contains(m.issuesOf(s), id) {
				sub.Branch = s.Branch
				break
			}
		}
		data.SubIssues = append(data.SubIssues, sub)
	}
	for _, id := range issue.BlockedBy {
		if blocker := ref(id); blocker.Status != "done" && blocker.Status != "cancelled" {
			data.Blockers = append(data.Blockers, blocker)
		}
	}
	return data
}

// showIssues opens the issues screen at the issue session s works on. With
// several, a picker chooses; with none, or no session, grapes opens as it was.
func (m Model) showIssues(s session.Session, ok bool) (tea.Model, tea.Cmd) {
	if m.grapesErr != nil {
		return m, m.setStatus("No issues to show: "+m.grapesErr.Error(), true)
	}
	var ids []int
	if ok {
		ids = m.shownIssues(s)
	}
	switch len(ids) {
	case 0:
		m.issuesOpen = true
		return m, nil
	case 1:
		return m.openIssue(ids[0])
	}
	choices := make([]dialog.Choice, len(ids))
	for i, id := range ids {
		issue, _ := m.grapes.Issue(id)
		choices[i] = dialog.Choice{Label: fmt.Sprintf("#%d %s", id, issue.Title), Msg: openIssueMsg{id: id}}
	}
	m.dialog = dialog.NewPick(m.theme, "Issues of "+s.Title, choices, min(70, m.width-4))
	return m, nil
}

// openIssue shows grapes at issue id.
func (m Model) openIssue(id int) (tea.Model, tea.Cmd) {
	m.issuesOpen = true
	var cmd tea.Cmd
	m.grapes, cmd = m.grapes.OpenIssue(id)
	return m, cmd
}

// seeSubagents records the subagents a status poll found on a session's
// screen. Claude Code draws its agent list last, so a poll that catches it
// redrawing, as right after a detach, sees fewer subagents than there are,
// often none. A poll showing fewer is believed only when the next one
// agrees; until then the shown list stays.
func (m *Model) seeSubagents(id string, seen subagent.List) {
	if seen.Len() < m.subagents[id].Len() && !m.fewer[id] {
		m.fewer[id] = true
		return
	}
	m.subagents[id] = seen
	delete(m.fewer, id)
}

// applyStatus updates running/ready states from fresh screen fingerprints.
// If this process leads, it also answers prompts for auto-yes sessions and
// delivers pending prompts.
func (m Model) applyStatus(msg statusMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	if msg.err != nil {
		cmds = append(cmds, m.setError(msg.err))
	}
	var changed []session.Session
	for _, r := range msg.results {
		i := m.find(r.id)
		if i < 0 || m.busy[r.id] || !m.sessions[i].Status.Active() {
			continue // removed, or changed state while the capture ran
		}
		s := m.sessions[i]
		if r.gone {
			s.Status = session.StatusStopped
			m.sessions[i] = s
			delete(m.hashes, s.ID)
			delete(m.subagents, s.ID)
			delete(m.conversations, s.ID)
			changed = append(changed, s)
			continue
		}
		m.seeSubagents(s.ID, r.subagents)
		if r.conversationErr != nil {
			cmds = append(cmds, m.setError(fmt.Errorf("reading %s's subagents: %w", s.Title, r.conversationErr)))
		}
		m.conversations[s.ID] = r.conversation
		previous, seen := m.hashes[s.ID]
		m.hashes[s.ID] = r.hash
		// Only a screen seen twice unchanged proves the agent is waiting. The
		// saved status may be stale, for example right after a restart.
		still := seen && previous == r.hash
		if seen && !still {
			s.Status = session.StatusRunning
		} else if still {
			s.Status = session.StatusReady
		}

		save := false
		switch {
		case !msg.lead:
		case r.permission && s.AutoYes:
			cmds = append(cmds, m.enterCmd(s))
		case still && s.PendingPrompt != "" && r.trust:
			if s.AutoYes {
				cmds = append(cmds, m.enterCmd(s))
			} else {
				cmds = append(cmds, m.setStatus(s.Title+" is asking whether to trust its worktree. Attach to answer; the prompt is sent after.", false))
			}
		case still && s.PendingPrompt != "":
			cmds = append(cmds, m.promptCmd(s, s.PendingPrompt))
			s.PendingPrompt = ""
			save = true
		}
		// Resume continues the conversation the agent is in. Only the leader
		// saves it, so that several Vineyards do not write it at once.
		if id := r.conversation.ID; msg.lead && id != "" && id != s.Conversation {
			s.Conversation = id
			save = true
		}
		m.sessions[i] = s
		if save {
			changed = append(changed, s)
		}
	}
	m.syncList()
	if len(changed) > 0 {
		cmds = append(cmds, m.replaceCmd(changed...))
	}
	cmds = append(cmds, m.refreshShown())
	return m, tea.Batch(cmds...)
}

// tabNames are the pane tabs' names, as the status bar offers them.
var tabNames = map[tab]string{tabPreview: "preview", tabDiff: "diff", tabIssue: "issue", tabRecap: "recap"}

// openNames say what enter does on each tab.
var openNames = map[tab]string{tabPreview: "attach", tabDiff: "full diff", tabIssue: "issues", tabRecap: "full recap"}

// nextTab is the tab that the tab key switches to.
func (m Model) nextTab() tab { return (m.tab + 1) % tab(len(tabNames)) }

// prevTab is the tab that shift+tab switches to.
func (m Model) prevTab() tab { return (m.tab + tab(len(tabNames)) - 1) % tab(len(tabNames)) }

// switchTab shows the next tab, or the previous one if back.
func (m *Model) switchTab(back bool) tea.Cmd {
	if back {
		return m.showTab(m.prevTab())
	}
	return m.showTab(m.nextTab())
}

// showTab shows tab t in the pane, and fetches the recap if it is the recap
// tab.
func (m *Model) showTab(t tab) tea.Cmd {
	m.tab = t
	m.preview = m.preview.ExitScroll()
	m.refreshIssue()
	m.refreshRecap()
	return m.recapCmd()
}

func (m Model) profiles() []config.Profile {
	if m.opts.Program != "" {
		return []config.Profile{{Name: session.ProgramName(m.opts.Program), Program: m.opts.Program}}
	}
	return m.opts.Config.ResolvedProfiles()
}

func (m Model) createSession(msg common.NewSessionMsg) (tea.Model, tea.Cmd) {
	program := msg.Profile.Program
	if program == "" {
		program = m.profiles()[0].Program
	}
	worktreeDir, err := config.ResolveWorktreeDir(m.opts.RepoRoot, m.opts.Config.WorktreeDir)
	if err != nil {
		return m, m.setError(fmt.Errorf("worktree directory: %w", err))
	}
	s := m.backend.New(session.NewOptions{
		Title:        msg.Title,
		Program:      program,
		Prompt:       msg.Prompt,
		AutoYes:      m.opts.Config.AutoYes || m.opts.AutoYes,
		BranchPrefix: m.opts.Config.BranchPrefix,
		WorktreeDir:  worktreeDir,
		Issue:        msg.Issue,
	})
	m.issuesOpen = false // show the new session, even when started from an issue
	m.sessions = append(m.sessions, s)
	m.syncList()
	m.list = m.list.Select(s.ID)
	return m, tea.Batch(m.startCmd(s), m.refreshShown())
}

func (m Model) runConfirmed(msg common.ConfirmedMsg) (tea.Model, tea.Cmd) {
	i := m.find(msg.SessionID)
	if i < 0 {
		return m, nil
	}
	s := m.sessions[i]
	m.busy[s.ID] = true
	m.syncList()
	backend := m.backend
	var cmd tea.Cmd
	switch msg.Action {
	case common.ActionKill, common.ActionKillDiscardingCommits:
		discard := msg.Action == common.ActionKillDiscardingCommits
		cmd = func() tea.Msg {
			result, err := backend.Kill(s, discard)
			return killedMsg{session: s, result: result, err: err}
		}
	case common.ActionPause:
		cmd = func() tea.Msg {
			paused, err := backend.Pause(s)
			return lifecycleMsg{session: paused, verb: "pause", err: err}
		}
	case common.ActionPush:
		cmd = func() tea.Msg {
			pushed, err := backend.Push(s)
			return pushedMsg{session: pushed, err: err}
		}
	}
	return m, tea.Batch(cmd, m.refreshShown())
}

func (m Model) handleKey(msg tea.KeyPressMsg) (tea.Model, tea.Cmd) {
	if msg.String() == "ctrl+c" {
		return m, tea.Quit
	}
	if m.dialog != nil {
		var cmd tea.Cmd
		m.dialog, cmd = m.dialog.Update(msg)
		return m, cmd
	}
	if m.configOpen {
		var cmd tea.Cmd
		m.settings, cmd = m.settings.Update(msg)
		return m, cmd
	}
	if m.issuesOpen {
		var cmd tea.Cmd
		m.grapes, cmd = m.grapes.Update(msg)
		return m, cmd
	}
	if m.focus == focusPane {
		var cmd tea.Cmd
		switch m.tab {
		case tabDiff:
			m.diff, cmd = m.diff.Update(msg)
		case tabIssue:
			m.issue, cmd = m.issue.Update(msg)
		case tabRecap:
			m.recap, cmd = m.recap.Update(msg)
		case tabPreview:
			if m.shownAgent != "" {
				m.transcript, cmd = m.transcript.Update(msg)
			} else {
				m.preview, cmd = m.preview.Update(msg)
			}
		}
		return m, cmd
	}

	keys := common.ListKeyMap
	s, ok := m.selected()
	switch {
	case key.Matches(msg, keys.Quit):
		return m, tea.Quit
	case key.Matches(msg, keys.Up, keys.Down):
		m.list, _ = m.list.Update(msg)
		return m, m.refreshShown()
	case key.Matches(msg, keys.New, keys.NewPrompt):
		width := min(70, m.width-4)
		d, cmd := dialog.NewSessionDialog(m.theme, m.profiles(), key.Matches(msg, keys.NewPrompt), width)
		m.dialog = d
		return m, cmd
	case key.Matches(msg, keys.Tab, keys.PrevTab):
		return m, m.switchTab(key.Matches(msg, keys.PrevTab))
	case key.Matches(msg, keys.Config):
		_, _, bodyHeight := m.layout()
		m.settings = settings.New(m.opts.Config, m.opts.ConfigPath, m.opts.ConfigErr, m.theme).SetSize(m.width, bodyHeight)
		m.configOpen = true
		return m, nil
	case key.Matches(msg, keys.Focus):
		// The selected session's subagent rows come first, then the pane.
		var expanded bool
		if m.list, expanded = m.list.Expand(); !expanded && ok {
			m.setFocus(focusPane)
		}
		return m, nil
	case key.Matches(msg, keys.Collapse):
		m.list = m.list.Collapse()
		return m, nil
	case key.Matches(msg, keys.Issues), key.Matches(msg, keys.Open) && m.tab == tabIssue:
		return m.showIssues(s, ok)
	case key.Matches(msg, keys.Open) && m.tab == tabRecap:
		if ok {
			m.setFocus(focusPane)
			m.setZoom(true)
		}
		return m, nil
	}

	if !ok {
		return m, nil
	}
	if m.busy[s.ID] || s.Status == session.StatusLoading {
		return m, m.setStatus(s.Title+" is busy.", false)
	}
	switch {
	case key.Matches(msg, keys.Open) && m.tab == tabDiff:
		if !m.diffable(s) {
			return m, m.setStatus(s.Title+" has no worktree to diff. Press r to resume it.", false)
		}
		backend, command, tree := m.backend, m.opts.Config.DiffCommand, m.agentTree
		return m, func() tea.Msg {
			if tree != "" { // the shown subagent's own worktree
				cmd, err := backend.TreeDiffTool(tree, command)
				return diffToolMsg{id: s.ID, cmd: cmd, err: err}
			}
			cmd, err := backend.DiffTool(s, command)
			return diffToolMsg{id: s.ID, cmd: cmd, err: err}
		}
	case key.Matches(msg, keys.Attach, keys.Open):
		if !s.Status.Active() {
			return m, m.setStatus(s.Title+" is not running. Press r to resume it.", false)
		}
		if a, ok := m.list.SelectedSubagent(); ok {
			return m.openSubagent(s, a)
		}
		id := s.ID
		return m, tea.ExecProcess(m.backend.AttachCommand(s), func(err error) tea.Msg {
			return attachDoneMsg{id: id, err: err}
		})
	case key.Matches(msg, keys.Shell):
		if s.Status == session.StatusPaused {
			return m, m.setStatus(s.Title+" is paused. Press r to resume it.", false)
		}
		backend := m.backend
		w, h := m.agentSize()
		return m, func() tea.Msg {
			return shellReadyMsg{session: s, err: backend.EnsureShell(s, w, h)}
		}
	case key.Matches(msg, keys.Kill):
		m.dialog = dialog.NewConfirm(m.theme,
			fmt.Sprintf("Kill %q? Its worktree is deleted, uncommitted changes included. The branch is kept if it has commits.", s.Title),
			common.ActionKill, s.ID)
	case key.Matches(msg, keys.Pause):
		if s.Status == session.StatusPaused {
			return m, nil
		}
		m.dialog = dialog.NewConfirm(m.theme,
			fmt.Sprintf("Check out %q? Its work is committed to %s and the worktree removed, so you can check out the branch elsewhere.", s.Title, s.Branch),
			common.ActionPause, s.ID)
	case key.Matches(msg, keys.Resume):
		if s.Status.Active() {
			return m, nil
		}
		m.busy[s.ID] = true
		m.syncList()
		backend := m.backend
		w, h := m.agentSize()
		return m, tea.Batch(func() tea.Msg {
			resumed, err := backend.Resume(s, w, h)
			return lifecycleMsg{session: resumed, verb: "resume", err: err}
		}, m.refreshShown())
	case key.Matches(msg, keys.Push):
		if s.Status == session.StatusPaused {
			return m, m.setStatus(s.Title+" is paused. Press r to resume it before pushing.", false)
		}
		m.dialog = dialog.NewConfirm(m.theme,
			fmt.Sprintf("Commit all changes in %q and push %s to origin?", s.Title, s.Branch),
			common.ActionPush, s.ID)
	case key.Matches(msg, keys.AutoYes):
		s.AutoYes = !s.AutoYes
		m.replace(s)
		m.syncList()
		state := "off"
		if s.AutoYes {
			state = "on"
		}
		write := m.replaceCmd(s)
		return m, tea.Batch(write, m.setStatus("Auto-yes "+state+" for "+s.Title+".", false))
	}
	return m, nil
}

// handleMouse handles clicks and the wheel. A double click on the list or
// pane does what enter does there.
func (m Model) handleMouse(msg tea.MouseMsg) (tea.Model, tea.Cmd) {
	mouse := msg.Mouse()
	_, click := msg.(tea.MouseClickMsg)
	var first clickAt // the first click of a double click
	double := false
	switch {
	case !click:
	case m.dialog != nil || m.issuesOpen || m.configOpen:
		// The click that closes a dialog must not pair with the next one.
		m.lastClick = clickAt{}
	default:
		first, double = m.doubleClick(mouse.X, mouse.Y)
	}
	listWidth, _, bodyHeight := m.layout()
	var cmd tea.Cmd
	switch {
	case m.width < minWidth || m.height < minHeight:
		return m, nil
	case m.dialog != nil:
		area := headerHeight + bodyHeight
		if m.issuesOpen {
			area = m.height
		}
		left, top := centerOffset(m.dialog.View(), m.width, area)
		m.dialog, cmd = m.dialog.Update(moved(msg, -left, -top))
		return m, cmd
	case m.issuesOpen:
		m.grapes, cmd = m.grapes.Update(msg)
		return m, cmd
	case mouse.Y < headerHeight || mouse.Y >= headerHeight+bodyHeight:
		return m, nil // the header or the status bar
	case m.configOpen && m.settings.PickerActive():
		left, top := centerOffset(m.settings.PickerView(), m.width, headerHeight+bodyHeight)
		m.settings, cmd = m.settings.Update(moved(msg, -left, -top))
		return m, cmd
	case m.configOpen:
		m.settings, cmd = m.settings.Update(moved(msg, 0, -headerHeight))
		return m, cmd
	case mouse.X < listWidth && double:
		// The first click selected the row.
		switch m.list.LineAt(mouse.Y - headerHeight) {
		case list.LineSession:
			return m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
		case list.LineSubagent:
			return m.handleKey(tea.KeyPressMsg{Code: 'o', Text: "o"}) // open it on any tab
		}
		return m, nil // the first click expanded or collapsed the subagents
	case mouse.X < listWidth:
		if click {
			m.setFocus(focusList)
		}
		m.list, _ = m.list.Update(moved(msg, 0, -headerHeight))
		return m, m.refreshShown()
	}

	// The pane: its border, then the tab bar, then the content.
	x, y := mouse.X-listWidth-1, mouse.Y-headerHeight-1
	if t, ok := m.tabAt(x); click && ok && y == 0 {
		if double {
			return m.enter() // the first click showed the tab
		}
		m.setZoom(false)
		return m, m.showTab(t)
	}
	content := moved(msg, -listWidth-1, -headerHeight-2)
	switch {
	case double && first.acted:
		return m, nil // the first click folded a file or jumped to one
	case double:
		return m.enter()
	case click:
		m.setFocus(focusPane)
		m.lastClick.acted = m.tab == tabDiff && m.diff.Clickable(content.Mouse().Y)
	}
	switch m.tab {
	case tabDiff:
		m.diff, cmd = m.diff.Update(content)
	case tabIssue:
		m.issue, cmd = m.issue.Update(content)
	case tabRecap:
		m.recap, cmd = m.recap.Update(content)
	case tabPreview:
		if m.shownAgent != "" {
			m.transcript, cmd = m.transcript.Update(content)
		} else {
			m.preview, cmd = m.preview.Update(content)
		}
	}
	return m, cmd
}

// clickAt is where and when a click landed.
type clickAt struct {
	x, y int
	at   time.Time
	// acted marks a click that did something itself, such as folding a file
	// in the diff, so that a double click it starts does nothing more.
	acted bool
}

// doubleClick records a click at x, y and reports whether it makes a double
// click with the one before, returned as first: on the same line, within a
// column, and soon enough. The click after a double click starts over.
func (m *Model) doubleClick(x, y int) (first clickAt, double bool) {
	at, last := now(), m.lastClick
	double = y == last.y && x >= last.x-1 && x <= last.x+1 && at.Sub(last.at) <= doubleClickTime
	m.lastClick = clickAt{x: x, y: y, at: at}
	if double {
		m.lastClick = clickAt{}
	}
	return last, double
}

// enter does what the enter key does in the list, for a double click.
func (m Model) enter() (tea.Model, tea.Cmd) {
	m.setFocus(focusList)
	return m.handleKey(tea.KeyPressMsg{Code: tea.KeyEnter})
}

// moved returns the click or wheel event msg moved by dx and dy, to make its
// position relative to the part of the screen that receives it.
func moved(msg tea.MouseMsg, dx, dy int) tea.MouseMsg {
	mouse := msg.Mouse()
	mouse.X += dx
	mouse.Y += dy
	if _, ok := msg.(tea.MouseClickMsg); ok {
		return tea.MouseClickMsg(mouse)
	}
	return tea.MouseWheelMsg(mouse)
}

// --- View ---

// View renders the screen.
func (m Model) View() tea.View {
	var content string
	switch {
	case m.width == 0:
	case m.width < minWidth || m.height < minHeight:
		content = lipgloss.Place(m.width, m.height, lipgloss.Center, lipgloss.Center,
			m.theme.StyleSubtitle.Render(fmt.Sprintf("Vineyard needs at least %d×%d cells.", minWidth, minHeight)))
	default:
		content = m.render()
	}
	v := tea.NewView(content)
	v.AltScreen = true
	v.MouseMode = tea.MouseModeCellMotion
	return v
}

func (m Model) render() string {
	if m.issuesOpen {
		// Grapes draws the whole screen, its own header and status bar included.
		screen := m.grapes.View()
		if m.dialog != nil {
			screen = overlayCenter(screen, m.dialog.View(), m.width, m.height)
		}
		return screen
	}
	_, _, bodyHeight := m.layout()
	var body string
	switch {
	case m.configOpen:
		body = m.settings.View()
	case m.zoomed:
		body = m.renderPane()
	default:
		body = lipgloss.JoinHorizontal(lipgloss.Top, m.list.View(), m.renderPane())
	}
	screen := lipgloss.JoinVertical(lipgloss.Left, m.renderHeader(), body)
	if m.configOpen && m.settings.PickerActive() {
		screen = overlayCenter(screen, m.settings.PickerView(), m.width, headerHeight+bodyHeight)
	}
	if m.dialog != nil {
		screen = overlayCenter(screen, m.dialog.View(), m.width, headerHeight+bodyHeight)
	}
	return lipgloss.JoinVertical(lipgloss.Left, screen, m.renderStatusBar())
}

func (m Model) renderHeader() string {
	t := m.theme
	left := t.StyleAppTitle.Render("vineyard") + t.StyleFaint.Render(m.opts.Version+"  ") + t.StyleSubtitle.Render(m.opts.RepoName)

	counts := map[session.Status]int{}
	for _, s := range m.sessions {
		counts[s.Status]++
	}
	var parts []string
	for _, st := range []session.Status{session.StatusRunning, session.StatusReady, session.StatusPaused, session.StatusStopped} {
		if n := counts[st]; n > 0 {
			icon := lipgloss.NewStyle().Foreground(t.StatusColor(st)).Render(common.StatusIcon(st))
			parts = append(parts, icon+t.StyleSubtitle.Render(fmt.Sprintf(" %d %s", n, st)))
		}
	}
	right := strings.Join(parts, "   ") + "  "
	gap := max(1, m.width-ansi.StringWidth(left)-ansi.StringWidth(right))
	row := ansi.Truncate(left+strings.Repeat(" ", gap)+right, m.width, "")
	return row + "\n" + t.StyleSeparator.Render(strings.Repeat("━", m.width))
}

// tabLabel is a tab as the tab bar draws it.
type tabLabel struct {
	tab  tab
	text string
}

// tabLabels returns the pane's tabs in order, each rendered for the tab bar,
// where one space separates them.
func (m Model) tabLabels() []tabLabel {
	t := m.theme
	diffLabel := "Diff"
	issueLabel := "Issue"
	if s, ok := m.selected(); ok {
		stat := m.stats[s.ID]
		if m.agentTree != "" { // the diff of the shown subagent's own worktree
			stat = git.Stat{}
			for _, f := range m.diff.Files() {
				stat.Added += f.Added
				stat.Removed += f.Removed
			}
		}
		if !stat.IsZero() {
			diffLabel += fmt.Sprintf(" +%d -%d", stat.Added, stat.Removed)
		}
		ids := m.shownIssues(s)
		if len(ids) > 1 {
			issueLabel = "Issues"
		}
		for _, id := range ids {
			issueLabel += fmt.Sprintf(" #%d", id)
		}
	}
	texts := map[tab]string{tabPreview: "Preview", tabDiff: diffLabel, tabIssue: issueLabel, tabRecap: "Recap"}
	var labels []tabLabel
	for which := range tab(len(texts)) {
		style := t.StyleTabInactive
		if which == m.tab {
			style = t.StyleTabActive
		}
		labels = append(labels, tabLabel{which, style.Render(texts[which])})
	}
	return labels
}

// tabAt returns the tab drawn at column x of the tab bar.
func (m Model) tabAt(x int) (tab, bool) {
	for _, l := range m.tabLabels() {
		width := ansi.StringWidth(l.text)
		if x < width {
			return l.tab, x >= 0
		}
		x -= width + 1
	}
	return 0, false
}

func (m Model) renderPane() string {
	t := m.theme
	_, paneWidth, bodyHeight := m.layout()
	innerWidth, _ := m.paneContentSize()

	var labels []string
	for _, l := range m.tabLabels() {
		labels = append(labels, l.text)
	}
	tabs := strings.Join(labels, " ")

	info := ""
	switch {
	case m.tab == tabPreview && m.preview.Scrolling():
		info = "scrollback · G returns to live"
	case m.tab == tabDiff && len(m.diff.Files()) > 0:
		if i := m.diff.CurrentFile(); i >= 0 {
			info = fmt.Sprintf("file %d of %d", i+1, len(m.diff.Files()))
		} else {
			info = "summary"
		}
	}
	info = t.StyleFaint.Render(info)
	gap := max(1, innerWidth-ansi.StringWidth(tabs)-ansi.StringWidth(info)-1)
	tabBar := ansi.Truncate(tabs+strings.Repeat(" ", gap)+info+" ", innerWidth, "")

	content := m.preview.View()
	if m.shownAgent != "" {
		content = m.transcript.View()
	}
	switch m.tab {
	case tabDiff:
		content = m.diff.View()
		if s, ok := m.selected(); !ok || !m.diffable(s) {
			// Without a worktree there is nothing current to diff.
			content = m.preview.View()
		}
	case tabIssue:
		content = m.issue.View()
	case tabRecap:
		content = m.recap.View()
	}
	style := t.StylePane
	if m.focus == focusPane {
		style = t.StylePaneFocused
	}
	return style.Width(paneWidth).Height(bodyHeight).Render(tabBar + "\n" + content)
}

func (m Model) renderStatusBar() string {
	t := m.theme
	if m.status != "" {
		text := m.status
		if m.statusIsErr {
			text = t.StyleError.Render(text)
		}
		return t.StyleStatusBar.Width(m.width).Render(ansi.Truncate(text, m.width-2, "…"))
	}

	var hints [][2]string
	switch {
	case m.dialog != nil:
		hints = m.dialog.Hints()
	case m.configOpen:
		hints = m.settings.Hints()
	case m.focus == focusPane && m.tab == tabDiff:
		hints = [][2]string{{"j/k", "scroll"}, {"ctrl+d/u", "page"}, {"]/[", "next/prev file"}, {"enter", "fold"}, {"c/e", "fold all/none"}, {"g/G", "top/bottom"}, {"tab", tabNames[m.nextTab()]}, {"esc", "back"}}
	case m.focus == focusPane && (m.tab == tabIssue || m.tab == tabRecap || m.tab == tabPreview && m.shownAgent != ""):
		hints = [][2]string{{"j/k", "scroll"}, {"ctrl+d/u", "page"}, {"g/G", "top/bottom"}, {"tab", tabNames[m.nextTab()]}, {"esc", "back"}}
	case m.focus == focusPane:
		hints = [][2]string{{"j/k", "scroll"}, {"ctrl+d/u", "page"}, {"G", "live"}, {"tab", tabNames[m.nextTab()]}, {"esc", "back"}}
	default:
		open, attach := openNames[m.tab], "attach"
		if _, ok := m.list.SelectedSubagent(); ok {
			attach = "open subagent"
			if m.tab == tabPreview {
				open = attach
			}
		}
		hints = [][2]string{{"n", "new"}, {"N", "new+prompt"}, {"enter", open}}
		if m.tab != tabPreview {
			hints = append(hints, [2]string{"o", attach})
		}
		switch {
		case m.list.Expandable():
			hints = append(hints, [2]string{"→", "show subagents"})
		case m.list.Collapsible():
			hints = append(hints, [2]string{"←", "hide subagents"})
		}
		hints = append(hints, [][2]string{
			{"t", "shell"}, {"s", "push"}, {"c", "checkout"}, {"r", "resume"}, {"D", "kill"},
			{"a", "auto-yes"}, {"tab", tabNames[m.nextTab()]},
		}...)
		if !m.list.Expandable() { // otherwise l shows the subagents first
			hints = append(hints, [2]string{"l", "scroll"})
		}
		hints = append(hints, [][2]string{{"i", "issues"}, {"C", "config"}, {"q", "quit"}}...)
	}
	dot := t.StyleFaint.Render(" · ")
	var b strings.Builder
	used := 0
	for i, h := range hints {
		part := t.FormatKeyHint(h[0], h[1])
		width := ansi.StringWidth(part)
		if i > 0 {
			width += ansi.StringWidth(dot)
		}
		if used+width > m.width-2 {
			break
		}
		if i > 0 {
			b.WriteString(dot)
		}
		b.WriteString(part)
		used += width
	}
	return t.StyleStatusBar.Width(m.width).Render(b.String())
}

// centerOffset returns where overlayCenter draws fg's top-left corner.
func centerOffset(fg string, width, height int) (left, top int) {
	return max(0, (width-lipgloss.Width(fg))/2), max(0, (height-lipgloss.Height(fg))/2)
}

// overlayCenter draws fg centred over bg, keeping the background visible on
// either side of it. Adapted from grapes.
func overlayCenter(bg, fg string, width, height int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")
	for len(bgLines) < height {
		bgLines = append(bgLines, "")
	}
	fgWidth := lipgloss.Width(fg)
	left, top := centerOffset(fg, width, height)
	for i, fgLine := range fgLines {
		y := top + i
		if y >= len(bgLines) {
			break
		}
		line := bgLines[y]
		prefix := ansi.Truncate(line, left, "")
		if w := ansi.StringWidth(prefix); w < left {
			prefix += strings.Repeat(" ", left-w)
		}
		suffix := ansi.TruncateLeft(line, left+fgWidth, "")
		bgLines[y] = prefix + "\x1b[0m" + fgLine + "\x1b[0m" + suffix
	}
	return strings.Join(bgLines[:height], "\n")
}
