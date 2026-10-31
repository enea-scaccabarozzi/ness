> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Model select 1: ladder and selectors Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the three-tier text policy with an ordered ladder of (model, effort) rungs, a project `base`, and swappable selectors (declared, command) that answer an offset from base; wire them into `ness open` and a new `ness pick`, and render the named subagents from base.

**Architecture:** A new pure package `internal/ladder` orders the tier table and clamps offsets. The policy seam in `internal/session` changes from "return a tier" to "return an offset" (`Verdict`); `session.Pick` turns a verdict into a rung and is the only place that falls back to base. Selectors are atoms (`internal/atom/declared`, `internal/atom/cmdpolicy`), built in `internal/cli/atoms.go`. Plan 2 adds the Agent hook, store log and stats; plan 3 adds the Jev atom and the eval set.

**Tech Stack:** Go (mise), cobra, BurntSushi toml, testscript (`internal/cli/testdata/script/*.txtar`).

**Spec:** `docs/sessions/model-select/specs.md` (§1, §2, §3 "ness open" and "Escalation"); decisions MS-D3, MS-D5.

## Global Constraints
- Default ladder, cheapest first: bulk haiku/medium · gpt-6-luna/low; economy sonnet/medium · gpt-6-luna/medium; lean opus/low · gpt-6.1-sol/low; standard opus/medium · gpt-6.1-sol/medium; deep opus/high · gpt-6.1-sol/high; frontier fable/medium · astra/medium; apex fable/high · astra/high.
- Default `models.base = "standard"`, `models.ceiling = ""` (top of ladder), `models.policy = "declared"`.
- Default kinds: search −2, run −2, implement −1, debug 0, research 0, review 0, verify +1, design +2.
- Named agents: ness-sweep=search, ness-review=review, ness-verify=verify, ness-design=design.
- R-MOD-010 and R-MOD-012 stand: effort per launch flag, never `CLAUDE_CODE_EFFORT_LEVEL`; never change a running session.
- Interfaces declared by the consumer, ≤3 methods; no package-level mutable state; black-box tests with hand-written fakes; every package comment one sentence.
- `models.policy = "text"` is refused at config load with: `models.policy "text" was replaced by "declared" (ladder and kinds; see ness pick)`.

## Review Focus
- A project that redefines `[tiers.standard]` only partially (claude row only) still gets the default codex row (existing fillTiers behaviour, now for 7 rows).
- A `ladder` naming a rung missing from `[tiers]`, or `base`/`ceiling` not on the ladder, or `ceiling` below `base`: refused at config load naming the key.
- A command selector that hangs, exits non-zero, or prints non-JSON: `Pick` returns base with a reason naming the failure, within `models.command_timeout` (default 10s); open does not refuse.
- `--tier up` on the top rung or at the ceiling stays there and says so in the reason; `--tier down` at the bottom likewise.
- An offset that is huge (±100) from a command selector is clamped, not an index panic.

---

### Task 1: Config — ladder keys and seven default rungs

**Files:**
- Modify: `internal/config/config.go` (Models struct ~138, DefaultTiers ~173, defaults ~204, ModelPolicies ~398, validate ~435, validateNumbers ~552)
- Test: `internal/config/config_test.go`
- Modify: `docs/spec/core-config.md`, `docs/spec/models.md` (R-MOD-001 table → 7 rows; new R-MOD-005..008 for ladder/base/ceiling/kinds)

**Interfaces:**
- Produces: `config.Models{Policy string; Base string; Ceiling string; Ladder []string; Kinds map[string]int; Command string; CommandTimeout Duration; RouterThreshold float64; FallbackModel string}` with toml keys `policy, base, ceiling, ladder, kinds, command, command_timeout, router_threshold, fallback_model`.
- Produces: `config.DefaultLadder() []string`, `config.DefaultKinds() map[string]int`, `config.ModelPolicies = []string{"declared", "command"}` (plan 3 appends "jev").

- [ ] **Step 1: failing tests** `TestDefaultLadder` (DefaultTiers has the 7 rows above exactly; Config of an empty file has Ladder == DefaultLadder, Base "standard", Policy "declared", Kinds == DefaultKinds, CommandTimeout 10s), `TestLadderValidation` table: ladder rung not in tiers → error mentioning `models.ladder`; base not on ladder → `models.base`; ceiling below base → `models.ceiling`; policy "command" without command → `models.command`; policy "text" → the exact replacement message; a kinds entry overrides one default and keeps the others.
- [ ] **Step 2:** `go test ./internal/config/` → FAIL.
- [ ] **Step 3:** implement; kinds merge like fillTiers (file values win per key). `validateNumbers` refuses `command_timeout <= 0`.
- [ ] **Step 4:** `go test ./internal/config/` → PASS.
- [ ] **Step 5:** update spec rows; commit `feat(config): model ladder, base, ceiling and kinds`.

### Task 2: `internal/ladder` — order, clamp, find

**Files:**
- Create: `internal/ladder/ladder.go`, `internal/ladder/ladder_test.go` (package `ladder_test`)

**Interfaces:**
- Consumes: `config.Config` (Tiers, Models), `session`-free (no import of session).
- Produces:
  - `type Rung struct{ Name, Model, Effort string }`
  - `type Ladder struct{ Rungs []Rung; Base, Ceiling int }` (indices; Ceiling = len−1 when unset)
  - `func New(cfg *config.Config, client string) Ladder` (config already validated)
  - `func (l Ladder) At(offset int) (Rung, bool)` — rung at base+offset clamped to [0, Ceiling]; bool false when clamped.
  - `func (l Ladder) Index(name string) (int, bool)`
  - `func (l Ladder) Step(name string, delta int) (Rung, bool)` — for `--tier up/down`; clamps to [0, Ceiling].
  - `func (l Ladder) Find(model, effort string) (int, bool)` — exact match on the client's model/effort.

- [ ] **Step 1: failing tests** — a separate subagent writes `ladder_test.go` from these Interfaces and the clamp table before Step 3: default ladder claude: At(0)=standard opus/medium; At(-2)=economy; At(+2)=frontier; At(+9)=apex,false; At(-9)=bulk,false; with Ceiling "frontier": At(+3)=frontier,false; base "economy": At(-2)=bulk, At(+2)=standard; codex client returns codex ids; Step("apex",+1)=apex,false; Find("opus","high")=4,true; Find("opus","max")=_,false.
- [ ] **Step 2:** `go test ./internal/ladder/` → FAIL. **Step 3:** implement. **Step 4:** PASS. **Step 5:** commit `feat(ladder): ordered rungs with base and ceiling`.

### Task 3: Selector seam (Verdict, Pick) and the declared selector

**Files:**
- Modify: `internal/session/policy.go` (whole file)
- Modify: `internal/testkit/contracts.go:152-182` (FakeRouter → FakeSelector, PolicyContract)
- Delete: `internal/atom/textpolicy/` (both files)
- Test: `internal/session/policy_test.go` (new, package `session_test`)

**Interfaces:**
- Produces (replacing Brief/Choice/Policy/ErrNeedTier):
  - `type Brief struct{ Text, Role, Client, Kind, AgentType string }` — Role is root|child|subagent.
  - `type Verdict struct{ Offset int; Confidence float64; Reason string }`
  - `type Policy interface{ Choose(Brief) (Verdict, error) }`
  - `var ErrNoKind = errors.New("name the work kind")` — a selector that needs a kind and has none.
  - `type Choice struct{ Tier, Model, Effort string; Index int; Confidence float64; Reason string }`
  - `func Pick(p Policy, b Brief, l ladder.Ladder, threshold float64) (Choice, error)`: verdict → `l.At(offset)`; on selector error the Choice is the base rung with Reason `"base: <err>"` and the error is returned; confidence < threshold → base rung, Reason `"base: low confidence (0.42): <selector reason>"`, nil error; clamped → reason suffixed `" (clamped)"`.
  - `testkit.FakeSelector{Offset int; Confidence float64; Err error}`; `testkit.PolicyContract(t, p)`: for Brief{Kind:"implement"} answer has confidence in 0..1 and a one-line non-empty reason.

