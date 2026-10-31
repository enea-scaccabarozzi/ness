---
title: Configuration
description: Every key of .ness/ness.toml, its type, its default and what reads it.
---

ness reads project configuration only from `.ness/ness.toml`. An unknown key, a wrong type or a
value outside its set is refused, naming the key. Profiles can set defaults for any key through
their `[config]` table; a value in `ness.toml` wins.

## Top level

| Key | Type | Default | Meaning |
|---|---|---|---|
| `ness` | string, required | none | The version pin, `"MAJOR.MINOR"`. `"0.2"` accepts `0.2.x` and `0.3.x` binaries; `"1.2"` accepts `1.2` and any later `1.x`. `ness upgrade` sets it. See [Compatibility](../concepts/compatibility.md). |
| `project` | string, required | The repository directory name, at `ness init` | The project name. It names the worker unit and the secrets file. |
| `base` | string, required | none | The base branch for roots and their integration |
| `gate` | string, required | none | The gate command `ness integrate` runs once per fold |
| `sessions` | string | `"docs/sessions"` | The directory of session records |
| `client` | string | `"claude"` | `claude` or `codex`. `codex` needs `experimental = ["codex"]`. |
| `profiles` | list of strings | `[]` | Profiles to apply. `local` is applied last wherever it sits. |
| `features` | list of strings | `[]` | `probes`, `notify`, `contracts` |
| `experimental` | list of strings | `[]` | Opt-ins to experimental atoms: `codex` |

## `[worktree]`

| Key | Type | Default | Meaning |
|---|---|---|---|
| `copy` | list of paths | `[]` | Copied from the primary checkout into each new session worktree |
| `link` | list of paths | `[]` | Symlinked from the primary checkout into each new session worktree |

## `[integrate]`

| Key | Type | Default | Meaning |
|---|---|---|---|
| `mode` | string | `"squash"` | `squash`, `merge`, `rebase` or `pr` |
| `push` | bool | `false` | Push the target to `origin` after a successful fold |
| `setup` | string | `""` | A shell command run in the gate worktree before the gate |

## `[contracts]`

Read from the merge-base copy of `ness.toml`, so a branch cannot change its own rules.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `frozen` | list of paths or globs | `[]` | Project-wide frozen paths. `path.Match` globs, no `**`. |
| `prefix` | list of globs | `[]` | `.md` files frozen as an append-only prefix |
| `formats.NAME.match` | list of globs, required in a format | none | Files read as format NAME. NAME matches `[a-z][a-z0-9-]*`. |
| `formats.NAME.units` | string, required in a format | none | A command that reads the file on stdin and prints one `{"id","body"}` JSON line per unit |

## `[probes]`

| Key | Type | Default | Meaning |
|---|---|---|---|
| `redact_env` | list of variable names | `[]` | Values of these variables are masked in probe evidence |

## `[notify]`

| Key | Type | Default | Meaning |
|---|---|---|---|
| `operator` | string | `""` | The channel the worker alerts you on when a session needs you. `""` alerts nobody. It needs `notify` in `features`, and a github channel is refused. |
| `channels.NAME.kind` | string, required | none | `slack`, `telegram`, `github` or `command` |
| `channels.NAME.token` | reference | none | `secret:NAME` or `env:VAR`, never the token. Required for slack, telegram and github. |
| `channels.NAME.channel_id` | string | none | Slack: the channel id. Required for slack. |
| `channels.NAME.chat_id` | string | none | Telegram: the chat id. Required for telegram. |
| `channels.NAME.repo` | string | none | GitHub: `owner/name`. Required for github. |
| `channels.NAME.command` | string | none | Command: the program, run with `sh -c` in the project root |

A literal token is refused. Any channel value may be a reference instead of a literal.

## `[tiers.NAME]`

One table per rung. Each value is `"MODEL/EFFORT"`.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `claude` | string | The built-in rung's | Claude model and effort. Efforts: `low`, `medium`, `high`, `xhigh`, `max`. |
| `codex` | string | The built-in rung's | Codex model and effort. Efforts: `minimal`, `low`, `medium`, `high`, `xhigh`. |

