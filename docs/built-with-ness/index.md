---
title: How ness was built with ness
description: The v1 plans, the field rounds that reshaped ness, the numbers from three weeks of real use, and where the records live.
---

ness was written by agent sessions, and from its first week those sessions ran on ness. The v1
work followed six plans.

Each later round was a root session with a brief, and usually decisions
and a report. The records are kept in this repository as an archive, with private project names
replaced.

## The numbers

Counted with read-only SQLite queries on the session stores of four projects, 2026-10-10 to
2026-10-31. The project names are anonymised.

| Measure | Count |
|---|---|
| Sessions | 81: 17 roots, 64 children |
| Sessions integrated | 74 |
| Store items: messages between sessions and the operator, plus 89 probe wakes | 1,722 |
| Probes | 122 |
| Probe runs | 1,484 |

The projects were ness itself, a production backend, a research lab and an infrastructure lab.

## The rounds

| Date | Round | What changed |
|---|---|---|
| 2026-10-08 to 10-09 | [v1 plans p1 to p6](../superpowers/plans/2026-10-08-p1-foundation-sync.md) | Sync and profiles, the store and hooks, sessions and launch, integrate, close and doctor, the worker, notify, contracts and probes |
| 2026-10-10 | [cli-ux](../sessions/cli-ux/report.md) | The rich renderer: tables, colour, the `--ui` modes |
| 2026-10-15 | [field-feedback](../sessions/field-feedback/report.md) | Operator alerts, the `program` profile, rules paired with checks |
| 2026-10-20 | [field-feedback-2](../sessions/field-feedback-2/brief.md) | Probe labels, profile sizes, contract scenarios, prefixes, project-wide paths and formats |
| 2026-10-21 | [model-select](../sessions/model-select/report.md) | The rung ladder, selectors, hook enforcement |
| 2026-10-29 | [field-feedback-3](../sessions/field-feedback-3/report.md) | State-based liveness, the integrate queue, the base-tip rule for contracts, phone alerts that point at a session |
| 2026-10-29 | [operator-channel](../sessions/operator-channel/report.md) | The operator works only in session chat. The phone only notifies. |
| 2026-10-31 | [public-release](../sessions/public-release/brief.md) | These docs, checked by running every command shown |

## What the field taught

Each field round started from real use. Three findings changed the shape of ness more than any
plan:

- Rules in prose were skipped. Each rule now names a sensor, and each refusal names its rule.
  See [Give a rule a check](../guides/checks.md).
- Sessions picked the most expensive model by habit. After the ladder and the hook, the
  research lab's share of spend on the `frontier` rung fell from at most 70.8% to 21.7%. More in
  the
  [field-feedback-3 report](../sessions/field-feedback-3/report.md) and
  [Pick the model for each job](../guides/models.md).
- The operator layer was noisy: 58 of 59 stall alerts in round 3 were false. Liveness now comes
  from session state, and the phone only points you at the session that needs you.

## Read the records

| Folder | Holds |
|---|---|
| [design/](../design/ness-design.md) | The design and its decision log |
| [spec/](../spec/README.md) | The requirements, each with its verification |
| [sessions/](../sessions/) | One folder per round: a brief, and usually decisions, a plan and a report |
| [superpowers/plans/](../superpowers/plans/) | The implementation plans, v1 and later batches |

Next: [Roadmap](./roadmap.md).
