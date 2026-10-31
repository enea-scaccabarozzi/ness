package store_test

import (
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func at(min int) time.Time { return now.Add(time.Duration(min) * time.Minute) }

// landingTree adds root r, its child c, c's child g and an unrelated root o with child oc.
func landingTree(t *testing.T, s *store.Store) {
	t.Helper()
	for _, a := range [][2]string{
		{"s-r00000000000", store.Operator}, {"s-c00000000000", "s-r00000000000"},
		{"s-g00000000000", "s-c00000000000"}, {"s-o00000000000", store.Operator},
		{"s-oc0000000000", "s-o00000000000"},
	} {
		if err := s.AddSession(a[0], "scope", a[1], now); err != nil {
			t.Fatal(err)
		}
	}
}

func landing(t *testing.T, s *store.Store, root string) time.Time {
	t.Helper()
	got, err := s.LastLanding(root)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestLastLandingIsZeroWhenNothingLanded(t *testing.T) {
	// covers: R-MSG-071
	s := mustOpen(t, freshPath(t))
	landingTree(t, s)
	if got := landing(t, s, "s-r00000000000"); !got.IsZero() {
		t.Fatalf("LastLanding = %v, want zero", got)
	}
}

func TestLastLandingCountsDescendantsAndClosedChildren(t *testing.T) {
	// covers: R-MSG-071
	s := mustOpen(t, freshPath(t))
	landingTree(t, s)
	if err := s.SetIntegration("s-g00000000000", "aaa", at(5)); err != nil {
		t.Fatal(err)
	}
	if got := landing(t, s, "s-r00000000000"); !got.Equal(at(5)) {
		t.Fatalf("grandchild landing: LastLanding = %v, want %v", got, at(5))
	}
	if err := s.SetIntegration("s-c00000000000", "bbb", at(3)); err != nil {
		t.Fatal(err)
	}
	if got := landing(t, s, "s-r00000000000"); !got.Equal(at(5)) {
		t.Fatalf("latest of two: LastLanding = %v, want %v", got, at(5))
	}
	if err := s.RecordClose("s-g00000000000", "done", "", at(6)); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordClose("s-c00000000000", "done", "", at(7)); err != nil {
		t.Fatal(err)
	}
	if got := landing(t, s, "s-r00000000000"); !got.Equal(at(5)) {
		t.Fatalf("closed sessions: LastLanding = %v, want %v", got, at(5))
	}
}

func TestLastLandingIgnoresTheRootItselfAndOtherTrees(t *testing.T) {
	// covers: R-MSG-071
	s := mustOpen(t, freshPath(t))
	landingTree(t, s)
	if err := s.SetIntegration("s-r00000000000", "root", at(9)); err != nil {
		t.Fatal(err)
	}
	if err := s.SetIntegration("s-oc0000000000", "other", at(8)); err != nil {
		t.Fatal(err)
	}
	if got := landing(t, s, "s-r00000000000"); !got.IsZero() {
		t.Fatalf("LastLanding = %v, want zero: only descendants of the root count", got)
	}
	if got := landing(t, s, "s-o00000000000"); !got.Equal(at(8)) {
		t.Fatalf("other tree: LastLanding = %v, want %v", got, at(8))
	}
}

func TestLastCheckpointIsTheLatestFromTheRootToTheOperator(t *testing.T) {
	// covers: R-MSG-072
	s := mustOpen(t, freshPath(t))
	landingTree(t, s)
	got, err := s.LastCheckpoint("s-r00000000000")
	if err != nil || !got.IsZero() {
		t.Fatalf("no checkpoint: %v, %v; want zero", got, err)
	}
	send := func(from, to, intent, body string, when time.Time) {
		t.Helper()
		if _, _, err := s.Send(from, to, intent, body, false, when); err != nil {
			t.Fatal(err)
		}
	}
	send("s-r00000000000", store.Operator, store.IntentCheckpoint, "## Done\n## Now\n## Next\none", at(2))
	send("s-r00000000000", store.Operator, store.IntentCheckpoint, "## Done\n## Now\n## Next\ntwo", at(4))
	// None of these counts: another sender, another recipient, another intent.
	send("s-o00000000000", store.Operator, store.IntentCheckpoint, "## Done\n## Now\n## Next\nother", at(20))
	send("s-r00000000000", "s-c00000000000", store.IntentCheckpoint, "## Done\n## Now\n## Next\nto child", at(21))
	send("s-r00000000000", store.Operator, "fyi", "note", at(22))
	got, err = s.LastCheckpoint("s-r00000000000")
	if err != nil {
		t.Fatal(err)
	}
	if !got.Equal(at(4)) {
		t.Fatalf("LastCheckpoint = %v, want %v", got, at(4))
	}
}
