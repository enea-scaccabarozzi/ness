---
title: "ness size"
description: "Show or change the work size of a root"
---

## Usage

```text
ness size [SIZE] [flags]
```

## Examples

```sh
# the current size and the valid ones
ness size
# the work grew: a quick root becomes a full session
ness size full
# the operator resizes a root
ness size full --session s-0123456789ab
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--session ID` |  | the root ID to show or resize (the operator names it) |

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |
