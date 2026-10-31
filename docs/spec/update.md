# Update path: releases, install, self-update, upgrade, compatibility (UPD)

Packages: `internal/update`, `internal/config`, `internal/store`, `internal/worker`, cli
`self-update`, `upgrade`, `hook`, `doctor`; `install.sh`, `scripts/upgrade-fixture.sh`,
`.github/workflows/`. Sources: `docs/sessions/update-path/specs.md` (its "Builder check:
answers" win over earlier text), decisions UP-D1..UP-D10. Every point is settled; this file has
no open points.

## Compatibility

- **R-UPD-001** A patch release (`X.Y.Z+1`) MUST NOT add a store migration.
  Verify: testscript `upgrade_previous` — the `patchrule` command fails when the release-please
  manifest version is a patch over the newest fixture's version and `store.Version()` differs
  from the fixture's schema; unit `TestPatchRule` — manifest 0.1.1 over fixture 0.1.0 with schema
  12 → 13 errors with `adds a store migration`; the same schema, a minor bump (0.2.0) and the
  fixture's own version (0.1.0) pass.
- **R-UPD-002** A `0.N` binary MUST run every command, hook and the worker on a project whose
  pin, lock and store are from `0.N-1`.
  Verify: testscript `upgrade_previous` — the current binary against the newest fixture
  (R-UPD-094): a benign `ness hook pre-tool` exits 0 with empty stderr; `ness status`,
  `ness inbox` and `ness worker run --once` exit 0; `ness sync --check` exits 1 with no
  `conflict` in stdout.
- **R-UPD-003** Removing a config key, slot, command or flag MUST take one minor of deprecation
  first, in which the old form still works and prints its replacement.
  Verify: review.
- **R-UPD-004** A migration MUST NOT change the behaviour of an existing session. New defaults
  apply only to sessions opened after it.
  Verify: testscript `upgrade_previous` — after the migration, the fixture root's
  `ness open --scope x --tier standard --brief b.md` still exits 4 with
  `no recorded autonomy mode` in stderr, and a default guard's pattern in `ness hook pre-tool`
  still exits 2.

## Store

- **R-UPD-010** Before a migration runs, the store MUST be copied to `state.sqlite3.bak-v<N>`
  (N = the schema before the migration) next to it, replacing any older file of that name. A
  fresh store gets no copy.
  Verify: unit — opening a v11 store leaves `bak-v11` at schema 11 and the store at the current
  schema, also over a stale planted `bak-v11`; a new store, and a second open of a current one,
  leave no `bak-v*` file.
- **R-UPD-011** At most two backups MUST be kept: the one just written and the newest one below
  it.
  Verify: unit — a v11 store with planted v9, v10 and v12 keeps v10 and v11.
- **R-UPD-012** `ness doctor` MUST report a store newer than the binary as a `store` failure
  naming both schemas, with the fix `ness self-update`, and, when the backup of this binary's
  schema M exists, the rollback steps in this order, joined by `; `:
  `systemctl --user stop ness-<project>`, `rm -f <dir>/state.sqlite3-wal <dir>/state.sqlite3-shm`,
  `cp <dir>/state.sqlite3.bak-v<M> <dir>/state.sqlite3`, `systemctl --user start ness-<project>`.
  The -wal goes first because SQLite would replay a leftover -wal of the newer store onto the copy.
  Verify: testscript `doctor_store_rollback` — store schema 99: `ness doctor` exits 1 and stdout
  has `fail<TAB>store<TAB>store schema 99, binary schema M` with `fix: ness self-update`; with
  `state.sqlite3.bak-vM` present stdout also has the four steps in order, ending with
  `cp <dir>/state.sqlite3.bak-vM <dir>/state.sqlite3; systemctl --user start ness-<project>`, and
  without it no `cp` line. A current store gives `ok<TAB>store<TAB>schema M`.

## Hooks

- **R-UPD-020** Inside a ness session (`NESS_SESSION` set, or the current branch matches
  `session/*`; the session test reads one `git symbolic-ref` and no store), `ness hook pre-tool`
  MUST block (exit 2) on a pin the binary cannot run, a store newer than the binary, or a config
  that does not parse or resolve. It prints one line to stderr (R-HOOK-014),
  `ness: hooks cannot run in this session: <cause>; fix: <fix>`, the pin fix being
  `config.PinError.Fix()` (R-CFG-013).
  Verify: testscript `hook_skew` — `NESS_SESSION` set, an Edit payload: pin `0.3` under binary
  0.1.0 → exit 2, exactly one stderr line ending `fix: ness self-update`; pin `0.1` under 0.3.0
  → ends `fix: run ness upgrade in the base checkout; open sessions then merge their base`;
  store schema 99 → `store schema 99, binary schema N` and `fix: ness self-update`; an unknown
  config key → names the key and `fix: correct .ness/ness.toml`. An adopted root (no
  `NESS_SESSION`, branch `session/r-1`, store schema 99) → exit 2 with `store schema 99`.
- **R-UPD-021** While R-UPD-020 blocks, a Bash call MUST pass only when it is one simple command
  whose first word is `ness`, containing none of `;` `&` `|` `` ` `` `$` `<` `>` `(` `)` `\` or
  a newline. Every other tool call, and a payload that does not parse, stays blocked.
  Verify: unit `TestNessCommand` — `ness doctor`, `  ness self-update --check` and `ness` pass;
  `nessy x`, `cd x && ness sync`, `ness doctor & rm -rf x`, `ness doctor && rm -rf x`,
  `ness doctor; rm x`, `ness doctor | sh`, `` ness `rm x` ``, `ness $(rm x)`, `ness (x)`,
  `ness a \` + newline + `b`, `ness doctor > f`, `ness doctor < f`, and an Edit payload do not.
  Testscript `hook_skew` — under skew `ness doctor` exits 0 with empty stderr; `rm x`,
  `ness doctor && rm -rf x` and `ness doctor & rm -rf x` exit 2.
- **R-UPD-022** `ness hook session-start` in the same conditions MUST print the same line to
  stdout and exit 0. The line replaces the unbound note of R-HOOK-019, which is not printed.
  Verify: testscript `hook_skew` — for each cause of R-UPD-020, stdout starts
  `ness: hooks cannot run in this session: `, has no unbound note, and the exit is 0.
- **R-UPD-023** Outside a ness session (no `NESS_SESSION` and a branch not matching `session/*`),
  for an unknown event, and for a session the store does not know in a project without skew,
  hooks MUST keep R-HOOK-002 and R-HOOK-016 (silent, exit 0).
  Verify: testscript `hook_skew` — (1) no `NESS_SESSION` on branch `main`: each skew cause gives
  `pre-tool` and `session-start` empty stdout and stderr, exit 0; (2) `NESS_SESSION` set and pin
  skew: an unknown event gives empty stdout and stderr, exit 0; (3) a project without skew and
  `NESS_SESSION=s-zzz`, unknown to the store: `pre-tool` gives empty stdout and stderr, exit 0.

## Configuration and registry

- **R-UPD-030** `[update]` MUST accept `check` (bool, default true), `channel` (`stable` or `rc`,
  default `stable`) and `auto` (`off`, `notify` or `install`, default `notify`), and refuse other
  values, naming the key.
  Verify: unit — a `ness.toml` with no `[update]` resolves to check true, channel `stable`, auto
  `notify`; testscript `config_errors` — `channel = "beta"` exits 4 with
  `update.channel = "beta": accepted values are stable, rc`; `auto = "always"` exits 4 with
  `accepted values are off, notify, install`; `check = "yes"` exits 4 naming `update.check`.
- **R-UPD-031** `ness sync`, when it writes, MUST record the project's primary checkout path in
  `$XDG_CONFIG_HOME/ness/projects` once, and drop listed paths that no longer hold
  `.ness/ness.toml`. `ness worker run` MUST record its project the same way when it starts.
  Verify: unit (Registry) — adding a, b, a leaves the file `a\nb\n`; after a's `.ness` is
  removed, the next add of c leaves `b\nc\n`; the directory holds only `projects`. Testscript
  `sync_registry` — after `ness init` the file equals `$WORK/repo` + newline; with the file
  removed, `ness worker run --once` writes it back; a second sync leaves one line; `ness sync --check` in another repo does not list it; `$XDG_CONFIG_HOME/ness` holds
  only `projects`.

## Self-update

- **R-UPD-040** `ness self-update` MUST resolve the target from the release source: `--version`,
  else the newest release on the channel. The rc channel also offers a newer stable.
  Verify: unit (Resolve) — releases 0.1.0, 0.2.0 and 0.3.0-rc.1: stable → 0.2.0; rc →
  0.3.0-rc.1; `--version` 0.1.0 or v0.1.0 → 0.1.0; 9.9.9 → error `no release 9.9.9`; no release →
  error `no stable release`; rc with 0.3.0-rc.2 and 0.3.0 → 0.3.0; rc.10 beats rc.9. Testscript
  `self_update` — `ness self-update` installs 0.2.0 (`ness 0.2.0 installed at`), `--channel rc`
  installs 0.3.0-rc.1, and with only 0.1.0 released stdout is
  `ness 0.1.0 is the latest on channel stable`.
- **R-UPD-041** The channel MUST be `--channel`, else the current project's `[update] channel`,
  else `stable`.
  Verify: testscript `self_update` — outside a project, no flag → 0.2.0; in a project with
  `[update] channel = "rc"`, no flag → 0.3.0-rc.1, also with that project's pin at `"0.9"`;
  there, `--channel stable` → 0.2.0.
- **R-UPD-042** The archive MUST match its `checksums.txt` sha256 line. A mismatch, or a missing
  line, MUST refuse and write nothing.
  Verify: unit — the right bytes pass; other bytes → `checksum mismatch for <asset>`; no line for
  the asset → `no entry for <asset>`. Testscript `self_update` — a bad sum exits 4 with
  `checksum mismatch`, the binary is byte-identical, and its directory gains no file.
- **R-UPD-043** The install MUST write the new binary in the running binary's directory, run its
  `--version`, keep the old one as `<binary>.prev` and rename into place. Any failure leaves the
  old binary byte-identical.
  Verify: unit (Install) — success: the binary holds the new bytes with mode 0755, `.prev` holds
  the old bytes (replacing an older `.prev`), the probed file was a temp file in the same
  directory, and no temp file remains. Failures, each leaving the binary byte-identical and its
  directory holding only `ness`: the probe errors; the probe reports another version
  (`ness version 0.1.9` for 0.2.0); the directory is not writable.
- **R-UPD-044** An OS/arch pair the release does not build MUST refuse, naming the pair and
  `go install`.
  Verify: unit (Asset) — linux/amd64 present → its URL; linux/riscv64 → an error containing
  `linux/riscv64` and `go install github.com/enea-scaccabarozzi/ness/cmd/ness@v0.2.0`.
- **R-UPD-045** A binary managed by mise or `go install` MUST NOT be written. The command prints
  that tool's command and refuses.
  Verify: unit (ManagedBy) — a binary under `$MISE_DATA_DIR/installs` or, unset,
  `~/.local/share/mise/installs` → `mise use -g github:enea-scaccabarozzi/ness@0.2.0`; under
  `$GOBIN`, `$GOPATH/bin` or `~/go/bin` →
  `go install github.com/enea-scaccabarozzi/ness/cmd/ness@v0.2.0`; `~/.local/bin/ness` → not
  managed. Testscript `self_update_managed` — a binary at `$HOME/go/bin/ness` exits 4 with
  `run: go install github.com/enea-scaccabarozzi/ness/cmd/ness@v0.2.0`, the file unchanged.
- **R-UPD-046** The target MUST be refused when a registered project's base checkout or open
  session worktree pins a version it cannot run. The refusal names the `ness.toml` path and the
  pin, and the fix `run ness upgrade in <primary checkout path>; open sessions then merge their
  base`; for a target older than the pin, `revert the upgrade commit in <primary> first, or
  install a release it runs: ness self-update --version <pin>.0`.
  Verify: unit (Ceiling) — pins 0.2 (base) and 0.1 (worktree): 0.2.5 passes; 0.3.0 errors naming
  only the worktree's `ness.toml` and `ness = "0.1"): run ness upgrade in <primary>; open
  sessions then merge their base`; pin 0.3 and target 0.2.0 → the revert fix. Testscript `self_update_ceiling` — a registered project
  pinned 0.1 and release 0.3.0 → exit 4 with that message, the binary unchanged;
  `--version 0.2.0` succeeds.
- **R-UPD-047** `--check` MUST only report, writing nothing but the check cache.
  Verify: testscript `self_update` — releases 0.2.0 and 0.3.0-rc.1, installed 0.1.0:
  `ness self-update --check` prints `ness 0.2.0 available (installed 0.1.0): ness self-update`,
  the binary still reports 0.1.0, no `.prev` exists, `update.json` holds `"stable": "0.2.0"`
  and `"rc": "0.3.0-rc.1"`, and the release log has exactly one `GET /releases`;
  `--check --channel rc` prints the fix `ness self-update --channel rc`.
- **R-UPD-048** After an install, every registered project's worker unit whose `ExecStart`
  binary differs from the installed path MUST be rewritten and restarted.
  Verify: testscript `self_update` — a registered project's unit running `/old/ness`: after the
  install its `ExecStart` names the installed path and the fake `systemctl --user` calls include
  `daemon-reload` and a restart of that unit; a unit already naming the installed path is not
  rewritten.

## Update check

- **R-UPD-050** The check MUST fetch at most once per 24 h per machine and write `update.json`
  (`checked_at`, `stable`, `rc`). A failed fetch is quiet, and the next try comes 24 h later. The
  worker runs it on its pass whenever `[update] check = true`, whatever `auto` is
  (`auto = "off"` only hides the result); doctor runs it only when `check = true` and the cache is
  older than 24 h. With `check = false` neither uses the network. An empty cache directory is an
  error and nothing is written.
  Verify: unit (Cache, value clock) — not due at 23 h, due at 24 h, a zero cache due; a failing
  source keeps the cached versions and stamps `checked_at` now; `CachePath("")` errors with
  `no user cache directory` (`TestCacheEmptyDir`). Testscript `worker_auto` — a notify-mode
  worker writes `update.json` with `"stable": "0.2.0"`. Testscript `update_notice` — a second
  doctor within 24 h leaves the release log empty; with `check = false` and no cache, doctor and
  `worker run --once` leave the release log empty and write no `update.json`; with `HOME` and
  `XDG_CACHE_HOME` empty, doctor leaves `git status --porcelain` empty.
- **R-UPD-051** Hooks MUST read only the cache, never the network.
  Verify: testscript `update_notice` — a release server that fails every request,
  `hook session-start` succeeds and the server's log is empty.
- **R-UPD-052** `ness doctor` MUST show an `update` line: `warn` with `ness self-update` (for a
  managed binary, that tool's command, R-UPD-045) when the channel offers a newer version, and
  `ok` otherwise. With `check = false` or `auto = "off"` it shows no result.
  Verify: testscript `update_notice` — release 0.2.0, installed 0.1.0: stdout has
  `warn<TAB>update<TAB>ness 0.2.0 available (installed 0.1.0)<TAB>fix: ness self-update`;
  installed 0.2.0: `ok<TAB>update<TAB>`; `auto = "off"` and `check = false` each:
  `ok<TAB>update<TAB>check off` and no version offered.
- **R-UPD-053** A root's session-start MUST print
  `ness X.Y.Z available (installed A.B.C): ness self-update` when the cache offers a newer version
  and the project has `check = true` and `auto` other than `off`. A child gets no such line.
  Nothing goes to a channel or an inbox.
  Verify: testscript `update_notice` — a root's session-start prints that line; a child's, and a
  root's with `auto = "off"` or `check = false`, print no `available` line; the fake HTTP log has
  no channel request, and `store items` is empty for the operator and the root.

## Upgrade

- **R-UPD-060** `ness upgrade` MUST refuse outside the base checkout, when HEAD is not the base
  branch, with any uncommitted or untracked change, while the gate lock or the base's integrate
  lock is held, and when the project's lock is newer than the binary.
  Verify: testscript `upgrade_refusals` — each case exits 4, adds no commit, leaves the pin
  unchanged, and stderr names the cause: run from a session worktree
  (`run ness upgrade in the base checkout <path>`); HEAD on `other` (`HEAD is on other, not
  main`); a detached HEAD (`HEAD is detached`); an uncommitted file (`uncommitted`); an untracked
  file; `gate.lock` held; `integrate-main.lock` held; a lock recording 0.3.0 under binary 0.2.0
  (`newer than this binary`).
- **R-UPD-061** `ness upgrade` MUST migrate the store (R-UPD-010), set the pin to the binary's
  major.minor, run `ness sync`, and commit everything with `chore(ness): upgrade to X.Y.Z`. It
  never pushes. It skips the pin check (R-CFG-011), so it runs on a pin two minors behind. On a
  sync conflict it stops before the commit with exit 3 and says to resolve the conflicts and
  commit, then run `ness upgrade` again. With nothing to change it prints `already at X.Y.Z`,
  commits nothing and exits 0.
  Verify: testscript `upgrade` — init under 0.1.0, upgrade under 0.2.0: `ness = "0.2"` in
  `ness.toml`, `"ness": "0.2.0"` in `lock.json`, last commit subject
  `chore(ness): upgrade to 0.2.0`, `git status --porcelain` empty, no `git push` in the fake exec
  log; a second run prints `already at 0.2.0`, exits 0 and adds no commit; pin 0.1 under 0.3.0
  succeeds with pin `"0.3"`; a sync conflict exits 3, adds no commit, and stderr says
  `resolve them and commit`, then `run ness upgrade again`.
- **R-UPD-062** `ness upgrade` MUST restart the project's worker when its unit exists,
  re-pointing it first when its binary path changed.
  Verify: testscript `upgrade` — a unit already naming this binary: the fake `systemctl` calls
  include `--user restart ness-repo.service`; a unit naming `/old/ness`: its `ExecStart` is
  rewritten to this binary and stdout has `worker re-pointed to `; no unit: no `systemctl` call.
- **R-UPD-063** `ness upgrade` MUST print the release notes between the old lock version and the
  binary, and one line saying open sessions get an update item.
  Verify: testscript `upgrade` — stdout has `ness 0.2.0 release notes` with its Running-sessions
  text, and `open sessions get an update item`.

## Live sessions

- **R-UPD-070** On each pass, the worker MUST compare every open session's worktree lock version
  with the lock version on the session's base branch. When the worktree is older and the pane is
  idle, it puts one request in the session's inbox. The request carries the versions, the
  Running-sessions notes between them, the skills whose files changed, and the action: merge the
  base branch, run `ness sync --check` and expect exit 0, re-load the skills. It is sent once per
  version pair.
  Verify: unit (worker) — a busy pane gets no item; once idle, one request whose body is the
  stale body; a further pass adds none. Testscript `worker_update_item` — worktree lock 0.1.0,
  `main` 0.2.0, pane idle: `worker run --once` puts one request in the session's inbox with the
  line `ness update: this worktree's ness files are 0.1.0; main has 0.2.0.` and
  `ness sync --check (expect exit 0)`; a second run adds none; after `git merge main` in the
  worktree a third adds none.
- **R-UPD-071** The session's root MUST get one fyi copy. A root gets none for itself.
  Verify: unit — a stale child: its root gets one fyi containing the child's body, and a further
  pass adds none; a stale root: its own request only, no fyi.
- **R-UPD-072** Session-start MUST print one line when the worktree lock is behind its base
  branch's.
  Verify: testscript `worker_update_item` — in the worktree, session-start stdout has
  `ness: this worktree's ness files are 0.1.0 and main has 0.2.0`; after the merge, no such line.

## Unattended

- **R-UPD-080** With `auto = "install"`, the worker MUST self-update only when every registered
  project has `auto = "install"`, the binary is not managed, and the ceiling allows the version.
  It holds the machine install lock while doing so. Otherwise it logs why, once.
  Verify: unit (Decide) — every project `install` → install 0.2.0; one project `notify` →
  note `auto-install held: <root> has auto = "notify"`; managed → `auto-install held: the binary
  is managed by another tool`; a ceiling error → `auto-install held: <error>`. Testscript
  `worker_auto` — install: the binary reports 0.2.0 and `.prev` 0.1.0; the install lock held:
  the binary unchanged, no error line in `worker.log`; veto: the binary unchanged and `worker.log`
  has one `auto-install held: .*has auto = "notify"` line after two passes.
- **R-UPD-081** With `auto = "install"`, the worker MUST run the upgrade of its project at the
  idle point (a clean base checkout on the base branch, neither the gate lock nor the base's
  integrate lock held). Before the idle point (dirty base, a lock held) it waits silently and
  records nothing. A failure after the idle
  point (a sync conflict, a failing commit) is written to `upgrade-failed.json` with its cause,
  logged only when the cause changes, retried on each pass, and shown in doctor and in the root's
  session-start until it succeeds.
  Verify: testscript `worker_auto` — binary 0.2.0, lock 0.1.0: (1) idle → last commit
  `chore(ness): upgrade to 0.2.0`, no `upgrade-failed.json`; (2) an untracked file, then
  `gate.lock` held → no commit, no `upgrade-failed.json`, no `upgrade` line in `worker.log`;
  (3) a sync conflict, two passes → no commit, `upgrade-failed.json` holds
  `sync stopped on conflicts`, `worker.log` has exactly one line naming it, doctor shows
  `warn<TAB>update<TAB>automatic upgrade to 0.2.0 failed` and the root's session-start
  `automatic upgrade to 0.2.0 failed`; (4) resolved and committed, one pass → the upgrade commit,
  and `upgrade-failed.json` is gone. Unit — `RecordFailure` reports a change for a new cause
  only.
- **R-UPD-082** When the base lock is older than the running binary and the project does not
  upgrade itself, the worker MUST send each open root one fyi per version pair, saying the
  project is behind the binary and naming `ness upgrade`.
  Verify: testscript `worker_auto` — `auto = "notify"`, binary 0.2.0, lock 0.1.0, an open root:
  one fyi in the root's inbox containing `run ness upgrade in the base checkout`; a second pass
  adds none; with `auto = "off"` the same one fyi.

## Release

- **R-UPD-090** Exactly one workflow MUST run goreleaser for each tag: `rc.yml` for `-rc.N` tags,
  `release-please.yml` for stable ones.
  Verify: unit — `release.yml` does not exist, and the workflows containing `goreleaser release`
  are exactly `rc.yml` and `release-please.yml`.
- **R-UPD-091** The rc workflow MUST cut `vX.Y.Z-rc.N` (N = the count of existing
  `vX.Y.Z-rc.*` tags + 1) from the release PR head, after the notes test, and publish it as a
  pre-release.
  Verify: unit (release-check script) — `rc 0.2.0` with no `## [0.2.0]` CHANGELOG section exits
  1; with it, prints `v0.2.0-rc.1`; after tagging that, `v0.2.0-rc.2`; review — `rc.yml` runs
  the notes test before pushing the tag, and goreleaser publishes a pre-release.
- **R-UPD-092** The stable job MUST refuse when no `vX.Y.Z-rc.*` tag exists, when no such tag's
  tree equals HEAD's tree, or when the testdata lacks an upgrade fixture of the previous stable
  tag. The `sessions:` check counts from the last stable tag.
  Verify: unit (release-check script against a scratch repo) — `stable 0.2.0` exits 1 with:
  no rc tag → `no v0.2.0-rc.* tag: cut a release candidate`; an rc on an older tree →
  `no v0.2.0-rc.* tag has HEAD's tree: cut a new rc`; no `testdata/upgrade/v0.1.0` →
  `no upgrade fixture for v0.1.0`; only `docs(sessions):` commits since v0.1.0 →
  `no sessions: commit since v0.1.0`. An rc on HEAD's tree, the fixture and a `sessions:` commit
  since v0.1.0 → exit 0.
- **R-UPD-093** `install.sh` MUST detect OS and arch, resolve the version (latest stable,
  `NESS_VERSION` or `--channel rc`), verify the sha256, install to `$NESS_INSTALL_DIR` (default
  `~/.local/bin`), and say when that directory is not on PATH. It fails clearly on a checksum
  mismatch or an unbuilt pair.
  Verify: unit (`install_test.go` against the fake server) — releases 0.1.0, 0.2.0 and
  0.3.0-rc.1: no option installs 0.2.0; `NESS_VERSION=0.1.0` and `v0.1.0` install 0.1.0;
  `--channel rc` installs 0.3.0-rc.1; each prints `ness <v> installed to <dir>/ness` and, with
  `<dir>` off PATH, `is not on your PATH`; a bad sum exits non-zero with
  `install.sh: checksum mismatch for <asset>`; an unbuilt pair exits non-zero with
  `install.sh: no ness build for <os>/<arch>`.
- **R-UPD-094** Each stable release MUST have a fixture under
  `internal/cli/testdata/upgrade/vX.Y.Z/`, built by `scripts/upgrade-fixture.sh`. CI runs the
  current binary against the newest one.
  Verify: testscript `upgrade_previous` — `upgradefixture` loads the newest fixture's tree and
  store, and the R-UPD-002 commands pass against it; unit (release-check) — a stable release
  without the previous stable tag's fixture is refused (R-UPD-092).
- **R-UPD-095** A stable promotion MUST follow `docs/guides/releasing.md`: an rc field window
  meeting the promotion criteria there (false stalls < 10%, worker notices < 20% of phone posts,
  0 lost gate runs, 0 inbox false positives, 0 pick hangs, 0 sessions that missed the update
  item). Hooks failing open are covered by the `hook_skew` testscripts, not by the field window.
  Verify: review.
