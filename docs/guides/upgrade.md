---
title: Upgrade ness
description: Replace the binary with ness self-update, move each project with ness upgrade, and roll back when you need to.
---

An upgrade has two steps. `ness self-update` replaces the binary once per machine. `ness upgrade`
then moves each project to that binary: it migrates the store, raises the pin, syncs and commits.
A `0.N` binary still runs every project pinned to `0.N-1`, so you can upgrade the projects one at
a time (R-UPD-002). [Compatibility](../concepts/compatibility.md) explains the rules.

## Prerequisites

- ness installed with the script or the archive, as in [Install](../start/install.md). For a
  binary that mise or `go install` manages, `self-update` prints that tool's command instead.
- For `ness upgrade`: each project's base checkout, clean and on its base branch.

## Check for a new release

```sh
ness self-update --check
```

```text title="Output"
ness 0.2.0 available (installed 0.1.0): ness self-update
```

`--check` installs nothing (R-UPD-047). It writes one file, the machine's check cache
`~/.cache/ness/update.json`, which `ness doctor` and the session-start hook read:

```json title="~/.cache/ness/update.json"
{
  "checked_at": "2026-10-31T20:22:48.563756052+01:00",
  "stable": "0.2.0",
  "rc": "0.3.0-rc.1"
}
```

The channel is `--channel`, else the current project's `[update] channel`, else `stable`
(R-UPD-041). The `rc` channel also offers a newer stable release (R-UPD-040).

## Install the new binary

```sh
ness self-update
```

```text title="Output"
ness 0.2.0 installed at /home/you/.local/bin/ness (previous kept at /home/you/.local/bin/ness.prev); workers pick it up within a minute
```

`self-update` checks the archive against the release's `checksums.txt` (R-UPD-042). It writes
the new binary next to the old one, runs its `--version`, keeps the old one as `ness.prev` and
renames the new one into place (R-UPD-043). A failure at any step leaves the old binary as it
was. Each registered project's worker unit that runs another binary is re-pointed at the new one
and restarted (R-UPD-048).

To install one release, name it. A version that was never released is refused:

```sh
ness self-update --version 0.9.9
```

```text title="Output (exit 4)"
ness: no release 0.9.9
```

When you are already on the newest release, `self-update` says so and exits 0:

```text title="Output"
ness 0.2.0 is the latest on channel stable
```

### When self-update refuses

`self-update` refuses a version that a registered project cannot run (the ceiling, R-UPD-046).
Here a second project still pins `0.1`, and `0.3.0-rc.1` runs only pins `0.3` and `0.2`:

```text title="Output (exit 4)"
ness: ness 0.3.0-rc.1 cannot run /home/you/src/legacy/.ness/ness.toml (ness = "0.1"): run ness upgrade in /home/you/src/legacy; open sessions then merge their base
```

Run `ness upgrade` in that project first, then `ness self-update` again. A registered project is
one whose base checkout a ness with the update path has synced, or whose worker such a ness has
started (R-UPD-031); the machine's list is
`~/.config/ness/projects`.

A binary that another tool manages is never written (R-UPD-045). The ceiling is checked first, so
the command it prints names a version every registered project can run:

```text title="Output (exit 4)"
ness: ness at /home/you/go/bin/ness is managed by another tool; run: go install github.com/enea-scaccabarozzi/ness/cmd/ness@v0.3.0-rc.1
```

## Upgrade each project

In the project's base checkout:

```sh
ness upgrade
```

```text title="Output"
pin ness = "0.2"
ness 0.2.0 release notes
### Running sessions

* Nothing to do.

committed chore(ness): upgrade to 0.2.0
open sessions get an update item from the worker at their next idle point
```

`ness upgrade` migrates the store, sets the pin to the binary's major.minor, runs `ness sync` and
commits it all as `chore(ness): upgrade to X.Y.Z` (R-UPD-061). It prints the release notes
between the project's old version and the binary (R-UPD-063). It never pushes: push the commit
the way the project lands any other.

When the project has a worker unit, `ness upgrade` restarts it, and re-points it first when the
binary moved (R-UPD-062). The output then has one more line:

```text title="Output"
pin ness = "0.3"
committed chore(ness): upgrade to 0.3.0-rc.1
worker restarted
open sessions get an update item from the worker at their next idle point
```

A second run changes nothing:

```text title="Output"
already at 0.2.0
```

A sync conflict stops `ness upgrade` before the commit with exit 3. Resolve the conflicts and
commit, then run `ness upgrade` again. See [Files ness writes](../reference/files.md).

### When upgrade refuses

`ness upgrade` runs only at a point where nothing else is changing the base checkout (R-UPD-060).
Each refusal exits 4, commits nothing and leaves the pin as it was:

