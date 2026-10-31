> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Resolutions of open points

Resolved by the program root on 2026-10-07 (r1 → r2 of the spec). One line each: OP-id →
resolution → requirement ids that carry it. No open points remain.

## core-config
- OP-CFG-1 → exits 0 ok, 1 check failed (sync --check, doctor, contract check, probe run non-GREEN), 2 usage, 3 conflict, 4 refused → R-CFG-035, R-CFG-036, R-CFG-038.
- OP-CFG-2 → 0.x pin accepts exactly that minor (any patch); ≥1.0 same major and binary minor ≥ pin; older binary refuses → R-CFG-011, R-CFG-012.
- OP-CFG-3 → full schema drafted (ness, project, base, gate, sessions, client, profiles, features, experimental, [worktree], [integrate], [contracts], [probes], [notify.channels.NAME], [tiers.NAME], [models], [launch], [worker]) → R-CFG-040..046.
- OP-CFG-4 → unknown keys refused → R-CFG-043.
- OP-CFG-5 → TTY: interactive prompts; non-TTY without --yes refuses listing missing flags → R-CFG-051, R-CFG-052.
- OP-CFG-6 → outside a project only init/version/help run; hook exits 0 silently → R-CFG-053.
- OP-CFG-7 → disabled-feature commands refuse naming the feature → R-CFG-054.
- OP-CFG-8 → `sessions` configurable, default `docs/sessions` → R-CFG-041.
- OP-CFG-9 → `project` is a required key; init fills from the directory name → R-CFG-040, R-CFG-050.
- OP-CFG-10 → Codex requires `experimental = ["codex"]`, else refuse → R-CFG-044.
- OP-CFG-11 → --json on status, inbox, doctor, stats, probe run, sync --dry-run; `"schema": 1`; field schema frozen by golden tests → R-CFG-032, R-CFG-037.

## sync
- OP-SYNC-1 → comment-syntax markers per file type → R-SYNC-040.
- OP-SYNC-2 → file granularity: whole-file `.ness-new` + `--keep PATH` → R-SYNC-045, R-SYNC-051, R-SYNC-052.
- OP-SYNC-3 → `--keep` records and deletes `.ness-new` → R-SYNC-017.
- OP-SYNC-4 → conflict exit 3 wins over 1 → R-SYNC-030, R-SYNC-031, R-CFG-038.
- OP-SYNC-5 → deleted seeded file never recreated → R-SYNC-081.
- OP-SYNC-6 → real `.claude/skills` dir is a conflict; refuse to write into it → R-SYNC-080.
- OP-SYNC-7 → markers `ness:begin <owner>/<block>` → R-SYNC-040, R-SYNC-082.
- OP-SYNC-8 → adoption guidance via release notes/docs only, no heuristic → R-SYNC-083.
- OP-SYNC-9 → autoCompactWindow 265000; subagent model = economy Claude model; Codex model_auto_compact_token_limit 235000 → R-SYNC-084..086.
- OP-SYNC-10 → unowned bytes identical; JSON order/indent kept; TOML line-level → R-SYNC-050.
- OP-SYNC-11 → SHA-256, lock schema 1 → R-SYNC-087, R-SYNC-088.
- OP-SYNC-12 → notes embedded from CHANGELOG with "Running sessions" per version → R-SYNC-089.

## profiles
- OP-PROF-1 → missing `requires` is an error → R-PROF-040.
- OP-PROF-2 → `[files]` `{ own }` / `{ seed }` → R-PROF-007.
- OP-PROF-3 → `[config]`, `[features] enable`, `[atoms]` syntax → R-PROF-001, R-PROF-008, R-PROF-009.
- OP-PROF-4 → unknown slot is an error → R-PROF-041.
- OP-PROF-5 → local named like a built-in is an error → R-PROF-042.
- OP-PROF-6 → project profile overrides single slots only; seams/owned paths via ness.toml → R-PROF-022, R-PROF-023.
- OP-PROF-7 → always `local` → R-PROF-022.
- OP-PROF-8 → local profile version = content hash → R-PROF-043.
- OP-PROF-9 → name ≠ directory refused → R-PROF-044.

