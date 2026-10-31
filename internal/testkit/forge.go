package testkit

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rogpeppe/go-internal/testscript"
)

type fakePR struct {
	URL, Head, Base, Tip string
	Merged               bool
}

type forgeState struct {
	PRs []fakePR `json:"prs"`
}

// FakeForge is the pull-request fake (R-SESS-096, R-SESS-071): state in Dir/forge.json.
type FakeForge struct{ Dir string }

func (f FakeForge) path() string { return filepath.Join(f.Dir, "forge.json") }

func (f FakeForge) load() (forgeState, error) {
	var st forgeState
	err := loadJSON(f.path(), &st)
	return st, err
}

// tipOf is head's current commit in repo, as gh reports headRefOid.
func tipOf(repo, head string) string {
	out, err := exec.CommandContext(context.Background(), "git", "-C", repo, //nolint:gosec // test fake
		"rev-parse", "refs/heads/"+head).Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// FindPR returns the pair's PR url, or "": an open PR, or a merged one whose head is still the
// branch's tip (a merged PR the branch has outgrown does not count), like GH.FindPR.
func (f FakeForge) FindPR(repo, head, base string) (string, error) {
	st, err := f.load()
	if err != nil {
		return "", err
	}
	for _, p := range st.PRs {
		if p.Head == head && p.Base == base && (!p.Merged || p.Tip == tipOf(repo, head)) {
			return p.URL, nil
		}
	}
	return "", nil
}

// OpenPR records a PR per head/base pair and returns its url.
func (f FakeForge) OpenPR(repo, head, base, _ string) (string, error) {
	if url, err := f.FindPR(repo, head, base); err != nil || url != "" {
		return url, err
	}
	st, err := f.load()
	if err != nil {
		return "", err
	}
	p := fakePR{
		URL: fmt.Sprintf("https://example.invalid/pr/%d", len(st.PRs)+1), Head: head, Base: base,
		Tip: tipOf(repo, head),
	}
	st.PRs = append(st.PRs, p)
	return p.URL, saveJSON(f.path(), st)
}

// Merged reports the PR's state.
func (f FakeForge) Merged(_, url string) (bool, error) {
	st, err := f.load()
	if err != nil {
		return false, err
	}
	for _, p := range st.PRs {
		if p.URL == url {
			return p.Merged, nil
		}
	}
	return false, fmt.Errorf("no pull request %s", url)
}

// Merge marks a PR merged (what a human does on GitHub).
func (f FakeForge) Merge(url string) error {
	st, err := f.load()
	if err != nil {
		return err
	}
	for i := range st.PRs {
		if st.PRs[i].URL == url {
			st.PRs[i].Merged = true
		}
	}
	return saveJSON(f.path(), st)
}

func loadJSON(path string, v any) error {
	data, err := os.ReadFile(path) //nolint:gosec // the fakes dir of a test
	if err != nil {
		return err
	}
	return json.Unmarshal(data, v)
}

// forgeCmd runs the script command fake forge: prs N, merge URL or last VAR.
func forgeCmd(ts *testscript.TestScript, neg bool, dir string, args []string) {
	f := FakeForge{Dir: dir}
	st, err := f.load()
	ts.Check(err)
	switch args[0] {
	case "prs":
		n, aerr := strconv.Atoi(args[1])
		ts.Check(aerr)
		if (len(st.PRs) == n) == neg {
			ts.Fatalf("fake forge has %d PRs, want %s", len(st.PRs), args[1])
		}
	case "merge":
		ts.Check(f.Merge(args[1]))
	case "last":
		if len(st.PRs) == 0 {
			ts.Fatalf("fake forge has no PR")
		}
		ts.Setenv(args[1], st.PRs[len(st.PRs)-1].URL)
	default:
		ts.Fatalf("fake forge: unknown subcommand %s", args[0])
	}
}
