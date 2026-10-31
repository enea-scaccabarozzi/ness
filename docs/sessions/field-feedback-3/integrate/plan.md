> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Plan: integrate

1. Acceptance tests (separate subagent, black-box, failing first) for specs items 1–5.
2. Implement in parallel, one subagent each, no commits by them:
   a. integrate: internal/session/integrate.go, internal/cli/integrate.go (items 1–3).
   b. contracts: internal/cli/contract.go, internal/session/contract*.go (item 4).
   c. close: internal/session/close.go, internal/cli/close.go (item 5).
3. Spec text: docs/spec/sessions.md (R-SESS-164..167, R-SESS-064 amended), docs/spec/contracts.md
   (R-CTR-055/056, R-CTR-040 changed); pre-push note.
4. Review subagent against specs.md; fix findings.
5. Scratch-repo reproduction of the moved-target race, old binary vs new.
6. ness-documentation pass, report.md with ## Lessons, handoff, ness integrate.
