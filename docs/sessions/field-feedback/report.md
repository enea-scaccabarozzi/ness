> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# Report: field-feedback

Fixes for the frictions found while running ness in the research lab. Evidence is in the private
`docs/discovery/findings/field-research/` folder.

## Delivered
- **B: operator alerts and liveness** (main 5594409). There are three alert kinds: Needs you, Checkpoint and Trouble. Alerts are formatted for Telegram and Slack and can carry links to pages. One cause sends one alert. An owed answer is reported whatever the session's descendants are doing; only the session's own background shell excuses it.
- **C: program profile** (main 5594409).
  - The refinement loop is part of planning.
  - The root challenges the plan in a Socratic way whenever new information comes in.
  - Checkpoints use Done, Now and Next.
  - The autonomy mode is asked for each program run.
  - Plans state success criteria instead of "scoring".
  - Only the root numbers decisions (FF-D5, FF-D6).
- **E: rules come with checks** (main 683fe32).
  - Every refusal ends with the skill and section it enforces, and a meta-test verifies this.
  - Profiles can add `[[checks]]` and projects can add `[[guards]]`.
  - The program profile ships four checks.
  - Sleep loops and Explore are refused.
  - Handoffs must have Done, Now and Next, and reports must have Lessons.
  - `docs/profiles/authoring.md` tells projects to pair each prose rule with a check.
- **D: port to the research lab.**
  - ness was synced in the research lab and the infrastructure lab, and the workers were restarted.
  - The `research-port` root folded the local profile, checks ranks 1-5, the guard against deleting units, one wording for heavy runs, and the removal of mdformat (the research lab's main c4153330).
  - Its review sent back fixes (item m-xw4e2aid): the deletion guard refuses relocations, the suite guards are too narrow, and two documents still disagree on heavy runs.
  - study-protocol was regrounded and has sent the operator its autonomy checkpoint.

## Evidence
- The gate was green at each fold (`scratchpad/integrate-e.out`, `go test -race` across all packages).
- Field check: a real checkpoint reached Telegram from this root at 13:09:56Z (`alerted_at` stamped, notify logged).

## Open
- Accepted limits: a child can delete its local checks (M5), and the guards have gaps (L2).
- research-port's review fixes are folded into the research lab's main 5ff5b1d2 (m-xw4e2aid resolved).

## Lessons
- Settle the direction once, then execute it. Brainstorming items that were already agreed cost a detour (FF-D3).
- One session per batch beat splitting a batch across children. Two of the children were withdrawn.
- Before ending a turn to wait, check that the thing you wait for is addressed to you. A sibling root's handoff goes to the operator, and the idle alerts that follow are correct.
- Private evidence goes to the discovery folder as soon as it is written, not when it is moved later.
