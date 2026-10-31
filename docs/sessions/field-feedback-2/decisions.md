> [!NOTE]
> Archive: altered for the public release. Names of private projects were replaced; the content is otherwise as written.

# Decisions: field-feedback-2

## FF2-D1: tier standard, scope field-feedback-2 (2026-10-20)
Same tier as field-feedback (opus/medium). Evidence goes to the private docs/discovery/findings/field-2/.

## FF2-D2: mechanical fixes proceed without a design round (2026-10-20)
The operator called alerts and parallel releases mechanical fixes. Once grounded they go to plan and TDD directly
(no-rebrainstorm rule); contracts and execution modes come back to the operator as a report first.

## FF2-D3: no release supersession in ness (2026-10-20)
Probe labels let one file run once per release (R-PRB-085..088). Unlike the production backend's harness, ness never ends an older
release's window when a newer one starts; the owner stops what the newer release replaces.

## FF2-D4: alert fixes folded to main at 569e3a7 (2026-10-20)
Eight causes fixed with tests, then eleven review findings (review-field2.md). Binary installed, the infrastructure lab, the research lab and
Ness workers restarted on it. Unconfirmed for lack of a capture: the real limit screen, a permission prompt screen, and
herdr's "blocked" during compaction; the new screen log on first stall exists to capture them next time.

## FF2-D5: operator answers to checkpoint m-xk5sjm23 (2026-10-20)
- Sizes as restated, built now and judged in use. Quick roots close by option B (done state; the operator's words still close).
- ness-monitor round 2 is final.
- Contracts get project-defined formats instead of `keep-lines` (FF2-D6). Good formats may become builtins later.
- The production backend cuts over now, once all ness changes have landed. The research lab follows the same pattern: a prepared branch plus a
  guide handed to its live root, never merged under it.
- Gate scope question re-asked in chat with more context.

## FF2-D6: contract formats are project-defined unit extractors (2026-10-20)
`[contracts.formats.NAME]` has `match` (globs) and `units` (a command). Ness runs the command at the repo root, with the
file's text on stdin and its path as the last argument. The command prints JSON lines `{"id": ..., "body": ...}`, and ness
locks the sha of each body under its id. Check: every locked id is present with the same body; new ids are free; a
missing or changed id fails; an extractor that fails, prints invalid JSON or repeats an id fails closed. The command's
script, when it is a repository file, is frozen whole-file by the same lock, so a branch cannot weaken its own judge.
Formats are matched in config order before the builtins (scenario for .feature, prefix, whole-file). The builtins stay
built in; a format that proves itself can graduate into ness as a builtin extractor.

## FF2-D7: probes floors run in parallel; gate scope stays project-side (2026-10-20)
The operator asked for parallel probe execution before the release. NESS_BASE ships; consuming projects adopt it on their own.

## FF2-D8: batched rollout authorized (2026-10-20)
Once formats, sizes and parallel probes are on Ness main, roll out as agreed with no further go: install the binary and
restart the workers; the infrastructure lab sync; the research lab and the production backend get their branches and guides through their live roots.

## FF2-D9: the production backend cutover on 2026-10-21 from 08:15Z, root merges the port (2026-10-20)
The operator chose option A, after the zero-cross verdict. Wake: transient systemd user timer ba-cutover-wake.timer (08:15:00Z)
runs ~/.local/share/ness-wakes/ba-cutover.sh, which prompts pane w6:pR or raises a herdr notification if the pane is gone.
The timer does not survive a reboot. The chore/ness-port push waits for the root to pull develop (stale local develop made
contract-check count origin's legacy locks as branch changes).
