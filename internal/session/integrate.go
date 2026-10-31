package session

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

var (
	// ErrConflict is returned by a fold that conflicts; the message names the paths (R-SESS-097).
	ErrConflict = errors.New("the fold conflicts")
	// ErrGate is returned when the gate command fails (R-SESS-092).
	ErrGate = errors.New("the gate failed")
	// ErrNoGate is returned when the gate is empty: an empty gate would pass everything (R-SESS-105).
	ErrNoGate = errors.New("the gate is empty")
	// ErrTargetMoved is returned when the target moved while the gate ran and the fold cannot land
	// as gated: a re-fold conflicts, or the target moved outside the records twice (R-SESS-165).
	ErrTargetMoved = errors.New("moved while the gate ran")
	// ErrDirtyTarget is returned when the target's checkout has uncommitted tracked changes (R-SESS-104).
	ErrDirtyTarget = errors.New("the target checkout has uncommitted changes")
	// ErrNothingToIntegrate is returned when the source is already integrated into the target.
	ErrNothingToIntegrate = errors.New("nothing to integrate")
)

// Integration names a fold: source and target refs and, once folded, the recorded ref (the new
// target sha, or the PR url for the pr integrator).
type Integration struct{ Source, Target, Ref string }

// Integrator is the integrator seam (design §2): merge, squash, rebase, pr. Fold runs in dir, a
// throwaway worktree checked out at target, and returns the ref to record; it must leave the
// real target untouched. Integrated answers "integrated?" for a pair (R-SESS-102).
type Integrator interface {
	Fold(g Git, dir, source, target string) (ref string, err error)
	Integrated(g Git, repo, source, target string) (bool, error)
}

// Publisher is an Integrator whose result lives outside the local repository (pr). Integrate calls
// Publish only after the gate passed, so a failing gate pushes and opens nothing (R-SESS-092); the
// ref it returns is recorded and the local target never moves.
type Publisher interface {
	Publish(g Git, repo, source, target string) (ref string, err error)
}

// Forge is what the pr integrator needs from GitHub: find the open pull request of a pair ("" when
// none), open one, and tell whether one is merged (R-SESS-096).
type Forge interface {
	FindPR(repo, source, target string) (url string, err error)
	OpenPR(repo, source, target, title string) (url string, err error)
	Merged(repo, url string) (bool, error)
}

// IntegrateSpec is one ness integrate.
type IntegrateSpec struct {
	Repo, Common, TmpName string // the checkout to run git in; the common dir; the throwaway name prefix
	LockDir               string // where the per-target lock lives (the store dir); empty means <Common>/ness
	Gate                  string
	// Setup is an optional shell command run in the throwaway, inside the gate lock, right before
	// the gate (R-SESS-146).
	Setup       string
	Integration Integration
	Push        bool
	Log         io.Writer // the gate's output; nil discards it
	// Holder names who integrates, written into the target lock for a waiter to read (R-SESS-164).
	Holder string
	// Records is the repo-relative record directory: a target move touching only it keeps the gate
	// (R-SESS-165). Empty means no move is record-only.
	Records string
	// SSHSet is whether the environment sets GIT_SSH or GIT_SSH_COMMAND: the push then adds no
	// keepalive core.sshCommand, which would beat a GIT_SSH wrapper (R-SESS-166).
	SSHSet bool
}

// Result is what integrate records.
type Result struct {
	Ref      string // new target sha, or PR url
	MovedRef bool   // whether the local target moved (false for pr)
	// Published is set when a Publisher opened the request: Ref is then its url.
	Published bool
}

// Integrate folds source into target: throwaway worktree at target, Fold, gate, then the target
// moves (fast-forward where it is checked out, update-ref otherwise), then push if asked
// (R-SESS-091..098, R-SESS-104). The throwaway is removed on every path (R-SESS-093).
func Integrate(g Git, it Integrator, s IntegrateSpec) (res Result, err error) {
	unlock, err := guard(s)
	if err != nil {
		return res, err
	}
	defer unlock()
	checkout, before, err := prepare(g, it, s)
	if err != nil {
		return res, err
	}
	if s.Push {
		if before, err = catchUp(g, s, checkout, before); err != nil {
			return res, err
		}
	}
	sweepLeftovers(g, s)
	tmp, err := throwaway(s)
	if err != nil {
		return res, err
	}
	if werr := worktreeOp(g, s, "add", "-q", "--detach", tmp, s.Integration.Target); werr != nil {
		return res, werr
	}
	defer func() {
		if rerr := worktreeOp(g, s, "remove", "--force", tmp); rerr != nil {
			err = errors.Join(err, rerr)
		}
		_ = os.RemoveAll(tmp) //nolint:errcheck // already removed by git on the normal path
	}()
	ref, err := it.Fold(g, tmp, s.Integration.Source, s.Integration.Target)
	if err != nil {
		return res, err
	}
	if err = runGate(g, tmp, s, before); err != nil { //nolint:gocritic // err is the named result the defer joins into
		return res, err
	}
	return land(g, it, s, gated{tmp: tmp, checkout: checkout, ref: ref, base: before})
}

