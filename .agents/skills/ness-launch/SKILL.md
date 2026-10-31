---
name: ness-launch
description: How a manager opens, briefs, waits for and closes child sessions, and which tier each job gets.
---

# ness launch

## Open a child
`ness open --scope NAME --tier TIER --brief FILE [--title T] [--effort E] [--client claude|codex]`
opens a worktree, a branch and a tab for the child and hands it your brief. Only the operator
opens a root (`--root`). Write the brief first: the question, the result owed, the limits, the
autonomy level (plan first; report, then act; act within the brief: never lower than the
operator's words set), the
tier, and how to report. The child's record lives under yours.

## Child or subagent
Measured over 2,388 delegations: a child costs about $10 fixed plus $1.7 per 100 changed lines
and an hour of wall time; a subagent about $2 fixed plus $0.7 per 100 lines, in minutes.
A child never picks a size: it is the one child shape, sized by its brief. Quick work under a
parent is a subagent, never a quick child; a quick root opens no child at all (`ness open`
refuses) and resizes to full if the work grew.
- **Inline**: a few edits you can see whole.
- **Subagent**, the default: any task one agent can finish in one run from a written task,
  without asking anyone along the way. Size alone (up to about 1,500 lines) is no reason for a
  child. Run several at once only on disjoint files, or each in its own worktree; review each
  result, and run the gate once at the end.
- **Child**, only when the work needs what a subagent cannot have:
  - a conversation: decisions to agree with the operator or with you along the way, or
    steering over days;
  - its own decomposition: a vertical it will plan and split into subagent tasks itself;
  - isolation or duration: a live environment (tabs, services, long evaluations), or a run of
    more than about an hour that must survive your compaction.
- A design question you can settle with the operator yourself first: settle it, then dispatch a
  subagent. Never open a child just to ask.
- At the child cap (`launch.max_children`), wait for a child to land and never raise the cap:
  work that needs a child never moves into a subagent to get past the cap.

## Tiers
A child runs at the project's base rung unless its job says otherwise: open it with
`--kind <name>` and the selector picks the rung, or name one with `--tier` (a rung above the
recommendation needs `--reason`). Choose by who judges the output: work a check or a reviewer
will judge runs low; work whose output is itself the judgment (design between alternatives,
conclusion audits) runs high. Escalate a stuck child with `ness open --resume ID --tier up`.

## Wait, then fold
Pull `ness inbox` once and end your turn; the worker wakes you when the child reports. When
the child's branch is in, `ness close ID --note TEXT` closes it. A handoff needs no reply:
closing the child settles everything it sent you. A request of yours that no longer holds:
`ness reply ITEM --as withdrawn --body REASON`, or close with `--withdraw`.
`ness integrate --subject 'type(scope): summary'` folds your own branch through the gate into your
manager's branch as one commit with that Conventional Commit subject. Always pass `--subject`: a
project not on `integrate.mode = "squash"` ignores it with a warning.
