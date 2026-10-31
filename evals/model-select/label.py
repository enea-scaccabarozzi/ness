#!/usr/bin/env python3
"""Label each eval call with its work kind and the lowest adequate rung, by an independent labeller.

The labeller is `claude -p --model opus --effort high` with no tools, no MCP and no skills, the
rubric below as its system prompt and a JSON schema for its answer. Resumable: ids already in
the labels file are skipped.

    python3 -I evals/model-select/label.py [--calls .private/model-select/calls.jsonl]
        [--out .private/model-select/labels.jsonl] [--jobs 6] [--limit N] [--spot 20]
"""

import argparse
import concurrent.futures
import json
import os
import random
import subprocess
import sys
import tempfile
import threading

# The ladder as `ness pick` prints it (Claude column), cheapest first.
RUNGS = [("bulk", "haiku/medium"), ("economy", "sonnet/medium"), ("lean", "opus/low"),
         ("standard", "opus/medium"), ("deep", "opus/high"), ("frontier", "fable/medium"),
         ("apex", "fable/high")]
KINDS = {
    "search": "find, map, list or read code/docs and report; no edits",
    "run": "execute a specified procedure (tests, experiments, data collection, a script) and report",
    "implement": "edit code or docs to a written spec that has a check (tests, build, gate)",
    "debug": "find the cause of a failure and usually fix it",
    "research": "gather and synthesise external or cross-cutting evidence into findings",
    "review": "judge a change against a plan or rules and report findings",
    "verify": "try to prove one specific claim wrong (adversarial check)",
    "design": "choose between alternative designs or create a new mechanism",
}

RUBRIC = f"""You label tasks that an AI coding agent delegated to a subagent. For each task you get the
subagent's description and prompt exactly as the delegating agent wrote them. You have no tools; judge
from the text alone.

Answer two things.

1. kind: the kind of work the task asks for. Judge the work itself, not the subagent type or any label
in the prompt. If the task mixes kinds, pick the one that decides success (the hardest essential part).
{chr(10).join(f"- {k}: {v}" for k, v in KINDS.items())}

2. lowest_rung: the cheapest rung of this model ladder (cheapest first) that would do this task
correctly on the first attempt with high probability (about 9 in 10):
{chr(10).join(f"- {r} ({m})" for r, m in RUNGS)}
Rough guide: bulk = mechanical reading, listing, transcribing, running given commands; economy =
searching and summarising code, straightforward edits fully specified; lean = small implementation to a
clear spec with a check; standard = ordinary implementation, debugging, review needing judgment; deep =
subtle reasoning, long dependent chains, adversarial verification, large blast radius; frontier = novel
design between alternatives, hard ambiguous problems; apex = only the hardest, highest-stakes design or
reasoning, where frontier would likely fail.
Judge by the task as written: its reasoning depth, ambiguity, blast radius and irreversibility, novelty,
breadth of context, and whether an automated check catches mistakes. Do NOT infer the rung from the model
the agent requested or any model/effort named in the prompt; that choice is what is being evaluated.
"Lowest adequate" means: if a cheaper rung would succeed, choose it; do not pad for safety.

why: one or two sentences naming the decisive features. Do not quote secrets or long passages.

Output only the JSON object required by the schema."""

SCHEMA = {"type": "object", "additionalProperties": False, "required": ["kind", "lowest_rung", "why"],
          "properties": {"kind": {"type": "string", "enum": list(KINDS)},
                         "lowest_rung": {"type": "string", "enum": [r for r, _ in RUNGS]},
                         "why": {"type": "string"}}}

CMD = ["claude", "-p", "--model", "opus", "--effort", "high", "--tools", "", "--strict-mcp-config",
       "--disable-slash-commands", "--no-session-persistence", "--output-format", "json",
       "--system-prompt", RUBRIC, "--json-schema", json.dumps(SCHEMA)]


def task_text(call, limit=60_000):
    t = f"description: {call['description']}\nsubagent_type: {call['subagent_type']}\n\nprompt:\n{call['prompt']}"
    return t if len(t) <= limit else t[:limit] + "\n[... truncated]"


def parse(stdout):
    """The labeller's answer from `claude -p --output-format json`; raises on anything off-schema."""
    d = json.loads(stdout)
    if d.get("is_error"):
        raise ValueError(f"claude error: {d.get('result', '')[:200]}")
    o = d.get("structured_output") or json.loads(d["result"])
    if o.get("kind") not in KINDS or o.get("lowest_rung") not in [r for r, _ in RUNGS] or not o.get("why"):
        raise ValueError(f"off-schema answer: {o}")
    return {"kind": o["kind"], "lowest_rung": o["lowest_rung"], "why": o["why"],
            "labeller_cost_usd": d.get("total_cost_usd")}


def label(call, cwd):
    p = subprocess.run(CMD, input=task_text(call), capture_output=True, text=True, cwd=cwd, timeout=900)
    if p.returncode:
        raise RuntimeError(f"exit {p.returncode}: {p.stderr[-300:]}")
    return {"id": call["id"], **parse(p.stdout)}


def read_jsonl(path):
    if not os.path.exists(path):
        return []
    return [json.loads(line) for line in open(path) if line.strip()]


def spot(calls, labels, n, path, seed=7):
    by = {c["id"]: c for c in calls}
    pick = random.Random(seed).sample([x for x in labels if x["id"] in by], min(n, len(labels)))
    out = ["# Model-select labels: spot check", "",
           "Mark each item OK or give the kind/rung you would pick.", ""]
    for k, x in enumerate(pick, 1):
        c = by[x["id"]]
        out += [f"## {k}. {c['description']} (`{x['id']}`)", "",
                f"**label:** {x['kind']} / {x['lowest_rung']} | **used:** {c['subagent_type']}, "
                f"model param {c['model'] or '-'}, actual {c['actual_model'] or '-'}", "",
                f"**why:** {x['why']}", "", "```text", c["prompt"][:4000], "```", "", "**verdict:** ", ""]
    with open(path, "w") as f:
        f.write("\n".join(out))


def selftest():
    for r, m in RUNGS:
        assert f"- {r} ({m})" in RUBRIC
    for k in KINDS:
        assert f"- {k}:" in RUBRIC
    assert CMD[CMD.index("--tools") + 1] == "" and "opus" in CMD and "high" in CMD
    rows = [{"id": "a", "kind": "search", "lowest_rung": "bulk", "why": "w"},
            {"id": "b", "kind": "design", "lowest_rung": "frontier", "why": "w"},
            {"id": "c", "kind": "nope", "lowest_rung": "bulk", "why": "w"}]
    for x in rows[:2]:
        got = parse(json.dumps({"is_error": False, "result": "", "structured_output": x, "total_cost_usd": 0.1}))
        assert got["kind"] == x["kind"] and got["lowest_rung"] == x["lowest_rung"]
    assert parse(json.dumps({"result": json.dumps(rows[0])}))["kind"] == "search"
    try:
        parse(json.dumps({"structured_output": rows[2]}))
        raise AssertionError("off-schema accepted")
    except ValueError:
        pass
    assert task_text({"description": "d", "subagent_type": "t", "prompt": "x" * 70_000}).endswith("[... truncated]")
    with tempfile.TemporaryDirectory() as d:
        calls = [{"id": i, "description": "d", "prompt": "p", "subagent_type": "t", "model": None,
                  "actual_model": None} for i in "abc"]
        spot(calls, rows, 2, os.path.join(d, "spot.md"))
        assert open(os.path.join(d, "spot.md")).read().count("**verdict:**") == 2
    print("label selftest ok")


def main():
    ap = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    ap.add_argument("--calls", default=".private/model-select/calls.jsonl")
    ap.add_argument("--out", default=".private/model-select/labels.jsonl")
    ap.add_argument("--jobs", type=int, default=6)
    ap.add_argument("--limit", type=int, help="label at most N new calls")
    ap.add_argument("--spot", type=int, metavar="N", help="only write N random labelled items to spot.md")
    ap.add_argument("--selftest", action="store_true")
    a = ap.parse_args()
    if a.selftest:
        return selftest()
    calls, done = read_jsonl(a.calls), read_jsonl(a.out)
    if a.spot:
        path = os.path.join(os.path.dirname(a.out), "spot.md")
        spot(calls, done, a.spot, path)
        return print(f"wrote {path}")
    seen = {x["id"] for x in done}
    todo = [c for c in calls if c["id"] not in seen][:a.limit]
    print(f"{len(seen)} labelled, {len(todo)} to go", file=sys.stderr)
    lock, fails = threading.Lock(), 0
    with tempfile.TemporaryDirectory() as cwd, open(a.out, "a") as f, \
            concurrent.futures.ThreadPoolExecutor(a.jobs) as ex:  # empty cwd: no project CLAUDE.md
        futs = {ex.submit(label, c, cwd): c["id"] for c in todo}
        for fut in concurrent.futures.as_completed(futs):
            try:
                row = fut.result()
            except Exception as e:  # one bad call must not lose the others; rerun resumes it
                fails += 1
                print(f"{futs[fut]}: {e}", file=sys.stderr)
                continue
            with lock:
                f.write(json.dumps(row) + "\n")
                f.flush()
            print(f"{row['id']}: {row['kind']} / {row['lowest_rung']}", file=sys.stderr)
    sys.exit(1 if fails else 0)


if __name__ == "__main__":
    main()
