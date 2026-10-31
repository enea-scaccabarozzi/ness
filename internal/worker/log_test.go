package worker_test

import (
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

var t0 = time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)

func newLog(t *testing.T) (*worker.Log, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "ness", "worker.log")
	return &worker.Log{Path: path, Now: func() time.Time { return t0 }}, path
}

func lines(t *testing.T, path string) []string {
	t.Helper()
	data, err := os.ReadFile(path) //nolint:gosec // the test's own log
	if err != nil {
		return nil
	}
	return strings.Split(strings.TrimSpace(string(data)), "\n")
}

func TestLogOneLinePerAction(t *testing.T) {
	// covers: R-WRK-021, R-WRK-023
	l, path := newLog(t)
	for _, a := range [][3]string{
		{"hint", "s-aaaaaaaaaaaa", "3 unread"}, {"compact", "s-aaaaaaaaaaaa", "plan written"},
		{"retire", "m-abcdefgh", "recipient closed"}, {"resume", "s-aaaaaaaaaaaa", "after the reset"},
		{"stall", "s-aaaaaaaaaaaa", "idle 31m with 1 obligation"}, {"restart", "-", "binary is now 0.2.0"},
	} {
		l.Action(a[0], a[1], a[2])
	}
	got := lines(t, path)
	if len(got) != 6 || got[0] != "2026-10-08T10:00:00Z hint s-aaaaaaaaaaaa 3 unread" ||
		!strings.HasSuffix(got[5], " restart - binary is now 0.2.0") {
		t.Fatalf("log:\n%s", strings.Join(got, "\n"))
	}
}

func TestLogRepeatedErrorOnce(t *testing.T) {
	// The same error for the same key is written once; a success clears it, so a recurrence is
	// visible; a different error for the key is written (R-WRK-022, the log-storm regression).
	// covers: R-WRK-022
	l, path := newLog(t)
	for range 100 {
		l.Error("s-aaaaaaaaaaaa", errors.New("pane wF:p1 not found"))
	}
	if got := lines(t, path); len(got) != 1 ||
		!strings.HasSuffix(got[0], " error s-aaaaaaaaaaaa pane wF:p1 not found") {
		t.Fatalf("100 identical errors wrote %d lines: %v", len(got), got)
	}
	l.Error("s-aaaaaaaaaaaa", errors.New("another"))
	l.Error("s-bbbbbbbbbbbb", errors.New("pane wF:p1 not found"))
	l.Clear("s-aaaaaaaaaaaa")
	l.Error("s-aaaaaaaaaaaa", errors.New("another"))
	if got := lines(t, path); len(got) != 4 {
		t.Fatalf("want 4 lines (first, another, other key, after clear), got %d: %v", len(got), got)
	}
}

func TestLogRotatesAt10MB(t *testing.T) {
	// covers: R-WRK-026
	l, path := newLog(t)
	line := strings.Repeat("x", 1000)
	for range 25 * 1024 {
		l.Action("hint", "s-aaaaaaaaaaaa", line)
	}
	fi, err := os.Stat(path)
	if err != nil || fi.Size() >= worker.LogMax {
		t.Fatalf("worker.log = %v, %v; want under %d bytes", fi, err, worker.LogMax)
	}
	for _, p := range []string{path + ".1", path + ".2"} {
		if fi, err := os.Stat(p); err != nil || fi.Size() < worker.LogMax-2000 {
			t.Fatalf("%s: %v, %v; want a full rotated file", p, fi, err)
		}
	}
	if _, err := os.Stat(path + ".3"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("worker.log.3 exists: %v", err)
	}
}

func TestLogKeepsTwoRotatedFiles(t *testing.T) {
	// covers: R-WRK-026
	l, path := newLog(t)
	l.Max = 500
	for range 50 {
		l.Action("hint", "s-aaaaaaaaaaaa", strings.Repeat("x", 100))
	}
	for _, p := range []string{path + ".1", path + ".2"} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
	}
	if _, err := os.Stat(path + ".3"); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("worker.log.3 exists: %v", err)
	}
}

func TestLogFailureNeverStopsTheCaller(t *testing.T) {
	// Review Focus 4: an unwritable log is reported once on Err and the caller goes on.
	// covers: R-WRK-021
	if os.Getuid() == 0 {
		t.Skip("root writes anywhere")
	}
	var stderr bytes.Buffer
	l := &worker.Log{
		Path: filepath.Join(t.TempDir(), "missing", "worker.log"),
		Now:  func() time.Time { return t0 },
		Err:  &stderr,
	}
	if err := os.MkdirAll(filepath.Dir(l.Path), 0o500); err != nil {
		t.Fatal(err)
	}
	l.Action("hint", "s-aaaaaaaaaaaa", "x")
	l.Action("hint", "s-aaaaaaaaaaaa", "y")
	l.Error("pass", errors.New("boom"))
	if n := strings.Count(stderr.String(), "\n"); n != 1 {
		t.Fatalf("stderr reported %d times, want once:\n%s", n, stderr.String())
	}
}

func TestLogFoldsAMultiLineErrorOntoOneLine(t *testing.T) {
	// covers: R-WRK-021
	path := filepath.Join(t.TempDir(), "worker.log")
	l := &worker.Log{Path: path, Now: func() time.Time { return time.Unix(0, 0) }}
	l.Error("exec", errors.New("first\nsecond\r\nthird"))
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := strings.Count(string(b), "\n"); got != 1 {
		t.Fatalf("lines = %d, want 1: %q", got, b)
	}
}
