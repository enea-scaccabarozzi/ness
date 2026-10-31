//go:build e2e

package cli_test

// Real-client runs (R-QA-027, R-QA-042). Every test calls testkit.RequireBudget first: it skips
// without NESS_E2E=1 and fails fast when weekly usage is at or above 80% (D31). Each test logs
// the observation the P3a final review asked for with t.Log; run with -v and put the lines in
// the report. The binary under test is `ness` on PATH, and the tests run inside herdr.

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
	"github.com/enea-scaccabarozzi/ness/internal/atom/herdr"
	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

// Time budget: every test gets testBudget, then its cleanup gets cleanupBudget on a fresh
// context, all well inside the Makefile's -timeout, so a hung step cannot reach the binary's
// panic, which skips every t.Cleanup and leaks running agents (and budget).
const (
	testBudget    = 8 * time.Minute
	cleanupBudget = 2 * time.Minute
)

var deadlines sync.Map // test name -> time.Time

// startClock sets the test's deadline; e2eRepo calls it first.
func startClock(t *testing.T) {
	t.Helper()
	deadlines.Store(t.Name(), time.Now().Add(testBudget))
}

// run executes name in dir and returns its combined output, trimmed. It stops at the test's deadline.
func run(t *testing.T, dir, name string, args ...string) (string, error) {
	t.Helper()
	ctx, cancel := t.Context(), context.CancelFunc(func() {})
	if dl, ok := deadlines.Load(t.Name()); ok {
		ctx, cancel = context.WithDeadline(ctx, dl.(time.Time)) //nolint:forcetypeassert // only startClock stores
	}
	defer cancel()
	out, err := execIn(ctx, dir, nil, name, args...)
	return out, err
}

// execIn is the one place an e2e command is built: dir is the working directory, env extra variables.
func execIn(ctx context.Context, dir string, env []string, name string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	return strings.TrimSpace(string(out)), err
}

func mustRun(t *testing.T, dir, name string, args ...string) string {
	t.Helper()
	out, err := run(t, dir, name, args...)
	if err != nil {
		t.Fatalf("%s %v: %v\n%s", name, args, err, out)
	}
	return out
}

