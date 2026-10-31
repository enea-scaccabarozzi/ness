> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# Model select 3: Jev selector and the eval set Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the optional built-in `jev` selector (one fan-out request of narrow judgments, combined in Go into an offset), and an eval set from real research-lab Agent calls that scores every selector and calibrates the Jev combination.

**Architecture:** `internal/atom/jev` is a `session.Policy`: it builds one `/v1/systemone` request, decodes the typed answers, and hands them to a pure `Combine` with a weights table; any failure or low confidence returns the declared selector's verdict. `ness pick --explain` prints the raw answers and the combination, so the eval harness (Python, `evals/model-select/`) scores through the same code path. Calibration changes only the weights table.

**Tech Stack:** Go `net/http` (no SDK), `httptest` fake server; Python 3 stdlib for the eval; `claude -p` for the independent labeller.

**Spec:** `docs/sessions/model-select/specs.md` §2 (jev), §4, §5; `docs/discovery/findings/model-select/jev-grounding.md` for the API contract; decision MS-D4.

## Global Constraints
- API: `POST {endpoint}/v1/systemone`, `Authorization: Bearer <key>`, body `{"model","state","questions"}`; answers: noul → `noul`; choice → `choice`, `probabilities`, `confidence`; score → `score` (0..n−1), `probabilities`, `confidence`. Retry once on 429/529 with backoff ≤ 1s, never more (the hook is waiting).
- Config `[models.jev]`: `model` default `"jev-1.14.0"` (pinned, never an alias), `key` default `"env:TYPESAFE_API_KEY"` (also `secret:NAME`; never a literal, refused at load like notify tokens R-CFG-081), `endpoint` default `"https://api.typesafe.ai"`, `timeout` default `"5s"`.
- `config.ModelPolicies` gains `"jev"`.
- State sent: `{"task": text, "role": role, "agent_type": type, "declared_kind": kind}` (empty fields omitted). Never send file contents beyond the prompt the agent wrote.
- The key never appears in output, errors, logs or the store.

## Review Focus
- Endpoint returns 200 with an answer missing (`answers.kind` absent) or a probability map not summing ~1: treated as failure → declared fallback, not a panic.
- A prompt of 200k characters: truncated to the 32k-token per-question limit's safe size (cut at 60,000 characters, head kept, marker appended) before sending.
- Key reference resolves to empty (env unset): selector reports `jev: no key (models.jev.key)` and falls back; `ness doctor` warns once.
- 401 from the endpoint: fallback, and the reason says `jev: 401` (no key material).
- Jev's Choice confident on a kind but the declared kind disagrees: Jev wins (it is the independent judgment); the reason names both.

---

### Task 1: Questions and Combine (pure)

**Files:** Create `internal/atom/jev/questions.go`, `internal/atom/jev/combine.go`, test `internal/atom/jev/combine_test.go`.

