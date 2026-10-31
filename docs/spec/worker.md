> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Worker (WRK)

Package: `internal/worker`, `internal/atom/systemd`, cli `worker`. Sources: §3 (messages,
release), §4 (systemd, probe owners, model unavailable), §5 (`ness worker`), §6, §13, §14,
migration hot-fixes F1–F4 (fyi wake, usage-limit formats, quiet window, logging and zombie
items), P3 (placeholder), D10.

All timing requirements are verified with `testing/synctest` and the fake runtime/client unless
stated otherwise.

## Process and supervisor

- **R-WRK-001** There MUST be at most one worker process per project; a second `ness worker run`
  for the same project MUST refuse while the first is alive.
  Verify: testscript — start two; the second exits non-zero naming the first's pid.
- **R-WRK-002** `ness worker install` MUST write `~/.config/systemd/user/ness-<project>.service`
  and no other file outside the repository.
  Verify: testscript — `$HOME` diff after install is exactly that file.
- **R-WRK-003** The installed unit MUST run the binary by its absolute path (the one resolved at
  install time), not via `PATH` lookup (§16 #3), re-pointed by `ness upgrade` and
  `ness self-update` when the installed path changes (R-UPD-048, R-UPD-062).
  Verify: testscript — `ExecStart=` starts with `/` and equals the resolved binary path.
- **R-WRK-004** `ness worker install` MUST refuse when no systemd user session is available.
  Verify: testscript — fake `systemctl --user` failing; exit non-zero; no unit file written.
- **R-WRK-005** `ness worker run` MUST work without systemd (the operator may run it knowingly).
  Verify: testscript — no systemd; `worker run` starts and writes a heartbeat.
- **R-WRK-006** `ness worker uninstall` MUST stop and remove the unit file it installed.
  Verify: testscript — file gone; fake systemctl received stop/disable.
- **R-WRK-007** The worker MUST refuse to start when herdr is missing or below the tested version
  (§4).
  Verify: testscript — as R-SESS-001/002 for `worker run`.
- **R-WRK-008** The worker MUST write a heartbeat (time and version) to the store on every pass.
  Verify: unit — after a pass the heartbeat time equals the fake clock and version the binary's.
- **R-WRK-009** The worker MUST re-resolve its binary path every 60 s and, when the resolved
  binary's version differs from its own, log one restart line and exit 0 so systemd restarts it.
  A self-update writes at the same path, so this check picks the new binary up.
  Verify: unit (synctest) — fake resolver reports a new version at t=61 s → the run loop returns
  exit 0 by t=120 s; log has one restart line; same version → keeps running.

## Delivery (waking)

- **R-WRK-010** The worker MUST wake an idle recipient for an unread item of every intent
  (`request`, `fyi`, `decision`, `handoff`) (F1 fyi-wake regression).
  Verify: unit — one item per intent to idle sessions; each gets a hint.
- **R-WRK-011** Hints MUST be batched: at most one hint per recipient per pass, covering all its
  unread items; urgent items bypass batching (R-MSG-042).
  Verify: unit — three items to one idle session → one typed hint in the pass.
- **R-WRK-024** The worker pass interval MUST be 5 s.
  Verify: unit (synctest) — passes at 5 s, 10 s, 15 s of fake time.
- **R-WRK-025** Hint text MUST be short, fixed per intent mix, and frozen by golden tests.
  Verify: unit — golden hint strings.
- **R-WRK-012** The worker MUST NEVER hint an item that has already been pulled.
  Verify: unit — item pulled; subsequent passes type nothing for it.
- **R-WRK-013** The worker MUST NOT type into a pane whose session is working.
  Verify: unit — fake runtime reports working; no type-text call.
- **R-WRK-084** The worker MUST treat a pane's live agent as the session's own when its herdr
  name or its client conversation id (herdr's `agent_session`) matches the stored one; herdr can
  clear a name while the agent lives on, and a session must not go deaf for it (field run 2).
  Verify: unit — name cleared, same conversation id: hinted; name cleared, other id: not typed.
- **R-WRK-090** The worker MUST deliver to exactly the clients whose atom reads its screen and
  gives the resume and rename texts, found through the client seam, never by naming an atom
  outside the composition root; a session of any other client is logged as not delivered to.
  Verify: unit — the worker's clients are claude, and not codex.
- **R-WRK-091** `ness doctor` MUST warn, naming the client, when the configured client or the
  client of an open session has no worker support, listing the sessions it leaves unwoken.
  Verify: unit — claude only: ok; an open codex session: warn naming codex and the session;
  codex configured, no session: warn naming codex.
- **R-WRK-092** `ness close --authorized` MUST be refused, naming the client, for a session whose
  client cannot check the operator's words (R-SESS-121).
  Verify: unit — a codex root closing itself with `--authorized` → refused naming codex.
- **R-WRK-014** The worker MUST NOT type into a non-empty input box.
  Verify: unit — fake screen with a draft on the prompt line; no type-text call.
- **R-WRK-015** The worker MUST read the input box as ANSI and treat dim text on the prompt line
  (the client's suggested prompt / placeholder) as empty, not as a draft (P3 placeholder
  regression).
  Verify: unit — captured ANSI screens: dim suggestion → hint delivered at once; normal-intensity
  text → no hint. Table covers the screens in the client's rendering (prompt glyph, SGR 2 / 90 /
  grey 38;2), synthesized (OP-WRK-11).
- **R-WRK-016** A placeholder MUST NOT delay delivery (no staleness timeout, no extra interrupt
  key sent).
  Verify: unit (synctest) — dim placeholder; hint typed in the same pass; no ctrl+c sent.
- **R-WRK-017** Focus and scrolling MUST NOT hold a delivery: a pane the operator has focused
  gets its hint, compaction, resume or rename like any other idle pane; only the input box holds
  (R-WRK-014, R-WRK-018). The stop of a session whose model is unavailable still spares a focused
  pane: it closes a tab and delivers nothing. (Amended by CX-15: herdr reports the selected tab as
  focused with nobody at the terminal, and held a root's hints for two hours in field run 1.)
  Verify: unit — fake runtime reports focused, box empty and unchanged → hint typed in the same
  pass; no focus log line.
- **R-WRK-018** The quiet window MUST be keyed on the input box only: a pane is quiet while its
  box holds non-placeholder text, or for `worker.quiet_seconds` (default 60 s) after the box's
  content last changed (a draft typed, edited or cleared; a draft submitted as a prompt is not a
  change). Focus, scrolling, the session's own transcript writes and tool calls MUST NOT start
  or extend it (F3 regression, CX-15).
  Verify: unit (synctest) — root busy writing its transcript then idle with empty box, no draft
  ever → hint in the first pass after idle; a draft seen at t=0 and cleared at t=5 s → no hint
  before t=65 s, hint at the first pass after.
- **R-WRK-019** When the pane is idle and the box empty and unchanged within the quiet window, the
  worker MUST deliver in the same pass, focused or not.
  Verify: unit — immediate delivery.

## Zombie items and logging

- **R-WRK-020** The worker MUST retire items whose route is revoked or whose recipient is closed,
  writing one log line per retired item (F4 zombie regression).
  Verify: unit — item to a closed session; after one pass it is retired; log has one line; later
  passes do nothing for it.
- **R-WRK-021** The worker MUST write exactly one line to `<git-common-dir>/ness/worker.log` per
  action (hint, compaction, retirement, limit resume, stall report, probe wake, operator
  notification, restart).
  Verify: unit — each action once → one line each, naming the action and session.
- **R-WRK-026** `worker.log` MUST be rotated at 10 MB, keeping 2 rotated files.
  Verify: unit — writing 25 MB leaves `worker.log` < 10 MB plus `worker.log.1`, `worker.log.2`
  and no `.3`.
- **R-WRK-022** A repeated identical error MUST be logged once, not once per pass.
  Verify: unit — failing runtime for 100 passes → one log line for that error.
- **R-WRK-023** Successful hints MUST be logged (one line each).
  Verify: unit — see R-WRK-021.

## Boundary compaction

- **R-WRK-030** After a session ran `ness inbox --boundary NOTE`, the worker MUST compact it when
  it is next idle (box empty and quiet, R-WRK-018) by typing `/compact <NOTE>`.
  Verify: unit — boundary recorded; on idle the fake runtime receives `/compact <NOTE>`; log
  line written.
- **R-WRK-071** A Claude Code root adopted with `--here`, which ness did not launch and so could
  not name (R-SESS-120), MUST be renamed by the worker the first time it sees it idle and free
  (box empty and quiet, R-WRK-018), by typing `/rename <project>/<title>`
  once per worker run (D42).
  Verify: unit — an adopted idle session receives `/rename repo/title` once over two passes; a
  launched session and a drafted pane receive nothing; a focused pane is renamed (R-WRK-017).
- **R-WRK-031** The worker MUST NOT compact a session without a recorded boundary.
  Verify: unit — no boundary; no compact command over many idle passes.
- **R-WRK-032** The boundary request MUST be cleared after one compaction.
  Verify: unit — second idle pass sends nothing.

## Usage limits

- **R-WRK-040** The worker MUST parse every usage-limit notice format seen in the discovery
  transcripts, including at least:
  `You've hit your session limit · resets 2:10pm`, `… · resets 4am`, `… · resets 3:40am`,
  `You've hit your weekly limit · resets Oct 1, 4am`, `… · resets Oct 8`,
  `Agent terminated early due to an API error: You've hit your weekly limit · resets Oct 8`,
  and `You've reached your Fable limit` (F2 regression).
  Verify: unit — table test of the real notices → (limited: true, reset time or "unknown").
- **R-WRK-041** The limit parser MUST NOT panic on any input.
  Verify: fuzz — seeded with the notices above.
- **R-WRK-042** The worker MUST type nothing into a pane while its session is limited.
  Verify: unit (synctest) — weekly limit with pending items over 27 h of fake time → zero
  type-text calls before the reset.
- **R-WRK-043** The worker MUST resume a limited session at the parsed reset time + 60 s by typing
  the client atom's resume text, once.
  Verify: unit (synctest) — at reset + 59 s nothing; at the first pass after reset + 60 s the
  resume text is typed once; log line.
- **R-WRK-045** For a limit notice without a reset time, the worker MUST keep the pane silent and
  re-read its screen every 15 min, resuming normal delivery once the notice is gone.
  Verify: unit (synctest) — "You've reached your Fable limit" → no typing; screen reads at 15-min
  intervals; notice cleared → next hint delivered.
- **R-WRK-046** Reset times MUST be interpreted in the box's local time zone.
  Verify: unit — TZ=Europe/Rome fixture: "resets 4am" → 04:00 local.
- **R-WRK-047** Limit detection MUST use per-pane notices, plus the client's usage file when
  present.
  Verify: unit — usage file reporting an active limit silences a pane without a notice; absent
  file → per-pane only.
- **R-WRK-044** A reset time without a date MUST resolve to its next occurrence; a date without a
  year MUST resolve to that date this year; a day already past, an impossible date or a reset more
  than 8 days ahead is Unknown (OP-WRK-11).
  Verify: unit — notices at 23:50 "resets 4am" → next day 04:00; "resets Oct 1" seen Sep 28 → Oct
  1 this year; seen Oct 2 → Unknown; "Feb 30" → Unknown.

## Stall reports

- **R-WRK-050** A session idle with open obligations for `worker.stall_minutes` (default 30)
  minutes MUST get exactly one stall message to its manager per stall episode. The clock starts
  at the later of going idle and the oldest open obligation's arrival. Every stall clock (this one,
  the owed brief's and R-WRK-093's) also restarts when an item reaches the session: one sent to it,
  or the answer disposing one it sent, the operator's items and answers included.
  A child session (one with a manager that is a session) that has never sent a `handoff` owes its
  brief: idle for `worker.stall_minutes` it MUST get the same single stall message per episode to
  its manager even with no open obligation, worded `... has been idle since T and has not handed
  off its brief`; the clock is the start of the idle episode. The worker MUST NOT type into the
  child. A root, and a child that sent a handoff, with no obligation get no stall message. A child
  whose screen shows a live background shell or a background agent, or whose transcript shows
  background work not finished (R-WRK-096), is working, not stalled: it gets no stall message and
  raises no alert. A session that waits (R-WRK-099: an unanswered item it sent, an active probe, a
  park) gets no stall message of either kind, owed brief or obligations; the screen's own signals
  (R-WRK-051, R-WRK-062, R-WRK-081) are unchanged. Known ceiling: two sessions that each wait on
  the other's item are both silent; status shows both waits. Whenever a stall is first reported the worker MUST log the last 6 lines of the
  pane's screen (escapes stripped, each cut at 160 runes, the probes' configured secrets redacted).
  Verify: unit (synctest) — idle with an open item for 31 min → one item in the manager's inbox;
  longer → still one; at 29 min → none; idle two hours with nothing owed, then a request → none
  until 30 min after it; a child with no handoff idle 30 min → one message with `has not handed
  off its brief` (the full sentence, with `and`), nothing typed into it; after a handoff, a root, a
  launching child, or a screen showing a background shell → none; a manager whose child works, a
  child waiting on a request to its manager or a request or decision to the operator → none, and
  stalling again once answered; a returned handoff → stalls; a child with pending background work in its transcript, owing its brief or
  holding an old obligation → none, and one report once the work is finished; a request disposed at
  30 min 3 s → none until 30 min after it; the log has the screen lines, without escapes or secrets.
  `stalls_waits_test` — a child with an accepted request, an active probe, an open park or a park
  with an until-time, or an obligation plus a decision to the operator or a probe, idle two hours →
  no message, no alert; a child with an open handoff and a follow-up request → reported; a park reports once its until-time passes or something arrives; a probe
  once it stops.
