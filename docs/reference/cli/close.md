---
title: "ness close"
description: "Close a child you manage, as the operator a root, or your own root with the operator's words"
---

## Usage

```text
ness close ID [flags]
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--authorized WORDS` |  | close your own root: WORDS the operator typed in this conversation |
| `--discard` |  | close without an integration |
| `--keep-work REASON` |  | keep the worktree and branch, recording REASON |
| `--note TEXT` |  | close note TEXT, recorded with the close |
| `--withdraw` |  | withdraw what you sent the session that is still open, the note as reason |

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |
