> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# Review: field-feedback Batch B (B1-B7)

Scope: `git diff main...HEAD`, excluding `docs/discovery` and `docs/sessions`, on
`session/field-feedback-c5a82a215386` at 792a9bc. Judged against
`docs/superpowers/plans/2026-10-15-field-batch-b.md` (root rulings R1 and R2 included),
`docs/sessions/field-feedback/specs.md` §1, `docs/spec/*.md` and the ness test and interface skills.

## Checks run

| Check | Result |
|---|---|
| `go test ./... -count=1` | pass |
| `go test -race ./internal/worker ./internal/store ./internal/cli -count=1` | pass |
| `golangci-lint run --allow-serial-runners ./internal/...`, plus `-c .golangci.tests.yml` | 0 issues, 0 issues |
| `python3 .ness/audit.py` | exit 0 |
| `go run ./cmd/ness sync --check` | exit 0 |
| Migration 7 on a copy of the research lab's `state.sqlite3` (v6, 217 items, 10 sessions) | v7; integrity ok; `foreign_key_check` clean; 217 rows, all with `alerted_at = created_at`; `ask = 0`; every v6 column identical except `pulled_at` on 5 operator fyis, which the `ness inbox` call used to trigger the migration set itself |
| Scanner probes (throwaway copy of `background.go`), worker probes (temporary test file, since deleted) | results below |

The R-ids added or amended (R-HOOK-020/021, R-SESS-080/144/146, R-CFG-061, R-WRK-050/080-083/093-095,
R-MSG-010/040/041/065-067, R-BLT-035/102/106-108) each have a spec line and a test with a
`covers:` marker. R1 (no channel means the backlog is stamped, not replayed) and R2 (a background
shell blocks a stall) are implemented and tested. The other problems are below.

## Blocking

### 1. B3 reports a child that is waiting on someone else, not stalled, and sends the operator a Trouble alert for it
`internal/worker/stalls.go:80-91`, `internal/worker/stalls.go:103` (`owesBrief`)

`owesBrief` only checks "never sent a handoff". It does not check whether the child is waiting on
someone. These children all owe their brief and sit idle by design:
- a mid-level manager waiting on its own working children;
- a child waiting on a `request` it sent to its manager;
- a child waiting on a `request` or `decision` it sent to the operator.

After 30 minutes each one gets the stall fyi to its manager, and through R-WRK-094 a 🔴 Trouble
alert to the operator.

