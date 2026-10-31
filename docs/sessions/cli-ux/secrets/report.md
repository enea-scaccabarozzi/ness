> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# secrets — report

## Delivered
CX-22, option A as approved (design.md, SE-2):
- A `[notify.channels.NAME]` value is a literal or a reference: `secret:NAME` (the user's
  `~/.config/ness/secrets/<project>.toml`, 0600 in 0700, refused when too open) or `env:NAME`.
  `token` must be a reference; `chat_id`, `channel_id`, `repo`, `command` take either.
- ness resolves at channel build (`buildChannel`, one generic `Channel.Resolve`), so the worker
  and one-shot commands need nothing in their environment and `ness secret set` takes effect
  without a worker restart. A dry run needs no value.
- `ness secret set NAME` (stdin; no echo on a TTY), `list` (names, `referenced` marker),
  `unset`. New top-level command: R-CFG-030 now lists 17.
- Doctor `secrets` check: ok with the count, or fail per missing reference with `ness secret set
  NAME` / `export NAME`, or the `chmod 600` fix. `operator-notify` points to it.
- Every referenced value is masked in post errors (`masked` wrapper over any atom).
- `token_env` is refused with its rewrite `token = "env:<same>"`.
- Spec: R-CFG-080..085 (R-CFG-046 superseded), R-NTF-042 amended, R-NTF-060..061, R-CFG-002
  amended. Code: `internal/config/secret.go` (inside config: R-QA-002 fixes the package list),
  `internal/cli/secret.go`, `internal/cli/atoms.go`, `internal/cli/doctor_checks.go`.

## Evidence
- Acceptance script `internal/cli/testdata/script/secrets.txtar` written by a separate agent from
  the spec alone (the implementer did not write the judge); passed on the first run.
- Unit: `internal/config/secret_test.go`, `TestChannelSecrets`; updated post, post_command,
  doctor, probe_start, surface, builtin_* scripts.
- Full gate run before the load-control notice: everything green except one load timeout in
  `open_here_identity` (unrelated, it times out while running a command). Since then only targeted tests, per the manager's
  order: `go test ./internal/cli -run 'TestScript/(secrets|doctor)$'`, `./internal/config`,
  `sync --check`, audit — all pass. The integrate gate runs the full suite alone.
- End to end on this box with the branch binary and `env -i` (empty environment): stored both
  Ness secrets, `ness doctor` → `ok secrets 2 references resolve`, `ok operator-notify phone
  (telegram)`, and a real `ness post --channel phone` → `posted 1 part to phone`.

## Migration
- Ness: `.ness/ness.toml` (from the manager branch) now has `token = "secret:telegram-token"`,
  `chat_id = "secret:telegram-chat"` (public repo: the chat id is personal). Secrets file
  `~/.config/ness/secrets/Ness.toml` holds both. Removed `~/.config/ness/telegram-Ness.env` and
  the `ness-Ness.service.d/telegram.conf` drop-in, `daemon-reload` done (the running worker
  is on main, which has no notify, so nothing lost).
- The infrastructure lab: `~/.config/ness/secrets/infra-lab.toml` holds `telegram-token`. Its ness.toml change
  is for the operator. **Order matters**: the new binary refuses `token_env`, so the change must
  land together with the ness update there. Its env file and drop-in stay until then (deleting
  now would cut its notifications on the next worker restart).

## Lessons
- R-QA-002's fixed package list rejects a new `internal/` package; check
  `internal/testkit/repo_test.go` before creating one.
- `ness doctor` from a worktree reports sync drift against the primary checkout, not the
  worktree (`sync --check` in the worktree was clean). Worth a look by whoever owns doctor.
- The gate's lint can refuse with "parallel golangci-lint is running" when sibling sessions gate
  at once; that is load, not a finding.
