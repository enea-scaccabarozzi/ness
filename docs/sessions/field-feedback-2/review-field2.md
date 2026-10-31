> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Review: field-feedback-2 (a0a3c2d..HEAD)

Scope: 7e2ee36 (probe labels), 2f2d543, c9ced1f, 975af89, ed1db6c (alerts). `go test ./internal/... ./cmd/...`
passes. Findings ranked by severity.

## High

### H1. Launch and notification text anywhere in a user record counts, quoted text included
`internal/atom/claude/activity.go:127-133` (and `notified`, L139-151). `launchRes`, `notificationRe` and
`stillRunningRe` run over the whole raw line of every `user` record. A tool result that only *quotes*
the text counts too: a Read of `internal/atom/claude/testdata/transcript-background.jsonl` or
`activity_test.go`, a `grep -rn "Command running in background"`, a `cat` of a transcript (field analysis
sessions do exactly this), or a prompt that pastes one. Each quoted launch, or quoted "is waiting on its
own background work" notice (fixture line 7), adds a task id that no notification will ever clear. The
session is then excused from every stall, Your move and owed-brief report for `pendingMax` (12 h). In
this repo, where agents work on this code, the false silence is likely. The `api-request-blob` guard
does not cover it, because quoting happens inside ordinary `user` records.
Fix: read launches from the structured `toolUseResult` field, not from text. The fixture already shows
`toolUseResult.backgroundTaskId` for Bash. Agent launches carry `agentId`/`status: async_launched` and
SendMessage carries `resumedAgentId`. Count a notification only when it is the whole content of a
`user` string message or a `queue-operation` `content`, never when it sits inside a `tool_result`.
Add a fixture line: a Read tool_result that quotes a launch. It must not be pending.

### H2. Killed or stopped tasks may never leave the pending set
`internal/atom/claude/activity.go:139-151`. Only `<task-notification>` removes an id. Nothing handles
`KillShell`/`TaskStop` (result "Successfully killed shell: <id>", `toolUseResult.shell_id`/`task_id`).
If the client sends no notification for a killed task (not shown by any fixture, and the plan does not
claim one), a killed shell excuses the session for 12 h. A client exit or restart kills its background
shells and agents, and their notifications are lost the same way.
Fix: treat a KillShell/TaskStop tool result as a completion of its id. Capture a real transcript of
a kill and add it to the fixture. Also drop pending ids launched before the client's last start.

### H3. The 12 h cap is too long for the failure it guards
`internal/atom/claude/activity.go:18`. Per-run age is refreshed by a resume and by a still-running notice
(`at` is re-set), so the cap bounds one run, not a chain of runs. The longest single run in the evidence
is about 1 h 50 (a823a93f9d3f0e964). Any H1/H2 false pending, or a notification lost to a restart,
silences a real stall for a whole night: the window the overnight fix exists for.
Fix: `pendingMax = 3 * time.Hour` (still well over the observed runs). Keep the ponytail note.

## Medium

### M1. A stale `ClientSession` freezes the episode and the pending set
`internal/cli/worker.go:187` builds the path from the stored `sess.ClientSession`. `agent()`
(`internal/worker/worker.go:293-295`) accepts a pane whose herdr *name* matches even when
`st.ClientSession` differs (after `/clear`, or a resume that forks a new id). The old file still exists,
so `tracked` is true but `Turn` never advances. Since ed1db6c only a newer Turn ends an episode
(`worker.go` `working()`), so after one stall that session never gets another report. A stale pending id
in the old file also excuses it.
Fix: pass the live id `st.ClientSession` (when not empty) to `Activity`. Otherwise, when it differs
from the stored one, return `ok=false` so the screen rules apply.

### M2. A limit read again after a worker restart alerts again and mutes reports for good
`internal/worker/activity.go:43-50, 105-108`. On restart `Transcripts` re-reads the whole file and
`alerted` is empty. A session whose last record is a days-old limit error sends a new "is out of usage"
Trouble alert on every restart. `report()` then returns early forever (it checks `limitText`, not the
hold), so stall and error reports for that pane stay muted until a real turn. That can be never for a
dormant session with nothing to hint.
Fix: in `noteActivity`, alert only when `now.Sub(act.LimitAt) < limitHold` (or a day). In `report()`,
suppress only while `p.limited(now)`, or while a retry hint is in flight, not for as long as
`limitText != ""`.

