---
title: "ness"
description: "Install and run an AI coding-agent harness in this project"
---

## Usage

```text
ness [flags]
```

## Flags

| Flag | Default | Description |
|---|---|---|
| `--ui string` | `auto` | renderer: auto, rich or plain (also NESS_UI) |
| `-v, --version` |  | version for ness |

## Commands

| Command | Description |
|---|---|
| [ness close](./close.md) | Close a child you manage, as the operator a root, or your own root with the operator's words |
| [ness contract](./contract.md) | Freeze files, open one for amendment, or check every frozen file (the gate) |
| [ness contract amend](./contract-amend.md) | Open frozen files for change; the reason goes to decisions.md, one line each |
| [ness contract check](./contract-check.md) | Verify every frozen file; exit 1 naming each one that differs |
| [ness contract freeze](./contract-freeze.md) | Record each file's blob sha in your contract.lock |
| [ness doctor](./doctor.md) | Check this installation and project, read-only; print a fix for every failure |
| [ness hook](./hook.md) | Entry point for client hooks: session-start, pre-tool |
| [ness inbox](./inbox.md) | List your open items, one line each; --show ITEM... or --unread prints them in full |
| [ness init](./init.md) | Create .ness/ness.toml and render the harness into this repository |
| [ness integrate](./integrate.md) | Fold a branch into its target through the gate, push if configured, record it |
| [ness open](./open.md) | Launch a child or a root session, or resume one |
| [ness pick](./pick.md) | Show the rung the selector picks for a task, or the ladder |
| [ness post](./post.md) | Post one Markdown note to a configured channel, rendered for its platform |
| [ness probe](./probe.md) | Start, run, answer or stop probes: scheduled questions about the running world |
| [ness probe run](./probe-run.md) | Run the floor now and print the state and evidence; record nothing |
| [ness probe start](./probe-start.md) | Register probe files or a directory of them; you own them |
| [ness probe stop](./probe-stop.md) | Stop your probe at PATH (--label X when it runs under several labels) and record why |
| [ness probe verdict](./probe-verdict.md) | Answer a probe wake with green or red; posts it when the probe is judged |
| [ness reply](./reply.md) | Answer an item and dispose it in one step |
| [ness secret](./secret.md) | Keep the values ness.toml references as secret:NAME |
| [ness secret list](./secret-list.md) | List the stored names, marking those ness.toml references |
| [ness secret set](./secret-set.md) | Store a secret read from stdin (never from the command line) |
| [ness secret unset](./secret-unset.md) | Remove a stored secret |
| [ness self-update](./self-update.md) | Install a newer ness release in place of this binary |
| [ness send](./send.md) | Send a message to your manager, one of your children, another root (as a root) or the operator |
| [ness size](./size.md) | Show or change the work size of a root |
| [ness stats](./stats.md) | Tokens, context, compactions, tool bytes and cost of this project's sessions, from their transcripts |
| [ness status](./status.md) | The session tree: tier, phase, context, idle time, obligations; probes; the worker |
| [ness sync](./sync.md) | Render core, features and profiles into this repository |
| [ness upgrade](./upgrade.md) | Move this project to the running binary: migrate, sync, bump the pin and commit |
| [ness worker](./worker.md) | The project's worker: run it here, or install it as a systemd user unit |
| [ness worker install](./worker-install.md) | Write and start the systemd user unit ness-\<project>.service |
| [ness worker run](./worker-run.md) | Run the worker in this terminal: one process per project, a pass every 5 s |
| [ness worker uninstall](./worker-uninstall.md) | Stop, disable and remove the unit |
