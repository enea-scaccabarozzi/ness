---
title: "ness probe"
description: "Start, run, answer or stop probes: scheduled questions about the running world"
---

## Usage

```text
ness probe
```

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |

## Subcommands

| Command | Description |
|---|---|
| [ness probe run](./probe-run.md) | Run the floor now and print the state and evidence; record nothing |
| [ness probe start](./probe-start.md) | Register probe files or a directory of them; you own them |
| [ness probe stop](./probe-stop.md) | Stop your probe at PATH (--label X when it runs under several labels) and record why |
| [ness probe verdict](./probe-verdict.md) | Answer a probe wake with green or red; posts it when the probe is judged |
