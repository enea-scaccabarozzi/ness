package session

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// Exec runs a program; cli.SystemExec in production, the fake in tests.
type Exec func(dir, name string, args ...string) ([]byte, error)

// Launcher runs the launch phases of a registered session (design §5 "Sessions"). Primary is
// the primary checkout; worktrees go under its .worktrees/. Every step is idempotent so a
// stalled launch resumes from its stored phase (R-SESS-045).
type Launcher struct {
	Store                      *store.Store
	Runtime                    Runtime
	Client                     Client
	Exec                       Exec
	Primary, Project, Sessions string
	Home                       string // the user's home, where the client keeps its transcripts
	Path                       string // PATH of the ness that opens the tab; the tab shell may lack ness
	Copy, Link                 []string
	ReadyTimeout               time.Duration
	// Read classifies the client's screen (its input box); nil for a client without a reader,
	// whose unsubmitted prompt then fails the launch instead of getting Enter (R-SESS-150).
	Read            func(ansi string, now time.Time) Screen
	SkipPermissions bool // launch.permissions = "skip" (R-SESS-125)
	Now             func() time.Time
	Log             io.Writer
}

// Request is one run of the phases: the resolved model and effort, the brief to write (nil
// keeps the existing one). Whether the transcript is resumed is not the caller's to say: the
// stored client session and its transcript decide (R-SESS-047).
type Request struct {
	ID, Model, Effort, FallbackModel, Ness string
	Brief                                  []byte
}

// Legal reports whether the launch may move from one stored phase to another (R-SESS-044): to
// the same phase (its step is re-run, R-SESS-045) or the next one in Phases, never skipping
// ahead and never back to registered. The one way back is the resume of a launch past the agent
// start, to opening-tab (the tab is gone) or starting-agent (the agent is gone). The store
// accepts any phase string; this is where the machine is enforced.
func Legal(from, to string) bool {
	i, j := slices.Index(Phases, from), slices.Index(Phases, to)
	if i < 0 || j <= 0 {
		return false
	}
	if j == i || j == i+1 {
		return true
	}
	return i >= slices.Index(Phases, "starting-agent") && slices.Contains([]string{"opening-tab", "starting-agent"}, to)
}

// Run executes the phases from the resume point, persisting each before its side effect
// (R-SESS-031) and refusing any transition that is not Legal. It returns ErrStalled when the
// ready mark does not arrive (R-SESS-033).
func (l *Launcher) Run(r Request) error {
	sess, err := l.Store.Session(r.ID)
	if err != nil {
		return err
	}
	if sess.Adopted || sess.Phase == store.PhaseReady && !sess.ReadyAt.IsZero() {
		return nil
	}
	if !slices.Contains(Phases, sess.Phase) {
		return fmt.Errorf("session %s is in unknown launch phase %q", sess.ID, sess.Phase)
	}
	from, err := l.resumePoint(sess)
	if err != nil {
		return err
	}
	for _, phase := range NextPhases(from) {
		if err := l.enter(&sess, phase, r); err != nil {
			return err
		}
	}
	return nil
}

// enter persists phase after checking the transition is Legal, then runs its step.
func (l *Launcher) enter(sess *store.Session, phase string, r Request) error {
	if !Legal(sess.Phase, phase) {
		return fmt.Errorf("session %s: %s after %s is not a legal launch transition", sess.ID, phase, sess.Phase)
	}
	if err := l.Store.SetPhase(sess.ID, phase, l.Now()); err != nil {
		return err
	}
	sess.Phase = phase
	steps := map[string]func(*store.Session, Request) error{
		"creating-worktree": l.createWorktree, "linking-files": l.linkFiles, "writing-brief": l.writeBrief,
		"opening-tab": l.openTab, "starting-agent": l.startAgent, "sending-brief": l.receipt,
		"waiting-ready": l.waitReady,
	}
	if step := steps[phase]; step != nil {
		if err := step(sess, r); err != nil {
			return fmt.Errorf("%s: %w", phase, err)
		}
	}
	return nil
}

// resumePoint is the phase a run starts at: the stored one, except that a launch past the agent
// start first checks what is still there. A missing tab resumes at opening-tab, a tab without a
// live agent at starting-agent; both fine means the stored step again (R-SESS-045, R-SESS-046).
// A ready mark does not skip the check: a trust dialog accepted after a stall writes it before
// the prompt went in (R-SESS-025).
func (l *Launcher) resumePoint(s store.Session) (string, error) {
	if slices.Index(Phases, s.Phase) < slices.Index(Phases, "starting-agent") {
		return s.Phase, nil
	}
	tabs, err := l.Runtime.Tabs()
	if err != nil {
		return "", err
	}
	if !slices.ContainsFunc(tabs, func(t Tab) bool { return t.ID == s.Tab }) {
		return "opening-tab", nil
	}
	st, err := l.Runtime.State(s.Pane)
	if errors.Is(err, ErrGone) {
		return "opening-tab", nil
	}
	if err != nil {
		return "", err
	}
	if !st.Live {
		return "starting-agent", nil
	}
	return s.Phase, nil
}

func (l *Launcher) say(format string, a ...any) {
	if l.Log != nil {
		fmt.Fprintf(l.Log, format+"\n", a...) //nolint:errcheck // a progress note; a broken log is not a launch failure
	}
}

// Prepare makes an adopted root's worktree, linked and copied files and brief, as a launch does
// before its tab (R-SESS-038): the running conversation is the root's client.
func (l *Launcher) Prepare(id string, brief []byte) error {
	sess, err := l.Store.Session(id)
	if err != nil {
		return err
	}
	r := Request{ID: id, Brief: brief}
	for _, step := range []func(*store.Session, Request) error{l.createWorktree, l.linkFiles, l.writeBrief} {
		if serr := step(&sess, r); serr != nil {
			return serr
		}
	}
	return nil
}

func (l *Launcher) worktreeDir(s *store.Session) string { return filepath.Join(l.Primary, s.Worktree) }

// createWorktree adds the branch from the base ref and the worktree under the primary checkout
// (R-SESS-020, R-SESS-021). An existing worktree is kept.
func (l *Launcher) createWorktree(s *store.Session, _ Request) error {
	dir := l.worktreeDir(s)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return nil
	}
	unlock, err := lockWorktrees(Git{l.Exec}, l.Primary)
	if err != nil {
		return err
	}
	defer unlock()
	if _, err = l.Exec(l.Primary, "git", "worktree", "prune"); err != nil {
		return err
	}
	args := []string{"worktree", "add", "-q"}
	if _, rerr := l.Exec(l.Primary, "git", "rev-parse", "--verify", "--quiet", "refs/heads/"+s.Branch); rerr == nil {
		args = append(args, dir, s.Branch) // the branch survived an earlier attempt
	} else {
		args = append(args, "-b", s.Branch, dir, s.Base)
	}
	_, err = l.Exec(l.Primary, "git", args...)
	return err
}

// linkFiles symlinks worktree.link and copies worktree.copy from the primary checkout
// (R-SESS-034). A source missing in the primary is skipped with a note.
func (l *Launcher) linkFiles(s *store.Session, _ Request) error {
	dir := l.worktreeDir(s)
	for _, p := range l.Link {
		if err := l.place(dir, p, os.Symlink); err != nil {
			return err
		}
	}
	for _, p := range l.Copy {
		if err := l.place(dir, p, copyFile); err != nil {
			return err
		}
	}
	return nil
}

// place puts the primary's p at the same path in the worktree with put, unless it is already
// there or the primary has none.
func (l *Launcher) place(dir, p string, put func(src, dst string) error) error {
	src, dst := filepath.Join(l.Primary, p), filepath.Join(dir, p)
	if _, err := os.Lstat(dst); err == nil {
		return nil
	}
	if _, err := os.Stat(src); errors.Is(err, fs.ErrNotExist) {
		l.say("ness: worktree path %s is not in the primary checkout; skipped", p)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
		return err
	}
	return put(src, dst)
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src) //nolint:gosec // a path from the project's own config
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0o600) //nolint:gosec // inside the worktree, from the project's own config
}

