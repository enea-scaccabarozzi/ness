> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Probe labels: parallel releases with overlapping windows

Source: docs/discovery/findings/field-2/releases.md section 4. A label is free text on a probe row;
identity becomes (path, owner, label). Empty label is today's behaviour.

1. Store: migration 8 (`label`, `probes_live` over path, owner, label); `StartProbe`, `ProbeOf`,
   `moveProbes` respect the label; `ValidLabel`. Tests first: two labels, restart keeps the other,
   adopt/merge by label, migration keeps old rows.
2. Floor and worker: `NESS_PROBE_LABEL`; label in wake body, auto-post header, thread key
   `probe:<path>:<label>` (`probe:<path>` when empty). Tests: overlapping windows end independently,
   per-label env, posts and threads.
3. CLI: `probe start --label`, `probe stop [--label]` (refuses when ambiguous, lists labels), label
   in `status` text, rich table and JSON, verdict thread. Testscript.
4. Docs: R-PRB-084 amended, R-PRB-085..088 added, example-release skill, godoc.

Deferred: parallel floors, per-instance `--window`, pinning the floor cwd to the release commit,
any supersession (the owner stops what a newer release replaces).
