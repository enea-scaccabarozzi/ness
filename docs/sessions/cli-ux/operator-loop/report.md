> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# operator-loop: report

Session s-1eb79f19a3bf (child of s-4ad0a273d838, scope cli-ux, frontier). Delivered the operator
loop of rulings CX-16..CX-19: the worker tells the operator when a session needs them, any session
can ask the operator, briefs carry an autonomy level, and a root can close itself with the
operator's words.

## What was built

| Ruling | Delivered | Spec ids |
|---|---|---|
| CX-16 | `[notify] operator = NAME` names the channel; the worker posts one line per episode when a session is idle with an open request/decision/handoff to the operator, or its pane is blocked on a prompt for 2 minutes; `ness doctor` has an `operator-notify` check (warn unset, ok naming channel and kind, fail when the token is not exported). No channel: nothing sent, nothing logged. | R-CFG-055, R-WRK-080..083, R-DOC-024 |
| CX-17 | `ness send --to operator` from any session; a child's item sends its manager an fyi copy `[copy of <id> to operator]`; the asker records the chat answer with `ness reply` and gets no reply item back; the sender's manager may answer too; a sibling root may not. | R-MSG-017/023/024 amended, R-MSG-051, R-MSG-052 |
| CX-18 | ness-entry: "Ask the operator" (when, what the item contains, same text in chat, record with reply), "Autonomy" (plan first; report, then act; act within the brief; "me" stays the operator; never lowered by a root; approved plans skip checkpoints), reserved decisions default list before the project slot, self-close rule under Lifecycle. ness-launch brief contents include the autonomy level. Cheat sheet lists `--to operator` and `--authorized`. | R-BLT-102..105 |
| CX-19 | `ness close <own root> --authorized "<words>"`: accepted when the words appear (case-insensitive, whitespace folded) in a user line the operator typed in the session's transcript; tool results, meta/system/sidechain/compaction lines and ness-typed texts (hint, `/compact`, `/rename`, resume, launch prompt) never count; no transcript or a codex client → refused telling the agent to ask. Closing a root never closes its tab, launched or adopted. | R-SESS-060/065 amended, R-SESS-121, R-SESS-122 |

Commits, oldest first: `55ad096` record, `0122ef7` CX-17, `d8f295a` config+doctor, `0d6bcf1`
worker, `cc1493d` close, `3adcecf` core text, then the review fixes with this report.

## Turning Telegram on (the operator does this; nothing live was touched here)

In `.ness/ness.toml`:

```toml
features = ["notify"]            # add to the existing list if one exists

[notify]
operator = "phone"

[notify.channels.phone]
kind = "telegram"
token_env = "NESS_TELEGRAM_TOKEN"   # the bot token, exported in the worker's environment
chat_id = "123456789"               # the operator's chat with the bot
```

Then export the token where the worker runs (the systemd user unit's environment), restart the
worker (`ness worker install` rewrites the unit; `systemctl --user restart ness-<project>`), and
run `ness doctor`: `ok  operator-notify  phone (telegram)` means the next idle-with-a-question or
2-minute prompt sends a line. Posts fail safely: a missing token is logged once and retried every
pass, the episode is not spent.

## Rulings made while building (decisions.md OL-1..OL-6 plus)

- The notification names the newest open item to the operator when several are open.
- Blocked threshold is `>= 2 min` (the Verify line of R-WRK-081: 2 min → one line).
- Transcript lines without `promptSource`/`origin` (older Claude Code) count as typed; lines with
  any other `promptSource` or `origin.kind` do not.
- A refused landed check on a root says "nothing was touched" (its tab was never closed); the
  `ness open --resume` hint stays for children, whose tab was.
- `closeRight`: `--authorized` from anyone but the root itself on itself is refused before any
  other check, with "ask your manager".

## Evidence

- Tests written from the specs before the code, per OL-6: `send_routes`, `reply`,
  `close_authorized` (new), `doctor`, `config_errors`, `core_text`, `close*`, `probe_owner`,
  `hook_session_start` testscripts; `internal/worker/operator_test.go` (four bench tests);
  `internal/atom/claude/said_test.go` (one case per transcript line kind);
  `TestOperatorChannel` in config.
- Coverage gate: every new or amended id has a `covers:` marker; `make validate` green at the
  handoff commit (see the integrate log).
- Compliance audit (`python3 .ness/audit.py`): no findings.
- ness-review at handoff: findings and their disposition below.

## Review findings (ness-review at handoff)

| Finding | Severity | Disposition |
|---|---|---|
| H1 substring match accepted a blank or a common word as the operator's words | high | fixed: at least two whole consecutive words (OL-12; `said_test`, `close_authorized`) |
| H2 lines without `promptSource`/`origin` let slash commands and command output through | high | fixed: typed-by-human marks required, texts starting with `<` rejected (OL-12) |
| M1 an agent can forge a transcript line or type into its own pane | medium | accepted as the design's limit, recorded (OL-11) |
| M2 the operator closing a launched root no longer stops its agent first; a commit between the landed check and the teardown is lost | medium | open, for the manager (OL-13): a `working`-pane guard belongs with the sibling's close work |
| M3 a usage-limited or unreadable pane was never notified | medium | fixed: the notification runs before the screen read (`TestLimitedPaneStillTellsTheOperator`) |
| L1 a transient `unknown` status or a worker restart resends the line | low | accepted: one extra line, same as the stall report (R-WRK-050 note) |
| L2 a github operator channel validated but could never deliver | low | fixed: refused by config |
| L3 a failed manager copy was never retried (repeat said "already sent") | low | fixed: the copy is attempted on a repeat too, idempotent |
| L4 tests that passed a wrong implementation (copy id, manager's reply reaching the child) | low | fixed where cheap; bench titles all "x" left |
| L5 notifications ignore operator focus | low | accepted: the line is cheap and the spec is silent |
| L6 "never below your own" misread CX-18 | low | fixed: "never lowers the level the operator's words set for a child" |

## For the manager at integration

- Expect conflicts with the delivery sibling (s-62214e9d3763) in `internal/worker/worker.go`
  (`visit`: the `noteBlocked` call before the `!idle` return, `act`: `notifyAsked` after
  `reportStall`, the three pane fields) and `internal/cli/close.go` (`closeRight`,
  `stopThenLanded`, the `--authorized` flag). Both are additive; keep both sides.
- Four existing scripts that closed a root now expect its tab to stay (`close`, `close_gone`,
  `close_landed`, `close_pr_late`, `probe_owner`); the sibling's close changes must keep that.

## Lessons

- The coverage gate makes "spec first" mean "spec amendments commit with their covering tests":
  the six spec files could not land in one commit before the code.
- testscript's `grep` is a regexp: parentheses in skill text need escaping.
- The shadow and sloppyReassign linters pull against each other; the way out is a smaller
  function, not a different assignment.
