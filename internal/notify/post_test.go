package notify_test

import (
	"bytes"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// fakeChannel records posts; failAt makes that part (1-based) fail once.
type fakeChannel struct {
	plain
	kind   string
	posts  []string // "<thread>|<text>"
	failAt int
	next   int
	thread bool
	isKey  bool
}

func (f *fakeChannel) Kind() string {
	if f.kind != "" {
		return f.kind
	}
	return "fake"
}
func (f *fakeChannel) NeedsThread() bool { return f.thread }
func (f *fakeChannel) ThreadIsKey() bool { return f.isKey }
func (f *fakeChannel) Payload(p notify.Part, thread string) []byte {
	return []byte("[" + thread + "]" + p.Text())
}

func (f *fakeChannel) Post(p notify.Part, thread string) (string, error) {
	if len(f.posts)+1 == f.failAt {
		f.failAt = 0
		return "", errors.New("boom")
	}
	f.posts = append(f.posts, thread+"|"+p.Text())
	f.next++
	return "id" + strings.Repeat("0", f.next), nil
}

func poster(t *testing.T, ch notify.Channel) (notify.Poster, *store.Store) {
	t.Helper()
	s, err := store.Open(filepath.Join(t.TempDir(), "state.sqlite3"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return notify.Poster{
		Channel: func(name string) (notify.Channel, error) {
			if name != "rel" {
				return nil, notify.ErrNoChannel
			}
			return ch, nil
		},
		Store: s, Now: func() time.Time { return time.Date(2026, 10, 9, 10, 0, 0, 0, time.UTC) },
	}, s
}

func TestPostThreadsByKeyAndRecordsTheMapping(t *testing.T) {
	// The first post with a new key starts the thread and records the platform id under the
	// channel's name; the second replies in it (R-NTF-005, R-NTF-040).
	// covers: R-NTF-005, R-NTF-040
	ch := &fakeChannel{plain: plain{limit: 100}}
	p, s := poster(t, ch)
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: "one", Thread: "v1"}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: "two", Thread: "v1"}); err != nil {
		t.Fatal(err)
	}
	if len(ch.posts) != 2 || ch.posts[0] != "|one" || ch.posts[1] != "id0|two" {
		t.Fatalf("posts = %v", ch.posts)
	}
	if id, _ := s.Thread("rel", "v1"); id != "id0" {
		t.Fatalf("recorded thread = %q", id)
	}
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: "three"}); err != nil || ch.posts[2] != "|three" {
		t.Fatalf("unthreaded post: %v %v", err, ch.posts)
	}
}

func TestPostResumesAtTheFirstUnsentPart(t *testing.T) {
	// A multi-part post records each sent part; a failure names the parts sent; the retry
	// sends only the rest and never reposts a sent part (R-NTF-007). Once finished, the same
	// text posted again is a new post and is sent again.
	// covers: R-NTF-007
	ch := &fakeChannel{plain: plain{limit: 5}, failAt: 2}
	p, _ := poster(t, ch)
	md := "aaaa\n\nbbbb\n\ncccc"
	_, err := p.Post(notify.Request{Channel: "rel", Markdown: md})
	var pe *notify.PartialError
	if !errors.As(err, &pe) || pe.Sent != 1 || pe.Parts != 3 || !strings.Contains(err.Error(), "part 1 was sent") {
		t.Fatalf("first attempt: %v", err)
	}
	if _, err = p.Post(notify.Request{Channel: "rel", Markdown: md}); err != nil {
		t.Fatal(err)
	}
	if len(ch.posts) != 3 || ch.posts[0] != "|aaaa" || ch.posts[1] != "|bbbb" || ch.posts[2] != "|cccc" {
		t.Fatalf("posts = %v; want each part once", ch.posts)
	}
	if _, err = p.Post(notify.Request{Channel: "rel", Markdown: md}); err != nil || len(ch.posts) != 6 {
		t.Fatalf("an identical post after success is sent again: %v, %d posts", err, len(ch.posts))
	}
}

func TestADifferentNoteDoesNotShareProgress(t *testing.T) {
	// The resume key covers the parts: a different note to the same channel and thread after a
	// failure is a new post sent from its first part, and the failed note still resumes.
	// covers: R-NTF-007
	ch := &fakeChannel{plain: plain{limit: 5}, failAt: 2}
	p, _ := poster(t, ch)
	a, b := "aaaa\n\nbbbb\n\ncccc", "xxxx\n\nyyyy\n\nzzzz"
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: a}); err == nil {
		t.Fatal("the first attempt should fail at part 2")
	}
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: b}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: a}); err != nil {
		t.Fatal(err)
	}
	want := "|aaaa |xxxx |yyyy |zzzz |bbbb |cccc"
	if got := strings.Join(ch.posts, " "); got != want {
		t.Fatalf("posts = %q; want %q", got, want)
	}
}

