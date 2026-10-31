package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

func newUpgradeCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Move this project to the running binary: migrate, sync, bump the pin and commit",
		Long: "Run in the base checkout, clean and on the base branch. It migrates the store, sets the pin\n" +
			"to this binary's major.minor, syncs, and commits as chore(ness): upgrade to X.Y.Z. It never pushes.",
		Args: cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error { return runUpgrade(d, upgradeOpts{restart: true}) },
	}
}

// upgradeOpts tunes runUpgrade: restart restarts (or re-points) the project's worker; the worker's
// own unattended upgrade turns it off, as a restart would kill its own cgroup.
type upgradeOpts struct{ restart bool }

// runUpgrade moves the base checkout to this binary (R-UPD-060..063). It skips the pin check
// (R-CFG-011): it is the pin fixer, so it runs on a pin two minors behind.
func runUpgrade(d Deps, o upgradeOpts) error {
	p, cfg, release, err := prepareUpgrade(d)
	if err != nil {
		return err
	}
	defer release()
	committed, err := syncAndCommit(d, p)
	if err != nil {
		return err
	}
	// A manual or the worker's upgrade ends a failed automatic one (R-UPD-081).
	if cerr := clearFailure(p); cerr != nil {
		return cerr
	}
	// Also when already at: a rerun finishing an upgrade (a resolved conflict, a commit made by
	// hand) still leaves the worker on the old code (R-UPD-062).
	if o.restart {
		if werr := freshWorker(d, cfg.Project, p.root); werr != nil {
			return werr
		}
	}
	if committed {
		say(d.Stdout, "open sessions get an update item from the worker at their next idle point")
	}
	return nil
}

// prepareUpgrade loads the project, takes the idle point and, once the lock is no newer than the
// binary, migrates the store and bumps the pin. The returned project is reloaded after the bump.
func prepareUpgrade(d Deps) (*project, *config.Config, func(), error) {
	p, err := locateProject(d)
	if err != nil {
		return nil, nil, nil, err
	}
	cfg, err := projectConfig(d, p)
	if err != nil {
		return nil, nil, nil, err
	}
	release, err := idlePoint(d, p, cfg)
	if err != nil {
		return nil, nil, nil, err
	}
	err = checkLockVersion(d, p.root)
	if err == nil {
		err = migrateStore(d, p.root)
	}
	if err == nil {
		err = bumpPin(d, p)
	}
	if err == nil {
		p, err = locateProject(d)
	}
	if err != nil {
		release()
		return nil, nil, nil, err
	}
	return p, cfg, release, nil
}

// syncAndCommit syncs and commits the tracked changes and the untracked files the sync wrote as
// the upgrade commit; any other untracked file stays out. With nothing to commit it says already
// at and reports false. A sync conflict stops it before the commit with exit 3.
func syncAndCommit(d Deps, p *project) (committed bool, err error) {
	if serr := runSync(d, p, syncOpts{}); serr != nil {
		var ee *exitError
		if errors.As(serr, &ee) && ee.code == ExitConflict {
			return false, &exitError{code: ExitConflict, err: errors.New("sync stopped on conflicts: resolve them " +
				"and commit the ness files (git status lists them), then run ness upgrade again")}
		}
		return false, serr
	}
	g := session.Git{Exec: d.exec()}
	staged, err := stageUpgrade(d, p)
	if err != nil {
		return false, err
	}
	if !staged {
		say(d.Stdout, "already at %s", d.Version)
		return false, nil
	}
	subject := "chore(ness): upgrade to " + d.Version
	if _, err = g.Out(p.root, "commit", "-q", "-m", subject); err != nil {
		// A commit-msg hook rejecting the subject is the common case: the operator commits by hand.
		return false, refuse("the upgrade is staged but git commit refused it (%s): commit the staged files "+
			"with your project's message, then run ness upgrade again", firstStderrLine(err))
	}
	say(d.Stdout, "committed %s", subject)
	return true, nil
}

