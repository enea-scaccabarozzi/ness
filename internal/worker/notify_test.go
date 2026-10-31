package worker_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/shell"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestNotifyNoneAndAutoPostStateChangesDirectly(t *testing.T) {
	// none posts nothing; auto on a floored probe posts each confirmed state change to the
	// channel with no agent involved; judged posts nothing on runs (R-PRB-050, R-PRB-051, R-PRB-052).
	// covers: R-PRB-050, R-PRB-051, R-PRB-052
	for _, tc := range []struct {
		notify string
		posts  int
	}{{"none", 0}, {"auto", 2}, {"judged", 0}} {
		b := newBench(t)
		b.session(sA, store.Operator, "claude")
		fr, posts := b.probes(1)
		file := "---\nevery: 10m\nfloor: ./f.sh\nwake: never\nnotify: " + tc.notify + "\nchannel: rel\n---\ns\n"
		b.start(sA, b.probeFile(sA, "p.md", file))
		for _, code := range []int{0, 0, 1, 1, 0} {
			fr.results = []shell.Result{exit(code)}
			b.pass()
			b.now = b.now.Add(10 * time.Minute)
		}
		if len(*posts) != tc.posts {
			t.Errorf("notify %s: %d posts %v, want %d", tc.notify, len(*posts), *posts, tc.posts)
		}
		if tc.notify == "auto" && (len(*posts) != 2 || !strings.HasPrefix((*posts)[0], "rel|probe:probes/p.md|probe probes/p.md: GREEN → RED") ||
			!strings.Contains((*posts)[1], "RED → GREEN")) {
			t.Errorf("auto posts = %v", *posts)
		}
		if len(b.listed(sA)) != 0 {
			t.Errorf("notify %s: a wake was created with wake: never", tc.notify)
		}
	}
}

func TestAutoRespectsConfirm(t *testing.T) {
	// confirm: 2 → RED, GREEN, RED, RED posts (and wakes) once, at the fourth run; the GREEN at
	// run 2 is the silent baseline (R-PRB-037, P-10).
	// covers: R-PRB-037, R-PRB-051
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	fr, posts := b.probes(1)
	b.start(sA, b.probeFile(sA, "p.md", "---\nevery: 10m\nfloor: ./f.sh\nconfirm: 2\nnotify: auto\nchannel: rel\n---\ns\n"))
	var wakesAt, postsAt []int
	for i, code := range []int{1, 0, 1, 1} {
		fr.results = []shell.Result{exit(code)}
		w, p := len(b.listed(sA)), len(*posts)
		b.pass()
		if len(b.listed(sA)) > w {
			wakesAt = append(wakesAt, i+1)
		}
		if len(*posts) > p {
			postsAt = append(postsAt, i+1)
		}
		b.now = b.now.Add(10 * time.Minute)
	}
	if len(wakesAt) != 1 || wakesAt[0] != 4 || len(postsAt) != 1 || postsAt[0] != 4 {
		t.Fatalf("wakes at %v, posts at %v; want one wake and one post, both at run 4", wakesAt, postsAt)
	}
}

func TestFloorlessAutoBehavesAsJudged(t *testing.T) {
	// covers: R-PRB-053
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, posts := b.probes(1)
	b.start(sA, b.probeFile(sA, "p.md", "---\nevery: 10m\nnotify: auto\nchannel: rel\n---\nlook\n"))
	b.passes(3, 10*time.Minute)
	if len(*posts) != 0 || len(b.listed(sA)) != 3 {
		t.Fatalf("floorless auto: %d posts, %d wakes", len(*posts), len(b.listed(sA)))
	}
}

func TestNotifyOffIsLoggedOnce(t *testing.T) {
	// covers: R-PRB-051
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	fr, _ := b.probes(1)
	b.w.Probes.Post = nil
	b.start(sA, b.probeFile(sA, "p.md", "---\nevery: 10m\nfloor: ./f.sh\nnotify: auto\nchannel: rel\n---\ns\n"))
	for _, code := range []int{0, 1, 0} {
		fr.results = []shell.Result{exit(code)}
		b.pass()
		b.now = b.now.Add(10 * time.Minute)
	}
	if n := b.logCount("notify is off"); n != 1 {
		t.Fatalf("notify-off logged %d times", n)
	}
}

