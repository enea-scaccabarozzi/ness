> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Models (MOD)

Packages: `internal/ladder`, `internal/atom/declared`, `internal/atom/{claude,codex}`, generated
`.claude/agents/ness-*.md` (sync). Sources: §2 (model policy seam), §3 (model choice, Codex), §4
(model ids), §5 (`--model` removed, `--effort` stays), §12, D17, D18, D23.

## Tier table

- **R-MOD-001** The default tier table MUST be, cheapest first: bulk = Haiku · medium (Claude) /
  gpt-6-luna · low (Codex); economy = Sonnet 5.5 · medium / gpt-6-luna · medium; lean = Opus 5.5 ·
  low / gpt-6.1-sol · low; standard = Opus 5.5 · medium / gpt-6.1-sol · medium; deep = Opus 5.5 ·
  high / gpt-6.1-sol · high; frontier = Fable 5.1 · medium / astra · medium; apex = Fable 5.1 ·
  high / astra · high.
  Verify: unit — the rendered default table equals these seven rows.
- **R-MOD-002** The generated skill text MUST name the project's base rung (`standard` unless
  `models.base` moves it) as every session's default (D23); a rung above the recommendation only
  per launch with a reason.
  Verify: testscript `core_text` — ness-launch Tiers text pinned.
- **R-MOD-003** The tier table MUST live in `.ness/ness.toml` `[tiers.NAME]` as
  `claude = "<model>/<effort>"`, `codex = "<model>/<effort>"`, defaulting to the R-MOD-001 table;
  ness MUST use the edited values at open and in generated subagents.
  Verify: testscript — change economy to another model; open passes it; `.claude/agents` updated
  after sync.
- **R-MOD-004** The frontier tier MUST pass `medium` effort explicitly at launch (Claude Code's
  default for Fable is high).
  Verify: contract(fake) — frontier launch argv contains effort `medium`.

- **R-MOD-005** `models.ladder` MUST order the rungs cheapest first, defaulting to the R-MOD-001
  order; every name on it MUST be a row of the tier table, else config load refuses naming
  `models.ladder`.
  Verify: unit — default order; a rung missing from `[tiers]` → refused naming it.
- **R-MOD-006** `models.base` (default `standard`) MUST be on the ladder; selectors answer an
  offset from it and every fallback lands on it.
  Verify: unit — base off the ladder → refused naming `models.base`.
- **R-MOD-007** `models.ceiling` (default empty: the top rung) MUST be on the ladder and not
  below base; no selector or step goes above it.
  Verify: unit — ceiling off the ladder or below base → refused naming `models.ceiling`.
- **R-MOD-008** `[models.kinds]` MUST map each work kind to its offset from base, defaulting to
  search −2, run −2, implement 0, debug 0, research +1, review +1, verify +1, design +1 (MS-D7); a project
  entry overrides that kind only.
  Verify: unit — defaults; `review = 2` overrides review and keeps the rest.

## Effort and model mechanics

- **R-MOD-010** Effort MUST be passed per launch with the client's effort flag, never via
  `CLAUDE_CODE_EFFORT_LEVEL` (it overrides subagent frontmatter).
  Verify: contract(fake) — see R-SESS-028; sync never writes that variable into settings.json.
- **R-MOD-011** `ness open --effort` MUST override the tier's effort for that launch only.
  Verify: contract(fake) — `--effort high` on standard → argv high; the stored tier stays
  standard/medium for later launches.
- **R-MOD-012** ness MUST NOT change a running session's model or effort.
  Verify: unit — no code path types `/model` or effort commands into a pane; fake runtime records
  none across the worker and session suites.
- **R-MOD-013** `ness open` MUST NOT accept `--model`.
  Verify: testscript — see R-CFG-031.

## Named subagents

- **R-MOD-020** Sync MUST generate `.claude/agents/ness-sweep.md`, `ness-review.md`,
  `ness-verify.md` and `ness-design.md`, each with the model and effort of its kind's rung
  (search, review, verify, design) from base, clamped to the ceiling, in its frontmatter;
  `CLAUDE_CODE_SUBAGENT_MODEL` is the search kind's model. At the defaults: Sonnet · medium,
  Opus · medium, Opus · high, Fable · medium.
  Verify: unit — golden render at the defaults; base economy, ceiling deep and a kinds override
  move the agents.
