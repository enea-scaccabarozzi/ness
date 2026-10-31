---
name: ness-program
description: Run a program as a root: read back, map invariants and verticals, refine the map with the operator, run waves of children, fold results back, release.
---

# ness program

A program is too big for one session. The root owns the map; children own the verticals.

## The six steps
1. **Read back.** Survey with `ness-sweep` subagents, then restate the brief as roadmap r1
   in `docs/program/roadmap.md`: parts, dependencies, holes, questions. Minutes, not hours.
2. **Only invariants at the root**: what two or more verticals depend on, or what is
   expensive to reverse. Nothing else is decided at the root.
3. **Everything else is a vertical** with its reason, its reopen trigger and its owner.
4. **Brainstorm and refine, in rounds, with the operator.** This phase decides the program;
   it is never one pass. Each round: show the map and the program doctrine, take the
   operator's answers, challenge the map with them (Challenge), revise. Each project doctrine
   entry is kept, amended or dropped for this run. The phase ends when the operator accepts
   the map and the program doctrine and has answered the autonomy question.
5. **Waves** of the verticals whose inputs exist; each result folds back as a new map
   revision.
6. **Children integrate into the root branch; the root releases.**

## Rules
- Grounding (measurements, surveys, audits) runs in subagents; the root reads conclusions.
- A named operator preference (a tool, a vendor, a shape) becomes an explicit open decision in
  `docs/program/decisions.md`, never a thing built past.
- The root reads every child packet; the operator gets what it must decide, with options and a
  recommendation.
- Each vertical names its spec owner in the map; the root never accepts a spec the operator
  owns, it asks for it.
- Injected work becomes a map row before its child opens.
- Every fold and every new vertical is a new map revision, and the map ends with a `## Recovery`
  section holding a recovery line naming the last decision id, so a compacted root can find its place.
- A verification or cutover phase is a map section with its own probes (`ness probe start`),
  never a root that watches a screen.
- Fixes to shared tooling go to base through a quick task, not through the root branch.
- Only the root numbers decisions (`docs/program/decisions.md`). A child may keep plain dated
  notes for its own memory, with no ids, never cited upward; its handoff states its choices and
  the rejected alternatives in words.
- A brief states the question and its limits, never candidate solutions.
- A wake-up with nothing to decide gets no chat summary.

## Challenge
New information (an operator remark, a result, a failed assumption) challenges the map; the map
does not absorb it. Ask which invariant, reopen trigger or doctrine entry it weakens, say so,
then revise. Never reword the information to fit. A research program goes deeper
(`ness-research`).

At spec acceptance and at every fold:
- Read the spec or result file, not the message.
- List the alternatives the child rejected and why; none listed, send it back.
- Write what would make this naive.
- Name the doctrine entries checked, in the decision.
- Outcomes: accept, amend, refuse. Refuse a naive question or baseline even when the mechanics
  hold.
- A fold is reviewed by an independent `ness-review` subagent, on the second pass too; the root
  decides from its findings.

## Doctrine
The operator's principles in their words, each with the date first said: project doctrine in
`docs/doctrine.md` (ids P<n>), program doctrine at the top of the roadmap (G<n>). The root
proposes an entry when the operator states a principle; it binds once the operator confirms,
and only their words change it. At the end of the run, propose which G entries move up to
project level.

## Checkpoints
`ness send --to operator --intent checkpoint --body @FILE` with Done, Now, Next (and Ask). Send
one at each wave landing, map change, doctrine change, change to the success criteria (what counts as done, correct or better), or
block; it replaces heartbeat lines and "since you left". Link the review page with `--page`.

## Autonomy
Asked per program run, never fixed by the profile: one question when the operator accepts the
map, and again at any checkpoint that may change it or the success criteria.
- **gate**: stop at every checkpoint.
- **bands**: free inside the stated bands; list what stops the root (a change to the success criteria or the method,
  a naive baseline, a new, dropped or redrawn vertical, a doctrine change, a budget overrun).
- **to checkpoint X**: free until X, then wait.
- **custom**: the operator's words.

Record the mode, the operator's words and the date under `## Autonomy` in the roadmap; only
the operator changes it. Set each child's brief level from it, never above it.
<!-- ness:slot single autonomy -->

## Operator review format
<!-- ness:slot single operator-review-format -->
