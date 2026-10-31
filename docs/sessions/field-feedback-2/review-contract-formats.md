> [!NOTE]
> Archive: altered for the public release. Local paths were replaced; the content is otherwise as written.

# Review: contract formats (d91d0a2, branch `contract-formats`)

Reviewed against FF2-D6, `docs/discovery/findings/field-2/contract-formats-build.md` and R-CTR-047..054.
Worktree `~/src/ness/.claude/worktrees/agent-aa14a8b430b69203f` was left as it was. Probes ran
in a scratch clone. The two attack scripts are in the scratchpad (`zz_attack_formats.txtar`, `zz_attack2.txtar`); drop them
into `internal/cli/testdata/script/` to replay them.

`mise exec -- go test ./internal/session/ ./internal/cli/ ./internal/config/` passes on d91d0a2 (exit 0).

Verdict: the plain cases hold. Branch-added formats, reordering, deleting or renaming the file, a changed or removed id,
a broken JSON stream, a duplicate id, an edit to the judge file itself and a timeout all fail closed. They hold because
the formats come from the merge-base config and a missing file is a diff. The claim "a branch cannot weaken its own
judge" (FF2-D6, R-CTR-052, the skill) does not hold. Only the judge's first word is frozen, and the judge runs from the
worktree, so anything it loads from the repo is open to the branch. A1, A2 and A4 below were reproduced end to end:
`ness contract check` prints `ok` after a locked unit was weakened.

## High

### H1. The judge's dependencies are not frozen: weaken a sourced or imported file and check passes
`internal/session/contract_format.go:108-127` (`JudgeFile`), `:57` (`runUnits`, `cmd.Dir = root`, the worktree).
Only `words[0]` is frozen. The judge runs at the worktree root, so it loads whatever the branch has put next to it.

Repro A1 (sh `source`): base has `units = "tools/units.sh"`, and `tools/units.sh` is `. tools/lib.sh; extract`.
Session freezes `specs/a.fmt` (the lock holds `tools/units.sh`, not `tools/lib.sh`). On the branch:
`sed -i 's/unit b: two/unit b: WEAK/' specs/a.fmt` and rewrite `tools/lib.sh` so that `extract` prints the old units.
Result: `ok 8 frozen files`.

