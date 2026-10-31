---
title: Philosophy
description: Several hand-built harnesses, one machinery that kept drifting, and why it became ness.
---

ness exists because I built the same machinery several times, and every copy drifted. This
page is that story, and the line it draws between your process and the machinery under it.

## A harness is a set of opinions

A harness is the structure around your coding agents. It decides how work is split, who
reviews it, when the human is asked, and what a release is.

Those decisions depend on the project:

- A project with a public API needs strict review and a release process.
- A personal tool needs neither, and pays for every rule it carries.
- A long program needs a map of what is in flight. A one-week fix does not.

So the starting point is per project. Decide what this project needs, and build that.

## Building them by hand

I built harnesses for a few of my own projects. Each started as a handful of skills and scripts.
Each grew as its project showed what it needed.

The processes ended up different:

- One added a strict code-quality profile.
- One added scheduled checks against a running service.
- One added a multi-week program structure.

## The drift

Under the different processes, every harness rebuilt the same four parts:

| Part | What each harness needed |
|---|---|
| Sessions | Start an agent on a scope, in its own branch and worktree, and know what it owes. |
| Messages | Ask the manager a question, hand back a result, wait without spending context. |
| Records | Keep the brief, plan, decisions and report with the code. |
| Extension points | Add a rule, a check or a file without forking everything. |

Each harness had its own copy of these parts. A fix to waking idle sessions landed in one
project and not the others. I invented a better record format twice.

## Pulling the machinery out

ness splits the two layers:

- **Machinery** is what every harness needed and what behaves the same everywhere. It lives in
  ness, with its own tests.
- **Process** is an opinion about how to work. It lives in your repository as text, files,
  config and checks. You review it as a diff.

When a piece could go either way, it ships as a default you can swap.

## What changes for you

Your attention moves to the product. You describe the work, take the decisions reserved for
you, and read the reports. The sessions coordinate, the worker wakes them, and the records keep
the reasons.

## ness builds ness

After its first steps, ness was developed with ness. The specs, session records and field rounds
from that work are public, anonymised. You can check each claim on this site against them.

Next: [Principles](./principles.md).
