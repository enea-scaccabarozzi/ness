> [!NOTE]
> Archive: the record of the session that prepared the public release. Names of private projects were replaced; the content is otherwise as written.

# Verification log: commands the concepts pages show

Each entry: the command run by hand, where, its outcome. ness 0.1.0+b13684c9f547, claude 2.1.316, herdr 0.9.3, 2026-10-31.
No Claude session was launched for these pages.

## Scratch repo (fresh `git init -b main`, one empty commit, directory `acme`)
- `ness init --gate true --yes`: exit 0. ness.toml has ness, project, base and gate. Committed.

## Models (models.md)
- `ness pick`: exit 0. Seven rungs, `standard` marked base, `apex` marked ceiling.
- `ness pick --kind review`: exit 0. `deep  opus/high  kind review`, then `model: opus  effort: high`.
- `ness pick --kind nope`: exit 4. "name the work kind: one of debug, design, implement, research, review, run, search, verify".
- `echo "fix the flaky retry test" | ness pick --kind debug --stdin`: exit 0, standard opus/medium.
- `.claude/agents/ness-sweep.md` frontmatter after init: model sonnet, effort medium.
- `[models] policy = "command"`, `command = "./pick"` (a sh script printing `{"offset", "confidence", "reason"}`):
  `echo "fix a typo in the docs" | ness pick --stdin` exit 0 → `economy sonnet/medium docs-only change`;
  `echo "add retry backoff to the client" | ness pick --stdin` exit 0 → `standard opus/medium code change`.
- `ness stats --help`: `--by session | tier | role | rung | tool`. `ness open --help`: `--tier`, `--kind`, `--reason`, `--effort`.

## Messages and the worker (messages-and-worker.md)
- `ness inbox` as the operator, empty store: exit 0, no output.
- `ness send --to operator --body hi` from a terminal: exit 4, "the operator cannot send to the operator; name a root session id with --to". Not shown in the page.
- `ness send --help`, `ness inbox --help`: flags as quoted in the page.
- `ness worker install`: exit 0, wrote `~/.config/systemd/user/ness-acme.service`. The first `ness status` right after install showed `worker: not running` (before the first pass); a few seconds later `ness status` exit 0 showed `worker: 0.1.0+b13684c9f547 heartbeat 0s`.
- `ness worker uninstall`: exit 0. `systemctl --user status ness-acme` afterwards: unit not found.

## Profiles and sync (profiles-and-sync.md)
- Local profile `.ness/profiles/local/` (append to `ness-entry#stop-and-ask`, block `AGENTS.md#local`, seed `docs/runbook.md`), `profiles = ["local"]`:
  `ness sync --dry-run` exit 0, plan `update` SKILL.md, `create AGENTS.md#local/local`, `create docs/runbook.md`, with diffs.
- `ness sync`: exit 0. Committed.
- Appended a line to `.agents/skills/ness-entry/SKILL.md` by hand, `ness sync`: exit 0, no output (upstream unchanged).
- Changed the local profile's slot text, `ness sync`: exit 3, `conflict .agents/skills/ness-entry/SKILL.md: merge ….ness-new into it, then run ness sync --keep …`; `SKILL.md.ness-new` written, SKILL.md untouched (hand line still last). `ness sync --check`: exit 3.
- Merged by hand, `ness sync --keep .agents/skills/ness-entry/SKILL.md`: exit 0, `kept …`. `ness sync --check`: exit 0, `.ness-new` gone.
- Appended a line to AGENTS.md outside the blocks, `ness sync --check`: exit 0.
- `profiles = ["behavior-specs", "local"]`, `ness sync --dry-run`: exit 4, "profile behavior-specs requires profile go-strict; add it to profiles in .ness/ness.toml".

## Features (features.md)
- `features = ["probes", "notify", "contracts"]`, `ness sync`: exit 0, created ness-contracts, ness-monitor (+ references/example-prod-week.md), ness-notify, ness-probes skills.
- Probe `probes/disk.md` (every 1h, floor `./probes/disk.sh` printing `n=1 used=<pct>`, expect n>=1, wake not-green): `ness probe run probes/disk.md` exit 0, `GREEN` then `n=1 used=65`.
- `[notify.channels.log] kind = "command"`, `ness post --channel log --file note.md --dry-run`: exit 0, printed the Markdown as is.
- `[contracts] frozen = ["specs/*.txt"]`, a branch changing specs/api.txt: `ness contract check` exit 1, `differs	specs/api.txt	.ness/ness.toml`. On main: exit 0, `ok	1 frozen files`.

## Atoms (atoms.md)
- `ness doctor`: exit 0. The page shows five of its lines: herdr-version 0.9.3, client-version claude 2.1.316, tier-models 7 tiers resolve for claude, worker-clients the worker delivers to claude, systemd user session running. (Run while the store still held the uninstalled worker's last heartbeat; those lines are not shown.)

## Clean-up
- Worker uninstalled (above); no `ness-acme` unit left.
- Paths in the worker install output are shown with `/home/you` in place of the real home directory.
