package check_test

import (
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/check"
	"github.com/enea-scaccabarozzi/ness/internal/profile"
	"github.com/enea-scaccabarozzi/ness/internal/shell"
)

func TestRunCheckPassesOnExitZero(t *testing.T) {
	// covers: R-CHK-006
	r := &fakeRunner{t: t, byCmd: map[string]outcome{"ok": {res: shell.Result{Output: []byte("fine")}}}}
	if got := check.Run(openCtx("/w"), []profile.Check{runCheck("p/a", "open", "ok")}, check.Deps{Shell: r.run}); len(got) != 0 {
		t.Fatalf("failures %+v, want none", got)
	}
}

func TestRunCheckFailsClosed(t *testing.T) {
	// covers: R-CHK-006, R-CHK-008
	for _, tc := range []struct {
		name   string
		out    outcome
		detail string
	}{
		{"exit 3 with output", outcome{res: shell.Result{Exit: 3, Output: []byte("  bad thing\n\n")}}, "bad thing"},
		{"exit 3 without output", outcome{res: shell.Result{Exit: 3}}, "exit 3"},
		{"timeout", outcome{res: shell.Result{Exit: -1, TimedOut: true}}, "timed out"},
		{"cannot start", outcome{err: errors.New("fork/exec /bin/sh: no such file")}, "no such file"},
		{"cannot start with exit 0", outcome{err: errors.New("pipe broke")}, "pipe broke"},
	} {
		r := &fakeRunner{t: t, byCmd: map[string]outcome{"c": tc.out}}
		got := check.Run(openCtx("/w"), []profile.Check{runCheck("p/a", "open", "c")}, check.Deps{Shell: r.run})
		if len(got) != 1 {
			t.Errorf("%s: failures %+v, want exactly one", tc.name, got)
			continue
		}
		if !strings.Contains(got[0].Detail, tc.detail) {
			t.Errorf("%s: detail %q does not contain %q", tc.name, got[0].Detail, tc.detail)
		}
	}
}

func TestRunCheckTimeoutIsDeclaredOrThirtySeconds(t *testing.T) {
	// covers: R-CHK-006
	declared, undeclared := runCheck("p/a", "open", "slow"), runCheck("p/b", "open", "default")
	declared.Timeout = 5 * time.Second
	r := passing(t, "slow", "default")
	check.Run(openCtx("/w"), []profile.Check{declared, undeclared}, check.Deps{Shell: r.run})
	if got := r.cmd(t, "slow").Timeout; got != 5*time.Second {
		t.Errorf("declared timeout = %v, want 5s", got)
	}
	if got := r.cmd(t, "default").Timeout; got != 30*time.Second {
		t.Errorf("default timeout = %v, want 30s", got)
	}
}

func TestRunCheckEnvironmentPerEvent(t *testing.T) {
	// covers: R-CHK-004
	forbidden := map[string][]string{
		"open":      {"NESS_BRANCH", "NESS_INTO", "NESS_RECORD"},
		"integrate": {"NESS_SCOPE"},
		"close":     {"NESS_SCOPE"},
	}
	for _, tc := range []struct {
		ctx  check.Context
		want []string
	}{
		{openCtx("/wt"), []string{
			"NESS_EVENT=open", "NESS_SESSION=s-root", "NESS_ROOT=s-root", "NESS_WORKTREE=/wt",
			"NESS_PRIMARY=/primary", "NESS_SCOPE=fix",
		}},
		{integrateCtx("/wt"), []string{
			"NESS_EVENT=integrate", "NESS_SESSION=s-child", "NESS_ROOT=s-root", "NESS_WORKTREE=/wt",
			"NESS_PRIMARY=/primary", "NESS_BRANCH=session/fix-1", "NESS_INTO=session/root-1",
			"NESS_RECORD=docs/sessions/root/fix",
		}},
		{func() check.Context { c := integrateCtx("/wt"); c.Event = "close"; return c }(), []string{
			"NESS_EVENT=close", "NESS_BRANCH=session/fix-1", "NESS_INTO=session/root-1",
		}},
	} {
		r := passing(t, "c")
		check.Run(tc.ctx, []profile.Check{runCheck("p/a", tc.ctx.Event, "c")}, check.Deps{Shell: r.run})
		c := r.cmd(t, "c")
		for _, w := range tc.want {
			if !slices.Contains(c.Env, w) {
				t.Errorf("%s: env lacks %s; has %v", tc.ctx.Event, w, c.Env)
			}
		}
		for _, f := range forbidden[tc.ctx.Event] {
			for _, e := range c.Env {
				if strings.HasPrefix(e, f+"=") {
					t.Errorf("%s: env sets %s", tc.ctx.Event, e)
				}
			}
		}
		if c.Dir != "/wt" {
			t.Errorf("%s: Dir = %q, want /wt", tc.ctx.Event, c.Dir)
		}
	}
}

func TestRunCheckEnvironmentForTheOperator(t *testing.T) {
	// covers: R-CHK-004
	r := passing(t, "c")
	ctx := check.Context{Event: "open", Worktree: "/wt", Primary: "/primary", Scope: "r"}
	check.Run(ctx, []profile.Check{runCheck("p/a", "open", "c")}, check.Deps{Shell: r.run})
	env := r.cmd(t, "c").Env
	for _, w := range []string{"NESS_SESSION=", "NESS_ROOT="} {
		if !slices.Contains(env, w) {
			t.Errorf("env lacks the empty %s; has %v", w, env)
		}
	}
}

func TestRunCheckDetailIsCappedAt1500Bytes(t *testing.T) {
	// covers: R-CHK-008
	r := &fakeRunner{t: t, byCmd: map[string]outcome{
		"c": {res: shell.Result{Exit: 1, Output: []byte(strings.Repeat("abcdefghij", 300))}},
	}}
	got := check.Run(openCtx("/w"), []profile.Check{runCheck("p/a", "open", "c")}, check.Deps{Shell: r.run})
	if len(got) != 1 || len(got[0].Detail) == 0 || len(got[0].Detail) > 1500 {
		t.Fatalf("detail length = %v, want 1..1500", got)
	}
	if !strings.HasPrefix(got[0].Detail, "abcdefghij") {
		t.Errorf("detail %.20q does not start with the output", got[0].Detail)
	}
}

func TestFailuresAggregateInCheckOrder(t *testing.T) {
	// covers: R-CHK-008
	r := &fakeRunner{t: t, byCmd: map[string]outcome{
		"first": {res: shell.Result{Exit: 1}}, "second": {}, "third": {res: shell.Result{Exit: 2}},
	}}
	checks := []profile.Check{
		runCheck("a/first", "open", "first"), runCheck("a/second", "open", "second"),
		runCheck("local/third", "open", "third"),
	}
	got := ids(check.Run(openCtx("/w"), checks, check.Deps{Shell: r.run}))
	if want := []string{"a/first", "local/third"}; !slices.Equal(got, want) {
		t.Fatalf("failures %v, want %v", got, want)
	}
}

func TestUnknownKindFailsClosed(t *testing.T) {
	// covers: R-CHK-007
	c := profile.Check{ID: "p/odd", On: "open", Skill: "ness-entry", Section: "Communicate", Kind: "no-such-kind"}
	got := check.Run(openCtx("/w"), []profile.Check{c}, check.Deps{})
	if len(got) != 1 || !strings.Contains(got[0].Detail, "no-such-kind") {
		t.Fatalf("failures %+v, want one naming the kind", got)
	}
}
