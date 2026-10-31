package store_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestMigrationSixKeepsItemsAndAcceptsTheNewDispositions(t *testing.T) {
	// A version-5 store migrates forward with its items intact, and the rebuilt table takes the
	// returned and withdrawn dispositions it refused before.
	// covers: R-MSG-046, R-MSG-063
	path := freshPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	db := raw(t, path)
	for _, q := range store.Migrations[:5] {
		if _, err := db.ExecContext(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []string{
		`INSERT INTO sessions (id, scope, created_at) VALUES ('` + sA + `', 'root', '2026-10-08T00:00:00Z')`,
		`INSERT INTO sessions (id, parent, scope, created_at) VALUES ('` + sB + `', '` + sA + `', 'c', '2026-10-08T00:00:00Z')`,
		`INSERT INTO items (id, key, sender, recipient, intent, body, created_at, disposition, answer, retired_at, probe)
		 VALUES ('m-old', 'k', '` + sA + `', '` + sB + `', 'request', 'kept', '2026-10-08T00:00:00Z',
		 'deferred', 'later', '2026-10-09T00:00:00Z', 'probes/x.md')`,
		`PRAGMA user_version = 5`,
	} {
		if _, err := db.ExecContext(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := db.ExecContext(t.Context(), `UPDATE items SET disposition = 'withdrawn'`); err == nil {
		t.Fatal("a version-5 store accepted withdrawn: the test does not exercise the rebuild")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	s := mustOpen(t, path)
	if got := userVersion(t, path); got != store.Version() {
		t.Fatalf("user_version = %d, want %d", got, store.Version())
	}
	it, err := s.Item("m-old")
	if err != nil || it.Body != "kept" || it.Disposition != store.Deferred || it.Answer != "later" ||
		it.Probe != "probes/x.md" || it.Sender != sA || it.Recipient != sB {
		t.Fatalf("legacy item = %+v, %v", it, err)
	}
	if _, err = s.Withdraw("m-old", "done myself", now); err != nil {
		t.Fatalf("Withdraw on a migrated store: %v", err)
	}
	id, _, err := s.Send(sA, sB, store.IntentRequest, "fresh", false, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Reply(id, sB, "yours again", store.Returned, now); err != nil {
		t.Fatalf("Reply as returned on a migrated store: %v", err)
	}
}

func TestWithdrawSettlesAndNotifiesTheRecipient(t *testing.T) {
	// covers: R-MSG-060, R-MSG-061
	s := tree(t)
	req, _, err := s.Send(sA, sB, store.IntentRequest, "do x", false, now)
	if err != nil {
		t.Fatal(err)
	}
	notice, err := s.Withdraw(req, "did it myself", now)
	if err != nil {
		t.Fatal(err)
	}
	it, _ := s.Item(req)
	if it.Open() || it.Disposition != store.Withdrawn || it.Answer != "did it myself" {
		t.Fatalf("withdrawn item = %+v", it)
	}
	n, _ := s.Item(notice)
	if n.Intent != store.IntentReply || n.Sender != sA || n.Recipient != sB || n.Body != "withdrew "+req+": did it myself" {
		t.Fatalf("notice = %+v", n)
	}
	if obl, _ := s.Obligations(sB); len(obl) != 0 {
		t.Fatalf("a withdrawn request still obliges: %v", ids(obl))
	}
	if _, err = s.Withdraw(req, "again", now); !errors.Is(err, store.ErrDisposed) {
		t.Fatalf("second Withdraw = %v, want ErrDisposed", err)
	}
	if _, err = s.Withdraw(notice, "a reply", now); !errors.Is(err, store.ErrReplyToReply) {
		t.Fatalf("Withdraw of a reply = %v, want ErrReplyToReply", err)
	}
}
