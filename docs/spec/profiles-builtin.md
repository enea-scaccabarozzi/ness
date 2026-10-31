> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# Core skill text and built-in profiles (BLT)

Directory: `profiles/` (embedded), core skill text rendered by sync. Sources: §2 (core, where
pieces land), §3 (sizes), §7, §8 (sizing guidance), §10, §11, §12, §13, §14 (skill-text fixes),
§16, D19, D20, D25, D26, D27, roadmap L9.

Most requirements here constrain rendered text. "Verify: unit" means a golden/grep test over the
rendered output of `ness sync` in a fixture repo; "review" means a fresh reviewer reads the
rendered text against the requirement.

## Every built-in

- **R-BLT-001** Each built-in profile (with the profiles it `requires`) MUST render in a fixture
  repo with zero conflicts and `ness sync --check` exiting 0 afterwards.
  Verify: testscript — one script per built-in.
- **R-BLT-005** All five built-ins enabled together MUST render without conflict.
  Verify: testscript — `--profile` for all five → exit 0; `sync --check` → 0.
- **R-BLT-006** Every skill a built-in profile renders MUST be named with the `ness-` prefix (inside
  the owned `.agents/skills/ness-*/` glob).
  Verify: unit — every skill directory in `profiles/` starts with `ness-`.
- **R-BLT-002** A skill rendered by a built-in MUST NOT have the same name as a skill in the
  global locations of R-SESS-006 on the CI machine image (passes the shadowing check).
  Verify: testscript — doctor shadowing check passes after each built-in renders.
- **R-BLT-003** Built-in text MUST contain no project specifics (no project names, hostnames,
  private identifiers): when `.private/denylist.txt` (gitignored) exists, a test MUST scan every
  rendered built-in file for its entries; CI, which lacks the file, skips the scan.
  Verify: unit — with a fixture denylist entry planted in a rendered file the scan fails; without
  the file the test reports skipped; review.
- **R-BLT-004** Feature skill text (probes, notify, contracts) MUST be rendered only when the
  feature is enabled.
  Verify: testscript — enable/disable each; the feature's skill directory appears/disappears.

## Core text

- **R-BLT-010** Core MUST render lifecycle skills including `ness-entry` and `ness-launch` under
  `.agents/skills/`.
  Verify: testscript — directories exist after `ness init`.
- **R-BLT-011** Core text MUST define the three core sizes answer, quick task and full session,
  as a list slot profiles can append to.
  Verify: unit — rendered sizes list contains the three; a profile append adds a fourth.
- **R-BLT-012** Core text MUST define the session record files `brief`, `specs`, `plan`,
  `decisions`, `report` under `<sessions>/<root>/[<child>/]`.
  Verify: unit — grep of ness-entry.
- **R-BLT-109** The Session record section of `ness-entry` MUST state that decision ids are the
  root's, that a child's `decisions.md` is plain dated notes without ids, never cited upward, and
  that `report.md` carries a `## Lessons` heading; `ness-entry` MUST have a `Lessons` heading so
  the close refusal's pointer resolves (FF-D6; R-SESS-153). It reconciles with the `program` rule
  on children and ledgers (R-BLT-033).
  Verify: unit `session_record_test` — grep of the embedded ness-entry.
- **R-BLT-101** `ness-entry` MUST say how a conversation that is not a session yet sizes the
  request, proposes or starts a session (starting at once when the operator asked for the work),
  promotes itself with `ness open --root --here`, and falls back to `ness open --root` when
  `--here` is refused.
  Verify: unit — grep of ness-entry.
- **R-BLT-102** `ness-entry` MUST say when a session asks the operator directly (what its manager
  cannot settle: a decision reserved to the operator, a fact only the operator has, an action
  only the operator can take), that the operator works only in the session's chat (never `ness inbox`,
  never answering on the phone, which only notifies), that every ask is written in full in the
  chat and also filed with `ness send --to operator` as the harness's record, what such an item
  contains for an operator who has read nothing (the question, the facts needed, options with a
  recommendation, what happens meanwhile), that a root reports progress with a `checkpoint`
  (`--intent checkpoint`, headings Done, Now, Next, and Ask when it needs an answer), and that the
  answer is recorded with `ness reply` (CX-17).
  Verify: unit — grep of ness-entry.
- **R-BLT-103** Core text MUST name the three autonomy levels (plan first; report, then act; act
  within the brief), state that the operator's words set the level and "me" in them stays the
  operator, that a root never lowers a child's level, and that a plan the operator approved in
  advance skips its checkpoints until work leaves its scope; `ness-launch`'s brief contents MUST
  include the autonomy level (CX-18).
  Verify: unit — grep of ness-entry and ness-launch.
