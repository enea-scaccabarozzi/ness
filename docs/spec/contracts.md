> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Contracts (CTR)

Contract logic in `internal/session`, guard in `ness hook pre-tool`; cli `contract
freeze|amend|check`. Sources: §2 (features), §5 (`ness contract`, hooks), §11 (behavior-specs
scenarios frozen), migration P2 (worktree path resolution), roadmap L8.

- **R-CTR-001** (CHANGED: `.feature` and `prefix` files record scenarios/prefix, R-CTR-031, -033) `ness contract freeze PATHS` MUST record the git blob sha of each other named file in
  `contract.lock` in the calling session's record directory, a committed file.
  Verify: testscript — `<sessions>/…/contract.lock` lists each path with `git hash-object PATH`.
- **R-CTR-002** `freeze` on a path that does not exist MUST be refused.
  Verify: testscript — exit non-zero; nothing recorded.
- **R-CTR-003** (CHANGED: scenario and prefix locks hold per R-CTR-032, -034; project-wide paths per R-CTR-040..042; a legacy lock per R-CTR-044) `ness contract check` MUST verify every `contract.lock` on the branch and the
  `[contracts] frozen` paths, exiting 0 when every frozen file's blob equals its recorded sha (or
  is open for amendment) and 1 otherwise, naming each differing file.
  Verify: testscript — two session locks; edit a file frozen in either → exit 1 naming it;
  revert → 0. A Bash-written change (`sed -i`) is caught here.
- **R-CTR-004** `ness contract check` MUST be usable in the gate (non-interactive, no writes).
  Verify: testscript — tree and store unchanged after check.
- **R-CTR-005** (CHANGED: owner needs `--item`, R-CTR-035; several paths, field feedback 3) `ness contract amend PATH... --reason TEXT` MUST open exactly the frozen files named for
  change and append `Amend <path>: <reason> (<YYYY-MM-DD>)` to the session's `decisions.md`, one
  line per path. Every path is validated before anything is written: one that is refused writes nothing.
  Verify: testscript — guard allows edits to PATH only; decisions.md gained exactly that line;
  `contract_project_amend` — two paths give two lines, a third not frozen refuses all and logs none.
- **R-CTR-006** `amend` MUST require `--reason`.
  Verify: testscript — no reason → refused.
- **R-CTR-007** `amend` on a file that is not frozen MUST be refused.
  Verify: testscript — exit non-zero.
- **R-CTR-008** Running `freeze PATH` on an amended file MUST record its new sha and close the
  amendment.
  Verify: testscript — after re-freeze, guard refuses edits again; check passes with new content.
- **R-CTR-009** (CHANGED: scenario and prefix locks, project-wide ones too, only warn, R-CTR-036, -043) The pre-tool hook MUST refuse an Edit, Write or MultiEdit call (Claude Code) or an
  `apply_patch` call (Codex) on a frozen file that is not open for amendment, naming the file and
  `ness contract amend`.
  Verify: contract(fake+real) — each tool on a frozen file → blocked; message names the command.
- **R-CTR-010** The guard MUST resolve the repository from the edited file's own worktree first,
  not from `CLAUDE_PROJECT_DIR` (P2 regression: an adopted root's project dir names the primary
  checkout while files are edited in a session worktree).
  Verify: testscript — `CLAUDE_PROJECT_DIR` = primary checkout, edit target in
  `.worktrees/x/frozen.feature` → refused.
- **R-CTR-011** The guard MUST allow edits to files that are not frozen.
  Verify: testscript — Edit on an unfrozen file → allowed, no output.
- **R-CTR-012** The guard MUST recognise frozen files given as relative paths, absolute paths and
  paths through symlinks.
  Verify: unit — the three spellings of one frozen file are all refused.

## Decided details

- **R-CTR-020** (CHANGED: the operator may amend any lock) `freeze` and `amend` MUST be allowed
  only to the session owning the `contract.lock` and to its manager; `amend` also to the operator.
  Verify: testscript — a sibling runs `amend` → exit 4; the manager → ok; the operator on a
  non-root lock → ok.
- **R-CTR-021** (CHANGED: a scenario or prefix lock's own token is its reference) The guard MUST NOT inspect Bash commands; Bash-made changes to frozen files MUST
  be caught by `contract check` in the gate. `check` trusts no lock content the branch wrote: a
  frozen file whose blob differs from its reference sha (the merge-base lock's, or the commit that
  first froze it) needs an `Amend <path>:` line added on the branch to the lock's `decisions.md`.
  Verify: testscript — `sed -i` on a frozen file passes the hook and fails `contract check`; so does
  a Bash re-sha of the lock or a Bash-written `amend` line; `amend` then `freeze` passes.

## Refinement (field feedback 2): scenarios, prefixes, authority

Changed by this round: R-CTR-001, -003, -005, -009, -020, -021 (marked CHANGED above). New ids follow.
Evidence: `docs/discovery/findings/field-2/contracts-value.md`, "Refined design".

- **R-CTR-030** `session.FeatureScenarios` MUST parse Gherkin line by line into scenario name ->
  hash of the normalised text of each Scenario and Scenario Outline (its tags, steps, tables and
  Examples) plus its context: the Feature's tags, the enclosing Rule's tags and every Background
  in scope (Feature-level and Rule-level). Whitespace is collapsed and comment lines dropped,
  except inside a doc string (`"""` or a code fence), whose lines are kept verbatim, indentation
  included, and never read as keywords, comments or tags. A repeated name gets " #2", " #3"; an
  unnamed scenario is `<unnamed N>`. A Background is no entry of its own: adding or editing one,
  or a tag above Feature or Rule, shows as an edit of the scenarios it reaches.
  Verify: unit — reflowing and comments keep the hashes; a step, an Examples row, a scenario tag
  changes that entry; a Feature tag, a Rule tag or a Background changes every scenario it reaches;
  a locked scenario copied into a doc string closed by a `Rule:` line does not stand in for the
  real one; `#`, `@` and indentation inside a doc string count.
- **R-CTR-031** `freeze` of a `.feature` file (not matched by `prefix`) MUST record, instead of
  its blob sha, the entry `scn:<digest>` plus one `scenario <hash> <path>\t<name>` line per
  scenario in the lock; the digest binds the lines to the entry, so a lock whose scenario lines
  are inconsistent with it (a line removed, edited, or with no entry) is malformed (`check` fails
  closed). The digest is not a secret: a Bash edit that recomputes it is caught by R-CTR-021
  (the lock's token against its reference), not here.
  Verify: unit — lock round-trips (an unnamed scenario included); a removed or orphan scenario
  line is `ErrBadLock`. testscript — the lock holds one `scn:` entry and two `scenario` lines.
- **R-CTR-032** `check` MUST pass a scenario-locked file while every locked scenario still exists
  with the same hash (added scenarios, reordering and layout are free) and MUST fail naming the
  file and each locked scenario edited or removed, unless the file is open for amendment.
  Verify: unit and testscript — addition, reorder pass; edit and removal print
  `differs<TAB>path<TAB>lock<TAB>scenario "X" edited|removed`.
- **R-CTR-033** `[contracts] prefix = ["glob", ...]` (`path.Match` syntax, repo-relative, no `**`)
  MUST make `freeze` of a matching `.md` file record `prefix:<N>:<sha256>` of its first N words,
  where words ignore line wrapping, trailing spaces, list markers (`-`, `*`, `+`, `1.`, `1)`),
  emphasis marks (`*`, `_`) at a word's edge and backslash escapes; `a_b` and `ab` differ. A glob
  matching a file that is not `.md` is ignored: that file freezes as usual. An empty file is
  refused. A prefix match wins over the `.feature` rule.
  Verify: testscript — a matching `.md` locks as `prefix:`, a matching `steps.go` stays
  whole-file. Unit — an empty file is refused.
- **R-CTR-034** `check` MUST pass a prefix-locked file while its words still start with the
  locked words, and fail naming the file otherwise (an edit, or text inserted before the end of
  the locked part).
  Verify: unit — reflow, markers, emphasis and appended text pass; an edited number, a preface and
  a truncation fail. testscript — the same through `ness contract check`.
- **R-CTR-035** `amend` by the lock's owner MUST require `--item ITEM`: an item in the store
  addressed to the caller whose sender is the caller's manager (its parent) or the operator (a
  reply by them is such an item), never one the caller sent, and whose body names the path being
  amended (substring). Without a valid item it MUST be refused (exit 4) with the route "ask your
  manager: ness send --to manager ...". The item id is appended to the `Amend` line in
  `decisions.md` (`[item ID]`). The lock's manager and the operator (any lock, a non-root
  session's included, R-CTR-020) amend on their own authority. This is an approval trail and
  friction, not a security boundary: the caller is whoever `NESS_SESSION` names, as everywhere in
  ness, so a session that impersonates its manager defeats it. The store is outside the clone, so
  the gate cannot re-verify the item; the `Amend` line is what `check` still requires (R-CTR-021).
  Verify: testscript — no item, unknown item, an unrelated item, an item about another file, own
  item → refused naming the route; a root's operator item, and a non-root owner's manager reply →
  opened and logged with `[item ID]`; the operator amends a non-root lock without an item.
- **R-CTR-036** The pre-tool hook, on a file locked by scenario or prefix, MUST allow the edit
  and print a PreToolUse `additionalContext` warning naming the file, that editing or removing
  locked text fails `contract check`, and the route (ask the manager, then `amend --item`). It
  MUST still refuse a whole-file frozen file (R-CTR-009). The warning is the Claude Code hook
  output; Codex gets none, and there the gate is the only enforcement.
  Verify: testscript — exit 0 with the warning for `pay.feature`; exit 2 for `steps.go`.
- **R-CTR-037** Every other path, and an existing whole-file entry for a `.feature`, MUST stay
  whole-file (blob sha) until it is amended and frozen again. `freeze` accepts any repo path, so
  step definitions and fakes are frozen the same way as the spec.
  Verify: testscript — `steps.go` and a legacy `.feature` entry differ on any edit.

## Project-wide locks and legacy locks (field feedback 2, round 3)

Changed by this round: R-CTR-003, -009 (marked CHANGED above); R-CTR-036 and -037 apply to
project-wide paths through R-CTR-040..043. A project-wide path is amended on the base branch by
the operator, or on a session branch with the operator's typed words (R-CTR-056); `freeze` refuses it.
Evidence: the two consuming projects' field trials (`docs/discovery/findings/field-2/contracts-project-build.md`).

- **R-CTR-040** (CHANGED R-CTR-001's whole-file-only project rule; new files per R-CTR-055) Each `[contracts] frozen` entry
  MUST be a repo-relative path or a `path.Match` glob (no `**`; metacharacters `* ? [ \`). A glob
  stands for every file that exists at the merge-base with the base branch and matches it; a file new on the branch is
  not frozen, but `check` reports it (R-CTR-055). A literal entry stands for itself and fails `check` when absent at the merge-base. Every
  "at base" of R-CTR-040..044 means the merge-base (R-CTR-046).
  Verify: unit — a glob over `features/*.feature` freezes the base's files, not a new one; a
  literal absent at base fails. testscript — `contract_project.txtar`.
- **R-CTR-041** A frozen `.feature` path MUST be scenario-locked against its text at base (the
  R-CTR-032 rule: every scenario present at base still exists unchanged, additions, reordering and
  layout free); `check` names the file and each scenario edited or removed, with `.ness/ness.toml`
  as the lock.
  Verify: unit and testscript — an added scenario passes; an edited base scenario prints
  `differs<TAB>path<TAB>.ness/ness.toml<TAB>scenario "X" edited`.
- **R-CTR-042** A frozen `.md` path matching `[contracts] prefix` MUST keep the words of its base
  text as the start of the file (the R-CTR-034 rule); every other path stays whole-file, equal to
  its blob at base (R-CTR-037). A prefix-glob `.md` that is empty at base is whole-file.
  Verify: unit and testscript — appended amendment passes; an edited word fails; `steps.go` fails
  on any edit.
- **R-CTR-043** The pre-tool guard MUST apply the R-CTR-036 rule to project-wide paths: allow and
  warn (`additionalContext`) for a scenario- or prefix-locked path, refuse a whole-file one; a
  glob match absent at base is not frozen. The guard needs `base`, `prefix` and `frozen` from the
  config.
  Verify: unit and testscript — warning for `features/pay.feature` and the prefix `.md`, exit 2
  for `steps.go`, silence for a new `.feature`.
- **R-CTR-044** (CHANGED R-CTR-003's fail-closed rule) `check` MUST skip a `contract.lock` that
  does not parse when it is byte-identical at the base branch (an old harness's JSON lock),
  printing a one-line `warning:` on stderr naming it; one the branch adds or modifies MUST still
  fail closed, and removing one on the branch MUST NOT fail. A lock version that does not
  parse, at the merge-base or in any commit of the branch (a legacy lock the branch added long ago
  and base since deleted), holds no paths and is skipped in the history walk. The guard and `freeze`/`amend` tolerate the same locks.
  Verify: unit — unchanged skipped and named, modified and added fail. testscript — JSON lock on
  base: ok plus warning; modified or added on the branch: exit 1; removed: ok. Unit — a legacy lock
  committed on the branch is skipped by the history walk.
- **R-CTR-045** (CHANGED by R-CTR-056: `amend` opens a project-wide path with the operator's words)
  A project-wide path MUST be checked on its own, beside any session lock that holds
  the same path: no lock entry (an `amend` line, forged by hand or opened with `ness contract
  amend` before a glob covered the path) hides it, and none opens it. `freeze` MUST refuse any
  path the project-wide config covers, and so did `ness contract amend` until R-CTR-056. The guard gives the stricter of
  the project-wide and the lock verdict, and decides a project entry's mode as `check` does: a
  literal absent at base, or a prefix `.md` empty there, is whole-file and refused.
  Verify: unit and testscript — a `contract.lock` with `amend features/pay.feature` leaves
  `check` at exit 1 and the guard at its warning; `amend` and `freeze` of `steps.go` exit 4.
- **R-CTR-046** `check`, `freeze`, `amend` and the guard MUST compare with the merge-base of the
  base branch and HEAD, as the history walk does (R-CTR-021), so a base that moved on after the
  fork fails no branch that never touched it. When `.ness/ness.toml` at the merge-base has the
  `contracts` feature on, they MUST take `[contracts]`, `sessions` and the feature from that copy,
  not the worktree's: a branch cannot weaken the lock by editing its own config (`frozen = []`, a
  wider `prefix`, `features` without `contracts`). Changing the config is done on the base branch.
  `base` itself is read from the worktree to find the merge-base, and the command is refused
  when the merge-base's config names another one; the base MUST be a ref the branch does not
  create. Without a config at the merge-base, or with the feature off there, the branch's own
  config applies (contracts adopted on this branch).
  Verify: testscript — a base edited after the fork passes; `frozen = []` or `features = []`
  on the branch still fails; `base = "HEAD"` is refused.

## Project-defined formats (field feedback 2, FF2-D6)

A consuming project defines its own contract format, e.g. pytest test functions, Go table tests or
a BDD flavour, instead of one more fixed mode. The builtin modes stay built in.
Evidence: `docs/sessions/field-feedback-2/decisions.md` (FF2-D6),
`docs/discovery/findings/field-2/contract-formats-build.md`.

- **R-CTR-047** `[contracts.formats.NAME]` MUST have `match` (a non-empty list of `path.Match`
  globs, no `**`) and `units` (a non-empty command); `NAME` MUST match `[a-z][a-z0-9-]*`. Load
  refuses a format that does not. A file is in the first format, in the order the file writes
  them, whose glob matches it, before the builtin modes (scenario, prefix, whole-file). Like the
  rest of `[contracts]`, the formats are taken from the merge-base copy of `.ness/ness.toml`
  (R-CTR-046): a format the branch adds is ignored.
  Verify: unit — bad names, missing keys, `**` and a bad glob refused; config order kept.
  testscript — `contract_formats.txtar`, a `[contracts.formats.mine]` on the branch freezes
  whole-file.
- **R-CTR-048** To read the units of a file, ness MUST run `sh -c` with the `units` command and the
  path appended as the last positional argument, in an export of the merge-base tree (`git
  archive`, made once per command and removed after; the file under test is not in it), with only
  `PATH`, `LANG`, and `HOME` and `TMPDIR` set to the export, the file's text on stdin (the
  worktree text for a session lock, the text at the merge-base for a project-wide path), and a 30 s
  timeout. The path is a label: stdin is the only source of the text. The command prints one JSON
  line `{"id": STRING, "body": STRING}` per unit, exactly those two keys; an `id` is 1 to 1024
  bytes without a tab or newline; stdout is at most 8 MiB. Once per format and command, ness MUST
  also run it with empty stdin, and a command that prints any unit then is refused ("must read
  the file from stdin"). A command that exits non-zero, times out, prints invalid or looser JSON,
  an `id` out of range, too much output, or repeats an `id` MUST fail closed: `freeze` is refused,
  `check` reports the file. Past the timeout the process group is killed; a process that left it
  (`setsid`) is not.
  Verify: unit — each failure; the path with a space arrives intact; a stdin-ignoring and a
  disk-reading command refused. testscript — exit 1 and a repeated id differ;
  `contract_formats_disk.txtar`, `contract_formats_env.txtar`.
- **R-CTR-049** `freeze` of a format file MUST record the token `fmt:NAME:DIGEST` and, for each
  unit, a line `scenario HASH PATH<TAB>ID` (HASH is the sha256 of the body), the lines stored and
  bound to the entry by DIGEST exactly as a scenario lock's (R-CTR-031); a hand edit that does
  not match is a malformed lock. A file with no units is refused. `amend` and a new `freeze`
  work as for any lock.
  Verify: unit — round trip, tampering refused, no units refused. testscript — the lock lines;
  amend then freeze relocks.
- **R-CTR-050** `check` MUST hold a format-locked file to every locked unit: each locked id is
  present with the same body hash, new ids are free. A missing or changed id differs, printing
  `unit "ID" removed` or `unit "ID" changed`, one per unit.
  Verify: unit and testscript — an added unit passes; a changed and a removed unit are named.
- **R-CTR-051** `check` MUST report a file locked under format A that now matches another format
  or none, naming both, and a units command that fails on it (R-CTR-048).
  Verify: unit — other format, no format, failing command.
- **R-CTR-052** The judge is the merge-base's: everything the units command loads from the
  repository (the script, a file it sources, a module next to it, a symlink) is read from the
  export of R-CTR-048, so a branch that weakens a unit cannot also change what judges it, and the
  extractor MUST exist at the merge-base. Not protected is whatever the command takes from the
  environment: the interpreter, a tool on `PATH`, a library installed on the box. When the first
  word of `units` is not a regular file of the export (`python3 -c ...`, a quoted or absolute
  path), `freeze` MUST print a warning that the extractor comes from the environment. A format
  added or changed on the branch is not used (R-CTR-046).
  Verify: testscript — `contract_formats_bypass.txtar`: a sourced helper, a shadowed stdlib module,
  a quoted path and an edited extractor on the branch each leave the weakened unit named; the
  warning; a symlinked extractor checks clean.
- **R-CTR-053** A `[contracts] frozen` entry that matches a format file MUST lock it unit by unit
  from its text at the merge-base, run as in R-CTR-048. A file with no units there is still a
  format lock with none locked, so adding units passes, as the guard says. A units command that
  fails on a base file fails `check`.
  Verify: unit — an added unit passes, a changed one is named, an edited extractor changes
  nothing, no units at base takes additions. testscript — `contract_formats.txtar`, and
  `contract_formats_bypass.txtar` (the hook and `check` agree on such a file).
- **R-CTR-054** The pre-tool guard MUST treat a format file, session-locked or project-wide, as a
  scenario lock (R-CTR-036, R-CTR-043): allow the edit and warn, naming the route; it never
  runs a units command.
  Verify: unit and testscript — warning for the format files.
- **R-CTR-055** (CHANGES R-CTR-040 for new files; operator request m-tw2j5gkv) `ness contract
  check` MUST report each worktree file that matches a `[contracts] frozen` glob of the merge-base
  and is absent there (`added\tPATH\t.ness/ness.toml (new under frozen glob G)`) as a problem,
  unless a project-wide amendment for it is logged (R-CTR-056). The guard still allows creating it.
  Verify: testscript `contract_added`.
- **R-CTR-056** `ness contract amend PATH... --reason TEXT --authorized WORDS` MUST open each project-wide
  path named (one deleted on the branch too) for a session when WORDS are in a message the operator typed to it (checked as for
  `close --authorized`, R-SESS-121), and refuse (exit 4) without them, naming the route. It
  appends `Amend PATH (project-wide): REASON [operator: "WORDS"] (DATE)`, one line per path, to the session's
  `decisions.md`. `check` and the guard MUST accept a differing or added project-wide path when the
  `decisions.md` files under `sessions` hold more such lines for it, counted together, than at the
  merge-base (so a moved or copied record opens nothing) (a record,
  like R-CTR-021, not a security boundary). Once the branch lands, the base holds the new text.
  Verify: testscript `contract_project_amend`, with a frozen file deleted on the branch opened and
  passing `check`.
- **R-CTR-057** (CHANGES R-CTR-046 for content base already holds; field feedback 3) `ness contract
  check` MUST pass a project-wide path, or a file added under a frozen glob (R-CTR-055), whose
  worktree content is byte-identical to its blob at a tip of the base branch that is at or ahead of
  what the branch knows: `refs/heads/<base>` when the merge-base is its ancestor, or
  `refs/remotes/origin/<base>` when the local base is its ancestor (origin level or ahead, never
  behind or diverged), read by full ref name as they are (no fetch), so a tag or other ref named like
  the base shadows neither. A path deleted on the branch passes when it is also absent at such a tip. The content is approved there by definition,
  whatever the path's mode (whole-file, scenario, prefix, format). The globs, modes and config still
  come from the merge-base (R-CTR-046); the guard is unchanged. It covers a branch whose link to base
  was lost (merged base, then squash-folded onto the old fork point) and a local base left stale by a
  push made outside ness. The rule is a record, like R-CTR-021 and R-CTR-056, not a security
  boundary: a session can still write `refs/remotes/origin/<base>` itself (`git update-ref`), which
  no local check can tell from a fetch; the ancestry rule keeps that ref from passing a revert.
  Verify: testscript `contract_base_tip` — a squash-folded branch carrying base's edited, added and
  deleted frozen files passes; the same file changed otherwise fails; only `origin/main` holding the
  text passes; with neither tip holding it every edit is reported; with `origin/main` behind the
  local base, restoring origin's older text fails and deleting a file only the local base added
  fails; a tag `main` on a side commit holding the edit passes nothing.
