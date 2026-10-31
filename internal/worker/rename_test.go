package worker_test

import (
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// adopted registers a root adopted with --here: no agent name, any agent accepted.
func (b *bench) adopted(id, title string) {
	b.t.Helper()
	pane := paneOf(id)
	l := store.Launch{
		ID: id, Parent: store.Operator, Scope: "x", Title: title, Tier: "standard", Effort: "medium",
		Client: "claude", Record: "docs/sessions/x", Base: "main", Worktree: ".worktrees/x-" + id,
		Branch: "session/x-" + id, Adopted: true,
	}
	for _, err := range []error{
		b.s.AddLaunch(l, b.now), b.s.BindTab(id, pane, "t:"+pane, 1), b.s.BindAgent(id, "", "u-1"),
		b.s.MarkReady(id, b.now),
	} {
		if err != nil {
			b.t.Fatal(err)
		}
	}
	b.rt.panes[pane] = &fakePane{status: "idle", agent: "claude", screen: idleScreen, live: true}
}

func TestRenamesAnAdoptedRootOnceWhenFree(t *testing.T) {
	// An adopted root is renamed <project>/<title> at its first idle, empty-box pass,
	// once, focused or not; while holding a draft it waits. A launched session is never renamed: ness
	// named it at launch.
	// covers: R-WRK-071
	b := newBench(t)
	b.adopted(sA, "nightly")
	b.session(sB, store.Operator, "claude")
	b.pane(sA).screen = draftScreen
	b.passes(3, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 0 {
		t.Fatalf("typed into a drafted pane: %q", got)
	}
	b.pane(sA).screen = idleScreen
	b.pane(sA).focused = true // the operator's focus holds nothing
	b.passes(2, time.Minute)  // past the quiet window the draft opened
	b.passes(5, 5*time.Second)
	if got := b.typedTo(sA); len(got) != 1 || got[0] != "/rename demo-app/nightly" {
		t.Fatalf("typed %q", got)
	}
	if got := b.typedTo(sB); len(got) != 0 {
		t.Fatalf("a launched session was typed into: %q", got)
	}
	if b.logCount(" rename "+sA+" demo-app/nightly") != 1 {
		t.Fatal("the rename was not logged")
	}
}