- [ ] **Step 1: failing tests** (`policy_test.go`, written by the judge subagent): FakeSelector offset −2 → economy, sonnet, medium, Index 1; Err → standard and the error returned (rejects an implementation that refuses); confidence 0.79 at threshold 0.8 → standard with "low confidence"; 0.8 → the offset's rung; offset +50 → apex with "(clamped)".
- [ ] **Step 2:** FAIL. **Step 3:** implement the seam and the declared atom below in the same task, so the build stays green: `buildPolicy(d, cfg)` returns the declared atom and `choose` in open.go calls `Pick` with the explicit `--tier` path unchanged (Task 5 rewrites it). **Step 4:** `go test ./internal/session/ ./internal/testkit/ ./internal/atom/declared/ ./internal/cli/` PASS. **Step 5:** commit `refactor(session): selector seam answers an offset; declared selector`.

#### Declared selector atom (same task)

**Files:**
- Create: `internal/atom/declared/declared.go`, `declared_test.go`
- Modify: `internal/cli/atoms.go:52-57` (buildPolicy takes cfg)

**Interfaces:**
- Produces: `type Policy struct{ Kinds map[string]int }`; `Choose(b)`: kind from `b.Kind`, else from `NamedKinds[b.AgentType]`; unknown/missing → `fmt.Errorf("%w: one of %s", session.ErrNoKind, sorted kinds)`; known → `Verdict{Offset: Kinds[k], Confidence: 1, Reason: "kind " + k}`.
- Produces: `var NamedKinds = map[string]string{"ness-sweep":"search","ness-review":"review","ness-verify":"verify","ness-design":"design"}` (read-only; used by sync and the hook).
- Produces: `func buildPolicy(d Deps, cfg *config.Config) session.Policy` — `d.Policy` when set (tests), else by `cfg.Models.Policy`.

- Tests (judge subagent, before the code) include `testkit.PolicyContract`; agent type ness-sweep with no kind → offset −2; kind "Design" (case) → ErrNoKind (kinds are lowercase, exact); a project kinds override `review = 1` is honoured.

### Task 4: Command selector atom

**Files:**
- Create: `internal/atom/cmdpolicy/cmdpolicy.go`, `cmdpolicy_test.go`, `testdata/` scripts
- Modify: `internal/cli/atoms.go`

**Interfaces:**
- Consumes: the existing shell seam (`Deps.Shell`, see cli.go:59 — runs commands with stdin, a timeout and an exit code); declare the one-method port it needs in this package.
- Produces: `type Policy struct{ Command, Dir string; Timeout time.Duration; Run Runner }`; stdin JSON `{"text","role","client","kind","agent_type"}`; stdout one JSON object `{"offset":int,"confidence":float,"reason":string}`. Non-zero exit, timeout, bad JSON, confidence outside 0..1, or a reason with a newline → error naming the command and the cause (Pick turns it into base).
- Doc: `docs/spec/models.md` new rows R-MOD-045 (protocol) with the JSON shapes verbatim.

- [ ] Steps 1–5: tests with fake Runner (hand-written) covering each failure and one success; one testscript in Task 5 runs a real script. Commit `feat(atom): command selector`.

### Task 5: `ness open` — optional --tier, --kind, --reason, up/down

**Files:**
- Modify: `internal/cli/open.go:46-56` (flags), `:184-202` (choose), `:253`, `:511-522` (resumeTier)
- Test: `internal/cli/testdata/script/open_ladder.txtar` (new), update `open_refusals.txtar:32-34` (valid rung list now 7 names, ladder order)
- Modify: `docs/spec/models.md` R-MOD-040..043 rewritten; `docs/spec/sessions.md` R-SESS-023 (`--tier` optional)

