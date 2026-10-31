---
title: Steer a multi-week program
description: Turn on the program profile, so a root keeps a map of the work, runs children in waves and checkpoints with you.
---

A **program** is work too big for one session. Its root owns a map, the roadmap, and opens
children in waves, one per vertical. You accept the map, set the autonomy for the run, and read
checkpoints in the root's chat.

The built-in `program` profile adds the size `program`, the
`ness-program` skill, seeded templates and four checks.

## Prerequisites

- A project set up with `ness init`, committed.
- The worker installed (`ness worker install`), so the root wakes when a child reports.

## Turn the profile on

1. List `program` in `.ness/ness.toml`.

   ```toml title=".ness/ness.toml"
   profiles = ["program"]
   ```

   If you have a [local profile](./local-profile.md), list it too: `["program", "local"]`. A
   name with no profile behind it is refused with exit 4.

2. Render it.

   ```sh
   ness sync
   ```

   ```text title="Output"
   update .agents/skills/ness-entry/SKILL.md
   create .agents/skills/ness-program/SKILL.md
   create docs/doctrine.md
   create docs/program/briefs/TEMPLATE.md
   create docs/program/decisions.md
   create docs/program/roadmap.md
   ```

3. Commit. The four files under `docs/` are seeds: created once, then yours.

| File | Holds |
|---|---|
| `docs/program/roadmap.md` | The map: Program doctrine, Invariants, Verticals (with a spec owner each), Waves, Conduct, Releases, Autonomy, Recovery |
| `docs/program/decisions.md` | The append-only ledger. Only the root numbers decisions: `- **D<n>** (date) decision; who decided; why.` |
| `docs/program/briefs/TEMPLATE.md` | The child brief: Question, Outcome, Invariants and doctrine, Operator words, Sources, Rights and limits, Tier, Spec owner, Integration target, How to report |
| `docs/doctrine.md` | Your principles in your words, ids `P<n>` |

## Start a program

Tell your agent what the program is for. It proposes a scope and the size `program` in one line.
On your yes, it promotes the conversation to a root with
`ness open --root --here --scope SCOPE --size program --brief FILE`. From a terminal you can open
one in its own tab:

```sh
ness open --root --scope payments --size program --brief brief.md
```

The size exists only while the profile is on. Without it, the same command is refused:

```text title="Output (exit 4)"
ness: size "program" is not one of: quick, full (skill ness-entry, section Work sizes)
```

## Work through the map

The root follows six steps from the `ness-program` skill:

1. Read back: it surveys with `ness-sweep` subagents and restates your brief as roadmap r1.
2. It decides only invariants at the root: what two or more verticals depend on, or what is
   expensive to reverse.
3. Everything else becomes a vertical, with its reason, its reopen trigger and its owner.
4. It refines the map with you, in rounds, until you accept it.
5. It runs waves of the verticals whose inputs exist. Each result folds back as a new map
   revision.
6. Children integrate into the root branch; the root releases.

## Set the autonomy

When you accept the map, the root asks how free it is for this run. Answer in your words. It
records the mode, your words and the date under `## Autonomy` in the roadmap.

| Mode | The root |
|---|---|
| gate | Stops at every checkpoint |
| bands | Works freely inside the bands you state, and stops on what you list |
| to checkpoint X | Works freely until X, then waits |
| custom | Follows your words |

## What the checks enforce

The four checks run only for children of a `program` root:

| Check | Event | Refuses |
|---|---|---|
| `autonomy-recorded` | open | Opening a child while `## Autonomy` lacks a mode word or a date |
| `checkpoint-before-wave` | open | Opening a child when one landed since the root's last checkpoint to you |
| `recovery-line` | open | Opening a child while the recovery line differs from the last `D<n>` of the ledger |
| `no-child-decision-ids` | integrate | A child branch that adds numbered decisions to its own record |

A fresh program fails only `autonomy-recorded`: the seeded recovery line already names `D1`.

## Check that it worked

- `ness size` in the root's session prints `size program (quick, full, program)`.
- Each landing is followed by a checkpoint in the root's chat, with Done, Now and Next.
- `ness status` shows the children of the current wave under the root.

## Change the defaults

The skill's Autonomy and Operator review format sections are single slots. Your local profile can
replace either:

```toml title=".ness/profiles/local/profile.toml"
[slots]
"ness-program#autonomy" = { set = "autonomy.md" }
```

Next: [Ask the running world a question](./probes.md).
