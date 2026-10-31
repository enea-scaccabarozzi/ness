#!/usr/bin/env python3
"""Grid-search the Jev combination weights on saved `--explain` answers (no new Jev calls), and fit
the kinds table (offset per kind) for the declared selector on the labels.

Ports internal/atom/jev Combine and replays it over picks-jev.jsonl and picks-jev-nokind.jsonl for
every Weights in 0.4..0.9 step 0.1, minimising cost-weighted error with under-rung weighted 2x over
both runs, and prints the Go literal. A pick Jev answered below the router threshold (or with an
unknown kind) replays as the fallback it took. Also reports the threshold's effect and the best
kinds table for declared, with a leave-one-out error and a bootstrap of how stable it is.

    python3 -I evals/model-select/calibrate.py [--dir .private/model-select]
"""

import argparse
import itertools
import json
import os
import random
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parent))
from score import RUNGS, err  # noqa: E402

BASE = RUNGS.index("standard")
KINDS = {"search": -2, "run": -2, "implement": -1, "debug": 0, "research": 0, "review": 0, "verify": 1, "design": 2}
DEFAULT = (0.6, 0.6, 0.7, 0.7)
THRESHOLD = 0.8  # models.router_threshold default
GRID = [0.4, 0.5, 0.6, 0.7, 0.8, 0.9]
UNDER = 2.0


def clamp(i):
    return max(0, min(len(RUNGS) - 1, i))


def combine(a, w, kinds=KINDS):
    """Rung index for snake_case answers a (as `ness pick --explain` prints them) and weights
    w = (up_novelty, up_blast, down_mechanical, down_check)."""
    un, ub, dm, dc = w
    up = (a["novelty"] >= un or a["design"] >= un or a["blast"] >= ub) and a["check"] < 0.5 and a["kind"] != "design"
    down = a["mechanical"] >= dm and a["check"] >= dc and a["kind"] not in ("search", "run")
    return clamp(BASE + kinds[a["kind"]] + (1 if up else -1 if down else 0))


def replay(p, w, thr=THRESHOLD, kinds=KINDS):
    """The rung a saved pick gets under weights w and threshold thr: the fallback's index when Jev
    gave no answers, an unknown kind or a confidence below thr."""
    a = p.get("answers")
    if not a or a["kind"] not in kinds or a["kind_confidence"] < thr:
        return p["index"]
    return combine(a, w, kinds)


def loss(picks, w, under=UNDER, thr=THRESHOLD, kinds=KINDS):
    return sum(err(replay(p, w, thr, kinds), p["label"], under) for p in picks) / len(picks)


def best(picks):
    return min(itertools.product(GRID, repeat=4), key=lambda w: (loss(picks, w), w))


def fit_kinds(rows, under=UNDER):
    """rows: (labelled kind, label index). The offset per kind minimising the summed error."""
    out = {}
    for k in sorted({k for k, _ in rows}):
        labs = [lab for kk, lab in rows if kk == k]
        out[k] = min(range(-3, 4), key=lambda o: (sum(err(clamp(BASE + o), lab, under) for lab in labs), abs(o)))
    return out


def kinds_loss(rows, kinds, under=UNDER):
    return sum(err(clamp(BASE + kinds[k]), lab, under) for k, lab in rows) / len(rows)


def kinds_report(rows):
    """Prints current vs best kinds table for declared, leave-one-out, and bootstrap stability."""
    fit = fit_kinds(rows)
    table = {k: (KINDS[k], fit[k]) for k in fit}
    print("kinds table for declared (current -> best, under-rung 2x):",
          ", ".join(f"{k} {c:+d}->{b:+d}" for k, (c, b) in table.items()))
    for u in (1.0, UNDER):
        print(f"  error (under {u:g}x): current ${kinds_loss(rows, KINDS, u):.4f} -> best ${kinds_loss(rows, fit, u):.4f}"
              f" per task (fit with under {u:g}x: {fit_kinds(rows, u)})")
    loo = sum(err(clamp(BASE + fit_kinds(rows[:i] + rows[i + 1:]).get(k, KINDS[k])), lab, UNDER)
              for i, (k, lab) in enumerate(rows)) / len(rows)
    print(f"  leave-one-out error of the fitted table ${loo:.4f} (in-sample ${kinds_loss(rows, fit):.4f})")
    rng, n, wins, same = random.Random(7), 1000, 0, {k: 0 for k in fit}
    gains = []
    for _ in range(n):
        s = [rng.choice(rows) for _ in rows]
        f = fit_kinds(s)
        for k in fit:
            same[k] += f.get(k, KINDS[k]) == fit[k]
        g = kinds_loss(s, KINDS) - kinds_loss(s, fit)
        gains.append(g)
        wins += g > 0
    gains.sort()
    print(f"  bootstrap ({n}): best table beats current in {wins / n:.0%}; gain ${gains[n // 20]:.4f}..${gains[-n // 20]:.4f}"
          f" (90% CI); share of resamples refitting the same offset: "
          + ", ".join(f"{k} {same[k] / n:.0%}" for k in fit))
    return fit


def selftest():
    a = lambda **k: {"kind": "implement", "kind_confidence": 0.9, "novelty": 0, "design": 0, "blast": 0,  # noqa: E731
                     "check": 0, "mechanical": 0, **k}
    assert combine(a(mechanical=0.9, check=0.9), DEFAULT) == BASE - 2
    assert combine(a(novelty=0.8), DEFAULT) == BASE
    assert combine(a(kind="design", novelty=1), DEFAULT) == BASE + 2
    assert replay({"answers": a(kind_confidence=0.5, novelty=1), "index": 6}, DEFAULT) == 6  # low confidence
    assert replay({"answers": a(kind="other"), "index": 5}, DEFAULT) == 5
    picks = [{"answers": a(mechanical=0.75, check=0.9), "index": 2, "label": BASE - 2},  # wants DownMechanical <= 0.7
             {"answers": a(novelty=0.55), "index": 2, "label": BASE - 1},                 # wants UpNovelty > 0.5
             {"answers": None, "index": 3, "label": 3}]                                   # a fallback counts as picked
    w = best(picks)
    assert loss(picks, w) == 0 and w[2] <= 0.7 and w[0] > 0.5, w
    rows = [("search", 1), ("search", 1), ("search", 2), ("design", 3)]
    assert fit_kinds(rows) == {"search": -1, "design": 0}, fit_kinds(rows)
    assert kinds_loss(rows, {"search": -1, "design": 0}) < kinds_loss(rows, KINDS)
    print("calibrate selftest ok")


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--dir", default=".private/model-select")
    ap.add_argument("--selftest", action="store_true")
    a = ap.parse_args()
    if a.selftest:
        return selftest()
    runs = {n: [json.loads(line) for line in open(os.path.join(a.dir, f"picks-{n}.jsonl")) if line.strip()]
            for n in ("jev", "jev-nokind")}
    both = runs["jev"] + runs["jev-nokind"]
    w = best(both)
    print(f"weights: default {DEFAULT} -> best {w} (fit on both runs, {len(both)} rows, under-rung 2x)")
    for n, picks in [*runs.items(), ("both", both)]:
        fb = sum(not p.get("answers") or p["answers"]["kind"] not in KINDS or p["answers"]["kind_confidence"] < THRESHOLD
                 for p in picks)
        print(f"  {n:10} loss default ${loss(picks, DEFAULT):.4f} -> best ${loss(picks, w):.4f}; "
              f"under 1x ${loss(picks, DEFAULT, 1):.4f} -> ${loss(picks, w, 1):.4f}; own best {best(picks)}; fallback {fb}")
    for n, picks in runs.items():
        print(f"  {n:10} threshold sensitivity (default weights, under 1x): "
              + ", ".join(f"{t:g} ${loss(picks, DEFAULT, 1, t):.4f}" for t in (0, 0.5, 0.6, 0.7, 0.8, 0.9)))
    rows = [(p["kind"], p["label"]) for p in runs["jev"]]
    fit = kinds_report(rows)
    full = {**KINDS, **fit}
    for n, picks in runs.items():
        # the fallbacks move with the table: declared on the labelled kind, or base with no kind
        moved = [{**p, "index": clamp(BASE + full[p["kind"]]) if n == "jev" else BASE} for p in picks]
        print(f"  {n:10} with the fitted kinds table: default weights under 1x ${loss(moved, DEFAULT, 1, kinds=full):.4f}"
              f" (current table ${loss(picks, DEFAULT, 1):.4f})")
    print(f"var DefaultWeights = Weights{{UpNovelty: {w[0]}, UpBlast: {w[1]}, DownMechanical: {w[2]}, DownCheck: {w[3]}}}")


if __name__ == "__main__":
    main()