**Interfaces:**
- Consumes: `session.Pick`, `ladder.New`, `buildPolicy(d, cfg)`.
- Produces: flags `--kind NAME` and `--reason TEXT`; `--tier` accepts a rung name, `up` or `down` (`up`/`down` only with `--resume`).
- Behaviour:
  - no `--tier`: `Pick` on Brief{Text: brief, Role: root|child, Kind: --kind}; a declared selector without kind gives base silently (reason `"base: no kind"`), not a refusal.
  - explicit `--tier R`: rung must be on the ladder and ≤ ceiling; when R's index > the recommendation's index and `--reason` is empty → refuse (exit 4) `--tier R is above the recommended RUNG for this brief (REASON); pass --reason TEXT to record why`; recorded `RouterReason` is `"explicit: " + reason` or `"explicit"`.
  - `--tier up|down` with `--resume`: `l.Step(stored, ±1)`; at the end → same rung, reason `"already at the top"`/`"already at the bottom"`.
  - `--effort` still overrides effort only.

- [ ] **Step 1:** failing testscript (judge subagent writes it): no tier + `--kind design` → stored tier frontier; no tier, no kind → standard; `--tier frontier` with a brief whose kind is implement and no `--reason` → exit 4 mentioning `--reason`; same with `--reason 'novel design'` → ok and stats `--json` shows the reason; resume `--tier up` from standard → deep; `--tier up` on apex → apex; `policy = "command"` with a testdata script printing offset −1 → lean.
- [ ] **Step 2:** FAIL. **Step 3:** implement. **Step 4:** `go test ./internal/cli/ -run 'TestScript/open'` PASS. **Step 5:** commit `feat(open): rung from the selector; --kind, --reason, --tier up/down`.

### Task 6: `ness pick`

**Files:**
- Create: `internal/cli/pick.go`; register in `internal/cli/cli.go:221`
- Test: `internal/cli/testdata/script/pick.txtar`
- Modify: `docs/spec/models.md` R-MOD-046

**Interfaces:**
- `ness pick [--kind NAME] [--role subagent|child] [--agent TYPE] [--json]`, task text on stdin (optional). Prints `RUNG  MODEL/EFFORT  REASON` and, for role subagent, the Agent call line `model: M  effort: E`; `--json` prints `{"rung","model","effort","index","confidence","reason"}`. With no argument and no stdin it prints the ladder (name, claude, codex) with base and ceiling marked. ErrNoKind → exit 4 listing kinds.

- [ ] Steps 1–5: script covers kind search → economy sonnet/medium; `--agent ness-design` → frontier; ladder listing marks `base`; base "economy" in ness.toml shifts search to bulk. Commit `feat(cli): ness pick`.

### Task 7: Named agents and subagent defaults from base

**Files:**
- Modify: `internal/sync/render.go:225` (env model), `:246-261` (codex), `:275-310` (subagents)
- Test: `internal/sync/render_test.go:101` (fix stale haiku golden)
- Modify: `docs/spec/models.md` R-MOD-020, R-MOD-050

**Interfaces:**
- Consumes: `ladder.New(cfg, "claude"|"codex")`, `declared.NamedKinds`, `cfg.Models.Kinds`.
- Behaviour: each named agent's model and effort = `At(Kinds[NamedKinds[name]])`; `CLAUDE_CODE_SUBAGENT_MODEL` and the codex `[agents]` defaults = `At(Kinds["search"])` (economy at base standard).

- [ ] Steps 1–5: tests: default → ness-sweep sonnet/medium, ness-review opus/medium, ness-verify opus/high, ness-design fable/medium; `base = "economy"` → ness-sweep haiku/medium, ness-design opus/medium; ceiling "deep" → ness-design opus/high. Commit `feat(sync): named agents follow the base rung`.

### Task 8: Docs and decisions
- Modify: `docs/design/ness-design.md` §12 (one paragraph: ladder, base, selectors), `docs/design/decisions.md` (D-entries mirroring MS-D3..D5), `CHANGELOG` untouched (release-please).
- [ ] Run `go test ./...` (package-level only, never the gate by hand) and `ness doctor` in the worktree; commit `docs: model ladder and selectors`.
