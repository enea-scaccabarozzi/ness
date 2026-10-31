> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# delivery: review of e165edb (against 7b82735)

Scope: F1, F2, F4, F5 and F6 against specs.md, plan.md, decisions.md and the amended requirements.
How I checked it: I read the diff and ran `go test` on session, cli, worker, store, atom and testkit
(all pass). I ran mutations in a scratch copy of the tree, never in this worktree. I also read the
real `herdr 0.8.2` help for `agent prompt`, `agent start` and `tab create`.

No blocker. Three medium findings, all in F4, and some test gaps.

## Medium

**M1. A launch that stalls at sending-brief and then has its dialog accepted shows as `ready`,
and the worker treats it as ready, but the first prompt is never sent until someone runs
`--resume` by hand.**
`internal/session/phases.go:35`, `internal/worker/worker.go:155`, `internal/session/launch.go:387`.
Scenario: the client starts on a trust dialog and the launch exits 4 (stalled at sending-brief).
The operator accepts the dialog in the tab, as the trust hint says. The SessionStart hook writes
the ready mark. From then on:
- `Shown` returns `ready`, so `ness status` and the identity line say `ready`.
- `visit` uses only `Pane` and `ReadyAt` to decide, so the worker can type a hint or a report into
  an agent that has no identity and no brief. Its manager may already have sent it items.

Nothing else submits the prompt. Before this commit the prompt was in argv, so a ready mark always
meant the brief was on its way.

Fix: make `Shown` and the worker require phase > sending-brief when the mark is set (the same
predicate `exitedOrStalled` now uses). Or show `stalled launch (prompt not sent)`.

**M2. If `herdr agent prompt` refuses the prompt with `agent_blocked`, the launch fails hard
instead of waiting.** `internal/session/launch.go:388-394`.
`prompt` returns `(true, l.Runtime.Prompt(...))`, so any error from the submission ends `waitFor`.
Scenario: `agent get` says `idle` or `unknown`, and an approval dialog (MCP servers, an update
notice) or herdr's status lag shows up before the prompt call. herdr answers `agent_blocked`. The
real help says this happens "before any input is sent", so a retry is safe. The launch exits with
`sending-brief: ... agent_blocked; fix the cause, then run ness open --resume`, and the trust hint
is not shown. The same applies to `agent_not_ready` while the client is still starting. `--resume`
recovers, but the launch reports a failure where the spec wants a wait and then a stall.

Fix: when the error code is `agent_blocked` (or `agent_not_ready`), return `(false, nil)` so the
loop keeps waiting.

**M3. The blocked-client tests never exercise the path real herdr takes.**
`internal/testkit/herdr.go:332-342` (fake `agentStart`), `internal/session/launch.go:344`.
- Real herdr 0.8.2 answers `agent start` with `agent_not_ready` at once for a client blocked on a
  dialog. The fake returns success for `start_status blocked`.
- So `TestBlockedAgentGetsNoPromptAndResumeSubmitsIt` and `open_blocked.txtar` never reach the
  `err != nil && agentUp(...)` branch, and that branch is the only thing that turns a real
  trust-dialog launch into a stall instead of an error.

Mutation 1: the fake returns `agent_not_ready` for a blocked start. Both tests fail, because the
fake discards its state on any error, so the agent is not listed.

Mutation 2: the fake also persists the agent. Both tests pass.

So the code is right only if `herdr agent list` shows a blocked-start agent under its name. Nobody
has verified that, and nothing records the "real herdr check of F4" from plan.md step 4. If herdr
does not list it, every launch that meets a trust dialog fails at starting-agent. That is the F4
field case.

Fix: make the fake answer `agent_not_ready` and keep the agent, and verify the listing once against
real herdr.

## Low

**L1. Neither launch-state change is pinned by a test.** Both mutations survive the whole suite:
- Putting back `!s.ReadyAt.IsZero() ||` in `resumePoint` (`internal/session/launch.go:116`). That
  shortcut matters in this case: a blocked stall, the dialog accepted (mark written), then the tab
  is closed or the agent exits before `--resume`.
  - With the shortcut, the resume goes to sending-brief, and `State` on a gone pane fails the
    launch with ErrGone.
  - Without it, the tab or the agent is reopened.

  So the change is correct but untested.
- Moving the `exitedOrStalled` boundary from `> sending-brief` to `> waiting-ready`
  (`internal/cli/open.go:442`). The case is waiting-ready, mark set, agent dead. The mutant would
  continue in the same run: same tab, stale mark, prompt submitted again, no new run (R-SESS-047,
  R-SESS-048).

