---
title: Profiles and sync
description: A profile is your process as files in the repository. ness sync renders the core, the features and your profiles, and never overwrites what you wrote.
---

A **profile** is a directory with a `profile.toml` and the files it renders: skills, slot text,
whole files, config defaults and checks. It is how you turn the shared machinery into your
harness. **Sync** is the command that renders the profiles into your repository, `ness sync`.
It records every byte it wrote in `.ness/lock.json`, so it can tell its own text from yours.

```text
core ──▶ features ──▶ profiles, in listed order ──▶ local profile ──▶ .ness/ness.toml
                                                                        │
                                       ness sync renders, compares with .ness/lock.json, writes
```

Later layers win only where the rules below allow it. Everything ness writes is committed, so an
upgrade arrives as a diff you review.

## What a profile can do

| Section of `profile.toml` | What it does |
|---|---|
| `[slots]` | Adds text to a named slot in a skill or file: `append` to a list slot, `set` a single slot, or add a `block` to `AGENTS.md` |
| `[files]` | Adds a file: `own` (ness keeps it current) or `seed` (created once if missing, then yours) |
| `[config]` | Sets defaults for `.ness/ness.toml` keys. A value in `ness.toml` wins. |
| `[features]` | Turns on `probes`, `notify` or `contracts` |
| `[atoms]` | Chooses the client, the integrator or the model selector: the keys `client`, `integrator` and `model_policy` |
| `[[checks]]` | Adds a check that runs at `ness open`, `ness integrate` or `ness close` |
| `sizes` | Adds a work size next to `quick` and `full` |
| `requires`, `conflicts` | Names profiles that must, or must not, be applied with it |
| `requires_tools` | Names tools that must be on `PATH`. Sync fails before writing when one is missing. |

ness ships five built-in profiles, embedded in the binary: `program`, `go-strict`,
`behavior-specs`, `git-conventional` and `example-release`. You list them in `ness.toml`:

```toml title=".ness/ness.toml"
profiles = ["go-strict", "behavior-specs", "local"]
```

## Slots

[Slots](../why/defaults.md) are where a profile adds text. A slot sits in a skill or in any other
file a profile renders, such as `AGENTS.md` or `.golangci.yml`. You address it as
`<skill-or-file>#<slot>`.

| Kind | Rule | Examples |
|---|---|---|
| list | Every profile may append. Entries keep the profile order. | `ness-entry#stop-and-ask`, `ness-entry#reserved-decisions`, `ness-entry#sizes`, `ness-entry#specify`, `ness-entry#plan`, `ness-entry#handoff`, `ness-entry#post-compact`, `ness-launch#tier-roles` |
| single | One profile sets it. Two profiles setting it is an error. | `ness-entry#autonomy-default` |

A slot name that does not exist in its target is refused, naming the profile and the slot.

## The local profile

A project profile is a directory under `.ness/profiles/<name>/`, with any name that is not a
built-in's. The one named `local` is your project's own profile. It is a profile like the
built-ins, with three differences:

- It is applied last, wherever it sits in the list.
- It is the only profile that may replace a single slot another profile set.
- Its version in the lock is a hash of its content, so any edit shows as a change. This holds
  for every project profile.

It may not override seam choices or another profile's owned paths.

This local profile adds a rule to a core skill, a block to `AGENTS.md` and a seeded runbook:

```toml title=".ness/profiles/local/profile.toml"
name    = "local"
summary = "acme house rules"

[slots]
"ness-entry#stop-and-ask" = { append = "stop-and-ask.md" }
"AGENTS.md#local"         = { block = "agents-local.md" }

[files]
"docs/runbook.md" = { seed = "runbook.md" }
```

With `profiles = ["local"]` in `ness.toml`, `ness sync --dry-run` prints the plan and writes
nothing. The diff of the seeded file is trimmed:

```sh
ness sync --dry-run
```

```text title="Output"
update .agents/skills/ness-entry/SKILL.md
create AGENTS.md#local/local
create docs/runbook.md
--- .agents/skills/ness-entry/SKILL.md
+++ .agents/skills/ness-entry/SKILL.md
@@ -121,6 +121,7 @@
 
 ## Stop and ask
 Stop and ask your manager before anything you cannot undo.
+- Stop and ask before you change a database migration.
 
 ## Reserved decisions
 Decisions the operator keeps; everything else is yours within your brief. Raise one as a
--- AGENTS.md
+++ AGENTS.md
@@ -4,3 +4,8 @@
 
 This repository runs the ness harness. Start every session by loading the `ness-entry` skill.
 <!-- ness:end core/router -->
+
+<!-- ness:begin local/local -->
+## Acme
+Run `make test` for the package you touched.
+<!-- ness:end local/local -->
```

## Requires and conflicts

Profiles do not settle disagreements by order. There is no "last wins" between two profiles
other than `local`, and every conflict is found before any file is written.

| Two profiles both… | Result |
|---|---|
| append to a list slot | Fine. Both appear, in profile order. |
| set the same single slot | Error naming both, unless `local` sets it |
| choose different atoms for one seam | Error naming both, unless `ness.toml` sets the seam |
| set one config key to different values | Error naming both, unless `ness.toml` sets the key |
| claim the same owned path | Error. Nothing overrides it. |
| one names the other in `conflicts` | Error naming both |

A profile's `requires` never turns the other profile on for you. `behavior-specs` requires
`go-strict`, so `ness sync --dry-run` with `profiles = ["behavior-specs", "local"]` refuses:

```text title="Output"
ness: profile behavior-specs requires profile go-strict; add it to profiles in .ness/ness.toml
```

The command exits 4 and writes nothing.

## Safe sync

Sync never overwrites or deletes bytes it did not write. For each file, block and key it
compares three things: what is on disk, what the lock says it wrote, and what it would write now.
The [sync rules](../reference/files.md#sync-rules) list what it does in each case.

On a conflict, sync leaves your file alone, writes its render next to it as `<path>.ness-new`,
and exits 3. The other files still sync. You merge the two by hand, then tell sync to keep
your version:

```sh
ness sync
```

```text title="Output"
conflict .agents/skills/ness-entry/SKILL.md: merge .agents/skills/ness-entry/SKILL.md.ness-new into it, then run ness sync --keep .agents/skills/ness-entry/SKILL.md
```

```sh
ness sync --keep .agents/skills/ness-entry/SKILL.md
```

```text title="Output"
kept .agents/skills/ness-entry/SKILL.md
```

Later syncs leave the kept file untouched until the upstream render changes again. Use
`ness sync --check` in your gate: it writes nothing, exits 1 when a sync would change something,
and exits 3 on a conflict.

## Managed blocks

In a file you share with ness, such as `AGENTS.md` or `.gitignore`, ness owns only the text
between its markers:

```markdown title="AGENTS.md"
<!-- ness:begin core/router -->
...
<!-- ness:end core/router -->
```

Files with `#` comments use `# ness:begin <owner>/<block>`. The owner is `core` or the
profile's name.

Sync finds a block wherever you moved it and updates it in place. A new block is
appended at the end of the file. Every byte outside the blocks stays as you wrote it.

JSON and TOML files work the same way at the key level. In `.claude/settings.json` ness owns
its hook entries, `autoCompactWindow` and `env.CLAUDE_CODE_SUBAGENT_MODEL`, and leaves every
other key and your own hooks in place.

## Links

- [Guides and sensors](./guides-and-sensors.md): slots and checks as guides and sensors.
- [Grow your own harness](../guides/local-profile.md): write a local profile step by step.
- [Give a rule a check](../guides/checks.md): `[[checks]]` in detail.
- [Profile format](../reference/profile-format.md): every key of `profile.toml`.
- [Built-in profiles](../reference/builtin-profiles.md): what the five built-ins add.
- [Files](../reference/files.md): every file ness writes, and its sync rule.

Next: [Guides and sensors](./guides-and-sensors.md).
