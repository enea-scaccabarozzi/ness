// Package merge is the integrator that folds a branch with a merge commit, preserving the
// source commits (R-SESS-094).
package merge

import "github.com/enea-scaccabarozzi/ness/internal/session"

// Integrator implements session.Integrator.
type Integrator struct{}

// Fold merges source into the throwaway's HEAD (checked out at target) with --no-ff.
func (Integrator) Fold(g session.Git, dir, source, target string) (string, error) {
	_, err := g.Out(dir, "merge", "--no-ff", "--no-edit", "-m", "integrate "+source+" into "+target, source)
	if err != nil {
		if cerr := g.ConflictError(dir); cerr != nil {
			_, _ = g.Out(dir, "merge", "--abort") //nolint:errcheck // the throwaway is removed next anyway
			return "", cerr
		}
		return "", err
	}
	return g.Rev(dir, "HEAD")
}

// Integrated is true when source is an ancestor of target.
func (Integrator) Integrated(g session.Git, repo, source, target string) (bool, error) {
	return g.IsAncestor(repo, source, target)
}
