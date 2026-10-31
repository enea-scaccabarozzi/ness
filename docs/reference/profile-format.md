---
title: Profile format
description: Every key of profile.toml, the slot markers, and the fields of a check.
---

A profile is a directory with a `profile.toml` and the files it names. Built-in profiles ship in
the binary.

The local profile lives in `.ness/profiles/local/`. Any other directory under `.ness/profiles/`
is a project profile that loads by its name.

File paths in a manifest are relative to the profile directory. A missing file is an error at
sync.

## Manifest keys

| Key | Type | Meaning |
|---|---|---|
| `name` | string, required | The profile name. It must equal the directory name. The local profile is `local`, the only one applied last. |
| `summary` | string | One line that says what the profile does |
| `requires` | list of profile names | Profiles that must also be applied |
| `conflicts` | list of profile names | Profiles that must not be applied with this one |
| `requires_tools` | list of program names | Programs that must be on `PATH`, checked at sync and by doctor |
| `sizes` | list of size names | Work sizes added to `quick` and `full`. Names match `[a-z][a-z0-9-]{0,31}`. |
| `[slots]` | table | Text for slots in skills and blocks in `AGENTS.md` |
| `[files]` | table | Owned and seeded files |
| `[config]` | table | Defaults for `.ness/ness.toml`, as dotted keys |
| `[features] enable` | list | Features to turn on: `probes`, `notify`, `contracts` |
| `[atoms]` | table | Seam choices |
| `[[checks]]` | array of tables | Checks run on open, integrate and close |

## `[slots]`

The key is `TARGET#SLOT`. TARGET is a skill name, `AGENTS.md`, or the path of any file a profile renders, such as `.golangci.yml`.

| Value | Slot kind | Effect |
|---|---|---|
| `{ append = FILE }` | list | Adds the text after core's and earlier profiles'. Two profiles may both append. |
| `{ set = FILE }` | single | Fills the slot. Two profiles that set the same slot are an error, except `local`, which wins. |
| `{ block = FILE }` | `AGENTS.md#NAME` | A managed block `PROFILE/NAME` between `<!-- ness:begin PROFILE/NAME -->` and `<!-- ness:end PROFILE/NAME -->` |

A slot is a marker line in a skill. The text goes above the marker:

```markdown title="A skill with slots"
## Stop and ask
Stop and ask your manager before anything you cannot undo.
<!-- ness:slot list stop-and-ask -->
```

`<!-- ness:slot list NAME -->` takes appends. `<!-- ness:slot single NAME -->` takes one `set`.
A slot key that names no slot in its target is an error.

## `[files]`

The key is the path in the project.

| Value | Effect |
|---|---|
| `{ own = FILE }` | ness writes the file and keeps it current at every sync |
| `{ seed = FILE }` | ness creates the file once if it is missing. Then it is yours. |

Two profiles that own the same path are an error.

## `[config]`, `[features]`, `[atoms]`

```toml title="profile.toml"
[config]
"integrate.push" = true

[features]
enable = ["contracts"]

[atoms]
integrator = "squash"
```

| Seam | Config key | Values |
|---|---|---|
| `client` | `client` | `claude`, `codex` |
| `integrator` | `integrate.mode` | `merge`, `squash`, `rebase`, `pr` |
| `model_policy` | `models.policy` | `declared`, `command`, `jev` |

A value in `.ness/ness.toml` wins over a profile's. Two profiles that disagree on a config key or
a seam are an error until `ness.toml` sets it. The local profile cannot override a seam.

## `[[checks]]`

| Key | Required | Meaning |
|---|---|---|
| `id` | yes | Unique within the profile. Reported as `PROFILE/ID`. |
| `on` | yes | `open`, `integrate` or `close` |
| `for` | no | `child` or `root`. Absent: both, the operator included. |
| `size` | no | A non-empty list of sizes. The check runs only when the root's size is listed. |
| `skill` | yes | The skill that states the rule |
| `section` | yes | A heading of that skill. Resolved at sync. |
| `message` | with `kind` | The refusal text. A `run` check may leave it out; its refusal then reads `PROFILE/ID failed`. |
| `run` | one of `run`, `kind` | A shell command, run in the checkout examined. Exit 0 passes. |
| `timeout` | no | For `run`: a duration, default `30s` |
| `kind` | one of `run`, `kind` | A built-in kind, with its arguments |

Kinds and their arguments:

| Kind | Arguments |
|---|---|
| `section-matches` | `file`, `heading`, `require` (a list of regexps) |
| `branch-forbids` | `path`, `pattern` (regexps). `{record}` in `path` is the session's record directory. |
| `match-last` | `file`, `pattern`, `against_file`, `against_pattern` (group 1 of each) |
| `checkpoint-since-landing` | none |

A `run` check gets the variables in [Environment](./environment.md). Its output, cut to 1,500
bytes, is the detail line. A failed check refuses the event with exit 4.

## Order

Profiles apply in the order `profiles` lists them, after core and the features. `local` applies
last. A `conflicts` or `requires` problem is reported before any file is written.

Next: [Probe format](./probe-format.md).
