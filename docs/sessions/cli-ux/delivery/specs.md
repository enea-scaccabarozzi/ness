> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# delivery: what this session builds

Fixes F1, F2, F4, F5 and F6 of field run 1 (`../infra-lab-feedback.md`), approved by the operator
(CX-14, CX-15). Each fix amends or adds one requirement in `docs/spec/` and has a test that fails
without it.

## F1: a child's tab opens in its parent's workspace

**R-SESS-030 (amended)** The launch MUST open a herdr tab for the session (titled from `--title`
when given) in its manager's herdr workspace: the workspace of the manager's tab. A session whose
manager has no tab (a root, or a manager adopted without one) opens in the workspace of the
process that runs `ness open` (`HERDR_WORKSPACE_ID`), else wherever herdr puts a new tab. The
same holds for `ness open --resume`.
Verify: contract(fake+real) — tab exists with that title; testscript — a child of a root whose
tab is `w7:t1` gets `tab create --workspace w7`; a root opened with `HERDR_WORKSPACE_ID=w9` gets
`--workspace w9`; without it, no `--workspace`.

Seams:
- `session.TabSpec` gains `Near string`: the id of a tab the new one opens beside (same
  workspace); "" means the runtime's default place.
- `herdr.Runtime` gains `Workspace string`: the workspace used when `Near` is "" (the cli sets it
  from `HERDR_WORKSPACE_ID`). herdr ids are `<workspace>:<tab>`; `OpenTab` passes
  `--workspace <prefix of Near>` or `--workspace <Workspace>`, else no flag.
- The fake herdr gives a tab created with `--workspace W` the ids `W:tN` / `W:pN` (default `wF`)
  and answers `workspace_id` accordingly.

## F2 / CX-15: delivery holds only on the input box

**R-WRK-017 (amended)** Focus and scrolling MUST NOT hold a delivery: a pane the operator has
focused gets its hint, compaction, resume or rename like any other idle pane. Only the input box
holds (R-WRK-014, R-WRK-018). (The stop of a session whose model is unavailable still spares a
focused pane: that closes a tab, it delivers nothing.)
Verify: unit — fake runtime reports focused, box empty and unchanged → hint typed in the same
pass; no "pane is focused" log line.

**R-WRK-018 (amended)** The quiet window MUST be keyed on the input box only: a pane is quiet
while its box holds non-placeholder text, or for `worker.quiet_seconds` (default 60 s) after the
box's content last changed (a draft typed, edited or cleared). Focus, scrolling, the session's
own transcript writes and tool calls MUST NOT start or extend it.
Verify: unit (synctest) — box empty, focused, never changed → hint in the first idle pass; a
draft seen at t=0 and cleared at t=5 s → no non-urgent hint before t=65 s, hint at the first
pass after.

**R-WRK-019 (amended)** When the pane is idle, the box empty and unchanged within the quiet
window, the worker MUST deliver in the same pass.

**R-MSG-042 (coherent, wording only)** "operator typed 10 s ago" reads "the box changed 10 s
ago": an urgent item skips the quiet window, never a non-empty box.

## F4: the first prompt goes in after the client is ready

**R-SESS-025 (amended)** The client MUST be started without a prompt argument. Once herdr reports
it ready for input, the launch MUST submit the one-line prompt
`You are ness session <ID> (<scope>). Read <path>/brief.md, then load ness-entry.` through the
runtime's prompt call (`herdr agent prompt <pane> <text>`), exactly once per launch step. While
the agent is `blocked` (a trust or approval dialog) the prompt waits; still blocked at
`launch.ready_timeout`, the launch stalls (R-SESS-033) and `ness open --resume` submits it.
Verify: contract(fake) — agent start args carry no prompt; one prompt submission to the
session's pane with exactly that text; a blocked agent gets no submission and the launch exits
4 as stalled; `--resume` after unblocking submits it.

**R-SESS-026** unchanged: the brief is never typed or pasted; `pane send-text`/`pane run` are
not used by a launch (the prompt is the one `agent prompt`).

Seams:
- `session.Runtime` gains `Prompt(pane, text string) error` (herdr: `agent prompt PANE TEXT`,
  no `--wait`).
- `session.LaunchSpec.Prompt` is removed; clients' `Args` end with their flags.
- The fake herdr records `agent prompt` in `Prompted [][2]string` (pane, text), refuses it with
  `agent_blocked` when the pane's agent is blocked and `agent_not_found` when there is none;
  testscript `fake herdr prompted N` (negatable) and `fake herdr prompt REGEX` (last prompt text).

## F5: the cheat sheet at every start

**R-HOOK-006 (amended)** `session-start` for a bound session MUST print a cheat sheet of at most
20 lines with one-line forms of `open`, `inbox`, `send`, `reply`, `integrate`, `close` and
`probe`, at every start (startup, resume, clear, compact). After a compaction it also says to
re-read the brief and the inbox.
Verify: testscript — startup payload → identity plus the cheat sheet; compact payload →
identity, the re-read line, the cheat sheet and the post-compact files; golden files.

## F6: closing a child settles what it sent its manager

**R-SESS-123 (new)** Closing a child MUST dispose every open item it sent to its manager as
`accepted` with the answer `closed with the child`, without a reply item, so the manager owes
nothing for a child it closed. Items to anyone else are untouched, and a root's items to the operator stay open (the operator queue is not a manager's inbox).
Verify: testscript — a child's open handoff, decision and fyi to its manager → after `ness close`
the manager's inbox and obligations are empty and the items show `accepted`; an item the child
sent elsewhere stays open.

Seam: `store.(*Store).SettleFrom(sender, recipient string, now time.Time) (int, error)`.

**ness-launch skill**: a handoff needs no reply; closing the child settles it.
