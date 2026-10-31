package cli

import (
	"path/filepath"
	"strings"

	pcheck "github.com/enea-scaccabarozzi/ness/internal/check"
	"github.com/enea-scaccabarozzi/ness/internal/profile"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// topRoot is the top ancestor of sess, itself for a root; empty for the operator (R-CHK-004).
func topRoot(s *store.Store, sess store.Session) (string, error) {
	for sess.Parent != store.Operator {
		var err error
		if sess, err = s.Session(sess.Parent); err != nil {
			return "", err
		}
	}
	return sess.ID, nil
}

// checkoutOf is the session's worktree below primary, or primary when it has none.
func checkoutOf(primary string, sess store.Session) string {
	if sess.Worktree == "" {
		return primary
	}
	return filepath.Join(primary, sess.Worktree)
}

// refuseOnChecks runs the profile-declared checks of ctx's event for the acting caller and refuses
// with every failure, before anything is written (R-CHK-001, R-CHK-009). ctx.Event, Child,
// Worktree, Scope, Branch, Into and Record are the caller's to fill; the rest is set here.
func refuseOnChecks(
	d Deps, s *store.Store, c caller, checks []profile.Check, primary string, ctx pcheck.Context,
) error {
	ctx.Primary = primary
	if !c.operator() {
		root, err := topRoot(s, c.sess)
		if err != nil {
			return err
		}
		ctx.Session, ctx.Root = c.id, root
		if ctx.Size == "" {
			top, terr := s.Session(root)
			if terr != nil {
				return terr
			}
			ctx.Size = top.Size
		}
	}
	fails := pcheck.Run(ctx, checks, pcheck.Deps{Git: session.Git{Exec: d.exec()}, Facts: s, Shell: d.shell()})
	if len(fails) == 0 {
		return nil
	}
	return refuse("%s", strings.TrimSuffix(pcheck.Refusal(ctx.Event, fails), "\n"))
}
