> [!NOTE]
> Archive: altered for the public release. Names of private projects, local paths and private links were replaced; the content is otherwise as written.

# Field-feedback Batch B Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Seven ness fixes from the research lab field analysis (B1-B7): refuse dead background waits, an honest `ness status` idle, a stall backstop for unfinished children, merge-mode `--subject`, operator alerts, a fuller post-compact reload, and a gate-worktree setup command.

**Architecture:** Small independent fixes first (B1, B2, B4, B6, B7, B3), each as a test task then an implementation task, each ending in its own commit; then B5 in four test/implementation pairs. The design is settled (`docs/sessions/field-feedback/plan.md`, `specs.md` §1); this plan does not reopen it.

**Tech Stack:** Go (cobra CLI), SQLite store (`modernc.org/sqlite`), testscript (`internal/cli/testdata/script/*.txtar`), unit tests with hand-written fakes (`internal/worker` bench, stepped clock, `package x_test`), profile text in `profiles/`.

**Spec:** `docs/sessions/field-feedback/specs.md` §1 (B5), `docs/sessions/field-feedback/plan.md` (B1-B7), evidence in `docs/discovery/findings/field-research/{clarifications,liveness-census}.md`. Normative requirements amended: `docs/spec/{hooks,sessions,worker,store-messages,core-config,profiles-builtin}.md`.


## Root rulings (override any task text that conflicts)

- R1. With no operator channel configured, the worker stamps `alerted_at` on operator items anyway (nothing to deliver is delivered): enabling a channel later must not replay a backlog. Tests: no channel → item stamped, no post; channel enabled later → no old item posted.
- R2. B3's stall backstop and its Trouble alert skip a child whose screen shows a live background shell (`Screen.Background`), the same signal Your move uses: a child running a long `run_in_background` job is working, not stalled. Tests: idle child with `Background` set → no stall report, no alert.

## Global Constraints

- Work only in the worktree `~/src/ness/.worktrees/field-feedback-c5a82a215386` (branch `session/field-feedback-c5a82a215386`), never in `~/src/ness`.
- Test rules (`.claude/skills/ness-test-strategy`, `ness-test-discipline`, `ness-interface-design`): the implementer never writes the judge, so each item has a test task before its implementation task and the implementer does not edit those tests; black-box (`package x_test`); hand-written fakes, no generated mocks; no `time.Sleep`; no permissive mocks or call-count assertions; helpers call `t.Helper()`; fixtures in `testdata/`; test files under 400 lines (`.ness/audit.toml`); every test carries a `// covers: R-…` marker; no `ForTest` identifiers; no package-level mutable state; interfaces declared by the consumer, at most three methods; every package keeps its one-sentence package comment; every export has godoc.
- A test task writes the spec text of the R-ids it covers (the contract first), then the failing tests. It may add only signature-only stubs that return zero values so the tests compile and fail on assertions. Its commit is red by design; the implementation task's commit makes it green.
- Any task that touches `profiles/**` text ends with `go run ./cmd/ness sync` and commits the re-rendered `.agents/skills/**` and `.ness/lock.json`; the gate's `selfcheck` is `ness sync --check`.
- Per-task checks: `go test ./<pkg>/... -count=1`; before each implementation commit also `golangci-lint run --allow-serial-runners ./... && golangci-lint run --allow-serial-runners -c .golangci.tests.yml ./...` and `python3 .ness/audit.py`. The full gate (`make validate`) runs once, in `ness integrate`; never run it next to other sessions.
- Commits: Conventional Commits; each ends with the trailer lines
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>` and `Claude-Session: <session link removed>`.
  Never pipe a gate or a commit through `tail`/`head`: check the exit status (`git commit …; echo $?`).
- Push (outside this plan) uses `mise exec -- git push` with SSH keepalives.

## Step 0 result: can ness see a live background task?

Measured on 2026-10-15 against herdr 0.9.3 and the live research-lab panes.

- herdr's Claude manifest (`~/.local/state/herdr/agent-detection/remote/claude.toml`, version 2026.10.11.1) reports `working` for the OSC title spinner, a live turn, `Waiting for N background agents to finish` and `N MCP tasks still running`. It has no rule for a background shell command (`run_in_background` Bash or a detached `&`).
- Evidence: the research lab pane `wV:pF` (l0b-band) at a free prompt with the footer `⏵⏵ bypass permissions on · 1 shell · ← for agents`: `herdr agent explain` says `state: idle`, rule `live_prompt_box`. So a background shell does not move herdr's status.
- The Claude Code pane screen does show it: the last line carries `· N shell(s)`. `internal/atom/claude/screen.go` does not read it today, and the worker already reads the screen of every idle pane (`observe`).

Decision (smallest correct one): add `Background bool` to `session.Screen`, set by `claude.Read` when the footer line matches `·\s*\d+\s+shells?\b`; the worker remembers it per pane. For Your move, a child counts as working when herdr says `working` or `blocked`, or its last screen read shows `Background`. This covers subagents and MCP tasks (herdr) and background shells (screen). Anything else a client shows stays a false positive, and the alert keeps the wording "nothing appears to be running". A descendant the worker has not seen yet this run counts as working (conservative, no alert).

## Forced choices (one line each)

- B2: "real work" is a transcript entry of type `user` or `assistant` with a parseable `timestamp`, not `isMeta`, not `isCompactSummary`; `attachment`, `system` (turn_duration, away_summary, local_command), `queue-operation`, `api-request*` and timestamp-less metadata lines (last-prompt, custom-title, agent-name, ai-title, mode…) are not work. With no such entry the file mtime stays the fallback. `status.go:219` needs no change: it already prints `Transcript.LastActive`.
- B3: "not handed off" is "no item with intent `handoff` sent by the session, in any disposition" (`Store.HandedOff`); close disposes a child's items as accepted, but a closed child is not visited.
- B1: any unquoted lone `&` is refused, not only `( … ) &` and a trailing one, because a mid-line `cmd & next` has the same dead end; a Bash call with `run_in_background: true` is exempt. `$(( a & b ))` is a known false positive (ponytail ceiling noted in code). `ps | grep` self-matching waits are not covered (no census evidence).
- B4: `integrate` warns on stderr and drops `--subject` for any mode other than `squash`.
- B5: `checkpoint` needs a store `ask` column (SQL cannot call the Go heading rule): migration 7 rebuilds `items` (the `intent` CHECK changes) adding `ask` and `alerted_at`, and stamps `alerted_at = created_at` on existing rows so an upgrade replays no alert. `Alert` also carries `Root` (the thread key). `Page:` stays in the body (no `page` column). Descendants, not only children, count for Your move. With no channel configured, items stay unalerted.
- B7: key `[integrate] setup`, a shell command run in the throwaway worktree, inside the gate lock, right before the gate; failure is a gate failure (`ErrGate`). It gets no extra environment: `git rev-parse --git-common-dir` locates the primary checkout.

## Review Focus

Failure modes the items imply; each is pinned by a test in the named task.

1. A `&` that is not backgrounding (`&&`, `&>`, `2>&1`, `>&2`, quoted, in a here-doc body or a comment) must pass (Task 1).
2. A transcript with no timestamps, non-JSON lines or only metadata entries must still give an idle number (mtime fallback) (Task 3).
3. A child idle after a handoff must not stall; a launching or root session must not get the B3 report (Task 11).
4. A worker restart or an enabled-later channel must not replay old alerts; a failed post retries without duplicating (Tasks 13, 15).
5. The post-compact output must stay within 20 lines after core adds its reload lines (Task 7).

## File map

| Item | Create | Modify |
|---|---|---|
| B1 | `internal/hook/background.go`, `internal/hook/background_test.go`, `internal/cli/testdata/script/hook_pretool_background.txtar` | `internal/hook/guards.go`, `docs/spec/hooks.md` |
| B2 | `internal/atom/claude/transcript_test.go`, `internal/atom/claude/testdata/transcript-idle.jsonl`, `internal/cli/testdata/script/status_idle.txtar` | `internal/atom/claude/claude.go`, `internal/session/client.go` (comment), `docs/spec/sessions.md` |
| B4 | | `profiles/core/skills/ness-launch/SKILL.md`, `internal/cli/integrate.go`, `internal/cli/testdata/script/integrate_squash.txtar`, `internal/cli/testdata/script/core_text.txtar`, `docs/spec/sessions.md`, `docs/spec/profiles-builtin.md` |
| B6 | `profiles/core/post-compact.md` | `profiles/core/profile.toml`, `internal/cli/testdata/script/hook_session_start.txtar`, `docs/spec/profiles-builtin.md`, `docs/spec/hooks.md` |
| B7 | `internal/session/integrate_setup_test.go`, `internal/cli/testdata/script/integrate_setup.txtar` | `internal/config/config.go`, `internal/session/integrate.go`, `internal/cli/integrate.go`, `docs/spec/core-config.md`, `docs/spec/sessions.md` |
| B3 | `internal/worker/stalls_brief_test.go` | `internal/worker/stalls.go`, `internal/store/items.go`, `internal/worker/stalls_test.go` (one premise), `docs/spec/worker.md` |
| B5 | `internal/store/checkpoint.go`, `internal/store/alerts_test.go`, `internal/worker/alert.go`, `internal/worker/alerts_test.go`, `internal/cli/alert_render_test.go`, `internal/cli/testdata/script/send_checkpoint.txtar`, golden files | `internal/store/{items,migrate}.go`, `internal/cli/{send,worker,hook}.go`, `internal/worker/{worker,operator,stalls}.go`, `internal/session/screen.go`, `internal/atom/claude/screen.go`, profile skill text, `docs/spec/{store-messages,worker,profiles-builtin}.md` |

---

## B1: refuse shell backgrounding and self-matching waits

### Task 1: B1 acceptance tests and spec

**Files:**
- Modify: `docs/spec/hooks.md` (after R-HOOK-013)
- Create: `internal/hook/background_test.go`, `internal/cli/testdata/script/hook_pretool_background.txtar`

**Interfaces:**
- Consumes: `hook.PreTool(p hook.Payload) string` (guards.go), the `payload(tool, input)` helper in `guards_test.go`.
- Produces: the contract of Task 2: new refusals from `PreTool`, reasons below.

- [ ] **Step 1: Spec.** Add to `docs/spec/hooks.md`:

```
- **R-HOOK-020** For a bound session, `pre-tool` MUST refuse a Bash command that backgrounds work
  with an unquoted lone `&` (`( … ) &`, a trailing `&`, `cmd & next`), unless the call sets
  `run_in_background: true`. `&&`, `&>`, `>&`, `<&`, `|&`, `2>&1`, an `&` inside quotes, a here-doc
  body or a comment MUST pass. The message says nothing wakes the session when the job ends and
  points to `run_in_background`. (Field run the research lab: a commit backgrounded with `( … ) &`
  left a session dead 4.8 h.)
  Verify: unit — blocked and allowed tables; testscript — exit 2 with the message on stderr.
- **R-HOOK-021** For a bound session, `pre-tool` MUST refuse a Bash `while`/`until` loop whose
  command runs `pgrep` with `-f` (any flag cluster containing `f`): the pattern matches the
  loop's own command line, so the wait never ends. The message points to `run_in_background`.
  (Field run the research lab: 8.7 h dead.)
  Verify: unit and testscript, as R-HOOK-020.
```

- [ ] **Step 2: Unit tests** in `internal/hook/background_test.go` (`package hook_test`, `// covers: R-HOOK-020, R-HOOK-021`), two tables with the `payload` helper. Blocked (reason must contain `run_in_background`): `( git commit -F m > c.log 2>&1; echo done >> c.log ) &`, `sleep 5 &`, `nohup ./run.sh > o.log 2>&1 &`, `make a & make b`, `echo hi &\n`, `bash -c 'sleep 1 &'` (the `&` is inside quotes: this one is ALLOWED, put it in the allowed table), `until ! pgrep -f 'pilot.py 500'; do sleep 10; done`, `while pgrep -af land.sh; do sleep 5; done`. Allowed (reason ""): `a && b`, `a&&b`, `cmd &> out.log`, `cmd >& out.log`, `cmd 2>&1 | tee x`, `cmd |& tee x`, `echo "a & b"`, `echo 'x &'`, `bash -c 'sleep 1 &'`, `echo a \& b`, `# cmd &` (comment), a here-doc `cat <<'EOF'\nfoo & bar\nEOF\nls`, an indented `<<-EOF` here-doc with a tab-indented terminator, `grep -f pat file` (no loop), `pgrep -x foo` (no `-f`), `until ! pgrep -x foo; do sleep 1; done`. One more allowed case: `sleep 5 &` with `{"command": "...", "run_in_background": true}`.

- [ ] **Step 3: Testscript** `hook_pretool_background.txtar`, style of `hook_pretool.txtar`: `exec ness init`, `store add-session s-aaaaaaaaaaaa root`, `env NESS_SESSION=s-aaaaaaaaaaaa`, then `stdin ../bg.json` / `exits 2 ness hook pre-tool` / `stderr 'run_in_background'` for one `&` case and one `until ! pgrep -f` case; `stdin ../and.json` (`a && b`) → `exec`, `! stderr .`; unbound (`env NESS_SESSION=`) → silent exit 0.

- [ ] **Step 4: Run, expect red.** `go test ./internal/hook/ ./internal/cli -run 'Background|TestScripts/hook_pretool_background' -count=1` → FAIL (allowed today).

- [ ] **Step 5: Commit** `test(hook): acceptance tests for the background and pgrep guards` (red by design).

### Task 2: B1 implementation

**Files:**
- Create: `internal/hook/background.go`
- Modify: `internal/hook/guards.go` (`toolInput`, `PreTool`, `polling`)

**Interfaces:**
- Consumes: the tests of Task 1.
- Produces: `PreTool` returns the two new reasons; `func backgrounded(cmd string) bool` (unexported).

- [ ] **Step 1: `toolInput`** gains `RunInBackground bool \`json:"run_in_background"\``. In `PreTool`, after `polling`, add `if reason := background(p.ToolName, in); reason != "" { return reason }` before `outputGuard`.

- [ ] **Step 2: `background.go`.**

```go
package hook

import "regexp"

const bgRule = "run it with the Bash tool's run_in_background option and end your turn; the task notification wakes you"

// pgrepSelf matches a pgrep whose -f flag searches full command lines, which include the
// waiting loop's own bash -c text.
var pgrepSelf = regexp.MustCompile(`\bpgrep\s+(-\w*\s+)*-\w*f`)

func background(tool string, in toolInput) string {
	if tool != "Bash" {
		return ""
	}
	if shellLoop.MatchString(in.Command) && pgrepSelf.MatchString(in.Command) {
		return "a wait loop over pgrep -f is refused: the pattern matches the loop's own command line and never ends; " + bgRule
	}
	if !in.RunInBackground && backgrounded(in.Command) {
		return "backgrounding with & is refused: nothing wakes you when the job ends; " + bgRule
	}
	return ""
}

// backgrounded reports an unquoted lone &. It tracks single and double quotes, backslashes,
// comments and here-doc bodies, and skips &&, &>, >&, <& and |&.
// ponytail: $(( a & b )) reads as backgrounding; parse arithmetic if that ever bites.
func backgrounded(cmd string) bool {
	var quote byte
	for i := 0; i < len(cmd); i++ {
		c := cmd[i]
		switch {
		case quote != 0:
			if c == '\\' && quote == '"' {
				i++
			} else if c == quote {
				quote = 0
			}
		case c == '\\':
			i++
		case c == '\'' || c == '"':
			quote = c
		case c == '#' && (i == 0 || cmd[i-1] == ' ' || cmd[i-1] == '\t' || cmd[i-1] == '\n'):
			for i < len(cmd) && cmd[i] != '\n' {
				i++
			}
		case c == '<' && i+1 < len(cmd) && cmd[i+1] == '<' && (i+2 >= len(cmd) || cmd[i+2] != '<'):
			i = skipHeredoc(cmd, i+2)
		case c == '&':
			if i+1 < len(cmd) && cmd[i+1] == '&' {
				i++
			} else if !(i+1 < len(cmd) && cmd[i+1] == '>') && !(i > 0 && (cmd[i-1] == '>' || cmd[i-1] == '<' || cmd[i-1] == '|')) {
				return true
			}
		}
	}
	return false
}
```

`skipHeredoc(cmd string, at int) int` (same file): `at` is just past `<<`; read an optional `-`, spaces, an optionally quoted word as the delimiter; move to the end of the current line; consume following lines until one equals the delimiter (with leading tabs stripped when `-` was given); return the index of that line's last byte (len-1 when unterminated). The remainder of the command line after the delimiter word is still scanned as normal text, so `cat <<EOF & ` is not hidden: scan to the newline first by returning an index that resumes after the current line only when the body is skipped (implement as: find line end `e`; scan `cmd[at+len(word):e]` with `backgrounded`; if true return the sentinel `-1` handled by the caller as `return true`).

- [ ] **Step 3: Run** `go test ./internal/hook/ ./internal/cli -run 'Background|TestScripts/hook_pretool' -count=1` → PASS; run the whole `./internal/hook/...` (the fuzz seed corpus must still pass: R-HOOK-015).

- [ ] **Step 4: Lint, audit, commit** `feat(hook): refuse shell backgrounding and pgrep -f waits`.