Repro A2 (Python stdlib shadowing, needs no helper at all): base has `units = "tools/pyunits.py"`, a shebang script that
runs `import json, re, sys` (the same shape as the skill's example). The branch weakens `py/a.fmt` and adds
`tools/json.py` with a `dumps` that prints the old body. The script's own directory is `sys.path[0]`, so the planted
module wins. Result: `ok`. This works against any Python judge, including the documented one.

Fix (pick one):
- (Preferred) Run the units command from a read-only export of the merge-base tree: `git archive <mb>` into a temp dir,
  used as cwd. The base's judge, with the base's helpers, then judges both the base text and the worktree text. This
  matches R-CTR-046, since the formats already come from base. It also closes H2, M1 and M2, and it removes the need to
  freeze the judge at all. Cost: one export per `check`. It can be cached per merge-base sha.
- Or add `deps = [globs]` to `Format` and freeze those globs, plus `dirname(judge)/*`, whole-file next to the judge.
  Then document that anything else the judge loads is unprotected.

### H2. A judge that reads its path argument from disk makes a project-wide format lock vacuous
`internal/session/contract_format.go:131-143` (`formatEntry`) passes the base text on stdin but the worktree path as
`$1`. If the extractor opens `$1`, the "locked" units are computed from the worktree file, so they always equal the
current units.

Repro A4: `units = "tools/disk.sh"`, where the script is `cat >/dev/null; sed -n '...' "$1"`, and `frozen = ["proj/*.fmt"]`.
On the branch: `sed -i 's/unit x: ex/unit x: WEAK/' proj/p.fmt`. Result: `ok`.

The skill invites this mistake: "Keep it deterministic and read only stdin and its argument"
(`profiles/features/contracts/SKILL.md`, new section).

Fix: the H1 export fixes it, because `$1` then resolves inside the base export: that is the base text for a project-wide
file, and absent (fail closed) for a session-only file. Cheaper extra guard: at freeze and in `formatEntry`, run the
extractor a second time with empty stdin and refuse if it prints any unit. Docs: "the path is a label for messages;
never open it".

## Medium

### M1. A `units` value whose first word is not a bare repo path is silently unprotected
`JudgeFile` returns "" with no diagnostic for `"tools/q.sh"` (quoted), `sh tools/x.sh`, `env X=1 tools/x.sh` and
`X=a/b tools/x.sh`.

Repro A3: `units = "\"tools/q.sh\""`. `freeze q/a.fmt` prints no judge, and its lock has no `tools/q.sh` line. The branch
weakens `q/a.fmt` and rewrites `tools/q.sh` to print fixed units. Result: `ok`.

Fix: at load (`validateFormats`), refuse a `units` whose first word does not resolve to a repo file unless the format
says so explicitly (`unprotected = true`). At least warn at `freeze`: "units command is not a repo file: unprotected".
H1's export fix makes this moot.

### M2. A project-wide judge that is a symlink makes check red forever on a pristine branch
`contract_project.go:66-69` adds the judge as whole-file. `CheckContracts` compares `git hash-object -- tools/link`
(git follows the link and hashes the target's content) with `rev-parse mb:tools/link` (the link text).

Repro: at base, `tools/link -> impl.sh`, `units = "tools/link"`, `frozen = ["proj/*.fmt"]`. A plain `ness contract check`
on base prints `differs tools/link .ness/ness.toml`. The operator cannot fix it except by changing the config.

Session locks happen to work, because both sides use `hash-object`. The guard resolves the link (`RealFile`), so an edit
to `impl.sh` gets no warning there.

Fix: in `JudgeFile`, resolve with `filepath.EvalSymlinks` and freeze the target path (refuse a target outside the repo).
Or refuse a symlinked judge at load.

### M3. The skill's pytest extractor example is itself weakenable
`profiles/features/contracts/SKILL.md`, the example regex `^def (test_\w+)\(.*?(?=^def |\Z)`. Text above the first
`def` belongs to no unit, and decorators land in the previous unit's body.

Repro: add `pytestmark = pytest.mark.skip` and `@pytest.mark.skip` above the first `def test_pays`. The extractor
output is byte-identical: same sha256 before and after.

A false positive follows from the same cause: a decorated new test added after a locked one shows up as the previous
unit "changed". Class methods (indented `def`) are folded into the preceding function's body.

Fix: use an `ast`-based example (each `FunctionDef`'s source with its decorators, plus a synthetic `__module__` unit for
the module-level statements other than the defs). Or say plainly that the example is a toy.

### M4. Running every project-wide extractor per path makes freeze and amend slow and fragile
`contract.go:177` (`refuseProject`) and `:332` (`freezeJudge`) call `e.project()`, which reruns `ProjectFrozen`. That runs
the extractor on every project-wide format file at base, for every argument. N args times M files gives up to 30 s each.

One base file whose extractor fails then blocks `freeze` and `amend` of unrelated files, with an error about a file the
caller never named.

Fix: compute `project()` once per command (memoize in `contractEnv`). For `refuseProject` and `freezeJudge`, use the
path set only (`projectPaths` plus judges), without extracting.

## Low

- **L1. The hook and check disagree on a project-wide format file that has no units at base.**
  `contract.go:533-536` (guard: always soft, "adding is fine") vs `contract_format.go:136-138` (check: whole-file).
  Repro A5: `empty/e.fmt` holds `nothing`. The hook prints "adding is fine". After appending a unit, check reports
  `differs empty/e.fmt`. Fix: in the guard, check the base blob for units without running the command (impossible), or
  word the soft message as "may fail check", or make a no-units project file soft in check too (lock zero units).
- **L2. The extractor gets the gate's full environment and can write to the worktree.** `runUnits` sets no `cmd.Env`.
  In the repro, the extractor wrote `leak.txt` with `SECRET_TOKEN=hunter2` into the repo root during `check`. The check
  also executes a judge the branch edited before reporting the judge diff. `withProjectOnly`'s comment ("reads the tree
  and git only") no longer holds. Fix: pass a minimal env (`PATH`, `HOME`, `LANG`) and say in the docs that check now
  runs project code. H1's export also keeps writes out of the worktree.
- **L3. A unit id over 64 KiB writes a lock nobody can read.** `ExtractUnits` accepts it. `ParseLock`'s
  `bufio.Scanner` drops the line, then reports "malformed contract.lock: the scenario lines of f do not match its
  entry", and every later contract command fails. Repro: an extractor printing a 70000-char id; `RecordUnits`, then
  `ParseLock(l.Bytes())`. Fix: cap the id (e.g. 1 KiB) in `ExtractUnits`; reject `\t` too, for safety.
- **L4. stdout and stderr buffers are unbounded** (`runUnits`). A runaway extractor can use as much memory as it can
  print in 30 s. Fix: `io.LimitReader`-style cap (e.g. 16 MiB) that fails closed.
- **L5. A process started with `setsid` outlives the timeout.** The group kill and `WaitDelay` return after about 2.3 s
  (verified), but `setsid sh -c 'sleep 30'` stays alive as an orphan. This is acceptable; worth a `ponytail:` note.
- **L6. The JSON protocol is looser than the spec.** `json.Decoder` accepts several objects on one line, pretty-printed
  objects, and case-insensitive keys (`{"ID":..,"BODY":..}` was accepted). R-CTR-048 says "one JSON line". Fix: scan
  lines and use `DisallowUnknownFields` plus an exact key check, or relax the spec text.
- **L7. A format from profile defaults is silently dead.** `formatOrder(meta)` (`config.go:487`) sees only the file's
  keys. A `contracts.formats.p` profile default decodes into `Formats` but not into `FormatOrder`, so `FormatOf` never
  matches it (verified). Fix: append the `Formats` keys missing from the order, sorted, or refuse formats in profiles.
- **L8. A base config change breaks historical format locks on main.** Renaming, reordering or removing a format, or
  changing its extractor output, breaks every merged session lock under it ("which no longer matches"). That reddens the
  gate for every branch. This is spec-intended (R-CTR-051), but neither the skill nor the spec gives the operator the
  procedure. Fix: one paragraph in the skill.
- **L9. Small issues.** `JudgeFile`'s `strings.HasPrefix(rel, "..")` misclassifies a repo dir named `..x`; use
  `rel == ".." || strings.HasPrefix(rel, "../")`. The timeout message says "timed out" without the limit. The
  malformed-lock error says "scenario lines" for format files.

## Checked and fine

- TOML key order: `formatOrder` gives the file order for `[contracts.formats.X]` tables, dotted keys
  (`zz.match` / `aa.match`) and inline tables (verified, all `[zz aa]`).
- Branch-added format: ignored when base has contracts on (txtar covers it). When base has no contracts, the branch
  config applies, as R-CTR-046 already says.
- Reordering formats on the branch has no effect (base config). New extension or deleted file: "file is missing".
  Changed id: shows as "removed". Exec bit dropped: the extractor fails closed.
- Reusing the `scenario` line: `badScenarios`' `HasSuffix(":"+digest)` is equivalent for `scn:` tokens. `FeatureScenarios`
  is never run on `fmt:` files (`lenientDiff` branches first). Format-before-builtin precedence is correct. No confusion
  found.
- Unit tests: no sleeps in test code. The timeout test uses a sleeping subprocess with a 3 s bound, which is fine. The
  hand-written fake is in testdata. The config order test would catch alphabetical ordering.

## Test gaps

- The build notes admit the txtar never showed a genuine red. Only a `freezeJudge` mutation was checked.
- There is no test for the "no units at base stays whole-file" path of R-CTR-053.
- There are no tests for symlinked, quoted or sourced judges. Add A1 to A4 as expected-failure cases once H1 lands.
- Docs: SKILL.md's "a branch cannot weaken its own judge" and R-CTR-052's "Editing it then fails check" are true only for
  the single file. Restate them after the fix.
