package telegram_test

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/telegram"
	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestTelegramGoldenPerConstruct(t *testing.T) {
	// HTML parse mode: <b>, <i>, <code>, <pre>, <a href>; lists as bullet lines; tables as
	// <pre>; & < > escaped in text (R-NTF-024, R-NTF-026).
	// covers: R-NTF-010, R-NTF-024, R-NTF-026
	got := testkit.RenderConstructs(t, telegram.Channel{})
	for name, text := range got {
		testkit.Golden(t, filepath.Join("testdata", "golden", name+".txt"), text)
	}
	if !strings.Contains(got["escaping"], "Tom &amp; Jerry &lt;tag&gt;") || !strings.HasPrefix(got["table"], "<pre>") ||
		!strings.Contains(got["bold"], "<b>strong</b>") || !strings.Contains(got["fence"], "<pre><code class=\"language-sh\">") {
		t.Fatalf("renders: %q", got)
	}
}

func TestTelegramMeasureAndPayload(t *testing.T) {
	// The limit is 4,096 UTF-16 units of the visible text: HTML source over 4,096 whose text is
	// under fits one part; 2,049 astral-plane emoji (4,098 units) need two; the payload has
	// parse_mode HTML and reply_to_message_id when threaded (R-NTF-025, R-NTF-027).
	// covers: R-NTF-024, R-NTF-025, R-NTF-027
	c := telegram.Channel{ChatID: "1001"}
	bold := strings.Repeat("**ab** ", 600) // 4,200 source bytes, 1,800 visible units
	parts, err := notify.Split(c, notify.Parse(bold))
	if err != nil || len(parts) != 1 {
		t.Fatalf("source over, text under: %d parts, %v", len(parts), err)
	}
	emoji := strings.Repeat("😀", 2049)
	parts, err = notify.Split(c, notify.Parse(emoji[:len("😀")*1025]+"\n\n"+emoji[len("😀")*1025:]))
	if err != nil || len(parts) != 2 {
		t.Fatalf("4,098 units in two blocks: %d parts, %v", len(parts), err)
	}
	if got := c.Measure("<b>a&amp;b</b>"); got != 3 {
		t.Fatalf("Measure = %d, want 3", got)
	}
	var body map[string]any
	if err = json.Unmarshal(c.Payload(notify.Part{Blocks: []string{"<b>x</b>"}}, "7"), &body); err != nil {
		t.Fatal(err)
	}
	if body["chat_id"] != "1001" || body["parse_mode"] != "HTML" || body["text"] != "<b>x</b>" || body["reply_to_message_id"] != float64(7) {
		t.Fatalf("payload = %v", body)
	}
}

// leaky answers every request the way a hostile or broken server would: the failure text and
// the body both echo the secret, so only redaction keeps it out of the error.
type leaky struct {
	secret string
	mode   string
}

func (l leaky) RoundTrip(*http.Request) (*http.Response, error) {
	switch l.mode {
	case "conn":
		return nil, errors.New("dial failed for " + l.secret)
	case "statusline":
		return &http.Response{StatusCode: 502, Status: "502 " + l.secret, Body: io.NopCloser(strings.NewReader("x"))}, nil
	case "status":
		return &http.Response{StatusCode: 500, Status: "500 Internal Server Error",
			Body: io.NopCloser(strings.NewReader(`{"ok":false,"description":"bad ` + l.secret + `"}`))}, nil
	}
	return &http.Response{StatusCode: 200, Status: "200 OK",
		Body: io.NopCloser(strings.NewReader("<html>" + l.secret))}, nil
}

func TestTelegramErrorsCarryNoToken(t *testing.T) {
	// Security: the bot token is in the URL path; a failing Post never puts it or the URL in its
	// error: connection failure, non-2xx, malformed reply.
	// covers: R-NTF-007, R-NTF-042
	const token = "123:test"
	for _, mode := range []string{"conn", "status", "statusline", "malformed"} {
		c := telegram.Channel{Token: token, ChatID: "1001", HTTP: &http.Client{Transport: leaky{token, mode}}}
		_, err := c.Post(notify.Part{Blocks: []string{"hi"}}, "")
		if err == nil {
			t.Fatalf("%s: no error", mode)
		}
		if strings.Contains(err.Error(), token) || strings.Contains(err.Error(), "api.telegram.org") {
			t.Errorf("%s: error leaks the token or URL: %v", mode, err)
		}
	}
}

func TestTelegramReservedCharactersEscaped(t *testing.T) {
	// HTML parse mode (chosen over MarkdownV2): its reserved characters are & < > (and the quote
	// in an attribute). Each is escaped in text, code, fences, tables and link targets; the
	// MarkdownV2 punctuation needs no escape and stays literal.
	// covers: R-NTF-024, R-NTF-026
	c := telegram.Channel{}
	for in, want := range map[string]string{
		"a & b < c > d":                        "a &amp; b &lt; c &gt; d",
		"`x<y&z>`":                             "<code>x&lt;y&amp;z&gt;</code>",
		"```\n<&>\n```":                        "<pre>&lt;&amp;&gt;</pre>",
		"| <a> |\n|---|\n| & |":                "<pre>| &lt;a&gt; |\n|---|\n| &amp; |</pre>",
		"[t<](https://x.invalid/?a=1&b=\"2\")": `<a href="https://x.invalid/?a=1&amp;b=&#34;2&#34;">t&lt;</a>`,
		"_*[]()~>#+-=|{}.!":                    "_*[]()~&gt;#+-=|{}.!",
	} {
		if got := c.Render(notify.Parse(in)[0]); got != want {
			t.Errorf("Render(%q) = %q, want %q", in, got, want)
		}
	}
}