- **R-MOD-021** Generated skill text MUST name `ness-sweep` for searches instead of Explore
  (Explore runs on the main model).
  Verify: review + unit — grep of rendered skills: search instructions reference `ness-sweep`.

## Model ids

- **R-MOD-030** Codex model ids MUST be validated against the output of `codex debug models`;
  a refusal MUST name the listed models and say to upgrade Codex, check the account's models, or
  set the tier's `codex` value.
  Verify: contract(fake+real) — fake lists `gpt-6-luna` only; `astra` and `gpt-6.1-sol` rows fail.
- **R-MOD-031** Claude model ids MUST be resolved by the client before launch (R-SESS-004); both
  aliases (`fable`, `opus`, `sonnet`, `haiku`) and full ids MUST be accepted.
  Verify: contract(fake+real) — `opus` and a full id accepted; unknown id → refusal naming the
  tier.
- **R-MOD-032** Codex tiers MUST use explicit model ids (no aliases), validated per R-MOD-030.
  Verify: unit — `codex = "best/medium"` → refused.
- **R-MOD-033** Accepted effort values MUST be defined per client atom and validated at config
  load and at `--effort`.
  Verify: unit — claude atom rejects a Codex-only effort; codex atom rejects a Claude-only one.

## Model policy seam

- **R-MOD-040** ness MUST ship the `declared` selector (the default): the caller declares a work
  kind (or a named subagent implies one) and the selector answers that kind's offset from base; a
  missing or unknown kind is refused listing the kinds.
  Verify: unit — ness-sweep → −2; kind "Design" → refused listing the kinds.
- **R-MOD-041** The selector contract MUST take (brief text, role root|child|subagent, client,
  kind, agent type) and return (offset from base, confidence 0..1, one-line reason).
  Verify: contract(fake) — a fake selector passes the seam's contract suite.
- **R-MOD-042** The verdict MUST become a rung clamped to the ladder and the ceiling; a selector
  error, or confidence below `models.router_threshold` (default 0.8), MUST give the base rung with
  a reason starting `base:`.
  Verify: unit — confidence 0.79 → base, "low confidence"; 0.8 → the offset's rung; +50 → apex
  "(clamped)"; an error → base and the error.
- **R-MOD-043** The selector's reason MUST be recorded for stats (R-STAT-009); an explicit
  `--tier` records `explicit` or `explicit: <--reason>` (also on a resume, which never asks the
  selector), `--tier up|down` records `up from X`/`down from X` or `already at the top`/`already
  at the bottom`, and a rung above the ceiling is refused. `ness open` prints the rung and reason.
  Verify: testscript — store and `ness stats --json` hold `explicit: novel design`.
- **R-MOD-045** The `command` selector MUST run `models.command` in the project root (sh -c,
  bounded by `models.command_timeout`, default 10s) with the brief on stdin as
  `{"text": "...", "role": "root|child|subagent", "client": "...", "kind": "...", "agent_type": "..."}`
  and read one JSON object from its output: `{"offset": -1, "confidence": 0.9, "reason": "..."}`.
  A non-zero exit, a timeout, output that is not that object, a confidence outside 0..1 or a
  reason with a newline MUST be an error naming the command (the launch then gets base). Only
  stdout is parsed; stderr appears in the error text of a failure only.
  Verify: unit — fake runner: stdin keys, verdict verbatim, each failure names the command;
  testscript — a script printing offset −1 opens lean.
- **R-MOD-046** `ness pick [--kind NAME] [--role subagent|child] [--agent TYPE] [--stdin] [--json]` MUST
  run the configured selector on the task text, read from stdin only under `--stdin` (a stdin that
  never closes must not hang a pick), and print `RUNG  MODEL/EFFORT  REASON`,
  plus `model: M  effort: E` for a Claude subagent; `--json` prints
  `{"rung","model","effort","index","confidence","reason"}`. With no kind, agent or text it MUST
  print the ladder (name, claude, codex) marking base and ceiling (`--json`:
  `{"schema","base","ceiling","rungs":[{"name","claude","codex"}]}`); `--kind sweep` MUST mean
  `search`; a missing or unknown kind MUST exit 4 listing the kinds.
  Verify: testscript — search → economy sonnet/medium; ness-design → deep; base economy
  shifts search to bulk; `--kind sweep` picks as search; a heredoc stdin without `--stdin` is not read.
