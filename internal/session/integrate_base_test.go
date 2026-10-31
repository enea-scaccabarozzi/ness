package session_test

import (
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/merge"
	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// Rejects an implementation that does not tell the gate and the setup command where the fold
// started: NESS_BASE must be the target's sha before the fold, not the folded commit and not unset.
func TestIntegrateGivesGateAndSetupTheFoldBase(t *testing.T) {
	// covers: R-SESS-156
	g, repo, common := setupRepo(t)
	before, err := g.Rev(repo, "main")
	if err != nil {
		t.Fatal(err)
	}
	spec := session.IntegrateSpec{
		Repo: repo, Common: common, TmpName: "tmp-s-1",
		Setup:       `test "$NESS_BASE" = ` + before,
		Gate:        `test "$(git rev-parse "$NESS_BASE")" = ` + before + ` && git diff --name-only "$NESS_BASE" | grep -qx a.txt`,
		Integration: session.Integration{Source: "session/s-1", Target: "main"},
	}
	if _, err = session.Integrate(g, merge.Integrator{}, spec); err != nil {
		t.Fatalf("integrate with a gate reading NESS_BASE: %v", err)
	}
}
