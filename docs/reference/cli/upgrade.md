---
title: "ness upgrade"
description: "Move this project to the running binary: migrate, sync, bump the pin and commit"
---

## Usage

```text
ness upgrade
```

Run in the base checkout, clean and on the base branch. It migrates the store, sets the pin
to this binary's major.minor, syncs, and commits as chore(ness): upgrade to X.Y.Z. It never pushes.

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |
