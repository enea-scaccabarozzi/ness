---
title: Quickstart
description: Set ness up in a repository and run your first quick session, for about $0.15 of model use.
---

This page takes a new repository to a finished session. The session adds one script, lands it
on `main` through the gate, and reports back. The verified run cost $0.14.

You need the tools from [Install](./install.md). Run every command in a herdr pane.

In a terminal, ness draws a rich view with colour and symbols. The samples on this page are the
plain renderer's output. Set it for this shell so your output matches:

```sh
export NESS_UI=plain
```

## 1. Create a repository

```sh
mkdir acme && cd acme
git init -b main
git commit --allow-empty -m "chore: start"
```

An existing repository works too. Start from a clean working tree on the base branch.

## 2. Initialise ness

```sh
ness init --gate 'true' --yes
```

```text title="Output"
create .agents/skills/ness-entry/SKILL.md
create .agents/skills/ness-launch/SKILL.md
create .claude/agents/ness-design.md
create .claude/agents/ness-review.md
create .claude/agents/ness-sweep.md
create .claude/agents/ness-verify.md
create .claude/settings.json:autoCompactWindow
create .claude/settings.json:env.CLAUDE_CODE_SUBAGENT_MODEL
create .claude/settings.json:hooks.SessionStart[ness hook session-start]
create .claude/settings.json:hooks.PreToolUse[ness hook pre-tool]
create .claude/skills
create .gitignore#core/ignore
create AGENTS.md#core/router
create CLAUDE.md
```

The [gate](../why/defaults.md#branches-land-as-one-squashed-commit) is the command `ness
integrate` runs before it lands a branch. `true` always passes, which is enough for this page.
In a real project, use your test command, for example `--gate 'make test'`.

Without `--yes`, `ness init` asks for what it cannot infer.

`ness init` also writes `.ness/ness.toml`:

```toml title=".ness/ness.toml"
ness = "0.x"
project = "acme"
base = "main"
gate = "true"
```

## 3. Commit the harness

```sh
git add -A
git commit -m "chore: add ness"
ness sync --check
```

`ness sync --check` exits 0 and prints nothing when the rendered files match the config. The
`ness = "0.x"` line pins the minor line of the binary you ran, as in [Install](./install.md#check-the-install).

## 4. Install the worker

The **worker** is a background process per project. It wakes a session when a message arrives
for it, and it tells you when a session needs you.

```sh
ness worker install
```

```text title="Output"
installed /home/you/.config/systemd/user/ness-acme.service (ExecStart=/home/you/go/bin/ness worker run); check with ness doctor or systemctl --user status ness-acme
```

## 5. Check the setup

```sh
ness doctor
```

```text title="Output"
ness 0.x.y at /home/you/go/bin/ness
ok	binary	0.x.y at /home/you/go/bin/ness, pin 0.x
ok	hooks-binary	hooks resolve /home/you/go/bin/ness
ok	worker-binary	unit runs /home/you/go/bin/ness
ok	herdr-version	0.9.3
ok	client-version	claude 2.1.316
…
ok	sync-drift	generated files are current
…
warn	operator-notify	none configured: the operator is not told when a session needs them	fix: set [notify] operator = NAME to a channel of [notify.channels] in .ness/ness.toml, with notify in features
ok	secrets	no references in .ness/ness.toml
ok	jev	policy declared
```

`ness doctor` exits 0 when no check fails. The `operator-notify` warning stays until you set up a
phone channel, which [Your first week](./first-week.md) covers.

> [!NOTE]
> Right after `ness worker install`, `doctor` can report `fail worker not running (no
> heartbeat)` for a few seconds. Run it again.

## 6. Trust the repository in Claude Code

Claude Code asks once per folder whether you trust it. A session cannot answer that dialog, so
answer it yourself before the first session.

```sh
claude
```

Claude Code shows `Quick safety check: Is this a project you created or one you trust?`. The
selection starts on `No, exit`. Choose `Yes, I trust this folder` and press Enter.

Quit Claude Code with Ctrl+C twice. Trusting the repository root also covers the worktrees ness
creates under it.

If you skip this step, `ness open` stops after one minute with exit 4:

```text title="Output"
ness: session s-ece98f9fb6bb did not report ready within 1m0s (stalled launch); check its tab, then run ness open --resume s-ece98f9fb6bb. Claude Code is asking whether to trust /home/you/acme/.worktrees/hello-ece98f9fb6bb; accept it in the session's tab (trusting the project root covers its worktrees), then ness open --resume s-ece98f9fb6bb
```

Accept the dialog in the session's tab, then run the `ness open --resume` command it prints.

## 7. Write a brief

A **brief** is the task you give a session, in a file. The session reads it first.

```sh
cat > ../brief.md <<'EOF'
Add a script `hello.sh` at the repository root that prints `hello, world`.
Make it executable. Integrate it, then send the checkpoint.
EOF
```

## 8. Open the session

```sh
ness open --root --scope hello --tier economy --size quick --brief ../brief.md
```

```text title="Output"
opened s-e852f9ffec34 scope hello branch session/hello-e852f9ffec34 record docs/sessions/hello tier economy (explicit)
```

The flags:

| Flag | Meaning |
|---|---|
| `--root` | Opens a [root](../why/defaults.md), which answers to you. |
| `--scope hello` | The part of the product it owns. It names the branch and worktree. |
| `--tier economy` | The [rung](../why/defaults.md#the-model-ladder) of the model ladder: here Sonnet at medium effort. |
| `--size quick` | One deliverable, one sitting, no plan file. See [Sizes](../concepts/sessions.md#sizes). |

A herdr tab named `hello` opens with Claude Code in it. You can watch the session work there.
In normal use you do not run `ness open --root`: you ask the agent in a chat and it promotes the
conversation to a root. [Sessions](../concepts/sessions.md#promoting-a-conversation) explains it.

## 9. Wait for the checkpoint

The session works in `.worktrees/hello-<id>/`, runs the gate and lands its branch on `main`.
Then it writes a [checkpoint](../why/defaults.md#checkpoints-say-done-now-next) in its tab. In the
verified run that took about a minute.

```sh
ness status
```

```text title="Output"
s-e852f9ffec34	hello	economy quick	done	ctx 53k	idle 0m	waits 0
probes: none (feature off)
worker: 0.x.y heartbeat 1s
```

`done` means the work is integrated. Read the checkpoint in the `hello` tab. In the verified
run, the root filed this text:

```markdown title="Checkpoint"
## Done
Added executable `hello.sh` (prints `hello, world`); verified by running it. Integrated into main at 79b065b.
## Now
Nothing running.
## Next
Close the session when you like.
```

The root writes the checkpoint in its chat and files the same text as an item addressed to you.
The item is the record, and it triggers the phone notification once you set one up. Sessions
read items with `ness inbox`; you read the chat.

## 10. Check the result

```sh
git log --oneline
./hello.sh
```

```text title="Output"
79b065b feat(hello): add hello.sh
bfce297 chore: add ness
99b889b chore: start
hello, world
```

The commit also holds the session record, `docs/sessions/hello/brief.md` and `report.md`. The
report says what was delivered, the evidence, and the lessons.

## 11. Read the cost

```sh
ness stats
```

```text title="Output"
session	sessions	turns	compactions	context	input	output	cache_read	cache_write	cost
s-e852f9ffec34	1	2	0	53936	12	849	235211	28181	$0.14
total	1	2	0	53936	12	849	235211	28181	$0.14
prices	2026-09-25
```

`ness stats` reads the session transcripts. The cost is computed from the price table dated on
the last line.

## 12. Close the session

A root closes only on your word. From your terminal:

```sh
ness close s-e852f9ffec34
```

```text title="Output"
closed s-e852f9ffec34 (hello)
```

ness removes the worktree. The `hello` tab stays open: close it in herdr. `ness status --all`
still lists the session, as `closed`.

## Remove the worker

Keep the worker installed for a project you use. To remove it from this one:

```sh
ness worker uninstall
```

```text title="Output"
uninstalled /home/you/.config/systemd/user/ness-acme.service
```

Next: [Your first week](./first-week.md).