// gated is a fold the gate passed: the throwaway it lives in, where the target is checked out
// ("" when nowhere), the folded ref and the target's sha it was folded on; once the target moved,
// the fold the gate last passed and whether the gate already ran twice.
type gated struct {
	tmp, checkout, ref, base, passed string
	regated                          bool
}

// land is what happens once the gate passed. A Publisher (pr) pushes the source and opens the
// request now, and the local target stays; any other integrator advances the target.
func land(g Git, it Integrator, s IntegrateSpec, f gated) (Result, error) {
	if p, ok := it.(Publisher); ok {
		url, err := p.Publish(g, s.Repo, s.Integration.Source, s.Integration.Target)
		return Result{Ref: url, Published: true}, err
	}
	res := Result{Ref: f.ref}
	if _, perr := g.Rev(s.Repo, f.ref); perr != nil {
		return res, nil //nolint:nilerr // not a commit: the target stays
	}
	ref, err := settle(g, it, s, f)
	res.Ref = ref
	if err != nil {
		return res, err
	}
	// The target moved: even if the push fails the caller must record the ref (a retry then
	// finds nothing to integrate).
	res.MovedRef = true
	if s.Push {
		return res, push(g, s)
	}
	return res, nil
}

// settleTries bounds how often a target that keeps moving is re-read before giving up.
const settleTries = 5

// settle moves the target to the gated fold. A target that moved while the gate ran is folded
// onto again (onto); a move racing the fast-forward re-enters.
func settle(g Git, it Integrator, s IntegrateSpec, f gated) (string, error) {
	target := s.Integration.Target
	f.passed = f.ref
	for range settleTries {
		now, err := reread(g, s, f.checkout)
		if err != nil {
			return f.ref, err
		}
		if now != f.base {
			if oerr := f.onto(g, it, s, now); oerr != nil {
				return f.ref, oerr
			}
		}
		merr := move(g, s, f.checkout, f.ref, f.base)
		if merr == nil {
			return f.ref, nil
		}
		if again, rerr := g.Rev(s.Repo, target); rerr != nil || again == f.base {
			logf(s, "ness: moving %s failed: %v\n", target, merr)
			return f.ref, fmt.Errorf("%s could not be moved to the folded commit %s; "+
				"see the log above, then integrate again", target, f.ref)
		}
	}
	return f.ref, fmt.Errorf("%s %w, and kept moving; integrate again", target, ErrTargetMoved)
}

// onto folds the source again onto the moved target now. The new fold keeps the gate when it
// differs from the fold the gate last passed only in the records; otherwise the gate runs once
// more, and a second such move is ErrTargetMoved (R-SESS-165).
func (f *gated) onto(g Git, it Integrator, s IntegrateSpec, now string) error {
	target := s.Integration.Target
	ref, err := refold(g, it, s, f.tmp, now)
	if err != nil {
		return err
	}
	f.ref = ref
	// --no-renames: a code file renamed into the records must list its old path too.
	changed, err := g.Out(f.tmp, "diff", "--no-renames", "--name-only", f.passed, ref)
	switch {
	case err != nil:
		return err
	case recordsOnly(changed, s.Records):
		logf(s, "ness: %s moved during the gate by session records only; landing without a second gate\n",
			target)
	case f.regated:
		what, _ := g.Out(s.Repo, "log", "-1", "--format=%h %s", now) //nolint:errcheck // only for the message
		return fmt.Errorf("%s %w twice, outside the session records (now at %s); integrate again",
			target, ErrTargetMoved, what)
	default:
		logf(s, "ness: %s moved during the gate; running the gate again on the new %s\n", target, target)
		if gerr := runGate(g, f.tmp, s, now); gerr != nil {
			return gerr
		}
		f.passed, f.regated = ref, true
	}
	f.base = now
	return nil
}