// stageUpgrade stages the tracked changes and the untracked files the sync wrote, and reports
// whether anything is staged.
func stageUpgrade(d Deps, p *project) (bool, error) {
	g := session.Git{Exec: d.exec()}
	if _, err := g.Out(p.root, "add", "-u"); err != nil {
		return false, err
	}
	written, err := ownUntracked(d, p)
	if err == nil && len(written) > 0 {
		_, err = g.Out(p.root, append([]string{"add", "--"}, written...)...)
	}
	if err != nil {
		return false, err
	}
	staged, err := g.Out(p.root, "diff", "--cached", "--name-only")
	return staged != "", err
}

// firstStderrLine is the first line of the stderr SystemExec appends after the exit status, or
// the whole error when there is none.
func firstStderrLine(err error) string {
	msg := err.Error()
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		if _, rest, ok := strings.Cut(msg, ee.Error()+": "); ok && rest != "" {
			msg = rest
		}
	}
	line, _, _ := strings.Cut(msg, "\n")
	return line
}

// notIdleError is an idle point refusal: a dirty base or a lock held. The worker waits silently.
type notIdleError struct{ error }

func (e notIdleError) Unwrap() error { return e.error }

// idlePoint refuses unless p is the primary checkout, on cfg.Base, with no uncommitted change and
// no untracked file where the sync writes, and no integrate holds the gate lock or the base's
// integrate lock; it then holds both locks until release.
func idlePoint(d Deps, p *project, cfg *config.Config) (release func(), err error) {
	primary, err := primaryRoot(p)
	if err != nil {
		return nil, err
	}
	if filepath.Clean(p.root) != filepath.Clean(primary) {
		return nil, refuse("%s is a session worktree; run ness upgrade in the base checkout %s", p.root, primary)
	}
	if cerr := checkBaseClean(d, p, cfg.Base); cerr != nil {
		return nil, cerr
	}
	path, err := store.Locate(primary)
	if err != nil {
		return nil, refuse("%v", err)
	}
	dir := filepath.Dir(path)
	if merr := os.MkdirAll(dir, 0o750); merr != nil {
		return nil, merr
	}
	gate, err := holdLock(filepath.Join(dir, "gate.lock"), "an integrate holds the gate lock")
	if err != nil {
		return nil, err
	}
	target, err := holdLock(filepath.Join(dir, session.TargetLockName(cfg.Base)),
		"an integrate into "+cfg.Base+" holds its lock")
	if err != nil {
		gate()
		return nil, err
	}
	return func() { target(); gate() }, nil
}

// checkBaseClean refuses unless p (the primary checkout) has base checked out with no uncommitted
// change and no untracked file at a path the sync writes: another branch or a detached HEAD is
// clean to git status, so it is checked first. Other untracked files are not the upgrade's. Only
// the refusals are notIdleError: a git or sync plan failure is the worker's to record (R-UPD-081).
func checkBaseClean(d Deps, p *project, base string) error {
	g := session.Git{Exec: d.exec()}
	primary := p.root
	branch, _ := g.Out(primary, "symbolic-ref", "-q", "--short", "HEAD") //nolint:errcheck // exit 1 is detached
	switch {
	case branch == "":
		return notIdleError{refuse("HEAD is detached; check out %s, then run ness upgrade", base)}
	case branch != base:
		return notIdleError{refuse("HEAD is on %s, not %s; check out %s, then run ness upgrade", branch, base, base)}
	}
	dirty, err := g.Out(primary, "status", "--porcelain", "--untracked-files=no", "--ignore-submodules=none")
	if err != nil {
		return err
	}
	if dirty != "" {
		first, _, _ := strings.Cut(dirty, "\n")
		return notIdleError{refuse("uncommitted changes in %s (%s); commit or remove them, then run ness upgrade",
			primary, first)}
	}
	own, err := ownUntracked(d, p)
	if err != nil {
		return err
	}
	if len(own) > 0 {
		return notIdleError{refuse("untracked %s in %s is a path ness sync writes; commit or remove it, "+
			"then run ness upgrade", own[0], primary)}
	}
	return nil
}

