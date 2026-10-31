# Backlog

Requirements that are known and owned but not yet scheduled. Each entry names when it becomes
due.

## Field run 2 leftovers (cli-ux, 2026-10-11)

- **Gate lock beyond integrate.** The project gate lock (R-SESS-145) covers `ness integrate`
  only; a hand-run `make validate` in a child still competes. Six parallel gates on a 4-core box
  made every gate time out; the root had to queue integrations by hand.
- **Shared tunnel** (an infrastructure lab, F10): parallel children need a reference-counted tunnel.
