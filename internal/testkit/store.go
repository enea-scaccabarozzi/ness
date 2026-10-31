package testkit

import (
	"context"
	"database/sql"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// storeCmd drives the project's store from a script through the store API. It stands in for
// the session tree until P3 (sessions are inserted directly) and reads state the commands do
// not print:
//
//	store add-session ID SCOPE [PARENT]   insert an open session; no PARENT makes a root
//	store close-session ID                mark it closed
//	store add-launch ID SCOPE PARENT|- FIELD=VALUE...   insert a launched session (fields: tier, effort, client,
//	                                      client_session, router_reason, worktree, size)
//	store session ID FIELD=VALUE...       fail unless every field matches (negatable with !)
//	store item ID FIELD=VALUE...          fail unless every field matches (negatable with !)
//	store items RECIPIENT N               fail unless RECIPIENT (an id or operator) lists N items
//	store pulled RECIPIENT N              fail unless N of RECIPIENT's listed items are pulled
//	store last RECIPIENT VAR              set $VAR to the id of RECIPIENT's newest listed item
//	store set-version N|current          write schema version N (current: this binary's), as a newer ness would
//	store claim-posts all                claim every post as another live run
//	store thread CHANNEL KEY ID          fail unless the thread of KEY on CHANNEL is ID
//	store phase ID PHASE                  set the launch phase
//	store bind ID PANE TAB AGENT          bind a tab, pane and agent for the next run
//	store integrate ID REF                record the integration ref
//	store heartbeat AGE_SECONDS VERSION   write a worker heartbeat AGE_SECONDS old
//	store last-session VAR                set $VAR to the id of the newest session
//	store get ID FIELD VAR                set $VAR to the session's field
//	store sessions N                      fail unless the store holds N sessions (closed included)
//	store probe PATH OWNER FIELD=VALUE... fail unless the owner's live probe at PATH matches (negatable)
//	store probes N                        fail unless the store holds N active or paused probes
//	store probe-id PATH OWNER VAR         set $VAR to the id of the owner's live probe at PATH
//
// Session fields: parent, scope, closed, ready (false or the RFC 3339 stamp), boundary, phase, tier,
// effort, run, pane, tab, agent, client_session, worktree, branch, record, integrated, title,
// adopted, client, router_reason, size, parked and parked_until (false or the stamp), parked_note.
// The scripts' clock is NESS_TEST_NOW when set.
// Item fields: sender, recipient, intent, pulled, shown (false or the stamp), disposition, answer, probe.
// Probe fields: state, owner, last_state, streak, runs, verdict, verdict_note, needs_edit (true or false),
// note, final_wake, evidence, file_error.
func storeCmd(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) < 2 {
		ts.Fatalf("usage: store add-session|close-session|session|item|items|pulled|last|set-version ...")
	}
	path, err := store.Locate(ts.MkAbs("."))
	ts.Check(err)
	if args[0] == "set-version" { // raw: Open refuses a newer store, and may not lower one
		db, oerr := sql.Open("sqlite", path)
		ts.Check(oerr)
		v := store.Version()
		if args[1] != "current" {
			v = wantCount(ts, args)
		}
		_, oerr = db.ExecContext(context.Background(), "PRAGMA user_version = "+strconv.Itoa(v))
		ts.Check(oerr)
		ts.Check(db.Close())
		return
	}
	if args[0] == "fail-close" || args[0] == "allow-close" { // a trigger makes closing a row fail
		sqlText := `DROP TRIGGER IF EXISTS fail_close`
		if args[0] == "fail-close" {
			sqlText = `CREATE TRIGGER fail_close BEFORE UPDATE OF state ON sessions WHEN NEW.state = 'closed'
				BEGIN SELECT RAISE(ABORT, 'injected close failure'); END`
		}
		db, oerr := sql.Open("sqlite", path)
		ts.Check(oerr)
		_, oerr = db.ExecContext(context.Background(), sqlText)
		ts.Check(oerr)
		ts.Check(db.Close())
		return
	}
	if args[0] == "claim-posts" { // as a run that is sending right now
		db, oerr := sql.Open("sqlite", path)
		ts.Check(oerr)
		_, oerr = db.ExecContext(context.Background(), `UPDATE posts SET claimed_by = 'other-run', claimed_at = ?`,
			scriptNow(ts).UTC().Format(time.RFC3339Nano))
		ts.Check(oerr)
		ts.Check(db.Close())
		return
	}
	s, err := store.Open(path)
	ts.Check(err)
	defer func() { ts.Check(s.Close()) }()
	sub, ok := storeSubs[args[0]]
	if !ok {
		ts.Fatalf("store: unknown subcommand %s", args[0])
	}
	sub(ts, neg, s, args[1:])
}

var storeSubs = map[string]func(ts *testscript.TestScript, neg bool, s *store.Store, args []string){
	"thread": func(ts *testscript.TestScript, _ bool, s *store.Store, args []string) {
		if len(args) != 3 {
			ts.Fatalf("usage: store thread CHANNEL KEY ID")
		}
		id, err := s.Thread(args[0], args[1])
		ts.Check(err)
		if id != args[2] {
			ts.Fatalf("thread of %s on %s = %q, want %q", args[1], args[0], id, args[2])
		}
	},
	"add-session": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		if neg || len(args) < 2 || len(args) > 3 {
			ts.Fatalf("usage: store add-session ID SCOPE [PARENT]")
		}
		parent := store.Operator
		if len(args) == 3 {
			parent = args[2]
		}
		ts.Check(s.AddSession(args[0], args[1], parent, scriptNow(ts)))
	},
	"add-launch": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		if neg || len(args) < 3 {
			ts.Fatalf("usage: store add-launch ID SCOPE PARENT|- FIELD=VALUE...")
		}
		l := store.Launch{ID: args[0], Scope: args[1], Title: args[1], Client: "claude"}
		if args[2] != "-" {
			l.Parent = args[2]
		}
		clientSession := ""
		for _, kv := range args[3:] {
			k, v, _ := strings.Cut(kv, "=")
			switch k {
			case "tier":
				l.Tier = v
			case "effort":
				l.Effort = v
			case "client":
				l.Client = v
			case "client_session":
				clientSession = v
			case "router_reason":
				l.RouterReason = v
			case "worktree":
				l.Worktree = v
			case "adopted":
				l.Adopted = v == "true"
			case "size":
				l.Size = v
			default:
				ts.Fatalf("store add-launch: unknown field %q", k)
			}
		}
		ts.Check(s.AddLaunch(l, scriptNow(ts)))
		if clientSession != "" {
			ts.Check(s.BindAgent(args[0], "", clientSession))
		}
	},
	"close-session": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		if neg || len(args) != 1 {
			ts.Fatalf("usage: store close-session ID")
		}
		ts.Check(s.CloseSession(args[0]))
	},
	"session": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		sess, err := s.Session(args[0])
		ts.Check(err)
		matchFields(ts, neg, args[1:], sessionFields(sess))
	},
	"get": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		if neg || len(args) != 3 {
			ts.Fatalf("usage: store get ID FIELD VAR")
		}
		sess, err := s.Session(args[0])
		ts.Check(err)
		v, ok := sessionFields(sess)[args[1]]
		if !ok {
			ts.Fatalf("store get: unknown field %q", args[1])
		}
		ts.Setenv(args[2], v)
	},
	"last-session": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		all, err := s.Sessions(true)
		ts.Check(err)
		if neg || len(args) != 1 || len(all) == 0 {
			ts.Fatalf("usage: store last-session VAR (with at least one session)")
		}
		ts.Setenv(args[0], all[len(all)-1].ID)
	},
	"phase": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		if neg || len(args) != 2 {
			ts.Fatalf("usage: store phase ID PHASE")
		}
		ts.Check(s.SetPhase(args[0], args[1], scriptNow(ts)))
	},
	"bind": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		if neg || len(args) != 4 {
			ts.Fatalf("usage: store bind ID PANE TAB AGENT")
		}
		sess, err := s.Session(args[0])
		ts.Check(err)
		ts.Check(s.BindTab(args[0], args[1], args[2], sess.Run+1))
		ts.Check(s.BindAgent(args[0], args[3], sess.ClientSession))
	},
	"integrate": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		if neg || len(args) != 2 {
			ts.Fatalf("usage: store integrate ID REF")
		}
		ts.Check(s.SetIntegration(args[0], args[1], scriptNow(ts)))
	},
	"set-ready": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		if neg || len(args) != 2 || args[1] != "false" {
			ts.Fatalf("usage: store set-ready ID false")
		}
		ts.Check(s.ClearReady(args[0]))
	},
	"heartbeat": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		if neg || len(args) != 2 {
			ts.Fatalf("usage: store heartbeat AGE_SECONDS VERSION")
		}
		age, err := strconv.Atoi(args[0])
		ts.Check(err)
		ts.Check(s.SetHeartbeat(scriptNow(ts).Add(-time.Duration(age)*time.Second), args[1]))
	},
	"sessions": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		all, err := s.Sessions(true)
		ts.Check(err)
		checkCount(ts, neg, "sessions", "sessions", len(all), args)
	},
	"probe": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		if len(args) < 3 {
			ts.Fatalf("usage: store probe PATH OWNER FIELD=VALUE...")
		}
		p, err := s.ProbeOf(args[0], args[1], "")
		ts.Check(err)
		matchFields(ts, neg, args[2:], map[string]string{
			"state": p.State, "owner": p.Owner, "last_state": p.LastState, "streak": strconv.Itoa(p.Streak),
			"runs": strconv.Itoa(p.Runs), "verdict": p.Verdict, "verdict_note": p.VerdictNote,
			"needs_edit": strconv.FormatBool(p.NeedsEdit != ""), "note": p.Note,
			"final_wake": strconv.FormatBool(p.FinalWake), "evidence": p.Evidence, "file_error": p.FileError,
		})
	},
	"probe-id": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		if neg || len(args) != 3 {
			ts.Fatalf("usage: store probe-id PATH OWNER VAR")
		}
		p, err := s.ProbeOf(args[0], args[1], "")
		ts.Check(err)
		ts.Setenv(args[2], p.ID)
	},
	"probes": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		live, err := s.Probes(false)
		ts.Check(err)
		checkCount(ts, neg, "probes", "live probes", len(live), args)
	},
	"item": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		it, err := s.Item(args[0])
		ts.Check(err)
		matchFields(ts, neg, args[1:], map[string]string{
			"sender": it.Sender, "recipient": it.Recipient, "intent": it.Intent,
			"pulled": stampOrFalse(it.PulledAt), "shown": stampOrFalse(it.ShownAt),
			"disposition": it.Disposition, "answer": it.Answer, "probe": it.Probe,
		})
	},
	"items": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		items := listed(ts, s, args)
		if (len(items) == wantCount(ts, args)) == neg {
			ts.Fatalf("%s lists %d items, want %s", args[0], len(items), args[1])
		}
	},
	"pulled": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		n := 0
		for _, it := range listed(ts, s, args) {
			if !it.PulledAt.IsZero() {
				n++
			}
		}
		if (n == wantCount(ts, args)) == neg {
			ts.Fatalf("%s has %d pulled items, want %s", args[0], n, args[1])
		}
	},
	"last": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		items := listed(ts, s, args)
		if neg || len(items) == 0 {
			ts.Fatalf("store last: %s lists no items", args[0])
		}
		ts.Setenv(args[1], items[len(items)-1].ID)
	},
}

