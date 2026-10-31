package store_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// waits reads id's waits at at, failing the test on an error.
func waits(t *testing.T, s *store.Store, id string, at time.Time) []store.Wait {
	t.Helper()
	got, err := s.Waits(id, at)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

// kinds lists the kinds of ws in order.
func kinds(ws []store.Wait) []string {
	out := make([]string, 0, len(ws))
	for _, w := range ws {
		out = append(out, w.Kind)
	}
	return out
}

// Each case rejects one wrong reading of "an item the session waits on": accepted dropped (the
// field's "I'm on it" silence), a settled, retired or label-only item kept, a handoff kept (a child
// that handed off owes no brief, and a follow-up obligation must still stall), the recipient's side
// counted, or the operator's ask not told apart from an item to a session.
func TestWaitsOnItemsSent(t *testing.T) {
	// covers: R-WRK-099
	cases := []struct {
		name string
		do   func(t *testing.T, s *store.Store) string // returns the item id, "" when none counts
		kind string
	}{
		{"an open request to the manager", func(t *testing.T, s *store.Store) string {
			t.Helper()
			return sendNow(t, s, sB, sA, store.IntentRequest, "may I?")
		}, "item"},
		{"an accepted request still waits", func(t *testing.T, s *store.Store) string {
			t.Helper()
			id := sendNow(t, s, sB, sA, store.IntentRequest, "may I?")
			reply(t, s, id, sA, store.Accepted)
			return id
		}, "item"},
		{"a deferred decision still waits", func(t *testing.T, s *store.Store) string {
			t.Helper()
			id := sendNow(t, s, sB, sA, store.IntentDecision, "which?")
			reply(t, s, id, sA, store.Deferred)
			return id
		}, "item"},
		{"an open handoff is delivered work, not a wait", func(t *testing.T, s *store.Store) string {
			t.Helper()
			sendNow(t, s, sB, sA, store.IntentHandoff, "done")
			return ""
		}, ""},
		{"a decision to the operator is an ask", func(t *testing.T, s *store.Store) string {
			t.Helper()
			return sendNow(t, s, sB, store.Operator, store.IntentDecision, "which?")
		}, "ask"},
		{"a checkpoint with an Ask is an ask", func(t *testing.T, s *store.Store) string {
			t.Helper()
			return sendNow(t, s, sB, store.Operator, store.IntentCheckpoint,
				"## Done\nd\n## Now\nn\n## Next\nx\n## Ask\nShip it?\n")
		}, "ask"},
		{"a resolved request", func(t *testing.T, s *store.Store) string {
			t.Helper()
			reply(t, s, sendNow(t, s, sB, sA, store.IntentRequest, "may I?"), sA, store.Resolved)
			return ""
		}, ""},
		{"a declined request", func(t *testing.T, s *store.Store) string {
			t.Helper()
			reply(t, s, sendNow(t, s, sB, sA, store.IntentRequest, "may I?"), sA, store.Declined)
			return ""
		}, ""},
		{"a returned handoff", func(t *testing.T, s *store.Store) string {
			t.Helper()
			reply(t, s, sendNow(t, s, sB, sA, store.IntentHandoff, "done"), sA, store.Returned)
			return ""
		}, ""},
		{"a withdrawn request", func(t *testing.T, s *store.Store) string {
			t.Helper()
			id := sendNow(t, s, sB, sA, store.IntentRequest, "may I?")
			if _, err := s.Withdraw(id, "did it", now); err != nil {
				t.Fatal(err)
			}
			return ""
		}, ""},
		{"a request retired with its closed recipient", func(t *testing.T, s *store.Store) string {
			t.Helper()
			sendNow(t, s, sB, sC, store.IntentRequest, "may I?")
			if err := s.CloseSession(sC); err != nil {
				t.Fatal(err)
			}
			if _, err := s.RetireDead(now); err != nil {
				t.Fatal(err)
			}
			return ""
		}, ""},
		{"an fyi", func(t *testing.T, s *store.Store) string {
			t.Helper()
			sendNow(t, s, sB, sA, store.IntentFYI, "note")
			return ""
		}, ""},
		{"a checkpoint without an Ask", func(t *testing.T, s *store.Store) string {
			t.Helper()
			sendNow(t, s, sB, store.Operator, store.IntentCheckpoint, "## Done\nd\n## Now\nn\n## Next\nx\n")
			return ""
		}, ""},
		{"a request received, not sent", func(t *testing.T, s *store.Store) string {
			t.Helper()
			sendNow(t, s, sA, sB, store.IntentRequest, "do it")
			return ""
		}, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			s := tree(t)
			id := c.do(t, s)
			got := waits(t, s, sB, now.Add(time.Hour))
			if id == "" {
				if len(got) != 0 {
					t.Fatalf("waits = %+v, want none", got)
				}
				return
			}
			if len(got) != 1 || got[0].Kind != c.kind || got[0].Ref != id || !got[0].Until.IsZero() {
				t.Fatalf("waits = %+v, want one %s on %s, open-ended", got, c.kind, id)
			}
			if c.kind == "item" && got[0].Note != "to "+sA {
				t.Fatalf("note = %q, want %q", got[0].Note, "to "+sA)
			}
		})
	}
}