// e2eRepo makes a throwaway project named name under $NESS_E2E_ROOT (default ~/.cache/ness-e2e)
// with `ness init` for the client and one commit, plus a brief that asks for one word and nothing
// else. The path is fixed, not a temp dir, because Claude Code keeps folder trust by path.
func e2eRepo(t *testing.T, name, client string, extraToml ...string) string {
	t.Helper()
	if os.Getenv("HERDR_ENV") != "1" {
		t.Skip("not inside herdr")
	}
	startClock(t)
	root := os.Getenv("NESS_E2E_ROOT")
	if root == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			t.Fatal(err)
		}
		root = filepath.Join(home, ".cache", "ness-e2e")
	}
	dir := filepath.Join(root, name)
	// Worktrees live inside the repo, so removing it removes them. Registered first, this cleanup
	// runs last, after the tab and agent cleanups.
	if err := os.RemoveAll(dir); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	if client == "claude" {
		requireClaudeTrust(t, dir)
	}
	mustRun(t, dir, "git", "init", "-q", "-b", "main")
	mustRun(t, dir, "git", "commit", "-q", "--allow-empty", "-m", "init")
	if client == "codex" { // init refuses --client codex without the experimental flag (R-CFG-044)
		mustRun(t, dir, "ness", "init", "--gate", "true", "--base", "main", "--yes")
		// the spec's default codex models are not offered to every account (R-MOD-030)
		extraToml = append([]string{
			`client = "codex"`, `experimental = ["codex"]`,
			"[tiers.standard]", `codex = "gpt-5.6-terra/medium"`,
			"[tiers.frontier]", `codex = "gpt-5.6-terra/high"`,
			"[tiers.economy]", `codex = "gpt-6-luna/low"`,
		}, extraToml...)
	} else {
		mustRun(t, dir, "ness", "init", "--client", client, "--gate", "true", "--base", "main", "--yes")
	}
	if len(extraToml) > 0 {
		toml := filepath.Join(dir, ".ness", "ness.toml")
		data, err := os.ReadFile(toml)
		if err != nil {
			t.Fatal(err)
		}
		if err = os.WriteFile(toml, append(data, []byte(strings.Join(extraToml, "\n")+"\n")...), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	mustRun(t, dir, "ness", "sync") // renders .codex/config.toml with the [agents] keys for codex
	mustRun(t, dir, "git", "add", "-A")
	mustRun(t, dir, "git", "commit", "-q", "-m", "ness init")
	brief := "Reply with the single word done and end your turn. Do not edit files.\n"
	if err := os.WriteFile(filepath.Join(dir, "brief.md"), []byte(brief), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

// requireClaudeTrust fails the test unless Claude Code already trusts dir. It only reads
// ~/.claude.json; the operator accepts the dialog once per path.
func requireClaudeTrust(t *testing.T, dir string) {
	t.Helper()
	var cfg struct {
		Projects map[string]struct {
			Trusted bool `json:"hasTrustDialogAccepted"`
		} `json:"projects"`
	}
	home, _ := os.UserHomeDir()
	data, err := os.ReadFile(filepath.Join(home, ".claude.json"))
	if err == nil {
		err = json.Unmarshal(data, &cfg)
	}
	if err != nil || !cfg.Projects[dir].Trusted {
		t.Fatalf("Claude Code has not trusted %s: run `claude` there once and accept the trust prompt "+
			"(trust is kept by path, so this is once per machine)", dir)
	}
}

type sess struct {
	ID            string `json:"id"`
	Scope         string `json:"scope"`
	Phase         string `json:"phase"`
	Worktree      string `json:"worktree"`
	Pane          string `json:"pane"`
	ClientSession string `json:"client_session"`
}

func sessions(t *testing.T, dir string) []sess {
	t.Helper()
	var st struct {
		Sessions []sess `json:"sessions"`
	}
	if err := json.Unmarshal([]byte(mustRun(t, dir, "ness", "status", "--json")), &st); err != nil {
		t.Fatal(err)
	}
	return st.Sessions
}

func byScope(t *testing.T, dir, scope string) sess {
	t.Helper()
	for _, s := range sessions(t, dir) {
		if s.Scope == scope {
			return s
		}
	}
	t.Fatalf("no session with scope %s", scope)
	return sess{}
}

// open launches a root session and registers its cleanup straight away, so a failed or parallel
// test never leaves an agent running on the operator's budget. It returns the open output
// (stdout and stderr, which carries ness's launch log lines) and the open error.
func open(t *testing.T, dir, scope string, args ...string) (string, error) {
	t.Helper()
	all := append([]string{"open", "--root", "--scope", scope, "--tier", "economy", "--brief", "brief.md"}, args...)
	out, err := run(t, dir, "ness", all...)
	t.Cleanup(func() { cleanup(t, dir, scope) })
	return out, err
}

// cleanup runs on a fresh context: t.Context is cancelled before cleanups run.
func cleanup(t *testing.T, dir, scope string) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), cleanupBudget)
	defer cancel()
	closeScope(ctx, cleanupExec, t.Logf, dir, scope)
}

// cleanupExec is the command runner of cleanup; the cleanup test swaps in a fake.
var cleanupExec execFn = execIn

// execFn is execIn's shape, so the cleanup logic can be tested with a fake.
type execFn func(ctx context.Context, dir string, env []string, name string, args ...string) (string, error)

