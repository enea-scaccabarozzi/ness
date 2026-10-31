---
title: "ness init"
description: "Create .ness/ness.toml and render the harness into this repository"
---

## Usage

```text
ness init [flags]
```

## Examples

```sh
# asks for what it cannot infer
ness init
# no questions
ness init --gate 'make validate' --profile go-strict --yes
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--base string` |  | base branch (default: the current branch) |
| `--client string` |  | client: claude or codex (default claude) |
| `--gate CMD` |  | gate CMD that ness integrate runs |
| `--profile NAME` |  | apply profile NAME (repeatable, applied in order) |
| `--project string` |  | project name (default: the repository directory name) |
| `--yes` |  | never prompt |

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |
