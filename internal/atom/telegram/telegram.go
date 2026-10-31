// Package telegram is the Telegram channel atom: Markdown rendered to HTML parse mode, posted
// with sendMessage, threaded as a reply (design §9).
//
// HTML was chosen over MarkdownV2: its reserved characters are only & < > (plus the quote inside
// an attribute), all escaped with html.EscapeString, where MarkdownV2 reserves eighteen
// punctuation marks that are easy to miss.
package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
)

// API is the Bot API base; the token and method follow.
const API = "https://api.telegram.org"

// Channel posts to one chat. URL overrides API in tests.
type Channel struct {
	Token, ChatID, URL string
	HTTP               *http.Client
}

// Kind implements notify.Channel.
func (Channel) Kind() string { return "telegram" }

// NeedsThread implements notify.Channel.
func (Channel) NeedsThread() bool { return false }

// ThreadIsKey implements notify.Channel.
func (Channel) ThreadIsKey() bool { return false }

// Limit is sendMessage's text limit (R-NTF-025).
func (Channel) Limit() int { return 4096 }

// MaxBlocks implements notify.Renderer: none.
func (Channel) MaxBlocks() int { return 0 }

var tagRe = regexp.MustCompile(`<[^>]*>`)

// Measure counts UTF-16 code units of the text after entity parsing: tags removed, entities
// decoded (R-NTF-025).
func (Channel) Measure(s string) int {
	return len(utf16.Encode([]rune(html.UnescapeString(tagRe.ReplaceAllString(s, "")))))
}

// Render turns one block into HTML (R-NTF-024): headings bold, lists as bullet lines (numbers
// are not kept), fences and tables in <pre> (R-NTF-026), text escaped.
func (Channel) Render(b notify.Block) string {
	switch b.Kind {
	case notify.Heading:
		return "<b>" + inlines(b.Text) + "</b>"
	case notify.Fence:
		if b.Lang != "" {
			return `<pre><code class="language-` + html.EscapeString(b.Lang) + `">` +
				html.EscapeString(b.Text) + "</code></pre>"
		}
		return "<pre>" + html.EscapeString(b.Text) + "</pre>"
	case notify.Table:
		return "<pre>" + html.EscapeString(b.Raw) + "</pre>"
	case notify.List:
		lines := make([]string, 0, len(b.Items))
		for _, it := range b.Items {
			lines = append(lines, "• "+inlines(it))
		}
		return strings.Join(lines, "\n")
	case notify.Paragraph:
	}
	return inlines(b.Text)
}

// inlines renders the spans of s; bold and italic render the spans inside them, so a bold link
// is a link (R-NTF-024).
func inlines(s string) string {
	var out strings.Builder
	for _, in := range notify.Inlines(s) {
		t := html.EscapeString(in.Text)
		switch in.Kind {
		case notify.Bold:
			out.WriteString("<b>" + inlines(in.Text) + "</b>")
		case notify.Italic:
			out.WriteString("<i>" + inlines(in.Text) + "</i>")
		case notify.Code:
			out.WriteString("<code>" + t + "</code>")
		case notify.Link:
			if !safeHref(in.URL) {
				out.WriteString(t) // any other scheme stays plain text
				continue
			}
			out.WriteString(`<a href="` + html.EscapeString(in.URL) + `">` + t + "</a>")
		case notify.Text:
			out.WriteString(t)
		}
	}
	return out.String()
}

// safeHref allows only http, https and mailto targets.
func safeHref(raw string) bool {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	switch strings.ToLower(u.Scheme) {
	case "http", "https", "mailto":
		return true
	}
	return false
}

type message struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode"`
	ReplyTo   int    `json:"reply_to_message_id,omitempty"`
}

// Payload is the exact sendMessage body (R-NTF-003, R-NTF-027).
func (c Channel) Payload(p notify.Part, thread string) []byte {
	reply, err := strconv.Atoi(thread)
	if err != nil {
		reply = 0 // "" or anything but a message id starts a new thread
	}
	out, _ := json.Marshal(message{ //nolint:errcheck,errchkjson // plain strings
		ChatID: c.ChatID, Text: p.Text(), ParseMode: "HTML", ReplyTo: reply,
	})
	return out
}

// Post sends one part and returns its message id, the thread id for replies (R-NTF-027). The
// request URL carries the bot token, so no error quotes it or the token (R-NTF-042).
func (c Channel) Post(p notify.Part, thread string) (string, error) {
	base := c.URL
	if base == "" {
		base = API
	}
	if c.HTTP == nil {
		return "", errors.New("telegram: no HTTP client")
	}
	endpoint := base + "/bot" + c.Token + "/sendMessage"
	body := bytes.NewReader(c.Payload(p, thread))
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, endpoint, body)
	if err != nil {
		return "", fmt.Errorf("telegram: %w", notify.SafeErr(err, c.Token))
	}
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	res, err := notify.Hardened(c.HTTP).Do(req)
	if err != nil {
		return "", fmt.Errorf("telegram: %w", notify.SafeErr(err, c.Token))
	}
	defer func() { _ = res.Body.Close() }() //nolint:errcheck // read fully below
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("telegram: %w", notify.SafeErr(err, c.Token))
	}
	var reply struct {
		OK          bool   `json:"ok"`
		Description string `json:"description"`
		Result      struct {
			MessageID int `json:"message_id"`
		} `json:"result"`
	}
	// The raw body is never quoted: only Telegram's description, masked, reaches the error.
	if json.Unmarshal(raw, &reply) != nil {
		return "", fmt.Errorf("telegram: %s: unreadable reply", notify.Redact(res.Status, c.Token))
	}
	if res.StatusCode != http.StatusOK || !reply.OK {
		return "", fmt.Errorf("telegram: %s", notify.Redact(res.Status+": "+reply.Description, c.Token))
	}
	return strconv.Itoa(reply.Result.MessageID), nil
}
