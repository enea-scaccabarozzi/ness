> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Review: contracts refinement (commit 50cb3c0, branch contracts-refine)

Reviewed: `git show 50cb3c0` in `.claude/worktrees/agent-a8d075d8d089a1978`, against
`contracts-value.md` "Refined design", `contracts-refine-build.md`, `docs/spec/contracts.md`,
`.agents/skills/ness-test-discipline`. I ran the tests on an exported copy of the commit:
`go test ./internal/session ./internal/config` passed, `go test ./internal/cli -run TestScript/contract`
passed, and `audit.py` was clean. I confirmed each parser bypass below with a throwaway unit test in
that scratch copy. The worktree itself is untouched.

## High

### H1. A locked scenario can be removed or weakened and `check` still passes (doc-string smuggling)
`internal/session/contract_scope.go:38-104` (`FeatureScenarios` / `gherkin.line` / `keyword`).
The parser reads `Scenario:` and `Rule:`/`Feature:` lines inside a doc string as keywords. An
implementer can copy the original locked scenario into the doc string of a new helper scenario and
close the copy with a `Rule:` line before the closing `"""`. The `Rule:` line flushes the block, so
the closing delimiter is dropped. Then they write the real scenario under the same name, weakened.
The parser files the copy under `pays` (same hash as the lock) and the real one under `pays #2`,
which counts as an addition. Cucumber/godog only runs the weakened one. Probe result: the hash for
`pays` matched, and the file had 3 entries.
```
Scenario: helper
  Given a note
    """
Scenario: pays
  When they pay
  Then it works
Rule: x
    """
Scenario: pays
  When they pay
  Then anything
```
They can also just delete the real scenario. The build record calls this a "misread" (`ponytail:`
note). It is not cosmetic: it breaks the main promise of item 1. Fix: track the open and close of
doc strings (`"""` / `` ``` ``) and keep their contents as body lines, never as keywords, comments
or tags. That is a few lines of state, so no real parser is needed. Add the case above as a unit test.

### H2. Feature-level and Rule-level tags are not hashed, so `@skip`/`@wip` on the Feature disables every locked scenario
`contract_scope.go:93-96`: `keyword("Feature"|"Rule")` sets `g.tags = nil`. Probe results:
`@skip\nFeature: ...` left every hash unchanged, and so did `@skip` above a `Rule:`. Tags are
inherited in Gherkin, and godog/cucumber tag filters (`~@wip`, `not @skip`) are the usual way to
skip scenarios. This weakens every locked scenario with a one-line addition. R-CTR-030's
"a tag ... changes exactly that entry" is true only for scenario-level tags. Fix: hash the Feature
tags into a pseudo-entry such as `Feature`, and fold Rule tags into each scenario under that Rule
(or into a `Rule <name>` entry).

### H3. Amendment authority can be forged by the agent it is meant to constrain
`internal/cli/contract.go:265-289` (`approved`) together with `internal/cli/session.go:60-83` (`identify`).
The caller is whoever `NESS_SESSION` names. If the variable is unset and no conversation is adopted,
the caller is the operator. The owner can therefore:
- run `NESS_SESSION=<manager id> ness contract amend X --reason y`. The manager passes `mayTouch`
  and `approved` returns nil, so no item is needed.
- as a root session, run `env -u NESS_SESSION -u CLAUDE_CODE_SESSION_ID ness contract amend X ...`
  and be the operator, which is its manager. `contract_rights.txtar` itself uses `env NESS_SESSION=`
  to act as the operator.
- run `NESS_SESSION=<manager id> ness send --to <self> ...` to mint a valid "approval" item.
- pass any item the manager ever sent it (a status question, a reply about something else). The
  item's content, file and amend intent are never checked, and one item approves any number of
  amendments of any file.

The build record admits only the Bash-forged `Amend` line. Yet the skill (`profiles/features/contracts/SKILL.md`)
says "you cannot approve your own amendment", and R-CTR-035 reads as an enforcement claim. Either
narrow the claim to "friction plus an audit trail: the reviewer checks the `[item ID]` against the
store", or close the cheap holes. Closing them could mean: require the item's body or intent to
name the path; mark an item as consumed once used; and have the manager or operator path record the
true process identity (the hook's bound session) instead of trusting `NESS_SESSION`. The env-var
impersonation also affects other commands, so it is out of scope here. It should still be named in
the spec deviation.

## Medium

### M1. Unnamed scenario produces a lock that its own parser rejects
`contract_scope.go:98-101` and `contract.go:31` (`scenarioLine` needs `\t(.+)$`). `Scenario:`
with no name records the key `""`, and `Bytes()` writes `scenario <h> a.feature\t`. Confirmed:
`ParseLock(l.Bytes())` returns `ErrBadLock`. After that `freeze` succeeds, but every `check` fails
closed. The guard sees `FindLocks` fail and allows all edits with a warning (`contract.go:414-418`),
so the hook goes silent for every frozen file in the repo. Fix: name unnamed blocks (for example
`(unnamed)`, then `#2`...) or allow `(.*)`. Add a round-trip test.

### M2. A new Background (or Rule-level Background) silently changes every locked scenario
`contract_scope.go:99-101`. The Background is its own entry, so adding one where none existed (or a
`Background #2` under a new `Rule:` that wraps locked scenarios) counts as an addition. Its steps
still run before every locked scenario, for example `Given payments are stubbed to succeed`. Fix:
treat a Background that is absent from the lock as an edit of every locked scenario it would apply
to. A cheaper option: fold the governing Background's hash into each scenario's hash.

### M3. Other line-level blind spots of the parser
`contract_scope.go:70-84`.
- A doc-string line starting with `#` is dropped. Confirmed: `# Total: 5` -> `# Total: 9` inside a
  locked doc string passes.
- A doc-string line starting with `@` is taken as a tag. It moves to the next scenario's hash, and
  if no scenario follows it is dropped, so editing it in the last scenario passes.
- Doc-string indentation is collapsed.

H1's doc-string state fixes all three.

### M4. `[contracts] prefix` applies to any file type, and the implementer controls it
`contract.go:217-230`. The glob is matched at freeze time against the branch's own
`.ness/ness.toml`. The spec says the prefix lock is for Markdown, but nothing restricts it. A
`prefix = ["*"]` added on the branch before `freeze steps.go` gives the judge an append-only lock,
and appending an `init()` or an override to Go code is a weakening. Fix: apply prefix only to
`.md`. Optionally refuse a prefix glob that does not exist at the merge-base config.

### M5. Spec and code disagree on who may approve
`contract.go:285`: the item must come from `e.c.sess.Parent`. R-CTR-035 and the skill say "from
its manager or the operator". For a non-root session, an operator item is refused. Separately, the
`approved` comment and R-CTR-035 say the operator amends on its own authority, but `mayTouch`
(R-CTR-020) refuses the operator on a non-root lock. Make the spec and the code agree. No test
covers a non-root owner with a manager item, a reply item, or the operator case.

## Low

- **L1. One Amend line licenses every later change** (`session/contract.go:316-331`, `amendLogged`
  counts more than zero lines since mb). After one legitimate amend and re-freeze, a Bash rewrite of
  the lock (with its digest recomputed, since sha256 of public data is not a secret binding) plus the
  file passes `check`. This was already true for whole-file locks; the scenario and prefix tokens
  inherit it. The build record's "cannot be defeated by editing the lock" holds only until the first
  amendment. Also note that the digest only detects inconsistent hand edits. An agent can recompute
  it. Protection actually comes from `Unamended`'s comparison with the reference token.
- **L2. `ParseLock` accepts orphan `scenario` lines** for a path with no entry (confirmed). They
  are harmless but carried forward by `Bytes()`. Reject them as malformed for consistency with R-CTR-031.
- **L3. Prefix lock of an empty file is `prefix:0:...`, so any content passes.** Refuse it at freeze,
  or document it.
- **L4. `_`/`*` stripped anywhere** (`a_b` == `ab`) and `\` stripped. This is a documented trade.
  Stripping only at word boundaries would be just as cheap.
- **L5. Duplicate names: reordering swaps identities** (documented). With H1 fixed, an inserted
  same-name copy ahead of the original still runs the original, so this is not a bypass on its own.
- **L6. Codex loses the warning** (documented). R-CTR-036 should state that it is Claude Code only.

## Test honesty (ness-test-discipline)
- The rules hold: no sleeps, helpers call `t.Helper()`, files are under 400 lines, and audit is clean.
- Fixtures are inline (`const feature`, the spec string in `contract_scope_test.go:13-31,170`).
  The skill says fixtures live in `testdata/`. This is a minor violation. The txtar fixtures are fine.
- The tests pin the happy paths named in the spec, but none tries an adversarial edit (H1, H2, M2,
  M3), which is exactly where the design makes its claim. The "a tag changes exactly that entry"
  test uses only a scenario tag.
- The R-CTR-035 test covers only the root plus operator case. There is no case for a non-root
  manager item, a reply item, or reusing an unrelated manager item.
- Older scripts were renamed from `.feature` to `.txt` to keep whole-file behaviour. That is
  acceptable, and the legacy `.feature` entry has its own case in `contract_refine.txtar`.

## Docs and spec
- R-CTR-031's "a lock whose scenario lines were edited is malformed" is accurate only for edits
  that do not recompute the digest. Say so, and point to R-CTR-021 as the real defence.
- R-CTR-030 does not mention Rule or Feature tags. Add them once H2 is decided.
- The skill's "you cannot approve your own amendment" overstates (H3).
