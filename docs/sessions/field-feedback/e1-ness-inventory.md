> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# E1: ness prose-to-check inventory

Principle (operator, 2026-10-15): every rule that would otherwise be enforced only in prose gets a gating static check, and the check fails with a pointer to the prose (skill and section) it enforces. Read-only inventory, 2026-10-15; line numbers are of this worktree (the `ness-entry` copy is being edited by another agent, so its numbers may move by a few lines). Batch C rules are taken from `batch-c-draft.md` (not yet applied) and marked `C`.

Legend. **Check**: where it is enforced today, or `none`. **Ptr**: does the failure text point to the prose (skill and section)? `Y` yes, `part` names the command or states the rule but not the skill/section, `N` no. **F**: broken in the research lab field run. **Class**: `C` checked now; `S` statically checkable; `P` partly (cheap proxy); `J` judgement only. FP = false-positive risk.

Paths: `PE` = `profiles/core/skills/ness-entry/SKILL.md`, `PL` = `profiles/core/skills/ness-launch/SKILL.md`, `PP` = `profiles/builtin/program`, `GS` = `profiles/builtin/go-strict`.

## 1. Inventory

### Core: ness-entry and ness-launch

| Id | Source | Rule | Check today (file:line) and its message | Ptr | F | Class |
|---|---|---|---|---|---|---|
| E1 | PE:12-16 | Pick the work size (answer, quick, full) | none | - | | J |
| E2 | PE:18-24 | A quick or full task in an unbound conversation is promoted (or proposed) before work | nudge only: `unboundNote`, `internal/cli/hook.go:19-25`, printed at session-start; pre-tool is silent for the operator (R-HOOK-002) | part | | J (a gate would block the operator's own answers) |
| E3 | PE:25-29 | After promoting, work only in the worktree, never in the starting checkout | none | - | | S |
| E4 | PE:35-39 | Full session keeps brief, specs, plan, decisions (append-only, stable ids), report | only the report: `internal/session/close.go:86,93` "no report at X; write it, then close" | N | | P |
| E5 | PE:42-45 | `ness send`/`reply` are the only way to reach others; `--as returned`/`withdrawn` | routing: `internal/cli/send.go:139-162` ("the operator has no manager; name a session id with --to", "session X is closed"); withdraw rights: `reply.go:84-90` | N | | C for routing; J for "only way" |
| E6 | PE:46-48, PL:45 | Waiting rule: pull inbox once, end the turn; no Monitor/cron/ScheduleWakeup/sleep loops over `ness inbox`/`status` | `internal/hook/guards.go:92-98` (R-HOOK-011) "polling with Monitor on ness inbox/status is refused: pull once with ness inbox, then end your turn; the worker wakes you when something arrives" | part (states the rule, no skill/section) | F: sleep loops around `ness integrate` (wave1:80, c1) and `until pgrep` waits (c1-c3:90) escaped; the regex `pollRead` only knows inbox/status (`guards.go:19`) | C, gap |
| E7 | none in any skill | No shell backgrounding with `&`, no `pgrep -f` wait loops; use `run_in_background` | `internal/hook/background.go:19-27` (R-HOOK-020/021) "backgrounding with & is refused: nothing wakes you when the job ends; run it with the Bash tool's run_in_background option and end your turn; the task notification wakes you" | part, and **no prose exists** to point at (rule lives only in `docs/spec/hooks.md`) | F: 13.6 h dead (L0a 4.8 h, C1 8.7 h) | C |
| E8 | none in any skill | Do not read a whole saved tool result | `guards.go:100-115` (R-HOOK-013) "reading a whole saved tool result is refused: pass offset and limit, or grep it" | part, no prose exists | | C |
| E9 | PE:49-50 | Operator messages decision-first and short; long content on a page | none | - | partly (cause 4: 80-char pushes) | P (body length over N without `--page` on request/decision to operator) |
| E10 | PE:51-54 | Root reports progress by `checkpoint` with `## Done`, `## Now`, `## Next` (`## Ask`); children's handoffs use the same headings | checkpoint only: `send.go:90-103`, `checkpointBody` "a checkpoint needs the headings: ## Done, ## Now, ## Next" and "only a root sends a checkpoint, and only to the operator" (`send.go:96`). Handoff headings: none | N | F (cause 5, before the command existed) | C for checkpoint; S for handoff headings |
| E11 | PE:55-57 | Subagent returns a path plus at most about 2k tokens (8,000 chars) | none; the "8,000 cap" the field sessions cite is prose only (no `8000` in `internal/`) | - | handoffs 7-7.5k (wave1:106) | J (the hook never sees a subagent's reply) |
| E12 | PE:56-57 | Search with `ness-sweep`, never Explore | none. `ness-sweep` is installed by sync (`internal/sync/render.go:255`); the hook sees tool name and `subagent_type` | - | not seen | S |
| E13 | PE:59-66 | Ask the operator with `ness send --to operator` (a root's every ask), not only in chat; record chat answers with `ness reply` | alerts for open items only (worker, B5); chat-only asks undetectable; no Stop event wired (`render.go:41`: SessionStart, PreToolUse only) | - | **F: cause 4, asked in chat three times, 9 h** | P |
| E14 | PE:69-78, PL:11-13 | A brief states its autonomy level; a child's level never below the operator's words | none: `open.go:239` only requires a brief file. Note PE:75 says a brief with no level uses the project default, so "must state" cannot be a hard refusal | - | F (cause 1) | P |
| E15 | PE:80-82 | Never run the full gate by hand; it runs once inside `ness integrate` under the gate lock | lock only: `internal/session/integrate.go:277` "another integrate is running: integrate into X already running" | N | **F: F2 121 s anti-drift run, F3 `pytest -x` killed, re-lints (wave1:94-96, 129)** | P (literal `cfg.Gate` S; pytest/ruff/mypy loops project-specific) |
| E16 | PE:84-86 | Model and effort never change mid-session | by construction (no command); `inbox.go:58` boundary | - | | J |
| E17 | PE:89-90 | Lessons section in the report at handoff; read the manager's lessons at entry | report existence only (E4) | N | | S (heading present); read-at-entry J |
| E18 | PE:91 | A brief does not promote side remarks to rules | none | - | | J |
| E19 | PE:92-93 | A root delegates anything over a few edits and keeps the map | none | - | **F: root authored D33-D44 conventions itself (c1-c3:72-73)** | P (warn only; "few" is undefined) |
| E20 | PE:95-101 | Lifecycle slots: at specify/plan/handoff invoke the profile's skills | none (go-strict adds `/ness-test-strategy` etc. as prose lines `GS/lifecycle/*.md`); the gate runs the audit, a different thing | - | | J |
| E21 | PE:102-104 | A root closes itself only with the operator's own words | `close.go:68-76,225-250` (`authorized`, transcript check): "%q is not in a message the operator typed to this session; ask them, then close with their words" | part | | C |
| E22 | PE:106-118 | Stop and ask before irreversible actions; reserved decisions (money, accounts, publishing, policy, scope) | none | - | | J |
| E23 | PE:120-122 | After compaction re-read brief and inbox (plus `post-compact` slot files) | printed, not checked: `hook.go:177-188` "After compaction: re-read your brief and your inbox first." | n/a | F (cause 8, root lost the rules after 7 compactions) | J |
| L1 | PL:10-11 | Only the operator opens a root | `open.go:201-203` "only the operator opens a root; a session opens children with ness open --scope" | part | | C |
| L2 | PL:11-14 | A brief states question, result, limits, autonomy, tier, how to report | none (brief only has to exist) | - | | P (program only, from the template headings) |
| L3 | PL:16-31 | Subagent by default; child only for conversation/decomposition/isolation; never open a child just to ask | none | - | F (partly: root did deep work, E19) | J |
| L4 | PL:32-33 | At the child cap wait for a child to land; never raise the cap or move child work into a subagent | cap: `open.go:263-278` "%d children are already open in this project (launch.max_children = %d); wait for one to land and close it, **or raise launch.max_children**". Nothing stops a session editing `.ness/ness.toml`. | N, and the text invites the breach | **F (operator-reported: root raised the cap; not in `docs/discovery`)** | S |
| L5 | PL:35-42 | Pass `--tier` with the row that fits the job | valid tier and model policy only: `open.go:175,181` | N | | J |
| L6 | PL:44-48 | After a child reports: close it with `--note`; a handoff needs no reply | open obligations refused: `close.go:217` | N | | C |
| L7 | PL:49-51 | Always pass `--subject 'type(scope): summary'` | squash only: `integrate.go:98-107` "a squash lands one commit and needs its subject: --subject 'type(scope): summary', a Conventional Commit subject such as 'feat(cli): add --subject' (got %q)" | part (B4 is turning this into a warning) | F (setup failures, report cause 7) | C |
| L8 | PL:32-33, PE:60 | Integrate pushes only the own branch; a child only into its manager's | `integrate.go:57` "a child integrates only its branch into its manager's; --from/--into between other refs is for roots and the operator" | part | | C |
| L9 | PE:80, integrate | Commit before integrate; clean worktree | `integrate.go:90` "the worktree of X has uncommitted changes; commit them before ness integrate: files" | part | | C |
| L10 | close | Close only landed, reported, obligation-free work | `close.go` / `session/close.go:86-237` (report, branch integrated, clean, children, obligations, PR merged), messages each name the fix | part | | C |

### Builtin `program` (current text, and `C` = Batch C draft)

| Id | Source | Rule | Check today | Ptr | F | Class |
|---|---|---|---|---|---|---|
| P1 | PP/SKILL.md:9-19 | Six steps in order; refinement until the operator accepts the map | none | - | F (cause 1) | J |
| P2 | :22 | Grounding runs in subagents; the root reads conclusions | none | - | partly (L0b early compaction) | J |
| P3 | :23-24 | A named operator preference becomes an open decision, never built past | none | - | | J |
| P4 | :25-26 | Root brings child packets to its chat as one-word answers | none (C 1b replaces it) | - | F (cause 1) | J |
| P5 | :27-28 | Each vertical names its spec owner; root never accepts an operator-owned spec | none | - | | S (Spec owner cell of the vertical's row is filled) / acceptance J |
| P6 | :29 | Injected work becomes a map row before its child opens | none | - | | S (brief's vertical id appears in `docs/program/roadmap.md`) |
| P7 | :30 | Checkpoint at each landing, plan change, scoring change | none (C 1e: success-criteria change) | - | **F: cause 5, asked twice** | S for landing |
| P8 | :31-33 | "Since you left"; ask only what the root cannot get itself | none (C 1c deletes) | - | F (cause 1) | J |
| P9 | :34-35 | Every fold and new vertical is a new map revision; the map ends with a recovery line naming the last decision id | none | - | F (cause 8, compaction) | S |
| P10 | :36-37 | Verification/cutover phase is a map section with probes | none | - | | J |
| P11 | :38 | Fixes to shared tooling go through a quick task, not the root branch | none | - | F (c2 framework commits not on root 11 h) | J |
| P12 | :40-43, C1 | Autonomy asked per run, recorded under `## Autonomy` before the first wave | none (`autonomy.md` slot is a prose line) | - | **F: cause 1, D27-D50 alone overnight** | S |
| P13 | C2 | Roadmap has `## Program doctrine`; project doctrine in `docs/doctrine.md`; entries only with operator words | none | - | F (cause 2) | S for presence only; content J |
| P14 | C3 | Spec/fold review: read the file, list rejected alternatives, "what would make this naive", refuse allowed, independent `ness-review` | none | - | **F: cause 3, specs accepted in 16-97 s, C1 unopened** | J (proxy: decision cites P/G ids, see 3.8) |
| P15 | C3 | A brief states the question and limits, never candidate solutions | none | - | F (C1 brief pinned the candidates) | J |
| P16 | C5 | Children keep no decision ledger; only the root has D-ids | none; PE:38 still tells every full session to keep `decisions.md` with stable ids | - | **F: C2-D1..12, L0a-D1..15, C3-D1..16** | S (program children) |
| P17 | C5 | A wake-up with nothing to decide gets no chat summary | none | - | F (~100 "Nothing waits on you" turns) | J |
| P18 | C4 | Brainstorm/refine in rounds; exit = operator accepts map, doctrine, answers autonomy | none | - | F (cause 1) | J (the Autonomy words can be verified, 3.1) |

### Builtin `go-strict`, `git-conventional`, `behavior-specs`, `example-release`, features

| Id | Source | Rule | Check today | Ptr | F | Class |
|---|---|---|---|---|---|---|
| G1 | ness-test-discipline:6 | No `time.Sleep` in tests | `GS/audit.py:83-85` SLEEP "time.Sleep in a test"; golangci tests `forbidigo` (`golangci.tests.yml`, msg "no wall-clock sleep in tests; use testing/synctest or pass now as a value") | part | | C |
| G2 | :9 | Test files under 400 lines | audit.py:79-82 T400 "N lines (ceiling 400)" | N | | C |
| G3 | :8 | Helpers call `t.Helper()` | `thelper` in `golangci.tests.yml` | N | | C |
| G4 | :7 | No permissive mocks, no call-count assertions; fakes hand-written, never generated | none (no `gomock`/`mock.Anything`/`AssertNumberOfCalls` ban) | - | | S (forbidigo or LIBS-style import ban; FP low) |
| G5 | test-discipline:8, test-strategy | Fixtures in `testdata/`; tests black-box (`package x_test`) | none (`testpackage` linter off) | - | | S (`testpackage`; FP: internal-test files legitimately need package x, so allow-regexp) |
| G6 | ness-interface-design:6 | Interfaces have at most three methods | none (`interfacebloat` off) | - | | S (linter, max 3) |
| G7 | :7 | Interfaces declared by the consumer | none | - | | J |
| G8 | :11-12 | Collaborators built only in composition roots; no package-level mutable state | partial: audit.py:93,111 uses `composition_roots` only for LIBS; `gochecknoglobals` off | - | | S for globals; roots J |
| G9 | :14-15 | No `ForTest` identifiers, no `Clock` interface | audit.py:87-91 FORTEST, CLOCK "pass now as a value, use synctest for timers" | part | | C |
| G10 | ness-documentation:6-9 | godoc on exports, comments end with a period | `revive`, `godot` in `golangci.yml` | N | | C |
| G11 | :9, interface-design:19 | Exactly one package comment per package | audit.py:112-113 PKGDOC "N package comments, want exactly one" | N | | C |
| G12 | ness-documentation:12-13 | README with an "Architectural Role" section | none | - | | S (audit rule README) |
| G13 | compliance-check:8-12 | Run `python3 .ness/audit.py` at plan time and final review; every MUST fixed or owned (`audit-exceptions.txt` as `RULE PATH REASON`) | the gate runs the audit (`GS/Makefile` `validate: ... audit`); exceptions without reason refused `audit.py:49` "exception without a reason: RULE PATH REASON", stale exceptions flagged `:173` | N | | C (at gate) |
| G14 | go-strict libs | Logger/validator from the slot | audit.py:105-107 LIBS "X: the logger is log/slog" | N | | C |
| C1 | ness-git-conventional:9-24 | Conventional Commit subject and trailers | `git-conventional/commit-msg`: "commit-msg: \"X\" is not a Conventional Commit: type(scope): subject, with type one of ..." (only when `core.hooksPath` is activated, a manual step) | N | F (the research lab's commitizen rejected ness's merge subject) | C (if hook installed) |
| B1 | ness-behavior-specs:9-10 | One When, at most 10 steps, 5 rows per scenario | audit.py:121-127 FEATURE "scenario X has N steps (max 10)" | N | | C |
| B2 | :13-14, :17-18 | Shared steps file; godog `--strict`; freeze accepted features; change only via `ness contract amend` | edit guard `internal/session/contract.go:358-360` "X is frozen (dir/contract.lock); run ness contract amend X --reason TEXT to change it" and project-wide variant ("change it on the base branch"). Bash edits are not inspected (`hook/edits.go`); `ness contract check` is **not in the go-strict gate** (`GS/Makefile`) | part | | C, gap: gate omits `contract check` |
| N1 | ness-notify | Always `--dry-run` first; a post is irreversible | none (`post.go:83` only honours the flag) | - | | P (record the dry-run payload hash in the store, refuse the live post of a never-dry-run payload; FP low-moderate) |
| N2 | ness-notify, ness-probes | After enabling the feature run `ness sync` and restart the worker | `ness doctor` / sync (not traced); worker reads features at start | - | | P |
| R1 | ness-example-release | Chain integrate, probe, post, promote | none (an example) | - | | J |
| K1 | ness-probes | Sizing: floor with a positive control, narrow scope | `ness probe run` is an authoring loop only | - | | J |

## 2. Existing checks whose message lacks a pointer (cheap fix: append the skill and section)

| Check | Where | Current text (quoted above) | Pointer to add |
|---|---|---|---|
| Polling guard | `hook/guards.go:19,94-98` `waitingRule` | "...pull once with ness inbox, then end your turn..." | "(ness-entry, Communicate: Waiting rule)" |
| Background guard | `hook/background.go:11,20-26` `bgRule` | "...run it with the Bash tool's run_in_background option..." | "(ness-entry, Communicate: Waiting rule)", **after adding the missing line to that section** (E7) |
| Output guard | `hook/guards.go:103,108` | "reading a whole saved tool result is refused..." | add a prose line (ness-entry, Communicate) and point to it (E8) |
| Child cap | `cli/open.go:277` | "...wait for one to land and close it, or raise launch.max_children" | drop "or raise ...", add "(ness-launch, Child or subagent: at the child cap, wait)" |
| Checkpoint | `cli/send.go:96,99` | "only a root sends a checkpoint..." / "a checkpoint needs the headings: ## Done, ## Now, ## Next" | "(ness-entry, Communicate: checkpoint)" |
| Subject | `cli/integrate.go:104` | "a squash lands one commit and needs its subject..." | "(ness-launch, Wait then fold)" |
| Dirty worktree | `cli/integrate.go:90` | "...commit them before ness integrate..." | "(ness-entry, Lessons/Lifecycle or ness-launch, Wait then fold)" |
| Root close | `cli/close.go:75,247` | "...ask them, then close with their words" | "(ness-entry, Lifecycle: a root closes itself...)" |
| Only operator opens a root | `cli/open.go:202` | "only the operator opens a root; ..." | "(ness-launch, Open a child)" |
| Gate lock busy | `session/integrate.go:277` | "another integrate is running" | "(ness-entry, Tests)" and "retry on the next wake; do not poll" |
| Report missing | `session/close.go:86,93` | "no report at X; write it, then close" | "(ness-entry, Session record)" |
| Contract guard | `session/contract.go:358-360` | "run ness contract amend X --reason TEXT" | "(ness-contracts)" |
| audit.py rules | `GS/audit.py:174-175` prints `RULE path: detail` | e.g. `SLEEP X: time.Sleep in a test` | map RULE to skill and section (T400, SLEEP -> ness-test-discipline, Rules; PKGDOC -> ness-documentation, Comments; FORTEST, CLOCK -> ness-interface-design, Time and test hooks; LIBS -> ness-interface-design, Wiring; FEATURE -> ness-behavior-specs, Scenarios); append `see skill X, section Y` per finding |
| Lint gates | `golangci.yml`, `golangci.tests.yml` forbidigo `msg:` | linter names only | forbidigo `msg:` can carry the pointer; for other linters append a `|| { echo "see ness-interface-design"; exit 1; }` to the Makefile target |
| commit-msg | `git-conventional/commit-msg` | "...is not a Conventional Commit..." | "see skill ness-git-conventional, Subject" |

Two rules are enforced by a hook but written in no skill (E7, E8), so no pointer can resolve today. Add the prose first.

## 3. Ranked shortlist of checks worth building

Value = field breakage x cheapness x low false positives. Item 0 is the batch of message fixes above (no new logic, pure pointer), do it first.

### 3.1 Program child cannot open until the Autonomy mode is recorded (P12)
- **Field**: cause 1, the worst one. **Cost**: low. **FP**: about none (program only; `ness open` of a child, not a root).
- **Mechanism**: precondition in `planLaunch` (`open.go:222`) for a child whose root has the program profile: parse `docs/program/roadmap.md` in the root's worktree; the `## Autonomy` section must contain a mode word (gate, bands, to checkpoint, custom) and a date `YYYY-MM-DD`. Optional hardening: the operator's words quoted there must pass `OperatorSaid` (the same transcript check as `ness close --authorized`, `close.go:225-250`), so the root cannot reword. Needs the profile-declared check slot (section 4, M2) so core does not know the program profile.
- **Trigger**: `ness open` (child) in a program root.
- **Message**: `ness open refused: docs/program/roadmap.md has no recorded autonomy mode. Ask the operator the autonomy question (gate / bands / to checkpoint X / custom), then record the mode, their words and the date under "## Autonomy" (skill ness-program, section Autonomy).`
- **Test**: testscript, three cases: program profile + seed roadmap -> exit 4 with stderr containing `ness-program` and `## Autonomy`; mode+date filled -> opens; project without the program profile -> unaffected. Plus a meta-test (4.M5) that the named section exists in the rendered skill.

### 3.2 Config-driven project guards on pre-tool (fixes E15, L4 and gives E2/E3 their mechanism)
- **Field**: E15 hand-run gates (wave1:94-96), L4 cap raised. **Cost**: medium (one table, one matcher). **FP**: controlled by the project's own regexes; ship two defaults.
- **Mechanism**: `[[guards]]` in `.ness/ness.toml` (or a profile): `tool`, `match` (regex on Bash command or Edit path), `message`, `skill`, `section`. `hook.PreTool` (`guards.go:41`) loops over them after the built-in guards. Built-in defaults: (a) Bash command equal to `cfg.Gate` (or the `make validate` literal) from a bound session -> refuse; (b) Edit/Write of `.ness/ness.toml` whose new text touches `max_children`, `[tiers`, `[launch`, `[worker` -> refuse (`Edit` carries `new_string`, `Write` the whole file).
- **Trigger**: pre-tool, bound session.
- **Messages**: `the full gate runs once, inside ness integrate, under the gate lock; run the tests of the packages you touch (skill ness-entry, section Tests)` and `sessions do not change launch limits, tiers or worker settings; wait for a child to land, or ask your manager (skill ness-launch, section Child or subagent)`.
- **Test**: hook unit tables (blocked and allowed) and a testscript exit 2 with the message on stderr; allowed: `go test ./pkg`, an edit of another key.
- **Honest limit**: the field's hand runs were `pytest`/`ruff`/`mypy`, not the literal gate; the pattern list is the project's (E3 for the research lab). The cap breach is evidenced only by the operator's report.

### 3.3 Chat-only ask detector (E13)
- **Field**: cause 4, 9 h overnight. **Cost**: medium. **FP**: medium if it blocks, low if it only nudges.
- **Mechanism, preferred**: the worker, which already sees a root go idle (B5), sends the root a wake `fyi` when it went idle with its last assistant message ending in a question (or a "?" in the last paragraph) and no operator-addressed open item newer than the operator's last message. Non-blocking, so a false positive costs one turn. Alternative: a `Stop` hook (not wired: `render.go:41` has only SessionStart and PreToolUse) that exits 2 once (`stop_hook_active` guard).
- **Message**: `your last message asks the operator something in chat. The operator reads alerts, not the chat: send it with ness send --to operator --intent request (skill ness-entry, section Ask the operator).`
- **Test**: worker test with a fake transcript: question + no item -> one wake; question + open item -> none; second idle -> no repeat.

### 3.4 Waiting rule covers `ness integrate` retry loops (E6 gap)
- **Field**: c1's `for i in $(seq 1 40) ... sleep 60` integrate loops (wave1:80). **Cost**: trivial. **FP**: low (only a loop with `sleep` around `ness integrate|send|open`).
- **Mechanism**: extend `pollRead` use in `polling()` (`guards.go:94-97`) with a second rule: `shellLoop` and `\bsleep\b` and `\bness\s+(integrate|send|open)\b`.
- **Message**: `a sleep loop around ness integrate is refused: on "another integrate is running" end your turn and retry when a message arrives (skill ness-entry, section Communicate: Waiting rule)`.
- **Test**: unit table next to `guards_test.go`: loop+sleep+integrate blocked; plain `ness integrate`, and `for b in a b; do ness integrate --from $b; done` without sleep, allowed.

### 3.5 Checkpoint required before the next wave (P7)
- **Field**: cause 5. **Cost**: medium (store query for the root's checkpoint items). **FP**: low-medium: batches of simultaneous landings share one checkpoint.
- **Mechanism**: program precondition in `ness open` (child): at least one checkpoint item from the root to the operator newer than the most recent child close. Same slot as 3.1.
- **Message**: `ness open refused: a child landed since your last checkpoint. Send one (ness send --to operator --intent checkpoint --body @FILE --page URL) before the next wave (skill ness-program, section Checkpoints).`
- **Test**: testscript: close a child, `ness open` -> refused; send checkpoint -> opens.

### 3.6 Children keep no decision ledger (P16, E4)
- **Field**: three ledgers (c2:90, l0a-l0b:67). **Cost**: low. **FP**: medium. **Prerequisite**: reconcile PE:38 ("`decisions.md` ... one stable id per entry" for every full session) with C5, otherwise two skills disagree.
- **Mechanism**: `ness integrate` precondition for a child of a program root: the branch must not add `<record>/decisions.md` containing ids matching `\b[A-Z0-9]+-D\d+\b` or `^\s*[-*]\s*\*\*D\d+`.
- **Message**: `integrate refused: <path> is a decision ledger. Children record choices in their report; only the root's docs/program/decisions.md has ids (skill ness-program, section Rules).`
- **Test**: testscript with a child branch adding the ledger -> refused; report only -> integrates.

### 3.7 Recovery line matches the ledger (P9)
- **Field**: cause 8, compaction. **Cost**: low. **FP**: low (fixed format in the seed, `PP/roadmap.md:36`).
- **Mechanism**: at `ness open` (child) and root `ness integrate`: `Recovery line: last decision id D<n>` must equal the highest `D<n>` in `docs/program/decisions.md`.
- **Message**: `the roadmap recovery line says D12 but docs/program/decisions.md ends at D15: update it so a compacted root finds its place (skill ness-program, section Rules: recovery line)`.
- **Test**: testscript with equal and unequal ids.

### 3.8 Report has a Lessons section at close (E17) and decisions.md stays append-only
- **Field**: not seen. **Cost**: low. **FP**: low (a one-line "None." passes).
- **Mechanism**: `session/close.go:93` precondition: `report.md` has a `## Lessons` heading. Append-only: `git diff base...branch -- <record>/decisions.md` shows no removed lines, at `ness integrate`.
- **Message**: `report.md has no "## Lessons" section: add what you learned, even one line (skill ness-entry, section Lessons and briefs)`.
- **Test**: close testscript, report without and with the heading.

### 3.9 Smaller ones, in order
- E12 refuse the Explore subagent (hook on the Agent/Task tool, input `subagent_type`): `Explore runs on the main model: use the ness-sweep subagent (skill ness-entry, section Communicate)`. Zero FP, tiny; no field evidence.
- E10 handoff headings: `send.go` `checkpointBody` clone for `--intent handoff` (Done/Now/Next). FP low.
- E3 edit outside the own worktree but inside the primary checkout: pre-tool using `EditedFiles` and the guard's worktree resolution (R-CTR-010). FP: edits to shared files by design; limit to the repository root.
- G12 audit rule README (Architectural Role); G4/G5/G6/G8 as golangci additions (`interfacebloat` max 3, `testpackage`, `gochecknoglobals`, a forbidigo ban on mock call counts) with the Makefile pointer echo. Existing adopters need the exceptions file; FP moderate on first adoption.
- N1 `ness post` refuses a live post whose payload was never dry-run.
- P5/P6 vertical row filled before `ness open` (spec owner, brief's vertical id in the roadmap).
- Add `ness contract check` to the go-strict `validate` target when `contracts` is on (B2 gap).

### 3.10 Do not build
- P13 (doctrine section present): satisfied by an empty header, no value. P14: a regex on "cites P/G ids" checks presence not truth; agree with the draft's recommendation to leave it to prose.
- E19 root does too much itself: no stable definition of "a few edits"; warn only at best.
- E2 unbound conversation gate: it would block the operator's legitimate "answer" and quick fixes.
- E23 post-compaction re-read: needs per-session tool tally; the printed reminder is the control.

## 4. Mechanisms for a consuming project today (feeds E2) and what is missing

**Exists**
1. `gate` command in `.ness/ness.toml` (run by `ness integrate` under the lock, `integrate.go:fold`): any script, any message, so a pointer is just an `echo`. Single moment: integrate.
2. Local profile `.ness/profiles/local/` applied last (R-PROF-021..023): `set`/`append`/`block` slots, `seed`/`own` files (it can ship its own scripts, lefthook-local.yml, extra skills), `AGENTS.md#local`, list slots (`stop-and-ask`, `post-compact`, `reserved-decisions`). Seams and owned paths are not overridable.
3. Contracts (`[contracts] frozen`, `ness contract freeze/amend/check`): the only project-extensible pre-tool refusal; edits via Edit/Write/MultiEdit/apply_patch only; message names `ness contract amend`, not a skill.
4. `ness sync --check` as a gate step (`GS/Makefile` `selfcheck`): drift in rendered text.
5. Git hooks: `git-conventional` commit-msg (manual `core.hooksPath` activation), lefthook pre-commit/pre-push, per-clone, bypassable.
6. `.ness/audit.py` and `audit.toml` (go-strict): thresholds and an exceptions file only; the `RULES` set is fixed (`audit.py:23`) and the file is `own`, so a project cannot add a rule.
7. Probes: scheduled questions about the running world, not about agent conduct.
8. Project's own `.claude/settings.json` hooks: technically possible beside the generated ones (sync owns keys), but unsupported, with no pointer convention.

**Missing**
- M1 Config or profile-declared pre-tool guards (regex on command or path, message, skill/section) -> 3.2.
- M2 Profile-declared checks at `ness open` / `integrate` / `close` (a script path plus `skill`/`section`; non-zero exit prints the script's text prefixed with the pointer). Lets `program` ship 3.1, 3.5, 3.6, 3.7 without core knowing the profile (the design rule the draft worried about). Inputs via env: `NESS_SESSION`, root, worktree.
- M3 A `Stop` hook event (and a worker nudge channel) for conduct checked at end of turn (3.3).
- M4 Audit plug-in rules (an `audit.d/*.py` or declared regex rules) and a RULE-to-skill table in `audit.toml`.
- M5 A pointer registry and a meta-test: every built-in refusal and profile check carries `skill` + `section`; `go test` and `ness sync --check` verify the section exists as a heading in the rendered skill, and that every MUST-level sentence in a skill has either a rule id with a check or an explicit `<!-- judgement -->` tag. This is what keeps the inventory true after the next prose edit.
- M6 One command (`ness check`) running all profile checks on demand and inside the gate, so a rule is gated even when the moment-based trigger is bypassed.
- M7 Convention doc for E2: a local rule is written as skill/section + check + message that names them; the test is the pointer-resolves meta-test of M5.

## 5. Field-broken rules at a glance

Polling and shell backgrounding (E6, E7: checked, gaps: sleep loops around integrate, no skill pointer, no prose for E7); asks in chat only (E13: unchecked); gate run by hand (E15: unchecked); root doing deep work (E19: unchecked); child cap raised (L4: cap checked, raising not, message invites it); D-id ledgers in children (P16: unchecked, and PE:38 invites it); autonomy never asked and checkpoints skipped (P12, P7: unchecked); specs accepted unread (P14: judgement); post-compact reload (E23: printed only); brief pinning candidates (P15: judgement); commit-msg rejecting ness's merge subject (C1, L7: checked, wrong target).
