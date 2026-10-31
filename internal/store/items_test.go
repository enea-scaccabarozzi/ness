package store_test

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

const (
	sA = "s-aaaaaaaaaaaa"
	sB = "s-bbbbbbbbbbbb"
	sC = "s-cccccccccccc"
)

// tree opens a store with root A and children B and C of A.
func tree(t *testing.T) *store.Store {
	t.Helper()
	s := mustOpen(t, freshPath(t))
	for _, row := range [][2]string{{sA, store.Operator}, {sB, sA}, {sC, sA}} {
		if err := s.AddSession(row[0], "scope", row[1], now); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func ids(items []store.Item) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

func TestSendIsIdempotentPerContent(t *testing.T) {
	// covers: R-MSG-014, R-MSG-015
	s := tree(t)
	id1, created, err := s.Send(sB, sA, store.IntentRequest, "same", false, now)
	if err != nil || !created {
		t.Fatalf("first send: %v, created %v", err, created)
	}
	id2, created, err := s.Send(sB, sA, store.IntentRequest, "same", true, now.Add(time.Hour))
	if err != nil || created || id2 != id1 {
		t.Fatalf("repeat: %v, created %v, id %s vs %s", err, created, id2, id1)
	}
	if _, err = s.Reply(id1, sA, "done", store.Resolved, now); err != nil {
		t.Fatal(err)
	}
	id3, created, err := s.Send(sB, sA, store.IntentRequest, "same", false, now)
	if err != nil || created || id3 != id1 {
		t.Fatalf("repeat after disposal: %v, created %v (the key is permanent)", err, created)
	}
	_, created, err = s.Send(sB, sA, store.IntentRequest, "other", false, now)
	if err != nil || !created {
		t.Fatalf("different body: %v, created %v", err, created)
	}
	_, created, err = s.Send(sB, sA, store.IntentFYI, "same", false, now)
	if err != nil || !created {
		t.Fatalf("same body, other intent: %v, created %v", err, created)
	}
	// The key includes the recipient: the same body and intent to two children is two items.
	toB, created, err := s.Send(sA, sB, store.IntentRequest, "same", false, now)
	if err != nil || !created {
		t.Fatalf("same body to B: %v, created %v", err, created)
	}
	toC, created, err := s.Send(sA, sC, store.IntentRequest, "same", false, now)
	if err != nil || !created || toC == toB {
		t.Fatalf("same body to C: %v, created %v, ids %s %s", err, created, toB, toC)
	}
	if _, _, err = s.Send(sB, "s-nope", store.IntentRequest, "x", false, now); err == nil {
		t.Fatal("a recipient that does not exist was accepted")
	}
}

func TestInboxListsOpenItemsAndMarksThemPulled(t *testing.T) {
	// covers: R-MSG-036, R-MSG-041, R-MSG-043, R-MSG-022, R-MSG-030
	s := tree(t)
	req, _, _ := s.Send(sB, sA, store.IntentRequest, "req", false, now)
	fyi, _, _ := s.Send(sB, sA, store.IntentFYI, "fyi", false, now.Add(time.Second))
	dec, _, _ := s.Send(sC, sA, store.IntentDecision, "dec", true, now.Add(2*time.Second))
	if _, _, err := s.Send(sA, sB, store.IntentRequest, "not mine", false, now); err != nil {
		t.Fatal(err)
	}
	listed, err := s.Listed(sA)
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(ids(listed)); got != fmt.Sprint([]string{req, fyi, dec}) {
		t.Fatalf("Listed = %s", got)
	}
	if !listed[0].PulledAt.IsZero() {
		t.Fatal("Listed must not pull")
	}
	items, err := s.Inbox(sA, now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || !items[0].PulledAt.Equal(now.Add(time.Minute)) || !items[2].Urgent {
		t.Fatalf("Inbox = %+v", items)
	}
	again, _ := s.Inbox(sA, now.Add(2*time.Minute))
	if len(again) != 3 || !again[0].PulledAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("a pulled item must stay open and keep its first pull: %+v", again)
	}

	// An fyi leaves the list once shown; a request does not.
	for _, id := range []string{fyi, req} {
		if err = s.MarkShown(id, now.Add(3*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	listed, _ = s.Listed(sA)
	if got := fmt.Sprint(ids(listed)); got != fmt.Sprint([]string{req, dec}) {
		t.Fatalf("after show: %s", got)
	}
	// A deferred item stays listed and marked; a resolved one leaves.
	if _, err = s.Reply(req, sA, "later", store.Deferred, now); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Reply(dec, sA, "yes", store.Accepted, now); err != nil {
		t.Fatal(err)
	}
	listed, _ = s.Listed(sA)
	if len(listed) != 1 || listed[0].ID != req || listed[0].Disposition != store.Deferred {
		t.Fatalf("after replies: %+v", listed)
	}
	// The answers arrived as reply items for the senders.
	b, _ := s.Listed(sB)
	if len(b) != 2 || b[1].Intent != store.IntentReply || b[1].Body != "later" || b[1].Sender != sA {
		t.Fatalf("B's inbox: %+v", b)
	}
	if _, err = s.Item("m-nope"); !errors.Is(err, store.ErrNoItem) {
		t.Fatalf("unknown item: %v", err)
	}
	if err = s.MarkShown("m-nope", now); !errors.Is(err, store.ErrNoItem) {
		t.Fatalf("MarkShown unknown: %v", err)
	}
	// An item read with --show before any listing counts as pulled: the worker must not hint an
	// item its recipient has already read (R-MSG-036).
	early, _, _ := s.Send(sC, sA, store.IntentRequest, "shown first", false, now)
	if err = s.MarkShown(early, now.Add(4*time.Minute)); err != nil {
		t.Fatal(err)
	}
	if it, _ := s.Item(early); !it.PulledAt.Equal(now.Add(4 * time.Minute)) {
		t.Fatalf("shown but not pulled: %+v", it)
	}
}

func TestReplyDisposesOnceAndAnswersTheSender(t *testing.T) {
	// covers: R-MSG-020, R-MSG-022, R-MSG-025, R-MSG-043
	s := tree(t)
	req, _, _ := s.Send(sB, sA, store.IntentRequest, "may I?", false, now)
	later := now.Add(time.Minute)
	replyID, err := s.Reply(req, sA, "yes", store.Resolved, later)
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.Item(req)
	if err != nil || it.Disposition != store.Resolved || it.Answer != "yes" || !it.DisposedAt.Equal(later) {
		t.Fatalf("Item = %+v, %v", it, err)
	}
	rep, err := s.Item(replyID)
	if err != nil || rep.Intent != store.IntentReply || rep.Recipient != sB || rep.Sender != sA || rep.Body != "yes" {
		t.Fatalf("reply item = %+v, %v", rep, err)
	}
	if rep.Obligation() {
		t.Fatal("a reply item must carry no obligation")
	}
	if _, err = s.Reply(req, sA, "no", store.Declined, later); !errors.Is(err, store.ErrDisposed) {
		t.Fatalf("second reply: %v, want ErrDisposed", err)
	}
	it, _ = s.Item(req)
	if it.Answer != "yes" {
		t.Fatalf("first answer changed to %q", it.Answer)
	}
	// Deferred may be answered again; the operator may reply (empty id).
	q, _, _ := s.Send(sA, store.Operator, store.IntentRequest, "op?", false, now)
	if _, err = s.Reply(q, store.Operator, "wait", store.Deferred, now); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Reply(q, store.Operator, "go", store.Resolved, now); err != nil {
		t.Fatal(err)
	}
	a, _ := s.Listed(sA)
	if len(a) != 2 || a[0].Sender != store.Operator || a[1].Body != "go" {
		t.Fatalf("A's inbox: %+v", a)
	}
	if _, err = s.Reply("m-nope", store.Operator, "x", store.Resolved, now); !errors.Is(err, store.ErrNoItem) {
		t.Fatalf("Reply unknown: %v", err)
	}
}

func TestObligations(t *testing.T) {
	// request, decision and handoff oblige until disposed; deferred still does; fyi and reply never.
	// covers: R-MSG-040, R-MSG-041, R-MSG-043, R-MSG-022
	s := tree(t)
	var want []string
	for i, intent := range []string{store.IntentRequest, store.IntentDecision, store.IntentHandoff} {
		id, _, _ := s.Send(sB, sA, intent, intent, false, now.Add(time.Duration(i)*time.Second))
		want = append(want, id)
	}
	fyi, _, _ := s.Send(sB, sA, store.IntentFYI, "fyi", false, now)
	got, err := s.Obligations(sA)
	if err != nil || fmt.Sprint(ids(got)) != fmt.Sprint(want) {
		t.Fatalf("Obligations = %v, %v; want %v", ids(got), err, want)
	}
	if _, err = s.Reply(want[0], sA, "later", store.Deferred, now); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Reply(want[1], sA, "ok", store.Accepted, now); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Obligations(sA)
	if fmt.Sprint(ids(got)) != fmt.Sprint([]string{want[0], want[2]}) {
		t.Fatalf("after replies: %v", ids(got))
	}
	it, _ := s.Item(fyi)
	if it.Obligation() || !it.Open() {
		t.Fatalf("fyi = %+v", it)
	}
}

func TestConcurrentSendsNeitherLoseNorDuplicate(t *testing.T) {
	// Forty senders (one store handle each, like one process each) send twenty distinct bodies
	// twice: exactly twenty items exist.
	// covers: R-MSG-003
	path := freshPath(t)
	seed := mustOpen(t, path)
	if err := seed.AddSession(sA, "root", store.Operator, now); err != nil {
		t.Fatal(err)
	}
	if err := seed.AddSession(sB, "child", sA, now); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for i := range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := store.Open(path)
			if err != nil {
				errs <- err
				return
			}
			defer func() { _ = s.Close() }()
			body := fmt.Sprintf("body %02d", i%20)
			if _, _, err = s.Send(sB, sA, store.IntentRequest, body, false, now); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	items, err := seed.Listed(sA)
	if err != nil {
		t.Fatal(err)
	}
	bodies := map[string]bool{}
	for _, it := range items {
		bodies[it.Body] = true
	}
	if len(items) != 20 || len(bodies) != 20 {
		t.Fatalf("%d items, %d distinct bodies; want 20 and 20", len(items), len(bodies))
	}
}

func TestInboxOrdersItemsStampedWithinOneSecond(t *testing.T) {
	// A whole second must sort before its own fractions: fixed-width stamps.
	// covers: R-MSG-030
	s := tree(t)
	whole := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	first, _, _ := s.Send(sB, sA, store.IntentRequest, "first", false, whole)
	second, _, _ := s.Send(sB, sA, store.IntentRequest, "second", false, whole.Add(500*time.Millisecond))
	got, err := s.Inbox(sA, whole.Add(time.Second))
	if err != nil || fmt.Sprint(ids(got)) != fmt.Sprint([]string{first, second}) {
		t.Fatalf("Inbox = %v, %v; want %v", ids(got), err, []string{first, second})
	}
}

func TestUnreadIsListedAndNeverPulled(t *testing.T) {
	// A hint covers the listed items the session has not pulled; one pull removes an item from
	// Unread for good while Listed still has it (R-MSG-036).
	// covers: R-MSG-036, R-WRK-012
	s := tree(t)
	a, _, _ := s.Send(sB, sA, store.IntentRequest, "one", false, now)
	b, _, _ := s.Send(sB, sA, store.IntentFYI, "two", false, now.Add(time.Second))
	if got, err := s.Unread(sA); err != nil || len(got) != 2 || got[0].ID != a || got[1].ID != b {
		t.Fatalf("Unread = %v, %v", ids(got), err)
	}
	if _, err := s.Inbox(sA, now.Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	if got, _ := s.Unread(sA); len(got) != 0 {
		t.Fatalf("Unread after a pull = %v", ids(got))
	}
	if got, _ := s.Listed(sA); len(got) != 2 {
		t.Fatalf("Listed after a pull = %v", ids(got))
	}
	c, _, _ := s.Send(sB, sA, store.IntentDecision, "three", true, now.Add(2*time.Second))
	if got, _ := s.Unread(sA); len(got) != 1 || got[0].ID != c || !got[0].Urgent {
		t.Fatalf("Unread after a new item = %v", ids(got))
	}
}

func TestRetireDeadRetiresItemsToClosedOrUnknownSessions(t *testing.T) {
	// covers: R-WRK-020
	s := tree(t)
	live, _, _ := s.Send(sB, sA, store.IntentRequest, "to a", false, now)
	dead, _, _ := s.Send(sA, sC, store.IntentRequest, "to c", false, now)
	deferred, _, _ := s.Send(sA, sC, store.IntentRequest, "deferred to c", false, now)
	if _, err := s.Reply(deferred, sC, "later", store.Deferred, now); err != nil {
		t.Fatal(err)
	}
	toOp, _, _ := s.Send(sA, store.Operator, store.IntentRequest, "to the operator", false, now)
	if err := s.RecordClose(sC, "", "", now); err != nil {
		t.Fatal(err)
	}
	got, err := s.RetireDead(now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{dead, deferred}
	slices.Sort(want)
	var gotIDs []string
	for _, r := range got {
		gotIDs = append(gotIDs, r.ID)
		if it, _ := s.Item(r.ID); it.Recipient != r.Recipient || r.Recipient == "" {
			t.Fatalf("retired %+v, item recipient %q", r, it.Recipient)
		}
	}
	if !slices.Equal(gotIDs, want) {
		t.Fatalf("RetireDead = %v, want %v", gotIDs, want)
	}
	for _, id := range []string{live, toOp} {
		if it, _ := s.Item(id); !it.Open() {
			t.Fatalf("%s was touched", id)
		}
	}
	if items, _ := s.Listed(sC); len(items) != 0 {
		t.Fatalf("closed session still lists %v", ids(items))
	}
	if again, _ := s.RetireDead(now.Add(2 * time.Minute)); len(again) != 0 {
		t.Fatalf("second RetireDead = %v, want nothing", again)
	}
	if dead, _ := s.DeadRoutes(); len(dead) != 0 {
		t.Fatalf("DeadRoutes after retirement = %v", dead)
	}
}

func TestReplyToAReplySettlesWithoutBouncing(t *testing.T) {
	// Rejects a store that refuses the recipient settling a reply, or that answers it with another
	// reply: two sessions could then bounce replies forever.
	// covers: R-MSG-050
	s := tree(t)
	req, _, _ := s.Send(sB, sA, store.IntentRequest, "q", false, now)
	reply, err := s.Reply(req, sA, "a", store.Resolved, now)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Reply(reply, sA, "hidden", store.Resolved, now); !errors.Is(err, store.ErrNotRecipient) {
		t.Fatalf("the replier settling its own reply: %v, want ErrNotRecipient", err)
	}
	back, err := s.Reply(reply, sB, "thanks", store.Resolved, now)
	if err != nil || back != "" {
		t.Fatalf("Reply on a reply = %q, %v; want settled, nothing delivered", back, err)
	}
	if it, _ := s.Item(reply); it.Disposition != store.Resolved {
		t.Fatalf("reply disposition = %q, want resolved", it.Disposition)
	}
	if items, _ := s.Listed(sA); len(items) != 0 {
		t.Fatalf("the replier received %v", ids(items))
	}
}
