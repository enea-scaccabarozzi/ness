package store_test

import (
	"context"
	"fmt"
	"math/rand/v2"
	"sync"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestInvariantsHoldUnderConcurrentOps(t *testing.T) {
	// Sends, replies, a close with its retirement and worker passes (dead-route retirement,
	// unread reads, heartbeats) race on one store; the invariants are checked after with SQL.
	// Operation errors are expected (a reply racing another on the same item is ErrDisposed) and
	// are not what is tested.
	// covers: R-MSG-005
	path := freshPath(t)
	s := mustOpen(t, path)
	for _, row := range [][2]string{{sA, store.Operator}, {sB, sA}, {sC, sA}} {
		if err := s.AddSession(row[0], "scope", row[1], now); err != nil {
			t.Fatal(err)
		}
	}
	// Three handles on one file: each Store is a single connection, so contention needs several.
	handles := []*store.Store{s, mustOpen(t, path), mustOpen(t, path)}
	var wg sync.WaitGroup
	ops := func(worker int) {
		s := handles[worker%len(handles)]
		defer wg.Done()
		r := rand.New(rand.NewPCG(uint64(worker), 7)) //nolint:gosec // a deterministic schedule, not a secret
		for i := range 60 {
			at := now.Add(time.Duration(worker*100+i) * time.Second)
			switch r.IntN(4) {
			case 0:
				_, _, _ = s.Send(sB, sA, store.IntentRequest, fmt.Sprintf("w%d-%d", worker, i), i%7 == 0, at)
			case 1:
				_, _, _ = s.Send(sA, sC, store.IntentFYI, fmt.Sprintf("to c %d-%d", worker, i), false, at)
			case 2:
				if items, err := s.Listed(sA); err == nil && len(items) > 0 {
					_, _ = s.Reply(items[r.IntN(len(items))].ID, sA, "ok", store.Resolved, at)
				}
			case 3:
				_, _ = s.RetireDead(at)
				_, _ = s.Unread(sA)
				_ = s.SetHeartbeat(at, "t")
			}
		}
	}
	for w := range 8 {
		wg.Add(1)
		go ops(w)
	}
	// The close is not left to the random schedule: it runs at a fixed step, mid-race.
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := range 40 {
			if i == 20 {
				_ = handles[1].RecordClose(sC, "", "", now.Add(time.Duration(i)*time.Second))
				_, _ = handles[1].Retire(sC, now.Add(time.Duration(i)*time.Second))
			}
			_, _, _ = handles[2].Send(sA, sC, store.IntentRequest, fmt.Sprintf("late %d", i), false, now)
		}
	}()
	wg.Wait()
	if sess, err := s.Session(sC); err != nil || !sess.Closed {
		t.Fatalf("sC after the race: %+v, %v", sess, err)
	}
	if _, err := s.RetireDead(now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	checks := map[string]string{
		"a disposition without its stamp, or a stamp without a disposition": `SELECT count(*) FROM items
			WHERE (disposition IS NULL) <> (disposed_at IS NULL)`,
		"a resolved item without its answer": `SELECT count(*) FROM items
			WHERE disposition IS NOT NULL AND answer IS NULL`,
		"an item naming a session the store does not have": `SELECT count(*) FROM items
			WHERE (sender IS NOT NULL AND sender NOT IN (SELECT id FROM sessions))
			   OR (recipient IS NOT NULL AND recipient NOT IN (SELECT id FROM sessions))`,
		"an open item to a closed session that is not retired": `SELECT count(*) FROM items i
			JOIN sessions s ON s.id = i.recipient WHERE s.state = 'closed' AND i.retired_at IS NULL
			  AND (i.disposition IS NULL OR i.disposition = 'deferred')`,
		"a reply item with a disposition": `SELECT count(*) FROM items WHERE intent = 'reply' AND disposition IS NOT NULL`,
	}
	for name, q := range checks {
		var n int
		if err := s.RawDB().QueryRowContext(context.Background(), q).Scan(&n); err != nil {
			t.Fatal(err)
		}
		if n != 0 {
			t.Errorf("%s: %d rows", name, n)
		}
	}
	if hb, err := s.Heartbeat(); err != nil || hb.Version != "t" {
		t.Fatalf("heartbeat after the race: %+v, %v", hb, err)
	}
}
