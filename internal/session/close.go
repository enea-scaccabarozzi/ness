package session

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/rule"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// CloseSpec is one ness close.
type CloseSpec struct {
	Sess    store.Session
	Primary string
	Discard bool
	Keep    string
	Merged  func(url string) (bool, error) // nil unless the pr integrator is configured
	// Withdraw excuses the open items By sent the session: close withdraws them (R-SESS-130).
	Withdraw bool
	By       string
}

// Checkout is where the session's files live: its worktree, or the primary checkout for an
// adopted root.
func (c CloseSpec) Checkout() string {
	if c.Sess.Worktree == "" {
		return c.Primary
	}
	return filepath.Join(c.Primary, c.Sess.Worktree)
}

// NeverRan reports a session that never wrote a ready mark in any run (R-SESS-066): a relaunch
// clears ReadyAt, but FirstReadyAt stays, so a session that was ready once is not exempt even when
// its --resume stalls before Run is bumped.
func (c CloseSpec) NeverRan() bool {
	return c.Sess.FirstReadyAt.IsZero() && c.Sess.Run <= 1 && !c.Sess.Adopted
}

// CheckClose returns the first refusal: report missing (R-SESS-061), open children
// (R-SESS-062), open obligations (R-SESS-063), no integration without --discard unless the branch
// is contained in its base (R-SESS-064, R-SESS-167), an unmerged PR (R-SESS-071). A never-ran
// session skips the report and integration checks.
func CheckClose(s *store.Store, g Git, c CloseSpec) error {
	if err := c.checkReport(g); err != nil {
		return err
	}
	if err := closeBlockers(s, c); err != nil {
		return err
	}
	if c.NeverRan() || c.Discard {
		return nil
	}
	return c.checkIntegration(g)
}

// checkout reports whether the session's checkout exists. Only "does not exist" counts as gone:
// any other stat error is returned, never read as an absent worktree.
func (c CloseSpec) checkout() (bool, error) {
	_, err := os.Stat(c.Checkout())
	switch {
	case err == nil:
		return true, nil
	case errors.Is(err, fs.ErrNotExist):
		return false, nil
	}
	return false, fmt.Errorf("cannot inspect %s: %w", c.Checkout(), err)
}

// checkReport wants report.md in the session's checkout, for --discard too (R-SESS-061). A
// checkout deleted by hand has no file to read, so the report must be committed on the session's
// branch instead.
func (c CloseSpec) checkReport(g Git) error {
	if c.NeverRan() {
		return nil
	}
	here, err := c.checkout()
	if err != nil {
		return err
	}
	if !here {
		text, gerr := g.Exec(c.Primary, "git", "show", c.Sess.Branch+":"+c.Sess.Record+"/report.md")
		if gerr != nil {
			return fmt.Errorf("worktree gone and no report.md committed on branch %s; "+
				"commit one there (a report is required even with --discard)%s",
				c.Sess.Branch, rule.Ref(rule.ReportMissing))
		}
		return lessonsIn(string(text))
	}
	report := filepath.Join(c.Checkout(), c.Sess.Record, "report.md")
	text, err := os.ReadFile(report) //nolint:gosec // report is under the session checkout
	if err != nil {
		return fmt.Errorf("no report at %s; write it, then close%s", report, rule.Ref(rule.ReportMissing))
	}
	return lessonsIn(string(text))
}

// lessonsIn refuses a report without a `## Lessons` heading; "None." under it is enough
// (R-SESS-153).
func lessonsIn(report string) error {
	if !store.HasLessons(report) {
		return fmt.Errorf("report.md has no ## Lessons heading; add one, with \"None.\" if there is nothing%s",
			rule.Ref(rule.Lessons))
	}
	return nil
}

// Destructive reports whether Teardown will delete work: a worktree removed without --keep-work
// or --discard. Such a close first proves nothing is lost (CheckLanded).
func (c CloseSpec) Destructive() bool {
	return c.Sess.Worktree != "" && c.Keep == "" && !c.Discard && !c.NeverRan()
}

// CheckLanded refuses a destructive close while the branch holds work the integration does not:
// commits made after ness integrate (the recorded ref is only a stamp), commits on a detached HEAD,
// or uncommitted files other than the ness-written brief. It runs after the agent's tab is closed
// (so nothing can change under it) and before the worktree or branch is touched.
func CheckLanded(g Git, it Integrator, c CloseSpec) error {
	here, err := c.checkout()
	if err != nil {
		return err
	}
	if here {
		if herr := c.checkHead(g); herr != nil {
			return herr
		}
		if cerr := c.checkClean(g); cerr != nil {
			return cerr
		}
		if serr := c.checkSubmodules(g); serr != nil {
			return serr
		}
	}
	done, err := c.landed(g, it)
	if err != nil {
		return err
	}
	if !done {
		return fmt.Errorf("branch %s has commits not integrated into %s; run ness integrate again, "+
			"or close with --discard", c.Sess.Branch, c.Sess.Base)
	}
	return nil
}

