package check_test

import (
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/check"
	"github.com/enea-scaccabarozzi/ness/internal/profile"
	"github.com/enea-scaccabarozzi/ness/internal/rule"
	"github.com/enea-scaccabarozzi/ness/internal/shell"
)

// fakeGit answers the one diff the branch-forbids kind may ask for, in dir only.
type fakeGit struct {
	dir, rng, diff string
	err            error
}

func (g fakeGit) Out(dir string, args ...string) (string, error) {
	if g.err != nil {
		return "", g.err
	}
	for len(args) > 1 && args[0] == "-c" { // config overrides precede the subcommand
		args = args[2:]
	}
	if dir != g.dir || len(args) == 0 || args[0] != "diff" || !slices.Contains(args, g.rng) {
		return "", fmt.Errorf("unexpected git call in %s: %v", dir, args)
	}
	return g.diff, nil
}

// fakeFacts returns fixed store facts.
type fakeFacts struct {
	landing, checkpoint time.Time
	err                 error
}

func (f fakeFacts) LastLanding(string) (time.Time, error)    { return f.landing, f.err }
func (f fakeFacts) LastCheckpoint(string) (time.Time, error) { return f.checkpoint, f.err }

// outcome is what the fake runner does for one command.
type outcome struct {
	res shell.Result
	err error
}

// fakeRunner returns the outcome registered for the command text, fails the test for a command
// nobody registered (a passing outcome is registered explicitly: no permissive mocks), and keeps
// the commands it was given, for the assertions on their environment.
type fakeRunner struct {
	t     *testing.T
	byCmd map[string]outcome
	cmds  []shell.Cmd
}

func (r *fakeRunner) run(c shell.Cmd) (shell.Result, error) {
	r.cmds = append(r.cmds, c)
	o, ok := r.byCmd[c.Command]
	if !ok {
		r.t.Errorf("unexpected command %q", c.Command)
		return shell.Result{Exit: 99}, nil
	}
	return o.res, o.err
}

// passing returns a runner that exits 0 for exactly these commands.
func passing(t *testing.T, cmds ...string) *fakeRunner {
	t.Helper()
	r := &fakeRunner{t: t, byCmd: map[string]outcome{}}
	for _, c := range cmds {
		r.byCmd[c] = outcome{}
	}
	return r
}

// cmd returns the command given for text, failing when there was none.
func (r *fakeRunner) cmd(t *testing.T, text string) shell.Cmd {
	t.Helper()
	for _, c := range r.cmds {
		if c.Command == text {
			return c
		}
	}
	t.Fatalf("command %q was never run; ran %v", text, r.cmds)
	return shell.Cmd{}
}

var base = time.Date(2026, 10, 15, 12, 0, 0, 0, time.UTC)

// openCtx is a root opening a child, rooted in the fixture tree dir.
func openCtx(dir string) check.Context {
	return check.Context{
		Event: "open", Child: true, Session: "s-root", Root: "s-root",
		Worktree: dir, Primary: "/primary", Scope: "fix",
	}
}

func integrateCtx(dir string) check.Context {
	return check.Context{
		Event: "integrate", Child: true, Session: "s-child", Root: "s-root", Worktree: dir,
		Primary: "/primary", Branch: "session/fix-1", Into: "session/root-1", Record: "docs/sessions/root/fix",
	}
}

func runCheck(id, event, command string) profile.Check {
	return profile.Check{ID: id, On: event, Skill: "ness-entry", Section: "Communicate", Run: command}
}

func kindCheck(kind string, args map[string]string, require ...string) profile.Check {
	return profile.Check{
		ID: "p/" + kind, On: "open", Skill: "ness-program", Section: "Rules", Message: "Fix " + kind + ".",
		Kind: kind, Args: args, Require: require,
	}
}

func ids(fs []check.Failure) []string {
	var out []string
	for _, f := range fs {
		out = append(out, f.ID)
	}
	return out
}

func TestRunOnlyRunsChecksOfTheEvent(t *testing.T) {
	// covers: R-CHK-001
	for _, event := range []string{"open", "integrate", "close"} {
		r := &fakeRunner{t: t, byCmd: map[string]outcome{}}
		checks := []profile.Check{
			runCheck("p/open", "open", "c-open"), runCheck("p/integrate", "integrate", "c-integrate"),
			runCheck("p/close", "close", "c-close"),
		}
		for _, c := range checks {
			r.byCmd[c.Run] = outcome{res: shell.Result{Exit: 3}}
		}
		got := check.Run(check.Context{Event: event, Worktree: "/w"}, checks, check.Deps{Shell: r.run})
		if want := []string{"p/" + event}; !slices.Equal(ids(got), want) {
			t.Errorf("event %s: failures %v, want only %v", event, ids(got), want)
		}
		if len(r.cmds) != 1 || r.cmds[0].Command != "c-"+event {
			t.Errorf("event %s: ran %v, want only c-%s", event, r.cmds, event)
		}
	}
}

