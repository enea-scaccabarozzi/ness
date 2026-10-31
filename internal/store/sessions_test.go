package store_test

import (
	"errors"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestSessionsTreeAndMarks(t *testing.T) {
	// The minimal session rows P3 builds on: parent, state, the ready mark and the boundary.
	// covers: R-HOOK-003, R-HOOK-009, R-MSG-034
	s := mustOpen(t, freshPath(t))
	if err := s.AddSession("s-aaaaaaaaaaaa", "root", store.Operator, now); err != nil {
		t.Fatal(err)
	}
	if err := s.AddSession("s-bbbbbbbbbbbb", "child", "s-aaaaaaaaaaaa", now); err != nil {
		t.Fatal(err)
	}
	if err := s.AddSession("s-cccccccccccc", "orphan", "s-nope", now); err == nil {
		t.Fatal("a parent that does not exist was accepted")
	}
	child, err := s.Session("s-bbbbbbbbbbbb")
	if err != nil {
		t.Fatal(err)
	}
	if child.Parent != "s-aaaaaaaaaaaa" || child.Scope != "child" || child.Closed ||
		!child.ReadyAt.IsZero() || !child.CreatedAt.Equal(now) {
		t.Fatalf("child = %+v", child)
	}
	root, err := s.Session("s-aaaaaaaaaaaa")
	if err != nil || root.Parent != store.Operator {
		t.Fatalf("root = %+v, %v", root, err)
	}
	if _, err := s.Session("s-nope"); !errors.Is(err, store.ErrNoSession) {
		t.Fatalf("unknown session: %v", err)
	}

	later := now.Add(5 * time.Minute)
	if err := s.MarkReady("s-bbbbbbbbbbbb", now); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkReady("s-bbbbbbbbbbbb", later); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkReady("s-nope", now); !errors.Is(err, store.ErrNoSession) {
		t.Fatalf("MarkReady unknown: %v", err)
	}
	if err := s.SetBoundary("s-bbbbbbbbbbbb", "plan written", time.Time{}, later); err != nil {
		t.Fatal(err)
	}
	if err := s.CloseSession("s-bbbbbbbbbbbb"); err != nil {
		t.Fatal(err)
	}
	if err := s.CloseSession("s-nope"); !errors.Is(err, store.ErrNoSession) {
		t.Fatalf("CloseSession unknown: %v", err)
	}
	child, err = s.Session("s-bbbbbbbbbbbb")
	if err != nil {
		t.Fatal(err)
	}
	if !child.ReadyAt.Equal(now) {
		t.Fatalf("ready mark moved to %v; the first mark must win", child.ReadyAt)
	}
	if child.BoundaryNote != "plan written" || !child.BoundaryAt.Equal(later) || !child.Closed {
		t.Fatalf("child = %+v", child)
	}
}

func TestClearBoundaryDropsTheRequest(t *testing.T) {
	// covers: R-WRK-032
	s := tree(t)
	if err := s.SetBoundary(sA, "plan written", time.Time{}, now); err != nil {
		t.Fatal(err)
	}
	if err := s.ClearBoundary(sA); err != nil {
		t.Fatal(err)
	}
	sess, err := s.Session(sA)
	if err != nil || sess.BoundaryNote != "" || !sess.BoundaryAt.IsZero() {
		t.Fatalf("after ClearBoundary: %+v, %v", sess, err)
	}
	if err := s.ClearBoundary("s-nope"); !errors.Is(err, store.ErrNoSession) {
		t.Fatalf("unknown session: %v", err)
	}
}
