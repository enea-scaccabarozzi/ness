> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# delivery: decisions

- **DL-1** The workspace rule lives in the herdr atom: the launcher passes the manager's tab id as
  `TabSpec.Near`, and only the atom knows herdr ids are `<workspace>:<tab>`. The fallback
  `HERDR_WORKSPACE_ID` is read in the composition root (`buildRuntime`) into
  `herdr.Runtime.Workspace`, so it also covers `--resume` by the operator.
- **DL-2** The first prompt is submitted in the existing `sending-brief` step, after `agent start`
  reported readiness, not after the hook's ready mark: Codex's SessionStart timing is unverified
  and may wait for a first turn. A trust dialog makes herdr answer `agent_not_ready` at once and
  the agent `blocked`; the step waits that out up to `launch.ready_timeout` and stalls, so the
  existing trust hint and `--resume` path apply unchanged. No phase is added or reordered.
- **DL-3** The unavailable-model stop (`mustStop`) still spares a focused pane: it closes a tab
  and is not a delivery, so CX-15 does not reach it.
- **DL-4** The quiet window restarts on any change of the box's content (typed, edited or
  cleared), read every pass; a draft that stays is held by R-WRK-014 regardless.
- **DL-5** R-SESS-123 settles only a child's items to its manager. A root's items to the operator
  are the operator queue, which the sibling session (operator-loop) owns.
- **DL-6** The cheat sheet's title no longer says "after compaction"; the re-read line is printed
  only after a compaction, above the sheet.
- **DL-7** (review M1, L2) "Ready" is the mark plus a launch past its prompt step
  (`session.PromptPending`): status, the worker and `--resume` share it. A live client still
  waiting for its prompt is continued by `--resume`; a dead one gets a new run, as before.
- **DL-8** (review M2, M3) `agent_blocked` from the prompt call means nothing was sent, so the
  step keeps waiting (`session.ErrBlocked`). A start refused with `agent_not_ready` counts as up
  when the pane's agent is blocked, whether or not herdr lists its name; the fake answers a
  blocked start that way, as herdr 0.8.2 documents.
- **DL-9** (review info) A draft the operator submits leaves the box when the pane goes working;
  it does not open the quiet window when the agent is idle again.
- **DL-10** Not done from the review: L3 (`--resume` submits the prompt to a live client the
  operator may be using by hand; that is the documented way out of a stall), L4 (a manager tab in
  a workspace that closed fails the child's tab with ErrGone; the manager opening it has its tab),
  L5 (a blocked launch may wait two ready timeouts before stalling).
- **DL-11** Rebased onto the operator loop (2c15abf): the settle rule is R-SESS-123, since the
  sibling's R-SESS-121 (`--authorized`) landed first; the audit does not catch a duplicate id.
- **DL-12** (manager item, OL-13) The operator's close of a working root is refused
  (R-SESS-124); state unreadable or the agent gone does not refuse: there is nothing to stop.
- **DL-13** F12 (skip-permissions by default) is not delivered: Claude Code's auto-mode
  classifier refused this session both the live check and the change itself ("Create Unsafe
  Agents"). The implementation is set aside as a patch for the operator; it needs their explicit
  go-ahead in this session's permissions, not a manager's relay.
