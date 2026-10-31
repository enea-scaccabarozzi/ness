---
title: "ness post"
description: "Post one Markdown note to a configured channel, rendered for its platform"
---

## Usage

```text
ness post [flags]
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--channel NAME` |  | channel NAME from [notify.channels] |
| `--dry-run` |  | print the exact payload of each part and send nothing |
| `--file FILE` |  | the Markdown note FILE |
| `--thread KEY` |  | thread KEY: the first post starts the thread, later ones reply in it |

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |
