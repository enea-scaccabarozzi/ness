# Changelog

One section per version, newest first, written by release-please from the commits. Every
version has a "Running sessions" subsection (from `sessions:` commits): what an open session
must do after `ness sync` picks the version up. `ness sync` prints the sections between the
lock's version and the binary's.

## [0.2.0](https://github.com/enea-scaccabarozzi/ness/compare/v0.1.0...v0.2.0) (2026-10-31)


### Features

* **atom:** GitHub release source; testkit fake release server; one source of the binary path ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **cli:** daily update check in doctor and the root's session-start line ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **cli:** ness self-update: checksum-verified atomic install with the ceiling rule ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **cli:** ness upgrade migrates, syncs, bumps the pin and commits the base checkout ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **config:** [update] check, channel and auto; sync records the project in the machine registry ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **config:** a 0.N binary runs 0.N-1 pins; the pin refusal names its fix; rc versions order and print their release's notes ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **hook:** block loudly on pin, store or config skew inside a ness session or session branch; single ness commands still pass ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* install.sh: checksum-verified install from the GitHub releases ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **store:** back up the store before a migration; doctor prints the rollback ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **update:** install.sh, self-update, upgrade, [update] checks and live-session updates ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **update:** resolve, verify and install releases; ceiling, cache, failure record and the unattended decision ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **worker:** daily check, unattended install under the machine veto and upgrade at the idle point, failures surfaced ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **worker:** one update item per live session whose worktree is behind its base, with an fyi to the root ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))


### Bug Fixes

* **ci:** check every merged release PR still labelled autorelease: pending at its merge commit ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **cli:** ness upgrade restarts the worker on an already-at rerun ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **cli:** the command surface lists self-update and upgrade ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **config:** check a newer pin before the strict ness.toml parse ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **docs:** drop a real home path from the update-path plan ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **doctor:** the binary check prints PinError.Fix, the one pin fix text ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **doctor:** the rollback stops the worker and drops the -wal before the cp; test the refused migration ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **hook:** session-start prints the skew line when profiles do not resolve ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **install:** run everything from main so a truncated download runs nothing ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **release:** upgrade-fixture.sh is executable ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **self-update:** apply the ceiling before the managed-binary refusal ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **self-update:** the right fix for a downgrade and for --check --channel ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **update:** RecordFailure overwrites a corrupt record ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **update:** resolve symlinked GOBIN, GOPATH and MISE_DATA_DIR ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **worker:** an install never holds the project's own upgrade ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **worker:** no update item while the base pins a newer ness than the worker ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **worker:** one unattended install per version with a 24 h back-off, failures cleared by any upgrade, the idle gap silent, the behind lock read from the base ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **worker:** one-line minors from the final review ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **worker:** re-point keeps the unit's PATH ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))
* **worker:** record the project in the registry when the worker starts ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))


### Running sessions

* merge the base, run ness sync --check, re-load the changed skills ([501ac01](https://github.com/enea-scaccabarozzi/ness/commit/501ac01646d2a041b6f5aa2765c2ccd695737422))

## [0.1.0] (2026-10-10)

### Features

* v1: the session tree, messages, the worker, probes, notify, contracts, stats and the five
  built-in profiles.
* Start a session by asking: in a plain Claude Code conversation in a ness project, the agent
  promotes the conversation to a root itself (`ness open --root --here`).

### Running sessions

* Nothing to do: this is the first version.
