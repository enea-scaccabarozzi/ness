---
name: ness-contracts
description: Freeze files you agreed on; the gate checks them, and changing a frozen file needs an approved amendment.
---

# ness contracts

## Frozen files

- `ness contract freeze PATHS...` — lock each file in your `contract.lock`. What is recorded depends
  on the file:
  - a `.feature` file is locked scenario by scenario (name plus normalised steps, Examples and
    Background included). Adding scenarios and reordering are free; editing or removing a locked
    scenario fails the gate, naming it. Tags on the Feature or a Rule and any Background count as
    part of each scenario they reach, as do doc-string lines (verbatim).
  - a `.md` file matching `[contracts] prefix` (globs, `path.Match` syntax, no `**`, e.g.
    `experiments/*/spec.md`) is an append-only prefix: its words are locked, formatting is free,
    appended text is free (put it under a dated "Amendment" heading).
  - anything else is locked by its blob sha, whole file. Freeze the judge, not only the spec: the
    step definitions and fakes that decide pass or fail are frozen the same way, e.g.
    `ness contract freeze features/pay.feature features/steps/pay_steps.go`.
- `ness contract amend PATH... --reason TEXT --item ITEM` — open frozen files for change. ITEM is
  an item in your inbox from your manager or the operator (or a reply by them) whose text names
  PATH, never one you sent. Without it, ask first:
  `ness send --to manager --intent request --body "amend PATH: why"`; the reply must name PATH
  too. Then amend with the reply's id. The reason and the item go to the session's
  `decisions.md`. `freeze` it again when done. Your manager and the operator amend on their own
  authority and need no item. This is an approval trail and friction against drift, not a
  security boundary.
- `ness contract check` — the gate's check: every frozen file still holds (blob equal, locked
  scenarios unchanged, locked prefix intact), or is open; exit 1 names each one that differs and,
  for scenario and prefix locks, what.

Project-wide paths take the same modes, compared with their text at the base branch. List paths
or `path.Match` globs (no `**`) in `.ness/ness.toml`:

```toml
[contracts]
frozen = ["features/*.feature", "experiments/*/spec.md", "features/steps/*.go"]
prefix = ["experiments/*/spec.md"]
```

Here every `.feature` is scenario-locked (base scenarios must stay, new ones are free), each
`spec.md` is an append-only prefix, and the step files are whole-file. You may create a file a
glob matches, but `check` reports it (`added\tPATH`) and the gate fails until it is amended. To
change, add or delete project-wide paths, ask the operator, then run
`ness contract amend PATH... --reason TEXT --authorized "their words"` (the words must be in a message
they typed to you; name every path in one call). It logs `Amend PATH (project-wide): ...` to your `decisions.md` and the path
is open on this branch; `--item` does not apply, and `freeze` still refuses project-wide paths.
A path whose content equals the base branch's tip (local or `origin`) passes: what is already on
base, brought in by a merge or a squash, needs no amendment. A `contract.lock` an old harness wrote (JSON) that is unchanged from base is
skipped with a warning; one you add or edit fails the gate.

## Project-defined formats

A project can define its own unit format in `.ness/ness.toml` (the base branch's copy counts; a
format added on a branch is ignored):

```toml
[contracts.formats.pytest]
match = ["tests/test_*.py"]      # globs, no **; the first format that matches wins, before the builtins
units = "tools/units.py"         # an executable script in the repo, committed on the base branch
```

`freeze` (and `frozen` globs, `amend` + `freeze`) then lock a matching file unit by unit. ness runs
`units` through `sh -c` in an export of the merge-base tree, with only `PATH`, `LANG` and a
throwaway `HOME`/`TMPDIR`, the file's text on stdin and its path as the last argument. The path is
a label: the file itself is not in the export, so read stdin only (a command that prints units from
empty stdin is refused). It prints one JSON line per unit, exactly
`{"id": "test_pays", "body": "<the unit's text>"}` (an id of at most 1 KiB, no tab; output at most
8 MiB). New ids are free; a locked id that is missing or whose body changed fails the gate, named
(`unit "test_pays" changed`). An extractor that exits non-zero, times out (30 s), prints invalid
JSON or repeats an id fails closed. Keep it deterministic. Cover what makes a test weaker, not just
its body: decorators (`skip`) and module-level marks count.

```python
#!/usr/bin/env python3
import ast, json, sys
tree, rest = ast.parse(sys.stdin.read()), []
for node in tree.body:
    if isinstance(node, (ast.FunctionDef, ast.AsyncFunctionDef)) and node.name.startswith("test_"):
        print(json.dumps({"id": node.name, "body": ast.dump(node)}))  # decorators are in the dump
    else:
        rest.append(ast.dump(node))  # imports, pytestmark, fixtures
print(json.dumps({"id": "__module__", "body": "\n".join(rest)}))
```

What is protected: everything the command loads from the repo (the script, files it sources, a
module next to it) is read from the merge-base, so a branch that weakens a test cannot also change
the judge. What is not: the interpreter and anything on `PATH` or installed on the box. If the
first word of `units` is not a file of the merge-base (`python3 -c ...`, a quoted path), `freeze`
warns that the extractor comes from the environment. The extractor must be committed on the base
branch before it can run. The hook warns on a format file like a scenario lock.

Changing a format on the base branch (renaming it, changing which files match, or an extractor
whose output changes) breaks every merged lock under it: `check` reports `locked under format "x"
...` or the units that differ, on every branch. Re-freeze on a branch off the new base: `amend`
each file (with the approval item), `freeze` it again, and merge; or move the change in two steps
(add the new format first, switch the files over, then remove the old one).

Edit, Write and MultiEdit on a whole-file frozen file are refused by the pre-tool hook. On a
scenario or prefix lock the hook only warns, since additions are legal; the gate is what
enforces. Changes made through Bash are caught by `ness contract check`. Project-wide frozen
paths are listed in `.ness/ness.toml` under `[contracts] frozen`. A `.feature` frozen by an older
ness stays whole-file until it is amended and frozen again.
