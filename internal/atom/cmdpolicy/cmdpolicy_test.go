// Assumption: the plan pins Policy{Command, Dir, Timeout, Run} but not Runner's shape; Run is
// assumed to take a func with shell.Run's signature (shell.Runner or a local func type with the
// same underlying type, like command.Channel.Run), so the fake's method value fits it.
package cmdpolicy_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/cmdpolicy"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/shell"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

// fakeRunner records the one command it is given and answers with res and err.
type fakeRunner struct {
	res shell.Result
	err error
	got []shell.Cmd
}

func (f *fakeRunner) run(c shell.Cmd) (shell.Result, error) {
	f.got = append(f.got, c)
	return f.res, f.err
}

const cmd = "./select.sh --fast"

func policy(f *fakeRunner) cmdpolicy.Policy {
	return cmdpolicy.Policy{Command: cmd, Dir: "/repo", Timeout: 7 * time.Second, Run: f.run}
}

func TestContract(t *testing.T) {
	// Rejects: a command selector that does not hand back the command's confidence and reason.
	f := &fakeRunner{res: shell.Result{Output: []byte(`{"offset":-1,"confidence":0.9,"reason":"edit to spec"}` + "\n")}}
	testkit.PolicyContract(t, policy(f))
}

func TestSuccessPassesBriefAndReadsVerdict(t *testing.T) {
	// covers: R-MOD-045
	// Rejects: ignoring Dir/Timeout, a stdin missing a protocol key, and a verdict not taken
	// verbatim from the command's JSON.
	f := &fakeRunner{res: shell.Result{Output: []byte(`{"offset":-1,"confidence":0.9,"reason":"edit to spec"}` + "\n")}}
	v, err := policy(f).Choose(session.Brief{
		Text: "Fix the parser.", Role: "subagent", Client: "claude", Kind: "implement", AgentType: "ness-sweep",
	})
	if err != nil {
		t.Fatal(err)
	}
	if v != (session.Verdict{Offset: -1, Confidence: 0.9, Reason: "edit to spec"}) {
		t.Fatalf("Verdict = %+v", v)
	}
	if len(f.got) != 1 {
		t.Fatalf("ran %d commands, want 1", len(f.got))
	}
	c := f.got[0]
	if c.Command != cmd || c.Dir != "/repo" || c.Timeout != 7*time.Second {
		t.Fatalf("Cmd = %+v", c)
	}
	var in map[string]any
	if err := json.Unmarshal([]byte(c.Stdin), &in); err != nil {
		t.Fatalf("stdin is not JSON: %q", c.Stdin)
	}
	want := map[string]any{
		"text": "Fix the parser.", "role": "subagent", "client": "claude", "kind": "implement", "agent_type": "ness-sweep",
	}
	for k, w := range want {
		if in[k] != w {
			t.Errorf("stdin[%q] = %v, want %q", k, in[k], w)
		}
	}
}

func TestConfidenceBoundsInclusive(t *testing.T) {
	// covers: R-MOD-045
	// Rejects: treating 0..1 as an open interval.
	for _, out := range []string{
		`{"offset":0,"confidence":0,"reason":"unsure"}`,
		`{"offset":2,"confidence":1,"reason":"sure"}`,
	} {
		if _, err := policy(&fakeRunner{res: shell.Result{Output: []byte(out)}}).Choose(session.Brief{Kind: "design"}); err != nil {
			t.Errorf("%s: %v", out, err)
		}
	}
}

func TestFailuresNameTheCommand(t *testing.T) {
	// covers: R-MOD-045
	// Rejects: parsing stdout despite a non-zero exit or a timeout, accepting a confidence
	// outside 0..1 or a multi-line reason, swallowing a start failure, and an error that does not
	// name the command (Pick turns each of these into base).
	ok := []byte(`{"offset":1,"confidence":0.9,"reason":"fine"}`)
	cases := []struct {
		name string
		f    fakeRunner
	}{
		{"non-zero exit", fakeRunner{res: shell.Result{Output: ok, Exit: 3}}},
		{"timeout", fakeRunner{res: shell.Result{Output: ok, Exit: 137, TimedOut: true}}},
		{"not json", fakeRunner{res: shell.Result{Output: []byte("offset: 1\n")}}},
		{"empty output", fakeRunner{}},
		{"confidence above 1", fakeRunner{res: shell.Result{Output: []byte(`{"offset":1,"confidence":1.5,"reason":"x"}`)}}},
		{"confidence below 0", fakeRunner{res: shell.Result{Output: []byte(`{"offset":1,"confidence":-0.1,"reason":"x"}`)}}},
		{"multi-line reason", fakeRunner{res: shell.Result{Output: []byte(`{"offset":1,"confidence":0.9,"reason":"a\nb"}`)}}},
		{"could not start", fakeRunner{err: errors.New("fork failed")}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := tc.f
			_, err := policy(&f).Choose(session.Brief{Kind: "implement"})
			if err == nil || !strings.Contains(err.Error(), cmd) {
				t.Fatalf("err = %v, want an error naming %q", err, cmd)
			}
		})
	}
}
