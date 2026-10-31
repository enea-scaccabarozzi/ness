package testkit

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/notify"
	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// RuntimeContract is the runtime seam's contract (R-SESS-110), run against the herdr atom over
// the fake and, under the e2e tag, over the real herdr. startAgent is false for the real run:
// starting a client there costs budget; the e2e launch covers it.
func RuntimeContract(t *testing.T, rt session.Runtime, startAgent bool) {
	t.Helper()
	v, err := rt.Version()
	if err != nil || v == "" {
		t.Fatalf("Version = %q, %v", v, err)
	}
	env := map[string]string{"NESS_SESSION": "s-000000000001"}
	tab, err := rt.OpenTab(session.TabSpec{Dir: t.TempDir(), Label: "ness-contract", Env: env})
	if err != nil || tab.ID == "" || tab.Pane == "" {
		t.Fatalf("OpenTab = %+v, %v", tab, err)
	}
	t.Cleanup(func() { _ = rt.CloseTab(tab.ID) }) //nolint:errcheck // best-effort cleanup; the test closes it itself
	contractListed(t, rt, tab)
	contractPane(t, rt, tab)
	if startAgent {
		contractAgent(t, rt, tab)
	}
	contractClose(t, rt, tab)
}

// contractListed checks the open tab is listed with its pane and label.
func contractListed(t *testing.T, rt session.Runtime, tab session.Tab) {
	t.Helper()
	tabs, err := rt.Tabs()
	i := tabIndex(tabs, tab.ID)
	if err != nil || i < 0 || tabs[i].Pane != tab.Pane || tabs[i].Label != "ness-contract" {
		t.Fatalf("Tabs = %v, %v; want %v labeled ness-contract", tabs, err, tab)
	}
}

func tabIndex(tabs []session.Tab, id string) int {
	return slices.IndexFunc(tabs, func(x session.Tab) bool { return x.ID == id })
}

