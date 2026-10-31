---
title: Ask the running world a question
description: Write a probe, a scheduled question about the deployed system, and choose when it wakes its owner.
---

A **probe** is a question about the running world, asked on a schedule or once, and owned by a
session. A cheap command, the **floor**, answers it first. The owner is woken only on the
verdicts you choose.

Use probes for facts that change after the code is merged. For conduct, use
a [check](./checks.md).

## Prerequisites

- The `probes` feature on:

  ```toml title=".ness/ness.toml"
  features = ["probes"]
  ```

  `ness sync` then renders the `ness-probes` and `ness-monitor` skills.
- The worker installed, to run the schedule.

## Write a probe

1. Write the floor. Its exit code gives the state. Its last line may carry `key=value` pairs.

   ```sh title="probes/disk.sh"
   #!/bin/sh
   # Prints used=<percent> for the filesystem holding this checkout; exits 0 when it can measure.
   used=$(df -P . | awk 'NR==2 {sub("%","",$5); print $5}')
   echo "used=$used"
   ```

2. Write the probe file. The frontmatter is the schedule and the policy. The body is the scope:
   what the woken owner judges.

   ```markdown title="probes/disk.md"
   ---
   every: 30m
   jitter: 5m
   floor: ./probes/disk.sh
   expect: used<90
   wake: not-green
   timeout: 20s
   ---
   Is the data disk below 90% full? When it is not, find what grew since the last run.
   ```

3. Run the floor once. `ness probe run` records nothing, wakes nobody and posts nothing.

   ```sh
   ness probe run probes/disk.md
   ```

   ```text title="Output (exit 0)"
   GREEN
   used=45
   ```

### Check that it worked

Make the expectation fail. With `expect: used<10` the floor still exits 0, but the run is not
GREEN:

```text title="Output (exit 1)"
VACUOUS
used=46
```

`ness probe run --json` prints a run as one object. Here the passing run of `used<90`:

```json title="Output"
{
  "evidence": "used=46\n",
  "exit": 0,
  "schema": 1,
  "state": "GREEN",
  "timed_out": false
}
```

## States

| State | When |
|---|---|
| GREEN | The floor exits 0 and `expect`, if set, holds |
| RED | The floor exits 1 |
| UNCLEAR | Any other exit, or the floor ran past `timeout` (it is killed) |
| VACUOUS | The floor exits 0 but `expect` fails, or a key it names is missing |

A probe without a floor has no state. It wakes its owner on every run, and the owner judges.

## Start, answer and stop

A probe needs a session as its owner. From a terminal, as the operator, `start` is refused:

```text title="Output (exit 4)"
ness: a probe needs a session as its owner; the operator owns none (run from a session)
```

Inside a session:

| Command | Does |
|---|---|
| `ness probe start PATH...` | Registers the probes (files or directories) with you as owner. Edits to the file are reloaded. |
| `ness probe start PATH --label v1.4.0` | A separate probe at the same path, for example one per release, with its own schedule, window, streak and stop. The floor sees `NESS_PROBE_LABEL`. |
| `ness probe verdict ITEM --state green\|red --note TEXT` | Answers a wake, records the verdict, posts it when `notify: judged` |
| `ness probe verdict ITEM --probe-bug --note TEXT` | Records a defect in the probe. Status shows `needs-edit` until the file changes. |
| `ness probe stop PATH --note TEXT` | Stops your probe. The note is required. |

When the owner closes, its probes pass to its manager. With no live session left to own it, a
probe pauses, shows red in `ness status` and is reported by `ness doctor`. It never runs headless.

## Choose when it wakes

| Field | Values | Default |
|---|---|---|
| `wake` | `never`, `change`, `red`, `not-green`, `always` | `not-green` |
| `confirm` | k consecutive non-GREEN runs before a wake | 1 |
| `sample` | Also wake on this fraction of GREEN runs, 0 to 1 | 0 |
| `notify` | `none`, `auto` (post state changes), `judged` (post verdicts) | `none` |
| `channel` | A `[notify.channels]` name, needed with `auto` or `judged` | none |

A wake is an item in the owner's inbox with the scope and the last 4 KB of the floor's output.
Values of the variables in `[probes] redact_env` are masked before the evidence is stored, sent
or printed.

Every field and its syntax: [Probe format](../reference/probe-format.md).

Next: [Post a note to a channel](./notify.md).
