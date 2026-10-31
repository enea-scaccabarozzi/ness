package session_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
	"github.com/enea-scaccabarozzi/ness/internal/atom/herdr"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

// screenRT overrides what the pane read returns; the rest is the real herdr atom over the fake.
type screenRT struct {
	session.Runtime
	screen string
	err    error
}

func (r *screenRT) ReadScreen(string) (string, error) { return r.screen, r.err }

type bench struct {
	home  string
	t     *testing.T
	l     *session.Launcher
	s     *store.Store
	fakes string
	repo  string
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir = dir
	cmd.Env = store.CleanGitEnv(append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func newBench(t *testing.T) *bench {
	t.Helper()
	work := t.TempDir()
	repo, fakes := filepath.Join(work, "repo"), filepath.Join(work, "fakes")
	if err := os.MkdirAll(filepath.Join(repo, "data"), 0o750); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, "README"), []byte("x\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "-A")
	git(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "init")
	// Created after the commit, so the worktree checkout does not contain it and the copy runs.
	if err := os.WriteFile(filepath.Join(repo, ".env.example"), []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := testkit.WriteFakeDefaults(fakes); err != nil {
		t.Fatal(err)
	}
	fe := testkit.FakeExec{Dir: fakes}.Run
	path, err := store.Locate(repo)
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	home := filepath.Join(work, "home")
	return &bench{t: t, s: s, home: home, fakes: fakes, repo: repo, l: &session.Launcher{
		Store: s, Runtime: herdr.Runtime{Exec: fe}, Client: claude.Client{Exec: fe}, Exec: fe,
		Primary: repo, Home: home, Project: "p", Sessions: "docs/sessions", Copy: []string{".env.example"}, Link: []string{"data"},
		ReadyTimeout: 2 * time.Second, Read: claude.Read, Now: time.Now, Log: os.Stderr, Path: "/opt/ness/bin:/usr/bin",
	}}
}

func (b *bench) register(t *testing.T, id, scope string) {
	t.Helper()
	suffix := strings.TrimPrefix(id, "s-")
	l := store.Launch{
		ID: id, Scope: scope, Title: scope, Tier: "standard", Effort: "medium", Client: "claude",
		Record: "docs/sessions/" + scope, Base: "main", Worktree: ".worktrees/" + scope + "-" + suffix,
		Branch: "session/" + scope + "-" + suffix,
	}
	if err := b.s.AddLaunch(l, time.Now()); err != nil {
		t.Fatal(err)
	}
}

func (b *bench) run(brief string) error {
	r := session.Request{ID: sA, Model: "opus", Effort: "medium"}
	if brief != "" {
		r.Brief = []byte(brief)
	}
	return b.l.Run(r)
}

// transcript writes the file Claude Code would have written for the session's client id.
func (b *bench) transcript(id string) {
	b.t.Helper()
	dir := claude.ProjectDir(b.home, filepath.Join(b.repo, b.session(b.t).Worktree))
	if err := os.MkdirAll(dir, 0o750); err != nil {
		b.t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, id+".jsonl"), []byte("{}\n"), 0o600); err != nil {
		b.t.Fatal(err)
	}
}

func (b *bench) state(t *testing.T) testkit.HerdrFake {
	t.Helper()
	st, err := testkit.HerdrState(b.fakes)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func (b *bench) session(t *testing.T) store.Session {
	t.Helper()
	sess, err := b.s.Session(sA)
	if err != nil {
		t.Fatal(err)
	}
	return sess
}

func setHerdr(t *testing.T, fakes, key, value string) {
	t.Helper()
	if err := testkit.SetHerdr(fakes, key, value); err != nil {
		t.Fatal(err)
	}
}

const sA = "s-aaaaaaaaaaaa"

func TestLaunchEndToEndOverFakes(t *testing.T) {
	// Worktree, branch, links, brief, tab, agent, mark, phases; nothing typed (so no /model or
	// effort command ever reaches the pane).
	// covers: R-SESS-020, R-SESS-021, R-SESS-022, R-SESS-025, R-SESS-026, R-SESS-028, R-SESS-030, R-SESS-032, R-SESS-034
	// covers: R-SESS-037
	b := newBench(t)
	b.register(t, sA, "root")
	if err := b.run("# brief\n"); err != nil {
		t.Fatal(err)
	}
	sess := b.session(t)
	wt := filepath.Join(b.repo, sess.Worktree)
	if sess.Phase != "ready" || sess.ReadyAt.IsZero() || sess.Tab == "" || sess.Pane == "" ||
		sess.Agent != "p-root" || sess.Run != 1 {
		t.Fatalf("session after launch: %+v", sess)
	}
	if !strings.Contains(git(t, b.repo, "worktree", "list"), sess.Worktree) ||
		!strings.Contains(git(t, b.repo, "branch", "--list", sess.Branch), sess.Branch) {
		t.Fatal("no worktree or branch")
	}
	git(t, wt, "merge-base", "--is-ancestor", "main", "HEAD") // fails the test when it is not
	if data, err := os.ReadFile(filepath.Join(wt, "docs", "sessions", "root", "brief.md")); err != nil ||
		string(data) != "# brief\n" {
		t.Fatalf("brief: %q, %v", data, err)
	}
	if target, err := os.Readlink(filepath.Join(wt, "data")); err != nil || target != filepath.Join(b.repo, "data") {
		t.Fatalf("link: %q, %v", target, err)
	}
	info, err := os.Lstat(filepath.Join(wt, ".env.example"))
	data, rerr := os.ReadFile(filepath.Join(wt, ".env.example"))
	if err != nil || rerr != nil || !info.Mode().IsRegular() || string(data) != "A=1\n" {
		t.Fatalf("copy: %v %v %q", err, rerr, data)
	}
	st := b.state(t)
	if len(st.Tabs) != 1 || len(st.Typed) != 0 || st.Tabs[0].Env["NESS_SESSION"] != sA || st.Tabs[0].Env["NESS_RUN"] != "1" ||
		st.Tabs[0].Env["PATH"] != "/opt/ness/bin:/usr/bin" { // the tab shell may not have ness on its PATH (no spec id states this)
		t.Fatalf("fake herdr: %+v", st)
	}
	if _, set := st.Tabs[0].Env["CLAUDE_CODE_EFFORT_LEVEL"]; set {
		t.Fatal("effort leaked into the tab environment")
	}
	args := strings.Join(st.Agents[0].Args, " ")
	if sess.ClientSession == "" || !strings.Contains(args, "--session-id "+sess.ClientSession) ||
		!strings.Contains(args, "--effort medium") {
		t.Fatalf("args %q, client session %q", args, sess.ClientSession)
	}
	if strings.Contains(args, session.Prompt(sA, "root", "docs/sessions/root")) {
		t.Fatalf("the prompt is still an argument: %q", args) // it goes in through agent prompt (R-SESS-025)
	}
}

// fault is one way a phase can fail, and how to let the resume succeed.
type fault struct {
	phase  string
	inject func(b *bench)
	heal   func(b *bench)
	brief  string // the brief of the failing run
	want   string // part of the error
}

func failGit(sub string) func(*bench) {
	return func(b *bench) {
		inner := b.l.Exec
		b.l.Exec = func(dir, name string, args ...string) ([]byte, error) {
			if name == "git" && len(args) > 1 && args[0] == "worktree" && args[1] == sub {
				return nil, errors.New("injected failure at git worktree " + sub)
			}
			return inner(dir, name, args...)
		}
	}
}

func faults() []fault {
	nop := func(*bench) {}
	return []fault{
		{
			phase: "creating-worktree", brief: "b", want: "injected failure", inject: failGit("add"),
			heal: func(b *bench) { b.l.Exec = testkit.FakeExec{Dir: b.fakes}.Run },
		},
		{
			phase: "linking-files", brief: "b", want: "is a directory",
			inject: func(b *bench) { b.l.Copy = []string{"data"}; b.l.Link = nil },
			heal:   func(b *bench) { b.l.Copy = nil },
		},
		{phase: "writing-brief", brief: "", want: "no brief", inject: nop, heal: nop},
		{
			phase: "opening-tab", brief: "b", want: "injected failure",
			inject: func(b *bench) { setHerdr(b.t, b.fakes, "fail_next", "tab create") }, heal: nop,
		},
		{
			phase: "starting-agent", brief: "b", want: "injected failure",
			inject: func(b *bench) { setHerdr(b.t, b.fakes, "fail_next", "agent start") }, heal: nop,
		},
		{
			phase: "sending-brief", brief: "b", want: "not on the tab's PATH",
			inject: func(b *bench) {
				b.l.Runtime = &screenRT{Runtime: b.l.Runtime, screen: "bash: claude: command not found"}
			},
			heal: func(b *bench) { b.l.Runtime = b.l.Runtime.(*screenRT).Runtime },
		},
		{
			phase: "waiting-ready", brief: "b", want: "did not report ready",
			inject: func(b *bench) {
				b.l.ReadyTimeout = 200 * time.Millisecond
				setHerdr(b.t, b.fakes, "ready_on_start", "false")
			},
			heal: func(b *bench) {
				if err := b.s.MarkReady(sA, time.Now()); err != nil { // the delayed hook arrives
					b.t.Fatal(err)
				}
			},
		},
	}
}

func TestLaunchPersistsPhaseBeforeEachStep(t *testing.T) {
	// A fault at every phase leaves the store at that phase and nothing of the next steps
	// (R-SESS-031); resume continues from there with one worktree, branch, tab and agent
	// (R-SESS-045).
	// covers: R-SESS-031, R-SESS-044, R-SESS-045
	opened := map[string]bool{"sending-brief": true, "waiting-ready": true}
	for _, f := range faults() {
		t.Run(f.phase, func(t *testing.T) {
			b := newBench(t)
			b.register(t, sA, "root")
			f.inject(b)
			err := b.run(f.brief)
			if err == nil || !strings.Contains(err.Error(), f.want) {
				t.Fatalf("error %v, want %q", err, f.want)
			}
			sess, st := b.session(t), b.state(t)
			if sess.Phase != f.phase || !sess.ReadyAt.IsZero() && !opened[f.phase] { // the fake marks at agent start
				t.Fatalf("phase %q ready %v, want %q", sess.Phase, sess.ReadyAt, f.phase)
			}
			wantTabs, wantAgents := 0, 0
			if opened[f.phase] || f.phase == "starting-agent" {
				wantTabs = 1
			}
			if opened[f.phase] {
				wantAgents = 1
			}
			if len(st.Tabs) != wantTabs || len(st.Agents) != wantAgents {
				t.Fatalf("after the fault: tabs %d agents %d, want %d and %d", len(st.Tabs), len(st.Agents),
					wantTabs, wantAgents)
			}
			f.heal(b)
			if err := b.run("b"); err != nil {
				t.Fatalf("resume: %v", err)
			}
			st = b.state(t)
			live := 0
			for _, a := range st.Agents {
				if a.Status != "gone" {
					live++
				}
			}
			if after := b.session(t); after.Phase != "ready" || len(st.Tabs) != 1 || live != 1 ||
				strings.Count(git(t, b.repo, "worktree", "list"), ".worktrees/") != 1 {
				t.Fatalf("after resume: %+v tabs %d live agents %d", after, len(st.Tabs), live)
			}
		})
	}
}

func TestResumeReusesTheBoundTab(t *testing.T) {
	// A crash after BindTab but before the next SetPhase leaves the phase at opening-tab with the
	// tab bound and run 1: the resume keeps that tab, never opens a second one.
	// covers: R-SESS-045
	b := newBench(t)
	b.register(t, sA, "root")
	setHerdr(t, b.fakes, "fail_next", "tab create")
	if err := b.run("b"); err == nil {
		t.Fatal("injected tab create failure did not fail the launch")
	}
	sess := b.session(t)
	tab, err := b.l.Runtime.OpenTab(session.TabSpec{
		Dir: filepath.Join(b.repo, sess.Worktree), Label: sess.Title,
		Env: map[string]string{"NESS_SESSION": sA, "NESS_RUN": "1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.s.BindTab(sA, tab.Pane, tab.ID, 1); err != nil {
		t.Fatal(err)
	}
	if err := b.run(""); err != nil {
		t.Fatalf("resume: %v", err)
	}
	after, st := b.session(t), b.state(t)
	if len(st.Tabs) != 1 || after.Tab != tab.ID || after.Run != 1 || after.Phase != "ready" {
		t.Fatalf("after resume: %+v, %d tabs", after, len(st.Tabs))
	}
}

// slowStartRT starts the agent, then reports herdr's timeout, as when herdr waits for an idle
// client that is already working on its prompt.
type slowStartRT struct{ session.Runtime }

func (r slowStartRT) StartAgent(a session.AgentSpec) error {
	if err := r.Runtime.StartAgent(a); err != nil {
		return err
	}
	return errors.New("herdr agent start: timed out waiting for the agent to be ready")
}

func TestStartTimeoutWithTheAgentUpContinues(t *testing.T) {
	// covers: R-SESS-030, R-SESS-032
	for _, ready := range []string{"true", "false"} { // the mark arrived, or only herdr lists the agent
		b := newBench(t)
		b.register(t, sA, "root")
		b.l.Runtime = slowStartRT{b.l.Runtime}
		setHerdr(t, b.fakes, "ready_on_start", ready)
		if ready == "false" {
			b.l.ReadyTimeout = 300 * time.Millisecond
			go func() { <-time.After(100 * time.Millisecond); _ = b.s.MarkReady(sA, time.Now()) }()
		}
		if err := b.run("b"); err != nil {
			t.Fatalf("ready_on_start %s: %v", ready, err)
		}
		if sess := b.session(t); sess.Phase != "ready" || len(b.state(t).Agents) != 1 {
			t.Fatalf("ready_on_start %s: %+v", ready, sess)
		}
	}
}

func TestLaunchStallsWithoutTheMark(t *testing.T) {
	// covers: R-SESS-033
	b := newBench(t)
	b.l.ReadyTimeout = 300 * time.Millisecond
	b.register(t, sA, "root")
	setHerdr(b.t, b.fakes, "ready_on_start", "false")
	if err := b.run("b"); !errors.Is(err, session.ErrStalled) {
		t.Fatalf("err = %v", err)
	}
	sess := b.session(t)
	if sess.Phase != "waiting-ready" || session.Shown(sess, time.Now().Add(time.Second), b.l.ReadyTimeout) != "stalled launch" {
		t.Fatalf("after stall: %+v", sess)
	}
}

func TestLaunchWaitsForADelayedMark(t *testing.T) {
	// The fake client's hook writes the mark late, while the launch is waiting.
	// covers: R-SESS-032
	b := newBench(t)
	b.register(t, sA, "root")
	setHerdr(b.t, b.fakes, "ready_on_start", "false")
	var marked atomic.Bool
	go func() {
		<-time.After(600 * time.Millisecond)
		if err := b.s.MarkReady(sA, time.Now()); err == nil {
			marked.Store(true)
		}
	}()
	if err := b.run("b"); err != nil || !marked.Load() {
		t.Fatalf("err %v, marked %v", err, marked.Load())
	}
	if sess := b.session(t); sess.Phase != "ready" {
		t.Fatalf("phase %q", sess.Phase)
	}
}

func TestLaunchReadyBeforeWaiting(t *testing.T) {
	// The mark arrives during agent start (the fake writes it); the phase never regresses, and
	// running again on a ready session is a no-op.
	// covers: R-SESS-032, R-HOOK-009
	b := newBench(t)
	b.register(t, sA, "root")
	if err := b.run("b"); err != nil {
		t.Fatal(err)
	}
	if sess := b.session(t); sess.Phase != "ready" {
		t.Fatalf("phase %q", sess.Phase)
	}
	if err := b.run(""); err != nil {
		t.Fatal(err)
	}
	if st := b.state(t); len(st.Tabs) != 1 || len(st.Agents) != 1 {
		t.Fatalf("tabs %d agents %d", len(st.Tabs), len(st.Agents))
	}
}

func TestResumeAfterTabGone(t *testing.T) {
	// R-SESS-046: the tab of a stalled launch was closed out of band; resume opens one new tab.
	// covers: R-SESS-045, R-SESS-046, R-SESS-048
	b := newBench(t)
	b.l.ReadyTimeout = 200 * time.Millisecond
	b.register(t, sA, "root")
	setHerdr(b.t, b.fakes, "ready_on_start", "false")
	if err := b.run("b"); !errors.Is(err, session.ErrStalled) {
		t.Fatalf("first run: %v", err)
	}
	before := b.session(t)
	setHerdr(b.t, b.fakes, "close_tab", before.Tab)
	setHerdr(b.t, b.fakes, "ready_on_start", "true")
	if err := b.run(""); err != nil {
		t.Fatalf("resume: %v", err)
	}
	after, st := b.session(t), b.state(t)
	if len(st.Tabs) != 1 || after.Tab == before.Tab || after.Pane == before.Pane || after.Run != 2 || after.Phase != "ready" {
		t.Fatalf("after resume: tabs %d, %+v", len(st.Tabs), after)
	}
}

func TestResumeAfterAgentExited(t *testing.T) {
	// The tab is there but the agent never ran or died: the agent restarts in the same tab.
	// covers: R-SESS-045, R-SESS-046
	b := newBench(t)
	b.l.ReadyTimeout = 200 * time.Millisecond
	b.register(t, sA, "root")
	setHerdr(b.t, b.fakes, "ready_on_start", "false")
	if err := b.run("b"); !errors.Is(err, session.ErrStalled) {
		t.Fatalf("first run: %v", err)
	}
	before := b.session(t)
	setHerdr(b.t, b.fakes, "exit", before.Agent)
	setHerdr(b.t, b.fakes, "ready_on_start", "true")
	if err := b.run(""); err != nil {
		t.Fatalf("resume: %v", err)
	}
	after, st := b.session(t), b.state(t)
	if len(st.Tabs) != 1 || after.Tab != before.Tab || after.Run != 1 || after.Phase != "ready" || len(st.Agents) != 2 {
		t.Fatalf("after resume: tabs %d agents %d, %+v", len(st.Tabs), len(st.Agents), after)
	}
}

func TestAmbiguousPaneReadIsNotARefusal(t *testing.T) {
	// covers: R-SESS-046
	b := newBench(t)
	b.l.Runtime = &screenRT{Runtime: b.l.Runtime, err: errors.New("pane read failed")}
	b.register(t, sA, "root")
	if err := b.run("b"); err != nil {
		t.Fatalf("launch refused on an unreadable pane: %v", err)
	}
	if sess := b.session(t); sess.Phase != "ready" {
		t.Fatalf("phase %q", sess.Phase)
	}
}

func TestLaunchNeverAttachesToAnotherAgent(t *testing.T) {
	// Another repository's agent already holds the name: the launch picks a distinct one and
	// nothing is typed anywhere.
	// covers: R-SESS-036
	b := newBench(t)
	b.register(t, sA, "root")
	setHerdr(b.t, b.fakes, "agent", "p-root,wX:p9")
	if err := b.run("b"); err != nil {
		t.Fatal(err)
	}
	sess, st := b.session(t), b.state(t)
	if sess.Agent == "p-root" || !strings.HasPrefix(sess.Agent, "p-root-") || len(st.Typed) != 0 || len(st.Agents[0].Args) != 0 {
		t.Fatalf("agent %q, typed %d, other agent args %v", sess.Agent, len(st.Typed), st.Agents[0].Args)
	}
}

func TestRelaunchOfAnExitedSession(t *testing.T) {
	// covers: R-SESS-047, R-SESS-048, R-SESS-051
	b := newBench(t)
	b.register(t, sA, "root")
	if err := b.run("b"); err != nil {
		t.Fatal(err)
	}
	first := b.session(t)
	b.transcript(first.ClientSession)
	setHerdr(b.t, b.fakes, "exit", first.Agent)
	if err := b.s.NewRun(sA, "opening-tab", "frontier", "medium", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := b.l.Run(session.Request{ID: sA, Model: "fable", Effort: "medium"}); err != nil {
		t.Fatal(err)
	}
	second, st := b.session(t), b.state(t)
	args := strings.Join(st.Agents[len(st.Agents)-1].Args, " ")
	if second.Run != 2 || second.Pane == first.Pane || second.Tier != "frontier" ||
		!strings.Contains(args, "--resume "+first.ClientSession) || !strings.Contains(args, "--model fable") ||
		second.ClientSession != first.ClientSession {
		t.Fatalf("relaunch: %+v args %q", second, args)
	}
}

func TestLaunchRefusesIllegalTransitions(t *testing.T) {
	// The store takes any phase string; the launcher does not.
	// covers: R-SESS-044
	idx := func(p string) int { return slices.Index(session.Phases, p) }
	for _, from := range session.Phases {
		for _, to := range session.Phases {
			want := to != "registered" && (idx(to) == idx(from) || idx(to) == idx(from)+1) // the exact graph
			rewind := idx(from) >= idx("starting-agent") && (to == "opening-tab" || to == "starting-agent")
			if got := session.Legal(from, to); got != (want || rewind) {
				t.Errorf("Legal(%s, %s) = %v", from, to, got)
			}
		}
	}
	if session.Legal("bogus", "opening-tab") || session.Legal("registered", "bogus") {
		t.Error("an unknown phase is legal")
	}
	b := newBench(t)
	b.register(t, sA, "root")
	if err := b.s.SetPhase(sA, "bogus", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := b.run("b"); err == nil || !strings.Contains(err.Error(), "bogus") {
		t.Fatalf("launch from an unknown phase: %v", err)
	}
	if st := b.state(t); len(st.Tabs) != 0 || b.session(t).Phase != "bogus" {
		t.Fatalf("a refused launch changed things: %+v", st)
	}
}

func TestRelaunchReusesTheStoredClientSession(t *testing.T) {
	// The transcript to resume comes from the store, not from a caller flag: a relaunch that
	// stalled without ever starting an agent leaves no ready mark, and the next resume (whose
	// request carries no flag) must still resume the first transcript and never replace it.
	// covers: R-SESS-047, R-SESS-027
	b := newBench(t)
	b.register(t, sA, "root")
	if err := b.run("b"); err != nil {
		t.Fatal(err)
	}
	first := b.session(t)
	b.transcript(first.ClientSession)
	setHerdr(t, b.fakes, "exit", first.Agent)
	if err := b.s.NewRun(sA, "opening-tab", "", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	setHerdr(t, b.fakes, "fail_next", "agent start")
	if err := b.l.Run(session.Request{ID: sA, Model: "opus", Effort: "medium"}); err == nil {
		t.Fatal("the relaunch should have stalled")
	}
	if got := b.session(t).ClientSession; got != first.ClientSession {
		t.Fatalf("stalled relaunch replaced the client session: %q, was %q", got, first.ClientSession)
	}
	if err := b.l.Run(session.Request{ID: sA, Model: "opus", Effort: "medium"}); err != nil { // no flag
		t.Fatal(err)
	}
	st := b.state(t)
	args := strings.Join(st.Agents[len(st.Agents)-1].Args, " ")
	if got := b.session(t).ClientSession; got != first.ClientSession || !strings.Contains(args, "--resume "+first.ClientSession) {
		t.Fatalf("second resume: client session %q, args %q", got, args)
	}
}

func TestResumeAdoptsATabNobodySaved(t *testing.T) {
	// ness died between opening the tab and saving its id: the resume finds the tab by its label
	// and worktree and binds it instead of opening a second one.
	// covers: R-SESS-045
	b := newBench(t)
	b.register(t, sA, "root")
	wt := filepath.Join(b.repo, b.session(t).Worktree)
	orphan, err := b.l.Runtime.OpenTab(session.TabSpec{
		Dir: wt, Label: "root",
		Env: map[string]string{"NESS_SESSION": sA, "NESS_RUN": "1"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := b.run("b"); err != nil {
		t.Fatal(err)
	}
	if sess, st := b.session(t), b.state(t); len(st.Tabs) != 1 || sess.Tab != orphan.ID || sess.Run != 1 {
		t.Fatalf("tabs %d, session %+v, orphan %+v", len(st.Tabs), sess, orphan)
	}
}

func TestRelaunchWithoutATranscriptStartsFreshUnderTheSameID(t *testing.T) {
	// The client never wrote a transcript, so `--resume` would fail: the relaunch starts fresh
	// under the stored id, which is never replaced.
	// covers: R-SESS-027, R-SESS-047
	b := newBench(t)
	b.register(t, sA, "root")
	if err := b.run("b"); err != nil {
		t.Fatal(err)
	}
	first := b.session(t)
	setHerdr(t, b.fakes, "exit", first.Agent)
	if err := b.s.NewRun(sA, "opening-tab", "", "", time.Now()); err != nil {
		t.Fatal(err)
	}
	if err := b.l.Run(session.Request{ID: sA, Model: "opus", Effort: "medium"}); err != nil {
		t.Fatal(err)
	}
	st := b.state(t)
	args := strings.Join(st.Agents[len(st.Agents)-1].Args, " ")
	if got := b.session(t).ClientSession; got != first.ClientSession ||
		!strings.Contains(args, "--session-id "+first.ClientSession) || strings.Contains(args, "--resume") {
		t.Fatalf("client session %q (was %q), args %q", got, first.ClientSession, args)
	}
}
