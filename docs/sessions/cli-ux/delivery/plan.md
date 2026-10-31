> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# delivery: plan

Judge and implementer are split (ness-test-strategy): a subagent writes the acceptance tests and
the fake-herdr changes from `specs.md`; this session writes the production code. Then the gate.

1. Tests (subagent, test files and `internal/testkit` only):
   - F1: herdr atom unit (Near / Workspace → `--workspace`), launch test or testscript (child in
     the parent's workspace, root from `HERDR_WORKSPACE_ID`, none otherwise), fake herdr ids.
   - F2: rewrite every focus-hold assertion in `internal/worker/*_test.go` (worker, limits,
     compact, rename) to the box-only rule; a focused-and-empty pane gets the hint; the quiet
     window follows box changes.
   - F4: open.txtar args regex without the prompt + `fake herdr prompt`; blocked agent → stall;
     resume submits; client Args tests; ClientContract drops the prompt rule.
   - F5: hook_session_start.txtar goldens.
   - F6: close testscript and store unit for SettleFrom.
2. Production (this session):
   - `session.TabSpec.Near`, `herdr.Runtime.Workspace`, `OpenTab --workspace`; Launcher passes
     the parent's tab; `buildRuntime` reads `HERDR_WORKSPACE_ID`.
   - worker: drop focus from `act`, `quiet` and the log; quiet keyed on box changes.
   - `Runtime.Prompt`; clients drop the prompt from Args; `sending-brief` waits out `blocked`, then
     prompts.
   - hook: cheat sheet at every start.
   - `store.SettleFrom`; `finishClose` calls it.
3. Spec amendments in `docs/spec/` (sessions, worker, hooks, store-messages), the ness-launch
   skill text, CHANGELOG if the repo keeps one by hand.
4. Real herdr check of F4 (one pane, then closed), `make validate`, review, report, integrate,
   handoff.