## store-messages
- OP-MSG-1 → pulled = listed by inbox; stops hints; open until disposed → R-MSG-036.
- OP-MSG-2 → fyi no obligation; listed until --show or reply; never blocks close → R-MSG-041.
- OP-MSG-3 → --urgent bypasses batching and quiet window, never a non-empty box → R-MSG-042, R-WRK-011.
- OP-MSG-4 → request/decision/handoff create obligations; intent otherwise a label → R-MSG-040.
- OP-MSG-5 → only roots address the operator; operator reads via root chat and inbox outside a session → R-MSG-017, R-MSG-047.
- OP-MSG-6 → a root's manager is the operator → R-MSG-013.
- OP-MSG-7 → reply arrives as a new `reply` item, no obligation → R-MSG-022, R-MSG-049.
- OP-MSG-8 → deferred stays listed, marked → R-MSG-043.
- OP-MSG-9 → `sent <id> to <to>` / `already sent <id>` → R-MSG-016.
- OP-MSG-10 → idempotency key permanent → R-MSG-014.
- OP-MSG-11 → id `m-` + 8 base32; age 5m/3h/2d; columns golden → R-MSG-044, R-MSG-045.
- OP-MSG-12 → forward-only automatic migrations → R-MSG-046.
- OP-MSG-13 → inbox outside a session shows operator items → R-MSG-047.

## sessions
- OP-SESS-1 → NESS_SESSION/NESS_RUN env; operator = no NESS_SESSION → R-SESS-037, R-SESS-041.
- OP-SESS-2 → id `s-` + 12 hex; scope `[a-z][a-z0-9-]` ≤ 32 → R-SESS-042, R-SESS-043.
- OP-SESS-3 → records nested under the parent's record dir → R-SESS-022.
- OP-SESS-4 → current harness phase names; ready timeout 60 s, configurable → R-SESS-044, R-SESS-033.
- OP-SESS-5 → --scope required; title defaults to scope → R-SESS-043.
- OP-SESS-6 → `<project>-<scope>` truncated, unique vs live herdr agents, id-char suffix on collision → R-SESS-035, R-SESS-036.
- OP-SESS-7 → delete local branch only when integrated; keep on --discard/--keep-work → R-SESS-072.
- OP-SESS-8 → pr: close refuses until merged → R-SESS-071.
- OP-SESS-9 → store keeps closed sessions (amends design §5 "store row removed") → R-SESS-065, R-SESS-086, R-STAT-012.
- OP-SESS-10 → never ran = no ready mark → R-SESS-066.
- OP-SESS-11 → effort low|medium|high|xhigh|max, validated by client atom → R-SESS-017.
- OP-SESS-12 → status default active tree; --all adds closed → R-SESS-080, R-SESS-086.
- OP-SESS-13 → clean source worktree required; fast-forward target in manager's worktree, refuse if dirty → R-SESS-103, R-SESS-104.
- OP-SESS-14 → arbitrary --from/--into only roots and operator → R-SESS-101.
- OP-SESS-15 → tested versions embedded; Linux-only v1; enterprise path via spike → R-SESS-014..016.
- OP-SESS-16 → worktree.copy/link from config → R-SESS-034.
- OP-SESS-17 → --resume takes --tier/--effort → R-SESS-051.
- OP-SESS-18 → gate required; integrate refuses without it → R-SESS-105, R-CFG-040.
- OP-SESS-19 → pr close refuses while unmerged, except with `--discard` (the explicit throw-away, open to the manager as well as the operator); the report (R-SESS-061) is still required → R-SESS-071.

## worker
- OP-WRK-1 → stall 30 min default; quiet = non-placeholder text in box or operator input in pane within 60 s → R-WRK-050, R-WRK-018.
- OP-WRK-2 → pass every 5 s → R-WRK-024.
- OP-WRK-3 → resume text per client atom at reset + 60 s → R-WRK-043.
- OP-WRK-4 → no reset time: silent, recheck screen every 15 min → R-WRK-045.
- OP-WRK-5 → local time zone → R-WRK-046.
- OP-WRK-6 → per-pane notices plus usage file when present → R-WRK-047.
- OP-WRK-7 → short fixed hint text, golden; compaction `/compact <note>` → R-WRK-025, R-WRK-030.
- OP-WRK-8 → re-resolve binary every 60 s; exit 0 on version change for systemd → R-WRK-009.
- OP-WRK-9 → worker.log capped 10 MB, keep 2 → R-WRK-026.
- OP-WRK-10 → error patterns from client atom (API error, overloaded, capacity, limit) → R-WRK-051.
- OP-WRK-11 → R-WRK-015's screens are synthesized in the client's rendering (prompt `❯`+nbsp or `>`, SGR 2 / 90 / grey 38;2), never copied from the private discovery (D30); error labels are fixed names (API Error, overloaded, capacity); no prompt on screen reads as an empty box and is flagged `Screen.NoPrompt` for a one-time worker.log line; notices are whole lines in the 3 non-blank lines above the input box starting with the client's phrase, never substrings of prose; a date-only reset is this year or Unknown, and a reset more than 8 days ahead is Unknown → R-WRK-044, R-WRK-014, R-WRK-015, R-WRK-051.
- OP-WRK-12 → v1 delivers only to clients with a screen reader (claude); a session of another client (codex) is skipped with one worker.log line and its mail waits in `ness inbox` → R-WRK-010, R-WRK-022.
- OP-WRK-13 → a stall, an error or an unavailable model is acted on only after two consecutive passes read it; the tab of a session whose model is unavailable is closed only for the session's own agent, unfocused with an empty box (else the manager is told of the error and the stop waits), the row stays open and the manager is told to fix the tier and `ness open --resume`; the worker never changes a model; stall, error and stop reports are fyi items from the session to its manager, one per episode in memory, so a worker restart mid-episode may send one more (accepted for v1); the worker's own notices are skipped when reading a screen → R-WRK-050, R-WRK-051, R-WRK-052, R-WRK-062, R-SESS-009, R-MOD-012.

## probes
- OP-PRB-1 → comma-separated `key op number`, all must hold → R-PRB-025.
- OP-PRB-2 → defaults jitter 0, confirm 1, timeout 60s, wake not-green, sample 0, notify none → R-PRB-080.
- OP-PRB-3 → Go durations plus `d` → R-PRB-081.
- OP-PRB-4 → confirm applies to waking and auto posts → R-PRB-037.
- OP-PRB-5 → evidence = last 4 KB stdout+stderr, redacted, also in probe run → R-PRB-039.
- OP-PRB-6 → window end stops probe; one final wake for a final verdict → R-PRB-042.
- OP-PRB-7 → --probe-bug not posted; probe flagged needs-edit until file changes → R-PRB-065.
- OP-PRB-8 → verdict states green|red only → R-PRB-082.
- OP-PRB-9 → `sh -c` from owner's worktree root; env + NESS_PROBE/NESS_SESSION → R-PRB-083.
- OP-PRB-10 → identity = repo-relative path + owner session → R-PRB-084.
- OP-PRB-11 → paused probe resumes only on a live session's probe start → R-PRB-071.

## notify
- OP-NTF-1 → slack uses `markdown` blocks, 12,000 per message, split → R-NTF-020, R-NTF-021.
- OP-NTF-2 → kind + kind keys; secrets via token_env only → R-CFG-045, R-CFG-046, R-NTF-042.
- OP-NTF-3 → thread map in store; github key = issue/PR number → R-NTF-040, R-NTF-028.
- OP-NTF-4 → NESS_THREAD_KEY → R-NTF-029.
- OP-NTF-5 → parts recorded; retry resumes at first unposted part; never repost → R-NTF-007.
- OP-NTF-6 → any session may post → R-NTF-008.
- OP-NTF-7 → slack/github characters, telegram UTF-16 code units → R-NTF-041, R-NTF-025.
- OP-NTF-8 → unknown kinds refused at config validation → R-CFG-045.

## contracts
- OP-CTR-1 → committed `contract.lock` in the session dir → R-CTR-001.
- OP-CTR-2 → session's decisions.md, "Amend <path>: <reason> (<date>)" → R-CTR-005.
- OP-CTR-3 → owner session or its manager → R-CTR-020.
- OP-CTR-4 → Edit/Write/MultiEdit and Codex apply_patch; Bash caught by contract check → R-CTR-009, R-CTR-021.
- OP-CTR-5 → lock per session dir; check verifies all locks on the branch → R-CTR-003.
- OP-CTR-6 → the trust root is the base branch and decisions.md: an `Amend <path>:` line written from Bash, or a moved base ref, is accepted by check; the gate integrator and review catch forged decisions → R-CTR-021.

