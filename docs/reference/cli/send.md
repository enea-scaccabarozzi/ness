---
title: "ness send"
description: "Send a message to your manager, one of your children, another root (as a root) or the operator"
---

## Usage

```text
ness send [flags]
```

## Examples

```sh
ness send --to manager --body 'done, see report.md' --intent handoff
ness send --to operator --body @question.md --intent decision --urgent
ness send --to operator --body @progress.md --intent checkpoint --page https://example.test/p
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--body TEXT` |  | the message TEXT, or @FILE to read it from a file |
| `--intent string` | `request` | request \| fyi \| decision \| handoff \| checkpoint |
| `--page URL` |  | a page URL for a checkpoint; appended as a Page: line |
| `--to ID` |  | recipient: a session ID, manager or operator |
| `--urgent` |  | ask the worker to hint the recipient at its next pass |

## Inherited flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |
