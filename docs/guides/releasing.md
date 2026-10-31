---
title: Release ness
description: Cut a release candidate from the release pull request, run it on the field projects, then promote it to stable.
---

Every stable release goes through a release candidate (R-UPD-095). release-please keeps a
release pull request open on main. You cut `vX.Y.Z-rc.N` tags from that pull request by hand,
install them on the field projects, and merge the pull request only once the field window meets
the promotion criteria below. Each tag is built by exactly one workflow: `rc.yml` builds rc tags
and `release-please.yml` builds stable ones.

## Prerequisites

- Push access to the ness repository and the `gh` CLI, logged in.
- One or more field projects on ness, each with a base checkout that you can commit to.
- The tools `mise install` puts on your path, goreleaser among them, to try a step locally.

## Checklist

1. **Wait for the release pull request.** release-please opens or updates it on every push to
   main. It bumps `.release-please-manifest.json` and adds a `## [X.Y.Z]` section to
   `CHANGELOG.md`.

2. **Cut a release candidate.**

   ```sh
   gh workflow run rc.yml
   ```

   The workflow checks out `release-please--branches--main`, which is the pull request's head.
   It reads the version from the manifest and picks the next tag, `vX.Y.Z-rc.N`, where N is the
   number of existing `vX.Y.Z-rc.*` tags plus one. It refuses when `CHANGELOG.md` has no
   `## [X.Y.Z]` section. It runs the Running sessions notes test (`go test ./internal/sync/`),
   pushes the tag, and has goreleaser publish the build as a GitHub pre-release. To cut from
   another branch or commit, pass `-f ref=REF`. If the run fails after it pushed the tag, delete
   that tag (`git push origin :refs/tags/vX.Y.Z-rc.N`) before you cut again.

   To see the tag it would pick, run the same check on a checkout of the pull request's head:

   ```sh
   sh .github/scripts/release-check.sh rc 0.2.0
   ```

   ```text title="Output"
   v0.2.0-rc.1
   ```

   Without the `## [0.2.0]` section, as on main, it refuses:

   ```text title="Output (exit 1)"
   CHANGELOG.md has no ## [0.2.0] section: run the rc workflow on release-please's pull request
   ```

   To build the archives and `checksums.txt` locally without publishing anything, run goreleaser
   in snapshot mode. It writes them to `dist/`:

   ```sh
   goreleaser release --snapshot --clean
   ```

   ```text title="Output (last lines)"
     • calculating checksums
     • writing artifacts metadata
     • release succeeded after 2m48s
     • thanks for using GoReleaser!
   ```

3. **Install the rc on the field projects.** In each project's base checkout, commit this:

   ```toml title=".ness/ness.toml"
   [update]
   channel = "rc"
   ```

   Then run `ness self-update` once, from one of those base checkouts: it follows that
   project's channel and installs the newest rc. Run `ness upgrade` in each project's base
   checkout.

4. **Run the field window.** Promote only when the window meets every one of these criteria:

   | Criterion | Bound |
   | --- | --- |
   | False stalls | under 10% |
   | Worker notices, as a share of phone posts | under 20% |
   | Lost gate runs | 0 |
   | Inbox false positives | 0 |
   | Pick hangs | 0 |
   | Sessions that missed the update item | 0 |

   A session missed the update item when it was open at upgrade time and neither had its update
   request disposed nor merged its base. Hooks failing open are not part of the window: the
   `hook_skew` testscripts cover them. Promoting the rc then sends every open session a second
   update item, for `0.N.0-rc.K` to `0.N.0`, with no notes: expect it, it is not a miss.

5. **If main moved, cut a new rc.** A stable release needs an rc tag whose tree equals the tree
   of the release commit. When the pull request picks up new commits after the rc, go back to
   step 2.

6. **Squash-merge the release pull request.** release-please releases every merged pull request
   that still carries the `autorelease: pending` label. On each push to main, before
   release-please runs, the stable job checks out the merge commit of each such pull request and
   runs `sh .github/scripts/release-check.sh stable X.Y.Z` there. It refuses, in this order, when:
   - no `vX.Y.Z-rc.*` tag exists;
   - no such tag has the merge commit's tree;
   - the previous stable tag has no upgrade fixture under `internal/cli/testdata/upgrade/`;
   - no `sessions:` commit has landed since the previous stable tag.

   After the checks pass, release-please tags the release. The job then runs the notes test and
   has goreleaser attach the binaries.

   Run the same check on the merge commit before you merge, from a checkout of the pull
   request's head with its rc tag fetched:

   ```sh
   sh .github/scripts/release-check.sh stable 0.2.0
   ```

   It prints nothing and exits 0 when the release may go out. Each refusal exits 1 with one line:

   ```text title="Output (exit 1)"
   no v0.2.0-rc.* tag: cut a release candidate with the rc workflow and run its field window first
   no v0.2.0-rc.* tag has HEAD's tree: cut a new rc
   no upgrade fixture for v0.1.0: run scripts/upgrade-fixture.sh v0.1.0
   no sessions: commit since v0.1.0; add one (e.g. 'sessions: nothing to do') before releasing
   ```

   A refusal holds every later push to main: the merged pull request keeps its
   `autorelease: pending` label, and each push runs the same check at its merge commit. To
   recover from a missing rc, cut an rc of the merge commit's tree, then rerun the job:

   ```sh
   gh workflow run rc.yml -f ref=MERGE_SHA
   gh workflow run release-please.yml
   ```

   A missing upgrade fixture or a missing `sessions:` commit cannot be fixed this way. The check
   reads the merge commit's tree and the commits before it, so a later commit does not change
   its answer. Take the release back out instead:

   1. Revert the release merge on main and push the revert. The push still fails the check,
      because the old pull request is still pending.

      ```sh
      git revert --no-edit MERGE_SHA
      git push origin main
      ```

   2. Fix the cause on main: commit the previous release's fixture
      (`scripts/upgrade-fixture.sh vPREVIOUS`, step 7), or a `sessions:` commit such as
      `sessions: nothing to do`. Push it.

   3. Take the label off the old pull request N, so the check and release-please skip it:

      ```sh
      gh pr edit N --remove-label 'autorelease: pending'
      ```

   4. Rerun the job. With no pull request pending, the check passes and release-please opens a
      fresh release pull request from main:

      ```sh
      gh workflow run release-please.yml
      ```

   5. Start again at step 2: the new pull request needs its own rc, because its tree differs
      from the old rc's.

7. **Commit the upgrade fixture of the new release.**

   ```sh
   scripts/upgrade-fixture.sh vX.Y.Z
   ```

   It builds ness at the local tag, runs it in a scratch repository and writes the fixture. It
   needs the tag in your checkout (`git fetch --tags`) and no network. For `v0.1.0`:

   ```text title="Output"
   wrote /home/you/src/ness/internal/cli/testdata/upgrade/v0.1.0
   ```

   Commit the fixture. In the same commit, point the `upgrade_previous` testscript at the new
   fixture and raise its `NESS_TEST_VERSION`. The next stable release refuses to go out without
   this fixture.

## Commit types

- Session-record commits use `docs(sessions): <scope> …`. They do not appear in the changelog
  and do not count as a sessions commit.
- `sessions:` is reserved for Running sessions notes. These land in the changelog's Running
  sessions section, and the update item carries them to live agents. A release with nothing to
  tell them still needs one, such as `sessions: nothing to do`.
