package worker_test

import (
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/worker"
)

// Rejects a cut whose "…" lands inside the link target of the bare URL it follows: the phone link
// would open a wrong URL.
func TestAlertCutAfterURLKeepsTheEllipsisOutOfTheLink(t *testing.T) {
	// covers: R-WRK-082
	line := strings.Repeat("w ", 150) + "see https://example.com/a" // a line end in the second half
	al := alertOfItem(t, store.IntentCheckpoint, "## Ask\n"+line+"\n"+strings.Repeat("x ", 100)+"\n")
	if !strings.HasSuffix(al.Text, "https://example.com/a…") {
		t.Fatalf("text = %q, want a cut right after the URL", al.Text)
	}
	if md := al.Markdown(); !strings.HasSuffix(md, "[🔗 example.com](https://example.com/a)…") {
		t.Fatalf("Markdown ends %q", md[len(md)-60:])
	}
}

// Rejects a title or project whose brackets, asterisks, underscores or backticks are read as
// markup: `]` would end the link text and leave the raw URL, `**` would end the bold.
func TestAlertTitleIsEscaped(t *testing.T) {
	// covers: R-WRK-082
	a := worker.Alert{Kind: worker.AlertNeedsYou, Project: "my_lab", Title: "[WIP] a**b_c `d`", Session: sessionURL}
	want := "🟠 **Needs you** · my\\_lab\n**[\\[WIP\\] a\\*\\*b\\_c \\`d\\`](" + sessionURL + ")**"
	if got := a.Markdown(); got != want {
		t.Fatalf("Markdown =\n%s\nwant\n%s", got, want)
	}
}

// Rejects a page link that is not checked like a text URL: parentheses that break the target, a
// non-http page wrapped in a link (lost on Telegram, a javascript: link on Slack), and an autolink
// whose ")" ends the target early.
func TestAlertPageAndAutolinkAreSafe(t *testing.T) {
	// covers: R-WRK-082
	for page, want := range map[string]string{
		"https://en.wikipedia.org/wiki/Foo_(bar)": "[📄 Open the page](https://en.wikipedia.org/wiki/Foo_%28bar%29)",
		"javascript:alert(1)":                     "📄 javascript:alert(1)",
		"docs/sessions/x/report.md":               "📄 docs/sessions/x/report.md",
		"https://x.com/a*b":                       "📄 https://x.com/a\\*b",
	} {
		md := worker.Alert{Kind: worker.AlertCheckpoint, Project: "p", Title: "t", Page: page}.Markdown()
		if got := md[strings.LastIndex(md, "\n\n")+2:]; got != want {
			t.Errorf("page %q → %q, want %q", page, got, want)
		}
	}
	md := worker.Alert{Kind: worker.AlertNeedsYou, Project: "p", Title: "t", Text: "go <https://x.com/a)b> now"}.Markdown()
	if want := "go [🔗 x.com](https://x.com/a%29b) now"; !strings.HasSuffix(md, want) {
		t.Fatalf("autolink: %q, want suffix %q", md, want)
	}
}
