package cli

import (
	"cmp"
	"fmt"
	"os"
	"path/filepath"

	"github.com/enea-scaccabarozzi/ness"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

// baseLock is a base branch's lock as read at one tip.
type baseLock struct {
	sha  string
	lock *nsync.Lock
}

// staleLock compares the session's worktree lock with its base branch's (R-UPD-070, R-UPD-072):
// true when the worktree's ness version is older, with the notes between the two versions from
// this binary's CHANGELOG and the skills whose files changed. sess.Base is the configured base
// for a root and the parent's branch for a child, so an update cascades down the tree. cache holds
// the base lock per base name, read again only when the base's tip moves.
func staleLock(d Deps, primary string, sess store.Session, cache map[string]baseLock) (worker.Stale, bool, error) {
	if sess.Worktree == "" { // a root in the base checkout cannot lag its own base
		return worker.Stale{}, false, nil
	}
	mine, err := worktreeLock(primary, sess)
	if err != nil {
		return worker.Stale{}, false, err
	}
	base, err := readBaseLock(d, primary, sess.Base, cache)
	if err != nil {
		return worker.Stale{}, false, err
	}
	c, err := nsync.Compare(mine.Ness, base.Ness)
	if err != nil || c >= 0 {
		return worker.Stale{}, false, err
	}
	// A base on a newer ness than this binary waits for the worker's restart on it: this
	// CHANGELOG lacks the newest notes, and their later arrival would change the body and send the
	// pair a second item.
	if ahead, cerr := nsync.Compare(base.Ness, d.Version); cerr == nil && ahead > 0 {
		return worker.Stale{}, false, nil
	}
	// A version without Running sessions refuses sync (R-SYNC-089), not the update item: its
	// notes read as empty here.
	all, _ := nsync.Notes(cmp.Or(d.Notes, ness.Changelog)) //nolint:errcheck // see above
	between, err := nsync.Between(all, mine.Ness, base.Ness)
	if err != nil {
		return worker.Stale{}, false, err
	}
	st := worker.Stale{From: mine.Ness, To: base.Ness, Base: sess.Base, Skills: nsync.ChangedSkills(mine, base)}
	for _, n := range between {
		st.Notes = append(st.Notes, "ness "+n.Version+": "+nsync.RunningSessions(n))
	}
	return st, true, nil
}

// worktreeLock reads the session's worktree lock.
func worktreeLock(primary string, sess store.Session) (*nsync.Lock, error) {
	data, err := os.ReadFile(filepath.Join(primary, sess.Worktree, nsync.LockPath)) //nolint:gosec // its worktree
	if err != nil {
		return nil, err
	}
	return nsync.ParseLock(data)
}

// readBaseLock reads the lock on base's tip, from cache while the tip has not moved.
func readBaseLock(d Deps, primary, base string, cache map[string]baseLock) (*nsync.Lock, error) {
	g := session.Git{Exec: d.exec()}
	// ponytail: one rev-parse per idle session per pass; cache per base per pass if session counts grow.
	sha, err := g.Rev(primary, base)
	if err != nil {
		return nil, fmt.Errorf("base %s: %w", base, err)
	}
	if b, ok := cache[base]; ok && b.sha == sha {
		return b.lock, nil
	}
	out, err := g.Out(primary, "show", sha+":"+nsync.LockPath)
	if err != nil {
		return nil, fmt.Errorf("%s on %s: %w", nsync.LockPath, base, err)
	}
	l, err := nsync.ParseLock([]byte(out))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", base, err)
	}
	cache[base] = baseLock{sha: sha, lock: l}
	return l, nil
}

// sayBehindBase prints the session-start line of a worktree behind its base (R-UPD-072); a session
// whose locks cannot be read gets none: the worker logs why.
func sayBehindBase(d Deps, p *project, sess store.Session) {
	primary, err := primaryRoot(p)
	if err != nil {
		return
	}
	st, stale, err := staleLock(d, primary, sess, map[string]baseLock{})
	if err != nil || !stale {
		return
	}
	say(d.Stdout, "ness: this worktree's ness files are %s and %s has %s: merge %s, run ness sync --check "+
		"(expect exit 0), re-load the changed skills (your inbox has the list)", st.From, st.Base, st.To, st.Base)
}
