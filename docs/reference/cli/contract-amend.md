---
title: "ness contract amend"
description: "Open frozen files for change; the reason goes to decisions.md, one line each"
---

## Usage

```text
ness contract amend PATHS... [flags]
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--authorized WORDS` |  | a project-wide path: WORDS the operator typed in this conversation approving the change |
| `--item string` |  | the item from your manager or the operator approving the change |
| `--reason string` |  | why the file changes (recorded in decisions.md) |

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |
