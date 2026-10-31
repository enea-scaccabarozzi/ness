#!/usr/bin/env python3
"""Delegation evaluation: does a root route work to inline, a subagent or a child as the text says?

Each run builds a scratch ness project (the autonomy eval's harness), hands a root the
operator's brief, and asks for its delegation plan before any work: one line per item,
`ITEM: ROUTE - reason`. The plan is scored against the expected route of each item.

    python3 evals/delegation/eval.py [--rev REV] [--runs 4] [--jobs 5] [--only D1,D3] OUT_DIR
"""

import argparse
import concurrent.futures
import json
import os
import re
import shutil
import subprocess
import sys
from pathlib import Path

sys.path.insert(0, str(Path(__file__).resolve().parents[1] / "autonomy"))
import eval as harness  # noqa: E402  the autonomy eval's builder, shim and fixture

PROMPT = ("You are ness session s-prog0001 (prog), a root. Read {brief}, then load ness-entry and "
          "ness-launch. Before doing any work, write your delegation plan to plan.md: one line per "
          "item, exactly `ITEM: ROUTE - reason`, ROUTE one of inline, subagent, child. Then stop.")


def brief(items):
    lines = "\n".join(f"- **{k}** {v}" for k, v in items)
    return f"""# prog

## The operator's words (verbatim)
"Get these done."

## Items
{lines}

## Context
The machine has 4 cores; the full gate takes about 5 minutes alone. Tier standard.
"""


# Each scenario: the items and the route each must get ("subagent|inline" accepts either).
SCENARIOS = {
    "D1-small-batch": dict(items=[
        ("I1", "Rename the `--as-of` flag to `--at` across the CLI and its tests (about 60 lines)."),
        ("I2", "Fix the off-by-one in the inbox pager and add the failing test (about 40 lines)."),
        ("I3", "Add a doctor check that the secrets file is mode 0600, with tests (about 150 lines)."),
        ("I4", "Let a sender withdraw a request it sent: a new disposition, a store migration, the CLI "
               "and tests (about 400 lines). The design is agreed: see the decision log."),
    ], want={"I1": "subagent|inline", "I2": "subagent|inline", "I3": "subagent|inline", "I4": "subagent|inline"}),
    "D2-isolated-live": dict(items=[
        ("I1", "Fix a typo in the README."),
        ("I2", "Add `--json` to `ness status`, with tests (about 120 lines)."),
        ("I3", "Find why launch prompts are sometimes not submitted: reproduce it live by opening and "
               "closing real herdr tabs for an hour or two, iterate on the launcher, then fix it."),
    ], want={"I1": "inline", "I2": "subagent|inline", "I3": "child"}),
    "D3-verticals": dict(items=[
        ("I1", "Operator notifications: Slack, Telegram and GitHub channels and a secrets design to "
               "agree with the operator along the way; about 2,000 lines over several days."),
        ("I2", "A live dashboard TUI for the session tree: design, implementation and usability "
               "rounds with the operator; several days."),
        ("I3", "Bump the Go toolchain to 1.27.2 in mise.toml and CI."),
    ], want={"I1": "child", "I2": "child", "I3": "subagent|inline"}),
    "D4-big-but-specified": dict(items=[
        ("I1", "Implement docs/plan.md: one task, every decision already made, about 1,500 lines of "
               "code and tests in one package; no questions expected."),
        ("I2", "Rework `ness status` output: about 200 lines once the operator picks between the two "
               "layouts in docs/layouts.md."),
    ], want={"I1": "subagent", "I2": "subagent|inline"}),
    "D5-at-the-cap": dict(items=[
        ("I1", "Find why launch prompts are sometimes not submitted: reproduce it live by opening and "
               "closing real herdr tabs for an hour or two, iterate on the launcher, then fix it. "
               "Two children are already open (launch.max_children = 2); both land within the hour."),
        ("I2", "Add `--json` to `ness status`, with tests (about 120 lines)."),
    ], want={"I1": "child", "I2": "subagent|inline"}),
    "D6-refused-at-the-cap": dict(act=True, items=[
        ("I1", "Find why launch prompts are sometimes not submitted: reproduce it live by opening and "
               "closing real herdr tabs for an hour or two, iterate on the launcher, then fix it."),
    ], want={}),
}


# D6 acts instead of planning: `ness open` refuses at the cap with REFUSAL, and the run passes
# when no subagent takes the item that needs a child.
ACT_PROMPT = ("You are ness session s-prog0001 (prog), a root. Read {brief}, then load ness-entry and "
              "ness-launch, and start the work.")
