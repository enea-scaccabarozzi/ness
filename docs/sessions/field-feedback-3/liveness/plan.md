> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Plan: liveness

Specs: specs.md (S1-S8). Judge and implementer are separate subagents (ness-test-strategy).

## T1 Judge (subagent, kind implement): acceptance tests, failing
Black-box tests in the existing files' style (`store_test`, `worker_test` with fakes_test.go,
testscripts in internal/cli/testdata/script), written from specs.md only:
- store: Waits per kind (accepted request still waits, resolved does not, retired does not, ask to
  operator, active probe, paused/stopped probe not, parked with until before/after, parked without
  until cleared by an arrival); parked record survives ClearBoundary; migration 12 on an old store.
- worker (stepped clock): child with an accepted request idle 2 h → no stall item, no alert; with
  an active probe → none; parked → none, then a stall after the until passes; obligation held while
  waiting → none; a plain stall → one fyi to manager, no Trouble alert, Trouble alert at 2× stall
  with nothing arrived, none if the manager's item arrived; blocked → fyi to manager at 2 min,
  Trouble at 10 min, 1 h, 4 h, 8 h, cleared episode → reset; Your move fires after the root worked
  past an old open ask, not while the ask is from its latest turn; checkpoint text: multi-line Ask
  whole, first Next item with continuation, 400-rune sentence cut, no mid-word cut.
- cli: open refusal lists the slot holders; a child parked until tomorrow frees a slot; status
  shows `on ...` and JSON `waits`.
Output: the test files, all failing for the right reason (compile errors on the new API allowed).

## T2 Implement (subagent, kind implement), against T1's tests
Store (migration 12, Wait, Waits, SetBoundary with until, parked fields on Session), worker
(stalls.go, alert.go, operator.go), notify only if needed, cli/open.go cap, cli/status.go waits,
inbox.go `--until` only after the manager agrees. Spec text edits: R-WRK-050, -080, -081, -082,
-093, -094, new R-WRK-099, R-MSG-034, R-SESS-152, R-SESS-080. godoc on every export.
Package tests of the touched packages pass; `go vet` clean. Never against a live store.

## T3 Review (ness-review, kind review) of T1+T2 against specs.md; fix what it finds.

## T4 Evidence (me): build the binary, scratch repo under the scratchpad, worker run against the
field cases (accepted request, probe window, boundary with until, blocked prompt via the fake
runtime tests if herdr cannot be driven headless). Record in report.md.

## T5 /ness-documentation, report.md with ## Lessons, handoff, `ness integrate`.
