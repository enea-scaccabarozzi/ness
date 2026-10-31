> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# P3b Integrate, Close and Doctor Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `ness integrate` folds a session's branch into its manager's through the gate in a throwaway worktree with the `merge`, `squash` or `pr` integrator and records it; `ness close` retires a session behind the close checks and removes its worktree, tab and branch; `ness doctor` reports every environment, binary, worker, sync and store problem read-only with a fix command; sync writes the Codex keys; the real-client e2e runs live behind the `e2e` tag and the weekly-usage stop rule. With P3a this completes SESS, MOD and DOC.

**Architecture:** `internal/session` gains the `Integrator` and `Forge` seams and the close and integrate logic over the `Exec` seam of P3a; `internal/atom/{merge,squash,pr}` implement the integrators with real git (the throwaway worktree lives under `<git-common-dir>/ness/tmp-<id>`), `pr` through `gh`. `internal/cli/doctor.go` runs a list of `check` functions and prints one line each. `internal/sync` learns line-level TOML keys for `.codex/config.toml`. `internal/testkit` gains the fake forge (`gh` through the exec table), a `systemctl` table entry, the e2e budget guard and the integrator contract suite.

**Tech Stack:** Go 1.25+, `modernc.org/sqlite`, cobra, testscript, BurntSushi/toml (validation of the edited TOML), git 2.47, `gh` for pull requests, `systemctl --user` for the worker check. The P1/P2 gate stays as is.

