package session_test

import (
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/herdr"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

const parentID = "s-pppppppppppp"

// registerChild registers sA as a child of parentID, a manager whose tab is parentTab ("" for none).
func (b *bench) registerChild(t *testing.T, parentTab string) {
	t.Helper()
	now := time.Now()
	if err := b.s.AddSession(parentID, "mgr", store.Operator, now); err != nil {
		t.Fatal(err)
	}
	if parentTab != "" {
		if err := b.s.BindTab(parentID, strings.Replace(parentTab, ":t", ":p", 1), parentTab, 1); err != nil {
			t.Fatal(err)
		}
	}
	l := store.Launch{
		ID: sA, Parent: parentID, Scope: "kid", Title: "kid", Tier: "standard", Effort: "medium", Client: "claude",
		Record: "docs/sessions/kid", Base: "main", Worktree: ".worktrees/kid-aaaaaaaaaaaa", Branch: "session/kid-aaaaaaaaaaaa",
	}
	if err := b.s.AddLaunch(l, now); err != nil {
		t.Fatal(err)
	}
}

func lastTab(t *testing.T, b *bench) testkit.HerdrTab {
	t.Helper()
	st := b.state(t)
	if len(st.Tabs) == 0 {
		t.Fatal("no tab opened")
	}
	return st.Tabs[len(st.Tabs)-1]
}

func TestChildTabOpensInItsManagersWorkspace(t *testing.T) {
	// A child lands beside its manager, not wherever herdr's focus is, and the manager's tab wins over the
	// caller's own workspace; a manager without a tab, or a root, uses the caller's workspace, else herdr's default.
	// covers: R-SESS-030
	for name, tc := range map[string]struct {
		parentTab, workspace, want string
	}{
		"beside the manager":            {"w7:t3", "", "w7:"},
		"manager wins over the caller":  {"w7:t3", "w9", "w7:"},
		"manager without a tab":         {"", "w9", "w9:"},
		"neither manager nor workspace": {"", "", "wF:"},
	} {
		t.Run(name, func(t *testing.T) {
			b := newBench(t)
			b.registerChild(t, tc.parentTab)
			b.l.Runtime = herdr.Runtime{Exec: herdr.Exec(b.l.Exec), Workspace: tc.workspace}
			if err := b.run("b"); err != nil {
				t.Fatal(err)
			}
			if got := lastTab(t, b).ID; !strings.HasPrefix(got, tc.want) {
				t.Fatalf("child tab %s, want workspace prefix %s", got, tc.want)
			}
		})
	}
}

func TestRootTabOpensInTheCallersWorkspace(t *testing.T) {
	// covers: R-SESS-030
	b := newBench(t)
	b.register(t, sA, "root")
	b.l.Runtime = herdr.Runtime{Exec: herdr.Exec(b.l.Exec), Workspace: "w9"}
	if err := b.run("b"); err != nil {
		t.Fatal(err)
	}
	if got := lastTab(t, b).ID; !strings.HasPrefix(got, "w9:") {
		t.Fatalf("root tab %s, want workspace w9", got)
	}
}

func TestResumeReopensTheTabInTheManagersWorkspace(t *testing.T) {
	// covers: R-SESS-030, R-SESS-048
	b := newBench(t)
	b.l.ReadyTimeout = 200 * time.Millisecond
	b.registerChild(t, "w7:t3")
	setHerdr(t, b.fakes, "ready_on_start", "false")
	if err := b.run("b"); !errors.Is(err, session.ErrStalled) {
		t.Fatalf("first run: %v", err)
	}
	first := b.session(t).Tab
	setHerdr(t, b.fakes, "close_tab", first)
	setHerdr(t, b.fakes, "ready_on_start", "true")
	if err := b.run(""); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if got := b.session(t).Tab; got == first || !strings.HasPrefix(got, "w7:") {
		t.Fatalf("resumed tab %s (was %s), want a new tab in w7", got, first)
	}
}

func TestPromptIsSubmittedOnceAfterTheClientIsReady(t *testing.T) {
	// The client starts without the prompt in its argv; the one-line prompt goes in through the
	// runtime's prompt call, never typed.
	// covers: R-SESS-025, R-SESS-026
	b := newBench(t)
	b.register(t, sA, "root")
	if err := b.run("b"); err != nil {
		t.Fatal(err)
	}
	sess, st := b.session(t), b.state(t)
	want := "You are ness session " + sA + " (root). Read docs/sessions/root/brief.md, then load ness-entry."
	if len(st.Prompted) != 1 || st.Prompted[0] != [2]string{sess.Pane, want} {
		t.Fatalf("prompted = %q, want one submission of %q to %s", st.Prompted, want, sess.Pane)
	}
	if args := strings.Join(st.Agents[0].Args, " "); strings.Contains(args, "You are ness") || strings.Contains(args, "brief.md") {
		t.Fatalf("agent args carry the prompt: %q", args)
	}
	if len(st.Typed) != 0 {
		t.Fatalf("a launch typed %q", st.Typed)
	}
}

func TestBlockedAgentGetsNoPromptAndResumeSubmitsIt(t *testing.T) {
	// A trust or approval dialog (status blocked) must not receive the prompt: the launch stalls,
	// and --resume submits it once the agent is idle.
	// covers: R-SESS-025, R-SESS-033
	b := newBench(t)
	b.l.ReadyTimeout = 300 * time.Millisecond
	b.register(t, sA, "root")
	setHerdr(t, b.fakes, "start_status", "blocked")
	if err := b.run("b"); !errors.Is(err, session.ErrStalled) {
		t.Fatalf("blocked launch: %v, want ErrStalled", err)
	}
	if st := b.state(t); len(st.Prompted) != 0 || len(st.Typed) != 0 {
		t.Fatalf("a blocked agent got input: prompted %q typed %q", st.Prompted, st.Typed)
	}
	sess := b.session(t)
	setHerdr(t, b.fakes, "status", sess.Agent+",idle")
	if err := b.run(""); err != nil {
		t.Fatalf("resume: %v", err)
	}
	st := b.state(t)
	if len(st.Prompted) != 1 || st.Prompted[0][0] != sess.Pane || st.Prompted[0][1] != session.Prompt(sA, "root", "docs/sessions/root") {
		t.Fatalf("prompted after resume = %q", st.Prompted)
	}
	if after := b.session(t); after.Phase != "ready" || len(st.Agents) != 1 {
		t.Fatalf("after resume: %+v, %d agents", after, len(st.Agents))
	}
	if err := b.run(""); err != nil || len(b.state(t).Prompted) != 1 {
		t.Fatalf("a finished launch submitted again: %v, %q", err, b.state(t).Prompted)
	}
}

func TestShownIsNotReadyWhileThePromptIsPending(t *testing.T) {
	// A ready mark written before the first prompt went in (the hook runs once a trust dialog is
	// accepted) does not make the launch ready: it shows as stalled past the timeout, and plain
	// as its phase before.
	// covers: R-SESS-025, R-SESS-033
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	for _, phase := range []string{"starting-agent", "sending-brief"} {
		s := store.Session{Phase: phase, ReadyAt: now.Add(-time.Hour), PhaseAt: now.Add(-61 * time.Second)}
		if got := session.Shown(s, now, time.Minute); got != "stalled launch" {
			t.Fatalf("marked, prompt pending in %s, shown as %q", phase, got)
		}
		s.PhaseAt = now.Add(-time.Second)
		if got := session.Shown(s, now, time.Minute); got != phase {
			t.Fatalf("young launch in %s shown as %q", phase, got)
		}
	}
	for _, phase := range []string{"waiting-ready", "ready"} {
		s := store.Session{Phase: phase, ReadyAt: now, PhaseAt: now.Add(-time.Hour)}
		if got := session.Shown(s, now, time.Minute); got != "ready" {
			t.Fatalf("marked in %s shown as %q", phase, got)
		}
	}
}

// blockOnce refuses the first agent prompt the way herdr does for an agent on a dialog.
func blockOnce(inner herdr.Exec) herdr.Exec {
	refused := false
	return func(dir, name string, args ...string) ([]byte, error) {
		if len(args) > 1 && args[0] == "agent" && args[1] == "prompt" && !refused {
			refused = true
			return nil, fmt.Errorf(`herdr agent prompt: exit status 1: {"error":{"code":"agent_blocked","message":"blocked"}}`)
		}
		return inner(dir, name, args...)
	}
}

func TestPromptRefusedAsBlockedIsWaitedOut(t *testing.T) {
	// A dialog that appears between the status read and the submission makes herdr refuse with
	// agent_blocked before sending anything: the launch waits and submits on a later poll, once.
	// covers: R-SESS-025
	b := newBench(t)
	b.register(t, sA, "root")
	b.l.Runtime = herdr.Runtime{Exec: blockOnce(herdr.Exec(b.l.Exec))}
	if err := b.run("b"); err != nil {
		t.Fatalf("launch: %v", err)
	}
	sess, st := b.session(t), b.state(t)
	if want := session.Prompt(sA, "root", "docs/sessions/root"); len(st.Prompted) != 1 || st.Prompted[0] != [2]string{sess.Pane, want} {
		t.Fatalf("prompted = %q, want the one prompt after the refusal", st.Prompted)
	}
	if sess.Phase != "ready" {
		t.Fatalf("phase %q", sess.Phase)
	}
}

func TestResumeReopensTheTabWhenTheMarkIsSetButThePromptIsPending(t *testing.T) {
	// The mark exists, the prompt never went in and the tab is gone: the resume reopens a tab (a
	// mark must not skip the check of what is still there).
	// covers: R-SESS-025, R-SESS-045, R-SESS-046
	b := newBench(t)
	b.register(t, sA, "root")
	if err := b.run("b"); err != nil {
		t.Fatal(err)
	}
	first := b.session(t)
	if err := b.s.SetPhase(sA, "sending-brief", time.Now()); err != nil {
		t.Fatal(err)
	}
	setHerdr(t, b.fakes, "close_tab", first.Tab)
	if err := b.run(""); err != nil {
		t.Fatalf("resume: %v", err)
	}
	after, st := b.session(t), b.state(t)
	if after.Tab == first.Tab || len(st.Tabs) != 1 || after.Phase != "ready" || len(st.Prompted) != 2 ||
		st.Prompted[1][0] != after.Pane {
		t.Fatalf("after resume: %+v, %d tabs, prompted %q", after, len(st.Tabs), st.Prompted)
	}
}
