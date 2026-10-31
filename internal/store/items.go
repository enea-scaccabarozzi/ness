package store

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"slices"
	"strings"
	"time"
)

// Intents (R-MSG-010, R-MSG-049). Reply items are created only by Reply; IntentCheckpoint is in checkpoint.go.
const (
	IntentRequest  = "request"
	IntentFYI      = "fyi"
	IntentDecision = "decision"
	IntentHandoff  = "handoff"
	IntentReply    = "reply"
)

// Dispositions (R-MSG-020). Returned hands the work back to the sender (R-MSG-064); Withdrawn is
// the sender's, never the recipient's (R-MSG-060).
const (
	Resolved  = "resolved"
	Declined  = "declined"
	Deferred  = "deferred"
	Accepted  = "accepted"
	Returned  = "returned"
	Withdrawn = "withdrawn"
)

var (
	// ErrNoItem is returned for an item id the store does not know.
	ErrNoItem = errors.New("no such item")
	// ErrDisposed is returned by Reply on an item that already has a final disposition (R-MSG-025).
	ErrDisposed = errors.New("item already disposed")
	// ErrReplyToReply is returned by Withdraw on a reply item: a reply carries no ask, so there is
	// nothing to withdraw (R-MSG-062).
	ErrReplyToReply = errors.New("a reply cannot be withdrawn")
	// ErrNotRecipient is returned by Reply when anyone but a reply's recipient settles it: its
	// sender, or the sender's manager, would otherwise hide it from the one it was for (R-MSG-050).
	ErrNotRecipient = errors.New("only a reply's recipient settles it")
)

// Item is one message. Sender and Recipient are session ids or Operator.
type Item struct {
	ID, Sender, Recipient, Intent, Body      string
	Urgent                                   bool
	CreatedAt, PulledAt, ShownAt, DisposedAt time.Time
	Disposition, Answer                      string
	Probe                                    string // the probe a wake belongs to, "" for a message (R-PRB-066)
}

// Open reports whether the item still awaits an answer: no disposition, or deferred (R-MSG-043).
func (it Item) Open() bool { return it.Disposition == "" || it.Disposition == Deferred }

// Obligation reports whether the item, while open, blocks its recipient's close (R-MSG-040):
// request, decision and handoff do; fyi and reply are labels only (R-MSG-041, R-MSG-022).
func (it Item) Obligation() bool {
	return it.Open() && (it.Intent == IntentRequest || it.Intent == IntentDecision || it.Intent == IntentHandoff ||
		it.Intent == IntentCheckpoint && HasAsk(it.Body))
}

const (
	selectItem = `SELECT id, sender, recipient, intent, body, urgent, created_at, pulled_at, shown_at,
		disposition, answer, disposed_at, probe FROM items`
	insertItem = `INSERT INTO items (id, key, sender, recipient, intent, body, urgent, created_at, ask)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`
	// listed is the inbox condition (R-MSG-036, R-MSG-041, R-MSG-043): open, and for an fyi or
	// reply, not shown yet. The recipient parameter uses IS so NULL (the operator) matches.
	listed = ` recipient IS ? AND (disposition IS NULL OR disposition = 'deferred')
		AND NOT (intent IN ('fyi', 'reply') AND shown_at IS NOT NULL)
		AND NOT (intent = 'checkpoint' AND ask = 0 AND shown_at IS NOT NULL) AND retired_at IS NULL`
	obliged = ` recipient IS ? AND (disposition IS NULL OR disposition = 'deferred')
		AND (intent IN ('request', 'decision', 'handoff') OR (intent = 'checkpoint' AND ask = 1))
		AND retired_at IS NULL`
	inOrder = ` ORDER BY created_at, rowid`
)

