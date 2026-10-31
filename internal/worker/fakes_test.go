package worker_test

import (
	"database/sql"
	"fmt"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/shell"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

const (
	sA = "s-aaaaaaaaaaaa"
	sB = "s-bbbbbbbbbbbb"
	sC = "s-cccccccccccc"
	sD = "s-dddddddddddd"

	placeholder = "\x1b[2mTry \"fix the failing test\"\x1b[22m"
	idleScreen  = "assistant output\n╭────────╮\n│ > " + placeholder + "\n╰────────╯\n  ? for shortcuts\n"
	draftScreen = "assistant output\n╭────────╮\n│ > fix the lint first\n╰────────╯\n"
)

// fakePane is one pane of the in-memory runtime.
type fakePane struct {
	status, agent, screen, client string
	focused, live                 bool
}

// fakeRT is the runtime the worker tests drive: panes by id, everything typed, tabs closed, reads
// counted. The embedded nil Runtime makes any other method a panic, so a call the worker must
// not make fails the test.
type fakeRT struct {
	session.Runtime
	panes    map[string]*fakePane
	typed    []string // "<pane> <text>"
	enters   []string // panes Enter was pressed in
	closed   []string
	reads    int
	runErr   error
	closeErr error // returned by CloseTab, the pane staying
}

func (f *fakeRT) State(pane string) (session.PaneState, error) {
	p, ok := f.panes[pane]
	if !ok {
		return session.PaneState{}, fmt.Errorf("%w: pane %s not found", session.ErrGone, pane)
	}
	return session.PaneState{Status: p.status, Live: p.live, Agent: p.agent, Focused: p.focused, ClientSession: p.client}, nil
}

func (f *fakeRT) ReadANSI(pane string) (string, error) {
	p, ok := f.panes[pane]
	if !ok {
		return "", fmt.Errorf("%w: pane %s not found", session.ErrGone, pane)
	}
	f.reads++
	return p.screen, nil
}

func (f *fakeRT) Run(pane, text string) error {
	if f.runErr != nil {
		return f.runErr
	}
	f.typed = append(f.typed, pane+" "+text)
	return nil
}

// PressEnter records the pane: the worker never submits a box (R-SESS-151).
func (f *fakeRT) PressEnter(pane string) error {
	f.enters = append(f.enters, pane)
	return nil
}

// CloseTab drops the tab's pane: a later State is ErrGone, as with herdr.
func (f *fakeRT) CloseTab(id string) error {
	f.closed = append(f.closed, id)
	if f.closeErr != nil {
		return f.closeErr
	}
	delete(f.panes, strings.TrimPrefix(id, "t:"))
	return nil
}

// bench is a store with the worker over the fake runtime and a stepped clock.
type bench struct {
	t   *testing.T
	s   *store.Store
	rt  *fakeRT
	w   *worker.Worker
	now time.Time
	log string

	primary string
	// acts is what the fake client's transcript says per session; a session with no entry has no
	// reading (the client falls back to the screen).
	acts map[string]session.Activity
	// readFor is the conversation id the fake client was asked about, per session.
	readFor map[string]string
}

func newBench(t *testing.T) *bench {
	t.Helper()
	dir := filepath.Join(t.TempDir(), "ness")
	s, err := store.Open(filepath.Join(dir, "state.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	b := &bench{t: t, s: s, rt: &fakeRT{panes: map[string]*fakePane{}}, now: t0, log: filepath.Join(dir, "worker.log"),
		primary: t.TempDir()}
	clock := func() time.Time { return b.now }
	b.w = &worker.Worker{
		Store: s, Runtime: b.rt, Project: "demo-app",
		Clients: map[string]worker.Client{"claude": b.client()},
		Version: "0.1.0", Now: clock, Log: &worker.Log{Path: b.log, Now: clock, Err: os.Stderr},
		Stall: 30 * time.Minute, Quiet: 60 * time.Second,
	}
	return b
}

func paneOf(id string) string { return "p-" + strings.TrimPrefix(id, "s-") }

// session registers a ready session of client under parent (store.Operator for a root) with an
// idle live agent named like the pane, showing the empty box.
func (b *bench) session(id, parent, client string) {
	b.t.Helper()
	pane := paneOf(id)
	l := store.Launch{
		ID: id, Parent: parent, Scope: "x", Title: "x", Tier: "standard", Effort: "medium", Client: client,
		Record: "docs/sessions/x", Base: "main", Worktree: ".worktrees/x-" + id, Branch: "session/x-" + id,
	}
	if err := b.s.AddLaunch(l, b.now); err != nil {
		b.t.Fatal(err)
	}
	for _, err := range []error{
		b.s.BindTab(id, pane, "t:"+pane, 1), b.s.BindAgent(id, "agent-"+pane, "c-"+pane), b.s.MarkReady(id, b.now),
	} {
		if err != nil {
			b.t.Fatal(err)
		}
	}
	b.rt.panes[pane] = &fakePane{status: "idle", agent: "agent-" + pane, client: "c-" + pane, screen: idleScreen, live: true}
}

func (b *bench) pane(id string) *fakePane { return b.rt.panes[paneOf(id)] }

// pass runs one pass and waits for the floors it started, so a test reads their bookings.
func (b *bench) pass() {
	b.t.Helper()
	b.launch()
	b.w.Wait()
}

// launch runs one pass and leaves its floors running.
func (b *bench) launch() {
	b.t.Helper()
	if err := b.w.Pass(); err != nil {
		b.t.Fatal(err)
	}
}

// passes runs n passes, advancing the clock by step before each.
func (b *bench) passes(n int, step time.Duration) {
	b.t.Helper()
	for range n {
		b.now = b.now.Add(step)
		b.pass()
	}
}

func (b *bench) send(from, to, intent, body string, urgent bool) string {
	b.t.Helper()
	id, _, err := b.s.Send(from, to, intent, body, urgent, b.now)
	if err != nil {
		b.t.Fatal(err)
	}
	return id
}

// typedTo lists what was typed into the session's pane.
func (b *bench) typedTo(id string) []string {
	var out []string
	for _, l := range b.rt.typed {
		if text, ok := strings.CutPrefix(l, paneOf(id)+" "); ok {
			out = append(out, text)
		}
	}
	return out
}

func (b *bench) logLines() []string {
	data, err := os.ReadFile(b.log) //nolint:gosec // the bench's own log
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func (b *bench) logCount(sub string) int {
	n := 0
	for _, l := range b.logLines() {
		if strings.Contains(l, sub) {
			n++
		}
	}
	return n
}

// listed lists what the recipient's inbox would show, without pulling.
func (b *bench) listed(recipient string) []store.Item {
	b.t.Helper()
	items, err := b.s.Listed(recipient)
	if err != nil {
		b.t.Fatal(err)
	}
	return items
}

// fakeRunner answers floors from a script of results, in order, and records each command.
type fakeRunner struct {
	mu      sync.Mutex // floors run in goroutines
	results []shell.Result
	cmds    []shell.Cmd
}

func (f *fakeRunner) run(c shell.Cmd) (shell.Result, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.cmds = append(f.cmds, c)
	if len(f.results) == 0 {
		return shell.Result{}, nil // GREEN
	}
	r := f.results[0]
	f.results = f.results[1:]
	return r, nil
}

func exit(code int) shell.Result { return shell.Result{Exit: code, Output: []byte("out\n")} }

// probes wires the probe slot onto the bench with a seeded RNG and a recording poster.
func (b *bench) probes(seed uint64) (*fakeRunner, *[]string) {
	b.t.Helper()
	fr := &fakeRunner{}
	var posts []string
	b.w.Probes = &worker.Probes{
		Primary: b.primary, Redact: []string{"SECRET"},
		Getenv: func(k string) string { return map[string]string{"SECRET": "hunter2"}[k] },
		Run:    fr.run, Rand: rand.New(rand.NewPCG(seed, seed)),
		Post: func(channel, md, thread string) error { posts = append(posts, channel+"|"+thread+"|"+md); return nil },
	}
	return fr, &posts
}

// probeFile writes a probe file into the session's worktree and returns its repo-relative path.
func (b *bench) probeFile(id, name, content string) string {
	b.t.Helper()
	sess, err := b.s.Session(id)
	if err != nil {
		b.t.Fatal(err)
	}
	dir := filepath.Join(b.primary, sess.Worktree, "probes")
	if err = os.MkdirAll(dir, 0o750); err != nil {
		b.t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600); err != nil {
		b.t.Fatal(err)
	}
	return "probes/" + name
}

// start registers a probe for the session, first run now.
func (b *bench) start(id, path string) store.Probe {
	b.t.Helper()
	p, err := b.s.StartProbe(path, id, "", false, b.now, b.now)
	if err != nil {
		b.t.Fatal(err)
	}
	return p
}

func (b *bench) probe(id string) store.Probe {
	b.t.Helper()
	p, err := b.s.Probe(id)
	if err != nil {
		b.t.Fatal(err)
	}
	return p
}

// breakBooking makes every update of the probe's run count fail, through a second handle.
func (b *bench) breakBooking(id string) {
	b.t.Helper()
	db, err := sql.Open("sqlite", "file:"+filepath.Join(filepath.Dir(b.log), "state.sqlite3")+"?_pragma=busy_timeout(5000)")
	if err != nil {
		b.t.Fatal(err)
	}
	defer func() { _ = db.Close() }()
	q := `CREATE TRIGGER nobook BEFORE UPDATE OF runs ON probes WHEN old.id = '` + id +
		`' BEGIN SELECT RAISE(ABORT, 'database is locked'); END`
	if _, err = db.ExecContext(b.t.Context(), q); err != nil {
		b.t.Fatal(err)
	}
}
