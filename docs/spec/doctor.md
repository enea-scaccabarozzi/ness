> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Doctor (DOC)

cli `doctor` over `internal/session` checks, `internal/sync`, `internal/store`. Sources: §4, §5
(`ness doctor`), §8 (paused probes), §12 (Codex ids), §16 (#3), D8, D14.

- **R-DOC-001** `ness doctor` MUST be read-only: it MUST NOT modify any file except the
  update-check cache `$XDG_CACHE_HOME/ness/update.json` (R-UPD-050), nor any store row, git ref,
  worktree or runtime tab. An empty cache directory is an error, never a write into the
  repository.
  Verify: testscript — tree hash, `git for-each-ref`, store dump and fake herdr state equal
  before and after, with every check failing; outside the repository only
  `$XDG_CACHE_HOME/ness/update.json` may change.
- **R-DOC-002** For every failed check doctor MUST print a fix command and MUST NOT run it.
  Verify: testscript — each failing check's output line contains a command; the command's effect
  is absent afterwards.
- **R-DOC-003** `ness doctor --json` MUST report each check with its name, status and fix.
  Verify: testscript — JSON array/object contains every check below by name.
- **R-DOC-004** Doctor MUST report the running binary's version and path against the project pin.
  Verify: testscript — pin mismatch → check fails naming both versions and the path.
- **R-DOC-005** Doctor MUST report which binary the client hooks and the worker unit resolve, and
  fail when either differs from the running binary (§16 #3).
  Verify: testscript — unit `ExecStart` points at another binary → check fails naming both paths.
- **R-DOC-006** Doctor MUST report the client and herdr versions and fail on untested ones.
  Verify: testscript — fakes report untested versions → two failures.
- **R-DOC-007** Doctor MUST check that every tier model id resolves, naming the failing tier row;
  for Codex it MUST validate ids against `codex debug models`.
  Verify: testscript — fake `codex debug models` lacking `astra` → failure naming the frontier row.
- **R-DOC-008** Doctor MUST report every global skill that shadows a project skill, naming both
  paths.
  Verify: testscript — as R-SESS-005; doctor lists the pair.
- **R-DOC-009** Doctor MUST report every tool in an applied profile's `requires_tools` that is
  not on `PATH`.
  Verify: testscript — missing tool → failure naming tool and profile.
- **R-DOC-010** Doctor MUST report whether the worker is running, whether its version equals the
  binary's, and the age of its heartbeat.
  Verify: testscript — stale heartbeat fixture → failure with the age; other-version heartbeat →
  failure.
- **R-DOC-011** Doctor MUST report sync drift (what `ness sync --check` would find) without
  writing.
  Verify: testscript — an edit to a generated file is reported as a `warn` (`ness sync --check` passes on a
  kept edit); a real drift, what `sync --check` fails on, is a `fail`; the tree is unchanged.
- **R-DOC-012** Doctor MUST report dead routes (open items addressed to closed or unknown
  sessions).
  Verify: testscript — store fixture with such an item → listed.
- **R-DOC-013** Doctor MUST report sessions whose runtime pane is gone.
  Verify: testscript — fake herdr lacks the pane → session listed.
- **R-DOC-014** Doctor MUST report worktrees under `.worktrees/` without a session.
  Verify: testscript — stray worktree → listed.
- **R-DOC-015** Doctor MUST report active probes without a live owner.
  Verify: testscript — paused probe → listed.
- **R-DOC-016** Doctor MUST report a missing systemd user session as a worker-install problem.
  Verify: testscript — fake systemctl failing → reported with the `ness worker run` alternative.
- **R-DOC-017** Doctor MUST complete all checks even when earlier ones fail.
  Verify: testscript — every check failing → every check reported.
- **R-DOC-024** Doctor MUST report the operator notification channel (`notify.operator`): `ok`
  naming the channel and its kind; `warn` when none is configured, saying the operator is not told
  when a session needs them; `fail` when the named channel cannot be built (its token variable
  unset).
  Verify: testscript — no `[notify] operator` → `warn\toperator-notify\t…not told…`; a telegram
  channel with its token set → `ok\toperator-notify\tphone (telegram)`; token unset → `fail`.
- **R-DOC-025** Doctor MUST list a `jev` check last: `ok` with `policy NAME` when the policy is not
  `jev`; under `jev` a `warn`, never a `fail`, when `models.jev.key` does not resolve (named once
  in the whole output, no request); otherwise one minimal `POST {endpoint}/v1/systemone` (the
  pinned model, state `{"task":"ness doctor probe"}`, one `noul` question, `models.jev.timeout`)
  — `ok` naming the model on 200; `warn` with the status on 401 or any other non-200, naming the
  model on 404/422, and on unreachable (offline). Never `GET /v1/models` (it lists only aliases),
  never a response body, never the key.
  Verify: testscript doctor_jev — declared, unset key, 200, 404/422, 401, 500, offline.

## Output and exit

- **R-DOC-020** Doctor MUST print one line per check with status `ok`, `warn` or `fail`, and a fix
  command on every `fail` line.
  Verify: testscript — golden output for a fixture with each status.
- **R-DOC-021** Doctor MUST exit 1 when any check is `fail`, and 0 otherwise (warnings included).
  Verify: testscript — one failing check → 1; only warnings → 0.
- **R-DOC-022** A worker heartbeat older than 60 s MUST be `fail`.
  Verify: testscript — heartbeat 61 s old → fail; 59 s → ok.
- **R-DOC-023** Outside a ness project, doctor MUST print the binary version and then refuse
  (exit 4).
  Verify: testscript — stdout has the version line; exit 4.
