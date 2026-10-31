---
title: "ness pick"
description: "Show the rung the selector picks for a task, or the ladder"
---

## Usage

```text
ness pick [flags]
```

## Examples

```sh
# the rung for a search subagent
ness pick --kind search
# a named agent's rung, as JSON
ness pick --agent ness-design --json
# the ladder, base and ceiling
ness pick
# judge a task text (policy jev or command)
ness pick --stdin --kind implement < task.md
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--agent TYPE` |  | named subagent TYPE (implies its kind) |
| `--explain` |  | also print the Jev answers (policy jev) |
| `--json` |  | print the pick as JSON |
| `--kind NAME` |  | work NAME (search, implement, design, ...) |
| `--role string` | `subagent` | who runs the task: subagent or child |
| `--stdin` |  | read the task text from stdin |

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |
