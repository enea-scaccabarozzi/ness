package cli

import (
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// Rejects a route table that lets a child leave its tree, a root address itself, or still refuses
// a root its peer root.
func TestRouteRefusalRootToRoot(t *testing.T) {
	// covers: R-MSG-017, R-MSG-073
	rootA := store.Session{ID: "s-aaaaaaaaaaaa", Parent: store.Operator}
	rootB := store.Session{ID: "s-bbbbbbbbbbbb", Parent: store.Operator}
	childA := store.Session{ID: "s-cccccccccccc", Parent: rootA.ID}
	childB := store.Session{ID: "s-dddddddddddd", Parent: rootB.ID}
	as := func(s store.Session) caller { return caller{id: s.ID, sess: s} }
	cases := []struct {
		name   string
		from   caller
		target store.Session
		ok     bool
	}{
		{"root to peer root", as(rootA), rootB, true},
		{"root to itself", as(rootA), rootA, false},
		{"root to another tree's child", as(rootA), childB, false},
		{"child to another root", as(childA), rootB, false},
		{"child to another tree's child", as(childA), childB, false},
		{"root to own child", as(rootA), childA, true},
	}
	for _, c := range cases {
		if got := routeRefusal(c.from, c.target) == ""; got != c.ok {
			t.Errorf("%s: allowed = %v, want %v", c.name, got, c.ok)
		}
	}
}

// Rejects a copy header that drops the item, the sender or the recipient.
func TestPeerCopyHeader(t *testing.T) {
	// covers: R-MSG-073
	if got := PeerCopyHeader("m-abcdefgh", "s-a", "s-b"); got != "[copy of m-abcdefgh from s-a to s-b]" {
		t.Fatalf("PeerCopyHeader = %q", got)
	}
}
