package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/charmbracelet/x/ansi"

	"github.com/mikeryanboss/vineyard/internal/config"
	"github.com/mikeryanboss/vineyard/internal/git"
	"github.com/mikeryanboss/vineyard/internal/logfile"
	"github.com/mikeryanboss/vineyard/internal/session"
	"github.com/mikeryanboss/vineyard/internal/subagent"
	"github.com/mikeryanboss/vineyard/internal/tui"
	"github.com/mikeryanboss/vineyard/internal/vterm"
)

// maxMessageBytes caps a peer message at 1 MiB, matching Claude Code's local
// peer message limit.
const maxMessageBytes = 1 << 20

var (
	// sendStillFor is how long a recipient's screen must remain unchanged to
	// count as waiting for input, matching stillFor in internal/tui/app.go.
	sendStillFor = 600 * time.Millisecond
	// sendPasteDelay lets the harness ingest a bracketed paste before Enter
	// submits it, matching promptCmd in internal/tui/app.go.
	sendPasteDelay = 150 * time.Millisecond
	// drainPollDelay paces background checks on a busy recipient.
	drainPollDelay = time.Second

	spawnDrainer = startDrainerProcess
)

var xmlAttrEscaper = strings.NewReplacer(
	"&", "&amp;",
	"\"", "&quot;",
	"<", "&lt;",
	">", "&gt;",
)

var errSessionStopped = errors.New("session is not running")

// runSend sends message to the session named by to, delivering immediately
// when the session is waiting for input or queuing for background delivery
// while it works.
func runSend(to, message string, stdout io.Writer) error {
	cwd, err := os.Getwd()
	if err != nil {
		return err
	}
	repo, err := git.FindRepo(cwd)
	if err != nil {
		return err
	}
	env, err := openSendEnv(repo)
	if err != nil {
		return err
	}
	if !env.peerMessages {
		return errors.New("peer messages are disabled in .vineyard/config.toml")
	}
	if strings.TrimSpace(message) == "" {
		return errors.New("message is empty")
	}
	if len(message) > maxMessageBytes {
		return fmt.Errorf("message exceeds %d bytes", maxMessageBytes)
	}
	saved, err := env.store.Load()
	if err != nil {
		return err
	}
	sessions, err := env.manager.Locate(saved, env.worktreeDir)
	if err != nil {
		return err
	}
	target, err := resolveTarget(sessions, to)
	if err != nil {
		return err
	}
	sender, hasSender, err := env.resolveSender(sessions)
	if err != nil {
		return err
	}
	if hasSender && sender.ID == target.ID {
		return fmt.Errorf("cannot send a message to %q from itself", target.Address())
	}

	agents, err := claudeAgents()
	if err != nil {
		return err
	}
	if len(agents[repo.Checkout]) > 0 && len(agents[target.WorktreePath]) > 0 {
		return fmt.Errorf("both sessions run Claude Code; message %q with SendMessage instead", agents[target.WorktreePath][0])
	}

	if target.Status == session.StatusPaused {
		return fmt.Errorf("session %q is not running", to)
	}
	running, ok, err := env.manager.Running(target)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("session %q is not running", to)
	}
	term, err := env.terminal(running)
	if err != nil {
		return err
	}
	screen1, err := term.Capture(running.TmuxName)
	if err != nil {
		if !term.Exists(running.TmuxName) {
			return fmt.Errorf("session %q is not running", to)
		}
		return err
	}

	from := "main"
	if hasSender {
		from = sender.Address()
	} else if repo.Checkout != repo.Root {
		from = filepath.Base(repo.Checkout)
	}
	formatted := formatPeerMessage(from, hasSender, message)

	inbox := session.NewInbox(env.dir)
	d, err := inbox.Enqueue(running.ID, formatted)
	if err != nil {
		return err
	}
	if d == nil {
		// Another sender or drainer holds delivery for running.ID and will
		// pick up the queued message before releasing ownership.
		fmt.Fprintf(stdout, "Queued for %s\n", running.Address())
		return nil
	}

	ready, err := env.awaitStill(running, term, screen1)
	if errors.Is(err, errSessionStopped) {
		_, _ = d.Pop()
		d.Release()
		return fmt.Errorf("session %q stopped before delivery", running.Address())
	}
	if err != nil {
		_, _ = d.Pop()
		d.Release()
		return err
	}
	if !ready {
		d.Release()
		if err := spawnDrainer(env.dir, repo.Root, running.ID); err != nil {
			return err
		}
		fmt.Fprintf(stdout, "Queued for %s\n", running.Address())
		return nil
	}

	msgs, err := d.Pop()
	if err != nil {
		d.Release()
		return err
	}
	if len(msgs) > 0 {
		if err := deliverText(term, running.TmuxName, strings.Join(msgs, "\n\n")); err != nil {
			d.Release()
			return err
		}
	}
	empty, err := d.FinishIfEmpty()
	if err != nil {
		d.Release()
		return err
	}
	if !empty {
		d.Release()
		if err := spawnDrainer(env.dir, repo.Root, running.ID); err != nil {
			return err
		}
	}
	fmt.Fprintf(stdout, "Delivered to %s\n", running.Address())
	return nil
}

