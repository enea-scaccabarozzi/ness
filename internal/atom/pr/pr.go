// Package pr is the integrator that opens a pull request from the session's branch and answers
// integrated? once it is merged (R-SESS-096, §16 #12). The gate runs on a local merge preview;
// only after it passes does Publish push the source and open the request. The local target never
// moves.
package pr

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/atom/merge"
	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// Exec runs a program; cli.SystemExec in production, the fake in tests.
type Exec func(dir, name string, args ...string) ([]byte, error)

// Integrator implements session.Integrator and session.Publisher over a Forge.
type Integrator struct {
	Forge  session.Forge
	Remote string // "" means origin
}

func (it Integrator) remote() string {
	if it.Remote == "" {
		return "origin"
	}
	return it.Remote
}

// Fold previews the merge locally in dir, for the gate. It pushes nothing and opens nothing.
func (Integrator) Fold(g session.Git, dir, source, target string) (string, error) {
	return (merge.Integrator{}).Fold(g, dir, source, target)
}

// Publish pushes the source and opens (or finds) the pull request; the returned ref is its url.
// The caller runs it only after the gate passed (R-SESS-092).
func (it Integrator) Publish(g session.Git, repo, source, target string) (string, error) {
	if _, err := g.Out(repo, "push", "-q", it.remote(), source+":"+source); err != nil {
		return "", err
	}
	return it.Forge.OpenPR(repo, source, target, "integrate "+source+" into "+target)
}

// Integrated is true when the pair's PR is merged; without a PR it is false.
func (it Integrator) Integrated(_ session.Git, repo, source, target string) (bool, error) {
	url, err := it.Forge.FindPR(repo, source, target)
	if err != nil || url == "" {
		return false, err
	}
	return it.Forge.Merged(repo, url)
}

// GH is the forge over the gh CLI.
type GH struct{ Exec Exec }

// FindPR returns the pair's current PR, or "": an open one wins, else a merged one whose head is
// the source's current tip. A merged PR the source has since outgrown, or a closed one, does not
// count, so a new request opens for the new commits. gh's row order is not relied on.
func (f GH) FindPR(repo, head, base string) (string, error) {
	out, err := f.Exec(repo, "gh", "pr", "list", "--head", head, "--base", base,
		"--state", "all", "--json", "url,state,headRefOid")
	if err != nil {
		return "", err
	}
	var prs []struct{ URL, State, HeadRefOid string }
	if err = json.Unmarshal(out, &prs); err != nil {
		return "", fmt.Errorf("gh pr list: %w", err)
	}
	for _, p := range prs {
		if p.State == "OPEN" {
			return p.URL, nil
		}
	}
	tip, err := f.Exec(repo, "git", "rev-parse", "--verify", head+"^{commit}")
	if err != nil {
		return "", err
	}
	for _, p := range prs {
		if p.State == "MERGED" && p.HeadRefOid == strings.TrimSpace(string(tip)) {
			return p.URL, nil
		}
	}
	return "", nil
}

// OpenPR returns the PR for head→base or creates one.
func (f GH) OpenPR(repo, head, base, title string) (string, error) {
	if url, err := f.FindPR(repo, head, base); err != nil || url != "" {
		return url, err
	}
	out, err := f.Exec(repo, "gh", "pr", "create", "--head", head, "--base", base,
		"--title", title, "--body", "opened by ness integrate")
	if err != nil {
		return "", err
	}
	url := strings.TrimSpace(string(out))
	if url == "" {
		return "", errors.New("gh pr create printed no url")
	}
	return url, nil
}

// Merged reports whether the PR's state is MERGED.
func (f GH) Merged(repo, url string) (bool, error) {
	out, err := f.Exec(repo, "gh", "pr", "view", url, "--json", "state")
	if err != nil {
		return false, err
	}
	var v struct{ State string }
	if err := json.Unmarshal(out, &v); err != nil {
		return false, fmt.Errorf("gh pr view: %w", err)
	}
	return v.State == "MERGED", nil
}
