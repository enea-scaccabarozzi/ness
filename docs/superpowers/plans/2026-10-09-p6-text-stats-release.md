> [!NOTE]
> Archive: altered for the public release. Names of private projects and local paths were replaced; the content is otherwise as written.

# P6 Text, Stats and Release Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** The last v1 plan: the core skill text (`ness-entry`, a new `ness-launch`) and the three feature skill texts say what the design says; the five built-in profiles (`program`, `go-strict`, `behavior-specs`, `git-conventional`, `example-release`) render in a fixture repo and together; `ness stats` replaces the stub and reads the Claude Code transcripts bound to this project's sessions into tokens, context, compactions, tool bytes and cost with an embedded dated price table; `ness init` prompts on a terminal; every `--json` command has a golden and the exit-code table has one script; `ness sync` prints the release notes between the lock's version and the binary's from an embedded `CHANGELOG.md`; goreleaser, release-please, commit lint and the weekly gremlins run are configured and checked locally without publishing anything; the compliance audit joins `make validate`; BLT and STAT join the coverage gate and every id deferred to P6 is delivered.

**Architecture:** Text lives in `profiles/` (embedded, R-PROF-002) and reaches the repository through the existing slot and file machinery of `internal/profile` and `internal/sync`: core gains the lifecycle slots (`specify`, `plan`, `handoff`, `reserved-decisions`, `tier-roles`) the quality profiles append to; each built-in is a directory of `profile.toml` plus Markdown, scripts and configs; no new profile capability is added. `internal/stats` is the transcript reader and the price table, a pure package over `io.Reader`; `internal/cli/stats.go` binds it to the store's closed-session rows and the claude atom's transcript path. Release notes are a root package `ness` embedding `CHANGELOG.md`, parsed by `internal/sync/notes.go`, printed by `runSync` after `Apply`. The compliance audit is one Python script rendered by `go-strict` and run by `make validate`; ness applies `go-strict` to itself (R-QA-006), so the rendered lint configs are the repo's own.

**Tech Stack:** Go 1.27.2 and golangci-lint 2.14.0 through `mise exec --`; python 3.13 (mise, R-QA-041) for the audit; goreleaser 2 (mise) for `goreleaser check` and a snapshot build; testscript; `encoding/json`, `bufio`; no new module dependency.

**Spec:** `docs/superpowers/specs/2026-10-08-ness-v1-scope.md` (P6 row); `docs/spec/profiles-builtin.md` (BLT, all), `docs/spec/stats.md` (STAT, all), `docs/spec/quality.md` (R-QA-004, R-QA-005, R-QA-023, R-QA-030, R-QA-031, R-QA-041), and the carry-ins `docs/spec/core-config.md` R-CFG-032, R-CFG-034, R-CFG-035, R-CFG-037 (the audit of goldens), R-CFG-051; `docs/spec/sync.md` R-SYNC-032, R-SYNC-033, R-SYNC-089; `docs/spec/profiles.md` R-PROF-002; `docs/spec/models.md` R-MOD-002, R-MOD-021. Design: `docs/design/ness-design.md` §2, §3, §5, §7, §8 (sizing), §10, §11, §12, §13, §14, §16; decisions D19, D20, D25, D26, D27, D29, D31.

## Global Constraints

- Toolchain: every command below runs under `mise exec --` (Go 1.27.2, golangci-lint 2.14.0, python 3.13, goreleaser 2 after Task 7). The gate is `make validate` plus `make fuzz FUZZTIME=10s`.
- Lint: cyclop at 10 or below, funlen default, lll at 120 columns with tab width 4, gofumpt, no shadowed `err`, `errcheck check-blank` (never `_ = f()`), no `fmt.Print*`; tests are excluded from lint. Every exported symbol has a doc comment starting with its name.
- Exit codes (R-CFG-035): 0 success; 1 a check found problems; 2 usage; 3 sync conflict; 4 refused, nothing written.
- Tests never send a real network request, never start a real claude, codex or herdr session, never run a mutating `systemctl`, never `time.Sleep`; the e2e tag stays off. Scripts are hermetic (`testkit.Setup`); every test names the ids it covers (`// covers:` / `# covers:`).
- Migrations are append-only; the latest is 5 and this plan adds none. Tests never pin the store version; they use `store.Version()`.
- Built-in profile text and examples carry no private product information: no company or product names, hostnames, channel ids, people, repository paths, real URLs, tokens or e-mails (D27, R-BLT-003). Examples use `example.com`, `acme`, `releases`, `prod`. The design's private-named example profile ships as `example-release` (ruling below).
- Every skill a built-in or core renders is `.agents/skills/ness-<name>/` with `name: ness-<name>` in its frontmatter (R-BLT-006).
- goreleaser and release-please are configured and checked locally (`goreleaser check`, a `--snapshot` build); nothing is published, no tag is pushed, nothing is uploaded. No `git push`, no CI watch.
- Conventional commits; the type `sessions` (the "Running sessions" release-notes section) is accepted by the hook and the CI lint.
- Dogfood only in a throwaway repository, except `ness sync` in this checkout after the core text or go-strict changes (this repository's generated files must stay current for `make validate`'s selfcheck).

## Review Focus

1. **A transcript line with a `<synthetic>` model or an all-zero `usage`** (Claude Code writes them for local events). It is neither priced nor a turn and never sets the context. Pinned in Task 4 (`TestReadTheFixture`, the synthetic line).
2. **`--since` in the future** (`--since 2999-01-01`). Zero figures, every row present with zeros, exit 0; only a value that does not parse is a usage error. Pinned in Task 5 (`stats.txtar`, the future date).
3. **A bound session whose transcript file is missing** (the client never wrote it, or the home directory moved). The session is listed with "no transcript" and zero figures; stats never fails on it. Pinned in Task 5 (`stats.txtar`, `s-eeeeeeeeeeee`).
4. **A CHANGELOG heading that is not a version** (`## Unreleased`, `## Notes`). The parser skips it with its text and never crashes; `## [0.1.0]`, `## 0.1.0` and `## v0.1.0` all parse. Pinned in Task 7 (`TestNotesSkipsNonVersionHeadings`).
5. **`ness init` on a terminal answered with an empty gate.** The refusal names `--gate`, exit 4, and `.ness/` is not created. Pinned in Task 6 (`init_tty.txtar`, the empty answers).

## Deferred and rulings

Deferred (listed by the coverage test in Task 8 so the gate stays green while the gap stays visible):

- R-BLT-056, R-BLT-063, R-BLT-073 (skill evals against the old text, local real-client runs under D31): `"evals (local, D31)"`. They need the private source skills and a real client; they run on demand after v1, never in CI (R-QA-042).
- R-QA-032, R-QA-033 stay `"publication"`; R-QA-025 stays `"withdrawn (D39)"`.
- The review-kind BLT ids (R-BLT-017, R-BLT-018, R-BLT-031, R-BLT-035, R-BLT-036, R-BLT-037, R-BLT-040, R-BLT-051, R-BLT-052, R-BLT-062, R-BLT-090) are listed as `"review"` until the end-of-plan reviewer writes their `docs/spec/reviews.md` entries and deletes the lines (as P5a did for R-NTF-030).

Rulings:

- **The design's private-named example profile ships as `example-release`.** The hard rule is no private product name anywhere in the built-ins; a profile directory name is part of the built-in. Task 2 renames it in `docs/spec/profiles-builtin.md` with a note; the ids R-BLT-090..093 are unchanged.
- **Slot keys use the rendered skill or file as their target** (R-PROF-017, R-PROF-041): the spec's `program-roadmap#autonomy` and `operator-review-format` are `ness-program#autonomy` and `ness-program#operator-review-format`; go-strict's slots live in the files they fill (`.golangci.yml#module-path`, `.golangci.yml#go-version`, `.ness/audit.toml#<key>`); `ness-launch#tier-roles` and `ness-entry#reserved-decisions` are list slots of core.
- **go-strict's skills carry the prefix** (`ness-interface-design`, `ness-test-strategy`, `ness-test-discipline`, `ness-documentation`, `ness-compliance-check`): R-BLT-006 wins over the bare names in R-BLT-050.
- **The compliance audit is a new, small Python script** (`audit.py`, rendered as `.ness/audit.py`) with decidable rules (test-file ceiling, `time.Sleep` in tests, `ForTest`, a `Clock` interface, one package comment per package, a logger or validator other than the configured one outside the composition roots, the Go version floor, godog scenario shape) and an exceptions file with mandatory reasons. D26's "existing script" is private and cannot ship; the judgment items of §11 stay in the skill text.
- **git-conventional's hook is `.githooks/commit-msg`, activated once with `git config core.hooksPath .githooks` and `chmod +x`.** Sync writes files, never git config or modes; the skill text says how to activate it and how to run it from `lefthook-local.yml` when go-strict's lefthook is installed.
- **ness applies go-strict to itself** (R-QA-006): `.ness/ness.toml` lists `profiles = ["go-strict"]`, a `local` profile sets the module path and Go version, and the rendered `.golangci.yml`, `.golangci.tests.yml`, `lefthook.yml`, `.ness/audit.py` and `.ness/audit.toml` are committed. The hand-written `Makefile` stays (a present seed is adopted).
- **`ness stats` reads the store read-only** (`store.OpenReadOnly`, as doctor does) and refuses with the migration hint when the store is behind; no store means no sessions and zero figures.
- **Context size of a group row is the maximum over its sessions**; a session row's is its last turn's last call (R-STAT-010). Cost is whole cents; a model the table does not know is `unpriced` with cost 0 and the row says so.
- **The price table** is the Anthropic first-party rates of 2026-09-25 (input, output, cache read as published; cache write at 1.25x input, the 5-minute cache) by model-id prefix, longest prefix wins. `PriceDate` is printed with every output.
- **The private stats fixture** (R-STAT-008) is `.private/stats/*.jsonl` plus `.private/stats/expected.json` (`{"cost_cents": N, "input_tokens": N, "output_tokens": N}`), gitignored; the test skips without it; CI runs the synthetic golden.
- **Release notes are a root package** `ness` (`notes.go`, `//go:embed CHANGELOG.md`) so the changelog stays at the repository root where release-please and readers expect it; `TestPackageLayout` admits the root.
- **R-SYNC-032's "in order" is oldest first** (0.3.1, then 0.3.2) so a reader upgrades through the versions as they were cut.
- **`ness init` prompts for project, base, gate and client** (each value a flag did not give) when stdin is a terminal and `--yes` is absent; `Deps.TTY` is set by `cmd/ness` and forced by scripts with `NESS_TEST_TTY=1` (a pseudo-terminal in testscript would test util-linux, not ness).
- **The CI-kind QA ids (R-QA-004, R-QA-005, R-QA-030) and the review-kind ones (R-QA-023, R-QA-031, R-QA-041) are pinned by a unit test over the config files** (`internal/testkit/release_test.go`) rather than a review entry: a mechanical test holds when the files change.
- **Mutation testing** runs weekly through `go run github.com/go-gremlins/gremlins/cmd/gremlins@v0.5.0` in its own workflow with `continue-on-error`, reports as artifacts, never failing the gate (R-QA-023). If that version does not resolve, pick the newest from `go list -m -versions github.com/go-gremlins/gremlins`.

## File map

```
profiles/core/profile.toml                        ness-launch joins the owned skills
profiles/core/skills/ness-entry/SKILL.md          rewritten: sizes, record, communicate, lifecycle slots, reserved decisions
profiles/core/skills/ness-launch/SKILL.md         new: open a child, tier roles slot, wait then fold
profiles/features/{probes,notify,contracts}/SKILL.md   rewritten (BLT); probes carries the §8 sizing guidance
profiles/embed.go                                 Scan (denylist), Names
profiles/profiles_test.go                         five built-ins, ness- prefix, denylist scan
profiles/builtin/program/                         profile.toml, skills/ness-program/SKILL.md, size.md, post-compact.md,
                                                  autonomy.md, review-format.md, roadmap.md, decisions.md, brief-template.md
profiles/builtin/git-conventional/                profile.toml, skills/ness-git-conventional/SKILL.md, commit-msg
profiles/builtin/example-release/                 profile.toml, skills/ness-example-release/SKILL.md, ci-after-push.md,
                                                  release-notes-template.md
profiles/builtin/go-strict/                       profile.toml, golangci.yml, golangci.tests.yml, lefthook.yml, Makefile,
                                                  audit.py, audit.toml, audit-exceptions.txt, slots/*.md, lifecycle/*.md,
                                                  skills/ness-{interface-design,test-strategy,test-discipline,documentation,
                                                  compliance-check}/SKILL.md
profiles/builtin/behavior-specs/                  profile.toml, skills/ness-behavior-specs/SKILL.md, specify.md
profiles/builtin/README.md                        updated
.ness/ness.toml, .ness/profiles/local/            ness applies go-strict to itself (Task 3)
.golangci.yml, .golangci.tests.yml, lefthook.yml, .ness/audit.py, .ness/audit.toml, .ness/audit-exceptions.txt   rendered
Makefile                                          audit target in validate
mise.toml                                         python, goreleaser
internal/stats/stats.go, prices.go, since.go      Read, Transcript, Usage, Cost, PriceDate, ParseSince
internal/stats/stats_test.go, prices_test.go, since_test.go, private_test.go, testdata/fixture.jsonl
internal/cli/stats.go                             ness stats --since --by --json
internal/cli/atoms.go                             transcriptPath
internal/cli/init.go                              prompts on a terminal (R-CFG-051)
internal/cli/cli.go                               Deps.TTY, Deps.Notes
internal/cli/sync.go                              printNotes after Apply
internal/cli/script_test.go                       NESS_TEST_TTY, NESS_TEST_NOTES seams
internal/cli/testdata/script/core_text.txtar, feature_text.txtar, builtin_program.txtar, builtin_git_conventional.txtar,
  builtin_example_release.txtar, builtin_go_strict.txtar, builtin_behavior_specs.txtar, builtin_all.txtar, stats.txtar,
  stats_json.txtar, init_tty.txtar, exit_codes.txtar, json_surface.txtar, json_goldens.txtar, release_notes.txtar
internal/cli/testdata/script/surface.txtar        the stub assertion goes
internal/testkit/testkit.go                       onejson verb; GOCACHE/GOMODCACHE pass-through
internal/testkit/store.go                         store add-launch verb
internal/testkit/release_test.go                  goreleaser, release-please, workflows, mise pins (R-QA-004/005/023/030/031/041)
internal/testkit/repo_test.go                     "." joins the layout
internal/testkit/coverage_repo_test.go            BLT, STAT join doneAreas; deferred edited
internal/sync/notes.go, notes_test.go, engine.go  Notes, Between, Plan.Previous
notes.go, CHANGELOG.md                            the root package and the embedded notes
cmd/ness/main.go                                  TTY wiring
.goreleaser.yaml, release-please-config.json, .release-please-manifest.json
.github/workflows/ci.yml, release.yml, release-please.yml, mutation.yml
.gitignore                                        .private/
docs/spec/profiles-builtin.md                     example-release note
README.md                                         status line
```

---

### Task 1: Core text and the feature skill texts

**Files:**
- Modify: `profiles/core/profile.toml`, `profiles/core/skills/ness-entry/SKILL.md`, `profiles/features/probes/SKILL.md`, `profiles/features/notify/SKILL.md`, `profiles/features/contracts/SKILL.md`
- Create: `profiles/core/skills/ness-launch/SKILL.md`, `internal/cli/testdata/script/core_text.txtar`, `internal/cli/testdata/script/feature_text.txtar`
- Regenerate in this checkout: `.agents/skills/ness-entry/SKILL.md`, `.agents/skills/ness-launch/SKILL.md`, `.ness/lock.json`

**Interfaces:**
- Produces: the list slots `ness-entry#sizes`, `ness-entry#specify`, `ness-entry#plan`, `ness-entry#handoff`, `ness-entry#stop-and-ask`, `ness-entry#reserved-decisions`, `ness-entry#post-compact`, `ness-launch#tier-roles`. Tasks 2 and 3 append to `sizes`, `post-compact`, `specify`, `plan`, `handoff`.

- [ ] **Step 1: Write the failing scripts**

`internal/cli/testdata/script/core_text.txtar`:
```
# Core renders ness-entry and ness-launch; the text defines the three sizes, the record files,
# the waiting rule, the subagent summary cap, the no-mid-session-change rule, names standard as
# the default tier and ness-sweep for searches; every slot accepts an append from a profile.
# covers: R-BLT-010, R-BLT-011, R-BLT-012, R-BLT-013, R-BLT-014, R-BLT-015, R-BLT-016, R-BLT-019, R-BLT-061, R-MOD-002, R-MOD-021
cd repo
exec ness init --gate true --base main --yes
grep 'Applies only when the `ness` command exists in this checkout. Otherwise ignore this block.' AGENTS.md
exists .agents/skills/ness-entry/SKILL.md
exists .agents/skills/ness-launch/SKILL.md
grep '^name: ness-entry$' .agents/skills/ness-entry/SKILL.md
grep '^name: ness-launch$' .agents/skills/ness-launch/SKILL.md
grep '^- \*\*answer\*\*' .agents/skills/ness-entry/SKILL.md
grep '^- \*\*quick task\*\*' .agents/skills/ness-entry/SKILL.md
grep '^- \*\*full session\*\*' .agents/skills/ness-entry/SKILL.md
grep '`brief.md`' .agents/skills/ness-entry/SKILL.md
grep '`specs.md`' .agents/skills/ness-entry/SKILL.md
grep '`plan.md`' .agents/skills/ness-entry/SKILL.md
grep '`decisions.md`' .agents/skills/ness-entry/SKILL.md
grep '`report.md`' .agents/skills/ness-entry/SKILL.md
grep '<sessions>/<root>/\[<child>/\]' .agents/skills/ness-entry/SKILL.md
grep 'pull the inbox once, then end your turn' .agents/skills/ness-entry/SKILL.md
grep 'no Monitor, no cron, no ScheduleWakeup, no sleep loops' .agents/skills/ness-entry/SKILL.md
grep '8,000 characters' .agents/skills/ness-entry/SKILL.md
grep 'never change mid-session' .agents/skills/ness-entry/SKILL.md
grep 'one tier up' .agents/skills/ness-entry/SKILL.md
grep '`ness-sweep`' .agents/skills/ness-entry/SKILL.md
grep 'decision-first' .agents/skills/ness-entry/SKILL.md
grep '`standard` tier by default' .agents/skills/ness-launch/SKILL.md
grep '`high` only' .agents/skills/ness-launch/SKILL.md
! grep 'ness:slot' .agents/skills/ness-entry/SKILL.md
! grep 'ness:slot' .agents/skills/ness-launch/SKILL.md
# A local profile appends to every slot core provides.
exec sh -c 'printf "profiles = [\"local\"]\n" >> .ness/ness.toml'
exec ness sync
grep '^- \*\*program\*\*: four' .agents/skills/ness-entry/SKILL.md
grep '^- never force-push' .agents/skills/ness-entry/SKILL.md
grep '^- reload the roadmap' .agents/skills/ness-entry/SKILL.md
grep '^- pricing' .agents/skills/ness-entry/SKILL.md
grep '^  - invoke /ness-x-specify' .agents/skills/ness-entry/SKILL.md
grep '^  - invoke /ness-x-plan' .agents/skills/ness-entry/SKILL.md
grep '^  - invoke /ness-x-handoff' .agents/skills/ness-entry/SKILL.md
grep '^- \*\*archivist\*\*' .agents/skills/ness-launch/SKILL.md

-- repo/.ness/profiles/local/profile.toml --
name = "local"
[slots]
"ness-entry#sizes" = { append = "sizes.md" }
"ness-entry#stop-and-ask" = { append = "stop.md" }
"ness-entry#post-compact" = { append = "compact.md" }
"ness-entry#reserved-decisions" = { append = "reserved.md" }
"ness-entry#specify" = { append = "specify.md" }
"ness-entry#plan" = { append = "plan.md" }
"ness-entry#handoff" = { append = "handoff.md" }
"ness-launch#tier-roles" = { append = "roles.md" }
-- repo/.ness/profiles/local/sizes.md --
- **program**: four verticals and a map.
-- repo/.ness/profiles/local/stop.md --
- never force-push
-- repo/.ness/profiles/local/compact.md --
- reload the roadmap
-- repo/.ness/profiles/local/reserved.md --
- pricing
-- repo/.ness/profiles/local/specify.md --
  - invoke /ness-x-specify
-- repo/.ness/profiles/local/plan.md --
  - invoke /ness-x-plan
-- repo/.ness/profiles/local/handoff.md --
  - invoke /ness-x-handoff
-- repo/.ness/profiles/local/roles.md --
- **archivist**: economy.
```

