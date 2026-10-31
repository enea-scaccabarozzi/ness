---
title: Environment and exit codes
description: The environment variables ness reads and sets, and what each exit code means.
---

ness changes behaviour only on the variables below, the variables your `env:` references name,
and the values listed in `[probes] redact_env`. Configuration lives in `.ness/ness.toml`, not in
the environment.

## Variables ness reads

| Variable | Effect |
|---|---|
| `NESS_UI` | `auto` (default), `rich` or `plain`. The `--ui` flag wins. Another value exits 2. |
| `NESS_THEME` | `dark` or `light`: the rich palette, instead of asking the terminal |
| `NO_COLOR` | No colour in rich output. The layout stays. |
| `CLICOLOR_FORCE` | Colour with the rich renderer even when stdout is not a terminal. Under `auto`, a pipe gets the plain renderer, so it takes `--ui rich` or `NESS_UI=rich` too. `NO_COLOR` wins. |
| `TERM` | `dumb` selects the plain renderer |
| `CI` | Set: the plain renderer |
| `COLUMNS` | The width for rich output when the terminal reports none. Else 100. |
| `ACCESSIBLE` | Set: no spinner or full-screen view, and `ness init` prompts one line at a time |
| `NESS_SESSION` | The session this process acts as. Set in every session's tab. |
| `CLAUDECODE`, `CLAUDE_CODE_SESSION_ID` | Mark an agent: the plain renderer, and the client session `ness open --here` promotes |
| `HERDR_PANE_ID`, `HERDR_TAB_ID`, `HERDR_WORKSPACE_ID` | The terminal pane, tab and workspace. `--here` needs the pane and tab. New tabs open in the workspace. |
| `XDG_CONFIG_HOME` | Replaces `~/.config` for the secrets file |
| `GIT_SSH`, `GIT_SSH_COMMAND` | Either set: the `ness integrate` push adds no ssh keepalive option, so your wrapper or command applies |

An agent always gets the plain renderer, whatever `NESS_UI` says, because agents inherit your
environment. The commands agents read, `hook`, `post`, `contract` and `probe`, are always plain,
even with `--ui rich`, and a bad `NESS_UI` does not fail them. Outside an agent:

```sh
NESS_UI=fancy ness status
```

```text title="Output (exit 2)"
ness: unknown ui mode "fancy"; accepted: auto, rich, plain
```

## Variables ness sets

| Variable | Set for | Value |
|---|---|---|
| `NESS_SESSION`, `NESS_RUN` | A session's tab | The session id, and its run number |
| `NESS_BASE` | `[integrate] setup` and the gate | The commit the branch is folded onto |
| `NESS_EVENT` | A profile `run` check | `open`, `integrate` or `close` |
| `NESS_SESSION`, `NESS_ROOT` | A profile `run` check | The acting session and its root, empty for the operator |
| `NESS_WORKTREE`, `NESS_PRIMARY` | A profile `run` check | The checkout examined, and the primary checkout |
| `NESS_SCOPE` | A `run` check on `open` | The scope being opened |
| `NESS_BRANCH`, `NESS_INTO`, `NESS_RECORD` | A `run` check on `integrate` and `close` | Source branch, target branch, record directory |
| `NESS_PROBE`, `NESS_SESSION`, `NESS_PROBE_LABEL` | A probe floor | The probe path, its owner, its label |
| `NESS_THREAD_KEY` | A `command` channel | The `--thread` key, empty without one |

## Exit codes

| Code | Meaning | Examples |
|---|---|---|
| 0 | Success | Any command that did its work. `doctor` with warnings only. |
| 1 | A check found problems | `sync --check`, `doctor`, `contract check`, `probe run` with a state other than GREEN, a failing gate in `integrate` |
| 2 | Usage error | An unknown command or flag, a missing or invalid argument, a bad `NESS_UI` |
| 3 | Sync conflict | `sync` or `sync --check` with a `.ness-new` to settle. Wins over 1. |
| 4 | Refused | An unmet precondition or unsupported environment: a failed profile check, a version outside the pin, a command outside a project |

A refusal writes nothing: no file, store row or runtime change. `ness hook pre-tool` is the one
exception to the table: it blocks a tool call with exit 2, as the client requires.

```sh
ness pick --bogus
```

```text title="Output (exit 2)"
ness: unknown flag: --bogus
```

Next: [Built with ness](../built-with-ness/index.md).
