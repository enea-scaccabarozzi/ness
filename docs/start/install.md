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

## Install with the script

```sh
curl -fsSL https://raw.githubusercontent.com/enea-scaccabarozzi/ness/main/install.sh | sh
```

```text title="Output"
ness 0.2.0 installed to /home/you/.local/bin/ness
```

The script installs the newest stable release (R-UPD-093). It detects the OS and arch, downloads
the archive and `checksums.txt` from the GitHub release, and checks the sha256 before it writes
anything. It needs `curl` or `wget`, `tar`, and `sha256sum` or `shasum`.

To install the newest release candidate instead, pass `--channel rc`:

```sh
curl -fsSL https://raw.githubusercontent.com/enea-scaccabarozzi/ness/main/install.sh | sh -s -- --channel rc
```

```text title="Output"
ness 0.3.0-rc.1 installed to /home/you/.local/bin/ness
```

`NESS_VERSION` installs one version, and `NESS_INSTALL_DIR` replaces `~/.local/bin`:

```sh
curl -fsSL https://raw.githubusercontent.com/enea-scaccabarozzi/ness/main/install.sh | NESS_VERSION=0.2.0 NESS_INSTALL_DIR="$HOME/bin" sh
```

When the install directory is not on your `PATH`, the script says so, as here:

```text title="Output"
ness 0.2.0 installed to /home/you/bin/ness
note: /home/you/bin is not on your PATH; add it: export PATH="/home/you/bin:$PATH"
```

A checksum mismatch fails with `install.sh: checksum mismatch for <archive>`, and an OS or arch
without a build fails with `install.sh: no ness build for <os>/<arch>`. Neither writes the binary.

## Other ways to install

Each of these installs the same release. Use one way per machine: `ness self-update` refuses to
replace a binary that mise or `go install` manages, and prints that tool's command instead
(R-UPD-045).

### With mise

```sh
mise use -g github:enea-scaccabarozzi/ness@0.2.0
```

mise downloads the release archive, checks its checksum, and makes it your global `ness`.

### From source

```sh
go install github.com/enea-scaccabarozzi/ness/cmd/ness@v0.2.0
```

`go install` prints only `go: downloading …` lines, and only for modules it has not cached. It
puts the binary in `$(go env GOPATH)/bin`, which is `~/go/bin` unless you changed it. Add that
directory to your `PATH`:

```sh
export PATH="$(go env GOPATH)/bin:$PATH"
```

### From the archive

Every release attaches `ness_<version>_linux_amd64.tar.gz`, `ness_<version>_linux_arm64.tar.gz`
and `checksums.txt`. Download the archive for your arch and the checksums from the
[releases page](https://github.com/enea-scaccabarozzi/ness/releases), then check and install it:

```sh
sha256sum --ignore-missing -c checksums.txt
tar -xzf ness_0.2.0_linux_amd64.tar.gz ness
install -m 0755 ness ~/.local/bin/ness
```

```text title="Output"
ness_0.2.0_linux_amd64.tar.gz: OK
```

### The version ceiling

Only `ness self-update` checks that every project on the machine can run the new version (the
ceiling, R-UPD-046). The script, mise and `go install` install what you ask for. When a project
then pins a version the new binary cannot run, `ness doctor` fails its `binary` check, and the
hooks of a session in that project block with the fix (R-UPD-020). See
[Compatibility](../concepts/compatibility.md).

## Put it on your PATH

The Claude Code hooks that ness installs call `ness` by name. The binary they find on `PATH` must
be the one you run. Add the install directory to your shell profile, so new terminals get it:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

## Check the install

```sh
ness --version
```

```text title="Output"
ness version 0.2.0+c28510c39edf
```

On these pages, `0.x.y` stands for the version you installed, and `0.x` for its minor line. The
part after `+` is the commit the binary was built from. A build from a checkout with uncommitted
changes ends in `.dirty`.

Next: [Quickstart](./quickstart.md).
