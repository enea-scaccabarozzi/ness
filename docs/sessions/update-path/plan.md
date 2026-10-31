# Update path Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans or superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax.

**Goal:** Tagged releases reach the operator's projects and public users through `install.sh`,
`ness self-update` and `ness upgrade`, under a written and tested compatibility contract
(binary N runs files and store of minor N-1). A guard never turns off silently. Live sessions
learn about an update from one worker item and apply it themselves.

**Architecture:** A new package `internal/update` holds the pure release logic: resolve,
verify, extract, atomic install, ceiling, cache, registry, failure record and the unattended
decision. Its port `Source` (list releases, fetch an asset) is declared in `internal/update`
and implemented by a new atom `internal/atom/releases` (GitHub releases API over HTTP). Both are
built in `cmd/ness` and in the testscript harness, with the base URL from `NESS_RELEASES_URL`.
`internal/cli` gains `self-update` and `upgrade` and wires the worker's two new hooks. The
first, `Worker.Stale`, sends the per-session update item. The second, `Worker.Update`, runs the
daily check and the unattended install and upgrade on each pass. The store writes a backup
before any migration. Hooks block loudly on skew, and only inside ness sessions. The release
workflows cut an rc from release-please's PR head, and the stable publish refuses without an rc.

**Tech Stack:** Go (go.mod 1.26, mise go 1.27.3), cobra, modernc.org/sqlite, BurntSushi/toml,
testscript (rogpeppe/go-internal), `testing/synctest`, `net/http/httptest`, POSIX sh,
GitHub Actions, release-please v4, goreleaser 2.18.2.

**Spec:** `docs/sessions/update-path/specs.md`, together with its "Builder check: answers"
section, which wins wherever it differs from the earlier text. Also `decisions.md` (UP-D1..D7).

## Global Constraints

- Pin: a `0.N` binary runs pins `0.N` and `0.N-1`. A pin with major ≥ 1 keeps R-CFG-012 (same major, minor ≥ pin).
- `ness upgrade` sets `ness = "<binary major.minor>"` and commits it with the synced files.
- Upgrade commit message: `chore(ness): upgrade to X.Y.Z`, where X.Y.Z is the binary's release version. `ness upgrade` never pushes.
- Store backup: `<git-common-dir>/ness/state.sqlite3.bak-v<N>`, where N is the schema before the migration. Keep the just-written backup and the highest one below it. A fresh store (schema 0) gets no backup.
- Config: `[update]` with `check = true`, `channel = "stable"` (`stable` or `rc`), `auto = "notify"` (`off`, `notify` or `install`).
- Machine registry: `$XDG_CONFIG_HOME/ness/projects`, one absolute primary-checkout path per line. Stale paths (no `.ness/ness.toml`) are pruned. No lock file: writes go through a temp file + rename in the same directory; an append lost to a concurrent sync is healed by the next sync.
- Check cache: `$XDG_CACHE_HOME/ness/update.json`, with keys `checked_at`, `stable` and `rc`. At most one fetch per 24 h per machine. The fetch timeout is 5 s. Hooks never touch the network. The worker fetches whenever `[update] check = true`, whatever `auto` is; `auto = "off"` only hides the result. An empty cache dir (`Deps.CacheDir == ""`) is an error, never a write into the repository.
- Install lock: `$XDG_CACHE_HOME/ness/install.lock` (flock, non-blocking), taken with the existing flock helper (`worker.LockPath`, generalised from `worker.Lock` in Task 10), never a new copy.
- Unattended-upgrade failure record: `<git-common-dir>/ness/upgrade-failed.json`, written only for a failure after the idle point (a sync conflict, a failing commit), holding the cause; the worker logs only when the cause changes; removed on success. A dirty base or a held gate lock is "not idle": skipped silently, no record.
- Root session-start line, verbatim: `ness X.Y.Z available (installed A.B.C): ness self-update`. It never goes to the phone or an inbox.
- Release source: `GET {base}/releases?per_page=100`, where `base` is `NESS_RELEASES_URL`, default `https://api.github.com/repos/enea-scaccabarozzi/ness`. Assets are `ness_<version>_<os>_<arch>.tar.gz` and `checksums.txt` (sha256).
- `self-update` keeps the old binary as `<binary>.prev`. The new file is written in the same directory, its `--version` is run, and then it is renamed into place.
- Managed binaries: a binary under `$MISE_DATA_DIR/installs` (default `~/.local/share/mise/installs`), `$GOBIN`, `$GOPATH/bin` or `~/go/bin` is never written. ness prints `mise use -g github:enea-scaccabarozzi/ness@X.Y.Z` or `go install github.com/enea-scaccabarozzi/ness/cmd/ness@vX.Y.Z` instead.
- Loud hooks block only inside a ness session: `NESS_SESSION` is set or the current branch matches `session/*` (settled point 2). While blocked, only a single simple Bash command passes: its first word is `ness` and it contains none of `;` `&` `|` `` ` `` `$` `<` `>` `(` `)` `\` or a newline.
- Unattended install happens only when every registered project has `auto = "install"` (UP-D6 veto). The idle point for an unattended upgrade is a clean base checkout on the base branch with no gate lock held.
- Pin-check exemptions: `init`, `--version`, `--help`, `doctor`, `self-update` and `upgrade` skip the pin check (amended R-CFG-011); `upgrade` is the pin fixer and runs even two minors behind. The pin fix text is defined once, in `config.PinError.Fix()`.
- rc tags are `vX.Y.Z-rc.N`, cut by a manual workflow from release-please's PR head and published as GitHub pre-releases. The stable job refuses when no `vX.Y.Z-rc.*` tag exists, or when no such tag's tree equals HEAD's tree. The first rc is `v0.2.0-rc.1`; the baseline is the public release `v0.1.0` (tag on a7903ca).
- Commit types: session-record commits use `docs(sessions): <scope> …`; `sessions:` is reserved for Running-sessions notes.
- `install.sh` is POSIX sh and needs only `curl` or `wget`, `tar`, and `sha256sum` or `shasum`. It installs to `$NESS_INSTALL_DIR` (default `~/.local/bin`) and also honours `NESS_VERSION`, `--channel rc` and `NESS_RELEASES_URL`.
- Tests: black-box (`package x_test`), hand-written fakes, no `time.Sleep`, fixtures in `testdata/`, test files under 400 lines, `now` passed as a value, timers under `testing/synctest`.
- Network in tests goes to an `httptest` server whose URL is injected. Testscripts get it through `NESS_RELEASES_URL`. No test hook goes into production code.
- No branch-built ness runs against a live store or the real `~/.config/ness`, `~/.cache/ness` or `~/.local/bin/ness`. Manual runs use a scratch `git init` repo under the scratchpad, with `HOME`, `XDG_CONFIG_HOME` and `XDG_CACHE_HOME` pointed into the scratchpad and `NESS_RELEASES_URL` pointed at a local server.
- Spec coverage (R-QA-010) is red from Task 0 (UPD is in `doneAreas`) until the last covering test (Task 15) and the R-UPD-003/095 review entries (Task 17) land. Do not integrate before Task 17's gates; nothing is pushed to origin without the operator's go.

## Review Focus

These are failure modes the spec implies but states no test for. Each test is added to the task
that owns the code.

1. **Concurrent first openers of an old store.** After a self-update, two hooks and the worker
   open the store at the same moment. The `VACUUM INTO` backup must not fail the second opener
   ("output file already exists"), and the migration must still run exactly once. Test
   `TestBackupConcurrentOpeners` in Task 2.
2. **rc ordering.** `nsync` compares prerelease tags as plain strings, so `rc.10 < rc.2`, and
   Resolve would offer an older rc. Tests `TestCompareNumericPrerelease` in Task 1 and
   `TestResolveRCTen` in Task 5.
3. **Partial installs.** The temporary file must sit in the binary's own directory, because a
   rename across `/tmp` (tmpfs) fails. The installed file needs mode 0755. A probe that fails or
   reports the wrong version must leave the old bytes in place and no temporary file behind.
   A checksums file without the asset's line is a refusal, never "verified". Tests
   `TestInstallSameDirMode`, `TestInstallProbeFailsKeepsOld` and `TestVerifyMissingEntry` in
   Task 5.
4. **Update item idempotency.** A worker restart must not send the update item twice. A base
   that moves again (0.2 to 0.3) before the session merged sends exactly one new item, for the
   new pair. A worktree with no lock sends nothing, and its error is logged once. Test
   `TestUpdateItemOncePerPair` in Task 11.
5. **An unattended upgrade on a base checkout in a transient state.** A detached HEAD (bisect,
   or a rebase in progress) or another branch checked out can look "clean" to
   `git status --porcelain`. The upgrade must refuse there and never commit on it. Testscript
   cases in `upgrade_refusals.txtar` (Task 10).

---

### Task 0: Spec — `docs/spec/update.md` (UPD) and amended rules

**Files:**
- Create: `docs/spec/update.md`
- Modify: `docs/spec/README.md` (index row `| [update.md](update.md) | UPD | update-path | releases, install, self-update, upgrade, compatibility |`), `docs/spec/core-config.md`, `docs/spec/hooks.md`, `docs/spec/sync.md`, `docs/spec/worker.md`, `docs/spec/doctor.md`, `docs/spec/store-messages.md`, `docs/spec/quality.md`, `docs/spec/reviews.md` (Step 3)

**Interfaces:** none (text only).

- [ ] **Step 1: Write `docs/spec/update.md`.** Use the README requirement format, with one
  `Verify:` line for each rule. Give it the same archive-free header as the newer spec files. It has no
  "Open points" section: every point is settled (see "Settled points" at the end). The rules, in order:

  Compatibility
  - **R-UPD-001** A patch release (`X.Y.Z+1`) MUST NOT add a store migration. Verify: testscript `upgrade_previous` — the `patchrule` command fails when the release-please manifest version is a patch over the newest fixture's version and `store.Version()` differs from the fixture's schema; unit `TestPatchRule`.
  - **R-UPD-002** A `0.N` binary MUST run every command, hook and the worker on a project whose pin, lock and store are from `0.N-1`. Verify: testscript `upgrade_previous` — binary 0.2.0 against the v0.1.0 fixture.
  - **R-UPD-003** Removing a config key, slot, command or flag MUST take one minor of deprecation first, in which the old form still works and prints its replacement. Verify: review.
  - **R-UPD-004** A migration MUST NOT change the behaviour of an existing session. New defaults apply only to sessions opened after it. Verify: testscript `upgrade_previous` — the fixture root's program check still refuses.

  Store
  - **R-UPD-010** Before a migration runs, the store MUST be copied to `state.sqlite3.bak-v<N>` (N = the schema before the migration) next to it, replacing any older file of that name. A fresh store gets no copy. Verify: unit — opening a v11 store leaves `bak-v11` at schema 11, also over a stale planted `bak-v11`; a new store leaves none.
  - **R-UPD-011** At most two backups MUST be kept: the one just written and the newest one below it. Verify: unit — a v11 store with planted v9, v10 and v12 keeps v10 and v11.
  - **R-UPD-012** `ness doctor` MUST report a store newer than the binary as a `store` failure naming both schemas, with the fix `ness self-update`, and the rollback `cp <dir>/state.sqlite3.bak-v<M> <dir>/state.sqlite3` when the backup of this binary's schema M exists. Verify: testscript `doctor_store_rollback`.

  Hooks
  - **R-UPD-020** Inside a ness session (`NESS_SESSION` set, or the current branch matches `session/*`, read with one `git symbolic-ref`, no store read), `ness hook pre-tool` MUST block (exit 2) on a pin the binary cannot run, a store newer than the binary, or a config that does not parse or resolve. It prints one line naming the problem and the fix. Verify: testscript `hook_skew`, including an adopted root (no `NESS_SESSION`, a `session/*` branch, a newer store).
  - **R-UPD-021** While R-UPD-020 blocks, a Bash call MUST pass only when it is one simple command whose first word is `ness`, containing none of `;` `&` `|` `` ` `` `$` `<` `>` `(` `)` `\` or a newline. Verify: unit `TestNessCommand` and testscript `hook_skew` — `ness doctor` passes; `rm x` and `ness doctor && rm -rf x` are blocked.
  - **R-UPD-022** `ness hook session-start` in the same conditions MUST print the same line to stdout and exit 0. Verify: testscript `hook_skew`.
  - **R-UPD-023** Outside a ness session (no `NESS_SESSION` and a branch not matching `session/*`), for an unknown event, and for a session the store does not know in a project without skew, hooks MUST keep R-HOOK-002 and R-HOOK-016 (silent, exit 0). Verify: testscript `hook_skew`.

  Configuration and registry
  - **R-UPD-030** `[update]` MUST accept `check` (bool, default true), `channel` (`stable` or `rc`, default `stable`) and `auto` (`off`, `notify` or `install`, default `notify`), and refuse other values, naming the key. Verify: unit and testscript `config_errors`.
  - **R-UPD-031** `ness sync`, when it writes, MUST record the project's primary checkout path in `$XDG_CONFIG_HOME/ness/projects` once, and drop listed paths that no longer hold `.ness/ness.toml`. Verify: unit (Registry) and testscript `sync_registry`.

  Self-update
  - **R-UPD-040** `ness self-update` MUST resolve the target from the release source: `--version`, else the newest release on the channel. The rc channel also offers a newer stable. Verify: unit (Resolve) and testscript `self_update`.
  - **R-UPD-041** The channel MUST be `--channel`, else the current project's `[update] channel`, else `stable`. Verify: testscript `self_update`.
  - **R-UPD-042** The archive MUST match its `checksums.txt` sha256 line. A mismatch, or a missing line, MUST refuse and write nothing. Verify: unit and testscript `self_update`.
  - **R-UPD-043** The install MUST write the new binary in the running binary's directory, run its `--version`, keep the old one as `<binary>.prev` and rename into place. Any failure leaves the old binary byte-identical. Verify: unit (Install).
  - **R-UPD-044** An OS/arch pair the release does not build MUST refuse, naming the pair and `go install`. Verify: unit (Asset).
  - **R-UPD-045** A binary managed by mise or `go install` MUST NOT be written. The command prints that tool's command and refuses. Verify: unit (ManagedBy) and testscript `self_update_managed`.
  - **R-UPD-046** The target MUST be refused when a registered project's base checkout or open session worktree pins a version it cannot run. The refusal names the `ness.toml` path and the pin, and the fix `run ness upgrade in <primary checkout path>; open sessions then merge their base`. Verify: unit (Ceiling) and testscript `self_update_ceiling`.
  - **R-UPD-047** `--check` MUST only report, writing nothing but the check cache. Verify: testscript `self_update`.
  - **R-UPD-048** After an install, every registered project's worker unit whose `ExecStart` binary differs from the installed path MUST be rewritten and restarted. Verify: testscript `self_update`.

  Update check
  - **R-UPD-050** The check MUST fetch at most once per 24 h per machine and write `update.json` (`checked_at`, `stable`, `rc`). A failed fetch is quiet, and the next try comes 24 h later. The worker runs it on its pass whenever `[update] check = true`, whatever `auto` is (`auto = "off"` only hides the result); doctor runs it when due. An empty cache directory is an error and nothing is written. Verify: unit (Cache, value clock; `TestCacheEmptyDir`) and testscript `worker_auto` (a notify-mode worker writes the cache).
  - **R-UPD-051** Hooks MUST read only the cache, never the network. Verify: testscript `update_notice` — a release server that fails every request, `hook session-start` succeeds and the server's log is empty.
  - **R-UPD-052** `ness doctor` MUST show an `update` line: `warn` with `ness self-update` when the channel offers a newer version, and `ok` otherwise. With `check = false` or `auto = "off"` it shows no result. Verify: testscript `update_notice`.
  - **R-UPD-053** A root's session-start MUST print `ness X.Y.Z available (installed A.B.C): ness self-update` when the cache offers a newer version and the project has `check = true` and `auto` other than `off`. A child gets no such line. Nothing goes to a channel or an inbox. Verify: testscript `update_notice` — the fake HTTP log has no channel request, and `store items` is empty.

  Upgrade
  - **R-UPD-060** `ness upgrade` MUST refuse outside the base checkout, when HEAD is not the base branch, with any uncommitted or untracked change, while a gate or integrate lock of the base is held, and when the project's lock is newer than the binary. Verify: testscript `upgrade_refusals`.
  - **R-UPD-061** `ness upgrade` MUST migrate the store (R-UPD-010), set the pin to the binary's major.minor, run `ness sync`, and commit everything with `chore(ness): upgrade to X.Y.Z`. It never pushes. It skips the pin check (R-CFG-011), so it runs on a pin two minors behind. On a sync conflict it stops before the commit with exit 3 and says to resolve the conflicts and commit, then run `ness upgrade` again. Verify: testscript `upgrade`.
  - **R-UPD-062** `ness upgrade` MUST restart the project's worker when its unit exists, re-pointing it first when its binary path changed. Verify: testscript `upgrade` — fake `systemctl` calls.
  - **R-UPD-063** `ness upgrade` MUST print the release notes between the old lock version and the binary, and one line saying open sessions get an update item. Verify: testscript `upgrade`.

  Live sessions
  - **R-UPD-070** On each pass, the worker MUST compare every open session's worktree lock version with the lock version on the session's base branch. When the worktree is older and the pane is idle, it puts one request in the session's inbox. The request carries the versions, the Running-sessions notes between them, the skills whose files changed, and the action: merge the base branch, run `ness sync --check` and expect exit 0, re-load the skills. It is sent once per version pair. Verify: unit (worker) and testscript `worker_update_item`.
  - **R-UPD-071** The session's root MUST get one fyi copy. A root gets none for itself. Verify: unit.
  - **R-UPD-072** Session-start MUST print one line when the worktree lock is behind its base branch's. Verify: testscript `worker_update_item`.

  Unattended
  - **R-UPD-080** With `auto = "install"`, the worker MUST self-update only when every registered project has `auto = "install"`, the binary is not managed, and the ceiling allows the version. It holds the machine install lock while doing so. Otherwise it logs why, once. Verify: unit (Decide) and testscript `worker_auto`.
  - **R-UPD-081** With `auto = "install"`, the worker MUST run the upgrade of its project at the idle point (a clean base checkout on the base branch, no gate lock). Before the idle point (dirty base, gate lock held) it waits silently and records nothing. A failure after the idle point (a sync conflict, a failing commit) is written to `upgrade-failed.json` with its cause, logged only when the cause changes, retried on each pass, and shown in doctor and in the root's session-start until it succeeds. Verify: testscript `worker_auto`.
  - **R-UPD-082** When the base lock is older than the running binary and the project does not upgrade itself, the worker MUST send each open root one fyi per version pair, saying the project is behind the binary and naming `ness upgrade`. Verify: testscript `worker_auto`.

  Release
  - **R-UPD-090** Exactly one workflow MUST run goreleaser for each tag: `rc.yml` for `-rc.N` tags, `release-please.yml` for stable ones. Verify: unit — the workflow text check.
  - **R-UPD-091** The rc workflow MUST cut `vX.Y.Z-rc.N` (N = existing rc count + 1) from the release PR head, after the notes test, and publish it as a pre-release. Verify: unit (release-check script) and review.
  - **R-UPD-092** The stable job MUST refuse when no `vX.Y.Z-rc.*` tag exists, when no such tag's tree equals HEAD's tree, or when the testdata lacks an upgrade fixture of the previous stable tag. The `sessions:` check counts from the last stable tag. Verify: unit (release-check script against a scratch repo).
  - **R-UPD-093** `install.sh` MUST detect OS and arch, resolve the version (latest stable, `NESS_VERSION` or `--channel rc`), verify the sha256, install to `$NESS_INSTALL_DIR` (default `~/.local/bin`), and say when that directory is not on PATH. It fails clearly on a checksum mismatch or an unbuilt pair. Verify: unit (`install_test.go` against the fake server).
  - **R-UPD-094** Each stable release MUST have a fixture under `internal/cli/testdata/upgrade/vX.Y.Z/`, built by `scripts/upgrade-fixture.sh`. CI runs the current binary against the newest one. Verify: testscript `upgrade_previous`.
  - **R-UPD-095** A stable promotion MUST follow `docs/guides/releasing.md`: an rc field window meeting the promotion criteria there (false stalls < 10%, worker notices < 20% of phone posts, 0 lost gate runs, 0 inbox false positives, 0 pick hangs, 0 sessions that missed the update item). Hooks failing open are covered by the `hook_skew` testscripts, not by the field window. Verify: review.

- [ ] **Step 2: Amend the rules in the other spec files** (keep each id, rewrite the text, update its Verify line):
  - **R-CFG-010**: "…written by `ness init`, and by `ness upgrade` (R-UPD-061), as the running binary's major.minor."
  - **R-CFG-011**: "For a 0.x pin `0.N`, a command MUST run only when the binary is `0.N.x` or `0.(N+1).x`, and refuse otherwise. `init`, `--version`, `--help`, `doctor`, `self-update` and `upgrade` skip the check." Verify: "pin `0.3`: binary 0.3.0, 0.3.7 and 0.4.0 run; 0.2.9 and 0.5.0 exit 4; under 0.5.0 `ness doctor` and `ness upgrade` still run."
  - **R-CFG-013**: the refusal also names the fix, from the one string `config.PinError.Fix()` that the hooks print too: a binary older than the pin gets `ness self-update`; a pin older than N-1 gets `run ness upgrade in the base checkout; open sessions then merge their base`. Verify: "pin 0.3, binary 0.2.9: stderr ends `fix: ness self-update`; binary 0.5.0: stderr ends `fix: run ness upgrade in the base checkout; open sessions then merge their base`."
  - **R-CFG-014**: "Within the pinned minor or the next one…" Verify: "lock 0.3.0, binary 0.4.1; `send` and `inbox` succeed."
  - **R-HOOK-002**: append "…except R-UPD-020..022 inside a ness session (`NESS_SESSION` set, or the current branch matches `session/*`)." **R-HOOK-016**: add "Skew is not an unknown event: see R-UPD-020."
  - **R-SYNC-032**: add "A binary `X.Y.Z-rc.N` counts as X.Y.Z for the upper bound, and a lock of `X.Y.Z-rc.N` counts as X.Y.Z for the lower bound." Verify: "lock 0.1.0, binary 0.2.0-rc.1 → the 0.2.0 notes are printed; lock 0.2.0-rc.1, binary 0.2.0 → no notes."
  - **R-SYNC-033**: unchanged text, plus the second Verify case above.
  - **R-SYNC-089**: add "The worker's update item quotes the same Running-sessions subsections (R-UPD-070)."
  - **R-SYNC-072**: "Sync MUST NOT write any file outside the repository except the machine registry file `$XDG_CONFIG_HOME/ness/projects` (R-UPD-031)." Verify: "`$HOME` snapshot unchanged except `$XDG_CONFIG_HOME/ness/projects`; no other file (no lock file) appears."
  - **R-WRK-003**: add "…re-pointed by `ness upgrade` and `ness self-update` when the installed path changes (R-UPD-048, R-UPD-062)."
  - **R-WRK-009**: add "A self-update writes at the same path, so this check picks the new binary up."
  - **R-DOC-001**: "…MUST NOT modify any file except the update-check cache `$XDG_CACHE_HOME/ness/update.json` (R-UPD-050)…" (settled point 3).
  - **R-MSG-046**: add "…after the backup of R-UPD-010."
  - **R-QA-004/R-QA-031**: add "…and rc pre-releases (R-UPD-091)."

- [ ] **Step 3: Review-only rules.** A fresh `ness-review` subagent (not the author) reads R-UPD-003
  and R-UPD-095 against `specs.md` and appends, in the ledger format,
  `- R-UPD-003 | <reviewer session> | <date> | <spec commit>` and the same for R-UPD-095 to
  `docs/spec/reviews.md`. (R-UPD-091 gets its entry in Task 14, after `rc.yml` exists.)

- [ ] **Step 4: Check that the ids parse and are unique**
  Add "UPD" to `doneAreas` in `internal/testkit/coverage_repo_test.go`.
  Run: `go test ./internal/testkit/ -run TestRequirementCoverage -count=1`
  Expected: FAIL, listing every R-UPD id as uncovered (the expected red state until the covering
  tests and the Task 17 review entries land) and no duplicate or undefined id.

- [ ] **Step 5: Commit**
  `git add docs/spec && git commit -m "docs(spec): update path rules (UPD) and the amended pin, hook, sync, worker and doctor rules"`

---

### Task 1: Compatibility — pin widening, pin exemptions, version order, rc notes

**Files:**
- Modify: `internal/config/pin.go`, `internal/config/config.go` (`ParseError`), `internal/sync/notes.go`, `internal/cli/project.go` (`loadProject` wraps with `%w` and appends the fix; `locateProject` wraps the parse error with `%w`)
- Test: `internal/config/pin_test.go`, `internal/sync/notes_test.go`, Create `internal/sync/version_test.go` (holds `TestCompareNumericPrerelease`)
- Test (judge): Modify `internal/cli/testdata/script/pin.txtar`, `pin_patch.txtar` and `doctor.txtar` (its "every check fails at once" case moves from `NESS_TEST_VERSION=0.2.0` to `0.3.0`, because 0.2.0 now runs pin 0.1); Create `internal/cli/testdata/script/sync_notes_rc.txtar`

**Interfaces:**
- Consumes: —
- Produces:
  - `func CheckPin(pin, version string) error`: same signature, widened. A mismatch is a `*PinError`.
  - `type PinError struct{ Pin, Version string; Older bool }`, where `Older` means the binary is older than the pin.
    - `func (e *PinError) Error() string`: for a 0.x pin, `this project pins ness 0.N.x or 0.(N+1).x (ness = "0.N"); the running binary is V` (major ≥ 1 keeps today's text).
    - `func (e *PinError) Fix() string`: `ness self-update` when `Older`, else `run ness upgrade in the base checkout; open sessions then merge their base`. The only definition of the pin fix text: `loadProject` and the hooks (Task 3) both print it.
  - `type ParseError struct{ Err error }` with `Error()` = `Err.Error()` (text unchanged) and `Unwrap()`. `Parse` returns every error wrapped in it.
  - `loadProject(d Deps) (*project, error)`: the refusal becomes `refuse("%w (binary: %s); fix: %s", pe, executable(), pe.Fix())` (`executable()` becomes `d.Executable` in Task 6), so `errors.As` finds `*config.PinError` through the `exitError`. `locateProject` returns `refuse("%w", err)` for a parse error.
  - `func SetPin(data []byte, pin string) ([]byte, error)` rewrites the top-level `ness = "…"` line and leaves every other byte unchanged. It errors when there is no such line.
  - `func Compare(a, b string) (int, error)` in package `sync`: semver order, with numeric prerelease identifiers compared as numbers.
  - `func Between(notes []Note, from, to string) ([]Note, error)`: same signature, rc rule added.
  - `func RunningSessions(n Note) string` returns the body under `### Running sessions`, trimmed.

- [ ] **Step 1 (judge: written by a separate subagent from the implementer): acceptance scripts.**
  - `pin.txtar`, `# covers: R-CFG-011, R-CFG-013, R-CFG-014`: pin `0.3` under `NESS_TEST_VERSION` 0.3.0, 0.3.7 and 0.4.0 runs `ness status`. 0.2.9 → `exits 4`, `stderr 'pins ness 0\.3\.x or 0\.4\.x .*; fix: ness self-update$'`. 0.5.0 → `exits 4`, `stderr 'fix: run ness upgrade in the base checkout; open sessions then merge their base$'`. Under 0.5.0, `exec ness doctor` still runs (exit 1, `stdout '^fail\tbinary\t'`) and `exec ness --version` runs.
  - `pin_patch.txtar`: a lock of 0.3.0 under binary 0.4.1 lets `ness send` and `ness inbox` succeed.
  - New `sync_notes_rc.txtar`, `# covers: R-SYNC-032, R-SYNC-033`: a lock with `"ness": "0.1.0"`, `NESS_TEST_VERSION=0.2.0-rc.1` and `NESS_TEST_NOTES` holding a 0.2.0 section → `stdout 'ness 0.2.0 release notes'`. Then `NESS_TEST_VERSION=0.2.0` → `! stdout 'release notes'`.
  - `doctor.txtar`: the all-fail case under 0.3.0 keeps its expected lines.

- [ ] **Step 2: Write the failing unit tests** (every error checked; no `_ =` on a result the test reads)
```go
func TestCheckPinWidened(t *testing.T) {
	// covers: R-CFG-011
	for _, v := range []string{"0.3.0", "0.3.7", "0.4.0", "0.4.2-rc.1"} {
		if err := config.CheckPin("0.3", v); err != nil { t.Errorf("pin 0.3, binary %s: %v", v, err) }
	}
	for v, older := range map[string]bool{"0.2.9": true, "0.5.0": false, "1.0.0": false} {
		var pe *config.PinError
		if err := config.CheckPin("0.3", v); !errors.As(err, &pe) { t.Fatalf("pin 0.3, binary %s: %v, want *PinError", v, err) }
		if pe.Older != older { t.Errorf("%s: Older = %v", v, pe.Older) }
	}
}
func TestPinErrorTextAndFix(t *testing.T) {
	// covers: R-CFG-013
	var pe *config.PinError
	if !errors.As(config.CheckPin("0.3", "0.2.9"), &pe) { t.Fatal("want *PinError") }
	if got := pe.Error(); got != `this project pins ness 0.3.x or 0.4.x (ness = "0.3"); the running binary is 0.2.9` { t.Errorf("Error = %q", got) }
	if pe.Fix() != "ness self-update" { t.Errorf("Fix = %q", pe.Fix()) }
	if !errors.As(config.CheckPin("0.1", "0.3.0"), &pe) { t.Fatal("want *PinError") }
	if pe.Fix() != "run ness upgrade in the base checkout; open sessions then merge their base" { t.Errorf("Fix = %q", pe.Fix()) }
}
func TestParseErrorWraps(t *testing.T) {
	_, err := config.Parse([]byte("ness = \"0.1\"\nbogus = 1\n"))
	var pe *config.ParseError
	if !errors.As(err, &pe) || !strings.Contains(err.Error(), "unknown key") { t.Fatalf("Parse = %v", err) }
}
func TestSetPinKeepsTheRest(t *testing.T) {
	in := []byte("# top\nness = \"0.1\"   # pinned\nproject = \"x\"\n[update]\nauto = \"install\"\n")
	got, err := config.SetPin(in, "0.2")
	want := "# top\nness = \"0.2\"   # pinned\nproject = \"x\"\n[update]\nauto = \"install\"\n"
	if err != nil || string(got) != want { t.Fatalf("SetPin = %q, %v", got, err) }
	if _, err := config.SetPin([]byte("project = \"x\"\n"), "0.2"); err == nil { t.Fatal("no pin line: want error") }
	if _, err := config.SetPin([]byte("[x]\nness = \"0.1\"\n"), "0.2"); err == nil { t.Fatal("table key: want error") }
}
func TestCompareNumericPrerelease(t *testing.T) {
	for _, c := range []struct{ a, b string; want int }{
		{"0.2.0-rc.10", "0.2.0-rc.9", 1}, {"0.2.0-rc.1", "0.2.0", -1}, {"0.2.0+abc", "0.2.0", 0},
		{"v0.10.0", "0.9.9", 1},
	} {
		if got, err := nsync.Compare(c.a, c.b); err != nil || got != c.want { t.Errorf("Compare(%s,%s) = %d, %v", c.a, c.b, got, err) }
	}
	if _, err := nsync.Compare("main", "0.1.0"); err == nil { t.Error("non-semver: want error") }
}
func TestBetweenRC(t *testing.T) {
	// covers: R-SYNC-032, R-SYNC-033
	notes := []nsync.Note{{Version: "0.2.0", Body: "a\n### Running sessions\n* merge\n"}, {Version: "0.1.0", Body: "### Running sessions\n* none\n"}}
	got, err := nsync.Between(notes, "0.1.0", "0.2.0-rc.1")
	if err != nil || len(got) != 1 || got[0].Version != "0.2.0" { t.Fatalf("0.1.0 → rc.1 = %v, %v, want [0.2.0]", got, err) }
	got, err = nsync.Between(notes, "0.2.0-rc.1", "0.2.0")
	if err != nil || len(got) != 0 { t.Fatalf("rc.1 → 0.2.0 = %v, %v, want none", got, err) }
	if s := nsync.RunningSessions(notes[0]); s != "* merge" { t.Fatalf("RunningSessions = %q", s) }
}
```

- [ ] **Step 3: Run them and see them fail**
  Run: `go test ./internal/config/ ./internal/sync/ -run 'TestCheckPinWidened|TestPinError|TestParseError|TestSetPin|TestCompare|TestBetweenRC' -count=1`
  Expected: FAIL. `PinError`, `ParseError`, `SetPin`, `Compare` and `RunningSessions` are undefined.

- [ ] **Step 4: Implement**
  - `CheckPin`: for pm == 0, accept `vm == 0 && (vn == pn || vn == pn+1)`. On a mismatch, return `&PinError{Pin: pin, Version: version, Older: vm < pm || (vm == pm && vn < pn)}`.
  - `loadProject`/`locateProject` as in Produces.
  - `SetPin`: match the first line `(?m)^ness\s*=\s*"[^"]*"` that comes before the first `[` table header, and replace only the quoted value.
  - `Compare`: replace `version.less` with SemVer 2.0 precedence. A prerelease identifier made of digits only compares numerically. Build metadata is ignored. `Between` uses `Compare`, and before comparing it maps `to` = `X.Y.Z-pre` to `X.Y.Z` and `from` = `X.Y.Z-pre` to `X.Y.Z`.
  - `RunningSessions`: the text after the `### Running sessions` line, up to the next `### ` heading or the end, trimmed.

- [ ] **Step 5: Run the unit tests and the judge scripts**
  Run: `go test ./internal/config/ ./internal/sync/ -count=1 && go test ./internal/cli/ -run 'TestScripts/(sync_notes_rc|pin.*|doctor|release_notes.*)' -count=1`
  Expected: PASS.

- [ ] **Step 6: Commit**
  `git commit -m "feat(config): a 0.N binary runs 0.N-1 pins; the pin refusal names its fix; rc versions order and print their release's notes"`

---

### Task 2: Store backup before migration; doctor rollback

**Files:**
- Modify: `internal/store/migrate.go`, `internal/store/store.go` (`BackupPath`; `Open` becomes `open(path, migrations)`), `internal/store/export_test.go` (`OpenWith`), `internal/cli/doctor.go` (checks list), `internal/cli/doctor_checks.go` (`checkStore`), `internal/testkit/store.go` (`store schema VAR`)
- Test: Create `internal/store/backup_test.go`. Judge: create `internal/cli/testdata/script/doctor_store_rollback.txtar`; refresh the `golden.json` of `doctor.txtar` and the doctor entry of `json_goldens.txtar` for the new `store` check.

**Interfaces:**
- Consumes: —
- Produces:
  - `func BackupPath(path string, schema int) string` returns `path + ".bak-v" + strconv.Itoa(schema)`.
  - `store.Open(path string) (*Store, error)`: unchanged signature. It now backs up before migrating.
  - Test-only, in `export_test.go`: `func OpenWith(path string, migs []string) (*Store, error)` opens through the same `open` with a prefix of the migrations, so a test builds a store at schema N with `OpenWith(path, store.Migrations[:N])`.
  - `func checkStore(e *doctorEnv) check`, check name `store`, first in `checks` after `checkBinary`.
  - Testscript `store schema VAR` sets `$VAR` to `store.Version()`.

- [ ] **Step 1 (judge: written by a separate subagent): `doctor_store_rollback.txtar`**, `# covers: R-UPD-012`.
  After `ness init`: `store schema SCHEMA`, `store set-version 99` → `exits 1 ness doctor`, and
  `stdout '^fail\tstore\tstore schema 99, binary schema '$SCHEMA'.*\tfix: ness self-update'`.
  Then create `.git/ness/state.sqlite3.bak-v$SCHEMA` → `stdout 'cp .*state\.sqlite3\.bak-v'$SCHEMA' .*state\.sqlite3'`.
  An older store (`store set-version 1`, written raw) gives `stdout '^warn\tstore\t'`. A current
  store gives `stdout '^ok\tstore\tschema '$SCHEMA`. No schema number is hard-coded.

- [ ] **Step 2: Write the failing unit tests** (black-box, `package store_test`; helpers
  `oldStore(t, n) string` = `OpenWith(path, store.Migrations[:n])` then `Close`, and
  `schemaOf(t, path) int` = `PRAGMA user_version` over raw `database/sql`, both with `t.Helper()`)
```go
func TestBackupBeforeMigration(t *testing.T) {
	// covers: R-UPD-010, R-MSG-046
	path := oldStore(t, store.Version()-1)
	writeFile(t, store.BackupPath(path, store.Version()-1), "stale") // a stale file of the same name is replaced
	s, err := store.Open(path)
	if err != nil { t.Fatal(err) }
	t.Cleanup(func() { _ = s.Close() })
	if v := schemaOf(t, store.BackupPath(path, store.Version()-1)); v != store.Version()-1 { t.Fatalf("backup schema = %d", v) }
	if v := schemaOf(t, path); v != store.Version() { t.Fatalf("store schema = %d", v) }
}
func TestNoBackupForFreshOrCurrent(t *testing.T) {
	// covers: R-UPD-010
	path := filepath.Join(t.TempDir(), "state.sqlite3")
	for range 2 {
		s, err := store.Open(path)
		if err != nil { t.Fatal(err) }
		if err := s.Close(); err != nil { t.Fatal(err) }
	}
	if m, err := filepath.Glob(path + ".bak-v*"); err != nil || len(m) != 0 { t.Fatalf("backups = %v, %v, want none", m, err) }
}
func TestKeepTwoBackups(t *testing.T) {
	// covers: R-UPD-011 — keeps the backup just written and the newest one below it
	n := store.Version() - 1
	path := oldStore(t, n)
	for _, k := range []int{n - 2, n - 1, n + 1} { writeFile(t, store.BackupPath(path, k), "old") }
	s, err := store.Open(path)
	if err != nil { t.Fatal(err) }
	if err := s.Close(); err != nil { t.Fatal(err) }
	got, err := filepath.Glob(path + ".bak-v*")
	if err != nil { t.Fatal(err) }
	slices.Sort(got)
	if want := []string{store.BackupPath(path, n-1), store.BackupPath(path, n)}; !slices.Equal(got, want) { t.Fatalf("backups = %v, want %v", got, want) }
}
func TestBackupConcurrentOpeners(t *testing.T) {
	// Review focus 1: no opener fails, and the backup holds the old schema, not a half-migrated copy.
	n := store.Version() - 1
	path := oldStore(t, n)
	errs := make(chan error, 4)
	for range 4 { go func() { s, err := store.Open(path); if err == nil { err = s.Close() }; errs <- err }() }
	for range 4 { if err := <-errs; err != nil { t.Fatalf("Open: %v", err) } }
	if v := schemaOf(t, store.BackupPath(path, n)); v != n { t.Fatalf("backup schema = %d, want %d", v, n) }
	if m, _ := filepath.Glob(path + ".bak-v*.tmp*"); len(m) != 0 { t.Fatalf("temp files left: %v", m) }
}
```

- [ ] **Step 3: Run them** with `go test ./internal/store/ -run 'TestBackup|TestNoBackup|TestKeepTwo' -count=1`.
  Expected: FAIL (`BackupPath`, `OpenWith` undefined).

- [ ] **Step 4: Implement**
  - In `migrate`, when `0 < v < len(migs)`, before the transaction (VACUUM cannot run inside
    one): `VACUUM INTO '<BackupPath(v)>.tmp-<pid>'`, read the temp file's `user_version`, and
    rename it over `BackupPath(v)` only when it equals `v`; otherwise another opener migrated
    first: remove the temp and carry on. Then the existing immediate transaction (it re-reads the
    version, so the migration runs once). Prune afterwards: keep `BackupPath(v)` and the highest
    N below v, remove every other `bak-v*`. A failed prune is ignored. A failed backup (disk
    full) refuses the migration and leaves the store untouched. Quote the path for SQL by
    doubling any single quote.
  - `checkStore`:
    - `storeErr` wrapping `ErrNewer` → `fail("store", "store schema A, binary schema B (written by a newer ness)", fix)`. The fix is `ness self-update`, followed by `; or roll back: cp <dir>/state.sqlite3.bak-vB <dir>/state.sqlite3 (loses what the newer ness wrote since)` when that file exists.
    - `ErrOlder` → `warn` with the `ErrOlder` text.
    - No store → `ok("store", "no store yet")`.
    - Otherwise `ok("store", "schema N")`.
  - `viaStore`'s generic fix becomes `see the store check`.

- [ ] **Step 5: Run** `go test ./internal/store/ -count=1 -race && go test ./internal/cli/ -run 'TestScripts/(doctor|json_goldens)' -count=1`, refreshing the goldens where the new `store` line is the only difference. Expected: PASS.

- [ ] **Step 6: Commit** `git commit -m "feat(store): back up the store before a migration; doctor prints the rollback"`

---

### Task 3: Loud hooks on skew

**Files:**
- Create: `internal/hook/ness.go`, `internal/hook/ness_test.go`
- Modify: `internal/cli/hook.go` (`bind` including its early return at hook.go:130, `preTool`, `sessionStart`, new `skewLine`, new `onSessionBranch`)
- Test (judge): Create `internal/cli/testdata/script/hook_skew.txtar`

**Interfaces:**
- Consumes: `*config.PinError` and `PinError.Fix()`, `*config.ParseError` (Task 1, found with `errors.As` through `loadProject`'s `%w`), `store.ErrNewer`
- Produces:
  - `func NessCommand(p Payload) bool` in package `hook`: true only for `tool_name == "Bash"` when `tool_input.command`, trimmed, has `ness` as its first field and contains none of `;` `&` `|` `` ` `` `$` `<` `>` `(` `)` `\` `\n`.
  - `func onSessionBranch(d Deps) bool`: `git symbolic-ref -q --short HEAD` in the working directory has the prefix `session/`. One git call, no store read.
  - `func inSession(d Deps) bool` = `d.Session != store.Operator || onSessionBranch(d)`.
  - A new `bindSkew bindState`. `bound` gains `skew string`. `bind(d Deps, always bool) (bound, bindState)` keeps its signature. Its early return becomes `if !always && d.Session == store.Operator && d.Conversation == "" && !onSessionBranch(d)`.
  - `func skewLine(err error) string` builds the one line from the cause: `hooks cannot run in this session: <cause>; fix: <fix>`. The pin fix is `PinError.Fix()` (never a second copy of the text); the store fix is `ness self-update (ness doctor shows the rollback)`; the config fix is `correct .ness/ness.toml`.

- [ ] **Step 1 (judge: written by a separate subagent): `hook_skew.txtar`**, `# covers: R-UPD-020, R-UPD-021, R-UPD-022, R-UPD-023, R-HOOK-002, R-HOOK-016`.
  Init a project, commit, `store add-session s-a a`, `env NESS_SESSION=s-a`.
  - Pin skew: edit the pin to `"0.3"` (binary 0.1.0). `stdin edit.json` → `exits 2 ness hook pre-tool`, with
    `stderr '^ness: hooks cannot run in this session: this project pins ness 0\.3\.x or 0\.4\.x .*; fix: ness self-update$'`
    and exactly one stderr line.
    - `stdin bash_ness.json` (`{"tool_name":"Bash","tool_input":{"command":"ness doctor"}}`) → `exec ness hook pre-tool`, `! stderr .`.
    - `stdin bash_rm.json` (`rm x`) → `exits 2`.
    - `stdin bash_chain.json` (`ness doctor && rm -rf x`) → `exits 2`. Same for `ness doctor & rm -rf x`, `ness doctor; rm x`, `ness doctor | sh` and `ness $(rm x)`.
    - `exec ness hook session-start` → `stdout '^ness: hooks cannot run in this session: '`, exit 0.
  - Pin older than N-1: the pin goes to `"0.1"` with `NESS_TEST_VERSION=0.3.0` → `stderr 'fix: run ness upgrade in the base checkout; open sessions then merge their base$'`.
  - Newer store: `store set-version 99` → `stderr 'store schema 99, binary schema [0-9]+.*; fix: ness self-update \(ness doctor shows the rollback\)$'`.
  - Broken config: append `bogus = 1` → `stderr 'unknown key bogus.*; fix: correct \.ness/ness\.toml$'`.
  - Adopted root: `env NESS_SESSION=` and `env CLAUDE_CODE_SESSION_ID=`, `git checkout -q -b session/r-1`, `store set-version 99` → `stdin edit.json`, `exits 2 ness hook pre-tool`, `stderr 'store schema 99'`.
  - Outside a session: `env NESS_SESSION=` on branch `main` → each skew case gives empty stdout and stderr and exit 0. An unknown event gives the same.
  - A healthy project (no skew) with `NESS_SESSION=s-zzz`, a session the store does not know → exit 0, silent.

- [ ] **Step 2: Failing unit test**
```go
func TestNessCommand(t *testing.T) {
	// covers: R-UPD-021
	for cmd, want := range map[string]bool{
		"ness doctor": true, "  ness self-update --check": true, "ness": true, "ness inbox --show i-1": true,
		"nessy x": false, "cd x && ness sync": false, "rm -rf ness": false, "": false,
		"ness doctor && rm -rf x": false, "ness doctor; rm x": false, "ness doctor || rm x": false,
		"ness doctor | sh": false, "ness `rm x`": false, "ness $(rm x)": false, "ness doctor > f": false,
		"ness doctor < f": false, "ness doctor\nrm x": false, "ness doctor & rm -rf x": false,
		"ness (x)": false, "ness a \\\nb": false,
	} {
		in, err := json.Marshal(map[string]string{"command": cmd})
		if err != nil { t.Fatal(err) }
		if got := hook.NessCommand(hook.Payload{ToolName: "Bash", ToolInput: in}); got != want { t.Errorf("%q = %v, want %v", cmd, got, want) }
	}
	edit, err := json.Marshal(map[string]string{"command": "ness doctor"})
	if err != nil { t.Fatal(err) }
	if hook.NessCommand(hook.Payload{ToolName: "Edit", ToolInput: edit}) { t.Error("non-Bash tool passed") }
	if hook.NessCommand(hook.Payload{ToolName: "Bash", ToolInput: json.RawMessage(`"ness"`)}) { t.Error("non-object input passed") }
}
```

- [ ] **Step 3: Run** `go test ./internal/hook/ -run TestNessCommand -count=1` and `go test ./internal/cli/ -run 'TestScripts/hook_skew' -count=1`. Expected: FAIL.

- [ ] **Step 4: Implement.** In `bind`, when `inSession(d)` and `loadProject` fails with a
  `*config.PinError` or `*config.ParseError`, or `openStore` fails with `ErrNewer`, return
  `bindSkew` with `skew = skewLine(err)`. Skew is checked before the session lookup, so it blocks
  for any session id. Every other failure, and every failure outside a session, stays
  `bindUnbound`. In `newHookCmd`, `bindSkew` + pre-tool → parse the payload. A `NessCommand`
  returns nil. Anything else, including a payload that does not parse, returns
  `block(b.skew)`. `bindSkew` + session-start → `say(stdout, "ness: %s", b.skew)` and return nil.
  `preTool`'s `projectConfig` error (profiles that do not resolve) is also blocked with
  `skewLine` when `b.c` is a session.

- [ ] **Step 5: Run** `go test ./internal/hook/ ./internal/cli/ -run 'TestNessCommand|TestScripts/hook_' -count=1`. Expected: PASS, the existing `hook_unbound*` scripts included.

- [ ] **Step 6: Commit** `git commit -m "feat(hook): block loudly on pin, store or config skew inside a ness session or session branch; single ness commands still pass"`

---

### Task 4: `[update]` config and the machine registry

**Files:**
- Modify: `internal/config/config.go` (`Update`, defaults, validate)
- Create: `internal/config/head.go`, `internal/config/head_test.go`, `internal/update/update.go` (package comment only: "Package update finds, verifies and installs ness releases, and knows which projects on this machine use them."), `internal/update/registry.go`, `internal/update/registry_test.go`
- Modify: `internal/cli/cli.go` (`Deps.Registry update.Registry`, `Deps.CacheDir string`), `internal/cli/sync.go` (record after apply), `cmd/ness/main.go`, `internal/cli/script_test.go`, `internal/testkit/testkit.go` (`Setup` sets `XDG_CACHE_HOME=$HOME/.cache`)
- Test (judge): Create `internal/cli/testdata/script/sync_registry.txtar`. Modify `config_errors.txtar` and the R-SYNC-072 script (`grep -l R-SYNC-072 internal/cli/testdata/script/*`).

**Interfaces:**
- Consumes: —
- Produces:
  - `type Update struct{ Check bool \`toml:"check"\`; Channel string \`toml:"channel"\`; Auto string \`toml:"auto"\` }`, and `Config.Update Update \`toml:"update"\``.
  - `type Head struct{ Ness, Project string; Update Update }` and `func ReadHead(data []byte) (Head, error)`. ReadHead is lenient: it decodes only `ness`, `project` and `[update]`, ignores every other key (another binary's file), and applies the `Update` defaults.
  - `type Registry struct{ Path string }`:
    - `func (r Registry) Add(root string) error` reads the file, drops stale paths, appends `root` when missing, and writes through a temp file + rename in the same directory. No lock file: an append lost to a concurrent sync is healed by the next sync.
    - `func (r Registry) List() ([]string, error)` returns the listed paths that still hold `.ness/ness.toml`, in file order. It never writes.
    - An empty `Path` makes both return the error `no user config directory: the machine registry is off`.

- [ ] **Step 1 (judge: written by a separate subagent).**
  - `sync_registry.txtar`, `# covers: R-UPD-031, R-SYNC-072`:
    - `ness init` → `$XDG_CONFIG_HOME/ness/projects` equals `$WORK/repo\n`. A second `ness sync` leaves one line.
    - `ness sync --check` in a second repo leaves it unlisted.
    - `rm` the first repo's `.ness`, then `ness sync` in the second → the file lists only the second.
    - `exec ls $XDG_CONFIG_HOME/ness` → `stdout '^projects$'` and nothing else (no lock or temp file).
  - The R-SYNC-072 script excludes exactly `.config/ness/projects` from its `$HOME` snapshot.
  - `config_errors.txtar`, `# covers: R-UPD-030`: `[update] channel = "beta"` → exit 4 and `stderr 'update.channel = "beta": accepted values are stable, rc'`. Same for `auto = "always"` (`accepted values are off, notify, install`).

- [ ] **Step 2: Failing unit tests** (helper `project(t, dir, name) string` writes `<dir>/<name>/.ness/ness.toml` and returns the path)
```go
func TestUpdateDefaults(t *testing.T) {
	// covers: R-UPD-030
	f, err := config.Parse([]byte("ness = \"0.1\"\nproject = \"p\"\nbase = \"main\"\ngate = \"true\"\n"))
	if err != nil { t.Fatal(err) }
	c, err := f.Config(nil)
	if err != nil { t.Fatal(err) }
	if c.Update != (config.Update{Check: true, Channel: "stable", Auto: "notify"}) { t.Fatalf("defaults = %+v", c.Update) }
}
func TestReadHeadIgnoresUnknownKeys(t *testing.T) {
	// covers: R-UPD-046 — another binary's ness.toml is still readable for the ceiling and the veto
	h, err := config.ReadHead([]byte("ness = \"0.4\"\nproject = \"p\"\nfuture = 1\n[update]\nauto = \"install\"\n[newtable]\nx = 1\n"))
	if err != nil || h.Ness != "0.4" || h.Project != "p" || h.Update.Auto != "install" || !h.Update.Check || h.Update.Channel != "stable" { t.Fatalf("%+v, %v", h, err) }
}
func TestRegistryAddListPrune(t *testing.T) {
	// covers: R-UPD-031
	dir := t.TempDir()
	a, b := project(t, dir, "a"), project(t, dir, "b")
	r := update.Registry{Path: filepath.Join(dir, "cfg", "ness", "projects")}
	for _, p := range []string{a, b, a} { if err := r.Add(p); err != nil { t.Fatal(err) } }
	if data, err := os.ReadFile(r.Path); err != nil || string(data) != a+"\n"+b+"\n" { t.Fatalf("file = %q, %v", data, err) }
	if err := os.RemoveAll(filepath.Join(a, ".ness")); err != nil { t.Fatal(err) }
	got, err := r.List()
	if err != nil || !slices.Equal(got, []string{b}) { t.Fatalf("List = %v, %v, want [%s]", got, err, b) }
	if data, _ := os.ReadFile(r.Path); string(data) != a+"\n"+b+"\n" { t.Fatalf("List wrote the file: %q", data) }
	c := project(t, dir, "c")
	if err := r.Add(c); err != nil { t.Fatal(err) }
	if data, _ := os.ReadFile(r.Path); string(data) != b+"\n"+c+"\n" { t.Fatalf("Add did not prune: %q", data) }
	if entries, _ := os.ReadDir(filepath.Dir(r.Path)); len(entries) != 1 { t.Fatalf("dir = %v, want only projects", entries) }
}
func TestRegistryEmptyPath(t *testing.T) {
	if err := (update.Registry{}).Add("/x"); err == nil || !strings.Contains(err.Error(), "machine registry is off") { t.Fatalf("Add = %v", err) }
	if _, err := (update.Registry{}).List(); err == nil { t.Fatal("List: want error") }
}
```

- [ ] **Step 3: Run** `go test ./internal/config/ ./internal/update/ -count=1`. Expected: FAIL.

- [ ] **Step 4: Implement.**
  - `Add` and `List` as in Produces; `ponytail:` comment on `Add`: "no lock; a lost concurrent append is healed by the next sync".
  - `runSync`, after `applyWithNotes` succeeds: `d.Registry.Add(primary)`. An error is printed as `ness: registry: …` and does not fail the sync.
  - `cmd/ness` sets `Registry{Path: filepath.Join(os.UserConfigDir(), "ness", "projects")}` and `CacheDir = filepath.Join(os.UserCacheDir(), "ness")`. When the user dir is unknown it uses `""` (both then error where used, never a relative path).
  - The testscript harness does the same.

- [ ] **Step 5: Run** `go test ./internal/config/ ./internal/update/ -race -count=1 && go test ./internal/cli/ -run 'TestScripts/(sync_|config_errors|init)' -count=1`. Expected: PASS.

- [ ] **Step 6: Commit** `git commit -m "feat(config): [update] check, channel and auto; sync records the project in the machine registry"`

---

### Task 5: `internal/update` core

**Files:**
- Create: `internal/update/release.go`, `verify.go`, `install.go`, `managed.go`, `ceiling.go`, `cache.go`, `decide.go`, `failure.go`, with tests `release_test.go`, `verify_test.go`, `install_test.go`, `managed_test.go`, `ceiling_test.go`, `cache_test.go`, `decide_test.go`, `failure_test.go`, and the test helper file `helpers_test.go` (`tarGz`, `writeFile`, `failingSource`)

**Interfaces:**
- Consumes: `config.CheckPin` (Task 1), `nsync.Compare` (Task 1)
- Produces:
```go
type Release struct { Version string; Prerelease bool; Assets map[string]string } // asset name → URL
type Source interface { // consumer-declared port, 2 methods
	Releases(ctx context.Context) ([]Release, error)
	Fetch(ctx context.Context, url string) (io.ReadCloser, error)
}
func Resolve(rels []Release, channel, want string) (Release, error)
func AssetName(version, goos, goarch string) string // "ness_<v>_<os>_<arch>.tar.gz"
func (r Release) Asset(goos, goarch string) (string, error) // URL, or the unbuilt-pair error
const Checksums = "checksums.txt"
func Verify(sums []byte, name string, data []byte) error
func Extract(archive []byte) ([]byte, error)
func Install(binary string, data []byte, want string, probe func(path string) (string, error)) (prev string, err error)
func ManagedBy(binary, version string, getenv func(string) string, home string) (command string, ok bool)
type Pin struct { Path, Pin, Primary string } // Path: the ness.toml; Primary: its registered root, named in the fix
func Ceiling(version string, pins []Pin) error
type Cache struct { CheckedAt time.Time `json:"checked_at"`; Stable string `json:"stable"`; RC string `json:"rc"` }
const CheckEvery = 24 * time.Hour
const CheckTimeout = 5 * time.Second
func CachePath(dir string) (string, error)  // dir == "" → error "no user cache directory: the update check is off"
func ReadCache(path string) (Cache, error)   // missing → zero Cache, nil
func WriteCache(path string, c Cache) error  // temp + rename
func CacheFrom(rels []Release, now time.Time) Cache // newest stable and newest prerelease, no fetch
func (c Cache) Due(now time.Time) bool
func (c Cache) Offer(channel, installed string) string // "" when nothing newer
func Refresh(ctx context.Context, src Source, old Cache, now time.Time) Cache // one Releases call + CacheFrom; a failure keeps old versions, stamps now
type Inputs struct {
	Installed, Lock, Offer, Auto string
	Autos   map[string]string // registered project root → its update.auto
	Managed bool
	Ceiling error
	Idle    bool
}
type Plan struct { Install string; Upgrade, Behind bool; Note string }
func Decide(in Inputs) Plan
type Failure struct { Version, Cause string; At time.Time }
func ReadFailure(path string) (Failure, bool, error)
func RecordFailure(path string, f Failure) (changed bool, err error) // writes f; changed = the stored cause differed (or none was stored)
func ClearFailure(path string) error // missing file is not an error
```

- [ ] **Step 1 (judge: written by a separate subagent from the implementer): the unit tests below**, failing. Each test names the wrong implementation it rejects in its comment. Every error is checked.
```go
func TestResolve(t *testing.T) {
	// covers: R-UPD-040
	rels := []update.Release{{Version: "0.2.0"}, {Version: "0.3.0-rc.1", Prerelease: true}, {Version: "0.1.0"}}
	for _, c := range []struct{ ch, want, got string }{{"stable", "", "0.2.0"}, {"rc", "", "0.3.0-rc.1"}, {"stable", "0.1.0", "0.1.0"}, {"stable", "v0.1.0", "0.1.0"}} {
		r, err := update.Resolve(rels, c.ch, c.want)
		if err != nil || r.Version != c.got { t.Errorf("Resolve(%s,%q) = %s, %v", c.ch, c.want, r.Version, err) }
	}
	if _, err := update.Resolve(rels, "stable", "9.9.9"); err == nil || !strings.Contains(err.Error(), "no release 9.9.9") { t.Errorf("unknown version: %v", err) }
	if _, err := update.Resolve(nil, "stable", ""); err == nil || !strings.Contains(err.Error(), "no stable release") { t.Errorf("empty: %v", err) }
	r, err := update.Resolve([]update.Release{{Version: "0.3.0-rc.2", Prerelease: true}, {Version: "0.3.0"}}, "rc", "")
	if err != nil || r.Version != "0.3.0" { t.Errorf("rc channel = %s, %v, want 0.3.0", r.Version, err) } // rejects: rc channel = prereleases only
}
func TestResolveRCTen(t *testing.T) { // Review focus 2; rejects: string comparison of prerelease ids
	r, err := update.Resolve([]update.Release{{Version: "0.3.0-rc.9", Prerelease: true}, {Version: "0.3.0-rc.10", Prerelease: true}}, "rc", "")
	if err != nil || r.Version != "0.3.0-rc.10" { t.Fatalf("got %s, %v", r.Version, err) }
}
func TestAssetUnbuiltPair(t *testing.T) {
	// covers: R-UPD-044
	r := update.Release{Version: "0.2.0", Assets: map[string]string{"ness_0.2.0_linux_amd64.tar.gz": "u"}}
	if u, err := r.Asset("linux", "amd64"); err != nil || u != "u" { t.Fatal(u, err) }
	_, err := r.Asset("linux", "riscv64")
	if err == nil || !strings.Contains(err.Error(), "linux/riscv64") || !strings.Contains(err.Error(), "go install github.com/enea-scaccabarozzi/ness/cmd/ness@v0.2.0") { t.Fatalf("err = %v", err) }
}
func TestVerify(t *testing.T) {
	// covers: R-UPD-042
	data := []byte("archive"); sum := sha256.Sum256(data)
	sums := []byte("0000  ness_0.2.0_linux_arm64.tar.gz\n" + hex.EncodeToString(sum[:]) + "  ness_0.2.0_linux_amd64.tar.gz\n")
	if err := update.Verify(sums, "ness_0.2.0_linux_amd64.tar.gz", data); err != nil { t.Fatal(err) }
	if err := update.Verify(sums, "ness_0.2.0_linux_amd64.tar.gz", []byte("other")); err == nil || !strings.Contains(err.Error(), "checksum mismatch for ness_0.2.0_linux_amd64.tar.gz") { t.Fatal(err) }
}
func TestVerifyMissingEntry(t *testing.T) { // Review focus 3; rejects: "no line" treated as verified
	if err := update.Verify([]byte("abc  other.tar.gz\n"), "ness_0.2.0_linux_amd64.tar.gz", nil); err == nil || !strings.Contains(err.Error(), "no entry for ness_0.2.0_linux_amd64.tar.gz") { t.Fatal(err) }
}
func TestExtract(t *testing.T) {
	// archives are built in the test with archive/tar + compress/gzip (helper tarGz(t, entries ...tarEntry))
	got, err := update.Extract(tarGz(t, file("README.md", "r"), file("ness", "bin")))
	if err != nil || string(got) != "bin" { t.Fatalf("Extract = %q, %v", got, err) }
	for name, archive := range map[string][]byte{
		"no ness":  tarGz(t, file("README.md", "r")),
		"nested":   tarGz(t, file("sub/ness", "bin")),
		"symlink":  tarGz(t, symlink("ness", "/bin/sh")),
		"not gzip": []byte("plain"),
	} {
		if _, err := update.Extract(archive); err == nil { t.Errorf("%s: want error", name) }
	}
	if _, err := update.Extract(tarGz(t, file("README.md", "r"))); err == nil || !strings.Contains(err.Error(), "archive has no ness binary") { t.Errorf("no ness: %v", err) }
}
func TestInstallSameDirMode(t *testing.T) {
	// covers: R-UPD-043 — Review focus 3; rejects: temp file in os.TempDir, lost mode, lost .prev
	dir := t.TempDir(); bin := filepath.Join(dir, "ness"); writeFile(t, bin, "old")
	writeFile(t, bin+".prev", "older")
	var probed string
	prev, err := update.Install(bin, []byte("new"), "0.2.0", func(p string) (string, error) { probed = p; return "ness version 0.2.0+abc", nil })
	if err != nil || prev != bin+".prev" { t.Fatal(prev, err) }
	if filepath.Dir(probed) != dir || probed == bin { t.Fatalf("probed %s, want a temp file in %s", probed, dir) }
	if b, err := os.ReadFile(bin); err != nil || string(b) != "new" { t.Fatalf("installed = %q, %v", b, err) }
	if b, err := os.ReadFile(prev); err != nil || string(b) != "old" { t.Fatalf("prev = %q, %v", b, err) }
	fi, err := os.Stat(bin)
	if err != nil || fi.Mode().Perm() != 0o755 { t.Fatalf("mode %v, %v", fi.Mode(), err) }
	if m, _ := filepath.Glob(filepath.Join(dir, ".ness-new-*")); len(m) != 0 { t.Fatal("temp left", m) }
}
func TestInstallProbeFailsKeepsOld(t *testing.T) {
	// covers: R-UPD-043 — Review focus 3
	for name, probe := range map[string]func(string) (string, error){
		"error": func(string) (string, error) { return "", errors.New("exec format error") },
		"wrong": func(string) (string, error) { return "ness version 0.1.9", nil },
	} {
		dir := t.TempDir(); bin := filepath.Join(dir, "ness"); writeFile(t, bin, "old")
		if _, err := update.Install(bin, []byte("new"), "0.2.0", probe); err == nil { t.Errorf("%s: want error", name) }
		if b, err := os.ReadFile(bin); err != nil || string(b) != "old" { t.Errorf("%s: old binary = %q, %v", name, b, err) }
		if m, err := os.ReadDir(dir); err != nil || len(m) != 1 { t.Errorf("%s: dir = %v, want only ness", name, m) }
	}
}
func TestManagedBy(t *testing.T) {
	// covers: R-UPD-045
	env := map[string]string{"GOPATH": "/g", "MISE_DATA_DIR": "/m"}
	get := func(k string) string { return env[k] }
	for bin, want := range map[string]string{
		"/m/installs/github-enea-scaccabarozzi-ness/0.1.0/ness":                  "mise use -g github:enea-scaccabarozzi/ness@0.2.0",
		"/h/.local/share/mise/installs/github-enea-scaccabarozzi-ness/0.1.0/ness": "",
		"/g/bin/ness":        "go install github.com/enea-scaccabarozzi/ness/cmd/ness@v0.2.0",
		"/h/go/bin/ness":     "go install github.com/enea-scaccabarozzi/ness/cmd/ness@v0.2.0",
		"/h/.local/bin/ness": "",
	} {
		got, ok := update.ManagedBy(bin, "0.2.0", get, "/h")
		if got != want || ok != (want != "") { t.Errorf("%s = %q,%v want %q", bin, got, ok, want) }
	}
	got, ok := update.ManagedBy("/h/.local/share/mise/installs/x/ness", "0.2.0", func(string) string { return "" }, "/h")
	if !ok || !strings.HasPrefix(got, "mise use -g") { t.Errorf("default mise dir = %q,%v", got, ok) }
}
func TestCeiling(t *testing.T) {
	// covers: R-UPD-046
	pins := []update.Pin{{Path: "/a/.ness/ness.toml", Pin: "0.2", Primary: "/a"}, {Path: "/a/.worktrees/x/.ness/ness.toml", Pin: "0.1", Primary: "/a"}}
	if err := update.Ceiling("0.2.5", pins); err != nil { t.Fatal(err) }
	err := update.Ceiling("0.3.0", pins)
	if err == nil || !strings.Contains(err.Error(), "/a/.worktrees/x/.ness/ness.toml") || !strings.Contains(err.Error(), `ness = "0.1"): run ness upgrade in /a; open sessions then merge their base`) { t.Fatal(err) }
	if strings.Contains(err.Error(), "/a/.ness/ness.toml (") { t.Errorf("names the passing pin too: %v", err) }
}
func TestCacheDueOffer(t *testing.T) {
	// covers: R-UPD-050
	t0 := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	c := update.Cache{CheckedAt: t0, Stable: "0.2.0", RC: "0.3.0-rc.1"}
	if c.Due(t0.Add(23*time.Hour)) || !c.Due(t0.Add(24*time.Hour)) || !(update.Cache{}).Due(t0) { t.Fatal("Due") }
	for _, o := range []struct{ ch, inst, want string }{
		{"stable", "0.1.0", "0.2.0"}, {"rc", "0.2.0", "0.3.0-rc.1"}, {"stable", "0.2.0", ""}, {"stable", "0.2.0+abc", ""}, {"stable", "0.3.0", ""},
	} {
		if got := c.Offer(o.ch, o.inst); got != o.want { t.Errorf("Offer(%s,%s) = %q, want %q", o.ch, o.inst, got, o.want) }
	}
}
func TestCacheFrom(t *testing.T) {
	t0 := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	got := update.CacheFrom([]update.Release{{Version: "0.1.0"}, {Version: "0.2.0-rc.10", Prerelease: true}, {Version: "0.2.0-rc.9", Prerelease: true}}, t0)
	if got != (update.Cache{CheckedAt: t0, Stable: "0.1.0", RC: "0.2.0-rc.10"}) { t.Fatalf("%+v", got) }
}
func TestCacheRoundTripAndEmptyDir(t *testing.T) {
	// covers: R-UPD-050 — an empty dir never resolves to a relative path in the repository
	if _, err := update.CachePath(""); err == nil || !strings.Contains(err.Error(), "no user cache directory") { t.Fatalf("CachePath(\"\") = %v", err) }
	p, err := update.CachePath(filepath.Join(t.TempDir(), "ness"))
	if err != nil || filepath.Base(p) != "update.json" || !filepath.IsAbs(p) { t.Fatalf("%s, %v", p, err) }
	if c, err := update.ReadCache(p); err != nil || c != (update.Cache{}) { t.Fatalf("missing: %+v, %v", c, err) }
	want := update.Cache{CheckedAt: time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC), Stable: "0.2.0"}
	if err := update.WriteCache(p, want); err != nil { t.Fatal(err) }
	if c, err := update.ReadCache(p); err != nil || !c.CheckedAt.Equal(want.CheckedAt) || c.Stable != "0.2.0" { t.Fatalf("%+v, %v", c, err) }
}
func TestRefreshFailureKeepsVersions(t *testing.T) {
	// covers: R-UPD-050
	t0 := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	got := update.Refresh(context.Background(), failingSource{}, update.Cache{Stable: "0.2.0"}, t0)
	if got.Stable != "0.2.0" || !got.CheckedAt.Equal(t0) { t.Fatalf("%+v", got) }
}
func TestFailureRecord(t *testing.T) {
	// covers: R-UPD-081 — log only when the cause changes, across processes
	p := filepath.Join(t.TempDir(), "upgrade-failed.json")
	t0 := time.Date(2026, 10, 31, 10, 0, 0, 0, time.UTC)
	if _, ok, err := update.ReadFailure(p); err != nil || ok { t.Fatalf("missing: %v, %v", ok, err) }
	for i, c := range []struct{ cause string; changed bool }{{"sync conflict", true}, {"sync conflict", false}, {"commit failed", true}} {
		changed, err := update.RecordFailure(p, update.Failure{Version: "0.2.0", Cause: c.cause, At: t0})
		if err != nil || changed != c.changed { t.Errorf("%d: changed = %v, %v", i, changed, err) }
	}
	if f, ok, err := update.ReadFailure(p); err != nil || !ok || f.Cause != "commit failed" || f.Version != "0.2.0" { t.Fatalf("%+v %v %v", f, ok, err) }
	if err := update.ClearFailure(p); err != nil { t.Fatal(err) }
	if err := update.ClearFailure(p); err != nil { t.Fatal("second clear:", err) }
	if _, ok, _ := update.ReadFailure(p); ok { t.Fatal("still recorded") }
}
func TestDecide(t *testing.T) {
	// covers: R-UPD-080, R-UPD-081, R-UPD-082 — table
	all := map[string]string{"/a": "install", "/b": "install"}
	mixed := map[string]string{"/a": "install", "/b": "notify"}
	for _, c := range []struct{ name string; in update.Inputs; want update.Plan }{
		{"install", update.Inputs{Installed: "0.1.0", Lock: "0.1.0", Offer: "0.2.0", Auto: "install", Autos: all}, update.Plan{Install: "0.2.0"}},
		{"veto", update.Inputs{Installed: "0.1.0", Lock: "0.1.0", Offer: "0.2.0", Auto: "install", Autos: mixed}, update.Plan{Note: `auto-install held: /b has auto = "notify"`}},
		{"managed", update.Inputs{Installed: "0.1.0", Lock: "0.1.0", Offer: "0.2.0", Auto: "install", Autos: all, Managed: true}, update.Plan{Note: "auto-install held: the binary is managed by another tool"}},
		{"ceiling", update.Inputs{Installed: "0.1.0", Lock: "0.1.0", Offer: "0.3.0", Auto: "install", Autos: all, Ceiling: errors.New("x")}, update.Plan{Note: "auto-install held: x"}},
		{"upgrade idle", update.Inputs{Installed: "0.2.0", Lock: "0.1.0", Auto: "install", Autos: all, Idle: true}, update.Plan{Upgrade: true}},
		{"upgrade busy", update.Inputs{Installed: "0.2.0", Lock: "0.1.0", Auto: "install", Autos: all}, update.Plan{}},
		{"behind notify", update.Inputs{Installed: "0.2.0", Lock: "0.1.0", Auto: "notify", Autos: mixed}, update.Plan{Behind: true}},
		{"behind off", update.Inputs{Installed: "0.2.0", Lock: "0.1.0", Auto: "off", Autos: mixed}, update.Plan{Behind: true}},
		{"notify offer", update.Inputs{Installed: "0.1.0", Lock: "0.1.0", Offer: "0.2.0", Auto: "notify", Autos: mixed}, update.Plan{}},
		{"current", update.Inputs{Installed: "0.2.0", Lock: "0.2.0", Auto: "notify"}, update.Plan{}},
	} {
		if got := update.Decide(c.in); got != c.want { t.Errorf("%s: %+v, want %+v", c.name, got, c.want) }
	}
}
```
  Install is evaluated before Upgrade: when both apply, Plan.Install wins this pass. The upgrade
  follows after the worker restarts on the new binary. In the veto Note, the vetoing project is
  the first in sorted root order.

- [ ] **Step 2: Run** `go test ./internal/update/ -count=1`. Expected: FAIL (undefined).

- [ ] **Step 3: Implement** the signatures above. Pinned behaviour the tests do not fix:
  - `Install` writes with `os.CreateTemp(dir, ".ness-new-*")` and `Chmod 0o755`, then runs the probe. The probe output's last field, with `+build` stripped, must equal `want`. Then it removes the old `.prev` and makes it with `os.Link(binary, prev)`, falling back to a copy. Last comes `os.Rename(tmp, binary)`.
  - `Extract` reads at most 200 MiB and accepts only a regular file named `ness` at the archive root.
  - Leave a `ponytail:` comment on `Decide`'s caller (Task 12) saying the veto reads each project's `ness.toml` on every pass.

- [ ] **Step 4: Run** `go test ./internal/update/ -race -count=1`. Expected: PASS.

- [ ] **Step 5: Commit** `git commit -m "feat(update): resolve, verify and install releases; ceiling, cache, failure record and the unattended decision"`

---

### Task 6: Release source atom, fake release server, wiring

**Files:**
- Create: `internal/atom/releases/releases.go` and `internal/atom/releases/releases_test.go`. Package comment: "Package releases is the release-source atom: the GitHub releases API of the ness repository over HTTP."
- Create: `internal/testkit/releases.go` (`ReleaseServer`, `FakeRelease`, plus the testscript commands `releases` and `lock`), `internal/testkit/releases_test.go`
- Modify: `internal/testkit/testkit.go` (`Cmds` gains `releases` and `lock`. `Setup` sets `NESS_RELEASES_URL=http://127.0.0.1:9`, a refused port, so no script reaches GitHub.)
- Modify: `internal/cli/cli.go` (`Deps.Releases update.Source`, `Deps.Executable string`), `internal/cli/project.go` (delete `executable()`; `trimDeleted` moves to `cmd/ness`), every `executable()` caller (`internal/cli/doctor.go`, `doctor_checks.go`, `project.go`, `open.go`) switches to `d.Executable`, `cmd/ness/main.go`, `internal/cli/script_test.go`

**Interfaces:**
- Consumes: `update.Release`, `update.Source` (Task 5)
- Produces:
  - `type Client struct{ Base string; HTTP *http.Client }`, `const DefaultBase = "https://api.github.com/repos/enea-scaccabarozzi/ness"`
  - `func (c Client) Releases(ctx context.Context) ([]update.Release, error)`. It sends GET `{Base}/releases?per_page=100`, skips drafts, strips the `v` from the tag, and maps assets `name → browser_download_url`.
  - `func (c Client) Fetch(ctx context.Context, url string) (io.ReadCloser, error)`. Any non-2xx answer is an error `releases: GET <url>: <status>`.
  - `func ReleaseServer(t testing.TB, rels ...FakeRelease) *httptest.Server` and `type FakeRelease struct{ Version string; Prerelease, BadSum bool; Arches []string }` (default `amd64`, `arm64`). Each archive holds a `ness` sh script that prints `ness version <Version>`. **`/releases` lists the releases newest first by semver (prerelease ids numerically, via `nsync.Compare`), whatever order they were given in, as GitHub does.** It serves the assets, and answers 500 to everything when built with `Version: "fail"`.
  - Testscript command `releases SPEC...`, with SPEC = `VERSION[:pre][:badsum]` or `fail`. It starts the server, `ts.Setenv("NESS_RELEASES_URL", url)`, truncates `$WORK/releases.log` and logs each request line to it, and defers Close.
  - Testscript command `lock PATH` holds a flock on PATH, from the test process, until the script ends.
  - `Deps.Executable string` replaces `executable()` (one source of the binary path). `cmd/ness` sets it from `os.Executable()` with " (deleted)" trimmed, or `"unknown path"` on error. The harness takes it from `NESS_TEST_EXECUTABLE`, else `os.Executable()`, so existing scripts see the same path as today.
  - `cmd/ness`: `Releases: releases.Client{Base: cmp.Or(os.Getenv("NESS_RELEASES_URL"), releases.DefaultBase), HTTP: &http.Client{Timeout: 2 * time.Minute}}`. The harness builds the same `Releases`.

  Acceptance for this task's CLI wiring is downstream: the judge scripts of Tasks 8, 10 and 12 (`self_update*`, `upgrade*`, `worker_auto`) drive the atom, the fake server and `Deps.Executable` end to end. This task's own tests are the unit tests below.

- [ ] **Step 1: Failing tests**
```go
func TestReleasesAgainstServer(t *testing.T) {
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "0.1.0"}, testkit.FakeRelease{Version: "0.3.0-rc.1", Prerelease: true}, testkit.FakeRelease{Version: "0.2.0"})
	c := releases.Client{Base: srv.URL, HTTP: srv.Client()}
	rels, err := c.Releases(context.Background())
	if err != nil { t.Fatal(err) }
	var order []string
	for _, r := range rels { order = append(order, r.Version) }
	if !slices.Equal(order, []string{"0.3.0-rc.1", "0.2.0", "0.1.0"}) { t.Fatalf("order = %v, want newest first", order) }
	r, err := update.Resolve(rels, "stable", "")
	if err != nil { t.Fatal(err) }
	url, err := r.Asset("linux", "amd64")
	if err != nil { t.Fatal(err) }
	data := fetchAll(t, c, url) // helper: Fetch + ReadAll + Close, each error fatal
	sums := fetchAll(t, c, r.Assets[update.Checksums])
	if err := update.Verify(sums, update.AssetName("0.2.0", "linux", "amd64"), data); err != nil { t.Fatal(err) }
	bin, err := update.Extract(data)
	if err != nil || !strings.Contains(string(bin), "ness version 0.2.0") { t.Fatalf("binary = %q, %v", bin, err) }
}
func TestReleaseServerOrdersRCNumerically(t *testing.T) {
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "0.3.0-rc.9", Prerelease: true}, testkit.FakeRelease{Version: "0.3.0-rc.10", Prerelease: true})
	rels, err := releases.Client{Base: srv.URL, HTTP: srv.Client()}.Releases(context.Background())
	if err != nil || len(rels) != 2 || rels[0].Version != "0.3.0-rc.10" { t.Fatalf("%v, %v", rels, err) }
}
func TestReleaseServerBadSum(t *testing.T) {
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "0.2.0", BadSum: true})
	c := releases.Client{Base: srv.URL, HTTP: srv.Client()}
	rels, err := c.Releases(context.Background())
	if err != nil { t.Fatal(err) }
	url, _ := rels[0].Asset("linux", "amd64")
	if err := update.Verify(fetchAll(t, c, rels[0].Assets[update.Checksums]), update.AssetName("0.2.0", "linux", "amd64"), fetchAll(t, c, url)); err == nil { t.Fatal("BadSum verified") }
}
func TestReleasesErrorStatus(t *testing.T) {
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "fail"})
	_, err := releases.Client{Base: srv.URL, HTTP: srv.Client()}.Releases(context.Background())
	if err == nil || !strings.Contains(err.Error(), "500") { t.Fatal(err) }
}
func TestReleasesSkipsDrafts(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/releases" || r.URL.Query().Get("per_page") != "100" { http.NotFound(w, r); return }
		_, _ = io.WriteString(w, `[{"tag_name":"v0.3.0","draft":true},{"tag_name":"v0.2.0","prerelease":false,"assets":[{"name":"checksums.txt","browser_download_url":"u"}]}]`)
	}))
	t.Cleanup(srv.Close)
	rels, err := releases.Client{Base: srv.URL, HTTP: srv.Client()}.Releases(context.Background())
	if err != nil || len(rels) != 1 || rels[0].Version != "0.2.0" || rels[0].Assets["checksums.txt"] != "u" { t.Fatalf("%+v, %v", rels, err) }
}
```

- [ ] **Step 2: Run** `go test ./internal/atom/releases/ ./internal/testkit/ -run 'TestReleases|TestReleaseServer' -count=1`. Expected: FAIL.

- [ ] **Step 3: Implement** as specified. The atom decodes only `tag_name`, `prerelease`, `draft` and `assets[].name/browser_download_url`.

- [ ] **Step 4: Run** `go test ./internal/atom/releases/ ./internal/testkit/ -count=1 && go test ./internal/cli/ -count=1`. Expected: PASS. Every existing script still passes with the new Setup variables and `d.Executable`.

- [ ] **Step 5: Commit** `git commit -m "feat(atom): GitHub release source; testkit fake release server; one source of the binary path"`

---

### Task 7: Worker unit re-point

**Files:**
- Modify: `internal/atom/systemd/systemd.go` (`ExecBinary`), `internal/cli/doctor_checks.go` (`checkWorkerBinary` uses it), `internal/cli/worker.go` (`repointWorker`, `restartWorker`)
- Test: `internal/atom/systemd/systemd_test.go`. Acceptance is downstream: no judge script of its own; the judge scripts `self_update` (Task 8), `upgrade` (Task 10) and `worker_auto` (Task 12) drive `repointWorker` and `restartWorker` through fake `systemctl` calls.

**Interfaces:**
- Consumes: —
- Produces:
  - `func (s Supervisor) ExecBinary(project string) (string, error)`. It returns `fs.ErrNotExist` when there is no unit, and an error when there is no ExecStart.
  - `func repointWorker(d Deps, project, primary, bin string) (changed bool, err error)` calls `sup.Install` only when `realPath(ExecBinary) != realPath(bin)`; `Install` rewrites the unit, runs `daemon-reload` and restarts it. With no unit, or an unchanged binary, it returns `false, nil`; a caller that needs fresh code anyway (`ness upgrade`) then calls `restartWorker`.
  - `func restartWorker(d Deps, project string) error` runs `systemctl --user restart ness-<project>.service` through `d.exec()`. With no unit it is a no-op.

- [ ] **Step 1: Failing test**
```go
func TestExecBinary(t *testing.T) {
	home := t.TempDir(); s := systemd.Supervisor{Home: home}
	if _, err := s.ExecBinary("p"); !errors.Is(err, fs.ErrNotExist) { t.Fatal(err) }
	writeUnit(t, s.UnitPath("p"), systemd.Unit("p", "/opt/my bin/ness", "/w", "/usr/bin"))
	if got, err := s.ExecBinary("p"); err != nil || got != "/opt/my bin/ness" { t.Fatal(got, err) }
}
```
- [ ] **Step 2: Run** `go test ./internal/atom/systemd/ -run TestExecBinary -count=1`. Expected: FAIL.
- [ ] **Step 3: Implement.** Move the ExecStart scan out of `checkWorkerBinary` into `ExecBinary`. Doctor's output must stay byte-identical (`doctor*.txtar`).
- [ ] **Step 4: Run** `go test ./internal/atom/systemd/ ./internal/cli/ -run 'TestExecBinary|TestScripts/(doctor|worker_install)' -count=1`. Expected: PASS.
- [ ] **Step 5: Commit** `git commit -m "refactor(systemd): read the unit's binary in one place; cli can re-point and restart a worker"`

---

### Task 8: `ness self-update`

**Files:**
- Create: `internal/cli/selfupdate.go`
- Modify: `internal/cli/cli.go` (register `newSelfUpdateCmd`, fix the "eighteen" comment)
- Test (judge): Create `internal/cli/testdata/script/self_update.txtar`, `self_update_ceiling.txtar` and `self_update_managed.txtar`

**Interfaces:**
- Consumes: `update.*` (Task 5), `Deps.Releases`/`Executable`/`Registry`/`CacheDir` (Tasks 4 and 6), `config.ReadHead` (Task 4), `repointWorker` (Task 7)
- Produces:
  - `func newSelfUpdateCmd(d Deps) *cobra.Command`, `Use: "self-update"`, with flags `--version X.Y.Z`, `--channel stable|rc` and `--check`. It reads the current project, if any, with `locateProject` (no pin check: R-CFG-011 exempts it).
  - `func machinePins(d Deps) ([]update.Pin, error)`: for every `d.Registry.List()` root, the root's `.ness/ness.toml` plus every `git -C root worktree list --porcelain` path holding one, read with `config.ReadHead`. Each `Pin.Primary` is its registered root.
  - `func installVersion(d Deps, rel update.Release) (prev string, err error)`: `Asset(runtime.GOOS, GOARCH)` → `Fetch` the archive and `checksums.txt` → `Verify` → `Extract` → `Install` into `d.Executable` (probe = `cli.SystemExec("", path, "--version")`, its output adapted to `(string, error)`) → `repointWorker` for every registered project (project name from `ReadHead`, `primary` = the root). It does no managed or ceiling check; its callers do (`runSelfUpdate` here, `Decide` in Task 12).
  - Output lines, pinned:
    - `ness X.Y.Z installed at P (previous kept at P.prev); workers pick it up within a minute`
    - `ness X.Y.Z is the latest on channel C`
    - `ness X.Y.Z available (installed A.B.C): ness self-update` (with `--check`)
    - The ceiling refusal: `ness X.Y.Z cannot run <ness.toml path> (ness = "0.N"): run ness upgrade in <primary checkout path>; open sessions then merge their base`
    - The managed refusal: `ness at P is managed by another tool; run: <command>`

- [ ] **Step 1 (judge: written by a separate subagent).** Every script writes the stub
  `$WORK/bin/ness` (`#!/bin/sh` + `echo ness version 0.1.0`, chmod 755) and sets
  `env NESS_TEST_EXECUTABLE=$WORK/bin/ness`.
  - `self_update.txtar`, `# covers: R-UPD-040, R-UPD-041, R-UPD-042, R-UPD-047, R-UPD-048`:
    - `releases 0.2.0 0.3.0-rc.1:pre` → `exec ness self-update --check` → `stdout '^ness 0\.2\.0 available \(installed 0\.1\.0\): ness self-update$'`, the stub is unchanged (`exec $WORK/bin/ness --version` → `0.1.0`), `$HOME/.cache/ness/update.json` holds `"stable": "0.2.0"` and `"rc": "0.3.0-rc.1"`, and `$WORK/releases.log` has exactly one `GET /releases` line (one fetch).
    - `exec ness self-update` → `stdout '^ness 0\.2\.0 installed at'`, `exec $WORK/bin/ness --version` → `stdout 'ness version 0\.2\.0'`, and `grep 0.1.0 $WORK/bin/ness.prev`.
    - `--channel rc` → 0.3.0-rc.1.
    - Inside a project with `[update] channel = "rc"` and no flag → rc. The same with that project's pin at `"0.9"` (pin exemption) → rc.
    - `releases 0.2.0:badsum` → `exits 4`, `stderr 'checksum mismatch'`, and the stub is unchanged.
    - A registered project whose fake unit (`$HOME/.config/systemd/user/ness-repo.service`) runs `/old/ness` → after the install the unit's `ExecStart` names `$WORK/bin/ness`, and the fake `systemctl` calls include `daemon-reload`.
    - `releases 0.1.0` → `stdout '^ness 0\.1\.0 is the latest on channel stable$'`; with `--check` the same line.
  - `self_update_ceiling.txtar`, `# covers: R-UPD-046`: a registered project with `ness = "0.1"` and `releases 0.3.0` → `exits 4`, `stderr 'cannot run .*repo/\.ness/ness\.toml \(ness = "0\.1"\): run ness upgrade in .*repo; open sessions then merge their base$'`, and the stub is unchanged. With the base pin at 0.2 and a worktree `.worktrees/x` pinned 0.1 → the refusal names the worktree's `ness.toml` and still the primary checkout as the place to run `ness upgrade`. `--version 0.2.0` succeeds.
  - `self_update_managed.txtar`, `# covers: R-UPD-045`: `NESS_TEST_EXECUTABLE=$HOME/go/bin/ness` → `exits 4`, `stderr 'run: go install github.com/enea-scaccabarozzi/ness/cmd/ness@v0\.2\.0'`, and the file is unchanged.

- [ ] **Step 2: Run** `go test ./internal/cli/ -run 'TestScripts/self_update' -count=1`. Expected: FAIL (unknown command).

- [ ] **Step 3: Implement** `runSelfUpdate` in this order: channel → one `Releases` call →
  with `--check`: `WriteCache(CacheFrom(rels, now))` (the same `rels`, no second fetch; a
  `CachePath` error is printed as `ness: update cache: …` and does not fail) and print → `Resolve`
  → latest check → `ManagedBy` → `Ceiling(machinePins)` → `installVersion`. The probe uses the
  real exec on purpose: the file it just wrote is not an external program to fake. Every refusal
  uses `refuse` (exit 4). A nil `d.Releases` refuses with `no release source`.

- [ ] **Step 4: Run** `go test ./internal/cli/ -run 'TestScripts/self_update|TestReference' -count=1`. Expected: the scripts PASS. `TestReferenceIsFresh` fails until it is refreshed: run `go test ./internal/cli/ -run TestReferenceIsFresh -update -count=1` and check the new `docs/reference/cli/self-update.md` and `meta.json`.

- [ ] **Step 5: Commit** `git commit -m "feat(cli): ness self-update: checksum-verified atomic install with the ceiling rule"`

---

### Task 9: Update-available check — doctor line and root session-start line

**Files:**
- Create: `internal/cli/update_check.go`
- Modify: `internal/cli/doctor.go` (add `checkUpdate` after `checkWorkerBinary`), `internal/cli/hook.go` (`sessionStart` prints the root lines)
- Test (judge): Create `internal/cli/testdata/script/update_notice.txtar`; refresh the `golden.json` of `doctor.txtar` and the doctor entry of `json_goldens.txtar` for the new `update` check.

**Interfaces:**
- Consumes: `update.CachePath`/`Cache`/`Refresh`/`Offer`/`ReadFailure` (Task 5), `Deps.CacheDir` and `Deps.Releases`
- Produces:
  - `func refreshCache(d Deps, now time.Time) update.Cache` resolves `update.CachePath(d.CacheDir)` (an error: return the zero Cache, write nothing), reads the cache, and when `Due` runs `Refresh` with a `context.WithTimeout(update.CheckTimeout)` and writes it. Errors are swallowed (R-UPD-050).
  - `func failurePath(p *project) (string, error)` returns `<git-common-dir>/ness/upgrade-failed.json`.
  - `func rootNotices(d Deps, p *project, cfg *config.Config, now time.Time) []string` reads only. It returns the available line (R-UPD-053) and the failure line `ness: the automatic upgrade to X.Y.Z failed: <cause>; the worker retries each pass (ness doctor shows it)`. The available line is hidden when `check = false` or `auto = "off"`; the failure line always shows.
  - `func checkUpdate(e *doctorEnv) check`, name `update`. It returns `warn("update", "ness X available (installed A)", "ness self-update")`, or `ok("update", "ness A is the latest on channel C (checked <date>)")`, or `ok("update", "check off ([update])")` when `check = false` or `auto = "off"`, or `ok("update", "no release information yet; next try within 24 h")`. A failure record gives `warn("update", "automatic upgrade to X failed: cause", "fix the cause; the worker retries each pass, or run ness upgrade")`.

- [ ] **Step 1 (judge: written by a separate subagent): `update_notice.txtar`**, `# covers: R-UPD-050, R-UPD-051, R-UPD-052, R-UPD-053`.
  - `releases 0.2.0`, then `exec ness doctor` → `stdout '^warn\tupdate\tness 0\.2\.0 available \(installed 0\.1\.0\)\tfix: ness self-update'`. `$HOME/.cache/ness/update.json` exists with `"stable": "0.2.0"`.
  - `releases fail` (truncates `$WORK/releases.log`), then doctor again within 24 h (`NESS_TEST_NOW` unchanged) → the same line from the cache, and `releases.log` is empty (no fetch).
  - Root session (`store add-session s-r r`) → `exec ness hook session-start` with `NESS_SESSION=s-r` → `stdout '^ness 0\.2\.0 available \(installed 0\.1\.0\): ness self-update$'`, and `releases.log` is empty. A child (`store add-session s-c c s-r`, `NESS_SESSION=s-c`) → `! stdout 'available'`.
  - `[update] auto = "off"` → no session-start line and `stdout '^ok\tupdate\tcheck off'`.
  - `store items operator 0`, `store items s-r 0`, and the fake HTTP log (`http.json`) has no request.
  - A planted `.git/ness/upgrade-failed.json` (`{"version":"0.2.0","cause":"sync conflict"}`) → the root's session-start shows `stdout 'automatic upgrade to 0\.2\.0 failed: sync conflict'`, and doctor shows `warn\tupdate\tautomatic upgrade`.
  - No cache directory: `env XDG_CACHE_HOME=` and `env HOME=` → `exec ness doctor` (any exit) → `! exists $WORK/repo/update.json` and no new file under `$WORK/repo` (`exec git status --porcelain` → `! stdout .`).

- [ ] **Step 2: Run** `go test ./internal/cli/ -run 'TestScripts/update_notice' -count=1`. Expected: FAIL.

- [ ] **Step 3: Implement.** `newDoctorEnv` calls `refreshCache(d, now)` once, only when `[update] check = true` (R-UPD-050) and keeps the
  result on `doctorEnv` (new field `cache update.Cache`). `sessionStart` prints `rootNotices`
  after the identity line, only when `b.c.sess.Parent == store.Operator`. Hooks call
  `update.ReadCache` only, never `refreshCache`.

- [ ] **Step 4: Run** `go test ./internal/cli/ -run 'TestScripts/(update_notice|doctor|json_goldens|hook_session_start)' -count=1`. Expected: PASS.

- [ ] **Step 5: Commit** `git commit -m "feat(cli): daily update check in doctor and the root's session-start line"`

---

### Task 10: `ness upgrade`

**Files:**
- Create: `internal/cli/upgrade.go`
- Modify: `internal/cli/cli.go` (register `newUpgradeCmd`), `internal/worker/lock.go` (`LockPath`; `Lock(dir)` becomes `LockPath(filepath.Join(dir, "worker.lock"))`)
- Test: `internal/worker/lock_test.go` (`TestLockPath`)
- Test (judge): Create `internal/cli/testdata/script/upgrade.txtar` and `upgrade_refusals.txtar`

**Interfaces:**
- Consumes: `config.SetPin`/`PinOf`, `nsync.Compare`, `store.Open`/`OpenReadOnly`/`BackupPath`, `runSync`, `repointWorker`/`restartWorker`
- Produces:
  - `func LockPath(path string) (release func(), err error)` in package `worker`: the body of today's `Lock`, on any path. A held lock is an error wrapping `ErrLocked`. The one flock helper for `worker.lock`, `install.lock` and the upgrade's hold on `gate.lock` and `integrate-<base>.lock` (it writes its pid there; integrate's waiter message then names a pid, which is advisory).
  - `func newUpgradeCmd(d Deps) *cobra.Command`, `Use: "upgrade"`, no flags. It loads the project with `locateProject` (no pin check: R-CFG-011 exempts it, so it runs on a pin two minors behind).
  - `func idlePoint(d Deps, p *project, cfg *config.Config) (release func(), err error)`: refuses (exit 4) outside the primary checkout, when HEAD is not `cfg.Base` (detached or another branch), with any uncommitted or untracked change, and when `gate.lock` or `integrate-<base>.lock` in the store dir is held; otherwise it holds both locks until `release`.
  - `type upgradeOpts struct{ restart bool }` and `func runUpgrade(d Deps, o upgradeOpts) error`. The command passes `restart: true`. The worker (Task 12) passes `false`, because a restart would kill the worker's own cgroup.
  - Lines, pinned:
    - `store schema A → B (backup <path>)` when the store migrated
    - `pin ness = "0.N"`
    - the sync report and the release notes (from `applyWithNotes`)
    - `committed chore(ness): upgrade to X.Y.Z`
    - `worker restarted` / `worker re-pointed to <bin>`
    - `open sessions get an update item from the worker at their next idle point`
    - `already at X.Y.Z` when nothing changed (exit 0, no commit)
    - on a sync conflict, stderr: `ness: sync stopped on conflicts: resolve them and commit (git add -A && git commit), then run ness upgrade again`

- [ ] **Step 1: Failing unit test**
```go
func TestLockPath(t *testing.T) {
	p := filepath.Join(t.TempDir(), "install.lock")
	release, err := worker.LockPath(p)
	if err != nil { t.Fatal(err) }
	if _, err := worker.LockPath(p); !errors.Is(err, worker.ErrLocked) { t.Fatalf("second = %v, want ErrLocked", err) }
	release()
	again, err := worker.LockPath(p)
	if err != nil { t.Fatalf("after release: %v", err) }
	again()
	if _, err := os.Stat(p); err != nil { t.Fatalf("lock file removed: %v", err) }
}
```
  The existing `Lock` tests stay green unchanged.

- [ ] **Step 2 (judge: written by a separate subagent).**
  - `upgrade.txtar`, `# covers: R-UPD-061, R-UPD-062, R-UPD-063, R-CFG-010, R-CFG-011`:
    - Init under `NESS_TEST_VERSION=0.1.0` and commit.
    - Then `env NESS_TEST_VERSION=0.2.0` and `NESS_TEST_NOTES=notes.md` (a 0.2.0 section with `### Running sessions` `* merge the base branch`) → `exec ness upgrade`.
    - Expect `grep '^ness = "0.2"' .ness/ness.toml`, `grep '"ness": "0.2.0"' .ness/lock.json`, and `git log -1 --format=%s` → `stdout '^chore\(ness\): upgrade to 0\.2\.0$'`.
    - `exec git status --porcelain` → `! stdout .`. `stdout 'ness 0\.2\.0 release notes'` and `stdout 'open sessions get an update item'`. No push happened: the fake exec log has no `git push`.
    - With a fake unit already pointing at this binary, the fake `systemctl` calls include `--user restart ness-repo.service`; with a unit pointing at `/old/ness`, the unit is rewritten (re-pointed) and `stdout 'worker re-pointed to '`.
    - A second `exec ness upgrade` → `stdout '^already at 0\.2\.0$'`, and the commit count is unchanged.
    - Two minors behind: a fresh project pinned `0.1` with lock 0.1.0 under `NESS_TEST_VERSION=0.3.0` → `exec ness upgrade` succeeds and the pin becomes `"0.3"`.
    - A sync conflict (a planted unrecorded edit of a synced file, committed so the base is clean) → `exits 3`, no new commit, `stderr 'resolve them and commit \(git add -A && git commit\), then run ness upgrade again'`. After `git add -A && git commit -qm resolve`, `exec ness upgrade` succeeds or prints `already at`.
  - `upgrade_refusals.txtar`, `# covers: R-UPD-060` (and Review focus 5): each case gives `exits 4`, no commit, the pin unchanged, and a stderr naming the cause:
    - an uncommitted file (`uncommitted`) and an untracked file
    - `lock .git/ness/gate.lock` (`an integrate holds the gate lock`)
    - `lock .git/ness/integrate-main.lock`
    - run from a session worktree (`run ness upgrade in the base checkout <path>`)
    - `git checkout -q -b other` (`HEAD is on other, not main`)
    - `git checkout -q --detach` (`HEAD is detached`)
    - a lock recording `0.3.0` under binary 0.2.0 (`newer than this binary`)

- [ ] **Step 3: Run** `go test ./internal/worker/ -run TestLockPath -count=1` and `go test ./internal/cli/ -run 'TestScripts/upgrade' -count=1`. Expected: FAIL.

- [ ] **Step 4: Implement** `LockPath`, then `runUpgrade` in this order:
  1. `locateProject` → `primaryRoot` → `resolvedConfig`.
  2. `release, err := idlePoint(d, p, cfg)`; `defer release()`.
  3. The lock version must not be newer than `d.Version` (`Compare`).
  4. `OpenReadOnly` the schema → `store.Open` → print the migration line when the schema changed.
  5. `SetPin` when it differs.
  6. Reload the project → `runSync`. On a conflict, return exit 3 with the conflict message.
  7. `git add -A` → `git commit -m` only when `git status --porcelain` is non-empty.
  8. With `restart`: `repointWorker`; when it did not change the unit (a changed unit is restarted by `sup.Install` itself), `restartWorker`.
  9. Print the final line. Never `git push`.

- [ ] **Step 5: Run** `go test ./internal/worker/ -count=1 && go test ./internal/cli/ -run 'TestScripts/upgrade' -count=1`, then refresh the reference with `-update`. Expected: PASS.

- [ ] **Step 6: Commit** `git commit -m "feat(cli): ness upgrade migrates, syncs, bumps the pin and commits the base checkout"`

---

### Task 11: Worker — the live-session update item and the behind-base line

**Files:**
- Create: `internal/worker/update.go`, `internal/worker/update_test.go`, `internal/cli/stale.go`, `internal/sync/skills.go`, `internal/sync/skills_test.go`
- Modify: `internal/worker/worker.go` (`Worker.Stale`, a call in `visit` when idle), `internal/worker/fakes_test.go` (`bench.restart`), `internal/cli/worker.go` (wire `Stale`), `internal/cli/hook.go` (`sessionStart` behind-base line)
- Test (judge): Create `internal/cli/testdata/script/worker_update_item.txtar`

**Interfaces:**
- Consumes: `nsync.ParseLock`, `nsync.Notes`/`Between`/`RunningSessions`/`Compare` (Task 1), `store.Send`
- Produces:
  - `func ChangedSkills(from, to *Lock) []string` in package `sync`: the sorted skill directory names (`.agents/skills/<name>/…`) whose entry hashes differ, or that exist on one side only.
  - `type Stale struct{ From, To, Base string; Notes []string; Skills []string }`, where `Notes` holds one `ness V: <running sessions>` per version, oldest first.
  - `func (s Stale) Body() string`, pinned:
    ```
    ness update: this worktree's ness files are <From>; <Base> has <To>.
    At your next stopping point:
    1. git merge <Base>
    2. ness sync --check (expect exit 0)
    3. re-load these skills: <a, b> (none: skip)
    Running sessions notes:
    <Notes joined by \n, or "none">
    ```
  - `Worker.Stale func(sess store.Session) (Stale, bool, error)`. A nil value means the check is off.
  - `func (w *Worker) deliverUpdate(sess store.Session, now time.Time)`:
    - `Send(sess.ID, sess.ID, IntentRequest, st.Body(), false, now)` (self-addressed, like a probe wake).
    - For a non-root, it also sends `Send(sess.ID, root, IntentFYI, "ness update copy for "+sess.ID+":\n"+st.Body(), false, now)`, where root is walked up `Parent` through `w.open`.
    - Errors are logged once per text under key `sess.ID+" update"`.
  - `func staleLock(d Deps, primary string, sess store.Session) (worker.Stale, bool, error)` reads `<primary>/<sess.Worktree>/.ness/lock.json` and `git show <sess.Base>:.ness/lock.json`, compares their `Ness` with `Compare`, and builds the notes from the running binary's CHANGELOG. `sess.Base` is the configured base for a root and the parent's branch for a child (open.go:477), so updates cascade down the tree. A session with an empty `Worktree` or a missing worktree lock returns `false` with an error.
  - Session-start line, pinned: `ness: this worktree's ness files are <From> and <Base> has <To>: merge <Base>, run ness sync --check (expect exit 0), re-load the changed skills (your inbox has the list)`.
  - Test helper `func (b *bench) restart()` in `fakes_test.go`: replaces `b.w` with a fresh `worker.Worker` built as in `newBench` over the same store and log (no in-memory state), keeping `Stale` and `Update`.

- [ ] **Step 1 (judge: written by a separate subagent): `worker_update_item.txtar`**, `# covers: R-UPD-070, R-UPD-072`. `env NESS_TEST_VERSION=0.2.0` for the whole script.
  - `ness init` on main (pin 0.2, lock 0.2.0) and commit. Rewrite the lock's `"ness"` to `"0.1.0"` with `sed` and commit (the old state).
  - Register a **root** session the way `worker_run.txtar` does (its `store add-launch` and fake herdr pane), with `worktree=.worktrees/r-1`, base `main`, and `git worktree add -q .worktrees/r-1 -b session/r-1` from this commit.
  - On main, restore `"ness": "0.2.0"` in the lock and commit (main now ahead).
  - `exec ness worker run --once` with the pane idle → `store items s-r 1`, and `store item <id> sender=s-r intent=request` with body lines `ness update: this worktree's ness files are 0.1.0; main has 0.2.0.` and `2. ness sync --check (expect exit 0)`.
  - A second `--once` → `store items s-r 1`.
  - `cd .worktrees/r-1`, `env NESS_SESSION=s-r`, `exec ness hook session-start` → `stdout '^ness: this worktree.s ness files are 0\.1\.0 and main has 0\.2\.0'`.
  - `exec git merge -q main` in the worktree → `exec ness hook session-start` → `! stdout 'ness files are'`, and a third `--once` adds no item (`store items s-r 1`).

- [ ] **Step 2: Failing unit tests** (`package worker_test`, with the `bench` of `fakes_test.go`: `newBench`, `b.session(id, parent, client)`, `b.pane(id)`, `b.pass()`, `b.listed(recipient)`, `b.logCount(sub)`, plus the new `b.restart()`)
```go
func TestUpdateItemAtIdle(t *testing.T) {
	// covers: R-UPD-070, R-UPD-071
	b := newBench(t)
	b.session("s-r", store.Operator, "claude")
	b.session("s-c", "s-r", "claude")
	stale := worker.Stale{From: "0.1.0", To: "0.2.0", Base: "session/x-s-r", Notes: []string{"ness 0.2.0: * merge"}, Skills: []string{"ness-entry"}}
	b.w.Stale = func(s store.Session) (worker.Stale, bool, error) { return stale, s.ID == "s-c", nil }
	b.pane("s-c").status = "working"
	b.pass()
	if n := len(b.listed("s-c")); n != 0 { t.Fatalf("busy pane got %d items", n) }
	b.pane("s-c").status = "idle"
	b.pass()
	got := b.listed("s-c")
	if len(got) != 1 || got[0].Sender != "s-c" || got[0].Intent != store.IntentRequest || got[0].Body != stale.Body() { t.Fatalf("%+v", got) }
	fyi := b.listed("s-r")
	if len(fyi) != 1 || fyi[0].Intent != store.IntentFYI || !strings.Contains(fyi[0].Body, stale.Body()) { t.Fatalf("root fyi %+v", fyi) }
}
func TestUpdateItemOncePerPair(t *testing.T) { // Review focus 4
	b := newBench(t)
	b.session("s-r", store.Operator, "claude") // a root: no fyi to itself
	pair := worker.Stale{From: "0.1.0", To: "0.2.0", Base: "main"}
	b.w.Stale = func(store.Session) (worker.Stale, bool, error) { return pair, true, nil }
	b.pass(); b.pass()
	b.restart()
	b.pass()
	if n := len(b.listed("s-r")); n != 1 { t.Fatalf("items = %d after restart, want 1", n) }
	pair.To = "0.3.0"
	b.pass()
	if n := len(b.listed("s-r")); n != 2 { t.Fatalf("items = %d after a base hop, want 2", n) }
	b.w.Stale = func(store.Session) (worker.Stale, bool, error) { return worker.Stale{}, false, errors.New("no lock") }
	for range 3 { b.pass() }
	if n := b.logCount("no lock"); n != 1 { t.Fatalf("logged %d times, want once", n) }
}
func TestStaleBody(t *testing.T) {
	got := worker.Stale{From: "0.1.0", To: "0.2.0", Base: "main"}.Body()
	want := "ness update: this worktree's ness files are 0.1.0; main has 0.2.0.\nAt your next stopping point:\n1. git merge main\n2. ness sync --check (expect exit 0)\n3. re-load these skills: none: skip\nRunning sessions notes:\nnone"
	if got != want { t.Fatalf("Body =\n%s\nwant\n%s", got, want) }
	full := worker.Stale{From: "0.1.0", To: "0.3.0", Base: "main", Skills: []string{"a", "b"}, Notes: []string{"ness 0.2.0: x", "ness 0.3.0: y"}}.Body()
	for _, s := range []string{"3. re-load these skills: a, b\n", "Running sessions notes:\nness 0.2.0: x\nness 0.3.0: y"} {
		if !strings.Contains(full, s) { t.Errorf("Body lacks %q:\n%s", s, full) }
	}
}
func TestChangedSkills(t *testing.T) {
	a := &nsync.Lock{Entries: []nsync.LockEntry{{Path: ".agents/skills/ness-entry/SKILL.md", Hash: "1"}, {Path: ".agents/skills/ness-launch/SKILL.md", Hash: "1"}, {Path: ".agents/skills/ness-gone/SKILL.md", Hash: "1"}, {Path: "AGENTS.md", Hash: "1"}}}
	b := &nsync.Lock{Entries: []nsync.LockEntry{{Path: ".agents/skills/ness-entry/SKILL.md", Hash: "2"}, {Path: ".agents/skills/ness-launch/SKILL.md", Hash: "1"}, {Path: ".agents/skills/ness-new/SKILL.md", Hash: "1"}, {Path: "AGENTS.md", Hash: "2"}}}
	if got := nsync.ChangedSkills(a, b); !slices.Equal(got, []string{"ness-entry", "ness-gone", "ness-new"}) { t.Fatal(got) }
}
```

- [ ] **Step 3: Run** `go test ./internal/worker/ ./internal/sync/ -run 'TestUpdateItem|TestStaleBody|TestChangedSkills' -count=1`. Expected: FAIL.

- [ ] **Step 4: Implement.**
  - `visit` calls `w.deliverUpdate(sess, now)` right after the idle check, before `observe`. The hint that follows (R-WRK-010) wakes the session.
  - The idempotent item key (R-MSG-014) makes "once per pair" hold across restarts, as long as the body is the same for the pair (it is built only from the pair's versions, the CHANGELOG and the two locks).
  - `staleLock` caches the base lock per `(Base, git rev-parse Base)` inside the closure (planner call: one `git show` per base tip, not per session per pass).
  - `sessionStart` prints the behind-base line for every session.

- [ ] **Step 5: Run** `go test ./internal/worker/ ./internal/sync/ -race -count=1 && go test ./internal/cli/ -run 'TestScripts/(worker_|hook_session)' -count=1`. Expected: PASS.

- [ ] **Step 6: Commit** `git commit -m "feat(worker): one update item per live session whose worktree is behind its base, with an fyi to the root"`

---

### Task 12: Worker — daily check, unattended install and upgrade

**Files:**
- Create: `internal/cli/unattended.go`
- Modify: `internal/worker/worker.go` (`Worker.Update func(now time.Time) error`, called at the end of `Pass`), `internal/worker/worker_test.go`, `internal/cli/worker.go` (wire it), `internal/cli/selfupdate.go` (`installVersion` shared, from Task 8), `internal/cli/upgrade.go` (`idlePoint`, `runUpgrade` shared, from Task 10)
- Test (judge): Create `internal/cli/testdata/script/worker_auto.txtar`

**Interfaces:**
- Consumes: `update.Decide`/`Refresh`/`RecordFailure`/`ClearFailure`/`ManagedBy`/`Ceiling` (Task 5), `worker.LockPath` (Task 10), `idlePoint(d, p, cfg) (release func(), err error)` and `runUpgrade(d, upgradeOpts{restart: false})` (Task 10), `installVersion(d Deps, rel update.Release) (prev string, err error)` and `machinePins` (Task 8), `refreshCache` (Task 9)
- Produces:
  - `Worker.Update func(now time.Time) error`. Wired whenever the project loads (every `auto` mode: the behind fyi applies to all). An error is logged once per text under key `update` (in memory, per worker process).
  - `func unattended(d Deps, env workerEnv, s *store.Store) func(now time.Time) error`. Each call:
    1. `refreshCache(d, now)` when `cfg.Update.Check`, whatever `auto` is (R-UPD-050); `auto = "off"` only hides the result in doctor and session-start.
    2. Build `update.Inputs`: `Installed = d.Version`; `Offer = cache.Offer(cfg.Update.Channel, d.Version)`; `Auto = cfg.Update.Auto`; `Autos` from `ReadHead` of each registered root; `Managed` from `ManagedBy(d.Executable, Offer, …)`; `Ceiling = update.Ceiling(Offer, machinePins(d))` when `Offer != ""`; `Lock` from the base checkout's `.ness/lock.json`; `Idle` = `idlePoint` succeeds (its locks released at once). A dirty base or a held lock is just `Idle = false`: no record, no log.
    3. `Plan.Install` → `worker.LockPath(<CacheDir>/install.lock)`. Held (`ErrLocked`): skip without error. Otherwise one `Releases` call, `Resolve(rels, channel, Plan.Install)`, `installVersion`.
    4. `Plan.Upgrade` → `runUpgrade(..., restart: false)`. An error here is after the idle point: `RecordFailure(failurePath, {Version: d.Version, Cause: <first line of err>})`; return the error only when `changed` (so the log line appears once per cause, across worker processes). On success, `ClearFailure`.
    5. `Plan.Behind` → to each open root, `Send(root.ID, root.ID, IntentFYI, "ness update: this project's ness files are <Lock>, the binary is <Installed>: run ness upgrade in the base checkout", false, now)` (idempotent per pair through the item key).
    6. `Plan.Note` → returned as an error, so it is logged once per worker process.
    `ponytail:` comment: the veto reads each registered project's `ness.toml` on every pass.

- [ ] **Step 1 (judge: written by a separate subagent): `worker_auto.txtar`**, `# covers: R-UPD-050, R-UPD-080, R-UPD-081, R-UPD-082`.
  - **Daily check under notify:** `auto = "notify"`, `releases 0.2.0`, no cache → `exec ness worker run --once` → `$HOME/.cache/ness/update.json` holds `"stable": "0.2.0"`; the stub is unchanged.
  - **Install:** the stub binary `NESS_TEST_EXECUTABLE`, one registered project with `auto = "install"`, `releases 0.2.0`. `exec ness worker run --once` → `exec $WORK/bin/ness --version` shows 0.2.0, and `.prev` holds 0.1.0.
  - **Install lock held:** reset the stub, `lock $HOME/.cache/ness/install.lock` → `--once` → the stub is unchanged, and `worker.log` has no error line for it.
  - **Veto:** register a second project with `auto = "notify"` and reset the stub. `--once` → the stub is unchanged, and `grep 'auto-install held: .*has auto = "notify"' .git/ness/worker.log` matches.
  - **Upgrade:** `NESS_TEST_VERSION=0.2.0`, the lock at 0.1.0, `auto = "install"`, the cache primed (`update.json` with `stable 0.2.0`). `--once` → `git log -1 --format=%s` → `chore(ness): upgrade to 0.2.0`, and `upgrade-failed.json` is absent.
  - **Not idle:** the lock back at 0.1.0 (committed), an untracked file in the base → `--once` → no new commit, `! exists .git/ness/upgrade-failed.json`, and no `upgrade` line in `worker.log`. The same with `lock .git/ness/gate.lock`.
  - **Failure after the idle point:** a clean base with a sync conflict (a committed, unrecorded edit of a synced file) → `--once` twice → no commit, `upgrade-failed.json` holds `sync stopped on conflicts`, `worker.log` has exactly one line naming it (the second process sees the same cause), and the root's session-start shows `automatic upgrade to 0.2.0 failed`. Resolve and commit, run `--once` → a commit, and the failure file is gone.
  - **Behind:** `auto = "notify"`, binary 0.2.0, lock 0.1.0, an open root (`store add-session s-r r`) → `store items s-r 1`, and its body has `run ness upgrade in the base checkout`. A second `--once` → still `store items s-r 1`. With `auto = "off"` the same fyi is sent (settled point 11).

- [ ] **Step 2: Failing unit test**
```go
func TestPassRunsUpdateAndLogsOnce(t *testing.T) {
	b := newBench(t)
	calls := 0
	b.w.Update = func(time.Time) error { calls++; return errors.New("auto-install held: x") }
	for range 3 { b.pass() }
	if calls != 3 { t.Fatalf("Update ran %d times, want each pass", calls) }
	if n := b.logCount("auto-install held: x"); n != 1 { t.Fatalf("logged %d", n) }
}
```

- [ ] **Step 3: Run** `go test ./internal/worker/ -run TestPassRunsUpdate -count=1` and `go test ./internal/cli/ -run 'TestScripts/worker_auto' -count=1`. Expected: FAIL.

- [ ] **Step 4: Implement** as specified. `Pass` calls `w.Update(now)` after the sessions loop.
  A managed binary sets `Inputs.Managed`; Decide's note is logged once per worker process,
  which is how "degrades to notify, said once" is met.

- [ ] **Step 5: Run** `go test ./internal/worker/ ./internal/update/ -race -count=1 && go test ./internal/cli/ -run 'TestScripts/worker_' -count=1`. Expected: PASS.

- [ ] **Step 6: Commit** `git commit -m "feat(worker): daily check, unattended install under the machine veto and upgrade at the idle point, failures surfaced"`

---

### Task 13: `install.sh`

**Files:**
- Create: `install.sh` (repository root, POSIX sh, `set -eu`), `install_test.go` (package `ness_test` at the repository root)

**Interfaces:**
- Consumes: `testkit.ReleaseServer` (Task 6; it lists newest first, as GitHub does)
- Produces:
  - Usage: `install.sh [--channel stable|rc]`, with env `NESS_VERSION`, `NESS_INSTALL_DIR`, `NESS_RELEASES_URL`.
  - Messages, pinned:
    - `ness X.Y.Z installed to DIR/ness`
    - `note: DIR is not on your PATH; add it: export PATH="DIR:$PATH"`
    - `install.sh: no ness build for OS/ARCH; build from source: go install github.com/enea-scaccabarozzi/ness/cmd/ness@latest`
    - `install.sh: checksum mismatch for NAME`

- [ ] **Step 1 (judge: written by a separate subagent from the implementer): failing tests.** Each one runs `sh install.sh` with `PATH` = a temp dir of shims plus `/usr/bin:/bin`, and `HOME` = a temp dir. Helpers (`t.Helper()`): `runInstall(t, url, dir string, args, env []string) (string, error)` sets `NESS_RELEASES_URL` and `NESS_INSTALL_DIR=dir`; `versionOf(t, bin)` runs `bin --version` and returns the last field; `shim(t, shimDir, name, script)` writes an executable sh script; `entries(t, dir) int`.
```go
func TestInstallSh(t *testing.T) {
	// covers: R-UPD-093
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "0.1.0"}, testkit.FakeRelease{Version: "0.3.0-rc.1", Prerelease: true}, testkit.FakeRelease{Version: "0.2.0"})
	for _, c := range []struct{ name string; args, env []string; want string }{
		{"latest stable, not the newer rc", nil, nil, "0.2.0"},
		{"pinned", nil, []string{"NESS_VERSION=0.1.0"}, "0.1.0"},
		{"pinned with v", nil, []string{"NESS_VERSION=v0.1.0"}, "0.1.0"},
		{"rc", []string{"--channel", "rc"}, nil, "0.3.0-rc.1"},
	} {
		dir := t.TempDir()
		out, err := runInstall(t, srv.URL, dir, c.args, c.env)
		if err != nil { t.Fatalf("%s: %v\n%s", c.name, err, out) }
		if v := versionOf(t, filepath.Join(dir, "ness")); v != c.want { t.Errorf("%s: installed %s, want %s", c.name, v, c.want) }
		if !strings.Contains(out, "ness "+c.want+" installed to "+dir+"/ness") { t.Errorf("%s: no installed line:\n%s", c.name, out) }
		if !strings.Contains(out, "is not on your PATH") { t.Errorf("%s: no PATH note:\n%s", c.name, out) }
	}
}
func TestInstallShChecksumMismatch(t *testing.T) {
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "0.2.0", BadSum: true})
	dir := t.TempDir()
	out, err := runInstall(t, srv.URL, dir, nil, nil)
	if err == nil || !strings.Contains(out, "install.sh: checksum mismatch for ness_0.2.0_linux_amd64.tar.gz") { t.Fatalf("%v\n%s", err, out) }
	if n := entries(t, dir); n != 0 { t.Fatalf("install dir has %d entries, want none", n) }
}
func TestInstallShUnknownArch(t *testing.T) {
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "0.2.0"})
	dir := t.TempDir()
	out, err := runInstall(t, srv.URL, dir, nil, []string{"SHIM_UNAME=riscv64"}) // uname shim: -m prints $SHIM_UNAME, -s prints Linux
	if err == nil || !strings.Contains(out, "install.sh: no ness build for linux/riscv64; build from source: go install github.com/enea-scaccabarozzi/ness/cmd/ness@latest") { t.Fatalf("%v\n%s", err, out) }
	if n := entries(t, dir); n != 0 { t.Fatalf("install dir has %d entries", n) }
}
func TestInstallShOnPath(t *testing.T) {
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "0.2.0"})
	dir := t.TempDir()
	out, err := runInstall(t, srv.URL, dir, nil, []string{"PATH_PREFIX=" + dir}) // the helper puts dir first on PATH
	if err != nil || strings.Contains(out, "is not on your PATH") { t.Fatalf("%v\n%s", err, out) }
}
func TestInstallShWget(t *testing.T) {
	if _, err := exec.LookPath("wget"); err != nil { t.Skip("no wget on this system") }
	srv := testkit.ReleaseServer(t, testkit.FakeRelease{Version: "0.2.0"})
	dir := t.TempDir()
	out, err := runInstall(t, srv.URL, dir, nil, []string{"SHIM_NO_CURL=1"}) // the helper adds a curl shim that exits 127
	if err != nil || versionOf(t, filepath.Join(dir, "ness")) != "0.2.0" { t.Fatalf("%v\n%s", err, out) }
}
```
  `runInstall` runs `uname`, `curl` and `wget` through the shim dir only when a test asks for
  a shim; otherwise the real tools on `/usr/bin:/bin` run.

- [ ] **Step 2: Run** `go test . -run TestInstallSh -count=1`. Expected: FAIL (no `install.sh`).

- [ ] **Step 3: Implement.**
  - OS: `uname -s` to lower case. Arch: `x86_64|amd64→amd64`, `aarch64|arm64→arm64`, anything else is the error.
  - Fetch `${NESS_RELEASES_URL:-https://api.github.com/repos/enea-scaccabarozzi/ness}/releases?per_page=100` with `curl -fsSL`, falling back to `wget -qO-` when curl is missing or fails to start.
  - Pick the version: `NESS_VERSION` (a leading `v` stripped), else the first `tag_name` whose release has `"prerelease": false`, or the first of any for rc. The list is newest first. To get the order without a JSON parser, split the objects on `"tag_name"`.
  - Pull the `browser_download_url` lines for the archive and `checksums.txt` out with `grep -o`.
  - Download into `mktemp -d` (removed by `trap`), verify with `sha256sum -c` or `shasum -a 256 -c` on the single matching line, then `tar -xzf` and `install -m 0755` (or `cp` + `chmod`) into `$NESS_INSTALL_DIR`, which defaults to `$HOME/.local/bin` and is created with `mkdir -p` only after the checksum passed.

- [ ] **Step 4: Run** `go test . -count=1` and `sh -n install.sh`. Expected: PASS. Also run `shellcheck -s sh install.sh` when it is installed (planner call: not a gate dependency).

- [ ] **Step 5: Commit** `git commit -m "feat: install.sh: checksum-verified install from the GitHub releases"`

---

### Task 14: Release workflows and the releasing checklist

**Files:**
- Create: `.github/workflows/rc.yml`, `.github/scripts/release-check.sh`, `internal/testkit/release_check_test.go`, `docs/guides/releasing.md`
- Modify: `.github/workflows/release-please.yml`, `.goreleaser.yaml` (`release: prerelease: auto`, keeping `mode: keep-existing`), `internal/testkit/release_test.go` (drop the `release.yml` assertion), `docs/guides/meta.json` (add `releasing`), `docs/spec/reviews.md` (the R-UPD-091 entry)
- Delete: `.github/workflows/release.yml`

**Interfaces:**
- Produces:
  - `release-check.sh stable VERSION` exits 1, checking in this order, with:
    - `no v<VERSION>-rc.* tag: cut a release candidate with the rc workflow and run its field window first` when no rc tag exists
    - `no v<VERSION>-rc.* tag has HEAD's tree: cut a new rc` when no rc tag's `^{tree}` equals `HEAD^{tree}`
    - `no upgrade fixture for <prev>: run scripts/upgrade-fixture.sh <prev>` when a previous stable tag exists without `internal/cli/testdata/upgrade/<prev>/`
    - `no sessions: commit since <tag>` (counting from `git describe --tags --abbrev=0 --exclude '*-rc.*'`; `docs(sessions):` record commits do not count)
  - `release-check.sh rc VERSION` prints the next tag `vVERSION-rc.N`. It exits 1 when `CHANGELOG.md` has no `## [VERSION]` section.
  - `rc.yml`: `workflow_dispatch` only, with input `ref` (default `release-please--branches--main`). It checks out the ref with `fetch-depth: 0`, sets up Go and the tools the way `ci.yml` does (`jdx/mise-action`, same pinned version), then:
    1. `ver=$(jq -r '."."' .release-please-manifest.json)`
    2. `tag=$(sh .github/scripts/release-check.sh rc "$ver")`
    3. `go test ./internal/sync/`
    4. `git tag "$tag" && git push origin "$tag"`
    5. `goreleaser release --clean` with `GITHUB_TOKEN`
  - `release-please.yml`: the "Require a sessions commit" step becomes `sh .github/scripts/release-check.sh stable "$(jq -r '."."' .release-please-manifest.json)"`, guarded by the same `if:`.

- [ ] **Step 1 (judge: written by a separate subagent from the implementer): failing tests** (a scratch repo in `t.TempDir()` with commits and tags made by `git`; helpers with `t.Helper()`: `scratchRepo`, `commit(t, repo, msg)`, `gitTag`, `mkdirAll`, `writeChangelog`, `releaseCheck(t, repo, args...) (string, error)` running the script from `ModuleRoot` with `repo` as its working directory)
```go
func TestReleaseCheckStable(t *testing.T) {
	// covers: R-UPD-092
	repo := scratchRepo(t) // git init; commit "feat: x"; tag v0.1.0; commit "sessions: nothing to do"
	if out, err := releaseCheck(t, repo, "stable", "0.2.0"); err == nil || !strings.Contains(out, "no v0.2.0-rc.* tag: cut a release candidate") { t.Fatal(out) }
	gitTag(t, repo, "v0.2.0-rc.1")
	if out, err := releaseCheck(t, repo, "stable", "0.2.0"); err == nil || !strings.Contains(out, "no upgrade fixture for v0.1.0") { t.Fatal(out) }
	mkdirAll(t, repo, "internal/cli/testdata/upgrade/v0.1.0") // untracked: the tree is unchanged
	if out, err := releaseCheck(t, repo, "stable", "0.2.0"); err != nil { t.Fatal(out) } // the rc tag does not hide the sessions commit
}
func TestReleaseCheckStableTreeMoved(t *testing.T) {
	// covers: R-UPD-092
	repo := scratchRepo(t); mkdirAll(t, repo, "internal/cli/testdata/upgrade/v0.1.0")
	gitTag(t, repo, "v0.2.0-rc.1")
	commit(t, repo, "fix: y")
	if out, err := releaseCheck(t, repo, "stable", "0.2.0"); err == nil || !strings.Contains(out, "no v0.2.0-rc.* tag has HEAD's tree: cut a new rc") { t.Fatal(out) }
	gitTag(t, repo, "v0.2.0-rc.2")
	if out, err := releaseCheck(t, repo, "stable", "0.2.0"); err != nil { t.Fatal(out) }
}
func TestReleaseCheckNoSessionsCommit(t *testing.T) {
	repo := t.TempDir(); gitInit(t, repo); commit(t, repo, "feat: x"); gitTag(t, repo, "v0.1.0")
	commit(t, repo, "feat: y"); commit(t, repo, "docs(sessions): update-path plan")
	mkdirAll(t, repo, "internal/cli/testdata/upgrade/v0.1.0"); gitTag(t, repo, "v0.2.0-rc.1")
	if out, err := releaseCheck(t, repo, "stable", "0.2.0"); err == nil || !strings.Contains(out, "no sessions: commit since v0.1.0") { t.Fatal(out) }
}
func TestReleaseCheckRC(t *testing.T) {
	// covers: R-UPD-091
	repo := scratchRepo(t)
	if out, err := releaseCheck(t, repo, "rc", "0.2.0"); err == nil { t.Fatalf("no changelog section accepted: %s", out) }
	writeChangelog(t, repo, "## [0.2.0] (2026-11-01)\n### Running sessions\n* x\n")
	if out, err := releaseCheck(t, repo, "rc", "0.2.0"); err != nil || strings.TrimSpace(out) != "v0.2.0-rc.1" { t.Fatal(out, err) }
	gitTag(t, repo, "v0.2.0-rc.1")
	if out, err := releaseCheck(t, repo, "rc", "0.2.0"); err != nil || strings.TrimSpace(out) != "v0.2.0-rc.2" { t.Fatal(out, err) }
}
func TestOneGoreleaserPerTag(t *testing.T) {
	// covers: R-UPD-090, R-QA-004
	root := ModuleRoot(t)
	if _, err := os.Stat(filepath.Join(root, ".github/workflows/release.yml")); !errors.Is(err, fs.ErrNotExist) { t.Fatalf("release.yml still exists: %v", err) }
	files, err := filepath.Glob(filepath.Join(root, ".github/workflows/*.yml"))
	if err != nil { t.Fatal(err) }
	var runners []string
	for _, f := range files {
		if strings.Contains(read(t, f), "goreleaser release") { runners = append(runners, filepath.Base(f)) }
	}
	if !slices.Equal(runners, []string{"rc.yml", "release-please.yml"}) { t.Fatalf("goreleaser runs in %v", runners) }
	rc := read(t, filepath.Join(root, ".github/workflows/rc.yml"))
	if !strings.Contains(rc, "workflow_dispatch") || strings.Contains(rc, "push:") || strings.Contains(rc, "pull_request") || strings.Contains(rc, "schedule:") { t.Fatalf("rc.yml triggers:\n%s", rc) }
	if !strings.Contains(read(t, filepath.Join(root, ".goreleaser.yaml")), "prerelease: auto") { t.Fatal(".goreleaser.yaml lacks prerelease: auto") }
}
```

- [ ] **Step 2: Run** `go test ./internal/testkit/ -run 'TestReleaseCheck|TestOneGoreleaser|TestGoreleaser' -count=1`. Expected: FAIL.

- [ ] **Step 3: Implement** the script and the workflows as specified. A fresh `ness-review`
  subagent then reads `rc.yml` against R-UPD-091 and appends
  `- R-UPD-091 | <reviewer session> | <date> | <commit>` to `docs/spec/reviews.md`.

- [ ] **Step 4: Write `docs/guides/releasing.md`** (and add it to `docs/guides/meta.json`):
  - Wait for the release PR.
  - Run the rc workflow (`gh workflow run rc.yml`).
  - Install the rc on the field projects: commit `[update] channel = "rc"` in each base checkout, run `ness self-update` once, then `ness upgrade` in each.
  - Field window. Promotion criteria (R-UPD-095): false stalls < 10%, worker notices < 20% of phone posts, 0 lost gate runs, 0 inbox false positives, 0 pick hangs, 0 sessions that missed the update item (every session open at upgrade time has its update request disposed or has merged its base). Hooks failing open are covered by the `hook_skew` testscripts.
  - Main moved: a new rc (the stable job refuses without an rc of HEAD's tree).
  - Merge the release PR.
  - `scripts/upgrade-fixture.sh vX.Y.Z` and commit the fixture; bump the `upgrade_previous` script's fixture and `NESS_TEST_VERSION` in the same commit.
  - The commit-type rule: session records use `docs(sessions): <scope> …`; `sessions:` is reserved for Running-sessions notes, which reach live agents in the update item.

- [ ] **Step 5: Run** `go test ./internal/testkit/ -count=1`. Expected: PASS.

- [ ] **Step 6: Commit** `git commit -m "ci(release): one goreleaser per tag, rc from the release PR, stable refused without an rc of HEAD's tree"`

---

### Task 15: The v0.1.0 upgrade fixture and the CI upgrade test

**Files:**
- Create: `scripts/upgrade-fixture.sh`, `internal/cli/testdata/upgrade/v0.1.0/tree.txtar`, `internal/cli/testdata/upgrade/v0.1.0/state.sql`
- Modify: `internal/testkit/testkit.go` (`Cmds` gains `upgradefixture` and `patchrule`). Create `internal/testkit/upgrade.go`, `internal/testkit/upgrade_test.go`
- Test (judge): Create `internal/cli/testdata/script/upgrade_previous.txtar`

**Interfaces:**
- Produces:
  - `scripts/upgrade-fixture.sh TAG`. The first fixture is `sh scripts/upgrade-fixture.sh v0.1.0` (the public release on `a7903ca`). It works in this order:
    1. `git worktree add` TAG into a `mktemp -d` dir and build ness there with `-ldflags "-X main.version=<TAG without v>"`.
    2. In a scratch `git init` repo with `HOME`, `XDG_CONFIG_HOME` and `XDG_CACHE_HOME` under the temp dir: `ness init --gate true --base main --profile program --yes`, commit, then `ness inbox` to create the store.
    3. `python3 -I` with `sqlite3` inserts one open root `s-fixture` (size `program`, ready, worktree empty, base `main`) and writes `iterdump()` to `state.sql`.
    4. `txtar`-pack the tracked tree into `tree.txtar`.
    5. Remove the worktree and the temp dir.
  - Testscript command `upgradefixture latest|vX.Y.Z`. It unpacks the newest (or the named) fixture into `$WORK/repo`, commits it, and loads `state.sql` with raw `database/sql` (no migration) into `.git/ness/state.sqlite3`. It sets `FIXTURE_VERSION` and `FIXTURE_SCHEMA`.
  - `func PatchRule(manifest, fixtureVersion string, fixtureSchema, schema int) error`: an error `patch release <manifest> over <fixtureVersion> adds a store migration (schema <fixtureSchema> → <schema>)` when the two versions share major.minor, the manifest's patch is higher, and the schemas differ; nil otherwise.
  - Testscript command `patchrule` calls `PatchRule` with `.release-please-manifest.json`'s `"."` (from `ModuleRoot`), `$FIXTURE_VERSION`, `$FIXTURE_SCHEMA` and `store.Version()`.

- [ ] **Step 1: Failing unit test**
```go
func TestPatchRule(t *testing.T) {
	// covers: R-UPD-001
	for _, c := range []struct{ manifest, fixture string; fs, s int; bad bool }{
		{"0.1.1", "0.1.0", 12, 13, true},
		{"0.1.1", "0.1.0", 12, 12, false},
		{"0.2.0", "0.1.0", 12, 13, false},
		{"0.1.0", "0.1.0", 12, 13, false}, // the fixture's own release: no patch over it
	} {
		err := testkit.PatchRule(c.manifest, c.fixture, c.fs, c.s)
		if (err != nil) != c.bad { t.Errorf("%+v: %v", c, err) }
		if c.bad && !strings.Contains(err.Error(), "adds a store migration") { t.Errorf("%+v: %v", c, err) }
	}
}
```

- [ ] **Step 2 (judge: written by a separate subagent): `upgrade_previous.txtar`**, `# covers: R-UPD-001, R-UPD-002, R-UPD-004, R-UPD-094`. `env NESS_TEST_VERSION=0.2.0` for the whole script, against `upgradefixture v0.1.0`:
  - `patchrule` passes.
  - `env NESS_SESSION=s-fixture`, then `stdin benign.json` → `exec ness hook pre-tool`, `! stderr .`. The store now has the current schema. When `FIXTURE_SCHEMA` < current, `exists .git/ness/state.sqlite3.bak-v$FIXTURE_SCHEMA`; with this fixture both are 12, so `! exists .git/ness/state.sqlite3.bak-v12` (settled point 6).
  - `stdin rm.json` (a default guard's pattern) → `exits 2` (the guards still enforce).
  - `exec ness status`, `exec ness inbox`, and `exec ness worker run --once` (with the fake herdr of `worker_run.txtar`) each exit 0.
  - `exits 1 ness sync --check` and `! stdout conflict`: the lock's `ness` changes from 0.1.0 to 0.2.0, so a planned lock change always exists (R-SYNC-030), and an untouched fixture has no conflict.
  - The program check still fires: `exits 4 ness open --scope x --tier standard --brief b.md`, and `stderr 'no recorded autonomy mode'`.

- [ ] **Step 3: Run** `go test ./internal/testkit/ -run TestPatchRule -count=1` and `go test ./internal/cli/ -run 'TestScripts/upgrade_previous' -count=1`. Expected: FAIL (undefined `PatchRule`, unknown command `upgradefixture`).

- [ ] **Step 4: Implement** the script and the testkit commands. Build the fixture with
  `sh scripts/upgrade-fixture.sh v0.1.0`, outside any live repo (the script works only under
  `mktemp -d`). Commit the two fixture files. Check the `.txtar` for private terms with the
  leak scan (Task 17 Step 4).

- [ ] **Step 5: Run** `go test ./internal/cli/ -run 'TestScripts/upgrade_previous' -count=1` (expected: PASS), then `go test ./internal/testkit/ -run TestRequirementCoverage -count=1`. Expected: FAIL listing only R-UPD-003 and R-UPD-095, whose review entries land in Task 17; every other UPD id is covered.

- [ ] **Step 6: Commit** `git commit -m "test(upgrade): v0.1.0 fixture and the CI upgrade test against the previous release"`

---

### Task 16: Docs (every documented command run by hand)

**Files:**
- Modify: `docs/start/install.md` (install.sh default; `go install …@vX.Y.Z`, mise `github:enea-scaccabarozzi/ness`, the manual archive and checksum as options), `docs/guides/upgrade.md` (rewritten around `self-update` and `upgrade`, rollback with `ness.prev` and `bak-v<N>`, and what live sessions see), `docs/reference/config.md` (`[update]`), `docs/reference/environment.md` (`NESS_RELEASES_URL`, `NESS_INSTALL_DIR`, `NESS_VERSION`), `docs/reference/cli/*` (regenerated), `README.md` (the install line), `SECURITY.md` (supported versions: the latest stable plus the previous minor while R-UPD-002 holds), `docs/backlog.md` (close the "Update path" and "Binary and store skew" entries), `CONTRIBUTING.md` (the release flow and rc steps, linking `docs/guides/releasing.md`)
- Create: `docs/concepts/compatibility.md` (the four layers, N/N-1, the pin, the store backup, loud hooks, the update item, the veto and the ceiling), registered in `docs/concepts/meta.json`

- [ ] **Step 1: Write the pages** in the CONTRIBUTING "Writing style" voice. Every claim cites the rule id it rests on in the spec. `install.md` and `compatibility.md` say that only `ness self-update` applies the ceiling: `install.sh`, `go install` and mise skip it, and `ness doctor` and the loud hooks then report the skew (settled point 9).
- [ ] **Step 2: Run every documented command by hand.** Use a scratch `git init` repo under the
  scratchpad, with `HOME`, `XDG_CONFIG_HOME` and `XDG_CACHE_HOME` in the scratchpad, against a
  local release: run `goreleaser release --snapshot --clean`, write a `releases` JSON file into
  `dist/`, serve `dist/` with `python3 -m http.server`, and set
  `NESS_RELEASES_URL=http://127.0.0.1:<port>`. Paste each command's real output into the page
  where the page shows output. The commands to run:
  - `install.sh`, `install.sh --channel rc`
  - `ness self-update --check`, `ness self-update`, `ness self-update --version`
  - `ness upgrade` and its refusals
  - `ness doctor` (the `update` and `store` lines)
  - the rollback steps
  Task 18 re-runs install and upgrade against the real rc (planner call: the real release does not exist before the push).
- [ ] **Step 3: Run** `go test ./internal/cli/ -run TestReference -count=1 && go test ./profiles/ -count=1`, then build the site (`cd site && bun install && bun run build`, as `site.yml` does). Expected: PASS.
- [ ] **Step 4: Commit** `git commit -m "docs: install, upgrade, [update], self-update, upgrade and the compatibility page"`

---

### Task 17: Close-out gates and integration

- [ ] **Step 1: Running-sessions commit.** `git commit --allow-empty -m "sessions: when the ness update item arrives, merge the base branch, run ness sync --check, and re-load the skills it lists; a session whose hooks say they cannot run runs the single ness command they name"`
- [ ] **Step 2:** Run `/ness-compliance-check`, then `/ness-documentation`. Fix or own every finding.
- [ ] **Step 3:** Run a fresh `ness-review` of the whole branch against `specs.md` and this plan. Fix every finding at or above medium, or own it in the report.
- [ ] **Step 4: Leak scan, on the exact tree to be integrated and later pushed.** In the
  worktree `.private` is absent and the scan skips silently. So first run
  `mkdir -p .private && cp <main checkout>/.private/denylist.txt .private/denylist.txt`,
  then `go test ./profiles/ -run 'TestDenylistScan|TestPublicTreeSelection|TestPublicTreeDenylist' -count=1 -v`.
  Expected: exit 0 and `--- PASS: TestPublicTreeDenylist`. A `--- SKIP` means the list is
  missing: stop.
- [ ] **Step 5: Gate.** Run `make validate` and `make repeat`. Expected: exit 0. Do not pipe either into `tail` (memory: gate-exit-status).
- [ ] **Step 6:** `ness integrate --subject 'feat(update): update path: install.sh, self-update, upgrade, loud hooks, live-session items'`.
- [ ] **Step 7: Recap, then wait.** Write the operator a recap in chat (what landed, the gates' results, what the push and the rc will do, how to roll back) and file it with `ness send --to operator`. Push to origin only after the operator's go, re-running Step 4 on the pushed tree first, with `mise exec -- git push` and SSH keepalives (memory: gate-push-mise).

---

### Task 18: rc and live testing on the operator's three projects (manual, after the operator's go)

**Files:** private notes only, under `docs/discovery/update-path/` (git-excluded through
`.git/info/exclude`). Committed records (`report.md`) use only the anonymised names: research
lab, infrastructure lab, production backend.

- [ ] **Step 1: Ask before touching anything.** The Task 17 recap covers what will be installed
  and run on the three projects, in which order, and how to roll back. Wait for the operator's
  go: one batch only (batched-rollout ruling). No agent message counts as that go.
- [ ] **Step 2: Scratch run first.** If any task added a store migration (check
  `store.Version()` against v0.1.0's schema), run the rc binary in a scratch `git init` repo
  first and never in a live store (test-binary-store rule). This plan adds none, but check anyway.
- [ ] **Step 3: Cut `v0.2.0-rc.1`.** Wait for release-please's release PR. Run
  `gh workflow run rc.yml` and check that a pre-release `v0.2.0-rc.1` with the archives and
  `checksums.txt` exists (`gh release view v0.2.0-rc.1`).
- [ ] **Step 4: Back up the live stores.** Stop each worker (`systemctl --user stop ness-<project>`)
  and copy `.git/ness/state.sqlite3*` into `docs/discovery/update-path/backups/<anon>/`. Keep
  `~/.local/bin/ness` as `ness.pre-rc`.
- [ ] **Step 5: Install and upgrade.**
  - Run `ness self-update --channel rc` once on the machine and record the output privately. Check that the ceiling passes: all pins are 0.1, accepted by 0.2.
  - In each project's base checkout: set `[update] channel = "rc"` (an operator-approved edit) and **commit it** (`chore(ness): rc channel`), because `ness upgrade` refuses a dirty base. Then run `ness upgrade` and record the commit hash privately.
  - Check that every open session received exactly one update item (`ness status` and `ness inbox` as operator, read-only) and applied it (merge, `sync --check` 0).
  - Check that no hook failed open: `ness doctor` in every project, and the `worker.log` scan.
- [ ] **Step 6: Re-run** Task 16's install and upgrade commands against the real rc, and record any drift.
- [ ] **Step 7: Follow-up fixes.** Any fix (docs drift included) goes through a new session
  branch and a new `ness integrate`, Task 17's gates, the operator's go to push, and a new rc
  (`rc.2`, because of the same-tree rule).
- [ ] **Step 8: Field window.** Run it per `docs/guides/releasing.md`, then write the anonymised summary in `docs/sessions/update-path/report.md`, and stop for the operator's stable decision.

---

## Self-review

- **Coverage.**
  - Every spec section maps to a task:
    - §1 distribution: Tasks 6, 13, 14
    - §2 contract: Tasks 0, 1, 15
    - §3 store: Task 2
    - §4 hooks and unit: Tasks 3 and 7
    - §5 config, registry, ceiling, channel: Tasks 4, 5, 8
    - §6 self-update and check: Tasks 8, 9 and 12 (the worker's daily fetch)
    - §7 upgrade and live sessions: Tasks 10 and 11
    - §8 gates: Tasks 14, 15, 17
    - live testing: Task 18
    - docs: Task 16
  - Builder answers:
    - 1 (pin bump): Task 10
    - 2 (merge base, ceiling on worktrees): Tasks 8 and 11
    - 3 (in a session = `NESS_SESSION` or a `session/*` branch; a single `ness` command allowed): Task 3
    - 4 (rc from PR, stable refusal incl. the rc tree): Task 14
    - 5 (idle point, retries, failure surfaces, lock file): Tasks 10 and 12
    - 6 (veto): Tasks 5 and 12
  - Judge steps by a separate subagent: Tasks 0 (reviews), 3, 4, 5, 8–15. Tasks 6 and 7 have unit tests only; their acceptance is the Task 8, 10 and 12 scripts.
- **Types.** Each type is produced in one task and used unchanged later:
  - `update.Release`, `Source`, `Pin`, `Cache`, `Inputs`, `Plan` and `Failure`: Task 5
  - `worker.Stale`: Task 11; `worker.LockPath`: Task 10
  - `config.Update`, `Head`: Task 4; `PinError`, `ParseError`: Task 1
  - `Deps.Registry`, `CacheDir`: Task 4; `Deps.Releases`, `Executable`: Task 6
- **Order.**
  - `installVersion(d, rel)` and `machinePins` are produced in Task 8 and consumed in Task 12.
  - `idlePoint(d, p, cfg)`, `runUpgrade` and `upgradeOpts` are produced in Task 10 and consumed in Task 12.
  - `refreshCache` is produced in Task 9 and consumed in Task 12.
  - Docs (16) → close-out gates and integrate (17) → push and rc only after the operator's go (18).

## Settled points

1. v0.1.0 is a real public release (tag `v0.1.0` on `a7903ca`). The first fixture is built from it; the first rc is `v0.2.0-rc.1`.
2. "In a ness session" means `NESS_SESSION` is set or the branch matches `session/*` (one `git symbolic-ref`, no store read), so adopted roots get loud hooks (Task 3).
3. R-DOC-001 exempts `$XDG_CACHE_HOME/ness/update.json`; an empty cache dir is an error, never a write into the repository.
4. R-SYNC-072 exempts only `$XDG_CONFIG_HOME/ness/projects`. The registry has no lock: temp + rename, and a lost concurrent append is healed by the next sync.
5. Each session compares with its own `Session.Base` (a child: its parent's branch), so updates cascade down the tree.
6. The v0.1.0 fixture adds no migration, so `upgrade_previous` asserts no backup; the backup is unit-tested in Task 2.
7. Session records use `docs(sessions): <scope> …`; `sessions:` is reserved for Running-sessions notes (in `releasing.md`).
8. The stable job also refuses when no `vX.Y.Z-rc.*` tag's tree equals HEAD's tree.
9. `install.sh`, `go install` and mise skip the ceiling; doctor and the loud hooks report the skew (Task 16 docs).
10. 1.0 does not accept 0.x pins; the 1.0 notes will say so.
11. The behind-the-binary fyi is sent under every `auto` mode.
12. Promotion criteria: the field-feedback-3 list minus "0 hooks failing open", which `hook_skew` covers.
13. The planner calls are accepted.
14. The skew escape passes only a single simple command whose first word is `ness`, with no `;` `&` `|` `` ` `` `$` `<` `>` `(` `)` `\` or newline.
15. `self-update`, `upgrade` and `doctor` (besides `init`, `--version`, `--help`) skip the pin check; `upgrade` runs two minors behind. `config.PinError.Fix()` is the one fix text.
16. A dirty base or a held gate lock is "not idle": a silent skip. The failure record is only for failures after the idle point, stores the cause, and the worker logs only when the cause changes.
17. One flock helper (`worker.LockPath`); `Deps.Executable` is the one binary path; `self-update --check` fetches once.
18. The worker fetches whenever `check = true`, whatever `auto` is; `auto = "off"` only hides the result.