---

## B2: `ness status` idle from the last real work entry

### Task 3: B2 acceptance tests and spec

**Files:**
- Modify: `docs/spec/sessions.md` (R-SESS-080 verify text)
- Create: `internal/atom/claude/transcript_test.go`, `internal/atom/claude/testdata/transcript-idle.jsonl`, `internal/cli/testdata/script/status_idle.txtar`

**Interfaces:**
- Consumes: `claude.Client.Transcript(home, dir, id) (session.Transcript, error)`; `fakeClient(t)` from `claude_test.go`.
- Produces: the contract that `Transcript.LastActive` is the timestamp of the last real work entry.

- [ ] **Step 1: Spec.** Amend R-SESS-080: "minutes idle" MUST be measured from the last real work entry of the transcript (a `user` or `assistant` entry that is not meta and not a compaction summary), never the file's modification time, which the client's own bookkeeping entries (away summaries, turn durations, metadata rewrites) refresh. Verify adds: unit — a transcript whose last real entry is hours older than later bookkeeping entries and the file mtime reports the real entry's time; a transcript with no timestamped entry falls back to the mtime.

- [ ] **Step 2: Fixture** `testdata/transcript-idle.jsonl`, shaped from a real Claude Code transcript (one JSON object per line): a `user` entry `2026-10-13T16:14:00Z` (string content), an `assistant` entry `2026-10-13T16:14:56Z` with usage `input_tokens 10, cache_read_input_tokens 1000, cache_creation_input_tokens 0`, then later non-work lines: `{"type":"system","subtype":"away_summary","timestamp":"2026-10-13T16:57:15Z"}`, `{"type":"system","subtype":"turn_duration","timestamp":"2026-10-13T16:58:00Z"}`, `{"type":"attachment","timestamp":"2026-10-13T17:00:00Z","attachment":{"type":"hook_success"}}`, `{"type":"queue-operation","timestamp":"2026-10-13T17:01:00Z"}`, a `user` entry with `"isMeta":true` at 17:02:00Z, a `user` entry with `"isCompactSummary":true` at 17:03:00Z, timestamp-less `{"type":"last-prompt"}`, `{"type":"custom-title"}`, and `not json`.

- [ ] **Step 3: Unit tests** in `transcript_test.go` (`// covers: R-SESS-080`): copy the fixture into a temp `~/.claude/projects/<encoded>/u-1.jsonl` (same layout as `TestTranscriptContextAndActivity`), `os.Chtimes` it to 21:25Z (the mtime the bookkeeping bumped), assert `LastActive == 2026-10-13T16:14:56Z` and `Context == 1010`. Second case: the existing style file with no timestamps (`TestTranscriptContextAndActivity`'s content) keeps `LastActive == mtime`. Third: a file of only `last-prompt` lines → mtime.

- [ ] **Step 4: Testscript** `status_idle.txtar`, modelled on `status.txtar` lines 8-28: root session, transcript with an assistant entry `"timestamp":"2026-10-08T08:20:00Z"` and `touch -d 2026-10-08T09:59:00Z`, `NESS_TEST_NOW=2026-10-08T10:00:00Z` → `stdout 'idle 100m'` and `--json` `"idle_minutes": 100`.

- [ ] **Step 5: Run, expect red.** `go test ./internal/atom/claude -run Transcript -count=1; go test ./internal/cli -run TestScripts/status_idle -count=1` → FAIL.

- [ ] **Step 6: Commit** `test(claude): idle minutes come from the last real work entry`.

### Task 4: B2 implementation

**Files:**
- Modify: `internal/atom/claude/claude.go` (`usageLine`, `Transcript`), `internal/session/client.go:25` (field comment)

**Interfaces:**
- Consumes: Task 3 tests.
- Produces: `Transcript.LastActive` = last real work entry time, else mtime.

- [ ] **Step 1:** extend `usageLine` with `Type string \`json:"type"\``, `IsMeta bool \`json:"isMeta"\``, `IsCompactSummary bool \`json:"isCompactSummary"\``, `Timestamp string \`json:"timestamp"\``. In the existing scan loop, before the usage test:

```go
if (u.Type == "user" || u.Type == "assistant") && !u.IsMeta && !u.IsCompactSummary {
	if at, perr := time.Parse(time.RFC3339Nano, u.Timestamp); perr == nil && at.After(last) {
		last = at
	}
}
```
with `var last time.Time` declared before the loop; after the loop `tr.LastActive = last` when `!last.IsZero()`, else keep `info.ModTime()` (set it before the loop as now). `time` is already imported.

- [ ] **Step 2:** `session.Transcript.LastActive` comment: "the time of the last real work entry; the file's mtime when the transcript has none".

- [ ] **Step 3: Run** `go test ./internal/atom/claude ./internal/cli -run 'Transcript|TestScripts/status' -count=1` → PASS.

- [ ] **Step 4: Lint, audit, commit** `fix(status): idle minutes from the last real transcript entry`.

---

## B4: `--subject` in merge mode

### Task 5: B4 acceptance tests and spec

**Files:**
- Modify: `docs/spec/sessions.md` (R-SESS-144), `docs/spec/profiles-builtin.md` (new R-BLT-106), `internal/cli/testdata/script/integrate_squash.txtar`, `internal/cli/testdata/script/core_text.txtar`

**Interfaces:**
- Produces: contract for Task 6.

- [ ] **Step 1: Spec.** Replace R-SESS-144: "`--subject` with any mode other than `squash` MUST be ignored with a warning on stderr (`--subject ignored: integrate.mode merge writes its own message`) and the integrate MUST proceed (merge, rebase and pr write their own messages)." Verify: testscript — `integrate.mode = "merge"` with `--subject` → exit 0, the warning, the target moved, the merge commit's message is not the subject. Add **R-BLT-106**: "`ness-launch` MUST tell a session to always pass `--subject 'type(scope): summary'` to `ness integrate`, and that a project not on `squash` ignores it with a warning (it never fails). Verify: unit — grep."

- [ ] **Step 2: Testscript.** In `integrate_squash.txtar` find the case asserting exit 4 for merge + `--subject` (search `R-SESS-144`) and change it to: `exec` succeeds, `stderr '--subject ignored'`, `git log -1 --format=%s main` does not contain the subject. In `core_text.txtar` add `grep 'Always pass `--subject`' .agents/skills/ness-launch/SKILL.md`, `grep 'ignores it with a warning' .agents/skills/ness-launch/SKILL.md`, `! grep 'omits `--subject`' .agents/skills/ness-launch/SKILL.md`; extend the `covers:` line with R-BLT-106.

- [ ] **Step 3: Run, expect red.** `go test ./internal/cli -run 'TestScripts/(integrate_squash|core_text)' -count=1` → FAIL.

- [ ] **Step 4: Commit** `test(integrate): --subject is ignored with a warning outside squash`.

### Task 6: B4 implementation

**Files:**
- Modify: `internal/cli/integrate.go` (`checkSubject`, `runIntegrate`), `profiles/core/skills/ness-launch/SKILL.md:49-51`

- [ ] **Step 1:** `checkSubject` keeps only the squash branch (drop the refusal for other modes). In `runIntegrate`, before `checkSubject`:

```go
if cfg.Integrate.Mode != "squash" && subject != "" {
	say(d.Stderr, "ness: --subject ignored: integrate.mode %s writes its own message", cfg.Integrate.Mode)
	subject = ""
}
```
Update the flag help to "ignored with a warning in other modes". Fix the `checkSubject` doc comment (R-SESS-140 only).

- [ ] **Step 2: Skill text** (lines 49-51) becomes:

```
`ness integrate --subject 'type(scope): summary'` folds your own branch through the gate into your
manager's branch as one commit with that Conventional Commit subject. Always pass `--subject`: a
project not on `integrate.mode = "squash"` ignores it with a warning.
```

- [ ] **Step 3:** `go run ./cmd/ness sync`; `go test ./internal/cli -count=1 -run 'TestScripts/(integrate|core_text)'` → PASS; `go run ./cmd/ness sync --check` exit 0.

- [ ] **Step 4: Lint, audit, commit** (include the re-rendered `.agents/skills/ness-launch/SKILL.md`, `.ness/lock.json`) `fix(integrate): warn and ignore --subject outside squash`.

---

## B6: post-compact reload

### Task 7: B6 acceptance tests and spec

**Files:**
- Modify: `docs/spec/profiles-builtin.md` (R-BLT-106 is taken by B4; use **R-BLT-107**), `docs/spec/hooks.md` (R-HOOK-007 verify), `internal/cli/testdata/script/hook_session_start.txtar`

**Interfaces:**
- Produces: contract for Task 8: with no project profile, a compact `session-start` prints core's reload lines after the cheat sheet.

- [ ] **Step 1: Spec.** R-BLT-107: "Core MUST append to the `ness-entry#post-compact` list slot the rules a compaction loses: how to talk to the operator (decision-first and short, a page for long content, `ness send --to operator` only for what the manager cannot settle) and the autonomy rule (use the level the brief states; a root never lowers a child's level; reserved decisions stay the operator's). The lines are printed by `session-start` after a compaction and not after startup, resume or clear, and the whole compact output stays within 20 lines with a two-line project profile. Verify: testscript."

- [ ] **Step 2: Testscript.** In `hook_session_start.txtar` extend `compact.txt` (the golden) with core's two reload lines before the profile's two lines (word them exactly as in Task 8 Step 1, one line each, `- ` prefix); `startup.txt` is unchanged (resume/clear/startup still `cmp stdout ../startup.txt`). The existing `! stdout '(?s)(.*\n){21}'` assertion stays. Add `stdout 'Operator communication'` and `stdout 'Autonomy'` on the compact run.

- [ ] **Step 3: Run, expect red.** `go test ./internal/cli -run TestScripts/hook_session_start -count=1` → FAIL.

- [ ] **Step 4: Commit** `test(hook): the post-compact reload carries operator and autonomy rules`.

### Task 8: B6 implementation

**Files:**
- Create: `profiles/core/post-compact.md`
- Modify: `profiles/core/profile.toml` (`[slots]`)

- [ ] **Step 1: `post-compact.md`** (exactly two lines; the test golden in Task 7 must match them):

```
- Operator communication: decision-first and short, long content on a page; ask the operator with `ness send --to operator` only for what your manager cannot settle.
- Autonomy: act at the level your brief states; a root never lowers a child's level; reserved decisions stay the operator's.
```

- [ ] **Step 2:** in `profile.toml` add `"ness-entry#post-compact" = { append = "post-compact.md" }` to `[slots]`.

- [ ] **Step 3:** `go run ./cmd/ness sync` (the list slot also renders into the "After compaction" section of the skill); `go test ./internal/cli -count=1 -run 'TestScripts/(hook_session_start|core_text|builtin)'` → PASS; `ness sync --check` exit 0. If a builtin golden lists the slot contents, update it by hand and say so in the commit.

- [ ] **Step 4: Lint, audit, commit** `feat(core): reload operator and autonomy rules after compaction`.

---

## B7: gate-worktree setup command

### Task 9: B7 acceptance tests and spec

**Files:**
- Modify: `docs/spec/core-config.md` (config example near line 129; new R-CFG-061), `docs/spec/sessions.md` (new R-SESS-146)
- Create: `internal/session/integrate_setup_test.go`, `internal/cli/testdata/script/integrate_setup.txtar`
- Stub: add `Setup string` to `session.IntegrateSpec` and `Setup string \`toml:"setup"\`` to `config.Integrate` (fields only)

**Interfaces:**
- Consumes: `session.Integrate(g, it, spec)`, `testkit.NewRepo`, `testkit.Commit`, the helpers used in `integrate_test.go`.
- Produces: contract: `IntegrateSpec.Setup` runs before the gate in the throwaway worktree.

- [ ] **Step 1: Spec.** R-CFG-061: "`[integrate] setup` is an optional shell command (default empty)." R-SESS-146: "When `integrate.setup` is set, `ness integrate` MUST run it with `sh -c` in the throwaway worktree, after the fold and inside the gate lock, immediately before the gate; a non-zero exit MUST fail the integrate like a failing gate (`ErrGate`, target unmoved, throwaway removed) without running the gate; its output goes to the gate log. Unset, nothing runs. Verify: unit and testscript." Add `setup = "just data-link"  # runs in the gate worktree before the gate (optional)` to the `[integrate]` example.

- [ ] **Step 2: Unit tests** (`// covers: R-SESS-146`, `package session_test`, patterned on `TestIntegrateRunsTheGateInAThrowawayAndMovesTheTarget`): (a) `Setup: "touch linked"`, `Gate: "test -f linked"` → integrate succeeds, and `linked` does not exist in the real checkout; (b) `Setup: "echo boom; exit 3"`, `Gate: "touch gate-ran-" + marker` outside the repo → `errors.Is(err, session.ErrGate)`, log contains `boom`, the marker file does not exist, target sha unchanged, no leftover `tmp-*` under `<common>/ness`; (c) empty `Setup` behaves as before (the existing tests cover it; assert nothing extra). Also a `config` test in `internal/config/config_test.go`: a file with `[integrate]\nsetup = "just data-link"` loads `c.Integrate.Setup`.

- [ ] **Step 3: Testscript** `integrate_setup.txtar`: `ness init --gate 'test -f linked' --base main`, put `[integrate]\nsetup = "touch linked"` in `.ness/ness.toml` (as `integrate_gate.txtar` does with `squash.toml`), open a root, commit a file, `ness integrate --subject "feat: x"` → exit 0 and `main` moved; with `setup = "false"` → exit 1 and `main` unchanged.

- [ ] **Step 4: Run, expect red.** `go test ./internal/session ./internal/config ./internal/cli -run 'Setup|TestScripts/integrate_setup' -count=1` → FAIL.

- [ ] **Step 5: Commit** `test(integrate): a setup command runs before the gate`.

### Task 10: B7 implementation

**Files:**
- Modify: `internal/session/integrate.go` (`runGate`), `internal/cli/integrate.go` (`fold`), `internal/config/config.go` (doc on `Integrate.Setup`)

- [ ] **Step 1:** in `fold`, pass `Setup: sp.cfg.Integrate.Setup` in `session.IntegrateSpec`. In `runGate`, after `defer unlock()` and before the gate:

```go
if strings.TrimSpace(s.Setup) != "" {
	out, err := g.Exec(tmp, "sh", "-c", s.Setup)
	_, _ = log.Write(out) //nolint:errcheck // a broken log writer must not decide the gate
	if err != nil {
		_, _ = fmt.Fprintln(log, err) //nolint:errcheck // same
		return fmt.Errorf("%w: setup: %w", ErrGate, err)
	}
}
```
(`strings` is already imported.) Document the field: "Setup runs in the throwaway worktree before Gate (R-SESS-146)".

- [ ] **Step 2: Run** `go test ./internal/session ./internal/config ./internal/cli -count=1 -run 'Setup|TestScripts/integrate'` → PASS.

- [ ] **Step 3: Lint, audit, commit** `feat(integrate): run integrate.setup in the gate worktree before the gate`.

---

## B3: stall backstop for a child that has not handed off

### Task 11: B3 acceptance tests and spec

**Files:**
- Modify: `docs/spec/worker.md` (R-WRK-050, R-WRK-052 verify)
- Create: `internal/worker/stalls_brief_test.go`
- Modify: `internal/worker/stalls_test.go` (`TestStallCountsFromTheObligation` premise only)

**Interfaces:**
- Consumes: the `bench` fakes in `fakes_test.go` (`newBench`, `session`, `send`, `passes`, `listed`, `typedTo`, `pane(...).status`).
- Produces: contract: a child with no handoff counts as owing its brief.

- [ ] **Step 1: Spec.** Extend R-WRK-050: "A child session (one with a manager that is a session) that has never sent a `handoff` owes its brief: idle for `worker.stall_minutes` it MUST get the same single stall message per episode to its manager even with no open obligation, worded `… has been idle since T and has not handed off its brief`; the clock is the start of the idle episode. The worker MUST NOT type into the child. A root, and a child that sent a handoff, with no obligation get no stall message."

- [ ] **Step 2: Tests** in `stalls_brief_test.go` (`// covers: R-WRK-050, R-WRK-052`): (1) A root, B child, no handoff, nothing owed: pass, 29 one-minute passes → A's `listed` has no fyi; one more pass → exactly one fyi from B whose body contains `has not handed off`; 30 more passes → still one; B `working` one pass then `idle` 31 passes → a second. (2) `b.typedTo(sB)` has no text containing `idle since` (the child is never woken). (3) B sent `handoff` to A first (`b.send(sB, sA, store.IntentHandoff, …)`): idle 90 minutes → no stall fyi from B (filter `listed(sA)` to `Intent == IntentFYI`). (4) the root A itself idle 90 minutes with no children and nothing owed → the operator's `listed` is empty. (5) a child that is not ready yet (do not call `b.pass()` before readiness; use a session with `ReadyAt` zero via the existing helper pattern in `worker_test.go` for launching sessions) → no message. (6) child with an obligation behaves as before (existing tests).

- [ ] **Step 3: Fix the old premise.** `TestStallCountsFromTheObligation` has B idle two hours "with nothing owed"; under the new rule B would stall at 30 min. Make B hand off first (`b.send(sB, sA, store.IntentHandoff, "done", false)`), have the assertions look only at fyi items of `b.listed(sA)`, and update the comment to say the parked session is a handed-off child.

- [ ] **Step 4: Run, expect red.** `go test ./internal/worker -run 'Stall' -count=1` → the new cases FAIL, the amended one passes.

- [ ] **Step 5: Commit** `test(worker): a child that has not handed off owes its brief`.

### Task 12: B3 implementation

**Files:**
- Modify: `internal/store/items.go` (new method), `internal/worker/stalls.go` (`reportStall`)

**Interfaces:**
- Produces: `func (s *Store) HandedOff(sender string) (bool, error)`.

- [ ] **Step 1:** in `items.go`:

```go
// HandedOff reports whether sender ever sent a handoff, in any disposition: the child has
// delivered its brief (R-WRK-050).
func (s *Store) HandedOff(sender string) (bool, error) {
	var n int
	err := s.db.QueryRowContext(context.Background(),
		`SELECT EXISTS (SELECT 1 FROM items WHERE sender = ? AND intent = 'handoff')`, sender).Scan(&n)
	return n == 1, err
}
```

- [ ] **Step 2:** rewrite the middle of `reportStall`:

```go
obl, err := w.Store.Obligations(sess.ID)
if err != nil { w.Log.Error(sess.ID+" stall", err); return }
what := fmt.Sprintf("%d open obligation(s): %s", len(obl), joinIDs(obl))
if len(obl) == 0 {
	if sess.Parent == store.Operator { return } // a root owes nothing the worker can see (R-WRK-093 covers it)
	done, herr := w.Store.HandedOff(sess.ID)
	if herr != nil { w.Log.Error(sess.ID+" stall", herr); return }
	if done { return }
	what = "not handed off its brief"
} else if now.Sub(obl[0].CreatedAt) < w.Stall {
	return
}
```
then the existing `tell` with the body `"ness worker: session %s (%s) has been idle since %s and has %s"`; for the obligation case keep today's exact text (`with %d open obligation(s): …`) so existing assertions on `idle since` hold; for the brief case use `"has not handed off its brief"`. Extract `joinIDs`. Log action `stall` with `what`.

- [ ] **Step 3: Run** `go test ./internal/worker -count=1 -race` → PASS.

- [ ] **Step 4: Lint, audit, commit** `feat(worker): stall report for a child that has not handed off`.

---

## B5: operator alerts (specs.md §1)

Contract summary for all B5 tasks: kinds `needs-you`, `checkpoint`, `trouble`, `your-move` with titles 🟠 Needs you, 🟢 Checkpoint, 🔴 Trouble, 🟡 Your move; format and thread as in `specs.md` §1 "Alert format".

### Task 13: B5-1 store and send: acceptance tests and spec

**Files:**
- Modify: `docs/spec/store-messages.md`
- Create: `internal/store/alerts_test.go`, `internal/cli/testdata/script/send_checkpoint.txtar`
- Modify: `internal/cli/testdata/script/{send,surface,hook_session_start}.txtar` and any golden that lists send's intents or flags (find with `grep -rn "request | fyi | decision | handoff\|accepted values are" internal`)
- Stub (compile only, zero values): `store.IntentCheckpoint`, `store.CheckpointMissing`, `store.HasAsk`, `store.Section`, `store.PageOf`, `(*Store).UnalertedToOperator`, `(*Store).MarkAlerted`

**Interfaces (the contract Task 14 implements):**

```go
const IntentCheckpoint = "checkpoint"
func CheckpointMissing(body string) []string // of "Done","Now","Next": those without a line `## <name>` (trailing blanks allowed), in that order
func HasAsk(body string) bool                // a line `## Ask`
func Section(body, heading string) string    // first non-blank line under `## heading`, "" when none
func PageOf(body string) string              // URL of the first line `Page: URL`, "" when none
func (s *Store) UnalertedToOperator() ([]Item, error) // open request/decision/handoff/checkpoint items to the operator with alerted_at NULL, not retired, oldest first
func (s *Store) MarkAlerted(id string, now time.Time) error
```
`Item.Obligation()` is true for a `checkpoint` whose body `HasAsk`.

- [ ] **Step 1: Spec.** `store-messages.md`: R-MSG-010 add `checkpoint` and `--page URL`; R-MSG-040 add "a `checkpoint` with an `## Ask` section"; R-MSG-041 add "a `checkpoint` without `## Ask` carries no obligation and, like an fyi, leaves the inbox once shown"; **R-MSG-065** "a `checkpoint` body MUST contain the headings `## Done`, `## Now` and `## Next`; `send` refuses (exit 2) a body without any of them, naming the missing ones, and creates nothing"; **R-MSG-066** "only a root sends a `checkpoint`, and only `--to operator`; any other sender or recipient is refused (exit 4)"; `--page URL` appends the line `Page: URL` to the body unless the body already has a `Page:` line, is refused with other intents (exit 2); **R-MSG-067** "Migration 7 rebuilds `items` with intent `checkpoint`, the columns `ask` (0/1) and `alerted_at`, keeps every row, and stamps `alerted_at = created_at` on existing rows". Each with a Verify line naming the tests below.

- [ ] **Step 2: Store tests** (`package store_test`, `// covers: R-MSG-040, R-MSG-041, R-MSG-067` …; helpers `mustOpen`, `raw`, `Migrations`/`MigrateWith` from `withdraw_test.go`; copy the version-6 fixture approach of `TestMigrationSixKeepsItemsAndAcceptsTheNewDispositions`): (a) migration 7 on a version-6 store with a request (open) and a deferred decision keeps both rows and sets their `alerted_at`; `UnalertedToOperator` is empty afterwards; (b) a fresh store: operator-bound `request`, `decision`, `handoff`, `checkpoint` (with `## Ask`) and `checkpoint` (without) are all returned oldest first; an `fyi` and a `reply` to the operator are not; an item to a session is not; (c) `MarkAlerted` removes the item from the selection and the stamp survives a close/reopen (restart repeats nothing); (d) an item replied or withdrawn before the call is not selected; a retired item is not; (e) `Obligations(recipient)`/`OpenToOperator(sender)` include a checkpoint with `## Ask` and exclude one without; the checkpoint without Ask leaves `Listed(operator)` after `MarkShown`, the one with Ask does not; (f) pure-function tables for `CheckpointMissing` (missing one, all, none; `## Done extra` and `### Done` do not count; a heading inside indented text does not), `HasAsk`, `Section` (first non-blank line, stops at the next `## `), `PageOf`.

- [ ] **Step 3: Testscript** `send_checkpoint.txtar` (style of `send.txtar`): root `s-aaaaaaaaaaaa`, child `s-bbbbbbbbbbbb`; a body file with the three headings: root `ness send --to operator --intent checkpoint --body @../cp.md --page https://example.test/p` → `stdout '^sent m-[a-z2-7]{8} to operator$'`; `ness inbox --show` (operator) ends with `Page: https://example.test/p`; the same send again → `already sent`; body missing `## Next` → `exits 2`, `stderr 'Next'`, `! stderr 'Done'`, no item created (`store items operator N`); body with `## Ask` → item blocks the root's close (use the pattern of `close_obligations.txtar`, or assert via `ness status` waits count); child `--intent checkpoint --to manager` → `exits 4`; root `--to <child id>` checkpoint → `exits 4`; `--page` with `--intent fyi` → `exits 2`. Update `send.txtar` (the accepted-values message becomes `request, fyi, decision, handoff, checkpoint`), the cheat sheet in `hook_session_start.txtar` goldens (`[--intent request|fyi|decision|handoff|checkpoint]`; keep ≤ 20 lines) and the surface/help goldens by hand.

- [ ] **Step 4: Run, expect red.** `go test ./internal/store ./internal/cli -count=1 -run 'Checkpoint|Alert|Migration|TestScripts/(send|surface|hook)'` → FAIL on assertions, compiles.

- [ ] **Step 5: Commit** `test(store): acceptance tests for checkpoint items and delivery state`.

### Task 14: B5-1 store and send: implementation

**Files:**
- Create: `internal/store/checkpoint.go`
- Modify: `internal/store/{items,migrate}.go`, `internal/cli/send.go`, `internal/cli/hook.go` (cheat sheet), `internal/cli/inbox.go` (`intentColor`: `checkpoint` → `"ok"`)

- [ ] **Step 1: `checkpoint.go`:** the five pure functions of Task 13 with one anchored regex, `headingRe = regexp.MustCompile("(?m)^## (Done|Now|Next|Ask)[ \\t]*$")`; `Section` scans lines, finds `## heading`, returns the first following non-blank line before the next line starting `## `. Add the const to `items.go`.

- [ ] **Step 2: Migration 7** appended to `migrations` (never edit shipped ones):

```sql
CREATE TABLE items_v7 ( …all v6 columns…,
  intent TEXT NOT NULL CHECK (intent IN ('request','fyi','decision','handoff','reply','checkpoint')),
  ask INTEGER NOT NULL DEFAULT 0, alerted_at TEXT );
INSERT INTO items_v7 (id, key, sender, recipient, intent, body, urgent, created_at, pulled_at, shown_at,
  disposition, answer, disposed_at, retired_at, probe, alerted_at)
SELECT id, key, sender, recipient, intent, body, urgent, created_at, pulled_at, shown_at,
  disposition, answer, disposed_at, retired_at, probe, created_at FROM items;
DROP TABLE items; ALTER TABLE items_v7 RENAME TO items;
CREATE INDEX items_recipient ON items (recipient, disposed_at);
```
(copy the v6 table definition, including the disposition CHECK, from migration 6). Update the comment above `migrations` ("migration 7: checkpoint, ask, alerted_at").

- [ ] **Step 3: items.go.** `Send`: compute `ask := intent == IntentCheckpoint && HasAsk(body)` and insert it (`insertItem` gets `ask`; reply inserts keep the default by naming columns). `Obligation()`: add `|| (it.Intent == IntentCheckpoint && HasAsk(it.Body))`. SQL: `obliged` becomes `… AND (intent IN ('request','decision','handoff') OR (intent = 'checkpoint' AND ask = 1)) AND retired_at IS NULL`; `listed` adds `AND NOT (intent = 'checkpoint' AND ask = 0 AND shown_at IS NOT NULL)`. Add:

```go
// UnalertedToOperator lists the open items to the operator that no alert covered yet, oldest first.
func (s *Store) UnalertedToOperator() ([]Item, error) {
	return queryItems(s.db, selectItem+` WHERE recipient IS NULL AND alerted_at IS NULL AND retired_at IS NULL
		AND disposition IS NULL AND intent IN ('request','decision','handoff','checkpoint')`+inOrder)
}

// MarkAlerted records that an alert covered the item; the first stamp is kept.
func (s *Store) MarkAlerted(id string, now time.Time) error {
	return s.touch(ErrNoItem, `UPDATE items SET alerted_at = COALESCE(alerted_at, ?) WHERE id = ?`, stamp(now), id)
}
```

- [ ] **Step 4: send.go.** `sendIntents` += checkpoint; error text and flag help list it; new `--page URL` flag (`sendOpts.page`). In `RunE`, before the store: `--page` with another intent → `usage`. In `runSend`, before `s.Send`: for `checkpoint`: `c.root() && to == store.Operator` else `refuse("only a root sends a checkpoint, and only to the operator")`; `missing := store.CheckpointMissing(body)` non-empty → `usage("a checkpoint needs the headings: %s", strings.Join(prefixed, ", "))` naming only the missing ones as `## Next`; if `o.page != ""` and `store.PageOf(body) == ""` then `body += "\n\nPage: " + o.page` (trim a trailing newline first). Example in `Use` gets a checkpoint line. Cheat sheet line in `hook.go` gets `|checkpoint`.

- [ ] **Step 5: Run** `go test ./internal/store ./internal/cli -count=1 -run 'Checkpoint|Alert|Migration|TestScripts'` → PASS (whole script suite, since goldens moved).

- [ ] **Step 6: Lint, audit, commit** `feat(msg): checkpoint intent, page link and alert delivery state`.

### Task 15: B5-2 worker alerts and Your move: acceptance tests and spec

**Files:**
- Modify: `docs/spec/worker.md` (replace R-WRK-080 and R-WRK-082; keep R-WRK-081 text but route to the operator as Trouble; add R-WRK-093, R-WRK-094; R-WRK-083 unchanged)
- Replace: `internal/worker/operator_test.go` (old R-WRK-080/081/082 tests), Modify `internal/worker/fakes_test.go` only to add the alert recorder helper
- Create: `internal/worker/alerts_test.go`, `internal/atom/claude/background_test.go`, `internal/atom/claude/testdata/footer-1-shell.txt`
- Stub: `worker.Alert`, `worker.AlertKind` consts, `Worker.Notify func(Alert) error`, `session.Screen.Background` (zero use)

**Interfaces (contract for Task 16):**

```go
type AlertKind string
const (AlertNeedsYou AlertKind = "needs-you"; AlertCheckpoint = "checkpoint"; AlertTrouble = "trouble"; AlertYourMove = "your-move")
type Alert struct {
	Kind                            AlertKind
	Project, Title, Tab, Text, Page string // Title: session title; Tab: herdr tab label (= session title); Text: <= 200 runes
	Root                            string // id of the root of the tree: the thread key
}
// Worker.Notify func(Alert) error  (nil: no channel, nothing sent, nothing logged, items stay unalerted)
```

- [ ] **Step 1: Spec.** R-WRK-080 replaced: "Each pass, for every open request, decision or handoff to the operator, and every checkpoint, whose `alerted_at` is null the worker MUST send one alert and stamp it on success; kind Needs you, except a checkpoint without `## Ask` (kind Checkpoint). Text: the item's first line (Checkpoint: first line under `## Next`; Needs you from a checkpoint: under `## Ask`), at most 200 runes; Page from `Page:`. It does not wait for idle. A restart repeats none; an item disposed before its alert gets none." R-WRK-081: the blocked prompt is a Trouble alert. R-WRK-082 replaced by the alert format (worker's `Alert`, rendering in cli, Task 17). **R-WRK-093**: "A root idle for `worker.stall_minutes` that holds no open item to the operator, with no open descendant working, blocked, not yet seen this run or showing a background task on its screen, MUST get one Your move alert per idle episode (reset when it works again)." **R-WRK-094**: "A stall report (R-WRK-050), a screen error report (R-WRK-051), a stop for an unavailable model (R-WRK-062) and a pane blocked for 2 minutes (R-WRK-081) MUST each also send one Trouble alert per episode, text = the worker's own sentence." Add the Claude screen requirement to the client section: `Read` sets `Background` when the footer shows `· N shell(s)`.

- [ ] **Step 2: Screen test** `background_test.go` (`package claude_test`, `// covers: R-WRK-093`): fixture `footer-1-shell.txt` = the real footer captured 2026-10-15: an empty prompt box (rule lines, `❯`, rule) then `  Fable 5.1 | l0b-band… | effort: med | 5h 8% @16:20 | 7d 6%` and `  ⏵⏵ bypass permissions on · 1 shell · ← for agents`. Cases: `1 shell` and `3 shells` → `Read(...).Background == true`; the same footer without `· N shell` → false; the text `· 1 shell` in agent prose above the box (not in the last 3 lines) → false; `Draft` and `NoPrompt` unchanged for the fixture.

- [ ] **Step 3: Worker tests** in `alerts_test.go` (stepped clock via `b.passes`, `bench`, recorder `type alerts struct{ got []worker.Alert; fail int }` with `post(a worker.Alert) error`; `b.w.Notify = rec.post`; a `b.restart()` helper builds a second `worker.Worker` on the same store/runtime/clock). Cases, each `// covers:` its R-id:
  1. Needs you at creation, pane working: root A `working`, sends `decision` to the operator `"Go on L0b?\nBand study…"` → one pass → one alert `{Kind: NeedsYou, Project: "infra-lab", Title: "x", Tab: "x", Text: "Go on L0b?", Root: sA}`; 20 more passes → still one; the item's `alerted_at` set (`UnalertedToOperator` empty).
  2. A child B's `request` to the operator → alert with `Root: sA`, `Title` = B's title.
  3. `fyi` to the operator → none; an item replied (`b.s.Reply`) or withdrawn before the first pass → none.
  4. Restart: after case 1 `b.restart()` + 3 passes → no second alert. Post failure: `fail: 2` → passes 1-2 log one `notify` error line (`b.logCount(" notify "+…) == 1`, R-WRK-083), pass 3 delivers once and stamps; a failed post leaves the item unalerted.
  5. Checkpoint, no Ask: body with `## Done/## Now/## Next` and `Page: https://claude.ai/x` → `{Kind: Checkpoint, Text: <first line under ## Next>, Page: "https://claude.ai/x"}`; with `## Ask\nShip it?` → `{Kind: NeedsYou, Text: "Ship it?"}`; Text longer than 200 runes is cut to exactly 200.
  6. Trouble: pane `blocked` 1 min 59 s → none, 2 min → one Trouble alert, later passes none, working then blocked 2 min again → second; child stall (B idle 30 min with an open request from A) → one Trouble with Text containing `idle since`; error screen and unavailable model (copy the fixtures of `stalls_test.go`: `errorScreen`, the unavailable-model screen) → one each; a Trouble alert's post failing is retried without a second stall item (`b.listed(sA)` unchanged).
  7. Your move: root A idle `Stall` minutes (30 one-minute passes after the first), no open operator item, no children → exactly one `{Kind: YourMove, Text: "idle for 30 minutes; nothing appears to be running"}` (Text wording: `idle for N minutes; nothing appears to be running`); no more until A is `working` for a pass and idle 30 min again. Suppressed (no alert, 40 passes) when: A holds an open `request` to the operator; B (child) `working`; B `blocked`; B grandchild C working while B idle; B idle with screen `idleScreen` plus a footer line `  ⏵⏵ bypass permissions on · 1 shell · ← for agents`; B launching (no pane memory yet). Fires (dead child): B idle with a plain screen for the full 40 passes. A child is never given Your move (only roots).
  8. `Notify == nil`: items stay unalerted, `b.logLines()` has no `notify`.

- [ ] **Step 4: Run, expect red.** `go test ./internal/worker ./internal/atom/claude -count=1` → FAIL on assertions, compiles. Old tests of R-WRK-080/082 are gone with `operator_test.go`.

- [ ] **Step 5: Commit** `test(worker): acceptance tests for operator alerts and Your move`.

### Task 16: B5-2 worker alerts and Your move: implementation

**Files:**
- Create: `internal/worker/alert.go`
- Modify: `internal/worker/{worker,operator,stalls}.go`, `internal/session/screen.go` (`Screen.Background`), `internal/atom/claude/screen.go` (`Read`), `internal/cli/worker.go` (temporary adapter, see Step 6)

- [ ] **Step 1: `alert.go`:** `AlertKind`, consts, `Alert` as in Task 15, and

```go
// alertItems alerts every unalerted open operator item and stamps the ones that went out
// (R-WRK-080, R-WRK-083); called once per pass, before the panes.
func (w *Worker) alertItems(now time.Time, open map[string]store.Session)
```
It returns at once when `w.Notify == nil`. For each item of `w.Store.UnalertedToOperator()`: sender session from `open` (fall back to `w.Store.Session`; a vanished sender uses title `?`); `Root` by walking `Parent` until `store.Operator`; kind/text per Task 15 Step 1 (`store.Section`, `store.PageOf`, `store.Excerpt(first, 200)`); on `w.Notify` error `w.Log.Error(item.ID+" notify", err)` and continue; on success `w.Log.Clear`, `w.Log.Action("notify", sess.ID, text)`, `w.Store.MarkAlerted(item.ID, now)`. Also `func (w *Worker) once(sess store.Session, p *pane, key string, kind AlertKind, text string)`: builds the Alert (Project, Title, Tab = `sess.Title`, Root), skips when `p.alerted[key]`, sets it on success, logs like `notify` today. `p.alerted map[string]bool` replaces `notified` and `blockedTold`; `pane.working` clears keys `stall`, `error`, `yourmove`; `noteBlocked` clears `blocked` when the status leaves blocked.

- [ ] **Step 2: `operator.go`:** delete `notifyAsked`; `noteBlocked` calls `w.once(sess, p, "blocked", AlertTrouble, "is blocked on a prompt in its tab")` after `BlockedAfter`. Remove the `w.notifyAsked(...)` call in `visit`.

- [ ] **Step 3: `stalls.go`:** after `reportStall`'s `tell` succeeds, `p.stalled = true` and remember `p.stallText` (the sentence); in `act`, after `reportError` and `reportStall`, call `w.once` for `stall` (when `p.stalled`, text `p.stallText`) and `error` (when `p.errored`, text `shows "<name>" on its screen`); `stop` calls `w.once(sess, p, "stop", AlertTrouble, "stopped: its tier's model id no longer resolves")` after a successful tell. This keeps a failed post retried on later passes without a second item.

- [ ] **Step 4: Your move.** `Worker` gets `open map[string]store.Session` rebuilt each `Pass` (the `live` map already built there) and `Pass` calls `w.alertItems(now, live)` right after the probes block; `pane` gets `status string` (set in `visit` right after `agent`) and `background bool` (set in `observe` from `sc.Background`; cleared when the pane is not idle). In `act`, after the reports: `w.yourMove(sess, p, now)`:

```go
func (w *Worker) yourMove(sess store.Session, p *pane, now time.Time) {
	if sess.Parent != store.Operator || p.alerted["yourmove"] || now.Sub(p.idleSince) < w.Stall {
		return
	}
	held, err := w.Store.OpenToOperator(sess.ID)
	if err != nil { w.Log.Error(sess.ID+" notify", err); return }
	if len(held) > 0 || w.treeBusy(sess.ID) {
		return
	}
	w.once(sess, p, "yourmove", AlertYourMove,
		fmt.Sprintf("idle for %d minutes; nothing appears to be running", int(now.Sub(p.idleSince).Minutes())))
}
```
`treeBusy(root)` walks `w.open` for descendants of root: busy when `w.panes[id]` is nil (not seen yet), or `status` is `working`, `blocked` or `unknown`... use `!idle(status) || background`. Keep `alerted` reset in `working()`.

- [ ] **Step 5: Screen.** `session.Screen.Background bool` ("the footer shows a live background task"). In `claude.Read`, over the last three non-empty lines: `bgFooter = regexp.MustCompile(`·\s*\d+\s+shells?\b`)`.

- [ ] **Step 6: Compile the composition root.** Replace the closure in `wireNotify` with a temporary adapter `w.Notify = func(a worker.Alert) error { _, err := poster.Post(notify.Request{Channel: op, Markdown: a.Text}); return err }` so the build stays green; Task 18 replaces it.

- [ ] **Step 7: Run** `go test ./internal/worker ./internal/atom/claude ./internal/session -count=1 -race` → PASS. The worker tests run `-count=4` in the gate: check `go test ./internal/worker -count=4`.

- [ ] **Step 8: Lint, audit, commit** `feat(worker): operator alerts at creation, Trouble kinds and Your move`.

### Task 17: B5-3 renderer, wiring and skill text: acceptance tests and spec

**Files:**
- Modify: `docs/spec/worker.md` (R-WRK-082 text), `docs/spec/profiles-builtin.md` (R-BLT-102, R-BLT-035, new R-BLT-108), `internal/cli/testdata/script/{core_text,builtin_program}.txtar`
- Create: `internal/cli/alert_render_test.go`, `internal/cli/testdata/golden/alert_{needs-you,checkpoint,trouble,your-move}_{telegram,slack}.txt` (hand-written)
- Stub: `func (a Alert) Markdown() string { return "" }` in `internal/worker/alert.go`

**Interfaces:**
- Produces: `func (a Alert) Markdown() string` (in `worker`), the note the poster renders:

```
**🟠 Needs you · the research lab / study-protocol**
Go on L0b? Band study over the three income cells, 2 children.
↳ tab study-protocol · page https://claude.ai/x
```
Title emoji and name per kind (🟠 Needs you, 🟢 Checkpoint, 🔴 Trouble, 🟡 Your move); `· page URL` only when `Page != ""`; the thread key is `"alerts:" + Root` (set by the wiring, Task 18).

- [ ] **Step 1: Spec.** R-WRK-082 (replaced): the format above, one Markdown note per alert, thread per root; no channel, nothing sent and nothing logged. R-BLT-102 amended: a root's every ask goes through `ness send --to operator` because the operator reads alerts, not the chat; the chat copy is optional; a root reports progress with a `checkpoint`. R-BLT-035: "a checkpoint at each landing, plan change or scoring change" replaces "a heartbeat line at each wave event". **R-BLT-108**: the core post-compact line states that a root's every ask goes through `ness send --to operator`.

- [ ] **Step 2: Golden tests** `alert_render_test.go` (`package cli_test`, `// covers: R-WRK-082`): for each kind build an `worker.Alert` (project `research-lab`, title `study-protocol`, text as in the spec example, the Checkpoint with a page, the Trouble `is blocked on a prompt in its tab`, the Your move `idle for 30 minutes; nothing appears to be running`), run `notify.Split(telegram.Channel{}, notify.Parse(a.Markdown()))` and `slack.Channel{}`, join `Part.Text()`, compare with `testkit.Golden`. Before writing each golden file, run the renderer once to see the shape and verify by eye that it is the spec's (bold title line, text line, location line, URL as the channel renders a bare URL), then hand-write the file; never `-update`. Add: a Needs you whose text has `<` and `&` (escaping in Telegram), and a 200-rune text stays one part.

- [ ] **Step 3: Skill-text scripts.** `core_text.txtar`: `grep 'because the operator reads alerts, not the chat' .agents/skills/ness-entry/SKILL.md`, `grep '`checkpoint`' .agents/skills/ness-entry/SKILL.md`, `! grep 'Put the same full text in the chat' …` (replaced by "The chat copy is optional."), `grep 'every ask goes through' ` on the rendered post-compact section; `builtin_program.txtar`: `grep 'send a checkpoint at each' .agents/skills/ness-program/SKILL.md`, `! grep 'heartbeat line' …`; add the covers ids.

- [ ] **Step 4: Run, expect red.** `go test ./internal/cli -count=1 -run 'Alert|TestScripts/(core_text|builtin_program)'` → FAIL on assertions, compiles.

- [ ] **Step 5: Commit** `test(cli): alert rendering goldens and the operator-communication text`.

### Task 18: B5-3 renderer, wiring and skill text: implementation

**Files:**
- Modify: `internal/worker/alert.go` (`Markdown`), `internal/cli/worker.go` (`wireNotify`), `profiles/core/skills/ness-entry/SKILL.md` ("Ask the operator", "Communicate"), `profiles/core/post-compact.md`, `profiles/builtin/program/skills/ness-program/SKILL.md:30`

- [ ] **Step 1: `Markdown()`:**

```go
var kindTitle = map[AlertKind]string{AlertNeedsYou: "🟠 Needs you", AlertCheckpoint: "🟢 Checkpoint",
	AlertTrouble: "🔴 Trouble", AlertYourMove: "🟡 Your move"}

// Markdown is the note the poster renders for the channel (R-WRK-082).
func (a Alert) Markdown() string {
	where := "↳ tab " + a.Tab
	if a.Page != "" {
		where += " · page " + a.Page
	}
	return fmt.Sprintf("**%s · %s / %s**\n%s\n%s", kindTitle[a.Kind], a.Project, a.Title, a.Text, where)
}
```
Adjust to whatever line-break form made the goldens of Task 17 come out as agreed (the goldens are the judge; do not edit them).

- [ ] **Step 2: `wireNotify`:**

```go
w.Notify = func(a worker.Alert) error {
	_, err := poster.Post(notify.Request{Channel: op, Markdown: a.Markdown(), Thread: "alerts:" + a.Root})
	return err
}
```

- [ ] **Step 3: Skill text.** `ness-entry` "Ask the operator": replace "Put the same full text in the chat." with "A root's every ask goes through `ness send --to operator`, because the operator reads alerts, not the chat; a chat copy is optional." and add one line under "Communicate": "A root reports progress to the operator with `ness send --to operator --intent checkpoint --body @FILE [--page URL]`: the body has `## Done`, `## Now` and `## Next` (add `## Ask` to ask something); a quick root sends one at the end, a program root at each landing, plan change or scoring change. Children's handoffs use the same three headings." `post-compact.md`: reword the Operator-communication line to start with "a root's every ask goes through `ness send --to operator` (the operator reads alerts, not the chat); progress goes as a `checkpoint`;" and keep two lines (update the golden in `hook_session_start.txtar` is Task 7's file: this is the one allowed edit of it, copy the new lines verbatim from Task 17's R-BLT-108 wording). `ness-program` line 30 becomes "- Send a checkpoint at each landing, plan change or scoring change: what started, what landed, what waits." Remove the duplicated cadence prose in `ness-program` that repeats it (grep `wave event`).

- [ ] **Step 4:** `go run ./cmd/ness sync`; `go test ./internal/cli ./internal/worker -count=1` → PASS; `ness sync --check` exit 0.

- [ ] **Step 5: Lint, audit, commit** `feat(notify): render operator alerts per kind; operator-communication text`.

---

## Review

### Task 19: Fresh-reviewer pass over Batch B

**Files:** read-only review of `git diff main...HEAD` for the branch.

- [ ] **Step 1: Dispatch one fresh `ness-review` subagent** (not the implementers) with: this plan, `docs/sessions/field-feedback/specs.md` §1, the diff, and the rules in `.claude/skills/ness-test-strategy`, `ness-test-discipline`, `ness-interface-design`, `ness-documentation`. Ask for findings ranked by severity against: each R-id amended or added has a test with a `covers:` marker and a matching spec line; no test was edited by its implementer (check task commit boundaries); no sleeps, permissive mocks or call-count assertions; the Review Focus list; `Alert`/`Notify` port consumer-declared, no package state; exported identifiers documented; `docs/spec` and `.agents/skills` in sync.
- [ ] **Step 2: Run the compliance check** (`ness-compliance-check` skill) and fix or own every finding; fix review findings in follow-up commits (`fix(...)`), tests changed only by a test-writing pass.
- [ ] **Step 3: Gate.** Run `ness integrate --subject 'feat(field-feedback): batch B operator alerts and liveness fixes'` as the session's integrate (never `make validate` by hand); check its exit status; do not pipe it. Capture the result in `docs/sessions/field-feedback/report.md` lessons.
- [ ] **Step 4: Field check (operator-gated, not run by this plan).** After merge and with the operator's agreement: a dry run of each alert kind against the research lab's channel configuration (`ness post --dry-run` style payloads), then one real post to the operator's Telegram.

---

## Self-review

- Spec coverage: B1 (T1-2), B2 (T3-4), B4 (T5-6), B6 (T7-8), B7 (T9-10), B3 (T11-12), B5 store/send/migration/`alerted_at`/`--page`/root-only/heading validation (T13-14), worker Alert port, kinds, Trouble, Your move, step 0 (T15-16), renderer, thread, skill text in ness-entry and ness-program (T17-18), review (T19).
- Interfaces consistent: `Alert`/`AlertKind`/`Notify func(Alert) error` (T15 defined, T16-18 use), `Alert.Markdown` (T17 defined, T18 implements), store functions (T13 defined, T14 implements, T15/T16 consume), `Store.HandedOff` (T12 only).
- Known interplay: B3 plus B5 mean a child that has not handed off and sits idle 30 minutes also raises a Trouble alert to the operator (specs.md: a stall report is Trouble). Accepted by the agreed design; revisit only if the field check shows noise.

## Review fixes (root rulings on review-batch-b.md, 2026-10-15)

Definitions used below. Busy(s): herdr working or blocked, or `Screen.Background`, or not yet seen by the worker. Waiting(s): s has an open item it sent (request, decision, handoff, checkpoint with Ask) not yet answered or disposed.

- F1 (blocking 1). B3 stall for a non-root child fires only when the child is idle for stall_minutes, has no live (non-returned) handoff, is not Busy, is not Waiting, and none of its descendants is Busy. A returned handoff no longer counts as handed off (nit).
- F2 (2). Your move is suppressed when the root itself is Busy (its own background shell).
- F3 (3). Your move is evaluated after the draft check: a box holding a draft, or changed within quiet_seconds, sends nothing.
- F4 (4). One cause, one alert. Your move fires only when the root holds no open obligation (otherwise the stall report covers it) and no descendant is in the B3 owes-its-brief condition (that case is a Trouble alert). Overnight case to keep: root idle, last child handed off and closed or idle with its handoff accepted, root asked only in chat → exactly one Your move.
- F5 (5). The model-stop Trouble alert is retried on a failed post like every other alert; the stop itself is not delayed by it.
- F6 (6). B3 sentence per R-WRK-050 ("… idle since T and has not handed off its brief"); the test pins the full sentence.
- F7 (7 and nits). B1 scanner: a here-doc started inside a command substitution inside double quotes is skipped as a here-doc; `$(( … ))` arithmetic is skipped; `& wait` (an `&` whose next command is `wait` on the same line) is allowed; `&` inside `${ … }` is allowed; the pgrep guard also matches `pgrep --full` and `-f` combined with other flags (`-u me -f`, `-af`).
- F8 (8). Alert text for Needs you: the first non-blank line, leading `#` and spaces stripped (checkpoint sections unchanged).
- F9 (9). The thread-key test is rewritten black-box (`package cli_test`) through an exported seam or the existing command path, by the test author, not the implementer.
- F10 (nits). Checkpoint heading detection ignores lines inside code fences; `--page` with a body that already has `Page:` is refused with a usage error naming both; the background footer pattern is checked against the footers of the other permission modes (default, accept edits, plan) using real samples where available; the over-long ness-entry line is wrapped.
- Not fixed (accepted): Telegram `*` pairing across lines (HTML stays balanced).
- F1 refinement (root, after implementation surfaced a conflict with TestStallCountsFromTheObligation): Waiting suppresses only the brief-owed path. On the obligation path only Busy suppresses; a session that owes an answer is not excused by its own open outbound items (A waiting on B while B "waits" on A is a deadlock that must be reported). TestYourMoveFiresForADeadChildAndOnlyForRoots updated by root to F4 (one Trouble, no Your move).
