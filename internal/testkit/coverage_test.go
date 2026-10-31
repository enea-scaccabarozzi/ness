package testkit_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func fixture(t *testing.T) (specDir, root string) {
	t.Helper()
	root = t.TempDir()
	specDir = filepath.Join(root, "docs", "spec")
	write(t, filepath.Join(specDir, "a.md"), "- **R-AAA-001** x MUST y.\n- **R-AAA-002** x MUST z.\n"+
		"- **R-AAA-003** x MUST w.\n- **R-BBB-001** other area.\n")
	write(t, filepath.Join(specDir, "reviews.md"),
		"- R-AAA-003 | s-123 | 2026-10-09 | abc1234\n- R-AAA-002 | s-123 | 2026-10-09\n")
	// The markers are split so the repository's own scan does not read them as real markers.
	marker := "cov" + "ers:"
	write(t, filepath.Join(root, "pkg", "x_test.go"), "package x\n// "+marker+" R-AAA-001, R-ZZZ-999\n")
	write(t, filepath.Join(root, "pkg", "testdata", "script", "s.txtar"), "# "+marker+" R-AAA-001\n")
	return specDir, root
}

func TestCoverageProblems(t *testing.T) {
	// covers: R-QA-010, R-QA-011, R-QA-012, R-QA-013
	specDir, root := fixture(t)
	c, err := testkit.ScanCoverage(specDir, root)
	if err != nil {
		t.Fatal(err)
	}
	got := c.Problems([]string{"AAA"}, nil)
	want := []string{
		"R-AAA-002 has no covering test or review entry",
		"R-ZZZ-999 is named by a covers marker or review entry but not defined in docs/spec",
	}
	if !slices.Equal(got, want) {
		t.Fatalf("problems:\n%q\nwant:\n%q", got, want)
	}
}

func TestCoverageDeferred(t *testing.T) {
	// covers: R-QA-010
	specDir, root := fixture(t)
	c, err := testkit.ScanCoverage(specDir, root)
	if err != nil {
		t.Fatal(err)
	}
	got := c.Problems([]string{"AAA"}, map[string]string{"R-AAA-002": "P2"})
	if len(got) != 1 {
		t.Fatalf("problems: %q", got)
	}
}
