> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# batch-e: decomposition

Plan: docs/superpowers/plans/2026-10-15-field-batch-e.md (tasks 1-16). Batch B has landed (17cd1a0, b9364c1), so tasks 3, 4, 12-14 are unblocked.

Salvage: 1654415 (Task 5 tests) cherry-picked as 8a99dd4; the Task 6 implementer reviews it against the plan first and reports anything unsound instead of editing it.

Order (each test task by a different subagent than its implementation):
1. Phase A, this worktree: T1 tests -> T2+T3+T4 implementation.
2. Phase B, two lanes in separate worktrees, merged back here:
   - lane hook: T12 tests -> T13+T14 implementation (adds the polling-guard refinement from the brief: ness call must sit in the loop body).
   - lane checks: T6 impl -> T7 tests -> T8 impl -> T9 tests -> T9 impl.
3. Phase C, this worktree: T10 tests -> T11 impl; T15 docs.
4. T16, independent ness-review, fixes, integrate, handoff.