// contractPane drives a fresh pane: no agent, then a command, typed text and a read.
func contractPane(t *testing.T, rt session.Runtime, tab session.Tab) {
	t.Helper()
	st, err := rt.State(tab.Pane)
	if err != nil || st.Live {
		t.Fatalf("State of a fresh pane = %+v, %v; want no agent", st, err)
	}
	if err = rt.Run(tab.Pane, "true"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if err = rt.TypeText(tab.Pane, "echo contract"); err != nil {
		t.Fatalf("TypeText: %v", err)
	}
	if _, err = rt.ReadScreen(tab.Pane); err != nil {
		t.Fatalf("ReadScreen: %v", err)
	}
	if _, err = rt.ReadANSI(tab.Pane); err != nil {
		t.Fatalf("ReadANSI: %v", err)
	}
}

// contractClose closes the tab and checks that everything about it is gone.
func contractClose(t *testing.T, rt session.Runtime, tab session.Tab) {
	t.Helper()
	if err := rt.CloseTab(tab.ID); err != nil {
		t.Fatalf("CloseTab: %v", err)
	}
	if after, err := rt.Tabs(); err != nil || tabIndex(after, tab.ID) >= 0 {
		t.Fatalf("closed tab still listed: %v, %v", after, err)
	}
	if err := rt.CloseTab(tab.ID); !errors.Is(err, session.ErrGone) {
		t.Fatalf("closing a closed tab: %v, want ErrGone", err)
	}
	if _, err := rt.ReadScreen(tab.Pane); !errors.Is(err, session.ErrGone) {
		t.Fatalf("reading a closed pane: %v, want ErrGone", err)
	}
}

func contractAgent(t *testing.T, rt session.Runtime, tab session.Tab) {
	t.Helper()
	spec := session.AgentSpec{Name: "ness-contract", Pane: tab.Pane, Kind: "claude", Args: []string{"--effort", "low"}}
	if err := rt.Prompt(tab.Pane, "ness contract prompt"); err == nil {
		t.Fatal("Prompt to a pane without an agent was accepted")
	}
	if err := rt.StartAgent(spec); err != nil {
		t.Fatalf("StartAgent: %v", err)
	}
	names, err := rt.Agents()
	if err != nil || !slices.Contains(names, "ness-contract") {
		t.Fatalf("Agents = %v, %v", names, err)
	}
	if st, serr := rt.State(tab.Pane); serr != nil || !st.Live {
		t.Fatalf("State after start = %+v, %v", st, serr)
	}
	if err = rt.Prompt(tab.Pane, "ness contract prompt"); err != nil {
		t.Fatalf("Prompt to an idle agent: %v", err)
	}
	spec.Args = nil
	if err = rt.StartAgent(spec); err == nil || !strings.Contains(err.Error(), "agent_name_taken") {
		t.Fatalf("a taken name was accepted: %v", err)
	}
}

// ClientContract is the client seam's contract (R-SESS-111): tested version, efforts, the launch
// argv rules shared by every client (effort on the command line, never through the environment;
// no prompt in the argv: the launch submits it once the client is ready, R-SESS-025) and model
// resolution.
func ClientContract(t *testing.T, c session.Client) {
	t.Helper()
	v, err := c.Version()
	if c.Tested() == "" || c.Name() == "" || len(c.Efforts()) == 0 || err != nil || v == "" {
		t.Fatalf("Name %q Tested %q Efforts %v Version %q, %v", c.Name(), c.Tested(), c.Efforts(), v, err)
	}
	prompt := session.Prompt("s-aaaaaaaaaaaa", "root", "docs/sessions/root")
	effort := c.Efforts()[0]
	spec := session.LaunchSpec{Dir: "/w", Model: "m", Effort: effort, SessionID: c.NewSessionID()}
	contractArgv(t, c.Args(spec), prompt, effort)
	if err := c.ResolveModel("no-such-model-xyz"); err == nil {
		t.Fatal("an unknown model id resolved")
	}
}

// contractArgv checks the launch argv: no prompt, the effort on it, never through the environment.
func contractArgv(t *testing.T, args []string, prompt, effort string) {
	t.Helper()
	has := func(sub string) bool {
		return slices.ContainsFunc(args, func(a string) bool { return strings.Contains(a, sub) })
	}
	if has(prompt) || has("load ness-entry") {
		t.Fatalf("the prompt is in the argv: %q", args)
	}
	if !has(effort) {
		t.Fatalf("effort missing from argv %q", args)
	}
	if has("CLAUDE_CODE_EFFORT_LEVEL") {
		t.Fatalf("effort passed through the environment: %q", args)
	}
}

// FakeSelector is a selector that always answers Offset with Confidence, or Err (R-MOD-041).
type FakeSelector struct {
	Offset     int
	Confidence float64
	Err        error
}

// Choose implements session.Policy.
func (f FakeSelector) Choose(b session.Brief) (session.Verdict, error) {
	if f.Err != nil {
		return session.Verdict{}, f.Err
	}
	return session.Verdict{Offset: f.Offset, Confidence: f.Confidence, Reason: "fake selector: " + b.Role}, nil
}

// PolicyContract is the selector seam's contract (R-MOD-041): for a brief with a kind the
// answer has a confidence in 0..1 and a one-line reason.
func PolicyContract(t *testing.T, p session.Policy) {
	t.Helper()
	v, err := p.Choose(session.Brief{Text: "Implement the thing.", Role: "child", Client: "claude", Kind: "implement"})
	if err != nil {
		t.Fatalf("Choose: %v", err)
	}
	if v.Confidence < 0 || v.Confidence > 1 || v.Reason == "" || strings.Contains(v.Reason, "\n") {
		t.Fatalf("Verdict = %+v", v)
	}
}

// NewRepo makes a throwaway repository with one commit on main and returns its path.
func NewRepo(t *testing.T, g session.Git) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{
		{"init", "-q", "-b", "main"},
		{"config", "user.email", "t@x"},
		{"config", "user.name", "t"},
		{"config", "commit.gpgsign", "false"},
		{"commit", "-q", "--allow-empty", "-m", "init"},
	} {
		if _, err := g.Out(dir, args...); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Commit writes path with content on the current branch of dir and commits it. The identity is
// passed on the command line: dir may be a clone (a submodule) that NewRepo never configured, and
// CI has no global identity.
func Commit(t *testing.T, g session.Git, dir, path, content, msg string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	commit := []string{
		"-c", "user.email=t@x", "-c", "user.name=t", "-c", "commit.gpgsign=false",
		"commit", "-q", "-m", msg,
	}
	for _, args := range [][]string{{"add", path}, commit} {
		if _, err := g.Out(dir, args...); err != nil {
			t.Fatal(err)
		}
	}
	sha, err := g.Rev(dir, "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	return sha
}

// git runs git in dir and fails the test on error.
func git(t *testing.T, g session.Git, dir string, args ...string) string {
	t.Helper()
	out, err := g.Out(dir, args...)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// IntegratorContract is the integrator seam's contract (R-SESS-102): integrated? is false before
// the fold and true after; the fold lands the source's changes on the target; a conflicting
// change leaves the target untouched and names the path (R-SESS-097).
func IntegratorContract(t *testing.T, it session.Integrator, g session.Git) {
	t.Helper()
	repo := NewRepo(t, g)
	git(t, g, repo, "checkout", "-q", "-b", "session/x-1")
	Commit(t, g, repo, "a.txt", "from the session\n", "session work")
	git(t, g, repo, "checkout", "-q", "main")
	if ok, err := it.Integrated(g, repo, "session/x-1", "main"); err != nil || ok {
		t.Fatalf("Integrated before = %v, %v", ok, err)
	}
	tmp := filepath.Join(t.TempDir(), "tmp")
	git(t, g, repo, "worktree", "add", "-q", "--detach", tmp, "main")
	before := git(t, g, repo, "rev-parse", "main")
	ref, err := it.Fold(g, tmp, "session/x-1", "main")
	if err != nil || ref == "" {
		t.Fatalf("Fold = %q, %v", ref, err)
	}
	git(t, g, repo, "update-ref", "refs/heads/main", ref, before)
	if ok, ierr := it.Integrated(g, repo, "session/x-1", "main"); ierr != nil || !ok {
		t.Fatalf("Integrated after = %v, %v", ok, ierr)
	}
	if out := git(t, g, repo, "show", "main:a.txt"); out != "from the session" {
		t.Fatalf("main:a.txt = %q", out)
	}
	// A commit after the fold is not integrated: the answer is pinned at the tip (R-SESS-072).
	git(t, g, repo, "checkout", "-q", "session/x-1")
	Commit(t, g, repo, "later.txt", "after\n", "after the fold")
	git(t, g, repo, "checkout", "-q", "main")
	if ok, ierr := it.Integrated(g, repo, "session/x-1", "main"); ierr != nil || ok {
		t.Fatalf("Integrated after a later commit = %v, %v", ok, ierr)
	}
	contractConflict(t, it, g, repo, tmp, before)
}

// contractConflict folds a branch that conflicts with main in a fresh throwaway: the error is
// ErrConflict naming the path and main does not move (R-SESS-097).
func contractConflict(t *testing.T, it session.Integrator, g session.Git, repo, tmp, base string) {
	t.Helper()
	git(t, g, repo, "checkout", "-q", "-b", "session/y-2", base)
	Commit(t, g, repo, "a.txt", "conflicting\n", "conflict")
	git(t, g, repo, "checkout", "-q", "main")
	git(t, g, repo, "worktree", "remove", "--force", tmp)
	git(t, g, repo, "worktree", "add", "-q", "--detach", tmp, "main")
	mainBefore := git(t, g, repo, "rev-parse", "main")
	_, err := it.Fold(g, tmp, "session/y-2", "main")
	if !errors.Is(err, session.ErrConflict) || !strings.Contains(err.Error(), "a.txt") {
		t.Fatalf("conflicting fold: %v", err)
	}
	if after := git(t, g, repo, "rev-parse", "main"); after != mainBefore {
		t.Fatal("target moved on a conflict")
	}
}

// ForgeContract is the forge seam's contract: a PR opens once per head/base pair and reports
// merged only after the merge (R-SESS-096).
func ForgeContract(t *testing.T, f session.Forge, merge func(url string)) {
	t.Helper()
	const head, base, title = "session/x-1", "main", "integrate session/x-1 into main"
	if url, err := f.FindPR("/repo", head, base); err != nil || url != "" {
		t.Fatalf("FindPR before = %q, %v", url, err)
	}
	url, err := f.OpenPR("/repo", head, base, title)
	if err != nil || url == "" {
		t.Fatalf("OpenPR = %q, %v", url, err)
	}
	if again, aerr := f.OpenPR("/repo", head, base, title); aerr != nil || again != url {
		t.Fatalf("second OpenPR = %q, %v; want %q", again, aerr, url)
	}
	wantMerged(t, f, url, false)
	merge(url)
	wantMerged(t, f, url, true)
	if found, ferr := f.FindPR("/repo", head, base); ferr != nil || found != url {
		t.Fatalf("FindPR after = %q, %v", found, ferr)
	}
}

func wantMerged(t *testing.T, f session.Forge, url string, want bool) {
	t.Helper()
	if ok, err := f.Merged("/repo", url); err != nil || ok != want {
		t.Fatalf("Merged = %v, %v; want %v", ok, err, want)
	}
}

// ChannelContract is the channel seam's contract (R-QA-024, R-NTF-005): a first post starts a
// thread (or uses the given one) and a second post replies in it. inspect (nil on a real run)
// returns the nth request's url and body; the reply must address the thread.
func ChannelContract(t *testing.T, ch notify.Channel, thread string, inspect func(n int) (url, body string)) {
	t.Helper()
	first, err := ch.Post(notify.Part{Blocks: []string{"ness channel contract: first"}}, thread)
	if err != nil || first == "" {
		t.Fatalf("first Post = %q, %v", first, err)
	}
	second, err := ch.Post(notify.Part{Blocks: []string{"ness channel contract: reply"}}, first)
	if err != nil || second == "" {
		t.Fatalf("second Post = %q, %v", second, err)
	}
	if inspect == nil {
		return
	}
	url, body := inspect(2)
	addressed := strings.Contains(body, `"thread_ts":"`+first+`"`) ||
		strings.Contains(body, `"reply_to_message_id":`+first) ||
		strings.Contains(body, `"message_thread_id":`+first) ||
		strings.Contains(url, "/issues/"+first+"/")
	if !addressed {
		t.Fatalf("the reply does not address thread %q: url %s body %s", first, url, body)
	}
}