**Spec:** `docs/superpowers/specs/2026-10-08-ness-v1-scope.md` (split), `docs/design/ness-design.md` §4, §5 (close, doctor, integrate), §6, §12, §16 (#3, #4, #12), and the requirements in `docs/spec/sessions.md` (close, integrate, the pr integrator), `docs/spec/doctor.md` (DOC), `docs/spec/models.md` (R-MOD-050), `docs/spec/sync.md` (R-SYNC-068, R-SYNC-086), `docs/spec/quality.md` (R-QA-024, R-QA-027, R-QA-042), and the MSG close re-covers (R-MSG-022, R-MSG-040, R-MSG-041, R-MSG-043).

## Global Constraints

- Everything of P3a's Global Constraints holds; in particular every external program runs through `Deps.Exec`, git discovery variables stripped.
- Exit codes: 0 ok, 1 a check found problems (`doctor`), 2 usage, 3 sync conflict, 4 refused (R-CFG-035). `doctor` exits 1 on any `fail`, 0 with only `ok`/`warn` (R-DOC-021); outside a project it prints the version line, then refuses with exit 4 (R-DOC-023).
- `doctor` is read-only: no file, store row, git ref, worktree or tab changes; fix commands are printed, never run (R-DOC-001, R-DOC-002).
- `gate` must be configured; `integrate` refuses otherwise (R-SESS-105 via R-CFG-040). The gate runs with `sh -c <gate>` in the throwaway worktree, never in the session's (R-SESS-091).
- The throwaway worktree is `<git-common-dir>/ness/tmp-<session id>` (or `tmp-op-<n>` for the operator), removed with `--force` on success and failure (R-SESS-093).
- The target is updated only after the gate passes; on gate failure or conflict nothing moves, nothing is pushed, nothing is recorded (R-SESS-092, R-SESS-097). Pushing happens only with `integrate.push = true` (R-SESS-098).
- `integrate.mode` defaults to `merge` (R-CFG-042). The squash commit message is `integrate <source> into <target>` with the trailer `Ness-Integrates: <source sha>`; the merge is `--no-ff`.
- `close` is allowed to the manager (child) or the operator (root) only (R-SESS-060). Report path: `<checkout>/<record>/report.md` in the session's worktree (the primary checkout for an adopted root). The never-ran exemption is "no ready mark ever" (R-SESS-066).
- Close deletes the local branch only when an integration is recorded and neither `--discard` nor `--keep-work` is given (R-SESS-072). The store row stays, marked closed (R-SESS-065).
- A worker heartbeat older than 60 s is `fail` in doctor (R-DOC-022); doctor's status words are exactly `ok`, `warn`, `fail` (R-DOC-020).
- Codex keys written by sync when `client = "codex"`: `.codex/config.toml` `model_auto_compact_token_limit = 235000` (R-SYNC-086), `agents.model = "<economy codex model>"`, `agents.model_reasoning_effort = "<economy codex effort>"` (R-MOD-050, R-SYNC-068). Nothing per named agent.
- e2e tests carry `//go:build e2e`, never run in default CI, and abort before any launch when `~/.claude/usage-latest.json` reports `rate_limits.seven_day.used_percentage >= 80` (R-QA-027, R-QA-042, D31).
- Every test names the requirement ids it covers. Conventional commits with the session's trailers.

## Review Focus

1. **A gate command that writes files into the throwaway worktree (build artefacts, a formatter).** The fold result must be what is recorded and the dirty throwaway must still be removed. Pinned in Task 3 (`integrate_gate.txtar`).
2. **`ness close` on a session whose worktree directory was deleted by hand.** `git worktree remove` fails; close must prune and still succeed, removing the tab and marking the row closed. Pinned in Task 4 (`close_gone.txtar`).
3. **`ness integrate` into a target that has moved on since the branch point** (the manager committed meanwhile). The merge integrator must fold with a real merge commit and the squash integrator must still answer "integrated?" afterwards. Pinned in Task 1 (`TestIntegratorsWithDivergedTarget`).
4. **`ness doctor` on a repository whose `.ness/ness.toml` is malformed or missing a required key.** It must print the version line and refuse naming the file (R-CFG-003), not crash or report every check as fail. Pinned in Task 5 (`doctor_outside.txtar`).
5. **Closing a ready session whose tab is already gone (the operator closed it).** The tab removal must tolerate `ErrGone` and the normal close checks (report, integration) still apply: only a never-ran session is exempt. Pinned in Task 4 (`close_never_ran.txtar`).

## Deferred to later plans

The coverage test (Task 9) lists these so the gate stays green while the gap stays visible:

- P4 (worker): R-SESS-009 (model unavailable mid-run), the worker half of R-MOD-012, the "worker version differs" half of R-DOC-010 is covered here with a fixture (no deferral).
- P5 (features): R-SESS-070 (probe ownership on close), R-SESS-081, R-SESS-083 (probes in status), R-DOC-015 (probes without an owner), R-SESS-100's probe half (the channel/probe fakes land in P5; the test here asserts no `post`/probe call through the exec table).
- P6 (text): R-MOD-002, R-MOD-021.
- Review entry in `docs/spec/reviews.md`: none new (R-SESS-014 was P3a's).
- The real-client halves of R-SESS-027, R-SESS-112, R-SESS-113 and the real half of R-SESS-071/R-SESS-096 (a real GitHub PR) are e2e tests here (Task 8); they carry their markers and run on demand under the budget rule. The end-of-plan report records the run or states it waits for the weekly reset (D31).

## Rulings

- "integrated?" for `squash` is answered by the trailer `Ness-Integrates: <source sha>` found by `git log --grep` on the target; for `merge` by `git merge-base --is-ancestor`; for `pr` by the forge's merged state.
- The `pr` integrator pushes the source branch to `origin`, opens the PR with `gh pr create` (or finds the open one with `gh pr list --head`), records the URL, and never touches the local target; the gate still runs on a local merge preview in the throwaway worktree.
- When the target branch is checked out in some worktree (the manager's, or the primary checkout for a root), integrate fast-forwards it there with `git merge --ff-only`; otherwise it moves the ref with `git update-ref`. A dirty checkout of the target refuses before the fold.
- `close` without `--discard` on a `pr` session asks the forge whether the recorded PR is merged and refuses while it is not (R-SESS-071); `--discard` skips the question.
- Doctor's "which binary the hooks resolve" is `exec.LookPath("ness")` from doctor's own PATH (the hooks run `command -v ness`); the worker unit is parsed from `~/.config/systemd/user/ness-<project>.service` `ExecStart=`. A missing unit is `warn` (not installed), not `fail`.
- Doctor runs the shadowing, version and tier checks itself through `session.Env` pieces rather than `Env.Check`, so every check reports instead of stopping at the first (R-DOC-017).
- The e2e budget guard reads the statusline snapshot `~/.claude/usage-latest.json`; a missing file aborts too ("no usage data: run one Claude Code session first"), since the rule cannot be checked.

## File map

```
internal/session/integrate.go               Integrator, Forge seams; Integration{Source, Target, Ref}; Integrate, ErrConflict, ErrDirty
internal/session/close.go                   CloseChecks, Close
internal/session/git.go                     git helpers over Exec: rev-parse, worktrees, branch delete, status
internal/session/*_test.go
internal/atom/merge/merge.go                merge integrator (--no-ff)
internal/atom/squash/squash.go              squash integrator (trailer)
internal/atom/pr/pr.go                      pr integrator over Forge; pr.GH (gh CLI forge)
internal/atom/*/..._test.go                 contract runs over real git (and the gh table)
internal/testkit/contracts.go               IntegratorContract, ForgeContract
internal/testkit/budget.go                  UsageGuard, RequireBudget (e2e stop rule)
internal/testkit/exec.go                    default `systemctl --user is-system-running` output
internal/cli/integrate.go                   ness integrate
internal/cli/close.go                       ness close
internal/cli/doctor.go                      ness doctor: checks, text and JSON output
internal/cli/doctor_checks.go               the check functions
internal/cli/e2e_test.go                    //go:build e2e: real launch, 20 parallel, codex
internal/cli/testdata/script/integrate.txtar, integrate_gate.txtar, integrate_rights.txtar, integrate_pr.txtar,
  close.txtar, close_obligations.txtar, close_never_ran.txtar, close_gone.txtar, close_pr.txtar,
  doctor.txtar, doctor_store.txtar, doctor_outside.txtar, sync_codex.txtar
internal/sync/tomlkeys.go                   tomlGet, tomlSet, tomlDelete (line-level), FuzzTOMLKeys
internal/sync/engine.go                     opsFor: .toml → tomlkeys
internal/sync/render.go                     Codex entries when client = codex
internal/testkit/coverage_repo_test.go      SESS and DOC join doneAreas
README.md                                   status line
```

---

### Task 1: The integrator seam, `merge` and `squash`

**Files:**
- Create: `internal/session/integrate.go` (seams and `Integrate`), `internal/session/git.go`, `internal/atom/merge/merge.go`, `internal/atom/merge/merge_test.go`, `internal/atom/squash/squash.go`, `internal/atom/squash/squash_test.go`, `internal/session/integrate_test.go`
- Modify: `internal/testkit/contracts.go` (`IntegratorContract`)

**Interfaces:**
- Produces:
  - `session.Git struct{ Exec Exec }` with `Out(dir string, args ...string) (string, error)` (trimmed stdout), `Rev(dir, ref string) (string, error)`, `Worktrees(repo string) ([]Worktree, error)` (`Worktree{Path, Branch string}`), `Dirty(dir string) (bool, error)`, `CommonDir(dir string) (string, error)`.
  - `session.Integrator` interface: `Fold(g Git, dir, source, target string) (ref string, err error)` (dir is the throwaway worktree checked out at target; returns the new target sha, or the PR url for `pr`), `Integrated(g Git, repo, source, target string) (bool, error)`.
  - `session.ErrConflict` (wrapped, message names the conflicting paths), `session.Integration{Source, Target, Ref string}`.
  - `merge.Integrator{}`, `squash.Integrator{}`; `squash.Trailer = "Ness-Integrates"`.
  - `testkit.IntegratorContract(t *testing.T, it session.Integrator, g session.Git)`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/testkit/contracts.go`:
```go
// NewRepo makes a throwaway repository with one commit on main and returns its path.
func NewRepo(t *testing.T, g session.Git) string {
	t.Helper()
	dir := t.TempDir()
	for _, args := range [][]string{{"init", "-q", "-b", "main"}, {"config", "user.email", "t@x"}, {"config", "user.name", "t"},
		{"config", "commit.gpgsign", "false"}, {"commit", "-q", "--allow-empty", "-m", "init"}} {
		if _, err := g.Out(dir, args...); err != nil {
			t.Fatal(err)
		}
	}
	return dir
}

// Commit writes path with content on the current branch of dir and commits it.
func Commit(t *testing.T, g session.Git, dir, path, content, msg string) string {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, path), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", path}, {"commit", "-q", "-m", msg}} {
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

// IntegratorContract is the integrator seam's contract (R-SESS-102): integrated? is false before
// the fold and true after; the fold lands the source's changes on the target; a conflicting
// change leaves the target untouched and names the path (R-SESS-097).
func IntegratorContract(t *testing.T, it session.Integrator, g session.Git) {
	t.Helper()
	repo := NewRepo(t, g)
	if _, err := g.Out(repo, "checkout", "-q", "-b", "session/x-1"); err != nil {
		t.Fatal(err)
	}
	Commit(t, g, repo, "a.txt", "from the session\n", "session work")
	if _, err := g.Out(repo, "checkout", "-q", "main"); err != nil {
		t.Fatal(err)
	}
	if ok, err := it.Integrated(g, repo, "session/x-1", "main"); err != nil || ok {
		t.Fatalf("Integrated before = %v, %v", ok, err)
	}
	tmp := filepath.Join(t.TempDir(), "tmp")
	if _, err := g.Out(repo, "worktree", "add", "-q", "--detach", tmp, "main"); err != nil {
		t.Fatal(err)
	}
	before, _ := g.Rev(repo, "main")
	ref, err := it.Fold(g, tmp, "session/x-1", "main")
	if err != nil || ref == "" {
		t.Fatalf("Fold = %q, %v", ref, err)
	}
	if _, err := g.Out(repo, "update-ref", "refs/heads/main", ref, before); err != nil {
		t.Fatal(err)
	}
	if ok, err := it.Integrated(g, repo, "session/x-1", "main"); err != nil || !ok {
		t.Fatalf("Integrated after = %v, %v", ok, err)
	}
	if out, _ := g.Out(repo, "show", "main:a.txt"); out != "from the session" {
		t.Fatalf("main:a.txt = %q", out)
	}
	// A conflicting branch.
	if _, err := g.Out(repo, "checkout", "-q", "-b", "session/y-2", before); err != nil {
		t.Fatal(err)
	}
	Commit(t, g, repo, "a.txt", "conflicting\n", "conflict")
	if _, err := g.Out(repo, "checkout", "-q", "main"); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Out(repo, "worktree", "remove", "--force", tmp); err != nil {
		t.Fatal(err)
	}
	if _, err := g.Out(repo, "worktree", "add", "-q", "--detach", tmp, "main"); err != nil {
		t.Fatal(err)
	}
	mainBefore, _ := g.Rev(repo, "main")
	_, err = it.Fold(g, tmp, "session/y-2", "main")
	if !errorsIs(err, session.ErrConflict) || !strings.Contains(err.Error(), "a.txt") {
		t.Fatalf("conflicting fold: %v", err)
	}
	if after, _ := g.Rev(repo, "main"); after != mainBefore {
		t.Fatal("target moved on a conflict")
	}
}
```

`internal/atom/merge/merge_test.go`:
```go
package merge_test

import (
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/merge"
	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestMergeContract(t *testing.T) {
	// covers: R-SESS-094, R-SESS-097, R-SESS-102, R-QA-024
	g := session.Git{Exec: cli.SystemExec}
	testkit.IntegratorContract(t, merge.Integrator{}, g)
}

func TestMergePreservesSourceCommits(t *testing.T) {
	// Every source commit is an ancestor of the target after the fold, through a merge commit.
	// covers: R-SESS-094
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	must := func(args ...string) string {
		t.Helper()
		out, err := g.Out(repo, args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	must("checkout", "-q", "-b", "session/s-1")
	c1 := testkit.Commit(t, g, repo, "a.txt", "1\n", "one")
	c2 := testkit.Commit(t, g, repo, "b.txt", "2\n", "two")
	must("checkout", "-q", "main")
	testkit.Commit(t, g, repo, "c.txt", "3\n", "main moved") // Review Focus 3
	tmp := t.TempDir() + "/tmp"
	must("worktree", "add", "-q", "--detach", tmp, "main")
	ref, err := merge.Integrator{}.Fold(g, tmp, "session/s-1", "main")
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []string{c1, c2} {
		if _, err := g.Out(repo, "merge-base", "--is-ancestor", c, ref); err != nil {
			t.Fatalf("%s is not an ancestor of %s", c, ref)
		}
	}
	if parents := must("rev-list", "--parents", "-n", "1", ref); len(strings.Fields(parents)) != 3 {
		t.Fatalf("not a merge commit: %q", parents)
	}
}
```

`internal/atom/squash/squash_test.go`:
```go
package squash_test

import (
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/squash"
	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestSquashContract(t *testing.T) {
	// covers: R-SESS-095, R-SESS-097, R-SESS-102, R-QA-024
	g := session.Git{Exec: cli.SystemExec}
	testkit.IntegratorContract(t, squash.Integrator{}, g)
}

func TestSquashIsOneCommitWithTheTrailer(t *testing.T) {
	// The target advances by exactly one commit carrying the combined diff and the trailer that
	// answers integrated? later, also when the target moved on (Review Focus 3).
	// covers: R-SESS-095, R-SESS-102
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	must := func(args ...string) string {
		t.Helper()
		out, err := g.Out(repo, args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	must("checkout", "-q", "-b", "session/s-1")
	testkit.Commit(t, g, repo, "a.txt", "1\n", "one")
	src := testkit.Commit(t, g, repo, "b.txt", "2\n", "two")
	must("checkout", "-q", "main")
	testkit.Commit(t, g, repo, "c.txt", "3\n", "main moved")
	before := must("rev-parse", "main")
	tmp := t.TempDir() + "/tmp"
	must("worktree", "add", "-q", "--detach", tmp, "main")
	ref, err := squash.Integrator{}.Fold(g, tmp, "session/s-1", "main")
	if err != nil {
		t.Fatal(err)
	}
	if n := must("rev-list", "--count", before+".."+ref); n != "1" {
		t.Fatalf("target advanced by %s commits", n)
	}
	if body := must("log", "-1", "--format=%B", ref); !strings.Contains(body, squash.Trailer+": "+src) ||
		!strings.HasPrefix(body, "integrate session/s-1 into main") {
		t.Fatalf("message %q", body)
	}
	if files := must("diff", "--name-only", before, ref); files != "a.txt\nb.txt" {
		t.Fatalf("diff %q", files)
	}
	must("update-ref", "refs/heads/main", ref, before)
	if ok, err := (squash.Integrator{}).Integrated(g, repo, "session/s-1", "main"); err != nil || !ok {
		t.Fatalf("Integrated = %v, %v", ok, err)
	}
}
```

`internal/session/integrate_test.go` (the orchestration over a fake gate, real git, real integrators):
```go
package session_test

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/merge"
	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestIntegrateRunsTheGateInAThrowawayAndMovesTheTarget(t *testing.T) {
	// covers: R-SESS-091, R-SESS-092, R-SESS-093, R-SESS-099
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	if _, err := g.Out(repo, "checkout", "-q", "-b", "session/s-1"); err != nil {
		t.Fatal(err)
	}
	testkit.Commit(t, g, repo, "a.txt", "1\n", "one")
	if _, err := g.Out(repo, "checkout", "-q", "main"); err != nil {
		t.Fatal(err)
	}
	common, _ := g.CommonDir(repo)
	before, _ := g.Rev(repo, "main")
	cwdFile := filepath.Join(t.TempDir(), "cwd")
	in := session.Integration{Source: "session/s-1", Target: "main"}
	res, err := session.Integrate(g, merge.Integrator{}, session.IntegrateSpec{Repo: repo, Common: common, TmpName: "tmp-s-1",
		Gate: "pwd > " + cwdFile, Integration: in, Log: os.Stderr})
	if err != nil {
		t.Fatal(err)
	}
	cwd, _ := os.ReadFile(cwdFile)
	if !strings.HasPrefix(strings.TrimSpace(string(cwd)), filepath.Join(common, "ness", "tmp-s-1")) {
		t.Fatalf("gate ran in %q", cwd)
	}
	if _, err := os.Stat(filepath.Join(common, "ness", "tmp-s-1")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("throwaway worktree not removed")
	}
	after, _ := g.Rev(repo, "main")
	if after == before || res.Ref != after {
		t.Fatalf("main %s → %s, recorded %s", before, after, res.Ref)
	}
	// A failing gate: nothing moves, the throwaway is gone.
	if _, err := g.Out(repo, "checkout", "-q", "-b", "session/s-2"); err != nil {
		t.Fatal(err)
	}
	testkit.Commit(t, g, repo, "b.txt", "2\n", "two")
	if _, err := g.Out(repo, "checkout", "-q", "main"); err != nil {
		t.Fatal(err)
	}
	_, err = session.Integrate(g, merge.Integrator{}, session.IntegrateSpec{Repo: repo, Common: common, TmpName: "tmp-s-2",
		Gate: "false", Integration: session.Integration{Source: "session/s-2", Target: "main"}})
	if !errors.Is(err, session.ErrGate) {
		t.Fatalf("failing gate: %v", err)
	}
	if now, _ := g.Rev(repo, "main"); now != after {
		t.Fatal("target moved although the gate failed")
	}
	if _, err := os.Stat(filepath.Join(common, "ness", "tmp-s-2")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("throwaway worktree not removed after a failed gate")
	}
}

var _ = time.Second
```
(Remove the `time` line if unused.)

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/atom/merge/ ./internal/atom/squash/ ./internal/session/ ./internal/testkit/`
Expected: FAIL — packages and symbols missing.

- [ ] **Step 3: Git helpers**

`internal/session/git.go`:
```go
package session

import (
	"fmt"
	"strings"
)

// Git runs git through the Exec seam, always with -C dir.
type Git struct{ Exec Exec }

// Out runs git in dir and returns trimmed stdout.
func (g Git) Out(dir string, args ...string) (string, error) {
	out, err := g.Exec(dir, "git", args...)
	return strings.TrimSpace(string(out)), err
}

// Rev resolves ref to a sha.
func (g Git) Rev(dir, ref string) (string, error) { return g.Out(dir, "rev-parse", "--verify", ref+"^{commit}") }

// CommonDir is the absolute git common dir of dir.
func (g Git) CommonDir(dir string) (string, error) {
	return g.Out(dir, "rev-parse", "--path-format=absolute", "--git-common-dir")
}

// Dirty reports uncommitted changes in dir, untracked files included (R-SESS-103).
func (g Git) Dirty(dir string) (bool, error) {
	out, err := g.Out(dir, "status", "--porcelain")
	return out != "", err
}

// Worktree is one entry of git worktree list.
type Worktree struct{ Path, Branch string }

// Worktrees lists the checkouts of the repository containing dir.
func (g Git) Worktrees(dir string) ([]Worktree, error) {
	out, err := g.Out(dir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	var list []Worktree
	var cur Worktree
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			cur = Worktree{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "branch refs/heads/"):
			cur.Branch = strings.TrimPrefix(line, "branch refs/heads/")
		case line == "":
			if cur.Path != "" {
				list = append(list, cur)
			}
			cur = Worktree{}
		}
	}
	if cur.Path != "" {
		list = append(list, cur)
	}
	return list, nil
}

// CheckedOut returns the worktree where branch is checked out, or "".
func (g Git) CheckedOut(dir, branch string) (string, error) {
	wts, err := g.Worktrees(dir)
	if err != nil {
		return "", err
	}
	for _, w := range wts {
		if w.Branch == branch {
			return w.Path, nil
		}
	}
	return "", nil
}

// Conflicts lists the unmerged paths in dir.
func (g Git) Conflicts(dir string) (string, error) {
	out, err := g.Out(dir, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return "", err
	}
	return strings.ReplaceAll(out, "\n", ", "), nil
}

// ConflictError wraps ErrConflict with the paths.
func (g Git) ConflictError(dir string) error {
	paths, _ := g.Conflicts(dir)
	return fmt.Errorf("%w: %s", ErrConflict, paths)
}
```

- [ ] **Step 4: The seam and the orchestration**

`internal/session/integrate.go`:
```go
package session

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

var (
	// ErrConflict is returned by a fold that conflicts; the message names the paths (R-SESS-097).
	ErrConflict = errors.New("the fold conflicts")
	// ErrGate is returned when the gate command fails (R-SESS-092).
	ErrGate = errors.New("the gate failed")
)

// Integration names a fold: source and target refs and, once folded, the recorded ref (the new
// target sha, or the PR url for the pr integrator).
type Integration struct{ Source, Target, Ref string }

// Integrator is the integrator seam (design §2): merge, squash, pr. Fold runs in dir, a
// throwaway worktree checked out at target, and returns the ref to record; it must leave the
// real target untouched. Integrated answers "integrated?" for a pair (R-SESS-102).
type Integrator interface {
	Fold(g Git, dir, source, target string) (ref string, err error)
	Integrated(g Git, repo, source, target string) (bool, error)
}

// Forge is what the pr integrator needs from GitHub: find the open pull request of a pair ("" when
// none), open one, and tell whether one is merged (R-SESS-096).
type Forge interface {
	FindPR(repo, source, target string) (url string, err error)
	OpenPR(repo, source, target, title string) (url string, err error)
	Merged(repo, url string) (bool, error)
}

// IntegrateSpec is one ness integrate.
type IntegrateSpec struct {
	Repo, Common, TmpName string // the checkout to run git in; the common dir; the throwaway name
	Gate                  string
	Integration           Integration
	Push                  bool
	Log                   io.Writer
}

// Result is what integrate records.
type Result struct {
	Ref      string // new target sha, or PR url
	MovedRef bool   // whether the local target moved (false for pr)
}

// Integrate folds source into target: throwaway worktree at target, Fold, gate, then the target
// moves (fast-forward where it is checked out, update-ref otherwise), then push if asked
// (R-SESS-091..098, R-SESS-104). The throwaway is removed on every path (R-SESS-093).
func Integrate(g Git, it Integrator, s IntegrateSpec) (res Result, err error) {
	target := s.Integration.Target
	checkout, err := g.CheckedOut(s.Repo, target)
	if err != nil {
		return res, err
	}
	if checkout != "" {
		dirty, derr := g.Dirty(checkout)
		if derr != nil {
			return res, derr
		}
		if dirty {
			return res, fmt.Errorf("%s is checked out in %s with uncommitted changes; commit or stash them first", target, checkout)
		}
	}
	before, err := g.Rev(s.Repo, target)
	if err != nil {
		return res, err
	}
	tmp := filepath.Join(s.Common, "ness", s.TmpName)
	_, _ = g.Out(s.Repo, "worktree", "remove", "--force", tmp) // a leftover from a killed run
	if _, err = g.Out(s.Repo, "worktree", "add", "-q", "--detach", tmp, target); err != nil {
		return res, err
	}
	defer func() {
		if _, rerr := g.Out(s.Repo, "worktree", "remove", "--force", tmp); rerr != nil {
			err = errors.Join(err, rerr)
		}
		_ = os.RemoveAll(tmp)
	}()
	ref, err := it.Fold(g, tmp, s.Integration.Source, target)
	if err != nil {
		return res, err
	}
	if _, err = g.Exec(tmp, "sh", "-c", s.Gate); err != nil {
		return res, fmt.Errorf("%w: %v", ErrGate, err)
	}
	folded, err := g.Rev(tmp, "HEAD")
	if err != nil {
		return res, err
	}
	res.Ref = ref
	if _, perr := g.Rev(s.Repo, ref); perr != nil {
		return res, nil // not a commit (a PR url): the target stays
	}
	switch {
	case checkout != "":
		if _, err = g.Out(checkout, "merge", "--ff-only", folded); err != nil {
			return res, err
		}
	default:
		if _, err = g.Out(s.Repo, "update-ref", "refs/heads/"+target, folded, before); err != nil {
			return res, err
		}
	}
	res.MovedRef = true
	if s.Push {
		if _, err = g.Out(s.Repo, "push", "origin", target); err != nil {
			return res, err
		}
	}
	return res, nil
}
```
Note: the gate runs on the throwaway's HEAD after the fold; a gate that leaves files behind does not change HEAD, so `folded` is the fold result (Review Focus 1); `worktree remove --force` drops those files.

- [ ] **Step 5: The two integrators**

`internal/atom/merge/merge.go`:
```go
// Package merge is the integrator that folds a branch with a merge commit, preserving the
// source commits (R-SESS-094).
package merge

import (
	"errors"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// Integrator implements session.Integrator.
type Integrator struct{}

// Fold merges source into the throwaway's HEAD (checked out at target) with --no-ff.
func (Integrator) Fold(g session.Git, dir, source, target string) (string, error) {
	if _, err := g.Out(dir, "merge", "--no-ff", "--no-edit", "-m", "integrate "+source+" into "+target, source); err != nil {
		if strings.Contains(err.Error(), "CONFLICT") || strings.Contains(err.Error(), "conflict") {
			cerr := g.ConflictError(dir)
			_, _ = g.Out(dir, "merge", "--abort")
			return "", cerr
		}
		return "", err
	}
	return g.Rev(dir, "HEAD")
}

// Integrated is true when source is an ancestor of target.
func (Integrator) Integrated(g session.Git, repo, source, target string) (bool, error) {
	_, err := g.Out(repo, "merge-base", "--is-ancestor", source, target)
	if err == nil {
		return true, nil
	}
	if strings.Contains(err.Error(), "exit status 1") {
		return false, nil
	}
	return false, errors.Unwrap(err)
}
```
`internal/atom/squash/squash.go`:
```go
// Package squash is the integrator that folds a branch as one commit on the target
// (R-SESS-095), answering integrated? by a trailer naming the source sha.
package squash

import (
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// Trailer names the folded source commit in the squash commit's message.
const Trailer = "Ness-Integrates"

// Integrator implements session.Integrator.
type Integrator struct{}

// Fold squashes source onto the throwaway's HEAD as one commit.
func (Integrator) Fold(g session.Git, dir, source, target string) (string, error) {
	src, err := g.Rev(dir, source)
	if err != nil {
		return "", err
	}
	if _, err := g.Out(dir, "merge", "--squash", source); err != nil {
		if strings.Contains(err.Error(), "CONFLICT") || strings.Contains(err.Error(), "conflict") {
			cerr := g.ConflictError(dir)
			_, _ = g.Out(dir, "reset", "--hard", "HEAD")
			return "", cerr
		}
		return "", err
	}
	msg := "integrate " + source + " into " + target + "\n\n" + Trailer + ": " + src
	if _, err := g.Out(dir, "commit", "-q", "--no-verify", "-m", msg); err != nil {
		return "", err
	}
	return g.Rev(dir, "HEAD")
}

// Integrated looks for the trailer on the target.
func (Integrator) Integrated(g session.Git, repo, source, target string) (bool, error) {
	src, err := g.Rev(repo, source)
	if err != nil {
		return false, err
	}
	out, err := g.Out(repo, "log", "--format=%H", "--grep", Trailer+": "+src, "-n", "1", target)
	return out != "", err
}
```

- [ ] **Step 6: Run and commit**

Run: `go test ./internal/atom/merge/ ./internal/atom/squash/ ./internal/session/ ./internal/testkit/ && make lint`
Expected: PASS. `merge` needs `git config user.*` in the throwaway: the contract's `NewRepo` sets it; production inherits the user's config.

```bash
git add internal/session/git.go internal/session/integrate.go internal/session/integrate_test.go internal/atom/merge internal/atom/squash internal/testkit/contracts.go
git commit -m "feat(session): integrator seam with merge and squash atoms and the gated fold"
```

---

### Task 2: The `pr` integrator and the forge

**Files:**
- Create: `internal/atom/pr/pr.go`, `internal/atom/pr/pr_test.go`
- Modify: `internal/testkit/contracts.go` (`ForgeContract`, `FakeForge`)

**Interfaces:**
- Consumes: `session.Forge`, `session.Integrator`, `merge.Integrator` (the local preview fold).
- Produces: `pr.Integrator{Forge session.Forge; Remote string}` (Remote defaults to `origin`), `pr.GH{Exec pr.Exec}` implementing `session.Forge` over `gh pr list --head … --base … --state all --json url`, `gh pr create`, `gh pr view URL --json state`; `testkit.FakeForge{Dir string}` (state in `<fakes>/forge.json`: PRs with url, head, base, merged) and the script subcommands `fake forge prs N`, `fake forge merge URL`, `fake forge last VAR`.

- [ ] **Step 1: Write the failing tests**

Append to `internal/testkit/contracts.go`:
```go
// ForgeContract is the forge seam's contract: a PR opens once per head/base pair and reports
// merged only after the merge (R-SESS-096).
func ForgeContract(t *testing.T, f session.Forge, merge func(url string)) {
	t.Helper()
	if url, err := f.FindPR("/repo", "session/x-1", "main"); err != nil || url != "" {
		t.Fatalf("FindPR before = %q, %v", url, err)
	}
	url, err := f.OpenPR("/repo", "session/x-1", "main", "integrate session/x-1 into main")
	if err != nil || url == "" {
		t.Fatalf("OpenPR = %q, %v", url, err)
	}
	again, err := f.OpenPR("/repo", "session/x-1", "main", "integrate session/x-1 into main")
	if err != nil || again != url {
		t.Fatalf("second OpenPR = %q, %v; want %q", again, err, url)
	}
	if ok, err := f.Merged("/repo", url); err != nil || ok {
		t.Fatalf("Merged before = %v, %v", ok, err)
	}
	merge(url)
	if ok, err := f.Merged("/repo", url); err != nil || !ok {
		t.Fatalf("Merged after = %v, %v", ok, err)
	}
	if found, err := f.FindPR("/repo", "session/x-1", "main"); err != nil || found != url {
		t.Fatalf("FindPR after = %q, %v", found, err)
	}
}
```
And the fake forge, in a new `internal/testkit/forge.go`:
```go
package testkit

import (
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/rogpeppe/go-internal/testscript"
)

type fakePR struct {
	URL, Head, Base string
	Merged          bool
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

// FindPR returns the pair's PR url in any state, or "".
func (f FakeForge) FindPR(_, head, base string) (string, error) {
	st, err := f.load()
	if err != nil {
		return "", err
	}
	for _, p := range st.PRs {
		if p.Head == head && p.Base == base {
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
	p := fakePR{URL: fmt.Sprintf("https://example.invalid/pr/%d", len(st.PRs)+1), Head: head, Base: base}
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

// forgeCmd: fake forge prs N | merge URL | last VAR
func forgeCmd(ts *testscript.TestScript, neg bool, dir string, args []string) {
	f := FakeForge{Dir: dir}
	st, err := f.load()
	ts.Check(err)
	switch args[0] {
	case "prs":
		n, _ := strconv.Atoi(args[1])
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
```
Add `loadJSON(path string, v any) error` next to `saveJSON` in `exec.go`, `WriteFakeDefaults` writes an empty `forge.json`, and `fakeCmd` routes `fake forge …` to `forgeCmd`. The CLI uses `testkit.FakeForge` when `NESS_TEST_FAKES` is set: add `Forge session.Forge` to `cli.Deps` (nil → `pr.GH{Exec: d.exec()}`); `script_test.go` sets `d.Forge = testkit.FakeForge{Dir: dir}`.

`internal/atom/pr/pr_test.go`:
```go
package pr_test

import (
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/pr"
	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestFakeForgeContract(t *testing.T) {
	// covers: R-SESS-096, R-QA-024
	f := testkit.FakeForge{Dir: t.TempDir()}
	if err := testkit.WriteFakeDefaults(f.Dir); err != nil {
		t.Fatal(err)
	}
	testkit.ForgeContract(t, f, func(url string) {
		if err := f.Merge(url); err != nil {
			t.Fatal(err)
		}
	})
}

func TestGHForgeOverTheTable(t *testing.T) {
	// gh pr create prints the url; gh pr list finds the open one; gh pr view reports the state.
	// covers: R-SESS-096
	dir := t.TempDir()
	if err := testkit.WriteFakeDefaults(dir); err != nil {
		t.Fatal(err)
	}
	set := func(k, v string) {
		if err := testkit.SetFake(dir, "output", k, v); err != nil {
			t.Fatal(err)
		}
	}
	set("gh pr list --head session/x-1 --base main --state all --json url", "[]\n")
	set("gh pr create --head session/x-1 --base main --title integrate session/x-1 into main --body opened by ness integrate", "https://github.com/o/r/pull/7\n")
	set("gh pr view https://github.com/o/r/pull/7 --json state", `{"state":"OPEN"}`+"\n")
	g := pr.GH{Exec: testkit.FakeExec{Dir: dir}.Run}
	url, err := g.OpenPR("/repo", "session/x-1", "main", "integrate session/x-1 into main")
	if err != nil || url != "https://github.com/o/r/pull/7" {
		t.Fatalf("OpenPR = %q, %v", url, err)
	}
	if ok, err := g.Merged("/repo", url); err != nil || ok {
		t.Fatalf("Merged open = %v, %v", ok, err)
	}
	set("gh pr list --head session/x-1 --base main --state all --json url", `[{"url":"https://github.com/o/r/pull/7"}]`+"\n")
	if again, _ := g.OpenPR("/repo", "session/x-1", "main", "x"); again != url {
		t.Fatalf("existing PR not reused: %q", again)
	}
	set("gh pr view https://github.com/o/r/pull/7 --json state", `{"state":"MERGED"}`+"\n")
	if ok, err := g.Merged("/repo", url); err != nil || !ok {
		t.Fatalf("Merged after = %v, %v", ok, err)
	}
}

func TestPRIntegratorPreviewsLocallyAndRecordsTheURL(t *testing.T) {
	// The fold is a local merge preview for the gate; the ref is the PR url; the source is pushed;
	// integrated? follows the forge.
	// covers: R-SESS-096, R-SESS-102
	g := session.Git{Exec: cli.SystemExec}
	repo := testkit.NewRepo(t, g)
	remote := t.TempDir()
	must := func(dir string, args ...string) string {
		t.Helper()
		out, err := g.Out(dir, args...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	must(remote, "init", "-q", "--bare")
	must(repo, "remote", "add", "origin", remote)
	must(repo, "push", "-q", "origin", "main")
	must(repo, "checkout", "-q", "-b", "session/x-1")
	testkit.Commit(t, g, repo, "a.txt", "1\n", "one")
	must(repo, "checkout", "-q", "main")
	f := testkit.FakeForge{Dir: t.TempDir()}
	if err := testkit.WriteFakeDefaults(f.Dir); err != nil {
		t.Fatal(err)
	}
	it := pr.Integrator{Forge: f}
	if ok, _ := it.Integrated(g, repo, "session/x-1", "main"); ok {
		t.Fatal("integrated before any PR")
	}
	tmp := t.TempDir() + "/tmp"
	must(repo, "worktree", "add", "-q", "--detach", tmp, "main")
	ref, err := it.Fold(g, tmp, "session/x-1", "main")
	if err != nil || !strings.HasPrefix(ref, "https://") {
		t.Fatalf("Fold = %q, %v", ref, err)
	}
	if out := must(tmp, "show", "HEAD:a.txt"); out != "1" {
		t.Fatalf("preview lacks the fold: %q", out)
	}
	if must(remote, "rev-parse", "session/x-1") == "" {
		t.Fatal("source not pushed")
	}
	if ok, _ := it.Integrated(g, repo, "session/x-1", "main"); ok {
		t.Fatal("integrated while the PR is open")
	}
	if err := f.Merge(ref); err != nil {
		t.Fatal(err)
	}
	if ok, err := it.Integrated(g, repo, "session/x-1", "main"); err != nil || !ok {
		t.Fatalf("Integrated after merge = %v, %v", ok, err)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/atom/pr/ ./internal/testkit/`
Expected: FAIL — package `pr` missing.

- [ ] **Step 3: The atom**

`internal/atom/pr/pr.go`:
```go
// Package pr is the integrator that opens a pull request from the session's branch and answers
// integrated? once it is merged (R-SESS-096, §16 #12). The gate still runs on a local merge
// preview; the local target never moves.
package pr

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/atom/merge"
	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// Exec runs a program; cli.SystemExec in production, the fake in tests.
type Exec func(dir, name string, args ...string) ([]byte, error)

// Integrator implements session.Integrator over a Forge.
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

// Fold previews the merge locally in dir (for the gate), pushes the source and opens or finds
// the PR; the returned ref is its url. The store keeps the url; integrated? is the PR's state.
func (it Integrator) Fold(g session.Git, dir, source, target string) (string, error) {
	if _, err := (merge.Integrator{}).Fold(g, dir, source, target); err != nil {
		return "", err
	}
	if _, err := g.Out(dir, "push", "-q", it.remote(), source+":"+source); err != nil {
		return "", err
	}
	return it.Forge.OpenPR(dir, source, target, "integrate "+source+" into "+target)
}

// Integrated is true when the pair's PR is merged; without a PR it is false. The open PR is
// found first; a merged one no longer lists as open, so the recorded url is tried through
// Merged by the cli (close reads the store's url, R-SESS-071).
func (it Integrator) Integrated(_ session.Git, repo, source, target string) (bool, error) {
	url, err := it.Forge.FindPR(repo, source, target)
	if err != nil {
		return false, err
	}
	if url == "" {
		return false, nil
	}
	return it.Forge.Merged(repo, url)
}

// GH is the forge over the gh CLI.
type GH struct{ Exec Exec }

// FindPR returns the url of the PR for head→base in any state (open or merged), or "".
func (f GH) FindPR(repo, head, base string) (string, error) {
	out, err := f.Exec(repo, "gh", "pr", "list", "--head", head, "--base", base, "--state", "all", "--json", "url")
	if err != nil {
		return "", err
	}
	var prs []struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(out, &prs); err != nil {
		return "", fmt.Errorf("gh pr list: %w", err)
	}
	if len(prs) == 0 {
		return "", nil
	}
	return prs[0].URL, nil
}

// OpenPR returns the PR for head→base or creates one.
func (f GH) OpenPR(repo, head, base, title string) (string, error) {
	if url, err := f.FindPR(repo, head, base); err != nil || url != "" {
		return url, err
	}
	out, err := f.Exec(repo, "gh", "pr", "create", "--head", head, "--base", base, "--title", title, "--body", "opened by ness integrate")
	if err != nil {
		return "", err
	}
	url := strings.TrimSpace(string(out))
	if url == "" {
		return "", fmt.Errorf("gh pr create printed no url")
	}
	return url, nil
}

// Merged reports whether the PR's state is MERGED.
func (f GH) Merged(repo, url string) (bool, error) {
	out, err := f.Exec(repo, "gh", "pr", "view", url, "--json", "state")
	if err != nil {
		return false, err
	}
	var v struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(out, &v); err != nil {
		return false, fmt.Errorf("gh pr view: %w", err)
	}
	return v.State == "MERGED", nil
}
```
- [ ] **Step 4: Run and commit**

Run: `go test ./internal/atom/pr/ ./internal/testkit/ && make lint`
Expected: PASS.

```bash
git add internal/atom/pr internal/testkit/contracts.go internal/testkit/forge.go internal/testkit/exec.go internal/cli/cli.go internal/cli/script_test.go
git commit -m "feat(atom): pr integrator over a forge seam, with the gh forge and a fake"
```

---

### Task 3: `ness integrate`

**Files:**
- Create: `internal/cli/integrate.go`, `internal/cli/testdata/script/integrate.txtar`, `integrate_gate.txtar`, `integrate_rights.txtar`, `integrate_pr.txtar`
- Modify: `internal/cli/cli.go` (register), `internal/cli/atoms.go` (`buildIntegrator`, `buildForge`)

**Interfaces:**
- Consumes: `session.Integrate`, `session.Git`, `merge/squash/pr`, `store.SetIntegration`, `projectConfig`, `primaryRoot`, `caller`.
- Produces: `newIntegrateCmd(d Deps) *cobra.Command`; `buildIntegrator(d Deps, mode string) session.Integrator`; `buildForge(d Deps) session.Forge`. Output: one line `integrated <source> into <target> at <sha>` or `opened <url> for <source> into <target>`.

- [ ] **Step 1: Write the failing scripts**

`integrate.txtar`:
```
# A child's integrate defaults --from to its branch and --into to its manager's and
# fast-forwards the manager's worktree; a root's lands in base; the gate runs in a throwaway
# worktree that is removed; the store records the sha; nothing is pushed unless configured;
# a dirty source refuses; nothing else is touched.
# covers: R-SESS-090, R-SESS-091, R-SESS-093, R-SESS-098, R-SESS-099, R-SESS-100, R-SESS-103, R-SESS-104, R-SESS-105
cd repo
exec ness init --gate 'pwd > $WORK/gate-cwd' --base main --yes
exec git add -A
exec git commit -q -m 'ness init'
exec git init -q --bare ../remote
exec git remote add origin ../remote
exec git push -q origin main
exec ness open --root --scope r --tier standard --brief ../b.md
store last-session R
exec sh -c 'cd .worktrees/r-* && NESS_SESSION=$R ness open --scope c --tier standard --brief ../../../b.md'
store last-session C
# The child commits work.
exec sh -c 'cd .worktrees/c-* && echo work > work.txt && git add work.txt && git commit -q -m work'
# A dirty source refuses; the manager's branch is unchanged.
exec sh -c 'cd .worktrees/c-* && echo dirty > dirty.txt'
exec sh -c 'cd .worktrees/r-* && git rev-parse HEAD'
cp stdout ../r-before
exec sh -c 'cd .worktrees/c-* && NESS_SESSION=$C ness integrate; echo "exit $?"'
stdout 'exit 4'
exec sh -c 'cd .worktrees/r-* && git rev-parse HEAD'
cmp stdout ../r-before
exec sh -c 'cd .worktrees/c-* && rm dirty.txt'
# A dirty manager worktree refuses before folding.
exec sh -c 'cd .worktrees/r-* && echo x > manager-dirty.txt'
exec sh -c 'cd .worktrees/c-* && NESS_SESSION=$C ness integrate; echo "exit $?"'
stdout 'exit 4'
exec sh -c 'cd .worktrees/r-* && rm manager-dirty.txt'
# The fold: gate in a temporary worktree, manager's worktree fast-forwarded, sha recorded.
exec sh -c 'cd .worktrees/c-* && NESS_SESSION=$C ness integrate'
stdout '^integrated session/c-[0-9a-f]{12} into session/r-[0-9a-f]{12} at [0-9a-f]{40}$'
grep 'ness/tmp-s-' $WORK/gate-cwd
! grep 'worktrees/c-' $WORK/gate-cwd
exec git worktree list
! stdout 'ness/tmp-'
exec sh -c 'cd .worktrees/r-* && test -f work.txt && git status --porcelain | wc -l'
stdout '^0$'
exec sh -c 'cd .worktrees/r-* && git rev-parse HEAD'
cp stdout ../r-after
exec sh -c 'cd .worktrees/c-* && git rev-parse HEAD'
! cmp stdout ../r-after
store session $C integrated=
! store session $C integrated=
exec sh -c 'cd .worktrees/c-* && git -C . merge-base --is-ancestor HEAD $(cat ../../../r-after)'
# Push is off: the remote did not move.
exec git ls-remote --heads origin main
cp stdout ../remote-main
exec git rev-parse main
exec sh -c 'cut -f1 ../remote-main'
cp stdout ../remote-sha
exec git rev-parse main
cmp stdout ../remote-sha
# The root integrates into base (checked out in the primary checkout), push on.
cp ../push.toml .ness/ness.toml
exec sh -c 'cd .worktrees/r-* && NESS_SESSION=$R ness integrate'
stdout 'into main at'
exec git rev-parse main
cp stdout ../main-after
exec sh -c 'git ls-remote --heads origin main | cut -f1'
cmp stdout ../main-after
exec sh -c 'test -f work.txt'
# Nothing else: no channel post, no probe start went through the exec table.
fake calls 'gh' 0
fake calls 'curl' 0

-- b.md --
b
-- push.toml --
ness = "0.1"
project = "repo"
base = "main"
gate = "true"
[integrate]
push = true
```
`integrate_gate.txtar` (Review Focus 1, R-SESS-092, R-SESS-097):
```
# A failing gate leaves the target unchanged, pushes nothing, records nothing, exits non-zero,
# and the throwaway worktree is removed even when the gate dirtied it; a conflicting fold names
# the path; the squash mode lands one commit.
# covers: R-SESS-092, R-SESS-093, R-SESS-095, R-SESS-097
cd repo
exec ness init --gate 'echo junk > junk.txt && false' --base main --yes
exec git add -A
exec git commit -q -m 'ness init'
exec ness open --root --scope r --tier standard --brief ../b.md
store last-session R
exec sh -c 'cd .worktrees/r-* && echo a > a.txt && git add a.txt && git commit -q -m a'
exec git rev-parse main
cp stdout ../main-before
exec sh -c 'cd .worktrees/r-* && NESS_SESSION=$R ness integrate; echo "exit $?"'
stdout 'exit 1'
exec git rev-parse main
cmp stdout ../main-before
store session $R integrated=
exec git worktree list
! stdout 'ness/tmp-'
! exists .git/ness/tmp-$R
# A conflict: main changed the same file.
exec sh -c 'echo b > a.txt && git add a.txt && git commit -q -m b'
cp ../squash.toml .ness/ness.toml
exec git add -A
exec git commit -q -m squash
exec sh -c 'cd .worktrees/r-* && NESS_SESSION=$R ness integrate; echo "exit $?"'
stdout 'exit 1'
stderr 'a\.txt'
exec git rev-parse main
cp stdout ../main-2
exec sh -c 'cd .worktrees/r-* && git merge -q --no-edit main >/dev/null 2>&1 || (git checkout --theirs a.txt && git add a.txt && git -c core.editor=true commit -q --no-edit)'
exec git rev-list --count main
cp stdout ../n-before
exec sh -c 'cd .worktrees/r-* && NESS_SESSION=$R ness integrate'
stdout 'into main at'
exec sh -c 'echo $(( $(cat ../n-before) + 1 ))'
cp stdout ../n-want
exec git rev-list --count main
cmp stdout ../n-want
exec git log -1 --format=%B main
stdout 'Ness-Integrates: [0-9a-f]{40}'

-- b.md --
b
-- squash.toml --
ness = "0.1"
project = "repo"
base = "main"
gate = "true"
[integrate]
mode = "squash"
```
`integrate_rights.txtar`:
```
# Arbitrary --from/--into (a promotion between long-lived branches) is for roots and the
# operator; a child doing it is refused with exit 4; a child's explicit --into of its manager's
# branch works.
# covers: R-SESS-101, R-SESS-090
cd repo
exec ness init --gate true --base main --yes
exec git add -A
exec git commit -q -m 'ness init'
exec git branch dev
exec git branch prod
exec sh -c 'git checkout -q dev && echo d > d.txt && git add d.txt && git commit -q -m dev && git checkout -q main'
exec ness open --root --scope r --tier standard --brief ../b.md
store last-session R
exec sh -c 'cd .worktrees/r-* && NESS_SESSION=$R ness open --scope c --tier standard --brief ../../../b.md'
store last-session C
exec sh -c 'cd .worktrees/c-* && NESS_SESSION=$C ness integrate --from dev --into prod; echo "exit $?"'
stdout 'exit 4'
exec git rev-parse prod
cp stdout ../prod-before
exec git rev-parse main
cmp stdout ../prod-before
exec sh -c 'cd .worktrees/r-* && NESS_SESSION=$R ness integrate --from dev --into prod'
stdout 'integrated dev into prod'
exec git rev-parse prod
! cmp stdout ../prod-before
exec ness integrate --from dev --into main
stdout 'integrated dev into main'

-- b.md --
b
```

`integrate_pr.txtar`:
```
# With the pr integrator the fold opens a PR after the gate, records its url, moves no local
# ref, and integrated? is true only once merged.
# covers: R-SESS-096, R-SESS-099, R-SESS-102
cd repo
exec ness init --gate true --base main --yes
cp ../pr.toml .ness/ness.toml
exec git add -A
exec git commit -q -m 'ness init'
exec git init -q --bare ../remote
exec git remote add origin ../remote
exec git push -q origin main
exec ness open --root --scope r --tier standard --brief ../b.md
store last-session R
exec sh -c 'cd .worktrees/r-* && echo a > a.txt && git add a.txt && git commit -q -m a'
exec git rev-parse main
cp stdout ../main-before
exec sh -c 'cd .worktrees/r-* && NESS_SESSION=$R ness integrate'
stdout '^opened https://example\.invalid/pr/1 for session/r-[0-9a-f]{12} into main$'
exec git rev-parse main
cmp stdout ../main-before
store session $R integrated=https://example.invalid/pr/1
fake forge prs 1
exec sh -c 'git ls-remote --heads origin | grep -c session/r-'
stdout '^1$'
# Integrating again reuses the PR.
exec sh -c 'cd .worktrees/r-* && NESS_SESSION=$R ness integrate'
fake forge prs 1

-- b.md --
b
-- pr.toml --
ness = "0.1"
project = "repo"
base = "main"
gate = "true"
[integrate]
mode = "pr"
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run 'TestScripts/integrate'`
Expected: FAIL — `unknown command "integrate"`.

- [ ] **Step 3: The command**

Append to `internal/cli/atoms.go`:
```go
func buildForge(d Deps) session.Forge {
	if d.Forge != nil {
		return d.Forge
	}
	return pr.GH{Exec: d.exec()}
}

func buildIntegrator(d Deps, mode string) session.Integrator {
	switch mode {
	case "squash":
		return squash.Integrator{}
	case "pr":
		return pr.Integrator{Forge: buildForge(d)}
	default:
		return merge.Integrator{}
	}
}
```
`internal/cli/integrate.go`:
```go
package cli

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func newIntegrateCmd(d Deps) *cobra.Command {
	var from, into string
	cmd := &cobra.Command{
		Use:   "integrate",
		Short: "Fold a branch into its target through the gate, push if configured, record it",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return withStore(d, func(s *store.Store, c caller) error { return runIntegrate(d, s, c, from, into) })
		},
	}
	cmd.Flags().StringVar(&from, "from", "", "source `REF` (default: this session's branch)")
	cmd.Flags().StringVar(&into, "into", "", "target `REF` (default: the manager's branch; base for a root)")
	return cmd
}

// runIntegrate resolves the pair (R-SESS-090, R-SESS-101), refuses a dirty source (R-SESS-103),
// folds through session.Integrate and records the ref (R-SESS-099).
func runIntegrate(d Deps, s *store.Store, c caller, from, into string) error {
	p, err := loadProject(d)
	if err != nil {
		return err
	}
	cfg, err := projectConfig(d, p)
	if err != nil {
		return err
	}
	primary, err := primaryRoot(p)
	if err != nil {
		return err
	}
	g := session.Git{Exec: d.exec()}
	explicit := from != "" || into != ""
	switch {
	case c.operator() && (from == "" || into == ""):
		return usage("the operator names both --from and --into")
	case !c.operator() && from == "":
		from = c.sess.Branch
	}
	if !c.operator() && into == "" {
		into = cfg.Base
		if !c.root() {
			manager, merr := s.Session(c.sess.Parent)
			if merr != nil {
				return merr
			}
			into = manager.Branch
		}
	}
	if explicit && !c.operator() && !c.root() {
		manager, merr := s.Session(c.sess.Parent)
		if merr != nil {
			return merr
		}
		if from != c.sess.Branch || into != manager.Branch {
			return refuse("a child integrates only its branch into its manager's; --from/--into between other refs is for roots and the operator")
		}
	}
	if !c.operator() && from == c.sess.Branch && c.sess.Worktree != "" {
		dirty, derr := g.Dirty(filepath.Join(primary, c.sess.Worktree))
		if derr != nil {
			return derr
		}
		if dirty {
			return refuse("the worktree of %s has uncommitted changes; commit them before ness integrate", c.id)
		}
	}
	common, err := g.CommonDir(primary)
	if err != nil {
		return err
	}
	tmpName := "tmp-op-" + strings.ReplaceAll(from, "/", "-")
	if !c.operator() {
		tmpName = "tmp-" + c.id
	}
	res, err := session.Integrate(g, buildIntegrator(d, cfg.Integrate.Mode), session.IntegrateSpec{
		Repo: primary, Common: common, TmpName: tmpName, Gate: cfg.Gate,
		Integration: session.Integration{Source: from, Target: into}, Push: cfg.Integrate.Push, Log: d.Stderr})
	switch {
	case errors.Is(err, session.ErrGate):
		printErr(d.Stderr, err)
		return exitCode(ExitProblems)
	case errors.Is(err, session.ErrConflict):
		printErr(d.Stderr, fmt.Errorf("%v; resolve in the session branch, then integrate again", err))
		return exitCode(ExitProblems)
	case err != nil && strings.Contains(err.Error(), "uncommitted changes"):
		return refuse("%v", err)
	case err != nil:
		return err
	}
	if !c.operator() && from == c.sess.Branch {
		if err := s.SetIntegration(c.id, res.Ref, d.Now()); err != nil {
			return err
		}
	}
	if res.MovedRef {
		say(d.Stdout, "integrated %s into %s at %s", from, into, res.Ref)
	} else {
		say(d.Stdout, "opened %s for %s into %s", res.Ref, from, into)
	}
	return nil
}
```
Register `newIntegrateCmd(d)`.

- [ ] **Step 4: Run and commit**

Run: `go test ./internal/cli/ && make lint`
Expected: PASS. If `cyclop` flags `runIntegrate`, extract `resolvePair(s, c, cfg, from, into) (from, into string, err error)`.

```bash
git add internal/cli
git commit -m "feat(cli): ness integrate folds through the gate in a throwaway worktree and records it"
```

---

### Task 4: `ness close`

**Files:**
- Create: `internal/session/close.go`, `internal/cli/close.go`, `internal/cli/testdata/script/close.txtar`, `close_obligations.txtar`, `close_never_ran.txtar`, `close_gone.txtar`, `close_pr.txtar`
- Modify: `internal/cli/cli.go` (register)

**Interfaces:**
- Consumes: `store.Children/Obligations/RecordClose/Retire/Session`, `session.Runtime.CloseTab`, `session.Git`, `session.Forge`, `buildIntegrator`.
- Produces: `session.CloseSpec{Sess store.Session; Primary string; Discard bool; Keep string; Merged func(url string) (bool, error)}`, `session.CheckClose(s *store.Store, spec CloseSpec) error` (the refusals, in order: report, children, obligations, integration/PR), `session.Teardown(g Git, rt Runtime, spec CloseSpec, log io.Writer) error` (tab, worktree, branch), `newCloseCmd(d Deps) *cobra.Command`. Output: `closed <id> (<scope>)`.

- [ ] **Step 1: Write the failing scripts**

`close.txtar`:
```
# Only the manager (child) or the operator (root) closes; close needs report.md, no open
# children, no obligations and a recorded integration unless --discard; a successful close
# removes the worktree and tab, deletes the branch only when integrated, keeps the row as
# closed with its note; --keep-work keeps worktree and branch and records the reason; items
# addressed to the closed session are retired.
# covers: R-SESS-060, R-SESS-061, R-SESS-062, R-SESS-064, R-SESS-065, R-SESS-067, R-SESS-068, R-SESS-069, R-SESS-072, R-SESS-086
cd repo
exec ness init --gate true --base main --yes
exec git add -A
exec git commit -q -m 'ness init'
exec ness open --root --scope r --tier standard --brief ../b.md
store last-session R
exec sh -c 'cd .worktrees/r-* && NESS_SESSION=$R ness open --scope c --tier standard --brief ../../../b.md'
store last-session C
exec sh -c 'cd .worktrees/c-* && NESS_SESSION=$C ness open --scope g --tier standard --brief ../../../b.md'
store last-session G
exec sh -c 'cd .worktrees/r-* && NESS_SESSION=$R ness open --scope d --tier standard --brief ../../../b.md'
store last-session D
# Rights.
env NESS_SESSION=$D
exits 4 ness close $C
stderr 'manager'
env NESS_SESSION=$R
exits 4 ness close $R
stderr 'operator'
# No report.
exits 4 ness close $C
stderr 'rec/r/c/report\.md|docs/sessions/r/c/report\.md'
exec sh -c 'cd .worktrees/c-* && echo done > docs/sessions/r/c/report.md && git add -A && git commit -q -m report'
# Open child.
exits 4 ness close $C
stderr $G
exec sh -c 'cd .worktrees/g-* && echo done > docs/sessions/r/c/g/report.md && git add -A && git commit -q -m report'
env NESS_SESSION=$C
exec ness close $G --discard --note 'nothing to keep'
stdout '^closed s-[0-9a-f]{12} \(g\)$'
store session $G closed=true
exec git branch --list 'session/g-*'
stdout 'session/g-'
exec git worktree list
! stdout 'worktrees/g-'
fake herdr tabs 3
# No integration recorded: refused; --keep-work keeps everything and records the reason.
env NESS_SESSION=$R
exits 4 ness close $C
stderr 'integrat'
exec sh -c 'cd .worktrees/c-* && NESS_SESSION=$C ness integrate'
# A pending item to C is retired by the close.
env NESS_SESSION=$R
exec ness send --to $C --body 'late request'
store items $C 1
exec ness close $C --keep-work 'keep for the demo' --note 'merged'
store session $C closed=true
store items $C 0
exec git worktree list
stdout 'worktrees/c-'
exec git branch --list 'session/c-*'
stdout 'session/c-'
# A plain close after integration deletes the branch.
exec sh -c 'cd .worktrees/d-* && echo done > docs/sessions/r/d/report.md && git add -A && git commit -q -m report && NESS_SESSION=$D ness integrate'
exec ness close $D --note ok
exec git branch --list 'session/d-*'
! stdout .
exec git worktree list
! stdout 'worktrees/d-'
fake herdr tabs 2
# The operator closes the root after its report and integration.
env NESS_SESSION=
exec sh -c 'cd .worktrees/r-* && echo done > docs/sessions/r/report.md && git add -A && git commit -q -m report && NESS_SESSION=$R ness integrate'
exec ness close $R --note 'program done'
store session $R closed=true
exec ness status --all
stdout '\tr\tstandard\tclosed\t'
exec ness status
! stdout '\tr\t'

-- b.md --
b
```
(`store items $C 0` after the retire: `Listed` ignores retired items, Task 1 of P3a.)

`close_obligations.txtar` (the MSG re-covers):
```
# request, decision and handoff items block the recipient's close until disposed; a deferred
# reply still blocks; an fyi never does and neither does a reply item.
# covers: R-SESS-063, R-MSG-040, R-MSG-041, R-MSG-043, R-MSG-022
cd repo
exec ness init --gate true --base main --yes
exec git add -A
exec git commit -q -m 'ness init'
exec ness open --root --scope r --tier standard --brief ../b.md
store last-session R
exec sh -c 'cd .worktrees/r-* && NESS_SESSION=$R ness open --scope c --tier standard --brief ../../../b.md'
store last-session C
exec sh -c 'cd .worktrees/c-* && echo done > docs/sessions/r/c/report.md && git add -A && git commit -q -m report && NESS_SESSION=$C ness integrate'
env NESS_SESSION=$R
exec ness send --to $C --body req --intent request
store last $C REQ
exec ness send --to $C --body dec --intent decision
store last $C DEC
exec ness send --to $C --body hand --intent handoff
store last $C HAND
exec ness send --to $C --body fyi --intent fyi
exits 4 ness close $C
stderr $REQ
stderr $DEC
stderr $HAND
env NESS_SESSION=$C
exec ness reply $REQ --body later --as deferred
exec ness reply $DEC --body yes --as accepted
exec ness reply $HAND --body taken --as accepted
env NESS_SESSION=$R
exits 4 ness close $C
stderr $REQ
! stderr $DEC
env NESS_SESSION=$C
exec ness reply $REQ --body now --as resolved
# C's reply items landed in R's inbox and do not block R; the fyi to C does not block C.
env NESS_SESSION=$R
exec ness inbox
stdout 'reply'
exec ness close $C --note ok
store session $C closed=true

-- b.md --
b
```

`close_never_ran.txtar` (R-SESS-066 and Review Focus 5):
```
# A session that never reached ready closes without report or integration, even with its tab
# gone; a session that was ready is not exempt even when its tab is gone.
# covers: R-SESS-066
cd repo
exec ness init --gate true --base main --yes
cp ../ness.toml .ness/ness.toml
exec git add -A
exec git commit -q -m 'ness init'
fake herdr ready-on-start false
exits 4 ness open --root --scope stuck --tier standard --brief ../b.md
store last-session S
fake herdr close-tab stuck
exec ness close $S
stdout '^closed s-[0-9a-f]{12} \(stuck\)$'
store session $S closed=true
exec git worktree list
! stdout 'worktrees/stuck-'
# A ready session whose tab is gone still needs its report.
fake herdr ready-on-start true
exec ness open --root --scope ran --tier standard --brief ../b.md
store last-session Q
fake herdr close-tab ran
exits 4 ness close $Q
stderr 'report\.md'
exec sh -c 'cd .worktrees/ran-* && echo d > docs/sessions/ran/report.md && git add -A && git commit -q -m r'
exec ness close $Q --discard
store session $Q closed=true

-- ness.toml --
ness = "0.1"
project = "repo"
base = "main"
gate = "true"
[launch]
ready_timeout = "200ms"
-- b.md --
b
```

`close_gone.txtar` (Review Focus 2):
```
# The worktree directory was deleted by hand: close prunes and still succeeds.
# covers: R-SESS-065
cd repo
exec ness init --gate true --base main --yes
exec git add -A
exec git commit -q -m 'ness init'
exec ness open --root --scope r --tier standard --brief ../b.md
store last-session R
exec sh -c 'cd .worktrees/r-* && mkdir -p docs/sessions/r && echo d > docs/sessions/r/report.md && git add -A && git commit -q -m r && NESS_SESSION=$R ness integrate'
exec sh -c 'rm -rf .worktrees/r-*'
exec ness close $R
stdout 'closed'
store session $R closed=true
exec git worktree list
! stdout 'worktrees/'
fake herdr tabs 0

-- b.md --
b
```

`close_pr.txtar` (R-SESS-071):
```
# With the pr integrator close refuses while the PR is open and succeeds once merged.
# covers: R-SESS-071
cd repo
exec ness init --gate true --base main --yes
cp ../pr.toml .ness/ness.toml
exec git add -A
exec git commit -q -m 'ness init'
exec git init -q --bare ../remote
exec git remote add origin ../remote
exec git push -q origin main
exec ness open --root --scope r --tier standard --brief ../b.md
store last-session R
exec sh -c 'cd .worktrees/r-* && echo d > docs/sessions/r/report.md && git add -A && git commit -q -m r && NESS_SESSION=$R ness integrate'
fake forge last PR
exits 4 ness close $R
stderr 'not merged'
exec git worktree list
stdout 'worktrees/r-'
fake forge merge $PR
exec ness close $R
store session $R closed=true

-- b.md --
b
-- pr.toml --
ness = "0.1"
project = "repo"
base = "main"
gate = "true"
[integrate]
mode = "pr"
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run 'TestScripts/close'`
Expected: FAIL — `unknown command "close"`.

- [ ] **Step 3: The close logic**

`internal/session/close.go`:
```go
package session

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// CloseSpec is one ness close.
type CloseSpec struct {
	Sess    store.Session
	Primary string
	Discard bool
	Keep    string
	Merged  func(url string) (bool, error) // nil unless the pr integrator is configured
}

// Checkout is where the session's files live: its worktree, or the primary checkout for an
// adopted root.
func (c CloseSpec) Checkout() string {
	if c.Sess.Worktree == "" {
		return c.Primary
	}
	return filepath.Join(c.Primary, c.Sess.Worktree)
}

// NeverRan reports a session that never wrote a ready mark (R-SESS-066).
func (c CloseSpec) NeverRan() bool { return c.Sess.ReadyAt.IsZero() && !c.Sess.Adopted }

// CheckClose returns the first refusal: report missing (R-SESS-061), open children
// (R-SESS-062), open obligations (R-SESS-063), no integration without --discard (R-SESS-064),
// an unmerged PR (R-SESS-071). A never-ran session skips the report and integration checks.
func CheckClose(s *store.Store, c CloseSpec) error {
	report := filepath.Join(c.Checkout(), c.Sess.Record, "report.md")
	if !c.NeverRan() {
		if _, err := os.Stat(report); err != nil {
			return fmt.Errorf("no report at %s; write it, then close", report)
		}
	}
	kids, err := s.Children(c.Sess.ID)
	if err != nil {
		return err
	}
	if len(kids) > 0 {
		ids := make([]string, 0, len(kids))
		for _, k := range kids {
			ids = append(ids, k.ID+" ("+k.Scope+")")
		}
		return fmt.Errorf("session %s has open children: %s; close them first", c.Sess.ID, strings.Join(ids, ", "))
	}
	obl, err := s.Obligations(c.Sess.ID)
	if err != nil {
		return err
	}
	if len(obl) > 0 {
		ids := make([]string, 0, len(obl))
		for _, it := range obl {
			ids = append(ids, it.ID+" ("+it.Intent+" from "+nameOf(it.Sender)+")")
		}
		return fmt.Errorf("session %s has open obligations: %s; reply to them, then close", c.Sess.ID, strings.Join(ids, ", "))
	}
	if c.NeverRan() || c.Discard {
		return nil
	}
	if c.Sess.Integrated == "" {
		return fmt.Errorf("session %s has no integration recorded; run ness integrate, or close with --discard", c.Sess.ID)
	}
	if c.Merged != nil && strings.HasPrefix(c.Sess.Integrated, "http") {
		ok, err := c.Merged(c.Sess.Integrated)
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("pull request %s is not merged; merge it, then close (or --discard)", c.Sess.Integrated)
		}
	}
	return nil
}

func nameOf(id string) string {
	if id == store.Operator {
		return "operator"
	}
	return id
}

// Teardown removes the tab (a gone tab is fine), the worktree (a missing directory is pruned)
// and, when integrated and not kept, the branch (R-SESS-065, R-SESS-072, R-SESS-067).
func Teardown(g Git, rt Runtime, c CloseSpec, log io.Writer) error {
	if c.Sess.Tab != "" && !c.Sess.Adopted {
		if err := rt.CloseTab(c.Sess.Tab); err != nil && !errors.Is(err, ErrGone) {
			return err
		}
	}
	if c.Sess.Worktree == "" || c.Keep != "" {
		return nil
	}
	wt := filepath.Join(c.Primary, c.Sess.Worktree)
	if _, err := os.Stat(wt); err == nil {
		if _, err := g.Out(c.Primary, "worktree", "remove", "--force", wt); err != nil {
			return err
		}
	}
	if _, err := g.Out(c.Primary, "worktree", "prune"); err != nil {
		return err
	}
	if c.Sess.Integrated != "" && !c.Discard {
		if _, err := g.Out(c.Primary, "branch", "-D", c.Sess.Branch); err != nil {
			_, _ = fmt.Fprintf(log, "ness: branch %s not deleted: %v\n", c.Sess.Branch, err)
		}
	}
	return nil
}
```

- [ ] **Step 4: The command**

`internal/cli/close.go`:
```go
package cli

import (
	"errors"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func newCloseCmd(d Deps) *cobra.Command {
	var note, keep string
	var discard bool
	cmd := &cobra.Command{
		Use:   "close ID",
		Short: "Close a child you manage (or, as the operator, a root)",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			return withStore(d, func(s *store.Store, c caller) error { return runClose(d, s, c, args[0], note, keep, discard) })
		},
	}
	f := cmd.Flags()
	f.StringVar(&note, "note", "", "close note `TEXT`, recorded with the close")
	f.StringVar(&keep, "keep-work", "", "keep the worktree and branch, recording `REASON`")
	f.BoolVar(&discard, "discard", false, "close without an integration")
	return cmd
}

func runClose(d Deps, s *store.Store, c caller, id, note, keep string, discard bool) error {
	sess, err := s.Session(id)
	if errors.Is(err, store.ErrNoSession) {
		return refuse("%s is not a session of this project", id)
	}
	if err != nil {
		return err
	}
	if sess.Closed {
		return refuse("session %s is already closed", id)
	}
	switch {
	case sess.Parent == store.Operator && !c.operator():
		return refuse("only the operator closes a root")
	case sess.Parent != store.Operator && c.id != sess.Parent:
		return refuse("only %s's manager (%s) closes it", id, sess.Parent)
	}
	p, err := loadProject(d)
	if err != nil {
		return err
	}
	cfg, err := projectConfig(d, p)
	if err != nil {
		return err
	}
	primary, err := primaryRoot(p)
	if err != nil {
		return err
	}
	spec := session.CloseSpec{Sess: sess, Primary: primary, Discard: discard, Keep: keep}
	if cfg.Integrate.Mode == "pr" {
		forge := buildForge(d)
		spec.Merged = func(url string) (bool, error) { return forge.Merged(primary, url) }
	}
	if err := session.CheckClose(s, spec); err != nil {
		return refuse("%v", err)
	}
	if err := session.Teardown(session.Git{Exec: d.exec()}, buildRuntime(d), spec, d.Stderr); err != nil {
		return err
	}
	if _, err := s.Retire(id, d.Now()); err != nil {
		return err
	}
	if err := s.RecordClose(id, note, keep, d.Now()); err != nil {
		return err
	}
	say(d.Stdout, "closed %s (%s)", id, sess.Scope)
	return nil
}
```
Register `newCloseCmd(d)`. P5 adds the probe ownership transfer before `RecordClose` (R-SESS-070).

- [ ] **Step 5: Run and commit**

Run: `go test ./internal/cli/ ./internal/session/ && make lint`
Expected: PASS.

```bash
git add internal/session/close.go internal/cli/close.go internal/cli/cli.go internal/cli/testdata/script
git commit -m "feat(cli): ness close behind the report, children, obligation and integration checks"
```

---

### Task 5: `ness doctor`: the environment and binary checks

**Files:**
- Create: `internal/cli/doctor.go`, `internal/cli/doctor_checks.go`, `internal/cli/testdata/script/doctor.txtar`, `doctor_outside.txtar`
- Modify: `internal/cli/cli.go` (register)

**Interfaces:**
- Consumes: `session.CheckVersion`, `session.CheckShadowing`, `session.SkillRoots`, `session.TierEffort`, `buildRuntime`, `buildClient`, `loadProfiles`, `config.CheckPin`, `executable()`.
- Produces: `type check struct { Name, Status, Detail, Fix string }` (Status `ok|warn|fail`), `type checker func(*doctorEnv) check`, `doctorEnv{d Deps; p *project; cfg *config.Config; primary string; s *store.Store; now time.Time}`, `newDoctorCmd(d Deps) *cobra.Command`, the text line `<status>\t<name>\t<detail>[\tfix: <command>]`, the JSON `{"schema":1,"version":"…","checks":[{"name","status","detail","fix"}]}`. Checks of this task: `binary`, `hooks-binary`, `worker-binary`, `herdr-version`, `client-version`, `tier-models`, `skill-shadowing`, `profile-tools`. Task 6 adds the rest.

- [ ] **Step 1: Write the failing scripts**

`doctor.txtar`:
```
# Doctor reports each check as ok, warn or fail on one line with a fix command on every fail,
# completes all checks when several fail, never applies a fix, exits 1 on any fail and 0 with
# only warnings, and --json carries name, status and fix per check. Pin mismatch names both
# versions and the path; the worker unit's ExecStart is compared with the running binary;
# untested client and herdr versions fail; a Codex tier id missing from codex debug models
# names its row; a shadowing skill names both paths; a profile tool missing from PATH names
# the tool and the profile.
# covers: R-DOC-001, R-DOC-002, R-DOC-003, R-DOC-004, R-DOC-005, R-DOC-006, R-DOC-007, R-DOC-008, R-DOC-009, R-DOC-017, R-DOC-020, R-DOC-021, R-SESS-016
cd repo
exec ness init --gate true --base main --yes
exec git add -A
exec git commit -q -m 'ness init'
# All green: a unit pointing at this binary, worker fresh (Task 6 adds the store checks).
exec sh -c 'which ness'
cp stdout ../nesspath
exec sh -c 'mkdir -p $HOME/.config/systemd/user && printf "[Service]\nExecStart=%s worker run\n" "$(cat ../nesspath)" > $HOME/.config/systemd/user/ness-repo.service'
store heartbeat 5 0.1.0
exec ness doctor
stdout '^ok\tbinary\t0\.1\.0 .*pin 0\.1'
stdout '^ok\thooks-binary\t'
stdout '^ok\tworker-binary\t'
stdout '^ok\therdr-version\t0\.8\.2'
stdout '^ok\tclient-version\tclaude 2\.1\.294'
stdout '^ok\ttier-models\t'
stdout '^ok\tskill-shadowing\t'
stdout '^ok\tprofile-tools\t'
! stdout '^fail'
exec ness doctor --json
stdout '"schema": 1'
stdout '"name": "binary"'
stdout '"status": "ok"'
stdout '"fix": ""'
# Every check fails at once; each line carries a fix; the tree, refs and store are untouched.
snap before
exec git for-each-ref
cp stdout ../refs-before
store sessions 0
env NESS_TEST_VERSION=0.2.0
exec sh -c 'printf "[Service]\nExecStart=/usr/local/bin/other-ness worker run\n" > $HOME/.config/systemd/user/ness-repo.service'
fake output 'codex debug models' '{"models":[{"slug":"gpt-6-luna"}]}'
fake herdr version 0.5.0
fake output 'claude --version' '1.0.0 (Claude Code)'
cp ../bad.toml .ness/ness.toml
mkdir $HOME/.claude/skills/deploy
mkdir .agents/skills/deploy
exits 1 ness doctor
stdout '^fail\tbinary\t.*0\.2\.0.*0\.1.*ness.*\tfix: '
stdout '^fail\tworker-binary\t.*/usr/local/bin/other-ness.*\tfix: ness worker install'
stdout '^fail\therdr-version\t0\.5\.0.*0\.8\.2\tfix: '
stdout '^fail\tclient-version\tclaude 1\.0\.0.*2\.1\.294\tfix: '
stdout '^fail\ttier-models\t.*frontier.*astra.*\tfix: '
stdout '^fail\tskill-shadowing\t.*\.claude/skills/deploy.*\.agents/skills/deploy\tfix: '
stdout '^fail\tprofile-tools\t.*nonexistent-tool-xyz.*local\tfix: '
stdout -count=1 '^(ok|warn|fail)\tbinary\t'
samesnap before
exec git for-each-ref
cmp stdout ../refs-before
store sessions 0
fake herdr tabs 0
# Only warnings exit 0: no unit installed is a warning.
env NESS_TEST_VERSION=0.1.0
rm $HOME/.config/systemd/user/ness-repo.service
exec sh -c 'cp ../ness-codex.toml .ness/ness.toml; rm -r $HOME/.claude/skills/deploy .agents/skills/deploy'
fake herdr version 0.8.2
fake output 'claude --version' '2.1.294 (Claude Code)'
fake output 'codex debug models' '{"models":[{"slug":"gpt-6-luna"},{"slug":"gpt-6.1-sol"},{"slug":"astra"}]}'
exec ness doctor
stdout '^warn\tworker-binary\tno unit .*\tfix: ness worker install'
# Codex ids are checked against codex debug models when the client is codex.
fake output 'codex debug models' '{"models":[{"slug":"gpt-6-luna"}]}'
exits 1 ness doctor
stdout '^fail\ttier-models\ttier frontier \(codex = "astra"\).*\tfix: '

-- bad.toml --
ness = "0.1"
project = "repo"
base = "main"
gate = "true"
profiles = ["local"]
experimental = ["codex"]
client = "codex"
-- repo/.ness/profiles/local/profile.toml --
name = "local"
requires_tools = ["nonexistent-tool-xyz"]
-- ness-codex.toml --
ness = "0.1"
project = "repo"
base = "main"
gate = "true"
experimental = ["codex"]
client = "codex"
```
`doctor_outside.txtar` (R-DOC-023, Review Focus 4):
```
# Outside a project doctor prints the version line, then refuses; a malformed ness.toml is a
# refusal naming the file, not a crash.
# covers: R-DOC-023, R-CFG-003
exits 4 ness doctor
stdout '^ness 0\.1\.0 at /.*ness$'
stderr 'not inside a git repository'
cd repo
exits 4 ness doctor
stdout '^ness 0\.1\.0 at '
stderr 'run ness init'
exec ness init --gate true --base main --yes
cp ../broken.toml .ness/ness.toml
exits 4 ness doctor
stdout '^ness 0\.1\.0 at '
stderr '\.ness/ness\.toml:2:'
cp ../nogate.toml .ness/ness.toml
exits 4 ness doctor
stderr 'gate is required'

-- broken.toml --
ness = "0.1"
project = = "x"
-- nogate.toml --
ness = "0.1"
project = "repo"
base = "main"
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run 'TestScripts/doctor'`
Expected: FAIL — `unknown command "doctor"`.

- [ ] **Step 3: The command**

`internal/cli/doctor.go`:
```go
package cli

import (
	"encoding/json"
	"errors"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// check is one doctor line (R-DOC-020): Status is ok, warn or fail; Fix is a command, set on
// every fail and never run (R-DOC-002).
type check struct {
	Name   string `json:"name"`
	Status string `json:"status"`
	Detail string `json:"detail"`
	Fix    string `json:"fix"`
}

func ok(name, detail string) check   { return check{Name: name, Status: "ok", Detail: detail} }
func warn(name, detail, fix string) check { return check{Name: name, Status: "warn", Detail: detail, Fix: fix} }
func fail(name, detail, fix string) check { return check{Name: name, Status: "fail", Detail: detail, Fix: fix} }

// doctorEnv is what the checks read. Nothing in it writes.
type doctorEnv struct {
	d       Deps
	p       *project
	cfg     *config.Config
	primary string
	s       *store.Store
	now     time.Time
}

type checker func(*doctorEnv) check

// checks run in this order; each one completes whatever the others found (R-DOC-017).
var checks = []checker{
	checkBinary, checkHooksBinary, checkWorkerBinary, checkHerdrVersion, checkClientVersion,
	checkTierModels, checkShadowing, checkProfileTools,
}

type doctorJSON struct {
	Schema  int     `json:"schema"`
	Version string  `json:"version"`
	Checks  []check `json:"checks"`
}

func newDoctorCmd(d Deps) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Check this installation and project, read-only; print a fix for every failure",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return runDoctor(d, asJSON) },
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "print one JSON object")
	return cmd
}

// runDoctor prints the version line first, so a refusal outside a project still says which
// binary answered (R-DOC-023), then runs every check (R-DOC-001: nothing here writes).
func runDoctor(d Deps, asJSON bool) (err error) {
	if !asJSON {
		say(d.Stdout, "ness %s at %s", d.Version, executable())
	}
	p, err := loadProject(d)
	if err != nil {
		return err
	}
	cfg, err := projectConfig(d, p)
	if err != nil {
		return err
	}
	primary, err := primaryRoot(p)
	if err != nil {
		return err
	}
	s, _, err := openStore(d, p)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.Close()) }()
	env := &doctorEnv{d: d, p: p, cfg: cfg, primary: primary, s: s, now: d.Now()}
	results := make([]check, 0, len(checks))
	failed := false
	for _, c := range checks {
		r := c(env)
		failed = failed || r.Status == "fail"
		results = append(results, r)
	}
	if asJSON {
		enc := json.NewEncoder(d.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(doctorJSON{Schema: 1, Version: d.Version, Checks: results}); err != nil {
			return err
		}
	} else {
		for _, r := range results {
			line := r.Status + "\t" + r.Name + "\t" + r.Detail
			if r.Fix != "" {
				line += "\tfix: " + r.Fix
			}
			say(d.Stdout, "%s", line)
		}
	}
	if failed {
		return exitCode(ExitProblems)
	}
	return nil
}

var _ = os.Getenv
```
(`loadProject` refuses outside a project and on a malformed file; the version line is already out. Drop the `var _` once `os` is used by Task 6.) Note `projectConfig` runs the profiles' `requires_tools` only through `checkProfileTools`, never `checkTools` (which refuses).

- [ ] **Step 4: The checks**

`internal/cli/doctor_checks.go`:
```go
package cli

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// checkBinary: the running binary against the project pin, with its path (R-DOC-004).
func checkBinary(e *doctorEnv) check {
	detail := fmt.Sprintf("%s at %s, pin %s", e.d.Version, executable(), e.p.file.Pin())
	if err := config.CheckPin(e.p.file.Pin(), e.d.Version); err != nil {
		return fail("binary", detail+": "+err.Error(), "install ness "+e.p.file.Pin()+".x (mise use ness@"+e.p.file.Pin()+") or change the pin in .ness/ness.toml")
	}
	return ok("binary", detail)
}

// checkHooksBinary: the ness the client hooks resolve through PATH (R-DOC-005).
func checkHooksBinary(_ *doctorEnv) check {
	me := realPath(executable())
	found, err := exec.LookPath("ness")
	if err != nil {
		return fail("hooks-binary", "ness is not on PATH; the client hooks resolve nothing", "put "+me+" on PATH")
	}
	if realPath(found) != me {
		return fail("hooks-binary", "hooks resolve "+found+", this binary is "+me, "put "+me+" first on PATH")
	}
	return ok("hooks-binary", "hooks resolve "+found)
}

// checkWorkerBinary: the worker unit's ExecStart against this binary (R-DOC-005, §16 #3).
func checkWorkerBinary(e *doctorEnv) check {
	unit := unitPath(e.cfg.Project)
	f, err := os.Open(unit) //nolint:gosec // the unit of this project
	if err != nil {
		return warn("worker-binary", "no unit at "+unit+" (worker not installed)", "ness worker install")
	}
	defer func() { _ = f.Close() }()
	me := realPath(executable())
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if bin, okLine := strings.CutPrefix(sc.Text(), "ExecStart="); okLine {
			bin = strings.Fields(bin)[0]
			if realPath(bin) != me {
				return fail("worker-binary", "unit ExecStart runs "+bin+", this binary is "+me, "ness worker install")
			}
			return ok("worker-binary", "unit runs "+bin)
		}
	}
	return fail("worker-binary", "unit "+unit+" has no ExecStart", "ness worker install")
}

func unitPath(project string) string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "systemd", "user", "ness-"+project+".service")
}

func realPath(p string) string {
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

// checkHerdrVersion (R-DOC-006).
func checkHerdrVersion(e *doctorEnv) check {
	v, err := buildRuntime(e.d).Version()
	if errors.Is(err, session.ErrNotInstalled) {
		return fail("herdr-version", "herdr is not installed", "install herdr "+session.TestedHerdr+" (https://herdr.dev)")
	}
	if err != nil {
		return fail("herdr-version", err.Error(), "herdr --version")
	}
	if err := session.CheckVersion("herdr", v, session.TestedHerdr); err != nil {
		return fail("herdr-version", v+" is not supported; tested "+session.TestedHerdr, "herdr update")
	}
	return ok("herdr-version", v)
}

// checkClientVersion (R-DOC-006).
func checkClientVersion(e *doctorEnv) check {
	c := buildClient(e.d, e.cfg.Client)
	v, err := c.Version()
	if errors.Is(err, session.ErrNotInstalled) {
		return fail("client-version", c.Name()+" is not installed", "install "+c.Name()+" "+c.Tested())
	}
	if err != nil {
		return fail("client-version", err.Error(), c.Name()+" --version")
	}
	if err := session.CheckVersion(c.Name(), v, c.Tested()); err != nil {
		return fail("client-version", c.Name()+" "+v+" is not supported; tested "+c.Tested(), c.Name()+" update")
	}
	return ok("client-version", c.Name()+" "+v)
}

// checkTierModels: every tier row resolves for the configured client (R-DOC-007).
func checkTierModels(e *doctorEnv) check {
	c := buildClient(e.d, e.cfg.Client)
	var bad []string
	for _, name := range slices.Sorted(tierNames(e.cfg.Tiers)) {
		model, _ := session.TierEffort(e.cfg.Tiers[name], c.Name())
		if err := c.ResolveModel(model); err != nil {
			bad = append(bad, fmt.Sprintf("tier %s (%s = %q): %v", name, c.Name(), model, err))
		}
	}
	if len(bad) > 0 {
		return fail("tier-models", strings.Join(bad, "; "), "edit [tiers.<name>] in .ness/ness.toml")
	}
	return ok("tier-models", fmt.Sprintf("%d tiers resolve for %s", len(e.cfg.Tiers), c.Name()))
}

// checkShadowing (R-DOC-008): every pair, not just the first.
func checkShadowing(e *doctorEnv) check {
	project := filepath.Join(e.primary, ".agents", "skills")
	var pairs []string
	local, err := os.ReadDir(project)
	if err != nil {
		return ok("skill-shadowing", "no project skills")
	}
	for _, root := range skillRoots() {
		for _, d := range local {
			if !d.IsDir() || strings.Contains(d.Name(), ":") {
				continue
			}
			if info, serr := os.Stat(filepath.Join(root, d.Name())); serr == nil && info.IsDir() {
				pairs = append(pairs, filepath.Join(root, d.Name())+" shadows "+filepath.Join(project, d.Name()))
			}
		}
	}
	if len(pairs) > 0 {
		return fail("skill-shadowing", strings.Join(pairs, "; "), "rename or remove the global skill")
	}
	return ok("skill-shadowing", "no global skill shadows a project skill")
}

// checkProfileTools: requires_tools of the applied profiles on PATH (R-DOC-009).
func checkProfileTools(e *doctorEnv) check {
	res, err := loadProfiles(e.d, e.p, e.cfg.Profiles)
	if err != nil {
		return fail("profile-tools", err.Error(), "fix the profile, then ness sync")
	}
	var missing []string
	for _, tool := range slices.Sorted(mapsKeysOf(res.Tools)) {
		if _, lerr := exec.LookPath(tool); lerr != nil {
			missing = append(missing, tool+" (profile "+res.Tools[tool]+")")
		}
	}
	if len(missing) > 0 {
		return fail("profile-tools", "not on PATH: "+strings.Join(missing, ", "), "install them (mise use …)")
	}
	return ok("profile-tools", fmt.Sprintf("%d required tools present", len(res.Tools)))
}

func mapsKeysOf(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
```
`checkShadowing` reuses the rule of `session.CheckShadowing` but lists every pair (R-DOC-008 "every global skill"); keep the single-pair refusal in `session` for open and the hook. Register `newDoctorCmd(d)`.

- [ ] **Step 5: Run and commit**

Run: `go test ./internal/cli/ -run 'TestScripts/doctor' && make lint`
Expected: PASS for `doctor_outside`; `doctor.txtar` passes its first half and fails on the Task 6 lines (`worker`, store checks) — those lines are added in Task 6, so `doctor.txtar` as written above passes now; keep it so.

```bash
git add internal/cli
git commit -m "feat(cli): ness doctor checks the binary, hooks, unit, versions, tiers, skills and tools"
```

---

### Task 6: `ness doctor`: worker, sync drift, store health, systemd

**Files:**
- Create: `internal/cli/testdata/script/doctor_store.txtar`
- Modify: `internal/cli/doctor.go` (checks list), `internal/cli/doctor_checks.go`, `internal/cli/testdata/script/doctor.txtar` (the systemd and worker lines), `internal/testkit/exec.go` (default `systemctl` output)

**Interfaces:**
- Produces the checks `worker`, `systemd`, `sync-drift`, `dead-routes`, `gone-panes`, `stray-worktrees`, `probes` (P5 fills; here `ok` with "probes feature off" or `warn` when the feature is on).

- [ ] **Step 1: Write the failing script**

`doctor_store.txtar`:
```
# Doctor reports the worker (running, version, heartbeat age with 60 s as the line), a missing
# systemd user session with the ness worker run alternative, sync drift without writing, dead
# routes, sessions whose pane is gone and worktrees without a session.
# covers: R-DOC-010, R-DOC-011, R-DOC-012, R-DOC-013, R-DOC-014, R-DOC-016, R-DOC-022, R-DOC-001
cd repo
exec ness init --gate true --base main --yes
exec git add -A
exec git commit -q -m 'ness init'
env NESS_TEST_NOW=2026-10-08T10:00:00Z
exec ness doctor
stdout '^fail\tworker\tnot running \(no heartbeat\)\tfix: ness worker install'
stdout '^ok\tsystemd\t'
stdout '^ok\tsync-drift\t'
stdout '^ok\tdead-routes\t'
stdout '^ok\tgone-panes\t'
stdout '^ok\tstray-worktrees\t'
store heartbeat 59 0.1.0
exec ness doctor
stdout '^ok\tworker\trunning 0\.1\.0, heartbeat 59s ago'
store heartbeat 61 0.1.0
exits 1 ness doctor
stdout '^fail\tworker\theartbeat 61s ago.*\tfix: systemctl --user restart ness-repo'
store heartbeat 5 0.0.9
exits 1 ness doctor
stdout '^fail\tworker\trunning 0\.0\.9, this binary is 0\.1\.0.*\tfix: systemctl --user restart ness-repo'
store heartbeat 5 0.1.0
# No systemd user session.
fake fail 'systemctl --user is-system-running' 'Failed to connect to bus: No such file or directory'
exits 1 ness doctor
stdout '^fail\tsystemd\tno systemd user session.*\tfix: ness worker run \(in a terminal, without systemd\)'
fake output 'systemctl --user is-system-running' 'running'
# Sync drift: a generated file edited; the tree is not written.
exec sh -c 'echo extra >> .agents/skills/ness-entry/SKILL.md'
snap drift
exits 1 ness doctor
stdout '^fail\tsync-drift\t.*ness-entry.*\tfix: ness sync'
samesnap drift
exec git checkout -q -- .agents/skills/ness-entry/SKILL.md
# A dead route: an open item to a closed session, and one to an unknown session.
store add-session s-aaaaaaaaaaaa root
store add-session s-bbbbbbbbbbbb child s-aaaaaaaaaaaa
env NESS_SESSION=s-aaaaaaaaaaaa
exec ness send --to s-bbbbbbbbbbbb --body pending
store last s-bbbbbbbbbbbb ITEM
env NESS_SESSION=
store close-session s-bbbbbbbbbbbb
exits 1 ness doctor
stdout '^fail\tdead-routes\t.*'$ITEM'.*s-bbbbbbbbbbbb.*closed.*\tfix: '
# A session whose pane is gone.
store bind s-aaaaaaaaaaaa wF:p9 wF:t9 p-root
exits 1 ness doctor
stdout '^fail\tgone-panes\t.*s-aaaaaaaaaaaa.*wF:p9.*\tfix: ness open --resume s-aaaaaaaaaaaa'
# A worktree without a session.
exec git worktree add -q .worktrees/stray-000000000000
exits 1 ness doctor
stdout '^fail\tstray-worktrees\t.*stray-000000000000.*\tfix: git worktree remove'
exec git worktree list
stdout 'stray-000000000000'
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run 'TestScripts/doctor_store'`
Expected: FAIL — no `worker` line.

- [ ] **Step 3: The checks**

Append to `checks` in `doctor.go`: `checkWorker, checkSystemd, checkSyncDrift, checkDeadRoutes, checkGonePanes, checkStrayWorktrees, checkProbes`. Append to `doctor_checks.go`:
```go
// checkWorker: running, same version, heartbeat age with the 60 s line (R-DOC-010, R-DOC-022).
func checkWorker(e *doctorEnv) check {
	restart := "systemctl --user restart ness-" + e.cfg.Project
	hb, err := e.s.Heartbeat()
	if errors.Is(err, store.ErrNoHeartbeat) {
		return fail("worker", "not running (no heartbeat)", "ness worker install")
	}
	if err != nil {
		return fail("worker", err.Error(), restart)
	}
	age := int(e.now.Sub(hb.At).Seconds())
	if age > 60 {
		return fail("worker", fmt.Sprintf("heartbeat %ds ago (older than 60 s): the worker is not running", age), restart)
	}
	if hb.Version != e.d.Version {
		return fail("worker", fmt.Sprintf("running %s, this binary is %s", hb.Version, e.d.Version), restart)
	}
	return ok("worker", fmt.Sprintf("running %s, heartbeat %ds ago", hb.Version, age))
}

// checkSystemd: a systemd user session for the worker unit (R-DOC-016).
func checkSystemd(e *doctorEnv) check {
	out, err := e.d.exec()("", "systemctl", "--user", "is-system-running")
	if err != nil && !strings.Contains(strings.TrimSpace(string(out)), "degraded") {
		return fail("systemd", "no systemd user session: "+strings.TrimSpace(err.Error()), "ness worker run (in a terminal, without systemd)")
	}
	return ok("systemd", "user session "+strings.TrimSpace(string(out)))
}

// checkSyncDrift: what ness sync --check would report, without writing (R-DOC-011).
func checkSyncDrift(e *doctorEnv) check {
	root, err := os.OpenRoot(e.primary)
	if err != nil {
		return fail("sync-drift", err.Error(), "ness sync --check")
	}
	defer func() { _ = root.Close() }()
	plan, err := planSync(e.d, e.p, root)
	if err != nil {
		return fail("sync-drift", err.Error(), "ness sync")
	}
	var changed []string
	for _, fp := range plan.Files {
		if fp.Conflict || fp.Err != nil || fp.Writes() {
			changed = append(changed, fp.Path)
		}
	}
	if len(changed) > 0 {
		return fail("sync-drift", "would change "+strings.Join(changed, ", "), "ness sync")
	}
	return ok("sync-drift", "generated files are current")
}

// checkDeadRoutes: open items addressed to closed or unknown sessions (R-DOC-012).
func checkDeadRoutes(e *doctorEnv) check {
	dead, err := e.s.DeadRoutes()
	if err != nil {
		return fail("dead-routes", err.Error(), "ness inbox")
	}
	if len(dead) > 0 {
		return fail("dead-routes", strings.Join(dead, "; "), "ness worker run (it retires them) or ness reply ITEM --as declined")
	}
	return ok("dead-routes", "every open item has a live recipient")
}

// checkGonePanes: open sessions whose pane the runtime no longer has (R-DOC-013).
func checkGonePanes(e *doctorEnv) check {
	open, err := e.s.Sessions(false)
	if err != nil {
		return fail("gone-panes", err.Error(), "ness status")
	}
	rt := buildRuntime(e.d)
	var gone []string
	for _, s := range open {
		if s.Pane == "" {
			continue
		}
		if _, serr := rt.State(s.Pane); errors.Is(serr, session.ErrGone) {
			gone = append(gone, s.ID+" (pane "+s.Pane+")")
		}
	}
	if len(gone) > 0 {
		return fail("gone-panes", strings.Join(gone, "; "), "ness open --resume "+strings.Fields(gone[0])[0])
	}
	return ok("gone-panes", "every open session has its pane")
}

// checkStrayWorktrees: directories under .worktrees/ no open session owns (R-DOC-014).
func checkStrayWorktrees(e *doctorEnv) check {
	entries, err := os.ReadDir(filepath.Join(e.primary, ".worktrees"))
	if err != nil {
		return ok("stray-worktrees", "no .worktrees directory")
	}
	open, err := e.s.Sessions(true)
	if err != nil {
		return fail("stray-worktrees", err.Error(), "ness status --all")
	}
	owned := map[string]bool{}
	for _, s := range open {
		if !s.Closed || s.KeepReason != "" {
			owned[filepath.Base(s.Worktree)] = true
		}
	}
	var stray []string
	for _, d := range entries {
		if d.IsDir() && !owned[d.Name()] {
			stray = append(stray, d.Name())
		}
	}
	if len(stray) > 0 {
		return fail("stray-worktrees", ".worktrees/"+strings.Join(stray, ", .worktrees/")+" without a session",
			"git worktree remove --force .worktrees/"+stray[0])
	}
	return ok("stray-worktrees", "every worktree has a session")
}

// checkProbes is filled by P5 (R-DOC-015).
func checkProbes(e *doctorEnv) check {
	if slices.Contains(e.cfg.Features, "probes") {
		return warn("probes", "probe checks arrive with the probes feature (P5)", "")
	}
	return ok("probes", "feature off")
}
```
`DeadRoutes` is new in `internal/store/items.go`:
```go
// DeadRoutes lists open items whose recipient is closed or unknown (R-DOC-012), as
// "<item> to <recipient> (closed|unknown)".
func (s *Store) DeadRoutes() (out []string, err error) {
	rows, err := s.db.QueryContext(context.Background(),
		`SELECT i.id, i.recipient, COALESCE(r.state, 'unknown') FROM items i LEFT JOIN sessions r ON r.id = i.recipient
		 WHERE i.recipient IS NOT NULL AND i.disposed_at IS NULL AND i.retired_at IS NULL AND COALESCE(r.state, '') <> 'open'
		 ORDER BY i.created_at`)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		var id, to, state string
		if err := rows.Scan(&id, &to, &state); err != nil {
			return nil, err
		}
		out = append(out, id+" to "+to+" ("+state+")")
	}
	return out, rows.Err()
}
```
with a unit test in `items_test.go` (`// covers: R-DOC-012`). `WriteFakeDefaults` adds `"systemctl --user is-system-running": "running\n"` to the outputs table. The runtime's `State` on a pane the fake does not know returns `ErrGone` (P3a's `pane read` answers `not_found`).

- [ ] **Step 4: Run and commit**

Run: `go test ./internal/cli/ ./internal/store/ && make lint`
Expected: PASS.

```bash
git add internal/cli internal/store internal/testkit/exec.go
git commit -m "feat(cli): doctor checks the worker, systemd, sync drift, dead routes, gone panes and stray worktrees"
```

---

### Task 7: The Codex sync keys (`.codex/config.toml`)

**Files:**
- Create: `internal/sync/tomlkeys.go`, `internal/sync/tomlkeys_test.go`, `internal/cli/testdata/script/sync_codex.txtar`
- Modify: `internal/sync/engine.go` (`opsFor`), `internal/sync/render.go` (`codexEntries`), `internal/sync/render_test.go`

**Interfaces:**
- Produces: `tomlGet(data []byte, key string) ([]byte, bool, error)`, `tomlSet(data []byte, key string, v []byte) ([]byte, error)`, `tomlDelete(data []byte, key string) ([]byte, error)` (dotted keys `a.b` = table `[a]`, key `b`; top-level keys before the first table; values are TOML literals as rendered by `rawTOML`), `FuzzTOMLKeys`. Render entries when `cfg.Client == "codex"`: `.codex/config.toml:model_auto_compact_token_limit` = `235000`, `:agents.model` = `"<economy codex model>"`, `:agents.model_reasoning_effort` = `"<economy codex effort>"`.

- [ ] **Step 1: Write the failing tests**

`internal/sync/tomlkeys_test.go` (package `sync`, white-box like `jsonkeys_test.go`):
```go
package sync

import (
	"testing"

	"github.com/BurntSushi/toml"
)

func TestTOMLKeysSetGetDelete(t *testing.T) {
	// Line-level edits: a top-level key, a key in an existing table, a new table appended; the
	// user's other lines and comments are untouched; the result parses.
	// covers: R-SYNC-068, R-SYNC-086, R-MOD-050
	in := []byte("# mine\nmodel = \"gpt-6-luna\"\n\n[mcp_servers.x]\ncommand = \"npx\"\n")
	out, err := tomlSet(in, "model_auto_compact_token_limit", []byte("235000"))
	if err != nil {
		t.Fatal(err)
	}
	out, err = tomlSet(out, "agents.model", []byte(`"gpt-6-luna"`))
	if err != nil {
		t.Fatal(err)
	}
	out, err = tomlSet(out, "agents.model_reasoning_effort", []byte(`"low"`))
	if err != nil {
		t.Fatal(err)
	}
	want := "# mine\nmodel = \"gpt-6-luna\"\nmodel_auto_compact_token_limit = 235000\n\n[mcp_servers.x]\ncommand = \"npx\"\n\n[agents]\nmodel = \"gpt-6-luna\"\nmodel_reasoning_effort = \"low\"\n"
	if string(out) != want {
		t.Fatalf("got\n%s\nwant\n%s", out, want)
	}
	if v, ok, err := tomlGet(out, "agents.model"); err != nil || !ok || string(v) != `"gpt-6-luna"` {
		t.Fatalf("tomlGet = %q, %v, %v", v, ok, err)
	}
	if _, ok, _ := tomlGet(out, "agents.nope"); ok {
		t.Fatal("missing key found")
	}
	out, err = tomlSet(out, "agents.model", []byte(`"astra"`))
	if err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if _, err := toml.Decode(string(out), &m); err != nil {
		t.Fatalf("result does not parse: %v\n%s", err, out)
	}
	if m["agents"].(map[string]any)["model"] != "astra" {
		t.Fatalf("set did not replace: %s", out)
	}
	out, err = tomlDelete(out, "agents.model")
	if err != nil {
		t.Fatal(err)
	}
	if _, ok, _ := tomlGet(out, "agents.model"); ok {
		t.Fatalf("delete left the key: %s", out)
	}
	if _, _, err := tomlGet([]byte("a = = 1\n"), "a"); err == nil {
		t.Fatal("invalid TOML accepted")
	}
}

func FuzzTOMLKeys(f *testing.F) {
	// covers: R-QA-022
	f.Add([]byte("a = 1\n[t]\nb = 2\n"), "t.b", "3")
	f.Add([]byte(""), "agents.model", `"x"`)
	f.Add([]byte("[agents]\nmodel = \"y\"\n"), "agents.model", `"z"`)
	f.Fuzz(func(t *testing.T, data []byte, key, val string) {
		out, err := tomlSet(data, key, []byte(val))
		if err != nil {
			return
		}
		if v, ok, gerr := tomlGet(out, key); gerr != nil || !ok || string(v) != val {
			t.Fatalf("after set, get %q = %q, %v, %v", key, v, ok, gerr)
		}
		if _, err := tomlDelete(out, key); err != nil {
			t.Fatalf("delete after set: %v", err)
		}
	})
}
```
Append to `render_test.go`:
```go
func TestCodexEntriesOnlyForCodex(t *testing.T) {
	// covers: R-SYNC-068, R-SYNC-086, R-MOD-050
	root := openRoot(t)
	entries, err := render(t, root, "")
	if err != nil {
		t.Fatal(err)
	}
	if find(entries, ".codex/config.toml:agents.model") != nil {
		t.Fatal("codex entries rendered for the claude client")
	}
	entries, err = render(t, root, "client = \"codex\"\nexperimental = [\"codex\"]\n")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{
		".codex/config.toml:model_auto_compact_token_limit": "235000",
		".codex/config.toml:agents.model":                   `"gpt-6-luna"`,
		".codex/config.toml:agents.model_reasoning_effort":  `"low"`,
	}
	for id, content := range want {
		e := find(entries, id)
		if e == nil || string(e.Content) != content || e.Kind != nsync.KindKey {
			t.Fatalf("%s = %+v, want %q", id, e, content)
		}
	}
	for _, e := range entries {
		if e.Path == ".codex/config.toml" && want[e.ID] == "" {
			t.Fatalf("unexpected codex entry %s", e.ID)
		}
	}
}
```
(`openRoot`, `render` and `find` are the helpers `render_test.go` already has.)

`sync_codex.txtar`:
```
# With client = "codex" sync owns the compaction limit and the [agents] defaults in
# .codex/config.toml at the key level, recorded in the lock, keeping the user's other keys;
# the economy tier's codex row drives the agents defaults.
# covers: R-SYNC-068, R-SYNC-086, R-MOD-050
cd repo
exec ness init --gate true --base main --yes
cp ../ness.toml .ness/ness.toml
exec ness sync
stdout 'create \.codex/config\.toml:model_auto_compact_token_limit'
stdout 'adopt \.codex/config\.toml:agents\.model$'
grep 'model_auto_compact_token_limit = 235000' .codex/config.toml
grep '^\[agents\]' .codex/config.toml
grep 'model = "gpt-6-luna"' .codex/config.toml
grep 'model_reasoning_effort = "low"' .codex/config.toml
grep '^# keep me' .codex/config.toml
grep 'sandbox_mode = "workspace-write"' .codex/config.toml
! grep '\[agents\.' .codex/config.toml
grep -count=3 '"id": "\.codex/config\.toml:' .ness/lock.json
exec ness sync --check
# Changing the economy row follows into the file.
cp ../ness-terra.toml .ness/ness.toml
exec ness sync
grep 'model = "gpt-5.6-terra"' .codex/config.toml
grep 'model_reasoning_effort = "medium"' .codex/config.toml

-- ness.toml --
ness = "0.1"
project = "repo"
base = "main"
gate = "true"
client = "codex"
experimental = ["codex"]
-- ness-terra.toml --
ness = "0.1"
project = "repo"
base = "main"
gate = "true"
client = "codex"
experimental = ["codex"]
[tiers.economy]
codex = "gpt-5.6-terra/medium"
-- repo/.codex/config.toml --
# keep me
sandbox_mode = "workspace-write"

[agents]
model = "gpt-6-luna"
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/sync/ ./internal/cli/ -run 'TOML|Codex|TestScripts/sync_codex'`
Expected: FAIL — `tomlSet` undefined.

- [ ] **Step 3: Line-level TOML keys**

`internal/sync/tomlkeys.go`:
```go
package sync

import (
	"bytes"
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/BurntSushi/toml"
)

// TOML keys are edited line by line (OP-SYNC-10): a dotted key "t.k" is the line "k = v" under
// the header "[t]"; a plain key sits before the first header. Values are TOML literals. The
// result is parsed to prove it is still TOML. Arrays of tables and dotted keys inside a line
// are not handled: ness only writes the keys of R-SYNC-068.
// ponytail: headers must be exactly "[t]"; a quoted or spaced header is treated as another table.

var tomlKeyLine = regexp.MustCompile(`^\s*([A-Za-z0-9_-]+)\s*=`)

func splitKey(key string) (table, name string) {
	if i := strings.LastIndex(key, "."); i >= 0 {
		return key[:i], key[i+1:]
	}
	return "", key
}

func checkTOML(data []byte) error {
	var m map[string]any
	_, err := toml.Decode(string(data), &m)
	return err
}

// locate returns the line index of key's value within its table (-1 if absent) and the index
// where a new line for that table would go (len(lines) when the table is absent, -1 then
// signalled by tableFound=false).
func locate(lines []string, table, name string) (line, end int, tableFound bool) {
	cur := ""
	tableFound = table == ""
	end = len(lines)
	for i, l := range lines {
		t := strings.TrimSpace(l)
		if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") && !strings.HasPrefix(t, "[[") {
			if cur == table && tableFound {
				return -1, i, true
			}
			cur = strings.TrimSpace(t[1 : len(t)-1])
			if cur == table {
				tableFound = true
			}
			continue
		}
		if cur != table {
			continue
		}
		if m := tomlKeyLine.FindStringSubmatch(l); m != nil && m[1] == name {
			return i, i, true
		}
	}
	return -1, end, tableFound
}

func tomlGet(data []byte, key string) ([]byte, bool, error) {
	if err := checkTOML(data); err != nil {
		return nil, false, err
	}
	table, name := splitKey(key)
	lines := strings.Split(string(data), "\n")
	i, _, _ := locate(lines, table, name)
	if i < 0 {
		return nil, false, nil
	}
	_, v, _ := strings.Cut(lines[i], "=")
	v = strings.TrimSpace(v)
	if j := strings.Index(v, " #"); j > 0 && !strings.HasPrefix(v, `"`) {
		v = strings.TrimSpace(v[:j])
	}
	return []byte(v), true, nil
}

func tomlSet(data []byte, key string, v []byte) ([]byte, error) {
	if err := checkTOML(data); err != nil {
		return nil, err
	}
	table, name := splitKey(key)
	lines := strings.Split(strings.TrimRight(string(data), "\n"), "\n")
	if len(lines) == 1 && lines[0] == "" {
		lines = nil
	}
	i, end, found := locate(lines, table, name)
	line := name + " = " + string(v)
	switch {
	case i >= 0:
		lines[i] = line
	case found:
		// Insert before the next header, dropping trailing blank lines of the table first.
		for end > 0 && strings.TrimSpace(lines[end-1]) == "" {
			end--
		}
		lines = append(lines[:end], append([]string{line}, lines[end:]...)...)
	default:
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "["+table+"]", line)
	}
	out := []byte(strings.Join(lines, "\n") + "\n")
	if err := checkTOML(out); err != nil {
		return nil, fmt.Errorf("setting %s: %w", key, err)
	}
	return out, nil
}

func tomlDelete(data []byte, key string) ([]byte, error) {
	if err := checkTOML(data); err != nil {
		return nil, err
	}
	table, name := splitKey(key)
	lines := strings.Split(string(data), "\n")
	i, _, _ := locate(lines, table, name)
	if i < 0 {
		return data, nil
	}
	lines = append(lines[:i], lines[i+1:]...)
	out := []byte(strings.Join(lines, "\n"))
	if !bytes.HasSuffix(out, []byte("\n")) && len(out) > 0 {
		out = append(out, '\n')
	}
	return out, nil
}

var _ = errors.New
```
(Remove the `errors` import if unused.) In `engine.go`'s `opsFor`, before the block fallback:
```go
	if strings.HasSuffix(p, ".toml") {
		return partOps{get: tomlGet, set: tomlSet, remove: tomlDelete}
	}
```
In `render.go`, `coreEntries` appends `codexEntries(cfg)` when `cfg.Client == "codex"`:
```go
// codexEntries are the keys sync owns in .codex/config.toml for the codex client: the
// compaction limit (R-SYNC-086) and the [agents] defaults from the economy tier (R-MOD-050),
// never a named agent (R-SYNC-068).
func codexEntries(cfg *config.Config) []Entry {
	model, effort := config.SplitTier(cfg.Tiers["economy"].Codex)
	const p = ".codex/config.toml"
	return []Entry{
		{Path: p, Kind: KindKey, Owner: coreOwner, Name: "model_auto_compact_token_limit", Content: []byte("235000")},
		{Path: p, Kind: KindKey, Owner: coreOwner, Name: "agents.model", Content: []byte(strconv.Quote(model))},
		{Path: p, Kind: KindKey, Owner: coreOwner, Name: "agents.model_reasoning_effort", Content: []byte(strconv.Quote(effort))},
	}
}
```
The lock id of a key entry is `<path>:<name>` (P1), so these are `.codex/config.toml:agents.model` etc. The engine's `planParts` indents a fresh JSON file only; a fresh TOML file comes out of `tomlSet` as written.

- [ ] **Step 4: Run and commit**

Run: `go test ./internal/sync/ ./internal/cli/ && go test ./internal/sync/ -run '^$' -fuzz FuzzTOMLKeys -fuzztime 10s && make lint`
Expected: PASS.

```bash
git add internal/sync internal/cli/testdata/script/sync_codex.txtar
git commit -m "feat(sync): own the Codex compaction limit and agents defaults in .codex/config.toml"
```

---

### Task 8: The e2e runner and the budget rule

**Files:**
- Create: `internal/testkit/budget.go`, `internal/testkit/budget_test.go`, `internal/cli/e2e_test.go`
- Modify: `Makefile` (`e2e` target), `README.md` (how to run it)

**Interfaces:**
- Produces: `testkit.UsageGuard(path string, limit int) error` (reads the Claude Code statusline snapshot; errors when `rate_limits.seven_day.used_percentage >= limit`, when the file is missing or unreadable), `testkit.RequireBudget(t *testing.T)` (skips with the reason unless `NESS_E2E=1`; fails fast on the guard), `make e2e`.

**Checks carried from the P3a final review** (verify in the real launch, record each result in the report):
- `herdr agent start` with a prompt argument: does herdr report the agent ready while Claude is already working on the prompt, or only once it is idle? P3a's `startAgent` already continues when the start errors but the ready mark exists or `herdr agent list` shows the name (logged as `agent … is up, continuing`). If the real start returns on first detection, nothing more is needed; if it times out on a long first turn, the log line shows it and the 120 s `--timeout` is the knob.
- Codex trust: P3a passes `-c projects={"<worktree>"={trust_level="trusted"}}` (top-level key, inline-table value) because the dotted form `projects."<worktree>".trust_level` breaks if Codex splits `-c` keys on every dot (every worktree path has `.worktrees`). Verify that a real Codex launch shows no trust dialog (R-SESS-113), and that the bare `projects` override does not cost trust that Codex looks up elsewhere (for example the primary checkout's entry, if Codex resolves a worktree to its main repository); if it does, add that entry to the same inline table.
- The `hooks.SessionStart` / `hooks.PreToolUse` `-c` overrides reach Codex's hook loader (the ruling's open point).

- [ ] **Step 1: Write the failing test**

`internal/testkit/budget_test.go`:
```go
package testkit_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestUsageGuardStopsAtEightyPercent(t *testing.T) {
	// covers: R-QA-027, R-QA-042
	dir := t.TempDir()
	write := func(pct int) string {
		p := filepath.Join(dir, "usage-latest.json")
		body := `{"rate_limits":{"five_hour":{"used_percentage":6},"seven_day":{"used_percentage":` + strconv.Itoa(pct) + `}}}`
		if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
		return p
	}
	if err := testkit.UsageGuard(write(79), 80); err != nil {
		t.Fatalf("79%%: %v", err)
	}
	if err := testkit.UsageGuard(write(80), 80); err == nil || !strings.Contains(err.Error(), "80%") {
		t.Fatalf("80%%: %v", err)
	}
	if err := testkit.UsageGuard(filepath.Join(dir, "missing.json"), 80); err == nil {
		t.Fatal("a missing usage file passed the guard")
	}
}
```
(import `strconv`.)

- [ ] **Step 2: The guard**

`internal/testkit/budget.go`:
```go
package testkit

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// UsageGuard is the D31 stop rule (R-QA-027, R-QA-042): the real-client runs abort when the
// client's usage snapshot shows weekly usage at or above limit percent. The snapshot is the
// statusline input Claude Code last wrote (~/.claude/usage-latest.json). No snapshot, no run.
func UsageGuard(path string, limit int) error {
	data, err := os.ReadFile(path) //nolint:gosec // the user's own usage snapshot
	if err != nil {
		return fmt.Errorf("no usage data at %s (run one Claude Code session first): %w", path, err)
	}
	var u struct {
		RateLimits struct {
			SevenDay struct {
				UsedPercentage int `json:"used_percentage"`
			} `json:"seven_day"`
		} `json:"rate_limits"`
	}
	if err := json.Unmarshal(data, &u); err != nil {
		return fmt.Errorf("usage data at %s: %w", path, err)
	}
	if u.RateLimits.SevenDay.UsedPercentage >= limit {
		return fmt.Errorf("weekly usage is at %d%% (stop rule: %d%%); the real-client runs wait for the reset",
			u.RateLimits.SevenDay.UsedPercentage, limit)
	}
	return nil
}

// RequireBudget gates a real-client test: it runs only with NESS_E2E=1 and under the usage
// guard, so a default `go test` and CI never spend budget.
func RequireBudget(t *testing.T) {
	t.Helper()
	if os.Getenv("NESS_E2E") != "1" {
		t.Skip("real-client run: set NESS_E2E=1")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := UsageGuard(filepath.Join(home, ".claude", "usage-latest.json"), 80); err != nil {
		t.Fatalf("budget: %v", err)
	}
}
```

- [ ] **Step 3: The real-client tests**

`internal/cli/e2e_test.go`:
```go
//go:build e2e

package cli_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

// e2eRepo makes a throwaway project with ness init and one commit; the binary is the one on
// PATH (build it first: make build && go install ./cmd/ness, or mise).
func e2eRepo(t *testing.T) string {
	t.Helper()
	if os.Getenv("HERDR_ENV") != "1" {
		t.Skip("not inside herdr")
	}
	dir := t.TempDir()
	run := func(args ...string) string {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%v: %v\n%s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	run("git", "init", "-q", "-b", "main")
	run("git", "commit", "-q", "--allow-empty", "-m", "init")
	run("ness", "init", "--gate", "true", "--base", "main", "--yes")
	run("git", "add", "-A")
	run("git", "commit", "-q", "-m", "ness init")
	brief := "Reply with the single word done and end your turn. Do not edit files.\n"
	if err := os.WriteFile(filepath.Join(dir, "brief.md"), []byte(brief), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestRealLaunchReachesReadyAndBindsTheTranscript(t *testing.T) {
	// One economy session: the ready mark arrives, the transcript file is named by the stored
	// client session id (R-SESS-027 real half), the runtime and client contracts hold for real.
	// covers: R-SESS-027, R-SESS-111
	testkit.RequireBudget(t)
	dir := e2eRepo(t)
	cmd := exec.CommandContext(t.Context(), "ness", "open", "--root", "--scope", "e2e", "--tier", "economy", "--brief", "brief.md")
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("ness open: %v\n%s", err, out)
	}
	id := strings.Fields(string(out))[1]
	st := exec.CommandContext(t.Context(), "ness", "status", "--json")
	st.Dir = dir
	js, err := st.Output()
	if err != nil {
		t.Fatal(err)
	}
	cs := between(string(js), `"client_session": "`, `"`)
	home, _ := os.UserHomeDir()
	wt := filepath.Join(dir, ".worktrees")
	entries, _ := os.ReadDir(wt)
	transcript := filepath.Join(claude.ProjectDir(home, filepath.Join(wt, entries[0].Name())), cs+".jsonl")
	if _, err := os.Stat(transcript); err != nil {
		t.Fatalf("no transcript at %s: %v", transcript, err)
	}
	t.Cleanup(func() {
		c := exec.CommandContext(t.Context(), "ness", "close", id, "--discard", "--note", "e2e")
		c.Dir = dir
		_, _ = c.CombinedOutput()
	})
}

func TestTwentyParallelLaunches(t *testing.T) {
	// L6 exit: NESS_E2E_PARALLEL launches (default 2; 20 for the real check) all reach ready.
	// covers: R-SESS-112
	testkit.RequireBudget(t)
	n, _ := strconv.Atoi(os.Getenv("NESS_E2E_PARALLEL"))
	if n == 0 {
		n = 2
	}
	dir := e2eRepo(t)
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			cmd := exec.CommandContext(t.Context(), "ness", "open", "--root", "--scope", "p"+strconv.Itoa(i), "--tier", "economy", "--brief", "brief.md")
			cmd.Dir = dir
			if out, err := cmd.CombinedOutput(); err != nil {
				errs <- fmt.Errorf("launch %d: %v\n%s", i, err, out)
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	st := exec.CommandContext(t.Context(), "ness", "status")
	st.Dir = dir
	out, _ := st.Output()
	if strings.Contains(string(out), "stalled") {
		t.Fatalf("stalled launches:\n%s", out)
	}
	_ = time.Second
}

func TestRealCodexLaunch(t *testing.T) {
	// The Codex atom's trust, update and hook flags keep the launch free of dialogs (R-SESS-113
	// real half, R-HOOK-018 real half). Needs codex on PATH and a login.
	// covers: R-SESS-113
	testkit.RequireBudget(t)
	if _, err := exec.LookPath("codex"); err != nil {
		t.Skip("codex not installed")
	}
	dir := e2eRepo(t)
	toml := filepath.Join(dir, ".ness", "ness.toml")
	data, _ := os.ReadFile(toml)
	if err := os.WriteFile(toml, append(data, []byte("experimental = [\"codex\"]\n")...), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.CommandContext(t.Context(), "ness", "open", "--root", "--scope", "cx", "--tier", "economy", "--client", "codex", "--brief", "brief.md")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("codex launch: %v\n%s", err, out)
	}
}

func between(s, a, b string) string {
	_, rest, ok := strings.Cut(s, a)
	if !ok {
		return ""
	}
	v, _, _ := strings.Cut(rest, b)
	return v
}
```
(add `fmt` to the imports.) `Makefile`:
```make
e2e:
	NESS_E2E=1 go test -tags e2e -run 'Real|Parallel' ./internal/cli/ ./internal/atom/herdr/ -v -count=1
```
and `.PHONY` gains `e2e`. README, under Status: "Real-client runs: `make e2e` inside a herdr session; they skip without `NESS_E2E=1` and abort at 80% weekly usage (D31)."

- [ ] **Step 4: Run and commit**

Run: `go test ./internal/testkit/ && go vet -tags e2e ./internal/cli/ && make lint`
Expected: PASS; without the tag `go test ./...` compiles no e2e test (R-QA-027). Then, if the weekly usage is below 80% (`python3 -c "import json;print(json.load(open('$HOME/.claude/usage-latest.json'))['rate_limits']['seven_day'])"`), run `make e2e` once with the default parallelism and record the result for the report; the 20-launch run (`NESS_E2E_PARALLEL=20 make e2e`) is the operator's call under D31. At the time of writing the weekly usage is 76%: expect the guard to stop the run until the reset, and say so in the report.

```bash
git add internal/testkit/budget.go internal/testkit/budget_test.go internal/cli/e2e_test.go Makefile README.md
git commit -m "test(e2e): real-client runs behind the e2e tag and the weekly-usage stop rule"
```

---

### Task 9: Coverage gate, status line, wrap-up

**Files:**
- Modify: `internal/testkit/coverage_repo_test.go`, `README.md`

- [ ] **Step 1: SESS and DOC join the gate**

Replace the two variables in `internal/testkit/coverage_repo_test.go`:
```go
// doneAreas are the spec areas whose plans have landed (P1, P2, P3).
var doneAreas = []string{"CFG", "SYNC", "PROF", "QA", "MSG", "HOOK", "MOD", "SESS", "DOC"}

// deferred names ids of done areas that a later plan, a review entry or publication delivers.
var deferred = map[string]string{
	"R-MSG-005": "P4", "R-MSG-042": "P4", "R-MOD-012": "P4 (worker half)", "R-SESS-009": "P4",
	"R-CFG-030": "P5", "R-CFG-031": "P5", "R-CFG-054": "P5", "R-PROF-015": "P5", "R-HOOK-010": "P5",
	"R-SESS-070": "P5", "R-SESS-081": "P5", "R-SESS-083": "P5", "R-DOC-015": "P5",
	"R-CFG-032": "P6", "R-CFG-034": "P6", "R-CFG-035": "P6", "R-CFG-037": "P6", "R-CFG-051": "P6",
	"R-SYNC-032": "P6", "R-SYNC-033": "P6", "R-SYNC-089": "P6", "R-PROF-002": "P6",
	"R-QA-004": "P6", "R-QA-005": "P6", "R-QA-023": "P6", "R-QA-030": "P6", "R-QA-031": "P6", "R-QA-041": "P6",
	"R-MOD-002": "P6", "R-MOD-021": "P6",
	"R-QA-032": "publication", "R-QA-033": "publication",
	"R-QA-025": "withdrawn (D39)",
}
```
R-SYNC-068, R-SYNC-086, R-MOD-050 (Task 7), R-QA-024, R-QA-027, R-QA-042 (Tasks 1, 2, 8) leave the map. The e2e tests carry R-SESS-027, R-SESS-112, R-SESS-113 and R-SESS-111 markers; their fake halves are covered by P3a's tests too.

Run: `go test ./internal/testkit/ -run TestRequirementCoverage`
Expected: PASS. An uncovered SESS or DOC id is a real gap: find the task that delivers it, add the test with its marker, rerun. Defer only to a named later plan.

- [ ] **Step 2: README status**

Replace the status line with:
`Status: P3 (sessions and atoms) implemented: ness open, close, status, integrate, doctor, the herdr/Claude Code/Codex atoms and the merge/squash/pr integrators on top of P2. Plans: docs/superpowers/specs/2026-10-08-ness-v1-scope.md.`

- [ ] **Step 3: The full gate and the dogfood check**

Run: `make validate && make fuzz FUZZTIME=10s`
Expected: PASS end to end (`FuzzTOMLKeys` included). Then in this repository: `go run ./cmd/ness doctor` — expected: the version line, `ok`/`warn` lines (the worker is not installed here: `warn worker-binary`, `fail worker` is expected until P4 — note it), exit 1 only because of `worker`; `git status` clean (doctor wrote nothing).

- [ ] **Step 4: Commit and push, check CI**

```bash
git add -A
git commit -m "build: SESS and DOC join the requirement coverage gate; P3 status"
git push
gh run watch --exit-status
```
Expected: green.

---

## End-of-plan review

After Task 9, launch one fresh reviewer on the most capable model with: `docs/spec/sessions.md` (close, integrate), `docs/spec/doctor.md`, `docs/spec/models.md` (R-MOD-050), this plan and P3a's, and `git diff <first P3b commit>^..HEAD`. The reviewer checks:
1. Each covered requirement's test would fail without its behaviour (spot-check: run the gate in the session's worktree instead of the throwaway, skip `Retire` in close, drop the `--force` on the throwaway removal, let doctor stop at the first failing check).
2. The Review Focus items of both plans hold.
3. The rulings of both plans; in particular that doctor never writes (grep `WriteFile`, `Exec(` with mutating git verbs, `CloseTab` in `doctor*.go`) and that `integrate` never calls `post`, probes or environments (R-SESS-100).
4. The e2e evidence: the recorded `make e2e` run, or the budget stop (D31) with the usage percentage at the time.

Then fix each finding, rerun `make validate`, and report to the operator: what shipped, the evidence (test counts, fuzz time, the e2e run or its deferral, CI run link), and what P4 takes over (R-SESS-009, the worker half of R-MOD-012, `ness worker` itself).
