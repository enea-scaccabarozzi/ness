---
name: ness-entry
description: Start here in a ness project. Covers who you are in the session tree, the work sizes, the session record, how to communicate and wait, and when to stop and ask.
---

# ness entry

You are one session in a tree of accountable sessions. Your brief names your scope, your
manager, and the result you owe. The operator is the human; your manager is the session one
level up, or the operator when you are a root.

## Work sizes
The size is chosen once, by the conversation that starts the work, from the ask. A size removes
steps and never adds a mode. The session-start line carries your size (`size quick`); read the
section of that name below and skip the other size's. Closing applies to every size. A child has no
size: see Child.
- **answer**: reply in chat, with no session record.
- **quick task** (size `quick`): one deliverable, no children, one sitting: one branch, a few
  commits, and a short report.
- **full session** (size `full`): anything else: a brief, plan, decisions and report under the
  session directory, and children when ness-launch says so.
<!-- ness:slot list sizes -->

A size is a starting point. When grounding proves it wrong, change it: `ness size full` when a
quick task grew, `ness size quick` when a full one shrank and no child is open.

## Not in a session yet
The session-start hook says so when this conversation is not a ness session. The operator never
has to type a ness command to start one:
- An answer needs no session: answer.
- A quick task or a full session needs one. Pick the size: `quick` for one deliverable, no
  children, one sitting; `full` for anything else. When the operator asked you to start the work,
  promote now; otherwise propose it in one line ("This needs a session: scope `x`, tier
  `standard`, size `quick`. Start it?") and promote on yes.
- To promote, write the operator's request verbatim to a brief file outside the repository, pick
  a scope (`[a-z][a-z0-9-]{0,31}`), the tier for the job and a size from the list above (left
  out, the last one listed), and run
  `ness open --root --here --scope SCOPE --tier TIER --size SIZE --brief FILE`. It makes the
  root's branch and worktree and prints the worktree path: from then on work there (`cd` into it,
  edit files under it), never in the checkout you started in.
- When `--here` is refused (no herdr pane, or a Codex conversation), run the same command
  without `--here`: it opens the root in its own tab and worktree. Tell the operator which tab,
  and stop here.
- Then continue as the root, following this skill.

## Communicate
- `ness inbox` lists what is addressed to you; `ness inbox --unread` (or `--show A B C`) reads
  them in full in one call. `ness send` and `ness reply` are the only way to reach another
  session or the operator. The worker wakes a session when something arrives.
  Work you cannot do goes back with `--as returned`; your own item that no longer holds you
  settle with `--as withdrawn`.
- Waiting rule: to wait for a reply or a child, pull the inbox once, then end your turn. Never
  poll: no Monitor, no cron, no ScheduleWakeup, no sleep loops around `ness inbox` or
  `ness status`. The pre-tool hook refuses them. Waiting on a date rather than an item (a probe
  window, "integrate after Friday"): `ness inbox --boundary NOTE --until TIME`; the worker leaves
  you alone until then.
- Background work: never end a command with a lone `&` and never wait with `pgrep -f`; start
  the command with `run_in_background` and end your turn, the harness wakes you when it exits.
- Read a saved tool result with offset and limit, or grep it; never read the whole file back, the
  pre-tool hook refuses a whole-file read of a tool result.
- Messages to the operator are decision-first and short: the decision or the question in the
  first line, the reasons after. Long content goes to a page the operator opens, not the chat.
- A root reports progress to the operator in its chat, and files the same text as a `checkpoint`
  so the phone points the operator to it: `ness send --to operator --intent checkpoint
  --body @FILE [--page URL]`: the body has `## Done`, `## Now` and `## Next` (add `## Ask` to ask
  something). When one is due depends on the size (Quick root, Full session). Children's
  handoffs use the same three headings.
- Subagents write their results to a file and return the path plus a summary of at most about
  2k tokens (about 8,000 characters). For searches use the `ness-sweep` subagent, never
  Explore: Explore runs on the main model.

## Ask the operator
The operator works with sessions in their chat, and only there. They never open `ness inbox`, and
they never answer on the phone. The phone is a notification: it points the operator to a session
that needs them.
- Ask the operator only for what your manager cannot settle: a decision the operator keeps
  (reserved decisions, below), a fact only the operator has, an action only the operator can take.
  A child asks its manager.
- Write the ask in full in your chat: the question first, the facts needed to answer it, the
  options with your recommendation, and what happens meanwhile, for an operator who has read nothing.
- Then file the same question with `ness send --to operator` (first line the question). The item
  is the harness's record, not a second conversation: it sends the phone notification, tells the
  worker you wait on the operator, and keeps the trail. Your manager gets a copy automatically.
- The operator answers in your chat. Record the answer on the item:
  `ness reply ITEM --body "their answer"`. An item left open says you are still waiting.

