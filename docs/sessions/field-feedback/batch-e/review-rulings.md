> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Batch E review rulings (2026-10-15)

Review: independent ness-review of 56f11b3..563f85f. Findings and rulings:

- H1 loop scanner ends a loop at any word `done` (`echo done`): fix. Count `done` only in command position, ignore quoted text.
- L1 guards fire on here-doc/quoted prose: fix together with H1 (strip here-doc bodies and quoted strings before scanning; reuse any existing helper from 17cd1a0).
- M1 branch-forbids reads a diff shaped by user git config (color, external diff): fix with `--no-color --no-ext-diff --no-textconv --src-prefix=a/ --dst-prefix=b/` and `-c core.quotePath=false`.
- M2 autonomy-recorded passes once the recovery line carries a date, because the seed's Autonomy section runs to the end of roadmap.md: fix by giving the recovery line its own heading in the seed (`## Recovery`), so the section is bounded. The Q5 ceiling (instruction line contains the mode words; a date is the discriminator) stays as accepted.
- M3 close checks in a gone worktree always fail: fix. With the worktree gone, close checks are skipped and the close output says so. (Fail-open on that path is accepted: nothing exists to examine.)
- M4 checkpoint-since-landing applies to a child opening grandchildren, which can never send a checkpoint: fix. The kind passes when the acting session is not a root.
- L5 operator acting gives root "" and LastLanding("") counts all roots: fix. checkpoint-since-landing passes when root is empty.
- L3 the check test's fake runner returns 0 for unknown commands: fix (unknown command fails the test).
- L4 authoring.md omits that a local profile must be listed in `profiles`, and that guards only match Bash commands or edited paths: fix the doc.
- L7 three heading parsers disagree: fix by making check kinds and the Lessons check use rule.Headings' rules (one parser), if it stays small.
- M5 a child can delete a local check or guard from its own worktree and integrate: not built. Known ceiling; review of `.ness/` diffs at integrate catches it; a guard over `.ness/profiles/` and `[[guards]]` is the upgrade path.
- L2 default guards miss subshell/`time`/`bash -c` gate forms, `[worker]` key edits without the header, `sed -i` on ness.toml: not built. Guards are a nudge, not a sandbox.
- L6 open checks run before cheaper refusals: not built, cosmetic.
- L8 file sizes (production files over 400 lines; the ceiling is for test files), audit EXCEPTION pointer, immutable package-level tables: not built.