- **R-BLT-104** `ness-entry`'s reserved decisions MUST list irreversible actions, money, accounts
  and credentials, publishing outside the repository, policy changes, scope beyond the brief, and
  non-trivial product decisions with any product call a test, a contract or a frozen file would
  lock in (FF3-D10); it MUST say a reserved decision is raised as a `decision` item with options
  and a recommendation, and that a trivial product call is marked PROVISIONAL in the handoff or
  report; the slot MUST still accept a project's additions (CX-18).
  Verify: unit — grep; the fixture profile's append lands after the list.
- **R-BLT-105** `ness-entry` MUST say how a root closes itself: `ness close <id> --authorized
  "<the operator's words>"`, only after the operator said so in the chat, and that its tab stays
  (CX-19).
  Verify: unit — grep.
- **R-BLT-106** `ness-launch` MUST tell a session to always pass `--subject 'type(scope): summary'`
  to `ness integrate`, and that a project not on `squash` ignores it with a warning (it never
  fails).
  Verify: unit — grep.
- **R-BLT-107** Core MUST append to the `ness-entry#post-compact` list slot the rules a compaction
  loses: how to talk to the operator (decision-first and short, a page for long content,
  `ness send --to operator` only for what the manager cannot settle) and the autonomy rule (use the
  level the brief states; a root never lowers a child's level; reserved decisions stay the
  operator's). The lines are printed by `session-start` after a compaction and not after startup,
  resume or clear, and the whole compact output stays within 20 lines with a two-line project
  profile.
  Verify: testscript — `hook_session_start`.
- **R-BLT-108** (CHANGED 2026-10-29, operator: "All interactions between me and sessions should land in chat") The core post-compact list MUST state that a root's every ask goes through
  `ness send --to operator`, written in full in the chat, which is the only place the operator reads.
  Verify: testscript — the rendered After compaction section of ness-entry, and the
  session-start hook output after compaction.
- **R-BLT-013** Core text MUST state the waiting rule: to wait for a reply or a child, pull the
  inbox once, then end the turn; never poll with Monitor, cron, ScheduleWakeup or sleep loops.
  Verify: unit — grep for the rule in ness-entry (and the communicate section).
- **R-BLT-014** Core text MUST tell agents that subagents write results to a file and return the
  path plus a summary of at most about 2k tokens (≈ 8,000 characters).
  Verify: unit — grep for "8,000 characters" (or "2k tokens") in the rendered rule.
- **R-BLT-015** Core text MUST state that model and effort never change mid-session; escalation
  is by resuming one tier up.
  Verify: unit — grep.
- **R-BLT-016** Core text MUST provide the slots `ness-entry#stop-and-ask` (list),
  `ness-entry#post-compact` (list), `ness-launch#tier-roles` (list) and a reserved-vs-delegated
  decisions slot.
  Verify: unit — each slot accepts an append in a fixture profile.
- **R-BLT-017** Core text MUST state: operator messages decision-first and short (a page for long
  content); lessons appended at handoff and loaded at entry; briefs do not promote side remarks to
  rules; a root delegates anything over a few edits.
  Verify: review.
- **R-BLT-018** Core text MUST give the probe sizing guidance of §8 when `probes` is enabled
  (floor first with a positive control; raise `sample` as floor coverage falls; skip the floor
  when no command can judge; narrow named scopes; random timing when risk depends on the hour).
  Verify: review.
- **R-BLT-019** The router block MUST be inert for contributors without ness (R-SYNC-061).
  Verify: unit.

## `program` (D19, §10)

- **R-BLT-030** `program` MUST add the size "program" via the sizes list slot.
  Verify: unit.
- **R-BLT-031** `program` text MUST describe the six steps: read back (`ness-sweep` survey, brief
  restated as roadmap r1 with parts, dependencies, holes, questions); only invariants at the root;
  everything else a vertical with reason, reopen trigger, owner; breadth-first refinement until the
  operator accepts the map; waves of verticals whose inputs exist, each result folding back as a
  new map revision; children integrate into the root branch and the root releases.
  Verify: review — one reviewer checklist item per step.
- **R-BLT-032** `program` MUST seed a `roadmap.md` template with sections Invariants, Verticals
  (with a spec-owner column), Waves, Conduct, Releases, Autonomy and a recovery line, preceded by a Program doctrine section; it also seeds `docs/doctrine.md`.
  Verify: unit — headings and the spec-owner column present.
- **R-BLT-033** `program` MUST seed an append-only `decisions.md` ledger with stable ids, and a
  committed `briefs/` directory.
  Verify: unit — files exist; template states append-only and id format.
- **R-BLT-034** The child brief template MUST have slots: question, outcome, invariants by id,
  operator words verbatim by id, sources, rights and limits, tier, spec owner, integration
  target, how to report.
  Verify: unit — each slot heading present.
- **R-BLT-035** `program` text MUST contain the rules: grounding runs in subagents and the root
  reads conclusions; a named operator preference becomes an explicit open decision; the root
  brings every child packet into its chat as one-word answers; the root never accepts
  operator-owned specs; injected work becomes a map row before its child opens; a checkpoint to
  the operator at each landing, plan change or scoring change (replacing a heartbeat line at each
  wave event); "since you left" from decision ids newer than the
  operator's last message; ask only for what the root cannot get itself.
  Verify: review — one checklist item per rule.
- **R-BLT-036** `program` text MUST require a new map revision on every fold or new vertical,
  with a recovery line naming the last decision id.
  Verify: review.
- **R-BLT-037** `program` text MUST make a verification/cutover phase a map section with its own
  probes, and send fixes to shared tooling to base via a quick task.
  Verify: review.
- **R-BLT-038** `program` MUST provide the single slot `ness-program#autonomy` (Autonomy,
  asked per run, recorded by the operator) and the slot `operator-review-format` with default
  the checkpoint page rule.
  Verify: unit — default render says "Checkpoint page"; a local `set` replaces it.
- **R-BLT-039** `program` MUST add no new commands.
  Verify: testscript — command list unchanged with program enabled.
- **R-BLT-110** `program` MUST declare four profile checks, all `for = "child"`, each pointing at
  `ness-program` and the section of the rule it enforces (R-CHK-001..014; FF-D6): at `open`,
  `autonomy-recorded` (section Autonomy) refuses while `## Autonomy` of `docs/program/roadmap.md`
  lacks a mode word and a `YYYY-MM-DD` date, its message telling the root to ask the operator and
  record the mode, their words and the date under `## Autonomy`; `checkpoint-before-wave` (section
  Checkpoints) refuses while a child landed since the root's last checkpoint to the operator;
  `recovery-line` (section Rules) refuses while `Recovery line: last decision id D<n>` differs from
  the last `D<n>` of `docs/program/decisions.md`; at `integrate`, `no-child-decision-ids` (section
  Rules) refuses a branch that adds to `<record>/decisions.md` a decision id (`- **D1**`, `C2-D7`,
  `L0a-D3`); plain dated notes without ids, or no decisions.md, integrate. The seed roadmap's
  recovery line MUST be `D1` to match the seed ledger, so a fresh program fails only
  `autonomy-recorded`, and it MUST sit under its own `## Recovery` heading after `## Autonomy`, so
  a dated recovery line cannot satisfy the autonomy check. A project without `program` MUST be unaffected.
  Verify: testscripts `program_checks`, `program_checks_off`, `program_autonomy`,
  `builtin_program`; unit `profiles` pointer test.
- **R-BLT-111** `ness-entry` MUST place the ceremony under its size: the `sizes` slot in Work
  sizes, a `Closing` section (the report with its Lessons heading, the operator-worded root close) and a
  `Quick root` section before `Full session`, the quick root sending one checkpoint at the end, and the specify, plan and handoff slots
  inside `Full session`; its promote command MUST carry `--size SIZE`. `program` MUST declare
  `sizes = ["program"]` and gate its four checks to that size.
  Verify: unit `TestEntryPlacesCeremonyUnderItsSize`, `TestEntryPromotionNamesTheSize`.
- **R-BLT-040** `program` text MUST NOT contain root specs/plans/frozen contracts, design detail
  at the root, pre-designed later waves, root implementation, pings for every step, or project
  specifics.
  Verify: review.
- **R-BLT-041** Map size MUST be guidance only; `program` MUST NOT add a size check (D25).
  Verify: testscript — a 100 KB roadmap.md triggers no failure in sync, doctor or gate.

## `go-strict` (D2, D20, §11)

- **R-BLT-050** `go-strict` MUST render the skills interface-design, test-strategy,
  test-discipline, documentation and compliance-check.
  Verify: testscript — five skill directories.
- **R-BLT-051** interface-design text MUST state: consumer-declared interfaces of ≤ 3 methods
  where faked; injected collaborators built only in `cmd/`; outcomes via ports; no `ForTest`/test
  hooks; `now` as a value, synctest for timers, no Clock interface; no package mutable state; a
  one-sentence package role.
  Verify: review.
- **R-BLT-052** test-strategy text MUST state: the implementer does not write the judge; contract
  first in consumer words; black-box tests; hand-written fakes; "which wrong implementation would
  this reject?".
  Verify: review.
- **R-BLT-053** test-discipline MUST be enforced by `.golangci.tests.yml` and audit: no
  `time.Sleep`, no permissive mocks or call counts, helpers marked, testdata fixtures, test files
  < 400 lines, `-race`, `-count=4`.
  Verify: testscript — fixture test file with `time.Sleep` fails the test lint; 400-line file
  fails the audit.
- **R-BLT-054** The `-count=4` policy MUST have a target that runs it (fix while porting).
  Verify: testscript — `make validate`'s test step includes `-count=4`.
- **R-BLT-055** documentation MUST be enforced by revive and godot: godoc on every export
  starting with its name; one package comment; README floor with "Architectural Role".
  Verify: testscript — export without doc comment fails lint.
- **R-BLT-056** documentation MUST ship with evals (fix while porting).
  Verify: review — eval files present and run.
- **R-BLT-057** compliance-check MUST run the existing Python audit script (D26) in the gate over
  the whole tree (v1; a diff scope is not shipped); MUSTs fixed or owned; exceptions with reasons
  in the exceptions file.
  Verify: testscript — fixture violation fails the gate; an exception entry with reason passes.
- **R-BLT-058** `go-strict` MUST declare `python3` (and every other tool its files run) in
  `requires_tools`.
  Verify: unit — profile.toml lists `python3`; testscript — missing python3 fails sync
  (R-SYNC-005).
- **R-BLT-059** `go-strict` MUST render a cleaned `.golangci.yml` (same linters as the source,
  without copied comments and dead settings), `.golangci.tests.yml`, a lefthook config with
  failure-only output, and a `validate` target invoked as `make validate`.
  Verify: unit — files present; `golangci-lint config verify` passes; `make -n validate` runs the
  gate steps; review for dead settings.
- **R-BLT-060** `go-strict` MUST expose slots: module path, package roots, composition roots,
  generated dirs, excluded packages, exceptions file, ceilings, validator/logger libraries, Go
  version (≥ 1.25).
  Verify: unit — each slot set in a fixture renders into the files; Go version < 1.25 refused.
- **R-BLT-061** Lifecycle wiring MUST embed: at specify → test-strategy, behavior-spec,
  interface-design; at plan → compliance-check and literal `invoke /<skill>` lines; at handoff →
  documentation.
  Verify: unit — grep of rendered lifecycle skills for the literal invokes.
- **R-BLT-062** The gate docs MUST state one scope (diff-scoped or whole-tree) consistently
  (fix while porting).
  Verify: review.
- **R-BLT-063** Each ported skill MUST have an eval comparing new vs old text, with the new score
  not below the recorded figure (interface-design 100%, test-strategy 96%, test-discipline 90%,
  compliance-check 100%); evals run locally on demand (skill-creator style), not in CI, within
  the D31 budget rule (R-QA-042).
  Verify: review — eval run recorded with scores and usage.

## `behavior-specs` (§11)

- **R-BLT-070** `behavior-specs` MUST declare `requires = ["go-strict"]`.
  Verify: unit.
- **R-BLT-071** `behavior-specs` text MUST require declarative godog scenarios with one `When`,
  ≤ 10 steps and ≤ 5 example rows, a shared step vocabulary and strict runners.
  Verify: review; unit — the rendered lint/audit (if any) rejects a fixture with two `When`.
- **R-BLT-072** `behavior-specs` MUST enable the `contracts` feature and freeze scenarios with
  it.
  Verify: testscript — enabling the profile renders the contracts skill text.
- **R-BLT-073** `behavior-specs` MUST ship an eval (recorded 91% vs 64%) not below the recorded
  figure.
  Verify: review.

## `git-conventional`

- **R-BLT-080** `git-conventional` MUST render guidance for Conventional Commits and for commit
  trailers.
  Verify: unit — rendered text names the `type(scope): subject` form and the trailer rules.
- **R-BLT-081** `git-conventional` MUST install a commit-msg hook that rejects a message not in
  Conventional Commits form.
  Verify: testscript — `git commit -m "stuff"` fails; `git commit -m "fix: x"` succeeds.

## `example-release` (D12, D27; renamed by P6)

- **R-BLT-090** `example-release` MUST show one complete chain of integrate → post-push
  probes → notify, written as profile text (ness does not chain them).
  Verify: review.
- **R-BLT-091** It MUST include release notes, a CI check after push as a probe, and dev → prod
  promotion via `ness integrate --from/--into`.
  Verify: unit — rendered probe file parses (R-PRB-001) and the text uses
  `integrate --from … --into …`.
- **R-BLT-092** It MUST contain no private information about the product it was modelled on (D27).
  Verify: unit — denylist scan (service names, hostnames, channel ids, people, repo paths from the
  source harness) over every rendered file; review.
- **R-BLT-093** Its probe files MUST pass `ness probe run` against a fixture floor.
  Verify: testscript.

## Out of scope for v1

- **R-BLT-100** The research lab's research line MUST NOT ship as a built-in in v1 (it stays a local
  profile until a second project needs it).
  Verify: unit — the embedded profile list is exactly the five built-ins.
