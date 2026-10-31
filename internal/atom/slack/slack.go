// Package slack is the Slack channel atom: Markdown rendered to markdown blocks of
// chat.postMessage, threaded by thread_ts (design §9).
package slack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
)

// API is chat.postMessage.
const API = "https://slack.com/api/chat.postMessage"

// Channel posts to one Slack channel. URL overrides API in tests.
type Channel struct {
	Token, ChannelID, URL string
	HTTP                  *http.Client
}

// Kind implements notify.Channel.
func (Channel) Kind() string { return "slack" }

// NeedsThread implements notify.Channel.
func (Channel) NeedsThread() bool { return false }

// ThreadIsKey implements notify.Channel.
func (Channel) ThreadIsKey() bool { return false }

// Limit is the markdown block text per message (R-NTF-021). Split counts the blank line between
// blocks although Slack sends separate blocks: a message is never over the limit, at worst one
// more message than strictly needed.
func (Channel) Limit() int { return 12000 }

// MaxBlocks is the blocks per message (R-NTF-021).
func (Channel) MaxBlocks() int { return 50 }

// Measure counts code points (R-NTF-041).
func (Channel) Measure(s string) int { return utf8.RuneCountInString(s) }

// escaper is Slack's control-character escaping: & < > only.
var escaper = strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;")

// Render keeps Markdown for a markdown block (R-NTF-020) with & < > escaped, in code too; a
// table becomes a code block (R-NTF-022).
func (Channel) Render(b notify.Block) string {
	if b.Kind == notify.Table {
		return "```\n" + escaper.Replace(b.Raw) + "\n```"
	}
	return escaper.Replace(b.Raw)
}

type block struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type message struct {
	Channel  string  `json:"channel"`
	Blocks   []block `json:"blocks"`
	ThreadTS string  `json:"thread_ts,omitempty"`
}

// Payload is the exact chat.postMessage body (R-NTF-003): one markdown block per block.
func (c Channel) Payload(p notify.Part, thread string) []byte {
	m := message{Channel: c.ChannelID, ThreadTS: thread, Blocks: make([]block, 0, len(p.Blocks))}
	for _, t := range p.Blocks {
		m.Blocks = append(m.Blocks, block{Type: "markdown", Text: t})
	}
	out, _ := json.Marshal(m) //nolint:errcheck,errchkjson // plain strings never fail to encode
	return out
}

// Post sends one part and returns its ts, the thread id (R-NTF-023). No error carries the
// request URL or the token (R-NTF-042).
func (c Channel) Post(p notify.Part, thread string) (string, error) {
	url := c.URL
	if url == "" {
		url = API
	}
	if c.HTTP == nil {
		return "", errors.New("slack: no HTTP client")
	}
	body := bytes.NewReader(c.Payload(p, thread))
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, url, body)
	if err != nil {
		return "", fmt.Errorf("slack: %w", notify.SafeErr(err, c.Token))
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	res, err := notify.Hardened(c.HTTP).Do(req)
	if err != nil {
		return "", fmt.Errorf("slack: %w", notify.SafeErr(err, c.Token))
	}
	defer func() { _ = res.Body.Close() }() //nolint:errcheck // read fully below
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("slack: %w", notify.SafeErr(err, c.Token))
	}
	var reply struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
		TS    string `json:"ts"`
	}
	// The raw body is never quoted: only Slack's error code, masked, reaches the error.
	if json.Unmarshal(raw, &reply) != nil {
		return "", fmt.Errorf("slack: %s: unreadable reply", notify.Redact(res.Status, c.Token))
	}
	if res.StatusCode != http.StatusOK || !reply.OK {
		return "", fmt.Errorf("slack: %s", notify.Redact(res.Status+": "+reply.Error, c.Token))
	}
	if reply.TS == "" {
		return "", errors.New("slack: no ts in the reply")
	}
	return reply.TS, nil
}
