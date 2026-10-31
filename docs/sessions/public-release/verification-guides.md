> [!NOTE]
> Archive: the record of the session that prepared the public release. Names of private projects were replaced; the content is otherwise as written.

# Verification log: commands the guides and reference show

Each entry: the command run by hand, where, its outcome. ness 0.1.0+b13684c9f547, claude 2.1.316, herdr 0.9.3, 2026-10-31.

All in one scratch repo (`git init -b main`, one empty commit, project `qs-guides`), unless noted. Paths in shown output were rewritten to `/home/you`.

## Setup
- `ness init --gate true --yes`: exit 0. Skills, agents, settings keys, hooks, .gitignore block, AGENTS.md router, CLAUDE.md symlink; ness.toml has ness, project, base, gate.
- `ness sync --check`: exit 0.
- `ness doctor` (no worker): exit 1. Checks in the order shown in guides/doctor.md; worker fail "not running (no heartbeat)", operator-notify warn.

## Local profile (guides/local-profile.md)
- `ness sync` with the local profile of the guide: exit 0. create acme-planning SKILL.md, update ness-entry SKILL.md, create AGENTS.md#local/local, create plans/README.md. Slot text under "## Stop and ask"; block markers `local/local`; lock entries carry hash, id, kind, owner, path, upstream.
- `grep -n -B3 -A1 'billing schema' .agents/skills/ness-entry/SKILL.md`: exit 0, lines 122-126 as shown.

## Checks (guides/checks.md)
- `ness sync` with `section = "Plan"`: exit 4, "has no section "Plan"; its sections: Planning at acme, Plans". Fixed: exit 0.
- `ness open --root --scope retry-backoff --size quick --tier economy --brief FILE` with the plan-linked run check: exit 4, message and detail as shown; no session, worktree or branch created.
- `ness sync --check` with a guard pointing at skill acme-tests: exit 4, "guards[0]: skill "acme-tests" does not ship". Fixed: exit 0.
- `ness integrate --from feat/greet --into main --subject 'feat(greet): add greet.sh'` with the no-todo branch-forbids check: exit 4 as shown. After removing the TODO: exit 0, "integrated feat/greet into main at …".
- Gate script with `sh -n` on a broken branch: exit 1, output as shown, target unchanged. Passing branch: exit 0, "gate ok on top of <sha>".

## Integration (guides/integration.md)
- `git log -1 --format=%B` of the squash: subject, nested commits, `Ness-Integrates:` trailer, as shown.
- `ness integrate` without `--subject`, and with `--subject 'add bye'`: exit 4, the squash-subject refusal.
- `mode = "merge"` with `--subject`: exit 0, stderr "--subject ignored: integrate.mode merge writes its own message"; merge commit "integrate feat/bye into main".
- `mode = "rebase"`: exit 0, two commits replayed linearly. Repeat: exit 4, "nothing to integrate: feat/wave is already in main".
- `push = true`, `setup = "mkdir -p tmp"`, bare origin in the scratchpad: exit 0, remote main advanced.
- `release-please-config.json` read from the ness repository (sections incl. "Running sessions"); the release workflow not run.
- `mode = "pr"` not run (it would open a real pull request); described from internal/atom/pr.

## Program (guides/program.md)
- `ness sync` with `profiles = ["program", "local"]`: exit 0, the six lines shown.
- `ness open --root --scope payments --size program --brief FILE` without the profile: exit 4, size refusal as shown. With the profile: not run (it would start a Claude session).

## Probes (guides/probes.md, reference/probe-format.md)
- `ness probe run probes/disk.md`: exit 0, "GREEN / used=45".
- Same with `expect: used<10`: exit 1, VACUOUS.
- Floor `exit 1`: exit 1, RED. Floor `exit 2`: exit 1, UNCLEAR.
- `ness probe run --json probes/disk.md`: exit 0, object as shown.
- `evry: 30m`: exit 4, "unknown field evry". `once: 1h`: exit 4, strconv.ParseBool error.
- `ness probe start probes/disk.md` from a terminal: exit 4, "a probe needs a session as its owner".

