package session

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
)

// Git runs git through the Exec seam, always in a given directory.
type Git struct{ Exec Exec }

// Out runs git in dir and returns trimmed stdout.
func (g Git) Out(dir string, args ...string) (string, error) {
	out, err := g.Exec(dir, "git", args...)
	return strings.TrimSpace(string(out)), err
}

// Rev resolves ref to a sha.
func (g Git) Rev(dir, ref string) (string, error) {
	return g.Out(dir, "rev-parse", "--verify", ref+"^{commit}")
}

// CommonDir is the absolute git common dir of dir.
func (g Git) CommonDir(dir string) (string, error) {
	return g.Out(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
}

// DirtyFiles lists the porcelain lines of uncommitted changes in dir, untracked files included
// (R-SESS-103).
func (g Git) DirtyFiles(dir string) ([]string, error) {
	out, err := g.Out(dir, "status", "--porcelain", "--ignore-submodules=none")
	if out == "" {
		return nil, err
	}
	return strings.Split(out, "\n"), err
}

// DirtyTracked is Dirty without untracked files: what a fast-forward of dir's branch can collide
// with (R-SESS-104). A manager's untracked brief is no obstacle.
func (g Git) DirtyTracked(dir string) (bool, error) {
	out, err := g.Out(dir, "status", "--porcelain", "--untracked-files=no")
	return out != "", err
}

// IsAncestor reports whether ancestor is reachable from ref; exit status 1 is "no", not an error.
func (g Git) IsAncestor(dir, ancestor, ref string) (bool, error) {
	_, err := g.Out(dir, "merge-base", "--is-ancestor", ancestor, ref)
	var ee *exec.ExitError
	if errors.As(err, &ee) && ee.ExitCode() == 1 {
		return false, nil
	}
	return err == nil, err
}

// Worktree is one entry of git worktree list.
type Worktree struct{ Path, Branch string }

// Worktrees lists the checkouts of the repository containing dir.
func (g Git) Worktrees(dir string) ([]Worktree, error) {
	out, err := g.Out(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var list []Worktree
	var cur Worktree
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur = Worktree{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "branch refs/heads/"):
			cur.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "":
			if cur.Path != "" {
				list = append(list, cur)
			}
			cur = Worktree{}
		}
	}
	if cur.Path != "" {
		list = append(list, cur)
	}
	return list, nil
}

// CheckedOut returns the worktree where branch is checked out, or "".
func (g Git) CheckedOut(dir, branch string) (string, error) {
	wts, err := g.Worktrees(dir)
	if err != nil {
		return "", err
	}
	for _, w := range wts {
		if w.Branch == branch {
			return w.Path, nil
		}
	}
	return "", nil
}

// ConflictError turns a failed merge in dir into ErrConflict naming the unmerged paths. Git prints
// CONFLICT on stdout, which Exec does not return, so the unmerged-path list is the signal (A1).
// It returns nil when nothing is unmerged: the failure was something else and the caller keeps it.
func (g Git) ConflictError(dir string) error {
	out, err := g.Out(dir, "diff", "--name-only", "--diff-filter=U")
	if err != nil || out == "" {
		return nil //nolint:nilerr // no readable conflict list: the caller reports the original error
	}
	return fmt.Errorf("%w: %s", ErrConflict, strings.ReplaceAll(out, "\n", ", "))
}

// lockWorktrees serializes every git call that prunes, adds or removes worktrees of one
// repository: git does not lock .git/worktrees, so one caller's prune can delete another's
// half-made entry and two adds can collide (seen with 20 parallel real launches).
// Callers are launch, close, and integrate's throwaway add, remove and sweep. The lock blocks;
// the file stays, as unlinking a lock file races with the next opener.
//
// Lock order: the resume-ID lock or the integrate target lock first, this one last. Hold it only
// around those git calls, never across a gate, a hook, or a wait on anything that takes it.
func lockWorktrees(g Git, repo string) (func(), error) {
	common, err := g.CommonDir(repo)
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(common, "ness")
	if merr := os.MkdirAll(dir, 0o750); merr != nil {
		return nil, merr
	}
	lock := filepath.Join(dir, "worktree.lock")
	f, err := os.OpenFile(lock, os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // a fixed name under git's dir
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil { //nolint:gosec // an fd fits an int
		return nil, errors.Join(err, f.Close())
	}
	return func() { _ = f.Close() }, nil //nolint:errcheck // closing drops the lock; the file stays
}
