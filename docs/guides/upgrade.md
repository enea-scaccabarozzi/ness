---
title: Upgrade ness
description: Build a new binary, re-render each project with ness sync, and restart its worker.
---

ness has no self-update yet. An upgrade is three manual steps: build the binary, sync each
project, restart each worker. The project's pin in `.ness/ness.toml` says which versions may run
it.

## Prerequisites

- A checkout of the ness source and the Go toolchain its `go.mod` names.
- The installed binary at `~/.local/bin/ness`, first on your `PATH`.

## Build the binary

From the ness source checkout:

```sh
go build -o ~/.local/bin/ness ./cmd/ness
ness --version
```

```text title="Output"
ness version 0.1.0+b13684c9f547
```

A build from a checkout with uncommitted changes ends in `.dirty`.

## Sync each project

1. In each project, see what the new binary would change.

   ```sh
   ness sync --check
   ```

   Exit 0 means nothing changes. Exit 1 lists the files that would.

2. Render the changes and commit them.

   ```sh
   ness sync
   ```

   An exit 3 is a conflict with a file you edited. Merge the `.ness-new` file into it, then run
   `ness sync --keep PATH`. See [Files ness writes](../reference/files.md).

## Restart the worker

```sh
systemctl --user restart ness-PROJECT.service
```

The worker also checks its binary every 60 seconds. When the version changed, it exits, and
systemd starts the new one.

### Check that it worked

```sh
ness doctor
```

The `binary`, `hooks-binary` and `worker-binary` checks name the new binary, and `worker` reads
`running VERSION, heartbeat 0s ago`.

## When the pin refuses the binary

`ness = "0.1"` in `.ness/ness.toml` accepts any `0.1.x`. A binary outside the pin is refused:

```text title="Output (exit 4)"
ness: this project pins ness 0.2.x (ness = "0.2"); the running binary is 0.1.0 (binary: /home/you/.local/bin/ness)
```

Install a matching binary, or raise the pin once the project is ready for the new version.

Next: [Configuration reference](../reference/config.md).
