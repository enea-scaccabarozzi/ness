> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# ness v1: scope and build split

Date: 2026-10-08. Replaces the waves-and-lanes roadmap (D39).

The design is [`../../design/ness-design.md`](../../design/ness-design.md). The decisions are in
[`../../design/decisions.md`](../../design/decisions.md). The testable requirements are in
[`../../spec/`](../../spec/README.md), one file per area. This document does not restate them. It
splits v1 into six plans that are built one after another in this repository, each by one
session that follows a superpowers plan task by task.

## How the work runs

- One plan at a time. Each one is written against the code that exists when it starts, so it can
  name real types and files. Each plan produces working, tested software on its own.
- TDD inside each task: write the failing test, make it pass, then commit. The plan carries the
  tests. There is no separate test-writer session (this supersedes D33).
- There are no waves, lanes or parallel roots, and no child sessions (this supersedes D34 and
  D36).
- Tests name the requirement ids they cover (`// covers: R-…`, `# covers: R-…`). The coverage
  check fails only for the areas finished plans have delivered. A finished area may list ids it
  deliberately leaves to a later plan.
- At the end of each plan, one fresh reviewer on the most capable model reviews the whole
  branch. Then the operator gets a short report with the evidence.
- Real-client e2e runs keep the D31 stop rule.

## The plans

| # | Plan | Spec areas | Delivers |
|---|---|---|---|
| P1 | Foundation and sync | CFG, SYNC, PROF, QA (gate) | module, gate, CI, `.ness/ness.toml`, version pin, profiles, the sync engine, `init`, `sync` |
| P2 | Store, messages, hooks | MSG, HOOK | SQLite store, `send`, `reply`, `inbox`, `hook session-start/pre-tool` |
| P3 | Sessions and atoms | SESS, MOD, DOC | herdr and Claude Code atoms, integrators, `open`, `close`, `status`, `integrate`, environment checks, `doctor`, Codex keys |
| P4 | Worker | WRK | `worker run/install/uninstall`: delivery, compaction, limit resume, stalls, liveness |
| P5 | Features | PRB, NTF, CTR | probes, notify with channel atoms and `post`, contracts |
| P6 | Text, stats, release | BLT, STAT, QA (rest) | core skill text and the built-in profiles, `stats`, release notes, goreleaser, the compliance audit |

After P6, the field validation runs in the same order as before: the infrastructure lab canary, then the production backend,
then the research lab. The open-source documentation comes after that (D29).

## Deferred within v1

Each plan lists the requirement ids it leaves to a later plan. Mutation testing (R-QA-023) is
deferred until P6.
