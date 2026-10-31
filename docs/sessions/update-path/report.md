# Report: update-path

## Delivered
- ness v0.2.0 (stable, 2026-10-31), the update path of `specs.md`: `install.sh` as the default install with
  `go install` and mise as options; `ness self-update` (the ceiling, `ness.prev`, worker re-point) and
  `ness upgrade` (store backup `bak-v<N>`, sync, pin bump, commit, worker restart); `[update]` check, channel and
  auto per project; the daily check cache; the machine registry and the install veto (UP-D6); loud hooks on
  pin, store and config skew; one update item per open session at its idle point plus the root's fyi; rc tags
  through `rc.yml`, stable refused without an rc of the same tree, a fixture and a `sessions:` commit; the CI
  upgrade test from the v0.1.0 fixture.
- Docs: install, upgrade, `[update]` config, environment, the compatibility page, releasing and CONTRIBUTING.
- main: 501ac01 (feature), 62f080e (field fixes), f70781d (release 0.2.0), deb689b (source-build version, the
  ness repository on 0.2).

## Evidence
- Every task reviewed (UP-D11 batching); the final whole-branch review found 0 Critical and 5 Important, all fixed
  with the 8 parked items and confirmed by a scoped re-review. TestRequirementCoverage green with reviews.md
  entries for R-UPD-003, R-UPD-091 and R-UPD-095.
- Leak scan (denylist, public-tree selection, home path) PASS on a clean clone before every push.
- Every `ness integrate` gate green (the first run caught the command-surface scripts, fixed in 72e5628).
- Live test on the three field projects (the infrastructure lab, the research lab, the production backend):
  0.1.0 → rc.1 through the public `install.sh --channel rc`, rc.1 → rc.2 and rc.2 → 0.2.0 through
  `ness self-update` (previous kept as `ness.prev`), `ness upgrade` in each; every worker on 0.2.0; the open roots
  got exactly one update item each.

## Field findings and their state
- Fixed (62f080e): upgrade's fixed `chore(ness)` subject refused by a scope-checking commit hook now ends with a
  one-line fix; unrelated untracked files no longer block an upgrade; `--check` wording for an rc newer than
  stable; releasing.md names the Actions pull-request setting.
- Fixed (deb689b): the source-build version was a constant release-please never bumped, so the ness repository
  could not move its own pin.
- Open: the refused-commit line quotes the hook's first line, which can be a banner rather than the cause; CI on
  the release pull request ran no jobs ("workflow file issue"), the push CI on main is the gate that ran; the
  production backend's upgrade commits are local on its base until its root integrates (a push there deploys).

## Rulings I made
- reviews.md entries written at the final review; R-UPD-021 uses a character blacklist; the R-UPD-053 line stays
  verbatim for managed binaries; internal/update added to the R-QA-002 layout.
- PROVISIONAL product call: a project's own upgrade is never held by other projects' auto; the UP-D6 veto gates
  only the machine-wide install.
- The worker records its project in the registry at start (R-UPD-031 amended), so a project left on the old pin
  counts for the ceiling and the veto.
- R-CFG-030 lists 21 commands (self-update and upgrade added).
- No re-review of the last three small fixes (each had fail-before evidence); the home-path test's false positive
  on a hand-built untrimmed binary in a dev checkout is left; the downgrade hint keeps `--version <pin>.0`.

## Lessons
- Focused tests per task let a cross-cutting script (the command surface) stay red until the gate; run the
  surface scripts whenever a command is added.
- Parallel writers in one worktree block each other's commits (the pre-commit lint runs on the working tree);
  serialize writers, parallelize only read-only reviewers.
- A live test on real projects finds what scratch repos cannot: project commit-message rules, stray untracked
  files, repository settings and the release tool's version bookkeeping all surfaced only there.
- A release pipeline is not tested until a release runs through it: check every place a version is written
  (manifest, changelog, ldflags, the source constant) before the first cut.
