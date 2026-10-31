---
title: "ness secret"
description: "Keep the values ness.toml references as secret:NAME"
---

## Usage

```text
ness secret
```

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |

## Subcommands

| Command | Description |
|---|---|
| [ness secret list](./secret-list.md) | List the stored names, marking those ness.toml references |
| [ness secret set](./secret-set.md) | Store a secret read from stdin (never from the command line) |
| [ness secret unset](./secret-unset.md) | Remove a stored secret |
