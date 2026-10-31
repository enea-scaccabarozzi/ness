> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# prompt-submit report

## Delivered
1. **The launch confirms its first prompt** (`internal/session/launch.go`, `submission`).
   - `herdr agent prompt … --wait` (every `--until` state, `--timeout 10000`) maps
     `agent_prompt_stalled` to `session.ErrNotSubmitted`.
   - Recovery presses Enter with `herdr pane send-keys PANE Enter` (the new `Runtime.PressEnter`).
     It does so only while the agent is not blocked and the client's box starts with the prompt.
   - The launch tries at most 3 Enters, waiting up to 5 s after each for `working` or `blocked`.
     After that it fails loudly, naming the pane, and its phase stays where `--resume` retries.
   - The text is typed at most once per run, and a resume presses Enter on a prompt still in the
     box. A client without a screen reader fails the launch at the stall.
2. **Fake herdr** (`internal/testkit/herdr.go`):
   - It understands `--wait` and returns `agent_prompt_stalled` on a dropped Enter.
   - It answers `pane send-keys`, with `drop_enter` / `fake herdr drop-enter N` to drop Enters.
   - An unsubmitted box is drawn in `pane read`, and a `box` setter changes it.
3. **Spec**:
   - R-SESS-025 amended.
   - R-SESS-150 added: the confirmation, the Enter guard and type-once.
   - R-SESS-151 added: the worker ruling. The worker does not submit a launch prompt left in a box
     (PS-5).
4. **Judge tests**, written by a separate subagent from the spec:
   - `internal/session/launch_submit_test.go`: four synctest cases.
     - One dropped Enter.
     - Enters dropped past the attempts, then a resume.
     - Blocked at the stall: no key pressed.
     - Typed at most once.
   - `internal/atom/herdr/herdr_test.go`: the `PressEnter` argv, and the stall mapping.
   - `internal/worker/quiet_test.go`: the worker leaves the session's own launch prompt in its box
     untouched.

## Evidence
- **Repro with the current code.** On real herdr 0.8.2 and Claude Code 2.1.296 I launched three
  clients back to back in workspace wS. The no-wait prompt (the old call) and one `--wait` prompt
  both left their text in the box with the agent idle; the `--wait` one returned
  `agent_prompt_stalled`. `pane send-keys PANE Enter` turned each to `working` within about 1 s.
- **First fix run.** Four `ness open` in an untrusted throwaway repo. herdr reported two clients
  ready before the trust dialog drew, and the unguarded Enter answered "No, exit", killing both
  clients. That led to the box guard (PS-3). Rerun with the guard: both clients stayed alive on
  the dialog and the launches stalled with the trust hint.
- **Second run (trusted throwaway repo).** All four opened, but three submitted the prompt
  doubled: the text appeared in the box after herdr's verdict, and the loop typed it again. That
  led to type-once (PS-4).
- **Final run.** Six back-to-back `ness open`: **6/6 stalled, 6/6 recovered by one Enter**, each
  pane holding exactly one copy of the prompt. Five answered "OK" and one was still working.
- **Cleanup.** Every tab I created was closed and the throwaway repos were removed. One thing is
  left: Claude Code's trust entry for the deleted `.livecheck` dir in `~/.claude.json`. I did not
  touch that file while other clients run.
- **Gate.** `mise exec -- make validate` exit 0 (lint, tests, audit, vuln, sync check).

## Not done / notes
- The prompt may still be typed into a dialog that draws after herdr reports idle; digits in the
  text can pick dialog options. This was already true before the change, and it was not hit live
  once the Enter guard was in place.
- The ready timeout bounds the whole prompt step. A `launch.ready_timeout` under 15 s cuts the
  Enter attempts short, and the launch then ends as a stalled launch: still loud.

## Lessons
- herdr's "ready" for Claude Code comes before the client takes input: 6 of 6 Enters were dropped
  on this box. Any automation that types into a starting client must confirm the submission.
- Never press a key blindly into a client pane. Check that the screen shows what you expect it to
  act on, because a dialog's default answer can be destructive.
- A screen read right after typing can lag the client's rendering by seconds. "Not on screen" is
  not "lost", so never type again on that evidence alone.
- Live verification found both defects the fake could not show. Keep the real run in the loop.
