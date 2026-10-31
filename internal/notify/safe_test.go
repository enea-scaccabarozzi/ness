package notify_test

import (
	"net/http"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
)

func TestHardenedClient(t *testing.T) {
	// A client never follows redirects and always has a timeout; the caller's own is kept.
	// covers: R-NTF-042
	h := notify.Hardened(&http.Client{})
	if h.Timeout != notify.DefaultTimeout || h.CheckRedirect == nil || h.CheckRedirect(nil, nil) != http.ErrUseLastResponse {
		t.Fatalf("Hardened = %+v", h)
	}
	if got := notify.Hardened(&http.Client{Timeout: 5}).Timeout; got != 5 {
		t.Fatalf("own timeout = %v, want 5", got)
	}
}
