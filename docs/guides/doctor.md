---
title: Diagnose a project
description: Run ness doctor, read each check, and apply the fix it prints.
---

`ness doctor` checks the installation and the project, and changes nothing. Every check prints
one line: `ok`, `warn` or `fail`, a detail and, when it is not ok, the fix to run. Doctor never
runs the fix for you.

## Prerequisites

- A project checkout. Outside one, doctor prints the binary and refuses:

  ```text title="Output (exit 4)"
  ness 0.1.0+b13684c9f547 at /home/you/.local/bin/ness
  ness: /tmp is not inside a git repository; run ness from a project checkout
  ```

## Run it

```sh
ness doctor
```

```text title="Output (exit 1)"
ness 0.1.0+b13684c9f547 at /home/you/.local/bin/ness
ok	binary	0.1.0 at /home/you/.local/bin/ness, pin 0.1
ok	hooks-binary	hooks resolve /home/you/.local/bin/ness
warn	worker-binary	no unit at /home/you/.config/systemd/user/ness-acme.service (worker not installed)	fix: ness worker install
ok	herdr-version	0.9.3
ok	client-version	claude 2.1.316
ok	tier-models	7 tiers resolve for claude
ok	skill-shadowing	no global skill shadows a project skill
ok	profile-tools	0 required tools present
fail	worker	not running (no heartbeat)	fix: ness worker install
ok	worker-clients	no store yet
ok	systemd	user session running
ok	sync-drift	generated files are current
ok	dead-routes	no store yet
ok	gone-panes	no store yet
ok	stray-worktrees	no .worktrees directory
ok	probes	feature off
warn	operator-notify	none configured: the operator is not told when a session needs them	fix: set [notify] operator = NAME to a channel of [notify.channels] in .ness/ness.toml, with notify in features
ok	secrets	no references in .ness/ness.toml
ok	jev	policy declared
```

This is a fresh project: `worker-clients`, `dead-routes` and `gone-panes` read the store, and
there is none until a command such as `ness status` creates it.

Doctor exits 1 when any check fails, and 0 otherwise, warnings included. Every check runs, even
after a failure. `ness doctor --json` prints one object, with `name`, `status`, `detail` and
`fix` for each check.

## Read the checks

| Check | Looks at | Common fix |
|---|---|---|
| `binary` | The running binary's version against the pin in `ness.toml` | Install a matching binary, or change the pin |
| `hooks-binary` | The binary the client hooks resolve | Put this binary first on `PATH` |
| `worker-binary` | The binary the worker unit runs | `ness worker install` |
| `herdr-version` | The herdr version, against the tested ones | `herdr update` |
| `client-version` | The client version, against the tested ones | `claude update` (or `codex update`) |
| `tier-models` | Every rung's model, against the models the client lists | Edit `[tiers.NAME]` |
| `skill-shadowing` | Global skills with the name of a project skill | Rename or remove the global skill |
| `profile-tools` | Tools named in a profile's `requires_tools` | Install the tool, for example with mise |
| `worker` | The worker's heartbeat: a fail when older than 60 seconds | `ness worker install`, or `systemctl --user restart ness-PROJECT` |
| `worker-clients` | Whether the worker can deliver to the configured client | Run sessions on Claude, or read mail with `ness inbox` |
| `systemd` | The systemd user session the worker needs | `ness worker run` in a terminal, without systemd |
| `sync-drift` | What `ness sync --check` would find, and hand edits | `ness sync`, or keep the edit |
| `dead-routes` | Open items addressed to closed or unknown sessions | `ness worker run` (it retires them), or `ness reply ITEM --as declined` |
| `gone-panes` | Open sessions whose terminal pane is gone | `ness open --resume ID` |
| `stray-worktrees` | Worktrees under `.worktrees/` with no session | `git worktree remove PATH` |
| `probes` | Active probes without a live owner | `ness probe start PATH`, from a live session |
| `operator-notify` | The channel `[notify] operator` names | Set `[notify] operator` |
| `secrets` | Every `secret:` and `env:` reference | `ness secret set NAME`, or export the variable |
| `jev` | The Jev key and endpoint, when the policy is `jev`. Always listed last. | Set `models.jev.key` or `models.jev.endpoint` |

## Check that it worked

Apply the fix of each `fail` line and run `ness doctor` again. Once the worker runs, the worker
line reads `running VERSION, heartbeat 0s ago`. Doctor exits 0 once no line is `fail`.

Next: [Upgrade ness](./upgrade.md).
