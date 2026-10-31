> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# Decisions: model-select

## MS-D1: tier standard, size full, built in this session with subagents (2026-10-21)
Root on opus/medium. No child sessions (simple-build-process); subagents per plan task. Evidence goes to the
private docs/discovery/findings/model-select/.

## MS-D2: approach approved; strict refusal first (2026-10-21)
Operator: "fully aligned" with the ladder/base/selector/hook approach; start with refusal on above-recommendation
subagent calls and loosen to log-only only if data shows too much friction (`models.enforce`).

## MS-D3: one ladder of (model, effort) rungs, one offset (2026-10-21)
The chat proposal spoke of separate model and effort offsets. Research shows tiers are points on one cost/quality
frontier and effort scales are model-calibrated, so a rung carries both and selectors return one offset. The Jev
combination still weighs capability signals and depth/checkability signals separately before producing it.

## MS-D4: Jev ships as a built-in Go atom, not an external script (2026-10-21)
The brief imagined a built-in optional script. A Go atom (plain HTTP, no SDK) runs inside the hook and `ness pick`
without a Python runtime in consumer projects and is tested against a fake server. Swappability is kept by the same
seam's `command` policy, which runs any project script. `ness pick` is the executable the agent calls.

## MS-D5: names of the new rungs (2026-10-21)
bulk, lean, deep and apex join economy, standard and frontier, so existing `--tier` habits and briefs keep working.

## MS-D6: labels accepted, kinds table after calibration, MS-D3/D4 confirmed (2026-10-21)
Operator: "I trust opus" (no spot check of the eval labels); the default kinds table is decided after calibration;
"Fine on both" for MS-D3 and MS-D4.

## MS-D7: kinds table A and the rollout (2026-10-21)
Operator chose option A after the calibration (eval-results.md): search −2, run −2, implement 0, debug 0,
research +1, review +1, verify +1, design +1; policy stays `declared` (Jev opt-in), DefaultWeights unchanged.
Rollout authorized for this round, as field-feedback-2 did: install the binary from main, restart the workers, sync
the infrastructure lab, and message the live research-lab and production-backend roots to integrate it in their sessions.