**Interfaces:**
- Produces: `func Questions(kinds []string) map[string]Question` (the request's question set; `Question{Type, Instructions string; Criteria any}`), and `type Answers struct{ Kind string; KindConfidence float64; Depth, Ambiguity, Blast, Novelty, Breadth float64; Check, Mechanical, Design, Judge, Chain float64 }` (Scores normalised to 0..1 by dividing by top level; Nouls as probabilities).
- Produces: `type Weights struct{ UpNovelty, UpBlast, DownMechanical, DownCheck float64 }` and `var DefaultWeights = Weights{UpNovelty: 0.6, UpBlast: 0.6, DownMechanical: 0.7, DownCheck: 0.7}` (read-only; calibration edits the literal).
- Produces: `func Combine(a Answers, kinds map[string]int, w Weights) session.Verdict`:
  ```
  off := kinds[a.Kind]
  up := (a.Novelty >= w.UpNovelty || a.Design >= w.UpNovelty) && a.Check < 0.5 && a.Kind != "design"
  up = up || (a.Blast >= w.UpBlast && a.Check < 0.5)
  down := a.Mechanical >= w.DownMechanical && a.Check >= w.DownCheck && a.Kind != "search" && a.Kind != "run"
  off += +1 if up, −1 if down and not up   // at most one step from the kind's offset
  confidence := a.KindConfidence
  reason := "jev: kind K (c=0.xx)" + " +1 novel/irreversible without a check" | " −1 mechanical with a check"
  ```
- Question wording (copy into questions.go; criteria as structured objects where the plan gives them):
  - kind (choice): "Which kind of work does `task` ask for? Judge the work itself, not any label in `declared_kind`." criteria: one entry per kind with covers/excludes — search: find, map, list or read code/docs, no edits; run: execute a specified procedure (tests, experiments, data collection) and report; implement: edit code or docs to a written spec with a check; debug: find the cause of a failure; research: gather and synthesise external or cross-cutting evidence; review: judge a change against a plan or rules; verify: try to prove one claim wrong; design: choose between alternative designs or create a new mechanism.
  - depth (score, 4 levels): none / light / substantial / deep (as in scratchpad smoke.py's reasoning_difficulty).
  - ambiguity (score, 3): fully specified / some assumptions needed / goal itself unclear.
  - blast (score, 3): local and reversible / several modules or shared state, reversible / irreversible or external effects (data loss, publishing, money, migrations).
  - novelty (score, 3): a pattern exists in the codebase to copy / adapt a known pattern / no precedent, new mechanism.
  - breadth (score, 3): one file / a package / many packages or documents.
  - check (noul): "Does `task` name or clearly imply an automated check (tests, build, a script's exit status) that decides whether the work succeeded?"
  - mechanical (noul): "Is `task` mechanical: reading, searching, running given commands or transcribing, with no design decisions?"
  - design (noul): "Does `task` require choosing between several viable designs or approaches?"
  - judge (noul): "Is the output of `task` itself a judgment that others will act on without re-checking it (a verdict, a review, a conclusion)?"
  - chain (noul): "Does `task` require a long chain of dependent steps where an early mistake silently spoils the rest?"

- [ ] Judge tests (separate subagent): each table row of Combine (search+mechanical+check stays −2, implement+mechanical+check → −2, implement+novel no check → 0, design+novel → +2 only, blast irreversible no check on review → +1, up and down both true → up wins); reason strings name the rule; Questions(kinds) has the 11 ids and the kind criteria list exactly the kinds passed. Implement; commit `feat(jev): fan-out questions and their combination`.

### Task 2: HTTP selector

**Files:** Create `internal/atom/jev/jev.go`, test `internal/atom/jev/jev_test.go` (httptest server, hand-written); modify `internal/config/config.go` (`Models.Jev{Model, Key, Endpoint string; Timeout Duration}`, validation), `internal/cli/atoms.go` (buildPolicy "jev": resolve key via the existing secret resolver used by notify; fallback = declared).

**Interfaces:**
- Produces: `type Policy struct{ Endpoint, Model, Key string; Timeout time.Duration; Client *http.Client; Kinds map[string]int; Weights Weights; Threshold float64; Fallback session.Policy }`; `func (p Policy) Choose(b session.Brief) (session.Verdict, error)`; `func (p Policy) Explain(b session.Brief) (Answers, session.Verdict, error)` (used by pick --explain).
- Behaviour: success and confidence ≥ Threshold → Combine; else `Fallback.Choose(b)` with reason prefixed `"jev <cause>; "`; Fallback's ErrNoKind passes through.

- [ ] Judge tests: request body has the pinned model, the state fields, all 11 questions, Bearer header; 200 → Combine's verdict; 401, 529 twice, timeout, malformed body, missing answer → fallback verdict with cause; key never in any error string; prompt truncation at 60,000 chars. Implement; commit `feat(jev): built-in Jev selector`.

### Task 3: `ness pick --explain` and doctor
**Files:** `internal/cli/pick.go`, `internal/cli/doctor_checks.go`, scripts `pick_jev.txtar` (fake server via an env-provided endpoint: `[models.jev] endpoint` set to a testscript server — use the existing script helpers or a tiny `httpfake` command registered in script_test.go), `doctor.txtar` addition.
- `--explain` (with `--json`): adds `"answers": {...}` (the Answers fields) and `"fallback": bool`; plain output prints one line per answer.
- doctor: when `policy = "jev"`, check the key resolves and `GET /v1/models` lists the pinned model (warn, never fail the gate, when offline).
- [ ] Judge script; implement; commit `feat(pick): explain the Jev answers; doctor checks the Jev key`.

### Task 4: Eval set (private data, committed harness)
**Files:** Create `evals/model-select/extract.py`, `label.py`, `score.py`, `calibrate.py`, `README.md`. Data only under `.private/model-select/` (gitignored).
- `extract.py PROJECT_TRANSCRIPT_DIRS... --n 150 --seed 7`: Agent/Task tool_use blocks from main transcripts (prompt, description, subagent_type, model, effort) joined with their subagent meta/transcript (actual model, effort, turns, cost via the ness price table numbers copied in a dict), stratified by subagent_type and model; writes `calls.jsonl`.
- `label.py`: for each call runs `claude -p --model opus --effort high` with a written rubric (in the file): output JSON `{"kind", "lowest_rung", "why"}`, rungs named from `ness pick` listing; writes `labels.jsonl`; `--spot 20` writes `spot.md` (20 random items with label and why) for the operator.
- `score.py --policy declared|jev`: runs `ness pick --json --explain --role subagent` per call (declared uses the labelled kind), reports exact / over-rung / under-rung rates and cost-weighted error (rung cost proxies: bulk 0.01, economy 0.07, lean 0.12, standard 0.21, deep 0.29, frontier 0.60, apex 1.19 $/task from research-tiering.md; under-rung costs one retry at the next rung), and the same for "what the session actually used".
- `calibrate.py`: grid over Weights (0.4..0.9 step 0.1) on the saved `--explain` answers (no new Jev calls), minimising cost-weighted error with under-rung weighted 2×; prints the Weights literal.
- [ ] No Go tests; each script has an `assert`-based self-check on a 3-row inline fixture (`python3 -I script.py --selftest`). Commit `feat(evals): model-select eval set harness`.

### Task 5: Run, calibrate, report (needs a working production Jev key)
- [ ] extract (research-lab and production-backend transcripts), label, spot file to the operator; score declared and jev; calibrate; update `DefaultWeights`; re-score; write `docs/discovery/findings/model-select/eval-results.md` (numbers only, no prompts). Commit `perf(jev): weights calibrated on the eval set`.