- **R-WRK-051** A session whose screen matches one of the client atom's error patterns (at least
  API error, overloaded, capacity) MUST get one stall message to its manager. Usage-limit
  notices are excluded: a limited pane stays silent (R-WRK-042) and resumes after reset.
  Verify: unit — fake screens per pattern → one message each; a limit screen → no message.
- **R-WRK-052** A new stall episode (and a new Your move episode, R-WRK-093) MUST start only after
  the session did real work again. For a client with an activity reading (R-WRK-096) that is a new
  assistant turn in its transcript: a blip of work that writes none, such as an auto-compaction,
  does not end the episode and so repeats neither the report nor the alert. For a client without
  one, any work does.
  Verify: unit — a real turn then idle again for `stall_minutes` → a second message; a pane that
  worked without a new turn → none; a client without a reading: working then idle → a second.

## Operator alerts (CX-16)

- **R-WRK-080** (CHANGED: a reply to an operator-sent item counts; the Ask keeps its lines) Each pass, for every open request, decision or handoff to the operator, every
  checkpoint, and every reply a session sends to an item the operator sent (whatever its status; the
  alert text is the reply's first line) that the operator has not yet read, whose `alerted_at` is null the worker MUST send one alert and stamp it on success;
  kind Needs you, except a checkpoint without `## Ask` (kind Checkpoint). Text: the item's first
  non-blank line, leading `#` marks and spaces stripped (Needs you from a checkpoint: the whole
  `## Ask` section, its lines, list items and line breaks kept, blank lines around it trimmed; Checkpoint: the first item under `## Next`, its first line and
  the continuation lines up to a blank line or the next list item), cut as R-WRK-082 says; Page from the `Page:` line. It does not wait for idle and does not depend on the
  pane's status. A restart repeats none; an item disposed before its alert gets none. With no
  operator channel configured the worker MUST stamp the item without posting, so a channel enabled
  later posts none of them.
  Verify: unit — a working root's decision → one alert at once, none later; a child's item carries
  the root as thread key; fyi, replied and withdrawn items → none (a withdrawal's notice too); a reply to an operator-sent
  item → one alert, for each of resolved, accepted, declined, returned and deferred; a restart → none; checkpoint
  kinds, sections, page and the cut; `alerts_text_test` — a multi-line Ask whole and nothing of the
  next section; `alerts_markdown_test` — an Ask's list items on their own lines; the first Next item of a list, a paragraph and a numbered list, without the next
  item or the Page line; no channel → stamped, no post, no log, and enabling a
  channel posts none of the backlog.
- **R-WRK-081** A session whose pane reports `blocked` for 2 minutes MUST get exactly one fyi to its
  manager per blocked episode (a new episode once the status leaves `blocked`), and the operator a
  Trouble alert 10 minutes, 1 hour and 4 hours after the block began, then every 4 hours, while the
  status stays `blocked`; after an alert the next is the first of those points after now, so steps
  missed while the channel failed are not sent in a burst; none of it while its screen shows the client compacting its conversation
  (`✻ Compacting conversation…`): herdr reads that as blocked. The screen is read when a step is
  due, and the step is taken on the first pass that shows no compaction; that read is made at most
  every 30 seconds while the pane stays blocked.
  Verify: unit `operator_backoff_test` (stepped clock) — blocked 1 min 59 s → nothing; 2 min → one
  fyi to the manager, no alert; Trouble at 10 min, 1 h, 4 h, 8 h, 12 h and not a second before each;
  working then blocked again → a second fyi and the schedule from the start; blocked 15 min on a
  compacting screen → nothing for anyone; posts failing for 5 h, then one alert, the next at 8 h.
- **R-WRK-082** (CHANGED: session link, block layout, named URLs, no tab line) An alert
  (`worker.Alert`) MUST carry its kind (Needs you, Checkpoint, Trouble, Your move), the project, the
  session's title, the session's URL (the last Remote Control URL its transcript recorded,
  R-WRK-096, kept by the worker per session while the session's pane is the same run, or "" when
  none was read), the text (Markdown with its lines kept, at most 400 runes: a longer one is cut at
  the last sentence end (`. `, `? `, `! `) or line end in the second
  half of the cap, else at the last space, and `…` appended; a single word longer than the cap is
  cut at the cap; no cut point lies inside a link, an autolink, a bare URL, a code span, bold or
  italic, and a span with no cut point before it inside the cap is cut before the span), the page
  link or "" and the id of the root of the session's tree, which is the thread key. With no operator
  channel configured the worker MUST send nothing and log nothing; the log line of a sent alert
  carries the text on one line. The composition root renders each alert to one generic Markdown
  note through the existing poster, keyed to the thread `alerts:<root>`: the line
  `<emoji> **<Kind>** · <project>`, then `**[<title>](<session URL>)**` (`**<title>**` without a
  URL), a blank line and the text, and, only with a page, a blank line and
  `[📄 Open the page](<page URL>)`. The project and title are backslash-escaped (`\`, `[`, `]`,
  `*`, `_`, backtick) so they render as written and the title still links. A URL is put in a link
  target only when it is http(s) with a host and holds no space, `<>[]*"` or backtick, its `(` and
  `)` percent-encoded; a page that fails this shows as `📄 <page>` in plain text. In the text every
  bare or `<…>` http(s) URL (outside code and links, inside bold and italic too, trailing
  `.,;:!?'"` left out, ending before `*`, a backtick or the `…` of a cut) becomes a named link:
  `[📄 page](<URL>)` for a `claude.ai` URL whose path holds `/artifact/`, else `[🔗 <host>](<URL>)`;
  a list that follows a paragraph line gets a blank line before it. No raw URL is left. Emoji and
  kind: 🟠 Needs you, 🟢 Checkpoint, 🔴 Trouble, 🟡 Your move.
  Verify: testscript `worker_alerts` — a root's and its child's items share one thread key, another
  root's gets its own; unit — the alerts of R-WRK-080, R-WRK-081, R-WRK-093 and R-WRK-094 carry these fields; no
  channel → no post, no log line; golden — each kind rendered for Telegram and Slack, with and
  without the session link, with markup characters escaped, a page, named URLs and a list, and a cut
  400-rune text in one message; `alerts_markdown_test` — the exact note of each kind with and without
  links, URLs named, links and code kept, a list split from its paragraph, a cut at a line end and
  never inside a link, a bare URL or bold, the session URL read from the activity and kept when a
  later reading has none; `alerts_links_test` — a cut right after a URL keeps `…` out of the link,
  a title with `[WIP]`, `**`, `_` and backticks escaped, a page with parentheses encoded, a
  `javascript:` page and a path as plain text, an autolink with `)` encoded; `alerts_text_test` — a text cut at its
  last sentence end, at the last space without one or with one only in the first half, one that
  fits and one of exactly 400 runes left whole, a 401-rune word cut at 400.
- **R-WRK-083** A failed post MUST be logged once and retried on the next pass (the item or
  episode is not spent); a sent one MUST be logged as action `notify`.
  Verify: unit — fake channel fails twice then succeeds → one error line, the item still unalerted
  until the third pass, one delivered alert.
- **R-WRK-093** A root idle for `worker.stall_minutes` that waits on nothing else (R-WRK-099: an
  item to a session, an active probe, a park), has no ask to the operator from its latest turn (one
  created at or after its latest run of busy passes began: it went idle because it asked; an older
  ask does not cover it), and holds no open obligation whose stall report can fire (it waits on
  nothing at all: an old ask silences that report, so Your move speaks), whose own
  screen shows no background task and whose transcript shows no pending background work
  (R-WRK-096; the same holds for each descendant below), with no open descendant working, blocked, not yet seen this run
  or showing a background task on its screen, and none that owes its brief while waiting on nothing
  (R-WRK-050: that is its stall report; a descendant that waits cannot report), MUST get one Your move alert per idle episode (reset when it works again). It
  waits like a hint: a box with a draft, or changed within `worker.quiet_seconds`, gets none. One
  cause, one alert: the stall report or a Trouble alert covers what Your move would say. A child
  never gets one.
  Verify: unit — idle root alone → one alert at 30 min, none before or after, a second after it
  worked; suppressed by an ask from its latest turn, a working or blocked child, a working grandchild, a
  child with a live background shell, a launching child, the root's own background shell, a draft in
  its box, an open obligation (one Trouble alert only), a child owing its brief (one Trouble alert
  only); an idle child whose handoff was accepted does not suppress (the overnight case: exactly one
  Your move); `alerts_yourmove_waits_test` — an ask sent in the latest turn covers the idle root,
  after a later turn with the ask still open → one Your move at 30 min; an active probe or a park
  → none; an obligation behind an old ask → one; a child owing its brief that waits on the root's
  accepted answer, both idle 4 h → one; `stalls_park_turn_test` — a root parked, compacted, then
  a turn → one.
- **R-WRK-094** A screen error report (R-WRK-051) and a stop for an unavailable model (R-WRK-062)
  MUST each also send one Trouble alert per episode, its text the worker's own sentence; a blocked
  pane follows R-WRK-081. A stall report (R-WRK-050) goes to the manager only (a root's to the
  operator's inbox); the phone gets one Trouble alert only at the second miss: the same stall
  episode still open `worker.stall_minutes` after the report, with nothing reaching the session
  since. A failed post is retried without a second report to the manager, including the post of a
  stop, which the stop does not wait for. A child with a live background shell (R-WRK-050) raises
  none.
  Verify: unit — an error and a stop → one Trouble alert each; `alerts_routing_test` — a child owing
  its brief and a root owing an answer: the report at 31 min, no alert at 58, one at 62, still one
  later; an item reaching the child after the report, or the operator answering a root's report →
  no alert; with a background shell on its
  screen → none.
- **R-WRK-095** The Claude Code screen reader MUST set `Screen.Background` when the footer, the last
  non-blank line, shows `N shell` or `N shells` (with or without a leading `·`, in any permission mode), and not for
  the same text elsewhere.
  It MUST also set it for the client's status line `✻ Waiting for N background agent(s) to finish`
  when it is a line of the screen above the box (within 12 lines) that starts with the client's
  glyphs, not agent prose (`●`).
  Verify: unit — a real footer with one and with three shells → set; none, or the text in agent
  prose above the box → not set; the status line (real capture) with one and three agents → set;
  in agent prose, in a diff line or far above the box → not set.
- **R-WRK-096** A client MAY give the worker an activity reading of its conversation
  (`worker.Client.Activity`, `session.Activity`); the Claude Code atom reads the transcript. It
  MUST report background work as pending when a launch has no later finish. A launch is an id a tool
  result carries as a field (`toolUseResult`: `backgroundTaskId`, `resumedAgentId`, `agentId` or
  `taskId` with `status: async_launched`), never text a result quotes. A finish is a
  `<task-notification>` that is the whole text of a queue-operation, a typed user message or an
  attachment prompt (never inside a tool result), a `task_id`/`shell_id` result of TaskStop or
  KillShell, or a `system` record `agents_killed`; all in the conversation itself (not a subagent's,
  not an API request blob). A notification that says the agent "is waiting on its own background
  work" is not a finish. A launch with no record for 3 hours is no longer pending.
  The reading MUST also give the time of the last assistant turn that is not an API error, a
  compaction summary or a subagent's, and MUST cost only the bytes appended since the last read
  (state by file; a complete line at a time; a shorter file is read again). The worker MUST treat
  pending work like the footer's background shell (`Screen.Background`): the pane is working, not
  stalled (R-WRK-050), raises no Your move (R-WRK-093) and keeps its manager's tree busy. The
  conversation read is the one herdr reports for the agent when it reports one, else the stored id.
  The reading MUST also give the URL of the last `remote_session_change` attachment of the
  conversation (Claude Code writes one when the session runs with Remote Control, R-SESS-120), only
  an `https://` URL without whitespace, parentheses, brackets or angle brackets, "" when none: the
  alert links the session with it (R-WRK-082). A client
  without a reading, or one that fails, leaves the screen alone to decide.
  Verify: unit — `remote_test`: none, one, the last of two, a subagent's ignored, a non-https or
  link-breaking URL ignored, kept across an appended read; a trimmed real transcript: each kind of launch → pending; finished (enqueued or
  delivered), failed, or stopped-with-work-of-its-own → as stated; a blob or a Read result quoting
  a launch or a notification, a sidechain launch → not; TaskStop and agents_killed → finished; a launch
  older than 3 hours → not; a file not asked for in an hour is forgotten; appended lines, a partial line and
  a rewritten file; a compaction is not a turn; worker (stepped clock) — a child idle 90 minutes
  with pending work → no stall, no alert, and a parent's Your move waits; with the work finished →
  one stall.
- **R-WRK-097** A conversation whose last assistant record is a usage-limit error (Claude Code: a
  synthetic assistant message with `isApiErrorMessage` whose text is the `You've reached your X
  limit` notice, as `limitRe` reads it) is limited: the worker MUST type nothing into it
  (R-WRK-042), report no stall and raise exactly one Trouble alert, "out of usage", per episode,
  which lasts until a real assistant turn follows. Because the notice names no reset, the worker
  MUST look again after 30 minutes with one hint, so credits that came back are used; the alert is not
  repeated by that. A limit error older than those 30 minutes (a worker restart reads the whole
  transcript again) raises no alert and silences nothing. When the limit is suspected the worker logs the last 6 lines of the screen.
  Verify: unit — the real Fable error tail → no hint, no stall report after 90 minutes, one Trouble
  alert; a hint after 30 minutes, still one alert; an error 5 hours old → no alert, and the stall is reported; a later real turn then a second limit → a second
  alert; another API error → not a limit.
- **R-WRK-098** A root that is `done` (R-SESS-157) MUST get no Your move alert, for its own
  integrated work or its idle pane; a `full` root, a quick one not yet integrated, and a quick one whose pane worked again after the
  integration (a follow-up turn) are unchanged.
  Verify: unit `alerts_done_test` — quick integrated, quick not integrated, full integrated.

- **R-WRK-099** What a session waits on (`store.Waits`) MUST be derived from the store on every
  call, no worker memory, in this order: `ask`, each item it sent to the operator, and `item`, each
  it sent to another session (Note `to <id>`), that is a request, decision or a checkpoint with an
  Ask (not a handoff: delivered work), not retired, with no disposition or `deferred` or `accepted`
  (accepted: the answer is still owed); `probe`, each active probe it owns (Note its path and
  label); `parked`, its parked record (R-MSG-034) while it holds: with an until-time, until that
  time and nothing else; without one, until something reaches the session after it parked (the
  operator's items and answers included), or until the session's first real assistant turn
  (R-WRK-096; a compaction is none) after the compaction consumed its boundary, when the worker
  drops the record. A client without an activity reading ends it only by an arrival; after a worker
  restart the first turn read does not end it.
  Verify: unit `waits_test` — each disposition, an fyi, a handoff, an Ask-less checkpoint, a retired
  item and an item received; the operator's answer and item end an open park; `Unpark`; active, paused, stopped and another owner's probes; a park until a time (an
  arrival does not end it), an open park (an arrival does), after the compaction, replaced by a new
  boundary; the order; `stalls_park_turn_test` — a turn after the compaction ends an open park (a
  root's Your move, a child's stall), not a park with an until-time; testscript `status` — the row's `on ...` and the JSON `waits`.

## Probes and model availability (scheduler side; probe semantics in probes.md)

- **R-WRK-060** The worker MUST run probe floors on their schedule and put wakes in the owner's
  inbox; it MUST NEVER start an agent or run a probe scope headlessly (D10).
  Verify: unit — floorless probe due → an inbox item; the fake runtime records no new tab/client.
- **R-WRK-061** When a probe has no live owner, the worker MUST pause it and log it once.
  Verify: unit — owner and manager closed; probe state paused; one log line.
- **R-WRK-062** When the client reports the tier's model unavailable, the worker MUST stop the
  session and tell its manager (R-SESS-009).
  Verify: unit — fake notice → session stopped; item to manager.

## Regression corpus

- **R-WRK-070** The worker's tests MUST include the discovery regression corpus as table cases:
  launch race, draft vs placeholder screens, every usage-limit notice, fyi wake, quiet window on
  operator input, zombie items, repeated-error log storm, never-ran launch close.
  Verify: review — each case maps to a named test with a `covers:` marker (L7 exit evidence).
