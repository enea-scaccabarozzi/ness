---
title: "ness doctor"
description: "Check this installation and project, read-only; print a fix for every failure"
---

## Usage

```text
ness doctor [flags]
```

## Examples

```sh
# every check, a fix under each failure
ness doctor
# the same, for scripts
ness doctor --json
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--json` |  | print one JSON object |

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |
