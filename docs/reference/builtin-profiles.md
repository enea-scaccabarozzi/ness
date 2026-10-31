---
title: Built-in profiles
description: The five profiles that ship in the binary, what each renders, and the slots each offers.
---

Turn a built-in profile on by listing it in `profiles` in `.ness/ness.toml`, then run
`ness sync`. A seeded file is created once, then yours. An owned file is kept current by every
sync.

## `program`

A root that maps a program into verticals, runs waves of children and releases. Guide:
[Steer a multi-week program](../guides/program.md).

| Item | Value |
|---|---|
| Sizes | `program` |
| Skills (owned) | `ness-program` |
| Seeded | `docs/program/roadmap.md`, `docs/program/decisions.md`, `docs/program/briefs/TEMPLATE.md`, `docs/doctrine.md` |
| Appends | `ness-entry#sizes`, `ness-entry#post-compact` |
| Slots it offers | `ness-program#autonomy` (single), `ness-program#operator-review-format` (single) |
| Checks | `autonomy-recorded`, `checkpoint-before-wave`, `recovery-line` (open); `no-child-decision-ids` (integrate). All for children of a `program` root. |

## `go-strict`

Strict Go quality: lint, test lint, a compliance audit, lefthook and `make validate`.

| Item | Value |
|---|---|
| Requires tools | `python3`, `golangci-lint`, `go`, `make` |
| Skills (owned) | `ness-interface-design`, `ness-test-strategy`, `ness-test-discipline`, `ness-documentation`, `ness-compliance-check` |
| Owned files | `.golangci.yml`, `.golangci.tests.yml`, `lefthook.yml`, `.ness/audit.py`, `.ness/audit.toml` |
| Seeded | `.ness/audit-exceptions.txt`, `Makefile` |
| Appends | `ness-entry#specify`, `ness-entry#plan`, `ness-entry#handoff` |
| Slots it offers | `.golangci.yml#module-path`, `.golangci.yml#go-version`, and in `.ness/audit.toml`: `package-roots`, `composition-roots`, `generated-dirs`, `excluded-packages`, `exceptions-file`, `ceilings`, `libraries` (all single) |

Set the module path and Go version from your local profile:

```toml title=".ness/profiles/local/profile.toml"
[slots]
".golangci.yml#module-path" = { set = "module-path.md" }
".golangci.yml#go-version"  = { set = "go-version.md" }
```

## `behavior-specs`

Declarative godog scenarios with a shared step vocabulary, frozen with contracts.

| Item | Value |
|---|---|
| Requires | `go-strict` |
| Enables | `contracts` |
| Skills (owned) | `ness-behavior-specs` |
| Appends | `ness-entry#specify` |

## `git-conventional`

Conventional Commits, checked by a `commit-msg` hook.

| Item | Value |
|---|---|
| Skills (owned) | `ness-git-conventional` |
| Owned files | `.githooks/commit-msg` |
| Types accepted | `build`, `chore`, `ci`, `docs`, `feat`, `fix`, `perf`, `refactor`, `revert`, `sessions`, `style`, `test` |

Turn the hook on once per clone:

```sh
git config core.hooksPath .githooks && chmod +x .githooks/commit-msg
```

## `example-release`

One complete release chain as profile text: integrate, a probe on CI after the push, and a
release note.

| Item | Value |
|---|---|
| Enables | `probes`, `notify` |
| Skills (owned) | `ness-example-release` |
| Seeded | `probes/ci-after-push.md`, `docs/release-notes/TEMPLATE.md` |

## Core and features

Core is always on. It renders `ness-entry`, `ness-launch`, the agents `ness-design`,
`ness-review`, `ness-sweep` and `ness-verify`, the `AGENTS.md` router block and the client
settings. Each feature adds its skills:

| Feature | Skills |
|---|---|
| `probes` | `ness-probes`, `ness-monitor` |
| `notify` | `ness-notify` |
| `contracts` | `ness-contracts` |

Next: [Files ness writes](./files.md).
