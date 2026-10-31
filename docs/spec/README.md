> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# ness requirements specification

Status: draft r1, 2026-10-07. Derived from [`../design/ness-design.md`](../design/ness-design.md)
(the design, "§n" below) and [`../design/decisions.md`](../design/decisions.md) ("Dn", authoritative
where it differs from the design), split into plans by [`../superpowers/specs/2026-10-08-ness-v1-scope.md`](../superpowers/specs/2026-10-08-ness-v1-scope.md).
This spec adds no behaviour the design does not state, except where the program root resolved an
open point; each such resolution is listed in [resolutions.md](resolutions.md). A question still
unresolved stays under **Open points** at the end of its file and must be answered before the
acceptance tests for it are frozen (D33).

## Index

| File | Area code | Plan | Scope |
|---|---|---|---|
| [core-config.md](core-config.md) | CFG | P1 | `.ness/ness.toml`, version pin, `ness init`, CLI conventions, exit codes |
| [sync.md](sync.md) | SYNC | P1 | `ness sync`, file table, sync rules, managed blocks, keys, lock |
| [profiles.md](profiles.md) | PROF | P1 | profile format, slots, ordering, conflict rules |
| [checks.md](checks.md) | CHK | E | profile-declared checks at open, integrate and close |
| [rules.md](rules.md) | RULE | E | refusal pointers `(skill, section)`, the registry and its resolution |
| [store-messages.md](store-messages.md) | MSG | P2 | SQLite store, `send`, `reply`, `inbox` |
| [sessions.md](sessions.md) | SESS | P3 | `open`, `close`, `status`, `integrate`, environment checks (§4) |
| [worker.md](worker.md) | WRK | P4 | `ness worker`, delivery, compaction, limits, stalls, liveness |
| [probes.md](probes.md) | PRB | P5 | probe files, floors, states, waking, `ness probe` |
| [notify.md](notify.md) | NTF | P5 | `ness post`, channel atoms, rendering, limits |
| [contracts.md](contracts.md) | CTR | P5 | `ness contract`, edit guard |
| [hooks.md](hooks.md) | HOOK | P2 | `ness hook session-start`, `ness hook pre-tool` |
| [doctor.md](doctor.md) | DOC | P3 | `ness doctor` |
| [stats.md](stats.md) | STAT | P6 | `ness stats` |
| [models.md](models.md) | MOD | P3/P6 | tier table, effort, named subagents, model policy seam |
| [profiles-builtin.md](profiles-builtin.md) | BLT | P6 | core skill text and the five built-in profiles |
| [ui.md](ui.md) | UI | cli-ux | plain and rich renderers, `--ui`, colour, `status --watch` |
| [quality.md](quality.md) | QA | P1/P6 | ness's own repository quality gate |
| [update.md](update.md) | UPD | update-path | releases, install, self-update, upgrade, compatibility |
| [resolutions.md](resolutions.md) | — | — | trail of resolved open points (OP-id → resolution) |

## Requirement format

Each requirement is one list item:

```markdown
- **R-<AREA>-<NNN>** <subject> MUST <one observable behaviour>.
  Verify: <kind> — <the observable that fails if the behaviour is broken>.
```

- **Id**: `R-<AREA>-<NNN>`, AREA from the index, NNN a zero-padded three-digit number unique
  within the area. Ids are never reused or renumbered; a withdrawn requirement keeps its id and
  is marked `(withdrawn: reason)`.
- **Statement**: one testable behaviour, in RFC 2119 language. MUST / MUST NOT are absolute;
  SHOULD / SHOULD NOT allow a deviation only with a recorded reason in the session's
  `decisions.md`. A statement names the command, flag, file or observable it constrains.
- **Verify**: the test kind and the observable that fails if the behaviour is broken.

## Verify kinds

| Kind | Meaning |
|---|---|
| `unit` | Go test of one package, black-box, hand-written fakes; timers via `testing/synctest` |
| `fuzz` | Go native fuzz target; the seed corpus includes every real-world example cited |
| `testscript` | CLI test (`rogpeppe/go-internal/testscript`) against a throwaway git repo and the `internal/testkit` fakes |
| `contract(fake+real)` | one shared contract suite run against the test fake and the real atom (herdr, Claude Code, Codex, git integrators, channels); the real run may be gated by environment availability but must run in CI or a recorded field run before its plan closes |
| `e2e(real client)` | a real client session (Claude Code) launched by ness; bounded by D31 (stop if it eats the weekly budget) |
| `review` | checked by a fresh reviewer against the rendered output or source; used only where no mechanical test can observe the behaviour |

## Coverage convention

- A Go test covers a requirement by a comment in the test function body or immediately above
  it: `// covers: R-SYNC-001` (several ids separated by commas or on several lines).
- A testscript file covers a requirement by a line `# covers: R-SYNC-001` anywhere in the
  script's comment header.
- A `review` requirement is covered by an entry in `docs/spec/reviews.md` naming the id, the
  reviewer session, the date and the commit reviewed (see R-QA-012).
- The gate extracts every id defined in `docs/spec/*.md` and every id referenced by a `covers:`
  marker; it **fails on any defined id with no covering test** (I4, R-QA-010) and on any marker
  naming an undefined id (R-QA-013).

## Conventions used throughout

- "Refuse" means: exit 4, write nothing (no file, store row or herdr tab created),
  and print to stderr what is missing and how to fix it (§4, D15). Exit codes are in R-CFG-035.
- "One line" means exactly one `\n`-terminated line on stdout.
- `<sessions>` is the session-record directory (`sessions` key, default `docs/sessions`);
  `<git-common-dir>` is `git rev-parse --git-common-dir` of the project.
- "Operator" is the human; "manager" is the session (or operator, for a root) one level up.
