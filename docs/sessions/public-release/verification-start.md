> [!NOTE]
> Archive: the record of the session that prepared the public release. Names of private projects were replaced; the content is otherwise as written.

# Verification log: start/ and why/defaults, why/limits

Each entry: the command run by hand, where, its outcome. ness 0.1.0+b13684c9f547 (PATH), claude 2.1.316, herdr 0.9.3, codex 0.162.1, go 1.27.3, 2026-10-31. Scratch repos `acme` (quickstart) and `swap` (defaults, limits) under the session scratchpad. In the pages, the home directory is shown as `/home/you` and the scratch path as `/home/you/acme`.

## Install
- `git clone https://github.com/enea-scaccabarozzi/ness`: fails, the repo is private ("could not read Username"). Not in the pages.
- `git clone <local worktree> src && GOBIN=… go install ./cmd/ness`: exit 0; `ness --version` → `ness version 0.1.0+b13684c9f547`.
- `go install github.com/enea-scaccabarozzi/ness/cmd/ness@latest` with GOPRIVATE and an https→ssh rewrite (private repo): exit 0, silent; resolves the old `v0.1.0` tag; `ness --version` → `ness version 0.1.0`. The public form (no GOPRIVATE, no rewrite) cannot run until the repo is public.
- `export PATH="<GOBIN>:$PATH"; which ness; ness --version`: the go-installed binary resolves first.
- Release archives: `gh release view v0.1.0` lists linux amd64/arm64 tarballs on a private pre-release. Not downloaded; the page says they come with the first public release.
- Prerequisite versions from code: TestedHerdr 0.8.2, claude Tested 2.1.294, codex 0.162.1; CheckVersion accepts same major, at least tested. go.mod `go 1.26.0`.

## Quickstart (`acme`, fresh `git init -b main`, one empty commit)
- `ness init --gate 'true' --yes`: exit 0, the 14 `create` lines shown. ness.toml: ness, project, base, gate.
- `git add -A && git commit`, `ness sync --check`: exit 0, silent.
- `ness doctor` before the worker: exit 1 (fail worker, warn worker-binary, warn operator-notify).
- `ness worker install`: exit 0, prints the unit path and ExecStart.
- `ness doctor` right after install: exit 1, `fail worker not running (no heartbeat)` (race with the first beat). Rerun a few seconds later: exit 0, only `warn operator-notify`. FRICTION.
- `ness status` right after install: exit 0, `worker: … heartbeat 0s`.
- `claude` in a herdr tab in the repo: trust dialog, selection starts on `No, exit`; chose `Yes, I trust this folder`, then Ctrl+C twice. No prompt sent, no cost.
- `ness open --root --scope hello --tier economy --size quick --brief ../brief.md`: exit 0, `opened s-e852f9ffec34 … tier economy (explicit)`. No trust stall: the root's trust covered the worktree.
- Waiting: a background `until ness inbox | grep checkpoint` loop was refused by the pre-tool polling hook (correct). Waited on `git log main` instead. Landed `79b065b feat(hello): add hello.sh` about a minute after open.
- `ness status`: `done ctx 53k`. `ness inbox`: one checkpoint line. `ness inbox --show m-vmeyazjt`: Done/Now/Next text shown.
- `git log --oneline`, `./hello.sh`: 3 commits, `hello, world`. Commit holds docs/sessions/hello/{brief,report}.md and hello.sh.
- `ness stats`: 1 session, 2 turns, $0.14.
- `ness stats --by rung` and `--since 7d --by role`: exit 0, economy/root rows $0.14.
- `systemctl --user status ness-acme`: active (running), enabled.
- `ness status --all` in a terminal tab: rich view (header with worker beat, `○ s-e852f9ffec34 hello closed`). `ness status --watch` launched in the same tab; its view was not captured, so the page shows no output for it.
- `ness close s-e852f9ffec34`: exit 0, `closed s-e852f9ffec34 (hello)`; worktree removed; the `hello` tab stayed open (closed by hand).
- `ness worker uninstall`: exit 0; no `ness-acme` unit left.

## Trust stall (`swap`, never trusted)
- `ness open --root --scope hello --tier economy --size quick --brief ../brief.md` (meant as an outside-herdr test; the herdr socket env still reached herdr): exit 4 after 1 min, `stalled launch … Claude Code is asking whether to trust …/.worktrees/hello-ece98f9fb6bb; accept it in the session's tab (trusting the project root covers its worktrees), then ness open --resume s-ece98f9fb6bb`. Shown in the quickstart with the repo path written as `/home/you/acme`. Closed with `ness close` (exit 0), tab closed. No model cost.

## First week (`swap`)
- `[notify]` operator phone, telegram channel with `token = "secret:telegram-token"`, `features = ["notify"]`; `ness sync`: exit 0, `create .agents/skills/ness-notify/SKILL.md`.
- `printf dummy | ness secret set telegram-token`: exit 0; `ness secret list`: `telegram-token referenced`.
- `ness doctor`: `ok operator-notify phone (telegram)`, `ok secrets 1 reference resolves`.
- `ness post --channel phone --file note.md --dry-run`: exit 0, the Telegram JSON payload. No real post (no real token used).
- Cleanup: `ness secret unset telegram-token` exit 0; removed the empty secrets file of `swap`.