// Rejects a probe wait that counts paused, stopped or another owner's probes, or that names
// neither the probe nor its path.
func TestWaitsOnActiveProbes(t *testing.T) {
	// covers: R-WRK-099
	s := tree(t)
	active, err := s.StartProbe("probes/deploy.md", sB, "", false, now, now)
	if err != nil {
		t.Fatal(err)
	}
	got := waits(t, s, sB, now)
	if len(got) != 1 || got[0].Kind != "probe" || got[0].Ref != active.ID ||
		!strings.Contains(got[0].Note, "probes/deploy.md") {
		t.Fatalf("active probe: waits = %+v", got)
	}
	if other := waits(t, s, sC, now); len(other) != 0 {
		t.Fatalf("another owner's probe: %+v", other)
	}
	for _, state := range []string{store.ProbePaused, store.ProbeStopped} {
		if err := s.SetProbeState(active.ID, state, "x", now); err != nil {
			t.Fatal(err)
		}
		if got := waits(t, s, sB, now); len(got) != 0 {
			t.Fatalf("%s probe: waits = %+v", state, got)
		}
	}
}

// Rejects a parked record that ignores its until-time, ends an until-park on an arrival, never
// ends an open-ended park, or is lost when the worker clears the compaction request.
func TestWaitsWhileParked(t *testing.T) {
	// covers: R-WRK-099, R-MSG-034
	t.Run("until a time", func(t *testing.T) {
		s := tree(t)
		until := now.Add(3 * time.Hour)
		if err := s.SetBoundary(sB, "waiting for the release window", until, now); err != nil {
			t.Fatal(err)
		}
		// an arrival does not end an until-park
		if _, _, err := s.Send(sA, sB, store.IntentFYI, "fyi meanwhile", false, now.Add(10*time.Minute)); err != nil {
			t.Fatal(err)
		}
		got := waits(t, s, sB, now.Add(time.Hour))
		if len(got) != 1 || got[0].Kind != "parked" || !got[0].Until.Equal(until) ||
			got[0].Note != "waiting for the release window" {
			t.Fatalf("before the until: %+v", got)
		}
		if got := waits(t, s, sB, until.Add(time.Minute)); len(got) != 0 {
			t.Fatalf("after the until: %+v", got)
		}
	})
	t.Run("until something arrives", func(t *testing.T) {
		s := tree(t)
		if err := s.SetBoundary(sB, "plan written", time.Time{}, now); err != nil {
			t.Fatal(err)
		}
		if got := waits(t, s, sB, now.Add(5*time.Hour)); len(got) != 1 || got[0].Kind != "parked" ||
			!got[0].Until.IsZero() {
			t.Fatalf("open-ended park: %+v", got)
		}
		if _, _, err := s.Send(sA, sB, store.IntentFYI, "go on", false, now.Add(5*time.Hour)); err != nil {
			t.Fatal(err)
		}
		if got := waits(t, s, sB, now.Add(6*time.Hour)); len(got) != 0 {
			t.Fatalf("after an arrival: %+v", got)
		}
	})
	t.Run("the compaction keeps the record", func(t *testing.T) {
		s := tree(t)
		until := now.Add(time.Hour)
		if err := s.SetBoundary(sB, "plan written", until, now); err != nil {
			t.Fatal(err)
		}
		if err := s.ClearBoundary(sB); err != nil {
			t.Fatal(err)
		}
		sess, err := s.Session(sB)
		if err != nil || sess.BoundaryNote != "" || !sess.ParkedAt.Equal(now) || !sess.ParkedUntil.Equal(until) ||
			sess.ParkedNote != "plan written" {
			t.Fatalf("after ClearBoundary: %+v, %v", sess, err)
		}
		if got := waits(t, s, sB, now.Add(time.Minute)); len(got) != 1 || got[0].Kind != "parked" {
			t.Fatalf("waits after the compaction: %+v", got)
		}
	})
	t.Run("the operator's item or answer ends it", func(t *testing.T) {
		// Rejects LastInbound comparing with <> against the operator's NULL: neither counted.
		s := tree(t)
		ask := sendNow(t, s, sA, store.Operator, store.IntentDecision, "which?")
		if err := s.SetBoundary(sA, "asked", time.Time{}, now.Add(time.Minute)); err != nil {
			t.Fatal(err)
		}
		if _, err := s.Reply(ask, store.Operator, "the first", store.Resolved, now.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
		if last, err := s.LastInbound(sA); err != nil || !last.Equal(now.Add(time.Hour)) {
			t.Fatalf("LastInbound after the operator's answer = %v, %v", last, err)
		}
		if got := waits(t, s, sA, now.Add(2*time.Hour)); len(got) != 0 {
			t.Fatalf("after the operator's answer: %+v", got)
		}
		if _, _, err := s.Send(store.Operator, sA, store.IntentFYI, "fyi", false, now.Add(3*time.Hour)); err != nil {
			t.Fatal(err)
		}
		if last, err := s.LastInbound(sA); err != nil || !last.Equal(now.Add(3*time.Hour)) {
			t.Fatalf("LastInbound after the operator's item = %v, %v", last, err)
		}
	})
	t.Run("Unpark drops it", func(t *testing.T) {
		s := tree(t)
		if err := s.SetBoundary(sB, "plan written", time.Time{}, now); err != nil {
			t.Fatal(err)
		}
		if err := s.Unpark(sB); err != nil {
			t.Fatal(err)
		}
		sess, err := s.Session(sB)
		if err != nil || !sess.ParkedAt.IsZero() || sess.ParkedNote != "" || sess.BoundaryNote != "plan written" {
			t.Fatalf("after Unpark: %+v, %v", sess, err)
		}
		if got := waits(t, s, sB, now); len(got) != 0 {
			t.Fatalf("waits after Unpark: %+v", got)
		}
	})
	t.Run("a new boundary replaces it", func(t *testing.T) {
		s := tree(t)
		if err := s.SetBoundary(sB, "first", now.Add(time.Hour), now); err != nil {
			t.Fatal(err)
		}
		later := now.Add(10 * time.Minute)
		if err := s.SetBoundary(sB, "second", time.Time{}, later); err != nil {
			t.Fatal(err)
		}
		sess, err := s.Session(sB)
		if err != nil || sess.ParkedNote != "second" || !sess.ParkedAt.Equal(later) || !sess.ParkedUntil.IsZero() {
			t.Fatalf("after a second boundary: %+v, %v", sess, err)
		}
	})
}

// Rejects an order other than asks, items, probes, parked (here created in another order).
func TestWaitsOrder(t *testing.T) {
	// covers: R-WRK-099
	s := tree(t)
	if err := s.SetBoundary(sB, "parked", now.Add(time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartProbe("probes/x.md", sB, "", false, now, now); err != nil {
		t.Fatal(err)
	}
	sendNow(t, s, sB, sA, store.IntentRequest, "may I?")
	sendNow(t, s, sB, store.Operator, store.IntentDecision, "which?")
	got := kinds(waits(t, s, sB, now.Add(time.Minute)))
	if want := []string{"ask", "item", "probe", "parked"}; !slices.Equal(got, want) {
		t.Fatalf("kinds = %v, want %v", got, want)
	}
}

// Rejects a schema change made by editing a shipped migration or without one: a version-11 store
// with a session opens at the current version, the session unparked and parkable.
func TestMigrationTwelveAddsTheParkedRecord(t *testing.T) {
	// covers: R-MSG-034
	path := freshPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	db := raw(t, path)
	for _, q := range store.Migrations[:11] {
		if _, err := db.ExecContext(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []string{
		`INSERT INTO sessions (id, scope, created_at, boundary_note, boundary_at)
		 VALUES ('` + sA + `', 'root', '2026-10-08T00:00:00Z', 'old', '2026-10-08T00:00:00Z')`,
		`PRAGMA user_version = 11`,
	} {
		if _, err := db.ExecContext(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	if store.Version() < 12 {
		t.Fatalf("schema version %d: migration 12 missing", store.Version())
	}
	s := mustOpen(t, path)
	if got := userVersion(t, path); got != store.Version() {
		t.Fatalf("user_version = %d, want %d", got, store.Version())
	}
	sess, err := s.Session(sA)
	if err != nil || sess.BoundaryNote != "old" || !sess.ParkedAt.IsZero() || sess.ParkedNote != "" {
		t.Fatalf("old row = %+v, %v", sess, err)
	}
	if got := waits(t, s, sA, now); len(got) != 0 {
		t.Fatalf("an old boundary is not a park: %+v", got)
	}
	if err := s.SetBoundary(sA, "new", now.Add(time.Hour), now); err != nil {
		t.Fatal(err)
	}
	if got := waits(t, s, sA, now); len(got) != 1 || got[0].Kind != "parked" {
		t.Fatalf("after a boundary: %+v", got)
	}
}

func sendNow(t *testing.T, s *store.Store, from, to, intent, body string) string {
	t.Helper()
	id, _, err := s.Send(from, to, intent, body, false, now)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

func reply(t *testing.T, s *store.Store, id, by, disposition string) {
	t.Helper()
	if _, err := s.Reply(id, by, "answer", disposition, now); err != nil {
		t.Fatal(err)
	}
}
