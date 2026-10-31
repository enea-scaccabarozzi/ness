---
title: "ness contract"
description: "Freeze files, open one for amendment, or check every frozen file (the gate)"
---

## Usage

```text
ness contract
```

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |

## Subcommands

| Command | Description |
|---|---|
| [ness contract amend](./contract-amend.md) | Open frozen files for change; the reason goes to decisions.md, one line each |
| [ness contract check](./contract-check.md) | Verify every frozen file; exit 1 naming each one that differs |
| [ness contract freeze](./contract-freeze.md) | Record each file's blob sha in your contract.lock |
