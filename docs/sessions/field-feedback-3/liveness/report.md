> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Report: liveness

Brief items 1-6 delivered (specs.md S1-S8), commit 90a9a85 on session/liveness-08e81663ef79.

## Delivered
- **Waits (R-WRK-099):** `store.Waits(id, now)` derives what a session waits on and until when.
  - Kinds:
    - asks to the operator;
    - requests, decisions and Ask checkpoints still open, `deferred` or `accepted`;
    - active probes;
    - a parked record.
  - Migration 12 adds `parked_at`, `parked_until` and `parked_note`. The compaction does not clear them.
  - A park with an until-time holds until that time. A park without one ends when something arrives
    (the operator's items included) or at the first real turn after the compaction.
  - `ness inbox --boundary NOTE --until TIME` sets the until-time. Only the flag parsing was added to
    inbox.go, allowed by m-tkyftsgl.
- **Stall rule (R-WRK-050):** a session that waits on anything gets no stall report, whether it owes
  its brief or holds obligations. Screen signals are unchanged.
- **Routing (R-WRK-094, R-WRK-081):**
  - A stall is told to the manager only. The phone gets one Trouble alert on the second miss.
  - A blocked prompt is told to the manager at 2 min. The phone gets alerts at 10 min, 1 h, 4 h, then
    every 4 h, with no burst after a channel outage.
- **Your move (R-WRK-093):** an ask covers the root only when it was sent in the root's latest turn.
  - Obligations cover it only when the root waits on nothing.
  - A descendant that owes its brief covers it only when that descendant waits on nothing.
- **Phone text (R-WRK-080, R-WRK-082):** the alert carries the whole Ask section, or the first Next
  item. The cap is 400 runes, cut at a sentence end in the second half, else at a space, with "…".
- **Child cap (R-SESS-152):** a child parked until a future time holds no slot. The refusal lists
  each slot holder with its manager and its idle time.
- **Status:** each row gets `on …` with its waits, and the JSON gets `waits`.

## Evidence
- **Tests:** `go test -count=1 ./internal/...` exits 0. `go vet`, `golangci-lint` (0 issues) and
  `python3 .ness/audit.py` are clean.
- **Who wrote what:**
  - A separate subagent wrote the acceptance tests (the judge).
  - Another implemented the change.
  - An opus/high review found 4 high-severity holes, all fixed in round 2. Each fix has a test that
    fails without it.
- **Field replay:** the real worker, a real sqlite store in a temp dir, herdr faked, one pass a minute.

  | Case | Field (before) | Now |
  |---|---|---|
  | Child whose request was accepted, idle 4 h | a stall every 30 min | nothing |
  | Child with its own probe window, idle 4 h | a stall every 30 min (wave5: 6) | nothing |
  | Child parked until +3 h, idle 4 h | a stall every 30 min | 1 fyi after the until-time, 1 phone alert on the second miss |
  | Child idle 4 h, waiting on nothing (control) | stalls | 1 fyi, 1 phone alert |
  | Pane blocked on a prompt 17.6 h | 1 alert, then silence | manager at 2 min; phone at 11 m, 1 h, 4 h, 8 h, 12 h, 16 h |

- **Not run:** the replay used a faked herdr. A live herdr worker with real child sessions was not
  run, because it costs real Claude sessions.

## Left as is (known ceilings)
- **Mutual waits:** two sessions that each wait on the other's item are both silent. `status` shows both waits.
- **Worker restart:** after a restart the worker does not know when the current turn began, so any
  open ask covers Your move until the root works again.
- **Second miss:** if something arrives while the session cannot act on it (a draft in its box), the
  second-miss alert of that episode is lost.
- **Next item:** the alert keeps the `- ` list marker. `inbox_until.txtar` does not pin which exit
  code each refusal returns.
- **Child cap:** idle time comes from the transcript, so a child with no transcript shows `idle -`.

## Lessons
- Rules derived from "waits" interact. Silencing one alert (a stall) can also silence the alert that
  assumed the first one would speak (Your move covered by "the stall covers it"). Review every
  "X covers Y" clause whenever X gets an exception.
- A boundary set during a turn is followed by more assistant text from that same turn. "A turn after
  parked_at" ends every park at once. Anchor on the compaction, not on the record time.
- The operator is NULL in the store. Every `<>` against a recipient or sender silently drops the
  operator. Use `IS NOT`.
- A separate judge plus a strong review paid off: the review's 4 high findings were all paths that
  the judge's tests, written from my spec, made pass while the spec itself had holes.
