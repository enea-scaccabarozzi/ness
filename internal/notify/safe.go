package notify

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// SafeErr is err with no URL and no secret in its text: a *url.Error is unwrapped to its cause
// (it prints the request URL, which for Telegram carries the bot token) and every occurrence of
// secret is masked. Channels return it for any transport failure (R-NTF-042).
func SafeErr(err error, secret string) error {
	var ue *url.Error
	if errors.As(err, &ue) {
		err = ue.Err
	}
	return errors.New(Redact(err.Error(), secret))
}

// Redact masks secret in s.
func Redact(s, secret string) string {
	if secret == "" {
		return s
	}
	return strings.ReplaceAll(s, secret, "***")
}

// DefaultTimeout bounds a post whose client set none.
const DefaultTimeout = 30 * time.Second

// Hardened is a copy of c that never follows a redirect (a 3xx is a failed post, so the token,
// body and Authorization header never travel to a Location the server chose) and always has a
// timeout: c's own, or DefaultTimeout when it has none (R-NTF-042).
func Hardened(c *http.Client) *http.Client {
	h := *c
	h.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	if h.Timeout == 0 {
		h.Timeout = DefaultTimeout
	}
	return &h
}
