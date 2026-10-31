> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# prompt-submit

## The operator's words (verbatim)
"Also seams and secrets are not started and are stuk so a rpoblem is present"

## Context (diagnosed by the root)
`ness open` launched four children in a row. For two of them (seams, secrets) the first prompt sat in Claude Code's input box, unsubmitted; the agents stayed idle. The worker then treats the box text as an operator draft and never types (R-WRK-014), so nothing recovers. `herdr agent send-keys <name> enter` did NOT submit it; `herdr pane send-keys <pane> Enter` did.
Cause: internal/session/launch.go `prompt` calls `Runtime.Prompt` (internal/atom/herdr/herdr.go: `herdr agent prompt PANE TEXT` without `--wait`), which returns once the text and Enter are written, whether or not the client took the Enter (Claude Code still starting up drops it). Nothing checks the submission.
herdr 0.8.2: `herdr agent prompt <target> <text> [--wait] [--until STATUS]... [--timeout MS]`; per its docs a prompt from a non-working state must produce a lifecycle change within 5 s, else `agent_prompt_stalled`.

## Result owed
1. The launch confirms the submission: e.g. `agent prompt --wait --until working` (a short timeout), and on `agent_prompt_stalled` (or still idle with the prompt text in the box) press Enter with `herdr pane send-keys PANE Enter` and confirm again, a bounded number of times; then fail the phase loudly so `--resume` retries, never a silent idle session. Verify live with a real herdr: start a throwaway Claude session in a new tab of THIS workspace (`wS`), reproduce if you can (launch several back to back), close only the tabs you created.
2. The fake herdr in internal/testkit/herdr.go learns `--wait --until` and a stalled prompt (Enter dropped) so the fix has a judge test.
3. Spec: amend R-SESS-025 and add ids from R-SESS-150 up. Also evaluate whether the worker should recognise its own launch prompt left in the box (not an operator draft) and submit it; record the ruling.

## Limits and autonomy
Act within the brief. Tier standard.

## Rules for this session
- Never read or grep anything under `docs/discovery` (exclude it from every search).
- Gate: `mise exec -- make validate`; check its exit status directly, never pipe it into tail before a commit.
- No `git stash`, no pkill, no `--no-verify`. Do not push. Integrate into the manager branch with `ness integrate` when done, then hand off.
- Two sibling sessions run in parallel (withdraw, integrate-fixes, seams, secrets, prompt-submit, autonomy). Before picking a new requirement id, grep the spec and pick from your own reserved range below, to avoid duplicate ids.
- Record decisions in your session decisions.md, and a report.md with evidence and lessons at handoff.
