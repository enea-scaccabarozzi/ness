> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# autonomy: decisions

- **A1** Core sets `ness-entry#autonomy-default` through its own profile (`set`), so a local
  profile replaces it under the existing single-slot rule (R-PROF-022); no renderer change.
- **A2** The evaluation scratch project is made by `ness init` from the binary built at the arm's
  revision (`git archive REV`), so each arm sees exactly its own skill text; a `ness` shim records
  calls instead of a real store.
- **A3** Runs cost about $0.27 each, so 6 runs per scenario per arm (30 a round) instead of 4.
- **A4** S1's before arm cannot express a project default (the slot does not exist; sync refuses
  it); the run goes ahead without it, which is the state field run 1 had.
- **A5** Spec ids R-PROF-080 (default slot) and R-PROF-081 (grant sentence) in profiles.md
  "Autonomy slots"; tests in core_text.txtar and builtin_program.txtar.