`internal/cli/testdata/script/feature_text.txtar`:
```
# A feature's skill text is rendered only while the feature is on; the probes text carries the
# sizing guidance and the worker-restart note; notify and contracts name their commands.
# covers: R-BLT-004
cd repo
exec ness init --gate true --base main --yes
! exists .agents/skills/ness-probes
! exists .agents/skills/ness-notify
! exists .agents/skills/ness-contracts
exec sh -c 'printf "features = [\"probes\", \"notify\", \"contracts\"]\n" >> .ness/ness.toml'
exec ness sync
grep '^name: ness-probes$' .agents/skills/ness-probes/SKILL.md
grep '^## Sizing a probe' .agents/skills/ness-probes/SKILL.md
grep 'positive control' .agents/skills/ness-probes/SKILL.md
grep 'Raise `sample`' .agents/skills/ness-probes/SKILL.md
grep 'Skip the floor' .agents/skills/ness-probes/SKILL.md
grep '`jitter`' .agents/skills/ness-probes/SKILL.md
grep 'restart the worker' .agents/skills/ness-probes/SKILL.md
grep '^name: ness-notify$' .agents/skills/ness-notify/SKILL.md
grep 'ness post --channel NAME --file NOTE.md' .agents/skills/ness-notify/SKILL.md
grep 'restart the worker' .agents/skills/ness-notify/SKILL.md
grep '^name: ness-contracts$' .agents/skills/ness-contracts/SKILL.md
grep 'ness contract freeze' .agents/skills/ness-contracts/SKILL.md
exec sh -c 'sed -i "/^features = /d" .ness/ness.toml'
exec ness sync
! exists .agents/skills/ness-probes
! exists .agents/skills/ness-notify
! exists .agents/skills/ness-contracts
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/cli/ -run 'TestScripts/(core_text|feature_text)'`
Expected: FAIL (`ness-launch` does not exist; the greps miss).

- [ ] **Step 3: Write the core text**

`profiles/core/profile.toml`:
```toml
name    = "core"
summary = "ness core: the AGENTS.md router and the lifecycle skills"

[slots]
"AGENTS.md#router" = { block = "router.md" }

[files]
".agents/skills/ness-entry/SKILL.md"  = { own = "skills/ness-entry/SKILL.md" }
".agents/skills/ness-launch/SKILL.md" = { own = "skills/ness-launch/SKILL.md" }
```

`profiles/core/skills/ness-entry/SKILL.md`:
```markdown
---
name: ness-entry
description: Start here in a ness project. Covers who you are in the session tree, the work sizes, the session record, how to communicate and wait, and when to stop and ask.
---

# ness entry

You are one session in a tree of accountable sessions. Your brief names your scope, your
manager, and the result you owe. The operator is the human; your manager is the session one
level up, or the operator when you are a root.

## Work sizes
- **answer**: reply in chat, with no session record.
- **quick task**: one branch, a few commits, and a short report.
- **full session**: a brief, plan, decisions and report under the session directory.
<!-- ness:slot list sizes -->

## Session record
A full session keeps its record under `<sessions>/<root>/[<child>/]` (`sessions` in
`.ness/ness.toml`, default `docs/sessions`): `brief.md` (what you were asked, verbatim),
`specs.md` (what you agreed to build), `plan.md` (the steps), `decisions.md` (append-only, one
stable id per entry) and `report.md` (what you delivered, with the evidence and your lessons).

## Communicate
- `ness inbox` lists what is addressed to you; `ness send` and `ness reply` are the only way to
  reach another session or the operator. The worker wakes a session when something arrives.
- Waiting rule: to wait for a reply or a child, pull the inbox once, then end your turn. Never
  poll: no Monitor, no cron, no ScheduleWakeup, no sleep loops around `ness inbox` or
  `ness status`. The pre-tool hook refuses them.
- Messages to the operator are decision-first and short: the decision or the question in the
  first line, the reasons after. Long content goes to a page the operator opens, not the chat.
- Subagents write their results to a file and return the path plus a summary of at most about
  2k tokens (about 8,000 characters). For searches use the `ness-sweep` subagent, never
  Explore: Explore runs on the main model.

## Model and effort
Your model and effort never change mid-session. When a task needs more, write your state down
(`ness inbox --boundary NOTE`), end your turn, and let your manager resume you one tier up.

## Lessons and briefs
- Append what you learned to the lessons section of your report at handoff; read your
  manager's lessons at entry.
- A brief does not promote side remarks to rules: only what it states as a requirement binds.
- A root delegates anything over a few edits to a child or a subagent and keeps the map.

## Lifecycle
- At specify, before any design:
<!-- ness:slot list specify -->
- At plan, before the first commit:
<!-- ness:slot list plan -->
- At handoff, before the report:
<!-- ness:slot list handoff -->

## Stop and ask
Stop and ask your manager before anything you cannot undo.
<!-- ness:slot list stop-and-ask -->

## Reserved decisions
Decisions the operator keeps; everything else is yours within your brief.
<!-- ness:slot list reserved-decisions -->

## After compaction
Re-read your brief and your inbox before continuing.
<!-- ness:slot list post-compact -->
```

`profiles/core/skills/ness-launch/SKILL.md`:
```markdown
---
name: ness-launch
description: How a manager opens, briefs, waits for and closes child sessions, and which tier each job gets.
---

# ness launch

## Open a child
`ness open --scope NAME --tier TIER --brief FILE [--title T] [--effort E] [--client claude|codex]`
opens a worktree, a branch and a tab for the child and hands it your brief. Only the operator
opens a root (`--root`). Write the brief first: the question, the result owed, the limits, the
tier, and how to report. The child's record lives under yours.

## Tiers
Every session runs on the `standard` tier by default; pass `--tier` with the row that fits the
job. Effort is set per launch with `--effort`, `high` only where verification matters, and is
never changed once the session runs: escalate by resuming one tier up.
- **standard**: coordination, implementation, reviews.
- **frontier**: design between alternatives, pre-registration, conclusion audits.
- **economy**: mechanical children.
<!-- ness:slot list tier-roles -->

## Wait, then fold
Pull `ness inbox` once and end your turn; the worker wakes you when the child reports. When
the child's branch is in, `ness close ID --note TEXT` closes it. `ness integrate` folds your
own branch through the gate into your manager's branch.
```

- [ ] **Step 4: Write the feature skill texts**

`profiles/features/probes/SKILL.md`:
```markdown
---
name: ness-probes
description: Probes are scheduled questions about the running world, owned by a session; the worker runs the floor, the woken agent judges within the scope.
---

# ness probes

A probe is one Markdown file: frontmatter (`every` or `once`; optional `jitter`, `window`,
`floor`, `expect`, `confirm`, `timeout`, `wake`, `sample`, `notify`, `channel`) and a body,
the scope you check when woken. The floor is a command: exit 0 is GREEN, 1 is RED, anything
else or a timeout is UNCLEAR; with `expect`, the last line's `key=value` pairs must satisfy it
or the run is VACUOUS.

- `ness probe start PATH... [--keep-window]` — register files or a directory; you own them.
  Edits reload; a paused probe at the same path is adopted.
- `ness probe run PATH [--json]` — run the floor now, print the state and the evidence, record
  nothing: the authoring loop.
- `ness probe verdict ITEM --state green|red --note TEXT [--probe-bug]` — answer a wake.
- `ness probe stop PATH --note TEXT` — stop your probe.

A wake is a request in your inbox carrying the scope and the capped, redacted evidence. Judge
it, then answer with `ness probe verdict`. `--probe-bug` records a defect in the probe, posts
nothing and flags it `needs-edit` until the file changes. When you close, your probes move to
your manager; without a live owner a probe pauses and shows red in `ness status`.

## Sizing a probe
- Floor first when a cheap command separates RED from GREEN, and give it a positive control
  (a case you know is RED) before you trust its first GREEN.
- Raise `sample` as the floor's coverage falls: the less the command sees, the more GREENs you
  look at yourself.
- Skip the floor when no command can judge, or when your own look is cheaper than writing one.
- Keep the scope narrow and named: one question per file.
- Set `jitter` when the risk depends on the hour, so the runs do not always land at the same
  minute.

## Enabling the feature
The worker reads `features` and the channel configuration at its start: after enabling
`probes`, or changing a channel, run `ness sync`, then restart the worker
(`systemctl --user restart ness-<project>`), or no floor will run.
```

`profiles/features/notify/SKILL.md`:
```markdown
---
name: ness-notify
description: Post one Markdown note to a configured channel (Slack, Telegram, GitHub, command); separate from ness send because an external post is irreversible.
---

# ness notify

- `ness post --channel NAME --file NOTE.md [--thread KEY] [--dry-run]`

Channels are named in `.ness/ness.toml` under `[notify.channels]`; a secret comes only from
the environment variable `token_env` names. The note is rendered for the platform and split on
paragraph and code-fence boundaries; a `NOTE.NAME.md` or `NOTE.KIND.md` next to it is sent
verbatim instead. `--thread KEY` keeps related posts in one thread: the first post starts it,
later ones reply in it. Always `--dry-run` first: it prints exactly what would be sent. A post
is irreversible.

## Enabling the feature
The worker reads `features` and the channel configuration at its start: after enabling
`notify`, or changing a channel, run `ness sync`, then restart the worker
(`systemctl --user restart ness-<project>`), or probe posts will not go out.
```

`profiles/features/contracts/SKILL.md`:
```markdown
---
name: ness-contracts
description: Freeze files you agreed on; the pre-tool guard refuses edits to them until an amendment is opened, and the gate checks them.
---

# ness contracts

- `ness contract freeze PATHS...` — record each file's blob sha in your `contract.lock`.
- `ness contract amend PATH --reason TEXT` — open one frozen file for change; the reason goes
  to the session's `decisions.md`. `freeze` it again when done.
- `ness contract check` — the gate's check: every frozen file matches its sha, or is open;
  exit 1 names each one that differs.

Edit, Write and MultiEdit on a frozen file are refused by the pre-tool hook. Changes made
through Bash are caught by `ness contract check`. Project-wide frozen paths are listed in
`.ness/ness.toml` under `[contracts] frozen`.
```

- [ ] **Step 5: Run the scripts, then the tree, then re-sync this checkout**