// closeScope writes the report that close demands (R-SESS-061, even for --discard), then closes
// every session of the scope, which closes its tab and so stops the agent. It never fails the
// test and never stops early: each step's error is logged and the next session is still closed.
// Every command runs in the test project dir, where ness finds its store.
func closeScope(ctx context.Context, ex execFn, logf func(string, ...any), dir, scope string) {
	js, err := ex(ctx, dir, nil, "ness", "status", "--json")
	if err != nil {
		logf("cleanup: ness status: %v\n%s", err, js)
		return
	}
	var st struct {
		Sessions []sess `json:"sessions"`
	}
	if err = json.Unmarshal([]byte(js), &st); err != nil {
		logf("cleanup: ness status --json: %v", err)
		return
	}
	for _, s := range st.Sessions {
		if s.Scope != scope {
			continue
		}
		_ = filepath.WalkDir(filepath.Join(dir, s.Worktree), func(p string, d fs.DirEntry, werr error) error {
			if werr == nil && !d.IsDir() && d.Name() == "brief.md" && !strings.Contains(p, "/.git/") {
				if werr = os.WriteFile(filepath.Join(filepath.Dir(p), "report.md"), []byte("## Lessons\nNone.\n"), 0o600); werr != nil {
					logf("cleanup: report for %s: %v", s.ID, werr)
				}
			}
			return nil
		})
		if out, cerr := ex(ctx, dir, nil, "ness", "close", s.ID, "--discard", "--note", "e2e"); cerr != nil {
			logf("cleanup: ness close %s: %v\n%s", s.ID, cerr, out)
		}
	}
}

func paneText(t *testing.T, pane string) string {
	t.Helper()
	return mustRun(t, "", "herdr", "pane", "read", pane, "--source", "recent-unwrapped", "--format", "text", "--lines", "200")
}

func TestRealLaunchReachesReadyAndBindsTheTranscript(t *testing.T) {
	// One economy Claude session: ready mark arrives, the transcript is named by the stored
	// client session id (real half of R-SESS-027), and the herdr agent start log line is
	// recorded (is the agent reported ready while Claude works on its prompt argument?).
	// covers: R-SESS-027, R-SESS-111
	testkit.RequireBudget(t)
	dir := e2eRepo(t, "launch", "claude")
	out, err := open(t, dir, "e2e", "--effort", "low")
	if err != nil {
		t.Fatalf("ness open: %v\n%s", err, out)
	}
	t.Logf("ness open output:\n%s", out)
	t.Logf("herdr agent start returned on first detection (no 'is up, continuing' line): %v",
		!strings.Contains(out, "is up, continuing"))
	s := byScope(t, dir, "e2e")
	if s.Phase != "ready" {
		t.Fatalf("phase %q after open, want ready\n%s", s.Phase, out)
	}
	home, _ := os.UserHomeDir()
	transcript := filepath.Join(claude.ProjectDir(home, filepath.Join(dir, s.Worktree)), s.ClientSession+".jsonl")
	if _, err = os.Stat(transcript); err != nil {
		t.Fatalf("no transcript at %s: %v", transcript, err)
	}
	t.Logf("herdr agent list (real output shape): %s", mustRun(t, "", "herdr", "agent", "list"))
	assertCloseStopsTheAgent(t, dir, s)
}

// assertCloseStopsTheAgent is the premise of CheckLanded (R-SESS-065): after `ness close` the
// pane is gone and no client process of the session survives.
func assertCloseStopsTheAgent(t *testing.T, dir string, s sess) {
	t.Helper()
	cleanup(t, dir, s.Scope) // writes the report, then ness close --discard
	// SystemExec, as ness itself runs herdr: it folds herdr's stderr error envelope into the
	// error, which is where the *_not_found code that means ErrGone lives.
	rt := herdr.Runtime{Exec: cli.SystemExec}
	if _, err := rt.State(s.Pane); !errors.Is(err, session.ErrGone) {
		t.Fatalf("pane %s after ness close: err = %v, want ErrGone", s.Pane, err)
	}
	deadline := time.After(10 * time.Second)
	for {
		if _, err := exec.CommandContext(t.Context(), "pgrep", "-f", s.ClientSession).Output(); err != nil {
			return // pgrep exits 1 when nothing matches
		}
		select {
		case <-deadline:
			t.Fatalf("a client process of %s survives ness close", s.ClientSession)
		case <-time.After(200 * time.Millisecond):
		}
	}
}