// ownUntracked lists the untracked files of p at paths ness sync writes, its lock included.
func ownUntracked(d Deps, p *project) (own []string, err error) {
	out, err := session.Git{Exec: d.exec()}.Out(p.root, "ls-files", "-z", "--others", "--exclude-standard")
	if err != nil || out == "" {
		return nil, err
	}
	root, err := os.OpenRoot(p.root)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, root.Close()) }()
	plan, err := planSync(d, p, root)
	if err != nil {
		return nil, err
	}
	writes := map[string]bool{nsync.LockPath: true}
	for _, f := range plan.Files {
		writes[f.Path] = true
	}
	for _, f := range strings.Split(out, "\x00") {
		if writes[f] {
			own = append(own, f)
		}
	}
	return own, nil
}

// holdLock takes the lock at path; a held one is a refusal saying held.
func holdLock(path, held string) (func(), error) {
	release, err := worker.LockPath(path)
	if errors.Is(err, worker.ErrLocked) {
		return nil, notIdleError{refuse("%s; run ness upgrade again when it finishes", held)}
	}
	return release, err
}

// checkLockVersion refuses when the project's lock was written by a newer ness than this binary.
func checkLockVersion(d Deps, root string) error {
	data, err := os.ReadFile(filepath.Join(root, nsync.LockPath)) //nolint:gosec // fixed path under the repo
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	l, err := nsync.ParseLock(data)
	if err != nil {
		return refuse("%v", err)
	}
	c, err := nsync.Compare(l.Ness, d.Version)
	if err != nil {
		return refuse("%s: %v", nsync.LockPath, err)
	}
	if c > 0 {
		return refuse("%s records ness %s, newer than this binary (%s); run ness self-update",
			nsync.LockPath, l.Ness, d.Version)
	}
	return nil
}

// migrateStore brings the store to this binary's schema (R-UPD-010) and says so when an existing
// store migrated.
func migrateStore(d Deps, root string) error {
	path, err := store.Locate(root)
	if err != nil {
		return refuse("%v", err)
	}
	old, err := storeSchema(path)
	if err != nil {
		return err
	}
	s, err := store.Open(path)
	if errors.Is(err, store.ErrNewer) {
		return refuse("%v", err)
	}
	if err != nil {
		return err
	}
	if cerr := s.Close(); cerr != nil {
		return cerr
	}
	if 0 < old && old < store.Version() {
		say(d.Stdout, "store schema %d → %d (backup %s)", old, store.Version(), store.BackupPath(path, old))
	}
	return nil
}

// storeSchema is the schema of the store at path, read without writing; 0 when it does not exist.
func storeSchema(path string) (int, error) {
	ro, err := store.OpenReadOnly(path)
	var se *store.SchemaError
	switch {
	case err == nil:
		return store.Version(), ro.Close()
	case errors.Is(err, fs.ErrNotExist):
		return 0, nil
	case errors.As(err, &se):
		return se.Store, nil
	}
	return 0, err
}

// bumpPin sets ness.toml's pin to the binary's major.minor when it differs (R-CFG-010).
func bumpPin(d Deps, p *project) error {
	pin, err := config.PinOf(d.Version)
	if err != nil {
		return refuse("%v", err)
	}
	if p.file.Pin() == pin {
		return nil
	}
	path := filepath.Join(p.root, config.Path)
	data, err := os.ReadFile(path) //nolint:gosec // fixed path under the repo
	if err != nil {
		return err
	}
	if data, err = config.SetPin(data, pin); err != nil {
		return refuse("%s: %v", path, err)
	}
	if werr := os.WriteFile(path, data, 0o644); werr != nil { //nolint:gosec // a tracked config file
		return werr
	}
	say(d.Stdout, "pin ness = %q", pin)
	return nil
}

// freshWorker puts the project's worker on this binary's code: re-pointed (Install restarts it)
// when its unit runs another binary, else restarted; nothing without a unit (R-UPD-062).
func freshWorker(d Deps, project, primary string) error {
	bin := realPath(d.Executable)
	changed, err := repointWorker(d, project, primary, bin)
	if err != nil {
		return fmt.Errorf("re-point the worker: %w", err)
	}
	if changed {
		say(d.Stdout, "worker re-pointed to %s", bin)
		return nil
	}
	restarted, err := restartWorker(d, project)
	if err != nil {
		return fmt.Errorf("restart the worker: %w", err)
	}
	if restarted {
		say(d.Stdout, "worker restarted")
	}
	return nil
}
