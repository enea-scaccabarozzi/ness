> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Specs: liveness

What this child builds, from brief items 1-6. Rule ids are the spec's (docs/spec/*.md); each
behaviour change edits the rule text in the same commit.

## S1 Waits (store, R-WRK-099 new)
`store.Waits(id string, now time.Time) ([]Wait, error)`: what session id waits on, until when.
`Wait{Kind, Ref, Note string; Until time.Time}` (Until zero: open-ended). Kinds:
- `item`: an item id sent (sender = id, recipient another) with intent request, decision or
  an Ask checkpoint (not a handoff: delivered work, and a follow-up obligation must still stall), not retired, disposition NULL, `deferred` or `accepted` (accepted = "I'm on
  it": the answer is still owed). Ref the item id; Note `to <recipient>`; `ask` instead of `item`
  when the recipient is the operator.
- `probe`: an active probe owned by id. Ref the probe id; Note its path (and label).
- `parked`: the session's boundary record (S2) while it holds: with an until-time, until that time;
  without one, until something reaches the session after it parked (LastInbound > parked_at, the
  operator's items and answers included), or until its first real assistant turn after the
  compaction consumed the boundary (the worker unparks it, `Store.Unpark`; a client without an
  activity reading ends it only by an arrival).
Order: asks, items, probes, parked. One query per kind; no worker memory.

## S2 Parked record (store, migration 12, R-MSG-034 changed)
`ness inbox --boundary NOTE [--until TIME]` records the compaction request (as today, cleared by
the compaction, R-WRK-032) and a parked record `parked_at`, `parked_note`, `parked_until` that the
compaction does not clear. A new boundary replaces it. `--until` takes RFC3339, `YYYY-MM-DD` or
`YYYY-MM-DDTHH:MM` (UTC); refused in the past or without `--boundary`. (The manager allowed the
flag-parsing edit to inbox.go, m-tkyftsgl.)

## S3 Stall rule (worker, R-WRK-050 changed)
A session that waits (S1 non-empty) gets no stall report of either kind: owed brief or obligations.
The "Waiting excuses only the owed brief" sentence goes. Screen signals (R-WRK-051 error, R-WRK-081
blocked, R-WRK-062 stop, pane gone) are unchanged. Known ceiling: two sessions that each wait on the
other's item are both silent (the field's mutual-wait lockstep); status shows both waits.

## S4 Routing (worker, R-WRK-094 and R-WRK-081 changed)
- A stall report is an fyi to the manager only (as today). The phone gets one Trouble alert only on
  the second miss: the same stall episode is still open `worker.stall_minutes` after the report and
  nothing reached the session since (LastInbound before the report). Same for a root (its fyi goes
  to the operator's inbox, not the phone).
- Blocked prompt: after 2 minutes an fyi to the manager at once (one per episode); the operator gets
  a Trouble alert at 10 min, 1 h, 4 h after the block began, then every 4 h, until the status
  leaves `blocked`. After an alert the next one is the first schedule point after now: steps missed
  in a channel outage are not sent in a burst. The compaction exception stays.
- Error and stop alerts are unchanged.

## S5 Your move (worker, R-WRK-093 changed)
An open ask to the operator covers Your move only while it is the newest unanswered ask and was sent
in the root's latest turn (created at or after `busyStart`): the root went idle because it asked.
Once the root works again and idles with the same ask open, Your move fires (one per episode).
Other waits (S1 item/probe/parked) also cover Your move: the root is not idle on the operator.
An obligation covers Your move only when the root waits on nothing at all (its stall report can
fire; an old ask silences that report, so Your move speaks). A descendant that owes its brief
covers it only when that descendant waits on nothing (its own stall report can fire).

## S6 Phone checkpoint text (worker, R-WRK-080 and R-WRK-082 changed)
- Ask checkpoint: the whole `## Ask` section (all lines, joined); Checkpoint: the first item under
  `## Next` (its first line and continuation lines up to the next item or blank line).
- Cap 400 runes, cut at the last sentence end (`. `, `? `, `! `) in the second half of the cap,
  else the last space, with `…` appended when cut. A single word longer than the cap is cut at the
  cap. Page link as today.

## S7 Child cap (cli/open.go, R-SESS-152 changed)
- A child whose parked record has an until-time in the future does not hold a slot.
- The refusal lists the children holding slots: `<id> <scope> (manager <id>) idle <age>`, idle from
  the transcript's last activity as status computes it (`-` when unknown).

## S8 Status (cli/status.go, R-SESS-080 area)
Each row adds `on <kinds+refs>` from S1 (e.g. `on ask m-x, probe p-y, parked until 10-27 18:00`);
JSON gets `waits: [{kind, ref, note, until}]`.
