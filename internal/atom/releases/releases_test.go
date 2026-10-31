package releases_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/releases"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
	"github.com/enea-scaccabarozzi/ness/internal/update"
)

// fetchAll fetches url through c and returns the whole body; every error is fatal.
func fetchAll(t *testing.T, c releases.Client, url string) []byte {
	t.Helper()
	rc, err := c.Fetch(context.Background(), url)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if err := rc.Close(); err != nil {
		t.Fatal(err)
	}
	return data
}

// Rejects a client that keeps the server's order wrong, drops assets, or mismaps the archive:
// the stable release resolves, verifies against checksums.txt and extracts its binary.
func TestReleasesAgainstServer(t *testing.T) {
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "0.1.0"}, testkit.FakeRelease{Version: "0.3.0-rc.1", Prerelease: true}, testkit.FakeRelease{Version: "0.2.0"})
	c := releases.Client{Base: srv.URL, HTTP: srv.Client()}
	rels, err := c.Releases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var order []string
	for _, r := range rels {
		order = append(order, r.Version)
	}
	if !slices.Equal(order, []string{"0.3.0-rc.1", "0.2.0", "0.1.0"}) {
		t.Fatalf("order = %v, want newest first", order)
	}
	r, err := update.Resolve(rels, "stable", "")
	if err != nil {
		t.Fatal(err)
	}
	url, err := r.Asset("linux", "amd64")
	if err != nil {
		t.Fatal(err)
	}
	data := fetchAll(t, c, url)
	sums := fetchAll(t, c, r.Assets[update.Checksums])
	if err := update.Verify(sums, update.AssetName("0.2.0", "linux", "amd64"), data); err != nil {
		t.Fatal(err)
	}
	bin, err := update.Extract(data)
	if err != nil || !strings.Contains(string(bin), "ness version 0.2.0") {
		t.Fatalf("binary = %q, %v", bin, err)
	}
}

// Rejects a fake that sorts prerelease ids as strings (rc.9 above rc.10).
func TestReleaseServerOrdersRCNumerically(t *testing.T) {
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "0.3.0-rc.9", Prerelease: true}, testkit.FakeRelease{Version: "0.3.0-rc.10", Prerelease: true})
	rels, err := releases.Client{Base: srv.URL, HTTP: srv.Client()}.Releases(context.Background())
	if err != nil || len(rels) != 2 || rels[0].Version != "0.3.0-rc.10" {
		t.Fatalf("%v, %v", rels, err)
	}
}

// Rejects a BadSum release whose archive still matches checksums.txt.
func TestReleaseServerBadSum(t *testing.T) {
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "0.2.0", BadSum: true})
	c := releases.Client{Base: srv.URL, HTTP: srv.Client()}
	rels, err := c.Releases(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	url, _ := rels[0].Asset("linux", "amd64") //nolint:errcheck // a missing asset fails the fetch below
	if err := update.Verify(fetchAll(t, c, rels[0].Assets[update.Checksums]), update.AssetName("0.2.0", "linux", "amd64"), fetchAll(t, c, url)); err == nil {
		t.Fatal("BadSum verified")
	}
}

// Rejects a client that decodes an error page as an empty release list.
func TestReleasesErrorStatus(t *testing.T) {
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "fail"})
	_, err := releases.Client{Base: srv.URL, HTTP: srv.Client()}.Releases(context.Background())
	if err == nil || !strings.Contains(err.Error(), "500") {
		t.Fatal(err)
	}
}

// Rejects a client that offers drafts, keeps the v, or asks without per_page=100.
func TestReleasesSkipsDrafts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases" || r.URL.Query().Get("per_page") != "100" {
			http.NotFound(w, r)
			return
		}
		_, _ = io.WriteString(w, `[{"tag_name":"v0.3.0","draft":true},{"tag_name":"v0.2.0","prerelease":false,"assets":[{"name":"checksums.txt","browser_download_url":"u"}]}]`) //nolint:errcheck // the client sees a short body
	}))
	t.Cleanup(srv.Close)
	rels, err := releases.Client{Base: srv.URL, HTTP: srv.Client()}.Releases(context.Background())
	if err != nil || len(rels) != 1 || rels[0].Version != "0.2.0" || rels[0].Assets["checksums.txt"] != "u" {
		t.Fatalf("%+v, %v", rels, err)
	}
}

// Rejects a Fetch that hands back a truncated body instead of failing past MaxDownload.
func TestFetchCapsTheDownload(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.CopyN(w, zeros{}, releases.MaxDownload+1) //nolint:errcheck // the client stops reading
	}))
	t.Cleanup(srv.Close)
	c := releases.Client{Base: srv.URL, HTTP: srv.Client()}
	rc, err := c.Fetch(context.Background(), srv.URL+"/big")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = rc.Close() }) //nolint:errcheck // read-only body
	n, err := io.Copy(io.Discard, rc)
	if err == nil || !strings.Contains(err.Error(), "larger than 200 MiB") {
		t.Fatalf("read %d bytes, err %v", n, err)
	}
}

type zeros struct{}

func (zeros) Read(p []byte) (int, error) { clear(p); return len(p), nil }
