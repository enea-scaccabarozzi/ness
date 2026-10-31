package testkit

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
)

var update = flag.Bool("update", false, "rewrite golden files")

// Golden compares got with the file at path; -update rewrites it (R-NTF-030).
func Golden(t *testing.T, path, got string) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(path) //nolint:gosec // a golden file of the test
	if err != nil {
		t.Fatalf("%s: %v (run with -update to write it)", path, err)
	}
	if string(want) != got {
		t.Errorf("%s differs:\n got: %q\nwant: %q", path, got, string(want))
	}
}

// ModuleRoot is the directory holding go.mod above the test's working directory.
func ModuleRoot(t *testing.T) string {
	t.Helper()
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	for {
		if _, serr := os.Stat(filepath.Join(dir, "go.mod")); serr == nil {
			return dir
		}
		if dir == filepath.Dir(dir) {
			t.Fatal("no go.mod above the test directory")
		}
		dir = filepath.Dir(dir)
	}
}

// RenderConstructs renders every construct file through r, by construct name.
func RenderConstructs(t *testing.T, r notify.Renderer) map[string]string {
	t.Helper()
	files, err := filepath.Glob(filepath.Join(ModuleRoot(t), "internal", "notify", "testdata", "constructs", "*.md"))
	if err != nil || len(files) == 0 {
		t.Fatalf("constructs: %v, %d files", err, len(files))
	}
	out := map[string]string{}
	for _, f := range files {
		b, rerr := os.ReadFile(f) //nolint:gosec // a construct fixture
		if rerr != nil {
			t.Fatal(rerr)
		}
		parts := make([]string, 0, 8)
		for _, blk := range notify.Parse(string(b)) {
			parts = append(parts, r.Render(blk))
		}
		out[strings.TrimSuffix(filepath.Base(f), ".md")] = strings.Join(parts, "\n\n")
	}
	return out
}
