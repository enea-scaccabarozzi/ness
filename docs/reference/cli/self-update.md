---
title: "ness self-update"
description: "Install a newer ness release in place of this binary"
---

## Usage

```text
ness self-update [flags]
```

## Examples

```sh
# the newest release on the project's channel (stable outside a project)
ness self-update
# only say whether a newer release exists
ness self-update --check
# install a specific release
ness self-update --version 0.2.0
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--channel channel` |  | release channel: stable or rc (default: the project's [update] channel) |
| `--check` |  | report whether a newer release exists; install nothing |
| `--version X.Y.Z` |  | install release X.Y.Z instead of the newest |

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |
