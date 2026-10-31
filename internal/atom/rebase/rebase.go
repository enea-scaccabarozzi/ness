// Package rebase is the integrator that replays a branch's commits on the target, so the target
// stays linear with one commit per source commit and no merge commit (R-SESS-118).
package rebase

import (
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// Integrator implements session.Integrator.
type Integrator struct{}

// Fold cherry-picks onto the throwaway's HEAD every source commit whose change the target does
// not hold yet, oldest first; the source branch itself is never rewritten.
func (Integrator) Fold(g session.Git, dir, source, target string) (string, error) {
	commits, err := pending(g, dir, source, target)
	if err != nil {
		return "", err
	}
	if len(commits) > 0 {
		if _, err = g.Out(dir, append([]string{"cherry-pick", "--allow-empty"}, commits...)...); err != nil {
			if cerr := g.ConflictError(dir); cerr != nil {
				_, _ = g.Out(dir, "cherry-pick", "--abort") //nolint:errcheck // the throwaway is removed next anyway
				return "", cerr
			}
			return "", err
		}
	}
	return g.Rev(dir, "HEAD")
}

// Integrated is true when every source commit's change is on the target: replayed commits have
// new shas, so the answer compares patches, not ancestry.
func (Integrator) Integrated(g session.Git, repo, source, target string) (bool, error) {
	commits, err := pending(g, repo, source, target)
	return len(commits) == 0, err
}

// pending lists the source commits, oldest first, whose patch the target does not contain.
func pending(g session.Git, dir, source, target string) ([]string, error) {
	out, err := g.Out(dir, "rev-list", "--reverse", "--no-merges", "--right-only", "--cherry-pick",
		target+"..."+source)
	if err != nil || out == "" {
		return nil, err
	}
	return strings.Fields(out), nil
}
