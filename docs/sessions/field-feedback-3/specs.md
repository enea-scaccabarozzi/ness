> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# Specs — field feedback 3

Agreed with the operator on 2026-10-29 ("Perfect go on"):
- Window 2026-10-21 → 2026-10-29; the production backend (2 roots, 16 children) and the research lab (1 root, 8 children). The infrastructure lab excluded (1 transcript).
- Axes: A3 operator loop/autonomy, A2 messaging/liveness, A4 model ladder/cost, A5 probes/contracts, A1 lifecycle, A6 sizes/ceremony, A7 context hygiene, A8 integration robustness, A9 release readiness.
- Deliverable: a data-grounded report (every weak point verified against raw data), plus next steps: fix cycle, next field run, distance to public release.
- Working files: docs/discovery/findings/field-3/ (private, like earlier rounds).

# Fix round (second ask)

## Big picture: five causes behind the ten findings
Questioning each finding ("what is this feature for? did it ever pay? is the fix a new verb or a
removal?") reduces the ten findings to five causes. The fix targets the cause, not each symptom.

1. **ness guesses state instead of knowing it.** The stall rule guesses from idle time plus
   obligations, "Your move" guesses from any open ask, the phone guesses which line matters. The
   store already knows most of what a session waits on: items it sent and nobody has settled
   (an `accepted` request still pending), its open probe windows, its operator asks, a boundary.
   One derived answer, "what is this session waiting on, until when", feeds the stall rule,
   alerts, status and the child cap. Findings 1, 2 (part), 4, 6 (cap), 9 (part).
   - Asked: does stall detection need to exist? This round it had 1 true positive against 58
     false; the one costly real stall (17 h prompt) came from screen reading, not from the rule.
     Keep the screen signals (prompt, error, pane gone) and keep the rule only for "idle,
     nothing waited on, nothing handed off".
   - Asked: is a new `ness wait` verb needed? Mostly not: the store has the waits. The one case it
     lacks is a date ("integrate after 10-27"), which a boundary note with an until-time covers.
2. **The operator channel has no budget.** Anything the worker sees can reach the phone, so the
   phone stopped being read (60% worker notices; 6 returns with "read nothing"). A routing rule,
   not a feature: the phone gets operator asks, checkpoints, and escalations a manager failed to
   act on (second miss, a prompt still blocked). Everything else goes to the manager.
   - Asked: is a "since you left" digest needed? Not yet. Fix noise and the checkpoint line on the
     phone (the ask plus the page link) first; the field run measures whether returns drop.
3. **Peers share one branch and one cap, but the tree only models a hierarchy.** Two roots in one
   project, a project-wide child cap, a manager committing records to the branch its children
   integrate into: all are peers writing to shared state. Serialise writes per target (queue,
   not refuse) and let a target that moved only by session-record commits keep the gate result.
   - Asked: should the cap become per-root? No: it is half the cores because the box overloaded on
     10-21. A child that is parked (waiting with an until-time) should not hold a slot, and
     status should say who holds the slots.
4. **Guards are regexes over shell text, so they both over-refuse and leak.** Instead of refining
   regexes, give agents a verb for each legitimate need (`ness inbox --show A B C`, `--unread`) and
   keep the guard for the one real pattern: an unbounded loop. Same family: `ness pick` reading
   stdin, effort required when the kind already implies it, `sweep` vs `search`.
5. **Intent is never checked.** Reviews judge code; nothing judges "does this answer the brief in
   its own terms". Part of this is normal product iteration that no harness removes. Changing the
   reserved decisions is a policy change, so it is the operator's call (asked, FF3-D4).

## Dropped or deferred, with the reason
- `ness promote`: release flows are the project's; a frozen-file amend path with the operator's
  words covers what ness owns.
- Refusing `git push --no-verify`: prod pushes are already an irreversible reserved decision.
- Guarding shell writes to `.ness/ness.toml`: whack-a-mole; the one case had the operator's go.
- Monitor inside subagents: the empty frontier wakes cost 3–6% of the root's spend; revisit with data.
- Digest command: wait for the field run.

## Scope of the fix round
- Child `liveness` (worker, alerts, notify, cap): causes 1 and 2.
- Child `integrate` (integrate, close, contracts): cause 3 on the branch side, finding 7.
- Root, through subagents: cause 4 and small defects (inbox verbs, loop guard, pick stdin, effort
  default, sweep alias, reply disposal, transcript path recorded at adoption, version at build),
  root-to-root sends once the operator rules (FF3-D3).
- Root, after the operator rules: core text for cause 5.
- Then one batched rollout: install, sync in the production backend and the research lab, worker restarts, a
  "what changed" note to each live root (operator's go given in the second ask).