// runDrain polls a busy session until its screen settles and delivers its
// queued peer messages, stopping if the session pauses, stops, or is removed.
func runDrain(repoRoot, id string) error {
	dir := config.Dir(repoRoot)
	d, err := session.NewInbox(dir).Claim(id)
	if err != nil || d == nil {
		return err
	}
	defer d.Release()

	repo, err := git.FindRepo(repoRoot)
	if err != nil {
		_, _ = d.Pop()
		return err
	}
	env, err := openSendEnv(repo)
	if err != nil {
		_, _ = d.Pop()
		return err
	}

	for {
		if cfg, err := config.Load(env.dir); err == nil && !cfg.PeerMessages {
			_, _ = d.Pop()
			return nil
		}
		saved, err := env.store.Load()
		if err != nil {
			return err
		}
		i := slices.IndexFunc(saved, func(s session.Session) bool { return s.ID == id })
		if i < 0 || saved[i].Status == session.StatusPaused {
			_, _ = d.Pop()
			return nil
		}
		running, ok, err := env.manager.Running(saved[i])
		if err != nil {
			_, _ = d.Pop()
			return err
		}
		if !ok {
			_, _ = d.Pop()
			return nil
		}
		term, err := env.terminal(running)
		if err != nil {
			_, _ = d.Pop()
			return err
		}
		screen1, err := term.Capture(running.TmuxName)
		if err != nil {
			if !term.Exists(running.TmuxName) {
				_, _ = d.Pop()
				return nil
			}
			_, _ = d.Pop()
			return err
		}
		ready, err := env.awaitStill(running, term, screen1)
		if errors.Is(err, errSessionStopped) {
			_, _ = d.Pop()
			return nil
		}
		if err != nil {
			_, _ = d.Pop()
			return err
		}
		if ready {
			msgs, err := d.Pop()
			if err != nil {
				return err
			}
			if len(msgs) > 0 {
				if err := deliverText(term, running.TmuxName, strings.Join(msgs, "\n\n")); err != nil {
					if !term.Exists(running.TmuxName) {
						return nil
					}
					return err
				}
			}
			empty, err := d.FinishIfEmpty()
			if err != nil || empty {
				return err
			}
		}
		time.Sleep(drainPollDelay)
	}
}

type sendEnv struct {
	repo         git.Repo
	dir          string
	worktreeDir  string
	peerMessages bool
	store        session.Store
	vineyards    *vineyards
	terminals    map[string]tui.Terminal
	manager      *session.Manager
}

func openSendEnv(repo git.Repo) (*sendEnv, error) {
	dir := config.Dir(repo.Root)
	cfg, err := config.Load(dir)
	if err != nil {
		return nil, err
	}
	worktreeDir, err := config.ResolveWorktreeDir(repo.Root, cfg.WorktreeDir)
	if err != nil {
		return nil, err
	}
	vy, err := newVineyards()
	if err != nil {
		return nil, err
	}
	terminals := multiplexers(vy.current())
	managed := map[string]session.Terminal{}
	for name, t := range terminals {
		managed[name] = t
	}
	mgr := session.NewManager(repo, managed, cfg.Multiplexer, config.ProjectName(repo.Root))
	mgr.Protocol = vterm.Protocol()
	mgr.Older = func(protocol int) session.Terminal { return vy.Older(protocol) }
	mgr.Protocols = vy.protocols
	return &sendEnv{
		repo:         repo,
		dir:          dir,
		worktreeDir:  worktreeDir,
		peerMessages: cfg.PeerMessages,
		store:        session.NewStore(dir),
		vineyards:    vy,
		terminals:    terminals,
		manager:      mgr,
	}, nil
}

func (e *sendEnv) terminal(s session.Session) (tui.Terminal, error) {
	if s.Multiplexer == config.MultiplexerVineyard && s.Protocol != vterm.Protocol() {
		return e.vineyards.Older(s.Protocol), nil
	}
	t, ok := e.terminals[s.Multiplexer]
	if !ok {
		return nil, session.Unavailable(s)
	}
	return t, nil
}