## Defaults (`swap`)
- `ness pick`: the seven rungs, base standard, ceiling apex.
- `[models] base = "economy"`, `ceiling = "deep"`, `enforce = "log"`; `ness sync`: exit 0, updates the 4 agents and CLAUDE_CODE_SUBAGENT_MODEL; `ness pick` marks economy base, deep ceiling; `ness pick --kind review` → `lean opus/low`.
- Plus `[models.kinds] review = 2` and `[tiers.economy] claude = "sonnet/high"`: sync exit 0; `ness pick --kind review` → `standard opus/medium`; economy row `sonnet/high`, codex value kept.
- Local profile: `"ness-entry#autonomy-default" = { set = "autonomy.md" }` and `"ness-entry#reserved-decisions" = { append = "reserved.md" }`, `profiles = ["local"]`; sync exit 0; rendered ness-entry holds `**plan first**.` under the default line and the appended reserved line.
- `profiles = ["program", "local"]`, sync: creates ness-program skill (has `## Challenge`), docs/doctrine.md, docs/program/*. Back to `["local"]`: `update …ness-entry`, `delete …ness-program/SKILL.md`; the seeded docs/program files stay.
- Hand edit of `.agents/skills/ness-entry/SKILL.md`: `ness sync --check` exit 0, `ness sync` exit 0, edit kept; `ness sync --keep` on it exit 4 (not in conflict); `ness doctor` `warn sync-drift edited since the last sync …`. The upgrade-conflict path (R-SYNC-015, `.ness-new`, exit 3) was not exercised; the page states it from the spec.
- `[launch] permissions = "client"` and `[integrate] mode = "merge"`: sync exit 0 (keys accepted).

## Limits
- `client = "codex"` without opt-in: `ness sync` exit 4, `client = "codex" needs experimental = ["codex"]`.
- With `experimental = ["codex"]`: sync exit 0, creates `.codex/config.toml` keys; `ness doctor`: `ok client-version codex 0.162.1`, `fail tier-models` (gpt-6.1-sol and astra not listed by `codex debug models` on this account), `warn worker-clients codex has no worker support`. FRICTION: the default Codex tier ids do not resolve on this box.
- Non-Linux refusal: not runnable here. `go test ./internal/cli -run TestOtherOSRefusesEverythingButVersion`: ok (darwin → exit 4, "Linux only"; `--version` exit 0). The page quotes the message from internal/cli/cli.go.

## Review round 1 fixes (`reverify-start`, 2026-10-31)
Scratch repo `reverify-start/acme`. Binary built from the worktree with `go build -buildvcs=false` into `reverify-start/go/bin/ness`, the way a module-mode `go install` stamps it (no commit). `NESS_UI=plain` exported. Secrets under a scratch `XDG_CONFIG_HOME`. No Claude session launched.
- `ness --version`: exit 0, `ness version 0.1.0`. The pages now show `0.x.y` (and the pin `0.x`), with a note in install.md.
- `ness init --gate 'true' --yes`: exit 0, the same 14 lines; `.ness/ness.toml` holds `ness = "0.1"` (page: `0.x`).
- `ness sync --check`: exit 0, silent with `NESS_UI=plain`. With `--ui rich`: `✓ in sync: nothing to change`. The quickstart now sets `NESS_UI=plain` before step 1.
- `ness worker install`: exit 0, `installed …/ness-acme.service (ExecStart=<bin>/ness worker run); check with …`. Page path written as `/home/you/go/bin/ness`.
- `ness doctor`: exit 0, first line `ness 0.1.0 at <bin>/ness`, `ok binary 0.1.0 at <bin>/ness, pin 0.1`, `hooks-binary`/`worker-binary` the same path, only `warn operator-notify`. Page: `0.x.y`, `pin 0.x`, `/home/you/go/bin/ness`.
- `ness status`: exit 0, `worker: 0.1.0 heartbeat 3s` (page: `worker: 0.x.y heartbeat 1s`).
- `ness worker uninstall`: exit 0; no `ness-acme` unit left, `systemctl --user is-active ness-acme` → inactive.
- Notify config, `ness sync`: exit 0, `create .agents/skills/ness-notify/SKILL.md`. `printf dummy | ness secret set telegram-token`: exit 0, file `acme.toml` under the scratch secrets dir. `ness secret list`: `telegram-token referenced`. Unset afterwards.
- `ness post --channel phone --file note.md --dry-run`: exit 0, `{"chat_id":"123456789","text":"<b>Done</b>\n\nThe first note.","parse_mode":"HTML"}`. first-week.md now shows this.
- `ness status --all --ui rich` (no sessions in the scratch repo): exit 0, rich header and tree; `--ui rich` wins over `NESS_UI=plain` and over the agent environment. The page's closed-session sample is from the earlier run, with the worker version written `0.x.y`.
- `profiles = ["program"]`, `ness sync`: exit 0, creates ness-program (has `## Challenge`) and the seeded docs. Removing it: `update .agents/skills/ness-entry/SKILL.md`, `delete .agents/skills/ness-program/SKILL.md`, as defaults.md shows.
- Quickstart step 9 no longer runs `ness inbox`; it shows the checkpoint text the earlier run filed (`ness inbox --show m-vmeyazjt`), labelled as the checkpoint.
