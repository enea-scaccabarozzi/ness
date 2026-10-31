> [!NOTE]
> Archive: altered for the public release. Local paths were replaced; the content is otherwise as written.

# P2 Store, Messages and Hooks Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The SQLite store under the git common dir, the three message commands (`ness send`, `ness reply`, `ness inbox`) with the routing, idempotency and obligation semantics of the design, and the two client hooks (`ness hook session-start`, `ness hook pre-tool`) with the ready mark, the cheat sheet, the polling guard and the output guard. The session tree itself (open, close, herdr, launch phases) is P3: this plan ships the minimal `sessions` table and an identity seam (`NESS_SESSION` → `Deps.Session`) that P3 fills.

**Architecture:** `internal/store` owns the database: `Locate` finds `<git-common-dir>/ness/state.sqlite3`, `Open` applies forward-only migrations in one immediate transaction, and every write is one short transaction so twenty processes can send at once. `internal/hook` parses client hook payloads and decides the pre-tool guards as pure functions. `internal/cli` stays thin: `session.go` resolves the caller (a session row or the operator), `send.go`/`reply.go`/`inbox.go` apply the routing rules and print one line, `hook.go` binds a session (or stays silent) and wires the guards. `internal/testkit` gains a `store` script command that inserts sessions through the store API, standing in for P3's `open`.

**Tech Stack:** Go 1.25+ (local 1.27.1), `modernc.org/sqlite` v1.60.1 (pure Go, driver name `sqlite`), cobra, testscript. The P1 gate (golangci-lint go-strict, `go test -race`, `-count=4`, govulncheck, requirement coverage, `ness sync --check`) stays as is, plus a `CGO_ENABLED=0` build.

**Spec:** `docs/superpowers/specs/2026-10-08-ness-v1-scope.md` (split), `docs/design/ness-design.md` §5 (Messages, Machinery), §6 (runtime files), §13 (polling guard, cheat sheet), §14 (worker rules), and the requirements in `docs/spec/store-messages.md` (MSG), `docs/spec/hooks.md` (HOOK), with the CLI conventions of `docs/spec/core-config.md`.

## Global Constraints

- Module `github.com/enea-scaccabarozzi/ness`, one binary, `CGO_ENABLED=0` (R-QA-001, R-MSG-002). The driver is `modernc.org/sqlite`; no cgo SQLite package may enter the dependency graph.
- The store is `<git-common-dir>/ness/state.sqlite3`, found with `git rev-parse --path-format=absolute --git-common-dir`, shared by the primary checkout and every worktree (R-MSG-001). ness opens no other store (R-MSG-004).
- Exit codes: 0 ok, 1 a check found problems, 2 usage, 3 sync conflict, 4 refused (R-CFG-035). A refusal writes nothing: no file, no store row (R-CFG-036). The one exception: a `pre-tool` block exits 2 with the reason on stderr (R-HOOK-014).
- Identity: `NESS_SESSION=<id>` makes the process that session; unset means the operator (R-SESS-037). Only the composition roots (`cmd/ness/main.go`, the test `ness` command) read the variable, into `cli.Deps.Session` (R-CFG-002).
- Outside a ness project every command refuses except `init`, `--version`, `--help` and `hook`, which exits 0 silently (R-CFG-053). `hook` is also silent without `NESS_SESSION` and for a session the store does not know (R-HOOK-002).
- Item ids are `m-` plus 8 lowercase base32 characters `[a-z2-7]` (R-MSG-044). Session ids in fixtures follow P3's shape `s-` plus 12 hex characters (R-SESS-042).
- `ness send` prints exactly `sent <id> to <recipient>` or `already sent <id>`, never the body (R-MSG-016). `--request-key` does not exist (R-MSG-019). `--intent reply` is a usage error (R-MSG-049).
- Ages print as `<n>m`, `<n>h`, `<n>d`, largest unit, floored (R-MSG-045). Excerpts are at most 80 runes with newlines and tabs replaced (R-MSG-031).
- Timestamps are stored as RFC 3339 text in UTC. Commands take the clock once from `Deps.Now()` and pass the value down; the store never calls `time.Now`.
- Output goes to the injected writers in `cli.Deps`. Only `cmd/ness` touches `os.Stdout` (R-CFG-033).
- Every test names the requirement ids it covers: `// covers: R-…` in Go tests and `# covers: R-…` in scripts.
- Conventional commits. Each commit ends with the session's Co-Authored-By/Claude-Session lines.

## Review Focus

1. **`--body ""`, `--body @` or `--body @missing-file`.** There is nothing to deliver, so this must be a usage error (exit 2) and no item may be created; the body must never be echoed. Pinned in Task 4 (`send.txtar`).
2. **A body with multi-byte or invalid UTF-8 bytes in `ness inbox`.** The excerpt must count runes, never split one, and stay on one line. Pinned in Task 3 (`FuzzExcerpt`).
3. **Two processes creating the store for the first time at once (fresh clone, twenty hooks or sends).** The migration must not fail with "table already exists" and the version must be written once. Pinned in Task 2 (`TestConcurrentFirstOpen`) and Task 4 (`send_concurrent.txtar`).
4. **`NESS_SESSION` naming a session the store does not know (another project's id, a typo, a closed session).** `send`, `inbox` and `reply` must refuse with exit 4 and say how to fix it; `hook` must stay silent so the client keeps working. Pinned in Task 1 (`hook_unbound.txtar`) and Task 4 (`send_routes.txtar`).
5. **`ness inbox --show` or `ness reply` on an item addressed to someone else, or on an id that does not exist.** Must refuse with exit 4 naming the recipient or the id, and must not mark the item shown or disposed. Pinned in Task 5 (`inbox.txtar`) and Task 6 (`reply.txtar`).

## Deferred to later plans

These MSG and HOOK ids need a command, a runtime or a feature from a later plan. The coverage test (Task 9) lists them so the gate stays green while the gap stays visible:

- P3 (sessions): R-HOOK-005 (shadowing check needs R-SESS-005/006 and the real client contract), R-HOOK-008 (adopted root, `open --root --here`), R-HOOK-018 (Codex launch wiring). P3 also extends scripts this plan writes: `hook_session_start.txtar` gains the phase in the identity line (R-HOOK-004) and the "phase unchanged" assertion (R-HOOK-009); the close scripts re-cover R-MSG-040, R-MSG-041, R-MSG-043 and R-MSG-022 (close blocked or not), which this plan covers at the store level.
- P4 (worker): R-MSG-005 (invariants under concurrent close and worker passes), R-MSG-042 (urgent hint delivery; the flag is accepted and stored here).
- P5 (features): R-HOOK-010 (contract guard in `pre-tool`).
- Review entry in `docs/spec/reviews.md`, written by the end-of-plan reviewer: R-MSG-004 (no other store is opened).
- Real-client halves of R-HOOK-014 (a blocked Edit leaves the file unchanged in a real session) run with P3's e2e tag; the exit code and reason are covered here.

Ids this plan removes from P1's deferred list: R-CFG-014 (Task 5), R-CFG-053 (Tasks 1, 4, 5).

## File map

```
go.mod, go.sum                                modernc.org/sqlite v1.60.1
cmd/ness/main.go                              Deps.Session from NESS_SESSION, Deps.Now = time.Now
internal/cli/cli.go                           Deps.Session, Deps.Now, usage(), register hook/send/reply/inbox
internal/cli/session.go                       caller: identity seam, withStore, openStore, displayName
internal/cli/send.go                          ness send: flags, body, routing
internal/cli/inbox.go                         ness inbox: list, --show, --boundary, --json
internal/cli/inbox_test.go                    golden inbox line (white-box)
internal/cli/reply.go                         ness reply
internal/cli/hook.go                          ness hook: bind or stay silent, pre-tool, session-start, cheat sheet
internal/cli/script_test.go                   NESS_SESSION and NESS_TEST_NOW seams
internal/cli/testdata/script/hook_unbound.txtar, send.txtar, send_routes.txtar, send_concurrent.txtar,
  inbox.txtar, inbox_boundary.txtar, store_worktree.txtar, pin_patch.txtar, reply.txtar,
  hook_pretool.txtar, hook_session_start.txtar
internal/store/store.go                       Locate, Open (WAL, busy_timeout, immediate), Close, inTx
internal/store/migrate.go                     migrations, Version, migrate
internal/store/sessions.go                    Session, AddSession, CloseSession, Session, MarkReady, SetBoundary
internal/store/items.go                       Item, Send, Listed, Inbox, Item, MarkShown, Reply, Obligations
internal/store/ids.go                         NewItemID
internal/store/format.go                      Excerpt, FormatAge
internal/store/*_test.go                      unit tests and fuzz targets
internal/hook/payload.go                      Payload, Parse
internal/hook/guards.go                       PreTool: polling guard, output guard
internal/hook/*_test.go                       guard tables, FuzzParse
internal/testkit/store.go                     `store` script command (fixtures and checks through the store API)
internal/testkit/testkit.go                   register `store` in Cmds
internal/testkit/repo_test.go                 pure-Go SQLite dependency check
internal/testkit/coverage_repo_test.go        MSG and HOOK join doneAreas
Makefile                                      build target with CGO_ENABLED=0
README.md                                     status line
```

---

### Task 1: `ness hook` as a silent no-op, and the identity seam

**Files:**
- Create: `internal/cli/hook.go`, `internal/cli/testdata/script/hook_unbound.txtar`
- Modify: `internal/cli/cli.go` (Deps fields, `usage`, register the command), `cmd/ness/main.go`, `internal/cli/script_test.go`, `internal/cli/testdata/script/sync_json.txtar` (covers line only)

**Interfaces:**
- Consumes: `cli.Deps`, `cli.Main`, `exitError`, `refuse`, `exitCode` from P1.
- Produces: `Deps.Session string` (the caller's session id, `""` = operator), `Deps.Now func() time.Time`, `usage(format string, a ...any) error` (exit 2), `newHookCmd(d Deps) *cobra.Command`, `hookEvents`. Scripts may set `NESS_SESSION` and `NESS_TEST_NOW` (RFC 3339).

- [ ] **Step 1: Write the failing script**

`internal/cli/testdata/script/hook_unbound.txtar`:
```
# ness hook is a silent exit 0 outside a ness project, without NESS_SESSION, and for a session the
# store does not know — whatever the payload. An unknown event is a usage error.
# covers: R-HOOK-002, R-HOOK-016, R-CFG-053
# No repository at all.
exec ness hook session-start
! stdout .
! stderr .
stdin garbage.txt
exec ness hook pre-tool
! stdout .
! stderr .
stdin blocked.json
exec ness hook pre-tool
! stdout .
! stderr .
exits 2 ness hook nope
stderr 'unknown hook event'
# A repository without .ness/ness.toml: other commands refuse, hook does not.
cd repo
exits 4 ness sync
stdin blocked.json
exec ness hook pre-tool
! stdout .
! stderr .
# A ness project, but NESS_SESSION names a session the store does not know.
exec ness init --gate true --base main --yes
env NESS_SESSION=s-000000000000
stdin blocked.json
exec ness hook pre-tool
! stdout .
! stderr .
stdin garbage.txt
exec ness hook session-start
! stdout .
! stderr .
# Bound to nothing (no NESS_SESSION) inside the project.
env NESS_SESSION=
stdin blocked.json
exec ness hook pre-tool
! stdout .
! stderr .
exec ness hook session-start
! stdout .

-- garbage.txt --
{not json at all
-- blocked.json --
{"tool_name":"Bash","tool_input":{"command":"while true; do ness inbox; sleep 5; done"}}
```

Append `, R-HOOK-001` to the covers line of `internal/cli/testdata/script/sync_json.txtar` (it already runs each wrapped hook command with `ness` absent from `PATH` and asserts silence and exit 0):
```
# covers: R-SYNC-050, R-SYNC-051, R-SYNC-053, R-SYNC-054, R-SYNC-067, R-HOOK-001
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run 'TestScripts/hook_unbound'`
Expected: FAIL — `ness hook session-start` exits 2 with `unknown command "hook"`.

- [ ] **Step 3: The identity seam in Deps and the composition roots**

In `internal/cli/cli.go`, replace the `Deps` type and add `usage`:
```go
// Deps carries what a command needs from the process.
type Deps struct {
	Stdin   io.Reader
	Stdout  io.Writer
	Stderr  io.Writer
	Version string
	// Session is the calling session's id, from NESS_SESSION; empty means the operator
	// (R-SESS-037). Only the composition roots read the variable (R-CFG-002).
	Session string
	// Now is the clock. A command reads it once per run and passes the value down.
	Now func() time.Time
}

// usage is the error of a bad invocation: exit 2.
func usage(format string, a ...any) error {
	return &exitError{code: ExitUsage, err: fmt.Errorf(format, a...)}
}
```
Add `"time"` to the imports and register the command: `root.AddCommand(newInitCmd(d), newSyncCmd(d), newHookCmd(d))`.

`cmd/ness/main.go`:
```go
// Command ness installs and runs an AI coding-agent harness inside one project.
package main

import (
	"os"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/cli"
)

// version is set at release time with -ldflags "-X main.version=<version>".
var version = "0.1.0"

func main() {
	os.Exit(cli.Main(os.Args[1:], cli.Deps{
		Stdin:   os.Stdin,
		Stdout:  os.Stdout,
		Stderr:  os.Stderr,
		Version: version,
		Session: os.Getenv("NESS_SESSION"), // the design's identity variable (R-SESS-037)
		Now:     time.Now,
	}))
}
```

`internal/cli/script_test.go`, the `ness` command:
```go
func TestMain(m *testing.M) {
	testscript.Main(m, map[string]func(){
		"ness": func() {
			// Test-only seams: scripts pick the binary version to exercise the pin (R-CFG-011)
			// and freeze the clock so ages and marks are deterministic.
			v := os.Getenv("NESS_TEST_VERSION")
			if v == "" {
				v = "0.1.0"
			}
			now := time.Now
			if s := os.Getenv("NESS_TEST_NOW"); s != "" {
				t, err := time.Parse(time.RFC3339, s)
				if err != nil {
					panic(err)
				}
				now = func() time.Time { return t }
			}
			os.Exit(cli.Main(os.Args[1:], cli.Deps{
				Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr,
				Version: v, Session: os.Getenv("NESS_SESSION"), Now: now,
			}))
		},
	})
}
```
Add `"time"` to its imports.

- [ ] **Step 4: The hook command**

`internal/cli/hook.go`:
```go
package cli

import (
	"slices"

	"github.com/spf13/cobra"
)

// hookEvents are the client events ness handles.
var hookEvents = []string{"session-start", "pre-tool"}

// newHookCmd is the entry point of the client hooks (R-HOOK-002, R-HOOK-016, R-CFG-053). Outside
// a ness project, without NESS_SESSION, or for a session the store does not know, every event is
// a silent exit 0: a contributor's client must never see ness fail.
func newHookCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "hook EVENT",
		Short: "Entry point for client hooks: session-start, pre-tool",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if !slices.Contains(hookEvents, args[0]) {
				return usage("unknown hook event %q; events are session-start and pre-tool", args[0])
			}
			if d.Session == "" {
				return nil
			}
			return nil // Task 8 binds the session and runs the guards
		},
	}
}
```

- [ ] **Step 5: Run the scripts and the gate**

Run: `go test ./internal/cli/ && make lint`
Expected: PASS; lint clean (if `revive` flags the two identical returns, merge them into one `return nil` with the comment).

- [ ] **Step 6: Commit**

```bash
git add cmd/ness/main.go internal/cli/cli.go internal/cli/hook.go internal/cli/script_test.go internal/cli/testdata/script/hook_unbound.txtar internal/cli/testdata/script/sync_json.txtar
git commit -m "feat(cli): ness hook exists as a silent no-op outside a bound session"
```

---

### Task 2: The store: location, open, migrations, sessions

**Files:**
- Create: `internal/store/store.go`, `internal/store/migrate.go`, `internal/store/sessions.go`, `internal/store/store_test.go`, `internal/store/sessions_test.go`
- Modify: `go.mod`/`go.sum` (dependency), `Makefile` (`build` target), `internal/testkit/repo_test.go` (pure-Go check)

**Interfaces:**
- Produces:
  - `store.Locate(dir string) (string, error)` — `<git-common-dir>/ness/state.sqlite3`.
  - `store.Open(path string) (*Store, error)`, `(*Store).Close() error`, `store.ErrNewer`, `store.Version() int`.
  - `store.Operator = ""`, `store.ErrNoSession`.
  - `type Session struct { ID, Parent, Scope string; Closed bool; CreatedAt, ReadyAt, BoundaryAt time.Time; BoundaryNote string }`.
  - `(*Store).AddSession(id, scope, parent string, now time.Time) error`, `CloseSession(id string) error`, `Session(id string) (Session, error)`, `MarkReady(id string, now time.Time) error` (first mark wins), `SetBoundary(id, note string, now time.Time) error`.
  - unexported `inTx(fn func(*sql.Tx) error) error`, `stamp`, `nullStamp`, `parseStamp`, `nullable`.

- [ ] **Step 1: Add the dependency and the build target**

```bash
cd ~/src/ness
go get modernc.org/sqlite@v1.60.1
```

`Makefile`: add `build` to `validate` and define it:
```make
validate: build lint test vuln selfcheck

build:
	CGO_ENABLED=0 go build ./...
```
(Keep the other targets unchanged; add `build` to the `.PHONY` line.)

- [ ] **Step 2: Write the failing tests**

`internal/store/store_test.go`:
```go
package store_test

import (
	"database/sql"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

var now = time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC)

func mustOpen(t *testing.T, path string) *store.Store {
	t.Helper()
	s, err := store.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := s.Close(); err != nil {
			t.Error(err)
		}
	})
	return s
}

func freshPath(t *testing.T) string {
	t.Helper()
	return filepath.Join(t.TempDir(), "ness", "state.sqlite3")
}

// raw opens the file with database/sql alone (the driver is registered by importing store).
func raw(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", "file:"+path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func userVersion(t *testing.T, path string) int {
	t.Helper()
	var v int
	if err := raw(t, path).QueryRow("PRAGMA user_version").Scan(&v); err != nil {
		t.Fatal(err)
	}
	return v
}

func TestOpenCreatesDirectoryAndSchema(t *testing.T) {
	// covers: R-MSG-046, R-MSG-002
	path := freshPath(t)
	s := mustOpen(t, path)
	if got := userVersion(t, path); got != store.Version() {
		t.Fatalf("user_version = %d, want %d", got, store.Version())
	}
	if err := s.AddSession("s-aaaaaaaaaaaa", "root", store.Operator, now); err != nil {
		t.Fatalf("schema not usable: %v", err)
	}
	var mode string
	if err := raw(t, path).QueryRow("PRAGMA journal_mode").Scan(&mode); err != nil || mode != "wal" {
		t.Fatalf("journal_mode = %q, %v; want wal", mode, err)
	}
}

func TestOpenMigratesAnOlderStoreForward(t *testing.T) {
	// covers: R-MSG-046
	path := freshPath(t)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	// A version-0 store: the file exists with no schema at all.
	if _, err := raw(t, path).Exec("PRAGMA user_version = 0"); err != nil {
		t.Fatal(err)
	}
	s := mustOpen(t, path)
	if _, err := s.Session("s-aaaaaaaaaaaa"); !errors.Is(err, store.ErrNoSession) {
		t.Fatalf("after migration Session() = %v, want ErrNoSession (table exists, row absent)", err)
	}
	if got := userVersion(t, path); got != store.Version() {
		t.Fatalf("user_version = %d, want %d", got, store.Version())
	}
}

func TestOpenRefusesANewerStore(t *testing.T) {
	// covers: R-MSG-046
	path := freshPath(t)
	first, err := store.Open(path) // not mustOpen: closed here, once
	if err != nil {
		t.Fatal(err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := raw(t, path).Exec("PRAGMA user_version = 99"); err != nil {
		t.Fatal(err)
	}
	_, err = store.Open(path)
	if !errors.Is(err, store.ErrNewer) {
		t.Fatalf("Open = %v, want ErrNewer", err)
	}
}

func TestConcurrentFirstOpen(t *testing.T) {
	// Review Focus 3: many processes create the store at once (fresh clone, twenty hooks).
	// covers: R-MSG-003
	path := freshPath(t)
	var wg sync.WaitGroup
	errs := make(chan error, 10)
	for range 10 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := store.Open(path)
			if err != nil {
				errs <- err
				return
			}
			errs <- s.Close()
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Error(err)
		}
	}
	if got := userVersion(t, path); got != store.Version() {
		t.Fatalf("user_version = %d, want %d", got, store.Version())
	}
}

// gitRepo makes a repository with one commit and one linked worktree, hermetically.
func gitRepo(t *testing.T) (repo, worktree string) {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.CommandContext(t.Context(), "git", args...)
		cmd.Dir = dir
		cmd.Env = []string{"PATH=" + os.Getenv("PATH"), "HOME=" + dir, "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_NOSYSTEM=1"}
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("-c", "init.defaultBranch=main", "init", "-q", "repo")
	run("-C", "repo", "-c", "user.name=t", "-c", "user.email=t@example.invalid", "commit", "-q", "--allow-empty", "-m", "init")
	run("-C", "repo", "worktree", "add", "-q", filepath.Join(dir, "wt"))
	return filepath.Join(dir, "repo"), filepath.Join(dir, "wt")
}

func TestLocateIsSharedByWorktrees(t *testing.T) {
	// covers: R-MSG-001
	repo, wt := gitRepo(t)
	want := filepath.Join(repo, ".git", "ness", "state.sqlite3")
	for _, dir := range []string{repo, wt, filepath.Join(wt, ".")} {
		got, err := store.Locate(dir)
		if err != nil {
			t.Fatal(err)
		}
		if got != want {
			t.Fatalf("Locate(%s) = %s, want %s", dir, got, want)
		}
	}
	if _, err := store.Locate(t.TempDir()); err == nil {
		t.Fatal("Locate outside a repository succeeded")
	}
}
```

`internal/store/sessions_test.go`:
```go
package store_test

import (
	"errors"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestSessionsTreeAndMarks(t *testing.T) {
	// The minimal session rows P3 builds on: parent, state, the ready mark and the boundary.
	// covers: R-HOOK-003, R-HOOK-009, R-MSG-034
	s := mustOpen(t, freshPath(t))
	if err := s.AddSession("s-aaaaaaaaaaaa", "root", store.Operator, now); err != nil {
		t.Fatal(err)
	}
	if err := s.AddSession("s-bbbbbbbbbbbb", "child", "s-aaaaaaaaaaaa", now); err != nil {
		t.Fatal(err)
	}
	if err := s.AddSession("s-cccccccccccc", "orphan", "s-nope", now); err == nil {
		t.Fatal("a parent that does not exist was accepted")
	}
	child, err := s.Session("s-bbbbbbbbbbbb")
	if err != nil {
		t.Fatal(err)
	}
	if child.Parent != "s-aaaaaaaaaaaa" || child.Scope != "child" || child.Closed || !child.ReadyAt.IsZero() || !child.CreatedAt.Equal(now) {
		t.Fatalf("child = %+v", child)
	}
	root, err := s.Session("s-aaaaaaaaaaaa")
	if err != nil || root.Parent != store.Operator {
		t.Fatalf("root = %+v, %v", root, err)
	}
	if _, err := s.Session("s-nope"); !errors.Is(err, store.ErrNoSession) {
		t.Fatalf("unknown session: %v", err)
	}

	later := now.Add(5 * time.Minute)
	if err := s.MarkReady("s-bbbbbbbbbbbb", now); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkReady("s-bbbbbbbbbbbb", later); err != nil {
		t.Fatal(err)
	}
	if err := s.MarkReady("s-nope", now); !errors.Is(err, store.ErrNoSession) {
		t.Fatalf("MarkReady unknown: %v", err)
	}
	if err := s.SetBoundary("s-bbbbbbbbbbbb", "plan written", later); err != nil {
		t.Fatal(err)
	}
	if err := s.CloseSession("s-bbbbbbbbbbbb"); err != nil {
		t.Fatal(err)
	}
	if err := s.CloseSession("s-nope"); !errors.Is(err, store.ErrNoSession) {
		t.Fatalf("CloseSession unknown: %v", err)
	}
	child, err = s.Session("s-bbbbbbbbbbbb")
	if err != nil {
		t.Fatal(err)
	}
	if !child.ReadyAt.Equal(now) {
		t.Fatalf("ready mark moved to %v; the first mark must win", child.ReadyAt)
	}
	if child.BoundaryNote != "plan written" || !child.BoundaryAt.Equal(later) || !child.Closed {
		t.Fatalf("child = %+v", child)
	}
}
```

Add to `internal/testkit/repo_test.go`:
```go
func TestPureGoSQLite(t *testing.T) {
	// covers: R-MSG-002
	cmd := exec.CommandContext(t.Context(), "go", "list", "-deps", "./cmd/ness")
	cmd.Dir = "../.."
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	deps := string(out)
	if !strings.Contains(deps, "modernc.org/sqlite") {
		t.Fatal("modernc.org/sqlite is not a dependency of cmd/ness")
	}
	for _, cgo := range []string{"github.com/mattn/go-sqlite3", "crawshaw.io/sqlite", "zombiezen.com/go/sqlite"} {
		if strings.Contains(deps, cgo) {
			t.Fatalf("cgo SQLite package %s in the dependency graph", cgo)
		}
	}
}
```
(add `"os/exec"` to that file's imports.)

- [ ] **Step 3: Run to verify failure**

Run: `go test ./internal/store/ ./internal/testkit/`
Expected: FAIL to build (`no Go files in internal/store` / undefined `store.Open`); `TestPureGoSQLite` fails because `cmd/ness` does not import the driver yet.

- [ ] **Step 4: Implement the store**

`internal/store/store.go`:
```go
// Package store is ness's SQLite state: the sessions of a project and the messages between
// them, shared by the primary checkout and every worktree (R-MSG-001). It is the only store ness
// reads (R-MSG-004); the old harness stores are never opened.
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite" // pure-Go driver, registered as "sqlite" (R-MSG-002)
)

// Operator is the sender, recipient or parent value of the operator: no session (R-SESS-037).
const Operator = ""

// ErrNewer is returned by Open when the store was written by a newer ness (R-MSG-046).
var ErrNewer = errors.New("store schema is newer than this ness")

// Store is an open state database.
type Store struct {
	db *sql.DB
}

// Locate returns the store path of the checkout containing dir: <git-common-dir>/ness/state.sqlite3,
// the same file from the primary checkout and from every worktree.
func Locate(dir string) (string, error) {
	args := []string{"-C", dir, "rev-parse", "--path-format=absolute", "--git-common-dir"}
	out, err := exec.CommandContext(context.Background(), "git", args...).Output() //nolint:gosec // fixed git subcommand
	if err != nil {
		return "", fmt.Errorf("%s is not inside a git repository", dir)
	}
	return filepath.Join(strings.TrimSpace(string(out)), "ness", "state.sqlite3"), nil
}

// Open opens or creates the store at path and brings its schema to Version.
//
// Connection settings, one reason each: journal_mode=WAL, so readers (inbox, status, the
// worker) never block a writer and a crash cannot corrupt the file; busy_timeout=5000, so a
// second process waits up to 5 s for the write lock instead of failing with SQLITE_BUSY;
// _txlock=immediate, so every transaction takes the write lock at BEGIN and two processes can
// never deadlock upgrading read locks; foreign_keys=ON, so an item can only name sessions that
// exist. One connection per process keeps the pragmas and the transactions serial.
func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return nil, err
	}
	dsn := "file:" + path + "?_txlock=immediate" +
		"&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)&_pragma=foreign_keys(ON)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		return nil, errors.Join(err, db.Close())
	}
	return s, nil
}

// Close releases the database.
func (s *Store) Close() error { return s.db.Close() }

// inTx runs fn in one immediate transaction: committed when fn returns nil, rolled back otherwise.
func (s *Store) inTx(fn func(tx *sql.Tx) error) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	if err := fn(tx); err != nil {
		return errors.Join(err, tx.Rollback())
	}
	return tx.Commit()
}

const timeLayout = time.RFC3339Nano

// stamp formats a time for storage: RFC 3339 in UTC, so text order is time order.
func stamp(t time.Time) string { return t.UTC().Format(timeLayout) }

func parseStamp(s string) time.Time {
	t, err := time.Parse(timeLayout, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func nullStamp(ns sql.NullString) time.Time {
	if !ns.Valid {
		return time.Time{}
	}
	return parseStamp(ns.String)
}

// nullable stores the operator (empty id) as NULL so foreign keys hold.
func nullable(id string) any {
	if id == Operator {
		return nil
	}
	return id
}
```

`internal/store/migrate.go`:
```go
package store

import (
	"database/sql"
	"fmt"
)

// migrations[i] brings the schema from version i to i+1. Append only; a shipped entry is never
// edited (R-MSG-046). P3 appends the session columns it needs (tier, phase, worktree, tab).
var migrations = []string{
	`CREATE TABLE sessions (
		id            TEXT PRIMARY KEY,
		parent        TEXT REFERENCES sessions(id),
		scope         TEXT NOT NULL,
		state         TEXT NOT NULL DEFAULT 'open' CHECK (state IN ('open', 'closed')),
		created_at    TEXT NOT NULL,
		ready_at      TEXT,
		boundary_note TEXT,
		boundary_at   TEXT
	);
	CREATE TABLE items (
		id          TEXT PRIMARY KEY,
		key         TEXT NOT NULL UNIQUE,
		sender      TEXT REFERENCES sessions(id),
		recipient   TEXT REFERENCES sessions(id),
		intent      TEXT NOT NULL CHECK (intent IN ('request', 'fyi', 'decision', 'handoff', 'reply')),
		body        TEXT NOT NULL,
		urgent      INTEGER NOT NULL DEFAULT 0,
		created_at  TEXT NOT NULL,
		pulled_at   TEXT,
		shown_at    TEXT,
		disposition TEXT CHECK (disposition IN ('resolved', 'declined', 'deferred', 'accepted')),
		answer      TEXT,
		disposed_at TEXT
	);
	CREATE INDEX items_recipient ON items (recipient, disposed_at);`,
}

// Version is the schema version this binary writes.
func Version() int { return len(migrations) }

// migrate applies the missing migrations in one immediate transaction, so concurrent first
// openers serialize and the second one finds the work done, and refuses a newer store.
func (s *Store) migrate() error {
	return s.inTx(func(tx *sql.Tx) error {
		var v int
		if err := tx.QueryRow("PRAGMA user_version").Scan(&v); err != nil {
			return err
		}
		if v > len(migrations) {
			return fmt.Errorf("%w: store schema %d, binary schema %d; upgrade ness", ErrNewer, v, len(migrations))
		}
		for i := v; i < len(migrations); i++ {
			if _, err := tx.Exec(migrations[i]); err != nil {
				return fmt.Errorf("migration %d: %w", i+1, err)
			}
		}
		_, err := tx.Exec(fmt.Sprintf("PRAGMA user_version = %d", len(migrations)))
		return err
	})
}
```

`internal/store/sessions.go`:
```go
package store

import (
	"database/sql"
	"errors"
	"time"
)

// ErrNoSession is returned for a session id the store does not know.
var ErrNoSession = errors.New("no such session")

// Session is one row of the session tree. Parent is Operator for a root, whose manager is the
// operator (R-MSG-013).
type Session struct {
	ID, Parent, Scope      string
	Closed                 bool
	CreatedAt, ReadyAt     time.Time
	BoundaryAt             time.Time
	BoundaryNote           string
}

// AddSession records an open session; parent Operator makes a root.
func (s *Store) AddSession(id, scope, parent string, now time.Time) error {
	_, err := s.db.Exec(`INSERT INTO sessions (id, parent, scope, created_at) VALUES (?, ?, ?, ?)`,
		id, nullable(parent), scope, stamp(now))
	return err
}

// CloseSession marks the session closed; the row stays for history (D37).
func (s *Store) CloseSession(id string) error {
	return s.touch(`UPDATE sessions SET state = 'closed' WHERE id = ?`, id)
}

// MarkReady writes the ready mark the launch waits for; the first mark is kept (R-HOOK-003,
// R-HOOK-009).
func (s *Store) MarkReady(id string, now time.Time) error {
	return s.touch(`UPDATE sessions SET ready_at = COALESCE(ready_at, ?) WHERE id = ?`, stamp(now), id)
}

// SetBoundary records that the session wrote its state down and asks to be compacted when idle
// (R-MSG-034); the worker consumes it (R-WRK-030).
func (s *Store) SetBoundary(id, note string, now time.Time) error {
	return s.touch(`UPDATE sessions SET boundary_note = ?, boundary_at = ? WHERE id = ?`, note, stamp(now), id)
}

// Session returns one session or ErrNoSession.
func (s *Store) Session(id string) (Session, error) {
	var (
		out                                 Session
		parent, readyAt, bNote, bAt, created sql.NullString
		state                               string
	)
	err := s.db.QueryRow(`SELECT id, parent, scope, state, created_at, ready_at, boundary_note, boundary_at
		FROM sessions WHERE id = ?`, id).
		Scan(&out.ID, &parent, &out.Scope, &state, &created, &readyAt, &bNote, &bAt)
	if errors.Is(err, sql.ErrNoRows) {
		return Session{}, ErrNoSession
	}
	if err != nil {
		return Session{}, err
	}
	out.Parent, out.Closed, out.BoundaryNote = parent.String, state == "closed", bNote.String
	out.CreatedAt, out.ReadyAt, out.BoundaryAt = nullStamp(created), nullStamp(readyAt), nullStamp(bAt)
	return out, nil
}

// touch runs an update that must hit exactly one session row.
func (s *Store) touch(query string, args ...any) error {
	res, err := s.db.Exec(query, args...)
	if err != nil {
		return err
	}
	n, err := res.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrNoSession
	}
	return nil
}
```

Make `cmd/ness` import the driver transitively now so `TestPureGoSQLite` passes before Task 4 wires the commands: add a blank import to `internal/cli/hook.go`:
```go
import (
	"slices"

	"github.com/spf13/cobra"

	_ "github.com/enea-scaccabarozzi/ness/internal/store" // the driver joins the binary here; Task 4 uses the package
)
```
(Task 4 replaces this with a real import.)

- [ ] **Step 5: Run the tests, the race run and the gate**

Run: `go test -race ./internal/store/ ./internal/testkit/ && go test -count=4 ./internal/store/ && make build lint`
Expected: PASS. `TestConcurrentFirstOpen` passes four times. If `gofumpt` reorders the struct fields' alignment in `Session`, accept its format.

- [ ] **Step 6: Commit**

```bash
git add go.mod go.sum Makefile internal/store internal/cli/hook.go internal/testkit/repo_test.go
git commit -m "feat(store): SQLite store under the git common dir with forward-only migrations and sessions"
```

---

### Task 3: Items: ids, send, inbox, show, reply, obligations, formatting

**Files:**
- Create: `internal/store/ids.go`, `internal/store/items.go`, `internal/store/format.go`, `internal/store/ids_test.go`, `internal/store/items_test.go`, `internal/store/format_test.go`

**Interfaces:**
- Consumes: `Store`, `inTx`, `stamp`, `nullStamp`, `parseStamp`, `nullable`, `ErrNoSession` from Task 2.
- Produces:
  - Constants `IntentRequest, IntentFYI, IntentDecision, IntentHandoff, IntentReply` and `Resolved, Declined, Deferred, Accepted`.
  - `type Item struct { ID, Sender, Recipient, Intent, Body string; Urgent bool; CreatedAt, PulledAt, ShownAt, DisposedAt time.Time; Disposition, Answer string }`, `(Item).Open() bool`, `(Item).Obligation() bool`.
  - `NewItemID() string`, `ErrNoItem`, `ErrDisposed`.
  - `(*Store).Send(sender, recipient, intent, body string, urgent bool, now time.Time) (id string, created bool, err error)`.
  - `(*Store).Listed(recipient string) ([]Item, error)` (read-only), `(*Store).Inbox(recipient string, now time.Time) ([]Item, error)` (marks pulled), `(*Store).Item(id string) (Item, error)`, `(*Store).MarkShown(id string, now time.Time) error`, `(*Store).Reply(id, by, answer, disposition string, now time.Time) (replyID string, err error)`, `(*Store).Obligations(recipient string) ([]Item, error)`.
  - `Excerpt(body string, runes int) string`, `FormatAge(d time.Duration) string`.

- [ ] **Step 1: Write the failing tests**

`internal/store/ids_test.go`:
```go
package store_test

import (
	"regexp"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestNewItemIDShapeAndUniqueness(t *testing.T) {
	// covers: R-MSG-044
	// 10,000 draws: 40 random bits make a birthday collision at 100,000 a 0.45% event per run,
	// which would flake the gate; at 10,000 it is 4.5e-5. Uniqueness in the store itself rests on
	// the primary key.
	re := regexp.MustCompile(`^m-[a-z2-7]{8}$`)
	seen := make(map[string]struct{}, 10000)
	for range 10000 {
		id := store.NewItemID()
		if !re.MatchString(id) {
			t.Fatalf("id %q does not match ^m-[a-z2-7]{8}$", id)
		}
		if _, dup := seen[id]; dup {
			t.Fatalf("duplicate id %q", id)
		}
		seen[id] = struct{}{}
	}
}
```

`internal/store/format_test.go`:
```go
package store_test

import (
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestFormatAge(t *testing.T) {
	// covers: R-MSG-045
	cases := map[time.Duration]string{
		0:                            "0m",
		59 * time.Second:             "0m",
		61 * time.Second:             "1m",
		3*time.Hour + 10*time.Minute: "3h",
		50 * time.Hour:               "2d",
		-time.Minute:                 "0m",
	}
	for d, want := range cases {
		if got := store.FormatAge(d); got != want {
			t.Errorf("FormatAge(%v) = %q, want %q", d, got, want)
		}
	}
}

func TestExcerpt(t *testing.T) {
	// covers: R-MSG-031
	cases := []struct{ in, want string }{
		{"short", "short"},
		{"a\nb\tc\r\nd", "a b c d"},
		{strings.Repeat("é", 100), strings.Repeat("é", 80)},
		{"", ""},
	}
	for _, c := range cases {
		if got := store.Excerpt(c.in, 80); got != c.want {
			t.Errorf("Excerpt(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func FuzzExcerpt(f *testing.F) {
	// Review Focus 2: arbitrary bytes never break the one-line, 80-rune promise.
	// covers: R-MSG-031
	f.Add("hello\nworld\tagain")
	f.Add(strings.Repeat("é", 100))
	f.Add("\xff\xfe invalid utf-8 \xc0")
	f.Add("")
	f.Fuzz(func(t *testing.T, body string) {
		got := store.Excerpt(body, 80)
		if strings.ContainsAny(got, "\n\r\t") {
			t.Fatalf("excerpt %q spans lines", got)
		}
		if utf8.RuneCountInString(got) > 80 {
			t.Fatalf("excerpt has %d runes", utf8.RuneCountInString(got))
		}
		if utf8.ValidString(body) && !utf8.ValidString(got) {
			t.Fatalf("excerpt %q split a rune of a valid body", got)
		}
	})
}
```

`internal/store/items_test.go`:
```go
package store_test

import (
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// tree opens a store with root A and children B and C of A.
func tree(t *testing.T) *store.Store {
	t.Helper()
	s := mustOpen(t, freshPath(t))
	for _, row := range [][2]string{{"s-aaaaaaaaaaaa", store.Operator}, {"s-bbbbbbbbbbbb", "s-aaaaaaaaaaaa"}, {"s-cccccccccccc", "s-aaaaaaaaaaaa"}} {
		if err := s.AddSession(row[0], "scope", row[1], now); err != nil {
			t.Fatal(err)
		}
	}
	return s
}

func ids(items []store.Item) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.ID)
	}
	return out
}

func TestSendIsIdempotentPerContent(t *testing.T) {
	// covers: R-MSG-014, R-MSG-015
	s := tree(t)
	id1, created, err := s.Send("s-bbbbbbbbbbbb", "s-aaaaaaaaaaaa", store.IntentRequest, "same", false, now)
	if err != nil || !created {
		t.Fatalf("first send: %v, created %v", err, created)
	}
	id2, created, err := s.Send("s-bbbbbbbbbbbb", "s-aaaaaaaaaaaa", store.IntentRequest, "same", true, now.Add(time.Hour))
	if err != nil || created || id2 != id1 {
		t.Fatalf("repeat: %v, created %v, id %s vs %s", err, created, id2, id1)
	}
	if _, err := s.Reply(id1, "s-aaaaaaaaaaaa", "done", store.Resolved, now); err != nil {
		t.Fatal(err)
	}
	id3, created, err := s.Send("s-bbbbbbbbbbbb", "s-aaaaaaaaaaaa", store.IntentRequest, "same", false, now)
	if err != nil || created || id3 != id1 {
		t.Fatalf("repeat after disposal: %v, created %v (the key is permanent)", err, created)
	}
	_, created, err = s.Send("s-bbbbbbbbbbbb", "s-aaaaaaaaaaaa", store.IntentRequest, "other", false, now)
	if err != nil || !created {
		t.Fatalf("different body: %v, created %v", err, created)
	}
	_, created, err = s.Send("s-bbbbbbbbbbbb", "s-aaaaaaaaaaaa", store.IntentFYI, "same", false, now)
	if err != nil || !created {
		t.Fatalf("same body, other intent: %v, created %v", err, created)
	}
	if _, _, err := s.Send("s-bbbbbbbbbbbb", "s-nope", store.IntentRequest, "x", false, now); err == nil {
		t.Fatal("a recipient that does not exist was accepted")
	}
}

func TestInboxListsOpenItemsAndMarksThemPulled(t *testing.T) {
	// covers: R-MSG-036, R-MSG-041, R-MSG-043, R-MSG-022, R-MSG-030
	s := tree(t)
	req, _, _ := s.Send("s-bbbbbbbbbbbb", "s-aaaaaaaaaaaa", store.IntentRequest, "req", false, now)
	fyi, _, _ := s.Send("s-bbbbbbbbbbbb", "s-aaaaaaaaaaaa", store.IntentFYI, "fyi", false, now.Add(time.Second))
	dec, _, _ := s.Send("s-cccccccccccc", "s-aaaaaaaaaaaa", store.IntentDecision, "dec", true, now.Add(2*time.Second))
	if _, _, err := s.Send("s-aaaaaaaaaaaa", "s-bbbbbbbbbbbb", store.IntentRequest, "not mine", false, now); err != nil {
		t.Fatal(err)
	}
	listed, err := s.Listed("s-aaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	if got := fmt.Sprint(ids(listed)); got != fmt.Sprint([]string{req, fyi, dec}) {
		t.Fatalf("Listed = %s", got)
	}
	if !listed[0].PulledAt.IsZero() {
		t.Fatal("Listed must not pull")
	}
	items, err := s.Inbox("s-aaaaaaaaaaaa", now.Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	if len(items) != 3 || !items[0].PulledAt.Equal(now.Add(time.Minute)) || !items[2].Urgent {
		t.Fatalf("Inbox = %+v", items)
	}
	again, _ := s.Inbox("s-aaaaaaaaaaaa", now.Add(2*time.Minute))
	if len(again) != 3 || !again[0].PulledAt.Equal(now.Add(time.Minute)) {
		t.Fatalf("a pulled item must stay open and keep its first pull: %+v", again)
	}

	// An fyi leaves the list once shown; a request does not.
	for _, id := range []string{fyi, req} {
		if err := s.MarkShown(id, now.Add(3*time.Minute)); err != nil {
			t.Fatal(err)
		}
	}
	listed, _ = s.Listed("s-aaaaaaaaaaaa")
	if got := fmt.Sprint(ids(listed)); got != fmt.Sprint([]string{req, dec}) {
		t.Fatalf("after show: %s", got)
	}
	// A deferred item stays listed and marked; a resolved one leaves.
	if _, err := s.Reply(req, "s-aaaaaaaaaaaa", "later", store.Deferred, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reply(dec, "s-aaaaaaaaaaaa", "yes", store.Accepted, now); err != nil {
		t.Fatal(err)
	}
	listed, _ = s.Listed("s-aaaaaaaaaaaa")
	if len(listed) != 1 || listed[0].ID != req || listed[0].Disposition != store.Deferred {
		t.Fatalf("after replies: %+v", listed)
	}
	// The answers arrived as reply items for the senders.
	b, _ := s.Listed("s-bbbbbbbbbbbb")
	if len(b) != 2 || b[1].Intent != store.IntentReply || b[1].Body != "later" || b[1].Sender != "s-aaaaaaaaaaaa" {
		t.Fatalf("B's inbox: %+v", b)
	}
	if _, err := s.Item("m-nope"); !errors.Is(err, store.ErrNoItem) {
		t.Fatalf("unknown item: %v", err)
	}
	if err := s.MarkShown("m-nope", now); !errors.Is(err, store.ErrNoItem) {
		t.Fatalf("MarkShown unknown: %v", err)
	}
}

func TestReplyDisposesOnceAndAnswersTheSender(t *testing.T) {
	// covers: R-MSG-020, R-MSG-022, R-MSG-025, R-MSG-043
	s := tree(t)
	req, _, _ := s.Send("s-bbbbbbbbbbbb", "s-aaaaaaaaaaaa", store.IntentRequest, "may I?", false, now)
	later := now.Add(time.Minute)
	replyID, err := s.Reply(req, "s-aaaaaaaaaaaa", "yes", store.Resolved, later)
	if err != nil {
		t.Fatal(err)
	}
	it, err := s.Item(req)
	if err != nil || it.Disposition != store.Resolved || it.Answer != "yes" || !it.DisposedAt.Equal(later) {
		t.Fatalf("Item = %+v, %v", it, err)
	}
	rep, err := s.Item(replyID)
	if err != nil || rep.Intent != store.IntentReply || rep.Recipient != "s-bbbbbbbbbbbb" || rep.Sender != "s-aaaaaaaaaaaa" || rep.Body != "yes" {
		t.Fatalf("reply item = %+v, %v", rep, err)
	}
	if rep.Obligation() {
		t.Fatal("a reply item must carry no obligation")
	}
	if _, err := s.Reply(req, "s-aaaaaaaaaaaa", "no", store.Declined, later); !errors.Is(err, store.ErrDisposed) {
		t.Fatalf("second reply: %v, want ErrDisposed", err)
	}
	it, _ = s.Item(req)
	if it.Answer != "yes" {
		t.Fatalf("first answer changed to %q", it.Answer)
	}
	// Deferred may be answered again; the operator may reply (empty id).
	q, _, _ := s.Send("s-aaaaaaaaaaaa", store.Operator, store.IntentRequest, "op?", false, now)
	if _, err := s.Reply(q, store.Operator, "wait", store.Deferred, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reply(q, store.Operator, "go", store.Resolved, now); err != nil {
		t.Fatal(err)
	}
	a, _ := s.Listed("s-aaaaaaaaaaaa")
	if len(a) != 2 || a[0].Sender != store.Operator || a[1].Body != "go" {
		t.Fatalf("A's inbox: %+v", a)
	}
	if _, err := s.Reply("m-nope", store.Operator, "x", store.Resolved, now); !errors.Is(err, store.ErrNoItem) {
		t.Fatalf("Reply unknown: %v", err)
	}
}

func TestObligations(t *testing.T) {
	// request, decision and handoff oblige until disposed; deferred still does; fyi and reply never.
	// covers: R-MSG-040, R-MSG-041, R-MSG-043, R-MSG-022
	s := tree(t)
	var want []string
	for i, intent := range []string{store.IntentRequest, store.IntentDecision, store.IntentHandoff} {
		id, _, _ := s.Send("s-bbbbbbbbbbbb", "s-aaaaaaaaaaaa", intent, intent, false, now.Add(time.Duration(i)*time.Second))
		want = append(want, id)
	}
	fyi, _, _ := s.Send("s-bbbbbbbbbbbb", "s-aaaaaaaaaaaa", store.IntentFYI, "fyi", false, now)
	got, err := s.Obligations("s-aaaaaaaaaaaa")
	if err != nil || fmt.Sprint(ids(got)) != fmt.Sprint(want) {
		t.Fatalf("Obligations = %v, %v; want %v", ids(got), err, want)
	}
	if _, err := s.Reply(want[0], "s-aaaaaaaaaaaa", "later", store.Deferred, now); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Reply(want[1], "s-aaaaaaaaaaaa", "ok", store.Accepted, now); err != nil {
		t.Fatal(err)
	}
	got, _ = s.Obligations("s-aaaaaaaaaaaa")
	if fmt.Sprint(ids(got)) != fmt.Sprint([]string{want[0], want[2]}) {
		t.Fatalf("after replies: %v", ids(got))
	}
	it, _ := s.Item(fyi)
	if it.Obligation() || !it.Open() {
		t.Fatalf("fyi = %+v", it)
	}
}

func TestConcurrentSendsNeitherLoseNorDuplicate(t *testing.T) {
	// Forty senders (one store handle each, like one process each) send twenty distinct bodies
	// twice: exactly twenty items exist.
	// covers: R-MSG-003
	path := freshPath(t)
	seed := mustOpen(t, path)
	if err := seed.AddSession("s-aaaaaaaaaaaa", "root", store.Operator, now); err != nil {
		t.Fatal(err)
	}
	if err := seed.AddSession("s-bbbbbbbbbbbb", "child", "s-aaaaaaaaaaaa", now); err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 40)
	for i := range 40 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			s, err := store.Open(path)
			if err != nil {
				errs <- err
				return
			}
			defer func() { _ = s.Close() }()
			body := fmt.Sprintf("body %02d", i%20)
			if _, _, err := s.Send("s-bbbbbbbbbbbb", "s-aaaaaaaaaaaa", store.IntentRequest, body, false, now); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	items, err := seed.Listed("s-aaaaaaaaaaaa")
	if err != nil {
		t.Fatal(err)
	}
	bodies := map[string]bool{}
	for _, it := range items {
		bodies[it.Body] = true
	}
	if len(items) != 20 || len(bodies) != 20 {
		t.Fatalf("%d items, %d distinct bodies; want 20 and 20", len(items), len(bodies))
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/store/`
Expected: FAIL to build (`undefined: store.NewItemID`, `store.IntentRequest`, …).

- [ ] **Step 3: Implement ids, formatting and items**

`internal/store/ids.go`:
```go
package store

import (
	"crypto/rand"
	"encoding/base32"
	"strings"
)

// NewItemID returns a fresh item id: "m-" plus 8 lowercase base32 characters (R-MSG-044). Five
// random bytes encode to exactly eight characters without padding.
//
// ponytail: 40 bits and no retry on a primary-key collision; the odds at the scale of one
// project's store are below 1e-8 per insert. Add a retry in Send if a collision is ever seen.
func NewItemID() string {
	var b [5]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err) // crypto/rand cannot fail on supported platforms
	}
	return "m-" + strings.ToLower(base32.StdEncoding.EncodeToString(b[:]))
}
```

`internal/store/format.go`:
```go
package store

import (
	"fmt"
	"strings"
	"time"
)

var oneLine = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ", "\t", " ")

// Excerpt returns at most n runes of body on one line: newlines, carriage returns and tabs
// become spaces (R-MSG-031). Shared by inbox and, in P4, the worker's hints.
func Excerpt(body string, n int) string {
	s := oneLine.Replace(body)
	if runes := []rune(s); len(runes) > n {
		return string(runes[:n])
	}
	return s
}

// FormatAge prints d as its largest whole unit of minutes, hours or days, floored (R-MSG-045).
func FormatAge(d time.Duration) string {
	if d < 0 {
		d = 0
	}
	switch {
	case d >= 24*time.Hour:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	case d >= time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
}
```

`internal/store/items.go`:
```go
package store

import (
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"time"
)

// Intents (R-MSG-010, R-MSG-049). Reply items are created only by Reply.
const (
	IntentRequest  = "request"
	IntentFYI      = "fyi"
	IntentDecision = "decision"
	IntentHandoff  = "handoff"
	IntentReply    = "reply"
)

// Dispositions (R-MSG-020).
const (
	Resolved = "resolved"
	Declined = "declined"
	Deferred = "deferred"
	Accepted = "accepted"
)

var (
	// ErrNoItem is returned for an item id the store does not know.
	ErrNoItem = errors.New("no such item")
	// ErrDisposed is returned by Reply on an item that already has a final disposition (R-MSG-025).
	ErrDisposed = errors.New("item already disposed")
)

// Item is one message. Sender and Recipient are session ids or Operator.
type Item struct {
	ID, Sender, Recipient, Intent, Body     string
	Urgent                                  bool
	CreatedAt, PulledAt, ShownAt, DisposedAt time.Time
	Disposition, Answer                     string
}

// Open reports whether the item still awaits an answer: no disposition, or deferred (R-MSG-043).
func (it Item) Open() bool { return it.Disposition == "" || it.Disposition == Deferred }

// Obligation reports whether the item, while open, blocks its recipient's close (R-MSG-040):
// request, decision and handoff do; fyi and reply are labels only (R-MSG-041, R-MSG-022).
func (it Item) Obligation() bool {
	return it.Open() && (it.Intent == IntentRequest || it.Intent == IntentDecision || it.Intent == IntentHandoff)
}

const (
	selectItem = `SELECT id, sender, recipient, intent, body, urgent, created_at, pulled_at, shown_at,
		disposition, answer, disposed_at FROM items`
	insertItem = `INSERT INTO items (id, key, sender, recipient, intent, body, urgent, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`
	// listed is the inbox condition (R-MSG-036, R-MSG-041, R-MSG-043): open, and for an fyi or
	// reply, not shown yet. The recipient parameter uses IS so NULL (the operator) matches.
	listed = ` recipient IS ? AND (disposition IS NULL OR disposition = 'deferred')
		AND NOT (intent IN ('fyi', 'reply') AND shown_at IS NOT NULL)`
	obliged = ` recipient IS ? AND (disposition IS NULL OR disposition = 'deferred')
		AND intent IN ('request', 'decision', 'handoff')`
	inOrder = ` ORDER BY created_at, rowid`
)

// itemKey is the idempotency key: sender, recipient, intent and body (R-MSG-014).
func itemKey(parts ...string) string {
	h := sha256.New()
	for _, p := range parts {
		h.Write([]byte(p))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil))
}

// Send records an item unless one with the same key exists, open or disposed (R-MSG-014,
// R-MSG-015). It returns the item's id and whether it was created by this call.
func (s *Store) Send(sender, recipient, intent, body string, urgent bool, now time.Time) (id string, created bool, err error) {
	key := itemKey(sender, recipient, intent, body)
	err = s.inTx(func(tx *sql.Tx) error {
		err := tx.QueryRow(`SELECT id FROM items WHERE key = ?`, key).Scan(&id)
		if err == nil {
			return nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		id, created = NewItemID(), true
		_, err = tx.Exec(insertItem, id, key, nullable(sender), nullable(recipient), intent, body, urgent, stamp(now))
		return err
	})
	if err != nil {
		return "", false, err
	}
	return id, created, nil
}

// Listed returns the recipient's inbox without side effects, in arrival order.
func (s *Store) Listed(recipient string) ([]Item, error) {
	return queryItems(s.db, selectItem+` WHERE`+listed+inOrder, nullable(recipient))
}

// Inbox lists the recipient's inbox and marks every listed item pulled, keeping an earlier pull
// (R-MSG-036); a pulled item stays open until disposed.
func (s *Store) Inbox(recipient string, now time.Time) (items []Item, err error) {
	err = s.inTx(func(tx *sql.Tx) error {
		if _, err := tx.Exec(`UPDATE items SET pulled_at = ? WHERE pulled_at IS NULL AND`+listed,
			stamp(now), nullable(recipient)); err != nil {
			return err
		}
		items, err = queryItems(tx, selectItem+` WHERE`+listed+inOrder, nullable(recipient))
		return err
	})
	return items, err
}

// Obligations lists the open items that block the recipient's close (R-MSG-040).
func (s *Store) Obligations(recipient string) ([]Item, error) {
	return queryItems(s.db, selectItem+` WHERE`+obliged+inOrder, nullable(recipient))
}

// Item returns one item or ErrNoItem.
func (s *Store) Item(id string) (Item, error) {
	return scanItem(s.db.QueryRow(selectItem+` WHERE id = ?`, id))
}

// MarkShown records that the item's full body was read; the first stamp is kept, and the item
// counts as pulled too (R-MSG-041).
func (s *Store) MarkShown(id string, now time.Time) error {
	res, err := s.db.Exec(`UPDATE items SET shown_at = COALESCE(shown_at, ?), pulled_at = COALESCE(pulled_at, ?)
		WHERE id = ?`, stamp(now), stamp(now), id)
	if err != nil {
		return err
	}
	if n, err := res.RowsAffected(); err != nil || n == 0 {
		return errors.Join(err, ErrNoItem)
	}
	return nil
}

// Reply records the answer and disposition on the item and delivers the answer to the item's
// sender as a new reply item from by (R-MSG-020, R-MSG-022). A deferred item may be answered
// again; any other disposed item is ErrDisposed (R-MSG-025, R-MSG-043).
func (s *Store) Reply(id, by, answer, disposition string, now time.Time) (replyID string, err error) {
	err = s.inTx(func(tx *sql.Tx) error {
		it, err := scanItem(tx.QueryRow(selectItem+` WHERE id = ?`, id))
		if err != nil {
			return err
		}
		if !it.Open() {
			return ErrDisposed
		}
		if _, err := tx.Exec(`UPDATE items SET disposition = ?, answer = ?, disposed_at = ? WHERE id = ?`,
			disposition, answer, stamp(now), id); err != nil {
			return err
		}
		replyID = NewItemID()
		_, err = tx.Exec(insertItem, replyID, "reply:"+replyID, nullable(by), nullable(it.Sender), IntentReply, answer, false, stamp(now))
		return err
	})
	if err != nil {
		return "", err
	}
	return replyID, nil
}

type querier interface {
	Query(query string, args ...any) (*sql.Rows, error)
}

type scanner interface {
	Scan(dest ...any) error
}

func queryItems(q querier, query string, args ...any) (items []Item, err error) {
	rows, err := q.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { err = errors.Join(err, rows.Close()) }()
	for rows.Next() {
		it, err := scanItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, it)
	}
	return items, rows.Err()
}

func scanItem(r scanner) (Item, error) {
	var (
		it                                                       Item
		sender, recipient, created, pulled, shown, disp, answer sql.NullString
		disposed                                                 sql.NullString
	)
	err := r.Scan(&it.ID, &sender, &recipient, &it.Intent, &it.Body, &it.Urgent, &created, &pulled, &shown,
		&disp, &answer, &disposed)
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNoItem
	}
	if err != nil {
		return Item{}, err
	}
	it.Sender, it.Recipient, it.Disposition, it.Answer = sender.String, recipient.String, disp.String, answer.String
	it.CreatedAt, it.PulledAt, it.ShownAt, it.DisposedAt = nullStamp(created), nullStamp(pulled), nullStamp(shown), nullStamp(disposed)
	return it, nil
}
```

- [ ] **Step 4: Run the tests, race, repeats, a short fuzz, and lint**

Run: `go test -race ./internal/store/ && go test -count=4 ./internal/store/ && go test -run '^$' -fuzz FuzzExcerpt -fuzztime 10s ./internal/store/ && make lint`
Expected: PASS. If `lll` flags a long test line, wrap it. If `funlen` flags `TestInboxListsOpenItemsAndMarksThemPulled`, the main lint config has `tests: false`, so it will not; leave the test as one story.

- [ ] **Step 5: Commit**

```bash
git add internal/store
git commit -m "feat(store): items with idempotent send, pulled inbox, reply and obligations"
```

---

### Task 4: The caller seam, the `store` script command, and `ness send`

**Files:**
- Create: `internal/cli/session.go`, `internal/cli/send.go`, `internal/testkit/store.go`, `internal/cli/testdata/script/send.txtar`, `internal/cli/testdata/script/send_routes.txtar`, `internal/cli/testdata/script/send_concurrent.txtar`
- Modify: `internal/cli/cli.go` (register `newSendCmd`), `internal/cli/hook.go` (drop the blank import), `internal/testkit/testkit.go` (register `store`)

**Interfaces:**
- Consumes: `store.Locate/Open/Session/Send/Listed`, `loadProject`, `refuse`, `usage`, `say`, `Deps.Session`, `Deps.Now`.
- Produces:
  - `type caller struct { id string; sess store.Session }`, `(caller).operator() bool`, `(caller).root() bool`, `(caller).addressed(it store.Item) bool`, `displayName(id string) string`.
  - `openStore(d Deps, p *project) (*store.Store, caller, error)`, `withStore(d Deps, fn func(s *store.Store, c caller) error) error`.
  - `readBody(arg string) (string, error)` (shared with `reply`), `resolveRecipient(s *store.Store, c caller, to string) (string, error)`.
  - Script command `store add-session|close-session|session|item|items|pulled|last …` (usage in the code comment below).

- [ ] **Step 1: Write the failing scripts**

`internal/cli/testdata/script/send.txtar`:
```
# ness send: every flag value, @FILE bodies, the permanent idempotency key, one line of output
# without the body, and the removed --request-key flag. An empty or unreadable body is a usage
# error that creates nothing (Review Focus 1).
# covers: R-MSG-010, R-MSG-014, R-MSG-015, R-MSG-016, R-MSG-019, R-MSG-049
cd repo
exec ness init --gate true --base main --yes
store add-session s-aaaaaaaaaaaa root
store add-session s-bbbbbbbbbbbb child-b s-aaaaaaaaaaaa
env NESS_SESSION=s-bbbbbbbbbbbb
exec ness send --to manager --body 'need a decision on the API'
stdout '^sent m-[a-z2-7]{8} to s-aaaaaaaaaaaa$'
! stdout 'decision on the API'
! stderr .
store items s-aaaaaaaaaaaa 1
# The same content again is the same item.
exec ness send --to manager --body 'need a decision on the API'
stdout '^already sent m-[a-z2-7]{8}$'
! stdout 'decision on the API'
! stderr .
store items s-aaaaaaaaaaaa 1
# Different content is a second item.
exec ness send --to manager --body 'second body'
stdout '^sent m-'
store items s-aaaaaaaaaaaa 2
# Every intent and --urgent are accepted; reply and anything else are usage errors.
exec ness send --to manager --body fyi-body --intent fyi
exec ness send --to manager --body decision-body --intent decision
exec ness send --to manager --body handoff-body --intent handoff
exec ness send --to manager --body urgent-body --urgent
store items s-aaaaaaaaaaaa 6
exits 2 ness send --to manager --body x --intent reply
stderr 'accepted values are request, fyi, decision, handoff'
exits 2 ness send --to manager --body x --intent question
store items s-aaaaaaaaaaaa 6
# @FILE reads the body from the file; a missing file, an empty body or a missing flag is a
# usage error.
exec ness send --to manager --body @../body.txt
stdout '^sent m-'
store items s-aaaaaaaaaaaa 7
exits 2 ness send --to manager --body @../nope.txt
exits 2 ness send --to manager --body @
exits 2 ness send --to manager --body ''
stderr 'empty'
exits 2 ness send --body x
stderr '--to'
exits 2 ness send --to manager
stderr '--body'
store items s-aaaaaaaaaaaa 7
# The cut flag does not exist.
exits 2 ness send --to manager --body x --request-key k
stderr 'unknown flag'

-- body.txt --
line one
	line two with a tab
```

`internal/cli/testdata/script/send_routes.txtar`:
```
# Routing: a session talks to its manager and its children; siblings go through the manager;
# only a root addresses the operator; the operator addresses roots and has no manager; closed
# or unknown recipients and unknown or closed callers are refused; a refusal creates nothing;
# outside a ness project send refuses.
# covers: R-MSG-013, R-MSG-017, R-MSG-018, R-CFG-053
cd repo
exec ness init --gate true --base main --yes
store add-session s-aaaaaaaaaaaa root
store add-session s-bbbbbbbbbbbb child-b s-aaaaaaaaaaaa
store add-session s-cccccccccccc child-c s-aaaaaaaaaaaa
store add-session s-dddddddddddd child-d s-bbbbbbbbbbbb
# child -> manager lands in the root's inbox (by name or by id); child -> its own child works.
env NESS_SESSION=s-bbbbbbbbbbbb
exec ness send --to manager --body 'to my manager'
stdout '^sent m-[a-z2-7]{8} to s-aaaaaaaaaaaa$'
store items s-aaaaaaaaaaaa 1
exec ness send --to s-aaaaaaaaaaaa --body 'to my manager by id'
store items s-aaaaaaaaaaaa 2
exec ness send --to s-dddddddddddd --body 'to my child'
store items s-dddddddddddd 1
# root -> manager is an operator item; so is root -> operator; root -> child works.
env NESS_SESSION=s-aaaaaaaaaaaa
exec ness send --to manager --body 'root to operator'
stdout '^sent m-[a-z2-7]{8} to operator$'
store items operator 1
exec ness send --to operator --body 'root to operator again'
store items operator 2
exec ness send --to s-bbbbbbbbbbbb --body 'root to child'
store items s-bbbbbbbbbbbb 1
# Siblings, a grandparent, and a child addressing the operator are refused with nothing created.
env NESS_SESSION=s-bbbbbbbbbbbb
exits 4 ness send --to s-cccccccccccc --body 'sibling'
stderr 'siblings talk through the manager'
store items s-cccccccccccc 0
exits 4 ness send --to operator --body 'child to operator'
stderr 'only a root'
store items operator 2
env NESS_SESSION=s-dddddddddddd
exits 4 ness send --to s-aaaaaaaaaaaa --body 'grandchild to root'
store items s-aaaaaaaaaaaa 2
# The operator (no NESS_SESSION) talks to roots only and has no manager.
env NESS_SESSION=
exec ness send --to s-aaaaaaaaaaaa --body 'operator to root'
stdout '^sent m-[a-z2-7]{8} to s-aaaaaaaaaaaa$'
store items s-aaaaaaaaaaaa 3
exits 4 ness send --to s-bbbbbbbbbbbb --body 'operator to child'
stderr 'not a root'
exits 4 ness send --to manager --body 'operator to manager'
stderr 'no manager'
exits 4 ness send --to operator --body 'operator to operator'
store items s-bbbbbbbbbbbb 1
# Closed and unknown recipients, and an unknown or closed caller (Review Focus 4).
store close-session s-cccccccccccc
env NESS_SESSION=s-aaaaaaaaaaaa
exits 4 ness send --to s-cccccccccccc --body 'to a closed child'
stderr 'closed'
exits 4 ness send --to s-eeeeeeeeeeee --body 'to nobody'
stderr 'not a session of this project'
env NESS_SESSION=s-eeeeeeeeeeee
exits 4 ness send --to manager --body 'from nobody'
stderr 'NESS_SESSION=s-eeeeeeeeeeee is not a session of this project'
env NESS_SESSION=s-cccccccccccc
exits 4 ness send --to manager --body 'from a closed session'
stderr 'closed'
store items s-aaaaaaaaaaaa 3
# Outside a ness project, send refuses (R-CFG-053).
cd $WORK
env NESS_SESSION=
exits 4 ness send --to s-aaaaaaaaaaaa --body x
```

`internal/cli/testdata/script/send_concurrent.txtar`:
```
# Twenty concurrent ness send processes with distinct bodies produce exactly twenty items.
# covers: R-MSG-003
cd repo
exec ness init --gate true --base main --yes
store add-session s-aaaaaaaaaaaa root
store add-session s-bbbbbbbbbbbb child-b s-aaaaaaaaaaaa
env NESS_SESSION=s-bbbbbbbbbbbb
exec ness send --to manager --body 'body 01' &
exec ness send --to manager --body 'body 02' &
exec ness send --to manager --body 'body 03' &
exec ness send --to manager --body 'body 04' &
exec ness send --to manager --body 'body 05' &
exec ness send --to manager --body 'body 06' &
exec ness send --to manager --body 'body 07' &
exec ness send --to manager --body 'body 08' &
exec ness send --to manager --body 'body 09' &
exec ness send --to manager --body 'body 10' &
exec ness send --to manager --body 'body 11' &
exec ness send --to manager --body 'body 12' &
exec ness send --to manager --body 'body 13' &
exec ness send --to manager --body 'body 14' &
exec ness send --to manager --body 'body 15' &
exec ness send --to manager --body 'body 16' &
exec ness send --to manager --body 'body 17' &
exec ness send --to manager --body 'body 18' &
exec ness send --to manager --body 'body 19' &
exec ness send --to manager --body 'body 20' &
wait
stdout -count=20 '^sent m-[a-z2-7]{8} to s-aaaaaaaaaaaa$'
store items s-aaaaaaaaaaaa 20
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run 'TestScripts/send'`
Expected: FAIL — `store` is an unknown script command (`send.txtar` stops at `store add-session`).

- [ ] **Step 3: The `store` script command**

`internal/testkit/store.go`:
```go
package testkit

import (
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/rogpeppe/go-internal/testscript"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// storeCmd drives the project's store from a script through the store API. It stands in for
// the session tree until P3 (sessions are inserted directly) and reads state the commands do
// not print:
//
//	store add-session ID SCOPE [PARENT]   insert an open session; no PARENT makes a root
//	store close-session ID                mark it closed
//	store session ID FIELD=VALUE...       fail unless every field matches (negatable with !)
//	store item ID FIELD=VALUE...          fail unless every field matches (negatable with !)
//	store items RECIPIENT N               fail unless RECIPIENT (an id or operator) lists N items
//	store pulled RECIPIENT N              fail unless N of RECIPIENT's listed items are pulled
//	store last RECIPIENT VAR              set $VAR to the id of RECIPIENT's newest listed item
//
// Session fields: parent, scope, closed, ready (false or the RFC 3339 stamp), boundary.
// Item fields: sender, recipient, intent, pulled, shown (false or the stamp), disposition, answer.
func storeCmd(ts *testscript.TestScript, neg bool, args []string) {
	if len(args) < 2 {
		ts.Fatalf("usage: store add-session|close-session|session|item|items|pulled|last ...")
	}
	path, err := store.Locate(ts.MkAbs("."))
	ts.Check(err)
	s, err := store.Open(path)
	ts.Check(err)
	defer func() { ts.Check(s.Close()) }()
	sub, ok := storeSubs[args[0]]
	if !ok {
		ts.Fatalf("store: unknown subcommand %s", args[0])
	}
	sub(ts, neg, s, args[1:])
}

var storeSubs = map[string]func(ts *testscript.TestScript, neg bool, s *store.Store, args []string){
	"add-session": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		if neg || len(args) < 2 || len(args) > 3 {
			ts.Fatalf("usage: store add-session ID SCOPE [PARENT]")
		}
		parent := store.Operator
		if len(args) == 3 {
			parent = args[2]
		}
		ts.Check(s.AddSession(args[0], args[1], parent, time.Now()))
	},
	"close-session": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		if neg || len(args) != 1 {
			ts.Fatalf("usage: store close-session ID")
		}
		ts.Check(s.CloseSession(args[0]))
	},
	"session": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		sess, err := s.Session(args[0])
		ts.Check(err)
		matchFields(ts, neg, args[1:], map[string]string{
			"parent": sess.Parent, "scope": sess.Scope, "closed": strconv.FormatBool(sess.Closed),
			"ready": stampOrFalse(sess.ReadyAt), "boundary": sess.BoundaryNote,
		})
	},
	"item": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		it, err := s.Item(args[0])
		ts.Check(err)
		matchFields(ts, neg, args[1:], map[string]string{
			"sender": it.Sender, "recipient": it.Recipient, "intent": it.Intent,
			"pulled": stampOrFalse(it.PulledAt), "shown": stampOrFalse(it.ShownAt),
			"disposition": it.Disposition, "answer": it.Answer,
		})
	},
	"items": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		items := listed(ts, s, args)
		if (len(items) == wantCount(ts, args)) == neg {
			ts.Fatalf("%s lists %d items, want %s", args[0], len(items), args[1])
		}
	},
	"pulled": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		n := 0
		for _, it := range listed(ts, s, args) {
			if !it.PulledAt.IsZero() {
				n++
			}
		}
		if (n == wantCount(ts, args)) == neg {
			ts.Fatalf("%s has %d pulled items, want %s", args[0], n, args[1])
		}
	},
	"last": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		items := listed(ts, s, args)
		if neg || len(items) == 0 {
			ts.Fatalf("store last: %s lists no items", args[0])
		}
		ts.Setenv(args[1], items[len(items)-1].ID)
	},
}

func listed(ts *testscript.TestScript, s *store.Store, args []string) []store.Item {
	if len(args) != 2 {
		ts.Fatalf("usage: store items|pulled|last RECIPIENT N|VAR")
	}
	recipient := args[0]
	if recipient == "operator" {
		recipient = store.Operator
	}
	items, err := s.Listed(recipient)
	ts.Check(err)
	return items
}

func wantCount(ts *testscript.TestScript, args []string) int {
	n, err := strconv.Atoi(args[1])
	ts.Check(err)
	return n
}

func stampOrFalse(t time.Time) string {
	if t.IsZero() {
		return "false"
	}
	return t.UTC().Format(time.RFC3339)
}

// matchFields compares FIELD=VALUE pairs against got; with neg it fails when all of them match.
func matchFields(ts *testscript.TestScript, neg bool, want []string, got map[string]string) {
	if len(want) == 0 {
		ts.Fatalf("store: give FIELD=VALUE pairs to check")
	}
	all := true
	for _, kv := range want {
		k, v, ok := strings.Cut(kv, "=")
		gv, known := got[k]
		if !ok || !known {
			ts.Fatalf("store: unknown field %q; fields are %v", kv, slices.Sorted(maps.Keys(got)))
		}
		if gv != v {
			all = false
			ts.Logf("store: %s = %q, want %q", k, gv, v)
		}
	}
	if all == neg {
		ts.Fatalf("store: all fields match = %v", all)
	}
}
```

Register it in `internal/testkit/testkit.go`: add `"store": storeCmd,` to the map in `Cmds()` and a line to its doc comment:
```go
//	store SUB ...             drive the store through its API (see storeCmd)
```

- [ ] **Step 4: The caller seam and `ness send`**

`internal/cli/session.go`:
```go
package cli

import (
	"errors"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// caller is who runs a command: a session (NESS_SESSION) or the operator (R-SESS-037).
type caller struct {
	id   string        // store.Operator for the operator
	sess store.Session // zero for the operator
}

func (c caller) operator() bool { return c.id == store.Operator }

// root reports whether c is a root session: its manager is the operator (R-MSG-013).
func (c caller) root() bool { return !c.operator() && c.sess.Parent == store.Operator }

// addressed reports whether c may read and answer it: it is addressed to c, or to the operator
// while c is a root, since roots relay operator items in chat (R-MSG-023, R-MSG-024).
func (c caller) addressed(it store.Item) bool {
	return it.Recipient == c.id || (it.Recipient == store.Operator && c.root())
}

// displayName prints a session id, or "operator" for the empty id.
func displayName(id string) string {
	if id == store.Operator {
		return "operator"
	}
	return id
}

// openStore opens the project's store and resolves the caller. A store from a newer ness, and a
// NESS_SESSION the store does not know or has closed, are refusals: the process claims an
// identity it cannot hold.
func openStore(d Deps, p *project) (*store.Store, caller, error) {
	path, err := store.Locate(p.root)
	if err != nil {
		return nil, caller{}, refuse("%v", err)
	}
	s, err := store.Open(path)
	if err != nil {
		return nil, caller{}, refuse("%v", err)
	}
	c, err := identify(s, d.Session)
	if err != nil {
		return nil, caller{}, errors.Join(err, s.Close())
	}
	return s, c, nil
}

func identify(s *store.Store, id string) (caller, error) {
	if id == store.Operator {
		return caller{}, nil
	}
	sess, err := s.Session(id)
	if errors.Is(err, store.ErrNoSession) {
		return caller{}, refuse("NESS_SESSION=%s is not a session of this project; unset it, or run from a session ness opened", id)
	}
	if err != nil {
		return caller{}, err
	}
	if sess.Closed {
		return caller{}, refuse("session %s is closed; unset NESS_SESSION or open a new session", id)
	}
	return caller{id: id, sess: sess}, nil
}

// withStore loads the project, opens the store, identifies the caller and runs fn.
func withStore(d Deps, fn func(s *store.Store, c caller) error) (err error) {
	p, err := loadProject(d)
	if err != nil {
		return err
	}
	s, c, err := openStore(d, p)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, s.Close()) }()
	return fn(s, c)
}
```

`internal/cli/send.go`:
```go
package cli

import (
	"errors"
	"os"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// sendIntents are the intents a send may carry; reply items come only from ness reply (R-MSG-049).
var sendIntents = []string{store.IntentRequest, store.IntentFYI, store.IntentDecision, store.IntentHandoff}

type sendOpts struct {
	to, body, intent string
	urgent           bool
}

func newSendCmd(d Deps) *cobra.Command {
	var o sendOpts
	cmd := &cobra.Command{
		Use:   "send",
		Short: "Send a message to your manager, one of your children or (from a root) the operator",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			if o.to == "" {
				return usage("--to is required: a session id, manager or operator")
			}
			if !slices.Contains(sendIntents, o.intent) {
				return usage("--intent %s: accepted values are request, fyi, decision, handoff", o.intent)
			}
			body, err := readBody(o.body)
			if err != nil {
				return err
			}
			return withStore(d, func(s *store.Store, c caller) error {
				return runSend(d, s, c, o, body)
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.to, "to", "", "recipient: a session `ID`, manager or operator")
	f.StringVar(&o.body, "body", "", "the message `TEXT`, or @FILE to read it from a file")
	f.StringVar(&o.intent, "intent", store.IntentRequest, "request | fyi | decision | handoff")
	f.BoolVar(&o.urgent, "urgent", false, "ask the worker to hint the recipient at its next pass")
	return cmd
}

func runSend(d Deps, s *store.Store, c caller, o sendOpts, body string) error {
	to, err := resolveRecipient(s, c, o.to)
	if err != nil {
		return err
	}
	id, created, err := s.Send(c.id, to, o.intent, body, o.urgent, d.Now())
	if err != nil {
		return err
	}
	if !created {
		say(d.Stdout, "already sent %s", id)
		return nil
	}
	say(d.Stdout, "sent %s to %s", id, displayName(to))
	return nil
}

// readBody returns TEXT literally, or the bytes of FILE for @FILE (R-MSG-012). An empty body is
// a usage error: there is nothing to deliver.
func readBody(arg string) (string, error) {
	body := arg
	if strings.HasPrefix(arg, "@") {
		data, err := os.ReadFile(arg[1:]) //nolint:gosec // the caller names its own file
		if err != nil {
			return "", usage("--body %s: %v", arg, err)
		}
		body = string(data)
	}
	if body == "" {
		return "", usage("--body is empty; give the message text, or @FILE")
	}
	return body, nil
}

// resolveRecipient applies the routing rules (R-MSG-013, R-MSG-017, R-MSG-018): a session talks
// to its manager and its children, only a root addresses the operator, and the operator (no
// session) addresses roots. It returns the recipient id, store.Operator for the operator.
func resolveRecipient(s *store.Store, c caller, to string) (string, error) {
	switch to {
	case "operator":
		if !c.root() {
			return "", refuse("only a root session sends to the operator; send to manager instead")
		}
		return store.Operator, nil
	case "manager":
		if c.operator() {
			return "", refuse("the operator has no manager; name a session id with --to")
		}
		return c.sess.Parent, nil
	}
	target, err := s.Session(to)
	if errors.Is(err, store.ErrNoSession) {
		return "", refuse("%s is not a session of this project", to)
	}
	if err != nil {
		return "", err
	}
	if target.Closed {
		return "", refuse("session %s is closed", to)
	}
	switch {
	case target.ID == c.sess.Parent && !c.operator(), target.Parent == c.id && !c.operator():
		return to, nil
	case c.operator() && target.Parent == store.Operator:
		return to, nil
	case c.operator():
		return "", refuse("%s is not a root; the operator talks to roots", to)
	default:
		return "", refuse("%s is neither your manager nor one of your children; siblings talk through the manager", to)
	}
}
```

In `internal/cli/cli.go` register it: `root.AddCommand(newInitCmd(d), newSyncCmd(d), newHookCmd(d), newSendCmd(d))`. In `internal/cli/hook.go` delete the blank `store` import (the package is now imported by `session.go`). The missing `--body` case: with `o.body == ""` and no `@`, `readBody` returns the "empty" usage error, whose text must contain `--body` (it does).

- [ ] **Step 5: Run the scripts, race and lint**

Run: `go test -race ./internal/cli/ ./internal/testkit/ && make lint`
Expected: PASS. `send_concurrent.txtar` runs twenty processes against one fresh store. If `cyclop` flags `resolveRecipient`, split the final `switch` into a helper `allowed(c caller, target store.Session) (bool, string)` returning the refusal text.

- [ ] **Step 6: Commit**

```bash
git add internal/cli internal/testkit
git commit -m "feat(cli): ness send with routing rules, idempotent items and the NESS_SESSION seam"
```

---

### Task 5: `ness inbox`

**Files:**
- Create: `internal/cli/inbox.go`, `internal/cli/inbox_test.go`, `internal/cli/testdata/script/inbox.txtar`, `internal/cli/testdata/script/inbox_boundary.txtar`, `internal/cli/testdata/script/store_worktree.txtar`, `internal/cli/testdata/script/pin_patch.txtar`
- Modify: `internal/cli/cli.go` (register `newInboxCmd`)

**Interfaces:**
- Consumes: `withStore`, `caller.addressed`, `displayName`, `store.Inbox/Item/MarkShown/SetBoundary`, `store.Excerpt`, `store.FormatAge`, `refuse`, `say`.
- Produces: `newInboxCmd(d Deps) *cobra.Command`, `inboxLine(it store.Item, now time.Time) string` (tab-separated: id, from, intent [+ ` deferred`], age, excerpt).

- [ ] **Step 1: Write the failing tests**

`internal/cli/inbox_test.go` (white-box: the line layout is frozen here, R-MSG-045):
```go
package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

func TestInboxLineGolden(t *testing.T) {
	// covers: R-MSG-030, R-MSG-031, R-MSG-043, R-MSG-045
	now := time.Date(2026, 10, 8, 13, 10, 0, 0, time.UTC)
	cases := []struct {
		it   store.Item
		want string
	}{
		{
			store.Item{ID: "m-abcdefgh", Sender: "s-bbbbbbbbbbbb", Intent: "request", Body: "need a decision", CreatedAt: now.Add(-3*time.Hour - 10*time.Minute)},
			"m-abcdefgh\ts-bbbbbbbbbbbb\trequest\t3h\tneed a decision",
		},
		{
			store.Item{ID: "m-abcdefgh", Sender: store.Operator, Intent: "reply", Body: "yes\nmerge\tit", CreatedAt: now.Add(-59 * time.Second)},
			"m-abcdefgh\toperator\treply\t0m\tyes merge it",
		},
		{
			store.Item{ID: "m-abcdefgh", Sender: "s-bbbbbbbbbbbb", Intent: "handoff", Disposition: store.Deferred, Body: strings.Repeat("x", 100), CreatedAt: now.Add(-50 * time.Hour)},
			"m-abcdefgh\ts-bbbbbbbbbbbb\thandoff deferred\t2d\t" + strings.Repeat("x", 80),
		},
	}
	for _, c := range cases {
		if got := inboxLine(c.it, now); got != c.want {
			t.Errorf("inboxLine = %q\nwant        %q", got, c.want)
		}
	}
}
```

`internal/cli/testdata/script/inbox.txtar`:
```
# ness inbox: one line per open item (id, from, intent, age, 80-rune excerpt), nothing on an
# empty inbox, --show prints exact bytes and nothing else, listing marks items pulled, an fyi
# leaves the list once shown, --json lists the same items, the operator's inbox outside a session
# lists operator items, and a --show of someone else's item is refused (Review Focus 5).
# covers: R-MSG-011, R-MSG-012, R-MSG-030, R-MSG-032, R-MSG-033, R-MSG-035, R-MSG-036, R-MSG-041, R-MSG-045, R-MSG-047, R-MSG-048, R-CFG-053
cd repo
exec ness init --gate true --base main --yes
store add-session s-aaaaaaaaaaaa root
store add-session s-bbbbbbbbbbbb child-b s-aaaaaaaaaaaa
env NESS_SESSION=s-aaaaaaaaaaaa
exec ness inbox
! stdout .
! stderr .
exec ness inbox --json
cmp stdout ../empty.json
# Three items sent at 10:00; listed at 13:10 they are 3h old. A 10 KB body is cut at 80 runes.
exec sh -c 'head -c 10240 /dev/zero | tr "\0" x > ../big.txt'
env NESS_TEST_NOW=2026-10-08T10:00:00Z
env NESS_SESSION=s-bbbbbbbbbbbb
exec ness send --to manager --body @../big.txt
exec ness send --to manager --body @../multiline.txt --intent decision
store last s-aaaaaaaaaaaa DEC
exec ness send --to manager --body @../fyi.txt --intent fyi
store last s-aaaaaaaaaaaa FYI
env NESS_SESSION=s-aaaaaaaaaaaa
store pulled s-aaaaaaaaaaaa 0
env NESS_TEST_NOW=2026-10-08T13:10:00Z
exec ness inbox
stdout -count=3 '^m-[a-z2-7]{8}\t'
stdout '^m-[a-z2-7]{8}\ts-bbbbbbbbbbbb\trequest\t3h\tx{80}$'
! stdout 'x{81}'
stdout '^m-[a-z2-7]{8}\ts-bbbbbbbbbbbb\tdecision\t3h\tfirst line second line  third $'
stdout '^m-[a-z2-7]{8}\ts-bbbbbbbbbbbb\tfyi\t3h\tfyi body with two lines $'
store pulled s-aaaaaaaaaaaa 3
# Listing again shows the same items (pulled, still open).
exec ness inbox
stdout -count=3 '^m-[a-z2-7]{8}\t'
# --json lists the same items with the frozen fields.
exec ness inbox --json
stdout -count=3 '"id": "m-[a-z2-7]{8}"'
stdout '"schema": 1'
stdout '"intent": "decision"'
stdout '"age": "3h"'
! stdout 'x{81}'
# --show prints the exact bytes of one body and nothing else; the fyi then leaves the list.
exec ness inbox --show $FYI
cmp stdout ../fyi.txt
exec ness inbox --show $DEC
cmp stdout ../multiline.txt
exec ness inbox
stdout -count=2 '^m-[a-z2-7]{8}\t'
! stdout '\tfyi\t'
# Someone else's item cannot be shown and stays unshown; an unknown id is refused.
env NESS_SESSION=s-bbbbbbbbbbbb
exits 4 ness inbox --show $DEC
stderr 'addressed to s-aaaaaaaaaaaa'
exits 4 ness inbox --show m-nope
stderr 'no item m-nope'
# Days: an item from two days ago.
env NESS_TEST_NOW=2026-10-06T10:00:00Z
exec ness send --to manager --body old
env NESS_SESSION=s-aaaaaaaaaaaa
env NESS_TEST_NOW=2026-10-08T12:00:00Z
exec ness inbox
stdout '\trequest\t2d\told$'
# The operator's inbox (no NESS_SESSION) lists operator items; --boundary has no session there.
exec ness send --to operator --body 'for the operator'
env NESS_SESSION=
exec ness inbox
stdout '^m-[a-z2-7]{8}\ts-aaaaaaaaaaaa\trequest\t0m\tfor the operator$'
exits 4 ness inbox --boundary note
# Outside a ness project, inbox refuses (R-CFG-053).
cd $WORK
exits 4 ness inbox

-- empty.json --
{
  "items": [],
  "schema": 1
}
-- multiline.txt --
first line
second line
	third
-- fyi.txt --
fyi body
with two lines
```

`internal/cli/testdata/script/inbox_boundary.txtar`:
```
# ness inbox --boundary NOTE records a pending boundary for the session and still lists.
# covers: R-MSG-034
cd repo
exec ness init --gate true --base main --yes
store add-session s-aaaaaaaaaaaa root
store add-session s-bbbbbbbbbbbb child-b s-aaaaaaaaaaaa
env NESS_SESSION=s-bbbbbbbbbbbb
exec ness send --to manager --body hello
env NESS_SESSION=s-aaaaaaaaaaaa
store session s-aaaaaaaaaaaa boundary=
exec ness inbox --boundary 'plan written to docs/plan.md'
stdout '\trequest\t'
store session s-aaaaaaaaaaaa 'boundary=plan written to docs/plan.md'
exits 2 ness inbox --boundary x --show m-nope
```

`internal/cli/testdata/script/store_worktree.txtar`:
```
# One store per repository: a send from a worktree is in the primary checkout's inbox, and the
# file lives under the git common dir, not under the worktree.
# covers: R-MSG-001
cd repo
exec ness init --gate true --base main --yes
exec git add -A
exec git commit -q -m 'ness init'
exec git worktree add -q ../wt
store add-session s-aaaaaaaaaaaa root
store add-session s-bbbbbbbbbbbb child-b s-aaaaaaaaaaaa
cd ../wt
env NESS_SESSION=s-bbbbbbbbbbbb
exec ness send --to manager --body 'from the worktree'
cd ../repo
env NESS_SESSION=s-aaaaaaaaaaaa
exec ness inbox
stdout '^m-[a-z2-7]{8}\ts-bbbbbbbbbbbb\trequest\t0m\tfrom the worktree$'
exists .git/ness/state.sqlite3
! exists ../wt/.git/ness
```

`internal/cli/testdata/script/pin_patch.txtar`:
```
# Within the pinned minor, generated files whose lock records an older patch are accepted:
# send and inbox run without ness sync.
# covers: R-CFG-014
cd repo
env NESS_TEST_VERSION=0.3.2
store add-session s-aaaaaaaaaaaa root
store add-session s-bbbbbbbbbbbb child-b s-aaaaaaaaaaaa
env NESS_SESSION=s-bbbbbbbbbbbb
exec ness send --to manager --body hello
stdout '^sent m-'
env NESS_SESSION=s-aaaaaaaaaaaa
exec ness inbox
stdout '\thello$'
cmp .ness/lock.json ../lock.json

-- repo/.ness/ness.toml --
ness = "0.3"
project = "p"
base = "main"
gate = "true"
-- repo/.ness/lock.json --
{
  "entries": [],
  "ness": "0.3.0",
  "profiles": {},
  "schema": 1
}
-- lock.json --
{
  "entries": [],
  "ness": "0.3.0",
  "profiles": {},
  "schema": 1
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run 'TestInboxLineGolden|TestScripts/(inbox|store_worktree|pin_patch)'`
Expected: FAIL — `undefined: inboxLine`; scripts fail with `unknown command "inbox"`.

- [ ] **Step 3: Implement `ness inbox`**

`internal/cli/inbox.go`:
```go
package cli

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// excerptRunes is the inbox excerpt length (R-MSG-030).
const excerptRunes = 80

type inboxOpts struct {
	show, boundary string
	asJSON         bool
}

func newInboxCmd(d Deps) *cobra.Command {
	var o inboxOpts
	cmd := &cobra.Command{
		Use:   "inbox",
		Short: "List your open items, one line each; --show ITEM prints one in full",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			return withStore(d, func(s *store.Store, c caller) error {
				if o.show != "" {
					return showItem(d, s, c, o.show)
				}
				return listInbox(d, s, c, o)
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&o.show, "show", "", "print the full body of `ITEM` and nothing else")
	f.StringVar(&o.boundary, "boundary", "", "record `NOTE`: your state is written down; compact me when idle")
	f.BoolVar(&o.asJSON, "json", false, "print the list as one JSON object")
	cmd.MarkFlagsMutuallyExclusive("show", "boundary")
	cmd.MarkFlagsMutuallyExclusive("show", "json")
	return cmd
}

func listInbox(d Deps, s *store.Store, c caller, o inboxOpts) error {
	now := d.Now()
	if o.boundary != "" {
		if c.operator() {
			return refuse("--boundary needs a session: the operator is not compacted")
		}
		if err := s.SetBoundary(c.id, o.boundary, now); err != nil {
			return err
		}
	}
	items, err := s.Inbox(c.id, now)
	if err != nil {
		return err
	}
	if o.asJSON {
		return printInboxJSON(d, items, now)
	}
	for _, it := range items {
		say(d.Stdout, "%s", inboxLine(it, now))
	}
	return nil
}

// inboxLine is one item: id, from, intent (marked deferred), age and the excerpt, tab-separated
// (R-MSG-030, R-MSG-043, R-MSG-045). Tabs in the body are gone by then (R-MSG-031).
func inboxLine(it store.Item, now time.Time) string {
	intent := it.Intent
	if it.Disposition == store.Deferred {
		intent += " deferred"
	}
	return strings.Join([]string{
		it.ID, displayName(it.Sender), intent, store.FormatAge(now.Sub(it.CreatedAt)), store.Excerpt(it.Body, excerptRunes),
	}, "\t")
}

// showItem prints the item's exact body bytes and marks it shown (R-MSG-033, R-MSG-041).
func showItem(d Deps, s *store.Store, c caller, id string) error {
	it, err := s.Item(id)
	if errors.Is(err, store.ErrNoItem) {
		return refuse("no item %s", id)
	}
	if err != nil {
		return err
	}
	if !c.addressed(it) {
		return refuse("item %s is addressed to %s, not to you", id, displayName(it.Recipient))
	}
	if err := s.MarkShown(id, d.Now()); err != nil {
		return err
	}
	_, err = io.WriteString(d.Stdout, it.Body)
	return err
}

type inboxJSON struct {
	Items  []itemJSON `json:"items"`
	Schema int        `json:"schema"`
}

type itemJSON struct {
	Age       string `json:"age"`
	CreatedAt string `json:"created_at"`
	Deferred  bool   `json:"deferred"`
	Excerpt   string `json:"excerpt"`
	From      string `json:"from"`
	ID        string `json:"id"`
	Intent    string `json:"intent"`
	Urgent    bool   `json:"urgent"`
}

// printInboxJSON lists the same items as the text output with an excerpt, never the body
// (R-MSG-048, R-MSG-032).
func printInboxJSON(d Deps, items []store.Item, now time.Time) error {
	out := inboxJSON{Schema: 1, Items: []itemJSON{}}
	for _, it := range items {
		out.Items = append(out.Items, itemJSON{
			ID: it.ID, From: displayName(it.Sender), Intent: it.Intent, Deferred: it.Disposition == store.Deferred,
			Urgent: it.Urgent, Age: store.FormatAge(now.Sub(it.CreatedAt)), CreatedAt: it.CreatedAt.UTC().Format(time.RFC3339),
			Excerpt: store.Excerpt(it.Body, excerptRunes),
		})
	}
	enc := json.NewEncoder(d.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}
```

Register in `cli.go`: `root.AddCommand(newInitCmd(d), newSyncCmd(d), newHookCmd(d), newSendCmd(d), newInboxCmd(d))`.

- [ ] **Step 4: Run the tests and lint**

Run: `go test -race ./internal/cli/ && make lint`
Expected: PASS. Note on `inbox.txtar`: the fyi excerpt line ends with a space because the file's trailing newline became one (`fyi body with two lines `); that is the documented replacement, not a bug.

- [ ] **Step 5: Commit**

```bash
git add internal/cli
git commit -m "feat(cli): ness inbox lists, shows, pulls and records boundaries"
```

---

### Task 6: `ness reply`

**Files:**
- Create: `internal/cli/reply.go`, `internal/cli/testdata/script/reply.txtar`
- Modify: `internal/cli/cli.go` (register `newReplyCmd`)

**Interfaces:**
- Consumes: `withStore`, `readBody`, `caller.addressed`, `store.Item/Reply`, `store.ErrNoItem/ErrDisposed`, `refuse`, `usage`, `say`.
- Produces: `newReplyCmd(d Deps) *cobra.Command`; output line `replied <id> as <disposition>`.

- [ ] **Step 1: Write the failing script**

`internal/cli/testdata/script/reply.txtar`:
```
# ness reply answers and disposes in one step: the default is resolved, the answer reaches the
# sender as a reply item with no obligation, a disposed item cannot be replied again, deferred
# stays listed and can be answered later, only the addressee (or a root / the operator for
# operator items) may reply, the idempotency key survives disposal, and an unknown item is refused.
# covers: R-MSG-014, R-MSG-020, R-MSG-021, R-MSG-022, R-MSG-023, R-MSG-024, R-MSG-025, R-MSG-043
cd repo
exec ness init --gate true --base main --yes
store add-session s-aaaaaaaaaaaa root
store add-session s-bbbbbbbbbbbb child-b s-aaaaaaaaaaaa
store add-session s-cccccccccccc child-c s-aaaaaaaaaaaa
env NESS_TEST_NOW=2026-10-08T10:00:00Z
env NESS_SESSION=s-bbbbbbbbbbbb
exec ness send --to manager --body 'may I merge?'
store last s-aaaaaaaaaaaa REQ
# Default disposition resolved; answer stored; the sender gets a reply item; the body is not echoed.
env NESS_SESSION=s-aaaaaaaaaaaa
exec ness reply $REQ --body 'yes, merge'
stdout '^replied m-[a-z2-7]{8} as resolved$'
! stdout 'yes, merge'
! stderr .
store item $REQ disposition=resolved 'answer=yes, merge'
store items s-aaaaaaaaaaaa 0
env NESS_SESSION=s-bbbbbbbbbbbb
exec ness inbox
stdout '^m-[a-z2-7]{8}\ts-aaaaaaaaaaaa\treply\t0m\tyes, merge$'
store last s-bbbbbbbbbbbb REP
store item $REP intent=reply sender=s-aaaaaaaaaaaa recipient=s-bbbbbbbbbbbb
# A second reply is refused and the first answer stays.
env NESS_SESSION=s-aaaaaaaaaaaa
exits 4 ness reply $REQ --body 'changed my mind'
stderr 'already resolved'
store item $REQ 'answer=yes, merge'
# The same send after resolution is still the same item: nothing new is listed.
env NESS_SESSION=s-bbbbbbbbbbbb
exec ness send --to manager --body 'may I merge?'
stdout '^already sent m-[a-z2-7]{8}$'
store items s-aaaaaaaaaaaa 0
# --as values: deferred stays listed and marked, then can be declined; accepted; a bad value is a
# usage error; an empty body is a usage error.
exec ness send --to manager --body q2
store last s-aaaaaaaaaaaa Q2
exec ness send --to manager --body q3
store last s-aaaaaaaaaaaa Q3
env NESS_SESSION=s-aaaaaaaaaaaa
exec ness reply $Q2 --body later --as deferred
stdout '^replied m-[a-z2-7]{8} as deferred$'
exec ness inbox
stdout '^m-[a-z2-7]{8}\ts-bbbbbbbbbbbb\trequest deferred\t0m\tq2$'
exec ness reply $Q2 --body no --as declined
store item $Q2 disposition=declined answer=no
exec ness reply $Q3 --body ok --as accepted
store item $Q3 disposition=accepted
store items s-aaaaaaaaaaaa 0
exits 2 ness reply $Q3 --body x --as maybe
stderr 'accepted values are resolved, declined, deferred, accepted'
exits 2 ness reply $Q3 --body ''
# A reply to an fyi takes it off the list (R-MSG-041 path through reply).
env NESS_SESSION=s-bbbbbbbbbbbb
exec ness send --to manager --body heads-up --intent fyi
store last s-aaaaaaaaaaaa FYI
env NESS_SESSION=s-aaaaaaaaaaaa
exec ness reply $FYI --body noted
store items s-aaaaaaaaaaaa 0
# A child may not reply to its sibling's item; nothing changes (Review Focus 5).
exec ness send --to s-cccccccccccc --body 'for c'
store last s-cccccccccccc FORC
env NESS_SESSION=s-bbbbbbbbbbbb
exits 4 ness reply $FORC --body hi
stderr 'addressed to s-cccccccccccc, not to you'
store item $FORC disposition=
# Operator items: the operator (no session) replies; a root records a chat answer; a child cannot.
env NESS_SESSION=s-aaaaaaaaaaaa
exec ness send --to operator --body 'op q1'
store last operator OP1
exec ness send --to operator --body 'op q2'
store last operator OP2
env NESS_SESSION=
exec ness reply $OP1 --body 'operator says yes'
store item $OP1 disposition=resolved
env NESS_SESSION=s-aaaaaaaaaaaa
exec ness inbox
stdout '^m-[a-z2-7]{8}\toperator\treply\t0m\toperator says yes$'
exec ness reply $OP2 --body 'operator said no in chat' --as declined
store item $OP2 disposition=declined 'answer=operator said no in chat'
env NESS_SESSION=s-bbbbbbbbbbbb
exits 4 ness reply $OP2 --body nope
# Unknown item.
exits 4 ness reply m-nope --body x
stderr 'no item m-nope'
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run 'TestScripts/reply'`
Expected: FAIL with `unknown command "reply"`.

- [ ] **Step 3: Implement `ness reply`**

`internal/cli/reply.go`:
```go
package cli

import (
	"errors"
	"slices"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/store"
)

var dispositions = []string{store.Resolved, store.Declined, store.Deferred, store.Accepted}

func newReplyCmd(d Deps) *cobra.Command {
	var body, as string
	cmd := &cobra.Command{
		Use:   "reply ITEM",
		Short: "Answer an item and dispose it in one step",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) error {
			if !slices.Contains(dispositions, as) {
				return usage("--as %s: accepted values are resolved, declined, deferred, accepted", as)
			}
			answer, err := readBody(body)
			if err != nil {
				return err
			}
			return withStore(d, func(s *store.Store, c caller) error {
				return runReply(d, s, c, args[0], answer, as)
			})
		},
	}
	f := cmd.Flags()
	f.StringVar(&body, "body", "", "the answer `TEXT`, or @FILE")
	f.StringVar(&as, "as", store.Resolved, "resolved | declined | deferred | accepted")
	return cmd
}

// runReply checks the item is open and addressed to the caller (R-MSG-023, R-MSG-025) before the
// store disposes it and delivers the answer (R-MSG-020, R-MSG-022).
func runReply(d Deps, s *store.Store, c caller, id, answer, as string) error {
	it, err := s.Item(id)
	if errors.Is(err, store.ErrNoItem) {
		return refuse("no item %s", id)
	}
	if err != nil {
		return err
	}
	if !c.addressed(it) {
		return refuse("item %s is addressed to %s, not to you", id, displayName(it.Recipient))
	}
	if !it.Open() {
		return refuse("item %s is already %s", id, it.Disposition)
	}
	if _, err := s.Reply(id, c.id, answer, as, d.Now()); err != nil {
		if errors.Is(err, store.ErrDisposed) {
			return refuse("item %s was disposed meanwhile", id)
		}
		return err
	}
	say(d.Stdout, "replied %s as %s", id, as)
	return nil
}
```

Register in `cli.go`: `root.AddCommand(newInitCmd(d), newSyncCmd(d), newHookCmd(d), newSendCmd(d), newInboxCmd(d), newReplyCmd(d))`.

- [ ] **Step 4: Run the tests and lint**

Run: `go test -race ./internal/cli/ && make lint`
Expected: PASS. If `dupl` reports `newSendCmd`/`newReplyCmd` as duplicates (threshold 75 tokens), it is a false positive on cobra boilerplate: add `//nolint:dupl // cobra command wiring` above the smaller one.

- [ ] **Step 5: Commit**

```bash
git add internal/cli
git commit -m "feat(cli): ness reply disposes an item and answers its sender"
```

---

### Task 7: Hook payloads and guards (`internal/hook`)

**Files:**
- Create: `internal/hook/payload.go`, `internal/hook/guards.go`, `internal/hook/payload_test.go`, `internal/hook/guards_test.go`

**Interfaces:**
- Produces:
  - `type Payload struct { Source string; ToolName string; ToolInput json.RawMessage }`, `Parse(r io.Reader) (Payload, error)`.
  - `PreTool(p Payload) string` — the refusal reason, `""` to allow.
  - Note for `internal/testkit/repo_test.go`: `internal/hook` must join the `allowed` list of `TestPackageLayout` (R-QA-002 names the design's layout; `hook` is machinery the design puts under `internal/cli`, but the guards are pure functions with their own fuzz target, so they get a package. Add `"internal/hook"` to `allowed` in this task and say so in the commit).

- [ ] **Step 1: Write the failing tests**

`internal/hook/payload_test.go`:
```go
package hook_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/hook"
)

func TestParse(t *testing.T) {
	// covers: R-HOOK-015, R-HOOK-017
	p, err := hook.Parse(strings.NewReader(`{"hook_event_name":"SessionStart","source":"compact","session_id":"x"}`))
	if err != nil || p.Source != "compact" {
		t.Fatalf("Parse = %+v, %v", p, err)
	}
	p, err = hook.Parse(strings.NewReader(`{"tool_name":"Bash","tool_input":{"command":"ls"}}`))
	if err != nil || p.ToolName != "Bash" || string(p.ToolInput) != `{"command":"ls"}` {
		t.Fatalf("Parse = %+v, %v", p, err)
	}
	for _, bad := range []string{"", "{not json", "[1,2]", "null"} {
		if _, err := hook.Parse(strings.NewReader(bad)); err == nil {
			t.Errorf("Parse(%q) accepted", bad)
		}
	}
}

func FuzzParse(f *testing.F) {
	// The parser and the guards never panic on any input (R-HOOK-015, R-QA-022).
	// covers: R-HOOK-015
	f.Add([]byte(`{"source":"compact"}`))
	f.Add([]byte(`{"tool_name":"Bash","tool_input":{"command":"while true; do ness inbox; done"}}`))
	f.Add([]byte(`{"tool_name":"Read","tool_input":{"file_path":"/x/tool-results/a.txt","offset":1}}`))
	f.Add([]byte(`{"tool_name":"Monitor","tool_input":"ness inbox"}`))
	f.Add([]byte("{not json"))
	f.Fuzz(func(_ *testing.T, data []byte) {
		p, err := hook.Parse(bytes.NewReader(data))
		if err != nil {
			return
		}
		_ = hook.PreTool(p)
	})
}
```

`internal/hook/guards_test.go`:
```go
package hook_test

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/hook"
)

func payload(tool string, input any) hook.Payload {
	raw, err := json.Marshal(input)
	if err != nil {
		panic(err)
	}
	return hook.Payload{ToolName: tool, ToolInput: raw}
}

const saved = "/home/u/.claude/projects/-home-u-p/9ed8d0d6/tool-results/b3d68f2ny.txt"

func TestPreToolPollingGuard(t *testing.T) {
	// covers: R-HOOK-011, R-HOOK-012
	blocked := []hook.Payload{
		payload("Monitor", map[string]any{"command": "ness inbox", "timeout_sec": 300}),
		payload("ScheduleWakeup", map[string]any{"prompt": "run ness status and report"}),
		payload("CronCreate", map[string]any{"cron": "*/5 * * * *", "prompt": "ness inbox"}),
		payload("Bash", map[string]any{"command": "while true; do ness inbox; sleep 30; done"}),
		payload("Bash", map[string]any{"command": "until ness status | grep -q done; do sleep 5; done"}),
		payload("Bash", map[string]any{"command": "sleep 60; while ! ness inbox | grep m-; do :; done"}),
	}
	for _, p := range blocked {
		reason := hook.PreTool(p)
		if reason == "" || !strings.Contains(reason, "end your turn") {
			t.Errorf("%s %s: allowed or no waiting rule: %q", p.ToolName, p.ToolInput, reason)
		}
	}
	allowed := []hook.Payload{
		payload("Bash", map[string]any{"command": "ness inbox"}),
		payload("Bash", map[string]any{"command": "ness inbox --show m-abcdefgh"}),
		payload("Bash", map[string]any{"command": "ness status --json"}),
		payload("Bash", map[string]any{"command": "while read l; do echo $l; done < f"}),
		payload("Monitor", map[string]any{"command": "tail -f build.log"}),
		payload("ScheduleWakeup", map[string]any{"prompt": "check the deploy"}),
		payload("CronCreate", map[string]any{"prompt": "rotate logs"}),
		payload("Edit", map[string]any{"file_path": "x.go"}),
	}
	for _, p := range allowed {
		if reason := hook.PreTool(p); reason != "" {
			t.Errorf("%s %s: blocked: %q", p.ToolName, p.ToolInput, reason)
		}
	}
}

func TestPreToolOutputGuard(t *testing.T) {
	// covers: R-HOOK-013
	blocked := []hook.Payload{
		payload("Read", map[string]any{"file_path": saved}),
		payload("Bash", map[string]any{"command": "cat " + saved}),
		payload("Bash", map[string]any{"command": "cat " + saved + " | head"}),
		payload("Bash", map[string]any{"command": "sed 's/a/b/' " + saved}),
	}
	for _, p := range blocked {
		if reason := hook.PreTool(p); reason == "" {
			t.Errorf("%s %s: allowed", p.ToolName, p.ToolInput)
		}
	}
	allowed := []hook.Payload{
		payload("Read", map[string]any{"file_path": saved, "offset": 1, "limit": 200}),
		payload("Read", map[string]any{"file_path": saved, "limit": 50}),
		payload("Read", map[string]any{"file_path": "/home/u/p/main.go"}),
		payload("Bash", map[string]any{"command": "sed -n '1,40p' " + saved}),
		payload("Bash", map[string]any{"command": "grep -n error " + saved}),
		payload("Bash", map[string]any{"command": "head -c 2000 " + saved}),
		payload("Bash", map[string]any{"command": "cat main.go"}),
	}
	for _, p := range allowed {
		if reason := hook.PreTool(p); reason != "" {
			t.Errorf("%s %s: blocked: %q", p.ToolName, p.ToolInput, reason)
		}
	}
	// A tool_input that is not an object is not a guard's business.
	if reason := hook.PreTool(hook.Payload{ToolName: "Read", ToolInput: json.RawMessage(`"x"`)}); reason != "" {
		t.Errorf("non-object tool_input blocked: %q", reason)
	}
}
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/hook/`
Expected: FAIL to build (no package).

- [ ] **Step 3: Implement the package**

`internal/hook/payload.go`:
```go
// Package hook parses client hook payloads and decides the pre-tool guards as pure functions:
// no store, no files, so the CLI can wire them and the fuzz target covers every input
// (R-HOOK-011..015).
package hook

import (
	"encoding/json"
	"fmt"
	"io"
)

// maxPayload bounds what a hook reads from stdin; Claude Code payloads are a few KB.
const maxPayload = 1 << 20

// Payload is the part of a client hook's JSON stdin that ness reads.
type Payload struct {
	Source    string          `json:"source"`     // session-start: startup, resume, clear, compact
	ToolName  string          `json:"tool_name"`  // pre-tool
	ToolInput json.RawMessage `json:"tool_input"` // pre-tool; kept raw, the guards decode what they need
}

// Parse decodes one JSON object from r. It never panics (R-HOOK-015); anything that is not an
// object is an error (R-HOOK-017).
func Parse(r io.Reader) (Payload, error) {
	var p Payload
	data, err := io.ReadAll(io.LimitReader(r, maxPayload))
	if err != nil {
		return Payload{}, fmt.Errorf("hook payload: %w", err)
	}
	if err := json.Unmarshal(data, &p); err != nil {
		return Payload{}, fmt.Errorf("hook payload: %w", err)
	}
	if len(data) == 0 || data[firstNonSpace(data)] != '{' {
		return Payload{}, fmt.Errorf("hook payload: not a JSON object")
	}
	return p, nil
}

func firstNonSpace(b []byte) int {
	for i, c := range b {
		if c != ' ' && c != '\n' && c != '\r' && c != '\t' {
			return i
		}
	}
	return len(b) - 1
}
```

`internal/hook/guards.go`:
```go
package hook

import (
	"encoding/json"
	"regexp"
	"slices"
	"strings"
)

const waitingRule = "pull once with ness inbox, then end your turn; the worker wakes you when something arrives"

var (
	// pollingTools re-run their input on a timer; with a ness read inside they are polling.
	pollingTools = []string{"Monitor", "ScheduleWakeup", "CronCreate"}
	pollRead     = regexp.MustCompile(`\bness (inbox|status)\b`)
	// shellLoop matches a while/until loop keyword.
	// ponytail: a word match, so `echo while` with a ness read in the same command is blocked
	// too; parse the shell if that ever bites.
	shellLoop = regexp.MustCompile(`(^|[\s;&|(])(while|until)\b`)
	// toolResult is the shape of a saved tool-result file (Claude Code: …/<session>/tool-results/<id>.txt).
	// ponytail: path shape only; add the Codex location when known.
	toolResult = regexp.MustCompile(`/tool-results/[^\s"'|;&<>]+`)
	wholeCat   = regexp.MustCompile(`(^|[\s;&|(])cat\s`)
	sedCmd     = regexp.MustCompile(`(^|[\s;&|(])sed\s`)
)

// toolInput are the fields the guards read from any tool's input.
type toolInput struct {
	Command  string `json:"command"`
	FilePath string `json:"file_path"`
	Offset   *int   `json:"offset"`
	Limit    *int   `json:"limit"`
}

// PreTool decides a pre-tool call for a bound session: the polling guard (R-HOOK-011,
// R-HOOK-012) and the output guard (R-HOOK-013). It returns the refusal reason, or "" to allow.
func PreTool(p Payload) string {
	var in toolInput
	// A tool_input that is not an object has no command or path to guard.
	_ = json.Unmarshal(p.ToolInput, &in)
	if reason := polling(p.ToolName, in.Command, string(p.ToolInput)); reason != "" {
		return reason
	}
	return outputGuard(p.ToolName, in)
}

func polling(tool, command, raw string) string {
	switch {
	case slices.Contains(pollingTools, tool) && pollRead.MatchString(raw):
		return "polling with " + tool + " on ness inbox/status is refused: " + waitingRule
	case tool == "Bash" && shellLoop.MatchString(command) && pollRead.MatchString(command):
		return "a shell loop over ness inbox/status is refused: " + waitingRule
	}
	return ""
}

func outputGuard(tool string, in toolInput) string {
	switch tool {
	case "Read":
		if toolResult.MatchString(in.FilePath) && in.Offset == nil && in.Limit == nil {
			return "reading a whole saved tool result is refused: pass offset and limit, or grep it"
		}
	case "Bash":
		if toolResult.MatchString(in.Command) && (wholeCat.MatchString(in.Command) ||
			sedCmd.MatchString(in.Command) && !strings.Contains(in.Command, " -n")) {
			return "cat or sed over a whole saved tool result is refused: use head, grep or sed -n with a range"
		}
	}
	return ""
}
```

Add `"internal/hook"` to `allowed` in `internal/testkit/repo_test.go` (`TestPackageLayout`).

- [ ] **Step 4: Run the tests, a short fuzz, and lint**

Run: `go test -race ./internal/hook/ ./internal/testkit/ && go test -run '^$' -fuzz FuzzParse -fuzztime 10s ./internal/hook/ && make lint`
Expected: PASS. If `errcheck` (check-blank) flags `_ = json.Unmarshal(...)`, replace it with `if err := json.Unmarshal(p.ToolInput, &in); err != nil { in = toolInput{} }`. If `gocritic` flags the mixed `||`/`&&` in `outputGuard`, add parentheses around the `sedCmd … && !strings.Contains …` pair.

- [ ] **Step 5: Commit**

```bash
git add internal/hook internal/testkit/repo_test.go
git commit -m "feat(hook): payload parser with fuzz target, polling guard and output guard"
```

---

### Task 8: `ness hook` for a bound session: pre-tool guards, ready mark, cheat sheet

**Files:**
- Create: `internal/cli/testdata/script/hook_pretool.txtar`, `internal/cli/testdata/script/hook_session_start.txtar`
- Modify: `internal/cli/hook.go`

**Interfaces:**
- Consumes: `hook.Parse`, `hook.PreTool`, `loadProject`, `openStore`, `loadProfiles` (P1, `sync.go`), `store.MarkReady`, `profile.Part`, `Deps.Now`.
- Produces: `bound{p *project; s *store.Store; c caller}`, `bind(d Deps) (bound, bool)`, `preTool(d Deps) error`, `sessionStart(d Deps, b bound) error`, `block(reason string) error` (exit 2), `cheatSheet` const. Identity line: `ness session <id> scope <scope> manager <id|operator>` (P3 appends ` phase <phase>`).

- [ ] **Step 1: Write the failing scripts**

`internal/cli/testdata/script/hook_pretool.txtar`:
```
# For a bound session, pre-tool blocks polling and whole-file reads of saved tool results with
# exit 2 and the reason on stderr, allows everything else silently, and refuses a payload it
# cannot parse. Unbound, the same payloads pass silently.
# covers: R-HOOK-011, R-HOOK-012, R-HOOK-013, R-HOOK-014, R-HOOK-017, R-HOOK-002
cd repo
exec ness init --gate true --base main --yes
store add-session s-aaaaaaaaaaaa root
env NESS_SESSION=s-aaaaaaaaaaaa
stdin monitor.json
exits 2 ness hook pre-tool
stderr 'end your turn'
! stdout .
stdin schedule.json
exits 2 ness hook pre-tool
stderr 'end your turn'
stdin cron.json
exits 2 ness hook pre-tool
stderr 'end your turn'
stdin loop_while.json
exits 2 ness hook pre-tool
stderr 'end your turn'
stdin loop_until.json
exits 2 ness hook pre-tool
stderr 'end your turn'
stdin monitor_ok.json
exec ness hook pre-tool
! stdout .
! stderr .
stdin inbox_once.json
exec ness hook pre-tool
! stdout .
! stderr .
stdin read_whole.json
exits 2 ness hook pre-tool
stderr 'offset and limit'
stdin read_range.json
exec ness hook pre-tool
! stderr .
stdin cat.json
exits 2 ness hook pre-tool
stderr 'saved tool result'
stdin sed_range.json
exec ness hook pre-tool
! stderr .
stdin edit.json
exec ness hook pre-tool
! stdout .
! stderr .
stdin garbage.txt
exits 2 ness hook pre-tool
stderr 'hook payload'
# Unbound: silent.
env NESS_SESSION=
stdin monitor.json
exec ness hook pre-tool
! stdout .
! stderr .
stdin garbage.txt
exec ness hook pre-tool
! stdout .
! stderr .

-- monitor.json --
{"tool_name":"Monitor","tool_input":{"command":"ness inbox","timeout_sec":300}}
-- schedule.json --
{"tool_name":"ScheduleWakeup","tool_input":{"prompt":"run ness status and tell me"}}
-- cron.json --
{"tool_name":"CronCreate","tool_input":{"cron":"*/5 * * * *","prompt":"ness inbox"}}
-- loop_while.json --
{"tool_name":"Bash","tool_input":{"command":"while true; do ness inbox; sleep 30; done"}}
-- loop_until.json --
{"tool_name":"Bash","tool_input":{"command":"until ness status | grep -q closed; do sleep 5; done"}}
-- monitor_ok.json --
{"tool_name":"Monitor","tool_input":{"command":"tail -f build.log","timeout_sec":300}}
-- inbox_once.json --
{"tool_name":"Bash","tool_input":{"command":"ness inbox"}}
-- read_whole.json --
{"tool_name":"Read","tool_input":{"file_path":"/home/u/.claude/projects/-home-u-p/9ed8d0d6/tool-results/b3d68f2ny.txt"}}
-- read_range.json --
{"tool_name":"Read","tool_input":{"file_path":"/home/u/.claude/projects/-home-u-p/9ed8d0d6/tool-results/b3d68f2ny.txt","offset":1,"limit":100}}
-- cat.json --
{"tool_name":"Bash","tool_input":{"command":"cat /home/u/.claude/projects/-home-u-p/9ed8d0d6/tool-results/b3d68f2ny.txt"}}
-- sed_range.json --
{"tool_name":"Bash","tool_input":{"command":"sed -n '1,40p' /home/u/.claude/projects/-home-u-p/9ed8d0d6/tool-results/b3d68f2ny.txt"}}
-- edit.json --
{"tool_name":"Edit","tool_input":{"file_path":"main.go","old_string":"a","new_string":"b"}}
-- garbage.txt --
{not json
```

`internal/cli/testdata/script/hook_session_start.txtar`:
```
# For a bound session, session-start writes the ready mark once, prints the identity, and after
# a compaction prints the cheat sheet (under 20 lines) and the post-compact files a profile
# listed. P3 adds the phase to the identity line and the phase-unchanged check.
# covers: R-HOOK-003, R-HOOK-004, R-HOOK-006, R-HOOK-007, R-HOOK-009
cd repo
exec ness init --gate true --base main --yes
cp ../ness.toml .ness/ness.toml
store add-session s-aaaaaaaaaaaa root
store add-session s-bbbbbbbbbbbb child-b s-aaaaaaaaaaaa
env NESS_SESSION=s-bbbbbbbbbbbb
env NESS_TEST_NOW=2026-10-08T10:00:00Z
store session s-bbbbbbbbbbbb ready=false
stdin startup.json
exec ness hook session-start
stdout '^ness session s-bbbbbbbbbbbb scope child-b manager s-aaaaaaaaaaaa$'
! stdout 'cheat sheet'
! stderr .
store session s-bbbbbbbbbbbb ready=2026-10-08T10:00:00Z
# A second startup keeps the first mark.
env NESS_TEST_NOW=2026-10-08T10:05:00Z
stdin startup.json
exec ness hook session-start
store session s-bbbbbbbbbbbb ready=2026-10-08T10:00:00Z
# After compaction: identity, the cheat sheet, the post-compact list; at most 20 lines.
stdin compact.json
exec ness hook session-start
cmp stdout ../compact.txt
! stdout '(?s)(.*\n){21}'
# A root's manager is the operator.
env NESS_SESSION=s-aaaaaaaaaaaa
stdin startup.json
exec ness hook session-start
stdout '^ness session s-aaaaaaaaaaaa scope root manager operator$'

-- ness.toml --
ness = "0.1"
project = "p"
base = "main"
gate = "true"
profiles = ["local"]
-- repo/.ness/profiles/local/profile.toml --
name = "local"
[slots]
"ness-entry#post-compact" = { append = "reload.md" }
-- repo/.ness/profiles/local/reload.md --
- docs/sessions/root/plan.md
- docs/sessions/root/decisions.md
-- startup.json --
{"hook_event_name":"SessionStart","source":"startup","session_id":"c0ffee"}
-- compact.json --
{"hook_event_name":"SessionStart","source":"compact","session_id":"c0ffee"}
-- compact.txt --
ness session s-bbbbbbbbbbbb scope child-b manager s-aaaaaaaaaaaa
ness cheat sheet (after compaction; re-read your brief and your inbox first)
  ness inbox                                       list your open items, one line each
  ness inbox --show ITEM                           read one item in full
  ness inbox --boundary NOTE                       state written down; compact me when idle
  ness send --to manager|ID --body TEXT|@FILE      [--intent request|fyi|decision|handoff] [--urgent]
  ness reply ITEM --body TEXT|@FILE                [--as resolved|declined|deferred|accepted]
  ness open --scope NAME --tier TIER --brief FILE  launch a child
  ness integrate                                   fold this branch through the gate into the manager's
  ness close ID [--note TEXT]                      close a child you manage
  ness probe start|run|verdict|stop                probes, when the feature is on
Waiting rule: pull once, then end your turn; the worker wakes you when something arrives.
- docs/sessions/root/plan.md
- docs/sessions/root/decisions.md
```

- [ ] **Step 2: Run to verify failure**

Run: `go test ./internal/cli/ -run 'TestScripts/hook_'`
Expected: `hook_pretool` FAILS at the first `exits 2` (the hook exits 0); `hook_session_start` FAILS at the first `stdout` (nothing printed). `hook_unbound` still PASSES.

- [ ] **Step 3: Implement the bound hook**

Replace `internal/cli/hook.go`:
```go
package cli

import (
	"errors"
	"slices"
	"strings"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/hook"
	"github.com/enea-scaccabarozzi/ness/internal/profile"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

// hookEvents are the client events ness handles.
var hookEvents = []string{"session-start", "pre-tool"}

// cheatSheet is printed after a compaction (R-HOOK-006): one line per hot-path command, under
// 20 lines, so a compacted session does not re-read --help.
const cheatSheet = `ness cheat sheet (after compaction; re-read your brief and your inbox first)
  ness inbox                                       list your open items, one line each
  ness inbox --show ITEM                           read one item in full
  ness inbox --boundary NOTE                       state written down; compact me when idle
  ness send --to manager|ID --body TEXT|@FILE      [--intent request|fyi|decision|handoff] [--urgent]
  ness reply ITEM --body TEXT|@FILE                [--as resolved|declined|deferred|accepted]
  ness open --scope NAME --tier TIER --brief FILE  launch a child
  ness integrate                                   fold this branch through the gate into the manager's
  ness close ID [--note TEXT]                      close a child you manage
  ness probe start|run|verdict|stop                probes, when the feature is on
Waiting rule: pull once, then end your turn; the worker wakes you when something arrives.`

// newHookCmd is the entry point of the client hooks (R-HOOK-002, R-HOOK-016, R-CFG-053). Outside
// a ness project, without NESS_SESSION, or for a session the store does not know, every event is
// a silent exit 0: a contributor's client must never see ness fail.
func newHookCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "hook EVENT",
		Short: "Entry point for client hooks: session-start, pre-tool",
		Args:  cobra.ExactArgs(1),
		RunE: func(_ *cobra.Command, args []string) (err error) {
			if !slices.Contains(hookEvents, args[0]) {
				return usage("unknown hook event %q; events are session-start and pre-tool", args[0])
			}
			b, ok := bind(d)
			if !ok {
				return nil
			}
			defer func() { err = errors.Join(err, b.s.Close()) }()
			if args[0] == "pre-tool" {
				return preTool(d)
			}
			return sessionStart(d, b)
		},
	}
}

// bound is a hook's context: the project, the store and the caller's session.
type bound struct {
	p *project
	s *store.Store
	c caller
}

// bind resolves the session a hook runs for. Any failure — no NESS_SESSION, no ness project, a
// pin mismatch, a session the store does not know — means the client is not bound here and the
// hook stays silent (R-HOOK-002, R-CFG-053). The next ness command the session runs reports the
// cause loudly.
func bind(d Deps) (bound, bool) {
	if d.Session == store.Operator {
		return bound{}, false
	}
	p, err := loadProject(d)
	if err != nil {
		return bound{}, false
	}
	s, c, err := openStore(d, p)
	if err != nil {
		return bound{}, false
	}
	return bound{p: p, s: s, c: c}, true
}

// block is the client's blocking response: exit 2 with the reason on stderr (R-HOOK-014).
func block(reason string) error { return &exitError{code: ExitUsage, err: errors.New(reason)} }

// preTool runs the guards for a bound session; a payload that does not parse is refused
// (R-HOOK-017). P5 adds the contract guard here (R-HOOK-010).
func preTool(d Deps) error {
	p, err := hook.Parse(d.Stdin)
	if err != nil {
		return block(err.Error())
	}
	if reason := hook.PreTool(p); reason != "" {
		return block(reason)
	}
	return nil
}

// sessionStart writes the ready mark (R-HOOK-003, first mark wins R-HOOK-009), prints the
// identity (R-HOOK-004) and, after a compaction, the cheat sheet and the post-compact files
// (R-HOOK-006, R-HOOK-007). P3 appends the phase to the identity line.
func sessionStart(d Deps, b bound) error {
	p, err := hook.Parse(d.Stdin)
	if err != nil {
		p = hook.Payload{} // an unreadable payload is a plain startup: identity still loads
	}
	if err := b.s.MarkReady(b.c.id, d.Now()); err != nil {
		return err
	}
	say(d.Stdout, "ness session %s scope %s manager %s", b.c.id, b.c.sess.Scope, displayName(b.c.sess.Parent))
	if p.Source != "compact" {
		return nil
	}
	say(d.Stdout, "%s", cheatSheet)
	for _, part := range postCompact(d, b.p) {
		say(d.Stdout, "%s", strings.TrimSuffix(string(part.Text), "\n"))
	}
	return nil
}

// postCompact returns the ness-entry#post-compact list slot (R-HOOK-007). A profile error is
// reported, not fatal: sync owns profile validation.
func postCompact(d Deps, p *project) []profile.Part {
	cfg, err := p.file.Config(nil)
	if err != nil {
		say(d.Stderr, "ness: %v", err)
		return nil
	}
	res, err := loadProfiles(d, p, cfg.Profiles)
	if err != nil {
		say(d.Stderr, "ness: %v", err)
		return nil
	}
	return res.Lists["ness-entry#post-compact"]
}
```

- [ ] **Step 4: Run every script, race and lint**

Run: `go test -race ./internal/cli/ && make lint`
Expected: PASS, including `hook_unbound.txtar` (the unknown-session case now goes through `bind` and must stay silent). If `nilerr` flags `bind`, it is wrong about a bool return; if it does flag it, add `//nolint:nilerr // fail open: the hook is silent when unbound` to the two `return bound{}, false` lines.

- [ ] **Step 5: Commit**

```bash
git add internal/cli
git commit -m "feat(cli): ness hook runs the pre-tool guards, writes the ready mark and prints the cheat sheet"
```

---

### Task 9: Coverage gate, status, wrap-up

**Files:**
- Modify: `internal/testkit/coverage_repo_test.go`, `README.md`

- [ ] **Step 1: MSG and HOOK join the gate**

Replace the two variables in `internal/testkit/coverage_repo_test.go`:
```go
// doneAreas are the spec areas whose plans have landed (P1, P2).
var doneAreas = []string{"CFG", "SYNC", "PROF", "QA", "MSG", "HOOK"}

// deferred names ids of done areas that a later plan, a review entry or publication delivers.
var deferred = map[string]string{
	"R-CFG-041": "P3", "R-SYNC-068": "P3", "R-SYNC-086": "P3",
	"R-QA-024": "P3", "R-QA-027": "P3", "R-QA-042": "P3",
	"R-HOOK-005": "P3", "R-HOOK-008": "P3", "R-HOOK-018": "P3",
	"R-MSG-005": "P4", "R-MSG-042": "P4",
	"R-CFG-030": "P5", "R-CFG-031": "P5", "R-CFG-054": "P5", "R-PROF-015": "P5", "R-HOOK-010": "P5",
	"R-CFG-032": "P6", "R-CFG-034": "P6", "R-CFG-035": "P6", "R-CFG-037": "P6", "R-CFG-051": "P6",
	"R-SYNC-032": "P6", "R-SYNC-033": "P6", "R-SYNC-089": "P6", "R-PROF-002": "P6",
	"R-QA-004": "P6", "R-QA-005": "P6", "R-QA-023": "P6", "R-QA-030": "P6", "R-QA-031": "P6", "R-QA-041": "P6",
	"R-QA-032": "publication", "R-QA-033": "publication",
	"R-QA-025": "withdrawn (D39)",
	// Review entry, written to docs/spec/reviews.md by the end-of-plan reviewer.
	"R-MSG-004": "review",
}
```
R-CFG-014 and R-CFG-053 leave the map: Tasks 1, 4 and 5 cover them.

Run: `go test ./internal/testkit/ -run TestRequirementCoverage`
Expected: PASS. If it names an uncovered MSG or HOOK id, it is a real gap: find the task whose code delivers it, add the test with its covers marker, and rerun. Defer an id only when its behaviour needs a later plan's command, and write that plan's name.

- [ ] **Step 2: README status**

In `README.md`, replace the status line with:
`Status: P2 (store, messages, hooks) implemented: ness send, reply, inbox and hook on top of P1's init and sync. Plans: docs/superpowers/specs/2026-10-08-ness-v1-scope.md.`

- [ ] **Step 3: The full gate and the dogfood check**

Run: `make validate && make fuzz FUZZTIME=10s`
Expected: PASS end to end: `CGO_ENABLED=0 go build`, both lint configs, `-race`, `-count=4`, govulncheck, the coverage gate, `ness sync --check` on this repository (no generated file changed in P2), and the three fuzz targets (`FuzzSyncNeverClobbers`, `FuzzExcerpt`, `FuzzParse`) plus P1's config/profile fuzzers.

Then, in this repository (which dogfoods ness and whose `.claude/settings.json` already calls both hooks):
```bash
echo '{"tool_name":"Bash","tool_input":{"command":"ness inbox"}}' | go run ./cmd/ness hook pre-tool; echo "exit $?"
go run ./cmd/ness inbox; echo "exit $?"
```
Expected: the hook prints nothing and exits 0 (no `NESS_SESSION` in this shell); `inbox` prints nothing and exits 0 (the operator's inbox is empty), and `.git/ness/state.sqlite3` now exists. `git status` must not show it (`.git/` is never tracked).

- [ ] **Step 4: Commit and push, check CI**

```bash
git add -A
git commit -m "build: MSG and HOOK join the requirement coverage gate"
git push
gh run watch --exit-status
```
Expected: the CI run is green. If it fails only in CI, fix the cause (often the environment: mise, PATH, git identity, `/dev/zero` or `sh` in `inbox.txtar`) and push again.

---

## End-of-plan review

After Task 9, launch one fresh reviewer on the most capable model with: `docs/spec/store-messages.md`, `docs/spec/hooks.md`, this plan, and `git diff <first P2 commit>^..HEAD`. The reviewer checks:
1. Each covered requirement's test would fail without its behaviour (spot-check by breaking the code: drop `COALESCE` in `MarkReady`, drop the `key` lookup in `Send`, drop `_txlock=immediate`).
2. The Review Focus items hold.
3. R-MSG-004: no code path opens a file other than `<git-common-dir>/ness/state.sqlite3` (grep `sql.Open`, `os.Open`, `ReadFile` in `internal/store` and `internal/cli`).
4. The rulings this plan made where the spec was silent or inconsistent (recorded in the plan summary): a child cannot address the operator (R-MSG-017 over R-MSG-024's example); any open root may answer an operator item; a deferred item may be replied again; `sent … to` prints the resolved recipient; the inbox line is tab-separated with `<intent> deferred`; `--show` sets `shown_at` rather than disposing; item ids are 40 random bits without a collision retry.

For R-MSG-004, the reviewer appends a line to `docs/spec/reviews.md` (`- R-MSG-004 | <reviewer session> | <date> | <commit>`) and removes the id from `deferred`. Then fix each finding, rerun `make validate`, and report to the operator: what shipped, the evidence (test counts, fuzz time, CI run link), and what is deferred.
