> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# field-feedback: plan

The direction was settled when the operator approved the report and the clarifications
(`docs/discovery/findings/field-research/report.md`, `clarifications.md`, census in
`liveness-census.md`). This plan executes them; it does not re-open them (FF-D3). Section 1 of
`specs.md` (operator alerts) is the agreed design for B5.

Execution: the root keeps the map; each batch is one superpowers implementation plan under
`docs/superpowers/plans/`, built task by task by subagents with TDD, acceptance tests written by a
separate task, one fresh reviewer per batch. Child sessions only if a batch outgrows that.

## Batch A: the research lab setup (its repository, no ness release)
- A1 Drop pymarkdown from pre-commit and the gate; drop the 700-char MD013 cap; exclude `docs/data-sources.md` from Markdown tooling.
- A2 mdformat on staged files only, re-staged by the hook (no leftover commits), or gate-only.
- A3 Pre-commit: ruff on staged Python only; typecheck, tests, Markdown check once in the gate.
- A4 Commit-msg: accept ness's integrate subject and the `research` type.
- A5 One workflow rulebook: remove the pre-ness line loop (round card, board, digest) from `AGENTS.md:128-139` and `docs/lines/README.md`; cut `research-communication` back to the writing standard.
- A6 One heavy simulation at a time on the 15 GB box (local rule).

Running as the research lab root s-176c2b204b08 (scope setup-cleanup, own tab). At its end it rebriefs the study-protocol root by an fyi the worker delivers when that root is idle, then asks the operator for its close (operator, 2026-10-15).