The built-in rungs and their models are in [The ladder](../concepts/models.md#the-ladder).

## `[models]`

| Key | Type | Default | Meaning |
|---|---|---|---|
| `policy` | string | `"declared"` | The selector: `declared`, `command` or `jev` |
| `base` | string | `"standard"` | The rung offsets count from, and every fallback |
| `ceiling` | string | `""` | The highest rung any pick reaches. `""` is the top. Must not be below `base`. |
| `ladder` | list of rung names | The seven built-in rungs | The rungs, cheapest first |
| `kinds.NAME` | integer | See below | A work kind's offset from `base`. Entries merge with the defaults. |
| `command` | string | `""` | The command selector's program (`policy = "command"`) |
| `command_timeout` | duration | `"10s"` | The bound on one run of `command` |
| `router_threshold` | float, 0 to 1 | `0.8` | A verdict below this confidence gives the base rung |
| `fallback_model` | string | `""` | Passed to the client as `--fallback-model` when set |
| `enforce` | string | `"refuse"` | `refuse` or `log`: what the hook does with a subagent call it would refuse. `log` lets it run and records `would-refuse`. |

The default kinds and their offsets are in [Kinds](../concepts/models.md#kinds).

## `[models.jev]`

The `jev` selector. Any failure falls back to `declared`.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `model` | string | `"jev-1.14.0"` | A pinned version. Aliases such as `jev-latest` are refused. |
| `key` | reference | `"env:TYPESAFE_API_KEY"` | `env:VAR` or `secret:NAME`, never a literal |
| `endpoint` | string | The Jev service's API | `https://`, or `http://` to a loopback host only |
| `timeout` | duration | `"5s"` | The bound on one request, retry included. At most `10s`. |

## `[launch]`

| Key | Type | Default | Meaning |
|---|---|---|---|
| `ready_timeout` | duration | `"60s"` | How long `ness open` waits for the client to be ready |
| `permissions` | string | `"skip"` | `skip` (the client's skip-permissions mode) or `client` (its own settings) |
| `max_children` | integer | `0` | Children open at once across the project. `0` is half the cores. |

A session cannot edit these keys, `[tiers]` or `[worker]` with Edit, Write or MultiEdit: a built-in guard refuses the edit. A shell write passes it.

## `[worker]`

| Key | Type | Default | Meaning |
|---|---|---|---|
| `stall_minutes` | integer | `30` | Idle minutes with open work before the worker sends one stall report to the session's manager. If nothing reaches the session in the next `stall_minutes`, your phone is told. |
| `quiet_seconds` | integer | `60` | Seconds after a prompt box last changed before the worker types into it |

## `[update]`

How this machine hears about new releases (R-UPD-030). The binary is shared, so the worker
installs on its own only when every registered project says `install`.

| Key | Type | Default | Meaning |
|---|---|---|---|
| `check` | bool | `true` | The worker and `ness doctor` check the release source, at most once per 24 h per machine. `false`: no network, and no result shown. |
| `channel` | string | `"stable"` | `stable` or `rc`. The channel `ness self-update` installs from, and the one the check reports. `rc` also offers a newer stable release. |
| `auto` | string | `"notify"` | `off`, `notify` or `install`. `notify`: `ness doctor` and the root's session-start show a newer release. `off`: they show nothing, and the worker still checks. `install`: the worker installs it and runs `ness upgrade` at the idle point. |

A value outside its set is refused, naming the key and the accepted values:

```text title="Output (exit 4)"
ness: .ness/ness.toml: update.channel = "beta": accepted values are stable, rc
```

## `[[guards]]`

One table per guard, checked by the pre-tool hook.

| Key | Type | Required | Meaning |
|---|---|---|---|
| `tool` | regexp | yes | Matches the tool name |
| `match` | regexp | yes | Matches the Bash command, or the edited file path |
| `content` | regexp | no | Matches the new text of an edit |
| `message` | string | yes | The refusal text |
| `skill` | string | yes | The skill that states the rule |
| `section` | string | yes | The heading in that skill |

Durations are Go durations (`30s`, `5m`, `1h`).

Next: [Profile format](./profile-format.md).
