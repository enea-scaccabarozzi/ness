---
title: Limits
description: What ness has not been tested on, and so does not claim.
---

ness promises only what it was tested on. This page lists what lies outside that, and what
happens when you try it.

| Area | Tested | Not claimed |
|---|---|---|
| Scale | One operator, or a small team | Larger teams |
| Operating system | Linux | macOS, Windows, BSD |
| Service manager | systemd user units | Any other init system |
| Terminal multiplexer | herdr | tmux, zellij, screen, plain terminals |
| Client | Claude Code | Codex beyond experimental, any other agent |

## Scale

ness was tested solo and in small teams. It has not been run with a larger team, so nothing on
this site says how it behaves there.

What is known: every project has one worker and one store, and the store is a local file. The
child cap defaults to half the machine's cores. Sessions coordinate on one machine.

## Platforms

ness runs on Linux only. On any other OS, every command stops with exit 4 and `ness v1 runs on
Linux only; this is <os>`. Only the `--version` and `--help` flags still answer.

The worker needs a systemd user session, which `ness doctor` checks. `ness worker install`
writes a user unit. `ness worker run` runs the same worker in the foreground of a terminal,
without systemd, and nothing restarts it when it stops.

Sessions need herdr. `ness open` creates each session's tab through herdr, and `ness doctor`
fails `herdr-version` when herdr is missing or on another major version.

## Codex

Codex support is experimental, and you opt in to it. Without the opt-in, ness refuses a Codex
client with exit 4:

```text title="Output"
ness: .ness/ness.toml: client = "codex" needs experimental = ["codex"]
```

With `experimental = ["codex"]`, `ness sync` renders the Codex config. Two gaps remain:

- The worker cannot wake a Codex session. `ness doctor` warns `codex has no worker support`, and
  a Codex session reads its mail only when it runs `ness inbox`.
- The default Codex model ids may not match your account. `ness doctor` checks each rung against
  `codex debug models` and names the rungs that do not resolve. Set `codex` in each
  `[tiers.<rung>]` to a model your account lists.

Codex cannot promote a running conversation to a root. `ness open --root` opens it in its own
tab instead.

Next: [Install](../start/install.md).