func TestRunFiltersOnTheSessionKind(t *testing.T) {
	// covers: R-CHK-003
	forChild, forRoot, both := runCheck("p/child", "open", "c"), runCheck("p/root", "open", "c"), runCheck("p/both", "open", "c")
	forChild.For, forRoot.For = "child", "root"
	r := &fakeRunner{t: t, byCmd: map[string]outcome{"c": {res: shell.Result{Exit: 1}}}}
	deps := check.Deps{Shell: r.run}
	for _, tc := range []struct {
		child bool
		want  []string
	}{{true, []string{"p/child", "p/both"}}, {false, []string{"p/root", "p/both"}}} {
		ctx := check.Context{Event: "open", Child: tc.child, Worktree: "/w"}
		got := ids(check.Run(ctx, []profile.Check{forChild, forRoot, both}, deps))
		if !slices.Equal(got, tc.want) {
			t.Errorf("Child=%v: failures %v, want %v", tc.child, got, tc.want)
		}
	}
}

func TestRunReturnsNothingWithoutChecks(t *testing.T) {
	// covers: R-CHK-008
	if got := check.Run(openCtx("/w"), nil, check.Deps{}); len(got) != 0 {
		t.Fatalf("failures %v, want none", got)
	}
}

func TestFailureCarriesIDMessagePointerAndDetail(t *testing.T) {
	// covers: R-CHK-008
	c := runCheck("local/needs-allow", "open", "c")
	c.Message = "Create the allow file."
	r := &fakeRunner{t: t, byCmd: map[string]outcome{"c": {res: shell.Result{Exit: 3, Output: []byte("no allow file\n")}}}}
	got := check.Run(openCtx("/w"), []profile.Check{c}, check.Deps{Shell: r.run})
	want := check.Failure{
		ID: "local/needs-allow", Message: "Create the allow file.",
		Pointer: rule.Pointer{Skill: "ness-entry", Section: "Communicate"}, Detail: "no allow file",
	}
	if len(got) != 1 || got[0] != want {
		t.Fatalf("failures %+v, want [%+v]", got, want)
	}
}

func TestRefusalRendersMessagePointerAndDetail(t *testing.T) {
	// covers: R-CHK-009
	fs := []check.Failure{
		{
			ID: "program/autonomy-recorded", Message: "Record the autonomy mode.",
			Pointer: rule.Pointer{Skill: "ness-program", Section: "Autonomy"}, Detail: "no heading Autonomy",
		},
		{
			ID: "local/script", Pointer: rule.Pointer{Skill: "ness-entry", Section: "Communicate"}, Detail: "exit 3",
		},
	}
	want := "ness open refused: Record the autonomy mode. (skill ness-program, section Autonomy)\n" +
		"  program/autonomy-recorded: no heading Autonomy\n" +
		"ness open refused: local/script failed (skill ness-entry, section Communicate)\n" +
		"  local/script: exit 3"
	if got := strings.TrimRight(check.Refusal("open", fs), "\n"); got != want {
		t.Fatalf("Refusal =\n%s\nwant\n%s", got, want)
	}
	if got := check.Refusal("close", nil); got != "" {
		t.Fatalf("Refusal(no failures) = %q, want empty", got)
	}
}

func withEvent(c profile.Check, event string) profile.Check {
	c.On = event
	return c
}

func TestRunFiltersOnTheRootsSize(t *testing.T) {
	// covers: R-CHK-015
	program, every := runCheck("p/program", "open", "c"), runCheck("p/any", "open", "c")
	program.Size = []string{"program"}
	r := &fakeRunner{t: t, byCmd: map[string]outcome{"c": {res: shell.Result{Exit: 1}}}}
	deps := check.Deps{Shell: r.run}
	for _, tc := range []struct {
		size string
		want []string
	}{{"program", []string{"p/program", "p/any"}}, {"full", []string{"p/any"}}, {"", []string{"p/any"}}} {
		ctx := check.Context{Event: "open", Child: true, Size: tc.size, Worktree: "/w"}
		got := ids(check.Run(ctx, []profile.Check{program, every}, deps))
		if !slices.Equal(got, tc.want) {
			t.Errorf("Size=%q: failures %v, want %v", tc.size, got, tc.want)
		}
	}
}
