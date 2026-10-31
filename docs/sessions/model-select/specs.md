> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# Specs: model-select

What we agreed to build (brief: approved approach 1–4). Evidence in
`docs/discovery/findings/model-select/` (field-models, research-tiering, sweep-ness-models, jev-grounding).

## Problem, in numbers
The research lab, ness era (10-12..10-21), $1,807: 21 of 24 sessions opened at `frontier` (Fable) and their own turns are
62% of the bill. Subagents were mostly Sonnet/Opus, but Fable parents passed `model: opus` to search, scan and run
jobs (~$166 that `ness-sweep` work would have done for a fraction). The rules exist only as prose: `--tier` needs no
reason, the only hook check refuses Explore, no Agent call passed `effort`, "one tier up" has no code.

## Goals
- The default is cheap for mechanical work and heavy only where a step earns it, decided per step, and the
  decision survives compaction because code enforces it.
- One knob per project: move the **base rung** up or down. Thresholds stay possible, less directly.
- Selection is a swappable, client-agnostic ness primitive: built-in selectors and a project-space command.
- Jev is an optional built-in selector: a fan-out of narrow judgments combined in code.

## Non-goals
- Changing a running session's model or effort (R-MOD-012 stands).
- Choosing the root's model for the operator; ness only recommends one when `--tier` is left out.
- Enforcing subagent rungs on Codex (no spawn hook known); Codex gets the ladder at open and in its `[agents]`
  defaults only.

## 1. The ladder
- A **rung** is a row of the tier table (`[tiers.NAME]`, claude + codex `model/effort`). The default table grows
  from 3 to 7 rows, ordered cheapest first:

  | rung | claude | codex |
  |---|---|---|
  | bulk | haiku/medium | gpt-6-luna/low |
  | economy | sonnet/medium | gpt-6-luna/medium |
  | lean | opus/low | gpt-6.1-sol/low |
  | standard | opus/medium | gpt-6.1-sol/medium |
  | deep | opus/high | gpt-6.1-sol/high |
  | frontier | fable/medium | astra/medium |
  | apex | fable/high | astra/high |

  (Codex ids/efforts are checked against `codex debug models` and the codex atom before release.)
- `[models] ladder = [...]` orders the rung names (default: the order above; a project may drop or reorder
  rungs and define its own rows). `base` (default `standard`) and optional `ceiling` name rungs on the ladder.
- Every selector answers an **offset from base**; ness clamps `base + offset` to the ladder and to `ceiling`.
  A budget project writes `base = "economy"` and everything shifts down two rungs, sweeps included.
- Existing names `standard`, `frontier`, `economy` keep their meaning, so `--tier standard` still works.

## 2. The selector primitive
The existing seam (`session.Policy`, D17, R-MOD-041) becomes the selector. Input: task text, role
(root|child|subagent), declared kind if any, agent type, whether this is a retry; output: offset, confidence 0..1,
one-line reason. `[models] policy` picks it:
- **declared** (default, offline): the caller names a work kind; a kind→offset table decides. Default table
  (overridable in `[models.kinds]`; changed by MS-D7):

  | kind | offset | from base standard |
  |---|---|---|
  | search (find, map, inventory, read-only) | −2 | economy |
  | run (execute a specified procedure, tests, experiments, collection) | −2 | economy |
  | implement (edit to a written spec with a check) | 0 | standard |
  | debug | 0 | standard |
  | research, review | +1 | deep |
  | verify (adversarial check of one claim) | +1 | deep |
  | design (choose between alternatives, novel or irreversible) | +1 | deep |

  A missing or unknown kind is refused with the list.
- **jev** (optional, built in): one Jev request per selection over the task state, a fan-out of narrow questions
  (§4) combined in Go into an offset. Key from the `[models.jev] key` reference (`env:NAME` or `secret:NAME`, default `env:TYPESAFE_API_KEY`); on any failure,
  timeout or confidence below `models.router_threshold` the answer is the declared selector's (or base when no kind).
- **command**: `[models] command = "..."` runs at the repo root with the input as JSON on stdin and prints
  `{"offset": n, "confidence": c, "reason": "..."}`; the swap point for any project.
- `ness pick` runs the configured selector on a task (text on stdin or `--kind`) and prints the rung, model,
  effort and reason, so an agent can ask before it spawns.
