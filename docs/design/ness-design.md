> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# ness: design (v3, 2026-10-07)

Status: accepted direction, pre-implementation. Source: the discovery brainstorm of 2026-10-07
(evidence in [`../discovery/`](../discovery/README.md)). Operator answers to the last open
questions are in [`decisions.md`](decisions.md); where this file and a decision disagree, the
decision wins and this file is fixed.

---

## 1. What ness is

ness is a single Go binary that installs and runs an AI coding-agent harness inside one
project: a tree of accountable sessions (a root, its children, their children), the store and
messages between them, a worker that wakes idle sessions, the skills and docs that tell agents
how to work, and optional features (probes, notify, contracts). It was distilled from three
hand-maintained harness forks (an infrastructure lab, a production backend, a research lab).

Principles:

1. **A tree of accountable sessions.** Each session owns a scope and a result, reports up, and
   integrates its children. Parallelism is a manager's option, not a premise. (It started as
   "sessions are roles in a company" and settled into a hierarchy.)
2. **The operator controls ideas, not code.** Engagement concentrates at the start-of-work
   brainstorm and at reserved decisions; routine work proceeds autonomously.
3. **Ceremony must earn its place.** Every change to skills or workflow is backed by evidence
   (a test, a replay, a measurement). No "shoot and pray".
4. **No universal harness, shared primitives.** Each project grows its own declination;
   ness ships the primitives and opinionated, swappable defaults.
5. **Context is the scarce resource.** Fresh sessions, boundary compaction, small outputs.
6. **Explicit and per project.** No global configuration. Everything ness knows about a project
   lives in that repo.
7. **Supported or refused.** ness runs only where it was built and tested; anything else fails
   loudly with what is missing. No silent degradation (section 4).
8. **Tested, not claimed.** Every behaviour ness promises has a test that fails without it.

---

## 2. Building blocks

| Block | Definition | Test to classify a new piece |
|---|---|---|
| **Core** | Always on, never swapped | If missing, ness is not ness |
| **Feature** | Machinery in the binary a project switches on; brings its own skill text | Needs code, many projects never use it, guidance should load only when on |
| **Atom** | One implementation behind a seam; swapping changes nothing else | A contract exists and there are ≥2 real implementations or a test fake |
| **Profile** | Bundle of text, files, config and atom choices that steers the harness; composable, conflicts detected | Mostly words and settings, touching several places at once |
| Project-owned | Skills/docs ness never touches | Everything else |

The **project layer is a profile** (`.ness/profiles/local/`), the last applied and the only one
allowed to override single slots other profiles set.

### Core
Session tree, store and messages, launch, worker, sizes (answer / quick task / full session),
lifecycle skills, compaction policy, tier table, sync engine, doctor, stats.

### Features (v1)
`probes`, `notify`, `contracts`.

### Seams and atoms

| Seam | Contract | v1 | Later |
|---|---|---|---|
| runtime | open tab, run command, type text, read screen, idle/working, focused | herdr; fake for tests | tmux (until then: refuse without herdr) |
| client | launch args (model, effort, prompt, session id, permissions), hooks wiring, transcript → context size and usage, limit/ready notices | Claude Code | Codex (experimental until one clean launch passes in CI) |
| integrator | fold a branch into a target; answer "integrated?" | merge (preserve commits), squash, pr | — |
| channel | render Markdown, post, reply in thread by key | slack, telegram, github, command | discord |
| supervisor | keep the worker running | systemd user unit | launchd |
| model policy | brief + role → tier, effort, confidence, reason; refuses when unsure | text | router (e.g. typed one-shot model such as TypeSafe's Jev) |

There is no headless judge: probes wake the owning session.

### Where today's pieces land

| Piece today | Lands as |
|---|---|
| Registry, queue, dispositions; launch phase machine, adopt, resume, close; delivery worker, boundary compaction, limit resume; sizes | core (the production backend's mechanics as base) |
| Program size, roadmap, waves | profile `program` |
| Research line, rounds, round card | The research lab's local profile; built-in when a second project needs it |
| Contract freeze and guard | feature `contracts` |
| verify-release, watch, deep audit | feature `probes` |
| Slack release bot, Telegram, issues | feature `notify` + channel atoms |
| `ship` / `integrate` | core `ness integrate` + integrator atom |
| Release notes, CI check after push, dev → prod promotion | profile `example-release` (generalised example) |
| golangci, lefthook, test discipline, doc standards | profiles `go-strict`, `behavior-specs` |
| Godog domain contracts, compliance specifics, ops policy, Italian notes, experiments framework, lab policy | project-owned |

---

## 3. Design decisions per goal

| Goal | Choice | Rejected | Known drawback |
|---|---|---|---|
| Distribution, versions | One static Go binary (goreleaser). Project pins a compatible range in `.ness/ness.toml` (`ness = "0.3"`); binary refuses other majors / newer minors. mise recommended for per-directory binaries | Plugin marketplace (less control, can't touch arbitrary files), Python tool, vendored copies (today's forks) | Hooks, unit, panes must resolve the same binary; doctor checks |
| Writing into projects | `ness sync` renders core+features+profiles; committed lock with hashes; generated files committed so upgrades are reviewable diffs | copier, symlinks to installed files, runtime-only injection | Contradictions between ness blocks and free text undetectable |
| Session tree | Worktree + branch + herdr tab per session; brief in session dir; client started with a one-line prompt argument; `--session-id` binds the transcript | CC background sessions / native messaging alone, subagents only, pasting the brief | Requires herdr |
| Messages | SQLite in git common dir; `send` / `reply` / `inbox`; worker wakes for every intent, batched | Files in repo, GitHub issues, native messaging | Worker types into terminals; mitigations in §8 |
| Work sizes, artifacts | Core sizes answer/quick/full; profiles append sizes via a list slot. Records `<sessions>/<root>/[<child>/]{brief,specs,plan,decisions,report}.md` | Fixed size list, templates for every artifact | Quick tasks stay a skill convention |
| Release and verification | `integrate` folds, gates, pushes, records — nothing more. Post-push checks are probes the session starts if its profile says so | One systemd unit per probe; environments as a ness concept | Worker is also a scheduler; heartbeat in status, doctor |
| Model choice | Tier table + job guidance (text policy), named subagents generated, `stats` feedback | Automatic router as default | Still agent judgment; stats exist for that |
| Codex | Client atom, experimental; trust/update flags at launch; model ids checked against `codex debug models` | — | 0 clean launches of 3 so far |

---

## 4. Supported or refused

| Check | Where | On failure |
|---|---|---|
| Binary within project pin; hooks and unit resolve the same binary | every command, doctor | refuse; print expected version and path |
| herdr installed, ≥ tested version | open, worker | refuse; no tmux/plain-terminal fallback |
| Client at a tested version; every tier model id resolves | open, doctor | refuse, naming the tier row |
| No global skill shadows a project skill | open, hook session-start, doctor | refuse, naming both paths |
| systemd user session | worker install | refuse (operator may run `ness worker run` knowingly) |
| Tools a profile declares (`requires_tools`) | sync, doctor | sync fails before writing |
| Every active probe has a live owner | worker, status, doctor | probe pauses, shown red; never a headless run |
| Tier model unavailable mid-run | open, worker | session stops, manager told; `--fallback-model` only if the project sets it |

**Skill shadowing.** Claude Code resolves same-named skills enterprise > personal > project
("With `deploy` in both `~/.claude/skills/` and the project's `.claude/skills/`, `/deploy` runs
the personal one"). Codex does not merge same-named skills; both appear. Plugin skills are
namespaced and cannot shadow; subagent definitions resolve project > user. ness compares names
in `.agents/skills` with `~/.claude/skills`, the enterprise path, `~/.agents/skills`,
`/etc/codex/skills`; an exact match refuses.

---

## 5. Command reference

16 commands; `worker` and `hook` are machinery. Agent hot path: `inbox`, `send`, `reply`,
`open`, `status`, `integrate`, `close`. Short output by default; `--json` where a program reads it.

### Setup and upkeep
- **`ness init`** — create `.ness/ness.toml`, then sync. Flags: `--profile NAME` (repeatable),
  `--client`, `--base`, `--gate "CMD"`, `--yes`. Refuses if the file exists. Short form on TTY.
- **`ness sync`** — render into the repo. Flags: `--check` (exit 1 if anything would change;
  for the gate), `--dry-run` (plan + diffs), `--keep PATH` (keep your version of a conflicted
  file until upstream changes again). Touches only planned paths, `.ness/lock.json`,
  `*.ness-new`. Exit 3 on conflict. Prints release notes for the version range, including what
  running sessions must do.
- **`ness doctor`** — read-only. Binary vs pin and what hooks/unit resolve; client and herdr
  versions; model ids; shadowing global skills; profile-required tools; worker running, same
  version, heartbeat; sync drift; store health (dead routes, sessions whose pane is gone,
  worktrees without a session). Prints fix commands, never applies them. `--json`.
- **`ness stats`** — cost and context from transcripts bound by session id. `--since`,
  `--by session|tier|role|tool`, `--json`. Read-only.

### Sessions
- **`ness open`** — launch a child or root, or resume. Flags: `--scope`, `--tier` (required with
  the text policy), `--brief FILE`, `--title`, `--client`, `--effort`, `--root` (operator: fresh
  root), `--here` (with `--root`: register this running conversation), `--base REF` (root),
  `--resume ID` (relaunch an exited session or continue a stalled launch; `--brief` replaces).
  Touches: worktree under `.worktrees/`, branch `session/<scope>-<id>`, copied/linked files,
  `<sessions>/…/brief.md`, store, herdr tab. Phases persisted before each step; client started
  with "You are ness session ID (scope). Read …/brief.md, then load ness-entry."; SessionStart
  hook writes a ready mark the launch waits for; on timeout the session shows *stalled launch*
  and `--resume` continues from the recorded phase. Refuses when any §4 check fails.
- **`ness close ID`** — manager closes a child; operator closes a root. Flags: `--note`,
  `--discard` (no integration), `--keep-work REASON`. Checks report exists, no open children or
  obligations, integration recorded (unless discard). Removes worktree and tab; the store keeps the session as closed for history and stats (D37).
- **`ness status`** — tree: tier, phase, context, minutes idle, open obligations and who they
  wait on; active probes with state and evidence; worker heartbeat and version. `--all`, `--json`.

### Messages
- **`ness send`** — `--to ID|manager|operator`, `--body TEXT|@FILE`,
  `--intent request|fyi|decision|handoff` (default request), `--urgent`. Idempotency key derived
  from sender + content. Prints one line, never the body.
- **`ness reply ITEM`** — answer and dispose in one step: `--body`,
  `--as resolved|declined|deferred|accepted` (default resolved). Also records an operator's chat
  answer.
- **`ness inbox`** — one line per open item (id, from, intent, age, first 80 chars).
  `--show ITEM` (one full body), `--boundary NOTE` (state written down; compact me when idle).

### Integration
- **`ness integrate`** — `--from REF` (default: this session's branch), `--into REF` (default:
  manager's branch; a root's goes to base), `--subject` (the squash commit's Conventional Commit
  subject, required by the default `squash` mode, D44). Runs the gate in a throwaway worktree,
  folds with the integrator, pushes if configured, records the integration commit or PR. Nothing
  else.

### Features
- **`ness probe start|run|verdict|stop`** — see §7.
- **`ness post`** — `--channel NAME --file MD [--thread KEY] [--dry-run]`. Separate from `send`
  because an external post is irreversible.
- **`ness contract freeze|amend|check`** — `freeze PATHS` records blob shas; `amend PATH --reason`
  opens one file for an accepted change and logs it to decisions.md; `freeze` again closes it;
  `check` for the gate. The edit guard runs in `ness hook pre-tool`.

### Machinery
- **`ness worker run|install|uninstall`** — one process per project: wakes, boundary compaction,
  limit resume, probe floors and schedules, stall reports to managers, self-restart when the
  resolved binary changes. `install` writes `~/.config/systemd/user/ness-<project>.service`, the
  only file outside the repo.
- **`ness hook EVENT`** — called by client hooks. `session-start`: identity, phase, ready mark,
  shadowing check, cheat sheet after compaction. `pre-tool`: contract guard, output guard,
  polling guard (refuses Monitor/loops on `ness inbox`).

### Cut from today's harness (mentions in session digests)

| Today | Mentions | In ness |
|---|---|---|
| `--reconcile --phase --evidence --pane --workspace` | 14 | removed; launch verifies itself; `open --resume` |
| `--request-key` (required) | 433 | removed; derived |
| `--hold`, `--review-at`, `--transfer-to`, `--via issue` | 0 | removed |
| `coordination-authorize` / `close` | 4 / 1 | removed in v1; siblings talk through manager |
| `--record-operator-response` | 18 | folded into `reply` |
| `--reap` / `--retain-work` | 14 / 5 | default / `--keep-work` |
| `--model` | 8 | removed; edit the table; `--effort` stays |
| `ship dev|child|prod|announce|status` | 42/35/28/9/8 | `integrate`, `post`, `status` |
| `verify-release`, `watch` (9 verbs) + units | ~180 | `probe` (4 verbs); worker runs floors |
| earlier ideas `wait`, watchdog, governor, catch-up page | — | dropped; worker rules and skill text |

---

## 6. Files and safe sync

| Path | Mode | Written by | Notes |
|---|---|---|---|
| `.ness/ness.toml` | yours | init, then you | the only config |
| `.ness/lock.json` | ness, committed | sync | owner + hashes per file/block/key; versions; sorted |
| `.ness/profiles/<name>/` | yours | you | local profiles incl. the project's own |
| `AGENTS.md` | blocks | sync | `ness:core/router` + one block per profile |
| `CLAUDE.md` | symlink if absent | sync | if a file, an `@AGENTS.md` import block |
| `.agents/skills/ness-*/` | owned | sync | other skills in the folder untouched |
| `.claude/skills` | symlink if absent | sync | → `.agents/skills` |
| `.claude/agents/ness-*.md` | owned | sync | from the tier table |
| `.claude/settings.json` | keys | sync | hook entries running `ness hook`, `autoCompactWindow`, `env.CLAUDE_CODE_SUBAGENT_MODEL` |
| `.codex/config.toml` | keys | sync | compaction limit, subagent defaults |
| `.gitignore` | block | sync | `.worktrees/`, `*.ness-new` |
| profile files | owned or seeded | sync | seeded = created once if missing, then yours |
| `<sessions>/<id>/brief.md` | session record | open | committed with the work |
| `<sessions>/<id>/{specs,plan,decisions,report}.md`, `probes/*.md` | session record | agents | close checks report |
| `.worktrees/<scope>-<id>/` | runtime | open, close | gitignored |
| `<git-common-dir>/ness/state.sqlite3`, `worker.log` | runtime | all | one log line per action |
| `~/.config/systemd/user/ness-<project>.service` | machine | worker install | only file outside the repo |

Sync rules:
1. Render desired state from core, enabled features, profiles in order, project config.
2. Absent target → create. Present but unrecorded → adopt if identical, else conflict. Never
   overwrite unrecorded content.
3. Recorded and unchanged since ness wrote it → write the new version.
4. Recorded but changed by you → if upstream unchanged since you kept yours, leave it; else write
   `<path>.ness-new`, list it, exit 3. Merge, then `sync --keep PATH`.
5. No longer desired → delete only if unchanged, else report orphaned (yours from then on).

Managed Markdown blocks:
```markdown
<!-- ness:begin core/router -->
## Sessions (ness)
Applies only when the `ness` command exists in this checkout. Otherwise ignore this block.
…
<!-- ness:end core/router -->
```
Found by markers wherever moved; new blocks appended; hash covers only the inside.

JSON/TOML: ness records exact key paths it set; hook entries recognised by command; a changed
owned key is a key-level conflict, the rest still syncs.

Hooks are wrapped so contributors without ness see nothing break:
`command -v ness >/dev/null || exit 0; ness hook …`.

Running sessions and upgrades: code lives in the binary (shared); text lives on the branch (a
child keeps the text of its branch point — consistent, not stale). Within a minor version the
binary accepts older generated text; breaking text changes ship in a minor whose notes say
"upgrade between waves".

---

## 7. Profiles

A profile is a directory with `profile.toml` plus files it renders. Built-ins are embedded
(`embed.FS`) and versioned with the binary; local ones live in `.ness/profiles/`.

```toml
# .ness/profiles/local/profile.toml (example)
name    = "local"
summary = "project policy"
requires_tools = ["python3"]

[slots]
"ness-entry#stop-and-ask"   = { append = "stop-and-ask.md" }
"program-roadmap#autonomy"  = { set = "autonomy-grant.md" }     # single slot: one owner
"ness-launch#tier-roles"    = { append = "tier-roles.md" }
"AGENTS.md#local"           = { block = "agents-local.md" }

[files]
"docs/workflow/ops-policy.md" = { seed = "ops-policy.md" }
```

A profile can: add owned or seeded files; contribute to slots (named places in core/feature
skills, blocks in AGENTS.md); set config defaults, enable features, choose atoms; declare
`requires`, `conflicts`, `requires_tools`.

Conflict rules:

| Two profiles both… | Result |
|---|---|
| append to a list slot | fine, ordered as profiles are listed |
| set a single slot, choose the same seam, set one config key differently | error naming both, unless the project sets it; no "last wins" |
| claim the same owned path | error |
| declare a conflict | error |
| write contradictory prose in their own blocks | not detectable; review the rendered diff |

Only the project's own profile may override a single slot another profile set.

Built-in profiles in v1: `program`, `go-strict`, `behavior-specs` (requires go-strict),
`git-conventional`, `example-release` (generalised; no private information about the production backend).
A local profile becomes built-in when a second project needs it. Sharing profiles by git URL
can come later without changing the format.

---

## 8. Probes (feature)

A probe is a question about the running world, asked on a schedule, owned by a session. Up to
two layers: the **floor** (optional command returning a state cheaply) and the **scope** (the
body: what the woken agent checks, with freedom). An audit is a probe with no floor.

**The worker asks; the agent answers.** `ness probe start` hands files to the worker; on
schedule it runs floors; when `wake` says so (always, for a floorless probe) it puts a request
in the owner's inbox, waking it; the agent investigates within the scope and answers with
`ness probe verdict`; the probe's `notify` policy decides what leaves for a channel.

One file per probe, Markdown with frontmatter:

```markdown
---
every: 30m
window: 48h            # optional; without it, runs until stopped
floor: ./probes/hard-wait.sh
expect: n>=1           # fewer samples = VACUOUS, never GREEN
confirm: 2             # consecutive non-GREEN runs before waking
wake: not-green        # never | change | red | not-green | always
sample: 0.1            # also wake on 10% of GREENs, at random
notify: judged         # none | auto | judged
channel: releases
---
Every hard send under the new config waited at least 50 s on the feed clock.

When woken: read the rows behind the evidence, look for alerts the query cannot see, and
decide whether a GREEN is real.
```

Fields: schedule `every`, `jitter`, `once` (single check at a random time within `jitter`: how
an agent asks for a check on itself), `window`; floor `floor`, `expect`, `confirm`, `timeout`
(exit 0 GREEN, 1 RED, other/timeout UNCLEAR, unmet `expect` on the last line's `key=value`
pairs VACUOUS); waking `wake`, `sample`; posting `notify` (`none`; `auto` = floor state changes
post directly, no agent; `judged` = only verdicts post; floorless `auto` behaves as `judged`),
`channel`; body = scope, sent with every wake.

Commands: `start PATH…` (files or dir; edits reload; `--keep-window`, `--label` to run one file once per release), `run PATH` (run floors
now, print, record nothing: authoring loop), `verdict ITEM --state green|red --note TEXT
[--probe-bug]` (answers a wake; posts if `judged`), `stop PATH --note TEXT`. State shows in
`status`. Ownership falls to the manager when the owner closes; with no live session the probe
pauses, loudly. Evidence capped and redacted (`[probes] redact_env`).

Sizing guidance (skill text): floor first when a cheap command separates RED from GREEN, with a
positive control; raise `sample` as floor coverage falls; skip the floor when no command can
judge or the agent's look is cheaper; narrow named scopes; random timing when risk depends on
the hour.

Integrate, probes and notify are independent. How they chain is the organisation's choice,
written in profile text; `example-release` shows one complete chain.

## 9. Notify (feature)

`ness post` takes one Markdown input; each channel atom renders it, splits on paragraph and
code-fence boundaries when too long, fails when a single block exceeds the limit. A
`note.<channel>.md` next to the note is sent verbatim for that channel. `--dry-run` shows the
exact payload.

| Channel | Renders to | Limit | Tables | Threads |
|---|---|---|---|---|
| slack | mrkdwn (`*bold*`, `<url\|text>`, escape `& < >`) or a `markdown` block | 40,000 text; 12,000 markdown block; 50 blocks | code block | thread ts by key |
| telegram | HTML parse mode (MarkdownV2 needs 18 chars escaped) | 4,096 after entities | `<pre>` | reply / forum topic by key |
| github | GFM as is | 65,536 | native | issue or PR is the thread |
| command | raw Markdown on stdin | — | — | key in env var |

Each renderer has golden tests per construct.

---

## 10. The program profile

Grounded in the one the production backend program root (service-oriented data platform, 09-27 → 10-07, 256
commits, five waves, a 37-step cutover), which followed a discarded 5h27m root.

How it works:
1. Read back: Explore surveys, then the brief restated as roadmap r1 (parts, dependencies,
   holes, questions) within minutes.
2. Only invariants at the root (what ≥2 verticals depend on, or expensive to reverse).
3. Everything else is a vertical with reason, reopen trigger, owner.
4. Breadth-first refinement until the operator accepts the map.
5. Waves of verticals whose inputs exist; each result folds back as a new map revision.
6. Children integrate into the root branch; the root releases.

Why the hand-off worked: restating before designing; operator's words verbatim, by decision id,
in every child brief; detail decided in the vertical where evidence is; progress while the
operator is away; decisions as packets with one-word answers; waves shipped dormant (release ≠
activation).

Feedback from that program that never reached the skill, and what the profile adds:

| Operator feedback | Profile |
|---|---|
| what needs a subagent vs a child, what order | **Conduct** section: executor per vertical and why, order, concurrency cap |
| which unit comes with a deployment | **Releases** section: unit of deploy, dormant vs activation, who gives the go |
| measurements via subagent, "we only care about the conclusions" | rule: grounding runs in subagents; the root reads conclusions |
| a named preference (SigNoz) built past | rule: a named preference becomes an explicit open decision |
| "I won't read child packets" | rule: root brings every child packet into its chat as one-word answers |
| "much more autonomy than you think" (×4) | **Autonomy grant** section, recorded once, widened by the operator |
| specs the operator wanted to own were accepted by the root | **spec owner** column per vertical; root never accepts operator-owned specs |
| CEO verticals never on the map | rule: injected work becomes a map row before its child opens |
| children finished but idle, unnoticed (×3) | core: every notice wakes; status shows who waits on whom; stall reports |
| "are we stuck?" | rule: heartbeat line to the operator at each wave event |
| catch-up requests (×5) | rule: "since you left" from decision ids newer than the operator's last message |
| MD doc unclear, wants HTML | slot `operator-review-format`, default HTML page |
| asked for something the root could get itself | rule: ask only for what the root cannot get |

Fragile: the map went stale (r7 vs ledger O149), got heavy (44.9 KB, re-read after 33
compactions), the root became an ops watcher during cutover, harness fixes stranded on the root
branch.

Contents:
- Skill text: the six steps; the rules above; new map revision on every fold/new vertical with
  recovery line naming the last decision id; a verification/cutover phase is a map section with
  its own probes; fixes to shared tooling go to base via a quick task.
- Artifacts: `roadmap.md` (invariants, verticals with spec owner, waves, Conduct, Releases,
  Autonomy grant, recovery line; size guidance once measured), `decisions.md` (append-only
  ledger, stable ids), `briefs/` committed.
- Child brief slots: question, outcome, invariants by id, operator words verbatim by id,
  sources, rights and limits, tier, spec owner, integration target, how to report.
- Relies on core, no new commands.
- Must not contain: root specs/plans/frozen contracts, design detail at the root, pre-designed
  later waves, root implementation, pings for every step, project specifics.

---

## 11. Quality profiles and ness's own code

`go-strict` (from the production backend; eval new vs old skill):

| Skill | Enforces | Enforced by | Eval |
|---|---|---|---|
| interface-design | consumer-declared interfaces ≤3 methods where faked; injected collaborators built only in `cmd/`; outcomes via ports; no `ForTest`/test hooks; `now` as value, synctest for timers, no Clock interface; no package mutable state; one-sentence package role | audit + judgment | 100% vs 89% |
| test-strategy | implementer doesn't write the judge; contract first in consumer words; black-box tests; hand-written fakes; "which wrong implementation would this reject?" | skill at specify | 96% vs 58% |
| test-discipline | no `time.Sleep`, no permissive mocks/call counts, helpers marked, testdata fixtures, files <400 lines, `-race`, `-count=4` | `.golangci.tests.yml`, audit | 90% vs 52% |
| documentation | code authoritative; godoc on every export starting with its name; package comment once; README floor with Architectural Role | revive, godot, audit | no evals yet |
| compliance-check | audit touched packages at plan and final review; MUSTs fixed or owned; exceptions with reasons | audit in gate | 100% vs 83% |

Files: cleaned `.golangci.yml` (same linters), `.golangci.tests.yml`, lefthook (failure-only
output), `validate` target, the compliance audit as the existing Python script (v1). Slots:
module path, package roots, composition roots, generated dirs, excluded packages, exceptions
file, ceilings, validator/logger libraries, Go version (≥1.25). Lifecycle wiring: specify →
test-strategy, behavior-spec, interface-design; plan → compliance-check and literal
`invoke /<skill>` embeds; handoff → documentation.

`behavior-specs` (requires go-strict): godog contracts — declarative scenarios (one `When`, ≤10
steps, ≤5 example rows), shared step vocabulary, strict runners, scenarios frozen with
`contracts`. Eval 91% vs 64%.

Fix while porting: diff-scoped vs whole-tree gate docs mismatch; `-count=4` policy with no
target; documentation skill lacks evals; copied golangci comments and dead settings.

ness's own code layout:
```
cmd/ness/                 main only
internal/cli/             cobra commands, thin; output via injected writers
internal/config/          ness.toml schema and validation
internal/sync/            render, managed blocks, JSON/TOML keys, lock
internal/profile/         loading, slots, conflicts, requires_tools
internal/store/           SQLite (modernc.org/sqlite, no cgo)
internal/session/         launch phases, close, integrate, environment checks
internal/worker/          delivery, compaction, limit resume, probe scheduler, liveness
internal/probe/           frontmatter parser, floor runner, states, redaction
internal/stats/           transcript reader
internal/notify/          Markdown model, splitter, channel seam, poster
internal/shell/           the command runner behind Deps.Shell
internal/rule/            refusal pointers (skill, section) and their resolver
internal/check/           profile-declared checks at open, integrate, close
internal/ladder/          the model ladder: rungs cheapest first, base, ceiling, clamped moves
internal/atom/{herdr,claude,codex,merge,squash,pr,slack,telegram,github,command,systemd,declared}/
profiles/                 built-ins, embedded
```
ness uses go-strict itself; CLI tests with testscript against throwaway repos and fakes; fuzz
every parser that reads user files; hermetic tests (clear `GIT_*`, refuse outside temp dirs);
gate = lint, test lint, audit, `go test -race`, govulncheck, `ness sync --check` on itself;
weekly mutation testing on sync and store; conventional commits, release-please (with a
"running sessions" section), goreleaser.

---

## 12. Models

Defaults (researched 2026-10-07; Anthropic: "most workloads start with Claude Opus 5.5",
"tuning effort is often a better lever than switching models"; Opus 5.5 medium 86.6% at $0.84
vs high 88.4% at $1.70 on coding; Fable 5.1 low/medium ≥ Fable 5 high):

| Tier | Claude | Codex | Jobs |
|---|---|---|---|
| standard (default for every session) | Opus 5.5 · medium | gpt-6.1-sol · medium | coordination, implementation, reviews; `--effort high` per launch where verification matters |
| frontier | Fable 5.1 · medium (explicit; CC default is high) | astra · medium | design between alternatives, pre-registration, conclusion audits |
| economy | Sonnet 5.5 · medium | gpt-6-luna · low | mechanical children |

Named subagents (generated): `ness-sweep` Sonnet·low (search, digests), `ness-review`
Opus·medium, `ness-verify` Opus·high, `ness-design` Fable·medium.

Mechanics: effort passed per launch with `--effort`, never `CLAUDE_CODE_EFFORT_LEVEL` (it
overrides subagent frontmatter); never change model/effort mid-session (escalate by resuming
one tier up); Explore uses the main model, so skills name `ness-sweep` for searches; Codex ids
validated by doctor (this box's 0.157.1 lists neither Astra nor Sol); advisor mode noted, not
used.

Ladder and selectors (2026-10-21, D45–D47): the tiers are seven rungs of one (model, effort)
ladder, cheapest first — bulk, economy, lean, standard, deep, frontier, apex — with a project
`models.base` (default standard) and an optional `models.ceiling`. A selector answers an offset
from base with a confidence and a one-line reason; `session.Pick` clamps it to the ladder and the
ceiling and is the only place that falls back to base (selector error, or confidence below
`models.router_threshold`). The `declared` selector maps a work kind (`--kind`, or the named
subagent's) to its offset; the `command` selector runs a project script on the brief as JSON.
`ness open` takes the selector's rung unless `--tier` names one (above the recommendation it needs
`--reason`; `--tier up|down` steps a resumed session), `ness pick` asks the selector for a
subagent, and the named agents follow their kind's rung, so moving base moves them all.
When a model ships: sweep effort on the current model first; switch only if that falls short.

---

## 13. Tokens: quick wins only

1. Kill polling: worker wakes on every intent; waiting rule ("pull, then end your turn");
   polling guard hook. (the research lab root: Monitor turns were 49% of API calls, 51% of cache reads.)
2. `inbox` one line per item. (Pulls of 19–29k chars.)
3. Cheat sheet printed after compaction. (`--help` re-read 3–6× per session.)
4. Subagents write to a file and return path + ≤~2k summary.
Plus the bug fix: worker parses every usage-limit format.

Deferred until `stats` data: output-filter hooks, whole-file read rules, lean child plugin
sets, verifier-ladder trimming.

## 14. Worker rules (frictions fixed without new commands)

- Never type into a non-empty input box; read the box as ANSI (dim placeholder ≠ draft); skip
  the operator-focused pane; quiet window keyed on operator input only.
- Wake on every intent, batched; never re-hint items already pulled; retire items whose route is
  revoked; one log line per action, repeated errors once.
- Stall report: a session idle with open obligations for N minutes, or showing an error, gets
  one message to its manager.
- Silent on a limited pane; parse every limit format; resume after reset.
- Restart when the resolved binary changes.

Skill-text fixes: operator messages decision-first and short (page for long); reserved vs
delegated decisions slot; lessons appended at handoff, loaded at entry; briefs don't promote
side remarks to rules; root delegates anything over a few edits.

---

## 15. Migration (agreed)

1. Hot-fix the live harnesses in place (one quick task per repo): worker fyi wake, weekly-limit
   parsing, quiet window, success logs, zombie retirement; launch via brief file + prompt
   argument, retry from `sending-brief`; the research lab `--kind claude`, the production backend's contract-guard and
   ANSI fixes; waiting rule and cheat sheet. Notify roots with a request to reload at their next
   boundary.
2. Build ness in Go in this repo behind the quality bar. No data import from old stores.
3. Canary on the infrastructure lab (dormant).
4. The production backend at a quiet window: sync, delete duplicated AGENTS.md sections, retire harness
   scripts, re-adopt the root with `--here`.
5. The research lab between rounds, research line as its local profile.
6. Improve only on evidence.

## 16. Problems found by mind walks (design responses already folded in)

| # | Project | Problem | Response |
|---|---|---|---|
| 1 | The production backend | AGENTS.md already says what the router block says | adoption notes list sections to delete |
| 2 | The production backend | settings key already set | adopt if equal, else key-level conflict |
| 3 | The production backend | hooks/unit/panes resolve `ness` differently | absolute pinned path in unit; doctor |
| 4 | The production backend | prod promotion has no session | `integrate --from/--into`; no environments concept |
| 5 | The production backend | quick task gone before CI | probe owner falls back to manager |
| 6 | The production backend | probe evidence could carry secrets | `redact_env`, capped evidence |
| 7 | The production backend | live store holds root identity | no import; quiet-window migration, `--here` |
| 8 | The research lab | line and program both add a size | list slot |
| 9 | The research lab | hard-coded compaction reload list | `ness-entry#post-compact` list slot |
| 10 | The research lab | gitignored data missing in worktrees | `worktree.link` |
| 11 | The research lab | research line is one project's idea | stays local until a second user |
| 12 | OSS lib | most projects integrate via PRs | `pr` integrator in v1; close waits for merge |
| 13 | OSS lib | contributors without ness | wrapped hooks; inert router block |
| 14 | OSS lib | no herdr/systemd | refuse loudly; out of scope until atoms exist |
| 15 | OSS lib | committing generated files | decided: always commit; no private mode planned |
| 16 | team app | lock conflicts across versions | shared pin; deterministic sorted lock |
| 17 | team app | teammate edits inside a block | block-level conflict, never silent overwrite |