I reproduced the first case with a temporary bench test: A working, B idle under A, C (B's child)
working. After 40 one-minute passes the result was one Trouble alert,
`has been idle since … has not handed off its brief`, and one stall fyi in A's inbox.

The child-asked-the-operator case does the most damage. The child's ask fires 🟠 Needs you at
creation. If the operator is asleep, 30 minutes later the same session also fires 🔴 Trouble.
Overnight that repeats for every child that asked something. The plan accepted the Trouble
alert for a child that is actually unfinished (Self-review, "Known interplay"). It did not
accept one for a child that is waiting on someone. No test covers either case.

Direction: when a child has no obligation, skip it if it has a busy descendant (reuse `treeBusy`)
or holds an open item it sent (`OpenToOperator`, or the same query for a recipient that is a
session).

## Should-fix

### 2. Your move fires on a root that is running its own background shell
`internal/worker/alert.go:134-149` (`yourMove`), `internal/worker/alert.go:152` (`treeBusy`)

`treeBusy` counts a descendant's `p.background`. `yourMove` never reads the root's own
`p.background`. B1 now makes every session use `run_in_background` and end its turn, so a root
with a long job (a 40-minute gate, a scoring run) is idle in herdr and its footer shows
`· 1 shell`.

Probe: root alone with `backgroundScreen`, 40 one-minute passes, gives one 🟡 Your move
("nothing appears to be running") while something visibly is running. R2's reasoning ("a pane
showing a live background shell is working") applies to the root just as much. Neither the spec
text of R-WRK-093 nor the tests cover the root's own screen.

### 3. Your move fires while the operator is typing in the root's box
`internal/worker/worker.go:226` (`alertReports` runs before `case p.draft`)

`act` calls `alertReports` before the `p.draft` / `p.quiet` guards that keep hints away from an
operator who is present.

Probe: root screen with a half-typed draft, 40 passes, gives one Your move alert, with nothing
typed into the pane. The operator, sitting at the root's tab and writing to it, gets a phone
alert telling them it is their move. At minimum a draft, and probably a focused pane, should
suppress Your move.

### 4. One idle root gets two alerts in the same pass
`internal/worker/worker.go:225-226`

Setup: root A with a child B that handed off; B's handoff is an open obligation on A; A is idle.
At 30 minutes the same pass sends:
- 🔴 Trouble `has been idle since … with 1 open obligation(s): m-…` (stall);
- 🟡 Your move `idle for 30 minutes; nothing appears to be running`.

Both come from one cause. The same doubling happens in the "dead child" case of R-WRK-093: B's
B3 stall gives a Trouble alert, and A's Your move fires too. Consider suppressing Your move when
the root's own stall Trouble alert has fired in this episode, or the other way round.

### 5. The Trouble alert for a model stop is lost if its post fails
`internal/worker/stalls.go:147` and `:155`

`stop` calls `w.once(…"stop"…)` after the tell succeeds. It then closes the tab and sets
`p.stopped = true` whether or not the post went out. After that the pane is never stopped
again: `mustStop` checks `!p.stopped`, and the closed tab makes `agent` fail. So a failed post
is never retried. This breaks R-WRK-083/094 ("a failed post is retried").

The stall and error alerts avoid this by retrying from `alertReports` on later passes. Stop
needs the same treatment: retry while `p.stopped && !p.alerted["stop"]`, or send before the
close and gate the close on it.

### 6. The B3 sentence does not match its spec wording
`internal/worker/stalls.go:80,94`

The format `"… has been idle since %s %s"` with `what = "has not handed off its brief"` produces
`has been idle since 2026-…Z has not handed off its brief`. The "and" is missing.

R-WRK-050 (`docs/spec/worker.md:175`) requires `… has been idle since T and has not handed off
its brief`. The text goes both to the manager and into the Trouble alert. The tests only check
for the substring `has not handed off`, so they cannot catch this.

### 7. The B1 scanner refuses commit and PR messages that contain an odd number of `"`
`internal/hook/background.go:54` (quote tracking) and `:58` (here-docs only detected outside quotes)

The standard agent form `git commit -m "$(cat <<'EOF' … EOF\n)"` puts the here-doc inside a
double quote. The scanner never sees the `<<` and reads the body as quoted text. A body with an
odd number of `"` before an `&` flips the quote state, and the `&` reads as backgrounding.

Probe: `git commit -m "$(cat <<'EOF'\nfeat: the 6" pipe & co\nEOF\n)"` is refused with
"backgrounding with & is refused". The same applies to `gh pr create --body "$(cat <<'EOF' …)"`
with Markdown bodies.

The agent gets a refusal for a commit that backgrounds nothing, and the message tells it to use
`run_in_background`, which is wrong advice. Recognising `$(` inside double quotes as a nested
unquoted context would fix it. Add the case to the allowed table.

### 8. A Needs you alert takes the item's literal first line, which is often useless
`internal/worker/alert.go:123`

`strings.Cut(it.Body, "\n")` takes the first line, whatever it is. `@FILE` bodies written by
agents often open with a Markdown heading (`# Decision: …`, `## Context`) or a blank line:
- a heading gives `## Context`, or a separately rendered heading block (probe: Telegram renders
  `# Decision: go?` as a bold paragraph wrapped in blank lines);
- a blank line gives an empty text line.

The same empty-line problem hits `Section(body, "Next")` and `Section(body, "Ask")` when the
heading has nothing under it. The spec says "first line", so this conforms. But the alert's job
is the question in one line: take the first non-blank line and strip its leading `#`s.

### 9. The implementer wrote a white-box test in an implementation commit
`internal/cli/worker_test.go` (added in e9c7c21 `feat(notify)`)

`TestAlertSenderPostsToTheRootThread` is `package cli` (white-box) and was written by the
implementation task. That breaks two plan rules:
- "the implementer never writes the judge";
- "black-box (`package x_test`)".

It is the only test of the `alerts:<root>` thread key in R-WRK-082. Either a test-writing pass
re-derives it black-box (for example through a testscript with a fake notify command), or the
plan's owner records the exception.

Separately, 792a9bc edited `core_text.txtar` after its implementation commit. The edit was
needed (testscript's `grep` takes no `--`) and loosened the pattern only slightly.

## Nits

- `internal/hook/background.go:58`: the arithmetic `$(( x << 1 ))` is read as a here-doc whose
  delimiter is `1`. The unterminated "body" then swallows the rest of the command:
  `echo $(( x << 1 ))\nnohup srv &` passes (a false negative). This is the mirror of the
  `$(( a & b ))` ceiling already noted in the code. Mention both in the ponytail comment.
- `internal/hook/background.go`: these patterns are refused although none leaves a dead wait:
  - `cmd & wait` and `a & b & wait`, which run in the foreground;
  - `${v#*&}`.
  This follows the plan ruling ("any lone `&`"). `cmd & wait` is a common way to parallelise and
  could be exempted.
- `internal/hook/background.go:12`: `pgrepSelf` misses `pgrep -u me -f x` (a flag with a value
  in between) and `pgrep --full x`.
- `internal/atom/claude/screen.go:99`: `shellsRe` needs a `·` before the count. It was checked
  against one footer only (bypass mode, `⏵⏵ bypass permissions on · 1 shell · …`). A footer
  where the shell count comes first (default permission mode, no mode label) is unverified and
  would read as not background.
- `internal/worker/alert.go:46`: Text, Project and Title go into the Markdown note unescaped. On
  Telegram, `*` in the text pairs with `*` in the tab label across lines.
  - Probe: Text `Is 2*3*4 ok? **x` with title `my_title*x` gives
    `<i>x\n↳ tab my_title</i>x`.
  - The HTML stays balanced, so the post does not fail.
  - `[a](b)` and backticks are rendered as markup too.
- `internal/store/checkpoint.go:13`: a `## Done` / `## Ask` heading inside a fenced code block
  still counts. An `## Ask` quoted in a code sample makes the checkpoint an obligation.
- `internal/cli/send.go` (`checkpointBody`): `--page` is silently dropped when the body already
  has a `Page:` line. The spec says the same, so this is a wording choice; a stderr note would
  help.
- `internal/store/items.go` (`HandedOff`): a handoff the manager *returned* still counts as
  handed off. A child sent back to rework that then dies idle is never stall-reported. The plan
  ruled "any disposition"; flag it for the field check.
- `profiles/core/skills/ness-entry/SKILL.md:64`: one line runs past 100 columns
  ("…what happens meanwhile. A root's every ask goes through `ness send --to operator`,");
  the wrapping is inconsistent with the rest of the file.

## Not findings (checked)

- **Duplicate or missed item alerts.** The `alerted_at` stamp survives restarts. Items replied,
  withdrawn or retired before their alert get none. A failed post leaves the item unalerted and
  is retried. With no channel the backlog is stamped (R1).
- **Trouble and Your move after a restart.** A restart may send one duplicate per episode, as
  the spec accepts.
- **Your move with descendants not yet seen.** A descendant with no pane memory counts as busy,
  so no alert. A grandchild that is working suppresses the alert.
- **B2 on real Claude Code jsonl.** Entry types in a live transcript are `assistant`, `user`
  (tool results, prompts, task notifications), `isMeta` and `isCompactSummary` users,
  `attachment`, `api-request*`, `system/*`, `queue-operation`, and timestamp-less metadata.
  The filter keeps exactly the work entries. Timestamps (`…T11:12:47.239Z`) parse as RFC3339Nano.
- **Checkpoint refusals.**
  - A child sending one, or a root sending one to a child: exit 4.
  - Missing headings: exit 2, naming only the missing ones; checked after the sender check.
  - `--page` with another intent: exit 2.
  - Idempotency covers the body with the `Page:` line appended.
- **Telegram length.** A 200-rune text plus a 150-character page URL stays one part.
- **Skill text.** The profile texts and their `ness sync` renders are consistent (`sync --check`
  exit 0). None of the replaced phrases remain in any skill: "heartbeat line", "Put the same full
  text in the chat" and "omits `--subject`".

## Second pass (fa6665d, 17cd1a0, b9364c1, fc40f45)

`go test -race -count=1 ./internal/worker ./internal/hook ./internal/store ./internal/cli`: pass.
Probes ran from temporary test files (scanner and worker bench), deleted afterwards.

First-pass findings: 2, 3, 4, 5, 6, 7 and 8, plus the nits on the scanner, pgrep, footer, fences,
`--page`, the returned handoff and the wrapped line, are fixed and reproduce as fixed. Finding 9
is rewritten black-box (worker_alerts.txtar). Finding 1 is fixed for the brief path, but see
blocking S1. The scanner probe set behaves as intended. These are allowed: `&&`, `2>&1`, `&>`,
quoted `&` and URLs, here-docs (top level, `<<-`, inside `"$(cat <<'EOF' … EOF)"` with odd `"`
and `'` in the body), `$((a & b))` (also inside `"…"`), `${v#*&}`, `a & b & wait`. These are
refused: `cmd &`, `nohup x &`, `(x &)`, `x & disown`, `f() { x & }`, and
`echo $(( x << 1 ))\nnohup srv &` (the old false negative). The deadlock "A owes B's request
while B owes A's request" still gives two Trouble alerts, at root level and one level down.

### Blocking

**S1. Any busy descendant now suppresses the obligation stall, which hides deadlocks.**
`internal/worker/stalls.go:76` (`if !ok || w.treeBusy(sess.ID)`)

`treeBusy` is applied to both paths. The F1 refinement says only the session's own Busy suppresses
on the obligation path. Before b9364c1 the obligation path ignored descendants. The probes below
each ran 120 one-minute passes, and each gave zero alerts and no stall fyi:

- A is the root. B handed off, so A holds the obligation. Sibling C shows a background shell.
- Same setup, but C is still launching (`ClearReady`). `treeBusy` counts a pane not seen yet as
  busy (`alert.go:181`), so a child whose launch never completes silences its ancestors forever.
- Deadlock: B sends A a request, then waits for the answer through a `run_in_background` poll.
  This is the pattern B1 now pushes every session toward. B is Busy, so A's overdue obligation is
  never reported. Your move is also suppressed, by `owed` and by `treeBusy`. The operator hears
  nothing.

Fix direction: on the obligation path, check only `p.background`, which is already in the guard
at `stalls.go:72`. Keep `treeBusy` for the brief path. Add a test for "busy child, root owes it an
answer".

### Should-fix

None.

### Nits

- `internal/hook/background.go:39` (`waitRe`): the `& wait` exemption matches the word anywhere
  later on the line, comments and arguments included. Two false negatives:
  - `nohup srv &  # wait for it`
  - `srv & echo now wait here`

  Anchor it to a command position, after `&`, `;` or `&&`, outside a comment.
- `internal/hook/background.go:164` (`skipHeredoc`): with two here-docs on one line
  (`cat <<A <<B`), only the first body is skipped. A `&` in B's body is refused (a false positive,
  rare).
- `internal/store/checkpoint.go:62` (`Section`) is not fence-aware, unlike `headings`. A fenced
  `## Ask` sample above the real one makes the Needs you text come from the code sample.
- `internal/worker/alert.go:133` (`firstLine`): `TrimLeft(…, "# ")` also strips a leading issue
  reference: `#123 breaks CI` becomes `123 breaks CI`. Strip only a `#`-run followed by a space.
