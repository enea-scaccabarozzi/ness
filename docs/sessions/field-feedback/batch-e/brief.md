> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# Brief: batch-e (child of field-feedback root s-c5a82a215386)

## Question
Build Batch E of the field-feedback work: every rule ness otherwise enforces only in prose gets a gating check whose failure points to the prose it enforces. The full plan is docs/superpowers/plans/2026-10-15-field-batch-e.md (tasks 1-16, with "Root answers" at the end); the scope rulings are in docs/sessions/field-feedback/plan.md "Batch E scope" and decisions.md FF-D5, FF-D6.

Operator's principle (2026-10-15): "Ness overall should always pair some gating static checks or try to place them for every thing that otherwise would only be enforced in prose, and it's really important that the check fails with a pointer to the relative prose piece, to steer the agent in the right direction." Three levels: ness itself (E1), ness docs advising consuming projects (E2), and the research lab (E3, not yours: it goes with the research lab port).

## Result owed
All 16 tasks committed on your branch; `go test ./...`, `go test -race` on the touched packages, golangci-lint (both configs), `ness sync --check` and `python3 .ness/audit.py` green; an independent ness-review of the whole batch with its findings fixed or ruled; then `ness integrate --subject 'feat: …'` into the root's branch and a handoff.

## Limits
- Scope is settled; do not reopen it. A ruling that proves infeasible: ask the root (`ness send --to manager --intent request`) with options and a recommendation, then end your turn.
- You own the decomposition. Parallelise with subagents on disjoint files where the plan's dependency line allows (1→2 first; then 3-4/12-14 and 5-9 can run side by side; 10-11 after 2 and 6; 15 after 8 and 11; 16 last). Test tasks are written by a different subagent than their implementation (.claude/skills/ness-test-strategy).
- Run `ness sync` from a build of your worktree (`go build -o <scratch>/ness ./cmd/ness`), not the installed `ness`: it is older and renders stale text.
- Field observation for the pointer work: the installed hook refused `for b in a b; do git log …; done; ness status` style commands as "a shell loop over ness inbox/status" because the loop and `ness status` were in the same command line but not in the same loop; if cheap, the polling guard should require the ness call inside the loop body (add a test row).

## Autonomy
act within the brief.

## Tier
standard.

## Report
Handoff to the root with Done / Now / Next, the not-built items with reasons, and lessons.

## Salvage
Branch `keep/e-checks-task5` (commit 1654415) holds Task 5 acceptance tests a withdrawn sibling's test subagent committed (R-PROF-045..047, docs/spec/checks.md, internal/profile checks tests and testdata, sync_check_pointer.txtar, and small stubs in internal/profile). Review it against the plan; cherry-pick 1654415 if sound (it was written by a test agent, so it still counts as written apart from the implementer), otherwise ignore it. Lesson from that sibling: tasks 5 and 7 both add stubs to internal/profile, so run their test agents in sequence. The plan's Global Constraints name the root's worktree and trailer: use your own worktree; the commit trailers stay as given there.
