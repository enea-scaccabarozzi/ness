package github_test

import (
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/github"
	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestGitHubGoldenAndPayload(t *testing.T) {
	// GFM as is: every construct renders to its own source, tables native; the body is the
	// input; the limit is 65,536 characters (R-NTF-028, R-NTF-041).
	// covers: R-NTF-010, R-NTF-028, R-NTF-041
	c := github.Channel{Repo: "o/r"}
	for name, got := range testkit.RenderConstructs(t, c) {
		testkit.Golden(t, filepath.Join("testdata", "golden", name+".txt"), got)
	}
	md := "# T\n\n| a |\n|---|\n| 1 |\n"
	parts, err := notify.Split(c, notify.Parse(md))
	if err != nil || len(parts) != 1 || string(c.Payload(parts[0], "42")) != `{"body":"# T\n\n| a |\n|---|\n| 1 |"}` {
		t.Fatalf("payload = %s, %v", c.Payload(parts[0], "42"), err)
	}
	if parts, err = notify.Split(c, notify.Parse(strings.Repeat("é", 65536))); err != nil || len(parts) != 1 {
		t.Fatalf("65,536 code points: %d parts, %v", len(parts), err)
	}
	if !c.NeedsThread() {
		t.Fatal("github posts need an issue")
	}
}

func TestGitHubPostsACommentOnTheThreadIssue(t *testing.T) {
	// covers: R-NTF-028
	var path, auth, body string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path, auth = r.URL.Path, r.Header.Get("Authorization")
		b, _ := io.ReadAll(r.Body)
		body = string(b)
		w.WriteHeader(http.StatusCreated)
		_, _ = io.WriteString(w, `{"id":1}`)
	}))
	defer srv.Close()
	c := github.Channel{Token: "test-token", Repo: "o/r", URL: srv.URL, HTTP: srv.Client()}
	id, err := c.Post(notify.Part{Blocks: []string{"hi"}}, "42")
	if err != nil || id != "42" || path != "/repos/o/r/issues/42/comments" || auth != "Bearer test-token" || body != `{"body":"hi"}` {
		t.Fatalf("Post = %q, %v; path %q auth %q body %q", id, err, path, auth, body)
	}
	if _, err = c.Post(notify.Part{Blocks: []string{"hi"}}, ""); err == nil {
		t.Fatal("a post without a thread must fail")
	}
}

type leaky struct{ secret, mode string }

func (l leaky) RoundTrip(*http.Request) (*http.Response, error) {
	switch l.mode {
	case "conn":
		return nil, errors.New("dial failed for " + l.secret)
	case "statusline":
		return &http.Response{StatusCode: 502, Status: "502 " + l.secret, Body: io.NopCloser(strings.NewReader("x"))}, nil
	}
	return &http.Response{StatusCode: 500, Status: "500 Internal Server Error",
		Body: io.NopCloser(strings.NewReader("oops " + l.secret))}, nil
}

func TestGitHubErrorsCarryNoTokenOrURL(t *testing.T) {
	// Security: a failing Post never puts the token or the request URL in its error.
	// covers: R-NTF-007, R-NTF-042
	const token = "ghp-test-token"
	for _, mode := range []string{"conn", "status", "statusline"} {
		c := github.Channel{Token: token, Repo: "o/r", HTTP: &http.Client{Transport: leaky{token, mode}}}
		_, err := c.Post(notify.Part{Blocks: []string{"hi"}}, "42")
		if err == nil {
			t.Fatalf("%s: no error", mode)
		}
		if strings.Contains(err.Error(), token) || strings.Contains(err.Error(), "api.github.com") {
			t.Errorf("%s: error leaks the token or URL: %v", mode, err)
		}
	}
}

func TestGitHubNeverFollowsARedirect(t *testing.T) {
	// A 307 must not carry the token or body to the Location: the second server sees nothing.
	// covers: R-NTF-042
	hit := false
	other := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { hit = true }))
	defer other.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL, http.StatusTemporaryRedirect)
	}))
	defer srv.Close()
	c := github.Channel{Token: "test-token", Repo: "o/r", URL: srv.URL, HTTP: srv.Client()}
	if _, err := c.Post(notify.Part{Blocks: []string{"hi"}}, "42"); err == nil || hit {
		t.Fatalf("redirect followed (hit %v) or no error (%v)", hit, err)
	}
}

func FuzzRender(f *testing.F) {
	// covers: R-NTF-013
	f.Add("# h\n\n| a |\n|---|\n| 1 |\n\n```go\nx\n```\n- **b**")
	f.Fuzz(func(t *testing.T, md string) {
		c := github.Channel{Repo: "o/r"}
		parts, err := notify.Split(c, notify.Parse(md))
		for _, p := range parts {
			if err == nil && c.Measure(p.Text()) > c.Limit() {
				t.Fatal("part over the limit")
			}
		}
	})
}

func TestGitHubRefusesBadRepoOrThreadBeforeAnyRequest(t *testing.T) {
	// Security (F19): the repo and thread build the URL path; anything else is refused with no
	// request, so the token is never sent.
	// covers: R-NTF-042
	reqs := 0
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { reqs++ }))
	defer srv.Close()
	for _, tc := range []struct{ repo, thread string }{
		{"o/r?x=1", "42"}, {"o/..", "42"}, {"../r", "42"}, {"o/r/../../x", "42"}, {"o", "42"}, {"o/r", "probe:x"},
		{"o/r", "42/../1"}, {"o/r", "-1"},
	} {
		c := github.Channel{Token: "test-token", Repo: tc.repo, URL: srv.URL, HTTP: srv.Client()}
		if _, err := c.Post(notify.Part{Blocks: []string{"hi"}}, tc.thread); err == nil {
			t.Errorf("repo %q thread %q: no error", tc.repo, tc.thread)
		}
	}
	if reqs != 0 {
		t.Fatalf("%d requests reached the server", reqs)
	}
}

func TestGitHubContractOverFake(t *testing.T) {
	// covers: R-NTF-005, R-NTF-028, R-QA-024
	f := testkit.FakeHTTP{Dir: t.TempDir()}
	c := github.Channel{Token: "test-token", Repo: "o/r", HTTP: &http.Client{Transport: f}}
	testkit.ChannelContract(t, c, "42", func(n int) (string, string) {
		reqs, err := f.Requests()
		if err != nil || len(reqs) < n {
			t.Fatalf("requests: %v, %d", err, len(reqs))
		}
		return reqs[n-1].URL, reqs[n-1].Body
	})
}
