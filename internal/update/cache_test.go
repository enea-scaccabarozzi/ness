package update_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/update"
)

func TestCacheDueOffer(t *testing.T) {
	// covers: R-UPD-050
	t0 := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	c := update.Cache{CheckedAt: t0, Stable: "0.2.0", RC: "0.3.0-rc.1"}
	// rejects: > instead of >= at the 24 h boundary, and a zero cache never due
	if c.Due(t0.Add(23*time.Hour)) || !c.Due(t0.Add(24*time.Hour)) || !(update.Cache{}).Due(t0) {
		t.Fatal("Due")
	}
	for _, o := range []struct{ ch, inst, want string }{
		{"stable", "0.1.0", "0.2.0"},
		{"rc", "0.2.0", "0.3.0-rc.1"},
		{"stable", "0.2.0", ""},
		{"stable", "0.2.0+abc", ""}, // rejects: build metadata making the same version look older
		{"stable", "0.3.0", ""},     // rejects: offering a downgrade
	} {
		if got := c.Offer(o.ch, o.inst); got != o.want {
			t.Errorf("Offer(%s,%s) = %q, want %q", o.ch, o.inst, got, o.want)
		}
	}
	// rejects: the rc channel offering only RC when a newer stable exists (R-UPD-040)
	if got := (update.Cache{Stable: "0.3.0", RC: "0.3.0-rc.2"}).Offer("rc", "0.2.0"); got != "0.3.0" {
		t.Errorf("rc offer with newer stable = %q, want 0.3.0", got)
	}
}

func TestCacheFrom(t *testing.T) {
	// rejects: last-listed prerelease, or string ordering of rc ids
	t0 := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	got := update.CacheFrom([]update.Release{{Version: "0.1.0"}, {Version: "0.2.0-rc.10", Prerelease: true}, {Version: "0.2.0-rc.9", Prerelease: true}}, t0)
	if got != (update.Cache{CheckedAt: t0, Stable: "0.1.0", RC: "0.2.0-rc.10"}) {
		t.Fatalf("%+v", got)
	}
}

func TestCacheEmptyDir(t *testing.T) {
	// covers: R-UPD-050 — rejects: an empty dir resolving to a relative path in the repository
	if _, err := update.CachePath(""); err == nil || !strings.Contains(err.Error(), "no user cache directory") {
		t.Fatalf("CachePath(\"\") = %v", err)
	}
}

func TestCacheRoundTrip(t *testing.T) {
	// covers: R-UPD-050 — rejects: a missing file reported as an error, fields lost on write
	p, err := update.CachePath(filepath.Join(t.TempDir(), "ness"))
	if err != nil || filepath.Base(p) != "update.json" || !filepath.IsAbs(p) {
		t.Fatalf("%s, %v", p, err)
	}
	if c, err := update.ReadCache(p); err != nil || c != (update.Cache{}) {
		t.Fatalf("missing: %+v, %v", c, err)
	}
	want := update.Cache{CheckedAt: time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC), Stable: "0.2.0", RC: "0.3.0-rc.1"}
	if err := update.WriteCache(p, want); err != nil {
		t.Fatal(err)
	}
	if c, err := update.ReadCache(p); err != nil || !c.CheckedAt.Equal(want.CheckedAt) || c.Stable != "0.2.0" || c.RC != "0.3.0-rc.1" {
		t.Fatalf("%+v, %v", c, err)
	}
}

func TestRefreshFailureKeepsVersions(t *testing.T) {
	// covers: R-UPD-050 — rejects: a failed fetch wiping the versions, or not stamping now
	t0 := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	got := update.Refresh(context.Background(), failingSource{}, update.Cache{Stable: "0.2.0", RC: "0.3.0-rc.1"}, t0)
	if got.Stable != "0.2.0" || got.RC != "0.3.0-rc.1" || !got.CheckedAt.Equal(t0) {
		t.Fatalf("%+v", got)
	}
}