// itemKey is the idempotency key: sender, recipient, intent and body (R-MSG-014).
func itemKey(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Send records an item unless one with the same key exists, open or disposed (R-MSG-014,
// R-MSG-015). It returns the item's id and whether it was created by this call.
func (s *Store) Send(
	sender, recipient, intent, body string, urgent bool, now time.Time,
) (id string, created bool, err error) {
	key := itemKey(sender, recipient, intent, body)
	err = s.inTx(func(tx *sql.Tx) error {
		qerr := tx.QueryRowContext(context.Background(), `SELECT id FROM items WHERE key = ?`, key).Scan(&id)
		if qerr == nil {
			return nil
		}
		if !errors.Is(qerr, sql.ErrNoRows) {
			return qerr
		}
		id, created = NewItemID(), true
		_, qerr = tx.ExecContext(context.Background(), insertItem,
			id, key, nullable(sender), nullable(recipient), intent, body, urgent,
			stamp(now), intent == IntentCheckpoint && HasAsk(body))
		return qerr
	})
	if err != nil {
		return "", false, err
	}
	return id, created, nil
}

// Listed returns the recipient's inbox without side effects, in arrival order.
func (s *Store) Listed(recipient string) ([]Item, error) {
	return queryItems(s.db, selectItem+` WHERE`+listed+inOrder, nullable(recipient))
}

// Unread lists the recipient's listed items that were never pulled: what a worker hint covers;
// a pulled item is never hinted again (R-MSG-036, R-WRK-012).
func (s *Store) Unread(recipient string) ([]Item, error) {
	return queryItems(s.db, selectItem+` WHERE pulled_at IS NULL AND`+listed+inOrder, nullable(recipient))
}

// Inbox lists the recipient's inbox and marks every listed item pulled, keeping an earlier pull
// (R-MSG-036); a pulled item stays open until disposed.
func (s *Store) Inbox(recipient string, now time.Time) (items []Item, err error) {
	err = s.inTx(func(tx *sql.Tx) error {
		_, qerr := tx.ExecContext(context.Background(),
			`UPDATE items SET pulled_at = ? WHERE pulled_at IS NULL AND`+listed, stamp(now), nullable(recipient))
		if qerr != nil {
			return qerr
		}
		items, qerr = queryItems(tx, selectItem+` WHERE`+listed+inOrder, nullable(recipient))
		return qerr
	})
	return items, err
}

// Obligations lists the open items that block the recipient's close (R-MSG-040).
func (s *Store) Obligations(recipient string) ([]Item, error) {
	return queryItems(s.db, selectItem+` WHERE`+obliged+inOrder, nullable(recipient))
}

// HandedOff reports whether sender has a handoff its manager has not returned: the child has
// delivered its brief (R-WRK-050).
func (s *Store) HandedOff(sender string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM items WHERE sender = ? AND intent = 'handoff'
		AND (disposition IS NULL OR disposition <> ?))`, sender, Returned).Scan(&n)
	return n == 1, err
}

// LastInbound is when something last reached id from someone else: the newest item sent to it, or
// the newest disposition of an item it sent (an answer), the operator's included. Zero when none. Items a session sends
// itself (a probe's wake) and disposes itself are not arrivals. The stall clock runs from it, since
// the hint that carries the item has not reached the session yet (R-WRK-050).
func (s *Store) LastInbound(id string) (time.Time, error) {
	var at sql.NullString
	err := s.db.QueryRowContext(context.Background(),
		`SELECT MAX(at) FROM (SELECT created_at AS at FROM items WHERE recipient = ? AND sender IS NOT recipient
		UNION ALL SELECT disposed_at FROM items WHERE sender = ? AND recipient IS NOT sender
		AND disposed_at IS NOT NULL)`, id, id).Scan(&at)
	return nullStamp(at), err
}

// Item returns one item or ErrNoItem.
func (s *Store) Item(id string) (Item, error) {
	return scanItem(s.db.QueryRowContext(context.Background(), selectItem+` WHERE id = ?`, id))
}

// MarkShown records that the item's full body was read; the first stamp is kept, and the item
// counts as pulled too (R-MSG-041).
func (s *Store) MarkShown(id string, now time.Time) error {
	return s.touch(ErrNoItem,
		`UPDATE items SET shown_at = COALESCE(shown_at, ?), pulled_at = COALESCE(pulled_at, ?) WHERE id = ?`,
		stamp(now), stamp(now), id)
}

// Reply records the answer and disposition on the item and delivers the answer to the item's
// sender as a new reply item from by (R-MSG-020, R-MSG-022); a sender recording an answer it
// already knows (the operator's, given in its chat) gets none and replyID is "" (R-MSG-052). A
// deferred item may be answered again; any other disposed item is ErrDisposed (R-MSG-025,
// R-MSG-043). A reply item is settled by its recipient and delivers nothing back, so two
// sessions cannot bounce replies (R-MSG-050).
func (s *Store) Reply(id, by, answer, disposition string, now time.Time) (replyID string, err error) {
	err = s.inTx(func(tx *sql.Tx) error {
		it, qerr := scanItem(tx.QueryRowContext(context.Background(), selectItem+` WHERE id = ?`, id))
		if qerr != nil {
			return qerr
		}
		if it.Intent == IntentReply && it.Recipient != by {
			return ErrNotRecipient
		}
		if derr := disposeTx(tx, it, disposition, answer, now); derr != nil {
			return derr
		}
		if it.Sender == by || it.Intent == IntentReply {
			return nil
		}
		replyID = NewItemID()
		_, qerr = tx.ExecContext(context.Background(), insertItem,
			replyID, "reply:"+replyID, nullable(by), nullable(it.Sender), IntentReply, answer, false, stamp(now), false)
		return qerr
	})
	if err != nil {
		return "", err
	}
	return replyID, nil
}

// Withdraw settles an open item as withdrawn by its sender, with the reason as its answer, and
// delivers the reason to the recipient as a reply item from the sender (R-MSG-060, R-MSG-061).
// The caller checks that the withdrawer is the sender.
func (s *Store) Withdraw(id, reason string, now time.Time) (noticeID string, err error) {
	err = s.inTx(func(tx *sql.Tx) error {
		it, qerr := scanItem(tx.QueryRowContext(context.Background(), selectItem+` WHERE id = ?`, id))
		if qerr != nil {
			return qerr
		}
		if it.Intent == IntentReply {
			return ErrReplyToReply
		}
		if derr := disposeTx(tx, it, Withdrawn, reason, now); derr != nil {
			return derr
		}
		noticeID = NewItemID()
		_, qerr = tx.ExecContext(context.Background(), insertItem, noticeID, "reply:"+noticeID,
			nullable(it.Sender), nullable(it.Recipient), IntentReply,
			"withdrew "+id+": "+reason, false, stamp(now), false)
		if qerr != nil {
			return qerr
		}
		// the operator was told of the ask, not of its withdrawal: the notice is born alerted
		_, qerr = tx.ExecContext(context.Background(),
			`UPDATE items SET alerted_at = ? WHERE id = ?`, stamp(now), noticeID)
		return qerr
	})
	if err != nil {
		return "", err
	}
	return noticeID, nil
}

// disposeTx records a disposition on an item that is still open; any other is ErrDisposed.
func disposeTx(tx *sql.Tx, it Item, disposition, answer string, now time.Time) error {
	if !it.Open() {
		return ErrDisposed
	}
	_, err := tx.ExecContext(context.Background(),
		`UPDATE items SET disposition = ?, answer = ?, disposed_at = ? WHERE id = ?`,
		disposition, answer, stamp(now), it.ID)
	return err
}

type querier interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

type scanner interface {
	Scan(dest ...any) error
}

func queryItems(q querier, query string, args ...any) (items []Item, err error) {
	rows, err := q.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		it, serr := scanItem(rows)
		if serr != nil {
			return nil, serr
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func scanItem(r scanner) (Item, error) {
	var (
		it                                                      Item
		sender, recipient, created, pulled, shown, disp, answer sql.NullString
		disposed, probe                                         sql.NullString
	)
	err := r.Scan(&it.ID, &sender, &recipient, &it.Intent, &it.Body, &it.Urgent, &created, &pulled, &shown,
		&disp, &answer, &disposed, &probe)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNoItem
	}
	if err != nil {
		return Item{}, err
	}
	it.Sender, it.Recipient, it.Disposition, it.Answer = sender.String, recipient.String, disp.String, answer.String
	it.CreatedAt, it.PulledAt = nullStamp(created), nullStamp(pulled)
	it.ShownAt, it.DisposedAt = nullStamp(shown), nullStamp(disposed)
	it.Probe = probe.String
	return it, nil
}

// DeadRoutes lists open items whose recipient is closed or unknown (R-DOC-012), as
// "<item> to <recipient> (closed|unknown)". Deferred items are still open (R-MSG-043).
func (s *Store) DeadRoutes() (out []string, err error) {
	rows, err := s.db.QueryContext(context.Background(),
		`SELECT i.id, i.recipient, COALESCE(r.state, 'unknown')
		 FROM items i LEFT JOIN sessions r ON r.id = i.recipient
		 WHERE i.recipient IS NOT NULL AND (i.disposition IS NULL OR i.disposition = 'deferred')
		   AND i.retired_at IS NULL AND COALESCE(r.state, '') <> 'open'
		 ORDER BY i.created_at, i.id`)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		var id, to, state string
		if scanErr := rows.Scan(&id, &to, &state); scanErr != nil {
			return nil, scanErr
		}
		out = append(out, id+" to "+to+" ("+state+")")
	}
	return out, rows.Err()
}

// Retired is an item RetireDead retired and the dead recipient it was addressed to.
type Retired struct{ ID, Recipient string }

// RetireDead retires every open item whose recipient is closed or unknown and returns them,
// sorted by id (R-WRK-020); an item to the operator is never dead. It is DeadRoutes' condition as an
// update.
func (s *Store) RetireDead(now time.Time) (out []Retired, err error) {
	rows, err := s.db.QueryContext(context.Background(),
		`UPDATE items SET retired_at = ? WHERE retired_at IS NULL AND recipient IS NOT NULL
		   AND (disposition IS NULL OR disposition = 'deferred')
		   AND recipient NOT IN (SELECT id FROM sessions WHERE state = 'open')
		 RETURNING id, recipient`, stamp(now))
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		var r Retired
		if serr := rows.Scan(&r.ID, &r.Recipient); serr != nil {
			return nil, serr
		}
		out = append(out, r)
	}
	slices.SortFunc(out, func(a, b Retired) int { return strings.Compare(a.ID, b.ID) })
	return out, rows.Err()
}
