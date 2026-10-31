package sync_test

import (
	"strings"
	"testing"

	ness "github.com/enea-scaccabarozzi/ness"
	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
)

const fixture = `# Changelog

## [0.3.2](https://example.com/compare/v0.3.1...v0.3.2) (2026-10-20)

### Features

* stats

### Running sessions

* nothing to do

## Unreleased notes that are not a version

ignored text

## v0.3.1 (2026-10-15)

### Running sessions

* resume each root after sync

## 0.3.0 (2026-10-10)

### Running sessions

* first
`

func TestNotesParseVersionsAndRequireRunningSessions(t *testing.T) {
	// covers: R-SYNC-089
	notes, err := nsync.Notes(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 3 || notes[0].Version != "0.3.2" || notes[1].Version != "0.3.1" || notes[2].Version != "0.3.0" {
		t.Fatalf("Notes = %+v", notes)
	}
	if !strings.Contains(notes[1].Body, "### Running sessions\n\n* resume each root after sync") {
		t.Fatalf("body of 0.3.1 = %q", notes[1].Body)
	}
	if _, err = nsync.Notes("## 0.1.0\n\n### Features\n\n* x\n"); err == nil || !strings.Contains(err.Error(), "0.1.0") {
		t.Fatalf("a version without Running sessions must be an error, got %v", err)
	}
	embedded, err := nsync.Notes(ness.Changelog)
	if err != nil || len(embedded) == 0 {
		t.Fatalf("the embedded CHANGELOG: %d versions, %v", len(embedded), err)
	}
}

func TestNotesSkipsNonVersionHeadings(t *testing.T) {
	// "## Unreleased" and its text are skipped, never a crash or a version.
	notes, err := nsync.Notes(fixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range notes {
		if strings.Contains(n.Body, "ignored text") {
			t.Fatalf("non-version text leaked into %s", n.Version)
		}
	}
	if _, err = nsync.Notes("## Unreleased\n\ntext\n\n## Notes\n"); err != nil {
		t.Fatal(err)
	}
}

func between(t *testing.T, notes []nsync.Note, from, to string) []nsync.Note {
	t.Helper()
	got, err := nsync.Between(notes, from, to)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestBetweenIsOldestFirstAndBounded(t *testing.T) {
	// covers: R-SYNC-032, R-SYNC-033
	notes, _ := nsync.Notes(fixture)
	got := between(t, notes, "0.3.0", "0.3.2")
	if len(got) != 2 || got[0].Version != "0.3.1" || got[1].Version != "0.3.2" {
		t.Fatalf("Between(0.3.0, 0.3.2) = %+v", got)
	}
	if got = between(t, notes, "0.3.2", "0.3.2"); len(got) != 0 {
		t.Fatalf("equal versions print nothing: %+v", got)
	}
	if got = between(t, notes, "0.3.1", "0.3.10"); len(got) != 1 || got[0].Version != "0.3.2" {
		t.Fatalf("numeric comparison: %+v", got)
	}
	if got = between(t, notes, "0.3.2", "0.3.1"); len(got) != 0 {
		t.Fatalf("a downgrade prints nothing: %+v", got)
	}
	if got = between(t, notes, "0.2.9", "0.3.1"); len(got) != 2 || got[0].Version != "0.3.0" {
		t.Fatalf("an unknown from is a range: %+v", got)
	}
}

func TestBetweenSortsAndOrdersPrereleases(t *testing.T) {
	notes, err := nsync.Notes("## 0.2.0\n### Running sessions\n## 0.1.0\n### Running sessions\n## 0.3.0\n### Running sessions\n")
	if err != nil {
		t.Fatal(err)
	}
	got := between(t, notes, "0.0.1", "0.3.0")
	if len(got) != 3 || got[0].Version != "0.1.0" || got[1].Version != "0.2.0" || got[2].Version != "0.3.0" {
		t.Fatalf("out-of-order file must print sorted: %+v", got)
	}
	if got = between(t, notes, "0.1.0", "0.2.0-rc.1"); len(got) != 0 {
		t.Fatalf("an rc is below its release: %+v", got)
	}
	if got = between(t, notes, "0.2.0-rc.1", "0.2.0"); len(got) != 1 || got[0].Version != "0.2.0" {
		t.Fatalf("rc to release prints the release: %+v", got)
	}
	for _, bad := range [][2]string{{"dev", "0.2.0"}, {"0.1.0", "latest"}} {
		if _, err = nsync.Between(notes, bad[0], bad[1]); err == nil {
			t.Fatalf("Between(%v) must be an error", bad)
		}
	}
}