// writeBrief puts the brief at <worktree>/<record>/brief.md (R-SESS-022, R-SESS-049).
func (l *Launcher) writeBrief(s *store.Session, r Request) error {
	path := filepath.Join(l.worktreeDir(s), s.Record, "brief.md")
	if r.Brief == nil {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		return fmt.Errorf("no brief at %s; pass --brief FILE", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, r.Brief, 0o600)
}

// openTab opens the session's tab beside its manager's, with NESS_SESSION and NESS_RUN in its
// environment (R-SESS-030, R-SESS-037), plus PATH (not in the spec). A tab that still exists is
// reused. A tab opened by an earlier attempt that died before saving its id is found by what a
// launch gives it (label and worktree) and adopted, never duplicated. A dead one is replaced.
func (l *Launcher) openTab(s *store.Session, _ Request) error {
	tabs, err := l.Runtime.Tabs()
	if err != nil {
		return err
	}
	run, dir := s.Run+1, l.worktreeDir(s)
	if slices.ContainsFunc(tabs, func(t Tab) bool { return t.ID == s.Tab && s.Tab != "" }) {
		return nil
	}
	tab, found := orphanTab(tabs, *s, dir)
	if !found {
		if tab, err = l.Runtime.OpenTab(TabSpec{
			Dir: dir, Label: s.Title, Near: l.managerTab(*s),
			Env: tabEnv(s.ID, run, l.Path),
		}); err != nil {
			return err
		}
	}
	if berr := l.Store.BindTab(s.ID, tab.Pane, tab.ID, run); berr != nil {
		return berr
	}
	s.Pane, s.Tab, s.Run, s.Agent = tab.Pane, tab.ID, run, ""
	return nil
}

// managerTab is the tab of the session's manager, "" for a root or a manager without one: the
// runtime then opens the tab in its own default place (R-SESS-030).
func (l *Launcher) managerTab(s store.Session) string {
	if s.Parent == store.Operator {
		return ""
	}
	m, err := l.Store.Session(s.Parent)
	if err != nil {
		return "" // a manager the store cannot read still gets its child, in the default place
	}
	return m.Tab
}

// tabEnv is the tab's environment. PATH is passed so the SessionStart hook (`command -v ness`)
// finds ness in a tab shell whose own PATH lacks it; an empty path leaves the shell's alone.
func tabEnv(id string, run int, path string) map[string]string {
	env := map[string]string{"NESS_SESSION": id, "NESS_RUN": strconv.Itoa(run)}
	if path != "" {
		env["PATH"] = path
	}
	return env
}

// orphanTab finds the tab an earlier first attempt opened but never saved.
// ponytail: only a first launch adopts; after a relaunch the exited run's tab has the same label
// and directory and cannot be told from an orphan (R-SESS-048 wants a new pane). Upgrade by
// recording the tab before opening it.
func orphanTab(tabs []Tab, s store.Session, dir string) (Tab, bool) {
	if s.Run != 0 {
		return Tab{}, false
	}
	i := slices.IndexFunc(tabs, func(t Tab) bool { return t.Label == s.Title && t.Dir == dir })
	if i < 0 {
		return Tab{}, false
	}
	return tabs[i], true
}

// startAgent starts the client under a unique herdr name, without a prompt: herdr reports a
// client ready in seconds, not after its first turn (R-SESS-025, R-SESS-027, R-SESS-035,
// R-SESS-036). A live agent in the pane is kept.
func (l *Launcher) startAgent(s *store.Session, r Request) error {
	if s.Agent != "" {
		if st, err := l.Runtime.State(s.Pane); err == nil && st.Live {
			return nil
		}
	}
	live, err := l.Runtime.Agents()
	if err != nil {
		return err
	}
	name := AgentName(l.Project, s.Scope, s.ID, live)
	// The id is generated once and kept. Whether to resume it is the client's word on its
	// transcript, never a flag of the caller: a client that never wrote one cannot resume, so it
	// starts fresh under the same id (R-SESS-027, R-SESS-047).
	clientSession := s.ClientSession
	if clientSession == "" {
		clientSession = l.Client.NewSessionID()
	}
	resume := clientSession != "" && l.hasTranscript(l.worktreeDir(s), clientSession)
	spec := LaunchSpec{
		Dir: l.worktreeDir(s), Model: r.Model, Effort: r.Effort,
		SessionID: clientSession, FallbackModel: r.FallbackModel, Ness: r.Ness, Resume: resume,
		Name: ClientName(l.Project, s.Title), SkipPermissions: l.SkipPermissions,
	}
	if berr := l.Store.BindAgent(s.ID, name, clientSession); berr != nil {
		return berr
	}
	s.Agent, s.ClientSession = name, clientSession
	err = l.Runtime.StartAgent(AgentSpec{Name: name, Pane: s.Pane, Kind: l.Client.Name(), Args: l.Client.Args(spec)})
	if err != nil && l.agentUp(s.ID, name, s.Pane) {
		// herdr answers agent_not_ready at once for a client blocked on a dialog (trust): the
		// prompt step waits it out.
		l.say("ness: herdr agent start: %v; agent %s is up, continuing", err, name)
		return nil
	}
	return err
}

// agentUp reports whether a start that herdr reported as failed got the client running anyway:
// the hook wrote the ready mark, herdr lists the agent under its name, or the pane's agent is
// blocked on a dialog (herdr's agent_not_ready), which the prompt step waits out.
func (l *Launcher) agentUp(id, name, pane string) bool {
	if cur, err := l.Store.Session(id); err == nil && !cur.ReadyAt.IsZero() {
		return true
	}
	if live, err := l.Runtime.Agents(); err == nil && slices.Contains(live, name) {
		return true
	}
	st, err := l.Runtime.State(pane)
	return err == nil && st.Live && st.Status == "blocked"
}

// hasTranscript reports whether the client wrote a transcript for the session (Codex never does).
func (l *Launcher) hasTranscript(dir, id string) bool {
	_, err := l.Client.Transcript(l.Home, dir, id)
	return err == nil
}

// receipt checks the pane once after the start, then submits the first prompt (R-SESS-025). An
// ambiguous or unreadable screen is not a refusal (R-SESS-046); only a shell that says the client
// is missing fails the launch.
func (l *Launcher) receipt(s *store.Session, _ Request) error {
	screen, err := l.Runtime.ReadScreen(s.Pane)
	if err != nil {
		l.say("ness: pane %s could not be read after the start; continuing", s.Pane)
	} else if strings.Contains(screen, l.Client.Name()+": command not found") {
		return fmt.Errorf("%s is not on the tab's PATH", l.Client.Name())
	}
	return l.prompt(s)
}

// prompt submits the one-line prompt once the agent is not blocked on a dialog (a trust prompt);
// still blocked at the ready timeout, the launch stalls here and --resume submits it (R-SESS-025,
// R-SESS-033). The submission is confirmed (R-SESS-150).
// ponytail: a launch that dies between the submission and the next phase's write submits it
// again on --resume; record the submission if that ever shows up.
func (l *Launcher) prompt(s *store.Session) error {
	sub := &submission{l: l, pane: s.Pane, text: Prompt(s.ID, s.Scope, s.Record)}
	return waitFor(sub.poll, l.ReadyTimeout, 250*time.Millisecond)
}

// submission is one launch's attempt at its first prompt. The text is typed at most once per run:
// a client still starting draws it in its box seconds after herdr gave up on the Enter, and typing
// it again would append a second copy. A prompt in the box gets Enter, at most submitAttempts
// times, then the launch fails; one that never shows up stalls the launch and --resume types it
// again (R-SESS-150).
type submission struct {
	l          *Launcher
	pane, text string
	sent       bool
	enters     int
}

func (p *submission) poll() (bool, error) {
	st, err := p.l.Runtime.State(p.pane)
	switch {
	case err != nil || st.Status == "blocked":
		return false, err
	case p.sent && (st.Status == "working" || st.Status == "done"): // taken after herdr gave up
		return true, nil
	case p.l.inBox(p.pane, p.text):
		if p.enters == submitAttempts {
			return false, fmt.Errorf("%w: the first prompt sits in pane %s's input box after %d Enter presses",
				ErrNotSubmitted, p.pane, submitAttempts)
		}
		p.enters++
		return p.l.pressEnter(p.pane)
	case p.sent:
		return false, nil
	}
	return p.send()
}

// send types the prompt through the runtime's prompt call.
func (p *submission) send() (bool, error) {
	err := p.l.Runtime.Prompt(p.pane, p.text)
	switch {
	case errors.Is(err, ErrBlocked): // a dialog came up after the read: nothing was sent
		return false, nil
	case errors.Is(err, ErrNotSubmitted) && p.l.Read == nil: // no box to look at: never Enter blind
		return false, fmt.Errorf("pane %s: %w", p.pane, err)
	case errors.Is(err, ErrNotSubmitted): // the next polls look at the box
		p.sent = true
		return false, nil
	}
	return err == nil, err
}

// submitAttempts bounds the Enters pressed on an unsubmitted prompt, each waited submitWait for the
// agent's turn to start (R-SESS-150).
const (
	submitAttempts = 3
	submitWait     = 5 * time.Second
)

// inBox reports whether the client's input box starts with text: the prompt went in, its Enter did
// not. Only then is Enter pressed, so it never lands on a dialog (Claude Code's trust dialog
// answers Enter with "No, exit"). Whitespace is ignored: the box wraps a long prompt.
func (l *Launcher) inBox(pane, text string) bool {
	if l.Read == nil {
		return false
	}
	screen, err := l.Runtime.ReadANSI(pane)
	if err != nil {
		return false
	}
	box := squash(l.Read(screen, l.Now()).Draft)
	return box != "" && strings.HasPrefix(squash(text), box)
}

func squash(s string) string { return strings.Join(strings.Fields(s), "") }

// pressEnter presses Enter in the pane and reports whether the agent's turn started (working, or
// blocked on a dialog the prompt raised) within submitWait.
func (l *Launcher) pressEnter(pane string) (bool, error) {
	l.say("ness: the prompt sits unsubmitted in pane %s; pressing Enter", pane)
	if err := l.Runtime.PressEnter(pane); err != nil {
		return false, err
	}
	err := waitFor(func() (bool, error) {
		st, err := l.Runtime.State(pane)
		return err == nil && (st.Status == "working" || st.Status == "blocked"), err
	}, submitWait, 250*time.Millisecond)
	if errors.Is(err, ErrStalled) {
		return false, nil
	}
	return err == nil, err
}

// waitReady waits for the mark the SessionStart hook writes (R-SESS-032, R-SESS-033).
func (l *Launcher) waitReady(s *store.Session, _ Request) error {
	return waitFor(func() (bool, error) {
		cur, err := l.Store.Session(s.ID)
		if err != nil {
			return false, err
		}
		return !cur.ReadyAt.IsZero(), nil
	}, l.ReadyTimeout, 250*time.Millisecond)
}
