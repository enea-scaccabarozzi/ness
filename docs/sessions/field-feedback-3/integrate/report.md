> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Report: integrate (child of field-feedback-3)

## Done
Every item in the brief is built, specified and tested (specs.md has the design):
1. **The target lock queues** (R-SESS-164). A second integrate into the same target logs
   `ness: waiting for <holder>'s integrate into <target>` and waits instead of refusing.
   `ErrBusy` and `rule.GateBusy` are removed.
2. **A moved target is folded again** (R-SESS-165). After the gate, integrate re-reads the
   target. If it moved, integrate folds onto the new tip and compares the new fold with the one
   the gate passed (`diff --no-renames`):
   - only `sessions` paths differ: it lands without a second gate;
   - anything else differs: the gate runs once more;
   - a second code move or a conflict: `ErrTargetMoved`, a plain message, exit 1.

   I questioned the brief's rule: comparing the folded trees is better than checking each new
   commit. Ceiling: a gate that reads the records is not re-run after a records-only move.
3. **The push** (R-SESS-166). It runs with ssh keepalives unless the project set
   `core.sshCommand`, `GIT_SSH` or `GIT_SSH_COMMAND`. A dropped connection is retried once,
   matched on git and ssh's own lines, so a hook that prints "connection reset" is not re-run.
   The spec says a pre-push hook that repeats the gate is the project's to skip.
4. **Contracts.**
   - R-CTR-055: `check` reports files added under a frozen glob. The guard still allows creating
     them.
   - R-CTR-056: `ness contract amend PATH --reason R --authorized WORDS` opens a project-wide path
     using the operator's typed words. The trail is a decisions.md line; check and the guard count
     these lines across all records against the merge-base.
   - The contracts skill text and R-CTR-040 and -045 are updated.
5. **Close** (R-SESS-167). A child whose branch is already in its base (an ancestor, a hand fold
   or a rebase, judged by `merge-tree`) closes without `--discard`, and its base sha is recorded.
   Containment is consulted only when nothing is recorded, so a revert made after an integrate is
   never lost. The refusal for uncommitted files now names `--keep-work`.

## Evidence
- Acceptance and regression tests were written by separate subagents from the implementers:
  - `internal/session/integrate_{moved,queue,push}_test.go`;
  - testscripts `close_{contained,handfold,revert}`, `contract_{added,project_amend,project_archive}`
    and `integrate_push_ssh`.

  Against HEAD the new-behaviour tests fail. Each regression test also fails when its fix is
  reverted (checked for H1, M1, M2, M6 and M7).
- Full package test binaries pass with `-race`. Both lint configs report 0 issues, the audit and
  the e2e vet pass.
- An independent review (ness-review) found 1 high and 3 medium issues, all fixed. The deferred
  lows are listed under Next.
- **Race reproduction** (a scratch repo, operator integrate; the gate's first run commits onto
  the checked-out `main`):
  - old binary: both the records-only move and the code move fail with
    `fatal: Not possible to fast-forward, aborting.`. The gate run is lost and `main` lacks the
    feature.
  - new binary, records-only move: `main moved during the gate by session records only; landing
    without a second gate`, 1 gate run, landed.
  - new binary, code move: `running the gate again on the new main`, 2 gate runs, landed.
- **Push drop reproduction** (a bare remote over a fake ssh that drops the first receive-pack,
  plus a slow pre-push hook):
  - old binary: `moved to … but the push failed …; push it: git push origin main`, exit 1, remote
    behind.
  - new binary: `the push lost its connection; retrying once`, exit 0, remote = local.

## Now
Integrating through the gate.

## Next
- Deferred lows from the review:
  - the pr integrator's source push has no keepalive or retry;
  - the target lock waits with no time limit;
  - `--keep-work` on a contained, dirty branch records the base sha;
  - a re-fold over untracked files the first gate left can still print git's own text;
  - copying a record dir (not moving it) raises the amend count and can reopen a path.
- Out of scope, seen: on promotions the contract base is fixed to `cfg.Base` (v3 C3).
- `/tmp` is 98% full, mostly other projects' Claude scratch dirs (7 GB). `go test -race` can fail
  on it, which is the operator's to clear.

## Lessons
- Fixing the interface (names, log lines, argv) in specs.md before the work let one subagent write
  the judge while three others implemented in parallel, and the tests matched on the first run.
- Parallel implementers in one worktree break each other's build. Each verified in a copy of
  HEAD with only its own files, which worked; say so up front next time.
- Testscripts refuse work dirs outside /tmp, so with /tmp full: build the test binaries with
  GOTMPDIR on disk, then run them with the normal TMPDIR.
- The review caught a data-loss bug (containment overriding a recorded integration) that the
  first acceptance tests missed. Ask the test writer for "must refuse" cases next to every new
  "now accepts" rule.
