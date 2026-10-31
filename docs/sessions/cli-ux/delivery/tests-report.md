> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# delivery: acceptance tests (the judge)

Written against `specs.md` (with the R-SESS-123 root exception). No production file touched. Run
against the implementer's tree as of this report: everything passes except one script, see
"Current status".

## Test infrastructure (internal/testkit)

- `herdr.go`, fake herdr:
  - `tab create --workspace W` gives ids `W:tN` / `W:pN`; `workspace_id` in tab, pane and agent info follows the id (default `wF`).
  - `agent prompt PANE TEXT` records into `Prompted [][2]string` (json `prompted`), not `Typed`. It refuses with `agent_blocked` for a blocked agent and `agent_not_found` for none.
  - Knob `StartStatus` (testscript `fake herdr start-status blocked`, Go `SetHerdr(..., "start_status", ...)`): agents start with that status. A blocked start writes no ready mark; the mark is written when the agent first leaves `blocked` (the real SessionStart hook fires after the dialog is accepted).
  - testscript: `fake herdr workspace W` (workspace of the last created tab), `prompted N`, `prompt REGEX`, `start-status S`. All negatable where checks.
- `contracts.go`: `ClientContract` no longer sets `LaunchSpec.Prompt` and rejects a prompt (`session.Prompt(...)` or "load ness-entry") anywhere in Args. `RuntimeContract` (agent part, fake only; the real run uses startAgent=false): `Prompt` to a pane without an agent errors; `Prompt` to an idle agent succeeds.

## Tests, requirement, wrong implementation rejected

F1, R-SESS-030
- `internal/atom/herdr/herdr_test.go` `TestOpenTabWorkspace` (4 cases): Near `w7:t3` gives `--workspace w7` (also over Runtime.Workspace `w9`); no Near and Workspace `w9` gives `--workspace w9`; neither gives no flag. Rejects: ignoring Near, ignoring Runtime.Workspace, an empty `--workspace ""`, Runtime.Workspace beating Near.
- `internal/session/launch_prompt_test.go` `TestChildTabOpensInItsManagersWorkspace` (manager tab w7 / manager wins over caller w9 / manager without tab uses caller w9 / default wF), `TestRootTabOpensInTheCallersWorkspace`, `TestResumeReopensTheTabInTheManagersWorkspace` (stalled child, tab closed out of band, resume lands in w7). Rejects: launcher not passing Near, using the caller's workspace for children.
- `internal/cli/testdata/script/open_workspace.txtar`: end to end through `ness open` with `HERDR_WORKSPACE_ID`: no env gives wF; w7 root gives tab `w7:t2`; w9 root; child opened from a process in w3 lands in w7; `--resume` of a child whose tab was lost lands in w7. Rejects: the cli not reading `HERDR_WORKSPACE_ID`, the cli reading it for children too.

F2, R-WRK-017/018/019, R-MSG-042 (internal/worker; the bench has a stepped clock, so no synctest/sleeps)
- Rewritten: `TestFocusedPaneIsNotHeld` (was ...IsSkipped: hint in the first pass while focused, and none held after focus leaves), `TestQuietWindowIgnoresWorkAndFocus`, `TestUrgentBypassesTheQuietWindowNotTheDraft` (window opened by a box change, a non-urgent item held, urgent goes, urgent never over a draft), `TestUrgentAfterANonUrgentHint`, `TestDriftIsLoggedOnceAndFocusNever` (no "focused" log line), `TestUrgentIsTypedIntoAFocusedPane` (was ...IsNot...), `TestBoundaryAndResumeNeverTypeIntoABusyBox` (limits_test), `TestRenamesAnAdoptedRootOnceWhenFree` (renamed while focused), `TestDraftBlocksDelivery` and `TestCompactAndResumeWaitOutTheQuietWindow` (the window now runs from the change, i.e. the clearing of the draft, not from the last pass that saw a draft: one pass later than before), compact_test comment.
- New `quiet_test.go`: `TestQuietWindowRunsFromTheBoxChange` (draft at t=0, cleared at t=5: nothing through t=60, hint by t=70), `TestEditingTheDraftExtendsTheWindow` (retyped at t=35, cleared at 40: nothing at t=65, hint by t=105), `TestFocusedEmptyBoxGetsEverythingInTheFirstPass` (compaction in the first pass, no focus log line).
- Unchanged and still passing: stalls_test `TestStopWaitsForTheOperator` (the unavailable-model stop still spares a focused pane).
- Checked against the old `worker.go` (HEAD): 9 of these fail (focus tests, window tests, rename, log line).

F4, R-SESS-025/026
- `launch_test.go` `TestLaunchEndToEndOverFakes`: the prompt is no longer an argument. `launch_prompt_test.go` `TestPromptIsSubmittedOnceAfterTheClientIsReady`: exactly one `Prompted` entry `[pane, text]` with the exact text, no prompt text in Args, nothing typed. Rejects: prompt in argv, typing it, never submitting, submitting twice.
- `TestBlockedAgentGetsNoPromptAndResumeSubmitsIt` (session level): agent started blocked gives `ErrStalled`, no prompt, nothing typed; after unblocking, the next `Run` submits it once, and a further `Run` does not submit again.
- `herdr_test.go` `TestPromptSubmitsToTheAgentWithoutWaiting`: argv is exactly `agent prompt PANE TEXT` (no `--wait`), `ErrGone` for a pane without an agent, `agent_blocked` error and nothing recorded for a blocked agent.
- Scripts: `open.txtar` (args regex ends at `--name infra-lab/prog`, `! fake herdr args 'You are ness'`, `prompted 1` / `prompt` exact text for root, `prompted 2` for the child, `typed 0`), `open_env.txtar` (codex args end with the hook flags, prompt submitted), new `open_blocked.txtar` (blocked start exits 4 as a stalled launch with no prompt and nothing typed; `--resume` after the agent is idle submits once; a second `--resume` is refused and submits nothing).
- `claude_test.go` / `codex_test.go`: Args without `Prompt`; claude Args end with `--name` value, codex Args end with the hook override; no prompt text.

