---
name: ness-behavior-specs
description: Write declarative godog scenarios with a shared step vocabulary and freeze accepted features as contracts.
---

# ness behavior specs

## Scenarios
- Scenarios are declarative: what, not how.
- Each scenario has exactly one `When`, at most 10 steps and at most 5 example rows (the audit's FEATURE rule).

## Steps
- Steps come from one shared vocabulary file per feature directory (`features/steps.md`), never invented per scenario.
- Runners are strict: `godog` with `--strict`, so undefined or pending steps fail.

## Freezing
- Once the operator accepts a feature file, freeze it with `ness contract freeze features/<name>.feature`.
- Freezing a `.feature` locks it scenario by scenario: add scenarios freely; editing or removing a locked one needs `ness contract amend` with a reason and an approval item (see the ness-contracts skill).
