package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/update"
)

// refreshCache is the machine's release cache, refreshed and written when it is due (R-UPD-050).
// Every error is quiet: no cache directory gives the zero Cache and writes nothing.
func refreshCache(d Deps, now time.Time) update.Cache {
	path, err := update.CachePath(d.CacheDir)
	if err != nil {
		return update.Cache{}
	}
	c, _ := update.ReadCache(path) //nolint:errcheck // an unreadable cache is a due one
	if !c.Due(now) || d.Releases == nil {
		return c
	}
	c = update.Refresh(context.Background(), d.Releases, c, now)
	_ = update.WriteCache(path, c) //nolint:errcheck // a failed write retries on the next run
	return c
}

// readCache is the machine's release cache as it stands; hooks read only this (R-UPD-051).
func readCache(d Deps) update.Cache {
	path, err := update.CachePath(d.CacheDir)
	if err != nil {
		return update.Cache{}
	}
	c, _ := update.ReadCache(path) //nolint:errcheck // an unreadable cache offers nothing
	return c
}

// failurePath is <git-common-dir>/ness/upgrade-failed.json, the unattended-upgrade failure record.
func failurePath(p *project) (string, error) {
	path, err := store.Locate(p.root)
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "upgrade-failed.json"), nil
}

// readFailure is the project's failure record; ok is false when none can be read.
func readFailure(p *project) (update.Failure, bool) {
	path, err := failurePath(p)
	if err != nil {
		return update.Failure{}, false
	}
	f, ok, err := update.ReadFailure(path)
	return f, ok && err == nil
}

// installFailure is the machine's failed unattended install of a version newer than this binary.
func installFailure(d Deps) (update.Failure, bool) {
	if d.CacheDir == "" {
		return update.Failure{}, false
	}
	f, ok, err := update.ReadFailure(filepath.Join(d.CacheDir, "install-failed.json"))
	return f, ok && err == nil && newerThan(f.Version, d.Version)
}

// showsOffer reports that the project wants the check's result shown: check on, auto not off.
func showsOffer(cfg *config.Config) bool {
	return cfg != nil && cfg.Update.Check && cfg.Update.Auto != "off"
}

// rootNotices are a root's session-start lines from the cache and the failure record alone: the
// newer release (R-UPD-053) and a failed automatic upgrade. It never fetches and never posts.
func rootNotices(d Deps, p *project, cfg *config.Config) []string {
	var lines []string
	if showsOffer(cfg) {
		if v := readCache(d).Offer(cfg.Update.Channel, d.Version); v != "" {
			lines = append(lines, fmt.Sprintf("ness %s available (installed %s): ness self-update", v, d.Version))
		}
	}
	if f, ok := readFailure(p); ok {
		lines = append(lines, fmt.Sprintf(
			"ness: the automatic upgrade to %s failed: %s; the worker retries each pass (ness doctor shows it)",
			f.Version, f.Cause))
	}
	return lines
}

// checkUpdate: a failed automatic upgrade, else a failed automatic install, else the newer release
// the cache offers (R-UPD-052).
func checkUpdate(e *doctorEnv) check {
	if f, ok := readFailure(e.p); ok {
		return warn("update", fmt.Sprintf("automatic upgrade to %s failed: %s", f.Version, f.Cause),
			"ness upgrade")
	}
	if f, ok := installFailure(e.d); ok {
		return warn("update", fmt.Sprintf("automatic install of %s failed: %s", f.Version, f.Cause), "ness self-update")
	}
	if !showsOffer(e.cfg) {
		return ok("update", "check off ([update])")
	}
	c := e.cache
	if c.Stable == "" && c.RC == "" {
		return ok("update", "no release information yet; next try within 24 h")
	}
	installed := e.d.Version
	if v := c.Offer(e.cfg.Update.Channel, installed); v != "" {
		fix := "ness self-update"
		if command, managed := update.ManagedBy(realPath(e.d.Executable), v, os.Getenv, realPath(userHome())); managed {
			fix = command
		}
		return warn("update", fmt.Sprintf("ness %s available (installed %s)", v, installed), fix)
	}
	channel := e.cfg.Update.Channel
	return ok("update", fmt.Sprintf("%s (checked %s)",
		upToDate(installed, channel, c.Latest(channel), false), c.CheckedAt.Format(time.DateOnly)))
}