func TestOnceRunsOnceWithinJitterThenStops(t *testing.T) {
	// covers: R-PRB-041
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	fr, _ := b.probes(3)
	path := b.probeFile(sA, "p.md", "---\nonce: true\njitter: 20m\nfloor: ./f.sh\nwake: always\n---\ns\n")
	first := b.now.Add(7 * time.Minute) // the CLI draws the first run within jitter; the bench picks it
	p, err := b.s.StartProbe(path, sA, "", false, first, b.now)
	if err != nil {
		t.Fatal(err)
	}
	b.passes(1, 5*time.Minute)
	if cur := b.probe(p.ID); cur.Runs != 0 {
		t.Fatal("ran before its time")
	}
	b.passes(1, 5*time.Minute)
	cur := b.probe(p.ID)
	if cur.Runs != 1 || cur.State != store.ProbeStopped || cur.Note != "once: ran" || len(b.listed(sA)) != 1 {
		t.Fatalf("after the once run: %+v, %d wakes", cur, len(b.listed(sA)))
	}
	b.passes(12, time.Hour)
	if cur = b.probe(p.ID); cur.Runs != 1 || len(fr.cmds) != 1 {
		t.Fatalf("a stopped once probe ran again: %+v", cur)
	}
}

func TestWindowEndsWithOneFinalWakeAndJudgedPostsOnlyVerdicts(t *testing.T) {
	// window: 48h → no floor runs after 48 h; exactly one final wake asking for a verdict; with
	// judged, runs post nothing (R-PRB-042, R-PRB-052).
	// covers: R-PRB-042, R-PRB-052
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	fr, posts := b.probes(1)
	p := b.start(sA, b.probeFile(sA, "p.md", "---\nevery: 6h\nwindow: 48h\nfloor: ./f.sh\nnotify: judged\nchannel: rel\n---\ns\n"))
	for range 10 { // 60 h in 6 h steps
		fr.results = []shell.Result{exit(1)}
		b.pass()
		b.now = b.now.Add(6 * time.Hour)
	}
	cur := b.probe(p.ID)
	if cur.Runs != 8 || cur.State != store.ProbeStopped || !cur.FinalWake || cur.Note != "window elapsed" {
		t.Fatalf("probe = %+v; want 8 runs (0..42 h), stopped, final wake", cur)
	}
	items := b.listed(sA)
	finals := 0
	for _, it := range items {
		if strings.Contains(it.Body, "final verdict requested") {
			finals++
		}
	}
	if finals != 1 || len(*posts) != 0 {
		t.Fatalf("%d final wakes, %d posts", finals, len(*posts))
	}
}

func TestWindowShorterThanEveryStillEnds(t *testing.T) {
	// Review Focus 5: window: 1h with every: 2h → the final wake at 1 h, once, no floor run.
	// covers: R-PRB-042
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	fr, _ := b.probes(1)
	path := b.probeFile(sA, "p.md", "---\nevery: 2h\nwindow: 1h\nfloor: ./f.sh\n---\ns\n")
	p, _ := b.s.StartProbe(path, sA, "", false, b.now.Add(2*time.Hour), b.now)
	if err := b.s.RecordRun(p.ID, store.ProbeRun{Hash: "", Runs: 0, At: b.now, Next: b.now.Add(time.Hour)}); err != nil {
		t.Fatal(err)
	}
	b.passes(3, 30*time.Minute)
	cur := b.probe(p.ID)
	if cur.State != store.ProbeStopped || !cur.FinalWake || len(fr.cmds) != 0 || len(b.listed(sA)) != 1 {
		t.Fatalf("probe = %+v, %d commands, %d wakes", cur, len(fr.cmds), len(b.listed(sA)))
	}
}

func TestKeepWindowKeepsTheOriginalStart(t *testing.T) {
	// A restart at 24 h with --keep-window stops at 48 h from the first start (R-PRB-062).
	// covers: R-PRB-062
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, _ = b.probes(1)
	path := b.probeFile(sA, "p.md", "---\nevery: 6h\nwindow: 48h\nfloor: ./f.sh\n---\ns\n")
	p := b.start(sA, path)
	b.passes(4, 6*time.Hour) // 24 h
	if _, err := b.s.StartProbe(path, sA, "", true, b.now, b.now); err != nil {
		t.Fatal(err)
	}
	b.passes(4, 6*time.Hour) // 48 h
	if cur := b.probe(p.ID); cur.State != store.ProbeStopped || !cur.FinalWake {
		t.Fatalf("kept window did not end at 48 h: %+v", cur)
	}
}

func TestEditsReloadTheSchedule(t *testing.T) {
	// covers: R-PRB-061
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, _ = b.probes(1)
	p := b.start(sA, b.probeFile(sA, "p.md", "---\nevery: 30m\nfloor: true\n---\ns\n"))
	b.pass()
	b.probeFile(sA, "p.md", "---\nevery: 5m\nfloor: true\n---\ns\n")
	b.passes(6, 5*time.Minute) // 30 min: the old schedule fires once, then the new one
	if cur := b.probe(p.ID); cur.NextRunAt.Sub(cur.LastRunAt) != 5*time.Minute || cur.Runs != 2 {
		t.Fatalf("after the edit: %+v", cur)
	}
	b.passes(2, 5*time.Minute)
	if cur := b.probe(p.ID); cur.Runs != 4 {
		t.Fatalf("new every not in force: %+v", cur)
	}
}