F5, R-HOOK-006: `hook_session_start.txtar` with golden `startup.txt` (identity then cheat sheet, exactly the text of the brief), `compact.txt` (identity, `After compaction: re-read ...`, cheat sheet, post-compact files), resume and clear payloads equal to `startup.txt`, `! stdout 'After compaction'` on startup, a root also gets the sheet; each at most 20 lines. Rejects: sheet only after compaction, no re-read line, wrong order. `open_here.txtar` already expected `cheat sheet`; no other script asserted its absence.

F6, R-SESS-123
- `internal/store/settle_test.go` `TestSettleFromDisposesOnlyWhatTheSenderSentTheRecipient`: handoff, decision, fyi and a deferred request are disposed accepted, answer `closed with the child`, DisposedAt=now; count 4; an already declined item, an item to a sibling, an item from another child and the reverse direction are untouched; no item created; second call returns 0.
- `close_settle.txtar`: a child sends handoff, decision, fyi, request and one request the manager defers; the manager closes the child: all five accepted with the answer, the manager's inbox shows none, the operator can then close the root (no obligations); a root's request to the operator stays open after the root closes (the spec tweak). Siblings cannot message each other, so "item to someone else" is covered at store level only.
- Rejects: not settling at all, leaving deferred open, creating replies after the retire, settling the operator's queue on a root close.

## Current status

`go vet ./...` clean. `go test ./...`: everything passes except `TestScripts/open_blocked`.

- `open_blocked.txtar` fails at the `ness open --resume $R` after the agent was unblocked: `session ... is running; close it or wait for it to exit before --resume` (exit 4, `exitedOrStalled` in `internal/cli/open.go`). This is an implementation gap, not a test bug: once the operator accepts the dialog, the client's SessionStart hook writes the ready mark, and the cli then treats a live, marked agent as "running" although the launch is still in `sending-brief` and the prompt was never submitted. Resume must continue a launch whose phase is not `ready` even when the mark is set. (The Launcher itself behaves right: the session-level `TestBlockedAgentGetsNoPromptAndResumeSubmitsIt` passes.)

Pre-existing, not mine: `gofmt -l` lists `internal/session/screen_test.go` and `internal/worker/loop_test.go`; `launch_test.go`, `worker_test.go` are over the 400-line ceiling under existing audit exceptions. My new tests are in new files.

## Follow-up: gaps from the review (e165edb plus the production fixes)

Each test below was checked by mutating the production code in a scratch copy; every mutation
is killed. `open_blocked` and the earlier tests pass against the new fake (a blocked start now
answers `agent_not_ready` and keeps the agent).

| Gap | Test | Mutation killed |
|---|---|---|
| Shown with mark + sending-brief | `TestShownIsNotReadyWhileThePromptIsPending` (session): starting-agent/sending-brief with a mark show `stalled launch` past the timeout, the phase before; waiting-ready/ready with a mark show `ready` | `Shown` back to "mark means ready" |
| Worker and a pending prompt | `TestNothingTypedWhileThePromptIsPending` (worker/quiet_test.go): mark + sending-brief gets nothing; the same session at `ready` gets its hint | dropping `PromptPending` from `visit` |
| Prompt refused as blocked | `TestPromptRefusedAsBlockedIsWaitedOut` (session): the first `agent prompt` is refused with an `agent_blocked` envelope through the real herdr atom; the launch succeeds and the fake records exactly one prompt. `TestPromptSubmitsToTheAgentWithoutWaiting` (herdr) now requires `errors.Is(err, session.ErrBlocked)`, not `ErrGone` | launch treating the refusal as failure or as success; the atom not mapping `agent_blocked` |
| resumePoint with a mark | `TestResumeReopensTheTabWhenTheMarkIsSetButThePromptIsPending` (session): mark + sending-brief + tab closed: a new tab, phase ready, a second prompt to the new pane | reinstating the `ReadyAt` shortcut |
| `--resume` boundaries | new `open_resume_pending.txtar`: mark + sending-brief + live agent continues in the same run (prompt 2, run 1); mark + sending-brief + exited agent is a new run (run 2, prompt 3); mark + waiting-ready + live agent is refused as running, nothing submitted | moving the pending boundary to waiting-ready; computing `ready` without `PromptPending` |
| Submitted draft | `TestSubmittedDraftStartsNoQuietWindow` (worker/quiet_test.go): draft at t=0, two minutes of work, then an idle empty box gets its hint in the first idle pass | `working()` not resetting the remembered box |

Also: `gofmt` of `rename_test.go` fixed (mine). `mise exec -- make validate` exits 0 (build, both lint configs, all tests, audit, vuln, selfcheck).

## Follow-up: R-SESS-124 (operator close of a working root)

`close_root_working.txtar` (covers R-SESS-124): a working root and the operator's close exits 4 with
`working`, leaving row, branch, worktree and tab alone; a working root closing itself with
`--authorized` and a typed transcript succeeds; the same root once idle closes for the operator.
Mutations killed in a scratch copy: `idleRoot` call removed, the `self` exemption removed, the
`Status == "working"` test dropped (an idle root would be refused).
