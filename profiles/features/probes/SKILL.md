---
name: ness-probes
description: Probes are scheduled questions about the running world, owned by a session; the worker runs the floor, the woken agent judges within the scope.
---

# ness probes

A probe is one Markdown file: frontmatter (`every` or `once`; optional `jitter`, `window`,
`floor`, `expect`, `confirm`, `timeout`, `wake`, `sample`, `notify`, `channel`) and a body,
the scope you check when woken. The floor is a command: exit 0 is GREEN, 1 is RED, anything
else or a timeout is UNCLEAR; with `expect`, the last line's `key=value` pairs must satisfy it
or the run is VACUOUS.

- `ness probe start PATH... [--keep-window] [--label X]` — register files or a directory; you own them. A label (a release sha, a version) lets the same file run once per label with its own window; the floor sees `$NESS_PROBE_LABEL`. Starting the same file and label again restarts it.
  Edits reload; a paused probe at the same path is adopted.
- `ness probe run PATH [--json]` — run the floor now, print the state and the evidence, record
  nothing: the authoring loop.
- `ness probe verdict ITEM --state green|red --note TEXT [--probe-bug]` — answer a wake.
- `ness probe stop PATH [--label X] --note TEXT` — stop your probe; several live labels at one path refuse until you name one.

A wake is a request in your inbox carrying the scope and the capped, redacted evidence. Judge
it, then answer with `ness probe verdict`. `--probe-bug` records a defect in the probe, posts
nothing and flags it `needs-edit` until the file changes. When you close, your probes move to
your manager; without a live owner a probe pauses and shows red in `ness status`.

## Sizing a probe
- Floor first when a cheap command separates RED from GREEN, and give it a positive control
  (a case you know is RED) before you trust its first GREEN.
- Raise `sample` as the floor's coverage falls: the less the command sees, the more GREENs you
  look at yourself.
- Skip the floor when no command can judge, or when your own look is cheaper than writing one.
- Keep the scope narrow and named: one question per file.
- Set `jitter` when the risk depends on the hour, so the runs do not always land at the same
  minute.

## Enabling the feature
The worker reads `features` and the channel configuration at its start: after enabling
`probes`, or changing a channel, run `ness sync`, then restart the worker
(`systemctl --user restart ness-<project>`), or no floor will run.
