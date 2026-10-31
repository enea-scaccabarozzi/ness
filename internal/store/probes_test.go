package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestMigrationFiveAddsProbes(t *testing.T) {
	// A version-4 store opens, migrates to the current version and serves probes and items.probe.
	// covers: R-MSG-046, R-PRB-066
	path := freshPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	db := raw(t, path)
	for _, q := range store.Migrations[:4] {
		if _, err := db.ExecContext(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []string{
		`INSERT INTO sessions (id, scope, created_at) VALUES ('` + sA + `', 'root', '2026-10-08T00:00:00Z')`,
		`INSERT INTO items (id, key, sender, recipient, intent, body, created_at)
		 VALUES ('m-old', 'k', NULL, '` + sA + `', 'fyi', 'kept', '2026-10-08T00:00:00Z')`,
		`PRAGMA user_version = 4`,
	} {
		if _, err := db.ExecContext(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s := mustOpen(t, path)
	if got := userVersion(t, path); got != store.Version() {
		t.Fatalf("user_version = %d, want %d", got, store.Version())
	}
	if it, err := s.Item("m-old"); err != nil || it.Probe != "" || it.Body != "kept" {
		t.Fatalf("legacy item = %+v, %v", it, err)
	}
	p, err := s.StartProbe("probes/x.md", sA, "", false, now, now)
	if err != nil || p.State != store.ProbeActive || p.NotifiedState != "" || p.ConfirmedState != "" {
		t.Fatalf("StartProbe on a migrated store = %+v, %v", p, err)
	}
}

func TestStartProbeRulesByPath(t *testing.T) {
	// A fresh start creates; the owner's own restart keeps the id and restarts the window unless
	// kept; a paused probe at the path is adopted by the caller; another session's start at the
	// same path is a second probe (R-PRB-060, R-PRB-062, R-PRB-071, R-PRB-084).
	// covers: R-PRB-060, R-PRB-062, R-PRB-071, R-PRB-084
	s := tree(t)
	p1, err := s.StartProbe("probes/x.md", sA, "", false, now, now)
	if err != nil || p1.Owner != sA || p1.State != store.ProbeActive || !p1.StartedAt.Equal(now) || !p1.NextRunAt.Equal(now) {
		t.Fatalf("StartProbe = %+v, %v", p1, err)
	}
	later := now.Add(24 * time.Hour)
	again, err := s.StartProbe("probes/x.md", sA, "", true, later, later)
	if err != nil || again.ID != p1.ID || !again.StartedAt.Equal(now) {
		t.Fatalf("--keep-window restart: %+v, %v", again, err)
	}
	if again, _ = s.StartProbe("probes/x.md", sA, "", false, later, later); !again.StartedAt.Equal(later) {
		t.Fatalf("restart without --keep-window keeps the old window: %+v", again)
	}
	p2, err := s.StartProbe("probes/x.md", sB, "", false, now, now)
	if err != nil || p2.ID == p1.ID {
		t.Fatalf("a second owner: %+v, %v", p2, err)
	}
	if err = s.SetProbeState(p2.ID, store.ProbePaused, "owner closed", now); err != nil {
		t.Fatal(err)
	}
	adopted, err := s.StartProbe("probes/x.md", sC, "", true, later, later)
	if err != nil || adopted.ID != p2.ID || adopted.Owner != sC || adopted.State != store.ProbeActive ||
		!adopted.StartedAt.Equal(now) {
		t.Fatalf("adopting the paused probe: %+v, %v", adopted, err)
	}
	list, err := s.Probes(false)
	if err != nil || len(list) != 2 || list[0].Owner != sA || list[1].Owner != sC {
		t.Fatalf("Probes = %+v, %v", list, err)
	}
	if _, err = s.ProbeOf("probes/x.md", sB, ""); !errors.Is(err, store.ErrNoProbe) {
		t.Fatalf("ProbeOf the old owner: %v", err)
	}
}

func TestStartStopStartAgain(t *testing.T) {
	// A stopped row does not block the same owner starting the path again: a new probe, and the
	// stopped one stays as history (R-PRB-062, R-PRB-084).
	// covers: R-PRB-062, R-PRB-084
	s := tree(t)
	p1, err := s.StartProbe("probes/x.md", sA, "", false, now, now)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.SetProbeState(p1.ID, store.ProbeStopped, "done", now); err != nil {
		t.Fatal(err)
	}
	p2, err := s.StartProbe("probes/x.md", sA, "", false, now, now)
	if err != nil || p2.ID == p1.ID || p2.State != store.ProbeActive {
		t.Fatalf("start after stop = %+v, %v", p2, err)
	}
	if got, err := s.ProbeOf("probes/x.md", sA, ""); err != nil || got.ID != p2.ID {
		t.Fatalf("ProbeOf = %+v, %v", got, err)
	}
	if all, err := s.Probes(true); err != nil || len(all) != 2 {
		t.Fatalf("Probes(all) = %+v, %v", all, err)
	}
}

func TestRecordRunVerdictAndStop(t *testing.T) {
	// covers: R-PRB-061, R-PRB-064, R-PRB-065, R-PRB-067
	s := tree(t)
	p, _ := s.StartProbe("probes/x.md", sA, "", false, now, now)
	run := store.ProbeRun{State: "RED", Evidence: "boom", Hash: "h1", Streak: 1, Runs: 1, At: now, Next: now.Add(time.Hour)}
	if err := s.RecordRun(p.ID, run); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordVerdict(p.ID, "red", "it is broken", "h1", now); err != nil {
		t.Fatal(err)
	}
	got, _ := s.Probe(p.ID)
	if got.LastState != "RED" || got.Evidence != "boom" || got.Streak != 1 || got.Runs != 1 ||
		!got.NextRunAt.Equal(now.Add(time.Hour)) || got.Verdict != "red" || got.VerdictNote != "it is broken" ||
		got.NeedsEdit != "h1" {
		t.Fatalf("after a run and a verdict: %+v", got)
	}
	run.Confirmed = "RED"
	if err := s.RecordRun(p.ID, run); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.Probe(p.ID); got.ConfirmedState != "RED" {
		t.Fatalf("confirmed state = %q", got.ConfirmedState)
	}
	run.Hash, run.Runs = "h2", 2
	if err := s.RecordRun(p.ID, run); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.Probe(p.ID); got.NeedsEdit != "" {
		t.Fatalf("a changed file keeps needs-edit: %+v", got)
	}
	if err := s.RecordRun(p.ID, store.ProbeRun{FileError: "no such file", At: now, Next: now.Add(time.Minute)}); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.Probe(p.ID); got.FileError != "no such file" || got.LastState != "RED" {
		t.Fatalf("a file error keeps the last state: %+v", got)
	}
	if err := s.SetNotifiedState(p.ID, "RED"); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.Probe(p.ID); got.NotifiedState != "RED" {
		t.Fatalf("notified state = %q", got.NotifiedState)
	}
	if err := s.SetProbeState(p.ID, store.ProbeStopped, "done", now); err != nil {
		t.Fatal(err)
	}
	if list, _ := s.Probes(false); len(list) != 0 {
		t.Fatalf("a stopped probe is listed: %+v", list)
	}
	if list, _ := s.Probes(true); len(list) != 1 || list[0].Note != "done" || list[0].StoppedAt.IsZero() {
		t.Fatalf("Probes(all) = %+v", list)
	}
	if err := s.MarkFinalWake(p.ID); err != nil {
		t.Fatal(err)
	}
	if got, _ = s.Probe(p.ID); !got.FinalWake {
		t.Fatal("final wake not marked")
	}
	if err := s.SetNotifiedState("p-nope0000", "RED"); !errors.Is(err, store.ErrNoProbe) {
		t.Fatalf("unknown probe: %v", err)
	}
}

func TestWakeItemsAndDispose(t *testing.T) {
	// A wake is a request from the owner to itself tagged with the probe; Dispose answers it
	// without a reply item; a second Dispose is ErrDisposed (R-PRB-038, R-PRB-064, R-PRB-066).
	// covers: R-PRB-038, R-PRB-064, R-PRB-066
	s := tree(t)
	p, _ := s.StartProbe("probes/x.md", sA, "", false, now, now)
	id, err := s.SendWake(p.ID, "probe probes/x.md run 1: RED\n\nscope", now)
	if err != nil {
		t.Fatal(err)
	}
	it, _ := s.Item(id)
	if it.Probe != p.ID || it.Sender != sA || it.Recipient != sA || it.Intent != store.IntentRequest || !it.Open() {
		t.Fatalf("wake = %+v", it)
	}
	if listed, _ := s.Listed(sA); len(listed) != 1 || listed[0].Probe != p.ID {
		t.Fatalf("Listed = %+v", listed)
	}
	plain, _, _ := s.Send(sB, sA, store.IntentRequest, "hello", false, now)
	if it, _ = s.Item(plain); it.Probe != "" {
		t.Fatalf("a plain item carries a probe: %+v", it)
	}
	if err = s.Dispose(id, store.Resolved, "green: fine", now); err != nil {
		t.Fatal(err)
	}
	if it, _ = s.Item(id); it.Disposition != store.Resolved || it.Answer != "green: fine" || it.DisposedAt.IsZero() {
		t.Fatalf("after Dispose = %+v", it)
	}
	if listed, _ := s.Listed(sA); len(listed) != 1 || listed[0].ID != plain {
		t.Fatalf("no reply item may appear: %+v", listed)
	}
	if err = s.Dispose(id, store.Resolved, "again", now); !errors.Is(err, store.ErrDisposed) {
		t.Fatalf("second Dispose: %v", err)
	}
	if err = s.Dispose("m-nope0000", store.Resolved, "x", now); !errors.Is(err, store.ErrNoItem) {
		t.Fatalf("unknown item: %v", err)
	}
}

func TestTransferProbes(t *testing.T) {
	// Closing an owner moves its probes to the manager; to the operator they pause; a manager
	// already watching the path keeps its own and the moved one stops (R-PRB-070).
	// covers: R-PRB-070
	s := tree(t) // sA root; sB, sC children of sA
	pb, _ := s.StartProbe("probes/x.md", sB, "", false, now, now)
	pb2, _ := s.StartProbe("probes/y.md", sB, "", false, now, now)
	pa, _ := s.StartProbe("probes/y.md", sA, "", false, now, now)
	n, err := s.TransferProbes(sB, sA, now)
	if err != nil || n != 2 {
		t.Fatalf("TransferProbes = %d, %v", n, err)
	}
	got, _ := s.Probe(pb.ID)
	if got.Owner != sA || got.State != store.ProbeActive {
		t.Fatalf("moved probe = %+v", got)
	}
	if got, _ = s.Probe(pb2.ID); got.State != store.ProbeStopped || got.Note != "merged into "+pa.ID {
		t.Fatalf("colliding probe = %+v", got)
	}
	if n, err = s.TransferProbes(sA, store.Operator, now); err != nil || n != 2 {
		t.Fatalf("to the operator: %d, %v", n, err)
	}
	if got, _ = s.Probe(pb.ID); got.State != store.ProbePaused || got.Owner != sA {
		t.Fatalf("paused probe keeps its last owner: %+v", got)
	}
}

func TestTransferActivatesAPausedProbe(t *testing.T) {
	// A pause that raced the close does not stick: a move to a session sets active (P-08).
	// covers: R-PRB-070, R-PRB-071
	s := tree(t)
	p, _ := s.StartProbe("probes/x.md", sB, "", false, now, now)
	if err := s.SetProbeState(p.ID, store.ProbePaused, "owner closed", now); err != nil {
		t.Fatal(err)
	}
	if n, err := s.TransferProbes(sB, sA, now); err != nil || n != 1 {
		t.Fatalf("TransferProbes = %d, %v", n, err)
	}
	if got, _ := s.Probe(p.ID); got.Owner != sA || got.State != store.ProbeActive {
		t.Fatalf("after the transfer = %+v", got)
	}
}

func TestTransferOntoAStoppedRow(t *testing.T) {
	// The target's stopped row at the path is history, not a collision: the probe moves (P-06).
	// covers: R-PRB-070, R-PRB-084
	s := tree(t)
	old, _ := s.StartProbe("probes/x.md", sA, "", false, now, now)
	if err := s.SetProbeState(old.ID, store.ProbeStopped, "done", now); err != nil {
		t.Fatal(err)
	}
	pb, _ := s.StartProbe("probes/x.md", sB, "", false, now, now)
	if n, err := s.TransferProbes(sB, sA, now); err != nil || n != 1 {
		t.Fatalf("TransferProbes = %d, %v", n, err)
	}
	if got, _ := s.Probe(pb.ID); got.Owner != sA || got.State != store.ProbeActive {
		t.Fatalf("moved probe = %+v", got)
	}
	if got, _ := s.Probe(old.ID); got.State != store.ProbeStopped || got.Note != "done" {
		t.Fatalf("the stopped row changed: %+v", got)
	}
}

func TestNeedsEditClearsOnlyWhenTheFlaggedContentChanges(t *testing.T) {
	// The flag compares with the hash at the verdict, not with the last run: a file edited before
	// the verdict stays flagged until it changes again (R-PRB-065).
	// covers: R-PRB-065
	s := tree(t)
	p, _ := s.StartProbe("probes/x.md", sA, "", false, now, now)
	run := func(hash string) store.Probe {
		t.Helper()
		if err := s.RecordRun(p.ID, store.ProbeRun{State: "RED", Hash: hash, At: now, Next: now}); err != nil {
			t.Fatal(err)
		}
		got, _ := s.Probe(p.ID)
		return got
	}
	run("A") // the file is edited to B after this run, then the verdict is recorded
	if err := s.RecordVerdict(p.ID, "red", "bug", "B", now); err != nil {
		t.Fatal(err)
	}
	if got := run("B"); got.NeedsEdit != "B" {
		t.Fatalf("a run with the flagged content cleared the flag: %+v", got)
	}
	if got := run("C"); got.NeedsEdit != "" {
		t.Fatalf("a run with changed content kept the flag: %+v", got)
	}
}

func TestSetProbeStateRefusesActive(t *testing.T) {
	// covers: R-PRB-062
	s := tree(t)
	p, _ := s.StartProbe("probes/x.md", sA, "", false, now, now)
	if err := s.SetProbeState(p.ID, store.ProbeActive, "", now); !errors.Is(err, store.ErrProbeActive) {
		t.Fatalf("SetProbeState(active) = %v", err)
	}
}

func TestSendWakeRefusesAProbeThatIsNotActive(t *testing.T) {
	// A wake goes to the row's current owner and only while the probe is active (R-PRB-038).
	// covers: R-PRB-038
	s := tree(t)
	p, _ := s.StartProbe("probes/x.md", sA, "", false, now, now)
	if err := s.SetProbeState(p.ID, store.ProbePaused, "owner closed", now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SendWake(p.ID, "run 1", now); !errors.Is(err, store.ErrProbeNotActive) {
		t.Fatalf("paused probe: %v", err)
	}
	if listed, _ := s.Listed(sA); len(listed) != 0 {
		t.Fatalf("Listed = %+v", listed)
	}
}

func TestStartProbeClearsFileErrorAndMergesOtherPausedRows(t *testing.T) {
	// A restart or adoption clears a stale file_error; the other paused rows at the path are
	// stopped, not left orphaned (R-PRB-071, R-PRB-084).
	// covers: R-PRB-071, R-PRB-084
	s := tree(t)
	p1, _ := s.StartProbe("probes/x.md", sA, "", false, now, now)
	if err := s.RecordRun(p1.ID, store.ProbeRun{FileError: "boom", Next: now}); err != nil {
		t.Fatal(err)
	}
	if again, _ := s.StartProbe("probes/x.md", sA, "", false, now, now); again.FileError != "" {
		t.Fatalf("restart kept file_error %q", again.FileError)
	}
	p2, _ := s.StartProbe("probes/x.md", sB, "", false, now, now)
	for _, id := range []string{p1.ID, p2.ID} {
		if err := s.SetProbeState(id, store.ProbePaused, "owner closed", now); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.StartProbe("probes/x.md", sC, "", false, now, now)
	if err != nil {
		t.Fatal(err)
	}
	other := p1.ID
	if got.ID == p1.ID {
		other = p2.ID
	}
	if o, _ := s.Probe(other); o.State != store.ProbeStopped || o.Note != "merged into "+got.ID {
		t.Fatalf("other paused row = %+v", o)
	}
	if list, _ := s.Probes(false); len(list) != 1 {
		t.Fatalf("Probes = %+v", list)
	}
}

func TestRecordRunForAChangedRowIsDropped(t *testing.T) {
	// A run made for a row (StartedAt, Due) writes only while the row is active and unchanged: a stop
	// or a restart in between leaves it alone (R-PRB-089).
	// covers: R-PRB-089
	s := tree(t)
	p, _ := s.StartProbe("probes/x.md", sA, "", false, now, now)
	run := store.ProbeRun{State: "RED", Runs: 1, At: now, Next: now.Add(time.Hour), StartedAt: p.StartedAt, Due: p.NextRunAt}
	if _, err := s.StartProbe("probes/x.md", sA, "", false, now.Add(time.Minute), now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordRun(p.ID, run); !errors.Is(err, store.ErrNoProbe) {
		t.Fatalf("RecordRun after a restart = %v", err)
	}
	cur, _ := s.Probe(p.ID)
	run.StartedAt, run.Due = cur.StartedAt, cur.NextRunAt
	if err := s.SetProbeState(p.ID, store.ProbeStopped, "x", now); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordRun(p.ID, run); !errors.Is(err, store.ErrNoProbe) {
		t.Fatalf("RecordRun after a stop = %v", err)
	}
	if got, _ := s.Probe(p.ID); got.Runs != 0 {
		t.Fatalf("runs = %d", got.Runs)
	}
}
