package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestMigrationTwoAddsLaunchColumns(t *testing.T) {
	// A real v1 store (built by hand) opens, migrates forward and serves the new fields.
	// covers: R-MSG-046
	path := freshPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	db := raw(t, path)
	for _, q := range []string{
		store.Migrations[0],
		`INSERT INTO sessions (id, scope, created_at) VALUES ('` + sA + `', 'root', '2026-10-08T00:00:00Z')`,
		`INSERT INTO items (id, key, sender, recipient, intent, body, created_at)
		 VALUES ('i-old', 'k', NULL, '` + sA + `', 'fyi', 'kept', '2026-10-08T00:00:00Z')`,
		`PRAGMA user_version = 1`,
	} {
		if _, err := db.ExecContext(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s := mustOpen(t, path)
	if got := userVersion(t, path); got != store.Version() {
		t.Fatalf("user_version = %d, want %d", got, store.Version())
	}
	sess, err := s.Session(sA)
	if err != nil || sess.Phase != store.PhaseReady || sess.Client != "claude" {
		t.Fatalf("legacy row = %+v, %v", sess, err)
	}
	if items, lerr := s.Listed(sA); lerr != nil || len(items) != 1 || items[0].Body != "kept" {
		t.Fatalf("legacy item after upgrade = %v, %v", items, lerr)
	}
}

func TestLaunchPhasesAndRuns(t *testing.T) {
	// covers: R-SESS-031, R-SESS-048, R-SESS-051
	s := mustOpen(t, freshPath(t))
	l := store.Launch{
		ID: sA, Parent: store.Operator, Scope: "root", Title: "root", Tier: "standard",
		Effort: "medium", Client: "claude", Record: "docs/sessions/root", Base: "main",
		Worktree: ".worktrees/root-aaaaaaaaaaaa", Branch: "session/root-aaaaaaaaaaaa", RouterReason: "explicit --tier",
	}
	if err := s.AddLaunch(l, now); err != nil {
		t.Fatal(err)
	}
	sess, _ := s.Session(sA)
	if sess.Phase != store.PhaseRegistered || !sess.PhaseAt.Equal(now) || sess.Run != 0 ||
		sess.RouterReason != "explicit --tier" {
		t.Fatalf("after AddLaunch: %+v", sess)
	}
	if err := s.SetPhase(sA, "opening-tab", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := s.BindTab(sA, "w1:p2", "w1:t2", 1); err != nil {
		t.Fatal(err)
	}
	if err := s.BindAgent(sA, "p-root", "c0ffee"); err != nil {
		t.Fatal(err)
	}
	sess, _ = s.Session(sA)
	if sess.Phase != "opening-tab" || sess.Pane != "w1:p2" || sess.Tab != "w1:t2" || sess.Run != 1 ||
		sess.Agent != "p-root" || sess.ClientSession != "c0ffee" {
		t.Fatalf("bound: %+v", sess)
	}
	if err := s.MarkReady(sA, now); err != nil {
		t.Fatal(err)
	}
	// A new run clears the binding and the ready mark and may change the tier (R-SESS-048,
	// R-SESS-051). The client session survives so the relaunch resumes the transcript (R-SESS-047).
	if err := s.NewRun(sA, "opening-tab", "frontier", "medium", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	sess, _ = s.Session(sA)
	if sess.Pane != "" || sess.Tab != "" || sess.Agent != "" || sess.ClientSession != "c0ffee" ||
		!sess.ReadyAt.IsZero() || !sess.FirstReadyAt.Equal(now) || sess.Tier != "frontier" || sess.Phase != "opening-tab" || !sess.PhaseAt.Equal(now.Add(time.Hour)) {
		t.Fatalf("new run: %+v", sess)
	}
	if err := s.MarkReady(sA, now.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if sess, _ = s.Session(sA); !sess.FirstReadyAt.Equal(now) {
		t.Fatalf("first ready mark moved to %v", sess.FirstReadyAt)
	}
	if err := s.BindTab(sA, "w1:p9", "w1:t9", 2); err != nil {
		t.Fatal(err)
	}
	sess, _ = s.Session(sA)
	if sess.Run != 2 || sess.Pane != "w1:p9" || sess.ClientSession != "c0ffee" {
		t.Fatalf("second run: %+v", sess)
	}
	if err := s.SetPhase("s-nope", "ready", now); !errors.Is(err, store.ErrNoSession) {
		t.Fatalf("SetPhase unknown: %v", err)
	}
}

func TestAdoptedRootIsReadyAtOnce(t *testing.T) {
	// covers: R-SESS-038
	s := mustOpen(t, freshPath(t))
	l := store.Launch{ID: sA, Scope: "root", Client: "claude", Adopted: true, Base: "main"}
	if err := s.AddLaunch(l, now); err != nil {
		t.Fatal(err)
	}
	sess, _ := s.Session(sA)
	if !sess.Adopted || sess.Phase != store.PhaseReady || !sess.ReadyAt.Equal(now) || sess.Worktree != "" {
		t.Fatalf("adopted: %+v", sess)
	}
}

func TestSessionsChildrenCloseAndRetire(t *testing.T) {
	// covers: R-SESS-065, R-SESS-069, R-SESS-086, R-SESS-099
	s := tree(t) // sA root, sB and sC children of sA (items_test.go)
	kids, err := s.Children(sA)
	if err != nil || len(kids) != 2 || kids[0].ID != sB || kids[1].ID != sC {
		t.Fatalf("Children = %v, %v", kids, err)
	}
	roots, err := s.Children(store.Operator)
	if err != nil || len(roots) != 1 || roots[0].ID != sA {
		t.Fatalf("Children(operator) = %v, %v", roots, err)
	}
	if _, _, err = s.Send(sA, sB, store.IntentRequest, "pending", false, now); err != nil {
		t.Fatal(err)
	}
	dec, _, err := s.Send(sA, sB, store.IntentDecision, "deferred one", false, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Reply(dec, sB, "later", store.Deferred, now); err != nil {
		t.Fatal(err)
	}
	if err = s.SetIntegration(sB, "abc123", now); err != nil {
		t.Fatal(err)
	}
	n, err := s.Retire(sB, now)
	if err != nil || n != 2 {
		t.Fatalf("Retire = %d, %v", n, err)
	}
	if items, _ := s.Listed(sB); len(items) != 0 {
		t.Fatalf("retired item still listed: %v", items)
	}
	if obl, _ := s.Obligations(sB); len(obl) != 0 {
		t.Fatalf("retired item still an obligation: %v", obl)
	}
	if err = s.RecordClose(sB, "done", "", now); err != nil {
		t.Fatal(err)
	}
	sess, _ := s.Session(sB)
	if !sess.Closed || sess.CloseNote != "done" || sess.Integrated != "abc123" || !sess.ClosedAt.Equal(now) {
		t.Fatalf("closed row: %+v", sess)
	}
	open, _ := s.Sessions(false)
	all, _ := s.Sessions(true)
	sids := func(ss []store.Session) []string {
		out := make([]string, 0, len(ss))
		for _, x := range ss {
			out = append(out, x.ID)
		}
		return out
	}
	if !slices.Equal(sids(open), []string{sA, sC}) || !slices.Equal(sids(all), []string{sA, sB, sC}) {
		t.Fatalf("Sessions: open %v all %v", sids(open), sids(all))
	}
}

func TestHeartbeat(t *testing.T) {
	// covers: R-SESS-082
	s := mustOpen(t, freshPath(t))
	if _, err := s.Heartbeat(); !errors.Is(err, store.ErrNoHeartbeat) {
		t.Fatalf("empty heartbeat: %v", err)
	}
	if err := s.SetHeartbeat(now, "0.1.0"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetHeartbeat(now.Add(time.Minute), "0.1.1"); err != nil {
		t.Fatal(err)
	}
	hb, err := s.Heartbeat()
	if err != nil || !hb.At.Equal(now.Add(time.Minute)) || hb.Version != "0.1.1" {
		t.Fatalf("Heartbeat = %+v, %v", hb, err)
	}
}

func TestCleanGitEnv(t *testing.T) {
	// The discovery variables go; authorship and transport stay.
	// covers: R-QA-021
	in := []string{"GIT_DIR=/x", "GIT_WORK_TREE=/y", "GIT_AUTHOR_NAME=a", "GIT_SSH_COMMAND=ssh", "PATH=/bin"}
	got := store.CleanGitEnv(in)
	if !slices.Equal(got, []string{"GIT_AUTHOR_NAME=a", "GIT_SSH_COMMAND=ssh", "PATH=/bin"}) {
		t.Fatalf("CleanGitEnv = %v", got)
	}
}

func TestAdoptedByFindsOnlyOpenAdoptedRoots(t *testing.T) {
	// covers: R-SESS-114
	s := mustOpen(t, freshPath(t))
	if err := s.AddLaunch(store.Launch{ID: sA, Scope: "root", Client: "claude", Adopted: true, Base: "main"}, now); err != nil {
		t.Fatal(err)
	}
	if err := s.AddLaunch(store.Launch{ID: sB, Scope: "kid", Client: "claude", Base: "main"}, now); err != nil {
		t.Fatal(err)
	}
	if err := s.BindAgent(sA, "a", "c1"); err != nil {
		t.Fatal(err)
	}
	if err := s.BindAgent(sB, "b", "c2"); err != nil {
		t.Fatal(err)
	}
	got, err := s.AdoptedBy("c1")
	if err != nil || got.ID != sA {
		t.Fatalf("AdoptedBy(c1) = %+v, %v", got, err)
	}
	for _, id := range []string{"c2", "", "nope"} {
		if _, err := s.AdoptedBy(id); !errors.Is(err, store.ErrNoSession) {
			t.Fatalf("AdoptedBy(%q): %v", id, err)
		}
	}
	if err := s.CloseSession(sA); err != nil {
		t.Fatal(err)
	}
	if _, err := s.AdoptedBy("c1"); !errors.Is(err, store.ErrNoSession) {
		t.Fatalf("closed adopted root found: %v", err)
	}
}
