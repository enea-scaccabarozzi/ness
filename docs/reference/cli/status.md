---
title: "ness status"
description: "The session tree: tier, phase, context, idle time, obligations; probes; the worker"
---

## Usage

```text
ness status [flags]
```

## Examples

```sh
# the open sessions
ness status
# a live view: select a session for its branch, worktree and waits
ness status --watch
# one JSON object, for scripts
ness status --json
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--all` |  | also list closed sessions |
| `--json` |  | print one JSON object |
| `--watch` |  | a live view, refreshed every 2s (rich renderer only) |

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |
