package worker_test

import (
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

const sessionURL = "https://claude.ai/code/session_01AAA"

// Rejects the old layout (kind, project and title in one bold line, the redundant tab line, a raw
// page URL), a title that is not a link when the session URL is known, or a link when it is not.
// The expected notes are written by hand from R-WRK-082.
func TestAlertMarkdownLayout(t *testing.T) {
	// covers: R-WRK-082
	cases := []struct {
		name string
		a    worker.Alert
		want string
	}{
		{"needs you with session and page",
			worker.Alert{Kind: worker.AlertNeedsYou, Project: "demo-app", Title: "backups", Session: sessionURL,
				Text: "Ship it?", Page: "https://claude.ai/artifact/p1"},
			"🟠 **Needs you** · demo-app\n**[backups](" + sessionURL + ")**\n\nShip it?\n\n" +
				"[📄 Open the page](https://claude.ai/artifact/p1)"},
		{"checkpoint without links", worker.Alert{Kind: worker.AlertCheckpoint, Project: "demo-app", Title: "backups",
			Text: "rung two"}, "🟢 **Checkpoint** · demo-app\n**backups**\n\nrung two"},
		{"trouble", worker.Alert{Kind: worker.AlertTrouble, Project: "p", Title: "t", Text: "is blocked"},
			"🔴 **Trouble** · p\n**t**\n\nis blocked"},
		{"your move", worker.Alert{Kind: worker.AlertYourMove, Project: "p", Title: "t", Session: sessionURL, Text: "idle"},
			"🟡 **Your move** · p\n**[t](" + sessionURL + ")**\n\nidle"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := c.a.Markdown(); got != c.want {
				t.Fatalf("Markdown =\n%s\nwant\n%s", got, c.want)
			}
		})
	}
}

// Rejects a raw URL left in the text, a page URL not labelled as a page, a link or code span
// rewritten, trailing punctuation swallowed into the link, a URL inside bold left raw, and a list
// glued to the paragraph above it (it would render as the paragraph's text).
func TestAlertMarkdownNamesURLsAndKeepsLists(t *testing.T) {
	// covers: R-WRK-082
	for text, want := range map[string]string{
		"See https://example.com/a?b=1.":              "See [🔗 example.com](https://example.com/a?b=1).",
		"Read https://claude.ai/artifact/X1 first":    "Read [📄 page](https://claude.ai/artifact/X1) first",
		"Raw <https://example.com/x> here":            "Raw [🔗 example.com](https://example.com/x) here",
		"Keep [the doc](https://example.com/d)":       "Keep [the doc](https://example.com/d)",
		"Keep `curl https://example.com` as code":     "Keep `curl https://example.com` as code",
		"**go https://example.com/g** now":            "**go [🔗 example.com](https://example.com/g)** now",
		"Pick one:\n- ship\n- hold\n  for docs\nThen": "Pick one:\n\n- ship\n- hold\n  for docs\nThen",
	} {
		md := worker.Alert{Kind: worker.AlertNeedsYou, Project: "p", Title: "t", Text: text}.Markdown()
		if body := strings.SplitN(md, "\n\n", 2)[1]; body != want {
			t.Errorf("text %q →\n%q\nwant\n%q", text, body, want)
		}
	}
}

// Rejects an Ask flattened to one line (its list items and line breaks lost), a cut that splits a
// link, a bare URL or a bold span, and a cut that ignores a line end in the second half.
func TestAlertTextKeepsLinesAndCutsWholeMarkdown(t *testing.T) {
	// covers: R-WRK-080, R-WRK-082
	t.Run("an Ask keeps its lines", func(t *testing.T) {
		body := "## Done\nd\n## Now\nn\n## Next\nx\n## Ask\nWhich one?\n- ship now\n- hold for the docs\n## Lessons\nno\n"
		if al := alertOfItem(t, store.IntentCheckpoint, body); al.Text != "Which one?\n- ship now\n- hold for the docs" {
			t.Fatalf("text = %q", al.Text)
		}
	})
	t.Run("at a line end", func(t *testing.T) {
		line := strings.Repeat("word ", 50) + "end" // 253 runes, no sentence end
		text := alertOfItem(t, store.IntentCheckpoint, "## Ask\n"+line+"\n"+line+"\n").Text
		if text != line+"…" {
			t.Fatalf("text = %q", text)
		}
	})
	pad := strings.Repeat("a", 390) + " "
	for name, span := range map[string]string{
		"a link":     "[the design note](https://example.com/design)",
		"a bare URL": "https://example.com/" + strings.Repeat("p", 30),
		"bold":       "**do not split me**",
	} {
		t.Run("never inside "+name, func(t *testing.T) {
			text := alertOfItem(t, store.IntentRequest, pad+span+" tail").Text
			if text != strings.TrimSpace(pad)+"…" || utf8.RuneCountInString(text) > alertCap+1 {
				t.Fatalf("text = %q", text)
			}
		})
	}
}

// Rejects a worker that never reads the Remote Control URL from the transcript, or that forgets it
// when a later reading has none (a new conversation after /clear).
func TestAlertTitleLinksTheSession(t *testing.T) {
	// covers: R-WRK-082
	b := newBench(t)
	a := b.alerts()
	b.session(sA, store.Operator, "claude")
	b.act(sA, session.Activity{Remote: sessionURL})
	b.pass()
	b.act(sA, session.Activity{})
	b.pass()
	b.send(sA, store.Operator, store.IntentRequest, "Go?", false)
	b.pass()
	if len(a.got) != 1 || a.got[0].Session != sessionURL {
		t.Fatalf("alerts = %+v, want one with the session URL", a.got)
	}
	b.session(sB, store.Operator, "claude")
	b.send(sB, store.Operator, store.IntentRequest, "Go too?", false)
	b.pass()
	if len(a.got) != 2 || a.got[1].Session != "" {
		t.Fatalf("alerts = %+v, want the second without a session URL", a.got)
	}
}
