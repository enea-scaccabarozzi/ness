# Specs — update path

Agreed with the operator in chat on 2026-10-31 (brief.md, messages 1–4: "Perfect, i like
everything, and auto shoudl stay also", "Perfect go on"). Calls marked **(root call)** were made
by the root and are listed in the report for the operator to check.

## Why
ness will keep changing from field feedback. A release must reach users (the operator's three
projects and public users) without breaking consuming projects or sessions that are open while
the update lands. Today every rollout is manual and the layers drift apart: a store migrated by
one binary locks out another; a migration silently switched off a project's program checks; a
hook that meets a pin or store it cannot run exits 0 and the guards vanish; live roots needed
hand-sent "please sync" items.

## The four layers an update changes
| Layer | Scope | Changes when |
|---|---|---|
| binary (cli, hooks, worker) | machine | install; hooks find it on PATH, worker restarts on `--version` change |
| store schema `.git/ness/state.sqlite3` | project | first touch by a newer binary |
| synced files (skills, hooks, agents, AGENTS blocks, lock) | branch | `ness sync` on that branch |
| skill text in an agent's context | session | the agent re-reads it |

The design keeps them compatible while they move at different speeds, and makes the drift short.

## Goals
1. Tagged releases are the distribution unit; installing and upgrading need no particular tool.
2. A written, tested compatibility contract between binary, store and synced files.
3. `ness self-update` and an update-available check, configured per project, including
   automatic install.
4. `ness upgrade` moves one project to the installed binary; live sessions are told and apply it
   themselves at an idle point.
5. A release cannot reach stable without passing an upgrade test against the previous release
   and a release-candidate field run on the operator's projects.
6. Docs (install, upgrade, config, cli, concepts, spec) reflect all of it; every command run by hand.

## Non-goals
- Homebrew tap or other package-manager repos (a separate public repo is the operator's call).
- Downgrade migrations. Rollback is previous binary + store backup.
- Supporting more than one previous minor. Windows/macOS self-update beyond what the release
  archives already cover (goreleaser builds linux amd64/arm64 today; the install script and
  self-update detect OS/arch and fail clearly on an unbuilt pair).
- Artifact signing (before 1.0).

## Anti-goals
- An update that silently changes behaviour of a session already open.
- A guard that silently turns off.
- Any private project name, path or record reaching the public repository.
- Update notices on the phone or the inbox-for-operator (operator reads only chat).

## Design

### 1. Distribution and install
- Release flow stays release-please → tag → goreleaser (archives + `checksums.txt`). Fix the
  double trigger: exactly one workflow runs goreleaser per tag.
- Release candidates: tags `vX.Y.Z-rc.N`, published as GitHub pre-releases. **(root call)** how
  rc tags are cut (manual `workflow_dispatch` or release-please prerelease config) is left to the
  plan; stable promotion = cutting `vX.Y.Z` from the same commit.
- `install.sh` at the repository root, served from GitHub raw:
  `curl -fsSL https://raw.githubusercontent.com/enea-scaccabarozzi/ness/main/install.sh | sh`.
  Detects OS/arch, resolves the version (latest stable, or `NESS_VERSION`, or `--channel rc`),
  downloads the archive and `checksums.txt`, verifies sha256, installs to `$NESS_INSTALL_DIR`
  (default `~/.local/bin`), and says when that directory is not on PATH. POSIX sh, needs only
  `curl` or `wget`, `tar`, and `sha256sum` or `shasum`.
- Documented alternatives: `go install github.com/enea-scaccabarozzi/ness/cmd/ness@vX.Y.Z`, mise
  (`github:` backend), and manual archive download.

### 2. Compatibility contract (new spec file `docs/spec/update.md`, prefix UPD)
- **Patch** (`X.Y.Z+1`): no store migration, no synced-text change that needs action.
- **Minor** (pre-1.0 `0.N`): may migrate the store and change synced files. The binary MUST run
  a project whose lock and store are from the previous minor (`0.N-1`) or its own: every command,
  hook and the worker work; sync updates it.
- Removing a config key, slot, command or flag takes one minor of loud deprecation first.
- A migration MUST NOT change the behaviour of an existing session; new defaults apply to
  sessions opened after it (the size backfill incident).
- The pin rule (`ness = "X.Y"`, R-CFG-010..012) is widened to match: a 0.N binary accepts pins
  0.N and 0.N-1. **(root call)** — without it, every minor is a hard break for live sessions,
  which is what goal 4 forbids.

### 3. Store safety
- Before a migration runs, copy the store to `.git/ness/state.sqlite3.bak-v<N>` (N = schema
  before). Keep the last two backups.
- `ness doctor` reports a store newer than the binary with the exact rollback: install the
  previous binary, restore the backup, or upgrade.

### 4. No silent degradation
- A hook that meets a pin mismatch, a newer store, or a broken config MUST print one line naming
  the problem and the fix, and block the tool call (PreToolUse) or put it in the context
  (SessionStart). Unknown events and unknown sessions keep failing open (they are not skew).
- The worker unit runs a stable path: the installed binary's resolved path at install time stays,
  but `ness upgrade` and `self-update` re-point and restart the unit when the path changed.

### 5. Per-project configuration
`.ness/ness.toml`:
```toml
[update]
check   = true        # daily update-available check (default true)
channel = "stable"    # "stable" | "rc"
auto    = "notify"    # "off" | "notify" | "install"
```
- `off`: no check result shown. `notify`: the check result is shown (doctor, root session-start
  text). `install`: the worker installs the new binary itself (self-update) and then runs
  `ness upgrade` for its project at an idle point. **(root call)** default `notify`.
- The binary is machine-wide, config is per project. Every project that runs `ness sync` is
  recorded in a machine registry `$XDG_CONFIG_HOME/ness/projects` (one absolute path per line,
  stale paths pruned). **(root call)**
- Ceiling rule: self-update (manual or auto) refuses a version that a registered project's pin
  cannot run, naming the project and the pin. Channel only decides what the check offers; a
  stable-channel project can run under an rc binary. **(root call)**
- Which project's channel applies to a manual `ness self-update`: the current project's, or
  `--channel`; outside a project, stable.

### 6. Self-update and the check
- `ness self-update [--version X.Y.Z] [--channel stable|rc] [--check]`: resolve target from the
  GitHub releases API, download archive + checksums, verify, write next to the running binary,
  keep the old one as `ness.prev`, atomic rename, run the new `--version` to prove it.
  `--check` only reports.
- When the binary is managed by another tool (path under a mise install dir or `$GOPATH/bin` /
  `$GOBIN`), self-update does not write; it prints that tool's command. `auto = "install"` then
  degrades to notify, said once.
- The check: at most once per 24 h per machine, cached in `$XDG_CACHE_HOME/ness/update.json`
  (`checked_at`, latest stable, latest rc). Hooks only read the cache, never the network. The
  network fetch happens in the worker loop or `ness doctor`, with a short timeout; failure is
  quiet and retried next day.
- Shown in: `ness doctor` (a check line), and the session-start text of a root (one line:
  "ness X.Y.Z available (installed A.B.C): ness self-update"). Never phone, never inbox.

### 7. `ness upgrade` and live sessions
- `ness upgrade` in a project (base checkout): back up + migrate the store, `ness sync` on the
  base branch, commit the synced files with message `chore(ness): upgrade to X.Y.Z`, restart the
  project's worker, print the Running-sessions notes. Refuses with a dirty base checkout or while
  an integrate holds the gate lock. Never pushes.
- The worker, each pass, compares each open session's worktree lock version with the base
  branch's lock version. When the worktree is older, at the session's next idle point it
  delivers once one item to that session with:
  - the Running-sessions notes between the two versions;
  - the skills whose files changed (to re-load);
  - the action: `ness sync` in the worktree, commit, re-load those skills.
  The session's root gets an fyi copy. The session applies it itself; nothing waits on the operator.
- Running-sessions text is the CHANGELOG section release CI already requires per version.
- Session-start text (resume/compact) also says when the worktree lock is behind base, so a
  session that missed the item still learns it.

### 8. Stability gates
- **Upgrade test in CI**: each release commits a fixture under `internal/…/testdata/upgrade/vX.Y.Z/`
  (a synced project tree + a store dump with one open session, built by a script). CI runs the
  current binary against the previous release's fixture and asserts: store migrates (backup
  written); a hook call for the old-text session passes; `sync --check` lists only expected
  changes; a program check still fires. **(root call)** the first fixture is cut from v0.1.0 as
  released state of main before this work.
- **rc field run**: rc goes to the operator's three projects (`channel = "rc"`) and runs a field
  window before stable. Promotion criteria are the field-feedback-3 targets plus: 0 hooks failing
  open, 0 sessions that missed the update item.

## Live testing and privacy
- Authorized by the operator: real installs and upgrades on the three projects on this machine.
- Any record naming them lives in `docs/discovery/` (excluded). Committed records use the
  anonymized names (research lab, infrastructure lab, production backend).
- The denylist leak scan runs on the tree before every push.
- A build with a new migration is first run in a scratch `git init` repo, never in a live store
  (test-binary-store rule). Live store backups are taken before the first rc touches them.
- Rollout to the projects follows the batched-rollout ruling: one batch, after the operator's go,
  with a recap first.

## Test strategy
- Each UPD/changed rule gets a `Verify:` line in `docs/spec/update.md` before code.
- Acceptance tests are written by a separate task from the implementer (reviewer-authored
  testscripts per rule group), black-box (`package x_test`), hand-written fakes.
- Network: a local `httptest` server stands in for the GitHub releases API and asset downloads;
  the base URL is an injected value (env `NESS_RELEASES_URL` for testscripts), not a test hook.
- `install.sh`: run under testscript or a shell test against the same fake server, with
  checksum-mismatch, unknown-arch, not-on-PATH cases.
- Time (daily check, idle point) passed in as `now`; timers under `testing/synctest`.
- Each kept test answers which wrong implementation it rejects.

## Interfaces (sketch, the plan fixes the details)
- `internal/update` (new): role "find, verify and install ness releases". Consumer-declared
  ports: a release source (list releases, fetch asset) — the http implementation built in `cmd/`.
- Registry and cache are files under injected dirs; no package-level state.
- The worker gets the release source and the registry injected from `cmd/`.

## Docs (last)
`docs/start/install.md`, `docs/guides/upgrade.md`, config reference (`[update]`), cli reference
(`self-update`, `upgrade`), a concepts page on compatibility and live sessions, `docs/spec/update.md`,
README install line, SECURITY.md supported versions, backlog entry closed. Every command run by
hand against a real release.

## Builder check (2026-10-31): answers
A fresh builder read this document; its questions and the settlements:
1. **Pin moves with `ness upgrade`.** `ness upgrade` sets `ness = "0.N"` to the installed binary's
   minor in `.ness/ness.toml` and commits it with the synced files. **(root call)**
2. **Session action is "merge the base branch", not "sync in the worktree".** The base branch
   carries the synced files and the pin together; the update item says: merge base into your
   branch, `ness sync --check` (expect exit 0), re-load the listed skills. The ceiling rule reads
   the pins of the base checkout and of every open session worktree. **(root call)**
3. **Loud hooks only in ness sessions, with an escape.** The PreToolUse block applies only when
   the hook runs in a known ness session (`NESS_SESSION` set); elsewhere R-HOOK-002's silent exit
   stands. While blocked, Bash commands whose first word is `ness` are allowed, so the agent can
   run the fix it is told. **(root call)**
4. **rc and release-please.** When release-please opens its release PR, a manual workflow cuts
   `vX.Y.Z-rc.N` from that PR's head (CHANGELOG section included). Merging the release PR is the
   stable promotion; "same tree" is the rule, not same commit. The stable publish job refuses when
   no `vX.Y.Z-rc.*` tag exists. The field-run promotion criteria are a maintainer checklist
   (`docs/guides/releasing.md`), measured from existing store and hook records; no new stats
   counters this release. **(root call)**
5. **Unattended upgrade.** Idle point = base checkout clean and no gate lock held. The worker
   retries each pass; a failure is logged once per cause and shown in the root's session-start
   text and in `ness doctor` until it succeeds. Several workers coordinate the binary install
   through one lock file in `$XDG_CACHE_HOME/ness/`. **(root call)**
6. **Mixed `auto` on one machine: veto.** Unattended install runs only when every registered
   project has `auto = "install"`; otherwise it degrades to notify. Manual `ness self-update`
   still works; each other project's worker then tells its root the project is behind the
   binary. (operator, UP-D6)