## hooks
- OP-HOOK-1 → output guard refuses whole-file reads of saved tool-result files → R-HOOK-013.
- OP-HOOK-2 → Monitor, ScheduleWakeup, CronCreate, Bash while/until with `ness inbox`/`ness status` → R-HOOK-011.
- OP-HOOK-3 → ≤ 20 lines: open/inbox/send/reply/integrate/close/probe one-liners → R-HOOK-006.
- OP-HOOK-4 → Codex hooks passed with -c at launch → R-HOOK-018.
- OP-HOOK-5 → fail open unbound, closed (exit 2 + reason) bound → R-HOOK-002, R-HOOK-017.
- OP-HOOK-6 → shadowing check in the hook only for bound sessions → R-HOOK-002, R-HOOK-005.

## doctor
- OP-DOC-1 → exit 1 on any fail → R-DOC-021.
- OP-DOC-2 → ok/warn/fail, one line each, fix on fail → R-DOC-020.
- OP-DOC-3 → heartbeat stale > 60 s → R-DOC-022.
- OP-DOC-4 → outside a project: print version, then refuse → R-DOC-023.

## stats
- OP-STAT-1 → embedded dated price table; cents → R-STAT-001.
- OP-STAT-2 → --since date or duration → R-STAT-004.
- OP-STAT-3 → context = last call's prompt tokens per turn; compactions from markers → R-STAT-010.
- OP-STAT-4 → Codex out of v1 → R-STAT-013.
- OP-STAT-5 → from closed-session history → R-STAT-012.
- OP-STAT-6 → tool_result bytes per tool → R-STAT-011.
- OP-STAT-7 → private fixture in gitignored local dir; CI synthetic → R-STAT-008.

## models
- OP-MOD-1 → Claude aliases + full ids; Codex explicit ids → R-MOD-031, R-MOD-032.
- OP-MOD-2 → `[tiers]` in ness.toml, default §12 table → R-MOD-003.
- OP-MOD-3 → Claude ids resolved by the client → R-MOD-031.
- OP-MOD-4 → effort values per client atom → R-MOD-033.
- OP-MOD-5 → Codex `[agents]` defaults only → R-MOD-050.
- OP-MOD-6 → router_threshold default 0.8 → R-MOD-042, R-CFG-042.

## profiles-builtin
- OP-BLT-1 → `ness-` prefix for all built-in skills → R-BLT-006.
- OP-BLT-2 → commit-msg hook + trailer guidance → R-BLT-080, R-BLT-081.
- OP-BLT-3 → all five render together → R-BLT-005.
- OP-BLT-4 → ≈2k tokens ≈ 8,000 characters → R-BLT-014.
- OP-BLT-5 → `make validate` → R-BLT-054, R-BLT-059.
- OP-BLT-6 → gitignored `.private/denylist.txt`; runs when present, CI skips → R-BLT-003.
- OP-BLT-7 → research line out of v1 → R-BLT-100.
- OP-BLT-8 → evals local on demand within D31 rule → R-BLT-063, R-QA-042.
- OP-BLT-9 → python3 in requires_tools → R-BLT-058.

## quality
- OP-QA-1 → undefined ids in markers fail → R-QA-013.
- OP-QA-2 → reviews.md ledger accepted → R-QA-012.
- OP-QA-3 → gremlins, report-only in v1 → R-QA-023.
- OP-QA-4 → fuzz 30 s per target in CI, 10 min nightly → R-QA-022.
- OP-QA-5 → e2e behind build tag `e2e`, local on demand; abort at weekly usage ≥ 80% → R-QA-027.
- OP-QA-6 → module path `github.com/enea-scaccabarozzi/ness` (D38) → R-QA-043.
- OP-QA-7 → no line-coverage floor → R-QA-040.
- OP-QA-8 → python3 via mise in CI → R-QA-041.
