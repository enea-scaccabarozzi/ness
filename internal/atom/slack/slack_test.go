package slack_test

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

	"github.com/enea-scaccabarozzi/ness/internal/atom/slack"
	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestSlackGoldenPerConstruct(t *testing.T) {
	// Slack renders to markdown block text: headings, emphasis, code, links and lists pass as
	// Markdown, a table becomes a code block (R-NTF-020, R-NTF-022).
	// covers: R-NTF-010, R-NTF-020, R-NTF-022
	for name, got := range testkit.RenderConstructs(t, slack.Channel{}) {
		testkit.Golden(t, filepath.Join("testdata", "golden", name+".txt"), got)
	}
	table := notify.Parse("| a | b |\n|---|---|\n| 1 | 2 |\n")[0]
	if got := (slack.Channel{}).Render(table); !strings.HasPrefix(got, "```\n") || !strings.Contains(got, "| 1 | 2 |") {
		t.Fatalf("table = %q; want a code block", got)
	}
}

func TestSlackLimitsAndPayload(t *testing.T) {
	// 12,000 characters of block text fit one message, 12,001 need two, 51 blocks need two;
	// the payload carries only markdown blocks and thread_ts when threaded; characters are code
	// points, so multibyte text at the limit fits (R-NTF-021, R-NTF-041).
	// covers: R-NTF-020, R-NTF-021, R-NTF-023, R-NTF-041
	c := slack.Channel{ChannelID: "C0TEST"}
	one := strings.Repeat("é", 12000)
	parts, err := notify.Split(c, notify.Parse(one))
	if err != nil || len(parts) != 1 {
		t.Fatalf("12,000 code points: %d parts, %v", len(parts), err)
	}
	parts, err = notify.Split(c, notify.Parse(strings.Repeat("x", 6000)+"\n\n"+strings.Repeat("y", 5999)))
	if err != nil || len(parts) != 2 {
		t.Fatalf("12,001 with the blank line: %d parts, %v", len(parts), err)
	}
	parts, err = notify.Split(c, notify.Parse(strings.Repeat("p\n\n", 51)))
	if err != nil || len(parts) != 2 {
		t.Fatalf("51 blocks: %d parts, %v", len(parts), err)
	}
	var body struct {
		Channel  string `json:"channel"`
		ThreadTS string `json:"thread_ts"`
		Blocks   []struct {
			Type, Text string
		} `json:"blocks"`
	}
	if err = json.Unmarshal(c.Payload(parts[0], "1700000000.000100"), &body); err != nil {
		t.Fatal(err)
	}
	if body.Channel != "C0TEST" || body.ThreadTS != "1700000000.000100" || len(body.Blocks) != 50 {
		t.Fatalf("payload = %+v", body)
	}
	for _, b := range body.Blocks {
		if b.Type != "markdown" || b.Text != "p" {
			t.Fatalf("block = %+v; want markdown p", b)
		}
	}
	if strings.Contains(string(c.Payload(parts[1], "")), "thread_ts") {
		t.Fatal("an unthreaded payload carries thread_ts")
	}
}

func TestSlackOKFalseIsAnError(t *testing.T) {
	// Review Focus 4: Slack answers 200 with ok:false; that is a failed part naming the reason.
	// covers: R-NTF-007
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("Authorization = %q", r.Header.Get("Authorization"))
		}
		_, _ = io.WriteString(w, `{"ok":false,"error":"channel_not_found"}`)
	}))
	defer srv.Close()
	c := slack.Channel{Token: "test-token", ChannelID: "C0TEST", HTTP: srv.Client(), URL: srv.URL}
	if _, err := c.Post(notify.Part{Blocks: []string{"hi"}}, ""); err == nil || !strings.Contains(err.Error(), "channel_not_found") {
		t.Fatalf("Post = %v; want the Slack error", err)
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
			Body: io.NopCloser(strings.NewReader("oops " + l.secret))}, nil
	}
	return &http.Response{StatusCode: 200, Status: "200 OK",
		Body: io.NopCloser(strings.NewReader("<html>" + l.secret))}, nil
}

func TestSlackErrorsCarryNoToken(t *testing.T) {
	// Security: a failing Post never puts the token or the URL in its error: connection failure,
	// non-2xx, malformed reply.
	// covers: R-NTF-007, R-NTF-042
	const token = "test-token"
	for _, mode := range []string{"conn", "status", "statusline", "malformed"} {
		c := slack.Channel{Token: token, ChannelID: "C0TEST", HTTP: &http.Client{Transport: leaky{token, mode}}}
		_, err := c.Post(notify.Part{Blocks: []string{"hi"}}, "")
		if err == nil {
			t.Fatalf("%s: no error", mode)
		}
		if strings.Contains(err.Error(), token) || strings.Contains(err.Error(), "slack.com") {
			t.Errorf("%s: error leaks the token or URL: %v", mode, err)
		}
	}
}

func TestSlackEscapes(t *testing.T) {
	// Slack reads & < > as control characters: each is escaped in text and in code.
	// covers: R-NTF-020
	for in, want := range map[string]string{
		"a & b <c> d":             "a &amp; b &lt;c&gt; d",
		"```\nif a<b && c>d\n```": "```\nif a&lt;b &amp;&amp; c&gt;d\n```",
	} {
		if got := (slack.Channel{}).Render(notify.Parse(in)[0]); got != want {
			t.Errorf("Render(%q) = %q, want %q", in, got, want)
		}
	}
}

func FuzzRender(f *testing.F) {
	// covers: R-NTF-013
	f.Add("# h\n\n| a |\n|---|\n| <1 |\n\n```go\nx&y\n```\n- <i> & **b**")
	f.Fuzz(func(t *testing.T, md string) {
		c := slack.Channel{ChannelID: "C0TEST"}
		blocks := notify.Parse(md)
		for _, b := range blocks {
			if r := c.Render(b); strings.ContainsAny(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(
				r, "&amp;", ""), "&lt;", ""), "&gt;", ""), "<>") {
				t.Fatalf("unescaped < or > in %q", r)
			}
		}
		parts, err := notify.Split(c, blocks)
		for _, p := range parts {
			if err == nil && !json.Valid(c.Payload(p, "")) {
				t.Fatal("invalid payload JSON")
			}
		}
	})
}

func TestSlackOKFalseEchoingTokenIsRedacted(t *testing.T) {
	// covers: R-NTF-007, R-NTF-042
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"ok":false,"error":"bad test-token"}`)
	}))
	defer srv.Close()
	c := slack.Channel{Token: "test-token", ChannelID: "C0TEST", HTTP: srv.Client(), URL: srv.URL}
	_, err := c.Post(notify.Part{Blocks: []string{"hi"}}, "")
	if err == nil || strings.Contains(err.Error(), "test-token") || !strings.Contains(err.Error(), "bad") {
		t.Fatalf("Post = %v; want the error with the token masked", err)
	}
}

func TestSlackNeverFollowsRedirects(t *testing.T) {
	// A 307 must not carry the token or body to the Location: the second server sees nothing.
	// covers: R-NTF-042
	var hits atomic.Int32
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hits.Add(1) }))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	c := slack.Channel{Token: "test-token", ChannelID: "C0TEST", HTTP: srv.Client(), URL: srv.URL}
	_, err := c.Post(notify.Part{Blocks: []string{"hi"}}, "")
	if err == nil || strings.Contains(err.Error(), "test-token") || hits.Load() != 0 {
		t.Fatalf("Post = %v, redirect target hits = %d; want a failed post and no hit", err, hits.Load())
	}
}

func TestSlackContractOverFake(t *testing.T) {
	// covers: R-NTF-005, R-NTF-023, R-QA-024
	f := testkit.FakeHTTP{Dir: t.TempDir()}
	c := slack.Channel{Token: "test-token", ChannelID: "C0TEST", HTTP: &http.Client{Transport: f}}
	testkit.ChannelContract(t, c, "", func(n int) (string, string) {
		reqs, err := f.Requests()
		if err != nil || len(reqs) < n {
			t.Fatalf("requests: %v, %d", err, len(reqs))
		}
		return reqs[n-1].URL, reqs[n-1].Body
	})
}
