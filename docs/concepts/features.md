---
title: Features
description: Probes, notify and contracts are machinery in the binary that a project switches on. Each brings its own commands and its own skill.
---

A **feature** is machinery built into the ness binary that a project switches on. It needs code,
so it is not a profile. It is off until you enable it, and its skill text reaches your agents
only when it is on.

| Feature | What it answers | Commands | Skill |
|---|---|---|---|
| probes | "Is the running world still right?" | `ness probe start\|run\|verdict\|stop` | `ness-probes`, `ness-monitor` |
| notify | "Who outside the repository needs to read this?" | `ness post` | `ness-notify` |
| contracts | "Did someone change what we agreed to keep?" | `ness contract freeze\|amend\|check` | `ness-contracts` |

Turn them on in `.ness/ness.toml`, or let a profile enable them, then sync:

```toml title=".ness/ness.toml"
features = ["probes", "notify", "contracts"]
```

```sh
ness sync
```

```text title="Output"
create .agents/skills/ness-contracts/SKILL.md
create .agents/skills/ness-monitor/SKILL.md
create .agents/skills/ness-monitor/references/example-prod-week.md
create .agents/skills/ness-notify/SKILL.md
create .agents/skills/ness-probes/SKILL.md
```

A command of a feature that is off is refused and names the feature to enable.

## Probes

A **probe** is a question about the running world, asked on a schedule and owned by a session.
It is one Markdown file. The frontmatter says when to ask, and the body, the scope, says what
the woken agent should check.

```markdown title="probes/disk.md"
---
every: 1h
floor: ./probes/disk.sh
expect: n>=1
wake: not-green
---
The root disk stays under 90% full.

When woken: find what grew since the last GREEN and say whether it is expected.
```

The optional **floor** is a cheap command. Its exit code gives the state: 0 GREEN, 1 RED, any
other code or a timeout UNCLEAR. A floor that exits 0 but whose last line's `key=value` pairs do
not meet `expect` gives VACUOUS, never GREEN.

`ness probe run` runs the floor once and records
nothing, so you can try a probe while you write it:

```sh
ness probe run probes/disk.md
```

```text title="Output"
GREEN
n=1 used=65
```

`ness probe start` hands the file to the worker. On schedule the worker runs the floor, and when
`wake` says so it puts a request in the owner's inbox. The owner investigates within the scope
and answers with `ness probe verdict`.

The worker never starts an agent and never runs the scope itself. A probe whose owner closes
passes to the manager. With no live owner, it pauses.

## Notify

`ness post` sends one Markdown note to a channel you configured: Slack, Telegram, a GitHub issue
or pull request, or your own command. The channel atom renders the note for its platform and
splits it when it is too long. `--dry-run` prints the exact payload and sends nothing:

```toml title=".ness/ness.toml"
[notify.channels.log]
kind    = "command"
command = "cat >> notes.log"
```

```sh
ness post --channel log --file note.md --dry-run
```

```text title="Output"
# Release 1.2

The retry backoff is live.
```

A post is separate from `ness send` because it cannot be taken back. `ness send` never posts to
a channel. The same channels carry the worker's alerts to the operator, when `[notify] operator`
names one.

## Contracts

A **contract** is a file frozen so that no session changes it by accident: an API schema, an
acceptance scenario, a spec. There are two ways to freeze one:

- `ness contract freeze PATH` records the file in the session's `contract.lock`.
- `[contracts] frozen` in `ness.toml` freezes paths project-wide, against the base branch.

How a file is locked depends on its kind:

| File | Lock |
|---|---|
| `.feature` | Each scenario is locked. Lines outside the scenarios may change. |
| A `.md` file in `[contracts] prefix` | The existing text is locked. New text may be appended. |
| A file of a `[contracts.formats]` entry | Each unit the format's command reads is locked |
| Any other file | The whole file |

The pre-tool hook refuses an edit to a whole-file contract and lets the other kinds through with
a warning. `ness contract check` catches every change, including one made from a shell, and
belongs in your gate:

```toml title=".ness/ness.toml"
[contracts]
frozen = ["specs/*.txt"]
```

```sh
ness contract check
```

```text title="Output, on a branch that changed specs/api.txt"
differs	specs/api.txt	.ness/ness.toml
```

It exits 1 when a frozen file differs and 0 when none does. To change a frozen file on purpose, a
session runs `ness contract amend` and the reason goes to its `decisions.md`. The command needs an
approval:

```sh
ness contract amend PATHS... --reason TEXT [--item ITEM | --authorized WORDS]
```

| Lock | Approval |
|---|---|
| The session's own `contract.lock` | `--item ITEM`: an item from its manager or the operator, addressed to it, that names the path. Its manager or the operator may amend without one. |
| `[contracts] frozen`, project-wide | `--authorized WORDS`: the words the operator typed to the session approving the change |

Without the approval, `amend` exits 4 and names the way to get it.

## Links

- [Probes](../guides/probes.md): write, start and answer a probe.
- [Probe format](../reference/probe-format.md): every frontmatter field.
- [Notify](../guides/notify.md): configure channels and secrets.
- [Contracts](../guides/contracts.md): freeze, amend and check.
- [Atoms](./atoms.md): the channel atoms and their limits.
- [Session record](./session-record.md): where `contract.lock` and the `Amend` lines live.

Next: [models](./models.md).