| Cause | Output |
|---|---|
| Run from a session worktree | `ness: /home/you/src/acme/.worktrees/x is a session worktree; run ness upgrade in the base checkout /home/you/src/acme` |
| HEAD on another branch | `ness: HEAD is on other, not main; check out main, then run ness upgrade` |
| A detached HEAD | `ness: HEAD is detached; check out main, then run ness upgrade` |
| An uncommitted or untracked file | `ness: uncommitted or untracked changes in /home/you/src/acme (?? notes.txt); commit or remove them, then run ness upgrade` |
| An integrate running | `ness: an integrate holds the gate lock; run ness upgrade again when it finishes` |
| A project newer than the binary | `ness: .ness/lock.json records ness 0.2.0, newer than this binary (0.1.0); run ness self-update` |

`ness upgrade` skips the pin check, so it also moves a project pinned two minors behind the
binary (R-CFG-011).

## What open sessions see

An open session keeps working on its own branch. Its worktree still holds the old ness files
until it merges its base branch.

- On its next idle point, the worker puts one update request in the session's inbox
  (R-UPD-070). It names both versions, for example
  `ness update: this worktree's ness files are 0.1.0; main has 0.2.0.`, carries the Running
  sessions notes between them and the skills that changed, and asks the session to merge its base
  branch, run `ness sync --check` (expect exit 0) and re-load the skills. The session's root gets
  one copy as an fyi (R-UPD-071).
- The session-start hook prints one line while the worktree is behind (R-UPD-072):

  ```
  ness: this worktree's ness files are 0.1.0 and main has 0.2.0: merge main, run ness sync --check (expect exit 0), re-load the changed skills (your inbox has the list)
  ```
- When a session's binary cannot run its project, its hooks block with one line that names the
  cause and the fix (R-UPD-020). Only a single plain `ness` command passes, such as
  `ness self-update` (R-UPD-021):

  ```text title="Output (exit 2)"
  ness: hooks cannot run in this session: this project pins ness 0.2.x or 0.3.x (ness = "0.2"); the running binary is 0.1.0; fix: ness self-update
  ```

## Roll back

1. In each project you upgraded with the newer binary, revert the upgrade commit, so its pin is
   one the older binary runs.

   ```sh
   git revert --no-edit HEAD
   ```

   ```text title="Output"
   [main d029620] Revert "chore(ness): upgrade to 0.3.0-rc.1"
    Date: Sat Oct 31 20:27:57 2026 +0100
    2 files changed, 3 insertions(+), 3 deletions(-)
   ```

2. Put the older binary back. `ness self-update --version` installs any release, and keeps the
   current binary as `ness.prev`:

   ```sh
   ness self-update --version 0.2.0
   ```

   ```text title="Output"
   ness 0.2.0 installed at /home/you/.local/bin/ness (previous kept at /home/you/.local/bin/ness.prev); workers pick it up within a minute
   ```

   The ceiling applies here too: while a registered project still pins `0.3`, installing `0.2.0`
   is refused, naming that project and saying to revert its upgrade commit first (step 1). Without the network, move the previous binary back by hand:

   ```sh
   mv ~/.local/bin/ness.prev ~/.local/bin/ness
   ```

   After either, the older binary refuses a project you did not revert:

   ```text title="Output (exit 4)"
   ness: this project pins ness 0.3.x or 0.4.x (ness = "0.3"); the running binary is 0.2.0 (binary: /home/you/.local/bin/ness); fix: ness self-update
   ```

3. If the newer binary migrated the store, `ness doctor` fails its `store` check and prints the
   rollback steps (R-UPD-012). Before each migration, ness copies the store to
   `state.sqlite3.bak-v<N>`, where N is the schema before it (R-UPD-010):

   ```sh
   ness doctor
   ```

   ```text title="Output (exit 1, store line)"
   fail	store	store schema 13, binary schema 12 (written by a newer ness)	fix: ness self-update; or roll back (loses what the newer ness wrote since): systemctl --user stop ness-acme; rm -f /home/you/src/acme/.git/ness/state.sqlite3-wal /home/you/src/acme/.git/ness/state.sqlite3-shm; cp /home/you/src/acme/.git/ness/state.sqlite3.bak-v12 /home/you/src/acme/.git/ness/state.sqlite3; systemctl --user start ness-acme
   ```

   Run the four commands in that order. The `rm` comes before the `cp`, because SQLite would
   replay a leftover `-wal` of the newer store onto the copy. The rollback loses what the newer ness
   wrote to the store since the migration.

## Check that it worked

```sh
ness doctor
```

```text title="Output (binary, store and update lines)"
ok	binary	0.3.0-rc.1 at /home/you/.local/bin/ness, pin 0.3
ok	store	schema 12
ok	update	ness 0.3.0-rc.1 is the latest on channel rc (checked 2026-10-31)
```

`binary` names the new version and the new pin. `update` reads `warn` with the fix
`ness self-update` while the channel offers a newer release (R-UPD-052).

Next: [Release ness](./releasing.md).
