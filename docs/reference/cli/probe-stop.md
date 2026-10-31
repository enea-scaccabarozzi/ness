---
title: "ness probe stop"
description: "Stop your probe at PATH (--label X when it runs under several labels) and record why"
---

## Usage

```text
ness probe stop PATH [flags]
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--label string` |  | the label the probe was started with; needed when several are live |
| `--note string` |  | why the probe stops (recorded) |

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |
