---
title: Opinions in the defaults
description: The choices ness makes for you, why, and how to change each one.
---

The defaults are decisions, not principles. They are what worked on the projects ness was built
on. Each one below has a reason and a way to change it in your repository.

A few terms recur on this page. A **root** is a session that answers to you, the **operator**.
A **child** is a session that another session, its **manager**, opened.

| Default | Where it lives | How you change it |
|---|---|---|
| [Results are challenged](#results-are-challenged), with the opt-in `program` profile | `program` profile | Add it to `profiles`, or leave it out |
| [You work in the session chat](#you-work-in-the-session-chat) | Core skill text | Edit the rendered skill |
| [Checkpoints say Done, Now, Next](#checkpoints-say-done-now-next) | Core skill text | Edit the rendered skill |
| [The model ladder](#the-model-ladder) | `[tiers]` and `[models]` in `.ness/ness.toml` | Config keys |
| [Reserved decisions](#reserved-decisions) | Core skill text, a list slot | Add with a profile, remove by editing |
| [Autonomy per run](#autonomy-per-run) | Core skill text, a single slot | Your local profile |
| [Sessions skip permission prompts](#sessions-skip-permission-prompts) | `[launch]` | `permissions = "client"` |
| [Branches land as one squashed commit](#branches-land-as-one-squashed-commit) | `[integrate]` | `mode` |

Every row but the first works out of the box. The first comes only with the `program` profile,
which `ness init` does not add. Three mechanisms cover every change on this page:

- **Config**: a key in `.ness/ness.toml`, then `ness sync`.
- **Your local profile**: `.ness/profiles/local/`, listed in `profiles`. It is applied last,
  and it may replace a single slot another profile set. A **slot** is a marked place in a
  rendered file, such as a skill or `AGENTS.md`, that profiles fill.
- **An edit to a rendered file**: `ness sync` keeps your edit. When an upgrade changes the
  same file, sync stops with a conflict, writes its version next to yours as `<path>.ness-new`,
  and exits 3. `ness sync --keep <path>` keeps yours.

## Results are challenged

This default comes with the `program` profile, which you opt in to with `profiles =
["program"]`. With it, a program root questions what it gets back: a child's result, a
measurement, a remark from you. The reason: a manager that folds results unread passes errors up
the tree, and you find them last.

The profile carries this as the `Challenge` section of its `ness-program` skill. At every fold,
the root reads the result file instead of the message and lists the alternatives the child
rejected. It also writes what would make the result naive.

To drop it, remove `program` from `profiles` in `.ness/ness.toml` and run `ness sync`:

```text title="Output"
update .agents/skills/ness-entry/SKILL.md
delete .agents/skills/ness-program/SKILL.md
```

The files the profile seeded for you, such as `docs/program/roadmap.md`, stay. They are yours.

## You work in the session chat

You talk to sessions in their chat, and only there. Sessions read `ness inbox`; you never do. A
session also files each question and [checkpoint](#checkpoints-say-done-now-next) for you as an item, which is the record and
triggers the phone notification.

The phone only notifies: it points you at the session that needs you, and you answer in that
session's chat. The reason: one conversation per session keeps the context in one place, for
you and for the agent.

This is core text in the `ness-entry` skill, section `Ask the operator`. To change it, edit
`.agents/skills/ness-entry/SKILL.md`. `ness sync` keeps the edit, and `ness doctor` reminds you
of it:

```text title="Output"
warn	sync-drift	edited since the last sync: nothing to do to keep it; to take ness's version: rm .agents/skills/ness-entry/SKILL.md && ness sync
```

The phone channel itself is optional: without `[notify] operator`, nothing is posted.

## Checkpoints say Done, Now, Next

A **checkpoint** is a root's progress report, in its chat: `## Done`, `## Now` and `## Next`, plus
`## Ask` when it needs you. A child's handoff to its manager uses the same three headings. The
reason: you read many of these, often on a small screen, and a fixed shape keeps each line in
the same place.

This is core text too, in `ness-entry`, section `Communicate`. Change it the same way as the
section above.

## The model ladder

Every session runs at a **rung** of a seven-rung ladder, cheapest first. Flags and config call a
rung by its name, the **tier**, such as `economy`. Each kind of work (search, implement, review,
design and others) gets a rung counted from the project's base.

A pre-tool hook refuses a subagent call above that rung. An `escalate: <reason>` line in the
header at the top of the prompt allows one rung more. The reason: most work does not need the
strongest model, and the ladder turns the expensive call into a decision with a written reason.

`ness pick` prints the ladder:

```text title="Output"
bulk      haiku/medium   gpt-6-luna/low
economy   sonnet/medium  gpt-6-luna/medium
lean      opus/low       gpt-6.1-sol/low
standard  opus/medium    gpt-6.1-sol/medium    base
deep      opus/high      gpt-6.1-sol/high
frontier  fable/medium   astra/medium
apex      fable/high     astra/high            ceiling
```

Change it in `.ness/ness.toml`:

```toml title=".ness/ness.toml"
[models]
base = "economy"     # the rung every offset counts from, and every fallback
ceiling = "deep"     # nothing goes above this rung
enforce = "log"      # record the calls the hook would refuse, and allow them

[models.kinds]
review = 2           # review work runs two rungs above base instead of one

[tiers.economy]
claude = "sonnet/high"
```

After `ness sync`, `ness pick` marks the new base and ceiling, and the generated subagents in
`.claude/agents/` move with them. With this block, review work lands two rungs above economy:

```sh
ness pick --kind review
```

```text title="Output"
standard  opus/medium    kind review
model: opus  effort: medium
```

To choose per brief instead of by a fixed offset per kind, set `[models] policy = "command"`.
Point `command` at a script that reads the brief and answers a rung offset.

## Reserved decisions

Some decisions stay yours:

- irreversible actions;
- money, accounts and credentials;
- publishing outside the repository;
- policy changes and scope beyond the brief;
- product calls that users would notice.

A session raises one as a `decision` item with options and a recommendation. The reason: these
calls are expensive to undo, and an agent cannot see their cost.

The list is a list slot, `ness-entry#reserved-decisions`. Add to it from your local profile:

```toml title=".ness/profiles/local/profile.toml"
name    = "local"
summary = "this project's own rules"

[slots]
"ness-entry#reserved-decisions" = { append = "reserved.md" }
```

```markdown title=".ness/profiles/local/reserved.md"
- changes to the database schema
```

A profile can only append to a list slot. To remove one of the core entries, edit the rendered
skill.

## Autonomy per run

You set how much a session may do alone each time you start work, in your own words. The brief
records one of three levels:

| Level | The session |
|---|---|
| plan first | writes the plan and stops until you approve it |
| report, then act | says what it is about to do before each step the brief did not spell out, then does it |
| act within the brief | does the work and asks only for what is outside the brief or reserved |

When the brief names no level, the project default applies: `act within the brief`. The
reason: you know per run how much you trust the task, and a project-wide setting would be wrong
half the time.

The default is the single slot `ness-entry#autonomy-default`. Replace it from your local profile:

```toml title=".ness/profiles/local/profile.toml"
[slots]
"ness-entry#autonomy-default" = { set = "autonomy.md" }
```

```markdown title=".ness/profiles/local/autonomy.md"
**plan first**.
```

List the profile in `.ness/ness.toml` with `profiles = ["local"]`, then run `ness sync`.

## Sessions skip permission prompts

Sessions ness launches start Claude Code with `--dangerously-skip-permissions`. The reason:
sessions run in tabs you are not watching, and a permission prompt there stops the work until you
notice it. Hooks still run and still block in this mode.

To keep Claude Code's own permission settings:

```toml title=".ness/ness.toml"
[launch]
permissions = "client"
```

## Branches land as one squashed commit

`ness integrate` folds a session's branch into its target as one commit, after the **gate**, the
project's check command, passes. The reason: the target's history gets one commit per landed
result, with a Conventional Commit subject. `ness integrate` refuses a squash without a
`--subject` in that form.

The body lists the subject of every folded commit, each between `BEGIN_NESTED_COMMIT` and
`END_NESTED_COMMIT` lines. release-please reads each of them as its own change, so the changelog
keeps every one.

```toml title=".ness/ness.toml"
[integrate]
mode = "merge"       # squash (default), merge, rebase or pr
push = false         # push the target after a successful fold
```

Next: [Limits](./limits.md).
