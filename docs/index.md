---
title: Introduction
description: There is no universal harness. ness is the shared machinery under the one you build for yourself.
---

ness is the machinery under an AI coding-agent harness. You build the harness: the process,
rules and checks your project needs. ness carries the parts every harness needs and nobody
wants to rebuild.

It is one CLI. It works with Claude Code today, and with Codex as experimental. The name comes
from har**ness**.

## What ness ships

| Part | What it does |
|---|---|
| Sessions | A tree of sessions. Each one owns a scope and a result, in its own worktree, branch and tab. |
| Messages | Sessions send, reply and read an inbox. A worker wakes the session a message is for. |
| Records | Each session commits its brief and report next to the code. A full session adds a plan and decisions. |
| Extension surface | Profiles, slots and atoms let you change the process without forking ness. |
| Defaults | Features, profiles and atoms that work out of the box. You can swap each one. |
| Guards | Hooks and a gate that refuse what is not supported, and say why. |
| Guides and sensors | Skills and briefs that guide the agent before it acts, and checks that refuse drift. Each refusal names its guide. |

## The argument

1. **No harness fits every project.** A setup tuned for a public API is dead weight on a weekend
   tool. You decide, per project, what to keep and what to drop.
2. **I built several harnesses by hand.** Each one ran a different process. That part was right.
3. **The machinery under them was the same, and it drifted.** Each harness rebuilt sessions,
   messages, records and extension points. A fix in one never reached the others.
4. **So I pulled the machinery out.** ness is that machinery, extracted once and tested on its
   own. Your harness sits on top of it, in your repository.
5. **You steer the product.** You bring the ideas and take the reserved decisions. The sessions
   carry the plumbing.
6. **ness builds ness.** After its first steps, ness was developed with ness. The specs, session
   records and field rounds are public, anonymised.

> [!NOTE]
> ness is tested solo and in small teams. Larger teams are untested. See
> [Limits](./why/limits.md) for every limit.

Next: [Philosophy](./why/philosophy.md), how ness came out of building harnesses by hand.
