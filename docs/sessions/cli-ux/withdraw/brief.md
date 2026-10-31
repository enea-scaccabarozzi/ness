> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# withdraw

## The operator's words (verbatim)
"Ok perfect regarding A manager can't withdraw a request it sent, go with D as raccomended."

## Context
Option D, from the root's analysis: today only the recipient can settle an item (`ness reply`), and an open request/decision/handoff blocks the recipient's close (R-MSG-040). A manager whose picture changed (it did the work itself, the operator answered elsewhere, the child is deaf) had to send a second request asking the child to decline the first: a round trip, a new obligation, and a deadlock when the child can't respond (it happened: m-64kuxw2f / m-nxvl2mqk in field run 2). Decision CX-21 in docs/sessions/cli-ux/decisions.md.

## Result owed
1. The sender of an open item can settle it as `withdrawn` with a reason (`ness reply ITEM --as withdrawn --body REASON` or a dedicated verb, your call; keep the CLI's existing shape). Only the sender may withdraw; the recipient sees the reason (its inbox lists a withdrawn notice, or the item's answer, your call). New disposition in the store (schema migration forward), `Item.Open` false for it.
2. `ness close ID --withdraw` (manager closing a child): settles every open item the caller sent that child as withdrawn (reason = the close note) and closes in one step. Without the flag an open request still blocks the close, unchanged.
3. Also evaluate and, if cheap and clean, add a `returned` disposition for a child handing work back (today it answers `deferred`, which keeps the item open forever); if you add it, it settles the item. Record the ruling.
4. Spec ids (store-messages.md R-MSG-* from R-MSG-060 up; sessions.md R-SESS-* from R-SESS-130 up), testscript coverage, skill text (ness-entry / ness-launch) updated in one or two lines, `ness sync` re-rendered.

## Limits and autonomy
Act within the brief. Tier standard.

## Rules for this session
- Never read or grep anything under `docs/discovery` (exclude it from every search).
- Gate: `mise exec -- make validate`; check its exit status directly, never pipe it into tail before a commit.
- No `git stash`, no pkill, no `--no-verify`. Do not push. Integrate into the manager branch with `ness integrate` when done, then hand off.
- Two sibling sessions run in parallel (withdraw, integrate-fixes, seams, secrets). Before picking a new requirement id, grep the spec and pick from your own reserved range below, to avoid duplicate ids.
- Record decisions in your session decisions.md, and a report.md with evidence and lessons at handoff.
