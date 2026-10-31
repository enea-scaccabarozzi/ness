> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# Report: model-select

## Delivered
Model and effort are now chosen per step by code, around one per-project knob, and enforced across compaction.
- **Ladder** (MS-D3, MS-D5): seven rungs, cheapest first, each a (model, effort) pair with Claude and Codex
  rows: bulk, economy, lean, standard, deep, frontier, apex. `[models] base` (default standard) and optional
  `ceiling`. A project moves its whole spend by moving `base`.
- **Selector primitive**: `[models] policy` = `declared` (default, offline: `kind:` → offset table), `command` (any
  project script, JSON in/out) or `jev` (optional built-in, MS-D4). It is the swap point for any project. `ness pick`
  prints the rung for a task; `--explain` shows Jev's answers.
- **Kinds table** (MS-D7, from the eval): search −2, run −2, implement 0, debug 0, research +1, review +1,
  verify +1, design +1. Named agents follow it: ness-sweep economy, ness-review, ness-verify and ness-design deep.
- **Enforcement**: the pre-tool hook judges every Agent call from a bound session.
  - It allows calls at or below the recommendation, and one rung above with `escalate: <reason>`.
  - It refuses anything higher, anything above the ceiling, off-ladder pairs, and general-purpose calls with no pair.
  - `models.enforce = "log"` is the loosening switch (MS-D2).
  - Every decision is stored.
- **Other paths**:
  - `ness open`: `--tier` is optional (`--kind` picks), a tier above the recommendation needs `--reason`, and
    `--tier up|down` resumes one rung over or under.
  - Post-compact text and the ness-entry and ness-launch skills carry the rules.
  - `ness stats --by rung|role` shows cost by rung and the hook's verdict counts.
- **Jev selector**: one request of 11 narrow questions (Choice kind, 5 Scores, 5 Nouls), combined in Go.
  - The model is pinned to `jev-1.14.0`.
  - The key comes from an env:/secret: reference only.
  - Answers are validated.
  - Any failure or low confidence falls back to declared.
  - `ness doctor` probes the pinned model with one request.

## Evidence
- Research (docs/discovery/findings/model-select/):
  - The research lab cost $1,807: 21 of 24 sessions ran on Fable, and their own turns were 62% of the bill.
  - Effort is a lever as strong as model: Opus/medium ≈ Fable/high at one sixth of the cost.
  - The operator's hypotheses: "model is the main lever" is contradicted; "medium 99%" is unsupported; targeted
    escalation and moving the base are supported.
- Eval (eval-results.md): 150 research-lab Agent calls, labelled by Opus/high.

  | | error (2x under) |
  |---|---|
  | declared, old table | 0.175 |
  | declared, new table | ≈0.05 (leave-one-out 0.056; best in 1,000/1,000 resamples) |
  | jev, kind given | 0.165 |
  | jev alone | 0.186 |

  - Jev: p50 0.32s, p95 0.40s, $0.00008 per pick, no errors; it fell back on 37% of calls (low confidence).
  - Recalibrated weights failed cross-validation, so DefaultWeights is unchanged.
- Tests: every task had its acceptance tests written by a separate judge subagent before the code. Three fresh
  reviews (plans 1, 2, 3) found no high findings; all medium ones are fixed (12a5f34, 894ab86, 9ee3331).
  `go test ./...`, `go vet ./...` and `audit.py` are clean; the hook was checked by hand with real payloads.

## Open
- Labels lean high: the labeller never chose bulk or frontier, and sessions finished on rungs it called too low.
  - Watch `ness stats --by rung` and the refusal counts after rollout.
  - Switch `enforce = "log"` if friction shows.
- implement 0 is the weakest table change (61% of resamples); confirm on a production backend or Ness sample.
- The hook cannot judge Codex subagents (no spawn hook); Codex gets the ladder at open and in its agent defaults.

## Lessons
- A test build with a new store migration, run in a live repo, migrates the shared store and locks out the
  installed ness (worker included). Run schema-changing builds only in scratch clones. `ness sync --check` does
  not open the store.
- Verify an external API's live behaviour before trusting a check built from its docs. `/v1/models` lists only
  aliases, and `jev-latest` served an older version than the newest pin.
- A cost formula needs a self-test on multi-step under-shoots; the first harness went negative two rungs down.
- Fitting weights on 150 points looked like a 5% gain and lost under cross-validation; keep the cross-validation in
  the harness.
