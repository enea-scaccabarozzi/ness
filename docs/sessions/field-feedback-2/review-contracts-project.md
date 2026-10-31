> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Review: contracts-project (3f331c7)

Scope: `git show 3f331c7` in `.claude/worktrees/agent-a4fce1d3e7ac9627b`, against
`docs/spec/contracts.md` R-CTR-040..044 and the build notes. `go test ./internal/{session,cli,config}`
passes. I checked the bypasses with a throwaway testscript in the scratchpad, run against this commit.

## High

1. **A session lock overrides a project-wide path, so one hand-written line removes the lock.**
   `internal/session/contract.go` `FrozenFiles` (lines ~213-228): an entry from a session lock wins,
   and the project entry is added only when `!seen`. `CheckContracts` then skips `Amended`. Probe:
   on the branch, edit a base scenario of the glob-frozen `features/pay.feature` (check exits 1).
   Then run `mkdir -p docs/sessions/x && printf "amend features/pay.feature\n" > docs/sessions/x/contract.lock`
   through Bash. Result: `ok 1 frozen files`, exit 0. Nothing says `missing`, `changed` or
   `differs`, and nobody checks who owns the lock. The guard stops Edit/Write on lock files, but
   it does not stop Bash. The same route works without forging anything: a path that sat in a
   session lock before a glob covered it can be reopened with `ness contract amend` by that lock's
   owner or manager. That breaks the spec rule that project-wide paths are amended only on base.
   The precedence predates this commit (literal entries had it too). Globs make it matter much
   more, because consuming projects already hold session locks on `.feature` files. Fix: check
   project-wide entries on their own, never shadowed by a lock entry and never opened by `Amended`,
   and make `amend`/`freeze` refuse any path that `ProjectFrozen` covers.

2. **The branch's own `.ness/ness.toml` decides what is frozen.** `requireFeature` → `loadProject`
   reads the worktree's config, so `[contracts] frozen`, `prefix`, `base` and `features` all come
   from the branch under test. Probe: `frozen = []` on the branch gives `ok 0 frozen files`, exit 0.
   Other ways to weaken the lock the same way: add a `prefix` glob (a whole-file `.md` becomes
   append-only), narrow a glob, point `base` at a ref that agrees with the branch, or drop
   `"contracts"` from `features`. This also predates the commit, and no guard rule protects
   `.ness/ness.toml` for contracts (R-HOOK-024 covers launch limits only). The commit widens what
   one config line controls but adds no defence. Fix: read `[contracts]` (and `base`) from
   `git show <base>:.ness/ness.toml` for `check`, `ProjectFrozen` and the guard. Or freeze
   `.ness/ness.toml` whole-file unless the operator amends it.

## Medium

3. **Comparing with the base tip instead of the merge-base fails branches that are not behind.**
   `ProjectFrozen`, `CheckContracts` (`BaseBlob(base)`) and `FindLocksTolerant(cfg.Base)` all read
   the base branch tip. Suppose base adds `features/new.feature`, amends a scenario, or removes or
   fixes a legacy lock after the branch forked. A branch that never touched those files then fails:
   `file is missing`, `scenario edited`, or `malformed contract.lock` (the lock is now "added on
   the branch"). These are false failures until the branch merges base. The build notes admit it,
   but the spec (R-CTR-040/044) says "at the base branch" without a word on staleness, and the
   godoc and the txtar header say "merge-base" (`contract.go` `FindLocksTolerant` comment;
   `contract_project.txtar` line 5). Either use the merge-base, as `HistoricalFrozen` already does,
   or fix the docs and the spec to say tip and require a merge.
4. **The guard and `check` disagree on edge literals.** `Guard.projectEntry` gives a literal
   `LockKind` without reading base. Two cases differ: (a) a literal `.feature` or prefix-`.md`
   absent at base gets a soft warning from the guard, but `check` fails closed (literal not in
   base); (b) a prefix-`.md` that is empty at base gets a soft warning, but `check` treats it as
   whole-file. In both, the agent is told "adding is fine" and then fails the gate. Fix: have the
   guard reuse `projectEntry(g, root, base, rel, prefix)` and read the mode from `Lenient(f.Sha)`.

## Low

5. **Test gaps.** No test covers a glob-frozen file deleted or renamed out of the glob. My probe
   shows it is caught (`differs ... file is missing`), but nothing pins it. No test covers how a
   project entry and a lock entry overlap (finding 1). No guard test covers a literal `.feature`
   entry. The txtar was written after the code (the build notes say so), so it was never seen
   failing.
6. **The guard test passes when the guard errors.** In `TestGuardWarnsOnProjectScenarioAndPrefixLocksAndRefusesWholeFile`
   the `new.feature` assertion only checks `r == ""`. A guard error also returns `""` (it writes
   to `os.Stderr` and allows the edit). The other assertions in the same test would catch a global
   error, so the risk is small, but the case would read more honestly with `Warn` sent to a buffer
   that is asserted empty.
7. **The history walk skips by `seen` blob.** In `lockPathsAt`, a malformed blob first seen at the
   merge-base is marked seen. An identical copy at another path in a branch commit is then skipped
   with no error. The worktree walk still catches the copy while it exists, and it holds no paths,
   so there is no bypass. Worth a comment.
8. **Mode tokens written as string literals.** `contract.go` `record` matches on the string literals `"prefix:"`/`"scn:"`
   rather than the constants (`prefixTag`, `scenarioTag`, not exported). Export them or have
   `LockKind` return a small enum.

## What holds

- Spec R-CTR-040..044 matches the code, apart from the merge-base wording (finding 3).
- Weakening a base scenario directly is caught; so are deleting the frozen file or renaming it
  out of the glob.
- Making a valid lock malformed fails closed, because its blob then differs from base. Adding a
  malformed lock fails closed; removing a legacy lock passes, as the spec says.
- A literal entry absent at base fails closed.
- The prefix and scenario modes reuse the session-lock code, so the rules stay the same.