// landed asks the integrator, and only with no integration recorded the containment of R-SESS-167:
// a recorded integration keeps the integrator's answer, so a commit reverting part of the work
// after it is not "contained" against the fork point.
func (c CloseSpec) landed(g Git, it Integrator) (bool, error) {
	done, err := it.Integrated(g, c.Primary, c.Sess.Branch, c.Sess.Base)
	if err == nil && !done && c.Sess.Integrated == "" {
		return c.Contained(g)
	}
	return done, err
}

// checkHead wants the worktree on the session's branch, or its HEAD reachable from that branch:
// commits on a detached HEAD have no ref and would vanish with the worktree.
func (c CloseSpec) checkHead(g Git) error {
	head, _ := g.Out(c.Checkout(), "symbolic-ref", "-q", "HEAD") //nolint:errcheck // detached HEAD exits 1
	if head == "refs/heads/"+c.Sess.Branch {
		return nil
	}
	ok, err := g.IsAncestor(c.Checkout(), "HEAD", "refs/heads/"+c.Sess.Branch)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("the worktree of %s is not on branch %s and its HEAD holds commits outside it; "+
			"check the branch out or merge them in, or close with --discard", c.Sess.ID, c.Sess.Branch)
	}
	return nil
}

// checkClean refuses uncommitted changes, submodules included; the brief is ness's own file.
func (c CloseSpec) checkClean(g Git) error {
	files, err := g.DirtyFiles(c.Checkout())
	if err != nil {
		return err
	}
	files = slices.DeleteFunc(files, func(l string) bool { return l == "?? "+c.Sess.Record+"/brief.md" })
	if len(files) > 0 {
		return fmt.Errorf("the worktree of %s has uncommitted changes: %s; commit them and ness integrate, "+
			"or close with --keep-work REASON or --discard", c.Sess.ID, strings.Join(files, ", "))
	}
	return nil
}

// checkSubmodules refuses an initialized submodule: its commits live in the worktree's module
// store, which the removal deletes, and a clean status does not show them.
// ponytail: refuses every initialized submodule; upgrade to "every submodule commit is reachable
// from its remote" if this refusal bites.
func (c CloseSpec) checkSubmodules(g Git) error {
	out, err := g.Out(c.Checkout(), "submodule", "status")
	if err != nil {
		return err
	}
	var subs []string
	for _, l := range strings.Split(out, "\n") {
		if l != "" && l[0] != '-' {
			subs = append(subs, strings.Fields(l)[1])
		}
	}
	if len(subs) > 0 {
		return fmt.Errorf("the worktree of %s has initialized submodules (%s) whose commits are deleted with it; "+
			"check that they are pushed, then close with --discard", c.Sess.ID, strings.Join(subs, ", "))
	}
	return nil
}

// closeBlockers refuses open children, then open obligations; on --withdraw the closer's own
// items are not owed (R-SESS-130).
func closeBlockers(s *store.Store, c CloseSpec) error {
	id := c.Sess.ID
	kids, err := s.Children(id)
	if err != nil {
		return err
	}
	if len(kids) > 0 {
		ids := make([]string, 0, len(kids))
		for _, k := range kids {
			ids = append(ids, k.ID+" ("+k.Scope+")")
		}
		return fmt.Errorf("session %s has open children: %s; close them first", id, strings.Join(ids, ", "))
	}
	all, err := s.Obligations(id)
	if err != nil {
		return err
	}
	// A probe wake is not owed: close retires it (P-07).
	obl := slices.DeleteFunc(all, func(it store.Item) bool {
		return it.Probe != "" || (c.Withdraw && it.Sender == c.By)
	})
	if len(obl) > 0 {
		ids := make([]string, 0, len(obl))
		for _, it := range obl {
			ids = append(ids, it.ID+" ("+it.Intent+" from "+nameOf(it.Sender)+")")
		}
		return fmt.Errorf("session %s has open obligations: %s; reply to them, then close "+
			"(your own: --withdraw)", id, strings.Join(ids, ", "))
	}
	return nil
}

