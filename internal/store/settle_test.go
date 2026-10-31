package store_test

import (
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestSettleFromDisposesOnlyWhatTheSenderSentTheRecipient(t *testing.T) {
	// covers: R-SESS-123
	s := tree(t)
	send := func(from, to, intent, body string) string {
		t.Helper()
		id, _, err := s.Send(from, to, intent, body, false, now)
		if err != nil {
			t.Fatal(err)
		}
		return id
	}
	hand := send(sB, sA, store.IntentHandoff, "take it")
	dec := send(sB, sA, store.IntentDecision, "which way")
	fyi := send(sB, sA, store.IntentFYI, "news")
	deferred := send(sB, sA, store.IntentRequest, "later")
	done := send(sB, sA, store.IntentRequest, "done already")
	toC := send(sB, sC, store.IntentRequest, "to a sibling")
	fromC := send(sC, sA, store.IntentRequest, "from another child")
	toB := send(sA, sB, store.IntentRequest, "the other direction")
	if _, err := s.Reply(deferred, sA, "not now", store.Deferred, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reply(done, sA, "ok", store.Declined, now); err != nil {
		t.Fatal(err)
	}
	before := len(mustListed(t, s, sA)) + len(mustListed(t, s, sB)) + len(mustListed(t, s, sC))

	at := now.Add(time.Hour)
	n, err := s.SettleFrom(sB, sA, store.Accepted, "closed with the child", at)
	if err != nil || n != 4 {
		t.Fatalf("SettleFrom = %d, %v; want the 4 open items (a deferred one included)", n, err)
	}
	for _, id := range []string{hand, dec, fyi, deferred} {
		it, ierr := s.Item(id)
		if ierr != nil || it.Disposition != store.Accepted || it.Answer != "closed with the child" || !it.DisposedAt.Equal(at) {
			t.Fatalf("settled item %+v, %v; want accepted, answered, disposed at %v", it, ierr, at)
		}
	}
	if it, _ := s.Item(done); it.Disposition != store.Declined || it.Answer != "ok" {
		t.Fatalf("an already disposed item was rewritten: %+v", it)
	}
	for _, id := range []string{toC, fromC, toB} {
		if it, _ := s.Item(id); !it.Open() || it.Answer != "" {
			t.Fatalf("an item outside sender->recipient was touched: %+v", it)
		}
	}
	if obliged, oerr := s.Obligations(sA); oerr != nil || len(obliged) != 1 || obliged[0].ID != fromC {
		t.Fatalf("manager's obligations = %v, %v; want only the other child's request", ids(obliged), oerr)
	}
	if after := len(mustListed(t, s, sA)) + len(mustListed(t, s, sB)) + len(mustListed(t, s, sC)); after > before {
		t.Fatalf("settling created items: %d listed before, %d after", before, after)
	}
	if n, err = s.SettleFrom(sB, sA, store.Accepted, "closed with the child", at); err != nil || n != 0 {
		t.Fatalf("a second settle = %d, %v; want 0", n, err)
	}
}

func mustListed(t *testing.T, s *store.Store, who string) []store.Item {
	t.Helper()
	items, err := s.Listed(who)
	if err != nil {
		t.Fatal(err)
	}
	return items
}
