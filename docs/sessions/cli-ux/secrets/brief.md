> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# secrets

## The operator's words (verbatim)
"Regarding telegram maybe it's time to desgin a system to allow adding secrets into configurations, that works better and it's more exrensible than the one used now. [...] Regarding the question i wat both options, sice in many cases chat id is not a secret"
(The question was: `chat_id` written in the committed .ness/ness.toml, or a private option next to the token. "Both" = a value may be inline or come from the secret mechanism.)

## Context
Today a notify channel names an environment variable (`token_env`) and the worker must have it in its environment (internal/config/config.go channel keys, internal/cli/atoms.go buildChannel). The generated systemd unit (internal/atom/systemd/systemd.go) sets only PATH, so the root wired the Telegram tokens by hand: `~/.config/ness/telegram-<project>.env` (0600) loaded by a drop-in `~/.config/systemd/user/ness-<project>.service.d/telegram.conf` with `EnvironmentFile=`. The repo will become public, so nothing secret or personal may land in tracked files. Decision CX-22 in docs/sessions/cli-ux/decisions.md. Other secret-bearing places to consider: slack/github channel tokens, the `command` channel, probes' redact_env, future atoms (forges).

## Result owed (design first)
Autonomy: plan first. Design, then stop and send the design to your manager as a `decision` item (`ness send --to manager --intent decision --body @FILE`), written for an operator who has read nothing: the problem, 2-3 options with trade-offs (e.g. a per-user secrets file outside the repo keyed by project, references like `env:NAME` / `file:PATH` / `cmd:...` in any secret-capable key, systemd credentials, OS keyring), your recommendation, how a value is marked secret vs plain (chat_id inline or by reference), how the worker and one-shot commands get the values, how `ness doctor` reports a missing secret without printing it, redaction in logs/errors, migration from `token_env`, and what ness writes (e.g. `ness secret set`?) vs what the user does. Keep it tunnel-vision: the smallest mechanism that covers the cases above and extends to new atoms without per-atom code. Do not implement until the manager relays the operator's go; then implement it (spec ids core-config.md from R-CFG-080 up, notify from R-NTF-060 up).

## Limits
Tier standard. Never print a secret value; never read the real token files under ~/.config/ness beyond checking they exist.

## Rules for this session
- Never read or grep anything under `docs/discovery` (exclude it from every search).
- Gate: `mise exec -- make validate`; check its exit status directly, never pipe it into tail before a commit.
- No `git stash`, no pkill, no `--no-verify`. Do not push. Integrate into the manager branch with `ness integrate` when done, then hand off.
- Two sibling sessions run in parallel (withdraw, integrate-fixes, seams, secrets). Before picking a new requirement id, grep the spec and pick from your own reserved range below, to avoid duplicate ids.
- Record decisions in your session decisions.md, and a report.md with evidence and lessons at handoff.