## Notify (guides/notify.md)
- `ness sync` with the `command` channel: exit 0, renders ness-notify.
- `ness post --channel log --file note.md --dry-run`: exit 0, raw Markdown.
- `ness post --channel log --file note.md --thread release-1.4`: exit 0, "posted 1 part to log"; notes.log starts "thread=release-1.4".
- `ness post --channel nope …`: exit 4, "no such channel nope; configured: log".
- `ness post --channel team --file note.md --dry-run` (slack): exit 0, JSON as shown. `--channel phone` (telegram): exit 0, HTML JSON as shown. Nothing posted to any real service.
- `ness doctor` with both references missing: secrets fail as shown.
- `printf … | ness secret set slack-token`: exit 0. `ness secret list`: exit 0, "slack-token	referenced". With `TELEGRAM_TOKEN` set: operator-notify and secrets ok. `ness secret unset slack-token`: exit 0; the empty secrets file was removed by hand.

## Contracts (guides/contracts.md)
- `ness sync` with `contracts`: exit 0, renders ness-contracts.
- `ness contract check` on main: exit 0, "ok	3 frozen files".
- `ness contract freeze specs/api.json` from a terminal: exit 4, "freeze needs a session".
- On a branch, the edits of the guide: exit 1, "differs specs/api.json" and "added features/refund.feature"; added scenario and appended text pass. Editing a base scenario and the prefix: exit 1, both lines as shown.
- `ness contract amend specs/api.json --reason …` on the branch from a terminal: exit 4, "amend it on main directly".
- Same api.json committed on main: the branch check no longer reports it. refund.feature removed: exit 0.

## Models (guides/models.md)
- `ness pick`: exit 0, ladder with base standard, ceiling apex.
- `ness pick --kind review`: exit 0, "deep opus/high kind review" + model line. `--kind search`: economy. `--kind Design`: exit 4, kinds listed.
- `[models] base = "economy", ceiling = "deep"`, kinds review 2, migrate 1; `ness sync`: exit 0, four agents and the subagent model rewritten. `ness pick --kind review`: exit 0, standard opus/medium.
- `policy = "command"`, `echo '…migration.' | ness pick --stdin --kind implement`: exit 0, "standard opus/medium touches a migration". `ness pick --kind implement --json`: exit 0, economy "routine change" object.
- `ness stats --by rung`: exit 0, header and total row.

## Codex (guides/codex.md)
- `client = "codex"` without `experimental`: exit 4, the opt-in refusal.
- With the opt-in, `ness sync`: exit 0, .codex/config.toml with the three keys shown.
- `ness doctor`: client-version codex 0.162.1; tier-models fail per rung naming the listed models; worker-clients warn.
- With lean..apex mapped to gpt-5.6-terra: `ness doctor` tier-models ok "7 tiers resolve for codex"; `ness pick` shows the new column.
- Back to claude, `ness sync`: exit 0, the three keys deleted; an `[agents]` stub left in .codex/config.toml, removed by hand.

## Doctor (guides/doctor.md)
- `ness doctor`: exit 1 with a failing check; `ness doctor --json`: object with schema, version, checks (name, status, detail, fix).
- `ness doctor` in /tmp: exit 4 after printing the binary line.
- Fix texts compared with internal/cli/doctor_checks.go.

## Upgrade (guides/upgrade.md)
- `mise exec -- go build -o <scratch>/bin/ness ./cmd/ness` in the worktree: exit 0; `--version` "0.1.0+b13684c9f547.dirty". The installed binary was not replaced.
- `ness --version`: exit 0, "ness version 0.1.0+b13684c9f547".
- That build's `ness sync --check` in the scratch repo: exit 0.
- Deleted an owned agent file: `ness sync --check` exit 1 naming it; `ness sync` exit 0 recreated it.
- Pin "0.2": `ness status` exit 4, pin refusal as shown.
- `ness worker install`: exit 0. `systemctl --user restart ness-qs-guides.service`: exit 0, active; doctor worker "running 0.1.0+…, heartbeat 0s ago". `ness worker uninstall`: exit 0, unit removed.

## Files and sync (reference/files.md)
- `.git/ness/` holds gate.lock, integrate-main.lock, state.sqlite3, worker.lock, worktree.lock.
- Hand edit of ness-launch SKILL.md: `ness sync --check` exit 0; doctor sync-drift warn.
- User edit plus upstream change: `ness sync` exit 3, "conflict <path>: merge <path>.ness-new into it, then run ness sync --keep <path>". `ness sync --keep PATH`: exit 0, "kept <path>". Next sync: exit 0.
- `ness sync --dry-run` on a clean tree: exit 0, no output.

