---
title: "ness probe start"
description: "Register probe files or a directory of them; you own them"
---

## Usage

```text
ness probe start PATH... [flags]
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--keep-window` |  | on a restart, keep the original window start |
| `--label string` |  | name this start (a release, a sha) so the same file can run once per label; the floor sees NESS_PROBE_LABEL |

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |
