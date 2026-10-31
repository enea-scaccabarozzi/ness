package session_test

import (
	"errors"
	"strings"
	"testing"
	"testing/synctest"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
	"github.com/enea-scaccabarozzi/ness/internal/atom/herdr"
	"github.com/enea-scaccabarozzi/ness/internal/session"
)

func TestDroppedEnterIsPressedAgainAndThePromptSubmittedOnce(t *testing.T) {
	// The client drops the prompt's Enter (the text sits in its box, the prompt call stalls): the
	// launch presses Enter in the pane once, the one submission carries exactly the prompt text.
	// covers: R-SESS-150, R-SESS-025
	b := newBench(t)
	b.register(t, sA, "root")
	b.l.Read = claude.Read
	setHerdr(t, b.fakes, "drop_enter", "1")
	if err := b.run("b"); err != nil {
		t.Fatalf("launch: %v", err)
	}
	sess, st := b.session(t), b.state(t)
	want := session.Prompt(sA, "root", "docs/sessions/root")
	if len(st.Keys) != 1 || st.Keys[0] != [2]string{sess.Pane, "Enter"} {
		t.Fatalf("keys = %q, want one Enter in %s", st.Keys, sess.Pane)
	}
	if len(st.Prompted) != 1 || st.Prompted[0] != [2]string{sess.Pane, want} {
		t.Fatalf("prompted = %q, want one submission of %q", st.Prompted, want)
	}
	if sess.Phase != "ready" || len(st.Boxes) != 0 {
		t.Fatalf("phase %q, boxes %q", sess.Phase, st.Boxes)
	}
}

func TestEntersDroppedPastTheAttemptsFailTheLaunchAndResumeSubmits(t *testing.T) {
	// The client drops the prompt's Enter and all three pressed ones: the launch fails naming the
	// pane, nothing is submitted, the phase stays short of ready; once Enters are taken, --resume
	// submits the prompt. The 3x5 s wait runs on synctest's clock.
	// covers: R-SESS-150
	synctest.Test(t, func(t *testing.T) {
		b := newBench(t)
		b.register(t, sA, "root")
		b.l.Read = claude.Read
		b.l.ReadyTimeout = time.Minute // the deadline of the whole prompt step: 3 Enters x 5 s must fit (production default)
		setHerdr(t, b.fakes, "drop_enter", "4")
		err := b.run("b")
		sess, st := b.session(t), b.state(t)
		if !errors.Is(err, session.ErrNotSubmitted) || !strings.Contains(err.Error(), sess.Pane) {
			t.Fatalf("launch error %v, want ErrNotSubmitted naming pane %q", err, sess.Pane)
		}
		if len(st.Keys) != 3 {
			t.Fatalf("keys = %q, want exactly 3 Enters", st.Keys)
		}
		for _, k := range st.Keys {
			if k != [2]string{sess.Pane, "Enter"} {
				t.Fatalf("key %q, want Enter in %s", k, sess.Pane)
			}
		}
		if len(st.Prompted) != 0 || sess.Phase == "ready" {
			t.Fatalf("prompted %q, phase %q: want none and short of ready", st.Prompted, sess.Phase)
		}

		setHerdr(t, b.fakes, "drop_enter", "0")
		if err := b.run(""); err != nil {
			t.Fatalf("resume: %v", err)
		}
		st = b.state(t)
		want := session.Prompt(sA, "root", "docs/sessions/root")
		// The prompt is still in the box: the resume presses Enter (a 4th) instead of typing it again.
		if len(st.Prompted) != 1 || st.Prompted[0] != [2]string{sess.Pane, want} || len(st.Keys) != 4 {
			t.Fatalf("after resume: prompted %q (want exactly %q once), keys %q", st.Prompted, want, st.Keys)
		}
		if after := b.session(t); after.Phase != "ready" {
			t.Fatalf("phase after resume %q", after.Phase)
		}
	})
}

