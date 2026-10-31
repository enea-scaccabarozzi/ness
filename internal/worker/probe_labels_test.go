package worker_test

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/shell"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// startLabel registers a probe with a label at the bench clock's time at, first run at once.
func (b *bench) startLabel(id, path, label string, at time.Time) store.Probe {
	b.t.Helper()
	p, err := b.s.StartProbe(path, id, label, false, at, at)
	if err != nil {
		b.t.Fatal(err)
	}
	return p
}

// labelsRun is the NESS_PROBE_LABEL of each floor run, in order.
func labelsRun(fr *fakeRunner) []string {
	var out []string
	for _, c := range fr.cmds {
		for _, e := range c.Env {
			if v, ok := strings.CutPrefix(e, "NESS_PROBE_LABEL="); ok {
				out = append(out, v)
			}
		}
	}
	return out
}

func bodiesWith(items []store.Item, sub string) int {
	n := 0
	for _, it := range items {
		if strings.Contains(it.Body, sub) {
			n++
		}
	}
	return n
}

func TestOverlappingWindowsEndIndependently(t *testing.T) {
	// One file, two releases: A's 2h window from t0, B's from t0+1h. A gets exactly one final wake
	// and stops at t0+2h while B keeps running; B ends the same way an hour later; each floor sees
	// its own label and each wake names it (R-PRB-042, R-PRB-085, R-PRB-087).
	// covers: R-PRB-042, R-PRB-085, R-PRB-087
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	fr, _ := b.probes(1)
	path := b.probeFile(sA, "p.md", "---\nevery: 30m\nwindow: 2h\nfloor: ./f.sh\nwake: always\n---\ns\n")
	t0 := b.now
	pa := b.startLabel(sA, path, "a", t0)
	var pb store.Probe
	for i := range 8 { // t0 .. t0+3h30 in 30 min steps
		if i == 2 {
			pb = b.startLabel(sA, path, "b", b.now)
		}
		b.pass()
		switch i {
		case 4: // t0+2h: A's window is over, B has an hour left
			if a, c := b.probe(pa.ID), b.probe(pb.ID); a.State != store.ProbeStopped || !a.FinalWake ||
				c.State != store.ProbeActive || c.FinalWake {
				t.Fatalf("at t0+2h: a = %+v, b = %+v", a, c)
			}
		case 5:
			if c := b.probe(pb.ID); c.State != store.ProbeActive {
				t.Fatalf("b stopped early: %+v", c)
			}
		}
		b.now = b.now.Add(30 * time.Minute)
	}
	if a, c := b.probe(pa.ID), b.probe(pb.ID); a.Runs != 4 || c.Runs != 4 || c.State != store.ProbeStopped || !c.FinalWake {
		t.Fatalf("after both windows: a = %+v, b = %+v; want 4 runs each, both stopped with a final wake", a, c)
	}
	items := b.listed(sA)
	if bodiesWith(items, "probe "+path+" [a]: window elapsed") != 1 || bodiesWith(items, "probe "+path+" [b]: window elapsed") != 1 {
		t.Fatalf("final wakes: %d for a, %d for b", bodiesWith(items, "probe "+path+" [a]: window elapsed"),
			bodiesWith(items, "probe "+path+" [b]: window elapsed"))
	}
	if bodiesWith(items, "probe "+path+" [a] run 1") != 1 || bodiesWith(items, "probe "+path+" [b] run 1") != 1 {
		t.Fatalf("run wakes do not name their label: %+v", items)
	}
	want := []string{"a", "a", "a", "a", "b", "b", "b", "b"}
	if got := labelsRun(fr); !slices.Equal(slices.Sorted(slices.Values(got)), want) {
		t.Fatalf("NESS_PROBE_LABEL per floor run = %v, want %v", got, want)
	}
}

func TestUnlabelledProbeKeepsTheOldTexts(t *testing.T) {
	// No label: empty NESS_PROBE_LABEL, no brackets in the wake, the old thread (R-PRB-087).
	// covers: R-PRB-087
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	fr, posts := b.probes(1)
	path := b.probeFile(sA, "p.md", "---\nevery: 10m\nfloor: ./f.sh\nwake: always\nnotify: auto\nchannel: rel\n---\ns\n")
	b.start(sA, path)
	fr.results = []shell.Result{exit(1)}
	b.pass()
	if got := labelsRun(fr); !slices.Equal(got, []string{""}) {
		t.Fatalf("labels = %q; want one empty", got)
	}
	items := b.listed(sA)
	if len(items) != 1 || !strings.HasPrefix(items[0].Body, "probe "+path+" run 1: RED\n") {
		t.Fatalf("wake = %+v", items)
	}
	if len(*posts) != 1 || !strings.HasPrefix((*posts)[0], "rel|probe:"+path+"|probe "+path+": - → RED") {
		t.Fatalf("posts = %q", *posts)
	}
}

func TestAutoPostIsPerLabelAndPerThread(t *testing.T) {
	// Two labelled probes on one file post to their own threads with the label in the header; a
	// repeat RED in A does not repost and B's baseline is its own (R-PRB-037, R-PRB-087).
	// covers: R-PRB-037, R-PRB-087
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, posts := b.probes(1)
	path := b.probeFile(sA, "p.md", "---\nevery: 10m\nfloor: ./f.sh\nwake: red\nnotify: auto\nchannel: rel\n---\ns\n")
	b.startLabel(sA, path, "a", b.now)
	b.startLabel(sA, path, "b", b.now)
	// Floors run together, so the exit code is chosen by the floor's label, not by call order.
	codes := map[string]int{"a": 1, "b": 1}
	b.w.Probes.Run = func(c shell.Cmd) (shell.Result, error) {
		for _, e := range c.Env {
			if v, ok := strings.CutPrefix(e, "NESS_PROBE_LABEL="); ok {
				return exit(codes[v]), nil
			}
		}
		return exit(0), nil
	}
	b.pass()
	if len(*posts) != 2 || !hasPost(*posts, "rel|probe:"+path+":a|probe "+path+" [a]: - → RED") ||
		!hasPost(*posts, "rel|probe:"+path+":b|probe "+path+" [b]: - → RED") {
		t.Fatalf("posts = %q", *posts)
	}
	b.now = b.now.Add(10 * time.Minute)
	b.pass()
	if len(*posts) != 2 {
		t.Fatalf("a repeat RED reposted: %q", *posts)
	}
	b.now = b.now.Add(10 * time.Minute)
	codes["a"] = 0 // a recovers, b stays RED
	b.pass()
	if len(*posts) != 3 || !hasPost(*posts, "rel|probe:"+path+":a|probe "+path+" [a]: RED → GREEN") {
		t.Fatalf("posts = %q; want a third post, a's recovery in a's thread", *posts)
	}
}

// hasPost reports whether one of the posts starts with prefix.
func hasPost(posts []string, prefix string) bool {
	return slices.ContainsFunc(posts, func(p string) bool { return strings.HasPrefix(p, prefix) })
}
