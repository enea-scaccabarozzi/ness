package store_test

import (
	"regexp"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestNewItemIDShapeAndUniqueness(t *testing.T) {
	// covers: R-MSG-044
	// 10,000 draws, as the spec says: 40 random bits make a birthday collision at
	// 100,000 a 0.45% event per run, which would flake the gate; at 10,000 it is 4.5e-5.
	// Uniqueness in the store itself rests on the primary key.
	re := regexp.MustCompile(`^m-[a-z2-7]{8}$`)
	seen := make(map[string]struct{}, 10000)
	for range 10000 {
		id := store.NewItemID()
		if !re.MatchString(id) {
			t.Fatalf("id %q does not match ^m-[a-z2-7]{8}$", id)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = struct{}{}
	}
}
