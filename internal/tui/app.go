// Package tui is Vineyard's terminal interface. The root Model in this file
// owns the sessions and every side effect; the views in its subpackages only
// render and emit messages.
package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"charm.land/bubbles/v2/key"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/git"
	"github.com/mikeryanboss/vineyard/internal/session"
	"github.com/mikeryanboss/vineyard/internal/tui/common"
	"github.com/mikeryanboss/vineyard/internal/tui/dialog"
	"github.com/mikeryanboss/vineyard/internal/tui/diffview"
	"github.com/mikeryanboss/vineyard/internal/tui/list"
	"github.com/mikeryanboss/vineyard/internal/tui/preview"
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
	}
	statusMsg struct{ results []screenStatus }
	diffMsg   struct {
		selectedID string
		hasRaw     bool // whether raw was fetched; paused sessions have no worktree to diff
		raw        string
		rawErr     error
		stats      map[string]git.Stat
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
	scrollbackMsg struct {
		id      string
		history string
		err     error
	}
	promptSentMsg struct {
		id  string
		err error
	}
	saveErrMsg     struct{ err error }
	clearStatusMsg struct{ seq int }
)

// tick schedules timer messages. Tests replace it to drive the polling loops
// and status timeouts by hand.
var tick = tea.Tick

// Options configure the model.
type Options struct {
	Config   config.Config
	RepoName string
	Version  string
	// AutoYes turns auto-yes on for new sessions regardless of config.
	AutoYes bool
	// Program, when set, is the only agent offered for new sessions.
	Program string
	// Status is shown in the status bar at startup, such as a config error.
	Status string
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

	previewInFlight bool
	statusInFlight  bool
	diffInFlight    bool

	list    list.Model
	preview preview.Model
	diff    diffview.Model
	dialog  dialog.Dialog
	focus   focus
	tab     tab

	status      string
	statusIsErr bool
	statusSeq   int
}

// NewModel returns the root model showing sessions.
func NewModel(backend Backend, sessions []session.Session, opts Options) Model {
	theme := common.NewTheme(true) // dark until the terminal reports its background
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
	}
	if opts.Status != "" {
		m.status, m.statusIsErr = opts.Status, true
	}
	m.syncList()
	// Mark the initial selection as shown; the first ticks fetch its screen and diff.
	_ = m.refreshShown()
	m.diffInFlight = false // the fetch refreshShown planned was discarded above
	return m
}

// Init starts the polling loops.
func (m Model) Init() tea.Cmd {
	return tea.Batch(
		tea.RequestBackgroundColor,
		func() tea.Msg { return previewTickMsg{} },
		func() tea.Msg { return statusTickMsg{} },
		func() tea.Msg { return diffTickMsg{} },
	)
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
}

// syncList rebuilds the list entries from the sessions.
func (m *Model) syncList() {
	items := make([]list.Item, len(m.sessions))
	for i, s := range m.sessions {
		if m.busy[s.ID] {
			s.Status = session.StatusLoading
		}
		items[i] = list.Item{Session: s, Stat: m.stats[s.ID]}
	}
	m.list = m.list.SetItems(items)
}

// layout returns the list width, pane width, and body height.
func (m Model) layout() (listWidth, paneWidth, bodyHeight int) {
	bodyHeight = m.height - headerHeight - statusBarHeight
	listWidth = max(26, min(44, m.width*3/10))
	paneWidth = m.width - listWidth
	return listWidth, paneWidth, bodyHeight
}

// paneContentSize is the size of the preview and diff content areas: the pane
// minus its border and tab bar. Agents' tmux windows are kept at this size.
func (m Model) paneContentSize() (int, int) {
	_, paneWidth, bodyHeight := m.layout()
	return max(1, paneWidth-2), max(1, bodyHeight-3)
}

func (m *Model) applySizes() {
	listWidth, _, bodyHeight := m.layout()
	w, h := m.paneContentSize()
	m.list = m.list.SetSize(listWidth, bodyHeight)
	m.preview = m.preview.SetSize(w, h)
	m.diff = m.diff.SetSize(w, h)
}

