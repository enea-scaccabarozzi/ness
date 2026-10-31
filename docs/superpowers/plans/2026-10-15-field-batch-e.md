> [!NOTE]
> Archive: altered for the public release. Names of private projects, local paths and private links were replaced; the content is otherwise as written.

# Field-feedback Batch E Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Every rule that ness enforces only in prose gets a gating static check whose failure names the skill and section it enforces (E1), and the profile authoring docs tell a consuming project how to do the same (E2).

**Architecture:** One pointer type (`internal/rule`) that every refusal appends and one resolver that proves a pointer lands on a real heading, used by a Go meta-test (built-in refusals, profile-shipped files) and by `ness sync` (declared checks and guards of a consuming project). Profile checks are a declarative `[[checks]]` table in `profile.toml` executed by core at `ness open`, `ness integrate`, `ness close`; core knows kinds, never the `program` profile. Project guards are `[[guards]]` in `.ness/ness.toml`, run on pre-tool. Small built-ins last.

**Tech Stack:** Go (cobra CLI, `internal/shell` runner), SQLite store, testscript (`internal/cli/testdata/script/*.txtar`), unit tests with hand-written fakes (`package x_test`), profile text in `profiles/`.

**Spec:** scope is fixed in `docs/sessions/field-feedback/plan.md` ("## Batch E" and "### Batch E scope"); per-item detail in `docs/sessions/field-feedback/e1-ness-inventory.md` §2-§4. This plan does not reopen it. Normative requirements are added to `docs/spec/`: new `rules.md` (area RULE) and `checks.md` (area CHK), plus `profiles.md`, `core-config.md`, `hooks.md`, `sessions.md`, `store-messages.md`, `profiles-builtin.md`.

## Questions for root

Where a ruling is awkward as stated, the plan builds the alternative below. Answer only if you disagree.

- Q1. **E1.2 "scripts".** Program's four checks are regex/section/diff/store questions. Python would add a runtime `program` never required; `sh` would be brittle awk over markdown. Recommendation, built here: core ships four Go **kinds** (`section-matches`, `branch-forbids`, `match-last`, `checkpoint-since-landing`) parameterised from `profile.toml`, so `program` ships no script at all. A `run = "<command>"` form (sh -c via the existing `shell.Runner`, any language the project already requires) is also built, because E2 and the research lab (E3) need an escape hatch for rules no kind covers; it is confined to Tasks 7-8 and can be dropped without touching the rest.
- Q2. **E1.1 "every built-in refusal declares skill + section".** A static scan of the Go source for refusal sites that skip the registry is too noisy to gate (`refuse(` is used for usage errors too). Built: refusals are typed (`rule.Ref(rule.ID)`), a Go test proves each registered pointer resolves, and each existing refusal's testscript asserts `skill ness-` on stderr. A new refusal that forgets the registry is caught at review, not by a gate. Say if you want the source scanner anyway.
- Q3. **Section granularity.** Inventory pointers like "Communicate: Waiting rule" cannot resolve (the skill has no such heading). A pointer's section is the exact text of an existing heading (any level). Where a finer pointer is wanted, the same task adds the heading; the plan adds none by default (all pointers use existing `##` headings).
- Q4. **`ness contract check` in go-strict `validate` (E1.4).** The go-strict `Makefile` is a `seed`, so only new adopters get the line; existing adopters (ness itself, the infrastructure lab) must add it by hand. Task 14 adds it to the seed and stops there. Also unverified: that `ness contract check` is a silent no-op with the feature off (Task 14 measures it; if not, the line is dropped and reported).
- Q5. **Autonomy check strength.** As ruled, mode word plus date under `## Autonomy`, no transcript check. The seed instruction line already contains the four mode words, so the real discriminator is the date; the check therefore requires a date and a mode word in the section. Known ceiling: a root that writes a date without asking passes. Not hardened.
- Q6. **Seed defect found.** `profiles/builtin/program/roadmap.md` ends `Recovery line: last decision id D0` while the seed `decisions.md` already holds D1, so a fresh program would fail check 3.7 on its first `ness open`. Task 9 changes the seed to `D1`.
- Q7. **Order against Batch B.** Another agent is finishing Batch B in `internal/worker`, `internal/hook`, `internal/store`; `git status` shows uncommitted edits in `internal/hook/background.go`, `internal/cli/send.go`, `internal/store/{checkpoint,items}.go` and two core skills, which this plan also touches. Tasks 3, 4 and 12-14 (hook, `send.go`, the core skills) must not start before that work is committed (rebase first); Tasks 1, 2 and 5-11 do not overlap it.

## Global Constraints

- Work only in `~/src/ness/.worktrees/field-feedback-c5a82a215386` (branch `session/field-feedback-c5a82a215386`).
- Test rules (`.claude/skills/ness-test-strategy`, `ness-test-discipline`, `ness-interface-design`): the implementer never writes the judge, so each feature has a test task (separate agent) before its implementation task, and the implementer does not edit those tests; black-box (`package x_test`); hand-written fakes; no `time.Sleep`; no permissive mocks or call-count assertions; helpers call `t.Helper()`; fixtures in `testdata/`; test files under 400 lines; every test carries `// covers: R-…`; no `ForTest` identifiers; no package-level mutable state; interfaces declared by the consumer, at most three methods; one package comment per package; godoc on every export.
- A test task writes the spec text first, then failing tests; it may add only signature-only stubs returning zero values. Its commit is red by design; the implementation commit makes it green.
- Any task touching `profiles/**` ends with `go run ./cmd/ness sync` and commits the re-rendered `.agents/skills/**` and `.ness/lock.json` (the gate's `selfcheck` is `ness sync --check`).
- Per-task checks: `go test ./<pkg>/... -count=1`; before each implementation commit `golangci-lint run --allow-serial-runners ./... && golangci-lint run --allow-serial-runners -c .golangci.tests.yml ./...` and `python3 .ness/audit.py`. The full gate runs once, in `ness integrate`.
- Commits: Conventional Commits, each ending with
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>` and `Claude-Session: <session link removed>`.
  Never pipe a gate or a commit through `tail`/`head`.

## Design

### D1. Pointer and registry (E1.0, E1.1)

New leaf package `internal/rule` (imports nothing of ness):

- `type Pointer struct{ Skill, Section string }`; `func (p Pointer) String() string` returns `(skill ness-entry, section Communicate)`. This one shape is the only pointer format anywhere (Go messages, profile-shipped scripts, config), so one regexp finds them all: `\(skill ([a-z0-9-]+), section ([^)]+)\)`.
- `type ID string` constants (`rule.HookPolling`, `rule.HookBackground`, `rule.HookOutput`, `rule.ChildCap`, `rule.Checkpoint`, `rule.Subject`, `rule.DirtyWorktree`, `rule.RootClose`, `rule.RootOpen`, `rule.GateBusy`, `rule.ReportMissing`, `rule.ContractGuard`, …) and `func Table() []Entry` (a fresh slice, no package-level state) mapping each to a Pointer. `func Ref(id ID) string` returns `" " + pointer.String()` for appending to a message; an unknown id panics at first use, which the meta-test reaches.
- `func Headings(markdown []byte) []string` and `func Resolve(skills map[string][]string, p Pointer) error`: error text names the missing skill or the missing section and lists the section headings that exist.

Coverage, both halves with the same `Resolve`:

1. **Built-in refusals and profile-shipped files (Go test, no rendering needed).** `profiles/pointers_test.go` walks the embedded core, built-in and feature profiles, builds `skill -> headings` from every `skills/*/SKILL.md`, then (a) resolves every `rule.Table()` entry, (b) regex-scans every embedded profile file (audit.py, golangci forbidigo `msg:`, Makefile, commit-msg, scripts, skill text) for pointers and resolves each, (c) parses every built-in `profile.toml` and resolves each declared `[[checks]]` pointer.
2. **A consuming project's checks and guards (runtime).** `ness sync` (so `ness sync --check` in the gate) resolves every `[[checks]]` of every applied profile, local included, and every `[[guards]]` of `.ness/ness.toml` against the skills it renders (entries `.agents/skills/<skill>/SKILL.md`; falling back to the file on disk for a skill the project owns). An unresolved pointer refuses sync, naming the check or guard. This is the meta-test pattern E2 documents.

### D2. Profile-declared checks (E1.2)

Declared in `profile.toml` as an array of tables (parsed by `internal/profile`, resolved into `Resolved.Checks` in profile order, local last, id prefixed `<profile>/`):

```toml
[[checks]]
id      = "autonomy-recorded"            # unique in the profile
on      = "open"                         # open | integrate | close
for     = "child"                        # optional: child | root; default both
skill   = "ness-program"                 # required, with section
section = "Autonomy"
message = "docs/program/roadmap.md records no autonomy mode. Ask the operator the autonomy question, then record the mode, their words and the date under \"## Autonomy\"."
kind    = "section-matches"              # exactly one of kind / run
file    = "docs/program/roadmap.md"      # kind arguments are flat keys
heading = "Autonomy"
require = ['(?i)\b(gate|bands|to checkpoint|custom)\b', '\d{4}-\d{2}-\d{2}']
```

Kinds (core knows these four; none mentions a profile or a program):

| kind | arguments | passes when | used by |
|---|---|---|---|
| `section-matches` | `file`, `heading`, `require` (regexps) | the body of that heading (to the next heading of equal or higher level) matches every `require` | autonomy recorded (3.1) |
| `branch-forbids` | `path` (regexp, `{record}` expands to the session's record dir), `pattern` | no line added by `git diff <into>...<branch>` in files matching `path` matches `pattern` (the file itself may exist) | no decision ids in a child's record (3.6, FF-D6) |
| `match-last` | `file`, `pattern`, `against_file`, `against_pattern` (group 1 of each) | group 1 of the first match of `pattern` in `file` equals group 1 of the **last** match of `against_pattern` in `against_file` | recovery line = ledger (3.7) |
| `checkpoint-since-landing` | none | the root has no landed child, or has sent the operator a checkpoint newer than its latest landing | checkpoint before next wave (3.5) |

`run = "<command>"` instead of `kind`: executed by `shell.Runner` (`sh -c`, working directory the worktree, timeout `timeout` default 30s, combined output). A script is shipped with the profile's `[files]` (`own`), exactly as go-strict ships `.ness/audit.py`, and invoked through its interpreter (`run = "sh .ness/checks/x.sh"`), because owned files are not marked executable.

Context handed to every check (Go struct for kinds, environment for `run`): `NESS_EVENT` (open/integrate/close), `NESS_SESSION` (the acting session; empty for the operator), `NESS_ROOT` (the top ancestor of the acting session), `NESS_WORKTREE` (absolute; the directory examined and the cwd: the opener's checkout at open, the integrating session's at integrate, the closed session's at close), `NESS_PRIMARY`, and per event `NESS_SCOPE` (open), `NESS_BRANCH`/`NESS_INTO`/`NESS_RECORD` (integrate, close). The new session does not exist yet at open, so there is no record dir there.

Result format and exit codes. Script exit 0 passes; any other exit, a timeout, or a command that cannot start fails (fail closed). All checks of the event run in order and all failures are reported together. The refusal is the existing refusal exit (`refuse()`, exit 4), nothing is written, and the text is built by core:

```
ness open refused: <message> (skill ness-program, section Autonomy)
  program/autonomy-recorded: <detail>
```

`<message>` is the declared `message` (required for a `kind`, optional for `run` where the script's output, trimmed and capped at 1500 bytes, is the detail line); the pointer is appended by core and a script never prints its own. `ness open --resume` does not rerun checks (the child already passed them).

Interfaces (consumer-declared in `internal/check`, wired in the CLI composition root): `Git interface{ Out(dir string, args ...string) (string, error) }` (satisfied by `session.Git`), `Facts interface{ LastLanding(root string) (time.Time, error); LastCheckpoint(root string) (time.Time, error) }` (satisfied by `*store.Store`), and the existing `shell.Runner` func type. `check.Run(ctx Context, checks []profile.Check, deps Deps) []Failure`; no globals.

### D3. Project guards (E1.3)

`[[guards]]` in `.ness/ness.toml`: `tool` (regexp on the tool name, e.g. `Bash`, `Edit|Write|MultiEdit`), `match` (regexp on the Bash command, or on the edited file path for edit tools), optional `content` (regexp on the new text of an edit: `new_string`, `content`, `edits[].new_string`), `message`, `skill`, `section`. `hook.ProjectGuards(p Payload, guards []Guard) string` runs after `hook.PreTool` (whose signature stays); `cli.preTool` builds the list from two defaults plus the config entries. Defaults: (a) a Bash command that runs the configured `gate` (anchored at command start or after `&&`/`;`, `\b`-terminated; no guard when `gate` is empty) from a bound session; (b) an edit of `.ness/ness.toml` whose new text mentions `max_children` or a `[launch`/`[tiers`/`[worker` table. Config parse refuses a guard with an invalid regexp or without skill and section, naming the entry; sync resolves the pointer (D1).

## File map

| Task | Create | Modify |
|---|---|---|
| 1-2 | `docs/spec/rules.md`, `internal/rule/{rule.go,rule_test.go}`, `profiles/pointers_test.go` | `docs/spec/README.md`, `profiles/core/skills/ness-entry/SKILL.md`, `profiles/core/skills/ness-launch/SKILL.md`, `profiles/builtin/go-strict/{audit.py,golangci.yml,golangci.tests.yml,Makefile}`, `profiles/builtin/git-conventional/commit-msg` |
| 3-4 | | `internal/hook/{guards,background}.go`, `internal/cli/{open,send,integrate,close}.go`, `internal/session/{integrate,close,contract}.go`, existing `internal/cli/testdata/script/*.txtar` (stderr assertions) |
| 5-8 | `docs/spec/checks.md`, `internal/check/*`, `internal/store/landing.go` | `internal/profile/{profile,resolve}.go`, `internal/cli/{sync,open,integrate,close}.go`, `internal/sync/render.go`, `docs/spec/{profiles,store-messages}.md` |
| 10-11 | | `internal/config/config.go`, `internal/hook/guards.go` (new file `internal/hook/project.go`), `internal/cli/hook.go`, `docs/spec/{core-config,hooks}.md` |
| 9 | | `profiles/builtin/program/{profile.toml,roadmap.md}`, `docs/spec/profiles-builtin.md` |
| 12-14 | | hook, `internal/cli/send.go`, `internal/session/close.go`, `profiles/core/skills/ness-entry/SKILL.md`, `profiles/builtin/go-strict/Makefile` |
| 15 | `docs/profiles/authoring.md` | `README.md`, `profiles/builtin/README.md` |

---

## E1.0 and E1.1: pointers on every refusal, and the registry

### Task 1: pointer registry acceptance tests and spec

**Files:** Create `docs/spec/rules.md`, `internal/rule/rule.go` (signature-only stubs), `internal/rule/rule_test.go`, `profiles/pointers_test.go`. Modify `docs/spec/README.md` (index row RULE), the existing testscripts listed in step 3.

**Interfaces:** Produces the contract of Tasks 2-4.

- [ ] **Step 1: Spec** in `docs/spec/rules.md`:
  - **R-RULE-001** A pointer MUST read `(skill NAME, section HEADING)` and be the only pointer format. Verify: unit.
  - **R-RULE-002** Every refusal a built-in check raises (list: polling, background, output, child cap, checkpoint, handoff, subject, dirty worktree, root close, root open, gate busy, report missing, contract guard, and the ones added by this batch) MUST end with the pointer of its registry entry. Verify: testscript per refusal, `stderr 'skill ness-[a-z-]+, section'`.
  - **R-RULE-003** Every registry pointer, and every pointer in a file a profile ships (scripts, `audit.py`, linter messages, Makefile, hooks), MUST name a skill that ships and a heading that exists in it. Verify: unit (`profiles/pointers_test.go`), including a negative table (missing skill, missing section, heading in a code fence not counted).
  - **R-RULE-004** The child-cap refusal MUST NOT suggest raising `launch.max_children`. Verify: testscript `open_child_cap`.
  - **R-RULE-005** Skill text MUST state every rule a hook refuses: no `&` backgrounding, no `pgrep -f` waits, no whole tool-result reads (ness-entry, Communicate). Verify: unit — the rendered skill contains `run_in_background` and `tool result`.
- [ ] **Step 2: Tests.** `rule_test.go`: `Ref` format; `Table()` ids unique, each Pointer non-empty, and contains the ids of R-RULE-002's list (the list is written in the test, not read from the code); `Resolve` accepts and refuses per R-RULE-003 using inline markdown (a `## X` inside a fenced block is not a heading). `pointers_test.go`: the walk of D1.1 (a/b/c) over the embedded profiles, with a fixture skill tree under `profiles/testdata/` for the negative cases.
- [ ] **Step 3: Testscripts** (add assertions only): `hook_pretool.txtar`, `hook_pretool_background.txtar`, `open_child_cap.txtar` (also `! stderr 'raise launch'`), `open_refusals.txtar`, `send_checkpoint.txtar`, `integrate_squash.txtar`, `close*.txtar` (report missing, root close), `contract*.txtar`: `stderr 'skill ness-[a-z-]+, section [A-Z]'`.
- [ ] **Step 4: Run, expect red** (`go test ./internal/rule/ ./profiles/ ./internal/cli -count=1`).
- [ ] **Step 5: Commit** `test(rule): acceptance tests for refusal pointers and the registry` (red by design).

### Task 2: rule package, profile-shipped pointers, missing prose

**Files:** Create `internal/rule/rule.go`. Modify `profiles/core/skills/ness-entry/SKILL.md` (Communicate: add the two missing lines: a lone `&` and `pgrep -f` waits go through `run_in_background`; read a saved tool result with offset/limit or grep), `profiles/core/skills/ness-launch/SKILL.md` (child cap: wait, never raise), `profiles/builtin/go-strict/audit.py` (a `POINTERS` table RULE -> (skill, section): T400, SLEEP -> ness-test-discipline/Rules; PKGDOC -> ness-documentation/Comments; FORTEST, CLOCK -> ness-interface-design/Time and test hooks; LIBS -> ness-interface-design/Wiring; FEATURE -> ness-behavior-specs/Scenarios; each printed finding ends with the pointer; headings verified against the real skills, fix the table to existing headings), `golangci.yml` and `golangci.tests.yml` forbidigo `msg:`, the `Makefile` seed (`|| { echo '… (skill …)'; exit 1; }` after the lint targets), `git-conventional/commit-msg`.

- [ ] **Step 1:** implement `Pointer`, `ID` consts, `Table`, `Ref`, `Headings`, `Resolve` (D1). Table headings come from the inventory §2 table, corrected to existing headings (ness-entry: Communicate, Tests, Session record, Lifecycle; ness-launch: Open a child, Child or subagent, Wait, then fold; ness-contracts and ness-git-conventional per their files).
- [ ] **Step 2:** prose and profile-file edits above, then `go run ./cmd/ness sync`.
- [ ] **Step 3:** `go test ./internal/rule/ ./profiles/ -count=1` green; lint; audit.
- [ ] **Step 4: Commit** `feat(rule): pointer registry and pointers in profile-shipped checks`.

### Task 3: wire pointers into the hook refusals

**Files:** Modify `internal/hook/guards.go`, `internal/hook/background.go` (after Batch B lands).

- [ ] Append `rule.Ref(...)` to the polling, background and output guard reasons (`waitingRule` stays a clause of the polling message). The hook testscripts of Task 1 go green; unit tables keep matching on `run_in_background`.
- [ ] Commit `feat(hook): refusals name the skill and section they enforce`.

### Task 4: wire pointers into the CLI and session refusals

**Files:** Modify `internal/cli/{open,send,integrate,close}.go`, `internal/session/{integrate,close,contract}.go`.

- [ ] Child cap (`open.go checkPlace`): drop "or raise launch.max_children" (R-RULE-004), pointer ChildCap. Append pointers to: checkpoint headings and root-only (`send.go`), subject and dirty worktree (`integrate.go`), root close (`close.go`), root open (`open.go manager`), gate busy and report missing and contract guard (`session/*`). Messages keep their first clause so existing assertions still match.
- [ ] Commit `feat(cli): every built-in refusal names the skill and section it enforces`.

---

## E1.2: profile-declared checks

### Task 5: checks format and sync resolution, tests and spec

**Files:** Create `docs/spec/checks.md` (events, kinds, context, output, exit; R-CHK-001..), modify `docs/spec/profiles.md` (R-PROF-045..), `internal/profile/profile_test.go`, `internal/profile/resolve_test.go`, `internal/cli/testdata/script/sync_check_pointer.txtar`.

- [ ] **Step 1: Spec.** R-PROF-045 `[[checks]]` fields: `id`, `on`, `skill`, `section`, `message` required (message optional for `run`), exactly one of `kind` or `run`; unknown field, unknown kind, unknown or missing kind argument, bad regexp, duplicate id, `file`/`path` that is absolute or contains `..`: refused naming file, line and field (R-PROF-005 style). R-PROF-046 sync MUST refuse a check or `[[guards]]` entry whose pointer does not resolve in the rendered skills, naming it and listing the headings that exist (D1.2). R-PROF-047 `Resolved.Checks` keeps profile order, local last, ids `<profile>/<id>`. Fuzz: R-PROF-005's corpus gains a `[[checks]]` seed.
- [ ] **Step 2: Tests.** Profile parse table (good, one case per refusal), resolve order, and a testscript: local profile with a check pointing at `ness-entry`/`No such section` → `ness sync` exits non-zero naming `local/<id>`, tree unchanged; fixed pointer → exit 0; `ness sync --check` also refuses.
- [ ] **Step 3: Run red; commit** `test(profile): acceptance tests for profile-declared checks`.

### Task 6: checks format and sync resolution, implementation

**Files:** Modify `internal/profile/{profile,resolve}.go` (`Check` type, `manifest.Checks`, validation, `Resolved.Checks`), `internal/sync/render.go` or `internal/cli/sync.go planSync` (resolve D1.2 with `rule.Resolve` over the rendered entries).

- [ ] Implement; tests of Task 5 green; lint; audit; commit `feat(profile): [[checks]] table and pointer resolution at sync`.

### Task 7: check runner, kinds and wiring, tests and spec

**Files:** Create `internal/check/{check_test.go,kinds_test.go,run_test.go}`, fixtures `internal/check/testdata/`, `internal/store/landing_test.go`, testscripts `check_open.txtar`, `check_integrate.txtar`, `check_close.txtar`.

- [ ] **Spec** R-CHK-001..: events and where they run (`open`: in `planLaunch` after the manager and `--here` checks, before any write; not on `--resume`; `integrate`: before the throwaway and the gate lock; `close`: in the close preconditions, before any state change), `for` filter, context and `NESS_*` variables, exit semantics, all-failures-reported, message format of D2, output cap, timeout fails closed, a check of another event never runs, the operator (no session) skips `for = "child"` checks. R-MSG-0xx store facts: `LastLanding(root)` is the latest `integrated_at` of any descendant of root, zero if none; `LastCheckpoint(root)` the latest checkpoint item from root to the operator, zero if none.
- [ ] **Unit tests** (fake `Git`, fake `Facts`, fake `shell.Runner`, fixture trees): each kind passes and fails (section body bounded by the next heading of equal or higher level; a heading in a code fence is not a heading; `match-last` with equal, unequal, no match in either file; `branch-forbids` added line vs a removed line vs a different path; `checkpoint-since-landing` with no landing, landing before and after the checkpoint, equal timestamps pass); `run` exit 0, exit 3, timeout, cannot start, env contents asserted from the runner's `Cmd`, output capped; failures aggregate in order.
- [ ] **Store test** for the two facts over a real store (descendants, not only children; closed children count).
- [ ] **Testscripts** with a throwaway local profile: `run` check refuses `ness open` with message, detail and pointer on stderr and exit 4, and nothing is written (no session row, no worktree); passing check opens; integrate and close likewise; `--resume` does not rerun.
- [ ] Run red; commit `test(check): acceptance tests for profile checks at open, integrate and close`.

### Task 8: check runner, kinds and wiring, implementation

**Files:** Create `internal/check/{check.go,kinds.go,run.go}`, `internal/store/landing.go`. Modify `internal/cli/{open,integrate,close}.go` (composition: resolve checks with `resolvedConfig`, build `check.Deps{Git, Facts, Shell}`), `internal/cli/hook.go` untouched.

- [ ] Implement per D2. `planLaunch` gains the check call and nothing else changes; the call sites pass the resolved `Checks` from the config they already load.
- [ ] Green; lint; audit; commit `feat(check): run profile-declared checks at open, integrate and close`.

### Task 9: program ships its four checks

(Depends on Task 8.)

**Files:** Modify `profiles/builtin/program/profile.toml` (four `[[checks]]`), `profiles/builtin/program/roadmap.md` (recovery line `last decision id D1`), `docs/spec/profiles-builtin.md` (R-BLT-: program refuses `ness open` of a child with no recorded autonomy, with no checkpoint since the last landing, with a recovery line differing from the ledger; refuses integrate of a child whose branch adds decision ids to its record).

- [ ] **Tests first (separate agent):** `internal/cli/testdata/script/program_checks.txtar`: init with `--profile program`; (1) child `ness open` with the seed roadmap → exit 4, stderr contains `ness-program`, `section Autonomy` and `## Autonomy`; fill mode and date → opens; a project without `program` is unaffected. (2) close a landed child, `ness open` → refused (`section Checkpoints`); `ness send --intent checkpoint` → opens. (3) recovery line `D12` vs ledger last `D15` → refused (`section Rules`); equal → opens; the fresh seed passes this check. (4) child branch adds `<record>/decisions.md` with id entries (`- **D1**`, `C2-D7`, `L0a-D3`) → `ness integrate` refused (`section Rules`); the same file as plain dated notes without ids, or a report only → integrates (FF-D6: a child's decisions.md is allowed, ids are the root's). Plus `profiles/pointers_test.go` already resolves the four pointers.
- [ ] **Declarations** (all with `for = "child"`): 
  - `autonomy-recorded`: `on = "open"`, `section-matches`, `docs/program/roadmap.md`, heading `Autonomy`, require `(?i)\b(gate|bands|to checkpoint|custom)\b` and `\d{4}-\d{2}-\d{2}`; pointer `ness-program`/`Autonomy`.
  - `checkpoint-before-wave`: `on = "open"`, `checkpoint-since-landing`; `ness-program`/`Checkpoints`.
  - `recovery-line`: `on = "open"`, `match-last`, file `docs/program/roadmap.md` pattern `last decision id (D\d+)`, against `docs/program/decisions.md` pattern `(?m)^- \*\*(D\d+)\*\*`; `ness-program`/`Rules`.
  - `no-child-decision-ids`: `on = "integrate"`, `branch-forbids`, path `{record}/decisions\.md$`, pattern `(?m)^\s*[-*]\s*\*\*D\d+|\b[A-Z0-9]+-D\d+\b`; `ness-program`/`Rules`.
  Messages carry the instruction from inventory 3.1, 3.5, 3.7, 3.6 respectively.
- [ ] Sync, green, commit `feat(profiles): program ships its checks`.

---

## E1.3: project guards

### Task 10: `[[guards]]` acceptance tests and spec

**Files:** Modify `docs/spec/core-config.md` (R-CFG-: `[[guards]]` keys, regexp and pointer validation), `docs/spec/hooks.md` (R-HOOK-022 project guards run after the built-in ones for a bound session; R-HOOK-023 default gate guard; R-HOOK-024 default launch-limits guard). Create `internal/hook/project_test.go` (+ stub `project.go`), `internal/config/guards_test.go`, testscript `hook_pretool_guards.txtar`.

- [ ] Unit tables: gate guard blocked (`make validate`, `cd x && make validate`, `make validate; echo`), allowed (`make validate-docs`, `go test ./internal/hook`, empty `gate`); edit guard blocked (Edit whose `new_string` sets `max_children = 8`; Write whose `content` contains `[launch]`; MultiEdit second edit), allowed (Edit of another key, Edit of another file naming `max_children`, Edit whose only mention of `[launch` is in `old_string`); configured guard (Bash `pytest` over the whole suite) with its message and pointer; invalid regexp and missing skill/section refused at config parse naming the entry. Testscript: exit 2, message and pointer on stderr; unbound session silent.
- [ ] Run red; commit `test(hook): acceptance tests for project guards`.

### Task 11: `[[guards]]` implementation

**Files:** Modify `internal/config/config.go` (`Guards []Guard`, validation), create `internal/hook/project.go` (`Guard`, `ProjectGuards`, `DefaultGuards(gate string) []Guard`), modify `internal/cli/hook.go preTool` (call after `hook.PreTool`, defaults then config), and the sync pointer resolution of Task 6 (already covers guards; add the call).

- [ ] Implement per D3; reuse `inputText` and `edits.go`'s tool list; messages from inventory 3.2 with pointers (ness-entry/Tests, ness-launch/Child or subagent).
- [ ] Green; lint; audit; commit `feat(hook): project guards on pre-tool with a default gate and limits guard`.

---

## E1.4: small built-ins

### Task 12: small built-ins, acceptance tests and spec

**Files:** Modify `docs/spec/hooks.md`, `docs/spec/store-messages.md`, `docs/spec/sessions.md`, `docs/spec/profiles-builtin.md`; tests in `internal/hook/guards_test.go`-adjacent new file `internal/hook/waiting_test.go`, `internal/cli/testdata/script/{hook_pretool_explore,send_handoff,close_lessons}.txtar`, `internal/session/close_test.go` additions.

- [ ] R-HOOK-025: a Bash loop with `sleep` around `ness integrate|send|open` is refused (blocked: `for i in $(seq 40); do ness integrate && break; sleep 60; done`; allowed: plain `ness integrate`, `for b in a b; do ness integrate --from $b; done`), pointer ness-entry/Communicate. R-HOOK-026: an `Agent` (and legacy `Task`) call with `subagent_type` `Explore` is refused, message names `ness-sweep`, pointer ness-entry/Communicate; other types allowed (the exact tool name and field are verified against a real payload in Task 14 and the fixture is recorded). R-MSG-070: `--intent handoff` needs `## Done`, `## Now`, `## Next` like a checkpoint (reuse `CheckpointMissing`), refused with pointer ness-entry/Communicate. R-SESS-: `ness close` refuses a report without a `## Lessons` heading (worktree present, and the worktree-gone path reading `report.md` from the branch), pointer ness-entry/Lessons and briefs; a one-line "None." passes. R-BLT-: the core skill's Session record states that decision ids are the root's and a child's `decisions.md` is plain dated notes without ids, never cited upward (FF-D6; reconciles with the program rule on children and ledgers).
- [ ] Run red; commit `test(hook): acceptance tests for the waiting, Explore, handoff and Lessons checks`.

### Task 13: hook and message built-ins

**Files:** Modify `internal/hook/guards.go` (second rule in `polling()`; new `agent` guard), `internal/cli/send.go` (handoff body check).

- [ ] Implement, green, lint, audit; commit `feat(hook): refuse sleep loops around ness commands and the Explore subagent; handoff needs headings`.

### Task 14: Lessons at close, record prose, contract check

**Files:** Modify `internal/session/close.go checkReport`, `profiles/core/skills/ness-entry/SKILL.md` (Session record: decision ids are the root's, a child's `decisions.md` is plain dated notes; Lessons wording), `profiles/builtin/go-strict/Makefile` (Q4).

- [ ] Implement the Lessons heading check; edit prose; sync.
- [ ] Verify the Explore payload shape (Task 12) with a recorded real payload and adjust the fixture if it differs.
- [ ] Q4: run `ness contract check` with the feature off; if silent exit 0, add it to the seed's `validate`; otherwise leave the Makefile and say so in the report.
- [ ] Commit `feat(session): report needs a Lessons section; decision ids belong to the root`.

---

## E2: docs

### Task 15: "Rules come with checks" section

**Files:** Create `docs/profiles/authoring.md`; modify `README.md` and `profiles/builtin/README.md` (one link each).

- [ ] Content: a local rule is three things: a skill section (the prose), a check, and a failure message that ends with `(skill X, section Y)`. The mechanisms, each with a ten-line example: the `gate` command (any script; message is an `echo` with the pointer; runs once at integrate), a local profile `[[checks]]` at open/integrate/close (kinds and `run`, context variables, exit codes), `[[guards]]` for pre-tool, contracts (`ness contract freeze`), git hooks (per clone, bypassable), probes (questions about the running world, not conduct). The meta-test pattern: `ness sync --check` in the gate resolves every declared pointer; for a project's own scripts, add the `(skill X, section Y)` form and one test that scans for it. What cannot be checked statically (judgement) is marked in the prose, not in the check. Examples use fake names only (the leak scan in `profiles_test.go` stays green because docs are outside `profiles/`, but keep the same discipline).
- [ ] Commit `docs(profiles): rules come with checks`.

### Task 16: finish

- [ ] `go run ./cmd/ness sync` (clean), `ness sync --check`, lint, audit, `go test ./... -count=1`. Add the "not built" list to the session record's report with reasons from the scope: chat-only ask detector (B5 Your move alerts deterministically; revisit if the field shows root asks only in chat), Stop hook, audit plug-in rules, `ness check`, golangci additions to go-strict, doctrine and "cites P/G" checks, root-does-too-much; and the Q2 and Q5 limits. Do not run the full gate next to other sessions.

## Task order and dependencies

1 -> 2 -> (3, 4 after Batch B lands); 5 -> 6 -> 7 -> 8 -> 9; 10 -> 11 (needs the sync pointer call from Task 6); 12 -> 13 -> 14 (after Batch B); 15 after 8 and 11; 16 last. Test tasks (1, 5, 7, 10, 12, and the test step of 9) are done by an agent that does not do the matching implementation.

## Review Focus

1. A pointer that resolves today but breaks when someone renames a heading: the meta-test must fail then (R-RULE-003 negative table).
2. A check that fails open: a `run` that cannot start, times out, or a kind whose file is missing must refuse, never pass.
3. `ness open` side effects: when a check refuses, no session row, worktree or branch exists (Task 7 testscript asserts).
4. Core knows no profile: grep `internal/**` for `program`/`ness-program` outside tests must stay empty.
5. Guard false positives: the default gate guard must not catch `make validate-docs`; the limits guard must not catch an edit that only quotes `[launch` in `old_string`.
6. Seed freshness: a brand-new `program` project passes `recovery-line` and fails only `autonomy-recorded`.

## Root answers (2026-10-15)
Q1 built-in kinds for program, keep `run` (the research lab and the E2 docs need it). Q2 no scanner. Q3 accepted. Q4 accepted. Q5 fix the seed. Q6 tasks 3, 4, 12-14 start after Batch B lands. Q7 accepted; transcript check of the operator's words deferred.
