---
title: Install
description: The tools ness needs, and how to put the ness binary on your PATH.
---

ness is one Go binary. It runs on Linux, drives Claude Code inside herdr tabs, and runs its worker
as a systemd user service.

## Prerequisites

| Tool | Version | Why ness needs it |
|---|---|---|
| Linux | amd64 or arm64 | ness refuses every other OS: `ness v1 runs on Linux only`. |
| systemd | a user session (`systemctl --user`) | The worker runs as a user unit. |
| git | any recent release | Sessions get worktrees and branches. |
| herdr | 0.8.2 or newer, same major | Each session runs in its own herdr tab. |
| Claude Code | 2.1.294 or newer, same major | The agent client. |
| Go | 1.26 or newer | Only to build ness from source. |

ness checks the herdr and Claude Code versions before it opens a session, and `ness doctor`
reports them. A newer minor or patch release is accepted. A different major is refused.

Run the commands on these pages in a herdr pane. `ness open` starts each session in a new herdr
tab, and it needs a running herdr to do that.

## Install from source

```sh
go install github.com/enea-scaccabarozzi/ness/cmd/ness@latest
```

`go install` prints nothing on success. It puts the binary in `$(go env GOPATH)/bin`, which is
`~/go/bin` unless you changed it.

> [!NOTE]
> Until the first release, `@latest` installs the newest commit on `main`. From the first
> release on, it installs the newest release. Each release also attaches prebuilt archives for
> `linux_amd64` and `linux_arm64` on its GitHub release page.

## Put it on your PATH

The Claude Code hooks that ness installs call `ness` by name. The binary they find on `PATH` must
be the one you run.

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
```

Add the line to your shell profile so new terminals get it.

## Check the install

```sh
ness --version
```

```text title="Output"
ness version 0.x.y
```

On these pages, `0.x.y` stands for the version you installed, and `0.x` for its minor line. A
build from a checkout adds the commit, as in `0.x.y+b13684c9f547`.

Next: [Quickstart](./quickstart.md).