func (m *Model) applyTheme() {
	m.list = m.list.SetTheme(m.theme)
	m.preview = m.preview.SetTheme(m.theme)
	m.diff = m.diff.SetTheme(m.theme)
}

func (m *Model) setFocus(f focus) {
	m.focus = f
	m.list = m.list.SetFocused(f == focusList)
	if f == focusList {
		m.preview = m.preview.ExitScroll()
	}
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
	if text := m.placeholder(s, ok); text != "" {
		m.preview = m.preview.SetPlaceholder(text)
	}
	if s.ID == m.shownID {
		return nil
	}
	m.shownID = s.ID
	m.preview = m.preview.ExitScroll()
	m.diff = m.diff.SetDiff("")
	if !ok {
		return nil
	}
	m.diffInFlight = true
	if !s.Status.Active() || m.busy[s.ID] {
		return m.diffCmd() // no live screen to capture
	}
	return tea.Batch(m.captureCmd(s), m.diffCmd())
}

func (m Model) saveCmd() tea.Cmd {
	snapshot := slices.Clone(m.sessions)
	backend := m.backend
	return func() tea.Msg {
		if err := backend.Save(snapshot); err != nil {
			return saveErrMsg{err: fmt.Errorf("saving sessions: %w", err)}
		}
		return nil
	}
}

// --- Commands ---

func (m Model) captureCmd(s session.Session) tea.Cmd {
	backend := m.backend
	w, h := m.paneContentSize()
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
			}()
		}
		for range active {
			<-done
		}
		return statusMsg{results: results}
	}
}

// diffCmd fetches the full diff of the selected session and the line counts
// of all others with a worktree.
func (m Model) diffCmd() tea.Cmd {
	backend := m.backend
	selected, hasSelected := m.selected()
	var others []session.Session
	for _, s := range m.sessions {
		if s.ID != selected.ID && (s.Status.Active() || s.Status == session.StatusStopped) && !m.busy[s.ID] {
			others = append(others, s)
		}
	}
	showDiff := hasSelected && m.diffable(selected)
	return func() tea.Msg {
		msg := diffMsg{selectedID: selected.ID, hasRaw: showDiff, stats: map[string]git.Stat{}}
		if showDiff {
			msg.raw, msg.rawErr = backend.Diff(selected)
		}
		for _, s := range others {
			if stat, err := backend.DiffStat(s); err == nil {
				msg.stats[s.ID] = stat
			}
		}
		return msg
	}
}

