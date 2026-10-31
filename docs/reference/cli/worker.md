---
title: "ness worker"
description: "The project's worker: run it here, or install it as a systemd user unit"
---

## Usage

```text
ness worker
```

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |

## Subcommands

| Command | Description |
|---|---|
| [ness worker install](./worker-install.md) | Write and start the systemd user unit ness-\<project>.service |
| [ness worker run](./worker-run.md) | Run the worker in this terminal: one process per project, a pass every 5 s |
| [ness worker uninstall](./worker-uninstall.md) | Stop, disable and remove the unit |
