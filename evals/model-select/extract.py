#!/usr/bin/env python3
"""Extract real Agent/Task calls from Claude Code transcripts into the model-select eval set.

Each call (a tool_use block in a main transcript) is joined with its subagent transcript
(<session>/subagents/agent-*.jsonl and .meta.json, matched on toolUseId) for the model and
effort actually used, the turns and an estimated cost; then a stratified sample is drawn.

    python3 -I evals/model-select/extract.py DIR... [--n 150] [--seed 7] [--out .private/model-select/calls.jsonl]
"""

import argparse
import collections
import glob
import json
import os
import random
import sys

# $/M tokens (input, output, cache write 5m, cache read); 1h write = 2x input. Copied from
# internal/stats/prices.go (PriceDate 2026-09-25); haiku-5-5 assumed equal to haiku-4-5.
PRICES = {
    "claude-fable-5-1": (10, 50, 12.5, 0.25), "claude-fable-5": (10, 50, 12.5, 1),
    "claude-opus-5-5": (4, 20, 5, 0.20), "claude-opus-5": (5, 25, 6.25, 0.5),
    "claude-opus-4-1": (15, 75, 18.75, 1.5), "claude-sonnet-5-5": (2, 10, 2.5, 0.10),
    "claude-sonnet-5": (2, 10, 2.5, 0.20), "claude-sonnet-4-6": (3, 15, 3.75, 0.3),
    "claude-haiku-4-5": (1, 5, 1.25, 0.1), "claude-haiku-5-5": (1, 5, 1.25, 0.1),
}


def cost(model, u):
    p = PRICES.get(model[:-9] if model[-9:-8] == "-" and model[-8:].isdigit() else model)
    if not p:
        return 0.0
    w1 = (u.get("cache_creation") or {}).get("ephemeral_1h_input_tokens", 0)
    w5 = u.get("cache_creation_input_tokens", 0) - w1
    return (u.get("input_tokens", 0) * p[0] + u.get("output_tokens", 0) * p[1] + w5 * p[2]
            + w1 * 2 * p[0] + u.get("cache_read_input_tokens", 0) * p[3]) / 1e6


def turns(lines):
    """Assistant turns deduplicated by message id (last usage wins)."""
    m = {}
    for d in lines:
        if d.get("type") != "assistant":
            continue
        msg = d["message"]
        e = m.setdefault(msg.get("id") or d.get("uuid"), {"model": msg.get("model"), "ts": d.get("timestamp", ""),
                                                          "effort": None, "u": None, "tools": []})
        e["u"] = msg.get("usage") or e["u"]
        e["effort"] = d.get("effort") or e["effort"]
        for b in msg.get("content") or []:
            if isinstance(b, dict) and b.get("type") == "tool_use" and b["id"] not in {t["id"] for t in e["tools"]}:
                e["tools"].append(b)
    return [e for e in m.values() if e["u"] and e["model"] and not e["model"].startswith("<")]


def top(xs):
    c = collections.Counter(x for x in xs if x)
    return c.most_common(1)[0][0] if c else None


def record(project, ts, b, sub):
    """One eval row: the call as written plus what its subagent transcript shows (sub may be None)."""
    i = b["input"]
    st = sub["turns"] if sub else []
    return {"id": b["id"], "project": project, "ts": ts, "description": i.get("description", ""),
            "prompt": i.get("prompt", ""), "subagent_type": i.get("subagent_type") or "general-purpose",
            "model": i.get("model"), "effort": i.get("effort"),
            "actual_model": top(t["model"] for t in st), "actual_effort": top(t["effort"] for t in st),
            "turns": len(st), "cost_usd": round(sum(cost(t["model"], t["u"]) for t in st), 4)}


def load(path):
    out = []
    with open(path, errors="replace") as f:
        for line in f:
            try:
                out.append(json.loads(line))
            except ValueError:
                pass
    return out


def scan(d):
    """All calls in one project transcript directory."""
    subs = {}
    for sf in glob.glob(os.path.join(d, "*", "subagents", "*.jsonl")):
        try:
            meta = json.load(open(sf[:-6] + ".meta.json"))
        except (OSError, ValueError):
            continue
        if meta.get("toolUseId"):
            subs[meta["toolUseId"]] = {"meta": meta, "turns": turns(load(sf))}
    out = []
    for f in glob.glob(os.path.join(d, "*.jsonl")):
        for t in turns(load(f)):
            for b in t["tools"]:
                if b["name"] in ("Agent", "Task") and b["input"].get("prompt"):
                    out.append(record(os.path.basename(d.rstrip("/")), t["ts"], b, subs.get(b["id"])))
    return out


def sample(rows, n, seed):
    """Round-robin over (subagent_type, model) strata, so rare strata are covered."""
    rng = random.Random(seed)
    strata = collections.defaultdict(list)
    for r in sorted(rows, key=lambda r: r["id"]):
        strata[(r["subagent_type"], r["model"] or "")].append(r)
    for v in strata.values():
        rng.shuffle(v)
    keys, out = sorted(strata), []
    while len(out) < n and any(strata.values()):
        for k in keys:
            if strata[k] and len(out) < n:
                out.append(strata[k].pop())
    return out


def selftest():
    u = {"input_tokens": 1_000_000, "output_tokens": 0}
    assert cost("claude-haiku-4-5-20251001", u) == 1 and cost("nope", u) == 0
    lines = [{"type": "assistant", "timestamp": "t", "effort": "high", "message": {
        "id": f"m{k}", "model": "claude-opus-5-5", "usage": {"output_tokens": 50_000},
        "content": [{"type": "tool_use", "id": f"u{k}", "name": "Agent",
                     "input": {"prompt": "p", "subagent_type": st, "model": md}}]}}
             for k, (st, md) in enumerate([("Explore", "haiku"), ("Explore", "haiku"), (None, "opus")])]
    ts = turns(lines + lines[:1])
    assert len(ts) == 3
    rows = [record("p", t["ts"], t["tools"][0], {"turns": ts} if k == 0 else None) for k, t in enumerate(ts)]
    assert rows[0]["actual_model"] == "claude-opus-5-5" and rows[0]["cost_usd"] == 3 and rows[0]["turns"] == 3
    assert rows[2]["subagent_type"] == "general-purpose" and rows[2]["actual_model"] is None
    s = sample(rows, 2, 7)
    assert {r["subagent_type"] for r in s} == {"Explore", "general-purpose"}, s
    assert sample(rows, 2, 7) == s and len(sample(rows, 9, 7)) == 3
    print("extract selftest ok")


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("dirs", nargs="*")
    ap.add_argument("--n", type=int, default=150)
    ap.add_argument("--seed", type=int, default=7)
    ap.add_argument("--out", default=".private/model-select/calls.jsonl")
    ap.add_argument("--selftest", action="store_true")
    a = ap.parse_args()
    if a.selftest:
        return selftest()
    if not a.dirs:
        ap.error("name at least one transcript directory")
    rows = [r for d in a.dirs for r in scan(d)]
    picked = sample(rows, a.n, a.seed)
    os.makedirs(os.path.dirname(a.out) or ".", exist_ok=True)
    with open(a.out, "w") as f:
        for r in picked:
            f.write(json.dumps(r) + "\n")
    c = collections.Counter((r["subagent_type"], r["model"] or "-", r["actual_model"] or "-") for r in picked)
    print(f"{len(rows)} calls found, {len(picked)} written to {a.out}", file=sys.stderr)
    for k, v in sorted(c.items()):
        print(f"{v:4}  {' / '.join(k)}", file=sys.stderr)


if __name__ == "__main__":
    main()
