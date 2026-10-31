---
title: "ness open"
description: "Launch a child or a root session, or resume one"
---

## Usage

```text
ness open [flags]
```

## Examples

```sh
# a root in its own tab and worktree
ness open --root --scope api --tier standard --size full --brief brief.md
# a child of this session
ness open --scope fix-login --tier economy --brief child.md
# relaunch an exited session
ness open --resume s-0123456789ab
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--base REF` |  | base REF of a root (default: the configured base) |
| `--brief FILE` |  | brief FILE, copied to the session's record directory |
| `--client string` |  | client for this launch: claude or codex (default: the configured client) |
| `--effort string` |  | effort for this launch (default: the tier's) |
| `--here` |  | with --root: register this running conversation as the root |
| `--kind NAME` |  | work NAME for the declared selector (see ness pick) |
| `--reason TEXT` |  | why an explicit --tier is above the recommended rung (TEXT) |
| `--resume ID` |  | relaunch an exited session ID or continue its stalled launch |
| `--root` |  | open a root (operator only) |
| `--scope NAME` |  | the session's scope NAME (required; [a-z][a-z0-9-]\{0,31}) |
| `--size string` |  | work size of a root: quick, full or a profile's (default: full) |
| `--tier string` |  | rung from the ladder, or up/down with --resume (default: the selector's) |
| `--title string` |  | tab title (default: the scope) |

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |
