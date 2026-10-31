package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
	"github.com/enea-scaccabarozzi/ness/internal/update"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

// unattended is the worker's update step (R-UPD-050, R-UPD-080..082): the daily check whatever
// auto is, then the machine-wide install, this project's upgrade at the idle point, or the behind
// fyi to each open root, as update.Decide plans. Its error is for the worker log.
func unattended(d Deps, env workerEnv, s *store.Store) func(now time.Time) error {
	bases := map[string]baseLock{}
	return func(now time.Time) error {
		var cache update.Cache
		if env.cfg.Update.Check {
			cache = refreshCache(d, now)
		}
		in := update.Inputs{
			Installed: d.Version, Offer: cache.Offer(env.cfg.Update.Channel, d.Version), Auto: env.cfg.Update.Auto,
			// The idle point is runUpgrade's own: a refusal there is not idle, silent (errNotIdle).
			Idle: env.cfg.Update.Auto == "install",
		}
		// The base branch's lock, not the working tree's; unreadable, the project is neither
		// upgraded nor said to be behind, and the log says why once per cause.
		in.Lock = in.Installed
		lockNote := ""
		if l, err := readBaseLock(d, env.primary, env.cfg.Base, bases); err == nil {
			in.Lock = l.Ness
		} else {
			lockNote = "the base lock cannot be read: " + err.Error()
		}
		if in.Auto == "install" && in.Offer != "" {
			machineInputs(d, &in)
		}
		plan := update.Decide(in)
		var err error
		if plan.Install != "" {
			var pending bool
			// Backed off or failed, the install must not starve this project's own upgrade.
			if pending, err = unattendedInstall(d, env.cfg.Update.Channel, plan.Install, now); !pending {
				in.Offer = ""
				plan.Upgrade = update.Decide(in).Upgrade
			}
		}
		switch {
		case plan.Upgrade:
			err = errors.Join(err, unattendedUpgrade(d, env.p, now))
		case plan.Behind:
			err = sendBehind(s, in, now)
		}
		if in.Auto != "install" {
			err = errors.Join(err, clearFailure(env.p))
		}
		return errors.Join(err, noteOnce(env.p, "update-note", plan.Note), noteOnce(env.p, "base-lock-note", lockNote))
	}
}

// machineInputs fills the veto's inputs: every registered project's auto, whether another tool
// manages the binary, and the ceiling's verdict on the offer. An unreadable registry holds the
// install through Ceiling, never this project's upgrade or fyi.
// ponytail: the veto reads each registered project's ness.toml on every pass with an offer; cache
// the heads by mtime if many projects make that slow.
func machineInputs(d Deps, in *update.Inputs) {
	roots, err := d.Registry.List()
	if err != nil {
		in.Ceiling = fmt.Errorf("registry: %w", err)
		return
	}
	in.Autos = map[string]string{}
	var unreadable []error // they hold the install only, never this project's upgrade or fyi
	for _, root := range roots {
		h, herr := readHeadAt(filepath.Join(root, config.Path))
		if herr != nil {
			unreadable = append(unreadable, herr)
			continue
		}
		in.Autos[root] = h.Update.Auto
	}
	_, in.Managed = update.ManagedBy(realPath(d.Executable), in.Offer, os.Getenv, realPath(userHome()))
	pins, err := machinePins(d)
	if err == nil {
		err = update.Ceiling(in.Offer, pins)
	}
	in.Ceiling = errors.Join(append(unreadable, err)...)
}

// unattendedInstall installs version under the machine install lock. A lock held by another
// process, or a binary already at version or newer (installed by an earlier pass, another
// project's worker or by hand, whose .prev must stay the old one), skips the pass silently,
// before any release request; so does a failure of version within CheckEvery. A failure is
// recorded in the cache directory and returned only when its cause is new (R-UPD-080). pending
// is true when the binary is installed or being installed, so the restarted worker upgrades the
// project; false when the install backed off or failed.
func unattendedInstall(d Deps, channel, version string, now time.Time) (pending bool, err error) {
	release, err := worker.LockPath(filepath.Join(d.CacheDir, "install.lock"))
	if errors.Is(err, worker.ErrLocked) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	defer release()
	if atLeast(d, version) {
		return true, nil
	}
	path := filepath.Join(d.CacheDir, "install-failed.json")
	if backedOff(path, version, now) {
		return false, nil
	}
	ierr := installRelease(d, channel, version)
	if ierr == nil {
		return true, update.ClearFailure(path)
	}
	cause, _, _ := strings.Cut(ierr.Error(), "\n")
	changed, err := update.RecordFailure(path, update.Failure{Version: version, Cause: cause, At: now})
	if err != nil || !changed {
		return false, err
	}
	return false, fmt.Errorf("automatic install of %s failed: %s; retried in 24 h or for a newer release",
		version, cause)
}

// backedOff reports a failure of version within CheckEvery recorded at path; an unreadable record
// is no record (the next failure overwrites it).
func backedOff(path, version string, now time.Time) bool {
	f, ok, err := update.ReadFailure(path)
	return err == nil && ok && f.Version == version && now.Sub(f.At) < update.CheckEvery
}

// atLeast reports that the binary on disk is version or newer, so an older offer never
// overwrites a newer binary someone installed by hand.
func atLeast(d Deps, version string) bool {
	out, err := probeVersion(realPath(d.Executable))
	if err != nil {
		return false
	}
	c, err := nsync.Compare(lastField(out), version)
	return err == nil && c >= 0
}

// installRelease lists the releases once and installs version from channel.
func installRelease(d Deps, channel, version string) error {
	rels, err := d.Releases.Releases(context.Background())
	if err != nil {
		return fmt.Errorf("release list: %w", err)
	}
	rel, err := update.Resolve(rels, channel, version)
	if err != nil {
		return err
	}
	_, err = installVersion(d, rel)
	return err
}

// lastField is the last word of out: X in "ness version X".
func lastField(out string) string {
	f := strings.Fields(out)
	if len(f) == 0 {
		return ""
	}
	return f[len(f)-1]
}

// unattendedUpgrade upgrades the base checkout without restarting this worker. A refusal at the
// idle point (a dirty base, a lock held) is not idle: silent, retried next pass. A failure after
// it is recorded, and returned only when its cause is new, so the log names each cause once across
// worker processes (R-UPD-081). runUpgrade clears the record on success.
// ponytail: the upgrade's git commit runs the project's commit hooks inline in the pass, so a slow
// pre-commit hook stalls all session supervision while it runs; move it off the pass if that bites.
func unattendedUpgrade(d Deps, p *project, now time.Time) error {
	uerr := runUpgrade(d, upgradeOpts{restart: false})
	if uerr == nil || errors.As(uerr, new(notIdleError)) {
		return nil
	}
	path, err := failurePath(p)
	if err != nil {
		return err
	}
	cause, _, _ := strings.Cut(uerr.Error(), "\n")
	changed, err := update.RecordFailure(path, update.Failure{Version: d.Version, Cause: cause, At: now})
	if err != nil || !changed {
		return err
	}
	return fmt.Errorf("automatic upgrade to %s failed: %s", d.Version, cause)
}

// clearFailure removes the project's unattended-upgrade failure record.
func clearFailure(p *project) error {
	path, err := failurePath(p)
	if err != nil {
		return err
	}
	return update.ClearFailure(path)
}

// sendBehind tells each open root the project is behind the binary; the item key sends it once per
// version pair (R-UPD-082).
func sendBehind(s *store.Store, in update.Inputs, now time.Time) error {
	sessions, err := s.Sessions(false)
	if err != nil {
		return err
	}
	body := fmt.Sprintf("ness update: this project's ness files are %s, the binary is %s: "+
		"run ness upgrade in the base checkout", in.Lock, in.Installed)
	var errs []error
	for _, sess := range sessions {
		if sess.Parent == store.Operator {
			_, _, serr := s.Send(sess.ID, sess.ID, store.IntentFYI, body, false, now)
			errs = append(errs, serr)
		}
	}
	return errors.Join(errs...)
}

// noteOnce returns note as an error when it differs from the one last returned, kept in the file
// name beside the failure record, so the log says it once across worker processes (R-UPD-080);
// no note forgets it.
func noteOnce(p *project, name, note string) error {
	failure, err := failurePath(p)
	if err != nil {
		return err
	}
	path := filepath.Join(filepath.Dir(failure), name)
	old, _ := os.ReadFile(path) //nolint:errcheck,gosec // a missing note is none
	if note == string(old) {
		return nil
	}
	if note == "" {
		return os.Remove(path)
	}
	if werr := os.WriteFile(path, []byte(note), 0o600); werr != nil {
		return werr
	}
	return errors.New(note)
}