func FuzzRender(f *testing.F) {
	// covers: R-NTF-013
	f.Add("# h\n\n| a |\n|---|\n| <1 |\n\n```go\nx&y\n```\n- <i> & **b** [l](http://x.invalid/\"a)")
	f.Fuzz(func(t *testing.T, md string) {
		c := telegram.Channel{ChatID: "1001"}
		blocks := notify.Parse(md)
		for _, b := range blocks {
			_ = c.Measure(c.Render(b))
		}
		parts, err := notify.Split(c, blocks)
		for _, p := range parts {
			if err == nil && !json.Valid(c.Payload(p, "7")) {
				t.Fatal("invalid payload JSON")
			}
		}
	})
}

func TestTelegramNeverFollowsRedirects(t *testing.T) {
	// A 307 must not carry the token path or body to the Location: the second server sees nothing.
	// covers: R-NTF-042
	var hits atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	c := telegram.Channel{Token: "123:test", ChatID: "1001", HTTP: srv.Client(), URL: srv.URL}
	_, err := c.Post(notify.Part{Blocks: []string{"hi"}}, "")
	if err == nil || strings.Contains(err.Error(), "123:test") || hits.Load() != 0 {
		t.Fatalf("Post = %v, redirect target hits = %d; want a failed post and no hit", err, hits.Load())
	}
}

func TestTelegramHrefAllowlist(t *testing.T) {
	// Only http, https and mailto become links; any other scheme is plain text.
	// covers: R-NTF-024
	c := telegram.Channel{}
	for _, ok := range []string{"http://x.invalid/a", "https://x.invalid/a", "mailto:a@x.invalid"} {
		if got := c.Render(notify.Parse("[t](" + ok + ")")[0]); !strings.HasPrefix(got, `<a href="`) {
			t.Errorf("%s: %q; want a link", ok, got)
		}
	}
	for _, bad := range []string{"tg://resolve?domain=x", "javascript:alert", "data:text/html,x", "JavaScript:x"} {
		if got := c.Render(notify.Parse("[t](" + bad + ")")[0]); got != "t" {
			t.Errorf("%s: %q; want plain text t", bad, got)
		}
	}
}

func TestTelegramContractOverFake(t *testing.T) {
	// covers: R-NTF-005, R-NTF-027, R-QA-024
	f := testkit.FakeHTTP{Dir: t.TempDir()}
	c := telegram.Channel{Token: "test-token", ChatID: "1001", HTTP: &http.Client{Transport: f}}
	testkit.ChannelContract(t, c, "", func(n int) (string, string) {
		reqs, err := f.Requests()
		if err != nil || len(reqs) < n {
			t.Fatalf("requests: %v, %d", err, len(reqs))
		}
		return reqs[n-1].URL, reqs[n-1].Body
	})
}

// Rejects a renderer that escapes a link inside bold as text (the alert's linked title), drops an
// emoji label, or loses the blank line between blocks or a list's bullets.
func TestTelegramRendersTheAlertShapes(t *testing.T) {
	// covers: R-NTF-024
	md := "🟠 **Needs you** · p\n**[t_1](https://claude.ai/code/session_01A)** and *[x](https://e.x/a)*\n\n" +
		"Pick:\n\n- [🔗 e.x](https://e.x/b)\n- **hold** `a<b`"
	parts, err := notify.Split(telegram.Channel{}, notify.Parse(md))
	if err != nil || len(parts) != 1 {
		t.Fatalf("%d parts, %v", len(parts), err)
	}
	want := "🟠 <b>Needs you</b> · p\n<b><a href=\"https://claude.ai/code/session_01A\">t_1</a></b> and " +
		"<i><a href=\"https://e.x/a\">x</a></i>\n\nPick:\n\n• <a href=\"https://e.x/b\">🔗 e.x</a>\n• <b>hold</b> <code>a&lt;b</code>"
	if got := parts[0].Text(); got != want {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
}

// Rejects a renderer that reads an escaped title's brackets and asterisks as markup: the link
// would be lost and the raw URL shown, or the bold cut short.
func TestTelegramRendersAnEscapedTitleLiterally(t *testing.T) {
	// covers: R-NTF-024
	md := "**[\\[WIP\\] a\\*\\*b\\_c \\`d\\`](https://claude.ai/code/session_01A)** x\\*"
	parts, err := notify.Split(telegram.Channel{}, notify.Parse(md))
	if err != nil || len(parts) != 1 {
		t.Fatalf("%d parts, %v", len(parts), err)
	}
	want := "<b><a href=\"https://claude.ai/code/session_01A\">[WIP] a**b_c `d`</a></b> x*"
	if got := parts[0].Text(); got != want {
		t.Fatalf("got\n%q\nwant\n%q", got, want)
	}
}
