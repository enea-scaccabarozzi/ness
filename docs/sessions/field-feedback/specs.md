> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# field-feedback: what this session builds

The session turns the research lab field analysis (`docs/discovery/findings/field-research/`) into
designs, one at a time, each agreed with the operator before its plan. Order: the fixes that help
every size of work first.

## 1. Operator alerts (agreed 2026-10-15)

### Goal

The operator learns from the phone, within minutes, that a session needs them, that a checkpoint
landed, or that something is in trouble. An alert is short and says where to go: the session's
herdr tab and, when there is one, a page link. The content stays in the session and on the page;
the operator reads it through herdr web or Claude Code mobile (operator, 2026-10-15: "Mainly A,
but may include artifact links … i only need alerts mainly").

Evidence: the root's overnight ask made only in chat cost 9 h (10-14 22:57 to 10-15 07:59);
fyi items such as landings and the worker's own stall report never reached the phone; the
operator repeatedly returned to an unread chat and asked for done / now / next.

### Events

| Kind | Fires when | Source |
|---|---|---|
| Needs you | an open `request`, `decision` or `handoff` to the operator is created, by any session; or a `checkpoint` with an `## Ask` section | items |
| Checkpoint | a root sends a `checkpoint` to the operator, without `## Ask` | items |
| Trouble | a pane blocked on a prompt for 2 min (today's R-WRK-081); a stall report (R-WRK-050); a screen error report (R-WRK-051); a stop for an unavailable model (R-WRK-062) | the worker's detectors |
| Your move | a root has been idle for `stall_minutes`, holds no open item to the operator, and none of its child sessions is working or blocked | the worker pass |

- Needs you fires at creation, not on idle: this replaces R-WRK-080's idle rule. A question the
  session settles itself costs one alert; the session withdraws it with `--as withdrawn`.
- Your move fires once per root idle episode and resets when the root works again. It is the
  deterministic fix for the ask made in chat only: an idle root with nothing running below it
  waits on the operator, whatever it wrote. It also catches a dead child (in the census the root
  waited 10 h on C1 and L0a while both sat dead).
- Known false positive: a child running a long `run_in_background` job may read as idle in herdr;
  Your move then fires during legitimate compute, at most once per root idle episode. Plan step 0
  checks whether herdr's status or the pane screen shows a live background task. If it does, a
  child with one counts as working. If not, the alert keeps the wording "nothing appears to be
  running".

### The checkpoint item

- `ness send --to operator --intent checkpoint --body @FILE [--page URL]`.
- The body MUST contain the headings `## Done`, `## Now` and `## Next`; `send` refuses a body
  without any of them and names the missing ones.
- An `## Ask` heading makes the item an obligation (it blocks close like a `decision`) and its
  alert kind Needs you. Without it the checkpoint carries no obligation, like an `fyi`.
- `--page URL` appends the line `Page: URL` to the body; the alert links it. Without the flag, a
  `Page:` line already in the body counts the same.
- Only a root sends a checkpoint, only to the operator; `send` refuses other senders and
  recipients.
- When: a quick root, once at the end, with what it delivered; a program root, at each wave or
  rung landing, at a plan change, and at a change to how results are scored.
- Children's handoffs use the same three headings through skill text only, not enforced.

### Alert format

Every alert goes through the existing `notify.Poster` and Markdown renderer (Telegram HTML,
Slack blocks); no new channel code.

```
🟠 Needs you · the research lab / study-protocol
Go on L0b? Band study over the three income cells, 2 children.
↳ tab study-protocol · page https://claude.ai/…
```

- Title line, bold: 🟠 Needs you, 🟢 Checkpoint, 🔴 Trouble, 🟡 Your move; then
  `<project> / <session title>`.
- Text line: the item's first line (for a checkpoint, the first line under `## Next`; for Needs
  you from a checkpoint, the first line under `## Ask`), at most 200 runes; for Trouble and Your
  move, the worker's own sentence.
- Location line: the herdr tab name, then `page URL` when the item has one.
- Thread: one per root, keyed by the root's id, so a program's alerts group together.
- The worker's port changes from `Notify func(line string) error` to
  `Notify func(Alert) error`; `Alert` (in `worker`) carries kind, project, title, tab, text and
  page. The composition root (`cli/worker.go` `wireNotify`) renders it to Markdown and posts it
  with the thread key.

### Delivery state

- Items gain `alerted_at`. Each pass, the worker alerts every open operator item whose
  `alerted_at` is null and stamps it on success; a restart neither loses nor repeats an item
  alert. An item disposed before its alert went out gets none.
- A failed post is logged once and retried next pass (R-WRK-083 unchanged).
- Trouble and Your move episodes stay in memory as today; a worker restart mid-episode may send
  one duplicate (accepted).

### Skill text

- ness-entry, "Ask the operator": a root's every ask goes through `ness send --to operator`,
  because the operator reads alerts, not the chat; the chat copy is optional. One line: a root
  reports progress to the operator with a `checkpoint`.
- ness-program: the "heartbeat line at each wave event" becomes "send a checkpoint at each
  landing, plan change or scoring change"; the duplicated cadence prose goes.
- ness-notify: unchanged; `ness post` stays the manual tool.

### Requirement changes (docs/spec)

- R-MSG-010: `--intent` also accepts `checkpoint`; `--page URL` added.
- R-MSG-040/041: a `checkpoint` with `## Ask` creates an obligation; without, none.
- New R-MSG-065: checkpoint body headings refused when missing; R-MSG-066: only a root to the
  operator.
- R-WRK-080: replaced: one alert per operator item, at creation, recorded in `alerted_at`.
- R-WRK-082: replaced by the alert format above.
- New R-WRK-093: Your move; R-WRK-094: Trouble kinds reach the operator.

### Out of scope

Per-kind mute and quiet hours; alerts for child events (launch, landing); full content on the
phone.

### Tests

Acceptance tests are written by a separate task from the implementation (ness-test-strategy),
black-box, with hand-written fakes.

- Store: `alerted_at` selection; a restart does not repeat an alert; a disposed item gets none.
- Send: a checkpoint missing any heading is refused naming it; `## Ask` makes an obligation;
  `--page` stored; a child's checkpoint and a checkpoint to a session are refused.
- Worker (stepped clock or synctest): each kind fires exactly once; Your move fires only with all
  three conditions and not while a child is working or blocked; no channel, nothing sent.
- Renderer: golden output per kind for Telegram and Slack.
- Field check before merge: a dry run of each kind against the research lab's channel configuration,
  then one real post to the operator's Telegram after the operator agrees.
