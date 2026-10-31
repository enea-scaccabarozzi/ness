// Package github is the GitHub channel atom: GFM as is, one comment per part on the issue or
// pull request the thread key names (design §9).
package github

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
)

// API is the REST base. URL overrides it in tests.
const API = "https://api.github.com"

var (
	threadRE = regexp.MustCompile(`^\d+$`)
	repoRE   = regexp.MustCompile(`^[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+$`)
)

// validRepo is owner/name with neither part "." or "..", so the repo cannot rewrite the URL path.
func validRepo(r string) bool {
	if !repoRE.MatchString(r) {
		return false
	}
	for _, seg := range strings.Split(r, "/") {
		if seg == "." || seg == ".." {
			return false
		}
	}
	return true
}

// Channel comments on issues of one repository (owner/name).
type Channel struct {
	Token, Repo, URL string
	HTTP             *http.Client
}

// Kind implements notify.Channel.
func (Channel) Kind() string { return "github" }

// NeedsThread is true: a comment needs its issue or PR number (R-NTF-028).
func (Channel) NeedsThread() bool { return true }

// ThreadIsKey implements notify.Channel.
func (Channel) ThreadIsKey() bool { return true }

// Limit is the comment body limit (R-NTF-028).
func (Channel) Limit() int { return 65536 }

// MaxBlocks implements notify.Renderer: none.
func (Channel) MaxBlocks() int { return 0 }

// Measure counts characters (R-NTF-041).
func (Channel) Measure(s string) int { return utf8.RuneCountInString(s) }

// Render is the identity: GFM as is, tables native (R-NTF-028).
func (Channel) Render(b notify.Block) string { return b.Raw }

// Payload is the exact comment body (R-NTF-003).
func (Channel) Payload(p notify.Part, _ string) []byte {
	out, _ := json.Marshal(map[string]string{"body": p.Text()}) //nolint:errcheck,errchkjson // strings
	return out
}

// Post comments on issue thread and returns thread: the issue is the thread (R-NTF-028). No
// error carries the request URL or the token (R-NTF-042).
func (c Channel) Post(p notify.Part, thread string) (string, error) {
	if thread == "" {
		return "", errors.New("github: a post is a comment on an issue or PR; pass --thread NUMBER")
	}
	if !threadRE.MatchString(thread) {
		return "", errors.New("github: the thread must be an issue or PR number")
	}
	if !validRepo(c.Repo) {
		return "", errors.New("github: the repo must be owner/name")
	}
	if c.HTTP == nil {
		return "", errors.New("github: no HTTP client")
	}
	base := c.URL
	if base == "" {
		base = API
	}
	target := fmt.Sprintf("%s/repos/%s/issues/%s/comments", base, c.Repo, thread)
	req, err := http.NewRequestWithContext(context.Background(), http.MethodPost, target,
		bytes.NewReader(c.Payload(p, thread)))
	if err != nil {
		return "", fmt.Errorf("github: %w", notify.SafeErr(err, c.Token))
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Content-Type", "application/json")
	res, err := notify.Hardened(c.HTTP).Do(req)
	if err != nil {
		return "", fmt.Errorf("github: %w", notify.SafeErr(err, c.Token))
	}
	defer func() { _ = res.Body.Close() }() //nolint:errcheck // read fully below
	raw, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("github: %w", notify.SafeErr(err, c.Token))
	}
	if res.StatusCode != http.StatusCreated {
		// Only GitHub's message field, masked; the raw body is never quoted.
		var reply struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(raw, &reply) //nolint:errcheck // an unreadable body just has no message
		return "", fmt.Errorf("github: %s", notify.Redact(res.Status+": "+reply.Message, c.Token))
	}
	return thread, nil
}
