> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# Decisions

Append-only. Each entry: id, date, decision, who decided, why. Later entries may supersede
earlier ones by id; nothing is edited in place.

## 2026-10-07: discovery brainstorm (operator: Enea)

- **D1** ness is a single Go CLI written from scratch; no plugin registries. Why: careful build,
  operator fluency in Go, rich CLI ecosystem, granular control needed by profiles.
- **D2** Strict code quality and checks; the production backend's policies are the starting point (go-strict).
- **D3** Atoms swap alone; profiles are composable cross-cutting bundles with conflict detection;
  the project layer is a profile, tightly bound to the project.
- **D4** ness modifies project files safely: never deletes or overwrites user content; injects
  into shared files (AGENTS.md, docs) via managed blocks / owned keys.
- **D5** No global configuration; setup is project-specific and explicit.
- **D6** "Sessions are roles in a company" is not a core principle; the core is a hierarchy of
  accountable sessions; parallelism is optional.
- **D7** Claude Code native session messaging is not a foundation; the custom store + worker is.
- **D8** Keep CLI surface minimal; `stats` and `doctor` stay; no `wait`, watchdog, governor or
  catch-up commands.
- **D9** Probes and notify are first-class features. A probe has an optional mechanical floor and
  a semantic scope; the floor may wake the agent (`wake`, `sample`); floorless probes are pure
  semantic checks; each probe has a notify policy `none | auto | judged`.
- **D10** Audits wake the owning agent (queue item); no headless runs.
- **D11** Probe files use Markdown with YAML frontmatter, one probe per file.
- **D12** Integrate, probes and notify are not chained by ness; ship an example profile (the
  production backend's flow, generalised) and keep the design adaptable to different organisations.
- **D13** Channels render each platform's format (Slack, Telegram, GitHub, …) from one Markdown
  input.
- **D14** Doctor and launch check for globally installed skills that shadow project skills and
  refuse on exact name matches.
- **D15** Unsupported environments fail loudly; no silent degradation. A known trade-off.
- **D16** Token optimisation: only low-hanging fruit now; the rest deferred until ness projects
  produce data.
- **D17** Model policy is text-based by default, swappable (router seam) for future approaches
  such as typed one-shot models (e.g. TypeSafe Jev).
- **D18** Default model for sessions is Opus 5.5 at medium effort; high per launch; Fable at
  medium. (Operator: "Correct".)
- **D19** The program profile is one of ness's most important features and must carry the
  operator feedback from the production backend program (see design §10).
- **D20** go-strict includes the interface-design, test-strategy, test-discipline, documentation
  and compliance skills; godog contracts in a separate behavior-specs profile.
- **D21** Migration plan (hot-fix in place → build ness → canary the infrastructure lab → the production backend → the research lab →
  evidence-gated improvements) accepted.

## 2026-10-07: answers to v3 open questions (operator)

- **D22** Generated files are committed by default. A private mode is not planned: a private
  methodology would damage product development and cause drift between team members.
- **D23** Opus 5.5 at medium for every session, high only per launch; Fable at medium. Confirmed.
- **D24** `wake` + `sample` are enough for floor-triggered semantic checks for now.
- **D25** Program map size: guidance only, once measured (no enforced check).
- **D26** Compliance audit ships as the production backend's existing Python script for now.
- **D27** `example-release` is generalised: it must not leak any private information about the production
  backend.
- **D28** mise is the recommended version pin. ness will be published open source on the
  operator's GitHub account.
- **D29** Build carefully for open-source publication; everything must be tested, not claimed.
  After a first version: clean documentation for open source (philosophy, what it is, why it
  exists).

## 2026-10-07: development and verification approach (operator)

- **D30** Initial commit on `main` includes the private `docs/discovery/`. Publication will use
  a fresh history so that folder never reaches the public repository.
- **D31** Real end-to-end runs (real Claude Code sessions) are allowed. Stop if they eat the
  operator's weekly Claude Code budget too quickly.
- **D32** A private GitHub repository with Actions CI from W0. Created as
  `enea-scaccabarozzi/ness`, the authenticated account. The module path agreed as
  `github.com/scaccabarozzi-enea/ness` names a different GitHub account, so the module path
  follows the repo: `github.com/enea-scaccabarozzi/ness`, pending operator confirmation.
- **D33** Acceptance tests are written and frozen by a separate session before implementation;
  the implementer may not edit them.
- **D34** Waves W0–W8 as proposed (skeleton and gate, sync, messaging, sessions, worker,
  features, profiles, field validation, open-source docs). Commit to W0–W1 first and reassess
  velocity.
- **D35** Migration step 1 (in-place hot-fixes of the live harnesses) runs before W0. Briefs in
  `docs/discovery/migration/`, executed as one operator-launched root per repo.
- **D36** Waves and lanes run in parallel wherever dependencies allow, without asking first
  (operator authorization). The dependency map is in `docs/roadmap.md`.

## 2026-10-07: requirements spec (root)

- **D37** The requirements spec lives in `docs/spec/` (546 requirements). The root resolved the
  draft's open points; the trail is in `docs/spec/resolutions.md`. Notable: the store keeps
  closed sessions for history and stats (amends design §5). `[contracts] frozen` lists
  project-wide frozen paths that `contract check` verifies alongside the session locks. A
  `pre-tool` block exits 2 because Claude Code requires it; this is the only exception to the
  exit-code table. Usage-limit screens are not stall patterns. Only OP-QA-6 (module path,
  D32) remains open for the operator.
- **D38** The module path is `github.com/enea-scaccabarozzi/ness`; the operator confirmed it,
  closing D32 and OP-QA-6.

## 2026-10-08: simpler build process (operator)

- **D39** The waves, lanes, parallel roots and child sessions are dropped: "this program is not
  that big to require all this program and workflow" (operator), and the session tree had no
  machinery to wake its root when children finished. v1 is split into six superpowers plans
  (`docs/superpowers/specs/2026-10-08-ness-v1-scope.md`), built one at a time by one session
  with TDD inside each task and one fresh reviewer per plan. Supersedes D33 (separate test
  writer), D34 (waves) and D36 (parallel lanes); `docs/roadmap.md` is removed.

## 2026-10-08: P3 spikes (session)

- **D40** The Claude Code enterprise skills path on Linux is `/etc/claude-code/skills` (the
  managed directory `/etc/claude-code`, which the installed client 2.1.294 names in its binary;
  the enterprise skills directory is `skills/` under it, as the Claude Code skills reference
  documents). It is the constant `claude.EnterpriseSkills` (R-SESS-014). Evidence: `grep -a
  'etc/claude-code'` on the installed binary.

## 2026-10-10: rich terminal output (operator)

- **D41** The operator asked for "rich tables, colors, interactivity via rich patterns, options
  to disable/configure this aspects in different environments". ness gets a second renderer for
  a human at a terminal, built on the Charm libraries (lipgloss, huh, glamour, bubbletea, fang);
  the plain renderer stays byte-identical, because agents, scripts and tests read it, and agents
  never get the rich one. The choice is per run: `--ui`, `NESS_UI`, then terminal detection
  (`docs/spec/ui.md`). Preferences are environment variables, not a user config file, so
  R-CFG-001 holds; R-CFG-002 lists the new variables.

## 2026-10-10: session names in the client (operator)

- **D42** The operator does not want Claude Code to title sessions by itself; each session's name
  in the client must be its tab title prefixed with the project, so transcripts and the mobile
  apps show which session is which. ness launches and resumes Claude Code with
  `--name <project>/<title>` (an explicit name replaces the generated title, and the client
  restores it on resume). A conversation adopted with `--here` is already running and Claude Code
  offers no programmatic rename, so the worker types `/rename <project>/<title>` into its pane the
  first time it is idle and free, as it types `/compact`. Codex has no session-name flag; its
  sessions keep their tab title only.
- **D43** Sessions ness launches skip the client's permission prompts by default (R-SESS-125): the
  operator wants children to run without auto-mode's classifier stopping them, on both clients.
  ness's own guards are hooks, and hooks still run and still block in that mode (checked live on
  Claude Code: a ness session started with `--dangerously-skip-permissions` had its inbox poll
  refused). `launch.permissions = "client"` keeps the client's own settings for a project that
  wants them.