// reread is the target's sha now, caught up with origin first when the integrate pushes.
func reread(g Git, s IntegrateSpec, checkout string) (string, error) {
	now, err := g.Rev(s.Repo, s.Integration.Target)
	if err != nil || !s.Push {
		return now, err
	}
	return catchUp(g, s, checkout, now)
}

// refold resets the throwaway to the moved target and folds the source again. A conflict is
// ErrTargetMoved naming the paths; a source the target now holds stays ErrNothingToIntegrate, so
// the target is left alone and the caller says the work is already in.
func refold(g Git, it Integrator, s IntegrateSpec, tmp, now string) (string, error) {
	in := s.Integration
	if _, err := g.Out(tmp, "reset", "-q", "--hard", now); err != nil {
		return "", err
	}
	ref, err := it.Fold(g, tmp, in.Source, in.Target)
	if errors.Is(err, ErrConflict) {
		paths := strings.TrimPrefix(err.Error(), ErrConflict.Error()+": ")
		return "", fmt.Errorf("%s %w and the fold onto it conflicts in %s; "+
			"resolve that in %s, then integrate again", in.Target, ErrTargetMoved, paths, in.Source)
	}
	return ref, err
}

// recordsOnly reports whether every path of a git name list lies under the records directory.
// An empty list (the same tree) counts: there is nothing new to gate.
func recordsOnly(paths, records string) bool {
	dir := strings.Trim(filepath.ToSlash(records), "/") + "/"
	for p := range strings.FieldsSeq(paths) {
		if dir == "/" || !strings.HasPrefix(p, dir) {
			return false
		}
	}
	return true
}

// keepalive keeps an ssh push alive through a long pre-push hook (R-SESS-166).
const keepalive = "core.sshCommand=ssh -o ServerAliveInterval=30 -o ServerAliveCountMax=10"

// dropped matches the git and ssh lines of a dropped connection, at the start of a stderr line
// (or right after the exec error's ": "), so a failing pre-push hook that merely prints such words
// mid-line is not re-run. A hook line starting with them still matches.
var dropped = regexp.MustCompile(`(?m)(?:^|: )(?:Connection to \S+ closed by remote host|client_loop: |` +
	`packet_write_wait: |Read from remote host |fatal: the remote end hung up|fatal: early EOF|` +
	`send-pack: unexpected disconnect)`)

// push pushes the target to origin, with ssh keepalives unless the project set its own ssh command
// (core.sshCommand, GIT_SSH, GIT_SSH_COMMAND), and retries once a push whose connection dropped
// (R-SESS-166).
func push(g Git, s IntegrateSpec) error {
	args := []string{"push", "origin", s.Integration.Target}
	//nolint:errcheck // unset exits 1: the project has no command of its own
	if own, _ := g.Out(s.Repo, "config", "--get", "core.sshCommand"); own == "" && !s.SSHSet {
		args = append([]string{"-c", keepalive}, args...)
	}
	_, err := g.Out(s.Repo, args...)
	if err != nil && dropped.MatchString(err.Error()) {
		logf(s, "ness: the push lost its connection; retrying once\n")
		_, err = g.Out(s.Repo, args...)
	}
	return err
}

// logf writes one advisory line to the log, if any.
func logf(s IntegrateSpec, format string, args ...any) {
	if s.Log != nil {
		_, _ = fmt.Fprintf(s.Log, format, args...) //nolint:errcheck // advisory
	}
}

// catchUp fetches the target from origin before a pushing integrate and fast-forwards the local
// target when origin is ahead, so the fold lands on what others integrated and the push is a
// fast-forward (R-SESS-119). A local target that diverged from origin is refused; a failed fetch
// is logged and the fold goes on.
func catchUp(g Git, s IntegrateSpec, checkout, before string) (string, error) {
	target := s.Integration.Target
	if _, err := g.Out(s.Repo, "fetch", "-q", "origin", target); err != nil {
		// Offline or no such remote: fold on the local target; the push then decides (R-SESS-098).
		logf(s, "ness: fetching origin %s failed, folding on the local %s: %v\n", target, target, err)
		return before, nil
	}
	remote, err := g.Rev(s.Repo, "refs/remotes/origin/"+target)
	if err != nil || remote == before {
		return before, err
	}
	behind, err := g.IsAncestor(s.Repo, before, remote)
	if err != nil {
		return before, err
	}
	if !behind {
		ahead, aerr := g.IsAncestor(s.Repo, remote, before)
		if aerr != nil || ahead {
			return before, aerr
		}
		return before, fmt.Errorf("%s and origin/%s have diverged; reconcile them, then integrate again",
			target, target)
	}
	if merr := move(g, s, checkout, remote, before); merr != nil {
		return before, merr
	}
	return remote, nil
}