// repeatChannel is a channel that records the text of every part it is asked to send.
type repeatChannel struct{ sent []string }

func (*repeatChannel) Render(b notify.Block) string       { return b.Raw }
func (*repeatChannel) Measure(s string) int               { return len(s) }
func (*repeatChannel) Limit() int                         { return 0 }
func (*repeatChannel) MaxBlocks() int                     { return 0 }
func (*repeatChannel) Kind() string                       { return "fake" }
func (*repeatChannel) NeedsThread() bool                  { return true }
func (*repeatChannel) ThreadIsKey() bool                  { return true }
func (*repeatChannel) Payload(notify.Part, string) []byte { return nil }
func (c *repeatChannel) Post(p notify.Part, _ string) (string, error) {
	c.sent = append(c.sent, p.Text())
	return "id", nil
}

func TestRepeatedAlertIsPostedAgainThroughTheRealPoster(t *testing.T) {
	// RED, GREEN, RED, GREEN, RED with the same evidence sends the same "GREEN -> RED" text twice:
	// BeginPost restarts a completed post (P-18).
	// covers: R-PRB-051
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	fr, _ := b.probes(1)
	ch := &repeatChannel{}
	poster := notify.Poster{
		Channel: func(string) (notify.Channel, error) { return ch, nil }, Store: b.s, Now: func() time.Time { return b.now },
	}
	b.w.Probes.Post = func(channel, md, thread string) error {
		_, err := poster.Post(notify.Request{Channel: channel, Markdown: md, Thread: thread})
		return err
	}
	b.start(sA, b.probeFile(sA, "p.md", "---\nevery: 10m\nfloor: ./f.sh\nwake: never\nnotify: auto\nchannel: rel\n---\ns\n"))
	for _, code := range []int{1, 0, 1, 0, 1} {
		fr.results = []shell.Result{exit(code)}
		b.pass()
		b.now = b.now.Add(10 * time.Minute)
	}
	n := 0
	for _, s := range ch.sent {
		if strings.Contains(s, "GREEN \u2192 RED") {
			n++
		}
	}
	if len(ch.sent) != 5 || n != 2 {
		t.Fatalf("sent %d messages, %d of them GREEN to RED: %q", len(ch.sent), n, ch.sent)
	}
}

func TestFailedPostRetriesAtTheNextConfirmedRunAndLogsOnce(t *testing.T) {
	// covers: R-PRB-051
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	fr, posts := b.probes(1)
	fail := true
	b.w.Probes.Post = func(channel, md, thread string) error {
		if fail {
			return errors.New("channel down")
		}
		*posts = append(*posts, channel+"|"+thread+"|"+md)
		return nil
	}
	p := b.start(sA, b.probeFile(sA, "p.md", "---\nevery: 10m\nfloor: ./f.sh\nwake: never\nnotify: auto\nchannel: rel\n---\ns\n"))
	for _, code := range []int{0, 1, 1} {
		fr.results = []shell.Result{exit(code)}
		b.pass()
		b.now = b.now.Add(10 * time.Minute)
	}
	if n := b.logCount("channel down"); n != 1 || len(*posts) != 0 || b.probe(p.ID).NotifiedState != "GREEN" {
		t.Fatalf("%d log lines, %d posts, notified %q", n, len(*posts), b.probe(p.ID).NotifiedState)
	}
	fail = false
	fr.results = []shell.Result{exit(1)}
	b.pass()
	if len(*posts) != 1 || !strings.Contains((*posts)[0], "GREEN \u2192 RED") || b.probe(p.ID).NotifiedState != "RED" {
		t.Fatalf("posts %v, notified %q", *posts, b.probe(p.ID).NotifiedState)
	}
}

func TestFailedFinalWakeRetriesAndDoesNotStop(t *testing.T) {
	// covers: R-PRB-042
	b := newBench(t)
	b.session(sA, store.Operator, "claude")
	_, _ = b.probes(1)
	p := b.start(sA, b.probeFile(sA, "p.md", "---\nevery: 2h\nwindow: 1h\nfloor: ./f.sh\n---\ns\n"))
	b.sql(`CREATE TRIGGER deny BEFORE INSERT ON items BEGIN SELECT RAISE(ABORT, 'denied'); END`)
	b.passes(2, 30*time.Minute) // the window ends at 1 h
	if cur := b.probe(p.ID); cur.State != store.ProbeActive || cur.FinalWake {
		t.Fatalf("stopped with the wake failed: %+v", cur)
	}
	b.sql(`DROP TRIGGER deny`)
	b.passes(1, time.Minute)
	if cur := b.probe(p.ID); cur.State != store.ProbeStopped || !cur.FinalWake || len(b.listed(sA)) != 1 {
		t.Fatalf("after the retry: %+v, %d wakes", cur, len(b.listed(sA)))
	}
}
