package update_test

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"errors"
	"io"
	"os"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/update"
)

// tarEntry is one archive member built by tarGz.
type tarEntry struct {
	name, body, link string
	typ              byte
}

func file(name, body string) tarEntry { return tarEntry{name: name, body: body, typ: tar.TypeReg} }

func symlink(name, target string) tarEntry {
	return tarEntry{name: name, link: target, typ: tar.TypeSymlink}
}

// tarGz builds a gzip-compressed tar archive holding entries.
func tarGz(t *testing.T, entries ...tarEntry) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for _, e := range entries {
		h := &tar.Header{Name: e.name, Typeflag: e.typ, Mode: 0o755, Size: int64(len(e.body)), Linkname: e.link}
		if e.typ != tar.TypeReg {
			h.Size = 0
		}
		if err := tw.WriteHeader(h); err != nil {
			t.Fatal(err)
		}
		if e.typ == tar.TypeReg {
			if _, err := tw.Write([]byte(e.body)); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func writeFile(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
}

// failingSource is a release source that fails every call.
type failingSource struct{}

func (failingSource) Releases(context.Context) ([]update.Release, error) {
	return nil, errors.New("network down")
}

func (failingSource) Fetch(context.Context, string) (io.ReadCloser, error) {
	return nil, errors.New("network down")
}

var _ update.Source = failingSource{}
