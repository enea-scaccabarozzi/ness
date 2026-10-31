---
title: Principles
description: The seven rules ness is built by, and what ness is not.
---

These seven rules constrain ness itself. They are not a process for your project: that part is
yours. Each rule is enforced by a test, a hook or a review rule.

| # | Principle | In practice |
|---|---|---|
| 1 | No universal harness | ness ships primitives and swappable defaults, never one right process. |
| 2 | You steer the product, not the code | Your attention goes to ideas and decisions. Sessions carry the rest. |
| 3 | Explicit and per project | No global config. The process lives in the repository and is reviewed as a diff. |
| 4 | Ceremony must earn its place | Every rule needs evidence. A rule that does not pay is removed. |
| 5 | Context is the scarce resource | Sessions start fresh, compact at boundaries and print short output. |
| 6 | Supported or refused | ness fails loudly where it was not tested. It never degrades silently. |
| 7 | Tested, not claimed | Every promised behaviour has a test that fails without it. |

## 1. No universal harness

Sizes, profiles, atoms and the model ladder are all replaceable. When a default stops fitting,
you change it in your repository. You do not work around it.

## 2. You steer the product, not the code

Each session owns its scope and its result. The decisions you keep are listed as reserved.
Everything else proceeds without you.

## 3. Explicit and per project

ness reads one config file in the repository, `.ness/ness.toml`. `ness sync` renders the files
it needs into the repository and writes a lock with their hashes, `.ness/lock.json`, which you
commit. An upgrade is a diff you read before you accept it.

## 4. Ceremony must earn its place

A new rule needs a test, a replay or a measurement behind it. Work comes in sizes so that a
small job skips the steps a large one needs.

## 5. Context is the scarce resource

An agent's context window is the budget for everything else. ness starts sessions fresh,
compacts them at boundaries instead of mid-task, and keeps command output short.

## 6. Supported or refused

ness runs only where it was built and tested. Anything else stops with what is missing and how
to fix it.

> [!IMPORTANT]
> A refusal protects the work. A missing tool, a version outside the project's pin, or a global
> skill that shadows a project skill stops the command before it produces work you cannot trust.

## 7. Tested, not claimed

A behaviour without a test is not promised. Codex, the one experimental part, is marked so on
this site, and the config refuses it until you opt in. [Limits](./limits.md) lists what ness was
not tested on.

## What ness is not

- It is not a process. It ships defaults for planning, review and release, and you can swap
  each one.
- It is not an agent. It runs the agent you already use and gives its sessions structure.
- It is not a hosted service. It is one binary and the files it writes into your repository.
- It is not global. Two kinds of file live outside the repository: the optional user service
  that runs the worker, and the secrets in `~/.config/ness/secrets/<project>.toml`.

Next: [Opinions in the defaults](./defaults.md).
