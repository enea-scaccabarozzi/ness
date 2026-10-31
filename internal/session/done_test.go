package session_test

import (
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// Rejects an implementation that shows a quick root done before it integrated, shows a full root
// done, or hides a launch phase behind done.
func TestShownIsDoneForAnIntegratedQuickRoot(t *testing.T) {
	// covers: R-SESS-157
	now := time.Date(2026, 10, 20, 10, 0, 0, 0, time.UTC)
	ready := store.Session{Phase: store.PhaseReady, ReadyAt: now.Add(-time.Hour), Size: "quick"}
	if got := session.Shown(ready, now, time.Minute); got != "ready" {
		t.Fatalf("quick, not integrated shown as %q", got)
	}
	ready.IntegratedAt = now.Add(-time.Minute)
	if got := session.Shown(ready, now, time.Minute); got != "done" {
		t.Fatalf("quick, integrated shown as %q, want done", got)
	}
	ready.Size = "full"
	if got := session.Shown(ready, now, time.Minute); got != "ready" {
		t.Fatalf("full, integrated shown as %q, want ready", got)
	}
	if !session.Done(store.Session{Size: "quick", IntegratedAt: now}) || session.Done(ready) {
		t.Fatal("Done must hold for an integrated quick root only")
	}
}
