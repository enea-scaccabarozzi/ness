> [!NOTE]
> Archive: altered for the public release. Names of private projects and local paths were replaced; the content is otherwise as written.

# Child: the operator loop (escalation, notifications, autonomy, root close)

Context (read first): docs/sessions/cli-ux/infra-lab-feedback.md (findings, the escalation and
autonomy proposals) and docs/sessions/cli-ux/decisions.md CX-16..CX-19, which are the operator's
rulings and bind this session.

Operator's words (verbatim):
- "for me it would be fine having to respond directly in a child session, however maybe when
  something is requiring my attention i should be notified via a swappable atom, that for now
  i'll implement only via telegram or similar things. It's a small notification for me telling
  that a session is idle and stopped waiting from me on something it cannot settle with manager.
  It should eb a small notification, not the full cotnext probably, since it should only tell me
  that i am required, full context goes in that chat. Assume that i do not read anything (both in
  manager, root and childs) if not explicitly required, so this points should include the full
  cotnext required for me to answer correctly and provide the correct informations or give the
  right direction"
- "Autonomy: fully agreed especially with previous point on notifications. However in some cases
  i may authorize in adcance multiple planned child sessions or a scope bigger than one session,
  in that case that checkpoint must be skipped after plan, and only require me if going outside
  the planned scope"
- "closing a root session is something that the agent cannot do right now, and it should be
  designed better. An agent should be able to close a root session by himself without closing the
  tab nor the client agent itslef (CC) sicne i may want to read it back after. A root session
  cannot be closed without explicit authoriation from operator." (The failing case: the end of the
  infrastructure lab's root transcript, ~/.claude/projects/<encoded project dir>/
  <session-id>.jsonl: "ness: only the operator closes a root".)

Result owed (specify first: specs.md with the requirement ids you add or amend, then build):
1. CX-16 notification: the worker notifies the operator, once per episode, when a session is idle
   with an open item it sent to the operator, or when its pane is blocked on a prompt for more
   than a short while. One short line naming `<project>/<title>` and the item's first line; no
   context. The channel is swappable: a configured notify channel (Telegram first, reusing
   internal/atom/telegram and the [notify] config), chosen in .ness/ness.toml. No channel
   configured → nothing sent, and doctor says so. Tested against the fake; do not enable a live
   channel or touch any token (the operator configures it).
2. CX-17: any session may `ness send --to operator` for what its manager cannot settle; the manager
   gets an automatic copy; the asker records the operator's answer given in its own chat with
   `ness reply`. Amend R-MSG-017/R-MSG-024. The skill text says what such an item must contain
   (written for an operator who has read nothing: question, facts needed, options with a
   recommendation, what happens meanwhile) and that the same full text goes in the chat.
3. CX-18: autonomy levels in the brief (plan first / report then act / act within the brief), set
   from the operator's words, never lowered by a root for a child, and a plan the operator approved
   in advance skips its checkpoints until work leaves its scope. Core skill text (ness-entry,
   ness-launch brief template), and fill ness-entry's empty "Reserved decisions" with the default
   list (irreversible actions, money, accounts and credentials, publishing outside the repository,
   policy changes, scope beyond the brief); a project may extend it.
4. CX-19: `ness close <own root id> --authorized "<operator's words>"` closes a root from inside
   it when those words appear in an operator-typed message of the session's transcript (not a
   worker-typed hint, not a tool result); otherwise refused, telling the agent to ask. Closing a
   root never closes its tab or its client, launched or adopted. The existing landed, report and
   obligation checks still apply. Regenerate core text with `ness sync` as the repo requires.

Limits:
- A sibling (delivery) changes the worker's focus hold, launch, the session-start cheat sheet and
  close's disposal of a child's items. Expect to rebase internal/worker/worker.go and
  internal/cli/close.go at integration.
- Autonomy: act within this brief; the operator approved it in advance. Ask me only for something
  outside it (e.g. a design that cannot satisfy a ruling).
- Lifecycle skills as ness-entry says; gate is `make validate`; never --no-verify.

Report: report.md in your record (include the exact config the operator adds to turn Telegram on),
then `ness integrate` into my branch and a handoff via ness send.
