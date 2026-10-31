package probe

import (
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/shell"
)

// Floor states (R-PRB-020..023).
const (
	Green   = "GREEN"
	Red     = "RED"
	Unclear = "UNCLEAR"
	Vacuous = "VACUOUS"
)

// EvidenceCap is how much of the floor's output is kept (R-PRB-039).
const EvidenceCap = 4096

// Run is one floor run: its state, the capped and redacted evidence, and how it ended.
type Run struct {
	State    string
	Evidence string
	Exit     int
	TimedOut bool
}

// Floor is where a floor runs: the owner's worktree, the owner id, the probe's label ("" for
// none) and the environment variables whose values are redacted from the evidence
// ([probes] redact_env).
type Floor struct {
	Dir, Owner, Label string
	Redact            []string
	Getenv            func(string) string
}

// RunFloor runs p's floor as sh -c in f.Dir with NESS_PROBE, NESS_SESSION and NESS_PROBE_LABEL
// (R-PRB-083, R-PRB-087),
// killed at p.Timeout (R-PRB-022). The error is only "could not start".
func RunFloor(run shell.Runner, p Probe, f Floor) (Run, error) {
	r, err := run(shell.Cmd{
		Command: p.Floor, Dir: f.Dir, Timeout: p.Timeout, KeepTail: true, // the evidence is the tail
		Env: []string{"NESS_PROBE=" + p.Path, "NESS_SESSION=" + f.Owner, "NESS_PROBE_LABEL=" + f.Label},
	})
	if err != nil {
		return Run{}, err
	}
	return Run{State: State(r, p.Expect), Evidence: Evidence(r.Output, f), Exit: r.Exit, TimedOut: r.TimedOut}, nil
}

// State classifies a floor result: timeout or an exit other than 0 and 1 is UNCLEAR, 1 is RED,
// 0 is GREEN unless expect is set and the last line's pairs do not satisfy it (VACUOUS).
func State(r shell.Result, expect []Cond) string {
	switch {
	case r.TimedOut, r.Exit != 0 && r.Exit != 1:
		return Unclear
	case r.Exit == 1:
		return Red
	case len(expect) > 0 && !Satisfied(expect, ParseKV(lastLine(r.Output))):
		return Vacuous
	}
	return Green
}

func lastLine(out []byte) string {
	lines := strings.Split(strings.TrimRight(string(out), "\n"), "\n")
	return lines[len(lines)-1]
}

// Evidence redacts the output, then keeps its last EvidenceCap bytes at a rune boundary
// (R-PRB-039): a secret can never survive as a fragment at the cut.
func Evidence(output []byte, f Floor) string {
	secrets := make([]string, 0, len(f.Redact))
	for _, name := range f.Redact {
		if v := f.Getenv(name); v != "" {
			secrets = append(secrets, v)
		}
	}
	text := Redact(string(output), secrets)
	if len(text) > EvidenceCap {
		text = text[len(text)-EvidenceCap:]
		for text != "" && !utf8.RuneStart(text[0]) {
			text = text[1:]
		}
	}
	return text
}

// Redact replaces every occurrence of each secret with [redacted], longest first and in one
// pass, so overlapping secrets leave no fragment and a replacement is never matched again.
func Redact(text string, secrets []string) string {
	sorted := slices.SortedFunc(slices.Values(secrets), func(a, b string) int { return len(b) - len(a) })
	pairs := make([]string, 0, 2*len(sorted))
	for _, s := range sorted {
		pairs = append(pairs, s, "[redacted]")
	}
	return strings.NewReplacer(pairs...).Replace(text)
}

// Name is how a probe reads in a wake or a post: its path, then its label in brackets when it
// has one (R-PRB-087).
func Name(path, label string) string {
	if label == "" {
		return path
	}
	return path + " [" + label + "]"
}

// Thread is the channel thread key of a probe: one thread per path, and per label when it has
// one, so an unlabelled probe keeps the thread it always had (R-PRB-087).
func Thread(path, label string) string {
	if label == "" {
		return "probe:" + path
	}
	return "probe:" + path + ":" + label
}

// VerdictNote is what a judged verdict posts (R-PRB-052, R-PRB-064).
func VerdictNote(pf Probe, label, state, note string) string {
	return fmt.Sprintf("probe %s verdict: %s\n\n%s\n", Name(pf.Path, label), strings.ToUpper(state), note)
}

// FirstRun is when a started probe first runs: now for every, within jitter for once
// (R-PRB-041), never past the window's end (R-PRB-042).
func FirstRun(pf Probe, now time.Time, r *rand.Rand) time.Time {
	first := now
	if pf.Once && pf.Jitter > 0 {
		first = now.Add(time.Duration(r.Int64N(int64(pf.Jitter))))
	}
	if pf.Window > 0 && first.After(now.Add(pf.Window)) {
		first = now.Add(pf.Window)
	}
	return first
}
