> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Model select 2: enforcement Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make the ladder binding in long sessions: the pre-tool hook refuses Agent calls above the selector's recommendation (or logs them, per `models.enforce`), every spawn decision is stored, `ness stats` shows cost by rung and the hook's decisions, and the guidance survives compaction.

**Architecture:** Parsing and the decision are pure functions in `internal/hook` (like the existing guards); `internal/cli/hook.go` wires config, ladder, selector (`session.Pick`) and the store around them. A new store table records each spawn decision; stats reads it and the subagent transcripts.

**Tech Stack:** Go, sqlite store migrations (`internal/store/migrate.go`), testscript.

**Spec:** `docs/sessions/model-select/specs.md` §3, §5 "Stats"; plan 1 (`2026-10-21-model-select-1-ladder.md`) for `ladder`, `session.Pick`, `declared.NamedKinds`.

## Global Constraints
- `models.enforce`: `"refuse"` (default) or `"log"`; log never refuses and stores the verdict it would have given.
- Prompt lines, matched at line start, key case-insensitive: `kind: <name>` and `escalate: <reason>`. One escalate allows exactly one rung above the recommendation.
- Fork subagents (`subagent_type` "fork") are allowed and logged with verdict `fork`.
- The hook stays silent when unbound (existing rule) and fails open on an unreadable config (existing rule).
- Refusal text names: the requested rung, the recommended rung with model/effort, the selector's reason, and the two ways out (lower the rung, or add `escalate: <reason>`), plus `rule.Ref` to the ness-entry section "Model and effort".

## Review Focus
- An Agent call naming a full model id (`claude-opus-5-5`) instead of the alias: refused as off-ladder with the ladder listed (not a crash, not silently allowed).
- A named ness agent called with an explicit `model` above its rung (ness-sweep with `model: fable`): judged on the explicit pair, refused.
- `escalate:` present but the request is two rungs above: refused, message says one rung per escalate.
- The selector command times out inside the hook: the recommendation is base, the call is judged against base, the hook returns within timeout + 1s.
- Store busy/locked when logging: the verdict still applies (log error goes to stderr), never a blocked tool because of the log.

---

### Task 1: Store — spawn log

**Files:** Modify `internal/store/migrate.go` (next migration), create `internal/store/spawns.go`, test `internal/store/spawns_test.go`.

**Interfaces:**
- Produces: `type Spawn struct{ Session, AgentType, Kind, Requested, Recommended, Verdict, Reason string; At time.Time }`; `func (s *Store) AddSpawn(sp Spawn) error`; `func (s *Store) Spawns(session string) ([]Spawn, error)` (oldest first; session "" = all).
- Verdict values: `allowed`, `below`, `escalated`, `refused`, `fork`, `would-refuse`.

- [ ] Judge tests: round trip; ordering; filter by session; migration from the previous schema version keeps existing rows (follow the pattern of the newest migration test). Implement; `go test ./internal/store/`; commit `feat(store): spawn decision log`.

### Task 2: Hook — parse and decide (pure)

**Files:** Create `internal/hook/spawn.go`, test `internal/hook/spawn_test.go`; extend `toolInput` in `internal/hook/guards.go` with `Model`, `Effort`, `Prompt`.

**Interfaces:**
- Produces: `type Spawn struct{ AgentType, Model, Effort, Kind, Escalate string }`; `func ParseSpawn(p Payload) (Spawn, bool)` — true for tool `Agent`/`Task`; Kind and Escalate read from the prompt lines.
- Produces: `func Judge(requested, recommended int, escalate, enforce string) (verdict string, refuse bool)` — table:

  | requested vs recommended | escalate | enforce refuse | enforce log |
  |---|---|---|---|
  | ≤ rec−1 | any | below | below |
  | = rec | any | allowed | allowed |
  | = rec+1 | set | escalated | escalated |
  | = rec+1 | empty | refused (refuse) | would-refuse |
  | ≥ rec+2 | any | refused (refuse) | would-refuse |

- [ ] Judge tests: the full table; ParseSpawn on `kind: Search` (key case-insensitive, value lowercased), `  kind: run` (leading spaces ok), `escalate:` with empty reason counts as empty, `Task` legacy tool name, non-Agent tools false; fuzz target extended to ParseSpawn (existing fuzz pattern R-HOOK-015). Commit `feat(hook): spawn parsing and the rung verdict`.

### Task 3: Hook wiring

**Files:** Modify `internal/cli/hook.go` `preTool` (~line 156), `internal/config/config.go` (`Models.Enforce`, validation `oneOf("models.enforce", ..., "refuse", "log")`), `internal/rule` (new ref `rule.ModelEffort` → skill ness-entry, section "Model and effort"). Test: `internal/cli/testdata/script/hook_pretool_spawn.txtar`; update `hook_pretool_explore.txtar` fixtures whose Agent calls now need a kind or a pair (keep their intent).