// stallThenBlock lets the prompt call run, then puts the agent on a dialog, as a trust prompt that
// came up while the client ate the Enter.
func stallThenBlock(t *testing.T, b *bench, inner herdr.Exec) herdr.Exec {
	t.Helper()
	return func(dir, name string, args ...string) ([]byte, error) {
		out, err := inner(dir, name, args...)
		if len(args) > 1 && args[0] == "agent" && args[1] == "prompt" && err != nil {
			setHerdr(t, b.fakes, "status", b.session(t).Agent+",blocked")
		}
		return out, err
	}
}

func TestNoEnterIsPressedOnABlockedPane(t *testing.T) {
	// The prompt stalls and the agent is blocked on a dialog by then (Enter would answer the trust
	// prompt's default, "No, exit"): no key reaches the pane, the launch stalls at its ready timeout
	// and nothing is submitted.
	// covers: R-SESS-150, R-SESS-025
	synctest.Test(t, func(t *testing.T) {
		b := newBench(t)
		b.register(t, sA, "root")
		b.l.Read = claude.Read
		b.l.Runtime = herdr.Runtime{Exec: stallThenBlock(t, b, herdr.Exec(b.l.Exec))}
		setHerdr(t, b.fakes, "drop_enter", "1")
		if err := b.run("b"); !errors.Is(err, session.ErrStalled) {
			t.Fatalf("launch: %v, want ErrStalled", err)
		}
		st := b.state(t)
		if len(st.Keys) != 0 || len(st.Prompted) != 0 || b.session(t).Phase == "ready" {
			t.Fatalf("keys %q prompted %q phase %q", st.Keys, st.Prompted, b.session(t).Phase)
		}
	})
}

// clearBoxAfterStall counts the prompt calls and, after a stalled one, empties the client's box, as
// when the client draws the text only seconds after herdr's stall verdict.
func clearBoxAfterStall(t *testing.T, b *bench, inner herdr.Exec, prompts *int) herdr.Exec {
	t.Helper()
	return func(dir, name string, args ...string) ([]byte, error) {
		out, err := inner(dir, name, args...)
		if len(args) > 1 && args[0] == "agent" && args[1] == "prompt" {
			*prompts++
			if err != nil {
				setHerdr(t, b.fakes, "box", args[2]+",")
			}
		}
		return out, err
	}
}

func TestPromptIsTypedAtMostOncePerRun(t *testing.T) {
	// The prompt call stalls and the text is not (yet) visible in the box: the launch must not type it
	// again (the text would be submitted doubled), nor press Enter on an empty box. It stalls at its
	// ready timeout; a resume is a new run and types it once more.
	// covers: R-SESS-150
	synctest.Test(t, func(t *testing.T) {
		b := newBench(t)
		b.register(t, sA, "root")
		b.l.Read = claude.Read
		prompts := 0
		b.l.Runtime = herdr.Runtime{Exec: clearBoxAfterStall(t, b, herdr.Exec(b.l.Exec), &prompts)}
		setHerdr(t, b.fakes, "drop_enter", "1")
		if err := b.run("b"); !errors.Is(err, session.ErrStalled) {
			t.Fatalf("launch: %v, want ErrStalled", err)
		}
		st := b.state(t)
		if prompts != 1 || len(st.Keys) != 0 || len(st.Prompted) != 0 {
			t.Fatalf("prompt calls %d, keys %q, prompted %q: want 1, none, none", prompts, st.Keys, st.Prompted)
		}
		if err := b.run(""); err != nil {
			t.Fatalf("resume: %v", err)
		}
		want := session.Prompt(sA, "root", "docs/sessions/root")
		if st = b.state(t); prompts != 2 || len(st.Prompted) != 1 || st.Prompted[0][1] != want || b.session(t).Phase != "ready" {
			t.Fatalf("after resume: prompt calls %d, prompted %q, phase %q", prompts, st.Prompted, b.session(t).Phase)
		}
	})
}
