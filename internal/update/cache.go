package update

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"
)

// Cache is update.json, the machine's last release check (R-UPD-050): when it ran and the newest
// stable and prerelease versions it saw.
type Cache struct {
	CheckedAt time.Time `json:"checked_at"`
	Stable    string    `json:"stable"`
	RC        string    `json:"rc"`
}

// CheckEvery is the least time between two release checks on one machine.
const CheckEvery = 24 * time.Hour

// CheckTimeout bounds one release check.
const CheckTimeout = 5 * time.Second

// CachePath is update.json inside dir, the ness user cache directory; an empty dir is an error so
// the cache never lands in the working directory.
func CachePath(dir string) (string, error) {
	if dir == "" {
		return "", errors.New("no user cache directory: the update check is off")
	}
	return filepath.Join(dir, "update.json"), nil
}

// ReadCache reads the cache at path; a missing file is the zero Cache.
func ReadCache(path string) (Cache, error) {
	var c Cache
	data, err := os.ReadFile(path) //nolint:gosec // our own cache
	if errors.Is(err, os.ErrNotExist) {
		return c, nil
	}
	if err != nil {
		return c, err
	}
	err = json.Unmarshal(data, &c)
	return c, err
}

// WriteCache writes c to path through a temp file renamed into place.
func WriteCache(path string, c Cache) error {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, append(data, '\n'))
}

// CacheFrom is the cache for rels checked at now: the newest stable and the newest prerelease.
func CacheFrom(rels []Release, now time.Time) Cache {
	stable, _ := newest(rels, false)
	var rc Release
	for _, r := range rels {
		if r.Prerelease && newer(r.Version, rc.Version) {
			rc = r
		}
	}
	return Cache{CheckedAt: now, Stable: stable.Version, RC: rc.Version}
}

// Due reports whether the next check may run at now.
func (c Cache) Due(now time.Time) bool {
	return c.CheckedAt.IsZero() || now.Sub(c.CheckedAt) >= CheckEvery
}

// Offer is the version channel offers above installed, or "" when nothing is newer. The rc
// channel offers the newer of RC and Stable (R-UPD-040).
func (c Cache) Offer(channel, installed string) string {
	best := c.Stable
	if channel == "rc" && newer(c.RC, best) {
		best = c.RC
	}
	if newer(best, installed) {
		return best
	}
	return ""
}

// Refresh runs one release check bounded by CheckTimeout. A failure is quiet: it keeps old's
// versions and still stamps now, so the next try comes CheckEvery later.
func Refresh(ctx context.Context, src Source, old Cache, now time.Time) Cache {
	ctx, cancel := context.WithTimeout(ctx, CheckTimeout)
	defer cancel()
	rels, err := src.Releases(ctx)
	if err != nil {
		old.CheckedAt = now
		return old
	}
	return CacheFrom(rels, now)
}

// writeAtomic writes data to path through a temp file renamed into place in the same directory,
// creating the directory.
func writeAtomic(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, "."+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	_, err = tmp.Write(data)
	if err = errors.Join(err, tmp.Close()); err == nil {
		err = os.Rename(tmp.Name(), path)
	}
	if err != nil {
		return errors.Join(err, os.Remove(tmp.Name()))
	}
	return nil
}
