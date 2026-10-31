package store_test

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

const (
	cpBody    = "## Done\nx\n## Now\ny\n## Next\nz\n"
	cpAskBody = cpBody + "## Ask\nShip it?\n"
)

// send puts an item on the store and fails the test on error.
func send(t *testing.T, s *store.Store, from, to, intent, body string, at time.Time) string {
	t.Helper()
	id, _, err := s.Send(from, to, intent, body, false, at)
	if err != nil {
		t.Fatalf("Send(%s, %s, %s): %v", from, to, intent, err)
	}
	return id
}

func unalerted(t *testing.T, s *store.Store) []string {
	t.Helper()
	items, err := s.UnalertedToOperator()
	if err != nil {
		t.Fatal(err)
	}
	return ids(items)
}

// Rejects a migration that drops rows, leaves old items unalerted (an upgrade would replay every
// open item as an alert) or does not reach version 7.
func TestMigrationSevenKeepsItemsAndStampsThemAlerted(t *testing.T) {
	// covers: R-MSG-067, R-MSG-046
	path := freshPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	db := raw(t, path)
	for _, q := range store.Migrations[:6] {
		if _, err := db.ExecContext(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	for _, q := range []string{
		`INSERT INTO sessions (id, scope, created_at) VALUES ('` + sA + `', 'root', '2026-10-08T00:00:00Z')`,
		`INSERT INTO items (id, key, sender, intent, body, created_at) VALUES
		 ('m-open', 'k1', '` + sA + `', 'request', 'open ask', '2026-10-08T01:00:00Z')`,
		`INSERT INTO items (id, key, sender, intent, body, created_at, disposition, answer) VALUES
		 ('m-defer', 'k2', '` + sA + `', 'decision', 'deferred ask', '2026-10-08T02:00:00Z', 'deferred', 'later')`,
		`PRAGMA user_version = 6`,
	} {
		if _, err := db.ExecContext(t.Context(), q); err != nil {
			t.Fatal(err)
		}
	}
	s := mustOpen(t, path)
	if got := userVersion(t, path); got != store.Version() {
		t.Fatalf("user_version = %d, want %d", got, store.Version())
	}
	for id, body := range map[string]string{"m-open": "open ask", "m-defer": "deferred ask"} {
		if it, err := s.Item(id); err != nil || it.Body != body {
			t.Fatalf("legacy item %s = %+v, %v", id, it, err)
		}
	}
	var stamped int
	row := raw(t, path).QueryRowContext(t.Context(), `SELECT COUNT(*) FROM items WHERE alerted_at = created_at`)
	if err := row.Scan(&stamped); err != nil || stamped != 2 {
		t.Fatalf("rows stamped alerted_at = created_at: %d, %v; want 2", stamped, err)
	}
	if got := unalerted(t, s); len(got) != 0 {
		t.Fatalf("an upgrade left %v to alert", got)
	}
	fresh := send(t, s, sA, store.Operator, store.IntentDecision, "new ask", now)
	if got := unalerted(t, s); !slices.Equal(got, []string{fresh}) {
		t.Fatalf("after the upgrade a new item is unalerted: %v, want [%s]", got, fresh)
	}
}

// Rejects a selection by intent that is too wide (fyi, a reply to a session), too narrow (a
// checkpoint or the answer to an operator item missing),
// not ordered, or not limited to the operator.
func TestUnalertedToOperatorSelectsTheOperatorsOpenAsks(t *testing.T) {
	// covers: R-MSG-067, R-MSG-040
	s := tree(t)
	req := send(t, s, sB, store.Operator, store.IntentRequest, "r", now)
	dec := send(t, s, sA, store.Operator, store.IntentDecision, "d", now.Add(time.Minute))
	hand := send(t, s, sB, store.Operator, store.IntentHandoff, "h", now.Add(2*time.Minute))
	cpAsk := send(t, s, sA, store.Operator, store.IntentCheckpoint, cpAskBody, now.Add(3*time.Minute))
	cp := send(t, s, sA, store.Operator, store.IntentCheckpoint, cpBody, now.Add(4*time.Minute))
	send(t, s, sA, store.Operator, store.IntentFYI, "f", now.Add(5*time.Minute))
	send(t, s, sA, sB, store.IntentRequest, "to a session", now.Add(6*time.Minute))
	opAsk := send(t, s, store.Operator, sA, store.IntentRequest, "operator asks", now.Add(7*time.Minute))
	ans, err := s.Reply(opAsk, sA, "answer", store.Resolved, now.Add(8*time.Minute))
	if err != nil {
		t.Fatal(err) // the reply goes to the operator
	}
	// the notice of a withdrawn ask is born alerted
	if _, err = s.Withdraw(req, "never mind", now.Add(9*time.Minute)); err != nil {
		t.Fatal(err)
	}
	want := []string{dec, hand, cpAsk, cp, ans}
	if got := unalerted(t, s); !slices.Equal(got, want) {
		t.Fatalf("unalerted = %v, want %v", got, want)
	}
}

// Rejects a stamp that is not durable (a restart would repeat the alert) or that hides other items.
func TestMarkAlertedSurvivesReopen(t *testing.T) {
	// covers: R-MSG-067
	path := freshPath(t)
	s := mustOpen(t, path)
	for _, id := range []string{sA, sB} {
		if err := s.AddSession(id, "scope", store.Operator, now); err != nil {
			t.Fatal(err)
		}
	}
	first := send(t, s, sA, store.Operator, store.IntentDecision, "one", now)
	second := send(t, s, sB, store.Operator, store.IntentRequest, "two", now.Add(time.Minute))
	if err := s.MarkAlerted(first, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := unalerted(t, s); !slices.Equal(got, []string{second}) {
		t.Fatalf("after one stamp: %v, want [%s]", got, second)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	again := mustOpen(t, path)
	if got := unalerted(t, again); !slices.Equal(got, []string{second}) {
		t.Fatalf("after a reopen: %v, want [%s]", got, second)
	}
}

// Rejects a selection that alerts an item already answered, withdrawn or retired (an item
// disposed before its alert goes out gets none), and one that drops a deferred item (still open).
func TestUnalertedSkipsDisposedWithdrawnAndRetired(t *testing.T) {
	// covers: R-MSG-067
	path := freshPath(t)
	s := mustOpen(t, path)
	if err := s.AddSession(sA, "scope", store.Operator, now); err != nil {
		t.Fatal(err)
	}
	replied := send(t, s, sA, store.Operator, store.IntentDecision, "replied", now)
	withdrawn := send(t, s, sA, store.Operator, store.IntentRequest, "withdrawn", now.Add(time.Minute))
	retired := send(t, s, sA, store.Operator, store.IntentRequest, "retired", now.Add(2*time.Minute))
	deferred := send(t, s, sA, store.Operator, store.IntentHandoff, "deferred", now.Add(3*time.Minute))
	if _, err := s.Reply(replied, store.Operator, "yes", store.Resolved, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Withdraw(withdrawn, "myself", now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if _, err := raw(t, path).ExecContext(t.Context(),
		`UPDATE items SET retired_at = ? WHERE id = ?`, "2026-10-08T12:00:00Z", retired); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reply(deferred, store.Operator, "later", store.Deferred, now.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got := unalerted(t, s); !slices.Equal(got, []string{deferred}) {
		t.Fatalf("unalerted = %v, want only the deferred %s", got, deferred)
	}
}

// Rejects a checkpoint that always obliges, never obliges, or stays listed after it was read
// although it asks nothing.
func TestCheckpointObligationFollowsAsk(t *testing.T) {
	// covers: R-MSG-040, R-MSG-041
	s := tree(t)
	plain := send(t, s, sA, store.Operator, store.IntentCheckpoint, cpBody, now)
	asking := send(t, s, sA, store.Operator, store.IntentCheckpoint, cpAskBody, now.Add(time.Minute))
	if it, err := s.Item(asking); err != nil || !it.Obligation() {
		t.Fatalf("a checkpoint with ## Ask is no obligation: %+v, %v", it, err)
	}
	if it, err := s.Item(plain); err != nil || it.Obligation() {
		t.Fatalf("a checkpoint without ## Ask is an obligation: %+v, %v", it, err)
	}
	owed, err := s.Obligations(store.Operator)
	if err != nil || !slices.Equal(ids(owed), []string{asking}) {
		t.Fatalf("Obligations(operator) = %v, %v; want [%s]", ids(owed), err, asking)
	}
	for _, id := range []string{plain, asking} {
		if err = s.MarkShown(id, now.Add(time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	listed, err := s.Listed(store.Operator)
	if err != nil || !slices.Equal(ids(listed), []string{asking}) {
		t.Fatalf("Listed(operator) after --show = %v, %v; want [%s]", ids(listed), err, asking)
	}
}

func TestCheckpointMissing(t *testing.T) {
	// covers: R-MSG-065
	cases := []struct {
		name, body string
		want       []string
	}{
		{"complete", cpBody, nil},
		{"trailing blanks", "## Done  \nx\n## Now\t\ny\n## Next \nz", nil},
		{"one missing", "## Done\nx\n## Next\nz\n", []string{"Now"}},
		{"all missing", "just text\n", []string{"Done", "Now", "Next"}},
		{"extra words do not count", "## Done extra\n## Now\n## Next\n", []string{"Done"}},
		{"deeper heading does not count", "### Done\n## Now\n## Next\n", []string{"Done"}},
		{"indented heading does not count", "  ## Done\n## Now\n## Next\n", []string{"Done"}},
		{"order is Done, Now, Next", "## Next\n", []string{"Done", "Now"}},
		// F10: a heading quoted in a fenced block is not a heading.
		{"fenced heading does not count", "## Done\n```\n## Now\n## Next\n```\n", []string{"Now", "Next"}},
		{"tilde fence does not count", "~~~md\n## Done\n~~~\n## Now\n## Next\n", []string{"Done"}},
		{"heading after the fence closes counts", "```\nx\n```\n## Done\n## Now\n## Next\n", nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := store.CheckpointMissing(c.body); !slices.Equal(got, c.want) {
				t.Fatalf("CheckpointMissing = %v, want %v", got, c.want)
			}
		})
	}
}

func TestHasAsk(t *testing.T) {
	// covers: R-MSG-040
	cases := []struct {
		name, body string
		want       bool
	}{
		{"ask section", cpAskBody, true},
		{"no ask", cpBody, false},
		{"ask in prose", cpBody + "I would ask you\n", false},
		{"deeper heading", cpBody + "### Ask\n", false},
		{"indented heading", cpBody + "  ## Ask\n", false},
		{"ask with extra words", cpBody + "## Asking\n", false},
		// An empty section asks nothing (field-2): it was true before, on the heading alone.
		{"ask last line, empty", cpBody + "## Ask", false},
		{"ask last line with a question", cpBody + "## Ask\nShip it?", true},
		{"nothing", cpBody + "## Ask\nNothing.\n", false},
		{"none, any case and markup", cpBody + "## Ask\n\n- **NONE**\n", false},
		{"nothing blocking", cpBody + "## Ask\nNothing blocking.\n## Lessons\nx\n", false},
		{"nothing is asked", cpBody + "## Ask\nNothing is asked of you today.\n", false},
		{"nothing blocking, but approve", cpBody + "## Ask\nNothing blocking, but please approve the migration.\n", true},
		{"nothing needed from you until", cpBody + "## Ask\nNothing needed until you approve X.\n", true},
		{"nothing then a question", cpBody + "## Ask\nNothing works here: which one?\n", true},
		// F10: an `## Ask` quoted in a code sample must not make the checkpoint an obligation.
		{"ask inside a code fence", cpBody + "```\n## Ask\nShip it?\n```\n", false},
		{"ask after a closed fence", cpBody + "```\nx\n```\n## Ask\nShip it?\n", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := store.HasAsk(c.body); got != c.want {
				t.Fatalf("HasAsk = %v, want %v", got, c.want)
			}
		})
	}
}

func TestSection(t *testing.T) {
	// covers: R-WRK-080
	body := "## Done\nfinished\n\n## Now\n\n  \nworking on it\nsecond line\n## Next\n## Ask\nShip it?\n"
	cases := []struct{ heading, want string }{
		{"Done", "finished"},
		{"Now", "working on it"},
		{"Next", ""}, // the next heading ends the section
		{"Ask", "Ship it?"},
		{"Missing", ""},
	}
	for _, c := range cases {
		if got := store.Section(body, c.heading); got != c.want {
			t.Errorf("Section(%q) = %q, want %q", c.heading, got, c.want)
		}
	}
}

// Rejects a Section that reads a `## Ask` sample from a code fence above the real one.
func TestSectionSkipsFencedHeadings(t *testing.T) {
	// covers: R-WRK-080
	for _, fence := range []string{"```", "~~~"} {
		body := "## Done\nx\n" + fence + "md\n## Ask\nsample\n" + fence + "\n## Ask\nShip it?\n"
		if got := store.Section(body, "Ask"); got != "Ship it?" {
			t.Errorf("fence %s: Section = %q, want %q", fence, got, "Ship it?")
		}
	}
}

func TestPageOf(t *testing.T) {
	// covers: R-MSG-065
	cases := []struct{ name, body, want string }{
		{"last line", cpBody + "Page: https://claude.ai/x", "https://claude.ai/x"},
		{"none", cpBody, ""},
		{"first of two", "Page: https://a.test\nPage: https://b.test\n", "https://a.test"},
		{"mid-line is prose", "see Page: https://a.test\n", ""},
		{"trailing newline", "Page: https://a.test\n", "https://a.test"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := store.PageOf(c.body); got != c.want {
				t.Fatalf("PageOf = %q, want %q", got, c.want)
			}
		})
	}
}
