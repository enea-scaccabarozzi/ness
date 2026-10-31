> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# delivery: report

## Delivered

| Item | Requirement | Change | Judge test |
|---|---|---|---|
| F1 child tab in the parent's workspace | R-SESS-030 amended | herdr atom adds `--workspace` from the manager's tab id, else `HERDR_WORKSPACE_ID` | `TestOpenTabWorkspace`, `open_workspace.txtar` |
| F2 delivery held only by the input box | R-WRK-017/018/019 amended, R-MSG-042 wording | focus no longer holds; quiet window keyed on box changes; a submitted draft is not a change | `quiet_test.go`, worker/limits/rename/compact tests rewritten |
| F4 first prompt via `herdr agent prompt` | R-SESS-025 amended | clients drop the prompt from argv; `sending-brief` waits out `blocked`, then submits; `agent_blocked` keeps waiting | `launch_prompt_test.go`, `open.txtar`, `open_blocked.txtar`, `open_resume_pending.txtar` |
| F5 cheat sheet at every start | R-HOOK-006 amended | session-start prints it always; the re-read line only after compaction | `hook_session_start.txtar` |
| F6 closing a child settles what it sent | R-SESS-123 added; ness-launch text | `store.SettleFrom` in `finishClose` (children only) | `settle_test.go`, `close_settle.txtar` |
| OL-13 root close while working | R-SESS-124 added | `idleRoot` refuses the operator's close of a working root; `--authorized` self-close passes | `close_root_working.txtar` |

Review (review.md) found no blocker; M1–M3 and L2 fixed (DL-7..DL-9), L3–L5 left (DL-10). Each
judge test was checked against a mutation of the production code (tests-report.md).

## Not delivered

- **F12 (skip-permissions by default).** Claude Code's auto-mode classifier refused this session
  both the live check and the change ("Create Unsafe Agents"); I did not work around it (DL-13).
  The implementation is kept outside the branch as a patch
  (`launch.permissions = "skip" | "client"`, default skip; `--dangerously-skip-permissions` and
  `--dangerously-bypass-approvals-and-sandbox` in the clients' Args). It needs the operator's own
  go-ahead. The manager's item m-64kuxw2f is answered deferred.

## Evidence

- `mise exec -- make validate` exit 0 at 76b1b89 (build, both lint configs, tests, audit,
  govulncheck, `sync --check`).
- Real herdr (F4): Claude (haiku) started by `herdr agent start` without a prompt, ready in 4 s;
  `herdr agent prompt` submitted and Claude answered; the tab was closed. The trust-dialog path
  (`agent_not_ready`, then `blocked`) was not observed live: the directory was already trusted.
  It rests on herdr 0.8.2's documentation, which the fake now follows.
- Not verified live: that `herdr agent list` names an agent whose start was refused; `agentUp`
  also accepts the pane's agent being blocked, so the launch does not depend on it.

## Lessons

- Two parallel sessions picked the same next free requirement id (R-SESS-121). The audit checks
  that a covered id exists, not that it is defined once; a duplicate check would have caught it
  at the gate instead of at the rebase.
- A hook's ready mark and "the agent has its brief" are different facts once the prompt leaves
  argv. Every reader of `ReadyAt` (status, worker, `--resume`) needed the same predicate
  (`PromptPending`); grep every consumer of a flag whose meaning shifts.
- My own pre-tool hook refused a command whose argument text contained the pattern it guards.
  Put such text in a file when a check needs it.
- A permission classifier can refuse a brief item the operator approved through the tree. That
  approval does not reach the session's permissions; say so and hand the item back.