- **R-MOD-064** Under policy `jev`, `ness pick --explain` MUST add Jev's answers: `--json` adds
  `"fallback": false` and `"answers"` (`kind, kind_confidence, depth, ambiguity, blast, novelty,
  breadth, check, mechanical, design, judge, chain`); the plain output adds one `NAME VALUE` line
  per answer after the pick line. A fallback MUST print `"fallback": true` with the cause in the
  reason; after low confidence (or kind `other`) it keeps `"answers"`, after any other fallback
  (401, no key, unreachable, malformed) `"answers"` is absent. Under any other policy `--explain`
  adds nothing. The key MUST never appear in output or request body.
  Verify: testscript pick_jev — fake API: answers, low confidence, 401, unset key, offline, declared.
- **R-MOD-065** The `jev` selector MUST send one `POST <endpoint>/v1/systemone` (a trailing `/` on
  the endpoint trimmed) with `Authorization: Bearer <key>` and `{"model": models.jev.model,
  "state": {"task", "role", "agent_type", "declared_kind"} (empty ones omitted), "questions"}`; the
  kind choice MUST offer the configured kinds in their configured order (built-ins as R-MOD-008,
  then project kinds by name) and last `other`. A task over 60,000 bytes MUST be cut to its head on
  a UTF-8 rune boundary plus a `[... truncated]` marker.
  Verify: unit internal/atom/jev — request shape, option order, trailing slash, ASCII and multi-byte cuts.
- **R-MOD-066** A 429 or 529 MUST be retried once after a backoff of at most 1 s; the request,
  backoff and retry together MUST stay within `models.jev.timeout`. No other status is retried.
  Verify: unit internal/atom/jev — 429 then ok, 529 twice, 401 and 500 once, a hanging endpoint.
- **R-MOD-067** Any failure (no key, a non-200 status, a timeout, an unreadable or malformed
  answer, kind `other`, or a kind confidence below `models.router_threshold`) MUST fall back to the
  declared selector with the reason `jev: <cause>; <declared reason>`; with no declared kind the
  fallback MUST be base (confidence 1, reason `jev: <cause>; no kind: base`), never a refusal.
  The key MUST never appear in a reason, an error or output; a response body is never quoted.
  Verify: unit internal/atom/jev — each cause; testscript hook_pretool_spawn_jev — Jev down, no kind line, standard allowed.
- **R-MOD-068** An answer MUST be refused as malformed when missing, when a choice is not among
  the options offered, when a noul or a confidence is outside 0..1, when a score is outside its
  levels, or when its probabilities do not sum to 1.
  Verify: unit internal/atom/jev — one fallback row per case.
- **R-MOD-069** `models.jev.timeout` MUST be positive and at most 10 s, and `models.jev.endpoint`
  MUST be `https://`, or `http://` to a loopback host (127.0.0.0/8, ::1, localhost); otherwise a
  config error naming the field.
  Verify: unit internal/config — accepted and refused endpoints and timeouts.
- **R-MOD-044** `--fallback-model` MUST be passed only when the project sets a fallback
  (R-SESS-009).
  Verify: contract(fake).
- **R-MOD-060** `models.enforce` MUST be `"refuse"` (the default) or `"log"`; any other value is a
  config error.
  Verify: testscript `hook_pretool_spawn_limits`.
- **R-MOD-061** Under `models.enforce = "log"` the hook MUST never refuse a subagent call for its
  rung or kind, and MUST store the verdict it would have given as `would-refuse`.
  Verify: testscript `hook_pretool_spawn`.
- **R-MOD-062** The core post-compact list MUST carry the ladder line (kind line, `ness pick`,
  one-rung escalate) and the cheat sheet a `ness pick` line, so both survive compaction.
  Verify: testscript `hook_session_start`.
- **R-MOD-063** ness-entry "Model and effort" and ness-launch "Tiers" MUST state the ladder: the
  kind line, `ness pick`, named agents' own rungs, the one-rung escalate, `--kind`/`--tier`/`--reason`
  for children and `--tier up` to escalate.
  Verify: testscript `core_text` — both sections pinned.

## Decided details

- **R-MOD-050** For Codex, sync MUST write only `[agents]` defaults in `.codex/config.toml`; no
  named Codex subagents are generated. The defaults are the search kind's codex rung.
  Verify: unit — rendered config.toml has an `[agents]` table and no per-agent entries.
