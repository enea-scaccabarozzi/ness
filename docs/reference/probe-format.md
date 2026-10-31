---
title: Probe format
description: The frontmatter fields of a probe file, their values and defaults, and how a floor is run.
---

A probe is one Markdown file. The frontmatter between `---` lines holds flat `key: value` pairs.
The body is the scope: the question the woken owner judges. A comment after `#` is dropped.

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

## Fields

| Field | Value | Default | Meaning |
|---|---|---|---|
| `every` | duration | none | Run at this interval |
| `jitter` | duration | `0` | A random delay added to each run, within this bound |
| `once` | `true` or `false` | `false` | Run one time, within `jitter`, then stop |
| `window` | duration | none | Stop after this long, with one final wake for a verdict. Without it, run until stopped. |
| `floor` | shell command | none | The cheap check. Without it, every run wakes the owner. |
| `expect` | conditions | none | Conditions on the floor's last output line |
| `confirm` | integer ≥ 1 | `1` | Consecutive non-GREEN runs before a wake |
| `timeout` | duration | `60s` | The floor's time limit. Past it, the floor is killed and the run is UNCLEAR. |
| `wake` | `never`, `change`, `red`, `not-green`, `always` | `not-green` | Which runs wake the owner |
| `sample` | number in [0, 1] | `0` | Also wake on this fraction of GREEN runs |
| `notify` | `none`, `auto`, `judged` | `none` | `auto` posts state changes, `judged` posts verdicts |
| `channel` | channel name | none | The `[notify.channels]` entry. Required with `auto` or `judged`. |

A probe needs `every` or `once`. Durations are Go durations, such as `90s`, `5m` or `1h30m`, plus `d` for days. An unknown field
is refused, naming the file and the field:

```text title="Output (evry: 30m, exit 4)"
ness: probes/disk.md: unknown field evry
```

`once` is a boolean. `once: 1h` is a parse error.

## `expect`

A comma-separated list of `KEY OP NUMBER` conditions. OP is one of `>=`, `<=`, `>`, `<`, `==`,
`!=`. The floor's last output line carries `key=value` pairs, such as `used=45 inodes=12`. A
condition that fails, or a key that is missing, makes the run VACUOUS.

## States

| Floor result | State |
|---|---|
| Exit 0, `expect` holds or is unset | GREEN |
| Exit 0, `expect` fails | VACUOUS |
| Exit 1 | RED |
| Any other exit, or a timeout | UNCLEAR |

## How a floor runs

- `sh -c FLOOR`, in the owner's worktree.
- With `NESS_PROBE` (the probe path), `NESS_SESSION` (the owner) and `NESS_PROBE_LABEL` (the
  label given to `ness probe start --label`, if any).
- The evidence kept is the last 4 KB of output, with the values of `[probes] redact_env` masked.
- At most 8 floors run at once.

Labels match `[A-Za-z0-9][A-Za-z0-9._-]{0,31}`.

## Commands

| Command | Exit |
|---|---|
| `ness probe run PATH` | 0 on GREEN, 1 otherwise. Records nothing. `--json` prints `schema`, `state`, `exit`, `timed_out`, `evidence`. |
| `ness probe start PATH... [--label L] [--keep-window]` | 0. Needs a session as owner, otherwise 4. `--keep-window` keeps the original window start on a restart. |
| `ness probe verdict ITEM --state green\|red --note TEXT` | 0 |
| `ness probe verdict ITEM --probe-bug --note TEXT` | 0 |
| `ness probe stop PATH [--label L] --note TEXT` | 0. `--label` is needed when several labels of PATH are live. |

Next: [Built-in profiles](./builtin-profiles.md).