func TestRealClaudeFlagsAreAccepted(t *testing.T) {
	// --session-id, --model, --effort and --fallback-model on the first launch, then --resume
	// of the exited session (real half of R-SESS-025, R-MOD-044).
	// covers: R-SESS-025, R-MOD-044
	testkit.RequireBudget(t)
	dir := e2eRepo(t, "flags", "claude", "[models]", `fallback_model = "sonnet"`)
	out, err := open(t, dir, "flags", "--effort", "low")
	if err != nil {
		t.Fatalf("first launch (--session-id --model --effort --fallback-model): %v\n%s", err, out)
	}
	s := byScope(t, dir, "flags")
	if s.Phase != "ready" || s.ClientSession == "" {
		t.Fatalf("first launch: phase %q, client session %q\n%s", s.Phase, s.ClientSession, out)
	}
	t.Logf("claude accepted --session-id %s with --model/--effort/--fallback-model", s.ClientSession)
	// Close the tab only (the agent stops), keep the session row, then resume it.
	mustRun(t, dir, "herdr", "pane", "close", s.Pane)
	out, err = run(t, dir, "ness", "open", "--resume", s.ID)
	if err != nil {
		t.Fatalf("--resume %s: %v\n%s", s.ID, err, out)
	}
	t.Logf("claude accepted --resume on session %s:\n%s", s.ClientSession, out)
}

func TestRealParallelLaunches(t *testing.T) {
	// L6 exit: NESS_E2E_PARALLEL launches (default 2; 20 for the real check) all reach ready,
	// and each one is closed afterwards.
	// covers: R-SESS-112
	testkit.RequireBudget(t)
	n, _ := strconv.Atoi(os.Getenv("NESS_E2E_PARALLEL"))
	if n == 0 {
		n = 2
	}
	dir := e2eRepo(t, "parallel", "claude")
	for i := range n {
		scope := "p" + strconv.Itoa(i)
		t.Cleanup(func() { cleanup(t, dir, scope) })
	}
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if out, err := run(t, dir, "ness", "open", "--root", "--scope", "p"+strconv.Itoa(i),
				"--tier", "economy", "--brief", "brief.md", "--effort", "low"); err != nil {
				errs <- fmt.Errorf("launch %d: %w\n%s", i, err, out)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	if out := mustRun(t, dir, "ness", "status"); strings.Contains(out, "stalled") {
		t.Fatalf("stalled launches:\n%s", out)
	}
}

// codexSession launches a Codex root on a synced project and returns its pane text.
func codexSession(t *testing.T, dir, scope string) (sess, string) {
	t.Helper()
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skip("codex not installed")
	}
	out, err := open(t, dir, scope, "--client", "codex")
	if err != nil {
		t.Fatalf("codex launch: %v\n%s", err, out)
	}
	s := byScope(t, dir, scope)
	return s, paneText(t, s.Pane)
}

