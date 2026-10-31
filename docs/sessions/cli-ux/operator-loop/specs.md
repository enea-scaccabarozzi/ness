> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# operator-loop specs

What this session builds for CX-16..CX-19, as requirement ids added to or amended in
`docs/spec/`. The ruling text in `docs/sessions/cli-ux/decisions.md` binds; the ids below are its
testable form.

## CX-17: any session asks the operator (store-messages.md)

- **R-MSG-017 (amended)** A send to a session that is not the sender's manager or one of its
  children is refused; `--to operator` is accepted from any session.
- **R-MSG-051 (new)** A child's send `--to operator` also creates one fyi to the child's manager,
  body `[copy of <id> to operator]` + blank line + the body; a root's send creates no copy; an
  idempotent repeat creates no second copy.
- **R-MSG-023 (amended)** An operator item may be replied to by the operator, by its sender (the
  asker records the operator's chat answer) or by the sender's manager (who holds the copy).
- **R-MSG-024 (amended)** The sender of an operator item, root or child, records the operator's
  chat answer with `ness reply`.
- **R-MSG-052 (new)** When the replier is the item's sender, no reply item is created (the asker
  already knows the answer); any other replier's answer reaches the sender as today.

## CX-16: the operator is notified (worker.md, core-config.md, doctor.md)

- **R-CFG-055 (new)** `[notify] operator = NAME` names the channel operator notifications go to;
  NAME must be a key of `[notify.channels]` and the `notify` feature must be on, else the config
  is refused naming the key.
- **R-WRK-080 (new)** A session idle with an open request, decision or handoff it sent to the
  operator gets one operator notification per idle episode (a new episode after it worked again).
- **R-WRK-081 (new)** A session whose pane has been blocked on a prompt for more than 2 minutes
  gets one operator notification per blocked episode.
- **R-WRK-082 (new)** The notification is one line, `<project>/<title> needs you: <first line of
  the item, at most 80 runes>` or `<project>/<title> is blocked on a prompt in its tab`, with
  nothing else; without a configured operator channel nothing is sent and nothing is logged.
- **R-WRK-083 (new)** A failed post is logged once and retried on the next pass; a sent one is
  logged as action `notify` (R-WRK-021's list gains it).
- **R-DOC-024 (new)** Doctor reports the operator channel: `ok` naming channel and kind, `warn`
  when none is configured (the operator is not told when a session needs them), `fail` when the
  named channel cannot be built.

## CX-19: a root closes itself (sessions.md)

- **R-SESS-060 (amended)** The operator closes a root; a root closes itself only with
  `--authorized WORDS` (R-SESS-121).
- **R-SESS-121 (new)** `ness close <own root id> --authorized WORDS` is accepted when WORDS appear
  (case-insensitive, whitespace folded) in a message the operator typed in the session's client
  transcript: typed user lines only, never a tool result, a meta or system line, a sidechain or a
  text ness types (worker hints, `/compact`, `/rename`, the resume text, the launch prompt).
  Otherwise, and for a client without transcripts, it is refused telling the agent to ask the
  operator; `--authorized` on a child or on another session's root is refused.
- **R-SESS-122 (new)** Closing a root never closes its tab or its client, launched or adopted; the
  report, obligation, landed and integration checks apply unchanged.
- **R-SESS-065 (amended)** The tab is removed for a child only.

## CX-18 and skill text (profiles-builtin.md, hooks cheat sheet)

- **R-BLT-102 (new)** `ness-entry` says when to ask the operator directly (what the manager cannot
  settle: a reserved decision, a fact only the operator has, an action only the operator can take),
  what the item contains (question, facts needed, options with a recommendation, what happens
  meanwhile), that the same full text goes in the chat, and that the answer is recorded with
  `ness reply`.
- **R-BLT-103 (new)** `ness-entry` names the three autonomy levels (plan first; report, then act;
  act within the brief), that the operator's words set the level and "me" stays the operator,
  that a root never lowers a child's level, and that a plan the operator approved in advance skips
  its checkpoints until work leaves its scope; `ness-launch`'s brief template carries an
  Autonomy line.
- **R-BLT-104 (new)** `ness-entry`'s reserved decisions list irreversible actions, money, accounts
  and credentials, publishing outside the repository, policy changes and scope beyond the brief;
  the slot still accepts a project's additions.
- **R-BLT-105 (new)** `ness-entry` says how a root closes itself: `ness close <id> --authorized
  "<the operator's words>"`, only after the operator said so in the chat, and that the tab stays.
- The session-start cheat sheet (R-HOOK-006) lists `--to operator` and `--authorized`.
