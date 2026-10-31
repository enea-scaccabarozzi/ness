package update_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/update"
)

func TestFailureRecord(t *testing.T) {
	// covers: R-UPD-081 — log only when the cause changes, across processes
	p := filepath.Join(t.TempDir(), "upgrade-failed.json")
	t0 := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	// rejects: a missing record reported as an error
	if _, ok, err := update.ReadFailure(p); err != nil || ok {
		t.Fatalf("missing: %v, %v", ok, err)
	}
	for i, c := range []struct {
		cause   string
		changed bool
	}{
		{"sync conflict", true},  // rejects: no record treated as unchanged
		{"sync conflict", false}, // rejects: always reporting a change (logs every pass)
		{"commit failed", true},  // rejects: changed computed from the version, not the cause
	} {
		changed, err := update.RecordFailure(p, update.Failure{Version: "0.2.0", Cause: c.cause, At: t0.Add(time.Duration(i) * time.Hour)})
		if err != nil || changed != c.changed {
			t.Errorf("%d: changed = %v, %v", i, changed, err)
		}
	}
	if f, ok, err := update.ReadFailure(p); err != nil || !ok || f.Cause != "commit failed" || f.Version != "0.2.0" {
		t.Fatalf("%+v %v %v", f, ok, err)
	}
	if err := update.ClearFailure(p); err != nil {
		t.Fatal(err)
	}
	// rejects: ENOENT surfaced on a clear with nothing recorded
	if err := update.ClearFailure(p); err != nil {
		t.Fatal("second clear:", err)
	}
	if _, ok, err := update.ReadFailure(p); err != nil || ok {
		t.Fatalf("still recorded: %v, %v", ok, err)
	}
}

func TestFailureRecordCorrupt(t *testing.T) {
	// covers: R-UPD-080, R-UPD-081 — a corrupt record heals instead of disabling the back-off
	p := filepath.Join(t.TempDir(), "install-failed.json")
	if err := os.WriteFile(p, []byte("{not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	t0 := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	// rejects: the read error returned before the write, so the record never heals
	changed, err := update.RecordFailure(p, update.Failure{Version: "0.2.0", Cause: "checksum mismatch", At: t0})
	if err != nil || !changed {
		t.Fatalf("changed = %v, %v", changed, err)
	}
	if f, ok, err := update.ReadFailure(p); err != nil || !ok || f.Cause != "checksum mismatch" || !f.At.Equal(t0) {
		t.Fatalf("%+v %v %v", f, ok, err)
	}
}
