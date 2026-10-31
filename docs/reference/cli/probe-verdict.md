---
title: "ness probe verdict"
description: "Answer a probe wake with green or red; posts it when the probe is judged"
---

## Usage

```text
ness probe verdict ITEM [flags]
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--note string` |  | your finding, posted when the probe is judged |
| `--probe-bug` |  | the probe itself is wrong: record a defect, post nothing, flag needs-edit |
| `--state string` |  | green \| red |

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |
