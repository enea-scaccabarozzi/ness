---
title: Files ness writes
description: Every path ness writes, who owns it, and the rules ness sync follows so it never overwrites your work.
---

ness writes three kinds of files. Generated files in the project are committed. Local state under
`.git/ness/` never leaves the clone. Per-user files live under your home directory.

## In the project, committed

| Path | Written by | Owned how |
|---|---|---|
| `.ness/ness.toml` | `ness init` | Yours after init |
| `.ness/lock.json` | `ness sync` | ness: what it wrote, with hashes |
| `.ness/profiles/local/` | You | Yours. Any other `.ness/profiles/NAME/` loads too; only `local` applies last. |
| `.agents/skills/ness-*/` | `ness sync` | Owned. Other entries of `.agents/skills/` are left alone. |
| `.claude/skills` | `ness sync` | A symlink to `../.agents/skills`, created when absent |
| `.claude/agents/ness-{design,review,sweep,verify}.md` | `ness sync` | Owned |
| `.claude/settings.json` | `ness sync` | Only the keys `autoCompactWindow`, `env.CLAUDE_CODE_SUBAGENT_MODEL` and the `ness hook` entries of `hooks.SessionStart` and `hooks.PreToolUse` |
| `.codex/config.toml` | `ness sync`, with `client = "codex"` | Only `model_auto_compact_token_limit` and the `[agents]` subagent defaults |
| `AGENTS.md` | `ness sync` | Only its managed blocks: `core/router` and one per profile block |
| `CLAUDE.md` | `ness sync` | A symlink to `AGENTS.md` when absent. If it is a file, a managed block importing `@AGENTS.md`. |
| `.gitignore` | `ness sync` | A managed block with `.worktrees/` and `*.ness-new` |
| Profile `own` and `seed` files | `ness sync` | Owned, or created once |
| `docs/sessions/<root>/<child>/` | `ness open`, sessions | Each session's record: `brief.md`, `report.md`, `decisions.md`, `contract.lock` |

`docs/sessions` is the default of `sessions` in `ness.toml`.

Each hook command is wrapped so a clone without ness keeps working:

```json title=".claude/settings.json"
"command": "command -v ness >/dev/null || exit 0; ness hook pre-tool"
```

A managed block sits between marker lines. Text outside the markers is never changed:

```markdown title="AGENTS.md"
<!-- ness:begin local/local -->
## Acme conventions
<!-- ness:end local/local -->
```

## In the project, not committed

| Path | Holds |
|---|---|
| `.worktrees/<scope>-<id>/` | One git worktree per session, on branch `session/<scope>-<id>` |
| `PATH.ness-new` | The new render of a file in conflict |

## In the git directory

Under `.git/ness/`, shared by every worktree of the clone:

| Path | Holds |
|---|---|
| `state.sqlite3` | The store: sessions, items, probes, runs |
| `worker.log` | One line per worker action. Rotated at 10 MB, keeping 2 old files. |
| `gate.lock`, `integrate-BRANCH.lock`, `worker.lock`, `worktree.lock` | Locks for one gate, one fold per target, one worker, one worktree change at a time |
| `resume-ID.lock` | The lock for one `ness open --resume` of session ID at a time |
| `tmp-ID/`, `tmp-op-BRANCH/` | The throwaway worktree an integrate runs the gate in, for a session or for you |

## In your home directory

| Path | Holds |
|---|---|
| `~/.config/ness/secrets/<project>.toml` | Secrets from `ness secret set`, mode 0600. `XDG_CONFIG_HOME` replaces `~/.config`. |
| `~/.config/systemd/user/ness-<project>.service` | The worker unit, from `ness worker install` |

## Sync rules

`ness sync` renders core, then the features, then the listed profiles, then `local`. For each
target it compares three things: the file on disk, the hash it recorded, and the new render.

| The target is | Sync |
|---|---|
| Absent | Creates it and records it |
| Present, unrecorded, equal to the render | Adopts it: records it, bytes unchanged |
| Present, unrecorded, different | Conflict: writes `PATH.ness-new`, exit 3 |
| Recorded, unchanged by you | Replaces it with the new render |
| Changed by you, render unchanged | Leaves it. Doctor warns `sync-drift`. |
| Changed by you, render changed | Conflict: writes `PATH.ness-new`, exit 3 |
| No longer rendered, unchanged | Deletes it |
| No longer rendered, changed by you | Keeps it, reports it orphaned, forgets it |

A conflict never stops the rest of the sync. Settle it by merging `PATH.ness-new` into the file,
then:

```sh
ness sync --keep PATH
```

```text title="Output"
kept PATH
```

Later syncs leave the path alone until the render changes again. To take ness's version instead,
delete the file and run `ness sync`.

| Command | Writes | Exit |
|---|---|---|
| `ness sync` | Yes | 0, or 3 on a conflict |
| `ness sync --check` | Nothing | 0 when nothing would change, 1 otherwise, 3 on a conflict |
| `ness sync --dry-run` | Nothing | 0, or 3 on a conflict, even when files would change. Prints the plan and a diff per changed file, nothing when the plan is empty. `--json` prints the plan as JSON. |
| `ness sync --keep PATH` | The lock | 0. Refused for a path not in conflict. |

Next: [Environment and exit codes](./environment.md).