// checkCount fails unless got matches the single numeric argument (negated with neg).
func checkCount(ts *testscript.TestScript, neg bool, verb, what string, got int, args []string) {
	if len(args) != 1 {
		ts.Fatalf("usage: store %s N", verb)
	}
	want, err := strconv.Atoi(args[0])
	ts.Check(err)
	if (got == want) == neg {
		ts.Fatalf("store has %d %s, want %s", got, what, args[0])
	}
}

// sessionFields is the session's fields as `store session` and `store get` name them.
func sessionFields(sess store.Session) map[string]string {
	return map[string]string{
		"parent": sess.Parent, "scope": sess.Scope, "closed": strconv.FormatBool(sess.Closed),
		"ready": stampOrFalse(sess.ReadyAt), "boundary": sess.BoundaryNote,
		"phase": sess.Phase, "tier": sess.Tier, "effort": sess.Effort, "run": strconv.Itoa(sess.Run),
		"pane": sess.Pane, "tab": sess.Tab, "agent": sess.Agent, "client_session": sess.ClientSession,
		"worktree": sess.Worktree, "branch": sess.Branch, "base": sess.Base, "record": sess.Record,
		"integrated": sess.Integrated, "title": sess.Title, "adopted": strconv.FormatBool(sess.Adopted),
		"client": sess.Client, "router_reason": sess.RouterReason, "size": sess.Size,
		"close_note": sess.CloseNote, "keep_reason": sess.KeepReason,
		"parked": stampOrFalse(sess.ParkedAt), "parked_until": stampOrFalse(sess.ParkedUntil),
		"parked_note": sess.ParkedNote,
	}
}

func listed(ts *testscript.TestScript, s *store.Store, args []string) []store.Item {
	if len(args) != 2 {
		ts.Fatalf("usage: store items|pulled|last RECIPIENT N|VAR")
	}
	recipient := args[0]
	if recipient == "operator" {
		recipient = store.Operator
	}
	items, err := s.Listed(recipient)
	ts.Check(err)
	return items
}

func wantCount(ts *testscript.TestScript, args []string) int {
	n, err := strconv.Atoi(args[1])
	ts.Check(err)
	return n
}

func stampOrFalse(t time.Time) string {
	if t.IsZero() {
		return "false"
	}
	return t.UTC().Format(time.RFC3339)
}

// matchFields compares FIELD=VALUE pairs against got; with neg it fails when all of them match.
func matchFields(ts *testscript.TestScript, neg bool, want []string, got map[string]string) {
	if len(want) == 0 {
		ts.Fatalf("store: give FIELD=VALUE pairs to check")
	}
	all := true
	for _, kv := range want {
		k, v, ok := strings.Cut(kv, "=")
		gv, known := got[k]
		if !ok || !known {
			ts.Fatalf("store: unknown field %q; fields are %v", kv, slices.Sorted(maps.Keys(got)))
		}
		if gv != v {
			all = false
			ts.Logf("store: %s = %q, want %q", k, gv, v)
		}
	}
	if all == neg {
		ts.Fatalf("store: all fields match = %v", all)
	}
}

// scriptNow is the script's clock: NESS_TEST_NOW when set, else the wall clock.
func scriptNow(ts *testscript.TestScript) time.Time {
	if v := ts.Getenv("NESS_TEST_NOW"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		ts.Check(err)
		return t
	}
	return time.Now()
}
