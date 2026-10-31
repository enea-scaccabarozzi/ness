#!/usr/bin/env python3
"""Score a selector against the labelled eval set, next to what the sessions actually used.

Runs `ness pick --json --role subagent --kind K` per call (K = the labelled kind, the declared
selector's best case) and saves each pick to picks-<policy>.jsonl. For jev it runs twice with
`--explain`: with the labelled kind declared (picks-jev.jsonl) and with no kind (picks-jev-nokind.jsonl:
Jev judges alone, the hook's case for an agent that omits the kind); the raw answers and the wall
time of each pick are saved so calibrate.py needs no new calls. The selector is the one configured
in the repo the binary runs in (`--cwd`): `[models] policy = "jev"` there for the jev run.

    python3 -I evals/model-select/score.py --policy declared|jev [--ness ness] [--cwd .] [--dir .private/model-select] [--limit N]
"""

import argparse
import json
import math
import os
import subprocess
import sys
import time

RUNGS = ["bulk", "economy", "lean", "standard", "deep", "frontier", "apex"]
# $/task proxies per rung (research-tiering.md).
COST = [0.01, 0.07, 0.12, 0.21, 0.29, 0.60, 1.19]
FAMILY = {"bulk": ("haiku", "medium"), "economy": ("sonnet", "medium"), "lean": ("opus", "low"),
          "standard": ("opus", "medium"), "deep": ("opus", "high"), "frontier": ("fable", "medium"),
          "apex": ("fable", "high")}
EFFORTS = ["low", "medium", "high", "xhigh", "max"]


def err(picked, label, under_weight=1.0):
    """Dollar error of picking `picked` when `label` was the lowest adequate rung: overspend, or
    (under-rung) the wasted attempts, retrying one rung up each time until the label's rung."""
    if picked >= label:
        return COST[picked] - COST[label]
    return under_weight * sum(COST[picked:label])


def actual_rung(model, effort):
    """The rung a subagent really ran at: same family, nearest effort (unknown effort = medium)."""
    if not model:
        return None
    e = EFFORTS.index(effort) if effort in EFFORTS else 1
    cands = [i for i, r in enumerate(RUNGS) if FAMILY[r][0] in model]
    return min(cands, key=lambda i: abs(EFFORTS.index(FAMILY[RUNGS[i]][1]) - e)) if cands else None


def report(name, pairs):
    """pairs: (picked index, label index). Returns the summary dict and prints it."""
    n = len(pairs)
    if not n:
        print(f"{name}: no rows")
        return {}
    s = {"n": n, "exact": sum(p == l for p, l in pairs) / n, "over": sum(p > l for p, l in pairs) / n,
         "under": sum(p < l for p, l in pairs) / n, "cost_err": sum(err(p, l) for p, l in pairs) / n,
         "cost_label": sum(COST[l] for _, l in pairs) / n}
    print(f"{name:10} n={n:3}  exact {s['exact']:.0%}  over {s['over']:.0%}  under {s['under']:.0%}  "
          f"cost-weighted error ${s['cost_err']:.3f}/task (labelled ideal ${s['cost_label']:.3f}/task)")
    return s


def pick(ness, cwd, policy, call, kind):
    """One pick; kind None declares none. secs is the wall time (process start included)."""
    cmd = [ness, "pick", "--stdin", "--json", "--role", "subagent"] + (["--kind", kind] if kind else [])
    cmd += ["--explain"] if policy == "jev" else []
    t = time.monotonic()
    p = subprocess.run(cmd, input=call["prompt"], capture_output=True, text=True, cwd=cwd, timeout=60)
    secs = time.monotonic() - t
    if p.returncode:
        raise RuntimeError(f"{call['id']}: {' '.join(cmd)}: {p.stderr.strip()}")
    d = json.loads(p.stdout)
    return {"id": call["id"], "index": d["index"], "rung": d["rung"], "reason": d.get("reason"),
            "answers": d.get("answers"), "fallback": d.get("fallback"), "secs": round(secs, 3)}


def pct(xs, q):
    """The q-quantile of xs, nearest rank."""
    xs = sorted(xs)
    return xs[max(0, math.ceil(q * len(xs)) - 1)]


def run(a, name, policy, calls, labels, declare):
    """Picks every labelled call into picks-<name>.jsonl and reports them."""
    out = os.path.join(a.dir, f"picks-{name}.jsonl")
    if a.saved:
        picks = read_jsonl(out)
    else:
        picks = []
        with open(out, "w") as f:
            for x in labels:
                p = pick(a.ness, a.cwd, policy, calls[x["id"]], x["kind"] if declare else None)
                p["kind"], p["label"] = x["kind"], RUNGS.index(x["lowest_rung"])
                picks.append(p)
                f.write(json.dumps(p) + "\n")
        print(f"{len(picks)} picks saved to {out}", file=sys.stderr)
    report(name, [(p["index"], p["label"]) for p in picks])
    if policy == "jev":
        fb, secs = sum(bool(p["fallback"]) for p in picks), [p["secs"] for p in picks]
        to = sum("timeout" in (p["reason"] or "") for p in picks)
        print(f"{name:10} fallback {fb}/{len(picks)} (timeouts {to}); pick wall time p50 {pct(secs, .5):.2f}s "
              f"p95 {pct(secs, .95):.2f}s max {max(secs):.2f}s")


def read_jsonl(path):
    return [json.loads(line) for line in open(path) if line.strip()]


def selftest():
    assert err(3, 3) == 0 and round(err(5, 3), 2) == 0.39
    assert round(err(1, 3), 2) == 0.19 and err(1, 3, 2) == 2 * err(1, 3) and round(err(2, 3), 2) == 0.12
    assert actual_rung("claude-haiku-4-5-20251001", None) == 0
    assert actual_rung("claude-opus-5-5", "xhigh") == 4 and actual_rung("claude-fable-5-1", "low") == 5
    assert actual_rung(None, "high") is None
    assert pct([1, 2, 3, 4], .5) == 2 and pct(list(range(1, 21)), .95) == 19 and pct([7], .95) == 7
    # 3-row fixture: exact, over by two, under by one
    s = report("fixture", [(3, 3), (5, 3), (1, 2)])
    assert s["n"] == 3 and abs(s["exact"] - 1 / 3) < 1e-9 and abs(s["under"] - 1 / 3) < 1e-9
    assert abs(s["cost_err"] - (0 + 0.39 + (0.07 + 0.12 - 0.12)) / 3) < 1e-9
    print("score selftest ok")


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--policy", choices=["declared", "jev"])
    ap.add_argument("--ness", default="ness", help="ness binary PATH")
    ap.add_argument("--cwd", default=".", help="repo whose ness config selects the policy")
    ap.add_argument("--dir", default=".private/model-select")
    ap.add_argument("--limit", type=int, default=0, help="score only the first N calls (smoke run)")
    ap.add_argument("--saved", action="store_true", help="report the saved picks, no new picks")
    ap.add_argument("--selftest", action="store_true")
    a = ap.parse_args()
    if a.selftest:
        return selftest()
    if not a.policy:
        ap.error("--policy is required")
    calls = {c["id"]: c for c in read_jsonl(os.path.join(a.dir, "calls.jsonl"))}
    labels = [x for x in read_jsonl(os.path.join(a.dir, "labels.jsonl")) if x["id"] in calls]
    labels = labels[:a.limit] if a.limit else labels
    run(a, a.policy, a.policy, calls, labels, True)
    if a.policy == "jev":
        run(a, "jev-nokind", "jev", calls, labels, False)
    act = [(actual_rung(calls[x["id"]]["actual_model"], calls[x["id"]]["actual_effort"]), RUNGS.index(x["lowest_rung"]))
           for x in labels]
    report("actual", [(p, l) for p, l in act if p is not None])


if __name__ == "__main__":
    main()
