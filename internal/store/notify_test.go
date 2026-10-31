package store_test

import (
	"errors"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestThreadsAndPostProgress(t *testing.T) {
	// The thread key → platform id mapping is per channel (R-NTF-040); a post's progress is
	// kept by key so a retry resumes at the first unsent part (R-NTF-007); a finished post
	// restarts, so a later identical post is a new one.
	// covers: R-NTF-040, R-NTF-007
	s := tree(t)
	if id, err := s.Thread("rel", "v1"); err != nil || id != "" {
		t.Fatalf("unknown thread = %q, %v", id, err)
	}
	if err := s.SetThread("rel", "v1", "1700.1"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetThread("other", "v1", "9"); err != nil {
		t.Fatal(err)
	}
	if id, _ := s.Thread("rel", "v1"); id != "1700.1" {
		t.Fatalf("Thread = %q", id)
	}
	sent, err := s.BeginPost("k1", "rel", 3, now, "me")
	if err != nil || sent != 0 {
		t.Fatalf("BeginPost = %d, %v", sent, err)
	}
	if err = s.MarkSent("k1", 1, now, store.ThreadRef{}); err != nil {
		t.Fatal(err)
	}
	if sent, err = s.BeginPost("k1", "rel", 3, now, "me"); err != nil || sent != 1 {
		t.Fatalf("BeginPost again = %d, %v", sent, err)
	}
	posts, err := s.Posts()
	if err != nil || len(posts) != 1 || posts[0].Sent != 1 || posts[0].Parts != 3 || posts[0].Channel != "rel" {
		t.Fatalf("Posts = %+v, %v", posts, err)
	}
	if err = s.MarkSent("k1", 3, now, store.ThreadRef{}); err != nil {
		t.Fatal(err)
	}
	if sent, err = s.BeginPost("k1", "rel", 3, now, "me"); err != nil || sent != 0 {
		t.Fatalf("a finished post restarts at 0: %d, %v", sent, err)
	}
	if posts, _ = s.Posts(); len(posts) != 1 || posts[0].Sent != 0 {
		t.Fatalf("restart is recorded: %+v", posts)
	}
}

func TestBeginPostClaimsTheUnclaimedAndRefusesALiveClaim(t *testing.T) {
	// Two runs of the same post never both send: a post claimed by a live run is refused, a claim
	// older than ClaimTTL is stale and taken over, and a released post can be claimed again.
	// covers: R-NTF-007
	s := tree(t)
	if _, err := s.BeginPost("k", "rel", 2, now, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BeginPost("k", "rel", 2, now.Add(time.Minute), "b"); !errors.Is(err, store.ErrPostBusy) {
		t.Fatalf("live claim: %v", err)
	}
	if _, err := s.BeginPost("k", "rel", 2, now.Add(store.ClaimTTL+time.Second), "b"); err != nil {
		t.Fatalf("stale claim is taken over: %v", err)
	}
	if err := s.ReleasePost("k", "a"); err != nil { // not the owner: no effect
		t.Fatal(err)
	}
	if _, err := s.BeginPost("k", "rel", 2, now.Add(store.ClaimTTL+time.Minute), "a"); !errors.Is(err, store.ErrPostBusy) {
		t.Fatalf("a release by a stranger must not free b's claim: %v", err)
	}
	if err := s.ReleasePost("k", "b"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.BeginPost("k", "rel", 2, now.Add(store.ClaimTTL+time.Minute), "a"); err != nil {
		t.Fatalf("released post: %v", err)
	}
}

func TestMarkSentRecordsTheThreadInTheSameTransaction(t *testing.T) {
	// The first part's sent mark and its thread id land together: a failure in between cannot
	// leave a part sent without its thread (R-NTF-040, R-NTF-007).
	// covers: R-NTF-040, R-NTF-007
	s := tree(t)
	if _, err := s.BeginPost("k", "rel", 2, now, "a"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.RawDB().ExecContext(t.Context(), `CREATE TRIGGER no_threads BEFORE INSERT ON threads
		BEGIN SELECT RAISE(ABORT, 'injected thread failure'); END`); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkSent("k", 1, now, store.ThreadRef{Channel: "rel", Key: "v1", ID: "1.1"}); err == nil {
		t.Fatal("want the injected failure")
	}
	if posts, _ := s.Posts(); len(posts) != 1 || posts[0].Sent != 0 {
		t.Fatalf("the sent mark must roll back with the thread: %+v", posts)
	}
	if _, err := s.RawDB().ExecContext(t.Context(), `DROP TRIGGER no_threads`); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkSent("k", 1, now, store.ThreadRef{Channel: "rel", Key: "v1", ID: "1.1"}); err != nil {
		t.Fatal(err)
	}
	if id, _ := s.Thread("rel", "v1"); id != "1.1" {
		t.Fatalf("thread = %q", id)
	}
}
