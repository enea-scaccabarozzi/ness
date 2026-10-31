---
title: Sessions
description: A tree of accountable sessions, each owning a scope and a result, sized to the work.
---

A **session** is one agent conversation with a job. It owns a **scope**, the part of the product
it works on, and a **result**, what it owes when it is done. Sessions form a tree, and each one
answers to the session above it.

## The tree

| Role | Answers to | Owns |
|---|---|---|
| Operator | nobody | The product: ideas, reserved decisions, closing roots |
| Root | the operator | One line of work, end to end |
| Child | its manager, the session that opened it | One scope inside its manager's work |

A manager briefs its children, waits for their handoffs, integrates their branches and closes
them. A root does a few edits itself and gives anything larger to a subagent by default. It
opens a child only when the work needs a conversation, its own decomposition, or isolation and
duration, as the `ness-launch` skill says. Size alone is not a reason.

`ness status` draws the tree. Each row shows the session's id, scope,
[rung](./models.md#the-ladder), size, phase, context, idle time and its `waits` count.

`waits N` counts the open items addressed to the session, which it still owes an answer on. The
senders of those items follow in parentheses. In `ness status --watch`, the selected session's
detail lists each one as a `waits on SENDER ITEM` line.

```text title="ness status"
 ness  acme · base main   ● worker 0.1.0+b13684c9f547 · beat 2s

   ● s-3f9a1c0e7b21  checkout  standard full  ready  ctx 84k  idle 0m  waits 1 (s-0c9d8e7f6a5b)
├─ ● s-a1b2c3d4e5f6  payments  economy        done   ctx 41k  idle 4m  waits 0
└─ ● s-0c9d8e7f6a5b  receipts  economy        ready  ctx 22k  idle 0m  waits 0
```

## What a session gets

`ness open` gives each session four things of its own:

| Thing | Where | Why |
|---|---|---|
| Worktree | `.worktrees/<scope>-<id>/` | Sessions never edit each other's files. |
| Branch | `session/<scope>-<id>` | The manager integrates it when the work lands. |
| Terminal tab | a herdr tab | You can look at any session at any time. |
| Record directory | `<sessions>/<root>/[<child>/]` | It holds the brief, and later the plan, decisions and report. |

The session starts with a one-line prompt. The prompt points it at its brief and at the entry
skill, which tells it who it is and what it owes.

## Sizes

A **size** sets how much process a piece of work carries. The conversation that starts the work
picks it once. A size removes steps; it never adds a mode.

| Size | Use it for | What it leaves behind |
|---|---|---|
| answer | A question, a lookup, an opinion | A reply in the chat. No session record. |
| quick | One deliverable, no children, one sitting | One branch, a few commits, a short report |
| full | Anything else | Brief, specs, plan, decisions and report. Children when `ness-launch` calls for one. |

When the work proves the size wrong, the root changes it:

```sh
ness size full
```

A quick root that outgrew one sitting becomes full. A full root that shrank becomes quick, if no
child is open. Profiles can add sizes: the built-in `program` profile adds one for multi-week
work.

> [!TIP]
> Most work is an answer or a quick task. Pick full when the work needs a plan someone else can
> review.

## Promoting a conversation

You never type a ness command to start a session. When a chat with your agent turns into real
work, the agent proposes a scope and a size in one line. On your yes, it promotes the
conversation to a root.

```text title="Output"
✓ adopted s-3f9a1c0e7b21 scope checkout branch session/checkout-3f9a1c0e7b21 worktree /home/you/acme/.worktrees/checkout-3f9a1c0e7b21 record docs/sessions/checkout
work in that worktree from now on; the primary checkout stays as it is
```

If the conversation cannot be registered in place, ness refuses `--here`. The agent then runs
the same command without it, which opens the root in its own tab and worktree, and tells you
which tab.

## Closing

A quick or full session ends with a report: what was delivered, the evidence, and a
`## Lessons` heading. An answer leaves no record.
[The session record](./session-record.md#the-report-and-its-lessons) describes the report.

A manager closes its children. A root closes only on your words: the agent passes what you
typed, and ness checks that you typed it.

Next: [Messages and the worker](./messages-and-worker.md).
