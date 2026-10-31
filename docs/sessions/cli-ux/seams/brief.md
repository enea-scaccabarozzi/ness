> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# seams

## The operator's words (verbatim)
"the half-day composability task is even smaller and should simply do N1, N3."

## Context
From the composability analysis (decision CX-24 in docs/sessions/cli-ux/decisions.md):
- N1. Close the composition-root leak. Claude-specific behaviour is wired around the client seam (`session.Client`) instead of through it: internal/cli/worker.go (around lines 185-188) hard-wires `"claude": {claude.Read, claude.ResumeText, claude.RenameText}` and `claude.Usage`; internal/cli/close.go (around line 210) calls `claude.OperatorSaid` for `--authorized`; internal/cli/atoms.go uses `claude.ProjectDir` for transcripts. So a Codex session is silently never woken ("client codex is not delivered to") and a Codex root can't self-close. Move this behind the client atom so internal/cli/atoms.go is the only file in cli that names an implementation (it already claims to be). A client without worker support or without an authorization check must be reported explicitly (a `ness doctor` line, and a refusal naming the client), not skipped silently. No behaviour change for Claude.
- N3. One list per set of names: seam/atom names, feature names and hook events are each written in several places (config.validate, enumKeys, profile.atomSeams, the atoms.go switch; the feature list in config.validate and the R-PROF-009 check; hook events in internal/cli/hook.go and internal/sync/render.go). Export each once and have the others read it. Existing refusal tests must keep passing.
Keep interfaces at three methods or fewer (ness-interface-design), consumers declare them.

## Result owed
The two refactors, gate green, a doctor line for a client without worker support with a test, spec ids from R-CFG-070 / R-WRK-090 up where needed, report.

## Limits and autonomy
Act within the brief. Tier standard.

## Rules for this session
- Never read or grep anything under `docs/discovery` (exclude it from every search).
- Gate: `mise exec -- make validate`; check its exit status directly, never pipe it into tail before a commit.
- No `git stash`, no pkill, no `--no-verify`. Do not push. Integrate into the manager branch with `ness integrate` when done, then hand off.
- Two sibling sessions run in parallel (withdraw, integrate-fixes, seams, secrets). Before picking a new requirement id, grep the spec and pick from your own reserved range below, to avoid duplicate ids.
- Record decisions in your session decisions.md, and a report.md with evidence and lessons at handoff.
