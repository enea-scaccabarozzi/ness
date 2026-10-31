> [!NOTE]
> Archive: altered for the public release. Local paths and private links were replaced; the content is otherwise as written.

# Brief: integrate (child of field-feedback-3)

## Question
Stop `ness integrate` from throwing away finished gate runs when the target branch moves, make
concurrent integrates queue instead of refuse, make the push survive a long pre-push hook, and
close the contract and close gaps the field round found.

## Evidence (read first; read-only, private, never copy into the repo)
- Report: <private report link removed> (findings 3 and 7)
- Root's reasoning: specs.md section "Fix round" in your manager's record (cause 3).
- Verified detail: ~/src/ness/.worktrees/field-feedback-3-404360a52d03/docs/discovery/findings/field-3/v3-integrate.md

## Result owed
1. Per-target lock queues (with a visible "waiting for X's integrate") instead of refusing.
2. After the gate, if the target moved: when every new target commit touches only the session
   record directory (`sessions` in ness.toml), rebase and land without re-running the gate; else
   rebase and re-run the gate once under the lock; a conflict returns a plain-language error
   (no raw git output). Question this rule yourself before building it; propose a better one if
   the code shows one.
3. Push: SSH keepalive for the push and one retry on a dropped connection; say in the docs that
   a project's pre-push hook repeating the gate is the project's to skip.
4. Contracts: `ness contract check` counts files added under a frozen glob (operator request
   m-tw2j5gkv; it changes the current spec on purpose). A project-wide frozen file gets an amend
   path that takes the operator's typed words, like close --authorized.
5. Close: a child whose tree is already contained in the manager's branch closes without
   `--discard` ("nothing to integrate" and "no integration recorded" must not block each other).
Out of scope: worker/alerts (sibling child `liveness`), hook, inbox/send/pick (the root).

## Limits
- Files: internal/cli/integrate.go, close.go, contract code, internal/session/**, gate/lock code.
  Do not touch internal/worker/**, internal/hook/**, open.go, inbox.go, send.go, pick.go.
- Never run a test binary against a live store; build the race and the push drop in scratch repos
  (a bare remote with a slow pre-push hook reproduces the drop's shape).
- Autonomy: act within the brief. Ask me (your manager) for anything outside it.
- Lifecycle: invoke /ness-test-strategy and /ness-interface-design before design, /ness-compliance-check
  at plan, /ness-documentation before the handoff. Delegate implementation to subagents; review each.

## Report
Handoff with ## Done / ## Now / ## Next, the evidence (tests plus a scratch-repo reproduction of
the moved-target race before and after), report.md with ## Lessons, then
`ness integrate --subject 'fix(integrate): …'`.
