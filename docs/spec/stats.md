> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Stats (STAT)

Package: `internal/stats`, cli `stats`. Sources: §2 (client contract: transcript → context and
usage), §3 (model choice feedback), §5 (`ness stats`), §12 (router reason recorded for stats),
§13 (deferred until stats data), roadmap L5, D8, D16.

- **R-STAT-001** `ness stats` MUST compute cost and context size from the Claude Code transcripts
  bound to ness sessions by session id, pricing with a table embedded in the binary that carries
  its date, and rounding cost to cents.
  Verify: unit — fixture transcripts for two sessions → per-session tokens and cost equal
  hand-computed values to the cent; output names the price-table date.
- **R-STAT-002** `ness stats` MUST read only transcripts whose id is bound to a session of this
  project.
  Verify: testscript — an unbound transcript in the client's directory is not counted.
- **R-STAT-003** `ness stats` MUST be read-only.
  Verify: testscript — transcripts, store and tree unchanged.
- **R-STAT-004** `--since` MUST accept a date (`YYYY-MM-DD`, RFC 3339) or a duration (Go syntax plus
  `d`) and restrict the figures to entries at or after that time.
  Verify: unit — `2026-10-01` and `7d` each exclude older entries; `yesterday` → exit 2.
- **R-STAT-005** `--by session|tier|role|rung|tool` MUST group the figures by that dimension; any
  other value MUST be refused.
  Verify: testscript — each grouping sums to the same total; `--by colour` refused.
- **R-STAT-006** `--json` MUST output the same figures as the text output.
  Verify: testscript — totals equal.
- **R-STAT-007** The transcript reader MUST NOT panic on any input, and MUST skip and count lines
  it cannot parse.
  Verify: fuzz — arbitrary JSONL; reported skipped-line count equals injected bad lines.
- **R-STAT-008** Running the stats reader over the saved discovery transcripts MUST reproduce the
  discovery figures (`docs/discovery/data/cost.txt`, `tokens.txt`) (L5 exit evidence); the
  fixture lives in a gitignored local directory and the test skips when it is absent, while CI
  uses synthetic transcripts with known figures.
  Verify: unit — local run matches within the figures' rounding; CI: synthetic golden passes.
- **R-STAT-009** When a router assigned the tier, its recorded reason MUST be available per
  session in stats output.
  Verify: unit — fixture session with a reason → present in `--json`.

## Decided details

- **R-STAT-010** Context size MUST be the prompt tokens of the last API call of each turn;
  compactions MUST be counted from the transcript's compaction markers.
  Verify: unit — fixture with two compactions → count 2; context per turn equals the last call's
  prompt tokens.
- **R-STAT-011** `--by tool` MUST report `tool_result` bytes per tool.
  Verify: unit — fixture with known Read/Bash result sizes → equal byte sums.
- **R-STAT-012** Stats MUST include closed sessions, taking tier and role from the store's
  closed-session rows (R-SESS-065).
  Verify: testscript — closed session appears under its tier in `--by tier`.
- **R-STAT-013** v1 stats MUST read Claude Code transcripts only; Codex sessions MUST be listed as
  not counted.
  Verify: unit — a codex session appears with "not counted" and no figures.
- **R-STAT-020** `--by rung` MUST group a session's own turns under its tier and each subagent's
  under the ladder rung of its meta model/effort (`off-ladder` when none matches); `--by role`
  MUST put subagent turns in a `subagent` row; no turn is counted twice. Every row and the total
  MUST end with `"spawns": {"allowed","below","escalated","refused","would_refuse","fork"}`
  (zeros when none); the total counts sessions without a transcript.
  Verify: testscripts `stats_rung`, `stats_json`, `hook_pretool_spawn`; unit `meta_test`.
- **R-STAT-021** A subagent's agent type, model and effort MUST be read from the
  `agent-*.meta.json` beside its transcript; a missing model or effort stays empty.
  Verify: unit `meta_test`.