## Environment and exit codes (reference/environment.md)
- `ness pick --bogus`: exit 2, "unknown flag: --bogus".
- `NESS_UI=fancy ness status` with the agent variables unset: exit 2, accepted values listed. Inside the agent environment: plain output, exit 0.
- `ness status` in /tmp: exit 4.
- `ness hook pre-tool </dev/null`: exit 0.
- Final cleanup: `ness worker uninstall` in the scratch repo: exit 0, unit removed; `systemctl --user is-active ness-qs-guides.service` prints inactive.

## Re-verification after the review fixes (2026-10-31)
Binary 0.1.0+b13684c9f547.dirty; scratch repos under the scratchpad's reverify-guides/, `XDG_CONFIG_HOME` pointed at a scratch directory. Pages keep `/home/you/.local/bin/ness` and version 0.1.0+b13684c9f547 in place of the scratch path. No worker was installed and no client session was launched.
- program.md: `profiles = ["program", "local"]` with no local profile: `ness sync` exit 4, `ness: unknown profile "local" (not built in, and no .ness/profiles/local)`. `profiles = ["program"]`: `ness sync` exit 0, six lines as shown. `ness size` as the operator: exit 4, `ness: name the root with --session ID`.
- notify.md: `ness secret set slack-token` and `github-token`: exit 0 each; `ness secret list`: exit 0, both names. Telegram `ness post --dry-run`: exit 0, JSON with `<` and `>` escaped as `<`, `>`. Tracker post without `--thread`: exit 2; with `--thread 12`: exit 0, `{"body":...}`. `note.phone.md` beside `--file`: sent verbatim, exit 0. `note.slack.md` (kind fallback): exit 0, `{"channel":"C0123","blocks":[{"type":"markdown","text":"slack text\n"}]}`. `ness doctor` with the token exported: `ok secrets 3 references resolve`; with `TELEGRAM_TOKEN` unset: exit 1, the operator-notify and secrets fail lines as shown.
- doctor.md: `ness doctor` in a fresh repo `acme`: exit 1; worker-binary names `ness-acme.service`; worker-clients, dead-routes and gone-panes read `no store yet`; operator-notify fix and secrets detail as shown.
- codex.md: `client = "codex"`, `experimental = ["codex"]`: `ness sync` exit 0, three create lines; `.codex/config.toml` has `default_subagent_reasoning_effort = "medium"`. `ness doctor`: tier-models fails naming `astra` for apex and frontier and `gpt-6.1-sol` for the others; worker-clients `no store yet`. With every `[tiers.NAME] codex` set to listed models: `ok tier-models 7 tiers resolve for codex`. After `ness status` (exit 0) created the store: `warn worker-clients codex has no worker support`.
- integration.md: squash `ness integrate --from feat/greet --into main --subject 'feat(greet): add greet.sh'`: exit 0, `integrated feat/greet into main at <40-hex>`; message body as shown with the full `Ness-Integrates` hash. `mode = "rebase"` with `--subject`: exit 0, `ness: --subject ignored: integrate.mode rebase writes its own message`, then the integrated line.
- local-profile.md: the page's local profile: `ness sync` exit 0, four lines; the appended line sits at line 124 of ness-entry SKILL.md (heading at 122); `ness sync --check` after commit: exit 0.
- contracts.md: base check `ok 3 frozen files` exit 0; branch with changed api.json and a new feature file: exit 1, two lines; after editing a base scenario step and the experiment's first line: exit 1, four lines in the order shown. Renamed scenario: `scenario "Pay an invoice" removed`. `ness contract amend specs/api.json --reason test` from a terminal: exit 4, `...is frozen project-wide and this checkout is no session's; amend it on main directly`.
- reference/files.md: `ness sync --dry-run` with a pending local-profile change: exit 0, the plan line and a unified diff. Clean tree: exit 0, no output.
- reference/environment.md: agent variables unset, `CLICOLOR_FORCE=1 ness --help | grep -c ESC`: 0; with `NESS_UI=rich`: 29. `NESS_UI=fancy ness contract check`: runs and prints its findings; no ui-mode refusal.