func TestPostDryRunVerbatimAndRefusals(t *testing.T) {
	// --dry-run prints each part's payload and posts nothing (R-NTF-003); a verbatim note is one
	// part as is (R-NTF-004); an over-long block fails before anything is sent (R-NTF-012); an
	// unknown channel, an empty note and a thread-needing channel without a thread refuse.
	// covers: R-NTF-003, R-NTF-004, R-NTF-012, R-NTF-002
	ch := &fakeChannel{plain: plain{limit: 5}}
	p, _ := poster(t, ch)
	var dry bytes.Buffer
	req := notify.Request{Channel: "rel", Markdown: "aaaa\n\nbbbb", Thread: "t", DryRun: &dry}
	if _, err := p.Post(req); err != nil {
		t.Fatal(err)
	}
	if dry.String() != "[]aaaa\n[]bbbb\n" || len(ch.posts) != 0 {
		t.Fatalf("dry run printed %q, posts %v", dry.String(), ch.posts)
	}
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: "x", Verbatim: "**raw** stays"}); err != nil ||
		ch.posts[0] != "|**raw** stays" {
		t.Fatalf("verbatim: %v %v", err, ch.posts)
	}
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: "toolongblock"}); !errors.Is(err, notify.ErrBlockTooLong) ||
		len(ch.posts) != 1 {
		t.Fatalf("over-long: %v, %d posts", err, len(ch.posts))
	}
	if _, err := p.Post(notify.Request{Channel: "nope", Markdown: "x"}); !errors.Is(err, notify.ErrNoChannel) {
		t.Fatalf("unknown channel: %v", err)
	}
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: "\n\n"}); !errors.Is(err, notify.ErrEmpty) {
		t.Fatalf("empty note: %v", err)
	}
	ch.thread = true
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: "x"}); err == nil || !strings.Contains(err.Error(), "--thread") {
		t.Fatalf("thread needed: %v", err)
	}
}

func TestPostPassesTheKeyToChannelsWithoutPlatformThreads(t *testing.T) {
	// A channel whose thread is the key itself (github, command) gets the key as the thread and
	// nothing is recorded for it (R-NTF-029, R-NTF-028); a github key must be a number.
	// covers: R-NTF-029, R-NTF-028
	ch := &fakeChannel{plain: plain{limit: 100}, isKey: true}
	p, s := poster(t, ch)
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: "x", Thread: "k9"}); err != nil {
		t.Fatal(err)
	}
	if len(ch.posts) != 1 || ch.posts[0] != "k9|x" {
		t.Fatalf("posts = %v", ch.posts)
	}
	if id, _ := s.Thread("rel", "k9"); id != "" {
		t.Fatalf("a key-only channel records no thread, got %q", id)
	}
	ch.kind, ch.thread = "github", true
	var dry bytes.Buffer
	_, err := p.Post(notify.Request{Channel: "rel", Markdown: "x", Thread: "probe:a", DryRun: &dry})
	if err == nil || !strings.Contains(err.Error(), "number") || dry.Len() != 0 {
		t.Fatalf("non-numeric github thread: %v, printed %q", err, dry.String())
	}
	if _, err = p.Post(notify.Request{Channel: "rel", Markdown: "x", Thread: "42"}); err != nil {
		t.Fatal(err)
	}
}

// gateChannel blocks its first Post until release is closed, signalling entered first.
type gateChannel struct {
	fakeChannel
	entered chan struct{}
	release chan struct{}
}

func (g *gateChannel) Post(p notify.Part, thread string) (string, error) {
	g.entered <- struct{}{}
	<-g.release
	return g.fakeChannel.Post(p, thread)
}

func TestConcurrentIdenticalPostsSendOnce(t *testing.T) {
	// A second run of a post that is being sent is refused and sends nothing; once the first
	// finishes, the claim is released and the same post can be sent again (R-NTF-007).
	// covers: R-NTF-007
	g := &gateChannel{fakeChannel: fakeChannel{plain: plain{limit: 100}},
		entered: make(chan struct{}, 2), release: make(chan struct{})}
	p1, _ := poster(t, g)
	path := filepath.Join(t.TempDir(), "state.sqlite3")
	s1, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s1.Close() })
	s2, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s2.Close() })
	p1.Store = s1
	p2 := p1
	p2.Store = s2
	done := make(chan error, 1)
	go func() { _, perr := p1.Post(notify.Request{Channel: "rel", Markdown: "one"}); done <- perr }()
	<-g.entered
	if _, err = p2.Post(notify.Request{Channel: "rel", Markdown: "one"}); !errors.Is(err, store.ErrPostBusy) ||
		!strings.Contains(err.Error(), "is being sent by another run") {
		t.Fatalf("second run: %v", err)
	}
	close(g.release)
	if err = <-done; err != nil {
		t.Fatal(err)
	}
	if len(g.posts) != 1 {
		t.Fatalf("posts = %v; want exactly one send", g.posts)
	}
	if _, err = p2.Post(notify.Request{Channel: "rel", Markdown: "one"}); err != nil || len(g.posts) != 2 {
		t.Fatalf("after completion the claim is free: %v, %v", err, g.posts)
	}
}

func TestAFailedPostReleasesItsClaim(t *testing.T) {
	// A failure releases the claim, so the retry resumes at once instead of waiting out the TTL.
	// covers: R-NTF-007
	ch := &fakeChannel{plain: plain{limit: 100}, failAt: 1}
	p, _ := poster(t, ch)
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: "x"}); err == nil {
		t.Fatal("want the failure")
	}
	if _, err := p.Post(notify.Request{Channel: "rel", Markdown: "x"}); err != nil || len(ch.posts) != 1 {
		t.Fatalf("retry: %v %v", err, ch.posts)
	}
}