// prepare refuses what cannot be integrated and returns where the target is checked out ("" when
// nowhere) and its current sha.
func prepare(g Git, it Integrator, s IntegrateSpec) (checkout, before string, err error) {
	in := s.Integration
	if checkout, err = g.CheckedOut(s.Repo, in.Target); err != nil {
		return "", "", err
	}
	if checkout != "" {
		dirty, derr := g.DirtyTracked(checkout)
		if derr != nil {
			return "", "", derr
		}
		if dirty {
			return "", "", fmt.Errorf("%w: %s is checked out in %s; commit or stash them first",
				ErrDirtyTarget, in.Target, checkout)
		}
	}
	if err = nothingToIntegrate(g, it, s); err != nil { //nolint:gocritic // err is the named result
		return "", "", err
	}
	before, err = g.Rev(s.Repo, in.Target)
	return checkout, before, err
}

// nothingToIntegrate refuses a source that is already in the target: by ancestry for every
// integrator (a pr would otherwise push and open an empty request), then by the integrator's own
// answer (a merged PR).
func nothingToIntegrate(g Git, it Integrator, s IntegrateSpec) error {
	in := s.Integration
	done, err := g.IsAncestor(s.Repo, in.Source, in.Target)
	if err == nil && !done {
		done, err = it.Integrated(g, s.Repo, in.Source, in.Target)
	}
	if err != nil {
		return err
	}
	if done {
		return fmt.Errorf("%w: %s is already in %s", ErrNothingToIntegrate, in.Source, in.Target)
	}
	return nil
}

// runGate runs the configured gate in the throwaway, never in the session's checkout (R-SESS-091);
// its output goes to the log so a failure explains itself. Setup and gate see NESS_BASE, the
// target's sha before the fold, so a project can scope its checks to the diff (R-SESS-156).
func runGate(g Git, tmp string, s IntegrateSpec, base string) error {
	log := s.Log
	if log == nil {
		log = io.Discard
	}
	unlock, err := lockGate(s, log)
	if err != nil {
		return err
	}
	defer unlock()
	if strings.TrimSpace(s.Setup) != "" {
		if serr := runIn(g, tmp, base, s.Setup, log, "setup: "); serr != nil {
			return serr
		}
	}
	return runIn(g, tmp, base, s.Gate, log, "")
}

var hashRe = regexp.MustCompile(`^[0-9a-f]{40,64}$`)

// runIn runs one shell command in the throwaway, logs its output, and wraps a failure as ErrGate.
func runIn(g Git, tmp, base, command string, log io.Writer, what string) error {
	if !hashRe.MatchString(base) {
		return fmt.Errorf("fold base %q is not a commit hash", base) // it is quoted into a shell line
	}
	out, err := g.Exec(tmp, "sh", "-c", "export NESS_BASE='"+base+"'\n"+command)
	_, _ = log.Write(out) //nolint:errcheck // a broken log writer must not decide the gate
	if err != nil {
		_, _ = fmt.Fprintln(log, err) //nolint:errcheck // same
		return fmt.Errorf("%w: %s%w", ErrGate, what, err)
	}
	return nil
}

// move advances the target to the folded commit (R-SESS-092).
func move(g Git, s IntegrateSpec, checkout, folded, before string) error {
	var err error
	if checkout != "" {
		_, err = g.Out(checkout, "merge", "--ff-only", folded)
	} else {
		_, err = g.Out(s.Repo, "update-ref", "refs/heads/"+s.Integration.Target, folded, before)
	}
	return err
}

var unsafeRef = regexp.MustCompile(`[^A-Za-z0-9._-]`)

