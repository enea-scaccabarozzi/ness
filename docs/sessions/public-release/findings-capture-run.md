> [!NOTE]
> Archive: the record of the session that prepared the public release. Names of private projects were replaced; the content is otherwise as written.

# Findings from the real capture run (2026-10-31)

A scratch Go todo repo, three rounds, $2.95. Evidence is in the scratchpad (capture/README.md). These are ness frictions to triage, not docs content.

1. A root ignored the operator's tier ("economy"): the root opened at standard; only the children were economy.
2. The root's herdr tab keeps herdr's default label "1"; child tabs take their --title.
3. The pane id appears only in the "prompt sits unsubmitted … pressing Enter" notice; the root lost one child's pane id.
4. Typing then Enter sometimes does not submit, at child launch (ness recovers by pressing Enter).
5. The pre-tool hook refused a hand-run `make test` (the gate) twice; both recovered with `go test`. Expected behaviour, but the agents tripped on it.
6. A child wrote its handoff to /tmp/h.md, outside the repo and scratch.
7. Run 1: the agent promoted the conversation without proposing first (ness-entry says propose unless asked to start).
8. Docs: the first launch in a new repo stalls on the Claude Code trust dialog. Trust the repo once first.

## Hero recapture round (2026-10-31, root todo-undo-open, ~$1.48)
9. The root asked twice to integrate (once per child), so the phone got two alerts for one decision.
10. `ness close` of the children after integrate refused: no child report. That cost a third ask and a second integrate (b3083fc) just for the report files.
11. The status bar showed `todo@main` for a while after promotion.
12. The worker's `ness: 1 unread (handoff); run ness inbox` lands on the root's prompt line.
13. The root again ignored the economy tier in the request (finding 1 repeats).

## Docs lane "start" (2026-10-31)
14. `ness doctor` right after `ness worker install` exits 1 ("no heartbeat"), then passes seconds later.
15. Default Codex model ids for five rungs do not resolve on codex 0.162.1; `doctor` fails tier-models right after opt-in.
16. A hand edit to a rendered skill passes `ness sync --check` (exit 0); only doctor warns (sync-drift).
17. Removing the program profile leaves its seeded docs, and sync says nothing.
18. doctor's pin-mismatch fix suggests `mise use ness@<pin>`, which is unverified as an install path.

## Docs lane "guides" (2026-10-31)
19. Switching the client from codex back to claude leaves an empty `[agents]` table in `.codex/config.toml`.
20. docs/spec/core-config.md shows `fallback_model` under `[models.kinds]`; the code reads it from `[models]`.

## Review round 1 (2026-10-31)
21. `ness open --resume ID --tier up` refuses while the session's client is live, so the escalation the skills describe needs the session to exit first.
22. The `waits` status column and its code label read inverted (product call; documented as is, PR-D11).
23. The ness-entry and ness-launch skills (profiles/core) still describe resuming a session "with --tier up" while it is live; the code refuses that. Skill text and code must agree in the next round.
