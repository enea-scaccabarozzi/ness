package store_test

import (
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// Rejects a clock that ignores either half of "something reached the session": an item sent to it,
// and the answer that disposed an item it sent.
func TestLastInboundIsTheNewestArrivalOrAnswer(t *testing.T) {
	// covers: R-WRK-050
	s := tree(t)
	if at, err := s.LastInbound(sA); err != nil || !at.IsZero() {
		t.Fatalf("LastInbound with nothing = %v, %v; want zero", at, err)
	}
	asked := now.Add(time.Minute)
	req := send(t, s, sA, sB, store.IntentRequest, "may I?", asked)
	if at, err := s.LastInbound(sA); err != nil || !at.IsZero() {
		t.Fatalf("a request A sent is not inbound for A: %v, %v", at, err)
	}
	if at, err := s.LastInbound(sB); err != nil || !at.Equal(asked) {
		t.Fatalf("LastInbound(B) = %v, %v; want %v", at, err, asked)
	}
	answered := now.Add(30*time.Minute + 3*time.Second)
	if _, err := s.Reply(req, sB, "yes", store.Resolved, answered); err != nil {
		t.Fatal(err)
	}
	if at, err := s.LastInbound(sA); err != nil || !at.Equal(answered) {
		t.Fatalf("LastInbound(A) after the answer = %v, %v; want %v", at, err, answered)
	}
	fyi := answered.Add(time.Hour)
	send(t, s, sC, sA, store.IntentFYI, "note", fyi)
	if at, err := s.LastInbound(sA); err != nil || !at.Equal(fyi) {
		t.Fatalf("LastInbound(A) after an fyi = %v, %v; want %v", at, err, fyi)
	}
}

// Rejects a clock restarted by what a session sends itself: a probe's wake and its disposal come
// from the owner to the owner, so a probe on a short period must not keep a stuck owner from a stall.
func TestLastInboundIgnoresAProbeOwnersOwnWakes(t *testing.T) {
	// covers: R-WRK-050
	s := tree(t)
	p, err := s.StartProbe("probes/x.md", sB, "", false, now, now)
	if err != nil {
		t.Fatal(err)
	}
	wake, err := s.SendWake(p.ID, "probe probes/x.md run 1: RED", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if err = s.Dispose(wake, store.Resolved, "ok", now.Add(2*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if at, lerr := s.LastInbound(sB); lerr != nil || !at.IsZero() {
		t.Fatalf("LastInbound(owner) = %v, %v; a wake it sent itself is not an arrival", at, lerr)
	}
}