### M3. `LastInbound` counts probe wakes and self-disposals, so a probe owner may never stall
`internal/store/items.go:170-176`. A wake is a request from the owner to itself (`SendWake`).
`wake: always` (R-PRB-035) or `sample:` on a short `every` creates a fresh item, then a disposal, every run.
The owner's clock restarts each time, and with a period under 30 min a stuck owner is never reported. The
session disposing its own items also counts as "inbound". Smaller cost: the `sender = ?` branch has no
index and runs twice per idle pane per 5 s pass.
Fix: `WHERE recipient = ? AND sender <> recipient` and `WHERE sender = ? AND recipient <> sender`
(or exclude `probe IS NOT NULL`). Add an index on `items(sender, disposed_at)` if the table grows.

### M4. `noAskRe` swallows a real ask after "Nothing blocking"
`internal/store/checkpoint.go:36-37`. The `\b.*` tail matches "Nothing blocking, but please approve the
migration", which then reads as no ask and becomes a Checkpoint, not Needs you. That is a missed alert,
the costly direction.
Fix: anchor the long forms to the end of the line, for example
`nothing (?:is )?(?:blocking|asked(?: of you)?(?: today)?|needed|to ask)\W*$`, and add the "but ..."
case as a test.

## Low

### L1. The transcript reader keeps every file forever and reads the first file in the loop
`internal/atom/claude/activity.go:69-75, 118-120`. `files` is never pruned, so closed sessions and
superseded ids stay; this is small but unbounded. The first read of a large transcript (hundreds of MB
for long roots) runs `json.Unmarshal` on every full line synchronously inside the 5 s pass, and a
worker restart repeats it for every open session.
Fix: drop entries for paths not seen in the pass. Before unmarshalling, skip lines that contain none
of `"type":"user"`, `"type":"assistant"`, `queue-operation` or `attachment`. A file truncated and regrown
past `off` between two polls is missed. That is acceptable, since Claude Code only appends, but say so
in the godoc.

### L2. Task-id class differs between launch and notification
`internal/atom/claude/activity.go:24-30`. `[0-9a-z]+` stops at `_`, `-` or capitals. An id like
`bash_1` (older clients) is stored as `bash` and the notification's `<task-id>bash_1</task-id>` does
not match, so it stays pending for the full cap. Fix: `[\w-]+` in all four patterns (moot if H1 moves
to structured fields).

### L3. Blocked check reads the screen on every pass while blocked
`internal/worker/operator.go:26`. After 2 min, `compacting()` calls `ReadANSI` every 5 s for as long as
herdr says blocked and the compaction line shows. This is cheap, but it bypasses `readable()`. Fix:
rate-limit with `p.nextRead`, or accept it with a ponytail note.

### L4. Spec id drift
`internal/store/probes.go:130` cites R-PRB-085 for `LiveLabels`, which is R-PRB-088 (stop
ambiguity). `docs/spec/probes.md:158` cites R-PRB-085 for `NESS_PROBE_LABEL`, which the code
attributes to R-PRB-087 (`internal/probe/floor.go:43`). The plan says "R-PRB-085..087" but 088 was
added. Fix: align the citations.

### L5. Missing tests
No test covers H1 (quoted launch), H2 (kill), M1 (id change), M2 (old limit on restart), M4 ("but"
case), or the probe-owner wake case in M3. The worker tests use a fake `Activity`, which is fine. The
reader tests use real-shaped fixtures, which is good. No sleeps found.

## Checked and fine
- Migration 8: `ALTER ... ADD COLUMN ... DEFAULT ''` keeps old rows. The `probes_live` index that
  `DROP INDEX` removes is created at `migrate.go:118`, and no later migration rebuilds `probes`. The migration runs in a transaction.
- StartProbe adopt and merge, `moveProbes` merge by (path, label), and `stopTarget` ambiguity
  (exact label first, a single live label when none is given, otherwise refuse with the list) are
  correct. Thread key `probe:<path>` is unchanged for an empty label.
- Partial last line waits for its newline, and a shorter file resets the scan (tested).
- Episode reset on a new assistant turn: a compaction summary is skipped and API errors are not
  turns, so no extra alert after a compaction blip. Re-alerting depends on M1.
- Limit retry: one hint after `limitHold`, which refreshes `limitAt` when the error returns, with one
  alert per episode (tested).
- `LastInbound` cannot cause the opposite race (an alert sent too early). It only pushes the clock
  later; the cost is M3.
