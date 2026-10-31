> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# integrate-fixes

## The operator's words (verbatim)
"integrate should work as squash merge (more friendly to rel please)"
"You can go on the fixes"

## Context
The harness review (decision CX-23 in docs/sessions/cli-ux/decisions.md) found:
- CI on main is red: the `commits` job (profiles/builtin/git-conventional/commit-msg, run in .github/workflows/ci.yml) rejects `integrate session/... into main`, the subject ness's merge integrator writes (internal/atom/merge/merge.go; squash and pr use the same text). Session commits like `cli-ux: ...`, `delivery: ...` also fail it.
- The nightly fuzz job fails: `FuzzTOMLKeys` (internal/sync/tomlkeys_test.go) on input `[0]#\n0 = ""` (see `gh run list --event schedule` and the failed log).
- The gate (`make validate`, about 7 min, almost all in `go test -race -count=1` plus `go test -count=4`) runs at every integrate, again in the lefthook pre-push hook, then in CI. The pre-push length also makes `git push` die with SIGPIPE when GitHub drops the idle SSH connection.

## Result owed
1. Squash integration, friendly to release-please (see the repo's release-please config under .github): the commit that lands on the target carries a conventional subject (`type(scope): summary`) and a body that lists the folded commits. Decide where the subject comes from (a `--subject` flag on `ness integrate`, a line in the session record, ...) and what happens without a valid one (refuse, with a clear message). Make squash the default `integrate.mode` (merge/rebase/pr stay available); this repo uses the default. Handle the case of a branch integrated again after a squash (the source branch is no longer an ancestor of the target): it must not re-apply already-landed changes or conflict spuriously; the root keeps working on its branch after integrating into main.
2. commit-msg allowlist: accept ness's own integrate subjects for projects still on merge mode, with a test.
3. Fix the `FuzzTOMLKeys` bug; commit the failing input under internal/sync/testdata/fuzz/FuzzTOMLKeys/.
4. Lighter pre-push in go-strict's lefthook (build, lint, audit, sync check); the full test suite stays in the gate used by integrate and in CI; `-count=4` runs only in CI (amend R-QA-009 accordingly). Re-render with `ness sync`.
5. Spec ids: integrate in sessions.md from R-SESS-140 up, config in core-config.md from R-CFG-060 up, quality.md from R-QA-040 up. A decision in docs/design/decisions.md (next free D number) for the squash default.

## Limits and autonomy
Act within the brief. Tier standard. Do not touch the infrastructure lab (it sets its own integrate mode explicitly).

## Rules for this session
- Never read or grep anything under `docs/discovery` (exclude it from every search).
- Gate: `mise exec -- make validate`; check its exit status directly, never pipe it into tail before a commit.
- No `git stash`, no pkill, no `--no-verify`. Do not push. Integrate into the manager branch with `ness integrate` when done, then hand off.
- Two sibling sessions run in parallel (withdraw, integrate-fixes, seams, secrets). Before picking a new requirement id, grep the spec and pick from your own reserved range below, to avoid duplicate ids.
- Record decisions in your session decisions.md, and a report.md with evidence and lessons at handoff.
