> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Specs: integrate (child of field-feedback-3)

## What is built
1. **Target lock queues** (R-SESS-164). `lockTarget` waits instead of refusing. The holder
   writes who it is (session id, or "the operator") into the lock file; a waiter logs
   `ness: waiting for <holder>'s integrate into <target>` and blocks. `ErrBusy` goes away.
2. **A moved target keeps the gate when only records moved** (R-SESS-165). After the gate passes,
   Integrate re-reads the target (after a fresh `catchUp` when pushing). If it moved, it resets the
   throwaway to the new target and folds again. Then:
   - the new fold differs from the gated fold only under `sessions` (ness.toml): land, no gate;
   - otherwise run the gate once more (its NESS_BASE is the new target), then re-check;
   - a second non-record move, or a conflict, returns a plain error naming the target, what moved
     it and the next step; no raw git text.
   A move between the re-check and the fast-forward re-enters the same loop (bounded).
   - *Rule questioned:* the brief asked "every new target commit touches only the record
     directory". Comparing the two **folded trees** is better: it is exactly "the tree that lands
     differs from the tree the gate passed only in records", covers a commit and its revert, and
     a fold that resolves differently. Ceiling: a gate that reads the record directory (link
     checks over `docs/sessions`) is not re-run for a record-only move.
   - The pr integrator never moves the local target: no re-check.
3. **Push survives a long pre-push hook** (R-SESS-166). The push runs with
   `-c core.sshCommand='ssh -o ServerAliveInterval=30 -o ServerAliveCountMax=10'` unless the
   project set `core.sshCommand` itself (GIT_SSH_COMMAND still wins, as git does). A push whose
   error reads as a dropped connection (closed by remote host, broken pipe, connection reset,
   remote end hung up, early EOF) is retried once. The docs say a pre-push hook that repeats the
   gate is the project's to skip.
4. **Contracts.**
   - R-CTR-055 (CHANGES R-CTR-040's "a file new on the branch is not frozen"): `contract check`
     reports `added\tPATH\t.ness/ness.toml (new under frozen glob G)` for each file of the worktree
     (`ls-files -co --exclude-standard`) that matches a frozen glob and is absent at the merge-base.
     The guard keeps allowing creation (hook untouched); check is the judge.
   - R-CTR-056: `ness contract amend PATH --reason TEXT --authorized WORDS` opens a project-wide
     path. A session needs the operator's words, checked like `close --authorized` (R-SESS-121);
     the operator caller amends on their own authority. It appends to the caller's
     `decisions.md`: `Amend PATH (project-wide): REASON [operator: "WORDS"] (DATE)`. `check` and
     the guard accept a differing or added project-wide path when some `decisions.md` under
     `sessions` holds more such lines for it than at the merge-base (same trail as R-CTR-021:
     a record, not a security boundary). `freeze` still refuses project-wide paths.
5. **Close of a contained child** (R-SESS-167, amends R-SESS-064). With no integration recorded,
   close accepts a branch whose merge into its base changes nothing in the base's tree
   (`merge-tree --write-tree`), which covers ancestry, hand folds and rebases; it records the
   base's sha as the integration and says so. `CheckLanded` accepts the same containment. The
   uncommitted-files refusal names `--keep-work REASON`.

## Tests (judges written before the code, by a separate task)
- session_test (real git, scratch repos): a waiter logs the holder and lands after release; a
  record-only move during the gate lands without a second gate (gate counter = 1); a code move
  re-runs the gate once (= 2); a conflicting move returns a plain error, target untouched; a
  dropped push (fake ssh / pre-push hook simulating the drop) is retried once and succeeds.
- contract: an added file under a frozen glob fails check; an authorized project amend passes
  check; a session amend without words, or words not in the transcript, is refused.
- close: a contained child (ancestor; hand-folded squash) closes without `--discard` and records
  the base sha; an uncontained one is still refused.

## Out of scope (seen, not done)
- The contract base fixed to `cfg.Base` on promotions (v3 C3): not in the brief.

## Interfaces (fixed, so tests and code can be written apart)
- `session.IntegrateSpec` gains `Holder string` (written into the target lock file; cli passes
  the session id, or "the operator") and `Records string` (the `sessions` dir from ness.toml,
  repo-relative; "" means no move is record-only).
- Waiting log line: `ness: waiting for <Holder>'s integrate into <target>` (when the holder is
  unreadable: `ness: waiting for another integrate into <target>`). `session.ErrBusy` is removed.
- Record-only re-fold log line: `ness: <target> moved during the gate by session records only; landing without a second gate`.
  Code move: `ness: <target> moved during the gate; running the gate again on the new <target>`.
- `session.ErrTargetMoved`: a re-fold conflict or a second non-record move. Its message is plain
  ("<target> moved while the gate ran ... integrate again" / naming conflicting paths), never raw
  git output (no "exit status", no "hint:"). cli maps it to exit 1 (problems).
- Push argv: `git -c core.sshCommand=ssh -o ServerAliveInterval=30 -o ServerAliveCountMax=10 push origin <target>`
  (the `-c` pair only when `git config --get core.sshCommand` is empty). A dropped-connection
  error is retried once and logs `ness: the push lost its connection; retrying once`.
- Close: `session.CheckClose` passes with no recorded integration when the branch is contained
  in its base; cli records the base sha with `SetIntegration` and prints
  `nothing to integrate: <branch> is already in <base>; recorded <sha>` before `closed ...`.
- `ness contract check` line for an added file: `added\t<PATH>\t.ness/ness.toml (new under frozen glob <G>)`.
- `ness contract amend PATH --reason TEXT --authorized WORDS` for a project-wide path; decisions
  line `Amend <PATH> (project-wide): <REASON> [operator: "<WORDS>"] (<YYYY-MM-DD>)`; prints
  `amending <PATH> (project-wide, by the operator's words); it lands with this branch`. Without
  `--authorized` a session is refused (exit 4) with the route; the operator caller needs no words.
