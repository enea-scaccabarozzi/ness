> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# prompt-submit decisions

- **PS-1** The launch confirms its first prompt (amends R-SESS-025, adds R-SESS-150). `Prompt` now
  runs `herdr agent prompt PANE TEXT --wait` with every `--until` state and `--timeout 10000`, so it
  returns at the agent's first state change after the submission, and `agent_prompt_stalled` maps
  to `session.ErrNotSubmitted`. Listing every state matters: `--until working` alone would miss a
  turn that went working → done between herdr's observations and wait out the timeout.
- **PS-2** Recovery presses Enter with `herdr pane send-keys PANE Enter`, a new runtime method
  `PressEnter` (the thirteenth; the root found `agent send-keys … enter` did not submit, and
  `pane run PANE ""` was unreliable live). The attempts are bounded: 3 Enters, each followed by a
  wait of up to 5 s for `working` or `blocked`, then the launch fails with `ErrNotSubmitted`
  naming the pane. Its phase stays at sending-brief, so `ness open --resume` retries. Both
  numbers are constants, not config: nothing calls for tuning them.
- **PS-3** Enter is pressed only on our own text. The agent must not be blocked, and the
  client's box, read with the client's screen reader (`Launcher.Read`, `claude.Read`; the same
  reader the worker uses), must start with the prompt (whitespace ignored, since the box wraps).
  The reason is the first live run: herdr reported the agent ready before Claude Code's trust
  dialog drew, the prompt stalled, and an unguarded Enter answered the dialog's default
  "No, exit" and killed two clients. A client without a reader (codex) fails the launch at the
  stall rather than pressing Enter blind.
- **PS-4** The prompt is typed at most once per run. The second live run showed Claude Code drawing
  the stalled text in its box seconds after herdr's 5 s verdict. A loop that saw an empty box
  and typed again submitted the prompt doubled, three times out of four. After a stall the
  launch only presses Enter on the box, or accepts a turn that started late; a prompt that
  never shows up stalls at `launch.ready_timeout`, and `--resume` types it again. A resume that
  finds the prompt still in the box presses Enter instead of typing it.
- **PS-5** Ruling on the worker (R-SESS-151): the worker does **not** recognise its own launch
  prompt in a box and submit it. The launch owns its first prompt and now never leaves it silently
  unsubmitted (PS-1..PS-4). A session whose prompt is pending stays in a launch phase, which the
  worker leaves alone, and teaching the worker to submit "its own" text would open a hole in
  R-WRK-014 (never act on a non-empty box) for a case the launch already closes.
- **PS-6** The judge tests were written by a separate subagent from the spec text, as
  ness-test-strategy requires. Its one failing test asked for 3 × 5 s of Enter waits inside the
  bench's 2 s ready timeout; the ready timeout bounds the whole prompt step, so that test now
  sets the production default (1 min).