func (m Model) startCmd(s session.Session) tea.Cmd {
	backend := m.backend
	w, h := m.paneContentSize()
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
		return m, nil

	case tea.BackgroundColorMsg:
		m.theme = common.NewTheme(msg.IsDark())
		m.applyTheme()
		return m, nil

	case tea.KeyPressMsg:
		return m.handleKey(msg)

	case tea.MouseClickMsg, tea.MouseWheelMsg:
		return m.handleMouse(msg)

	case previewTickMsg:
		next := tick(previewInterval, func(time.Time) tea.Msg { return previewTickMsg{} })
		s, ok := m.selected()
		if !ok || !s.Status.Active() || m.busy[s.ID] || m.previewInFlight || m.preview.Scrolling() {
			return m, next
		}
		m.previewInFlight = true
		return m, tea.Batch(next, m.captureCmd(s))

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
		next := tick(statusInterval, func(time.Time) tea.Msg { return statusTickMsg{} })
		if m.statusInFlight {
			return m, next
		}
		m.statusInFlight = true
		return m, tea.Batch(next, m.statusCmd())

	case statusMsg:
		m.statusInFlight = false
		return m.applyStatus(msg)

	case diffTickMsg:
		next := tick(diffInterval, func(time.Time) tea.Msg { return diffTickMsg{} })
		if m.diffInFlight {
			return m, next
		}
		m.diffInFlight = true
		return m, tea.Batch(next, m.diffCmd())

	case diffMsg:
		m.diffInFlight = false
		for id, stat := range msg.stats {
			m.stats[id] = stat
		}
		if msg.selectedID == m.shownID && msg.hasRaw && msg.rawErr == nil {
			m.diff = m.diff.SetDiff(msg.raw)
			added, removed := 0, 0
			for _, f := range m.diff.Files() {
				added += f.Added
				removed += f.Removed
			}
			m.stats[msg.selectedID] = git.Stat{Added: added, Removed: removed}
		}
		m.syncList()
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
		return m, tea.Batch(m.saveCmd(), m.refreshShown())

	case lifecycleMsg:
		delete(m.busy, msg.session.ID)
		var cmds []tea.Cmd
		if msg.err != nil {
			cmds = append(cmds, m.setError(msg.err))
		} else {
			m.replace(msg.session)
			delete(m.hashes, msg.session.ID)
			cmds = append(cmds, m.saveCmd())
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
			return m, m.setError(msg.err)
		}
		m.remove(msg.session.ID)
		m.syncList()
		text := "Killed " + msg.session.Title + "."
		if msg.result.KeptBranch {
			text = "Killed " + msg.session.Title + ". Its commits are kept on " + msg.session.Branch + "."
		}
		return m, tea.Batch(m.saveCmd(), m.setStatus(text, false), m.refreshShown())

	case pushedMsg:
		delete(m.busy, msg.session.ID)
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

	case scrollbackMsg:
		if msg.err != nil {
			return m, m.setError(msg.err)
		}
		if msg.id == m.shownID {
			m.preview = m.preview.SetScrollback(msg.history)
		}
		return m, nil

	case promptSentMsg:
		if msg.err != nil {
			return m, m.setError(fmt.Errorf("sending prompt: %w", msg.err))
		}
		return m, nil

	case saveErrMsg:
		return m, m.setError(msg.err)

	case clearStatusMsg:
		if msg.seq == m.statusSeq {
			m.status = ""
		}
		return m, nil

	case common.NewSessionMsg:
		m.dialog = nil
		return m.createSession(msg)

	case common.ConfirmedMsg:
		m.dialog = nil
		return m.runConfirmed(msg)

	case common.DialogCancelledMsg:
		m.dialog = nil
		return m, nil

	case common.LeavePaneMsg:
		m.setFocus(focusList)
		return m, nil

	case common.SwitchTabMsg:
		m.toggleTab()
		return m, nil

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

	// Anything else, such as cursor blinks, belongs to an open dialog.
	if m.dialog != nil {
		var cmd tea.Cmd
		m.dialog, cmd = m.dialog.Update(msg)
		return m, cmd
	}
	return m, nil
}

// applyStatus updates running/ready states from fresh screen fingerprints,
// answers prompts for auto-yes sessions, and delivers pending prompts.
func (m Model) applyStatus(msg statusMsg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd
	changed := false
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
			changed = true
			continue
		}
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

		switch {
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
			changed = true
		}
		m.sessions[i] = s
	}
	m.syncList()
	if changed {
		cmds = append(cmds, m.saveCmd())
	}
	cmds = append(cmds, m.refreshShown())
	return m, tea.Batch(cmds...)
}

