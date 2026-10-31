> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# Child: delivery fixes from field run 1

Context (read first): docs/sessions/cli-ux/infra-lab-feedback.md (findings F1-F6) and
docs/sessions/cli-ux/decisions.md CX-14..CX-19. The operator approved fixes F1-F6 ("yes").

Operator's words on delivery (verbatim): "I may be scrolling chat page of an idle session and the
session needs to recieve a notification, i do not want to stall its delivery. The only thing to
control are input box and its changes, not screen itslef i think"

Result owed, each with its spec requirement amended or added and a test that fails without it:
1. F1: a child's tab opens in its parent's herdr workspace (the workspace prefix of the parent's
   tab id); a root opened outside a herdr pane falls back to HERDR_WORKSPACE_ID, then to herdr's
   default. Same for `ness open --resume`. (internal/atom/herdr OpenTab takes a workspace.)
2. F2 / CX-15: the worker holds a delivery only while the input box holds a draft or the draft
   changed within the quiet window. Focus and scrolling never hold. Amend R-WRK-017 and every
   test that asserted the focus hold; keep the urgent rule (R-MSG-042) coherent.
3. F4: `ness open` starts the agent without the first prompt in argv, waits for readiness, then
   sends the first prompt with `herdr agent prompt` (no 120 s timeout per launch). Check with a
   real herdr pane once (herdr skill), then close what you opened.
4. F5: the session-start hook prints the cheat sheet at every start, not only after compaction.
5. F6: closing a child disposes the items that child sent to its manager and that are still open
   (accepted, answer "closed with the child"), so the manager owes no acknowledgement replies;
   ness-launch says a handoff needs no reply.

Limits:
- A sibling session (operator-loop) works in parallel on notifications, `--to operator`, the root
  self-close and the skill texts for escalation and autonomy. Stay out of those; if you must touch
  internal/worker/worker.go or internal/cli/close.go, keep your change small and expect to rebase.
- Autonomy: act within this brief; the operator approved it in advance. Ask me only for something
  outside it.
- Lifecycle skills as ness-entry says; gate is `make validate`; never --no-verify.

Report: report.md in your record, then `ness integrate` into my branch and a handoff via ness send.