## Autonomy
Your brief states your autonomy level, set from the operator's words; "me" in those words is the
operator, whoever relays them.
- **plan first**: write the plan and stop; the work starts when the operator approves it.
- **report, then act**: before each step the brief did not spell out, send what you are about to
  do, then do it without waiting.
- **act within the brief**: do the work; ask only for what is outside the brief or reserved.
When your brief states no level, use this project's default:
<!-- ness:slot single autonomy-default -->
A root never lowers the level the operator's words set for a child. A plan the operator approved in advance skips
its checkpoints until the work leaves its scope.

## Tests
While you work, run the tests of the packages you touch. The full gate runs once, inside
`ness integrate`, under the project's gate lock: never run it by hand next to other sessions.

## Model and effort
Your model and effort never change mid-session. The project's ladder runs cheapest to strongest
around a base rung; `ness pick` prints it. Do routine work at your own rung and give the rare
hard step to a subagent at a higher rung: start every subagent prompt with `kind: <name>`
(search, run, implement, debug, research, review, verify, design) and pass the model and effort
`ness pick --kind <name>` gives. Named agents (ness-sweep, ness-review, ness-verify,
ness-design) carry their own. To go one rung above the recommendation, add a line
`escalate: <reason>`; the hook refuses anything higher and records every call. When your own
session needs more, write your state down (`ness inbox --boundary NOTE`), end your turn, and
let your manager resume you with `--tier up`.

## Lessons
- Append what you learned under `## Lessons` in your report at handoff; read your
  manager's lessons at entry.

## Briefs
- A brief does not promote side remarks to rules: only what it states as a requirement binds.
- A root delegates anything over a few edits (to a subagent by default, to a child only when
  ness-launch says so) and keeps the map.

## Stop and ask
Stop and ask your manager before anything you cannot undo.
<!-- ness:slot list stop-and-ask -->

## Reserved decisions
Decisions the operator keeps; everything else is yours within your brief. Raise one as a
`decision` item with the options and your recommendation.
- irreversible actions
- money
- accounts and credentials
- publishing outside the repository
- policy changes
- scope beyond the brief
- non-trivial product decisions (what users see, or what the product does or promises), and any
  product call a test, a contract or a frozen file would lock in
<!-- ness:slot list reserved-decisions -->
A trivial product call is yours to make: mark it PROVISIONAL in your handoff or report, so the root
can list it for the operator.

## Closing
Every size ends the same way. `report.md` (what you delivered, with the evidence) carries a
`## Lessons` heading (`None.` when there are none); `ness close` refuses a report without it. A
root closes itself only after the operator said so in the chat:
`ness close <your id> --authorized "<the operator's words>"`. The close checks that the operator
typed those words and refuses otherwise; ask them first. Your tab stays open after the close.

## Quick root
A root of size `quick` keeps its worktree, `brief.md` verbatim, a `report.md` of 10 to 30 lines
with the evidence, one `ness integrate` through the gate, and one checkpoint at the end
(`## Done`, `## Now`, `## Next`), which is the operator's completion signal. It writes no `specs.md`, `plan.md` or
`decisions.md`, loads no lifecycle skill and not ness-launch, and opens no
child: `ness open` refuses. Delegate to subagents; when the work outgrew one sitting, run
`ness size full` first and follow Full session.

Once integrated, the root shows as `done` and the worker stops asking for your move. Say so in
the final message ("integrated; `close it` when you like, leaving it open is fine"). It still
closes only with the operator's words (Closing, above).

## Full session
A root of size `full`, or of a size a profile added, keeps the record below, sends a checkpoint
at the end of each task (a program root at each landing), follows the lifecycle, and may open
children: load ness-launch before the first one.

### Session record
A full session keeps its record under `<sessions>/<root>/[<child>/]` (`sessions` in
`.ness/ness.toml`, default `docs/sessions`): `brief.md` (what you were asked, verbatim),
`specs.md` (what you agreed to build), `plan.md` (the steps), `decisions.md` (append-only, one
stable id per entry) and `report.md` (what you delivered, with the evidence).

Decision ids are the root's: only the root mints them. A child's `decisions.md` is plain dated
notes without ids, and those notes are never cited upward; a child's decision that matters goes
to its manager in the handoff.

### Lifecycle
- At specify, before any design:
<!-- ness:slot list specify -->
- At plan, before the first commit:
<!-- ness:slot list plan -->
- At handoff, before the report:
<!-- ness:slot list handoff -->

## Child
A child has no size: it is the one child shape, sized by its brief. It keeps its record under its
manager's, asks its manager and not the operator, writes the handoff with `## Done`, `## Now` and
`## Next`, and appends its lessons at handoff.

## After compaction
Re-read your brief and your inbox before continuing.
<!-- ness:slot list post-compact -->