**Interfaces:**
- Consumes: `hook.ParseSpawn`, `hook.Judge`, `ladder.New`, `session.Pick` with `Brief{Role:"subagent", Text: prompt, Kind, AgentType}`, `declared.NamedKinds`, `store.AddSpawn`.
- Requested rung: explicit Model+Effort → `l.Find`; neither and a named agent → its rendered rung `l.At(kinds[NamedKinds[type]])`; fork → verdict fork; anything else (one of the pair, off-ladder, unnamed agent without a pair) → refuse listing the ladder (`name model/effort` per rung) and pointing at `ness pick`. Effort-only or model-only is refused the same way.
- Recommendation: `session.Pick`; `ErrNoKind` with no named kind → refuse listing the kinds (in log mode: would-refuse, allowed).
- Explore guard keeps running first (unchanged).

- [ ] Judge testscript: default config — `general-purpose` with `kind: search`, `model: sonnet`, `effort: medium` → exit 0; same with `opus/medium` → exit 2 naming `economy` and `escalate`; with `escalate: needs judgment` and `opus/low` (lean, rec+1) → exit 0; `ness-sweep` with no pair → exit 0; `ness-sweep` with `model: fable, effort: medium` → exit 2; no kind, unnamed, pair given → exit 2 listing kinds; `claude-opus-5-5`/medium → exit 2 listing the ladder; fork → exit 0; `enforce = "log"` → the refused case exits 0; `ness stats --json` afterwards lists the spawns with verdicts; unbound → silent. Commit `feat(hook): refuse subagent calls above the recommended rung`.

### Task 4: Guidance that survives compaction

**Files:** `profiles/core/post-compact.md` (one line), `profiles/core/skills/ness-entry/SKILL.md` "Model and effort" section, `profiles/core/skills/ness-launch/SKILL.md` "Tiers" section, cheat sheet in `internal/cli/hook.go` (`cheatSheet`, one line for `ness pick`), and the tests that pin these texts (find them with grep for the current wording).

Exact text (copy):
- post-compact line: `- Models: every subagent call carries a \`kind: <name>\` line and the model and effort \`ness pick\` gives; the hook refuses a rung above the recommendation unless the prompt has \`escalate: <reason>\` (one rung).`
- ness-entry "Model and effort" (replace the section body):
  ```
  Your model and effort never change mid-session. The project's ladder runs cheapest to strongest
  around a base rung; `ness pick` prints it. Do routine work at your own rung and give the rare
  hard step to a subagent at a higher rung: start every subagent prompt with `kind: <name>`
  (search, run, implement, debug, research, review, verify, design) and pass the model and effort
  `ness pick --kind <name>` gives. Named agents (ness-sweep, ness-review, ness-verify,
  ness-design) carry their own. To go one rung above the recommendation, add a line
  `escalate: <reason>`; the hook refuses anything higher and records every call. When your own
  session needs more, write your state down (`ness inbox --boundary NOTE`), end your turn, and
  let your manager resume you with `--tier up`.
  ```
- ness-launch "Tiers" (replace the section body):
  ```
  A child runs at the project's base rung unless its job says otherwise: open it with
  `--kind <name>` and the selector picks the rung, or name one with `--tier` (a rung above the
  recommendation needs `--reason`). Choose by who judges the output: work a check or a reviewer
  will judge runs low; work whose output is itself the judgment (design between alternatives,
  conclusion audits) runs high. Escalate a stuck child with `ness open --resume ID --tier up`.
  ```
  and keep the `tier-roles` slot.

- [ ] Update the pinned-text tests, run `go test ./internal/cli/ ./internal/profile/ ./internal/sync/`; commit `docs(skills): ladder guidance in entry, launch and post-compact`.

### Task 5: Stats — rung and spawns

**Files:** `internal/stats/stats.go` (subagent transcripts: read `<transcript dir>/<client session>/subagents/agent-*.meta.json` for agentType, model, effort, and price the sibling `.jsonl`), `internal/cli/stats.go` (`--by rung`, a spawns summary), tests `internal/stats/stats_test.go` with fixtures under `internal/stats/testdata/`, `internal/cli/testdata/script/stats_json.txtar`.

**Interfaces:**
- `ness stats --by rung` groups cost by rung (session turns by the session's tier; subagent turns by `ladder.Find(meta model, meta effort)`, `off-ladder` otherwise) and by role (root, child, subagent).
- `--json` gains `"spawns": {"allowed":n,"below":n,"escalated":n,"refused":n,"would_refuse":n,"fork":n}` per session and in total.
- First read stats.go to check whether subagent `.jsonl` files are already counted (they may be, via sidechain lines): never count a turn twice; the test pins the total of a fixture session with one subagent.

- [ ] Judge tests: fixture with one session (opus/medium) and two subagents (sonnet/medium, opus/high) → three rung groups with the right costs; a subagent with a model not on the ladder → off-ladder; spawns counts from a seeded store. Commit `feat(stats): cost by rung and spawn decisions`.

### Task 6: Spec and design docs
- `docs/spec/hooks.md` (R-HOOK-03x rows for spawn), `docs/spec/stats.md`, `docs/spec/models.md` (R-MOD-060.. enforce), `docs/design/decisions.md`.
- [ ] `go test ./...` per package (never the gate by hand), `python3 .ness/audit.py` clean; commit `docs: subagent rung enforcement`.
