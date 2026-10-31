---
title: Atoms
description: Each piece of ness that touches the outside world sits behind a seam. An atom is one implementation of a seam, and swapping it changes nothing else.
---

A **seam** is a contract between ness and something outside it: the terminal, the coding agent,
git, a chat service, the service manager, the model choice. An **atom** is one implementation
of a seam. You choose atoms in `.ness/ness.toml`, or a profile chooses them for you, and the rest
of ness works the same with any of them.

## Seams and their atoms

| Seam | What it does | Built-in atoms | Chosen by |
|---|---|---|---|
| runtime | Opens a tab, runs a command, types text, reads the screen, tells idle from working | herdr | fixed |
| client | Launches the coding agent with model, effort, prompt and permissions; wires the hooks; reads the transcript for context size, usage and limit notices | Claude Code; Codex (experimental) | `client` |
| integrator | Folds a session's branch into its target and answers "integrated?" | `squash` (default), `merge`, `rebase`, `pr` | `integrate.mode` |
| channel | Renders one Markdown note for a platform, posts it, replies in a thread by key | `slack`, `telegram`, `github`, `command` | `kind` of each `[notify.channels.NAME]` |
| supervisor | Keeps the worker running | systemd user unit | fixed |
| model selector | Turns a task into a rung of the model ladder, with a confidence and a reason | `declared` (default), `command`, `jev` | `models.policy` |

A profile's `[atoms]` table may choose three of them, with the keys `client`, `integrator` and
`model_policy`. When two profiles choose differently for one seam, sync refuses until `ness.toml`
settles it.

## Runtime: herdr

Every session runs in a herdr tab, and the worker reads and types into those tabs. ness refuses
to open a session or start the worker without herdr at a tested version. There is no tmux or
plain-terminal fallback.

## Client: Claude Code and Codex

The client atom knows how to start the agent and how to read it. For Claude Code that means the
launch arguments, the hooks in `.claude/settings.json`, the transcript reader, and the screen
patterns for limits, errors and background work.

Codex is experimental. It is refused unless `ness.toml` opts in:

```toml title=".ness/ness.toml"
client       = "codex"
experimental = ["codex"]
```

The worker delivers only to clients whose atom can read the screen and type the resume text.
`ness doctor` warns when a session's client has no worker support.

## Integrator: squash, merge, rebase, pr

`ness integrate` folds a session's branch through your gate, in a throwaway checkout. The
integrator decides how the fold looks on the target.

[Pick a mode](../guides/integration.md#pick-a-mode) shows what each mode lands. With `pr`, the
local target does not move, and "integrated?" is true once the PR is merged.

## Channel: slack, telegram, github, command

`ness post` and the operator alerts go through channel atoms. Each atom renders the same
Markdown note into its platform's format and splits a long note on paragraph and code-fence
boundaries.

The [channel kinds table](../guides/notify.md#add-a-slack-telegram-or-github-channel) lists
what each one needs, how it renders, its limit per part and how it threads.

## Supervisor: systemd

`ness worker install` writes a systemd user unit that runs the binary by its absolute path. It
refuses without a systemd user session. `ness worker run` runs the worker in the foreground
instead.

## Model selector: declared, command, jev

A selector answers an offset from the base rung, with a confidence and a reason.
[Selectors](./models.md#selectors) describes each one. `jev` asks Jev, a hosted judgement model
from TypeSafe.

## Checking the atoms

`ness doctor` checks each atom the project uses. These lines of its output cover the runtime,
the client, the tier models, worker support and the supervisor; the other lines are trimmed:

```text title="ness doctor"
ok	herdr-version	0.9.3
ok	client-version	claude 2.1.316
ok	tier-models	7 tiers resolve for claude
ok	worker-clients	the worker delivers to claude
ok	systemd	user session running
```

## Links

- [Models](./models.md): the ladder the model selectors climb.
- [Profiles and sync](./profiles-and-sync.md): how a profile chooses atoms.
- [Integration](../guides/integration.md): squash, merge, rebase and PR in practice.
- [Notify](../guides/notify.md): configure a channel.
- [Codex](../guides/codex.md): the experimental client.
- [Doctor](../guides/doctor.md): read and fix doctor's checks.
- [Configuration](../reference/config.md): `client`, `[integrate]`, `[notify]`, `[models]`.

Next: [Features](./features.md).