// Contained reports whether merging the branch into its base changes nothing in the base's tree:
// an ancestor, a hand-folded squash and a rebased branch all count (R-SESS-167). A conflicting
// merge (exit 1) is "no", not an error.
func (c CloseSpec) Contained(g Git) (bool, error) {
	out, err := g.Out(c.Primary, "merge-tree", "--write-tree", c.Sess.Base, c.Sess.Branch)
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	base, err := g.Out(c.Primary, "rev-parse", "--verify", c.Sess.Base+"^{tree}")
	if err != nil {
		return false, err
	}
	tree, _, _ := strings.Cut(out, "\n")
	return tree == base, nil
}

// checkIntegration wants a recorded integration, or a branch contained in its base (R-SESS-167),
// and, for a recorded PR, that it is merged. A containment check that fails is a refusal: a gone
// branch still gets the --discard route.
func (c CloseSpec) checkIntegration(g Git) error {
	if c.Sess.Integrated == "" {
		if ok, err := c.Contained(g); err == nil && ok {
			return nil
		}
		return fmt.Errorf("session %s has no integration recorded; run ness integrate, or close with --discard",
			c.Sess.ID)
	}
	if c.Merged == nil || !strings.HasPrefix(c.Sess.Integrated, "http") {
		return nil
	}
	ok, err := c.Merged(c.Sess.Integrated)
	if err != nil {
		return err
	}
	if !ok {
		return fmt.Errorf("pull request %s is not merged; merge it, then close (or --discard)", c.Sess.Integrated)
	}
	return nil
}

func nameOf(id string) string {
	if id == store.Operator {
		return "operator"
	}
	return id
}

// CloseTab closes a child's tab, which stops its agent; a tab already gone is fine. A root's tab,
// launched or adopted, stays: the conversation remains readable (R-SESS-122).
func CloseTab(rt Runtime, c CloseSpec) (closed bool, err error) {
	if c.Sess.Tab == "" || c.Sess.Adopted || c.Sess.Parent == store.Operator {
		return false, nil
	}
	if err := rt.CloseTab(c.Sess.Tab); err != nil && !errors.Is(err, ErrGone) {
		return false, err
	}
	return true, nil
}

// Teardown removes the worktree (a missing directory is pruned) (R-SESS-065, R-SESS-067). The
// tab is already closed (CloseTab). The branch goes last, after the row is closed (DeleteBranch),
// so a failure in between leaves the report findable for a retry.
func Teardown(g Git, c CloseSpec) error {
	if c.Sess.Worktree == "" || c.Keep != "" {
		return nil
	}
	return removeWorktree(g, c)
}

// DeleteBranch drops the local branch of a landed session once the row is closed (R-SESS-072).
func DeleteBranch(g Git, c CloseSpec, log io.Writer) {
	if c.Sess.Worktree == "" || c.Keep != "" {
		return
	}
	deleteBranch(g, c, log)
}

// deleteBranch drops the local branch of an integrated, not discarded session. A session that
// never ran keeps a branch that has commits beyond its start point. A failure is reported, not
// fatal, since the close itself has happened.
func deleteBranch(g Git, c CloseSpec, log io.Writer) {
	if c.Sess.Integrated == "" || c.Discard {
		return
	}
	note := func(format string, a ...any) {
		_, _ = fmt.Fprintf(log, "ness: "+format+"\n", a...) //nolint:errcheck // advisory
	}
	if c.NeverRan() {
		if ok, err := g.IsAncestor(c.Primary, "refs/heads/"+c.Sess.Branch, c.Sess.Base); err != nil || !ok {
			note("branch %s kept: it has commits beyond %s", c.Sess.Branch, c.Sess.Base)
			return
		}
	}
	if _, err := g.Out(c.Primary, "branch", "-D", c.Sess.Branch); err != nil {
		note("branch %s not deleted: %v", c.Sess.Branch, err)
	}
}

// removeWorktree force-removes the worktree if its directory exists, then prunes the record of
// one deleted by hand.
func removeWorktree(g Git, c CloseSpec) error {
	unlock, err := lockWorktrees(g, c.Primary)
	if err != nil {
		return err
	}
	defer unlock()
	wt := filepath.Join(c.Primary, c.Sess.Worktree)
	if _, serr := os.Stat(wt); serr == nil {
		if _, rerr := g.Out(c.Primary, "worktree", "remove", "--force", wt); rerr != nil {
			return rerr
		}
	}
	_, err = g.Out(c.Primary, "worktree", "prune")
	return err
}
