> [!NOTE]
> Archive: the record of the session that prepared the public release. Names of private projects were replaced; the content is otherwise as written.

# Verification log: commands the docs show

Each entry: the command run by hand, where, its outcome. ness 0.1.0+b13684c9f547, claude 2.1.316, herdr 0.9.3, 2026-10-31.

## Quickstart (scratch repo, fresh `git init`, one empty commit)
- `ness init --gate 'true' --yes`: exit 0. Created the skills, agents, settings keys, hooks, the .gitignore block, the AGENTS.md router block and the CLAUDE.md symlink. ness.toml has ness, project, base and gate.
- Committed, then `ness sync --check`: exit 0.
- `ness doctor`: exit 1 before the worker exists (fail worker: not running; warn operator-notify). The quickstart must show worker install before doctor reads clean.
- `ness worker install`: exit 0. Unit active; `ness status` shows the worker heartbeat.
- `ness open --root --scope hello --tier economy --size quick --brief FILE`: exit 4, stalled launch. Claude Code asked to trust the new worktree. ness printed the exact fix (trust it, then `ness open --resume ID`). DOC: open claude once in the repo and trust it first; trusting the project root covers its worktrees.
- After trusting: `ness open --resume ID` exit 0. The session added hello.sh, integrated d563a4a `feat(hello): add hello.sh`, wrote docs/sessions/hello/{brief,report}.md, and sent a Done/Now/Next checkpoint. `ness inbox` and `ness inbox --show` read it.
- `ness stats`: 1 session, 2 turns, $0.15 (economy rung).
- `ness close ID` (operator, from a terminal): exit 0; `ness status --all` shows closed.
- `ness worker uninstall`: exit 0.

## Features (scratch repo, features probes+notify+contracts, profile git-conventional)
- `ness sync` after enabling features: renders ness-contracts, ness-monitor, ness-notify and ness-probes.
- Probe file with floor `./probes/disk.sh` (prints `n=1 used=64`, exits 0): `ness probe run` prints GREEN and the evidence.
- `[notify.channels.log] kind = "command"`: `ness post --channel log --file note.md --dry-run` prints the payload; the real post wrote it to stdin of the command.
- `ness contract freeze PATH` outside a session: refused, exit 4 ("freeze needs a session: the operator freezes project-wide paths in [contracts] frozen").
- `[contracts] frozen = ["specs/*.txt"]`: a branch changing specs/api.txt → `ness contract check` exit 1, `differs specs/api.txt`. A `.feature` file is scenario-locked (R-CTR-041): appending a non-scenario line does not differ. On base, merge-base = HEAD, so check passes.
- `ness pick`: prints the seven rungs, base standard, ceiling apex. `ness pick --kind review`: deep, opus/high.

## Field numbers (landing proof)
From read-only sqlite queries on the four stores (scratchpad field-usage.md, q.sql, q2.sql), 2026-10-10 to 10-31: 81 sessions (17 roots, 64 children), 74 integrated, 1,722 messages, 122 probes and 1,484 runs.

## Site rendering (2026-10-31)
- Landing via the tailnet URL in dev: blank below the nav. Cause: Next dev blocks /_next resources for non-localhost origins, so React never hydrated and the reveal animations kept content at opacity 0. Fixes: allowedDevOrigins from DEV_ORIGINS (no IPs committed); a failsafe shows hidden content when hydration has not happened after 3 s.
- Checked: headless chromium via the tailnet URL, full landing renders with no console errors. Static export with every .js request blocked: the hero, copy and buttons render.
