---
title: Freeze acceptance files
description: Freeze the files that judge the work, so a session cannot weaken its own tests to pass them.
---

A **contract** is a file frozen so the work cannot bend it: an API schema, an acceptance
scenario, an experiment's protocol. `ness contract check` fails when a frozen file drifts. Put it
in the gate, and a branch that edits its own judge cannot land.

## Prerequisites

- The `contracts` feature on. `ness sync` then renders the `ness-contracts` skill.

  ```toml title=".ness/ness.toml"
  features = ["contracts"]
  ```

- `ness contract check` in your gate script.

## Freeze files for the whole project

1. List the paths or globs under `[contracts] frozen`. Globs use `path.Match` syntax, without
   `**`.

   ```toml title=".ness/ness.toml"
   [contracts]
   frozen = ["specs/api.json", "features/*.feature", "docs/experiments/*.md"]
   prefix = ["docs/experiments/*.md"]
   ```

2. Commit it on the base branch. The rules always come from the merge-base copy of
   `.ness/ness.toml`, so a branch cannot loosen them for itself.

3. Check.

   ```sh
   ness contract check
   ```

   ```text title="Output"
   ok	3 frozen files
   ```

### Check that it worked

On a branch, change `specs/api.json`, add a scenario to `features/pay.feature`, append text to
`docs/experiments/e1.md` and create `features/refund.feature`:

```text title="Output (exit 1)"
differs	specs/api.json	.ness/ness.toml
added	features/refund.feature	.ness/ness.toml (new under frozen glob features/*.feature)
```

The added scenario and the appended text pass. Then change a step of the scenario that exists
at base, and the first line of the experiment file. Both now fail, and the check lists every
finding:

```text title="Output (exit 1)"
differs	docs/experiments/e1.md	.ness/ness.toml	locked text is no longer the start of the file (append under an Amendment heading)
differs	features/pay.feature	.ness/ness.toml	scenario "Pay an invoice" edited
differs	specs/api.json	.ness/ness.toml
added	features/refund.feature	.ness/ness.toml (new under frozen glob features/*.feature)
```

A renamed scenario counts as removed: `scenario "Pay an invoice" removed`.

## How each file is held

| File | Mode | Passes while |
|---|---|---|
| A `.feature` file | scenario | Every scenario at base still exists unchanged. Adding and reordering scenarios is allowed. |
| A `.md` file matching `prefix` | prefix | Its base text is still the start of the file |
| A file matching a `[contracts.formats]` entry | format | Every unit the format's command reads is unchanged |
| Any other file | whole-file | It equals its blob at base |

A format teaches ness to read units of your own file type. `units` is a command that reads the
file on stdin and prints one `{"id": "...", "body": "..."}` JSON line per unit:

```toml title=".ness/ness.toml"
[contracts.formats.openapi]
match = ["specs/*.yaml"]
units = "python3 scripts/openapi-units.py"
```

## Change a frozen file

| Who | Where | How |
|---|---|---|
| You, the operator | On the base branch | Commit the change directly. `check` on every branch then compares with the new text. |
| A session | On its branch | `ness contract amend PATHS... --reason TEXT --authorized WORDS`, quoting words you typed to it |
| Anyone | On a branch | Bring in the text base already holds. A file equal to a base tip passes. |

From a terminal on a branch, `amend` is refused:

```text title="Output (exit 4)"
ness: specs/api.json is frozen project-wide and this checkout is no session's; amend it on main directly
```

After you commit the same `specs/api.json` on `main`, the branch check stops reporting it.

## Freeze a file for one session

A session can freeze a file it agreed with its manager, such as its acceptance test:
`ness contract freeze PATH` records the file in the session's `contract.lock`. The operator owns
no session, so `freeze` from a terminal is refused:

```text title="Output (exit 4)"
ness: freeze needs a session: the operator freezes project-wide paths in [contracts] frozen
```

To change a file it froze, the session asks its manager or you for approval. It then runs
`ness contract amend PATHS... --reason TEXT --item ITEM`, where ITEM is the approving item
addressed to it, naming the path. Without the item, `amend` exits 4 and prints the request to
send.

The pre-tool hook refuses an edit to a whole-file contract and warns on the other modes.
`ness contract check` stays the judge, because a Bash command can still write a file.

Next: [Choose how work lands](./integration.md).
