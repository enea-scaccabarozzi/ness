# Model-select eval set

Scores the subagent rung selectors (`declared`, `jev`) on real Agent calls, labelled by an
independent labeller, and calibrates the Jev combination weights. Data (prompts, labels, picks)
stays under `.private/model-select/` (gitignored); only this harness and aggregate numbers are
committed.

| Step | Command | Output |
|---|---|---|
| 1. Extract | `python3 -I evals/model-select/extract.py ~/.claude/projects/*research-lab* --n 150 --seed 7` | `calls.jsonl`: prompt, description, type, requested model/effort, actual model/effort, turns, cost |
| 2. Label | `python3 -I evals/model-select/label.py --jobs 6` (resumable; costs opus/high calls) | `labels.jsonl`: kind, lowest adequate rung, why |
| 3. Spot check | `python3 -I evals/model-select/label.py --spot 20` | `spot.md` for the operator |
| 4. Score | `python3 -I evals/model-select/score.py --policy declared --ness PATH`; for jev, run with `--cwd` at a repo whose config has `[models] policy = "jev"` | `picks-<policy>.jsonl` (jev: also `picks-jev-nokind.jsonl`, with the answers and wall time); exact / over / under rates and cost-weighted error, also for the rung actually used; jev fallback rate and latency |
| 5. Calibrate | `python3 -I evals/model-select/calibrate.py` (no Jev calls) | the `DefaultWeights` literal for `internal/atom/jev`; threshold sensitivity; the best kinds table for `declared` with leave-one-out and bootstrap |

Rung cost proxies ($/task, research-tiering.md): bulk 0.01, economy 0.07, lean 0.12, standard
0.21, deep 0.29, frontier 0.60, apex 1.19. An under-rung pick costs the wasted attempts, retrying
one rung up each time until the labelled rung (one rung under costs the picked rung); calibration
weights under-rung errors 2x. `declared` receives the labelled kind, so it is scored at its best
case; jev is scored with the labelled kind declared and with none (Jev judges alone).

Each script checks itself: `python3 -I evals/model-select/<script>.py --selftest`.
