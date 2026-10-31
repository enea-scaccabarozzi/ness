// Package releases is the release-source atom: the GitHub releases API of the ness repository over HTTP.
package releases

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/update"
)

// DefaultBase is the API root of the ness repository; NESS_RELEASES_URL replaces it.
const DefaultBase = "https://api.github.com/repos/enea-scaccabarozzi/ness"

// MaxDownload caps one response body: reading past it is an error, never a truncated archive.
const MaxDownload = 200 << 20

// Client lists the releases under Base and fetches their assets through HTTP.
type Client struct {
	Base string
	HTTP *http.Client
}

// release is the part of a GitHub release object ness reads.
type release struct {
	Tag        string `json:"tag_name"`
	Prerelease bool   `json:"prerelease"`
	Draft      bool   `json:"draft"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

// ListTimeout bounds the release listing; asset downloads have only the HTTP client's timeout.
const ListTimeout = 5 * time.Second

// Releases lists the published releases (drafts skipped) in the order the API returns them,
// versions without the leading v and assets by name → download URL, within ListTimeout.
func (c Client) Releases(ctx context.Context) (rels []update.Release, err error) {
	ctx, cancel := context.WithTimeout(ctx, ListTimeout)
	defer cancel()
	body, err := c.Fetch(ctx, c.Base+"/releases?per_page=100")
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, body.Close()) }()
	var raw []release
	if err := json.NewDecoder(body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("releases: %w", err)
	}
	for _, r := range raw {
		if r.Draft {
			continue
		}
		assets := make(map[string]string, len(r.Assets))
		for _, a := range r.Assets {
			assets[a.Name] = a.URL
		}
		v := strings.TrimPrefix(r.Tag, "v")
		rels = append(rels, update.Release{Version: v, Prerelease: r.Prerelease, Assets: assets})
	}
	return rels, nil
}

// Fetch opens url; any non-2xx answer is an error naming the url and the status, and a body past
// MaxDownload fails the read. The caller closes the body.
func (c Client) Fetch(ctx context.Context, url string) (io.ReadCloser, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("releases: %w", err)
	}
	resp, err := c.HTTP.Do(req) //nolint:gosec // the URL is the configured release source or one of its assets
	if err != nil {
		return nil, fmt.Errorf("releases: %w", err)
	}
	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return nil, errors.Join(fmt.Errorf("releases: GET %s: %s", url, resp.Status), resp.Body.Close())
	}
	return &capped{body: resp.Body, r: io.LimitReader(resp.Body, MaxDownload+1), url: url}, nil
}

// capped reads a body and fails once it passes MaxDownload.
type capped struct {
	body io.ReadCloser
	r    io.Reader
	n    int64
	url  string
}

func (c *capped) Read(p []byte) (int, error) {
	n, err := c.r.Read(p)
	c.n += int64(n)
	if c.n > MaxDownload {
		return n, fmt.Errorf("releases: GET %s: larger than %d MiB", c.url, MaxDownload>>20)
	}
	return n, err
}

func (c *capped) Close() error { return c.body.Close() }
