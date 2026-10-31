package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/ui"
)

// excerptRunes is the inbox excerpt length (R-MSG-030).
const excerptRunes = 80

type inboxOpts struct {
	show, boundary, until string
	asJSON, unread        bool
}

func newInboxCmd(d Deps) *cobra.Command {
	var o inboxOpts
	cmd := &cobra.Command{
		Use: "inbox",
		Example: "  # what waits for you\n  ness inbox\n  # read one in full\n  ness inbox --show m-abcd2345\n" +
			"  # read several, or every unread one, in full\n" +
			"  ness inbox --show m-abcd2345 m-efgh6789\n  ness inbox --unread",
		Short: "List your open items, one line each; --show ITEM... or --unread prints them in full",
		Args:  cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := checkInboxArgs(cmd, o, args); err != nil {
				return err
			}
			return withStore(d, func(s *store.Store, c caller) error {
				switch {
				case o.show != "":
					return showItems(d, s, c, append([]string{o.show}, args...))
				case o.unread:
					return showUnread(d, s, c)
				}
				return listInbox(d, s, c, o)
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.show, "show", "", "print the full body of `ITEM` (more items may follow) and nothing else")
	f.BoolVar(&o.unread, "unread", false, "print every open item not shown yet in full, each under a header line")
	f.StringVar(&o.boundary, "boundary", "", "record `NOTE`: your state is written down; compact me when idle")
	f.StringVar(&o.until, "until", "",
		"with --boundary: parked until `TIME` (RFC 3339, YYYY-MM-DD or YYYY-MM-DDTHH:MM, UTC)")
	f.BoolVar(&o.asJSON, "json", false, "print the list as one JSON object")
	for _, other := range []string{"boundary", "json", "unread"} {
		cmd.MarkFlagsMutuallyExclusive("show", other)
	}
	cmd.MarkFlagsMutuallyExclusive("unread", "boundary")
	cmd.MarkFlagsMutuallyExclusive("unread", "json")
	return cmd
}

// checkInboxArgs refuses an empty --show or --boundary, --until without --boundary, and items
// named without --show.
func checkInboxArgs(cmd *cobra.Command, o inboxOpts, args []string) error {
	for _, name := range []string{"show", "boundary"} {
		if cmd.Flags().Changed(name) && cmd.Flags().Lookup(name).Value.String() == "" {
			return usage("--%s needs a non-empty value", name)
		}
	}
	if o.until != "" && o.boundary == "" {
		return usage("--until needs --boundary")
	}
	if len(args) > 0 && o.show == "" {
		return usage("unexpected argument %q: items to read go after --show", args[0])
	}
	return nil
}

func listInbox(d Deps, s *store.Store, c caller, o inboxOpts) error {
	now := d.Now()
	if o.boundary != "" {
		if c.operator() {
			return refuse("--boundary needs a session: the operator is not compacted")
		}
		until, err := parseUntil(o.until, now)
		if err != nil {
			return err
		}
		if err := s.SetBoundary(c.id, o.boundary, until, now); err != nil {
			return err
		}
	}
	items, err := s.Inbox(c.id, now)
	if err != nil {
		return err
	}
	if o.asJSON {
		return printInboxJSON(d, items, now)
	}
	if d.rich() {
		printRichInbox(d, items, now)
		return nil
	}
	for _, it := range items {
		say(d.Stdout, "%s", inboxLine(it, now))
	}
	return nil
}

// parseUntil reads --until: RFC 3339, YYYY-MM-DD or YYYY-MM-DDTHH:MM in UTC, zero when empty;
// a time not after now is refused (R-MSG-034).
func parseUntil(v string, now time.Time) (time.Time, error) {
	if v == "" {
		return time.Time{}, nil
	}
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04", time.DateOnly} {
		if t, err := time.Parse(layout, v); err == nil {
			if !t.After(now) {
				return time.Time{}, refuse("--until %s is not in the future", v)
			}
			return t.UTC(), nil
		}
	}
	return time.Time{}, usage("--until %q: want RFC 3339, YYYY-MM-DD or YYYY-MM-DDTHH:MM", v)
}

// inboxLine is one item: id, from, intent (marked deferred), age and the excerpt, tab-separated
// (R-MSG-030, R-MSG-043, R-MSG-045). Tabs in the body are gone by then (R-MSG-031).
func inboxLine(it store.Item, now time.Time) string {
	intent := it.Intent
	if it.Disposition == store.Deferred {
		intent += " deferred"
	}
	return strings.Join([]string{
		it.ID, displayName(it.Sender), intent,
		store.FormatAge(now.Sub(it.CreatedAt)), store.Excerpt(it.Body, excerptRunes),
	}, "\t")
}

// showItems prints the items' bodies and marks them shown (R-MSG-033, R-MSG-037, R-MSG-041). One
// item prints its exact body bytes; several print each under its header line. Every id is checked
// before anything is shown, so a bad id shows nothing.
func showItems(d Deps, s *store.Store, c caller, ids []string) error {
	items := make([]store.Item, 0, len(ids))
	for _, id := range ids {
		it, err := addressedItem(s, c, id)
		if err != nil {
			return err
		}
		items = append(items, it)
	}
	return printItems(d, s, items, len(items) > 1)
}

// showUnread prints every listed item not shown yet, each under its header line (R-MSG-037).
func showUnread(d Deps, s *store.Store, c caller) error {
	items, err := s.Inbox(c.id, d.Now())
	if err != nil {
		return err
	}
	unread := items[:0]
	for _, it := range items {
		if it.ShownAt.IsZero() {
			unread = append(unread, it)
		}
	}
	return printItems(d, s, unread, true)
}

// printItems marks each item shown and prints its body, under a header line when headed.
func printItems(d Deps, s *store.Store, items []store.Item, headed bool) error {
	now := d.Now()
	for _, it := range items {
		if err := s.MarkShown(it.ID, now); err != nil {
			return err
		}
		if headed {
			say(d.out(), "%s", itemHeader(it, now))
		}
		body := it.Body
		if headed && !strings.HasSuffix(body, "\n") {
			body += "\n"
		}
		if d.rich() { // the operator reads it rendered (R-UI-024); agents get the exact bytes
			body = d.theme().Markdown(body)
		}
		if _, err := io.WriteString(d.out(), body); err != nil {
			return err
		}
	}
	return nil
}

// itemHeader is the line above a body when several are printed: "--- " and the item's id, from,
// intent and age, tab-separated as in the list.
func itemHeader(it store.Item, now time.Time) string {
	line := inboxLine(it, now)
	return "--- " + line[:strings.LastIndexByte(line, '\t')]
}

// intentColor is the badge color of an intent: what wants an answer stands out.
func intentColor(intent string) string {
	switch intent {
	case store.IntentRequest, store.IntentDecision:
		return "warn"
	case store.IntentHandoff, store.IntentReply, store.IntentCheckpoint:
		return "ok"
	}
	return "accent"
}

// printRichInbox is the same items as a table (R-UI-024).
func printRichInbox(d Deps, items []store.Item, now time.Time) {
	t, w := d.theme(), d.out()
	if len(items) == 0 {
		say(w, "%s", t.Dim.Render("inbox empty"))
		return
	}
	tb := ui.Table{Headers: []string{"id", "from", "intent", "age", "message"}, Right: []int{3}}
	for _, it := range items {
		intent := t.Pill(it.Intent, intentColor(it.Intent))
		if it.Urgent {
			intent = t.Pill(it.Intent+" urgent", "fail")
		}
		if it.Disposition == store.Deferred {
			intent += t.Dim.Render(" deferred")
		}
		tb.Rows = append(tb.Rows, []string{
			it.ID, displayName(it.Sender), intent,
			store.FormatAge(now.Sub(it.CreatedAt)), store.Excerpt(it.Body, excerptRunes),
		})
	}
	say(w, "%s", t.Render(tb))
	say(w, "%s", t.Dim.Render(fmt.Sprintf("%d open · ness inbox --show ID to read one", len(items))))
}

type inboxJSON struct {
	Items  []itemJSON `json:"items"`
	Schema int        `json:"schema"`
}

type itemJSON struct {
	Age       string `json:"age"`
	CreatedAt string `json:"created_at"`
	Deferred  bool   `json:"deferred"`
	Excerpt   string `json:"excerpt"`
	From      string `json:"from"`
	ID        string `json:"id"`
	Intent    string `json:"intent"`
	Urgent    bool   `json:"urgent"`
}

// printInboxJSON lists the same items as the text output with an excerpt, never the body
// (R-MSG-048, R-MSG-032).
func printInboxJSON(d Deps, items []store.Item, now time.Time) error {
	out := inboxJSON{Schema: 1, Items: []itemJSON{}}
	for _, it := range items {
		out.Items = append(out.Items, itemJSON{
			ID: it.ID, From: displayName(it.Sender), Intent: it.Intent, Deferred: it.Disposition == store.Deferred,
			Urgent: it.Urgent, Age: store.FormatAge(now.Sub(it.CreatedAt)),
			CreatedAt: it.CreatedAt.UTC().Format(time.RFC3339),
			Excerpt:   store.Excerpt(it.Body, excerptRunes),
		})
	}
	enc := json.NewEncoder(d.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
