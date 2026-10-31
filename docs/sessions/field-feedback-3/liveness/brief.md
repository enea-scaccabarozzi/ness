> [!NOTE]
> Archive: altered for the public release. Names of private projects, local paths and private links were replaced; the content is otherwise as written.

# Brief: liveness (child of field-feedback-3)

## Question
Make the worker's view of a session state-based instead of guessed, and give the operator's phone a
budget, so that a stall alert means a session is really stuck and the phone carries only what needs
the operator.

## Evidence (read first; read-only, private, never copy into the repo)
- Report: <private report link removed> (findings 1, 2, 4, 6-cap, 9)
- Root's reasoning: specs.md section "Fix round" in your manager's record (causes 1 and 2).
- Verified detail: ~/src/ness/.worktrees/field-feedback-3-404360a52d03/docs/discovery/findings/field-3/v1-liveness.md, ~/src/ness/.worktrees/field-feedback-3-404360a52d03/docs/discovery/findings/field-3/v5-operator.md, ~/src/ness/.worktrees/field-feedback-3-404360a52d03/docs/discovery/findings/field-3/v2-messaging.md (C2, C3)

## Result owed
1. One derived answer "what is this session waiting on, until when" from the store: items it sent
   that are not settled (an `accepted` request still waits), its open probe windows, its operator
   asks, a boundary note (optionally with an until-time). Used by the stall rule, alerts, status
   and the child cap.
2. Stall rule (internal/worker/stalls.go): no stall while the session waits on something; a
   boundary suppresses it; obligation stalls get the same exception. Screen signals (blocked
   prompt, error, pane gone) stay.
3. Routing: stall and idle notices go to the manager only; the phone gets them only on a second
   miss (the manager did not act). A blocked prompt is told to the manager at once and repeated to
   the operator on a backoff (about 10 min, 1 h, 4 h) until it clears.
4. "Your move" alert (internal/worker/alert.go ~166-178): an open operator ask must not silence it
   for good; key on the newest unanswered ask.
5. Phone checkpoint: the ask (or the first Next item) plus the page link, with a cap that fits a
   whole sentence (no mid-word cut at 200).
6. Child cap (internal/cli/open.go ~329-363): a child parked with an until-time does not hold a
   slot; the refusal names the children that hold the slots and how long each has been idle.
Out of scope: integrate/close/contracts (sibling child `integrate`), hook, inbox/send/pick verbs
(the root), core skill text (the root).

## Limits
- Files: internal/worker/**, internal/notify/**, internal/store/** as needed for the waits query,
  internal/cli/open.go (cap only), status display of waits. Do not touch internal/cli/integrate.go,
  close.go, internal/hook/**, inbox.go, send.go, pick.go.
- Specs are the code's rule ids (R-WRK-…): update the spec text where behaviour changes.
- Never run a test binary against a live store (the production backend, the research lab, this repo's store): a
  schema change migrates it. Scratch repos only.
- Autonomy: act within the brief. Ask me (your manager) for anything outside it.
- Lifecycle: invoke /ness-test-strategy and /ness-interface-design before design, /ness-compliance-check
  at plan, /ness-documentation before the handoff. Delegate implementation to subagents; review each.

## Report
Handoff with ## Done / ## Now / ## Next, the evidence (tests, a scratch-repo run of the worker
against the field cases: a child waiting on an accepted request, on a probe window, with a
boundary, a blocked prompt), report.md with ## Lessons, then `ness integrate --subject 'fix(worker): …'`.
