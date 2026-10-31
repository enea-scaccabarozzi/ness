package update_test

import (
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/update"
)

func TestResolve(t *testing.T) {
	// covers: R-UPD-040
	rels := []update.Release{{Version: "0.2.0"}, {Version: "0.3.0-rc.1", Prerelease: true}, {Version: "0.1.0"}}
	for _, c := range []struct{ ch, want, got string }{
		{"stable", "", "0.2.0"},       // rejects: first or last listed release instead of newest
		{"rc", "", "0.3.0-rc.1"},      // rejects: rc channel ignoring prereleases
		{"stable", "0.1.0", "0.1.0"},  // rejects: --version ignored
		{"stable", "v0.1.0", "0.1.0"}, // rejects: leading v not stripped
	} {
		r, err := update.Resolve(rels, c.ch, c.want)
		if err != nil || r.Version != c.got {
			t.Errorf("Resolve(%s,%q) = %s, %v", c.ch, c.want, r.Version, err)
		}
	}
	// rejects: an unknown --version falling back to the newest release
	if _, err := update.Resolve(rels, "stable", "9.9.9"); err == nil || !strings.Contains(err.Error(), "no release 9.9.9") {
		t.Errorf("unknown version: %v", err)
	}
	// rejects: an empty list returning a zero Release without error
	if _, err := update.Resolve(nil, "stable", ""); err == nil || !strings.Contains(err.Error(), "no stable release") {
		t.Errorf("empty: %v", err)
	}
	r, err := update.Resolve([]update.Release{{Version: "0.3.0-rc.2", Prerelease: true}, {Version: "0.3.0"}}, "rc", "")
	if err != nil || r.Version != "0.3.0" { // rejects: rc channel = prereleases only
		t.Errorf("rc channel = %s, %v, want 0.3.0", r.Version, err)
	}
}

func TestResolveRCTen(t *testing.T) {
	// covers: R-UPD-040 — Review focus 2; rejects: string comparison of prerelease ids
	for _, rels := range [][]update.Release{
		{{Version: "0.3.0-rc.9", Prerelease: true}, {Version: "0.3.0-rc.10", Prerelease: true}},
		{{Version: "0.3.0-rc.10", Prerelease: true}, {Version: "0.3.0-rc.9", Prerelease: true}}, // rejects: last listed wins
	} {
		r, err := update.Resolve(rels, "rc", "")
		if err != nil || r.Version != "0.3.0-rc.10" {
			t.Fatalf("got %s, %v", r.Version, err)
		}
	}
}

func TestAssetName(t *testing.T) {
	// rejects: a v-prefixed or differently ordered name that the release never publishes
	if got := update.AssetName("0.2.0", "linux", "amd64"); got != "ness_0.2.0_linux_amd64.tar.gz" {
		t.Fatal(got)
	}
}

func TestAssetUnbuiltPair(t *testing.T) {
	// covers: R-UPD-044
	r := update.Release{Version: "0.2.0", Assets: map[string]string{"ness_0.2.0_linux_amd64.tar.gz": "u"}}
	if u, err := r.Asset("linux", "amd64"); err != nil || u != "u" {
		t.Fatal(u, err)
	}
	// rejects: an empty URL without error, or an error that names neither the pair nor the fix
	_, err := r.Asset("linux", "riscv64")
	if err == nil || !strings.Contains(err.Error(), "linux/riscv64") || !strings.Contains(err.Error(), "go install github.com/enea-scaccabarozzi/ness/cmd/ness@v0.2.0") {
		t.Fatalf("err = %v", err)
	}
}
