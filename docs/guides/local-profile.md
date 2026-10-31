---
title: Grow your own harness
description: Put your project's process in its local profile, a directory ness renders after every other profile.
---

Your harness lives in the project's **local profile**: `.ness/profiles/local/`. It holds your
skills, the text you add to the core skills, blocks for `AGENTS.md`, seeded files and checks.
`ness sync` renders it last, after core, the features and every other profile.

## Prerequisites

- A project set up with `ness init` and committed.
- The format reference, for every key: [Profile format](../reference/profile-format.md).

## Create the profile

1. Make the directory and its manifest. Name it `local`: only the profile of that name is
   applied last and may replace a single slot another profile set.

   ```toml title=".ness/profiles/local/profile.toml"
   name    = "local"
   summary = "How acme plans and ships"

   [slots]
   "ness-entry#stop-and-ask" = { append = "stop-and-ask.md" }
   "AGENTS.md#local"         = { block = "agents-local.md" }

   [files]
   ".agents/skills/acme-planning/SKILL.md" = { own = "skills/acme-planning/SKILL.md" }
   "plans/README.md"                       = { seed = "plans-readme.md" }
   ```

2. Write the skill. A skill is a directory with a `SKILL.md`. Its headings are the sections that
   checks and guards point at.

   ```markdown title=".ness/profiles/local/skills/acme-planning/SKILL.md"
   ---
   name: acme-planning
   description: How acme plans a task before a child opens for it.
   ---

   # Planning at acme

   ## Plans

   Every child works from a plan file, `plans/<scope>.md`, written before the child opens.
   The plan names the files to change and the test that proves the change.
   ```

3. Write the slot text, the block and the seed.

   ```markdown title=".ness/profiles/local/stop-and-ask.md"
   - A change to the billing schema: ask before you write it.
   ```

   ```markdown title=".ness/profiles/local/agents-local.md"
   ## Acme conventions

   Plans live in `plans/`. Read `.agents/skills/acme-planning/SKILL.md` before opening a child.
   ```

   ```markdown title=".ness/profiles/local/plans-readme.md"
   # Plans

   One file per scope: `plans/<scope>.md`.
   ```

4. Turn the profile on. A local profile runs only when `ness.toml` lists it.

   ```toml title=".ness/ness.toml"
   profiles = ["local"]
   ```

5. Render it.

   ```sh
   ness sync
   ```

   ```text title="Output"
   create .agents/skills/acme-planning/SKILL.md
   update .agents/skills/ness-entry/SKILL.md
   create AGENTS.md#local/local
   create plans/README.md
   ```

6. Commit the profile and everything sync wrote. Generated files are committed, so a change to
   your harness is reviewable as a diff.

## Check that it worked

The slot text sits at the end of the core section:

```sh
grep -n -B3 -A1 'billing schema' .agents/skills/ness-entry/SKILL.md
```

```text title="Output"
121-
122-## Stop and ask
123-Stop and ask your manager before anything you cannot undo.
124:- A change to the billing schema: ask before you write it.
125-
```

After the commit, `ness sync --check` exits 0 and prints nothing.

## What each entry does

| Entry | Effect | Sync owns it? |
|---|---|---|
| `{ append = FILE }` in `[slots]` | Adds FILE's text to a list slot, after core and the earlier profiles | The rendered skill, yes |
| `{ set = FILE }` in `[slots]` | Fills a single slot. Only `local` may replace text another profile set. | The rendered skill, yes |
| `{ block = FILE }` on `AGENTS.md#NAME` | A managed block `local/NAME` in `AGENTS.md` | Only the text between its markers |
| `{ own = FILE }` in `[files]` | A file ness writes and keeps current | Yes |
| `{ seed = FILE }` in `[files]` | A file created once if missing, then yours | No |

The slots of the core skills:

| Slot | Kind | Section it fills |
|---|---|---|
| `ness-entry#sizes` | list | Work sizes |
| `ness-entry#autonomy-default` | single | Autonomy: the level a session uses when its brief states none |
| `ness-entry#stop-and-ask` | list | Stop and ask |
| `ness-entry#reserved-decisions` | list | Reserved decisions |
| `ness-entry#specify`, `#plan`, `#handoff` | list | Full session, Lifecycle |
| `ness-entry#post-compact` | list | After compaction: the rules printed again after a compaction |
| `ness-launch#tier-roles` | list | Tiers |

A skill your profile owns can carry its own slots. Put a `<!-- ness:slot list NAME -->` or
`<!-- ness:slot single NAME -->` line where the text goes. Built-in profiles have slots too; see
[Built-in profiles](../reference/builtin-profiles.md).

## Set config, features and atoms

A profile can set defaults for `.ness/ness.toml` keys, turn features on and choose atoms:

```toml title=".ness/profiles/local/profile.toml"
[config]
"integrate.push" = true

[features]
enable = ["contracts"]

[atoms]
integrator = "squash"
```

A value written in `.ness/ness.toml` wins over a profile default. Two profiles that disagree on a
key, a seam or a single slot are an error until `ness.toml` (or, for a slot, `local`) settles it.
There is no "last wins".

## Pick where a rule lives

Prose is a guide. A rule that must hold also gets a sensor, chosen by where the breach can be
seen:

| The breach shows up | Use | Guide |
|---|---|---|
| In the code that lands | The gate, a command `ness integrate` runs once | [Give a rule a check](./checks.md) |
| When a session opens, integrates or closes | A `[[checks]]` entry in the profile | [Give a rule a check](./checks.md) |
| In a tool call, before it runs | A `[[guards]]` entry in `ness.toml` | [Give a rule a check](./checks.md) |
| In a file that must not drift | A frozen contract | [Freeze acceptance files](./contracts.md) |
| At commit or push time, on one clone | A git hook, in front of the gate | [Give a rule a check](./checks.md) |
| In the running world, after the merge | A probe | [Ask the running world a question](./probes.md) |

Some rules need judgement, such as whether a name is clear. Say in the skill section that no
check enforces the rule, so nobody looks for one.

## Limits

- The local profile cannot override an owned path or a seam another profile chose. Settle a seam
  in `.ness/ness.toml`.
- A local profile with the name of a built-in is refused.
- The lock records the local profile's version as the SHA-256 of its directory, so any edit to it
  shows up in `ness sync --check`.

Next: [Give a rule a check](./checks.md).