// fakeCodexHome makes a CODEX_HOME whose config already trusts trusted, with the operator's login
// copied in (auth only), and points the process at it so the real ~/.codex is never read for
// config or written. It returns the config path.
func fakeCodexHome(t *testing.T, trusted string) string {
	t.Helper()
	home := t.TempDir()
	cfg := filepath.Join(home, "config.toml")
	body := "[projects.\"" + trusted + "\"]\ntrust_level = \"trusted\"\n"
	if err := os.WriteFile(cfg, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
	if real, err := os.UserHomeDir(); err == nil {
		if auth, rerr := os.ReadFile(filepath.Join(real, ".codex", "auth.json")); rerr == nil {
			if err = os.WriteFile(filepath.Join(home, "auth.json"), auth, 0o600); err != nil {
				t.Fatal(err)
			}
		}
	}
	t.Setenv("CODEX_HOME", home)
	return cfg
}

func TestRealCodexTrustAndHooks(t *testing.T) {
	// Codex on a synced project: the inline `-c projects={...}` trust shows no dialog for two
	// worktrees, a project already trusted in the CODEX_HOME config keeps its trust after the
	// launches (the table override must not cost it), the SessionStart hook installed through
	// inline TOML fires (the ready mark is the proof), and the synced [agents] keys do not stop
	// Codex from starting. Records the pane text.
	// covers: R-SESS-113, R-HOOK-018, R-SYNC-068, R-MOD-050
	testkit.RequireBudget(t)
	dir := e2eRepo(t, "codex-trust", "codex")
	other := filepath.Join(t.TempDir(), "other-trusted-project")
	cfg := fakeCodexHome(t, other)
	toml, err := os.ReadFile(filepath.Join(dir, ".codex", "config.toml"))
	if err != nil {
		t.Fatalf("sync wrote no .codex/config.toml: %v", err)
	}
	t.Logf(".codex/config.toml as synced:\n%s", toml)
	for _, scope := range []string{"cx-one", "cx-two"} {
		s, text := codexSession(t, dir, scope)
		if low := strings.ToLower(text); strings.Contains(low, "trust this") || strings.Contains(low, "do you trust") {
			t.Errorf("%s: a trust dialog is on screen:\n%s", scope, text)
		}
		if s.Phase != "ready" {
			t.Errorf("%s: phase %q: the SessionStart hook did not fire or Codex did not start", scope, s.Phase)
		}
		t.Logf("%s: phase %s, pane:\n%s", scope, s.Phase, text)
	}
	after, err := os.ReadFile(cfg)
	if err != nil || !strings.Contains(string(after), other) || !strings.Contains(string(after), `"trusted"`) {
		t.Errorf("the pre-trusted project %s lost its trust in the CODEX_HOME config: %v\n%s", other, err, after)
	}
	t.Logf("CODEX_HOME config after the launches:\n%s", after)
	// agents.default_subagent_model / default_subagent_reasoning_effort: their table placement
	// is unverified, so a Codex that rejected the config would show it in the pane text above.
	for _, key := range []string{"default_subagent_model", "default_subagent_reasoning_effort"} {
		if !strings.Contains(string(toml), key) {
			t.Errorf("sync did not write %s", key)
		}
	}
}

// assertShadowExitsTwo is the no-client precondition of the advisory test: with a session row in
// the store (seeded directly, nothing launched) and a shadowed skill, `ness hook session-start`
// must exit 2 with the shadowing message. Without it, a Codex that shows nothing would prove nothing.
func assertShadowExitsTwo(t *testing.T, dir string) {
	t.Helper()
	path, err := store.Locate(dir)
	if err != nil {
		t.Fatal(err)
	}
	st, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = st.AddSession("precheck", "precheck", store.Operator, time.Now()); err != nil {
		t.Fatal(err)
	}
	if err = st.Close(); err != nil {
		t.Fatal(err)
	}
	out, err := execIn(t.Context(), dir, []string{"NESS_SESSION=precheck"}, "ness", "hook", "session-start")
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != 2 || !strings.Contains(out, "shadows") {
		t.Fatalf("precondition: session-start with a shadowed skill must exit 2 naming the shadow, got %v\n%s", err, out)
	}
}

func TestRealCodexShadowedSkillRefusedBeforeLaunch(t *testing.T) {
	// A project skill shadowing a global one makes `ness hook session-start` exit 2, and
	// `ness open` runs the same check first, so Codex is never launched with it. The
	// client-side half of R-HOOK-005 (Codex receiving the blocking response) is not reachable here.
	// covers: R-SESS-005
	testkit.RequireBudget(t)
	dir := e2eRepo(t, "codex-sessionstart", "codex")
	home, _ := os.UserHomeDir()
	global, _ := filepath.Glob(filepath.Join(home, ".claude", "skills", "*"))
	if len(global) == 0 {
		t.Skip("no global skill to shadow")
	}
	shadow := filepath.Join(dir, ".agents", "skills", filepath.Base(global[0]))
	if err := os.MkdirAll(shadow, 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(shadow, "SKILL.md"), []byte("---\nname: x\n---\nx\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	mustRun(t, dir, "git", "add", "-A")
	mustRun(t, dir, "git", "commit", "-q", "-m", "shadow")
	assertShadowExitsTwo(t, dir)
	// ness open runs the same shadow check before the client starts, so the launch is refused
	// up front (exit 4 naming the shadow) and Codex never reaches its SessionStart hook: the
	// advisory exit-2 path is only reachable for a shadow that appears mid-session.
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skip("codex not installed")
	}
	out, err := open(t, dir, "cx-exit2", "--client", "codex")
	var ee *exec.ExitError
	if !errors.As(err, &ee) || ee.ExitCode() != cli.ExitRefused || !strings.Contains(out, "shadows") {
		t.Fatalf("open with a shadowed skill: %v\n%s; want exit 4 naming the shadow", err, out)
	}
	t.Logf("open refused before launch, as designed:\n%s", out)
}