## 2026-10-11: integration by squash (operator)

- **D44** The operator wants integrate to "work as squash merge (more friendly to rel please)".
  `integrate.mode` defaults to `squash` (R-CFG-060); merge, rebase and pr stay available. The
  subject of the commit that lands is the session's to write, so `ness integrate --subject` carries
  it and a squash without a Conventional Commit subject is refused before the fold (R-SESS-140): a
  default subject would be either non-conventional (CI rejects it) or a guess (release-please files
  it under the wrong type). The body lists every folded subject between `BEGIN_NESTED_COMMIT` and
  `END_NESTED_COMMIT`, the form release-please reads as separate changes of any configured type,
  `sessions` included; its top-level split knows only the eleven standard types (R-SESS-141). A
  branch integrated again after a squash folds from the source sha the newest trailer names, so
  the root keeps working on its branch and landed changes are not replayed (R-SESS-142). With the
  gate run by every integrate and by CI, the pre-push hook keeps only its light half (R-QA-044) and
  `-count=4` runs in CI only (R-QA-009).

## 2026-10-21: model ladder and selectors (session model-select)

- **D45** One ladder of (model, effort) rungs, one offset (MS-D3). Separate model and effort
  offsets were proposed; tiers are points on one cost/quality frontier and effort scales are
  model-calibrated, so a rung carries both and every selector returns one offset from base.
- **D46** The Jev selector ships as a built-in Go atom (plain HTTP, no SDK), not an external
  script (MS-D4): it runs inside the hook and `ness pick` without a Python runtime in consumer
  projects. Swappability stays through the same seam's `command` policy; `ness pick` is the
  executable an agent calls.
- **D47** The new rungs are named bulk, lean, deep and apex (MS-D5); economy, standard and frontier
  keep their names so existing `--tier` habits and briefs keep working.
- **D48** The ladder is enforced in long sessions at the pre-tool hook, not left to the agent: a
  subagent call above the selector's recommendation is refused unless the prompt carries
  `escalate: <reason>`, which buys one rung. Every decision is stored, and `models.enforce = "log"`
  is the loosening switch if stats show friction. The hook's log is best effort: a store write that
  fails never blocks a tool.
