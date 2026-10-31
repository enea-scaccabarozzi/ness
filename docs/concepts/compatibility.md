---
title: Compatibility
description: The four layers a ness version touches, the N/N-1 rule between them, and how a machine and its projects move to a new release.
---

Compatibility is the rule that lets you replace the ness binary before you move every project to
it: a `0.N` binary runs every command, hook and the worker on a project whose pin, lock and store
are from `0.N-1` (R-UPD-002). You upgrade the machine once, then each project when it is ready.

## The four layers

A ness version lives in four places. Each moves with a different command.

| Layer | Where | Moved by | Scope |
|---|---|---|---|
| Binary | `~/.local/bin/ness` | `ness self-update`, or the [install script](../start/install.md) | The machine |
| Pin | `ness = "0.N"` in `.ness/ness.toml` | `ness upgrade` (R-CFG-010) | One project, committed |
| Lock | `"ness": "0.N.Z"` in `.ness/lock.json`, with the files `ness sync` renders | `ness upgrade`, `ness sync` | One project, committed |
| Store | The schema of `<git-common-dir>/ness/state.sqlite3` | The first command of a newer binary | One project, not committed |

The binary is shared by every project on the machine. The other three belong to one project, and
a session worktree carries its own pin and lock until it merges its base branch.

## N and N-1

| Pin | Binaries that run it |
|---|---|
| `0.N` | `0.N.x` and `0.(N+1).x` (R-CFG-011) |
| `M.N`, with M ≥ 1 | Major M, minor N or higher (R-CFG-012) |

A binary outside that range refuses every command with exit 4, naming the range, the binary and
the fix (R-CFG-013). `init`, `--version`, `--help`, `doctor`, `self-update` and `upgrade` skip the
check, so you can always diagnose and repair:

```text title="Output (exit 4)"
ness: this project pins ness 0.2.x or 0.3.x (ness = "0.2"); the running binary is 0.1.0 (binary: /home/you/.local/bin/ness.prev); fix: ness self-update
```

A binary older than the pin gets the fix `ness self-update`. A pin more than one minor behind
the binary gets `run ness upgrade in the base checkout; open sessions then merge their base`.

Releases keep the rule possible:

- A patch release adds no store migration (R-UPD-001).
- Removing a config key, slot, command or flag takes one minor of deprecation, in which the old
  form still works and prints its replacement (R-UPD-003).
- A migration does not change how an existing session behaves. New defaults apply to sessions
  opened after it (R-UPD-004).
- Each stable release has an upgrade fixture, and CI runs the current binary against the newest
  one (R-UPD-094).

## The store backup

The store is the one layer not in git. Before a newer binary migrates it, ness copies it to
`state.sqlite3.bak-v<N>` next to it, where N is the schema before the migration (R-UPD-010). It
keeps two backups: the newest one and the one below it (R-UPD-011). A new store gets
none.

An older binary does not open a newer store. `ness doctor` fails its `store` check, naming both
schemas, and prints the rollback steps when the backup of the binary's schema exists
(R-UPD-012). [Roll back](../guides/upgrade.md#roll-back) shows them.

## Loud hooks

A hook that cannot load its project must not let a session work unguarded. Inside a ness session
(`NESS_SESSION` set, or the branch matches `session/*`), `ness hook pre-tool` blocks with exit 2
when the binary cannot run the pin, the store is newer than the binary, or `ness.toml` does not
parse (R-UPD-020):

```text title="Output (exit 2)"
ness: hooks cannot run in this session: this project pins ness 0.2.x or 0.3.x (ness = "0.2"); the running binary is 0.1.0; fix: ness self-update
```

While it blocks, only one simple Bash command passes: a command whose first word is `ness` and
that contains none of `;` `&` `|` `` ` `` `$` `<` `>` `(` `)` `\` or a newline (R-UPD-021). The
agent can run `ness doctor` or `ness self-update`, and nothing else. `ness hook session-start`
prints the same line and exits 0 (R-UPD-022). Outside a session the hooks stay silent
(R-UPD-023).

## The update item

An open session does not see a new version until it merges its base branch. The worker tells it:

- On each pass, the worker compares each open session's worktree lock with the lock on its base
  branch. When the worktree is older and the pane is idle, the session gets one request with
  both versions, the Running sessions notes between them, the skills that changed, and the
  action: merge the base branch, run `ness sync --check` and expect exit 0, re-load the skills.
  It is sent once per version pair (R-UPD-070).
- The session's root gets one fyi copy (R-UPD-071).
- The session-start hook prints one line while the worktree is behind (R-UPD-072).
- When the binary is newer than the base checkout's lock and the project does not upgrade
  itself, each open root gets one fyi per version pair that names `ness upgrade` (R-UPD-082).

## The update check

The worker checks the release source at most once per 24 hours per machine and writes
`~/.cache/ness/update.json` (R-UPD-050). Hooks read only that file, never the network
(R-UPD-051). `ness doctor` shows the result on its `update` line (R-UPD-052), and a root's
session-start prints `ness X.Y.Z available (installed A.B.C): ness self-update` (R-UPD-053).
Nothing goes to your phone or an inbox. [`[update]`](../reference/config.md#update) sets the
channel and turns the check off.

## The ceiling

`ness self-update` refuses a version that a registered project cannot run: the pin of its base
checkout or of an open session worktree is outside the target's range (R-UPD-046). The refusal
names the `ness.toml` and its pin, and the fix: run `ness upgrade` in that project, then let open
sessions merge their base. For a target older than the pin (a rollback), the fix is to revert
that project's upgrade commit first, or to install a release the pin runs. A project is registered once a binary with the update path has synced
it or started its worker (R-UPD-031).

Only `ness self-update` applies the ceiling. The install script, mise and `go install` install
what you ask for. A project that the new binary cannot run then shows it in `ness doctor`, and
the hooks of its sessions block with the fix.

## The veto

`[update] auto = "install"` lets the worker install new releases and upgrade its project with no
one at the terminal. It installs only when every registered project has `auto = "install"`, the
binary is not managed by mise or `go install`, and the ceiling allows the version (R-UPD-080). One
project left on `notify` vetoes it for the whole machine, because the binary is shared. The
worker logs why, once, in `worker.log`: `auto-install held: <project> has auto = "notify"`. A
failed install is logged once, retried after 24 hours or for a newer release, and shown in
`ness doctor`. A held or failed install never holds the project's own upgrade below.

The worker then runs `ness upgrade` at the idle point: a clean base checkout on the base branch,
with no gate or integrate lock held. Before that point it waits silently. A failure after it,
such as a sync conflict, is recorded, logged once per cause, retried on each pass, and shown in
`ness doctor` and the root's session-start until it succeeds (R-UPD-081).

Next: [Upgrade ness](../guides/upgrade.md).
