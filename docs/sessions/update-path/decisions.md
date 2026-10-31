# Decisions — update path

Append-only. Ids are the root's.

- **UP-D1** (2026-10-31, operator, chat) Design approved: tagged releases + install.sh default
  (go install and mise as options, no Homebrew tap), compatibility contract, store backup, loud
  hooks on skew, `ness upgrade` + worker-delivered update items for live sessions, CI upgrade
  test, rc field run. See specs.md.
- **UP-D2** (2026-10-31, operator, chat) Self-update and the update-available check ship in this
  release, configured per project; `auto = "install"` is kept.
- **UP-D3** (2026-10-31, operator, chat) Live testing on the three private projects is
  authorized; nothing naming them reaches the public repository.
- **UP-D4** (2026-10-31, operator, chat) Docs are updated at the end to reflect all changes.
- **UP-D5** (2026-10-31, root, PROVISIONAL) Root calls listed in specs.md: pin accepts N and N-1
  minor; `auto` default `notify`; machine registry `$XDG_CONFIG_HOME/ness/projects`; ceiling rule
  on pins only, channel only decides what is offered; first upgrade fixture cut from v0.1.0.
- **UP-D6** (2026-10-31, operator, chat "As raccomended") Mixed `auto` on one machine: veto.
  Unattended install only when every registered project has `auto = "install"`; otherwise notify.
- **UP-D7** (2026-10-31, root, PROVISIONAL) Builder-check settlements 1–5 in specs.md: upgrade
  bumps the pin; sessions merge base; loud hooks only in ness sessions with `ness` commands
  allowed; rc cut from the release PR, stable refused without an rc; unattended upgrade retries
  each pass, failures in doctor and root session-start.
- **UP-D8** (2026-10-31, operator, chat "go as raccomended") Stale local tag v0.1.0 (old private history, kept in the pre-public bundle) deleted; v0.1.0 tagged on main a7903ca and published via goreleaser (linux amd64/arm64 + checksums). It is the first release to upgrade from.
- **UP-D9** (2026-10-31, root, PROVISIONAL) Plan settlements: loud hooks apply when NESS_SESSION
  is set or the branch matches `session/*`; while blocked only a single simple `ness …` command
  passes; `self-update`, `upgrade` and `doctor` skip the pin check; 1.0 does not accept 0.x pins;
  the behind-the-binary fyi is sent under every `auto` mode; the worker fetches daily whenever
  `check = true`; session-record commits use `docs(sessions):`; stable needs an rc tag whose
  tree equals HEAD's; children compare with their own `Session.Base`.
- **UP-D10** (2026-10-31, root) Order at the end: docs → close-out gates and integrate (push only
  after the operator's go) → rc cut and live testing on the three projects (batched, operator's
  go) → field fixes through a new integrate.
- **UP-D11** (2026-10-31, operator, chat "Go with faster appraoch") Execution speed-up: judges written ahead in parallel; full per-task review only for tasks 2, 3, 5, 8, 10, 11, 12; the others reviewed in combined batches (4+6+7, 9, 13+14, 15, 16); final whole-branch review unchanged.
