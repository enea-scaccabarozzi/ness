---
name: ness-interface-design
description: Design Go interfaces and wiring so the code stays testable: consumer-declared ports, injected collaborators, no test hooks.
---

# ness interface design

## Interfaces
- Interfaces are declared by the consumer, in the package that fakes them, with at most three methods.
- Outcomes travel through ports (return values, channels, callbacks), never through shared state.

## Wiring
- Injected collaborators are built only in the composition roots (`cmd/`); everything else receives them.
- No package-level mutable state.

## Time and test hooks
- No `ForTest` identifiers or test hooks in production code.
- `now` is a value passed in. Timers are tested with `testing/synctest`. There is no `Clock` interface.

## Packages
- Every package has a one-sentence role in its package comment.
