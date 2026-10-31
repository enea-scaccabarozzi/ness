> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# batch-e report

## Done
All 16 tasks of docs/superpowers/plans/2026-10-15-field-batch-e.md, each test task written by a different subagent than its implementation.
- E1.0/E1.1: `internal/rule` (pointer `(skill X, section Y)`, registry, `Headings`, `Resolve`); every built-in refusal ends with its pointer; `profiles/pointers_test.go` resolves the registry, every pointer in profile-shipped files (audit.py, golangci msgs, Makefile seed, commit-msg) and every built-in `[[checks]]`. Child-cap refusal no longer suggests raising the limit; ness-entry states the background/`pgrep -f`/tool-result rules.
- E1.2: `[[checks]]` in profile.toml (four kinds plus `run`), validated at parse, pointers resolved at `ness sync` / `sync --check`; `internal/check` runs them at open (not `--resume`), integrate (before the gate lock) and close; fail closed; exit 4, nothing written. `program` ships four checks (autonomy recorded, checkpoint before the next wave, recovery line = ledger, no decision ids in a child's record); seed fixed (`D1`, own `## Recovery` heading).
- E1.3: `[[guards]]` in `.ness/ness.toml` on pre-tool, plus default gate and launch-limits guards; pointers resolved at sync.
- E1.4: sleep loops around `ness integrate|send|open` and the Explore subagent refused; handoff needs Done/Now/Next; close needs `## Lessons`; ness-entry Session record says decision ids are the root's (FF-D6). Brief's field item: the polling guard now requires the ness call inside the loop body (`for b in a b; do git log $b; done; ness status` passes), `done` counts only in command position, here-doc and quoted prose are ignored.
- E2: docs/profiles/authoring.md "Rules come with checks", linked from README.md and profiles/builtin/README.md.
- Independent ness-review: 1 high, 5 medium, 8 low; rulings in review-rulings.md; H1, M1-M4, L1, L3, L4, L5, L7 fixed with regression tests written first.

Evidence (final HEAD): `go test ./... -count=1` ok; `go test -race` on rule, check, hook, config, session, store, profile, sync, profiles, testkit, cli ok; golangci-lint both configs 0 issues; `ness sync --check` (worktree build) exit 0; `python3 .ness/audit.py` exit 0.

## Now
Integrating into the root's branch.

## Next
- E3 (the research lab checks) goes with the research lab port, as scoped.
- Existing go-strict adopters (ness, the infrastructure lab) do not get the Makefile seed's new pointer lines automatically (seed file).

## Rulings made here
- Salvaged 1654415 cherry-picked after review (sound).
- ness-entry "Lessons and briefs" split into `## Lessons` and `## Briefs` so the close pointer is exact.
- Profile check parse errors name file and `checks[N].field`, no line number (the TOML decoder gives none).
- Q4: `ness contract check` exits 4 with the feature off, so the go-strict `validate` seed was left unchanged.
- no-child-decision-ids pattern widened to `[A-Za-z0-9]+-D\d+` (the plan's missed `L0a-D3`).
- `rule` and `check` added to the R-QA-002 package layout and the design §11 tree.
- An indented (1-3 spaces) heading counts everywhere: one heading parser.

## Not built
- Chat-only ask detector (Your move alerts cover it; revisit if roots ask only in chat), Stop hook, audit plug-in rules, `ness check` command, golangci additions to go-strict, doctrine and "cites P/G" checks, root-does-too-much: out of Batch E scope.
- Q2: no source scanner for refusals that skip the registry; caught at review.
- Q5: autonomy check takes a date plus a mode word in the section; a root that writes a date without asking passes. Transcript check deferred by the root.
- Review M5: a child can delete a local check or guard in its own worktree before integrating; upgrade path is a guard over `.ness/profiles/` and `[[guards]]`.
- Review L2: default guards miss `(cd x; make validate)`, `time make validate`, `bash -c`, header-less `[worker]` key edits and `sed -i` on ness.toml. Guards nudge; they are not a sandbox.
- Review L6 (open checks run before cheaper refusals), L8 (production files over 400 lines, audit EXCEPTION lines without a pointer, immutable package-level tables).
- A command substitution inside a double-quoted string is blanked by the loop scanner, so a loop hidden there is missed (`ponytail:` comment in internal/hook/guards.go).

## Lessons
- Parallel lanes in separate git worktrees merged back cleanly; the only conflicts were appended spec rows (hooks.md, store-messages.md). Merge the shared branch into a lane before its next implementer starts, not after.
- Test writers that pin text (a heading name, an indent rule) silently extend the plan; read their result notes before launching the implementer and rule on deviations then.
- An independent review after green tests still found a high: the loop scanner's `done` matched prose. Guards over shell text need adversarial rows (quoted words, here-docs) from the start.
- The installed hook refused our own here-doc commands during the work (the L1 bug); subagents wrote such content with the Write tool.
