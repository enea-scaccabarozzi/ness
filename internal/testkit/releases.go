package testkit

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"syscall"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"

	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
	release "github.com/enea-scaccabarozzi/ness/internal/update"
)

// FakeRelease is one release the fake server publishes: an archive per arch (default amd64 and
// arm64) holding a ness sh script that prints "ness version <Version>", and checksums.txt, wrong
// for every archive when BadSum. Version "fail" makes the server answer 500 to everything.
type FakeRelease struct {
	Version            string
	Prerelease, BadSum bool
	Arches             []string
}

// ReleaseServer serves rels as the GitHub releases API does, closed when the test ends.
func ReleaseServer(tb testing.TB, rels ...FakeRelease) *httptest.Server {
	tb.Helper()
	srv := newReleaseServer(rels, nil)
	tb.Cleanup(srv.Close)
	return srv
}

// newReleaseServer serves rels: GET /releases lists them newest first by semver, whatever order
// they came in, and /download/<version>/<asset> serves the assets. Each request line goes to log
// when it is not nil.
func newReleaseServer(rels []FakeRelease, log io.Writer) *httptest.Server {
	var mu sync.Mutex
	fail := slices.ContainsFunc(rels, func(r FakeRelease) bool { return r.Version == "fail" })
	rels = slices.Clone(rels)
	slices.SortFunc(rels, func(a, b FakeRelease) int {
		c, _ := nsync.Compare(b.Version, a.Version) //nolint:errcheck // fixtures carry valid versions
		return c
	})
	files := map[string][]byte{}
	for _, r := range rels {
		if !fail {
			releaseFiles(r, files)
		}
	}
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		if log != nil {
			mu.Lock()
			//nolint:errcheck // a lost line fails the script's grep
			_, _ = fmt.Fprintf(log, "%s %s\n", req.Method, req.URL.RequestURI())
			mu.Unlock()
		}
		switch {
		case fail:
			http.Error(w, "fake release server failing", http.StatusInternalServerError)
		case req.URL.Path == "/releases":
			w.Header().Set("Content-Type", "application/json")
			//nolint:errcheck // the client sees a short body
			_ = json.NewEncoder(w).Encode(listing(rels, "http://"+req.Host))
		case files[req.URL.Path] != nil:
			_, _ = w.Write(files[req.URL.Path]) //nolint:errcheck // as above
		default:
			http.NotFound(w, req)
		}
	}))
}

func assetPath(version, name string) string { return "/download/" + version + "/" + name }

func arches(r FakeRelease) []string {
	if len(r.Arches) == 0 {
		return []string{"amd64", "arm64"}
	}
	return r.Arches
}

// releaseFiles adds r's archives and checksums.txt to files by URL path.
func releaseFiles(r FakeRelease, files map[string][]byte) {
	var sums strings.Builder
	for _, arch := range arches(r) {
		name := release.AssetName(r.Version, "linux", arch)
		data := archive("#!/bin/sh\necho 'ness version " + r.Version + "'\n")
		files[assetPath(r.Version, name)] = data
		if r.BadSum {
			data = append(slices.Clone(data), 'x')
		}
		sum := sha256.Sum256(data)
		fmt.Fprintf(&sums, "%s  %s\n", hex.EncodeToString(sum[:]), name)
	}
	files[assetPath(r.Version, release.Checksums)] = []byte(sums.String())
}

// archive is a .tar.gz holding one executable file named ness.
func archive(script string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	err := tw.WriteHeader(&tar.Header{Name: "ness", Mode: 0o755, Size: int64(len(script)), Typeflag: tar.TypeReg})
	if err == nil {
		_, err = tw.Write([]byte(script))
	}
	if err = errors.Join(err, tw.Close(), gz.Close()); err != nil {
		panic(err) // writes into a bytes.Buffer do not fail
	}
	return buf.Bytes()
}

type ghAsset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}

type ghRelease struct {
	Tag        string    `json:"tag_name"`
	Prerelease bool      `json:"prerelease"`
	Draft      bool      `json:"draft"`
	Assets     []ghAsset `json:"assets"`
}

// listing is the GitHub JSON for rels, asset URLs under base.
func listing(rels []FakeRelease, base string) []ghRelease {
	out := make([]ghRelease, 0, len(rels))
	for _, r := range rels {
		names := make([]string, 0, 3)
		for _, arch := range arches(r) {
			names = append(names, release.AssetName(r.Version, "linux", arch))
		}
		gr := ghRelease{Tag: "v" + r.Version, Prerelease: r.Prerelease}
		for _, n := range append(names, release.Checksums) {
			gr.Assets = append(gr.Assets, ghAsset{Name: n, URL: base + assetPath(r.Version, n)})
		}
		out = append(out, gr)
	}
	return out
}

// releasesCmd is the script command `releases SPEC...`, SPEC = VERSION[:pre][:badsum] or fail:
// it starts a release server for the script, points NESS_RELEASES_URL at it, and logs each
// request line to $WORK/releases.log, truncated first.
func releasesCmd(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) == 0 {
		ts.Fatalf("usage: releases SPEC...")
	}
	rels := make([]FakeRelease, 0, len(args))
	for _, spec := range args {
		parts := strings.Split(spec, ":")
		r := FakeRelease{Version: parts[0]}
		for _, flag := range parts[1:] {
			switch flag {
			case "pre":
				r.Prerelease = true
			case "badsum":
				r.BadSum = true
			default:
				ts.Fatalf("releases: unknown flag %q in %s", flag, spec)
			}
		}
		if _, err := nsync.Compare(r.Version, r.Version); err != nil && r.Version != "fail" {
			ts.Fatalf("releases: %v", err)
		}
		rels = append(rels, r)
	}
	f, err := os.Create(filepath.Join(ts.Getenv("WORK"), "releases.log"))
	ts.Check(err)
	srv := newReleaseServer(rels, f)
	ts.Setenv("NESS_RELEASES_URL", srv.URL)
	ts.Defer(func() {
		srv.Close()
		_ = f.Close() //nolint:errcheck // the script is over
	})
}

// lockCmd is the script command `lock PATH`: it holds an exclusive flock on PATH from the test
// process until the script ends.
func lockCmd(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 1 {
		ts.Fatalf("usage: lock PATH")
	}
	f, err := os.OpenFile(ts.MkAbs(args[0]), os.O_CREATE|os.O_RDWR, 0o600)
	ts.Check(err)
	ts.Check(syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)) //nolint:gosec // an fd fits an int
	ts.Defer(func() { _ = f.Close() })                                    //nolint:errcheck // closing drops the lock
}