// resolveSender finds the session owning e.repo.Checkout, if a single session
// does. In the main checkout, only a single live main-checkout session counts,
// so a human running vineyard send from the main checkout is reported as
// "main" rather than attributed to a stopped session.
func (e *sendEnv) resolveSender(sessions []session.Session) (session.Session, bool, error) {
	if e.repo.Checkout != e.repo.Root {
		for _, s := range sessions {
			if !s.MainCheckout && s.WorktreePath == e.repo.Checkout {
				return s, true, nil
			}
		}
		return session.Session{}, false, nil
	}
	var live []session.Session
	for _, s := range sessions {
		if !s.MainCheckout || s.Status == session.StatusPaused {
			continue
		}
		_, ok, err := e.manager.Running(s)
		if err != nil {
			return session.Session{}, false, err
		}
		if ok {
			live = append(live, s)
		}
	}
	if len(live) == 1 {
		return live[0], true, nil
	}
	return session.Session{}, false, nil
}

// awaitStill reports whether s's screen accepts input and remains unchanged
// across sendStillFor, with no PendingPrompt waiting ahead of peer messages.
func (e *sendEnv) awaitStill(s session.Session, term tui.Terminal, screen1 string) (bool, error) {
	if s.PendingPrompt != "" || !screenAcceptsInput(s.Program, screen1) {
		return false, nil
	}
	time.Sleep(sendStillFor)

	saved, err := e.store.Load()
	if err != nil {
		return false, err
	}
	i := slices.IndexFunc(saved, func(o session.Session) bool { return o.ID == s.ID })
	if i < 0 || saved[i].Status == session.StatusPaused {
		return false, errSessionStopped
	}
	if saved[i].PendingPrompt != "" {
		return false, nil
	}
	screen2, err := term.Capture(s.TmuxName)
	if err != nil {
		if !term.Exists(s.TmuxName) {
			return false, errSessionStopped
		}
		return false, err
	}
	if session.ScreenHash(screen1) != session.ScreenHash(screen2) {
		return false, nil
	}
	return screenAcceptsInput(saved[i].Program, screen2), nil
}

func resolveTarget(sessions []session.Session, to string) (session.Session, error) {
	for _, s := range sessions {
		if s.ID == to {
			return s, nil
		}
	}
	var matches []session.Session
	for _, s := range sessions {
		if s.Branch != "" && s.Branch == to {
			matches = append(matches, s)
		}
	}
	switch len(matches) {
	case 1:
		return matches[0], nil
	case 0:
		return session.Session{}, fmt.Errorf("no session %q", to)
	default:
		ids := make([]string, len(matches))
		for i, s := range matches {
			ids[i] = s.ID
		}
		return session.Session{}, fmt.Errorf("several sessions are on branch %q (%s); pass a session ID", to, strings.Join(ids, ", "))
	}
}

func screenAcceptsInput(program, screen string) bool {
	if strings.TrimSpace(ansi.Strip(screen)) == "" {
		return false
	}
	if session.AwaitingTrust(screen) || session.AwaitingPermission(program, screen) {
		return false
	}
	if session.ProgramName(program) == "claude" {
		if list, ok := subagent.Parse(screen); ok && (list.Shown != 0 || list.Focus >= 0) {
			return false
		}
	}
	return true
}

func formatPeerMessage(from string, replyable bool, body string) string {
	body = strings.TrimRight(body, "\r\n")
	if !replyable {
		return fmt.Sprintf("<peer_message from=\"%s\">\n%s\n</peer_message>",
			xmlAttrEscaper.Replace(from), body)
	}
	reply := "vineyard send " + session.ShellQuote(from) + " '...'"
	return fmt.Sprintf("<peer_message from=\"%s\" reply=\"%s\">\n%s\n</peer_message>",
		xmlAttrEscaper.Replace(from), xmlAttrEscaper.Replace(reply), body)
}

func deliverText(term tui.Terminal, name, text string) error {
	if err := term.Paste(name, text); err != nil {
		return err
	}
	time.Sleep(sendPasteDelay)
	return term.SendEnter(name)
}

func startDrainerProcess(dir, repoRoot, id string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	logFile, err := logfile.Open(tuiLog(dir))
	if err != nil {
		return err
	}
	defer logFile.Close()
	cmd := exec.Command(exe, "drain", repoRoot, id)
	cmd.Dir = "/"
	cmd.Stdout, cmd.Stderr = logFile, logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("starting message drainer: %w", err)
	}
	go cmd.Wait()
	return nil
}

func runDrainCommand(repoRoot, id string, stderr io.Writer) int {
	log.SetOutput(stderr)
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	log.SetPrefix(fmt.Sprintf("[%d] ", os.Getpid()))
	if err := runDrain(repoRoot, id); err != nil {
		log.Printf("drain %s: %v", id, err)
		return 1
	}
	return 0
}
