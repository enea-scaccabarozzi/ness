> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# autonomy

## The operator's words (verbatim)
"Regarding autonomy machinery i approve both N5 and N6 now, but i want this changed to be evaluated, not simpe shoot and pray"

## Context
Autonomy today is text only (decision CX-18; profiles/core/skills/ness-entry/SKILL.md "## Autonomy"; profiles/core/skills/ness-launch/SKILL.md brief checklist; profiles/builtin/program/skills/ness-program/SKILL.md "## Autonomy grant" single slot, default text profiles/builtin/program/autonomy.md). No config key, no store field, no enforcement. A brief without a stated level leaves the agent to guess (in practice "act within the brief"); field run 1 (docs/sessions/cli-ux/infra-lab-feedback.md, F8) lost the operator's "report to me, then fix" checkpoint that way.
- N5: a single slot `ness-entry#autonomy-default` holding the project's default level, used when a brief states none; core default text "act within the brief". A project sets it from its local profile.
- N6: one sentence in ness-program: the autonomy grant is a plan approved in advance (CX-18): its checkpoints are skipped until the work leaves its scope.

## Result owed
1. An evaluation designed BEFORE the change and run before and after it, on the real client and models sessions use (Claude Code headless, `claude -p` with the rendered skills in a scratch copy of a ness project; never in this repo's live tree). Scenarios at least: brief with no level + project default "report, then act" (does it announce steps and keep going?), "plan first" (does it stop after the plan, changing nothing?), a relayed "report to me, then fix" (does "me" stay the operator?), a program grant covering the work vs work outside it. Define observable pass criteria per scenario (tool calls made, files changed, ness send items / text emitted), several runs each, report pass rates before vs after. Keep the cost reasonable (state it). The harness may live under a test-only path (e.g. evals/autonomy/) if it is reusable, with a make target not in the gate.
2. N5 and N6 implemented (slot marker, core text, sync test, re-render), spec ids from R-PROF-080 up.
3. If the evaluation shows the text fails a scenario, iterate on the wording and report each iteration's numbers; do not claim an improvement the numbers don't show.

## Limits and autonomy
Report, then act: before running the evaluation, send your manager the scenarios and pass criteria (fyi) and continue without waiting. Tier standard.

## Rules for this session
- Never read or grep anything under `docs/discovery` (exclude it from every search).
- Gate: `mise exec -- make validate`; check its exit status directly, never pipe it into tail before a commit.
- No `git stash`, no pkill, no `--no-verify`. Do not push. Integrate into the manager branch with `ness integrate` when done, then hand off.
- Two sibling sessions run in parallel (withdraw, integrate-fixes, seams, secrets, prompt-submit, autonomy). Before picking a new requirement id, grep the spec and pick from your own reserved range below, to avoid duplicate ids.
- Record decisions in your session decisions.md, and a report.md with evidence and lessons at handoff.