Add one testscript or session test for each.

**L2. A session at sending-brief with the mark set and a dead agent continues in the same run, with
the old mark still set.** `internal/cli/open.go:442`, `internal/session/launch.go:385`.
What happens: `resumePoint` goes to starting-agent (Legal) and a new agent starts in the same tab
with the same `NESS_RUN`. `waitReady` passes at once on the stale `ReadyAt`, so the new client's
hook is never confirmed. In practice the `prompt` step's `State` and `Prompt` calls act as the
liveness check.
- If the prompt went in before a crash (the window the `ponytail:` note names), the resumed
  transcript gets it twice, and the run counter does not move although a client did run.
- Before this commit the same case was a relaunch: NewRun, new tab, mark cleared.

Acceptable as documented, but call `ClearReady` when continuing from a phase at or before
sending-brief with a dead agent.

**L3. `--resume` no longer refuses a live session whose phase is at or before sending-brief.**
`internal/cli/open.go:442`.
Scenario: the operator accepts the dialog and starts working with the agent by hand. `--resume`
then submits the identity prompt in the middle of that work. herdr accepts a prompt while the agent
is `working`. R-SESS-047 refuses a running session. The new rule can't tell "accepted, idle,
waiting" apart from "accepted and in use".

**L4. The workspace comes from parsing the stored tab id, with no fallback.**
`internal/atom/herdr/herdr.go:166-171`, `internal/session/launch.go:276`.
- `strings.Cut(near, ":")` with no colon passes the whole id as the workspace.
- A manager tab in a workspace that has since closed makes `tab create --workspace wX` fail with
  `*_not_found`. `raw` maps that to ErrGone and the launch fails. Example: an operator
  `--resume`s a child after the manager's workspace closed.
- The R-SESS-030 Verify says contract(fake+real), but the real contract never passes `Near`.

Fallback: retry without `--workspace`, or read `workspace_id` from herdr.

**L5. A blocked launch can now take up to twice `launch.ready_timeout` before it stalls.**
`internal/session/launch.go:394` and `:405`. `prompt` waits up to the timeout, then `waitReady`
waits again. R-SESS-033 says it stalls "within launch.ready_timeout".

## Info

- **The quiet window starts when the worker sees the change, not when it happens**
  (`internal/worker/worker.go:240`). The box is read only while the agent is idle. A prompt the
  operator submits counts as "cleared" when the agent's turn ends, which holds deliveries for
  `quiet_seconds` after each operator turn. This is arguably what CX-15 wants. Note it in
  R-WRK-018.
- **R-SESS-123 also settles unanswered `request` and `decision` items as accepted.** That matches
  the spec as written, but a question the manager never read disappears without a trace in its
  inbox.

## Checked and fine

- **Crash at waiting-ready with the mark set and the agent gone, treated as exited.** Same as
  before the commit, and correct. `enter` writes waiting-ready only after `receipt` returned nil,
  so phase ≥ waiting-ready means the prompt was submitted. A relaunch (NewRun → opening-tab)
  submits the prompt again, as the argv prompt did before (R-SESS-049).
- **Removing the ReadyAt shortcut in `resumePoint` changes no other path.** For phase >
  sending-brief, `exitedOrStalled` either refuses the resume or calls NewRun (which clears
  `ReadyAt`). The first `ness open` starts with no mark. `Run` returns early for phase ready with
  the mark set. The only path that reaches `resumePoint` with the mark set is the new one (phase ≤
  sending-brief).
- **Mutations the tests catch:**
  - dropping the `blocked` wait in `prompt`: session test and `open_blocked`
  - going back to the old `exitedOrStalled`: `open_blocked`
  - keying the quiet window on `Draft != ""`, or not counting a cleared box as a change: 3 worker
    tests
- **The fake matches real herdr 0.8.2 on `agent prompt`.** The help shows
  `herdr agent prompt <TARGET> <TEXT>` with no `--wait`, and `agent_blocked` "before any input is
  sent", matching the atom and the fake's `agentPrompt`. `tab create --workspace` exists.
- **F5:** the cheat sheet prints at every start, and the re-read line only after a compaction.
  Goldens cover startup, resume, clear and compact.
- **F6:** the operator queue is excluded, deferred items are included, and `Retire` runs first, so
  a second close settles nothing.
