> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# integrate-fixes decisions

- **IF-1** The squash subject comes from `ness integrate --subject`, required in squash mode for
  every integrate (child into manager too): a child's subject becomes a nested commit in the
  root's squash, so it reaches the changelog. Malformed or missing → exit 4 before the fold.
  Valid = `^[a-z]+(\([^()]+\))?!?: \S` (generic, no project type list). `--subject` in another
  mode is refused rather than ignored.
- **IF-2** The body lists folded subjects as release-please `BEGIN_NESTED_COMMIT` blocks, not
  plain paragraphs: release-please's top-level split (checked in its src/commit.ts) knows only the
  eleven standard types, so `sessions:` would never reach "Running sessions". The release
  workflow's sessions check reads `%B` for the same reason.
- **IF-3** Re-integration after a squash: `git merge-tree --write-tree --merge-base=<T>` where T
  is the source sha in the newest `Ness-Integrates` trailer on the target that is an ancestor of
  the source; git's own base is used when the source already merged that squash commit. Sound
  because T's content is in both sides. The body excludes `^HEAD ^T`. A fold whose tree equals
  the target's is `ErrNothingToIntegrate`. Needs git ≥ 2.40.
- **IF-4** R-QA-040..043 already existed, so the quality ids are R-QA-044 (pre-push) and R-QA-045
  (commit-msg allowlist); R-QA-009 amended. D44 records the squash default and the gate split.
- **IF-5** The go-strict seed Makefile keeps `-count=4` in `make validate` (R-BLT-054, projects
  without CI); only this repository's Makefile moves it to `make repeat`, run by CI.
- **IF-6** Existing testscripts that integrate on the default mode now pass `--subject`; none was
  switched to merge mode, so the squash path is what they exercise.
- **IF-7** (m-emtclz54 item 1) golangci-lint runs with `--allow-serial-runners` (checked in
  2.14.0) in this Makefile, the go-strict seed Makefile and the go-strict pre-commit hook, and in
  `builtin_go_strict.txtar`'s direct runs: parallel sessions queue on its lock instead of exiting 3.
- **IF-8** (m-emtclz54 item 2) Added: a project-wide gate lock, `<common>/ness/gate.lock`, a
  blocking flock taken by `session.Integrate` around the gate only (after the target lock; the
  worktree lock is never held across it). A waiting gate logs one line (R-SESS-145). It covers
  gates run by `ness integrate`; a `make validate` started by hand does not take it, so sessions
  should run the gate through integrate rather than by hand under load. Measured: a hand-run gate
  at load ~36 failed with 39 testscript timeouts and no real failure.
