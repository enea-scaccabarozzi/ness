# Brief (operator's words, verbatim)

## Message 1
As you can see i've done my first public release of this tool in last session, and now that we have a public registry or a public surface i would love to start desigining an update system for ness.
In aprtivulr i fully expect that ness will eveolve a lot further as times passes, with its direction settled from fiedl feedbacks from rela world, jsut like happened in 1 and 2 field feedbacks root sessions in past iterations.
We thus need a system that allows us to easily push updates to users, while also ensuring that the updates are stable and do not break existing functionality or consuming projects. 
Another key requirement is the fact that a ness update should also work with opened ongoing sessions, as we've done in field feedback 1 and 2. 
This is not a simple task and you have to fully understand its implications and requirements, before committing to anything propose me a raccomended  approache and we can discuss it in detail, refining it, before moving forward.

## Message 2 (after the first proposal)
I like this however i would like to add support for Self-update and update-available checks, configurable in each project in this release.

Also while building this thing you can test it live using the 3 personal projects which you can find on this amchine using ness, however they must stay private and do not leak into public repo.

Another thing that this update path should not rely fully on mise (which is only a tool that i personally use) but should have more spread toolings as default, and mise only as an option for mise users. This aprt touches also the first install part i think

Also be sure to update docs accordingly to refelct this changes once you have completed everything

## Message 3 (after the refined proposal, which asked whether auto = "install" should exist)
Perfect, i like everything, and auto shoudl stay also

## Message 4 (after the session proposal)
Perfect go on

## Approved design (agreed in chat, summarised by the root)
- Distribution: tagged GitHub releases via release-please + goreleaser. Default install: curl install.sh (OS/arch detection, checksum-verified, ~/.local/bin or $NESS_INSTALL_DIR). `go install` and mise documented as options. No Homebrew tap this release (separate public repo is the operator's call).
- Compatibility contract: patch = no migration, no action-requiring synced-text change; minor may migrate/change synced files but the binary accepts the previous minor's files and store; removals take one deprecation minor; a migration never changes the behaviour of an existing session.
- Store: backup state.sqlite3.bak-v<N> before migrating; rollback = previous binary + backup, `ness doctor` prints the steps.
- No silent degradation: hooks block loudly on pin/store skew instead of failing open silently; worker unit points at a stable binary path.
- Live sessions: `ness upgrade` per project (migrate with backup, sync base branch, commit, restart worker); worker detects a session worktree lock older than base, delivers at the session's idle point one item with the Running-sessions notes + changed skills to re-load + `ness sync` in the worktree; session self-applies, root gets an fyi copy.
- Self-update: `ness self-update [--version X] [--channel stable|rc]`, checksum-verified atomic swap, keeps ness.prev; defers to mise/go when they manage the binary.
- Update-available check: at most daily, cached in ~/.cache/ness, never blocks hooks; shown in `ness doctor` and the root's session-start text, never phone/inbox.
- Per-project `.ness/ness.toml [update]`: check = bool, channel = stable|rc, auto = off|notify|install (auto stays). The binary is per-machine: self-update refuses a version a pinned project on the machine cannot run, naming it.
- Stability: CI upgrade test against the previous release's committed fixture (store migrates, old-text session passes hooks, sync --check shows expected changes, program checks still fire); rc releases field-tested on the operator's 3 projects before stable promotion.
- Live testing on the 3 private projects is authorized; their names stay in docs/discovery, leak scan before every push; schema-changing builds tested in a scratch repo first.
- Docs updated at the end, every command run by hand.