Run: `go test ./internal/cli/ -run 'TestScripts/(core_text|feature_text)' && go test ./...`
Expected: PASS. If a script or unit test elsewhere pinned the old `ness-entry` text (grep the failure's expectation), update that expectation to the new text: the slot names `sizes`, `stop-and-ask`, `post-compact` are unchanged, so `profiles_conflicts.txtar` and the P1 slot tests still hold.

Then, in this checkout: `go run ./cmd/ness sync` (expected stdout: `update .agents/skills/ness-entry/SKILL.md`, `create .agents/skills/ness-launch/SKILL.md`) and `go run ./cmd/ness sync --check` (exit 0, no output).

- [ ] **Step 6: Commit**

```bash
git add profiles internal/cli/testdata .agents .ness/lock.json
git commit -m "feat(profiles): core text: sizes, record, waiting rule, lifecycle slots, ness-launch; feature skill texts with probe sizing and the worker-restart note"
```

---
### Task 2: Built-ins: program, git-conventional, example-release; the denylist scan

**Files:**
- Create: `profiles/builtin/program/{profile.toml,size.md,post-compact.md,autonomy.md,review-format.md,roadmap.md,decisions.md,brief-template.md,skills/ness-program/SKILL.md}`, `profiles/builtin/git-conventional/{profile.toml,commit-msg,skills/ness-git-conventional/SKILL.md}`, `profiles/builtin/example-release/{profile.toml,ci-after-push.md,release-notes-template.md,skills/ness-example-release/SKILL.md}`, `profiles/profiles_test.go`, `internal/cli/testdata/script/builtin_program.txtar`, `builtin_git_conventional.txtar`, `builtin_example_release.txtar`
- Modify: `profiles/embed.go` (`Scan`, `Names`), `profiles/builtin/README.md`, `docs/spec/profiles-builtin.md` (the name note), `.gitignore` (`.private/`)

**Interfaces:**
- Produces: `profiles.Names() ([]string, error)` (the built-in directory names, sorted); `profiles.Scan(fsys fs.FS, words []string) []string` (`path: word` per hit, case-insensitive); the slots `ness-program#autonomy` and `ness-program#operator-review-format` (single).

- [ ] **Step 1: Write the failing tests**

`profiles/profiles_test.go`:
```go
package profiles_test

import (
	"errors"
	"io/fs"
	"os"
	"path"
	"slices"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/enea-scaccabarozzi/ness/profiles"
)

// fixedDenylist guards the generalisation of D27 on every machine, private list or not.
// fixedDenylist: generic leak patterns only (see preflight P-06); product names live in .private/denylist.txt.
var fixedDenylist = []string{ /* replaced by the P-06 pattern scan */ }

func TestFiveBuiltins(t *testing.T) {
	// covers: R-BLT-100
	names, err := profiles.Names()
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"behavior-specs", "example-release", "git-conventional", "go-strict", "program"}
	if !slices.Equal(names, want) {
		t.Fatalf("built-ins = %v, want %v", names, want)
	}
}

func TestSkillsCarryThePrefix(t *testing.T) {
	// Every skill directory of core and the built-ins is ness-<name>, and its frontmatter name
	// equals the directory (R-BLT-006). Feature skills are named by the renderer (ness-<feature>).
	// covers: R-BLT-006
	for _, fsys := range []fs.FS{profiles.Core(), profiles.Builtin()} {
		err := fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() || d.Name() != "SKILL.md" {
				return err
			}
			dir := path.Base(path.Dir(p))
			b, rerr := fs.ReadFile(fsys, p)
			if rerr != nil {
				return rerr
			}
			if !strings.HasPrefix(dir, "ness-") {
				t.Errorf("%s: skill directory %q lacks the ness- prefix", p, dir)
			}
			if !strings.Contains(string(b), "\nname: "+dir+"\n") {
				t.Errorf("%s: frontmatter name is not %q", p, dir)
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestDenylistScan(t *testing.T) {
	// A planted entry is found; the real built-ins contain none of the fixed list; the private
	// list is scanned when present and skipped when absent (R-BLT-003, R-BLT-092).
	// covers: R-BLT-003, R-BLT-092
	planted := fstest.MapFS{
		"x/profile.toml": {Data: []byte("name = \"x\"\n")},
		"x/text.md":      {Data: []byte("Deploy to Acme-Prod at 10:00\n")},
	}
	if hits := profiles.Scan(planted, []string{"acme-prod", "nothing"}); !slices.Equal(hits, []string{"x/text.md: acme-prod"}) {
		t.Fatalf("Scan(planted) = %v", hits)
	}
	all := []fs.FS{profiles.Core(), profiles.Builtin(), profiles.Features()}
	for _, fsys := range all {
		if hits := profiles.Scan(fsys, fixedDenylist); len(hits) > 0 {
			t.Errorf("fixed denylist: %v", hits)
		}
	}
	data, err := os.ReadFile("../.private/denylist.txt")
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip(".private/denylist.txt is absent (CI): the private scan is skipped")
	}
	if err != nil {
		t.Fatal(err)
	}
	var words []string
	for _, l := range strings.Split(string(data), "\n") {
		if l = strings.TrimSpace(l); l != "" && !strings.HasPrefix(l, "#") {
			words = append(words, l)
		}
	}
	for _, fsys := range all {
		if hits := profiles.Scan(fsys, words); len(hits) > 0 {
			t.Errorf("private denylist: %v", hits)
		}
	}
}
```

`internal/cli/testdata/script/builtin_program.txtar`:
```
# program renders alone with zero conflicts and a clean --check, adds the size, seeds the
# roadmap, the ledger and the brief template, exposes the autonomy and review-format slots,
# adds no command, and a 100 KB roadmap triggers nothing.
# covers: R-BLT-001, R-BLT-030, R-BLT-032, R-BLT-033, R-BLT-034, R-BLT-038, R-BLT-039, R-BLT-041, R-PROF-002
cd repo
exec ness init --gate true --base main --profile program --yes
! stdout conflict
exec ness sync --check
grep '"program": "0.1.0"' .ness/lock.json
grep '^name: ness-program$' .agents/skills/ness-program/SKILL.md
grep '^- \*\*program\*\*' .agents/skills/ness-entry/SKILL.md
grep '^## Invariants' docs/program/roadmap.md
grep '^## Verticals' docs/program/roadmap.md
grep '| Spec owner |' docs/program/roadmap.md
grep '^## Waves' docs/program/roadmap.md
grep '^## Conduct' docs/program/roadmap.md
grep '^## Releases' docs/program/roadmap.md
grep '^## Autonomy grant' docs/program/roadmap.md
grep '^Recovery line:' docs/program/roadmap.md
grep 'append-only' docs/program/decisions.md
grep 'D<n>' docs/program/decisions.md
exists docs/program/briefs/TEMPLATE.md
grep '^## Question' docs/program/briefs/TEMPLATE.md
grep '^## Outcome' docs/program/briefs/TEMPLATE.md
grep '^## Invariants' docs/program/briefs/TEMPLATE.md
grep '^## Operator words' docs/program/briefs/TEMPLATE.md
grep '^## Sources' docs/program/briefs/TEMPLATE.md
grep '^## Rights and limits' docs/program/briefs/TEMPLATE.md
grep '^## Tier' docs/program/briefs/TEMPLATE.md
grep '^## Spec owner' docs/program/briefs/TEMPLATE.md
grep '^## Integration target' docs/program/briefs/TEMPLATE.md
grep '^## How to report' docs/program/briefs/TEMPLATE.md
grep 'Review format: HTML page' .agents/skills/ness-program/SKILL.md
grep 'No grant recorded yet' .agents/skills/ness-program/SKILL.md
# The command list is the same sixteen.
exec sh -c 'ness --help | sed -n "/^Available Commands:/,/^$/p" | grep -E "^  [a-z]+ " | awk "{print \$1}" | sort | tr "\n" " "'
stdout '^close contract doctor hook inbox init integrate open post probe reply send stats status sync worker $'
# A local set replaces both single slots.
exec sh -c 'sed -i "s/^profiles = .*/profiles = [\"program\", \"local\"]/" .ness/ness.toml'
exec ness sync
grep 'Review format: Markdown file' .agents/skills/ness-program/SKILL.md
! grep 'HTML page' .agents/skills/ness-program/SKILL.md
grep 'may merge without asking' .agents/skills/ness-program/SKILL.md
# A 100 KB roadmap is guidance only.
exec sh -c 'head -c 102400 /dev/zero | tr "\0" x >> docs/program/roadmap.md'
exec ness sync
! stdout .
exec ness sync --check
exits 1 ness doctor
! stdout roadmap

-- repo/.ness/profiles/local/profile.toml --
name = "local"
[slots]
"ness-program#operator-review-format" = { set = "format.md" }
"ness-program#autonomy" = { set = "grant.md" }
-- repo/.ness/profiles/local/format.md --
Markdown file
-- repo/.ness/profiles/local/grant.md --
The root may merge without asking.
```

`internal/cli/testdata/script/builtin_git_conventional.txtar`:
```
# git-conventional renders the guidance and a commit-msg hook that rejects a non-conventional
# message once activated.
# covers: R-BLT-001, R-BLT-080, R-BLT-081
cd repo
exec ness init --gate true --base main --profile git-conventional --yes
exec ness sync --check
grep 'type(scope): subject' .agents/skills/ness-git-conventional/SKILL.md
grep '^## Trailers' .agents/skills/ness-git-conventional/SKILL.md
grep 'Co-Authored-By' .agents/skills/ness-git-conventional/SKILL.md
grep 'git config core.hooksPath .githooks' .agents/skills/ness-git-conventional/SKILL.md
exists .githooks/commit-msg
exec git config core.hooksPath .githooks
chmod 755 .githooks/commit-msg
exec git add -A
! exec git commit -q -m 'stuff'
stderr 'not a Conventional Commit'
exec git commit -q -m 'fix: x'
exec git commit -q --allow-empty -m 'sessions(worker): restart after sync'
exec git commit -q --allow-empty -m 'feat(cli)!: drop --model'
! exec git commit -q --allow-empty -m 'feat: '
! exec git commit -q --allow-empty -m 'fixes: x'
exec git commit -q --allow-empty -m 'Merge branch x'
```

`internal/cli/testdata/script/builtin_example_release.txtar`:
```
# example-release renders one chain as text: integrate, a post-push CI probe, notify; its probe
# parses and runs GREEN against a fixture gh; the text promotes with integrate --from/--into.
# covers: R-BLT-001, R-BLT-091, R-BLT-093
cd repo
exec ness init --gate true --base main --profile example-release --yes
exec ness sync --check
exists .agents/skills/ness-probes/SKILL.md
exists .agents/skills/ness-notify/SKILL.md
grep 'integrate --from main --into prod' .agents/skills/ness-example-release/SKILL.md
grep 'ness probe start probes/ci-after-push.md' .agents/skills/ness-example-release/SKILL.md
grep 'ness post --channel releases' .agents/skills/ness-example-release/SKILL.md
exists docs/release-notes/TEMPLATE.md
grep '^## Running sessions' docs/release-notes/TEMPLATE.md
mkdir ../bin
exec sh -c 'printf "#!/bin/sh\necho success\n" > ../bin/gh && chmod +x ../bin/gh'
env PATH=$WORK/bin:$PATH
exec ness probe run probes/ci-after-push.md
stdout '^GREEN$'
stdout 'conclusion=success'
exec sh -c 'printf "#!/bin/sh\necho failure\n" > ../bin/gh'
exits 1 ness probe run probes/ci-after-push.md
stdout '^RED$'
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./profiles/ ./internal/cli/ -run 'TestFiveBuiltins|TestSkills|TestDenylist|TestScripts/builtin_'`
Expected: FAIL (`profiles.Names` undefined; unknown profiles).

- [ ] **Step 3: `Names` and `Scan` in `profiles/embed.go`**

Append:
```go
// Names lists the built-in profiles, sorted (R-BLT-100).
func Names() ([]string, error) {
	entries, err := fs.ReadDir(Builtin(), ".")
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out, nil
}

// Scan reports every file of fsys containing one of words, case-insensitively, as
// "path: word" (R-BLT-003): the denylist check that keeps private names out of the built-ins.
func Scan(fsys fs.FS, words []string) []string {
	var hits []string
	_ = fs.WalkDir(fsys, ".", func(p string, d fs.DirEntry, err error) error { //nolint:errcheck // a read error is a miss, reported by the embed build
		if err != nil || d.IsDir() {
			return err
		}
		b, rerr := fs.ReadFile(fsys, p)
		if rerr != nil {
			return rerr
		}
		text := strings.ToLower(string(b))
		for _, w := range words {
			if strings.Contains(text, strings.ToLower(w)) {
				hits = append(hits, p+": "+w)
			}
		}
		return nil
	})
	return hits
}
```
(`strings` joins the imports. If `errcheck check-blank` still flags the `_ =`, write `if err := fs.WalkDir(...); err != nil { return append(hits, "walk: "+err.Error()) }` instead.) `fs.ReadDir` sorts by name, so `Names` is sorted; the `README.md` file is skipped because it is not a directory.

- [ ] **Step 4: The program profile**

`profiles/builtin/program/profile.toml`:
```toml
name    = "program"
summary = "A root that maps a program into verticals, runs waves of children and releases (design §10)"

[slots]
"ness-entry#sizes"                    = { append = "size.md" }
"ness-entry#post-compact"             = { append = "post-compact.md" }
"ness-program#autonomy"               = { set = "autonomy.md" }
"ness-program#operator-review-format" = { set = "review-format.md" }

[files]
".agents/skills/ness-program/SKILL.md" = { own = "skills/ness-program/SKILL.md" }
"docs/program/roadmap.md"              = { seed = "roadmap.md" }
"docs/program/decisions.md"            = { seed = "decisions.md" }
"docs/program/briefs/TEMPLATE.md"      = { seed = "brief-template.md" }
```

`size.md`: `- **program**: a root that owns a map (`docs/program/roadmap.md`), opens children in waves and releases; load the `ness-program` skill.` (one line, then a newline).

`post-compact.md`: `- Re-read `docs/program/roadmap.md` and the last decision id in `docs/program/decisions.md`.` (one line).

`autonomy.md`: `No grant recorded yet: ask the operator for one before the first wave, record it in the Autonomy grant section of `docs/program/roadmap.md`, and widen it only when the operator says so.` (one line).

`review-format.md`: `Review format: HTML page` (one line).

`skills/ness-program/SKILL.md`:
```markdown
---
name: ness-program
description: Run a program as a root: read back, map invariants and verticals, refine the map with the operator, run waves of children, fold results back, release.
---

# ness program

A program is too big for one session. The root owns the map; children own the verticals.

## The six steps
1. **Read back.** Survey with `ness-sweep` subagents, then restate the brief as roadmap r1
   in `docs/program/roadmap.md`: parts, dependencies, holes, questions. Minutes, not hours.
2. **Only invariants at the root**: what two or more verticals depend on, or what is
   expensive to reverse. Nothing else is decided at the root.
3. **Everything else is a vertical** with its reason, its reopen trigger and its owner.
4. **Breadth-first refinement** of the map with the operator until the operator accepts it.
5. **Waves** of the verticals whose inputs exist; each result folds back as a new map
   revision.
6. **Children integrate into the root branch; the root releases.**

## Rules
- Grounding (measurements, surveys, audits) runs in subagents; the root reads conclusions.
- A named operator preference (a tool, a vendor, a shape) becomes an explicit open decision in
  `docs/program/decisions.md`, never a thing built past.
- The root brings every child packet into its own chat as one-word answers the operator can
  give; the operator never reads child packets.
- Each vertical names its spec owner in the map; the root never accepts a spec the operator
  owns, it asks for it.
- Injected work becomes a map row before its child opens.
- A heartbeat line to the operator at each wave event: what started, what landed, what waits.
- "Since you left": when the operator returns, list the decision ids newer than the operator's
  last message, one line each.
- Ask only for what the root cannot get itself.
- Every fold and every new vertical is a new map revision, and the map ends with a recovery
  line naming the last decision id, so a compacted root can find its place.
- A verification or cutover phase is a map section with its own probes (`ness probe start`),
  never a root that watches a screen.
- Fixes to shared tooling go to base through a quick task, not through the root branch.

## Autonomy grant
<!-- ness:slot single autonomy -->

## Operator review format
<!-- ness:slot single operator-review-format -->
```
(A slot marker must start its line; the slot text is the whole line, so `review-format.md` is `Review format: HTML page` and the script's local override is `Review format: Markdown file`; the autonomy text is likewise a whole paragraph.)

`roadmap.md` (seed):
```markdown
# Roadmap r1

Revision r1. Every fold and every new vertical is a new revision (r2, r3, ...).

## Invariants
What two or more verticals depend on, or what is expensive to reverse. One line each, by id.

| Id | Invariant | Why it is one |
|---|---|---|
| I1 | | |

## Verticals
Everything else. The spec owner is who writes the vertical's spec: the root, the child, or
the operator (then the root asks for it and never writes it).

| Id | Vertical | Reason | Reopen trigger | Owner | Spec owner | State |
|---|---|---|---|---|---|---|
| V1 | | | | | | open |

## Waves
A wave is the set of verticals whose inputs exist. Each result folds back as a new revision.

| Wave | Verticals | Started | Landed |
|---|---|---|---|
| W1 | | | |

## Conduct
Executor per vertical (child session or subagent) and why; order; concurrency cap.

## Releases
Unit of deploy; dormant versus activation; who gives the go.

## Autonomy grant
Recorded once, widened only by the operator. Decision ids: 

Recovery line: last decision id D0; last wave event none.
```

`decisions.md` (seed):
```markdown
# Decisions

Append-only ledger with stable ids. Each entry: `- **D<n>** (date) decision; who decided; why.`
Ids are never reused or renumbered; a superseded decision stays and the newer entry names it.

- **D1** (YYYY-MM-DD) The program opened; the map is r1.
```

`brief-template.md` (seed): one `## ` heading per slot of R-BLT-034, in this order, each followed by one line of guidance: `## Question`, `## Outcome`, `## Invariants` (by id), `## Operator words` (verbatim, by decision id), `## Sources`, `## Rights and limits`, `## Tier`, `## Spec owner`, `## Integration target`, `## How to report`. Start the file with `# Brief: <vertical id> <title>` and the line `Copy this file to `briefs/<vertical>.md`; a brief does not promote side remarks to rules.`

- [ ] **Step 5: The git-conventional profile**

`profiles/builtin/git-conventional/profile.toml`:
```toml
name    = "git-conventional"
summary = "Conventional Commits with trailers, checked by a commit-msg hook"

[files]
".agents/skills/ness-git-conventional/SKILL.md" = { own = "skills/ness-git-conventional/SKILL.md" }
".githooks/commit-msg"                           = { own = "commit-msg" }
```

`commit-msg`:
```sh
#!/bin/sh
# Conventional Commits check, rendered by the ness git-conventional profile.
# Usage: commit-msg FILE (git passes the message file). Activate once per clone:
#   git config core.hooksPath .githooks && chmod +x .githooks/commit-msg
subject=$(sed -n '1p' "$1")
case "$subject" in
  "Merge "*|"Revert "*|"fixup! "*|"squash! "*) exit 0 ;;
esac
types='build|chore|ci|docs|feat|fix|perf|refactor|revert|sessions|style|test'
if printf '%s\n' "$subject" | grep -Eq "^($types)(\([^)]+\))?!?: [^ ].*"; then
  exit 0
fi
echo "commit-msg: \"$subject\" is not a Conventional Commit: type(scope): subject, with type one of $types" >&2
exit 1
```

`skills/ness-git-conventional/SKILL.md`:
```markdown
---
name: ness-git-conventional
description: Write commits in Conventional Commits form with trailers; the commit-msg hook rejects anything else.
---

# ness git-conventional

## Subject
`type(scope): subject` — the type is one of `build`, `chore`, `ci`, `docs`, `feat`, `fix`,
`perf`, `refactor`, `revert`, `sessions`, `style`, `test`; the scope is optional and names the
package or area; `!` before the colon marks a breaking change; the subject is imperative and
has no trailing period. `sessions` is what a running session must do after the change lands
(release notes put it under "Running sessions").

## Body
A blank line, then why the change is made, not what the diff shows.

## Trailers
After a blank line, one per line, `Key: value`: `Co-Authored-By: Name <email>`, `Refs: #123`,
`BREAKING CHANGE: what breaks and how to migrate`. Nothing follows the trailers.

## The hook
`.githooks/commit-msg` rejects a subject that is not in this form (merge, revert, fixup and
squash commits pass). Activate it once per clone:
`git config core.hooksPath .githooks && chmod +x .githooks/commit-msg`. With lefthook (the
go-strict profile) run it from `lefthook-local.yml` instead:
`commit-msg: { commands: { conventional: { run: .githooks/commit-msg {1} } } }`.
```

- [ ] **Step 6: The example-release profile**

`profiles/builtin/example-release/profile.toml`:
```toml
name    = "example-release"
summary = "One complete release chain as profile text: integrate, a post-push CI probe, notify (design §8, D12)"

[features]
enable = ["probes", "notify"]

[files]
".agents/skills/ness-example-release/SKILL.md" = { own = "skills/ness-example-release/SKILL.md" }
"probes/ci-after-push.md"                      = { seed = "ci-after-push.md" }
"docs/release-notes/TEMPLATE.md"               = { seed = "release-notes-template.md" }
```

`ci-after-push.md`:
```
---
every: 5m
window: 1h
floor: c=$(gh run list --branch "$(git rev-parse --abbrev-ref HEAD)" --limit 1 --json conclusion --jq '.[0].conclusion'); echo "conclusion=$c"; test "$c" = success
confirm: 3
wake: red
notify: judged
channel: releases
---
The CI run of the commit just pushed is green. When woken: open the failing job, decide
whether the failure is the change or the pipeline, and answer with `ness probe verdict`.
```

`release-notes-template.md`:
```markdown
# Release <version> (<date>)

## Changes
- one line per change, user-facing words

## Running sessions
- what an open session must do after this version lands (or "nothing")

## Verification
- the probe that watched the push and its verdict
```

`skills/ness-example-release/SKILL.md`:
```markdown
---
name: ness-example-release
description: An example of one release chain written as profile text: integrate, then a post-push CI probe, then a note to the releases channel, then promotion with integrate --from/--into.
---

# ness example-release

ness does not chain integrate, probes and notify; a profile says how a project chains them.
This is one way, with generic names: a `releases` channel in `[notify.channels]`, a `main`
branch for development and a `prod` branch for production.

1. **Integrate.** `ness integrate` folds your branch through the gate into your manager's
   branch and pushes it.
2. **Watch the push.** `ness probe start probes/ci-after-push.md` registers the probe: every
   5 minutes for an hour its floor asks the forge for the latest run on your branch; three
   consecutive non-green runs wake you with the evidence; your verdict is posted to
   `releases` because the probe says `notify: judged`.
3. **Tell people.** Copy `docs/release-notes/TEMPLATE.md` to
   `docs/release-notes/<version>.md`, fill it, and send it with
   `ness post --channel releases --file docs/release-notes/<version>.md --thread release-<version>`
   after a `--dry-run`.
4. **Promote.** When the operator says go, `ness integrate --from main --into prod` folds the
   development branch into production through the same gate; the same probe, started on
   `prod`, watches that push.
```

- [ ] **Step 7: Housekeeping files**

`profiles/builtin/README.md`: `One directory per built-in profile, each with a profile.toml (R-PROF-002): program, go-strict, behavior-specs, git-conventional, example-release. The text contains no project specifics (R-BLT-003): .private/denylist.txt, when present, is scanned over every file here by profiles_test.go.`

`.gitignore`: append the line `.private/` after the managed block.

`docs/spec/profiles-builtin.md`: rename the example profile's heading to `## `example-release` (D12, D27; renamed by P6)` and use `example-release` in R-BLT-090; the old name must not remain in the file. Nothing else in the spec changes.

- [ ] **Step 8: Run the tests**

Run: `go test ./profiles/ ./internal/cli/ -run 'TestFiveBuiltins|TestSkills|TestDenylist|TestScripts/builtin_' && golangci-lint run ./profiles/`
Expected: PASS. The go-strict and behavior-specs directories do not exist yet, so `TestFiveBuiltins` fails until Task 3: run it with `-run 'TestSkills|TestDenylist'` now and note that `TestFiveBuiltins` passes in Task 3.

- [ ] **Step 9: Commit**

```bash
git add profiles .gitignore docs/spec/profiles-builtin.md internal/cli/testdata
git commit -m "feat(profiles): built-ins program, git-conventional and example-release; the denylist scan"
```

---
### Task 3: go-strict and behavior-specs; the compliance audit; ness applies go-strict to itself

**Files:**
- Create: `profiles/builtin/go-strict/{profile.toml,golangci.yml,golangci.tests.yml,lefthook.yml,Makefile,audit.py,audit.toml,audit-exceptions.txt}`, `profiles/builtin/go-strict/slots/{module-path,go-version,package-roots,composition-roots,generated-dirs,excluded-packages,exceptions-file,ceilings,libraries}.md`, `profiles/builtin/go-strict/lifecycle/{specify,plan,handoff}.md`, `profiles/builtin/go-strict/skills/ness-{interface-design,test-strategy,test-discipline,documentation,compliance-check}/SKILL.md`, `profiles/builtin/behavior-specs/{profile.toml,specify.md,skills/ness-behavior-specs/SKILL.md}`, `internal/cli/testdata/script/builtin_go_strict.txtar`, `builtin_behavior_specs.txtar`, `builtin_all.txtar`, `.ness/profiles/local/{profile.toml,module-path.md,go-version.md,composition-roots.md}`
- Modify: `internal/testkit/testkit.go` (GOCACHE pass-through), `mise.toml` (python), `Makefile` (audit target), `.ness/ness.toml` (profiles), and the files go-strict renders into this repository

**Interfaces:**
- Consumes: the lifecycle slots of Task 1.
- Produces: the audit's exit codes (0 clean, 1 findings, 2 a bad config or exceptions file), its finding line `RULE path: detail` on stdout and refusals on stderr.

- [ ] **Step 1: Write the failing scripts**

`internal/cli/testdata/script/builtin_go_strict.txtar`:
```
# go-strict renders the five skills, the lint configs, lefthook, the Makefile and the audit;
# every slot renders into its file; the configs verify; the audit and the test lint reject the
# fixtures the spec names; the shadowing check passes; the gate's steps include -count=4.
# covers: R-BLT-001, R-BLT-002, R-BLT-050, R-BLT-053, R-BLT-054, R-BLT-055, R-BLT-057, R-BLT-058, R-BLT-059, R-BLT-060, R-BLT-061, R-BLT-071, R-QA-041
[short] skip 'go-strict renders a Go module and lints it'
[!exec:golangci-lint] skip 'golangci-lint is not on PATH'
[!exec:python3] skip 'python3 is not on PATH'
cd repo
cp ../go.mod go.mod
mkdir internal/p cmd/p
cp ../p.go internal/p/p.go
cp ../main.go cmd/p/main.go
exec ness init --gate 'make validate' --base main --profile go-strict --yes
! stdout conflict
exec ness sync --check
exists .golangci.yml .golangci.tests.yml lefthook.yml Makefile .ness/audit.py .ness/audit.toml .ness/audit-exceptions.txt
exists .agents/skills/ness-interface-design/SKILL.md
exists .agents/skills/ness-test-strategy/SKILL.md
exists .agents/skills/ness-test-discipline/SKILL.md
exists .agents/skills/ness-documentation/SKILL.md
exists .agents/skills/ness-compliance-check/SKILL.md
grep 'local-prefixes: \[\]' .golangci.yml
grep '^  go: "1.25"$' .golangci.yml
grep '^test_file_lines = 400$' .ness/audit.toml
grep '^  - invoke /ness-test-strategy$' .agents/skills/ness-entry/SKILL.md
grep '^  - invoke /ness-interface-design$' .agents/skills/ness-entry/SKILL.md
grep '^  - invoke /ness-compliance-check$' .agents/skills/ness-entry/SKILL.md
grep '^  - invoke /ness-documentation$' .agents/skills/ness-entry/SKILL.md
! grep 'ness:slot' .golangci.yml
! grep 'ness:slot' .ness/audit.toml
exec golangci-lint config verify
exec golangci-lint config verify -c .golangci.tests.yml
exec make -n validate
stdout 'golangci-lint run ./\.\.\.'
stdout 'golangci-lint run -c .golangci.tests.yml'
stdout 'go test -count=4'
stdout 'python3 .ness/audit.py'
stdout 'ness sync --check'
exits 1 ness doctor
stdout '^ok\tskill-shadowing\t'
stdout '^ok\tprofile-tools\t2 required tools present'
# The audit: clean fixture passes; a 400-line test file fails; an owned exception passes; an
# exception without a reason is refused.
exec python3 .ness/audit.py
exec sh -c 'printf "package p\n" > internal/p/big_test.go; yes "// x" | head -399 >> internal/p/big_test.go'
exits 1 python3 .ness/audit.py
stdout '^T400 internal/p/big_test.go: 400 lines'
exec sh -c 'printf "T400 internal/p/big_test.go legacy suite, split when touched\n" >> .ness/audit-exceptions.txt'
exec python3 .ness/audit.py
exec sh -c 'printf "T400 internal/p/other_test.go\n" >> .ness/audit-exceptions.txt'
exits 2 python3 .ness/audit.py
stderr 'exception without a reason'
exec sh -c 'sed -i "/other_test/d" .ness/audit-exceptions.txt'
rm internal/p/big_test.go
# ForTest and a Clock interface are findings; a logger other than the configured one too.
cp ../fortest.go internal/p/fortest.go
exits 1 python3 .ness/audit.py
stdout '^FORTEST internal/p/fortest.go'
stdout '^CLOCK internal/p/fortest.go'
stdout '^LIBS internal/p/fortest.go: go.uber.org/zap'
rm internal/p/fortest.go
# Two When in a scenario, more than 10 steps, more than 5 example rows.
mkdir features
cp ../two_when.feature features/x.feature
exits 1 python3 .ness/audit.py
stdout '^FEATURE features/x.feature: scenario "two" has 2 When'
rm features/x.feature
# The test lint: time.Sleep fails it.
cp ../sleep_test.go internal/p/sleep_test.go
exits 1 golangci-lint run -c .golangci.tests.yml ./...
stdout 'time.Sleep'
rm internal/p/sleep_test.go
# documentation: an export without a doc comment fails lint; the clean fixture passes.
cp ../undoc.go internal/p/undoc.go
exits 1 golangci-lint run ./...
stdout 'exported'
rm internal/p/undoc.go
exec golangci-lint run ./...
# Slots render into the files; a Go version below 1.25 is refused by the audit.
exec sh -c 'sed -i "s/^profiles = .*/profiles = [\"go-strict\", \"local\"]/" .ness/ness.toml'
exec ness sync
grep 'local-prefixes: \[example.com/p\]' .golangci.yml
grep '^  go: "1.24"$' .golangci.yml
grep '^test_file_lines = 300$' .ness/audit.toml
grep '^composition_roots = \["cmd", "internal/wire"\]$' .ness/audit.toml
exits 2 python3 .ness/audit.py
stderr 'go version 1.24 is below 1.25'

-- go.mod --
module example.com/p

go 1.27
-- p.go --
// Package p is the fixture package of the go-strict script.
package p

// Answer is the fixture export.
func Answer() int { return 42 }
-- main.go --
// Command p is the fixture composition root.
package main

import "example.com/p/internal/p"

func main() { _ = p.Answer() }
-- fortest.go --
package p

import "go.uber.org/zap"

// Clock is what the audit refuses: pass now as a value.
type Clock interface{ Now() int }

// ResetForTest is a test hook in production code.
func ResetForTest() { _ = zap.String }
-- two_when.feature --
Feature: x
  Scenario: two
    Given a
    When b
    When c
    Then d
-- sleep_test.go --
package p

import (
	"testing"
	"time"
)

func TestSleep(t *testing.T) { time.Sleep(time.Millisecond) }
-- undoc.go --
package p

func Undocumented() {}
-- repo/.ness/profiles/local/profile.toml --
name = "local"
[slots]
".golangci.yml#module-path" = { set = "module-path.md" }
".golangci.yml#go-version" = { set = "go-version.md" }
".ness/audit.toml#ceilings" = { set = "ceilings.md" }
".ness/audit.toml#composition-roots" = { set = "roots.md" }
-- repo/.ness/profiles/local/module-path.md --
      local-prefixes: [example.com/p]
-- repo/.ness/profiles/local/go-version.md --
  go: "1.24"
-- repo/.ness/profiles/local/ceilings.md --
[ceilings]
test_file_lines = 300
-- repo/.ness/profiles/local/roots.md --
composition_roots = ["cmd", "internal/wire"]
```
(The `fortest.go` fixture imports `go.uber.org/zap`, which is not in the fixture module: the audit reads the import line, it never builds the file, and the file is removed before `golangci-lint run ./...`.)

`internal/cli/testdata/script/builtin_behavior_specs.txtar`:
```
# behavior-specs requires go-strict (refused alone, naming both), enables contracts and renders
# its skill and the specify invoke.
# covers: R-BLT-001, R-BLT-070, R-BLT-072
[!exec:golangci-lint] skip 'go-strict needs golangci-lint on PATH'
[!exec:python3] skip 'go-strict needs python3 on PATH'
cd repo
snap before
exits 4 ness init --gate true --base main --profile behavior-specs --yes
stderr 'behavior-specs'
stderr 'go-strict'
samesnap before
exec ness init --gate true --base main --profile go-strict --profile behavior-specs --yes
exec ness sync --check
grep '^name: ness-behavior-specs$' .agents/skills/ness-behavior-specs/SKILL.md
grep 'one `When`' .agents/skills/ness-behavior-specs/SKILL.md
grep 'ness contract freeze' .agents/skills/ness-behavior-specs/SKILL.md
exists .agents/skills/ness-contracts/SKILL.md
grep '^  - invoke /ness-behavior-specs$' .agents/skills/ness-entry/SKILL.md
```

`internal/cli/testdata/script/builtin_all.txtar`:
```
# All five built-ins render together without conflict, --check is clean, no skill shadows a
# global one, and the command list is unchanged.
# covers: R-BLT-005, R-BLT-002, R-BLT-039
[!exec:golangci-lint] skip 'go-strict needs golangci-lint on PATH'
[!exec:python3] skip 'go-strict needs python3 on PATH'
cd repo
exec ness init --gate true --base main --profile program --profile go-strict --profile behavior-specs --profile git-conventional --profile example-release --yes
! stdout conflict
exec ness sync --check
exec sh -c 'ls -d .agents/skills/ness-* | wc -l'
stdout '^14$'
exits 1 ness doctor
stdout '^ok\tskill-shadowing\t'
exec sh -c 'ness --help | sed -n "/^Available Commands:/,/^$/p" | grep -E "^  [a-z]+ " | awk "{print \$1}" | sort | tr "\n" " "'
stdout '^close contract doctor hook inbox init integrate open post probe reply send stats status sync worker $'
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/cli/ -run 'TestScripts/builtin_(go_strict|behavior_specs|all)'`
Expected: FAIL (unknown profile `go-strict`).

- [ ] **Step 3: The go cache pass-through in `internal/testkit/testkit.go`**

golangci-lint in a fixture module compiles the standard library once per cache; the script's `HOME` is inside `$WORK`, so without this every run starts cold. Add to `Setup`, after the `NESS_TEST_FAKES` line:
```go
	for _, k := range []string{"GOCACHE", "GOMODCACHE"} {
		e.Setenv(k, goEnv(k))
	}
```
and:
```go
var goEnvOnce sync.Map // variable -> value, from one `go env` per process

// goEnv is the outer process's Go cache locations, so a script's lint and build reuse the
// developer's caches instead of filling a cold one under $WORK.
func goEnv(name string) string {
	if v, ok := goEnvOnce.Load(name); ok {
		return v.(string) //nolint:forcetypeassert // only strings are stored
	}
	v := os.Getenv(name)
	if v == "" {
		out, err := exec.CommandContext(context.Background(), "go", "env", name).Output()
		if err == nil {
			v = strings.TrimSpace(string(out))
		}
	}
	goEnvOnce.Store(name, v)
	return v
}
```
(`sync` joins the imports.)

- [ ] **Step 4: The go-strict profile**

`profiles/builtin/go-strict/profile.toml`:
```toml
name    = "go-strict"
summary = "Strict Go quality (design §11, D2, D20): lint, test lint, the compliance audit, lefthook, make validate"
requires_tools = ["python3", "golangci-lint"]

[slots]
".golangci.yml#module-path"          = { set = "slots/module-path.md" }
".golangci.yml#go-version"           = { set = "slots/go-version.md" }
".ness/audit.toml#package-roots"     = { set = "slots/package-roots.md" }
".ness/audit.toml#composition-roots" = { set = "slots/composition-roots.md" }
".ness/audit.toml#generated-dirs"    = { set = "slots/generated-dirs.md" }
".ness/audit.toml#excluded-packages" = { set = "slots/excluded-packages.md" }
".ness/audit.toml#exceptions-file"   = { set = "slots/exceptions-file.md" }
".ness/audit.toml#ceilings"          = { set = "slots/ceilings.md" }
".ness/audit.toml#libraries"         = { set = "slots/libraries.md" }
"ness-entry#specify"                 = { append = "lifecycle/specify.md" }
"ness-entry#plan"                    = { append = "lifecycle/plan.md" }
"ness-entry#handoff"                 = { append = "lifecycle/handoff.md" }

[files]
".golangci.yml"               = { own = "golangci.yml" }
".golangci.tests.yml"         = { own = "golangci.tests.yml" }
"lefthook.yml"                = { own = "lefthook.yml" }
".ness/audit.py"              = { own = "audit.py" }
".ness/audit.toml"            = { own = "audit.toml" }
".ness/audit-exceptions.txt"  = { seed = "audit-exceptions.txt" }
"Makefile"                    = { seed = "Makefile" }
".agents/skills/ness-interface-design/SKILL.md"  = { own = "skills/ness-interface-design/SKILL.md" }
".agents/skills/ness-test-strategy/SKILL.md"     = { own = "skills/ness-test-strategy/SKILL.md" }
".agents/skills/ness-test-discipline/SKILL.md"   = { own = "skills/ness-test-discipline/SKILL.md" }
".agents/skills/ness-documentation/SKILL.md"     = { own = "skills/ness-documentation/SKILL.md" }
".agents/skills/ness-compliance-check/SKILL.md"  = { own = "skills/ness-compliance-check/SKILL.md" }
```

Slot default files (each exactly the line(s) shown, newline-terminated; the slot text replaces the marker line in the target):
- `slots/module-path.md`: `      local-prefixes: []`
- `slots/go-version.md`: `  go: "1.25"`
- `slots/package-roots.md`: `package_roots = ["internal", "cmd"]`
- `slots/composition-roots.md`: `composition_roots = ["cmd"]`
- `slots/generated-dirs.md`: `generated_dirs = []`
- `slots/excluded-packages.md`: `excluded_packages = []`
- `slots/exceptions-file.md`: `exceptions_file = ".ness/audit-exceptions.txt"`
- `slots/ceilings.md`: two lines `[ceilings]` and `test_file_lines = 400`
- `slots/libraries.md`: three lines `[libraries]`, `logger = "log/slog"`, `validator = ""`
- `lifecycle/specify.md`: two lines `  - invoke /ness-test-strategy` and `  - invoke /ness-interface-design`
- `lifecycle/plan.md`: `  - invoke /ness-compliance-check`
- `lifecycle/handoff.md`: `  - invoke /ness-documentation`

`golangci.yml`: this repository's current `.golangci.yml` byte for byte, with two edits: under `run:` insert the line `<!-- ness:slot single go-version -->` after `modules-download-mode: readonly`, and replace the `local-prefixes: [github.com/enea-scaccabarozzi/ness]` line under `goimports:` with `<!-- ness:slot single module-path -->`. (Markers sit at column 0; the slot text carries its own indentation.)

`golangci.tests.yml`: this repository's `.golangci.tests.yml` byte for byte.

`lefthook.yml`:
```yaml
# go-strict: git hooks with failure-only output. Install once per clone: lefthook install
skip_output: [meta, summary, success]
pre-commit:
  commands:
    lint:
      glob: "*.go"
      run: golangci-lint run ./... && golangci-lint run -c .golangci.tests.yml ./...
pre-push:
  commands:
    validate:
      run: make validate
```

`Makefile` (seed):
```make
.PHONY: validate build lint test audit vuln selfcheck

# The gate (go-strict): every step must pass. Run it before every push.
validate: build lint test audit vuln selfcheck

build:
	CGO_ENABLED=0 go build ./...

lint:
	golangci-lint run ./...
	golangci-lint run -c .golangci.tests.yml ./...

test:
	go test -race -count=1 ./...
	go test -count=4 ./...

audit:
	python3 .ness/audit.py

vuln:
	go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...

selfcheck:
	ness sync --check
```

`audit.toml`:
```toml
# go-strict compliance audit configuration, rendered by ness sync (R-BLT-060). Change a value
# through a slot of your local profile, for example in .ness/profiles/local/profile.toml:
#   ".ness/audit.toml#ceilings" = { set = "ceilings.md" }
<!-- ness:slot single package-roots -->
<!-- ness:slot single composition-roots -->
<!-- ness:slot single generated-dirs -->
<!-- ness:slot single excluded-packages -->
<!-- ness:slot single exceptions-file -->
<!-- ness:slot single ceilings -->
<!-- ness:slot single libraries -->
```

`audit-exceptions.txt` (seed):
```
# go-strict audit exceptions: RULE PATH REASON, one per line. A line without a reason is
# refused. Rules: T400 SLEEP FORTEST CLOCK PKGDOC LIBS FEATURE.
```

`audit.py`:
```python
#!/usr/bin/env python3
"""go-strict compliance audit (ness, design §11, R-BLT-053, R-BLT-057, R-BLT-060, R-BLT-071).

Exit 0: clean. Exit 1: findings, one line each on stdout: RULE PATH: DETAIL. Exit 2: the
configuration, the exceptions file or the Go version is unusable (stderr).
Rules: T400 (a test file at or over the ceiling), SLEEP (time.Sleep in a test), FORTEST (a
ForTest identifier in production code), CLOCK (a Clock interface), PKGDOC (not exactly one
package or command comment per package), LIBS (a logger or validator other than the configured one
outside the composition roots), FEATURE (a godog scenario with more than one When, more than
10 steps or more than 5 example rows). The judgment rules of the skills stay with the agent.
"""
import re
import sys
import tomllib
from pathlib import Path

CONFIG = Path(".ness/audit.toml")
MIN_GO = (1, 25)
LOGGERS = ["log/slog", "github.com/sirupsen/logrus", "go.uber.org/zap", "github.com/rs/zerolog"]
VALIDATORS = ["github.com/go-playground/validator", "github.com/go-ozzo/ozzo-validation"]


def fail(msg):
    print(f"audit: {msg}", file=sys.stderr)
    sys.exit(2)


def load_config():
    try:
        cfg = tomllib.loads(CONFIG.read_text())
    except (OSError, tomllib.TOMLDecodeError) as e:
        fail(f"{CONFIG}: {e}")
    cfg.setdefault("package_roots", ["internal", "cmd"])
    cfg.setdefault("composition_roots", ["cmd"])
    cfg.setdefault("generated_dirs", [])
    cfg.setdefault("excluded_packages", [])
    cfg.setdefault("exceptions_file", ".ness/audit-exceptions.txt")
    cfg.setdefault("ceilings", {}).setdefault("test_file_lines", 400)
    cfg.setdefault("libraries", {}).setdefault("logger", "log/slog")
    cfg["libraries"].setdefault("validator", "")
    return cfg


def check_go_version():
    m = re.search(r'^\s*go:\s*"?(\d+)\.(\d+)', Path(".golangci.yml").read_text(), re.M)
    if m and (int(m[1]), int(m[2])) < MIN_GO:
        fail(f"go version {m[1]}.{m[2]} is below {MIN_GO[0]}.{MIN_GO[1]} (go-strict floor)")


def load_exceptions(path):
    out = set()
    for n, line in enumerate(Path(path).read_text().splitlines(), 1) if Path(path).exists() else []:
        line = line.strip()
        if not line or line.startswith("#"):
            continue
        parts = line.split(None, 2)
        if len(parts) < 3:
            fail(f"{path}:{n}: exception without a reason: RULE PATH REASON")
        out.add((parts[0], parts[1]))
    return out


def skipped(path, cfg):
    s = str(path)
    return any(s.startswith(d) for d in cfg["generated_dirs"] + cfg["excluded_packages"])


def go_files(cfg):
    for root in cfg["package_roots"]:
        for p in sorted(Path(root).rglob("*.go")):
            if not skipped(p, cfg):
                yield p


def in_composition_root(path, cfg):
    return any(str(path).startswith(r.rstrip("/") + "/") for r in cfg["composition_roots"])


def check_file(path, text, cfg, out):
    lines = text.splitlines()
    if path.name.endswith("_test.go"):
        if len(lines) >= cfg["ceilings"]["test_file_lines"]:
            out.append(("T400", str(path), f"{len(lines)} lines (ceiling {cfg['ceilings']['test_file_lines']})"))
        if "time.Sleep(" in text:
            out.append(("SLEEP", str(path), "time.Sleep in a test"))
        return
    for n, line in enumerate(lines, 1):
        if re.search(r"\b\w*ForTest\w*\b", line):
            out.append(("FORTEST", str(path), f"line {n}: test hook in production code"))
        if re.match(r"^type\s+Clock\s+interface", line):
            out.append(("CLOCK", str(path), f"line {n}: pass now as a value, use synctest for timers"))
    if not in_composition_root(path, cfg):
        check_libs(path, text, cfg, out)


def check_libs(path, text, cfg, out):
    libs = cfg["libraries"]
    for imp in re.findall(r'^\s*(?:\w+\s+)?"([^"]+)"', text, re.M):
        if imp in LOGGERS and imp != libs["logger"]:
            out.append(("LIBS", str(path), f"{imp}: the logger is {libs['logger']}"))
        if imp in VALIDATORS and imp != libs["validator"]:
            out.append(("LIBS", str(path), f"{imp}: the validator is {libs['validator'] or 'none'}"))


def check_packages(docs, out):
    for pkg, n in sorted(docs.items()):
        if n != 1:
            out.append(("PKGDOC", pkg, f"{n} package comments, want exactly one"))


def check_feature(path, text, out):
    name, whens, steps, rows, in_examples = None, 0, 0, 0, False

    def flush():
        if name is None:
            return
        if whens > 1:
            out.append(("FEATURE", str(path), f'scenario "{name}" has {whens} When'))
        if steps > 10:
            out.append(("FEATURE", str(path), f'scenario "{name}" has {steps} steps (max 10)'))
        if rows > 5:
            out.append(("FEATURE", str(path), f'scenario "{name}" has {rows} example rows (max 5)'))

    for line in text.splitlines():
        s = line.strip()
        if s.startswith(("Scenario:", "Scenario Outline:")):
            flush()
            name, whens, steps, rows, in_examples = s.split(":", 1)[1].strip(), 0, 0, 0, False
        elif s.startswith("Examples:"):
            in_examples, rows = True, -1
        elif s.startswith("|") and in_examples:
            rows += 1
        elif s.split(" ")[0] in ("Given", "When", "Then", "And", "But"):
            steps += 1
            whens += s.startswith("When")
    flush()


def main():
    cfg = load_config()
    check_go_version()
    exceptions = load_exceptions(cfg["exceptions_file"])
    out, docs = [], {}
    for p in go_files(cfg):
        text = p.read_text(errors="replace")
        check_file(p, text, cfg, out)
        if not p.name.endswith("_test.go"):
            docs[str(p.parent)] = docs.get(str(p.parent), 0) + bool(re.search(r"^// (Package|Command) \w", text, re.M))
    check_packages(docs, out)
    for p in sorted(Path(".").rglob("*.feature")):
        if not skipped(p, cfg):
            check_feature(p, p.read_text(errors="replace"), out)
    findings = [f for f in out if (f[0], f[1]) not in exceptions]
    for rule, path, detail in findings:
        print(f"{rule} {path}: {detail}")
    sys.exit(1 if findings else 0)


if __name__ == "__main__":
    main()
```
(`load_exceptions` has a conditional expression in its `for` header; if python flags it, write the `if not exists: return set()` guard first. The script runs from the repository root, where `make` runs it.)

Skill texts, each with frontmatter `name: ness-<skill>` and a one-line description, under `## ` headings:
- `ness-interface-design`: consumer-declared interfaces of at most three methods, declared where they are faked; injected collaborators are built only in the composition roots (`cmd/`); outcomes travel through ports, never through shared state; no `ForTest` identifiers or test hooks in production code; `now` is a value passed in, timers are tested with `testing/synctest`, there is no `Clock` interface; no package-level mutable state; every package has a one-sentence role in its package comment.
- `ness-test-strategy`: the implementer does not write the judge (a reviewer or a separate task writes the acceptance test); the contract comes first, in the consumer's words; tests are black-box (`package x_test`); fakes are hand-written, never generated mocks; before keeping a test, answer "which wrong implementation would this reject?".
- `ness-test-discipline`: no `time.Sleep`; no permissive mocks and no call-count assertions; helpers call `t.Helper()`; fixtures live in `testdata/`; test files stay under the ceiling in `.ness/audit.toml` (400 lines); the gate runs `-race` and `-count=4`; the rules are enforced by `.golangci.tests.yml` and `python3 .ness/audit.py`.
- `ness-documentation`: the code is authoritative; a godoc comment on every export, starting with its name (revive); every comment ends with a period (godot); exactly one package comment per package (audit); the README floor: a section "Architectural Role" naming what the package is for and what it is not.
- `ness-compliance-check`: at plan and at the final review run `python3 .ness/audit.py` over the touched packages (`package_roots` in `.ness/audit.toml`); every MUST is fixed or owned; an owned finding goes to `.ness/audit-exceptions.txt` as `RULE PATH REASON`; the gate (`make validate`) runs the audit over the whole tree, the one scope the gate docs state.

- [ ] **Step 5: The behavior-specs profile**

`profiles/builtin/behavior-specs/profile.toml`:
```toml
name     = "behavior-specs"
summary  = "Declarative godog scenarios with a shared step vocabulary, frozen with contracts (design §11)"
requires = ["go-strict"]

[features]
enable = ["contracts"]

[slots]
"ness-entry#specify" = { append = "specify.md" }

[files]
".agents/skills/ness-behavior-specs/SKILL.md" = { own = "skills/ness-behavior-specs/SKILL.md" }
```
`specify.md`: `  - invoke /ness-behavior-specs`.

`skills/ness-behavior-specs/SKILL.md`: frontmatter plus: scenarios are declarative (what, not how); each scenario has exactly one `When`, at most 10 steps and at most 5 example rows (the audit's FEATURE rule); steps come from one shared vocabulary file per feature directory (`features/steps.md`), never invented per scenario; runners are strict (`godog` with `--strict`: undefined or pending steps fail); once the operator accepts a feature file, freeze it with `ness contract freeze features/<name>.feature` and change it only through `ness contract amend` with a reason.

- [ ] **Step 6: Run the scripts and the profile tests**

Run: `go test ./profiles/ && go test ./internal/cli/ -run 'TestScripts/builtin_'`
Expected: PASS (`TestFiveBuiltins` now passes). If `golangci-lint config verify` rejects `run.go`, drop the `go-version` slot from `.golangci.yml` and put `  go: "1.25"`-style text nowhere: instead read the floor from `go.mod`'s `go` directive in `check_go_version` and make the `go-version` slot a line of `.ness/audit.toml` (`go_version = "1.25"`); keep the script's expectations in step.

- [ ] **Step 7: ness applies go-strict to itself**

1. `mise.toml`: add `python = "3.13"` under `[tools]`.
2. `.ness/profiles/local/profile.toml`:
```toml
name = "local"
summary = "ness's own go-strict values"

[slots]
".golangci.yml#module-path"          = { set = "module-path.md" }
".golangci.yml#go-version"           = { set = "go-version.md" }
".ness/audit.toml#composition-roots" = { set = "composition-roots.md" }
```
with `module-path.md` = `      local-prefixes: [github.com/enea-scaccabarozzi/ness]`, `go-version.md` = `  go: "1.27"`, `composition-roots.md` = `composition_roots = ["cmd", "internal/cli"]`.
3. `.ness/ness.toml`: add `profiles = ["go-strict", "local"]`.
4. `go run ./cmd/ness sync`. Expected: `adopt .golangci.yml`, `adopt .golangci.tests.yml`, `adopt Makefile`, `create lefthook.yml`, `create .ness/audit.py`, `create .ness/audit.toml`, `create .ness/audit-exceptions.txt`, five `create .agents/skills/ness-...`, `update .agents/skills/ness-entry/SKILL.md`. If `.golangci.yml` is reported as a conflict, `diff .golangci.yml .golangci.yml.ness-new`: the profile is the source of truth, so `mv .golangci.yml.ness-new .golangci.yml` and sync again (it adopts).
5. `.ness/audit-exceptions.txt`: append
```
T400 internal/worker/worker_test.go P4 suite; split when touched
T400 internal/session/launch_test.go P3 suite; split when touched
T400 internal/cli/e2e_test.go real-client suite behind the e2e tag; split when touched
```
6. `Makefile`: `validate: build lint test audit vuln selfcheck` and the target `audit:` / `\tpython3 .ness/audit.py`.
7. Run: `make validate`. Expected: PASS, including `audit` (fix any PKGDOC or LIBS finding by editing the code or owning it with a reason) and `selfcheck`.

- [ ] **Step 8: Commit**

```bash
git add profiles internal/testkit/testkit.go internal/cli/testdata mise.toml Makefile .ness .golangci.yml .golangci.tests.yml lefthook.yml .agents
git commit -m "feat(profiles): go-strict and behavior-specs with the compliance audit; ness applies go-strict to itself"
```

---
### Task 4: The stats package: transcript reader, price table, `--since`

**Files:**
- Create: `internal/stats/stats.go`, `internal/stats/prices.go`, `internal/stats/since.go`, `internal/stats/stats_test.go`, `internal/stats/prices_test.go`, `internal/stats/since_test.go`, `internal/stats/private_test.go`, `internal/stats/testdata/fixture.jsonl`

**Interfaces:**
- Consumes: `probe.ParseDuration` (Go syntax plus `d`, R-PRB-081).
- Produces:
  - `stats.Usage{Input, Output, CacheWrite, CacheRead int}`, `(Usage).Add(Usage) Usage`.
  - `stats.Transcript{Model string; Usage Usage; Turns []int; Compactions, Skipped int; ToolBytes map[string]int}`, `(Transcript).Context() int`.
  - `stats.Read(r io.Reader, since time.Time) (Transcript, error)`: never panics; counts unparsable lines (R-STAT-007); keeps entries at or after `since` (zero: all).
  - `stats.PriceDate = "2026-09-25"`, `stats.Price{Input, Output, CacheWrite, CacheRead float64}` (USD per million), `stats.Cost(model string, u Usage) (cents int, priced bool)`.
  - `stats.ParseSince(s string, now time.Time) (time.Time, error)` (R-STAT-004).

- [ ] **Step 1: Write the fixture and the failing tests**

`internal/stats/testdata/fixture.jsonl` (eleven lines; the numbers are chosen so the cents are checkable by hand; the second session of the CLI scripts reuses it):
```
{"type":"user","timestamp":"2026-10-09T10:00:00.000Z","message":{"role":"user","content":"do it"}}
{"type":"assistant","timestamp":"2026-10-09T10:00:05.000Z","message":{"model":"claude-opus-5-5","usage":{"input_tokens":200000,"cache_creation_input_tokens":1000000,"cache_read_input_tokens":0,"output_tokens":50000},"content":[{"type":"tool_use","id":"t1","name":"Read","input":{}}]}}
{"type":"user","timestamp":"2026-10-09T10:00:06.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}]}}
{"type":"assistant","timestamp":"2026-10-09T10:00:09.000Z","message":{"model":"claude-opus-5-5","usage":{"input_tokens":10000,"cache_creation_input_tokens":0,"cache_read_input_tokens":1200000,"output_tokens":20000},"content":[{"type":"tool_use","id":"t2","name":"Bash","input":{}}]}}
{"type":"user","timestamp":"2026-10-09T10:00:10.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":[{"type":"text","text":"ok"}]}]}}
{"type":"user","timestamp":"2026-10-09T11:00:00.000Z","message":{"role":"user","content":"next"}}
{"type":"assistant","timestamp":"2026-10-09T11:00:04.000Z","message":{"model":"claude-opus-5-5","usage":{"input_tokens":5000,"cache_creation_input_tokens":0,"cache_read_input_tokens":1215000,"output_tokens":1000},"content":[{"type":"text","text":"done"}]}}
not json at all
{"type":"user","timestamp":"2026-10-09T12:00:00.000Z","isCompactSummary":true,"message":{"role":"user","content":"This session is being continued"}}
{"type":"assistant","timestamp":"2026-10-09T12:00:03.000Z","message":{"model":"claude-opus-5-5","usage":{"input_tokens":3000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":100},"content":[{"type":"text","text":"back"}]}}
{"type":"assistant","timestamp":"2026-10-09T12:00:04.000Z","message":{"model":"<synthetic>","usage":{"input_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":0},"content":[{"type":"text","text":"local"}]}}
```
The third line's tool result is exactly 100 `x`. Hand figures: input 218,000; output 71,100; cache write 1,000,000; cache read 2,415,000. Opus 5.5 prices: 0.872 + 1.422 + 5.000 + 0.483 = 7.777 USD → 778 cents. Turns [1,210,000, 1,220,000, 3,000]; one compaction; one skipped line; Read 100 bytes, Bash `len([{"type":"text","text":"ok"}])` = 28 bytes. From 11:00: input 8,000, output 1,100, cache read 1,215,000 → 0.032 + 0.022 + 0.243 = 0.297 → 30 cents; turns [1,220,000, 3,000].

`internal/stats/stats_test.go`:
```go
package stats_test

import (
	"bytes"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/stats"
)

func fixture(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", "fixture.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestReadTheFixture(t *testing.T) {
	// Tokens per class, the context of each turn from the last call, compactions from the
	// marker, tool_result bytes per tool, the unparsable line counted, the synthetic call
	// ignored (Review Focus 1) (R-STAT-001, R-STAT-007, R-STAT-010, R-STAT-011).
	// covers: R-STAT-001, R-STAT-007, R-STAT-010, R-STAT-011
	tr, err := stats.Read(bytes.NewReader(fixture(t)), time.Time{})
	if err != nil {
		t.Fatal(err)
	}
	want := stats.Usage{Input: 218000, Output: 71100, CacheWrite: 1000000, CacheRead: 2415000}
	if tr.Usage != want || tr.Model != "claude-opus-5-5" {
		t.Fatalf("Usage = %+v, Model = %q", tr.Usage, tr.Model)
	}
	if !slices.Equal(tr.Turns, []int{1210000, 1220000, 3000}) || tr.Context() != 3000 {
		t.Fatalf("Turns = %v, Context = %d", tr.Turns, tr.Context())
	}
	if tr.Compactions != 1 || tr.Skipped != 1 {
		t.Fatalf("Compactions = %d, Skipped = %d", tr.Compactions, tr.Skipped)
	}
	if tr.ToolBytes["Read"] != 100 || tr.ToolBytes["Bash"] != len(`[{"type":"text","text":"ok"}]`) || len(tr.ToolBytes) != 2 {
		t.Fatalf("ToolBytes = %v", tr.ToolBytes)
	}
	if cents, ok := stats.Cost(tr.Model, tr.Usage); !ok || cents != 778 {
		t.Fatalf("Cost = %d, %v; want 778 cents", cents, ok)
	}
}

func TestReadSince(t *testing.T) {
	// covers: R-STAT-004
	since := time.Date(2026, 10, 9, 11, 0, 0, 0, time.UTC)
	tr, err := stats.Read(bytes.NewReader(fixture(t)), since)
	if err != nil {
		t.Fatal(err)
	}
	if tr.Usage != (stats.Usage{Input: 8000, Output: 1100, CacheRead: 1215000}) {
		t.Fatalf("Usage since 11:00 = %+v", tr.Usage)
	}
	if !slices.Equal(tr.Turns, []int{1220000, 3000}) || tr.Compactions != 1 || tr.Skipped != 1 {
		t.Fatalf("since 11:00: Turns %v, Compactions %d, Skipped %d", tr.Turns, tr.Compactions, tr.Skipped)
	}
	if cents, _ := stats.Cost(tr.Model, tr.Usage); cents != 30 {
		t.Fatalf("Cost since 11:00 = %d, want 30", cents)
	}
	future, _ := stats.Read(bytes.NewReader(fixture(t)), time.Date(2999, 1, 1, 0, 0, 0, 0, time.UTC))
	if future.Usage != (stats.Usage{}) || len(future.Turns) != 0 {
		t.Fatalf("a future since keeps figures: %+v", future)
	}
}

func TestSkippedCountsEveryBadLine(t *testing.T) {
	// covers: R-STAT-007
	lines := strings.Split(strings.TrimSpace(string(fixture(t))), "\n")
	r := rand.New(rand.NewPCG(1, 2))
	bad := 0
	var out []string
	for _, l := range lines {
		if l == "not json at all" {
			continue
		}
		out = append(out, l)
		for range r.IntN(3) {
			out = append(out, []string{"{", "[1,2]", "\"str\"", "{\"type\":}", "\x00\xff"}[r.IntN(5)])
			bad++
		}
	}
	tr, err := stats.Read(strings.NewReader(strings.Join(out, "\n")), time.Time{})
	if err != nil || tr.Skipped != bad {
		t.Fatalf("Skipped = %d, %v; injected %d", tr.Skipped, err, bad)
	}
}

func FuzzRead(f *testing.F) {
	// covers: R-STAT-007
	b, err := os.ReadFile(filepath.Join("testdata", "fixture.jsonl"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(b)
	f.Add([]byte(`{"type":"assistant","message":{"content":"plain","usage":{"input_tokens":1}}}`))
	f.Add([]byte("[1]\nnull\n{}\n"))
	f.Fuzz(func(_ *testing.T, data []byte) { _, _ = stats.Read(bytes.NewReader(data), time.Time{}) })
}
```

`internal/stats/prices_test.go`:
```go
package stats_test

import (
	"testing"

	"github.com/enea-scaccabarozzi/ness/internal/stats"
)

func TestCostRoundsToCentsByLongestPrefix(t *testing.T) {
	// covers: R-STAT-001
	u := stats.Usage{Input: 1000000, Output: 100000, CacheWrite: 0, CacheRead: 1000000}
	for model, want := range map[string]int{
		"claude-opus-5-5":          620, // 4.00 + 2.00 + 0.20
		"claude-opus-5-5-20260401": 620,
		"claude-sonnet-5-5":        320, // 2.00 + 1.00 + 0.20
		"claude-fable-5-1":         1525, // 10.00 + 5.00 + 0.25
		"claude-haiku-4-5":         160, // 1.00 + 0.50 + 0.10
	} {
		if cents, ok := stats.Cost(model, u); !ok || cents != want {
			t.Errorf("Cost(%s) = %d, %v; want %d", model, cents, ok, want)
		}
	}
	if cents, ok := stats.Cost("<synthetic>", u); ok || cents != 0 {
		t.Errorf("an unknown model is unpriced: %d, %v", cents, ok)
	}
	if cents, _ := stats.Cost("claude-opus-5-5", stats.Usage{Input: 1250}); cents != 1 {
		t.Errorf("0.005 USD rounds to 1 cent, got %d", cents)
	}
	if stats.PriceDate != "2026-09-25" {
		t.Errorf("PriceDate = %q", stats.PriceDate)
	}
}
```

`internal/stats/since_test.go`:
```go
package stats_test

import (
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/stats"
)

func TestParseSince(t *testing.T) {
	// covers: R-STAT-004
	now := time.Date(2026, 10, 9, 12, 0, 0, 0, time.UTC)
	for in, want := range map[string]time.Time{
		"2026-10-01":           time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC),
		"2026-10-01T08:30:00Z": time.Date(2026, 10, 1, 8, 30, 0, 0, time.UTC),
		"7d":                   now.Add(-7 * 24 * time.Hour),
		"36h":                  now.Add(-36 * time.Hour),
	} {
		got, err := stats.ParseSince(in, now)
		if err != nil || !got.Equal(want) {
			t.Errorf("ParseSince(%q) = %v, %v; want %v", in, got, err, want)
		}
	}
	for _, bad := range []string{"yesterday", "", "0s", "-1d", "2026-13-01", "1w"} {
		if _, err := stats.ParseSince(bad, now); err == nil {
			t.Errorf("ParseSince(%q) accepted", bad)
		}
	}
}
```

`internal/stats/private_test.go`:
```go
package stats_test

import (
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/stats"
)

func TestPrivateDiscoveryFigures(t *testing.T) {
	// The saved discovery transcripts reproduce the discovery figures (R-STAT-008). The fixture
	// is .private/stats/*.jsonl with expected.json {"cost_cents","input_tokens","output_tokens"};
	// gitignored, absent in CI, where TestReadTheFixture is the synthetic golden.
	// covers: R-STAT-008
	dir := filepath.Join("..", "..", ".private", "stats")
	data, err := os.ReadFile(filepath.Join(dir, "expected.json"))
	if errors.Is(err, fs.ErrNotExist) {
		t.Skip(".private/stats is absent: the discovery figures are checked locally only")
	}
	if err != nil {
		t.Fatal(err)
	}
	var want struct {
		CostCents    int `json:"cost_cents"`
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	}
	if err = json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	files, _ := filepath.Glob(filepath.Join(dir, "*.jsonl"))
	var total stats.Usage
	cents := 0
	for _, f := range files {
		r, oerr := os.Open(f) //nolint:gosec // the local fixture
		if oerr != nil {
			t.Fatal(oerr)
		}
		tr, rerr := stats.Read(r, time.Time{})
		_ = r.Close()
		if rerr != nil {
			t.Fatal(rerr)
		}
		c, _ := stats.Cost(tr.Model, tr.Usage)
		total, cents = total.Add(tr.Usage), cents+c
	}
	if total.Input != want.InputTokens || total.Output != want.OutputTokens || cents < want.CostCents-len(files) || cents > want.CostCents+len(files) {
		t.Fatalf("got in %d out %d %d cents; want in %d out %d %d cents (±%d)", total.Input, total.Output, cents,
			want.InputTokens, want.OutputTokens, want.CostCents, len(files))
	}
}
```

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/stats/`
Expected: FAIL to compile.

- [ ] **Step 3: Implement `internal/stats/stats.go`**

```go
// Package stats reads the Claude Code transcripts bound to ness sessions and adds them up:
// tokens per price class, the context size of each turn, compactions, tool-result bytes per
// tool, and the cost from an embedded dated price table (design §5, STAT).
package stats

import (
	"bufio"
	"encoding/json"
	"io"
	"time"
)

// Usage is a token count per price class.
type Usage struct {
	Input, Output, CacheWrite, CacheRead int
}

// Add sums u and o.
func (u Usage) Add(o Usage) Usage {
	return Usage{u.Input + o.Input, u.Output + o.Output, u.CacheWrite + o.CacheWrite, u.CacheRead + o.CacheRead}
}

// Transcript is what one transcript file adds up to.
type Transcript struct {
	Model       string         // the model of the last priced call
	Usage       Usage          // every priced call, subagent (sidechain) calls included
	Turns       []int          // the prompt tokens of the last API call of each turn (R-STAT-010)
	Compactions int            // compaction markers (R-STAT-010)
	Skipped     int            // lines that did not parse (R-STAT-007)
	ToolBytes   map[string]int // tool_result bytes per tool (R-STAT-011)
}

// Context is the context size: the prompt tokens of the last turn's last call.
func (t Transcript) Context() int {
	if len(t.Turns) == 0 {
		return 0
	}
	return t.Turns[len(t.Turns)-1]
}

type block struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	ToolUseID string          `json:"tool_use_id"`
	Content   json.RawMessage `json:"content"`
}

type line struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Compact   bool   `json:"isCompactSummary"`
	Sidechain bool   `json:"isSidechain"`
	Message   struct {
		Model   string          `json:"model"`
		Content json.RawMessage `json:"content"`
		Usage   struct {
			Input      int `json:"input_tokens"`
			Output     int `json:"output_tokens"`
			CacheWrite int `json:"cache_creation_input_tokens"`
			CacheRead  int `json:"cache_read_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

// reader carries the state of one Read.
type reader struct {
	t     Transcript
	tools map[string]string // tool_use id -> tool name
	cur   int               // prompt tokens of the last call of the open turn
}

// Read sums the JSONL transcript r, keeping entries at or after since (zero: all). A line that
// is not a JSON object is skipped and counted; nothing panics (R-STAT-007).
func Read(r io.Reader, since time.Time) (Transcript, error) {
	rd := reader{t: Transcript{ToolBytes: map[string]int{}}, tools: map[string]string{}}
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 1<<20), 64<<20)
	for sc.Scan() {
		var l line
		if err := json.Unmarshal(sc.Bytes(), &l); err != nil {
			rd.t.Skipped++
			continue
		}
		if !inRange(l.Timestamp, since) {
			continue
		}
		switch l.Type {
		case "assistant":
			rd.assistant(l)
		case "user":
			rd.user(l)
		}
	}
	rd.closeTurn()
	return rd.t, sc.Err()
}

func inRange(stamp string, since time.Time) bool {
	if since.IsZero() {
		return true
	}
	ts, err := time.Parse(time.RFC3339Nano, stamp)
	return err == nil && !ts.Before(since)
}

func blocks(raw json.RawMessage) []block {
	var out []block
	if json.Unmarshal(raw, &out) != nil {
		return nil // a plain string, or absent
	}
	return out
}

// assistant adds a priced call's usage and remembers its tool uses; the call's prompt is the
// open turn's context. A call without prompt tokens (synthetic) is neither. Sidechain
// (subagent) calls are billed but are not turns.
func (rd *reader) assistant(l line) {
	for _, b := range blocks(l.Message.Content) {
		if b.Type == "tool_use" {
			rd.tools[b.ID] = b.Name
		}
	}
	u := l.Message.Usage
	prompt := u.Input + u.CacheWrite + u.CacheRead
	if prompt == 0 {
		return
	}
	rd.t.Usage = rd.t.Usage.Add(Usage{Input: u.Input, Output: u.Output, CacheWrite: u.CacheWrite, CacheRead: u.CacheRead})
	rd.t.Model = l.Message.Model
	if !l.Sidechain {
		rd.cur = prompt
	}
}

// user counts a compaction marker, attributes tool_result bytes to their tools, and closes the
// open turn on a human message (one without tool results, outside a sidechain).
func (rd *reader) user(l line) {
	if l.Compact {
		rd.t.Compactions++
	}
	results := 0
	for _, b := range blocks(l.Message.Content) {
		if b.Type != "tool_result" {
			continue
		}
		results++
		name := rd.tools[b.ToolUseID]
		if name == "" {
			name = "unknown"
		}
		rd.t.ToolBytes[name] += resultBytes(b.Content)
	}
	if results == 0 && !l.Sidechain {
		rd.closeTurn()
	}
}

func (rd *reader) closeTurn() {
	if rd.cur > 0 {
		rd.t.Turns = append(rd.t.Turns, rd.cur)
	}
	rd.cur = 0
}

// resultBytes is the size of a tool result: the text of a string, else its JSON.
func resultBytes(raw json.RawMessage) int {
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return len(s)
	}
	return len(raw)
}
```

- [ ] **Step 4: Implement `internal/stats/prices.go` and `internal/stats/since.go`**

`prices.go`:
```go
package stats

import (
	"math"
	"strings"
)

// PriceDate is the date of the embedded price table (R-STAT-001); every output names it.
const PriceDate = "2026-09-25"

// Price is USD per million tokens.
type Price struct {
	Input, Output, CacheWrite, CacheRead float64
}

// prices by model-id prefix: the Anthropic first-party rates on PriceDate (input, output and
// cache read as published; cache write at 1.25x input, the 5-minute cache). A longer prefix
// wins, so a dated id (claude-opus-5-5-20260401) finds its family. Verify against the pricing
// page when updating, and move PriceDate with it.
var prices = map[string]Price{
	"claude-fable-5-1":  {10, 50, 12.5, 0.25},
	"claude-fable-5":    {10, 50, 12.5, 1},
	"claude-opus-5-5":   {4, 20, 5, 0.20},
	"claude-opus-5":     {5, 25, 6.25, 0.5},
	"claude-opus-4":     {5, 25, 6.25, 0.5},
	"claude-sonnet-5":   {2, 10, 2.5, 0.20},
	"claude-sonnet-4-6": {3, 15, 3.75, 0.3},
	"claude-sonnet-4":   {3, 15, 3.75, 0.3},
	"claude-haiku-4":    {1, 5, 1.25, 0.1},
}

// Cost prices u for model in whole cents (R-STAT-001). priced is false for a model the table
// does not know; its cost is 0 and the caller says "unpriced".
func Cost(model string, u Usage) (cents int, priced bool) {
	best := ""
	for prefix := range prices {
		if strings.HasPrefix(model, prefix) && len(prefix) > len(best) {
			best = prefix
		}
	}
	if best == "" {
		return 0, false
	}
	p := prices[best]
	usd := (float64(u.Input)*p.Input + float64(u.Output)*p.Output +
		float64(u.CacheWrite)*p.CacheWrite + float64(u.CacheRead)*p.CacheRead) / 1e6
	return int(math.Round(usd * 100)), true
}
```

`since.go`:
```go
package stats

import (
	"fmt"
	"time"

	"github.com/enea-scaccabarozzi/ness/internal/probe"
)

// ParseSince reads --since: a date (2026-10-01), an RFC 3339 time, or a positive duration in
// Go syntax plus d, counted back from now (R-STAT-004).
func ParseSince(s string, now time.Time) (time.Time, error) {
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	d, err := probe.ParseDuration(s)
	if err != nil {
		return time.Time{}, fmt.Errorf("--since %q: want a date (2026-10-01), an RFC 3339 time or a duration (7d, 36h)", s)
	}
	return now.Add(-d), nil
}
```

- [ ] **Step 5: Run the tests, a short fuzz and lint**

Run: `go test ./internal/stats/ && go test ./internal/stats/ -run '^$' -fuzz FuzzRead -fuzztime 10s && golangci-lint run ./internal/stats/`
Expected: PASS. `TestPrivateDiscoveryFigures` skips.

- [ ] **Step 6: Commit**

```bash
git add internal/stats
git commit -m "feat(stats): transcript reader with turns, compactions and tool bytes; the dated price table; --since parsing"
```

---
### Task 5: `ness stats`

**Files:**
- Modify: `internal/cli/stats.go` (replace the stub), `internal/cli/atoms.go` (`transcriptPath`), `internal/testkit/store.go` (`store add-launch`), `internal/cli/testdata/script/surface.txtar` (drop the stub lines)
- Create: `internal/cli/testdata/script/stats.txtar`, `internal/cli/testdata/script/stats_json.txtar`

**Interfaces:**
- Consumes: `stats.Read`, `stats.Cost`, `stats.ParseSince`, `stats.PriceDate`; `store.OpenReadOnly`, `store.ErrOlder`; `claude.ProjectDir`.
- Produces: `ness stats [--since WHEN] [--by session|tier|role|tool] [--json]`; the JSON schema `statsJSON` below (frozen by `stats_json.txtar`); the script verb `store add-launch ID SCOPE PARENT|- FIELD=VALUE...` (fields `tier`, `effort`, `client`, `client_session`, `router_reason`, `worktree`).

- [ ] **Step 1: Write the failing scripts**

`internal/cli/testdata/script/stats.txtar`:
```
# stats reads only the transcripts bound to this project's sessions, is read-only, filters
# with --since, groups by session, tier, role and tool with one total, prints the same figures
# as JSON, carries the router reason, includes closed sessions and lists codex sessions and
# sessions without a transcript as not counted.
# covers: R-STAT-002, R-STAT-003, R-STAT-004, R-STAT-005, R-STAT-006, R-STAT-009, R-STAT-012, R-STAT-013
cd repo
exec ness init --gate true --base main --yes
exec git add -A
exec git commit -q -m init
env NESS_TEST_NOW=2026-10-09T12:00:00Z
store add-launch s-aaaaaaaaaaaa root - tier=standard client=claude client_session=aaaaaaaa-0000-4000-8000-000000000001 router_reason='brief says review'
store add-launch s-bbbbbbbbbbbb kid s-aaaaaaaaaaaa tier=economy client=claude client_session=bbbbbbbb-0000-4000-8000-000000000002
store add-launch s-cccccccccccc codex-kid s-aaaaaaaaaaaa tier=economy client=codex client_session=cccccccc-0000-4000-8000-000000000003
store add-launch s-dddddddddddd closed-kid s-aaaaaaaaaaaa tier=economy client=claude client_session=dddddddd-0000-4000-8000-000000000004
store add-launch s-eeeeeeeeeeee lost-kid s-aaaaaaaaaaaa tier=economy client=claude client_session=eeeeeeee-0000-4000-8000-000000000005
store close-session s-dddddddddddd
exec sh -c 'd=$(pwd -P | sed "s/[^A-Za-z0-9]/-/g"); mkdir -p $HOME/.claude/projects/$d; cp ../a.jsonl $HOME/.claude/projects/$d/aaaaaaaa-0000-4000-8000-000000000001.jsonl; cp ../b.jsonl $HOME/.claude/projects/$d/bbbbbbbb-0000-4000-8000-000000000002.jsonl; cp ../b.jsonl $HOME/.claude/projects/$d/dddddddd-0000-4000-8000-000000000004.jsonl; cp ../b.jsonl $HOME/.claude/projects/$d/99999999-0000-4000-8000-000000000009.jsonl'
snap home $HOME
snap tree
exec ness stats
stdout '^s-aaaaaaaaaaaa\t1\t3\t1\t3000\t218000\t71100\t2415000\t1000000\t\$7\.78$'
stdout '^s-bbbbbbbbbbbb\t1\t1\t0\t1000000\t1000000\t100000\t0\t0\t\$3\.00$'
stdout '^s-dddddddddddd\t1\t1\t0\t1000000\t1000000\t100000\t0\t0\t\$3\.00$'
! stdout 99999999
stdout '^total\t3\t5\t1\t1000000\t2218000\t271100\t2415000\t1000000\t\$13\.78$'
stdout '^not counted\ts-cccccccccccc\tcodex: not counted in v1$'
stdout '^not counted\ts-eeeeeeeeeeee\tno transcript$'
stdout '^prices\t2026-09-25$'
samesnap home $HOME
samesnap tree
store sessions 5
# --since: a date, a duration, and a future date; a word is a usage error.
exec ness stats --since 2026-10-09
stdout '^total\t3\t5\t1\t'
exec ness stats --since 1h
stdout '^total\t3\t2\t1\t'
exec ness stats --since 2999-01-01
stdout '^total\t3\t0\t0\t0\t0\t0\t0\t0\t\$0\.00$'
exits 2 ness stats --since yesterday
stderr 'want a date'
# --by: every grouping has the same total; an unknown dimension is refused.
exec ness stats --by tier
stdout '^standard\t1\t3\t1\t3000\t'
stdout '^economy\t2\t2\t0\t1000000\t2000000\t200000\t0\t0\t\$6\.00$'
exec ness stats --by role
stdout '^root\t1\t'
stdout '^child\t2\t'
exec ness stats --by tool
stdout '^Read\t100$'
stdout '^Bash\t28$'
exec sh -c 'for by in session tier role tool; do ness stats --by $by | grep "^total"; done | sort -u | wc -l'
stdout '^1$'
exits 2 ness stats --by colour
stderr 'accepted values are session, tier, role, tool'
# --json carries the same figures and the router reason.
exec ness stats --json
stdout '"cost_cents": 1378'
stdout '"router_reason": "brief says review"'
stdout '"price_date": "2026-09-25"'
stdout '"reason": "codex: not counted in v1"'

-- a.jsonl --
{"type":"user","timestamp":"2026-10-09T10:00:00.000Z","message":{"role":"user","content":"do it"}}
{"type":"assistant","timestamp":"2026-10-09T10:00:05.000Z","message":{"model":"claude-opus-5-5","usage":{"input_tokens":200000,"cache_creation_input_tokens":1000000,"cache_read_input_tokens":0,"output_tokens":50000},"content":[{"type":"tool_use","id":"t1","name":"Read","input":{}}]}}
{"type":"user","timestamp":"2026-10-09T10:00:06.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t1","content":"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}]}}
{"type":"assistant","timestamp":"2026-10-09T10:00:09.000Z","message":{"model":"claude-opus-5-5","usage":{"input_tokens":10000,"cache_creation_input_tokens":0,"cache_read_input_tokens":1200000,"output_tokens":20000},"content":[{"type":"tool_use","id":"t2","name":"Bash","input":{}}]}}
{"type":"user","timestamp":"2026-10-09T10:00:10.000Z","message":{"role":"user","content":[{"type":"tool_result","tool_use_id":"t2","content":[{"type":"text","text":"ok"}]}]}}
{"type":"user","timestamp":"2026-10-09T11:00:00.000Z","message":{"role":"user","content":"next"}}
{"type":"assistant","timestamp":"2026-10-09T11:00:04.000Z","message":{"model":"claude-opus-5-5","usage":{"input_tokens":5000,"cache_creation_input_tokens":0,"cache_read_input_tokens":1215000,"output_tokens":1000},"content":[{"type":"text","text":"done"}]}}
not json at all
{"type":"user","timestamp":"2026-10-09T12:00:00.000Z","isCompactSummary":true,"message":{"role":"user","content":"This session is being continued"}}
{"type":"assistant","timestamp":"2026-10-09T12:00:03.000Z","message":{"model":"claude-opus-5-5","usage":{"input_tokens":3000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":100},"content":[{"type":"text","text":"back"}]}}
{"type":"assistant","timestamp":"2026-10-09T12:00:04.000Z","message":{"model":"<synthetic>","usage":{"input_tokens":0,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":0},"content":[{"type":"text","text":"local"}]}}
-- b.jsonl --
{"type":"user","timestamp":"2026-10-09T09:00:00.000Z","message":{"role":"user","content":"go"}}
{"type":"assistant","timestamp":"2026-10-09T09:00:05.000Z","message":{"model":"claude-sonnet-5-5","usage":{"input_tokens":1000000,"cache_creation_input_tokens":0,"cache_read_input_tokens":0,"output_tokens":100000},"content":[{"type":"text","text":"ok"}]}}
```
(`a.jsonl` is `internal/stats/testdata/fixture.jsonl` verbatim; the third line's result is 100 `x`. `--since 1h` from 12:00 keeps 11:00 onwards: a.jsonl's turns 2 and 3, nothing of b.jsonl, so 2 turns in total while the three sessions stay counted. Session s-dddd is closed and counted under `economy` (R-STAT-012). The tool total line for `--by tool` is the same `total` line as the others.)

`internal/cli/testdata/script/stats_json.txtar`:
```
# The --json output of stats is pinned byte for byte.
# covers: R-CFG-037, R-STAT-006, R-STAT-009, R-STAT-013
cd repo
exec ness init --gate true --base main --yes
env NESS_TEST_NOW=2026-10-09T12:00:00Z
store add-launch s-aaaaaaaaaaaa root - tier=standard client=claude client_session=aaaaaaaa-0000-4000-8000-000000000001 router_reason='brief says review'
store add-launch s-cccccccccccc codex-kid s-aaaaaaaaaaaa tier=economy client=codex client_session=cccccccc-0000-4000-8000-000000000003
store add-launch s-eeeeeeeeeeee lost-kid s-aaaaaaaaaaaa tier=economy client=claude client_session=eeeeeeee-0000-4000-8000-000000000005
exec sh -c 'd=$(pwd -P | sed "s/[^A-Za-z0-9]/-/g"); mkdir -p $HOME/.claude/projects/$d; cp ../a.jsonl $HOME/.claude/projects/$d/aaaaaaaa-0000-4000-8000-000000000001.jsonl'
exec ness stats --json
cmp stdout ../golden.json
exec ness stats --json --since 2026-10-09T11:00:00Z
stdout '"since": "2026-10-09T11:00:00Z"'
stdout '"cost_cents": 30'

-- golden.json --
{
  "schema": 1,
  "by": "session",
  "since": "",
  "price_date": "2026-09-25",
  "rows": [
    {
      "key": "s-aaaaaaaaaaaa",
      "sessions": 1,
      "turns": 3,
      "compactions": 1,
      "context_tokens": 3000,
      "input_tokens": 218000,
      "output_tokens": 71100,
      "cache_read_tokens": 2415000,
      "cache_write_tokens": 1000000,
      "cost_cents": 778,
      "unpriced": false,
      "skipped_lines": 1,
      "tool_bytes": 128,
      "router_reason": "brief says review"
    }
  ],
  "total": {
    "key": "total",
    "sessions": 1,
    "turns": 3,
    "compactions": 1,
    "context_tokens": 3000,
    "input_tokens": 218000,
    "output_tokens": 71100,
    "cache_read_tokens": 2415000,
    "cache_write_tokens": 1000000,
    "cost_cents": 778,
    "unpriced": false,
    "skipped_lines": 1,
    "tool_bytes": 128,
    "router_reason": ""
  },
  "not_counted": [
    {
      "id": "s-cccccccccccc",
      "reason": "codex: not counted in v1"
    },
    {
      "id": "s-eeeeeeeeeeee",
      "reason": "no transcript"
    }
  ]
}
-- a.jsonl --
(the same eleven lines as in stats.txtar)
```
Write the eleven lines out in full; the plan abbreviates them here only. In `surface.txtar` delete the two stub lines (`exits 4 ness stats` and its `stderr`) and the comment above them.

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/cli/ -run 'TestScripts/(stats|stats_json|surface)'`
Expected: FAIL (`store add-launch` unknown; stats refuses).

- [ ] **Step 3: The script verb in `internal/testkit/store.go`**

Add to the verb map and the doc comment (`store add-launch ID SCOPE PARENT|- FIELD=VALUE...   insert a launched session with tier, effort, client, client_session, router_reason, worktree`):
```go
	"add-launch": func(ts *testscript.TestScript, neg bool, s *store.Store, args []string) {
		if neg || len(args) < 3 {
			ts.Fatalf("usage: store add-launch ID SCOPE PARENT|- FIELD=VALUE...")
		}
		l := store.Launch{ID: args[0], Scope: args[1], Title: args[1], Client: "claude"}
		if args[2] != "-" {
			l.Parent = args[2]
		}
		clientSession := ""
		for _, kv := range args[3:] {
			k, v, _ := strings.Cut(kv, "=")
			switch k {
			case "tier":
				l.Tier = v
			case "effort":
				l.Effort = v
			case "client":
				l.Client = v
			case "client_session":
				clientSession = v
			case "router_reason":
				l.RouterReason = v
			case "worktree":
				l.Worktree = v
			default:
				ts.Fatalf("store add-launch: unknown field %q", k)
			}
		}
		ts.Check(s.AddLaunch(l, scriptNow(ts)))
		if clientSession != "" {
			ts.Check(s.BindAgent(args[0], "", clientSession))
		}
	},
```

- [ ] **Step 4: `transcriptPath` in `internal/cli/atoms.go`**

```go
// transcriptPath is where Claude Code keeps the transcript of a session: the project directory
// of its worktree (or the primary checkout) under the user's home (R-STAT-002).
func transcriptPath(primary string, sess store.Session) string {
	dir := primary
	if sess.Worktree != "" {
		dir = filepath.Join(primary, sess.Worktree)
	}
	return filepath.Join(claude.ProjectDir(userHome(), dir), sess.ClientSession+".jsonl")
}
```
(`path/filepath` joins the imports.)

- [ ] **Step 5: Implement `internal/cli/stats.go`**

```go
package cli

import (
	"cmp"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"maps"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/enea-scaccabarozzi/ness/internal/stats"
	"github.com/enea-scaccabarozzi/ness/internal/store"
)

var statsGroups = []string{"session", "tier", "role", "tool"}

type statsOpts struct {
	since, by string
	asJSON    bool
}

func newStatsCmd(d Deps) *cobra.Command {
	var o statsOpts
	cmd := &cobra.Command{
		Use:   "stats",
		Short: "Tokens, context, compactions, tool bytes and cost of this project's sessions, from their transcripts",
		Args:  cobra.NoArgs,
		RunE:  func(*cobra.Command, []string) error { return runStats(d, o) },
	}
	f := cmd.Flags()
	f.StringVar(&o.since, "since", "", "only entries at or after `WHEN`: a date, an RFC 3339 time or a duration (7d)")
	f.StringVar(&o.by, "by", "session", "group by session | tier | role | tool")
	f.BoolVar(&o.asJSON, "json", false, "print one JSON object")
	return cmd
}

// statsRow is one line of stats; the JSON field schema is frozen by stats_json.txtar (R-CFG-037).
type statsRow struct {
	Key              string `json:"key"`
	Sessions         int    `json:"sessions"`
	Turns            int    `json:"turns"`
	Compactions      int    `json:"compactions"`
	ContextTokens    int    `json:"context_tokens"`
	InputTokens      int    `json:"input_tokens"`
	OutputTokens     int    `json:"output_tokens"`
	CacheReadTokens  int    `json:"cache_read_tokens"`
	CacheWriteTokens int    `json:"cache_write_tokens"`
	CostCents        int    `json:"cost_cents"`
	Unpriced         bool   `json:"unpriced"`
	SkippedLines     int    `json:"skipped_lines"`
	ToolBytes        int    `json:"tool_bytes"`
	RouterReason     string `json:"router_reason"`
}

type notCountedJSON struct {
	ID     string `json:"id"`
	Reason string `json:"reason"`
}

type statsJSON struct {
	Schema     int              `json:"schema"`
	By         string           `json:"by"`
	Since      string           `json:"since"`
	PriceDate  string           `json:"price_date"`
	Rows       []statsRow       `json:"rows"`
	Total      statsRow         `json:"total"`
	NotCounted []notCountedJSON `json:"not_counted"`
}

// figure is one counted session.
type figure struct {
	ID, Tier, Role, Reason string
	T                      stats.Transcript
	Cents                  int
	Priced                 bool
}

func runStats(d Deps, o statsOpts) error {
	if !slices.Contains(statsGroups, o.by) {
		return usage("--by %q: accepted values are %s", o.by, strings.Join(statsGroups, ", "))
	}
	var since time.Time
	if o.since != "" {
		t, err := stats.ParseSince(o.since, d.Now())
		if err != nil {
			return usage("%v", err)
		}
		since = t
	}
	p, err := loadProject(d)
	if err != nil {
		return err
	}
	primary, err := primaryRoot(p)
	if err != nil {
		return err
	}
	rows, err := statsSessions(p)
	if err != nil {
		return err
	}
	figs, nc := collectStats(rows, primary, since)
	out := groupStats(figs, nc, o.by)
	if o.since != "" {
		out.Since = since.UTC().Format(time.RFC3339)
	}
	if o.asJSON {
		enc := json.NewEncoder(d.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(out)
	}
	printStats(d, out)
	return nil
}

// statsSessions reads every session, closed ones included (R-STAT-012), without writing
// (R-STAT-003): no store means no sessions; a store behind this binary is refused with the fix.
func statsSessions(p *project) ([]store.Session, error) {
	path, err := store.Locate(p.root)
	if err != nil {
		return nil, refuse("%v", err)
	}
	s, err := store.OpenReadOnly(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil, nil
	case errors.Is(err, store.ErrOlder):
		return nil, refuse("%v", err)
	case err != nil:
		return nil, err
	}
	defer func() { _ = s.Close() }() //nolint:errcheck // read-only store
	return s.Sessions(true)
}

// collectStats reads each bound Claude Code transcript; codex sessions (R-STAT-013) and
// sessions without a transcript are listed as not counted.
func collectStats(rows []store.Session, primary string, since time.Time) ([]figure, []notCountedJSON) {
	var figs []figure
	var nc []notCountedJSON
	for _, sess := range rows {
		if sess.Client == "codex" {
			nc = append(nc, notCountedJSON{ID: sess.ID, Reason: "codex: not counted in v1"})
			continue
		}
		f, err := os.Open(transcriptPath(primary, sess)) //nolint:gosec // the client's transcript of this session
		if sess.ClientSession == "" || err != nil {
			nc = append(nc, notCountedJSON{ID: sess.ID, Reason: "no transcript"})
			continue
		}
		tr, rerr := stats.Read(f, since)
		_ = f.Close() //nolint:errcheck // read-only file
		if rerr != nil {
			nc = append(nc, notCountedJSON{ID: sess.ID, Reason: "unreadable transcript: " + rerr.Error()})
			continue
		}
		fig := figure{ID: sess.ID, Tier: cmp.Or(sess.Tier, "-"), Role: "child", Reason: sess.RouterReason, T: tr}
		if sess.Parent == store.Operator {
			fig.Role = "root"
		}
		fig.Cents, fig.Priced = stats.Cost(tr.Model, tr.Usage)
		figs = append(figs, fig)
	}
	return figs, nc
}

func (r *statsRow) add(f figure) {
	r.Sessions++
	r.Turns += len(f.T.Turns)
	r.Compactions += f.T.Compactions
	r.ContextTokens = max(r.ContextTokens, f.T.Context())
	r.InputTokens += f.T.Usage.Input
	r.OutputTokens += f.T.Usage.Output
	r.CacheReadTokens += f.T.Usage.CacheRead
	r.CacheWriteTokens += f.T.Usage.CacheWrite
	r.CostCents += f.Cents
	r.Unpriced = r.Unpriced || (!f.Priced && f.T.Usage != (stats.Usage{}))
	r.SkippedLines += f.T.Skipped
	for _, b := range f.T.ToolBytes {
		r.ToolBytes += b
	}
}

// groupStats builds the rows of one dimension and the total, which is the same for every
// dimension (R-STAT-005). Tool rows carry bytes per tool (R-STAT-011).
func groupStats(figs []figure, nc []notCountedJSON, by string) statsJSON {
	out := statsJSON{Schema: 1, By: by, PriceDate: stats.PriceDate, Rows: []statsRow{}, Total: statsRow{Key: "total"}}
	if nc == nil {
		nc = []notCountedJSON{}
	}
	out.NotCounted = nc
	groups := map[string]*statsRow{}
	for _, f := range figs {
		out.Total.add(f)
		if by == "tool" {
			for name, b := range f.T.ToolBytes {
				row := group(groups, name)
				row.ToolBytes += b
			}
			continue
		}
		key := map[string]string{"session": f.ID, "tier": f.Tier, "role": f.Role}[by]
		row := group(groups, key)
		row.add(f)
		if by == "session" {
			row.RouterReason = f.Reason
		}
	}
	for _, key := range slices.Sorted(maps.Keys(groups)) {
		out.Rows = append(out.Rows, *groups[key])
	}
	return out
}

func group(groups map[string]*statsRow, key string) *statsRow {
	if groups[key] == nil {
		groups[key] = &statsRow{Key: key}
	}
	return groups[key]
}

func printStats(d Deps, out statsJSON) {
	if out.By == "tool" {
		say(d.Stdout, "tool\tbytes")
		for _, r := range out.Rows {
			say(d.Stdout, "%s\t%d", r.Key, r.ToolBytes)
		}
	} else {
		say(d.Stdout, "%s\tsessions\tturns\tcompactions\tcontext\tinput\toutput\tcache_read\tcache_write\tcost", out.By)
		for _, r := range out.Rows {
			say(d.Stdout, "%s", statsLine(r))
		}
	}
	say(d.Stdout, "%s", statsLine(out.Total))
	for _, n := range out.NotCounted {
		say(d.Stdout, "not counted\t%s\t%s", n.ID, n.Reason)
	}
	say(d.Stdout, "prices\t%s", stats.PriceDate)
}

func statsLine(r statsRow) string {
	cost := fmt.Sprintf("$%d.%02d", r.CostCents/100, r.CostCents%100)
	if r.Unpriced {
		cost += " (unpriced model)"
	}
	return fmt.Sprintf("%s\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%d\t%s", r.Key, r.Sessions, r.Turns, r.Compactions,
		r.ContextTokens, r.InputTokens, r.OutputTokens, r.CacheReadTokens, r.CacheWriteTokens, cost)
}
```
If `errcheck check-blank` flags the two `_ =` closes, use the `if err := f.Close(); err != nil { return }` form inside a tiny `closeQuietly(io.Closer)` helper. If `statsRow.add` trips `cyclop`, split the token sums into `addUsage(u stats.Usage)`.

- [ ] **Step 6: Run the scripts, lint and the tree**

Run: `go test ./internal/cli/ -run 'TestScripts/(stats|stats_json|surface)' && golangci-lint run ./internal/cli/ ./internal/testkit/ && go test ./...`
Expected: PASS.

- [ ] **Step 7: Commit**

```bash
git add internal/cli internal/testkit/store.go
git commit -m "feat(cli): ness stats replaces the stub: per-session, tier, role and tool figures from the bound transcripts, --since, --json"
```

---
### Task 6: `init` prompts, the exit-code and JSON surface scripts, the missing goldens

**Files:**
- Modify: `internal/cli/cli.go` (`Deps.TTY`), `internal/cli/init.go`, `cmd/ness/main.go`, `internal/cli/script_test.go` (`NESS_TEST_TTY`), `internal/testkit/testkit.go` (`onejson`)
- Create: `internal/cli/testdata/script/init_tty.txtar`, `exit_codes.txtar`, `json_surface.txtar`, `json_goldens.txtar`

**Interfaces:**
- Produces: `Deps.TTY bool` (stdin is a terminal; `cmd/ness` sets it, scripts force it with `NESS_TEST_TTY=1`); the script verb `onejson` (the last stdout is exactly one JSON object with `"schema": 1`).

- [ ] **Step 1: Write the failing scripts**

`internal/cli/testdata/script/init_tty.txtar`:
```
# On a terminal without --yes, init prompts for each value a flag did not give, shows the
# inferred default in brackets, takes an empty answer as the default, refuses an empty gate
# (Review Focus 5) and prompts for nothing with --yes.
# covers: R-CFG-051
cd repo
env NESS_TEST_TTY=1
stdin ../empty.txt
exits 4 ness init
stderr 'missing --gate'
! exists .ness
stdin ../answers.txt
exec ness init
stdout 'project \[repo\]: '
stdout 'base \[main\]: '
stdout 'gate: '
stdout 'client \[claude\]: '
grep '^project = "repo"$' .ness/ness.toml
grep '^base = "main"$' .ness/ness.toml
grep '^gate = "make check"$' .ness/ness.toml
! grep '^client' .ness/ness.toml
rm -r .ness AGENTS.md CLAUDE.md .claude .agents .gitignore
stdin ../answers.txt
exec ness init --gate true --project p
! stdout 'gate: '
! stdout 'project \['
stdout 'base \[main\]: '
grep '^gate = "true"$' .ness/ness.toml
grep '^project = "p"$' .ness/ness.toml
rm -r .ness AGENTS.md CLAUDE.md .claude .agents .gitignore
exec ness init --gate true --yes
! stdout '\]: '
env NESS_TEST_TTY=
stdin ../answers.txt
rm -r .ness AGENTS.md CLAUDE.md .claude .agents .gitignore
exits 4 ness init
stderr 'missing --gate'

-- empty.txt --




-- answers.txt --


make check

```
(`empty.txt` is four empty lines; `answers.txt` is an empty line, an empty line, `make check`, an empty line.)

`internal/cli/testdata/script/exit_codes.txtar`:
```
# One script per exit code of the table: 0 for every happy path, 1 for a check that found
# problems, 2 for a usage error, 3 for a sync conflict, 4 for a refusal.
# covers: R-CFG-034, R-CFG-035
cd repo
exec ness init --gate true --base main --yes
exec git add -A
exec git commit -q -m init
# 0
exec ness status
exec ness sync
exec ness sync --check
exec ness stats
exec ness --version
# 1: a check found problems
rm .agents/skills/ness-launch/SKILL.md
exits 1 ness sync --check
exec ness sync
exits 1 ness doctor
exec sh -c 'printf "features = [\"probes\", \"contracts\"]\n" >> .ness/ness.toml'
exec ness sync
cp ../red.md red.md
exits 1 ness probe run red.md
exec ness contract check
# 2: usage
exits 2 ness bogus
exits 2 ness status --nope
exits 2 ness send
exits 2 ness stats --by colour
exits 2 ness sync --json
# 3: sync conflict
exec sh -c 'printf "profiles = [\"local\"]\n" >> .ness/ness.toml'
exec sh -c 'mkdir -p docs && printf "mine\n" > docs/x.md'
exits 3 ness sync
exits 3 ness sync --check
# 4: refused
exits 4 ness post --channel x --file ../red.md
exits 4 ness open --scope x --tier standard --brief ../red.md
exits 4 ness probe run missing.md
exits 4 ness sync --keep README.md
cd $WORK
exits 4 ness status

-- red.md --
---
every: 1m
floor: exit 1
---
-- repo/.ness/profiles/local/profile.toml --
name = "local"
[files]
"docs/x.md" = { own = "x.md" }
-- repo/.ness/profiles/local/x.md --
theirs
```
(`ness open` as the operator without `--root` is a refusal; if the P3 code reports it as usage, swap that line for `exits 4 ness open --root --scope x --tier standard --brief ../red.md` on a fake herdr that is absent: `fake herdr version 0.0.0` makes the environment check refuse. `ness contract check` with nothing frozen exits 0.)

`internal/cli/testdata/script/json_surface.txtar`:
```
# Every --json command writes exactly one JSON object with "schema": 1 to stdout and nothing
# else, whatever its exit code.
# covers: R-CFG-032, R-CFG-037
cd repo
exec ness init --gate true --base main --yes
exec sh -c 'printf "features = [\"probes\"]\n" >> .ness/ness.toml'
exec ness sync
store add-session s-aaaaaaaaaaaa root
cp ../green.md ../red.md .
env NESS_SESSION=s-aaaaaaaaaaaa
exec ness status --json
onejson
exec ness inbox --json
onejson
exits 1 ness doctor --json
onejson
exec ness stats --json
onejson
exec ness probe run green.md --json
onejson
exits 1 ness probe run red.md --json
onejson
exec ness sync --dry-run --json
onejson

-- green.md --
---
every: 1m
floor: echo n=1
---
-- red.md --
---
every: 1m
floor: exit 1
---
```

`internal/cli/testdata/script/json_goldens.txtar`:
```
# The --json field schemas of inbox, probe run and sync --dry-run are pinned byte for byte
# (status, doctor and stats have their own golden scripts).
# covers: R-CFG-037
cd repo
exec ness init --gate true --base main --yes
exec sh -c 'printf "features = [\"probes\"]\n" >> .ness/ness.toml'
exec ness sync
env NESS_TEST_NOW=2026-10-08T10:00:00Z
store add-session s-aaaaaaaaaaaa root
store add-session s-bbbbbbbbbbbb kid s-aaaaaaaaaaaa
env NESS_SESSION=s-bbbbbbbbbbbb
exec ness send --to manager --body 'need a decision' --intent decision
env NESS_SESSION=s-aaaaaaaaaaaa
env NESS_TEST_NOW=2026-10-08T13:10:00Z
exec sh -c 'ness inbox --json | sed "s/m-[a-z2-7]\{8\}/m-ITEM/" > ../got.json'
cmp ../got.json ../inbox.json
cp ../green.md green.md
exec ness probe run green.md --json
cmp stdout ../probe.json
exec sh -c 'printf "profiles = [\"local\"]\n" >> .ness/ness.toml'
exec ness sync --dry-run --json
cmp stdout ../plan.json

-- green.md --
---
every: 1m
floor: echo n=1
---
-- inbox.json --
{
  "items": [
    {
      "age": "3h",
      "created_at": "2026-10-08T10:00:00Z",
      "deferred": false,
      "excerpt": "need a decision",
      "from": "s-bbbbbbbbbbbb",
      "id": "m-ITEM",
      "intent": "decision",
      "urgent": false
    }
  ],
  "schema": 1
}
-- probe.json --
{
  "evidence": "n=1\n",
  "exit": 0,
  "schema": 1,
  "state": "GREEN",
  "timed_out": false
}
-- plan.json --
{
  "files": [
    {
      "conflict": false,
      "path": "docs/x.md",
      "steps": [
        {
          "action": "create",
          "id": "docs/x.md"
        }
      ]
    }
  ],
  "schema": 1
}
-- repo/.ness/profiles/local/profile.toml --
name = "local"
[files]
"docs/x.md" = { own = "x.md" }
-- repo/.ness/profiles/local/x.md --
x
```
(The `inbox.json` field order follows `itemJSON`'s declaration order; if `cmp` shows a different order or an extra field, the golden is wrong, not the code: fix the golden from the output, since P2 froze that schema.)

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/cli/ -run 'TestScripts/(init_tty|exit_codes|json_surface|json_goldens)'`
Expected: FAIL (`onejson` unknown; no prompts).

- [ ] **Step 3: `onejson` in `internal/testkit/testkit.go`**

Add `"onejson": onejson,` to `Cmds` (and its line to the doc comment: `onejson                   fail unless the last stdout is one JSON object with "schema": 1`) and:
```go
// onejson checks R-CFG-032/037: the last command's stdout parses as exactly one JSON object
// carrying "schema": 1, with nothing after it.
func onejson(ts *testscript.TestScript, neg bool, args []string) {
	if neg || len(args) != 0 {
		ts.Fatalf("usage: onejson")
	}
	dec := json.NewDecoder(strings.NewReader(ts.ReadFile("stdout")))
	var obj map[string]any
	if err := dec.Decode(&obj); err != nil {
		ts.Fatalf("stdout is not a JSON object: %v", err)
	}
	if dec.More() {
		ts.Fatalf("stdout holds more than one JSON value")
	}
	if schema, _ := obj["schema"].(float64); schema != 1 {
		ts.Fatalf(`"schema" = %v, want 1`, obj["schema"])
	}
}
```
(`encoding/json` joins the imports.)

- [ ] **Step 4: `Deps.TTY` and the prompts**

`internal/cli/cli.go`, in `Deps` after `Stderr`:
```go
	// TTY reports that stdin is a terminal: init prompts for missing values (R-CFG-051).
	// cmd/ness sets it; scripts force it with NESS_TEST_TTY=1.
	TTY bool
```

`cmd/ness/main.go`: add `TTY: isTerminal(os.Stdin),` to the `Deps` literal and:
```go
// isTerminal reports whether f is a character device (a terminal).
func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
```

`internal/cli/script_test.go`, in the `ness` func after the `GOOS: "linux"` field: `d.TTY = os.Getenv("NESS_TEST_TTY") == "1"`.

`internal/cli/init.go`: replace `fillInitDefaults` and its call with:
```go
	given := o
	inferInitDefaults(&o, root)
	if d.TTY && !o.yes {
		if perr := promptInit(d, &o, given); perr != nil {
			return perr
		}
	}
	if merr := missingInit(o); merr != nil {
		return merr
	}
```
and the functions:
```go
func inferInitDefaults(o *initOpts, root string) {
	if o.project == "" {
		o.project = filepath.Base(root)
	}
	if o.base == "" {
		o.base = currentBranch(root)
	}
}

// missingInit refuses without a gate or, on a detached HEAD, a base, listing the flags
// (R-CFG-052).
func missingInit(o initOpts) error {
	var missing []string
	if o.gate == "" {
		missing = append(missing, `--gate (the command ness integrate runs, e.g. --gate "make validate")`)
	}
	if o.base == "" {
		missing = append(missing, "--base (HEAD is detached, so the base branch cannot be inferred)")
	}
	if len(missing) > 0 {
		return refuse("missing %s", strings.Join(missing, " and "))
	}
	return nil
}

// promptInit asks, on a terminal, for each value no flag gave (R-CFG-051); the inferred
// default shows in brackets and an empty answer takes it.
func promptInit(d Deps, o *initOpts, given initOpts) error {
	rd := bufio.NewReader(d.Stdin)
	ask := func(name, flag string, value *string) error {
		if flag != "" {
			return nil
		}
		prompt := name + ": "
		if *value != "" {
			prompt = name + " [" + *value + "]: "
		}
		if _, err := fmt.Fprint(d.Stdout, prompt); err != nil {
			return err
		}
		line, err := rd.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return err
		}
		if v := strings.TrimSpace(line); v != "" {
			*value = v
		}
		return nil
	}
	return errors.Join(ask("project", given.project, &o.project), ask("base", given.base, &o.base),
		ask("gate", given.gate, &o.gate), ask("client", given.client, &o.client))
}
```
(`bufio`, `io` join the imports; `fmt.Fprint` is allowed, only `fmt.Print*` is forbidden.) The client prompt shows `[claude]` by setting `o.client`'s display default: pass a copy with `claude` when empty and keep `o.client` empty unless the answer was given, so `ness.toml` stays without a `client` line: in `ask("client", ...)` use a local `client := cmp.Or(o.client, "claude")`, then `if client != "claude" { o.client = client }`. Write it as a small `askClient` wrapper to keep `promptInit` under the function-length limit.

- [ ] **Step 5: Run the scripts, lint and the tree**

Run: `go test ./internal/cli/ -run 'TestScripts/(init_tty|exit_codes|json_surface|json_goldens)' && golangci-lint run ./internal/cli/ ./cmd/... ./internal/testkit/ && go test ./...`
Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add cmd internal/cli internal/testkit/testkit.go
git commit -m "feat(cli): init prompts on a terminal; the exit-code table and every --json command pinned by scripts and goldens"
```

---
### Task 7: Release notes, goreleaser, release-please, commit lint, mutation testing

**Files:**
- Create: `notes.go`, `CHANGELOG.md`, `internal/sync/notes.go`, `internal/sync/notes_test.go`, `.goreleaser.yaml`, `release-please-config.json`, `.release-please-manifest.json`, `.github/workflows/release.yml`, `.github/workflows/release-please.yml`, `.github/workflows/mutation.yml`, `internal/testkit/release_test.go`, `internal/cli/testdata/script/release_notes.txtar`
- Modify: `internal/sync/engine.go` (`Plan.Previous`), `internal/cli/cli.go` (`Deps.Notes`), `internal/cli/sync.go` (`printNotes`), `internal/cli/script_test.go` (`NESS_TEST_NOTES`), `internal/testkit/repo_test.go` (the root package), `.github/workflows/ci.yml`, `mise.toml` (goreleaser), `README.md`

**Interfaces:**
- Produces: `ness.Changelog string` (root package `github.com/enea-scaccabarozzi/ness`); `sync.Note{Version, Body string}`, `sync.Notes(changelog string) ([]Note, error)`, `sync.Between(notes []Note, from, to string) []Note`; `Plan.Previous string` (the lock's `ness` version before this sync); `Deps.Notes string` (empty means the embedded changelog).

- [ ] **Step 1: Write the failing tests**

`internal/sync/notes_test.go`:
```go
package sync_test

import (
	"strings"
	"testing"

	ness "github.com/enea-scaccabarozzi/ness"
	nsync "github.com/enea-scaccabarozzi/ness/internal/sync"
)

const fixture = `# Changelog

## [0.3.2](https://example.com/compare/v0.3.1...v0.3.2) (2026-10-20)

### Features

* stats

### Running sessions

* nothing to do

## Unreleased notes that are not a version

ignored text

## v0.3.1 (2026-10-15)

### Running sessions

* resume each root after sync

## 0.3.0 (2026-10-10)

### Running sessions

* first
`

func TestNotesParseVersionsAndRequireRunningSessions(t *testing.T) {
	// covers: R-SYNC-089
	notes, err := nsync.Notes(fixture)
	if err != nil {
		t.Fatal(err)
	}
	if len(notes) != 3 || notes[0].Version != "0.3.2" || notes[1].Version != "0.3.1" || notes[2].Version != "0.3.0" {
		t.Fatalf("Notes = %+v", notes)
	}
	if !strings.Contains(notes[1].Body, "### Running sessions\n\n* resume each root after sync") {
		t.Fatalf("body of 0.3.1 = %q", notes[1].Body)
	}
	if _, err = nsync.Notes("## 0.1.0\n\n### Features\n\n* x\n"); err == nil || !strings.Contains(err.Error(), "0.1.0") {
		t.Fatalf("a version without Running sessions must be an error, got %v", err)
	}
	embedded, err := nsync.Notes(ness.Changelog)
	if err != nil || len(embedded) == 0 {
		t.Fatalf("the embedded CHANGELOG: %d versions, %v", len(embedded), err)
	}
}

func TestNotesSkipsNonVersionHeadings(t *testing.T) {
	// Review Focus 4: "## Unreleased" and its text are skipped, never a crash or a version.
	notes, err := nsync.Notes(fixture)
	if err != nil {
		t.Fatal(err)
	}
	for _, n := range notes {
		if strings.Contains(n.Body, "ignored text") {
			t.Fatalf("non-version text leaked into %s", n.Version)
		}
	}
	if _, err = nsync.Notes("## Unreleased\n\ntext\n\n## Notes\n"); err != nil {
		t.Fatal(err)
	}
}

func TestBetweenIsOldestFirstAndBounded(t *testing.T) {
	// covers: R-SYNC-032, R-SYNC-033
	notes, _ := nsync.Notes(fixture)
	got := nsync.Between(notes, "0.3.0", "0.3.2")
	if len(got) != 2 || got[0].Version != "0.3.1" || got[1].Version != "0.3.2" {
		t.Fatalf("Between(0.3.0, 0.3.2) = %+v", got)
	}
	if got = nsync.Between(notes, "0.3.2", "0.3.2"); len(got) != 0 {
		t.Fatalf("equal versions print nothing: %+v", got)
	}
	if got = nsync.Between(notes, "0.3.1", "0.3.10"); len(got) != 1 || got[0].Version != "0.3.2" {
		t.Fatalf("numeric comparison: %+v", got)
	}
	if got = nsync.Between(notes, "0.3.2", "0.3.1"); len(got) != 0 {
		t.Fatalf("a downgrade prints nothing: %+v", got)
	}
}
```

`internal/cli/testdata/script/release_notes.txtar`:
```
# sync prints the notes of every version after the lock's and up to the binary's, oldest
# first, with their Running sessions subsection; a second sync prints none; a fresh init
# prints none.
# covers: R-SYNC-032, R-SYNC-033
cd repo
env NESS_TEST_NOTES=$WORK/notes.md
env NESS_TEST_VERSION=0.3.0
exec ness init --gate true --base main --yes
! stdout 'release notes'
grep '"ness": "0.3.0"' .ness/lock.json
env NESS_TEST_VERSION=0.3.2
exec ness sync
cp stdout ../out.txt
stdout 'ness 0.3.1 release notes'
stdout 'ness 0.3.2 release notes'
! stdout '0\.3\.0 release notes'
stdout '### Running sessions'
stdout 'resume each root after sync'
exec sh -c 'grep -n "release notes" ../out.txt | head -1'
stdout '0\.3\.1'
grep '"ness": "0.3.2"' .ness/lock.json
exec ness sync
! stdout .
exec ness sync --check
! stdout .

-- notes.md --
# Changelog

## [0.3.2](https://example.com/compare/v0.3.1...v0.3.2) (2026-10-20)

### Features

* stats

### Running sessions

* nothing to do

## [0.3.1](https://example.com/compare/v0.3.0...v0.3.1) (2026-10-15)

### Bug Fixes

* a fix

### Running sessions

* resume each root after sync

## [0.3.0] (2026-10-10)

### Running sessions

* first
```

`internal/testkit/release_test.go`:
```go
package testkit_test

import (
	"os"
	"strings"
	"testing"
)

func read(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile("../../" + path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func wantAll(t *testing.T, path, text string, needles ...string) {
	t.Helper()
	for _, n := range needles {
		if !strings.Contains(text, n) {
			t.Errorf("%s lacks %q", path, n)
		}
	}
}

func TestGoreleaserBuildsStaticLinuxBinaries(t *testing.T) {
	// covers: R-QA-004
	g := read(t, ".goreleaser.yaml")
	wantAll(t, ".goreleaser.yaml", g, "version: 2", "main: ./cmd/ness", "CGO_ENABLED=0", "-X main.version={{.Version}}",
		"- linux", "mode: keep-existing")
	ci := read(t, ".github/workflows/ci.yml")
	wantAll(t, "ci.yml", ci, "goreleaser check", "goreleaser build --snapshot", "statically linked")
	wantAll(t, "release.yml", read(t, ".github/workflows/release.yml"), "tags:", "goreleaser release --clean")
}

func TestTheGateRunsEveryStep(t *testing.T) {
	// covers: R-QA-005, R-QA-041
	mk := read(t, "Makefile")
	wantAll(t, "Makefile", mk, "validate: build lint test audit vuln selfcheck", "golangci-lint run ./...",
		"golangci-lint run -c .golangci.tests.yml ./...", "python3 .ness/audit.py", "go test -race", "govulncheck",
		"sync --check")
	wantAll(t, "mise.toml", read(t, "mise.toml"), `python = "3.13"`, `golangci-lint = "2.14.0"`, "goreleaser")
	wantAll(t, "ci.yml", read(t, ".github/workflows/ci.yml"), "jdx/mise-action", "make validate")
}

func TestCommitsAreLinted(t *testing.T) {
	// covers: R-QA-030
	ci := read(t, ".github/workflows/ci.yml")
	wantAll(t, "ci.yml", ci, "pull_request", "git log --format=%s", "profiles/builtin/git-conventional/commit-msg")
}

func TestReleasePleaseHasRunningSessions(t *testing.T) {
	// covers: R-QA-031
	cfg := read(t, "release-please-config.json")
	wantAll(t, "release-please-config.json", cfg, `"release-type": "go"`, `"type": "sessions"`,
		`"section": "Running sessions"`)
	wantAll(t, ".release-please-manifest.json", read(t, ".release-please-manifest.json"), `"."`)
	wantAll(t, "release-please.yml", read(t, ".github/workflows/release-please.yml"), "googleapis/release-please-action")
}

func TestMutationTestingIsWeeklyAndReportOnly(t *testing.T) {
	// covers: R-QA-023
	m := read(t, ".github/workflows/mutation.yml")
	wantAll(t, "mutation.yml", m, "schedule:", "cron:", "gremlins", "internal/sync", "internal/store",
		"continue-on-error: true", "upload-artifact")
}
```

In `internal/testkit/repo_test.go` add `"."` to `allowed` (the root package holding the embedded changelog) and update the R-QA-002 comment: `"." is the root package that embeds CHANGELOG.md (R-SYNC-089).`

- [ ] **Step 2: Run them to verify they fail**

Run: `go test ./internal/sync/ ./internal/testkit/ ./internal/cli/ -run 'TestNotes|TestBetween|TestGoreleaser|TestTheGate|TestCommits|TestReleasePlease|TestMutation|TestScripts/release_notes'`
Expected: FAIL to compile / files missing.

- [ ] **Step 3: The root package and the changelog**

`notes.go` (repository root):
```go
// Package ness holds what the binary embeds from the repository root: the release notes
// (R-SYNC-089). Everything else lives under internal.
package ness

import _ "embed"

// Changelog is CHANGELOG.md as built into this binary; sync prints the versions between the
// lock's and the binary's from it (R-SYNC-032).
//
//go:embed CHANGELOG.md
var Changelog string
```

`CHANGELOG.md`:
```markdown
# Changelog

One section per version, newest first, written by release-please from the commits. Every
version has a "Running sessions" subsection (from `sessions:` commits): what an open session
must do after `ness sync` picks the version up. `ness sync` prints the sections between the
lock's version and the binary's.

## [0.1.0] (2026-10-09)

### Features

* v1: the session tree, messages, the worker, probes, notify, contracts, stats and the five
  built-in profiles.

### Running sessions

* Nothing to do: this is the first version.
```

- [ ] **Step 4: `internal/sync/notes.go` and `Plan.Previous`**

```go
package sync

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Note is one version's release notes, its "### Running sessions" subsection included.
type Note struct {
	Version, Body string
}

var versionRe = regexp.MustCompile(`^## \[?v?(\d+\.\d+\.\d+)`)

// Notes splits a CHANGELOG into versions in file order (newest first as release-please writes
// it). A "## " heading that is not a version is skipped with its text; a version without a
// "### Running sessions" subsection is an error (R-SYNC-089).
func Notes(changelog string) ([]Note, error) {
	var out []Note
	var cur *Note
	flush := func() error {
		if cur == nil {
			return nil
		}
		if !strings.Contains(cur.Body, "### Running sessions") {
			return fmt.Errorf("CHANGELOG %s has no \"### Running sessions\" subsection", cur.Version)
		}
		out = append(out, Note{Version: cur.Version, Body: strings.TrimSpace(cur.Body) + "\n"})
		cur = nil
		return nil
	}
	for _, l := range strings.Split(changelog, "\n") {
		if strings.HasPrefix(l, "## ") {
			if err := flush(); err != nil {
				return nil, err
			}
			if m := versionRe.FindStringSubmatch(l); m != nil {
				cur = &Note{Version: m[1]}
			}
			continue
		}
		if cur != nil {
			cur.Body += l + "\n"
		}
	}
	return out, flush()
}

// Between returns the notes of the versions after from and up to to, oldest first
// (R-SYNC-032); none when they are equal (R-SYNC-033) or to is older.
func Between(notes []Note, from, to string) []Note {
	var out []Note
	for i := len(notes) - 1; i >= 0; i-- {
		n := notes[i]
		if versionLess(from, n.Version) && !versionLess(to, n.Version) {
			out = append(out, n)
		}
	}
	return out
}

func versionLess(a, b string) bool {
	pa, pb := versionParts(a), versionParts(b)
	for i := range 3 {
		if pa[i] != pb[i] {
			return pa[i] < pb[i]
		}
	}
	return false
}

func versionParts(v string) [3]int {
	var out [3]int
	for i, s := range strings.SplitN(strings.TrimPrefix(v, "v"), ".", 3) {
		out[i], _ = strconv.Atoi(s)
	}
	return out
}
```
(If `errcheck check-blank` flags `out[i], _ =`, write `n, err := strconv.Atoi(s); if err == nil { out[i] = n }`.)

In `internal/sync/engine.go`: add `Previous string // the lock's ness version before this sync; "" without a lock` to `Plan`, and in `Compute` build the plan as `plan := &Plan{meta: meta, oldLock: oldLock, Previous: lock.Ness}`.

- [ ] **Step 5: `Deps.Notes` and `printNotes`**

`internal/cli/cli.go`, in `Deps` after `Version`: `// Notes is the release notes text; empty means the embedded CHANGELOG. Scripts load a fixture with NESS_TEST_NOTES.` and `Notes string`.

`internal/cli/script_test.go`, after the version seam: `if p := os.Getenv("NESS_TEST_NOTES"); p != "" { b, err := os.ReadFile(p); if err != nil { panic(err) }; d.Notes = string(b) }`.

`internal/cli/sync.go`, in `runSync` replace the apply block with:
```go
	if !o.check && !o.dryRun {
		if err := nsync.Apply(root, plan); err != nil {
			return err
		}
		if err := printNotes(d, plan.Previous); err != nil {
			return err
		}
	}
```
and add:
```go
// printNotes prints the release notes of every version after the lock's and up to this
// binary's, oldest first, Running sessions included (R-SYNC-032); nothing when the versions
// are equal (R-SYNC-033) or there was no lock (a fresh init is not an upgrade).
func printNotes(d Deps, previous string) error {
	if previous == "" || previous == d.Version {
		return nil
	}
	notes, err := nsync.Notes(cmp.Or(d.Notes, ness.Changelog))
	if err != nil {
		return err
	}
	for _, n := range nsync.Between(notes, previous, d.Version) {
		say(d.Stdout, "ness %s release notes\n%s", n.Version, n.Body)
	}
	return nil
}
```
(`cmp` and `ness "github.com/enea-scaccabarozzi/ness"` join the imports.)

- [ ] **Step 6: goreleaser, release-please, the workflows, mise**

`.goreleaser.yaml`:
```yaml
# ness releases: static Linux binaries (R-QA-004). release-please cuts the tag and the GitHub
# release; the release workflow runs goreleaser on the tag and attaches the archives.
version: 2
project_name: ness
builds:
  - id: ness
    main: ./cmd/ness
    binary: ness
    env:
      - CGO_ENABLED=0
    goos:
      - linux
    goarch:
      - amd64
      - arm64
    flags:
      - -trimpath
    ldflags:
      - -s -w -X main.version={{.Version}}
archives:
  - formats: [tar.gz]
    name_template: "{{ .ProjectName }}_{{ .Version }}_{{ .Os }}_{{ .Arch }}"
checksum:
  name_template: checksums.txt
changelog:
  disable: true
release:
  mode: keep-existing
```
(If the installed goreleaser predates `formats`, use `format: tar.gz`; `goreleaser check` says which.)

`release-please-config.json`:
```json
{
  "$schema": "https://raw.githubusercontent.com/googleapis/release-please/main/schemas/config.json",
  "release-type": "go",
  "include-component-in-tag": false,
  "packages": {
    ".": {
      "changelog-sections": [
        { "type": "feat", "section": "Features" },
        { "type": "fix", "section": "Bug Fixes" },
        { "type": "perf", "section": "Performance" },
        { "type": "sessions", "section": "Running sessions" },
        { "type": "revert", "section": "Reverts" },
        { "type": "docs", "section": "Documentation", "hidden": true },
        { "type": "build", "section": "Build", "hidden": true },
        { "type": "ci", "section": "CI", "hidden": true },
        { "type": "chore", "section": "Chores", "hidden": true },
        { "type": "refactor", "section": "Refactoring", "hidden": true },
        { "type": "style", "section": "Style", "hidden": true },
        { "type": "test", "section": "Tests", "hidden": true }
      ]
    }
  }
}
```
`.release-please-manifest.json`: `{ ".": "0.1.0" }`. Every release needs at least one `sessions:` commit (even `sessions: nothing to do`), or `TestNotesParseVersionsAndRequireRunningSessions` fails on the next CHANGELOG; the README says so.

`.github/workflows/release-please.yml`:
```yaml
name: release-please
on:
  push:
    branches: [main]
permissions:
  contents: write
  pull-requests: write
jobs:
  release-please:
    runs-on: ubuntu-latest
    steps:
      - uses: googleapis/release-please-action@v4
        with:
          config-file: release-please-config.json
          manifest-file: .release-please-manifest.json
```

`.github/workflows/release.yml`:
```yaml
name: release
on:
  push:
    tags: ["v*"]
permissions:
  contents: write
jobs:
  goreleaser:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: jdx/mise-action@v2
      - run: goreleaser release --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}
```

`.github/workflows/mutation.yml`:
```yaml
# Weekly mutation testing on sync and store, report-only (R-QA-023): it never fails the gate.
name: mutation
on:
  schedule:
    - cron: "23 4 * * 1"
  workflow_dispatch:
jobs:
  gremlins:
    runs-on: ubuntu-latest
    strategy:
      matrix:
        pkg: [internal/sync, internal/store]
    steps:
      - uses: actions/checkout@v4
      - uses: jdx/mise-action@v2
      - run: go run github.com/go-gremlins/gremlins/cmd/gremlins@v0.5.0 unleash --output=gremlins-$(basename ${{ matrix.pkg }}).json ./${{ matrix.pkg }}
        continue-on-error: true
      - uses: actions/upload-artifact@v4
        with:
          name: gremlins-${{ strategy.job-index }}
          path: gremlins-*.json
          if-no-files-found: warn
```

`.github/workflows/ci.yml`: add two jobs:
```yaml
  commits:
    if: github.event_name == 'pull_request'
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - run: |
          git log --format=%s origin/${{ github.base_ref }}..HEAD | while IFS= read -r subject; do
            printf '%s\n' "$subject" > "$RUNNER_TEMP/msg"
            sh profiles/builtin/git-conventional/commit-msg "$RUNNER_TEMP/msg" || exit 1
          done
  release-check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0
      - uses: jdx/mise-action@v2
      - run: goreleaser check
      - run: goreleaser build --snapshot --clean --single-target
      - run: file "$(find dist -type f -name ness)" | grep 'statically linked'
```

`mise.toml`: add `goreleaser = "2"`.

- [ ] **Step 7: Run the tests, lint, the local release checks and the tree**

Run: `mise install && go test ./internal/sync/ ./internal/testkit/ ./internal/cli/ -run 'TestNotes|TestBetween|TestGoreleaser|TestTheGate|TestCommits|TestReleasePlease|TestMutation|TestPackageLayout|TestScripts/release_notes' && golangci-lint run ./... && goreleaser check && goreleaser build --snapshot --clean --single-target && file "$(find dist -type f -name ness)" && ./dist/*/ness --version && rm -rf dist && make validate`
Expected: PASS; `file` prints `statically linked`; the snapshot binary prints `0.1.1-SNAPSHOT-<sha>` or similar; nothing is pushed or uploaded. Add `dist/` to `.gitignore` next to `.private/`.

`README.md`: replace the status line with `Status: v1 complete (P1–P6): core text and the five built-in profiles, ness stats, release notes in ness sync, goreleaser and release-please configured (first release pending the field validation). Every release needs one `sessions:` commit for its Running sessions section. Plans: docs/superpowers/specs/2026-10-08-ness-v1-scope.md.`

- [ ] **Step 8: Commit**

```bash
git add notes.go CHANGELOG.md internal/sync internal/cli internal/testkit .goreleaser.yaml release-please-config.json .release-please-manifest.json .github mise.toml .gitignore README.md
git commit -m "feat(release): sync prints the release notes between the lock and the binary from the embedded CHANGELOG; goreleaser, release-please, commit lint and weekly mutation testing"
```

---
### Task 8: Coverage gate, wrap-up

**Files:**
- Modify: `internal/testkit/coverage_repo_test.go`

- [ ] **Step 1: BLT and STAT join the gate**

Edit the two variables in place (never replace the file):
- `doneAreas`: append `"BLT", "STAT"`; comment `(P1 to P6: every v1 area)`.
- `deferred`: delete the sixteen P6 entries (`R-CFG-032`, `R-CFG-034`, `R-CFG-035`, `R-CFG-051`, `R-SYNC-032`, `R-SYNC-033`, `R-SYNC-089`, `R-PROF-002`, `R-QA-004`, `R-QA-005`, `R-QA-023`, `R-QA-030`, `R-QA-031`, `R-QA-041`, `R-MOD-002`, `R-MOD-021`), keep the publication and withdrawn lines, add the evals and the review-kind BLT ids.

The result:
```go
// doneAreas are the spec areas whose plans have landed (P1 to P6: every v1 area).
var doneAreas = []string{
	"CFG", "SYNC", "PROF", "QA", "MSG", "HOOK", "MOD", "SESS", "DOC", "WRK", "NTF", "CTR", "PRB", "BLT", "STAT",
}

// deferred names ids of done areas that a review entry, a local run or publication delivers.
var deferred = map[string]string{
	"R-BLT-017": "review", "R-BLT-018": "review", "R-BLT-031": "review", "R-BLT-035": "review",
	"R-BLT-036": "review", "R-BLT-037": "review", "R-BLT-040": "review", "R-BLT-051": "review",
	"R-BLT-052": "review", "R-BLT-062": "review", "R-BLT-090": "review",
	"R-BLT-056": "evals (local, D31)", "R-BLT-063": "evals (local, D31)", "R-BLT-073": "evals (local, D31)",
	"R-QA-032": "publication", "R-QA-033": "publication",
	"R-QA-025": "withdrawn (D39)",
}
```
(The end-of-plan reviewer writes the ledger entries for the `"review"` ids and deletes those lines; the gate then reports any id that is both deferred and covered.)

Run: `go test ./internal/testkit/ -run TestRequirementCoverage`
Expected: PASS. An uncovered BLT or STAT id is a real gap: find the task that delivers it, add the test with its marker, rerun. Where the ids live: BLT 001 (every `builtin_*.txtar`), 002/005/039 (`builtin_all.txtar`), 003/006/092/100 (`profiles_test.go`), 004 (`feature_text.txtar`), 010–016/061 (`core_text.txtar`), 019 (R-SYNC-061's golden in P1, `sync` tests), 030/032/033/034/038/041 (`builtin_program.txtar`), 050/053/054/055/057/058/059/060/071 (`builtin_go_strict.txtar`), 070/072 (`builtin_behavior_specs.txtar`), 080/081 (`builtin_git_conventional.txtar`), 091/093 (`builtin_example_release.txtar`); STAT 001/007/010/011 (`stats_test.go`), 004 (`since_test.go`, `stats.txtar`), 008 (`private_test.go`), 002/003/005/006/009/012/013 (`stats.txtar`, `stats_json.txtar`). R-BLT-019 is the router block's inert line: if the gate names it, add `R-BLT-019` to the `covers:` of the P1 test that pins R-SYNC-061.

- [ ] **Step 2: The full gate and the dogfood**

Run: `make validate && make fuzz FUZZTIME=10s`
Expected: PASS end to end (`FuzzRead` included; the audit, the selfcheck with go-strict applied).

Then the dogfood, in a throwaway repository, never in this checkout:
```bash
cd "$(mktemp -d)" && git init -q && git commit -q --allow-empty -m init
go run ~/src/ness/cmd/ness init --gate true --base main --profile program --profile git-conventional --profile example-release --yes
go run ~/src/ness/cmd/ness sync --check
ls .agents/skills
go run ~/src/ness/cmd/ness stats
go run ~/src/ness/cmd/ness probe run probes/ci-after-push.md; echo "exit $?"
```
Expected: `sync --check` silent, exit 0; the skills `ness-entry`, `ness-launch`, `ness-program`, `ness-git-conventional`, `ness-example-release`, `ness-probes`, `ness-notify`; `stats` prints the header, a `total` line of zeros, `prices 2026-09-25`; `probe run` prints `RED` or `UNCLEAR` with `gh`'s complaint as the evidence and exits 1 (no forge in a throwaway repo; the probe parses and runs, which is the point).

- [ ] **Step 3: Commit**

```bash
git add internal/testkit/coverage_repo_test.go
git commit -m "build: BLT and STAT join the requirement coverage gate; every P6 id delivered"
```

---

## End-of-plan review

After Task 8, launch one fresh reviewer on the most capable model with: `docs/spec/profiles-builtin.md`, `docs/spec/stats.md`, the carry-in ids of this plan's Spec line, this plan, `git diff <first P6 commit>^..HEAD`, and the rendered skill texts under `.agents/skills/` of the dogfood repository. The reviewer checks:
1. **The review-kind requirements**, one ledger entry each in `docs/spec/reviews.md` (`- <id> | <session> | <date> | <commit>`), then the matching `"review"` lines deleted from `deferred`: R-BLT-017 (operator messages decision-first; lessons at handoff and entry; briefs do not promote side remarks; a root delegates over a few edits), R-BLT-018 (the five sizing rules in the probes text), R-BLT-031 (one checklist item per step of the six), R-BLT-035 (one item per rule of the nine), R-BLT-036, R-BLT-037, R-BLT-040 (no root specs, plans, frozen contracts, design detail, pre-designed waves, root implementation, pings per step or project specifics in the program text), R-BLT-051 and R-BLT-052 (each clause of the interface-design and test-strategy texts), R-BLT-062 (the gate docs state one scope: the whole tree), R-BLT-090 (the chain integrate → probe → notify is complete and ness chains nothing).
2. **No private information in the built-ins**: `grep -ri` over `profiles/` for the fixed denylist and for any hostname, real URL, token-looking string or e-mail; `TestDenylistScan` run with the operator's `.private/denylist.txt` present.
3. **Each covered requirement's test would fail without its behaviour** (spot-check: let `Read` count a synthetic call as a turn; make `Cost` ignore cache reads; let `Between` include `from`; make `printNotes` print on equal versions; let `promptInit` skip the gate; drop the `ness-launch` file from core's `profile.toml`; let `Scan` compare case-sensitively; remove `continue-on-error` from `mutation.yml`).
4. **The rulings hold**: no `time.Sleep` in tests; nothing published (`git tag` lists nothing new, no `dist/` committed); `.ness/audit-exceptions.txt` entries all carry reasons; `make validate` green on a clean checkout with `mise install`.
5. **Review Focus**: the five items of the header are each pinned where the plan says.

Then fix each finding, rerun `make validate`, and report to the operator: what shipped, the evidence (test counts, fuzz time, the dogfood run, the snapshot build's `file` line), the rulings above, and the v1-ready items that follow (the first release through release-please, the infrastructure lab canary, the private denylist and stats fixtures to populate).