## Batch B: ness fixes for every size of work
- B1 Pre-tool hook refuses shell backgrounding `( … ) &` / trailing `&` and `until … pgrep -f` waits, pointing to `run_in_background` (census: 13.6 h dead).
- B2 `ness status` idle from the last real work entry, not the transcript mtime (`status.go:219`, `claude.go:148`).
- B3 Stall backstop: a child that has not handed off owes its brief; the stall report goes to its manager without waking the child.
- B4 `ness-launch`: `--subject` text fixed for merge mode, and `integrate` warns instead of failing on it.
- B5 Operator alerts (`specs.md` §1).
- B6 After compaction, ness-entry's post-compact slot reloads the operator-communication and autonomy rules, not only brief and inbox.
- B7 A gate-worktree setup hook in `ness.toml` (the research lab's `data/` link).

## Batch C: ness program profile
- C1 Autonomy per program run: an explicit question after the roadmap is agreed and at checkpoints that change the map; modes gate-every-checkpoint / free within stated bands / run to checkpoint X / custom; recorded as `## Autonomy` in the roadmap; the production backend defaults go.
- C2 Doctrine at two levels: project doctrine (short, slow) and a program section at the top of the roadmap; reviewed keep/amend/drop at brainstorm; operator words only; spec acceptance and fold review cite the entries checked.
- C3 Review challenge step: specs reviewed from the file, rejected alternatives listed, "what would make this naive", refuse as an outcome; fold review by an independent subagent; briefs do not pin candidate solutions; the Socratic "which roadmap assumption does this weaken?" at every fold.
- C4 Brainstorm/refinement loop as an explicit multi-round phase whose exit is the operator accepting map plus program doctrine.
- C5 Less ceremony: no per-wake "nothing waits on you" chatter, children keep no decision ledgers of their own, checkpoints replace heartbeat prose.
- C6 Research profile (the research lab local): followability lens as the child's self-test before handoff; "stop and report world facts" rule.

## Batch D: port to the research lab (operator, 2026-10-15)
After B and C: build and install the new ness, `ness sync` in the research lab, restart its worker (memory: ness-update-path). Then reground the study-protocol root, once it is idle and not mid-work, on what changed and what it now owes (checkpoints, autonomy question, doctrine, review step).

## Order
B (B1-B4, B6, B7 small; B5 larger) and A in parallel, then C. C is profile text plus small checks and needs the operator's read of the wording, not a new design round.

## Batch E: every prose rule paired with a check (operator, 2026-10-15)
"Ness overall should always pair some gating static checks … for every thing that otherwise would only be enforced in prose, and it's really important that the check fails with a pointer to the relative prose piece, to steer the agent in the right direction." Three levels:
- E1 ness: inventory every rule in ness skills/profiles (core, builtin, features); classify as already checked / checkable statically (hook, CLI refusal, sync/audit) / judgement only; every check's failure message names the skill and section it enforces. Build the checkable ones. The Batch C autonomy check (`ness open` refuses a program child while `## Autonomy` is empty) is one instance.
- E2 ness docs: the profile authoring docs tell consuming projects to pair each local rule with a check that points back to its prose, and say which ness mechanisms a project can use for that.
- E3 the research lab: the same inventory and pass over its local profile, AGENTS.md and skills.
Started from this root with inventory subagents; promoted to a child session or plan if it grows.

### Batch E scope (root rulings on e1-ness-inventory.md and e3-research-inventory.md, 2026-10-15)
Build in ness (E1):
- E1.0 Every existing refusal names its skill and section (inventory §2 table, incl. audit.py RULE→skill map, forbidigo msg, commit-msg). Missing prose added first: no `&`/self-matching pgrep and no whole tool-result reads go into ness-entry Communicate. Child-cap message drops "or raise launch.max_children".
- E1.1 Pointer registry + meta-test (M5): every built-in refusal and profile check declares skill + section; a Go test verifies each resolves to a heading in the rendered skill. The MUST-sentence/judgement-tag coverage half is not built (prose churn, high noise).
- E1.2 Profile-declared checks (M2): a profile declares scripts run at `ness open` (child), `ness integrate`, `ness close`, each with skill + section; non-zero exit refuses with the script's text plus the pointer. Program ships: autonomy recorded before a child opens (3.1, mode word + date under `## Autonomy`; no transcript check of the words for now); checkpoint since the last child landing before the next open (3.5); no decision ledger with ids in a child's record at integrate (3.6); recovery line matches the ledger (3.7).
- E1.3 Project guards on pre-tool (M1): `[[guards]]` in ness.toml (tool, match, message, skill, section). Defaults: hand-run of the configured gate command in a bound session; session edits to `[launch]`/`[tiers]`/`[worker]`/max_children in .ness/ness.toml.
- E1.4 Small built-ins: waiting rule covers sleep loops around `ness integrate|send|open` (3.4); report needs `## Lessons` at close (3.8); Agent tool with subagent_type Explore refused (pointer: ness-entry Communicate); handoff body needs Done/Now/Next like a checkpoint; go-strict validate runs `ness contract check` when contracts is on.
- Reconcile PE:38 with FF-D6: ness-entry record says decisions.md ids are the root's; a child's decisions.md is plain dated notes without ids, never cited upward. Check 3.6 refuses child-style ids (`X-D<n>`, `**D<n>**`) in a child's record, not the file.
Not built (say why in the report): chat-only ask detector (B5 Your move already alerts the operator deterministically; revisit if the field shows the root still asks in chat only), Stop hook, audit plug-in rules, `ness check`, golangci additions to go-strict (first-adoption noise), doctrine presence and "cites P/G" checks (presence, not truth), root-does-too-much.
E2 (docs): a "Rules come with checks" section in the profile authoring docs (docs/profiles or README for consumers): local rule = prose section + check + message naming it; the mechanisms (gate, local profile scripts at open/integrate/close, `[[guards]]`, contracts, git hooks); the meta-test pattern.
E3 (the research lab, with Batch D port, in a research lab session): resolve AGENTS.md:114 vs experiment skill (ruling: run_in_background yes, shell `&` no, one heavy run at a time; the experiment skill's "never background" means `&`); pointers on all existing structure tests; shortlist items 1, 3, 4, 5; the research lab's own `[[guards]]` for hand-run pytest/ruff/mypy over the whole suite; C6 profile edits.

### Batch E execution (operator, 2026-10-15: "if so just open childs and leverage them in parallel")
Children: e-pointers s-e2eccad430d0 (tasks 1-4, 12-14; integrates 1-2 early), e-checks s-e0313b2db0fb (tasks 5-9; merges the root branch once the registry lands). Next: a third child or subagent for tasks 10-11 and 15 after both registry and task 6 land; task 16 at the root. Batch B fixes landed on the root branch (17cd1a0, b9364c1); a second-pass review runs alongside.
- Superseded (operator: "giving a session for each batch may be better, not splitting batch E only"): e-pointers and e-checks withdrawn and closed with --discard; e-checks' task 5 tests kept on `keep/e-checks-task5`. Batch E runs in one child, batch-e s-4776a53a4a19. Batch B's tail (second review, integrate to main, Telegram field check) stays at the root; Batch D (the research lab port, E3, C6, reground) gets its own research-lab session after E lands.
