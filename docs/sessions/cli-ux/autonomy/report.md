> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# autonomy: report

## Delivered
- **N5** `ness-entry` "## Autonomy" gains "When your brief states no level, use this project's
  default:" and the single slot `autonomy-default`. Core sets it (core profile.toml →
  `profiles/core/autonomy-default.md`, "**act within the brief**."); a local profile replaces it
  with `"ness-entry#autonomy-default" = { set = "..." }`. R-PROF-080, tested in
  `core_text.txtar` (core default rendered; a local `set` replaces it).
- **N6** `ness-program` "## Autonomy grant" opens with "The grant is a plan the operator approved
  in advance (CX-18): its checkpoints are skipped until the work leaves its scope." R-PROF-081,
  tested in `builtin_program.txtar` (default grant and a local grant).
- **Evaluation harness** `evals/autonomy/eval.py`, `make eval-autonomy` (not in the gate;
  `EVAL_ARGS="--rev REV --runs N --only S1,S3"`, `--rescore OUT` re-scores existing runs).
  This repository's rendered skills re-synced.

## Evaluation
Design (sent to the manager before the first run, m-frpfosey): `specs.md`. Real client
`claude -p` 2.1.296, Opus 5.5 medium, bypassPermissions; fresh scratch ness project per run made
by `ness init` from the binary built at the arm's revision; a `ness` shim records calls. Before =
HEAD 9309b8b, after = this branch. 6 runs per scenario per arm.

| scenario | before | after |
|---|---|---|
| S1 no level, project default "report, then act" | 0/6 | 6/6 |
| S2 plan first | 6/6 | 6/6 |
| S3 relayed "report to me, then fix" | 6/6 | 6/6 |
| S4 grant covers the wave (fixture v2) | 6/6 | 6/6 |
| S5 wave holds work outside the grant (fixture v2) | 6/6 | 6/6 |

Reading the numbers honestly:
- N5 is the only measured gain. Before, a project could not state a default (sync refuses the
  unknown slot), and all 6 runs fixed first and reported only at handoff. After, all 6 sent an
  fyi naming the step, then fixed and passed check.sh.
- N6 shows no measured effect: S4/S5 are at ceiling on both arms, because `ness-entry` already
  says an approved plan skips its checkpoints. It is kept because the operator approved it and it
  does not hurt any scenario; this evaluation does not show it helps.
- S3 (F8) does not reproduce with the current CX-18 text: "me" stays the operator 6/6 before.
- No scenario fails after the change, so no wording iteration was run.

Iterations of the evaluation itself (both arms always re-scored or re-run identically):
1. S4 fixture v1 (the grant cited D3 approving r2 while the map was r3; no ledger) scored 1/6
   before and 2/6 after: agents rightly refused to treat an inconsistent record as approval. That
   was the fixture's fault. v2 has a consistent ledger D1-D3 (S5: D4 adds billing after D3);
   both arms re-run on v2 (table above). v1 S5 was 6/6 on both arms.
2. The scorer read `ness send ... && sed -i app.conf` in one Bash call as edit-before-send; it now
   compares offsets inside a call. Re-scoring changed after-S1 from 4/6 to 6/6, before unchanged.
3. The shim answered `ness open --help` with "opened"; it now prints usage (no score affected).

Cost: $8.99 (before) + $9.02 (after) + $4.37 + $4.20 (S4/S5 v2) + $0.26 smoke = $26.84, 85 runs,
about $0.30 a run.

Evidence: run dirs (transcript, shim log, verdict) were in the session scratchpad
`before/`, `after1/`, `before-g2/`, `after-g2/`; per-run verdicts reproduce with
`make eval-autonomy`.

## Gate
Two full `make validate` runs failed only on testscript timeouts ("context deadline exceeded",
SIGQUIT), a different set of scripts each time, with load 25-40 on 4 cores (the evaluation and
sibling gates). `go test -race -count=1 -timeout 40m ./internal/cli/` passed (504 s); the two
edited scripts pass alone. The full gate runs at `ness integrate`, scheduled by the manager.

## Lessons
- An evaluation fixture is a claim too: S4 v1 measured my inconsistent ledger, not the text.
  Read the failing transcripts before believing a low score, and re-run both arms on a fix.
- Score ordering by position inside a command, not per tool call: agents chain send && edit.
- Do not run evaluation rounds and a gate at the same time on this box.
- Run the background job with the tool's background flag; a `( … &)` launch gives no wake-up.
