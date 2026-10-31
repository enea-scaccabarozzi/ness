---
title: Session record
description: Every session leaves committed files that say what it was asked, what it agreed, how it decided and what it delivered. Why something was built stays readable after the agents are gone.
---

The **session record** is a directory of Markdown files a session writes as it works. It lands
on the base branch with the session's code, so the reasons behind a change are in the same
history as the change.

```text
docs/sessions/
  checkout/                 a root
    brief.md
    specs.md
    plan.md
    decisions.md
    report.md
    payments/               one of its children
      brief.md
      report.md
      ...
```

The directory is `<sessions>/<root>/[<child>/]`. `sessions` is a key in `.ness/ness.toml`,
`docs/sessions` by default.

## The files

| File | What it holds | Written |
|---|---|---|
| `brief.md` | What the session was asked, verbatim | By `ness open`, from the brief file it was given |
| `specs.md` | What the session agreed to build | At specify, before any design |
| `plan.md` | The steps | Before the first commit |
| `decisions.md` | One entry per choice, append-only, each with a stable id | As choices are made |
| `report.md` | What was delivered, with the evidence, and a `## Lessons` heading | At handoff, before close |

The size sets which files a session writes:

| Size | Record |
|---|---|
| answer | None. The reply stays in the chat. |
| quick | `brief.md` and a `report.md` of 10 to 30 lines |
| full | All five files |

## The brief

`ness open --brief FILE` copies the brief into the new session's record directory. The
session's first prompt is one line that points at it. A root promoted from a chat writes your
request to the brief word for word.

When the work is resumed with a new brief, the new one
replaces `brief.md`.

A brief binds only what it states as a requirement. A side remark in it does not become a rule.

## Decisions

`decisions.md` is append-only. Each entry has a stable id, a title and a date, and says what was
chosen and why. A real entry from ness's own record:

```markdown title="docs/sessions/model-select/decisions.md"
## MS-D3: one ladder of (model, effort) rungs, one offset (2026-10-21)
The chat proposal spoke of separate model and effort offsets. Research shows tiers are points on one cost/quality
...
```

Only the root mints decision ids. A child writes plain dated notes without ids, and puts a choice
that matters in its handoff to its manager. One tree keeps one list of ids, so a citation never
points at two decisions.

Other parts of ness write to the same file. `ness contract amend` appends one line per frozen
file it opens for change:

| Lock | Line |
|---|---|
| The session's own `contract.lock` | `Amend PATH: REASON [item ID] (DATE)` |
| `[contracts] frozen`, project-wide | `Amend PATH (project-wide): REASON [operator: "WORDS"] (DATE)` |

The item is the approval from the manager or the operator, and WORDS are what the operator typed.
[Features](./features.md#contracts) explains who may amend.

## The report and its lessons

A quick or full session ends with `report.md`: what was delivered, the evidence, and a
`## Lessons` heading. Write `None.` under it when there are none. `ness close` refuses a report
without the heading.

A child appends its lessons at handoff. Its manager reads them, and a new session reads its
manager's lessons at entry. A lesson travels up the tree instead of dying with the conversation.

## Why it matters

An agent's conversation ends when its session closes, and its context is gone after the next
compaction. The record is what survives.

- A reviewer reads `specs.md` and `plan.md` before the code, and checks the code against them.
- A compacted session re-reads its brief and finds its place again.
- Months later, `git log` shows the change and `decisions.md` shows why it was made that way.

Records are files in your repository. They are reviewed in the same diff as the code, and a
gate that checks links can check them too. ness's own records are published in its repository,
as evidence of how it was built.

## Links

- [Sessions](./sessions.md): sizes and closing.
- [Messages and the worker](./messages-and-worker.md): the handoff and checkpoint headings.
- [Contracts](../guides/contracts.md): frozen files and the `Amend` lines.
- [Program](../guides/program.md): the `program` profile keeps a project-wide decisions ledger.
- [Files](../reference/files.md): what ness writes where.

Next: [Profiles and sync](./profiles-and-sync.md).