- Named agents carry their kind: ness-sweep = search, ness-review = review, ness-verify = verify,
  ness-design = design. Their frontmatter is rendered from `base + offset`, so they move with the base.
  `CLAUDE_CODE_SUBAGENT_MODEL` is the model of the search kind's rung (`base − 2` at the default kinds).

## 3. Enforcement
- **Agent hook (Claude).** The pre-tool hook, on every `Agent`/`Task` call from a bound session:
  - resolves the requested rung from the call's `model` + `effort` (a named ness agent with neither uses its
    rendered rung); a pair not on the ladder, or a general-purpose call without both, is refused with the ladder;
  - asks the selector for the recommended rung (kind from a `kind: <name>` line in the prompt, or the named
    agent's kind);
  - refuses a requested rung above the recommendation unless the prompt carries `escalate: <reason>`
    (one rung per escalate; the reason is logged);
  - allows at or below; `fork` subagents inherit the parent and are allowed and logged.
  - Every decision (session, agent type, kind, requested, recommended, verdict, escalate reason) is appended to the
    store for stats.
  - `[models] enforce = "refuse"` (default) or `"log"`: log-only keeps the record and never refuses — the
    loosening switch the operator asked for if the data shows friction.
- **ness open.** `--tier` becomes optional: without it the selector runs on the brief (role child/root). An
  explicit `--tier` above the recommendation needs `--reason TEXT` (refused otherwise); the reason is recorded
  (R-MOD-043). Low confidence falls back to base instead of refusing (replaces R-MOD-042's refusal).
- **Escalation.** `ness open --resume ID --tier up` resumes one rung above the stored one (the boundary-note
  path in ness-entry gets code); `--tier down` likewise.
- **After compaction.** The post-compact list gains one factual line on the ladder, the kind line and escalate.
- **Skills.** ness-launch "Tiers" and ness-entry "Model and effort" are rewritten around base, kinds, `ness pick`
  and escalate; the predecessor doctrine (judge-based choice, routine rung + higher-rung subagent for rare hard
  moments) is stated once.

## 4. Jev fan-out
One request, state = {task, role, agent_type, declared_kind}. Questions (ids for code only):
- Scores: reasoning depth; ambiguity/underspecification; blast radius/irreversibility; novelty (no pattern to copy);
  breadth of context needed.
- Nouls: objective check exists; mechanical (read/search/run/transcribe); choosing between design alternatives;
  judge/reviewer role; long dependent chain.
- Choice: work kind (the declared kinds).
Code: start from the Choice's kind offset; move up for high novelty/design/irreversibility with no check, down for
mechanical with a check; confidence = the kind Choice's confidence. The weights and cut
points live in the jev atom with a test table; they are tuned on the eval set below, not guessed.

## 5. Evidence and measurement
- **Eval set** `evals/model-select/`: ~150 real Agent calls sampled from research-lab transcripts (prompt, description,
  type), labelled with kind and the lowest adequate rung by an independent opus/high labeller with a written
  rubric; the operator spot-checks 20. Each selector is scored on exact rung, over-rung and under-rung rates and
  cost-weighted error. The private prompts stay under `.private/`; only the harness and aggregate numbers are
  committed.
- **Stats.** `ness stats` adds cost by rung and role and the hook decisions (refused, escalated, below) per session,
  so the operator can judge friction before switching to `enforce = "log"`.

## Spec and docs
`docs/spec/models.md` is rewritten (R-MOD-001..004, 020, 040..043 change; new rows for ladder, selectors, hook,
pick, stats); `docs/spec/hooks.md`, `stats.md`, `core-config.md` get the new rows; design §12 gets a short
"ladder" update; decisions in `docs/design/decisions.md`.

## Tests (strategy)
- Black-box per package; hand-written fakes: a fake selector (offset, confidence, error), a fake Jev HTTP server
  (`httptest`), a fake command selector script in testdata.
- Judges written separately from the implementation: the hook's decision table (requested × recommended ×
  escalate × enforce) and the ladder clamp table are written as acceptance tables before the code by a separate
  task.
- Each test names the wrong implementation it rejects (e.g. a hook that compares models but ignores effort; a
  clamp that ignores `ceiling`; a selector failure that refuses instead of falling back).
- A live spike before the hook ships: an Agent call with `model` and `effort` on this Claude Code (2.1.306) and
  the transcript showing that model and effort; and the hook payload showing both fields.
