package update

import (
	"context"
	"fmt"
	"io"
	"strings"

	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
)

// Release is one published ness release: its version without the leading v, whether it is a
// GitHub pre-release, and its assets by name → download URL.
type Release struct {
	Version    string
	Prerelease bool
	Assets     map[string]string
}

// Source lists the published releases and fetches an asset by URL.
type Source interface {
	Releases(ctx context.Context) ([]Release, error)
	Fetch(ctx context.Context, url string) (io.ReadCloser, error)
}

const goInstall = "go install github.com/enea-scaccabarozzi/ness/cmd/ness@v"

// Resolve picks the release to install (R-UPD-040): want when set (a leading v is allowed), else
// the newest release on channel. The rc channel offers prereleases and stable releases alike.
func Resolve(rels []Release, channel, want string) (Release, error) {
	if want != "" {
		want = strings.TrimPrefix(want, "v")
		for _, r := range rels {
			if strings.TrimPrefix(r.Version, "v") == want {
				return r, nil
			}
		}
		return Release{}, fmt.Errorf("no release %s", want)
	}
	best, ok := newest(rels, channel == "rc")
	if !ok {
		return Release{}, fmt.Errorf("no %s release", channel)
	}
	return best, nil
}

// newest returns the highest-precedence release with a valid version, prereleases only when pre
// is true.
func newest(rels []Release, pre bool) (Release, bool) {
	var best Release
	for _, r := range rels {
		if (pre || !r.Prerelease) && newer(r.Version, best.Version) {
			best = r
		}
	}
	return best, best.Version != ""
}

// newer reports whether a is a valid version above b; "" is below every valid version.
func newer(a, b string) bool {
	if b == "" {
		_, err := nsync.Compare(a, a)
		return err == nil
	}
	c, err := nsync.Compare(a, b)
	return err == nil && c > 0
}

// AssetName is the archive name goreleaser publishes for one version and OS/arch pair.
func AssetName(version, goos, goarch string) string {
	return fmt.Sprintf("ness_%s_%s_%s.tar.gz", strings.TrimPrefix(version, "v"), goos, goarch)
}

// Asset returns the URL of r's archive for goos/goarch, or an error naming the unbuilt pair and
// the go install command that builds it (R-UPD-044).
func (r Release) Asset(goos, goarch string) (string, error) {
	if u, ok := r.Assets[AssetName(r.Version, goos, goarch)]; ok {
		return u, nil
	}
	return "", fmt.Errorf("ness %s is not built for %s/%s: run %s%s",
		r.Version, goos, goarch, goInstall, strings.TrimPrefix(r.Version, "v"))
}
