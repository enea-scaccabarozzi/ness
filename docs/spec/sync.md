> [!NOTE]
> Archive: published as written for the public release; only this note was added.

# Sync (SYNC)

Package: `internal/sync`, cli `sync`. Sources: §3 (writing into projects), §5 (`ness sync`), §6
(file table, sync rules, managed blocks, JSON/TOML, hooks wrapping, running sessions), §16
(#1, #2, #13, #15, #16, #17), D4, D22.

## Rendering and scope of writes

- **R-SYNC-001** `ness sync` MUST render the desired state from core, then enabled features,
  then profiles in their configured order, then project config (§6 rule 1).
  Verify: unit — a list slot filled by core, a feature and two profiles renders the parts in
  exactly that order.
- **R-SYNC-002** Rendering MUST be deterministic: two syncs from the same inputs produce
  byte-identical files and lock.
  Verify: testscript — sync, copy tree, delete generated files and lock, sync again; trees equal.
- **R-SYNC-003** `ness sync` MUST write only to planned paths, `.ness/lock.json` and
  `<path>.ness-new` files.
  Verify: testscript — tree snapshot before/after; every changed path is in the `--dry-run` plan,
  the lock or matches `*.ness-new`.
- **R-SYNC-004** `ness sync` MUST NOT overwrite or delete content it has not recorded in the
  lock (D4).
  Verify: testscript + fuzz(sync state machine) — for every unrecorded pre-existing file, block
  or key, its bytes are unchanged after sync.
- **R-SYNC-005** When a profile's `requires_tools` names a tool not on `PATH`, `ness sync` MUST
  fail before writing anything, naming the tool and the profile.
  Verify: testscript — `requires_tools = ["nonexistent-tool"]`; exit non-zero; tree unchanged.
- **R-SYNC-006** `ness sync` MUST NOT add generated files to `.gitignore` (generated files are
  committed, D22).
  Verify: testscript — the ness `.gitignore` block contains exactly `.worktrees/` and
  `*.ness-new`.

## Sync rules (§6)

- **R-SYNC-010** An absent target MUST be created and recorded in the lock (rule 2).
  Verify: testscript — file exists with rendered bytes; lock has its path, owner and hash.
- **R-SYNC-011** A present but unrecorded target identical to the desired content MUST be
  adopted: recorded in the lock, bytes unchanged (rule 2).
  Verify: testscript — pre-create identical file; after sync, mtime-independent bytes equal and
  lock records it; exit 0.
- **R-SYNC-012** A present but unrecorded target that differs from the desired content MUST be
  a conflict: not overwritten, desired content written to `<path>.ness-new`, exit 3 (rule 2).
  Verify: testscript — file bytes unchanged; `.ness-new` holds the render; exit 3.
- **R-SYNC-013** A recorded target unchanged since ness wrote it MUST be replaced with the new
  desired version and its lock hash updated (rule 3).
  Verify: testscript — upgrade a profile's file; target holds the new render; lock hash updated.
- **R-SYNC-014** A recorded target changed by the user, whose upstream render is unchanged since
  the user kept their version, MUST be left untouched with exit 0 (rule 4).
  Verify: testscript — `--keep` then re-sync with the same binary; bytes unchanged, no
  `.ness-new`, exit 0.
- **R-SYNC-015** A recorded target changed by the user, whose upstream render changed, MUST NOT
  be overwritten: the new render goes to `<path>.ness-new`, the path is listed on stdout, and
  sync exits 3 (rule 4).
  Verify: testscript — edit target, change profile text; target bytes unchanged; `.ness-new`
  present; path printed; exit 3.
- **R-SYNC-016** A conflict MUST NOT stop the rest of the sync: non-conflicted targets are still
  written.
  Verify: testscript — one conflicting and one clean target; the clean one is updated; exit 3.
- **R-SYNC-017** `ness sync --keep PATH` MUST record the user's current version of a conflicted
  path, delete `<path>.ness-new`, and make later syncs leave the path untouched until the upstream
  render changes again.
  Verify: testscript — conflict, `--keep PATH` → `.ness-new` gone; `sync` → exit 0; change
  upstream again → `.ness-new` and exit 3.
- **R-SYNC-018** `ness sync --keep PATH` MUST refuse a path that is not in conflict.
  Verify: testscript — `--keep README.md`; exit non-zero; lock unchanged.
- **R-SYNC-019** A target no longer desired and unchanged since ness wrote it MUST be deleted
  and removed from the lock (rule 5).
  Verify: testscript — drop a profile; its owned unchanged file is gone; lock entry gone.
- **R-SYNC-020** A target no longer desired but changed by the user MUST be kept, reported as
  orphaned on stdout, and removed from the lock so ness never touches it again (rule 5).
  Verify: testscript — edit owned file, drop the profile; file kept; stdout names it "orphaned";
  lock entry gone; a later sync leaves it.
- **R-SYNC-021** The sync decision engine MUST satisfy, for any sequence of user edits, profile
  changes and syncs: no unrecorded or user-changed bytes are ever overwritten or deleted.
  Verify: fuzz — state-machine fuzz over (edit, change-profile, sync, keep) sequences with the
  invariant checked after each step.
- **R-SYNC-022** `ness sync` MUST print the conflicted paths one per line.
  Verify: testscript — two conflicts → two lines naming each path.
- **R-SYNC-024** `ness sync` MUST exit 3 when at least one conflict (file, block or key) exists
  after the run, including unresolved `.ness-new` files from a previous run.
  Verify: testscript — leave a `.ness-new` unresolved; second sync exits 3.

## Flags

- **R-SYNC-030** `ness sync --check` MUST write nothing and exit 1 if any planned write,
  deletion or lock change exists, exit 3 if any conflict exists (3 wins over 1), and 0 otherwise.
  Verify: testscript — clean tree → 0; edit a profile's text → 1 and tree unchanged; pending
  conflict → 3.
- **R-SYNC-031** `ness sync --dry-run` MUST write nothing and print the plan (create, update,
  adopt, delete, conflict, orphan per path) with a unified diff per changed path; it MUST exit 3
  when the plan contains a conflict.
  Verify: testscript — tree unchanged; stdout contains one plan line per path and `@@` hunks;
  planned conflict → exit 3.
- **R-SYNC-034** `ness sync --dry-run --json` MUST print the plan as JSON (R-CFG-032/037).
  Verify: testscript — JSON with one entry per path and its action.
- **R-SYNC-032** `ness sync` MUST print the release notes of every version between the lock's
  recorded binary version (exclusive) and the running binary (inclusive), including each
  version's "running sessions" instructions.
  Verify: testscript — lock records 0.3.0, binary 0.3.2; stdout contains the 0.3.1 and 0.3.2
  notes in order and their "running sessions" section; none for 0.3.0.
- **R-SYNC-033** `ness sync` MUST NOT print release notes when the lock's version equals the
  binary's.
  Verify: testscript — second sync prints no notes.

## Managed Markdown blocks

- **R-SYNC-040** A managed block MUST be delimited by begin/end marker lines naming
  `<owner>/<block>` in the file type's comment syntax: Markdown `<!-- ness:begin <owner>/<block> -->`
  / `<!-- ness:end <owner>/<block> -->`; `#`-comment files such as `.gitignore`
  `# ness:begin <owner>/<block>` / `# ness:end <owner>/<block>`.
  Verify: unit — rendered AGENTS.md and .gitignore match the markers byte-for-byte.
- **R-SYNC-041** A managed block MUST be found by its markers wherever it has been moved in the
  file, and updated in place.
  Verify: testscript — move the block to the top of AGENTS.md; after an upstream change it is
  updated at the top; surrounding text unchanged.
- **R-SYNC-042** A new block MUST be appended at the end of the file.
  Verify: testscript — enable a profile; its block is the last thing in AGENTS.md.
- **R-SYNC-043** A block's recorded hash MUST cover only the content between its markers.
  Verify: unit — editing text outside the block leaves the block "unchanged"; sync exit 0.
- **R-SYNC-044** Text outside managed blocks MUST be byte-identical after sync.
  Verify: fuzz — random surrounding text and block positions; outside bytes preserved.
- **R-SYNC-045** A user edit inside a managed block whose upstream changed MUST be a conflict at
  file granularity: the file is not modified, the whole file's render goes to `<path>.ness-new`
  and sync exits 3 (§16 #17).
  Verify: testscript — edit inside `core/router`, change upstream; AGENTS.md bytes unchanged;
  `AGENTS.md.ness-new` present; exit 3.
- **R-SYNC-046** Malformed markers (begin without end, duplicate names, nested blocks) MUST make
  sync refuse to touch that file, naming the file and line, while other files still sync.
  Verify: testscript — remove an end marker; file unchanged; error names file:line; exit
  non-zero.
- **R-SYNC-047** The managed-block parser MUST NOT panic on any input.
  Verify: fuzz — arbitrary Markdown with marker fragments.

## JSON/TOML keys

- **R-SYNC-050** In JSON and TOML targets ness MUST record the exact key paths it set and leave
  every unowned byte identical: JSON key order and indentation preserved; TOML edited line-level
  for owned keys only (comments and other lines untouched).
  Verify: testscript + fuzz — settings.json and config.toml with user keys, comments and odd
  indentation; after sync, a byte diff touches only owned key lines.
- **R-SYNC-051** A key present with the desired value but unrecorded MUST be adopted; present
  with a different value MUST be a conflict at file granularity (whole-file `.ness-new`, resolved
  with `--keep PATH`) (§16 #2).
  Verify: testscript — `autoCompactWindow` pre-set equal → adopted, exit 0; different → value
  unchanged, `.claude/settings.json.ness-new` present, exit 3.
- **R-SYNC-052** A changed owned key MUST be a conflict for that file (whole-file `.ness-new`,
  file unchanged) while other files still sync.
  Verify: testscript — change one owned key in settings.json and upgrade a skill; settings.json
  unchanged with a `.ness-new`; the skill updated; exit 3.
- **R-SYNC-053** Hook entries in `.claude/settings.json` MUST be recognised by their command
  (`ness hook …`), not by position, so user-added hook entries are preserved and an existing
  ness entry is not duplicated.
  Verify: testscript — user hook before the ness hook in the array; after two syncs exactly one
  ness entry per event and the user entry unchanged.
- **R-SYNC-054** Every hook command ness writes MUST be wrapped as
  `command -v ness >/dev/null || exit 0; ness hook <EVENT>`.
  Verify: testscript — read settings.json; run each hook command with `ness` absent from PATH →
  exit 0, empty output.
- **R-SYNC-055** The JSON/TOML key editors MUST NOT panic on any input and MUST refuse a file
  that does not parse, naming it.
  Verify: fuzz — arbitrary bytes for settings.json and config.toml.

## The file table (§6)

- **R-SYNC-060** `AGENTS.md` MUST contain the `ness:core/router` block and one block per
  profile that contributes one.
  Verify: testscript — with two block-contributing profiles, three ness blocks exist.
- **R-SYNC-061** The `core/router` block MUST begin with the line "Applies only when the `ness`
  command exists in this checkout. Otherwise ignore this block." after its heading.
  Verify: unit — golden render.
- **R-SYNC-062** When `CLAUDE.md` is absent, sync MUST create it as a symlink to `AGENTS.md`.
  Verify: testscript — `readlink CLAUDE.md` = `AGENTS.md`.
- **R-SYNC-063** When `CLAUDE.md` is a regular file, sync MUST manage an `@AGENTS.md` import
  block in it and leave the rest unchanged.
  Verify: testscript — pre-existing CLAUDE.md; after sync it has a ness block containing
  `@AGENTS.md`; other lines unchanged.
- **R-SYNC-064** Sync MUST own `.agents/skills/ness-*/` and MUST NOT touch any other entry of
  `.agents/skills/`.
  Verify: testscript — a project skill `.agents/skills/deploy/` survives sync and profile
  removal byte-identical.
- **R-SYNC-065** When `.claude/skills` is absent, sync MUST create it as a symlink to
  `.agents/skills`.
  Verify: testscript — `readlink .claude/skills` = `../.agents/skills` (or equivalent relative
  target resolving to it).
- **R-SYNC-066** Sync MUST generate `.claude/agents/ness-*.md` from the tier table (see
  models.md) and own them.
  Verify: testscript — files exist for each named subagent; lock owner = core.
- **R-SYNC-067** Sync MUST own in `.claude/settings.json` the hook entries running `ness hook`,
  `autoCompactWindow` and `env.CLAUDE_CODE_SUBAGENT_MODEL`.
  Verify: testscript — lock lists exactly these key paths for that file.
- **R-SYNC-068** Sync MUST own in `.codex/config.toml` the compaction limit and subagent default
  keys when the client `codex` is configured.
  Verify: testscript — `client = "codex"` → keys present and recorded.
- **R-SYNC-069** Sync MUST manage a `.gitignore` block containing `.worktrees/` and
  `*.ness-new`.
  Verify: testscript — `git check-ignore .worktrees/x foo.ness-new` succeeds.
- **R-SYNC-070** A profile file declared `seed` MUST be created only if missing and then never
  modified by ness.
  Verify: testscript — seed created; user edits it; profile text changes; sync leaves it, exit 0.
- **R-SYNC-071** `.ness/lock.json` MUST record, per file, block and key, the owner and the hash,
  plus the binary version and profile versions, with all keys and arrays sorted.
  Verify: unit — lock written from a shuffled plan is byte-identical to one from a sorted plan.
- **R-SYNC-072** Sync MUST NOT write any file outside the repository.
  Verify: testscript — `$HOME` snapshot unchanged after sync.
- **R-SYNC-073** The lock reader MUST NOT panic on any input and MUST refuse a lock that does
  not parse, naming it.
  Verify: fuzz — arbitrary bytes for lock.json.
- **R-SYNC-074** ness itself MUST pass `ness sync --check` on its own repository.
  Verify: testscript in CI (see R-QA-006).

## Decided details

- **R-SYNC-080** A present `.claude/skills` that is a real directory (not a symlink) MUST be a
  conflict: sync refuses to write into it and exits 3 naming the path.
  Verify: testscript — `mkdir .claude/skills`; exit 3; directory contents unchanged.
- **R-SYNC-081** A seeded file deleted by the user MUST NOT be recreated by later syncs.
  Verify: testscript — seed created, `rm` it, sync → absent, exit 0.
- **R-SYNC-082** Profile blocks in AGENTS.md MUST use the marker name `<profile>/<block>`; core
  blocks `core/<block>`.
  Verify: unit — `local` profile block renders as `ness:begin local/local` (or its declared block
  name) and the router as `core/router`.
- **R-SYNC-083** Sync MUST NOT try to detect hand-written AGENTS.md sections that duplicate ness
  blocks; adoption guidance is delivered only through release notes and docs.
  Verify: review — no heuristic code; release notes of the first version carry the adoption
  notes.
- **R-SYNC-084** Sync MUST set `.claude/settings.json` `autoCompactWindow` to 265000.
  Verify: unit — rendered value.
- **R-SYNC-085** Sync MUST set `env.CLAUDE_CODE_SUBAGENT_MODEL` to the economy tier's Claude
  model.
  Verify: testscript — change the economy tier; after sync the variable follows.
- **R-SYNC-086** With `client = "codex"`, sync MUST set `.codex/config.toml`
  `model_auto_compact_token_limit` to 235000.
  Verify: unit — rendered value.
- **R-SYNC-087** Lock hashes MUST be SHA-256 (hex) and the lock MUST carry `"schema": 1`.
  Verify: unit — recorded hash equals `sha256sum` of the content (block inside only); schema = 1.
- **R-SYNC-088** A lock with an unknown schema version MUST be refused.
  Verify: testscript — `"schema": 2` → exit 4.
- **R-SYNC-089** Release notes MUST be embedded in the binary from `CHANGELOG.md`, each version
  with a "Running sessions" subsection, which sync prints with the notes (R-SYNC-032).
  Verify: unit — the embedded notes equal the CHANGELOG sections; each has the subsection.