func (m *Model) toggleTab() {
	if m.tab == tabPreview {
		m.tab = tabDiff
	} else {
		m.tab = tabPreview
	}
	m.preview = m.preview.ExitScroll()
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
	s := m.backend.New(session.NewOptions{
		Title:   msg.Title,
		Program: program,
		Prompt:  msg.Prompt,
		AutoYes: m.opts.Config.AutoYes || m.opts.AutoYes,
	})
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
	case common.ActionKill:
		cmd = func() tea.Msg {
			result, err := backend.Kill(s)
			return killedMsg{session: s, result: result, err: err}
		}
	case common.ActionPause:
		cmd = func() tea.Msg {
			paused, err := backend.Pause(s)
			return lifecycleMsg{session: paused, verb: "pause", err: err}
		}
	case common.ActionPush:
		cmd = func() tea.Msg {
			return pushedMsg{session: s, err: backend.Push(s)}
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
	if m.focus == focusPane {
		var cmd tea.Cmd
		if m.tab == tabDiff {
			m.diff, cmd = m.diff.Update(msg)
		} else {
			m.preview, cmd = m.preview.Update(msg)
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
	case key.Matches(msg, keys.Tab):
		m.toggleTab()
		return m, nil
	case key.Matches(msg, keys.Focus):
		if ok {
			m.setFocus(focusPane)
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
	case key.Matches(msg, keys.Attach):
		if !s.Status.Active() {
			return m, m.setStatus(s.Title+" is not running. Press r to resume it.", false)
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
		w, h := m.paneContentSize()
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
		w, h := m.paneContentSize()
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
		return m, tea.Batch(m.saveCmd(), m.setStatus("Auto-yes "+state+" for "+s.Title+".", false))
	}
	return m, nil
}

func (m Model) handleMouse(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.dialog != nil {
		return m, nil
	}
	var mouse tea.Mouse
	switch msg := msg.(type) {
	case tea.MouseClickMsg:
		mouse = tea.Mouse(msg)
	case tea.MouseWheelMsg:
		mouse = tea.Mouse(msg)
	}
	listWidth, _, _ := m.layout()
	if mouse.X < listWidth {
		mouse.Y -= headerHeight
		var local tea.Msg = tea.MouseWheelMsg(mouse)
		if _, ok := msg.(tea.MouseClickMsg); ok {
			local = tea.MouseClickMsg(mouse)
			m.setFocus(focusList)
		}
		m.list, _ = m.list.Update(local)
		return m, m.refreshShown()
	}
	if _, ok := msg.(tea.MouseClickMsg); ok {
		m.setFocus(focusPane)
		return m, nil
	}
	var cmd tea.Cmd
	if m.tab == tabDiff {
		m.diff, cmd = m.diff.Update(msg)
	} else {
		m.preview, cmd = m.preview.Update(msg)
	}
	return m, cmd
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
	_, _, bodyHeight := m.layout()
	screen := lipgloss.JoinVertical(lipgloss.Left,
		m.renderHeader(),
		lipgloss.JoinHorizontal(lipgloss.Top, m.list.View(), m.renderPane()),
	)
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

func (m Model) renderPane() string {
	t := m.theme
	_, paneWidth, bodyHeight := m.layout()
	innerWidth, _ := m.paneContentSize()

	tabLabel := func(label string, which tab) string {
		if m.tab == which {
			return t.StyleTabActive.Render(label)
		}
		return t.StyleTabInactive.Render(label)
	}
	diffLabel := "Diff"
	if s, ok := m.selected(); ok {
		if stat := m.stats[s.ID]; !stat.IsZero() {
			diffLabel += fmt.Sprintf(" +%d -%d", stat.Added, stat.Removed)
		}
	}
	tabs := tabLabel("Preview", tabPreview) + " " + tabLabel(diffLabel, tabDiff)

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
	if m.tab == tabDiff {
		content = m.diff.View()
		if s, ok := m.selected(); !ok || !m.diffable(s) {
			// Without a worktree there is nothing current to diff.
			content = m.preview.View()
		}
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
	case m.focus == focusPane && m.tab == tabDiff:
		hints = [][2]string{{"j/k", "scroll"}, {"ctrl+d/u", "page"}, {"]/[", "next/prev file"}, {"g/G", "top/bottom"}, {"tab", "preview"}, {"esc", "back"}}
	case m.focus == focusPane:
		hints = [][2]string{{"j/k", "scroll"}, {"ctrl+d/u", "page"}, {"G", "live"}, {"tab", "diff"}, {"esc", "back"}}
	default:
		hints = [][2]string{
			{"n", "new"}, {"N", "new+prompt"}, {"enter", "attach"}, {"t", "shell"}, {"s", "push"},
			{"c", "checkout"}, {"r", "resume"}, {"D", "kill"}, {"a", "auto-yes"}, {"tab", "diff"},
			{"l", "scroll"}, {"q", "quit"},
		}
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

// overlayCenter draws fg centred over bg, keeping the background visible on
// either side of it. Adapted from grapes.
func overlayCenter(bg, fg string, width, height int) string {
	bgLines := strings.Split(bg, "\n")
	fgLines := strings.Split(fg, "\n")
	for len(bgLines) < height {
		bgLines = append(bgLines, "")
	}
	fgWidth := 0
	for _, line := range fgLines {
		fgWidth = max(fgWidth, ansi.StringWidth(line))
	}
	top := max(0, (height-len(fgLines))/2)
	left := max(0, (width-fgWidth)/2)
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
