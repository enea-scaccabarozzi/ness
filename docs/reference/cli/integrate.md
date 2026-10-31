---
title: "ness integrate"
description: "Fold a branch into its target through the gate, push if configured, record it"
---

## Usage

```text
ness integrate [flags]
```

## Examples

```sh
# this session's branch into its base as one commit, through the gate
ness integrate --subject 'feat(cli): add --subject'
# any pair, as the operator
ness integrate --from topic --into main --subject 'fix: x'
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--from BRANCH` |  | source BRANCH (default: this session's branch) |
| `--into BRANCH` |  | target BRANCH (default: this session's base, its manager's branch) |
| `--subject SUBJECT` |  | the squash commit's Conventional Commit SUBJECT, type(scope): summary (other modes warn and ignore it) |

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |
