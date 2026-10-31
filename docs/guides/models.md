---
title: Pick the model for each job
description: Read the rung ladder, shift it for your project, and plug in your own selector for subagents and children.
---

ness picks the model and effort for each job from the [ladder](../concepts/models.md) of seven
rungs, cheapest first. A job starts at the base rung and moves by its kind: a search moves down,
a review moves up. The pre-tool hook refuses a subagent call above its recommended rung.

## Prerequisites

- A project set up with `ness init`.

## Read the ladder

```sh
ness pick
```

```text title="Output"
bulk      haiku/medium   gpt-6-luna/low
economy   sonnet/medium  gpt-6-luna/medium
lean      opus/low       gpt-6.1-sol/low
standard  opus/medium    gpt-6.1-sol/medium    base
deep      opus/high      gpt-6.1-sol/high
frontier  fable/medium   astra/medium
apex      fable/high     astra/high            ceiling
```

Each row is a rung: its name, the Claude model and effort, the Codex model and effort.

Ask for a kind of work:

```sh
ness pick --kind review
```

```text title="Output"
deep      opus/high      kind review
model: opus  effort: high
```

[Kinds](../concepts/models.md#kinds) lists each kind's offset from base. `--kind sweep` means
`search`. An unknown kind is refused and the kinds are listed:

```text title="Output (exit 4)"
ness: name the work kind: one of debug, design, implement, research, review, run, search, verify
```

## How sessions use it

- Every subagent call carries a `kind: NAME` line and the model and effort `ness pick` gives.
- The pre-tool hook refuses a rung above the recommendation. An `escalate: REASON` line allows
  one rung more. It counts only in the `key: value` lines at the very start of the prompt.
- The built-in agents (`ness-design`, `ness-review`, `ness-sweep`, `ness-verify`) carry their
  rung in their frontmatter.
- A manager opens a child with `--kind`, or with `--tier NAME --reason TEXT`.
- `ness open --resume ID --tier up` relaunches a session one rung higher, once its client has
  exited. [Moving a session up a rung](../concepts/models.md#moving-a-session-up-a-rung) gives the
  steps.

## Shift the ladder for your project

1. Set the base, the ceiling and your own kinds.

   ```toml title=".ness/ness.toml"
   [models]
   base    = "economy"
   ceiling = "deep"

   [models.kinds]
   review  = 2
   migrate = 1
   ```

2. Run `ness sync`. It rewrites the four agents and the subagent model in
   `.claude/settings.json`.

### Check that it worked

```sh
ness pick --kind review
```

```text title="Output"
standard  opus/medium    kind review
model: opus  effort: medium
```

`ness pick` marks `economy` as base and `deep` as ceiling. No pick goes above the ceiling.

## Change a rung's models

`[tiers.NAME]` replaces a rung's models. Efforts for Claude are `low`, `medium`, `high`, `xhigh`
and `max`. Efforts for Codex are `minimal`, `low`, `medium`, `high` and `xhigh`.

```toml title=".ness/ness.toml"
[tiers.deep]
claude = "opus/xhigh"
```

`ness doctor` checks that every rung's model is one the client lists (`tier-models`).

## Plug in your own selector

A `command` selector decides the offset from the task text. It reads one JSON object on stdin,
`{"text", "role", "client", "kind", "agent_type"}`, and prints one answer.

```sh title="scripts/pick.sh"
#!/bin/sh
# A command selector: reads {"text","role","client","kind","agent_type"} on stdin,
# prints one {"offset","confidence","reason"} object.
in=$(cat)
case "$in" in
  *migration*) echo '{"offset": 2, "confidence": 0.9, "reason": "touches a migration"}' ;;
  *)           echo '{"offset": 0, "confidence": 0.9, "reason": "routine change"}' ;;
esac
```

```toml title=".ness/ness.toml"
[models]
base    = "economy"
policy  = "command"
command = "./scripts/pick.sh"
```

```sh
echo 'Add a column; this touches a migration.' | ness pick --stdin --kind implement
```

```text title="Output"
standard  opus/medium    touches a migration
model: opus  effort: medium
```

`ness pick --kind implement --json`, on a task without a migration, prints one object:

```json title="Output"
{
  "schema": 1,
  "rung": "economy",
  "model": "sonnet",
  "effort": "medium",
  "index": 1,
  "confidence": 0.9,
  "reason": "routine change"
}
```

A selector error, a timeout (`command_timeout`, 10 seconds) or a confidence below
`router_threshold` (0.8) gives the base rung, with a reason that starts `base:`.

[Selectors](../concepts/models.md#selectors) describes the three policies. The `jev` policy
reads its key from `[models.jev] key`.

## Log instead of refuse

While you tune the ladder, `enforce = "log"` lets every subagent call through and records the
verdict the hook would have given as `would-refuse`. Read the picks back with `ness stats --by rung`.

Next: [Run sessions on Codex](./codex.md).