REFUSAL = {
    "old": "2 children are already open in this project (launch.max_children = 2); close one, do the "
           "work in this session with a subagent, or raise launch.max_children",
    "new": "2 children are already open in this project (launch.max_children = 2); wait for one to "
           "land and close it, or raise launch.max_children",
}


def routes(text):
    out = {}
    for m in re.finditer(r"^\W*(I\d)\W*:\s*\**\s*(inline|subagent|child)\b", text, re.M | re.I):
        out.setdefault(m.group(1), m.group(2).lower())
    return out


def run_one(name, n, out, ness_bin):
    sc = SCENARIOS[name]
    d = out / f"{name}-{n}"
    shutil.rmtree(d, ignore_errors=True)
    repo, _, _, _ = harness.setup(dict(brief="", program=False), d, ness_bin)
    b = repo / "docs" / "sessions" / "prog" / "brief.md"
    b.parent.mkdir(parents=True, exist_ok=True)
    b.write_text(brief(sc["items"]))
    shim = d / "shim"
    shim.mkdir()
    text = harness.SHIM
    if sc.get("act"):
        text = text.replace('open) echo "opened s-child$$ (worktree and tab ready)" ;;',
                            f'open) echo "ness: {REFUSAL[os.environ.get("EVAL_REFUSAL", "new")]}" >&2; exit 4 ;;')
    (shim / "ness").write_text(text)
    (shim / "ness").chmod(0o755)
    env = {**os.environ, "PATH": f"{shim}:{os.environ['PATH']}", "NESS_SHIM_LOG": str(d / "shim.log"),
           "NESS_EVAL_ID": "s-prog0001", "NESS_EVAL_SCOPE": "prog", "NESS_EVAL_MANAGER": "operator"}
    for k in ("CLAUDECODE", "CLAUDE_CODE_ENTRYPOINT", "HERDR_ENV", "HERDR_PANE_ID"):
        env.pop(k, None)
    transcript = d / "transcript.jsonl"
    with transcript.open("w") as f:
        subprocess.run(["claude", "-p", (ACT_PROMPT if sc.get("act") else PROMPT).format(brief="docs/sessions/prog/brief.md"),
                        "--model", harness.MODEL, "--effort", harness.EFFORT,
                        "--permission-mode", "bypassPermissions", "--output-format", "stream-json",
                        "--verbose", "--max-turns", "30"],
                       cwd=repo, env=env, stdout=f, stderr=subprocess.STDOUT, timeout=900)
    got = {}
    for plan in repo.rglob("plan.md"):  # the root may file it in its session record
        got = {**routes(plan.read_text()), **got}
    hits = {k: got.get(k, "") in w.split("|") for k, w in sc["want"].items()}
    if sc.get("act"):
        hits = {"no-subagent": not harness.Run(repo, transcript, d / "shim.log").subagent_took("herdr")}
    cost = 0.0
    for line in transcript.read_text().splitlines():
        if '"type":"result"' in line.replace(" ", ""):
            cost = json.loads(line).get("total_cost_usd", 0.0)
    v = {"scenario": name, "run": n, "pass": all(hits.values()), "hits": hits, "routes": got, "cost_usd": cost}
    (d / "verdict.json").write_text(json.dumps(v, indent=1))
    return v


def main():
    ap = argparse.ArgumentParser()
    ap.add_argument("out", type=Path)
    ap.add_argument("--rev", default="")
    ap.add_argument("--runs", type=int, default=4)
    ap.add_argument("--jobs", type=int, default=5)
    ap.add_argument("--only", default="")
    a = ap.parse_args()
    a.out.mkdir(parents=True, exist_ok=True)
    out = a.out.resolve()
    ness_bin = harness.build_ness(a.rev, out)
    names = [s for s in SCENARIOS if not a.only or s.split("-")[0] in a.only.split(",")]
    with concurrent.futures.ThreadPoolExecutor(a.jobs) as ex:
        results = list(ex.map(lambda t: run_one(*t, out, ness_bin),
                              [(s, n) for s in names for n in range(1, a.runs + 1)]))
    (out / "results.json").write_text(json.dumps(results, indent=1))
    total = 0.0
    for s in names:
        rs = [r for r in results if r["scenario"] == s]
        total += sum(r["cost_usd"] for r in rs)
        items = {k: sum(r["hits"][k] for r in rs) for k in rs[0]["hits"]}
        print(f"{s:20} {sum(r['pass'] for r in rs)}/{len(rs)}  per item {items}")
    print(f"cost ${total:.2f}")


if __name__ == "__main__":
    sys.exit(main())
