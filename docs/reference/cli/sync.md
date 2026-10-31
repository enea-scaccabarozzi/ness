---
title: "ness sync"
description: "Render core, features and profiles into this repository"
---

## Usage

```text
ness sync [flags]
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--check` |  | write nothing; exit 1 if anything would change, 3 on conflict |
| `--dry-run` |  | write nothing; print the plan and a diff per changed file |
| `--json` |  | with --dry-run, print the plan as JSON |
| `--keep PATH` |  | keep your version of conflicted PATH until upstream changes again |

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |
