---
name: ness-test-strategy
description: Plan the tests before the code: the judge is not written by the implementer, black-box, hand-written fakes.
---

# ness test strategy

## Who judges
- The implementer does not write the judge: a reviewer or a separate task writes the acceptance test.
- The contract comes first, in the consumer's words.

## Shape
- Tests are black-box (`package x_test`).
- Fakes are hand-written, never generated mocks.

## Keeping a test
- Before keeping a test, answer: which wrong implementation would this reject? If none, delete it.
