# Backlog

Requirements that are known and owned but not yet scheduled. Each entry names when it becomes
due.

## Update path (due once the repository is public, after the field feedback is in)

How an installed ness and the projects using it move to a new release: fetching the binary
from the public GitHub releases, `ness sync` in each project, restarting each project's worker,
what happens to sessions that are mid-turn, and how a project pinned to an older `ness = "X.Y"`
is told. It needs its own design session; do not grow it piecemeal.

Until then the update is manual, by the operator or on their request:

1. build and install the binary from main (`go build -o ~/.local/bin/ness ./cmd/ness`);
2. in each project, `ness sync --check` (and `ness sync` when it reports changes, committed in
   that project);
3. restart each project's worker (`systemctl --user restart ness-<project>.service`), so it runs
   the new binary.

## Field run 2 leftovers (cli-ux, 2026-10-11)

- **Gate lock beyond integrate.** The project gate lock (R-SESS-145) covers `ness integrate`
  only; a hand-run `make validate` in a child still competes. Six parallel gates on a 4-core box
  made every gate time out; the root had to queue integrations by hand.
- **Binary and store skew.** A branch build migrates the shared store (schema 6) while the
  installed binary (schema 5) keeps running hooks and the worker; and a branch with new config
  keys is refused by the installed binary. Children had to build their own binary. Part of the
  update-path design above.
- **Shared tunnel** (an infrastructure lab, F10): parallel children need a reference-counted tunnel.