// lockTarget takes the per-target lock, waiting while another integrate into the target holds it:
// two integrates into one target would race on the ref (R-SESS-164). The waiter logs the holder
// the lock file names; the new holder then writes its own name. The lock is released by the
// returned func or when the process ends.
func lockTarget(s IntegrateSpec) (func(), error) {
	target := s.Integration.Target
	f, err := lockFile(s, "integrate-"+unsafeRef.ReplaceAllString(target, "_")+".lock")
	if err != nil {
		return nil, err
	}
	fd := int(f.Fd()) //nolint:gosec // an fd fits an int
	if err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		holder := "another integrate"
		b, _ := io.ReadAll(f) //nolint:errcheck // advisory: an unreadable holder stays unnamed
		if name := strings.TrimSpace(string(b)); name != "" {
			holder = name + "'s integrate"
		}
		logf(s, "ness: waiting for %s into %s\n", holder, target)
		err = syscall.Flock(fd, syscall.LOCK_EX)
	}
	if err != nil {
		return nil, errors.Join(err, f.Close())
	}
	if f.Truncate(0) == nil { // advisory: a waiter only names who it waits for
		_, _ = f.WriteAt([]byte(s.Holder), 0) //nolint:errcheck // same
	}
	// The file is never removed: unlinking a lock file races with a second ness that opened it.
	return func() { _ = f.Close() }, nil //nolint:errcheck // closing drops the lock; the file stays
}

// lockGate takes the project's gate lock, waiting (and saying so in the log) while another
// integrate's gate runs: parallel gates on one machine thrash each other into timeouts
// (R-SESS-145). Taken after the target lock, held only around the gate.
func lockGate(s IntegrateSpec, log io.Writer) (func(), error) {
	f, err := lockFile(s, "gate.lock")
	if err != nil {
		return nil, err
	}
	fd := int(f.Fd()) //nolint:gosec // an fd fits an int
	if err = syscall.Flock(fd, syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		//nolint:errcheck // advisory
		_, _ = fmt.Fprintln(log, "ness: another gate is running in this project; waiting for it")
		err = syscall.Flock(fd, syscall.LOCK_EX)
	}
	if err != nil {
		return nil, errors.Join(err, f.Close())
	}
	return func() { _ = f.Close() }, nil //nolint:errcheck // closing drops the lock; the file stays
}

// lockFile opens (creating) a lock file in the lock dir; the file is never removed.
func lockFile(s IntegrateSpec, name string) (*os.File, error) {
	dir := s.LockDir
	if dir == "" {
		dir = filepath.Join(s.Common, "ness")
	}
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return nil, err
	}
	return os.OpenFile(filepath.Join(dir, name), os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // sanitized name
}

// throwaway is the path of a fresh worktree: the name prefix plus a random suffix, so it cannot
// collide with another live run (R-SESS-093).
func throwaway(s IntegrateSpec) (string, error) {
	var b [4]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return filepath.Join(s.Common, "ness", s.TmpName+"-"+hex.EncodeToString(b[:])), nil
}

// worktreeOp runs one git worktree subcommand under the worktree lock.
func worktreeOp(g Git, s IntegrateSpec, args ...string) error {
	unlock, err := lockWorktrees(g, s.Repo)
	if err != nil {
		return err
	}
	defer unlock()
	_, err = g.Out(s.Repo, append([]string{"worktree"}, args...)...)
	return err
}

// guard refuses an empty gate, then takes the target's lock.
func guard(s IntegrateSpec) (func(), error) {
	if strings.TrimSpace(s.Gate) == "" {
		return nil, ErrNoGate
	}
	return lockTarget(s)
}

// sweepLeftovers removes the throwaways a killed run left behind. The target's lock is held, so
// none of them can belong to a live run.
func sweepLeftovers(g Git, s IntegrateSpec) {
	unlock, err := lockWorktrees(g, s.Repo)
	if err != nil {
		return // best effort, like the sweep itself
	}
	defer unlock()
	//nolint:errcheck // only a bad pattern errs
	// -???????? is the 8-hex suffix: a sibling prefix is not ours.
	old, _ := filepath.Glob(filepath.Join(s.Common, "ness", s.TmpName+"-????????"))
	for _, p := range old {
		_, _ = g.Out(s.Repo, "worktree", "remove", "--force", p) //nolint:errcheck // best effort; prune follows
		_ = os.RemoveAll(p)                                      //nolint:errcheck // same
	}
	_, _ = g.Out(s.Repo, "worktree", "prune") //nolint:errcheck // a failed prune only leaves a stale entry
}
