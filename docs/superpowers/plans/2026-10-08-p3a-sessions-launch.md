> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# P3a Sessions and Launch Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** `ness open` launches a root or a child as a worktree, a branch, a brief and a herdr tab running Claude Code (or Codex) behind the environment checks of design §4, persists each launch phase, waits for the ready mark and resumes stalled or exited sessions; `ness status` prints the tree; the hooks learn the phase, the shadowing check and the adopted root. The atoms (herdr, Claude Code, Codex, the text model policy) sit behind seams with fakes. Close, integrate and doctor are P3b.

**Architecture:** Every external program runs through one injected `Exec` function (`cli.Deps.Exec`), so the real atoms are exercised end to end in scripts against `internal/testkit`'s fake exec: a stateful fake herdr that answers the real CLI's JSON, a table for `claude`, `codex`, `gh` and `systemctl`, and real git. `internal/session` owns the seams (`Runtime`, `Client`, `Policy`), the environment checks, the launch state machine and the phase names; `internal/atom/{herdr,claude,codex,textpolicy}` implement them; `internal/store` gains the launch columns in an appended migration. `internal/cli` stays thin: `open.go`, `status.go`, a phase on the hook's identity line.

**Tech Stack:** Go 1.25+ (local 1.27.1), `modernc.org/sqlite`, cobra, testscript, `testing/synctest` for the ready timeout. herdr 0.8.2 (`herdr tab create`, `herdr agent start … -- ARGS`, `herdr agent list`, `herdr pane read`), Claude Code 2.1.294 (`--session-id`, `--model`, `--effort`, `--fallback-model`, `--resume`), Codex 0.157.1 (`-m`, `-c key=value`, `codex debug models`). The P1/P2 gate stays as is.

**Spec:** `docs/superpowers/specs/2026-10-08-ness-v1-scope.md` (split), `docs/design/ness-design.md` §2 (seams), §3, §4, §5 (Sessions), §6, §12, §16 (#7, #10), and the requirements in `docs/spec/sessions.md` (SESS: environment checks, open, resume, status, runtime and client contracts), `docs/spec/models.md` (MOD), with the carry-ins from `docs/spec/hooks.md` (R-HOOK-004, 005, 008, 009, 018) and `docs/spec/core-config.md` (R-CFG-041).

## Global Constraints

- Module `github.com/enea-scaccabarozzi/ness`, one binary, `CGO_ENABLED=0` (R-QA-001). No new dependency: uuids are 16 random bytes formatted by hand; herdr JSON is parsed with `encoding/json`.
- Exit codes: 0 ok, 1 a check found problems, 2 usage, 3 sync conflict, 4 refused (R-CFG-035). A refusal writes nothing: no file, store row, worktree, branch or tab (R-CFG-036, R-SESS-010).
- v1 is Linux only: with `Deps.GOOS` not `linux`, every command except `--version` and `--help` refuses (R-SESS-015).
- Identity: `NESS_SESSION=<id>` makes the process that session, `NESS_RUN=<n>` names the run; unset means the operator (R-SESS-037). The launch sets both for the client. Only the composition roots read them (R-CFG-002); the atoms may read what the runtime and client define (`HERDR_PANE_ID`, `HERDR_TAB_ID`, `CLAUDE_CODE_SESSION_ID`).
- Session ids are `s-` plus 12 lowercase hex characters (R-SESS-042). Scopes match `^[a-z][a-z0-9-]{0,31}$` and `--title` defaults to the scope (R-SESS-043).
- Worktree `.worktrees/<scope>-<id>/` under the **primary checkout**, branch `session/<scope>-<id>` (R-SESS-020). Record dir `<sessions>/<root scope>/[<child scope>/…]` inside the session's checkout; `sessions` defaults to `docs/sessions` (R-CFG-041, R-SESS-022).
- The prompt argument is exactly `You are ness session <ID> (<scope>). Read <record>/brief.md, then load ness-entry.` on one line; nothing is typed into the pane during a launch (R-SESS-025, R-SESS-026).
- Effort goes on the client command line, never through `CLAUDE_CODE_EFFORT_LEVEL` (R-SESS-028, R-MOD-010). `--model` does not exist (R-MOD-013). Claude efforts `low|medium|high|xhigh|max`; Codex efforts `minimal|low|medium|high|xhigh` (R-SESS-017, R-MOD-033).
- Default tiers (R-MOD-001): standard `opus/medium` · `gpt-6.1-sol/medium`; frontier `fable/medium` · `astra/medium`; economy `sonnet/medium` · `gpt-6-luna/low`. Effort is always passed explicitly, frontier included (R-MOD-004).
- Tested versions are constants in the atoms: herdr `0.8.2` (`session.TestedHerdr`), Claude Code `2.1.294` (`claude.Tested`), Codex `0.157.1` (`codex.Tested`) (R-SESS-016). herdr and the client must be at least the tested version (same major).
- herdr agent name `<project>-<scope>` lowercased, `[a-z0-9_-]`, at most 32 characters, unique against live agents (R-SESS-035, R-SESS-036).
- `launch.ready_timeout` defaults to 60 s (R-SESS-033). The launch polls the store for the ready mark every 250 ms.
- Launch phases are frozen (R-SESS-044): `registered`, `creating-worktree`, `linking-files`, `writing-brief`, `opening-tab`, `starting-agent`, `sending-brief`, `waiting-ready`, `ready`.
- Every git subprocess ness starts runs with the repository-discovery variables stripped from its environment (`GIT_DIR`, `GIT_WORK_TREE`, `GIT_COMMON_DIR`, `GIT_INDEX_FILE`, `GIT_OBJECT_DIRECTORY`, `GIT_ALTERNATE_OBJECT_DIRECTORIES`, `GIT_NAMESPACE`, `GIT_CEILING_DIRECTORIES`, `GIT_DISCOVERY_ACROSS_FILESYSTEM`); `GIT_AUTHOR_*`, `GIT_SSH_COMMAND` and the rest pass through.
- Output goes to the injected writers in `cli.Deps` (R-CFG-033). Timestamps are RFC 3339 UTC; commands take the clock once from `Deps.Now()`.
- Every test names the requirement ids it covers: `// covers: R-…` in Go tests and `# covers: R-…` in scripts.
- Conventional commits. Each commit ends with the session's Co-Authored-By/Claude-Session lines.

## Review Focus

1. **`ness open` run from inside a session's worktree (a manager opening a child).** The new worktree must land under the primary checkout's `.worktrees/`, never nested under the manager's worktree, and both must share one store. Pinned in Task 7 (`open_nested.txtar`).
2. **The ready mark arriving before the launch reaches `waiting-ready`** (`herdr agent start` returns only once the client is up, by which time the SessionStart hook has already run). The launch must complete and the stored phase must never regress or be overwritten by the hook. Pinned in Task 6 (`TestLaunchReadyBeforeWaiting`) and Task 8 (`hook_session_start.txtar`).
3. **`--resume` of a stalled launch whose tab was closed out of band.** The launch must open a new tab and bind to it, not fail on the dead tab id and not leave two tabs. Pinned in Task 6 (`TestResumeAfterTabGone`).
4. **A second child with the same `--scope` under the same manager.** The record directory and the herdr label would collide; open must refuse before any side effect and name the open sibling. Pinned in Task 7 (`open_refusals.txtar`).
5. **`herdr` answering with an error envelope on stderr (exit 1) or non-JSON output.** The atom must report herdr's message, never a decode panic or an empty tab id. Pinned in Task 3 (`TestHerdrErrors`).

## Deferred to later plans

The coverage test (Task 10) lists these so the gate stays green while the gap stays visible:

- P3b (close, integrate, doctor): R-SESS-060..072, R-SESS-090..105, R-SESS-071 and R-SESS-096 (pr integrator), every R-DOC id, R-SYNC-068, R-SYNC-086, R-MOD-050 (Codex sync keys), R-QA-024, R-QA-027, R-QA-042 (e2e runner), and the real-client halves of R-SESS-027, R-SESS-111, R-SESS-112, R-SESS-113 (e2e tag).
- P4 (worker): R-SESS-009 (model unavailable mid-run; `--fallback-model` is covered here as R-MOD-044), the worker half of R-MOD-012.
- P5 (features): R-SESS-070 (probe ownership on close), R-SESS-081, R-SESS-083 (probes in status).
- P6 (text): R-MOD-002, R-MOD-021 (generated skill text).
- Review entry in `docs/spec/reviews.md`, written by the end-of-plan reviewer: R-SESS-014 (the enterprise skills path spike, Task 4).
- The real-client half of R-HOOK-014 (a blocked Edit) belongs to P5's contract guard: no P2/P3 guard blocks an Edit.

Ids this plan removes from P2's deferred list: R-CFG-041 (Task 7), R-HOOK-005, R-HOOK-008 (Task 8), R-HOOK-018 (Task 4).

## Rulings

Where the spec is silent or the real tools differ from it, this plan decides; the end-of-plan reviewer checks each one.

- Every external program goes through one `Exec` seam; scripts run the real atoms over a fake herdr that speaks the real CLI's JSON. The "fake runtime" of R-SESS-110 is the herdr atom over that fake; the real run is opt-in (`HERDR_ENV=1`, tag `e2e`).
- Claude Code has no dry model resolver, so the claude atom accepts the four aliases and any `claude-*` id and the launch itself is the real check; Codex ids are checked against `codex debug models`.
- The client version check is "same major, at least the tested minor": Claude Code auto-updates weekly and an exact pin would refuse every Monday.
- Codex hooks go on the command line as `--enable codex_hooks --dangerously-bypass-hook-trust -c hooks.SessionStart=[…] -c hooks.PreToolUse=[…]` (mirroring the `~/.codex/hooks.json` shape), trust as `-c projects."<dir>".trust_level="trusted"`, updates as `-c check_for_update_on_startup=false`; the real Codex launch (P3b e2e) confirms them. Codex has no `--session-id`: the atom records no client session and `--resume` of an exited Codex session is refused as unsupported.
- The record directory is named by scope, nested under the parent's; a second open sibling with the same scope is refused.
- The session-start hook never writes the phase; it writes the ready mark only. The displayed phase is derived: `ready` once the mark exists, else the stored phase, shown as `stalled launch` when the mark is missing and the phase is older than `launch.ready_timeout`.
- A `worktree.copy`/`worktree.link` source missing in the primary checkout is skipped with a line on stderr, not a refusal (gitignored data may not exist yet).
- Git discovery variables are stripped from every subprocess (P2 hardening carry-in); the rest of `GIT_*` passes through.
- The polling guard also blocks Bash `for` loops and `watch` around `ness inbox`/`ness status` (P2 carry-in).
- `--here` binds to `$HERDR_PANE_ID`/`$HERDR_TAB_ID` and `$CLAUDE_CODE_SESSION_ID` and refuses when any is unset; it does not rename the running herdr agent.

## File map

```
cmd/ness/main.go                              Deps.Exec = cli.SystemExec, Deps.GOOS = runtime.GOOS
internal/cli/cli.go                           Deps.Exec, Deps.GOOS, Linux-only guard, register open/status
internal/cli/exec.go                          SystemExec: os/exec with the git discovery vars stripped
internal/cli/project.go                       projectConfig (profiles + defaults), primaryRoot
internal/cli/atoms.go                         buildRuntime, buildClient, skillRoots (composition inside cli)
internal/cli/open.go                          ness open: flags, checks, launch, --here, --resume
internal/cli/status.go                        ness status, --all, --json
internal/cli/hook.go                          phase on the identity line, shadowing check, for/watch guard
internal/cli/os_test.go                       Linux-only guard (white-box)
internal/cli/testdata/script/open.txtar, open_refusals.txtar, open_nested.txtar, open_here.txtar,
  open_resume.txtar, open_env.txtar, status.txtar, hook_shadowing.txtar; hook_session_start.txtar and
  hook_pretool.txtar extended
internal/config/config.go                     effort lists come from the client atoms
internal/store/store.go                       CleanGitEnv; Locate uses it
internal/store/migrate.go                     migration 2 (launch columns, items.retired_at, heartbeat)
internal/store/sessions.go                    Session fields, scanSession, Sessions, Children
internal/store/launch.go                      Launch, AddLaunch, SetPhase, BindTab, BindAgent, NewRun, SetTier,
                                              SetIntegration, RecordClose, Retire, Heartbeat, SetHeartbeat
internal/store/items.go                       listed/obliged ignore retired items
internal/store/launch_test.go
internal/session/runtime.go                   Runtime, Tab, TabSpec, AgentSpec, PaneState, ErrNotInstalled
internal/session/client.go                    Client, LaunchSpec, Transcript
internal/session/policy.go                    Policy, Brief, Choice, ErrNeedTier
internal/session/phases.go                    Phases, Shown (derived phase), NewID, NewUUID, Prompt
internal/session/checks.go                    Env.Check, CheckOS, CheckVersion, CheckShadowing, SkillRoots, TestedHerdr
internal/session/names.go                     AgentName
internal/session/launch.go                    Launcher, Request, Run, steps, waitFor
internal/session/*_test.go
internal/atom/herdr/herdr.go                  Runtime over Exec: tab create/close/list, agent start/list/get, pane read/send-text/run
internal/atom/herdr/herdr_test.go, e2e_test.go
internal/atom/claude/claude.go                Tested, Efforts, EnterpriseSkills, Client: version, models, args, transcript
internal/atom/codex/codex.go                  Tested, Efforts, Client: version, debug models, args with -c overrides
internal/atom/textpolicy/textpolicy.go        the text policy
internal/testkit/exec.go                      FakeExec, `fake` script command, default outputs
internal/testkit/herdr.go                     the stateful fake herdr CLI
internal/testkit/contracts.go                 RuntimeContract, ClientContract, PolicyContract, FakeRouter
internal/testkit/store.go                     store: phase, bind, integrate, heartbeat, sessions, more session fields
internal/testkit/testkit.go                   Setup writes the default fakes dir; register `fake`
internal/testkit/coverage_repo_test.go        MOD joins doneAreas; deferred map updated
docs/design/decisions.md                      D40: enterprise skills path (Task 4)
README.md                                     status line
```

---

### Task 1: Store: migration 2, launch columns, runs, heartbeat, retired items

**Files:**
- Create: `internal/store/launch.go`, `internal/store/launch_test.go`
- Modify: `internal/store/migrate.go` (append), `internal/store/sessions.go` (Session fields, scan, lists), `internal/store/items.go` (`retired_at`), `internal/store/store.go` (`CleanGitEnv`), `internal/testkit/store.go` (new subcommands and fields)

**Interfaces:**
- Consumes: `store.Open`, `inTx`, `stamp`, `nullStamp`, `nullable`, `touch` from P2.
- Produces:
  - `store.PhaseRegistered = "registered"`, `store.PhaseReady = "ready"`.
  - `type Session struct { ID, Parent, Scope, Title, Tier, Effort, Client, Phase string; Run int; Pane, Tab, Agent, ClientSession, Worktree, Branch, Record, Base, RouterReason, Integrated string; Adopted, Closed bool; CreatedAt, PhaseAt, ReadyAt, IntegratedAt, ClosedAt, BoundaryAt time.Time; BoundaryNote, CloseNote, KeepReason string }`.
  - `type Launch struct { ID, Parent, Scope, Title, Tier, Effort, Client, Record, Base, Worktree, Branch, RouterReason string; Adopted bool }`.
  - `(*Store).AddLaunch(l Launch, now time.Time) error`, `SetPhase(id, phase string, now time.Time) error`, `BindTab(id, pane, tab string, run int) error` (clears agent and client session), `BindAgent(id, agent, clientSession string) error`, `NewRun(id, phase, tier, effort string) error` (clears pane/tab/agent/client session and the ready mark; empty tier keeps the old one), `SetTier(id, tier, effort string) error`, `Sessions(all bool) ([]Session, error)` (created order; `all` includes closed), `Children(parent string) ([]Session, error)` (open only), `SetIntegration(id, ref string, now time.Time) error`, `RecordClose(id, note, keep string, now time.Time) error`, `Retire(recipient string, now time.Time) (int, error)`, `Heartbeat() (Heartbeat, error)` (`ErrNoHeartbeat`), `SetHeartbeat(now time.Time, version string) error`.
  - `store.CleanGitEnv(env []string) []string`.
  - Script: `store phase ID PHASE`, `store bind ID PANE TAB AGENT`, `store integrate ID REF`, `store heartbeat AGE_SECONDS VERSION`, `store sessions N`, and `store session ID` fields `phase`, `tier`, `effort`, `run`, `pane`, `tab`, `agent`, `client_session`, `worktree`, `branch`, `record`, `integrated`, `title`, `adopted`.

- [ ] **Step 1: Write the failing tests**

`internal/store/launch_test.go`:
```go
package store_test

import (
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestMigrationTwoAddsLaunchColumns(t *testing.T) {
	// A P2 store (version 1) opens, migrates forward and serves the new fields.
	// covers: R-MSG-046
	path := freshPath(t)
	s := mustOpen(t, path)
	if err := s.MigrateWith(store.Migrations[:1]); err != nil {
		t.Fatal(err)
	}
	if err := s.AddSession(sA, "root", store.Operator, now); err != nil {
		t.Fatal(err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	s = mustOpen(t, path)
	if got := userVersion(t, path); got != 2 {
		t.Fatalf("user_version = %d, want 2", got)
	}
	sess, err := s.Session(sA)
	if err != nil || sess.Phase != store.PhaseReady || sess.Client != "claude" {
		t.Fatalf("legacy row = %+v, %v", sess, err)
	}
}

func TestLaunchPhasesAndRuns(t *testing.T) {
	// covers: R-SESS-031, R-SESS-048, R-SESS-051
	s := mustOpen(t, freshPath(t))
	l := store.Launch{ID: sA, Parent: store.Operator, Scope: "root", Title: "root", Tier: "standard",
		Effort: "medium", Client: "claude", Record: "docs/sessions/root", Base: "main",
		Worktree: ".worktrees/root-aaaaaaaaaaaa", Branch: "session/root-aaaaaaaaaaaa", RouterReason: "explicit --tier"}
	if err := s.AddLaunch(l, now); err != nil {
		t.Fatal(err)
	}
	sess, _ := s.Session(sA)
	if sess.Phase != store.PhaseRegistered || !sess.PhaseAt.Equal(now) || sess.Run != 0 || sess.RouterReason != "explicit --tier" {
		t.Fatalf("after AddLaunch: %+v", sess)
	}
	if err := s.SetPhase(sA, "opening-tab", now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	if err := s.BindTab(sA, "w1:p2", "w1:t2", 1); err != nil {
		t.Fatal(err)
	}
	if err := s.BindAgent(sA, "p-root", "c0ffee"); err != nil {
		t.Fatal(err)
	}
	sess, _ = s.Session(sA)
	if sess.Phase != "opening-tab" || sess.Pane != "w1:p2" || sess.Tab != "w1:t2" || sess.Run != 1 ||
		sess.Agent != "p-root" || sess.ClientSession != "c0ffee" {
		t.Fatalf("bound: %+v", sess)
	}
	if err := s.MarkReady(sA, now); err != nil {
		t.Fatal(err)
	}
	// A new run clears the binding and the ready mark and may change the tier (R-SESS-048, R-SESS-051).
	if err := s.NewRun(sA, "opening-tab", "frontier", "medium"); err != nil {
		t.Fatal(err)
	}
	sess, _ = s.Session(sA)
	if sess.Pane != "" || sess.Tab != "" || sess.Agent != "" || sess.ClientSession != "" || !sess.ReadyAt.IsZero() ||
		sess.Tier != "frontier" || sess.Phase != "opening-tab" {
		t.Fatalf("new run: %+v", sess)
	}
	if err := s.BindTab(sA, "w1:p9", "w1:t9", 2); err != nil {
		t.Fatal(err)
	}
	sess, _ = s.Session(sA)
	if sess.Run != 2 || sess.Pane != "w1:p9" {
		t.Fatalf("second run: %+v", sess)
	}
	if err := s.SetPhase("s-nope", "ready", now); !errors.Is(err, store.ErrNoSession) {
		t.Fatalf("SetPhase unknown: %v", err)
	}
}

func TestAdoptedRootIsReadyAtOnce(t *testing.T) {
	// covers: R-SESS-038
	s := mustOpen(t, freshPath(t))
	if err := s.AddLaunch(store.Launch{ID: sA, Scope: "root", Client: "claude", Adopted: true, Base: "main"}, now); err != nil {
		t.Fatal(err)
	}
	sess, _ := s.Session(sA)
	if !sess.Adopted || sess.Phase != store.PhaseReady || !sess.ReadyAt.Equal(now) || sess.Worktree != "" {
		t.Fatalf("adopted: %+v", sess)
	}
}

func TestSessionsChildrenCloseAndRetire(t *testing.T) {
	// covers: R-SESS-065, R-SESS-069, R-SESS-086, R-SESS-099
	s := tree(t) // sA root, sB child of sA (items_test.go)
	if err := s.AddSession("s-cccccccccccc", "child-c", sA, now.Add(time.Second)); err != nil {
		t.Fatal(err)
	}
	kids, err := s.Children(sA)
	if err != nil || len(kids) != 2 || kids[0].ID != sB {
		t.Fatalf("Children = %v, %v", kids, err)
	}
	if _, _, err := s.Send(sA, sB, store.IntentRequest, "pending", false, now); err != nil {
		t.Fatal(err)
	}
	if err := s.SetIntegration(sB, "abc123", now); err != nil {
		t.Fatal(err)
	}
	n, err := s.Retire(sB, now)
	if err != nil || n != 1 {
		t.Fatalf("Retire = %d, %v", n, err)
	}
	if items, _ := s.Listed(sB); len(items) != 0 {
		t.Fatalf("retired item still listed: %v", items)
	}
	if obl, _ := s.Obligations(sB); len(obl) != 0 {
		t.Fatalf("retired item still an obligation: %v", obl)
	}
	if err := s.RecordClose(sB, "done", "", now); err != nil {
		t.Fatal(err)
	}
	sess, _ := s.Session(sB)
	if !sess.Closed || sess.CloseNote != "done" || sess.Integrated != "abc123" || !sess.ClosedAt.Equal(now) {
		t.Fatalf("closed row: %+v", sess)
	}
	open, _ := s.Sessions(false)
	all, _ := s.Sessions(true)
	ids := func(ss []store.Session) []string {
		out := make([]string, 0, len(ss))
		for _, x := range ss {
			out = append(out, x.ID)
		}
		return out
	}
	if !slices.Equal(ids(open), []string{sA, "s-cccccccccccc"}) || !slices.Equal(ids(all), []string{sA, sB, "s-cccccccccccc"}) {
		t.Fatalf("Sessions: open %v all %v", ids(open), ids(all))
	}
}

func TestHeartbeat(t *testing.T) {
	// covers: R-SESS-082
	s := mustOpen(t, freshPath(t))
	if _, err := s.Heartbeat(); !errors.Is(err, store.ErrNoHeartbeat) {
		t.Fatalf("empty heartbeat: %v", err)
	}
	if err := s.SetHeartbeat(now, "0.1.0"); err != nil {
		t.Fatal(err)
	}
	if err := s.SetHeartbeat(now.Add(time.Minute), "0.1.1"); err != nil {
		t.Fatal(err)
	}
	hb, err := s.Heartbeat()
	if err != nil || !hb.At.Equal(now.Add(time.Minute)) || hb.Version != "0.1.1" {
		t.Fatalf("Heartbeat = %+v, %v", hb, err)
	}
}

func TestCleanGitEnv(t *testing.T) {
	// The discovery variables go; authorship and transport stay.
	got := store.CleanGitEnv([]string{"GIT_DIR=/x", "GIT_WORK_TREE=/y", "GIT_AUTHOR_NAME=a", "GIT_SSH_COMMAND=ssh", "PATH=/bin"})
	if !slices.Equal(got, []string{"GIT_AUTHOR_NAME=a", "GIT_SSH_COMMAND=ssh", "PATH=/bin"}) {
		t.Fatalf("CleanGitEnv = %v", got)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/store/ -run 'TestMigrationTwo|TestLaunch|TestAdopted|TestSessionsChildren|TestHeartbeat|TestCleanGitEnv'`
Expected: FAIL — `store.Launch`, `AddLaunch`, `PhaseReady` undefined.

- [ ] **Step 3: Append migration 2**

In `internal/store/migrate.go`, append to `migrations` (the first entry stays byte-identical):
```go
	// Migration 2 (P3): launch state per session, retired items, the worker heartbeat.
	`ALTER TABLE sessions ADD COLUMN title TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN tier TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN effort TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN client TEXT NOT NULL DEFAULT 'claude';
	ALTER TABLE sessions ADD COLUMN phase TEXT NOT NULL DEFAULT 'ready';
	ALTER TABLE sessions ADD COLUMN phase_at TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN run INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE sessions ADD COLUMN pane TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN tab TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN agent TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN client_session TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN worktree TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN branch TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN record TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN base TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN adopted INTEGER NOT NULL DEFAULT 0;
	ALTER TABLE sessions ADD COLUMN router_reason TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN integrated TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN integrated_at TEXT;
	ALTER TABLE sessions ADD COLUMN closed_at TEXT;
	ALTER TABLE sessions ADD COLUMN close_note TEXT NOT NULL DEFAULT '';
	ALTER TABLE sessions ADD COLUMN keep_reason TEXT NOT NULL DEFAULT '';
	ALTER TABLE items ADD COLUMN retired_at TEXT;
	CREATE TABLE heartbeat (id INTEGER PRIMARY KEY CHECK (id = 1), at TEXT NOT NULL, version TEXT NOT NULL);`,
```
Update the comment above the slice: "P3 appended migration 2; P4+ append after it."

- [ ] **Step 4: The Session type and its scan**

Replace the `Session` type and `Session()` in `internal/store/sessions.go`:
```go
// Session is one row of the session tree. Parent is Operator for a root, whose manager is the
// operator (R-MSG-013). Phase is the stored launch phase; the displayed phase is derived by
// internal/session.Shown from Phase, PhaseAt and ReadyAt.
type Session struct {
	ID, Parent, Scope, Title                   string
	Tier, Effort, Client, Phase                string
	Run                                        int
	Pane, Tab, Agent, ClientSession            string
	Worktree, Branch, Record, Base             string
	RouterReason, Integrated                   string
	Adopted, Closed                            bool
	CreatedAt, PhaseAt, ReadyAt, IntegratedAt  time.Time
	ClosedAt, BoundaryAt                       time.Time
	BoundaryNote, CloseNote, KeepReason        string
}

const selectSession = `SELECT id, parent, scope, state, created_at, ready_at, boundary_note, boundary_at,
	title, tier, effort, client, phase, phase_at, run, pane, tab, agent, client_session, worktree, branch,
	record, base, adopted, router_reason, integrated, integrated_at, closed_at, close_note, keep_reason
	FROM sessions`

// Session returns one session or ErrNoSession.
func (s *Store) Session(id string) (Session, error) {
	row := s.db.QueryRowContext(context.Background(), selectSession+` WHERE id = ?`, id)
	out, err := scanSession(row)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNoSession
	}
	return out, err
}

// Sessions lists open sessions in creation order, or every session with all.
func (s *Store) Sessions(all bool) ([]Session, error) {
	q := selectSession
	if !all {
		q += ` WHERE state = 'open'`
	}
	return s.querySessions(q + ` ORDER BY created_at, rowid`)
}

// Children lists the open children of parent in creation order.
func (s *Store) Children(parent string) ([]Session, error) {
	return s.querySessions(selectSession+` WHERE parent = ? AND state = 'open' ORDER BY created_at, rowid`, parent)
}

func (s *Store) querySessions(query string, args ...any) (out []Session, err error) {
	rows, err := s.db.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		sess, serr := scanSession(rows)
		if serr != nil {
			return nil, serr
		}
		out = append(out, sess)
	}
	return out, rows.Err()
}

func scanSession(r scanner) (Session, error) {
	var (
		out                                                   Session
		parent, readyAt, bNote, bAt, created, phaseAt         sql.NullString
		integratedAt, closedAt                                sql.NullString
		state                                                 string
	)
	err := r.Scan(&out.ID, &parent, &out.Scope, &state, &created, &readyAt, &bNote, &bAt,
		&out.Title, &out.Tier, &out.Effort, &out.Client, &out.Phase, &phaseAt, &out.Run, &out.Pane, &out.Tab,
		&out.Agent, &out.ClientSession, &out.Worktree, &out.Branch, &out.Record, &out.Base, &out.Adopted,
		&out.RouterReason, &out.Integrated, &integratedAt, &closedAt, &out.CloseNote, &out.KeepReason)
	if err != nil {
		return Session{}, err
	}
	out.Parent, out.Closed, out.BoundaryNote = parent.String, state == "closed", bNote.String
	out.CreatedAt, out.ReadyAt, out.BoundaryAt = nullStamp(created), nullStamp(readyAt), nullStamp(bAt)
	out.PhaseAt, out.IntegratedAt, out.ClosedAt = nullStamp(phaseAt), nullStamp(integratedAt), nullStamp(closedAt)
	return out, nil
}
```
`scanner` is the interface already declared in `items.go`. `AddSession` keeps its signature and inserts `phase_at` as `stamp(now)` so legacy fixtures have a phase time:
```go
func (s *Store) AddSession(id, scope, parent string, now time.Time) error {
	_, err := s.db.ExecContext(context.Background(),
		`INSERT INTO sessions (id, parent, scope, created_at, phase_at, title) VALUES (?, ?, ?, ?, ?, ?)`,
		id, nullable(parent), scope, stamp(now), stamp(now), scope)
	return err
}
```

- [ ] **Step 5: The launch API**

`internal/store/launch.go`:
```go
package store

import (
	"context"
	"database/sql"
	"errors"
	"strings"
	"time"
)

// Phases the store names itself; the full ordered list lives in internal/session (R-SESS-044).
const (
	PhaseRegistered = "registered"
	PhaseReady      = "ready"
)

// ErrNoHeartbeat is returned by Heartbeat before the worker ever ran.
var ErrNoHeartbeat = errors.New("no worker heartbeat")

// Launch is what ness open records before its first side effect (R-SESS-010, R-SESS-031).
// Worktree and Branch are empty for an adopted root; Record is repo-relative.
type Launch struct {
	ID, Parent, Scope, Title, Tier, Effort, Client string
	Record, Base, Worktree, Branch                 string
	RouterReason                                   string
	Adopted                                        bool
}

// AddLaunch inserts the session in phase registered. An adopted root (open --root --here) is
// ready at once (R-SESS-038).
func (s *Store) AddLaunch(l Launch, now time.Time) error {
	phase, ready := PhaseRegistered, any(nil)
	if l.Adopted {
		phase, ready = PhaseReady, stamp(now)
	}
	_, err := s.db.ExecContext(context.Background(),
		`INSERT INTO sessions (id, parent, scope, created_at, title, tier, effort, client, phase, phase_at,
		 ready_at, record, base, worktree, branch, router_reason, adopted)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		l.ID, nullable(l.Parent), l.Scope, stamp(now), l.Title, l.Tier, l.Effort, l.Client, phase, stamp(now),
		ready, l.Record, l.Base, l.Worktree, l.Branch, l.RouterReason, l.Adopted)
	return err
}

// SetPhase records the phase about to start (R-SESS-031).
func (s *Store) SetPhase(id, phase string, now time.Time) error {
	return s.touch(ErrNoSession, `UPDATE sessions SET phase = ?, phase_at = ? WHERE id = ?`, phase, stamp(now), id)
}

// BindTab binds the session to a new tab and pane for run n, dropping the previous binding and
// agent (R-SESS-048).
func (s *Store) BindTab(id, pane, tab string, run int) error {
	return s.touch(ErrNoSession,
		`UPDATE sessions SET pane = ?, tab = ?, run = ?, agent = '', client_session = '' WHERE id = ?`,
		pane, tab, run, id)
}

// BindAgent records the herdr agent name and the client's session id (R-SESS-027, R-SESS-035).
func (s *Store) BindAgent(id, agent, clientSession string) error {
	return s.touch(ErrNoSession, `UPDATE sessions SET agent = ?, client_session = ? WHERE id = ?`, agent, clientSession, id)
}

// NewRun prepares a relaunch of an exited session: the binding and the ready mark are cleared,
// the phase restarts at phase, and a non-empty tier replaces the stored one (R-SESS-047,
// R-SESS-051).
func (s *Store) NewRun(id, phase, tier, effort string) error {
	return s.touch(ErrNoSession,
		`UPDATE sessions SET pane = '', tab = '', agent = '', client_session = '', ready_at = NULL, phase = ?,
		 tier = CASE WHEN ? = '' THEN tier ELSE ? END, effort = CASE WHEN ? = '' THEN effort ELSE ? END
		 WHERE id = ?`, phase, tier, tier, effort, effort, id)
}

// SetTier changes the tier and effort of a session whose launch continues (R-SESS-051).
func (s *Store) SetTier(id, tier, effort string) error {
	return s.touch(ErrNoSession, `UPDATE sessions SET tier = ?, effort = ? WHERE id = ?`, tier, effort, id)
}

// SetIntegration records the integration commit or PR url that close reads (R-SESS-099).
func (s *Store) SetIntegration(id, ref string, now time.Time) error {
	return s.touch(ErrNoSession, `UPDATE sessions SET integrated = ?, integrated_at = ? WHERE id = ?`, ref, stamp(now), id)
}

// RecordClose marks the session closed with its note and keep reason; the row stays for status
// --all and stats (R-SESS-065, R-SESS-067, R-SESS-068).
func (s *Store) RecordClose(id, note, keep string, now time.Time) error {
	return s.touch(ErrNoSession,
		`UPDATE sessions SET state = 'closed', closed_at = ?, close_note = ?, keep_reason = ? WHERE id = ?`,
		stamp(now), note, keep, id)
}

// Retire drops every open item addressed to recipient from inboxes and obligations, returning
// how many (R-SESS-069, R-WRK-020).
func (s *Store) Retire(recipient string, now time.Time) (int, error) {
	res, err := s.db.ExecContext(context.Background(),
		`UPDATE items SET retired_at = ? WHERE recipient IS ? AND disposed_at IS NULL AND retired_at IS NULL`,
		stamp(now), nullable(recipient))
	if err != nil {
		return 0, err
	}
	n, err := res.RowsAffected()
	return int(n), err
}

// Heartbeat is the worker's last pass (R-WRK-008).
type Heartbeat struct {
	At      time.Time
	Version string
}

// Heartbeat returns the worker's last heartbeat or ErrNoHeartbeat.
func (s *Store) Heartbeat() (Heartbeat, error) {
	var at, version string
	err := s.db.QueryRowContext(context.Background(), `SELECT at, version FROM heartbeat WHERE id = 1`).Scan(&at, &version)
	if errors.Is(err, sql.ErrNoRows) {
		return Heartbeat{}, ErrNoHeartbeat
	}
	if err != nil {
		return Heartbeat{}, err
	}
	return Heartbeat{At: parseStamp(at), Version: version}, nil
}

// SetHeartbeat writes the single heartbeat row.
func (s *Store) SetHeartbeat(now time.Time, version string) error {
	_, err := s.db.ExecContext(context.Background(),
		`INSERT INTO heartbeat (id, at, version) VALUES (1, ?, ?)
		 ON CONFLICT (id) DO UPDATE SET at = excluded.at, version = excluded.version`, stamp(now), version)
	return err
}

// gitDiscovery are the variables that make git ignore -C and use another repository. Any ness
// process started from a git hook or an unusual shell would otherwise open the wrong store.
var gitDiscovery = []string{"GIT_DIR=", "GIT_WORK_TREE=", "GIT_COMMON_DIR=", "GIT_INDEX_FILE=", "GIT_OBJECT_DIRECTORY=",
	"GIT_ALTERNATE_OBJECT_DIRECTORIES=", "GIT_NAMESPACE=", "GIT_CEILING_DIRECTORIES=", "GIT_DISCOVERY_ACROSS_FILESYSTEM="}

// CleanGitEnv returns env without the git discovery variables; authorship and transport stay.
func CleanGitEnv(env []string) []string {
	out := make([]string, 0, len(env))
	for _, kv := range env {
		keep := true
		for _, p := range gitDiscovery {
			if strings.HasPrefix(kv, p) {
				keep = false
				break
			}
		}
		if keep {
			out = append(out, kv)
		}
	}
	return out
}
```
In `store.go`, `Locate` sets `cmd.Env = CleanGitEnv(os.Environ())` before `cmd.Output()`.

In `items.go`, append ` AND retired_at IS NULL` to both the `listed` and `obliged` conditions.

- [ ] **Step 6: Run the store tests**

Run: `go test ./internal/store/`
Expected: PASS, including every P2 test (the P2 fixtures see phase `ready`).

- [ ] **Step 7: Extend the `store` script command**

In `internal/testkit/store.go`, extend the doc comment and `storeSubs`:
```go
	"phase": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		if neg || len(args) != 2 {
			ts.Fatalf("usage: store phase ID PHASE")
		}
		ts.Check(s.SetPhase(args[0], args[1], scriptNow(ts)))
	},
	"bind": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		if neg || len(args) != 4 {
			ts.Fatalf("usage: store bind ID PANE TAB AGENT")
		}
		sess, err := s.Session(args[0])
		ts.Check(err)
		ts.Check(s.BindTab(args[0], args[1], args[2], sess.Run+1))
		ts.Check(s.BindAgent(args[0], args[3], ""))
	},
	"integrate": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		if neg || len(args) != 2 {
			ts.Fatalf("usage: store integrate ID REF")
		}
		ts.Check(s.SetIntegration(args[0], args[1], scriptNow(ts)))
	},
	"heartbeat": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		if neg || len(args) != 2 {
			ts.Fatalf("usage: store heartbeat AGE_SECONDS VERSION")
		}
		age, err := strconv.Atoi(args[0])
		ts.Check(err)
		ts.Check(s.SetHeartbeat(scriptNow(ts).Add(-time.Duration(age)*time.Second), args[1]))
	},
	"sessions": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		if len(args) != 1 {
			ts.Fatalf("usage: store sessions N")
		}
		all, err := s.Sessions(true)
		ts.Check(err)
		if n, _ := strconv.Atoi(args[0]); (len(all) == n) == neg {
			ts.Fatalf("store has %d sessions, want %s", len(all), args[0])
		}
	},
```
and in the `"session"` sub's field map add:
```go
			"phase": sess.Phase, "tier": sess.Tier, "effort": sess.Effort, "run": strconv.Itoa(sess.Run),
			"pane": sess.Pane, "tab": sess.Tab, "agent": sess.Agent, "client_session": sess.ClientSession,
			"worktree": sess.Worktree, "branch": sess.Branch, "record": sess.Record, "integrated": sess.Integrated,
			"title": sess.Title, "adopted": strconv.FormatBool(sess.Adopted), "client": sess.Client,
```
Add the helper (scripts freeze the clock with `NESS_TEST_NOW`, as the `ness` command does):
```go
// scriptNow is the script's clock: NESS_TEST_NOW when set, else the wall clock.
func scriptNow(ts *testscript.TestScript) time.Time {
	if v := ts.Getenv("NESS_TEST_NOW"); v != "" {
		t, err := time.Parse(time.RFC3339, v)
		ts.Check(err)
		return t
	}
	return time.Now()
}
```
Make `add-session` use `scriptNow(ts)` too.

- [ ] **Step 8: Run everything and commit**

Run: `go test ./... && make lint`
Expected: PASS.

```bash
git add internal/store internal/testkit/store.go
git commit -m "feat(store): launch phases, runs, heartbeat and retired items in migration 2"
```

---

### Task 2: The `Exec` seam, the fake exec and the fake herdr

**Files:**
- Create: `internal/cli/exec.go`, `internal/testkit/exec.go`, `internal/testkit/herdr.go`, `internal/testkit/fakes_test.go`
- Modify: `internal/cli/cli.go` (`Deps.Exec`, `Deps.GOOS`), `cmd/ness/main.go`, `internal/cli/script_test.go`, `internal/testkit/testkit.go` (Setup writes the fakes dir; register `fake`)

**Interfaces:**
- Produces:
  - `cli.Deps.Exec func(dir, name string, args ...string) ([]byte, error)` (nil → `cli.SystemExec`), `cli.Deps.GOOS string` ("" → `runtime.GOOS`).
  - `cli.SystemExec(dir, name string, args ...string) ([]byte, error)`: stdout on success; on failure an error that wraps `exec.ErrNotFound` for a missing program, else `fmt.Errorf("%s: %w: %s", name, err, stderr)`.
  - `testkit.FakeExec{Dir string}` with `Run(dir, name string, args ...string) ([]byte, error)`; `testkit.WriteFakeDefaults(dir string) error`; `testkit.FakesDir(e *testscript.Env)` = `$WORK/.fakes`. Scripts get `NESS_TEST_FAKES=$WORK/.fakes`.
  - `testkit.FakeHerdr{Path string}` with `Run(dir string, args []string) ([]byte, error)` and the state file `herdr.json`.
  - Script command `fake`: `fake missing NAME…`, `fake output 'CMD ARGS' TEXT`, `fake fail 'CMD ARGS' TEXT`, `fake calls PREFIX N` (negatable), `fake herdr SUB …` (see Step 5).

- [ ] **Step 1: Write the failing tests**

`internal/testkit/fakes_test.go`:
```go
package testkit_test

import (
	"encoding/json"
	"errors"
	"os/exec"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestFakeExecTableMissingAndGit(t *testing.T) {
	dir := t.TempDir()
	if err := testkit.WriteFakeDefaults(dir); err != nil {
		t.Fatal(err)
	}
	f := testkit.FakeExec{Dir: dir}
	out, err := f.Run("", "claude", "--version")
	if err != nil || strings.TrimSpace(string(out)) != "2.1.294 (Claude Code)" {
		t.Fatalf("claude --version = %q, %v", out, err)
	}
	if _, err := f.Run("", "nope", "x"); err == nil || !strings.Contains(err.Error(), "no fake output") {
		t.Fatalf("unknown command: %v", err)
	}
	if err := testkit.SetFake(dir, "missing", "herdr", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Run("", "herdr", "--version"); !errors.Is(err, exec.ErrNotFound) {
		t.Fatalf("missing herdr: %v", err)
	}
	if out, err := f.Run(t.TempDir(), "git", "--version"); err != nil || !strings.HasPrefix(string(out), "git version") {
		t.Fatalf("git passthrough = %q, %v", out, err)
	}
}

func TestFakeHerdrTabsAndAgents(t *testing.T) {
	dir := t.TempDir()
	if err := testkit.WriteFakeDefaults(dir); err != nil {
		t.Fatal(err)
	}
	f := testkit.FakeExec{Dir: dir}
	out, err := f.Run("", "herdr", "tab", "create", "--cwd", "/tmp/x", "--label", "root", "--env", "NESS_SESSION=s-1", "--no-focus")
	if err != nil {
		t.Fatal(err)
	}
	var created struct {
		Result struct {
			Tab      struct{ TabID string `json:"tab_id"` } `json:"tab"`
			RootPane struct{ PaneID string `json:"pane_id"` } `json:"root_pane"`
		} `json:"result"`
	}
	if err := json.Unmarshal(out, &created); err != nil || created.Result.Tab.TabID != "wF:t1" || created.Result.RootPane.PaneID != "wF:p1" {
		t.Fatalf("tab create = %s, %v", out, err)
	}
	if _, err := f.Run("", "herdr", "agent", "start", "p-root", "--kind", "claude", "--pane", "wF:p1", "--", "--effort", "medium", "hi"); err != nil {
		t.Fatal(err)
	}
	out, _ = f.Run("", "herdr", "agent", "list")
	if !strings.Contains(string(out), `"name":"p-root"`) || !strings.Contains(string(out), `"pane_id":"wF:p1"`) {
		t.Fatalf("agent list = %s", out)
	}
	if _, err := f.Run("", "herdr", "agent", "start", "p-root", "--kind", "claude", "--pane", "wF:p1"); err == nil ||
		!strings.Contains(err.Error(), "agent_name_taken") {
		t.Fatalf("duplicate name: %v", err)
	}
	if _, err := f.Run("", "herdr", "tab", "close", "wF:t1"); err != nil {
		t.Fatal(err)
	}
	if _, err := f.Run("", "herdr", "tab", "close", "wF:t1"); err == nil || !strings.Contains(err.Error(), "not_found") {
		t.Fatalf("closing a closed tab: %v", err)
	}
	out, _ = f.Run("", "herdr", "tab", "list")
	if strings.Contains(string(out), "wF:t1") {
		t.Fatalf("closed tab listed: %s", out)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/testkit/ -run 'TestFake'`
Expected: FAIL — `testkit.FakeExec` undefined.

- [ ] **Step 3: The production exec and the Deps fields**

`internal/cli/exec.go`:
```go
package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// SystemExec runs name with args in dir and returns its stdout. Every external program ness
// starts (git, herdr, claude, codex, gh, systemctl) goes through here, with the git discovery
// variables stripped so a hook's GIT_DIR cannot point ness at another repository. A missing
// program is reported with exec.ErrNotFound; any other failure carries the program's stderr.
func SystemExec(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(context.Background(), name, args...) //nolint:gosec // the atoms pass fixed argv
	cmd.Dir = dir
	cmd.Env = store.CleanGitEnv(os.Environ())
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if errors.Is(err, exec.ErrNotFound) {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if err != nil {
		return out, fmt.Errorf("%s %s: %w: %s", name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// exec returns the process's exec seam.
func (d Deps) exec() func(dir, name string, args ...string) ([]byte, error) {
	if d.Exec == nil {
		return SystemExec
	}
	return d.Exec
}
```
In `internal/cli/cli.go` add to `Deps`:
```go
	// Exec runs external programs; nil means SystemExec. Tests inject testkit.FakeExec.
	Exec func(dir, name string, args ...string) ([]byte, error)
	// GOOS is the operating system; empty means runtime.GOOS. v1 runs on Linux only (R-SESS-015).
	GOOS string
```
and in `newRoot`, before `root.AddCommand`, the Linux guard (cobra skips PersistentPreRunE for `--version` and `--help`):
```go
	root.PersistentPreRunE = func(*cobra.Command, []string) error {
		goos := d.GOOS
		if goos == "" {
			goos = runtime.GOOS
		}
		if goos != "linux" {
			return refuse("ness v1 runs on Linux only; this is %s", goos)
		}
		return nil
	}
```
`cmd/ness/main.go` passes `Exec: cli.SystemExec, GOOS: runtime.GOOS`. `internal/cli/os_test.go` (package `cli`, white-box):
```go
package cli

import (
	"bytes"
	"testing"
)

func TestOtherOSRefusesEverythingButVersion(t *testing.T) {
	// covers: R-SESS-015
	var out, errb bytes.Buffer
	d := Deps{Stdout: &out, Stderr: &errb, Version: "0.1.0", GOOS: "darwin"}
	if code := Main([]string{"status"}, d); code != ExitRefused || !bytes.Contains(errb.Bytes(), []byte("Linux only")) {
		t.Fatalf("status on darwin: exit %d, stderr %q", code, errb.String())
	}
	if code := Main([]string{"--version"}, d); code != ExitOK {
		t.Fatalf("--version on darwin: exit %d", code)
	}
}
```

- [ ] **Step 4: The fake exec**

`internal/testkit/exec.go`:
```go
package testkit

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/enea-scaccabarozzi/ness/internal/cli"
)

// execState is exec.json: the table of canned outputs, what is "not installed", what fails, and
// every call made, so scripts can assert what ness ran (R-QA-024).
type execState struct {
	Missing []string            `json:"missing"`
	Outputs map[string]string   `json:"outputs"`
	Fail    map[string]string   `json:"fail"`
	Calls   [][]string          `json:"calls"`
}

// FakeExec stands in for every external program. git runs for real (the repositories are
// throwaway); herdr is the stateful fake in herdr.go; everything else answers from the table in
// Dir/exec.json, keyed by the command line.
type FakeExec struct{ Dir string }

// Run is assignable to cli.Deps.Exec and to each atom's Exec type.
func (f FakeExec) Run(dir, name string, args ...string) ([]byte, error) {
	st, err := loadExec(f.Dir)
	if err != nil {
		return nil, err
	}
	st.Calls = append(st.Calls, append([]string{name}, args...))
	if err := saveJSON(filepath.Join(f.Dir, "exec.json"), st); err != nil {
		return nil, err
	}
	for _, m := range st.Missing {
		if m == name {
			return nil, fmt.Errorf("%s: %w", name, exec.ErrNotFound)
		}
	}
	switch name {
	case "git":
		return cli.SystemExec(dir, name, args...)
	case "herdr":
		return FakeHerdr{Path: filepath.Join(f.Dir, "herdr.json")}.Run(dir, args)
	}
	key := strings.Join(append([]string{name}, args...), " ")
	if msg, ok := st.Fail[key]; ok {
		return nil, fmt.Errorf("%s: exit status 1: %s", key, msg)
	}
	if out, ok := st.Outputs[key]; ok {
		return []byte(out), nil
	}
	return nil, fmt.Errorf("fake exec: no fake output for %q", key)
}

// SetFake changes exec.json: kind is missing, output or fail.
func SetFake(dir, kind, key, value string) error {
	st, err := loadExec(dir)
	if err != nil {
		return err
	}
	switch kind {
	case "missing":
		st.Missing = append(st.Missing, key)
	case "output":
		st.Outputs[key] = value
	case "fail":
		st.Fail[key] = value
	default:
		return fmt.Errorf("fake: unknown kind %s", kind)
	}
	return saveJSON(filepath.Join(dir, "exec.json"), st)
}

// CodexModels is the default `codex debug models` answer: every default Codex tier resolves.
const CodexModels = `{"models":[{"slug":"gpt-6-luna"},{"slug":"gpt-6.1-sol"},{"slug":"astra"}]}` + "\n"

// WriteFakeDefaults creates dir with a table where the tested versions answer and herdr is
// installed with no tabs.
func WriteFakeDefaults(dir string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	st := execState{Outputs: map[string]string{
		"claude --version":   "2.1.294 (Claude Code)\n",
		"codex --version":    "codex-cli 0.157.1\n",
		"codex debug models": CodexModels,
	}, Fail: map[string]string{}, Calls: [][]string{}}
	if err := saveJSON(filepath.Join(dir, "exec.json"), st); err != nil {
		return err
	}
	return saveJSON(filepath.Join(dir, "herdr.json"), newHerdrState())
}

func loadExec(dir string) (execState, error) {
	var st execState
	data, err := os.ReadFile(filepath.Join(dir, "exec.json")) //nolint:gosec // the fakes dir of a test
	if err != nil {
		return st, err
	}
	return st, json.Unmarshal(data, &st)
}

func saveJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

// fakeCmd is the `fake` script command:
//
//	fake missing NAME...        the programs are not installed
//	fake output 'CMD ARGS' TEXT exact command line → stdout TEXT (a trailing newline is added)
//	fake fail 'CMD ARGS' TEXT   exact command line → exit 1 with TEXT on stderr
//	fake calls PREFIX N         fail unless N recorded calls start with PREFIX (negatable)
//	fake herdr SUB ...          the fake herdr (see herdrCmd)
func fakeCmd(ts *testscript.TestScript, neg bool, args []string) {
	dir := ts.Getenv("NESS_TEST_FAKES")
	if dir == "" || len(args) < 2 {
		ts.Fatalf("usage: fake missing|output|fail|calls|herdr ...")
	}
	switch args[0] {
	case "missing":
		for _, name := range args[1:] {
			ts.Check(SetFake(dir, "missing", name, ""))
		}
	case "output", "fail":
		if neg || len(args) != 3 {
			ts.Fatalf("usage: fake %s 'CMD ARGS' TEXT", args[0])
		}
		ts.Check(SetFake(dir, args[0], args[1], args[2]+"\n"))
	case "calls":
		st, err := loadExec(dir)
		ts.Check(err)
		want, err := strconv.Atoi(args[2])
		ts.Check(err)
		n := 0
		for _, c := range st.Calls {
			if strings.HasPrefix(strings.Join(c, " "), args[1]) {
				n++
			}
		}
		if (n == want) == neg {
			ts.Fatalf("%d calls start with %q, want %d", n, args[1], want)
		}
	case "herdr":
		herdrCmd(ts, neg, filepath.Join(dir, "herdr.json"), args[1:])
	default:
		ts.Fatalf("fake: unknown subcommand %s", args[0])
	}
}
```
(`errors` stays imported for `exec.ErrNotFound` wrapping in `Run`; drop it if goimports says it is unused.)

- [ ] **Step 5: The fake herdr**

`internal/testkit/herdr.go` answers the subset of the herdr 0.8.2 CLI the atom uses, with the real envelope `{"id":"cli:…","result":{…}}` on stdout and `{"error":{"code","message"}}` on stderr with exit 1 (the Exec error text):
```go
package testkit

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

type herdrTab struct {
	ID, Pane, Label, Cwd string
	Env                  map[string]string
}

type herdrAgent struct {
	Name, Pane, Kind, Status string // Status: idle | working | gone
	Args                     []string
}

type herdrState struct {
	Version      string            `json:"version"`
	Next         int               `json:"next"`
	Tabs         []herdrTab        `json:"tabs"`
	Agents       []herdrAgent      `json:"agents"`
	Typed        [][2]string       `json:"typed"` // pane, text (send-text and run)
	Screens      map[string]string `json:"screens"`
	ReadyOnStart bool              `json:"ready_on_start"`
	FailNext     string            `json:"fail_next"` // "tab create" | "agent start": the next such call fails once
}

func newHerdrState() herdrState {
	return herdrState{Version: "0.8.2", Next: 1, Screens: map[string]string{}, ReadyOnStart: true,
		Tabs: []herdrTab{}, Agents: []herdrAgent{}, Typed: [][2]string{}}
}

// FakeHerdr is the stateful fake behind `herdr …` in FakeExec. On `agent start` with
// ready_on_start it writes the ready mark the real client's SessionStart hook would write, into
// the store of the tab's cwd, for the tab's NESS_SESSION.
type FakeHerdr struct{ Path string }

func (h FakeHerdr) load() (herdrState, error) {
	var st herdrState
	data, err := os.ReadFile(h.Path)
	if err != nil {
		return st, err
	}
	return st, json.Unmarshal(data, &st)
}

func herdrErr(code, msg string) error {
	return fmt.Errorf(`herdr: exit status 1: {"error":{"code":%q,"message":%q}}`, code, msg)
}

func envelope(id string, result map[string]any) ([]byte, error) {
	return json.Marshal(map[string]any{"id": "cli:" + id, "result": result})
}

func tabInfo(t herdrTab) map[string]any {
	return map[string]any{"tab_id": t.ID, "label": t.Label, "workspace_id": "wF", "pane_count": 1, "focused": false}
}

func paneInfo(st herdrState, pane string) map[string]any {
	info := map[string]any{"pane_id": pane, "tab_id": "", "workspace_id": "wF", "agent": nil, "agent_status": "unknown", "focused": false}
	for _, t := range st.Tabs {
		if t.Pane == pane {
			info["tab_id"], info["cwd"] = t.ID, t.Cwd
		}
	}
	for _, a := range st.Agents {
		if a.Pane == pane && a.Status != "gone" {
			info["agent"], info["agent_status"], info["name"] = a.Kind, a.Status, a.Name
		}
	}
	return info
}

// Run answers one herdr command line.
func (h FakeHerdr) Run(_ string, args []string) ([]byte, error) {
	st, err := h.load()
	if err != nil {
		return nil, err
	}
	if len(args) == 1 && args[0] == "--version" {
		return []byte("herdr " + st.Version + "\n"), nil
	}
	if len(args) < 2 {
		return nil, herdrErr("usage", "unknown command")
	}
	verb := args[0] + " " + args[1]
	if st.FailNext == verb {
		st.FailNext = ""
		if err := saveJSON(h.Path, st); err != nil {
			return nil, err
		}
		return nil, herdrErr("internal", "injected failure at "+verb)
	}
	out, err := h.dispatch(&st, verb, args[2:])
	if err != nil {
		return nil, err
	}
	return out, saveJSON(h.Path, st)
}

func flagValue(args []string, name string) string {
	for i, a := range args {
		if a == name && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

func (h FakeHerdr) dispatch(st *herdrState, verb string, rest []string) ([]byte, error) {
	switch verb {
	case "tab create":
		n := st.Next
		st.Next++
		t := herdrTab{ID: fmt.Sprintf("wF:t%d", n), Pane: fmt.Sprintf("wF:p%d", n), Label: flagValue(rest, "--label"),
			Cwd: flagValue(rest, "--cwd"), Env: map[string]string{}}
		for i, a := range rest {
			if a == "--env" && i+1 < len(rest) {
				k, v, _ := strings.Cut(rest[i+1], "=")
				t.Env[k] = v
			}
		}
		st.Tabs = append(st.Tabs, t)
		return envelope("tab:create", map[string]any{"type": "tab_create", "tab": tabInfo(t), "root_pane": paneInfo(*st, t.Pane)})
	case "tab list":
		tabs := make([]map[string]any, 0, len(st.Tabs))
		for _, t := range st.Tabs {
			tabs = append(tabs, tabInfo(t))
		}
		return envelope("tab:list", map[string]any{"type": "tab_list", "tabs": tabs})
	case "tab close":
		i := slices.IndexFunc(st.Tabs, func(t herdrTab) bool { return t.ID == rest[0] })
		if i < 0 {
			return nil, herdrErr("not_found", "no tab "+rest[0])
		}
		for j := range st.Agents {
			if st.Agents[j].Pane == st.Tabs[i].Pane {
				st.Agents[j].Status = "gone"
			}
		}
		st.Tabs = slices.Delete(st.Tabs, i, i+1)
		return envelope("tab:close", map[string]any{"type": "tab_close", "tab_id": rest[0]})
	case "agent list":
		agents := make([]map[string]any, 0, len(st.Agents))
		for _, a := range st.Agents {
			if a.Status != "gone" {
				agents = append(agents, map[string]any{"name": a.Name, "pane_id": a.Pane, "agent": a.Kind, "agent_status": a.Status})
			}
		}
		return envelope("agent:list", map[string]any{"type": "agent_list", "agents": agents})
	case "agent start":
		return h.agentStart(st, rest)
	case "agent get":
		i := slices.IndexFunc(st.Agents, func(a herdrAgent) bool { return (a.Name == rest[0] || a.Pane == rest[0]) && a.Status != "gone" })
		if i < 0 {
			return nil, herdrErr("not_found", "no agent "+rest[0])
		}
		return envelope("agent:get", map[string]any{"type": "agent_get", "agent": paneInfo(*st, st.Agents[i].Pane)})
	case "pane read":
		pane := rest[0]
		if slices.IndexFunc(st.Tabs, func(t herdrTab) bool { return t.Pane == pane }) < 0 {
			return nil, herdrErr("not_found", "no pane "+pane)
		}
		return envelope("pane:read", map[string]any{"type": "pane_read", "pane_id": pane, "text": st.Screens[pane], "truncated": false})
	case "pane send-text", "pane run":
		st.Typed = append(st.Typed, [2]string{rest[0], strings.Join(rest[1:], " ")})
		return envelope("pane:input", map[string]any{"type": "pane_input", "pane_id": rest[0]})
	}
	return nil, herdrErr("usage", "unknown command "+verb)
}

var agentNameRe = regexp.MustCompile(`^[a-z][a-z0-9_-]{0,31}$`)

func (h FakeHerdr) agentStart(st *herdrState, rest []string) ([]byte, error) {
	name, pane, kind := rest[0], flagValue(rest, "--pane"), flagValue(rest, "--kind")
	if !agentNameRe.MatchString(name) {
		return nil, herdrErr("invalid_agent_name", name)
	}
	if slices.ContainsFunc(st.Agents, func(a herdrAgent) bool { return a.Name == name && a.Status != "gone" }) {
		return nil, herdrErr("agent_name_taken", name)
	}
	ti := slices.IndexFunc(st.Tabs, func(t herdrTab) bool { return t.Pane == pane })
	if ti < 0 {
		return nil, herdrErr("not_found", "no pane "+pane)
	}
	if slices.ContainsFunc(st.Agents, func(a herdrAgent) bool { return a.Pane == pane && a.Status != "gone" }) {
		return nil, herdrErr("pane_busy", "pane "+pane+" already runs an agent")
	}
	var args []string
	if i := slices.Index(rest, "--"); i >= 0 {
		args = rest[i+1:]
	}
	st.Agents = append(st.Agents, herdrAgent{Name: name, Pane: pane, Kind: kind, Status: "idle", Args: args})
	tab := st.Tabs[ti]
	if st.ReadyOnStart && tab.Env["NESS_SESSION"] != "" {
		if err := markReady(tab.Cwd, tab.Env["NESS_SESSION"]); err != nil {
			return nil, err
		}
	}
	return envelope("agent:start", map[string]any{"type": "agent_start", "agent": paneInfo(*st, pane), "argv": append([]string{kind}, args...)})
}

// markReady does what the client's SessionStart hook does in a real launch (R-HOOK-003).
func markReady(cwd, id string) (err error) {
	path, err := store.Locate(cwd)
	if err != nil {
		return err
	}
	s, err := store.Open(path)
	if err != nil {
		return err
	}
	defer func() { err = errorsJoin(err, s.Close()) }()
	return s.MarkReady(id, scriptClock())
}
```
Add two helpers at the bottom (`errorsJoin` is `errors.Join`; `scriptClock` reads `NESS_TEST_NOW` from the process env, since the fake runs inside the `ness` test process, else `time.Now()`):
```go
func errorsJoin(errs ...error) error { return errors.Join(errs...) }

func scriptClock() time.Time {
	if v := os.Getenv("NESS_TEST_NOW"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			return t
		}
	}
	return time.Now()
}
```
(add `errors` and `time` to the imports.)

The `fake herdr` script subcommands, in the same file:
```go
// herdrCmd drives and checks the fake herdr:
//
//	fake herdr version V              herdr --version answers V
//	fake herdr agent NAME PANE        a live agent of another repository
//	fake herdr tabs N                 fail unless N tabs are open (negatable)
//	fake herdr tab LABEL              fail unless a tab with LABEL is open (negatable)
//	fake herdr typed N                fail unless N texts were typed or run (negatable)
//	fake herdr args REGEX             fail unless the last agent start args, space-joined, match
//	fake herdr env KEY=VALUE          fail unless the last tab was created with that env entry
//	fake herdr agents REGEX           fail unless a live agent name matches (negatable)
//	fake herdr ready-on-start BOOL    whether agent start writes the ready mark
//	fake herdr fail 'tab create'|'agent start'   the next such call fails once
//	fake herdr close-tab LABEL        close the tab out of band
//	fake herdr exit NAME              the agent exited; its pane stays
//	fake herdr screen PANE TEXT       what pane read returns
func herdrCmd(ts *testscript.TestScript, neg bool, path string, args []string) {
	h := FakeHerdr{Path: path}
	st, err := h.load()
	ts.Check(err)
	last := func() herdrAgent {
		if len(st.Agents) == 0 {
			ts.Fatalf("fake herdr: no agent started")
		}
		return st.Agents[len(st.Agents)-1]
	}
	check := func(ok bool, format string, a ...any) {
		if ok == neg {
			ts.Fatalf(format, a...)
		}
	}
	switch args[0] {
	case "version":
		st.Version = args[1]
	case "agent":
		st.Agents = append(st.Agents, herdrAgent{Name: args[1], Pane: args[2], Kind: "claude", Status: "idle"})
	case "tabs":
		n, _ := strconv.Atoi(args[1])
		check(len(st.Tabs) == n, "fake herdr has %d tabs, want %s", len(st.Tabs), args[1])
	case "tab":
		check(slices.ContainsFunc(st.Tabs, func(t herdrTab) bool { return t.Label == args[1] }), "fake herdr: tab %q open = %v", args[1], !neg)
	case "typed":
		n, _ := strconv.Atoi(args[1])
		check(len(st.Typed) == n, "fake herdr typed %d texts, want %s", len(st.Typed), args[1])
	case "args":
		joined := strings.Join(last().Args, " ")
		check(regexp.MustCompile(args[1]).MatchString(joined), "agent args %q do not match %q", joined, args[1])
	case "env":
		if len(st.Tabs) == 0 {
			ts.Fatalf("fake herdr: no tab")
		}
		k, v, _ := strings.Cut(args[1], "=")
		check(st.Tabs[len(st.Tabs)-1].Env[k] == v, "last tab env %s = %q, want %q", k, st.Tabs[len(st.Tabs)-1].Env[k], v)
	case "agents":
		re := regexp.MustCompile(args[1])
		check(slices.ContainsFunc(st.Agents, func(a herdrAgent) bool { return a.Status != "gone" && re.MatchString(a.Name) }),
			"no live agent matches %q", args[1])
	case "ready-on-start":
		st.ReadyOnStart = args[1] == "true"
	case "fail":
		st.FailNext = args[1]
	case "close-tab":
		i := slices.IndexFunc(st.Tabs, func(t herdrTab) bool { return t.Label == args[1] })
		if i < 0 {
			ts.Fatalf("fake herdr: no tab %q", args[1])
		}
		_, err := h.dispatch(&st, "tab close", []string{st.Tabs[i].ID})
		ts.Check(err)
	case "exit":
		i := slices.IndexFunc(st.Agents, func(a herdrAgent) bool { return a.Name == args[1] })
		if i < 0 {
			ts.Fatalf("fake herdr: no agent %q", args[1])
		}
		st.Agents[i].Status = "gone"
	case "screen":
		st.Screens[args[1]] = args[2]
	default:
		ts.Fatalf("fake herdr: unknown subcommand %s", args[0])
	}
	ts.Check(saveJSON(path, st))
}
```

- [ ] **Step 6: Wire the fakes into Setup and the script harness**

In `internal/testkit/testkit.go`, `Setup` creates the fakes dir after the repository:
```go
	fakes := filepath.Join(e.WorkDir, ".fakes")
	if err := WriteFakeDefaults(fakes); err != nil {
		return err
	}
	e.Setenv("NESS_TEST_FAKES", fakes)
```
and `Cmds()` gains `"fake": fakeCmd`. In `internal/cli/script_test.go`, the `ness` command sets `Exec` and `GOOS`:
```go
			d := cli.Deps{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
				Version: v, Session: os.Getenv("NESS_SESSION"), Now: now, GOOS: "linux"}
			if dir := os.Getenv("NESS_TEST_FAKES"); dir != "" {
				d.Exec = testkit.FakeExec{Dir: dir}.Run
			}
			os.Exit(cli.Main(os.Args[1:], d))
```
`testkit` now imports `internal/cli` (for `SystemExec`) and `cli_test` imports `testkit`: no cycle, since `cli` itself does not import `testkit`. If `go vet` reports an import cycle through a `_test` package, move `SystemExec` into `internal/store` as `store.RunGit`… it does not: `cli_test` is an external test package.

- [ ] **Step 7: Run and commit**

Run: `go test ./internal/testkit/ ./internal/cli/ && make lint`
Expected: PASS; every P2 script still passes with the fakes dir present (none of them runs an external program but git).

```bash
git add cmd/ness/main.go internal/cli/cli.go internal/cli/exec.go internal/cli/os_test.go internal/cli/script_test.go internal/testkit
git commit -m "feat(testkit): one Exec seam for external programs, with a fake exec and a fake herdr"
```

---

### Task 3: The runtime seam and the herdr atom

**Files:**
- Create: `internal/session/runtime.go`, `internal/atom/herdr/herdr.go`, `internal/atom/herdr/herdr_test.go`, `internal/atom/herdr/e2e_test.go`, `internal/testkit/contracts.go`

**Interfaces:**
- Consumes: `testkit.FakeExec`, `testkit.WriteFakeDefaults`.
- Produces:
  - `session.Runtime` interface (below), `session.Tab{ID, Pane string}`, `session.TabSpec{Dir, Label string; Env map[string]string}`, `session.AgentSpec{Name, Pane, Kind string; Args []string}`, `session.PaneState{Status string; Live bool}`, `session.ErrNotInstalled`, `session.ErrGone` (tab, pane or agent no longer exists).
  - `herdr.Runtime{Exec herdr.Exec}` implementing `session.Runtime`; `herdr.Exec func(dir, name string, args ...string) ([]byte, error)`; `herdr.ParseVersion("herdr 0.8.2") (string, error)`.
  - `testkit.RuntimeContract(t *testing.T, rt session.Runtime, startAgent bool)`.

- [ ] **Step 1: The seam**

`internal/session/runtime.go`:
```go
// Package session owns the session tree: the seams for the runtime, the client and the model
// policy, the environment checks, the launch state machine and the phase names (design §2, §4,
// §5). Everything that runs a program goes through an injected Exec.
package session

import "errors"

var (
	// ErrNotInstalled is returned by Runtime.Version and Client.Version for a missing program.
	ErrNotInstalled = errors.New("not installed")
	// ErrGone is returned when a tab, pane or agent no longer exists.
	ErrGone = errors.New("gone")
)

// Tab is an open runtime tab and its root pane.
type Tab struct{ ID, Pane string }

// TabSpec describes the tab to open: the working directory and environment of its shell.
type TabSpec struct {
	Dir, Label string
	Env        map[string]string
}

// AgentSpec starts a client in a pane: Name is the runtime's agent name (unique, R-SESS-036),
// Kind the runtime's name for the client (claude, codex), Args the client's command line.
type AgentSpec struct {
	Name, Pane, Kind string
	Args             []string
}

// PaneState is what the runtime knows of a pane's agent: Status idle|working|blocked|done|unknown
// and whether an agent is there at all.
type PaneState struct {
	Status string
	Live   bool
}

// Runtime is the terminal multiplexer seam (R-SESS-110): herdr in v1, the test fake, tmux later.
// Ten methods because the design lists them (open tab, run, type, read, state, agents); each has
// two implementations.
type Runtime interface {
	Version() (string, error)
	OpenTab(TabSpec) (Tab, error)
	CloseTab(id string) error
	Tabs() ([]Tab, error)
	StartAgent(AgentSpec) error
	Agents() ([]string, error)
	State(pane string) (PaneState, error)
	ReadScreen(pane string) (string, error)
	TypeText(pane, text string) error
	Run(pane, command string) error
}
```

- [ ] **Step 2: Write the failing tests**

`internal/testkit/contracts.go` (the shared suites; `ClientContract` and `PolicyContract` are added in Tasks 4 and 5):
```go
package testkit

import (
	"slices"
	"strings"
	"testing"

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
	tab, err := rt.OpenTab(session.TabSpec{Dir: t.TempDir(), Label: "ness-contract", Env: map[string]string{"NESS_SESSION": "s-000000000001"}})
	if err != nil || tab.ID == "" || tab.Pane == "" {
		t.Fatalf("OpenTab = %+v, %v", tab, err)
	}
	t.Cleanup(func() { _ = rt.CloseTab(tab.ID) })
	tabs, err := rt.Tabs()
	if err != nil || !slices.Contains(tabs, tab) {
		t.Fatalf("Tabs = %v, %v; want %v", tabs, err, tab)
	}
	st, err := rt.State(tab.Pane)
	if err != nil || st.Live {
		t.Fatalf("State of a fresh pane = %+v, %v; want no agent", st, err)
	}
	if err := rt.Run(tab.Pane, "true"); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if err := rt.TypeText(tab.Pane, "echo contract"); err != nil {
		t.Fatalf("TypeText: %v", err)
	}
	if _, err := rt.ReadScreen(tab.Pane); err != nil {
		t.Fatalf("ReadScreen: %v", err)
	}
	if startAgent {
		if err := rt.StartAgent(session.AgentSpec{Name: "ness-contract", Pane: tab.Pane, Kind: "claude", Args: []string{"--effort", "low"}}); err != nil {
			t.Fatalf("StartAgent: %v", err)
		}
		names, err := rt.Agents()
		if err != nil || !slices.Contains(names, "ness-contract") {
			t.Fatalf("Agents = %v, %v", names, err)
		}
		if st, err := rt.State(tab.Pane); err != nil || !st.Live {
			t.Fatalf("State after start = %+v, %v", st, err)
		}
		if err := rt.StartAgent(session.AgentSpec{Name: "ness-contract", Pane: tab.Pane, Kind: "claude"}); err == nil ||
			!strings.Contains(err.Error(), "agent_name_taken") {
			t.Fatalf("a taken name was accepted: %v", err)
		}
	}
	if err := rt.CloseTab(tab.ID); err != nil {
		t.Fatalf("CloseTab: %v", err)
	}
	if tabs, _ := rt.Tabs(); slices.Contains(tabs, tab) {
		t.Fatalf("closed tab still listed: %v", tabs)
	}
	if err := rt.CloseTab(tab.ID); !errorsIs(err, session.ErrGone) {
		t.Fatalf("closing a closed tab: %v, want ErrGone", err)
	}
	if _, err := rt.ReadScreen(tab.Pane); !errorsIs(err, session.ErrGone) {
		t.Fatalf("reading a closed pane: %v, want ErrGone", err)
	}
}
```
(`errorsIs` is `errors.Is`; import `errors` and alias it to keep the file free of a name clash with the `errs` variables P2 scripts use.)

`internal/atom/herdr/herdr_test.go`:
```go
package herdr_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/herdr"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func fakeRuntime(t *testing.T) (herdr.Runtime, string) {
	t.Helper()
	dir := t.TempDir()
	if err := testkit.WriteFakeDefaults(dir); err != nil {
		t.Fatal(err)
	}
	return herdr.Runtime{Exec: testkit.FakeExec{Dir: dir}.Run}, dir
}

func TestRuntimeContractOverFake(t *testing.T) {
	// covers: R-SESS-110, R-QA-024
	rt, _ := fakeRuntime(t)
	testkit.RuntimeContract(t, rt, true)
}

func TestHerdrErrors(t *testing.T) {
	// Review Focus 5: an error envelope, a missing program and garbage output are reported, never
	// decoded into an empty tab.
	// covers: R-SESS-001
	rt, dir := fakeRuntime(t)
	if _, err := rt.OpenTab(session.TabSpec{}); err != nil {
		t.Fatal(err)
	}
	if err := rt.CloseTab("wF:t99"); !errors.Is(err, session.ErrGone) || !strings.Contains(err.Error(), "no tab wF:t99") {
		t.Fatalf("CloseTab unknown = %v", err)
	}
	if err := testkit.SetFake(dir, "missing", "herdr", ""); err != nil {
		t.Fatal(err)
	}
	if _, err := rt.Version(); !errors.Is(err, session.ErrNotInstalled) {
		t.Fatalf("missing herdr: %v", err)
	}
	garbage := herdr.Runtime{Exec: func(string, string, ...string) ([]byte, error) { return []byte("not json"), nil }}
	if tab, err := garbage.OpenTab(session.TabSpec{}); err == nil || tab.ID != "" {
		t.Fatalf("garbage output: %+v, %v", tab, err)
	}
	empty := herdr.Runtime{Exec: func(string, string, ...string) ([]byte, error) { return []byte(`{"id":"x","result":{"type":"tab_create"}}`), nil }}
	if tab, err := empty.OpenTab(session.TabSpec{}); err == nil || tab.ID != "" {
		t.Fatalf("envelope without a tab: %+v, %v", tab, err)
	}
}

func TestParseVersion(t *testing.T) {
	// covers: R-SESS-002
	for in, want := range map[string]string{"herdr 0.8.2\n": "0.8.2", "herdr 1.0.0-rc1": "1.0.0-rc1"} {
		if got, err := herdr.ParseVersion(in); err != nil || got != want {
			t.Fatalf("ParseVersion(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := herdr.ParseVersion("tmux 3.4"); err == nil {
		t.Fatal("tmux accepted as herdr")
	}
}
```

- [ ] **Step 3: Run to verify failure**

Run: `go test ./internal/atom/herdr/ ./internal/testkit/`
Expected: FAIL — package `herdr` does not exist.

- [ ] **Step 4: The atom**

`internal/atom/herdr/herdr.go`:
```go
// Package herdr is the runtime atom over the herdr CLI (0.8.2): tabs, agents and panes through
// the socket API's JSON answers. It never reads the UI-focused pane; every call names its target.
package herdr

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// Exec runs a program; cli.SystemExec in production, the fake in tests.
type Exec func(dir, name string, args ...string) ([]byte, error)

// Runtime implements session.Runtime over `herdr`.
type Runtime struct{ Exec Exec }

type envelope struct {
	Result json.RawMessage `json:"result"`
}

type tabInfo struct {
	TabID string `json:"tab_id"`
}

type paneInfo struct {
	PaneID      string  `json:"pane_id"`
	TabID       string  `json:"tab_id"`
	Agent       *string `json:"agent"`
	AgentStatus string  `json:"agent_status"`
	Name        *string `json:"name"`
}

// call runs herdr with args and decodes the result envelope into out. herdr prints an error
// envelope on stderr with exit 1; SystemExec folds that into the error text, which is kept.
func (r Runtime) call(out any, args ...string) error {
	data, err := r.Exec("", "herdr", args...)
	if errors.Is(err, exec.ErrNotFound) {
		return fmt.Errorf("herdr: %w", session.ErrNotInstalled)
	}
	if err != nil {
		if strings.Contains(err.Error(), `"not_found"`) {
			return fmt.Errorf("%w: %v", session.ErrGone, err)
		}
		return err
	}
	var env envelope
	if jerr := json.Unmarshal(data, &env); jerr != nil || env.Result == nil {
		return fmt.Errorf("herdr %s: unexpected output %.80q", strings.Join(args, " "), data)
	}
	if out == nil {
		return nil
	}
	return json.Unmarshal(env.Result, out)
}

// Version returns herdr's version from `herdr --version`.
func (r Runtime) Version() (string, error) {
	data, err := r.Exec("", "herdr", "--version")
	if errors.Is(err, exec.ErrNotFound) {
		return "", fmt.Errorf("herdr: %w", session.ErrNotInstalled)
	}
	if err != nil {
		return "", err
	}
	return ParseVersion(string(data))
}

// ParseVersion reads "herdr X.Y.Z".
func ParseVersion(s string) (string, error) {
	name, v, ok := strings.Cut(strings.TrimSpace(s), " ")
	if !ok || name != "herdr" || v == "" {
		return "", fmt.Errorf("herdr --version printed %q", strings.TrimSpace(s))
	}
	return v, nil
}

// OpenTab creates a tab whose shell starts in spec.Dir with spec.Env, unfocused (R-SESS-030).
func (r Runtime) OpenTab(spec session.TabSpec) (session.Tab, error) {
	args := []string{"tab", "create", "--no-focus"}
	if spec.Dir != "" {
		args = append(args, "--cwd", spec.Dir)
	}
	if spec.Label != "" {
		args = append(args, "--label", spec.Label)
	}
	for _, k := range sortedKeys(spec.Env) {
		args = append(args, "--env", k+"="+spec.Env[k])
	}
	var res struct {
		Tab      tabInfo  `json:"tab"`
		RootPane paneInfo `json:"root_pane"`
	}
	if err := r.call(&res, args...); err != nil {
		return session.Tab{}, err
	}
	if res.Tab.TabID == "" || res.RootPane.PaneID == "" {
		return session.Tab{}, errors.New("herdr tab create: answer has no tab or pane id")
	}
	return session.Tab{ID: res.Tab.TabID, Pane: res.RootPane.PaneID}, nil
}

// CloseTab closes the tab; a tab that is already gone is ErrGone.
func (r Runtime) CloseTab(id string) error { return r.call(nil, "tab", "close", id) }

// Tabs lists open tabs with their root pane (the pane a launch binds to).
func (r Runtime) Tabs() ([]session.Tab, error) {
	var tabs struct {
		Tabs []tabInfo `json:"tabs"`
	}
	if err := r.call(&tabs, "tab", "list"); err != nil {
		return nil, err
	}
	var panes struct {
		Panes []paneInfo `json:"panes"`
	}
	if err := r.call(&panes, "pane", "list"); err != nil {
		return nil, err
	}
	out := make([]session.Tab, 0, len(tabs.Tabs))
	for _, t := range tabs.Tabs {
		tab := session.Tab{ID: t.TabID}
		for _, p := range panes.Panes {
			if p.TabID == t.TabID && tab.Pane == "" {
				tab.Pane = p.PaneID
			}
		}
		out = append(out, tab)
	}
	return out, nil
}

// StartAgent starts the client in the pane under a herdr agent name; herdr returns once the
// client is ready for input (R-SESS-030, R-SESS-035).
func (r Runtime) StartAgent(spec session.AgentSpec) error {
	args := []string{"agent", "start", spec.Name, "--kind", spec.Kind, "--pane", spec.Pane, "--timeout", "120000"}
	if len(spec.Args) > 0 {
		args = append(append(args, "--"), spec.Args...)
	}
	return r.call(nil, args...)
}

// Agents lists the live agent names of every repository (R-SESS-036).
func (r Runtime) Agents() ([]string, error) {
	var res struct {
		Agents []paneInfo `json:"agents"`
	}
	if err := r.call(&res, "agent", "list"); err != nil {
		return nil, err
	}
	names := make([]string, 0, len(res.Agents))
	for _, a := range res.Agents {
		if a.Name != nil && *a.Name != "" {
			names = append(names, *a.Name)
		}
	}
	return names, nil
}

// State reports the agent in the pane; a pane without an agent is Live false, a pane that no
// longer exists is ErrGone.
func (r Runtime) State(pane string) (session.PaneState, error) {
	var res struct {
		Agent paneInfo `json:"agent"`
	}
	err := r.call(&res, "agent", "get", pane)
	if errors.Is(err, session.ErrGone) {
		// No agent in the pane, or no pane: tell them apart with a read.
		if _, rerr := r.ReadScreen(pane); rerr != nil {
			return session.PaneState{}, rerr
		}
		return session.PaneState{Status: "unknown"}, nil
	}
	if err != nil {
		return session.PaneState{}, err
	}
	return session.PaneState{Status: res.Agent.AgentStatus, Live: res.Agent.Agent != nil}, nil
}

// ReadScreen returns the pane's recent output as plain text.
func (r Runtime) ReadScreen(pane string) (string, error) {
	var res struct {
		Text string `json:"text"`
	}
	if err := r.call(&res, "pane", "read", pane, "--source", "recent-unwrapped", "--format", "text", "--lines", "200"); err != nil {
		return "", err
	}
	return res.Text, nil
}

// TypeText sends literal text without Enter.
func (r Runtime) TypeText(pane, text string) error { return r.call(nil, "pane", "send-text", pane, text) }

// Run sends a command line and Enter.
func (r Runtime) Run(pane, command string) error { return r.call(nil, "pane", "run", pane, command) }

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slicesSort(keys)
	return keys
}
```
(`slicesSort` is `slices.Sort`; import `slices`.) The tab `pane_list` the atom's `Tabs` needs: add `pane list` to the fake herdr's `dispatch`, answering `{"type":"pane_list","panes":[paneInfo…]}` for every tab's pane with `tab_id` set (the `paneInfo` helper already fills it).

- [ ] **Step 5: The real contract, opt-in**

`internal/atom/herdr/e2e_test.go`:
```go
//go:build e2e

package herdr_test

import (
	"os"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/herdr"
	"github.com/enea-scaccabarozzi/ness/internal/cli"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestRuntimeContractOverRealHerdr(t *testing.T) {
	// Runs only inside herdr (HERDR_ENV=1): opens one tab, types into it, closes it. No client
	// is started, so it costs nothing (R-SESS-110 real half).
	// covers: R-SESS-110
	if os.Getenv("HERDR_ENV") != "1" {
		t.Skip("not inside herdr")
	}
	testkit.RuntimeContract(t, herdr.Runtime{Exec: cli.SystemExec}, false)
}
```

- [ ] **Step 6: Run and commit**

Run: `go test ./internal/atom/herdr/ ./internal/testkit/ && go vet -tags e2e ./internal/atom/herdr/ && make lint`
Expected: PASS. Then, once, inside this herdr session: `go test -tags e2e -run RealHerdr ./internal/atom/herdr/ -v` — expected PASS with one tab opened and closed; if the real `tab list`/`pane list` shapes differ from the fake, fix the fake (the real CLI is the authority) and rerun both.

```bash
git add internal/session/runtime.go internal/atom/herdr internal/testkit/contracts.go internal/testkit/herdr.go
git commit -m "feat(atom): herdr runtime atom behind the session.Runtime seam, with its contract suite"
```

---

### Task 4: The client seam, the Claude Code atom and the Codex atom

**Files:**
- Create: `internal/session/client.go`, `internal/atom/claude/claude.go`, `internal/atom/claude/claude_test.go`, `internal/atom/codex/codex.go`, `internal/atom/codex/codex_test.go`
- Modify: `internal/testkit/contracts.go` (`ClientContract`), `internal/config/config.go` (effort lists from the atoms), `docs/design/decisions.md` (D40)

**Interfaces:**
- Produces:
  - `session.LaunchSpec{Dir, Model, Effort, Prompt, SessionID, FallbackModel, Ness string; Resume bool}`, `session.Transcript{Context int; LastActive time.Time}`, `session.Client` interface:
    `Name() string`, `Tested() string`, `Version() (string, error)`, `Efforts() []string`, `ResolveModel(id string) error`, `NewSessionID() string`, `Args(LaunchSpec) []string`, `CurrentSession() string`, `Transcript(home, dir, id string) (Transcript, error)`.
  - `claude.Client{Exec claude.Exec}`, `claude.Tested = "2.1.294"`, `claude.Efforts`, `claude.Aliases`, `claude.EnterpriseSkills = "/etc/claude-code/skills"`, `claude.ProjectDir(home, dir string) string`.
  - `codex.Client{Exec codex.Exec}`, `codex.Tested = "0.157.1"`, `codex.Efforts`, `codex.HookOverrides(ness string) []string`.
  - `testkit.ClientContract(t, c session.Client)`.
  - `config.ClaudeEfforts`, `config.CodexEfforts` are replaced by `claude.Efforts` and `codex.Efforts`.

- [ ] **Step 1: The seam**

`internal/session/client.go`:
```go
package session

import "time"

// LaunchSpec is one client launch: the worktree, the resolved model and effort, the one-line
// prompt, the client session id (R-SESS-027) and, on resume, the stored one (R-SESS-047).
// Ness is the ness executable the Codex hooks run; "" means "ness".
type LaunchSpec struct {
	Dir, Model, Effort, Prompt, SessionID, FallbackModel, Ness string
	Resume                                                      bool
}

// Transcript is what a client's transcript tells status: the context size of the last turn and
// when the session last wrote.
type Transcript struct {
	Context    int
	LastActive time.Time
}

// Client is the coding-agent seam (R-SESS-111): Claude Code, Codex (experimental) and the
// contract they share. Args is pure; Version, ResolveModel and Transcript touch the outside.
type Client interface {
	Name() string
	Tested() string
	Version() (string, error)
	Efforts() []string
	ResolveModel(id string) error
	NewSessionID() string
	Args(LaunchSpec) []string
	CurrentSession() string
	Transcript(home, dir, id string) (Transcript, error)
}
```

- [ ] **Step 2: Write the failing tests**

Append to `internal/testkit/contracts.go`:
```go
// ClientContract is the client seam's contract (R-SESS-111): tested version, efforts, the launch
// argv rules shared by every client (effort on the command line, the prompt as one element,
// never through the environment) and model resolution.
func ClientContract(t *testing.T, c session.Client) {
	t.Helper()
	if c.Tested() == "" || c.Name() == "" || len(c.Efforts()) == 0 {
		t.Fatalf("Name %q Tested %q Efforts %v", c.Name(), c.Tested(), c.Efforts())
	}
	v, err := c.Version()
	if err != nil || v == "" {
		t.Fatalf("Version = %q, %v", v, err)
	}
	prompt := "You are ness session s-aaaaaaaaaaaa (root). Read docs/sessions/root/brief.md, then load ness-entry."
	args := c.Args(session.LaunchSpec{Dir: "/w", Model: "m", Effort: c.Efforts()[0], Prompt: prompt, SessionID: c.NewSessionID()})
	if !slices.Contains(args, prompt) {
		t.Fatalf("the prompt is not one argv element: %q", args)
	}
	if !slices.ContainsFunc(args, func(a string) bool { return strings.Contains(a, c.Efforts()[0]) }) {
		t.Fatalf("effort missing from argv %q", args)
	}
	for _, a := range args {
		if strings.Contains(a, "CLAUDE_CODE_EFFORT_LEVEL") {
			t.Fatalf("effort passed through the environment: %q", args)
		}
	}
	if err := c.ResolveModel("no-such-model-xyz"); err == nil {
		t.Fatal("an unknown model id resolved")
	}
}
```

`internal/atom/claude/claude_test.go`:
```go
package claude_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func fakeClient(t *testing.T) claude.Client {
	t.Helper()
	dir := t.TempDir()
	if err := testkit.WriteFakeDefaults(dir); err != nil {
		t.Fatal(err)
	}
	return claude.Client{Exec: testkit.FakeExec{Dir: dir}.Run}
}

func TestClientContractOverFake(t *testing.T) {
	// covers: R-SESS-111, R-QA-024
	testkit.ClientContract(t, fakeClient(t))
}

func TestArgs(t *testing.T) {
	// The prompt is one element, --session-id binds the transcript, effort is explicit (frontier
	// included), --fallback-model only when set, --resume on a relaunch.
	// covers: R-SESS-025, R-SESS-027, R-SESS-028, R-SESS-047, R-MOD-004, R-MOD-010, R-MOD-044
	c := fakeClient(t)
	spec := session.LaunchSpec{Dir: "/w", Model: "fable", Effort: "medium", Prompt: "You are ness session s-1 (x). Read r/brief.md, then load ness-entry.", SessionID: "u-1"}
	got := c.Args(spec)
	want := []string{"--session-id", "u-1", "--model", "fable", "--effort", "medium", spec.Prompt}
	if !slices.Equal(got, want) {
		t.Fatalf("Args = %q, want %q", got, want)
	}
	spec.FallbackModel = "sonnet"
	if got := c.Args(spec); !slices.Contains(got, "--fallback-model") || got[slices.Index(got, "--fallback-model")+1] != "sonnet" {
		t.Fatalf("fallback: %q", got)
	}
	spec.Resume = true
	if got := c.Args(spec); got[0] != "--resume" || got[1] != "u-1" || slices.Contains(got, "--session-id") {
		t.Fatalf("resume: %q", got)
	}
}

func TestSessionIDsAreUUIDs(t *testing.T) {
	// covers: R-SESS-027
	re := regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$`)
	c := fakeClient(t)
	a, b := c.NewSessionID(), c.NewSessionID()
	if !re.MatchString(a) || a == b {
		t.Fatalf("ids %q %q", a, b)
	}
}

func TestModelsAndEfforts(t *testing.T) {
	// covers: R-MOD-031, R-MOD-033, R-SESS-017
	c := fakeClient(t)
	for _, ok := range []string{"opus", "fable", "sonnet", "haiku", "claude-opus-5-5", "claude-fable-5-1"} {
		if err := c.ResolveModel(ok); err != nil {
			t.Fatalf("ResolveModel(%q) = %v", ok, err)
		}
	}
	for _, bad := range []string{"gpt-6-luna", "best", "", "Opus"} {
		if err := c.ResolveModel(bad); err == nil {
			t.Fatalf("ResolveModel(%q) accepted", bad)
		}
	}
	if !slices.Equal(c.Efforts(), []string{"low", "medium", "high", "xhigh", "max"}) || slices.Contains(c.Efforts(), "minimal") {
		t.Fatalf("Efforts = %v", c.Efforts())
	}
}

func TestVersionAndCurrentSession(t *testing.T) {
	// covers: R-SESS-003, R-SESS-016, R-SESS-038
	c := fakeClient(t)
	if v, err := c.Version(); err != nil || v != "2.1.294" || c.Tested() != "2.1.294" {
		t.Fatalf("Version = %q, %v; Tested %q", v, err, c.Tested())
	}
	t.Setenv("CLAUDE_CODE_SESSION_ID", "abc")
	if c.CurrentSession() != "abc" {
		t.Fatalf("CurrentSession = %q", c.CurrentSession())
	}
}

func TestTranscriptContextAndActivity(t *testing.T) {
	// The transcript lives under ~/.claude/projects/<encoded dir>/<id>.jsonl; context is the last
	// assistant usage (input + cache read + cache creation); activity is the file's mtime.
	// covers: R-SESS-080
	home, dir := t.TempDir(), "/home/x/Projects/app/.worktrees/s-1"
	p := filepath.Join(home, ".claude", "projects", "-home-x-Projects-app--worktrees-s-1", "u-1.jsonl")
	if err := os.MkdirAll(filepath.Dir(p), 0o750); err != nil {
		t.Fatal(err)
	}
	lines := strings.Join([]string{
		`{"type":"user","message":{"role":"user","content":"hi"}}`,
		`{"type":"assistant","message":{"usage":{"input_tokens":10,"cache_creation_input_tokens":200,"cache_read_input_tokens":3000,"output_tokens":5}}}`,
		`{"type":"assistant","message":{"usage":{"input_tokens":2,"cache_creation_input_tokens":192,"cache_read_input_tokens":193021,"output_tokens":47}}}`,
		`not json at all`,
	}, "\n")
	if err := os.WriteFile(p, []byte(lines), 0o600); err != nil {
		t.Fatal(err)
	}
	stamp := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	if err := os.Chtimes(p, stamp, stamp); err != nil {
		t.Fatal(err)
	}
	tr, err := fakeClient(t).Transcript(home, dir, "u-1")
	if err != nil || tr.Context != 193215 || !tr.LastActive.Equal(stamp) {
		t.Fatalf("Transcript = %+v, %v", tr, err)
	}
	if _, err := fakeClient(t).Transcript(home, dir, "u-2"); err == nil {
		t.Fatal("missing transcript accepted")
	}
}

func TestEnterpriseSkillsPath(t *testing.T) {
	// covers: R-SESS-014
	if claude.EnterpriseSkills != "/etc/claude-code/skills" {
		t.Fatalf("EnterpriseSkills = %q", claude.EnterpriseSkills)
	}
}
```

`internal/atom/codex/codex_test.go`:
```go
package codex_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/codex"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func fakeClient(t *testing.T) (codex.Client, string) {
	t.Helper()
	dir := t.TempDir()
	if err := testkit.WriteFakeDefaults(dir); err != nil {
		t.Fatal(err)
	}
	return codex.Client{Exec: testkit.FakeExec{Dir: dir}.Run}, dir
}

func TestClientContractOverFake(t *testing.T) {
	// covers: R-SESS-111, R-QA-024
	c, _ := fakeClient(t)
	testkit.ClientContract(t, c)
}

func TestArgsCarryTrustUpdateAndHooks(t *testing.T) {
	// covers: R-SESS-113, R-HOOK-018, R-MOD-010
	c, _ := fakeClient(t)
	got := c.Args(session.LaunchSpec{Dir: "/w/t", Model: "gpt-6-luna", Effort: "low", Prompt: "You are ness session s-1 (x). Read r/brief.md, then load ness-entry."})
	joined := strings.Join(got, " ")
	for _, want := range []string{
		"-m gpt-6-luna", `-c model_reasoning_effort="low"`, `-c projects."/w/t".trust_level="trusted"`,
		"-c check_for_update_on_startup=false", "--enable codex_hooks", "--dangerously-bypass-hook-trust",
		`"command":"ness hook session-start"`, `"command":"ness hook pre-tool"`,
	} {
		if !strings.Contains(joined, want) {
			t.Fatalf("argv %q lacks %q", joined, want)
		}
	}
	if got[len(got)-1] != "You are ness session s-1 (x). Read r/brief.md, then load ness-entry." {
		t.Fatalf("prompt is not the last element: %q", got)
	}
	for i, a := range got {
		if a == "-c" && strings.HasPrefix(got[i+1], "hooks.") && !strings.HasPrefix(got[i+1], "hooks.SessionStart=") && !strings.HasPrefix(got[i+1], "hooks.PreToolUse=") {
			t.Fatalf("unexpected hook key %q", got[i+1])
		}
	}
}

func TestModelsFromDebugModels(t *testing.T) {
	// covers: R-MOD-030, R-MOD-032, R-MOD-033
	c, dir := fakeClient(t)
	if err := testkit.SetFake(dir, "output", "codex debug models", `{"models":[{"slug":"gpt-6-luna"}]}`); err != nil {
		t.Fatal(err)
	}
	if err := c.ResolveModel("gpt-6-luna"); err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"astra", "gpt-6.1-sol", "best", "opus"} {
		if err := c.ResolveModel(bad); err == nil || !strings.Contains(err.Error(), "codex debug models") {
			t.Fatalf("ResolveModel(%q) = %v", bad, err)
		}
	}
	if !slices.Equal(c.Efforts(), []string{"minimal", "low", "medium", "high", "xhigh"}) || slices.Contains(c.Efforts(), "max") {
		t.Fatalf("Efforts = %v", c.Efforts())
	}
	if c.NewSessionID() != "" || c.CurrentSession() != "" {
		t.Fatal("codex has no session id flag; the atom must not invent one")
	}
}
```

- [ ] **Step 3: Run to verify failure**

Run: `go test ./internal/atom/... ./internal/testkit/`
Expected: FAIL — packages `claude` and `codex` do not exist.

- [ ] **Step 4: The spike for the enterprise skills path (R-SESS-014)**

Run, against the installed client:
```bash
B=$(readlink -f "$(which claude)"); claude --version
LC_ALL=C grep -oa 'etc/claude-code[A-Za-z/._-]\{0,40\}' "$B" | sort | uniq -c
LC_ALL=C grep -oa 'Library/Application Support/ClaudeCode[A-Za-z/._-]\{0,30\}' "$B" | sort | uniq -c
```
Expected: the Linux binary names `etc/claude-code` as its managed directory (the macOS counterpart is `Library/Application Support/ClaudeCode`); the enterprise skills live in `skills/` under it. Append to `docs/design/decisions.md`:
```markdown
## 2026-10-08: P3 spikes (session)

- **D40** The Claude Code enterprise skills path on Linux is `/etc/claude-code/skills` (the
  managed directory `/etc/claude-code`, which the installed client 2.1.294 names in its binary;
  the enterprise skills directory is `skills/` under it, as the Claude Code skills reference
  documents). It is the constant `claude.EnterpriseSkills` (R-SESS-014). Evidence: `grep -a
  'etc/claude-code'` on the installed binary.
```
If the grep finds a different directory, use that one in both places.

- [ ] **Step 5: The Claude Code atom**

`internal/atom/claude/claude.go`:
```go
// Package claude is the client atom for Claude Code: launch arguments, model and effort rules,
// the transcript reader. Hooks are wired by sync into .claude/settings.json (R-HOOK-001).
package claude

import (
	"bufio"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// Exec runs a program; cli.SystemExec in production, the fake in tests.
type Exec func(dir, name string, args ...string) ([]byte, error)

const (
	// Tested is the Claude Code version this binary was built against (R-SESS-016).
	Tested = "2.1.294"
	// EnterpriseSkills is the enterprise skills directory on Linux (R-SESS-014, D40).
	EnterpriseSkills = "/etc/claude-code/skills"
)

// Efforts are the values --effort accepts (R-SESS-017); Aliases the model aliases the client
// resolves itself (R-MOD-031).
var (
	Efforts = []string{"low", "medium", "high", "xhigh", "max"}
	Aliases = []string{"fable", "opus", "sonnet", "haiku"}
)

// Client implements session.Client for Claude Code.
type Client struct{ Exec Exec }

// Name is the client's config and --client value and its herdr kind.
func (Client) Name() string { return "claude" }

// Tested returns the tested version.
func (Client) Tested() string { return Tested }

// Efforts returns the accepted effort values.
func (Client) Efforts() []string { return Efforts }

// Version runs `claude --version` ("2.1.294 (Claude Code)").
func (c Client) Version() (string, error) {
	out, err := c.Exec("", "claude", "--version")
	if errors.Is(err, exec.ErrNotFound) {
		return "", fmt.Errorf("claude: %w", session.ErrNotInstalled)
	}
	if err != nil {
		return "", err
	}
	v, _, _ := strings.Cut(strings.TrimSpace(string(out)), " ")
	if v == "" {
		return "", fmt.Errorf("claude --version printed %q", strings.TrimSpace(string(out)))
	}
	return v, nil
}

// ResolveModel accepts the aliases and full ids; the client has no dry resolver, so the launch
// is the real check (R-MOD-031).
func (Client) ResolveModel(id string) error {
	if slices.Contains(Aliases, id) || (strings.HasPrefix(id, "claude-") && len(id) > len("claude-")) {
		return nil
	}
	return fmt.Errorf("model %q is neither an alias (%s) nor a full claude-… id", id, strings.Join(Aliases, ", "))
}

// NewSessionID returns a random UUID v4 for --session-id (R-SESS-027).
func (Client) NewSessionID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand cannot fail on supported platforms
	}
	b[6], b[8] = (b[6]&0x0f)|0x40, (b[8]&0x3f)|0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

// Args builds the command line after `claude` (R-SESS-025, R-SESS-027, R-SESS-028, R-SESS-047,
// R-MOD-044): the prompt is the last, single element.
func (Client) Args(s session.LaunchSpec) []string {
	args := []string{"--session-id", s.SessionID}
	if s.Resume {
		args = []string{"--resume", s.SessionID}
	}
	args = append(args, "--model", s.Model, "--effort", s.Effort)
	if s.FallbackModel != "" {
		args = append(args, "--fallback-model", s.FallbackModel)
	}
	return append(args, s.Prompt)
}

// CurrentSession is the running conversation's id, which Claude Code exports to its hooks and
// shells (used by open --root --here, R-SESS-038).
func (Client) CurrentSession() string { return os.Getenv("CLAUDE_CODE_SESSION_ID") }

// ProjectDir is where Claude Code keeps the transcripts of dir: every "/" and "." becomes "-".
func ProjectDir(home, dir string) string {
	enc := strings.NewReplacer("/", "-", ".", "-").Replace(dir)
	return filepath.Join(home, ".claude", "projects", enc)
}

type usageLine struct {
	Message struct {
		Usage struct {
			Input         int `json:"input_tokens"`
			CacheCreation int `json:"cache_creation_input_tokens"`
			CacheRead     int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// Transcript reads <home>/.claude/projects/<dir>/<id>.jsonl: the context size is the prompt of
// the last assistant turn (input + cache read + cache creation); lines that do not parse are
// skipped (R-SESS-080, R-STAT-010).
func (Client) Transcript(home, dir, id string) (tr session.Transcript, err error) {
	path := filepath.Join(ProjectDir(home, dir), id+".jsonl")
	f, err := os.Open(path) //nolint:gosec // the client's transcript of this session
	if err != nil {
		return tr, err
	}
	defer func() { err = errors.Join(err, f.Close()) }()
	info, err := f.Stat()
	if err != nil {
		return tr, err
	}
	tr.LastActive = info.ModTime()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for sc.Scan() {
		var u usageLine
		if json.Unmarshal(sc.Bytes(), &u) != nil {
			continue
		}
		if n := u.Message.Usage.Input + u.Message.Usage.CacheRead + u.Message.Usage.CacheCreation; n > 0 {
			tr.Context = n
		}
	}
	return tr, sc.Err()
}
```

- [ ] **Step 6: The Codex atom**

`internal/atom/codex/codex.go`:
```go
// Package codex is the experimental client atom for Codex (R-CFG-044): launch arguments with
// the trust and update overrides that keep the launch free of dialogs (R-SESS-113), hooks on
// the command line (R-HOOK-018), model ids checked against `codex debug models` (R-MOD-030).
package codex

import (
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"sync"

	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// Exec runs a program; cli.SystemExec in production, the fake in tests.
type Exec func(dir, name string, args ...string) ([]byte, error)

// Tested is the Codex version this binary was built against (R-SESS-016).
const Tested = "0.157.1"

// Efforts are the reasoning efforts Codex accepts (R-MOD-033).
var Efforts = []string{"minimal", "low", "medium", "high", "xhigh"}

// Client implements session.Client for Codex. Model ids are fetched once per process.
type Client struct {
	Exec   Exec
	once   sync.Once
	models []string
	merr   error
}

// Name is the client's config and --client value and its herdr kind.
func (*Client) Name() string { return "codex" }

// Tested returns the tested version.
func (*Client) Tested() string { return Tested }

// Efforts returns the accepted effort values.
func (*Client) Efforts() []string { return Efforts }

// Version runs `codex --version` ("codex-cli 0.157.1").
func (c *Client) Version() (string, error) {
	out, err := c.Exec("", "codex", "--version")
	if errors.Is(err, exec.ErrNotFound) {
		return "", fmt.Errorf("codex: %w", session.ErrNotInstalled)
	}
	if err != nil {
		return "", err
	}
	_, v, ok := strings.Cut(strings.TrimSpace(string(out)), " ")
	if !ok || v == "" {
		return "", fmt.Errorf("codex --version printed %q", strings.TrimSpace(string(out)))
	}
	return v, nil
}

// ResolveModel accepts only a slug listed by `codex debug models` (R-MOD-030, R-MOD-032).
func (c *Client) ResolveModel(id string) error {
	c.once.Do(func() {
		out, err := c.Exec("", "codex", "debug", "models")
		if err != nil {
			c.merr = err
			return
		}
		var list struct {
			Models []struct {
				Slug string `json:"slug"`
			} `json:"models"`
		}
		if err := json.Unmarshal(out, &list); err != nil {
			c.merr = fmt.Errorf("codex debug models: %w", err)
			return
		}
		for _, m := range list.Models {
			c.models = append(c.models, m.Slug)
		}
	})
	if c.merr != nil {
		return c.merr
	}
	if !slices.Contains(c.models, id) {
		return fmt.Errorf("model %q is not listed by codex debug models (%s)", id, strings.Join(c.models, ", "))
	}
	return nil
}

// NewSessionID is empty: Codex has no --session-id (ruling); the transcript is not bound.
func (*Client) NewSessionID() string { return "" }

// CurrentSession is empty for the same reason.
func (*Client) CurrentSession() string { return "" }

// HookOverrides are the -c entries that run the ness hooks (R-HOOK-018), in the shape of
// ~/.codex/hooks.json.
func HookOverrides(ness string) []string {
	if ness == "" {
		ness = "ness"
	}
	entry := func(event string) string {
		b, _ := json.Marshal([]map[string]any{{"hooks": []map[string]string{{"type": "command", "command": ness + " hook " + event}}}})
		return string(b)
	}
	return []string{"-c", "hooks.SessionStart=" + entry("session-start"), "-c", "hooks.PreToolUse=" + entry("pre-tool")}
}

// Args builds the command line after `codex`: model and effort, trust for the worktree and no
// update prompt (R-SESS-113), the hooks, then the one-line prompt (R-SESS-025).
func (*Client) Args(s session.LaunchSpec) []string {
	args := []string{
		"-m", s.Model, "-c", fmt.Sprintf("model_reasoning_effort=%q", s.Effort),
		"-c", fmt.Sprintf("projects.%q.trust_level=\"trusted\"", s.Dir),
		"-c", "check_for_update_on_startup=false",
		"--enable", "codex_hooks", "--dangerously-bypass-hook-trust",
	}
	args = append(args, HookOverrides(s.Ness)...)
	return append(args, s.Prompt)
}

// Transcript is not available for Codex in v1 (R-STAT-013).
func (*Client) Transcript(string, string, string) (session.Transcript, error) {
	return session.Transcript{}, errors.New("codex transcripts are not read in v1")
}
```
`codex.Client` has pointer receivers (the memoised model list), so the cli builds `&codex.Client{…}`.

- [ ] **Step 7: Efforts come from the atoms**

In `internal/config/config.go`, delete `claudeEfforts` and `codexEfforts` and use `claude.Efforts` / `codex.Efforts` in `validate` (import the two atom packages; they do not import `config`). Run `go test ./internal/config/` to confirm nothing else moved.

- [ ] **Step 8: Run and commit**

Run: `go test ./internal/atom/... ./internal/config/ ./internal/testkit/ && make lint`
Expected: PASS.

```bash
git add internal/session/client.go internal/atom/claude internal/atom/codex internal/testkit/contracts.go internal/config/config.go docs/design/decisions.md
git commit -m "feat(atom): Claude Code and Codex client atoms behind the session.Client seam"
```

---

### Task 5: Environment checks, phases, ids and the model policy seam

**Files:**
- Create: `internal/session/phases.go`, `internal/session/checks.go`, `internal/session/names.go`, `internal/session/policy.go`, `internal/session/session_test.go`, `internal/atom/textpolicy/textpolicy.go`, `internal/atom/textpolicy/textpolicy_test.go`
- Modify: `internal/testkit/contracts.go` (`PolicyContract`, `FakeRouter`)

**Interfaces:**
- Produces:
  - `session.Phases []string` (frozen order), `session.NextPhases(phase string) []string`, `session.Shown(s store.Session, now time.Time, timeout time.Duration) string` (`ready` | `stalled launch` | the stored phase), `session.NewID() string`, `session.Prompt(id, scope, record string) string`, `session.ScopeRe`.
  - `session.TestedHerdr = "0.8.2"`, `session.Env{Runtime Runtime; Client Client; Tiers map[string]config.Tier; ProjectSkills string; SkillRoots []string}`, `(Env).Check() error`, `session.CheckVersion(what, found, tested string) error` (same major, minor ≥ tested; herdr and clients alike), `session.CheckShadowing(projectSkills string, roots []string) error`, `session.SkillRoots(home string) []string`, `session.Versionless`.
  - `session.AgentName(project, scope, id string, live []string) string`.
  - `session.Brief{Text, Role, Client, ParentTier, Tier string; Tiers map[string]config.Tier}`, `session.Choice{Tier, Effort string; Confidence float64; Reason string}`, `session.Policy` interface `Choose(Brief) (Choice, error)`, `session.ErrNeedTier`, `session.TierEffort(t config.Tier, client string) (model, effort string)`.
  - `textpolicy.Policy{}`; `testkit.FakeRouter{Tier string; Confidence float64}`; `testkit.PolicyContract(t, p session.Policy)`.

- [ ] **Step 1: Write the failing tests**

`internal/session/session_test.go`:
```go
package session_test

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestPhasesGolden(t *testing.T) {
	// covers: R-SESS-044
	want := []string{"registered", "creating-worktree", "linking-files", "writing-brief", "opening-tab",
		"starting-agent", "sending-brief", "waiting-ready", "ready"}
	if !slices.Equal(session.Phases, want) {
		t.Fatalf("Phases = %v", session.Phases)
	}
	if got := session.NextPhases("opening-tab"); !slices.Equal(got, want[4:]) {
		t.Fatalf("NextPhases(opening-tab) = %v", got)
	}
	if got := session.NextPhases("registered"); !slices.Equal(got, want[1:]) {
		t.Fatalf("NextPhases(registered) = %v", got)
	}
}

func TestShownPhase(t *testing.T) {
	// covers: R-SESS-033, R-SESS-085
	now := time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)
	s := store.Session{Phase: "waiting-ready", PhaseAt: now.Add(-59 * time.Second)}
	if got := session.Shown(s, now, time.Minute); got != "waiting-ready" {
		t.Fatalf("young launch shown as %q", got)
	}
	s.PhaseAt = now.Add(-61 * time.Second)
	if got := session.Shown(s, now, time.Minute); got != "stalled launch" {
		t.Fatalf("old launch shown as %q", got)
	}
	if got := session.Shown(s, now, 2*time.Minute); got != "waiting-ready" {
		t.Fatalf("with a 120 s timeout shown as %q", got)
	}
	s.ReadyAt = now
	if got := session.Shown(s, now, time.Minute); got != "ready" {
		t.Fatalf("marked ready shown as %q", got)
	}
}

func TestIDsScopeAndPrompt(t *testing.T) {
	// covers: R-SESS-042, R-SESS-043, R-SESS-025
	seen := map[string]bool{}
	for range 1000 {
		id := session.NewID()
		if len(id) != 14 || !strings.HasPrefix(id, "s-") || strings.Trim(id[2:], "0123456789abcdef") != "" || seen[id] {
			t.Fatalf("bad or repeated id %q", id)
		}
		seen[id] = true
	}
	for s, ok := range map[string]bool{"sync": true, "a1-b2": true, "Sync_1": false, "1sync": false, "": false,
		strings.Repeat("a", 32): true, strings.Repeat("a", 33): false} {
		if session.ScopeRe.MatchString(s) != ok {
			t.Fatalf("scope %q accepted = %v", s, !ok)
		}
	}
	want := "You are ness session s-aaaaaaaaaaaa (sync). Read docs/sessions/root/sync/brief.md, then load ness-entry."
	if got := session.Prompt("s-aaaaaaaaaaaa", "sync", "docs/sessions/root/sync"); got != want || strings.Contains(got, "\n") {
		t.Fatalf("Prompt = %q", got)
	}
}

func TestCheckVersion(t *testing.T) {
	// covers: R-SESS-002, R-SESS-003
	if err := session.CheckVersion("herdr", "0.8.2", "0.8.2"); err != nil {
		t.Fatal(err)
	}
	if err := session.CheckVersion("herdr", "0.9.0", "0.8.2"); err != nil {
		t.Fatal(err)
	}
	err := session.CheckVersion("herdr", "0.5.0", "0.8.2")
	if err == nil || !strings.Contains(err.Error(), "0.5.0") || !strings.Contains(err.Error(), "0.8.2") {
		t.Fatalf("older herdr: %v", err)
	}
	if err := session.CheckVersion("claude", "3.0.0", "2.1.294"); err == nil {
		t.Fatal("another major accepted")
	}
	if err := session.CheckVersion("claude", "weird", "2.1.294"); err == nil {
		t.Fatal("unparsable version accepted")
	}
}

func TestShadowing(t *testing.T) {
	// An exact name in any global root refuses, naming both paths; plugin (namespaced) and
	// near names pass.
	// covers: R-SESS-005, R-SESS-006, R-SESS-007
	home := t.TempDir()
	project := filepath.Join(home, "repo", ".agents", "skills")
	for _, d := range []string{filepath.Join(project, "deploy"), filepath.Join(project, "x:deploy"), filepath.Join(project, "review")} {
		if err := os.MkdirAll(d, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	roots := session.SkillRoots(home)
	want := []string{filepath.Join(home, ".claude", "skills"), "/etc/claude-code/skills", filepath.Join(home, ".agents", "skills"), "/etc/codex/skills"}
	if !slices.Equal(roots, want) {
		t.Fatalf("SkillRoots = %v", roots)
	}
	fakeRoots := []string{filepath.Join(home, "r1"), filepath.Join(home, "r2"), filepath.Join(home, "r3"), filepath.Join(home, "r4")}
	if err := session.CheckShadowing(project, fakeRoots); err != nil {
		t.Fatalf("no globals: %v", err)
	}
	for i, r := range fakeRoots {
		if err := os.MkdirAll(filepath.Join(r, "deploy-prod"), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := session.CheckShadowing(project, fakeRoots); err != nil {
			t.Fatalf("near name refused: %v", err)
		}
		if err := os.MkdirAll(filepath.Join(r, "deploy"), 0o750); err != nil {
			t.Fatal(err)
		}
		err := session.CheckShadowing(project, fakeRoots)
		if err == nil || !strings.Contains(err.Error(), filepath.Join(r, "deploy")) || !strings.Contains(err.Error(), filepath.Join(project, "deploy")) {
			t.Fatalf("root %d: %v", i, err)
		}
		if err := os.RemoveAll(filepath.Join(r, "deploy")); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(fakeRoots[0], "x:deploy"), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := session.CheckShadowing(project, fakeRoots); err != nil {
		t.Fatalf("plugin skill refused: %v", err)
	}
}

func TestAgentName(t *testing.T) {
	// covers: R-SESS-035, R-SESS-036
	if got := session.AgentName("infra-lab", "sync", "s-0123456789ab", nil); got != "infra-lab-sync" {
		t.Fatalf("AgentName = %q", got)
	}
	long := session.AgentName("infra-lab", strings.Repeat("s", 60), "s-0123456789ab", nil)
	if len(long) != 32 || !strings.HasPrefix(long, "infra-lab-sss") {
		t.Fatalf("long = %q", long)
	}
	got := session.AgentName("fin-harness", "hotfixes", "s-0123456789ab", []string{"fin-harness-hotfixes"})
	if got != "fin-harness-hotfixes-0" {
		t.Fatalf("collision = %q", got)
	}
	got = session.AgentName("fin-harness", "hotfixes", "s-0123456789ab", []string{"fin-harness-hotfixes", "fin-harness-hotfixes-0"})
	if got != "fin-harness-hotfixes-01" {
		t.Fatalf("double collision = %q", got)
	}
	if got := session.AgentName("Ness", "Sync.1", "s-0123456789ab", nil); got != "ness-sync-1" {
		t.Fatalf("sanitised = %q", got)
	}
	if got := session.AgentName(strings.Repeat("p", 31), "x", "s-0123456789ab", []string{strings.Repeat("p", 31) + "-"}); len(got) != 32 {
		t.Fatalf("suffix within the limit: %q (%d)", got, len(got))
	}
}

func TestTierEffort(t *testing.T) {
	// covers: R-MOD-003
	tier := config.Tier{Claude: "opus/medium", Codex: "gpt-6.1-sol/high"}
	if m, e := session.TierEffort(tier, "claude"); m != "opus" || e != "medium" {
		t.Fatalf("claude = %s/%s", m, e)
	}
	if m, e := session.TierEffort(tier, "codex"); m != "gpt-6.1-sol" || e != "high" {
		t.Fatalf("codex = %s/%s", m, e)
	}
}
```

`internal/atom/textpolicy/textpolicy_test.go`:
```go
package textpolicy_test

import (
	"errors"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/atom/textpolicy"
	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

func TestTextPolicyNeedsAnExplicitTier(t *testing.T) {
	// covers: R-MOD-040, R-MOD-041, R-MOD-043
	p := textpolicy.Policy{}
	b := session.Brief{Text: "do x", Role: "child", Client: "claude", Tiers: config.DefaultTiers()}
	if _, err := p.Choose(b); !errors.Is(err, session.ErrNeedTier) {
		t.Fatalf("without --tier: %v", err)
	}
	b.Tier = "economy"
	c, err := p.Choose(b)
	if err != nil || c.Tier != "economy" || c.Effort != "medium" || c.Confidence != 1 || c.Reason != "explicit --tier" {
		t.Fatalf("Choose = %+v, %v", c, err)
	}
	b.Tier = "ultra"
	if _, err := p.Choose(b); err == nil {
		t.Fatal("unknown tier accepted")
	}
	testkit.PolicyContract(t, p)
	testkit.PolicyContract(t, testkit.FakeRouter{Tier: "standard", Confidence: 0.9})
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/session/ ./internal/atom/textpolicy/`
Expected: FAIL — undefined symbols.

- [ ] **Step 3: Phases, ids, prompt**

`internal/session/phases.go`:
```go
package session

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"slices"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// Phases are the launch phases in order (R-SESS-044), persisted before each step's side effect
// (R-SESS-031). Names are frozen by TestPhasesGolden.
var Phases = []string{
	store.PhaseRegistered, "creating-worktree", "linking-files", "writing-brief", "opening-tab",
	"starting-agent", "sending-brief", "waiting-ready", store.PhaseReady,
}

// NextPhases returns the phases to run after the stored phase: the stored step is re-run (its
// side effect may be half done; every step is idempotent) and the ones after it. An unknown
// phase is treated as registered.
func NextPhases(phase string) []string {
	i := slices.Index(Phases, phase)
	if i <= 0 {
		return Phases[1:]
	}
	return Phases[i:]
}

// Shown is the displayed phase (R-SESS-033, R-SESS-085): ready once the mark exists, stalled
// launch when the mark is missing and the phase is older than the ready timeout, else the phase.
func Shown(s store.Session, now time.Time, timeout time.Duration) string {
	if !s.ReadyAt.IsZero() || s.Phase == store.PhaseReady {
		return store.PhaseReady
	}
	if now.Sub(s.PhaseAt) > timeout {
		return "stalled launch"
	}
	return s.Phase
}

// ScopeRe is the scope syntax (R-SESS-043).
var ScopeRe = regexp.MustCompile(`^[a-z][a-z0-9-]{0,31}$`)

// NewID returns "s-" plus 12 hex characters (R-SESS-042).
func NewID() string {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand cannot fail on supported platforms
	}
	return "s-" + hex.EncodeToString(b[:])
}

// Prompt is the one-line prompt argument the client starts with (R-SESS-025).
func Prompt(id, scope, record string) string {
	return fmt.Sprintf("You are ness session %s (%s). Read %s/brief.md, then load ness-entry.", id, scope, record)
}
```

- [ ] **Step 4: Checks and names**

`internal/session/checks.go`:
```go
package session

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

// TestedHerdr is the herdr version this binary was built against (R-SESS-016).
const TestedHerdr = "0.8.2"

// Env is what the environment checks of design §4 need. Check runs them in order and returns
// the first refusal; nothing has been written when it returns (R-SESS-010).
type Env struct {
	Runtime       Runtime
	Client        Client
	Tiers         map[string]config.Tier
	ProjectSkills string   // <root>/.agents/skills
	SkillRoots    []string // SkillRoots(home) in production; injected in tests (R-SESS-006)
}

// Check refuses when herdr is missing or old (R-SESS-001, R-SESS-002), the client is missing or
// old (R-SESS-003), a tier's model does not resolve (R-SESS-004) or a global skill shadows a
// project skill (R-SESS-005).
func (e Env) Check() error {
	v, err := e.Runtime.Version()
	if errors.Is(err, ErrNotInstalled) {
		return errors.New("herdr is required and is not installed; install herdr 0.8.2 or newer (there is no tmux or plain-terminal fallback)")
	}
	if err != nil {
		return err
	}
	if err := CheckVersion("herdr", v, TestedHerdr); err != nil {
		return err
	}
	cv, err := e.Client.Version()
	if errors.Is(err, ErrNotInstalled) {
		return fmt.Errorf("%s is not installed; install version %s", e.Client.Name(), e.Client.Tested())
	}
	if err != nil {
		return err
	}
	if err := CheckVersion(e.Client.Name(), cv, e.Client.Tested()); err != nil {
		return err
	}
	for _, name := range slices.Sorted(mapsKeys(e.Tiers)) {
		model, _ := TierEffort(e.Tiers[name], e.Client.Name())
		if err := e.Client.ResolveModel(model); err != nil {
			return fmt.Errorf("tier %s (%s = %q): %v; fix [tiers.%s] in .ness/ness.toml", name, e.Client.Name(), model, err, name)
		}
	}
	return CheckShadowing(e.ProjectSkills, e.SkillRoots)
}

// CheckVersion accepts found when it has tested's major and at least its minor (herdr and the
// clients release often; a newer client is not a reason to refuse).
func CheckVersion(what, found, tested string) error {
	fm, fn, err := majorMinor(found)
	if err != nil {
		return fmt.Errorf("%s version %q is not a version; tested version is %s", what, found, tested)
	}
	tm, tn, err := majorMinor(tested)
	if err != nil {
		return err
	}
	if fm != tm || fn < tn {
		return fmt.Errorf("%s %s is not supported: ness was tested with %s %s; install that version or newer in the same major",
			what, found, what, tested)
	}
	return nil
}

func majorMinor(v string) (major, minor int, err error) {
	parts := strings.SplitN(strings.TrimPrefix(v, "v"), ".", 3)
	if len(parts) < 2 {
		return 0, 0, fmt.Errorf("version %q", v)
	}
	if major, err = strconv.Atoi(parts[0]); err != nil {
		return 0, 0, err
	}
	minor, err = strconv.Atoi(strings.SplitN(parts[1], "-", 2)[0])
	return major, minor, err
}

// SkillRoots are the global skill directories a project skill can be shadowed from (§4, D14):
// Claude Code personal and enterprise (R-SESS-014), Codex personal and system.
func SkillRoots(home string) []string {
	return []string{filepath.Join(home, ".claude", "skills"), "/etc/claude-code/skills",
		filepath.Join(home, ".agents", "skills"), "/etc/codex/skills"}
}

// CheckShadowing refuses when a directory in any root has exactly the name of a directory in
// projectSkills (R-SESS-005, R-SESS-006). Plugin skills are namespaced (name:skill) and cannot
// shadow; near names do not match (R-SESS-007).
func CheckShadowing(projectSkills string, roots []string) error {
	local, err := os.ReadDir(projectSkills)
	if err != nil {
		return nil //nolint:nilerr // no project skills, nothing to shadow
	}
	for _, root := range roots {
		for _, d := range local {
			if !d.IsDir() || strings.Contains(d.Name(), ":") {
				continue
			}
			global := filepath.Join(root, d.Name())
			if info, err := os.Stat(global); err == nil && info.IsDir() {
				return fmt.Errorf("global skill %s shadows project skill %s; rename or remove one of them",
					global, filepath.Join(projectSkills, d.Name()))
			}
		}
	}
	return nil
}

// TierEffort splits the tier row for the client.
func TierEffort(t config.Tier, client string) (model, effort string) {
	if client == "codex" {
		return config.SplitTier(t.Codex)
	}
	return config.SplitTier(t.Claude)
}

func mapsKeys(m map[string]config.Tier) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
```
`internal/session/names.go`:
```go
package session

import (
	"regexp"
	"slices"
	"strings"
)

// nameLimit is herdr's agent name length ([a-z][a-z0-9_-]{0,31}).
const nameLimit = 32

var nameBad = regexp.MustCompile(`[^a-z0-9_-]+`)

// AgentName is "<project>-<scope>" lowercased, truncated to herdr's limit, made unique against
// the live agents of every repository by appending characters of the session id (never by
// attaching to the other agent) (R-SESS-035, R-SESS-036).
func AgentName(project, scope, id string, live []string) string {
	base := nameBad.ReplaceAllString(strings.ToLower(project+"-"+scope), "-")
	base = strings.TrimLeft(base, "0123456789_-")
	if base == "" {
		base = "ness"
	}
	if len(base) > nameLimit {
		base = base[:nameLimit]
	}
	if !slices.Contains(live, base) {
		return base
	}
	chars := strings.TrimPrefix(id, "s-")
	for n := 1; n <= len(chars); n++ {
		suffix := "-" + chars[:n]
		stem := base
		if len(stem)+len(suffix) > nameLimit {
			stem = stem[:nameLimit-len(suffix)]
		}
		if name := stem + suffix; !slices.Contains(live, name) {
			return name
		}
	}
	return base[:nameLimit-len(chars)-1] + "-" + chars
}
```

- [ ] **Step 5: The policy seam and the text policy**

`internal/session/policy.go`:
```go
package session

import (
	"errors"

	"github.com/enea-scaccabarozzi/ness/internal/config"
)

// ErrNeedTier is returned by a policy that wants the agent to choose (the text policy, or a
// router below its threshold): open refuses and asks for --tier (R-SESS-023, R-MOD-042).
var ErrNeedTier = errors.New("choose a tier with --tier")

// Brief is the model policy's input (R-MOD-041): the brief text, the role, the parent's tier,
// the project's tier table, the client, and the explicit --tier when given.
type Brief struct {
	Text, Role, Client, ParentTier, Tier string
	Tiers                                map[string]config.Tier
}

// Choice is the policy's output: a tier of the table, the effort for the client, a confidence
// in 0..1 and a one-line reason recorded for stats (R-MOD-043).
type Choice struct {
	Tier, Effort string
	Confidence   float64
	Reason       string
}

// Policy is the model policy seam (design §2, D17): text in v1, a router later.
type Policy interface {
	Choose(Brief) (Choice, error)
}
```
`internal/atom/textpolicy/textpolicy.go`:
```go
// Package textpolicy is the v1 model policy (R-MOD-040): the agent reads the tier guidance in
// the generated skill text and passes --tier; the policy only validates it.
package textpolicy

import (
	"fmt"
	"slices"
	"strings"

	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// Policy chooses nothing itself.
type Policy struct{}

// Choose returns the explicit tier with full confidence, or ErrNeedTier without one.
func (Policy) Choose(b session.Brief) (session.Choice, error) {
	if b.Tier == "" {
		return session.Choice{}, session.ErrNeedTier
	}
	t, ok := b.Tiers[b.Tier]
	if !ok {
		keys := make([]string, 0, len(b.Tiers))
		for k := range b.Tiers {
			keys = append(keys, k)
		}
		slices.Sort(keys)
		return session.Choice{}, fmt.Errorf("--tier %s is not in the tier table; valid tiers are %s", b.Tier, strings.Join(keys, ", "))
	}
	_, effort := session.TierEffort(t, b.Client)
	return session.Choice{Tier: b.Tier, Effort: effort, Confidence: 1, Reason: "explicit --tier"}, nil
}
```
Append to `internal/testkit/contracts.go`:
```go
// FakeRouter is a model policy that always answers Tier with Confidence (R-MOD-041, R-MOD-042).
type FakeRouter struct {
	Tier       string
	Confidence float64
}

// Choose implements session.Policy.
func (f FakeRouter) Choose(b session.Brief) (session.Choice, error) {
	_, effort := session.TierEffort(b.Tiers[f.Tier], b.Client)
	return session.Choice{Tier: f.Tier, Effort: effort, Confidence: f.Confidence, Reason: "fake router: " + b.Role}, nil
}

// PolicyContract is the model policy seam's contract (R-MOD-041): for a brief with an explicit
// tier the answer names a tier of the table, an effort, a confidence in 0..1 and a one-line
// reason.
func PolicyContract(t *testing.T, p session.Policy) {
	t.Helper()
	b := session.Brief{Text: "Implement the thing.", Role: "child", Client: "claude", ParentTier: "standard",
		Tier: "standard", Tiers: config.DefaultTiers()}
	c, err := p.Choose(b)
	if err != nil {
		t.Fatalf("Choose: %v", err)
	}
	if _, ok := b.Tiers[c.Tier]; !ok || c.Effort == "" || c.Confidence < 0 || c.Confidence > 1 ||
		c.Reason == "" || strings.Contains(c.Reason, "\n") {
		t.Fatalf("Choice = %+v", c)
	}
}
```
(import `internal/config` in contracts.go.)

- [ ] **Step 6: Run and commit**

Run: `go test ./internal/session/ ./internal/atom/textpolicy/ ./internal/testkit/ && make lint`
Expected: PASS.

```bash
git add internal/session internal/atom/textpolicy internal/testkit/contracts.go
git commit -m "feat(session): environment checks, frozen phases, agent names and the model policy seam"
```

---

### Task 6: The launch state machine

**Files:**
- Create: `internal/session/launch.go`, `internal/session/wait.go`, `internal/session/launch_test.go`, `internal/session/wait_test.go`

**Interfaces:**
- Consumes: `store.AddLaunch/SetPhase/BindTab/BindAgent/NewRun/Session/MarkReady`, `session.Runtime`, `session.Client`, `session.NextPhases`, `session.AgentName`, `session.Prompt`.
- Produces:
  - `session.Launcher{Store *store.Store; Runtime Runtime; Client Client; Exec Exec; Primary, Project, Sessions string; Copy, Link []string; ReadyTimeout time.Duration; Now func() time.Time; Log io.Writer}`, `session.Exec` type.
  - `session.Request{ID, Model, Effort, FallbackModel, Ness string; Brief []byte; Resume bool}`.
  - `(*Launcher).Run(r Request) error`; `session.ErrStalled` (wrapped when the ready mark does not arrive).
  - `session.waitFor(ready func() (bool, error), timeout, tick time.Duration) error` (unexported; tested in package).

- [ ] **Step 1: Write the failing tests**

`internal/session/wait_test.go` (package `session`, white-box, synctest):
```go
package session

import (
	"errors"
	"testing"
	"testing/synctest"
	"time"
)

func TestWaitForTimesOutAtTheConfiguredTimeout(t *testing.T) {
	// covers: R-SESS-033
	synctest.Test(t, func(t *testing.T) {
		start := time.Now()
		err := waitFor(func() (bool, error) { return false, nil }, 60*time.Second, 250*time.Millisecond)
		if !errors.Is(err, ErrStalled) || time.Since(start) != 60*time.Second {
			t.Fatalf("err %v after %v", err, time.Since(start))
		}
		start = time.Now()
		err = waitFor(func() (bool, error) { return false, nil }, 120*time.Second, 250*time.Millisecond)
		if !errors.Is(err, ErrStalled) || time.Since(start) != 120*time.Second {
			t.Fatalf("120 s: err %v after %v", err, time.Since(start))
		}
	})
}

func TestWaitForReturnsOnTheMark(t *testing.T) {
	// covers: R-SESS-032
	synctest.Test(t, func(t *testing.T) {
		start, calls := time.Now(), 0
		err := waitFor(func() (bool, error) { calls++; return calls == 4, nil }, 60*time.Second, 250*time.Millisecond)
		if err != nil || time.Since(start) != 750*time.Millisecond {
			t.Fatalf("err %v after %v", err, time.Since(start))
		}
	})
}
```

`internal/session/launch_test.go` (black-box over the fake herdr and the real atoms):
```go
package session_test

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
	"github.com/enea-scaccabarozzi/ness/internal/atom/herdr"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
	"github.com/enea-scaccabarozzi/ness/internal/testkit"
)

type bench struct {
	l     *session.Launcher
	s     *store.Store
	fakes string
	repo  string
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.CommandContext(t.Context(), "git", args...)
	cmd.Dir, cmd.Env = dir, store.CleanGitEnv(append(os.Environ(), "GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@x", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@x"))
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return strings.TrimSpace(string(out))
}

func newBench(t *testing.T) *bench {
	t.Helper()
	work := t.TempDir()
	repo, fakes := filepath.Join(work, "repo"), filepath.Join(work, "fakes")
	if err := os.MkdirAll(filepath.Join(repo, "data"), 0o750); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(repo, ".env.example"), []byte("A=1\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	git(t, repo, "add", "-A")
	git(t, repo, "-c", "commit.gpgsign=false", "commit", "-q", "-m", "init")
	if err := testkit.WriteFakeDefaults(fakes); err != nil {
		t.Fatal(err)
	}
	fe := testkit.FakeExec{Dir: fakes}.Run
	path, err := store.Locate(repo)
	if err != nil {
		t.Fatal(err)
	}
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return &bench{s: s, fakes: fakes, repo: repo, l: &session.Launcher{
		Store: s, Runtime: herdr.Runtime{Exec: fe}, Client: claude.Client{Exec: fe}, Exec: fe,
		Primary: repo, Project: "p", Sessions: "docs/sessions", Copy: []string{".env.example"}, Link: []string{"data"},
		ReadyTimeout: 2 * time.Second, Now: time.Now, Log: os.Stderr,
	}}
}

func (b *bench) register(t *testing.T, id, scope string) {
	t.Helper()
	l := store.Launch{ID: id, Scope: scope, Title: scope, Tier: "standard", Effort: "medium", Client: "claude",
		Record: "docs/sessions/" + scope, Base: "main", Worktree: ".worktrees/" + scope + "-" + strings.TrimPrefix(id, "s-"),
		Branch: "session/" + scope + "-" + strings.TrimPrefix(id, "s-")}
	if err := b.s.AddLaunch(l, time.Now()); err != nil {
		t.Fatal(err)
	}
}

const sA = "s-aaaaaaaaaaaa"

func TestLaunchEndToEndOverFakes(t *testing.T) {
	// Worktree, branch, links, brief, tab, agent, mark, phases; nothing typed (so no /model or
	// effort command ever reaches the pane, R-MOD-012's session half).
	// covers: R-SESS-020, R-SESS-021, R-SESS-022, R-SESS-026, R-SESS-030, R-SESS-032, R-SESS-034, R-SESS-037, R-MOD-012
	b := newBench(t)
	b.register(t, sA, "root")
	err := b.l.Run(session.Request{ID: sA, Model: "opus", Effort: "medium", Brief: []byte("# brief\n")})
	if err != nil {
		t.Fatal(err)
	}
	sess, _ := b.s.Session(sA)
	wt := filepath.Join(b.repo, sess.Worktree)
	if sess.Phase != "ready" || sess.ReadyAt.IsZero() || sess.Tab == "" || sess.Pane == "" || sess.Agent != "p-root" || sess.Run != 1 {
		t.Fatalf("session after launch: %+v", sess)
	}
	if git(t, b.repo, "worktree", "list") == "" || !strings.Contains(git(t, b.repo, "branch", "--list", sess.Branch), sess.Branch) {
		t.Fatal("no worktree or branch")
	}
	if git(t, wt, "merge-base", "--is-ancestor", "main", "HEAD") != "" {
		t.Fatal("branch does not start from main")
	}
	if data, err := os.ReadFile(filepath.Join(wt, "docs", "sessions", "root", "brief.md")); err != nil || string(data) != "# brief\n" {
		t.Fatalf("brief: %q, %v", data, err)
	}
	if target, err := os.Readlink(filepath.Join(wt, "data")); err != nil || target != filepath.Join(b.repo, "data") {
		t.Fatalf("link: %q, %v", target, err)
	}
	if info, err := os.Lstat(filepath.Join(wt, ".env.example")); err != nil || !info.Mode().IsRegular() {
		t.Fatalf("copy: %v", err)
	}
	st, err := testkit.HerdrState(b.fakes)
	if err != nil || len(st.Tabs) != 1 || len(st.Typed) != 0 || st.Tabs[0].Env["NESS_SESSION"] != sA || st.Tabs[0].Env["NESS_RUN"] != "1" {
		t.Fatalf("fake herdr: %+v, %v", st, err)
	}
	if !strings.Contains(strings.Join(st.Agents[0].Args, " "), "--session-id "+sess.ClientSession) || sess.ClientSession == "" {
		t.Fatalf("args %q, client session %q", st.Agents[0].Args, sess.ClientSession)
	}
	if last := st.Agents[0].Args[len(st.Agents[0].Args)-1]; last != session.Prompt(sA, "root", "docs/sessions/root") {
		t.Fatalf("prompt argv = %q", last)
	}
}

func TestLaunchPersistsPhaseBeforeEachStep(t *testing.T) {
	// A fault at step k leaves the store at phase k and nothing of step k+1 (R-SESS-031); resume
	// continues from there with exactly one worktree, branch and tab (R-SESS-045).
	// covers: R-SESS-031, R-SESS-045
	for _, fail := range []string{"tab create", "agent start"} {
		b := newBench(t)
		b.register(t, sA, "root")
		if err := testkit.SetHerdr(b.fakes, "fail_next", fail); err != nil {
			t.Fatal(err)
		}
		err := b.l.Run(session.Request{ID: sA, Model: "opus", Effort: "medium", Brief: []byte("b")})
		if err == nil || !strings.Contains(err.Error(), "injected failure") {
			t.Fatalf("%s: %v", fail, err)
		}
		sess, _ := b.s.Session(sA)
		want := map[string]string{"tab create": "opening-tab", "agent start": "starting-agent"}[fail]
		if sess.Phase != want || !sess.ReadyAt.IsZero() {
			t.Fatalf("%s: phase %q ready %v", fail, sess.Phase, sess.ReadyAt)
		}
		st, _ := testkit.HerdrState(b.fakes)
		if fail == "tab create" && (len(st.Tabs) != 0 || sess.Tab != "") {
			t.Fatalf("tab exists after a failed tab create: %+v", st.Tabs)
		}
		if fail == "agent start" && (len(st.Tabs) != 1 || len(st.Agents) != 0) {
			t.Fatalf("after a failed agent start: tabs %d agents %d", len(st.Tabs), len(st.Agents))
		}
		if err := b.l.Run(session.Request{ID: sA, Model: "opus", Effort: "medium"}); err != nil {
			t.Fatalf("%s resume: %v", fail, err)
		}
		st, _ = testkit.HerdrState(b.fakes)
		if len(st.Tabs) != 1 || len(st.Agents) != 1 || strings.Count(git(t, b.repo, "worktree", "list"), ".worktrees/") != 1 {
			t.Fatalf("%s resume: tabs %d agents %d worktrees %q", fail, len(st.Tabs), len(st.Agents), git(t, b.repo, "worktree", "list"))
		}
	}
}

func TestLaunchStallsWithoutTheMark(t *testing.T) {
	// covers: R-SESS-033
	b := newBench(t)
	b.l.ReadyTimeout = 300 * time.Millisecond
	b.register(t, sA, "root")
	if err := testkit.SetHerdr(b.fakes, "ready_on_start", "false"); err != nil {
		t.Fatal(err)
	}
	err := b.l.Run(session.Request{ID: sA, Model: "opus", Effort: "medium", Brief: []byte("b")})
	if !errors.Is(err, session.ErrStalled) {
		t.Fatalf("err = %v", err)
	}
	sess, _ := b.s.Session(sA)
	if sess.Phase != "waiting-ready" || session.Shown(sess, time.Now().Add(time.Second), b.l.ReadyTimeout) != "stalled launch" {
		t.Fatalf("after stall: %+v", sess)
	}
}

func TestLaunchReadyBeforeWaiting(t *testing.T) {
	// Review Focus 2: the mark arrives during agent start; the launch completes and the phase
	// never regresses.
	// covers: R-SESS-032, R-HOOK-009
	b := newBench(t)
	b.register(t, sA, "root")
	if err := b.l.Run(session.Request{ID: sA, Model: "opus", Effort: "medium", Brief: []byte("b")}); err != nil {
		t.Fatal(err)
	}
	sess, _ := b.s.Session(sA)
	if sess.Phase != "ready" {
		t.Fatalf("phase %q", sess.Phase)
	}
	// Running again on a ready session is a no-op: no second tab.
	if err := b.l.Run(session.Request{ID: sA, Model: "opus", Effort: "medium"}); err != nil {
		t.Fatal(err)
	}
	if st, _ := testkit.HerdrState(b.fakes); len(st.Tabs) != 1 {
		t.Fatalf("tabs %d", len(st.Tabs))
	}
}

func TestResumeAfterTabGone(t *testing.T) {
	// Review Focus 3 and R-SESS-046: the tab of a stalled launch was closed out of band and the
	// pane read is ambiguous; resume opens one new tab and continues.
	// covers: R-SESS-045, R-SESS-046, R-SESS-048
	b := newBench(t)
	b.l.ReadyTimeout = 200 * time.Millisecond
	b.register(t, sA, "root")
	if err := testkit.SetHerdr(b.fakes, "ready_on_start", "false"); err != nil {
		t.Fatal(err)
	}
	if err := b.l.Run(session.Request{ID: sA, Model: "opus", Effort: "medium", Brief: []byte("b")}); !errors.Is(err, session.ErrStalled) {
		t.Fatalf("first run: %v", err)
	}
	before, _ := b.s.Session(sA)
	if err := testkit.SetHerdr(b.fakes, "close_tab", before.Tab); err != nil {
		t.Fatal(err)
	}
	if err := testkit.SetHerdr(b.fakes, "ready_on_start", "true"); err != nil {
		t.Fatal(err)
	}
	if err := b.l.Run(session.Request{ID: sA, Model: "opus", Effort: "medium"}); err != nil {
		t.Fatalf("resume: %v", err)
	}
	after, _ := b.s.Session(sA)
	st, _ := testkit.HerdrState(b.fakes)
	if len(st.Tabs) != 1 || after.Tab == before.Tab || after.Pane == before.Pane || after.Run != 2 || after.Phase != "ready" {
		t.Fatalf("after resume: tabs %d, %+v", len(st.Tabs), after)
	}
}

func TestRelaunchOfAnExitedSession(t *testing.T) {
	// covers: R-SESS-047, R-SESS-048, R-SESS-051
	b := newBench(t)
	b.register(t, sA, "root")
	if err := b.l.Run(session.Request{ID: sA, Model: "opus", Effort: "medium", Brief: []byte("b")}); err != nil {
		t.Fatal(err)
	}
	first, _ := b.s.Session(sA)
	if err := testkit.SetHerdr(b.fakes, "exit", first.Agent); err != nil {
		t.Fatal(err)
	}
	if err := b.s.NewRun(sA, "opening-tab", "frontier", "medium"); err != nil {
		t.Fatal(err)
	}
	if err := b.l.Run(session.Request{ID: sA, Model: "fable", Effort: "medium", Resume: true}); err != nil {
		t.Fatal(err)
	}
	second, _ := b.s.Session(sA)
	st, _ := testkit.HerdrState(b.fakes)
	args := strings.Join(st.Agents[len(st.Agents)-1].Args, " ")
	if second.Run != 2 || second.Pane == first.Pane || second.Tier != "frontier" || !strings.Contains(args, "--resume "+first.ClientSession) ||
		!strings.Contains(args, "--model fable") || second.ClientSession != first.ClientSession {
		t.Fatalf("relaunch: %+v args %q", second, args)
	}
}
```
These tests need two small testkit helpers; add to `internal/testkit/herdr.go`:
```go
// HerdrState returns the fake herdr's state for assertions in Go tests.
func HerdrState(fakes string) (herdrState, error) { return FakeHerdr{Path: filepath.Join(fakes, "herdr.json")}.load() }

// SetHerdr changes the fake herdr from a Go test: key is fail_next, ready_on_start, close_tab
// (a tab id) or exit (an agent name).
func SetHerdr(fakes, key, value string) error {
	h := FakeHerdr{Path: filepath.Join(fakes, "herdr.json")}
	st, err := h.load()
	if err != nil {
		return err
	}
	switch key {
	case "fail_next":
		st.FailNext = value
	case "ready_on_start":
		st.ReadyOnStart = value == "true"
	case "close_tab":
		if _, err := h.dispatch(&st, "tab close", []string{value}); err != nil {
			return err
		}
	case "exit":
		for i := range st.Agents {
			if st.Agents[i].Name == value {
				st.Agents[i].Status = "gone"
			}
		}
	default:
		return fmt.Errorf("SetHerdr: unknown key %s", key)
	}
	return saveJSON(h.Path, st)
}
```
Export the state type as `HerdrState` fields need to be readable: rename `herdrState` → `HerdrFake` with exported fields `Tabs []HerdrTab`, `Agents []HerdrAgent`, `Typed [][2]string` (and the tab/agent structs exported likewise). Keep the JSON tags.

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/session/`
Expected: FAIL — `session.Launcher` undefined.

- [ ] **Step 3: The wait**

`internal/session/wait.go`:
```go
package session

import (
	"errors"
	"time"
)

// ErrStalled is returned when the ready mark does not arrive within the timeout (R-SESS-033).
var ErrStalled = errors.New("the session did not report ready")

// waitFor polls ready every tick until it is true or timeout passes. Timers only, so synctest
// drives it.
func waitFor(ready func() (bool, error), timeout, tick time.Duration) error {
	deadline := time.After(timeout)
	for {
		ok, err := ready()
		if err != nil {
			return err
		}
		if ok {
			return nil
		}
		select {
		case <-deadline:
			return ErrStalled
		case <-time.After(tick):
		}
	}
}
```

- [ ] **Step 4: The launcher**

`internal/session/launch.go`:
```go
package session

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// Exec runs a program; cli.SystemExec in production, the fake in tests.
type Exec func(dir, name string, args ...string) ([]byte, error)

// Launcher runs the launch phases of a registered session (design §5 "Sessions"). Primary is
// the primary checkout; worktrees go under its .worktrees/. Every step is idempotent so a
// stalled launch resumes from its stored phase (R-SESS-045).
type Launcher struct {
	Store                      *store.Store
	Runtime                    Runtime
	Client                     Client
	Exec                       Exec
	Primary, Project, Sessions string
	Copy, Link                 []string
	ReadyTimeout               time.Duration
	Now                        func() time.Time
	Log                        io.Writer
}

// Request is one run of the phases: the resolved model and effort, the brief to write (nil
// keeps the existing one), Resume for the relaunch of an exited session (R-SESS-047).
type Request struct {
	ID, Model, Effort, FallbackModel, Ness string
	Brief                                  []byte
	Resume                                 bool
}

// Run executes the phases after the stored one, persisting each before its side effect
// (R-SESS-031). It returns ErrStalled when the ready mark does not arrive (R-SESS-033).
func (l *Launcher) Run(r Request) error {
	sess, err := l.Store.Session(r.ID)
	if err != nil {
		return err
	}
	if sess.Adopted || !sess.ReadyAt.IsZero() && sess.Phase == store.PhaseReady {
		return nil
	}
	steps := map[string]func(*store.Session, Request) error{
		"creating-worktree": l.createWorktree, "linking-files": l.linkFiles, "writing-brief": l.writeBrief,
		"opening-tab": l.openTab, "starting-agent": l.startAgent, "sending-brief": l.receipt,
		"waiting-ready": l.waitReady,
	}
	for _, phase := range NextPhases(sess.Phase) {
		if phase == store.PhaseReady {
			return l.Store.SetPhase(r.ID, store.PhaseReady, l.Now())
		}
		if err := l.Store.SetPhase(r.ID, phase, l.Now()); err != nil {
			return err
		}
		if err := steps[phase](&sess, r); err != nil {
			return fmt.Errorf("%s: %w", phase, err)
		}
	}
	return nil
}

func (l *Launcher) say(format string, a ...any) {
	if l.Log != nil {
		_, _ = fmt.Fprintf(l.Log, format+"\n", a...)
	}
}

func (l *Launcher) worktreeDir(s *store.Session) string { return filepath.Join(l.Primary, s.Worktree) }

// createWorktree adds the branch from the base ref and the worktree under the primary checkout
// (R-SESS-020, R-SESS-021). An existing worktree is kept.
func (l *Launcher) createWorktree(s *store.Session, _ Request) error {
	dir := l.worktreeDir(s)
	if _, err := os.Stat(filepath.Join(dir, ".git")); err == nil {
		return nil
	}
	if _, err := l.Exec(l.Primary, "git", "worktree", "prune"); err != nil {
		return err
	}
	args := []string{"worktree", "add", "-q"}
	if _, err := l.Exec(l.Primary, "git", "rev-parse", "--verify", "--quiet", "refs/heads/"+s.Branch); err == nil {
		args = append(args, dir, s.Branch) // the branch survived an earlier attempt
	} else {
		args = append(args, "-b", s.Branch, dir, s.Base)
	}
	_, err := l.Exec(l.Primary, "git", args...)
	return err
}

// linkFiles symlinks worktree.link and copies worktree.copy from the primary checkout
// (R-SESS-034). A source missing in the primary is skipped with a note.
func (l *Launcher) linkFiles(s *store.Session, _ Request) error {
	dir := l.worktreeDir(s)
	for _, p := range l.Link {
		src, dst := filepath.Join(l.Primary, p), filepath.Join(dir, p)
		if _, err := os.Lstat(dst); err == nil {
			continue
		}
		if _, err := os.Stat(src); err != nil {
			l.say("ness: worktree.link %s is not in the primary checkout; skipped", p)
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
			return err
		}
		if err := os.Symlink(src, dst); err != nil {
			return err
		}
	}
	for _, p := range l.Copy {
		src, dst := filepath.Join(l.Primary, p), filepath.Join(dir, p)
		if _, err := os.Lstat(dst); err == nil {
			continue
		}
		data, err := os.ReadFile(src) //nolint:gosec // a path from the project's own config
		if errors.Is(err, fs.ErrNotExist) {
			l.say("ness: worktree.copy %s is not in the primary checkout; skipped", p)
			continue
		}
		if err != nil {
			return err
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(dst, data, 0o600); err != nil {
			return err
		}
	}
	return nil
}

// writeBrief puts the brief at <worktree>/<record>/brief.md (R-SESS-022, R-SESS-049).
func (l *Launcher) writeBrief(s *store.Session, r Request) error {
	path := filepath.Join(l.worktreeDir(s), s.Record, "brief.md")
	if r.Brief == nil {
		if _, err := os.Stat(path); err == nil {
			return nil
		}
		return fmt.Errorf("no brief at %s; pass --brief FILE", path)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, r.Brief, 0o600)
}

// openTab opens the session's tab with NESS_SESSION and NESS_RUN in its environment
// (R-SESS-030, R-SESS-037). A tab that still exists is reused; a dead one is replaced.
func (l *Launcher) openTab(s *store.Session, _ Request) error {
	if s.Tab != "" {
		tabs, err := l.Runtime.Tabs()
		if err != nil {
			return err
		}
		if slices.ContainsFunc(tabs, func(t Tab) bool { return t.ID == s.Tab }) {
			return nil
		}
	}
	run := s.Run + 1
	tab, err := l.Runtime.OpenTab(TabSpec{Dir: l.worktreeDir(s), Label: s.Title,
		Env: map[string]string{"NESS_SESSION": s.ID, "NESS_RUN": strconv.Itoa(run)}})
	if err != nil {
		return err
	}
	if err := l.Store.BindTab(s.ID, tab.Pane, tab.ID, run); err != nil {
		return err
	}
	s.Pane, s.Tab, s.Run, s.Agent = tab.Pane, tab.ID, run, ""
	return nil
}

// startAgent starts the client under a unique herdr name with the prompt argument
// (R-SESS-025, R-SESS-027, R-SESS-035, R-SESS-036). A live agent in the pane is kept.
func (l *Launcher) startAgent(s *store.Session, r Request) error {
	if s.Agent != "" {
		if st, err := l.Runtime.State(s.Pane); err == nil && st.Live {
			return nil
		}
	}
	live, err := l.Runtime.Agents()
	if err != nil {
		return err
	}
	name := AgentName(l.Project, s.Scope, s.ID, live)
	clientSession := s.ClientSession
	if clientSession == "" || !r.Resume {
		clientSession = l.Client.NewSessionID()
	}
	spec := LaunchSpec{Dir: l.worktreeDir(s), Model: r.Model, Effort: r.Effort, Prompt: Prompt(s.ID, s.Scope, s.Record),
		SessionID: clientSession, FallbackModel: r.FallbackModel, Ness: r.Ness, Resume: r.Resume && s.ClientSession != ""}
	if err := l.Store.BindAgent(s.ID, name, clientSession); err != nil {
		return err
	}
	s.Agent, s.ClientSession = name, clientSession
	return l.Runtime.StartAgent(AgentSpec{Name: name, Pane: s.Pane, Kind: l.Client.Name(), Args: l.Client.Args(spec)})
}

// receipt checks the pane once after the start; an ambiguous or unreadable screen is not a
// refusal (R-SESS-046, F5.3). Only a shell that says the client is missing fails the launch.
func (l *Launcher) receipt(s *store.Session, _ Request) error {
	screen, err := l.Runtime.ReadScreen(s.Pane)
	if err != nil {
		l.say("ness: pane %s could not be read after the start; continuing", s.Pane)
		return nil
	}
	if strings.Contains(screen, l.Client.Name()+": command not found") {
		return fmt.Errorf("%s is not on the tab's PATH", l.Client.Name())
	}
	return nil
}

// waitReady waits for the mark the SessionStart hook writes (R-SESS-032, R-SESS-033).
func (l *Launcher) waitReady(s *store.Session, _ Request) error {
	return waitFor(func() (bool, error) {
		cur, err := l.Store.Session(s.ID)
		if err != nil {
			return false, err
		}
		return !cur.ReadyAt.IsZero(), nil
	}, l.ReadyTimeout, 250*time.Millisecond)
}
```

- [ ] **Step 5: Run and commit**

Run: `go test ./internal/session/ ./internal/testkit/ && make lint`
Expected: PASS. `TestLaunchStallsWithoutTheMark` and `TestResumeAfterTabGone` take under a second each (300 ms and 200 ms timeouts on real timers; the wait itself is pinned by synctest in `wait_test.go`).

```bash
git add internal/session internal/testkit/herdr.go
git commit -m "feat(session): the launch state machine with persisted phases, resume and the ready wait"
```

---

### Task 7: `ness open`

**Files:**
- Create: `internal/cli/open.go`, `internal/cli/atoms.go`, `internal/cli/testdata/script/open.txtar`, `open_refusals.txtar`, `open_nested.txtar`, `open_here.txtar`, `open_resume.txtar`, `open_env.txtar`
- Modify: `internal/cli/project.go` (`projectConfig`, `primaryRoot`), `internal/cli/cli.go` (register)

**Interfaces:**
- Consumes: `session.Launcher`, `session.Env`, `session.Policy`, `herdr.Runtime`, `claude.Client`, `codex.Client`, `textpolicy.Policy`, `store.AddLaunch/NewRun/SetTier/Children`, `withStore`, `caller`.
- Produces: `projectConfig(d Deps, p *project) (*config.Config, error)` (profiles' defaults applied), `primaryRoot(p *project) (string, error)` (parent of the git common dir), `buildRuntime(d Deps) session.Runtime`, `buildClient(d Deps, name string) session.Client`, `skillRoots() []string`, `newOpenCmd(d Deps) *cobra.Command`. Output: one line `opened <id> scope <scope> branch <branch> record <record>` (for `--here`: `adopted <id> scope <scope> branch <branch> record <record>`; for `--resume`: `resumed <id> run <n>`).

- [ ] **Step 1: Write the failing scripts**

`internal/cli/testdata/script/open.txtar`:
```
# ness open launches a root and a child: worktree and branch under the primary checkout, the
# child's branch from the manager's, nested record dirs with the brief bytes, a herdr tab titled
# from --title, the agent named <project>-<scope>, the one-line prompt as one argv element with
# --session-id and explicit effort, NESS_SESSION/NESS_RUN in the tab, nothing typed, the row
# ready. The economy tier edited in ness.toml reaches the launch.
# covers: R-SESS-020, R-SESS-021, R-SESS-022, R-SESS-025, R-SESS-026, R-SESS-027, R-SESS-028, R-SESS-030, R-SESS-032, R-SESS-035, R-SESS-037, R-SESS-043, R-CFG-041, R-MOD-003, R-MOD-004, R-MOD-011, R-MOD-040, R-SESS-008
cd repo
exec ness init --gate true --base main --yes
cp ../ness.toml .ness/ness.toml
exec git add -A
exec git commit -q -m 'ness init'
# A root: the operator opens it with --root.
exec ness open --root --scope prog --tier standard --brief ../brief.md
stdout '^opened s-[0-9a-f]{12} scope prog branch session/prog-[0-9a-f]{12} record rec/prog$'
store sessions 1
store last-session ROOT
exec git worktree list
stdout 'repo/\.worktrees/prog-[0-9a-f]{12}'
exec git branch --list 'session/prog-*'
stdout 'session/prog-'
fake herdr tabs 1
fake herdr tab prog
fake herdr agents '^infra-lab-prog$'
fake herdr typed 0
fake herdr env NESS_RUN=1
fake herdr args '^--session-id [0-9a-f-]{36} --model opus --effort medium You are ness session s-[0-9a-f]{12} \(prog\)\. Read rec/prog/brief\.md, then load ness-entry\.$'
exists .worktrees
# The brief landed in the worktree's record dir, byte for byte.
exec sh -c 'cmp ../brief.md .worktrees/prog-*/rec/prog/brief.md'
# A child from inside the root: economy tier (edited to haiku), title given, effort overridden.
exec sh -c 'cd .worktrees/prog-* && NESS_SESSION=$ROOT ness open --scope sync --tier economy --effort high --title "Sync work" --brief ../../../brief.md'
stdout '^opened s-[0-9a-f]{12} scope sync branch session/sync-[0-9a-f]{12} record rec/prog/sync$'
store sessions 2
fake herdr tabs 2
fake herdr tab 'Sync work'
fake herdr agents '^infra-lab-sync$'
fake herdr args '--model haiku --effort high '
fake herdr env NESS_SESSION=s-
# The child's branch starts from the root's branch, under the primary checkout's .worktrees.
exec sh -c 'cd .worktrees/sync-* && git merge-base --is-ancestor $(git -C ../prog-* rev-parse HEAD) HEAD'
exec sh -c 'test -f .worktrees/sync-*/rec/prog/sync/brief.md'
! exists .worktrees/prog-*/.worktrees
# The frontier tier passes medium explicitly; the stored tier keeps the table's effort.
exec ness open --root --scope design --tier frontier --brief ../brief.md
fake herdr args '--model fable --effort medium '
# A session cannot open a root (R-SESS-041) — see open_refusals; the tab title defaults to the scope.
fake herdr tab design

-- ness.toml --
ness = "0.1"
project = "infra-lab"
base = "main"
gate = "true"
sessions = "rec"
[tiers.economy]
claude = "haiku/medium"
-- brief.md --
# Brief

Do the thing.
```
`store sessions` and `fake herdr …` come from Tasks 1 and 2. `store last-session VAR` is new: add it to `storeSubs` in this task (it sets `$VAR` to the id of the newest session, by creation order):
```go
	"last-session": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		all, err := s.Sessions(true)
		ts.Check(err)
		if neg || len(args) != 1 || len(all) == 0 {
			ts.Fatalf("usage: store last-session VAR (with at least one session)")
		}
		ts.Setenv(args[0], all[len(all)-1].ID)
	},
```

`internal/cli/testdata/script/open_refusals.txtar`:
```
# Every environment and flag check refuses with exit 4 before any side effect: no worktree,
# branch, file, store row or tab. Flag errors are usage (exit 2).
# covers: R-SESS-001, R-SESS-002, R-SESS-003, R-SESS-004, R-SESS-005, R-SESS-010, R-SESS-017, R-SESS-023, R-SESS-024, R-SESS-039, R-SESS-040, R-SESS-041, R-SESS-043, R-MOD-013, R-MOD-031, R-MOD-042, R-CFG-036, R-CFG-044
cd repo
exec ness init --gate true --base main --yes
cp ../ness.toml .ness/ness.toml
exec git add -A
exec git commit -q -m 'ness init'
snap clean
# Usage errors.
exits 2 ness open --root --tier standard --brief ../brief.md
stderr -- '--scope'
exits 2 ness open --root --scope Sync_1 --tier standard --brief ../brief.md
stderr 'scope'
exits 2 ness open --root --scope x --tier standard --effort huge --brief ../brief.md
stderr 'claude'
exits 2 ness open --root --scope x --tier standard --model opus --brief ../brief.md
stderr 'unknown flag'
# Precondition refusals.
exits 4 ness open --root --scope x --brief ../brief.md
stderr -- '--tier'
exits 4 ness open --root --scope x --tier ultra --brief ../brief.md
stderr 'economy, frontier, standard'
exits 4 ness open --scope x --tier standard --here --brief ../brief.md
stderr -- '--here needs --root'
exits 4 ness open --scope x --tier standard --base main --brief ../brief.md
stderr -- '--base needs --root'
exits 4 ness open --root --scope x --tier standard --client codex --brief ../brief.md
stderr 'experimental'
exits 4 ness open --root --scope x --tier standard --brief ../missing.md
stderr 'missing.md'
# A session may not open a root; the operator may not open a child.
env NESS_SESSION=s-aaaaaaaaaaaa
store add-session s-aaaaaaaaaaaa prog
exits 4 ness open --root --scope x --tier standard --brief ../brief.md
stderr 'only the operator opens a root'
env NESS_SESSION=
exits 4 ness open --scope x --tier standard --brief ../brief.md
stderr 'a child needs a manager'
# Environment checks, each named with its fix.
fake missing herdr
exits 4 ness open --root --scope x --tier standard --brief ../brief.md
stderr 'herdr is required'
! stderr 'tmux'
cp ../fakes-reset.json $NESS_TEST_FAKES/exec.json
fake herdr version 0.5.0
exits 4 ness open --root --scope x --tier standard --brief ../brief.md
stderr '0\.5\.0'
stderr '0\.8\.2'
fake herdr version 0.8.2
fake output 'claude --version' '1.0.99 (Claude Code)'
exits 4 ness open --root --scope x --tier standard --brief ../brief.md
stderr '1\.0\.99'
stderr '2\.1\.294'
fake output 'claude --version' '2.1.294 (Claude Code)'
cp ../tiers-bad.toml .ness/ness.toml
exits 4 ness open --root --scope x --tier standard --brief ../brief.md
stderr 'tier frontier'
stderr 'best'
cp ../ness.toml .ness/ness.toml
mkdir $HOME/.claude/skills/deploy
mkdir .agents/skills/deploy
exits 4 ness open --root --scope x --tier standard --brief ../brief.md
stderr '\.claude/skills/deploy'
stderr '\.agents/skills/deploy'
rm $HOME/.claude/skills/deploy
rm .agents/skills/deploy
# A fake router below the threshold refuses and asks for --tier; at the threshold it is accepted.
env NESS_TEST_ROUTER=0.79
exits 4 ness open --root --scope x --brief ../brief.md
stderr -- '--tier'
env NESS_TEST_ROUTER=
# Nothing happened.
samesnap clean
store sessions 1
fake herdr tabs 0
exec git worktree list
! stdout worktrees
exec git branch --list 'session/*'
! stdout .
# At the threshold the router's tier is used and its reason recorded.
env NESS_TEST_ROUTER=0.8
exec ness open --root --scope x --brief ../brief.md
store last-session X
store session $X tier=standard
env NESS_TEST_ROUTER=
# A second open sibling with the same scope under the same manager is refused (Review Focus 4).
exits 4 ness open --root --scope x --tier standard --brief ../brief.md
stderr 'scope x is already open'
store sessions 2

-- ness.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
-- tiers-bad.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
[tiers.frontier]
claude = "best/medium"
-- brief.md --
b
-- fakes-reset.json --
{"missing":[],"outputs":{"claude --version":"2.1.294 (Claude Code)\n","codex --version":"codex-cli 0.157.1\n","codex debug models":"{\"models\":[{\"slug\":\"gpt-6-luna\"},{\"slug\":\"gpt-6.1-sol\"},{\"slug\":\"astra\"}]}\n"},"fail":{},"calls":[]}
```
`NESS_TEST_ROUTER` is a test-only seam of the script harness: when set, `script_test.go` wires `Deps.Policy = testkit.FakeRouter{Tier: "standard", Confidence: <value>}` (R-MOD-042, R-MOD-043). Add `Policy session.Policy` to `Deps` (nil → `textpolicy.Policy{}`); production never sets it.

`internal/cli/testdata/script/open_nested.txtar` (Review Focus 1):
```
# Opening from inside a worktree (a manager's cwd) puts the new worktree under the primary
# checkout and uses the shared store; the record dir nests three deep for a grandchild.
# covers: R-SESS-020, R-SESS-022
cd repo
exec ness init --gate true --base main --yes
exec git add -A
exec git commit -q -m 'ness init'
exec ness open --root --scope r --tier standard --brief ../b.md
store last-session R
cd .worktrees
exec sh -c 'cd r-* && NESS_SESSION=$R ness open --scope c --tier standard --brief ../../../b.md'
store last-session C
exec sh -c 'cd c-* && NESS_SESSION=$C ness open --scope g --tier economy --brief ../../../b.md'
store sessions 3
exec sh -c 'ls -d r-* c-* g-* | wc -l'
stdout '^3$'
exec sh -c 'test -f g-*/docs/sessions/r/c/g/brief.md'
exec sh -c 'test ! -e c-*/.worktrees'
cd ..
exec git worktree list
stdout -count=3 'worktrees/'

-- b.md --
b
```

`internal/cli/testdata/script/open_here.txtar`:
```
# open --root --here registers the running conversation: no tab, no worktree, bound to the
# current pane and transcript, ready at once, on the checkout's branch; needs the pane and
# session variables of a herdr-hosted Claude Code session.
# covers: R-SESS-038, R-SESS-039, R-SESS-041, R-HOOK-008
cd repo
exec ness init --gate true --base main --yes
exec git add -A
exec git commit -q -m 'ness init'
exits 4 ness open --root --here --scope prog --tier standard --brief ../b.md
stderr 'HERDR_PANE_ID'
env HERDR_PANE_ID=w1:p7
env HERDR_TAB_ID=w1:t7
env CLAUDE_CODE_SESSION_ID=0bad0bad-0000-4000-8000-000000000000
exec ness open --root --here --scope prog --tier standard --brief ../b.md
stdout '^adopted s-[0-9a-f]{12} scope prog branch main record docs/sessions/prog$'
store last-session R
store session $R adopted=true phase=ready pane=w1:p7 tab=w1:t7 client_session=0bad0bad-0000-4000-8000-000000000000 worktree= branch=main
! store session $R ready=false
fake herdr tabs 0
exec git worktree list
! stdout worktrees
exists docs/sessions/prog/brief.md
# After compaction the adopted root keeps its identity and is not told to detach.
env NESS_SESSION=$R
stdin ../compact.json
exec ness hook session-start
stdout '^ness session s-[0-9a-f]{12} scope prog manager operator phase ready$'
! stdout 'do not attach'
# A root's child opened from the adopted root starts from main under .worktrees.
exec ness open --scope c --tier standard --brief ../b.md
exec git worktree list
stdout 'worktrees/c-'

-- b.md --
b
-- compact.json --
{"hook_event_name":"SessionStart","source":"compact","session_id":"0bad0bad-0000-4000-8000-000000000000"}
```

`internal/cli/testdata/script/open_resume.txtar`:
```
# --resume continues a stalled launch from its phase, relaunches an exited session resuming its
# transcript one tier up, replaces the brief, and refuses a running session.
# covers: R-SESS-033, R-SESS-045, R-SESS-047, R-SESS-049, R-SESS-050, R-SESS-051
cd repo
exec ness init --gate true --base main --yes
cp ../ness.toml .ness/ness.toml
exec git add -A
exec git commit -q -m 'ness init'
# Stall: the fake never writes the mark; open exits 4 naming --resume and status shows it.
fake herdr ready-on-start false
env NESS_TEST_NOW=2026-10-08T10:00:00Z
exits 4 ness open --root --scope prog --tier standard --brief ../b1.md
stderr 'ness open --resume s-[0-9a-f]{12}'
store last-session R
store session $R phase=waiting-ready ready=false
# (status shows it as `stalled launch`: status.txtar)
# Resume: one worktree, one tab, the mark arrives, brief replaced.
fake herdr ready-on-start true
exec ness open --resume $R --brief ../b2.md
stdout '^resumed s-[0-9a-f]{12} run 1$'
store session $R phase=ready
fake herdr tabs 1
exec git worktree list
stdout -count=1 'worktrees/'
exec sh -c 'cmp ../b2.md .worktrees/prog-*/docs/sessions/prog/brief.md'
# Running: refused, no new tab.
exits 4 ness open --resume $R
stderr 'running'
fake herdr tabs 1
# Exited: relaunched in a new run, resuming the transcript, one tier up.
fake herdr exit p-prog
exec ness open --resume $R --tier frontier
stdout '^resumed s-[0-9a-f]{12} run 2$'
store session $R run=2 tier=frontier phase=ready
fake herdr tabs 2
fake herdr args '^--resume [0-9a-f-]{36} --model fable --effort medium '
# A closed session cannot be resumed.
store close-session $R
exits 4 ness open --resume $R
stderr 'closed'

-- ness.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
[launch]
ready_timeout = "200ms"
-- b1.md --
one
-- b2.md --
two
```

`internal/cli/testdata/script/open_env.txtar`:
```
# --client codex with the opt-in sends the launch to the codex atom with its trust, update and
# hook flags; --fallback-model is passed only when configured; the router's reason is stored.
# covers: R-SESS-029, R-SESS-113, R-HOOK-018, R-MOD-044, R-MOD-043, R-MOD-030
cd repo
exec ness init --gate true --base main --yes
cp ../ness.toml .ness/ness.toml
exec git add -A
exec git commit -q -m 'ness init'
exec ness open --root --scope a --tier standard --brief ../b.md
fake herdr args '--fallback-model sonnet '
store last-session A
store session $A client=claude
exec ness open --root --scope b --tier economy --client codex --brief ../b.md
fake herdr args '^-m gpt-6-luna -c model_reasoning_effort="low" -c projects\.".*/\.worktrees/b-[0-9a-f]{12}"\.trust_level="trusted" -c check_for_update_on_startup=false --enable codex_hooks --dangerously-bypass-hook-trust -c hooks\.SessionStart=.*ness hook session-start.* -c hooks\.PreToolUse=.*ness hook pre-tool.* You are ness session '
! fake herdr args 'fallback'
store last-session B
store session $B client=codex client_session=
# Codex ids come from codex debug models: without astra the frontier row refuses.
fake output 'codex debug models' '{"models":[{"slug":"gpt-6-luna"}]}'
exits 4 ness open --root --scope c --tier economy --client codex --brief ../b.md
stderr 'tier frontier'
stderr 'astra'

-- ness.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
experimental = ["codex"]
[models]
fallback_model = "sonnet"
-- b.md --
b
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run 'TestScripts/open'`
Expected: FAIL — `unknown command "open"`.

- [ ] **Step 3: Project helpers and atom construction**

Append to `internal/cli/project.go`:
```go
// projectConfig is the project's configuration with the applied profiles' defaults (R-PROF-008).
func projectConfig(d Deps, p *project) (*config.Config, error) {
	base, err := p.file.Config(nil)
	if err != nil {
		return nil, refuse("%v", err)
	}
	res, err := loadProfiles(d, p, base.Profiles)
	if err != nil {
		return nil, err
	}
	cfg, err := p.file.Config(res.Defaults)
	if err != nil {
		return nil, refuse("%v", err)
	}
	return cfg, nil
}

// primaryRoot is the primary checkout: the parent of the git common dir, the same from a
// worktree (R-SESS-020).
func primaryRoot(p *project) (string, error) {
	path, err := store.Locate(p.root)
	if err != nil {
		return "", refuse("%v", err)
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(path))), nil
}
```
(`store.Locate` returns `<common>/ness/state.sqlite3`; three `Dir`s up is the checkout.)

`internal/cli/atoms.go`:
```go
package cli

import (
	"os"

	"github.com/enea-scaccabarozzi/ness/internal/atom/claude"
	"github.com/enea-scaccabarozzi/ness/internal/atom/codex"
	"github.com/enea-scaccabarozzi/ness/internal/atom/herdr"
	"github.com/enea-scaccabarozzi/ness/internal/atom/textpolicy"
	"github.com/enea-scaccabarozzi/ness/internal/session"
)

// The atoms are built here, the only place in cli that names an implementation; the commands
// see the seams.

func buildRuntime(d Deps) session.Runtime { return herdr.Runtime{Exec: d.exec()} }

func buildClient(d Deps, name string) session.Client {
	if name == "codex" {
		return &codex.Client{Exec: d.exec()}
	}
	return claude.Client{Exec: d.exec()}
}

func buildPolicy(d Deps) session.Policy {
	if d.Policy != nil {
		return d.Policy
	}
	return textpolicy.Policy{}
}

// skillRoots are the global skill directories of this user (R-SESS-006).
func skillRoots() []string {
	home, err := os.UserHomeDir()
	if err != nil {
		home = "/nonexistent"
	}
	return session.SkillRoots(home)
}
```
Add `Policy session.Policy` to `Deps` (doc: "Policy is the model policy; nil means the text policy. Tests inject a router."). In `script_test.go`, when `NESS_TEST_ROUTER` is set, parse it as a float and set `d.Policy = testkit.FakeRouter{Tier: "standard", Confidence: v}`.

- [ ] **Step 4: The command**

`internal/cli/open.go`:
```go
package cli

import (
	"errors"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/config"
	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

type openOpts struct {
	scope, tier, brief, title, client, effort, base, resume string
	root, here                                               bool
}

func newOpenCmd(d Deps) *cobra.Command {
	var o openOpts
	cmd := &cobra.Command{
		Use:   "open",
		Short: "Launch a child or a root session, or resume one",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return runOpen(d, o) },
	}
	f := cmd.Flags()
	f.StringVar(&o.scope, "scope", "", "the session's scope `NAME` (required; [a-z][a-z0-9-]{0,31})")
	f.StringVar(&o.tier, "tier", "", "tier from the project's tier table (required with the text policy)")
	f.StringVar(&o.brief, "brief", "", "brief `FILE`, copied to the session's record directory")
	f.StringVar(&o.title, "title", "", "tab title (default: the scope)")
	f.StringVar(&o.client, "client", "", "client for this launch: claude or codex (default: the configured client)")
	f.StringVar(&o.effort, "effort", "", "effort for this launch (default: the tier's)")
	f.StringVar(&o.base, "base", "", "base `REF` of a root (default: the configured base)")
	f.StringVar(&o.resume, "resume", "", "relaunch an exited session `ID` or continue its stalled launch")
	f.BoolVar(&o.root, "root", false, "open a root (operator only)")
	f.BoolVar(&o.here, "here", false, "with --root: register this running conversation as the root")
	return cmd
}

// launchEnv is everything an open needs after the project loaded.
type launchEnv struct {
	p       *project
	cfg     *config.Config
	primary string
	rt      session.Runtime
	client  session.Client
}

func runOpen(d Deps, o openOpts) error {
	if o.resume == "" {
		if o.scope == "" {
			return usage("--scope is required")
		}
		if !session.ScopeRe.MatchString(o.scope) {
			return usage("--scope %q: a scope matches [a-z][a-z0-9-]{0,31}", o.scope)
		}
	}
	if o.here && !o.root {
		return refuse("--here needs --root: only a root is registered from a running conversation")
	}
	if o.base != "" && !o.root {
		return refuse("--base needs --root: a child starts from its manager's branch")
	}
	brief, err := readBrief(o.brief)
	if err != nil {
		return err
	}
	return withStore(d, func(s *store.Store, c caller) error {
		env, err := loadLaunchEnv(d, o)
		if err != nil {
			return err
		}
		if o.resume != "" {
			return runResume(d, s, c, env, o, brief)
		}
		return runLaunch(d, s, c, env, o, brief)
	})
}

func readBrief(arg string) ([]byte, error) {
	if arg == "" {
		return nil, nil
	}
	data, err := os.ReadFile(arg) //nolint:gosec // the caller names its own file
	if err != nil {
		return nil, refuse("--brief %s: %v", arg, err)
	}
	return data, nil
}

func loadLaunchEnv(d Deps, o openOpts) (*launchEnv, error) {
	p, err := loadProject(d)
	if err != nil {
		return nil, err
	}
	cfg, err := projectConfig(d, p)
	if err != nil {
		return nil, err
	}
	name := cfg.Client
	if o.client != "" {
		name = o.client
	}
	if name != "claude" && name != "codex" {
		return nil, usage("--client %s: accepted values are claude, codex", name)
	}
	if name == "codex" && !slices.Contains(cfg.Experimental, "codex") {
		return nil, refuse(`the codex client needs experimental = ["codex"] in .ness/ness.toml`)
	}
	client := buildClient(d, name)
	if o.effort != "" && !slices.Contains(client.Efforts(), o.effort) {
		return nil, usage("--effort %s: %s accepts %s", o.effort, name, strings.Join(client.Efforts(), ", "))
	}
	primary, err := primaryRoot(p)
	if err != nil {
		return nil, err
	}
	env := &launchEnv{p: p, cfg: cfg, primary: primary, rt: buildRuntime(d), client: client}
	checks := session.Env{Runtime: env.rt, Client: client, Tiers: cfg.Tiers,
		ProjectSkills: filepath.Join(primary, ".agents", "skills"), SkillRoots: skillRoots()}
	if err := checks.Check(); err != nil {
		return nil, refuse("%v", err)
	}
	return env, nil
}

// choose asks the policy for the tier and effort (R-MOD-040..043); --effort overrides the
// tier's for this launch (R-MOD-011).
func choose(d Deps, env *launchEnv, o openOpts, role, parentTier string, brief []byte) (session.Choice, error) {
	ch, err := buildPolicy(d).Choose(session.Brief{Text: string(brief), Role: role, Client: env.client.Name(),
		ParentTier: parentTier, Tier: o.tier, Tiers: env.cfg.Tiers})
	if errors.Is(err, session.ErrNeedTier) {
		return ch, refuse("%v (valid tiers: %s)", err, strings.Join(slices.Sorted(tierNames(env.cfg.Tiers)), ", "))
	}
	if err != nil {
		return ch, refuse("%v", err)
	}
	if ch.Confidence < env.cfg.Models.RouterThreshold {
		return ch, refuse("the model policy is not confident (%.2f < %.2f): choose a tier with --tier",
			ch.Confidence, env.cfg.Models.RouterThreshold)
	}
	if o.effort != "" {
		ch.Effort = o.effort
	}
	return ch, nil
}

func tierNames(m map[string]config.Tier) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func runLaunch(d Deps, s *store.Store, c caller, env *launchEnv, o openOpts, brief []byte) error {
	var parent store.Session
	switch {
	case o.root && !c.operator():
		return refuse("only the operator opens a root; a session opens children with ness open --scope")
	case !o.root && c.operator():
		return refuse("a child needs a manager: run from a session, or open a root with --root")
	case !o.root:
		parent = c.sess
	}
	if o.here && (os.Getenv("HERDR_PANE_ID") == "" || os.Getenv("HERDR_TAB_ID") == "" || env.client.CurrentSession() == "") {
		return refuse("--here needs HERDR_PANE_ID, HERDR_TAB_ID and the client's session id: run it from inside the %s session in its herdr pane",
			env.client.Name())
	}
	siblings, err := s.Children(parent.ID)
	if err != nil {
		return err
	}
	if slices.ContainsFunc(siblings, func(x store.Session) bool { return x.Scope == o.scope }) {
		return refuse("scope %s is already open under %s; close it or choose another --scope", o.scope, displayName(parent.ID))
	}
	ch, err := choose(d, env, o, roleOf(o.root), parent.Tier, brief)
	if err != nil {
		return err
	}
	if brief == nil && !o.here {
		return refuse("--brief FILE is required for a new session")
	}
	id := session.NewID()
	l := store.Launch{ID: id, Parent: parent.ID, Scope: o.scope, Title: o.title, Tier: ch.Tier, Effort: ch.Effort,
		Client: env.client.Name(), RouterReason: ch.Reason, Adopted: o.here}
	if l.Title == "" {
		l.Title = o.scope
	}
	l.Record = path.Join(env.cfg.Sessions, o.scope)
	if !o.root {
		l.Record = path.Join(parent.Record, o.scope)
	}
	suffix := o.scope + "-" + strings.TrimPrefix(id, "s-")
	switch {
	case o.here:
		l.Base, l.Branch = currentBranch(env.p.root), currentBranch(env.p.root)
	case o.root:
		l.Base = env.cfg.Base
		if o.base != "" {
			l.Base = o.base
		}
		l.Worktree, l.Branch = ".worktrees/"+suffix, "session/"+suffix
	default:
		l.Base, l.Worktree, l.Branch = parent.Branch, ".worktrees/"+suffix, "session/"+suffix
	}
	if err := s.AddLaunch(l, d.Now()); err != nil {
		return err
	}
	if o.here {
		return adoptHere(d, s, env, l, brief)
	}
	model, _ := session.TierEffort(env.cfg.Tiers[ch.Tier], env.client.Name())
	err = launcher(d, s, env).Run(session.Request{ID: id, Model: model, Effort: ch.Effort,
		FallbackModel: env.cfg.Models.FallbackModel, Ness: executable(), Brief: brief})
	if errors.Is(err, session.ErrStalled) {
		return refuse("session %s did not report ready within %s (stalled launch); check its tab, then run ness open --resume %s",
			id, env.cfg.Launch.ReadyTimeout.Duration, id)
	}
	if err != nil {
		return err
	}
	say(d.Stdout, "opened %s scope %s branch %s record %s", id, o.scope, l.Branch, l.Record)
	return nil
}

func roleOf(root bool) string {
	if root {
		return "root"
	}
	return "child"
}

// adoptHere binds the registered root to the running conversation and writes its brief into
// the primary checkout (R-SESS-038).
func adoptHere(d Deps, s *store.Store, env *launchEnv, l store.Launch, brief []byte) error {
	if err := s.BindTab(l.ID, os.Getenv("HERDR_PANE_ID"), os.Getenv("HERDR_TAB_ID"), 1); err != nil {
		return err
	}
	if err := s.BindAgent(l.ID, "", env.client.CurrentSession()); err != nil {
		return err
	}
	if brief != nil {
		dir := filepath.Join(env.p.root, filepath.FromSlash(l.Record))
		if err := os.MkdirAll(dir, 0o750); err != nil {
			return err
		}
		if err := os.WriteFile(filepath.Join(dir, "brief.md"), brief, 0o600); err != nil {
			return err
		}
	}
	say(d.Stdout, "adopted %s scope %s branch %s record %s", l.ID, l.Scope, l.Branch, l.Record)
	return nil
}

func launcher(d Deps, s *store.Store, env *launchEnv) *session.Launcher {
	return &session.Launcher{Store: s, Runtime: env.rt, Client: env.client, Exec: d.exec(), Primary: env.primary,
		Project: env.cfg.Project, Sessions: env.cfg.Sessions, Copy: env.cfg.Worktree.Copy, Link: env.cfg.Worktree.Link,
		ReadyTimeout: env.cfg.Launch.ReadyTimeout.Duration, Now: d.Now, Log: d.Stderr}
}

// runResume continues a stalled launch or relaunches an exited session (R-SESS-045..051).
func runResume(d Deps, s *store.Store, c caller, env *launchEnv, o openOpts, brief []byte) error {
	sess, err := s.Session(o.resume)
	if errors.Is(err, store.ErrNoSession) {
		return refuse("%s is not a session of this project", o.resume)
	}
	if err != nil {
		return err
	}
	if sess.Closed {
		return refuse("session %s is closed", sess.ID)
	}
	if !c.operator() && sess.Parent != c.id {
		return refuse("only %s's manager or the operator resumes it", sess.ID)
	}
	if sess.Adopted {
		return refuse("session %s was adopted with --here and has no launch to resume", sess.ID)
	}
	live := false
	if sess.Pane != "" {
		if st, serr := env.rt.State(sess.Pane); serr == nil && st.Live {
			live = true
		}
	}
	ready := !sess.ReadyAt.IsZero()
	if ready && live {
		return refuse("session %s is running; close it or wait for it to exit before --resume", sess.ID)
	}
	tier, effort := sess.Tier, sess.Effort
	if o.tier != "" {
		ch, cerr := choose(d, env, o, roleOf(sess.Parent == store.Operator), "", nil)
		if cerr != nil {
			return cerr
		}
		tier, effort = ch.Tier, ch.Effort
	} else if o.effort != "" {
		effort = o.effort
	}
	if brief != nil {
		p := filepath.Join(env.primary, sess.Worktree, sess.Record, "brief.md")
		if werr := os.WriteFile(p, brief, 0o600); werr != nil && !ready {
			brief = nil // the worktree is not there yet; the launch writes it
		} else if werr != nil {
			return werr
		}
	}
	if ready { // exited: a new run that resumes the transcript (R-SESS-047)
		if env.client.NewSessionID() == "" {
			return refuse("%s sessions cannot be resumed: the client has no session id", env.client.Name())
		}
		if err := s.NewRun(sess.ID, "opening-tab", tier, effort); err != nil {
			return err
		}
	} else if err := s.SetTier(sess.ID, tier, effort); err != nil {
		return err
	}
	model, _ := session.TierEffort(env.cfg.Tiers[tier], env.client.Name())
	err = launcher(d, s, env).Run(session.Request{ID: sess.ID, Model: model, Effort: effort,
		FallbackModel: env.cfg.Models.FallbackModel, Ness: executable(), Brief: brief, Resume: ready})
	if errors.Is(err, session.ErrStalled) {
		return refuse("session %s did not report ready within %s; run ness open --resume %s again",
			sess.ID, env.cfg.Launch.ReadyTimeout.Duration, sess.ID)
	}
	if err != nil {
		return err
	}
	after, err := s.Session(sess.ID)
	if err != nil {
		return err
	}
	say(d.Stdout, "resumed %s run %s", sess.ID, strconv.Itoa(after.Run))
	return nil
}
```
Register `newOpenCmd(d)` in `newRoot`. The client used on resume is the stored one: in `loadLaunchEnv`, when `o.resume != ""` and `o.client == ""`, read the row's `Client` before building (move the `Session` lookup ahead, or build the env after `withStore` has the row; simplest: `runOpen` resolves `o.client` from the row when resuming before calling `loadLaunchEnv`).

- [ ] **Step 5: Run the scripts**

Run: `go test ./internal/cli/ -run 'TestScripts/open' -v`
Expected: PASS for all six scripts. (`open_here.txtar` asserts the hook's identity line with `phase ready`; that line is produced by Task 8, so run `open_here` again after Task 8 and expect it to pass then; until then its last `stdout` check before `! stdout 'do not attach'` fails and that is the only expected failure.)

- [ ] **Step 6: Lint, then commit**

Run: `make lint && go test ./...`
Expected: PASS. If `funlen` or `cyclop` flags `runLaunch`, split the `store.Launch` construction into `newLaunch(env, o, parent, id, ch) store.Launch`.

```bash
git add internal/cli
git commit -m "feat(cli): ness open launches, adopts and resumes sessions behind the environment checks"
```

---

### Task 8: Hook carry-ins: phase, shadowing, adopted root, `for`/`watch`

**Files:**
- Modify: `internal/cli/hook.go`, `internal/hook/guards.go`, `internal/hook/guards_test.go`, `internal/cli/testdata/script/hook_session_start.txtar`, `internal/cli/testdata/script/hook_pretool.txtar`
- Create: `internal/cli/testdata/script/hook_shadowing.txtar`

**Interfaces:**
- Consumes: `session.Shown`, `session.CheckShadowing`, `skillRoots()`, `primaryRoot`, `projectConfig`.
- Produces: the identity line `ness session <id> scope <scope> manager <manager> phase <shown phase>`; `session-start` exits 2 naming both paths when a global skill shadows a project skill (R-HOOK-005).

- [ ] **Step 1: Extend the scripts**

In `hook_session_start.txtar`, change the header comment's last sentence to "The identity line carries the derived phase; a second startup leaves the stored phase untouched." and the covers line to:
```
# covers: R-HOOK-003, R-HOOK-004, R-HOOK-006, R-HOOK-007, R-HOOK-009
```
Replace the three identity assertions and add the phase-unchanged checks:
```
store session s-bbbbbbbbbbbb ready=false
store phase s-bbbbbbbbbbbb waiting-ready
stdin ../startup.json
exec ness hook session-start
stdout '^ness session s-bbbbbbbbbbbb scope child-b manager s-aaaaaaaaaaaa phase ready$'
! stdout 'cheat sheet'
! stderr .
store session s-bbbbbbbbbbbb ready=2026-10-08T10:00:00Z phase=waiting-ready
# A second startup keeps the first mark and does not touch the phase (R-HOOK-009).
env NESS_TEST_NOW=2026-10-08T10:05:00Z
stdin ../startup.json
exec ness hook session-start
store session s-bbbbbbbbbbbb ready=2026-10-08T10:00:00Z phase=waiting-ready
```
and the root line becomes `stdout '^ness session s-aaaaaaaaaaaa scope root manager operator phase ready$'`. In `compact.txt`, the first line becomes `ness session s-bbbbbbbbbbbb scope child-b manager s-aaaaaaaaaaaa phase ready`. (The stored phase stays `waiting-ready` while the shown phase is `ready`: the launch, not the hook, moves the stored phase; Review Focus 2.)

`hook_shadowing.txtar`:
```
# For a bound session, session-start runs the shadowing check and blocks (exit 2) naming both
# paths on an exact match; a near name and a plugin skill pass; unbound, nothing runs.
# covers: R-HOOK-005, R-SESS-007
cd repo
exec ness init --gate true --base main --yes
store add-session s-aaaaaaaaaaaa root
mkdir .agents/skills/deploy
mkdir .agents/skills/x:deploy
mkdir $HOME/.claude/skills/deploy-prod
mkdir $HOME/.agents/skills/x:deploy
env NESS_SESSION=s-aaaaaaaaaaaa
stdin ../startup.json
exec ness hook session-start
stdout 'phase ready'
mkdir $HOME/.claude/skills/deploy
stdin ../startup.json
exits 2 ness hook session-start
stderr '\.claude/skills/deploy'
stderr '\.agents/skills/deploy'
env NESS_SESSION=
stdin ../startup.json
exec ness hook session-start
! stdout .
! stderr .

-- startup.json --
{"hook_event_name":"SessionStart","source":"startup","session_id":"c0ffee"}
```

In `hook_pretool.txtar`, add after the `loop_until.json` block (and `R-HOOK-011` already in its covers line):
```
stdin ../loop_for.json
exits 2 ness hook pre-tool
stderr 'end your turn'
stdin ../watch.json
exits 2 ness hook pre-tool
stderr 'end your turn'
stdin ../for_ok.json
exec ness hook pre-tool
! stderr .
```
with the files:
```
-- loop_for.json --
{"tool_name":"Bash","tool_input":{"command":"for i in 1 2 3; do ness inbox; sleep 10; done"}}
-- watch.json --
{"tool_name":"Bash","tool_input":{"command":"watch -n 5 ness status"}}
-- for_ok.json --
{"tool_name":"Bash","tool_input":{"command":"for f in *.md; do wc -l $f; done"}}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run 'TestScripts/hook'`
Expected: FAIL — no `phase` on the identity line; `for`/`watch` payloads allowed; no shadowing block.

- [ ] **Step 3: The hook**

In `internal/cli/hook.go`, `sessionStart` becomes:
```go
// sessionStart runs the shadowing check (R-HOOK-005), writes the ready mark (R-HOOK-003, first
// mark wins, R-HOOK-009; the stored phase is the launch's, never touched here), prints the
// identity with the shown phase (R-HOOK-004) and, after a compaction, the cheat sheet and the
// post-compact files (R-HOOK-006, R-HOOK-007). An adopted root is a session like any other
// (R-HOOK-008).
func sessionStart(d Deps, b bound) error {
	p, err := hook.Parse(d.Stdin)
	if err != nil {
		p = hook.Payload{} // an unreadable payload is a plain startup: identity still loads
	}
	primary, perr := primaryRoot(b.p)
	if perr == nil {
		if serr := session.CheckShadowing(filepath.Join(primary, ".agents", "skills"), skillRoots()); serr != nil {
			return block(serr.Error())
		}
	}
	now := d.Now()
	markErr := b.s.MarkReady(b.c.id, now)
	sess, serr := b.s.Session(b.c.id)
	if serr != nil {
		sess = b.c.sess
	}
	timeout := time.Minute
	if cfg, cerr := b.p.file.Config(nil); cerr == nil {
		timeout = cfg.Launch.ReadyTimeout.Duration
	}
	say(d.Stdout, "ness session %s scope %s manager %s phase %s", sess.ID, sess.Scope, displayName(sess.Parent),
		session.Shown(sess, now, timeout))
	if p.Source == "compact" {
		say(d.Stdout, "%s", cheatSheet)
		for _, part := range postCompact(d, b.p) {
			say(d.Stdout, "%s", strings.TrimSuffix(string(part.Text), "\n"))
		}
	}
	return markErr
}
```
In `internal/hook/guards.go`, extend the loop regex and add `watch`:
```go
	// shellLoop matches a while/until/for loop keyword; watch re-runs its command on a timer.
	shellLoop = regexp.MustCompile(`(^|[\s;&|('"])(while|until|for|watch)\b`)
```
and add a table case in `guards_test.go` for `for … ness inbox` (blocked), `watch -n 5 ness status` (blocked) and `for f in *.md; do wc -l $f; done` (allowed), with `// covers: R-HOOK-011`.

- [ ] **Step 4: Run and commit**

Run: `go test ./internal/hook/ ./internal/cli/ && make lint`
Expected: PASS, including `open_here.txtar` now that the identity line carries the phase.

```bash
git add internal/cli/hook.go internal/hook internal/cli/testdata/script
git commit -m "feat(hook): phase on the identity line, shadowing check at session start, for/watch polling guard"
```

---

### Task 9: `ness status`

**Files:**
- Create: `internal/cli/status.go`, `internal/cli/testdata/script/status.txtar`
- Modify: `internal/cli/cli.go` (register)

**Interfaces:**
- Consumes: `store.Sessions`, `store.Obligations`, `store.Heartbeat`, `session.Shown`, `session.Client.Transcript`, `session.Runtime.Agents`.
- Produces: `newStatusCmd(d Deps) *cobra.Command`. Text: one line per session, depth-indented by two spaces, tab-separated fields `<id> <scope> <tier> <phase> ctx <n>k|- idle <n>m|- waits <n>[ (from…)]`, closed sessions marked `closed` with `--all`; then `probes: none (feature off)`; then `worker: <version> heartbeat <age>s` or `worker: not running`. JSON: `{"schema":1,"sessions":[…],"probes":[],"worker":{…}}`.

- [ ] **Step 1: Write the failing script**

`status.txtar`:
```
# status prints the active tree with tier, phase, context, idle minutes and obligations with whom
# they wait on; --all adds closed sessions marked closed; a stalled launch shows as such; the
# worker heartbeat age and version, or not running; --json carries every field.
# covers: R-SESS-080, R-SESS-082, R-SESS-084, R-SESS-085, R-SESS-086, R-SESS-033, R-CFG-053
cd repo
exec ness init --gate true --base main --yes
exec git add -A
exec git commit -q -m 'ness init'
env NESS_TEST_NOW=2026-10-08T10:00:00Z
exec ness status
stdout '^worker: not running$'
exec ness open --root --scope prog --tier standard --brief ../b.md
store last-session R
exec sh -c 'cd .worktrees/prog-* && NESS_SESSION=$R ness open --scope a --tier economy --brief ../../../b.md'
store last-session A
exec sh -c 'cd .worktrees/prog-* && NESS_SESSION=$R ness open --scope b --tier standard --brief ../../../b.md'
store last-session B
# A transcript for the root: context and idle come from it.
exec sh -c 'ness status --json | grep -o "\"client_session\": *\"[0-9a-f-]*\"" | head -1 | grep -o "[0-9a-f-]\{36\}" > ../cs'
exec sh -c 'd=$(cd .worktrees/prog-* && pwd | sed "s#[/.]#-#g"); mkdir -p $HOME/.claude/projects/$d; printf "%s\n" "{\"type\":\"assistant\",\"message\":{\"usage\":{\"input_tokens\":1000,\"cache_read_input_tokens\":119000,\"cache_creation_input_tokens\":0}}}" > $HOME/.claude/projects/$d/$(cat ../cs).jsonl; touch -d 2026-10-08T09:55:00Z $HOME/.claude/projects/$d/$(cat ../cs).jsonl'
# Obligations: a waits on the root's answer; the operator waits on the root.
env NESS_SESSION=$A
exec ness send --to manager --body 'need a decision' --intent decision
env NESS_SESSION=
exec ness send --to $R --body 'from the operator'
store heartbeat 12 0.1.0
exec ness status
stdout '^s-[0-9a-f]{12}\tprog\tstandard\tready\tctx 120k\tidle 5m\twaits 2 \(s-[0-9a-f]{12}, operator\)$'
stdout '^  s-[0-9a-f]{12}\ta\teconomy\tready\tctx -\tidle -\twaits 0$'
stdout '^  s-[0-9a-f]{12}\tb\tstandard\tready\tctx -\tidle -\twaits 0$'
stdout '^probes: none \(feature off\)$'
stdout '^worker: 0\.1\.0 heartbeat 12s$'
# A stalled launch.
store phase $B waiting-ready
store set-ready $B false
env NESS_TEST_NOW=2026-10-08T10:02:00Z
exec ness status
stdout '\tb\tstandard\tstalled launch\t'
# Closed sessions leave the default view and come back with --all.
store close-session $B
exec ness status
! stdout '\tb\t'
exec ness status --all
stdout '\tb\tstandard\tclosed\t'
# JSON carries the same fields.
exec ness status --json
stdout '"schema": 1'
stdout '"scope": "prog"'
stdout '"tier": "standard"'
stdout '"phase": "ready"'
stdout '"context_tokens": 120000'
stdout '"idle_minutes": 7'
stdout '"from": "operator"'
stdout '"heartbeat_age_seconds": 132'
stdout '"version": "0.1.0"'
# Outside a project status refuses.
cd $WORK
exits 4 ness status

-- b.md --
b
```
`store set-ready ID false` is new: add it to `storeSubs` (runs `UPDATE sessions SET ready_at = NULL` through a new `(*Store).ClearReady(id string) error` in `launch.go`; `NewRun` can use it too).

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run 'TestScripts/status'`
Expected: FAIL — `unknown command "status"`.

- [ ] **Step 3: The command**

`internal/cli/status.go`:
```go
package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/session"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func newStatusCmd(d Deps) *cobra.Command {
	var all, asJSON bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "The session tree: tier, phase, context, idle time, obligations; probes; the worker",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return withStore(d, func(s *store.Store, _ caller) error { return runStatus(d, s, all, asJSON) })
		},
	}
	cmd.Flags().BoolVar(&all, "all", false, "also list closed sessions")
	cmd.Flags().BoolVar(&asJSON, "json", false, "print one JSON object")
	return cmd
}

type sessionJSON struct {
	ID            string           `json:"id"`
	Parent        string           `json:"parent"`
	Scope         string           `json:"scope"`
	Title         string           `json:"title"`
	Tier          string           `json:"tier"`
	Effort        string           `json:"effort"`
	Client        string           `json:"client"`
	Phase         string           `json:"phase"`
	Closed        bool             `json:"closed"`
	ContextTokens *int             `json:"context_tokens"`
	IdleMinutes   *int             `json:"idle_minutes"`
	Obligations   []obligationJSON `json:"obligations"`
	Integrated    string           `json:"integrated"`
	Worktree      string           `json:"worktree"`
	Branch        string           `json:"branch"`
	Pane          string           `json:"pane"`
	ClientSession string           `json:"client_session"`
	Depth         int              `json:"depth"`
}

type obligationJSON struct {
	ID   string `json:"id"`
	From string `json:"from"`
}

type workerJSON struct {
	Running             bool   `json:"running"`
	Version             string `json:"version"`
	HeartbeatAgeSeconds *int   `json:"heartbeat_age_seconds"`
}

type statusJSON struct {
	Schema   int           `json:"schema"`
	Sessions []sessionJSON `json:"sessions"`
	Probes   []any         `json:"probes"`
	Worker   workerJSON    `json:"worker"`
}

func runStatus(d Deps, s *store.Store, all, asJSON bool) error {
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
	now := d.Now()
	rows, err := s.Sessions(all)
	if err != nil {
		return err
	}
	home, _ := os.UserHomeDir()
	out := statusJSON{Schema: 1, Sessions: []sessionJSON{}, Probes: []any{}}
	for _, sess := range ordered(rows) {
		j := sessionJSON{ID: sess.ID, Parent: displayName(sess.Parent), Scope: sess.Scope, Title: sess.Title, Tier: sess.Tier,
			Effort: sess.Effort, Client: sess.Client, Phase: session.Shown(sess, now, cfg.Launch.ReadyTimeout.Duration),
			Closed: sess.Closed, Obligations: []obligationJSON{}, Integrated: sess.Integrated, Worktree: sess.Worktree,
			Branch: sess.Branch, Pane: sess.Pane, ClientSession: sess.ClientSession, Depth: depth(rows, sess)}
		if sess.Closed {
			j.Phase = "closed"
		}
		if !sess.Closed && sess.ClientSession != "" {
			dir := filepath.Join(primary, sess.Worktree)
			if sess.Worktree == "" {
				dir = primary
			}
			if tr, terr := buildClient(d, sess.Client).Transcript(home, dir, sess.ClientSession); terr == nil {
				ctx, idle := tr.Context, int(now.Sub(tr.LastActive).Minutes())
				j.ContextTokens, j.IdleMinutes = &ctx, &idle
			}
		}
		obl, oerr := s.Obligations(sess.ID)
		if oerr != nil {
			return oerr
		}
		for _, it := range obl {
			j.Obligations = append(j.Obligations, obligationJSON{ID: it.ID, From: displayName(it.Sender)})
		}
		out.Sessions = append(out.Sessions, j)
	}
	hb, herr := s.Heartbeat()
	if herr == nil {
		age := int(now.Sub(hb.At).Seconds())
		out.Worker = workerJSON{Running: age <= 60, Version: hb.Version, HeartbeatAgeSeconds: &age}
	}
	if asJSON {
		enc := json.NewEncoder(d.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	printStatus(d, out)
	return nil
}

// ordered returns the rows depth-first: each session followed by its children, in creation order.
func ordered(rows []store.Session) []store.Session {
	var out []store.Session
	var walk func(parent string)
	walk = func(parent string) {
		for _, r := range rows {
			if r.Parent == parent {
				out = append(out, r)
				walk(r.ID)
			}
		}
	}
	walk(store.Operator)
	return out
}

func depth(rows []store.Session, s store.Session) int {
	n := 0
	for s.Parent != store.Operator {
		i := slices.IndexFunc(rows, func(r store.Session) bool { return r.ID == s.Parent })
		if i < 0 {
			break
		}
		s, n = rows[i], n+1
	}
	return n
}

func printStatus(d Deps, out statusJSON) {
	for _, j := range out.Sessions {
		ctx, idle := "-", "-"
		if j.ContextTokens != nil {
			ctx = fmt.Sprintf("%dk", *j.ContextTokens/1000)
		}
		if j.IdleMinutes != nil {
			idle = fmt.Sprintf("%dm", *j.IdleMinutes)
		}
		waits := fmt.Sprintf("waits %d", len(j.Obligations))
		if len(j.Obligations) > 0 {
			var from []string
			for _, o := range j.Obligations {
				if !slices.Contains(from, o.From) {
					from = append(from, o.From)
				}
			}
			waits += " (" + strings.Join(from, ", ") + ")"
		}
		say(d.Stdout, "%s%s\t%s\t%s\t%s\tctx %s\tidle %s\t%s", strings.Repeat("  ", j.Depth), j.ID, j.Scope, j.Tier, j.Phase, ctx, idle, waits)
	}
	say(d.Stdout, "probes: none (feature off)")
	if out.Worker.HeartbeatAgeSeconds == nil {
		say(d.Stdout, "worker: not running")
		return
	}
	say(d.Stdout, "worker: %s heartbeat %ds", out.Worker.Version, *out.Worker.HeartbeatAgeSeconds)
}

var _ = time.Minute
```
(Delete the trailing `var _` if `time` ends up unused.) P5 replaces the `probes:` line with the probe list (R-SESS-081, R-SESS-083). Register `newStatusCmd(d)`.

- [ ] **Step 4: Run and commit**

Run: `go test ./internal/cli/ && make lint`
Expected: PASS.

```bash
git add internal/cli/status.go internal/cli/cli.go internal/cli/testdata/script/status.txtar internal/store/launch.go internal/testkit/store.go
git commit -m "feat(cli): ness status prints the tree, stalled launches, obligations and the worker heartbeat"
```

---

### Task 10: Coverage gate, status line, wrap-up

**Files:**
- Modify: `internal/testkit/coverage_repo_test.go`, `README.md`

- [ ] **Step 1: MOD joins the gate; the deferred map moves on**

Replace the two variables in `internal/testkit/coverage_repo_test.go`:
```go
// doneAreas are the spec areas whose plans have landed (P1, P2, P3a: MOD).
var doneAreas = []string{"CFG", "SYNC", "PROF", "QA", "MSG", "HOOK", "MOD"}

// deferred names ids of done areas that a later plan, a review entry or publication delivers.
var deferred = map[string]string{
	"R-SYNC-068": "P3b", "R-SYNC-086": "P3b", "R-MOD-050": "P3b",
	"R-QA-024": "P3b", "R-QA-027": "P3b", "R-QA-042": "P3b",
	"R-MSG-005": "P4", "R-MSG-042": "P4", "R-MOD-012": "P4 (worker half; the session half is covered)",
	"R-CFG-030": "P5", "R-CFG-031": "P5", "R-CFG-054": "P5", "R-PROF-015": "P5", "R-HOOK-010": "P5",
	"R-CFG-032": "P6", "R-CFG-034": "P6", "R-CFG-035": "P6", "R-CFG-037": "P6", "R-CFG-051": "P6",
	"R-SYNC-032": "P6", "R-SYNC-033": "P6", "R-SYNC-089": "P6", "R-PROF-002": "P6",
	"R-QA-004": "P6", "R-QA-005": "P6", "R-QA-023": "P6", "R-QA-030": "P6", "R-QA-031": "P6", "R-QA-041": "P6",
	"R-MOD-002": "P6", "R-MOD-021": "P6",
	"R-QA-032": "publication", "R-QA-033": "publication",
	"R-QA-025": "withdrawn (D39)",
}
```
R-CFG-041, R-HOOK-005, R-HOOK-008 and R-HOOK-018 leave the map (Tasks 4, 7, 8). R-MOD-012's session half is `TestLaunchEndToEndOverFakes` (zero typed texts, marker in place); the map entry keeps the worker half visible. R-QA-024 stays deferred until P3b adds the integrator and channel fakes, although the runtime and client fakes already carry the marker: if the gate reports it as "deferred but covered", remove it from the map instead.

Run: `go test ./internal/testkit/ -run TestRequirementCoverage`
Expected: PASS. If it names an uncovered MOD id, it is a real gap: find the task whose code delivers it, add the test with its covers marker, and rerun.

- [ ] **Step 2: README status**

Replace the status line with:
`Status: P3a (sessions and launch) implemented: ness open, status, the herdr/Claude Code/Codex atoms and the environment checks on top of P2. Plans: docs/superpowers/specs/2026-10-08-ness-v1-scope.md.`

- [ ] **Step 3: The full gate and the dogfood check**

Run: `make validate && make fuzz FUZZTIME=10s`
Expected: PASS end to end. Then in this repository: `go run ./cmd/ness status` — expected: an empty tree, `probes: none (feature off)`, `worker: not running`, exit 0; `git status` clean.

- [ ] **Step 4: Commit and push, check CI**

```bash
git add -A
git commit -m "build: MOD joins the requirement coverage gate; P3a status"
git push
gh run watch --exit-status
```
Expected: green. The real-herdr contract (`-tags e2e`) is not in CI; run it once locally inside herdr before the review and note the result in the report.

---

## End-of-plan review

After Task 10, launch one fresh reviewer on the most capable model with: `docs/spec/sessions.md`, `docs/spec/models.md`, this plan, and `git diff <first P3a commit>^..HEAD`. The reviewer checks:
1. Each covered requirement's test would fail without its behaviour (spot-check: drop `SetPhase` before a step, drop the `live` check in `runResume`, drop `--no-focus` from `OpenTab`, make `AgentName` ignore `live`).
2. The Review Focus items hold.
3. The rulings above, in particular the client version rule, the Codex `-c` keys and the derived phase.
4. R-SESS-014: D40's evidence, and that `claude.EnterpriseSkills` equals it; the reviewer appends `- R-SESS-014 | <reviewer session> | <date> | <commit>` to `docs/spec/reviews.md`.

Then fix each finding, rerun `make validate`, and report to the operator: what shipped, the evidence (test counts, the real-herdr contract result, CI run link), and what P3b takes over.
